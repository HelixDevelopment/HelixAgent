# Video Course 79: Planning Algorithms Masterclass

## Course Overview

**Duration:** 3 hours
**Level:** Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 68 (Planning Algorithms), Course 77 (Agentic Workflows Deep Dive)

A deep technical dive into HelixAgent's three planning algorithms: HiPlan (hierarchical
task network planning), MCTS (Monte Carlo Tree Search), and Tree of Thoughts (ToT).
Learn when to choose each approach, how to configure them for your domain, and how to
combine them with agentic workflows for autonomous AI problem-solving.

---

## Learning Objectives

By the end of this course, you will be able to:

1. Explain the theoretical foundations of HiPlan, MCTS, and Tree of Thoughts
2. Configure each algorithm for a given planning problem
3. Evaluate plan quality using built-in scoring and comparison utilities
4. Combine planners with agentic workflow execution
5. Run and interpret planning endpoint responses via the REST API
6. Select the right algorithm based on problem structure, branching factor, and time budget

---

## Module 1: HiPlan — Hierarchical Planning (40 min)

### Video 1.1: Hierarchical Task Networks (20 min)

**Topics:**
- What is hierarchical planning? Goals decomposed into sub-goals recursively
- Task vs. method distinction: a task is the goal; a method is one way to achieve it
- Primitives (executable actions) vs. compound tasks (require further decomposition)
- The planning domain: operators, preconditions, effects
- How HelixAgent's `HiPlan` uses an LLM to generate decomposition steps
- Key file: `Planning/planning.go` (HiPlan implementation)

**HiPlan Concept:**
```
Goal: Deploy a new microservice
  |
  ├── Write service code
  │     ├── Define API contract   [primitive]
  │     ├── Implement handlers    [primitive]
  │     └── Write tests           [primitive]
  |
  ├── Build and containerize
  │     ├── Write Dockerfile      [primitive]
  │     └── Build image           [primitive]
  |
  └── Deploy
        ├── Push image to registry [primitive]
        └── Update compose file    [primitive]
```

**HiPlan Request:**
```bash
curl -X POST http://localhost:7061/v1/planning/hiplan \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "Deploy a new Go microservice to production",
    "context": "We use Docker Compose. Service must expose /health endpoint.",
    "max_depth": 4,
    "provider": "claude"
  }'
```

**HiPlan Response:**
```json
{
  "plan_id": "plan-abc123",
  "algorithm": "hiplan",
  "goal": "Deploy a new Go microservice to production",
  "tasks": [
    {
      "id": "t1", "description": "Define API contract",
      "type": "primitive", "depth": 2, "parent": "Write service code"
    },
    {
      "id": "t2", "description": "Implement handlers",
      "type": "primitive", "depth": 2, "parent": "Write service code"
    }
  ],
  "execution_order": ["t1", "t2", "t3", "t4", "t5", "t6"],
  "estimated_steps": 6
}
```

### Video 1.2: Configuring HiPlan (20 min)

**Topics:**
- `max_depth`: controls decomposition depth; too deep causes hallucination drift
- `provider`: which LLM generates the decomposition (Claude recommended for structure)
- `context`: inject domain knowledge to guide decomposition
- Handling ambiguous goals: adding clarifying context to reduce variance
- Post-processing: validating that all leaf tasks are executable primitives
- Integration with agentic workflows: feeding HiPlan output into `ExecutionPlanner`

**Depth Guidance:**
```
max_depth=2  Best for: well-defined, narrow goals (write a function)
max_depth=3  Best for: moderate complexity (build a feature)
max_depth=4  Best for: complex goals (deploy a full service)
max_depth=5+ Risk:     hallucination drift, repetitive sub-tasks
```

---

## Module 2: MCTS — Monte Carlo Tree Search (40 min)

### Video 2.1: MCTS Theory and Implementation (20 min)

