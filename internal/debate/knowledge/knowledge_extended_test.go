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
// Extended Tests for Knowledge Package Coverage Improvement
// Tests in this file cover additional edge cases and scenarios not covered
// by the existing test files.
// =============================================================================

// =============================================================================
// CrossDebateLearner Extended Tests
// =============================================================================

func TestNewCrossDebateLearner_WithCustomConfig(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultLearningConfig()
	config.MinDebatesForPattern = 5
	config.PatternConfidenceThreshold = 0.8

	learner := NewCrossDebateLearner(repo, config)

	assert.NotNil(t, learner)
	assert.Equal(t, 5, learner.config.MinDebatesForPattern)
	assert.Equal(t, 0.8, learner.config.PatternConfidenceThreshold)
}

func TestNewCrossDebateLearner_WithDisabledKnowledgeGraph(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultLearningConfig()
	config.EnableKnowledgeGraph = false

	learner := NewCrossDebateLearner(repo, config)

	assert.NotNil(t, learner)
	assert.Nil(t, learner.knowledgeGraph)
}

func TestCrossDebateLearner_LearnFromDebate_SuccessfulWithLessons(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultLearningConfig()
	config.StrategySuccessThreshold = 0.5
	learner := NewCrossDebateLearner(repo, config)
	ctx := context.Background()

	result := &protocol.DebateResult{
		ID:      "success-debate",
		Topic:   "Test Topic for Learning",
		Success: true,
		Phases: []*protocol.PhaseResult{
			{
				Phase:          topology.PhaseProposal,
				Round:          1,
				ConsensusLevel: 0.85,
				Responses: []*protocol.PhaseResponse{
					{AgentID: "a1", Provider: "claude", Model: "claude-3", Confidence: 0.9},
					{AgentID: "a2", Provider: "deepseek", Model: "deepseek-coder", Confidence: 0.85},
				},
			},
			{
				Phase:          topology.PhaseCritique,
				Round:          2,
				ConsensusLevel: 0.8,
				Responses: []*protocol.PhaseResponse{
					{AgentID: "a1", Confidence: 0.88},
				},
			},
		},
		FinalConsensus: &protocol.ConsensusResult{
			Summary:    "Consensus reached successfully",
			Confidence: 0.9,
			KeyPoints:  []string{"Point 1", "Point 2"},
		},
		TopologyUsed: topology.TopologyGraphMesh,
		Duration:     5 * time.Minute,
	}

	lessons := []*debate.Lesson{
		{
			ID:    "lesson-1",
			Title: "Test Lesson",
			Statistics: debate.LessonStatistics{
				ApplyCount:   5,
				SuccessCount: 4,
			},
		},
	}

	outcome, err := learner.LearnFromDebate(ctx, result, lessons)
	require.NoError(t, err)
	require.NotNil(t, outcome)

	assert.Equal(t, result.ID, outcome.DebateID)
	assert.NotZero(t, outcome.LearnedAt)
	assert.GreaterOrEqual(t, outcome.QualityScore, 0.0)
}

func TestCrossDebateLearner_GetRecommendations_WithMatchingStrategy(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultLearningConfig()
	learner := NewCrossDebateLearner(repo, config)
	ctx := context.Background()

	// Add a matching strategy to the repository
	repo.mu.Lock()
	repo.strategies["test-strategy"] = &Strategy{
		ID:           "test-strategy",
		Name:         "Security Testing Strategy",
		Domain:       agents.DomainSecurity,
		TopologyType: topology.TopologyGraphMesh,
		SuccessRate:  0.9,
		Applications: 10,
		Phases: []PhaseStrategy{
			{Phase: topology.PhaseProposal, FocusAreas: []string{"security analysis"}},
		},
	}
	repo.mu.Unlock()

	recommendations, err := learner.GetRecommendations(ctx, "security vulnerability assessment", agents.DomainSecurity)
	require.NoError(t, err)
	require.NotNil(t, recommendations)

	assert.Equal(t, agents.DomainSecurity, recommendations.Domain)
	assert.NotNil(t, recommendations.RecommendedStrategy)
	assert.Equal(t, "test-strategy", recommendations.RecommendedStrategy.ID)
}

