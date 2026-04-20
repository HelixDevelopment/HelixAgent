package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultOptimizerConfig(t *testing.T) {
	config := DefaultOptimizerConfig()

	require.NotNil(t, config)
	assert.Equal(t, 100, config.MaxPreparedStmts)
	assert.Equal(t, 5*time.Minute, config.CacheTTL)
	assert.True(t, config.EnableCache)
	assert.Equal(t, 1000, config.DefaultBatchSize)
	assert.Equal(t, 30*time.Second, config.QueryTimeout)
}

func TestOptimizerConfig_Fields(t *testing.T) {
	config := OptimizerConfig{
		MaxPreparedStmts: 50,
		CacheTTL:         10 * time.Minute,
		EnableCache:      false,
		DefaultBatchSize: 500,
		QueryTimeout:     1 * time.Minute,
	}

	assert.Equal(t, 50, config.MaxPreparedStmts)
	assert.Equal(t, 10*time.Minute, config.CacheTTL)
	assert.False(t, config.EnableCache)
	assert.Equal(t, 500, config.DefaultBatchSize)
	assert.Equal(t, 1*time.Minute, config.QueryTimeout)
}

func TestQueryMetrics_Fields(t *testing.T) {
	metrics := &QueryMetrics{
		TotalQueries:      100,
		CacheHits:         80,
		CacheMisses:       20,
		TotalLatencyUs:    50000,
		SlowQueries:       5,
		PreparedStmtHits:  70,
		BulkInsertRows:    10000,
		BulkInsertBatches: 10,
	}

	assert.Equal(t, int64(100), metrics.TotalQueries)
	assert.Equal(t, int64(80), metrics.CacheHits)
	assert.Equal(t, int64(20), metrics.CacheMisses)
	assert.Equal(t, int64(50000), metrics.TotalLatencyUs)
	assert.Equal(t, int64(5), metrics.SlowQueries)
	assert.Equal(t, int64(70), metrics.PreparedStmtHits)
	assert.Equal(t, int64(10000), metrics.BulkInsertRows)
	assert.Equal(t, int64(10), metrics.BulkInsertBatches)
}

func TestNewQueryCache(t *testing.T) {
	cache := NewQueryCache(5*time.Minute, 100)
	require.NotNil(t, cache)
	assert.Equal(t, 5*time.Minute, cache.ttl)
	assert.Equal(t, 100, cache.maxSize)
	assert.NotNil(t, cache.cache)
}

func TestQueryCache_SetAndGet(t *testing.T) {
	cache := NewQueryCache(5*time.Minute, 100)

	cache.Set("key1", "value1")
	cache.Set("key2", 123)
	cache.Set("key3", true)

	val1, ok1 := cache.Get("key1")
	assert.True(t, ok1)
	assert.Equal(t, "value1", val1)

	val2, ok2 := cache.Get("key2")
	assert.True(t, ok2)
	assert.Equal(t, 123, val2)

	val3, ok3 := cache.Get("key3")
	assert.True(t, ok3)
	assert.Equal(t, true, val3)
}

func TestQueryCache_Get_NotFound(t *testing.T) {
	cache := NewQueryCache(5*time.Minute, 100)

	val, ok := cache.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestQueryCache_Get_Expired(t *testing.T) {
	cache := NewQueryCache(1*time.Millisecond, 100)

	cache.Set("key", "value")
	time.Sleep(5 * time.Millisecond)

	val, ok := cache.Get("key")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestQueryCache_Invalidate(t *testing.T) {
	cache := NewQueryCache(5*time.Minute, 100)

	cache.Set("key", "value")
	cache.Invalidate("key")

	val, ok := cache.Get("key")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestQueryCache_InvalidatePrefix(t *testing.T) {
	cache := NewQueryCache(5*time.Minute, 100)

	cache.Set("provider_1", "val1")
	cache.Set("provider_2", "val2")
	cache.Set("mcp_1", "val3")

	cache.InvalidatePrefix("provider_")

	_, ok1 := cache.Get("provider_1")
	_, ok2 := cache.Get("provider_2")
	_, ok3 := cache.Get("mcp_1")

	assert.False(t, ok1)
	assert.False(t, ok2)
	assert.True(t, ok3)
}

func TestQueryCache_Clear(t *testing.T) {
	cache := NewQueryCache(5*time.Minute, 100)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	cache.Clear()

	_, ok1 := cache.Get("key1")
	_, ok2 := cache.Get("key2")
	_, ok3 := cache.Get("key3")

	assert.False(t, ok1)
	assert.False(t, ok2)
	assert.False(t, ok3)
}

func TestQueryCache_Eviction(t *testing.T) {
	cache := NewQueryCache(5*time.Minute, 3)

	// Fill cache to capacity
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// Add one more, should evict the oldest
	cache.Set("key4", "value4")

	// key4 should be present
	val4, ok4 := cache.Get("key4")
	assert.True(t, ok4)
	assert.Equal(t, "value4", val4)

	// Total entries should be maxSize
	cache.mu.RLock()
	assert.Equal(t, 3, len(cache.cache))
	cache.mu.RUnlock()
}

func TestNewQueryOptimizer_WithNilConfig(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, nil)

	require.NotNil(t, optimizer)
	assert.NotNil(t, optimizer.config)
	assert.NotNil(t, optimizer.metrics)
	assert.NotNil(t, optimizer.queryCache) // Default config enables cache
	assert.NotNil(t, optimizer.preparedStmts)
}

func TestNewQueryOptimizer_WithConfig(t *testing.T) {
	config := &OptimizerConfig{
		MaxPreparedStmts: 50,
		CacheTTL:         10 * time.Minute,
		EnableCache:      false, // Disable cache
		DefaultBatchSize: 500,
		QueryTimeout:     1 * time.Minute,
	}

	optimizer := NewQueryOptimizer(nil, config)

	require.NotNil(t, optimizer)
	assert.Nil(t, optimizer.queryCache) // Cache disabled
	assert.Equal(t, config, optimizer.config)
}

func TestQueryOptimizer_Metrics(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, nil)

	metrics := optimizer.Metrics()
	require.NotNil(t, metrics)
	assert.Equal(t, int64(0), metrics.TotalQueries)
	assert.Equal(t, int64(0), metrics.CacheHits)
}

func TestQueryOptimizer_AverageLatency_NoQueries(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, nil)

	latency := optimizer.AverageLatency()
	assert.Equal(t, time.Duration(0), latency)
}

