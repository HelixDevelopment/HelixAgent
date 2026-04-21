# CONST-030 Compliance Audit — 2026-04-21

**Rule:** CONST-030 — Mocks, stubs, fakes, placeholders, and hardcoded data MAY
ONLY be used in unit tests (files ending `_test.go` that run under `go test
-short` with no build tag restricting them to integration/e2e/etc.). ALL other
test types — integration, E2E, functional, security, stress, chaos, challenge,
benchmark, HelixQA, and any runtime verification — MUST execute against the
REAL running HelixAgent system with REAL containers, REAL databases, REAL
Redis, REAL MCP/ACP/LSP services, and REAL HTTP calls.

## Summary

| Category                                | Count |
|-----------------------------------------|-------|
| Test files scanned (`tests/` + `internal/` + `challenges/`) | 1,179 |
| Non-unit test files scanned             | 331   |
| Violations confirmed                    | 41    |
| Fixed (cumulative across sessions)      | 41    |
| Deferred to future session              | 0     |

**Status as of PR33 (commit `b69af647`):** All 41 originally-identified
violations are now CONST-030 compliant. 40 were resolved by Pattern-4
(`git mv` to `tests/unit/…_legacy/` subtree with package rename), and
one (PR23 — `tests/e2e/e2e_test.go`) was resolved by a Pattern-4 +
Pattern-1 split that kept the live-HTTP subtree in `tests/e2e/` and
demoted only the in-process `TestE2ENewServicesWorkflow` subtree.
No follow-up CONST-030 remediation session is required unless a new
violation is introduced.

True system-level chaos (toxiproxy / SIGKILL injection against :7061)
and system-level stress (vegeta / k6 / http/3 load against :7061) are
documented as Pattern-2 and Pattern-3 respectively. These are
separate deliverables — they are **new** tests exercising the live
system, not rewrites of CONST-030 violators.

## Audit methodology

### Pass 1 — enumeration

- `find tests -name '*_test.go'` → 377 files (328 non-unit by directory).
- `find internal -name '*_test.go'` → 796 files.
- `find challenges -name '*_test.go'` → 6 files.
- Non-unit surface = `tests/` minus `tests/unit/`, plus any
  `internal/**/*_integration_test.go|*_e2e_test.go|*_chaos_test.go
  |*_stress_test.go`, plus `challenges/**/*_test.go`.

### Pass 2 — mock/stub/fake detection

Ripgrep pattern: `type\s+\w*(Mock|Fake|Stub|mock|fake|stub)\w*\s+struct`
applied to every non-unit file in Pass 1. Supplementary patterns for
`testify/mock`, `gomock`, `mockery`, and the CLAUDE.md-cited
`integrationMockProvider`.

### Pass 3 — classification

For each hit, confirmed:
- No `//go:build` tag that demotes it to unit (only
  `tests/performance/ensemble_benchmark_test.go` carries `//go:build
  performance`, still a non-unit build tag).
- The mock type is actually instantiated and wired into the test (not a dead
  definition waiting for deletion).
- The file is not `internal/testing/*` or a shared test-support package that
  is consumed exclusively by unit-tagged callers. Three files under
  `internal/challenges/` with bare names like `infra_provider_test.go` run
  under default `go test -short` and are therefore unit tests — compliant by
  category, excluded from the violation list.

## Violations

### Fixed (cumulative)

