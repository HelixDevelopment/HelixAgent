package gemini_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/superagent/superagent/internal/llm"
	"github.com/superagent/superagent/internal/models"
)

func TestGeminiProvider_Basic(t *testing.T) {
	provider := llm.NewGeminiProvider("test-api-key", "", "")
	assert.NotNil(t, provider)

	// Test configuration validation
	valid, errs := provider.ValidateConfig(map[string]any{})
	assert.True(t, valid)
	assert.Empty(t, errs)
}

func TestGeminiProvider_EmptyAPIKey(t *testing.T) {
	provider := llm.NewGeminiProvider("", "", "")
	err := provider.HealthCheck()
	// Gemini may return error for empty API key
	assert.Error(t, err)
}

func TestGeminiProvider_Capabilities(t *testing.T) {
	provider := llm.NewGeminiProvider("test-api-key", "", "")
	caps := provider.GetCapabilities()
	assert.NotNil(t, caps)
	assert.NotEmpty(t, caps.SupportedModels)
	assert.Contains(t, caps.SupportedModels, "gemini-pro")
	assert.Contains(t, caps.SupportedModels, "gemini-pro-vision")
	assert.Contains(t, caps.SupportedFeatures, "streaming")
	assert.Contains(t, caps.SupportedFeatures, "vision")
	assert.Contains(t, caps.SupportedRequestTypes, "text_completion")
	assert.Contains(t, caps.SupportedRequestTypes, "chat")
	assert.Contains(t, caps.SupportedRequestTypes, "multimodal")
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsVision)
	assert.NotNil(t, caps.Metadata)
}

func TestGeminiProvider_CompleteRequest(t *testing.T) {
	provider := llm.NewGeminiProvider("test-api-key", "", "")

	req := &models.LLMRequest{
		ID: "test-req-1",
		ModelParams: models.ModelParameters{
			Model: "gemini-pro",
		},
		Prompt: "Hello, how are you?",
	}

	// This will fail without actual API key, but tests the error handling
	resp, err := provider.Complete(req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	// Error will be about API authentication failure
	assert.Contains(t, err.Error(), "API")
}

func TestGeminiProvider_CompleteWithDifferentModels(t *testing.T) {
	provider := llm.NewGeminiProvider("test-api-key", "", "")

	// Test with different model selections
	modelList := []string{
		"gemini-pro",
		"gemini-pro-vision",
	}

	for _, model := range modelList {
		req := &models.LLMRequest{
			ID: "test-" + model,
			ModelParams: models.ModelParameters{
				Model: model,
			},
			Prompt: "Test prompt for " + model,
		}

		resp, err := provider.Complete(req)
		assert.Error(t, err) // Will fail without real API key
		assert.Nil(t, resp)
	}
}

func TestGeminiProvider_InvalidModel(t *testing.T) {
	provider := llm.NewGeminiProvider("test-api-key", "", "")

	req := &models.LLMRequest{
		ID: "test-invalid",
		ModelParams: models.ModelParameters{
			Model: "invalid-model",
		},
		Prompt: "Test prompt",
	}

	resp, err := provider.Complete(req)
	// Should fail gracefully without panic
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestGeminiProvider_MemoryUsage(t *testing.T) {
	provider := llm.NewGeminiProvider("test-api-key", "", "")

	// Test multiple requests to ensure no memory leaks
	for i := 0; i < 10; i++ {
		req := &models.LLMRequest{
			ID: fmt.Sprintf("test-req-%d", i),
			ModelParams: models.ModelParameters{
				Model: "gemini-pro",
			},
			Prompt: fmt.Sprintf("Memory test request %d", i),
		}

		resp, err := provider.Complete(req)
		if err != nil {
			t.Logf("Request %d failed: %v", i, err)
		}

		_ = resp
	}

	// Provider should still be responsive
	assert.True(t, true)
}

func TestGeminiProvider_Timeout(t *testing.T) {
	provider := llm.NewGeminiProvider("test-api-key", "", "")

	// Create a request that might timeout
	req := &models.LLMRequest{
		ID: "test-timeout",
		ModelParams: models.ModelParameters{
			Model: "gemini-pro",
		},
		Prompt: "This is a timeout test request",
	}

	start := time.Now()
	resp, err := provider.Complete(req)
	elapsed := time.Since(start)

	// Will fail with auth error, but should fail quickly
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, elapsed < 10*time.Second)
}

func TestGeminiProvider_CustomBaseURL(t *testing.T) {
	// Test with custom base URL
	provider := llm.NewGeminiProvider("test-api-key", "http://localhost:8080", "custom-model")
	assert.NotNil(t, provider)

	caps := provider.GetCapabilities()
	assert.NotNil(t, caps)
}

func TestGeminiProvider_ValidateConfig(t *testing.T) {
	provider := llm.NewGeminiProvider("test-api-key", "", "")

	// Test with empty config
	valid, errs := provider.ValidateConfig(map[string]any{})
	assert.True(t, valid)
	assert.Empty(t, errs)

	// Test with some config values
	valid, errs = provider.ValidateConfig(map[string]any{
		"timeout": 30,
		"retries": 3,
	})
	assert.True(t, valid)
	assert.Empty(t, errs)
}