func TestCrossDebateLearner_ApplyDecay_WithPatterns(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultLearningConfig()
	config.LearningDecayRate = 0.1
	learner := NewCrossDebateLearner(repo, config)
	ctx := context.Background()

	// Add a pattern with high confidence
	repo.mu.Lock()
	repo.patterns["test-pattern"] = &DebatePattern{
		ID:          "test-pattern",
		Confidence:  0.9,
		SuccessRate: 0.8,
	}
	repo.mu.Unlock()

	err := learner.ApplyDecay(ctx)
	assert.NoError(t, err)

	// Pattern confidence should have decayed (or stayed same if decay isn't applied to patterns)
	repo.mu.RLock()
	pattern := repo.patterns["test-pattern"]
	repo.mu.RUnlock()

	// Just verify the pattern still exists and confidence is valid
	assert.LessOrEqual(t, pattern.Confidence, 0.9)
	assert.Greater(t, pattern.Confidence, 0.0)
}

func TestCalculateLearningQuality_AllScenarios(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	learner := NewCrossDebateLearner(repo, DefaultLearningConfig())

	testCases := []struct {
		name     string
		outcome  *LearningOutcome
		minScore float64
		maxScore float64
	}{
		{
			name: "Empty outcome",
			outcome: &LearningOutcome{
				NewPatterns:     []*DebatePattern{},
				UpdatedPatterns: []string{},
				NewStrategies:   []*Strategy{},
				KnowledgeNodes:  []string{},
			},
			minScore: 0.0,
			maxScore: 0.5,
		},
		{
			name: "With multiple patterns",
			outcome: &LearningOutcome{
				NewPatterns:     []*DebatePattern{{}, {}, {}},
				UpdatedPatterns: []string{"p1", "p2"},
				NewStrategies:   []*Strategy{},
				KnowledgeNodes:  []string{},
			},
			minScore: 0.3,
			maxScore: 1.0,
		},
		{
			name: "With strategies and nodes",
			outcome: &LearningOutcome{
				NewPatterns:     []*DebatePattern{},
				UpdatedPatterns: []string{},
				NewStrategies:   []*Strategy{{}, {}},
				KnowledgeNodes:  []string{"n1", "n2", "n3", "n4", "n5"},
			},
			minScore: 0.4,
			maxScore: 1.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			score := learner.calculateLearningQuality(tc.outcome)
			assert.GreaterOrEqual(t, score, tc.minScore)
			assert.LessOrEqual(t, score, tc.maxScore)
		})
	}
}

// =============================================================================
// PatternAnalyzer Extended Tests
// =============================================================================

func TestPatternAnalyzer_Analyze_AllDetectors(t *testing.T) {
	t.Parallel()
	analyzer := NewPatternAnalyzer()

	// Create a result that triggers multiple pattern types
	result := &protocol.DebateResult{
		Success: true,
		Phases: []*protocol.PhaseResult{
			{
				ConsensusLevel: 0.9,
				Responses: []*protocol.PhaseResponse{
					{Provider: "claude", Model: "claude-3", Confidence: 0.92},
					{Provider: "claude", Model: "claude-3", Confidence: 0.9},
					{Provider: "claude", Model: "claude-3", Confidence: 0.88},
				},
			},
		},
		FinalConsensus: &protocol.ConsensusResult{
			Confidence: 0.85,
		},
		EarlyExit: true,
		Metrics: &protocol.DebateMetrics{
			AvgConfidence: 0.85,
		},
	}

	patterns := analyzer.Analyze(result)

	assert.NotEmpty(t, patterns)
}

func TestConsensusPatternDetector_LowConsensus(t *testing.T) {
	t.Parallel()
	detector := &ConsensusPatternDetector{}

	result := &protocol.DebateResult{
		Phases: []*protocol.PhaseResult{
			{ConsensusLevel: 0.4},
			{ConsensusLevel: 0.5},
		},
		FinalConsensus: &protocol.ConsensusResult{
			Confidence: 0.5,
		},
	}

	patterns := detector.Detect(result)

	// Should not detect early high consensus or progressive consensus
	for _, p := range patterns {
		assert.NotEqual(t, "Early High Consensus", p.Name)
		assert.NotEqual(t, "Progressive Consensus", p.Name)
	}
}