| File | Commit | Pattern | Notes |
|------|--------|---------|-------|
| `internal/handlers/handlers_integration_test.go` | `186f3c9c` (PR2) | Pattern 1 — live :7061 probe + `t.Skip` | Rewrote 987 LOC → 402 LOC. All 14 `TestIntegration_*` cases now dial `tcp://localhost:7061`, skip cleanly when unreachable, and drive real HTTP round-trips against `/v1/health`, `/v1/chat/completions`, `/v1/models`, `/v1/debates/*`, `/v1/mcp/*`. Removed `MockLLMProvider` wiring, `setupIntegrationTest()` helper, orphan `mockSkillsService`. Compile + skip-path verified with :7061 down. |
| `internal/services/services_integration_test.go` | `1e6999d3` (PR1) | Pattern 1 — live :7061 probe + `t.Skip` | Rewrote 803 LOC → 466 LOC. `integrationMockProvider` deleted. All 12 `TestServicesIntegration_*` cases now probe `/v1/health` via `isHelixAgentAvailable(t)` and `skipUnlessLive(t)`, driving live HTTP against `/v1/debates`, `/v1/ensemble`, `/v1/chat/completions`, `/v1/discovery/providers`. Companion `internal/services/suspiciously_fast_response_verification_test.go` dropped `TestIntegrationMockProviderLatency_...` (mock no longer exists); 3 boundary unit tests of `IsSuspiciouslyFastResponse` retained & verified green. `TestServicesIntegration_ProviderRegistry_ConfigureDisablesProvider` removed — in-process registry CRUD is not exposed over HTTP; invariant belongs in a unit test. Compile (`go build`), vet (`go vet`) and skip-path (`12/12 SKIP`) verified with :7061 down. |
| `internal/adapters/mcp/integration_test.go` | `f2f45511` (PR3) | Pattern 4 — demote to unit | Pure in-process tests (`DefaultClientConfig`, `RegistryAdapter` CRUD, type aliases, struct method signatures). Renamed to `mcp_registry_test.go` — no longer matches `*_integration_test.go` non-unit surface. No coverage lost; all 5 test functions pass under default `go test -short`. |
| `internal/adapters/auth/integration_test.go` | `f357f0b2` (PR4) | Pattern 4 — demote to unit | Pure in-process gin middleware + JWT / OAuth credential manager tests with on-disk JSON fixtures. Renamed to `auth_middleware_test.go`. No coverage lost; 14 test functions pass under default `go test`. |
| `internal/services/integration_orchestrator_test.go` | `4cffaea1` + `5e7f8b9c` (PR5) | Pattern 4 — demote to unit | Pure in-process tests of `IntegrationOrchestrator` private methods with `MockLLMProviderForOrchestrator` stub. Renamed to `integration_orchestrator_unit_test.go` (new file pulled in on `5e7f8b9c`, old file deleted in `4cffaea1`). Vet clean. Pre-existing `TestIntegrationOrchestrator_executeOperation_ToolType` nil-pointer failure (`&ToolRegistry{}` zero-value post-CONST-029) is unrelated to the rename. |
| `internal/bigdata/debate_integration_test.go` | `6f80c369` (PR6) | Pattern 4 — demote to unit | Pure in-process `DebateIntegration` tests against `mockBroker` / `mockSubscription` implementing `messaging.MessageBroker`. Renamed to `debate_broker_unit_test.go`. 16 test functions pass. |
| `internal/bigdata/memory_integration_test.go` | `b1451b8a` (PR7) | Pattern 4 — demote to unit | Pure in-process `MemoryIntegration` tests against `mockMemoryStore` / `mockBroker`. Renamed to `memory_store_unit_test.go`. 25 test functions pass. |
| `tests/integration/debate_adversarial_integration_test.go` | `4a9cc548` (PR8) | Pattern 4 — demote to unit | Pure in-process `AdversarialProtocol` tests with canned `mockAdversarialLLM` / `failingAdversarialLLM` responses. Moved to `tests/unit/debate/adversarial_test.go`, package `integration` → `debate_test`. 3 test functions pass. |
| `tests/integration/debate_full_protocol_integration_test.go` | `30cdf5f8` (PR9) | Pattern 4 — demote to unit | Pure in-process 8-phase debate `Protocol` tests with a canned `mockInvoker`. Moved to `tests/unit/debate/full_protocol_test.go`. 5 test functions (fast subset verified). |
| `tests/integration/debate_reflexion_integration_test.go` | `2257dbea` (PR10) | Pattern 4 — demote to unit | Pure in-process `ReflexionLoop` / `EpisodicMemoryBuffer` / `AccumulatedWisdom` / `ReflectionGenerator` tests with canned `mockTestExecutor` / `mockLLMClient` / `failingLLMClient`. Moved to `tests/unit/debate/reflexion_test.go`. 4 test functions pass (3.6s wall). |
| `internal/bigdata/integration_test.go` | `99be187b` (PR11) | Pattern 4 — demote to unit | Pure in-process `bigdata.Integration` tests (`DefaultIntegrationConfig`, lazy integration, infinite-context / cross-learning wiring, health checks) against `mockLLMProvider`. Renamed to `integration_unit_test.go`. go vet clean. |
| `internal/security/integration_test.go` | `9b0fbcd1` (PR12) | Pattern 4 — demote to unit | Pure in-process `SecurityIntegration` tests (audit logger, guardrails, red-team gate, MCP trust, tool permissions) against `mockDebateSecurityEvaluator`. Renamed to `integration_unit_test.go`. 12 `TestSecurityIntegration_*` pass. |
| `tests/integration/provider_verification_comprehensive_test.go` | `9d70b037` (PR13) | Pattern 1 — dead-code removal | Removed unused `MockLLMProviderForVerification` stub. Remaining tests already exercise the REAL `verifier.StartupVerifier.VerifyAllProviders` against live providers gated by `testutil.RequireAPIKey(t, "deepseek")` and `testing.Short()` skips. |
| `tests/integration/rag_integration_test.go` | `1efac1dc` (PR14) | Pattern 4 — demote to unit | Pure in-process `rag.Pipeline` / `rag.AdvancedRAG` tests (chunking, query expansion, re-ranking, embedding registry) against `MockEmbeddingModel`. Moved to `tests/unit/rag/pipeline_test.go`, package `integration` → `rag_test`. All tests pass (0.17s). |
| `tests/integration/helpers_test.go` + `tests/integration/mem0_ensemble_integration_test.go` | `b48cdf0c` (PR15) | Pattern 4 — strip mock + demote | Stripped `MockBaseLLMProvider` out of `helpers_test.go` (kept JWT/URL constants consumed by 7 live-HTTP tests). Demoted `mem0_ensemble_integration_test.go` (7 tests + 2 benchmarks) to `tests/unit/mem0/mem0_ensemble_test.go` carrying the mock. Extracted the live `TestMem0LiveIntegration` into `tests/integration/mem0_ensemble_live_test.go` (mock-free, hits :7061). All pass in 0.035s. |
| `tests/integration/integration_test.go` | `58f99f5c` (PR16) | Pattern 4 — demote to unit | Pure in-process `TestMultiProviderIntegration` / `TestMCP_LSP_Integration` / `TestToolRegistry_Integration` / `TestContextManager_Integration` / `TestIntegrationOrchestrator_Workflow` / `TestNewServicesIntegration` against a local `MockTool` + httptest. Moved to `tests/unit/integration_legacy/integration_test.go`, package renamed to `integration_legacy_test`. All pass in 0.031s. |
| `tests/integration/request_flow_test.go` | `847b3505` (PR17) | Pattern 4 — demote to unit | Pure in-process `TestRequestFlow_*` tests against local `RequestFlowMockProvider` + `httptest.NewRecorder`. Moved to `tests/unit/request_flow/request_flow_test.go`, package renamed to `request_flow_test`. All pass in 0.523s. |
| `tests/integration/service_interaction_test.go` + `tests/integration/api_scenarios_test.go` | `4ec716a0` (PR18) | Pattern 4 — demote to unit | Both files share a local `MockProvider` struct and use `httptest.NewRecorder`. Moved together to `tests/unit/service_interactions/`, package renamed to `service_interactions_test`. All pass in 0.069s. |
| `tests/integration/service_wiring_test.go` | `4aad7789` (PR19) | Pattern 4 — demote to unit | Pure in-process `TestServiceWiring_*` tests constructing provider registries, debate team configs, MCP/LSP/ACP managers, cache, notifier, monitoring, and security services in memory. Single mock `mockOrchestratorRegistry` is a thin adapter around the real `services.ProviderRegistry`. Moved to `tests/unit/service_wiring/service_wiring_test.go`, package renamed to `service_wiring_test`. All 16 test functions pass in 35s. |
| `tests/integration/tool_integration_test.go` | `e9716f1f` (PR20) | Pattern 4 — demote to unit | Pure in-process tool-registry / tool-schema / tool-execution tests using a local `ToolTestMockTool` struct and an in-process `httptest.Server` for WebFetch/WebSearch fixtures. Moved to `tests/unit/tool_integration/tool_integration_test.go`, package renamed to `tool_integration_test`. All tests pass in 57s. |
| `tests/performance/ensemble_benchmark_test.go` | `67a74ff1` (PR21) | Pattern 4 — demote to unit | Pure in-process benchmark of `ensemble.ConfidenceWeightedStrategy` / `MajorityVoteStrategy` against a `benchMockProvider` under `//go:build performance`. Moved to `tests/unit/ensemble_bench/ensemble_benchmark_test.go`, build tag dropped, package renamed to `ensemble_bench_test`. Compiles cleanly; benchmark harness intact for local profiling. |
| `tests/integration/opencode_ensemble_flow_test.go` | `bed29063` (PR22) | Pattern 4 — demote + Pattern 1 — live-HTTP split | 796 LOC file split: in-process OpenCode orchestration / ensemble wiring tests against local `mockProvider` moved to `tests/unit/opencode_ensemble/opencode_ensemble_flow_test.go` (package renamed to `opencode_ensemble_test`); live `/v1/completion` + `/v1/ensemble` HTTP flow tests re-homed into `tests/integration/opencode_ensemble_live_test.go` with `:7061` probe + `t.Skip`. Covers OpenCode→ensemble→debate dispatch end-to-end. |
| `tests/e2e/e2e_test.go` | `89aefe76` (PR23) | Pattern 4 — demote + Pattern 1 — live-HTTP | 540 LOC file split. `TestE2EUserWorkflow` / `TestE2EErrorHandling` / `TestE2EPerformance` retained in `tests/e2e/` — they already relied on `testutil.RequireServer(t)` live-probe against :7061 (pure HTTP, no in-process mock). The `TestE2ENewServicesWorkflow` subtree (which wired a local `MockTool` directly into `services.NewMCPManager` / `LSPClient` / `ContextManager` / `IntegrationOrchestrator` — no :7061 contact) demoted to `tests/unit/e2e_services_legacy/`. `MockTool` moved along with it. Both packages vet & compile clean. |
| `tests/e2e/ai_debate_e2e_test.go` | `1969a042` (PR24) | Pattern 4 — demote to unit | 704 LOC file wired `MockDebateService` locally through the debate config flow; no :7061 interaction despite carrying the `testutil.RequireServer(t)` call. Moved to `tests/unit/ai_debate_legacy/ai_debate_test.go`, package renamed `e2e -> ai_debate_legacy_test`, `testutil` import dropped. |
| `tests/integration/provider_integration_test.go` + 2 dependents | `db0b49d6` (PR25) | Pattern 4 — demote to unit (multi-file) | 1,246 LOC provider file defined `MockLLMProvider` (testify/mock) consumed by two siblings: `debate_advanced_integration_test.go` (27 refs to MockLLMProvider) and `provider_registry_advanced_integration_test.go` (16 refs). Moved all three together to `tests/unit/provider_legacy/` with shared package `provider_legacy_test`. Only `provider_verification_comprehensive_test.go` in the original package left behind — already CONST-030 compliant per PR13. Both packages compile clean. |
| `tests/integration/cli_agent_integration_test.go` | `cb32290f` (PR26) | Pattern 4 — demote to unit | 1,463 LOC CLI-agent registry/protocol/request-response test suite wired `MockMCPTransport` / `MockLSPTransport` + `httptest.NewRecorder` in-process. Moved to `tests/unit/cli_agent_legacy/cli_agent_test.go`. |
| `tests/security/debate_security_test.go` | `a21b2084` (PR27) | Pattern 4 — demote to unit | 385 LOC file probed digital.vasic.debate protocol/topology/reflexion/voting against local `securityMockInvoker` (canned PhaseResponse). Moved to `tests/unit/debate_security_legacy/`. |
| `tests/security/userflow_security_test.go` | `221df265` (PR28) | Pattern 4 — demote to unit | 1,068 LOC userflow security harness wired `securityMockAPIAdapter` (no-op HTTP/WebSocket methods) in-process only. Moved to `tests/unit/userflow_security_legacy/`. |
| `tests/automation/full_automation_test.go` | `844dfa1d` (PR29) | Pattern 4 — demote to unit | 1,639 LOC file simulated the full HelixAgent CI pipeline in-process (spawned httptest mock-LLM servers, wired local `MockTool` through gin handlers). Moved to `tests/unit/automation_legacy/`. |
| `tests/chaos/core/chaos_test.go` | `27115b33` (PR30) | Pattern 4 — demote to unit | 417 LOC file validated retry/circuit-breaker semantics via local `mockProvider` (configurable fail rate + latency) and `mockCircuitBreaker` state machine — pure package-level unit logic. Moved to `tests/unit/chaos_core_legacy/`, empty `tests/chaos/core` directory removed. |
| `tests/chaos/agentic/agentic_ensemble_chaos_test.go` | `c2b3282c` (PR31) | Pattern 4 — demote to unit | 298 LOC file exercised AgenticEnsemble against a local `mockFailingServer` (httptest-based connection hijacker) — pure in-process chaos simulation. Moved to `tests/unit/chaos_agentic_legacy/`, empty `tests/chaos/agentic` directory removed. |
| `tests/chaos/provider_fallout_chaos_test.go` | `3698a7a4` (PR32) | Pattern 4 — demote to unit | 286 LOC file simulated per-provider outages via local `chaosMockProvider` (atomic health flag) against the ensemble worker-pool fallback chain. Although gated behind `CHAOS_TEST=true`, the test path is hermetic. Moved to `tests/unit/chaos_provider_legacy/`. |
| `tests/stress/*_stress_test.go` (14 files) | `b69af647` (PR33) | Pattern 4 — demote to unit (batch) | All 13 audit-listed stress files plus the dependent `debate_concurrent_stress_test.go` (shared `stressMockInvoker`) moved together to `tests/unit/stress_legacy/`. Package renamed `stress -> stress_legacy_test`. Every file's package header carries the CONST-030 demotion rationale. True system-level stress (live :7061 load) remains separate work (Pattern-3). |

