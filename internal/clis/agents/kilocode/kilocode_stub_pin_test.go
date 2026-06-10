package kilocode

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
// HISTORY: KiloCode.complete USED to return the FABRICATED literal
// "// KiloCode completion" WITHOUT any real backend (zero os/exec) — a stub
// bluff per BLUFF-001 / CONST-035.
//
// FIX (D-17): Kilo Code is an IDE extension with no headless CLI; complete now
// returns an HONEST error (ErrIDEExtensionOnly) rather than fabricating output.
// ---------------------------------------------------------------------------

func TestD17_KiloCode_NoFabricatedCompletion(t *testing.T) {
	k := New()
	res, err := k.complete(context.Background(), map[string]interface{}{"prefix": "func add("})
	if err == nil {
		t.Fatalf("D17 REGRESSION: KiloCode.complete returned success %v with no real backend — must return an honest error (BLUFF-001 reintroduced?).", res)
	}
	if !errors.Is(err, ErrIDEExtensionOnly) {
		t.Fatalf("D17: KiloCode.complete error should wrap ErrIDEExtensionOnly, got: %v", err)
	}
}

// TestD17_KiloCode_CompleteIsStubBluff — §11.4.115 RED-on-broken-artifact, RED_MODE=1.
func TestD17_KiloCode_CompleteIsStubBluff(t *testing.T) {
	if os.Getenv("RED_MODE") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with RED_MODE=1. " +
			"The standing GREEN guard is TestD17_KiloCode_NoFabricatedCompletion.")
	}
	k := New()
	res, err := k.complete(context.Background(), map[string]interface{}{"prefix": "x"})
	if err != nil {
		return
	}
	m, _ := res.(map[string]interface{})
	if c, _ := m["completion"].(string); strings.Contains(c, "KiloCode completion") {
		t.Fatalf("D17 BLUFF PINNED: KiloCode.complete returned the fabricated literal %q without a real backend (BLUFF-001).", c)
	}
}
