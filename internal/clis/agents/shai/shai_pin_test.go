package shai

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 PIN GUARD — shai real-exec wiring (§11.4.115 / §11.4.135)
//
// HISTORY (the bluff this guard catches): Shai.generate/explain USED to return
// TEMPLATED LITERAL strings ("# Shai command for: <desc>", "Explanation of:
// <cmd>") WITHOUT ever exec-ing the real `shai` CLI (zero os/exec in shai.go) —
// a BLUFF-001 / CONST-035 false-success: the agent claimed to generate a shell
// command while running nothing.
//
// FIX: generate/explain now exec the real shai CLI in headless mode (prompt on
// stdin) via exec.LookPath + exec.CommandContext (resolveShaiBinary + runShai).
// When the binary is absent they return an HONEST error, never a fabricated
// success.
//
// This guard proves REAL exec is wired by injecting a FAKE shai binary on PATH
// (SHAI_BIN override) whose stdout is an unforgeable marker, then asserting the
// methods return the fake binary's REAL stdout and never the old templates.
// ---------------------------------------------------------------------------

// writeFakeShai writes an executable shell script that echoes a marker plus the
// stdin it received (shai headless mode reads the prompt from stdin), proving
// both that exec ran AND that the prompt was forwarded. Skips on non-POSIX.
func writeFakeShai(t *testing.T, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: D-17 — fake-binary injection uses a POSIX shell script; " +
			"the real-exec code path is identical across platforms.")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-shai")
	// Reads all of stdin into $p, prints {marker}:{stdin}.
	script := "#!/bin/sh\np=$(cat)\nprintf '" + marker + ":%s' \"$p\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake shai: %v", err)
	}
	return bin
}

func TestD17_Shai_GenerateExecsRealBinary(t *testing.T) {
	const marker = "FAKE_SHAI_RAN_a71c"
	bin := writeFakeShai(t, marker)
	t.Setenv("SHAI_BIN", bin)

	s := New()
	ctx := context.Background()

	res, err := s.generate(ctx, map[string]interface{}{"description": "list files by size"})
	if err != nil {
		t.Fatalf("generate returned error with fake binary injected: %v", err)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}
	cmd, _ := m["command"].(string)
	if !strings.Contains(cmd, marker) {
		t.Fatalf("D-17 REGRESSION: Shai.generate did NOT exec the shai binary — marker %q absent from %q (BLUFF-001 reintroduced?).", marker, cmd)
	}
	if strings.HasPrefix(cmd, "# Shai command for:") {
		t.Fatalf("D-17 REGRESSION: Shai.generate returned the templated literal %q instead of real process output (BLUFF-001).", cmd)
	}
	if !strings.Contains(cmd, "list files by size") {
		t.Fatalf("D-17 REGRESSION: description was not forwarded to the shai binary (got %q).", cmd)
	}

	eres, err := s.explain(ctx, map[string]interface{}{"command": "tar -xzf x.tgz"})
	if err != nil {
		t.Fatalf("explain returned error with fake binary injected: %v", err)
	}
	em, _ := eres.(map[string]interface{})
	expl, _ := em["explanation"].(string)
	if !strings.Contains(expl, marker) {
		t.Fatalf("D-17 REGRESSION: Shai.explain did NOT exec the shai binary — marker %q absent from %q.", marker, expl)
	}
	if strings.HasPrefix(expl, "Explanation of:") {
		t.Fatalf("D-17 REGRESSION: Shai.explain returned the templated literal %q (BLUFF-001).", expl)
	}
}

// TestD17_Shai_AbsentBinaryIsHonestError proves that with NO shai binary
// available the methods return an honest error (NOT a fabricated success).
func TestD17_Shai_AbsentBinaryIsHonestError(t *testing.T) {
	t.Setenv("SHAI_BIN", filepath.Join(t.TempDir(), "does-not-exist-shai"))

	s := New()
	ctx := context.Background()

	if res, err := s.generate(ctx, map[string]interface{}{"description": "x"}); err == nil {
		t.Fatalf("D-17 BLUFF: generate returned success %v with NO shai binary — must be an honest error.", res)
	}
	if res, err := s.explain(ctx, map[string]interface{}{"command": "x"}); err == nil {
		t.Fatalf("D-17 BLUFF: explain returned success %v with NO shai binary — must be an honest error.", res)
	}
	// IsAvailable must be honest (false) when the binary is absent.
	if s.IsAvailable() {
		t.Fatal("D-17 BLUFF: IsAvailable() = true with NO shai binary — must reflect real PATH state.")
	}
}

// TestD17_Shai_IsStubBluff is the §11.4.115 RED-on-broken-artifact reproduction,
// runnable only under PIN_STUB_BLUFF=1. On the pre-fix stub artifact generate
// returned the templated literal; on the fixed artifact (no binary injected) it
// returns an honest error, so the bluff literal must be ABSENT.
func TestD17_Shai_IsStubBluff(t *testing.T) {
	if os.Getenv("PIN_STUB_BLUFF") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with PIN_STUB_BLUFF=1.")
	}
	t.Setenv("SHAI_BIN", filepath.Join(t.TempDir(), "does-not-exist-shai"))
	s := New()
	ctx := context.Background()
	res, err := s.generate(ctx, map[string]interface{}{"description": "list files"})
	if err != nil {
		return // fixed artifact: honest error — bluff gone.
	}
	m, _ := res.(map[string]interface{})
	cmd, _ := m["command"].(string)
	if strings.HasPrefix(cmd, "# Shai command for:") {
		t.Fatalf("D-17 BLUFF PINNED: Shai.generate returned templated literal %q without exec (BLUFF-001).", cmd)
	}
}
