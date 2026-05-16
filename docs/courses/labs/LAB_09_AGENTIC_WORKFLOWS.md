# Lab 9: Agentic Workflow Orchestration

## Lab Overview

**Duration**: 30 minutes
**Difficulty**: Advanced
**Module**: S7.1.1 — Agentic Module

## Objectives

By completing this lab, you will:
- Build a graph-based workflow with at least 3 nodes
- Implement dynamic routing using NodeOutput.NextNode
- Use WorkflowState to pass data between nodes
- Test error recovery via error-handling nodes
- Run the workflow via the HelixAgent agentic adapter

## Prerequisites

- Modules 1-3 completed (HelixAgent running locally)
- Go 1.24+ development environment
- At least one LLM provider configured
- `Agentic/` module available in the project

---

## Exercise 1: Understanding the Agentic Module (5 minutes)

### Task 1.1: Verify Module Setup

```bash
# Build the Agentic module
cd /run/media/milosvasic/DATA4TB/Projects/helix_agent/Agentic
go build ./...
go test ./... -short -count=1

# Verify the HelixAgent adapter compiles
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
go build ./internal/adapters/...
```

### Task 1.2: Explore Key Types

Read the module source:

```bash
# View core types
grep -n "type Workflow\|type Node\|type WorkflowState\|type NodeOutput\|NodeHandler" \
  Agentic/agentic/*.go
```

**Record what you find:**

| Type | Purpose |
|------|---------|
| `Workflow` | |
| `WorkflowGraph` | |
| `Node` | |
| `WorkflowState` | |
| `NodeHandler` | |
| `NodeOutput` | |

---

## Exercise 2: Build a Two-Node Research Workflow (10 minutes)

### Task 2.1: Create the Node Handlers

Create a file at `/tmp/lab09_workflow_test.go`:

```go
package main

import (
    "context"
    "fmt"
    "strings"
    "testing"

    "digital.vasic.agentic/agentic"
)

// Node 1: summarize the topic
func summarizeHandler(ctx context.Context, state *agentic.WorkflowState,
    input interface{}) (*agentic.NodeOutput, error) {

    topic, _ := state.Get("topic").(string)
    // In production: call an LLM provider here
    // For this lab: simulate the summary
    summary := fmt.Sprintf("Summary of '%s': A complex topic with "+
        "multiple perspectives and ongoing research.", topic)

    state.Set("summary", summary)
    return &agentic.NodeOutput{
        NextNode: "classify",
        Data:     summary,
    }, nil
}

// Node 2: classify complexity
func classifyHandler(ctx context.Context, state *agentic.WorkflowState,
    input interface{}) (*agentic.NodeOutput, error) {

    summary, _ := input.(string)
    complexity := "medium"
    if strings.Count(summary, " ") > 20 {
        complexity = "high"
    }
    state.Set("complexity", complexity)
    return &agentic.NodeOutput{Done: true, Data: complexity}, nil
}

func TestTwoNodeWorkflow(t *testing.T) {
    graph := &agentic.WorkflowGraph{
        Nodes: map[string]*agentic.Node{
            "summarize": {ID: "summarize", Handler: summarizeHandler},
            "classify":  {ID: "classify",  Handler: classifyHandler},
        },
        Edges: []agentic.Edge{
            {From: "summarize", To: "classify"},
        },
    }

    wf := agentic.NewWorkflow(graph)
    state := agentic.NewWorkflowState()
    state.Set("topic", "Distributed AI Systems")

    result, err := wf.Run(ctx, "summarize", state, nil)
    if err != nil {
        t.Fatalf("workflow failed: %v", err)
    }

    t.Logf("Complexity: %s", state.Get("complexity"))
    t.Logf("Result: %v", result)
}
```

### Task 2.2: Run the Test

```bash
# Copy test file and run
cp /tmp/lab09_workflow_test.go Agentic/agentic/lab09_test.go
cd Agentic
GOMAXPROCS=2 nice -n 19 go test ./agentic/... -v -run TestTwoNodeWorkflow
```

**Expected output:**
```
=== RUN   TestTwoNodeWorkflow
    lab09_test.go:XX: Complexity: medium
    lab09_test.go:XX: Result: medium
--- PASS: TestTwoNodeWorkflow (0.00s)
```

**Record your results:**

| Metric | Value |
|--------|-------|
| Nodes executed | |
| Final state.Get("complexity") | |
| Error (if any) | |

---

## Exercise 3: Add Dynamic Routing (10 minutes)

### Task 3.1: Three-Node Workflow with Branching

Extend the workflow to route based on complexity:

```go
// Node 3a: handle high-complexity topics
func deepAnalysisHandler(ctx context.Context, state *agentic.WorkflowState,
    input interface{}) (*agentic.NodeOutput, error) {

    complexity, _ := state.Get("complexity").(string)
    summary, _    := state.Get("summary").(string)

    analysis := fmt.Sprintf("[DEEP ANALYSIS] Complexity: %s\n"+
        "Detailed breakdown: %s\n"+
        "Recommendation: requires expert review", complexity, summary)

    state.Set("final_output", analysis)
    return &agentic.NodeOutput{Done: true, Data: analysis}, nil
}

// Node 3b: handle medium/low complexity
func quickSummaryHandler(ctx context.Context, state *agentic.WorkflowState,
    input interface{}) (*agentic.NodeOutput, error) {

    summary, _ := state.Get("summary").(string)
    output := fmt.Sprintf("[QUICK SUMMARY] %s", summary)
    state.Set("final_output", output)
    return &agentic.NodeOutput{Done: true, Data: output}, nil
}

// Updated classify handler — routes dynamically
func classifyWithRoutingHandler(ctx context.Context, state *agentic.WorkflowState,
    input interface{}) (*agentic.NodeOutput, error) {

    summary, _ := input.(string)
    complexity := "medium"
    if len(strings.Fields(summary)) > 15 {
        complexity = "high"
    }
    state.Set("complexity", complexity)

    // Dynamic routing based on complexity
    if complexity == "high" {
        return &agentic.NodeOutput{NextNode: "deep_analysis", Data: complexity}, nil
    }
    return &agentic.NodeOutput{NextNode: "quick_summary", Data: complexity}, nil
}
```

### Task 3.2: Update the Graph and Run

```go
func TestThreeNodeDynamicRouting(t *testing.T) {
    graph := &agentic.WorkflowGraph{
        Nodes: map[string]*agentic.Node{
            "summarize":    {ID: "summarize",    Handler: summarizeHandler},
            "classify":     {ID: "classify",     Handler: classifyWithRoutingHandler},
            "deep_analysis":{ID: "deep_analysis",Handler: deepAnalysisHandler},
            "quick_summary":{ID: "quick_summary",Handler: quickSummaryHandler},
        },
        Edges: []agentic.Edge{
            {From: "summarize",    To: "classify"},
            {From: "classify",     To: "deep_analysis"},
            {From: "classify",     To: "quick_summary"},
        },
    }

    // Test 1: short topic → quick_summary branch
    state1 := agentic.NewWorkflowState()
    state1.Set("topic", "REST APIs")
    wf := agentic.NewWorkflow(graph)
    _, err := wf.Run(context.Background(), "summarize", state1, nil)
    if err != nil {
        t.Fatalf("workflow 1 failed: %v", err)
    }
    t.Logf("Test 1 - complexity: %s", state1.Get("complexity"))
    t.Logf("Test 1 - output: %s", state1.Get("final_output"))

    // Test 2: long topic → deep_analysis branch
    state2 := agentic.NewWorkflowState()
    state2.Set("topic", "Distributed AI Systems with Ensemble LLM Orchestration")
    _, err = wf.Run(context.Background(), "summarize", state2, nil)
    if err != nil {
        t.Fatalf("workflow 2 failed: %v", err)
    }
    t.Logf("Test 2 - complexity: %s", state2.Get("complexity"))
    t.Logf("Test 2 - output: %s", state2.Get("final_output"))
}
```

**Record branching results:**

| Topic | Complexity | Branch Taken | Final Node |
|-------|-----------|--------------|------------|
| "REST APIs" | | | |
| "Distributed AI Systems..." | | | |

---

## Exercise 4: Test via HelixAgent API (5 minutes)

### Task 4.1: Run the Challenge

```bash
# Verify the agentic adapter challenge passes
cd /run/media/milosvasic/DATA4TB/Projects/HelixAgent
./challenges/scripts/agentic_challenge.sh 2>/dev/null | tail -20
```

### Task 4.2: API Test

```bash
# Test the agentic workflow via HelixAgent (if adapter endpoint exists)
curl -s http://localhost:7061/v1/health | jq '.status'

# Check adapter registration
curl -s http://localhost:7061/v1/monitoring/status | jq '.modules.agentic'
```

---

## Lab Completion Checklist

- [ ] Built and tested two-node workflow successfully
- [ ] Implemented dynamic routing between three nodes
- [ ] Verified WorkflowState carries data between nodes
- [ ] Both routing branches execute correctly
- [ ] Tests pass with `PASS` status

**Lab Score:**
- Two-node workflow passing: ___/25 points
- Dynamic routing implemented correctly: ___/50 points
- Both routing branches verified: ___/25 points

---

## Troubleshooting

### "import not found: digital.vasic.agentic"
Add the replace directive to go.mod:
```
replace digital.vasic.agentic => ./Agentic
```
Then run `go mod tidy`.

### "nil pointer in state.Get"
Always type-assert with a zero value: `state.Get("key").(string)` panics if nil.
Use: `val, _ := state.Get("key").(string)` instead.

### Workflow terminates early
Check that only the last node sets `Done: true`. Intermediate nodes should set `NextNode`.

---

*Lab Version: 1.0.0*
*Last Updated: February 2026*
