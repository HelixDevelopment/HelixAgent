# Video Course 80: Benchmarking & Provider Evaluation

## Course Overview

**Duration:** 2.5 hours
**Level:** Intermediate to Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 07 (Advanced Providers), Course 75 (Performance Tuning)

Learn how to rigorously benchmark HelixAgent's 43+ LLM providers using industry-standard
suites (SWE-bench, HumanEval, MMLU) and custom benchmarks. Understand how scores feed
into the dynamic provider selection system, how to build your own leaderboard, and how
to use benchmark results to make confident provider decisions.

---

## Learning Objectives

By the end of this course, you will be able to:

1. Run SWE-bench, HumanEval, and MMLU benchmarks against any configured provider
2. Design and run custom benchmarks for your specific use case
3. Interpret benchmark results and composite leaderboard scores
4. Understand how LLMsVerifier scores feed into dynamic provider selection
5. Compare providers across cost, latency, and quality dimensions
6. Automate nightly benchmarking and track performance over time

---

## Module 1: SWE-bench (30 min)

### Video 1.1: What Is SWE-bench? (15 min)

**Topics:**
- SWE-bench: a benchmark of 2294 real GitHub issues from popular Python projects
- Task format: given an issue description + repository context, generate a patch
- Evaluation: does the patch pass the test suite? (pass@1 metric)
- Why SWE-bench matters for code-generation providers
- HelixAgent's SWE-bench adapter: static code analysis used as a proxy for full execution
- Key file: `Benchmark/benchmark.go`

**SWE-bench Request:**
```bash
curl -X POST http://localhost:7061/v1/benchmark/run \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "benchmark": "swe-bench",
    "provider": "claude",
    "num_samples": 20,
    "timeout_per_sample": "120s"
  }'
```

**SWE-bench Response:**
```json
{
  "benchmark_id": "bench-abc123",
  "benchmark": "swe-bench",
  "provider": "claude",
  "num_samples": 20,
  "results": {
    "pass_at_1": 0.65,
    "avg_latency_ms": 4200,
    "avg_cost_usd": 0.042,
    "static_analysis_score": 0.78
  },
  "status": "completed"
}
```

### Video 1.2: Interpreting SWE-bench Results (15 min)

**Topics:**
- `pass_at_1`: fraction of issues where the first attempt passes the test suite
- `static_analysis_score`: HelixAgent's proxy metric (correctness, style, complexity)
- The five static analysis dimensions: correctness, test coverage, code style,
  complexity reduction, performance improvement
- Comparing SWE-bench scores across providers to identify best code-generation models
- Limitations: static analysis is a proxy; full execution requires the SWE-bench harness

**Static Analysis Score Breakdown:**
```
Correctness:          40% weight — does the patch logically address the issue?
Test Coverage:        20% weight — does the patch include or preserve tests?
Code Style:           15% weight — does the patch follow project conventions?
Complexity Reduction: 15% weight — does it simplify the affected code?
Performance:          10% weight — are there obvious performance improvements?
```

---

## Module 2: HumanEval (30 min)

### Video 2.1: Running HumanEval (15 min)

**Topics:**
- HumanEval: 164 hand-crafted Python programming problems from OpenAI
- Task format: given a function signature and docstring, complete the implementation
- pass@k metric: probability at least one of k attempts passes all unit tests
- HelixAgent's HumanEval adapter: sends each problem to the provider, evaluates the output
- Providers that excel at HumanEval vs. SWE-bench (different skill sets)
- Key file: `Benchmark/benchmark.go`

**HumanEval Request:**
```bash
curl -X POST http://localhost:7061/v1/benchmark/run \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "benchmark": "humaneval",
    "provider": "deepseek",
    "num_samples": 50,
    "k": 1,
    "timeout_per_sample": "30s"
  }'
```

### Video 2.2: Multi-Provider HumanEval Comparison (15 min)

