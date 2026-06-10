package taskweaver

import (
	"context"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 PIN GUARD — taskweaver honest-error (§11.4.115 / §11.4.135)
//
// HISTORY: Taskweaver.chat/code USED to return templated literals ("Taskweaver:
// <msg>", "// Taskweaver\n// <prompt>") WITHOUT running anything — BLUFF-001 /
// CONST-035 false-successes.
//
// FIX: TaskWeaver (microsoft/TaskWeaver) is a Python agent FRAMEWORK driven
// through its Python session API, not a headless binary, so chat/code now return
// honest errors rather than fabricate output.
// ---------------------------------------------------------------------------

func TestD17_Taskweaver_ChatAndCodeAreHonestErrors(t *testing.T) {
	tw := New()
	ctx := context.Background()

	if res, err := tw.chat(ctx, map[string]interface{}{"message": "hi"}); err == nil {
		t.Fatalf("D-17 BLUFF: chat returned success %v — must return an honest error, never a fabricated reply (BLUFF-003).", res)
	} else if res != nil {
		t.Fatalf("D-17 BLUFF: chat returned a result payload %v — must be nil.", res)
	}

	if res, err := tw.code(ctx, map[string]interface{}{"prompt": "write hello"}); err == nil {
		t.Fatalf("D-17 BLUFF: code returned success %v — must return an honest error, never fabricated code (BLUFF-001).", res)
	} else if res != nil {
		t.Fatalf("D-17 BLUFF: code returned a result payload %v — must be nil.", res)
	}
}

// Input-validation contract preserved: empty params still error.
func TestD17_Taskweaver_EmptyParamsStillError(t *testing.T) {
	tw := New()
	ctx := context.Background()
	if _, err := tw.chat(ctx, map[string]interface{}{}); err == nil {
		t.Fatal("D-17: chat with empty message must error.")
	}
	if _, err := tw.code(ctx, map[string]interface{}{}); err == nil {
		t.Fatal("D-17: code with empty prompt must error.")
	}
}
