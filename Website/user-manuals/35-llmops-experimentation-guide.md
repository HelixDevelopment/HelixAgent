# User Manual 35: LLMOps Experimentation Guide

## Overview

HelixAgent's LLMOps system provides continuous evaluation, A/B experimentation, prompt
versioning, dataset management, and alerting for LLM operations. Run experiments to compare
providers, models, and prompt strategies with real data and statistically rigorous analysis.

The LLMOps module (`digital.vasic.llmops`) is a standalone Go library that powers the
experimentation endpoints. It integrates with LLMsVerifier for provider scoring and with
the AI debate service for evaluation quality.

## Core Concepts

### Experiments (A/B Testing)

An experiment compares two or more variants (different providers, models, or prompt versions)
by splitting traffic according to configurable weights. Each variant receives a percentage of
requests, and metrics are tracked per variant to determine a statistically significant winner.

**Experiment lifecycle:** `draft` -> `running` -> `completed` (or `paused` / `cancelled`)

Key properties of an experiment:

- **Variants** -- At least two alternatives to compare. One is marked as the control.
- **Traffic Split** -- Percentage of requests routed to each variant (must sum to 1.0).
- **Metrics** -- What to measure: quality, latency, cost, token usage, satisfaction.
- **Target Metric** -- The primary metric used to determine the winner.

Variant assignment is deterministic: the same user ID always maps to the same variant
via FNV-32a hashing, ensuring consistent user experience within an experiment.

### Continuous Evaluation

Evaluation runs test a specific prompt/model combination against a reference dataset.
The evaluator scores each sample and tracks pass rates, metric scores, and failure reasons.

Evaluations can be:

- **One-shot** -- Run once against a dataset
- **Scheduled** -- Recurring on a cron-like schedule (e.g., hourly, daily)
- **Compared** -- Two runs compared to detect regressions or improvements

### Prompt Versioning

Prompts are stored as immutable, versioned templates with semantic versioning
(major.minor.patch). Each prompt version includes:

- Content with `{{variable}}` placeholders
- Variable definitions with types, defaults, and validation rules
- Tags and metadata for organization
- Active/inactive status (only one version active per prompt name)

The registry supports rendering prompts with variable substitution and validation,
comparing versions to see diffs, and listing all versions of a prompt.

### Datasets

Evaluation datasets contain input/output sample pairs. Four dataset types are supported:

| Type | Constant | Purpose |
|------|----------|---------|
| Golden | `golden` | Curated reference set with verified correct outputs |
| Regression | `regression` | Tests for known past failures to prevent recurrence |
| Benchmark | `benchmark` | Standardized evaluation sets for cross-provider comparison |
| User | `user` | User-generated samples from production traffic |

### Alerts

The alert system monitors evaluation results and experiment outcomes. Alert types:

| Type | Trigger |
|------|---------|
| `regression` | Pass rate drops more than 5% compared to previous run |
| `threshold` | A metric breaches a configured threshold |
| `anomaly` | Unusual pattern detected in metrics |
| `experiment` | An experiment reaches statistical significance |

Severity levels: `info`, `warning`, `critical`. Alerts can be subscribed to via callbacks.

## Architecture

```
+---------------------+
| LLMOpsSystem        |  Main orchestrator
+---------------------+
    |       |       |       |
+-------+ +-------+ +-------+ +-------+
|Prompt | |Exper. | | Eval  | |Alert  |
|Regist.| |Manager| |uator  | |Manager|
+-------+ +-------+ +-------+ +-------+
    |           |         |         |
+-------------------------------------------+
| DebateLLMEvaluator (optional)             |
| VerifierIntegration (provider scoring)    |
+-------------------------------------------+
```

The `LLMOpsSystem` is initialized once and provides access to all subsystems. It optionally
integrates with:

- **DebateLLMEvaluator** -- Uses AI debate consensus for evaluation scoring
- **VerifierIntegration** -- Uses LLMsVerifier scores for provider health and ranking

## Getting Started

### Prerequisites

- HelixAgent running locally on port 7061
- At least two LLM providers configured (for meaningful A/B tests)

### Quick Start: Run Your First Experiment

**Step 1: Create an experiment comparing two providers.**

```bash
curl -X POST http://localhost:7061/v1/llmops/experiments \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "name": "claude-vs-deepseek-coding",
    "description": "Compare Claude and DeepSeek for code generation tasks",
    "variants": [
      {
        "name": "claude-control",
        "model_name": "claude-3-sonnet",
        "is_control": true,
        "parameters": {"temperature": 0.3}
      },
      {
        "name": "deepseek-treatment",
        "model_name": "deepseek-coder",
        "is_control": false,
        "parameters": {"temperature": 0.3}
      }
    ],
    "metrics": ["quality", "latency", "cost"],
    "target_metric": "quality"
  }'
```

