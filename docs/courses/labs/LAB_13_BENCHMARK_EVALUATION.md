# Lab 13: Benchmark — Standardized LLM Provider Evaluation

## Lab Overview

**Duration**: 30 minutes
**Difficulty**: Advanced
**Module**: S7.2.2 — Benchmark Module

## Objectives

By completing this lab, you will:
- Run a standardized benchmark (HumanEval subset) against two providers
- Define a custom benchmark dataset for a Go domain task
- Interpret BenchmarkResult metrics: score, latency, and cost
- Generate a ComparisonReport and identify the winner
- Apply resource limits to prevent system overload during benchmarking

## Prerequisites

- Module S7.2.1 (Planning Lab) completed
- At least one LLM provider configured (two recommended for comparison)
- `Benchmark/` module available in the project
- Understanding of Module 14 (LLMsVerifier) recommended

---

## Exercise 1: Module Setup and Verification (5 minutes)

### Task 1.1: Build and Test the Module

```bash
cd /run/media/milosvasic/DATA4TB/Projects/helix_agent/Benchmark
go build ./...

# Run tests with MANDATORY resource limits
GOMAXPROCS=2 nice -n 19 go test ./... -short -count=1 -p 1
```

### Task 1.2: Review Available Benchmarks

```go
package benchmark_lab_test

import (
    "context"
    "fmt"
    "testing"

    "digital.vasic.benchmark/benchmark"
)

func TestListSupportedBenchmarks(t *testing.T) {
    runner := benchmark.NewBenchmarkRunner(benchmark.RunnerConfig{
        Parallelism: 2, // MANDATORY: limit parallelism
    })

    benchmarks := runner.ListSupportedBenchmarks()
    t.Log("=== Supported Benchmarks ===")
    for _, b := range benchmarks {
        t.Logf("  [%s] %s — metric: %s, size: %d examples",
            b.ID, b.Name, b.PrimaryMetric, b.TotalExamples)
    }
}
```

**Record the supported benchmarks:**

| ID | Name | Primary Metric | Total Examples |
|----|------|----------------|----------------|
| mmlu | | | |
| humaneval | | | |
| gsm8k | | | |
| custom | | | |

---

## Exercise 2: Run a HumanEval Benchmark Subset (10 minutes)

### Task 2.1: Run HumanEval on One Provider

```go
func TestHumanEvalBenchmark(t *testing.T) {
    ctx := context.Background()

    // MANDATORY resource limits
    runner := benchmark.NewBenchmarkRunner(benchmark.RunnerConfig{
        Providers: []benchmark.ProviderConfig{
            {Name: "deepseek", Model: "deepseek-coder"},
        },
        Parallelism: 2,   // max 2 concurrent requests
        OutputDir:   "/tmp/benchmark-lab-results",
    })

    // CRITICAL: use MaxExamples to limit run cost/time for lab
    cfg := &benchmark.RunConfig{
        BenchmarkID: "humaneval",
        Provider:    "deepseek",
        Model:       "deepseek-coder",
        MaxExamples: 10,        // 10 examples for the lab; full = 164
        Temperature: 0.2,       // low temperature for deterministic code
        Timeout:     30 * time.Second,
    }

    t.Log("Running HumanEval (10 examples)...")
    result, err := runner.Run(ctx, cfg)
    if err != nil {
        t.Fatalf("benchmark run failed: %v", err)
    }

    t.Logf("=== HumanEval Results ===")
    t.Logf("Provider:     %s/%s", result.Provider, result.Model)
    t.Logf("Score:        %.1f%% (pass@1)", result.Score*100)
    t.Logf("P50 latency:  %dms", result.Latency.P50.Milliseconds())
    t.Logf("P95 latency:  %dms", result.Latency.P95.Milliseconds())
    t.Logf("P99 latency:  %dms", result.Latency.P99.Milliseconds())
    t.Logf("Total tokens: %d", result.Cost.TotalTokens)
    t.Logf("Est. cost:    $%.4f", result.Cost.EstimatedUSD)
}
```

### Task 2.2: Record HumanEval Results

| Metric | Provider 1 |
|--------|-----------|
| Provider/Model | |
| Score (pass@1) | |
| P50 latency | |
| P95 latency | |
| Total tokens | |
| Estimated cost | |

---

## Exercise 3: Define a Custom Benchmark (10 minutes)

### Task 3.1: Create a Go-Specific Benchmark Dataset

```go
func TestCustomGoBenchmark(t *testing.T) {
    ctx := context.Background()

    // Define a domain-specific custom dataset
    customDataset := &benchmark.Dataset{
        ID:   "go-idiomatic-v1",
        Name: "Go Idiomatic Code Patterns",
        Examples: []benchmark.Example{
            {
                ID:    "go-001",
                Input: "Write a Go function that reads all lines from a file and returns them as a slice",
                Expected: `func readLines(path string) ([]string, error) {
    f, err := os.Open(path)
    if err != nil { return nil, fmt.Errorf("open: %w", err) }
    defer f.Close()
    var lines []string
    scanner := bufio.NewScanner(f)
    for scanner.Scan() { lines = append(lines, scanner.Text()) }
    return lines, scanner.Err()
}`,
            },
            {
                ID:    "go-002",
                Input: "Write a Go function that retries an operation up to 3 times with exponential backoff",
                Expected: `func withRetry(ctx context.Context, fn func() error) error {
    backoff := time.Second
    for i := 0; i < 3; i++ {
        if err := fn(); err == nil { return nil }
        select {
        case <-ctx.Done(): return ctx.Err()
        case <-time.After(backoff): backoff *= 2
        }
    }
    return errors.New("max retries exceeded")
}`,
            },
            {
                ID:    "go-003",
                Input: "Write a Go context-aware database query function with timeout",
                Expected: `func queryWithTimeout(db *sql.DB, query string, timeout time.Duration) (*sql.Rows, error) {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    return db.QueryContext(ctx, query)
}`,
            },
            {
                ID:    "go-004",
                Input: "Write a thread-safe Go counter using sync.Mutex",
                Expected: `type Counter struct { mu sync.Mutex; n int }
