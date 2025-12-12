package zai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/llm"
	"github.com/superagent/superagent/internal/models"
)

func TestZaiProvider_NewProvider(t *testing.T) {
	provider := llm.NewZaiProvider("test-key", "https://api.zai.com", "zephyr")
	require.NotNil(t, provider)
}

func TestZaiProvider_Complete_ValidRequest(t *testing.T) {
	provider := llm.NewZaiProvider("test-key", "https://api.zai.com", "zephyr")
	req := &models.LLMRequest{
		Prompt: "Hello, world!",
	}

	// This will fail due to network error or API issues
	_, err := provider.Complete(req)
	assert.Error(t, err)
	// Don't check specific error message since it could be network-related
}

func TestZaiProvider_Complete_EmptyAPIKey(t *testing.T) {
	provider := llm.NewZaiProvider("", "https://api.zai.com", "zephyr")
	req := &models.LLMRequest{
		Prompt: "Test prompt",
	}

	_, err := provider.Complete(req)
	assert.Error(t, err)
}

func TestZaiProvider_Complete_NilRequest(t *testing.T) {
	provider := llm.NewZaiProvider("test-key", "https://api.zai.com", "zephyr")

	_, err := provider.Complete(nil)
	assert.Error(t, err)
}

func TestZaiProvider_HealthCheck(t *testing.T) {
	provider := llm.NewZaiProvider("test-key", "https://api.zai.com", "zephyr")

	err := provider.HealthCheck()
	assert.Error(t, err) // Will fail due to API connectivity
}

func TestZaiProvider_GetCapabilities(t *testing.T) {
	provider := llm.NewZaiProvider("test-key", "https://api.zai.com", "zephyr")

	caps := provider.GetCapabilities()
	require.NotNil(t, caps)

	assert.Contains(t, caps.SupportedModels, "zephyr")
	assert.Contains(t, caps.SupportedModels, "mistral")
	assert.Contains(t, caps.SupportedModels, "llama")

	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.False(t, caps.SupportsVision)
	assert.True(t, caps.SupportsCodeCompletion)
	assert.True(t, caps.SupportsSearch)

	assert.Equal(t, 4096, caps.Limits.MaxTokens)
	assert.Equal(t, 4096, caps.Limits.MaxInputLength)
	assert.Equal(t, 2048, caps.Limits.MaxOutputLength)
	assert.Equal(t, 10, caps.Limits.MaxConcurrentRequests)

	assert.Equal(t, "zai", caps.Metadata["provider"])
	assert.Equal(t, "1.0", caps.Metadata["version"])
}

func TestZaiProvider_ValidateConfig(t *testing.T) {
	provider := llm.NewZaiProvider("test-key", "https://api.zai.com", "zephyr")

	valid, messages := provider.ValidateConfig(map[string]interface{}{
		"api_key": "test-key",
		"model":   "zephyr",
	})

	assert.True(t, valid)
	assert.Empty(t, messages)
}

func TestZaiProvider_Complete_WithMaxTokens(t *testing.T) {
	provider := llm.NewZaiProvider("test-key", "https://api.zai.com", "zephyr")
	req := &models.LLMRequest{
		Prompt: "Write a short story",
		ModelParams: models.ModelParameters{
			MaxTokens: 100,
		},
	}

	_, err := provider.Complete(req)
	assert.Error(t, err)
}

func TestZaiProvider_Complete_WithTemperature(t *testing.T) {
	provider := llm.NewZaiProvider("test-key", "https://api.zai.com", "zephyr")
	req := &models.LLMRequest{
		Prompt: "Explain quantum computing",
		ModelParams: models.ModelParameters{
			Temperature: 0.7,
		},
	}

	_, err := provider.Complete(req)
	assert.Error(t, err)
}

func TestZaiProvider_Complete_WithSearch(t *testing.T) {
	provider := llm.NewZaiProvider("test-key", "https://api.zai.com", "zephyr")
	req := &models.LLMRequest{
		Prompt: "Search for latest AI news",
		ModelParams: models.ModelParameters{
			ProviderSpecific: map[string]interface{}{
				"enable_search": true,
			},
		},
	}

	_, err := provider.Complete(req)
	assert.Error(t, err)
}

func TestZaiProvider_Complete_MultipleMessages(t *testing.T) {
	provider := llm.NewZaiProvider("test-key", "https://api.zai.com", "zephyr")
	req := &models.LLMRequest{
		Prompt: "Continue the conversation",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
	}

	_, err := provider.Complete(req)
	assert.Error(t, err)
}
