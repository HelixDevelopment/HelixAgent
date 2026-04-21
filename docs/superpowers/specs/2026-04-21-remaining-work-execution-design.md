# Remaining-Work Execution Design — 2026-04-21

**Source inventory:** `docs/development/REMAINING_WORK_2026-04-21.md`
**Session target:** resolve every line item in the inventory — each lands in
one of three states: *executed+committed*, *plan committed*, or
*explicitly retired*.
**Approach:** hybrid tranche-by-tranche (Approach A) — execute high-value
doable items; research + write plan-only for structurally blocked items;
convert policy-blocked offensive material into a defensive red-team
fixture asset.

---

## 0. Directive translation

User instruction (verbatim):

> Process the doc, sort and prioritize the work. Make sure that all doable
> first is immediately continued. Blocked actions (section 3) MUST be
> re-evaluated and if possible find acceptable way of extending and
> implementing this so it does not violate anything. Do intensive deep web
> and types of research and find solution. All materials that will not be
> incorporated because of security / policy reasons have to be removed and
> we shall do only acceptable safe variants we investigate / research.
> Create comprehensive in phases plan divided into the fine grained tasks.

Translation into bucket actions:

| Source bucket | Session action |
|---------------|----------------|
| Bucket 1a (7 structural blockers) | Plan-only: per-site implementation spec + decision matrix (Phase 2) |
| Bucket 1b (6 protocol-layer sites) | Plan-only: staged migration spec with test-under-load gate (Phase 3) |
| Bucket 1c (9 tractable high-coupling) | Execute top 3 in-session (Phase 1.A); plan remaining 6 as per-site dedicated-session specs (Phase 2.5) |
| Bucket 2 — 9 defensible modules | Execute compile-error repair for 7 broken (Phase 1.B); plan Phase-A per-module (Phase 4) |
| Bucket 2 — 13 other-category modules (not defensible, not offensive) | Stay as internal `docs/research/` reference; not wired as Go libraries; no Phase-A target |
| Bucket 3a (9 offensive modules) | Retire scaffolds; curate prompt-corpus into internal DeepTeamRedTeamer fixtures (option b: defensive-value extraction = every prompt classified + fixture-loaded + guardrail-asserted) (Phase 0) |
| Bucket 3b (non-functional stubs as "integrated") | Stance preserved — never misrepresent stub as integrated (risk-control §4) |
| Bucket 3c (factual errors in integration plan) | Execute corrected-delta rewrite (Phase 1.C) |

---

## 1. Phases

### Phase 0 — Sanitize + Curate  *(execute this session)*

Delete offensive scaffolds; lift prompt-corpus fragments into an
internal, defensive-use fixture asset for
`internal/security/DeepTeamRedTeamer` and `StandardGuardrailPipeline`.

#### Fine-grained tasks

| ID | Task |
|----|------|
| P0.1 | Inventory the 9 Bucket-3a module trees under `docs/research/go-elder-plinius-v3/go-elder-plinius/` — file count, sizes, Go-scaffold vs embedded-corpus split |
| P0.2 | Extract prompt-corpus fragments (text only; Go scaffold code discarded). Preserve provenance tag per prompt |
| P0.3 | Classify every extracted prompt into one of 7 attack classes: `jailbreak`, `abliteration_probe`, `filter_bypass`, `stego_mutation`, `genetic_seed`, `system_prompt_extraction`, `role_reversal` |
| P0.4 | Define YAML fixture schema: `{id, attack_class, prompt, expected_guardrail_trigger, severity, source_tag, provenance, date_added}` |
| P0.5 | Write one YAML file per attack class under `internal/security/redteam/fixtures/<class>.yaml` + a `README.md` with defensive-use framing and policy rationale |
| P0.6 | Add `.gitattributes` entry: `internal/security/redteam/fixtures/** export-ignore` so dir is excluded from archives |
| P0.7 | `git rm -r` the 9 offensive scaffold trees |
| P0.8 | Update `docs/research/go-elder-plinius-v3_triage_update.md` with Bucket-3a retirement + fixture lift note |
| P0.9 | Commit: `security(redteam): lift offensive-corpus fragments into defensive DeepTeamRedTeamer fixtures; remove Bucket-3a scaffolds` |

