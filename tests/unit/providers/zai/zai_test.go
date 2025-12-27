package zai_test

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

func TestNewZaiProvider(t *testing.T) {
	logger := logrus.New()

	t.Run("valid configuration", func(t *testing.T) {
		provider, err := providers.NewZaiProvider(
			"test-api-key",
			"https://api.zai.com",
			"zai-pro",
			30*time.Second,
			3,
			logger,
		)

		require.NoError(t, err)
		require.NotNil(t, provider)
	})

	t.Run("missing API key", func(t *testing.T) {
		provider, err := providers.NewZaiProvider(
			"",
			"https://api.zai.com",
			"zai-pro",
			30*time.Second,
			3,
			logger,
		)

		require.Error(t, err)
		require.Nil(t, provider)
		assert.Contains(t, err.Error(), "API key is required")
	})

	t.Run("missing model", func(t *testing.T) {
		provider, err := providers.NewZaiProvider(
			"test-api-key",
			"https://api.zai.com",
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

func TestZaiProvider_Complete(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewZaiProvider(
		"test-api-key",
		"https://api.zai.com",
		"zai-pro",
		30*time.Second,
		3,
		logger,
	)
	require.NoError(t, err)

	request := &models.LLMRequest{
		ModelParams: models.ModelParameters{
			Model: "zai-pro",
		},
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "Hello, Zai!",
			},
		},
	}

	ctx := context.Background()
	response, err := provider.Complete(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.NotEmpty(t, response.ID)
	assert.Equal(t, "zai", response.ProviderName)
	assert.NotEmpty(t, response.Content)
}

func TestZaiProvider_CompleteStream(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewZaiProvider(
		"test-api-key",
		"https://api.zai.com",
		"zai-pro",
		30*time.Second,
		3,
		logger,
	)
	require.NoError(t, err)

	request := &models.LLMRequest{
		ModelParams: models.ModelParameters{
			Model: "zai-pro",
		},
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "Hello, Zai!",
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

func TestZaiProvider_GetCapabilities(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewZaiProvider(
		"test-api-key",
		"https://api.zai.com",
		"zai-pro",
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

func TestZaiProvider_ValidateConfig(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewZaiProvider(
		"test-api-key",
		"https://api.zai.com",
		"zai-pro",
		30*time.Second,
		3,
		logger,
	)
	require.NoError(t, err)

	t.Run("valid config", func(t *testing.T) {
		config := map[string]interface{}{
			"api_key": "test-key",
			"model":   "zai-pro",
		}

		valid, errors := provider.ValidateConfig(config)
		assert.True(t, valid)
		assert.Empty(t, errors)
	})

	t.Run("invalid config - missing API key", func(t *testing.T) {
		config := map[string]interface{}{
			"model": "zai-pro",
		}

		valid, errors := provider.ValidateConfig(config)
		assert.False(t, valid)
		assert.NotEmpty(t, errors)
	})

	t.Run("invalid config - missing model", func(t *testing.T) {
		config := map[string]interface{}{
			"api_key": "test-key",
		}

		valid, errors := provider.ValidateConfig(config)
		assert.False(t, valid)
		assert.NotEmpty(t, errors)
	})
}

func TestZaiProvider_HealthCheck(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewZaiProvider(
		"test-api-key",
		"https://api.zai.com",
		"zai-pro",
		30*time.Second,
		3,
		logger,
	)
	require.NoError(t, err)

	err = provider.HealthCheck()
	assert.NoError(t, err)
}
