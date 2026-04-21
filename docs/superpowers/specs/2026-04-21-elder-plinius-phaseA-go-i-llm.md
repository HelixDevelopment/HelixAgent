# Phase-A Plan — go-i-llm (2026-04-21)

**Status:** GATED. Awaiting explicit approval.
**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-4
**Index:** `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md`
**Upstream Python:** To be located during brainstorm. Triage §2 maps it to "CoT/ReAct/ToT patterns → `internal/agentic/`", pointing at elder-plinius' introspection/pattern library for LLM reasoning.
**Defensible-subset justification:** Per `docs/research/go-elder-plinius-v3_triage.md` §2 table: "CoT/ReAct/ToT patterns → internal/agentic/ — Real impl needed". Introspection for provider scoring; no dual-use surface.

## 1. Upstream behavioral surface

**Placeholder — to be derived during `superpowers:brainstorming` against
Python upstream.** Do NOT copy signatures from the v3 Go codegen
scaffold — semantic bugs contaminate its type signatures.

## 2. Proposed Go API (draft, from scaffold — unverified)

Based on `docs/research/go-elder-plinius-v3/go-elder-plinius/go-i-llm/pkg/client/client.go`
and `pkg/types/types.go`, the scaffold exposes these (unverified) symbols:

- `pkg/client/client.go:22: type Client struct`
- `pkg/client/client.go:28: func New(opts ...config.Option) (*Client, error)`
- `pkg/client/client.go:38: func NewFromConfig(cfg *config.Config) (*Client, error)`
- `pkg/client/client.go:47: func (c *Client) Close() error`
- `pkg/client/client.go:54: func (c *Client) Config() *config.Config`
- `pkg/client/client.go:57: func (c *Client) GetPattern(ctx context.Context, id string) (*ConversationPattern, error)`
- `pkg/client/client.go:63: func (c *Client) ListPatterns(ctx context.Context, category string) ([]ConversationPattern, error)`
- `pkg/client/client.go:69: func (c *Client) RenderPattern(ctx context.Context, pattern ConversationPattern, vars map[string]string) (string, error)`
- `pkg/client/client.go:75: func (c *Client) CreateAgent(ctx context.Context, cfg AgentConfig) (*Agent, error)`
- `pkg/client/client.go:85: func (c *Client) RunChain(ctx context.Context, chain PromptChain, inputs map[string]string) (*ChainResult, error)`
- `pkg/client/client.go:91: func (c *Client) ChainOfThought(ctx context.Context, problem string, model string) (*ChainResult, error)`
- `pkg/client/client.go:97: func (c *Client) TreeOfThought(ctx context.Context, problem string, model string, breadth int) (*TreeResult, error)`
- `pkg/client/client.go:103: func (c *Client) GetCategories(ctx context.Context) ([]string, error)`
- `pkg/types/types.go:11: type ConversationPattern struct`
- `pkg/types/types.go:22: func (o *ConversationPattern) Validate() error`
- `pkg/types/types.go:36: type ReActStep struct`
- `pkg/types/types.go:45: type AgentConfig struct`
- `pkg/types/types.go:55: func (o *AgentConfig) Validate() error`
- `pkg/types/types.go:66: func (o *AgentConfig) Defaults()`
- `pkg/types/types.go:71: type Tool struct`
- `pkg/types/types.go:78: func (o *Tool) Validate() error`
- `pkg/types/types.go:89: type ChainResult struct`
- `pkg/types/types.go:98: type PromptChain struct`
- `pkg/types/types.go:107: func (o *PromptChain) Validate() error`
- `pkg/types/types.go:121: type Agent struct`
- `pkg/types/types.go:127: type TreeResult struct`
- `pkg/types/types.go:136: type ChainStep struct`
- `pkg/types/types.go:144: func (o *ChainStep) Validate() error`

These are starting points for review, NOT implementation commitments.

## 3. Core-surface scope (4 days)

Proposed subset of §2 to implement for Phase-A core:

- Model-metadata extraction: query a provider via the provider registry and record its advertised context window, modalities, supported tool calling, and pricing tier into a `ConversationPattern`-adjacent descriptor.
- Capability probing: run a fixed short probe prompt per provider (tool-use, structured-output, long-context truncation) and record per-capability pass/fail.
- `GetPattern` / `ListPatterns` / `GetCategories` read-only lookup over a bundled pattern catalog.

## 4. Full-spec scope (2 weeks)

Everything beyond core:

- `RenderPattern(vars)` template interpolation for CoT/ReAct scaffolds.
- `ChainOfThought` / `TreeOfThought` executors wired to HelixAgent's agentic graph.
- `CreateAgent` + `RunChain` agent-with-tool execution (overlaps with `internal/agentic/` — must integrate, not duplicate).
- Feed capability-probe results into LLMsVerifier's `Capability` score component as a calibration input.

## 5. Test plan

- **Unit:** per-function table tests (target: 100% coverage per CLAUDE.md §1).
- **Integration:** interaction with LLMsVerifier + provider registry — a test provider is probed, its capability map is compared against a golden descriptor.
- **Fixture contributions:** N/A — introspection module, no attack-pattern output.

## 6. Integration point

Intended consumer: LLMsVerifier + provider registry.
Likely path: `internal/elder_plinius/i_llm/` OR a top-level submodule.
Decided during brainstorm.

## 7. Documentation deliverables

- `CLAUDE.md` (per CLAUDE.md §7).
- `AGENTS.md`.
- `README.md`.
- `docs/` module documentation.

## 8. Risks

- Overlap with existing `internal/agentic/` CoT/ToT work — must be additive (introspection + pattern catalog), not a reimplementation of the agentic graph.
- Probe cost: capability probes run per-provider on every verifier pass; must be rate-limited and cached to avoid quota blowouts.

## 9. Approval checkpoint

> "Approve Phase-A for go-i-llm — INTERNAL only, no public repo,
>  clean-room re-implementation from Python upstream."
