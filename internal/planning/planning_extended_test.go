package planning

import (
	"context"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// MCTS extended tests
// =============================================================================

func TestMCTS_UCTValue_NilNode(t *testing.T) {
	config := DefaultMCTSConfig()
	mcts := NewMCTS(config, nil, nil, nil, nil)

	value := mcts.UCTValue(nil, 10)
	assert.Equal(t, 0.0, value)
}

func TestMCTS_UCTValue_Unvisited(t *testing.T) {
	config := DefaultMCTSConfig()
	mcts := NewMCTS(config, nil, nil, nil, nil)

	node := &MCTSNode{}
	value := mcts.UCTValue(node, 10)
	assert.True(t, value > 1e10, "unvisited node should get infinity value")
}

func TestMCTS_UCTValue_Normal(t *testing.T) {
	config := DefaultMCTSConfig()
	config.UseUCTDP = false
	mcts := NewMCTS(config, nil, nil, nil, nil)

	node := &MCTSNode{}
	node.SetVisits(10)
	node.SetTotalReward(7.0)
	value := mcts.UCTValue(node, 100)

	// exploitation = 7.0/10 = 0.7
	// exploration > 0
	assert.Greater(t, value, 0.7)
}

func TestMCTS_UCTValue_WithDepthPreference(t *testing.T) {
	config := DefaultMCTSConfig()
	config.UseUCTDP = true
	config.DepthPreferenceAlpha = 0.5
	config.MaxDepth = 10
	mcts := NewMCTS(config, nil, nil, nil, nil)

	shallow := &MCTSNode{Depth: 1}
	shallow.SetVisits(10)
	shallow.SetTotalReward(5.0)
	deep := &MCTSNode{Depth: 8}
	deep.SetVisits(10)
	deep.SetTotalReward(5.0)

	shallowValue := mcts.UCTValue(shallow, 100)
	deepValue := mcts.UCTValue(deep, 100)

	// Deep node should have higher value due to depth bonus
	assert.Greater(t, deepValue, shallowValue)
}

func TestMCTS_UCTValue_ZeroParentVisits(t *testing.T) {
	config := DefaultMCTSConfig()
	config.UseUCTDP = false
	mcts := NewMCTS(config, nil, nil, nil, nil)

	node := &MCTSNode{}
	node.SetVisits(5)
	node.SetTotalReward(3.0)
	value := mcts.UCTValue(node, 0)

	// Should use parentVisits = 1 as minimum
	assert.Greater(t, value, 0.0)
}

func TestMCTS_CountNodes(t *testing.T) {
	config := DefaultMCTSConfig()
	mcts := NewMCTS(config, nil, nil, nil, nil)

	c11 := &MCTSNode{ID: "c1-1"}
	c1 := &MCTSNode{
		ID:       "c1",
		Children: safe.NewSlice[*MCTSNode](c11),
	}
	c2 := &MCTSNode{ID: "c2"}
	root := &MCTSNode{
		ID:       "root",
		Children: safe.NewSlice[*MCTSNode](c1, c2),
	}

	count := mcts.countNodes(root)
	assert.Equal(t, 4, count)

	// Nil node
	assert.Equal(t, 0, mcts.countNodes(nil))
}

func TestMCTS_FindParent(t *testing.T) {
	config := DefaultMCTSConfig()
	mcts := NewMCTS(config, nil, nil, nil, nil)

	child := &MCTSNode{ID: "child"}
	root := &MCTSNode{
		ID:       "root",
		Children: safe.NewSlice[*MCTSNode](child),
	}

	found := mcts.findParent(root, "root")
	assert.Equal(t, root, found)

	found = mcts.findParent(root, "child")
	assert.Equal(t, child, found)

	found = mcts.findParent(root, "missing")
	assert.Nil(t, found)
}

func TestMCTS_GetBestPath_EmptyTree(t *testing.T) {
	config := DefaultMCTSConfig()
	config.MaxIterations = 1
	config.Timeout = 1 * time.Second

	actionGen := &MockActionGenerator{}
	rewardFunc := &MockRewardFunction{}
	mcts := NewMCTS(config, actionGen, rewardFunc, nil, logrus.New())

	result, err := mcts.Search(context.Background(), "init")
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// =============================================================================
// MCTSNode tests
// =============================================================================

func TestMCTSNode_AddReward_Concurrent(t *testing.T) {
	node := &MCTSNode{}

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			node.AddReward(0.5)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 10, node.Visits())
	assert.InDelta(t, 5.0, node.TotalReward(), 0.001)
}

// =============================================================================
// HiPlan extended tests
// =============================================================================

func TestHiPlan_SortMilestonesByDependencies(t *testing.T) {
	config := DefaultHiPlanConfig()
	hiplan := NewHiPlan(config, nil, nil, nil)

	milestones := []*Milestone{
		{ID: "c", Name: "C", Dependencies: []string{"b"}},
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B", Dependencies: []string{"a"}},
	}

	sorted := hiplan.sortMilestonesByDependencies(milestones)
	require.Len(t, sorted, 3)

	// a should come before b, b before c
	aIdx, bIdx, cIdx := -1, -1, -1
	for i, m := range sorted {
		switch m.ID {
		case "a":
			aIdx = i
		case "b":
			bIdx = i
		case "c":
			cIdx = i
		}
	}

	assert.Less(t, aIdx, bIdx)
	assert.Less(t, bIdx, cIdx)
}

func TestHiPlan_SortMilestonesByDependencies_CyclicDeps(t *testing.T) {
	config := DefaultHiPlanConfig()
	hiplan := NewHiPlan(config, nil, nil, nil)

	milestones := []*Milestone{
		{ID: "a", Name: "A", Dependencies: []string{"c"}},
		{ID: "b", Name: "B", Dependencies: []string{"a"}},
		{ID: "c", Name: "C", Dependencies: []string{"b"}},
	}

	// Should not panic on cyclic deps
	sorted := hiplan.sortMilestonesByDependencies(milestones)
	assert.Len(t, sorted, 3)
}

func TestHiPlan_GetCurrentPlan_Empty(t *testing.T) {
	config := DefaultHiPlanConfig()
	hiplan := NewHiPlan(config, nil, nil, nil)

	assert.Nil(t, hiplan.GetCurrentPlan())
}

func TestHiPlan_ExecuteStep(t *testing.T) {
	config := DefaultHiPlanConfig()
	config.RetryFailedSteps = false
	executor := NewMockStepExecutor()

	hiplan := NewHiPlan(config, &MockMilestoneGenerator{}, executor, logrus.New())

	step := &PlanStep{ID: "test-step", Action: "do something"}
	result, err := hiplan.ExecuteStep(context.Background(), step, []string{"hint1"})

	assert.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestHiPlan_ExecuteStep_Failure(t *testing.T) {
	config := DefaultHiPlanConfig()
	config.RetryFailedSteps = false
	executor := NewMockStepExecutor()
	executor.failSteps["fail-step"] = true

	hiplan := NewHiPlan(config, &MockMilestoneGenerator{}, executor, logrus.New())

	step := &PlanStep{ID: "fail-step", Action: "fail"}
	result, _ := hiplan.ExecuteStep(context.Background(), step, nil)

	require.NotNil(t, result)
	assert.False(t, result.Success)
}

func TestHiPlan_ExecuteStep_WithRetries(t *testing.T) {
	config := DefaultHiPlanConfig()
	config.RetryFailedSteps = true
	config.MaxRetries = 2
	config.StepTimeout = 5 * time.Second

	callCount := 0
	executor := &dynamicMockStepExecutor{
		executeFn: func(ctx context.Context, step *PlanStep, hints []string) (*StepResult, error) {
			callCount++
			if callCount < 3 {
				return &StepResult{Success: false, Error: "temporary failure"}, nil
			}
			return &StepResult{Success: true}, nil
		},
	}

	hiplan := NewHiPlan(config, &MockMilestoneGenerator{}, executor, logrus.New())

	step := &PlanStep{ID: "retry-step", Action: "retry"}
	result, _ := hiplan.ExecuteStep(context.Background(), step, nil)

	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 3, callCount)
}

type dynamicMockStepExecutor struct {
	executeFn  func(ctx context.Context, step *PlanStep, hints []string) (*StepResult, error)
	validateFn func(ctx context.Context, step *PlanStep, result *StepResult) error
}

func (m *dynamicMockStepExecutor) Execute(ctx context.Context, step *PlanStep, hints []string) (*StepResult, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, step, hints)
	}
	return &StepResult{Success: true}, nil
}

