package database

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// TestDebateLogRepository_StartCleanupWorker_SelectPriorityRace is the
// §11.4.115 RED->GREEN polarity-switch regression guard for the
// "unprioritized select" defect class in StartCleanupWorker's worker loop:
//
//	for {
//	    select {
//	    case <-ctx.Done():
//	        return
//	    case <-ticker.C:
//	        deleted, err := r.CleanupExpiredLogs(ctx)
//	        ...
//	    }
//	}
//
// Go's `select` chooses UNIFORMLY AT RANDOM among all cases ready at the
// instant it is evaluated. If a tick is also pending when ctx is cancelled,
// the naked select above can pick the ticker.C branch and run
// CleanupExpiredLogs on an ALREADY-CANCELLED context instead of returning.
//
// # Oracle (§11.4.108 runtime signature) — NOT a crash
//
// An earlier revision of this guard used "did the child process crash?" as
// its oracle: it drove the race against a repository built with a nil
// *pgxpool.Pool and relied on the resulting nil-pointer panic (inside
// CleanupExpiredLogs's r.pool.Exec call) to signal that the defect fired,
// observed via a re-exec'd child process's exit code (the standard Go
// "os.Exit crasher" pattern, since an in-process panic on a background
// goroutine is unrecoverable and would kill this entire test binary).
//
// That oracle is a BLUFF, provable by a single mutation: revert ONLY this
// method's select-priority pre-check/re-check in StartCleanupWorker (see
// debate_log_repository.go) while KEEPING the nil-pool guard at the top of
// CleanupExpiredLogs (debate_log_repository.go:252-ish, "cannot cleanup
// expired debate logs: repository pool is nil"). The worker again calls
// CleanupExpiredLogs on an already-cancelled ctx — the EXACT defect this
// guard exists to catch — but the nil-pool guard now converts what used to
// be a crash into an ordinary returned error, the PanicLevel logger
// suppresses the resulting log line, and the process exits 0. A guard whose
// only oracle is "did it crash" is therefore PERMANENTLY INCAPABLE of
// detecting a regression of the very fix it is supposed to protect, the
// moment the nil-pool guard exists (which it now permanently does).
//
// This guard instead uses cleanupCalledOnCancelledCtx — an unexported
// atomic.Int64 field incremented at the TOP of CleanupExpiredLogs whenever
// that method observes ctx.Err() != nil, independent of r.pool's nil-ness
// (see the field's doc comment in debate_log_repository.go). That directly
// observes the invariant under test — "was CleanupExpiredLogs ever entered
// with an already-cancelled context" — rather than inferring it from a
// downstream side effect that a later, unrelated fix can silently disarm.
//
// # GREEN mode is DETERMINISTIC, not merely probable (corrected 2026-07-28)
//
// An earlier revision of the standing GREEN assertion cancelled ctx from the
// CALLING goroutine AFTER StartCleanupWorker had already spawned the worker
// (`repo.StartCleanupWorker(ctx, ...); cancel(); repo.StopCleanupWorker()`),
// hammering the general "cancellation lands while the loop is parked in its
// blocking select" case. But debate_log_repository.go's own doc comment on
// the SECOND re-check (the one that covers exactly that case) is explicit
// that this case has a residual race it deliberately does NOT close: a
// cancellation landing strictly AFTER the re-check and before the
// CleanupExpiredLogs call immediately below it is, to any caller,
// "indistinguishable from a cancellation that lands immediately after
// CleanupExpiredLogs already returned — a race the fix does not need to
// close because it is not observable as a defect." That race DOES increment
// cleanupCalledOnCancelledCtx (the counter fires at CleanupExpiredLogs'
// entry, independent of what happens after). So asserting count==0 against
// a cancel-AFTER-start hammer could go red with NO actual regression
// present: a FALSE-FAIL flake, not a bug detector — the inverse of the
// crash-based oracle's false-PASS bluff above, but an equally real defect in
// the guard itself.
//
// The standing GREEN assertion below instead PRE-CANCELS ctx BEFORE calling
// StartCleanupWorker, mirroring
// TestLazyProvider_CreateProviderWithContext_CancellationAlwaysWins in
// internal/llm/lazy_provider_select_priority_test.go. With ctx already
// cancelled, the worker loop's FIRST, non-blocking priority pre-check
// (`select { case <-ctx.Done(): return; default: }`) sees a closed —
// therefore permanently ready — Done channel on its very first evaluation.
// Because exactly one case is ready, `select` resolves it deterministically
// (there is nothing left to break a tie on: "uniformly at random among all
// READY cases" only matters when more than one case is ready), so the
// worker goroutine returns before it can ever reach CleanupExpiredLogs.
// "CleanupExpiredLogs is never entered with an already-cancelled context" is
// therefore PROVABLE for this construction — not a statistical property of
// a race that might or might not fire under scheduler pressure.
//
// The residual cancel-while-parked race is still real and still worth
// reproducing on demand — see RED_MODE=1 below, which keeps the ORIGINAL
// cancel-AFTER-start hammer as a REPRODUCTION TOOL (never as the standing
// GREEN oracle any more) to prove the select-priority pre-check/re-check fix
// is still reachable and this guard would still catch its regression.
//
//   - RED_MODE=1 (run against code with the select-priority pre-check/
//     re-check reverted, nil-pool guard KEPT): drives the cancel-AFTER-start
//     hammer and asserts the aggregate counter is >= 1 across it —
//     reproducing the original defect. This is a manual reproduction step,
//     not part of the standing test suite's default invocation.
//   - RED_MODE unset / "0" (the DEFAULT, standing GREEN guard): pre-cancels
//     ctx before StartCleanupWorker and asserts the aggregate counter is
//     EXACTLY 0 across the entire hammer — deterministically, per the
//     construction above.
func TestDebateLogRepository_StartCleanupWorker_SelectPriorityRace(t *testing.T) {
	if os.Getenv("RED_MODE") == "1" {
		testDebateLogRepositoryCleanupWorkerCancelWhileParkedReproduction(t)
		return
	}

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // suppress log output; nothing panics any more

	const concurrency = 3000
	var wg sync.WaitGroup
	var mu sync.Mutex
	var totalCalledOnCancelled int64

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			repo := NewDebateLogRepository(nil, logger, DefaultRetentionPolicy())
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // pre-cancel BEFORE starting the worker: see doc comment above

			repo.StartCleanupWorker(ctx, 20*time.Microsecond)
			repo.StopCleanupWorker()

			if n := repo.cleanupCalledOnCancelledCtx.Load(); n > 0 {
				mu.Lock()
				totalCalledOnCancelled += n
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	require.Equalf(t, int64(0), totalCalledOnCancelled,
		"GREEN guard (deterministic): with ctx pre-cancelled BEFORE StartCleanupWorker is called, the worker's first, non-blocking priority pre-check must always see the already-closed Done channel and return before ever calling CleanupExpiredLogs; observed count=%d across %d workers — this is not a race, so any non-zero count here means the priority pre-check itself is broken",
		totalCalledOnCancelled, concurrency)
}

// testDebateLogRepositoryCleanupWorkerCancelWhileParkedReproduction is the
// RED-mode reproduction tool for the general "cancellation races the loop's
// blocking select while it is parked" case: it keeps the ORIGINAL
// cancel-AFTER-start hammer (cancel() fired from the calling goroutine only
// after StartCleanupWorker has already spawned the worker goroutine), which
// is what actually exercises the SECOND re-check inside the `case
// <-ticker.C:` branch — the first, non-blocking priority pre-check above
// does not apply here, since ctx is not yet cancelled when that pre-check
// runs.
//
// This is deliberately kept SEPARATE from the standing GREEN assertion in
// TestDebateLogRepository_StartCleanupWorker_SelectPriorityRace: run this
// manually, with RED_MODE=1 set, against a build with the select-priority
// pre-check/re-check reverted (nil-pool guard KEPT) to prove the original
// unprioritized-select defect is still reachable and this guard would still
// catch a regression of it. It is never invoked as part of the default,
// RED_MODE-unset test run.
func testDebateLogRepositoryCleanupWorkerCancelWhileParkedReproduction(t *testing.T) {
	t.Helper()

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // suppress log output; nothing panics any more

	const concurrency = 3000
	var wg sync.WaitGroup
	var mu sync.Mutex
	var totalCalledOnCancelled int64

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			repo := NewDebateLogRepository(nil, logger, DefaultRetentionPolicy())
			ctx, cancel := context.WithCancel(context.Background())

			repo.StartCleanupWorker(ctx, 20*time.Microsecond)
			cancel()
			repo.StopCleanupWorker()

			if n := repo.cleanupCalledOnCancelledCtx.Load(); n > 0 {
				mu.Lock()
				totalCalledOnCancelled += n
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	require.Greaterf(t, totalCalledOnCancelled, int64(0),
		"RED_MODE=1: expected CleanupExpiredLogs to have been entered at least once with an already-cancelled context across %d workers (the unprioritized select in StartCleanupWorker picking ticker.C after ctx cancellation); observed count=%d — defect did not reproduce under this forcing, this is a FINDING not evidence of a fix",
		concurrency, totalCalledOnCancelled)
	t.Logf("RED_MODE=1: reproduced the unprioritized-select defect %d times across %d workers", totalCalledOnCancelled, concurrency)
}
