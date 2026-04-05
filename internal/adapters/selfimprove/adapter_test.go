package selfimprove_test

import (
	"context"
	"testing"

	selfimproveadapter "dev.helix.agent/internal/adapters/selfimprove"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_NotNil(t *testing.T) {
	t.Parallel()
	adapter := selfimproveadapter.New(nil)
	require.NotNil(t, adapter)
}

func TestAdapter_NewRewardModel_NotNil(t *testing.T) {
	t.Parallel()
	adapter := selfimproveadapter.New(nil)
	model := adapter.NewRewardModel(nil)
	require.NotNil(t, model)
}

func TestAdapter_Train_NoError(t *testing.T) {
	t.Parallel()
	adapter := selfimproveadapter.New(nil)
	model := adapter.NewRewardModel(nil)
	ctx := context.Background()
	err := adapter.Train(ctx, model, nil)
	assert.NoError(t, err)
}
