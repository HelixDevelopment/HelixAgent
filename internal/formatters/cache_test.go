//go:build !integration

package formatters

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCache(ttl time.Duration, maxSize int) *FormatterCache {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	return NewFormatterCache(&CacheConfig{
		TTL:         ttl,
		MaxSize:     maxSize,
		CleanupFreq: 1 * time.Hour, // long cleanup so it doesn't interfere with tests
	}, logger)
}

func TestNewFormatterCache(t *testing.T) {
	t.Parallel()
	cache := newTestCache(1*time.Hour, 100)
	defer cache.Stop()

	assert.NotNil(t, cache)
	assert.Equal(t, 0, cache.Size())
}

func TestFormatterCache_Set_Get(t *testing.T) {
	t.Parallel()
	cache := newTestCache(1*time.Hour, 100)
	defer cache.Stop()

	req := &FormatRequest{
		Content:  "x = 1",
		Language: "python",
		FilePath: "test.py",
	}
	result := &FormatResult{
		Content:       "x = 1\n",
		Changed:       true,
		FormatterName: "black",
		Success:       true,
	}

	cache.Set(req, result)

	got, found := cache.Get(req)
	assert.True(t, found)
	require.NotNil(t, got)
	assert.Equal(t, "x = 1\n", got.Content)
	assert.Equal(t, "black", got.FormatterName)
	assert.True(t, got.Success)
}

func TestFormatterCache_Get_Miss(t *testing.T) {
	t.Parallel()
	cache := newTestCache(1*time.Hour, 100)
	defer cache.Stop()

	req := &FormatRequest{
		Content:  "not cached",
		Language: "python",
	}

	got, found := cache.Get(req)
	assert.False(t, found)
	assert.Nil(t, got)
}

func TestFormatterCache_Get_Expired(t *testing.T) {
	t.Parallel()
	cache := newTestCache(1*time.Millisecond, 100)
	defer cache.Stop()

	req := &FormatRequest{
		Content:  "x = 1",
		Language: "python",
	}
	result := &FormatResult{
		Content: "x = 1\n",
		Success: true,
	}

	cache.Set(req, result)

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	got, found := cache.Get(req)
	assert.False(t, found)
	assert.Nil(t, got)
}

func TestFormatterCache_Set_Eviction(t *testing.T) {
	t.Parallel()
	cache := newTestCache(1*time.Hour, 2)
	defer cache.Stop()

	// Fill cache to capacity
	for i := 0; i < 3; i++ {
		req := &FormatRequest{
			Content:  string(rune('a' + i)),
			Language: "python",
		}
		result := &FormatResult{
			Content: string(rune('a'+i)) + " formatted",
			Success: true,
		}
		cache.Set(req, result)
		// Small delay to ensure different timestamps for eviction ordering
		time.Sleep(1 * time.Millisecond)
	}

	// Size should not exceed max
	assert.LessOrEqual(t, cache.Size(), 2)
}

func TestFormatterCache_Clear(t *testing.T) {
	t.Parallel()
	cache := newTestCache(1*time.Hour, 100)
	defer cache.Stop()

	// Add entries
	for i := 0; i < 5; i++ {
		req := &FormatRequest{
			Content:  string(rune('a' + i)),
			Language: "python",
		}
		result := &FormatResult{Content: "result"}
		cache.Set(req, result)
	}

	assert.Equal(t, 5, cache.Size())

	cache.Clear()
	assert.Equal(t, 0, cache.Size())
}

func TestFormatterCache_Size(t *testing.T) {
	t.Parallel()
	cache := newTestCache(1*time.Hour, 100)
	defer cache.Stop()

	assert.Equal(t, 0, cache.Size())

	cache.Set(&FormatRequest{Content: "a", Language: "go"}, &FormatResult{Content: "a"})
	assert.Equal(t, 1, cache.Size())

	cache.Set(&FormatRequest{Content: "b", Language: "go"}, &FormatResult{Content: "b"})
	assert.Equal(t, 2, cache.Size())
}

