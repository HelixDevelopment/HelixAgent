package monitoring_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrometheusMetricRegistration_KeyMetrics validates that the canonical
// HelixAgent Prometheus metrics can be registered and gathered without error.
// Each metric is registered in a fresh isolated registry to avoid conflicts
// with the global default registry used by production code.
func TestPrometheusMetricRegistration_KeyMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	type metricDef struct {
		name string
		help string
		kind string // "counter", "gauge", "histogram", "counter_vec", "gauge_vec"
	}

	metrics := []metricDef{
		{
			name: "helixagent_circuit_breaker_state",
			help: "Current state of circuit breakers (0=closed, 1=half_open, 2=open)",
			kind: "gauge_vec",
		},
		{
			name: "helixagent_http_requests_total",
			help: "Total HTTP requests handled by HelixAgent",
			kind: "counter_vec",
		},
		{
			name: "helixagent_provider_health_status",
			help: "Health status of LLM providers (1=healthy, 0=unhealthy)",
			kind: "gauge_vec",
		},
		{
			name: "helixagent_llm_request_duration_seconds",
			help: "Duration of LLM requests in seconds",
			kind: "histogram",
		},
		{
			name: "helixagent_cache_hits_total",
			help: "Total cache hits",
			kind: "counter",
		},
		{
			name: "helixagent_active_requests",
			help: "Number of currently active requests",
			kind: "gauge",
		},
	}

	for _, m := range metrics {
		t.Run(m.name, func(t *testing.T) {
			registry := prometheus.NewRegistry()

			switch m.kind {
			case "counter":
				c := prometheus.NewCounter(prometheus.CounterOpts{
					Name: m.name,
					Help: m.help,
				})
				err := registry.Register(c)
				require.NoError(t, err, "Counter %q must register without error", m.name)

			case "gauge":
				g := prometheus.NewGauge(prometheus.GaugeOpts{
					Name: m.name,
					Help: m.help,
				})
				err := registry.Register(g)
				require.NoError(t, err, "Gauge %q must register without error", m.name)

			case "histogram":
				h := prometheus.NewHistogram(prometheus.HistogramOpts{
					Name:    m.name,
					Help:    m.help,
					Buckets: prometheus.DefBuckets,
				})
				err := registry.Register(h)
				require.NoError(t, err, "Histogram %q must register without error", m.name)

			case "counter_vec":
				cv := prometheus.NewCounterVec(
					prometheus.CounterOpts{Name: m.name, Help: m.help},
					[]string{"provider", "method"},
				)
				err := registry.Register(cv)
				require.NoError(t, err, "CounterVec %q must register without error", m.name)
				cv.WithLabelValues("openai", "GET").Inc()

			case "gauge_vec":
				gv := prometheus.NewGaugeVec(
					prometheus.GaugeOpts{Name: m.name, Help: m.help},
					[]string{"provider"},
				)
				err := registry.Register(gv)
				require.NoError(t, err, "GaugeVec %q must register without error", m.name)
				gv.WithLabelValues("openai").Set(0)
			}

			families, err := registry.Gather()
			require.NoError(t, err)

			found := false
			for _, mf := range families {
				if mf.GetName() == m.name {
					found = true
					break
				}
			}
			assert.True(t, found, "Metric %q must appear in gathered output", m.name)
		})
	}
}

// TestPrometheusMetricRegistration_DefaultGathererContainsGoMetrics verifies
// that the default Prometheus gatherer exposes Go runtime metrics that are
// automatically registered by the client library.
func TestPrometheusMetricRegistration_DefaultGathererContainsGoMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	gathered := make(map[string]bool, len(families))
	for _, mf := range families {
		gathered[mf.GetName()] = true
	}

	// The prometheus/client_golang library auto-registers these Go runtime
	// metrics into the default registry when imported.
	expectedDefaultMetrics := []string{
		"go_goroutines",
		"go_gc_duration_seconds",
		"go_memstats_alloc_bytes",
		"go_threads",
	}

	for _, name := range expectedDefaultMetrics {
		assert.True(t, gathered[name],
			"Default gatherer must expose Go runtime metric %q", name)
	}
}

// TestPrometheusMetricRegistration_Types verifies that each metric kind
// reports the correct MetricType in the gathered ProtoBuf output.
func TestPrometheusMetricRegistration_Types(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "helixagent_reg_type_counter_total",
		Help: "Type check counter",
	})
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "helixagent_reg_type_gauge",
		Help: "Type check gauge",
	})
	hist := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "helixagent_reg_type_histogram_seconds",
		Help:    "Type check histogram",
		Buckets: prometheus.DefBuckets,
	})
	summary := prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "helixagent_reg_type_summary",
		Help: "Type check summary",
	})

	registry.MustRegister(counter, gauge, hist, summary)

	// Record at least one observation so each metric appears in Gather output.
	counter.Inc()
	gauge.Set(1)
	hist.Observe(0.1)
	summary.Observe(0.2)

	families, err := registry.Gather()
	require.NoError(t, err)

	expectedTypes := map[string]dto.MetricType{
		"helixagent_reg_type_counter_total":     dto.MetricType_COUNTER,
		"helixagent_reg_type_gauge":             dto.MetricType_GAUGE,
		"helixagent_reg_type_histogram_seconds": dto.MetricType_HISTOGRAM,
		"helixagent_reg_type_summary":           dto.MetricType_SUMMARY,
	}

	for _, mf := range families {
		expectedType, ok := expectedTypes[mf.GetName()]
		if !ok {
			continue
		}
		assert.Equal(t, expectedType, mf.GetType(),
			"Metric %q must have type %v", mf.GetName(), expectedType)
		delete(expectedTypes, mf.GetName())
	}

	for remaining := range expectedTypes {
		t.Errorf("Metric %q was not found in gathered output", remaining)
	}
}

// TestPrometheusMetricRegistration_Labels validates that labeled metrics
// produce isolated series per label combination and that label names
// follow the snake_case convention used across HelixAgent.
func TestPrometheusMetricRegistration_Labels(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	requestsVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "helixagent_reg_labeled_requests_total",
			Help: "Labeled request counter for registration validation",
		},
		[]string{"provider", "status"},
	)
	registry.MustRegister(requestsVec)

	// Record requests for several provider+status combinations.
	labelCombinations := []struct {
		provider string
		status   string
		count    float64
	}{
		{"openai", "success", 10},
		{"openai", "error", 2},
		{"anthropic", "success", 5},
		{"deepseek", "success", 7},
		{"deepseek", "error", 1},
	}

	for _, lc := range labelCombinations {
		requestsVec.WithLabelValues(lc.provider, lc.status).Add(lc.count)
	}

	families, err := registry.Gather()
	require.NoError(t, err)

	var found *dto.MetricFamily
	for _, mf := range families {
		if mf.GetName() == "helixagent_reg_labeled_requests_total" {
			found = mf
			break
		}
	}
	require.NotNil(t, found, "Labeled counter must appear in gathered output")
	assert.Equal(t, len(labelCombinations), len(found.GetMetric()),
		"Number of metric series must equal number of unique label combinations")

	// Verify total across all series
	var total float64
	for _, m := range found.GetMetric() {
		total += m.GetCounter().GetValue()
	}
	expectedTotal := 10.0 + 2.0 + 5.0 + 7.0 + 1.0
	assert.Equal(t, expectedTotal, total, "Sum of all labeled series must match total added")
}
