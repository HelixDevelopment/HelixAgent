package cognitive

import (
	"context"
	"testing"
	"time"

	"dev.helix.agent/internal/debate/topology"
	"github.com/stretchr/testify/assert"
)

// Helper to create test agents
func createTestAgents(count int) []*topology.Agent {
	agents := make([]*topology.Agent, count)
	for i := 0; i < count; i++ {
		agents[i] = topology.CreateAgentFromSpec(
			"agent-"+string(rune('a'+i)),
			topology.RoleProposer,
			"test_provider",
			"test_model",
			7.5+float64(i)*0.1,
			"general",
		)
		agents[i].Confidence = 0.7
	}
	return agents
}

// ============================================================================
// Config Tests
// ============================================================================

func TestDefaultPlanningConfig(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()

	assert.True(t, config.EnableLearning)
	assert.Equal(t, 0.05, config.ExpectationThreshold)
	assert.Equal(t, 0.3, config.AdaptationRate)
	assert.Equal(t, 100, config.MaxHistorySize)
	assert.True(t, config.EnableMetaCognition)
}

// ============================================================================
// Planner Creation Tests
// ============================================================================

func TestNewCognitivePlanner(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	assert.NotNil(t, planner)
	assert.NotNil(t, planner.expectations)
	assert.NotNil(t, planner.comparisons)
	assert.NotNil(t, planner.refinements)
	assert.NotNil(t, planner.learningHistory)
	assert.NotNil(t, planner.phaseBaselines)
	assert.NotNil(t, planner.planningMetrics)
}

// ============================================================================
// Expectation Tests
// ============================================================================

func TestSetExpectation(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(3)

	expectation := planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)

	assert.NotNil(t, expectation)
	assert.Equal(t, topology.PhaseProposal, expectation.Phase)
	assert.Equal(t, 1, expectation.Round)
	assert.Greater(t, expectation.ExpectedConfidence, 0.0)
	assert.Greater(t, expectation.ExpectedConsensus, 0.0)
	assert.Greater(t, expectation.ExpectedInsights, 0)
	assert.NotEmpty(t, expectation.KeyGoals)
	assert.False(t, expectation.Timestamp.IsZero())
}

func TestSetExpectation_UpdatesMetrics(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(2)

	initialCount := planner.planningMetrics.TotalExpectations

	planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)
	planner.SetExpectation(ctx, topology.PhaseCritique, 1, agents)

	assert.Equal(t, initialCount+2, planner.planningMetrics.TotalExpectations)
}

func TestSetExpectation_DifferentPhases(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(3)

	phases := []topology.DebatePhase{
		topology.PhaseProposal,
		topology.PhaseCritique,
		topology.PhaseReview,
		topology.PhaseOptimization,
		topology.PhaseConvergence,
	}

	for _, phase := range phases {
		exp := planner.SetExpectation(ctx, phase, 1, agents)
		assert.Equal(t, phase, exp.Phase)
		assert.NotEmpty(t, exp.KeyGoals)
	}
}

func TestSetExpectation_HighScoringAgents(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()

	// Low scoring agents
	lowAgents := createTestAgents(2)
	lowAgents[0].Score = 5.0
	lowAgents[1].Score = 5.5

	lowExp := planner.SetExpectation(ctx, topology.PhaseProposal, 1, lowAgents)

	// Reset planner
	planner = NewCognitivePlanner(config)

	// High scoring agents
	highAgents := createTestAgents(2)
	highAgents[0].Score = 9.0
	highAgents[1].Score = 9.5

	highExp := planner.SetExpectation(ctx, topology.PhaseProposal, 1, highAgents)

	// Higher scoring agents should have higher expectations
	assert.Greater(t, highExp.ExpectedConfidence, lowExp.ExpectedConfidence)
}

// ============================================================================
// Comparison Tests
// ============================================================================

