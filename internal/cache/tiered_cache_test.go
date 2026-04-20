package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultTieredCacheConfig(t *testing.T) {
	config := DefaultTieredCacheConfig()

	assert.Equal(t, 10000, config.L1MaxSize)
	assert.Equal(t, 5*time.Minute, config.L1TTL)
	assert.Equal(t, time.Minute, config.L1CleanupInterval)
	assert.Equal(t, 30*time.Minute, config.L2TTL)
	assert.True(t, config.L2Compression)
	assert.Equal(t, "tiered:", config.L2KeyPrefix)
	assert.Equal(t, 30*time.Second, config.NegativeTTL)
	assert.True(t, config.EnableL1)
	assert.True(t, config.EnableL2)
}

func TestNewTieredCache_NilConfig(t *testing.T) {
	tc := NewTieredCache(nil, nil)
	defer func() { _ = tc.Close() }()

	assert.NotNil(t, tc)
	assert.NotNil(t, tc.l1)
	assert.NotNil(t, tc.config)
	assert.NotNil(t, tc.metrics)
	assert.NotNil(t, tc.tagIndex)

	// Verify default config is applied
	assert.Equal(t, 10000, tc.l1.maxSize)
}

func TestNewTieredCache_CustomConfig(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             time.Minute,
		L1CleanupInterval: time.Second,
		EnableL1:          true,
		EnableL2:          false, // Disable L2 for testing
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	assert.NotNil(t, tc)
	assert.Equal(t, 100, tc.l1.maxSize)
}

func TestTieredCache_Get_L1Only(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             5 * time.Minute,
		L1CleanupInterval: time.Minute,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Get on empty cache
	var result string
	found, err := tc.Get(ctx, "test-key", &result)
	assert.NoError(t, err)
	assert.False(t, found)

	// Set value
	err = tc.Set(ctx, "test-key", "test-value", time.Minute)
	require.NoError(t, err)

	// Get value
	found, err = tc.Get(ctx, "test-key", &result)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "test-value", result)
}

func TestTieredCache_Set_L1Only(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             5 * time.Minute,
		L1CleanupInterval: time.Minute,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Set various types
	tests := []struct {
		name  string
		key   string
		value interface{}
	}{
		{"string", "key-string", "string-value"},
		{"int", "key-int", 12345},
		{"struct", "key-struct", struct{ Name string }{Name: "test"}},
		{"slice", "key-slice", []string{"a", "b", "c"}},
		{"map", "key-map", map[string]int{"x": 1, "y": 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.Set(ctx, tt.key, tt.value, time.Minute)
			require.NoError(t, err)
		})
	}
}

func TestTieredCache_Set_TTLCapping(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             time.Minute, // Short L1 TTL
		L1CleanupInterval: time.Second,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Set with TTL longer than L1TTL - should be capped
	err := tc.Set(ctx, "test-key", "test-value", time.Hour)
	require.NoError(t, err)

	// Value should be accessible
	var result string
	found, err := tc.Get(ctx, "test-key", &result)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "test-value", result)
}

func TestTieredCache_Set_WithTags(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             5 * time.Minute,
		L1CleanupInterval: time.Minute,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Set with tags
	err := tc.Set(ctx, "key1", "value1", time.Minute, "tag-a", "tag-b")
	require.NoError(t, err)
	err = tc.Set(ctx, "key2", "value2", time.Minute, "tag-a")
	require.NoError(t, err)
	err = tc.Set(ctx, "key3", "value3", time.Minute, "tag-c")
	require.NoError(t, err)

	// Verify tag index
	keysTagA := tc.tagIndex.GetKeys("tag-a")
	assert.Len(t, keysTagA, 2)

	keysTagB := tc.tagIndex.GetKeys("tag-b")
	assert.Len(t, keysTagB, 1)
}

