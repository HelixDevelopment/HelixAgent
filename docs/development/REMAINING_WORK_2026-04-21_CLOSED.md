# Remaining Work Inventory — 2026-04-21 CLOSED

**Source:** `docs/development/REMAINING_WORK_2026-04-21.md` (original HEAD `0ed59e09`).
**Closed HEAD:** after Task 16 commit of this session.
**Session:** 2026-04-21 execution run per
`docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`
and `docs/superpowers/plans/2026-04-21-remaining-work-execution.md`.

## Terminal state per line item

Every item from the original inventory now sits in one of three
states: **executed+committed** / **plan committed** / **explicitly retired**.

### Bucket 1a — 7 structural blockers

| Site | Resolution |
|------|------------|
| `internal/optimization/context/window.go:22 ContextWindow` | **plan committed** — `docs/superpowers/specs/2026-04-21-const029-structural-blockers-plan.md` (commit `f5f00014`). Decision: `atomic.Pointer[*windowState]` immutable-state CAS. |
| `internal/optimization/gptcache/semantic_cache.go:50 SemanticCache` | **plan committed** — same spec. Decision: co-locate vector in map value inside `safe.Store`. |
| `internal/planning/mcts.go:27 MCTSNode` | **plan committed** — same spec. Decision: `atomic.Uint64` via `math.Float64bits` + custom MarshalJSON to preserve `/v1/planning/mcts` wire format. |
| `internal/services/provider_discovery.go:79 DiscoveredProvider` | **plan committed** — same spec. Decision: MarshalJSON-snapshot (option a). |
| `internal/handlers/extended/ensemble.go:26 AgentTeam` | **plan committed** — same spec. Decision: state-pointer refactor. |
| `internal/handlers/extended/ensemble.go:84 Task` | **plan committed** — same spec. Decision: state-pointer refactor. |
| `internal/handlers/extended/planning.go:24 ExtendedPlanModeSession` | **plan committed** — same spec. Decision: state-pointer refactor. |

### Bucket 1b — 6 protocol-layer sites

| Site | Resolution |
|------|------------|
| `internal/services/acp_client.go:20 LSPClient` | **plan committed** — `docs/superpowers/specs/2026-04-21-const029-protocol-layer-plan.md` (commit `09999b7a`). |
| `internal/services/acp_manager.go:22 ACPManager` + `:67 ACPClient` | **plan committed** (paired) — same spec. |
| `internal/services/mcp_client.go:20 MCPClient` + `:58 HTTPTransport` | **plan committed** (paired, highest transport risk) — same spec. |
| `internal/services/protocol_discovery.go:19 ACPDiscoveryClient` | **plan committed** — same spec. |
| `internal/services/protocol_federation.go:16 ProtocolDiscovery` | **plan committed** — same spec. |
| `internal/services/lsp_manager.go:22 LSPManager` | **plan committed** — same spec. |

### Bucket 1c — 9 tractable high-coupling

| Site | Resolution |
|------|------------|
| `internal/services/memory_service.go:32 MemoryService` | **EXECUTED** — commit `eb7def26`. Drained: `*safe.Store` + atomic.Bool/Int64/Pointer. All tests pass under `-race`. Allowlist line removed (Task 16). |
| `internal/services/concurrency_alert_manager.go:505 ConcurrencyAlertManager` | **EXECUTED** — commit `1a3f93e2`. 6 maps → safe.Store; cleanupOldEntries restructured to 3 independent Range+Delete passes with advisory-cleanup note. Bonus: pre-existing latent race on `shouldFail` bool fixed with `atomic.Bool`. Allowlist line removed. |
| `internal/services/context_manager.go:36 ContextManager` | **EXECUTED** — commit `afa8785e`. Two maps migrated. Snapshot-based iteration avoids re-entry deadlocks. Allowlist line removed. |
| `internal/verifier/adapters/free_adapter.go:68 FreeProviderAdapter` | **plan committed** — `docs/superpowers/specs/2026-04-21-const029-bucket1c-remaining-plan.md` (commit `ba4bb5af`). Bonus objective spec'd: fix the latent race between `fa.mu.RLock` readers and per-call `modelsMu` writers. |
| `internal/services/provider_registry.go:72 ProviderRegistry` | **plan committed** — same spec. 7 maps; 15 external caller files identified. |
| `internal/services/debate_team_config.go:304 DebateTeamConfig` | **plan committed** — same spec. |
| `internal/knowledge/code_graph.go:124 CodeGraph` | **plan committed** — same spec. Decision: state-pointer for cross-invariant index pairs. |
| `internal/clis/pool.go:13 InstancePool` | **plan committed** — same spec. Decision: **Pattern Zeta** — keep the mu, migrate only sub-state counters. |
| `internal/ensemble/background/worker_pool.go:33 WorkerPool` | **plan committed** — same spec. Quick-win candidate for next session. |

