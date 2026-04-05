package challenges

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/challenge"
)

func TestDebateFormation_ID(t *testing.T) {
	t.Parallel()
	c := &DebateFormationChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"helix-debate-formation",
			"Debate Group Formation",
			"Validates debate group formation",
			"validation",
			[]challenge.ID{"helix-provider-verification"},
		),
	}
	assert.Equal(t, challenge.ID("helix-debate-formation"), c.ID())
}

func TestDebateFormation_Dependencies(t *testing.T) {
	t.Parallel()
	c := &DebateFormationChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"helix-debate-formation",
			"Debate Group Formation",
			"Validates debate group formation",
			"validation",
			[]challenge.ID{"helix-provider-verification"},
		),
	}
	assert.Contains(t, c.Dependencies(), challenge.ID("helix-provider-verification"))
}

func TestDebateFormation_EnoughProviders(t *testing.T) {
	t.Parallel()
	providers := []ProviderInfo{
		{Name: "claude", Verified: true, Score: 8.5},
		{Name: "deepseek", Verified: true, Score: 7.2},
		{Name: "gemini", Verified: true, Score: 6.8},
	}
	c := &DebateFormationChallenge{providers: providers}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
}

func TestDebateFormation_InsufficientProviders(t *testing.T) {
	t.Parallel()
	providers := []ProviderInfo{
		{Name: "claude", Verified: true, Score: 8.5},
		{Name: "failing", Verified: false, Score: 3.0},
	}
	c := &DebateFormationChallenge{providers: providers}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
	assert.Contains(t, result.Error, "insufficient")
}

func TestDebateFormation_CustomGroupSize(t *testing.T) {
	t.Parallel()
	providers := []ProviderInfo{
		{Name: "a", Verified: true, Score: 9.0},
		{Name: "b", Verified: true, Score: 8.0},
	}
	c := &DebateFormationChallenge{
		providers:    providers,
		MinProviders: 2,
		GroupSize:    2,
	}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
}

func TestDebateFormation_AllUnverified(t *testing.T) {
	t.Parallel()
	providers := []ProviderInfo{
		{Name: "a", Verified: false, Score: 9.0},
		{Name: "b", Verified: false, Score: 8.0},
		{Name: "c", Verified: false, Score: 7.0},
	}
	c := &DebateFormationChallenge{providers: providers}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}

func TestDebateFormation_EmptyProviders(t *testing.T) {
	t.Parallel()
	c := &DebateFormationChallenge{providers: nil}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
	assert.Contains(t, result.Error, "insufficient")
}
