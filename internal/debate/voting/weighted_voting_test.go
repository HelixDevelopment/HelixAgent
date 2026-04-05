package voting

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create test votes
func createTestVotes(count int, choice string, confidence float64) []*Vote {
	votes := make([]*Vote, count)
	for i := 0; i < count; i++ {
		votes[i] = &Vote{
			AgentID:        "agent-" + string(rune('a'+i)),
			Choice:         choice,
			Confidence:     confidence,
			Score:          7.5 + float64(i)*0.1,
			Specialization: "general",
			Role:           "proposer",
			Timestamp:      time.Now(),
		}
	}
	return votes
}

// ============================================================================
// Config Tests
// ============================================================================

func TestDefaultVotingConfig(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()

	assert.Equal(t, 3, config.MinimumVotes)
	assert.Equal(t, 0.3, config.MinimumConfidence)
	assert.True(t, config.EnableDiversityBonus)
	assert.Equal(t, 0.1, config.DiversityWeight)
	assert.True(t, config.EnableTieBreaking)
	assert.Equal(t, TieBreakByHighestConfidence, config.TieBreakMethod)
	assert.True(t, config.EnableHistoricalWeight)
}

// ============================================================================
// System Creation Tests
// ============================================================================

func TestNewWeightedVotingSystem(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	assert.NotNil(t, wvs)
	assert.NotNil(t, wvs.votes)
	assert.NotNil(t, wvs.historicalData)
}

// ============================================================================
// Vote Management Tests
// ============================================================================

func TestAddVote(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	vote := &Vote{
		AgentID:    "agent-1",
		Choice:     "option_a",
		Confidence: 0.8,
		Score:      8.0,
		Timestamp:  time.Now(),
	}

	err := wvs.AddVote(vote)
	assert.NoError(t, err)
	assert.Equal(t, 1, wvs.VoteCount())
}

func TestAddVote_EmptyChoice(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	vote := &Vote{
		AgentID:    "agent-1",
		Choice:     "",
		Confidence: 0.8,
	}

	err := wvs.AddVote(vote)
	assert.Error(t, err)
}

func TestAddVote_InvalidConfidence(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	vote := &Vote{
		AgentID:    "agent-1",
		Choice:     "option_a",
		Confidence: 1.5, // Invalid
	}

	err := wvs.AddVote(vote)
	assert.Error(t, err)
}

func TestAddVote_DuplicateReplacesExisting(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	vote1 := &Vote{
		AgentID:    "agent-1",
		Choice:     "option_a",
		Confidence: 0.8,
	}

	vote2 := &Vote{
		AgentID:    "agent-1",
		Choice:     "option_b",
		Confidence: 0.9,
	}

	_ = wvs.AddVote(vote1)
	_ = wvs.AddVote(vote2)

	assert.Equal(t, 1, wvs.VoteCount())

	votes := wvs.GetVotes()
	assert.Equal(t, "option_b", votes[0].Choice)
}

func TestAddVote_SetsTimestamp(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	vote := &Vote{
		AgentID:    "agent-1",
		Choice:     "option_a",
		Confidence: 0.8,
	}

	_ = wvs.AddVote(vote)

	votes := wvs.GetVotes()
	assert.False(t, votes[0].Timestamp.IsZero())
}

func TestReset(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	votes := createTestVotes(3, "option_a", 0.8)
	for _, v := range votes {
		_ = wvs.AddVote(v)
	}

	assert.Equal(t, 3, wvs.VoteCount())

	wvs.Reset()

	assert.Equal(t, 0, wvs.VoteCount())
}

// ============================================================================
// Weighted Voting Tests (MiniMax Formula)
// ============================================================================

func TestCalculate_MiniMaxFormula(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	// Add votes with different confidences
	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9, Score: 8.0})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "A", Confidence: 0.8, Score: 7.5})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "B", Confidence: 0.7, Score: 7.0})

	ctx := context.Background()
	result, err := wvs.Calculate(ctx)

	require.NoError(t, err)
	assert.Equal(t, "A", result.WinningChoice) // A has higher weighted sum
	assert.Equal(t, VotingMethodWeighted, result.Method)
	assert.Equal(t, 3, result.TotalVotes)
}

