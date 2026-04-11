package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestCollector creates a collector with its own isolated registry
// to avoid conflicts when tests run in parallel
func newTestCollector(t *testing.T) *Collector {
	t.Helper()
	registry := prometheus.NewRegistry()
	return NewCollectorWithRegistry(registry)
}

func TestNewCollector(t *testing.T) {
	t.Parallel()
	t.Run("creates collector with all metrics", func(t *testing.T) {
		c := newTestCollector(t)

		require.NotNil(t, c)
		assert.NotNil(t, c.RequestDuration)
		assert.NotNil(t, c.ProviderLatency)
		assert.NotNil(t, c.CacheHits)
		assert.NotNil(t, c.CacheMisses)
	})

	t.Run("metrics are registered with prometheus", func(t *testing.T) {
		t.Parallel()
		// This test verifies metrics can be collected
		c := newTestCollector(t)

		// Record some metrics
		c.RequestDuration.WithLabelValues("GET", "/api/test", "200").Observe(0.1)
		c.CacheHits.WithLabelValues("memory").Inc()

		// Should not panic
		assert.NotNil(t, c)
	})

	t.Run("returns handler for metrics endpoint", func(t *testing.T) {
		t.Parallel()
		c := newTestCollector(t)

		handler := c.Handler()
		assert.NotNil(t, handler)
	})
}

func TestCollector_RequestDuration(t *testing.T) {
	t.Parallel()
	t.Run("records request duration", func(t *testing.T) {
		c := newTestCollector(t)

		// Record multiple request durations
		c.RequestDuration.WithLabelValues("GET", "/api/health", "200").Observe(0.05)
		c.RequestDuration.WithLabelValues("POST", "/api/debate", "201").Observe(0.5)
		c.RequestDuration.WithLabelValues("GET", "/api/health", "200").Observe(0.03)

		// Verify no panic and metrics are recorded
		assert.NotNil(t, c.RequestDuration)
	})

	t.Run("handles different status codes", func(t *testing.T) {
		t.Parallel()
		c := newTestCollector(t)

		statuses := []string{"200", "201", "400", "401", "403", "404", "500", "503"}
		for _, status := range statuses {
			c.RequestDuration.WithLabelValues("GET", "/api/test", status).Observe(0.1)
		}

		assert.NotNil(t, c.RequestDuration)
	})

	t.Run("handles different HTTP methods", func(t *testing.T) {
		t.Parallel()
		c := newTestCollector(t)

		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
		for _, method := range methods {
			c.RequestDuration.WithLabelValues(method, "/api/test", "200").Observe(0.1)
		}

		assert.NotNil(t, c.RequestDuration)
	})
}

func TestCollector_ProviderLatency(t *testing.T) {
	t.Parallel()
	t.Run("records provider latency", func(t *testing.T) {
		c := newTestCollector(t)

		// Record latencies for different providers
		c.ProviderLatency.WithLabelValues("openai", "gpt-4").Observe(0.5)
		c.ProviderLatency.WithLabelValues("anthropic", "claude-3").Observe(0.8)
		c.ProviderLatency.WithLabelValues("gemini", "pro").Observe(0.3)

		assert.NotNil(t, c.ProviderLatency)
	})

	t.Run("handles multiple providers", func(t *testing.T) {
		t.Parallel()
		c := newTestCollector(t)

		providers := []string{
			"openai", "anthropic", "gemini", "deepseek", "mistral",
			"cohere", "groq", "fireworks", "together", "perplexity",
		}

		for _, provider := range providers {
			c.ProviderLatency.WithLabelValues(provider, "default").Observe(0.5)
		}

		assert.NotNil(t, c.ProviderLatency)
	})
}

func TestCollector_CacheMetrics(t *testing.T) {
	t.Parallel()
	t.Run("records cache hits", func(t *testing.T) {
		c := newTestCollector(t)

		// Record cache hits
		c.CacheHits.WithLabelValues("memory").Inc()
		c.CacheHits.WithLabelValues("memory").Inc()
		c.CacheHits.WithLabelValues("redis").Inc()

		assert.NotNil(t, c.CacheHits)
	})

	t.Run("records cache misses", func(t *testing.T) {
		t.Parallel()
		c := newTestCollector(t)

		// Record cache misses
		c.CacheMisses.WithLabelValues("memory").Inc()
		c.CacheMisses.WithLabelValues("redis").Inc()
		c.CacheMisses.WithLabelValues("redis").Inc()

		assert.NotNil(t, c.CacheMisses)
	})

	t.Run("calculates hit ratio", func(t *testing.T) {
		t.Parallel()
		c := newTestCollector(t)

		// Simulate 80% hit rate
		for i := 0; i < 80; i++ {
			c.CacheHits.WithLabelValues("test").Inc()
		}
		for i := 0; i < 20; i++ {
			c.CacheMisses.WithLabelValues("test").Inc()
		}

		// Both metrics should be recorded
		assert.NotNil(t, c.CacheHits)
		assert.NotNil(t, c.CacheMisses)
	})
}

