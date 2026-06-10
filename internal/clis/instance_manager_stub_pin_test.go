package clis

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-9 STUB-BLUFF PIN GUARDS — RECONCILED TO GREEN POLARITY (§11.4.115 / §11.4.120)
//
// HISTORY (the bluff these guards catch): the InstanceManager type-specific
// execute* dispatch methods (executeAider, executeClaudeCode, executeCodex,
// executeCline, executeOpenHands) USED to return a TEMPLATED LITERAL map
// ({"status":"executed","message":"<Agent> execution completed"}) WITHOUT ever
// exec-ing a real CLI agent binary (zero os/exec in instance_manager.go) — a
// stub bluff per BLUFF-003 / CONST-035: the dispatch claimed "executed" while
// running nothing.
//
// FIX (SP4): each execute* method now resolves its agent's real CLI binary
// (resolveAgentBinary) and exec's it with the documented non-interactive flags
// (§11.4.99) via exec.CommandContext (runCLIAgent). Absent binary → honest
// error, never fake-success.
//
// These guards prove REAL exec is wired by injecting a FAKE binary per agent on
// PATH (HELIX_AGENT_BIN_<TYPE> override) whose stdout is an unforgeable marker,
// then asserting (a) the dispatch result carries the fake binary's REAL stdout
// (proves exec ran) and (b) the old "<Agent> execution completed" literal is
// NEVER returned (FAILs if reverted to the stub). GREEN by default; the standing
// regression guard for D-9 (§11.4.135).
// ---------------------------------------------------------------------------

// resultMessage extracts the "message" field from an execute* dispatch result.
func resultMessage(t *testing.T, res interface{}) string {
	t.Helper()
	m, ok := res.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string dispatch result, got %T", res)
	}
	return m["message"]
}

// writeFakeAgentBin creates an executable shell script echoing the marker plus
// the args it was invoked with, and returns its absolute path. Skips on Windows.
func writeFakeAgentBin(t *testing.T, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SKIP-OK: ATM-SP2-D9 — fake-binary injection uses a POSIX shell script; " +
			"not portable to Windows. The real-exec code path is identical across platforms.")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-agent")
	script := "#!/bin/sh\nprintf '" + marker + ":%s' \"$*\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake agent binary: %v", err)
	}
	return bin
}

func TestD9_InstanceManager_ExecuteExecsRealBinary(t *testing.T) {
	const marker = "FAKE_AGENT_RAN_7b1e"
	bin := writeFakeAgentBin(t, marker)

	mgr := &InstanceManager{}

	cases := []struct {
		name   string
		typ    CLIAgentType
		envKey string
		fn     func(*AgentInstance, interface{}) (interface{}, error)
	}{
		{"aider", TypeAider, "HELIX_AGENT_BIN_AIDER", mgr.executeAider},
		{"claude_code", TypeClaudeCode, "HELIX_AGENT_BIN_CLAUDE_CODE", mgr.executeClaudeCode},
		{"codex", TypeCodex, "HELIX_AGENT_BIN_CODEX", mgr.executeCodex},
		{"cline", TypeCline, "HELIX_AGENT_BIN_CLINE", mgr.executeCline},
		{"openhands", TypeOpenHands, "HELIX_AGENT_BIN_OPENHANDS", mgr.executeOpenHands},
	}

	bluffLiterals := []string{
		"Aider execution completed", "Claude Code execution completed",
		"Codex execution completed", "Cline execution completed",
		"OpenHands execution completed",
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.envKey, bin)
			inst := &AgentInstance{ID: "pin-" + tc.name, Type: tc.typ}
			res, err := tc.fn(inst, map[string]interface{}{"prompt": "print exactly: 42"})
			if err != nil {
				t.Fatalf("dispatch returned error with fake binary injected: %v", err)
			}
			msg := resultMessage(t, res)

			// (a) The dispatch must surface the fake binary's REAL stdout marker.
			if !strings.Contains(msg, marker) {
				t.Fatalf("D9 REGRESSION: %s dispatch did NOT exec the agent binary — "+
					"its stdout marker %q is absent from %q (BLUFF-003 reintroduced?).", tc.name, marker, msg)
			}
			// (b) None of the templated "<Agent> execution completed" literals.
			for _, lit := range bluffLiterals {
				if msg == lit {
					t.Fatalf("D9 REGRESSION: %s dispatch returned the templated literal %q "+
						"instead of real process output (BLUFF-003 reintroduced).", tc.name, msg)
				}
			}
			// (c) The prompt was forwarded to the binary.
			if !strings.Contains(msg, "print exactly: 42") {
				t.Fatalf("D9 REGRESSION: %s dispatch did not forward the prompt to the binary (got %q).", tc.name, msg)
			}
		})
	}
}

