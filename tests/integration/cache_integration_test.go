// Package integration provides cache service integration tests.
// These tests validate the TieredCache (L1 in-memory + L2 Redis),
// including TTL expiry, prefix invalidation, concurrent access,
// and in-memory-only fallback when Redis is unavailable.
package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/cache"
)

// =============================================================================
// Helpers
// =============================================================================

// skipIfNoRedis skips the test when Redis is not reachable.
func skipIfNoRedis(t *testing.T) {
	t.Helper()
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "16379"
	}
	conn, err := net.DialTimeout("tcp", host+":"+port, 2*time.Second)
	if err != nil {
		t.Skip("Redis not available -- start with: make test-infra-start") // SKIP-OK: #legacy-untriaged
	}
	conn.Close()
}

// newL1OnlyCache creates a TieredCache that uses only L1 (in-memory) with no Redis.
func newL1OnlyCache(t *testing.T) *cache.TieredCache {
	t.Helper()
	cfg := cache.DefaultTieredCacheConfig()
	cfg.EnableL1 = true
	cfg.EnableL2 = false // Disable Redis
	cfg.L1MaxSize = 1000
	cfg.L1TTL = 5 * time.Minute
	cfg.L1CleanupInterval = 500 * time.Millisecond

	tc := cache.NewTieredCache(nil, cfg)
	t.Cleanup(func() {
		_ = tc.Close()
	})
	return tc
}

// =============================================================================
// Test: Cache — Set and Get (round-trip via L1)
// =============================================================================

func TestIntegration_Cache_SetAndGet(t *testing.T) {
	tc := newL1OnlyCache(t)
	ctx := context.Background()

	// Store a string value
	err := tc.Set(ctx, "test:key:1", "hello world", 5*time.Minute)
	require.NoError(t, err)

	var result string
	found, err := tc.Get(ctx, "test:key:1", &result)
	require.NoError(t, err)
	assert.True(t, found, "Key should be found after Set")
	assert.Equal(t, "hello world", result)

	// Store a struct
	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	data := TestData{Name: "cache-test", Value: 42}
	err = tc.Set(ctx, "test:key:struct", data, 5*time.Minute)
	require.NoError(t, err)

	var retrieved TestData
	found, err = tc.Get(ctx, "test:key:struct", &retrieved)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "cache-test", retrieved.Name)
	assert.Equal(t, 42, retrieved.Value)

	// Non-existent key
	var missing string
	found, err = tc.Get(ctx, "test:key:nonexistent", &missing)
	require.NoError(t, err)
	assert.False(t, found, "Non-existent key should not be found")
}

// =============================================================================
// Test: Cache — TTL Expiry
// =============================================================================

func TestIntegration_Cache_TTLExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TTL expiry test in short mode (requires sleep)") // SKIP-OK: #short-mode
	}

	// Create L1-only cache with very short cleanup interval
	cfg := cache.DefaultTieredCacheConfig()
	cfg.EnableL1 = true
	cfg.EnableL2 = false
	cfg.L1MaxSize = 1000
	cfg.L1TTL = 200 * time.Millisecond
	cfg.L1CleanupInterval = 100 * time.Millisecond

	tc := cache.NewTieredCache(nil, cfg)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Store with a very short TTL
	err := tc.Set(ctx, "ttl:expiry:key", "ephemeral", 200*time.Millisecond)
	require.NoError(t, err)

	// Immediately readable
	var result string
	found, err := tc.Get(ctx, "ttl:expiry:key", &result)
	require.NoError(t, err)
	assert.True(t, found, "Key should be found immediately after Set")
	assert.Equal(t, "ephemeral", result)

	// Wait for TTL + cleanup to expire the entry
	time.Sleep(500 * time.Millisecond)

	found, err = tc.Get(ctx, "ttl:expiry:key", &result)
	require.NoError(t, err)
	assert.False(t, found, "Key should have expired after TTL")
}

// =============================================================================
// Test: Cache — Invalidate by Prefix
// =============================================================================

