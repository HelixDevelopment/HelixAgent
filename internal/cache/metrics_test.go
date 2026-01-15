package cache

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// IncrementingMetrics Tests
// ============================================================================

func TestNewIncrementingMetrics(t *testing.T) {
	m := NewIncrementingMetrics()
	require.NotNil(t, m)

	// All counters should start at 0
	assert.Equal(t, int64(0), m.Hits())
	assert.Equal(t, int64(0), m.Misses())
	assert.Equal(t, int64(0), m.Sets())
	assert.Equal(t, int64(0), m.Deletes())
	assert.Equal(t, int64(0), m.Invalidations())
	assert.Equal(t, int64(0), m.Errors())
}

func TestIncrementingMetrics_Hit(t *testing.T) {
	m := NewIncrementingMetrics()

	m.Hit()
	assert.Equal(t, int64(1), m.Hits())

	m.Hit()
	m.Hit()
	assert.Equal(t, int64(3), m.Hits())
}

func TestIncrementingMetrics_Miss(t *testing.T) {
	m := NewIncrementingMetrics()

	m.Miss()
	assert.Equal(t, int64(1), m.Misses())

	m.Miss()
	m.Miss()
	assert.Equal(t, int64(3), m.Misses())
}

func TestIncrementingMetrics_Set(t *testing.T) {
	m := NewIncrementingMetrics()

	m.Set()
	assert.Equal(t, int64(1), m.Sets())

	m.Set()
	assert.Equal(t, int64(2), m.Sets())
}

func TestIncrementingMetrics_Delete(t *testing.T) {
	m := NewIncrementingMetrics()

	m.Delete()
	assert.Equal(t, int64(1), m.Deletes())

	m.Delete()
	assert.Equal(t, int64(2), m.Deletes())
}

func TestIncrementingMetrics_Invalidate(t *testing.T) {
	m := NewIncrementingMetrics()

	m.Invalidate()
	assert.Equal(t, int64(1), m.Invalidations())

	m.Invalidate()
	m.Invalidate()
	assert.Equal(t, int64(3), m.Invalidations())
}

func TestIncrementingMetrics_Error(t *testing.T) {
	m := NewIncrementingMetrics()

	m.Error()
	assert.Equal(t, int64(1), m.Errors())

	m.Error()
	assert.Equal(t, int64(2), m.Errors())
}

func TestIncrementingMetrics_HitRate(t *testing.T) {
	m := NewIncrementingMetrics()

	// No hits or misses - should return 0
	assert.Equal(t, float64(0), m.HitRate())

	// 100% hit rate
	m.Hit()
	assert.Equal(t, float64(100), m.HitRate())

	// 50% hit rate
	m.Miss()
	assert.Equal(t, float64(50), m.HitRate())

	// 66.67% hit rate (2 hits, 1 miss)
	m.Hit()
	rate := m.HitRate()
	assert.InDelta(t, 66.67, rate, 0.01)

	// 75% hit rate (3 hits, 1 miss)
	m.Hit()
	assert.Equal(t, float64(75), m.HitRate())
}

func TestIncrementingMetrics_Reset(t *testing.T) {
	m := NewIncrementingMetrics()

	// Set some values
	m.Hit()
	m.Hit()
	m.Miss()
	m.Set()
	m.Delete()
	m.Invalidate()
	m.Error()

	// Verify values were set
	assert.Equal(t, int64(2), m.Hits())
	assert.Equal(t, int64(1), m.Misses())
	assert.Equal(t, int64(1), m.Sets())
	assert.Equal(t, int64(1), m.Deletes())
	assert.Equal(t, int64(1), m.Invalidations())
	assert.Equal(t, int64(1), m.Errors())

	// Reset
	m.Reset()

	// All should be 0
	assert.Equal(t, int64(0), m.Hits())
	assert.Equal(t, int64(0), m.Misses())
	assert.Equal(t, int64(0), m.Sets())
	assert.Equal(t, int64(0), m.Deletes())
	assert.Equal(t, int64(0), m.Invalidations())
	assert.Equal(t, int64(0), m.Errors())
}

func TestIncrementingMetrics_ConcurrentAccess(t *testing.T) {
	m := NewIncrementingMetrics()
	const numGoroutines = 100
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 6) // 6 different operations

	// Concurrent hits
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.Hit()
			}
		}()
	}

	// Concurrent misses
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.Miss()
			}
		}()
	}

	// Concurrent sets
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.Set()
			}
		}()
	}

	// Concurrent deletes
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.Delete()
			}
		}()
	}

	// Concurrent invalidations
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.Invalidate()
			}
		}()
	}

	// Concurrent errors
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.Error()
			}
		}()
	}

	wg.Wait()

	expected := int64(numGoroutines * opsPerGoroutine)
	assert.Equal(t, expected, m.Hits())
	assert.Equal(t, expected, m.Misses())
	assert.Equal(t, expected, m.Sets())
	assert.Equal(t, expected, m.Deletes())
	assert.Equal(t, expected, m.Invalidations())
	assert.Equal(t, expected, m.Errors())
}

// ============================================================================
// CacheMetrics Struct Tests
// ============================================================================

