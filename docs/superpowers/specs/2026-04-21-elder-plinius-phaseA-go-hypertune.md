# Phase-A Plan — go-hypertune (2026-04-21)

**Status:** GATED. Awaiting explicit approval.
**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-4
**Index:** `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md`
**Upstream Python:** To be located during brainstorm. Upstream is elder-plinius' hyperparameter-search helper — sibling tool to autotemp that generalizes sweep to arbitrary sampling parameters.
**Defensible-subset justification:** Per `docs/research/go-elder-plinius-v3_triage.md` §2 table: "LLMsVerifier scorer tuning — Real impl needed". Tuning orchestration for LLMOps and planning handlers; no dual-use surface.

## 1. Upstream behavioral surface

**Placeholder — to be derived during `superpowers:brainstorming` against
Python upstream.** Do NOT copy signatures from the v3 Go codegen
scaffold — semantic bugs contaminate its type signatures.

## 2. Proposed Go API (draft, from scaffold — unverified)

Based on `docs/research/go-elder-plinius-v3/go-elder-plinius/go-hypertune/pkg/client/client.go`
and `pkg/types/types.go`, the scaffold exposes these (unverified) symbols:

- `pkg/client/client.go:22: type Client struct`
- `pkg/client/client.go:28: func New(opts ...config.Option) (*Client, error)`
- `pkg/client/client.go:38: func NewFromConfig(cfg *config.Config) (*Client, error)`
- `pkg/client/client.go:47: func (c *Client) Close() error`
- `pkg/client/client.go:54: func (c *Client) Config() *config.Config`
- `pkg/client/client.go:57: func (c *Client) Optimize(ctx context.Context, space ParameterSpace, cfg OptimizationConfig) (*OptimizationResult, error)`
- `pkg/client/client.go:63: func (c *Client) GridSearch(ctx context.Context, space ParameterSpace, cfg OptimizationConfig) (*OptimizationResult, error)`
- `pkg/client/client.go:69: func (c *Client) BayesianOptimize(ctx context.Context, space ParameterSpace, cfg OptimizationConfig) (*OptimizationResult, error)`
- `pkg/client/client.go:75: func (c *Client) Evaluate(ctx context.Context, params map[string]float64, prompt string, model string) (*TrialResult, error)`
- `pkg/client/client.go:81: func (c *Client) GetMetrics(ctx context.Context) ([]EvaluationMetric, error)`
- `pkg/client/client.go:87: func (c *Client) SuggestParameters(ctx context.Context, space ParameterSpace, history []TrialResult) (map[string]float64, error)`
- `pkg/types/types.go:11: type ParameterSpace struct`
- `pkg/types/types.go:22: func (o *ParameterSpace) Defaults()`
- `pkg/types/types.go:29: type OptimizationConfig struct`
- `pkg/types/types.go:39: func (o *OptimizationConfig) Validate() error`
- `pkg/types/types.go:50: type OptimizationResult struct`
- `pkg/types/types.go:59: type TrialResult struct`
- `pkg/types/types.go:68: type EvaluationMetric struct`
- `pkg/types/types.go:75: func (o *EvaluationMetric) Validate() error`

These are starting points for review, NOT implementation commitments.

## 3. Core-surface scope (4 days)

Proposed subset of §2 to implement for Phase-A core:

- `ParameterSpace` definition: typed ranges over temperature, top-p, top-k, max-tokens, frequency-penalty, presence-penalty.
- Optimizer loop: pluggable `Optimize` entry that delegates to one concrete strategy (grid search for Phase-A core).
- `TrialResult` + `OptimizationResult` accumulation and best-pick logic.
- `Evaluate(params, prompt, model)` single-trial evaluation backed by the provider registry.

## 4. Full-spec scope (2 weeks)

Everything beyond core:

- `BayesianOptimize` (Gaussian-process acquisition function).
- `SuggestParameters(space, history)` online-suggestion API driven by accumulated trial history.
- `EvaluationMetric` pluggable metric registry (latency, cost, rubric score, composite).
- Wire `OptimizationResult` outputs into LLMOps experiment tracking and planning handlers' cost/quality tradeoff decisions.

## 5. Test plan

- **Unit:** per-function table tests (target: 100% coverage per CLAUDE.md §1).
- **Integration:** interaction with LLMOps + planning handlers — grid-search result drives a planning handler's choice between fast/cheap and slow/expensive provider presets.
- **Fixture contributions:** N/A — tuning module, no attack-pattern output.

## 6. Integration point

Intended consumer: LLMOps + planning handlers.
Likely path: `internal/elder_plinius/hypertune/` OR a top-level
submodule. Decided during brainstorm.

## 7. Documentation deliverables

- `CLAUDE.md` (per CLAUDE.md §7).
- `AGENTS.md`.
- `README.md`.
- `docs/` module documentation.

## 8. Risks

- Scope creep into "full autoML" territory — Phase-A must cap at sampling-param tuning; model selection and prompt optimization are explicit non-goals.
- Interaction with autotemp: if both modules tune temperature, priority/override semantics must be explicit (hypertune wins when both are configured, per brainstorm default assumption).

## 9. Approval checkpoint

> "Approve Phase-A for go-hypertune — INTERNAL only, no public repo,
>  clean-room re-implementation from Python upstream."