func TestIntegration_Cache_InvalidatePrefix(t *testing.T) {
	tc := newL1OnlyCache(t)
	ctx := context.Background()

	// Store multiple keys with shared prefix
	keys := []string{
		"prefix:alpha:1",
		"prefix:alpha:2",
		"prefix:alpha:3",
		"prefix:beta:1",
		"prefix:beta:2",
	}

	for _, key := range keys {
		err := tc.Set(ctx, key, "value-"+key, 5*time.Minute)
		require.NoError(t, err)
	}

	// Verify all are present
	for _, key := range keys {
		var v string
		found, err := tc.Get(ctx, key, &v)
		require.NoError(t, err)
		assert.True(t, found, "Key %s should be present before invalidation", key)
	}

	// Invalidate only "prefix:alpha:" keys
	count, err := tc.InvalidatePrefix(ctx, "prefix:alpha:")
	require.NoError(t, err)
	assert.Equal(t, 3, count, "Should invalidate 3 alpha-prefixed keys")

	// Alpha keys should be gone
	for _, key := range []string{"prefix:alpha:1", "prefix:alpha:2", "prefix:alpha:3"} {
		var v string
		found, _ := tc.Get(ctx, key, &v)
		assert.False(t, found, "Key %s should be invalidated", key)
	}

	// Beta keys should remain
	for _, key := range []string{"prefix:beta:1", "prefix:beta:2"} {
		var v string
		found, _ := tc.Get(ctx, key, &v)
		assert.True(t, found, "Key %s should still exist after alpha invalidation", key)
	}
}

// =============================================================================
// Test: Cache — Concurrent Read/Write
// =============================================================================

func TestIntegration_Cache_ConcurrentReadWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent cache test in short mode") // SKIP-OK: #short-mode
	}

	tc := newL1OnlyCache(t)
	ctx := context.Background()

	const numWorkers = 20
	const opsPerWorker = 100
	var wg sync.WaitGroup

	// Pre-populate some keys
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("conc:pre:%d", i)
		err := tc.Set(ctx, key, fmt.Sprintf("pre-value-%d", i), 5*time.Minute)
		require.NoError(t, err)
	}

	var writeErrors int64
	var readHits int64
	var readMisses int64
	var mu sync.Mutex

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for op := 0; op < opsPerWorker; op++ {
				if op%2 == 0 {
					// Write operation
					key := fmt.Sprintf("conc:w%d:%d", workerID, op)
					if err := tc.Set(ctx, key, fmt.Sprintf("val-%d-%d", workerID, op), 5*time.Minute); err != nil {
						mu.Lock()
						writeErrors++
						mu.Unlock()
					}
				} else {
					// Read operation (read pre-populated key)
					key := fmt.Sprintf("conc:pre:%d", op%50)
					var val string
					found, _ := tc.Get(ctx, key, &val)
					mu.Lock()
					if found {
						readHits++
					} else {
						readMisses++
					}
					mu.Unlock()
				}
			}
		}(w)
	}

	wg.Wait()

	assert.Equal(t, int64(0), writeErrors, "No write errors should occur under concurrency")
	assert.Greater(t, readHits, int64(0), "Some reads should hit pre-populated keys")

	// Verify final metrics
	metrics := tc.Metrics()
	assert.Greater(t, metrics.L1Hits, int64(0), "L1 should have recorded hits")
}

// =============================================================================
// Test: Cache — Fallback to In-Memory (L2 disabled / Redis unavailable)
// =============================================================================

func TestIntegration_Cache_FallbackToInMemory(t *testing.T) {
	// Create cache with L2 enabled but nil Redis client — simulates Redis unavailable
	cfg := cache.DefaultTieredCacheConfig()
	cfg.EnableL1 = true
	cfg.EnableL2 = true // Enabled but no Redis client
	cfg.L1MaxSize = 500
	cfg.L1TTL = 5 * time.Minute
	cfg.L1CleanupInterval = time.Minute

	tc := cache.NewTieredCache(nil, cfg)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// L1 should still work even when L2 (Redis) is nil
	err := tc.Set(ctx, "fallback:key:1", "in-memory-value", 5*time.Minute)
	require.NoError(t, err)

	var result string
	found, err := tc.Get(ctx, "fallback:key:1", &result)
	require.NoError(t, err)
	assert.True(t, found, "L1 should serve the cached value when L2 is unavailable")
	assert.Equal(t, "in-memory-value", result)

	// Store and retrieve multiple values
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("fallback:batch:%d", i)
		err := tc.Set(ctx, key, i*10, 5*time.Minute)
		require.NoError(t, err)
	}

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("fallback:batch:%d", i)
		var val int
		found, err := tc.Get(ctx, key, &val)
		require.NoError(t, err)
		assert.True(t, found, "Key %s should be in L1", key)
		assert.Equal(t, i*10, val)
	}

	// Metrics should show L1 hits but no L2 hits
	metrics := tc.Metrics()
	assert.Greater(t, metrics.L1Hits, int64(0), "L1 hits should be recorded")
	assert.Equal(t, int64(0), metrics.L2Hits, "L2 hits should be 0 when Redis is unavailable")
}

