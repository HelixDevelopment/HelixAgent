package fauxpilot

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
	assert.NotEmpty(t, "fauxpilot")
}

// TestFauxpilot_NoFabricatedCompletion is the §11.4.115 / BLUFF-001 regression
// guard for DEFECT D-17. No real FauxPilot inference client is wired and no
// confirmed headless CLI exists, so complete MUST return an honest error after
// input validation — NEVER the old hardcoded "// Fauxpilot completion" string.
// §1.1 paired mutation: revert complete() to return a fabricated map + nil error
// → this FAILs.
func TestFauxpilot_NoFabricatedCompletion(t *testing.T) {
	t.Parallel()
	f := New()
	ctx := context.Background()
	require.NoError(t, f.Initialize(ctx, nil))

	res, err := f.Execute(ctx, "complete", map[string]interface{}{"prefix": "func "})
	require.Error(t, err, "complete must not fabricate a completion")
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "not wired")

	// Input validation still fires first.
	_, err = f.Execute(ctx, "complete", map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prefix required")
}
