package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCacheTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel) // Suppress logging during tests
	return log
}

func TestNewProtocolCache(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)

	require.NotNil(t, cache)
	assert.NotNil(t, cache.cache)
	assert.NotNil(t, cache.invalidators)
	assert.Equal(t, 100, cache.maxSize)
	assert.Equal(t, 5*time.Minute, cache.ttl)
	assert.False(t, cache.stopped)

	// Clean up
	cache.Stop()
}

func TestProtocolCache_SetAndGet(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)
	defer cache.Stop()

	ctx := context.Background()

	t.Run("set and get string", func(t *testing.T) {
		err := cache.Set(ctx, "key1", "value1", []string{"tag1"}, 0)
		require.NoError(t, err)

		data, exists, err := cache.Get(ctx, "key1")
		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, "value1", data)
	})

	t.Run("set and get map", func(t *testing.T) {
		mapData := map[string]interface{}{
			"name":  "test",
			"value": 42,
		}
		err := cache.Set(ctx, "key2", mapData, []string{"tag2"}, 0)
		require.NoError(t, err)

		data, exists, err := cache.Get(ctx, "key2")
		require.NoError(t, err)
		assert.True(t, exists)
		assert.Equal(t, mapData, data)
	})

	t.Run("get non-existent key", func(t *testing.T) {
		data, exists, err := cache.Get(ctx, "nonexistent")
		require.NoError(t, err)
		assert.False(t, exists)
		assert.Nil(t, data)
	})
}

func TestProtocolCache_TTLExpiration(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 1*time.Millisecond, log)
	defer cache.Stop()

	ctx := context.Background()

	err := cache.Set(ctx, "expiring", "value", []string{}, 1*time.Millisecond)
	require.NoError(t, err)

	// Verify it exists immediately
	data, exists, _ := cache.Get(ctx, "expiring")
	assert.True(t, exists)
	assert.Equal(t, "value", data)

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	// Should be expired now
	data, exists, _ = cache.Get(ctx, "expiring")
	assert.False(t, exists)
	assert.Nil(t, data)
}

func TestProtocolCache_Delete(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)
	defer cache.Stop()

	ctx := context.Background()

	// Set then delete
	_ = cache.Set(ctx, "to_delete", "value", []string{}, 0)
	err := cache.Delete(ctx, "to_delete")
	require.NoError(t, err)

	// Should not exist
	_, exists, _ := cache.Get(ctx, "to_delete")
	assert.False(t, exists)
}

func TestProtocolCache_Clear(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)
	defer cache.Stop()

	ctx := context.Background()

	// Set multiple entries
	_ = cache.Set(ctx, "key1", "value1", []string{}, 0)
	_ = cache.Set(ctx, "key2", "value2", []string{}, 0)
	_ = cache.Set(ctx, "key3", "value3", []string{}, 0)

	// Clear all
	err := cache.Clear(ctx)
	require.NoError(t, err)

	// All should be gone
	stats := cache.GetStats()
	assert.Equal(t, 0, stats.TotalEntries)
}

func TestProtocolCache_InvalidateByTags(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)
	defer cache.Stop()

	ctx := context.Background()

	// Set entries with different tags
	_ = cache.Set(ctx, "mcp1", "value1", []string{"mcp", "server"}, 0)
	_ = cache.Set(ctx, "mcp2", "value2", []string{"mcp", "tools"}, 0)
	_ = cache.Set(ctx, "lsp1", "value3", []string{"lsp", "server"}, 0)

	// Invalidate all MCP entries
	err := cache.InvalidateByTags(ctx, []string{"mcp"})
	require.NoError(t, err)

	// MCP entries should be gone
	_, exists, _ := cache.Get(ctx, "mcp1")
	assert.False(t, exists)
	_, exists, _ = cache.Get(ctx, "mcp2")
	assert.False(t, exists)

	// LSP entry should still exist
	_, exists, _ = cache.Get(ctx, "lsp1")
	assert.True(t, exists)
}

func TestProtocolCache_InvalidateByTags_EmptyTags(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)
	defer cache.Stop()

	ctx := context.Background()

	_ = cache.Set(ctx, "key1", "value1", []string{"tag1"}, 0)

	// Empty tags should not invalidate anything
	err := cache.InvalidateByTags(ctx, []string{})
	require.NoError(t, err)

	_, exists, _ := cache.Get(ctx, "key1")
	assert.True(t, exists)
}

