# Module S7.2.1: Planning — HiPlan, MCTS, and Tree of Thoughts

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module S7.2.1: Planning Module
- Duration: 30 minutes
- Teaching AI Agents to Think Ahead

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Choose the right planning algorithm for each task type
- Implement HiPlan for hierarchical task decomposition
- Configure MCTS for exploratory planning under uncertainty
- Build Tree of Thoughts for open-ended reasoning
- Control planning cost with MaxDepth, MaxIterations, and BranchingFactor

---

## Slide 3: The Planning Problem

**Why single-shot LLM responses fail for complex tasks:**

| Problem | Consequence |
|---------|-------------|
| No lookahead | Commits to a path without exploring alternatives |
| No backtracking | Wrong sub-step contaminates all subsequent reasoning |
| No decomposition | Large tasks exceed context windows |
| No uncertainty | Cannot model "I don't know yet" states |

**The solution: three complementary planning algorithms:**
- **HiPlan** — structured tasks, known topology → hierarchical decomposition
- **MCTS** — game-like tasks, simulatable outcomes → Monte Carlo tree search
- **ToT** — open-ended reasoning, unknown path → breadth-first thought exploration

---

## Slide 4: Module Identity

**`digital.vasic.planning`**

| Property | Value |
|----------|-------|
| Module path | `digital.vasic.planning` |
| Go version | 1.25+ |
| Source directory | `Planning/` |
| HelixAgent adapter | `internal/adapters/planning/adapter.go` |
| Package | `planning` |
| Challenge | `challenges/scripts/planning_challenge.sh` |

---

## Slide 5: HiPlan — Hierarchical Planning

**For structured tasks with known milestone topology:**

```go
type HiPlan struct {
    config    *HiPlanConfig
    generator MilestoneGenerator // LLM-backed or custom
    executor  StepExecutor       // executes each step
}

type HiPlanConfig struct {
    MaxMilestones   int           // default: 5
    MaxStepsPerMile int           // default: 10
    TimeoutPerStep  time.Duration // default: 30s
}

func DefaultHiPlanConfig() *HiPlanConfig

// Core interfaces for extension
type MilestoneGenerator interface {
    GenerateMilestones(ctx context.Context,
        goal string) ([]Milestone, error)
}
type StepExecutor interface {
    ExecuteStep(ctx context.Context,
        step PlanStep, state interface{}) (interface{}, error)
}
```

---

## Slide 6: HiPlan Data Types

**Plan structure:**

```go
type HierarchicalPlan struct {
    Goal       string
    Milestones []Milestone
    CreatedAt  time.Time
}

type Milestone struct {
    ID          string
    Title       string
    Description string
    Steps       []PlanStep
    Completed   bool
}

type PlanStep struct {
    ID          string
    Description string
    Executor    string // which StepExecutor to use
    Input       interface{}
    Output      interface{}
    Error       error
}

type PlanResult struct {
    Plan      *HierarchicalPlan
    Completed int // milestones completed
    Failed    int
    Duration  time.Duration
}
```

---

## Slide 7: MCTS — Monte Carlo Tree Search

**For exploratory planning under uncertainty:**

```go
type MCTSConfig struct {
    MaxIterations int     // simulation budget; default: 100
    ExplorationC  float64 // UCB constant; classic value: 1.414 (sqrt(2))
    MaxDepth      int     // tree depth limit; default: 10
    RolloutDepth  int     // depth per rollout; default: 5
}

func DefaultMCTSConfig() *MCTSConfig

// Strategy interfaces
type MCTSActionGenerator interface {
    GenerateActions(ctx context.Context,
        node *MCTSNode) ([]interface{}, error)
}
type MCTSRewardFunction interface {
    Reward(ctx context.Context,
        node *MCTSNode, action interface{}) (float64, error)
}
type MCTSRolloutPolicy interface {
    Rollout(ctx context.Context,
        node *MCTSNode, depth int) (float64, error)
}

// Built-in implementations
type CodeActionGenerator  struct{ ... } // generates code variants
type CodeRewardFunction   struct{ ... } // runs tests for reward
type DefaultRolloutPolicy struct{ ... } // random rollout
```