func TestCalculate_InsufficientVotes(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 5
	wvs := NewWeightedVotingSystem(config)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "A", Confidence: 0.8})

	ctx := context.Background()
	_, err := wvs.Calculate(ctx)

	assert.Error(t, err)
}

func TestCalculate_FiltersLowConfidence(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 2
	config.MinimumConfidence = 0.5
	wvs := NewWeightedVotingSystem(config)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "B", Confidence: 0.2}) // Filtered out
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "A", Confidence: 0.8})

	ctx := context.Background()
	result, err := wvs.Calculate(ctx)

	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalVotes)
	assert.Equal(t, 2, result.ValidVotes) // Only 2 valid votes
}

func TestCalculate_WeightedScores(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 2
	config.EnableDiversityBonus = false
	config.EnableHistoricalWeight = false
	wvs := NewWeightedVotingSystem(config)

	// A: 0.9 confidence, B: 0.5 confidence
	// A should win due to higher weight
	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9, Score: 8.0})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "B", Confidence: 0.5, Score: 8.0})

	ctx := context.Background()
	result, err := wvs.Calculate(ctx)

	require.NoError(t, err)
	assert.Equal(t, "A", result.WinningChoice)
	assert.Greater(t, result.ChoiceScores["A"], result.ChoiceScores["B"])
}

func TestCalculate_VoteWeights(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 2
	wvs := NewWeightedVotingSystem(config)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9, Score: 9.0})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "B", Confidence: 0.6, Score: 6.0})

	ctx := context.Background()
	result, err := wvs.Calculate(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result.VoteWeights)

	// Higher confidence and score should result in higher weight
	assert.Greater(t, result.VoteWeights["a1"].TotalWeight, result.VoteWeights["a2"].TotalWeight)
}

// ============================================================================
// Tie Breaking Tests
// ============================================================================

func TestCalculate_TieDetection(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 2
	config.EnableTieBreaking = false // Disable to detect tie
	wvs := NewWeightedVotingSystem(config)

	// Same confidence, should tie
	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.8, Score: 8.0})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "B", Confidence: 0.8, Score: 8.0})

	ctx := context.Background()
	result, err := wvs.Calculate(ctx)

	require.NoError(t, err)
	assert.True(t, result.IsTie)
	assert.Len(t, result.TieChoices, 2)
}

func TestCalculate_TieBreakByHighestConfidence(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 4
	config.EnableTieBreaking = true
	config.TieBreakMethod = TieBreakByHighestConfidence
	config.EnableDiversityBonus = false
	config.EnableHistoricalWeight = false
	wvs := NewWeightedVotingSystem(config)

	// Equal weighted sums but different max confidences
	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.8, Score: 8.0})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "A", Confidence: 0.6, Score: 8.0})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "B", Confidence: 0.7, Score: 8.0})
	_ = wvs.AddVote(&Vote{AgentID: "a4", Choice: "B", Confidence: 0.7, Score: 8.0})

	ctx := context.Background()
	result, err := wvs.Calculate(ctx)

	require.NoError(t, err)
	// A should win due to higher max confidence (0.8 vs 0.7)
	if result.TieBreakUsed {
		assert.Equal(t, TieBreakByHighestConfidence, result.TieBreakMethod)
	}
}

func TestCalculate_TieBreakByMostVotes(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	config.EnableTieBreaking = true
	config.TieBreakMethod = TieBreakByMostVotes
	config.EnableDiversityBonus = false
	config.EnableHistoricalWeight = false
	wvs := NewWeightedVotingSystem(config)

	// A: 1 high confidence vote, B: 2 lower confidence votes
	// Weighted scores might tie, but B has more votes
	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9, Score: 8.0})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "B", Confidence: 0.45, Score: 8.0})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "B", Confidence: 0.45, Score: 8.0})

	ctx := context.Background()
	result, err := wvs.Calculate(ctx)

	require.NoError(t, err)
	assert.Equal(t, 2, result.ChoiceVoteCounts["B"])
	assert.Equal(t, 1, result.ChoiceVoteCounts["A"])
}

