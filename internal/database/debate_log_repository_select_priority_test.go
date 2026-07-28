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
// Because nothing crashes any more, the race is driven in-process (no
// child-process re-exec needed): many goroutines each build their own
// nil-pool repository, start the worker with a very short tick interval,
// and cancel it almost immediately, creating genuine scheduler contention
// across the host's CPUs so the rare "tick pending at the instant of
// cancellation" scheduling stall manifests reliably instead of by luck.
//
//   - RED_MODE=1 (run against code with the select-priority pre-check/
//     re-check reverted, nil-pool guard KEPT): asserts the aggregate counter
//     is >= 1 across the hammer — reproducing the defect.
//   - RED_MODE unset / "0" (the DEFAULT, standing GREEN guard): asserts the
//     aggregate counter is EXACTLY 0 across the entire hammer. In the
//     general (non-pre-cancelled) case exercised here — cancel() is called
//     AFTER StartCleanupWorker has already started the goroutine, so the
//     race is "does cancellation land while the loop is parked in its
//     blocking select" — it is the SECOND, immediate re-check inside the
//     `case <-ticker.C:` branch (not the first priority pre-check, which
//     only helps when ctx is already cancelled BEFORE a loop iteration
//     begins) that supplies this guarantee. See the re-check's doc comment
//     in debate_log_repository.go for the precise, honest scope of what it
//     delivers.
func TestDebateLogRepository_StartCleanupWorker_SelectPriorityRace(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

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

	if redMode {
		require.Greaterf(t, totalCalledOnCancelled, int64(0),
			"RED_MODE=1: expected CleanupExpiredLogs to have been entered at least once with an already-cancelled context across %d workers (the unprioritized select in StartCleanupWorker picking ticker.C after ctx cancellation); observed count=%d — defect did not reproduce under this forcing, this is a FINDING not evidence of a fix",
			concurrency, totalCalledOnCancelled)
		t.Logf("RED_MODE=1: reproduced the unprioritized-select defect %d times across %d workers", totalCalledOnCancelled, concurrency)
	} else {
		require.Equalf(t, int64(0), totalCalledOnCancelled,
			"RED_MODE=0 (GREEN guard): CleanupExpiredLogs must NEVER be entered with an already-cancelled context (ctx.Done() must always win once cancelled, so CleanupExpiredLogs is never called again after cancellation); observed count=%d across %d workers",
			totalCalledOnCancelled, concurrency)
	}
}