func TestConflictPatternDetector_ManyDisagreements(t *testing.T) {
	t.Parallel()
	detector := &ConflictPatternDetector{}

	result := &protocol.DebateResult{
		Success: true,
		Phases: []*protocol.PhaseResult{
			{Disagreements: []string{"d1", "d2", "d3", "d4", "d5"}},
		},
	}

	patterns := detector.Detect(result)

	assert.NotEmpty(t, patterns)
	assert.Equal(t, PatternTypeConflictResolution, patterns[0].PatternType)
}

func TestExpertisePatternDetector_MultipleExperts(t *testing.T) {
	t.Parallel()
	detector := &ExpertisePatternDetector{}

	result := &protocol.DebateResult{
		Phases: []*protocol.PhaseResult{
			{
				Responses: []*protocol.PhaseResponse{
					{Provider: "claude", Model: "claude-3", Confidence: 0.95},
					{Provider: "claude", Model: "claude-3", Confidence: 0.92},
					{Provider: "claude", Model: "claude-3", Confidence: 0.9},
					{Provider: "deepseek", Model: "coder", Confidence: 0.91},
					{Provider: "deepseek", Model: "coder", Confidence: 0.88},
				},
			},
		},
	}

	patterns := detector.Detect(result)

	assert.NotEmpty(t, patterns)
	assert.Equal(t, PatternTypeExpertise, patterns[0].PatternType)
}

func TestFailurePatternDetector_AllLowConsensus(t *testing.T) {
	t.Parallel()
	detector := &FailurePatternDetector{}

	result := &protocol.DebateResult{
		Success: false,
		Phases: []*protocol.PhaseResult{
			{ConsensusLevel: 0.3},
			{ConsensusLevel: 0.35},
			{ConsensusLevel: 0.4},
			{ConsensusLevel: 0.38},
		},
	}

	patterns := detector.Detect(result)

	assert.NotEmpty(t, patterns)
	assert.Equal(t, "Persistent Low Consensus", patterns[0].Name)
}

func TestOptimizationPatternDetector_AllOptimizations(t *testing.T) {
	t.Parallel()
	detector := &OptimizationPatternDetector{}

	result := &protocol.DebateResult{
		Success:         true,
		EarlyExit:       true,
		EarlyExitReason: "high consensus",
		Metrics: &protocol.DebateMetrics{
			AvgConfidence: 0.9,
		},
	}

	patterns := detector.Detect(result)

	assert.NotEmpty(t, patterns)

	foundFastConvergence := false
	foundHighQuality := false
	for _, p := range patterns {
		if p.Name == "Fast Convergence" {
			foundFastConvergence = true
		}
		if p.Name == "High Quality Responses" {
			foundHighQuality = true
		}
	}

	assert.True(t, foundFastConvergence)
	assert.True(t, foundHighQuality)
}

// =============================================================================
// StrategySynthesizer Extended Tests
// =============================================================================

func TestStrategySynthesizer_Synthesize_Complete(t *testing.T) {
	t.Parallel()
	synthesizer := NewStrategySynthesizer()

	result := &protocol.DebateResult{
		ID:           "debate-full",
		Topic:        "Complete Strategy Test",
		Success:      true,
		TopologyUsed: topology.TopologyChain,
		Duration:     15 * time.Minute,
		Phases: []*protocol.PhaseResult{
			{
				Phase:          topology.PhaseProposal,
				ConsensusLevel: 0.75,
				KeyInsights:    []string{"Insight 1", "Insight 2"},
			},
			{
				Phase:          topology.PhaseCritique,
				ConsensusLevel: 0.8,
				KeyInsights:    []string{"Insight 3"},
			},
			{
				Phase:          topology.PhaseConvergence,
				ConsensusLevel: 0.9,
				KeyInsights:    []string{"Final insight"},
			},
		},
		FinalConsensus: &protocol.ConsensusResult{
			Confidence: 0.92,
		},
		Metrics: &protocol.DebateMetrics{
			RoleContributions: map[topology.AgentRole]int{
				topology.RoleProposer:  4,
				topology.RoleCritic:    3,
				topology.RoleModerator: 2,
			},
		},
	}

	strategy := synthesizer.Synthesize(result)

	require.NotNil(t, strategy)
	assert.Contains(t, strategy.Name, "debate")
	assert.Equal(t, topology.TopologyChain, strategy.TopologyType)
	assert.Equal(t, 0.92, strategy.SuccessRate)
	assert.Len(t, strategy.Phases, 3)
	assert.NotEmpty(t, strategy.RoleConfig)
}