func TestQueryOptimizer_CacheHitRate_NoQueries(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, nil)

	rate := optimizer.CacheHitRate()
	assert.Equal(t, float64(0), rate)
}

func TestQueryOptimizer_InvalidateCache(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, nil)
	optimizer.queryCache.Set("key", "value")

	optimizer.InvalidateCache()

	_, ok := optimizer.queryCache.Get("key")
	assert.False(t, ok)
}

func TestQueryOptimizer_InvalidateCacheKey(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, nil)
	optimizer.queryCache.Set("key1", "value1")
	optimizer.queryCache.Set("key2", "value2")

	optimizer.InvalidateCacheKey("key1")

	_, ok1 := optimizer.queryCache.Get("key1")
	_, ok2 := optimizer.queryCache.Get("key2")

	assert.False(t, ok1)
	assert.True(t, ok2)
}

func TestQueryOptimizer_Close(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, nil)

	err := optimizer.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, optimizer.preparedStmts.Len())
}

func TestActiveProvider_Fields(t *testing.T) {
	provider := ActiveProvider{
		ID:           "provider-1",
		Name:         "OpenAI",
		Type:         "openai",
		Weight:       1.0,
		HealthStatus: "healthy",
		ResponseTime: 150,
		Config:       []byte(`{"api_key":"secret"}`),
	}

	assert.Equal(t, "provider-1", provider.ID)
	assert.Equal(t, "OpenAI", provider.Name)
	assert.Equal(t, "openai", provider.Type)
	assert.Equal(t, 1.0, provider.Weight)
	assert.Equal(t, "healthy", provider.HealthStatus)
	assert.Equal(t, int64(150), provider.ResponseTime)
	assert.NotEmpty(t, provider.Config)
}

func TestProviderPerformance_Fields(t *testing.T) {
	perf := ProviderPerformance{
		ProviderName:      "Claude",
		ProviderType:      "anthropic",
		HealthStatus:      "healthy",
		TotalRequests24h:  1000,
		AvgResponseTimeMs: 200,
		P95ResponseTimeMs: 500,
		AvgConfidence:     0.95,
		SuccessRate:       0.99,
	}

	assert.Equal(t, "Claude", perf.ProviderName)
	assert.Equal(t, "anthropic", perf.ProviderType)
	assert.Equal(t, "healthy", perf.HealthStatus)
	assert.Equal(t, int64(1000), perf.TotalRequests24h)
	assert.Equal(t, 200, perf.AvgResponseTimeMs)
	assert.Equal(t, 500, perf.P95ResponseTimeMs)
	assert.InDelta(t, 0.95, perf.AvgConfidence, 0.01)
	assert.InDelta(t, 0.99, perf.SuccessRate, 0.01)
}

func TestMCPServerHealth_Fields(t *testing.T) {
	health := MCPServerHealth{
		ServerID:             "mcp-1",
		ServerName:           "filesystem",
		ServerType:           "local",
		Enabled:              true,
		TotalOperations1h:    500,
		SuccessfulOperations: 490,
		FailedOperations:     10,
		AvgDurationMs:        50,
		P95DurationMs:        150,
		SuccessRate:          0.98,
		ToolCount:            10,
	}

	assert.Equal(t, "mcp-1", health.ServerID)
	assert.Equal(t, "filesystem", health.ServerName)
	assert.Equal(t, "local", health.ServerType)
	assert.True(t, health.Enabled)
	assert.Equal(t, int64(500), health.TotalOperations1h)
	assert.Equal(t, int64(490), health.SuccessfulOperations)
	assert.Equal(t, int64(10), health.FailedOperations)
	assert.Equal(t, 50, health.AvgDurationMs)
	assert.Equal(t, 150, health.P95DurationMs)
	assert.InDelta(t, 0.98, health.SuccessRate, 0.01)
	assert.Equal(t, 10, health.ToolCount)
}

func TestViewRefreshResult_Fields(t *testing.T) {
	result := ViewRefreshResult{
		ViewName:   "mv_provider_performance",
		Status:     "refreshed",
		DurationMs: 500,
	}

	assert.Equal(t, "mv_provider_performance", result.ViewName)
	assert.Equal(t, "refreshed", result.Status)
	assert.Equal(t, 500, result.DurationMs)
}
