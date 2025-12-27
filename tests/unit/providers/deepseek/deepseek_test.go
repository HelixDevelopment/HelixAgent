package deepseek_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/superagent/superagent/internal/llm/providers"
	"github.com/superagent/superagent/internal/models"
)

func TestNewDeepSeekProvider(t *testing.T) {
	logger := logrus.New()

	t.Run("valid configuration", func(t *testing.T) {
		provider, err := providers.NewDeepSeekProvider(
			"test-api-key",
			"https://api.deepseek.com",
			"deepseek-chat",
			30*time.Second,
			3,
			logger,
		)

		require.NoError(t, err)
		require.NotNil(t, provider)
	})

	t.Run("missing API key", func(t *testing.T) {
		provider, err := providers.NewDeepSeekProvider(
			"",
			"https://api.deepseek.com",
			"deepseek-chat",
			30*time.Second,
			3,
			logger,
		)

		require.Error(t, err)
		require.Nil(t, provider)
		assert.Contains(t, err.Error(), "API key is required")
	})

	t.Run("missing model", func(t *testing.T) {
		provider, err := providers.NewDeepSeekProvider(
			"test-api-key",
			"https://api.deepseek.com",
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

func TestDeepSeekProvider_Complete(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewDeepSeekProvider(
		"test-api-key",
		"https://api.deepseek.com",
		"deepseek-chat",
		30*time.Second,
		3,
		logger,
	)
	require.NoError(t, err)

	request := &models.LLMRequest{
		ModelParams: models.ModelParameters{
			Model: "deepseek-chat",
		},
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "Hello, DeepSeek!",
			},
		},
	}

	ctx := context.Background()
	response, err := provider.Complete(ctx, request)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.NotEmpty(t, response.ID)
	assert.Equal(t, "deepseek", response.ProviderName)
	assert.NotEmpty(t, response.Content)
}

func TestDeepSeekProvider_CompleteStream(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewDeepSeekProvider(
		"test-api-key",
		"https://api.deepseek.com",
		"deepseek-chat",
		30*time.Second,
		3,
		logger,
	)
	require.NoError(t, err)

	request := &models.LLMRequest{
		ModelParams: models.ModelParameters{
			Model: "deepseek-chat",
		},
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "Hello, DeepSeek!",
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

func TestDeepSeekProvider_GetCapabilities(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewDeepSeekProvider(
		"test-api-key",
		"https://api.deepseek.com",
		"deepseek-chat",
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

func TestDeepSeekProvider_ValidateConfig(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewDeepSeekProvider(
		"test-api-key",
		"https://api.deepseek.com",
		"deepseek-chat",
		30*time.Second,
		3,
		logger,
	)
	require.NoError(t, err)

	t.Run("valid config", func(t *testing.T) {
		config := map[string]interface{}{
			"api_key": "test-key",
			"model":   "deepseek-chat",
		}

		valid, errors := provider.ValidateConfig(config)
		assert.True(t, valid)
		assert.Empty(t, errors)
	})

	t.Run("invalid config - missing API key", func(t *testing.T) {
		config := map[string]interface{}{
			"model": "deepseek-chat",
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

func TestDeepSeekProvider_HealthCheck(t *testing.T) {
	logger := logrus.New()

	provider, err := providers.NewDeepSeekProvider(
		"test-api-key",
		"https://api.deepseek.com",
		"deepseek-chat",
		30*time.Second,
		3,
		logger,
	)
	require.NoError(t, err)

	err = provider.HealthCheck()
	assert.NoError(t, err)
}
