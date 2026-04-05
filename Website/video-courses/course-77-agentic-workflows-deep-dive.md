# Video Course 77: Agentic Workflows Deep Dive

## Course Overview

**Duration:** 3 hours
**Level:** Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 66 (Agentic Workflows), Course 76 (Agentic Ensemble)

Go beyond the basics of agentic workflows. This course provides an in-depth look at
graph-based workflow construction, advanced node types, complex conditional branching,
distributed state management, and production-grade error recovery strategies. You will
build a complete multi-step AI pipeline from scratch and learn to operate it reliably
in production.

---

## Learning Objectives

By the end of this course, you will be able to:

1. Design complex directed acyclic graphs (DAGs) and cyclic retry loops for AI workflows
2. Select and configure all six node types: Agent, Tool, Condition, Parallel, Human, Subgraph
3. Implement conditional branching with LLM-evaluated predicates
4. Manage workflow state across distributed workers and persistent checkpoints
5. Build error recovery strategies including retry, fallback, and compensating transactions
6. Trace workflow execution end-to-end using structured logs and SSE events

---

## Module 1: Graph-Based Workflows (30 min)

### Video 1.1: The Workflow Graph Model (15 min)

**Topics:**
- Directed graphs vs. linear chains: when graphs are necessary
- Nodes as stateful computation units; edges as typed transitions
- Entry points, end nodes, and cyclic paths for retry loops
- The `WorkflowGraph` struct: Nodes map, Edges slice, EntryPoint string, EndNodes slice
- Difference between Reason-mode graphs and Execute-mode task graphs
- Key file: `internal/agentic/workflow.go`

**Graph Structure Example:**
```go
graph := &WorkflowGraph{
    EntryPoint: "classify",
    EndNodes:   []string{"success", "failure"},
    Nodes: map[string]*WorkflowNode{
        "classify":   {Type: NodeTypeAgent,     Handler: classifyIntent},
        "simple_qa":  {Type: NodeTypeAgent,     Handler: directAnswer},
        "research":   {Type: NodeTypeAgent,     Handler: researchTopic},
        "validate":   {Type: NodeTypeCondition, Handler: validateQuality},
        "format":     {Type: NodeTypeTool,      Handler: formatOutput},
        "success":    {Type: NodeTypeAgent,     Handler: emitSuccess},
        "failure":    {Type: NodeTypeAgent,     Handler: emitFailure},
    },
    Edges: []WorkflowEdge{
        {From: "classify",  To: "simple_qa",  Condition: "intent == simple"},
        {From: "classify",  To: "research",   Condition: "intent == complex"},
        {From: "simple_qa", To: "validate"},
        {From: "research",  To: "validate"},
        {From: "validate",  To: "format",    Condition: "quality_ok"},
        {From: "validate",  To: "failure",   Condition: "!quality_ok"},
        {From: "format",    To: "success"},
    },
}
```

### Video 1.2: Workflow Execution Engine (15 min)

**Topics:**
- The execution loop: resolve current node, invoke `NodeHandler`, update state, traverse edges
- State threading: how `WorkflowState` carries context across nodes
- Visiting policies: detecting infinite loops vs. intentional retry cycles
- Max-iteration safeguard (`MaxIterations` in `WorkflowConfig`)
- Graceful shutdown: propagating context cancellation through the graph

**Execution Loop (simplified):**
```go
current := graph.EntryPoint
for !graph.IsEndNode(current) {
    node := graph.Nodes[current]
    output, err := node.Handler(ctx, state, input)
    if err != nil {
        current = resolveErrorEdge(graph, current, err)
        continue
    }
    state.Update(current, output)
    current = resolveNextNode(graph, current, output, state)
}
```

---

## Module 2: Node Types in Depth (30 min)

### Video 2.1: Agent and Tool Nodes (15 min)

**Topics:**
- `NodeTypeAgent`: invokes an LLM provider, wraps the call with retries and circuit breakers
- Configuring per-node provider selection: override the registry's default with a named provider
- `NodeTypeTool`: calls an MCP, LSP, RAG, or formatter tool; receives structured JSON output
- Tool node timeout configuration: `ToolTimeout` field in `WorkflowNodeConfig`
- Combining agent and tool nodes: agent reads tool output from state and reasons over it

**Agent Node Config:**
```go
node := &WorkflowNode{
    Type: NodeTypeAgent,
    Config: &WorkflowNodeConfig{
        Provider:    "claude",           // override registry default
        MaxRetries:  3,
        Temperature: 0.3,
        SystemPrompt: "You are a research assistant. ...",
    },
    Handler: makeAgentHandler(providerRegistry),
}
```

