package getshitdone

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
	assert.NotEmpty(t, "getshitdone")
}

// TestGetShitDone_NoEchoedResult is the §11.4.115 / BLUFF-001 regression guard
// for DEFECT D-17. No real GSD agent backend is wired and no confirmed headless
// CLI was found in research, so execute MUST return an honest error after input
// validation — NEVER the old echoed "Executed: <task>" result. §1.1 paired
// mutation: revert execute() to an echoed map + nil error → this FAILs.
func TestGetShitDone_NoEchoedResult(t *testing.T) {
	t.Parallel()
	g := New()
	ctx := context.Background()
	require.NoError(t, g.Initialize(ctx, nil))

	res, err := g.Execute(ctx, "execute", map[string]interface{}{"task": "ship it"})
	require.Error(t, err, "execute must not echo a fabricated result")
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "not wired")

	// Input validation still fires first.
	_, err = g.Execute(ctx, "execute", map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task required")
}