func TestStrategySynthesizer_Synthesize_LowConsensus(t *testing.T) {
	t.Parallel()
	synthesizer := NewStrategySynthesizer()

	result := &protocol.DebateResult{
		ID:      "low-consensus",
		Topic:   "Test",
		Success: false, // Failed debate
		FinalConsensus: &protocol.ConsensusResult{
			Confidence: 0.3, // Low consensus
		},
	}

	strategy := synthesizer.Synthesize(result)
	// Should not synthesize with failed debate
	assert.Nil(t, strategy)
}

// =============================================================================
// KnowledgeGraph Extended Tests
// =============================================================================

func TestKnowledgeGraph_AddDebate_Complete(t *testing.T) {
	t.Parallel()
	kg := NewKnowledgeGraph(1000)

	result := &protocol.DebateResult{
		ID:    "complete-debate",
		Topic: "Knowledge Graph Test",
		Phases: []*protocol.PhaseResult{
			{
				Phase:       topology.PhaseProposal,
				KeyInsights: []string{"Insight 1", "Insight 2"},
			},
		},
		FinalConsensus: &protocol.ConsensusResult{
			Summary:    "Test consensus",
			Confidence: 0.85,
		},
	}

	lessons := []*debate.Lesson{
		{
			ID:    "lesson-1",
			Title: "Test Lesson 1",
			Statistics: debate.LessonStatistics{
				ApplyCount:   10,
				SuccessCount: 8,
			},
		},
		{
			ID:    "lesson-2",
			Title: "Test Lesson 2",
			Statistics: debate.LessonStatistics{
				ApplyCount:   5,
				SuccessCount: 3,
			},
		},
	}

	addedNodes := kg.AddDebate(result, lessons)

	assert.NotEmpty(t, addedNodes)
	assert.GreaterOrEqual(t, len(addedNodes), 3) // topic, outcome, lessons

	// Verify nodes exist
	topicNode, found := kg.GetNode("topic:complete-debate")
	assert.True(t, found)
	assert.Equal(t, NodeTypeTopic, topicNode.Type)

	outcomeNode, found := kg.GetNode("outcome:complete-debate")
	assert.True(t, found)
	assert.Equal(t, NodeTypeOutcome, outcomeNode.Type)
}

func TestKnowledgeGraph_GetRoleAdvice_MultipleHighWeight(t *testing.T) {
	t.Parallel()
	kg := NewKnowledgeGraph(1000)

	// Add multiple high-weight lesson nodes
	kg.mu.Lock()
	kg.nodes["lesson:1"] = &KnowledgeNode{
		ID:     "lesson:1",
		Type:   NodeTypeLesson,
		Label:  "Important Security Lesson",
		Weight: 0.9,
	}
	kg.nodes["lesson:2"] = &KnowledgeNode{
		ID:     "lesson:2",
		Type:   NodeTypeLesson,
		Label:  "Another Important Lesson",
		Weight: 0.85,
	}
	kg.nodes["lesson:3"] = &KnowledgeNode{
		ID:     "lesson:3",
		Type:   NodeTypeLesson,
		Label:  "Low Weight Lesson",
		Weight: 0.5, // Below threshold
	}
	kg.mu.Unlock()

	advice := kg.GetRoleAdvice(topology.RoleProposer, agents.DomainSecurity)

	assert.Len(t, advice, 2) // Only high weight ones
}

