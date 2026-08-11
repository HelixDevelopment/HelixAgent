package integration

// Teardown-ownership guard for the container harness (HXC-248).
//
// # The defect this file exists to catch
//
// ContainerHarness.Cleanup used to decide whether it was allowed to run
// ComposeDown by re-deriving three conditions at teardown time, the last
// of which was a bare TCP dial of localhost:8100:
//
//	if helixAgentPortOpen() {
//	        h.Logger.Info("HelixAgent still running on :8100 — skipping ComposeDown (not our containers to stop)")
//	        h.cancel()
//	        return nil
//	}
//	...
//	if err := h.Adapter.ComposeDown(ctx, "docker-compose.yml", "default"); err != nil {
//
// Two independent faults, either of which alone is sufficient:
//
//	(1) IDENTITY-BLIND. "Something accepted a TCP connection" is a
//	    liveness signal. Ownership — "did THIS harness start these
//	    containers" — is a different question that no probe can answer.
//	    Even a correct, identity-verified probe of a real HelixAgent
//	    would not establish it.
//	(2) WRONG PROCESS. Measured on the development host 2026-08-11,
//	    :8100 is held by llm-verifier (HTTP 404 "404 page not found")
//	    while HelixAgent answers on :7061 with
//	    {"service":"helixagent","status":"healthy"}. The probe named in
//	    that log line never reached HelixAgent at all.
//
// The blast radius was the entire docker-compose.yml project — postgres,
// redis, ollama, cognee, chromadb, neo4j, and the helixagent service
// itself — torn down whenever that unrelated port happened not to answer,
// by a harness that had started nothing. The platform survived only
// because a foreign process kept the port occupied. That is a
// coincidence, not a guard, and it is load-bearing in the wrong
// direction: relocating llm-verifier off :8100 would ARM the destructive
// path rather than fix it.
//
// # Polarity switch (§11.4.115)
//
// RED_MODE=1 (opt-in) reproduces the defect: it asserts that Cleanup
// DOES tear down a project the harness never started. It passes only on
// the broken artifact — restore the historical guard in Cleanup and this
// goes green, which is how the guard was proven real (and is the paired
// §1.1 mutation).
//
// RED_MODE=0 is the standing regression guard: Cleanup MUST NOT tear
// down anything it did not start. THIS IS NOW THE DEFAULT, so the guard
// runs on every build (§11.4.135) rather than only when RED_MODE is set
// explicitly.
//
// # Why the destructive path is exercised through a seam
//
// The naive reproduction — let Cleanup run ComposeDown for real — would
// cause exactly the outage this item exists to prevent. Every test here
// injects a recording composeDown and drives the real Cleanup logic; the
// terminal syscall is captured rather than executed. No test in this file
// starts, stops, inspects, or contacts any container, and the only
// network fixtures are throwaway listeners this test owns.

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// redMode reports whether the suite runs in defect-reproduction mode.
// Only the literal "1" turns it ON; unset or anything else is the
// standing GREEN regression guard — THIS IS NOW THE DEFAULT.
//
// The inverse (unset = RED) was written first and rejected on review.
// The invariant that decides this — and the only thing worth freezing
// in a comment — is:
//
//	This file carries no //go:build tag, and neither does the
//	container_harness.go it tests. So the pair COMPILES under every
//	invocation of this package, and RUNS under every one that neither
//	filters it out by test name (-run) nor passes -short. Verify with:
//	`go test ./tests/integration/` (no env, no tags, no flags).
//
// The -short half is not hypothetical: TestMain in
// container_integration_test.go inspects os.Args and os.Exit(0)s the
// WHOLE package before m.Run() when -test.short is present (CONST-030,
// integration tests need real infrastructure). So `go test -short` runs
// zero tests here with no -run filter involved. Note `go test` without
// -v buffers package stdout and discards it on success, so the "SKIP:"
// line TestMain prints is invisible unless you pass -v — an absent
// message is not evidence the package ran.
//
// Under an unset-means-RED default every invocation that DOES run the
// pair goes permanently red, AND the standing GREEN guard never
// executes in any of them — a §11.4.135 violation, since the guard must
// run on every build. A permanent false alarm around a production-stack
// guard also trains operators to ignore red.
//
// Deliberately NOT enumerated here: which Makefile targets and scripts
// are filtered vs unfiltered. Three consecutive review rounds found
// that enumeration wrong — too few plain runners, then "every wired
// invocation" overreaching across -run-filtered targets, then a
// filtered target missed. It is a derived classification over a
// 2000-line Makefile plus scripts/ that drifts whenever a caller is
// added, so a frozen list in a comment is guaranteed to rot and has
// already misinformed three times.
//
// Re-derive it instead when you need it — but treat the search as a
// FLOOR, never a census. Two complementary sweeps, because neither
// alone reaches every caller:
//
//	git grep -n 'go test'           # callers that spell the go tool
//	git grep -n 'tests/integration' # callers that name the package
//
// For each hit, three questions. Any single "no"/"yes-gated" means that
// invocation does not run this pair:
//
//  1. Does its package spec include this package? Resolve make and
//     shell variables, line continuations, and any `cd` — all three
//     routinely sit somewhere other than the matched line. A subtree
//     spec like ./tests/integration/verifier/... does NOT include the
//     parent package, and a caller that cd's elsewhere first is not
//     ours at all.
//  2. Is it gated by a -run pattern matching none of this file's test
//     names?
//  3. Is it gated by -short?
//
// Why a floor and not a census — this is the load-bearing part. Three
// successive "complete" recipes were written here and each was
// falsified within one review round, on a different axis every time:
//
//   - spelling — `tests/integration` missed `go test -v ./tests/... -json`
//   - indirection — scripts/run-integration-tests.sh assigns
//     TEST_PACKAGE="./..." 380 lines above the four `go test
//     … "$TEST_PACKAGE"` lines that use it
//   - runner + roots — ci/scripts/ci-integration.sh runs this package
//     through gotestsum and contains ZERO `go test` literals, and it
//     lives outside Makefile/scripts/ entirely
//
// So do not believe any claim, including this one, that some pattern
// finds every caller. The second sweep exists precisely because the
// first cannot see gotestsum; it catches those lines because they NAME
// the package. Both together still miss a wrapper that holds the
// package path in a variable. Treat a caller you find as confirmed and
// a caller you did not find as unproven — that is all a grep over an
// open set can honestly promise, and it is why this framing is stable
// where the previous three were not: a new counterexample strengthens
// it instead of falsifying it.
//
// Matches this repo's existing polarity precedent:
// cmd/grpc-server/listen_addr_test.go and
// cmd/helixagent/mcp_startup_propagation_test.go.
func redMode() bool {
	return strings.TrimSpace(os.Getenv("RED_MODE")) == "1"
}

