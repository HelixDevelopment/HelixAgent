package debate_integration

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/services"
	"digital.vasic.debate/orchestrator"
)

// =============================================================================
// ServiceIntegrationConfig Tests
// =============================================================================

func TestDefaultServiceIntegrationConfig(t *testing.T) {
	config := DefaultServiceIntegrationConfig()

	assert.True(t, config.EnableNewFramework)
	assert.False(t, config.FallbackToLegacy) // Changed: new framework is mandatory, no fallback
	assert.True(t, config.EnableLearning)
	assert.Equal(t, 3, config.MinAgentsForNewFramework)
	assert.True(t, config.LogDebateDetails)
}

// =============================================================================
// ServiceIntegration Creation Tests
// =============================================================================

func TestNewServiceIntegration(t *testing.T) {
	logger := logrus.New()
	config := DefaultServiceIntegrationConfig()

	si := NewServiceIntegration(nil, logger, config)

	require.NotNil(t, si)
	assert.NotNil(t, si.orchestrator)
	assert.Equal(t, logger, si.logger)
	assert.Equal(t, config.EnableNewFramework, si.config.EnableNewFramework)
}

func TestNewServiceIntegration_NilLogger(t *testing.T) {
	config := DefaultServiceIntegrationConfig()

	si := NewServiceIntegration(nil, nil, config)

	require.NotNil(t, si)
	assert.Nil(t, si.logger)
}

func TestCreateIntegration(t *testing.T) {
	logger := logrus.New()

	si := CreateIntegration(nil, logger)

	require.NotNil(t, si)
	assert.NotNil(t, si.orchestrator)
	// Should use default config
	assert.True(t, si.config.EnableNewFramework)
}

// =============================================================================
// ShouldUseNewFramework Tests
// =============================================================================

func TestServiceIntegration_ShouldUseNewFramework_Disabled(t *testing.T) {
	config := DefaultServiceIntegrationConfig()
	config.EnableNewFramework = false

	si := NewServiceIntegration(nil, nil, config)

	debateConfig := &services.DebateConfig{
		DebateID: "test-123",
		Topic:    "Test topic",
	}

	assert.False(t, si.ShouldUseNewFramework(debateConfig))
}

func TestServiceIntegration_ShouldUseNewFramework_NotEnoughAgents(t *testing.T) {
	config := DefaultServiceIntegrationConfig()
	config.MinAgentsForNewFramework = 10 // Require more agents than available

	si := NewServiceIntegration(nil, nil, config)

	debateConfig := &services.DebateConfig{
		DebateID: "test-123",
		Topic:    "Test topic",
	}

	// Should return false because we don't have enough agents
	assert.False(t, si.ShouldUseNewFramework(debateConfig))
}

func TestServiceIntegration_ShouldUseNewFramework_Enabled(t *testing.T) {
	config := DefaultServiceIntegrationConfig()
	config.MinAgentsForNewFramework = 0 // No minimum

	si := NewServiceIntegration(nil, nil, config)

	debateConfig := &services.DebateConfig{
		DebateID: "test-123",
		Topic:    "Test topic",
	}

	assert.True(t, si.ShouldUseNewFramework(debateConfig))
}

// =============================================================================
// GetOrchestrator Tests
// =============================================================================

func TestServiceIntegration_GetOrchestrator(t *testing.T) {
	si := NewServiceIntegration(nil, nil, DefaultServiceIntegrationConfig())

	orch := si.GetOrchestrator()

	assert.NotNil(t, orch)
	assert.Equal(t, si.orchestrator, orch)
}

// =============================================================================
// ConvertDebateConfig Tests
// =============================================================================