func TestCompare(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(3)

	// Set expectation first
	planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)

	// Compare with actual results
	comparison := planner.Compare(
		ctx,
		topology.PhaseProposal,
		1,
		0.8,            // actualConfidence
		0.7,            // actualConsensus
		5,              // actualInsights
		25*time.Second, // actualLatency
		[]string{"Generate creative solutions"},
		[]string{"Unexpected finding"},
	)

	assert.NotNil(t, comparison)
	assert.Equal(t, topology.PhaseProposal, comparison.Phase)
	assert.Equal(t, 1, comparison.Round)
	assert.NotZero(t, comparison.OverallScore)
	assert.False(t, comparison.Timestamp.IsZero())
}

func TestCompare_CalculatesDeltas(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(2)

	// Set expectation
	exp := planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)

	// Compare with results exceeding expectations
	comparison := planner.Compare(
		ctx,
		topology.PhaseProposal,
		1,
		exp.ExpectedConfidence+0.1,        // Exceed confidence
		exp.ExpectedConsensus+0.1,         // Exceed consensus
		exp.ExpectedInsights+2,            // More insights
		exp.ExpectedLatency-5*time.Second, // Faster
		nil, nil,
	)

	assert.Greater(t, comparison.ConfidenceDelta, 0.0)
	assert.Greater(t, comparison.ConsensusDelta, 0.0)
	assert.Greater(t, comparison.InsightsDelta, 0)
	assert.Less(t, comparison.LatencyDelta, time.Duration(0))
	assert.Greater(t, comparison.OverallScore, 0.5) // Should be above expectations
}

func TestCompare_UpdatesBaseline(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(2)

	planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)

	// Multiple comparisons should update baseline
	planner.Compare(ctx, topology.PhaseProposal, 1, 0.9, 0.8, 6, 20*time.Second, nil, nil)
	planner.Compare(ctx, topology.PhaseProposal, 2, 0.85, 0.75, 5, 25*time.Second, nil, nil)

	baseline := planner.phaseBaselines[topology.PhaseProposal]
	assert.NotNil(t, baseline)
	assert.Greater(t, baseline.SampleCount, 0)
}

func TestCompare_UpdatesMetrics(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(2)

	planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)

	initialCount := planner.planningMetrics.TotalComparisons

	planner.Compare(ctx, topology.PhaseProposal, 1, 0.8, 0.7, 5, 25*time.Second, nil, nil)

	assert.Equal(t, initialCount+1, planner.planningMetrics.TotalComparisons)
}

func TestCompare_WithoutExpectation(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()

	// Compare without setting expectation - should use defaults
	comparison := planner.Compare(
		ctx,
		topology.PhaseCritique,
		1,
		0.75, 0.65, 4, 30*time.Second, nil, nil,
	)

	assert.NotNil(t, comparison)
	assert.Equal(t, topology.PhaseCritique, comparison.Phase)
}

// ============================================================================
// Refinement Tests
// ============================================================================

func TestRefine(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(3)

	planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)
	comparison := planner.Compare(ctx, topology.PhaseProposal, 1, 0.8, 0.7, 5, 25*time.Second, nil, nil)

	refinement := planner.Refine(ctx, comparison, agents)

	assert.NotNil(t, refinement)
	assert.Equal(t, topology.PhaseProposal, refinement.Phase)
	assert.NotNil(t, refinement.AgentPriorities)
	assert.NotNil(t, refinement.RoleEmphasis)
	assert.False(t, refinement.AppliedAt.IsZero())
}

func TestRefine_AdjustsAgentPriorities(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(3)

	planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)
	comparison := planner.Compare(ctx, topology.PhaseProposal, 1, 0.9, 0.85, 7, 20*time.Second, nil, nil)

	refinement := planner.Refine(ctx, comparison, agents)

	// All agents should have priorities set
	for _, agent := range agents {
		priority, exists := refinement.AgentPriorities[agent.ID]
		assert.True(t, exists)
		assert.Greater(t, priority, 0.0)
	}
}

