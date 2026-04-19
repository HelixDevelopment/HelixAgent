package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
)

// setupCacheServiceWithMiniRedis creates a CacheService with a working miniredis backend
func setupCacheServiceWithMiniRedis(t *testing.T) (*CacheService, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	service := &CacheService{
		redisClient: &RedisClient{client: client},
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    safe.NewStore[string, map[string]struct{}](),
	}

	t.Cleanup(func() {
		_ = service.Close()
		mr.Close()
	})

	return service, mr
}

// ============================================================================
// CacheService Unit Tests with Working Redis Mock
// ============================================================================

func TestCacheService_GetLLMResponse_Enabled(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	req := &models.LLMRequest{
		ID:     "test-1",
		Prompt: "Hello, world!",
		ModelParams: models.ModelParameters{
			Model:       "gpt-4",
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}

	// First, cache miss
	resp, err := service.GetLLMResponse(ctx, req)
	assert.Error(t, err) // Redis nil error
	assert.Nil(t, resp)

	// Set a response
	cachedResp := &models.LLMResponse{
		ID:           "resp-1",
		RequestID:    "test-1",
		Content:      "Hello back!",
		ProviderName: "openai",
		Confidence:   0.95,
		TokensUsed:   20,
	}

	err = service.SetLLMResponse(ctx, req, cachedResp, 5*time.Minute)
	require.NoError(t, err)

	// Now should be a cache hit
	resp, err = service.GetLLMResponse(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Hello back!", resp.Content)
	assert.Equal(t, "openai", resp.ProviderName)
}

func TestCacheService_SetLLMResponse_DefaultTTL(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	req := &models.LLMRequest{
		ID:     "test-2",
		Prompt: "Test prompt",
		ModelParams: models.ModelParameters{
			Model: "claude-3",
		},
	}
	resp := &models.LLMResponse{
		Content: "Test response",
	}

	// Set with zero TTL (should use default)
	err := service.SetLLMResponse(ctx, req, resp, 0)
	require.NoError(t, err)

	// Verify it was stored
	result, err := service.GetLLMResponse(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "Test response", result.Content)
}

func TestCacheService_GetMemorySources_Enabled(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	query := "test query for memory"
	dataset := "test-dataset"

	// First, cache miss
	sources, err := service.GetMemorySources(ctx, query, dataset)
	assert.Error(t, err)
	assert.Nil(t, sources)

	// Set memory sources
	testSources := []models.MemorySource{
		{
			DatasetName:    dataset,
			Content:        "Memory content 1",
			RelevanceScore: 0.9,
			SourceType:     "document",
		},
		{
			DatasetName:    dataset,
			Content:        "Memory content 2",
			RelevanceScore: 0.85,
			SourceType:     "web",
		},
	}

	err = service.SetMemorySources(ctx, query, dataset, testSources, 10*time.Minute)
	require.NoError(t, err)

	// Now should be a cache hit
	sources, err = service.GetMemorySources(ctx, query, dataset)
	require.NoError(t, err)
	require.Len(t, sources, 2)
	assert.Equal(t, "Memory content 1", sources[0].Content)
	assert.Equal(t, 0.9, sources[0].RelevanceScore)
}

func TestCacheService_SetMemorySources_DefaultTTL(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	sources := []models.MemorySource{
		{Content: "test"},
	}

	// Set with zero TTL (should use default)
	err := service.SetMemorySources(ctx, "query", "dataset", sources, 0)
	require.NoError(t, err)
}

func TestCacheService_GetProviderHealth_Enabled(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	providerName := "openai"

	// First, cache miss
	health, err := service.GetProviderHealth(ctx, providerName)
	assert.Error(t, err)
	assert.Nil(t, health)

	// Set health data
	healthData := map[string]interface{}{
		"status":     "healthy",
		"latency_ms": 150.5,
		"timestamp":  time.Now().Unix(),
		"errors":     0,
	}

	err = service.SetProviderHealth(ctx, providerName, healthData, 5*time.Minute)
	require.NoError(t, err)

	// Now should be a cache hit
	health, err = service.GetProviderHealth(ctx, providerName)
	require.NoError(t, err)
	require.NotNil(t, health)
	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, float64(0), health["errors"])
}

func TestCacheService_SetProviderHealth_DefaultTTL(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	healthData := map[string]interface{}{
		"status": "healthy",
	}

	// Set with zero TTL (should use 5 minute default for health)
	err := service.SetProviderHealth(ctx, "provider", healthData, 0)
	require.NoError(t, err)
}

func TestCacheService_GetUserSession_Enabled(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	sessionToken := "test-session-token-abc123"

	// First, cache miss
	session, err := service.GetUserSession(ctx, sessionToken)
	assert.Error(t, err)
	assert.Nil(t, session)

	// Set session
	testSession := &models.UserSession{
		ID:           "session-1",
		UserID:       "user-123",
		SessionToken: sessionToken,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	err = service.SetUserSession(ctx, testSession, 24*time.Hour)
	require.NoError(t, err)

	// Now should be a cache hit
	session, err = service.GetUserSession(ctx, sessionToken)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, "session-1", session.ID)
	assert.Equal(t, "user-123", session.UserID)
}

func TestCacheService_SetUserSession_DefaultTTL(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	session := &models.UserSession{
		ID:           "session-2",
		UserID:       "user-456",
		SessionToken: "token-2",
	}

	// Set with zero TTL (should use 24 hour default)
	err := service.SetUserSession(ctx, session, 0)
	require.NoError(t, err)

	// Verify key was tracked for user
	assert.Equal(t, 1, service.GetUserKeyCount("user-456"))
}

func TestCacheService_SetUserSession_EmptyUserID(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	session := &models.UserSession{
		ID:           "session-no-user",
		UserID:       "", // Empty user ID
		SessionToken: "token-no-user",
	}

	err := service.SetUserSession(ctx, session, time.Hour)
	require.NoError(t, err)

	// No key should be tracked (no user ID)
	assert.Equal(t, 0, service.GetUserKeyCount(""))
}

func TestCacheService_GetAPIKey_Enabled(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	apiKey := "sk-test-api-key-12345"

	// First, cache miss
	keyInfo, err := service.GetAPIKey(ctx, apiKey)
	assert.Error(t, err)
	assert.Nil(t, keyInfo)

	// Set API key info
	testKeyInfo := map[string]interface{}{
		"user_id":    "user-789",
		"rate_limit": 1000,
		"expires_at": time.Now().Add(30 * 24 * time.Hour).Unix(),
		"tier":       "premium",
	}

	err = service.SetAPIKey(ctx, apiKey, testKeyInfo, time.Hour)
	require.NoError(t, err)

	// Now should be a cache hit
	keyInfo, err = service.GetAPIKey(ctx, apiKey)
	require.NoError(t, err)
	require.NotNil(t, keyInfo)
	assert.Equal(t, "user-789", keyInfo["user_id"])
	assert.Equal(t, float64(1000), keyInfo["rate_limit"])
}

func TestCacheService_SetAPIKey_DefaultTTL(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	keyInfo := map[string]interface{}{
		"user_id": "test",
	}

	// Set with zero TTL (should use 1 hour default)
	err := service.SetAPIKey(ctx, "api-key", keyInfo, 0)
	require.NoError(t, err)
}

func TestCacheService_InvalidateUserCache_Enabled(t *testing.T) {
	service, mr := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	userID := "user-to-invalidate"

	// Set up some user data
	err := service.SetUserData(ctx, userID, "preferences", map[string]string{"theme": "dark"}, time.Hour)
	require.NoError(t, err)
	err = service.SetUserData(ctx, userID, "settings", map[string]int{"volume": 80}, time.Hour)
	require.NoError(t, err)

	// Also set some data with user prefix directly
	_ = mr.Set("user:"+userID+":other", "data")

	// Verify keys are tracked
	assert.Equal(t, 2, service.GetUserKeyCount(userID))

	// Invalidate user cache
	err = service.InvalidateUserCache(ctx, userID)
	require.NoError(t, err)

	// Tracking should be cleared
	assert.Equal(t, 0, service.GetUserKeyCount(userID))
}

func TestCacheService_InvalidateUserCache_EmptyUserID(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	err := service.InvalidateUserCache(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "userID cannot be empty")
}

func TestCacheService_SetUserData_Enabled(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	userID := "user-data-test"
	dataKey := "preferences"
	value := map[string]interface{}{
		"theme":    "dark",
		"language": "en",
		"timezone": "UTC",
	}

	// Set user data
	err := service.SetUserData(ctx, userID, dataKey, value, time.Hour)
	require.NoError(t, err)

	// Verify key was tracked
	assert.Equal(t, 1, service.GetUserKeyCount(userID))

	// Get user data
	var result map[string]interface{}
	err = service.GetUserData(ctx, userID, dataKey, &result)
	require.NoError(t, err)
	assert.Equal(t, "dark", result["theme"])
}

func TestCacheService_SetUserData_DefaultTTL(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	// Set with zero TTL (should use default)
	err := service.SetUserData(ctx, "user", "key", "value", 0)
	require.NoError(t, err)
}

func TestCacheService_SetUserData_EmptyUserID(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	err := service.SetUserData(ctx, "", "key", "value", time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "userID cannot be empty")
}

func TestCacheService_GetUserData_EmptyUserID(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	var result string
	err := service.GetUserData(ctx, "", "key", &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "userID cannot be empty")
}

func TestCacheService_DeleteUserData_Enabled(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	userID := "user-delete-test"
	dataKey := "to-delete"

	// Set user data
	err := service.SetUserData(ctx, userID, dataKey, "value", time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, service.GetUserKeyCount(userID))

	// Delete user data
	err = service.DeleteUserData(ctx, userID, dataKey)
	require.NoError(t, err)

	// Key should be untracked
	assert.Equal(t, 0, service.GetUserKeyCount(userID))

	// Data should be gone
	var result string
	err = service.GetUserData(ctx, userID, dataKey, &result)
	assert.Error(t, err) // Should not find it
}

func TestCacheService_DeleteUserData_EmptyUserID(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	err := service.DeleteUserData(ctx, "", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "userID cannot be empty")
}

func TestCacheService_ClearExpired_Enabled(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	// ClearExpired is a no-op (Redis handles it)
	err := service.ClearExpired(ctx)
	assert.NoError(t, err)
}

func TestCacheService_GetStats_Enabled(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	stats := service.GetStats(ctx)
	require.NotNil(t, stats)

	assert.True(t, stats["enabled"].(bool))
	assert.Equal(t, "connected", stats["status"])
	assert.Contains(t, stats, "default_ttl")
	assert.Contains(t, stats, "redis_info")
}

func TestCacheService_GetHitCount_Enabled(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	// Set up a cache entry
	req := &models.LLMRequest{
		Prompt: "hit count test",
		ModelParams: models.ModelParameters{
			Model: "test",
		},
	}
	resp := &models.LLMResponse{Content: "response"}

	err := service.SetLLMResponse(ctx, req, resp, time.Hour)
	require.NoError(t, err)

	// Get the response a few times to increment hit count
	for i := 0; i < 3; i++ {
		_, _ = service.GetLLMResponse(ctx, req)
	}

	// Get hit count for the key
	key := service.generateCacheKey(req)
	count, err := service.GetHitCount(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestCacheService_Close_Enabled(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)

	err := service.Close()
	assert.NoError(t, err)

	// Double close should be safe
	err = service.Close()
	assert.Error(t, err) // Already closed
}

func TestCacheService_GenerateCacheKey_Various(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)

	testCases := []struct {
		name string
		req  *models.LLMRequest
	}{
		{
			name: "Basic request",
			req: &models.LLMRequest{
				Prompt: "Hello",
				ModelParams: models.ModelParameters{
					Model:       "gpt-4",
					Temperature: 0.7,
				},
			},
		},
		{
			name: "Request with messages",
			req: &models.LLMRequest{
				Messages: []models.Message{
					{Role: "user", Content: "Hello"},
					{Role: "assistant", Content: "Hi!"},
				},
				ModelParams: models.ModelParameters{
					Model:     "claude-3",
					MaxTokens: 1000,
				},
			},
		},
		{
			name: "Request with stop sequences",
			req: &models.LLMRequest{
				Prompt: "Generate code:",
				ModelParams: models.ModelParameters{
					Model:         "codex",
					StopSequences: []string{"\n\n", "```"},
				},
			},
		},
		{
			name: "Request with all parameters",
			req: &models.LLMRequest{
				Prompt: "Complete prompt",
				Messages: []models.Message{
					{Role: "system", Content: "You are helpful"},
				},
				ModelParams: models.ModelParameters{
					Model:         "gpt-4",
					MaxTokens:     500,
					Temperature:   0.5,
					TopP:          0.9,
					StopSequences: []string{"END"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := service.generateCacheKey(tc.req)
			assert.NotEmpty(t, key)
			assert.Contains(t, key, "llm:")

			// Same request should produce same key
			key2 := service.generateCacheKey(tc.req)
			assert.Equal(t, key, key2)
		})
	}
}

func TestCacheService_GenerateCacheKey_Uniqueness(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)

	req1 := &models.LLMRequest{
		Prompt:      "Same prompt",
		ModelParams: models.ModelParameters{Model: "gpt-4", Temperature: 0.5},
	}
	req2 := &models.LLMRequest{
		Prompt:      "Same prompt",
		ModelParams: models.ModelParameters{Model: "gpt-4", Temperature: 0.7}, // Different temp
	}
	req3 := &models.LLMRequest{
		Prompt:      "Same prompt",
		ModelParams: models.ModelParameters{Model: "claude-3", Temperature: 0.5}, // Different model
	}
	req4 := &models.LLMRequest{
		Prompt:      "Different prompt",
		ModelParams: models.ModelParameters{Model: "gpt-4", Temperature: 0.5}, // Different prompt
	}

	key1 := service.generateCacheKey(req1)
	key2 := service.generateCacheKey(req2)
	key3 := service.generateCacheKey(req3)
	key4 := service.generateCacheKey(req4)

	assert.NotEqual(t, key1, key2, "Different temperatures should have different keys")
	assert.NotEqual(t, key1, key3, "Different models should have different keys")
	assert.NotEqual(t, key1, key4, "Different prompts should have different keys")
}

func TestCacheService_HashString(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)

	testCases := []string{
		"",
		"hello",
		"Hello, World!",
		"This is a longer string to test hashing",
		`{"key": "value", "nested": {"a": 1}}`,
		"Unicode: \u4e2d\u6587",
	}

	for _, input := range testCases {
		t.Run(fmt.Sprintf("hash_%s", input[:min(10, len(input))]), func(t *testing.T) {
			hash := service.hashString(input)
			assert.NotEmpty(t, hash)
			assert.Len(t, hash, 16) // FNV-64a produces 16 hex chars

			// Same input produces same hash
			hash2 := service.hashString(input)
			assert.Equal(t, hash, hash2)
		})
	}

	// Different inputs produce different hashes
	h1 := service.hashString("input1")
	h2 := service.hashString("input2")
	assert.NotEqual(t, h1, h2)
}

func TestCacheService_TrackUserKey_EdgeCases(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)

	// Empty userID should not track
	service.trackUserKey("", "key")
	assert.Equal(t, 0, service.GetUserKeyCount(""))

	// Empty cacheKey should not track
	service.trackUserKey("user", "")
	assert.Equal(t, 0, service.GetUserKeyCount("user"))

	// Both empty should not track
	service.trackUserKey("", "")
	assert.Equal(t, 0, service.GetUserKeyCount(""))

	// Valid tracking
	service.trackUserKey("user-1", "key-1")
	assert.Equal(t, 1, service.GetUserKeyCount("user-1"))

	// Duplicate key should not increase count
	service.trackUserKey("user-1", "key-1")
	assert.Equal(t, 1, service.GetUserKeyCount("user-1"))

	// Multiple keys for same user
	service.trackUserKey("user-1", "key-2")
	service.trackUserKey("user-1", "key-3")
	assert.Equal(t, 3, service.GetUserKeyCount("user-1"))
}

