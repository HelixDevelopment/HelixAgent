package integration

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/services"
)

// TestAIDebateIntegration_BasicWorkflow tests the basic AI debate integration workflow
func TestAIDebateIntegration_BasicWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logrus.New()
	ctx := context.Background()

	// Create debate service
	debateService := services.NewDebateService(logger)

	t.Run("BasicDebateWorkflow", func(t *testing.T) {
		// Create debate configuration
		debateConfig := &services.DebateConfig{
			DebateID: "integration-basic-001",
			Topic:    "Test basic debate workflow",
			Participants: []services.ParticipantConfig{
				{
					ParticipantID: "participant-1",
					Name:          "Alice",
					Role:          "proponent",
					LLMProvider:   "claude",
					LLMModel:      "claude-3-opus-20240229",
					MaxRounds:     2,
					Timeout:       30 * time.Second,
					Weight:        1.0,
				},
				{
					ParticipantID: "participant-2",
					Name:          "Bob",
					Role:          "opponent",
					LLMProvider:   "deepseek",
					LLMModel:      "deepseek-chat",
					MaxRounds:     2,
					Timeout:       30 * time.Second,
					Weight:        1.0,
				},
			},
			MaxRounds:    2,
			Timeout:      3 * time.Minute,
			Strategy:     "structured",
			EnableCognee: true,
		}

		// Conduct debate
		result, err := debateService.ConductDebate(ctx, debateConfig)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, debateConfig.DebateID, result.DebateID)
		assert.True(t, result.Success)
		assert.Greater(t, result.TotalRounds, 0)
		assert.Len(t, result.Participants, 2)
		assert.NotNil(t, result.Consensus)
		assert.NotNil(t, result.CogneeInsights)
	})

	t.Run("DebateWithDifferentStrategies", func(t *testing.T) {
		strategies := []string{"structured", "free_form", "round_robin"}

		for _, strategy := range strategies {
			debateConfig := &services.DebateConfig{
				DebateID: "integration-strategy-" + strategy,
				Topic:    "Testing strategy: " + strategy,
				Participants: []services.ParticipantConfig{
					{
						ParticipantID: "participant-1",
						Name:          "Alice",
						Role:          "proponent",
						LLMProvider:   "claude",
						LLMModel:      "claude-3-opus-20240229",
						MaxRounds:     2,
						Timeout:       30 * time.Second,
						Weight:        1.0,
					},
					{
						ParticipantID: "participant-2",
						Name:          "Bob",
						Role:          "opponent",
						LLMProvider:   "deepseek",
						LLMModel:      "deepseek-chat",
						MaxRounds:     2,
						Timeout:       30 * time.Second,
						Weight:        1.0,
					},
				},
				MaxRounds:    2,
				Timeout:      3 * time.Minute,
				Strategy:     strategy,
				EnableCognee: true,
			}

			result, err := debateService.ConductDebate(ctx, debateConfig)
			require.NoError(t, err)
			assert.True(t, result.Success)
			assert.Equal(t, strategy, debateConfig.Strategy)
		}
	})

	t.Run("DebateWithCogneeEnhancement", func(t *testing.T) {
		debateConfig := &services.DebateConfig{
			DebateID: "integration-cognee-001",
			Topic:    "Test debate with Cognee enhancement",
			Participants: []services.ParticipantConfig{
				{
					ParticipantID: "participant-1",
					Name:          "Alice",
					Role:          "proponent",
					LLMProvider:   "claude",
					LLMModel:      "claude-3-opus-20240229",
					MaxRounds:     2,
					Timeout:       30 * time.Second,
					Weight:        1.0,
				},
				{
					ParticipantID: "participant-2",
					Name:          "Bob",
					Role:          "opponent",
					LLMProvider:   "deepseek",
					LLMModel:      "deepseek-chat",
					MaxRounds:     2,
					Timeout:       30 * time.Second,
					Weight:        1.0,
				},
			},
			MaxRounds:    2,
			Timeout:      3 * time.Minute,
			Strategy:     "structured",
			EnableCognee: true,
		}

		result, err := debateService.ConductDebate(ctx, debateConfig)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.NotNil(t, result.CogneeInsights)
	})

	t.Run("DebateWithPerformanceMetrics", func(t *testing.T) {
		debateService := services.NewDebateService(logger)
		performanceService := services.NewDebatePerformanceService(logger)

		debateConfig := &services.DebateConfig{
			DebateID: "integration-performance-001",
			Topic:    "Test debate with performance metrics",
			Participants: []services.ParticipantConfig{
				{
					ParticipantID: "participant-1",
					Name:          "Alice",
					Role:          "proponent",
					LLMProvider:   "claude",
					LLMModel:      "claude-3-opus-20240229",
					MaxRounds:     2,
					Timeout:       30 * time.Second,
					Weight:        1.0,
				},
				{
					ParticipantID: "participant-2",
					Name:          "Bob",
					Role:          "opponent",
					LLMProvider:   "deepseek",
					LLMModel:      "deepseek-chat",
					MaxRounds:     2,
					Timeout:       30 * time.Second,
					Weight:        1.0,
				},
			},
			MaxRounds:    2,
			Timeout:      3 * time.Minute,
			Strategy:     "structured",
			EnableCognee: false,
		}

		result, err := debateService.ConductDebate(ctx, debateConfig)
		require.NoError(t, err)

		// Calculate performance metrics
		metrics := performanceService.CalculateMetrics(result)
		assert.NotNil(t, metrics)
		assert.Equal(t, result.Duration, metrics.Duration)
		assert.Equal(t, result.TotalRounds, metrics.TotalRounds)
		assert.Equal(t, result.QualityScore, metrics.QualityScore)
	})

	t.Run("DebateWithErrorHandling", func(t *testing.T) {
		resilienceService := services.NewDebateResilienceService(logger)

		// Test error handling
		err := resilienceService.HandleFailure(ctx, assert.AnError)
		assert.NoError(t, err)
	})

	t.Run("DebateWithSecurityValidation", func(t *testing.T) {
		securityService := services.NewDebateSecurityService(logger)

		debateConfig := &services.DebateConfig{
			DebateID: "integration-security-001",
			Topic:    "Test security validation",
			Participants: []services.ParticipantConfig{
				{
					ParticipantID: "participant-1",
					Name:          "Alice",
					Role:          "proponent",
					LLMProvider:   "claude",
					LLMModel:      "claude-3-opus-20240229",
					MaxRounds:     1,
					Timeout:       30 * time.Second,
					Weight:        1.0,
				},
			},
			MaxRounds:    1,
			Timeout:      30 * time.Second,
			Strategy:     "structured",
			EnableCognee: false,
		}

		err := securityService.ValidateDebateRequest(ctx, debateConfig)
		assert.NoError(t, err)

		err = securityService.AuditDebate(ctx, debateConfig.DebateID)
		assert.NoError(t, err)
	})
}