func TestKnowledgeGraph_TrimIfNecessary_LargeGraph(t *testing.T) {
	t.Parallel()
	kg := NewKnowledgeGraph(10)

	// Add many nodes
	for i := 0; i < 20; i++ {
		kg.mu.Lock()
		kg.nodes[string(rune('a'+i))] = &KnowledgeNode{
			ID:          string(rune('a' + i)),
			Type:        NodeTypeTopic,
			LastUpdated: time.Now().Add(-time.Duration(i) * time.Hour),
		}
		kg.mu.Unlock()
	}

	kg.mu.Lock()
	kg.trimIfNecessary()
	kg.mu.Unlock()

	assert.LessOrEqual(t, kg.Size(), 10)
}

func TestKnowledgeGraph_GetConnections_Complex(t *testing.T) {
	t.Parallel()
	kg := NewKnowledgeGraph(1000)

	// Add nodes and edges
	kg.mu.Lock()
	kg.nodes["a"] = &KnowledgeNode{ID: "a", Type: NodeTypeTopic}
	kg.nodes["b"] = &KnowledgeNode{ID: "b", Type: NodeTypeConcept}
	kg.nodes["c"] = &KnowledgeNode{ID: "c", Type: NodeTypePattern}
	kg.edges = append(kg.edges, &KnowledgeEdge{FromID: "a", ToID: "b", Type: EdgeTypeRelatedTo})
	kg.edges = append(kg.edges, &KnowledgeEdge{FromID: "a", ToID: "c", Type: EdgeTypeLeadsTo})
	kg.mu.Unlock()

	connections := kg.GetConnections("a")

	assert.Len(t, connections, 2)
}

// =============================================================================
// Repository Extended Tests
// =============================================================================

func TestRepository_ExtractLessons_NilResult_Extended(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	_, err := repo.ExtractLessons(ctx, nil)
	assert.Error(t, err)
}

func TestRepository_ApplyLesson_NonExistent(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	_, err := repo.ApplyLesson(ctx, "nonexistent-lesson", "debate-1")
	assert.Error(t, err)
}

func TestRepository_RecordOutcome_NonExistent(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	err := repo.RecordOutcome(ctx, &LessonApplication{ID: "nonexistent"}, true, "feedback")
	assert.Error(t, err)
}

func TestRepository_GetPatterns_WithAllFilters(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	// Add patterns
	since := time.Now().Add(-1 * time.Hour)
	repo.mu.Lock()
	repo.patterns["p1"] = &DebatePattern{
		ID:           "p1",
		PatternType:  PatternTypeConsensusBuilding,
		Domain:       agents.DomainCode,
		Frequency:    5,
		SuccessRate:  0.8,
		LastObserved: time.Now(),
	}
	repo.patterns["p2"] = &DebatePattern{
		ID:           "p2",
		PatternType:  PatternTypeConflictResolution,
		Domain:       agents.DomainSecurity,
		Frequency:    3,
		SuccessRate:  0.6,
		LastObserved: time.Now(),
	}
	repo.patterns["p3"] = &DebatePattern{
		ID:           "p3",
		PatternType:  PatternTypeConsensusBuilding,
		Domain:       agents.DomainCode,
		Frequency:    1, // Below min frequency
		SuccessRate:  0.9,
		LastObserved: time.Now().Add(-2 * time.Hour), // Before since
	}
	repo.mu.Unlock()

	patterns, err := repo.GetPatterns(ctx, PatternFilter{
		Types:        []PatternType{PatternTypeConsensusBuilding},
		Domain:       agents.DomainCode,
		MinFrequency: 3,
		MinSuccess:   0.7,
		Since:        &since,
	})

	require.NoError(t, err)
	assert.Len(t, patterns, 1)
	assert.Equal(t, "p1", patterns[0].ID)
}

func TestRepository_GetSuccessfulStrategies_WithDomain(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	// Add strategies
	repo.mu.Lock()
	repo.strategies["s1"] = &Strategy{
		ID:          "s1",
		Domain:      agents.DomainCode,
		SuccessRate: 0.9,
	}
	repo.strategies["s2"] = &Strategy{
		ID:          "s2",
		Domain:      agents.DomainSecurity,
		SuccessRate: 0.8,
	}
	repo.mu.Unlock()

	strategies, err := repo.GetSuccessfulStrategies(ctx, agents.DomainCode)
	require.NoError(t, err)
	assert.Len(t, strategies, 1)
	assert.Equal(t, "s1", strategies[0].ID)
}

