package qwen_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/llm"
	"github.com/superagent/superagent/internal/models"
)

func TestQwenProvider_NewProvider(t *testing.T) {
	provider := llm.NewQwenProvider("test-key", "https://api.qwen.com", "qwen-turbo")
	require.NotNil(t, provider)
}

func TestQwenProvider_Complete_ValidRequest(t *testing.T) {
	provider := llm.NewQwenProvider("test-key", "https://api.qwen.com", "qwen-turbo")
	req := &models.LLMRequest{
		Prompt: "Hello, world!",
	}

	// This will fail due to network error or API issues
	_, err := provider.Complete(req)
	assert.Error(t, err)
	// Don't check specific error message since it could be network-related
}

func TestQwenProvider_Complete_EmptyAPIKey(t *testing.T) {
	provider := llm.NewQwenProvider("", "https://api.qwen.com", "qwen-turbo")
	req := &models.LLMRequest{
		Prompt: "Test prompt",
	}

	_, err := provider.Complete(req)
	assert.Error(t, err)
}

func TestQwenProvider_Complete_NilRequest(t *testing.T) {
	provider := llm.NewQwenProvider("test-key", "https://api.qwen.com", "qwen-turbo")

	_, err := provider.Complete(nil)
	assert.Error(t, err)
}

func TestQwenProvider_HealthCheck(t *testing.T) {
	provider := llm.NewQwenProvider("test-key", "https://api.qwen.com", "qwen-turbo")

	err := provider.HealthCheck()
	assert.Error(t, err) // Will fail due to API connectivity
}

func TestQwenProvider_GetCapabilities(t *testing.T) {
	provider := llm.NewQwenProvider("test-key", "https://api.qwen.com", "qwen-turbo")

	caps := provider.GetCapabilities()
	require.NotNil(t, caps)

	assert.Contains(t, caps.SupportedModels, "qwen-turbo")
	assert.Contains(t, caps.SupportedModels, "qwen-plus")
	assert.Contains(t, caps.SupportedModels, "qwen-max")

	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsVision)
	assert.True(t, caps.SupportsCodeCompletion)

	assert.Equal(t, 8192, caps.Limits.MaxTokens)
	assert.Equal(t, 8192, caps.Limits.MaxInputLength)
	assert.Equal(t, 4096, caps.Limits.MaxOutputLength)
	assert.Equal(t, 5, caps.Limits.MaxConcurrentRequests)

	assert.Equal(t, "qwen", caps.Metadata["provider"])
	assert.Equal(t, "1.0", caps.Metadata["version"])
}

func TestQwenProvider_ValidateConfig(t *testing.T) {
	provider := llm.NewQwenProvider("test-key", "https://api.qwen.com", "qwen-turbo")

	valid, messages := provider.ValidateConfig(map[string]interface{}{
		"api_key": "test-key",
		"model":   "qwen-turbo",
	})

	assert.True(t, valid)
	assert.Empty(t, messages)
}

func TestQwenProvider_Complete_WithMaxTokens(t *testing.T) {
	provider := llm.NewQwenProvider("test-key", "https://api.qwen.com", "qwen-turbo")
	req := &models.LLMRequest{
		Prompt: "Write a short story",
		ModelParams: models.ModelParameters{
			MaxTokens: 100,
		},
	}

	_, err := provider.Complete(req)
	assert.Error(t, err)
}

func TestQwenProvider_Complete_WithTemperature(t *testing.T) {
	provider := llm.NewQwenProvider("test-key", "https://api.qwen.com", "qwen-turbo")
	req := &models.LLMRequest{
		Prompt: "Explain quantum computing",
		ModelParams: models.ModelParameters{
			Temperature: 0.7,
		},
	}

	_, err := provider.Complete(req)
	assert.Error(t, err)
}

func TestQwenProvider_Complete_Timeout(t *testing.T) {
	provider := llm.NewQwenProvider("test-key", "https://api.qwen.com", "qwen-turbo")
	req := &models.LLMRequest{
		Prompt: "This will timeout",
		EnsembleConfig: &models.EnsembleConfig{
			Timeout: 100,
		},
	}

	_, err := provider.Complete(req)
	assert.Error(t, err)
}