func (c *Counter) Inc() { c.mu.Lock(); defer c.mu.Unlock(); c.n++ }
func (c *Counter) Value() int { c.mu.RLock(); defer c.mu.RUnlock(); return c.n }`,
            },
            {
                ID:    "go-005",
                Input: "Write a Go function that merges two sorted slices into one sorted slice",
                Expected: `func mergeSorted(a, b []int) []int {
    result := make([]int, 0, len(a)+len(b))
    i, j := 0, 0
    for i < len(a) && j < len(b) {
        if a[i] <= b[j] { result = append(result, a[i]); i++ } else { result = append(result, b[j]); j++ }
    }
    return append(append(result, a[i:]...), b[j:]...)
}`,
            },
        },
    }

    runner := benchmark.NewBenchmarkRunner(benchmark.RunnerConfig{
        Parallelism: 2,
        OutputDir:   "/tmp/benchmark-lab-results",
    })
    runner.RegisterCustomDataset(customDataset)

    cfg := &benchmark.RunConfig{
        BenchmarkID:   "custom",
        Provider:      "deepseek",
        Model:         "deepseek-coder",
        MaxExamples:   0, // run all 5
        Temperature:   0.1,
        CustomDataset: customDataset,
    }

    result, err := runner.Run(ctx, cfg)
    if err != nil {
        t.Fatalf("custom benchmark failed: %v", err)
    }

    t.Logf("=== Custom Go Benchmark Results ===")
    t.Logf("Score:      %.1f%%", result.Score*100)
    t.Logf("Examples:   %d", len(customDataset.Examples))

    if result.SubScores != nil {
        for exID, score := range result.SubScores {
            t.Logf("  [%s] %.3f", exID, score)
        }
    }
}
```

### Task 3.2: Record Custom Benchmark Results

| Example ID | Score |
|-----------|-------|
| go-001 | |
| go-002 | |
| go-003 | |
| go-004 | |
| go-005 | |
| **Overall** | |

---

## Exercise 4: Generate a Comparison Report (5 minutes)

### Task 4.1: Compare Two Providers

```go
func TestProviderComparison(t *testing.T) {
    ctx := context.Background()

    runner := benchmark.NewBenchmarkRunner(benchmark.RunnerConfig{
        Parallelism: 2,
        OutputDir:   "/tmp/benchmark-lab-results",
    })
    runner.RegisterCustomDataset(goIdiomaticDataset)

    cfgs := []*benchmark.RunConfig{
        {
            BenchmarkID: "custom", Provider: "deepseek",
            Model: "deepseek-coder", CustomDataset: goIdiomaticDataset,
        },
        {
            BenchmarkID: "custom", Provider: "claude",
            Model: "claude-3.5-sonnet", CustomDataset: goIdiomaticDataset,
        },
    }

    results, err := runner.RunSuite(ctx, cfgs)
    if err != nil {
        t.Fatalf("suite failed: %v", err)
    }

    report, err := runner.Compare(ctx, results)
    if err != nil {
        t.Fatalf("compare failed: %v", err)
    }

    t.Log("=== Comparison Report ===")
    for i, entry := range report.Ranking {
        t.Logf("#%d %s/%s: score=%.1f%% p95=%dms cost=$%.4f",
            i+1, entry.Provider, entry.Model,
            entry.Score*100,
            entry.Latency.P95.Milliseconds(),
            entry.Cost.EstimatedUSD,
        )
    }
    t.Logf("Winner:      %s/%s", report.Winner.Provider, report.Winner.Model)
    t.Logf("P-value:     %.4f", report.PValue)
    t.Logf("Significant: %v", report.PValue < 0.05)
    t.Logf("Recommendation: %s", report.Recommendation)
}
```

### Task 4.2: Comparison Results

| Metric | Provider 1 | Provider 2 |
|--------|-----------|-----------|
| Provider/Model | | |
| Score | | |
| P95 latency | | |
| Cost | | |

- Winner: ____________
- P-value: ____________
- Statistically significant (p < 0.05)? ___
- Would you promote the winner to production? Explain: _______________

---

## Lab Completion Checklist

- [ ] Module built and tests pass with resource limits
- [ ] HumanEval (10 examples) ran successfully
- [ ] Custom Go benchmark dataset defined with 5 examples
- [ ] Custom benchmark ran and per-example scores recorded
- [ ] Comparison report generated with winner identified
- [ ] Resource limits (GOMAXPROCS=2, Parallelism=2) applied throughout

**Final Metrics:**
- HumanEval score: ____________%
- Custom benchmark score: ____________%
- Comparison winner: ____________

---

## Troubleshooting

### "benchmark: provider not configured"
Ensure the provider's API key is in `.env` and the provider is registered in HelixAgent.
For testing with a single provider, skip the comparison exercise.

### "benchmark: timeout on examples"
Increase the `Timeout` in `RunConfig` or reduce `MaxExamples` further.

### "out of memory during benchmark run"
Always use `GOMAXPROCS=2` and `Parallelism: 2`. Never run full benchmark suites locally.

### Import errors
Ensure `go.mod` has: `replace digital.vasic.benchmark => ./Benchmark`

---

*Lab Version: 1.0.0*
*Last Updated: February 2026*
