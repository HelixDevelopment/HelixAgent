package agentic_test

import (
	"context"
	"testing"

	agenticadapter "dev.helix.agent/internal/adapters/agentic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_NotNil(t *testing.T) {
	t.Parallel()
	adapter := agenticadapter.New(nil)
	require.NotNil(t, adapter)
}

func TestAdapter_NewWorkflow_NotNil(t *testing.T) {
	t.Parallel()
	adapter := agenticadapter.New(nil)
	wf := adapter.NewWorkflow("test-wf", "test workflow", nil)
	require.NotNil(t, wf)
}

func TestAdapter_ExecuteWorkflow_ReturnsState(t *testing.T) {
	t.Parallel()
	adapter := agenticadapter.New(nil)
	ctx := context.Background()
	state, err := adapter.ExecuteWorkflow(ctx, "test-wf", map[string]any{"key": "value"})
	require.NoError(t, err)
	require.NotNil(t, state)
}

func TestAdapter_ExecuteWorkflow_WithNilParams(t *testing.T) {
	t.Parallel()
	adapter := agenticadapter.New(nil)
	ctx := context.Background()
	state, err := adapter.ExecuteWorkflow(ctx, "test-wf", nil)
	require.NoError(t, err)
	assert.NotNil(t, state)
}