func TestProtocolCache_InvalidateByPattern(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)
	defer cache.Stop()

	ctx := context.Background()

	// Set entries with pattern-matchable keys
	_ = cache.Set(ctx, "mcp:server:1", "value1", []string{}, 0)
	_ = cache.Set(ctx, "mcp:server:2", "value2", []string{}, 0)
	_ = cache.Set(ctx, "lsp:server:1", "value3", []string{}, 0)

	// Invalidate all MCP entries
	err := cache.InvalidateByPattern(ctx, "mcp:*")
	require.NoError(t, err)

	// MCP entries should be gone
	_, exists, _ := cache.Get(ctx, "mcp:server:1")
	assert.False(t, exists)
	_, exists, _ = cache.Get(ctx, "mcp:server:2")
	assert.False(t, exists)

	// LSP entry should still exist
	_, exists, _ = cache.Get(ctx, "lsp:server:1")
	assert.True(t, exists)
}

func TestProtocolCache_InvalidateByPattern_EmptyPattern(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)
	defer cache.Stop()

	ctx := context.Background()

	_ = cache.Set(ctx, "key1", "value1", []string{}, 0)

	// Empty pattern should not invalidate anything
	err := cache.InvalidateByPattern(ctx, "")
	require.NoError(t, err)

	_, exists, _ := cache.Get(ctx, "key1")
	assert.True(t, exists)
}

func TestProtocolCache_InvalidateByPattern_WildcardAll(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)
	defer cache.Stop()

	ctx := context.Background()

	_ = cache.Set(ctx, "key1", "value1", []string{}, 0)
	_ = cache.Set(ctx, "key2", "value2", []string{}, 0)

	// Wildcard should invalidate everything
	err := cache.InvalidateByPattern(ctx, "*")
	require.NoError(t, err)

	stats := cache.GetStats()
	assert.Equal(t, 0, stats.TotalEntries)
}

func TestProtocolCache_GetStats(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)
	defer cache.Stop()

	ctx := context.Background()

	// Empty cache stats
	stats := cache.GetStats()
	assert.Equal(t, 0, stats.TotalEntries)

	// Add some entries
	_ = cache.Set(ctx, "key1", "value1", []string{}, 0)
	_ = cache.Set(ctx, "key2", "value2", []string{}, 0)

	// Access an entry to increase hits
	_, _, _ = cache.Get(ctx, "key1")
	_, _, _ = cache.Get(ctx, "key1")

	stats = cache.GetStats()
	assert.Equal(t, 2, stats.TotalEntries)
	assert.Equal(t, 2, stats.TotalHits)
	assert.True(t, stats.TotalSize > 0)
}

func TestProtocolCache_LRUEviction(t *testing.T) {
	log := newCacheTestLogger()
	// Very small cache to test eviction
	cache := NewProtocolCache(3, 5*time.Minute, log)
	defer cache.Stop()

	ctx := context.Background()

	// Fill the cache
	_ = cache.Set(ctx, "key1", "value1", []string{}, 0)
	time.Sleep(10 * time.Millisecond) // Ensure different access times
	_ = cache.Set(ctx, "key2", "value2", []string{}, 0)
	time.Sleep(10 * time.Millisecond)
	_ = cache.Set(ctx, "key3", "value3", []string{}, 0)

	// Access key1 to make it recently used
	_, _, _ = cache.Get(ctx, "key1")

	// Add another entry - should evict the oldest (key2)
	_ = cache.Set(ctx, "key4", "value4", []string{}, 0)

	// key4 should exist
	_, exists, _ := cache.Get(ctx, "key4")
	assert.True(t, exists)

	// key1 should still exist (was accessed)
	_, exists, _ = cache.Get(ctx, "key1")
	assert.True(t, exists)

	// Total entries should be maxSize (3)
	stats := cache.GetStats()
	assert.Equal(t, 3, stats.TotalEntries)
}

