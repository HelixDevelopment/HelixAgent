package challenges

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/challenge"
)

func TestProviderVerification_ID(t *testing.T) {
	t.Parallel()
	c := &ProviderVerificationChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"helix-provider-verification",
			"Provider Verification",
			"Verifies providers",
			"validation",
			nil,
		),
	}
	assert.Equal(t, challenge.ID("helix-provider-verification"), c.ID())
}

func TestProviderVerification_AllPass(t *testing.T) {
	t.Parallel()
	providers := []ProviderInfo{
		{Name: "claude", Verified: true, Score: 8.5},
		{Name: "deepseek", Verified: true, Score: 7.2},
		{Name: "gemini", Verified: true, Score: 6.8},
	}
	c := &ProviderVerificationChallenge{providers: providers}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	assert.Len(t, result.Assertions, 3)
	for _, a := range result.Assertions {
		assert.True(t, a.Passed)
	}
}

func TestProviderVerification_SomeFail(t *testing.T) {
	t.Parallel()
	providers := []ProviderInfo{
		{Name: "claude", Verified: true, Score: 8.5},
		{Name: "failing", Verified: false, Score: 3.0},
	}
	c := &ProviderVerificationChallenge{providers: providers}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
	assert.Contains(t, result.Error, "failing")
}

func TestProviderVerification_LowScore(t *testing.T) {
	t.Parallel()
	providers := []ProviderInfo{
		{Name: "low-score", Verified: true, Score: 4.0},
	}
	c := &ProviderVerificationChallenge{providers: providers}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
	assert.Contains(t, result.Error, "low-score")
}

func TestProviderVerification_Empty(t *testing.T) {
	t.Parallel()
	c := &ProviderVerificationChallenge{providers: nil}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	assert.Empty(t, result.Assertions)
}
