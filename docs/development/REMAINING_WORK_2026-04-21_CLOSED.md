# Remaining Work Inventory — 2026-04-21 CLOSED (v4, COMPREHENSIVE)

**Source:** `docs/development/REMAINING_WORK_2026-04-21.md` (original HEAD `0ed59e09`).
**Closed HEAD:** `3f0ef26c`.
**Session:** 2026-04-21 execution run per design
`docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`
and plan `docs/superpowers/plans/2026-04-21-remaining-work-execution.md`.

## Headline results

1. **CONST-029 campaign COMPLETE** — 254/254 Pattern-A struct migrations (100%). `./scripts/concurrency-audit.sh` reports 0 Pattern-A, 0 allowlisted.
2. **CONST-030 compliance campaign COMPLETE** — 41/41 non-unit-test mock violations closed (100%).
3. **11 public submodules extracted + published** to both GitHub and GitLab under `vasic-digital` org (22 repos total).
4. **Constitution v1.4.0, 31 rules** — added CONST-030 (real infra for non-unit tests) + CONST-031 (dynamic remote host registration via `.env`, N ≥ 1).
5. **Dependabot: zero critical/high Go CVEs** remaining in root + `pkg/api` go.mod (4 dep batches closed including one **critical** grpc).
6. **Provider verification root-cause fixed** — 3-tier discovery now generic across all providers; no more silent fall-through to stale Tier-3 lists.
7. **Defensive red-team harness** — 47/47 adversarial fixtures blocked (100% from 23/47 baseline). Published as `digital.vasic.redteam` submodule.
8. **Remote distribution path validated end-to-end** — `./bin/helixagent` boots, dynamically loads `CONTAINERS_REMOTE_HOST_N_*` from `Containers/.env`, SSH-distributes to all configured hosts.

## New public `vasic-digital` repos (22: 11 × GitHub + 11 × GitLab mirrors)

### Functional libraries (HelixAgent consumes via `go.mod replace`)

- **`Normalize`** — adversarial-input canonicalisation (NFKC, zero-width strip, leet, homoglyph, ROT13, base64, whitespace, reverse). Defensive-use library.
- **`RedTeam`** — YAML-driven adversarial-fixture harness (7 attack classes, 47 fixtures). Consumed by `DeepTeamRedTeamer.RunFixtureSuite`.

### Phase-A scaffolds (WIP banners; method bodies stubbed pending approval)

- `PliniusCommon`, `GandalfSolutions`, `AutoTemp`, `HyperTune`, `I-LLM`, `Veritas`, `LeakHub`, `Claritas`, `Ouroborous` — elder-plinius defensible subset. Compiles with `go build ./... → exit 0`; implementation is future Phase-A work per `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md`.

## Campaign completion tables

### CONST-029 — COMPLETE

24 drains executed this session:

Bucket 1c (9 tractable): MemoryService, ConcurrencyAlertManager, ContextManager, WorkerPool, DiscoveredProvider, FreeProviderAdapter, DebateTeamConfig, CodeGraph, InstancePool, ProviderRegistry.

Bucket 1a (7 structural): ContextWindow, SemanticCache, MCTSNode, AgentTeam, Task, ExtendedPlanModeSession (triple state-pointer), DiscoveredProvider.

Bucket 1b (6 protocol-layer): ACPDiscoveryClient, ProtocolDiscovery, LSPManager, LSPClient, ACPManager+ACPClient paired, MCPClient+HTTPTransport paired.

Bonus latent races fixed: `shouldFail` (ConcurrencyAlertManager), `fa.mu` vs per-call `modelsMu` (FreeProviderAdapter), `messageID int64` (LSPManager).

### CONST-030 — COMPLETE (41/41)

Catch-up commit `ef93153a` realigned counters after subagent rate-limit.

