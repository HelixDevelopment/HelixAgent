package precondition

import (
	"os"
	"testing"
)

// §11.4.115 RED-baseline / GREEN-guard for the PORT_CONFLICT ownership defect.
//
// Historical defect (captured 2026-07-27, qa-results/full_retest/
// helix_agent_infraup_20260727T181252Z.log): detectMultipleServiceInstances
// probed {port, port+10000} with a bare TCP dial and declared
//
//	PORT_CONFLICT: Service 'postgres' appears to be running on multiple ports: [5432 15432]
//
// with NO ownership verification. This project's own test stack deliberately
// publishes postgres on 15432 (= 5432 + 10000), so ANY unrelated postgres
// listening on the standard port 5432 — on a shared developer workstation that
// is the normal case, here another project's stack — tripped a false failure
// and blocked the precondition gate.
//
// That violates §11.4.174 (every process/port we draw a conclusion from MUST be
// positively verified as OURS) and §11.4.201 (a gate MUST assert the REAL
// condition; a false-positive refusal is a FAIL-bluff).
//
// Polarity switch per §11.4.115: RED_MODE=1 (default) reproduces and asserts the
// defect is PRESENT on a pre-fix artifact; RED_MODE=0 is the standing GREEN
// regression guard asserting the defect is ABSENT.
func redMode() bool { return os.Getenv("RED_MODE") == "1" }

// legacyPortConflict is the VERBATIM pre-fix decision logic, retained solely so
// the RED baseline can prove the defect really existed rather than asserting a
// synthetic failure. It is never used by production code paths.
func legacyPortConflict(basePort int, isListening func(int) bool) bool {
	var found []int
	for _, p := range []int{basePort, basePort + 10000} {
		if isListening(p) {
			found = append(found, p)
		}
	}
	return len(found) > 1
}

// TestPortConflict_ForeignInstanceOnStandardPortIsNotOurs is the reproduce-first
// test for the defect: a FOREIGN postgres on 5432 plus OUR postgres on 15432.
// Exactly one instance is ours, so a correct gate reports no conflict.
func TestPortConflict_ForeignInstanceOnStandardPortIsNotOurs(t *testing.T) {
	// Mirrors the live host at the time of capture: another project's stack
	// holds 5432; our test stack holds 15432.
	listening := map[int]bool{5432: true, 15432: true}
	isListening := func(p int) bool { return listening[p] }

	// Ownership as reported by the container runtime: only ONE of these
	// containers carries this project's prefix.
	runningContainers := []string{
		"helixterm-postgres",      // another project on the shared host
		"penpot-postgres",         // another project on the shared host
		"helixagent-postgres",     // OURS
	}

	if redMode() {
		// RED: the pre-fix logic conflates foreign listeners with our own.
		if !legacyPortConflict(5432, isListening) {
			t.Fatalf("RED baseline did not reproduce: legacy logic reported no conflict " +
				"for {5432 foreign, 15432 ours}; the defect must be present pre-fix")
		}
		t.Log("RED reproduced: legacy blind {port, port+10000} probe flags a foreign " +
			"listener as our own duplicate instance")
		return
	}

	// GREEN guard: the ownership-aware check must NOT report a conflict, because
	// only one running instance of 'postgres' belongs to this project.
	ours := ourServiceInstances(runningContainers, "postgres")
	if len(ours) != 1 {
		t.Fatalf("ownership filter wrong: want exactly 1 owned postgres instance, got %d (%v)",
			len(ours), ours)
	}
	if err := portConflict("postgres", ours); err != nil {
		t.Fatalf("false positive: foreign postgres on the standard port must not be "+
			"attributed to this project (§11.4.174); got error: %v", err)
	}
}

// TestPortConflict_TwoOwnedInstancesIsARealConflict proves the guard still
// catches the condition it exists to catch — the check must not be weakened
// into a tautology (§11.4.120).
func TestPortConflict_TwoOwnedInstancesIsARealConflict(t *testing.T) {
	runningContainers := []string{
		"helixagent-postgres",
		"helixagent-postgres-2", // a genuine duplicate of OURS
		"helixterm-postgres",    // foreign, must be ignored
	}

	ours := ourServiceInstances(runningContainers, "postgres")
	if len(ours) != 2 {
		t.Fatalf("want 2 owned postgres instances, got %d (%v)", len(ours), ours)
	}
	if err := portConflict("postgres", ours); err == nil {
		t.Fatal("two instances owned by THIS project must be reported as a conflict")
	}
}

// TestOurServiceInstances_IgnoresForeignAndUnrelated pins the ownership filter.
func TestOurServiceInstances_IgnoresForeignAndUnrelated(t *testing.T) {
	all := []string{
		"helixagent-postgres",
		"helixagent-redis",
		"helixterm-postgres",
		"penpot-postgres",
		"helixcode-infra-postgres",
		"",
	}

	cases := []struct {
		svc  string
		want int
	}{
		{"postgres", 1},
		{"redis", 1},
		{"chromadb", 0},
	}

	for _, c := range cases {
		got := ourServiceInstances(all, c.svc)
		if len(got) != c.want {
			t.Errorf("service %q: want %d owned instance(s), got %d (%v)",
				c.svc, c.want, len(got), got)
		}
	}
}
