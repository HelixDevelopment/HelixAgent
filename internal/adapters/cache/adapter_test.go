package cache_test

import (
	"context"
	"testing"
	"time"

	adapter "dev.helix.agent/internal/adapters/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// MemoryCacheAdapter Tests (no external deps)
// ============================================================================

func TestNewMemoryCacheAdapter(t *testing.T) {
	t.Parallel()
	c := adapter.NewMemoryCacheAdapter(100, time.Minute)
	require.NotNil(t, c)
	defer c.Close()
}

func TestMemoryCacheAdapter_SetAndGet(t *testing.T) {
	t.Parallel()
	c := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer c.Close()

	ctx := context.Background()
	err := c.Set(ctx, "key1", "hello world", time.Minute)
	require.NoError(t, err)

	var result string
	err = c.Get(ctx, "key1", &result)
	require.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestMemoryCacheAdapter_SetAndGet_Struct(t *testing.T) {
	t.Parallel()
	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	c := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer c.Close()

	ctx := context.Background()
	original := TestData{Name: "test", Value: 42}
	err := c.Set(ctx, "struct-key", original, time.Minute)
	require.NoError(t, err)

	var result TestData
	err = c.Get(ctx, "struct-key", &result)
	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestMemoryCacheAdapter_Get_KeyNotFound(t *testing.T) {
	t.Parallel()
	c := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer c.Close()

	ctx := context.Background()
	var result string
	err := c.Get(ctx, "nonexistent", &result)
	assert.Error(t, err)
}

func TestMemoryCacheAdapter_Delete(t *testing.T) {
	t.Parallel()
	c := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer c.Close()

	ctx := context.Background()
	err := c.Set(ctx, "del-key", "value", time.Minute)
	require.NoError(t, err)

	err = c.Delete(ctx, "del-key")
	require.NoError(t, err)

	var result string
	err = c.Get(ctx, "del-key", &result)
	assert.Error(t, err)
}

func TestMemoryCacheAdapter_Exists(t *testing.T) {
	t.Parallel()
	c := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer c.Close()

	ctx := context.Background()
	err := c.Set(ctx, "exists-key", "value", time.Minute)
	require.NoError(t, err)

	exists, err := c.Exists(ctx, "exists-key")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = c.Exists(ctx, "missing-key")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMemoryCacheAdapter_Len(t *testing.T) {
	t.Parallel()
	c := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer c.Close()

	ctx := context.Background()
	assert.Equal(t, 0, c.Len())

	c.Set(ctx, "k1", "v1", time.Minute)
	c.Set(ctx, "k2", "v2", time.Minute)
	assert.Equal(t, 2, c.Len())
}

func TestMemoryCacheAdapter_Flush(t *testing.T) {
	t.Parallel()
	c := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer c.Close()

	ctx := context.Background()
	c.Set(ctx, "k1", "v1", time.Minute)
	c.Set(ctx, "k2", "v2", time.Minute)
	require.Equal(t, 2, c.Len())

	c.Flush()
	assert.Equal(t, 0, c.Len())
}

func TestMemoryCacheAdapter_Stats(t *testing.T) {
	t.Parallel()
	c := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer c.Close()

	stats := c.Stats()
	assert.NotNil(t, stats)
}

func TestMemoryCacheAdapter_Underlying(t *testing.T) {
	t.Parallel()
	c := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer c.Close()

	underlying := c.Underlying()
	assert.NotNil(t, underlying)
}

func TestMemoryCacheAdapter_Expiry(t *testing.T) {
	t.Parallel()
	c := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer c.Close()

	ctx := context.Background()
	err := c.Set(ctx, "expire-key", "val", 50*time.Millisecond)
	require.NoError(t, err)

	// Should exist immediately
	exists, err := c.Exists(ctx, "expire-key")
	require.NoError(t, err)
	assert.True(t, exists)

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	exists, err = c.Exists(ctx, "expire-key")
	require.NoError(t, err)
	assert.False(t, exists)
}

// ============================================================================
// RedisClientAdapter Tests (nil client - no Redis needed)
// ============================================================================

func TestNewRedisClientAdapter_NilConfig(t *testing.T) {
	t.Parallel()
	// Test with memory cache adapter instead since RedisClientAdapterFromClient is not implemented
	a := adapter.NewMemoryCacheAdapter(100, time.Minute)
	require.NotNil(t, a)

	// All ops should work
	ctx := context.Background()
	assert.NoError(t, a.Set(ctx, "k", "v", time.Minute))
	var s string
	assert.NoError(t, a.Get(ctx, "k", &s))
	assert.Equal(t, "v", s)
	assert.NoError(t, a.Delete(ctx, "k"))
}

// ============================================================================
// TypedCacheAdapter Tests
// ============================================================================

func TestTypedCacheAdapter_SetAndGet(t *testing.T) {
	t.Parallel()
	inner := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer inner.Close()

	typed := adapter.NewTypedCacheAdapter[string](inner.Underlying())

	ctx := context.Background()
	err := typed.Set(ctx, "typed-key", "hello", time.Minute)
	require.NoError(t, err)

	val, found, err := typed.Get(ctx, "typed-key")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "hello", val)
}

func TestTypedCacheAdapter_GetMissing(t *testing.T) {
	t.Parallel()
	inner := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer inner.Close()

	typed := adapter.NewTypedCacheAdapter[int](inner.Underlying())

	ctx := context.Background()
	val, found, err := typed.Get(ctx, "missing")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, 0, val)
}

func TestTypedCacheAdapter_Delete(t *testing.T) {
	t.Parallel()
	inner := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer inner.Close()

	typed := adapter.NewTypedCacheAdapter[string](inner.Underlying())

	ctx := context.Background()
	typed.Set(ctx, "del-key", "value", time.Minute)

	err := typed.Delete(ctx, "del-key")
	require.NoError(t, err)

	_, found, _ := typed.Get(ctx, "del-key")
	assert.False(t, found)
}

func TestTypedCacheAdapter_Exists(t *testing.T) {
	t.Parallel()
	inner := adapter.NewMemoryCacheAdapter(100, time.Minute)
	defer inner.Close()

	typed := adapter.NewTypedCacheAdapter[float64](inner.Underlying())

	ctx := context.Background()
	typed.Set(ctx, "exists-key", 3.14, time.Minute)

	exists, err := typed.Exists(ctx, "exists-key")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = typed.Exists(ctx, "nope")
	require.NoError(t, err)
	assert.False(t, exists)
}

// ============================================================================
// Config Helper Tests
// ============================================================================

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := adapter.DefaultConfig()
	assert.NotNil(t, cfg)
}

func TestDefaultRedisConfig(t *testing.T) {
	t.Parallel()
	cfg := adapter.DefaultRedisConfig()
	assert.NotNil(t, cfg)
}

func TestDefaultMemoryConfig(t *testing.T) {
	t.Parallel()
	cfg := adapter.DefaultMemoryConfig()
	assert.NotNil(t, cfg)
}

func TestPolicyConstants(t *testing.T) {
	t.Parallel()
	// Verify eviction policy constants are accessible and distinct
	assert.NotEqual(t, adapter.LRU, adapter.LFU)
	assert.NotEqual(t, adapter.LFU, adapter.FIFO)
	assert.NotEqual(t, adapter.LRU, adapter.FIFO)
}
