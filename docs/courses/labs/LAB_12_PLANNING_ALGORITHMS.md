# Lab 12: Planning Algorithms — HiPlan, MCTS, and Tree of Thoughts

## Lab Overview

**Duration**: 30 minutes
**Difficulty**: Advanced
**Module**: S7.2.1 — Planning Module

## Objectives

By completing this lab, you will:
- Run HiPlan to decompose a multi-step software task into milestones and steps
- Configure MCTS to search for an optimal code solution
- Execute Tree of Thoughts for open-ended architectural reasoning
- Compare the three algorithms on the same problem
- Apply resource limits correctly (MaxDepth, MaxIterations, BranchingFactor)

## Prerequisites

- Module S7.1 (Agentic, LLMOps, SelfImprove Labs) completed
- `Planning/` module available in the project
- Understanding of Go interfaces

---

## Exercise 1: HiPlan — Hierarchical Task Decomposition (10 minutes)

### Task 1.1: Verify the Planning Module

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_agent/Planning
go build ./...
go test ./... -short -count=1
```

### Task 1.2: Implement HiPlan for a Software Feature

```go
package planning_lab_test

import (
    "context"
    "fmt"
    "testing"

    "digital.vasic.planning/planning"
)

// Mock MilestoneGenerator (in production: use LLMMilestoneGenerator)
type MockMilestoneGenerator struct{}

func (g *MockMilestoneGenerator) GenerateMilestones(
    ctx context.Context, goal string) ([]planning.Milestone, error) {

    return []planning.Milestone{
        {
            ID:    "m1",
            Title: "Design API contract",
            Steps: []planning.PlanStep{
                {ID: "s1", Description: "Define request/response schemas"},
                {ID: "s2", Description: "Write OpenAPI specification"},
            },
        },
        {
            ID:    "m2",
            Title: "Implement handlers",
            Steps: []planning.PlanStep{
                {ID: "s3", Description: "Implement POST /users handler"},
                {ID: "s4", Description: "Implement GET /users/{id} handler"},
                {ID: "s5", Description: "Add input validation"},
            },
        },
        {
            ID:    "m3",
            Title: "Test and document",
            Steps: []planning.PlanStep{
                {ID: "s6", Description: "Write unit tests"},
                {ID: "s7", Description: "Write integration tests"},
                {ID: "s8", Description: "Update API documentation"},
            },
        },
    }, nil
}

// Mock StepExecutor (in production: executes real tasks)
type MockStepExecutor struct {
    executedSteps []string
}

func (e *MockStepExecutor) ExecuteStep(ctx context.Context,
    step planning.PlanStep, state interface{}) (interface{}, error) {

    e.executedSteps = append(e.executedSteps, step.ID)
    return fmt.Sprintf("Completed: %s", step.Description), nil
}

func TestHiPlan_SoftwareFeature(t *testing.T) {
    ctx := context.Background()

    executor := &MockStepExecutor{}
    cfg := planning.DefaultHiPlanConfig()
    cfg.MaxMilestones = 5
    cfg.MaxStepsPerMile = 10

    planner := planning.NewHiPlan(cfg,
        &MockMilestoneGenerator{},
        executor,
    )

    result, err := planner.Execute(ctx, "Build user management REST API")
    if err != nil {
        t.Fatalf("HiPlan failed: %v", err)
    }

    t.Logf("=== HiPlan Results ===")
    t.Logf("Goal: %s", result.Plan.Goal)
    t.Logf("Milestones: %d", len(result.Plan.Milestones))
    t.Logf("Completed:  %d", result.Completed)
    t.Logf("Failed:     %d", result.Failed)
    t.Logf("Duration:   %s", result.Duration)

    for _, m := range result.Plan.Milestones {
        t.Logf("  [Milestone] %s: %d steps", m.Title, len(m.Steps))
        for _, s := range m.Steps {
            t.Logf("    [Step] %s: %s", s.ID, s.Description)
        }
    }

    t.Logf("Steps executed in order: %v", executor.executedSteps)
}
```

### Task 1.3: Record HiPlan Results

| Metric | Value |
|--------|-------|
| Milestones generated | |
| Total steps | |
| Steps executed | |
| Execution order | |
| Duration | |

---

## Exercise 2: MCTS — Exploratory Search (10 minutes)

### Task 2.1: Implement MCTS for Code Optimization

```go
// Mock action generator — generates code variants
type MockActionGenerator struct{}

func (g *MockActionGenerator) GenerateActions(
    ctx context.Context, node *planning.MCTSNode) ([]interface{}, error) {

    // In production: use LLM to generate different code approaches
    return []interface{}{
        "use_linear_search",
        "use_binary_search",
        "use_hash_map",
        "use_sorted_slice",
    }, nil
}

// Mock reward function — simulates test pass rates
type MockRewardFunction struct {
    scores map[string]float64
}

func newMockRewardFn() *MockRewardFunction {
    return &MockRewardFunction{
        scores: map[string]float64{
            "use_linear_search": 0.45,  // O(n) — slow
            "use_binary_search": 0.72,  // O(log n) — good for sorted
            "use_hash_map":      0.91,  // O(1) — best for lookup
            "use_sorted_slice":  0.68,  // O(n log n) sort + O(log n) lookup
        },
    }
}

