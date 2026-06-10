package agentdeck

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// D-18 (BLUFF-001) pin guard for agentdeck.orchestrate.
//
// RED-on-broken-artifact + polarity-switch guard per §11.4.115. When
// RED_MODE=1 (the historical-defect REPRODUCTION branch) the test asserts the
// fabricated `"status":"completed"` that orchestrate returned WITHOUT ever
// running any agent — orchestrate only builds a static plan via
// createOrchestrationPlan and performs NO real execution, so claiming
// "completed" is a §11.4 PASS-bluff (it tells the caller a multi-agent run
// finished when nothing ran). When RED_MODE=0 (the standing GREEN regression
// guard) the test asserts the honest state: a plan was PLANNED, not a run
// COMPLETED. The honest status is "planned" and the fabricated "completed"
// literal is gone.

const redMode = "RED_MODE"

func isRedMode() bool { return os.Getenv(redMode) == "1" }

func TestPin_Orchestrate_NotFabricatedCompleted(t *testing.T) {
	ad := New()
	ctx := context.Background()
	require.NoError(t, ad.Initialize(ctx, &Config{}))

	res, err := ad.Execute(ctx, "orchestrate", map[string]interface{}{
		"task": "build a feature",
	})
	require.NoError(t, err)
	m, ok := res.(map[string]interface{})
	require.True(t, ok, "orchestrate must return a map result")

	if isRedMode() {
		// Reproduce the defect on the pre-fix artifact: orchestrate fabricated
		// a "completed" status with no real agent run behind it.
		assert.Equal(t, "completed", m["status"],
			"RED: pre-fix orchestrate fabricates status=completed without running any agent")
		return
	}

	// GREEN: orchestrate produces a PLAN only; no agent executes. The honest
	// status reflects that a plan was produced, NOT that a run completed.
	assert.Equal(t, "planned", m["status"],
		"GREEN: orchestrate only builds a plan — status must honestly be 'planned', never a fabricated 'completed'")
	assert.NotEqual(t, "completed", m["status"],
		"GREEN: the fabricated 'completed' status must be gone (no real run happened)")
	// The plan itself MUST still be present — the real, honest output.
	assert.NotNil(t, m["plan"], "GREEN: the produced plan must be returned as the real output")
}