// teardownRecorder captures every composeDown invocation instead of
// performing it.
type teardownRecorder struct {
	mu    sync.Mutex
	calls []teardownCall
	err   error
}

type teardownCall struct {
	composeFile string
	profile     string
}

func (r *teardownRecorder) fn(_ context.Context, composeFile, profile string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, teardownCall{composeFile: composeFile, profile: profile})
	return r.err
}

func (r *teardownRecorder) snapshot() []teardownCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]teardownCall, len(r.calls))
	copy(out, r.calls)
	return out
}

func (r *teardownRecorder) count() int { return len(r.snapshot()) }

// newTestHarness builds a harness with no container adapter at all. A nil
// Adapter is deliberate: it proves these tests cannot reach a real
// container runtime even if the code under test tried to.
func newTestHarness(t *testing.T) (*ContainerHarness, *teardownRecorder) {
	t.Helper()

	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	rec := &teardownRecorder{}
	h := &ContainerHarness{
		Adapter:     nil,
		Logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		servicesUp:  make(map[string]bool),
		projectRoot: t.TempDir(),
		composeDown: rec.fn,
	}
	return h, rec
}

// listenOn binds a throwaway TCP listener and returns its port. Used to
// place a foreign responder on a port the historical guard probed, and to
// validate the probe instrument itself.
func listenOn(t *testing.T) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not bind throwaway listener: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	return ln, ln.Addr().(*net.TCPAddr).Port
}