### Bucket 2 — go-elder-plinius integration

| Item | Resolution |
|------|------------|
| 9 defensible modules compile | **EXECUTED** — commit `898e3947` (7 modules fixed) + `285ce618` (go.work cleanup). All 9 produce `go build ./... → exit 0`. |
| Phase-A implementation (398 methods) | **plan committed, GATED** — `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md` + 9 per-module stub specs (commit `36ffc305`). Awaiting explicit per-module approval. |
| 35 integration files + 15 submodules wired | **plan committed, GATED** — covered within Phase-A per-module specs. |
| 100% test coverage per module | **plan committed** — scoped in per-module test-plan sections. |
| 22 non-defensible modules (13 other-category + 9 offensive) | 13 other-category modules stay as `docs/research/` reference (not wired as Go libraries); 9 offensive retired (see Bucket 3a). |
| Public `vasic-digital` repos | **RETIRED** — policy preserved; INTERNAL-only integration per Phase 4 plan. |

### Bucket 3 — policy-declined

| Item | Resolution |
|------|------------|
| 3a — 9 offensive modules | **RETIRED + LIFTED** — scaffolds removed (commit `2e0fbf10`); prompt-corpus fixture harness wired in Phase 5 (commits `492ca2de`, `debaa646`, `6d745b09`, `dcb40520`); fixtures populated with 47 publicly-documented defensive patterns (commit `cce1583a`). |
| 3b — misrepresent stubs as integrated | **policy preserved** — no misrepresentation; method bodies still return `ErrCodeUnimplemented`; commit messages are honest. |
| 3c — factual errors in integration plan | **EXECUTED** — `docs/research/inbox/2026-04-21_go-elder-plinius_integration_plan_CORRECTED.md` (commit `989e6a90`) audits ~50 claims with real file:line citations. |

### "Decisions that unblock each bucket"

| Decision | Resolution |
|----------|------------|
| 1. Authorize per-site sessions for Bucket 1c | **3/9 executed in-session**; remaining 6 planned (Phase 2.5). |
| 2. JSON-tagged-slice struct decision (MarshalJSON vs state-pointer) | **DECIDED** in Phase-2 spec: DiscoveredProvider → MarshalJSON-snapshot; AgentTeam/Task/ExtendedPlanModeSession → state-pointer refactor. |
| 3. Authorize staged protocol-layer migration | **planned** with test-under-load gate (Phase 3) awaiting per-pair authorization. |
| 4. Approve Phase-A for 9 defensible modules | **plans committed, GATED per-module approval**. |
| 5. Brainstorm each module's upstream surface before Phase-A | **built into Phase-4 per-module workflow**. |
| 6. Policy line on Bucket 3a public repos | **preserved**; offensive scaffolds retired; defensive-use fixture lift completed. |
| 7. Corrected-delta integration plan | **EXECUTED** (commit `989e6a90`). |

## Campaign telemetry

### Allowlist state

| | Count |
|-|-|
| Original (2026-04-20 HEAD `0ed68638`) | 254 |
| Pre-session HEAD `0ed59e09` | 24 |
| **Post-session HEAD** | **21** |
| Drained this session | MemoryService, ConcurrencyAlertManager, ContextManager |
| Total campaign drain rate | **92.5%** (233 of 254) |

### Audit status

```
concurrency-audit: OK — 21 Pattern-A struct(s) total, 21 allowlisted, 0 new.
```

### Test health for drained sites