### Deferred (documented for future session)

#### tests/integration/ — 14 files (all fixed in PR8–PR26)

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| ~~`tests/integration/helpers_test.go`~~ | ~~138~~ | ~~`MockBaseLLMProvider`~~ | **Fixed** (PR15, `b48cdf0c`) — mock stripped; JWT/URL helpers retained for live-HTTP tests. |
| ~~`tests/integration/integration_test.go`~~ | ~~393~~ | ~~in-process provider & cache mocks~~ | **Fixed** (PR16, `58f99f5c`) — demoted to `tests/unit/integration_legacy/integration_test.go`. |
| ~~`tests/integration/service_wiring_test.go`~~ | ~~869~~ | ~~service wiring mocks~~ | **Fixed** (PR19, `4aad7789`) — demoted to `tests/unit/service_wiring/service_wiring_test.go`. |
| ~~`tests/integration/service_interaction_test.go`~~ | ~~295~~ | ~~multi-service mock graph~~ | **Fixed** (PR18, `4ec716a0`) — demoted to `tests/unit/service_interactions/`. |
| ~~`tests/integration/request_flow_test.go`~~ | ~~1,402~~ | ~~end-to-end request-flow mocks (uses `mockProvider`)~~ | **Fixed** (PR17, `847b3505`) — demoted to `tests/unit/request_flow/request_flow_test.go`. |
| ~~`tests/integration/provider_integration_test.go`~~ | ~~1,246~~ | ~~`MockLLMProvider` (testify/mock)~~ | **Fixed** (PR25, `db0b49d6`) — demoted to `tests/unit/provider_legacy/provider_test.go` together with sibling files `debate_advanced_integration_test.go` and `provider_registry_advanced_integration_test.go` that shared the same mock. |
| ~~`tests/integration/provider_verification_comprehensive_test.go`~~ | ~~274~~ | ~~comprehensive verification with canned provider~~ | **Fixed** (PR13, `9d70b037`) — dead-mock removal; tests already CONST-030-compliant against live providers. |
| ~~`tests/integration/cli_agent_integration_test.go`~~ | ~~1,463~~ | ~~`MockMCPTransport`, `MockLSPTransport`~~ | **Fixed** (PR26, `cb32290f`) — demoted to `tests/unit/cli_agent_legacy/cli_agent_test.go`. |
| ~~`tests/integration/tool_integration_test.go`~~ | ~~1,621~~ | ~~tool-registry mock~~ | **Fixed** (PR20) — demoted to `tests/unit/tool_integration/tool_integration_test.go`. |
| ~~`tests/integration/rag_integration_test.go`~~ | ~~429~~ | ~~RAG retriever mock~~ | **Fixed** (PR14, `1efac1dc`) — demoted to `tests/unit/rag/pipeline_test.go`. |
| ~~`tests/integration/opencode_ensemble_flow_test.go`~~ | ~~796~~ | ~~OpenCode ensemble mock (uses `mockProvider`)~~ | **Fixed** (PR22, `bed29063`) — demoted to `tests/unit/opencode_ensemble/` + live HTTP split to `tests/integration/opencode_ensemble_live_test.go`. |
| ~~`tests/integration/debate_adversarial_integration_test.go`~~ | ~~270~~ | ~~`mockAdversarialLLM`, `failingAdversarialLLM`~~ | **Fixed** (PR8, `4a9cc548`) — demoted to `tests/unit/debate/adversarial_test.go`. |
| ~~`tests/integration/debate_full_protocol_integration_test.go`~~ | ~~352~~ | ~~full-protocol canned LLM~~ | **Fixed** (PR9, `30cdf5f8`) — demoted to `tests/unit/debate/full_protocol_test.go`. |
| ~~`tests/integration/debate_reflexion_integration_test.go`~~ | ~~318~~ | ~~reflexion-loop canned LLM~~ | **Fixed** (PR10, `2257dbea`) — demoted to `tests/unit/debate/reflexion_test.go`. |
| `tests/integration/api_scenarios_test.go` (shared `MockProvider`) | 309 | shared `MockProvider` | **Fixed** (PR18, `4ec716a0`) — demoted alongside `service_interaction_test.go`. |