func TestCacheService_UntrackUserKey_EdgeCases(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)

	// Set up some keys
	service.trackUserKey("user-1", "key-1")
	service.trackUserKey("user-1", "key-2")

	// Empty userID should not affect anything
	service.untrackUserKey("", "key-1")
	assert.Equal(t, 2, service.GetUserKeyCount("user-1"))

	// Empty cacheKey should not affect anything
	service.untrackUserKey("user-1", "")
	assert.Equal(t, 2, service.GetUserKeyCount("user-1"))

	// Untrack non-existent key
	service.untrackUserKey("user-1", "non-existent")
	assert.Equal(t, 2, service.GetUserKeyCount("user-1"))

	// Untrack for non-existent user
	service.untrackUserKey("non-existent-user", "key-1")
	assert.Equal(t, 2, service.GetUserKeyCount("user-1"))

	// Valid untrack
	service.untrackUserKey("user-1", "key-1")
	assert.Equal(t, 1, service.GetUserKeyCount("user-1"))

	// Untrack last key should remove user from map
	service.untrackUserKey("user-1", "key-2")
	assert.Equal(t, 0, service.GetUserKeyCount("user-1"))
}

func TestCacheService_GetUserKeyCount_NonExistent(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)

	count := service.GetUserKeyCount("non-existent-user")
	assert.Equal(t, 0, count)
}

