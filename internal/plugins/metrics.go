package plugins

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/superagent/superagent/internal/utils"
)

var (
	// Plugin metrics
	PluginRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "plugin_requests_total",
			Help: "Total number of requests to plugins",
		},
		[]string{"plugin_name", "method"},
	)

	PluginRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "plugin_request_duration_seconds",
			Help:    "Request duration for plugin operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"plugin_name", "method"},
	)

	PluginHealthStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "plugin_health_status",
			Help: "Health status of plugins (1=healthy, 0=unhealthy)",
		},
		[]string{"plugin_name"},
	)

	PluginLoadErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "plugin_load_errors_total",
			Help: "Total number of plugin load errors",
		},
		[]string{"plugin_name"},
	)

	PluginActiveCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "plugin_active_count",
			Help: "Number of currently active plugins",
		},
	)
)

// MetricsCollector collects and exposes plugin metrics
type MetricsCollector struct {
	registry *Registry
	health   *HealthMonitor
	stopCh   chan struct{}
	running  bool
}

func NewMetricsCollector(registry *Registry, health *HealthMonitor) *MetricsCollector {
	return &MetricsCollector{
		registry: registry,
		health:   health,
		stopCh:   make(chan struct{}),
		running:  false,
	}
}

func (m *MetricsCollector) StartCollection() {
	if m.running {
		return
	}
	m.running = true

	// Update metrics periodically
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-m.stopCh:
				utils.GetLogger().Debug("Metrics collector stopped")
				return
			case <-ticker.C:
				m.collectMetrics()
			}
		}
	}()
}

// StopCollection stops the metrics collection goroutine
func (m *MetricsCollector) StopCollection() {
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
}

func (m *MetricsCollector) collectMetrics() {
	plugins := m.registry.List()
	activeCount := 0

	for _, name := range plugins {
		health, exists := m.health.GetHealth(name)
		if exists {
			status := 0.0
			if health.Status == "healthy" && !health.CircuitOpen {
				status = 1.0
				activeCount++
			}
			PluginHealthStatus.WithLabelValues(name).Set(status)
		}
	}

	PluginActiveCount.Set(float64(activeCount))
	utils.GetLogger().Debugf("Collected metrics for %d plugins", len(plugins))
}

func (m *MetricsCollector) RecordRequest(pluginName, method string, duration time.Duration) {
	PluginRequestsTotal.WithLabelValues(pluginName, method).Inc()
	PluginRequestDuration.WithLabelValues(pluginName, method).Observe(duration.Seconds())
}

func (m *MetricsCollector) RecordLoadError(pluginName string) {
	PluginLoadErrors.WithLabelValues(pluginName).Inc()
}