**Response:**

```json
{
  "id": "exp-a1b2c3d4",
  "name": "claude-vs-deepseek-coding",
  "status": "draft",
  "variants": [
    {"id": "var-001", "name": "claude-control", "is_control": true},
    {"id": "var-002", "name": "deepseek-treatment", "is_control": false}
  ],
  "traffic_split": {"var-001": 0.5, "var-002": 0.5}
}
```

**Step 2: Start the experiment.**

```bash
curl -X POST http://localhost:7061/v1/llmops/experiments/exp-a1b2c3d4/start \
  -H "Authorization: Bearer $HELIX_API_KEY"
```

**Step 3: Record metrics as requests come in.**

```bash
curl -X POST http://localhost:7061/v1/llmops/experiments/exp-a1b2c3d4/metrics \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "variant_id": "var-001",
    "metric": "quality",
    "value": 0.92
  }'
```

**Step 4: Check results.**

```bash
curl http://localhost:7061/v1/llmops/experiments/exp-a1b2c3d4/results \
  -H "Authorization: Bearer $HELIX_API_KEY"
```

**Response:**

```json
{
  "experiment_id": "exp-a1b2c3d4",
  "total_samples": 250,
  "variant_results": {
    "var-001": {
      "sample_count": 125,
      "metric_values": {
        "primary": {"value": 0.87, "std_dev": 0.12, "min": 0.45, "max": 1.0}
      }
    },
    "var-002": {
      "sample_count": 125,
      "metric_values": {
        "primary": {"value": 0.91, "std_dev": 0.09, "min": 0.55, "max": 1.0}
      },
      "improvement": 4.6
    }
  },
  "significance": 2.1,
  "confidence": 0.95,
  "winner": "var-002",
  "recommendation": "Deploy variant var-002 with 95.0% confidence"
}
```

## Configuration Reference

### LLMOpsConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enable_auto_evaluation` | bool | true | Automatically run evaluations on schedule |
| `evaluation_interval` | duration | 24h | How often to run scheduled evaluations |
| `min_samples_for_significance` | int | 100 | Minimum samples before calculating significance |
| `enable_debate_evaluation` | bool | true | Use AI debate for evaluation scoring |
| `alert_thresholds` | map | see below | Metric thresholds that trigger alerts |

**Default alert thresholds:**

```json
{
  "pass_rate": 0.85,
  "latency_p99": 5000
}
```

### Experiment Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique experiment name |
| `description` | string | No | Human-readable description |
| `variants` | array | Yes | At least 2 variants to compare |
| `traffic_split` | map | No | Variant ID to percentage (auto-equal if omitted) |
| `metrics` | array | No | Metrics to track (default: quality, latency, cost) |
| `target_metric` | string | No | Primary metric for winner determination |

### Prompt Version Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Prompt name (shared across versions) |
| `version` | string | Yes | Semantic version (e.g., "1.2.0") |
| `content` | string | Yes | Prompt template with `{{variable}}` placeholders |
| `variables` | array | No | Variable definitions with type, required, default, validation |
| `tags` | array | No | Organizational tags |
| `metadata` | object | No | Arbitrary key-value metadata |
| `author` | string | No | Author identifier |
| `is_active` | bool | No | Set as active version (first version auto-activates) |

### Evaluation Run Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Run name |
| `dataset` | string | Yes | Dataset ID to evaluate against |
| `prompt_name` | string | No | Prompt to use (from registry) |
| `prompt_version` | string | No | Specific version (or "latest") |
| `model_name` | string | No | Model to evaluate |
| `metrics` | array | No | Metrics to compute |

## Complete Code Examples

### Example 1: Prompt A/B Testing

Compare two prompt versions for a code review task.

```bash
# Create control prompt (v1.0.0)
curl -X POST http://localhost:7061/v1/llmops/prompts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "name": "code-review-prompt",
    "version": "1.0.0",
    "content": "Review the following code for bugs:\n\n{{code}}\n\nProvide a list of issues found.",
    "variables": [
      {"name": "code", "type": "string", "required": true, "description": "Code to review"}
    ],
    "tags": ["code-review", "production"],
    "metadata": {"author": "team-platform"}
  }'

# Create treatment prompt (v2.0.0) with more structured output
curl -X POST http://localhost:7061/v1/llmops/prompts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "name": "code-review-prompt-v2",
    "version": "2.0.0",
    "content": "Review the following {{language}} code for:\n1. Security vulnerabilities\n2. Performance issues\n3. Code style violations\n\nCode:\n{{code}}\n\nRespond in JSON with keys: security, performance, style (each an array of findings).",
    "variables": [
      {"name": "code", "type": "string", "required": true},
      {"name": "language", "type": "string", "required": false, "default": "Go"}
    ],
    "tags": ["code-review", "structured-output"],
    "metadata": {"author": "team-quality"}
  }'

# Create experiment comparing the two prompt versions
curl -X POST http://localhost:7061/v1/llmops/experiments \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "name": "structured-vs-freeform-review",
    "description": "Test structured JSON output vs freeform review",
    "variants": [
      {
        "name": "freeform-control",
        "prompt_name": "code-review-prompt",
        "prompt_version": "1.0.0",
        "is_control": true
      },
      {
        "name": "structured-treatment",
        "prompt_name": "code-review-prompt-v2",
        "prompt_version": "2.0.0",
        "is_control": false
      }
    ],
    "traffic_split": {"freeform-control": 0.5, "structured-treatment": 0.5},
    "metrics": ["quality", "latency", "parseability"],
    "target_metric": "quality"
  }'
```