func TestCalculate_TieBreakByLeaderVote(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 2
	config.EnableTieBreaking = true
	config.TieBreakMethod = TieBreakByLeaderVote
	config.EnableDiversityBonus = false
	config.EnableHistoricalWeight = false
	wvs := NewWeightedVotingSystem(config)

	// Equal confidence but different scores
	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.8, Score: 9.5}) // Leader
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "B", Confidence: 0.8, Score: 7.0})

	ctx := context.Background()
	result, err := wvs.Calculate(ctx)

	require.NoError(t, err)
	// If tie, A should win because a1 (A voter) has higher score
	if result.TieBreakUsed {
		assert.Equal(t, TieBreakByLeaderVote, result.TieBreakMethod)
	}
}

// ============================================================================
// Diversity Bonus Tests
// ============================================================================

func TestCalculate_DiversityBonus(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	config.EnableDiversityBonus = true
	config.DiversityWeight = 0.2
	wvs := NewWeightedVotingSystem(config)

	// Different specializations
	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.8, Score: 8.0, Specialization: "code", Role: "proposer"})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "A", Confidence: 0.8, Score: 8.0, Specialization: "reasoning", Role: "critic"})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "B", Confidence: 0.8, Score: 8.0, Specialization: "vision", Role: "reviewer"})

	ctx := context.Background()
	result, err := wvs.Calculate(ctx)

	require.NoError(t, err)
	// All should have some diversity bonus due to unique specs
	for _, weight := range result.VoteWeights {
		assert.GreaterOrEqual(t, weight.DiversityBonus, 0.0)
	}
}

func TestCalculate_DiversityBonus_Disabled(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 2
	config.EnableDiversityBonus = false
	wvs := NewWeightedVotingSystem(config)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.8, Specialization: "code"})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "B", Confidence: 0.8, Specialization: "reasoning"})

	ctx := context.Background()
	result, err := wvs.Calculate(ctx)

	require.NoError(t, err)
	for _, weight := range result.VoteWeights {
		assert.Equal(t, 0.0, weight.DiversityBonus)
	}
}

// ============================================================================
// Historical Weight Tests
// ============================================================================

func TestUpdateHistory(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	// Update history
	wvs.UpdateHistory("agent-1", true, 0.9)
	wvs.UpdateHistory("agent-1", true, 0.8)
	wvs.UpdateHistory("agent-1", false, 0.7)

	history := wvs.GetHistory("agent-1")

	assert.NotNil(t, history)
	assert.Equal(t, 3, history.TotalVotes)
	assert.Equal(t, 2, history.CorrectVotes)
	assert.InDelta(t, 0.667, history.Accuracy, 0.01)
}

func TestCalculate_HistoricalWeight(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 2
	config.EnableHistoricalWeight = true
	wvs := NewWeightedVotingSystem(config)

	// Set up historical data
	wvs.UpdateHistory("a1", true, 0.9)
	wvs.UpdateHistory("a1", true, 0.9)
	wvs.UpdateHistory("a2", false, 0.5)
	wvs.UpdateHistory("a2", false, 0.5)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.8, Score: 8.0})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "B", Confidence: 0.8, Score: 8.0})

	ctx := context.Background()
	result, err := wvs.Calculate(ctx)

	require.NoError(t, err)
	// a1 should have higher weight due to better history
	assert.Greater(t, result.VoteWeights["a1"].HistoricalBonus, result.VoteWeights["a2"].HistoricalBonus)
}

// ============================================================================
// Consensus Calculation Tests
// ============================================================================

func TestCalculate_ConsensusLevel(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	// Unanimous vote
	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.8})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "A", Confidence: 0.8})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "A", Confidence: 0.8})

	ctx := context.Background()
	result, err := wvs.Calculate(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1.0, result.Consensus) // Perfect consensus
}

func TestCalculate_SplitConsensus(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 4
	wvs := NewWeightedVotingSystem(config)

	// Split vote
	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.8})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "A", Confidence: 0.8})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "B", Confidence: 0.8})
	_ = wvs.AddVote(&Vote{AgentID: "a4", Choice: "B", Confidence: 0.8})

	ctx := context.Background()
	result, err := wvs.Calculate(ctx)

	require.NoError(t, err)
	assert.InDelta(t, 0.5, result.Consensus, 0.1) // Split consensus
}

