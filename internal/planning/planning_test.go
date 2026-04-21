package planning

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockThoughtGenerator implements ThoughtGenerator for testing
type MockThoughtGenerator struct {
	thoughts map[string][]*Thought
}

func NewMockThoughtGenerator() *MockThoughtGenerator {
	return &MockThoughtGenerator{
		thoughts: make(map[string][]*Thought),
	}
}

func (m *MockThoughtGenerator) GenerateThoughts(ctx context.Context, parent *Thought, count int) ([]*Thought, error) {
	thoughts := make([]*Thought, count)
	for i := 0; i < count; i++ {
		thoughts[i] = &Thought{
			ID:        fmt.Sprintf("%s-child-%d", parent.ID, i),
			Content:   fmt.Sprintf("Thought from %s: approach %d", parent.ID, i+1),
			State:     ThoughtStatePending,
			CreatedAt: time.Now(),
		}
	}
	return thoughts, nil
}

func (m *MockThoughtGenerator) GenerateInitialThoughts(ctx context.Context, problem string, count int) ([]*Thought, error) {
	thoughts := make([]*Thought, count)
	for i := 0; i < count; i++ {
		thoughts[i] = &Thought{
			ID:        fmt.Sprintf("init-%d", i),
			Content:   fmt.Sprintf("Initial approach %d for: %s", i+1, problem),
			State:     ThoughtStatePending,
			CreatedAt: time.Now(),
		}
	}
	return thoughts, nil
}

// MockThoughtEvaluator implements ThoughtEvaluator for testing
type MockThoughtEvaluator struct {
	scores map[string]float64
}

func NewMockThoughtEvaluator() *MockThoughtEvaluator {
	return &MockThoughtEvaluator{
		scores: make(map[string]float64),
	}
}

func (m *MockThoughtEvaluator) EvaluateThought(ctx context.Context, thought *Thought) (float64, error) {
	if score, exists := m.scores[thought.ID]; exists {
		return score, nil
	}
	// Generate a pseudo-random score based on ID
	score := 0.5 + float64(len(thought.ID)%5)*0.1
	return score, nil
}

func (m *MockThoughtEvaluator) EvaluatePath(ctx context.Context, path []*Thought) (float64, error) {
	if len(path) == 0 {
		return 0, nil
	}
	total := 0.0
	for _, t := range path {
		total += t.Score
	}
	return total / float64(len(path)), nil
}

func (m *MockThoughtEvaluator) IsTerminal(ctx context.Context, thought *Thought) (bool, error) {
	return thought.Depth >= 3, nil
}

// Tree of Thoughts Tests

func TestNewTreeOfThoughts(t *testing.T) {
	config := DefaultTreeOfThoughtsConfig()
	generator := NewMockThoughtGenerator()
	evaluator := NewMockThoughtEvaluator()

	tot := NewTreeOfThoughts(config, generator, evaluator, logrus.New())

	assert.NotNil(t, tot)
	assert.Equal(t, config.MaxDepth, tot.config.MaxDepth)
}

func TestTreeOfThoughts_Solve_BFS(t *testing.T) {
	config := DefaultTreeOfThoughtsConfig()
	config.SearchStrategy = "bfs"
	config.MaxIterations = 20
	config.Timeout = 10 * time.Second

	generator := NewMockThoughtGenerator()
	evaluator := NewMockThoughtEvaluator()

	tot := NewTreeOfThoughts(config, generator, evaluator, logrus.New())

	result, err := tot.Solve(context.Background(), "Test problem")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Test problem", result.Problem)
	assert.Equal(t, "bfs", result.Strategy)
	assert.Greater(t, result.NodesExplored, 0)
}