// =============================================================================
// Test: Cache — Tag-based Invalidation
// =============================================================================

func TestIntegration_Cache_TagInvalidation(t *testing.T) {
	tc := newL1OnlyCache(t)
	ctx := context.Background()

	// Store keys with tags
	err := tc.Set(ctx, "tag:item:1", "value-1", 5*time.Minute, "category:A", "priority:high")
	require.NoError(t, err)
	err = tc.Set(ctx, "tag:item:2", "value-2", 5*time.Minute, "category:A", "priority:low")
	require.NoError(t, err)
	err = tc.Set(ctx, "tag:item:3", "value-3", 5*time.Minute, "category:B", "priority:high")
	require.NoError(t, err)

	// Verify all present
	for _, key := range []string{"tag:item:1", "tag:item:2", "tag:item:3"} {
		var v string
		found, _ := tc.Get(ctx, key, &v)
		assert.True(t, found, "Key %s should exist before tag invalidation", key)
	}

	// Invalidate by tag "category:A" — should remove items 1 and 2
	count, err := tc.InvalidateByTag(ctx, "category:A")
	require.NoError(t, err)
	assert.Equal(t, 2, count, "Should invalidate 2 keys tagged with category:A")

	// Items 1 and 2 should be gone
	var v string
	found, _ := tc.Get(ctx, "tag:item:1", &v)
	assert.False(t, found)
	found, _ = tc.Get(ctx, "tag:item:2", &v)
	assert.False(t, found)

	// Item 3 should remain (only category:B)
	found, _ = tc.Get(ctx, "tag:item:3", &v)
	assert.True(t, found, "Item 3 (category:B) should survive category:A invalidation")
}

// =============================================================================
// Test: Cache — Delete single key
// =============================================================================

func TestIntegration_Cache_Delete(t *testing.T) {
	tc := newL1OnlyCache(t)
	ctx := context.Background()

	err := tc.Set(ctx, "delete:target", "to-be-deleted", 5*time.Minute)
	require.NoError(t, err)

	var v string
	found, _ := tc.Get(ctx, "delete:target", &v)
	assert.True(t, found)

	err = tc.Delete(ctx, "delete:target")
	require.NoError(t, err)

	found, _ = tc.Get(ctx, "delete:target", &v)
	assert.False(t, found, "Key should not be found after Delete")
}

// =============================================================================
// Test: Cache — Metrics tracking
// =============================================================================

func TestIntegration_Cache_Metrics(t *testing.T) {
	tc := newL1OnlyCache(t)
	ctx := context.Background()

	// Perform some operations to generate metrics
	_ = tc.Set(ctx, "metrics:k1", "v1", 5*time.Minute)
	_ = tc.Set(ctx, "metrics:k2", "v2", 5*time.Minute)

	// Hit
	var v string
	tc.Get(ctx, "metrics:k1", &v)
	tc.Get(ctx, "metrics:k2", &v)

	// Miss
	tc.Get(ctx, "metrics:nonexistent", &v)

	metrics := tc.Metrics()
	assert.Equal(t, int64(2), metrics.L1Hits, "Should record 2 L1 hits")
	assert.Equal(t, int64(1), metrics.L1Misses, "Should record 1 L1 miss")
	assert.Equal(t, int64(2), metrics.L1Size, "L1 should hold 2 entries")

	// Hit rate
	hitRate := tc.HitRate()
	assert.Greater(t, hitRate, 0.0, "Hit rate should be positive")
}

// =============================================================================
// Test: Cache — CacheService disabled mode (nil config)
// =============================================================================

func TestIntegration_Cache_CacheServiceDisabledMode(t *testing.T) {
	// Creating CacheService with nil config should result in disabled caching
	svc, err := cache.NewCacheService(nil)
	// Error is expected (Redis connection fails) but service should still be created
	assert.Error(t, err, "Should report Redis connection failure")
	require.NotNil(t, svc, "CacheService should still be created even with nil config")

	assert.False(t, svc.IsEnabled(), "Cache should be disabled when Redis is not available")
}

// =============================================================================
// Redis Container Integration Tests (via Container Harness)
// =============================================================================

