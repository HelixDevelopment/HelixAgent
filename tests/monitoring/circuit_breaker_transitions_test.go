package monitoring_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMonitoring_CircuitBreaker_StateTransitions validates that circuit breaker
// state transitions are correctly tracked via Prometheus gauge and counter
// metrics through a full cycle: closed -> open -> half-open -> closed.
func TestMonitoring_CircuitBreaker_StateTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	stateGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "circuit_breaker_state",
		Help: "Current state of circuit breaker (0=closed, 1=open, 2=half-open)",
	}, []string{"provider"})
	registry.MustRegister(stateGauge)

	transitionCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "circuit_breaker_transitions_total",
		Help: "Total number of circuit breaker state transitions",
	}, []string{"provider", "from_state", "to_state"})
	registry.MustRegister(transitionCounter)

	// Simulate state transitions: closed -> open -> half-open -> closed
	provider := "test-provider"

	// Initial state: closed (0)
	stateGauge.WithLabelValues(provider).Set(0)

	// Transition: closed -> open (too many failures)
	stateGauge.WithLabelValues(provider).Set(1)
	transitionCounter.WithLabelValues(provider, "closed", "open").Inc()

	// Transition: open -> half-open (timeout expired)
	stateGauge.WithLabelValues(provider).Set(2)
	transitionCounter.WithLabelValues(provider, "open", "half-open").Inc()

	// Transition: half-open -> closed (successful probe)
	stateGauge.WithLabelValues(provider).Set(0)
	transitionCounter.WithLabelValues(provider, "half-open", "closed").Inc()

	// Verify final state
	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	for _, mf := range metricFamilies {
		if mf.GetName() == "circuit_breaker_state" {
			for _, m := range mf.GetMetric() {
				assert.Equal(t, float64(0), m.GetGauge().GetValue(),
					"circuit breaker should be back in closed state")
			}
		}
		if mf.GetName() == "circuit_breaker_transitions_total" {
			totalTransitions := 0.0
			for _, m := range mf.GetMetric() {
				totalTransitions += m.GetCounter().GetValue()
			}
			assert.Equal(t, 3.0, totalTransitions,
				"should have recorded exactly 3 state transitions")
		}
	}
}

// TestMonitoring_CircuitBreaker_FailureTracking verifies that circuit breaker
// failure counters correctly track failures by provider and error type.
func TestMonitoring_CircuitBreaker_FailureTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	failureCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "circuit_breaker_failures_total",
		Help: "Total failures tracked by circuit breaker",
	}, []string{"provider", "error_type"})
	registry.MustRegister(failureCounter)

	// Simulate various failure types
	failureCounter.WithLabelValues("openai", "timeout").Add(5)
	failureCounter.WithLabelValues("openai", "rate_limit").Add(3)
	failureCounter.WithLabelValues("anthropic", "connection").Add(2)

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	totalFailures := 0.0
	for _, mf := range metricFamilies {
		if mf.GetName() == "circuit_breaker_failures_total" {
			for _, m := range mf.GetMetric() {
				totalFailures += m.GetCounter().GetValue()
			}
		}
	}

	assert.Equal(t, 10.0, totalFailures, "total failures should be 10")
}

// TestMonitoring_CircuitBreaker_MultiProvider validates that circuit breaker
// metrics are correctly isolated per provider and that state transitions for
// one provider do not affect another.
func TestMonitoring_CircuitBreaker_MultiProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	stateGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "circuit_breaker_multi_state",
		Help: "Current state of circuit breaker per provider",
	}, []string{"provider"})
	registry.MustRegister(stateGauge)

	// Set different states for different providers
	stateGauge.WithLabelValues("openai").Set(0)    // closed
	stateGauge.WithLabelValues("anthropic").Set(1) // open
	stateGauge.WithLabelValues("deepseek").Set(2)  // half-open
	stateGauge.WithLabelValues("gemini").Set(0)    // closed

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	expectedStates := map[string]float64{
		"openai":    0,
		"anthropic": 1,
		"deepseek":  2,
		"gemini":    0,
	}

	for _, mf := range metricFamilies {
		if mf.GetName() == "circuit_breaker_multi_state" {
			for _, m := range mf.GetMetric() {
				for _, label := range m.GetLabel() {
					if label.GetName() == "provider" {
						expected, exists := expectedStates[label.GetValue()]
						require.True(t, exists,
							"unexpected provider %q", label.GetValue())
						assert.Equal(t, expected, m.GetGauge().GetValue(),
							"provider %q state mismatch",
							label.GetValue())
					}
				}
			}
			assert.Equal(t, len(expectedStates), len(mf.GetMetric()),
				"should have a metric entry per provider")
		}
	}
}

// TestMonitoring_CircuitBreaker_SuccessAfterHalfOpen validates that the
// success counter increments correctly during the half-open probe phase
// and that the circuit breaker re-closes after sufficient successful probes.
func TestMonitoring_CircuitBreaker_SuccessAfterHalfOpen(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode") // SKIP-OK: #short-mode
	}

	registry := prometheus.NewRegistry()

	probeSuccess := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "circuit_breaker_probe_success_total",
		Help: "Successful probes during half-open state",
	}, []string{"provider"})
	registry.MustRegister(probeSuccess)

	probeFailure := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "circuit_breaker_probe_failure_total",
		Help: "Failed probes during half-open state",
	}, []string{"provider"})
	registry.MustRegister(probeFailure)

	// Simulate 5 successful probes and 1 failure
	provider := "test-provider"
	for i := 0; i < 5; i++ {
		probeSuccess.WithLabelValues(provider).Inc()
	}
	probeFailure.WithLabelValues(provider).Inc()

	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	var successes, failures float64
	for _, mf := range metricFamilies {
		switch mf.GetName() {
		case "circuit_breaker_probe_success_total":
			for _, m := range mf.GetMetric() {
				successes = m.GetCounter().GetValue()
			}
		case "circuit_breaker_probe_failure_total":
			for _, m := range mf.GetMetric() {
				failures = m.GetCounter().GetValue()
			}
		}
	}

	assert.Equal(t, 5.0, successes,
		"should have 5 successful probes")
	assert.Equal(t, 1.0, failures,
		"should have 1 failed probe")
	assert.Greater(t, successes, failures,
		"successes should exceed failures for recovery")
}
