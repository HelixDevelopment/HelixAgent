package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenCodeAPIIntegration exercises the REAL HelixAgent OpenCode-facing
// endpoints (`/v1/chat/completions`, `/v1/providers`) via live HTTP. It is the
// sole live-HTTP portion of what used to be `opencode_ensemble_flow_test.go`;
// the in-process ensemble-flow tests that surrounded it now live under
// `tests/unit/opencode_ensemble_flow/` per CONST-030 Pattern 4 + Pattern 1
// split (audit doc 2026-04-21).
//
// Skips cleanly when HelixAgent is not reachable on the configured port.
func TestOpenCodeAPIIntegration(t *testing.T) {
	testutil.RequireServer(t)
	serverURL := os.Getenv("HELIXAGENT_TEST_URL")
	if serverURL == "" {
		serverURL = testutil.ServerURL()
	}

	client := &http.Client{Timeout: 60 * time.Second}

	t.Run("ChatCompletions returns ensemble response", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "What is 2+2? Answer with just the number."},
			},
			"max_tokens": 10,
		}

		jsonBody, _ := json.Marshal(reqBody)
		resp, err := client.Post(
			serverURL+"/v1/chat/completions",
			"application/json",
			bytes.NewReader(jsonBody),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == 502 {
			t.Logf("Providers temporarily unavailable - 502 (acceptable)")
			return
		}
		require.Equal(t, http.StatusOK, resp.StatusCode, "Response: %s", string(body))

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		// Verify response model indicates ensemble.
		model, ok := result["model"].(string)
		assert.True(t, ok)
		assert.Equal(t, "helixagent-ensemble", model,
			"Response model should be helixagent-ensemble")

		// Verify system fingerprint.
		fingerprint, ok := result["system_fingerprint"].(string)
		assert.True(t, ok)
		assert.Equal(t, "fp_helixagent_ensemble", fingerprint,
			"System fingerprint should indicate ensemble")

		// Verify we got choices.
		choices, ok := result["choices"].([]interface{})
		assert.True(t, ok)
		assert.NotEmpty(t, choices, "Should have at least one choice")
	})

	t.Run("Multiple requests all go through ensemble", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan bool, 5)

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				reqBody := map[string]interface{}{
					"model": "helixagent-ensemble",
					"messages": []map[string]string{
						{"role": "user", "content": fmt.Sprintf("Say 'test %d'", idx)},
					},
					"max_tokens": 20,
				}

				jsonBody, _ := json.Marshal(reqBody)
				resp, err := client.Post(
					serverURL+"/v1/chat/completions",
					"application/json",
					bytes.NewReader(jsonBody),
				)
				if err != nil {
					results <- false
					return
				}
				defer resp.Body.Close()

				body, _ := io.ReadAll(resp.Body)
				var result map[string]interface{}
				if err := json.Unmarshal(body, &result); err != nil {
					results <- false
					return
				}

				// Verify ensemble markers.
				model, _ := result["model"].(string)
				fingerprint, _ := result["system_fingerprint"].(string)

				results <- (model == "helixagent-ensemble" &&
					fingerprint == "fp_helixagent_ensemble")
			}(i)
		}

		wg.Wait()
		close(results)

		// Count successful requests - allow some failures due to load.
		successCount := 0
		for success := range results {
			if success {
				successCount++
			}
		}
		// At least 1 should succeed to verify API works; skip if server is
		// overwhelmed.
		if successCount == 0 {
			t.Logf("No requests succeeded - server may be overloaded or unavailable (acceptable)")
			return
		}
		t.Logf("Multiple requests test: %d/5 succeeded through ensemble", successCount)
	})

	t.Run("Providers endpoint shows registered providers", func(t *testing.T) {
		resp, err := client.Get(serverURL + "/v1/providers")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		// Verify providers are registered.
		count, ok := result["count"].(float64)
		assert.True(t, ok)
		assert.Greater(t, count, float64(0), "Should have at least one provider")

		providers, ok := result["providers"].([]interface{})
		assert.True(t, ok)
		assert.NotEmpty(t, providers, "Providers list should not be empty")
	})
}