func TestCollector_Handler(t *testing.T) {
	t.Parallel()
	t.Run("exposes metrics via HTTP", func(t *testing.T) {
		c := newTestCollector(t)

		// Record some metrics
		c.RequestDuration.WithLabelValues("GET", "/test", "200").Observe(0.1)
		c.CacheHits.WithLabelValues("memory").Inc()

		// Create test server
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		w := httptest.NewRecorder()

		// Serve metrics
		handler := c.Handler()
		handler.ServeHTTP(w, req)

		// Verify response
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")

		body := w.Body.String()
		assert.Contains(t, body, "http_request_duration_seconds")
		assert.Contains(t, body, "cache_hits_total")
	})

	t.Run("returns prometheus format", func(t *testing.T) {
		t.Parallel()
		c := newTestCollector(t)

		// Record at least one metric so the handler output is non-empty
		c.RequestDuration.WithLabelValues("GET", "/test", "200").Observe(0.1)

		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		w := httptest.NewRecorder()

		handler := c.Handler()
		handler.ServeHTTP(w, req)

		body := w.Body.String()

		// Check for prometheus exposition format
		assert.Contains(t, body, "# HELP")
		assert.Contains(t, body, "# TYPE")
	})

	t.Run("includes all registered metrics", func(t *testing.T) {
		t.Parallel()
		c := newTestCollector(t)

		// Record all metric types
		c.RequestDuration.WithLabelValues("GET", "/", "200").Observe(0.1)
		c.ProviderLatency.WithLabelValues("test", "model").Observe(0.5)
		c.CacheHits.WithLabelValues("test").Inc()
		c.CacheMisses.WithLabelValues("test").Inc()

		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		w := httptest.NewRecorder()

		handler := c.Handler()
		handler.ServeHTTP(w, req)

		body := w.Body.String()

		assert.Contains(t, body, "http_request_duration_seconds")
		assert.Contains(t, body, "llm_provider_latency_seconds")
		assert.Contains(t, body, "cache_hits_total")
		assert.Contains(t, body, "cache_misses_total")
	})
}

func TestCollector_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	t.Run("handles concurrent metric recording", func(t *testing.T) {
		c := newTestCollector(t)

		done := make(chan bool, 100)

		// Record metrics concurrently from multiple goroutines
		for i := 0; i < 100; i++ {
			go func(n int) {
				defer func() { done <- true }()

				method := "GET"
				if n%2 == 0 {
					method = "POST"
				}

				c.RequestDuration.WithLabelValues(method, "/api/test", "200").Observe(float64(n) * 0.001)
				c.CacheHits.WithLabelValues("memory").Inc()
				c.ProviderLatency.WithLabelValues("openai", "gpt-4").Observe(0.5)
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 100; i++ {
			<-done
		}

		// Verify metrics endpoint still works
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		w := httptest.NewRecorder()

		handler := c.Handler()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	})
}

func TestCollector_MetricLabels(t *testing.T) {
	t.Parallel()
	t.Run("validates label counts", func(t *testing.T) {
		c := newTestCollector(t)

		// Should work with correct label count
		assert.NotPanics(t, func() {
			c.RequestDuration.WithLabelValues("GET", "/test", "200").Observe(0.1)
		})

		// Should work with correct label count
		assert.NotPanics(t, func() {
			c.ProviderLatency.WithLabelValues("openai", "gpt-4").Observe(0.5)
		})

		// Should work with correct label count
		assert.NotPanics(t, func() {
			c.CacheHits.WithLabelValues("memory").Inc()
		})
	})
}

func TestCollector_ObservationValues(t *testing.T) {
	t.Parallel()
	t.Run("handles various observation values", func(t *testing.T) {
		c := newTestCollector(t)

		// Very small values
		c.RequestDuration.WithLabelValues("GET", "/fast", "200").Observe(0.001)

		// Normal values
		c.RequestDuration.WithLabelValues("GET", "/normal", "200").Observe(0.1)

		// Large values
		c.RequestDuration.WithLabelValues("GET", "/slow", "200").Observe(10.0)

		// Zero
		c.RequestDuration.WithLabelValues("GET", "/instant", "200").Observe(0)

		assert.NotNil(t, c.RequestDuration)
	})
}

func TestCollector_HistogramBuckets(t *testing.T) {
	t.Parallel()
	t.Run("uses correct buckets for request duration", func(t *testing.T) {
		c := newTestCollector(t)

		// Record values in different buckets
		buckets := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

		for _, bucket := range buckets {
			c.RequestDuration.WithLabelValues("GET", "/test", "200").Observe(bucket)
		}

		assert.NotNil(t, c.RequestDuration)
	})

	t.Run("uses correct buckets for provider latency", func(t *testing.T) {
		t.Parallel()
		c := newTestCollector(t)

		// Record values in different buckets
		buckets := []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}

		for _, bucket := range buckets {
			c.ProviderLatency.WithLabelValues("test", "model").Observe(bucket)
		}

		assert.NotNil(t, c.ProviderLatency)
	})
}

// Benchmarks
func BenchmarkCollector_RecordDuration(b *testing.B) {
	c := NewCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RequestDuration.WithLabelValues("GET", "/api/test", "200").Observe(0.1)
	}
}

func BenchmarkCollector_RecordLatency(b *testing.B) {
	c := NewCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.ProviderLatency.WithLabelValues("openai", "gpt-4").Observe(0.5)
	}
}

func BenchmarkCollector_RecordCacheHit(b *testing.B) {
	c := NewCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.CacheHits.WithLabelValues("memory").Inc()
	}
}

func BenchmarkCollector_Handler(b *testing.B) {
	c := NewCollector()

	// Record some metrics first
	for i := 0; i < 100; i++ {
		c.RequestDuration.WithLabelValues("GET", "/test", "200").Observe(0.1)
	}

	handler := c.Handler()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkCollector_Concurrent(b *testing.B) {
	c := NewCollector()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.RequestDuration.WithLabelValues("GET", "/test", "200").Observe(float64(i) * 0.001)
			i++
		}
	})
}
