package metrics

// Unit tests for the Phase-3 Prometheus collector wiring.
// Hermetic: a local prometheus.NewRegistry() stands in for the global
// DefaultRegisterer so parallel test runs cannot collide with each other
// or with any metrics registered elsewhere in the test binary.
//
// We deliberately avoid importing prometheus/client_golang/prometheus/testutil
// because that subpackage is not in the project's vendor directory. Instead
// we read values straight from reg.Gather(), which is also closer to what
// Prometheus itself does at scrape time.

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSource is a mutable Phase3MetricsSource that lets tests drive
// each metric independently via atomic fields.
type stubSource struct {
	pending     atomic.Int64
	pendingCap  atomic.Int64
	rejected    atomic.Uint64
	keys        atomic.Int64
	keysDropped atomic.Int64
}

func (s *stubSource) EnsemblePendingCount() int64   { return s.pending.Load() }
func (s *stubSource) EnsemblePendingCap() int64     { return s.pendingCap.Load() }
func (s *stubSource) EnsembleTasksRejected() uint64 { return s.rejected.Load() }
func (s *stubSource) GuardrailStatsKeyCount() int64 { return s.keys.Load() }
func (s *stubSource) GuardrailStatsDropped() int64  { return s.keysDropped.Load() }

// metricValue scrapes the registry and returns the current value of the
// named metric. Fails the test if the metric is not present. Works for
// both gauges and counters — it returns whichever one is populated on
// the underlying protobuf.
func metricValue(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		metrics := mf.GetMetric()
		require.NotEmpty(t, metrics, "metric %q has no samples", name)
		m := metrics[0]
		if m.Gauge != nil {
			return m.Gauge.GetValue()
		}
		if m.Counter != nil {
			return m.Counter.GetValue()
		}
		t.Fatalf("metric %q is neither gauge nor counter", name)
	}
	t.Fatalf("metric %q not found in registry", name)
	return 0 // unreachable
}

// metricNames returns the sorted list of metric-family names in the
// registry — used for presence assertions.
func metricNames(t *testing.T, reg *prometheus.Registry) []string {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	out := make([]string, 0, len(mfs))
	for _, mf := range mfs {
		out = append(out, mf.GetName())
	}
	return out
}

