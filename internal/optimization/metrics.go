package optimization

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the optimization service.
type Metrics struct {
	// Cache metrics
	CacheHits      prometheus.Counter
	CacheMisses    prometheus.Counter
	CacheSize      prometheus.Gauge
	CacheEvictions prometheus.Counter
	CacheLookupDuration prometheus.Histogram

	// Structured output metrics
	ValidationAttempts  prometheus.Counter
	ValidationSuccesses prometheus.Counter
	ValidationFailures  prometheus.Counter
	ValidationDuration  prometheus.Histogram

	// Streaming metrics
	StreamsStarted   prometheus.Counter
	StreamsCompleted prometheus.Counter
	StreamErrors     prometheus.Counter
	TokensStreamed   prometheus.Counter
	StreamDuration   prometheus.Histogram
	TokensPerSecond  prometheus.Histogram

	// External service metrics
	ServiceRequests   *prometheus.CounterVec
	ServiceErrors     *prometheus.CounterVec
	ServiceLatency    *prometheus.HistogramVec
	ServiceAvailable  *prometheus.GaugeVec

	// Pipeline metrics
	RequestsOptimized    prometheus.Counter
	ResponsesOptimized   prometheus.Counter
	OptimizationDuration prometheus.Histogram
	ContextRetrieved     prometheus.Counter
	TasksDecomposed      prometheus.Counter
	PrefixesWarmed       prometheus.Counter
}

var (
	metricsInstance *Metrics
	metricsOnce     sync.Once
)

// GetMetrics returns the singleton metrics instance.
func GetMetrics() *Metrics {
	metricsOnce.Do(func() {
		metricsInstance = newMetrics()
	})
	return metricsInstance
}

// newMetrics creates and registers all Prometheus metrics.
func newMetrics() *Metrics {
	m := &Metrics{
		// Cache metrics
		CacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "cache_hits_total",
			Help:      "Total number of semantic cache hits",
		}),
		CacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "cache_misses_total",
			Help:      "Total number of semantic cache misses",
		}),
		CacheSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "cache_size",
			Help:      "Current number of entries in the semantic cache",
		}),
		CacheEvictions: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "cache_evictions_total",
			Help:      "Total number of cache evictions",
		}),
		CacheLookupDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "cache_lookup_duration_seconds",
			Help:      "Duration of cache lookups in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 10), // 0.1ms to ~100ms
		}),

		// Validation metrics
		ValidationAttempts: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "validation_attempts_total",
			Help:      "Total number of structured output validation attempts",
		}),
		ValidationSuccesses: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "validation_successes_total",
			Help:      "Total number of successful validations",
		}),
		ValidationFailures: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "validation_failures_total",
			Help:      "Total number of failed validations",
		}),
		ValidationDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "validation_duration_seconds",
			Help:      "Duration of validation in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.00001, 2, 12),
		}),

		// Streaming metrics
		StreamsStarted: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "streams_started_total",
			Help:      "Total number of enhanced streams started",
		}),
		StreamsCompleted: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "streams_completed_total",
			Help:      "Total number of enhanced streams completed",
		}),
		StreamErrors: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "stream_errors_total",
			Help:      "Total number of stream errors",
		}),
		TokensStreamed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "tokens_streamed_total",
			Help:      "Total number of tokens streamed",
		}),
		StreamDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "stream_duration_seconds",
			Help:      "Duration of streams in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.1, 2, 10), // 100ms to ~100s
		}),
		TokensPerSecond: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "tokens_per_second",
			Help:      "Token throughput per stream",
			Buckets:   prometheus.LinearBuckets(0, 10, 20), // 0 to 200 tokens/sec
		}),

		// External service metrics
		ServiceRequests: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "service_requests_total",
			Help:      "Total number of requests to external optimization services",
		}, []string{"service", "method"}),
		ServiceErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "service_errors_total",
			Help:      "Total number of errors from external optimization services",
		}, []string{"service", "error_type"}),
		ServiceLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "service_latency_seconds",
			Help:      "Latency of external service calls in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 10), // 10ms to ~10s
		}, []string{"service", "method"}),
		ServiceAvailable: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "service_available",
			Help:      "Whether an external service is available (1=yes, 0=no)",
		}, []string{"service"}),

		// Pipeline metrics
		RequestsOptimized: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "requests_optimized_total",
			Help:      "Total number of requests optimized",
		}),
		ResponsesOptimized: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "responses_optimized_total",
			Help:      "Total number of responses optimized",
		}),
		OptimizationDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "optimization_duration_seconds",
			Help:      "Duration of request/response optimization in seconds",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 12),
		}),
		ContextRetrieved: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "context_retrieved_total",
			Help:      "Total number of times context was retrieved from documents",
		}),
		TasksDecomposed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "tasks_decomposed_total",
			Help:      "Total number of complex tasks decomposed",
		}),
		PrefixesWarmed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "superagent",
			Subsystem: "optimization",
			Name:      "prefixes_warmed_total",
			Help:      "Total number of prefixes warmed in cache",
		}),
	}

	return m
}

