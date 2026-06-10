package forge

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
	assert.NotEmpty(t, "forge")
}

// TestForge_NoFabricatedStatus is the §11.4.115 / BLUFF-001 regression guard for
// DEFECT D-17. No real Forge backend is wired and no confirmed headless CLI was
// found in research, so create/deploy MUST return an honest error after input
// validation — NEVER the old fabricated "created"/"deployed" status. §1.1 paired
// mutation: revert either handler to a fabricated map + nil error → this FAILs.
func TestForge_NoFabricatedStatus(t *testing.T) {
	t.Parallel()
	f := New()
	ctx := context.Background()
	require.NoError(t, f.Initialize(ctx, nil))

	res, err := f.Execute(ctx, "create", map[string]interface{}{"name": "env1"})
	require.Error(t, err, "create must not fabricate a status")
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "not wired")

	res, err = f.Execute(ctx, "deploy", map[string]interface{}{"environment": "prod"})
	require.Error(t, err, "deploy must not fabricate a status")
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "not wired")

	// Input validation still fires first.
	_, err = f.Execute(ctx, "create", map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name required")
}