func TestRegisterPhase3Metrics_NilSource(t *testing.T) {
	reg := prometheus.NewRegistry()
	_, err := RegisterPhase3Metrics(reg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source must not be nil")
}

func TestRegisterPhase3Metrics_HappyPath_InitialZero(t *testing.T) {
	reg := prometheus.NewRegistry()
	src := &stubSource{}

	g, err := RegisterPhase3Metrics(reg, src)
	require.NoError(t, err)
	require.NotNil(t, g)
	t.Cleanup(g.Unregister)

	// Freshly constructed stub: every value is the int64 zero.
	assert.Equal(t, float64(0), metricValue(t, reg, "helixagent_ensemble_pending_results"))
	assert.Equal(t, float64(0), metricValue(t, reg, "helixagent_ensemble_pending_results_cap"))
	assert.Equal(t, float64(0), metricValue(t, reg, "helixagent_ensemble_tasks_rejected_total"))
	assert.Equal(t, float64(0), metricValue(t, reg, "helixagent_guardrails_stats_keys"))
	assert.Equal(t, float64(0), metricValue(t, reg, "helixagent_guardrails_stats_dropped_total"))
}

func TestRegisterPhase3Metrics_LiveValuesOnScrape(t *testing.T) {
	reg := prometheus.NewRegistry()
	src := &stubSource{}

	g, err := RegisterPhase3Metrics(reg, src)
	require.NoError(t, err)
	t.Cleanup(g.Unregister)

	// Change the underlying values and assert each collector reports
	// the fresh number — this is the core contract of GaugeFunc: values
	// are pulled at scrape time, not cached.
	src.pending.Store(42)
	src.pendingCap.Store(10_000)
	src.rejected.Store(7)
	src.keys.Store(128)
	src.keysDropped.Store(3)

	assert.Equal(t, float64(42), metricValue(t, reg, "helixagent_ensemble_pending_results"))
	assert.Equal(t, float64(10_000), metricValue(t, reg, "helixagent_ensemble_pending_results_cap"))
	assert.Equal(t, float64(7), metricValue(t, reg, "helixagent_ensemble_tasks_rejected_total"))
	assert.Equal(t, float64(128), metricValue(t, reg, "helixagent_guardrails_stats_keys"))
	assert.Equal(t, float64(3), metricValue(t, reg, "helixagent_guardrails_stats_dropped_total"))

	// Mutate again and reassert — GaugeFunc must NOT cache.
	src.pending.Store(0)
	src.keys.Store(0)
	assert.Equal(t, float64(0), metricValue(t, reg, "helixagent_ensemble_pending_results"))
	assert.Equal(t, float64(0), metricValue(t, reg, "helixagent_guardrails_stats_keys"))
}

func TestRegisterPhase3Metrics_MetricNamesPresent(t *testing.T) {
	reg := prometheus.NewRegistry()
	src := &stubSource{}

	g, err := RegisterPhase3Metrics(reg, src)
	require.NoError(t, err)
	t.Cleanup(g.Unregister)

	wanted := []string{
		"helixagent_ensemble_pending_results",
		"helixagent_ensemble_pending_results_cap",
		"helixagent_ensemble_tasks_rejected_total",
		"helixagent_guardrails_stats_keys",
		"helixagent_guardrails_stats_dropped_total",
	}
	got := metricNames(t, reg)
	for _, name := range wanted {
		assert.Containsf(t, got, name, "registry must expose %q", name)
	}
	assert.Len(t, got, len(wanted), "exactly 5 metric families registered")
}

func TestRegisterPhase3Metrics_DoubleRegistration_Errors(t *testing.T) {
	reg := prometheus.NewRegistry()
	src := &stubSource{}

	g, err := RegisterPhase3Metrics(reg, src)
	require.NoError(t, err)
	t.Cleanup(g.Unregister)

	// Registering again against the same registry must fail — Prometheus
	// enforces unique metric names per registry. The rollback path in
	// RegisterPhase3Metrics ensures no partial state is left behind.
	_, err = RegisterPhase3Metrics(reg, src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "register")
}

func TestPhase3Gauges_Unregister_Idempotent(t *testing.T) {
	reg := prometheus.NewRegistry()
	src := &stubSource{}

	g, err := RegisterPhase3Metrics(reg, src)
	require.NoError(t, err)

	// Unregister once.
	g.Unregister()
	// Unregister again must not panic.
	g.Unregister()
	// Nil receiver also safe.
	var nilG *Phase3Gauges
	nilG.Unregister()

	// After unregistration, re-registering on the SAME registry must
	// now succeed — proves Unregister fully released every collector.
	g2, err := RegisterPhase3Metrics(reg, src)
	require.NoError(t, err)
	g2.Unregister()
}

// TestRegisterPhase3Metrics_CounterSemantics confirms that the two
// *_total metrics are registered as counters (monotonic) rather than
// gauges. A Prometheus consumer needs this distinction to apply rate()
// correctly against the rejected/dropped counters.
func TestRegisterPhase3Metrics_CounterSemantics(t *testing.T) {
	reg := prometheus.NewRegistry()
	src := &stubSource{}
	src.rejected.Store(1)
	src.keysDropped.Store(1)

	g, err := RegisterPhase3Metrics(reg, src)
	require.NoError(t, err)
	t.Cleanup(g.Unregister)

	mfs, err := reg.Gather()
	require.NoError(t, err)

	counterNames := map[string]bool{
		"helixagent_ensemble_tasks_rejected_total":  false,
		"helixagent_guardrails_stats_dropped_total": false,
	}
	gaugeNames := map[string]bool{
		"helixagent_ensemble_pending_results":     false,
		"helixagent_ensemble_pending_results_cap": false,
		"helixagent_guardrails_stats_keys":        false,
	}

	for _, mf := range mfs {
		name := mf.GetName()
		metrics := mf.GetMetric()
		require.NotEmpty(t, metrics)
		m := metrics[0]
		if _, ok := counterNames[name]; ok {
			require.NotNil(t, m.Counter, "%s must be a counter", name)
			require.Nil(t, m.Gauge, "%s must not be a gauge", name)
			counterNames[name] = true
		} else if _, ok := gaugeNames[name]; ok {
			require.NotNil(t, m.Gauge, "%s must be a gauge", name)
			require.Nil(t, m.Counter, "%s must not be a counter", name)
			gaugeNames[name] = true
		} else {
			t.Errorf("unexpected metric %q in registry", name)
		}
	}
	for name, seen := range counterNames {
		assert.True(t, seen, "counter %s missing from gather", name)
	}
	for name, seen := range gaugeNames {
		assert.True(t, seen, "gauge %s missing from gather", name)
	}
}

// smokeTestSourceString is here to prove the helpers survive a full
// exposition loop without panicking. Not a contract assertion.
func TestRegisterPhase3Metrics_FullScrapeDoesNotPanic(t *testing.T) {
	reg := prometheus.NewRegistry()
	src := &stubSource{}
	src.pending.Store(17)
	src.pendingCap.Store(9999)
	src.rejected.Store(42)
	src.keys.Store(5)
	src.keysDropped.Store(0)

	g, err := RegisterPhase3Metrics(reg, src)
	require.NoError(t, err)
	t.Cleanup(g.Unregister)

	mfs, err := reg.Gather()
	require.NoError(t, err)
	require.Len(t, mfs, 5)

	// Debug aid: print the rendered values in case something changes in
	// future and the assertions above start failing mysteriously.
	for _, mf := range mfs {
		t.Logf("metric=%s samples=%d help=%q",
			mf.GetName(), len(mf.GetMetric()), mf.GetHelp())
	}
	_ = fmt.Sprint(mfs) // keep fmt import honest when tests tweak above
}