func TestProtocolCache_SetInvalidator(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)
	defer cache.Stop()

	invalidator := CacheInvalidator{
		Pattern: "test:*",
		Tags:    []string{"test"},
		TTL:     5 * time.Minute,
	}

	cache.SetInvalidator("test_key", invalidator)

	// Verify invalidator was set
	assert.Len(t, cache.invalidators["test_key"], 1)
}

func TestProtocolCache_RemoveInvalidator(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)
	defer cache.Stop()

	invalidator := CacheInvalidator{
		Pattern: "test:*",
	}

	cache.SetInvalidator("test_key", invalidator)
	cache.RemoveInvalidator("test_key")

	// Verify invalidator was removed
	_, exists := cache.invalidators["test_key"]
	assert.False(t, exists)
}

func TestProtocolCache_Warmup(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)
	defer cache.Stop()

	ctx := context.Background()

	warmupData := map[string]interface{}{
		"config1": map[string]interface{}{"setting": "value1"},
		"config2": map[string]interface{}{"setting": "value2"},
	}

	err := cache.Warmup(ctx, warmupData)
	require.NoError(t, err)

	// Verify warmup data exists
	data, exists, _ := cache.Get(ctx, "config1")
	assert.True(t, exists)
	assert.NotNil(t, data)

	data, exists, _ = cache.Get(ctx, "config2")
	assert.True(t, exists)
	assert.NotNil(t, data)
}

func TestProtocolCache_Stop(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)

	// Stop should work without panic
	cache.Stop()

	// Double stop should also work
	cache.Stop()

	assert.True(t, cache.stopped)
}

func TestProtocolCache_CalculateSize(t *testing.T) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(100, 5*time.Minute, log)
	defer cache.Stop()

	t.Run("string size", func(t *testing.T) {
		size := cache.calculateSize("hello")
		assert.Equal(t, 5, size)
	})

	t.Run("byte slice size", func(t *testing.T) {
		size := cache.calculateSize([]byte{1, 2, 3, 4, 5})
		assert.Equal(t, 5, size)
	})

	t.Run("map size", func(t *testing.T) {
		size := cache.calculateSize(map[string]interface{}{"key": "value"})
		assert.True(t, size > 0)
	})

	t.Run("slice size", func(t *testing.T) {
		size := cache.calculateSize([]interface{}{"a", "b", "c"})
		assert.True(t, size > 0)
	})

	t.Run("struct size", func(t *testing.T) {
		size := cache.calculateSize(struct {
			Name string
		}{Name: "test"})
		assert.True(t, size > 0)
	})
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern  string
		text     string
		expected bool
	}{
		// Basic patterns
		{"*", "anything", true},
		{"*", "", true},
		{"", "", true},
		{"", "text", false},

		// Simple wildcards
		{"*.txt", "file.txt", true},
		{"*.txt", "file.doc", false},
		{"test*", "testing", true},
		{"test*", "test", true},
		{"test*", "tes", false},

		// Middle wildcards
		{"mcp:*:server", "mcp:test:server", true},
		{"mcp:*:server", "mcp::server", true},
		{"mcp:*:server", "mcp:test:client", false},

		// Question mark
		{"test?", "test1", true},
		{"test?", "testab", false},
		{"te?t", "test", true},
		{"te?t", "tent", true},
		{"te?t", "teat", true},

		// Multiple wildcards
		{"*.*", "file.txt", true},
		{"*.*", "filename", false},
		{"*.*.bak", "file.txt.bak", true},

		// Complex patterns
		{"mcp:server:*", "mcp:server:abc123", true},
		{"mcp:server:*", "mcp:client:abc123", false},
		{"*:server:*", "mcp:server:1", true},
		{"*:server:*", "lsp:server:test", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.text, func(t *testing.T) {
			result := matchGlob(tt.pattern, tt.text)
			assert.Equal(t, tt.expected, result, "pattern=%s, text=%s", tt.pattern, tt.text)
		})
	}
}

