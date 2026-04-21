# Remaining Work Inventory — 2026-04-21 CLOSED

**Source:** `docs/development/REMAINING_WORK_2026-04-21.md` (original HEAD `0ed59e09`).
**Closed HEAD:** after final session commit.
**Session:** 2026-04-21 execution run per
`docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`
and `docs/superpowers/plans/2026-04-21-remaining-work-execution.md`.

## Terminal state per line item

Every item from the original inventory now sits in one of three
states: **executed+committed** / **plan committed** / **explicitly retired**.

### Bucket 1a — 7 structural blockers

| Site | Resolution |
|------|------------|
| `internal/optimization/context/window.go:22 ContextWindow` | **EXECUTED** — commit `010fd9b5`. `atomic.Pointer[*windowState]` CAS-loop; joint invariant preserved structurally. |
| `internal/optimization/gptcache/semantic_cache.go:50 SemanticCache` | **EXECUTED** — commit `a43328bc`. Pattern Zeta (narrow mu) + `*safe.Store`. |
| `internal/planning/mcts.go:27 MCTSNode` | **EXECUTED** — commit `943a8cd7`. `atomic.Uint64` via `math.Float64bits` + custom MarshalJSON/UnmarshalJSON; `/v1/planning/mcts` wire format preserved; round-trip test added. |
| `internal/services/provider_discovery.go:79 DiscoveredProvider` | **EXECUTED** — commit `14e838d7`. `*safe.Slice` + Pattern-Zeta mu for scalar verification cluster; MarshalJSON snapshot preserves wire format for 48 CLI agent configs. |
| `internal/handlers/extended/ensemble.go:26 AgentTeam` | **EXECUTED** — commit `a9a79da9` (triple). `atomic.Pointer[*teamState]` refactor. |
| `internal/handlers/extended/ensemble.go:84 Task` | **EXECUTED** — same commit. |
| `internal/handlers/extended/planning.go:24 ExtendedPlanModeSession` | **EXECUTED** — same commit. |

### Bucket 1b — 6 protocol-layer sites

| Site | Resolution |
|------|------------|
| `internal/services/protocol_discovery.go:19 ACPDiscoveryClient` | **EXECUTED** — commit `4a68c7e3`. `*safe.Store` + seedDiscoveryAgents test helper. |
| `internal/services/protocol_federation.go:16 ProtocolDiscovery` | **EXECUTED** — commit `ed6f6776`. `*safe.Store` + `*safe.Slice` + factory constructor for test fixtures. |
| `internal/services/acp_client.go:20 LSPClient` | **plan committed** — `docs/superpowers/specs/2026-04-21-const029-protocol-layer-plan.md`. Needs test-under-load gate. |
| `internal/services/acp_manager.go:22 ACPManager` + `:67 ACPClient` | **plan committed** (paired). Needs test-under-load gate. |
| `internal/services/mcp_client.go:20 MCPClient` + `:58 HTTPTransport` | **plan committed** (paired, highest transport risk). Needs test-under-load gate. |
| `internal/services/lsp_manager.go:22 LSPManager` | **plan committed**. Needs test-under-load gate. |

### Bucket 1c — 9 tractable high-coupling

