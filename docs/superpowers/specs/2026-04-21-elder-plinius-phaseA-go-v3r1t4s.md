# Phase-A Plan — go-v3r1t4s (2026-04-21)

**Status:** GATED. Awaiting explicit approval.
**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-4
**Index:** `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md`
**Upstream Python:** To be located during brainstorm. Triage §2 maps it to "startup `internal/verifier/` truthfulness checks"; upstream is elder-plinius' "V3R1T4S" ("veritas") truth/verification helper.
**Defensible-subset justification:** Per `docs/research/go-elder-plinius-v3_triage.md` §2 table: "Extend startup `internal/verifier/` truthfulness checks — Real impl needed". Truth-claim verification auxiliary for the debate validation phase; no dual-use surface.

## 1. Upstream behavioral surface

**Placeholder — to be derived during `superpowers:brainstorming` against
Python upstream.** Do NOT copy signatures from the v3 Go codegen
scaffold — semantic bugs contaminate its type signatures.

## 2. Proposed Go API (draft, from scaffold — unverified)

Based on `docs/research/go-elder-plinius-v3/go-elder-plinius/go-v3r1t4s/pkg/client/client.go`
and `pkg/types/types.go`, the scaffold exposes these (unverified) symbols:

- `pkg/client/client.go:22: type Client struct`
- `pkg/client/client.go:28: func New(opts ...config.Option) (*Client, error)`
- `pkg/client/client.go:38: func NewFromConfig(cfg *config.Config) (*Client, error)`
- `pkg/client/client.go:47: func (c *Client) Close() error`
- `pkg/client/client.go:54: func (c *Client) Config() *config.Config`
- `pkg/client/client.go:57: func (c *Client) VerifyClaim(ctx context.Context, req VerifyRequest) (*VerifyResult, error)`
- `pkg/client/client.go:66: func (c *Client) CheckConsistency(ctx context.Context, responses []string, models []string) (*ConsistencyCheck, error)`
- `pkg/client/client.go:72: func (c *Client) DetectHallucination(ctx context.Context, response string, model string) (*HallucinationResult, error)`
- `pkg/client/client.go:78: func (c *Client) CompareModels(ctx context.Context, claim string, models []string) (*ModelComparison, error)`
- `pkg/client/client.go:84: func (c *Client) GetFactSources(ctx context.Context, claim string) ([]Evidence, error)`
- `pkg/client/client.go:90: func (c *Client) BatchVerify(ctx context.Context, claims []string) ([]VerifyResult, error)`
- `pkg/types/types.go:11: type VerifyRequest struct`
- `pkg/types/types.go:20: func (o *VerifyRequest) Validate() error`
- `pkg/types/types.go:28: type VerifyResult struct`
- `pkg/types/types.go:38: func (o *VerifyResult) Validate() error`
- `pkg/types/types.go:46: type Evidence struct`
- `pkg/types/types.go:54: type Contradiction struct`
- `pkg/types/types.go:62: type ConsistencyCheck struct`
- `pkg/types/types.go:70: type HallucinationResult struct`
- `pkg/types/types.go:78: type FactCheck struct`
- `pkg/types/types.go:86: type ModelComparison struct`

These are starting points for review, NOT implementation commitments.

## 3. Core-surface scope (4 days)

Proposed subset of §2 to implement for Phase-A core:

- Claim-extraction: parse a model response into a list of discrete atomic claims.
- Source-check interface: pluggable `SourceChecker` with a no-op default impl (Phase-A ships the interface + skeleton; concrete checkers are Phase-B).
- `VerifyClaim` / `BatchVerify` with configurable confidence threshold; returns `VerifyResult` with per-claim verdict (supported / contradicted / unverified).

## 4. Full-spec scope (2 weeks)

Everything beyond core:

- `CheckConsistency` multi-response cross-checker with `Contradiction` reporter.
- `DetectHallucination` single-response hallucination scorer.
- `CompareModels` multi-model agreement matrix.
- `GetFactSources` concrete `SourceChecker` implementation (e.g., Wikipedia API, curated corpora) — **to be decided during brainstorm**, Phase-A ships the interface only.
- Wire into debate service validation phase so adversarial rounds can cite `VerifyResult` output.

## 5. Test plan

- **Unit:** per-function table tests (target: 100% coverage per CLAUDE.md §1).
- **Integration:** interaction with the debate validation phase — a canned debate transcript runs through `BatchVerify`, resulting verdicts feed the debate's `Validation → Polish` transition.
- **Fixture contributions:** N/A — verification module, no attack-pattern output (though hallucination fixtures feed verifier regression tests indirectly).

## 6. Integration point

Intended consumer: debate validation phase.
Likely path: `internal/elder_plinius/v3r1t4s/` OR a top-level submodule.
Decided during brainstorm.

## 7. Documentation deliverables

- `CLAUDE.md` (per CLAUDE.md §7).
- `AGENTS.md`.
- `README.md`.
- `docs/` module documentation.

## 8. Risks

- False authority: labeling a claim "verified" when the source-check is a no-op would mislead callers. Default `SourceChecker` must return `unverified`, never `supported`, until a real checker is wired.
- Scope overlap with `internal/verifier/` startup checks — the two components must partition cleanly (startup verifier validates provider health; v3r1t4s validates runtime claims).

## 9. Approval checkpoint

> "Approve Phase-A for go-v3r1t4s — INTERNAL only, no public repo,
>  clean-room re-implementation from Python upstream."