func TestCacheService_IncrementHitCount(t *testing.T) {
	service, mr := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	key := "test-key"
	hitKey := "hits:" + key

	// Increment hit count
	service.incrementHitCount(ctx, key)
	service.incrementHitCount(ctx, key)
	service.incrementHitCount(ctx, key)

	// Verify in redis
	val, err := mr.Get(hitKey)
	require.NoError(t, err)
	assert.Equal(t, "3", val)
}

func TestCacheService_DeleteByPattern(t *testing.T) {
	service, mr := setupCacheServiceWithMiniRedis(t)
	ctx := context.Background()

	// Set up some keys with pattern
	_ = mr.Set("user:test:key1", "value1")
	_ = mr.Set("user:test:key2", "value2")
	_ = mr.Set("user:test:key3", "value3")
	_ = mr.Set("other:key", "other")

	// Delete by pattern
	err := service.deleteByPattern(ctx, "user:test:*")
	require.NoError(t, err)

	// user:test:* keys should be deleted
	assert.False(t, mr.Exists("user:test:key1"))
	assert.False(t, mr.Exists("user:test:key2"))
	assert.False(t, mr.Exists("user:test:key3"))

	// other:key should still exist
	assert.True(t, mr.Exists("other:key"))
}

func TestCacheService_DeleteByPattern_NilClient(t *testing.T) {
	service := &CacheService{
		redisClient: nil,
		enabled:     true,
		userKeys:    safe.NewStore[string, map[string]struct{}](),
	}
	ctx := context.Background()

	err := service.deleteByPattern(ctx, "pattern:*")
	assert.NoError(t, err)
}

