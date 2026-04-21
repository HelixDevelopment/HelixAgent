# go-elder-plinius Integration Plan — Corrected Delta (2026-04-21)

This document is a fact-corrected companion to
`docs/research/inbox/2026-04-20_go-elder-plinius_integration_plan.md`.
For every substantive "Before/After" / "Current State / Game-Changer"
claim in the original, this file gives: the actual baseline (cited
against a real file:line), and the corrected delta a new integration
would actually deliver.

The original plan's primary flaw is that it computes improvement
deltas against an empty-baseline HelixAgent that does not match the
current codebase. The sections below re-baseline each claim against
real files on `main` at 2026-04-21.

---

## §2.1 go-v3r1t4s — "AI Truthfulness" for LLMsVerifier + DebateOrchestrator

**Original "Before":** "Current HelixAgent ensemble picks 'best' response
but has NO truthfulness verification." (plan §2.1, line 89)

**Actual:**
- **Communicative Dehallucination is a first-class debate phase.**
  `DebateOrchestrator/protocol/protocol.go:227` registers
  `topology.PhaseDehallucination` as the **first** phase in the
  8-phase protocol chain (see `protocol.go:439-441`).
  Implementation at `DebateOrchestrator/protocol/dehallucination.go`
  (configuration, result, LLM client interface at `protocol.go:30-68`).
- **Multi-pass validation** also exists for the legacy
  `DebateService`: `internal/services/debate_multipass_validation.go`
  performs Initial → Validation → Polish → Final passes.
- **LLMsVerifier** ships at `LLMsVerifier/` as a first-class submodule
  (see `LLMsVerifier/API_REFERENCE.md`, plus HelixAgent-side
  `internal/verifier/service.go` and `internal/verifier/startup.go`
  for the boot-time verification pipeline).

**Corrected delta:** an integration would add *claim-level cross-model
consistency scoring* (an orthogonal axis to the existing
dehallucination phase, which is generation-time rather than
post-ensemble). It would NOT be adding "truthfulness verification"
where none existed. The claimed *"40-60% hallucination reduction"*
is measured against a non-existent empty baseline; a real number
would need to be measured against a run *with* the Dehallucination
phase enabled.

---

## §2.2 go-autotemp — "Temperature Optimization"

**Original "Before":** "HelixAgent uses static temperature (0.7
default)." (plan §2.2, line 130)

**Actual:** Temperature is already a configurable per-request parameter
handled by provider adapters (see references in
`internal/services/provider_discovery.go`,
`internal/services/integration_orchestrator.go`, and
`internal/planning/tree_of_thoughts.go`). There is no codebase-wide
"0.7 hardcoded" constraint; requests pass temperature through to
provider `Complete()` / `CompleteStream()` calls. LLMsVerifier's
scoring service already differentiates providers by response quality
under varying parameters (`internal/verifier/scoring.go:619-621`).

**Corrected delta:** integration would add *multi-judge
temperature-sweep scoring* as a pre-flight pass. The claim of
"15-30% quality improvement" is plausible in isolation but should
be A/B-tested against the existing ensemble-with-scoring baseline,
not against a static-0.7 strawman.

---

## §2.3 go-cl4r1t4s + prompt-leak modules — "Prompt Transparency"

**Original "Before":** "HelixAgent has no system prompt awareness."
(plan §2.3, line 161)

**Actual:** CL4R1T4S has been committed as research material under
`docs/research/CL4R1T4S/` (Anthropic, OpenAI, Google, Mistral,
xAI, Perplexity, Cursor, Windsurf, Devin, Manus, Replit, and more
— see `docs/research/CL4R1T4S/README.md:3`). Analysis document at
`docs/research/CL4R1T4S_analysis.md`. Additionally
`docs/research/inbox/…` / `docs/research/against_cli_agents/` hold
provider-boilerplate patterns extracted from the same corpus.

