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

func TestRedisClient_Operations(t *testing.T) {
	// Test Redis client operations with mock Redis (not running)
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "localhost",
			Port: "6379",
		},
	}

	redisClient := NewRedisClient(cfg)
	require.NotNil(t, redisClient)

	ctx := context.Background()

	// Test Ping should fail when Redis is not running
	err := redisClient.Ping(ctx)
	require.Error(t, err)

	// Test Set should fail when Redis is not running
	testData := map[string]string{"key": "value"}
	err = redisClient.Set(ctx, "test-key", testData, 5*time.Minute)
	require.Error(t, err)

	// Test Get should fail when Redis is not running
	var result map[string]string
	err = redisClient.Get(ctx, "test-key", &result)
	require.Error(t, err)

	// Test Delete should fail when Redis is not running
	err = redisClient.Delete(ctx, "test-key")
	require.Error(t, err)
}

func TestCacheService_GenerateCacheKey(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)

	// Test cache key generation through public methods
	req := &models.LLMRequest{
		ID:          "test-request-id-1",
		Prompt:      "test prompt 1",
		RequestType: "completion",
		ModelParams: models.ModelParameters{
			Model:       "test-model",
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}

	resp := &models.LLMResponse{
		ID:           "test-response-id",
		RequestID:    "test-request-id-1",
		ProviderName: "test-provider",
		Content:      "test response content",
		Confidence:   0.95,
		TokensUsed:   50,
	}

	ctx := context.Background()

	// Test SetLLMResponse (uses generateCacheKey internally)
	err = service.SetLLMResponse(ctx, req, resp, 5*time.Minute)
	assert.NoError(t, err) // Should return nil when cache is disabled

	// Test GetLLMResponse (uses generateCacheKey internally)
	retrievedResp, err := service.GetLLMResponse(ctx, req)
	assert.Error(t, err) // Cache is disabled
	assert.Nil(t, retrievedResp)
}

func TestCacheService_HashString(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)

	// Test hashString method (private, but we can test through public methods)
	// Since hashString is private, we can't test it directly
	// But we can verify that the service methods that use it work correctly

	ctx := context.Background()
	query := "test query"
	dataset := "test-dataset"

	// Test GetMemorySources (uses hashString internally)
	sources, err := service.GetMemorySources(ctx, query, dataset)
	require.Error(t, err) // Cache is disabled
	assert.Nil(t, sources)

	// Test SetMemorySources (uses hashString internally)
	testSources := []models.MemorySource{
		{
			DatasetName:    dataset,
			Content:        "test content",
			RelevanceScore: 0.8,
			SourceType:     "document",
		},
	}
	err = service.SetMemorySources(ctx, query, dataset, testSources, 5*time.Minute)
	assert.NoError(t, err) // Should return nil when cache is disabled
}

func TestCacheService_UserSessionOperations(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// Test user session operations when cache is disabled
	session := &models.UserSession{
		ID:           "test-session-id",
		UserID:       "test-user-id",
		SessionToken: "test-session-token",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	// Get should return error when cache is disabled
	retrievedSession, err := service.GetUserSession(ctx, session.SessionToken)
	assert.Error(t, err)
	assert.Nil(t, retrievedSession)
	assert.Contains(t, err.Error(), "caching disabled")

	// Set should return nil (no error) when cache is disabled
	err = service.SetUserSession(ctx, session, 0)
	assert.NoError(t, err)
}

func TestCacheService_APIKeyOperations(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// Test API key operations when cache is disabled
	apiKey := "test-api-key-12345"
	keyInfo := map[string]interface{}{
		"user_id":    "test-user-id",
		"rate_limit": 1000,
		"expires_at": time.Now().Add(30 * 24 * time.Hour).Unix(),
	}

	// Get should return error when cache is disabled
	retrievedKeyInfo, err := service.GetAPIKey(ctx, apiKey)
	assert.Error(t, err)
	assert.Nil(t, retrievedKeyInfo)
	assert.Contains(t, err.Error(), "caching disabled")

	// Set should return nil (no error) when cache is disabled
	err = service.SetAPIKey(ctx, apiKey, keyInfo, 0)
	assert.NoError(t, err)
}

func TestCacheService_InvalidateUserCache(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// InvalidateUserCache should return nil when cache is disabled
	err = service.InvalidateUserCache(ctx, "test-user-id")
	assert.NoError(t, err)
}

func TestCacheService_ClearExpired(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// ClearExpired should return nil when cache is disabled
	err = service.ClearExpired(ctx)
	assert.NoError(t, err)
}

func TestCacheService_GetHitCount(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// GetHitCount should return error when cache is disabled
	count, err := service.GetHitCount(ctx, "test-key")
	assert.Error(t, err)
	assert.Equal(t, int64(0), count)
	assert.Contains(t, err.Error(), "caching disabled")
}

func TestCacheService_Close(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)

	// Close should not panic
	err = service.Close()
	assert.NoError(t, err)
}