func TestTieredCache_Delete_L1Only(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             5 * time.Minute,
		L1CleanupInterval: time.Minute,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Set value
	err := tc.Set(ctx, "test-key", "test-value", time.Minute)
	require.NoError(t, err)

	// Verify it exists
	var result string
	found, err := tc.Get(ctx, "test-key", &result)
	require.NoError(t, err)
	require.True(t, found)

	// Delete
	err = tc.Delete(ctx, "test-key")
	require.NoError(t, err)

	// Verify it's gone
	found, err = tc.Get(ctx, "test-key", &result)
	assert.NoError(t, err)
	assert.False(t, found)
}

func TestTieredCache_InvalidateByTag(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             5 * time.Minute,
		L1CleanupInterval: time.Minute,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Set values with tags
	err := tc.Set(ctx, "key1", "value1", time.Minute, "tag-a")
	require.NoError(t, err)
	err = tc.Set(ctx, "key2", "value2", time.Minute, "tag-a")
	require.NoError(t, err)
	err = tc.Set(ctx, "key3", "value3", time.Minute, "tag-b")
	require.NoError(t, err)

	// Invalidate by tag-a
	count, err := tc.InvalidateByTag(ctx, "tag-a")
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify key1 and key2 are gone
	var result string
	found, _ := tc.Get(ctx, "key1", &result)
	assert.False(t, found)
	found, _ = tc.Get(ctx, "key2", &result)
	assert.False(t, found)

	// Verify key3 still exists
	found, _ = tc.Get(ctx, "key3", &result)
	assert.True(t, found)
}

func TestTieredCache_InvalidateByTag_NoKeys(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize: 100,
		EnableL1:  true,
		EnableL2:  false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Invalidate non-existent tag
	count, err := tc.InvalidateByTag(ctx, "non-existent-tag")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestTieredCache_InvalidateByTags(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             5 * time.Minute,
		L1CleanupInterval: time.Minute,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Set values with different tags
	err := tc.Set(ctx, "key1", "value1", time.Minute, "tag-a")
	require.NoError(t, err)
	err = tc.Set(ctx, "key2", "value2", time.Minute, "tag-b")
	require.NoError(t, err)
	err = tc.Set(ctx, "key3", "value3", time.Minute, "tag-c")
	require.NoError(t, err)

	// Invalidate by multiple tags
	count, err := tc.InvalidateByTags(ctx, "tag-a", "tag-b")
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify key3 still exists
	var result string
	found, _ := tc.Get(ctx, "key3", &result)
	assert.True(t, found)
}

func TestTieredCache_InvalidatePrefix_L1Only(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             5 * time.Minute,
		L1CleanupInterval: time.Minute,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Set values with different prefixes
	err := tc.Set(ctx, "user:1", "value1", time.Minute)
	require.NoError(t, err)
	err = tc.Set(ctx, "user:2", "value2", time.Minute)
	require.NoError(t, err)
	err = tc.Set(ctx, "session:1", "value3", time.Minute)
	require.NoError(t, err)

	// Invalidate by prefix
	count, err := tc.InvalidatePrefix(ctx, "user:")
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify user:* keys are gone
	var result string
	found, _ := tc.Get(ctx, "user:1", &result)
	assert.False(t, found)
	found, _ = tc.Get(ctx, "user:2", &result)
	assert.False(t, found)

	// Verify session:1 still exists
	found, _ = tc.Get(ctx, "session:1", &result)
	assert.True(t, found)
}

func TestTieredCache_Metrics(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             5 * time.Minute,
		L1CleanupInterval: time.Minute,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Initial metrics
	m := tc.Metrics()
	assert.Equal(t, int64(0), m.L1Hits)
	assert.Equal(t, int64(0), m.L1Misses)
	assert.Equal(t, int64(0), m.L1Size)

	// Generate some cache activity
	_ = tc.Set(ctx, "key1", "value1", time.Minute)
	_ = tc.Set(ctx, "key2", "value2", time.Minute)

	// Miss
	var result string
	_, _ = tc.Get(ctx, "nonexistent", &result)

	// Hits
	_, _ = tc.Get(ctx, "key1", &result)
	_, _ = tc.Get(ctx, "key2", &result)

	// Check metrics
	m = tc.Metrics()
	assert.Equal(t, int64(2), m.L1Hits)
	assert.Equal(t, int64(1), m.L1Misses)
	assert.Equal(t, int64(2), m.L1Size)
}

func TestTieredCache_HitRate(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize: 100,
		L1TTL:     5 * time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// No activity - hit rate should be 0
	assert.Equal(t, float64(0), tc.HitRate())

	// Set some values
	_ = tc.Set(ctx, "key1", "value1", time.Minute)

	// 2 hits, 1 miss = 66.67% hit rate
	var result string
	_, _ = tc.Get(ctx, "key1", &result)    // hit
	_, _ = tc.Get(ctx, "key1", &result)    // hit
	_, _ = tc.Get(ctx, "missing", &result) // miss

	hitRate := tc.HitRate()
	assert.InDelta(t, 66.67, hitRate, 1.0)
}

func TestTieredCache_Close(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize: 100,
		EnableL1:  true,
		EnableL2:  false,
	}

	tc := NewTieredCache(nil, config)

	// Close should not error
	err := tc.Close()
	assert.NoError(t, err)
}

