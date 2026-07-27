package monitoring_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMonitoring_CacheHitRatio validates that cache hit/miss counters
// accurately reflect the expected hit ratio when operations are simulated.
func TestMonitoring_CacheHitRatio(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	cacheOps := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_operations_total",
		Help: "Total cache operations",
	}, []string{"operation", "result"})
	registry.MustRegister(cacheOps)

	// Simulate cache operations: 70% hit rate
	for i := 0; i < 100; i++ {
		if i%10 < 7 {
			cacheOps.WithLabelValues("get", "hit").Inc()
		} else {
			cacheOps.WithLabelValues("get", "miss").Inc()
		}
	}

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	var hits, misses float64
	for _, mf := range metricFamilies {
		if mf.GetName() == "cache_operations_total" {
			for _, m := range mf.GetMetric() {
				labels := m.GetLabel()
				for _, l := range labels {
					if l.GetName() == "result" {
						switch l.GetValue() {
						case "hit":
							hits = m.GetCounter().GetValue()
						case "miss":
							misses = m.GetCounter().GetValue()
						}
					}
				}
			}
		}
	}

	hitRatio := hits / (hits + misses) * 100
	assert.InDelta(t, 70.0, hitRatio, 1.0,
		"cache hit ratio should be ~70%%")
	t.Logf("Cache hit ratio: %.1f%% (hits=%.0f, misses=%.0f)",
		hitRatio, hits, misses)
}

// TestMonitoring_CacheEvictions validates that cache eviction and size
// metrics are correctly tracked as items are evicted from the cache.
func TestMonitoring_CacheEvictions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	evictions := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cache_evictions_total",
		Help: "Total cache evictions",
	})
	registry.MustRegister(evictions)

	cacheSize := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cache_current_size",
		Help: "Current number of items in cache",
	})
	registry.MustRegister(cacheSize)

	// Simulate cache filling and evicting
	cacheSize.Set(1000)
	for i := 0; i < 50; i++ {
		evictions.Inc()
		cacheSize.Dec()
	}

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	for _, mf := range metricFamilies {
		if mf.GetName() == "cache_current_size" {
			for _, m := range mf.GetMetric() {
				assert.Equal(t, 950.0, m.GetGauge().GetValue(),
					"cache size should be 950 after 50 evictions")
			}
		}
		if mf.GetName() == "cache_evictions_total" {
			for _, m := range mf.GetMetric() {
				assert.Equal(t, 50.0, m.GetCounter().GetValue(),
					"should have recorded 50 evictions")
			}
		}
	}
}

// TestMonitoring_CacheMemoryUsage validates that cache memory usage metrics
// accurately track the byte-level memory consumption of cached data.
func TestMonitoring_CacheMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	cacheMemory := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cache_memory_bytes",
		Help: "Current memory usage of cache in bytes",
	})
	registry.MustRegister(cacheMemory)

	cacheMemoryLimit := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cache_memory_limit_bytes",
		Help: "Maximum memory limit for cache in bytes",
	})
	registry.MustRegister(cacheMemoryLimit)

	// Simulate cache with 256MB limit, currently using 180MB
	const limitMB = 256
	const usedMB = 180
	cacheMemoryLimit.Set(float64(limitMB * 1024 * 1024))
	cacheMemory.Set(float64(usedMB * 1024 * 1024))

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	var usedBytes, limitBytes float64
	for _, mf := range metricFamilies {
		switch mf.GetName() {
		case "cache_memory_bytes":
			usedBytes = mf.GetMetric()[0].GetGauge().GetValue()
		case "cache_memory_limit_bytes":
			limitBytes = mf.GetMetric()[0].GetGauge().GetValue()
		}
	}

	utilization := usedBytes / limitBytes * 100
	assert.InDelta(t, 70.3, utilization, 1.0,
		"cache memory utilization should be ~70%%")
	assert.Less(t, usedBytes, limitBytes,
		"used memory should be below the limit")
}

// TestMonitoring_CacheTTLExpiration validates that cache TTL expiration
// counters are tracked per cache region (in-memory vs distributed).
func TestMonitoring_CacheTTLExpiration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	ttlExpired := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_ttl_expirations_total",
		Help: "Total cache entries expired by TTL",
	}, []string{"cache_type"})
	registry.MustRegister(ttlExpired)

	// Simulate TTL expirations across cache types
	ttlExpired.WithLabelValues("in_memory").Add(120)
	ttlExpired.WithLabelValues("redis").Add(45)

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	expirations := make(map[string]float64)
	for _, mf := range metricFamilies {
		if mf.GetName() == "cache_ttl_expirations_total" {
			for _, m := range mf.GetMetric() {
				for _, label := range m.GetLabel() {
					if label.GetName() == "cache_type" {
						expirations[label.GetValue()] =
							m.GetCounter().GetValue()
					}
				}
			}
		}
	}

	assert.Equal(t, 120.0, expirations["in_memory"],
		"in-memory expirations should be 120")
	assert.Equal(t, 45.0, expirations["redis"],
		"redis expirations should be 45")
	assert.Greater(t, expirations["in_memory"], expirations["redis"],
		"in-memory typically has more expirations due to smaller capacity")
}
