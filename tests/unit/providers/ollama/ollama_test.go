package ollama_test

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/superagent/superagent/internal/llm/providers"
	"github.com/superagent/superagent/internal/models"
)

func TestNewOllamaProvider(t *testing.T) {
	logger := logrus.New()

	t.Run("valid configuration", func(t *testing.T) {
		provider, err := providers.NewOllamaProvider(
			"http://localhost:11434",
			"llama2",
			30*time.Second,
			3,
			logger,
		)

		require.NoError(t, err)
		require.NotNil(t, provider)
	})

	t.Run("missing model", func(t *testing.T) {
		provider, err := providers.NewOllamaProvider(
			"http://localhost:11434",
			"",
			30*time.Second,
			3,
			logger,
		)

		require.Error(t, err)
		require.Nil(t, provider)
		assert.Contains(t, err.Error(), "model is required")
	})
}

func TestOllamaProvider_Complete(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewOllamaProvider(
		"http://localhost:11434",
		"llama2",
		30*time.Second,
		3,
		logger,
	)
	require.NoError(t, err)

	request := &models.LLMRequest{
		ModelParams: models.ModelParameters{
			Model: "llama2",
		},
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "Hello, Ollama!",
			},
		},
	}

	ctx := context.Background()
	response, err := provider.Complete(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.NotEmpty(t, response.ID)
	assert.Equal(t, "ollama", response.ProviderName)
	assert.NotEmpty(t, response.Content)
}

func TestOllamaProvider_CompleteStream(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewOllamaProvider(
		"http://localhost:11434",
		"llama2",
		30*time.Second,
		3,
		logger,
	)
	require.NoError(t, err)

	request := &models.LLMRequest{
		ModelParams: models.ModelParameters{
			Model: "llama2",
		},
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "Hello, Ollama!",
			},
		},
	}

	ctx := context.Background()
	responseChan, err := provider.CompleteStream(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, responseChan)

	response, ok := <-responseChan
	assert.True(t, ok)
	assert.NotNil(t, response)
	assert.NotEmpty(t, response.ID)
}

func TestOllamaProvider_GetCapabilities(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewOllamaProvider(
		"http://localhost:11434",
		"llama2",
		30*time.Second,
		3,
		logger,
	)
	require.NoError(t, err)

	capabilities := provider.GetCapabilities()

	assert.NotNil(t, capabilities)
	assert.True(t, capabilities.SupportsStreaming)
	assert.Greater(t, capabilities.Limits.MaxTokens, 0)
	assert.True(t, capabilities.SupportsFunctionCalling)
	assert.False(t, capabilities.SupportsVision)
	assert.NotEmpty(t, capabilities.SupportedModels)
}

func TestOllamaProvider_ValidateConfig(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewOllamaProvider(
		"http://localhost:11434",
		"llama2",
		30*time.Second,
		3,
		logger,
	)
	require.NoError(t, err)

	t.Run("valid config", func(t *testing.T) {
		config := map[string]interface{}{
			"model": "llama2",
		}

		valid, errors := provider.ValidateConfig(config)
		assert.True(t, valid)
		assert.Empty(t, errors)
	})

	t.Run("invalid config - missing model", func(t *testing.T) {
		config := map[string]interface{}{}

		valid, errors := provider.ValidateConfig(config)
		assert.False(t, valid)
		assert.NotEmpty(t, errors)
	})
}

func TestOllamaProvider_HealthCheck(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewOllamaProvider(
		"http://localhost:11434",
		"llama2",
		30*time.Second,
		3,
		logger,
	)
	require.NoError(t, err)

	err = provider.HealthCheck()
	assert.NoError(t, err)
}
