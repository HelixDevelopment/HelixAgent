# User Manual 36: Planning Algorithms Guide

## Overview

HelixAgent provides three AI planning algorithms for complex task decomposition and
decision-making: HiPlan (Hierarchical Planning), MCTS (Monte Carlo Tree Search), and
Tree of Thoughts (ToT). Each algorithm is exposed via a dedicated HTTP endpoint and is
backed by the Planning module (`digital.vasic.planning`).

These algorithms are useful for breaking down large goals into executable steps, finding
optimal action sequences under uncertainty, and exploring creative solutions through
structured reasoning.

## Core Concepts

### HiPlan -- Hierarchical Planning

HiPlan decomposes a high-level goal into milestones, each containing ordered steps with
contextual hints. It supports:

- **Milestone generation** -- LLM-driven decomposition of goals into 3-20 milestones
- **Step generation** -- Each milestone is further decomposed into 3-50 actionable steps
- **Hint generation** -- Contextual tips generated for each step to improve execution
- **Dependency tracking** -- Milestones can declare dependencies on other milestones
- **Topological sorting** -- Milestones are automatically sorted by dependency order
- **Parallel execution** -- Independent milestones run concurrently (configurable)
- **Adaptive planning** -- Continues past failures when enabled (does not stop on first error)
- **Retry with backoff** -- Failed steps are automatically retried

**Milestone states:** `pending` -> `in_progress` -> `completed` | `failed` | `skipped`

**Step states:** `pending` -> `in_progress` -> `completed` | `failed`

### MCTS -- Monte Carlo Tree Search

MCTS explores action sequences to find the optimal path from an initial state to a goal.
It implements the MASTER framework for code generation and planning with:

- **Selection** -- Navigate the tree using UCB1 or UCT-DP (depth-preferred UCT)
- **Expansion** -- Generate child nodes by applying possible actions to states
- **Simulation** -- Rollout from expanded nodes to estimate value
- **Backpropagation** -- Propagate rewards up the tree with discount factor

The UCT-DP formula adds a depth preference bonus to the standard UCB1 calculation,
encouraging deeper exploration when configured. The exploration constant (C) defaults
to sqrt(2) = 1.414.

**Node states:** `unexpanded` -> `expanded` | `terminal`

### Tree of Thoughts (ToT)

ToT solves problems by exploring a tree of intermediate reasoning steps. Each node is a
"thought" -- a reasoning step that can be evaluated, expanded, or pruned. Three search
strategies are available:

- **BFS (Breadth-First Search)** -- Explores all thoughts at each level before going deeper.
  Best for problems where the solution depth is unknown.
- **DFS (Depth-First Search)** -- Follows one chain of thought to completion before
  backtracking. Best for problems with clear sequential reasoning.
- **Beam Search** -- Keeps only the top-K thoughts at each level (controlled by
  `beam_width`). Best balance of exploration and efficiency.

**Thought states:** `pending` -> `active` -> `evaluated` -> `selected` | `pruned`

## Architecture

```
+---------------------------+
|     HTTP Handlers         |
| /v1/planning/{algorithm}  |
+---------------------------+
     |          |          |
+--------+ +--------+ +--------+
| HiPlan | |  MCTS  | |  ToT   |
+--------+ +--------+ +--------+
     |          |          |
+--------+ +--------+ +--------+
|Milestone| |Action  | |Thought |
|Generator| |Generator| |Generator|
+--------+ +--------+ +--------+
     |          |          |
+-----------------------------------+
| LLM Provider (via HelixAgent)     |
+-----------------------------------+
```

Each algorithm uses pluggable strategy interfaces:

- **HiPlan:** `MilestoneGenerator` + `StepExecutor`
- **MCTS:** `MCTSActionGenerator` + `MCTSRewardFunction` + `MCTSRolloutPolicy`
- **ToT:** `ThoughtGenerator` + `ThoughtEvaluator`

Default implementations use LLM-backed generation for all strategies.

## Getting Started

### Prerequisites

- HelixAgent running locally on port 7061
- At least one LLM provider configured

### Quick Start: Decompose a Goal with HiPlan

```bash
curl -X POST http://localhost:7061/v1/planning/hiplan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "goal": "Migrate the authentication system from JWT to OAuth2",
    "constraints": ["no downtime", "backward compatible for 30 days"],
    "config": {
      "max_milestones": 5,
      "max_steps_per_milestone": 7,
      "enable_parallel_milestones": true,
      "max_parallel_milestones": 2
    }
  }'
```