func TestRefine_GeneratesMitigationStrategies(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(2)

	planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)

	comparison := &Comparison{
		Phase:         topology.PhaseProposal,
		Round:         1,
		OverallScore:  0.4,
		GoalsMissed:   []string{"Goal A", "Goal B"},
		RisksRealized: []string{"Risk 1"},
	}

	refinement := planner.Refine(ctx, comparison, agents)

	assert.NotEmpty(t, refinement.NewGoals)
	assert.NotEmpty(t, refinement.MitigationStrategies)
}

func TestRefine_ExtractsPatterns(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(2)

	// High success comparison
	highComparison := &Comparison{
		Phase:           topology.PhaseProposal,
		Round:           1,
		OverallScore:    0.85,
		ConfidenceDelta: 0.15,
		ConsensusDelta:  0.12,
		InsightsDelta:   3,
	}

	refinement := planner.Refine(ctx, highComparison, agents)
	assert.NotEmpty(t, refinement.SuccessPatterns)

	// Low success comparison
	lowComparison := &Comparison{
		Phase:           topology.PhaseCritique,
		Round:           1,
		OverallScore:    0.2,
		ConfidenceDelta: -0.25,
		ConsensusDelta:  -0.25,
		InsightsDelta:   -4,
		RisksRealized:   []string{"Risk 1"},
	}

	refinement = planner.Refine(ctx, lowComparison, agents)
	assert.NotEmpty(t, refinement.FailurePatterns)
}

// ============================================================================
// Strategy Tests
// ============================================================================

func TestGetNextPhaseStrategy(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	strategy := planner.GetNextPhaseStrategy(topology.PhaseProposal)

	assert.NotNil(t, strategy)
	assert.Equal(t, topology.PhaseCritique, strategy.Phase)
	assert.NotEmpty(t, strategy.Goals)
	assert.Greater(t, strategy.ExpectedConfidence, 0.0)
	assert.Greater(t, strategy.ExpectedConsensus, 0.0)
}

func TestGetNextPhaseStrategy_WithRefinements(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(3)

	// Execute full cycle to populate refinements
	planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)
	comparison := planner.Compare(ctx, topology.PhaseProposal, 1, 0.85, 0.8, 6, 25*time.Second, nil, nil)
	planner.Refine(ctx, comparison, agents)

	strategy := planner.GetNextPhaseStrategy(topology.PhaseProposal)

	assert.NotNil(t, strategy)
	// Should have adjustments from refinement
	assert.NotNil(t, strategy.RoleEmphasis)
	assert.NotNil(t, strategy.AgentPriorities)
}

// ============================================================================
// Learning Tests
// ============================================================================

func TestLearning_ExtractsInsights(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	config.EnableLearning = true
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(2)

	// Multiple comparisons should generate learning insights
	planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)
	planner.Compare(ctx, topology.PhaseProposal, 1, 0.85, 0.8, 6, 25*time.Second, nil, nil)
	planner.Compare(ctx, topology.PhaseProposal, 2, 0.9, 0.85, 7, 20*time.Second, nil, nil)

	history := planner.GetLearningHistory()
	assert.NotEmpty(t, history)
}

func TestLearning_EnforcesMaxHistory(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	config.MaxHistorySize = 5
	config.EnableLearning = true
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(2)

	// Generate many comparisons to exceed max history
	for i := 0; i < 20; i++ {
		planner.SetExpectation(ctx, topology.PhaseProposal, i, agents)
		planner.Compare(ctx, topology.PhaseProposal, i, 0.7+float64(i)*0.01, 0.6, 5, 30*time.Second, nil, nil)
	}

	history := planner.GetLearningHistory()
	assert.LessOrEqual(t, len(history), config.MaxHistorySize)
}

func TestLearning_Disabled(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	config.EnableLearning = false
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(2)

	planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)
	planner.Compare(ctx, topology.PhaseProposal, 1, 0.85, 0.8, 6, 25*time.Second, nil, nil)

	history := planner.GetLearningHistory()
	assert.Empty(t, history)
}

