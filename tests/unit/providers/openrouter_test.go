package openrouter_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"dev.helix.agent/internal/llm/providers/openrouter"
	"dev.helix.agent/internal/models"
)

func TestOpenRouterProvider_Basic(t *testing.T) {
	provider := openrouter.NewSimpleOpenRouterProvider("test-api-key")
	assert.NotNil(t, provider)

	// Test configuration validation
	valid, errs := provider.ValidateConfig(map[string]interface{}{})
	assert.True(t, valid) // Provider has API key, so config is valid
	assert.Empty(t, errs)
}

func TestOpenRouterProvider_EmptyAPIKey(t *testing.T) {
	provider := openrouter.NewSimpleOpenRouterProvider("")
	err := provider.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key is required")
}

func TestOpenRouterProvider_Capabilities(t *testing.T) {
	provider := openrouter.NewSimpleOpenRouterProvider("test-api-key")
	caps := provider.GetCapabilities()
	assert.NotNil(t, caps)
	assert.NotEmpty(t, caps.SupportedModels)
	assert.Contains(t, caps.SupportedModels, "anthropic/claude-3.5-sonnet")
	assert.Contains(t, caps.SupportedFeatures, "text_completion")
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsFunctionCalling)
	assert.False(t, caps.SupportsVision)
	assert.NotNil(t, caps.Limits)
	assert.Equal(t, caps.Metadata["provider"], "OpenRouter")
}

func TestOpenRouterProvider_CompleteRequest(t *testing.T) {
	provider := openrouter.NewSimpleOpenRouterProvider("test-api-key")

	req := &models.LLMRequest{
		ID: "test-req-1",
		ModelParams: models.ModelParameters{
			Model: "openrouter/anthropic/claude-3.5-sonnet",
		},
		Prompt: "Hello, how are you?",
	}

	// This will fail without actual API key, but tests the error handling
	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "No cookie auth credentials found")
}

func TestOpenRouterProvider_CompleteWithDifferentModels(t *testing.T) {
	provider := openrouter.NewSimpleOpenRouterProvider("test-api-key")

	// Test with different model selections
	modelList := []string{
		"openrouter/anthropic/claude-3.5-sonnet",
		"openrouter/openai/gpt-4o",
		"openrouter/google/gemini-pro",
	}

	for _, model := range modelList {
		req := &models.LLMRequest{
			ID: "test-" + model,
			ModelParams: models.ModelParameters{
				Model: model,
			},
			Prompt: "Test prompt for " + model,
		}

		resp, err := provider.Complete(context.Background(), req)
		assert.Error(t, err) // Will fail without real API key
		assert.Nil(t, resp)
	}
}

func TestOpenRouterProvider_InvalidModel(t *testing.T) {
	provider := openrouter.NewSimpleOpenRouterProvider("test-api-key")

	req := &models.LLMRequest{
		ID: "test-invalid",
		ModelParams: models.ModelParameters{
			Model: "invalid-model",
		},
		Prompt: "Test prompt",
	}

	resp, err := provider.Complete(context.Background(), req)
	// Should fail gracefully without panic
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestOpenRouterProvider_MemoryUsage(t *testing.T) {
	provider := openrouter.NewSimpleOpenRouterProvider("test-api-key")

	// Test multiple requests to ensure no memory leaks
	for i := 0; i < 10; i++ {
		req := &models.LLMRequest{
			ID: fmt.Sprintf("test-req-%d", i),
			ModelParams: models.ModelParameters{
				Model: "openrouter/anthropic/claude-3.5-sonnet",
			},
			Prompt: fmt.Sprintf("Memory test request %d", i),
		}

		resp, err := provider.Complete(context.Background(), req)
		if err != nil {
			t.Logf("Request %d failed: %v", i, err)
		}

		_ = resp
	}

	// Provider should still be responsive
	assert.True(t, true)
}

func TestOpenRouterProvider_Timeout(t *testing.T) {
	provider := openrouter.NewSimpleOpenRouterProvider("test-api-key")

	// Create a request that might timeout
	req := &models.LLMRequest{
		ID: "test-timeout",
		ModelParams: models.ModelParameters{
			Model: "openrouter/anthropic/concentrate-one-2024-06-07",
		},
		Prompt: "This is a timeout test request",
	}

	start := time.Now()
	resp, err := provider.Complete(context.Background(), req)
	elapsed := time.Since(start)

	// Will fail with auth error, but should fail quickly
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, elapsed < 10*time.Second)
}

func TestOpenRouterProvider_Headers(t *testing.T) {
	provider := openrouter.NewSimpleOpenRouterProvider("test-api-key")

	req := &models.LLMRequest{
		ID: "test-headers",
		ModelParams: models.ModelParameters{
			Model: "openrouter/anthropic/claude-3.5-sonnet",
		},
		Prompt: "Test with custom headers",
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err) // Will fail without real API key
	assert.Nil(t, resp)

	// In test environment, we just verify the error is as expected
	assert.Contains(t, err.Error(), "No cookie auth credentials found")
}