func (r *MockRewardFunction) Reward(ctx context.Context,
    node *planning.MCTSNode, action interface{}) (float64, error) {

    key, _ := action.(string)
    score, ok := r.scores[key]
    if !ok {
        return 0.5, nil
    }
    return score, nil
}

func TestMCTS_CodeOptimization(t *testing.T) {
    ctx := context.Background()

    cfg := planning.DefaultMCTSConfig()
    cfg.MaxIterations = 50 // limited for lab; production uses 100+
    cfg.MaxDepth      = 5
    cfg.ExplorationC  = 1.414 // sqrt(2) — classic UCB constant

    mcts := planning.NewMCTS(cfg,
        &MockActionGenerator{},
        newMockRewardFn(),
        &planning.DefaultRolloutPolicy{},
    )

    initialState := map[string]interface{}{
        "problem": "find element in collection",
        "size":    "large",
    }

    result, err := mcts.Search(ctx, initialState)
    if err != nil {
        t.Fatalf("MCTS failed: %v", err)
    }

    t.Logf("=== MCTS Results ===")
    t.Logf("Best action:      %v", result.BestAction)
    t.Logf("Best score:       %.3f", result.BestScore)
    t.Logf("Nodes explored:   %d", result.NodesExplored)
    t.Logf("Iterations used:  %d", result.Iterations)
    t.Logf("Search duration:  %s", result.Duration)
}
```

### Task 2.2: Record MCTS Results

| Metric | Value |
|--------|-------|
| Best action found | |
| Best score | |
| Nodes explored | |
| Iterations used | |
| Expected winner (from scores) | `use_hash_map` (0.91) |
| Did MCTS find it? | |

---

## Exercise 3: Tree of Thoughts (5 minutes)

### Task 3.1: Run ToT for Architectural Decision

```go
// Mock thought generator
type MockThoughtGenerator struct{}

func (g *MockThoughtGenerator) Generate(
    ctx context.Context, parent *planning.ThoughtNode) ([]planning.Thought, error) {

    if parent == nil || parent.Depth == 0 {
        return []planning.Thought{
            {Content: "Microservices: independent scaling, complex coordination"},
            {Content: "Monolith: simple deployment, harder to scale independently"},
            {Content: "Modular monolith: bounded contexts, single deployment"},
        }, nil
    }
    // Deeper thoughts expand the selected branch
    return []planning.Thought{
        {Content: fmt.Sprintf("Detail A for: %s", parent.Thought.Content[:20])},
        {Content: fmt.Sprintf("Detail B for: %s", parent.Thought.Content[:20])},
    }, nil
}

// Mock thought evaluator — scores each thought branch
type MockThoughtEvaluator struct{}

func (e *MockThoughtEvaluator) Evaluate(
    ctx context.Context, thought planning.Thought) (float64, error) {

    if strings.Contains(thought.Content, "Modular") {
        return 0.88, nil // balanced approach scores highest
    }
    if strings.Contains(thought.Content, "Microservices") {
        return 0.71, nil
    }
    return 0.62, nil // monolith
}

func TestTreeOfThoughts_Architecture(t *testing.T) {
    ctx := context.Background()

    cfg := planning.DefaultTreeOfThoughtsConfig()
    cfg.BranchingFactor = 3
    cfg.MaxDepth        = 2  // shallow for lab
    cfg.BeamWidth       = 2  // keep top 2 branches

    tot := planning.NewTreeOfThoughts(cfg,
        &MockThoughtGenerator{},
        &MockThoughtEvaluator{},
    )

    result, err := tot.Run(ctx,
        "Choose architecture for a multi-tenant SaaS platform with variable load")
    if err != nil {
        t.Fatalf("ToT failed: %v", err)
    }

    t.Logf("=== Tree of Thoughts Results ===")
    t.Logf("Best thought:  %s", result.BestThought.Content)
    t.Logf("Best score:    %.3f", result.BestScore)
    t.Logf("Total nodes:   %d", result.TotalNodes)
    t.Logf("Pruned nodes:  %d", result.PrunedNodes)
    t.Logf("Depth reached: %d", result.MaxDepthReached)
}
```

### Task 3.3: Algorithm Comparison Table

| Dimension | HiPlan | MCTS | Tree of Thoughts |
|-----------|--------|------|-----------------|
| Best for | | | |
| Total LLM calls (this lab) | | | |
| Result type | | | |
| Deterministic? | | | |
| Handles uncertainty? | | | |

---

## Lab Completion Checklist

- [ ] HiPlan decomposed task into milestones and steps
- [ ] HiPlan executed all steps in correct order
- [ ] MCTS found the highest-scoring action (use_hash_map)
- [ ] ToT identified the best architectural approach
- [ ] Algorithm comparison table filled in

---

## Troubleshooting

### "MCTSConfig.MaxIterations must be > 0"
Use `planning.DefaultMCTSConfig()` as a base, then override specific fields.

### "ToT: no thoughts generated"
The ThoughtGenerator must return at least 1 thought for non-leaf nodes.
For leaf nodes (depth == MaxDepth), return an empty slice.

### Import errors
Ensure `go.mod` has: `replace digital.vasic.planning => ./Planning`

---

*Lab Version: 1.0.0*
*Last Updated: February 2026*