func TestCacheService_DeleteByPattern_NilInnerClient(t *testing.T) {
	service := &CacheService{
		redisClient: &RedisClient{client: nil},
		enabled:     true,
		userKeys:    safe.NewStore[string, map[string]struct{}](),
	}
	ctx := context.Background()

	err := service.deleteByPattern(ctx, "pattern:*")
	assert.NoError(t, err)
}

func TestCacheService_ConcurrentUserKeyTracking(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)

	userID := "concurrent-user"
	numGoroutines := 50
	keysPerGoroutine := 10

	done := make(chan bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(gid int) {
			for j := 0; j < keysPerGoroutine; j++ {
				key := fmt.Sprintf("key-%d-%d", gid, j)
				service.trackUserKey(userID, key)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// All unique keys should be tracked
	expectedCount := numGoroutines * keysPerGoroutine
	assert.Equal(t, expectedCount, service.GetUserKeyCount(userID))
}

// TestCacheService_UserKeys_RaceFree is the CONST-029 verification
// test for the nested-map COW migration. Readers taking a Snapshot of
// a user's key set while writers concurrently trackUserKey/
// untrackUserKey must not trip the race detector, and the snapshot
// must be stable for its reader (no mutation under the reader's feet).
func TestCacheService_UserKeys_RaceFree(t *testing.T) {
	service, _ := setupCacheServiceWithMiniRedis(t)

	const writers = 8
	const readers = 16
	const iterations = 500

	var wg sync.WaitGroup

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			userID := fmt.Sprintf("user-%d", base%4) // 4 overlapping users
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("k-%d-%d", base, i)
				service.trackUserKey(userID, key)
				if i%3 == 0 && i > 0 {
					service.untrackUserKey(userID, fmt.Sprintf("k-%d-%d", base, i-1))
				}
			}
		}(w)
	}

	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			userID := fmt.Sprintf("user-%d", idx%4)
			for i := 0; i < iterations; i++ {
				// GetUserKeyCount internally does a Get then len — the
				// returned map must be a frozen snapshot so iterating
				// it concurrently with other writers is safe.
				_ = service.GetUserKeyCount(userID)
			}
		}(r)
	}

	wg.Wait()
}

