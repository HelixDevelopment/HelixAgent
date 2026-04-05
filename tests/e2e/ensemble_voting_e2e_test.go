package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
)

// TestEnsembleVoting_ConfidenceWeighted_E2E verifies that the
// ConfidenceWeightedStrategy selects the highest-scored response.
func TestEnsembleVoting_ConfidenceWeighted_E2E(t *testing.T) {
	strategy := &services.ConfidenceWeightedStrategy{}

	responses := []*models.LLMResponse{
		{
			Content:      "low confidence answer",
			ProviderName: "provider-a",
			Confidence:   0.3,
		},
		{
			Content:      "high confidence answer with detail",
			ProviderName: "provider-b",
			Confidence:   0.9,
		},
		{
			Content:      "medium confidence answer",
			ProviderName: "provider-c",
			Confidence:   0.6,
		},
	}

	req := &models.LLMRequest{
		Prompt: "What is the capital of France?",
	}

	winner, scores, err := strategy.Vote(responses, req)
	require.NoError(t, err)
	require.NotNil(t, winner)
	assert.NotEmpty(t, scores, "scores map should not be empty")
	assert.Equal(t, "provider-b", winner.ProviderName,
		"highest confidence provider should win")
}

// TestEnsembleVoting_MajorityVote_E2E verifies that the
// MajorityVoteStrategy selects the most common answer.
func TestEnsembleVoting_MajorityVote_E2E(t *testing.T) {
	strategy := &services.MajorityVoteStrategy{}

	// Two providers agree on "Paris", one disagrees.
	responses := []*models.LLMResponse{
		{
			Content:  "Paris",
			ProviderName: "provider-a",
		},
		{
			Content:  "Paris",
			ProviderName: "provider-b",
		},
		{
			Content:  "Lyon",
			ProviderName: "provider-c",
		},
	}

	req := &models.LLMRequest{
		Prompt: "What is the capital of France?",
	}

	winner, scores, err := strategy.Vote(responses, req)
	require.NoError(t, err)
	require.NotNil(t, winner)
	assert.NotEmpty(t, scores)
	assert.Equal(t, "Paris", winner.Content,
		"majority answer should be Paris")
}

// TestEnsembleVoting_EmptyResponses verifies that both strategies
// return an error when given an empty slice.
func TestEnsembleVoting_EmptyResponses(t *testing.T) {
	req := &models.LLMRequest{Prompt: "test"}

	t.Run("ConfidenceWeighted", func(t *testing.T) {
		strategy := &services.ConfidenceWeightedStrategy{}
		_, _, err := strategy.Vote([]*models.LLMResponse{}, req)
		assert.Error(t, err, "empty responses should error")
	})

	t.Run("MajorityVote", func(t *testing.T) {
		strategy := &services.MajorityVoteStrategy{}
		_, _, err := strategy.Vote([]*models.LLMResponse{}, req)
		assert.Error(t, err, "empty responses should error")
	})
}

// TestEnsembleVoting_SingleResponse verifies that a single response
// is returned directly without error.
func TestEnsembleVoting_SingleResponse(t *testing.T) {
	strategy := &services.ConfidenceWeightedStrategy{}

	responses := []*models.LLMResponse{
		{
			Content:    "only answer",
			ProviderName: "solo-provider",
			Confidence: 0.8,
		},
	}

	req := &models.LLMRequest{Prompt: "question"}

	winner, _, err := strategy.Vote(responses, req)
	require.NoError(t, err)
	assert.Equal(t, "only answer", winner.Content)
}
