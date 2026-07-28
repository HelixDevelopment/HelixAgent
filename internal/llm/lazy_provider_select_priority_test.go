package llm

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLazyProvider_CreateProviderWithContext_CancellationAlwaysWins is the
// §11.4.115 RED->GREEN polarity-switch regression guard for the
// "unprioritized select" defect class: Go's `select` statement chooses
// UNIFORMLY AT RANDOM among all cases that are ready at the moment it is
// evaluated. createProviderWithContext races `<-ctx.Done()` against
// `<-done` (the factory's result channel) in a single naked select with no
// priority ordering. When BOTH channels are already ready at the instant the
// select is polled — which this test forces deterministically by (a)
// pre-cancelling the context BEFORE the call, so ctx.Done() is ready from
// t=0, and (b) using a factory that returns instantly with no I/O or sleep,
// maximising the chance its goroutine has already written to the buffered
// result channel before the select executes — the random pick can return
// the STALE factory result instead of the timeout error, even though the
// caller's context was already cancelled.
//
// This is the deterministic-forced-race reproduction of the flake originally
// observed via `go test -count=20 -run
// TestLazyProvider_Get_AfterTimeoutRetrySucceeds ./internal/llm/` (19 PASS /
// 1 FAIL: "An error is expected but got nil"). That flake required a rare
// ~60ms+ scheduler delay between goroutine spawn and select evaluation to
// manifest (~1/20). Pre-cancelling removes all timing dependency on the
// ctx.Done() side of the race, so across many repetitions the same defect
// reproduces far more reliably than waiting on a scheduler stall by luck.
//
//   - RED_MODE=1 (run against the PRE-FIX code): asserts at least one of many
//     forced-race iterations incorrectly returns success (nil error) despite
//     ctx already being cancelled — reproducing the defect.
//   - RED_MODE unset / "0" (the DEFAULT, standing GREEN guard): asserts EVERY
//     iteration correctly returns the timeout error, because the fix's
//     non-blocking priority pre-check on ctx.Done() makes cancellation win
//     unconditionally, regardless of the underlying channel race.
//
// Attribution note: this test pre-cancels ctx BEFORE calling
// createProviderWithContext, so ctx.Done() is guaranteed ready from the very
// first instruction — that is precisely the scenario the FIRST, non-blocking
// priority pre-check (checked before either select even parks) is
// responsible for, and crediting it here is accurate FOR THIS TEST. It is
// NOT the general case: when cancellation instead races the SECOND, blocking
// select while it is already parked (ctx cancelled AFTER the call begins,
// concurrently with the factory goroutine), the pre-check has already run
// and passed — it is the immediate re-check inside the `case r := <-done:`
// branch that supplies the guarantee there. See
// TestDebateLogRepository_StartCleanupWorker_SelectPriorityRace for the
// sibling guard covering that general (cancel-while-parked) case.
func TestLazyProvider_CreateProviderWithContext_CancellationAlwaysWins(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	const iterations = 20000
	var successDespiteCancelCount int32
	var wg sync.WaitGroup

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// Pre-cancel BEFORE calling createProviderWithContext: ctx.Done()
			// is already closed (ready) from the very first instruction of
			// the function under test, removing all timing dependency on
			// this side of the race.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			// Returns immediately with no I/O/sleep, maximising the chance
			// the spawned goroutine completes and populates the buffered
			// result channel before the select statement is reached and
			// polled. Running thousands of these concurrently (rather than
			// sequentially) creates genuine scheduler contention across the
			// host's CPUs, which is what lets the race manifest without
			// needing to wait for a rare GC-pause-class stall.
			factory := func() (LLMProvider, error) {
				return &lazyMockProvider{name: fmt.Sprintf("instant-%d", i)}, nil
			}

			lazy := NewLazyProvider(fmt.Sprintf("race-%d", i), factory, DefaultLazyProviderConfig())
			prov, err := lazy.createProviderWithContext(ctx)

			if err == nil {
				atomic.AddInt32(&successDespiteCancelCount, 1)
				require.NotNilf(t, prov, "iteration %d: nil error must come with a non-nil provider", i)
			}
		}(i)
	}
	wg.Wait()

	hits := atomic.LoadInt32(&successDespiteCancelCount)

	if redMode {
		require.Greaterf(t, hits, int32(0),
			"RED_MODE=1: expected at least one of %d forced-race iterations to incorrectly return success despite ctx being pre-cancelled (unprioritized select race in createProviderWithContext); got 0 hits — defect did not reproduce under this forcing, this is a FINDING not evidence of a fix",
			iterations)
		t.Logf("RED_MODE=1: reproduced the unprioritized-select defect in %d/%d forced-race iterations", hits, iterations)
	} else {
		require.Equalf(t, int32(0), hits,
			"RED_MODE=0 (GREEN guard): cancellation MUST always win once ctx is already cancelled before createProviderWithContext is called; got %d/%d iterations incorrectly returning success instead of the timeout error",
			hits, iterations)
	}
}
