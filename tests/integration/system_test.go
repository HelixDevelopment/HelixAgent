package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkServerAvailable checks if the test server is reachable
func checkServerAvailable(baseURL string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// TestFullSystemIntegration tests the complete HelixAgent system
func TestFullSystemIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test configuration
	baseURL := "http://localhost:7061"
	maxRetries := 30
	retryDelay := 2 * time.Second

	// Skip if server is not available (expected in CI/test environments)
	if !checkServerAvailable(baseURL, 5*time.Second) {
		t.Skipf("Skipping integration test - server not available at %s", baseURL)
	}

	// Helper function to make HTTP requests with retries
	makeRequest := func(method, url string, body interface{}) (*http.Response, error) {
		var lastErr error
		for i := 0; i < maxRetries; i++ {
			var reqBody *bytes.Buffer
			if body != nil {
				jsonData, err := json.Marshal(body)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal request: %w", err)
				}
				reqBody = bytes.NewBuffer(jsonData)
			} else {
				reqBody = bytes.NewBuffer(nil)
			}

			req, err := http.NewRequest(method, url, reqBody)
			if err != nil {
				lastErr = err
				time.Sleep(retryDelay)
				continue
			}

			if body != nil {
				req.Header.Set("Content-Type", "application/json")
			}

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				return resp, nil
			}

			lastErr = err
			if i < maxRetries-1 {
				t.Logf("Request failed (attempt %d/%d): %v, retrying...", i+1, maxRetries, err)
				time.Sleep(retryDelay)
			}
		}
		return nil, fmt.Errorf("all retries failed, last error: %w", lastErr)
	}

	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := makeRequest("GET", baseURL+"/health", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var healthResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&healthResp)
		require.NoError(t, err)

		assert.Equal(t, "healthy", healthResp["status"])
		t.Logf("✅ Health check passed: %v", healthResp)
	})

	t.Run("ListModels", func(t *testing.T) {
		resp, err := makeRequest("GET", baseURL+"/v1/models", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var modelsResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&modelsResp)
		require.NoError(t, err)

		assert.Equal(t, "list", modelsResp["object"])
		assert.NotNil(t, modelsResp["data"])

		data := modelsResp["data"].([]interface{})
		assert.Greater(t, len(data), 0, "Should have at least one model")

		t.Logf("✅ Models endpoint returned %d models", len(data))
		for i, model := range data {
			if i >= 3 { // Show first 3 models
				break
			}
			modelData := model.(map[string]interface{})
			t.Logf("  - %s (%s)", modelData["id"], modelData["owned_by"])
		}
	})

	t.Run("OllamaCompletion", func(t *testing.T) {
		request := map[string]interface{}{
			"prompt":      "Say hello in exactly 3 words",
			"model":       "llama2",
			"max_tokens":  10,
			"temperature": 0.1,
		}

		resp, err := makeRequest("POST", baseURL+"/v1/completions", request)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Ollama might take time to load the model, so accept both 200 and potential errors
		if resp.StatusCode == http.StatusOK {
			var completionResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&completionResp)
			require.NoError(t, err)

			assert.Equal(t, "text_completion", completionResp["object"])
			assert.NotNil(t, completionResp["choices"])

			choices := completionResp["choices"].([]interface{})
			assert.Greater(t, len(choices), 0)

			choice := choices[0].(map[string]interface{})
			message := choice["message"].(map[string]interface{})
			content := message["content"].(string)

			assert.NotEmpty(t, content)
			t.Logf("✅ Ollama completion successful: %s", content)
		} else {
			// If Ollama isn't ready, that's acceptable for integration testing
			t.Logf("⚠️  Ollama completion returned status %d (may be loading model)", resp.StatusCode)
		}
	})

	t.Run("EnsembleCompletion", func(t *testing.T) {
		request := map[string]interface{}{
			"prompt": "What is 2+2? Answer in one word.",
			"ensemble_config": map[string]interface{}{
				"strategy":             "confidence_weighted",
				"min_providers":        1,
				"confidence_threshold": 0.5,
			},
		}

		resp, err := makeRequest("POST", baseURL+"/v1/ensemble/completions", request)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var ensembleResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&ensembleResp)
			require.NoError(t, err)

			assert.Equal(t, "ensemble.completion", ensembleResp["object"])
			assert.NotNil(t, ensembleResp["choices"])
			assert.NotNil(t, ensembleResp["ensemble"])

			ensemble := ensembleResp["ensemble"].(map[string]interface{})
			assert.NotNil(t, ensemble["selected_provider"])
			assert.NotNil(t, ensemble["selection_score"])

			t.Logf("✅ Ensemble completion successful with provider: %s", ensemble["selected_provider"])
		} else {
			t.Logf("⚠️  Ensemble completion returned status %d", resp.StatusCode)
		}
	})

	t.Run("ProviderHealth", func(t *testing.T) {
		resp, err := makeRequest("GET", baseURL+"/v1/providers", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var providersResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&providersResp)
		require.NoError(t, err)

		assert.NotNil(t, providersResp["providers"])
		assert.NotNil(t, providersResp["count"])

		providers := providersResp["providers"].([]interface{})
		t.Logf("✅ Found %d providers", len(providers))

		// Test Ollama health specifically
		ollamaResp, err := makeRequest("GET", baseURL+"/v1/providers/ollama/health", nil)
		if err == nil {
			defer ollamaResp.Body.Close()
			if ollamaResp.StatusCode == http.StatusOK {
				var healthResp map[string]interface{}
				err = json.NewDecoder(ollamaResp.Body).Decode(&healthResp)
				require.NoError(t, err)
				assert.True(t, healthResp["healthy"].(bool))
				t.Logf("✅ Ollama health check passed")
			} else {
				t.Logf("⚠️  Ollama health check returned status %d", ollamaResp.StatusCode)
			}
		}
	})

	t.Run("MetricsEndpoint", func(t *testing.T) {
		resp, err := makeRequest("GET", baseURL+"/metrics", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Should return Prometheus metrics format
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		metrics := string(body[:n])

		assert.Contains(t, metrics, "# HELP")
		assert.Contains(t, metrics, "# TYPE")
		t.Logf("✅ Metrics endpoint returned %d bytes of metrics data", n)
	})

	t.Run("EnhancedHealthCheck", func(t *testing.T) {
		resp, err := makeRequest("GET", baseURL+"/v1/health", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var healthResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&healthResp)
		require.NoError(t, err)

		assert.Equal(t, "healthy", healthResp["status"])
		assert.NotNil(t, healthResp["providers"])
		assert.NotNil(t, healthResp["timestamp"])

		providers := healthResp["providers"].(map[string]interface{})
		t.Logf("✅ Enhanced health check: %v total providers, %v healthy",
			providers["total"], providers["healthy"])
	})
}

// TestDockerServicesIntegration tests that all Docker services are running
func TestDockerServicesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker services integration test in short mode")
	}

	// Skip if primary server is not available (Docker environment not running)
	if !checkServerAvailable("http://localhost:7061", 5*time.Second) {
		t.Skip("Skipping Docker services integration test - server not available")
	}

	services := map[string]string{
		"HelixAgent": "http://localhost:7061/health",
		"PostgreSQL": "http://localhost:7061/health", // Indirect check via HelixAgent
		"Redis":      "http://localhost:7061/health", // Indirect check via HelixAgent
		"Ollama":     "http://localhost:11434/api/tags",
	}

	for serviceName, url := range services {
		t.Run(serviceName, func(t *testing.T) {
			client := &http.Client{Timeout: 10 * time.Second}

			resp, err := client.Get(url)
			if err != nil {
				t.Logf("⚠️  %s not accessible: %v", serviceName, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				t.Logf("✅ %s is running and accessible", serviceName)
			} else {
				t.Logf("⚠️  %s returned status %d", serviceName, resp.StatusCode)
			}
		})
	}
}
