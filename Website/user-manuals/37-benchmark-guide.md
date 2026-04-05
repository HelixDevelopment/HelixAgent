# User Manual 37: LLM Benchmarking Guide

## Overview

HelixAgent's benchmarking system evaluates LLM providers against standard benchmarks and
custom test suites. Compare providers objectively with reproducible results, track quality
over time, and generate leaderboards.

The Benchmark module (`digital.vasic.benchmark`) provides a standalone runner with built-in
benchmark suites (SWE-bench, HumanEval, MMLU, GSM8K, MBPP, HellaSwag, LMSYS, MATH) plus
support for custom user-defined benchmarks. It integrates with LLMsVerifier for provider
scoring and with the AI debate service for evaluation quality.

## Core Concepts

### Benchmark Types

Nine benchmark types are supported:

| Type | Constant | Description |
|------|----------|-------------|
| SWE-bench | `swe-bench` | Real-world software engineering tasks from GitHub issues |
| HumanEval | `humaneval` | Code generation benchmark (164 Python problems) from OpenAI |
| MBPP | `mbpp` | Mostly Basic Python Programs benchmark |
| MMLU | `mmlu` | Massive Multitask Language Understanding (57 subjects) |
| GSM8K | `gsm8k` | Grade School Math 8K -- arithmetic word problems |
| MATH | `math` | Competition-level mathematics problems |
| HellaSwag | `hellaswag` | Commonsense natural language inference |
| LMSYS | `lmsys` | LMSYS Chatbot Arena style evaluation |
| Custom | `custom` | User-defined benchmark suites |

### Benchmark Tasks

Each benchmark contains individual tasks. A task includes:

- **Prompt** -- The input presented to the LLM
- **Expected output** -- The correct answer or solution (for automated scoring)
- **Test cases** -- For code benchmarks, input/output pairs to validate generated code
- **Difficulty** -- `easy`, `medium`, or `hard`
- **Tags** -- Categories for filtering and reporting (e.g., "algorithms", "string", "math")
- **Time limit** -- Per-task timeout override

### Benchmark Runs

A run represents a complete execution of a benchmark suite against a specific provider/model
combination. Runs track:

- **Status:** `pending` -> `running` -> `completed` | `failed` | `cancelled`
- **Results** -- Per-task pass/fail, score, latency, tokens used
- **Summary** -- Aggregate statistics: pass rate, average score, average latency, breakdowns
  by difficulty and tag

### Evaluation Methods

The system uses three evaluation approaches in order of preference:

1. **AI Debate Evaluation** -- When enabled, uses the multi-LLM debate service to evaluate
   response quality. Most accurate but slowest. Configured via `use_debate_for_eval`.

2. **Code Execution** -- For code benchmarks (HumanEval, SWE-bench), runs test cases against
   the generated code. A task passes if 50%+ of test cases pass.

3. **String Matching** -- For knowledge benchmarks (MMLU, GSM8K), compares the LLM response
   against the expected output. Handles answer extraction (looks for "answer:" patterns) and
   multiple-choice letter matching.

### Leaderboard

The system generates provider leaderboards by aggregating the best run per provider for a
given benchmark type. Each entry includes pass rate, average score, latency, and optionally
the LLMsVerifier score for that provider.

## Architecture

```
+---------------------------+
|     BenchmarkSystem       |  Main orchestrator
+---------------------------+
    |           |           |
+--------+ +--------+ +--------+
|Benchmark| |Debate  | |Verifier|
| Runner  | |Adapter | |Adapter |
+--------+ +--------+ +--------+
    |           |           |
+--------+ +--------+ +--------+
|Provider| |Debate  | |Verifier|
|Adapter | |Service | |Service |
+--------+ +--------+ +--------+
```

The `BenchmarkSystem` orchestrator:

- Initializes with a provider adapter for LLM completions
- Optionally connects to the debate service for evaluation quality
- Optionally connects to LLMsVerifier for provider health and scoring
- Can auto-select the best provider based on verifier scores
- Supports multi-provider comparison runs

