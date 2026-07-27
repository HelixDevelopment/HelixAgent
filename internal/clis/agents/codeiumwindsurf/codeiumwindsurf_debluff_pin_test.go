package codeiumwindsurf

import (
	"context"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 DE-BLUFF PIN GUARD — HONEST-ERROR class (§11.4.115 / §11.4.135)
//
// HISTORY: CodeiumWindsurf.complete/chat/cascade USED to fabricate output —
// complete returned "// Codeium completion", chat returned
// fmt.Sprintf("Codeium: %s", message), cascade returned a templated
// "Cascade result for: <prompt>" — WITHOUT running anything (BLUFF-001 /
// CONST-035). Windsurf is an AI-native IDE; its completion/chat/Cascade run
// INSIDE the editor and there is NO headless agent CLI this integration can
// exec, so the correct de-bluff is an HONEST error — never a fabricated reply.
//
// This guard asserts every method returns an honest error AND that none of the
// old fabricated literals are ever produced. It is GREEN by default.
// §1.1 paired mutation: reinstate any fabricated return (e.g. chat returning
// "Codeium: "+message with nil error) → TestD17_CodeiumWindsurf_NoFabrication
// FAILs because err==nil / the literal reappears.
// ---------------------------------------------------------------------------

func TestD17_CodeiumWindsurf_NoFabrication(t *testing.T) {
	c := New()
	ctx := context.Background()

	cases := []struct {
		cmd    string
		params map[string]interface{}
		banned []string
	}{
		{"complete", map[string]interface{}{"prefix": "func main"}, []string{"// Codeium completion"}},
		{"chat", map[string]interface{}{"message": "Hello"}, []string{"Codeium: Hello"}},
		{"cascade", map[string]interface{}{"prompt": "Create a web app"}, []string{"Cascade result for: Create a web app"}},
	}

	for _, tc := range cases {
		res, err := c.Execute(ctx, tc.cmd, tc.params)
		if err == nil {
			t.Fatalf("D17 BLUFF: CodeiumWindsurf.%s returned success — must return an honest error (no headless CLI), never a fabricated response. result=%v", tc.cmd, res)
		}
		if !strings.Contains(err.Error(), "no headless CLI") {
			t.Fatalf("D17: CodeiumWindsurf.%s error %q must explain there is no headless CLI.", tc.cmd, err.Error())
		}
		// Defensive: even if a future change returns a non-nil result alongside
		// the error, none of the banned fabricated literals may appear.
		if res != nil {
			if m, ok := res.(map[string]interface{}); ok {
				for _, b := range tc.banned {
					for _, v := range m {
						if s, ok := v.(string); ok && strings.Contains(s, b) {
							t.Fatalf("D17 REGRESSION: CodeiumWindsurf.%s produced the fabricated literal %q (BLUFF-001 reintroduced).", tc.cmd, b)
						}
					}
				}
			}
		}
	}
}

// TestD17_CodeiumWindsurf_IsStubBluff is the §11.4.115 RED polarity (RED_MODE=1):
// on the PRE-FIX artifact chat fabricated "Codeium: Hello" with nil error → this
// FAILs; on the FIXED artifact it returns an honest error → PASSes.
func TestD17_CodeiumWindsurf_IsStubBluff(t *testing.T) {
	if os.Getenv("RED_MODE") != "1" {
		t.Skip("SKIP-OK: D-17 — §11.4.115 RED-on-broken-artifact reproduction; runs only with RED_MODE=1. " +
			"Standing GREEN guard: TestD17_CodeiumWindsurf_NoFabrication.")
	}
	c := New()
	ctx := context.Background()
	res, err := c.chat(ctx, map[string]interface{}{"message": "Hello"})
	if err != nil {
		return // FIXED artifact: honest error.
	}
	m, _ := res.(map[string]interface{})
	reply, _ := m["response"].(string)
	t.Fatalf("D17 BLUFF PINNED: CodeiumWindsurf.chat returned %q with no error (BLUFF-001 — Windsurf has no headless CLI).", reply)
}
