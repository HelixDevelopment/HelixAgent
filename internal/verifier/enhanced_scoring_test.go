package verifier

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEnhancedScoringService(t *testing.T) {
	t.Parallel()
	t.Run("creates service with default weights", func(t *testing.T) {
		svc := NewEnhancedScoringService(nil)
		require.NotNil(t, svc)
		assert.NotNil(t, svc.weights)
		assert.Equal(t, 0.20, svc.weights.ResponseSpeed)
		assert.Equal(t, 0.15, svc.weights.CodeQuality)
		assert.Equal(t, 0.10, svc.weights.ReasoningScore)
	})
}

func TestDefaultEnhancedWeights(t *testing.T) {
	t.Parallel()
	weights := DefaultEnhancedWeights()

	// Verify all weights sum to 1.0
	sum := weights.ResponseSpeed + weights.ModelEfficiency +
		weights.CostEffectiveness + weights.Capability +
		weights.Recency + weights.CodeQuality + weights.ReasoningScore

	assert.InDelta(t, 1.0, sum, 0.001, "Weights should sum to 1.0")
}

func TestEnhancedScoringService_CalculateEnhancedScore(t *testing.T) {
	t.Parallel()
	svc := NewEnhancedScoringService(nil)

	t.Run("calculates score for model", func(t *testing.T) {
		t.Parallel()
		model := &UnifiedModel{
			ID:       "test-model",
			Name:     "Test Model",
			Provider: "test-provider",
			Verified: true,
			Latency:  500 * time.Millisecond,
			TestResults: map[string]bool{
				"basic_completion": true,
				"code_visibility":  true,
			},
		}

		provider := &UnifiedProvider{
			ID:       "test-provider",
			Name:     "Test Provider",
			Tier:     2,
			AuthType: AuthTypeAPIKey,
		}

		ctx := context.Background()
		result, err := svc.CalculateEnhancedScore(ctx, model, provider)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "test-model", result.ModelID)
		assert.Equal(t, "test-provider", result.Provider)
		assert.Greater(t, result.OverallScore, 0.0)
		assert.LessOrEqual(t, result.OverallScore, 10.0)
		assert.Greater(t, result.ConfidenceScore, 0.0)
	})

	t.Run("caches results", func(t *testing.T) {
		t.Parallel()
		model := &UnifiedModel{
			ID:       "cache-test-model",
			Name:     "Cache Test",
			Provider: "test-provider",
			Verified: true,
		}

		provider := &UnifiedProvider{
			ID:   "test-provider",
			Tier: 2,
		}

		ctx := context.Background()

		// First call
		result1, err := svc.CalculateEnhancedScore(ctx, model, provider)
		require.NoError(t, err)

		// Second call should return cached result
		result2, err := svc.CalculateEnhancedScore(ctx, model, provider)
		require.NoError(t, err)

		assert.Equal(t, result1.CalculatedAt, result2.CalculatedAt)
	})
}

func TestEnhancedScoringService_CalculateResponseSpeedScore(t *testing.T) {
	t.Parallel()
	svc := NewEnhancedScoringService(nil)

	tests := []struct {
		name     string
		latency  time.Duration
		expected float64
	}{
		{"very fast", 100 * time.Millisecond, 10.0},
		{"fast", 300 * time.Millisecond, 9.0},
		{"medium", 800 * time.Millisecond, 8.0},
		{"slow", 1500 * time.Millisecond, 7.0},
		{"very slow", 3000 * time.Millisecond, 6.0},
		{"extremely slow", 8000 * time.Millisecond, 5.0},
		{"timeout range", 15000 * time.Millisecond, 4.0},
		{"zero latency", 0, 7.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model := &UnifiedModel{Latency: tt.latency}
			score := svc.calculateResponseSpeedScore(model)
			assert.Equal(t, tt.expected, score)
		})
	}
}

func TestEnhancedScoringService_CalculateCodeQualityScore(t *testing.T) {
	t.Parallel()
	svc := NewEnhancedScoringService(nil)

	tests := []struct {
		name        string
		modelID     string
		testResults map[string]bool
		minExpected float64
	}{
		{"coder model", "deepseek-coder", nil, 9.0},
		{"codestral model", "codestral-latest", nil, 9.0},
		{"opus model", "claude-opus", nil, 9.0},
		{"pro model", "gemini-pro", nil, 8.0},
		{"with code test", "generic-model", map[string]bool{"code_visibility": true}, 8.0},
		{"generic model", "generic-model", nil, 6.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model := &UnifiedModel{
				ID:          tt.modelID,
				TestResults: tt.testResults,
			}
			score := svc.calculateCodeQualityScore(model)
			assert.GreaterOrEqual(t, score, tt.minExpected, "Score should be at least %f", tt.minExpected)
		})
	}
}