func dialable(port int) bool {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// -------------------------------------------------------------------------
// Instrument validation (§11.4.201) — before believing any negative result
// ("no teardown happened"), prove the recorder can observe a positive one.
// A recorder that never reports a call would make every assertion below
// vacuously green.
// -------------------------------------------------------------------------

func TestTeardownRecorder_DetectsAKnownPresentCall(t *testing.T) {
	h, rec := newTestHarness(t)

	if got := rec.count(); got != 0 {
		t.Fatalf("recorder should start empty, saw %d calls", got)
	}

	// Known-present case: a harness that DID start a project must produce
	// exactly one observable teardown.
	h.markOwned("docker-compose.yml", "default")
	if err := h.Cleanup(); err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}

	calls := rec.snapshot()
	if len(calls) != 1 {
		t.Fatalf("instrument invalid: expected 1 recorded teardown, got %d (%+v)", len(calls), calls)
	}
	t.Logf("instrument validated: recorder observed %+v", calls[0])
}

func TestPortProbeInstrument_PositiveAndNegativeControls(t *testing.T) {
	_, port := listenOn(t)

	if !dialable(port) {
		t.Fatalf("instrument invalid: could not reach a listener this test just bound on :%d", port)
	}
	t.Logf("positive control: :%d is dialable while the fixture listener is bound", port)

	// Negative control: a port nothing owns must NOT be dialable,
	// otherwise "port closed" carries no information.
	freeLn, freePort := listenOn(t)
	_ = freeLn.Close()
	// Give the kernel a moment to release the port.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && dialable(freePort) {
		time.Sleep(50 * time.Millisecond)
	}
	if dialable(freePort) {
		t.Skipf("negative control unavailable: :%d still accepting after close (SKIP-OK: #port-reuse-timing)", freePort)
	}
	t.Logf("negative control: :%d is not dialable after the listener closed", freePort)
}

// -------------------------------------------------------------------------
// CORE — the reproduce-first / confirm-fix pair (§11.4.115, §11.4.146)
// -------------------------------------------------------------------------

// TestCleanup_NeverStopsWhatItDidNotStart is the defect's home. In RED
// mode it asserts the destructive behaviour is present; in GREEN mode it
// asserts the destructive behaviour is gone. One source, two roles.
func TestCleanup_NeverStopsWhatItDidNotStart(t *testing.T) {
	h, rec := newTestHarness(t)

	// The harness has started nothing: BootAllServices never ran, or ran
	// and skipped its ComposeUp. This is the state during every run
	// against an already-running platform.
	if h.ownership() != nil {
		t.Fatalf("precondition violated: fresh harness must own nothing, got %+v", h.ownership())
	}

	if err := h.Cleanup(); err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}

	calls := rec.snapshot()

	if redMode() {
		// RED_MODE=1: reproduce. The historical guard reaches ComposeDown
		// here because nothing answered its probe.
		if len(calls) == 0 {
			t.Fatalf("RED_MODE=1 expected to reproduce the defect (a teardown of an unowned project) "+
				"but Cleanup performed none. Either the artifact is already fixed — run with RED_MODE=0 "+
				"for the regression guard — or this test no longer reaches the destructive path. "+
				"ownership=%v", h.ownership())
		}
		t.Logf("DEFECT REPRODUCED: harness started nothing yet Cleanup tore down %+v", calls)
		return
	}

	// RED_MODE=0: the standing guard.
	if len(calls) != 0 {
		t.Fatalf("REGRESSION: Cleanup tore down %d project(s) the harness never started: %+v. "+
			"Teardown authority must come from the ownership record alone.", len(calls), calls)
	}
	t.Logf("guard holds: unowned harness performed no teardown")
}

