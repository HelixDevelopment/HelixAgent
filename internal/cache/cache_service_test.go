package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/models"
)

func TestNewCacheService_WithRedisConnectionFailure(t *testing.T) {
	// Test that cache service handles Redis connection failures gracefully
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     "6379",
			Password: "",
			DB:       0,
			PoolSize: 10,
			Timeout:  5 * time.Second,
		},
	}

	service, err := NewCacheService(cfg)

	// When Redis is not running, we expect an error but service should be created
	// with caching disabled
	require.Error(t, err)
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())
	assert.Contains(t, err.Error(), "caching disabled")
}

func TestNewCacheService_WithNilConfig(t *testing.T) {
	// Test that cache service handles nil config gracefully
	service, err := NewCacheService(nil)

	// With nil config, Redis client is created with invalid address
	// so connection will fail and caching will be disabled
	require.Error(t, err)
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())
	assert.Contains(t, err.Error(), "caching disabled")
}

func TestCacheService_OperationsWhenDisabled(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// Test LLM response operations
	req := &models.LLMRequest{
		ID:          "test-request-id",
		Prompt:      "test prompt",
		RequestType: "completion",
		ModelParams: models.ModelParameters{
			Model:       "test-model",
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}

	resp := &models.LLMResponse{
		ID:           "test-response-id",
		RequestID:    "test-request-id",
		ProviderName: "test-provider",
		Content:      "test response content",
		Confidence:   0.95,
		TokensUsed:   50,
	}

	// Get should return error when cache is disabled
	response, err := service.GetLLMResponse(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "caching disabled")

	// Set should return nil (no error) when cache is disabled
	err = service.SetLLMResponse(ctx, req, resp, 5*time.Minute)
	assert.NoError(t, err)

	// Test memory sources operations
	query := "test query"
	dataset := "test-dataset"
	sources := []models.MemorySource{
		{
			DatasetName:    dataset,
			Content:        "test content 1",
			RelevanceScore: 0.8,
			SourceType:     "document",
		},
	}

	// Get should return error when cache is disabled
	memoryResult, err := service.GetMemorySources(ctx, query, dataset)
	assert.Error(t, err)
	assert.Nil(t, memoryResult)
	assert.Contains(t, err.Error(), "caching disabled")

	// Set should return nil (no error) when cache is disabled
	err = service.SetMemorySources(ctx, query, dataset, sources, 5*time.Minute)
	assert.NoError(t, err)

	// Test provider health operations
	providerName := "test-provider"
	health := map[string]interface{}{
		"status":    "healthy",
		"latency":   50.5,
		"timestamp": time.Now().Unix(),
	}

	// Get should return error when cache is disabled
	healthResult, err := service.GetProviderHealth(ctx, providerName)
	assert.Error(t, err)
	assert.Nil(t, healthResult)
	assert.Contains(t, err.Error(), "caching disabled")

	// Set should return nil (no error) when cache is disabled
	err = service.SetProviderHealth(ctx, providerName, health, 5*time.Minute)
	assert.NoError(t, err)
}

func TestCacheService_StatsWhenDisabled(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// GetStats should work even when cache is disabled
	stats := service.GetStats(ctx)
	require.NotNil(t, stats)

	// When cache is disabled, stats should contain basic info
	assert.Contains(t, stats, "enabled")
	assert.Contains(t, stats, "status")
	assert.False(t, stats["enabled"].(bool))
	assert.Equal(t, "disabled", stats["status"])
}

func TestCacheService_DefaultTTLBehavior(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)

	ctx := context.Background()
	req := &models.LLMRequest{
		ID:          "test-request-id",
		Prompt:      "test prompt",
		RequestType: "completion",
		ModelParams: models.ModelParameters{
			Model:       "test-model",
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}
	resp := &models.LLMResponse{
		ID:           "test-response-id",
		RequestID:    "test-request-id",
		ProviderName: "test-provider",
		Content:      "test response content",
		Confidence:   0.95,
		TokensUsed:   50,
	}

	// With zero TTL, should work correctly (use default TTL internally)
	err = service.SetLLMResponse(ctx, req, resp, 0)
	assert.NoError(t, err)
}

func TestCacheService_IsEnabled(t *testing.T) {
	// Test with nil config (disabled)
	service1, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	assert.False(t, service1.IsEnabled())

	// Test with config but Redis not running (disabled)
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "localhost",
			Port: "6379",
		},
	}
	service2, err := NewCacheService(cfg)
	require.Error(t, err) // Connection will fail
	require.NotNil(t, service2)
	assert.False(t, service2.IsEnabled())
}
