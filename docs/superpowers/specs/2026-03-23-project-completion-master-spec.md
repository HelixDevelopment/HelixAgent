# HelixAgent Project Completion Master Spec

**Date:** 2026-03-23
**Version:** 1.0.0
**Status:** Draft
**Scope:** Full project audit remediation across 10 sub-projects in 5 phases

---

## Table of Contents

1. [Overview](#1-overview)
2. [Audit Findings Summary](#2-audit-findings-summary)
3. [Phase 1 — Safety & Stability (P0)](#3-phase-1--safety--stability-p0)
4. [Phase 2 — Code Health & Coverage (P0)](#4-phase-2--code-health--coverage-p0)
5. [Phase 3 — Security & Observability (P1)](#5-phase-3--security--observability-p1)
6. [Phase 4 — Performance & Resilience (P1-P2)](#6-phase-4--performance--resilience-p1-p2)
7. [Phase 5 — Documentation & Content (P2-P3)](#7-phase-5--documentation--content-p2-p3)
8. [Dependency Graph](#8-dependency-graph)
9. [Test Strategy](#9-test-strategy)
10. [Challenge Strategy](#10-challenge-strategy)
11. [Documentation Deliverables](#11-documentation-deliverables)
12. [Risk Mitigation](#12-risk-mitigation)
13. [Success Criteria](#13-success-criteria)

---

## 1. Overview

### Purpose

This spec defines the complete remediation plan to bring HelixAgent to 100% completion: zero dead code, zero broken tests, zero concurrency bugs, full security scanning, maximum test coverage, complete documentation, and production-ready content.

### Guiding Principles

1. **Rock-solid changes** — every fix MUST NOT break existing working functionality
2. **Real data** — no mocks in production, no false-positive tests
3. **100% coverage** — unit, integration, E2E, security, stress, benchmark for every component
4. **Full documentation** — every module, feature, API documented to nano detail
5. **Resource limits** — all test/challenge execution limited to 30-40% host resources
6. **No interactive processes** — all commands fully non-interactive, no sudo/root prompts

### Constraints

- No CI/CD pipelines (manual Makefile targets only)
- SSH only for Git operations
- Container-based builds for reproducibility
- HTTP/3 (QUIC) primary transport with Brotli compression
- Constitution compliance (26 mandatory rules)

---

## 2. Audit Findings Summary

### By Dimension

| Dimension | Issues Found | Critical | Action Items |
|-----------|-------------|----------|-------------|
| Concurrency Safety | 15 | 10 HIGH | Fix all race conditions, deadlocks, goroutine leaks |
| Dead Code | 15 packages | 35,311 LOC | Integrate or remove all unused code |
| Test Coverage | 3 empty dirs + empty subtests | ~2,329 skips | Fill gaps, fix false positives |
| Documentation | 9 minor gaps | 0 critical | Complete 4 missing docs/, 3 sparse, 2 diagrams |
| Security Scanning | Infrastructure ready | 0 critical | Execute scans, analyze, resolve findings |
| Monitoring/Metrics | Comprehensive infra | 0 critical | Create validation tests |
| Website/Content | No video recordings | Markdown-only | Extend content, update courses |
| Submodule Health | 14 missing adapters | 0 critical | Add adapters where needed |

### Total Remediation Items: ~127

---

## 3. Phase 1 — Safety & Stability (P0)

**Goal:** Eliminate all concurrency bugs, race conditions, deadlocks, and goroutine leaks.
**Blocks:** All other phases (unsafe code invalidates any testing)

### SP-1: Concurrency Safety Fixes

#### SP-1.1: Channel Safety (3 fixes)

**Fix 1: SSE Manager — Send on Closed Channel**
- **File:** `internal/notifications/sse_manager.go`
- **Problem:** `Stop()` closes channels while senders may still write, causing panic
- **Fix:** Use `sync.Once` for channel closure. Add a `closed` atomic flag checked before sends. Drain channels before closing.
- **Pattern:**
  ```go
  type SSEManager struct {
      // ...
      closeOnce sync.Once
      closed    atomic.Bool
  }
  func (m *SSEManager) Stop() {
      m.closeOnce.Do(func() {
          m.closed.Store(true)
          // close channels safely
      })
  }
  ```
- **Tests:** Unit test for concurrent Stop()/Send(), race test with `-race` flag
- **Challenge:** Add to `goroutine_lifecycle_challenge.sh`

**Fix 2: Kafka Transport — Unprotected Channel Close**
- **File:** `internal/notifications/kafka_transport.go:120`
- **Problem:** Direct `close(t.stopCh)` and `close(t.eventCh)` without double-close protection
- **Fix:** Wrap in `sync.Once`, add `atomic.Bool` closed guard
- **Tests:** Unit test for concurrent Close() calls
- **Challenge:** Add to `goroutine_lifecycle_challenge.sh`

**Fix 3: Kafka Streams — Untracked Goroutine**
- **File:** `internal/streaming/kafka_streams.go:109`
- **Problem:** Goroutine not added to WaitGroup, potential use-after-close on stopCh
- **Fix:** Add `wg.Add(1)` before goroutine launch, `defer wg.Done()` inside
- **Tests:** Unit test verifying graceful shutdown waits for goroutine

#### SP-1.2: Race Condition Fixes (5 fixes)

**Fix 4: Model Metadata Handler — Defensive Goroutine Parameter Passing**
- **File:** `internal/handlers/model_metadata.go:230`
- **Problem:** Background goroutine reads `h.service` which is set once in constructor (immutable). While not a true data race, passing service as a goroutine parameter is a defensive best practice that prevents future regressions if the field ever becomes mutable.
- **Fix:** Pass `service` as parameter to the background goroutine instead of capturing `h.service` via closure. This is a defensive improvement, not a critical race fix.
- **Tests:** Verify goroutine receives correct service reference

**Fix 5: Cache Service — Nested Map Sync**
- **File:** `internal/cache/cache_service.go:23`
- **Problem:** `userKeys` outer map protected by mutex, but inner maps are not
- **Fix:** Return copies of inner maps or protect with per-key mutexes. Alternatively, use `sync.Map` for inner maps.
- **Tests:** Concurrent `InvalidateByTags()` test

**Fix 6: MCP Connection Pool — Non-Atomic Boolean**
- **File:** `internal/mcp/connection_pool.go:79`
- **Problem:** `closed bool` checked without lock in concurrent paths
- **Fix:** Replace `bool` with `atomic.Bool`
- **Tests:** Concurrent Get()/Close() race test

**Fix 7: WebSocket Server — Lock Ordering**
- **File:** `internal/notifications/websocket_server.go:28`
- **Problem:** Inconsistent lock ordering between broadcast and registration
- **Fix:** Document and enforce lock ordering: always acquire clients lock before broadcast lock
- **Tests:** Concurrent register/broadcast test

**Fix 8: Integration Orchestrator — Disabled Lock**
- **File:** `internal/services/integration_orchestrator.go:28`
- **Problem:** `mu sync.RWMutex //nolint:unused` — lock exists but marked unused while workflows map is shared
- **Fix:** Remove `//nolint:unused`, add proper Lock()/Unlock() calls around workflows map access
- **Tests:** Concurrent workflow registration test

#### SP-1.3: Goroutine Lifecycle Fixes (4 fixes)

**Fix 9: Debate Service — Untracked Participant Goroutines**
- **File:** `internal/services/debate_service.go:886-1024`
- **Problem:** Panic in goroutine would skip `wg.Done()`, causing hang. Fire-and-forget channel close goroutine.
- **Fix:** Add `defer func() { if r := recover(); r != nil { ... } }()` inside goroutines. Track channel close goroutine with WaitGroup.
- **Tests:** Test with panicking provider mock

**Fix 10: Debate Service — Untracked HelixMemory Store**
- **File:** `internal/services/debate_service.go:2047`
- **Problem:** Fire-and-forget goroutine with independent context, not tracked
- **Fix:** Add to service's WaitGroup, coordinate shutdown with service lifecycle
- **Tests:** Shutdown ordering test

**Fix 11: Plugin Hot Reload — Goroutine Leak**
- **File:** `internal/plugins/hot_reload.go:70`
- **Problem:** `watchLoop` goroutine spawned without WaitGroup tracking
- **Fix:** Add `wg.Add(1)` before launch, `defer wg.Done()` inside, wait in Stop()
- **Tests:** Start/Stop lifecycle test, leak detection

**Fix 12: Debate Orchestrator — Unprotected ActiveDebate State**
- **File:** `internal/debate/orchestrator/orchestrator.go:49`
- **Problem:** `activeDebates` map protected, but `ActiveDebate` struct fields are not
- **Fix:** Add `sync.RWMutex` to `ActiveDebate` struct, protect field access
- **Tests:** Concurrent debate state modification test

#### SP-1.4: Minor Safety Fixes (3 fixes)

**Fix 13: Boot Manager — Runtime Lock Enforcement**
- **File:** `internal/services/boot_manager.go:89`
- **Fix:** Add `//go:checklocksignore` annotation or runtime assertion that caller holds lock

**Fix 14: Polling Store — Panic Recovery in Cleanup Loop**
- **File:** `internal/notifications/polling_store.go:196`
- **Fix:** Add `defer func() { if r := recover()... }()` in cleanup loop

**Fix 15: Circuit Breaker — Silent Listener Failure**
- **File:** `internal/llm/circuit_breaker.go:99`
- **Fix:** Return error instead of -1 when listener limit reached, log warning

#### SP-1 Tests Required

| Test Type | Count | Details |
|-----------|-------|---------|
| Unit | 15 | One per fix, testing the specific concurrency scenario |
| Integration | 5 | Lifecycle tests for SSE, Kafka, WebSocket, Plugin, Debate |
| Race Detection | 15 | All fixes verified with `go test -race` |
| Stress | 3 | High-concurrency tests for channel safety, map access, goroutines |
| Benchmark | 3 | Before/after performance for mutex vs atomic changes |

#### SP-1 Challenges

- **Update:** `goroutine_lifecycle_challenge.sh` — add 15 new assertions for all fixes
- **Update:** `race_condition_challenge.sh` — add channel safety, map sync, atomic bool tests
- **New:** `concurrency_safety_comprehensive_challenge.sh` — validates all 15 fixes end-to-end

---

## 4. Phase 2 — Code Health & Coverage (P0)

**Goal:** Eliminate dead code, fix false-positive tests, achieve maximum coverage.
**Depends on:** Phase 1 (safe code required for reliable tests)

### SP-2: Dead Code Integration or Removal

The 15 unused packages fall into two categories: **integrate** (connect to the system) or **remove** (if superseded/duplicate).

#### SP-2.1: Packages to INTEGRATE (connect to router/services)

These packages have substantial, valuable code that should be wired into the system:

**1. `internal/agentic` (1,350 LOC) — INTEGRATE**
- **Action:** Register handler in router.go, create `handlers/agentic_handler.go`
- **Endpoint:** `POST /v1/agentic/workflows`, `GET /v1/agentic/workflows/:id`
- **Integration point:** Wire into `cmd/helixagent/main.go` service initialization
- **Tests:** Unit (handler), integration (workflow execution), E2E (API call), challenge script
- **Docs:** Add to API reference, create user guide

**2. `internal/benchmark` (2,043 LOC) — INTEGRATE**
- **Action:** Register handler in router.go, create `handlers/benchmark_handler.go`
- **Endpoint:** `POST /v1/benchmark/run`, `GET /v1/benchmark/results`
- **Integration point:** Wire into service layer, connect to LLMsVerifier
- **Tests:** Unit, integration, E2E, stress (benchmark of benchmarks)
- **Docs:** Add to API reference, extend video course 27

**3. `internal/events` (1,102 LOC) — INTEGRATE**
- **Action:** Initialize EventBus in main.go, publish events from services (debate, provider, cache)
- **Integration point:** Replace ad-hoc notification patterns with EventBus
- **Tests:** Unit (pub/sub), integration (event flow), stress (high-throughput events)
- **Docs:** Architecture diagram update, developer guide

**4. `internal/graphql` (3,004 LOC) — INTEGRATE (feature-flagged)**
- **Action:** Add feature flag `GRAPHQL_ENABLED=true` to enable GraphQL endpoint
- **Endpoint:** `POST /v1/graphql` (behind feature flag, default: false for backward compat)
- **Integration point:** router.go conditional registration
- **Tests:** Unit (schema), integration (resolvers), E2E (queries)
- **Docs:** GraphQL API guide, schema documentation

**5. `internal/llmops` (5,156 LOC) — INTEGRATE**
- **Action:** Register handler, create `handlers/llmops_handler.go`
- **Endpoints:** `POST /v1/llmops/experiments`, `GET /v1/llmops/evaluations`, `POST /v1/llmops/prompts`
- **Integration point:** Wire into debate service for A/B experimentation
- **Tests:** Full suite — unit, integration, E2E, security
- **Docs:** LLMOps user guide, extend video course

**6. `internal/observability` (4,927 LOC) — INTEGRATE**
- **Action:** Initialize tracer/metrics in main.go, add middleware to Gin engine
- **Integration point:** `llm_middleware.go` wraps all LLM calls with tracing/metrics
- **Tests:** Unit (metric recording), integration (trace propagation), E2E (Prometheus scrape)
- **Docs:** Observability setup guide, monitoring dashboard guide

**7. `internal/planning` (3,234 LOC) — INTEGRATE**
- **Action:** Register handler, create `handlers/planning_handler.go`
- **Endpoints:** `POST /v1/planning/hiplan`, `POST /v1/planning/mcts`, `POST /v1/planning/tot`
- **Integration point:** Wire into agentic workflows as planning backend
- **Tests:** Unit, integration, E2E, benchmark (algorithm performance)
- **Docs:** Planning algorithms guide

**8. `internal/lsp` (3,730 LOC) — INTEGRATE or CONSOLIDATE**
- **Action:** Evaluate overlap with `services/lsp_manager`. If `internal/lsp` is more complete, replace `services/lsp_manager`. If not, forward from `internal/lsp` to `services/lsp_manager`.
- **Decision criteria:** Which has more features, better test coverage, cleaner API?
- **Tests:** Whichever survives gets full coverage
- **Docs:** LSP integration guide

#### SP-2.2: Packages to REMOVE (superseded/duplicate)

**9. `internal/embedding` (4,360 LOC) — REMOVE**
- **Reason:** Duplicate of `internal/embeddings` and superseded by `Embeddings/` submodule
- **Action:** Verify no imports exist, delete directory
- **Post-action:** Run `make build` to confirm no breakage

**10. `internal/embeddings` (2,824 LOC) — REMOVE**
- **Reason:** Superseded by `Embeddings/` extracted submodule with adapters
- **Action:** Verify no imports, delete directory
- **Post-action:** Run `make build` to confirm

**11. `internal/http` (1,995 LOC) — EVALUATE then INTEGRATE or CONSOLIDATE**
- **Reason:** Contains production QUIC/HTTP3 code: `pool.go` (465 LOC — HTTP connection pool) and `quic_client.go` (328 LOC — QUIC client using `quic-go`), plus tests (1,202 LOC). NOT test-only.
- **Action:** Evaluate overlap with `internal/transport/http3.go` and `internal/router/quic_server.go`. If `internal/http` provides unique QUIC pool functionality, integrate it as the canonical HTTP3 client pool. If fully duplicated, consolidate into `internal/transport/` and remove.
- **Decision criteria:** Which implementation is more complete, better tested, and aligned with Constitution's HTTP/3 mandate?
- **Post-action:** Run `make build` and verify all HTTP/3 tests pass

#### SP-2.3: Unused Adapters — INTEGRATE

**12-15. `internal/adapters/{background,helixqa,storage,vectordb}`**
- **Action for each:** Wire into service initialization in main.go
- `adapters/background` → connect to `internal/background/` task queue
- `adapters/helixqa` → connect to QA validation pipeline
- `adapters/storage` → connect to `Storage/` submodule
- `adapters/vectordb` → connect to `VectorDB/` submodule
- **Tests per adapter:** Unit (interface compliance), integration (real backend)

#### SP-2 Tests Required

| Test Type | Count | Details |
|-----------|-------|---------|
| Unit | 40+ | Per-handler, per-integration-point |
| Integration | 20+ | Service-to-handler, handler-to-backend |
| E2E | 10+ | Full API endpoint testing |
| Security | 8 | Auth, input validation for new endpoints |
| Benchmark | 5 | Performance of new integrations |

#### SP-2 Challenges

- **New:** `dead_code_elimination_challenge.sh` — verifies no unused packages remain
- **New:** `agentic_workflow_challenge.sh` — validates workflow execution
- **New:** `llmops_experiment_challenge.sh` — validates A/B experimentation
- **New:** `graphql_endpoint_challenge.sh` — validates GraphQL schema and resolvers
- **New:** `observability_integration_challenge.sh` — validates tracing and metrics
- **New:** `planning_algorithms_challenge.sh` — validates HiPlan, MCTS, ToT
- **Update:** `adapter_coverage_challenge.sh` — verify all adapters connected

### SP-3: Test Coverage Maximization

#### SP-3.1: Fix False-Positive Tests

**Fix 1: debate_integration/integration_test.go:195-200**
- **Problem:** 7 subtests with zero assertions
- **Fix:** Add real assertions testing topology selection behavior
- **Validation:** Run test, verify assertions execute

**Fix 2: tests/challenge/provider_autodiscovery_test.go**
- **Problem:** 7+ subtests with only `t.Logf`, no assertions
- **Fix:** Add `assert.NotEmpty`, `assert.NoError`, `require.True` for each subtest
- **Validation:** Run test, verify assertions fire

**Fix 3: tests/challenge/debate_group_test.go**
- **Problem:** Multiple empty subtests (ServerHealth, ProvidersRegistered, etc.)
- **Fix:** Add actual HTTP request + response validation for each
- **Validation:** Run with infrastructure up

**Fix 4: challenges/panoptic/internal/ai/testgen_test.go:154**
- **Problem:** `TestGenerateRandomTests_EmptyElements` fully commented out
- **Fix:** Uncomment and fix, or remove if no longer applicable

#### SP-3.2: Fill Empty Test Directories

**tests/unit/analytics/** — Create:
- `analytics_test.go` — ClickHouse analytics validation
- `metrics_collection_test.go` — Prometheus metrics
- `event_tracking_test.go` — Event analytics

**tests/unit/bigdata/** — Create:
- `bigdata_integration_test.go` — BigData component validation
- `infinite_context_test.go` — Context window management
- `knowledge_graph_test.go` — Graph operations

**tests/unit/knowledge/** — Create:
- `knowledge_store_test.go` — Knowledge persistence
- `knowledge_retrieval_test.go` — Retrieval accuracy

#### SP-3.3: Expand Minimal Test Directories

For each of `concurrency/`, `events/`, `http/`, `learning/`, `llm/`, `mcp/`, `notifications/`, `verifier/`:
- Analyze what functionality exists in corresponding `internal/` package
- Create comprehensive test suite covering all exported functions
- Target: 100% line coverage per package

#### SP-3.4: Submodule Test Coverage

**BuildCheck** (currently 20% test ratio):
- Add tests for all 5 source files
- Target: 100% function coverage

**SkillRegistry** (currently 50%):
- Add tests for skill registration, lookup, deregistration
- Target: 100% function coverage

**Models** (currently 67%):
- Add tests for all model types, validation, serialization
- Target: 100% function coverage

#### SP-3.5: Review and Fix Skipped Tests

- Audit all 430 files with `t.Skip` calls
- For each: determine if skip reason is still valid
- For invalid skips: fix the underlying issue and remove skip
- For valid skips: ensure skip message is descriptive with tracking issue
- Target: Reduce skip count by 30%+ where infra is available

#### SP-3 Tests Required

| Test Type | Count | Details |
|-----------|-------|---------|
| Unit | 50+ | New tests for empty/minimal directories |
| Integration | 15+ | Cross-package integration coverage |
| E2E | 5+ | End-to-end validation of fixed tests |
| Fuzz | 5+ | Fuzz testing for input validation paths |
| Benchmark | 5+ | Performance baselines for new tests |

#### SP-3 Challenges

- **New:** `test_coverage_gate_challenge.sh` — enforce minimum coverage thresholds per package
- **New:** `false_positive_detection_challenge.sh` — verify no tests pass without assertions
- **Update:** `coverage_gate_challenge.sh` — raise thresholds to match new coverage

---

## 5. Phase 3 — Security & Observability (P1)

**Goal:** Execute security scans, resolve all findings, validate monitoring pipeline.
**Depends on:** Phase 2 (need integrated code to scan)

### SP-4: Security Scanning Execution & Resolution

#### SP-4.1: Execute Snyk Scanning

**Step 1: Infrastructure Verification**
- Verify `docker/security/snyk/docker-compose.yml` is operational
- Ensure `SNYK_TOKEN` environment variable is set
- Start Snyk containers: `docker compose -f docker/security/snyk/docker-compose.yml --profile all up`

**Step 2: Run All Scan Types**
1. **Dependency scan:** `docker compose -f docker/security/snyk/docker-compose.yml run snyk-deps`
   - Analyzes go.mod, go.sum for known vulnerabilities
   - Output: `reports/snyk-deps.json`
2. **Code scan:** `docker compose -f docker/security/snyk/docker-compose.yml run snyk-code`
   - Static analysis for security patterns
   - Output: `reports/snyk-code.json`
3. **IaC scan:** `docker compose -f docker/security/snyk/docker-compose.yml run snyk-iac`
   - Docker/Compose/K8s configuration analysis
   - Output: `reports/snyk-iac.json`

**Step 3: Analyze and Resolve**
- Parse JSON reports for CRITICAL and HIGH severity findings
- For each finding:
  - If dependency vulnerability: upgrade dependency or add `.snyk` policy exception with justification
  - If code vulnerability: fix the code pattern
  - If IaC vulnerability: fix the configuration
- Re-run scans to verify all findings resolved

#### SP-4.2: Execute SonarQube Scanning

**Step 1: Infrastructure Setup**
- Start SonarQube: `docker compose -f docker/security/sonarqube/docker-compose.yml up -d`
- Wait for health check: `curl http://localhost:9000/api/system/status`
- Default credentials: admin/admin (change on first login via API)

**Step 2: Run Scanner**
- Generate coverage report: `go test -coverprofile=coverage.out ./...`
- Generate test report: `go test -json ./... > test-report.json`
- Run scanner: `docker compose -f docker/security/sonarqube/docker-compose.yml --profile scanner run sonar-scanner`

**Step 3: Analyze Dashboard**
- Access: `http://localhost:9000/dashboard?id=helixagent`
- Review: Code smells, bugs, vulnerabilities, security hotspots, coverage
- For each finding:
  - Code smell: refactor or document exception
  - Bug: fix
  - Vulnerability: fix immediately
  - Security hotspot: review and resolve

**Step 4: Quality Gate**
- Configure quality gate thresholds:
  - Coverage: >= 80%
  - Duplicated lines: < 3%
  - Reliability: A
  - Security: A
  - Maintainability: A

#### SP-4.3: Semgrep Scanning

**Step 1: Run Semgrep via MCP**
- Use the semgrep MCP server (already configured)
- Scan with default rulesets + Go-specific rules
- Focus areas: injection, auth bypass, crypto misuse, resource leaks

**Step 2: Analyze and Fix**
- Fix all HIGH and CRITICAL findings
- Document exceptions for false positives

#### SP-4 Tests Required

| Test Type | Count | Details |
|-----------|-------|---------|
| Security | 20+ | Validate each vulnerability fix |
| Integration | 5 | Scanning pipeline end-to-end |
| Automation | 3 | Automated scan execution scripts |

#### SP-4 Challenges

- **Update:** `snyk_automated_scanning_challenge.sh` — add finding resolution verification
- **Update:** `sonarqube_automated_scanning_challenge.sh` — add quality gate verification
- **New:** `security_resolution_challenge.sh` — verify all findings resolved

### SP-5: Monitoring & Metrics Testing

#### SP-5.1: Prometheus Metrics Validation Tests

Create `tests/integration/monitoring/`:

**`prometheus_metrics_test.go`**
- Test that all registered metrics are actually emitted
- Verify metric types (counter, gauge, histogram) are correct
- Test label cardinality stays within bounds
- Validate scrape endpoint returns valid Prometheus format

**`grafana_dashboard_test.go`**
- Test that dashboard JSON is valid
- Verify all referenced metrics exist
- Test alert rule thresholds fire correctly

**`opentelemetry_test.go`**
- Test trace propagation across HTTP/gRPC boundaries
- Verify span attributes match OpenTelemetry GenAI conventions
- Test exporter connectivity (OTLP, Jaeger, Zipkin)

#### SP-5.2: Metrics-Based Optimization Tests

**`metrics_optimization_test.go`**
- Collect baseline metrics (latency, throughput, error rate)
- Run load test
- Collect post-load metrics
- Assert metrics within acceptable thresholds:
  - P99 latency < 500ms for single provider
  - P99 latency < 2s for debate
  - Error rate < 1%
  - Memory growth < 10% under sustained load

#### SP-5.3: Health Check Validation

**`health_endpoint_test.go`**
- Test `/v1/health` returns correct overall status
- Test `/v1/health/providers/{id}` for each registered provider
- Test circuit breaker state transitions reflected in health
- Test health degrades when dependencies fail

#### SP-5 Tests Required

| Test Type | Count | Details |
|-----------|-------|---------|
| Unit | 10 | Metric registration, label validation |
| Integration | 15 | End-to-end metric flow, trace propagation |
| E2E | 5 | Full monitoring pipeline |
| Benchmark | 5 | Overhead measurement of observability |

#### SP-5 Challenges

- **Update:** `monitoring_dashboard_challenge.sh` — add metric validation, alert testing
- **New:** `observability_pipeline_challenge.sh` — end-to-end tracing validation
- **New:** `metrics_accuracy_challenge.sh` — verify metrics match actual system state

---

## 6. Phase 4 — Performance & Resilience (P1-P2)

**Goal:** Maximize lazy loading, add stress tests, ensure flash-speed responsiveness.
**Depends on:** Phases 1-3 (safe, clean, monitored code)

### SP-6: Stress & Integration Testing

#### SP-6.1: Comprehensive Stress Tests

Create `tests/stress/`:

**`api_stress_test.go`**
- 100 concurrent requests to `/v1/chat/completions`
- 500 concurrent requests to `/v1/health`
- 50 concurrent debate sessions
- Validate: no panics, no goroutine leaks, response times < SLA

**`provider_stress_test.go`**
- All 43 providers called simultaneously
- Circuit breaker activation under load
- Fallback chain execution under pressure
- Provider registry concurrent access

**`websocket_stress_test.go`**
- 1000 concurrent WebSocket connections
- Message broadcast under load
- Connection churn (rapid connect/disconnect)
- Memory consumption validation

**`database_stress_test.go`**
- 100 concurrent database operations
- Connection pool exhaustion and recovery
- Transaction deadlock detection
- Query timeout handling

**`cache_stress_test.go`**
- Redis connection pool under load
- Cache stampede protection
- Eviction under memory pressure
- Concurrent read/write patterns

**`debate_stress_test.go`**
- 25 simultaneous debate sessions
- Each with 5 participants x 5 rounds
- Memory tracking per session
- Goroutine count stability

#### SP-6.2: Integration Tests (Cross-Component)

Create/expand `tests/integration/`:

**`boot_integration_test.go`**
- Full boot sequence with all services
- Verify container orchestration flow
- Health check cascade validation
- Graceful shutdown ordering

**`provider_failover_test.go`**
- Primary provider failure → fallback chain activation
- Circuit breaker open/close cycle
- Score-based re-ranking after failure

**`debate_e2e_test.go`**
- Complete debate from API request to response
- All topologies (mesh, star, chain, tree)
- All voting methods (weighted, majority, borda, condorcet)
- Persistence verification (PostgreSQL)

**`rag_pipeline_test.go`**
- Document ingestion → chunking → embedding → retrieval
- Cross-provider embedding comparison
- Relevance scoring validation

**`memory_lifecycle_test.go`**
- Memory creation → retrieval → consolidation → deletion
- Entity graph updates
- Cross-session persistence

#### SP-6 Resource Limits (MANDATORY)

All stress tests MUST enforce:
```go
func init() {
    runtime.GOMAXPROCS(2) // 30-40% of typical 4-8 core host
}
```
Plus: `nice -n 19`, `ionice -c 3`, `-p 1` for test processes.

#### SP-6 Tests Required

| Test Type | Count | Details |
|-----------|-------|---------|
| Stress | 15+ | Per-component and cross-component |
| Integration | 10+ | Cross-service validation |
| E2E | 5+ | Full pipeline tests |
| Chaos | 3 | Random failure injection |
| Benchmark | 5 | Throughput and latency baselines |

#### SP-6 Challenges

- **New:** `stress_comprehensive_challenge.sh` — runs all stress tests with resource limits
- **New:** `integration_comprehensive_challenge.sh` — runs all integration tests
- **Update:** `resource_limits_challenge.sh` — verify all new tests respect limits

### SP-7: Documentation Completion

#### SP-7.1: Missing docs/ Directories (4 modules)

**LLMProvider/docs/**
- `API_REFERENCE.md` — Interface definition, methods, return types
- `ARCHITECTURE.md` — Circuit breaker, health monitoring, retry patterns
- `EXAMPLES.md` — Provider implementation examples
- `MIGRATION_GUIDE.md` — How to migrate from direct provider usage

**BackgroundTasks/docs/**
- `ARCHITECTURE.md` — Task queue, worker pool, resource monitor
- `INTEGRATION_GUIDE.md` — How to use with HelixAgent
- `TASK_LIFECYCLE.md` — Task states, transitions, stuck detection

**ConversationContext/docs/**
- `ARCHITECTURE.md` — Compression strategies, event sourcing
- `KAFKA_INTEGRATION.md` — Kafka replay, streaming
- `EXAMPLES.md` — Context management code examples

**DebateOrchestrator/docs/**
- `PROTOCOL_GUIDE.md` — 8-phase protocol detailed walkthrough
- `VOTING_METHODS.md` — All 6 voting methods with examples
- `TOPOLOGY_EXAMPLES.md` — Mesh, star, chain, tree configurations

#### SP-7.2: Sparse docs/ Expansion (3 modules)

**Models/docs/**
- `SCHEMA.md` — All data types with field descriptions
- `ENUMS.md` — Enumeration types and valid values
- `TYPE_DEFINITIONS.md` — Go type definitions reference

**ToolSchema/docs/**
- `API_REFERENCE.md` — Schema definition API
- `VALIDATION_GUIDE.md` — Validation rules and constraints
- `EXAMPLES.md` — Tool schema examples for each tool type

**SkillRegistry/docs/**
- `SKILL_DEFINITION.md` — How to define a skill
- `REGISTRY_API.md` — Registration and lookup API
- `EXAMPLES.md` — Skill implementation examples

#### SP-7.3: Diagram Rendering (2 diagrams)

- Render `docs/diagrams/src/architecture.puml` to SVG/PNG
- Render `docs/diagrams/src/debate-orchestration-flow.puml` to SVG/PNG
- Use PlantUML container or CLI tool

#### SP-7.4: TODO/FIXME Resolution (8 items)

For each TODO/FIXME in source code (verified file locations):
1. `internal/mcp/validation/validator.go` — Complete validation logic for MCP tool schemas
2. `internal/mcp/config/generator_container.go` — Finish container config generation edge cases
3. `internal/mcp/config/generator_full.go` — Complete full config generation for all MCP servers
4. `internal/debate/agents/templates.go` — Finalize agent template system
5. `internal/debate/evaluation/benchmark_bridge.go` — Complete benchmark bridge integration
6. `internal/debate/comprehensive/analysis.go` — Finish comprehensive analysis implementation
7. `internal/debate/comprehensive/agents_specialized.go` — Complete specialized agent types
8. `internal/handlers/openai_compatible_test.go` — Fix or complete commented test scenarios

### SP-8: Lazy Loading & Non-Blocking Expansion

#### SP-8.1: New Lazy Loading Sites

Identify all eager initialization sites and convert to lazy where beneficial:

**Candidate 1: Handler Initialization**
- Currently: All handlers created at startup
- Change: Lazy-initialize handlers on first request to their route group
- Pattern: `sync.Once` per handler group

**Candidate 2: Formatter Registry**
- Currently: All 32+ formatters loaded at startup
- Change: Lazy-load formatters on first format request for that language
- Pattern: `LazyProvider` pattern from `internal/llm/lazy_provider.go`

**Candidate 3: MCP Adapter Pool**
- Currently: All 45+ adapters initialized at startup
- Change: Lazy-initialize on first MCP request
- Pattern: `sync.Once` per adapter

**~~Candidate 4: Database Migrations~~ — REJECTED**
- **Reason:** Lazy-running migrations on first database access introduces a "thundering herd" problem where many concurrent requests arrive, all try to trigger migrations via `sync.Once`, the first one blocks while migrations execute (potentially minutes), and every other request blocks waiting. This creates unacceptable latency spikes and moves startup failures to runtime. Database migrations MUST remain at startup time.

#### SP-8.2: Semaphore Mechanisms

**New: Provider Concurrency Limiter**
- Limit concurrent requests per provider (configurable per-provider)
- Pattern: `semaphore.Weighted` from `golang.org/x/sync`
- Default: 10 concurrent per provider, 50 total

**New: Debate Session Limiter**
- Limit concurrent active debates
- Pattern: `semaphore.Weighted`
- Default: 5 concurrent debates

**New: Background Task Throttle**
- Limit concurrent background tasks by resource consumption
- Pattern: `semaphore.Weighted` + resource monitor
- Default: 30% CPU, 40% memory

#### SP-8.3: Non-Blocking Patterns

**Convert blocking operations to non-blocking:**
1. Provider health checks — fire-and-forget with channel callback
2. Cache warming — background goroutine with rate limiting
3. Metric collection — non-blocking channel send with select+default
4. Event publishing — buffered channel with overflow discard

#### SP-8 Tests Required

| Test Type | Count | Details |
|-----------|-------|---------|
| Unit | 15 | Per lazy-loading site, per semaphore |
| Integration | 5 | Lazy init under load, semaphore fairness |
| Stress | 5 | Semaphore under extreme concurrency |
| Benchmark | 5 | Before/after startup time, memory usage |

#### SP-8 Challenges

- **Update:** `lazy_loading_validation_challenge.sh` — add new lazy sites
- **New:** `semaphore_mechanisms_challenge.sh` — validate all concurrency limiters
- **New:** `non_blocking_operations_challenge.sh` — verify no blocking operations in hot paths

---

## 7. Phase 5 — Documentation & Content (P2-P3)

**Goal:** Complete all documentation, extend video courses, update website.
**Depends on:** Phases 1-4 (document what's actually implemented)

### SP-9: Video Course Production & Website Update

#### SP-9.1: Video Course Content Extension

**New courses to create (markdown scripts):**
- Course 51: Dead Code Detection and Cleanup
- Course 52: Concurrency Safety in Go
- Course 66: Agentic Workflows Deep Dive
- Course 67: LLMOps Experimentation
- Course 68: Planning Algorithms (HiPlan, MCTS, ToT)
- Course 69: GraphQL API for HelixAgent
- Course 70: Observability Pipeline Setup

**Update existing courses:**
- Course 06 (Testing): Add stress test section, false-positive detection
- Course 10 (Security): Add Snyk/SonarQube execution results
- Course 18 (Security Scanning): Add resolution workflow
- Course 25 (Lazy Loading): Add new lazy loading sites
- Course 31 (SonarQube): Add quality gate configuration
- Course 32 (Snyk): Add automated resolution
- Course 49 (Monitoring): Add metrics validation tests
- Course 61 (Goroutine Safety): Add all 15 concurrency fixes

#### SP-9.2: Website Content Update

**Update existing pages:**
- `features.html` — Add agentic workflows, LLMOps, planning, GraphQL
- `docs/architecture.html` — Update with dead code removal, new integrations
- `docs/security.html` — Add scan results and resolution documentation
- `docs/optimization.html` — Add lazy loading expansion, semaphore mechanisms

**New pages:**
- `docs/agentic.html` — Agentic workflow documentation
- `docs/llmops.html` — LLMOps experimentation guide
- `docs/planning.html` — Planning algorithms guide
- `docs/graphql.html` — GraphQL API documentation

#### SP-9.3: User Manual Extension

**Update existing manuals:**
- Manual 10 (Security): Add scan resolution procedures
- Manual 11 (Performance): Add semaphore and lazy loading
- Manual 17 (Security Scanning): Add end-to-end workflow
- Manual 19 (Concurrency): Add race condition fixes
- Manual 20 (Testing): Add stress testing guide

**New manuals:**
- Manual 34: Agentic Workflows Guide
- Manual 35: LLMOps Experimentation Guide
- Manual 36: Planning Algorithms Guide
- Manual 37: GraphQL API Guide
- Manual 38: Complete Observability Guide

### SP-10: SDK & Supplementary Content

#### SP-10.1: SDK Updates

**Web SDK (`sdk/web/`):**
- Add agentic workflow client methods
- Add LLMOps experiment client methods
- Add planning endpoint client methods
- Add GraphQL client support
- Update README.md with new endpoints

**Python SDK (`sdk/python/`):**
- Mirror all web SDK additions
- Add async support for streaming endpoints
- Update pyproject.toml version

**CLI SDK (`sdk/cli/`):**
- Add commands for new endpoints
- Update help text

#### SP-10.2: Diagram Updates

**New diagrams to create:**
- `agentic-workflow-flow.mmd` — Agentic workflow execution
- `llmops-experiment-flow.mmd` — A/B experiment lifecycle
- `planning-algorithm-comparison.mmd` — HiPlan vs MCTS vs ToT
- `dead-code-integration-map.mmd` — Before/after integration map
- `semaphore-architecture.puml` — Concurrency limiter architecture

**Render all new + 2 existing unrendered diagrams to SVG/PNG**

#### SP-10.3: SQL Schema Updates

For new integrated features, add:
- `sql/schema/agentic_workflows.sql` — Workflow state persistence
- `sql/schema/llmops_experiments.sql` — Experiment data storage
- `sql/schema/planning_sessions.sql` — Planning session tracking

Update `sql/schema/complete_schema.sql` with all additions.

#### SP-10.4: Course Lab Exercises

Add labs for new features:
- `LAB_14_agentic_workflows.md`
- `LAB_15_llmops_experiments.md`
- `LAB_16_stress_testing.md`
- `LAB_17_security_scanning.md`

Add assessments:
- `QUIZ_07_new_features.md`
- `QUIZ_08_security_and_performance.md`

---

## 8. Dependency Graph

```
Phase 1 (Safety)
  └── SP-1: Concurrency Fixes ──────────────┐
                                              ▼
Phase 2 (Code Health)                    ┌─────────┐
  ├── SP-2: Dead Code Integration ───────┤ Phase 3 │
  └── SP-3: Test Coverage Maximization ──┤         │
                                          └────┬────┘
Phase 3 (Security & Observability)             │
  ├── SP-4: Security Scanning ─────────────────┤
  └── SP-5: Monitoring Tests ──────────────────┤
                                                ▼
Phase 4 (Performance & Resilience)        ┌─────────┐
  ├── SP-6: Stress & Integration Tests ───┤ Phase 5 │
  ├── SP-7: Documentation Completion ─────┤         │
  └── SP-8: Lazy Loading Expansion ───────┤         │
                                           └────┬────┘
Phase 5 (Content)                               │
  ├── SP-9: Video & Website ────────────────────┘
  └── SP-10: SDK & Supplementary
```

**Critical path:** SP-1 → SP-2 → SP-4 → SP-6 → SP-9

**Parallelizable within phases:**
- Phase 2: SP-2 and SP-3 can run in parallel
- Phase 3: SP-4 and SP-5 can run in parallel
- Phase 4: SP-6, SP-7, SP-8 can all run in parallel
- Phase 5: SP-9 and SP-10 can run in parallel

---

## 9. Test Strategy

### Test Types Matrix (per CLAUDE.md mandatory standards)

| Test Type | SP-1 | SP-2 | SP-3 | SP-4 | SP-5 | SP-6 | SP-7 | SP-8 | Total |
|-----------|------|------|------|------|------|------|------|------|-------|
| Unit | 15 | 40 | 50 | 10 | 10 | 5 | 0 | 15 | 145 |
| Integration | 5 | 20 | 15 | 5 | 15 | 10 | 0 | 5 | 75 |
| E2E | 0 | 10 | 5 | 2 | 5 | 5 | 0 | 0 | 27 |
| Security | 0 | 8 | 0 | 20 | 0 | 0 | 0 | 0 | 28 |
| Stress | 3 | 0 | 0 | 0 | 0 | 15 | 0 | 5 | 23 |
| Benchmark | 3 | 5 | 5 | 0 | 5 | 5 | 0 | 5 | 28 |
| Race | 15 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 15 |
| Fuzz | 0 | 0 | 5 | 0 | 0 | 0 | 0 | 0 | 5 |
| Chaos | 0 | 0 | 0 | 0 | 0 | 3 | 0 | 0 | 3 |
| Automation | 0 | 0 | 0 | 3 | 0 | 0 | 0 | 0 | 3 |
| **Total** | **41** | **83** | **80** | **40** | **35** | **43** | **0** | **30** | **352** |

### Test Naming Convention
```
Test<Struct>_<Method>_<Scenario>
```
Examples:
- `TestSSEManager_Stop_ConcurrentSendersNoPanic`
- `TestAgenticHandler_CreateWorkflow_Success`
- `TestPrometheusMetrics_ProviderLatency_RecordedCorrectly`

### Resource Limits for All Tests
```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -p 1 -race -count=1 ./...
```

---

## 10. Challenge Strategy

### New Challenges (16)

| Challenge | Tests | Sub-Project |
|-----------|-------|-------------|
| `concurrency_safety_comprehensive_challenge.sh` | 20 | SP-1 |
| `dead_code_elimination_challenge.sh` | 15 | SP-2 |
| `agentic_workflow_challenge.sh` | 12 | SP-2 |
| `llmops_experiment_challenge.sh` | 10 | SP-2 |
| `graphql_endpoint_challenge.sh` | 10 | SP-2 |
| `observability_integration_challenge.sh` | 15 | SP-2 |
| `planning_algorithms_challenge.sh` | 12 | SP-2 |
| `test_coverage_gate_challenge.sh` | 20 | SP-3 |
| `false_positive_detection_challenge.sh` | 10 | SP-3 |
| `security_resolution_challenge.sh` | 15 | SP-4 |
| `observability_pipeline_challenge.sh` | 12 | SP-5 |
| `metrics_accuracy_challenge.sh` | 10 | SP-5 |
| `stress_comprehensive_challenge.sh` | 25 | SP-6 |
| `integration_comprehensive_challenge.sh` | 15 | SP-6 |
| `semaphore_mechanisms_challenge.sh` | 12 | SP-8 |
| `non_blocking_operations_challenge.sh` | 10 | SP-8 |

### Updated Challenges (8)

| Challenge | New Tests Added | Sub-Project |
|-----------|----------------|-------------|
| `goroutine_lifecycle_challenge.sh` | +15 | SP-1 |
| `race_condition_challenge.sh` | +10 | SP-1 |
| `adapter_coverage_challenge.sh` | +8 | SP-2 |
| `coverage_gate_challenge.sh` | +5 | SP-3 |
| `snyk_automated_scanning_challenge.sh` | +5 | SP-4 |
| `sonarqube_automated_scanning_challenge.sh` | +5 | SP-4 |
| `monitoring_dashboard_challenge.sh` | +10 | SP-5 |
| `lazy_loading_validation_challenge.sh` | +8 | SP-8 |

### Challenge Framework Compliance
All new challenges MUST:
- Source `challenge_framework.sh`
- Use `record_assertion()` for every test
- Set `set -e` for strict error handling
- Use `GOMAXPROCS=2`, `nice -n 19`, `ionice -c 3` for resource limits
- Validate actual behavior, not just return codes
- Output JSON results to `$OUTPUT_DIR/results/`

---

## 11. Documentation Deliverables

### Per Sub-Project Documentation

| Sub-Project | New Docs | Updated Docs |
|-------------|----------|-------------|
| SP-1 | Concurrency safety guide | goroutine-lifecycle.puml, CLAUDE.md |
| SP-2 | 7 handler docs, 4 adapter docs, API refs | MODULES.md, AGENTS.md, API_REFERENCE.md |
| SP-3 | Test coverage report | Testing guide, CLAUDE.md coverage section |
| SP-4 | Security scan results, resolution log | SECURITY.md, scanning guides |
| SP-5 | Metrics validation guide | Monitoring guide, observability docs |
| SP-6 | Stress test guide, load test report | Performance cookbook |
| SP-7 | 12 new doc files for 7 modules | Diagrams, TODO resolution |
| SP-8 | Lazy loading expansion guide | Architecture diagrams |
| SP-9 | 7 new courses, 8 updated courses, 4 web pages | All video content |
| SP-10 | 3 SDK updates, 5 new diagrams, 3 SQL schemas, 4 labs, 2 quizzes | Complete catalog |

### Documentation Synchronization
Per Constitution rule "Documentation Synchronization":
- Every change to CLAUDE.md → reflected in AGENTS.md and CONSTITUTION.md
- Every new endpoint → added to API_REFERENCE.md, openapi.yaml, SDKs
- Every new module → added to MODULES.md
- Every new challenge → added to CLAUDE.md challenges section

---

## 12. Risk Mitigation

### Risk 1: Breaking Existing Functionality
- **Mitigation:** Run full test suite after every fix (`make test`)
- **Mitigation:** Run all challenges after each sub-project (`./challenges/scripts/run_all_challenges.sh`)
- **Mitigation:** Commit atomically per fix, not batched
- **Validation:** `make ci-validate-all` after each phase

### Risk 2: Resource Exhaustion During Tests
- **Mitigation:** All tests enforce GOMAXPROCS=2, nice -n 19, ionice -c 3
- **Mitigation:** Container memory/CPU limits in all compose files
- **Mitigation:** `-p 1` flag for sequential test package execution

### Risk 3: Security Scan False Positives
- **Mitigation:** Review each finding manually before acting
- **Mitigation:** Use `.snyk` policy for acknowledged false positives
- **Mitigation:** Re-scan after fixes to confirm resolution

### Risk 4: Dead Code Integration Conflicts
- **Mitigation:** Integrate one package at a time
- **Mitigation:** Run `make build` after each integration
- **Mitigation:** Feature-flag new endpoints (default: disabled for backward compat)

### Risk 5: Interactive Process Requests
- **Mitigation:** All SSH uses key-based auth with ssh-agent
- **Mitigation:** All container operations through Containers module adapter
- **Mitigation:** All secrets via .env files, never interactive prompts
- **Mitigation:** SonarQube admin password set via API, not interactive

### Risk 6: External Consumers of Dead Code
- **Problem:** While no Go imports reference dead packages, external tools, scripts, Makefiles, or documentation may reference them
- **Mitigation:** Before removing any package, search for references in Makefiles, shell scripts (*.sh), YAML configs, and documentation files — not just Go imports
- **Mitigation:** Run `grep -r "internal/embedding" --include="*.sh" --include="*.yml" --include="*.yaml" --include="Makefile"` for each package before deletion
- **Mitigation:** Update all documentation references after removal

### Risk 7: Lazy Loading Changing Startup Error Behavior
- **Problem:** Converting eager initialization to lazy loading means errors that currently surface at startup will instead surface on first use, potentially in production
- **Mitigation:** Add health check endpoints that exercise lazy-loaded components during startup verification
- **Mitigation:** The startup verification pipeline already calls providers — extend it to exercise lazy-loaded formatters, MCP adapters, and handlers
- **Mitigation:** Keep database migrations eager (not lazy) per SP-8.1 Candidate 4 rejection

### Risk 8: Container Orchestration for New Services
- **Problem:** Newly integrated packages (agentic, llmops, planning, graphql, observability) may need containerized dependencies (Prometheus for observability, database for llmops experiments)
- **Mitigation:** Each new service with external dependencies MUST have a container configuration entry in the Containers module adapter at `internal/adapters/containers/adapter.go`
- **Mitigation:** Add compose service definitions to appropriate docker-compose files
- **Mitigation:** Follow the Constitution's "Mandatory Container Orchestration Flow" — all container orchestration through HelixAgent binary boot sequence

---

## 13. Success Criteria

### Phase 1 Complete When:
- [ ] All 15 concurrency issues fixed (14 real races + 1 defensive improvement)
- [ ] `go test -race ./...` passes with zero data races
- [ ] `goroutine_lifecycle_challenge.sh` passes with new assertions
- [ ] `race_condition_challenge.sh` passes with new assertions
- [ ] No goroutine leaks detected under 10-minute stress test

### Phase 2 Complete When:
- [ ] Zero unused packages in `internal/`
- [ ] All integrated packages have handlers registered in router
- [ ] All empty/false-positive test subtests have real assertions
- [ ] `tests/unit/analytics/`, `bigdata/`, `knowledge/` have test files
- [ ] BuildCheck, SkillRegistry, Models at 100% function coverage
- [ ] `dead_code_elimination_challenge.sh` passes

### Phase 3 Complete When:
- [ ] Snyk reports zero CRITICAL/HIGH unresolved findings
- [ ] SonarQube quality gate passes (A rating across all dimensions)
- [ ] Semgrep reports zero CRITICAL findings
- [ ] All Prometheus metrics validated by automated tests
- [ ] OpenTelemetry traces propagate end-to-end verified by tests

### Phase 4 Complete When:
- [ ] All stress tests pass under resource limits
- [ ] API handles 100 concurrent requests without degradation
- [ ] All 7 module docs/ directories populated
- [ ] All TODO/FIXME comments resolved or documented
- [ ] All new lazy loading sites have sync.Once tests
- [ ] All semaphore mechanisms tested under load

### Phase 5 Complete When:
- [ ] 7 new video courses created
- [ ] 8 existing courses updated
- [ ] 4 new website pages live
- [ ] SDKs updated with all new endpoints
- [ ] 5 new architecture diagrams rendered
- [ ] 3 new SQL schemas created
- [ ] 4 new lab exercises, 2 new quizzes
- [ ] All documentation synchronized (CLAUDE.md ↔ AGENTS.md ↔ CONSTITUTION.md)

### Ultimate Success:
- [ ] `make ci-validate-all` passes
- [ ] `./challenges/scripts/run_all_challenges.sh` passes (all 482 existing + 16 new)
- [ ] `make test-coverage` shows 100% on all critical packages
- [ ] Zero dead code, zero broken tests, zero unresolved security findings
- [ ] Complete documentation for every module, feature, and API
- [ ] CLAUDE.md, AGENTS.md, and CONSTITUTION.md fully synchronized with all changes
- [ ] All new challenge scripts pass syntax validation and dry-run

---

*This spec is the single source of truth for the HelixAgent Project Completion initiative.*
*All 10 sub-projects, 5 phases, ~349 new tests, 16 new challenges, 8 updated challenges, and comprehensive documentation covered.*
