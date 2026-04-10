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

	if registry != nil {
		registry.MustRegister(m.requestDuration)
		registry.MustRegister(m.requestsTotal)
		registry.MustRegister(m.requestErrors)
	} else {
		prometheus.MustRegister(m.requestDuration)
		prometheus.MustRegister(m.requestsTotal)
		prometheus.MustRegister(m.requestErrors)
	}

	return m
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
