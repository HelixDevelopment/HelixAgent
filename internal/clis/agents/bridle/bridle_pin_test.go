package bridle

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// D-18 (BLUFF-001) pin guard for bridle.runWorkflow.
//
// RED-on-broken-artifact + polarity-switch guard per §11.4.115. When
// RED_MODE=1 (the historical-defect REPRODUCTION branch) the test asserts the
// fabricated `"status":"completed"` that runWorkflow returned for both the
// per-step result and the overall workflow WITHOUT ever running any step's
// action — runWorkflow only evaluates guardrails per step; the `Action` field
// is never dispatched. Claiming "completed" tells the caller the workflow ran
// when no action executed: a §11.4 PASS-bluff. When RED_MODE=0 (the standing
// GREEN regression guard) the test asserts the honest state: a guardrail-
// clean step is "evaluated" (guardrails checked, no action run) and a step
// with a blocking violation in strict mode is "blocked". The overall workflow
// is "evaluated" when nothing blocked, "blocked" when any step blocked. The
// fabricated "completed" literal is gone.

const redMode = "RED_MODE"

func isRedMode() bool { return os.Getenv(redMode) == "1" }

func newBridleWithWorkflow(t *testing.T) (*Bridle, context.Context) {
	t.Helper()
	b := New()
	ctx := context.Background()
	require.NoError(t, b.Initialize(ctx, &Config{WorkspaceDir: t.TempDir()}))

	// Inject a workflow with one guardrail-free step so the honest terminal
	// state is "evaluated" (not blocked).
	b.workflows = []Workflow{
		{
			ID:   "wf-1",
			Name: "demo",
			Steps: []Step{
				{ID: "step-1", Name: "do-thing", Action: "noop"},
			},
			Status: "created",
		},
	}
	return b, ctx
}

func TestPin_RunWorkflow_NotFabricatedCompleted(t *testing.T) {
	b, ctx := newBridleWithWorkflow(t)

	res, err := b.Execute(ctx, "run_workflow", map[string]interface{}{
		"workflow_id": "wf-1",
	})
	require.NoError(t, err)
	m, ok := res.(map[string]interface{})
	require.True(t, ok, "run_workflow must return a map result")

	results, ok := m["results"].([]map[string]interface{})
	require.True(t, ok && len(results) == 1, "run_workflow must return per-step results")

	if isRedMode() {
		// Reproduce the defect on the pre-fix artifact: both the per-step and
		// the overall workflow status were fabricated "completed" with no
		// action dispatched.
		assert.Equal(t, "completed", m["status"],
			"RED: pre-fix runWorkflow fabricates workflow status=completed without running any step action")
		assert.Equal(t, "completed", results[0]["status"],
			"RED: pre-fix runWorkflow fabricates per-step status=completed without running the step action")
		return
	}

	// GREEN: runWorkflow only EVALUATES guardrails; no action executes. The
	// honest per-step state for a guardrail-clean step is "evaluated", and the
	// overall workflow (nothing blocked) is "evaluated" — never a fabricated
	// "completed".
	assert.Equal(t, "evaluated", results[0]["status"],
		"GREEN: a guardrail-clean step must honestly report 'evaluated' (no action was run)")
	assert.Equal(t, "evaluated", m["status"],
		"GREEN: a workflow with nothing blocked must honestly report 'evaluated', never a fabricated 'completed'")
	assert.NotEqual(t, "completed", m["status"],
		"GREEN: the fabricated 'completed' status must be gone (no action executed)")
}

// TestPin_RunWorkflow_BlockedStillHonest asserts the strict-mode blocking path
// is preserved and remains honest (never "completed").
func TestPin_RunWorkflow_BlockedStillHonest(t *testing.T) {
	if isRedMode() {
		t.Skip("blocked-path honesty only meaningful post-fix")
	}
	b := New()
	ctx := context.Background()
	require.NoError(t, b.Initialize(ctx, &Config{WorkspaceDir: t.TempDir(), StrictMode: true}))

	b.guardrails = []Guardrail{
		{ID: "g-block", Name: "Blocker", Type: "safety", Action: "block", Enabled: true},
	}
	b.workflows = []Workflow{
		{
			ID:   "wf-block",
			Name: "blocked-demo",
			Steps: []Step{
				{ID: "step-1", Name: "risky", Action: "noop", Guardrails: []string{"g-block"}},
			},
			Status: "created",
		},
	}

	res, err := b.Execute(ctx, "run_workflow", map[string]interface{}{"workflow_id": "wf-block"})
	require.NoError(t, err)
	m := res.(map[string]interface{})
	results := m["results"].([]map[string]interface{})

	assert.Equal(t, "blocked", results[0]["status"],
		"GREEN: a step with a blocking guardrail violation in strict mode must be 'blocked'")
	assert.Equal(t, "blocked", m["status"],
		"GREEN: a workflow with a blocked step must honestly report 'blocked', never 'completed'")
}