func TestEnhancedScoringService_CalculateReasoningScore(t *testing.T) {
	t.Parallel()
	svc := NewEnhancedScoringService(nil)

	tests := []struct {
		name        string
		modelID     string
		minExpected float64
	}{
		{"reasoner model", "deepseek-reasoner", 9.0},
		{"opus model", "claude-opus-4", 9.0},
		{"4o model", "gpt-4o", 9.0},
		{"sonnet model", "claude-sonnet", 8.0},
		{"70b model", "llama-70b", 8.0},
		{"turbo model", "gpt-turbo", 7.0},
		{"haiku model", "claude-haiku", 6.0},
		{"flash model", "gemini-flash", 6.0},
		{"generic", "some-model", 6.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model := &UnifiedModel{ID: tt.modelID}
			score := svc.calculateReasoningScore(model)
			assert.GreaterOrEqual(t, score, tt.minExpected, "Score should be at least %f", tt.minExpected)
		})
	}
}

func TestEnhancedScoringService_DetermineSpecialization(t *testing.T) {
	t.Parallel()
	svc := NewEnhancedScoringService(nil)

	tests := []struct {
		name       string
		modelID    string
		components EnhancedScoreComponents
		expected   string
	}{
		{"coder in name", "deepseek-coder", EnhancedScoreComponents{}, "code"},
		{"code in name", "codellama", EnhancedScoreComponents{}, "code"},
		{"reasoner in name", "deepseek-reasoner", EnhancedScoreComponents{}, "reasoning"},
		{"vision in name", "gpt-4-vision", EnhancedScoreComponents{}, "vision"},
		{"online in name", "sonar-online", EnhancedScoreComponents{}, "search"},
		{"embed in name", "text-embed-3", EnhancedScoreComponents{}, "embedding"},
		{"high code score", "generic", EnhancedScoreComponents{CodeQuality: 9.5}, "code"},
		{"high reasoning score", "generic", EnhancedScoreComponents{ReasoningScore: 9.5}, "reasoning"},
		{"high speed score", "generic", EnhancedScoreComponents{ResponseSpeed: 9.5}, "speed"},
		{"high cost score", "generic", EnhancedScoreComponents{CostEffectiveness: 9.5}, "economy"},
		{"generic model", "generic", EnhancedScoreComponents{}, "general"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model := &UnifiedModel{ID: tt.modelID}
			specialization := svc.determineSpecialization(model, tt.components)
			assert.Equal(t, tt.expected, specialization)
		})
	}
}

func TestEnhancedScoringService_CalculateConfidenceScore(t *testing.T) {
	t.Parallel()
	svc := NewEnhancedScoringService(nil)

	t.Run("verified model gets bonus", func(t *testing.T) {
		t.Parallel()
		verifiedModel := &UnifiedModel{Verified: true}
		unverifiedModel := &UnifiedModel{Verified: false}
		provider := &UnifiedProvider{Tier: 3}

		verifiedScore := svc.calculateConfidenceScore(verifiedModel, provider)
		unverifiedScore := svc.calculateConfidenceScore(unverifiedModel, provider)

		assert.Greater(t, verifiedScore, unverifiedScore)
	})

	t.Run("OAuth provider gets bonus", func(t *testing.T) {
		t.Parallel()
		model := &UnifiedModel{Verified: true}
		oauthProvider := &UnifiedProvider{Tier: 2, AuthType: AuthTypeOAuth}
		apiKeyProvider := &UnifiedProvider{Tier: 2, AuthType: AuthTypeAPIKey}

		oauthScore := svc.calculateConfidenceScore(model, oauthProvider)
		apiKeyScore := svc.calculateConfidenceScore(model, apiKeyProvider)

		assert.Greater(t, oauthScore, apiKeyScore)
	})

	t.Run("test results improve confidence", func(t *testing.T) {
		t.Parallel()
		modelWithTests := &UnifiedModel{
			Verified: true,
			TestResults: map[string]bool{
				"test1": true,
				"test2": true,
				"test3": true,
			},
		}
		modelWithoutTests := &UnifiedModel{Verified: true}
		provider := &UnifiedProvider{Tier: 2}

		withScore := svc.calculateConfidenceScore(modelWithTests, provider)
		withoutScore := svc.calculateConfidenceScore(modelWithoutTests, provider)

		assert.Greater(t, withScore, withoutScore)
	})

	t.Run("confidence is capped at 1.0", func(t *testing.T) {
		t.Parallel()
		model := &UnifiedModel{
			Verified: true,
			Latency:  100 * time.Millisecond,
			TestResults: map[string]bool{
				"test1": true,
				"test2": true,
				"test3": true,
			},
		}
		provider := &UnifiedProvider{Tier: 1, AuthType: AuthTypeOAuth}

		confidence := svc.calculateConfidenceScore(model, provider)
		assert.LessOrEqual(t, confidence, 1.0)
	})
}