func TestServiceIntegration_ConvertDebateConfig(t *testing.T) {
	si := NewServiceIntegration(nil, nil, DefaultServiceIntegrationConfig())

	debateConfig := &services.DebateConfig{
		DebateID:  "debate-123",
		Topic:     "AI Ethics Discussion",
		MaxRounds: 5,
		Timeout:   5 * time.Minute, // time.Duration
		Strategy:  "consensus",
		Participants: []services.ParticipantConfig{
			{Name: "Agent1", LLMProvider: "claude", LLMModel: "claude-3"},
			{Name: "Agent2", LLMProvider: "deepseek", LLMModel: "deepseek-v2"},
		},
		Metadata: map[string]interface{}{"key": "value"},
	}

	request := si.convertDebateConfig(debateConfig)

	require.NotNil(t, request)
	assert.Equal(t, "debate-123", request.ID)
	assert.Equal(t, "AI Ethics Discussion", request.Topic)
	assert.Equal(t, 5, request.MaxRounds)
	assert.Equal(t, 5*time.Minute, request.Timeout)
	assert.Len(t, request.PreferredProviders, 2)
	assert.Contains(t, request.PreferredProviders, "claude")
	assert.Contains(t, request.PreferredProviders, "deepseek")
	assert.Equal(t, 0.75, request.MinConsensus)
	require.NotNil(t, request.EnableLearning)
	assert.True(t, *request.EnableLearning)
}

func TestServiceIntegration_ConvertDebateConfig_MinimalFields(t *testing.T) {
	si := NewServiceIntegration(nil, nil, DefaultServiceIntegrationConfig())

	debateConfig := &services.DebateConfig{
		Topic: "Simple Topic",
	}

	request := si.convertDebateConfig(debateConfig)

	require.NotNil(t, request)
	assert.Equal(t, "Simple Topic", request.Topic)
	assert.Equal(t, "", request.ID)
	assert.Equal(t, 0, request.MaxRounds)
}

func TestServiceIntegration_ConvertDebateConfig_NoLearning(t *testing.T) {
	config := DefaultServiceIntegrationConfig()
	config.EnableLearning = false

	si := NewServiceIntegration(nil, nil, config)

	debateConfig := &services.DebateConfig{
		Topic: "Topic",
	}

	request := si.convertDebateConfig(debateConfig)

	assert.Nil(t, request.EnableLearning)
}

// =============================================================================
// ConvertToDebateResult Tests
// =============================================================================

func TestServiceIntegration_ConvertToDebateResult(t *testing.T) {
	si := NewServiceIntegration(nil, nil, DefaultServiceIntegrationConfig())

	debateConfig := &services.DebateConfig{
		DebateID: "original-id",
		Topic:    "Original Topic",
	}

	response := &orchestrator.DebateResponse{
		ID:      "response-123",
		Topic:   "Test Topic",
		Success: true,
		Phases: []*orchestrator.PhaseResponse{
			{
				Phase: "proposal",
				Round: 1,
				Responses: []*orchestrator.AgentResponse{
					{
						AgentID:    "agent-1",
						Provider:   "claude",
						Model:      "claude-3",
						Role:       "proposer",
						Content:    "Test content",
						Confidence: 0.85,
						Score:      9.0,
						Latency:    2 * time.Second,
					},
				},
			},
		},
		Consensus: &orchestrator.ConsensusResponse{
			Summary:    "Final consensus",
			Confidence: 0.9,
			KeyPoints:  []string{"Point 1", "Point 2"},
			Dissents:   []string{"Dissent 1"},
		},
		Metrics: &orchestrator.DebateMetrics{
			// NOTE (round-342, HXA-002): the reconstructed DebateMetrics
			// counts provider invocations via ProviderCalls — the
			// pre-reconstruction TotalResponses field was deleted.
			ProviderCalls:  5,
			AvgConfidence:  0.85,
			ConsensusScore: 0.88,
		},
		LessonsLearned:   3,
		PatternsDetected: 2,
		Duration:         5 * time.Minute,
		Metadata:         map[string]interface{}{"key": "value"},
	}

	result := si.convertToDebateResult(response, debateConfig)

	require.NotNil(t, result)
	assert.Equal(t, "response-123", result.DebateID)
	assert.Equal(t, "Test Topic", result.Topic)
	assert.True(t, result.Success)

	// Check metadata
	assert.NotNil(t, result.Metadata)
	assert.Equal(t, "new_debate_system", result.Metadata["framework"])
	assert.Equal(t, 3, result.Metadata["lessons_learned"])
	assert.Equal(t, 2, result.Metadata["patterns_detected"])

	// Check responses
	assert.Len(t, result.AllResponses, 1)
	resp := result.AllResponses[0]
	assert.Equal(t, "agent-1", resp.ParticipantID)
	assert.Equal(t, "claude/claude-3", resp.ParticipantName)
	assert.Equal(t, "proposer", resp.Role)
	assert.Equal(t, "claude", resp.LLMProvider)
	assert.Equal(t, "claude-3", resp.LLMModel)
	assert.Equal(t, "Test content", resp.Content)
	assert.Equal(t, 0.85, resp.Confidence)
	assert.Equal(t, 9.0, resp.QualityScore)
	assert.Equal(t, 1, resp.Round)

	// Check consensus
	require.NotNil(t, result.Consensus)
	assert.True(t, result.Consensus.Reached)
	assert.Equal(t, 0.9, result.Consensus.AgreementLevel)
	assert.Equal(t, "Final consensus", result.Consensus.FinalPosition)
	assert.Len(t, result.Consensus.KeyPoints, 2)
	assert.Len(t, result.Consensus.Disagreements, 1)

	// Check scores
	assert.Equal(t, 0.85, result.QualityScore)
	assert.Equal(t, 0.9, result.FinalScore)
}

