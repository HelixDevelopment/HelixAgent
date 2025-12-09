package utils

import (
	"context"
	"testing"
	"time"

	"github.com/superagent/superagent/internal/models"
)

// TestContext provides a common context for testing
func TestContext() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	cancel() // Cancel immediately since we don't need the timeout
	return ctx
}

// MockLLMRequest creates a test LLM request
func MockLLMRequest() *models.LLMRequest {
	return &models.LLMRequest{
		ID:        "test-request-123",
		SessionID: "test-session-456",
		UserID:    "test-user-789",
		Prompt:    "Write a simple Go function that adds two numbers",
		Messages:  []models.Message{},
		ModelParams: models.ModelParameters{
			Model:            "test-model",
			Temperature:      0.7,
			MaxTokens:        1000,
			TopP:             1.0,
			StopSequences:    []string{},
			ProviderSpecific: map[string]any{},
		},
		EnsembleConfig: &models.EnsembleConfig{
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
}

// MockLLMResponse creates a test LLM response
func MockLLMResponse() *models.LLMResponse {
	return &models.LLMResponse{
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
}

// MockLLMProvider creates a test LLM provider
func MockLLMProvider() *models.LLMProvider {
	return &models.LLMProvider{
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
}

// AssertNoError fails the test if there's an error
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// AssertError fails the test if there's no error
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}

// AssertEqual fails the test if values are not equal
func AssertEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("Expected %v, got %v", expected, actual)
	}
}

// AssertNotEqual fails the test if values are equal
func AssertNotEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected == actual {
		t.Fatalf("Expected values to be different, but both were %v", expected)
	}
}

// AssertNotNil fails the test if value is nil
func AssertNotNil(t *testing.T, value any) {
	t.Helper()
	if value == nil {
		t.Fatal("Expected non-nil value")
	}
}

// AssertNil fails the test if value is not nil
func AssertNil(t *testing.T, value any) {
	t.Helper()
	if value != nil {
		t.Fatalf("Expected nil value, got %v", value)
	}
}

// AssertTrue fails the test if condition is false
func AssertTrue(t *testing.T, condition bool) {
	t.Helper()
	if !condition {
		t.Fatal("Expected true, got false")
	}
}

// AssertFalse fails the test if condition is true
func AssertFalse(t *testing.T, condition bool) {
	t.Helper()
	if condition {
		t.Fatal("Expected false, got true")
	}
}

// AssertContains fails the test if slice doesn't contain item
func AssertContains[T comparable](t *testing.T, slice []T, item T) {
	t.Helper()
	for _, v := range slice {
		if v == item {
			return
		}
	}
	t.Fatalf("Expected slice to contain %v, but it didn't", item)
}

// AssertNotContains fails the test if slice contains item
func AssertNotContains[T comparable](t *testing.T, slice []T, item T) {
	t.Helper()
	for _, v := range slice {
		if v == item {
			t.Fatalf("Expected slice to not contain %v, but it did", item)
		}
	}
}