// ============================================================================
// Meta-Cognition Tests
// ============================================================================

func TestReflectOnPerformance(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	config.EnableMetaCognition = true
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(3)

	// Execute several cycles
	for i := 1; i <= 3; i++ {
		planner.SetExpectation(ctx, topology.PhaseProposal, i, agents)
		comparison := planner.Compare(ctx, topology.PhaseProposal, i, 0.8, 0.75, 5, 25*time.Second, nil, nil)
		planner.Refine(ctx, comparison, agents)
	}

	report := planner.ReflectOnPerformance(ctx)

	assert.NotNil(t, report)
	assert.Equal(t, 3, report.TotalComparisons)
	assert.Equal(t, 3, report.TotalRefinements)
	assert.NotZero(t, report.ExpectationAccuracy)
	assert.False(t, report.GeneratedAt.IsZero())
}

func TestReflectOnPerformance_Disabled(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	config.EnableMetaCognition = false
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	report := planner.ReflectOnPerformance(ctx)

	assert.Nil(t, report)
}

func TestReflectOnPerformance_GeneratesRecommendations(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	config.EnableMetaCognition = true
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(2)

	// Create scenario with low accuracy
	for i := 1; i <= 5; i++ {
		planner.SetExpectation(ctx, topology.PhaseProposal, i, agents)
		// Results much lower than expectations
		comparison := planner.Compare(ctx, topology.PhaseProposal, i, 0.3, 0.2, 1, 60*time.Second, nil, nil)
		planner.Refine(ctx, comparison, agents)
	}

	report := planner.ReflectOnPerformance(ctx)

	assert.NotEmpty(t, report.Recommendations)
}

// ============================================================================
// Metrics Tests
// ============================================================================

func TestGetPlanningMetrics(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(2)

	// Execute some operations
	planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)
	comparison := planner.Compare(ctx, topology.PhaseProposal, 1, 0.8, 0.7, 5, 25*time.Second, nil, nil)
	planner.Refine(ctx, comparison, agents)

	metrics := planner.GetPlanningMetrics()

	assert.NotNil(t, metrics)
	assert.Equal(t, 1, metrics.TotalExpectations)
	assert.Equal(t, 1, metrics.TotalComparisons)
	assert.Equal(t, 1, metrics.TotalRefinements)
}

func TestMetrics_AccuracyRate(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(2)

	// High accuracy scenario
	planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)
	planner.Compare(ctx, topology.PhaseProposal, 1, 0.85, 0.8, 6, 25*time.Second, nil, nil)

	metrics := planner.GetPlanningMetrics()
	assert.Greater(t, metrics.AccuracyRate, 0.0)
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestFullCognitiveCycle(t *testing.T) {
	t.Parallel()
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)

	ctx := context.Background()
	agents := createTestAgents(5)

	phases := []topology.DebatePhase{
		topology.PhaseProposal,
		topology.PhaseCritique,
		topology.PhaseReview,
		topology.PhaseOptimization,
		topology.PhaseConvergence,
	}

	for round := 1; round <= 2; round++ {
		for _, phase := range phases {
			// 1. Set Expectation
			expectation := planner.SetExpectation(ctx, phase, round, agents)
			assert.NotNil(t, expectation)

			// 2. Compare with "actual" results
			comparison := planner.Compare(
				ctx,
				phase,
				round,
				expectation.ExpectedConfidence+0.05, // Slightly exceed
				expectation.ExpectedConsensus+0.02,
				expectation.ExpectedInsights+1,
				expectation.ExpectedLatency-2*time.Second,
				expectation.KeyGoals[:1], // Achieve one goal
				nil,
			)
			assert.NotNil(t, comparison)

			// 3. Refine strategy
			refinement := planner.Refine(ctx, comparison, agents)
			assert.NotNil(t, refinement)

			// 4. Get strategy for next phase
			strategy := planner.GetNextPhaseStrategy(phase)
			assert.NotNil(t, strategy)
		}
	}

	// Verify metrics
	metrics := planner.GetPlanningMetrics()
	assert.Equal(t, 10, metrics.TotalExpectations) // 5 phases × 2 rounds
	assert.Equal(t, 10, metrics.TotalComparisons)
	assert.Equal(t, 10, metrics.TotalRefinements)

	// Verify learning occurred
	history := planner.GetLearningHistory()
	assert.NotEmpty(t, history)

	// Verify meta-cognition
	report := planner.ReflectOnPerformance(ctx)
	assert.NotNil(t, report)
	assert.Greater(t, report.ExpectationAccuracy, 0.0)
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestGetNextPhase(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		current  topology.DebatePhase
		expected topology.DebatePhase
	}{
		{topology.PhaseProposal, topology.PhaseCritique},
		{topology.PhaseCritique, topology.PhaseReview},
		{topology.PhaseReview, topology.PhaseOptimization},
		{topology.PhaseOptimization, topology.PhaseConvergence},
		{topology.PhaseConvergence, topology.PhaseProposal},
	}

	for _, tc := range testCases {
		t.Run(string(tc.current), func(t *testing.T) {
				t.Parallel()
			assert.Equal(t, tc.expected, getNextPhase(tc.current))
		})
	}
}