func TestTreeOfThoughts_Solve_DFS(t *testing.T) {
	config := DefaultTreeOfThoughtsConfig()
	config.SearchStrategy = "dfs"
	config.MaxIterations = 20
	config.Timeout = 10 * time.Second

	generator := NewMockThoughtGenerator()
	evaluator := NewMockThoughtEvaluator()

	tot := NewTreeOfThoughts(config, generator, evaluator, logrus.New())

	result, err := tot.Solve(context.Background(), "Test problem")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "dfs", result.Strategy)
}

func TestTreeOfThoughts_Solve_Beam(t *testing.T) {
	config := DefaultTreeOfThoughtsConfig()
	config.SearchStrategy = "beam"
	config.BeamWidth = 2
	config.MaxIterations = 20
	config.Timeout = 10 * time.Second

	generator := NewMockThoughtGenerator()
	evaluator := NewMockThoughtEvaluator()

	tot := NewTreeOfThoughts(config, generator, evaluator, logrus.New())

	result, err := tot.Solve(context.Background(), "Test problem")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "beam", result.Strategy)
}

func TestTreeOfThoughts_Timeout(t *testing.T) {
	config := DefaultTreeOfThoughtsConfig()
	config.Timeout = 100 * time.Millisecond
	config.MaxIterations = 10000

	generator := NewMockThoughtGenerator()
	evaluator := NewMockThoughtEvaluator()

	tot := NewTreeOfThoughts(config, generator, evaluator, logrus.New())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := tot.Solve(ctx, "Test problem")

	// Either times out with error, or completes successfully before timeout
	// Both outcomes are valid - the test validates timeout handling doesn't panic
	assert.NotNil(t, result)
	if err != nil {
		assert.Contains(t, err.Error(), "context")
	}
}

func TestToTResult_GetSolutionContent(t *testing.T) {
	result := &ToTResult{
		Solution: []*Thought{
			{Content: "Step 1"},
			{Content: "Step 2"},
			{Content: "Step 3"},
		},
	}

	contents := result.GetSolutionContent()
	assert.Equal(t, 3, len(contents))
	assert.Equal(t, "Step 1", contents[0])
}

func TestToTResult_MarshalJSON(t *testing.T) {
	result := &ToTResult{
		Problem:   "Test",
		Duration:  5 * time.Second,
		BestScore: 0.9,
	}

	data, err := result.MarshalJSON()
	assert.NoError(t, err)
	assert.Contains(t, string(data), "duration_ms")
	assert.Contains(t, string(data), "5000")
}

// MCTS Tests

type MockActionGenerator struct{}

func (m *MockActionGenerator) GetActions(ctx context.Context, state interface{}) ([]string, error) {
	return []string{"action1", "action2", "action3"}, nil
}

func (m *MockActionGenerator) ApplyAction(ctx context.Context, state interface{}, action string) (interface{}, error) {
	stateStr, _ := state.(string)
	return fmt.Sprintf("%s -> %s", stateStr, action), nil
}

type MockRewardFunction struct {
	terminalDepth int
}

func (m *MockRewardFunction) Evaluate(ctx context.Context, state interface{}) (float64, error) {
	stateStr, _ := state.(string)
	return float64(len(stateStr)%10) / 10.0, nil
}

func (m *MockRewardFunction) IsTerminal(ctx context.Context, state interface{}) (bool, error) {
	stateStr, _ := state.(string)
	return len(stateStr) > 50, nil
}

func TestNewMCTS(t *testing.T) {
	config := DefaultMCTSConfig()
	actionGen := &MockActionGenerator{}
	rewardFunc := &MockRewardFunction{}

	mcts := NewMCTS(config, actionGen, rewardFunc, nil, logrus.New())

	assert.NotNil(t, mcts)
	assert.Equal(t, config.ExplorationConstant, mcts.config.ExplorationConstant)
}

func TestMCTS_Search(t *testing.T) {
	config := DefaultMCTSConfig()
	config.MaxIterations = 50
	config.MaxDepth = 5
	config.Timeout = 10 * time.Second

	actionGen := &MockActionGenerator{}
	rewardFunc := &MockRewardFunction{}

	mcts := NewMCTS(config, actionGen, rewardFunc, nil, logrus.New())

	result, err := mcts.Search(context.Background(), "initial")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Greater(t, result.TotalIterations, 0)
	assert.Greater(t, result.TreeSize, 0)
}

