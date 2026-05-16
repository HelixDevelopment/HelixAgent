# Lab 10: LLMOps — Continuous Evaluation and A/B Experiments

## Lab Overview

**Duration**: 30 minutes
**Difficulty**: Advanced
**Module**: S7.1.2 — LLMOps Module

## Objectives

By completing this lab, you will:
- Build a golden evaluation dataset for a code generation task
- Run continuous evaluation with InMemoryContinuousEvaluator
- Create an A/B experiment between two model configurations
- Interpret AggregateMetrics and statistical significance
- Set up a prompt versioning and rollback workflow

## Prerequisites

- Module S7.1.1 (Agentic Lab) completed
- Two LLM providers configured (e.g., DeepSeek + Claude or DeepSeek + Gemini)
- `LLMOps/` module available in the project

---

## Exercise 1: Build a Golden Dataset (5 minutes)

### Task 1.1: Define Evaluation Examples

A golden dataset contains examples with expected outputs. For code generation,
"expected" does not need to be exact — it is used to compute similarity scores.

```go
package llmops_lab_test

import (
    "context"
    "fmt"
    "testing"

    "digital.vasic.llmops/llmops"
)

var codeGenGoldenDataset = &llmops.Dataset{
    ID:   "code-gen-golden-v1",
    Name: "Go Code Generation Golden Dataset",
    Type: llmops.DatasetTypeGolden,
    Examples: []llmops.Example{
        {
            ID:       "go-http-handler",
            Input:    "Write a Go HTTP handler for GET /health that returns {status:ok}",
            Expected: `func healthHandler(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) }`,
        },
        {
            ID:       "go-error-wrap",
            Input:    "Write a Go function that wraps an error with context",
            Expected: `func wrapError(msg string, err error) error { return fmt.Errorf("%s: %w", msg, err) }`,
        },
        {
            ID:       "go-channel-timeout",
            Input:    "Write a Go function that reads from a channel with a 5-second timeout",
            Expected: `select { case v := <-ch: return v, nil; case <-time.After(5*time.Second): return zero, errors.New("timeout") }`,
        },
    },
}
```

### Task 1.2: Verify Dataset Structure

```bash
# Build LLMOps module
cd /run/media/milosvasic/DATA4TB/Projects/helix_agent/LLMOps
go build ./...
go test ./... -short -count=1
```

**Dataset inspection:**

| Field | Your Dataset |
|-------|-------------|
| Dataset ID | `code-gen-golden-v1` |
| Type | `golden` |
| Number of examples | 3 |
| Example IDs | go-http-handler, go-error-wrap, go-channel-timeout |

---

## Exercise 2: Run Continuous Evaluation (10 minutes)

### Task 2.1: Create and Run the Evaluator

```go
func TestContinuousEvaluation(t *testing.T) {
    ctx := context.Background()

    // Create a simple LLM client adapter (uses HelixAgent)
    // In production: use a real LLMClient implementation
    // For this lab: use a mock that returns plausible Go code
    llmClient := newMockLLMClient()

    evaluator := llmops.NewInMemoryContinuousEvaluator(llmClient)
    evaluator.RegisterDataset(codeGenGoldenDataset)

    // Model config for provider 1 (e.g., DeepSeek)
    modelCfg := llmops.ModelConfig{
        Provider: "deepseek",
        Model:    "deepseek-coder",
    }

    run, err := evaluator.Evaluate(ctx, "code-gen-golden-v1", modelCfg)
    if err != nil {
        t.Fatalf("evaluation failed: %v", err)
    }

    t.Logf("=== Evaluation Results ===")
    t.Logf("Mean score:   %.3f", run.AggMetrics.MeanScore)
    t.Logf("Median score: %.3f", run.AggMetrics.MedianScore)
    t.Logf("Pass rate:    %.1f%%", run.AggMetrics.PassRate*100)
    t.Logf("Total tokens: %d", run.AggMetrics.TotalTokens)
    t.Logf("Duration:     %s", run.AggMetrics.Duration)

    for _, result := range run.Results {
        t.Logf("  [%s] score=%.3f", result.ExampleID, result.Score)
    }
}
```

### Task 2.2: Record Evaluation Results

| Metric | Value |
|--------|-------|
| Mean score | |
| Median score | |
| Pass rate | |
| Total tokens | |
| Lowest-scoring example | |
| Highest-scoring example | |

---

## Exercise 3: Create an A/B Experiment (10 minutes)

### Task 3.1: Run Both Providers Against the Same Dataset

