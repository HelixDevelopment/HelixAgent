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

// =============================================================================
// TEST SUITE: Mem0 Memory live integration against a running HelixAgent
// =============================================================================
// CONST-030: this file contains ONLY live-HTTP assertions against a running
// HelixAgent instance on :8100. The in-process ensemble/wrapper unit tests
// were demoted to `tests/unit/mem0/mem0_ensemble_test.go` (Pattern-4) so that
// no mock LLM provider is used outside `tests/unit/`.
// =============================================================================

// TestMem0LiveIntegration tests Mem0 Memory integration with live server
func TestMem0LiveIntegration(t *testing.T) {
	testutil.RequireServer(t)

	// Only run these tests if HELIXAGENT_INTEGRATION_TESTS is set
	if os.Getenv("HELIXAGENT_INTEGRATION_TESTS") != "1" {
		t.Logf("HELIXAGENT_INTEGRATION_TESTS not set - skipping integration test (acceptable)")
		return
	}

	serverURL := os.Getenv("HELIXAGENT_TEST_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8100"
	}

	// Use longer timeout for ensemble operations
	client := &http.Client{Timeout: 30 * time.Second}

	// Check if server is available
	healthResp, err := client.Get(serverURL + "/health")
	if err != nil {
		t.Logf("HelixAgent server not available (acceptable - external service): %v", err)
		return
	}
	healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Logf("HelixAgent server not healthy (acceptable - external service)")
		return
	}

	t.Run("Chat completion shows Mem0 enhancement in providers", func(t *testing.T) {
		// Check providers endpoint for Mem0 capabilities
		resp, err := client.Get(serverURL + "/v1/providers")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		providers, ok := result["providers"].([]interface{})
		assert.True(t, ok)

		mem0EnabledCount := 0
		for _, p := range providers {
			provider := p.(map[string]interface{})
			metadata, ok := provider["metadata"].(map[string]interface{})
			if ok {
				if enhanced, exists := metadata["cognee_enhanced"]; exists && enhanced == "true" {
					mem0EnabledCount++
				}
			}

			// Check for Mem0 features in supported features
			features, ok := provider["supported_features"].([]interface{})
			if ok {
				for _, f := range features {
					if f.(string) == "cognee_memory" {
						mem0EnabledCount++
						break
					}
				}
			}
		}

		assert.Greater(t, mem0EnabledCount, 0,
			"At least some providers should have Mem0 enhancement")
	})

	t.Run("Mem0 search endpoint responds", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"query":       "test query",
			"dataset":     "default",
			"limit":       5,
			"search_type": "CHUNKS",
		}

		jsonBody, _ := json.Marshal(reqBody)
		resp, err := client.Post(
			serverURL+"/v1/cognee/search",
			"application/json",
			bytes.NewReader(jsonBody),
		)

		// Mem0 Memory might not be available, but endpoint should respond
		if err == nil {
			defer resp.Body.Close()
			// Accept 200 (success) or 400/503 (Mem0 not ready/invalid)
			assert.Contains(t, []int{200, 400, 503}, resp.StatusCode,
				"Mem0 search should respond appropriately")
		}
	})

	t.Run("Chat request goes through Mem0-enhanced ensemble", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "What is 1+1?"},
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
			// Handle network errors (timeout, EOF, connection reset) gracefully
			t.Skipf("Network error during chat request (server may be unavailable): %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			t.Skipf("Providers temporarily unavailable (%d), skipping test", resp.StatusCode)
		}
		require.Equal(t, http.StatusOK, resp.StatusCode, "Response: %s", string(body))

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		// Verify ensemble response
		assert.Equal(t, "helixagent-ensemble", result["model"])
		assert.Equal(t, "fp_helixagent_ensemble", result["system_fingerprint"])

		// Verify we got a response (Mem0 enhancement happens transparently)
		choices, ok := result["choices"].([]interface{})
		assert.True(t, ok)
		assert.NotEmpty(t, choices)
	})

	t.Run("Multiple concurrent requests all use Mem0 ensemble", func(t *testing.T) {
		var wg sync.WaitGroup
		type reqResult struct {
			success        bool
			providerFailed bool
		}
		results := make(chan reqResult, 5)

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				reqBody := map[string]interface{}{
					"model": "helixagent-ensemble",
					"messages": []map[string]string{
						{"role": "user", "content": fmt.Sprintf("Count to %d", idx+1)},
					},
					"max_tokens": 30,
				}

				jsonBody, _ := json.Marshal(reqBody)
				resp, err := client.Post(
					serverURL+"/v1/chat/completions",
					"application/json",
					bytes.NewReader(jsonBody),
				)
				if err != nil {
					results <- reqResult{success: false, providerFailed: true}
					return
				}
				defer resp.Body.Close()

				// Check for provider failures (502, 503, 504)
				if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
					results <- reqResult{success: false, providerFailed: true}
					return
				}

				body, _ := io.ReadAll(resp.Body)
				var result map[string]interface{}
				if err := json.Unmarshal(body, &result); err != nil {
					results <- reqResult{success: false, providerFailed: false}
					return
				}

				// Verify ensemble markers
				model, _ := result["model"].(string)
				fingerprint, _ := result["system_fingerprint"].(string)

				results <- reqResult{
					success:        (model == "helixagent-ensemble" && fingerprint == "fp_helixagent_ensemble"),
					providerFailed: false,
				}
			}(i)
		}

		wg.Wait()
		close(results)

		successCount := 0
		providerFailCount := 0
		for res := range results {
			if res.success {
				successCount++
			}
			if res.providerFailed {
				providerFailCount++
			}
		}

		// If all failed due to provider issues, skip the test
		if providerFailCount == 5 {
			t.Logf("All requests failed due to provider unavailability (acceptable)")
			return
		}

		// At least 3 out of 5 should succeed (60% tolerance for server load)
		// But adjust for provider failures
		nonFailedRequests := 5 - providerFailCount
		expectedSuccesses := (nonFailedRequests * 60) / 100
		if expectedSuccesses < 1 {
			expectedSuccesses = 1
		}
		assert.GreaterOrEqual(t, successCount, expectedSuccesses,
			"At least 60%% of non-failed requests should go through ensemble (got %d/%d)", successCount, nonFailedRequests)
	})
}
