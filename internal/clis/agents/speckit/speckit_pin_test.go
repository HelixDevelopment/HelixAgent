package speckit

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 PIN GUARD — speckit real-exec + honest-generate (§11.4.115 / §11.4.135)
//
// HISTORY: SpecKit.generate USED to return a templated literal "# Spec for:
// <requirement>" WITHOUT exec-ing anything — a BLUFF-001 / CONST-035
// false-success (the agent claimed to generate a specification while running
// nothing).
//
// FIX: the spec-kit `specify` CLI is a PROJECT SCAFFOLDER (`specify init`), NOT a
// headless requirement->spec-text generator (spec text is produced by a separate
// coding agent via the `/specify` slash command). So:
//   * `init` exec-s the real `specify init <name> --integration <agent>` CLI
//     (resolveSpecifyBinary) and returns its REAL output;
//   * `generate` returns an HONEST error (no headless requirement->spec command)
//     rather than fabricate spec text.
// ---------------------------------------------------------------------------

func writeFakeSpecify(t *testing.T, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: D-17 — POSIX shell-script fake binary; real-exec path is cross-platform.")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-specify")
	script := "#!/bin/sh\nprintf '" + marker + ":%s' \"$*\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake specify: %v", err)
	}
	return bin
}

func TestD17_SpecKit_InitExecsRealBinary(t *testing.T) {
	const marker = "FAKE_SPECIFY_RAN_c2a8"
	bin := writeFakeSpecify(t, marker)
	t.Setenv("SPECIFY_BIN", bin)

	s := New()
	ctx := context.Background()

	res, err := s.initProject(ctx, map[string]interface{}{"name": "myproj"})
	if err != nil {
		t.Fatalf("init returned error with fake binary injected: %v", err)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}
	out, _ := m["output"].(string)
	if !strings.Contains(out, marker) {
		t.Fatalf("D-17 REGRESSION: SpecKit.init did NOT exec the specify binary — marker %q absent from %q.", marker, out)
	}
	if !strings.Contains(out, "init") || !strings.Contains(out, "myproj") {
		t.Fatalf("D-17 REGRESSION: `init <name>` args were not forwarded to specify (got %q).", out)
	}
}

// TestD17_SpecKit_GenerateIsHonestError proves generate NEVER fabricates spec
// text: with the real binary PRESENT it still returns the honest "no headless
// requirement->spec command" error (the capability is agent-driven), and that
// error is NOT the old "# Spec for:" template.
func TestD17_SpecKit_GenerateIsHonestError(t *testing.T) {
	bin := writeFakeSpecify(t, "PRESENT")
	t.Setenv("SPECIFY_BIN", bin)

	s := New()
	ctx := context.Background()

	res, err := s.generate(ctx, map[string]interface{}{"requirement": "user login flow"})
	if err == nil {
		t.Fatalf("D-17 BLUFF: generate returned success %v — must return an honest error (no headless requirement->spec command), never a fabricated spec (BLUFF-001).", res)
	}
	if res != nil {
		t.Fatalf("D-17 BLUFF: generate returned a result payload %v — must be nil.", res)
	}
	if strings.Contains(err.Error(), "# Spec for:") {
		t.Fatalf("D-17 REGRESSION: generate error embeds the old fabricated template (BLUFF-001): %v", err)
	}
}

func TestD17_SpecKit_AbsentBinaryIsHonestError(t *testing.T) {
	t.Setenv("SPECIFY_BIN", filepath.Join(t.TempDir(), "does-not-exist-specify"))

	s := New()
	ctx := context.Background()

	if res, err := s.initProject(ctx, map[string]interface{}{"name": "x"}); err == nil {
		t.Fatalf("D-17 BLUFF: init returned success %v with NO specify binary — must be an honest error.", res)
	}
	if res, err := s.generate(ctx, map[string]interface{}{"requirement": "x"}); err == nil {
		t.Fatalf("D-17 BLUFF: generate returned success %v with NO specify binary — must be an honest error.", res)
	}
	if s.IsAvailable() {
		t.Fatal("D-17 BLUFF: IsAvailable() = true with NO specify binary — must reflect real PATH state.")
	}
}