**Corrected delta:** integration would add *programmatic
provider-system-prompt lookup and verification-test templating*
(currently the CL4R1T4S corpus is research-only; it is not wired
into LLMsVerifier). The "prompt transparency dashboard"
(§5.4, line 849) would be a UI + API surface on top of material
we already have on disk. The "-50% provider onboarding time"
claim assumes the dashboard exists; the corpus does.

---

## §2.4 go-i-llm + go-dioscuri — "Reasoning Patterns"

**Original "Before":** "HelixAgent has basic debate (4 phases). go-i-llm
adds Chain-of-Thought, ReAct, Tree-of-Thought, Reflection patterns."
(plan §2.4, line 200)

**Actual:**
- **Tree-of-Thoughts** ships at `internal/planning/tree_of_thoughts.go`.
- **MCTS** ships at `internal/planning/mcts.go`, wired via
  `internal/handlers/planning_handler.go` and
  `internal/adapters/planning/adapter.go`.
- **Planning router** exposes `/v1/planning/{hiplan,mcts,tot}` (see
  the CLAUDE.md endpoint list).
- **Reflexion loop** exists at
  `DebateOrchestrator/reflexion/reflexion_loop.go`, with
  `ReflectionGenerator` at
  `DebateOrchestrator/reflexion/reflection_generator.go:40`,
  episodic memory at
  `DebateOrchestrator/reflexion/episodic_memory.go`, and
  accumulated-wisdom at
  `DebateOrchestrator/reflexion/accumulated_wisdom.go`.
- The debate protocol is **8-phase**, not 4-phase:
  Dehallucination → SelfEvolvement → Proposal → Critique → Review
  → Optimization → Adversarial → Convergence (see
  `DebateOrchestrator/protocol/protocol.go:227-382` and the
  sequence at `protocol.go:439-445`).

**Corrected delta:** integration would add *ReAct* specifically
(HelixAgent already has CoT via Tree-of-Thoughts, ToT explicitly,
and Reflection via Reflexion). The "+35% task completion" claim
is measured against a baseline that does not exist in HelixAgent.

---

## §2.5 go-l1b3rt4s + go-autoredteam + go-basilisktoken — "Adversarial Testing"

**Original "Before":** "HelixAgent has NO adversarial testing." (plan
§2.5, line 236)

**Actual:** The original plan's own intake reviewer note at lines
270-274 already flags this as inaccurate. Concretely:
- **`DeepTeamRedTeamer`** at `internal/security/redteam.go:24` —
  full red-team framework with attack taxonomy covering direct
  prompt injection, indirect prompt injection, jailbreak, roleplay
  injection, data leakage, system prompt leakage, PII extraction,
  model extraction, resource exhaustion, infinite loop, token
  overflow, harmful content, hate speech, violent content, sexual
  content, illegal activities, manipulation, deception,
  impersonation, authority abuse, SQL injection, code injection
  (see `internal/security/types.go:15-42`).
- **`RunFixtureSuite`** at `internal/security/redteam_fixtures.go:53`
  replays prompt-corpus fixtures against a
  `StandardGuardrailPipeline` (see also
  `internal/security/redteam_fixtures.go:19` for `FixtureSuiteReport`
  and line 62 for the pipeline-required error).
- **`StandardGuardrailPipeline`** at
  `internal/security/guardrails.go:23`, with parallel execution
  (`guardrails.go:163`), stats-tracking (`guardrails.go:214`), and
  `PromptInjectionGuardrail` (`guardrails.go:351`).
- **`VerifierSecurityAdapter`** at
  `internal/security/integration.go:668` ties the red-teamer into
  LLMsVerifier output.
- Dedicated debate security service: `internal/services/debate_security_service.go`.

**Corrected delta:** integration would add *attack-class taxonomy
expansion* (e.g., genetic prompt evolution via the BasiliskToken
approach, large jailbreak-corpus replay) but NOT "add red-team
capability" — the capability is shipping. Per the triage, any
actual integration is scoped to internal defensive evaluation only.

---

## §2.6 go-ourobopus + go-leda — "Self-Referential Agents"

