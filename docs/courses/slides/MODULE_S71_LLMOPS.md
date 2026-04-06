# Module S7.1.2: LLMOps — Evaluation, Experiments, and Prompt Versioning

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module S7.1.2: LLMOps Module
- Duration: 30 minutes
- Operating LLMs in Production

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Run continuous evaluation pipelines against LLM datasets
- Create and manage A/B experiments between model configurations
- Version prompt templates and measure quality impact
- Integrate LLMOps with HelixAgent's LLMsVerifier pipeline
- Detect quality regressions before they reach production

---

## Slide 3: The LLMOps Lifecycle

**Four pillars of LLM operations:**

```
    ┌─────────┐
    │  Deploy │
    └────┬────┘
         │
         ▼
    ┌──────────┐     Score drops?
    │ Evaluate │────────────────► Alert + Rollback
    └────┬─────┘
         │
         ▼
    ┌────────────┐
    │ Experiment │ ◄── Try new model / new prompt
    └─────┬──────┘
          │
          ▼
    ┌─────────┐
    │ Version │ ◄── Promote winner; archive loser
    └────┬────┘
         │
         └──────► back to Deploy
```

---

## Slide 4: Module Identity

**`digital.vasic.llmops`**

| Property | Value |
|----------|-------|
| Module path | `digital.vasic.llmops` |
| Go version | 1.25+ |
| Source directory | `LLMOps/` |
| HelixAgent adapter | `internal/adapters/llmops/adapter.go` |
| Package | `llmops` |
| Challenge | `challenges/scripts/llmops_challenge.sh` |

---

## Slide 5: Dataset Types

**Three categories of evaluation data:**

```go
type DatasetType string

const (
    DatasetTypeGolden     DatasetType = "golden"     // hand-curated, ground truth
    DatasetTypeSynthetic  DatasetType = "synthetic"  // generated, may have noise
    DatasetTypeProduction DatasetType = "production" // sampled from real traffic
)

type Dataset struct {
    ID       string
    Name     string
    Type     DatasetType
    Examples []Example
}

type Example struct {
    ID       string
    Input    string
    Expected string // optional: ground truth for scored evaluation
    Metadata map[string]interface{}
}
```

---

## Slide 6: Continuous Evaluation

**Running evaluation pipelines:**

```go
// Create evaluator with an LLM client
evaluator := llmops.NewInMemoryContinuousEvaluator(llmClient)

// Register golden dataset
dataset := &llmops.Dataset{
    ID:   "code-generation-v1",
    Type: llmops.DatasetTypeGolden,
    Examples: []llmops.Example{
        {
            ID:       "ex1",
            Input:    "Write a Go HTTP handler for POST /users",
            Expected: "func(w http.ResponseWriter, r *http.Request)",
        },
    },
}
evaluator.RegisterDataset(dataset)

// Run evaluation
run, err := evaluator.Evaluate(ctx, "code-generation-v1", modelCfg)
fmt.Printf("Mean score: %.3f  |  Examples: %d\n",
    run.AggMetrics.MeanScore, len(run.Results))
```

---

## Slide 7: EvaluationRun Structure

**What an evaluation run produces:**

```go
type EvaluationRun struct {
    ID          string
    DatasetID   string
    ModelConfig ModelConfig
    StartedAt   time.Time
    CompletedAt time.Time
    Results     []ExampleResult // per-example scores
    AggMetrics  AggregateMetrics
}

type AggregateMetrics struct {
    MeanScore    float64
    MedianScore  float64
    P90Score     float64
    PassRate     float64 // fraction scoring above threshold
    TotalTokens  int
    TotalCostUSD float64
    Duration     time.Duration
}
```

---

## Slide 8: A/B Experiment Management

**Comparing two model configurations:**

```go
mgr := llmops.NewInMemoryExperimentManager()

exp, err := mgr.CreateExperiment(ctx, &llmops.Experiment{
    Name: "deepseek-vs-claude-code-gen",
    ControlConfig: llmops.ModelConfig{
        Provider: "deepseek",
        Model:    "deepseek-coder",
    },
    TreatmentConfig: llmops.ModelConfig{
        Provider: "claude",
        Model:    "claude-3.5-sonnet",
    },
    TrafficSplit: 0.5, // 50% control, 50% treatment
})

// Record live traffic results
mgr.RecordResult(ctx, exp.ID, result)

// Get winner after sufficient traffic
summary, _ := mgr.GetSummary(ctx, exp.ID)
fmt.Printf("Winner: %s (p-value: %.4f)\n",
    summary.Winner, summary.PValue)
```

---

## Slide 9: Prompt Versioning

**Track prompt changes and their quality impact:**

```go
// Register prompt version
mgr.RegisterPromptVersion("code-review-prompt", "v2.1",
    "Review this Go code for: security issues, error handling, and performance. "+
    "Format: JSON with fields: issues[], severity[], suggestions[]")

// Evaluate old vs new version
runV1, _ := evaluator.Evaluate(ctx, "golden", promptV1Config)
runV2, _ := evaluator.Evaluate(ctx, "golden", promptV2Config)

// Promote if improved
if runV2.AggMetrics.MeanScore > runV1.AggMetrics.MeanScore {
    mgr.PromotePrompt("code-review-prompt", "v2.1")
} else {
    mgr.RollbackPrompt("code-review-prompt") // revert to previous
}
```

---

## Slide 10: HelixAgent Integration

**How HelixAgent uses LLMOps:**

```
LLMsVerifier Startup
       │
       ▼
InMemoryContinuousEvaluator
       │  (8-test verification = evaluation run)
       ▼
Per-provider EvaluationRun
       │
       ├── MeanScore → ResponseSpeed (25%)
       ├── PassRate  → Capability (20%)
       ├── Duration  → ModelEfficiency (20%)
       └── CostUSD   → CostEffectiveness (25%)
```

```bash
# View evaluation results via HelixAgent
curl http://localhost:7061/v1/llmops/evaluations/latest | jq
curl http://localhost:7061/v1/llmops/experiments | jq
```

---

## Speaker Notes

### Slide 3 Notes
The LLMOps lifecycle is the "CI/CD for AI models." Just as code changes go through tests
before deployment, model changes (new provider, new prompt) should go through evaluation
before hitting production traffic.

### Slide 8 Notes
A/B experiments require sufficient traffic for statistical significance. Rule of thumb:
at least 100 examples per variant before interpreting p-values. The InMemoryExperimentManager
handles the bookkeeping; you supply the traffic routing logic.

### Slide 9 Notes
Prompt versioning is often undervalued. A prompt change that looks like an improvement on
one dataset can regress on another. Always evaluate both before promoting.
