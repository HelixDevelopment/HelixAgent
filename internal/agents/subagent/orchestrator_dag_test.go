package subagent

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redMode reports whether the §11.4.115 RED polarity switch is engaged.
// RED_MODE=1 → reproduce-and-assert-defect-present on the pre-fix artifact
// (opt-in; captured as evidence in qa-results/g2-streamD-evidence.txt). Default
// (unset) / RED_MODE=0 → the standing GREEN regression-guard asserting the
// defect is ABSENT on the fixed (dev.helix.dag-backed) artifact, so the
// committed suite stays green per §11.4.135.
func redMode() bool {
	return os.Getenv("RED_MODE") == "1"
}

// TestOrchestrator_ExecutePlan_UnknownDependency is the §11.4.115 RED-on-broken-
// artifact + polarity-switch regression guard for the dev.helix.dag integration.
//
// DEFECT (pre-fix): the hand-rolled scheduler in ExecutePlan ran an outer
//
//	for len(completedSteps) < len(plan.Steps) { ... find ready steps ... }
//
// busy-loop. A plan whose step declares a DependsOn naming a step that does
// NOT exist in the plan can NEVER become ready and NEVER completes — so the
// outer loop spins forever, burning a CPU with no progress and never
// returning an error to the caller (an infinite loop = §11.4 PASS-bluff at
// the orchestration layer: the API silently never returns).
//
// FIX: ExecutePlan delegates to dev.helix.dag's dag.Build, which validates the
// graph up front and returns "depends on unknown node" as a clean error.
//
// RED_MODE=1 (default): reproduce the hang on the pre-fix artifact — the call
// does NOT return within a generous deadline (captured as the defect proof).
// RED_MODE=0: the standing guard — the call returns a non-nil error promptly.
func TestOrchestrator_ExecutePlan_UnknownDependency(t *testing.T) {
	t.Parallel()
	manager := NewManager(nil)
	orchestrator := NewOrchestrator(manager)
	ctx := context.Background()

	session, err := orchestrator.CreateSession(ctx)
	require.NoError(t, err)

	// "step2" depends on "missing" — a step that is not in the plan.
	plan := OrchestrationPlan{
		Name: "unknown-dependency-plan",
		Steps: []OrchestrationStep{
			{Name: "step1", AgentType: ExploreAgent, Description: "First", DependsOn: []string{}},
			{Name: "step2", AgentType: PlanAgent, Description: "Second", DependsOn: []string{"missing"}},
		},
	}

	type outcome struct {
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		done <- outcome{err: orchestrator.ExecutePlan(ctx, session.ID, plan)}
	}()

	if redMode() {
		// RED: prove the pre-fix scheduler HANGS — no return within the deadline.
		select {
		case res := <-done:
			t.Fatalf("RED expectation failed: ExecutePlan returned (err=%v) on the "+
				"pre-fix artifact, but the busy-spin defect should hang. If this "+
				"fails, the defect is already fixed — flip RED_MODE=0.", res.err)
		case <-time.After(2 * time.Second):
			// Hang reproduced: the defect is genuinely present on this artifact.
		}
		return
	}

	// GREEN: the fixed (dag-backed) ExecutePlan returns a clean error promptly.
	select {
	case res := <-done:
		require.Error(t, res.err, "GREEN: unknown-dependency plan must return an error, not hang")
		assert.Contains(t, res.err.Error(), "unknown",
			"error should identify the unknown dependency")
	case <-time.After(5 * time.Second):
		t.Fatal("GREEN failed: ExecutePlan still hangs on an unknown-dependency plan — " +
			"the dev.helix.dag integration did not take")
	}

	// Session must reflect failure, not be stuck running.
	s, err := orchestrator.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, SessionStatusFailed, s.Status)
}