func (m *dynamicMockStepExecutor) Validate(ctx context.Context, step *PlanStep, result *StepResult) error {
	if m.validateFn != nil {
		return m.validateFn(ctx, step, result)
	}
	return nil
}

// =============================================================================
// LLMMilestoneGenerator tests
// =============================================================================

func TestLLMMilestoneGenerator_GenerateMilestones(t *testing.T) {
	mockFunc := func(ctx context.Context, prompt string) (string, error) {
		return "1. Setup environment\n2. Implement feature\n3. Write tests", nil
	}

	gen := NewLLMMilestoneGenerator(mockFunc, logrus.New())
	milestones, err := gen.GenerateMilestones(context.Background(), "Build a service")

	assert.NoError(t, err)
	assert.Len(t, milestones, 3)
	assert.Equal(t, MilestoneStatePending, milestones[0].State)
}

func TestLLMMilestoneGenerator_GenerateSteps(t *testing.T) {
	mockFunc := func(ctx context.Context, prompt string) (string, error) {
		return "1. Create file\n2. Add logic\n3. Test it", nil
	}

	gen := NewLLMMilestoneGenerator(mockFunc, logrus.New())
	milestone := &Milestone{ID: "m1", Name: "Test", Description: "Testing"}

	steps, err := gen.GenerateSteps(context.Background(), milestone)
	assert.NoError(t, err)
	assert.Len(t, steps, 3)
	assert.Equal(t, PlanStepStatePending, steps[0].State)
}

