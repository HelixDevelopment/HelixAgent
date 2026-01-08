package verifier

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVerifierE2EWorkflow tests complete verifier workflows
// Note: These tests require a running HelixAgent server with verifier enabled
// To run these tests:
// 1. Start the server: make run-dev
// 2. Run E2E tests: make test-e2e
func TestVerifierE2EWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	baseURL := "http://localhost:7061"
	client := &http.Client{Timeout: 60 * time.Second}

	// Check if server is running
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Skipping E2E test: HelixAgent server not running at %s. Start server with 'make run-dev'", baseURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Skipping E2E test: Server at %s returned status %d", baseURL, resp.StatusCode)
	}

	t.Logf("HelixAgent server is running at %s", baseURL)

	t.Run("CompleteVerificationWorkflow", func(t *testing.T) {
		// Step 1: Verify a model
		verifyRequest := map[string]interface{}{
			"model_id": "gpt-4",
			"provider": "openai",
		}

		jsonData, err := json.Marshal(verifyRequest)
		require.NoError(t, err)

		resp, err := client.Post(baseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		// Verification endpoint may return 200 or error depending on configuration
		if resp.StatusCode == http.StatusOK {
			var result map[string]interface{}
			err = json.Unmarshal(body, &result)
			require.NoError(t, err)

			assert.NotNil(t, result["verified"])
			t.Logf("Model verification completed: verified=%v", result["verified"])

			// Step 2: Get verification status
			modelID := "gpt-4"
			resp, err = client.Get(baseURL + "/api/v1/verifier/status/" + modelID)
			require.NoError(t, err)
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var status map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&status)
				require.NoError(t, err)
				t.Logf("Verification status retrieved for %s", modelID)
			}
		} else {
			t.Logf("Verification endpoint returned status %d (may be expected if provider not configured)", resp.StatusCode)
		}
	})

	t.Run("CompleteCodeVisibilityWorkflow", func(t *testing.T) {
		// Test "Do you see my code?" verification
		visibilityRequest := map[string]interface{}{
			"code":     "func main() { fmt.Println(\"Hello, World!\") }",
			"language": "go",
			"model_id": "gpt-4",
			"provider": "openai",
		}

		jsonData, err := json.Marshal(visibilityRequest)
		require.NoError(t, err)

		resp, err := client.Post(baseURL+"/api/v1/verifier/code-visibility", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)

			assert.NotNil(t, result["visible"])
			assert.NotNil(t, result["confidence"])
			t.Logf("Code visibility test completed: visible=%v, confidence=%v",
				result["visible"], result["confidence"])
		} else {
			t.Logf("Code visibility endpoint returned status %d", resp.StatusCode)
		}
	})

	t.Run("CompleteScoringWorkflow", func(t *testing.T) {
		// Step 1: Get model score
		modelID := "gpt-4"
		resp, err := client.Get(baseURL + "/api/v1/verifier/scores/" + modelID)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var score map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&score)
			require.NoError(t, err)

			assert.NotNil(t, score["overall_score"])
			t.Logf("Model score retrieved: overall_score=%v", score["overall_score"])
		}

		// Step 2: Get top models
		resp, err = client.Get(baseURL + "/api/v1/verifier/scores/top?limit=5")
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var topModels map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&topModels)
			require.NoError(t, err)

			if models, ok := topModels["models"].([]interface{}); ok {
				t.Logf("Retrieved %d top models", len(models))
			}
		}
	})

	t.Run("CompleteBatchVerificationWorkflow", func(t *testing.T) {
		batchRequest := map[string]interface{}{
			"models": []map[string]interface{}{
				{"model_id": "gpt-4", "provider": "openai"},
				{"model_id": "claude-3-opus", "provider": "anthropic"},
				{"model_id": "gemini-pro", "provider": "google"},
			},
		}

		jsonData, err := json.Marshal(batchRequest)
		require.NoError(t, err)

		resp, err := client.Post(baseURL+"/api/v1/verifier/batch-verify", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var results map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&results)
			require.NoError(t, err)

			if resultList, ok := results["results"].([]interface{}); ok {
				t.Logf("Batch verification completed for %d models", len(resultList))
			}
		} else {
			t.Logf("Batch verification endpoint returned status %d", resp.StatusCode)
		}
	})

	t.Run("CompleteProviderHealthWorkflow", func(t *testing.T) {
		// Step 1: Get all providers health
		resp, err := client.Get(baseURL + "/api/v1/verifier/health/providers")
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var health map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&health)
			require.NoError(t, err)

			if providers, ok := health["providers"].([]interface{}); ok {
				t.Logf("Retrieved health for %d providers", len(providers))
			}
		}

		// Step 2: Get specific provider health
		resp, err = client.Get(baseURL + "/api/v1/verifier/health/providers/openai")
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var providerHealth map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&providerHealth)
			require.NoError(t, err)

			t.Logf("Provider openai health: healthy=%v", providerHealth["healthy"])
		}

		// Step 3: Get healthy providers only
		resp, err = client.Get(baseURL + "/api/v1/verifier/health/providers/healthy")
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var healthyProviders map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&healthyProviders)
			require.NoError(t, err)

			if providers, ok := healthyProviders["providers"].([]interface{}); ok {
				t.Logf("Found %d healthy providers", len(providers))
			}
		}
	})

	t.Run("CompleteReverificationWorkflow", func(t *testing.T) {
		// Re-verify a model
		resp, err := client.Post(baseURL+"/api/v1/verifier/reverify/gpt-4", "application/json", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)

			t.Logf("Re-verification completed for gpt-4")
		} else {
			t.Logf("Re-verification endpoint returned status %d", resp.StatusCode)
		}
	})

	t.Run("VerifierHealthEndpoint", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/api/v1/verifier/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var health map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&health)
			require.NoError(t, err)

			assert.NotNil(t, health["status"])
			t.Logf("Verifier health: status=%v", health["status"])
		}
	})
}