func TestNewCacheConfig_Values(t *testing.T) {
	cfg := NewCacheConfig()

	require.NotNil(t, cfg)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 30*time.Minute, cfg.DefaultTTL)
	assert.Nil(t, cfg.Redis)
}

func TestCacheKey_Struct(t *testing.T) {
	key := CacheKey{
		Type:      "llm",
		ID:        "req-123",
		Provider:  "openai",
		UserID:    "user-456",
		SessionID: "sess-789",
	}

	assert.Equal(t, "llm", key.Type)
	assert.Equal(t, "req-123", key.ID)
	assert.Equal(t, "openai", key.Provider)
	assert.Equal(t, "user-456", key.UserID)
	assert.Equal(t, "sess-789", key.SessionID)
}

func TestCacheEntry_Struct(t *testing.T) {
	now := time.Now()
	expires := now.Add(time.Hour)

	entry := CacheEntry{
		Key:       "test-key",
		Value:     map[string]string{"data": "value"},
		CreatedAt: now,
		ExpiresAt: expires,
		HitCount:  42,
	}

	assert.Equal(t, "test-key", entry.Key)
	assert.NotNil(t, entry.Value)
	assert.Equal(t, now, entry.CreatedAt)
	assert.Equal(t, expires, entry.ExpiresAt)
	assert.Equal(t, int64(42), entry.HitCount)
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- Benchmarks ---

func BenchmarkCacheService_HashString(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	service := &CacheService{
		redisClient: &RedisClient{client: client},
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    safe.NewStore[string, map[string]struct{}](),
	}

	inputs := []string{
		"hello",
		"Hello, World!",
		"This is a longer string to test hashing performance",
		`{"key": "value", "nested": {"a": 1}}`,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.hashString(inputs[i%len(inputs)])
	}
}

func BenchmarkCacheService_GenerateCacheKey(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	service := &CacheService{
		redisClient: &RedisClient{client: client},
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    safe.NewStore[string, map[string]struct{}](),
	}

	reqs := []*models.LLMRequest{
		{
			Prompt:      "Hello",
			ModelParams: models.ModelParameters{Model: "gpt-4", Temperature: 0.7},
		},
		{
			Messages:    []models.Message{{Role: "user", Content: "Hello"}},
			ModelParams: models.ModelParameters{Model: "claude-3", MaxTokens: 1000},
		},
		{
			Prompt:      "Generate code:",
			ModelParams: models.ModelParameters{Model: "codex", StopSequences: []string{"\n\n", "```"}},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.generateCacheKey(reqs[i%len(reqs)])
	}
}

func BenchmarkCacheService_TrackUserKey(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	service := &CacheService{
		redisClient: &RedisClient{client: client},
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    safe.NewStore[string, map[string]struct{}](),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		service.trackUserKey("bench-user", key)
	}
}

func BenchmarkCacheService_GetUserKeyCount(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	service := &CacheService{
		redisClient: &RedisClient{client: client},
		enabled:     true,
		defaultTTL:  30 * time.Minute,
		userKeys:    safe.NewStore[string, map[string]struct{}](),
	}

	// Pre-populate
	for i := 0; i < 100; i++ {
		service.trackUserKey("bench-user", fmt.Sprintf("key-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.GetUserKeyCount("bench-user")
	}
}
