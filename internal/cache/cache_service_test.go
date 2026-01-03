package cache

import (
	"context"
	"fmt"
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
			Host:     "nonexistent.redis.host",
			Port:     "6379",
			Password: "",
			DB:       0,
			PoolSize: 10,
			Timeout:  1 * time.Second, // Short timeout for fast test
		},
	}

	service, err := NewCacheService(cfg)

	// When Redis connection fails, we expect an error but service should be created
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

	// Test with config but Redis not accessible (disabled)
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:    "nonexistent.redis.host",
			Port:    "6379",
			Timeout: 1 * time.Second,
		},
	}
	service2, err := NewCacheService(cfg)
	require.Error(t, err) // Connection will fail
	require.NotNil(t, service2)
	assert.False(t, service2.IsEnabled())
}

func TestRedisClient_Operations(t *testing.T) {
	// Test Redis client operations with invalid Redis config
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.redis.host",
			Port: "6379",
		},
	}

	redisClient := NewRedisClient(cfg)
	require.NotNil(t, redisClient)

	ctx := context.Background()

	// Test Ping should fail when Redis is not accessible
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

func TestRedisClient_NilConfig(t *testing.T) {
	// Test Redis client with nil config
	client := NewRedisClient(nil)
	require.NotNil(t, client)

	// Client should be created but not functional
	assert.NotNil(t, client.Client())

	// Test Pipeline
	pipeline := client.Pipeline()
	assert.NotNil(t, pipeline)
}

func TestRedisClient_MGet(t *testing.T) {
	// Test MGet with invalid Redis config
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.redis.host",
			Port: "6379",
		},
	}

	redisClient := NewRedisClient(cfg)
	require.NotNil(t, redisClient)

	ctx := context.Background()

	// Test MGet should fail when Redis is not running
	results, err := redisClient.MGet(ctx, "key1", "key2", "key3")
	require.Error(t, err)
	_ = results // May be nil or empty depending on error type
}

func TestRedisClient_Close(t *testing.T) {
	// Test Close method
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.redis.host",
			Port: "6379",
		},
	}

	redisClient := NewRedisClient(cfg)
	require.NotNil(t, redisClient)

	// Close should not panic
	err := redisClient.Close()
	// Close may or may not return an error depending on Redis client state
	_ = err
}

func TestCacheEntry_Struct(t *testing.T) {
	// Test CacheEntry struct
	entry := CacheEntry{
		Key:       "test-key",
		Value:     "test-value",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
		HitCount:  10,
	}

	assert.Equal(t, "test-key", entry.Key)
	assert.Equal(t, "test-value", entry.Value)
	assert.Equal(t, int64(10), entry.HitCount)
}

func TestCacheKey_Struct(t *testing.T) {
	// Test CacheKey struct
	key := CacheKey{
		Type:      "llm",
		ID:        "request-123",
		Provider:  "openai",
		UserID:    "user-456",
		SessionID: "session-789",
	}

	assert.Equal(t, "llm", key.Type)
	assert.Equal(t, "request-123", key.ID)
	assert.Equal(t, "openai", key.Provider)
	assert.Equal(t, "user-456", key.UserID)
	assert.Equal(t, "session-789", key.SessionID)
}

func TestCacheConfig_Struct(t *testing.T) {
	// Test CacheConfig struct
	cfg := &CacheConfig{
		Enabled:    true,
		DefaultTTL: 1 * time.Hour,
		Redis:      nil,
	}

	assert.True(t, cfg.Enabled)
	assert.Equal(t, 1*time.Hour, cfg.DefaultTTL)
	assert.Nil(t, cfg.Redis)
}

// Tests for user cache invalidation functionality

func TestCacheService_UserKeyTracking(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err) // Connection will fail with nil config
	require.NotNil(t, service)

	// Test trackUserKey functionality
	userID := "test-user-123"
	cacheKey1 := "session:token-abc"
	cacheKey2 := "session:token-def"

	// Initially no keys tracked
	assert.Equal(t, 0, service.GetUserKeyCount(userID))

	// Track first key
	service.trackUserKey(userID, cacheKey1)
	assert.Equal(t, 1, service.GetUserKeyCount(userID))

	// Track second key
	service.trackUserKey(userID, cacheKey2)
	assert.Equal(t, 2, service.GetUserKeyCount(userID))

	// Track duplicate key (should not increase count)
	service.trackUserKey(userID, cacheKey1)
	assert.Equal(t, 2, service.GetUserKeyCount(userID))

	// Test untrackUserKey
	service.untrackUserKey(userID, cacheKey1)
	assert.Equal(t, 1, service.GetUserKeyCount(userID))

	// Untrack non-existent key (should not panic)
	service.untrackUserKey(userID, "non-existent-key")
	assert.Equal(t, 1, service.GetUserKeyCount(userID))

	// Untrack last key
	service.untrackUserKey(userID, cacheKey2)
	assert.Equal(t, 0, service.GetUserKeyCount(userID))
}