func TestMCTS_Search_WithRollout(t *testing.T) {
	config := DefaultMCTSConfig()
	config.MaxIterations = 30
	config.RolloutDepth = 3
	config.Timeout = 10 * time.Second

	actionGen := &MockActionGenerator{}
	rewardFunc := &MockRewardFunction{}
	rolloutPolicy := NewDefaultRolloutPolicy(actionGen, rewardFunc)

	mcts := NewMCTS(config, actionGen, rewardFunc, rolloutPolicy, logrus.New())

	result, err := mcts.Search(context.Background(), "initial")

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMCTSNode_AverageReward(t *testing.T) {
	node := &MCTSNode{}
	node.SetVisits(10)
	node.SetTotalReward(7.5)

	assert.Equal(t, 0.75, node.AverageReward())
}

func TestMCTSNode_AverageReward_ZeroVisits(t *testing.T) {
	node := &MCTSNode{}

	assert.Equal(t, 0.0, node.AverageReward())
}

func TestMCTSNode_AddReward(t *testing.T) {
	node := &MCTSNode{}

	node.AddReward(0.5)
	node.AddReward(0.7)

	assert.Equal(t, 2, node.Visits())
	assert.InDelta(t, 1.2, node.TotalReward(), 0.0001)
}

func TestMCTSResult_MarshalJSON(t *testing.T) {
	result := &MCTSResult{
		Duration:        3 * time.Second,
		TotalIterations: 100,
	}

	data, err := result.MarshalJSON()
	assert.NoError(t, err)
	assert.Contains(t, string(data), "duration_ms")
	assert.Contains(t, string(data), "3000")
}

// TestMCTSNode_JSONRoundTrip guarantees the public /v1/planning/mcts wire
// format is preserved after the CONST-029 migration to atomic-backed
// visits/totalReward and safe.Slice children / safe.Store metadata.
func TestMCTSNode_JSONRoundTrip(t *testing.T) {
	original := &MCTSNode{
		ID:        "root",
		ParentID:  "",
		State:     "initial",
		Action:    "start",
		NodeState: MCTSNodeStateExpanded,
		Depth:     0,
		CreatedAt: time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC),
	}
	original.SetVisits(5)
	original.SetTotalReward(2.25)

	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Wire-format contract: "visits" and "total_reward" are top-level keys.
	assert.Contains(t, string(data), `"visits":5`)
	assert.Contains(t, string(data), `"total_reward":2.25`)

	var restored MCTSNode
	require.NoError(t, json.Unmarshal(data, &restored))

	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.NodeState, restored.NodeState)
	assert.Equal(t, original.Depth, restored.Depth)
	assert.Equal(t, 5, restored.Visits())
	assert.InDelta(t, 2.25, restored.TotalReward(), 0.0001)
}

// HiPlan Tests

type MockMilestoneGenerator struct{}

func (m *MockMilestoneGenerator) GenerateMilestones(ctx context.Context, goal string) ([]*Milestone, error) {
	return []*Milestone{
		{ID: "m1", Name: "Milestone 1", State: MilestoneStatePending},
		{ID: "m2", Name: "Milestone 2", State: MilestoneStatePending, Dependencies: []string{"m1"}},
		{ID: "m3", Name: "Milestone 3", State: MilestoneStatePending, Dependencies: []string{"m2"}},
	}, nil
}

func (m *MockMilestoneGenerator) GenerateSteps(ctx context.Context, milestone *Milestone) ([]*PlanStep, error) {
	return []*PlanStep{
		{ID: milestone.ID + "-s1", Action: "Step 1", State: PlanStepStatePending},
		{ID: milestone.ID + "-s2", Action: "Step 2", State: PlanStepStatePending},
	}, nil
}

