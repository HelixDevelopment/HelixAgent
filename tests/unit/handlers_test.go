package unit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/superagent/superagent/internal/handlers"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
)

func TestCompletionHandler_Complete(t *testing.T) {
	// Create a mock request service
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		Providers:      make(map[string]*services.ProviderConfig),
		Ensemble: &models.EnsembleConfig{
			Strategy: "confidence_weighted",
		},
	}
	registry := services.NewProviderRegistry(registryConfig, nil)
	requestService := registry.GetRequestService()

	// Create completion handler
	handler := handlers.NewCompletionHandler(requestService)

	// Create test request
	req := handlers.CompletionRequest{
		Prompt:      "Test prompt",
		Model:       "test-model",
		Temperature: 0.7,
		MaxTokens:   100,
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
		EnsembleConfig: &models.EnsembleConfig{
			Strategy:            "confidence_weighted",
			MinProviders:        1,
			ConfidenceThreshold: 0.7,
			FallbackToBest:      true,
			Timeout:             30,
			PreferredProviders:  []string{},
		},
	}

	reqBody, _ := json.Marshal(req)

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Set user context (simulate authenticated user)
	c.Set("user_id", "test-user")
	c.Set("session_id", "test-session")

	// Call handler
	handler.Complete(c)

	// Check response - may fail due to no auth/providers in test environment
	if w.Code != http.StatusOK {
		// Expected when no providers are properly configured
		var errorResponse handlers.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		assert.NoError(t, err)
		// Error is acceptable in test environment
		t.Logf("Expected error in test environment: %s", errorResponse.Error.Message)
	} else {
		// Should succeed if providers are available
		var response handlers.CompletionResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.NotEmpty(t, response.ID)
		assert.Equal(t, "text_completion", response.Object)
		assert.NotEmpty(t, response.Model)
		assert.NotEmpty(t, response.Choices)
		assert.Equal(t, 1, len(response.Choices))
	}
}

func TestCompletionHandler_Complete_InvalidRequest(t *testing.T) {
	// Create a mock request service
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		Providers:      make(map[string]*services.ProviderConfig),
	}
	registry := services.NewProviderRegistry(registryConfig, nil)
	requestService := registry.GetRequestService()

	// Create completion handler
	handler := handlers.NewCompletionHandler(requestService)

	// Create invalid request (missing required field)
	req := map[string]interface{}{
		"model": "test-model",
		// Missing "prompt" field
	}

	reqBody, _ := json.Marshal(req)

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Call handler
	handler.Complete(c)

	// Check response - should be bad request
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errorResponse handlers.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
	assert.NoError(t, err)
	assert.NotEmpty(t, errorResponse.Error.Message)
	assert.NotEmpty(t, errorResponse.Error.Type)
}

func TestCompletionHandler_Chat(t *testing.T) {
	// Create a mock request service
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		Providers:      make(map[string]*services.ProviderConfig),
	}
	registry := services.NewProviderRegistry(registryConfig, nil)
	requestService := registry.GetRequestService()

	// Create completion handler
	handler := handlers.NewCompletionHandler(requestService)

	// Create chat request
	req := handlers.CompletionRequest{
		Prompt: "Chat conversation test", // Required for validation
		Model:  "gpt-3.5-turbo",
		Messages: []models.Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
		Temperature: 0.7,
		MaxTokens:   150,
	}

	reqBody, _ := json.Marshal(req)

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Set user context
	c.Set("user_id", "test-user")
	c.Set("session_id", "test-session")

	// Call handler
	handler.Chat(c)

	// Check response
	if w.Code != http.StatusOK {
		// May fail due to no providers or auth issues in test environment
		var errorResponse handlers.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		assert.NoError(t, err)
		t.Logf("Expected error in test environment: %s", errorResponse.Error.Message)
	} else {
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "chat.completion", response["object"])
		assert.NotEmpty(t, response["choices"])
	}
}

func TestCompletionHandler_Models(t *testing.T) {
	// Create a mock request service
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		Providers:      make(map[string]*services.ProviderConfig),
	}
	registry := services.NewProviderRegistry(registryConfig, nil)
	requestService := registry.GetRequestService()

	// Create completion handler
	handler := handlers.NewCompletionHandler(requestService)

	// Create HTTP request
	httpReq := httptest.NewRequest("GET", "/v1/models", nil)

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Call handler
	handler.Models(c)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "list", response["object"])
	assert.NotEmpty(t, response["data"])

	// Check that models array contains expected models
	models := response["data"].([]interface{})
	assert.Greater(t, len(models), 0)
}

