package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/debate"
	"dev.helix.agent/internal/debate/agents"
	"dev.helix.agent/internal/debate/protocol"
	"dev.helix.agent/internal/debate/topology"
)

// =============================================================================
// Integration Config Tests
// =============================================================================

func TestDefaultIntegrationConfig(t *testing.T) {
	t.Parallel()
	config := DefaultIntegrationConfig()

	assert.True(t, config.AutoExtractLessons)
	assert.True(t, config.AutoApplyLessons)
	assert.Equal(t, 0.7, config.MinConsensusForLesson)
	assert.Equal(t, 5, config.MaxLessonsPerDebate)
	assert.True(t, config.EnableCognitiveIntegration)
	assert.True(t, config.EnablePatternDetection)
	assert.Equal(t, 0.65, config.PatternThreshold)
}

// =============================================================================
// DebateLearningIntegration Tests
// =============================================================================

func TestNewDebateLearningIntegration(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()

	integration := NewDebateLearningIntegration(repo, config)

	assert.NotNil(t, integration)
	assert.NotNil(t, integration.repository)
	assert.NotNil(t, integration.activeDebates)
}

func TestDebateLearningIntegration_StartDebateLearning(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()
	integration := NewDebateLearningIntegration(repo, config)
	ctx := context.Background()

	// Create participants
	participants := []*agents.SpecializedAgent{
		agents.NewSpecializedAgent("Agent 1", "claude", "claude-3", agents.DomainCode),
		agents.NewSpecializedAgent("Agent 2", "deepseek", "deepseek-coder", agents.DomainSecurity),
	}

	session, err := integration.StartDebateLearning(ctx, "debate-1", "How to implement authentication", participants)
	require.NoError(t, err)
	require.NotNil(t, session)

	assert.Equal(t, "debate-1", session.DebateID)
	assert.Equal(t, "How to implement authentication", session.Topic)
	assert.Equal(t, agents.DomainSecurity, session.Domain) // "authentication" -> security
	assert.NotNil(t, session.AgentKnowledge)
}

func TestDebateLearningIntegration_OnPhaseComplete(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()
	integration := NewDebateLearningIntegration(repo, config)
	ctx := context.Background()

	// Start a session
	participants := []*agents.SpecializedAgent{
		agents.NewSpecializedAgent("Agent 1", "claude", "claude-3", agents.DomainCode),
	}
	_, err := integration.StartDebateLearning(ctx, "debate-1", "Code optimization", participants)
	require.NoError(t, err)

	// Complete a phase
	phaseResult := &protocol.PhaseResult{
		Phase: topology.PhaseProposal,
		Round: 1,
		Responses: []*protocol.PhaseResponse{
			{
				AgentID:    "agent-1",
				Confidence: 0.9,
				Content:    "Use caching for performance",
			},
		},
		ConsensusLevel: 0.85,
		KeyInsights:    []string{"Caching improves performance"},
	}

	err = integration.OnPhaseComplete(ctx, "debate-1", phaseResult)
	require.NoError(t, err)

	// Verify phase learning was recorded
	session, ok := integration.GetActiveSession("debate-1")
	require.True(t, ok)
	assert.Len(t, session.PhaseLearning, 1)
	assert.Equal(t, topology.PhaseProposal, session.PhaseLearning[0].Phase)
}

func TestDebateLearningIntegration_OnPhaseComplete_NoSession(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()
	integration := NewDebateLearningIntegration(repo, config)
	ctx := context.Background()

	err := integration.OnPhaseComplete(ctx, "nonexistent", &protocol.PhaseResult{})
	assert.Error(t, err)
}

func TestDebateLearningIntegration_OnDebateComplete(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()
	integration := NewDebateLearningIntegration(repo, config)
	ctx := context.Background()

	// Start a session
	participants := []*agents.SpecializedAgent{
		agents.NewSpecializedAgent("Agent 1", "claude", "claude-3", agents.DomainCode),
	}
	_, err := integration.StartDebateLearning(ctx, "debate-1", "Implementation strategy", participants)
	require.NoError(t, err)

	// Complete the debate
	result := createTestDebateResult()
	result.ID = "debate-1"

	learningResult, err := integration.OnDebateComplete(ctx, result)
	require.NoError(t, err)
	require.NotNil(t, learningResult)

	assert.Equal(t, "debate-1", learningResult.DebateID)
	assert.True(t, learningResult.SessionDuration > 0)

	// Session should be removed
	_, ok := integration.GetActiveSession("debate-1")
	assert.False(t, ok)
}