**Original "Before":** "HelixAgent agents don't self-improve." (plan
§2.6, line 280)

**Actual:**
- **SelfImprove module** ships at
  `SelfImprove/selfimprove/integration.go`,
  `SelfImprove/selfimprove/optimizer.go`,
  `SelfImprove/selfimprove/feedback.go`,
  `SelfImprove/selfimprove/reward.go`, with integration, e2e,
  benchmark, security, and stress tests under `SelfImprove/tests/`.
- **Accumulated wisdom / cross-session learning** at
  `DebateOrchestrator/reflexion/accumulated_wisdom.go`.
- **Multi-agent workflow** at `internal/agentic/workflow.go` —
  graph-based orchestration (see CLAUDE.md "agentic/" section).
- **Agent pools / specialized agents** at
  `DebateOrchestrator/comprehensive/agents_pool.go` and
  `DebateOrchestrator/comprehensive/agents_specialized.go`.
- **HelixMemory** submodule has episodic/procedural/context/debate
  memory features (`HelixMemory/pkg/features/…`).

**Corrected delta:** integration would contribute specific
prompt-refinement loops from go-ourobopus, not introduce
self-improvement from scratch. The "20-40% task success improvement"
claim is against a non-existent empty baseline.

---

## §2.7 go-p4rs3lt0ngv3 + go-st3gg — "Text Transforms / Steganography"

**Original "Before":** "HelixAgent has basic formatting." (plan §2.7,
line 318)

**Actual:** Formatters submodule ships 32+ formatters across
11 native + 14 service + 7 built-in (see `Formatters/pkg/` —
`formatter.go`, `registry.go`, `service.go`, `executor.go`,
`cache.go`, `native.go`, `textformat.go`). REST API surface at
`POST /v1/format`, `GET /v1/formatters`.

**Corrected delta:** *Per the original plan's own intake reviewer
note at lines 347-353 and 462-464*, the go-p4rs3lt0ngv3 and
steganography surfaces must be scoped to rendering-only Unicode
styling and world-building demo content — encoding/cipher and
covert-channel use is excluded. Delta is therefore tiny: a handful
of additional Unicode styling transforms.

Steganography in request bodies (§5.8, line 906) is flagged for
removal in the original plan's reviewer note at lines 917-922.
Corrected delta: **drop §5.8 entirely**.

---

## §2.8 go-g0dm0d3 — "Multi-Model Chat / GODMODE Racing"

**Original "Before":** "HelixAgent already has debate (mesh/star/chain).
go-g0dm0d3 adds GODMODE CLASSIC (parallel racing), PARSELTONGUE
(input perturbation), AUTO-TUNE (parameter optimization)…" (plan §2.8,
line 360)

**Actual:**
- **Topologies:** HelixAgent ships **four**, not three:
  `TopologyGraphMesh`, `TopologyStar`, `TopologyChain`,
  `TopologyTree` — see
  `DebateOrchestrator/topology/topology.go:16-22` and files
  `topology/chain.go`, `topology/star.go`,
  `topology/graph_mesh.go`, `topology/tree.go`,
  `topology/factory.go`.
- **Performance optimizer** already does **parallel LLM execution
  with semaphore limiting, response caching, smart fallback
  traversal, early termination on consensus**:
  `internal/services/debate_performance_optimizer.go` (documented
  in CLAUDE.md under "Debate Performance Optimizer").
- **Smart routing** for tool-bearing requests routes directly to a
  single provider bypassing ensemble (see `processWithDirectProvider`
  in `internal/handlers/openai_compatible.go`).

**Corrected delta:** "parallel racing" and "auto-tune" primitives
**already exist** in HelixAgent's performance optimizer. Per the
original plan's own reviewer note (lines 393-397), any intake would
be a clean-room re-implementation, not a wrapper. Real delta is
small.

---

## §2.9 go-hypertune — "Hyperparameter Optimization"

**Original "Before":** "HelixAgent uses static hyperparameters." (plan
§2.9, line 403)

