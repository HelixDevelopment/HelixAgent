# go-elder-plinius Phase-A Plan — 2026-04-21

**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-4
**Status:** GATED. No Phase-A code may be written until explicit
per-module user approval.

## Scope

The 9 defensible-subset modules. INTERNAL integration only — no public
`vasic-digital` / GitLab repos. Each module is re-implemented from its
Python upstream as a clean-room port, integrated as a HelixAgent
internal submodule (final path decided during per-module
brainstorm).

## Per-module specs

| Module | Spec | Scope (core / full) | Intended consumer |
|--------|------|---------------------|-------------------|
| go-plinius-common | [link](2026-04-21-elder-plinius-phaseA-go-plinius-common.md) | 4d / 2wk | Common types/errors library consumed by the other 8 |
| go-gandalf-solutions | [link](2026-04-21-elder-plinius-phaseA-go-gandalf-solutions.md) | 4d / 2wk | Read-only solutions archive for prompt-leak-defense testing |
| go-autotemp | [link](2026-04-21-elder-plinius-phaseA-go-autotemp.md) | 4d / 2wk | Benchmark-driven temperature auto-tuning |
| go-hypertune | [link](2026-04-21-elder-plinius-phaseA-go-hypertune.md) | 4d / 2wk | Hyperparameter tuning orchestration |
| go-i-llm | [link](2026-04-21-elder-plinius-phaseA-go-i-llm.md) | 4d / 2wk | Introspection layer for LLM providers |
| go-v3r1t4s | [link](2026-04-21-elder-plinius-phaseA-go-v3r1t4s.md) | 4d / 2wk | Truth/verification auxiliary |
| go-leakhub | [link](2026-04-21-elder-plinius-phaseA-go-leakhub.md) | 4d / 2wk | Prompt-leak corpus for DeepTeamRedTeamer (internal) |
| go-cl4r1t4s | [link](2026-04-21-elder-plinius-phaseA-go-cl4r1t4s.md) | 4d / 2wk | System-prompt extraction detection |
| go-ourobopus | [link](2026-04-21-elder-plinius-phaseA-go-ourobopus.md) | 4d / 2wk | Recursive/self-referential safety patterns |

Total commitment if all 9 approved: **~36 person-days core surface
/ ~18 person-weeks full-spec**.

## Workflow per module (once approved)

1. Run `superpowers:brainstorming` against the Python upstream to
   pick behavioral surface. DO NOT copy signatures from the v3 Go
   codegen scaffold — semantic bugs contaminated the type
   signatures (see `docs/research/go-elder-plinius-v3_triage.md`).
2. Produce Go API signatures derived from the Python source.
3. TDD-implement core surface with 100% test coverage per CLAUDE.md §1.
4. Integrate as an internal HelixAgent submodule. Likely path:
   `internal/elder_plinius/<module>/` OR a top-level submodule under
   the HelixAgent repo. Final path decided during brainstorm.
5. Documentation: CLAUDE.md + AGENTS.md + README.md + docs/ per
   CLAUDE.md §7.

**NO PUBLIC REPO.** Per parent design §Phase-4 and Bucket-3a policy.

## Per-module consumer-intent cross-reference

| Module | Primary consumer | Defensive framing |
|--------|------------------|-------------------|
| go-plinius-common | all other 8 modules | utility, no dual-use surface |
| go-gandalf-solutions | DeepTeamRedTeamer + StandardGuardrailPipeline | defensive test corpus (read-only) |
| go-autotemp | LLMsVerifier + debate service | tuning augmentation |
| go-hypertune | LLMOps + planning handlers | tuning orchestration |
| go-i-llm | LLMsVerifier + provider registry | introspection for scoring |
| go-v3r1t4s | debate validation phase | truth-claim verification |
| go-leakhub | DeepTeamRedTeamer fixture population | **expands Phase-5 fixture corpus** |
| go-cl4r1t4s | DeepTeamRedTeamer + system_prompt_extraction guardrail | detection (not bypass) |
| go-ourobopus | debate recursion protection | guard against self-referential attacks |

## Approval gate

Per-module approval expected in the form:
> "Approve Phase-A for <module> — INTERNAL only, no public repo,
>  clean-room re-implementation from Python upstream."

No approval → no code. Specs remain as documentation of what WOULD
be built if authorized.

## Cross-reference

- Parent design spec.
- Bucket-3a policy history: `docs/research/go-elder-plinius-v3_triage.md`
- Triage update: `docs/research/go-elder-plinius-v3_triage_update.md`
- Corrected integration plan: `docs/research/inbox/2026-04-21_go-elder-plinius_integration_plan_CORRECTED.md`