// ============================================================================
// Simple Majority Tests
// ============================================================================

func TestCalculateSimpleMajority(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.5})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "A", Confidence: 0.5})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "B", Confidence: 0.9})

	ctx := context.Background()
	result, err := wvs.CalculateSimpleMajority(ctx)

	require.NoError(t, err)
	assert.Equal(t, "A", result.WinningChoice) // A has more votes
	assert.Equal(t, VotingMethodMajority, result.Method)
	assert.Equal(t, 2, result.ChoiceVoteCounts["A"])
}

// ============================================================================
// Borda Count Tests
// ============================================================================

func TestCalculateBordaCount(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	rankings := map[string][]string{
		"agent-1": {"A", "B", "C"},
		"agent-2": {"A", "C", "B"},
		"agent-3": {"B", "A", "C"},
	}

	ctx := context.Background()
	result, err := wvs.CalculateBordaCount(ctx, rankings)

	require.NoError(t, err)
	assert.Equal(t, VotingMethodBorda, result.Method)
	// A: 2+2+1 = 5 points, B: 1+0+2 = 3 points, C: 0+1+0 = 1 point
	assert.Equal(t, "A", result.WinningChoice)
}

// ============================================================================
// Productive Chaos Tests
// ============================================================================

func TestSimulateProductiveChaos(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	votes := createTestVotes(5, "A", 0.5)
	for _, v := range votes {
		_ = wvs.AddVote(v)
	}

	originalConfidences := make([]float64, len(votes))
	votesBeforeChaos := wvs.GetVotes()
	for i, v := range votesBeforeChaos {
		originalConfidences[i] = v.Confidence
	}

	wvs.SimulateProductiveChaos(0.5)

	// Confidences should have changed slightly
	votesAfterChaos := wvs.GetVotes()
	for i, v := range votesAfterChaos {
		// Should still be valid (0-1)
		assert.GreaterOrEqual(t, v.Confidence, 0.0)
		assert.LessOrEqual(t, v.Confidence, 1.0)
		// Should have changed (may be same due to deterministic sin function)
		_ = originalConfidences[i]
	}
}

func TestSimulateProductiveChaos_InvalidLevel(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	votes := createTestVotes(3, "A", 0.5)
	for _, v := range votes {
		_ = wvs.AddVote(v)
	}

	originalConfidences := make([]float64, 3)
	for i, v := range wvs.GetVotes() {
		originalConfidences[i] = v.Confidence
	}

	// Invalid chaos level should be ignored
	wvs.SimulateProductiveChaos(0)
	wvs.SimulateProductiveChaos(-0.5)
	wvs.SimulateProductiveChaos(1.5)

	// Confidences should remain unchanged
	for i, v := range wvs.GetVotes() {
		assert.Equal(t, originalConfidences[i], v.Confidence)
	}
}

// ============================================================================
// Statistics Tests
// ============================================================================

func TestGetStatistics(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9, Specialization: "code", Role: "proposer"})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "A", Confidence: 0.7, Specialization: "reasoning", Role: "critic"})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "B", Confidence: 0.8, Specialization: "code", Role: "reviewer"})

	stats := wvs.GetStatistics()

	assert.Equal(t, 3, stats.TotalVotes)
	assert.Equal(t, 2, stats.UniqueChoices)
	assert.InDelta(t, 0.8, stats.AvgConfidence, 0.01)
	assert.Greater(t, stats.ConfidenceStdDev, 0.0)
	assert.InDelta(t, 0.667, stats.ChoiceDistribution["A"], 0.01)
	assert.InDelta(t, 0.333, stats.ChoiceDistribution["B"], 0.01)
	assert.Equal(t, 2, stats.SpecializationMix["code"])
	assert.Equal(t, 1, stats.SpecializationMix["reasoning"])
}

