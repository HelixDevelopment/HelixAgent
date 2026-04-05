# User Manual 34: Agentic Workflows Guide

## Overview

HelixAgent's Agentic Workflows provide graph-based workflow orchestration for multi-step AI
tasks. Define nodes (processing steps), edges (transitions), and let the engine execute them
with checkpointing, state management, conditional branching, and automatic retries.

The Agentic module (`digital.vasic.agentic`) is a standalone Go library that powers the
workflow endpoints. It supports six node types, conditional edge traversal, parallel
execution, human-in-the-loop gates, nested subgraphs, and checkpoint-based recovery.

## Core Concepts

### Workflow Graph

Every workflow is a directed graph consisting of:

- **Nodes** -- Individual processing steps. Each node has a type, a handler function, and
  optional retry policy.
- **Edges** -- Directed connections between nodes. Edges may carry conditions that control
  whether the transition is taken.
- **Entry Point** -- The node where execution begins.
- **End Nodes** -- Nodes that, when reached, terminate the workflow successfully.

### Node Types

The engine supports six distinct node types:

| Type | Constant | Purpose |
|------|----------|---------|
| Agent | `agent` | LLM-based agent that processes messages and generates responses |
| Tool | `tool` | Executes a registered tool (file system, database, API call) |
| Condition | `condition` | Evaluates a condition and routes to different branches |
| Parallel | `parallel` | Executes multiple child branches concurrently |
| Human | `human` | Pauses execution until human approval is received |
| Subgraph | `subgraph` | Embeds a nested workflow as a single node |

### Workflow State

`WorkflowState` is a mutable object threaded through every node during execution. It tracks:

- `CurrentNode` -- Which node is currently executing
- `Messages` -- Accumulated conversation messages
- `Variables` -- Key-value store shared across all nodes
- `History` -- Record of every node execution (input, output, timing, errors)
- `Checkpoints` -- Saved snapshots for recovery
- `Status` -- One of: `pending`, `running`, `paused`, `completed`, `failed`

### Node Handler

Every node runs a handler function with this signature:

```go
type NodeHandler func(ctx context.Context, state *WorkflowState, input *NodeInput) (*NodeOutput, error)
```

