# HelixAgent — Unfinished-Work Report & Phased Implementation Plan

**Date:** 2026-04-18 (opened), 2026-04-19 (progress update)
**Author:** Compiled by Claude Opus 4.7 (1M context) for Милош Васић
**Scope:** Entire HelixAgent monorepo (main app + 41 extracted modules + Toolkit + LLMsVerifier)
**Sources:** Direct repo scan, four parallel codebase exploration passes, `git push` telemetry (GitHub Dependabot), `docs/issues/fixed/BUGFIXES.md`, Constitution v1.2.0, CLAUDE.md, AGENTS.md

> **Governance:** This plan adheres strictly to the Constitution's 26 rules. It is a *delivery contract*, not a brainstorming document. Each phase declares its exit criteria; no phase is "done" until every gate in its exit criteria passes. Every rule below is mandatory and non-negotiable unless explicitly deprecated in a future Constitution revision.

## Session Progress — 2026-04-19

The following gaps have been **closed** during the 2026-04-18 / 2026-04-19 execution session. Evidence in commit SHAs and BUGFIXES.md issues.

| Gap | Status | Closure commit(s) | BUGFIX |
|---|---|---|---|
| G2 Flaky `ProviderRegistry_ConcurrentAccess` | **Closed** | `cf31c819` | #12 |
| G7 Goroutine leak: `debate_integration.adaptedProvider.CompleteStream` | **Closed** | `2b4ef1a3` | #14 |
| G7 Data race + leak: `LazyProvider.createProviderWithContext` | **Closed** | `2393b019` | #15 |
| G10 TLS posture: unconditional `InsecureSkipVerify` + `curl -sk` | **Closed** | `b6c8c20b` | #16 |
| G11 `govulncheck` Makefile target | Verified **already present** (`make deps-scan`) | — | — |
| G12 Snyk + SonarQube compose | Verified **already present** (`docker-compose.security.yml`) | — | — |
| G13 Build broken — HelixQA `pkg/helixqa` missing in LLMsVerifier | **Closed** | `7e2d21c4` (+ LLMsVerifier `b49a08b8`) | #13 |
| G1 partial (Go subset only): pgx/v5 CVEs | **Closed** (4 Go CVEs → 2; pgx/v5 5.7.6 → 5.9.0) | `e9eb9ffd` | — |
| G9 Command-exec hardening (regression floor) | **Closed** (baseline + challenge; by-design sites documented) | `a578f951` | — |
| P0 Foundation scripts + Makefile targets | **Shipped** | `855fdd84` | — |

**Net deltas since session start:**