func TestCacheService_TrackUserKeyEmptyValues(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err)
	require.NotNil(t, service)

	// Empty userID should not track
	service.trackUserKey("", "some-key")
	assert.Equal(t, 0, service.GetUserKeyCount(""))

	// Empty cacheKey should not track
	service.trackUserKey("user-id", "")
	assert.Equal(t, 0, service.GetUserKeyCount("user-id"))

	// Both empty should not track
	service.trackUserKey("", "")
	assert.Equal(t, 0, service.GetUserKeyCount(""))
}

func TestCacheService_UntrackUserKeyEmptyValues(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err)
	require.NotNil(t, service)

	// Track a key first
	service.trackUserKey("user-id", "some-key")
	assert.Equal(t, 1, service.GetUserKeyCount("user-id"))

	// Untrack with empty userID should not affect anything
	service.untrackUserKey("", "some-key")
	assert.Equal(t, 1, service.GetUserKeyCount("user-id"))

	// Untrack with empty cacheKey should not affect anything
	service.untrackUserKey("user-id", "")
	assert.Equal(t, 1, service.GetUserKeyCount("user-id"))

	// Clean up
	service.untrackUserKey("user-id", "some-key")
	assert.Equal(t, 0, service.GetUserKeyCount("user-id"))
}

func TestCacheService_InvalidateUserCacheEmptyUserID(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err)
	require.NotNil(t, service)

	ctx := context.Background()

	// When cache is disabled, should return nil even for empty userID
	assert.False(t, service.IsEnabled())
	err = service.InvalidateUserCache(ctx, "")
	assert.NoError(t, err)
}

func TestCacheService_SetUserSessionTracksKey(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err)
	require.NotNil(t, service)

	ctx := context.Background()

	session := &models.UserSession{
		ID:           "test-session-id",
		UserID:       "test-user-id",
		SessionToken: "test-session-token",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	// Set should return nil when cache is disabled
	err = service.SetUserSession(ctx, session, 0)
	assert.NoError(t, err)

	// Even when disabled, the key should be tracked in memory
	// This is for consistency when cache becomes enabled
	// Note: When cache is disabled, we don't track keys to avoid memory leaks
	// so the count should be 0
	assert.Equal(t, 0, service.GetUserKeyCount(session.UserID))
}

func TestCacheService_SetUserSessionNilSession(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err)
	require.NotNil(t, service)

	ctx := context.Background()

	// Set with nil session should return nil
	err = service.SetUserSession(ctx, nil, 0)
	assert.NoError(t, err)
}

func TestCacheService_SetUserData(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err)
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	userID := "test-user-123"
	dataKey := "preferences"
	value := map[string]interface{}{
		"theme":    "dark",
		"language": "en",
	}

	// Set should return nil when cache is disabled
	err = service.SetUserData(ctx, userID, dataKey, value, 0)
	assert.NoError(t, err)

	// Get should return error when cache is disabled
	var result map[string]interface{}
	err = service.GetUserData(ctx, userID, dataKey, &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "caching disabled")
}

func TestCacheService_SetUserDataEmptyUserID(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err)
	require.NotNil(t, service)

	ctx := context.Background()

	// SetUserData with empty userID should return nil when disabled
	err = service.SetUserData(ctx, "", "key", "value", 0)
	assert.NoError(t, err)

	// GetUserData with empty userID should return error when disabled
	var result string
	err = service.GetUserData(ctx, "", "key", &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "caching disabled")
}

func TestCacheService_DeleteUserData(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err)
	require.NotNil(t, service)
	assert.False(t, service.IsEnabled())

	ctx := context.Background()

	// DeleteUserData should return nil when cache is disabled
	err = service.DeleteUserData(ctx, "user-id", "key")
	assert.NoError(t, err)

	// DeleteUserData with empty userID should return nil when disabled
	err = service.DeleteUserData(ctx, "", "key")
	assert.NoError(t, err)
}