The handler receives the current state and input (including the previous node's output), and
returns output that may include:

- `Result` -- Any result data
- `Messages` -- New messages to append to state
- `ToolCalls` -- Tool invocations to execute
- `NextNode` -- Override the default next node (skip ahead or loop back)
- `ShouldEnd` -- Signal immediate workflow termination

## Architecture

```
+--------------------+
|   HTTP Handler     |  POST /v1/agentic/workflows
+--------------------+
         |
+--------------------+
| Workflow Engine    |  Executes the graph loop
| (executeLoop)     |
+--------------------+
    |         |         |
+-------+ +-------+ +-------+
| Node  | | Node  | | Node  |  Agent / Tool / Condition / etc.
+-------+ +-------+ +-------+
    |                   |
+--------------------+  |
| WorkflowState     |--+  Shared state with checkpoints
+--------------------+
```

The execution loop runs until one of these conditions is met:

1. The current node is an end node
2. A node output sets `ShouldEnd = true`
3. No outgoing edges exist from the current node
4. The maximum iteration count is reached (default: 100)
5. The timeout expires (default: 30 minutes)

## Getting Started

### Prerequisites

- HelixAgent running locally on port 7061
- At least one LLM provider configured (for agent-type nodes)

### Quick Start: Create a Simple Workflow

This example creates a two-step workflow where an LLM analyzes code and then summarizes
the findings.

```bash
curl -X POST http://localhost:7061/v1/agentic/workflows \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "name": "code-review-workflow",
    "nodes": [
      {
        "id": "analyze",
        "type": "agent",
        "config": {
          "provider": "claude",
          "model": "claude-3-sonnet",
          "system_prompt": "You are a code reviewer. Analyze the code for bugs and style issues."
        }
      },
      {
        "id": "summarize",
        "type": "agent",
        "config": {
          "provider": "deepseek",
          "model": "deepseek-coder",
          "system_prompt": "Summarize the code review findings into a concise report."
        }
      }
    ],
    "edges": [
      {"from": "analyze", "to": "summarize"}
    ],
    "entry_point": "analyze",
    "end_nodes": ["summarize"],
    "input": {
      "query": "Review this function:\nfunc GetUser(id string) *User {\n  return db.users[id]\n}"
    }
  }'
```

**Response:**

```json
{
  "id": "wf-a1b2c3d4",
  "name": "code-review-workflow",
  "status": "completed",
  "result": {
    "summary": "The function has a potential nil map access and missing error handling...",
    "nodes_executed": 2
  },
  "execution_time_ms": 3200
}
```

### Check Workflow Status

For long-running workflows, poll the status endpoint:

```bash
curl http://localhost:7061/v1/agentic/workflows/wf-a1b2c3d4 \
  -H "Authorization: Bearer $HELIX_API_KEY"
```

## Configuration Reference

### WorkflowConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_iterations` | int | 100 | Maximum number of node executions before forced termination |
| `timeout` | duration | 30m | Overall workflow timeout |
| `enable_checkpoints` | bool | true | Whether to save state checkpoints during execution |
| `checkpoint_interval` | int | 5 | Create a checkpoint every N iterations |
| `enable_self_correction` | bool | true | Allow nodes to retry and self-correct on failure |
| `max_retries` | int | 3 | Default maximum retries per node |
| `retry_delay` | duration | 1s | Default delay between retries |

### Node RetryPolicy

Each node can override the global retry settings:

| Field | Type | Description |
|-------|------|-------------|
| `max_retries` | int | Maximum retry attempts for this specific node |
| `delay` | duration | Initial delay before first retry |
| `backoff` | float64 | Multiplier applied to delay after each retry (e.g., 2.0 for exponential) |

### Edge Conditions

Edges can include a condition function that is evaluated against the current workflow state.
The edge is only traversed if the condition returns true. When multiple edges leave a node,
the first edge whose condition matches (or has no condition) is selected.

## Complete Code Examples

### Example 1: Conditional Branching Workflow

This workflow classifies code as safe or unsafe, then routes to different handling branches.

```bash
curl -X POST http://localhost:7061/v1/agentic/workflows \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "name": "security-check-workflow",
    "nodes": [
      {
        "id": "classify",
        "type": "agent",
        "config": {
          "provider": "claude",
          "model": "claude-3-sonnet",
          "system_prompt": "Classify the code as SAFE or UNSAFE. Respond with just the word."
        }
      },
      {
        "id": "route",
        "type": "condition",
        "config": {
          "condition_field": "classification",
          "routes": {
            "SAFE": "approve",
            "UNSAFE": "flag"
          }
        }
      },
      {
        "id": "approve",
        "type": "agent",
        "config": {
          "provider": "deepseek",
          "model": "deepseek-coder",
          "system_prompt": "Generate an approval message for this safe code."
        }
      },
      {
        "id": "flag",
        "type": "agent",
        "config": {
          "provider": "deepseek",
          "model": "deepseek-coder",
          "system_prompt": "Generate a detailed security report for this unsafe code."
        }
      }
    ],
    "edges": [
      {"from": "classify", "to": "route"},
      {"from": "route", "to": "approve", "label": "safe"},
      {"from": "route", "to": "flag", "label": "unsafe"}
    ],
    "entry_point": "classify",
    "end_nodes": ["approve", "flag"],
    "input": {
      "query": "Review this code for unsafe shell invocation patterns"
    }
  }'
```

### Example 2: Multi-Step Pipeline with Tool Calls

```bash
curl -X POST http://localhost:7061/v1/agentic/workflows \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "name": "research-pipeline",
    "config": {
      "max_iterations": 50,
      "timeout": "10m",
      "enable_checkpoints": true,
      "checkpoint_interval": 3,
      "max_retries": 2,
      "retry_delay": "2s"
    },
    "nodes": [
      {
        "id": "search",
        "type": "tool",
        "config": {
          "tool_name": "semantic-search",
          "parameters": {"collection": "knowledge-base", "top_k": 5}
        }
      },
      {
        "id": "analyze",
        "type": "agent",
        "config": {
          "provider": "gemini",
          "model": "gemini-pro",
          "system_prompt": "Analyze the search results and extract key insights."
        }
      },
      {
        "id": "synthesize",
        "type": "agent",
        "config": {
          "provider": "claude",
          "model": "claude-3-sonnet",
          "system_prompt": "Synthesize the analysis into a comprehensive report."
        }
      }
    ],
    "edges": [
      {"from": "search", "to": "analyze"},
      {"from": "analyze", "to": "synthesize"}
    ],
    "entry_point": "search",
    "end_nodes": ["synthesize"],
    "input": {
      "query": "Latest advances in transformer architecture optimization"
    }
  }'
```

### Example 3: Iterative Refinement with Self-Correction

This workflow loops between a generator and a critic until the critic approves the output.

```bash
curl -X POST http://localhost:7061/v1/agentic/workflows \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "name": "iterative-refinement",
    "config": {
      "max_iterations": 20,
      "timeout": "15m",
      "enable_self_correction": true
    },
    "nodes": [
      {
        "id": "generate",
        "type": "agent",
        "config": {
          "provider": "deepseek",
          "model": "deepseek-coder",
          "system_prompt": "Generate a Go function based on the requirements. If previous feedback is provided, incorporate it."
        }
      },
      {
        "id": "critique",
        "type": "agent",
        "config": {
          "provider": "claude",
          "model": "claude-3-sonnet",
          "system_prompt": "Review the generated code. If it meets all requirements, respond with APPROVED. Otherwise, provide specific feedback for improvement."
        }
      },
      {
        "id": "check",
        "type": "condition",
        "config": {
          "condition_field": "approval_status",
          "routes": {
            "APPROVED": "done",
            "NEEDS_WORK": "generate"
          }
        }
      },
      {
        "id": "done",
        "type": "agent",
        "config": {
          "provider": "gemini",
          "model": "gemini-pro",
          "system_prompt": "Format the approved code with documentation."
        }
      }
    ],
    "edges": [
      {"from": "generate", "to": "critique"},
      {"from": "critique", "to": "check"},
      {"from": "check", "to": "generate", "label": "needs_work"},
      {"from": "check", "to": "done", "label": "approved"}
    ],
    "entry_point": "generate",
    "end_nodes": ["done"],
    "input": {
      "query": "Write a thread-safe LRU cache in Go with TTL support"
    }
  }'
```

### Example 4: Human-in-the-Loop Approval

```bash
curl -X POST http://localhost:7061/v1/agentic/workflows \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "name": "deployment-approval",
    "nodes": [
      {
        "id": "plan",
        "type": "agent",
        "config": {
          "provider": "claude",
          "model": "claude-3-sonnet",
          "system_prompt": "Create a deployment plan for the proposed changes."
        }
      },
      {
        "id": "review",
        "type": "human",
        "config": {
          "prompt": "Please review the deployment plan and approve or reject.",
          "timeout": "24h"
        }
      },
      {
        "id": "deploy",
        "type": "tool",
        "config": {
          "tool_name": "deploy",
          "parameters": {"environment": "staging"}
        }
      }
    ],
    "edges": [
      {"from": "plan", "to": "review"},
      {"from": "review", "to": "deploy"}
    ],
    "entry_point": "plan",
    "end_nodes": ["deploy"],
    "input": {
      "query": "Deploy authentication service v2.3.0 to staging"
    }
  }'
```

### Example 5: Parallel Fan-Out and Fan-In

```bash
curl -X POST http://localhost:7061/v1/agentic/workflows \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "name": "parallel-analysis",
    "nodes": [
      {
        "id": "split",
        "type": "parallel",
        "config": {
          "branches": ["security-check", "performance-check", "style-check"]
        }
      },
      {
        "id": "security-check",
        "type": "agent",
        "config": {
          "provider": "claude",
          "model": "claude-3-sonnet",
          "system_prompt": "Analyze the code for security vulnerabilities."
        }
      },
      {
        "id": "performance-check",
        "type": "agent",
        "config": {
          "provider": "deepseek",
          "model": "deepseek-coder",
          "system_prompt": "Analyze the code for performance issues."
        }
      },
      {
        "id": "style-check",
        "type": "agent",
        "config": {
          "provider": "gemini",
          "model": "gemini-pro",
          "system_prompt": "Check code style and formatting."
        }
      },
      {
        "id": "merge",
        "type": "agent",
        "config": {
          "provider": "claude",
          "model": "claude-3-sonnet",
          "system_prompt": "Merge all analysis reports into a unified code review."
        }
      }
    ],
    "edges": [
      {"from": "split", "to": "security-check"},
      {"from": "split", "to": "performance-check"},
      {"from": "split", "to": "style-check"},
      {"from": "security-check", "to": "merge"},
      {"from": "performance-check", "to": "merge"},
      {"from": "style-check", "to": "merge"}
    ],
    "entry_point": "split",
    "end_nodes": ["merge"],
    "input": {
      "query": "Review the submitted pull request code"
    }
  }'
```

## API Reference

### POST /v1/agentic/workflows

Create and execute a new workflow.

**Request Body:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Workflow name |
| `description` | string | No | Human-readable description |
| `nodes` | array | Yes | Array of node definitions |
| `edges` | array | Yes | Array of edge definitions |
| `entry_point` | string | Yes | ID of the starting node |
| `end_nodes` | array | Yes | IDs of terminal nodes |
| `config` | object | No | WorkflowConfig overrides |
| `input` | object | No | Initial input data |

**Response:** `200 OK` with workflow execution result.

### GET /v1/agentic/workflows/:id

Retrieve the status and result of a workflow execution.

**Response:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Workflow execution ID |
| `name` | string | Workflow name |
| `status` | string | One of: pending, running, paused, completed, failed |
| `result` | object | Final output from the last executed node |
| `execution_time_ms` | int | Total execution time in milliseconds |
| `nodes_executed` | int | Number of nodes that were executed |
| `history` | array | Execution history of each node (timing, errors) |
| `checkpoints` | array | Available checkpoint IDs for recovery |

## Checkpointing and Recovery

When `enable_checkpoints` is true (default), the engine saves state snapshots at regular
intervals. If a workflow fails midway, you can resume from the last checkpoint:

```bash
curl -X POST http://localhost:7061/v1/agentic/workflows/wf-a1b2c3d4/restore \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "checkpoint_id": "cp-xyz789"
  }'
```

Checkpoints store the current node, all workflow variables, and the message history.
They do not store node handler functions -- those are resolved from the workflow definition.

## Troubleshooting

### Workflow times out

- Check the `timeout` in your config. Default is 30 minutes.
- For LLM-heavy workflows, increase to 60 minutes or more.
- Check if any node is in an infinite loop (iterative patterns with no termination
  condition). The `max_iterations` safeguard (default 100) prevents truly infinite loops.

### Node execution fails with retry exhaustion

- Review the node's `retry_policy`. Default is 3 retries with 1-second delay.
- For unreliable providers, increase retries and use exponential backoff:
  `{"max_retries": 5, "delay": "2s", "backoff": 2.0}`
- Check the provider health at `/v1/health` -- the provider may be down.

### Workflow stuck in "running" status

- The workflow may be waiting on a `human` node for approval.
- Check the workflow history to see which node is currently active.
- If the workflow is genuinely stuck, cancel it and restore from a checkpoint.

### Conditional routing goes to wrong branch

- Verify that your condition node's `condition_field` matches the variable name set by the
  previous node in `state.Variables`.
- Edge conditions are evaluated in order -- the first matching edge is taken. Place more
  specific conditions before general fallbacks.

### Out-of-memory on large workflows

- Large workflows with many nodes and message accumulation can consume significant memory.
- Use `enable_checkpoints` with a small interval to allow garbage collection of old state.
- Limit the number of messages kept in state using workflow middleware.

## Best Practices

1. **Keep workflows under 10 nodes** for maintainability and debuggability.
2. **Use conditional nodes for error handling** -- route to recovery branches instead of
   letting the entire workflow fail.
3. **Set appropriate timeouts per node** -- a code generation node needs more time than a
   classification node.
4. **Monitor long-running workflows** via `GET /v1/agentic/workflows/:id`.
5. **Use checkpoints for critical workflows** -- enables recovery without re-executing
   completed nodes.
6. **Prefer explicit end nodes** over relying on `ShouldEnd` -- makes the graph easier to
   understand and debug.
7. **Test with small inputs first** before running large-scale workflows to verify routing
   logic.

## Related Documentation

- [Planning Algorithms Guide](36-planning-algorithms-guide.md) -- For task decomposition
  before building workflows
- [LLMOps Experimentation Guide](35-llmops-experimentation-guide.md) -- For A/B testing
  different workflow configurations
- [Benchmark Guide](37-benchmark-guide.md) -- For evaluating workflow output quality