func TestDefaultValues(t *testing.T) {
	t.Parallel()
	phases := []topology.DebatePhase{
		topology.PhaseProposal,
		topology.PhaseCritique,
		topology.PhaseReview,
		topology.PhaseOptimization,
		topology.PhaseConvergence,
	}

	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
				t.Parallel()
			confidence := getDefaultConfidence(phase)
			consensus := getDefaultConsensus(phase)
			insights := getDefaultInsights(phase)

			assert.Greater(t, confidence, 0.0)
			assert.LessOrEqual(t, confidence, 1.0)
			assert.Greater(t, consensus, 0.0)
			assert.LessOrEqual(t, consensus, 1.0)
			assert.Greater(t, insights, 0.0)
		})
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkCognitivePlanner_SetExpectation(b *testing.B) {
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)
	agents := createTestAgents(5)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		planner.SetExpectation(ctx, topology.PhaseProposal, i%10, agents)
	}
}

func BenchmarkCognitivePlanner_Compare(b *testing.B) {
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)
	agents := createTestAgents(5)
	ctx := context.Background()

	// Set an initial expectation
	planner.SetExpectation(ctx, topology.PhaseProposal, 1, agents)

	goalsAchieved := []string{"Generate creative solutions"}
	unexpectedOutcomes := []string{"Timeout on agent-c"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		planner.Compare(ctx, topology.PhaseProposal, i%10,
			0.75, 0.65, 4, 25*time.Second,
			goalsAchieved, unexpectedOutcomes)
	}
}

func BenchmarkCognitivePlanner_GetNextPhaseStrategy(b *testing.B) {
	config := DefaultPlanningConfig()
	planner := NewCognitivePlanner(config)
	agents := createTestAgents(5)
	ctx := context.Background()

	// Simulate a few rounds of planning to build up baselines
	phases := []topology.DebatePhase{
		topology.PhaseProposal,
		topology.PhaseCritique,
		topology.PhaseReview,
	}
	for _, phase := range phases {
		planner.SetExpectation(ctx, phase, 1, agents)
		comparison := planner.Compare(ctx, phase, 1,
			0.75, 0.65, 4, 20*time.Second,
			[]string{"Goal achieved"}, nil)
		planner.Refine(ctx, comparison, agents)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		planner.GetNextPhaseStrategy(phases[i%len(phases)])
	}
}

func BenchmarkGetDefaultConfidence(b *testing.B) {
	phases := []topology.DebatePhase{
		topology.PhaseProposal,
		topology.PhaseCritique,
		topology.PhaseReview,
		topology.PhaseOptimization,
		topology.PhaseConvergence,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getDefaultConfidence(phases[i%len(phases)])
	}
}