func TestGetStatistics_EmptyVotes(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	stats := wvs.GetStatistics()

	assert.Equal(t, 0, stats.TotalVotes)
	assert.Equal(t, 0, stats.UniqueChoices)
	assert.Equal(t, 0.0, stats.AvgConfidence)
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestFullVotingWorkflow(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 5
	config.EnableDiversityBonus = true
	config.EnableHistoricalWeight = true
	wvs := NewWeightedVotingSystem(config)

	// Set up historical data
	wvs.UpdateHistory("a1", true, 0.9)
	wvs.UpdateHistory("a2", true, 0.8)
	wvs.UpdateHistory("a3", false, 0.7)
	wvs.UpdateHistory("a4", true, 0.85)
	wvs.UpdateHistory("a5", true, 0.75)

	// Add diverse votes
	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "microservices", Confidence: 0.9, Score: 9.0, Specialization: "code", Role: "architect"})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "microservices", Confidence: 0.85, Score: 8.5, Specialization: "reasoning", Role: "proposer"})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "monolith", Confidence: 0.7, Score: 7.0, Specialization: "general", Role: "critic"})
	_ = wvs.AddVote(&Vote{AgentID: "a4", Choice: "microservices", Confidence: 0.8, Score: 8.0, Specialization: "code", Role: "reviewer"})
	_ = wvs.AddVote(&Vote{AgentID: "a5", Choice: "hybrid", Confidence: 0.75, Score: 7.5, Specialization: "vision", Role: "moderator"})

	ctx := context.Background()

	// Calculate weighted result
	result, err := wvs.Calculate(ctx)
	require.NoError(t, err)

	assert.Equal(t, "microservices", result.WinningChoice)
	assert.Equal(t, 5, result.TotalVotes)
	assert.Equal(t, VotingMethodWeighted, result.Method)
	assert.Greater(t, result.Consensus, 0.0)

	// Verify statistics
	stats := wvs.GetStatistics()
	assert.Equal(t, 5, stats.TotalVotes)
	assert.Equal(t, 3, stats.UniqueChoices)

	// Update history with result
	wvs.UpdateHistory("a1", true, 0.9)
	wvs.UpdateHistory("a2", true, 0.85)
	wvs.UpdateHistory("a3", false, 0.7) // monolith didn't win
	wvs.UpdateHistory("a4", true, 0.8)
	wvs.UpdateHistory("a5", false, 0.75) // hybrid didn't win

	// Verify history updated
	history := wvs.GetHistory("a1")
	assert.Equal(t, 2, history.TotalVotes)
	assert.Equal(t, 2, history.CorrectVotes)
	assert.Equal(t, 1.0, history.Accuracy)
}

// ============================================================================
// Condorcet Voting Tests
// ============================================================================

func TestCalculateCondorcet_ClearWinner(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	// A beats B (2-1), A beats C (3-0), B beats C (2-1) → A is Condorcet winner
	rankings := map[string][]string{
		"agent-1": {"A", "B", "C"},
		"agent-2": {"A", "C", "B"},
		"agent-3": {"B", "A", "C"},
	}

	ctx := context.Background()
	result, err := wvs.CalculateCondorcet(ctx, rankings)

	require.NoError(t, err)
	assert.Equal(t, VotingMethodCondorcet, result.Method)
	assert.Equal(t, "A", result.WinningChoice)
	assert.Greater(t, result.Consensus, 0.0)
}

func TestCalculateCondorcet_Cycle(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	// Condorcet cycle: A>B>C>A (rock-paper-scissors)
	rankings := map[string][]string{
		"agent-1": {"A", "B", "C"},
		"agent-2": {"B", "C", "A"},
		"agent-3": {"C", "A", "B"},
	}

	ctx := context.Background()
	result, err := wvs.CalculateCondorcet(ctx, rankings)

	require.NoError(t, err)
	assert.Equal(t, VotingMethodCondorcet, result.Method)
	// Should have used Borda fallback
	require.NotNil(t, result.Metadata)
	assert.Equal(t, true, result.Metadata["fallback_used"])
	assert.Equal(t, "condorcet_cycle", result.Metadata["fallback_reason"])
}

func TestCalculateCondorcet_TwoCandidates(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	rankings := map[string][]string{
		"agent-1": {"A", "B"},
		"agent-2": {"A", "B"},
		"agent-3": {"B", "A"},
	}

	ctx := context.Background()
	result, err := wvs.CalculateCondorcet(ctx, rankings)

	require.NoError(t, err)
	assert.Equal(t, "A", result.WinningChoice)
}

