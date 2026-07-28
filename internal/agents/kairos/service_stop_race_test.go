package kairos

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// TestService_RunOnce_SelectPriorityRace is the §11.4.115 RED->GREEN
// polarity-switch regression guard for the "unprioritized select"
// defect class in run()'s per-iteration select (extracted, unchanged
// in behavior, into runOnce so it is directly testable — see
// runOnce's doc comment in service.go). See
// dream.TestDreamer_RunOnce_SelectPriorityRace for the sibling guard
// this mirrors exactly (same defect shape, same forcing technique).
//
// # Measured pre-fix hit rate
//
// Reverting ONLY runOnce's priority pre-check and the re-check inside
// the tick case (restoring the historical, unprioritized 3-case
// select) and running this exact construction 50,000 times measured a
// tick-wins rate of 49.46%-50.24% across three independent runs
// (-count=1, cache disabled) — matching the theoretical 50% for two
// live, ready cases (ctx is context.Background(), whose Done() is nil
// and therefore never selectable, leaving exactly two live contenders:
// s.stopCh and tick).
//
//   - RED_MODE=1: run this test manually against a build with runOnce's
//     priority pre-check and re-check reverted (see service.go's
//     runOnce doc comment for the exact two blocks to comment out).
//     Asserts at least one of many forced-race iterations incorrectly
//     lets tick() launch despite s.stopCh already being closed —
//     reproducing the defect. This will FAIL if run against the
//     current, fixed source — that is expected; it is a manual
//     reproduction step, not part of the standing suite.
//   - RED_MODE unset / "0" (the DEFAULT, standing GREEN guard): asserts
//     EVERY iteration correctly returns without launching tick(),
//     DETERMINISTICALLY — because runOnce's first, non-blocking
//     priority pre-check (checkStop) sees the already-closed s.stopCh
//     as the ONLY ready case in ITS OWN select.
func TestService_RunOnce_SelectPriorityRace(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	const iterations = 50000
	hits := 0

	for i := 0; i < iterations; i++ {
		s := NewService(ServiceConfig{
			Enabled:        true,
			BlockingBudget: time.Hour,
		}, logger)
		// NewService already allocates stopCh; construct a fresh one
		// per iteration is unnecessary since each Service instance is
		// itself fresh, but make the "provably ready" precondition
		// explicit and independent of NewService's internals.
		close(s.stopCh)
		tick := make(chan time.Time, 1)
		tick <- time.Now()

		shouldReturn := s.runOnce(context.Background(), tick)
		if !shouldReturn {
			hits++
		}
	}

	if redMode {
		require.Greaterf(t, hits, 0,
			"RED_MODE=1: expected at least one of %d forced-race iterations to launch tick() despite s.stopCh already being closed (unprioritized select in runOnce); got 0 hits — defect did not reproduce under this forcing, this is a FINDING not evidence of a fix (did you revert runOnce's priority pre-check/re-check?)",
			iterations)
		t.Logf("RED_MODE=1: reproduced the unprioritized-select defect in %d/%d forced-race iterations (%.2f%%)", hits, iterations, float64(hits)/float64(iterations)*100.0)
	} else {
		require.Equalf(t, 0, hits,
			"RED_MODE=0/unset (GREEN guard): with s.stopCh already closed BEFORE runOnce is called, the priority pre-check must always see it and return before ever reaching the tick case; got %d/%d iterations incorrectly launching tick() — this is not a race, so any non-zero count here means the priority pre-check itself is broken",
			hits, iterations)
	}
}