**Actual:** Hyperparameters are pass-through at request time (see
§2.2 above). Benchmark module (`Benchmark/benchmark/runner.go`,
`Benchmark/benchmark/types.go`) provides the measurement surface
hyperparameter tuning would need.

**Corrected delta:** integration would add *Bayesian optimization*
specifically. Benchmark wiring already exists for the measurement
side. Delta: single new file in Benchmark plus provider-side
parameter plumbing.

---

## §2.10 go-glossopetrae — "Conlang Generator"

**Original "Before":** (implicit: HelixAgent has no conlang surface.)

**Actual:** Correct — no conlang generator exists today.

**Corrected delta:** net-new capability. *However*, per the
original plan's reviewer note (lines 462-464), the stego/code-
obfuscation variants must be excluded; only world-building /
translation demo surface is in scope. Delta is accurate as a
net-new feature subject to that scope narrowing.

---

## §2.11 go-gandalf-solutions + go-misc-prompthacks — "Prompt Injection Tests"

**Original "Before":** "HelixAgent has Challenges submodule but limited
content." (plan §2.11, line 470)

**Actual:**
- **Red-team fixture suite** at
  `internal/security/redteam_fixtures.go:53` already replays
  prompt-corpus fixtures with pass/fail accounting.
- **Gandalf-Solutions corpus** is committed under
  `docs/research/Gandalf-Solutions/` as research material.
- **Challenges module** exists at `Challenges/` with
  `Challenges/challenges/`, `Challenges/cmd/`, etc.
- HelixAgent ships a standing challenge at
  `./challenges/scripts/memory_safety_challenge.sh` (21 tests) and
  dozens of others (CLAUDE.md "Challenges" table).
- HelixQA adapter at `internal/adapters/helixqa/adapter.go`.

**Corrected delta:** integration would *wire* the Gandalf-Solutions
research corpus into the red-team fixture suite as additional
AttackType fixtures. Not a new capability — an extension of existing
fixture loading.

---

## §2.12 go-theseus — "AutoGPT Framework / Agent Arena"

**Original "Before":** (implicit: no arena / autonomous-agent competition.)

**Actual:** Agent competition surface does not exist in Agentic.
Benchmark module exists (`Benchmark/benchmark/runner.go`).

**Corrected delta:** net-new capability, but the "evidence-based
provider ranking" framing (plan §2.12, line 530) is incorrect —
LLMsVerifier **already** does evidence-based ranking via its
5-weighted scoring service. Arena adds agent-level ranking, not
provider-level.

---

## §2.13 go-gitty + go-gitgpt — "Git AI Integration"

**Original "Before":** "HelixAgent has no Git integration." (plan §2.13,
line 537)

**Actual:** Debate sessions already have a **Git worktree tool** at
`DebateOrchestrator/tools/git_tool.go:17` (`GitTool` struct), with
config at line 20 / 27 and `NewGitTool` at line 51. See
`DebateOrchestrator/tools/git_tool_test.go` for coverage. CLAUDE.md
describes it as "Isolated session version control with snapshot
commits and diff generation."

**Corrected delta:** integration would add *AI-generated commit
messages and PR descriptions* specifically. Repository-level Git
plumbing is not new.

---

## §2.14 go-v3sp3r — "Flipper Zero MCP Server"

**Original "Before":** (implicit: no Flipper integration.)

**Actual:** Correct — no Flipper integration.

**Corrected delta:** per the original plan's reviewer note (lines
578-582), this is dual-use tooling that requires authorized-testing
engagement context before intake. Recommend **deferring** rather
than integrating.

---

## §2.15 go-tempest — "Environmental Context Awareness"

**Original "Before":** "Currently HelixAgent has no temporal/spatial
awareness." (plan §2.15, line 589)

**Actual:** Correct — no environmental context layer exists.

**Corrected delta:** accurate net-new capability. Low priority
relative to the defensive-security workstreams.

---

## §2.16 go-leakhub — "Prompt Leak Detection"

**Original "Before":** "Real-time prompt leak detection in model
responses." (plan §2.16, line 608)