func TestGenerateCacheKey(t *testing.T) {
	t.Run("with params", func(t *testing.T) {
		key1 := GenerateCacheKey("mcp", "call", map[string]interface{}{"name": "test"})
		key2 := GenerateCacheKey("mcp", "call", map[string]interface{}{"name": "test"})

		// Same inputs should produce same key
		assert.Equal(t, key1, key2)
	})

	t.Run("different params produce different keys", func(t *testing.T) {
		key1 := GenerateCacheKey("mcp", "call", map[string]interface{}{"name": "test1"})
		key2 := GenerateCacheKey("mcp", "call", map[string]interface{}{"name": "test2"})

		assert.NotEqual(t, key1, key2)
	})

	t.Run("nil params", func(t *testing.T) {
		key := GenerateCacheKey("mcp", "list", nil)
		assert.NotEmpty(t, key)
		assert.Len(t, key, 32) // MD5 hex length
	})

	t.Run("different protocols", func(t *testing.T) {
		key1 := GenerateCacheKey("mcp", "call", nil)
		key2 := GenerateCacheKey("lsp", "call", nil)

		assert.NotEqual(t, key1, key2)
	})
}

func TestCacheEntry(t *testing.T) {
	entry := &CacheEntry{
		Key:        "test_key",
		Data:       "test_data",
		Tags:       []string{"tag1", "tag2"},
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		TTL:        5 * time.Minute,
		Hits:       10,
		Size:       100,
	}

	assert.Equal(t, "test_key", entry.Key)
	assert.Equal(t, "test_data", entry.Data)
	assert.Len(t, entry.Tags, 2)
	assert.Equal(t, 10, entry.Hits)
	assert.Equal(t, 100, entry.Size)
}

func TestCacheStats(t *testing.T) {
	stats := CacheStats{
		TotalEntries: 100,
		TotalSize:    50000,
		HitRate:      0.85,
		MissRate:     0.15,
		Evictions:    10,
		TotalHits:    850,
		TotalMisses:  150,
	}

	assert.Equal(t, 100, stats.TotalEntries)
	assert.Equal(t, 50000, stats.TotalSize)
	assert.Equal(t, 0.85, stats.HitRate)
	assert.Equal(t, 0.15, stats.MissRate)
}

func TestCacheInvalidator(t *testing.T) {
	invalidator := CacheInvalidator{
		Pattern: "mcp:*",
		Tags:    []string{"mcp", "tools"},
		TTL:     10 * time.Minute,
	}

	assert.Equal(t, "mcp:*", invalidator.Pattern)
	assert.Len(t, invalidator.Tags, 2)
	assert.Equal(t, 10*time.Minute, invalidator.TTL)
}

func TestCacheKeyConstants(t *testing.T) {
	// Verify key format constants exist and are valid format strings
	assert.Contains(t, CacheKeyMCPServer, "%s")
	assert.Contains(t, CacheKeyMCPTools, "%s")
	assert.Contains(t, CacheKeyLSPServer, "%s")
	assert.Contains(t, CacheKeyACPServer, "%s")
	assert.Contains(t, CacheKeyEmbedding, "%s")
}

func TestCacheTagConstants(t *testing.T) {
	// Verify tag constants
	assert.Equal(t, "mcp", CacheTagMCP)
	assert.Equal(t, "lsp", CacheTagLSP)
	assert.Equal(t, "acp", CacheTagACP)
	assert.Equal(t, "embedding", CacheTagEmbedding)
	assert.Equal(t, "server", CacheTagServer)
	assert.Equal(t, "tools", CacheTagTools)
	assert.Equal(t, "results", CacheTagResults)
}

func BenchmarkProtocolCache_Set(b *testing.B) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(10000, 5*time.Minute, log)
	defer cache.Stop()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Set(ctx, "key", "value", []string{"tag"}, 0)
	}
}

func BenchmarkProtocolCache_Get(b *testing.B) {
	log := newCacheTestLogger()
	cache := NewProtocolCache(10000, 5*time.Minute, log)
	defer cache.Stop()

	ctx := context.Background()
	_ = cache.Set(ctx, "key", "value", []string{"tag"}, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = cache.Get(ctx, "key")
	}
}

func BenchmarkMatchGlob(b *testing.B) {
	pattern := "mcp:*:server"
	text := "mcp:test123:server"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = matchGlob(pattern, text)
	}
}

func BenchmarkGenerateCacheKey(b *testing.B) {
	params := map[string]interface{}{
		"name":   "test",
		"value":  123,
		"nested": map[string]interface{}{"key": "value"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateCacheKey("mcp", "call", params)
	}
}