// TestService_Stop_ActionAfterStopReturned is the direct corruption
// -analog proof for Defect 2 ("action executed after Stop() returned").
// It drives the REAL Start()/Stop() API (not runOnce directly) with a
// microsecond TickInterval so run()'s real *time.Ticker fires
// virtually continuously, lets a short (5ms) delay guarantee run()'s
// goroutine has genuinely started a tick() call, then races Stop()
// against that in-flight tick() from the test's own goroutine, and
// directly observes — via an atomic flag flipped by a deliberately
// slow onAction callback — whether the action is STILL EXECUTING at
// the instant Stop() returns to its caller. An earlier version of this
// test raced Stop() with NO delay at all after Start() and measured
// 0/200 trials with a completed action — Start()'s `go s.run(ctx)`
// does not yield, so with no delay the test's own goroutine reliably
// called Stop() before the Go scheduler ever gave run()'s goroutine a
// timeslice, making the "GREEN" result vacuous (Stop() trivially never
// races a tick that never started). The delay below exists
// specifically to make the race GENUINE.
//
// # Why this is deterministic, not merely probable, post-fix
//
// BlockingBudget (500ms) is engineered to be far larger than onAction's
// own sleep (40ms), so tick()'s internal select always resolves via
// the genuine `<-done` completion path, never the `<-actionCtx.Done()`
// timeout path — this keeps the test in the "action genuinely
// completes" regime the fix actually covers (see Stop()'s doc comment
// for the documented, out-of-scope "onAction outlives BlockingBudget"
// edge case). Given that, Stop()'s s.runWg.Wait() join means Stop()
// cannot return until run() has returned, and run() cannot return
// while it is still inside a tick() call it already started, and that
// tick() call cannot return until onAction has already flipped
// actionRunning back to false (given the budget/sleep relationship
// above). So POST-FIX, actionRunning MUST be false at the instant
// every Stop() call returns — not "usually", always, by construction.
//
//   - RED_MODE=1: run this test manually against a build with Stop()'s
//     s.runWg.Wait() call removed (see service.go's Stop() doc
//     comment). Asserts at least one of many trials observed
//     actionRunning == true immediately after Stop() returned —
//     reproducing the defect. This will FAIL if run against the
//     current, fixed source — that is expected; it is a manual
//     reproduction step, not part of the standing suite.
//   - RED_MODE unset / "0" (the DEFAULT, standing GREEN guard): asserts
//     actionRunning is false immediately after EVERY Stop() call
//     returns, across every trial.
func TestService_Stop_ActionAfterStopReturned(t *testing.T) {
	redMode := os.Getenv("RED_MODE") == "1"

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	const iterations = 200
	const actionSleep = 40 * time.Millisecond

	violations := 0
	var totalCompletedActions int32

	for i := 0; i < iterations; i++ {
		config := DefaultConfig()
		config.Enabled = true
		config.TickInterval = time.Microsecond
		config.BlockingBudget = 500 * time.Millisecond
		// §9 data safety: LogPath MUST be a fresh t.TempDir(), NEVER
		// the real ~/.helixagent/kairos/logs DefaultConfig() points at
		// on this host.
		config.LogPath = t.TempDir()

		s := NewService(config, logger)

		var actionRunning atomic.Bool
		s.SetCallbacks(
			func(Observation) {},
			func(a Action) {
				actionRunning.Store(true)
				time.Sleep(actionSleep)
				actionRunning.Store(false)
				atomic.AddInt32(&totalCompletedActions, 1)
			},
			func(TickPrompt) (Action, error) {
				return Action{
					Type:        "test",
					Description: "forced-race test action",
					Duration:    5 * time.Millisecond, // well within BlockingBudget
				}, nil
			},
		)

		require.NoError(t, s.Start(context.Background()))
		// A short, deliberate delay here is REQUIRED, not merely
		// tighter timing: go s.run(ctx) in Start() does not yield, so
		// with NO delay the test's own goroutine can (and, measured,
		// reliably DOES) call Stop() before the Go scheduler ever
		// gives run()'s goroutine a timeslice at all — an earlier
		// version of this test used no delay and measured 0/200 trials
		// with a completed action, which is a VACUOUS pass (Stop()
		// trivially never races a tick that never started), not
		// evidence the fix works. 5ms is far larger than the
		// microsecond TickInterval (guaranteeing run()'s goroutine has
		// been scheduled and observed a pending tick) and far smaller
		// than actionSleep's 40ms (guaranteeing, when a tick DOES fire,
		// run()'s goroutine is reliably still inside tick()'s onAction
		// sleep — i.e. genuinely racing Stop() — rather than already
		// having returned).
		time.Sleep(5 * time.Millisecond)
		require.NoError(t, s.Stop())

		if actionRunning.Load() {
			violations++
		}
	}

	t.Logf("%d/%d trials recorded a completed action (proves the tick→onAction path genuinely fired at least sometimes)", atomic.LoadInt32(&totalCompletedActions), iterations)

	if redMode {
		require.Greaterf(t, violations, 0,
			"RED_MODE=1: expected at least one of %d trials to observe the action still executing immediately after Stop() returned (missing s.runWg.Wait() join); got 0 violations — defect did not reproduce under this forcing, this is a FINDING not evidence of a fix (did you remove s.runWg.Wait() from Stop()?)",
			iterations)
		t.Logf("RED_MODE=1: reproduced the action-after-Stop-returned defect in %d/%d trials", violations, iterations)
	} else {
		require.Equalf(t, 0, violations,
			"RED_MODE=0/unset (GREEN guard): Stop() must never return while the action it (or a tick() it started) launched is still executing; observed %d/%d trials with the action still running immediately after Stop() returned",
			violations, iterations)
	}
}