**Response:**

```json
{
  "id": "plan-1712345678",
  "goal": "Migrate the authentication system from JWT to OAuth2",
  "state": "created",
  "milestones": [
    {
      "id": "milestone-0",
      "name": "Design OAuth2 architecture alongside existing JWT",
      "state": "pending",
      "priority": 0,
      "steps": [
        {
          "id": "milestone-0-step-0",
          "action": "Audit current JWT endpoints and token lifecycle",
          "state": "pending"
        },
        {
          "id": "milestone-0-step-1",
          "action": "Define OAuth2 flow (authorization code with PKCE)",
          "state": "pending"
        }
      ]
    },
    {
      "id": "milestone-1",
      "name": "Implement OAuth2 provider integration",
      "state": "pending",
      "priority": 1,
      "dependencies": ["milestone-0"]
    }
  ],
  "progress": 0.0
}
```

### Quick Start: Find Optimal Path with MCTS

```bash
curl -X POST http://localhost:7061/v1/planning/mcts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "initial_state": {"codebase": "monolith", "tests": "passing", "coverage": "72%"},
    "available_actions": [
      "extract-service",
      "add-api-gateway",
      "setup-ci",
      "deploy-canary",
      "add-integration-tests"
    ],
    "config": {
      "iterations": 500,
      "exploration_weight": 1.41,
      "max_depth": 5,
      "discount_factor": 0.99,
      "enable_parallel": true,
      "parallel_workers": 4,
      "timeout": "3m"
    }
  }'
```

**Response:**

```json
{
  "best_actions": ["add-integration-tests", "extract-service", "add-api-gateway", "deploy-canary"],
  "final_reward": 0.87,
  "total_iterations": 500,
  "duration_ms": 45200,
  "root_visits": 500,
  "tree_size": 1247
}
```

### Quick Start: Creative Problem-Solving with Tree of Thoughts

```bash
curl -X POST http://localhost:7061/v1/planning/tot \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "problem": "Design a caching strategy for 10M daily requests with 99.9% hit rate",
    "strategy": "beam",
    "config": {
      "max_depth": 4,
      "branching_factor": 3,
      "beam_width": 2,
      "min_score": 0.3,
      "prune_threshold": 0.2,
      "temperature": 0.7
    }
  }'
```

**Response:**

```json
{
  "problem": "Design a caching strategy for 10M daily requests with 99.9% hit rate",
  "solution": [
    {"content": "Use a multi-tier cache: L1 in-memory (hot), L2 Redis (warm), L3 disk (cold)", "score": 0.82},
    {"content": "L1 uses LRU with 10K entries, TTL 60s. L2 uses LFU with 1M entries, TTL 1h", "score": 0.88},
    {"content": "Implement cache warming from access logs. Prefetch top-1000 keys on startup", "score": 0.91},
    {"content": "Add circuit breaker: if Redis latency > 50ms, serve stale from L1 with 5min grace", "score": 0.94}
  ],
  "best_score": 0.94,
  "iterations": 36,
  "duration_ms": 12400,
  "strategy": "beam",
  "tree_depth": 5,
  "nodes_explored": 42
}
```

## Configuration Reference

### HiPlan Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_milestones` | int | 20 | Maximum number of milestones to generate |
| `max_steps_per_milestone` | int | 50 | Maximum steps within a single milestone |
| `enable_parallel_milestones` | bool | true | Allow independent milestones to run concurrently |
| `max_parallel_milestones` | int | 3 | Maximum milestones executing in parallel |
| `enable_adaptive_planning` | bool | true | Continue past failures instead of stopping |
| `retry_failed_steps` | bool | true | Automatically retry failed steps |
| `max_retries` | int | 3 | Maximum retries per step |
| `timeout` | duration | 30m | Overall planning timeout |
| `step_timeout` | duration | 5m | Timeout per individual step |

### MCTS Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `exploration_constant` | float64 | 1.414 | C value in UCB1 formula (sqrt(2)) |
| `depth_preference_alpha` | float64 | 0.5 | Alpha for UCT-DP depth preference bonus |
| `max_depth` | int | 50 | Maximum search depth |
| `max_iterations` | int | 1000 | Total MCTS iterations |
| `rollout_depth` | int | 10 | Depth for simulation rollouts |
| `simulation_count` | int | 1 | Number of simulations per expansion |
| `discount_factor` | float64 | 0.99 | Discount for future rewards in backpropagation |
| `enable_parallel` | bool | true | Enable parallel simulations |
| `parallel_workers` | int | 4 | Number of parallel simulation workers |
| `timeout` | duration | 5m | Search timeout |
| `use_uct_dp` | bool | true | Use depth-preferred UCT formula |

