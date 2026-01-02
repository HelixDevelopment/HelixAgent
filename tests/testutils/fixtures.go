package testutils

import (
	"time"

	"github.com/superagent/superagent/internal/models"
)

// Test fixtures for common test data

// Providers
var (
	TestProviderNames = []string{"claude", "deepseek", "gemini", "qwen", "ollama"}

	TestProviderCapabilities = &models.ProviderCapabilities{
		SupportedModels:         []string{"test-model-1", "test-model-2"},
		SupportedFeatures:       []string{"streaming", "function_calling"},
		SupportedRequestTypes:   []string{"code_generation", "reasoning", "creative"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: true,
		SupportsVision:          false,
		Limits: models.ModelLimits{
			MaxTokens:             4096,
			MaxInputLength:        2048,
			MaxOutputLength:       2048,
			MaxConcurrentRequests: 10,
		},
		Metadata: map[string]string{
			"version": "1.0.0",
			"type":    "test",
		},
	}
)

// Users
var (
	TestUser = &models.User{
		ID:        "test-user-1",
		Email:     "test@example.com",
		Role:      "user",
		CreatedAt: time.Now(),
	}

	TestAdminUser = &models.User{
		ID:        "test-admin-1",
		Email:     "admin@example.com",
		Role:      "admin",
		CreatedAt: time.Now(),
	}
)

// Requests and Responses
func NewTestCompletionRequest() map[string]interface{} {
	return map[string]interface{}{
		"prompt":      "Test prompt for completion",
		"model":       "test-model",
		"max_tokens":  100,
		"temperature": 0.7,
	}
}

func NewTestChatCompletionRequest() map[string]interface{} {
	return map[string]interface{}{
		"model": "test-model",
		"messages": []map[string]string{
			{"role": "system", "content": "You are a helpful assistant."},
			{"role": "user", "content": "Hello, how are you?"},
		},
		"max_tokens":  100,
		"temperature": 0.7,
	}
}

func NewTestEnsembleRequest() map[string]interface{} {
	return map[string]interface{}{
		"prompt":      "Test ensemble prompt",
		"model":       "ensemble-model",
		"max_tokens":  100,
		"temperature": 0.7,
		"ensemble_config": map[string]interface{}{
			"strategy":             "confidence_weighted",
			"min_providers":        2,
			"confidence_threshold": 0.8,
			"fallback_to_best":     true,
		},
	}
}

func NewTestLLMRequest(prompt string) *models.LLMRequest {
	return &models.LLMRequest{
		ID:        "test-request-" + time.Now().Format("20060102150405"),
		SessionID: "test-session",
		UserID:    "test-user",
		Prompt:    prompt,
		ModelParams: models.ModelParameters{
			Model:       "test-model",
			Temperature: 0.7,
			MaxTokens:   100,
			TopP:        0.9,
		},
		Status:      "pending",
		CreatedAt:   time.Now(),
		RequestType: "completion",
	}
}

func NewTestLLMResponse(requestID, content string) *models.LLMResponse {
	return &models.LLMResponse{
		ID:             "response-" + requestID,
		RequestID:      requestID,
		ProviderID:     "test-provider",
		ProviderName:   "test",
		Content:        content,
		Confidence:     0.9,
		TokensUsed:     50,
		ResponseTime:   100,
		FinishReason:   "stop",
		Selected:       true,
		SelectionScore: 0.9,
		CreatedAt:      time.Now(),
	}
}

// Auth fixtures
func NewTestRegistrationRequest() map[string]interface{} {
	return map[string]interface{}{
		"email":    "newuser@example.com",
		"password": "SecurePassword123!",
	}
}

func NewTestLoginRequest() map[string]interface{} {
	return map[string]interface{}{
		"email":    "test@example.com",
		"password": "testpassword",
	}
}

// Token fixtures
const (
	TestValidToken   = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0LXVzZXItMSIsImVtYWlsIjoidGVzdEBleGFtcGxlLmNvbSIsInJvbGUiOiJ1c2VyIiwiZXhwIjoxOTk5OTk5OTk5fQ.test"
	TestExpiredToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0LXVzZXItMSIsImVtYWlsIjoidGVzdEBleGFtcGxlLmNvbSIsInJvbGUiOiJ1c2VyIiwiZXhwIjoxMDAwMDAwMDAwfQ.test"
	TestInvalidToken = "invalid-token"
	TestAPIKey       = "test-api-key-12345"
)

// Embedding fixtures
func NewTestEmbeddingRequest() map[string]interface{} {
	return map[string]interface{}{
		"input": "Test text for embedding generation",
		"model": "test-embedding-model",
	}
}

func NewTestEmbeddingResponse() []float64 {
	// Return a small test embedding vector
	return []float64{0.1, 0.2, 0.3, 0.4, 0.5, -0.1, -0.2, -0.3}
}

// Cognee fixtures
func NewTestCogneeAddRequest() map[string]interface{} {
	return map[string]interface{}{
		"dataset_name": "test-dataset",
		"content":      "Test content for Cognee processing",
	}
}

func NewTestCogneeSearchRequest() map[string]interface{} {
	return map[string]interface{}{
		"dataset_name": "test-dataset",
		"query":        "test query",
		"limit":        10,
	}
}

// MCP fixtures
func NewTestMCPToolCall() map[string]interface{} {
	return map[string]interface{}{
		"name": "test_tool",
		"arguments": map[string]interface{}{
			"param1": "value1",
			"param2": 42,
		},
	}
}

func NewTestMCPCapabilities() map[string]interface{} {
	return map[string]interface{}{
		"version":   "1.0",
		"providers": []string{"claude", "deepseek"},
		"tools": []map[string]interface{}{
			{
				"name":        "test_tool",
				"description": "A test tool",
			},
		},
	}
}

// Session fixtures
func NewTestUserSession() *models.UserSession {
	return &models.UserSession{
		ID:           "test-session-1",
		UserID:       "test-user-1",
		SessionToken: "test-token-123",
		Status:       "active",
		RequestCount: 0,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(time.Hour * 24),
		LastActivity: time.Now(),
		Context:      map[string]interface{}{},
	}
}

// Health check fixtures
func NewTestHealthResponse() map[string]interface{} {
	return map[string]interface{}{
		"status": "healthy",
		"providers": map[string]interface{}{
			"total":     5,
			"healthy":   4,
			"unhealthy": 1,
		},
		"timestamp": time.Now().Unix(),
	}
}

func NewTestProviderHealth() map[string]error {
	return map[string]error{
		"claude":   nil,
		"deepseek": nil,
		"gemini":   nil,
		"qwen":     nil,
		"ollama":   nil,
	}
}

// EnsembleConfig fixtures
func NewTestEnsembleConfig() *models.EnsembleConfig {
	return &models.EnsembleConfig{
		Strategy:            "confidence_weighted",
		MinProviders:        2,
		ConfidenceThreshold: 0.8,
		FallbackToBest:      true,
		Timeout:             30,
		PreferredProviders:  []string{"claude", "deepseek"},
	}
}

// Messages fixtures
func NewTestMessages(userMessage string) []models.Message {
	return []models.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: userMessage},
	}
}

// Provider fixtures
func NewTestLLMProvider(name string) *models.LLMProvider {
	return &models.LLMProvider{
		ID:           "provider-" + name,
		Name:         name,
		Type:         "api",
		BaseURL:      "https://api." + name + ".ai/v1",
		Model:        name + "-model",
		Weight:       1.0,
		Enabled:      true,
		HealthStatus: "healthy",
		ResponseTime: 100,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}
