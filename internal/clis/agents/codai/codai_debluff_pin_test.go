package codai

import (
	"context"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 DE-BLUFF PIN GUARD — HONEST-ERROR class (§11.4.115 / §11.4.135)
//
// HISTORY: Codai.review/analyze USED to return hardcoded "Code review by Codai"
// / "Code analysis by Codai" strings plus invented metrics WITHOUT running
// anything (BLUFF-001 / CONST-035). Codai has no confirmed standalone headless
// CLI this integration can exec, so the correct de-bluff is an HONEST error —
// never a fabricated review/analysis. IsAvailable() now honestly reports false.
//
// §1.1 paired mutation: reinstate a fabricated return (e.g. review returning
// {"review":"Code review by Codai"} with nil error) → TestD17_Codai_NoFabrication
// FAILs.
// ---------------------------------------------------------------------------

func TestD17_Codai_NoFabrication(t *testing.T) {
	c := New()
	ctx := context.Background()

	cases := []struct {
		cmd    string
		params map[string]interface{}
		banned string
	}{
		{"review", map[string]interface{}{"code": "func main(){}"}, "Code review by Codai"},
		{"analyze", map[string]interface{}{"file": "main.go"}, "Code analysis by Codai"},
	}

	for _, tc := range cases {
		res, err := c.Execute(ctx, tc.cmd, tc.params)
		if err == nil {
			t.Fatalf("D17 BLUFF: Codai.%s returned success — must return an honest error (no headless CLI), never a fabricated result. result=%v", tc.cmd, res)
		}
		if !strings.Contains(err.Error(), "no confirmed headless CLI") {
			t.Fatalf("D17: Codai.%s error %q must explain there is no confirmed headless CLI.", tc.cmd, err.Error())
		}
		if res != nil {
			if m, ok := res.(map[string]interface{}); ok {
				for _, v := range m {
					if s, ok := v.(string); ok && strings.Contains(s, tc.banned) {
						t.Fatalf("D17 REGRESSION: Codai.%s produced fabricated literal %q (BLUFF-001 reintroduced).", tc.cmd, tc.banned)
					}
				}
			}
		}
	}

	// IsAvailable must be honest (false) — Codai cannot really run.
	if c.IsAvailable() {
		t.Fatal("D17 BLUFF: Codai.IsAvailable() returned true while no real CLI exists — must be false.")
	}
}

// TestD17_Codai_IsStubBluff is the §11.4.115 RED polarity (RED_MODE=1).
func TestD17_Codai_IsStubBluff(t *testing.T) {
	if os.Getenv("RED_MODE") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with RED_MODE=1. " +
			"Standing GREEN guard: TestD17_Codai_NoFabrication.")
	}
	c := New()
	ctx := context.Background()
	res, err := c.review(ctx, map[string]interface{}{"code": "x"})
	if err != nil {
		return // FIXED artifact: honest error.
	}
	m, _ := res.(map[string]interface{})
	review, _ := m["review"].(string)
	t.Fatalf("D17 BLUFF PINNED: Codai.review returned %q with no error (BLUFF-001 — Codai has no headless CLI).", review)
}
