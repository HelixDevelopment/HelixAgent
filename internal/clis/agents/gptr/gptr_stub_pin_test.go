package gptr

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 STUB-BLUFF PIN GUARD — RED-on-broken-artifact + GREEN regression guard
// (§11.4.115 polarity switch / §11.4.135 standing guard)
//
// HISTORY: GPTR.run USED to return the FABRICATED literal "Result for: <prompt>"
// + status "completed" WITHOUT executing anything (zero os/exec) — a stub bluff
// per BLUFF-001 / CONST-035.
//
// FIX (D-17): GPTR has no headless LLM runner CLI; run now returns an HONEST
// error (ErrNoRunner). The task-management commands (create_task/list_tasks/
// get_result) remain REAL — they persist to and read from tasks.json on disk.
// ---------------------------------------------------------------------------

func TestD17_GPTR_RunNoFabricatedResult(t *testing.T) {
	g := New()
	res, err := g.run(context.Background(), map[string]interface{}{"prompt": "Process data"})
	if err == nil {
		t.Fatalf("D17 REGRESSION: GPTR.run returned success %v with no real runner — must return an honest error (BLUFF-001 reintroduced?).", res)
	}
	if !errors.Is(err, ErrNoRunner) {
		t.Fatalf("D17: GPTR.run error should wrap ErrNoRunner, got: %v", err)
	}
	if strings.Contains(err.Error(), "Result for: Process data") {
		t.Fatalf("D17 REGRESSION: GPTR.run fabricated the templated result.")
	}
}

// TestD17_GPTR_RunIsStubBluff — §11.4.115 RED-on-broken-artifact, RED_MODE=1.
func TestD17_GPTR_RunIsStubBluff(t *testing.T) {
	if os.Getenv("RED_MODE") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with RED_MODE=1. " +
			"The standing GREEN guard is TestD17_GPTR_RunNoFabricatedResult.")
	}
	g := New()
	res, err := g.run(context.Background(), map[string]interface{}{"prompt": "Process data"})
	if err != nil {
		return
	}
	m, _ := res.(map[string]interface{})
	if r, _ := m["result"].(string); strings.HasPrefix(r, "Result for: ") {
		t.Fatalf("D17 BLUFF PINNED: GPTR.run returned the fabricated template %q without executing anything (BLUFF-001).", r)
	}
}
