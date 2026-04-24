package monitoring_test

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// simulatedCircuitBreaker is a minimal in-test circuit breaker that emits
// Prometheus metrics for state transitions and failure counts. It mirrors the
// metric names used by the HelixAgent monitoring package.
type simulatedCircuitBreaker struct {
	mu           sync.Mutex
	state        int // 0=closed, 1=open, 2=half_open
	failures     int
	stateGauge   *prometheus.GaugeVec
	failCounter  *prometheus.CounterVec
	successCount *prometheus.CounterVec
	openGauge    prometheus.Gauge
}

func newSimulatedCircuitBreaker(registry *prometheus.Registry, suffix string) *simulatedCircuitBreaker {
	sg := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "helixagent_cb_metric_state_" + suffix,
		Help: "CB state (0=closed, 1=open, 2=half_open)",
	}, []string{"provider"})
	fc := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "helixagent_cb_metric_failures_total_" + suffix,
		Help: "CB failure count",
	}, []string{"provider"})
	sc := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "helixagent_cb_metric_successes_total_" + suffix,
		Help: "CB success count",
	}, []string{"provider"})
	og := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "helixagent_cb_metric_open_count_" + suffix,
		Help: "Number of open circuit breakers",
	})

	registry.MustRegister(sg, fc, sc, og)

	return &simulatedCircuitBreaker{
		stateGauge:   sg,
		failCounter:  fc,
		successCount: sc,
		openGauge:    og,
	}
}

func (cb *simulatedCircuitBreaker) recordFailure(provider string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.failCounter.WithLabelValues(provider).Inc()
	if cb.failures >= 3 && cb.state == 0 {
		cb.state = 1 // open
		cb.stateGauge.WithLabelValues(provider).Set(1)
		cb.openGauge.Inc()
	}
}

func (cb *simulatedCircuitBreaker) halfOpen(provider string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == 1 {
		cb.state = 2
		cb.stateGauge.WithLabelValues(provider).Set(2)
	}
}

func (cb *simulatedCircuitBreaker) recordSuccess(provider string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.successCount.WithLabelValues(provider).Inc()
	if cb.state == 2 {
		cb.state = 0
		cb.failures = 0
		cb.stateGauge.WithLabelValues(provider).Set(0)
		cb.openGauge.Dec()
	}
}

// gatherFamily is a helper that returns the named MetricFamily from a registry.
func gatherFamily(t *testing.T, reg *prometheus.Registry, name string) *dto.MetricFamily {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range families {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

// TestCircuitBreakerMetric_InitialStateClosed verifies that a newly created
// circuit breaker emits a state gauge of 0 (closed) before any failures.
func TestCircuitBreakerMetric_InitialStateClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()
	cb := newSimulatedCircuitBreaker(registry, "init")

	// Touch the gauge so it appears in gathered output.
	cb.stateGauge.WithLabelValues("openai").Set(0)

	mf := gatherFamily(t, registry, "helixagent_cb_metric_state_init")
	require.NotNil(t, mf, "State gauge must be present after initialisation")

	metrics := mf.GetMetric()
	require.Len(t, metrics, 1)
	assert.Equal(t, float64(0), metrics[0].GetGauge().GetValue(),
		"Initial circuit breaker state must be 0 (closed)")
}

// TestCircuitBreakerMetric_OpensAfterFailureThreshold verifies that after the
// failure threshold (3 failures) is reached the state gauge transitions from
// 0 (closed) to 1 (open) and the open-count gauge increments.
func TestCircuitBreakerMetric_OpensAfterFailureThreshold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()
	cb := newSimulatedCircuitBreaker(registry, "open")

	provider := "anthropic"
	cb.stateGauge.WithLabelValues(provider).Set(0) // initial closed state

	// Record 3 failures — should trip the breaker.
	for i := 0; i < 3; i++ {
		cb.recordFailure(provider)
	}

	// Verify state gauge is now 1 (open).
	stateMF := gatherFamily(t, registry, "helixagent_cb_metric_state_open")
	require.NotNil(t, stateMF)
	require.Len(t, stateMF.GetMetric(), 1)
	assert.Equal(t, float64(1), stateMF.GetMetric()[0].GetGauge().GetValue(),
		"Circuit breaker must be open (state=1) after 3 failures")

	// Verify failure counter equals 3.
	failMF := gatherFamily(t, registry, "helixagent_cb_metric_failures_total_open")
	require.NotNil(t, failMF)
	require.Len(t, failMF.GetMetric(), 1)
	assert.Equal(t, float64(3), failMF.GetMetric()[0].GetCounter().GetValue(),
		"Failure counter must equal 3")

	// Verify open-count gauge is 1.
	openMF := gatherFamily(t, registry, "helixagent_cb_metric_open_count_open")
	require.NotNil(t, openMF)
	require.Len(t, openMF.GetMetric(), 1)
	assert.Equal(t, float64(1), openMF.GetMetric()[0].GetGauge().GetValue(),
		"Open-count gauge must be 1 when circuit is open")
}

