# Module S7.2.2: Benchmark — Standardized LLM Evaluation

## Presentation Slides Outline

---

## Slide 1: Title Slide

**HelixAgent: Multi-Provider AI Orchestration**

- Module S7.2.2: Benchmark Module
- Duration: 30 minutes
- Data-Driven LLM Provider Selection

---

## Slide 2: Learning Objectives

**By the end of this module, you will:**

- Run MMLU, HumanEval, GSM8K, and other standard benchmarks
- Define custom domain benchmark datasets
- Interpret BenchmarkResult metrics (score, latency, cost)
- Generate ComparisonReports with statistical significance
- Gate provider promotions using benchmark results in LLMsVerifier

---

## Slide 3: Why Benchmark?

**The gap between marketing and reality:**

| What providers claim | What you need to know |
|----------------------|-----------------------|
| "State of the art on MMLU" | How does it perform on MY data? |
| "Best coding model" | Does it pass MY unit tests? |
| "Fastest inference" | What is p95 latency on MY request sizes? |
| "Most cost-effective" | What is actual cost for MY token budget? |

**The Benchmark module provides:**
- Standardized benchmark infrastructure for apples-to-apples comparison
- Custom benchmark support for domain-specific evaluation
- Statistical significance testing to avoid false winners
- PostgreSQL persistence for trend analysis over time

---

## Slide 4: Module Identity

**`digital.vasic.benchmark`**

| Property | Value |
|----------|-------|
| Module path | `digital.vasic.benchmark` |
| Go version | 1.25+ |
| Source directory | `Benchmark/` |
| HelixAgent adapter | `internal/adapters/benchmark/adapter.go` |
| Package | `benchmark` |
| Challenge | `challenges/scripts/benchmark_challenge.sh` |

---

## Slide 5: Supported Benchmarks

**Nine benchmark suites out of the box:**

| Benchmark | Domain | Primary Metric | Examples |
|-----------|--------|----------------|----------|
| MMLU | General knowledge (57 subjects) | Accuracy | 14K |
| HumanEval | Python code generation | pass@1 | 164 |
| GSM8K | Math word problems | Exact match | 8.5K |
| SWE-Bench | GitHub issue resolution | Resolution rate | 2.3K |
| MBPP | Basic Python problems | pass@1 | 374 |
| LMSYS | Chatbot head-to-head | ELO rating | Variable |
| HellaSwag | Commonsense NLI | Accuracy | 70K |
| MATH | Competition mathematics | Accuracy | 12.5K |
| Custom | Your domain | Configurable | You define |

---

## Slide 6: BenchmarkRunner Interface

**The core execution API:**

```go
type BenchmarkRunner interface {
    // Run a single benchmark configuration
    Run(ctx context.Context,
        cfg *RunConfig) (*BenchmarkResult, error)

    // Run multiple configurations in parallel
    RunSuite(ctx context.Context,
        cfgs []*RunConfig) ([]*BenchmarkResult, error)

    // Generate a comparison report
    Compare(ctx context.Context,
        results []*BenchmarkResult) (*ComparisonReport, error)

    // Register a custom dataset
    RegisterCustomDataset(dataset *Dataset)
}

// Create a runner
runner := benchmark.NewBenchmarkRunner(benchmark.RunnerConfig{
    Providers:   []benchmark.ProviderConfig{{Name: "deepseek"}, {Name: "claude"}},
    Parallelism: 4,
    OutputDir:   "./benchmark-results",
})
```

---

## Slide 7: RunConfig and BenchmarkResult

**Input and output structures:**

```go
type RunConfig struct {
    BenchmarkID   string        // "mmlu", "humaneval", "gsm8k", "custom"
    Provider      string        // provider name
    Model         string        // model identifier
    MaxExamples   int           // limit for fast runs (0 = full dataset)
    Temperature   float64       // generation temperature
    Timeout       time.Duration // per-example timeout
    CustomDataset *Dataset      // for custom benchmarks
}

type BenchmarkResult struct {
    BenchmarkID string
    Provider    string
    Model       string
    Score       float64            // primary metric
    SubScores   map[string]float64 // per-subject/category
    Latency     LatencyStats       // P50, P95, P99 in ms
    Cost        CostStats          // total tokens, estimated USD
    RunAt       time.Time
}

type LatencyStats struct{ P50, P95, P99 time.Duration }
type CostStats   struct{ TotalTokens int; EstimatedUSD float64 }
```

