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
// Test Helpers
// =============================================================================

func createTestLessonBank() *debate.LessonBank {
	config := debate.DefaultLessonBankConfig()
	config.EnableSemanticSearch = false // Disable for testing
	return debate.NewLessonBank(config, nil, nil)
}

func createTestRepository() *DefaultRepository {
	lessonBank := createTestLessonBank()
	config := DefaultRepositoryConfig()
	return NewDefaultRepository(lessonBank, config)
}

func createTestDebateResult() *protocol.DebateResult {
	return &protocol.DebateResult{
		ID:    "test-debate-1",
		Topic: "How to implement secure authentication",
		Phases: []*protocol.PhaseResult{
			{
				Phase: topology.PhaseProposal,
				Round: 1,
				Responses: []*protocol.PhaseResponse{
					{
						AgentID:    "agent-1",
						Role:       topology.RoleProposer,
						Provider:   "claude",
						Model:      "claude-3",
						Content:    "Use OAuth 2.0 with JWT tokens",
						Confidence: 0.85,
						Arguments:  []string{"Industry standard", "Scalable"},
					},
					{
						AgentID:    "agent-2",
						Role:       topology.RoleCritic,
						Provider:   "deepseek",
						Model:      "deepseek-coder",
						Content:    "Consider PKCE for public clients",
						Confidence: 0.80,
						Arguments:  []string{"Enhanced security"},
					},
				},
				ConsensusLevel: 0.75,
				KeyInsights:    []string{"OAuth 2.0 recommended", "JWT for stateless auth"},
			},
			{
				Phase: topology.PhaseCritique,
				Round: 1,
				Responses: []*protocol.PhaseResponse{
					{
						AgentID:    "agent-2",
						Role:       topology.RoleCritic,
						Provider:   "deepseek",
						Model:      "deepseek-coder",
						Content:    "Token rotation is important",
						Confidence: 0.88,
						Criticisms: []string{"Consider refresh token expiry"},
					},
				},
				ConsensusLevel: 0.82,
				KeyInsights:    []string{"Token rotation recommended"},
			},
		},
		FinalConsensus: &protocol.ConsensusResult{
			Summary:    "Use OAuth 2.0 with JWT and PKCE, implement token rotation",
			Confidence: 0.85,
			KeyPoints:  []string{"OAuth 2.0", "JWT tokens", "PKCE", "Token rotation"},
			Method:     protocol.ConsensusMethodWeightedVoting,
		},
		ParticipantCount: 3,
		TotalRounds:      3,
		RoundsCompleted:  3,
		Success:          true,
		StartTime:        time.Now().Add(-5 * time.Minute),
		EndTime:          time.Now(),
		Duration:         5 * time.Minute,
		TopologyUsed:     topology.TopologyGraphMesh,
	}
}

// =============================================================================
// Repository Interface Tests
// =============================================================================

func TestNewDefaultRepository(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()

	assert.NotNil(t, repo)
	assert.NotNil(t, repo.lessonBank)
	assert.NotNil(t, repo.patterns)
	assert.NotNil(t, repo.strategies)
	assert.NotNil(t, repo.history)
}

func TestDefaultSearchOptions(t *testing.T) {
	t.Parallel()
	opts := DefaultSearchOptions()

	assert.Equal(t, 0.5, opts.MinScore)
	assert.Equal(t, 10, opts.Limit)
}

func TestDefaultRepositoryConfig(t *testing.T) {
	t.Parallel()
	config := DefaultRepositoryConfig()

	assert.Equal(t, 1000, config.MaxPatterns)
	assert.Equal(t, 500, config.MaxStrategies)
	assert.Equal(t, 10000, config.MaxHistoryEntries)
	assert.Equal(t, 0.7, config.PatternThreshold)
}

// =============================================================================
// ExtractLessons Tests
// =============================================================================

func TestRepository_ExtractLessons(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()
	result := createTestDebateResult()

	lessons, err := repo.ExtractLessons(ctx, result)
	require.NoError(t, err)

	// Should extract some lessons
	assert.NotNil(t, lessons)

	// Should record history
	history, err := repo.GetDebateHistory(ctx, HistoryFilter{Limit: 10})
	require.NoError(t, err)
	assert.NotEmpty(t, history)
}

func TestRepository_ExtractLessons_NilResult(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	_, err := repo.ExtractLessons(ctx, nil)
	assert.Error(t, err)
}

// =============================================================================
// SearchLessons Tests
// =============================================================================

func TestRepository_SearchLessons_Empty(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	matches, err := repo.SearchLessons(ctx, "authentication", DefaultSearchOptions())
	require.NoError(t, err)
	assert.Empty(t, matches) // No lessons yet
}

// =============================================================================
// GetRelevantLessons Tests
// =============================================================================