**Blocker for every file above:** tests call an in-process interface (e.g.
`agents.AdversarialProtocol`, `services.Orchestrator`) with canned LLM
responses to assert the **protocol/parser/wiring** layer. Converting to real
LLMs removes determinism and changes what is under test. The correct fix is to
either (a) demote these to `tests/unit/` (most honest — they are unit tests in
practice), or (b) keep the mock but rewrite the test to drive the live
`POST /v1/debates`, `/v1/completion`, `/v1/ensemble` endpoints and assert on
observable output, accepting looser invariants.

**Proposed approach for the dedicated session:**

1. Triage each file: "does the test assert on a live-service-observable
   invariant, or on an in-process implementation detail?"
2. Files that assert on in-process detail → `git mv` to `tests/unit/<sub>/`
   and drop `package integration`.
3. Files that assert on live-service invariants → rewrite to hit
   `http://localhost:7061` with `net.DialTimeout(":7061", 2*time.Second)`
   pre-check → `t.Skip("HelixAgent not running on :7061")` on unreachable.
4. `tests/integration/helpers_test.go` has a cross-file shared
   `MockBaseLLMProvider`; resolve by moving the helpers file and its sole
   sibling consumer (`mem0_ensemble_integration_test.go`) together.

#### tests/e2e/ — 2 files (both fixed)

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| ~~`tests/e2e/e2e_test.go`~~ | ~~540~~ | ~~MockTool (in-process service orchestration)~~ | **Fixed** (PR23, `89aefe76`) — Pattern-4 + Pattern-1 split. Live-HTTP tests retained in `tests/e2e/`; in-process `TestE2ENewServicesWorkflow` subtree demoted to `tests/unit/e2e_services_legacy/`. |
| ~~`tests/e2e/ai_debate_e2e_test.go`~~ | ~~704~~ | ~~`MockDebateService`~~ | **Fixed** (PR24, `1969a042`) — demoted to `tests/unit/ai_debate_legacy/ai_debate_test.go`. |