### Tree of Thoughts Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_depth` | int | 10 | Maximum depth of the thought tree |
| `max_branches` | int | 5 | Maximum branches (child thoughts) per node |
| `min_score` | float64 | 0.3 | Minimum score for a thought to be considered |
| `prune_threshold` | float64 | 0.2 | Score below which thoughts are pruned |
| `search_strategy` | string | "beam" | One of: "bfs", "dfs", "beam" |
| `beam_width` | int | 3 | Top-K thoughts to keep at each level (beam only) |
| `temperature` | float64 | 0.7 | Diversity parameter for thought generation |
| `enable_backtracking` | bool | true | Allow backtracking on dead ends |
| `max_iterations` | int | 100 | Maximum total iterations |
| `timeout` | duration | 5m | Search timeout |

## Complete Code Examples

### Example 1: Feature Development Plan

```bash
curl -X POST http://localhost:7061/v1/planning/hiplan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "goal": "Add real-time collaborative editing to the document editor",
    "constraints": [
      "Must support 50 concurrent editors",
      "Conflict resolution via OT or CRDT",
      "Mobile browser support required"
    ],
    "config": {
      "max_milestones": 6,
      "max_steps_per_milestone": 10,
      "enable_parallel_milestones": true,
      "max_parallel_milestones": 3,
      "timeout": "20m"
    }
  }'
```

### Example 2: Optimization with Constrained MCTS

```bash
curl -X POST http://localhost:7061/v1/planning/mcts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "initial_state": {
      "budget": 10000,
      "team_size": 5,
      "deadline_weeks": 8,
      "features_pending": ["auth", "search", "payments", "notifications", "analytics"]
    },
    "available_actions": [
      "implement-auth",
      "implement-search",
      "implement-payments",
      "implement-notifications",
      "implement-analytics",
      "hire-contractor",
      "extend-deadline"
    ],
    "config": {
      "iterations": 2000,
      "exploration_weight": 2.0,
      "max_depth": 8,
      "rollout_depth": 5,
      "discount_factor": 0.95,
      "timeout": "5m"
    }
  }'
```

### Example 3: Architecture Design with DFS Tree of Thoughts

```bash
curl -X POST http://localhost:7061/v1/planning/tot \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "problem": "Design a distributed event sourcing system that handles 100K events/second with exactly-once delivery guarantees and sub-10ms query latency",
    "strategy": "dfs",
    "config": {
      "max_depth": 6,
      "branching_factor": 3,
      "min_score": 0.4,
      "prune_threshold": 0.25,
      "temperature": 0.8,
      "enable_backtracking": true,
      "max_iterations": 50,
      "timeout": "3m"
    }
  }'
```

### Example 4: BFS for Broad Exploration

```bash
curl -X POST http://localhost:7061/v1/planning/tot \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "problem": "What are the best approaches to reduce LLM inference latency by 50% while maintaining output quality?",
    "strategy": "bfs",
    "config": {
      "max_depth": 3,
      "branching_factor": 5,
      "min_score": 0.3,
      "beam_width": 4,
      "temperature": 0.9,
      "max_iterations": 75
    }
  }'
```

### Example 5: Execute a HiPlan Plan

After creating a plan, execute it to run all steps:

```bash
# Create the plan
PLAN_ID=$(curl -s -X POST http://localhost:7061/v1/planning/hiplan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "goal": "Set up monitoring for the production API",
    "config": {
      "max_milestones": 4,
      "enable_adaptive_planning": true,
      "retry_failed_steps": true,
      "max_retries": 2
    }
  }' | jq -r '.id')

# Execute the plan
curl -X POST http://localhost:7061/v1/planning/hiplan/$PLAN_ID/execute \
  -H "Authorization: Bearer $HELIX_API_KEY"
```

**Execution response:**

```json
{
  "plan_id": "plan-1712345678",
  "success": true,
  "completed_milestones": 4,
  "failed_milestones": 0,
  "duration_ms": 85000,
  "milestone_results": [
    {
      "milestone_id": "milestone-0",
      "success": true,
      "step_results": [
        {"success": true, "duration": 5200},
        {"success": true, "duration": 3100}
      ],
      "duration": 8300
    }
  ]
}
```