**Topics:**
- Running HumanEval against multiple providers in parallel
- The comparison endpoint: `POST /v1/benchmark/compare`
- Visualising the results: pass@1 bar chart, latency vs. quality scatter plot
- Identifying the best provider for code completion tasks in your stack
- Cost-adjusted score: `pass_at_1 / cost_usd_per_sample` for budget-aware selection

**Multi-Provider Comparison:**
```bash
curl -X POST http://localhost:7061/v1/benchmark/compare \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "benchmark": "humaneval",
    "providers": ["claude", "deepseek", "gemini", "openrouter"],
    "num_samples": 30,
    "metrics": ["pass_at_1", "latency_ms", "cost_usd"]
  }'
```

**Comparison Result:**
```json
{
  "benchmark": "humaneval",
  "results": [
    {"provider": "deepseek", "pass_at_1": 0.83, "latency_ms": 980,  "cost_usd": 0.0004},
    {"provider": "claude",   "pass_at_1": 0.81, "latency_ms": 1850, "cost_usd": 0.0018},
    {"provider": "gemini",   "pass_at_1": 0.77, "latency_ms": 820,  "cost_usd": 0.0006},
    {"provider": "openrouter","pass_at_1": 0.75, "latency_ms": 1200, "cost_usd": 0.0009}
  ],
  "cost_adjusted_winner": "deepseek"
}
```

---

## Module 3: MMLU (30 min)

### Video 3.1: Running MMLU (15 min)

**Topics:**
- MMLU: Massive Multitask Language Understanding — 57 subjects, 14000+ questions
- Task format: multiple-choice questions across STEM, humanities, social sciences
- Accuracy metric: fraction of questions answered correctly
- Which providers excel at knowledge breadth vs. reasoning depth
- MMLU as a proxy for general-purpose assistant quality
- Sampling strategy: run a subset of subjects relevant to your use case

**MMLU Request:**
```bash
curl -X POST http://localhost:7061/v1/benchmark/run \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "benchmark": "mmlu",
    "provider": "claude",
    "subjects": ["computer_science", "mathematics", "physics"],
    "num_samples_per_subject": 20,
    "timeout_per_sample": "15s"
  }'
```

### Video 3.2: Subject-Level Analysis (15 min)

**Topics:**
- Breaking MMLU accuracy down by subject group: STEM vs. humanities vs. social sciences
- Identifying domain-specific strengths per provider
- Using MMLU results to route queries to the best provider per topic
- Combining MMLU with SWE-bench and HumanEval to build a comprehensive profile

**Subject-Level Result:**
```json
{
  "benchmark": "mmlu",
  "provider": "claude",
  "overall_accuracy": 0.84,
  "by_subject": {
    "computer_science": 0.91,
    "mathematics":       0.88,
    "physics":           0.85,
    "history":           0.79,
    "law":               0.76
  }
}
```

---

## Module 4: Custom Benchmarks (30 min)

### Video 4.1: Defining a Custom Benchmark (15 min)

**Topics:**
- When industry benchmarks are insufficient: domain-specific tasks
- Custom benchmark structure: problem set, evaluation function, metric definition
- The `CustomBenchmark` struct: `Name`, `Problems`, `EvalFunc`, `Metrics`
- Writing evaluation functions: exact match, fuzzy match, LLM-as-judge
- Registering a custom benchmark in the benchmark registry

**Custom Benchmark Definition:**
```go
benchmark := &CustomBenchmark{
    Name:        "helixagent-api-qa",
    Description: "Tests correct API response formatting for HelixAgent endpoints",
    Problems: []BenchmarkProblem{
        {
            ID:    "p1",
            Input: "Generate a valid /v1/chat/completions request body for model claude-sonnet-4-5",
            Reference: `{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"..."}]}`,
        },
    },
    EvalFunc: func(output, reference string) float64 {
        var got, want map[string]interface{}
        json.Unmarshal([]byte(output), &got)
        json.Unmarshal([]byte(reference), &want)
        return structuralSimilarity(got, want)
    },
    Metrics: []string{"structural_similarity", "latency_ms"},
}
```