The `StandardBenchmarkRunner` handles the actual execution:

- Maintains an in-memory registry of benchmarks and tasks
- Initializes with built-in benchmark suites (SWE-bench Lite, HumanEval, MMLU Mini, GSM8K Mini)
- Executes tasks concurrently using a configurable worker pool
- Calculates summaries with breakdowns by difficulty and tag

## Getting Started

### Prerequisites

- HelixAgent running locally on port 7061
- At least one LLM provider configured with a valid API key
- For code evaluation: a code executor service running (optional)

### Quick Start: Run a Benchmark

```bash
curl -X POST http://localhost:7061/v1/benchmark/run \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "benchmark_type": "humaneval",
    "name": "weekly-deepseek-eval",
    "provider_name": "deepseek",
    "model_name": "deepseek-coder",
    "config": {
      "max_tasks": 50,
      "timeout": "60s",
      "concurrency": 4,
      "temperature": 0.0,
      "max_tokens": 4096,
      "save_responses": true
    }
  }'
```

**Response:**

```json
{
  "id": "run-a1b2c3d4",
  "name": "weekly-deepseek-eval",
  "benchmark_type": "humaneval",
  "provider_name": "deepseek",
  "model_name": "deepseek-coder",
  "status": "running",
  "config": {
    "max_tasks": 50,
    "timeout": 60000000000,
    "concurrency": 4,
    "temperature": 0.0,
    "max_tokens": 4096,
    "save_responses": true
  },
  "created_at": "2026-04-05T10:00:00Z"
}
```

### Check Results

```bash
curl http://localhost:7061/v1/benchmark/results/run-a1b2c3d4 \
  -H "Authorization: Bearer $HELIX_API_KEY"
```

**Response when completed:**

```json
{
  "id": "run-a1b2c3d4",
  "benchmark_type": "humaneval",
  "provider_name": "deepseek",
  "model_name": "deepseek-coder",
  "status": "completed",
  "summary": {
    "total_tasks": 50,
    "passed_tasks": 42,
    "failed_tasks": 7,
    "error_tasks": 1,
    "pass_rate": 0.84,
    "average_score": 0.87,
    "average_latency": 1250000000,
    "total_tokens": 45000,
    "by_difficulty": {
      "easy": {"total": 20, "passed": 19, "pass_rate": 0.95},
      "medium": {"total": 20, "passed": 16, "pass_rate": 0.80},
      "hard": {"total": 10, "passed": 7, "pass_rate": 0.70}
    },
    "by_tag": {
      "python": {"total": 50, "passed": 42, "pass_rate": 0.84},
      "list": {"total": 15, "passed": 14, "pass_rate": 0.93},
      "string": {"total": 12, "passed": 10, "pass_rate": 0.83}
    }
  },
  "started_at": "2026-04-05T10:00:00Z",
  "ended_at": "2026-04-05T10:05:30Z"
}
```

## Configuration Reference

### BenchmarkConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_tasks` | int | 0 (no limit) | Maximum number of tasks to run |
| `timeout` | duration | 5m | Per-task timeout |
| `concurrency` | int | 4 | Number of tasks to run in parallel |
| `retries` | int | 1 | Retry failed tasks this many times |
| `temperature` | float64 | 0.0 | Model temperature (0.0 for deterministic) |
| `max_tokens` | int | 4096 | Maximum tokens in model response |
| `system_prompt` | string | "" | System prompt prepended to every task |
| `difficulties` | array | all | Filter tasks by difficulty level |
| `tags` | array | all | Filter tasks by tag |
| `save_responses` | bool | true | Store full LLM responses in results |
| `use_debate_for_eval` | bool | false | Use AI debate for response evaluation |

### BenchmarkSystemConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enable_debate_evaluation` | bool | true | Allow debate-based evaluation |
| `use_verifier_scores` | bool | true | Use LLMsVerifier for provider ranking |
| `auto_select_provider` | bool | true | Auto-select best provider for runs |
| `default_concurrency` | int | 4 | Default worker pool size |

