# Module S7.1.1: Agentic — Graph-Based Workflow Orchestration

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module S7.1.1: Agentic Module
- Duration: 30 minutes
- Graph-Based Autonomous AI Workflows

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Understand graph-based workflow architecture
- Implement NodeHandler functions for each workflow step
- Use WorkflowState to thread mutable context across nodes
- Enable dynamic routing via NodeOutput
- Integrate Agentic with HelixAgent via adapters

---

## Slide 3: The Problem — Sequential vs Graph Workflows

**Why sequential code is not enough for AI agents:**

```
Sequential (brittle):
Step 1 → Step 2 → Step 3 → Done
         (no branching, no retry, no self-correction)

Graph-Based (resilient):
        ┌─────────────┐
        │    Plan     │
        └──────┬──────┘
               │
       ┌───────┴────────┐
       ▼                ▼
  ┌─────────┐    ┌──────────┐
  │ Execute │    │ Fallback │
  └────┬────┘    └────┬─────┘
       │              │
       └──────┬───────┘
              ▼
        ┌──────────┐
        │ Validate │
        └──────────┘
```

---

## Slide 4: Module Identity

**`digital.vasic.agentic`**

| Property | Value |
|----------|-------|
| Module path | `digital.vasic.agentic` |
| Go version | 1.25+ |
| Source directory | `Agentic/` |
| HelixAgent adapter | `internal/adapters/agentic/adapter.go` |
| Package | `agentic` |
| Challenge | `challenges/scripts/agentic_challenge.sh` |

---

## Slide 5: Core Types — Workflow and Graph

**The structural layer:**

```go
// WorkflowGraph defines the structure: nodes + edges
type WorkflowGraph struct {
    Nodes map[string]*Node
    Edges []Edge
}

// Edge connects two nodes (static topology)
type Edge struct {
    From      string
    To        string
    Condition string // optional: label for conditional edges
}

// Workflow is the executor: builds execution order, runs nodes
type Workflow struct {
    graph *WorkflowGraph
    // ... internal state
}

func NewWorkflow(graph *WorkflowGraph) *Workflow
```

---

## Slide 6: Core Types — Node and Handler

**The execution layer:**

```go
// Node is a single step in the workflow
type Node struct {
    ID       string
    Handler  NodeHandler
    Metadata map[string]interface{}
}

// NodeHandler is the function that runs at each step
type NodeHandler func(
    ctx   context.Context,
    state *WorkflowState,
    input interface{},
) (*NodeOutput, error)

// NodeOutput controls what happens next
type NodeOutput struct {
    NextNode string      // dynamic routing: which node runs next
    Data     interface{} // payload forwarded to the next node
    Done     bool        // true = workflow is complete
}
```

---

## Slide 7: Core Types — WorkflowState

**The state layer — threaded through every node:**

```go
// WorkflowState is a thread-safe mutable bag
type WorkflowState struct {
    data map[string]interface{}
    mu   sync.RWMutex
}

func NewWorkflowState() *WorkflowState

func (s *WorkflowState) Set(key string, value interface{})
func (s *WorkflowState) Get(key string) interface{}
func (s *WorkflowState) Delete(key string)
func (s *WorkflowState) Keys() []string

// Usage inside a NodeHandler
func myHandler(ctx context.Context, state *WorkflowState,
    input interface{}) (*NodeOutput, error) {

    state.Set("result", "computed value")
    prev := state.Get("previous_result")
    // ...
}
```

---

## Slide 8: Building a Workflow

**Three steps to define and run a workflow:**

```go
import "digital.vasic.agentic/agentic"

// Step 1: Define the graph
graph := &agentic.WorkflowGraph{
    Nodes: map[string]*agentic.Node{
        "plan":     {ID: "plan",     Handler: planHandler},
        "execute":  {ID: "execute",  Handler: executeHandler},
        "validate": {ID: "validate", Handler: validateHandler},
    },
    Edges: []agentic.Edge{
        {From: "plan",    To: "execute"},
        {From: "execute", To: "validate"},
    },
}

// Step 2: Create the workflow
wf := agentic.NewWorkflow(graph)

// Step 3: Run it
state := agentic.NewWorkflowState()
state.Set("goal", "refactor authentication module")
result, err := wf.Run(ctx, "plan", state, nil)
```

---

## Slide 9: Dynamic Routing

**NodeOutput.NextNode enables runtime branching:**

```go
func planHandler(ctx context.Context, state *agentic.WorkflowState,
    input interface{}) (*agentic.NodeOutput, error) {

    complexity, _ := state.Get("complexity").(string)

    switch complexity {
    case "high":
        // Route to the "deep_analysis" node
        return &agentic.NodeOutput{NextNode: "deep_analysis"}, nil
    case "low":
        // Route directly to "execute"
        return &agentic.NodeOutput{NextNode: "execute"}, nil
    default:
        return &agentic.NodeOutput{NextNode: "execute"}, nil
    }
}
```

Graph edges define *possible* paths; `NextNode` selects the *actual* path at runtime.

---

## Slide 10: Complete Example — Code Review Agent

**Three-node workflow: gather → review → report:**

```go
gatherHandler := func(ctx context.Context,
    state *agentic.WorkflowState, input interface{}) (*agentic.NodeOutput, error) {

    filePath, _ := state.Get("file_path").(string)
    code, err := os.ReadFile(filePath)
    if err != nil {
        return nil, fmt.Errorf("read file: %w", err)
    }
    state.Set("code", string(code))
    return &agentic.NodeOutput{NextNode: "review", Data: string(code)}, nil
}

reviewHandler := func(ctx context.Context,
    state *agentic.WorkflowState, input interface{}) (*agentic.NodeOutput, error) {

    code := input.(string)
    review, err := llmClient.Complete(ctx, "Review this Go code:\n"+code)
    if err != nil {
        return nil, err
    }
    state.Set("review", review)
    return &agentic.NodeOutput{NextNode: "report", Data: review}, nil
}
```

---

## Speaker Notes

### Slide 3 Notes
Emphasize that the graph model is NOT about complexity for its own sake. It exists because
real agent tasks have natural branching points: "did the test pass?" → yes = deploy, no = fix.
Sequential code requires nested if-else chains that become unmaintainable.

### Slide 6 Notes
The NodeHandler signature is the most important contract in the module. Spend time on it.
The `input interface{}` receives the `Data` from the previous node's `NodeOutput`.

### Slide 9 Notes
Static edges in the graph define what's *possible*. Dynamic NextNode in NodeOutput decides
what *actually happens*. This is the key insight that makes the Agentic module powerful.

### Slide 10 Notes
Walk through the complete three-node example line by line. Show how `Data` flows: gatherHandler
puts `string(code)` as Data, reviewHandler receives it as `input interface{}` and type-asserts.