### Example 2: Continuous Evaluation Pipeline

Set up automated quality monitoring against a golden dataset.

```bash
# Create a golden dataset
curl -X POST http://localhost:7061/v1/llmops/datasets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "name": "code-review-golden-set",
    "type": "golden",
    "description": "Curated code review samples with verified correct outputs"
  }'

# Add samples to the dataset
curl -X POST http://localhost:7061/v1/llmops/datasets/ds-abc123/samples \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "samples": [
      {
        "input": "func divide(a, b int) int { return a / b }",
        "expected_output": "Missing zero-division check: b could be 0, causing a panic.",
        "metadata": {"category": "null-safety", "difficulty": "easy"}
      },
      {
        "input": "func readFile(path string) []byte { data, _ := os.ReadFile(path); return data }",
        "expected_output": "Ignored error from os.ReadFile. Should return ([]byte, error) and propagate the error.",
        "metadata": {"category": "error-handling", "difficulty": "medium"}
      }
    ]
  }'

# Create and start an evaluation run
curl -X POST http://localhost:7061/v1/llmops/evaluate \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "name": "weekly-quality-check-2026-04",
    "dataset": "ds-abc123",
    "prompt_name": "code-review-prompt",
    "prompt_version": "1.0.0",
    "model_name": "claude-3-sonnet",
    "metrics": ["accuracy", "completeness", "latency"]
  }'
```

### Example 3: Model Comparison Experiment

Compare multiple providers for the same task.

```bash
curl -X POST http://localhost:7061/v1/llmops/experiments \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "name": "multi-model-translation",
    "description": "Compare 3 models for code translation accuracy",
    "variants": [
      {
        "name": "claude",
        "model_name": "claude-3-sonnet",
        "is_control": true,
        "parameters": {"temperature": 0.2, "max_tokens": 2048}
      },
      {
        "name": "deepseek",
        "model_name": "deepseek-coder",
        "is_control": false,
        "parameters": {"temperature": 0.2, "max_tokens": 2048}
      },
      {
        "name": "gemini",
        "model_name": "gemini-pro",
        "is_control": false,
        "parameters": {"temperature": 0.2, "max_tokens": 2048}
      }
    ],
    "metrics": ["accuracy", "latency", "cost", "token_usage"],
    "target_metric": "accuracy"
  }'
```

### Example 4: Prompt Versioning and Rendering

```bash
# Render a prompt with variables
curl -X POST http://localhost:7061/v1/llmops/prompts/code-review-prompt-v2/2.0.0/render \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $HELIX_API_KEY" \
  -d '{
    "variables": {
      "code": "func main() { http.ListenAndServe(\":8080\", nil) }",
      "language": "Go"
    }
  }'

# List all versions of a prompt
curl http://localhost:7061/v1/llmops/prompts?name=code-review-prompt \
  -H "Authorization: Bearer $HELIX_API_KEY"

# Activate a specific version
curl -X POST http://localhost:7061/v1/llmops/prompts/code-review-prompt/2.0.0/activate \
  -H "Authorization: Bearer $HELIX_API_KEY"

# Compare two prompt versions
curl http://localhost:7061/v1/llmops/prompts/code-review-prompt/diff?v1=1.0.0&v2=2.0.0 \
  -H "Authorization: Bearer $HELIX_API_KEY"
```

### Example 5: Regression Comparison

Compare two evaluation runs to detect quality regressions.

```bash
curl http://localhost:7061/v1/llmops/evaluate/compare?run1=run-abc&run2=run-def \
  -H "Authorization: Bearer $HELIX_API_KEY"
```

**Response:**

```json
{
  "run1_id": "run-abc",
  "run2_id": "run-def",
  "pass_rate_change": -0.03,
  "metric_changes": {
    "accuracy": -2.5,
    "latency": 15.0
  },
  "regressions": ["accuracy"],
  "improvements": ["latency"],
  "summary": "Regressions in 1 metrics"
}
```

## API Reference