func TestCacheService_GetUserKeyCountNonExistentUser(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err)
	require.NotNil(t, service)

	// Non-existent user should return 0
	assert.Equal(t, 0, service.GetUserKeyCount("non-existent-user"))
}

func TestCacheService_InvalidateUserCacheClearsTracking(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err)
	require.NotNil(t, service)

	ctx := context.Background()

	userID := "test-user-for-invalidation"

	// Manually track some keys (simulating what would happen if cache was enabled)
	service.trackUserKey(userID, "user:test-user-for-invalidation:key1")
	service.trackUserKey(userID, "user:test-user-for-invalidation:key2")
	service.trackUserKey(userID, "user:test-user-for-invalidation:key3")

	assert.Equal(t, 3, service.GetUserKeyCount(userID))

	// Invalidate user cache (will fail on Redis operations but should clear tracking)
	err = service.InvalidateUserCache(ctx, userID)
	// When disabled, it returns nil immediately
	assert.NoError(t, err)

	// But if we check the user key count, it should still be 3
	// because the cache is disabled and invalidation returns early
	// The tracking is only cleared when cache is enabled
	assert.Equal(t, 3, service.GetUserKeyCount(userID))
}

func TestCacheService_MultipleUsersKeyTracking(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err)
	require.NotNil(t, service)

	user1 := "user-1"
	user2 := "user-2"
	user3 := "user-3"

	// Track keys for multiple users
	service.trackUserKey(user1, "key1")
	service.trackUserKey(user1, "key2")
	service.trackUserKey(user1, "key3")

	service.trackUserKey(user2, "key-a")
	service.trackUserKey(user2, "key-b")

	service.trackUserKey(user3, "key-x")

	// Verify counts
	assert.Equal(t, 3, service.GetUserKeyCount(user1))
	assert.Equal(t, 2, service.GetUserKeyCount(user2))
	assert.Equal(t, 1, service.GetUserKeyCount(user3))

	// Untrack some keys
	service.untrackUserKey(user1, "key2")
	assert.Equal(t, 2, service.GetUserKeyCount(user1))
	assert.Equal(t, 2, service.GetUserKeyCount(user2)) // unchanged
	assert.Equal(t, 1, service.GetUserKeyCount(user3)) // unchanged

	// Untrack for different user should not affect others
	service.untrackUserKey(user2, "key1") // user2 doesn't have key1
	assert.Equal(t, 2, service.GetUserKeyCount(user1))
	assert.Equal(t, 2, service.GetUserKeyCount(user2))
}