func (m *MockMilestoneGenerator) GenerateHints(ctx context.Context, step *PlanStep, context string) ([]string, error) {
	return []string{"Hint 1", "Hint 2"}, nil
}

type MockStepExecutor struct {
	failSteps map[string]bool
}

func NewMockStepExecutor() *MockStepExecutor {
	return &MockStepExecutor{
		failSteps: make(map[string]bool),
	}
}

func (m *MockStepExecutor) Execute(ctx context.Context, step *PlanStep, hints []string) (*StepResult, error) {
	if m.failSteps[step.ID] {
		return &StepResult{Success: false, Error: "mock failure"}, nil
	}
	return &StepResult{
		Success:  true,
		Duration: 100 * time.Millisecond,
		Outputs:  map[string]interface{}{"result": "done"},
	}, nil
}

func (m *MockStepExecutor) Validate(ctx context.Context, step *PlanStep, result *StepResult) error {
	return nil
}

func TestNewHiPlan(t *testing.T) {
	config := DefaultHiPlanConfig()
	generator := &MockMilestoneGenerator{}
	executor := NewMockStepExecutor()

	hiplan := NewHiPlan(config, generator, executor, logrus.New())

	assert.NotNil(t, hiplan)
}

func TestHiPlan_CreatePlan(t *testing.T) {
	config := DefaultHiPlanConfig()
	generator := &MockMilestoneGenerator{}
	executor := NewMockStepExecutor()

	hiplan := NewHiPlan(config, generator, executor, logrus.New())

	plan, err := hiplan.CreatePlan(context.Background(), "Test goal")

	assert.NoError(t, err)
	assert.NotNil(t, plan)
	assert.Equal(t, "Test goal", plan.Goal)
	assert.Equal(t, 3, len(plan.Milestones))
}

func TestHiPlan_ExecutePlan(t *testing.T) {
	config := DefaultHiPlanConfig()
	config.EnableParallelMilestones = false
	generator := &MockMilestoneGenerator{}
	executor := NewMockStepExecutor()

	hiplan := NewHiPlan(config, generator, executor, logrus.New())

	plan, _ := hiplan.CreatePlan(context.Background(), "Test goal")
	result, err := hiplan.ExecutePlan(context.Background(), plan)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 3, result.CompletedMilestones)
	assert.Equal(t, 0, result.FailedMilestones)
}

func TestHiPlan_ExecutePlan_WithFailure(t *testing.T) {
	config := DefaultHiPlanConfig()
	config.EnableParallelMilestones = false
	config.EnableAdaptivePlanning = false
	config.RetryFailedSteps = false
	generator := &MockMilestoneGenerator{}
	executor := NewMockStepExecutor()
	executor.failSteps["m1-s1"] = true

	hiplan := NewHiPlan(config, generator, executor, logrus.New())

	plan, _ := hiplan.CreatePlan(context.Background(), "Test goal")
	result, err := hiplan.ExecutePlan(context.Background(), plan)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
}

func TestHiPlan_ExecutePlan_Parallel(t *testing.T) {
	config := DefaultHiPlanConfig()
	config.EnableParallelMilestones = true
	config.MaxParallelMilestones = 2
	generator := &MockMilestoneGenerator{}
	executor := NewMockStepExecutor()

	hiplan := NewHiPlan(config, generator, executor, logrus.New())

	plan, _ := hiplan.CreatePlan(context.Background(), "Test goal")
	result, err := hiplan.ExecutePlan(context.Background(), plan)

	assert.NoError(t, err)
	assert.True(t, result.Success)
}

func TestHiPlan_Library(t *testing.T) {
	config := DefaultHiPlanConfig()
	hiplan := NewHiPlan(config, nil, nil, logrus.New())

	milestone := &Milestone{ID: "test-m", Name: "Test Milestone"}
	hiplan.AddToLibrary(milestone)

	retrieved, exists := hiplan.GetFromLibrary("test-m")
	assert.True(t, exists)
	assert.Equal(t, "Test Milestone", retrieved.Name)

	_, exists = hiplan.GetFromLibrary("nonexistent")
	assert.False(t, exists)
}