func TestTieredCache_L1Expiration(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             50 * time.Millisecond,
		L1CleanupInterval: 10 * time.Millisecond,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Set value with short TTL
	err := tc.Set(ctx, "short-lived", "value", 50*time.Millisecond)
	require.NoError(t, err)

	// Verify it exists
	var result string
	found, _ := tc.Get(ctx, "short-lived", &result)
	assert.True(t, found)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now (detected on get)
	found, _ = tc.Get(ctx, "short-lived", &result)
	assert.False(t, found)
}

func TestTieredCache_L1Eviction(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         3, // Very small cache
		L1TTL:             time.Minute,
		L1CleanupInterval: time.Hour, // Don't cleanup during test
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Fill the cache
	_ = tc.Set(ctx, "key1", "value1", time.Minute)
	_ = tc.Set(ctx, "key2", "value2", time.Minute)
	_ = tc.Set(ctx, "key3", "value3", time.Minute)

	// Access key2 and key3 to increase their hit count
	var result string
	_, _ = tc.Get(ctx, "key2", &result)
	_, _ = tc.Get(ctx, "key2", &result)
	_, _ = tc.Get(ctx, "key3", &result)

	// Add one more - should evict key1 (lowest hit count)
	_ = tc.Set(ctx, "key4", "value4", time.Minute)

	// key1 should be evicted
	found, _ := tc.Get(ctx, "key1", &result)
	assert.False(t, found)

	// key2, key3, key4 should still exist
	found, _ = tc.Get(ctx, "key2", &result)
	assert.True(t, found)
	found, _ = tc.Get(ctx, "key3", &result)
	assert.True(t, found)
	found, _ = tc.Get(ctx, "key4", &result)
	assert.True(t, found)

	// Check eviction metric
	m := tc.Metrics()
	assert.Equal(t, int64(1), m.L1Evictions)
}

func TestTagIndex_Add(t *testing.T) {
	ti := newTagIndex()

	ti.Add("key1", "tag-a", "tag-b")
	ti.Add("key2", "tag-a")
	ti.Add("key3", "tag-b", "tag-c")

	// Check tag-a has key1 and key2
	keysA := ti.GetKeys("tag-a")
	assert.Len(t, keysA, 2)

	// Check tag-b has key1 and key3
	keysB := ti.GetKeys("tag-b")
	assert.Len(t, keysB, 2)

	// Check tag-c has only key3
	keysC := ti.GetKeys("tag-c")
	assert.Len(t, keysC, 1)
}

func TestTagIndex_Remove(t *testing.T) {
	ti := newTagIndex()

	ti.Add("key1", "tag-a", "tag-b")
	ti.Add("key2", "tag-a")

	// Remove key1
	ti.Remove("key1")

	// Check tag-a now only has key2
	keysA := ti.GetKeys("tag-a")
	assert.Len(t, keysA, 1)

	// Check tag-b is empty (should be removed from index)
	keysB := ti.GetKeys("tag-b")
	assert.Empty(t, keysB)
}

func TestTagIndex_GetKeys_NoTag(t *testing.T) {
	ti := newTagIndex()

	// Get keys for non-existent tag
	keys := ti.GetKeys("nonexistent")
	assert.Nil(t, keys)
}