func TestServiceIntegration_ConvertToDebateResult_NoConsensus(t *testing.T) {
	si := NewServiceIntegration(nil, nil, DefaultServiceIntegrationConfig())

	response := &orchestrator.DebateResponse{
		ID:      "response-123",
		Topic:   "Topic",
		Success: false,
		Phases:  []*orchestrator.PhaseResponse{},
	}

	result := si.convertToDebateResult(response, &services.DebateConfig{})

	assert.Nil(t, result.Consensus)
	assert.Equal(t, 0.0, result.FinalScore)
}

func TestServiceIntegration_ConvertToDebateResult_MetricsOnly(t *testing.T) {
	si := NewServiceIntegration(nil, nil, DefaultServiceIntegrationConfig())

	response := &orchestrator.DebateResponse{
		ID:      "response-123",
		Topic:   "Topic",
		Success: true,
		Phases:  []*orchestrator.PhaseResponse{},
		Metrics: &orchestrator.DebateMetrics{
			AvgConfidence:  0.8,
			ConsensusScore: 0.85,
		},
	}

	result := si.convertToDebateResult(response, &services.DebateConfig{})

	assert.Equal(t, 0.8, result.QualityScore)
	assert.Equal(t, 0.85, result.FinalScore)
}

// =============================================================================
// ConductDebate Tests
// =============================================================================

func TestServiceIntegration_ConductDebate_FrameworkDisabled(t *testing.T) {
	config := DefaultServiceIntegrationConfig()
	config.EnableNewFramework = false

	si := NewServiceIntegration(nil, nil, config)

	ctx := context.Background()
	debateConfig := &services.DebateConfig{
		Topic: "Test topic",
	}

	result, err := si.ConductDebate(ctx, debateConfig)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "new framework is disabled")
}

// =============================================================================
// GetStatistics Tests
// =============================================================================

func TestServiceIntegration_GetStatistics(t *testing.T) {
	si := NewServiceIntegration(nil, nil, DefaultServiceIntegrationConfig())

	ctx := context.Background()
	stats, err := si.GetStatistics(ctx)

	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.True(t, stats.FrameworkEnabled)
	assert.True(t, stats.LearningEnabled)
	assert.GreaterOrEqual(t, stats.ActiveDebates, 0)
}

// =============================================================================
// IntegrationStatistics Tests
// =============================================================================

func TestIntegrationStatistics_Structure(t *testing.T) {
	stats := IntegrationStatistics{
		FrameworkEnabled:    true,
		LearningEnabled:     true,
		ActiveDebates:       5,
		RegisteredAgents:    10,
		TotalLessons:        100,
		TotalPatterns:       50,
		TotalDebatesLearned: 200,
		OverallSuccessRate:  0.85,
	}

	assert.True(t, stats.FrameworkEnabled)
	assert.True(t, stats.LearningEnabled)
	assert.Equal(t, 5, stats.ActiveDebates)
	assert.Equal(t, 10, stats.RegisteredAgents)
	assert.Equal(t, 100, stats.TotalLessons)
	assert.Equal(t, 50, stats.TotalPatterns)
	assert.Equal(t, 200, stats.TotalDebatesLearned)
	assert.Equal(t, 0.85, stats.OverallSuccessRate)
}

// =============================================================================
// CreateLessonBank Tests
// =============================================================================

func TestServiceIntegration_CreateLessonBank(t *testing.T) {
	bank := CreateLessonBank()

	require.NotNil(t, bank)
}

