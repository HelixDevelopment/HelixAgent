package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/handlers"
	"github.com/superagent/superagent/internal/services"
)

// TestAPIEndToEndScenarios tests complete API workflows
func TestAPIEndToEndScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping API integration test in short mode")
	}

	// Setup test server
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create services directly without provider registry
	ensembleService := services.NewEnsembleService("confidence_weighted", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensembleService, nil)

	// Register mock provider
	mockProvider := &MockProvider{
		name:          "api-test-provider",
		response:      "API test response from mock provider",
		shouldSucceed: true,
		confidence:    0.95,
	}
	requestService.RegisterProvider(mockProvider.name, mockProvider)

	// Setup handlers
	completionHandler := handlers.NewCompletionHandler(requestService)

	// Register routes
	router.POST("/v1/completions", completionHandler.Complete)
	router.POST("/v1/chat/completions", completionHandler.Chat)

	t.Run("Basic completion API", func(t *testing.T) {
		// Create request
		reqBody := map[string]interface{}{
			"model":       "test-model",
			"prompt":      "Test API completion",
			"max_tokens":  100,
			"temperature": 0.7,
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response, "choices")
		choices := response["choices"].([]interface{})
		assert.Greater(t, len(choices), 0)

		choice := choices[0].(map[string]interface{})
		assert.Contains(t, choice, "message")
		message := choice["message"].(map[string]interface{})
		assert.Contains(t, message, "content")
		assert.Equal(t, "API test response from mock provider", message["content"])
	})

	t.Run("Chat completion API", func(t *testing.T) {
		// Create request - chat completion still needs prompt field
		reqBody := map[string]interface{}{
			"model":  "test-model",
			"prompt": "Hello, how are you?",
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hello, how are you?",
				},
			},
			"max_tokens":  100,
			"temperature": 0.7,
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response, "choices")
		choices := response["choices"].([]interface{})
		assert.Greater(t, len(choices), 0)

		choice := choices[0].(map[string]interface{})
		assert.Contains(t, choice, "message")
		message := choice["message"].(map[string]interface{})
		assert.Contains(t, message, "content")
		assert.Equal(t, "API test response from mock provider", message["content"])
	})

	t.Run("Ensemble completion API", func(t *testing.T) {
		// Register multiple providers for ensemble
		providers := []*MockProvider{
			{
				name:          "ensemble-provider-1",
				response:      "Response from provider 1",
				shouldSucceed: true,
				confidence:    0.9,
			},
			{
				name:          "ensemble-provider-2",
				response:      "Response from provider 2",
				shouldSucceed: true,
				confidence:    0.8,
			},
		}

		for _, provider := range providers {
			requestService.RegisterProvider(provider.name, provider)
		}

		// Create request with ensemble config
		reqBody := map[string]interface{}{
			"model":       "ensemble-test",
			"prompt":      "Test ensemble completion",
			"max_tokens":  100,
			"temperature": 0.7,
			"ensemble_config": map[string]interface{}{
				"strategy":             "confidence_weighted",
				"min_providers":        2,
				"confidence_threshold": 0.5,
				"fallback_to_best":     true,
				"timeout":              5,
				"preferred_providers":  []string{"ensemble-provider-1", "ensemble-provider-2"},
			},
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response, "choices")
		choices := response["choices"].([]interface{})
		assert.Greater(t, len(choices), 0)

		choice := choices[0].(map[string]interface{})
		assert.Contains(t, choice, "message")
		message := choice["message"].(map[string]interface{})
		assert.Contains(t, message, "content")
		// Should get one of the provider responses
		content := message["content"].(string)
		assert.Contains(t, []string{
			"Response from provider 1",
			"Response from provider 2",
			"API test response from mock provider",
		}, content)
	})

	t.Run("Error handling - provider failure", func(t *testing.T) {
		// Create a new isolated request service for this test
		isolatedEnsembleService := services.NewEnsembleService("confidence_weighted", 30*time.Second)
		isolatedRequestService := services.NewRequestService("weighted", isolatedEnsembleService, nil)

		// Register ONLY failing provider
		failingProvider := &MockProvider{
			name:          "failing-provider",
			response:      "",
			shouldSucceed: false,
			errorMessage:  "Provider failed",
		}
		isolatedRequestService.RegisterProvider(failingProvider.name, failingProvider)

		// Create isolated handler
		isolatedHandler := handlers.NewCompletionHandler(isolatedRequestService)

		// Create isolated router
		isolatedRouter := gin.New()
		isolatedRouter.POST("/v1/completions", isolatedHandler.Complete)

		// Create request
		reqBody := map[string]interface{}{
			"model":      "failing-model",
			"prompt":     "Test error handling",
			"max_tokens": 100,
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		isolatedRouter.ServeHTTP(w, req)

		// Should return error response
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response, "error")
	})

	t.Run("Memory enhanced completion", func(t *testing.T) {
		// Create isolated services for memory test
		isolatedEnsembleService := services.NewEnsembleService("confidence_weighted", 30*time.Second)
		isolatedRequestService := services.NewRequestService("weighted", isolatedEnsembleService, nil)

		// Register only the mock provider for this test
		isolatedMockProvider := &MockProvider{
			name:          "memory-test-provider",
			response:      "API test response from mock provider",
			shouldSucceed: true,
			confidence:    0.95,
		}
		isolatedRequestService.RegisterProvider(isolatedMockProvider.name, isolatedMockProvider)

		// Create isolated handler
		isolatedHandler := handlers.NewCompletionHandler(isolatedRequestService)

		// Create isolated router
		isolatedRouter := gin.New()
		isolatedRouter.POST("/v1/completions", isolatedHandler.Complete)

		// Create request with memory enhancement
		reqBody := map[string]interface{}{
			"model":           "test-model",
			"prompt":          "Test memory enhanced completion",
			"max_tokens":      100,
			"memory_enhanced": true,
			"memory": map[string]interface{}{
				"user_id": "test-user-123",
				"context": "Previous conversation about testing",
			},
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		isolatedRouter.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response, "choices")
		choices := response["choices"].([]interface{})
		assert.Greater(t, len(choices), 0)

		choice := choices[0].(map[string]interface{})
		assert.Contains(t, choice, "message")
		message := choice["message"].(map[string]interface{})
		assert.Contains(t, message, "content")
		assert.Equal(t, "API test response from mock provider", message["content"])
	})
}
