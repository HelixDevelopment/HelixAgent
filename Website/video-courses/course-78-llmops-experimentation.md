# Video Course 78: LLMOps Experimentation

## Course Overview

**Duration:** 2.5 hours
**Level:** Intermediate to Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 67 (LLMOps Experimentation overview), Course 75 (Performance Tuning)

Master LLMOps in HelixAgent: design and run A/B experiments across LLM providers and
prompts, manage versioned datasets, track prompt evolution, and evaluate outcomes with
quantitative metrics. By the end of this course you will have a repeatable experimentation
workflow that lets you confidently promote winning configurations to production.

---

## Learning Objectives

By the end of this course, you will be able to:

1. Design statistically valid A/B experiments comparing LLM providers or prompt variants
2. Create, version, and query evaluation datasets
3. Track prompt versions and link them to experiment runs
4. Define and collect evaluation metrics (BLEU, ROUGE, latency, cost)
5. Interpret experiment results and promote a winning configuration
6. Automate continuous evaluation to catch regressions in production

---

## Module 1: A/B Experiments (30 min)

### Video 1.1: Experiment Design Fundamentals (15 min)

**Topics:**
- What constitutes a valid LLMOps experiment: control vs. treatment, sample size, duration
- Defining a hypothesis: "Gemini Flash at temperature 0.2 reduces hallucination by 20%"
- Splitting traffic: percentage-based routing via `ExperimentRouter`
- Avoiding selection bias: random assignment using consistent session hashing
- Key file: `internal/llmops/experiments.go`

**Experiment Definition:**
```go
exp := &Experiment{
    ID:          "exp-2026-q1-flash-vs-sonnet",
    Name:        "Gemini Flash vs Claude Sonnet for code review",
    Hypothesis:  "Flash reduces latency by 30% with equivalent quality",
    TrafficSplit: map[string]float64{
        "control":   0.50, // Claude Sonnet 3.5
        "treatment": 0.50, // Gemini Flash 2.0
    },
    MinSamples:  200,
    MaxDuration: 7 * 24 * time.Hour,
    Metrics: []string{"latency_ms", "quality_score", "cost_usd"},
}
```

### Video 1.2: Running and Monitoring Experiments (15 min)

**Topics:**
- Starting an experiment via REST API: `POST /v1/llmops/experiments`
- Real-time experiment status: `GET /v1/llmops/experiments/{id}`
- Traffic allocation enforcement in the `ExperimentRouter` middleware
- Observing sample accumulation: samples per variant, confidence interval width
- Pausing and resuming experiments without losing collected data

**Start an Experiment:**
```bash
curl -X POST http://localhost:7061/v1/llmops/experiments \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "flash-vs-sonnet-code-review",
    "variants": [
      {"id": "control",   "provider": "claude",  "model": "claude-sonnet-4-5"},
      {"id": "treatment", "provider": "gemini",  "model": "gemini-flash-2.0"}
    ],
    "traffic_split": {"control": 0.5, "treatment": 0.5},
    "min_samples": 200,
    "metrics": ["latency_ms", "quality_score", "cost_usd"]
  }'
```

**Check Experiment Status:**
```bash
curl http://localhost:7061/v1/llmops/experiments/exp-abc123 \
  -H "Authorization: Bearer $API_KEY"
```

```json
{
  "id": "exp-abc123",
  "status": "running",
  "samples": {"control": 87, "treatment": 91},
  "metrics": {
    "control":   {"latency_ms": 1240, "quality_score": 0.88, "cost_usd": 0.0018},
    "treatment": {"latency_ms":  620, "quality_score": 0.86, "cost_usd": 0.0006}
  },
  "confidence": 0.73
}
```

---

## Module 2: Dataset Management (30 min)

### Video 2.1: Creating and Versioning Datasets (15 min)

**Topics:**
- Dataset anatomy: input prompts, reference outputs, metadata labels
- Creating a dataset via REST API: `POST /v1/llmops/datasets`
- Versioning: immutable dataset snapshots linked to experiment runs
- Importing datasets from JSONL files or CSV
- Dataset validation: checking for duplicates, empty inputs, encoding issues
- Key file: `internal/llmops/datasets.go`

**Create a Dataset:**
```bash
curl -X POST http://localhost:7061/v1/llmops/datasets \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "code-review-gold-standard-v1",
    "description": "200 hand-labeled Go code review examples",
    "examples": [
      {
        "input": "Review this function for correctness: func add(a, b int) int { return a - b }",
        "reference_output": "Bug: subtraction used instead of addition.",
        "labels": {"category": "bug", "language": "go"}
      }
    ]
  }'
```

**List Dataset Versions:**
```bash
curl http://localhost:7061/v1/llmops/datasets/ds-abc/versions \
  -H "Authorization: Bearer $API_KEY"
```

### Video 2.2: Querying and Filtering Datasets (15 min)

**Topics:**
- Filtering by label: retrieve only examples of a given category
- Stratified sampling: ensuring balanced representation across categories
- Splitting into train/eval/test subsets deterministically by hash
- Exporting a dataset version to JSONL for offline analysis
- Dataset lineage: tracing which experiment runs consumed which version

**Filter by Label:**
```bash
curl "http://localhost:7061/v1/llmops/datasets/ds-abc/examples?label_category=bug&limit=50" \
  -H "Authorization: Bearer $API_KEY"
```

---

## Module 3: Prompt Versioning (30 min)

### Video 3.1: The Prompt Registry (15 min)