// RecordCacheHit records a cache hit with timing.
func (m *Metrics) RecordCacheHit(duration time.Duration) {
	m.CacheHits.Inc()
	m.CacheLookupDuration.Observe(duration.Seconds())
}

// RecordCacheMiss records a cache miss with timing.
func (m *Metrics) RecordCacheMiss(duration time.Duration) {
	m.CacheMisses.Inc()
	m.CacheLookupDuration.Observe(duration.Seconds())
}

// RecordValidation records a validation attempt.
func (m *Metrics) RecordValidation(success bool, duration time.Duration) {
	m.ValidationAttempts.Inc()
	m.ValidationDuration.Observe(duration.Seconds())
	if success {
		m.ValidationSuccesses.Inc()
	} else {
		m.ValidationFailures.Inc()
	}
}

// RecordStreamComplete records a completed stream.
func (m *Metrics) RecordStreamComplete(duration time.Duration, tokens int) {
	m.StreamsCompleted.Inc()
	m.StreamDuration.Observe(duration.Seconds())
	m.TokensStreamed.Add(float64(tokens))
	if duration.Seconds() > 0 {
		m.TokensPerSecond.Observe(float64(tokens) / duration.Seconds())
	}
}

// RecordServiceCall records a call to an external service.
func (m *Metrics) RecordServiceCall(service, method string, duration time.Duration, err error) {
	m.ServiceRequests.WithLabelValues(service, method).Inc()
	m.ServiceLatency.WithLabelValues(service, method).Observe(duration.Seconds())
	if err != nil {
		m.ServiceErrors.WithLabelValues(service, "request_failed").Inc()
	}
}

// SetServiceAvailable sets the availability status of a service.
func (m *Metrics) SetServiceAvailable(service string, available bool) {
	val := 0.0
	if available {
		val = 1.0
	}
	m.ServiceAvailable.WithLabelValues(service).Set(val)
}

// RecordOptimization records an optimization operation.
func (m *Metrics) RecordOptimization(isRequest bool, duration time.Duration) {
	m.OptimizationDuration.Observe(duration.Seconds())
	if isRequest {
		m.RequestsOptimized.Inc()
	} else {
		m.ResponsesOptimized.Inc()
	}
}

// GetCacheHitRate returns the current cache hit rate.
func (m *Metrics) GetCacheHitRate() float64 {
	// Note: This is a simplified implementation. In production,
	// you'd want to track this with a sliding window.
	return 0 // Prometheus counters don't support reading values directly
}

// MetricsSnapshot provides a point-in-time view of key metrics.
type MetricsSnapshot struct {
	CacheHits         int64   `json:"cache_hits"`
	CacheMisses       int64   `json:"cache_misses"`
	CacheHitRate      float64 `json:"cache_hit_rate"`
	CacheSize         int     `json:"cache_size"`
	ValidationSuccess float64 `json:"validation_success_rate"`
	StreamsActive     int     `json:"streams_active"`
	ServicesHealthy   int     `json:"services_healthy"`
}
