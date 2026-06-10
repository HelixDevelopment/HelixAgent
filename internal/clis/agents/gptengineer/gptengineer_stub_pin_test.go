package gptengineer

import (
	"context"
	"errors"
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 STUB-BLUFF PIN GUARD — RED-on-broken-artifact + GREEN regression guard
// (§11.4.115 polarity switch / §11.4.135 standing guard)
//
// HISTORY: GPTEngineer.generate USED to return a FABRICATED fixed file list
// (["main.py","README.md","requirements.txt"], status "generated") and improve a
// fabricated "improved" status WITHOUT running anything (zero os/exec) — a stub
// bluff per BLUFF-001 / CONST-035.
//
// FIX (D-17): the real gpt-engineer project-scaffolding run is not wired;
// generate/improve return an HONEST error (ErrCLINotWired) rather than
// fabricating a file list.
// ---------------------------------------------------------------------------

func TestD17_GPTEngineer_NoFabricatedFileList(t *testing.T) {
	g := New()
	ctx := context.Background()

	gres, gerr := g.generate(ctx, map[string]interface{}{"prompt": "build a todo app"})
	if gerr == nil {
		t.Fatalf("D17 REGRESSION: GPTEngineer.generate returned success %v with no real run — must return an honest error (BLUFF-001 reintroduced?).", gres)
	}
	if !errors.Is(gerr, ErrCLINotWired) {
		t.Fatalf("D17: GPTEngineer.generate error should wrap ErrCLINotWired, got: %v", gerr)
	}

	ires, ierr := g.improve(ctx, map[string]interface{}{"file": "main.py"})
	if ierr == nil {
		t.Fatalf("D17 REGRESSION: GPTEngineer.improve returned success %v with no real run — must return an honest error.", ires)
	}
	if !errors.Is(ierr, ErrCLINotWired) {
		t.Fatalf("D17: GPTEngineer.improve error should wrap ErrCLINotWired, got: %v", ierr)
	}
}

// TestD17_GPTEngineer_GenerateIsStubBluff — §11.4.115 RED-on-broken-artifact, RED_MODE=1.
func TestD17_GPTEngineer_GenerateIsStubBluff(t *testing.T) {
	if os.Getenv("RED_MODE") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with RED_MODE=1. " +
			"The standing GREEN guard is TestD17_GPTEngineer_NoFabricatedFileList.")
	}
	g := New()
	res, err := g.generate(context.Background(), map[string]interface{}{"prompt": "x"})
	if err != nil {
		return
	}
	m, _ := res.(map[string]interface{})
	if files, ok := m["files"].([]string); ok && len(files) > 0 {
		t.Fatalf("D17 BLUFF PINNED: GPTEngineer.generate returned a fabricated file list %v without any real run (BLUFF-001).", files)
	}
}