### Experiments

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/llmops/experiments` | Create a new experiment |
| GET | `/v1/llmops/experiments` | List experiments (filter by `?status=running`) |
| GET | `/v1/llmops/experiments/:id` | Get experiment details |
| POST | `/v1/llmops/experiments/:id/start` | Start an experiment |
| POST | `/v1/llmops/experiments/:id/pause` | Pause a running experiment |
| POST | `/v1/llmops/experiments/:id/complete` | Complete with winner |
| POST | `/v1/llmops/experiments/:id/cancel` | Cancel an experiment |
| POST | `/v1/llmops/experiments/:id/assign` | Assign variant for a user |
| POST | `/v1/llmops/experiments/:id/metrics` | Record a metric value |
| GET | `/v1/llmops/experiments/:id/results` | Get experiment results |

### Evaluation

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/llmops/evaluate` | Create and start an evaluation run |
| GET | `/v1/llmops/evaluate/:id` | Get evaluation run status |
| GET | `/v1/llmops/evaluate` | List evaluation runs |
| GET | `/v1/llmops/evaluate/compare` | Compare two runs |
| POST | `/v1/llmops/evaluate/schedule` | Schedule recurring evaluation |

### Prompts

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/llmops/prompts` | Create a new prompt version |
| GET | `/v1/llmops/prompts` | List all prompts |
| GET | `/v1/llmops/prompts/:name` | List versions of a prompt |
| GET | `/v1/llmops/prompts/:name/:version` | Get a specific version |
| GET | `/v1/llmops/prompts/:name/latest` | Get the active version |
| POST | `/v1/llmops/prompts/:name/:version/activate` | Set as active |
| DELETE | `/v1/llmops/prompts/:name/:version` | Delete a version |
| POST | `/v1/llmops/prompts/:name/:version/render` | Render with variables |

### Datasets

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/llmops/datasets` | Create a dataset |
| GET | `/v1/llmops/datasets/:id` | Get dataset details |
| GET | `/v1/llmops/datasets` | List datasets |
| POST | `/v1/llmops/datasets/:id/samples` | Add samples |
| GET | `/v1/llmops/datasets/:id/samples` | Get samples (paginated) |

## Statistical Significance

The experiment manager uses a z-test approximation to determine statistical significance:

- **Minimum sample size:** 30 per variant (below this, confidence is capped at 0.5)
- **Confidence levels:**
  - z >= 2.576: 99% confidence
  - z >= 1.96: 95% confidence
  - z >= 1.645: 90% confidence
- **Winner determination:** Only declared when confidence >= 95%
- **Recommendation:** If confidence < 95%, the system recommends continuing the experiment

For production use, consider extending with proper t-tests or chi-squared tests for
non-normal distributions.

## Troubleshooting

### Experiment shows "insufficient confidence"

- Ensure at least 30 samples per variant. The z-test requires sufficient data.
- If variants are very similar, you may need 500+ samples to detect small differences.
- Check that metrics are being recorded correctly (not all zeros or all identical).

### Evaluation run stuck in "running"

- Evaluation runs execute asynchronously. Check if the LLM provider is responding.
- If the provider is down, the run will eventually fail with error details.
- Check alerts for regression or threshold breach notifications.

### Prompt rendering fails with "missing required variable"

- Verify that all variables marked `required: true` in the prompt definition are provided
  in the render request.
- Variables with `default` values do not need to be provided.
- Check variable names -- they must match the `{{variable_name}}` placeholders exactly.

### Cannot delete active prompt version

- The active version cannot be deleted. First activate a different version, then delete.
- If it is the only version, you cannot delete it (at least one must exist).

### Traffic split validation fails

- Traffic split percentages must sum to exactly 1.0 (with 0.01 tolerance).
- Every variant must have a split entry. If omitted, the system auto-distributes equally.
- No negative values are allowed.

## Best Practices

1. **Start with 50/50 splits** and adjust based on early results. For risky changes,
   use 90/10 (control/treatment) to limit exposure.
2. **Run evaluations for at least 100 samples** before making decisions.
3. **Version prompts semantically** -- major for breaking changes, minor for additions,
   patch for wording tweaks.
4. **Use golden datasets** that represent real production traffic patterns.
5. **Set up regression alerts** to catch quality drops automatically.
6. **Compare against control** -- always have one variant marked as `is_control: true`.
7. **Document experiments** with meaningful names and descriptions so the team can
   understand what was tested and why.

## Related Documentation

- [Benchmark Guide](37-benchmark-guide.md) -- For standardized provider evaluation
- [Agentic Workflows Guide](34-agentic-workflows-guide.md) -- For orchestrating
  multi-step evaluation pipelines
- [Planning Algorithms Guide](36-planning-algorithms-guide.md) -- For decomposing
  complex evaluation strategies