---

## Slide 8: Tree of Thoughts

**For open-ended reasoning where the right path is unknown:**

```go
type TreeOfThoughtsConfig struct {
    BranchingFactor  int    // thoughts per node; default: 3
    MaxDepth         int    // tree depth; default: 4
    BeamWidth        int    // keep top-N branches; default: 2
    EvaluationPrompt string // prompt for ThoughtEvaluator
}

func DefaultTreeOfThoughtsConfig() *TreeOfThoughtsConfig

type ThoughtGenerator interface {
    Generate(ctx context.Context,
        parent *ThoughtNode) ([]Thought, error)
}
type ThoughtEvaluator interface {
    Evaluate(ctx context.Context,
        thought Thought) (float64, error)
}

// LLM-backed implementations
type LLMThoughtGenerator struct{ Client LLMClient }
type LLMThoughtEvaluator struct{ Client LLMClient }
```

---

## Slide 9: Algorithm Selection Guide

**Choosing the right algorithm:**

| Scenario | Algorithm | Why |
|----------|-----------|-----|
| Software feature breakdown | HiPlan | Known phases: Design → Impl → Test → Deploy |
| Code generation with unit tests | MCTS | Tests = reward signal; simulatable outcome |
| Math proof search | ToT | Unknown path; need to explore and backtrack |
| Research pipeline | HiPlan | Structured: Literature → Hypothesis → Experiment |
| Strategic decision making | ToT | Multiple valid paths; need to evaluate depth |
| Game playing / planning games | MCTS | Classic MCTS domain |

**Cost rule of thumb:**
- Each tree node = 1 LLM call
- HiPlan: `N_milestones × N_steps` calls
- MCTS: `MaxIterations × RolloutDepth` calls
- ToT: `BranchingFactor^MaxDepth / BeamWidth` calls (pruned)

---

## Slide 10: HelixAgent Integration

**How HelixAgent uses all three algorithms:**

```
SpecKit Orchestrator (internal/services/speckit_orchestrator.go)
  └── GranularityRefactoring → HiPlan
        ├── Milestone: "Analyze existing code"
        ├── Milestone: "Design new architecture"
        ├── Milestone: "Implement changes"
        └── Milestone: "Test and validate"

AI Debate System (Phase: Proposal)
  └── TreeOfThoughts → explore 3 proposal branches
        ├── Branch A: microservices approach
        ├── Branch B: monolith approach
        └── Branch C: hybrid approach → selected (score: 0.87)

Code Generation Skill
  └── MCTS → search for optimal implementation
        └── CodeRewardFunction: run go test ./... → pass rate
```

```bash
# Run planning via HelixAgent API
curl -X POST http://localhost:7061/v1/planning/run \
  -H "Content-Type: application/json" \
  -d '{"algorithm":"hiplan","goal":"Build REST API for user management"}'
```

---

## Speaker Notes

### Slide 3 Notes
The key insight: planning is about exploring the *space* of possible actions before committing.
A single LLM call commits immediately. MCTS, HiPlan, and ToT all explore before committing.
The difference is *how* they explore.

### Slide 7 Notes
UCB (Upper Confidence Bound) formula: score + C * sqrt(ln(parent_visits) / node_visits).
The sqrt(2) constant balances exploration (visiting new nodes) vs exploitation (revisiting
high-scoring nodes). In code generation tasks, you can tune this higher (more exploration)
or lower (more exploitation).

### Slide 9 Notes
The algorithm selection guide is the most practical slide. Students will return to this table
when choosing algorithms for their projects. Emphasize: start with HiPlan for structured tasks
(it's the cheapest); use ToT when you genuinely don't know which path is right.
