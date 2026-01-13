package cerebras_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/llm/providers/cerebras"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/tests/testutils"
)

func TestCerebrasProvider_Basic(t *testing.T) {
	provider := cerebras.NewCerebrasProvider("test-api-key", "", "llama-3.3-70b")
	require.NotNil(t, provider)
}

func TestCerebrasProvider_WithCustomBaseURL(t *testing.T) {
	provider := cerebras.NewCerebrasProvider("test-api-key", "https://custom.api.com", "llama-3.3-70b")
	require.NotNil(t, provider)
}

func TestCerebrasProvider_WithDefaultModel(t *testing.T) {
	provider := cerebras.NewCerebrasProvider("test-api-key", "", "")
	require.NotNil(t, provider)
}

func TestCerebrasProvider_Capabilities(t *testing.T) {
	provider := cerebras.NewCerebrasProvider("test-api-key", "", "llama-3.3-70b")
	require.NotNil(t, provider)

	capabilities := provider.GetCapabilities()
	require.NotNil(t, capabilities)

	// Check supported features
	assert.Contains(t, capabilities.SupportedFeatures, "text_completion")
	assert.Contains(t, capabilities.SupportedFeatures, "chat")
	assert.Contains(t, capabilities.SupportedFeatures, "streaming")

	// Check capability flags
	assert.True(t, capabilities.SupportsStreaming)

	// Check limits
	assert.Greater(t, capabilities.Limits.MaxTokens, 0)
	assert.Greater(t, capabilities.Limits.MaxInputLength, 0)
	assert.Greater(t, capabilities.Limits.MaxOutputLength, 0)

	// Check supported models
	assert.NotEmpty(t, capabilities.SupportedModels)
	assert.Contains(t, capabilities.SupportedModels, "llama-3.3-70b")

	// Check metadata
	assert.NotNil(t, capabilities.Metadata)
}

func TestCerebrasProvider_ValidateConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		provider := cerebras.NewCerebrasProvider("test-api-key", "https://api.cerebras.ai", "llama-3.3-70b")
		require.NotNil(t, provider)

		valid, errors := provider.ValidateConfig(nil)
		assert.True(t, valid)
		assert.Empty(t, errors)
	})

	t.Run("missing api key", func(t *testing.T) {
		provider := cerebras.NewCerebrasProvider("", "https://api.cerebras.ai", "llama-3.3-70b")
		require.NotNil(t, provider)

		valid, errors := provider.ValidateConfig(nil)
		assert.False(t, valid)
		assert.NotEmpty(t, errors)
	})

	t.Run("missing model uses default", func(t *testing.T) {
		provider := cerebras.NewCerebrasProvider("test-api-key", "https://api.cerebras.ai", "")
		require.NotNil(t, provider)

		valid, errors := provider.ValidateConfig(nil)
		assert.True(t, valid)
		assert.Empty(t, errors)
	})
}

func TestCerebrasProvider_WithRetry(t *testing.T) {
	retryConfig := cerebras.RetryConfig{
		MaxRetries:   5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}
	provider := cerebras.NewCerebrasProviderWithRetry("test-api-key", "", "llama-3.3-70b", retryConfig)
	require.NotNil(t, provider)
}

func TestCerebrasProvider_DefaultRetryConfig(t *testing.T) {
	config := cerebras.DefaultRetryConfig()
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
}

func TestCerebrasProvider_HealthCheck_NoAPIKey(t *testing.T) {
	provider := cerebras.NewCerebrasProvider("", "", "")
	err := provider.HealthCheck()
	assert.Error(t, err)
}

func TestCerebrasProvider_HealthCheck_WithAPIKey(t *testing.T) {
	provider := cerebras.NewCerebrasProvider("test-api-key", "", "")
	// Health check requires actual API, so we just verify it doesn't panic
	_ = provider.HealthCheck()
}

// Integration tests that use mock LLM server when available
func TestCerebrasProvider_Complete(t *testing.T) {
	testutils.SkipIfNoInfrastructure(t, "llm")

	mockURL := testutils.GetMockLLMBaseURL()
	apiKey := testutils.GetMockAPIKey()

	provider := cerebras.NewCerebrasProvider(apiKey, mockURL, "llama-3.3-70b")
	require.NotNil(t, provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID:     "test-complete",
		Prompt: "Say hello",
		ModelParams: models.ModelParameters{
			Model: "llama-3.3-70b",
		},
	}

	result, err := provider.Complete(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestCerebrasProvider_CompleteStream(t *testing.T) {
	testutils.SkipIfNoInfrastructure(t, "llm")

	mockURL := testutils.GetMockLLMBaseURL()
	apiKey := testutils.GetMockAPIKey()

	provider := cerebras.NewCerebrasProvider(apiKey, mockURL, "llama-3.3-70b")
	require.NotNil(t, provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID:     "test-stream",
		Prompt: "Say hello",
		ModelParams: models.ModelParameters{
			Model: "llama-3.3-70b",
		},
	}

	stream, err := provider.CompleteStream(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// Collect some responses
	var responses []*models.LLMResponse
	for resp := range stream {
		responses = append(responses, resp)
		if len(responses) >= 3 {
			break
		}
	}
	assert.NotEmpty(t, responses)
}

func TestCerebrasProvider_Complete_WithMessages(t *testing.T) {
	testutils.SkipIfNoInfrastructure(t, "llm")

	mockURL := testutils.GetMockLLMBaseURL()
	apiKey := testutils.GetMockAPIKey()

	provider := cerebras.NewCerebrasProvider(apiKey, mockURL, "llama-3.3-70b")
	require.NotNil(t, provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID: "test-messages",
		Messages: []models.Message{
			{Role: "user", Content: "What is 2+2?"},
		},
		ModelParams: models.ModelParameters{
			Model:       "llama-3.3-70b",
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}

	result, err := provider.Complete(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestCerebrasProvider_Complete_ContextCancellation(t *testing.T) {
	provider := cerebras.NewCerebrasProvider("test-api-key", "https://api.cerebras.ai", "llama-3.3-70b")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := &models.LLMRequest{
		ID:     "test-cancelled",
		Prompt: "Say hello",
	}

	_, err := provider.Complete(ctx, req)
	assert.Error(t, err)
}

func TestCerebrasProvider_ModelMetadata(t *testing.T) {
	provider := cerebras.NewCerebrasProvider("test-api-key", "", "llama-3.3-70b")
	caps := provider.GetCapabilities()

	// Verify model is in the capabilities
	assert.Contains(t, caps.SupportedModels, "llama-3.3-70b")
	assert.Contains(t, caps.SupportedModels, "llama-3.1-70b")
	assert.Contains(t, caps.SupportedModels, "llama-3.1-8b")
}

func TestCerebrasProvider_HighSpeedInference(t *testing.T) {
	// Cerebras is known for high-speed inference
	provider := cerebras.NewCerebrasProvider("test-api-key", "", "llama-3.3-70b")
	caps := provider.GetCapabilities()

	// Verify metadata mentions high speed
	assert.NotNil(t, caps.Metadata)
	if vendor, ok := caps.Metadata["vendor"]; ok {
		assert.Equal(t, "Cerebras", vendor)
	}
}