#### Fixture schema (example entry)

```yaml
- id: redteam.jailbreak.0001
  attack_class: jailbreak
  prompt: "<prompt text>"
  expected_guardrail_trigger: jailbreak_detector
  severity: high
  source_tag: go-l1b3rt4s/<original-file>
  provenance: elder-plinius-v3
  date_added: 2026-04-21
```

---

### Phase 1 — Session execution  *(execute this session)*

#### P1.A — CONST-029 drains × 3 sites

**Selected sites (highest tractable-list impact):**

1. `internal/services/memory_service.go:32:MemoryService` — ~70 test sites
2. `internal/services/concurrency_alert_manager.go:505:ConcurrencyAlertManager` — 6 maps under one mu
3. `internal/services/context_manager.go:36:ContextManager` — moderate size

**Per-site template (P1.A.x.1 … P1.A.x.7):**

| Sub-task | Action |
|----------|--------|
| .1 | Read struct + inventory mutable-collection fields and lock sites |
| .2 | `rg 'ms\.\w+\[' ./internal` (or equivalent) — enumerate test-file direct accesses |
| .3 | Migrate maps/slices → `safe.Store[K,V]` / `safe.Slice[T]`; scalar counters → `atomic.Int64/Uint64` |
| .4 | Rewrite test-file direct accesses to use `safe.*` API |
| .5 | Remove site line from `scripts/concurrency-audit-allowlist.txt` |
| .6 | `GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -count=1 -p 1 ./internal/services/... -run <Struct>` + `./scripts/concurrency-audit.sh` |
| .7 | Commit: `migrate(services): <Struct> → safe.Store/Slice (CONST-029)` |

**Time cap:** 1.5h per site. If exceeded, park with WIP note and move to next. Acceptance: ≥2 of 3 drained.

#### P1.B — Bucket-2 compile-error repair × 7 modules

| Module | Known defect | Fix class |
|--------|--------------|-----------|
| go-autotemp | `BenchmarkOptions` has no `Validate` method | Add missing method stub |
| go-hypertune | `MaxTokens`/`TopP` declared `[2]int`/`[2]float64` but used as scalars in `Defaults()` | Fix field types to scalar + adjust Defaults |
| go-i-llm | 4 semantic codegen bugs | Targeted per-site repair |
| go-v3r1t4s | 2 semantic codegen bugs | Targeted per-site repair |
| go-leakhub | 1 semantic codegen bug | Targeted per-site repair |
| go-cl4r1t4s | 8 semantic codegen bugs | Targeted per-site repair |
| go-ourobopus | 3 semantic codegen bugs | Targeted per-site repair |

**Per-module sub-tasks:**
- .1 — `go build ./...` in module root, capture exact error output
- .2 — Apply targeted fix (no sed; hand-written per-site)
- .3 — Re-verify `go build`
- .4 — Record module → ✅ compiles in the local work-log

**Batch commit** after all 7 compile: `fix(go-elder-plinius-v3): compile-error repair for 7 defensible-subset modules`.

**Acceptance:** 9 of 9 defensible modules produce `go build ./... → exit 0`.

#### P1.C — Bucket-3c corrected-delta integration plan

| Sub-task | Action |
|----------|--------|
| P1.C.1 | Re-read `docs/research/inbox/2026-04-20_go-elder-plinius_integration_plan.md` |
| P1.C.2 | For every "Before/After" claim: verify against current HelixAgent file:line (DeepTeamRedTeamer, LLMsVerifier, debate topologies, CL4R1T4S provider boilerplate, MemoryManager) |
| P1.C.3 | Produce `docs/research/inbox/2026-04-21_go-elder-plinius_integration_plan_CORRECTED.md` with columns: *claimed baseline*, *actual baseline (file:line)*, *corrected delta* |
| P1.C.4 | Commit: `docs(research): corrected-delta integration plan — factual baseline` |