## Choosing an Algorithm

| Scenario | Algorithm | Why |
|----------|-----------|-----|
| Break down a feature into tasks | HiPlan | Natural hierarchical decomposition with dependencies |
| Find optimal deployment sequence | MCTS | Handles uncertainty and explores many paths |
| Design system architecture | ToT (beam) | Creative exploration with quality filtering |
| Migration planning | HiPlan | Clear milestones, ordered steps, parallel execution |
| Performance optimization | MCTS | Reward-based action selection finds optimal sequences |
| Debugging complex issues | ToT (DFS) | Deep reasoning chains follow one thread to completion |
| Brainstorming alternatives | ToT (BFS) | Breadth-first explores many options at each level |
| Resource allocation | MCTS | Multi-variable optimization with discount factors |
| Step-by-step tutorials | HiPlan | Generates ordered, actionable instructions |

## API Reference

### POST /v1/planning/hiplan

Create a hierarchical plan for a goal.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `goal` | string | Yes | The high-level goal to plan for |
| `constraints` | array | No | Constraints the plan must respect |
| `config` | object | No | HiPlanConfig overrides |

### POST /v1/planning/hiplan/:id/execute

Execute a previously created plan.

### POST /v1/planning/mcts

Run MCTS search from an initial state.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `initial_state` | object | Yes | Starting state for the search |
| `available_actions` | array | No | Possible actions (auto-generated if omitted) |
| `config` | object | No | MCTSConfig overrides |

### POST /v1/planning/tot

Solve a problem using Tree of Thoughts.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `problem` | string | Yes | The problem to solve |
| `strategy` | string | No | Search strategy: "bfs", "dfs", "beam" (default: "beam") |
| `config` | object | No | TreeOfThoughtsConfig overrides |

## Troubleshooting

### HiPlan generates too many milestones

- Set `max_milestones` to a lower value (e.g., 5).
- Provide more specific constraints to narrow the scope.
- Break very large goals into sub-goals and plan each separately.

### MCTS returns low-reward paths

- Increase `iterations` (default 1000). More iterations allow better tree exploration.
- Adjust `exploration_constant` -- higher values (2.0+) explore more, lower values
  (0.5-1.0) exploit known good paths.
- Check that your reward function produces meaningful gradients (not just 0/1).
- Increase `rollout_depth` for more accurate value estimates.

### Tree of Thoughts prunes good ideas

- Lower the `prune_threshold` (default 0.2). Very aggressive pruning can discard
  promising early thoughts that score low initially.
- Increase `beam_width` to keep more candidates alive.
- Switch from BFS to beam search for better quality-efficiency tradeoff.

### Planning timeout

- All three algorithms have configurable timeouts. Increase if needed.
- For MCTS, reduce `iterations` rather than increasing timeout for faster results.
- For HiPlan, reduce `max_steps_per_milestone` to generate simpler plans.

### Parallel milestone execution deadlocks

- Ensure no circular dependencies between milestones. The topological sort handles
  acyclic dependencies, but cycles cause milestones to be appended in original order.
- If milestones with unmet dependencies are encountered during parallel execution,
  they fall back to sequential execution automatically.

## Best Practices

1. **Start with HiPlan for project planning** -- it produces the most actionable output
   with clear milestone-step structure.
2. **Use MCTS when you need to optimize** -- it excels at finding the best sequence of
   actions when you can define a reward function.
3. **Use ToT beam search by default** -- it provides the best balance of exploration
   quality and computational cost.
4. **Set reasonable timeouts** -- planning should take seconds to minutes, not hours.
5. **Provide constraints to HiPlan** -- the more specific your constraints, the more
   focused the generated plan.
6. **Tune exploration vs exploitation** -- for MCTS, start with the default C=1.414
   and adjust based on results.
7. **Monitor tree size for MCTS** -- if `tree_size` in the response is very large
   (>10K nodes), consider reducing `max_depth` or `iterations`.

## Related Documentation

- [Agentic Workflows Guide](34-agentic-workflows-guide.md) -- For executing plans
  as automated workflows
- [LLMOps Experimentation Guide](35-llmops-experimentation-guide.md) -- For A/B
  testing different planning configurations
- [Benchmark Guide](37-benchmark-guide.md) -- For evaluating planning quality