func TestLLMMilestoneGenerator_GenerateHints(t *testing.T) {
	mockFunc := func(ctx context.Context, prompt string) (string, error) {
		return "- Check edge cases\n- Use proper error handling", nil
	}

	gen := NewLLMMilestoneGenerator(mockFunc, logrus.New())
	step := &PlanStep{ID: "s1", Action: "Implement"}

	hints, err := gen.GenerateHints(context.Background(), step, "some context")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(hints), 1)
}

// =============================================================================
// CodeActionGenerator tests
// =============================================================================

func TestCodeActionGenerator_GetActions(t *testing.T) {
	mockFunc := func(ctx context.Context, prompt string) (string, error) {
		return "1. Refactor code\n2. Add tests\n3. Fix bugs", nil
	}

	gen := NewCodeActionGenerator(mockFunc, logrus.New())
	actions, err := gen.GetActions(context.Background(), "initial state")

	assert.NoError(t, err)
	assert.Len(t, actions, 3)
}

func TestCodeActionGenerator_ApplyAction(t *testing.T) {
	mockFunc := func(ctx context.Context, prompt string) (string, error) {
		return "updated state", nil
	}

	gen := NewCodeActionGenerator(mockFunc, logrus.New())
	result, err := gen.ApplyAction(context.Background(), "initial", "refactor")

	assert.NoError(t, err)
	assert.Equal(t, "updated state", result)
}

// =============================================================================
// CodeRewardFunction tests
// =============================================================================

func TestCodeRewardFunction_Evaluate_NilFunc(t *testing.T) {
	rf := NewCodeRewardFunction(nil, nil, logrus.New())
	score, err := rf.Evaluate(context.Background(), "code")

	assert.NoError(t, err)
	assert.Equal(t, 0.5, score)
}

func TestCodeRewardFunction_IsTerminal_NilFunc(t *testing.T) {
	rf := NewCodeRewardFunction(nil, nil, logrus.New())
	terminal, err := rf.IsTerminal(context.Background(), "code")

	assert.NoError(t, err)
	assert.False(t, terminal)
}

func TestCodeRewardFunction_Evaluate_WithFunc(t *testing.T) {
	evalFn := func(ctx context.Context, code string) (float64, error) {
		return 0.85, nil
	}

	rf := NewCodeRewardFunction(evalFn, nil, logrus.New())
	score, err := rf.Evaluate(context.Background(), "good code")

	assert.NoError(t, err)
	assert.Equal(t, 0.85, score)
}

