# Phase-A Plan — go-leakhub (2026-04-21)

**Status:** GATED. Awaiting explicit approval.
**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-4
**Index:** `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md`
**Upstream Python:** To be located during brainstorm. Upstream is elder-plinius' leakhub corpus — a curated archive of real-world model-prompt-leak disclosures.
**Defensible-subset justification:** Per `docs/research/go-elder-plinius-v3_triage.md` §2 table: "Provider-boilerplate detector for LLMsVerifier (matches the JSON catalog added in `docs/cli-agents/provider_boilerplate_patterns.json`) — Real impl needed". Defensive detection corpus for DeepTeamRedTeamer fixture population; read-only.

## 1. Upstream behavioral surface

**Placeholder — to be derived during `superpowers:brainstorming` against
Python upstream.** Do NOT copy signatures from the v3 Go codegen
scaffold — semantic bugs contaminate its type signatures.

## 2. Proposed Go API (draft, from scaffold — unverified)

Based on `docs/research/go-elder-plinius-v3/go-elder-plinius/go-leakhub/pkg/client/client.go`
and `pkg/types/types.go`, the scaffold exposes these (unverified) symbols:

- `pkg/client/client.go:22: type Client struct`
- `pkg/client/client.go:28: func New(opts ...config.Option) (*Client, error)`
- `pkg/client/client.go:38: func NewFromConfig(cfg *config.Config) (*Client, error)`
- `pkg/client/client.go:47: func (c *Client) Close() error`
- `pkg/client/client.go:54: func (c *Client) Config() *config.Config`
- `pkg/client/client.go:57: func (c *Client) DetectLeak(ctx context.Context, opts DetectionOptions) (*DetectionResult, error)`
- `pkg/client/client.go:67: func (c *Client) SearchArchive(ctx context.Context, query string, limit int) ([]LeakEntry, error)`
- `pkg/client/client.go:73: func (c *Client) AddToArchive(ctx context.Context, entry LeakEntry) error`
- `pkg/client/client.go:79: func (c *Client) GetByModel(ctx context.Context, model string) ([]LeakEntry, error)`
- `pkg/client/client.go:85: func (c *Client) GetStats(ctx context.Context) (*ArchiveStats, error)`
- `pkg/client/client.go:91: func (c *Client) ExportArchive(ctx context.Context, format string) ([]byte, error)`
- `pkg/types/types.go:11: type LeakEntry struct`
- `pkg/types/types.go:23: func (o *LeakEntry) Validate() error`
- `pkg/types/types.go:34: type DetectionOptions struct`
- `pkg/types/types.go:42: func (o *DetectionOptions) Validate() error`
- `pkg/types/types.go:50: func (o *DetectionOptions) Defaults()`
- `pkg/types/types.go:57: type DetectionResult struct`
- `pkg/types/types.go:65: type LeakMatch struct`
- `pkg/types/types.go:73: type ArchiveStats struct`

These are starting points for review, NOT implementation commitments.

## 3. Core-surface scope (4 days)

Proposed subset of §2 to implement for Phase-A core:

- Corpus loader: load a bundled snapshot of labeled leak entries (per-model, per-date, per-category) from an embedded file; no network fetch, no public upload.
- `GetByModel(model)` read-only lookup.
- `LeakEntry`, `DetectionOptions`, `DetectionResult`, `LeakMatch` types with `Validate`/`Defaults`.
- Read-only stance: `AddToArchive` is **explicitly NOT included in core**; Phase-A ships without a write path.

## 4. Full-spec scope (2 weeks)

Everything beyond core:

- `DetectLeak(opts)` pattern-based detector that compares a candidate response against archive entries (substring / semantic similarity).
- `SearchArchive(query, limit)` full-text / tag-based search.
- `GetStats` aggregate counts.
- `ExportArchive(format)` for offline analysis.
- **Cross-reference:** populates `internal/security/redteam/fixtures/` classes further — leakhub entries become labeled regression inputs for the `system_prompt_extraction` and `sensitive_data_leak` guardrail families.

## 5. Test plan

- **Unit:** per-function table tests (target: 100% coverage per CLAUDE.md §1).
- **Integration:** interaction with DeepTeamRedTeamer — leak entries drive attack-catalog regression runs; guardrail verdicts are compared against golden labels.
- **Fixture contributions:** **This module is the primary fixture-corpus expansion mechanism for Phase-5.** Each leak entry becomes a labeled fixture under `internal/security/redteam/fixtures/` with the attack-class tag.

## 6. Integration point

Intended consumer: DeepTeamRedTeamer fixture population.
Likely path: `internal/elder_plinius/leakhub/` OR a top-level submodule.
Decided during brainstorm.

## 7. Documentation deliverables

- `CLAUDE.md` (per CLAUDE.md §7).
- `AGENTS.md`.
- `README.md`.
- `docs/` module documentation.

## 8. Risks

- Licensing / redistribution: real-world prompt leaks may be third-party-copyrighted or subject to platform ToS. Phase-A must ship only categorized summaries + detection patterns, not verbatim leaked prompts, unless per-entry licensing is explicitly verified during brainstorm.
- Dual-use: a leak corpus doubles as an attacker playbook. Package stays INTERNAL (no public repo) and the API surface is read-only; no `AddToArchive` in Phase-A.

## 9. Approval checkpoint

> "Approve Phase-A for go-leakhub — INTERNAL only, no public repo,
>  clean-room re-implementation from Python upstream."
