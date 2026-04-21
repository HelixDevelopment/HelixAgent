# Phase-A Plan — go-cl4r1t4s (2026-04-21)

**Status:** GATED. Awaiting explicit approval.
**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-4
**Index:** `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md`
**Upstream Python:** To be located during brainstorm. Upstream is elder-plinius' "CL4R1T4S" ("claritas") system-prompt archive. Note: per `MEMORY.md`, CL4R1T4S research integration is already complete under `docs/research/`; this Go module consumes that corpus, it does NOT extract or publish new system prompts.
**Defensible-subset justification:** Per `docs/research/go-elder-plinius-v3_triage.md` §2 table: "Structured access to the already-added CL4R1T4S corpus — Real impl needed". **System-prompt extraction DETECTION (not bypass)** — feeds the `system_prompt_extraction` guardrail classifier with known-leak reference patterns.

## 1. Upstream behavioral surface

**Placeholder — to be derived during `superpowers:brainstorming` against
Python upstream.** Do NOT copy signatures from the v3 Go codegen
scaffold — semantic bugs contaminate its type signatures.

## 2. Proposed Go API (draft, from scaffold — unverified)

Based on `docs/research/go-elder-plinius-v3/go-elder-plinius/go-cl4r1t4s/pkg/client/client.go`
and `pkg/types/types.go`, the scaffold exposes these (unverified) symbols:

- `pkg/client/client.go:22: type Client struct`
- `pkg/client/client.go:28: func New(opts ...config.Option) (*Client, error)`
- `pkg/client/client.go:38: func NewFromConfig(cfg *config.Config) (*Client, error)`
- `pkg/client/client.go:47: func (c *Client) Close() error`
- `pkg/client/client.go:54: func (c *Client) Config() *config.Config`
- `pkg/client/client.go:57: func (c *Client) SearchPrompts(ctx context.Context, opts SearchOptions) ([]PromptEntry, int, error)`
- `pkg/client/client.go:67: func (c *Client) GetPromptByID(ctx context.Context, id string) (*PromptEntry, error)`
- `pkg/client/client.go:73: func (c *Client) GetByCompany(ctx context.Context, company string) ([]PromptEntry, error)`
- `pkg/client/client.go:79: func (c *Client) GetByCategory(ctx context.Context, category string) ([]PromptEntry, error)`
- `pkg/client/client.go:85: func (c *Client) ComparePrompts(ctx context.Context, ids []string) (*ComparisonResult, error)`
- `pkg/client/client.go:91: func (c *Client) GetArchiveStats(ctx context.Context) (*ArchiveStats, error)`
- `pkg/client/client.go:97: func (c *Client) ExportToFormat(ctx context.Context, format string, opts ExportOptions) ([]byte, error)`
- `pkg/client/client.go:103: func (c *Client) AnalyzeTrends(ctx context.Context, opts TrendOptions) (*TrendAnalysis, error)`
- `pkg/types/types.go:11: type SystemPrompt struct`
- `pkg/types/types.go:26: func (o *SystemPrompt) Validate() error`
- `pkg/types/types.go:43: type PromptEntry struct`
- `pkg/types/types.go:56: func (o *PromptEntry) Validate() error`
- `pkg/types/types.go:73: type SearchOptions struct`
- `pkg/types/types.go:85: func (o *SearchOptions) Validate() error`
- `pkg/types/types.go:96: func (o *SearchOptions) Defaults()`
- `pkg/types/types.go:101: type ArchiveStats struct`
- `pkg/types/types.go:110: type ComparisonResult struct`
- `pkg/types/types.go:118: type ExportOptions struct`
- `pkg/types/types.go:126: type TrendOptions struct`
- `pkg/types/types.go:134: func (o *TrendOptions) Validate() error`
- `pkg/types/types.go:139: func (o *TrendOptions) Defaults()`
- `pkg/types/types.go:146: type TrendAnalysis struct`
- `pkg/types/types.go:153: type TrendPoint struct`

These are starting points for review, NOT implementation commitments.

## 3. Core-surface scope (4 days)

Proposed subset of §2 to implement for Phase-A core:

- Extraction-pattern detector: given a candidate LLM response, detect whether it appears to contain a system prompt leak using signatures derived from known archive entries (structural markers: role-preamble boilerplate, known company-specific disclaimers, tool-enumeration leakage).
- Corpus index lookup: `GetByCompany` / `GetByCategory` read-only access to the already-ingested CL4R1T4S corpus under `docs/research/`.
- `SystemPrompt` + `PromptEntry` types with `Validate`.

## 4. Full-spec scope (2 weeks)

Everything beyond core:

- `SearchPrompts` full-text / vector search.
- `ComparePrompts(ids)` diff viewer (read-only comparison of two archive entries).
- `AnalyzeTrends(opts)` + `TrendAnalysis` + `TrendPoint` temporal analysis of how system-prompt structure has evolved across providers.
- `ExportToFormat(format, opts)` for offline analysis — **redacted**, must not export verbatim prompt bodies by default.
- Wire detector output into the `system_prompt_extraction` guardrail as a confidence-boost signal when a response matches archive patterns.

## 5. Test plan

- **Unit:** per-function table tests (target: 100% coverage per CLAUDE.md §1).
- **Integration:** interaction with DeepTeamRedTeamer + `system_prompt_extraction` guardrail — synthetic responses (both leaks and non-leaks) are scored; detector verdict is compared against golden labels.
- **Fixture contributions:** labeled detection cases feed `internal/security/redteam/fixtures/system_prompt_extraction/` — expanding the Phase-5 fixture corpus with structured leak patterns.

## 6. Integration point

Intended consumer: DeepTeamRedTeamer + `system_prompt_extraction` guardrail.
Likely path: `internal/elder_plinius/cl4r1t4s/` OR a top-level submodule.
Decided during brainstorm.

## 7. Documentation deliverables

- `CLAUDE.md` (per CLAUDE.md §7).
- `AGENTS.md`.
- `README.md`.
- `docs/` module documentation.

## 8. Risks

- Dual-use framing: this module ships a signature catalog of real system-prompt leaks. Phase-A MUST remain detection-only — no API surface for "generate an extraction prompt" or "replay an extraction attack". Documentation must be unambiguous that this is the defender's side.
- Corpus churn: CL4R1T4S upstream updates frequently. Module must ship a pinned snapshot and a re-ingest procedure, not a live fetch.

## 9. Approval checkpoint

> "Approve Phase-A for go-cl4r1t4s — INTERNAL only, no public repo,
>  clean-room re-implementation from Python upstream."