**Topics:**
- Four MCTS phases: Selection, Expansion, Simulation (rollout), Backpropagation
- UCT (Upper Confidence Bound for Trees): balancing exploration vs. exploitation
- How MCTS applies to LLM planning: each node is a partial plan; rollout simulates completion
- The `MCTSPlanner` struct: `NumSimulations`, `ExplorationConstant`, `MaxDepth`
- LLM-scored rollouts: using a fast model to evaluate simulated plan quality
- Key file: `Planning/planning.go` (MCTS implementation)

**MCTS Phases:**
```
Selection:       Walk tree using UCT to find most promising node
                 UCT(node) = Q/N + C * sqrt(ln(N_parent) / N)

Expansion:       Generate child nodes (plan continuations) via LLM

Simulation:      Rollout to terminal state using fast LLM scoring

Backpropagation: Update Q and N values from leaf to root
```

**MCTS Request:**
```bash
curl -X POST http://localhost:7061/v1/planning/mcts \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "Optimize database query performance",
    "num_simulations": 50,
    "exploration_constant": 1.41,
    "max_depth": 5,
    "provider": "gemini",
    "scorer_provider": "deepseek"
  }'
```

### Video 2.2: Tuning MCTS for Your Problem (20 min)

**Topics:**
- `num_simulations`: more simulations = better quality, higher latency and cost
- `exploration_constant` C: 1.41 (sqrt(2)) is a common default; increase for wider search
- `max_depth`: caps the tree depth; set based on expected plan length
- `scorer_provider`: use a fast/cheap model (DeepSeek, Gemini Flash) for rollout scoring
- When MCTS beats HiPlan: adversarial or stochastic domains where greedy decomposition fails
- Visualizing the search tree: `GET /v1/planning/mcts/{plan_id}/tree`

**Simulation Count vs. Quality Trade-off:**
```
num_simulations=10   Latency: ~2s   Quality: exploratory, may miss best plan
num_simulations=50   Latency: ~8s   Quality: good for most tasks
num_simulations=100  Latency: ~15s  Quality: high; use for critical decisions
num_simulations=200  Latency: ~30s  Quality: near-optimal; use sparingly
```

---

## Module 3: Tree of Thoughts (40 min)

### Video 3.1: ToT Theory and Implementation (20 min)

**Topics:**
- Tree of Thoughts as deliberate, multi-path reasoning: explore multiple chains of thought
- Three ToT search strategies: BFS (breadth-first), DFS (depth-first), beam search
- Thought evaluation: the LLM scores each thought as "sure / likely / impossible"
- Pruning: discarding low-scoring branches to manage cost
- When ToT excels: open-ended creative tasks, math reasoning, strategy problems
- Key file: `Planning/planning.go` (ToT implementation)

**ToT Strategies:**
```
BFS:         Explore all thoughts at each depth before going deeper
             Best for: ensuring breadth of coverage at each step

DFS:         Follow one branch to maximum depth before backtracking
             Best for: tasks with long sequential reasoning chains

Beam search: Keep top-k thoughts at each level; prune the rest
             Best for: balancing quality and cost (recommended default)
```

**ToT Request:**
```bash
curl -X POST http://localhost:7061/v1/planning/tot \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "Design a caching strategy for a high-read API",
    "search_strategy": "beam",
    "beam_width": 3,
    "max_depth": 4,
    "num_thoughts_per_node": 3,
    "provider": "claude",
    "evaluator_provider": "claude"
  }'
```

### Video 3.2: Comparing ToT to Chain-of-Thought (20 min)

**Topics:**
- Chain-of-Thought (CoT): single reasoning chain, greedy; fast but brittle
- ToT: multiple branching chains, evaluated; slower but more robust
- When CoT is sufficient: straightforward factual or procedural tasks
- When ToT is necessary: tasks requiring backtracking, creative alternatives, or verification
- Cost model: `num_thoughts_per_node ^ max_depth` LLM calls — keep both small
- Result consolidation: merging the best branches into a coherent final plan

**Cost Comparison:**
```
Chain-of-Thought: 1 LLM call (fast, cheap)
ToT beam=3 depth=3: 3^3 = 27 evaluations + 3^3 = 27 generations = ~54 calls
ToT beam=2 depth=4: 2^4 = 16 evaluations + 16 generations = ~32 calls (pruning saves cost)
```

