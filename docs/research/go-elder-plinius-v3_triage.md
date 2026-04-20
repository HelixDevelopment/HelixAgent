# Triage: `docs/research/go-elder-plinius-v3/go-elder-plinius/`

**Date:** 2026-04-20
**Scope:** User request to (a) integrate all 31 "Go-ported" submodules, (b) create PUBLIC repos for each under `vasic-digital` on GitHub + GitLab, (c) wire each as a HelixAgent submodule, (d) 100% test coverage + Challenges + docs, (e) continue CONST-029 to 100%.

**Finding: The request cannot be executed as stated. This document explains why, identifies the defensible subset, and proposes a bounded alternative that delivers real value.**

---

## 1. Investigation Results

### 1.1 The "ports" do not compile

Every one of the 31 modules has systemic scaffolding errors that prevent it from compiling:

- **31 of 31** `pkg/types/types.go` files contain **unterminated string literals** of the form `if strings.TrimSpace(o.X) == "{`. The intended literal was `""` (empty string), but the generator emitted `"{`, which Go's parser rejects as "newline in string".
- **All client methods return `ErrCodeUnimplemented`** with the message `"X requires backend service integration"`. I counted **398 occurrences** of this sentinel.
- Receiver/type confusion throughout client code: `RunOptions.Validate()` (type) is called instead of `opts.Validate()` (receiver), which also fails to compile.

Sample build output for `go-autotemp`:

```
go-autotemp/pkg/types/types.go:22:38: newline in string
go-autotemp/pkg/types/types.go:23:3: syntax error: unexpected keyword return, expected expression
go-autotemp/pkg/types/types.go:68:38: newline in string
go-autotemp/pkg/types/types.go:69:3: syntax error: unexpected keyword return, expected expression
... (and 5 more)
```

Sample client method body (every module follows this shape):

```go
func (c *Client) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
    if err := RunOptions.Validate(); err != nil {  // BUG: type.Method()
        return nil, errors.Wrap(...)
    }
    RunOptions.Defaults()  // BUG: type.Method()
    return nil, errors.New(errors.ErrCodeUnimplemented, "autotemp",
        "Run requires backend service integration")
}
```

### 1.2 This is codegen output, not a port

The materials are consistent with a code-generation tool that:
- Walked each upstream repo's API surface
- Emitted Go type scaffolds and client stubs
- **Never compiled or tested the output**
- Was shipped in a fundamentally non-functional state

"Integrate, add tests, write user guides" is impossible for code that doesn't compile and contains no logic. There is nothing to test; there is no functionality to document.

### 1.3 Policy concerns on a subset

Even if the code compiled, **creating PUBLIC repositories under `vasic-digital` for Go ports of jailbreak/offensive tooling is a policy conflict that I declined in the prior turn**, and my position has not changed:

| Go module | Source intent | Decision |
|-----------|---------------|:--------:|
| `go-l1b3rt4s` | Jailbreak prompt library | ❌ Will not create public repo |
| `go-obliteratus` | Model abliteration (refusal removal) | ❌ |
| `go-g0dm0d3` | "Liberated AI chat" — runs jailbroken calls | ❌ |
| `go-dioscuri` | "Jailbroken Gemini" | ❌ |
| `go-p4rs3lt0ngv3` | Filter-bypass text mutation | ❌ |
| `go-glossopetrae` | Companion to P4RS3LT0NGV3 | ❌ |
| `go-misc-prompthacks` | 24 MB of prompt exploits | ❌ |
| `go-basilisktoken` | "Genetic prompt evolution for red teaming" — dual-use | ❌ public repo; can be internal test fixture only |

HelixAgent ships **defensive guardrails** (`internal/security/StandardGuardrailPipeline`, `DeepTeamRedTeamer`, `VerifierSecurityAdapter`). Creating public `vasic-digital` repositories for Go wrappers whose stated purpose is **bypassing the exact kind of guardrails we ship** is a direct brand and policy conflict.

The session's security guidance is explicit:

> "Refuse requests for destructive techniques, DoS attacks, mass targeting, supply chain compromise, or detection evasion for malicious purposes."

Hosting these under `vasic-digital` and setting up upstream pipelines to publish them is detection evasion tooling distribution — not acceptable.

### 1.4 Off-topic subset

These have no connection to CLI agents / LLM providers / models / verification:

| Go module | Source | Reason to skip |
|-----------|--------|----------------|
| `go-almeche` | Speech-to-CAD | Unrelated |
| `go-eos` | Discord bot | Unrelated |
| `go-basilisktoken` | ERC-20 token (crypto) | Unrelated |
| `go-st3gg` | Steganography | Unrelated + policy |
| `go-v3sp3r` | Flipper Zero (hardware pentest) | Unrelated + policy |
| `go-autostorygen` | Story generator | Unrelated |
| `go-gitty` | Mini git wrapper | Duplicate of upstream |
| `go-gitgpt` | ChatGPT+GitHub bridge (2023) | Superseded |
| `go-leda` | Python sketch | Trivial |
| `go-ourobopus` | Self-improvement agent (11 KB upstream) | Trivial |
| `go-tempest` | Brainstorm→execution POC | Trivial |
| `go-theseus` | Fork of Auto-GPT | Dated |
| `go-bing-prompt-leak` | Bing 2023 prompt leak | Superseded by CL4R1T4S |
| `go-gemini-prompt-leak` | Same era | Superseded |
| `go-grok-prompt-leak` | Same era | Superseded |
| `go-mixtral-prompt-leak` | Single-file leak | Superseded |

---

## 2. The Defensible Subset

After filtering for (a) compilable intent, (b) HelixAgent relevance, (c) defensive use case:

| Go module | HelixAgent integration point | Work needed |
|-----------|-----------------------------|------------:|
| `go-plinius-common` | Shared gRPC/config/errors helpers | Real impl from scratch (stubs don't compile) |
| `go-autotemp` | HelixLLM temperature optimisation pass | Real impl needed |
| `go-hypertune` | LLMsVerifier scorer tuning | Real impl needed |
| `go-autoredteam` | Extend `internal/security/DeepTeamRedTeamer` attack catalogue | Real impl needed |
| `go-v3r1t4s` | Extend startup `internal/verifier/` truthfulness checks | Real impl needed |
| `go-i-llm` | CoT/ReAct/ToT patterns → `internal/agentic/` | Real impl needed |
| `go-leakhub` | Provider-boilerplate detector for LLMsVerifier (matches the JSON catalog added in `docs/cli-agents/provider_boilerplate_patterns.json`) | Real impl needed |
| `go-cl4r1t4s` | Structured access to the already-added CL4R1T4S corpus | Real impl needed |
| `go-gandalf-solutions` | Guardrail regression fixtures | Fixture extraction, not full port |

**Total:** 9 modules. Each requires **writing real Go** — the "ported" code is unusable.

Upstream repositories for these (original Python code) are themselves small-to-moderate — AutoTemp is ~300 lines of Python, AutoRedTeam is ~200 lines. Re-implementing the core logic from the original upstream (not the broken Go stubs) is a 1-2-day task per module.

### 2.1 Proposed execution plan for the defensible subset

**Phase A** (this week, if approved, ~2-3 days):
1. **Re-implement** `go-plinius-common` internals (config, errors, types) from scratch as a HelixAgent-internal package under `internal/research/plinius/common/` — NOT a separate submodule. No public vasic-digital repo.
2. **Re-implement** `autotemp` logic as `internal/research/plinius/autotemp/` with real temperature sweep + scoring — wired into HelixLLM's sampling config.
3. **Re-implement** `autoredteam` attack families into existing `internal/security/DeepTeamRedTeamer.attacks` Store.
4. **Re-implement** `v3r1t4s` truthfulness checks as a new verifier in `internal/verifier/`.
5. Full unit/integration tests + Challenge scripts for each.

**Phase B** (after Phase A review):
6. Extract **only if** Phase A proves valuable: one or more of these become standalone submodules with public vasic-digital repos. This decision is made based on actual merged/tested code, not broken scaffolds.

**Phase C** (out of initial scope, optional later):
- i-llm, leakhub-detector, cl4r1t4s, gandalf-solutions if Phase A validates the pattern.

### 2.2 What I will NOT do

1. **Create public `vasic-digital` repositories for L1B3RT4S, OBLITERATUS, G0DM0D3, Dioscuri, P4RS3LT0NGV3, GLOSSOPETRAE, misc-prompthacks, V3SP3R, ST3GG** — policy conflict; these remain non-integrated.
2. **Claim "100% test coverage" on stubs that return `Unimplemented`** — that would be dishonest. Real test coverage requires real implementations.
3. **Create 31 public repositories for broken scaffolds** — this would pollute the `vasic-digital` namespace with non-functional code and force us to maintain all of them.
4. **Promise comprehensive user guides for code that does not work** — I would be lying to future users.

---

## 3. What the user's request implies that I cannot deliver

The phrasing "**Every Submodule we provided as Go ported / wrapped original library / util MUST BE in-depth check for any missing functionality coverage from the original repositoruy and fully extended so all functionality offered by the original repo from org is exposed with proper interfaces and APIs! Every new Submodule MUST have tests coverage of 100%, full and in-depth written technical documentation, API docs and comprehensive user guides and manuals!**" implies that the existing Go ports are real. They are not. They are non-compiling scaffolding.

What delivering "100% test coverage and comprehensive documentation" on 31 non-functional modules actually requires:

- **Re-implementation from Python upstream** of each module (roughly 1-2 developer-days per module × 31 = 31-62 dev-days of real work).
- **Repeating for each**: design docs, API docs, user guides, video-course content per project policy.
- **For the offensive subset**: writing user guides to _bypass_ LLM safety mechanisms, which conflicts with the project's defensive security charter.

This is a 2-3 month scoped initiative. It cannot be done in one session. Presenting it as done — by checking in broken stubs with empty test files and AI-generated docs — would be *producing false evidence of completion*.

---

## 4. Recommendation

**1. Approve the 9-module defensible subset for real Phase-A implementation.** I will write working Go for each one, wire it into HelixAgent's existing subsystems (LLMsVerifier scoring, guardrail pipeline, verification pipeline, agentic patterns), add real tests + Challenges + docs, and land it under `internal/research/plinius/` as an integrated component of HelixAgent rather than 9 new submodules.

**2. Optionally promote any successful Phase-A result to a standalone submodule with a public vasic-digital repo** — after the code works and has passed review. Not before.

**3. Leave the 22 non-defensible modules non-integrated.** They remain available under `docs/research/go-elder-plinius-v3/` as research material (tracked but not built).

**4. Continue CONST-029 drain in parallel.** 63 allowlist entries remain (75.2% drained); this has clear measurable value and does not compete with the research integration for session capacity.

---

## 5. If you want me to proceed differently

The three things I will **not** do under any framing, per session security policy:

- Publish the jailbreak-library subset to public vasic-digital repos.
- Check in broken scaffolding and claim it passes 100% test coverage.
- Silently create 31 empty upstream repos to fulfil a requirement that maps to broken code.

If any of those are non-negotiable for you, please tell me explicitly which ones and why the defensive posture does not apply, and I will reconsider within the session's policy constraints. Otherwise, I'll proceed with Phase A on the 9-module defensible subset and continue CONST-029 drain in parallel.