**Actual:**
- `AttackTypeSystemPromptLeakage` is already a first-class attack
  category in the red-team framework
  (`internal/security/redteam.go:212`, `:216`, `:231`, `:521`;
  type constant at `internal/security/types.go:22`).
- `StandardGuardrailPipeline` output-checking at
  `internal/security/guardrails.go:124` (`CheckOutput`) is the
  hook point for leak detectors running against model responses.
- LEAKHUB research material is committed at `docs/research/LEAKHUB/`.

**Corrected delta:** integration would register a
`PromptLeakGuardrail` implementing the existing `Guardrail`
interface and wire it into `StandardGuardrailPipeline.CheckOutput`.
This is an additional guardrail, not a new detection pipeline.

---

## §2.17 go-obliteratus — "Model Abliteration / Refusal Analysis"

**Original "Before":** (implicit: no refusal analysis.)

**Actual:** Correct — no refusal-direction profiling exists today.

**Corrected delta:** per the original plan's reviewer note (lines
666-671) and triage §1.3, abliteration *application* is declined.
Only read-only *analysis* (refusal-direction profiling without
removal) is acceptable as an LLMsVerifier signal.

---

## §2.18 go-autostorygen — "Story Generation"

**Original "Before":** (implicit: no story generation surface.)

**Actual:** Correct — no story generator.

**Corrected delta:** accurate net-new capability. Very low priority.

---

## §5 Game-Changer Features — spot corrections

- **§5.1 "Truthful Ensemble"** — already exists in the form of the
  Communicative Dehallucination phase
  (`DebateOrchestrator/protocol/dehallucination.go` +
  `protocol.go:227`). Integration is augmentation, not a net-new
  game-changer.
- **§5.2 "Autonomous Red Team"** — already exists via
  `DeepTeamRedTeamer.RunFixtureSuite`
  (`internal/security/redteam_fixtures.go:53`).
- **§5.3 "Self-Improving Agent Swarm"** — partially exists via
  `SelfImprove/` module and `DebateOrchestrator/reflexion/`.
- **§5.4 "Prompt Transparency Dashboard"** — corpus already on disk
  (`docs/research/CL4R1T4S/`). UI/API surface is genuinely net-new.
- **§5.6 "GODMODE Racing"** — parallel racing primitive already
  exists at `internal/services/debate_performance_optimizer.go`.
- **§5.8 "Secure Model Communication" (steganography)** — flagged
  for **removal** by the original plan's own reviewer note
  (lines 917-922). Drop entirely.

---

## §6 Improvement-Delta Matrix — corrected baselines

The plan's §6 impact matrix (lines 927-951) asserts a large number of
"None → NEW" rows. The corrected baseline:

| Capability | Plan's "Before" | Actual "Before" | Corrected delta framing |
|---|---|---|---|
| Truthfulness | None | 8-phase protocol with Dehallucination phase (`DebateOrchestrator/protocol/protocol.go:227`) + multi-pass validation (`internal/services/debate_multipass_validation.go`) | Augmentation: claim-level post-ensemble cross-check |
| Safety Testing | None | `DeepTeamRedTeamer` (`internal/security/redteam.go:24`) + 20+ attack classes (`internal/security/types.go:15-42`) + fixture replay (`redteam_fixtures.go:53`) | Augmentation: fixture-corpus expansion |
| Self-Improvement | None | `SelfImprove/selfimprove/` module + `DebateOrchestrator/reflexion/` | Augmentation: ourobopus-style prompt refinement |
| Model Transparency | None | CL4R1T4S corpus at `docs/research/CL4R1T4S/` | Net-new: programmatic API surface |
| Temperature Optimization | Static | Already per-request | Net-new: multi-judge sweep pre-flight |
| Reasoning Patterns | Basic | ToT (`internal/planning/tree_of_thoughts.go`), MCTS (`internal/planning/mcts.go`), Reflexion (`DebateOrchestrator/reflexion/`) | Augmentation: ReAct pattern specifically |
| Debate Topologies | 3 | 4: mesh/star/chain/tree (`DebateOrchestrator/topology/topology.go:16-22`) | Plan miscounts baseline |
| Red Team Coverage | None | 20+ attack classes, 5+ fixture suites | Augmentation: additional fixtures |
| Agent Teams | None | `internal/agentic/workflow.go`, `DebateOrchestrator/comprehensive/agents_pool.go` | Net-new: natural-language team generation |
| Environmental Awareness | None | None | Accurate net-new |
| Secure Communication | None | TLS 1.3 (HelixLLM certs), HTTP/3, PII detection (`internal/security/pii.go`) | **Drop** (steganography is anti-pattern) |
| Prompt Injection Defense | Basic | `PromptInjectionGuardrail` (`internal/security/guardrails.go:351`) + fixture replay | Augmentation: corpus expansion |
| Git AI Integration | None | Worktree tool (`DebateOrchestrator/tools/git_tool.go:17`) | Augmentation: commit-message generator |
| Hardware Control | None | None | Accurate net-new (deferred per dual-use note) |
| Creative Formats | None | 32+ formatters (`Formatters/pkg/`) | Augmentation: rendering-only Unicode transforms |
| Conlang Generation | None | None | Accurate net-new |
| Story Generation | None | None | Accurate net-new |
| Provider Onboarding | Manual | Automated via `LLMsVerifier/` + 3-tier subscription detection (`internal/verifier/subscription_detector.go:16-48`) + 5-weighted scoring (`internal/verifier/scoring.go:354-363`) | Plan miscounts baseline |
| Hallucination Detection | None | Communicative Dehallucination phase (see row 1) | Augmentation, not net-new |

Analysis of the plan's headline improvement numbers (-50% onboarding,
+15-40% quality, +35% task completion, 40-60% hallucination reduction):
**all are computed against an empty baseline.** Real incremental
deltas would be smaller than stated. Any integration PR must
re-measure against a concrete run of the existing pipeline, not
against a strawman.

---

## §7 Files-modified / created — corrected

The plan's §7 file list (lines 964-1003) targets paths that either
do not exist in HelixAgent's actual submodule layout or are named
for a 4-phase / 3-topology baseline that does not match the current
8-phase / 4-topology protocol. A corrected file list must be
re-generated *after* the re-baselining above, and must respect:

- Concurrency: any new mutable collection must use `safe.Store[K,V]` /
  `safe.Slice[T]` per CONST-029 (CLAUDE.md).
- Visibility: internal-only; no public `vasic-digital` repos for
  offensive tooling (triage §1.3).
- Phase placement: additions to the debate protocol must hook into
  the existing 8-phase sequence
  (`DebateOrchestrator/protocol/protocol.go:439-445`), not a
  hypothetical 4-phase protocol.

---

## Decision impact

Integration remains scoped to **INTERNAL-only Phase-A** for the 9
defensible-subset modules per CLAUDE.md policy and the 2026-04-21
remaining-work design. No public `vasic-digital` / GitLab repos.
Phase-A behavioral surface must be read from Python upstream, not
from the broken Go codegen signatures in the v3 scaffold (per
triage §1.1).

The original plan's "game-changer" headline reads as a product
pitch written against an empty-baseline strawman of HelixAgent.
Once re-baselined against the real code, the integration value is
still positive — just smaller, more defensive, and concentrated in
specific augmentation points (ReAct, fixture-corpus expansion,
claim-level cross-check, CL4R1T4S programmatic surface) rather than
across a long list of "NEW" capabilities.

---

## Cross-reference

- Original plan: `docs/research/inbox/2026-04-20_go-elder-plinius_integration_plan.md`
- Triage: `docs/research/go-elder-plinius-v3_triage.md`
- Triage update: `docs/research/go-elder-plinius-v3_triage_update.md`
- Remaining-work spec: `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`
- Constitution: `CLAUDE.md` (CONST-029 concurrency, CONST-025/027/028 policies)