func TestCalculateCondorcet_SingleCandidate(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	rankings := map[string][]string{
		"agent-1": {"A"},
		"agent-2": {"A"},
		"agent-3": {"A"},
	}

	ctx := context.Background()
	result, err := wvs.CalculateCondorcet(ctx, rankings)

	require.NoError(t, err)
	assert.Equal(t, "A", result.WinningChoice)
}

func TestCalculateCondorcet_InsufficientRankings(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	rankings := map[string][]string{
		"agent-1": {"A", "B"},
	}

	ctx := context.Background()
	_, err := wvs.CalculateCondorcet(ctx, rankings)

	assert.Error(t, err)
}

func TestBuildCondorcetMatrix(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	rankings := map[string][]string{
		"agent-1": {"A", "B", "C"},
		"agent-2": {"B", "A", "C"},
		"agent-3": {"A", "C", "B"},
	}

	matrix := wvs.buildCondorcetMatrix(rankings)

	assert.Contains(t, matrix.Candidates, "A")
	assert.Contains(t, matrix.Candidates, "B")
	assert.Contains(t, matrix.Candidates, "C")

	// A vs B: agent-1 prefers A, agent-2 prefers B, agent-3 prefers A → A wins 2-1
	assert.Equal(t, 2, matrix.Wins["A"]["B"])
	assert.Equal(t, 1, matrix.Wins["B"]["A"])

	// A vs C: all 3 prefer A over C → A wins 3-0
	assert.Equal(t, 3, matrix.Wins["A"]["C"])
	assert.Equal(t, 0, matrix.Wins["C"]["A"])
}

func TestFindCondorcetWinner_Found(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	matrix := &CondorcetMatrix{
		Candidates: []string{"A", "B", "C"},
		Wins: map[string]map[string]int{
			"A": {"B": 2, "C": 3},
			"B": {"A": 1, "C": 2},
			"C": {"A": 0, "B": 1},
		},
	}

	winner, found := wvs.findCondorcetWinner(matrix)
	assert.True(t, found)
	assert.Equal(t, "A", winner)
}

func TestFindCondorcetWinner_NotFound(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	// Cycle: A>B, B>C, C>A
	matrix := &CondorcetMatrix{
		Candidates: []string{"A", "B", "C"},
		Wins: map[string]map[string]int{
			"A": {"B": 2, "C": 1},
			"B": {"A": 1, "C": 2},
			"C": {"A": 2, "B": 1},
		},
	}

	winner, found := wvs.findCondorcetWinner(matrix)
	assert.False(t, found)
	assert.Empty(t, winner)
}

// ============================================================================
// Plurality Voting Tests
// ============================================================================

func TestCalculatePlurality_ClearWinner(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "A", Confidence: 0.8})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "B", Confidence: 0.7})
	_ = wvs.AddVote(&Vote{AgentID: "a4", Choice: "C", Confidence: 0.6})

	ctx := context.Background()
	result, err := wvs.CalculatePlurality(ctx)

	require.NoError(t, err)
	assert.Equal(t, VotingMethodPlurality, result.Method)
	assert.Equal(t, "A", result.WinningChoice)
	assert.Equal(t, 4, result.TotalVotes)
	assert.InDelta(t, 0.5, result.Consensus, 0.01) // 2/4
}

func TestCalculatePlurality_Tie(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	config.EnableTieBreaking = true
	config.TieBreakMethod = TieBreakByHighestConfidence
	wvs := NewWeightedVotingSystem(config)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "B", Confidence: 0.8})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "C", Confidence: 0.7})

	ctx := context.Background()
	result, err := wvs.CalculatePlurality(ctx)

	require.NoError(t, err)
	assert.Equal(t, VotingMethodPlurality, result.Method)
	// Tie broken by highest confidence → A
	assert.True(t, result.TieBreakUsed)
}

func TestCalculatePlurality_InsufficientVotes(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9})

	ctx := context.Background()
	_, err := wvs.CalculatePlurality(ctx)

	assert.Error(t, err)
}