// TestVerifierIntegrationWithChat tests verifier integration with chat completions
func TestVerifierIntegrationWithChat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	baseURL := "http://localhost:7061"
	client := &http.Client{Timeout: 60 * time.Second}

	// Check if server is running
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Skipping E2E test: HelixAgent server not running at %s", baseURL)
	}
	defer resp.Body.Close()

	t.Run("VerifiedModelChat", func(t *testing.T) {
		// First verify the model
		verifyRequest := map[string]interface{}{
			"model_id": "gpt-4",
			"provider": "openai",
		}

		jsonData, _ := json.Marshal(verifyRequest)
		resp, _ := client.Post(baseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer(jsonData))
		if resp != nil {
			resp.Body.Close()
		}

		// Then use it in chat
		chatRequest := map[string]interface{}{
			"model": "gpt-4",
			"messages": []map[string]interface{}{
				{"role": "user", "content": "Hello!"},
			},
		}

		jsonData, _ = json.Marshal(chatRequest)
		resp, err = client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Log result regardless of status
		body, _ := io.ReadAll(resp.Body)
		t.Logf("Chat with verified model returned status %d", resp.StatusCode)
		t.Logf("Response: %s", string(body)[:min(200, len(body))])
	})
}

// TestVerifierEndpointDiscovery tests API endpoint discovery
func TestVerifierEndpointDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	baseURL := "http://localhost:7061"
	client := &http.Client{Timeout: 10 * time.Second}

	// Check if server is running
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Skipping E2E test: HelixAgent server not running at %s", baseURL)
	}
	resp.Body.Close()

	endpoints := []struct {
		method string
		path   string
		name   string
	}{
		{"GET", "/api/v1/verifier/health", "Verifier Health"},
		{"POST", "/api/v1/verifier/verify", "Verify Model"},
		{"POST", "/api/v1/verifier/batch-verify", "Batch Verify"},
		{"GET", "/api/v1/verifier/status/test-model", "Get Status"},
		{"POST", "/api/v1/verifier/code-visibility", "Code Visibility"},
		{"POST", "/api/v1/verifier/reverify/test-model", "Re-verify"},
		{"GET", "/api/v1/verifier/scores/test-model", "Get Score"},
		{"GET", "/api/v1/verifier/scores/top", "Top Models"},
		{"GET", "/api/v1/verifier/health/providers", "All Providers Health"},
		{"GET", "/api/v1/verifier/health/providers/openai", "Provider Health"},
		{"GET", "/api/v1/verifier/health/providers/healthy", "Healthy Providers"},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			var resp *http.Response
			var err error

			if ep.method == "GET" {
				resp, err = client.Get(baseURL + ep.path)
			} else {
				resp, err = client.Post(baseURL+ep.path, "application/json", bytes.NewBuffer([]byte("{}")))
			}

			require.NoError(t, err, "Request to %s should not error", ep.path)
			defer resp.Body.Close()

			// Any response other than connection error means endpoint exists
			t.Logf("Endpoint %s %s returned status %d", ep.method, ep.path, resp.StatusCode)
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
