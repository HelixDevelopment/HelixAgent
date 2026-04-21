# Phase-A Plan — go-ourobopus (2026-04-21)

**Status:** GATED. Awaiting explicit approval.
**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-4
**Index:** `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md`
**Upstream Python:** To be located during brainstorm. Triage §1.4 labels the upstream as an ~11 KB self-improvement agent POC. The elder-plinius Python source is the self-referential / ouroboros-style prompt refinement loop; the Go Phase-A repurposes it as a **cycle detector**, not a refinement driver.
**Defensible-subset justification:** The triage originally flagged `go-ourobopus` as trivial, but it was promoted into the Phase-4 defensible subset by the parent design (§Phase-4) to guard the debate service against recursion/self-reference attacks. Defensive-only.

## 1. Upstream behavioral surface

**Placeholder — to be derived during `superpowers:brainstorming` against
Python upstream.** Do NOT copy signatures from the v3 Go codegen
scaffold — semantic bugs contaminate its type signatures.

## 2. Proposed Go API (draft, from scaffold — unverified)

Based on `docs/research/go-elder-plinius-v3/go-elder-plinius/go-ourobopus/pkg/client/client.go`
and `pkg/types/types.go`, the scaffold exposes these (unverified) symbols:

- `pkg/client/client.go:22: type Client struct`
- `pkg/client/client.go:28: func New(opts ...config.Option) (*Client, error)`
- `pkg/client/client.go:38: func NewFromConfig(cfg *config.Config) (*Client, error)`
- `pkg/client/client.go:47: func (c *Client) Close() error`
- `pkg/client/client.go:54: func (c *Client) Config() *config.Config`
- `pkg/client/client.go:57: func (c *Client) SelfReflect(ctx context.Context, prompt string, model string) (*SelfReflection, error)`
- `pkg/client/client.go:63: func (c *Client) Refine(ctx context.Context, cfg RefinementConfig) (*RefinementResult, error)`
- `pkg/client/client.go:73: func (c *Client) MetaEvaluate(ctx context.Context, prompt string, output string, criteria []string) (*MetaEvaluation, error)`
- `pkg/client/client.go:79: func (c *Client) SelfImprove(ctx context.Context, prompt string, model string, iterations int) (*RefinementResult, error)`
- `pkg/client/client.go:85: func (c *Client) GetMetaPatterns(ctx context.Context) ([]MetaPrompt, error)`
- `pkg/types/types.go:11: type MetaPrompt struct`
- `pkg/types/types.go:22: func (o *MetaPrompt) Validate() error`
- `pkg/types/types.go:36: type SelfReflection struct`
- `pkg/types/types.go:45: type IterationResult struct`
- `pkg/types/types.go:54: func (o *IterationResult) Validate() error`
- `pkg/types/types.go:62: type RefinementConfig struct`
- `pkg/types/types.go:72: func (o *RefinementConfig) Validate() error`
- `pkg/types/types.go:83: func (o *RefinementConfig) Defaults()`
- `pkg/types/types.go:93: type RefinementResult struct`
- `pkg/types/types.go:102: type MetaEvaluation struct`

These are starting points for review, NOT implementation commitments.

## 3. Core-surface scope (4 days)

Proposed subset of §2 to implement for Phase-A core:

- Cycle-detection in prompts: given a debate conversation transcript or a self-referential prompt chain, detect when the same hash-normalized turn has appeared N times (configurable) within a window — the signature of a recursion / stuck-loop attack.
- Hard cycle limits: emit a structured signal (via `go-plinius-common`'s `PliniusError` with `ErrCodeRecursionDetected`) that the debate orchestrator can trap and short-circuit.
- `RefinementConfig` with `MaxIterations`, `MaxIdenticalTurns`, `WindowSize` parameters and `Validate`/`Defaults`.

## 4. Full-spec scope (2 weeks)

Everything beyond core:

- Semantic cycle detection (embedding-similarity instead of hash-match) for near-duplicate turn detection.
- `SelfReflect` / `MetaEvaluate` / `SelfImprove` / `Refine` — **scope note:** these upstream APIs are about *driving* iterative refinement. In HelixAgent they are reframed as *observing* iterative refinement and flagging pathological recursion. Phase-A full-spec ships the observer variants, not the drivers. If the drivers are needed, they come as a separate proposal.
- `GetMetaPatterns` catalog of known recursion-attack patterns (including self-referential jailbreak attempts) that `cycle-detection` can match against.
- Wire into the debate orchestrator's `Convergence` phase as a safety-net check.

## 5. Test plan

- **Unit:** per-function table tests (target: 100% coverage per CLAUDE.md §1).
- **Integration:** interaction with the debate orchestrator — a canned debate transcript that loops on the same turn triggers the cycle detector and causes the debate to abort cleanly instead of hanging.
- **Fixture contributions:** recursion-attack patterns feed `internal/security/redteam/fixtures/recursive_injection/` or similar as a new attack class.

## 6. Integration point

Intended consumer: debate recursion protection.
Likely path: `internal/elder_plinius/ourobopus/` OR a top-level submodule.
Decided during brainstorm.

## 7. Documentation deliverables

- `CLAUDE.md` (per CLAUDE.md §7).
- `AGENTS.md`.
- `README.md`.
- `docs/` module documentation.

## 8. Risks

- Upstream scope drift: the Python upstream's primary purpose is *driving* self-improvement loops, not *guarding against* them. Phase-A explicitly reframes the scope to defensive observation. This must be called out in the module README and CLAUDE.md to prevent future contributors from reintroducing the driver APIs as a "missing feature".
- False positives on legitimate iteration: a convergent debate may revisit the same argument; cycle detection must distinguish "same turn N times" (pathological) from "same topic N times" (normal debate). Tunable thresholds + per-topic normalization are required.

## 9. Approval checkpoint

> "Approve Phase-A for go-ourobopus — INTERNAL only, no public repo,
>  clean-room re-implementation from Python upstream."