// TestCircuitBreakerMetric_HalfOpenTransition validates that transitioning from
// open to half-open updates the state gauge to 2.
func TestCircuitBreakerMetric_HalfOpenTransition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()
	cb := newSimulatedCircuitBreaker(registry, "halfopen")

	provider := "deepseek"
	cb.stateGauge.WithLabelValues(provider).Set(0)

	// Trip the breaker.
	for i := 0; i < 3; i++ {
		cb.recordFailure(provider)
	}

	// Simulate timeout expiry: transition to half-open.
	cb.halfOpen(provider)

	stateMF := gatherFamily(t, registry, "helixagent_cb_metric_state_halfopen")
	require.NotNil(t, stateMF)
	require.Len(t, stateMF.GetMetric(), 1)
	assert.Equal(t, float64(2), stateMF.GetMetric()[0].GetGauge().GetValue(),
		"State must be 2 (half-open) after half-open transition")
}

// TestCircuitBreakerMetric_FullCycle validates the complete closed→open→
// half-open→closed cycle and checks that all metric values are correct at each
// stage.
func TestCircuitBreakerMetric_FullCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()
	cb := newSimulatedCircuitBreaker(registry, "cycle")

	provider := "gemini"
	cb.stateGauge.WithLabelValues(provider).Set(0)

	// Stage 1: closed — record 3 failures to open.
	for i := 0; i < 3; i++ {
		cb.recordFailure(provider)
	}
	stateMF := gatherFamily(t, registry, "helixagent_cb_metric_state_cycle")
	require.NotNil(t, stateMF)
	assert.Equal(t, float64(1), stateMF.GetMetric()[0].GetGauge().GetValue(),
		"Stage 1: state must be open (1)")

	// Stage 2: half-open.
	cb.halfOpen(provider)
	stateMF = gatherFamily(t, registry, "helixagent_cb_metric_state_cycle")
	require.NotNil(t, stateMF)
	assert.Equal(t, float64(2), stateMF.GetMetric()[0].GetGauge().GetValue(),
		"Stage 2: state must be half-open (2)")

	// Stage 3: successful probe → closed.
	cb.recordSuccess(provider)
	stateMF = gatherFamily(t, registry, "helixagent_cb_metric_state_cycle")
	require.NotNil(t, stateMF)
	assert.Equal(t, float64(0), stateMF.GetMetric()[0].GetGauge().GetValue(),
		"Stage 3: state must be closed (0) after successful probe")

	// Open-count gauge must return to 0.
	openMF := gatherFamily(t, registry, "helixagent_cb_metric_open_count_cycle")
	require.NotNil(t, openMF)
	assert.Equal(t, float64(0), openMF.GetMetric()[0].GetGauge().GetValue(),
		"Open-count gauge must be 0 after circuit closes")

	// Success counter must equal 1.
	succMF := gatherFamily(t, registry, "helixagent_cb_metric_successes_total_cycle")
	require.NotNil(t, succMF)
	require.Len(t, succMF.GetMetric(), 1)
	assert.Equal(t, float64(1), succMF.GetMetric()[0].GetCounter().GetValue(),
		"Success counter must equal 1 after one successful probe")
}

// TestCircuitBreakerMetric_MultiProviderIsolation verifies that circuit breaker
// metrics for different providers are fully isolated — a state change for one
// provider must not affect another.
func TestCircuitBreakerMetric_MultiProviderIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()
	cb := newSimulatedCircuitBreaker(registry, "iso")

	// Initialise two providers to closed.
	cb.stateGauge.WithLabelValues("openai").Set(0)
	cb.stateGauge.WithLabelValues("cohere").Set(0)

	// Trip only the "cohere" circuit.
	for i := 0; i < 3; i++ {
		cb.recordFailure("cohere")
	}

	stateMF := gatherFamily(t, registry, "helixagent_cb_metric_state_iso")
	require.NotNil(t, stateMF)

	states := make(map[string]float64)
	for _, m := range stateMF.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "provider" {
				states[lp.GetValue()] = m.GetGauge().GetValue()
			}
		}
	}

	assert.Equal(t, float64(0), states["openai"],
		"openai circuit must remain closed (0) when only cohere trips")
	assert.Equal(t, float64(1), states["cohere"],
		"cohere circuit must be open (1) after 3 failures")
}

// TestCircuitBreakerMetric_CounterMonotonicity verifies that the failure and
// success counters only increase — they must never decrease.
func TestCircuitBreakerMetric_CounterMonotonicity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping monitoring test in short mode")  // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()
	cb := newSimulatedCircuitBreaker(registry, "mono")

	provider := "mistral"
	cb.stateGauge.WithLabelValues(provider).Set(0)

	var prevFailures, prevSuccesses float64

	for round := 0; round < 5; round++ {
		cb.recordFailure(provider)

		failMF := gatherFamily(t, registry, "helixagent_cb_metric_failures_total_mono")
		require.NotNil(t, failMF)
		cur := failMF.GetMetric()[0].GetCounter().GetValue()
		assert.GreaterOrEqual(t, cur, prevFailures,
			"Failure counter must be monotonically non-decreasing at round %d", round)
		prevFailures = cur
	}

	// Reset breaker state manually so we can record successes.
	cb.mu.Lock()
	cb.state = 2
	cb.stateGauge.WithLabelValues(provider).Set(2)
	cb.mu.Unlock()

	for round := 0; round < 3; round++ {
		cb.recordSuccess(provider)

		succMF := gatherFamily(t, registry, "helixagent_cb_metric_successes_total_mono")
		require.NotNil(t, succMF)
		cur := succMF.GetMetric()[0].GetCounter().GetValue()
		assert.GreaterOrEqual(t, cur, prevSuccesses,
			"Success counter must be monotonically non-decreasing at round %d", round)
		prevSuccesses = cur
	}
}
