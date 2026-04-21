# go-elder-plinius Phase-A Plan — 2026-04-21

**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-4
**Status:** GATED. No Phase-A code may be written until explicit
per-module user approval.

## Scope

The 9 defensible-subset modules. Scaffolds (WIP, `ErrCodeUnimplemented`
bodies) are now published as public `vasic-digital` submodules on
GitHub + GitLab (see table below). Phase-A implementation work is
still GATED and will be performed against these repos once per-module
approval is granted. Policy updated 2026-04-21 (see §Publication update).

## Per-module specs

| Module (source dir) | Scaffold repo | Spec | Scope (core / full) | Intended consumer |
|---------------------|---------------|------|---------------------|-------------------|
| go-plinius-common | [PliniusCommon](https://github.com/vasic-digital/PliniusCommon) / [GitLab](https://gitlab.com/vasic-digital/PliniusCommon) | [link](2026-04-21-elder-plinius-phaseA-go-plinius-common.md) | 4d / 2wk | Common types/errors library consumed by the other 8 |
| go-gandalf-solutions | [GandalfSolutions](https://github.com/vasic-digital/GandalfSolutions) / [GitLab](https://gitlab.com/vasic-digital/GandalfSolutions) | [link](2026-04-21-elder-plinius-phaseA-go-gandalf-solutions.md) | 4d / 2wk | Read-only solutions archive for prompt-leak-defense testing |
| go-autotemp | [AutoTemp](https://github.com/vasic-digital/AutoTemp) / [GitLab](https://gitlab.com/vasic-digital/AutoTemp) | [link](2026-04-21-elder-plinius-phaseA-go-autotemp.md) | 4d / 2wk | Benchmark-driven temperature auto-tuning |
| go-hypertune | [HyperTune](https://github.com/vasic-digital/HyperTune) / [GitLab](https://gitlab.com/vasic-digital/HyperTune) | [link](2026-04-21-elder-plinius-phaseA-go-hypertune.md) | 4d / 2wk | Hyperparameter tuning orchestration |
| go-i-llm | [I-LLM](https://github.com/vasic-digital/I-LLM) / [GitLab](https://gitlab.com/vasic-digital/I-LLM) | [link](2026-04-21-elder-plinius-phaseA-go-i-llm.md) | 4d / 2wk | Introspection layer for LLM providers |
| go-v3r1t4s | [Veritas](https://github.com/vasic-digital/Veritas) / [GitLab](https://gitlab.com/vasic-digital/Veritas) | [link](2026-04-21-elder-plinius-phaseA-go-v3r1t4s.md) | 4d / 2wk | Truth/verification auxiliary |
| go-leakhub | [LeakHub](https://github.com/vasic-digital/LeakHub) / [GitLab](https://gitlab.com/vasic-digital/LeakHub) | [link](2026-04-21-elder-plinius-phaseA-go-leakhub.md) | 4d / 2wk | Prompt-leak corpus for DeepTeamRedTeamer (defensive fixtures) |
| go-cl4r1t4s | [Claritas](https://github.com/vasic-digital/Claritas) / [GitLab](https://gitlab.com/vasic-digital/Claritas) | [link](2026-04-21-elder-plinius-phaseA-go-cl4r1t4s.md) | 4d / 2wk | System-prompt extraction detection |
| go-ourobopus | [Ouroborous](https://github.com/vasic-digital/Ouroborous) / [GitLab](https://gitlab.com/vasic-digital/Ouroborous) | [link](2026-04-21-elder-plinius-phaseA-go-ourobopus.md) | 4d / 2wk | Recursive/self-referential safety patterns |

### Publication update (2026-04-21)

The 9 scaffolds were published as public `vasic-digital` submodules
(GitHub + GitLab) after explicit user approval. Each repo ships with
a prominent WIP / `ErrCodeUnimplemented` banner in its README —
compilation succeeds (`go build ./...` exit 0) but runtime bodies are
stubs. Module paths use `digital.vasic.<clean>` following the Normalize
and RedTeam precedent; inter-module dependencies use relative
`replace` directives into sibling repos. The HelixAgent root consumes
them as top-level git submodules at `PliniusCommon/`, `AutoTemp/`,
`HyperTune/`, `I-LLM/`, `Veritas/`, `LeakHub/`, `Claritas/`,
`Ouroborous/`, `GandalfSolutions/` (sibling to `Normalize/`,
`RedTeam/`). Phase-A re-implementation work remains GATED per
§Approval gate.

Total commitment if all 9 approved: **~36 person-days core surface
/ ~18 person-weeks full-spec**.

## Workflow per module (once approved)

1. Run `superpowers:brainstorming` against the Python upstream to
   pick behavioral surface. DO NOT copy signatures from the v3 Go
   codegen scaffold — semantic bugs contaminated the type
   signatures (see `docs/research/go-elder-plinius-v3_triage.md`).
2. Produce Go API signatures derived from the Python source.
3. TDD-implement core surface with 100% test coverage per CLAUDE.md §1.
4. Commit + push Phase-A implementation directly to the public
   `vasic-digital/<CleanName>` repo (GitHub + GitLab mirror). Update
   the top-level submodule pointer in HelixAgent.
5. Documentation: CLAUDE.md + AGENTS.md + README.md + docs/ per
   CLAUDE.md §7. Replace the WIP banner in README.md with a normal
   status block once Phase-A core surface is complete.

**PUBLIC REPOS EXIST** (scaffolds only). The Bucket-3a
"no-public-repo" policy was explicitly overridden by user directive
on 2026-04-21 for these 9 defensible-subset modules. Phase-A code
authorship is still gated per §Approval gate.

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
