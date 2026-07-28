package kairos

import (
	"context"
	"os"
	"sync"
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
// # Why this is deterministic GIVEN the engineered margin holds, post-fix
//
// M6 honesty note (independent-review round): the heading and
// reasoning below describe a MARGIN-bound determinism, not an
// unconditional one — say so precisely rather than overclaiming.
// BlockingBudget (500ms) is engineered to be far larger than onAction's
// own sleep (40ms — a 12.5x margin), so tick()'s internal select always
// resolves via the genuine `<-done` completion path, never the
// `<-actionCtx.Done()` timeout path, PROVIDED the host scheduling this
// goroutine's onAction sleep and its completion signal actually
// resolves within that 500ms window — this keeps the test in the
// "action genuinely completes" regime the fix actually covers (see
// Stop()'s doc comment for the documented, out-of-scope "onAction
// outlives BlockingBudget" edge case). Given that the margin holds,
// Stop()'s s.runWg.Wait() join means Stop() cannot return until run()
// has returned, and run() cannot return while it is still inside a
// tick() call it already started, and that tick() call cannot return
// until onAction has already flipped actionRunning back to false. So,
// CONDITIONAL on the 12.5x margin holding on the host running this
// test, actionRunning MUST be false at the instant every Stop() call
// returns — not "usually", by construction, given that margin — but
// under extreme host contention (§11.4.174) that delays the 40ms sleep
// past the 500ms budget, the test would instead exercise the (also
// fixed, but functionally different) timeout-branch regime, and this
// specific determinism claim would no longer apply to that trial. The
// margin is generous enough that this is not expected in practice, but
// it is a margin-bound guarantee, not an unconditional one.
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

	completed := atomic.LoadInt32(&totalCompletedActions)
	t.Logf("%d/%d trials recorded a completed action (proves the tick→onAction path genuinely fired at least sometimes)", completed, iterations)

	// I3 fix (independent-review round): this guard was previously a
	// bare t.Logf — informative, but NOT an assertion. The test's own
	// doc comment above narrates the exact vacuity failure mode this
	// closes: an earlier version of this test raced Stop() with no
	// delay at all and measured 0/200 trials with a completed action —
	// a VACUOUS pass, because Stop() trivially never raced a tick that
	// never started, yet the `violations == 0` assertion below still
	// reported GREEN. If a future regression in the 5ms delay, the
	// TickInterval, or run()'s scheduling ever silently reintroduces
	// that vacuity, this test must FAIL loudly rather than pass green
	// while having proven nothing — hence a real assertion, not just a
	// log line.
	require.Greaterf(t, completed, int32(0),
		"anti-vacuity guard: 0/%d trials recorded a completed action — the tick→onAction path never genuinely fired, so the violations==0 assertion below would be VACUOUS (Stop() trivially never races a tick that never started), not evidence the fix works; see this test's doc comment for the historical 0/200 vacuous-pass incident this guards against",
		iterations)

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

// TestService_Tick_HangingOnDecisionDoesNotHangStop is the I2
// (independent-review round) regression guard for the FIRST failure
// mode callDecision's fix closes: onDecision is a caller-supplied
// callback invoked with NO context (see SetCallbacks's doc comment) —
// before the I2 fix it was called SYNCHRONOUSLY inside run()'s own
// goroutine, so a genuinely hanging onDecision (never returning) hung
// tick(), which hung run(), which meant run()'s deferred
// s.runWg.Done() never fired, which meant Stop()'s s.runWg.Wait() (see
// Stop()'s doc comment) blocked FOREVER — holding startMu for the
// duration, so EVERY subsequent Start()/Stop()/IsRunning() caller on
// this Service would also block. The I2 fix races onDecision against a
// decisionCtx bounded by s.config.BlockingBudget in its own goroutine
// (callDecision, mirroring the existing onAction pattern), so tick() —
// and therefore run(), and therefore Stop()'s join — is now bounded
// regardless of what onDecision does.
//
// This test drives the REAL Start()/Stop() API with an onDecision that
// sleeps far longer than BlockingBudget and asserts Stop() returns
// within a bounded deadline anyway. The onDecision goroutine itself is
// still running in the background after this test function returns —
// the SAME accepted, documented residual as onAction's detached
// -goroutine limitation (see Stop()'s doc comment); this test only
// proves Stop() itself is no longer hung by it.
func TestService_Tick_HangingOnDecisionDoesNotHangStop(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	config := DefaultConfig()
	config.Enabled = true
	config.TickInterval = time.Microsecond
	config.BlockingBudget = 50 * time.Millisecond
	// §9 data safety: LogPath MUST be a fresh t.TempDir(), NEVER the
	// real ~/.helixagent/kairos/logs DefaultConfig() points at on this
	// host.
	config.LogPath = t.TempDir()

	s := NewService(config, logger)

	decisionStarted := make(chan struct{}, 1)
	s.SetCallbacks(
		func(Observation) {},
		func(Action) {},
		func(TickPrompt) (Action, error) {
			select {
			case decisionStarted <- struct{}{}:
			default:
			}
			// Deliberately hangs FAR longer than BlockingBudget (50ms)
			// and cannot be told to stop — onDecision has no ctx
			// parameter at all (see SetCallbacks's doc comment) — this
			// simulates the worst case the I2 fix must survive.
			time.Sleep(2 * time.Second)
			return Action{}, nil
		},
	)

	require.NoError(t, s.Start(context.Background()))

	select {
	case <-decisionStarted:
		// onDecision has genuinely begun; Stop() below races a REAL
		// hanging decision, not one that never started (the same
		// vacuity concern I3's fix guards against for the sibling
		// onAction test above).
	case <-time.After(2 * time.Second):
		t.Fatal("onDecision never started — test cannot exercise the hang; the tick loop did not fire")
	}

	stopReturned := make(chan error, 1)
	go func() { stopReturned <- s.Stop() }()

	select {
	case err := <-stopReturned:
		require.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Stop() did not return within 1s while racing a hanging onDecision — I2 regression: Stop()'s s.runWg.Wait() join is blocked on a synchronous, unbounded onDecision call")
	}

	require.False(t, s.IsRunning())
}

// TestService_OnDecision_CallingStopSynchronously_DoesNotDeadlock is
// the I2 self-deadlock regression guard: an onDecision implementation
// that itself calls s.Stop() synchronously — a usage its signature does
// not forbid — used to deadlock PERMANENTLY prior to the I2 fix: that
// nested Stop() call would block on s.runWg.Wait() waiting for run() to
// return, but run() (calling onDecision synchronously) was itself
// blocked inside that exact nested Stop() call and could never return,
// so s.runWg.Done() could never fire, so the nested Stop() call could
// never unblock — a guaranteed, permanent hang. The I2 fix
// (callDecision's bounded race) means run()'s own goroutine is never
// blocked past BlockingBudget regardless of what onDecision's spawned
// goroutine does, so it reaches checkStop(), observes the
// already-closed s.stopCh (the nested Stop() call closed it before
// blocking on the wait), returns, and its deferred s.runWg.Done()
// unblocks the nested Stop() call.
//
// No production caller wires onDecision today (verified: no
// non-test/non-package reference to SetCallbacks/onDecision anywhere in
// this module), so this defect was latent, not live — fixed anyway
// before it becomes live, per the independent review.
func TestService_OnDecision_CallingStopSynchronously_DoesNotDeadlock(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	config := DefaultConfig()
	config.Enabled = true
	config.TickInterval = time.Microsecond
	config.BlockingBudget = 50 * time.Millisecond
	// §9 data safety: LogPath MUST be a fresh t.TempDir().
	config.LogPath = t.TempDir()

	s := NewService(config, logger)

	var selfStopOnce sync.Once
	var selfStopErr error
	selfStopDone := make(chan struct{})
	s.SetCallbacks(
		func(Observation) {},
		func(Action) {},
		func(TickPrompt) (Action, error) {
			// onDecision already executes in its OWN goroutine (spawned
			// by callDecision) — calling s.Stop() directly here IS the
			// "onDecision calls Stop() synchronously" scenario the I2
			// fix must survive; no further nested goroutine is needed to
			// reproduce it. sync.Once guards against the (harmless but
			// noisy) case where run() manages to fire a second tick
			// before observing the stopCh this Stop() call closes.
			selfStopOnce.Do(func() {
				selfStopErr = s.Stop()
				close(selfStopDone)
			})
			return Action{}, nil
		},
	)

	require.NoError(t, s.Start(context.Background()))

	select {
	case <-selfStopDone:
		require.NoError(t, selfStopErr)
	case <-time.After(2 * time.Second):
		t.Fatal("onDecision's nested s.Stop() call never returned within 2s — I2 self-deadlock regression: run() never reached checkStop() to observe the closed stopCh, so s.runWg.Done() never fired")
	}

	require.Eventually(t, func() bool { return !s.IsRunning() }, time.Second, 5*time.Millisecond,
		"service must report not-running once the self-triggered Stop() has completed")
}
