# Phase-A Plan — go-gandalf-solutions (2026-04-21)

**Status:** GATED. Awaiting explicit approval.
**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-4
**Index:** `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md`
**Upstream Python:** To be located during brainstorm. Source is the elder-plinius "Gandalf" Lakera CTF solutions archive (the Python repo bundles per-level and per-adventure writeups with categorized techniques).
**Defensible-subset justification:** Per `docs/research/go-elder-plinius-v3_triage.md` §2 table: "Guardrail regression fixtures — Fixture extraction, not full port". Read-only corpus that regression-tests HelixAgent's `StandardGuardrailPipeline` against known prompt-leak patterns; defensive use only.

## 1. Upstream behavioral surface

**Placeholder — to be derived during `superpowers:brainstorming` against
Python upstream.** Do NOT copy signatures from the v3 Go codegen
scaffold — semantic bugs contaminate its type signatures.

## 2. Proposed Go API (draft, from scaffold — unverified)

Based on `docs/research/go-elder-plinius-v3/go-elder-plinius/go-gandalf-solutions/pkg/client/client.go`
and `pkg/types/types.go`, the scaffold exposes these (unverified) symbols:

- `pkg/client/client.go:22: type Client struct`
- `pkg/client/client.go:28: func New(opts ...config.Option) (*Client, error)`
- `pkg/client/client.go:38: func NewFromConfig(cfg *config.Config) (*Client, error)`
- `pkg/client/client.go:47: func (c *Client) Close() error`
- `pkg/client/client.go:54: func (c *Client) Config() *config.Config`
- `pkg/client/client.go:57: func (c *Client) GetLevel(ctx context.Context, level int) (*LevelSolution, error)`
- `pkg/client/client.go:63: func (c *Client) GetAdventure(ctx context.Context, name string) (*AdventureSolution, error)`
- `pkg/client/client.go:69: func (c *Client) SearchSolutions(ctx context.Context, opts SearchOptions) ([]LevelSolution, error)`
- `pkg/client/client.go:79: func (c *Client) GetPromptLeaks(ctx context.Context, source string) ([]PromptLeak, error)`
- `pkg/client/client.go:85: func (c *Client) GetTechniques(ctx context.Context) ([]string, error)`
- `pkg/client/client.go:91: func (c *Client) GetCategories(ctx context.Context) ([]string, error)`
- `pkg/client/client.go:97: func (c *Client) GetArchiveStats(ctx context.Context) (*ArchiveStats, error)`
- `pkg/client/client.go:103: func (c *Client) ExportLevel(ctx context.Context, level int, format string) ([]byte, error)`
- `pkg/types/types.go:11: type LevelSolution struct`
- `pkg/types/types.go:23: func (o *LevelSolution) Validate() error`
- `pkg/types/types.go:34: type AdventureSolution struct`
- `pkg/types/types.go:43: func (o *AdventureSolution) Validate() error`
- `pkg/types/types.go:54: type PromptLeak struct`
- `pkg/types/types.go:65: func (o *PromptLeak) Validate() error`
- `pkg/types/types.go:76: type SearchOptions struct`
- `pkg/types/types.go:86: func (o *SearchOptions) Validate() error`
- `pkg/types/types.go:97: func (o *SearchOptions) Defaults()`
- `pkg/types/types.go:102: type ArchiveStats struct`

These are starting points for review, NOT implementation commitments.

## 3. Core-surface scope (4 days)

Proposed subset of §2 to implement for Phase-A core:

- Corpus loader: read a bundled JSON/YAML snapshot of level/adventure solutions shipped inside the module (no network fetch, no write path).
- Categorizer: `GetCategories`, `GetTechniques`, `GetLevel(n)`, `GetAdventure(name)` read-only lookups.
- `LevelSolution`, `AdventureSolution`, `PromptLeak` types with `Validate` guards against malformed fixtures.

## 4. Full-spec scope (2 weeks)

Everything beyond core:

- `SearchSolutions` / `GetPromptLeaks(source)` query paths with filter predicates.
- `GetArchiveStats` aggregation (counts by category, by level, by technique).
- `ExportLevel` format conversion (JSON / markdown / CSV).
- Integration adapter that feeds `DeepTeamRedTeamer` attack-catalog regression tests with categorized corpus entries.

## 5. Test plan

- **Unit:** per-function table tests (target: 100% coverage per CLAUDE.md §1).
- **Integration:** interaction with DeepTeamRedTeamer + StandardGuardrailPipeline — given a `LevelSolution` whose known-leak prompt is fed through the guardrail, the expected guardrail verdict is recorded as a regression fixture.
- **Fixture contributions:** categorized entries feed `internal/security/redteam/fixtures/` with labeled prompt-leak defense regression cases.

## 6. Integration point

Intended consumer: DeepTeamRedTeamer + StandardGuardrailPipeline.
Likely path: `internal/elder_plinius/gandalf_solutions/` OR a top-level
submodule. Decided during brainstorm.

## 7. Documentation deliverables

- `CLAUDE.md` (per CLAUDE.md §7).
- `AGENTS.md`.
- `README.md`.
- `docs/` module documentation.

## 8. Risks

- Corpus licensing: upstream Gandalf solutions are community-authored; module must ship only re-described categorizations, not verbatim attacker prompts, unless licensing is explicitly verified during brainstorm.
- Dual-use framing: even as a read-only corpus, the entries are attacker prompts. Package must enforce read-only API (no `Add`/`Mutate` methods) and documentation must frame as defensive regression material only.

## 9. Approval checkpoint

> "Approve Phase-A for go-gandalf-solutions — INTERNAL only, no public repo,
>  clean-room re-implementation from Python upstream."
