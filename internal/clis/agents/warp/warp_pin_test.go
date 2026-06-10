package warp

import (
	"context"
	"testing"
)

// ---------------------------------------------------------------------------
// D-17 BLUFF-001 PIN GUARD — warp honest-error (§11.4.115 / §11.4.135)
//
// HISTORY: Warp.aiCommand/workflow USED to return templated literals ("# Warp
// AI: <desc>", "Workflow: <name>") WITHOUT running anything — BLUFF-001 /
// CONST-035 false-successes. Warp is a GUI terminal app whose AI runs inside the
// app against Warp's backend; there is no headless CLI here.
//
// FIX: aiCommand/workflow now return honest errors rather than fabricate output.
// ---------------------------------------------------------------------------

func TestD17_Warp_AICommandAndWorkflowAreHonestErrors(t *testing.T) {
	w := New()
	ctx := context.Background()

	if res, err := w.aiCommand(ctx, map[string]interface{}{"description": "list large files"}); err == nil {
		t.Fatalf("D-17 BLUFF: aiCommand returned success %v — must return an honest error, never a fabricated command (BLUFF-001).", res)
	} else if res != nil {
		t.Fatalf("D-17 BLUFF: aiCommand returned a result payload %v — must be nil.", res)
	}

	if res, err := w.workflow(ctx, map[string]interface{}{"name": "deploy"}); err == nil {
		t.Fatalf("D-17 BLUFF: workflow returned success %v — must return an honest error, never a fabricated workflow (BLUFF-001).", res)
	} else if res != nil {
		t.Fatalf("D-17 BLUFF: workflow returned a result payload %v — must be nil.", res)
	}
}
