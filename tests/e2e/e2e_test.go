package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2EUserWorkflow tests complete user workflows
func TestE2EUserWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	baseURL := "http://localhost:8080"
	client := &http.Client{Timeout: 60 * time.Second}

	// Wait for server to be ready
	require.Eventually(t, func() bool {
		resp, err := client.Get(baseURL + "/health")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 30*time.Second, 2*time.Second, "Server should be ready")

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

		// Accept various status codes since providers might not be configured
		if resp.StatusCode == http.StatusOK {
			var chatResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&chatResp)
			require.NoError(t, err)

			assert.Equal(t, "chat.completion", chatResp["object"])
			assert.NotNil(t, chatResp["choices"])

			choices := chatResp["choices"].([]interface{})
			assert.Greater(t, len(choices), 0)

			t.Logf("✅ Chat workflow completed successfully")
		} else {
			t.Logf("⚠️  Chat workflow returned status %d (may be expected if providers not configured)", resp.StatusCode)
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

		providers := providersResp["providers"].([]interface{})
		t.Logf("✅ Found %d providers", len(providers))

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

			t.Logf("✅ Ensemble workflow completed successfully")
		} else {
			t.Logf("⚠️  Ensemble workflow returned status %d", resp.StatusCode)
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
			t.Logf("✅ Streaming workflow completed: received %d bytes", len(body))
		} else {
			t.Logf("⚠️  Streaming workflow returned status %d", resp.StatusCode)
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

		t.Logf("✅ Monitoring workflow completed: metrics size %d bytes", len(body))
	})
}

// TestE2EErrorHandling tests error scenarios end-to-end
func TestE2EErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E error handling test in short mode")
	}

	baseURL := "http://localhost:8080"
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

// TestE2EPerformance tests performance characteristics
func TestE2EPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E performance test in short mode")
	}

	baseURL := "http://localhost:8080"
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
			t.Logf("✅ Concurrent requests: %d/%d successful, avg duration: %v",
				successCount, concurrency, avgDuration)

			// Performance assertion - should respond within reasonable time
			assert.Less(t, avgDuration, 30*time.Second, "Average response time should be reasonable")
		} else {
			t.Logf("⚠️  No concurrent requests succeeded (may be expected if providers not configured)")
		}
	})
}
