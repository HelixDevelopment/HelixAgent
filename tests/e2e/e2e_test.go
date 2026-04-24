// Package e2e contains the end-to-end test suite. CONST-030 mandates all
// non-unit tests execute against a live HelixAgent on :8100 — any
// in-process mock is a violation. The in-process `TestE2ENewServicesWorkflow`
// subtree that used a local `MockTool` wired directly into
// `services.NewMCPManager` / `LSPClient` / `ContextManager` /
// `IntegrationOrchestrator` was demoted to
// `tests/unit/e2e_services_legacy/` in PR23 of the CONST-030 campaign.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2EUserWorkflow tests complete user workflows against the live HelixAgent.
// Requires a running server on localhost:8100. Probe + skip is done by
// `testutil.RequireServer(t)`.
func TestE2EUserWorkflow(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := testutil.ServerURL()
	client := &http.Client{Timeout: 60 * time.Second}

	t.Run("CompleteChatWorkflow", func(t *testing.T) {
		// Step 1: Check available models
		resp, err := client.Get(baseURL + "/v1/models")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var modelsResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&modelsResp)
		require.NoError(t, err)

		data := modelsResp["data"].([]interface{})
		assert.Greater(t, len(data), 0, "Should have available models")

		// Step 2: Start a chat conversation
		chatRequest := map[string]interface{}{
			"model": "gpt-3.5-turbo",
			"messages": []map[string]interface{}{
				{"role": "system", "content": "You are a helpful assistant."},
				{"role": "user", "content": "Hello! Can you help me with something?"},
			},
			"max_tokens":  100,
			"temperature": 0.7,
		}

		jsonData, err := json.Marshal(chatRequest)
		require.NoError(t, err)

		resp, err = client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var chatResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&chatResp)
			require.NoError(t, err)

			assert.Equal(t, "chat.completion", chatResp["object"])
			assert.NotNil(t, chatResp["choices"])

			choices := chatResp["choices"].([]interface{})
			assert.Greater(t, len(choices), 0)
		} else {
			t.Logf("chat workflow returned status %d (may be expected if providers not configured)", resp.StatusCode)
		}
	})

	t.Run("CompleteEnsembleWorkflow", func(t *testing.T) {
		// Step 1: Check provider health
		resp, err := client.Get(baseURL + "/v1/providers")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var providersResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&providersResp)
		require.NoError(t, err)

		providers, _ := providersResp["providers"].([]interface{})
		_ = providers

		// Step 2: Test ensemble completion
		ensembleRequest := map[string]interface{}{
			"prompt": "What is the capital of France?",
			"ensemble_config": map[string]interface{}{
				"strategy":             "confidence_weighted",
				"min_providers":        1,
				"confidence_threshold": 0.5,
			},
		}

		jsonData, err := json.Marshal(ensembleRequest)
		require.NoError(t, err)

		resp, err = client.Post(baseURL+"/v1/ensemble/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var ensembleResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&ensembleResp)
			require.NoError(t, err)

			assert.Equal(t, "ensemble.completion", ensembleResp["object"])
			assert.NotNil(t, ensembleResp["ensemble"])
		} else {
			t.Logf("ensemble workflow returned status %d", resp.StatusCode)
		}
	})

	t.Run("CompleteStreamingWorkflow", func(t *testing.T) {
		streamRequest := map[string]interface{}{
			"prompt":      "Count from 1 to 5",
			"model":       "gpt-3.5-turbo",
			"max_tokens":  50,
			"temperature": 0.1,
			"stream":      true,
		}

		jsonData, err := json.Marshal(streamRequest)
		require.NoError(t, err)

		resp, err := client.Post(baseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			// Read streaming response
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			// Should contain SSE data
			assert.Contains(t, string(body), "data:")
		} else {
			t.Logf("streaming workflow returned status %d", resp.StatusCode)
		}
	})

	t.Run("CompleteMonitoringWorkflow", func(t *testing.T) {
		// Step 1: Check basic health
		resp, err := client.Get(baseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Step 2: Check enhanced health
		resp, err = client.Get(baseURL + "/v1/health")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var healthResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&healthResp)
		require.NoError(t, err)

		assert.Equal(t, "healthy", healthResp["status"])
		assert.NotNil(t, healthResp["providers"])

		// Step 3: Check metrics
		resp, err = client.Get(baseURL + "/metrics")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Should contain Prometheus metrics
		assert.Contains(t, string(body), "# HELP")
		assert.Contains(t, string(body), "# TYPE")
	})
}

// TestE2EErrorHandling tests error scenarios end-to-end against :8100.
func TestE2EErrorHandling(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := testutil.ServerURL()
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("InvalidEndpoint", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/invalid/endpoint")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("InvalidRequestBody", func(t *testing.T) {
		resp, err := client.Post(baseURL+"/v1/completions", "application/json", bytes.NewBuffer([]byte("invalid json")))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("MissingRequiredFields", func(t *testing.T) {
		request := map[string]interface{}{
			"temperature": 0.5,
			// Missing required fields like prompt/model
		}

		jsonData, _ := json.Marshal(request)
		resp, err := client.Post(baseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("UnsupportedModel", func(t *testing.T) {
		request := map[string]interface{}{
			"prompt": "Hello",
			"model":  "unsupported-model-name",
		}

		jsonData, _ := json.Marshal(request)
		resp, err := client.Post(baseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return error (400 or 500 depending on implementation)
		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	})
}

// TestE2EPerformance tests performance characteristics against :8100.
func TestE2EPerformance(t *testing.T) {
	testutil.RequireServer(t)

	baseURL := testutil.ServerURL()
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("ConcurrentRequests", func(t *testing.T) {
		concurrency := 10
		responses := make(chan time.Duration, concurrency)

		// Launch concurrent requests
		for i := 0; i < concurrency; i++ {
			go func(id int) {
				start := time.Now()

				request := map[string]interface{}{
					"prompt":      fmt.Sprintf("Test request %d", id),
					"model":       "gpt-3.5-turbo",
					"max_tokens":  10,
					"temperature": 0.1,
				}

				jsonData, _ := json.Marshal(request)
				resp, err := client.Post(baseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))

				if resp != nil {
					resp.Body.Close()
				}

				if err == nil {
					responses <- time.Since(start)
				} else {
					responses <- 0
				}
			}(i)
		}

		// Collect responses
		var totalDuration time.Duration
		successCount := 0

		for i := 0; i < concurrency; i++ {
			duration := <-responses
			if duration > 0 {
				totalDuration += duration
				successCount++
			}
		}

		if successCount > 0 {
			avgDuration := totalDuration / time.Duration(successCount)
			// Performance assertion - should respond within reasonable time
			assert.Less(t, avgDuration, 30*time.Second, "Average response time should be reasonable")
			_ = avgDuration
		}
	})
}