### Video 2.2: Condition, Parallel, Human, and Subgraph Nodes (15 min)

**Topics:**
- `NodeTypeCondition`: evaluates a Go predicate or LLM-based classifier; routes to one of N edges
- `NodeTypeParallel`: spawns child goroutines for each branch; joins at a barrier node
- `NodeTypeHuman`: suspends workflow, emits an SSE approval event, resumes on API confirmation
- `NodeTypeSubgraph`: embeds a complete `WorkflowGraph` as a reusable component
- Nesting subgraphs: building a library of workflow primitives

**Parallel Node Pattern:**
```go
parallelNode := &WorkflowNode{
    Type: NodeTypeParallel,
    Config: &WorkflowNodeConfig{
        Branches: []string{"branch_a", "branch_b", "branch_c"},
        JoinNode:  "merge_results",
        Timeout:   60 * time.Second,
    },
}
```

---

## Module 3: Conditional Branching (30 min)

### Video 3.1: Static and Dynamic Conditions (15 min)

**Topics:**
- Static conditions: Go expressions evaluated against state fields
- Dynamic conditions: LLM classifier reads the question and state to choose a branch
- `ConditionEvaluator` interface: `Evaluate(ctx, state) (string, error)` returns an edge label
- Registering custom evaluators in the workflow engine
- Fallback routing: what happens when no condition matches

**LLM-Evaluated Condition:**
```go
type IntentClassifierCondition struct {
    provider LLMProvider
    labels   []string // e.g. ["simple", "complex", "clarify"]
}

func (c *IntentClassifierCondition) Evaluate(
    ctx context.Context,
    state *WorkflowState,
) (string, error) {
    prompt := buildClassificationPrompt(state.GetInput(), c.labels)
    resp, err := c.provider.Complete(ctx, prompt)
    if err != nil {
        return "simple", nil // safe default
    }
    return extractLabel(resp.Content, c.labels), nil
}
```

### Video 3.2: Multi-Criteria Routing (15 min)

**Topics:**
- Combining multiple state fields in a single routing decision
- Priority ordering: first-matching-condition wins
- Chaining conditions: A/B splits feeding into further C/D splits
- Debugging branching decisions: structured log fields `node`, `condition`, `selected_edge`
- Testing conditional routes without invoking live LLMs (mock evaluators)

**Multi-Criteria Example:**
```go
edges := []WorkflowEdge{
    // Priority 1: user is premium and query is complex
    {From: "classify", To: "premium_research",
     Condition: "tier == premium && complexity == high"},
    // Priority 2: any complex query
    {From: "classify", To: "research",
     Condition: "complexity == high"},
    // Default fallback
    {From: "classify", To: "simple_qa",
     Condition: ""},
}
```

---

## Module 4: State Management (30 min)

### Video 4.1: WorkflowState Design (15 min)

**Topics:**
- `WorkflowState` as a thread-safe key-value store threaded through the graph
- Typed accessors: `GetString`, `GetInt`, `GetJSON` to avoid runtime panics
- Scoping: global state vs. per-node scratch space
- Serialization: JSON marshaling for checkpoint persistence
- State size limits: preventing runaway growth in long workflows

**WorkflowState Usage:**
```go
// Producer node: store research results
state.Set("research_results", researchOutput)
state.Set("confidence",       0.87)

// Consumer node: read results
results, ok := state.GetString("research_results")
confidence,  _ := state.GetFloat("confidence")
if confidence < 0.7 {
    // route to human review
}
```

### Video 4.2: Checkpointing and Recovery (15 min)

**Topics:**
- Checkpoint triggers: after every node, after every N nodes, or on explicit markers
- Checkpoint storage: PostgreSQL via `workflow_checkpoints` table (key, state JSON, ts)
- Resuming a workflow from the last checkpoint on process restart
- Idempotency: ensuring node handlers produce the same output when replayed
- Cleanup: expiring checkpoints after workflow completion

**Checkpoint Config:**
```go
config := &WorkflowConfig{
    CheckpointAfterEvery: 1,            // checkpoint after each node
    CheckpointStore:      dbStore,      // PostgreSQL checkpoint store
    MaxDuration:          30 * time.Minute,
}
```

---

## Module 5: Error Recovery (30 min)

### Video 5.1: Retry and Fallback Strategies (15 min)