func TestCacheConfig(t *testing.T) {
	// Test NewCacheConfig
	config := NewCacheConfig()
	require.NotNil(t, config)

	assert.True(t, config.Enabled)
	assert.Equal(t, 30*time.Minute, config.DefaultTTL)
	assert.Nil(t, config.Redis)
}

func TestCacheService_TTLMechanisms(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// Test different TTL values
	testCases := []struct {
		name     string
		ttl      time.Duration
		expected string
	}{
		{"Zero TTL (use default)", 0, "should use default TTL"},
		{"Short TTL", 1 * time.Minute, "short TTL"},
		{"Medium TTL", 30 * time.Minute, "medium TTL"},
		{"Long TTL", 24 * time.Hour, "long TTL"},
		{"Very Long TTL", 7 * 24 * time.Hour, "very long TTL"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &models.LLMRequest{
				ID:          "test-request-" + tc.name,
				Prompt:      "test prompt for " + tc.name,
				RequestType: "completion",
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					MaxTokens:   100,
					Temperature: 0.7,
				},
			}

			resp := &models.LLMResponse{
				ID:           "test-response-" + tc.name,
				RequestID:    "test-request-" + tc.name,
				ProviderName: "test-provider",
				Content:      "test response for " + tc.name,
				Confidence:   0.95,
				TokensUsed:   50,
			}

			// Set with specific TTL
			err := service.SetLLMResponse(ctx, req, resp, tc.ttl)
			assert.NoError(t, err) // Should return nil when cache is disabled

			// Get should fail (cache disabled)
			retrievedResp, err := service.GetLLMResponse(ctx, req)
			assert.Error(t, err)
			assert.Nil(t, retrievedResp)
		})
	}
}

func TestCacheService_MemorySourcesTTL(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// Test memory sources with different TTLs
	query := "test query"
	dataset := "test-dataset"

	sources := []models.MemorySource{
		{
			DatasetName:    dataset,
			Content:        "test content 1",
			RelevanceScore: 0.8,
			SourceType:     "document",
		},
		{
			DatasetName:    dataset,
			Content:        "test content 2",
			RelevanceScore: 0.9,
			SourceType:     "web",
		},
	}

	// Test with zero TTL (should use default)
	err = service.SetMemorySources(ctx, query, dataset, sources, 0)
	assert.NoError(t, err)

	// Test with custom TTL
	err = service.SetMemorySources(ctx, query+"-custom", dataset, sources, 10*time.Minute)
	assert.NoError(t, err)

	// Get should fail (cache disabled)
	retrievedSources, err := service.GetMemorySources(ctx, query, dataset)
	assert.Error(t, err)
	assert.Nil(t, retrievedSources)
}

func TestCacheService_ProviderHealthTTL(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// Test provider health with different TTLs
	providerName := "test-provider"
	healthData := map[string]interface{}{
		"status":    "healthy",
		"latency":   50.5,
		"timestamp": time.Now().Unix(),
		"errors":    0,
	}

	// Test with zero TTL (should use health-specific default: 5 minutes)
	err = service.SetProviderHealth(ctx, providerName, healthData, 0)
	assert.NoError(t, err)

	// Test with custom TTL
	err = service.SetProviderHealth(ctx, providerName+"-custom", healthData, 2*time.Minute)
	assert.NoError(t, err)

	// Get should fail (cache disabled)
	retrievedHealth, err := service.GetProviderHealth(ctx, providerName)
	assert.Error(t, err)
	assert.Nil(t, retrievedHealth)
}