**Topics:**
- Why prompt versioning matters: reproducible experiments require pinned prompts
- The `PromptRegistry`: stores prompt text, variables, version hash, author, timestamp
- Semantic versioning for prompts: `v1.0.0`, `v1.1.0-beta`
- Diff between prompt versions: what changed and why
- Locking a prompt version to an experiment run
- Key file: `internal/llmops/prompts.go`

**Register a Prompt:**
```bash
curl -X POST http://localhost:7061/v1/llmops/prompts \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "code-review-system-prompt",
    "version": "1.2.0",
    "text": "You are an expert Go engineer. Review the following code for correctness, performance, and style. Be concise. {{language}} code:\n\n{{code}}",
    "variables": ["language", "code"],
    "change_notes": "Added language variable for multi-language support"
  }'
```

### Video 3.2: Prompt Experiments and Rollback (15 min)

**Topics:**
- Running two prompt versions in the same A/B experiment
- Comparing prompt v1 vs. v2 outcomes on the same dataset
- Rolling back to a previous prompt version in production
- Automated prompt regression detection: alert when quality_score drops > 5%
- Prompt template rendering with variable substitution

**Rollback a Prompt:**
```bash
curl -X POST http://localhost:7061/v1/llmops/prompts/code-review-system-prompt/rollback \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"target_version": "1.1.0"}'
```

---

## Module 4: Evaluation Metrics (30 min)

### Video 4.1: Automated Metrics (15 min)

**Topics:**
- Latency: p50, p95, p99 response time per variant
- Cost: per-request USD cost from provider usage data
- Token efficiency: output tokens / input tokens ratio
- BLEU and ROUGE-L: automated text similarity against reference outputs
- LLM-as-judge: using a separate strong LLM to score response quality (0-10)
- Key file: `internal/llmops/evaluation.go`

**Evaluation Request:**
```bash
curl -X POST http://localhost:7061/v1/llmops/evaluate \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "experiment_id": "exp-abc123",
    "dataset_id":    "ds-abc",
    "dataset_version": "v1",
    "metrics": ["latency_ms", "cost_usd", "bleu", "llm_judge"],
    "judge_provider": "claude"
  }'
```

**Evaluation Results:**
```json
{
  "experiment_id": "exp-abc123",
  "results": {
    "control": {
      "latency_p95_ms": 1850,
      "cost_usd_per_req": 0.0018,
      "bleu": 0.71,
      "llm_judge_avg": 7.4
    },
    "treatment": {
      "latency_p95_ms":  890,
      "cost_usd_per_req": 0.0006,
      "bleu": 0.69,
      "llm_judge_avg": 7.1
    }
  },
  "winner": "treatment",
  "confidence": 0.91
}
```

### Video 4.2: Statistical Significance (15 min)

**Topics:**
- p-value interpretation: when is a difference real vs. random noise?
- Sample size calculators: how many examples do you need?
- Effect size: practical significance vs. statistical significance
- Sequential testing: stopping early when significance is reached
- Reporting: generating a human-readable experiment summary

---

## Module 5: Promoting a Winner (30 min)

### Video 5.1: Promotion Workflow (15 min)

**Topics:**
- Promotion criteria: confidence threshold, effect size, minimum sample count
- The `PromoteExperiment` API: updates default provider/prompt in the registry
- Blue-green promotion: gradually shifting 100% of traffic to the winner
- Post-promotion monitoring: watch for regressions in production metrics
- Automated rollback trigger: if production metrics degrade, revert automatically

**Promote Winner:**
```bash
curl -X POST http://localhost:7061/v1/llmops/experiments/exp-abc123/promote \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "variant": "treatment",
    "rollout_percentage": 100,
    "auto_rollback_threshold": {"quality_score_drop": 0.05}
  }'
```

### Video 5.2: Continuous Evaluation in Production (15 min)

**Topics:**
- Sampling 5% of live traffic for continuous evaluation
- Nightly batch evaluation against the gold-standard dataset
- Alerting on metric drift: Prometheus alert rule for quality regression
- Integrating evaluation results into the Grafana LLMOps dashboard
- Scheduling continuous evaluation jobs via the `BackgroundTasks` module

---

## Code Demo: End-to-End Experiment

This demo runs a complete experiment comparing two prompts for a code review task:

1. Create dataset with 50 labeled Go code review examples
2. Register prompt v1 (generic) and prompt v2 (Go-specific)
3. Start experiment with 50/50 traffic split, 50-sample minimum
4. Run evaluation using BLEU and LLM-as-judge metrics
5. Interpret results and promote the winning prompt to production

**Key takeaways from the demo:**
- Always pin both the prompt version and dataset version to an experiment for reproducibility
- LLM-as-judge scores correlate better with human judgment than BLEU alone for open-ended tasks
- Confidence of 0.90+ before promotion prevents premature decisions on noisy data
- Post-promotion monitoring is as important as the experiment itself

---

## Key Takeaways

- LLMOps experiments require the same rigor as traditional A/B tests: clear hypothesis,
  sufficient sample size, and statistical significance before declaring a winner.
- Prompt versioning and dataset versioning together ensure every experiment is
  fully reproducible months after it was run.
- Evaluation metrics must be chosen to match the task: BLEU for factual retrieval,
  LLM-as-judge for open-ended quality, latency and cost for operational decisions.
- Continuous evaluation in production catches regressions that point experiments miss.

---

## Related Courses

- **Course 67: LLMOps Experimentation** — Introductory overview of the LLMOps module
- **Course 75: Performance Tuning** — Baseline benchmarks that feed into experiment metrics
- **Course 80: Benchmarking and Provider Evaluation** — Standardized benchmark suites
- **Course 84: Monitoring, Dashboards and Alerting** — Grafana dashboards for LLMOps metrics
