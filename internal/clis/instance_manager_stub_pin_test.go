package clis

import (
	"fmt"
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// D-9 STUB-BLUFF PINNING GUARDS (left RED on purpose — pin, do NOT fix here)
//
// The InstanceManager type-specific execute* dispatch methods (executeAider,
// executeClaudeCode, executeCodex, executeCline, executeOpenHands, …) return a
// TEMPLATED LITERAL map ({"status":"executed","message":"<Agent> execution
// completed"}) WITHOUT ever exec-ing a real CLI agent binary (zero os/exec in
// instance_manager.go). This is a stub bluff per BLUFF-003 / CONST-035: the
// dispatch claims "executed" while running nothing.
//
// These tests DOCUMENT that defect. They are skipped by default so they do not
// block the standing suite; set PIN_STUB_BLUFF=1 to run them, at which point
// they FAIL on the current stub (proving the bluff is live). They become GREEN
// when SP4 wires real os/exec execution into these dispatch methods.
//
//   PIN_STUB_BLUFF=1 go test -run TestD9_InstanceManager_ExecuteStubsAreBluffs -v ./internal/clis
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

func TestD9_InstanceManager_ExecuteStubsAreBluffs(t *testing.T) {
	if os.Getenv("PIN_STUB_BLUFF") != "1" {
		t.Skip("SKIP-OK: ATM-SP2-D9 — defect-pinning guard for the InstanceManager execute* " +
			"stub bluff (BLUFF-003). Left RED on purpose; runs only with PIN_STUB_BLUFF=1. " +
			"Becomes GREEN when SP4 replaces the templated-string returns with real os/exec.")
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
			res, err := tc.fn(inst, map[string]interface{}{"prompt": "print exactly: 42"})
			if err != nil {
				t.Fatalf("dispatch returned error: %v", err)
			}
			msg := resultMessage(t, res)

			// A REAL exec dispatch would surface the invoked binary's actual
			// stdout/exit-code — NEVER a hardcoded "<Agent> execution completed"
			// templated literal. Assert the bluff is GONE. On the current stub
			// this FAILs (msg == "<Agent> execution completed"), pinning the bluff.
			bluff := fmt.Sprintf("%s execution completed", titleish(tc.name))
			if msg == bluff || msg == "Aider execution completed" ||
				msg == "Claude Code execution completed" || msg == "Codex execution completed" ||
				msg == "Cline execution completed" || msg == "OpenHands execution completed" {
				t.Fatalf("D9 BLUFF PINNED: %s dispatch returned the templated literal %q without "+
					"exec-ing a real binary (BLUFF-003). Expected real process output.", tc.name, msg)
			}
		})
	}
}

// titleish is a tiny helper used only to construct the expected bluff string.
func titleish(s string) string {
	switch s {
	case "aider":
		return "Aider"
	case "claude_code":
		return "Claude Code"
	case "codex":
		return "Codex"
	case "cline":
		return "Cline"
	case "openhands":
		return "OpenHands"
	}
	return s
}