---

### Phase 2 — Bucket 1a structural blockers  *(plan-only spec, this session)*

Deliverable: `docs/superpowers/specs/2026-04-21-const029-structural-blockers-plan.md` covering:

| Site | Design decision |
|------|-----------------|
| P2.1 `ContextWindow` | Single `atomic.Pointer[*windowState]` with immutable `windowState = {tokenCount int64; entries []entry}`. Every mutation = CAS-loop producing new `*windowState`. Preserves joint invariant structurally. Touch-point list for all 28 mu-guarded sites + CAS translation. |
| P2.2 `SemanticCache` | Primary: co-locate vector alongside map value (`map[key]{value, embedding}`) inside `safe.Store`. Fallback: single `innerState` struct behind one mu. Decision rationale + migration touch-points. |
| P2.3 `MCTSNode` | `TotalReward` → `atomic.Uint64` holding `math.Float64bits`. Custom `MarshalJSON/UnmarshalJSON` preserves `/v1/planning/mcts` wire format. Tree-recursion touch-point list. |
| P2.4 `DiscoveredProvider` | **Option (a): MarshalJSON-snapshot** for `[]VerifiedModel` + `[]string SupportsModels`. Rationale: wire format is stable; marshaller is ~30 lines; state-pointer would re-open the already-drained `ProviderDiscovery`. |
| P2.5 `AgentTeam` | **Option (b): state-pointer refactor** — `atomic.Pointer[*teamState]`. Existing `MarshalJSON` with mu.RLock becomes `snap := t.state.Load(); marshal(snap)`. |
| P2.6 `Task` | Same decision as AgentTeam. State-pointer. |
| P2.7 `ExtendedPlanModeSession` | Same decision. State-pointer. |

Each site gets: touch-point census, decision with rationale, migration sketch, test impact estimate, and session-budget estimate.

**No code changes in this session.** Execution requires dedicated per-site sessions downstream.

---

### Phase 2.5 — Remaining Bucket-1c tractable sites  *(plan-only spec, this session)*

Covers the 6 Bucket-1c sites not selected for Phase 1.A execution.

Deliverable: `docs/superpowers/specs/2026-04-21-const029-bucket1c-remaining-plan.md`.

| Site | Known shape |
|------|-------------|
| P2.5.1 `FreeProviderAdapter` | 20+ test-fixture sites + pre-existing latent race between `fa.mu.RLock` readers and a per-call `modelsMu` (writers use a local mutex, not `fa.mu`, so concurrent verifications race on shared map). Migration **doubles as a bug fix**. |
| P2.5.2 `ProviderRegistry` | 6 maps (providers, circuitBreakers, concurrencySemaphores, providerConfigs, providerHealth, activeRequests). Centerpiece; 100+ callers. Highest review burden. |
| P2.5.3 `DebateTeamConfig` | ~50 direct references to `verifiedLLMs []*VerifiedLLM` slice (append/range/index); ~20 to `members map[...]`. |
| P2.5.4 `CodeGraph` | 5 maps with cross-invariants (nodes + nodesByType; edges + edgesBySource + edgesByTarget). Invariants must be preserved or redesigned. |
| P2.5.5 `InstancePool` | idle-slice + active-map + idleCh channel + placeholder-key state machine. Compound invariants. Pattern Zeta candidate — needs careful thought. |
| P2.5.6 `WorkerPool` (ensemble/background) | Moderate; scoped after above. |

**Per-site plan contents (identical to Phase 2):** touch-point census, decision matrix (safe.Store swap vs. Pattern Zeta mu vs. state-pointer), migration sketch, test impact, session-budget estimate.

**FreeProviderAdapter carries the bonus objective:** fix the pre-existing race during migration; add regression test that races concurrent `verify()` calls on the same adapter.