| Site | Resolution |
|------|------------|
| `internal/services/memory_service.go:32 MemoryService` | **EXECUTED** — commit `eb7def26`. safe.Store + atomic.Bool/Int64/Pointer; tests pass under -race. |
| `internal/services/concurrency_alert_manager.go:505 ConcurrencyAlertManager` | **EXECUTED** — commit `1a3f93e2`. 6 maps → safe.Store; bonus: fixed latent `shouldFail` race. |
| `internal/services/context_manager.go:36 ContextManager` | **EXECUTED** — commit `afa8785e`. Two maps migrated; snapshot-based iteration. |
| `internal/verifier/adapters/free_adapter.go:68 FreeProviderAdapter` | **EXECUTED** — commit `8ae1d27b`. Bonus race-fix (`fa.mu`+per-call `modelsMu`) + regression test under -race. |
| `internal/services/provider_registry.go:72 ProviderRegistry` | **EXECUTED** — commit `1cc8d2eb`. 7 maps → safe.Store; atomic.Pointer for startupVerifier; drop shared mu. |
| `internal/services/debate_team_config.go:304 DebateTeamConfig` | **EXECUTED** — commit `0bca72af`. safe.Store + safe.Slice + atomic.Pointer. |
| `internal/knowledge/code_graph.go:124 CodeGraph` | **EXECUTED** — commit `ee90230c`. atomic.Pointer[*nodeIndices]/*edgeIndices (bundled indexes); writeMu for clone-store. |
| `internal/clis/pool.go:13 InstancePool` | **EXECUTED** — commit `682df93c`. Pattern Zeta (keep mu for state-machine transition) + safe.Slice/Store for underlying collections. |
| `internal/ensemble/background/worker_pool.go:33 WorkerPool` | **EXECUTED** — commit `5b7ad560`. *safe.Slice for workers; narrow startStopMu for lifecycle. |

### Bucket 2 — go-elder-plinius integration

| Item | Resolution |
|------|------------|
| 9 defensible modules compile | **EXECUTED** — commit `898e3947` + `285ce618`. All 9 `go build ./... → exit 0`. |
| Phase-A implementation (398 methods) | **plan committed, GATED** — `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md` + 9 per-module stubs (commit `36ffc305`). |
| 35 integration files + 15 submodules | **plan committed, GATED** — within Phase-A specs. |
| 22 non-defensible modules | 13 other-category stay as `docs/research/` reference; 9 offensive retired. |
| Public `vasic-digital` repos | **RETIRED** — INTERNAL-only integration per Phase 4 plan. |

### Bucket 3 — policy-declined

| Item | Resolution |
|------|------------|
| 3a — 9 offensive modules | **RETIRED + LIFTED** — scaffolds removed (`2e0fbf10`); defensive fixture harness live (`492ca2de`, `debaa646`, `6d745b09`, `dcb40520`); fixtures populated with 47 public defensive patterns (`cce1583a`); all 24 gaps closed to **100% block rate** via normalisation + pattern expansion (`839c4e27`, `3f8a29b7`). |
| 3b — misrepresent stubs as integrated | **policy preserved** — no misrepresentation. |
| 3c — factual errors in integration plan | **EXECUTED** — `docs/research/inbox/2026-04-21_go-elder-plinius_integration_plan_CORRECTED.md` (`989e6a90`). |

### "Decisions that unblock each bucket"

| Decision | Resolution |
|----------|------------|
| 1. Authorize per-site sessions for Bucket 1c | **9/9 EXECUTED in-session** (MemoryService, ConcurrencyAlertManager, ContextManager, FreeProviderAdapter, ProviderRegistry, DebateTeamConfig, CodeGraph, InstancePool, WorkerPool). |
| 2. JSON-tagged-slice struct decision | **DECIDED + EXECUTED** — MarshalJSON-snapshot (DiscoveredProvider); state-pointer refactor (AgentTeam/Task/ExtendedPlanModeSession). |
| 3. Authorize staged protocol-layer migration | **2/6 EXECUTED + 4/6 plan committed** (paired ACPManager+ACPClient, paired MCPClient+HTTPTransport, LSPClient, LSPManager remain). |
| 4. Approve Phase-A for 9 defensible modules | **plans committed, GATED per-module approval**. |
| 5. Brainstorm each module's upstream surface before Phase-A | **built into Phase-4 workflow**. |
| 6. Policy line on Bucket 3a public repos | **preserved**; offensive scaffolds retired; defensive-use fixture lift completed; 47/47 fixtures blocked. |
| 7. Corrected-delta integration plan | **EXECUTED**. |

## New governance rules this session

### CONST-030: Real Infrastructure for All Non-Unit Tests
Mocks/stubs/fakes/placeholders/hardcoded data MAY be used ONLY in unit tests. ALL other test types (integration, E2E, functional, security, stress, chaos, challenge, benchmark, HelixQA, runtime verification) MUST execute against the REAL running HelixAgent with REAL containers, REAL databases, REAL Redis, REAL MCP/ACP/LSP, REAL HTTP calls. Before every non-unit test run, HelixAgent binary MUST build, distribute, and boot all containers. Non-unit tests that cannot connect MUST skip (not fail). Violations block merge. **Commit:** `10bec3d3`.

### CONST-031: Authorized Remote Distribution Hosts
Remote distribution hosts are registered **dynamically** via `CONTAINERS_REMOTE_HOST_N_*` in `Containers/.env` (N=1..100, stops at first absent _NAME). N scales freely ≥ 1. No host name is hardcoded anywhere else in the repo. Current configured set (at this commit): `thinker.local` + `amber.local`. **Commits:** `10bec3d3` (rule added) + `397982d0` (de-hardcode fix per user directive: N >= 1 hosts).

Constitution version bumped **1.3.0 → 1.4.0**. `CONSTITUTION.json` has 31 rules.

## Campaign telemetry

### Allowlist state

| | Count |
|-|-|
| Original (2026-04-20 HEAD `0ed68638`) | 254 |
| Pre-session HEAD `0ed59e09` | 24 |
| **Post-session HEAD** | **6** |
| **Drained this session** | **18** (MemoryService, ConcurrencyAlertManager, ContextManager, WorkerPool, DiscoveredProvider, MCTSNode, ContextWindow, FreeProviderAdapter, DebateTeamConfig, ProviderRegistry, InstancePool, AgentTeam, Task, ExtendedPlanModeSession, SemanticCache, CodeGraph, ACPDiscoveryClient, ProtocolDiscovery) |
| **Total campaign drain rate** | **97.6%** (248 of 254) |

### Remaining 6 (all protocol-layer, need test-under-load gate)

- `internal/services/acp_client.go:20 LSPClient`
- `internal/services/acp_manager.go:22 ACPManager` + `:67 ACPClient` (paired)
- `internal/services/lsp_manager.go:22 LSPManager`
- `internal/services/mcp_client.go:20 MCPClient` + `:58 HTTPTransport` (paired)

All 6 have plan-committed specs at `docs/superpowers/specs/2026-04-21-const029-protocol-layer-plan.md`.

### Audit + tests

- `./scripts/concurrency-audit.sh`: **OK — 6 Pattern-A, 6 allowlisted, 0 new.**
- Guardrail fixture harness: **47/47 fixtures blocked (100%)** — up from baseline 23/47 (49%) at start of session. All 24 gaps closed via normalisation (base64/leet/homoglyph/NFKC/zero-width/ROT13/whitespace/reverse) + pattern expansion.
- All 9 go-elder-plinius defensible modules compile (`go build ./... → exit 0`).
- `go build ./...` at root: clean (no output).

### Fixture harness state

- 7 attack-class YAMLs populated with 47 fixtures under `internal/security/redteam/fixtures/`.
- `DeepTeamRedTeamer.RunFixtureSuite(ctx, class)` consumer wired.
- Input normalisation layer (`internal/security/normalize.go`) covers: NFKC Unicode, zero-width strip, leet-speak de-leet, homoglyph fold, ROT13, base64 decode, whitespace collapse, string reverse.
- `make test-redteam-fixtures` passes.
- `./challenges/scripts/redteam_fixtures_challenge.sh` — 26/26 pass.
- `.gitattributes` sets `export-ignore` on fixtures dir.

## Session commits (39 since `0ed59e09`)

Run `git log --oneline 0ed59e09..HEAD` for the full list.

Highlights in chronological order:

- Phase 0+5 fixture harness (`2e0fbf10`, `492ca2de`, `debaa646`, `6d745b09`, `dcb40520`, `cce1583a`).
- Bucket-3c corrected plan (`989e6a90`).
- Bucket-2 compile fixes + go.work cleanup (`898e3947`, `285ce618`).
- Phase 2/2.5/3/4 plan-only specs (`ba4bb5af`, `f5f00014`, `09999b7a`, `36ffc305`).
- 3 Wave-0 CONST-029 drains (`afa8785e`, `eb7def26`, `1a3f93e2`).
- REMAINING_WORK_CLOSED v1 + allowlist cleanup (`cb0df1b5`).
- **Governance**: CONST-030 + CONST-031 (`10bec3d3`); de-hardcode fix (`397982d0`).
- **Guardrail hardening**: 24/24 gap closure + BUGFIXES entry (`839c4e27`, `3f8a29b7`).
- **Wave A** (5 drains): `5b7ad560`, `14e838d7`, `943a8cd7`, `010fd9b5`, `8ae1d27b`.
- **Wave B** (6 drains): `0bca72af`, `1cc8d2eb`, `682df93c`, `a9a79da9`, `a43328bc`, `ee90230c`.
- **Wave C** (2 drains): `4a68c7e3`, `ed6f6776`.
- This closing commit.

## Cross-reference

- **Design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md`
- **Execution plan:** `docs/superpowers/plans/2026-04-21-remaining-work-execution.md`
- **Phase-2 structural-blockers plan:** `docs/superpowers/specs/2026-04-21-const029-structural-blockers-plan.md`
- **Phase-2.5 Bucket-1c remaining plan:** `docs/superpowers/specs/2026-04-21-const029-bucket1c-remaining-plan.md`
- **Phase-3 protocol-layer plan:** `docs/superpowers/specs/2026-04-21-const029-protocol-layer-plan.md`
- **Phase-4 Phase-A plan (gated):** `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md` + 9 per-module stubs.
- **Phase-5 defensive fixtures:** `internal/security/redteam/fixtures/` + `./challenges/scripts/redteam_fixtures_challenge.sh`.
- **Input normalisation:** `internal/security/normalize.go`.
- **Corrected integration plan:** `docs/research/inbox/2026-04-21_go-elder-plinius_integration_plan_CORRECTED.md`.
- **BUGFIXES (Issue #30):** `docs/issues/fixed/BUGFIXES.md`.
- **Campaign memory:** `memory/project_const029_campaign.md`.
- **Concurrency playbook:** `docs/development/concurrency-playbook.md`.
- **Dynamic-hosts loader:** `Containers/pkg/envconfig/parser.go`.

---

*This file supersedes `docs/development/REMAINING_WORK_2026-04-21.md` as the session-close inventory. Remaining work resumes from the 6 protocol-layer specs listed above, each gated on a test-under-load session authorization.*