## Complete Code Examples

### Example 1: Multi-Provider Comparison

Compare three providers on the same benchmark.

```bash
# Run benchmark for each provider
for PROVIDER in claude deepseek gemini; do
  curl -X POST http://localhost:7061/v1/benchmark/run \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $HELIX_API_KEY" \
    -d "{
      \"benchmark_type\": \"mmlu\",
      \"name\": \"mmlu-${PROVIDER}-2026-04\",
      \"provider_name\": \"${PROVIDER}\",
      \"config\": {
        \"timeout\": \"30s\",
        \"concurrency\": 2,
        \"temperature\": 0.0
      }
    }"
done
```

Then generate a leaderboard:

```bash
curl http://localhost:7061/v1/benchmark/leaderboard?benchmark_type=mmlu \
  -H "Authorization: Bearer $HELIX_API_KEY"
```

**Response:**

```json
{
  "benchmark_type": "mmlu",
  "entries": [
    {
      "rank": 1,
      "provider_name": "claude",
      "pass_rate": 0.92,
      "average_score": 0.94,
      "average_latency": 850000000,
      "total_tasks": 3,
      "verifier_score": 8.7,
      "run_id": "run-claude-001"
    },
    {
      "rank": 2,
      "provider_name": "deepseek",
      "pass_rate": 0.88,
      "average_score": 0.90,
      "average_latency": 1200000000,
      "total_tasks": 3,
      "verifier_score": 8.2,
      "run_id": "run-deepseek-001"
    },
    {
      "rank": 3,
      "provider_name": "gemini",
      "pass_rate": 0.85,
      "average_score": 0.87,
      "average_latency": 950000000,
      "total_tasks": 3,
      "verifier_score": 7.9,
      "run_id": "run-gemini-001"
    }
  ],
  "generated_at": "2026-04-05T12:00:00Z"
}
```

### Example 2: Code Generation with Test Cases

Run SWE-bench tasks that include test case validation.

```bash
curl -X POST http://localhost:7061/v1/benchmark/run \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "benchmark_type": "swe_bench",
    "name": "swe-bench-claude-weekly",
    "provider_name": "claude",
    "model_name": "claude-3-sonnet",
    "config": {
      "timeout": "120s",
      "concurrency": 2,
      "temperature": 0.0,
      "max_tokens": 8192,
      "system_prompt": "You are an expert software engineer. Fix the bug in the provided code. Return only the corrected code.",
      "save_responses": true,
      "use_debate_for_eval": true
    }
  }'
```

### Example 3: Math Benchmark with Difficulty Filtering

Run only hard-difficulty math problems.

```bash
curl -X POST http://localhost:7061/v1/benchmark/run \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "benchmark_type": "gsm8k",
    "name": "gsm8k-hard-only",
    "provider_name": "deepseek",
    "config": {
      "difficulties": ["hard"],
      "timeout": "60s",
      "temperature": 0.0,
      "system_prompt": "Solve step by step. Provide the final numerical answer on the last line."
    }
  }'
```

### Example 4: Custom Benchmark Suite

Create and run a custom benchmark for your domain.

