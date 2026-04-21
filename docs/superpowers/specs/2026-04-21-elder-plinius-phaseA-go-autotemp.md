# Phase-A Plan — go-autotemp (2026-04-21)

**Status:** GATED. Awaiting explicit approval.
**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-4
**Index:** `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md`
**Upstream Python:** To be located during brainstorm. Triage §2 describes it as a ~300-line Python script that sweeps temperatures and picks the best per-prompt.
**Defensible-subset justification:** Per `docs/research/go-elder-plinius-v3_triage.md` §2 table: "HelixLLM temperature optimisation pass — Real impl needed". Tuning augmentation for LLMsVerifier and the debate service; no dual-use surface.

## 1. Upstream behavioral surface

**Placeholder — to be derived during `superpowers:brainstorming` against
Python upstream.** Do NOT copy signatures from the v3 Go codegen
scaffold — semantic bugs contaminate its type signatures.

## 2. Proposed Go API (draft, from scaffold — unverified)

Based on `docs/research/go-elder-plinius-v3/go-elder-plinius/go-autotemp/pkg/client/client.go`
and `pkg/types/types.go`, the scaffold exposes these (unverified) symbols:

- `pkg/client/client.go:22: type Client struct`
- `pkg/client/client.go:28: func New(opts ...config.Option) (*Client, error)`
- `pkg/client/client.go:38: func NewFromConfig(cfg *config.Config) (*Client, error)`
- `pkg/client/client.go:47: func (c *Client) Close() error`
- `pkg/client/client.go:54: func (c *Client) Config() *config.Config`
- `pkg/client/client.go:57: func (c *Client) Run(ctx context.Context, opts RunOptions) (*RunResult, error)`
- `pkg/client/client.go:67: func (c *Client) RunAdvanced(ctx context.Context, opts AdvancedOptions) (*RunResult, error)`
- `pkg/client/client.go:77: func (c *Client) Evaluate(ctx context.Context, opts EvaluateOptions) (*EvaluateResult, error)`
- `pkg/client/client.go:87: func (c *Client) Benchmark(ctx context.Context, opts BenchmarkOptions) (*BenchmarkResult, error)`
- `pkg/types/types.go:11: type RunOptions struct`
- `pkg/types/types.go:21: func (o *RunOptions) Validate() error`
- `pkg/types/types.go:29: func (o *RunOptions) Defaults()`
- `pkg/types/types.go:34: type RunResult struct`
- `pkg/types/types.go:43: type TokenUsage struct`
- `pkg/types/types.go:50: type AdvancedOptions struct`
- `pkg/types/types.go:57: type EvaluateOptions struct`
- `pkg/types/types.go:67: func (o *EvaluateOptions) Validate() error`
- `pkg/types/types.go:75: func (o *EvaluateOptions) Defaults()`
- `pkg/types/types.go:81: type EvaluateResult struct`
- `pkg/types/types.go:88: type ScoreBreakdown struct`
- `pkg/types/types.go:99: type BenchmarkOptions struct`
- `pkg/types/types.go:110: func (o *BenchmarkOptions) Validate() error`
- `pkg/types/types.go:115: func (o *BenchmarkOptions) Defaults()`
- `pkg/types/types.go:120: type BenchmarkItem struct`
- `pkg/types/types.go:126: func (o *BenchmarkItem) Validate() error`
- `pkg/types/types.go:134: type BenchmarkResult struct`
- `pkg/types/types.go:140: type ModelBenchmark struct`
- `pkg/types/types.go:148: func (o *ModelBenchmark) Validate() error`

These are starting points for review, NOT implementation commitments.

## 3. Core-surface scope (4 days)

Proposed subset of §2 to implement for Phase-A core:

- Benchmark runner: sweep a configurable set of temperatures against a fixed prompt across a set of models via HelixAgent's existing provider registry.
- Temperature-selector heuristic: aggregate per-temperature `EvaluateResult` scores into a `ScoreBreakdown` and pick the best temperature per (prompt, model) pair.
- `RunOptions`, `RunResult`, `BenchmarkOptions`, `BenchmarkResult`, `BenchmarkItem` with `Validate`/`Defaults`.

## 4. Full-spec scope (2 weeks)

Everything beyond core:

- `RunAdvanced` with nucleus/top-p/top-k joint sweep.
- `Evaluate` with pluggable scoring functions (embedding similarity, rubric-based, LLM-as-judge).
- `ModelBenchmark` cross-model comparison with per-model ranking tables.
- Integration with LLMsVerifier scoring so temperature-sweep results feed the `ResponseSpeed` and `Capability` score components.
- Hook into the debate service to auto-tune debate-round temperature based on topic category.

## 5. Test plan

- **Unit:** per-function table tests (target: 100% coverage per CLAUDE.md §1).
- **Integration:** interaction with LLMsVerifier + debate service — a canned prompt is sent through the provider registry at multiple temperatures, scores are compared against golden values.
- **Fixture contributions:** N/A — tuning module, no attack-pattern output.

## 6. Integration point

Intended consumer: LLMsVerifier + debate service.
Likely path: `internal/elder_plinius/autotemp/` OR a top-level
submodule. Decided during brainstorm.

## 7. Documentation deliverables

- `CLAUDE.md` (per CLAUDE.md §7).
- `AGENTS.md`.
- `README.md`.
- `docs/` module documentation.

## 8. Risks

- Benchmark cost: sweep-by-default burns provider credits; module must gate runs behind an explicit opt-in flag and respect per-provider rate limits (the verifier already tracks these).
- Metric drift: scoring heuristic must be reproducible across runs, or downstream LLMsVerifier scores become non-deterministic. Fixture-based golden scores need to be version-locked.

## 9. Approval checkpoint

> "Approve Phase-A for go-autotemp — INTERNAL only, no public repo,
>  clean-room re-implementation from Python upstream."
