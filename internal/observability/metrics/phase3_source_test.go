package metrics

// Tests for the default Phase-3 metrics singleton. These exercise the
// nil-safety of the contributor slots, the thread-safety of the
// setters under concurrent access, and the RegisterDefaultPhase3Metrics
// idempotency contract.

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPhase3Source_ZeroWhenUnset(t *testing.T) {
	// Make sure no prior test left contributors installed.
	ClearPhase3Contributors()
	t.Cleanup(ClearPhase3Contributors)

	assert.Equal(t, int64(0), phase3Singleton.EnsemblePendingCount())
	assert.Equal(t, int64(0), phase3Singleton.EnsemblePendingCap())
	assert.Equal(t, uint64(0), phase3Singleton.EnsembleTasksRejected())
	assert.Equal(t, int64(0), phase3Singleton.GuardrailStatsKeyCount())
	assert.Equal(t, int64(0), phase3Singleton.GuardrailStatsDropped())
}

func TestDefaultPhase3Source_ContributorLiveValues(t *testing.T) {
	ClearPhase3Contributors()
	t.Cleanup(ClearPhase3Contributors)

	var (
		pending  atomic.Int64
		cap_     atomic.Int64
		rejected atomic.Uint64
		gKeys    atomic.Int64
		gDropped atomic.Int64
	)
	pending.Store(17)
	cap_.Store(10_000)
	rejected.Store(4)
	gKeys.Store(128)
	gDropped.Store(2)

	SetEnsembleWorkerPoolContributor(WorkerPoolContributor{
		PendingCount:  pending.Load,
		PendingCap:    cap_.Load,
		TasksRejected: rejected.Load,
	})
	SetGuardrailPipelineContributor(GuardrailContributor{
		KeyCount:    gKeys.Load,
		KeysDropped: gDropped.Load,
	})

	assert.Equal(t, int64(17), phase3Singleton.EnsemblePendingCount())
	assert.Equal(t, int64(10_000), phase3Singleton.EnsemblePendingCap())
	assert.Equal(t, uint64(4), phase3Singleton.EnsembleTasksRejected())
	assert.Equal(t, int64(128), phase3Singleton.GuardrailStatsKeyCount())
	assert.Equal(t, int64(2), phase3Singleton.GuardrailStatsDropped())

	// Live mutation — reads must reflect the new value without
	// re-registering.
	pending.Store(99)
	gKeys.Store(200)
	assert.Equal(t, int64(99), phase3Singleton.EnsemblePendingCount())
	assert.Equal(t, int64(200), phase3Singleton.GuardrailStatsKeyCount())
}

func TestDefaultPhase3Source_NilFieldsSafe(t *testing.T) {
	ClearPhase3Contributors()
	t.Cleanup(ClearPhase3Contributors)

	// Partial contributor: only PendingCount installed.
	var pending atomic.Int64
	pending.Store(5)
	SetEnsembleWorkerPoolContributor(WorkerPoolContributor{
		PendingCount: pending.Load,
		// PendingCap and TasksRejected intentionally nil
	})

	assert.Equal(t, int64(5), phase3Singleton.EnsemblePendingCount())
	// Nil accessors must fall back to zero, not panic.
	assert.Equal(t, int64(0), phase3Singleton.EnsemblePendingCap())
	assert.Equal(t, uint64(0), phase3Singleton.EnsembleTasksRejected())
}

func TestDefaultPhase3Source_ConcurrentSetterSafe(t *testing.T) {
	// Race-detector smoke test: hammer the setters from many goroutines
	// while other goroutines read. Any data race would surface under
	// `go test -race`.
	ClearPhase3Contributors()
	t.Cleanup(ClearPhase3Contributors)

	const (
		writers = 8
		readers = 8
		iters   = 500
	)
	var wg sync.WaitGroup

	var counter atomic.Int64
	counter.Store(0)

	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				SetEnsembleWorkerPoolContributor(WorkerPoolContributor{
					PendingCount:  counter.Load,
					PendingCap:    func() int64 { return 10_000 },
					TasksRejected: func() uint64 { return 0 },
				})
				counter.Add(1)
			}
		}()
	}

	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				_ = phase3Singleton.EnsemblePendingCount()
				_ = phase3Singleton.EnsemblePendingCap()
				_ = phase3Singleton.EnsembleTasksRejected()
			}
		}()
	}
	wg.Wait()
}

func TestRegisterDefaultPhase3Metrics_Idempotent(t *testing.T) {
	// Ensure a clean slate since other tests may have registered
	// against the default registry.
	UnregisterDefaultPhase3Metrics()
	t.Cleanup(UnregisterDefaultPhase3Metrics)

	g1, err := RegisterDefaultPhase3Metrics()
	require.NoError(t, err)
	require.NotNil(t, g1)

	// Second call must return the same handle, not try to re-register
	// (which would panic on the underlying Prometheus registry).
	g2, err := RegisterDefaultPhase3Metrics()
	require.NoError(t, err)
	assert.Same(t, g1, g2, "second call must return the existing handle")

	// Unregister then re-register must succeed.
	UnregisterDefaultPhase3Metrics()
	g3, err := RegisterDefaultPhase3Metrics()
	require.NoError(t, err)
	require.NotNil(t, g3)
	// Different handle now since the old one was released.
	assert.NotSame(t, g1, g3)
}
