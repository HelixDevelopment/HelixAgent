package models

import (
	"testing"
	"time"
)

// Helper functions for testing
func AssertNotNil(t *testing.T, value any) {
	t.Helper()
	if value == nil {
		t.Fatal("Expected non-nil value")
	}
}

func AssertEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("Expected %v, got %v", expected, actual)
	}
}

func AssertTrue(t *testing.T, condition bool) {
	t.Helper()
	if !condition {
		t.Fatal("Expected true, got false")
	}
}

func AssertFalse(t *testing.T, condition bool) {
	t.Helper()
	if condition {
		t.Fatal("Expected false, got true")
	}
}

func AssertNil(t *testing.T, value any) {
	t.Helper()
	// Handle typed nil pointers
	if value != nil {
		switch v := value.(type) {
		case *string:
			if v != nil {
				t.Fatalf("Expected nil value, got %v", value)
			}
		default:
			t.Fatalf("Expected nil value, got %v", value)
		}
	}
}

func AssertContains[T comparable](t *testing.T, slice []T, item T) {
	t.Helper()
	for _, v := range slice {
		if v == item {
			return
		}
	}
	t.Fatalf("Expected slice to contain %v, but it didn't", item)
}

func TestLLMRequest_Creation(t *testing.T) {
	// Test creating a valid LLM request
	req := &LLMRequest{
		ID:        "test-request-123",
		SessionID: "test-session-456",
		UserID:    "test-user-789",
		Prompt:    "Write a simple Go function that adds two numbers",
		Messages:  []Message{},
		ModelParams: ModelParameters{
			Model:            "test-model",
			Temperature:      0.7,
			MaxTokens:        1000,
			TopP:             1.0,
			StopSequences:    []string{},
			ProviderSpecific: map[string]any{},
		},
		EnsembleConfig: &EnsembleConfig{
			Strategy:            "confidence_weighted",
			MinProviders:        2,
			ConfidenceThreshold: 0.8,
			FallbackToBest:      true,
			Timeout:             30,
			PreferredProviders:  []string{"test-provider"},
		},
		MemoryEnhanced: false,
		Memory:         map[string]string{},
		Status:         "pending",
		CreatedAt:      time.Now(),
		RequestType:    "code_generation",
	}

	AssertNotNil(t, req)
	AssertEqual(t, "test-request-123", req.ID)
	AssertEqual(t, "test-session-456", req.SessionID)
	AssertEqual(t, "test-user-789", req.UserID)
	AssertEqual(t, "Write a simple Go function that adds two numbers", req.Prompt)
	AssertEqual(t, "test-model", req.ModelParams.Model)
	AssertEqual(t, 0.7, req.ModelParams.Temperature)
	AssertEqual(t, 1000, req.ModelParams.MaxTokens)
	AssertEqual(t, "confidence_weighted", req.EnsembleConfig.Strategy)
	AssertEqual(t, 2, req.EnsembleConfig.MinProviders)
	AssertFalse(t, req.MemoryEnhanced)
	AssertEqual(t, "pending", req.Status)
	AssertEqual(t, "code_generation", req.RequestType)
}

func TestLLMResponse_Creation(t *testing.T) {
	// Test creating a valid LLM response
	resp := &LLMResponse{
		ID:             "test-response-123",
		RequestID:      "test-request-123",
		ProviderID:     "test-provider",
		ProviderName:   "Test Provider",
		Content:        "func add(a, b int) int {\n    return a + b\n}",
		Confidence:     0.95,
		TokensUsed:     50,
		ResponseTime:   500,
		FinishReason:   "stop",
		Metadata:       map[string]any{},
		Selected:       true,
		SelectionScore: 0.95,
		CreatedAt:      time.Now(),
	}

	AssertNotNil(t, resp)
	AssertEqual(t, "test-response-123", resp.ID)
	AssertEqual(t, "test-request-123", resp.RequestID)
	AssertEqual(t, "test-provider", resp.ProviderID)
	AssertEqual(t, "Test Provider", resp.ProviderName)
	AssertEqual(t, "func add(a, b int) int {\n    return a + b\n}", resp.Content)
	AssertEqual(t, 0.95, resp.Confidence)
	AssertEqual(t, 50, resp.TokensUsed)
	AssertEqual(t, int64(500), resp.ResponseTime)
	AssertEqual(t, "stop", resp.FinishReason)
	AssertTrue(t, resp.Selected)
	AssertEqual(t, 0.95, resp.SelectionScore)
}

