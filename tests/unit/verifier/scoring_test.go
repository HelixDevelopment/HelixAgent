package verifier_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/superagent/superagent/internal/verifier"
)

func TestScoringService_CalculateScore(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service, err := verifier.NewScoringService(nil, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := service.CalculateScore(ctx, "gpt-4")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "gpt-4", result.ModelID)
	assert.GreaterOrEqual(t, result.OverallScore, 0.0)
	assert.LessOrEqual(t, result.OverallScore, 10.0)
	assert.NotEmpty(t, result.ScoreSuffix)
}

func TestScoringService_BatchCalculateScores(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service, err := verifier.NewScoringService(nil, cfg)
	require.NoError(t, err)

	modelIDs := []string{"gpt-4", "claude-3", "gemini-pro"}
	ctx := context.Background()

	results, err := service.BatchCalculateScores(ctx, modelIDs)

	require.NoError(t, err)
	assert.Len(t, results, 3)

	// Results should be sorted by score descending
	for i := 1; i < len(results); i++ {
		assert.GreaterOrEqual(t, results[i-1].OverallScore, results[i].OverallScore)
	}
}

func TestScoringService_UpdateWeights(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service, err := verifier.NewScoringService(nil, cfg)
	require.NoError(t, err)

	tests := []struct {
		name        string
		weights     *verifier.ScoreWeights
		expectError bool
	}{
		{
			name: "valid weights",
			weights: &verifier.ScoreWeights{
				ResponseSpeed:     0.30,
				ModelEfficiency:   0.20,
				CostEffectiveness: 0.20,
				Capability:        0.20,
				Recency:           0.10,
			},
			expectError: false,
		},
		{
			name: "weights sum less than 1",
			weights: &verifier.ScoreWeights{
				ResponseSpeed:     0.20,
				ModelEfficiency:   0.20,
				CostEffectiveness: 0.20,
				Capability:        0.20,
				Recency:           0.10,
			},
			expectError: true,
		},
		{
			name: "weights sum more than 1",
			weights: &verifier.ScoreWeights{
				ResponseSpeed:     0.30,
				ModelEfficiency:   0.30,
				CostEffectiveness: 0.30,
				Capability:        0.20,
				Recency:           0.10,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := service.UpdateWeights(tt.weights)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScoringService_GetWeights(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service, err := verifier.NewScoringService(nil, cfg)
	require.NoError(t, err)

	weights := service.GetWeights()

	assert.NotNil(t, weights)
	sum := weights.ResponseSpeed + weights.ModelEfficiency +
		weights.CostEffectiveness + weights.Capability + weights.Recency
	assert.InDelta(t, 1.0, sum, 0.01)
}

func TestScoringService_DefaultWeights(t *testing.T) {
	t.Parallel()

	weights := verifier.DefaultWeights()

	assert.Equal(t, 0.25, weights.ResponseSpeed)
	assert.Equal(t, 0.20, weights.ModelEfficiency)
	assert.Equal(t, 0.25, weights.CostEffectiveness)
	assert.Equal(t, 0.20, weights.Capability)
	assert.Equal(t, 0.10, weights.Recency)

	sum := weights.ResponseSpeed + weights.ModelEfficiency +
		weights.CostEffectiveness + weights.Capability + weights.Recency
	assert.Equal(t, 1.0, sum)
}

func TestScoringService_GetModelNameWithScore(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service, err := verifier.NewScoringService(nil, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	name, err := service.GetModelNameWithScore(ctx, "gpt-4", "GPT-4")

	require.NoError(t, err)
	assert.Contains(t, name, "GPT-4")
	assert.Contains(t, name, "(SC:")
}

func TestScoringService_InvalidateCache(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service, err := verifier.NewScoringService(nil, cfg)
	require.NoError(t, err)

	ctx := context.Background()

	// Calculate score to populate cache
	_, err = service.CalculateScore(ctx, "gpt-4")
	require.NoError(t, err)

	// Invalidate and recalculate
	service.InvalidateCache("gpt-4")

	result, err := service.CalculateScore(ctx, "gpt-4")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestScoringService_InvalidateAllCache(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service, err := verifier.NewScoringService(nil, cfg)
	require.NoError(t, err)

	ctx := context.Background()

	// Calculate multiple scores to populate cache
	models := []string{"gpt-4", "claude-3", "gemini-pro"}
	for _, model := range models {
		_, err = service.CalculateScore(ctx, model)
		require.NoError(t, err)
	}

	// Invalidate all
	service.InvalidateAllCache()

	// Recalculate
	for _, model := range models {
		result, err := service.CalculateScore(ctx, model)
		require.NoError(t, err)
		assert.NotNil(t, result)
	}
}

func TestScoringService_ScoreComponents(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	service, err := verifier.NewScoringService(nil, cfg)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := service.CalculateScore(ctx, "gpt-4")

	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Components.SpeedScore, 0.0)
	assert.LessOrEqual(t, result.Components.SpeedScore, 10.0)
	assert.GreaterOrEqual(t, result.Components.EfficiencyScore, 0.0)
	assert.LessOrEqual(t, result.Components.EfficiencyScore, 10.0)
	assert.GreaterOrEqual(t, result.Components.CostScore, 0.0)
	assert.LessOrEqual(t, result.Components.CostScore, 10.0)
	assert.GreaterOrEqual(t, result.Components.CapabilityScore, 0.0)
	assert.LessOrEqual(t, result.Components.CapabilityScore, 10.0)
	assert.GreaterOrEqual(t, result.Components.RecencyScore, 0.0)
	assert.LessOrEqual(t, result.Components.RecencyScore, 10.0)
}

func TestScoringService_CacheExpiration(t *testing.T) {
	t.Parallel()

	cfg := verifier.DefaultConfig()
	cfg.Scoring.CacheTTL = 100 * time.Millisecond
	service, err := verifier.NewScoringService(nil, cfg)
	require.NoError(t, err)

	ctx := context.Background()

	// First calculation
	result1, err := service.CalculateScore(ctx, "gpt-4")
	require.NoError(t, err)

	// Should hit cache
	result2, err := service.CalculateScore(ctx, "gpt-4")
	require.NoError(t, err)
	assert.Equal(t, result1.CalculatedAt, result2.CalculatedAt)

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Should recalculate
	result3, err := service.CalculateScore(ctx, "gpt-4")
	require.NoError(t, err)
	assert.True(t, result3.CalculatedAt.After(result1.CalculatedAt))
}