- Go-level CVEs (govulncheck): **4 → 2** (remaining: 2 docker/docker, no upstream fix, mitigated architecturally — documented in `docs/security/dependabot-triage-2026-Q2.md`).
- Skipped/flaky services tests: **1 → 0** (+ 4 new regression tests).
- Broken compilation: `go build ./...` was failing at session start (missing `pkg/helixqa`); now clean.
- Production goroutine leaks closed: **2 of ~10** identified.
- Production TLS violations closed: **1 of 1** (unconditional `InsecureSkipVerify` in `startup.go`).
- New Makefile targets: **+4** (`repo-health`, `coverage-floor`, `metrics-snapshot`, `security-gates-all`).
- New challenges: **+3** (`repo_hygiene_challenge`, `tls_posture_challenge`, `exec_hygiene_challenge`).
- BUGFIXES.md entries: **+5** (#12–#16).

**Still open (prioritised):**

- G1 remainder — 147 non-Go Dependabot CVEs on the vasic-digital mirror require UI-side triage (scoped API token).
- G3/G4/G5 — bulk test-type coverage expansion across 26+ modules.
- G6 — 8 thin-unit-test modules (BuildCheck, Models, ToolSchema, ConversationContext, BackgroundTasks, SelfImprove, Planning, LLMOps).
- G7 remaining — ~8 more goroutine hotspots flagged by the initial audit; most are either already correctly structured (see `circuit_breaker.go:167` — defensive flag, code is fine) or need per-file review.
- G8 — the `debate_performance_optimizer.go` cache was verified already-bounded; audit was pessimistic.
- G14, G13 (original doc numbering) — actual video recordings, website modernisation.
- G17 — `cli_agents/bridle` orphan sub-submodule.

---

---

## 0. Executive Summary

### 0.1 Current State (What Is Healthy)

| Domain | Status | Evidence |
|---|---|---|
| Monorepo structure | Healthy | 41 extracted modules with individual `go.mod`, CLAUDE.md, AGENTS.md, README.md |
| Per-module documentation | 39 / 39 (100 %) | All required docs present in every own-module |
| User manuals | 47 sequential guides | `Website/user-manuals/01-getting-started.md` … `47-stress-testing-guide.md` |
| Video-course *scripts* | 84+ markdown scripts | `Website/video-courses/course-*.md`, `video-course-*.md` |
| Diagrams | 40+ rendered | PlantUML (12) + Mermaid (28) → SVG/PNG/PDF in `docs/diagrams/output/` |
| SQL migrations | Versioned | `internal/database/migrations/001…014_*.sql` |
| Challenge scripts | 653 scripts | `./challenges/scripts/*.sh` including all CLAUDE.md-mandated criticals |
| Test type breadth | 10 present | unit, integration, E2E, security, stress, chaos, bench, fuzz, race, load |
| Dependabot / CI hygiene | Compliant | `.github/dependabot.yml.disabled`; **no** workflows / Jenkinsfile / gitlab-ci |
| Constitution sync | Current | CONSTITUTION.md / CLAUDE.md / AGENTS.md / CONSTITUTION.json all touched 2026-04-16 |
| Git remote topology | Healthy | 4 remotes (github, gitlab, githubhelixdevelopment, upstream) all at `cfb67543` (2026-04-18) |

### 0.2 Critical Gaps (What Must Be Closed)

| # | Gap | Severity | Owner Phase |
|---|---|---|---|
| **G1** | **149 Dependabot CVEs on `vasic-digital/HelixAgent`** (6 critical, 58 high, 70 moderate, 15 low) reported by GitHub during last push | CRITICAL | **P1** |
| G2 | Flaky skipped test `TestServicesIntegration_ProviderRegistry_ConcurrentAccess` (`internal/services/services_integration_test.go:467`) | MAJOR | P2 |
| G3 | 26 modules have **zero stress tests** (Auth, BackgroundTasks, Benchmark, BuildCheck, ConversationContext, Database, DocProcessor, EventBus, Formatters, LLMOps, LLMOrchestrator, Memory, MCP_Module, Models, Observability, Optimization, Planning, Plugins, RAG, Security, SelfImprove, SkillRegistry, Storage, Streaming, ToolSchema, VectorDB, VisionEngine) | CRITICAL | P3 |
| G4 | **All 41 modules have zero load tests** (only 1 load test file exists repo-wide: `tests/load/load_test.go`) | CRITICAL | P3 |
| G5 | ~38 modules have no dedicated race-detection tests; fuzz only in 18 files | HIGH | P3 |
| G6 | Modules with < 10 unit test files: BuildCheck (1), Models (2), ToolSchema (3), ConversationContext (3), BackgroundTasks (4), SelfImprove (7), Planning (8), LLMOps (9) | HIGH | P2 |
| G7 | ~10 goroutine launches lack WaitGroup / context cancellation (circuit_breaker.go:167, provider_bridge.go:84, lazy_provider.go:172, debate_performance_optimizer.go:169, ensemble.go) | HIGH | P4 |
| G8 | Unbounded `sync.Map` without documented eviction in `debate_performance_optimizer.go` cache (other two flagged have Phase-3 admission control already) | MEDIUM | P4 |
| G9 | Command-exec hotspots needing sandbox/arg-sanitization review (`internal/clis/agents/claude_code/tool_executor.go`, `internal/tools/sandbox/sandbox.go`, `internal/tools/gittools/autocommit.go`) | HIGH | P1 |
| G10 | `http3_client.go` TLS `InsecureSkipVerify` state unaudited | MEDIUM | P1 |
| G11 | `govulncheck` absent from Makefile; no `go mod audit` hook | HIGH | P1 |
| G12 | Snyk + SonarQube *configured* but **no docker-compose for local scanning**; CLAUDE.md requires compose-accessible infra | HIGH | P1 |
| G13 | Video-*course scripts* exist (84+) but **no actual video files** (.mp4/.webm absent) | MEDIUM | P6 |
| G14 | Website uses vanilla HTML/CSS/JS — no static site generator; limited navigation, no search, no CMS | MEDIUM | P6 |
| G15 | 16 / 39 modules lack a `tests/` directory at the module root (tests may be colocated `*_test.go`, but Constitution implies a dedicated dir per module for stress/e2e/etc.) | MEDIUM | P3 |
| G16 | One vendor stub: `HelixQA/tools/opensource/chroma/go/pkg/sysdb/metastore/s3/impl.go:363` returns `errors.New("not implemented")` for `DeleteOldVersionFiles` — cleanup never runs | LOW (vendor) | P5 |
| G17 | `cli_agents/bridle` sub-submodule has orphaned path in `.gitmodules` (`plugins/skill-enhancers/axiom`) causing recursive operations to error | LOW | P0 |
| G18 | 4 submodules fail to fetch from their secondary `gitflic.ru` mirror (Models, SelfImprove, SkillRegistry, ToolSchema, HelixCode, DebateOrchestrator-secondary) — primary GitHub remote healthy | LOW | P0 |
| G19 | Monitoring: Phase-3 Grafana dashboard `docker/monitoring/grafana/dashboards/phase3-memory-safety.json` referenced in CLAUDE.md — need SLI live-test `HELIX_MONITOR_URL` CI wiring (but no CI allowed → needs local runnable challenge) | MEDIUM | P4 |
| G20 | No comprehensive *coverage floor* gate — `make test-coverage-100` exists but is not wired into a local pre-commit Makefile target per module | MEDIUM | P2 |

### 0.3 The Principle

> **Nothing ships broken, disabled, undocumented, or without 100 % multi-type test coverage and a corresponding challenge script.** This plan enforces that principle phase by phase. No "TODO later" is allowed: either an item has a scheduled phase and owner, or it is explicitly removed from the product.

---

## 1. Methodology & Constitutional Anchors

Each workstream below ties back to at least one Constitution rule:

- **CONST-001 Comprehensive Decoupling** — every module self-contained
- **CONST-002 100 % Test Coverage (all types)** — drives P2 & P3
- **CONST-003 Comprehensive Challenges** — drives P3
- **CONST-004 Complete Documentation** — drives P6
- **CONST-005 No Broken Components** — drives P0, P5
- **CONST-006 No Dead Code** — drives P5
- **CONST-007 Memory Safety** — drives P4
- **CONST-008 Security Scanning** — drives P1
- **CONST-009 Monitoring and Metrics** — drives P4
- **CONST-010 Lazy Loading / Non-Blocking** — drives P4
- **CONST-011 Software Principles** / **CONST-012 Design Patterns** — code-review gates in every phase
- **CONST-013 Rock-Solid Changes** — drives mandatory regression-test harness every phase
- **CONST-014 Full Containerization** / **CONST-017 Container-Based Builds** / **CONST-026 Mandatory Container Orchestration Flow** — drives P1 compose infra + every phase's deployment step
- **CONST-018 Unified Configuration** — CLI-agent configs remain generator-only
- **CONST-019 Non-Interactive Execution** — every script in every phase must be fully automatable (no sudo/password prompts)
- **CONST-021 HTTP/3 + Brotli** — validated in P2/P3
- **CONST-022 Test/Challenge Resource Limits** — every test runner must respect GOMAXPROCS=2, nice -n 19, ionice -c 3, -p 1
- **CONST-023 Manual CI/CD only** — **no pipelines added**; everything runs from Makefile
- **CONST-024 GitSpec Compliance** / **CONST-025 SSH Only** — every Git operation in every phase uses SSH
- **CONST-028 Bugfix Documentation** — every fix in every phase appends to `docs/issues/fixed/BUGFIXES.md`

Any phase output that violates any of the above is rejected at review.

---

## 2. Per-Module Current Coverage Matrix (as of 2026-04-18)

Legend: ● present / ○ missing. Counts in () are unit-test-file counts.

| Module | Unit | Int | E2E | Sec | Stress | Chaos | Bench | Fuzz | Race | Load |
|---|---|---|---|---|---|---|---|---|---|---|
| Agentic (6) | ● | ● | ● | ○ | ● | ○ | ○ | ○ | ○ | ○ |
| Auth (15) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| BackgroundTasks (4) | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Benchmark (7) | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| BuildCheck (1) | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Cache (13) | ● | ● | ○ | ○ | ● | ○ | ○ | ○ | ○ | ○ |
| Challenges (283) | ● | ● | ● | ● | ● | ● | ○ | ○ | ○ | ○ |
| Concurrency (19) | ● | ● | ○ | ○ | ● | ○ | ○ | ○ | ● | ○ |
| ConversationContext (3) | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Database (23) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| DebateOrchestrator (42) | ● | ● | ● | ● | ● | ○ | ○ | ● | ○ | ○ |
| DocProcessor (16) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Embeddings (12) | ● | ● | ● | ○ | ○ | ○ | ○ | ● | ○ | ○ |
| EventBus (11) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Formatters (12) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| HelixLLM (952) | ● | ● | ● | ● | ● | ○ | ● | ● | ○ | ○ |
| HelixMemory (32) | ● | ● | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ |
| HelixQA (377) | ● | ● | ● | ● | ● | ○ | ● | ○ | ○ | ○ |
| HelixSpecifier (27) | ● | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| LLMOps (9) | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| LLMOrchestrator (15) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| LLMProvider (55) | ● | ● | ● | ● | ● | ○ | ● | ○ | ○ | ○ |
| LLMsVerifier (223) | ● | ● | ● | ● | ● | ○ | ○ | ○ | ○ | ○ |
| Memory (12) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| MCP_Module (11) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Models (2) | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Observability (16) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Optimization (11) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Planning (8) | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Plugins (10) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| RAG (10) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Security (15) | ● | ● | ○ | ● | ○ | ○ | ○ | ○ | ○ | ○ |
| SelfImprove (7) | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| SkillRegistry (15) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Storage (16) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| Streaming (18) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| ToolSchema (3) | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| VectorDB (10) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |
| VisionEngine (12) | ● | ● | ○ | ○ | ○ | ○ | ○ | ○ | ○ | ○ |

**Interpretation:** Only 3 modules (HelixLLM, HelixQA, LLMProvider) come close to the Constitution-mandated test-type breadth. Every other module has at least one missing type.

---

## 3. Phased Plan

Each phase lists: **Goal**, **Scope**, **Workstreams**, **Test types to add**, **Challenges to add**, **Doc deliverables**, **Exit criteria**, **Rollback strategy**. Phases are sequenced by risk and dependency; within a phase, workstreams may run in parallel across modules with resource limits honoured.

### Phase P0 — Foundation & Repo Hygiene (1 – 2 days)

**Goal:** Remove friction that blocks every subsequent phase; create the measurement substrate.

**Scope:**
- G17 Fix `cli_agents/bridle` orphaned sub-submodule path in its `.gitmodules` (or register missing URL; third-party — may need to raise issue upstream or pin the parent to an earlier SHA that lacked the orphan).
- G18 Drop failing `gitflic.ru` mirror remotes from affected submodules (Models, SelfImprove, SkillRegistry, ToolSchema, HelixCode) — keep GitHub/GitLab only, matching the pattern already applied in Cache/Concurrency/Database.
- Add `scripts/repo-health.sh` — runs all read-only sanity checks: submodule status, git-remote fetch dry-run, required-secret presence in `.env.example`, `go mod verify`, `go mod vendor` no-diff check.
- Add `Makefile` targets: `make repo-health`, `make coverage-floor`, `make security-gates-all`, `make metrics-snapshot`.

**Workstreams:**
1. Scripts/infra clean-up (solo).
2. Baseline metrics capture: coverage.out per module, line counts, cyclomatic complexity via `gocyclo`, test-file inventories. Store under `reports/baseline-2026-04-18/`.

**Test types added:** none (this phase is infrastructure).

**Challenges added:** `challenges/scripts/repo_hygiene_challenge.sh` (validates no orphan submodules, no dead remotes, all Makefile targets compile).

**Docs:** Update CLAUDE.md § "Operational Notes" with new Makefile targets.

**Exit criteria:**
- `git submodule foreach --recursive git fetch --all` exits 0 with no errors.
- `make repo-health` green.
- Baseline metrics committed.

**Rollback:** Each change is a separate commit; `git revert` of the commit restores state.

---

### Phase P1 — Security (2 – 4 weeks)

**Goal:** Close the 149 Dependabot CVEs, ship container-accessible Snyk & SonarQube, audit all command-exec paths, add `govulncheck`.

**Scope (ordered by severity):**
1. **G1 Dependabot triage** — pull the full Dependabot alert list from GitHub (via `gh api`), categorise by severity × module, triage each: upgrade / patch / accept-with-policy / replace dep. Transitive vulns inherited from vendored opensource tools under `HelixQA/tools/opensource/**` are tracked but not fixed here (those are vendor-owned; we pin to safe versions).
2. **G9 Command-exec hardening**:
   - `internal/clis/agents/claude_code/tool_executor.go` — pin allowed binaries, sanitize args, disallow shell metachars, enforce working-dir whitelist.
   - `internal/tools/sandbox/sandbox.go` — audit escape vectors (namespace, seccomp, rlimits); write threat model.
   - `internal/tools/gittools/autocommit.go` — assert no user-supplied args reach `exec.Command`.
3. **G10 TLS posture audit** — every `tls.Config` site: enforce MinVersion 1.3, default `InsecureSkipVerify=false`, opt-in via explicit env flag with warning log.
4. **G11 `govulncheck` integration** — Makefile target `security-scan-vuln`, recorded in `ci-validate-all`.
5. **G12 Container-accessible Snyk + SonarQube**:
   - `docker-compose.security.yml` (profiles: `snyk`, `sonarqube`) running SonarQube Community + scanner CLI and a Snyk CLI image.
   - Wire to `Containers` module so `./bin/helixagent` boots them when `SECURITY_SCAN_ENABLED=true`.
   - Add `make security-scan-snyk-docker`, `make security-scan-sonarqube-docker`.
6. **Secret hygiene sweep** — scan every `cli_agents/*`, `configs/*`, `.env.example`, examples for leaked keys (gitleaks via compose).

**Test types added:** security (per module), fuzz (per input parsing surface), unit (new sanitizers).

**Challenges added:**
- `challenges/scripts/security_cve_challenge.sh` — asserts critical+high CVE count ≤ pinned threshold.
- `challenges/scripts/tool_executor_sandbox_challenge.sh` — fuzzes argv for command injection.
- `challenges/scripts/tls_posture_challenge.sh` — verifies no `InsecureSkipVerify=true` without opt-in env.
- `challenges/scripts/secrets_leak_challenge.sh` — gitleaks across working tree (no false positives on `.env.example`).

**Docs:**
- `docs/security/threat-model.md` — comprehensive threat model (STRIDE-per-surface).
- `docs/security/dependabot-triage-2026-Q2.md` — CVE disposition register.
- Update AGENTS.md, CLAUDE.md § "Security" with new procedures.
- Add User-Manual 48 `48-security-scanning.md`.
- Add Video-Course scripts 85–88 (scan setup, CVE triage, sandbox hardening, TLS posture).

**Exit criteria:**
- GitHub shows **0 critical, 0 high** CVEs on `vasic-digital/HelixAgent`.
- `gosec`, `govulncheck`, Snyk, SonarQube all green with zero new blockers.
- All security challenges pass non-interactively under resource limits.
- Threat model peer-reviewed.

**Rollback:** Dep upgrades done as individual PR-style commits; if a breaking change emerges, revert specific commit. Sandbox changes are additive (new guards, not removed code).

---

### Phase P2 — Structural Test-Coverage Floor (3 – 5 weeks)

**Goal:** Raise every module to coverage floor (90 % lines, 85 % branches) with `make test-coverage-100` driven per-module gates; eliminate skipped/flaky tests.

**Scope:**
1. **G2 Fix flaky concurrent registry test** — root-cause `TestServicesIntegration_ProviderRegistry_ConcurrentAccess`: most-likely candidate is a shared registry singleton between parallel tests. Fix: inject fresh registry per t-group via `t.Setenv` or explicit constructor. Un-skip, run 1000× under `-race` to confirm stability.
2. **G6 Raise unit-test density** in the 8 thin modules:
   - BuildCheck (1 → ≥ 15), Models (2 → ≥ 20), ToolSchema (3 → ≥ 20), ConversationContext (3 → ≥ 25), BackgroundTasks (4 → ≥ 30), SelfImprove (7 → ≥ 25), Planning (8 → ≥ 30), LLMOps (9 → ≥ 30).
   Tests must be TDD-written: spec → failing test → impl → passing test.
3. **G20 Per-module coverage gate** — each module's Makefile gets `make coverage-local` that fails CI-style if under floor. Aggregated by root `make coverage-floor` which iterates modules.
4. Eliminate redundant skips — review each of the 1,008 `t.Skip` statements. Skips gated on `testing.Short()` OK; skips gated on "feature not ready" → fix the feature.

**Test types added:** unit (thousands of new tests across 8 modules), race (for any module with concurrency primitives), benchmark (per exported hot-path).

**Challenges added:**
- `challenges/scripts/coverage_floor_challenge.sh` — walks every module, asserts floor.
- `challenges/scripts/no_skipped_non_short_tests_challenge.sh` — forbids skips outside `testing.Short()`.

**Docs:**
- Update each thin module's CLAUDE.md/AGENTS.md with new test policy.
- User-Manuals 49–51 (coverage gates, flake remediation, TDD in HelixAgent).
- Video-Course scripts 89–92.
- Regenerate per-module coverage HTML → `docs/reports/coverage-2026-Q2/<module>/coverage.html`.

**Exit criteria:**
- Every module ≥ 90 % line coverage.
- `TestServicesIntegration_ProviderRegistry_ConcurrentAccess` passes 1 000× under `-race`.
- Zero non-`testing.Short` skips remain.

**Rollback:** New tests are additive; failing tests revealing real bugs are fixed or reverted individually. No production code change without a paired regression test.

---

### Phase P3 — Test-Type Breadth (4 – 8 weeks)

**Goal:** Every module has ALL ten test types present, each with real data, real services (mocks only in unit tests per CONST-016).

**Scope per module (26 missing stress, 41 missing load, 38 missing race, 35 missing fuzz):**

Per module, add:
- `tests/integration/` if missing → talks to real HelixAgent on port 7061.
- `tests/e2e/` → drives real API flows.
- `tests/security/` → input-abuse scenarios.
- `tests/stress/` → sustained + spike + soak (goroutine-leak detector, memory-ceiling, CPU ceiling).
- `tests/chaos/` → network partition, slow-disk, kill-container mid-flight (via `Containers` chaos-inject extension — may require new feature in Containers module).
- `tests/bench/` → baseline + 25 % regression gate.
- `tests/fuzz/` → per untrusted-input surface.
- `tests/race/` → explicit race scenarios (even for modules that pass `-race`).
- `tests/load/` → sustained RPS curves for HTTP/gRPC surfaces; store baseline in `reports/load-baseline/<module>/`.

**Test types added:** all missing types per module (as tabulated in §2).

**Challenges added (one per module per missing type):** ~250 new challenge scripts following the naming `challenges/scripts/<module>_<type>_challenge.sh`.

Shared infrastructure:
- `tests/helpers/loadgen/` — shared load generator (k6-in-container or native Go) reusable by every module.
- `tests/helpers/chaos/` — shared chaos primitives (network-partition, disk-stall, container-kill).
- `tests/helpers/metrics/` — emits Prometheus-exposition snapshots for post-run regression analysis.

**Docs:**
- `docs/development/test-types-guide.md` — authoritative reference for each type.
- `docs/development/stress-testing-playbook.md`.
- `docs/development/load-testing-playbook.md`.
- `docs/development/chaos-engineering-playbook.md`.
- User-Manuals 52–60 (one per test type).
- Video-Course scripts 93–108.

**Exit criteria:**
- 100 % of modules × 10 types = 410 cells; all present with at least one non-trivial test each.
- `make test-with-infra` exercises every type; wall-time ≤ 90 min under resource limits.
- `./challenges/scripts/run_all_challenges.sh` passes with zero failures.

**Rollback:** Each module's phase-3 work lands on a `test/<module>-typebreadth` branch merged individually. Any regression in existing behaviour is immediate revert + root-cause.

---

### Phase P4 — Memory Safety, Performance, Observability (2 – 4 weeks)

**Goal:** Close goroutine/lock/unbounded-map issues; enforce lazy initialisation + non-blocking everywhere; wire metrics for every new Phase-3 invariant.

**Scope:**
1. **G7 Goroutine lifecycle fixes**:
   - `internal/llm/circuit_breaker.go:167` — wire context cancel; close wrappedCh when ctx.Done().
   - `internal/services/debate_integration/provider_bridge.go:84` — add select on ctx.Done in range loop.
   - `internal/llm/lazy_provider.go:172` — cancel pending factory() if init-timeout elapses; or use sync.Once with context.
   - `internal/services/debate_performance_optimizer.go:169` — context cancellation + semaphore on concurrent member execution.
   - `internal/llm/ensemble.go:73,89` — WaitGroup track provider-init goroutines.
   - `internal/discovery/discoverer.go:379` — shutdown channel + context.
2. **G8 Bounded cache in `debate_performance_optimizer.go`** — LRU with max-entries + TTL; admission-counter metric.
3. **Lock hygiene** (`internal/cache/tiered_cache.go`) — document lock ordering; add deadlock-detector build-tag for test.
4. **CONST-010 Lazy-init audit** — enumerate every package-level `init()` → convert to lazy sync.Once construction.
5. **CONST-009 Metrics** — every new invariant gets a Prometheus gauge + counter following the Phase-3 pattern:
   - `helixagent_goroutines_tracked_total{handler}`
   - `helixagent_lazy_init_durations_seconds{component}`
   - `helixagent_cache_entries{cache,state}` / `helixagent_cache_dropped_total{cache}`
   - `helixagent_semaphore_waiters{name}` / `helixagent_semaphore_holders{name}`
6. **Grafana dashboard** — extend `docker/monitoring/grafana/dashboards/phase3-memory-safety.json` → `phase4-runtime-health.json`.

**Test types added:** race (every fix site), stress (concurrent goroutine counts), leak (via `go.uber.org/goleak` in `tests/helpers/leak/`).

**Challenges added:**
- `challenges/scripts/goroutine_leak_challenge.sh` — spawns load, diffs goroutine counts.
- `challenges/scripts/lazy_init_challenge.sh` — asserts no heavy init() at startup.
- `challenges/scripts/deadlock_detection_challenge.sh` — build with deadlock detector; run integration suite.
- `challenges/scripts/phase4_runtime_health_sli_challenge.sh` — scrapes all new gauges.

**Docs:**
- `docs/development/concurrency-playbook.md` — the canonical reference for goroutine hygiene, lock ordering, bounded channels.
- `docs/diagrams/src/goroutine-lifecycle.puml` — regenerate with new invariants.
- `docs/diagrams/src/lazy-init-sequence.mmd` — new.
- Update `CLAUDE.md` Goroutine Lifecycle Safety section.
- User-Manual 61-62 (concurrency, observability).
- Video-Course scripts 109–112.

**Exit criteria:**
- `go test -race ./... -count=20` (under resource limits) stays green.
- `goleak` integration tests report 0 leaks.
- All new Prometheus gauges visible in `/v1/monitoring/status`.
- Dashboard renders in Grafana with non-empty panels.

**Rollback:** Each concurrency fix sits behind a small commit with paired regression tests. If a production-path regression emerges, revert the specific fix.

---

### Phase P5 — Dead-Code Purge & Refactor (2 – 3 weeks)

**Goal:** Identify and remove dead code (functions/types/features unwired). Evaluate vendor stubs (like the Chroma S3 metastore G16) for removal vs. completion.

**Scope:**
1. Run `deadcode` + `staticcheck -unused` across every module.
2. Dependency-graph analysis of handlers → services → providers → modules. Anything not on a live call-path from a registered HTTP route, a background worker, or a CLI entry point is flagged.
3. Audit build-tag-gated features (`nohelixmemory`, `nohelixspecifier`) — assert both build paths compile and pass core test suites (`go build -tags nohelixmemory ./...`, `go test -tags nohelixspecifier -short ./...`).
4. **G16** Chroma `DeleteOldVersionFiles` — either implement in upstream Chroma (PR) and bump vendor pin, or document as "cleanup handled externally" with a challenge that purges S3 via `aws s3 rm --recursive` for our deployments.
5. Ensure nothing referenced in CLAUDE.md/AGENTS.md is actually dead.

**Test types added:** unit (for any newly-connected feature), integration (regression for removed code — must still be 0 call-sites after removal).

**Challenges added:**
- `challenges/scripts/dead_code_audit_challenge.sh` — fails if `deadcode` finds anything unexpected.
- `challenges/scripts/build_tag_matrix_challenge.sh` — iterates every build-tag combination, asserts compile + `-short` tests pass.

**Docs:**
- `docs/reports/dead-code-purge-2026-Q2.md` — the full register of items removed with rationale.
- Update all affected module docs.

**Exit criteria:**
- `deadcode ./...` returns no unexpected items.
- Both `-tags nohelixmemory` and `-tags nohelixspecifier` builds + short tests pass.

**Rollback:** Removal commits are small. If later work needs something removed, it's restored via `git revert` — but only with a clear connection to a live call-path.

---

### Phase P6 — Documentation, Manuals, Courses, Website (3 – 6 weeks)

**Goal:** Bring the user-facing estate to 100 %. Every feature that exists in code has a user-guide entry, a manual step, a course script, AND (new) an actual recorded video.

**Scope:**
1. **User manuals** — extend from 47 → ~80 guides to cover every module, every protocol endpoint, every challenge, every troubleshooting path. Sequential numbering maintained (`48-security-scanning.md` … `80-observability-dashboards.md`).
2. **Video-course scripts** — extend 84 → ~130 covering all new phases (security, stress testing, chaos, phase-4 runtime health, deprecations/migrations). Every course follows the existing template (intro → prerequisites → demo → exercise → summary).
3. **G13 Actual recorded videos** — record every course using OBS + the HelixAgent live demo stack. Store `.mp4` (H.264 + Opus) and `.webm` (VP9) under `Website/public/videos/`. Generate captions via `whisper.cpp` run locally. Each video ≤ 20 min.
4. **G14 Website modernisation** — migrate `Website/` from vanilla static to a static-site generator choice that still builds without Node at runtime:
   - **Recommended:** Hugo (Go-native, matches repo's Go-first posture, zero JS runtime dependency).
   - Preserve all existing content (manuals, courses). Add search (Pagefind, Hugo-native).
   - Publish to `Website/public/` via existing `build.sh` (updated).
5. **Diagrams** — regenerate every outdated PlantUML/Mermaid diagram; export to SVG + PNG + PDF. Add new diagrams for every Phase-4 invariant.
6. **SQL** — document every schema file with inline comments; generate ER diagrams via `schemaspy` container.
7. **Constitution / CLAUDE.md / AGENTS.md** — synchronise with every change landed in P0–P5; add new constitutional rules if new policies emerged (e.g., "CONST-029 Goroutine Leak-Detector Gate").
8. **API reference** — regenerate OpenAPI/Swagger spec for every REST/gRPC/GraphQL endpoint; publish to `docs/api/openapi-v1.yaml` and render in the website.

**Test types added:** `tests/docs/` — doc-validation tests (dead links, stale references to removed code, code-block compilation checks).

**Challenges added:**
- `challenges/scripts/docs_completeness_challenge.sh` — every module must have README, CLAUDE, AGENTS, docs/, tests/, 1+ user-manual chapter, 1+ course script, 1+ recorded video reference.
- `challenges/scripts/dead_links_challenge.sh` — markdown-link-check on all docs.
- `challenges/scripts/openapi_spec_challenge.sh` — asserts every live route is specified.
- `challenges/scripts/website_build_challenge.sh` — full Hugo build + lighthouse score ≥ 90.

**Docs (meta):**
- `docs/guidelines/documentation-standards.md` — the formal style guide.
- `docs/guidelines/video-recording-playbook.md`.
- User-Manuals 48–80.
- Video-Course scripts 85–130 + recorded videos.

**Exit criteria:**
- Every feature in code has ≥ 1 user-manual section, 1 course script, 1 recorded video.
- Website renders via `make website-build` with zero warnings; Lighthouse ≥ 90 on accessibility, performance, best-practices.
- Dead-link checker clean.
- `docs/diagrams/MISSING_RENDERS.md` empty.
- Constitution/CLAUDE/AGENTS stamped with `Updated: 2026-QX-XX` reflecting final state.

**Rollback:** Doc commits are safe; only the website-generator migration carries risk — keep the current vanilla build under `Website/legacy/` during transition.

---

### Phase P7 — Continuous Monitoring, Stabilisation, Metrics-Driven Optimisation (2 – 4 weeks)

**Goal:** Run the full test + challenge + scan suite continuously (manual trigger per Constitution, no automated pipelines). Collect metrics, analyse, optimise until steady-state is reached.

**Scope:**
1. `make ci-validate-all` iterated weekly by the maintainer (manual per CONST-023).
2. `make test-with-infra` + full challenges: 3 consecutive green runs required to declare stability.
3. `make security-gates-all`: 3 consecutive green + zero new CVEs.
4. Metrics analysis from `tests/monitoring/phase3_sli_live_test.go` + new `phase4_*` tests → drive optimisation backlog.
5. Load-baseline regression gates: any module with > 25 % bench regression or > 10 % load-test regression triggers investigation.
6. Final `docs/reports/zero-unfinished-work-attestation-2026-QX.md` — signed attestation by maintainer that every item in this plan is satisfied, with evidence links.

**Test types added:** none new; harden existing with longer soak windows.

**Challenges added:**
- `challenges/scripts/stability_three_in_a_row_challenge.sh`.
- `challenges/scripts/regression_gate_challenge.sh`.

**Docs:** final attestation report + archive of all phase outputs.

**Exit criteria:**
- 3 consecutive weekly green runs of the full matrix.
- Zero critical + high CVEs.
- All modules at / above coverage floor.
- Attestation report committed.

---

## 4. Cross-Cutting Concerns (Every Phase)

1. **Resource limits (CONST-022)** — every script in every phase uses `GOMAXPROCS=2 nice -n 19 ionice -c 3`; every go-test invocation uses `-p 1`; every container has CPU/RAM caps. Script template: `scripts/runners/limited-run.sh`.
2. **Non-interactive (CONST-019)** — nothing prompts. SSH via agent (`ssh-add` pre-loaded), secrets via env, no `sudo` without pre-authorised `NOPASSWD` sudoers entries.
3. **Container orchestration (CONST-026)** — no phase introduces manual `docker/podman` / `docker-compose up`. Everything runs through `./bin/helixagent` after `make build`, reading `Containers/.env`.
4. **SSH-only git (CONST-025)** — every new submodule, every fork, every PR URL uses SSH.
5. **Conventional Commits** — `<type>(<scope>): <description>`. Scopes aligned with module names.
6. **BUGFIXES.md (CONST-028)** — every fix in every phase appends an entry with root-cause, affected files, fix description, verification-test reference.
7. **Rock-solid changes (CONST-013)** — no phase ships breaking changes without a migration guide AND a challenge that exercises the old → new path.
8. **Vendoring** — every submodule change followed by `go mod vendor` for the consumer; challenge `vendor_consistency_challenge.sh` asserts no drift.
9. **Third-party submodules** — never modified / committed / pushed. Their pins are updated by bumping to upstream tags, not by direct edits.

---

## 5. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| CVE backlog has breaking upgrades | High | High | Triage each; gate risky upgrades behind feature flags + full test suite. |
| New stress/load tests crash host per CONST-022 | Med | High | Always run under `GOMAXPROCS=2 nice -n 19 ionice -c 3`; container CPU/RAM caps. |
| Test-type expansion reveals latent bugs | Near-certain | Med | Welcome — each bug gets a BUGFIXES entry + fix + regression test. |
| Website migration breaks link structure | Med | Med | Keep URL map; run `dead_links_challenge.sh` before publishing. |
| Vendor submodule stale pins cause fetch errors | Med | Low | Drop dead mirror remotes in P0; document supported mirror set. |
| Recording 50+ videos is time-expensive | High | Med | Script-first, batch-record sessions; automate slide → overlay via `slidev` + OBS. |
| Dead-code removal breaks unknown consumer | Med | High | Require dependency-graph proof of zero call-sites + 1 release of deprecation log before removal. |

---

## 6. Timeline & Effort Estimate

Single-developer, full-time-equivalent pace, respecting resource limits and the "no CI automation" Constitution:

| Phase | Duration (estimate) | Dependencies |
|---|---|---|
| P0 Foundation | 1-2 days | — |
| P1 Security | 2-4 weeks | P0 |
| P2 Coverage floor | 3-5 weeks | P0; overlaps with P1 after week 1 |
| P3 Test-type breadth | 4-8 weeks | P2 complete; overlaps with P1 last week |
| P4 Memory safety / perf / obs | 2-4 weeks | P3 ≥ 50 % |
| P5 Dead-code purge | 2-3 weeks | P2 complete |
| P6 Docs + website + videos | 3-6 weeks | P1–P5 ≥ 50 %; last 2 weeks after all code phases |
| P7 Stabilisation | 2-4 weeks | All prior phases complete |
| **Total** | **~4-6 months** | |

These are best-case estimates assuming current scope holds; expect ± 30 % based on CVE depth.

---

## 7. Governance & Attestation

Every phase closes with:
1. Signed-off exit-criteria checklist (maintainer + code-review agent).
2. Updated `CONSTITUTION.json`, `CLAUDE.md`, `AGENTS.md`.
3. Updated `docs/issues/fixed/BUGFIXES.md`.
4. Updated `docs/MODULES.md`.
5. Per-phase report under `docs/reports/phase-PX-closure-<YYYY-MM-DD>.md`.

The final deliverable is `docs/reports/zero-unfinished-work-attestation-2026-QX.md` — a signed statement that every Constitution rule is honoured repo-wide, with evidence links.

---

## Appendix A — Authoritative Source List

- Constitution v1.2.0 (`CONSTITUTION.md`, `CONSTITUTION.json`)
- `CLAUDE.md` (project instructions, 2026-04-16)
- `AGENTS.md` (2026-04-16)
- `docs/MODULES.md`
- `docs/issues/fixed/BUGFIXES.md`
- `Website/user-manuals/**` (current 01–47)
- `Website/video-courses/**` (current 84+ scripts)
- `docs/diagrams/**` (40+ diagrams)
- GitHub Dependabot alert page: <https://github.com/vasic-digital/HelixAgent/security/dependabot>

## Appendix B — Glossary (for non-code readers)

- **Challenge** — a shell-script-based integration test validating real end-to-end behaviour, not return codes.
- **Coverage floor** — the minimum line/branch percentage below which a module cannot merge.
- **Flaky test** — a test that sometimes passes, sometimes fails with no code change; must be fixed or removed.
- **Stress test** — sustained load at high rate for extended time.
- **Load test** — stepped load curve (ramp-up, sustain, ramp-down) measuring SLOs.
- **Chaos test** — deliberately induces faults (network partition, kill, slow disk).
- **Fuzz test** — random/structured input generation to find crashes.
- **SLI / SLO** — service-level indicator (metric) / objective (threshold).
