package database

import (
	"context"
	"os"
	"os/exec"
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
// Against a repository built with a nil *pgxpool.Pool (as this guard uses,
// mirroring TestDebateLogRepository_StartCleanupWorker_ContextCancel), that
// stray call nil-pointer-panics — and because it happens in the worker's own
// background goroutine (not the test goroutine), the panic is unrecoverable
// and CRASHES THE WHOLE TEST PROCESS, not just this one test.
//
// To force the race deterministically without risking a crash of this (or
// any sibling) test binary, the crash-prone scenario is driven in an
// isolated CHILD PROCESS — the standard Go "os.Exit crasher" test pattern
// (re-invoking os.Args[0] with a narrowed -test.run and a sentinel env var).
// The child runs many concurrent StartCleanupWorker instances with a very
// short tick interval and cancels each immediately, which creates genuine
// scheduler contention across the host's CPUs — this is what lets the race
// manifest reliably instead of waiting on a rare scheduling stall by luck.
// A single occurrence anywhere in the child crashes the whole child process,
// which is the signal this (parent) test observes via the exit code.
//
//   - RED_MODE=1 (run against the PRE-FIX code): asserts the child process
//     CRASHES — reproducing the defect.
//   - RED_MODE unset / "0" (the DEFAULT, standing GREEN guard): asserts the
//     child process ALWAYS exits cleanly, because the fix's non-blocking
//     priority pre-check on ctx.Done() makes cancellation win unconditionally
//     before the worker loop ever looks at ticker.C again.
func TestDebateLogRepository_StartCleanupWorker_SelectPriorityRace(t *testing.T) {
	if os.Getenv("HELIX_CLEANUP_RACE_HELPER") == "1" {
		runCleanupWorkerRaceHelper()
		return
	}

	redMode := os.Getenv("RED_MODE") == "1"

	cmd := exec.Command(os.Args[0],
		"-test.run=^TestDebateLogRepository_StartCleanupWorker_SelectPriorityRace$",
		"-test.v")
	cmd.Env = append(os.Environ(), "HELIX_CLEANUP_RACE_HELPER=1")
	out, err := cmd.CombinedOutput()

	output := string(out)
	if len(output) > 4000 {
		output = output[:4000] + "...(truncated)"
	}
	t.Logf("race-helper child process exited with err=%v, output:\n%s", err, output)

	crashed := err != nil

	if redMode {
		require.Truef(t, crashed,
			"RED_MODE=1: expected the race-helper child process to crash (nil-pointer panic from the ticker.C branch winning after ctx cancellation) on the pre-fix unprioritized select in StartCleanupWorker; it exited cleanly instead (err=%v)", err)
	} else {
		require.Falsef(t, crashed,
			"RED_MODE=0 (GREEN guard): race-helper child process must ALWAYS exit cleanly (ctx.Done() must always win once cancelled, so CleanupExpiredLogs is never called on an already-cancelled context); got err=%v, output:\n%s",
			err, output)
	}
}

// runCleanupWorkerRaceHelper repeatedly starts cleanup workers, each against
// its own nil-pool repository with a very short tick interval, and cancels
// each almost immediately — all CONCURRENTLY, so the host's scheduler is
// under genuine contention. This maximises the chance that, for at least one
// worker, a tick is also pending at the exact moment its select re-evaluates
// after cancellation, forcing the random pick to sometimes choose ticker.C
// over ctx.Done(). A single such occurrence nil-pointer-panics this (child)
// process when it calls CleanupExpiredLogs against the nil pool, which the
// parent test observes as a non-zero exit code / crash signature in output.
func runCleanupWorkerRaceHelper() {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // suppress log output, still lets panics surface

	const concurrency = 3000
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			repo := NewDebateLogRepository(nil, logger, DefaultRetentionPolicy())
			ctx, cancel := context.WithCancel(context.Background())

			repo.StartCleanupWorker(ctx, 20*time.Microsecond)
			cancel()
			repo.StopCleanupWorker()
		}()
	}
	wg.Wait()
}