func TestCacheMetrics_DefaultValues(t *testing.T) {
	m := &CacheMetrics{}

	assert.Equal(t, int64(0), m.TotalHits)
	assert.Equal(t, int64(0), m.TotalMisses)
	assert.Equal(t, int64(0), m.TotalSets)
	assert.Equal(t, int64(0), m.L1Hits)
	assert.Equal(t, int64(0), m.L2Hits)
}

func TestCacheMetrics_WithValues(t *testing.T) {
	m := &CacheMetrics{
		TotalHits:        100,
		TotalMisses:      20,
		TotalSets:        50,
		L1Hits:           80,
		L1Misses:         40,
		L2Hits:           20,
		L2Misses:         10,
		ProviderHits:     30,
		ProviderMisses:   10,
		MCPHits:          15,
		MCPMisses:        5,
		CompressionSaved: 1024,
	}

	assert.Equal(t, int64(100), m.TotalHits)
	assert.Equal(t, int64(20), m.TotalMisses)
	assert.Equal(t, int64(50), m.TotalSets)
	assert.Equal(t, int64(80), m.L1Hits)
	assert.Equal(t, int64(1024), m.CompressionSaved)
}

// ============================================================================
// CacheSummary Tests
// ============================================================================

func TestCacheSummary_DefaultValues(t *testing.T) {
	s := &CacheSummary{}

	assert.Equal(t, float64(0), s.HitRate)
	assert.Equal(t, float64(0), s.L1HitRate)
	assert.Equal(t, float64(0), s.L2HitRate)
	assert.Equal(t, int64(0), s.TotalHits)
}

func TestCacheSummary_WithValues(t *testing.T) {
	s := &CacheSummary{
		HitRate:            85.5,
		L1HitRate:          90.0,
		L2HitRate:          70.0,
		TotalHits:          1000,
		TotalMisses:        171,
		TotalInvalidations: 50,
		L1Size:             500,
		CompressionSaved:   2048,
		ProviderHitRate:    88.0,
		MCPHitRate:         75.0,
	}

	assert.Equal(t, 85.5, s.HitRate)
	assert.Equal(t, 90.0, s.L1HitRate)
	assert.Equal(t, int64(1000), s.TotalHits)
	assert.Equal(t, int64(2048), s.CompressionSaved)
}

// ============================================================================
// CacheMetricsCollector Tests
// ============================================================================

func TestNewCacheMetricsCollector(t *testing.T) {
	// Test with nil components (valid use case)
	collector := NewCacheMetricsCollector(nil, nil, nil, nil)
	require.NotNil(t, collector)
	assert.NotNil(t, collector.aggregated)
}

func TestCacheMetricsCollector_CollectWithNilComponents(t *testing.T) {
	collector := NewCacheMetricsCollector(nil, nil, nil, nil)

	metrics := collector.Collect()
	require.NotNil(t, metrics)

	// All metrics should be 0 when no components are set
	assert.Equal(t, int64(0), metrics.TotalHits)
	assert.Equal(t, int64(0), metrics.TotalMisses)
	assert.Equal(t, int64(0), metrics.L1Hits)
	assert.Equal(t, int64(0), metrics.L2Hits)
}

func TestCacheMetricsCollector_HitRateWithNilComponents(t *testing.T) {
	collector := NewCacheMetricsCollector(nil, nil, nil, nil)

	// Should return 0 when no data
	assert.Equal(t, float64(0), collector.HitRate())
}

func TestCacheMetricsCollector_L1HitRateWithNilComponents(t *testing.T) {
	collector := NewCacheMetricsCollector(nil, nil, nil, nil)

	// Should return 0 when no data
	assert.Equal(t, float64(0), collector.L1HitRate())
}

func TestCacheMetricsCollector_L2HitRateWithNilComponents(t *testing.T) {
	collector := NewCacheMetricsCollector(nil, nil, nil, nil)

	// Should return 0 when no data
	assert.Equal(t, float64(0), collector.L2HitRate())
}

func TestCacheMetricsCollector_AverageGetLatencyWithNoData(t *testing.T) {
	collector := NewCacheMetricsCollector(nil, nil, nil, nil)

	// Should return 0 when no get operations
	assert.Equal(t, int64(0), int64(collector.AverageGetLatency()))
}

func TestCacheMetricsCollector_AverageSetLatencyWithNoData(t *testing.T) {
	collector := NewCacheMetricsCollector(nil, nil, nil, nil)

	// Should return 0 when no set operations
	assert.Equal(t, int64(0), int64(collector.AverageSetLatency()))
}

func TestCacheMetricsCollector_CompressionSavingsWithNoData(t *testing.T) {
	collector := NewCacheMetricsCollector(nil, nil, nil, nil)

	// Should return 0 when no compression
	assert.Equal(t, int64(0), collector.CompressionSavings())
}

func TestCacheMetricsCollector_SummaryWithNilComponents(t *testing.T) {
	collector := NewCacheMetricsCollector(nil, nil, nil, nil)

	summary := collector.Summary()
	require.NotNil(t, summary)

	assert.Equal(t, float64(0), summary.HitRate)
	assert.Equal(t, float64(0), summary.L1HitRate)
	assert.Equal(t, float64(0), summary.L2HitRate)
	assert.Equal(t, int64(0), summary.TotalHits)
	assert.Equal(t, int64(0), summary.TotalMisses)
}