// TestCleanup_StopsExactlyWhatItStarted is the positive half. A harness
// that DID start a project must still tear that project down — the fix
// must not achieve safety by simply never cleaning up.
func TestCleanup_StopsExactlyWhatItStarted(t *testing.T) {
	if redMode() {
		t.Skip("positive-path assertion is meaningful only against the fixed artifact (SKIP-OK: #red-mode)")
	}

	h, rec := newTestHarness(t)
	h.markOwned("docker-compose.integration.yml", "ci")

	if err := h.Cleanup(); err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}

	calls := rec.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 teardown of the owned project, got %d: %+v", len(calls), calls)
	}
	// The values torn down must be the ones recorded at start time, not
	// re-read defaults. A hardcoded "docker-compose.yml"/"default" would
	// tear down a project other than the one started.
	if calls[0].composeFile != "docker-compose.integration.yml" || calls[0].profile != "ci" {
		t.Fatalf("teardown targeted the wrong project: got %+v, want {docker-compose.integration.yml ci}", calls[0])
	}
	t.Logf("owned project torn down with its own identity: %+v", calls[0])
}

// -------------------------------------------------------------------------
// EXTEND — the case space of the same functionality (§11.4.146 STEP 3,
// §11.4.118 enumerated coverage). Every case below runs against the fixed
// artifact; each states the outcome it asserts.
// -------------------------------------------------------------------------