```bash
# Register a custom benchmark with tasks
curl -X POST http://localhost:7061/v1/benchmark/custom \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "benchmark": {
      "id": "api-design-bench",
      "type": "custom",
      "name": "API Design Benchmark",
      "description": "Evaluate LLM ability to design RESTful APIs",
      "version": "1.0.0"
    },
    "tasks": [
      {
        "id": "api-001",
        "name": "Design user CRUD endpoints",
        "description": "Design RESTful endpoints for user management",
        "prompt": "Design a complete set of RESTful API endpoints for user management (CRUD). Include request/response schemas, HTTP methods, status codes, and authentication requirements.",
        "expected": "GET /users, POST /users, GET /users/:id, PUT /users/:id, DELETE /users/:id",
        "difficulty": "easy",
        "tags": ["rest", "crud", "design"]
      },
      {
        "id": "api-002",
        "name": "Design pagination strategy",
        "description": "Design pagination for large datasets",
        "prompt": "Design a pagination strategy for an API endpoint that returns millions of records. Consider cursor-based vs offset-based pagination, include rate limiting, and handle concurrent modifications.",
        "difficulty": "medium",
        "tags": ["rest", "pagination", "scaling"]
      },
      {
        "id": "api-003",
        "name": "Design event-driven API",
        "description": "Design async event-driven endpoints",
        "prompt": "Design an API for an event-driven order processing system. Include webhook registration, event delivery, retry logic, idempotency, and dead letter handling. Provide OpenAPI schema.",
        "difficulty": "hard",
        "tags": ["events", "webhooks", "async"]
      }
    ]
  }'

# Run the custom benchmark
curl -X POST http://localhost:7061/v1/benchmark/run \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "benchmark_type": "custom",
    "name": "api-design-claude-eval",
    "provider_name": "claude",
    "config": {
      "timeout": "120s",
      "use_debate_for_eval": true
    }
  }'
```

### Example 5: Compare Two Runs

```bash
curl http://localhost:7061/v1/benchmark/results/compare?run1=run-abc&run2=run-def \
  -H "Authorization: Bearer $HELIX_API_KEY"
```

**Response:**

```json
{
  "run1_id": "run-abc",
  "run2_id": "run-def",
  "pass_rate_change": 0.05,
  "score_change": 0.03,
  "latency_change": -200000000,
  "regressions": ["swe-003"],
  "improvements": ["he-001", "mmlu-002"],
  "summary": "Run 2 improved with 2 improvements and 1 regressions"
}
```

## API Reference

### Benchmark Runs

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/benchmark/run` | Create and start a benchmark run |
| GET | `/v1/benchmark/results` | List benchmark results (with filters) |
| GET | `/v1/benchmark/results/:id` | Get a specific run's results |
| DELETE | `/v1/benchmark/results/:id` | Cancel a running benchmark |
| GET | `/v1/benchmark/results/compare` | Compare two runs |

### Query Parameters for Listing Results

| Parameter | Type | Description |
|-----------|------|-------------|
| `benchmark_type` | string | Filter by benchmark type |
| `provider_name` | string | Filter by provider |
| `model_name` | string | Filter by model |
| `status` | string | Filter by status (pending, running, completed, failed, cancelled) |
| `limit` | int | Maximum number of results to return |

### Benchmark Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/benchmark/list` | List available benchmark suites |
| GET | `/v1/benchmark/list/:id` | Get benchmark details and task count |
| GET | `/v1/benchmark/list/:id/tasks` | Get tasks for a benchmark |
| POST | `/v1/benchmark/custom` | Register a custom benchmark with tasks |
| GET | `/v1/benchmark/leaderboard` | Generate provider leaderboard |

## Interpreting Results

### Key Metrics

| Metric | Description | Good Value |
|--------|-------------|------------|
| `pass_rate` | Fraction of tasks passed (0.0-1.0) | > 0.80 |
| `average_score` | Mean task score (0.0-1.0) | > 0.80 |
| `average_latency` | Mean response time (nanoseconds) | < 2s (2,000,000,000 ns) |
| `total_tokens` | Total tokens consumed across all tasks | Varies by benchmark |
| `error_tasks` | Tasks that failed due to provider errors | 0 |

### Difficulty Breakdown

The `by_difficulty` section shows performance stratified by task complexity. A typical
pattern for a strong provider:

- **Easy:** 90-100% pass rate
- **Medium:** 75-90% pass rate
- **Hard:** 60-80% pass rate

Significant drops between difficulty levels may indicate the model lacks depth in that
domain rather than having general capability issues.

### Tag Breakdown

The `by_tag` section shows performance by topic area. Use this to identify specific
strengths and weaknesses. For example, a provider might score 95% on "string" tasks but
only 65% on "dynamic-programming" tasks.