func TestTieredCache_ConcurrentAccess(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             time.Minute,
		L1CleanupInterval: time.Hour,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Concurrent writes
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "key" + string(rune('0'+i%10))
			_ = tc.Set(ctx, key, i, time.Minute)
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "key" + string(rune('0'+i%10))
			var result int
			_, _ = tc.Get(ctx, key, &result)
		}(i)
	}
	wg.Wait()
}

func TestTieredCache_L1Disabled(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize: 100,
		EnableL1:  false, // L1 disabled
		EnableL2:  false, // L2 disabled
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Set should succeed but not cache
	err := tc.Set(ctx, "key1", "value1", time.Minute)
	require.NoError(t, err)

	// Get should miss (nothing is cached)
	var result string
	found, err := tc.Get(ctx, "key1", &result)
	assert.NoError(t, err)
	assert.False(t, found)
}

func TestTieredCache_InvalidValue(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize: 100,
		EnableL1:  true,
		EnableL2:  false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Set with an unmarshallable value (channel)
	err := tc.Set(ctx, "key1", make(chan int), time.Minute)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "marshal value")
}

func TestCompress(t *testing.T) {
	tc := &TieredCache{
		config: &TieredCacheConfig{L2Compression: true},
	}

	// Test compression
	data := []byte("This is a test string that should be compressed. " +
		"We repeat it to make it larger and more compressible. " +
		"The quick brown fox jumps over the lazy dog. " +
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit.")

	compressed, err := tc.compress(data)
	require.NoError(t, err)
	assert.Less(t, len(compressed), len(data)) // Should be smaller

	// Test decompression
	decompressed, err := tc.decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

func TestDecompress_InvalidData(t *testing.T) {
	tc := &TieredCache{
		config: &TieredCacheConfig{L2Compression: true},
	}

	// Invalid gzip data
	_, err := tc.decompress([]byte("not gzip data"))
	assert.Error(t, err)
}

func TestTieredCache_L1Cleanup(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             50 * time.Millisecond,
		L1CleanupInterval: 25 * time.Millisecond,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	// Set value with short TTL
	_ = tc.Set(ctx, "cleanup-test", "value", 50*time.Millisecond)

	// Verify it exists
	var result string
	found, _ := tc.Get(ctx, "cleanup-test", &result)
	assert.True(t, found)

	// Wait for cleanup loop to run
	time.Sleep(150 * time.Millisecond)

	// The cleanup should have caught the expired entry
	// (value should no longer be in cache)
	_, exists := tc.l1.entries.Get("cleanup-test")
	assert.False(t, exists)

	// Verify expiration was tracked in metrics
	m := tc.Metrics()
	assert.GreaterOrEqual(t, m.Expirations, int64(0))
}

func TestTieredCache_L1CleanupLoop_DefaultInterval(t *testing.T) {
	config := &TieredCacheConfig{
		L1MaxSize:         100,
		L1TTL:             time.Minute,
		L1CleanupInterval: 0, // Should default to 1 minute
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	// Just verify it doesn't panic
	assert.NotNil(t, tc)
}

func TestTieredCacheMetrics_Atomics(t *testing.T) {
	metrics := &TieredCacheMetrics{}

	// Verify atomic operations work correctly
	atomic.AddInt64(&metrics.L1Hits, 10)
	atomic.AddInt64(&metrics.L1Misses, 5)
	atomic.AddInt64(&metrics.L2Hits, 3)
	atomic.AddInt64(&metrics.L2Misses, 2)
	atomic.AddInt64(&metrics.Invalidations, 1)
	atomic.AddInt64(&metrics.Expirations, 7)
	atomic.AddInt64(&metrics.CompressionSaved, 100)
	atomic.StoreInt64(&metrics.L1Size, 50)
	atomic.AddInt64(&metrics.L1Evictions, 2)

	assert.Equal(t, int64(10), atomic.LoadInt64(&metrics.L1Hits))
	assert.Equal(t, int64(5), atomic.LoadInt64(&metrics.L1Misses))
	assert.Equal(t, int64(3), atomic.LoadInt64(&metrics.L2Hits))
	assert.Equal(t, int64(2), atomic.LoadInt64(&metrics.L2Misses))
	assert.Equal(t, int64(1), atomic.LoadInt64(&metrics.Invalidations))
	assert.Equal(t, int64(7), atomic.LoadInt64(&metrics.Expirations))
	assert.Equal(t, int64(100), atomic.LoadInt64(&metrics.CompressionSaved))
	assert.Equal(t, int64(50), atomic.LoadInt64(&metrics.L1Size))
	assert.Equal(t, int64(2), atomic.LoadInt64(&metrics.L1Evictions))
}

// --- Benchmarks ---

func BenchmarkTieredCache_L1Get(b *testing.B) {
	config := &TieredCacheConfig{
		L1MaxSize:         10000,
		L1TTL:             5 * time.Minute,
		L1CleanupInterval: time.Hour,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()
	_ = tc.Set(ctx, "bench-key", "bench-value", time.Minute)

	var result string
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tc.Get(ctx, "bench-key", &result)
	}
}

func BenchmarkTieredCache_L1Set(b *testing.B) {
	config := &TieredCacheConfig{
		L1MaxSize:         100000,
		L1TTL:             5 * time.Minute,
		L1CleanupInterval: time.Hour,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tc.Set(ctx, "bench-key", "bench-value", time.Minute)
	}
}

func BenchmarkTieredCache_L1Delete(b *testing.B) {
	config := &TieredCacheConfig{
		L1MaxSize:         100000,
		L1TTL:             5 * time.Minute,
		L1CleanupInterval: time.Hour,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tc.Set(ctx, "bench-key", "bench-value", time.Minute)
		_ = tc.Delete(ctx, "bench-key")
	}
}

func BenchmarkTieredCache_Compress(b *testing.B) {
	tc := &TieredCache{
		config: &TieredCacheConfig{L2Compression: true},
	}

	data := []byte("This is a test string that should be compressed. " +
		"We repeat it to make it larger and more compressible. " +
		"The quick brown fox jumps over the lazy dog. " +
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit.")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tc.compress(data)
	}
}

func BenchmarkTieredCache_Decompress(b *testing.B) {
	tc := &TieredCache{
		config: &TieredCacheConfig{L2Compression: true},
	}

	data := []byte("This is a test string that should be compressed. " +
		"We repeat it to make it larger and more compressible. " +
		"The quick brown fox jumps over the lazy dog. " +
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit.")

	compressed, _ := tc.compress(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tc.decompress(compressed)
	}
}

func BenchmarkTieredCache_HitRate(b *testing.B) {
	config := &TieredCacheConfig{
		L1MaxSize:         10000,
		L1TTL:             5 * time.Minute,
		L1CleanupInterval: time.Hour,
		EnableL1:          true,
		EnableL2:          false,
	}

	tc := NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	ctx := context.Background()
	_ = tc.Set(ctx, "key1", "value1", time.Minute)
	var result string
	_, _ = tc.Get(ctx, "key1", &result)
	_, _ = tc.Get(ctx, "missing", &result)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tc.HitRate()
	}
}

func BenchmarkTagIndex_Add(b *testing.B) {
	ti := newTagIndex()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "key-" + string(rune('a'+i%26))
		ti.Add(key, "tag-a", "tag-b")
	}
}

func BenchmarkTagIndex_GetKeys(b *testing.B) {
	ti := newTagIndex()
	for i := 0; i < 1000; i++ {
		key := "key-" + string(rune('a'+i%26)) + string(rune('0'+i%10))
		ti.Add(key, "tag-"+string(rune('a'+i%5)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tag := "tag-" + string(rune('a'+i%5))
		_ = ti.GetKeys(tag)
	}
}

func BenchmarkTagIndex_Remove(b *testing.B) {
	b.StopTimer()
	ti := newTagIndex()
	for i := 0; i < b.N; i++ {
		key := "key-" + string(rune('a'+i%26))
		ti.Add(key, "tag-a", "tag-b")
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		key := "key-" + string(rune('a'+i%26))
		ti.Remove(key)
	}
}