func TestRepository_GetRelevantLessons(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	// First extract some lessons
	result := createTestDebateResult()
	_, _ = repo.ExtractLessons(ctx, result)

	// Now search for relevant lessons
	matches, err := repo.GetRelevantLessons(ctx, "security authentication", agents.DomainSecurity)
	require.NoError(t, err)
	assert.NotNil(t, matches)
}

// =============================================================================
// Pattern Tests
// =============================================================================

func TestRepository_RecordPattern(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	pattern := &DebatePattern{
		Name:        "Test Pattern",
		Description: "A test pattern for validation",
		PatternType: PatternTypeConsensusBuilding,
		Domain:      agents.DomainCode,
		Frequency:   1,
		SuccessRate: 0.85,
		Confidence:  0.8,
	}

	err := repo.RecordPattern(ctx, pattern)
	require.NoError(t, err)
	assert.NotEmpty(t, pattern.ID)
}

func TestRepository_GetPatterns(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	// Record some patterns
	_ = repo.RecordPattern(ctx, &DebatePattern{
		Name:        "Pattern 1",
		PatternType: PatternTypeConsensusBuilding,
		Domain:      agents.DomainCode,
		Frequency:   5,
		SuccessRate: 0.9,
	})

	_ = repo.RecordPattern(ctx, &DebatePattern{
		Name:        "Pattern 2",
		PatternType: PatternTypeFailure,
		Domain:      agents.DomainSecurity,
		Frequency:   3,
		SuccessRate: 0.2,
	})

	// Get all patterns
	patterns, err := repo.GetPatterns(ctx, PatternFilter{})
	require.NoError(t, err)
	assert.Len(t, patterns, 2)

	// Filter by type
	patterns, err = repo.GetPatterns(ctx, PatternFilter{
		Types: []PatternType{PatternTypeConsensusBuilding},
	})
	require.NoError(t, err)
	assert.Len(t, patterns, 1)
	assert.Equal(t, "Pattern 1", patterns[0].Name)

	// Filter by domain
	patterns, err = repo.GetPatterns(ctx, PatternFilter{
		Domain: agents.DomainSecurity,
	})
	require.NoError(t, err)
	assert.Len(t, patterns, 1)
	assert.Equal(t, "Pattern 2", patterns[0].Name)
}

func TestRepository_PatternDuplicateDetection(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	// Record same pattern twice
	pattern := &DebatePattern{
		Name:        "Duplicate Pattern",
		PatternType: PatternTypeConsensusBuilding,
		Domain:      agents.DomainCode,
		Frequency:   1,
		SuccessRate: 0.8,
	}

	_ = repo.RecordPattern(ctx, pattern)
	_ = repo.RecordPattern(ctx, &DebatePattern{
		Name:        "Duplicate Pattern",
		PatternType: PatternTypeConsensusBuilding,
		Domain:      agents.DomainCode,
		Frequency:   1,
		SuccessRate: 0.9,
	})

	// Should merge into one with increased frequency
	patterns, _ := repo.GetPatterns(ctx, PatternFilter{})
	assert.Len(t, patterns, 1)
	assert.Equal(t, 2, patterns[0].Frequency)
}

// =============================================================================
// Strategy Tests
// =============================================================================

func TestRepository_GetSuccessfulStrategies(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	// Add a strategy manually for testing
	repo.mu.Lock()
	repo.strategies["test-strategy"] = &Strategy{
		ID:           "test-strategy",
		Name:         "Test Strategy",
		Domain:       agents.DomainCode,
		SuccessRate:  0.9,
		Applications: 10,
	}
	repo.mu.Unlock()

	strategies, err := repo.GetSuccessfulStrategies(ctx, agents.DomainCode)
	require.NoError(t, err)
	assert.Len(t, strategies, 1)
}

// =============================================================================
// AgentKnowledge Tests
// =============================================================================

func TestRepository_GetKnowledgeForAgent(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	// Create a specialized agent
	agent := agents.NewSpecializedAgent("Test Agent", "claude", "claude-3", agents.DomainSecurity)

	knowledge, err := repo.GetKnowledgeForAgent(ctx, agent, "security vulnerability detection")
	require.NoError(t, err)
	require.NotNil(t, knowledge)

	assert.Equal(t, agent.ID, knowledge.AgentID)
	assert.NotNil(t, knowledge.DomainInsights)
	assert.NotNil(t, knowledge.RoleGuidance)
}

// =============================================================================
// History Tests
// =============================================================================

func TestRepository_GetDebateHistory(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	// Extract lessons (which records history)
	result := createTestDebateResult()
	_, _ = repo.ExtractLessons(ctx, result)

	history, err := repo.GetDebateHistory(ctx, HistoryFilter{Limit: 10})
	require.NoError(t, err)
	require.NotEmpty(t, history)

	assert.Equal(t, result.ID, history[0].ID)
	assert.True(t, history[0].Success)
}