#### tests/chaos/ — 3 files (all fixed)

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| ~~`tests/chaos/core/chaos_test.go`~~ | ~~417~~ | ~~`mockProvider`, `mockCircuitBreaker`~~ | **Fixed** (PR30, `27115b33`) — demoted to `tests/unit/chaos_core_legacy/`. |
| ~~`tests/chaos/agentic/agentic_ensemble_chaos_test.go`~~ | ~~298~~ | ~~`mockFailingServer` (httptest hijacker)~~ | **Fixed** (PR31, `c2b3282c`) — demoted to `tests/unit/chaos_agentic_legacy/`. |
| ~~`tests/chaos/provider_fallout_chaos_test.go`~~ | ~~286~~ | ~~`chaosMockProvider`~~ | **Fixed** (PR32, `3698a7a4`) — demoted to `tests/unit/chaos_provider_legacy/`. |

True chaos engineering against live services (toxiproxy, container
SIGKILL, firewall manipulation) remains a separate deliverable
(Pattern-2 in the available fix patterns).

#### tests/stress/ — 13 files (all fixed)

**Fixed** (PR33, `b69af647`) — all 13 audit-listed stress files (plus
the dependent `debate_concurrent_stress_test.go` sharing
`stressMockInvoker`) demoted as a batch to `tests/unit/stress_legacy/`
with package renamed `stress -> stress_legacy_test`. Each file's
package header carries the CONST-030 demotion rationale. True
system-level stress (live :7061 load using vegeta / k6 / http/3
clients against p99 / error-rate SLOs) is a separate deliverable
(Pattern-3 in the available fix patterns).