func TestCacheService_UserSessionTTL(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// Test user session with different TTLs
	session := &models.UserSession{
		ID:           "test-session-id",
		UserID:       "test-user-id",
		SessionToken: "test-session-token",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	// Test with zero TTL (should use session default: 24 hours)
	err = service.SetUserSession(ctx, session, 0)
	assert.NoError(t, err)

	// Test with custom TTL
	session2 := &models.UserSession{
		ID:           "test-session-id-2",
		UserID:       "test-user-id-2",
		SessionToken: "test-session-token-2",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CreatedAt:    time.Now(),
	}
	err = service.SetUserSession(ctx, session2, 48*time.Hour)
	assert.NoError(t, err)

	// Get should fail (cache disabled)
	retrievedSession, err := service.GetUserSession(ctx, session.SessionToken)
	assert.Error(t, err)
	assert.Nil(t, retrievedSession)
}

func TestCacheService_APIKeyTTL(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// Test API key with different TTLs
	apiKey := "test-api-key-12345"
	keyInfo := map[string]interface{}{
		"user_id":    "test-user-id",
		"rate_limit": 1000,
		"expires_at": time.Now().Add(30 * 24 * time.Hour).Unix(),
	}

	// Test with zero TTL (should use API key default: 1 hour)
	err = service.SetAPIKey(ctx, apiKey, keyInfo, 0)
	assert.NoError(t, err)

	// Test with custom TTL
	apiKey2 := "test-api-key-67890"
	keyInfo2 := map[string]interface{}{
		"user_id":    "test-user-id-2",
		"rate_limit": 2000,
		"expires_at": time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	err = service.SetAPIKey(ctx, apiKey2, keyInfo2, 2*time.Hour)
	assert.NoError(t, err)

	// Get should fail (cache disabled)
	retrievedKeyInfo, err := service.GetAPIKey(ctx, apiKey)
	assert.Error(t, err)
	assert.Nil(t, retrievedKeyInfo)
}

func TestCacheService_CacheInvalidationPatterns(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// Test various cache invalidation scenarios

	// 1. Test InvalidateUserCache (currently no-op implementation)
	err = service.InvalidateUserCache(ctx, "test-user-id")
	assert.NoError(t, err)

	// 2. Test ClearExpired (currently no-op implementation)
	err = service.ClearExpired(ctx)
	assert.NoError(t, err)

	// 3. Test that setting new value with same key would overwrite old value
	// (can't test without Redis, but we can verify the method signatures)
	req := &models.LLMRequest{
		ID:          "test-request-id",
		Prompt:      "original prompt",
		RequestType: "completion",
		ModelParams: models.ModelParameters{
			Model:       "test-model",
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}

	resp1 := &models.LLMResponse{
		ID:           "test-response-1",
		RequestID:    "test-request-id",
		ProviderName: "test-provider",
		Content:      "original response",
		Confidence:   0.95,
		TokensUsed:   50,
	}

	resp2 := &models.LLMResponse{
		ID:           "test-response-2",
		RequestID:    "test-request-id",
		ProviderName: "test-provider",
		Content:      "updated response",
		Confidence:   0.98,
		TokensUsed:   60,
	}

	// Set first response
	err = service.SetLLMResponse(ctx, req, resp1, 5*time.Minute)
	assert.NoError(t, err)

	// Set second response (would overwrite first if cache was enabled)
	err = service.SetLLMResponse(ctx, req, resp2, 5*time.Minute)
	assert.NoError(t, err)

	// Note: Without Redis running, we can't test actual cache invalidation
	// These tests verify the API contracts and error handling
}

func TestCacheService_ErrorHandling(t *testing.T) {
	// Test error handling in cache operations

	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// Test with nil request
	var nilReq *models.LLMRequest
	resp := &models.LLMResponse{
		ID:           "test-response-id",
		RequestID:    "test-request-id",
		ProviderName: "test-provider",
		Content:      "test response",
		Confidence:   0.95,
		TokensUsed:   50,
	}

	// Set with nil request should work (cache disabled)
	err = service.SetLLMResponse(ctx, nilReq, resp, 5*time.Minute)
	assert.NoError(t, err)

	// Get with nil request should return error
	retrievedResp, err := service.GetLLMResponse(ctx, nilReq)
	assert.Error(t, err)
	assert.Nil(t, retrievedResp)

	// Test with empty query
	emptySources, err := service.GetMemorySources(ctx, "", "test-dataset")
	assert.Error(t, err)
	assert.Nil(t, emptySources)

	// Test with empty dataset
	emptyDatasetSources, err := service.GetMemorySources(ctx, "test query", "")
	assert.Error(t, err)
	assert.Nil(t, emptyDatasetSources)

	// Test with empty provider name
	emptyHealth, err := service.GetProviderHealth(ctx, "")
	assert.Error(t, err)
	assert.Nil(t, emptyHealth)

	// Test with empty session token
	emptySession, err := service.GetUserSession(ctx, "")
	assert.Error(t, err)
	assert.Nil(t, emptySession)

	// Test with empty API key
	emptyKeyInfo, err := service.GetAPIKey(ctx, "")
	assert.Error(t, err)
	assert.Nil(t, emptyKeyInfo)
}