All 3 drained sites pass `go test -race` under resource caps.
`ConcurrencyAlertManager` migration additionally fixed a pre-existing
latent race on `shouldFail`.

### Fixture harness state

- 7 attack-class YAMLs under `internal/security/redteam/fixtures/` populated with 47 defensive fixtures.
- `DeepTeamRedTeamer.RunFixtureSuite(ctx, class)` wired to `StandardGuardrailPipeline` via the `GuardrailInputChecker` extracted interface.
- `make test-redteam-fixtures` target present.
- `./challenges/scripts/redteam_fixtures_challenge.sh` — 26/26 checks PASS.
- `.gitattributes` sets `export-ignore` on fixtures dir.
- **Defensive gap signal:** with the populated fixtures, 24 of 47 currently slip the default pipeline. This is a real backlog for guardrail hardening, documented by class:
  - `filter_bypass`: 7/7 slipped (obfuscated/encoded variants bypass literal regex)
  - `stego_mutation`: 5/5 slipped (no Unicode normalization)
  - `abliteration_probe`: 5/7 slipped (keyword-list gaps)
  - `role_reversal`: 3/7 slipped
  - `system_prompt_extraction`: 2/8 slipped
  - `jailbreak`: 1/8 slipped (`jailbroken` vs `\bjailbreak\b`)
  - `genetic_seed`: 1/5 slipped (`{PERSONA}` template)

## Commits this session (17)

```
2e0fbf10 security(redteam): lift Bucket-3a corpora into defensive fixtures; retire offensive scaffolds
492ca2de feat(security/redteam): fixture loader with embed.FS + class taxonomy
debaa646 feat(security/redteam): RunFixtureSuite consumes attack-class fixtures via guardrail pipeline
6d745b09 feat(security/redteam): Makefile target + challenge script for fixture harness
dcb40520 docs(claude-md): document red-team fixture harness (defensive use policy)
cce1583a security(redteam/fixtures): populate with public defensive test patterns
989e6a90 docs(research): corrected-delta integration plan — factual baseline against actual HelixAgent code
898e3947 fix(go-elder-plinius-v3): compile-error repair for 7 defensible-subset modules
285ce618 chore(go-elder-plinius-v3): drop retired Bucket-3a entries from go.work
ba4bb5af docs(specs): CONST-029 Bucket-1c remaining sites plan (Phase 2.5)
f5f00014 docs(specs): CONST-029 structural blockers per-site plan (Phase 2)
09999b7a docs(specs): CONST-029 protocol-layer plan with test-under-load gate (Phase 3)
36ffc305 docs(specs): go-elder-plinius Phase-A index + 9 per-module stub plans (Phase 4, gated)
afa8785e migrate(services): ContextManager → safe.Store/Slice (CONST-029)
eb7def26 migrate(services): MemoryService → safe.Store + atomics (CONST-029)
1a3f93e2 migrate(services): ConcurrencyAlertManager → 6× safe.Store (CONST-029)
```
(+ this CLOSED commit = 18.)

## Cross-reference

- **Design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`
- **Execution plan:** `docs/superpowers/plans/2026-04-21-remaining-work-execution.md`
- **Phase-2 structural-blockers plan:** `docs/superpowers/specs/2026-04-21-const029-structural-blockers-plan.md`
- **Phase-2.5 Bucket-1c remaining plan:** `docs/superpowers/specs/2026-04-21-const029-bucket1c-remaining-plan.md`
- **Phase-3 protocol-layer plan:** `docs/superpowers/specs/2026-04-21-const029-protocol-layer-plan.md`
- **Phase-4 Phase-A plan (gated):** `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md` + 9 per-module stubs.
- **Phase-5 defensive fixtures:** `internal/security/redteam/fixtures/` + `./challenges/scripts/redteam_fixtures_challenge.sh`.
- **Corrected integration plan:** `docs/research/inbox/2026-04-21_go-elder-plinius_integration_plan_CORRECTED.md`.
- **Campaign memory:** `memory/project_const029_campaign.md`.
- **Concurrency playbook:** `docs/development/concurrency-playbook.md`.

---

*This file supersedes `docs/development/REMAINING_WORK_2026-04-21.md` as the session-close inventory. Future work resumes from the plan-committed specs listed above.*