func TestLLMProvider_Creation(t *testing.T) {
	// Test creating a valid LLM provider
	provider := &LLMProvider{
		ID:           "test-provider-123",
		Name:         "Test Provider",
		Type:         "test",
		APIKey:       "test-api-key",
		BaseURL:      "https://api.test.com",
		Model:        "test-model",
		Weight:       1.0,
		Enabled:      true,
		Config:       map[string]any{},
		HealthStatus: "healthy",
		ResponseTime: 500,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	AssertNotNil(t, provider)
	AssertEqual(t, "test-provider-123", provider.ID)
	AssertEqual(t, "Test Provider", provider.Name)
	AssertEqual(t, "test", provider.Type)
	AssertEqual(t, "test-api-key", provider.APIKey)
	AssertEqual(t, "https://api.test.com", provider.BaseURL)
	AssertEqual(t, "test-model", provider.Model)
	AssertEqual(t, 1.0, provider.Weight)
	AssertTrue(t, provider.Enabled)
	AssertEqual(t, "healthy", provider.HealthStatus)
	AssertEqual(t, int64(500), provider.ResponseTime)
}

func TestModelParameters_Defaults(t *testing.T) {
	// Test ModelParameters with default values
	params := ModelParameters{
		Model:            "default-model",
		Temperature:      0.7,
		MaxTokens:        1000,
		TopP:             1.0,
		StopSequences:    []string{},
		ProviderSpecific: map[string]any{},
	}

	AssertEqual(t, "default-model", params.Model)
	AssertEqual(t, 0.7, params.Temperature)
	AssertEqual(t, 1000, params.MaxTokens)
	AssertEqual(t, 1.0, params.TopP)
	AssertEqual(t, 0, len(params.StopSequences))
	AssertEqual(t, 0, len(params.ProviderSpecific))
}

func TestEnsembleConfig_Validation(t *testing.T) {
	// Test EnsembleConfig validation
	config := EnsembleConfig{
		Strategy:            "confidence_weighted",
		MinProviders:        2,
		ConfidenceThreshold: 0.8,
		FallbackToBest:      true,
		Timeout:             30,
		PreferredProviders:  []string{"provider1", "provider2"},
	}

	AssertEqual(t, "confidence_weighted", config.Strategy)
	AssertEqual(t, 2, config.MinProviders)
	AssertEqual(t, 0.8, config.ConfidenceThreshold)
	AssertTrue(t, config.FallbackToBest)
	AssertEqual(t, 30, config.Timeout)
	AssertEqual(t, 2, len(config.PreferredProviders))
	AssertContains(t, config.PreferredProviders, "provider1")
	AssertContains(t, config.PreferredProviders, "provider2")
}

func TestMessage_Creation(t *testing.T) {
	// Test creating a message
	var name *string = nil
	message := Message{
		Role:      "user",
		Content:   "Hello, world!",
		Name:      name,
		ToolCalls: map[string]any{},
	}

	AssertEqual(t, "user", message.Role)
	AssertEqual(t, "Hello, world!", message.Content)
	AssertNil(t, message.Name)
	AssertEqual(t, 0, len(message.ToolCalls))
}

func TestUserSession_Creation(t *testing.T) {
	// Test creating a user session
	now := time.Now()
	var memoryID *string = nil
	session := UserSession{
		ID:           "session-123",
		UserID:       "user-456",
		SessionToken: "token-789",
		Context:      map[string]any{},
		MemoryID:     memoryID,
		Status:       "active",
		RequestCount: 5,
		LastActivity: now,
		ExpiresAt:    now.Add(24 * time.Hour),
		CreatedAt:    now,
	}

	AssertEqual(t, "session-123", session.ID)
	AssertEqual(t, "user-456", session.UserID)
	AssertEqual(t, "token-789", session.SessionToken)
	AssertEqual(t, 0, len(session.Context))
	AssertNil(t, session.MemoryID)
	AssertEqual(t, "active", session.Status)
	AssertEqual(t, 5, session.RequestCount)
	AssertEqual(t, now, session.LastActivity)
	AssertEqual(t, now.Add(24*time.Hour), session.ExpiresAt)
	AssertEqual(t, now, session.CreatedAt)
}

func TestProviderCapabilities_Creation(t *testing.T) {
	// Test creating provider capabilities
	capabilities := ProviderCapabilities{
		SupportedModels:         []string{"model1", "model2"},
		SupportedFeatures:       []string{"streaming", "function_calling"},
		SupportedRequestTypes:   []string{"code_generation", "reasoning"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: true,
		SupportsVision:          false,
		Limits: ModelLimits{
			MaxTokens:             4096,
			MaxInputLength:        2048,
			MaxOutputLength:       2048,
			MaxConcurrentRequests: 10,
		},
		Metadata: map[string]string{
			"author":  "Test Author",
			"license": "MIT",
		},
	}

	AssertEqual(t, 2, len(capabilities.SupportedModels))
	AssertContains(t, capabilities.SupportedModels, "model1")
	AssertContains(t, capabilities.SupportedModels, "model2")

	AssertEqual(t, 2, len(capabilities.SupportedFeatures))
	AssertContains(t, capabilities.SupportedFeatures, "streaming")
	AssertContains(t, capabilities.SupportedFeatures, "function_calling")

	AssertTrue(t, capabilities.SupportsStreaming)
	AssertTrue(t, capabilities.SupportsFunctionCalling)
	AssertFalse(t, capabilities.SupportsVision)

	AssertEqual(t, 4096, capabilities.Limits.MaxTokens)
	AssertEqual(t, 2048, capabilities.Limits.MaxInputLength)
	AssertEqual(t, 2048, capabilities.Limits.MaxOutputLength)
	AssertEqual(t, 10, capabilities.Limits.MaxConcurrentRequests)

	AssertEqual(t, 2, len(capabilities.Metadata))
	AssertEqual(t, "Test Author", capabilities.Metadata["author"])
	AssertEqual(t, "MIT", capabilities.Metadata["license"])
}