```go
func TestABExperiment(t *testing.T) {
    ctx := context.Background()

    mgr := llmops.NewInMemoryExperimentManager()

    exp, err := mgr.CreateExperiment(ctx, &llmops.Experiment{
        Name: "deepseek-vs-claude-code-gen",
        ControlConfig: llmops.ModelConfig{
            Provider: "deepseek",
            Model:    "deepseek-coder",
        },
        TreatmentConfig: llmops.ModelConfig{
            Provider: "claude",   // change to available provider
            Model:    "claude-3.5-sonnet",
        },
        TrafficSplit: 0.5,
    })
    if err != nil {
        t.Fatalf("create experiment: %v", err)
    }

    t.Logf("Experiment ID: %s", exp.ID)

    // Simulate recording results from live traffic
    // (In production: record from real requests)
    controlResults   := generateMockResults(exp.ControlConfig, 50, 0.71)
    treatmentResults := generateMockResults(exp.TreatmentConfig, 50, 0.84)

    for _, r := range controlResults {
        mgr.RecordResult(ctx, exp.ID, r)
    }
    for _, r := range treatmentResults {
        mgr.RecordResult(ctx, exp.ID, r)
    }

    // Get summary
    summary, err := mgr.GetSummary(ctx, exp.ID)
    if err != nil {
        t.Fatalf("get summary: %v", err)
    }

    t.Logf("=== A/B Experiment Summary ===")
    t.Logf("Control   (%s): mean=%.3f", exp.ControlConfig.Model,
        summary.ControlMetrics.MeanScore)
    t.Logf("Treatment (%s): mean=%.3f", exp.TreatmentConfig.Model,
        summary.TreatmentMetrics.MeanScore)
    t.Logf("Winner: %s", summary.Winner)
    t.Logf("P-value: %.4f", summary.PValue)
    t.Logf("Significant: %v (p < 0.05)", summary.PValue < 0.05)
}
```

### Task 3.2: Interpret Results

| Metric | Control | Treatment |
|--------|---------|-----------|
| Provider/Model | | |
| Mean score | | |
| Win rate | | |
| P-value | | |
| Statistically significant? | | |
| Winner | | |

---

## Exercise 4: Prompt Versioning (5 minutes)

### Task 4.1: Version and Compare Prompts

```go
func TestPromptVersioning(t *testing.T) {
    ctx := context.Background()
    mgr := llmops.NewInMemoryExperimentManager()

    // Register two versions of a code generation prompt
    mgr.RegisterPromptVersion("code-gen-prompt", "v1.0",
        "Write Go code for: {{.Input}}")

    mgr.RegisterPromptVersion("code-gen-prompt", "v2.0",
        "You are an expert Go developer. Write clean, idiomatic Go code for: "+
        "{{.Input}}. Include error handling and follow Go best practices.")

    t.Logf("Registered prompt versions: v1.0, v2.0")

    // In production: evaluate both versions against the golden dataset
    // and compare scores
    // For this lab: verify the versioning API works

    versions, err := mgr.ListPromptVersions("code-gen-prompt")
    if err != nil {
        t.Fatalf("list versions: %v", err)
    }
    t.Logf("Available versions: %v", versions)

    // Promote the better version
    err = mgr.PromotePrompt("code-gen-prompt", "v2.0")
    if err != nil {
        t.Fatalf("promote prompt: %v", err)
    }
    t.Logf("Promoted v2.0 as current version")

    current, err := mgr.GetCurrentPrompt("code-gen-prompt")
    if err != nil {
        t.Fatalf("get current: %v", err)
    }
    t.Logf("Current prompt version: %s", current.Version)
}
```

---

## Lab Completion Checklist

- [ ] Built a golden dataset with at least 3 examples
- [ ] Ran evaluation and recorded AggregateMetrics
- [ ] Created A/B experiment comparing two providers
- [ ] Interpreted p-value and identified winner
- [ ] Registered and promoted a prompt version

**Key Takeaways:**
- Mean score from provider 1: ____________
- Mean score from provider 2: ____________
- Experiment winner: ____________
- P-value: ____________

---

## Troubleshooting

### "evaluator: no dataset found"
Call `evaluator.RegisterDataset(dataset)` before calling `evaluator.Evaluate()`.

### "p-value = 1.0 always"
Need sufficient sample size (at least 30 examples per variant). Increase `MaxExamples`.

### Import errors
Ensure `go.mod` has: `replace digital.vasic.llmops => ./LLMOps`

---

*Lab Version: 1.0.0*
*Last Updated: February 2026*