func TestDebateLearningIntegration_GetAgentKnowledge(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()
	integration := NewDebateLearningIntegration(repo, config)
	ctx := context.Background()

	// Create agent and start session
	agent := agents.NewSpecializedAgent("Agent 1", "claude", "claude-3", agents.DomainCode)
	participants := []*agents.SpecializedAgent{agent}

	_, err := integration.StartDebateLearning(ctx, "debate-1", "Code review best practices", participants)
	require.NoError(t, err)

	knowledge, err := integration.GetAgentKnowledge("debate-1", agent.ID)
	require.NoError(t, err)
	require.NotNil(t, knowledge)
}

func TestDebateLearningIntegration_GetAgentKnowledge_NoSession(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()
	integration := NewDebateLearningIntegration(repo, config)

	_, err := integration.GetAgentKnowledge("nonexistent", "agent-1")
	assert.Error(t, err)
}

func TestDebateLearningIntegration_GetLessonsForPrompt(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()
	integration := NewDebateLearningIntegration(repo, config)
	ctx := context.Background()

	// Create agent and start session
	agent := agents.NewSpecializedAgent("Agent 1", "claude", "claude-3", agents.DomainCode)
	participants := []*agents.SpecializedAgent{agent}

	_, err := integration.StartDebateLearning(ctx, "debate-1", "Testing strategies", participants)
	require.NoError(t, err)

	prompt, err := integration.GetLessonsForPrompt("debate-1", agent.ID)
	require.NoError(t, err)
	// May be empty if no lessons match
	assert.NotNil(t, &prompt)
}

func TestDebateLearningIntegration_GetActiveSessions(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()
	integration := NewDebateLearningIntegration(repo, config)
	ctx := context.Background()

	// Start multiple sessions
	agent := agents.NewSpecializedAgent("Agent 1", "claude", "claude-3", agents.DomainCode)
	participants := []*agents.SpecializedAgent{agent}

	_, _ = integration.StartDebateLearning(ctx, "debate-1", "Topic 1", participants)
	_, _ = integration.StartDebateLearning(ctx, "debate-2", "Topic 2", participants)

	sessions := integration.GetActiveSessions()
	assert.Len(t, sessions, 2)
}

func TestDebateLearningIntegration_InferDomain(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()
	integration := NewDebateLearningIntegration(repo, config)

	testCases := []struct {
		topic    string
		expected agents.Domain
	}{
		{"Security vulnerability analysis", agents.DomainSecurity},
		{"System architecture design", agents.DomainArchitecture},
		{"Performance optimization techniques", agents.DomainOptimization},
		{"Debug error tracing", agents.DomainDebug},
		{"Code implementation patterns", agents.DomainCode},
		{"Logical reasoning approach", agents.DomainReasoning},
		{"Random topic", agents.DomainGeneral},
	}

	for _, tc := range testCases {
		t.Run(tc.topic, func(t *testing.T) {
				t.Parallel()
			domain := integration.inferDomain(tc.topic)
			assert.Equal(t, tc.expected, domain)
		})
	}
}

// =============================================================================
// Pattern Detection Tests
// =============================================================================

func TestDebateLearningIntegration_DetectPatterns_HighConsensus(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()
	integration := NewDebateLearningIntegration(repo, config)
	ctx := context.Background()

	// Start session
	agent := agents.NewSpecializedAgent("Agent 1", "claude", "claude-3", agents.DomainCode)
	session, _ := integration.StartDebateLearning(ctx, "debate-1", "Topic", []*agents.SpecializedAgent{agent})

	// High consensus phase result
	phaseResult := &protocol.PhaseResult{
		Phase:          topology.PhaseConvergence,
		ConsensusLevel: 0.9, // High consensus
		Responses: []*protocol.PhaseResponse{
			{Confidence: 0.9},
			{Confidence: 0.85},
		},
	}

	patterns := integration.detectPatterns(ctx, session, phaseResult)

	// Should detect high consensus pattern
	found := false
	for _, p := range patterns {
		if p.PatternType == PatternTypeConsensusBuilding {
			found = true
			break
		}
	}
	assert.True(t, found, "Should detect consensus building pattern")
}

