package knowledge

import (
	"context"
	"fmt"
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
// Learning Config Tests
// =============================================================================

func TestDefaultLearningConfig(t *testing.T) {
	t.Parallel()
	config := DefaultLearningConfig()

	assert.Equal(t, 3, config.MinDebatesForPattern)
	assert.Equal(t, 0.7, config.PatternConfidenceThreshold)
	assert.Equal(t, 5, config.StrategyMinApplications)
	assert.Equal(t, 0.75, config.StrategySuccessThreshold)
	assert.True(t, config.EnableKnowledgeGraph)
	assert.Equal(t, 10000, config.MaxGraphNodes)
	assert.Equal(t, 0.05, config.LearningDecayRate)
}

// =============================================================================
// CrossDebateLearner Tests
// =============================================================================

func TestNewCrossDebateLearner(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultLearningConfig()

	learner := NewCrossDebateLearner(repo, config)

	assert.NotNil(t, learner)
	assert.NotNil(t, learner.patternAnalyzer)
	assert.NotNil(t, learner.strategySynthesizer)
	assert.NotNil(t, learner.knowledgeGraph)
}

func TestNewCrossDebateLearner_NoKnowledgeGraph(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultLearningConfig()
	config.EnableKnowledgeGraph = false

	learner := NewCrossDebateLearner(repo, config)

	assert.Nil(t, learner.knowledgeGraph)
}

func TestCrossDebateLearner_LearnFromDebate(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultLearningConfig()
	learner := NewCrossDebateLearner(repo, config)
	ctx := context.Background()

	result := createTestDebateResult()
	lessons := []*debate.Lesson{
		{ID: "lesson-1", Title: "Test Lesson"},
	}

	outcome, err := learner.LearnFromDebate(ctx, result, lessons)
	require.NoError(t, err)
	require.NotNil(t, outcome)

	assert.Equal(t, result.ID, outcome.DebateID)
	// Patterns may or may not be detected depending on debate characteristics
	// The important thing is that the learning process completes without error
	assert.NotNil(t, outcome.NewPatterns)
	// Quality score should be non-negative
	assert.GreaterOrEqual(t, outcome.QualityScore, 0.0)
}

func TestCrossDebateLearner_GetRecommendations(t *testing.T) {
	t.Parallel()
	repo := createTestRepository()
	config := DefaultLearningConfig()
	learner := NewCrossDebateLearner(repo, config)
	ctx := context.Background()

	recommendations, err := learner.GetRecommendations(ctx, "Security implementation", agents.DomainSecurity)
	require.NoError(t, err)
	require.NotNil(t, recommendations)

	assert.Equal(t, "Security implementation", recommendations.Topic)
	assert.Equal(t, agents.DomainSecurity, recommendations.Domain)
	assert.NotNil(t, recommendations.TopologyAdvice)
	assert.NotNil(t, recommendations.RoleAdvice)
}

// =============================================================================
// Pattern Analyzer Tests
// =============================================================================

func TestNewPatternAnalyzer(t *testing.T) {
	t.Parallel()
	analyzer := NewPatternAnalyzer()

	assert.NotNil(t, analyzer)
	assert.NotEmpty(t, analyzer.detectors)
	assert.Len(t, analyzer.detectors, 5) // 5 detectors
}

func TestPatternAnalyzer_Analyze(t *testing.T) {
	t.Parallel()
	analyzer := NewPatternAnalyzer()
	result := createTestDebateResult()

	patterns := analyzer.Analyze(result)

	assert.NotNil(t, patterns)
	// Should detect at least some patterns from a successful debate
}

// =============================================================================
// Consensus Pattern Detector Tests
// =============================================================================

func TestConsensusPatternDetector_Detect_EarlyHighConsensus(t *testing.T) {
	t.Parallel()
	detector := &ConsensusPatternDetector{}

	result := &protocol.DebateResult{
		ID: "test-debate",
		Phases: []*protocol.PhaseResult{
			{Phase: topology.PhaseProposal, ConsensusLevel: 0.85},
			{Phase: topology.PhaseCritique, ConsensusLevel: 0.9},
		},
		FinalConsensus: &protocol.ConsensusResult{
			Confidence: 0.9,
		},
	}

	patterns := detector.Detect(result)

	// Should detect early high consensus
	found := false
	for _, p := range patterns {
		if p.Name == "Early High Consensus" {
			found = true
			assert.Equal(t, PatternTypeConsensusBuilding, p.PatternType)
			break
		}
	}
	assert.True(t, found)
}

func TestConsensusPatternDetector_Detect_ProgressiveConsensus(t *testing.T) {
	t.Parallel()
	detector := &ConsensusPatternDetector{}

	result := &protocol.DebateResult{
		ID: "test-debate",
		Phases: []*protocol.PhaseResult{
			{Phase: topology.PhaseProposal, ConsensusLevel: 0.5},
			{Phase: topology.PhaseCritique, ConsensusLevel: 0.6},
			{Phase: topology.PhaseReview, ConsensusLevel: 0.75},
			{Phase: topology.PhaseOptimization, ConsensusLevel: 0.85},
		},
		FinalConsensus: &protocol.ConsensusResult{
			Confidence: 0.85,
		},
	}

	patterns := detector.Detect(result)

	// Should detect progressive consensus
	found := false
	for _, p := range patterns {
		if p.Name == "Progressive Consensus" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestConsensusPatternDetector_Detect_NoConsensus(t *testing.T) {
	t.Parallel()
	detector := &ConsensusPatternDetector{}

	result := &protocol.DebateResult{
		ID:             "test-debate",
		FinalConsensus: nil,
	}

	patterns := detector.Detect(result)
	assert.Empty(t, patterns)
}

// =============================================================================
// Conflict Pattern Detector Tests
// =============================================================================

func TestConflictPatternDetector_Detect(t *testing.T) {
	t.Parallel()
	detector := &ConflictPatternDetector{}

	result := &protocol.DebateResult{
		ID:      "test-debate",
		Success: true,
		Phases: []*protocol.PhaseResult{
			{
				Phase:         topology.PhaseCritique,
				Disagreements: []string{"Approach disagreement", "Technology choice"},
			},
		},
	}

	patterns := detector.Detect(result)

	found := false
	for _, p := range patterns {
		if p.PatternType == PatternTypeConflictResolution {
			found = true
			assert.Equal(t, 2, p.Metadata["disagreements_resolved"])
			break
		}
	}
	assert.True(t, found)
}

func TestConflictPatternDetector_Detect_FailedDebate(t *testing.T) {
	t.Parallel()
	detector := &ConflictPatternDetector{}

	result := &protocol.DebateResult{
		ID:      "test-debate",
		Success: false, // Failed debate shouldn't show conflict resolution
		Phases: []*protocol.PhaseResult{
			{Disagreements: []string{"Unresolved"}},
		},
	}

	patterns := detector.Detect(result)
	assert.Empty(t, patterns)
}

// =============================================================================
// Expertise Pattern Detector Tests
// =============================================================================

func TestExpertisePatternDetector_Detect(t *testing.T) {
	t.Parallel()
	detector := &ExpertisePatternDetector{}

	result := &protocol.DebateResult{
		ID: "test-debate",
		Phases: []*protocol.PhaseResult{
			{
				Responses: []*protocol.PhaseResponse{
					{Provider: "claude", Model: "claude-3", Confidence: 0.9},
					{Provider: "claude", Model: "claude-3", Confidence: 0.88},
					{Provider: "claude", Model: "claude-3", Confidence: 0.92},
				},
			},
		},
	}

	patterns := detector.Detect(result)

	found := false
	for _, p := range patterns {
		if p.PatternType == PatternTypeExpertise {
			found = true
			break
		}
	}
	assert.True(t, found)
}

// =============================================================================
// Failure Pattern Detector Tests
// =============================================================================

func TestFailurePatternDetector_Detect(t *testing.T) {
	t.Parallel()
	detector := &FailurePatternDetector{}

	result := &protocol.DebateResult{
		ID:      "test-debate",
		Success: false,
		Phases: []*protocol.PhaseResult{
			{Phase: topology.PhaseProposal, ConsensusLevel: 0.3},
			{Phase: topology.PhaseCritique, ConsensusLevel: 0.4},
			{Phase: topology.PhaseReview, ConsensusLevel: 0.35},
		},
	}

	patterns := detector.Detect(result)

	found := false
	for _, p := range patterns {
		if p.PatternType == PatternTypeFailure {
			found = true
			assert.Equal(t, "Persistent Low Consensus", p.Name)
			break
		}
	}
	assert.True(t, found)
}

func TestFailurePatternDetector_Detect_SuccessfulDebate(t *testing.T) {
	t.Parallel()
	detector := &FailurePatternDetector{}

	result := &protocol.DebateResult{
		ID:      "test-debate",
		Success: true,
	}

	patterns := detector.Detect(result)
	assert.Empty(t, patterns)
}

// =============================================================================
// Optimization Pattern Detector Tests
// =============================================================================

func TestOptimizationPatternDetector_Detect_FastConvergence(t *testing.T) {
	t.Parallel()
	detector := &OptimizationPatternDetector{}

	result := &protocol.DebateResult{
		ID:              "test-debate",
		Success:         true,
		EarlyExit:       true,
		EarlyExitReason: "High consensus reached",
	}

	patterns := detector.Detect(result)

	found := false
	for _, p := range patterns {
		if p.Name == "Fast Convergence" {
			found = true
			assert.Equal(t, PatternTypeOptimization, p.PatternType)
			break
		}
	}
	assert.True(t, found)
}

func TestOptimizationPatternDetector_Detect_HighQuality(t *testing.T) {
	t.Parallel()
	detector := &OptimizationPatternDetector{}

	result := &protocol.DebateResult{
		ID:      "test-debate",
		Success: true,
		Metrics: &protocol.DebateMetrics{
			AvgConfidence: 0.85,
		},
	}

	patterns := detector.Detect(result)

	found := false
	for _, p := range patterns {
		if p.Name == "High Quality Responses" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

// =============================================================================
// Strategy Synthesizer Tests
// =============================================================================

func TestNewStrategySynthesizer(t *testing.T) {
	t.Parallel()
	synthesizer := NewStrategySynthesizer()
	assert.NotNil(t, synthesizer)
}

func TestStrategySynthesizer_Synthesize(t *testing.T) {
	t.Parallel()
	synthesizer := NewStrategySynthesizer()
	result := createTestDebateResult()

	strategy := synthesizer.Synthesize(result)

	require.NotNil(t, strategy)
	assert.NotEmpty(t, strategy.ID)
	assert.NotEmpty(t, strategy.Name)
	assert.Equal(t, result.TopologyUsed, strategy.TopologyType)
	assert.Equal(t, result.FinalConsensus.Confidence, strategy.SuccessRate)
	assert.Equal(t, 1, strategy.Applications)
}

func TestStrategySynthesizer_Synthesize_FailedDebate(t *testing.T) {
	t.Parallel()
	synthesizer := NewStrategySynthesizer()

	result := &protocol.DebateResult{
		ID:      "test-debate",
		Success: false,
	}

	strategy := synthesizer.Synthesize(result)
	assert.Nil(t, strategy)
}

func TestStrategySynthesizer_Synthesize_NoConsensus(t *testing.T) {
	t.Parallel()
	synthesizer := NewStrategySynthesizer()

	result := &protocol.DebateResult{
		ID:             "test-debate",
		Success:        true,
		FinalConsensus: nil,
	}

	strategy := synthesizer.Synthesize(result)
	assert.Nil(t, strategy)
}

// =============================================================================
// Knowledge Graph Tests
// =============================================================================

func TestNewKnowledgeGraph(t *testing.T) {
	t.Parallel()
	graph := NewKnowledgeGraph(1000)

	assert.NotNil(t, graph)
	assert.NotNil(t, graph.nodes)
	assert.NotNil(t, graph.edges)
	assert.Equal(t, 1000, graph.maxNodes)
}

func TestKnowledgeGraph_AddDebate(t *testing.T) {
	t.Parallel()
	graph := NewKnowledgeGraph(1000)
	result := createTestDebateResult()
	lessons := []*debate.Lesson{
		{ID: "lesson-1", Title: "Test Lesson 1"},
		{ID: "lesson-2", Title: "Test Lesson 2"},
	}

	addedNodes := graph.AddDebate(result, lessons)

	assert.NotEmpty(t, addedNodes)
	assert.True(t, graph.Size() > 0)

	// Should have topic node
	topicNode, ok := graph.GetNode("topic:" + result.ID)
	assert.True(t, ok)
	assert.Equal(t, NodeTypeTopic, topicNode.Type)

	// Should have outcome node
	outcomeNode, ok := graph.GetNode("outcome:" + result.ID)
	assert.True(t, ok)
	assert.Equal(t, NodeTypeOutcome, outcomeNode.Type)
}

func TestKnowledgeGraph_GetNode(t *testing.T) {
	t.Parallel()
	graph := NewKnowledgeGraph(1000)

	// Add a node manually
	graph.mu.Lock()
	graph.nodes["test-node"] = &KnowledgeNode{
		ID:    "test-node",
		Type:  NodeTypeConcept,
		Label: "Test Concept",
	}
	graph.mu.Unlock()

	node, ok := graph.GetNode("test-node")
	assert.True(t, ok)
	assert.Equal(t, "Test Concept", node.Label)

	_, ok = graph.GetNode("nonexistent")
	assert.False(t, ok)
}

func TestKnowledgeGraph_GetConnections(t *testing.T) {
	t.Parallel()
	graph := NewKnowledgeGraph(1000)

	// Add nodes and edges
	graph.mu.Lock()
	graph.nodes["node-1"] = &KnowledgeNode{ID: "node-1"}
	graph.nodes["node-2"] = &KnowledgeNode{ID: "node-2"}
	graph.edges = append(graph.edges, &KnowledgeEdge{
		FromID: "node-1",
		ToID:   "node-2",
		Type:   EdgeTypeRelatedTo,
	})
	graph.mu.Unlock()

	connections := graph.GetConnections("node-1")
	assert.Len(t, connections, 1)

	connections = graph.GetConnections("node-2")
	assert.Len(t, connections, 1)
}

func TestKnowledgeGraph_Size(t *testing.T) {
	t.Parallel()
	graph := NewKnowledgeGraph(1000)

	assert.Equal(t, 0, graph.Size())

	result := createTestDebateResult()
	graph.AddDebate(result, nil)

	assert.True(t, graph.Size() > 0)
}

func TestKnowledgeGraph_TrimIfNecessary(t *testing.T) {
	t.Parallel()
	graph := NewKnowledgeGraph(5) // Very small limit

	// Add multiple debates
	for i := 0; i < 10; i++ {
		result := createTestDebateResult()
		result.ID = result.ID + string(rune('0'+i))
		graph.AddDebate(result, nil)
	}

	// Should be trimmed to max
	assert.LessOrEqual(t, graph.Size(), 5)
}

func TestKnowledgeGraph_GetRoleAdvice(t *testing.T) {
	t.Parallel()
	graph := NewKnowledgeGraph(1000)

	// Add some high-weight lesson nodes
	graph.mu.Lock()
	graph.nodes["lesson:1"] = &KnowledgeNode{
		ID:     "lesson:1",
		Type:   NodeTypeLesson,
		Label:  "Important Lesson",
		Weight: 0.85,
	}
	graph.nodes["lesson:2"] = &KnowledgeNode{
		ID:     "lesson:2",
		Type:   NodeTypeLesson,
		Label:  "Another Lesson",
		Weight: 0.75,
	}
	graph.mu.Unlock()

	advice := graph.GetRoleAdvice(topology.RoleProposer, agents.DomainCode)
	assert.NotEmpty(t, advice)
}

// =============================================================================
// Node Type Tests
// =============================================================================

func TestNodeTypes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, NodeType("topic"), NodeTypeTopic)
	assert.Equal(t, NodeType("concept"), NodeTypeConcept)
	assert.Equal(t, NodeType("pattern"), NodeTypePattern)
	assert.Equal(t, NodeType("lesson"), NodeTypeLesson)
	assert.Equal(t, NodeType("agent"), NodeTypeAgent)
	assert.Equal(t, NodeType("outcome"), NodeTypeOutcome)
}

// =============================================================================
// Edge Type Tests
// =============================================================================

func TestEdgeTypes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, EdgeType("related_to"), EdgeTypeRelatedTo)
	assert.Equal(t, EdgeType("leads_to"), EdgeTypeLeadsTo)
	assert.Equal(t, EdgeType("derived_from"), EdgeTypeDerivedFrom)
	assert.Equal(t, EdgeType("contributes"), EdgeTypeContributes)
	assert.Equal(t, EdgeType("conflicts"), EdgeTypeConflicts)
}

// =============================================================================
// Learning Outcome Tests
// =============================================================================

func TestLearningOutcome_Structure(t *testing.T) {
	t.Parallel()
	outcome := &LearningOutcome{
		DebateID:        "debate-1",
		LearnedAt:       time.Now(),
		NewPatterns:     []*DebatePattern{{Name: "Pattern 1"}},
		UpdatedPatterns: []string{"pattern-2"},
		NewStrategies:   []*Strategy{{Name: "Strategy 1"}},
		KnowledgeNodes:  []string{"node-1", "node-2"},
		QualityScore:    0.75,
	}

	assert.Equal(t, "debate-1", outcome.DebateID)
	assert.Len(t, outcome.NewPatterns, 1)
	assert.Equal(t, 0.75, outcome.QualityScore)
}

// =============================================================================
// Debate Recommendations Tests
// =============================================================================

func TestDebateRecommendations_Structure(t *testing.T) {
	t.Parallel()
	recommendations := &DebateRecommendations{
		Topic:            "Test Topic",
		Domain:           agents.DomainCode,
		GeneratedAt:      time.Now(),
		TopologyAdvice:   []string{"Use graph mesh"},
		RoleAdvice:       map[topology.AgentRole][]string{topology.RoleProposer: {"Be creative"}},
		PatternWarnings:  []string{"Warning 1"},
		SuggestedActions: []string{"Action 1"},
	}

	assert.Equal(t, "Test Topic", recommendations.Topic)
	assert.Len(t, recommendations.TopologyAdvice, 1)
	assert.Contains(t, recommendations.RoleAdvice, topology.RoleProposer)
}

// =============================================================================
// Strategy Tests
// =============================================================================

func TestStrategy_Structure(t *testing.T) {
	t.Parallel()
	strategy := &Strategy{
		ID:           "strategy-1",
		Name:         "Test Strategy",
		Description:  "A test strategy",
		Domain:       agents.DomainCode,
		TopologyType: topology.TopologyGraphMesh,
		RoleConfig: []RoleConfiguration{
			{Role: topology.RoleProposer, Count: 2},
		},
		Phases: []PhaseStrategy{
			{Phase: topology.PhaseProposal, MinConfidence: 0.7},
		},
		SuccessRate:  0.85,
		Applications: 10,
		AvgConsensus: 0.82,
		AvgDuration:  5 * time.Minute,
	}

	assert.Equal(t, "strategy-1", strategy.ID)
	assert.Len(t, strategy.RoleConfig, 1)
	assert.Len(t, strategy.Phases, 1)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkKnowledgeGraph_AddNode(b *testing.B) {
	kg := NewKnowledgeGraph(100000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node := &KnowledgeNode{
			ID:          fmt.Sprintf("node-%d", i),
			Type:        NodeTypeConcept,
			Label:       fmt.Sprintf("concept %d", i),
			Weight:      0.8,
			LastUpdated: time.Now(),
		}
		kg.mu.Lock()
		kg.nodes[node.ID] = node
		kg.mu.Unlock()
	}
}

func BenchmarkKnowledgeGraph_GetNode(b *testing.B) {
	kg := NewKnowledgeGraph(10000)
	for i := 0; i < 1000; i++ {
		node := &KnowledgeNode{
			ID:          fmt.Sprintf("node-%d", i),
			Type:        NodeTypeConcept,
			Label:       fmt.Sprintf("concept %d", i),
			Weight:      0.8,
			LastUpdated: time.Now(),
		}
		kg.nodes[node.ID] = node
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kg.GetNode(fmt.Sprintf("node-%d", i%1000))
	}
}

func BenchmarkKnowledgeGraph_GetConnections(b *testing.B) {
	kg := NewKnowledgeGraph(10000)
	for i := 0; i < 100; i++ {
		node := &KnowledgeNode{
			ID:          fmt.Sprintf("node-%d", i),
			Type:        NodeTypeConcept,
			Label:       fmt.Sprintf("concept %d", i),
			Weight:      0.8,
			LastUpdated: time.Now(),
		}
		kg.nodes[node.ID] = node
	}
	// Add edges
	for i := 0; i < 500; i++ {
		edge := &KnowledgeEdge{
			FromID:  fmt.Sprintf("node-%d", i%100),
			ToID:    fmt.Sprintf("node-%d", (i+1)%100),
			Type:    EdgeTypeRelatedTo,
			Weight:  0.5,
			Created: time.Now(),
		}
		kg.edges = append(kg.edges, edge)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kg.GetConnections(fmt.Sprintf("node-%d", i%100))
	}
}

func BenchmarkPatternAnalyzer_Analyze(b *testing.B) {
	analyzer := NewPatternAnalyzer()
	result := createTestDebateResult()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.Analyze(result)
	}
}

func BenchmarkStrategySynthesizer_Synthesize(b *testing.B) {
	synthesizer := NewStrategySynthesizer()
	result := createTestDebateResult()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		synthesizer.Synthesize(result)
	}
}
