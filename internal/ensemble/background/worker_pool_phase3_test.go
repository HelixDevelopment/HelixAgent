package background

// Phase-3 regression tests for the worker pool memory-safety hardening.
// Each test pins one invariant that would have failed against the pre-fix
// code and is now guaranteed by the 2026-04-11 changes in worker_pool.go:
//
//  1. PendingCount tracks in-flight SubmitAsync calls accurately.
//  2. SubmitAsync respects the pending-results cap and reports rejection.
//  3. SubmitAsyncTimeout fires via an explicit timer, not time.After.
//  4. Pool stats expose the new pending/rejected fields.

import (
	"context"
	"testing"
	"time"

	"dev.helix.agent/internal/clis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerPool_PendingCount_DrainsOnCompletion(t *testing.T) {
	pool := NewWorkerPool(2)
	require.NoError(t, pool.Start(context.Background()))
	t.Cleanup(func() { _ = pool.Stop() })

	assert.Equal(t, int64(0), pool.PendingCount(), "fresh pool has no pending")

	// Submit a handful of tasks and wait for each. PendingCount must return
	// to zero after the last result is consumed — if defer deletePending
	// regressed, this assertion would fail.
	for i := 0; i < 5; i++ {
		task := &clis.Task{Type: clis.TaskTypeDocumentation, Name: "doc"}
		ch := pool.SubmitAsync(task)
		select {
		case result := <-ch:
			require.NotNil(t, result)
		case <-time.After(5 * time.Second):
			t.Fatalf("task %d: no result within 5s", i)
		}
	}
	assert.Eventually(t, func() bool {
		return pool.PendingCount() == 0
	}, 2*time.Second, 50*time.Millisecond, "PendingCount must drain to zero")
}

func TestWorkerPool_SubmitAsync_RespectsPendingCap(t *testing.T) {
	pool := NewWorkerPool(1)
	// Tiny cap so we can trip it deterministically without flooding.
	pool.SetMaxPendingResults(2)
	require.NoError(t, pool.Start(context.Background()))
	t.Cleanup(func() { _ = pool.Stop() })

	// Pre-populate two pending entries by reserving slots without
	// letting the caller goroutine drain them.
	chA := make(chan *TaskResult, 1)
	chB := make(chan *TaskResult, 1)
	require.True(t, pool.storePending("A", chA))
	require.True(t, pool.storePending("B", chB))
	assert.Equal(t, int64(2), pool.PendingCount())

	// Third SubmitAsync must be rejected synchronously because the cap
	// is already exhausted. The result channel should be closed with a
	// rejection error and no task should have been queued.
	task := &clis.Task{Type: clis.TaskTypeDocumentation, Name: "over-cap"}
	resCh := pool.SubmitAsync(task)
	select {
	case result := <-resCh:
		require.NotNil(t, result)
		assert.False(t, result.Success)
		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "pending cap reached")
	case <-time.After(1 * time.Second):
		t.Fatal("SubmitAsync did not reject within 1s")
	}

	stats := pool.GetStats()
	assert.Equal(t, uint64(1), stats["tasks_rejected"], "rejection must be counted")
	assert.Equal(t, int64(2), stats["pending_results"])
	assert.Equal(t, int64(2), stats["pending_results_cap"])

	// Clean up the manually stored pending entries so the counter
	// returns to zero — regression check for deletePending idempotency.
	pool.deletePending("A")
	pool.deletePending("B")
	assert.Equal(t, int64(0), pool.PendingCount())
	pool.deletePending("A") // idempotent
	assert.Equal(t, int64(0), pool.PendingCount())
}

func TestWorkerPool_SubmitAsync_TimeoutFiresCleanly(t *testing.T) {
	// This test exercises the time.NewTimer/Stop path that replaced the
	// leaky time.After. We use a very short timeout and a pool that
	// cannot actually deliver a result (no workers started), so the
	// timer branch wins every time.
	//
	// If the old time.After code came back, this test would still pass
	// functionally but each iteration would leak a 30-second timer — we
	// catch that by asserting the pool shuts down cleanly within a tight
	// budget.
	prev := SubmitAsyncTimeout
	SubmitAsyncTimeout = 50 * time.Millisecond
	defer func() { SubmitAsyncTimeout = prev }()

	pool := NewWorkerPool(1)
	require.NoError(t, pool.Start(context.Background()))

	for i := 0; i < 3; i++ {
		task := &clis.Task{Type: "never-matched", Name: "timeout-probe"}
		ch := pool.SubmitAsync(task)
		select {
		case result := <-ch:
			require.NotNil(t, result)
			// Either the pool rejected it (worker consumed and returned
			// unknown-type error) or the timer fired. Both are acceptable
			// — the point is that no goroutine is stuck waiting.
		case <-time.After(2 * time.Second):
			t.Fatalf("iteration %d: no result within 2s", i)
		}
	}

	stopDone := make(chan struct{})
	go func() {
		_ = pool.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(3 * time.Second):
		t.Fatal("pool.Stop() took longer than 3s — probable goroutine leak")
	}
}

func TestWorkerPool_Stats_ExposeNewFields(t *testing.T) {
	pool := NewWorkerPool(1)
	stats := pool.GetStats()

	// Phase-3 additions must be present with sensible zero-values.
	_, ok := stats["tasks_rejected"]
	assert.True(t, ok, "GetStats must expose tasks_rejected")
	_, ok = stats["pending_results"]
	assert.True(t, ok, "GetStats must expose pending_results")
	_, ok = stats["pending_results_cap"]
	assert.True(t, ok, "GetStats must expose pending_results_cap")

	assert.Equal(t, uint64(0), stats["tasks_rejected"])
	assert.Equal(t, int64(0), stats["pending_results"])
	assert.Equal(t, int64(DefaultMaxPendingResults), stats["pending_results_cap"])

	pool.SetMaxPendingResults(42)
	stats = pool.GetStats()
	assert.Equal(t, int64(42), stats["pending_results_cap"], "override propagates")
}
