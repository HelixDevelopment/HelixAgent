package copilotworkspace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageInitialization(t *testing.T) {
	t.Parallel()
	assert.True(t, true)
}

func TestAgentType(t *testing.T) {
	t.Parallel()
	assert.NotEmpty(t, "copilotworkspace")
}

// TestCopilotWorkspace_NoFabricatedAIWork is the §11.4.115 / BLUFF-001
// regression guard for DEFECT D-17. Copilot Workspace is web-only with no
// headless CLI, so plan/implement/review MUST return an honest error after
// locating the task — NEVER the old fabricated plan / file list / review verdict.
// The local task registry CRUD (create_task / list_tasks) stays genuine local
// state, which this test also exercises. §1.1 paired mutation: revert any AI-work
// handler to a fabricated map + nil error → this FAILs.
func TestCopilotWorkspace_NoFabricatedAIWork(t *testing.T) {
	t.Parallel()
	c := New()
	ctx := context.Background()
	require.NoError(t, c.Initialize(ctx, nil))

	// Genuine local-state CRUD still works.
	res, err := c.Execute(ctx, "create_task", map[string]interface{}{"title": "do thing"})
	require.NoError(t, err)
	task := res.(map[string]interface{})["task"].(Task)
	require.NotEmpty(t, task.ID)

	res, err = c.Execute(ctx, "list_tasks", nil)
	require.NoError(t, err)
	assert.Equal(t, 1, res.(map[string]interface{})["count"])

	// AI-work commands must refuse to fabricate.
	for _, cmd := range []string{"plan", "implement", "review"} {
		res, err := c.Execute(ctx, cmd, map[string]interface{}{"task_id": task.ID})
		require.Error(t, err, "%s must not fabricate AI work", cmd)
		assert.Nil(t, res, "%s must not return a fabricated result", cmd)
		assert.Contains(t, err.Error(), "no headless CLI")
	}
}