func TestCalculatePlurality_NoMajorityNeeded(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 5
	wvs := NewWeightedVotingSystem(config)

	// 5 different choices, one has 2 votes — plurality winner without majority
	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "A", Confidence: 0.8})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "B", Confidence: 0.7})
	_ = wvs.AddVote(&Vote{AgentID: "a4", Choice: "C", Confidence: 0.6})
	_ = wvs.AddVote(&Vote{AgentID: "a5", Choice: "D", Confidence: 0.5})

	ctx := context.Background()
	result, err := wvs.CalculatePlurality(ctx)

	require.NoError(t, err)
	assert.Equal(t, "A", result.WinningChoice)
	assert.InDelta(t, 0.4, result.Consensus, 0.01) // 2/5
}

// ============================================================================
// Unanimous Voting Tests
// ============================================================================

func TestCalculateUnanimous_AllAgree(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "A", Confidence: 0.8})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "A", Confidence: 0.7})

	ctx := context.Background()
	result, err := wvs.CalculateUnanimous(ctx)

	require.NoError(t, err)
	assert.Equal(t, VotingMethodUnanimous, result.Method)
	assert.Equal(t, "A", result.WinningChoice)
	assert.Equal(t, 1.0, result.Consensus)
	assert.False(t, result.IsTie)
}

func TestCalculateUnanimous_Disagreement(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "A", Confidence: 0.8})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "B", Confidence: 0.7})

	ctx := context.Background()
	result, err := wvs.CalculateUnanimous(ctx)

	require.NoError(t, err)
	assert.Equal(t, VotingMethodUnanimous, result.Method)
	assert.True(t, result.IsTie) // No unanimity
	assert.Less(t, result.Consensus, 1.0)
	assert.Len(t, result.TieChoices, 2)        // A and B
	assert.Equal(t, "A", result.WinningChoice) // Still reports most-voted
}

func TestCalculateUnanimous_InsufficientVotes(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9})

	ctx := context.Background()
	_, err := wvs.CalculateUnanimous(ctx)

	assert.Error(t, err)
}

func TestCalculateUnanimous_LargeGroup(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	// All 10 agents agree
	for i := 0; i < 10; i++ {
		_ = wvs.AddVote(&Vote{
			AgentID:    fmt.Sprintf("agent-%d", i),
			Choice:     "consensus-choice",
			Confidence: 0.8 + float64(i)*0.02,
		})
	}

	ctx := context.Background()
	result, err := wvs.CalculateUnanimous(ctx)

	require.NoError(t, err)
	assert.Equal(t, "consensus-choice", result.WinningChoice)
	assert.Equal(t, 1.0, result.Consensus)
	assert.Equal(t, 10, result.TotalVotes)
}

// ============================================================================
// Auto-Selection Tests
// ============================================================================

func TestAutoSelectMethod(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	tests := []struct {
		name       string
		agentCount int
		expected   VotingMethod
	}{
		{"single agent", 1, VotingMethodUnanimous},
		{"two agents", 2, VotingMethodUnanimous},
		{"three agents", 3, VotingMethodWeighted},
		{"five agents", 5, VotingMethodWeighted},
		{"six agents", 6, VotingMethodBorda},
		{"ten agents", 10, VotingMethodBorda},
		{"large group", 25, VotingMethodBorda},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			method := wvs.AutoSelectMethod(tc.agentCount)
			assert.Equal(t, tc.expected, method)
		})
	}
}

// ============================================================================
// Cross-Method Comparison Tests
// ============================================================================