// =============================================================================
// Integration Tests with Mock Registry
// =============================================================================

func TestServiceIntegration_WithMockRegistry(t *testing.T) {
	mockRegistry := newMockProviderRegistry()
	mockRegistry.AddProvider("claude", newMockLLMProvider("claude"))
	mockRegistry.AddProvider("deepseek", newMockLLMProvider("deepseek"))
	mockRegistry.AddProvider("gemini", newMockLLMProvider("gemini"))

	config := DefaultServiceIntegrationConfig()
	config.MinAgentsForNewFramework = 3

	// Create orchestrator manually with mock
	orchConfig := orchestrator.DefaultOrchestratorConfig()
	orchConfig.EnableLearning = config.EnableLearning
	orchConfig.MinAgentsPerDebate = config.MinAgentsForNewFramework

	lessonBankConfig := defaultLessonBankConfig()
	lessonBank := createLessonBank(lessonBankConfig)

	orch := orchestrator.NewOrchestrator(mockRegistry, lessonBank, orchConfig)

	// Register providers
	_ = orch.RegisterProvider("claude", "claude-3", 0.9)
	_ = orch.RegisterProvider("deepseek", "deepseek-coder", 0.85)
	_ = orch.RegisterProvider("gemini", "gemini-pro", 0.8)

	// Create integration with the orchestrator
	si := &ServiceIntegration{
		orchestrator: orch,
		logger:       logrus.New(),
		config:       config,
	}

	// Test that framework would be used
	debateConfig := &services.DebateConfig{
		DebateID: "test-123",
		Topic:    "Test topic",
	}

	assert.True(t, si.ShouldUseNewFramework(debateConfig))
	assert.Equal(t, 3, si.GetOrchestrator().GetAgentPool().Size())
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestServiceIntegration_ConvertDebateConfig_EmptyParticipants(t *testing.T) {
	si := NewServiceIntegration(nil, nil, DefaultServiceIntegrationConfig())

	debateConfig := &services.DebateConfig{
		Topic:        "Topic",
		Participants: []services.ParticipantConfig{},
	}

	request := si.convertDebateConfig(debateConfig)

	assert.Len(t, request.PreferredProviders, 0)
}

func TestServiceIntegration_ConvertDebateConfig_ParticipantsWithoutProvider(t *testing.T) {
	si := NewServiceIntegration(nil, nil, DefaultServiceIntegrationConfig())

	debateConfig := &services.DebateConfig{
		Topic: "Topic",
		Participants: []services.ParticipantConfig{
			{Name: "Agent1"}, // No provider specified
			{Name: "Agent2", LLMProvider: "claude"},
		},
	}

	request := si.convertDebateConfig(debateConfig)

	assert.Len(t, request.PreferredProviders, 1)
	assert.Contains(t, request.PreferredProviders, "claude")
}

func TestServiceIntegration_ConvertToDebateResult_MultiplePhases(t *testing.T) {
	si := NewServiceIntegration(nil, nil, DefaultServiceIntegrationConfig())

	response := &orchestrator.DebateResponse{
		ID:      "response-123",
		Topic:   "Topic",
		Success: true,
		Phases: []*orchestrator.PhaseResponse{
			{
				Phase: "proposal",
				Round: 1,
				Responses: []*orchestrator.AgentResponse{
					{AgentID: "a1", Provider: "claude", Model: "claude-3", Content: "Content 1"},
				},
			},
			{
				Phase: "critique",
				Round: 1,
				Responses: []*orchestrator.AgentResponse{
					{AgentID: "a2", Provider: "deepseek", Model: "deepseek-v2", Content: "Content 2"},
				},
			},
			{
				Phase: "synthesis",
				Round: 2,
				Responses: []*orchestrator.AgentResponse{
					{AgentID: "a3", Provider: "gemini", Model: "gemini-pro", Content: "Content 3"},
				},
			},
		},
	}

	result := si.convertToDebateResult(response, &services.DebateConfig{})

	// Should have all responses from all phases
	assert.Len(t, result.AllResponses, 3)

	// Check phases are preserved in metadata
	assert.Equal(t, "proposal", result.AllResponses[0].Metadata["phase"])
	assert.Equal(t, "critique", result.AllResponses[1].Metadata["phase"])
	assert.Equal(t, "synthesis", result.AllResponses[2].Metadata["phase"])
}