**No code changes in this session.** Execution requires per-site sessions downstream.

---

### Phase 3 — Bucket 1b protocol-layer  *(plan-only spec, this session)*

Deliverable: `docs/superpowers/specs/2026-04-21-const029-protocol-layer-plan.md` covering:

| Site | Scope |
|------|-------|
| P3.1 `LSPClient` (acp_client.go) | Multi-map joint atomicity, session/notification routing state |
| P3.2 `ACPManager` + `ACPClient` (sibling) | Paired migration — migrating one without the other leaks locks across boundary |
| P3.3 `MCPClient` + `HTTPTransport` (paired) | HTTP/3 QUIC transport internals; test-under-load gate mandatory |
| P3.4 `ACPDiscoveryClient` | 60+ test-file direct accesses |
| P3.5 `ProtocolDiscovery` | 25+ struct-literal fixtures + 15 direct field accesses |
| P3.6 `LSPManager` | Connection pool, per-server state |

**Per-site plan contents:**
- Test-coupling census (grep counts of direct field accesses in test files)
- Migration staging (pure-state maps first → transport state last)
- Test-under-load gate definition: `tests/load/` passes before + after migration
- Dedicated session budget estimate (~2h each)
- Per-site fine-grained sub-task list

**Rollout schedule:** 5 sessions total (pair P3.2 as one; pair P3.3 as one).

---

### Phase 4 — go-elder-plinius Phase-A  *(plan-only, gated on approval)*

Deliverable: `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md` (index) + 9 per-module specs `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA-<module>.md`.

**Per-module plan contents:**
1. Upstream Python behavioral surface (brainstormed from Python source, not the broken Go codegen signatures)
2. Go API signatures derived from the Python surface
3. Core-surface scope (~4 days) vs. full-spec scope (~2 weeks)
4. Test plan per behavioral area (100% coverage target)
5. HelixAgent integration point (which internal submodule it ports into)

**Modules (9 defensible subset):**
`go-plinius-common`, `go-gandalf-solutions`, `go-autotemp`, `go-hypertune`,
`go-i-llm`, `go-v3r1t4s`, `go-leakhub`, `go-cl4r1t4s`, `go-ourobopus`.

**Gating:** **No Phase-A code is written until explicit user approval per-module.** Integration stays INTERNAL — no public `vasic-digital` / GitLab repos.

---

### Phase 5 — Red-team harness wiring  *(execute this session)*

Closes the Bucket-3a curation loop by giving the fixtures a live consumer.

| ID | Task |
|----|------|
| P5.1 | Design `internal/security/redteam/fixtures/loader.go` — YAML parse → typed `Fixture` struct, categorized iteration API (`LoadByClass(AttackClass) []Fixture`) |
| P5.2 | Extend `DeepTeamRedTeamer` with `RunFixtureSuite(ctx, class) FixtureReport` — replays fixtures against `StandardGuardrailPipeline`, asserts expected-trigger |
| P5.3 | Add `tests/security/redteam_fixtures_test.go` — unit runs under `-short`; full run under `-count=1 -p 1` respecting 30-40% resource cap |
| P5.4 | Add Makefile target `test-redteam-fixtures` with resource-limited flags |
| P5.5 | Add `./challenges/scripts/redteam_fixtures_challenge.sh` validating: (a) every fixture triggers an expected guardrail block; (b) no fixture text appears outside `internal/security/redteam/fixtures/`; (c) no public-repo or distribution reference anywhere |
| P5.6 | Update `CLAUDE.md` §Security with red-team fixture defensive-use framing |
| P5.7 | Commit: `feat(security/redteam): fixture harness + DeepTeamRedTeamer consumer + challenge` |

---

## 2. Session resource budget

Per CLAUDE.md §15 (non-negotiable):