func TestEnhancedScoringService_CalculateDiversityBonus(t *testing.T) {
	t.Parallel()
	svc := NewEnhancedScoringService(nil)

	t.Run("first model from provider gets highest bonus", func(t *testing.T) {
		t.Parallel()
		bonus := svc.calculateDiversityBonus("new-provider", "model-1")
		assert.Equal(t, 0.5, bonus)
	})

	t.Run("bonus decreases with more models", func(t *testing.T) {
		t.Parallel()
		// Simulate adding models to cache
		svc.cache["provider-a:model-1"] = &EnhancedScoringResult{}

		bonus1 := svc.calculateDiversityBonus("provider-a", "model-2")
		assert.Equal(t, 0.2, bonus1)

		svc.cache["provider-a:model-2"] = &EnhancedScoringResult{}
		bonus2 := svc.calculateDiversityBonus("provider-a", "model-3")
		assert.Equal(t, 0.1, bonus2)

		svc.cache["provider-a:model-3"] = &EnhancedScoringResult{}
		bonus3 := svc.calculateDiversityBonus("provider-a", "model-4")
		assert.Equal(t, 0.0, bonus3)
	})
}

func TestEnhancedScoringService_GetTopScoringModels(t *testing.T) {
	t.Parallel()
	svc := NewEnhancedScoringService(nil)

	// Add some cached results
	svc.cache["provider-a:model-1"] = &EnhancedScoringResult{
		ModelID:        "model-1",
		Provider:       "provider-a",
		OverallScore:   8.5,
		DiversityBonus: 0.2,
	}
	svc.cache["provider-b:model-2"] = &EnhancedScoringResult{
		ModelID:        "model-2",
		Provider:       "provider-b",
		OverallScore:   9.0,
		DiversityBonus: 0.5,
	}
	svc.cache["provider-c:model-3"] = &EnhancedScoringResult{
		ModelID:        "model-3",
		Provider:       "provider-c",
		OverallScore:   7.5,
		DiversityBonus: 0.5,
	}

	t.Run("returns top models sorted by score", func(t *testing.T) {
		t.Parallel()
		top := svc.GetTopScoringModels(2)
		require.Len(t, top, 2)
		// Model-2 (9.0 + 0.5 = 9.5) should be first
		assert.Equal(t, "model-2", top[0].ModelID)
	})

	t.Run("respects limit", func(t *testing.T) {
		t.Parallel()
		top := svc.GetTopScoringModels(1)
		require.Len(t, top, 1)
	})

	t.Run("handles limit larger than cache", func(t *testing.T) {
		t.Parallel()
		top := svc.GetTopScoringModels(100)
		assert.Len(t, top, 3)
	})
}

func TestEnhancedScoringService_SelectDebateTeamFromScores(t *testing.T) {
	t.Parallel()
	svc := NewEnhancedScoringService(nil)

	// Add diverse models
	for i := 0; i < 20; i++ {
		provider := "provider-" + string(rune('a'+i%5))
		modelID := "model-" + string(rune('0'+i))
		svc.cache[provider+":"+modelID] = &EnhancedScoringResult{
			ModelID:        modelID,
			Provider:       provider,
			OverallScore:   5.0 + float64(i%5),
			DiversityBonus: 0.1,
		}
	}

	t.Run("selects up to 12 models", func(t *testing.T) {
		t.Parallel()
		team := svc.SelectDebateTeamFromScores(5.0)
		assert.LessOrEqual(t, len(team), 12)
	})

	t.Run("respects minimum score", func(t *testing.T) {
		t.Parallel()
		team := svc.SelectDebateTeamFromScores(7.0)
		for _, m := range team {
			assert.GreaterOrEqual(t, m.OverallScore, 7.0)
		}
	})

	t.Run("enforces provider diversity", func(t *testing.T) {
		t.Parallel()
		team := svc.SelectDebateTeamFromScores(5.0)
		providerCounts := make(map[string]int)
		for _, m := range team {
			providerCounts[m.Provider]++
		}
		for provider, count := range providerCounts {
			assert.LessOrEqual(t, count, 3, "Provider %s should have at most 3 models", provider)
		}
	})
}