// =============================================================================
// Tree of Thoughts extended tests
// =============================================================================

func TestTreeOfThoughts_NilLogger(t *testing.T) {
	config := DefaultTreeOfThoughtsConfig()
	tot := NewTreeOfThoughts(config, NewMockThoughtGenerator(), NewMockThoughtEvaluator(), nil)
	assert.NotNil(t, tot)
}

func TestTreeOfThoughts_DefaultStrategy(t *testing.T) {
	config := DefaultTreeOfThoughtsConfig()
	config.SearchStrategy = "unknown"
	config.MaxIterations = 10
	config.Timeout = 5 * time.Second

	tot := NewTreeOfThoughts(config, NewMockThoughtGenerator(), NewMockThoughtEvaluator(), logrus.New())

	result, err := tot.Solve(context.Background(), "test problem")
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestLLMThoughtEvaluator_EvaluatePath_Empty(t *testing.T) {
	evaluator := NewLLMThoughtEvaluator(nil, logrus.New())
	score, err := evaluator.EvaluatePath(context.Background(), []*Thought{})

	assert.NoError(t, err)
	assert.Equal(t, 0.0, score)
}

func TestLLMThoughtEvaluator_EvaluatePath_NonEmpty(t *testing.T) {
	evaluator := NewLLMThoughtEvaluator(nil, logrus.New())
	path := []*Thought{
		{Score: 0.5},
		{Score: 0.8},
		{Score: 0.9},
	}

	score, err := evaluator.EvaluatePath(context.Background(), path)
	assert.NoError(t, err)
	assert.Greater(t, score, 0.0)
}

func TestLLMThoughtGenerator_GenerateInitialThoughts(t *testing.T) {
	mockFunc := func(ctx context.Context, prompt string) (string, error) {
		return "1. Approach A\n2. Approach B", nil
	}

	gen := NewLLMThoughtGenerator(mockFunc, 0.7, logrus.New())
	thoughts, err := gen.GenerateInitialThoughts(context.Background(), "problem", 2)

	assert.NoError(t, err)
	assert.Len(t, thoughts, 2)
	assert.Equal(t, 1, thoughts[0].Depth)
}

// =============================================================================
// Helper function edge case tests
// =============================================================================

func TestSplitLines_Empty(t *testing.T) {
	lines := splitLines("")
	assert.Len(t, lines, 0)
}

func TestSplitLines_SingleLine(t *testing.T) {
	lines := splitLines("hello world")
	assert.Len(t, lines, 1)
	assert.Equal(t, "hello world", lines[0])
}

func TestContainsIgnoreCase_EmptySubstring(t *testing.T) {
	assert.True(t, containsIgnoreCase("Hello", ""))
}

func TestContains_LongerSubstring(t *testing.T) {
	assert.False(t, contains("abc", "abcdef"))
}

func TestDefaultTreeOfThoughtsConfig_Values(t *testing.T) {
	config := DefaultTreeOfThoughtsConfig()
	assert.Equal(t, 10, config.MaxDepth)
	assert.Equal(t, 5, config.MaxBranches)
	assert.Equal(t, 0.3, config.MinScore)
	assert.Equal(t, "beam", config.SearchStrategy)
	assert.Equal(t, 3, config.BeamWidth)
	assert.True(t, config.EnableBacktracking)
}

func TestDefaultMCTSConfig_Values(t *testing.T) {
	config := DefaultMCTSConfig()
	assert.InDelta(t, 1.414, config.ExplorationConstant, 0.001)
	assert.Equal(t, 50, config.MaxDepth)
	assert.Equal(t, 1000, config.MaxIterations)
	assert.True(t, config.EnableParallel)
	assert.True(t, config.UseUCTDP)
}

func TestDefaultHiPlanConfig_Values(t *testing.T) {
	config := DefaultHiPlanConfig()
	assert.Equal(t, 20, config.MaxMilestones)
	assert.Equal(t, 50, config.MaxStepsPerMilestone)
	assert.True(t, config.EnableParallelMilestones)
	assert.True(t, config.EnableAdaptivePlanning)
	assert.True(t, config.RetryFailedSteps)
	assert.Equal(t, 3, config.MaxRetries)
}
