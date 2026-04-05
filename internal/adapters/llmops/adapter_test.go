package llmops_test

import (
	"context"
	"testing"

	llmopsadapter "dev.helix.agent/internal/adapters/llmops"
	llmopsmod "digital.vasic.llmops/llmops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_NotNil(t *testing.T) {
	t.Parallel()
	adapter := llmopsadapter.New(nil)
	require.NotNil(t, adapter)
}

func TestAdapter_NewEvaluator_NotNil(t *testing.T) {
	t.Parallel()
	adapter := llmopsadapter.New(nil)
	evaluator := adapter.NewEvaluator()
	require.NotNil(t, evaluator)
}

func TestAdapter_NewExperimentManager_NotNil(t *testing.T) {
	t.Parallel()
	adapter := llmopsadapter.New(nil)
	mgr := adapter.NewExperimentManager()
	require.NotNil(t, mgr)
}

func TestAdapter_CreateDataset(t *testing.T) {
	t.Parallel()
	adapter := llmopsadapter.New(nil)
	evaluator := adapter.NewEvaluator()
	ctx := context.Background()

	ds, err := adapter.CreateDataset(ctx, evaluator, "test-dataset", llmopsmod.DatasetTypeGolden)
	require.NoError(t, err)
	assert.NotNil(t, ds)
	assert.Equal(t, "test-dataset", ds.Name)
}