// TestD9_InstanceManager_AbsentBinaryIsHonestError proves that with NO agent
// binary available each dispatch returns an honest error (never fake-success).
func TestD9_InstanceManager_AbsentBinaryIsHonestError(t *testing.T) {
	mgr := &InstanceManager{}
	missing := filepath.Join(t.TempDir(), "does-not-exist-agent")

	cases := []struct {
		name   string
		typ    CLIAgentType
		envKey string
		fn     func(*AgentInstance, interface{}) (interface{}, error)
	}{
		{"aider", TypeAider, "HELIX_AGENT_BIN_AIDER", mgr.executeAider},
		{"claude_code", TypeClaudeCode, "HELIX_AGENT_BIN_CLAUDE_CODE", mgr.executeClaudeCode},
		{"codex", TypeCodex, "HELIX_AGENT_BIN_CODEX", mgr.executeCodex},
		{"cline", TypeCline, "HELIX_AGENT_BIN_CLINE", mgr.executeCline},
		{"openhands", TypeOpenHands, "HELIX_AGENT_BIN_OPENHANDS", mgr.executeOpenHands},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.envKey, missing)
			inst := &AgentInstance{ID: "pin-absent-" + tc.name, Type: tc.typ}
			if _, err := tc.fn(inst, map[string]interface{}{"prompt": "x"}); err == nil {
				t.Fatalf("D9 BLUFF: %s dispatch returned success with NO agent binary available — "+
					"must return an honest error, never a fabricated template.", tc.name)
			}
		})
	}
}

// TestD9_InstanceManager_ExecuteStubsAreBluffs is the §11.4.115 RED-polarity
// reproduction of the historical bluff, runnable ONLY under PIN_STUB_BLUFF=1.
// On the pre-fix artifact it FAILed (the stub literal IS returned); on the fixed
// artifact it is skipped (the standing GREEN guard above is the regression guard).
func TestD9_InstanceManager_ExecuteStubsAreBluffs(t *testing.T) {
	if os.Getenv("PIN_STUB_BLUFF") != "1" {
		t.Skip("SKIP-OK: ATM-SP2-D9 — §11.4.115 RED-on-broken-artifact reproduction; " +
			"runs only with PIN_STUB_BLUFF=1. The standing GREEN guard is " +
			"TestD9_InstanceManager_ExecuteExecsRealBinary.")
	}

	mgr := &InstanceManager{}

	cases := []struct {
		name string
		typ  CLIAgentType
		fn   func(*AgentInstance, interface{}) (interface{}, error)
	}{
		{"aider", TypeAider, mgr.executeAider},
		{"claude_code", TypeClaudeCode, mgr.executeClaudeCode},
		{"codex", TypeCodex, mgr.executeCodex},
		{"cline", TypeCline, mgr.executeCline},
		{"openhands", TypeOpenHands, mgr.executeOpenHands},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inst := &AgentInstance{ID: "pin-" + tc.name, Type: tc.typ}
			// On the FIXED artifact (no binary on PATH) the dispatch returns an
			// honest error → bluff gone. On the pre-fix STUB it returned the literal.
			res, err := tc.fn(inst, map[string]interface{}{"prompt": "print exactly: 42"})
			if err != nil {
				return
			}
			msg := resultMessage(t, res)
			if msg == "Aider execution completed" ||
				msg == "Claude Code execution completed" || msg == "Codex execution completed" ||
				msg == "Cline execution completed" || msg == "OpenHands execution completed" {
				t.Fatalf("D9 BLUFF PINNED: %s dispatch returned the templated literal %q without "+
					"exec-ing a real binary (BLUFF-003).", tc.name, msg)
			}
		})
	}
}