### Video 4.2: Running and Storing Custom Benchmark Results (15 min)

**Topics:**
- Submitting a custom benchmark via the API
- Storing results in the benchmark database for historical comparison
- Nightly scheduled benchmark runs via the BackgroundTasks module
- Regression detection: alert when accuracy drops more than 5% week-over-week
- Exporting results to CSV for offline analysis

**Submit Custom Benchmark:**
```bash
curl -X POST http://localhost:7061/v1/benchmark/custom \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "helixagent-api-qa",
    "provider": "gemini",
    "problems_file": "benchmarks/api-qa-v2.jsonl"
  }'
```

---

## Module 5: Leaderboard and Provider Selection (30 min)

### Video 5.1: The Benchmark Leaderboard (15 min)

**Topics:**
- The HelixAgent leaderboard: aggregated scores across all benchmarks and metrics
- Composite score formula: weighted average of SWE-bench, HumanEval, MMLU, and custom
- Weight configuration: adjust weights to match your workload profile
- Viewing the leaderboard: `GET /v1/benchmark/results`
- Leaderboard persistence: PostgreSQL `benchmark_results` table

**Leaderboard:**
```bash
curl http://localhost:7061/v1/benchmark/results \
  -H "Authorization: Bearer $API_KEY"
```

```json
{
  "leaderboard": [
    {"rank": 1, "provider": "claude",    "composite_score": 0.872},
    {"rank": 2, "provider": "deepseek",  "composite_score": 0.841},
    {"rank": 3, "provider": "gemini",    "composite_score": 0.828},
    {"rank": 4, "provider": "openrouter","composite_score": 0.791}
  ],
  "weights": {"swe_bench": 0.3, "humaneval": 0.3, "mmlu": 0.25, "custom": 0.15}
}
```

### Video 5.2: Connecting Benchmarks to Dynamic Provider Selection (15 min)

**Topics:**
- How LLMsVerifier's startup verification uses benchmark-like scoring
- The five scoring components: ResponseSpeed, CostEffectiveness, ModelEfficiency,
  Capability, Recency — and their weights
- Feeding custom benchmark scores into the LLMsVerifier score via the scoring API
- Dynamic debate team selection: top-scoring providers form the ensemble
- Re-scoring on demand: `POST /v1/scoring/rescore` after a benchmark run updates rankings

**LLMsVerifier Score Components:**
```
ResponseSpeed (25%):      p95 latency from health check pings
CostEffectiveness (25%):  cost per 1K tokens from provider pricing
ModelEfficiency (20%):    HumanEval pass@1 proxy score
Capability (20%):         MMLU accuracy proxy
Recency (10%):            model release date recency bonus
OAuth/Free bonus (+0.5):  applied to OAuth and free-tier providers
Minimum score: 5.0        providers below this are excluded from ensemble
```

---

## Key Takeaways

- SWE-bench, HumanEval, and MMLU each measure different capabilities; use all three for
  a complete provider profile rather than relying on any single benchmark.
- Custom benchmarks are essential when your workload differs significantly from public
  benchmark distributions.
- The composite leaderboard score gives a single number for provider ranking, but always
  check individual benchmark breakdowns before making high-stakes provider decisions.
- Benchmark scores feed directly into LLMsVerifier's dynamic provider selection —
  better benchmark performance means higher ensemble participation.

---

## Related Courses

- **Course 07: Advanced Providers** — Deep dive into configuring all 43+ providers
- **Course 75: Performance Tuning** — Latency and throughput baselines that complement benchmarks
- **Course 78: LLMOps Experimentation** — A/B experiments that use benchmark metrics for evaluation
- **Course 84: Monitoring, Dashboards and Alerting** — Tracking benchmark trends over time