func TestFormatterCache_Stats(t *testing.T) {
	t.Parallel()
	cache := newTestCache(30*time.Minute, 500)
	defer cache.Stop()

	cache.Set(&FormatRequest{Content: "a"}, &FormatResult{Content: "a"})

	stats := cache.Stats()
	assert.Equal(t, 1, stats.Size)
	assert.Equal(t, 500, stats.MaxSize)
	assert.Equal(t, 30*time.Minute, stats.TTL)
}

func TestFormatterCache_Stop(t *testing.T) {
	t.Parallel()
	cache := newTestCache(1*time.Hour, 100)
	// Should not panic
	cache.Stop()
}

func TestFormatterCache_CacheKey_DifferentContent(t *testing.T) {
	t.Parallel()
	cache := newTestCache(1*time.Hour, 100)
	defer cache.Stop()

	req1 := &FormatRequest{Content: "x = 1", Language: "python"}
	req2 := &FormatRequest{Content: "y = 2", Language: "python"}

	result1 := &FormatResult{Content: "x = 1\n", Success: true}
	result2 := &FormatResult{Content: "y = 2\n", Success: true}

	cache.Set(req1, result1)
	cache.Set(req2, result2)

	got1, found1 := cache.Get(req1)
	assert.True(t, found1)
	assert.Equal(t, "x = 1\n", got1.Content)

	got2, found2 := cache.Get(req2)
	assert.True(t, found2)
	assert.Equal(t, "y = 2\n", got2.Content)
}

func TestFormatterCache_CacheKey_DifferentLanguage(t *testing.T) {
	t.Parallel()
	cache := newTestCache(1*time.Hour, 100)
	defer cache.Stop()

	req1 := &FormatRequest{Content: "code", Language: "python"}
	req2 := &FormatRequest{Content: "code", Language: "javascript"}

	result1 := &FormatResult{Content: "python formatted"}
	result2 := &FormatResult{Content: "js formatted"}

	cache.Set(req1, result1)
	cache.Set(req2, result2)

	got1, _ := cache.Get(req1)
	assert.Equal(t, "python formatted", got1.Content)

	got2, _ := cache.Get(req2)
	assert.Equal(t, "js formatted", got2.Content)
}

func TestFormatterCache_CacheKey_DifferentFilePath(t *testing.T) {
	t.Parallel()
	cache := newTestCache(1*time.Hour, 100)
	defer cache.Stop()

	req1 := &FormatRequest{Content: "code", FilePath: "a.py"}
	req2 := &FormatRequest{Content: "code", FilePath: "b.py"}

	result1 := &FormatResult{Content: "result a"}
	result2 := &FormatResult{Content: "result b"}

	cache.Set(req1, result1)
	cache.Set(req2, result2)

	got1, _ := cache.Get(req1)
	assert.Equal(t, "result a", got1.Content)

	got2, _ := cache.Get(req2)
	assert.Equal(t, "result b", got2.Content)
}

func TestFormatterCache_Set_SameKeyOverwrites(t *testing.T) {
	t.Parallel()
	cache := newTestCache(1*time.Hour, 100)
	defer cache.Stop()

	req := &FormatRequest{Content: "x = 1", Language: "python"}

	cache.Set(req, &FormatResult{Content: "version1"})
	cache.Set(req, &FormatResult{Content: "version2"})

	got, found := cache.Get(req)
	assert.True(t, found)
	assert.Equal(t, "version2", got.Content)
	assert.Equal(t, 1, cache.Size())
}

func TestFormatterCache_Cleanup(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	cache := NewFormatterCache(&CacheConfig{
		TTL:         10 * time.Millisecond,
		MaxSize:     100,
		CleanupFreq: 20 * time.Millisecond,
	}, logger)
	defer cache.Stop()

	// Add entries
	cache.Set(&FormatRequest{Content: "a"}, &FormatResult{Content: "a"})
	cache.Set(&FormatRequest{Content: "b"}, &FormatResult{Content: "b"})
	assert.Equal(t, 2, cache.Size())

	// Wait for TTL expiry and cleanup cycle
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 0, cache.Size())
}