func TestCleanup_ExtendCaseSpace(t *testing.T) {
	if redMode() {
		t.Skip("extend case-set characterises the fixed artifact (SKIP-OK: #red-mode)")
	}

	t.Run("started_zero_containers", func(t *testing.T) {
		h, rec := newTestHarness(t)
		if err := h.Cleanup(); err != nil {
			t.Fatalf("Cleanup error: %v", err)
		}
		if rec.count() != 0 {
			t.Fatalf("expected no teardown, got %+v", rec.snapshot())
		}
	})

	t.Run("started_one_project", func(t *testing.T) {
		h, rec := newTestHarness(t)
		h.markOwned("docker-compose.yml", "default")
		if err := h.Cleanup(); err != nil {
			t.Fatalf("Cleanup error: %v", err)
		}
		if rec.count() != 1 {
			t.Fatalf("expected 1 teardown, got %+v", rec.snapshot())
		}
	})

	t.Run("started_n_times_tears_down_latest_once", func(t *testing.T) {
		// A harness re-booted within one process must not accumulate
		// teardowns; the record is the current project, not a log.
		h, rec := newTestHarness(t)
		h.markOwned("a.yml", "p1")
		h.markOwned("b.yml", "p2")
		h.markOwned("c.yml", "p3")
		if err := h.Cleanup(); err != nil {
			t.Fatalf("Cleanup error: %v", err)
		}
		calls := rec.snapshot()
		if len(calls) != 1 || calls[0].composeFile != "c.yml" || calls[0].profile != "p3" {
			t.Fatalf("expected single teardown of c.yml/p3, got %+v", calls)
		}
	})

	t.Run("foreign_responder_on_probed_port_does_not_authorise_teardown", func(t *testing.T) {
		// A foreign process answering a port must not change the verdict
		// in EITHER direction. This is the llm-verifier-on-:8100 shape.
		_, port := listenOn(t)
		if !dialable(port) {
			t.Fatalf("fixture invalid: listener not reachable on :%d", port)
		}
		h, rec := newTestHarness(t)
		if err := h.Cleanup(); err != nil {
			t.Fatalf("Cleanup error: %v", err)
		}
		if rec.count() != 0 {
			t.Fatalf("a foreign responder must not authorise teardown, got %+v", rec.snapshot())
		}
	})

	t.Run("probe_endpoint_unreachable_still_no_teardown", func(t *testing.T) {
		// THE historical destructive case: nothing answers, so the old
		// guard fell through to ComposeDown. Ownership is still nil, so
		// the answer must remain "stop nothing".
		ln, port := listenOn(t)
		_ = ln.Close()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) && dialable(port) {
			time.Sleep(50 * time.Millisecond)
		}
		h, rec := newTestHarness(t)
		if err := h.Cleanup(); err != nil {
			t.Fatalf("Cleanup error: %v", err)
		}
		if rec.count() != 0 {
			t.Fatalf("unreachable probe endpoint must not arm teardown, got %+v", rec.snapshot())
		}
	})

	t.Run("teardown_failure_retains_ownership", func(t *testing.T) {
		// If ComposeDown fails, the project is still (partly) ours. The
		// record must survive so a retry can finish the job; silently
		// releasing it would leak containers.
		h, rec := newTestHarness(t)
		rec.err = fmt.Errorf("compose down refused")
		h.markOwned("docker-compose.yml", "default")

		err := h.Cleanup()
		if err == nil {
			t.Fatalf("expected Cleanup to surface the teardown failure")
		}
		if h.ownership() == nil {
			t.Fatalf("ownership must be retained after a failed teardown so it can be retried")
		}
	})

	t.Run("double_cleanup_is_idempotent", func(t *testing.T) {
		h, rec := newTestHarness(t)
		h.markOwned("docker-compose.yml", "default")
		if err := h.Cleanup(); err != nil {
			t.Fatalf("first Cleanup error: %v", err)
		}
		if err := h.Cleanup(); err != nil {
			t.Fatalf("second Cleanup error: %v", err)
		}
		if rec.count() != 1 {
			t.Fatalf("second Cleanup must be a no-op, got %d teardowns: %+v", rec.count(), rec.snapshot())
		}
	})

	t.Run("env_opt_out_never_influences_teardown", func(t *testing.T) {
		// The historical Cleanup consulted HELIX_SKIP_CONTAINER_HARNESS
		// with a different predicate than BootAllServices used ("" vs
		// !=0/!=false), so `false` booted containers but skipped their
		// teardown. Teardown must not read the variable at all now.
		for _, v := range []string{"", "0", "false", "1", "true", "yes"} {
			t.Setenv("HELIX_SKIP_CONTAINER_HARNESS", v)

			hOwned, recOwned := newTestHarness(t)
			hOwned.markOwned("docker-compose.yml", "default")
			if err := hOwned.Cleanup(); err != nil {
				t.Fatalf("value %q: Cleanup error: %v", v, err)
			}
			if recOwned.count() != 1 {
				t.Fatalf("value %q: an owned project must be torn down regardless of the env var, got %+v",
					v, recOwned.snapshot())
			}

			hUnowned, recUnowned := newTestHarness(t)
			if err := hUnowned.Cleanup(); err != nil {
				t.Fatalf("value %q: Cleanup error: %v", v, err)
			}
			if recUnowned.count() != 0 {
				t.Fatalf("value %q: an unowned harness must tear down nothing, got %+v",
					v, recUnowned.snapshot())
			}
		}
	})

	t.Run("concurrent_harnesses_only_touch_their_own", func(t *testing.T) {
		const n = 8
		var wg sync.WaitGroup
		recs := make([]*teardownRecorder, n)
		for i := 0; i < n; i++ {
			h, rec := newTestHarness(t)
			recs[i] = rec
			// Even indices started a project; odd ones did not.
			owns := i%2 == 0
			if owns {
				h.markOwned(fmt.Sprintf("compose-%d.yml", i), "default")
			}
			wg.Add(1)
			go func(h *ContainerHarness) {
				defer wg.Done()
				_ = h.Cleanup()
			}(h)
		}
		wg.Wait()

		for i, rec := range recs {
			want := 0
			if i%2 == 0 {
				want = 1
			}
			if rec.count() != want {
				t.Fatalf("harness %d: expected %d teardown(s), got %d: %+v", i, want, rec.count(), rec.snapshot())
			}
			if want == 1 && rec.snapshot()[0].composeFile != fmt.Sprintf("compose-%d.yml", i) {
				t.Fatalf("harness %d tore down another harness's project: %+v", i, rec.snapshot()[0])
			}
		}
	})

	t.Run("teardown_invariant_under_BOTH_probe_states", func(t *testing.T) {
		// The decisive case, and the one that makes this guard
		// host-independent. Whether helixAgentPortOpen answers true or
		// false is ambient state no test controls: :8100 is occupied on
		// this host (llm-verifier) and may be free on another. The
		// destructive branch of the historical guard was reachable ONLY
		// in the false state, so a guard that can observe just one state
		// silently stops protecting anything on half the hosts — and
		// would let a re-introduced probe through. Forcing both states
		// proves the verdict does not depend on the probe at all.
		orig := helixAgentPortOpen
		t.Cleanup(func() { helixAgentPortOpen = orig })

		for _, probeAnswers := range []bool{true, false} {
			helixAgentPortOpen = func() bool { return probeAnswers }

			hUnowned, recUnowned := newTestHarness(t)
			if err := hUnowned.Cleanup(); err != nil {
				t.Fatalf("probe=%v: Cleanup error: %v", probeAnswers, err)
			}
			if recUnowned.count() != 0 {
				t.Fatalf("probe=%v: an unowned harness tore down %+v — teardown must not consult any probe",
					probeAnswers, recUnowned.snapshot())
			}

			hOwned, recOwned := newTestHarness(t)
			hOwned.markOwned("docker-compose.yml", "default")
			if err := hOwned.Cleanup(); err != nil {
				t.Fatalf("probe=%v: Cleanup error: %v", probeAnswers, err)
			}
			if recOwned.count() != 1 {
				t.Fatalf("probe=%v: an owned project must still be torn down, got %+v",
					probeAnswers, recOwned.snapshot())
			}
			t.Logf("probe=%v: unowned->0 teardowns, owned->1 teardown", probeAnswers)
		}
	})

	t.Run("ownership_record_is_race_free", func(t *testing.T) {
		// Exercised under -race: concurrent readers and a writer.
		h, _ := newTestHarness(t)
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				if i%2 == 0 {
					h.markOwned(fmt.Sprintf("c-%d.yml", i), "default")
					return
				}
				_ = h.ownership()
			}(i)
		}
		wg.Wait()
	})
}

// -------------------------------------------------------------------------
// The decision function is the whole teardown authority. Asserting it
// directly keeps the §1.1 mutation target unambiguous: adding any input
// other than the ownership record re-opens the defect.
// -------------------------------------------------------------------------

func TestDecideTeardown_OwnershipIsTheOnlyInput(t *testing.T) {
	if redMode() {
		t.Skip("decision-function contract describes the fixed artifact (SKIP-OK: #red-mode)")
	}

	got, reason := decideTeardown(nil)
	if got != teardownSkipNotOwned {
		t.Fatalf("nil ownership must yield %q, got %q", teardownSkipNotOwned, got)
	}
	if reason == "" {
		t.Fatalf("decision must carry a reason for the log")
	}

	got, reason = decideTeardown(&composeOwnership{
		composeFile: "docker-compose.yml",
		profile:     "default",
		startedAt:   time.Now(),
	})
	if got != teardownStopOwned {
		t.Fatalf("recorded ownership must yield %q, got %q", teardownStopOwned, got)
	}
	if !strings.Contains(reason, "docker-compose.yml") {
		t.Fatalf("reason must name the project being stopped, got %q", reason)
	}
}