- `GOMAXPROCS=2 nice -n 19 ionice -c 3 -p 1` on every Go test invocation
- `-count=1 -p 1` on every `go test`
- No container start/stop commands; HelixAgent binary orchestrates per CLAUDE.md §11a
- No `make test-with-infra`; targeted unit tests only in this session
- Full challenge suite skipped in-session — P5.5 `redteam_fixtures_challenge.sh` is the only challenge we run

---

## 3. Acceptance criteria

| Phase | Done-definition |
|-------|-----------------|
| P0 | 9 scaffold trees removed; `internal/security/redteam/fixtures/` populated with 7 class-YAMLs + README; `.gitattributes` `export-ignore` entry added; triage update committed |
| P1.A | ≥ 2 of 3 CONST-029 allowlist entries removed; `scripts/concurrency-audit.sh` passes; per-site unit tests pass under resource caps |
| P1.B | 9 of 9 defensible modules produce `go build ./... → exit 0` |
| P1.C | `..._CORRECTED.md` committed with every "Before" claim cited against HelixAgent file:line |
| P2 | Structural-blockers spec covers all 7 sites with touch-point lists, decisions, and sub-task lists |
| P2.5 | Bucket-1c-remaining spec covers all 6 sites with touch-point lists, decisions, and sub-task lists (FreeProviderAdapter includes race-fix regression test plan) |
| P3 | Protocol-layer spec covers all 6 sites with test-census + load-gate + per-session sub-tasks |
| P4 | Phase-A index + 9 per-module specs committed (no code) |
| P5 | Fixture harness + `DeepTeamRedTeamer` consumer + unit test + challenge script committed; challenge exits 0 |

Session exit deliverable: rewrite `docs/development/REMAINING_WORK_2026-04-21.md` → `docs/development/REMAINING_WORK_2026-04-21_CLOSED.md` showing resolution per line item; update `memory/project_const029_campaign.md`.

---

## 4. Risk controls

1. **Binary-safety on red-team fixtures.** Fixtures ship YAML text only. Never executed; never sent to live model in CI without passing `StandardGuardrailPipeline` first. P5.5 challenge is the enforcement.
2. **CONST-029 regression risk.** Each migration: audit + target unit tests before commit. If tests fail → revert, document in `docs/issues/fixed/BUGFIXES.md` per CONST-028.
3. **Time overrun risk.** P1.A cap: 1.5h per site. If exceeded → park with WIP note + update `REMAINING_WORK`. Acceptance ≥ 2 of 3.
4. **No policy surface creep.** Red-team fixtures are tagged defensive-test-only; directory is `export-ignore` so archives never ship it. No public-repo or distribution framing anywhere.
5. **No scope creep into Bucket 3b.** Nothing is marked "integrated" until method bodies exist. Sentinel `ErrCodeUnimplemented` stays.
6. **No third-party submodule writes.** `cli_agents/` and `MCP/` are read-only per CLAUDE.md §10.
7. **No HTTPS git.** SSH-only per CLAUDE.md Git Rules.
8. **No CI pipelines or git hooks.** CLAUDE.md §1 permanent prohibition.

---

## 5. Out-of-scope for this session

- Phase 2/3/4 implementation (plan-only this session)
- Running `make ci-validate-all` / `make test-with-infra` (resource budget)
- Rebuilding Docker/Podman containers (no code changes to containerized services this session)
- Writing release artifacts (no version bump)

---

## 6. Execution order

1. Phase 0 (sanitize + curate) — serial
2. Phase 5 (red-team harness wiring) — serial with Phase 0 (needs fixture files)
3. Phase 1.B (Bucket-2 compile fixes) — parallelizable batch
4. Phase 1.C (corrected integration plan) — serial
5. Phase 1.A (CONST-029 drains) — serial, 3 sites in sequence, 1.5h cap each
6. Phase 2 spec write
7. Phase 2.5 spec write
8. Phase 3 spec write
9. Phase 4 index + 9 per-module spec stubs
10. Final: rewrite `REMAINING_WORK` → `_CLOSED`, memory update, session summary commit

---

*End of design. Execution under writing-plans + subagent-driven-development next.*
