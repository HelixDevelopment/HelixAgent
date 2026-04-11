package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// HTTPMetrics holds per-handler Prometheus metrics for HTTP request tracking.
type HTTPMetrics struct {
	requestDuration *prometheus.HistogramVec
	requestsTotal   *prometheus.CounterVec
	requestErrors   *prometheus.CounterVec
}

// NewHTTPMetrics creates a new HTTPMetrics instance and registers the
// metrics with the default global Prometheus registry.
func NewHTTPMetrics() *HTTPMetrics {
	return NewHTTPMetricsWithRegistry(nil)
}

// NewHTTPMetricsWithRegistry creates a new HTTPMetrics instance and registers
// the metrics with the provided registry. If registry is nil, the default
// global Prometheus registry is used.
func NewHTTPMetricsWithRegistry(registry *prometheus.Registry) *HTTPMetrics {
	m := &HTTPMetrics{
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "helixagent_http_request_duration_seconds",
				Help:    "Duration of HTTP requests in seconds.",
				Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
			},
			[]string{"method", "path", "status_code"},
		),

		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helixagent_http_requests_total",
				Help: "Total number of HTTP requests.",
			},
			[]string{"method", "path", "status_code"},
		),

		requestErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helixagent_http_request_errors_total",
				Help: "Total number of HTTP request errors (status >= 500).",
			},
			[]string{"method", "path"},
		),
	}

	// Register each collector idempotently. If a collector with the same
	// name is already registered (common in test suites that construct
	// multiple routers against the same process-global registry, and
	// in hot-reload scenarios), reuse the existing instance via
	// prometheus.AlreadyRegisteredError.ExistingCollector instead of
	// panicking via MustRegister. This makes the constructor safe to
	// call more than once without having to pass a fresh registry.
	reg := prometheus.Registerer(prometheus.DefaultRegisterer)
	if registry != nil {
		reg = registry
	}
	m.requestDuration = registerIdempotent(reg, m.requestDuration).(*prometheus.HistogramVec)
	m.requestsTotal = registerIdempotent(reg, m.requestsTotal).(*prometheus.CounterVec)
	m.requestErrors = registerIdempotent(reg, m.requestErrors).(*prometheus.CounterVec)

	return m
}

// registerIdempotent attempts to register c with reg. If reg already has
// a collector with the same name, it returns the pre-existing instance
// so that callers can keep using it transparently. Any other error
// panics — those indicate genuine misconfiguration (e.g. label
// mismatch), not duplicate registration.
func registerIdempotent(reg prometheus.Registerer, c prometheus.Collector) prometheus.Collector {
	if err := reg.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector
		}
		panic(err)
	}
	return c
}

// Middleware returns a Gin middleware handler that records per-handler
// request duration, total request count, and error count. It uses
// c.FullPath() for path normalization to avoid label cardinality explosion
// from dynamic path segments.
func (m *HTTPMetrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		// Use FullPath (the route pattern) to avoid cardinality explosion.
		// Falls back to "unknown" for unmatched routes (404s from the
		// router itself).
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		method := c.Request.Method
		status := strconv.Itoa(c.Writer.Status())
		elapsed := time.Since(start).Seconds()

		m.requestDuration.WithLabelValues(method, path, status).Observe(elapsed)
		m.requestsTotal.WithLabelValues(method, path, status).Inc()

		if c.Writer.Status() >= 500 {
			m.requestErrors.WithLabelValues(method, path).Inc()
		}
	}
}