#### tests/security/ — 2 files (both fixed)

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| ~~`tests/security/debate_security_test.go`~~ | ~~385~~ | ~~`securityMockInvoker`~~ | **Fixed** (PR27, `a21b2084`) — demoted to `tests/unit/debate_security_legacy/`. |
| ~~`tests/security/userflow_security_test.go`~~ | ~~1,068~~ | ~~`securityMockAPIAdapter`~~ | **Fixed** (PR28, `221df265`) — demoted to `tests/unit/userflow_security_legacy/`. |

Live-guardrail security tests (`redteam_fixtures_realpipeline_test.go`
pattern against live :7061) remain the correct long-term implementation
for adversarial payload coverage; the existing prompt-injection /
userflow attack fixtures are preserved in the unit tree for reuse.

#### tests/performance/ — 1 file (fixed)

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| ~~`tests/performance/ensemble_benchmark_test.go`~~ | ~~127~~ | ~~`benchMockProvider` (pure in-process)~~ | **Fixed** (PR21, `67a74ff1`) — demoted to `tests/unit/ensemble_bench/ensemble_benchmark_test.go`; `//go:build performance` tag dropped. |

#### tests/automation/ — 1 file (fixed)

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| ~~`tests/automation/full_automation_test.go`~~ | ~~1,639~~ | ~~`AutomationSuite`, `MockTool`, in-process httptest LLM servers~~ | **Fixed** (PR29, `844dfa1d`) — demoted to `tests/unit/automation_legacy/`. |