func TestEnhancedScoringService_CalculateWeightedVote(t *testing.T) {
	t.Parallel()
	svc := NewEnhancedScoringService(nil)

	t.Run("selects response with highest weighted vote", func(t *testing.T) {
		t.Parallel()
		responses := map[string]string{
			"model-1": "answer A",
			"model-2": "answer A",
			"model-3": "answer B",
		}
		confidences := map[string]float64{
			"model-1": 0.8,
			"model-2": 0.9,
			"model-3": 0.95,
		}

		// A: 0.8 + 0.9 = 1.7, B: 0.95
		result := svc.CalculateWeightedVote(responses, confidences)
		assert.Equal(t, "answer A", result)
	})

	t.Run("handles missing confidence with default", func(t *testing.T) {
		t.Parallel()
		responses := map[string]string{
			"model-1": "answer A",
			"model-2": "answer B",
		}
		confidences := map[string]float64{
			"model-1": 0.9,
			// model-2 missing, should use default 0.5
		}

		result := svc.CalculateWeightedVote(responses, confidences)
		assert.Equal(t, "answer A", result)
	})

	t.Run("handles single response", func(t *testing.T) {
		t.Parallel()
		responses := map[string]string{
			"model-1": "only answer",
		}
		confidences := map[string]float64{
			"model-1": 0.5,
		}

		result := svc.CalculateWeightedVote(responses, confidences)
		assert.Equal(t, "only answer", result)
	})
}

func TestEnhancedScoringService_ComputeWeightedScore(t *testing.T) {
	t.Parallel()
	svc := NewEnhancedScoringService(nil)

	t.Run("computes weighted score correctly", func(t *testing.T) {
		t.Parallel()
		components := EnhancedScoreComponents{
			ResponseSpeed:     8.0,
			ModelEfficiency:   7.0,
			CostEffectiveness: 9.0,
			Capability:        8.5,
			Recency:           7.5,
			CodeQuality:       8.0,
			ReasoningScore:    7.5,
		}

		// Expected: 8*0.2 + 7*0.15 + 9*0.2 + 8.5*0.15 + 7.5*0.05 + 8*0.15 + 7.5*0.1
		// = 1.6 + 1.05 + 1.8 + 1.275 + 0.375 + 1.2 + 0.75 = 8.05
		score := svc.computeWeightedScore(components)
		assert.InDelta(t, 8.05, score, 0.01)
	})

	t.Run("caps score at 10.0", func(t *testing.T) {
		t.Parallel()
		components := EnhancedScoreComponents{
			ResponseSpeed:     10.0,
			ModelEfficiency:   10.0,
			CostEffectiveness: 10.0,
			Capability:        10.0,
			Recency:           10.0,
			CodeQuality:       10.0,
			ReasoningScore:    10.0,
		}

		score := svc.computeWeightedScore(components)
		assert.LessOrEqual(t, score, 10.0)
	})

	t.Run("returns non-negative score", func(t *testing.T) {
		t.Parallel()
		components := EnhancedScoreComponents{
			ResponseSpeed:     0.0,
			ModelEfficiency:   0.0,
			CostEffectiveness: 0.0,
			Capability:        0.0,
			Recency:           0.0,
			CodeQuality:       0.0,
			ReasoningScore:    0.0,
		}

		score := svc.computeWeightedScore(components)
		assert.GreaterOrEqual(t, score, 0.0)
	})
}

func TestContainsIgnoreCaseEnhanced(t *testing.T) {
	t.Parallel()
	// Test the containsIgnoreCase function (defined in discovery.go)
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"Hello World", "world", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "hello", true},
		{"Hello World", "notfound", false},
		{"", "anything", false},
		{"something", "", true},
		{"UPPERCASE", "uppercase", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"-"+tt.substr, func(t *testing.T) {
			t.Parallel()
			result := containsIgnoreCase(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}
