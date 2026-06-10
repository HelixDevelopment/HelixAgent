package gptme

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 STUB-BLUFF PIN GUARD — RED-on-broken-artifact + GREEN regression guard
// (§11.4.115 polarity switch / §11.4.135 standing guard)
//
// HISTORY (the bluff this guard catches): GPTMe.ask USED to return the
// FABRICATED literal "GPTMe: <question>" and GPTMe.shell the FABRICATED literal
// "Executed: <command>" WITHOUT exec-ing anything (zero os/exec in gptme.go) —
// stub bluffs per BLUFF-001 (ask) and BLUFF-003 (shell) / CONST-035.
//
// FIX (D-17): ask now execs the real `gptme --non-interactive` CLI (honest error
// when absent); shell now runs the command via real os/exec (`sh -c`) and
// surfaces the real exit code + combined output.
// ---------------------------------------------------------------------------

// writeFakeGptme creates an executable shell script that echoes the marker plus
// its args, then returns its absolute path. Skips on non-POSIX hosts.
func writeFakeGptme(t *testing.T, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: D-17 — fake-binary injection uses a POSIX shell script; not portable to Windows.")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-gptme")
	script := "#!/bin/sh\nprintf '" + marker + ":%s' \"$*\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gptme: %v", err)
	}
	return bin
}

// TestD17_GPTMe_AskExecsRealBinary — standing GREEN regression guard for ask.
func TestD17_GPTMe_AskExecsRealBinary(t *testing.T) {
	const marker = "FAKE_GPTME_RAN_91be"
	bin := writeFakeGptme(t, marker)
	t.Setenv("GPTME_BIN", bin)

	g := New()
	ctx := context.Background()

	res, err := g.ask(ctx, map[string]interface{}{"question": "what is 2+2?"})
	if err != nil {
		t.Fatalf("ask returned error with fake binary injected: %v", err)
	}
	m, _ := res.(map[string]interface{})
	answer, _ := m["answer"].(string)
	if !strings.Contains(answer, marker) {
		t.Fatalf("D17 REGRESSION: GPTMe.ask did NOT exec the gptme binary — marker %q absent from %q (BLUFF-001?).", marker, answer)
	}
	if strings.HasPrefix(answer, "GPTMe: ") {
		t.Fatalf("D17 REGRESSION: GPTMe.ask returned the echo template %q instead of a real model answer.", answer)
	}
	if !strings.Contains(answer, "what is 2+2?") {
		t.Fatalf("D17 REGRESSION: question was not forwarded to the gptme binary (got %q).", answer)
	}
}

// TestD17_GPTMe_AskAbsentBinaryIsHonestError — absent binary ⇒ honest error.
func TestD17_GPTMe_AskAbsentBinaryIsHonestError(t *testing.T) {
	t.Setenv("GPTME_BIN", filepath.Join(t.TempDir(), "does-not-exist-gptme"))
	g := New()
	if _, err := g.ask(context.Background(), map[string]interface{}{"question": "x"}); err == nil {
		t.Fatal("D17 BLUFF: ask returned success with NO gptme binary available — must be an honest error.")
	}
}

// TestD17_GPTMe_ShellRunsRealCommand — shell must run the REAL command and
// surface its REAL stdout + exit code, never the "Executed: <cmd>" template.
func TestD17_GPTMe_ShellRunsRealCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: D-17 — shell test uses POSIX `sh -c echo`; not portable to Windows.")
	}
	g := New()
	ctx := context.Background()

	// A real, unforgeable token printed by the actual shell.
	const token = "REAL_SHELL_TOKEN_4d7f"
	res, err := g.shell(ctx, map[string]interface{}{"command": "echo " + token})
	if err != nil {
		t.Fatalf("shell returned error: %v", err)
	}
	m, _ := res.(map[string]interface{})
	out, _ := m["output"].(string)
	if !strings.Contains(out, token) {
		t.Fatalf("D17 REGRESSION: GPTMe.shell did NOT run the real command — token %q absent from output %q (BLUFF-003?).", token, out)
	}
	if strings.HasPrefix(out, "Executed: ") {
		t.Fatalf("D17 REGRESSION: GPTMe.shell returned the fabricated template %q instead of real output.", out)
	}
	if ec, _ := m["exit_code"].(int); ec != 0 {
		t.Fatalf("expected exit_code 0 for a successful echo, got %d", ec)
	}

	// A failing command must surface a real non-zero exit code (real behaviour).
	fres, err := g.shell(ctx, map[string]interface{}{"command": "exit 7"})
	if err != nil {
		t.Fatalf("shell (exit 7) returned launch error: %v", err)
	}
	fm, _ := fres.(map[string]interface{})
	if ec, _ := fm["exit_code"].(int); ec != 7 {
		t.Fatalf("D17 REGRESSION: GPTMe.shell did not surface the real exit code — expected 7, got %d.", ec)
	}
}

// TestD17_GPTMe_ShellIsStubBluff — §11.4.115 RED-on-broken-artifact, RED_MODE=1.
func TestD17_GPTMe_ShellIsStubBluff(t *testing.T) {
	if os.Getenv("RED_MODE") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with RED_MODE=1. " +
			"The standing GREEN guard is TestD17_GPTMe_ShellRunsRealCommand.")
	}
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: D-17 — uses POSIX `sh -c`; not portable to Windows.")
	}
	g := New()
	res, err := g.shell(context.Background(), map[string]interface{}{"command": "echo REAL_SHELL_TOKEN_4d7f"})
	if err != nil {
		return
	}
	m, _ := res.(map[string]interface{})
	out, _ := m["output"].(string)
	if strings.HasPrefix(out, "Executed: ") {
		t.Fatalf("D17 BLUFF PINNED: GPTMe.shell returned the fabricated template %q without running the real command (BLUFF-003).", out)
	}
}