---

## Slide 8: Running a Benchmark Suite

**Comparing two providers on HumanEval:**

```go
results, err := runner.RunSuite(ctx, []*benchmark.RunConfig{
    {
        BenchmarkID: "humaneval",
        Provider:    "deepseek",
        Model:       "deepseek-coder",
        MaxExamples: 50, // fast CI run
    },
    {
        BenchmarkID: "humaneval",
        Provider:    "claude",
        Model:       "claude-3.5-sonnet",
        MaxExamples: 50,
    },
})

report, err := runner.Compare(ctx, results)
for _, entry := range report.Ranking {
    fmt.Printf("%s/%s: %.1f%%  p95=%dms  $%.4f\n",
        entry.Provider,
        entry.Model,
        entry.Score*100,
        entry.Latency.P95.Milliseconds(),
        entry.Cost.EstimatedUSD,
    )
}
// deepseek/deepseek-coder: 71.0%  p95=1842ms  $0.0031
// claude/claude-3.5-sonnet: 84.0%  p95=2201ms  $0.0092
```

---

## Slide 9: Custom Benchmarks

**Defining domain-specific evaluation:**

```go
// 1. Define your domain dataset
dataset := &benchmark.Dataset{
    ID:   "my-company-sql-tasks-v1",
    Name: "SQL Generation Tasks",
    Examples: []benchmark.Example{
        {
            ID:       "sql-001",
            Input:    "Find top 10 customers by revenue in Q4 2025",
            Expected: "SELECT customer_id, SUM(revenue) ...",
        },
        // add more examples...
    },
}

// 2. Register and run
runner.RegisterCustomDataset(dataset)
result, err := runner.Run(ctx, &benchmark.RunConfig{
    BenchmarkID:   "custom",
    CustomDataset: dataset,
    Provider:      "deepseek",
    Model:         "deepseek-chat",
    MaxExamples:   0, // run all
})

fmt.Printf("Custom benchmark score: %.1f%%\n", result.Score*100)
```

---

## Slide 10: HelixAgent Integration

**Provider promotion gating with benchmarks:**

```
LLMsVerifier Startup
    │
    ▼
BenchmarkAdapter.RunVerification(ctx, provider)
    │  (8-test suite = lightweight benchmark run)
    │
    ├── Score >= 5.0 → provider eligible
    └── Score < 5.0 → provider excluded

Provider Promotion (e.g., upgrading Claude model version)
    │
    ▼
benchmark.RunSuite(old_model, new_model)
    │
    ├── new_model wins AND p-value < 0.05 → promote
    └── otherwise → keep old_model
```

```bash
# Resource-limited benchmark run (CRITICAL: always limit resources)
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test ./benchmark/... -v -run TestBenchmarkRunner

# HelixAgent benchmark API
curl -X POST http://localhost:7061/v1/benchmark/run \
  -H "Content-Type: application/json" \
  -d '{"benchmark":"humaneval","providers":["deepseek","claude"],"max_examples":50}'
```

---

## Speaker Notes

### Slide 3 Notes
The "gap between marketing and reality" framing resonates strongly. Published MMLU scores
are measured on standardized conditions that may not match production. The most important
benchmarks are the ones you design yourself for your specific use case.

### Slide 5 Notes
MMLU is great for general knowledge. HumanEval and MBPP are the gold standard for code
generation. GSM8K measures mathematical reasoning which correlates with logical capability.
SWE-Bench is the hardest — it requires understanding real codebases.

### Slide 9 Notes
Custom benchmarks are where the real value is. Spend time helping students design
their own Example sets. Key principle: examples should come from real production
failures — what questions did your system get wrong last week?

### Slide 10 Notes
CRITICAL: Always emphasize resource limits. Benchmark runs are CPU-intensive.
A full HumanEval run can take 15+ minutes and saturate all cores. Always use
GOMAXPROCS=2, nice -n 19, and MaxExamples limits in automated contexts.
