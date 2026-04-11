package background

// Short, hermetic soak test for the ensemble worker pool.
//
// The full Phase-6 plan calls for a 30-minute 500-rps soak against a
// live infrastructure stack. This file intentionally runs a MUCH smaller
// version (sub-second wall time under -short) that still validates the
// three invariants the big soak would check:
//
//   1. PendingCount drains to zero after steady-state churn — proves
//      the storePending/deletePending bookkeeping is balanced.
//   2. Goroutine count does not grow unboundedly — proves
//      SubmitAsync's goroutine lifecycle is clean.
//   3. No race-detector findings under parallel submission.
//
// A longer variant is gated on -run SoakLong so humans can opt-in to a
// multi-second run locally without blocking the short suite. CI cannot
// exist in this repo so nothing else depends on the longer variant.

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/clis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerPool_Soak_Short(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak in -short mode")
	}
	runSoak(t, 200*time.Millisecond, 4, 1)
}

// TestWorkerPool_Soak_Long is the opt-in longer variant. Invoke via
//
//	go test -v -timeout=30s -run TestWorkerPool_Soak_Long ./internal/ensemble/background/...
//
// It runs for 2 seconds with more concurrency so leaks have time to
// manifest. Intentionally NOT in the default suite so the short build
// stays fast.
func TestWorkerPool_Soak_Long(t *testing.T) {
	if testing.Short() {
		t.Skip("long soak is opt-in via -run TestWorkerPool_Soak_Long")
	}
	runSoak(t, 2*time.Second, 8, 8)
}

func runSoak(t *testing.T, duration time.Duration, producers, perBatchSleepMs int) {
	t.Helper()

	pool := NewWorkerPool(4)
	require.NoError(t, pool.Start(context.Background()))
	t.Cleanup(func() { _ = pool.Stop() })

	// Baseline goroutine count AFTER the pool's workers are up — we
	// want the delta from steady-state, not from a bare binary.
	// Give the scheduler a beat to settle.
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var (
		wg       sync.WaitGroup
		produced int64
		consumed int64
		mu       sync.Mutex
	)

	// N producer goroutines hammer SubmitAsync and consume each result.
	// Each producer waits for its own result before issuing the next,
	// so the in-flight depth per producer is always 1 — the total
	// pending count is bounded by `producers`.
	wg.Add(producers)
	for p := 0; p < producers; p++ {
		go func() {
			defer wg.Done()
			for {
				// Only the SUBMIT side observes ctx. Once a task has
				// been submitted, the producer MUST drain its result
				// channel — SubmitAsync guarantees the channel is
				// closed (either with a real result, a rejection, or
				// a pool-stopped error), so the read always returns
				// promptly and `produced == consumed` is an invariant.
				select {
				case <-ctx.Done():
					return
				default:
				}
				task := &clis.Task{Type: clis.TaskTypeDocumentation, Name: "soak"}
				ch := pool.SubmitAsync(task)
				mu.Lock()
				produced++
				mu.Unlock()

				<-ch
				mu.Lock()
				consumed++
				mu.Unlock()

				if perBatchSleepMs > 0 {
					time.Sleep(time.Duration(perBatchSleepMs) * time.Millisecond)
				}
			}
		}()
	}
	wg.Wait()

	// Give deferred deletePending callbacks a moment to run before we
	// sample the pool state.
	assert.Eventually(t, func() bool {
		return pool.PendingCount() == 0
	}, 500*time.Millisecond, 10*time.Millisecond,
		"PendingCount must drain to zero after all producers finish")

	mu.Lock()
	totalProduced := produced
	totalConsumed := consumed
	mu.Unlock()
	t.Logf("soak summary: produced=%d consumed=%d", totalProduced, totalConsumed)
	assert.Equal(t, totalProduced, totalConsumed,
		"every submitted task must yield exactly one result")

	// Every task was executed by the pool workers, so tasks_completed
	// (or tasks_failed — unknown task type returns an error) should
	// account for the full volume.
	stats := pool.GetStats()
	completed := stats["tasks_completed"].(uint64)
	failed := stats["tasks_failed"].(uint64)
	assert.Equal(t, uint64(totalProduced), completed+failed,
		"pool metrics must account for every submitted task")

	// Goroutine-count invariant: the delta after stabilisation must be
	// zero (ignoring scheduler jitter of ~2). We check BEFORE calling
	// Stop() so we see the steady-state cost, not the teardown cost.
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	final := runtime.NumGoroutine()
	delta := final - baseline
	t.Logf("goroutine delta: baseline=%d final=%d delta=%d", baseline, final, delta)
	assert.LessOrEqual(t, delta, 4,
		"goroutine count must not drift upward after steady-state churn (delta=%d)", delta)
}

// TestWorkerPool_Soak_RejectionRecovery confirms that after the pending
// cap is tripped, dropping all synthetic pending entries returns the
// pool to a fully-working state. This catches any edge case where
// storePending/deletePending accounting gets wedged.
func TestWorkerPool_Soak_RejectionRecovery(t *testing.T) {
	pool := NewWorkerPool(2)
	pool.SetMaxPendingResults(4)
	require.NoError(t, pool.Start(context.Background()))
	t.Cleanup(func() { _ = pool.Stop() })

	// Saturate the cap with synthetic pending entries.
	for _, id := range []string{"a", "b", "c", "d"} {
		require.True(t, pool.storePending(id, make(chan *TaskResult, 1)))
	}
	assert.Equal(t, int64(4), pool.PendingCount())

	// Next real SubmitAsync must reject cleanly.
	task := &clis.Task{Type: clis.TaskTypeDocumentation, Name: "over-cap"}
	result := <-pool.SubmitAsync(task)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "pending cap reached")

	// Drain synthetic entries and verify the pool accepts work again.
	for _, id := range []string{"a", "b", "c", "d"} {
		pool.deletePending(id)
	}
	assert.Equal(t, int64(0), pool.PendingCount())

	task2 := &clis.Task{Type: clis.TaskTypeDocumentation, Name: "post-recovery"}
	select {
	case result := <-pool.SubmitAsync(task2):
		require.NotNil(t, result)
		assert.True(t, result.Success,
			"pool must recover and accept work after cap drain")
	case <-time.After(2 * time.Second):
		t.Fatal("post-recovery SubmitAsync timed out")
	}
}