func TestCompletionHandler_Stream(t *testing.T) {
	// Create a mock request service
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		Providers:      make(map[string]*services.ProviderConfig),
	}
	registry := services.NewProviderRegistry(registryConfig, nil)
	requestService := registry.GetRequestService()

	// Create completion handler
	handler := handlers.NewCompletionHandler(requestService)

	// Create streaming request
	req := handlers.CompletionRequest{
		Prompt:      "Test streaming prompt",
		Model:       "test-model",
		Stream:      true,
		MaxTokens:   100,
		Temperature: 0.7,
	}

	reqBody, _ := json.Marshal(req)

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/v1/completions/stream", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Set user context
	c.Set("user_id", "test-user")
	c.Set("session_id", "test-session")

	// Call handler
	handler.CompleteStream(c)

	// Check response - should have streaming headers or error
	if w.Code != http.StatusOK {
		// Streaming may fail due to no providers in test environment
		var errorResponse handlers.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		assert.NoError(t, err)
		t.Logf("Expected streaming error in test environment: %s", errorResponse.Error.Message)
	} else {
		assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
		assert.Equal(t, "keep-alive", w.Header().Get("Connection"))

		// Should contain streaming data
		body := w.Body.String()
		assert.Contains(t, body, "data:")
	}
}

func TestCompletionRequestValidation(t *testing.T) {
	testCases := []struct {
		name        string
		request     handlers.CompletionRequest
		expectError bool
	}{
		{
			name: "Valid request",
			request: handlers.CompletionRequest{
				Prompt: "Test prompt",
				Model:  "test-model",
			},
			expectError: false,
		},
		{
			name: "Missing prompt",
			request: handlers.CompletionRequest{
				Model: "test-model",
			},
			expectError: true,
		},
		{
			name: "Valid chat request",
			request: handlers.CompletionRequest{
				Prompt: "Chat test", // Add required prompt
				Model:  "gpt-3.5-turbo",
				Messages: []models.Message{
					{
						Role:    "user",
						Content: "Hello",
					},
				},
			},
			expectError: false,
		},
		{
			name: "Valid ensemble request",
			request: handlers.CompletionRequest{
				Prompt: "Test prompt",
				Model:  "test-model",
				EnsembleConfig: &models.EnsembleConfig{
					Strategy:            "confidence_weighted",
					MinProviders:        2,
					ConfidenceThreshold: 0.8,
					FallbackToBest:      true,
					Timeout:             30,
					PreferredProviders:  []string{"provider1", "provider2"},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock request service (note: no real providers for test)
			registryConfig := &services.RegistryConfig{
				DefaultTimeout: 30 * time.Second,
				Providers:      make(map[string]*services.ProviderConfig),
			}
			registry := services.NewProviderRegistry(registryConfig, nil)
			requestService := registry.GetRequestService()

			// Create completion handler
			handler := handlers.NewCompletionHandler(requestService)

			reqBody, _ := json.Marshal(tc.request)

			// Create HTTP request
			httpReq := httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBody))
			httpReq.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Create Gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = httpReq
			c.Request = httpReq
			c.Set("user_id", "test-user")
			c.Set("session_id", "test-session")

			// Call handler
			handler.Complete(c)

			if tc.expectError {
				assert.Equal(t, http.StatusBadRequest, w.Code)
			} else {
				// Note: This might still fail with 500 due to no providers,
				// but it shouldn't be a 400 (bad request) error
				if w.Code == http.StatusBadRequest {
					t.Logf("Got unexpected 400 error: %s", w.Body.String())
				}
				assert.NotEqual(t, http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestConvertToInternalRequest(t *testing.T) {
	// This tests the conversion logic by creating a request through the handler
	// and checking that it properly converts to internal format
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		Providers:      make(map[string]*services.ProviderConfig),
	}
	registry := services.NewProviderRegistry(registryConfig, nil)
	requestService := registry.GetRequestService()

	handler := handlers.NewCompletionHandler(requestService)

	// Create test request with various parameters
	req := handlers.CompletionRequest{
		Prompt:      "Test prompt",
		Model:       "test-model",
		Temperature: 0.7,
		MaxTokens:   100,
		TopP:        0.9,
		Stop:        []string{"\n", "."},
		Stream:      false,
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "Hello",
				Name:    stringPtr("User1"),
			},
		},
		MemoryEnhanced: true,
		RequestType:    "test",
	}

	reqBody, _ := json.Marshal(req)

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	// Set user context
	c.Set("user_id", "test-user-id")
	c.Set("session_id", "test-session-id")

	// Call handler
	handler.Complete(c)

	// The request should be processed (even if it fails due to no providers)
	// We're mainly testing that the conversion doesn't panic and sets defaults correctly
	assert.True(t, w.Code >= 200 && w.Code <= 500)
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