**Topics:**
- Per-node retry configuration: `MaxRetries`, `RetryDelay`, `BackoffMultiplier`
- Provider-level fallback: if Claude fails, retry with Gemini
- Workflow-level fallback: route to a simplified path on repeated node failure
- Distinguishing retryable errors (transient) from terminal errors (auth, validation)
- Circuit breaker integration: fail-fast when a provider is known to be down

**Retry Config:**
```go
node.Config.RetryPolicy = &RetryPolicy{
    MaxRetries:        3,
    InitialDelay:      500 * time.Millisecond,
    BackoffMultiplier: 2.0,
    RetryableErrors:   []string{"rate_limit", "timeout", "server_error"},
    FallbackProvider:  "gemini",
}
```

### Video 5.2: Compensating Transactions (15 min)

**Topics:**
- The Saga pattern applied to agentic workflows: forward steps + compensating undo steps
- Registering compensating handlers: `node.CompensateHandler`
- The compensation runner: triggered on workflow failure, executes undo steps in reverse
- Idempotent undo: ensuring compensating steps are safe to re-run
- Observing compensation via audit log entries in the `workflow_audit` table

**Saga Registration:**
```go
// Register forward and compensating handlers together
workflow.RegisterSagaStep(
    "write_to_db",
    writeHandler,
    deleteHandler, // compensating: undo the write on failure
)
workflow.RegisterSagaStep(
    "send_email",
    sendHandler,
    noopHandler, // email cannot be unsent; log and continue
)
```

---

## Module 6: Production Operations (30 min)

### Video 6.1: Monitoring and Tracing (15 min)

**Topics:**
- SSE event stream: `workflow.started`, `node.entered`, `node.exited`, `workflow.completed`
- Prometheus metrics: workflow duration histograms, node error rates, state size gauges
- OpenTelemetry spans: one span per node, parent span for the full workflow
- Log correlation: `workflow_id` and `node_id` fields in every structured log entry
- Grafana dashboard: workflow throughput, p95 latency, retry rates per node

**SSE Event Example:**
```json
{
  "event": "node.exited",
  "workflow_id": "wf-abc123",
  "node_id": "research",
  "duration_ms": 1842,
  "output_keys": ["research_results", "confidence"],
  "next_node": "validate"
}
```

### Video 6.2: Scaling and Debugging (15 min)

**Topics:**
- Horizontal scaling: multiple HelixAgent instances sharing PostgreSQL checkpoints
- Workflow routing: sticky sessions via `workflow_id` hash to reduce checkpoint contention
- Debugging stuck workflows: querying `workflow_checkpoints` for stale entries
- Force-resuming a workflow via the admin API: `POST /v1/agentic/workflows/{id}/resume`
- Challenge validation: `challenges/scripts/agentic_ensemble_challenge.sh`

**Admin Resume:**
```bash
curl -X POST http://localhost:7061/v1/agentic/workflows/wf-abc123/resume \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"from_checkpoint": true}'
```

---

## Code Demo: Building a Research Pipeline

This demo builds a five-node research pipeline end-to-end:

1. **Classify** (Condition node): route to `simple_qa` or `research` based on query complexity
2. **Research** (Agent node, Claude): retrieve background information using RAG tool
3. **Validate** (Condition node): check confidence score; retry research if below 0.7
4. **Format** (Tool node, formatter): apply Markdown formatting to the research output
5. **Respond** (Agent node, Gemini): synthesize a final user-facing answer

**Key takeaways from the demo:**
- Use `NodeTypeCondition` with an LLM evaluator for complex routing decisions
- Persist intermediate results in `WorkflowState` so downstream nodes do not re-compute
- Apply per-node `RetryPolicy` with provider fallback for fault tolerance
- Enable checkpointing every node for safe resumption after crashes

---

## Key Takeaways

- Graph-based workflows model real-world AI tasks far better than linear chains because
  they support conditional routing, parallel execution, and cyclic retry paths.
- `WorkflowState` is the single source of truth threaded through every node; design it
  carefully and keep it serializable for checkpoint persistence.
- Error recovery is a first-class concern: configure retry policies, provider fallbacks,
  and compensating transactions before deploying to production.
- Observability is built in: SSE events, Prometheus metrics, and OpenTelemetry spans
  provide full visibility into every workflow execution.

---

## Related Courses

- **Course 66: Agentic Workflows** — Foundation course for graph-based workflow concepts
- **Course 76: Agentic Ensemble** — Dual-mode execution that drives the workflow engine
- **Course 78: LLMOps Experimentation** — Run A/B experiments across workflow variants
- **Course 79: Planning Algorithms Masterclass** — HiPlan and MCTS for automated planning
