package challenges

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/challenge"
)

func TestAPIQuality_ID(t *testing.T) {
	t.Parallel()
	c := &APIQualityChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"helix-api-quality",
			"API Quality",
			"Tests API response quality",
			"validation",
			nil,
		),
	}
	assert.Equal(t, challenge.ID("helix-api-quality"), c.ID())
}

func TestAPIQuality_NoPrompts(t *testing.T) {
	t.Parallel()
	c := &APIQualityChallenge{}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusSkipped, result.Status)
}

func TestAPIQuality_WithBaseURL(t *testing.T) {
	t.Parallel()
	c := &APIQualityChallenge{
		BaseURL: "http://localhost:8080",
		Prompts: []TestPrompt{
			{
				ID:         "greeting",
				Category:   "basic",
				Prompt:     "Hello",
				MaxLatency: 5 * time.Second,
			},
		},
	}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	assert.Len(t, result.Assertions, 1)
}

func TestAPIQuality_NoBaseURL(t *testing.T) {
	t.Parallel()
	c := &APIQualityChallenge{
		Prompts: []TestPrompt{
			{ID: "test1", Prompt: "Hello"},
		},
	}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}

func TestAPIQuality_MultiplePrompts(t *testing.T) {
	t.Parallel()
	c := &APIQualityChallenge{
		BaseURL: "http://localhost:8080",
		Prompts: []TestPrompt{
			{ID: "p1", Prompt: "Hello"},
			{ID: "p2", Prompt: "World"},
			{ID: "p3", Prompt: "Test"},
		},
	}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	assert.Len(t, result.Assertions, 3)
}

func TestAPIQuality_MixedResults_NoBaseURL(t *testing.T) {
	t.Parallel()
	c := &APIQualityChallenge{
		Prompts: []TestPrompt{
			{ID: "p1", Prompt: "Hello"},
			{ID: "p2", Prompt: "World"},
		},
	}

	result, err := c.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
	assert.Contains(t, result.Error, "2 of 2")
}