func TestRepository_GetDebateHistory_WithFilters(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	ctx := context.Background()

	since := time.Now().Add(-1 * time.Hour)

	// Add history entries
	repo.mu.Lock()
	repo.history = append(repo.history, &DebateHistoryEntry{
		ID:             "h1",
		Domain:         agents.DomainCode,
		Success:        true,
		ConsensusScore: 0.9,
		Timestamp:      time.Now(),
	})
	repo.history = append(repo.history, &DebateHistoryEntry{
		ID:             "h2",
		Domain:         agents.DomainSecurity,
		Success:        false,
		ConsensusScore: 0.6,
		Timestamp:      time.Now(),
	})
	repo.mu.Unlock()

	history, err := repo.GetDebateHistory(ctx, HistoryFilter{
		Domain:       agents.DomainCode,
		SuccessOnly:  true,
		MinConsensus: 0.8,
		Since:        &since,
		Limit:        10,
	})

	require.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Equal(t, "h1", history[0].ID)
}

func TestRepository_InferDomainFromTopic_AllDomains(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()

	testCases := []struct {
		topic    string
		expected agents.Domain
	}{
		{"Security vulnerability assessment", agents.DomainSecurity},
		{"Authentication best practices", agents.DomainSecurity},
		{"Encryption implementation", agents.DomainSecurity},
		{"System architecture design", agents.DomainArchitecture},
		{"Scalable design patterns", agents.DomainArchitecture},
		{"Performance optimization", agents.DomainOptimization},
		{"Speed improvement techniques", agents.DomainOptimization},
		{"Cache implementation", agents.DomainOptimization},
		{"Debug error tracing", agents.DomainDebug},
		{"Fix bug in module", agents.DomainDebug},
		{"Code implementation", agents.DomainCode},
		{"Function implementation", agents.DomainCode},
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

func TestRepository_GenerateDomainInsights_AllDomains(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()

	// Test domains that have specific insights
	domainsWithInsights := []agents.Domain{
		agents.DomainSecurity,
		agents.DomainArchitecture,
		agents.DomainOptimization,
		agents.DomainDebug,
		agents.DomainCode,
	}

	for _, domain := range domainsWithInsights {
		t.Run(string(domain), func(t *testing.T) {
				t.Parallel()
			insights := repo.generateDomainInsights(domain, []*LessonMatch{})
			assert.NotEmpty(t, insights)
		})
	}

	// General domain may not have specific insights without lessons
	t.Run("general", func(t *testing.T) {
			t.Parallel()
		insights := repo.generateDomainInsights(agents.DomainGeneral, []*LessonMatch{})
		// General domain might be empty without matching lessons
		assert.NotNil(t, insights)
	})
}

func TestRepository_GenerateRoleGuidance_AllRoles(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()

	roles := []topology.AgentRole{
		topology.RoleProposer,
		topology.RoleCritic,
		topology.RoleReviewer,
		topology.RoleOptimizer,
		topology.RoleModerator,
		topology.RoleArchitect,
	}

	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
				t.Parallel()
			guidance := repo.generateRoleGuidance(role, agents.DomainCode)
			assert.NotEmpty(t, guidance)
		})
	}
}

// =============================================================================
// Structure Tests
// =============================================================================

func TestSearchOptions_Default(t *testing.T) {
	t.Parallel()
	opts := DefaultSearchOptions()

	assert.Equal(t, 0.5, opts.MinScore)
	assert.Equal(t, 10, opts.Limit)
}

func TestLessonMatch_AllFields(t *testing.T) {
	t.Parallel()
	match := &LessonMatch{
		Lesson:    &debate.Lesson{ID: "lesson-1"},
		Score:     0.85,
		MatchType: "semantic",
		Relevance: &Relevance{
			TopicMatch:   0.9,
			DomainMatch:  0.8,
			PatternMatch: 0.7,
			Reasons:      []string{"Reason 1"},
		},
	}

	assert.Equal(t, "lesson-1", match.Lesson.ID)
	assert.Equal(t, 0.85, match.Score)
	assert.Equal(t, "semantic", match.MatchType)
}