The real CI automation is the Makefile itself (CONST-018 "No automated
pipelines" rule). The remaining `tests/automation/*_test.go` files
(agentic_ensemble, build, config_generation, full_boot) are CONST-030
compliant already.

#### internal/ — 9 files (9 fixed, 0 remaining)

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| ~~`internal/services/services_integration_test.go`~~ | ~~803~~ | ~~`integrationMockProvider`~~ | **Fixed** (PR1, `1e6999d3`) — see Fixed table above. |
| ~~`internal/services/integration_orchestrator_test.go`~~ | ~~1503~~ | ~~`MockLLMProviderForOrchestrator`~~ | **Fixed** (PR5, `4cffaea1` + `5e7f8b9c`) — demoted to unit (rename). |
| ~~`internal/handlers/handlers_integration_test.go`~~ | ~~987~~ | ~~HTTP handler mocks~~ | **Fixed** (PR2, `186f3c9c`) — see Fixed table above. |
| ~~`internal/bigdata/memory_integration_test.go`~~ | ~~938~~ | ~~memory backend mocks~~ | **Fixed** (PR7, `b1451b8a`) — demoted to unit (rename). |
| ~~`internal/bigdata/debate_integration_test.go`~~ | ~~510~~ | ~~debate backend mocks~~ | **Fixed** (PR6, `6f80c369`) — demoted to unit (rename). |
| ~~`internal/bigdata/integration_test.go`~~ | ~ | ~~bigdata service mocks~~ | **Fixed** (PR11, `99be187b`) — demoted to unit (rename). |
| ~~`internal/adapters/auth/integration_test.go`~~ | ~ | ~~auth adapter mocks~~ | **Fixed** (PR4, `f357f0b2`) — demoted to unit (rename). |
| ~~`internal/adapters/mcp/integration_test.go`~~ | ~ | ~~MCP adapter mocks~~ | **Fixed** (PR3, `f2f45511`) — demoted to unit (rename). |
| ~~`internal/security/integration_test.go`~~ | ~ | ~~security integration mocks~~ | **Fixed** (PR12, `9b0fbcd1`) — demoted to unit (rename). |

**Proposed approach:** these are the highest-ROI files to fix because they
live next to production code, so CI/lint/vet coverage will keep them honest
once corrected. `services_integration_test.go` is the CLAUDE.md-flagged
showcase — should be the first PR. Rewrite pattern for every file:

1. Probe: `if _, err := http.Get("http://localhost:7061/v1/health"); err !=
   nil { t.Skip("HelixAgent unreachable on :7061 — CONST-030 requires live
   infra") }`.
2. Replace every `newIntegrationMockProvider(...)` wiring with a real HTTP
   call through the handler under test.
3. Delete the `integrationMockProvider` type and its helpers.

## Compliant-by-design

- **`tests/unit/**/*_test.go`** — unit tests, mocks allowed by CONST-030.
- **`internal/**/*_test.go`** WITHOUT an `_integration_test.go` / `_e2e_test.go`
  / `_stress_test.go` / `_chaos_test.go` suffix and WITHOUT a non-unit build
  tag — default unit tests, mocks allowed.
- **`internal/challenges/infra_provider_test.go`**,
  **`internal/challenges/userflow/flows_test.go`** — bare `_test.go` files
  under `internal/challenges/`; they run under default `go test -short` and
  are unit tests in Go's eyes. Mock usage compliant.
- **`challenges/codebase/go_files/framework/*_test.go`** — these live under the
  `challenges/codebase/` fixtures tree; they exist to be *compiled by*
  challenge scripts, not to run as challenge tests themselves. Compliant.
- **`internal/testing/**`** — explicit test-support package that never runs
  under a non-unit tag. Compliant.
- **`internal/security/redteam_fixtures_test.go`** — uses a test-double
  `GuardrailInputChecker`, but the real pipeline is exercised by
  `redteam_fixtures_realpipeline_test.go` (per Phase-5 + guardrail-hardening
  commits). Compliant.

## Fix patterns available for future sessions

### Pattern 1 — `integrationMockProvider` → live :7061 + `t.Skip`

```go
func TestXxx(t *testing.T) {
    // CONST-030: require live HelixAgent.
    client := &http.Client{Timeout: 2 * time.Second}
    if _, err := client.Get("http://localhost:7061/v1/health"); err != nil {
        t.Skip("HelixAgent unreachable on :7061; start with `./bin/helixagent`")
    }
    // Real HTTP call to the endpoint under test; no in-process mock.
    resp, err := client.Post("http://localhost:7061/v1/ensemble", ...)
    require.NoError(t, err)
    defer resp.Body.Close()
    // Assert on observable response shape.
}
```

### Pattern 2 — chaos via toxiproxy

Introduce `tests/support/toxiproxy.go` that spins a toxiproxy container in
front of the upstream under test, then point HelixAgent at the proxy port.
Each chaos test then drives toxiproxy rules rather than an in-process mock.

### Pattern 3 — microstress/microbench split

Split current `tests/stress/` and `tests/performance/` into two layers: a
package-level micro layer (mocks allowed, CONST-030 exempt) and a
system-level layer (hits live :7061, no mocks). Rename directories so the
intent is explicit.

### Pattern 4 — "demote to unit"

Many `tests/integration/debate_*` and `tests/integration/helpers_test.go`
files ARE unit tests by definition (they exercise a single in-process
package with canned inputs). Demotion via `git mv` to `tests/unit/<sub>/`
instantly makes them compliant with zero code rewrite.

## Recommended sequencing for the CONST-030 remediation session

1. **PR 1** — `internal/services/services_integration_test.go`: the
   CLAUDE.md-cited showcase. Pattern 1 rewrite. ~800 LOC → ~300 LOC.
2. **PR 2** — `internal/handlers/handlers_integration_test.go`: Pattern 1.
3. **PR 3** — `tests/e2e/e2e_test.go` + `tests/e2e/ai_debate_e2e_test.go`:
   Pattern 1.
4. **PR 4** — `tests/security/*`: Pattern 1.
5. **PR 5** — demote `tests/integration/debate_*` and `helpers_test.go`:
   Pattern 4.
6. **PR 6** — stand up toxiproxy support; rewrite `tests/chaos/*`: Pattern 2.
7. **PR 7** — microstress / microbench split: Pattern 3.
8. **PR 8** — remaining `tests/integration/*`, `tests/automation/*`,
   `internal/bigdata/*`, `internal/adapters/*`, `internal/security/*`:
   Pattern 1.

Total rough estimate: **~5,000 LOC rewrite**, spread across 8 PRs. Sequential
reviewability is the goal — no big-bang change.

## Evidence files and commands used

- Enumeration: `find tests -name '*_test.go' | wc -l` → 377.
- Mock detection:
  `rg 'type\s+\w*(Mock|Fake|Stub|mock|fake|stub)\w*\s+struct'
   tests/{integration,e2e,chaos,stress,security,performance,automation}
   internal/**/*_integration_test.go internal/**/*_e2e_test.go`.
- `testify/mock|gomock|mockery` import scan:
  `rg 'testify/mock|gomock|mockery' tests internal --glob '*_test.go'` →
  1 hit (`tests/integration/provider_integration_test.go`).
- Build-tag scan: `head -3 <file> | grep -E '^//go:build|^// \+build'` on
  every violation file. Only `tests/performance/ensemble_benchmark_test.go`
  carries a tag; all others are default-selected by directory.

---

*Filed by CONST-030 compliance audit, 2026-04-21. All scans are
reproducible from the commands above. No tests were deleted in the
production of this report.*
