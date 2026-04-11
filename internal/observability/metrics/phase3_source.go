package metrics

// Default Phase-3 metrics source — a thread-safe singleton that
// decouples the Prometheus collector wiring from the domain packages
// that own the live state (ensemble worker pool and guardrail pipeline).
//
// Direct-import wiring would create a cycle (metrics → ensemble/background
// → metrics) because the ensemble code needs to be observable but must
// not depend on Prometheus. Instead we expose two contributor slots and
// let each owner push its read-only accessors into the singleton at
// construction time. Unregistered slots return zero, so
// RegisterDefaultPhase3Metrics can run once at boot regardless of
// initialization order.
//
// Usage pattern:
//
//   // in router/bootstrap
//   _, _ = metrics.RegisterDefaultPhase3Metrics()
//
//   // later, wherever the ensemble worker pool is created:
//   metrics.SetEnsembleWorkerPoolContributor(metrics.WorkerPoolContributor{
//       PendingCount:    wp.PendingCount,
//       PendingCap:      func() int64 { return background.DefaultMaxPendingResults },
//       TasksRejected:   wp.TasksRejectedCount, // or equivalent getter
//   })
//
//   // and wherever the guardrail pipeline is created:
//   metrics.SetGuardrailPipelineContributor(metrics.GuardrailContributor{
//       KeyCount:    pipeline.StatsKeyCount,
//       KeysDropped: pipeline.StatsKeysDropped,
//   })

import (
	"sync"
)

// WorkerPoolContributor supplies the live gauges for the ensemble
// worker pool. Every field is a zero-arg accessor so the singleton
// never needs to import the worker pool's concrete type.
type WorkerPoolContributor struct {
	PendingCount  func() int64
	PendingCap    func() int64
	TasksRejected func() uint64
}

// GuardrailContributor supplies the live gauges for the guardrail
// pipeline. Same contract as WorkerPoolContributor.
type GuardrailContributor struct {
	KeyCount    func() int64
	KeysDropped func() int64
}

// defaultPhase3Source is the package-level singleton that
// RegisterDefaultPhase3Metrics wires into the global Prometheus registry.
// Read-locked on every scrape.
type defaultPhase3Source struct {
	mu sync.RWMutex

	worker    WorkerPoolContributor
	guardrail GuardrailContributor
}

var phase3Singleton = &defaultPhase3Source{}

// Phase3MetricsSource interface implementation — see phase3_gauges.go.

func (s *defaultPhase3Source) EnsemblePendingCount() int64 {
	s.mu.RLock()
	fn := s.worker.PendingCount
	s.mu.RUnlock()
	if fn == nil {
		return 0
	}
	return fn()
}

func (s *defaultPhase3Source) EnsemblePendingCap() int64 {
	s.mu.RLock()
	fn := s.worker.PendingCap
	s.mu.RUnlock()
	if fn == nil {
		return 0
	}
	return fn()
}

func (s *defaultPhase3Source) EnsembleTasksRejected() uint64 {
	s.mu.RLock()
	fn := s.worker.TasksRejected
	s.mu.RUnlock()
	if fn == nil {
		return 0
	}
	return fn()
}

func (s *defaultPhase3Source) GuardrailStatsKeyCount() int64 {
	s.mu.RLock()
	fn := s.guardrail.KeyCount
	s.mu.RUnlock()
	if fn == nil {
		return 0
	}
	return fn()
}

func (s *defaultPhase3Source) GuardrailStatsDropped() int64 {
	s.mu.RLock()
	fn := s.guardrail.KeysDropped
	s.mu.RUnlock()
	if fn == nil {
		return 0
	}
	return fn()
}

// SetEnsembleWorkerPoolContributor installs (or replaces) the ensemble
// worker pool's accessor bundle. Any nil fields in the contributor
// cause the corresponding gauge to report zero. Safe to call from any
// goroutine at any time; changes take effect on the next Prometheus
// scrape.
func SetEnsembleWorkerPoolContributor(c WorkerPoolContributor) {
	phase3Singleton.mu.Lock()
	defer phase3Singleton.mu.Unlock()
	phase3Singleton.worker = c
}

// SetGuardrailPipelineContributor installs (or replaces) the guardrail
// pipeline's accessor bundle. Same semantics as the worker-pool setter.
func SetGuardrailPipelineContributor(c GuardrailContributor) {
	phase3Singleton.mu.Lock()
	defer phase3Singleton.mu.Unlock()
	phase3Singleton.guardrail = c
}

// ClearPhase3Contributors removes every contributor and resets the
// singleton to its zero state. Primarily for tests — production code
// should rarely need this.
func ClearPhase3Contributors() {
	phase3Singleton.mu.Lock()
	defer phase3Singleton.mu.Unlock()
	phase3Singleton.worker = WorkerPoolContributor{}
	phase3Singleton.guardrail = GuardrailContributor{}
}

// defaultRegistration is the *Phase3Gauges handle returned by
// RegisterDefaultPhase3Metrics. Held in a package variable so a second
// call can detect the already-registered state and return cleanly.
var (
	defaultRegistrationMu sync.Mutex
	defaultRegistration   *Phase3Gauges
)

// RegisterDefaultPhase3Metrics wires the Phase-3 gauges to the global
// Prometheus default registry using the package singleton as their
// source. Safe to call exactly once per process — a second call
// returns the existing handle. The returned handle's Unregister()
// method fully releases the collectors and lets the next call
// re-register (important for tests that reuse the default registry).
func RegisterDefaultPhase3Metrics() (*Phase3Gauges, error) {
	defaultRegistrationMu.Lock()
	defer defaultRegistrationMu.Unlock()

	if defaultRegistration != nil {
		return defaultRegistration, nil
	}

	g, err := RegisterPhase3Metrics(nil, phase3Singleton)
	if err != nil {
		return nil, err
	}
	defaultRegistration = g
	return g, nil
}

// UnregisterDefaultPhase3Metrics releases the collectors registered by
// RegisterDefaultPhase3Metrics. Idempotent. Intended mainly for tests.
func UnregisterDefaultPhase3Metrics() {
	defaultRegistrationMu.Lock()
	defer defaultRegistrationMu.Unlock()

	if defaultRegistration != nil {
		defaultRegistration.Unregister()
		defaultRegistration = nil
	}
}