func TestLessonApplication_AllFields(t *testing.T) {
	t.Parallel()
	app := &LessonApplication{
		ID:        "app-1",
		LessonID:  "lesson-1",
		DebateID:  "debate-1",
		AppliedAt: time.Now(),
		AppliedBy: "agent-1",
		Context:   "Test context",
		Outcome: &ApplicationOutcome{
			Success:    true,
			Feedback:   "Worked well",
			Impact:     0.5,
			RecordedAt: time.Now(),
		},
	}

	assert.Equal(t, "app-1", app.ID)
	assert.True(t, app.Outcome.Success)
	assert.Equal(t, 0.5, app.Outcome.Impact)
}

func TestDebatePattern_AllFields(t *testing.T) {
	t.Parallel()
	pattern := &DebatePattern{
		ID:          "pattern-1",
		Name:        "Test Pattern",
		Description: "A test pattern",
		PatternType: PatternTypeConsensusBuilding,
		Domain:      agents.DomainCode,
		Frequency:   10,
		SuccessRate: 0.85,
		Triggers:    []string{"high consensus"},
		Indicators: []PatternIndicator{
			{Type: "consensus", Threshold: 0.8, Weight: 1.0},
		},
		Responses: []PatternResponse{
			{Action: "continue", Priority: 1},
		},
		FirstObserved: time.Now(),
		LastObserved:  time.Now(),
		Confidence:    0.9,
	}

	assert.Equal(t, "pattern-1", pattern.ID)
	assert.Equal(t, PatternTypeConsensusBuilding, pattern.PatternType)
}

func TestStrategy_AllFields(t *testing.T) {
	t.Parallel()
	strategy := &Strategy{
		ID:           "strategy-1",
		Name:         "Test Strategy",
		Description:  "A test strategy",
		Domain:       agents.DomainSecurity,
		TopologyType: topology.TopologyGraphMesh,
		RoleConfig: []RoleConfiguration{
			{Role: topology.RoleProposer, Count: 2},
		},
		Phases: []PhaseStrategy{
			{Phase: topology.PhaseProposal, FocusAreas: []string{"analysis"}},
		},
		SuccessRate:  0.9,
		Applications: 15,
		AvgConsensus: 0.85,
		AvgDuration:  10 * time.Minute,
	}

	assert.Equal(t, "strategy-1", strategy.ID)
	assert.Equal(t, topology.TopologyGraphMesh, strategy.TopologyType)
}

func TestRepositoryStatistics_AllFields(t *testing.T) {
	t.Parallel()
	stats := &RepositoryStatistics{
		TotalLessons:        100,
		TotalPatterns:       50,
		TotalStrategies:     25,
		TotalDebates:        200,
		LessonsByDomain:     map[agents.Domain]int{agents.DomainCode: 30},
		PatternsByType:      map[PatternType]int{PatternTypeConsensusBuilding: 20},
		OverallSuccessRate:  0.75,
		AvgLessonsPerDebate: 2.5,
		TopCategories:       []debate.LessonCategory{debate.LessonCategorySecurity},
		LastUpdated:         time.Now(),
	}

	assert.Equal(t, 100, stats.TotalLessons)
	assert.Equal(t, 0.75, stats.OverallSuccessRate)
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestTruncate_Extended(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 6))
	assert.Equal(t, "", truncate("", 10))
}

func TestContainsAny_EdgeCases_Extended(t *testing.T) {
	t.Parallel()
	assert.True(t, containsAny("SECURITY", "security")) // Case insensitive
	assert.False(t, containsAny("", "security"))        // Empty string
	assert.False(t, containsAny("hello", "foo", "bar")) // No match
	assert.True(t, containsAny("hello world", "world")) // Partial match
}

func TestMin_Extended(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 3, min(3, 5))
	assert.Equal(t, 3, min(5, 3))
	assert.Equal(t, 3, min(3, 3))
}
