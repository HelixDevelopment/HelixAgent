package background

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestWorkerPool_WaitForAsyncResult_DeliveredResultAlwaysWinsOverShutdown is
// the §11.4.115 RED->GREEN polarity-switch regression guard for the
// "unprioritized select, inverted" defect class in
// (*WorkerPool).waitForAsyncResult — the SubmitAsync wait select:
//
//	select {
//	case result := <-deliveryCh:
//	    resultCh <- result
//	case <-wp.ctx.Done():
//	    resultCh <- &TaskResult{..., Error: fmt.Errorf("worker pool stopped")}
//	case <-timer.C:
//	    ...
//	}
//
// deliveryCh is buffered(1). Once wp.ctx is cancelled (pool Stop()),
// wp.ctx.Done() stays PERMANENTLY ready for the rest of the process's
// lifetime. Go's select statement chooses UNIFORMLY AT RANDOM among all
// cases ready at the instant it is evaluated, so if a genuine,
// already-computed successful result is sitting buffered in deliveryCh at
// the same moment wp.ctx is already cancelled, the naked select above has
// ~50% odds of discarding the real result in favour of a fabricated
// "worker pool stopped" error — the INVERSE of the defect class fixed this
// session in lazy_provider.go / debate_log_repository.go (those give
// priority to cancellation winning over a stale/late result; here a real,
// already-delivered result must win over a fabricated shutdown error).
//
// This test forces the race deterministically exactly as
// TestLazyProvider_CreateProviderWithContext_CancellationAlwaysWins does:
// (a) pre-cancelling wp.ctx BEFORE calling waitForAsyncResult, so
// wp.ctx.Done() is ready from t=0, and (b) pre-buffering the real,
// already-computed result into deliveryCh BEFORE the call, so deliveryCh is
// ALSO ready from t=0 — both cases are provably ready before the select is
// ever evaluated, so there is zero scheduler-timing dependency left: only
// Go's uniform random pick decides the outcome.
//
//   - RED_MODE=1 (run against the PRE-FIX naked/unprioritized select):
//     asserts a substantial fraction (~50%, matching the theoretical odds of
//     a uniform pick between two ready cases) of iterations incorrectly
//     discard the buffered real result in favour of the fabricated shutdown
//     error.
//   - RED_MODE unset / "0" (the DEFAULT, standing GREEN guard): asserts
//     EVERY iteration returns the real, already-buffered result, because the
//     fix's non-blocking priority pre-check on deliveryCh (and the re-check
//     inside the ctx.Done() branch) makes an already-delivered result win
//     unconditionally over the fabricated error.
//
// HONESTY (§11.4.107 / no-bluff, matching the sibling guards' own
// disclosure): this does NOT claim the race is fully "closed" in the
// general case where deliverResult() sends into deliveryCh WHILE the select
// is already parked (rather than before it is entered) — under Go's async
// goroutine preemption (>=1.14) no select-based implementation can make
// that window provably zero-width. What this test proves, and what the fix
// genuinely guarantees, is narrower and still load-bearing: a result
// ALREADY buffered in deliveryCh at the moment the decision point is
// evaluated always wins over a fabricated "worker pool stopped" error.
func TestWorkerPool_WaitForAsyncResult_DeliveredResultAlwaysWinsOverShutdown(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	const iterations = 20000
	var lostRealResultCount int32
	var wg sync.WaitGroup

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			wp := NewWorkerPool(1)
			// Pre-cancel BEFORE calling waitForAsyncResult: wp.ctx.Done()
			// is already closed (ready) from the very first instruction of
			// the function under test, removing all timing dependency on
			// this side of the race.
			wp.cancel()

			taskID := fmt.Sprintf("race-%d", i)
			deliveryCh := make(chan *TaskResult, 1)
			// Pre-buffer the real, already-computed result BEFORE the
			// call, so deliveryCh is ALSO ready from t=0.
			deliveryCh <- &TaskResult{
				TaskID:  taskID,
				Success: true,
				Result:  "real-result",
			}

			timer := time.NewTimer(SubmitAsyncTimeout)
			defer timer.Stop()

			result := wp.waitForAsyncResult(taskID, deliveryCh, timer)

			if !result.Success {
				atomic.AddInt32(&lostRealResultCount, 1)
			}
		}(i)
	}
	wg.Wait()

	hits := atomic.LoadInt32(&lostRealResultCount)

	if redMode {
		require.Greaterf(t, hits, int32(0),
			"RED_MODE=1: expected a substantial fraction of %d forced-race iterations to incorrectly "+
				"discard the already-buffered real result in favour of the fabricated \"worker pool "+
				"stopped\" error (unprioritized select in waitForAsyncResult); got 0 hits -- defect did "+
				"not reproduce under this forcing, this is a FINDING not evidence of a fix",
			iterations)
		ratio := float64(hits) / float64(iterations)
		t.Logf("RED_MODE=1: reproduced the unprioritized-select (inverted) defect in %d/%d (%.2f%%) forced-race iterations",
			hits, iterations, ratio*100)
	} else {
		require.Equalf(t, int32(0), hits,
			"RED_MODE=0 (GREEN guard): an already-buffered real result MUST always win over a "+
				"fabricated \"worker pool stopped\" shutdown error once wp.ctx is cancelled; got %d/%d "+
				"iterations incorrectly returning the fabricated error instead of the real, "+
				"already-delivered result",
			hits, iterations)
	}
}
