package uiuxpromax

import (
	"context"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 PIN GUARD — uiuxpromax honest-error (§11.4.115 / §11.4.135)
//
// HISTORY: UIUXProMax.design/prototype USED to return templated literals ("UI
// design for: <prompt>", "Prototype: <name>") WITHOUT running anything —
// BLUFF-001 / CONST-035 false-successes. "UI/UX Pro Max" is a prompt-pack
// consumed by a separate LLM host, not a runnable agent with a CLI.
//
// FIX: design/prototype now return honest errors rather than fabricate output.
// ---------------------------------------------------------------------------

func TestD17_UIUXProMax_DesignAndPrototypeAreHonestErrors(t *testing.T) {
	u := New()
	ctx := context.Background()

	if res, err := u.design(ctx, map[string]interface{}{"prompt": "login screen"}); err == nil {
		t.Fatalf("D-17 BLUFF: design returned success %v — must return an honest error, never a fabricated design (BLUFF-001).", res)
	} else if res != nil {
		t.Fatalf("D-17 BLUFF: design returned a result payload %v — must be nil.", res)
	}

	if res, err := u.prototype(ctx, map[string]interface{}{"name": "onboarding"}); err == nil {
		t.Fatalf("D-17 BLUFF: prototype returned success %v — must return an honest error, never a fabricated prototype (BLUFF-001).", res)
	} else if res != nil {
		t.Fatalf("D-17 BLUFF: prototype returned a result payload %v — must be nil.", res)
	}
}