func TestCacheService_ConcurrentKeyTracking(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err)
	require.NotNil(t, service)

	userID := "concurrent-test-user"
	numGoroutines := 100
	keysPerGoroutine := 10

	// Track keys concurrently
	done := make(chan bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < keysPerGoroutine; j++ {
				key := fmt.Sprintf("key-%d-%d", goroutineID, j)
				service.trackUserKey(userID, key)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify total count (each goroutine tracks unique keys)
	expectedCount := numGoroutines * keysPerGoroutine
	assert.Equal(t, expectedCount, service.GetUserKeyCount(userID))
}

func TestCacheService_ConcurrentTrackUntrack(t *testing.T) {
	// Create a cache service with nil config (disabled)
	service, err := NewCacheService(nil)
	require.Error(t, err)
	require.NotNil(t, service)

	userID := "concurrent-track-untrack-user"
	numIterations := 50

	// Track and untrack concurrently without race conditions
	done := make(chan bool, 2)

	// Goroutine 1: Tracks keys
	go func() {
		for i := 0; i < numIterations; i++ {
			key := fmt.Sprintf("key-%d", i)
			service.trackUserKey(userID, key)
		}
		done <- true
	}()

	// Goroutine 2: Untracks some keys
	go func() {
		for i := 0; i < numIterations/2; i++ {
			key := fmt.Sprintf("key-%d", i)
			service.untrackUserKey(userID, key)
		}
		done <- true
	}()

	// Wait for completion
	<-done
	<-done

	// Just verify no race condition occurred (count may vary)
	// The important thing is that it doesn't panic
	count := service.GetUserKeyCount(userID)
	assert.GreaterOrEqual(t, count, 0)
}

func TestCacheService_DeleteByPatternNilClient(t *testing.T) {
	// Create a cache service directly with nil redisClient
	service := &CacheService{
		enabled:    true,
		defaultTTL: 30 * time.Minute,
		userKeys:   make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	// deleteByPattern should handle nil client gracefully
	err := service.deleteByPattern(ctx, "user:*")
	assert.NoError(t, err)
}

// ============================================================================
// Tests for enabled cache service with mock/direct struct access
// These tests cover the code paths that are not reached when cache is disabled
// ============================================================================

func TestCacheService_GenerateCacheKeyDirect(t *testing.T) {
	// Create a cache service directly to test private methods
	service := &CacheService{
		enabled:    true,
		defaultTTL: 30 * time.Minute,
		userKeys:   make(map[string]map[string]struct{}),
	}

	// Test generateCacheKey with various requests
	testCases := []struct {
		name string
		req  *models.LLMRequest
	}{
		{
			name: "Basic request",
			req: &models.LLMRequest{
				ID:          "test-1",
				Prompt:      "Hello, world!",
				RequestType: "completion",
				ModelParams: models.ModelParameters{
					Model:       "gpt-4",
					MaxTokens:   100,
					Temperature: 0.7,
				},
			},
		},
		{
			name: "Request with messages",
			req: &models.LLMRequest{
				ID:          "test-2",
				Prompt:      "",
				RequestType: "chat",
				Messages: []models.Message{
					{Role: "user", Content: "Hello"},
					{Role: "assistant", Content: "Hi there!"},
				},
				ModelParams: models.ModelParameters{
					Model:       "claude-3",
					MaxTokens:   500,
					Temperature: 0.5,
					TopP:        0.9,
				},
			},
		},
		{
			name: "Request with stop sequences",
			req: &models.LLMRequest{
				ID:          "test-3",
				Prompt:      "Generate code",
				RequestType: "completion",
				ModelParams: models.ModelParameters{
					Model:         "codex",
					MaxTokens:     1000,
					Temperature:   0.0,
					StopSequences: []string{"\n\n", "END"},
				},
			},
		},
		{
			name: "Empty prompt request",
			req: &models.LLMRequest{
				ID:          "test-4",
				Prompt:      "",
				RequestType: "completion",
				ModelParams: models.ModelParameters{
					Model:       "test-model",
					MaxTokens:   50,
					Temperature: 1.0,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := service.generateCacheKey(tc.req)
			assert.NotEmpty(t, key)
			assert.True(t, len(key) > 4) // "llm:" prefix + hash
			assert.Contains(t, key, "llm:")

			// Same request should generate same key
			key2 := service.generateCacheKey(tc.req)
			assert.Equal(t, key, key2)
		})
	}
}

func TestCacheService_GenerateCacheKeyDeterminism(t *testing.T) {
	service := &CacheService{
		enabled:    true,
		defaultTTL: 30 * time.Minute,
		userKeys:   make(map[string]map[string]struct{}),
	}

	req := &models.LLMRequest{
		ID:          "determinism-test",
		Prompt:      "Test prompt for determinism",
		RequestType: "completion",
		ModelParams: models.ModelParameters{
			Model:       "test-model",
			MaxTokens:   100,
			Temperature: 0.5,
			TopP:        0.95,
		},
	}

	// Generate key multiple times
	keys := make([]string, 10)
	for i := 0; i < 10; i++ {
		keys[i] = service.generateCacheKey(req)
	}

	// All keys should be identical
	for i := 1; i < len(keys); i++ {
		assert.Equal(t, keys[0], keys[i])
	}
}

func TestCacheService_GenerateCacheKeyDifferentRequests(t *testing.T) {
	service := &CacheService{
		enabled:    true,
		defaultTTL: 30 * time.Minute,
		userKeys:   make(map[string]map[string]struct{}),
	}

	req1 := &models.LLMRequest{
		Prompt: "Request 1",
		ModelParams: models.ModelParameters{
			Model:       "model-a",
			Temperature: 0.5,
		},
	}

	req2 := &models.LLMRequest{
		Prompt: "Request 2",
		ModelParams: models.ModelParameters{
			Model:       "model-a",
			Temperature: 0.5,
		},
	}

	req3 := &models.LLMRequest{
		Prompt: "Request 1",
		ModelParams: models.ModelParameters{
			Model:       "model-b",
			Temperature: 0.5,
		},
	}

	req4 := &models.LLMRequest{
		Prompt: "Request 1",
		ModelParams: models.ModelParameters{
			Model:       "model-a",
			Temperature: 0.7,
		},
	}

	key1 := service.generateCacheKey(req1)
	key2 := service.generateCacheKey(req2)
	key3 := service.generateCacheKey(req3)
	key4 := service.generateCacheKey(req4)

	// All should be different due to different prompts/params
	assert.NotEqual(t, key1, key2, "Different prompts should generate different keys")
	assert.NotEqual(t, key1, key3, "Different models should generate different keys")
	assert.NotEqual(t, key1, key4, "Different temperatures should generate different keys")
}

func TestCacheService_HashStringDirect(t *testing.T) {
	service := &CacheService{
		enabled:    true,
		defaultTTL: 30 * time.Minute,
		userKeys:   make(map[string]map[string]struct{}),
	}

	testCases := []struct {
		name  string
		input string
	}{
		{"Empty string", ""},
		{"Simple string", "hello"},
		{"Long string", "This is a very long string that should be hashed consistently"},
		{"Special characters", "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
		{"Unicode", "Hello, 世界! 🌍"},
		{"JSON-like", `{"key": "value", "number": 123}`},
		{"Whitespace", "   spaces   and\ttabs\nand\nnewlines   "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash := service.hashString(tc.input)
			assert.NotEmpty(t, hash)
			assert.Len(t, hash, 32) // MD5 produces 32 hex characters

			// Same input should produce same hash
			hash2 := service.hashString(tc.input)
			assert.Equal(t, hash, hash2)
		})
	}
}

func TestCacheService_HashStringUniqueness(t *testing.T) {
	service := &CacheService{
		enabled:    true,
		defaultTTL: 30 * time.Minute,
		userKeys:   make(map[string]map[string]struct{}),
	}

	inputs := []string{
		"test1",
		"test2",
		"Test1",
		"TEST1",
		"test 1",
		"test",
		"test1 ",
		" test1",
	}

	hashes := make(map[string]bool)
	for _, input := range inputs {
		hash := service.hashString(input)
		assert.False(t, hashes[hash], "Hash collision detected for input: %s", input)
		hashes[hash] = true
	}
}

func TestCacheService_EnabledGetLLMResponse(t *testing.T) {
	// Create enabled cache service with Redis client that will fail
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()
	req := &models.LLMRequest{
		ID:     "test",
		Prompt: "test prompt",
		ModelParams: models.ModelParameters{
			Model:       "test",
			Temperature: 0.5,
		},
	}

	// Should attempt Redis operation and fail
	resp, err := service.GetLLMResponse(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestCacheService_EnabledSetLLMResponse(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()
	req := &models.LLMRequest{
		ID:     "test",
		Prompt: "test prompt",
		ModelParams: models.ModelParameters{
			Model:       "test",
			Temperature: 0.5,
		},
	}
	resp := &models.LLMResponse{
		ID:      "resp-1",
		Content: "test response",
	}

	// Should attempt Redis operation and fail (with default TTL)
	err := service.SetLLMResponse(ctx, req, resp, 0)
	assert.Error(t, err)

	// With explicit TTL
	err = service.SetLLMResponse(ctx, req, resp, 5*time.Minute)
	assert.Error(t, err)
}

func TestCacheService_EnabledGetMemorySources(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	sources, err := service.GetMemorySources(ctx, "test query", "test-dataset")
	assert.Error(t, err)
	assert.Nil(t, sources)
}

func TestCacheService_EnabledSetMemorySources(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()
	sources := []models.MemorySource{
		{Content: "test content"},
	}

	// With default TTL
	err := service.SetMemorySources(ctx, "query", "dataset", sources, 0)
	assert.Error(t, err)

	// With explicit TTL
	err = service.SetMemorySources(ctx, "query", "dataset", sources, 10*time.Minute)
	assert.Error(t, err)
}

func TestCacheService_EnabledGetProviderHealth(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	health, err := service.GetProviderHealth(ctx, "test-provider")
	assert.Error(t, err)
	assert.Nil(t, health)
}

func TestCacheService_EnabledSetProviderHealth(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()
	health := map[string]interface{}{
		"status": "healthy",
	}

	// With default TTL (5 minutes for health)
	err := service.SetProviderHealth(ctx, "provider", health, 0)
	assert.Error(t, err)

	// With explicit TTL
	err = service.SetProviderHealth(ctx, "provider", health, 2*time.Minute)
	assert.Error(t, err)
}

func TestCacheService_EnabledGetUserSession(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	session, err := service.GetUserSession(ctx, "test-token")
	assert.Error(t, err)
	assert.Nil(t, session)
}

func TestCacheService_EnabledSetUserSession(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	// Test with nil session
	err := service.SetUserSession(ctx, nil, 0)
	assert.NoError(t, err)

	// Test with session without userID
	session := &models.UserSession{
		ID:           "session-1",
		UserID:       "",
		SessionToken: "token-abc",
	}
	err = service.SetUserSession(ctx, session, 0)
	assert.Error(t, err) // Redis fails

	// Test with session with userID (tracks key)
	sessionWithUser := &models.UserSession{
		ID:           "session-2",
		UserID:       "user-123",
		SessionToken: "token-def",
	}
	err = service.SetUserSession(ctx, sessionWithUser, 0)
	assert.Error(t, err) // Redis fails but key should be tracked

	// Verify key was tracked
	assert.Equal(t, 1, service.GetUserKeyCount("user-123"))
}

func TestCacheService_EnabledGetAPIKey(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	keyInfo, err := service.GetAPIKey(ctx, "sk-test-key")
	assert.Error(t, err)
	assert.Nil(t, keyInfo)
}

func TestCacheService_EnabledSetAPIKey(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()
	keyInfo := map[string]interface{}{
		"user_id": "user-1",
	}

	// With default TTL (1 hour)
	err := service.SetAPIKey(ctx, "sk-test", keyInfo, 0)
	assert.Error(t, err)

	// With explicit TTL
	err = service.SetAPIKey(ctx, "sk-test", keyInfo, 30*time.Minute)
	assert.Error(t, err)
}

func TestCacheService_EnabledInvalidateUserCache(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	// Test with empty userID
	err := service.InvalidateUserCache(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "userID cannot be empty")

	// Track some keys for a user
	userID := "user-to-invalidate"
	service.trackUserKey(userID, "key1")
	service.trackUserKey(userID, "key2")
	service.trackUserKey(userID, "key3")
	assert.Equal(t, 3, service.GetUserKeyCount(userID))

	// Invalidate - will fail on deleteByPattern but should clear tracked keys
	err = service.InvalidateUserCache(ctx, userID)
	assert.Error(t, err) // Pattern deletion fails on Redis

	// Keys should be cleared from tracking
	assert.Equal(t, 0, service.GetUserKeyCount(userID))
}

func TestCacheService_EnabledInvalidateUserCacheWithEmptyKeySet(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	// User with no tracked keys
	err := service.InvalidateUserCache(ctx, "user-without-keys")
	assert.Error(t, err) // Pattern deletion fails
}

func TestCacheService_EnabledSetUserData(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	// Test with empty userID
	err := service.SetUserData(ctx, "", "key", "value", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "userID cannot be empty")

	// Test with valid userID (with default TTL)
	err = service.SetUserData(ctx, "user-1", "preferences", map[string]string{"theme": "dark"}, 0)
	assert.Error(t, err) // Redis fails

	// Verify key was tracked
	assert.Equal(t, 1, service.GetUserKeyCount("user-1"))

	// Test with explicit TTL
	err = service.SetUserData(ctx, "user-2", "settings", "value", 1*time.Hour)
	assert.Error(t, err) // Redis fails
	assert.Equal(t, 1, service.GetUserKeyCount("user-2"))
}

func TestCacheService_EnabledGetUserData(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	// Test with empty userID
	var result string
	err := service.GetUserData(ctx, "", "key", &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "userID cannot be empty")

	// Test with valid userID
	err = service.GetUserData(ctx, "user-1", "preferences", &result)
	assert.Error(t, err) // Redis fails
}

func TestCacheService_EnabledDeleteUserData(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	// Test with empty userID
	err := service.DeleteUserData(ctx, "", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "userID cannot be empty")

	// Track a key first
	userID := "user-to-delete"
	dataKey := "preferences"
	fullKey := fmt.Sprintf("user:%s:%s", userID, dataKey)
	service.trackUserKey(userID, fullKey)
	assert.Equal(t, 1, service.GetUserKeyCount(userID))

	// Delete should untrack the key
	err = service.DeleteUserData(ctx, userID, dataKey)
	assert.Error(t, err) // Redis fails

	// Key should be untracked
	assert.Equal(t, 0, service.GetUserKeyCount(userID))
}

func TestCacheService_EnabledClearExpired(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	// ClearExpired is a no-op (Redis handles expiration)
	err := service.ClearExpired(ctx)
	assert.NoError(t, err)
}

func TestCacheService_EnabledGetHitCount(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	count, err := service.GetHitCount(ctx, "some-key")
	assert.Error(t, err)
	assert.Equal(t, int64(0), count)
}

func TestCacheService_EnabledGetStats(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	stats := service.GetStats(ctx)
	assert.NotNil(t, stats)
	assert.True(t, stats["enabled"].(bool))
	assert.Equal(t, "connected", stats["status"])
	assert.Contains(t, stats, "default_ttl")
	assert.Contains(t, stats, "redis_info")
}

func TestCacheService_EnabledClose(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	// Close should work even with failed Redis connection
	err := service.Close()
	assert.NoError(t, err)
}

func TestCacheService_CloseWithNilClient(t *testing.T) {
	service := &CacheService{
		redisClient: nil,
		enabled:     false,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	err := service.Close()
	assert.NoError(t, err)
}

func TestCacheService_DeleteByPatternWithRedisClient(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	// Should attempt SCAN and fail
	err := service.deleteByPattern(ctx, "user:test:*")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scan failed")
}

func TestCacheService_IncrementHitCountDirect(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	// Should not panic even if Redis fails
	service.incrementHitCount(ctx, "test-key")
	// No assertion needed - just verify no panic
}

func TestCacheService_GenerateCacheKeyWithNilRequest(t *testing.T) {
	service := &CacheService{
		enabled:    true,
		defaultTTL: 30 * time.Minute,
		userKeys:   make(map[string]map[string]struct{}),
	}

	// This will panic if not handled - catching panic
	defer func() {
		if r := recover(); r != nil {
			// Expected panic with nil request
			t.Log("Recovered from expected panic with nil request")
		}
	}()

	// Nil request will cause panic in JSON marshaling of request fields
	_ = service.generateCacheKey(nil)
}

func TestCacheService_GenerateCacheKeyWithEmptyModelParams(t *testing.T) {
	service := &CacheService{
		enabled:    true,
		defaultTTL: 30 * time.Minute,
		userKeys:   make(map[string]map[string]struct{}),
	}

	req := &models.LLMRequest{
		ID:          "test",
		Prompt:      "test",
		ModelParams: models.ModelParameters{}, // Empty params
	}

	key := service.generateCacheKey(req)
	assert.NotEmpty(t, key)
	assert.Contains(t, key, "llm:")
}

func TestRedisClient_SetWithMarshalError(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "localhost",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	ctx := context.Background()

	// Create a value that cannot be marshaled to JSON
	type unmarshalable struct {
		Ch chan int
	}
	value := unmarshalable{Ch: make(chan int)}

	err := redisClient.Set(ctx, "test-key", value, time.Minute)
	assert.Error(t, err)
	// JSON marshal error for channels
}

func TestCacheService_UserKeyTrackingEdgeCases(t *testing.T) {
	service := &CacheService{
		enabled:    true,
		defaultTTL: 30 * time.Minute,
		userKeys:   make(map[string]map[string]struct{}),
	}

	// Track same key multiple times (should only count once)
	service.trackUserKey("user-1", "key-a")
	service.trackUserKey("user-1", "key-a")
	service.trackUserKey("user-1", "key-a")
	assert.Equal(t, 1, service.GetUserKeyCount("user-1"))

	// Untrack key that doesn't exist for user
	service.untrackUserKey("user-1", "nonexistent-key")
	assert.Equal(t, 1, service.GetUserKeyCount("user-1"))

	// Untrack for user that doesn't exist
	service.untrackUserKey("nonexistent-user", "key-a")

	// Clean up properly
	service.untrackUserKey("user-1", "key-a")
	assert.Equal(t, 0, service.GetUserKeyCount("user-1"))
}

func TestCacheService_GetUserKeyCountThreadSafety(t *testing.T) {
	service := &CacheService{
		enabled:    true,
		defaultTTL: 30 * time.Minute,
		userKeys:   make(map[string]map[string]struct{}),
	}

	userID := "thread-safe-user"
	numGoroutines := 50

	done := make(chan bool, numGoroutines*3)

	// Concurrent tracks
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			service.trackUserKey(userID, fmt.Sprintf("key-%d", id))
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func() {
			_ = service.GetUserKeyCount(userID)
			done <- true
		}()
	}

	// Concurrent untracks
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			service.untrackUserKey(userID, fmt.Sprintf("key-%d", id))
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < numGoroutines*3; i++ {
		<-done
	}

	// No race condition should occur
	count := service.GetUserKeyCount(userID)
	assert.GreaterOrEqual(t, count, 0)
}

func TestCacheService_InvalidateUserCacheDeletesTrackedKeys(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()
	userID := "user-with-many-keys"

	// Track multiple keys
	for i := 0; i < 10; i++ {
		service.trackUserKey(userID, fmt.Sprintf("key-%d", i))
	}
	assert.Equal(t, 10, service.GetUserKeyCount(userID))

	// Invalidate - Redis operations will fail but tracking should be cleared
	_ = service.InvalidateUserCache(ctx, userID)

	// Tracking should be cleared
	assert.Equal(t, 0, service.GetUserKeyCount(userID))
}

func TestCacheService_MultipleTTLDefaults(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "nonexistent.host",
			Port: "6379",
		},
	}
	redisClient := NewRedisClient(cfg)

	service := &CacheService{
		redisClient: redisClient,
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    make(map[string]map[string]struct{}),
	}

	ctx := context.Background()

	// Test default TTL for provider health (5 minutes)
	err := service.SetProviderHealth(ctx, "test", map[string]interface{}{"status": "ok"}, 0)
	assert.Error(t, err) // Redis fails, but code path is exercised

	// Test default TTL for user session (24 hours)
	session := &models.UserSession{
		ID:           "s1",
		UserID:       "u1",
		SessionToken: "t1",
	}
	err = service.SetUserSession(ctx, session, 0)
	assert.Error(t, err) // Redis fails, but code path is exercised

	// Test default TTL for API key (1 hour)
	err = service.SetAPIKey(ctx, "key", map[string]interface{}{"user": "test"}, 0)
	assert.Error(t, err) // Redis fails, but code path is exercised

	// Test default TTL for user data
	err = service.SetUserData(ctx, "user", "pref", "value", 0)
	assert.Error(t, err) // Redis fails, but code path is exercised
}

func TestNewCacheConfig_Values(t *testing.T) {
	cfg := NewCacheConfig()

	assert.NotNil(t, cfg)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 30*time.Minute, cfg.DefaultTTL)
	assert.Nil(t, cfg.Redis)
}

func TestCacheEntry_AllFields(t *testing.T) {
	now := time.Now()
	expires := now.Add(1 * time.Hour)

	entry := CacheEntry{
		Key:       "test-key-123",
		Value:     map[string]interface{}{"data": "test"},
		CreatedAt: now,
		ExpiresAt: expires,
		HitCount:  42,
	}

	assert.Equal(t, "test-key-123", entry.Key)
	assert.NotNil(t, entry.Value)
	assert.Equal(t, now, entry.CreatedAt)
	assert.Equal(t, expires, entry.ExpiresAt)
	assert.Equal(t, int64(42), entry.HitCount)
}

func TestCacheKey_AllFields(t *testing.T) {
	key := CacheKey{
		Type:      "session",
		ID:        "abc-123",
		Provider:  "anthropic",
		UserID:    "user-456",
		SessionID: "sess-789",
	}

	assert.Equal(t, "session", key.Type)
	assert.Equal(t, "abc-123", key.ID)
	assert.Equal(t, "anthropic", key.Provider)
	assert.Equal(t, "user-456", key.UserID)
	assert.Equal(t, "sess-789", key.SessionID)
}

func TestRedisClient_AllMethods(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:     "nonexistent.host",
			Port:     "6379",
			Password: "test-password",
			DB:       1,
		},
	}

	client := NewRedisClient(cfg)
	assert.NotNil(t, client)

	// Test Client() accessor
	assert.NotNil(t, client.Client())

	// Test Pipeline()
	pipeline := client.Pipeline()
	assert.NotNil(t, pipeline)

	// Test Ping (will fail)
	ctx := context.Background()
	err := client.Ping(ctx)
	assert.Error(t, err)

	// Test Set (will fail)
	err = client.Set(ctx, "key", "value", time.Minute)
	assert.Error(t, err)

	// Test Get (will fail)
	var result string
	err = client.Get(ctx, "key", &result)
	assert.Error(t, err)

	// Test Delete (will fail)
	err = client.Delete(ctx, "key")
	assert.Error(t, err)

	// Test MGet (will fail)
	_, err = client.MGet(ctx, "key1", "key2")
	assert.Error(t, err)

	// Test Close
	err = client.Close()
	assert.NoError(t, err)
}