## Troubleshooting

### Benchmark run stuck in "running"

- Tasks execute concurrently with per-task timeouts. A stuck run usually means one or
  more tasks are waiting on a slow or unresponsive provider.
- Cancel the run and retry with a shorter timeout or lower concurrency.
- Check provider health at `/v1/health`.

### All tasks fail with errors

- Verify the provider API key is valid and has sufficient credits.
- Check that the model name is correct for the provider.
- Review the first few error messages in the results for patterns (rate limiting,
  authentication failures, etc.).

### Low pass rate on code benchmarks

- Use `temperature: 0.0` for deterministic, focused output.
- Increase `max_tokens` -- code solutions can be truncated if too short.
- Add a system prompt that instructs the model to return only code, no explanations.
- Enable `use_debate_for_eval` for more nuanced evaluation (string matching can be
  overly strict for code).

### Leaderboard shows stale results

- The leaderboard aggregates from completed runs. Re-run benchmarks periodically for
  fresh data.
- Provider quality changes over time as models are updated.
- Filter by recent run dates using the list endpoint.

### Custom benchmark tasks always pass

- Verify that `expected` output is set. Without it, any non-empty response scores 0.5
  and is considered a pass.
- For strict evaluation, enable `use_debate_for_eval` which performs semantic comparison.
- Add test cases for code tasks to enable execution-based validation.

## Best Practices

1. **Run benchmarks during off-peak hours** to avoid rate limits from providers.
2. **Compare at least 3 providers** for meaningful comparisons and to identify outliers.
3. **Use the same benchmark version** across runs for fair comparison.
4. **Track results over time** to detect provider quality regressions.
5. **Set `temperature: 0.0`** for reproducible benchmark results.
6. **Use `timeout` based on task complexity** -- 30s for MMLU, 120s for SWE-bench.
7. **Enable debate evaluation** for subjective tasks (architecture design, code review)
   where string matching is insufficient.
8. **Create custom benchmarks** for your specific domain to evaluate what matters to
   your use case.
9. **Use difficulty filtering** to quickly test a provider on hard tasks only, which is
   more discriminating than running the full suite.
10. **Set `concurrency` based on provider rate limits** -- start with 2 and increase
    if the provider supports higher throughput.

## Built-In Benchmark Suites

### SWE-bench Lite

Three tasks covering common software engineering scenarios:

- **swe-001:** Fix null pointer exception (easy)
- **swe-002:** Add error handling to file reader (medium)
- **swe-003:** Implement retry logic with exponential backoff (hard)

Tags: bug-fix, error-handling, implementation, go, best-practices, resilience

### HumanEval

Two tasks from the OpenAI HumanEval suite:

- **he-001:** `has_close_elements` -- Check if any two elements are closer than threshold (easy)
- **he-002:** `separate_paren_groups` -- Separate balanced parentheses groups (medium)

Tags: python, list, comparison, string, parsing

### MMLU Mini

Three multiple-choice questions:

- **mmlu-001:** Computer Science -- binary search time complexity (medium)
- **mmlu-002:** Mathematics -- polynomial derivative (medium)
- **mmlu-003:** Physics -- projectile velocity at highest point (medium)

Tags: computer-science, mathematics, physics, algorithms, calculus, mechanics

### GSM8K Mini

Two math word problems:

- **gsm8k-001:** Basic arithmetic -- duck egg sales (easy)
- **gsm8k-002:** Multi-step calculation -- wheat production (medium)

Tags: math, arithmetic, multiplication, addition, word-problem

## Related Documentation

- [LLMOps Experimentation Guide](35-llmops-experimentation-guide.md) -- For A/B testing
  providers beyond benchmark scores
- [Agentic Workflows Guide](34-agentic-workflows-guide.md) -- For building automated
  benchmark pipelines
- [Planning Algorithms Guide](36-planning-algorithms-guide.md) -- For decomposing
  complex evaluation strategies