func TestPlanResult_MarshalJSON(t *testing.T) {
	result := &PlanResult{
		PlanID:   "test-plan",
		Duration: 10 * time.Second,
		Success:  true,
	}

	data, err := result.MarshalJSON()
	assert.NoError(t, err)
	assert.Contains(t, string(data), "duration_ms")
	assert.Contains(t, string(data), "10000")
}

// Helper function tests

func TestSplitLines(t *testing.T) {
	input := "1. First line\n2. Second line\n3. Third line"
	lines := splitLines(input)

	assert.Equal(t, 3, len(lines))
}

func TestContainsIgnoreCase(t *testing.T) {
	assert.True(t, containsIgnoreCase("Hello World", "hello"))
	assert.True(t, containsIgnoreCase("Hello World", "WORLD"))
	assert.False(t, containsIgnoreCase("Hello World", "foo"))
}

// LLM Generator/Evaluator Tests (with mock function)

func TestLLMThoughtGenerator_GenerateThoughts(t *testing.T) {
	mockFunc := func(ctx context.Context, prompt string) (string, error) {
		return "1. First approach\n2. Second approach\n3. Third approach", nil
	}

	generator := NewLLMThoughtGenerator(mockFunc, 0.7, logrus.New())
	parent := &Thought{ID: "parent", Content: "Test"}

	thoughts, err := generator.GenerateThoughts(context.Background(), parent, 3)

	assert.NoError(t, err)
	assert.Equal(t, 3, len(thoughts))
}

func TestLLMThoughtEvaluator_EvaluateThought(t *testing.T) {
	mockFunc := func(ctx context.Context, prompt string) (string, error) {
		return "0.85", nil
	}

	evaluator := NewLLMThoughtEvaluator(mockFunc, logrus.New())
	thought := &Thought{Content: "Test thought"}

	score, err := evaluator.EvaluateThought(context.Background(), thought)

	assert.NoError(t, err)
	assert.Equal(t, 0.85, score)
}

func TestLLMThoughtEvaluator_IsTerminal(t *testing.T) {
	evaluator := NewLLMThoughtEvaluator(nil, logrus.New())

	// Test with terminal keyword
	thought := &Thought{Content: "This is the final solution"}
	isTerminal, _ := evaluator.IsTerminal(context.Background(), thought)
	assert.True(t, isTerminal)

	// Test with high score
	thought2 := &Thought{Content: "Some thought", Score: 0.95}
	isTerminal2, _ := evaluator.IsTerminal(context.Background(), thought2)
	assert.True(t, isTerminal2)

	// Test non-terminal
	thought3 := &Thought{Content: "Some thought", Score: 0.5}
	isTerminal3, _ := evaluator.IsTerminal(context.Background(), thought3)
	assert.False(t, isTerminal3)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkMCTSNode_AverageReward(b *testing.B) {
	node := &MCTSNode{}
	node.SetVisits(1000)
	node.SetTotalReward(750.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = node.AverageReward()
	}
}

func BenchmarkMCTSNode_AddReward(b *testing.B) {
	node := &MCTSNode{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		node.AddReward(0.75)
	}
}

func BenchmarkSplitLines(b *testing.B) {
	input := "1. First line\n2. Second line\n3. Third line\n4. Fourth line\n5. Fifth line"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = splitLines(input)
	}
}

func BenchmarkContainsIgnoreCase(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = containsIgnoreCase("Hello World this is a test string", "test")
	}
}

func BenchmarkDefaultMCTSConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DefaultMCTSConfig()
	}
}

func BenchmarkDefaultHiPlanConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DefaultHiPlanConfig()
	}
}