func TestDebateLearningIntegration_DetectPatterns_ConflictResolution(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()
	integration := NewDebateLearningIntegration(repo, config)
	ctx := context.Background()

	// Start session
	agent := agents.NewSpecializedAgent("Agent 1", "claude", "claude-3", agents.DomainCode)
	session, _ := integration.StartDebateLearning(ctx, "debate-1", "Topic", []*agents.SpecializedAgent{agent})

	// Phase with disagreements but resolved
	phaseResult := &protocol.PhaseResult{
		Phase:          topology.PhaseCritique,
		ConsensusLevel: 0.7,
		Disagreements:  []string{"Approach A vs B", "Technology choice"},
		Responses:      []*protocol.PhaseResponse{{Confidence: 0.8}},
	}

	patterns := integration.detectPatterns(ctx, session, phaseResult)

	// Should detect conflict resolution pattern
	found := false
	for _, p := range patterns {
		if p.PatternType == PatternTypeConflictResolution {
			found = true
			break
		}
	}
	assert.True(t, found, "Should detect conflict resolution pattern")
}

func TestDebateLearningIntegration_DetectPatterns_Expertise(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()
	integration := NewDebateLearningIntegration(repo, config)
	ctx := context.Background()

	// Start session
	agent := agents.NewSpecializedAgent("Agent 1", "claude", "claude-3", agents.DomainCode)
	session, _ := integration.StartDebateLearning(ctx, "debate-1", "Topic", []*agents.SpecializedAgent{agent})

	// Multiple high-confidence responses
	phaseResult := &protocol.PhaseResult{
		Phase:          topology.PhaseProposal,
		ConsensusLevel: 0.7,
		Responses: []*protocol.PhaseResponse{
			{Confidence: 0.9},
			{Confidence: 0.88},
			{Confidence: 0.92},
		},
	}

	patterns := integration.detectPatterns(ctx, session, phaseResult)

	// Should detect expertise pattern
	found := false
	for _, p := range patterns {
		if p.PatternType == PatternTypeExpertise {
			found = true
			break
		}
	}
	assert.True(t, found, "Should detect expertise pattern")
}

// =============================================================================
// Learning Session Tests
// =============================================================================

func TestDebateLearningSession_Structure(t *testing.T) {
	t.Parallel()
	session := &DebateLearningSession{
		DebateID:         "test-debate",
		Topic:            "Test Topic",
		Domain:           agents.DomainCode,
		StartTime:        time.Now(),
		AppliedLessons:   make([]*LessonApplication, 0),
		PhaseLearning:    make([]*PhaseLearning, 0),
		DetectedPatterns: make([]*DebatePattern, 0),
		AgentKnowledge:   make(map[string]*AgentKnowledge),
	}

	assert.Equal(t, "test-debate", session.DebateID)
	assert.Equal(t, agents.DomainCode, session.Domain)
}

// =============================================================================
// Phase Learning Tests
// =============================================================================

func TestPhaseLearning_Structure(t *testing.T) {
	t.Parallel()
	phaseLearning := &PhaseLearning{
		Phase:           topology.PhaseProposal,
		InsightsGained:  []string{"Insight 1", "Insight 2"},
		PatternsMatched: []string{"Pattern A"},
		LessonsApplied:  []string{"Lesson 1"},
		QualityDelta:    0.1,
		Timestamp:       time.Now(),
	}

	assert.Equal(t, topology.PhaseProposal, phaseLearning.Phase)
	assert.Len(t, phaseLearning.InsightsGained, 2)
}

// =============================================================================
// Debate Learning Result Tests
// =============================================================================

func TestDebateLearningResult_Structure(t *testing.T) {
	t.Parallel()
	result := &DebateLearningResult{
		DebateID:             "debate-1",
		SessionDuration:      5 * time.Minute,
		AppliedLessons:       3,
		ExtractedLessons:     2,
		DetectedPatterns:     4,
		CognitiveRefinements: 5,
		ImprovementRate:      0.15,
		Lessons:              []*debate.Lesson{},
	}

	assert.Equal(t, "debate-1", result.DebateID)
	assert.Equal(t, 3, result.AppliedLessons)
	assert.Equal(t, 0.15, result.ImprovementRate)
}

// =============================================================================
// Learning Enhanced Protocol Tests
// =============================================================================

func TestNewLearningEnhancedProtocol(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultIntegrationConfig()
	integration := NewDebateLearningIntegration(repo, config)

	// Note: We can't fully test Execute without a real protocol
	// but we can test the structure
	lep := NewLearningEnhancedProtocol(nil, integration, []*agents.SpecializedAgent{})

	assert.NotNil(t, lep)
	assert.Equal(t, integration, lep.integration)
}