func TestRepository_GetDebateHistory_Filtered(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	// Add some history entries
	repo.mu.Lock()
	repo.history = append(repo.history, &DebateHistoryEntry{
		ID:             "debate-1",
		Topic:          "Security topic",
		Domain:         agents.DomainSecurity,
		Success:        true,
		ConsensusScore: 0.9,
	})
	repo.history = append(repo.history, &DebateHistoryEntry{
		ID:             "debate-2",
		Topic:          "Code topic",
		Domain:         agents.DomainCode,
		Success:        false,
		ConsensusScore: 0.4,
	})
	repo.mu.Unlock()

	// Filter success only
	history, err := repo.GetDebateHistory(ctx, HistoryFilter{SuccessOnly: true})
	require.NoError(t, err)
	assert.Len(t, history, 1)

	// Filter by domain
	history, err = repo.GetDebateHistory(ctx, HistoryFilter{Domain: agents.DomainCode})
	require.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Equal(t, agents.DomainCode, history[0].Domain)
}

// =============================================================================
// Statistics Tests
// =============================================================================

func TestRepository_GetStatistics(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	// Add some data
	_ = repo.RecordPattern(ctx, &DebatePattern{
		Name:        "Test Pattern",
		PatternType: PatternTypeConsensusBuilding,
		Domain:      agents.DomainCode,
	})

	stats, err := repo.GetStatistics(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 1, stats.TotalPatterns)
}

// =============================================================================
// Domain Inference Tests
// =============================================================================

func TestRepository_InferDomainFromTopic(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()

	testCases := []struct {
		topic    string
		expected agents.Domain
	}{
		{"Security vulnerability in authentication", agents.DomainSecurity},
		{"Architecture design for microservices", agents.DomainArchitecture},
		{"Performance optimization for database", agents.DomainOptimization},
		{"Debug error in production", agents.DomainDebug},
		{"Code implementation of feature", agents.DomainCode},
		{"Random unrelated topic", agents.DomainGeneral},
	}

	for _, tc := range testCases {
		t.Run(tc.topic, func(t *testing.T) {
				t.Parallel()
			domain := repo.inferDomainFromTopic(tc.topic)
			assert.Equal(t, tc.expected, domain)
		})
	}
}

// =============================================================================
// Convert Debate Result Tests
// =============================================================================

func TestRepository_ConvertDebateResult(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	result := createTestDebateResult()

	converted := repo.convertDebateResult(result)

	assert.Equal(t, result.ID, converted.ID)
	assert.Equal(t, result.Topic, converted.Topic)
	assert.Len(t, converted.Rounds, len(result.Phases))
	assert.NotNil(t, converted.Consensus)
	assert.Equal(t, result.FinalConsensus.Summary, converted.Consensus.Summary)
}

// =============================================================================
// Relevance Tests
// =============================================================================

func TestRelevance_Structure(t *testing.T) {
	t.Parallel()
	relevance := &Relevance{
		TopicMatch:   0.8,
		DomainMatch:  0.9,
		PatternMatch: 0.7,
		Reasons:      []string{"Topic similarity", "Domain match"},
	}

	assert.Equal(t, 0.8, relevance.TopicMatch)
	assert.Len(t, relevance.Reasons, 2)
}

// =============================================================================
// LessonMatch Tests
// =============================================================================

func TestLessonMatch_Structure(t *testing.T) {
	t.Parallel()
	match := &LessonMatch{
		Lesson: &debate.Lesson{
			ID:    "lesson-1",
			Title: "Test Lesson",
		},
		Score:     0.85,
		MatchType: "semantic",
		Relevance: &Relevance{
			TopicMatch: 0.85,
		},
	}

	assert.Equal(t, "lesson-1", match.Lesson.ID)
	assert.Equal(t, 0.85, match.Score)
	assert.Equal(t, "semantic", match.MatchType)
}

// =============================================================================
// Pattern Type Tests
// =============================================================================

func TestPatternTypes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, PatternType("consensus_building"), PatternTypeConsensusBuilding)
	assert.Equal(t, PatternType("conflict_resolution"), PatternTypeConflictResolution)
	assert.Equal(t, PatternType("knowledge_gap"), PatternTypeKnowledgeGap)
	assert.Equal(t, PatternType("expertise"), PatternTypeExpertise)
	assert.Equal(t, PatternType("optimization"), PatternTypeOptimization)
	assert.Equal(t, PatternType("failure"), PatternTypeFailure)
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestTruncate(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "short", truncate("short", 10))
	// "long string here" (16 chars) truncated to 10 chars:
	// "long st" (7 chars) + "..." (3 chars) = 10 chars
	assert.Equal(t, "long st...", truncate("long string here", 10))
}

func TestMin(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 5, min(5, 10))
	assert.Equal(t, 3, min(10, 3))
	assert.Equal(t, 5, min(5, 5))
}
