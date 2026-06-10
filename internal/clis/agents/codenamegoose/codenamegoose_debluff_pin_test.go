package codenamegoose

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 DE-BLUFF PIN GUARD (§11.4.115 RED→GREEN / §11.4.135)
//
// HISTORY: CodenameGoose.run USED to return fmt.Sprintf("Goose result: %s",
// prompt) WITHOUT exec-ing any real goose binary (zero os/exec): BLUFF-001 /
// CONST-035.
//
// FIX (D-17): run now exec-s the real Block Goose CLI via exec.LookPath +
// exec.CommandContext (resolveGooseBinary, `goose run --text "<prompt>"`).
// Absent binary → honest error, never a fabricated reply.
// ---------------------------------------------------------------------------

func writeFakeGoose(t *testing.T, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: D-17 — POSIX shell-script fake binary; real-exec path is identical across platforms.")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-goose")
	script := "#!/bin/sh\nprintf '" + marker + ":%s' \"$*\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake goose: %v", err)
	}
	return bin
}

// TestD17_Goose_RunExecsRealBinary is the standing GREEN guard.
func TestD17_Goose_RunExecsRealBinary(t *testing.T) {
	const marker = "FAKE_GOOSE_RAN_5b20"
	bin := writeFakeGoose(t, marker)
	t.Setenv("GOOSE_BIN", bin)

	g := New()
	ctx := context.Background()

	res, err := g.run(ctx, map[string]interface{}{"prompt": "summarize this repo"})
	if err != nil {
		t.Fatalf("run returned error with fake binary injected: %v", err)
	}
	m, _ := res.(map[string]interface{})
	out, _ := m["result"].(string)
	if !strings.Contains(out, marker) {
		t.Fatalf("D17 REGRESSION: Goose.run did NOT exec the goose binary — marker %q absent from %q.", marker, out)
	}
	if strings.HasPrefix(out, "Goose result: ") {
		t.Fatalf("D17 REGRESSION: Goose.run returned the templated literal %q instead of real process output.", out)
	}
	if !strings.Contains(out, "summarize this repo") {
		t.Fatalf("D17 REGRESSION: prompt not forwarded to the goose binary (got %q).", out)
	}
}

// TestD17_Goose_AbsentBinaryIsHonestError proves the absent-binary path.
func TestD17_Goose_AbsentBinaryIsHonestError(t *testing.T) {
	t.Setenv("GOOSE_BIN", filepath.Join(t.TempDir(), "does-not-exist-goose"))

	g := New()
	ctx := context.Background()

	if _, err := g.run(ctx, map[string]interface{}{"prompt": "x"}); err == nil {
		t.Fatal("D17 BLUFF: Goose.run returned success with NO goose binary available — must return an honest error.")
	}
}

// TestD17_Goose_IsStubBluff is the §11.4.115 RED polarity (RED_MODE=1). §1.1
// paired mutation: revert run() to the "Goose result: %s" template → FAILs.
func TestD17_Goose_IsStubBluff(t *testing.T) {
	if os.Getenv("RED_MODE") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with RED_MODE=1. " +
			"Standing GREEN guard: TestD17_Goose_RunExecsRealBinary.")
	}
	t.Setenv("GOOSE_BIN", filepath.Join(t.TempDir(), "does-not-exist-goose"))

	g := New()
	ctx := context.Background()

	res, err := g.run(ctx, map[string]interface{}{"prompt": "x"})
	if err != nil {
		return // FIXED artifact: honest error.
	}
	m, _ := res.(map[string]interface{})
	out, _ := m["result"].(string)
	t.Fatalf("D17 BLUFF PINNED: Goose.run returned %q without exec-ing a real goose binary (BLUFF-001).", out)
}