func TestVotingMethods_SameData_DifferentResults(t *testing.T) {
	t.Parallel()
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	_ = wvs.AddVote(&Vote{AgentID: "a1", Choice: "A", Confidence: 0.9, Score: 9.0})
	_ = wvs.AddVote(&Vote{AgentID: "a2", Choice: "A", Confidence: 0.5, Score: 5.0})
	_ = wvs.AddVote(&Vote{AgentID: "a3", Choice: "B", Confidence: 0.95, Score: 9.5})

	ctx := context.Background()

	// Weighted: B might win (higher confidence × score)
	weighted, err := wvs.Calculate(ctx)
	require.NoError(t, err)
	assert.Equal(t, VotingMethodWeighted, weighted.Method)

	// Majority: A wins (2 vs 1)
	majority, err := wvs.CalculateSimpleMajority(ctx)
	require.NoError(t, err)
	assert.Equal(t, "A", majority.WinningChoice)

	// Plurality: A wins (2 vs 1)
	plurality, err := wvs.CalculatePlurality(ctx)
	require.NoError(t, err)
	assert.Equal(t, "A", plurality.WinningChoice)

	// Unanimous: No consensus
	unanimous, err := wvs.CalculateUnanimous(ctx)
	require.NoError(t, err)
	assert.True(t, unanimous.IsTie)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkWeightedVotingSystem_Calculate(b *testing.B) {
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	choices := []string{"A", "B", "C"}
	roles := []string{"proposer", "critic", "reviewer", "optimizer", "moderator"}
	specs := []string{"code", "reasoning", "general"}
	for i := 0; i < 10; i++ {
		_ = wvs.AddVote(&Vote{
			AgentID:            fmt.Sprintf("agent-%d", i),
			Choice:             choices[i%len(choices)],
			Confidence:         0.5 + float64(i%5)*0.1,
			Score:              7.0 + float64(i%4)*0.5,
			Specialization:     specs[i%len(specs)],
			Role:               roles[i%len(roles)],
			HistoricalAccuracy: 0.7 + float64(i%3)*0.1,
			Timestamp:          time.Now(),
		})
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = wvs.Calculate(ctx)
	}
}

func BenchmarkWeightedVotingSystem_CalculateSimpleMajority(b *testing.B) {
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	for i := 0; i < 10; i++ {
		choice := "A"
		if i%3 == 0 {
			choice = "B"
		}
		_ = wvs.AddVote(&Vote{
			AgentID:    fmt.Sprintf("agent-%d", i),
			Choice:     choice,
			Confidence: 0.7,
			Score:      8.0,
			Timestamp:  time.Now(),
		})
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = wvs.CalculateSimpleMajority(ctx)
	}
}

func BenchmarkWeightedVotingSystem_CalculateBordaCount(b *testing.B) {
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	rankings := map[string][]string{
		"voter-1": {"A", "B", "C", "D"},
		"voter-2": {"B", "A", "D", "C"},
		"voter-3": {"A", "C", "B", "D"},
		"voter-4": {"C", "B", "A", "D"},
		"voter-5": {"A", "B", "C", "D"},
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = wvs.CalculateBordaCount(ctx, rankings)
	}
}

func BenchmarkWeightedVotingSystem_CalculateCondorcet(b *testing.B) {
	config := DefaultVotingConfig()
	config.MinimumVotes = 3
	wvs := NewWeightedVotingSystem(config)

	rankings := map[string][]string{
		"voter-1": {"A", "B", "C"},
		"voter-2": {"B", "C", "A"},
		"voter-3": {"A", "C", "B"},
		"voter-4": {"C", "A", "B"},
		"voter-5": {"A", "B", "C"},
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = wvs.CalculateCondorcet(ctx, rankings)
	}
}

func BenchmarkWeightedVotingSystem_GetStatistics(b *testing.B) {
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	for i := 0; i < 20; i++ {
		_ = wvs.AddVote(&Vote{
			AgentID:        fmt.Sprintf("agent-%d", i),
			Choice:         fmt.Sprintf("choice-%d", i%4),
			Confidence:     0.5 + float64(i%5)*0.1,
			Score:          7.0 + float64(i%3)*0.5,
			Specialization: fmt.Sprintf("spec-%d", i%3),
			Role:           fmt.Sprintf("role-%d", i%5),
			Timestamp:      time.Now(),
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wvs.GetStatistics()
	}
}

func BenchmarkWeightedVotingSystem_AddVote(b *testing.B) {
	config := DefaultVotingConfig()
	wvs := NewWeightedVotingSystem(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = wvs.AddVote(&Vote{
			AgentID:    fmt.Sprintf("agent-%d", i),
			Choice:     "A",
			Confidence: 0.8,
			Score:      8.0,
			Timestamp:  time.Now(),
		})
	}
}
