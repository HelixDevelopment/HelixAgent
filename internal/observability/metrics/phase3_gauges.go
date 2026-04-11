package metrics

// Phase-3 gauges — Prometheus exposure for the memory-safety counters
// introduced in the 2026-04-11 remediation pass.
//
// The Phase-3 code (internal/ensemble/background.WorkerPool and
// internal/security.StandardGuardrailPipeline) deliberately does NOT
// import prometheus/client_golang. Instead, this file defines a small
// source interface and registers GaugeFunc collectors that pull live
// values from an implementation of that interface. This keeps the
// observability layering inverted: metrics depends on domain, not the
// other way around.
//
// Wire this from the boot path by constructing a Phase3MetricsSource
// implementation that composes the live WorkerPool and GuardrailPipeline,
// then calling RegisterPhase3Metrics(registry, source).

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// Phase3MetricsSource is the read-only contract the observability layer
// expects of any Phase-3 sentinel producer. All methods must be
// safe for concurrent use and non-blocking (they are called on every
// Prometheus scrape, typically once every 15 s).
type Phase3MetricsSource interface {
	// EnsemblePendingCount returns the current number of in-flight
	// SubmitAsync tasks in the ensemble worker pool. Zero if the pool
	// was never started.
	EnsemblePendingCount() int64

	// EnsemblePendingCap returns the configured maximum number of
	// in-flight SubmitAsync tasks the pool will accept before rejecting.
	// A fixed-in-practice value — exposed so dashboards can plot
	// utilisation as pending / cap.
	EnsemblePendingCap() int64

	// EnsembleTasksRejected returns the cumulative number of SubmitAsync
	// calls rejected because the pending cap was reached. Counter (monotonic).
	EnsembleTasksRejected() uint64

	// GuardrailStatsKeyCount returns the current number of distinct
	// guardrail names tracked in the pipeline stats map. Capped at
	// MaxGuardrailStatsKeys (1024).
	GuardrailStatsKeyCount() int64

	// GuardrailStatsDropped returns the cumulative number of updateStats
	// calls dropped because the stats-key cap was reached. A non-zero
	// value here is a signal of unexpected guardrail-name churn.
	GuardrailStatsDropped() int64
}

// Phase3Gauges bundles the registered collectors so callers can hold a
// reference for Unregister() during shutdown (important when the same
// registry is reused across test runs — otherwise MustRegister panics
// on a duplicate).
type Phase3Gauges struct {
	ensemblePending    prometheus.GaugeFunc
	ensemblePendingCap prometheus.GaugeFunc
	ensembleRejected   prometheus.CounterFunc
	guardrailKeys      prometheus.GaugeFunc
	guardrailDropped   prometheus.CounterFunc

	registry prometheus.Registerer
}

// RegisterPhase3Metrics wires the Phase-3 gauges to the given Prometheus
// registerer. Passing nil uses the default global registry. Panics if any
// metric is already registered (by name) on the target registry — callers
// should Unregister() an old instance first or use a fresh registry.
func RegisterPhase3Metrics(registry prometheus.Registerer, source Phase3MetricsSource) (*Phase3Gauges, error) {
	if source == nil {
		return nil, fmt.Errorf("phase3 metrics: source must not be nil")
	}
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}

	g := &Phase3Gauges{registry: registry}

	g.ensemblePending = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "helixagent",
			Subsystem: "ensemble",
			Name:      "pending_results",
			Help:      "Current number of in-flight SubmitAsync tasks (per-task delivery channels held by the ensemble worker pool).",
		},
		func() float64 { return float64(source.EnsemblePendingCount()) },
	)

	g.ensemblePendingCap = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "helixagent",
			Subsystem: "ensemble",
			Name:      "pending_results_cap",
			Help:      "Configured maximum number of in-flight SubmitAsync tasks the ensemble worker pool will accept before rejecting.",
		},
		func() float64 { return float64(source.EnsemblePendingCap()) },
	)

	g.ensembleRejected = prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Namespace: "helixagent",
			Subsystem: "ensemble",
			Name:      "tasks_rejected_total",
			Help:      "Cumulative number of SubmitAsync calls rejected because the pending cap was reached.",
		},
		func() float64 { return float64(source.EnsembleTasksRejected()) },
	)

	g.guardrailKeys = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: "helixagent",
			Subsystem: "guardrails",
			Name:      "stats_keys",
			Help:      "Current number of distinct guardrail names tracked in the pipeline stats map.",
		},
		func() float64 { return float64(source.GuardrailStatsKeyCount()) },
	)

	g.guardrailDropped = prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Namespace: "helixagent",
			Subsystem: "guardrails",
			Name:      "stats_dropped_total",
			Help:      "Cumulative number of updateStats calls dropped because the stats-key cap was reached.",
		},
		func() float64 { return float64(source.GuardrailStatsDropped()) },
	)

	for _, c := range []prometheus.Collector{
		g.ensemblePending,
		g.ensemblePendingCap,
		g.ensembleRejected,
		g.guardrailKeys,
		g.guardrailDropped,
	} {
		if err := registry.Register(c); err != nil {
			// Roll back anything we already registered so the caller
			// can retry cleanly.
			g.Unregister()
			return nil, fmt.Errorf("phase3 metrics: register: %w", err)
		}
	}

	return g, nil
}

// Unregister removes every Phase-3 collector from the registry it was
// registered against. Safe to call multiple times.
func (g *Phase3Gauges) Unregister() {
	if g == nil || g.registry == nil {
		return
	}
	for _, c := range []prometheus.Collector{
		g.ensemblePending,
		g.ensemblePendingCap,
		g.ensembleRejected,
		g.guardrailKeys,
		g.guardrailDropped,
	} {
		if c != nil {
			g.registry.Unregister(c)
		}
	}
}
