package cline

import (
	"context"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 DE-BLUFF PIN GUARD — HONEST-ERROR class (§11.4.115 / §11.4.135)
//
// HISTORY: Cline.chat/task/browserAction USED to return fabricated "sent" /
// "queued" / "executed" status maps WITHOUT doing the work — a BLUFF-001 /
// CONST-035 violation. Cline is a VS Code extension driven via the editor
// extension API + webview; it ships NO standalone headless chat/task/browser
// CLI, so the correct de-bluff is an HONEST error — never a fabricated status.
// (The "open" command, which really exec-s VS Code, is unchanged and honest.)
//
// §1.1 paired mutation: reinstate any fabricated return (e.g. chat returning a
// {"status":"sent"} map with nil error) → TestD17_Cline_NoFabrication FAILs.
// ---------------------------------------------------------------------------

func TestD17_Cline_NoFabrication(t *testing.T) {
	c := New()
	ctx := context.Background()

	cases := []struct {
		cmd    string
		params map[string]interface{}
	}{
		{"chat", map[string]interface{}{"message": "Hello"}},
		{"task", map[string]interface{}{"task": "do something"}},
		{"browser", map[string]interface{}{"action": "click", "url": "http://x"}},
	}

	for _, tc := range cases {
		res, err := c.Execute(ctx, tc.cmd, tc.params)
		if err == nil {
			t.Fatalf("D17 BLUFF: Cline.%s returned success — must return an honest error (no headless CLI), never a fabricated status. result=%v", tc.cmd, res)
		}
		if !strings.Contains(err.Error(), "no headless CLI") {
			t.Fatalf("D17: Cline.%s error %q must explain there is no headless CLI.", tc.cmd, err.Error())
		}
		// None of the former fabricated status strings may appear.
		if res != nil {
			if m, ok := res.(map[string]interface{}); ok {
				for _, v := range m {
					if s, ok := v.(string); ok {
						for _, banned := range []string{"sent", "queued", "executed"} {
							if s == banned {
								t.Fatalf("D17 REGRESSION: Cline.%s produced fabricated status %q (BLUFF-003 reintroduced).", tc.cmd, banned)
							}
						}
					}
				}
			}
		}
	}
}

// TestD17_Cline_IsStubBluff is the §11.4.115 RED polarity (RED_MODE=1).
func TestD17_Cline_IsStubBluff(t *testing.T) {
	if os.Getenv("RED_MODE") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with RED_MODE=1. " +
			"Standing GREEN guard: TestD17_Cline_NoFabrication.")
	}
	c := New()
	ctx := context.Background()
	res, err := c.task(ctx, map[string]interface{}{"task": "x"})
	if err != nil {
		return // FIXED artifact: honest error.
	}
	m, _ := res.(map[string]interface{})
	status, _ := m["status"].(string)
	t.Fatalf("D17 BLUFF PINNED: Cline.task returned status %q with no error (BLUFF-003 — Cline has no headless task CLI).", status)
}