---

## Module 4: Comparing the Approaches (20 min)

### Video 4.1: Algorithm Selection Guide (10 min)

**Topics:**
- Decision matrix: task type vs. recommended algorithm
- HiPlan strengths: structured, hierarchical, predictable decomposition
- MCTS strengths: stochastic/adversarial domains, when the evaluation function is clear
- ToT strengths: open-ended reasoning, tasks requiring creative exploration
- Combining planners: use ToT for high-level strategy, HiPlan to decompose chosen strategy

**Selection Matrix:**
```
Task Characteristics         Recommended Algorithm
────────────────────────────────────────────────────
Sequential, well-defined     HiPlan
Adversarial, stochastic      MCTS
Creative, open-ended         Tree of Thoughts
Requires verification        ToT or MCTS (both evaluate)
Budget-constrained           HiPlan (fewest LLM calls)
Time-constrained             HiPlan or MCTS (num_simulations=20)
```

### Video 4.2: Side-by-Side Demo (10 min)

**Topics:**
- Run the same goal through all three algorithms
- Compare output plans: structure, depth, creativity
- Compare latency and estimated LLM call count
- The planning comparison endpoint: `POST /v1/planning/compare`

**Compare All Three:**
```bash
curl -X POST http://localhost:7061/v1/planning/compare \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "Improve the reliability of our payment processing pipeline",
    "algorithms": ["hiplan", "mcts", "tot"],
    "provider": "claude"
  }'
```

---

## Module 5: Integration with Agentic Workflows (40 min)

### Video 5.1: Planning to Execution Pipeline (20 min)

**Topics:**
- Using a planner's output to populate a `WorkflowGraph` automatically
- The `PlanToWorkflow` adapter: converts plan tasks to workflow nodes with dependencies
- Injecting the plan into the `ExecutionPlanner` for `AgenticEnsemble` execution
- Handling plan revision: if execution fails, re-plan from the failure point
- End-to-end flow: ToT → winning plan → HiPlan decomposition → AgenticEnsemble execution

**Plan to Workflow Conversion:**
```go
plan, err := planner.Plan(ctx, goal, config)
if err != nil {
    return err
}

graph, err := adapter.PlanToWorkflow(plan, workflowOptions)
if err != nil {
    return err
}

result, err := agenticEnsemble.Execute(ctx, graph, state)
```

### Video 5.2: Replanning on Failure (20 min)

**Topics:**
- Detecting execution failures: node error threshold triggers replan signal
- Partial plan preservation: completed tasks are excluded from the new plan
- Providing failure context to the planner: which task failed and why
- Limiting replan attempts to prevent infinite loops
- Prometheus counter: `helix_plan_replan_total` tracks replan frequency

**Replan Request:**
```bash
curl -X POST http://localhost:7061/v1/planning/hiplan \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "Deploy a new Go microservice to production",
    "completed_tasks": ["t1", "t2", "t3"],
    "failure_context": "Task t4 failed: Docker daemon not running",
    "max_depth": 3,
    "provider": "claude"
  }'
```

---

## Key Takeaways

- HiPlan is the best default for structured, hierarchical goals; it produces clean,
  predictable task breakdowns with the fewest LLM calls.
- MCTS is superior when you have a reliable quality scoring function and need to explore
  a stochastic solution space systematically.
- Tree of Thoughts excels at open-ended creative and reasoning tasks where backtracking
  and alternative exploration are necessary.
- All three planners integrate with the AgenticEnsemble execution pipeline: plan first,
  then execute the resulting task graph.
- Choose `scorer_provider` / `evaluator_provider` as a fast, cheap model to reduce cost
  without sacrificing plan quality.

---

## Related Courses

- **Course 68: Planning Algorithms** — Introductory overview of the Planning module
- **Course 77: Agentic Workflows Deep Dive** — Execute the plans produced by these algorithms
- **Course 76: Agentic Ensemble** — The execution engine that runs planning outputs
- **Course 78: LLMOps Experimentation** — A/B test different planners on the same goal