// newL2CacheWithRedis creates a TieredCache with L2 (Redis) enabled using
// the container harness. This tests against a real Redis container.
func newL2CacheWithRedis(t *testing.T) *cache.TieredCache {
	t.Helper()

	// Require Redis container to be available
	RequireContainerService(t, "redis")

	harness, _ := GetContainerHarness()
	redisURL := harness.GetServiceURL("redis")

	// Parse Redis URL for client creation
	opt, err := redis.ParseURL(redisURL)
	require.NoError(t, err, "Failed to parse Redis URL")

	redisClient := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = redisClient.Ping(ctx).Err()
	require.NoError(t, err, "Failed to connect to Redis container")

	cfg := cache.DefaultTieredCacheConfig()
	cfg.EnableL1 = true
	cfg.EnableL2 = true
	cfg.L1MaxSize = 1000
	cfg.L1TTL = 1 * time.Minute
	cfg.L2TTL = 5 * time.Minute

	tc := cache.NewTieredCache(redisClient, cfg)
	t.Cleanup(func() {
		_ = tc.Close()
		redisClient.Close()
	})

	return tc
}

// TestIntegration_Cache_Redis_SetAndGet tests SET/GET operations via real Redis
func TestIntegration_Cache_Redis_SetAndGet(t *testing.T) {
	tc := newL2CacheWithRedis(t)
	ctx := context.Background()

	// Store a string value (should go to both L1 and L2)
	err := tc.Set(ctx, "redis:test:key:1", "hello-redis", 5*time.Minute)
	require.NoError(t, err)

	// Read from cache (L1 will serve initially)
	var result string
	found, err := tc.Get(ctx, "redis:test:key:1", &result)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "hello-redis", result)

	// Verify L2 (Redis) also has the value by creating new L1 cache
	harness, _ := GetContainerHarness()
	redisURL := harness.GetServiceURL("redis")
	opt, _ := redis.ParseURL(redisURL)
	redisClient := redis.NewClient(opt)
	defer redisClient.Close()

	val, err := redisClient.Get(ctx, "redis:test:key:1").Result()
	require.NoError(t, err)
	assert.Equal(t, "\"hello-redis\"", val, "Value should be stored in Redis")

	t.Log("✓ Redis container integration test passed")
}

// TestIntegration_Cache_Redis_TTLExpiry tests TTL works through Redis
func TestIntegration_Cache_Redis_TTLExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TTL expiry test in short mode") // SKIP-OK: #short-mode
	}

	tc := newL2CacheWithRedis(t)
	ctx := context.Background()

	// Store with 500ms TTL
	err := tc.Set(ctx, "redis:ttl:key", "ephemeral", 500*time.Millisecond)
	require.NoError(t, err)

	// Immediately readable from L1
	var result string
	found, err := tc.Get(ctx, "redis:ttl:key", &result)
	require.NoError(t, err)
	assert.True(t, found)

	// Wait for TTL to expire in Redis
	time.Sleep(1 * time.Second)

	// Create new cache instance (new L1) to force L2 read
	harness, _ := GetContainerHarness()
	redisURL := harness.GetServiceURL("redis")
	opt, _ := redis.ParseURL(redisURL)
	redisClient := redis.NewClient(opt)
	defer redisClient.Close()

	// Value should be gone from Redis
	_, err = redisClient.Get(ctx, "redis:ttl:key").Result()
	assert.Equal(t, redis.Nil, err, "Value should have expired in Redis")

	t.Log("✓ Redis TTL expiry test passed")
}

// TestIntegration_Cache_Redis_L1L2Consistency verifies L1 and L2 are consistent
func TestIntegration_Cache_Redis_L1L2Consistency(t *testing.T) {
	tc := newL2CacheWithRedis(t)
	ctx := context.Background()

	// Store value
	err := tc.Set(ctx, "consistency:key", "original", 5*time.Minute)
	require.NoError(t, err)

	// Read from L1 (first read)
	var l1Val string
	found, err := tc.Get(ctx, "consistency:key", &l1Val)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "original", l1Val)

	// Verify L2 (Redis) has same value
	harness, _ := GetContainerHarness()
	redisURL := harness.GetServiceURL("redis")
	opt, _ := redis.ParseURL(redisURL)
	redisClient := redis.NewClient(opt)
	defer redisClient.Close()

	l2Val, err := redisClient.Get(ctx, "consistency:key").Result()
	require.NoError(t, err)
	assert.Contains(t, l2Val, "original", "L2 should have the same value as L1")

	// Metrics should show L1 hit
	metrics := tc.Metrics()
	assert.Greater(t, metrics.L1Hits, int64(0), "Should have L1 hits")

	t.Log("✓ L1/L2 consistency test passed")
}
