package vtcode

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 PIN GUARD — vtcode real-exec wiring (§11.4.115 / §11.4.135)
//
// HISTORY: VTCode USED to expose a fabricated "transcribe" command returning a
// hardcoded "// Voice transcribed code" literal WITHOUT exec-ing anything (and
// mis-modelled vtcode as a voice-to-code tool — it is a terminal coding agent).
// That was a BLUFF-001 / CONST-035 false-success.
//
// FIX: the command is now "ask", which exec-s the real `vtcode ask "<prompt>"`
// CLI (stdout reply) via exec.LookPath + exec.CommandContext
// (resolveVtcodeBinary). When the binary is absent it returns an HONEST error.
// ---------------------------------------------------------------------------

func writeFakeVtcode(t *testing.T, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: D-17 — POSIX shell-script fake binary; real-exec path is cross-platform.")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-vtcode")
	// vtcode ask <prompt>: arg 1 is "ask", remaining args are the prompt. Echo
	// marker + all args to stdout (the `ask` reply stream), metadata to stderr.
	script := "#!/bin/sh\nprintf '" + marker + ":%s' \"$*\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake vtcode: %v", err)
	}
	return bin
}

func TestD17_VTCode_AskExecsRealBinary(t *testing.T) {
	const marker = "FAKE_VTCODE_RAN_9d40"
	bin := writeFakeVtcode(t, marker)
	t.Setenv("VTCODE_BIN", bin)

	v := New()
	ctx := context.Background()

	res, err := v.ask(ctx, map[string]interface{}{"prompt": "explain Rc vs Arc"})
	if err != nil {
		t.Fatalf("ask returned error with fake binary injected: %v", err)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}
	reply, _ := m["reply"].(string)
	if !strings.Contains(reply, marker) {
		t.Fatalf("D-17 REGRESSION: VTCode.ask did NOT exec the vtcode binary — marker %q absent from %q (BLUFF-001?).", marker, reply)
	}
	if strings.Contains(reply, "Voice transcribed code") {
		t.Fatalf("D-17 REGRESSION: VTCode returned the fabricated transcribe literal (BLUFF-001).")
	}
	if !strings.Contains(reply, "ask") || !strings.Contains(reply, "explain Rc vs Arc") {
		t.Fatalf("D-17 REGRESSION: `ask <prompt>` was not forwarded to the vtcode binary (got %q).", reply)
	}
}

func TestD17_VTCode_AbsentBinaryIsHonestError(t *testing.T) {
	t.Setenv("VTCODE_BIN", filepath.Join(t.TempDir(), "does-not-exist-vtcode"))

	v := New()
	ctx := context.Background()

	if res, err := v.ask(ctx, map[string]interface{}{"prompt": "x"}); err == nil {
		t.Fatalf("D-17 BLUFF: ask returned success %v with NO vtcode binary — must be an honest error.", res)
	}
	if v.IsAvailable() {
		t.Fatal("D-17 BLUFF: IsAvailable() = true with NO vtcode binary — must reflect real PATH state.")
	}
}

func TestD17_VTCode_NoFabricatedTranscribe(t *testing.T) {
	if os.Getenv("PIN_STUB_BLUFF") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with PIN_STUB_BLUFF=1.")
	}
	// The fabricated "transcribe" command must no longer be dispatchable.
	v := New()
	ctx := context.Background()
	if _, err := v.Execute(ctx, "transcribe", map[string]interface{}{"audio": "x"}); err == nil {
		t.Fatal("D-17 BLUFF PINNED: fabricated `transcribe` command still dispatchable (BLUFF-001).")
	}
}