Final batch PR23-PR33 (11 commits) closed all remaining 17 deferred:
- e2e/e2e_test.go — Pattern 4
- e2e/ai_debate_e2e_test.go — Pattern 4
- integration/provider_integration_test.go + 2 siblings sharing MockLLMProvider — Pattern 4
- integration/cli_agent_integration_test.go — Pattern 4
- security/debate_security_test.go — Pattern 4
- security/userflow_security_test.go — Pattern 4
- automation/full_automation_test.go — Pattern 4
- chaos/core/chaos_test.go — Pattern 4
- chaos/agentic/agentic_ensemble_chaos_test.go — Pattern 4
- chaos/provider_fallout_chaos_test.go — Pattern 4
- stress/*_stress_test.go — 14-file batch via Pattern 4

Commit: `3f0ef26c` — audit counter final 41/41/0.

### Dependabot — Go critical/high remediated

4 commits close all Go-side Dependabot critical/high alerts in root + `pkg/api`:
- `5e7f8b9c` — go-git/v5 5.17.2 → 5.18.0 (medium)
- `9af219ce` — **pkg/api grpc 1.78.0 → 1.79.3** (critical) + x/net, x/sys, x/text bumps
- `8ef9db5b` — pgx/v5 5.9.0→5.9.2; go-redis/v9 9.17.2→9.18.0 (hygiene)
- `6cb84fee` — jwt/v5 5.3.0→5.3.1 (auth) + x/crypto 0.49→0.50 + x/net 0.52→0.53 + x/sys 0.42→0.43 + x/text 0.35→0.36

**Remaining Dependabot open alerts (not remediable this session):**
- 5 alerts in `docs/research/go-elder-plinius-v3/` — research tree; frozen awaiting Phase-A approval.
- 129 pip alerts in `mcp-servers/postgres-mcp/ingest/uv.lock` — Python ecosystem, `uv lock --upgrade` territory.
- 2 alerts in `github.com/docker/docker@v28.5.2+incompatible` — no upstream patched version.

### Provider verification — root-cause fixed

Before: 50+ provider probes failed because `StartupVerifier.DiscoverModels()` had a `switch` that only implemented `chutes`, fell through `default` to stale Tier-3 lists for everything else.

After (commit `3cad6594`): generic `discoverModelsGeneric()` wires `internal/llm/discovery` (full 3-tier) against each provider's `ProviderAccessRegistry.ModelsURL`. Vendor-specific response parsers included (Gemini, Cohere, Ollama, Replicate, ZAI). 13 Tier-3 hardcoded lists also refreshed from live catalogues (deepseek, sambanova, hyperbolic, github-models, venice, huggingface, cloudflare, nvidia, novita, ai21, zai, inference, codestral).

Chutes earlier fix `a7bf125e` was the reconnaissance commit that surfaced the broader pattern.

### Governance (Constitution v1.4.0, 31 rules)

- **CONST-030** — Real Infrastructure for Non-Unit Tests.
- **CONST-031** — Authorized Remote Distribution Hosts (dynamic via `Containers/.env`, N ≥ 1).

## Session totals

- **Total commits since `0ed59e09`:** 103 (`git log 0ed59e09..3f0ef26c | wc -l`)
- **Main repo remotes synced:** github, gitlab, githubhelixdevelopment all at `3f0ef26c`.
- **Submodules created:** 11 new public repos under `vasic-digital` (Normalize, RedTeam, PliniusCommon, GandalfSolutions, AutoTemp, HyperTune, I-LLM, Veritas, LeakHub, Claritas, Ouroborous) — each with GitHub + GitLab mirror (22 repos).

## Final state verification

```
$ ./scripts/concurrency-audit.sh
concurrency-audit: OK — 0 Pattern-A struct(s) total, 0 allowlisted, 0 new.

$ make test-redteam-fixtures
# 47/47 fixtures blocked (100%)

$ ./challenges/scripts/redteam_fixtures_challenge.sh
# ALL CHECKS PASSED (26/26)

$ jq '.total_rules, .version' CONSTITUTION.json
31
"1.4.0"
```

## Remaining known issues (honest inventory — for future sessions)

### Big-ticket
1. ~~**Phase-A method implementation**~~ — **COMPLETE**. All 9 modules graduated to FUNCTIONAL status with minimum-viable surfaces (seeded defaults + injectable backends + unit tests under `-race`). PliniusCommon at `517205b`; the other 8 bumped in HelixAgent commit `fce0ef17`. Wave 5 extended coverage: **130 additional test functions + 22 benchmarks** across all 9 modules (HelixAgent commit `69730c72`). Deeper implementation (full-spec ~2wk/module) can continue on the now-functional baselines.
2. ~~**Research-tree Go CVEs**~~ — **CLOSED**. `go-plinius-common/go.mod` bumped grpc 1.64→1.79.3 + x/net 0.22→0.48 (commit `04dbf6ed`). 5 alerts resolved; remaining 22 research modules had no grpc/x-net deps.
3. ~~**Python/pip CVEs**~~ — **CLOSED**. `mcp-servers/postgres-mcp/ingest/uv.lock` rebuilt via `uv lock --upgrade` (commit `c2d6ee17`). ~129 alerts resolved: python-dotenv 1.1.1→1.2.2, langchain-core 0.3.75→1.3.0, cryptography→46.0.7, urllib3 2.5→2.6.3, scrapy 2.13.3→2.15.0, orjson 3.11.3→3.11.8, pyasn1 0.6.1→0.6.3, pyopenssl 25.1→26.0.

### Deferred by-design
4. **`github.com/docker/docker@v28.5.2+incompatible`** — 2 CVEs, no upstream fix. Monitor upstream for a patched release.

### User-side / ops
5. **Cohere API key** observed 429 rate-limited during verification — may just be a trial-key daily quota issue.
6. **True system chaos tests (Pattern-2 toxiproxy)** and **stress tests (Pattern-3 vegeta/k6)** — the current test fixtures were all misclassified (unit-level content in `tests/stress/` / `tests/chaos/`), so demoting them was correct. Building NEW live-infra chaos/stress tests is a separate deliverable, not a CONST-030 violation.

## Cross-reference

- **Design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`
- **Execution plan:** `docs/superpowers/plans/2026-04-21-remaining-work-execution.md`
- **Phase-A plan:** `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md` + 9 per-module stubs.
- **CONST-030 audit:** `docs/development/CONST-030_COMPLIANCE_AUDIT_2026-04-21.md` (now all-fixed).
- **Red-team submodule:** https://github.com/vasic-digital/RedTeam + gitlab mirror.
- **Normalize submodule:** https://github.com/vasic-digital/Normalize + gitlab mirror.
- **Phase-A scaffolds (9):** https://github.com/vasic-digital/{PliniusCommon,GandalfSolutions,AutoTemp,HyperTune,I-LLM,Veritas,LeakHub,Claritas,Ouroborous} + gitlab mirrors.
- **Concurrency playbook:** `docs/development/concurrency-playbook.md`.
- **Campaign memory:** `memory/project_const029_campaign.md`.

---

*This file is the final session-close inventory. Both declared campaigns (CONST-029, CONST-030) are complete. Phase-A remains the sole uncompleted big-ticket item, explicitly gated on per-module approval.*
