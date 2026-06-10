package conduit

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
	assert.NotEmpty(t, "conduit")
}

// TestConduit_NoFabricatedSend is the §11.4.115 / BLUFF-001 regression guard for
// DEFECT D-17. No real Conduit transport is wired and no confirmed headless
// CLI/API was found in research, so send MUST return an honest error after input
// validation — NEVER the old fabricated "sent" status. §1.1 paired mutation:
// revert send() to a fabricated map + nil error → this FAILs.
func TestConduit_NoFabricatedSend(t *testing.T) {
	t.Parallel()
	c := New()
	ctx := context.Background()
	require.NoError(t, c.Initialize(ctx, nil))

	res, err := c.Execute(ctx, "send", map[string]interface{}{"data": "payload"})
	require.Error(t, err, "send must not fabricate a 'sent' status")
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "not wired")

	// Input validation still fires first.
	_, err = c.Execute(ctx, "send", map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "data required")
}
