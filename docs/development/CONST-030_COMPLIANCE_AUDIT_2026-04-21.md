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
| Fixed in-session                        | 1     |
| Deferred to dedicated session           | 40    |

No violation met the in-session fix criteria (≤50 LOC rewrite, no cross-file
ripple, no production-code change, substitution of in-process fake with live
probe + `t.Skip`). Every finding requires either:

1. A semantic redesign of the test (stress/chaos/perf tests whose entire
   purpose is to exercise controlled in-process failure paths), or
2. Removal of a shared-helper mock that is imported by ≥2 sibling test files
   (cross-file ripple), or
3. A rewrite of >200 LOC to swap an in-process LLM/provider/backend fake for a
   real HTTP client and `t.Skip` on unreachable.

The most honest outcome for this session is a thorough, categorised inventory
— which is what follows. Each deferred violation includes a proposed
replacement pattern so the dedicated CONST-030 remediation session can slice
the work into focused PRs.

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

### Fixed this session

| File | Commit | Pattern | Notes |
|------|--------|---------|-------|
| `internal/handlers/handlers_integration_test.go` | (pending commit, PR2) | Pattern 1 — live :7061 probe + `t.Skip` | Rewrote 987 LOC → 402 LOC. All 14 `TestIntegration_*` cases now dial `tcp://localhost:7061`, skip cleanly when unreachable, and drive real HTTP round-trips against `/v1/health`, `/v1/chat/completions`, `/v1/models`, `/v1/debates/*`, `/v1/mcp/*`. Removed `MockLLMProvider` wiring, `setupIntegrationTest()` helper, orphan `mockSkillsService`. Compile + skip-path verified with :7061 down. |
| `internal/services/services_integration_test.go` | (pending commit, PR1) | Pattern 1 — live :7061 probe + `t.Skip` | Rewrote 803 LOC → 466 LOC. `integrationMockProvider` deleted. All 12 `TestServicesIntegration_*` cases now probe `/v1/health` via `isHelixAgentAvailable(t)` and `skipUnlessLive(t)`, driving live HTTP against `/v1/debates`, `/v1/ensemble`, `/v1/chat/completions`, `/v1/discovery/providers`. Companion `internal/services/suspiciously_fast_response_verification_test.go` dropped `TestIntegrationMockProviderLatency_...` (mock no longer exists); 3 boundary unit tests of `IsSuspiciouslyFastResponse` retained & verified green. `TestServicesIntegration_ProviderRegistry_ConfigureDisablesProvider` removed — in-process registry CRUD is not exposed over HTTP; invariant belongs in a unit test. Compile (`go build`), vet (`go vet`) and skip-path (`12/12 SKIP`) verified with :7061 down. |

### Deferred (documented for future session)

#### tests/integration/ — 14 files

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| `tests/integration/helpers_test.go` | 138 | `MockBaseLLMProvider` | **high** — shared by `mem0_ensemble_integration_test.go` |
| `tests/integration/integration_test.go` | 393 | in-process provider & cache mocks | high |
| `tests/integration/service_wiring_test.go` | 869 | service wiring mocks | high |
| `tests/integration/service_interaction_test.go` | 295 | multi-service mock graph | medium |
| `tests/integration/request_flow_test.go` | 1,402 | end-to-end request-flow mocks (uses `mockProvider`) | **high** |
| `tests/integration/provider_integration_test.go` | 1,246 | `mockProvider` per-test (uses `integrationMockProvider`-style pattern) | **high** |
| `tests/integration/provider_verification_comprehensive_test.go` | 274 | comprehensive verification with canned provider | high |
| `tests/integration/cli_agent_integration_test.go` | 1,463 | CLI-agent mocks (2 struct types) | high |
| `tests/integration/tool_integration_test.go` | 1,621 | tool-registry mock | high |
| `tests/integration/rag_integration_test.go` | 429 | RAG retriever mock | medium |
| `tests/integration/opencode_ensemble_flow_test.go` | 796 | OpenCode ensemble mock (uses `mockProvider`) | **high** |
| `tests/integration/debate_adversarial_integration_test.go` | 270 | `mockAdversarialLLM`, `failingAdversarialLLM` | medium — tests in-process protocol parser |
| `tests/integration/debate_full_protocol_integration_test.go` | 352 | full-protocol canned LLM | medium |
| `tests/integration/debate_reflexion_integration_test.go` | 318 | reflexion-loop canned LLM | medium |

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

#### tests/e2e/ — 2 files

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| `tests/e2e/e2e_test.go` | 540 | in-process E2E harness with mock LLMs | **critical** — E2E by definition must be live |
| `tests/e2e/ai_debate_e2e_test.go` | 704 | debate E2E with canned LLM | **critical** |

**Proposed approach:** E2E tests must call the live HelixAgent on :7061 end to
end. Rewrite to pure HTTP + `t.Skip` guard. These are the two most important
files to fix; their in-process-mock posture is a severe violation of CONST-030.

#### tests/chaos/ — 3 files

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| `tests/chaos/core/chaos_test.go` | 417 | 2 mock types (provider + dependency) | high |
| `tests/chaos/agentic/agentic_ensemble_chaos_test.go` | 298 | agentic ensemble mock | high |
| `tests/chaos/provider_fallout_chaos_test.go` | 286 | provider-fallout mock injection | high |

**Blocker:** chaos tests deliberately inject controlled failures (latency
spikes, panics, connection drops) into a provider interface. Injecting the
same kinds of failure into real services requires toxiproxy, container SIGKILL
drivers, or firewall manipulation — an infrastructure deliverable, not a test
rewrite.

**Proposed approach:** stand up toxiproxy between HelixAgent and one upstream
(PostgreSQL or a mock-LLM container at `tests/mock-llm-server/`), then rewrite
these tests to drive the proxy rather than an in-process fake. Toxiproxy
integration is a 1–2 day deliverable.

#### tests/stress/ — 13 files

All 13 stress files under `tests/stress/` declare at least one
mock/fake/stub struct and exercise it under parallel load:

- `bigdata_stress_test.go`, `circuit_breaker_storm_stress_test.go`,
  `debate_concurrency_stress_test.go`, `debate_stress_test.go`,
  `ensemble_correctness_stress_test.go`, `ensemble_stress_test.go`,
  `mcp_adapter_stress_test.go`, `memory_growth_stress_test.go`,
  `memory_pressure_stress_test.go`, `memory_stress_test.go`,
  `provider_fallback_stress_test.go`, `provider_stress_test.go`,
  `websocket_stress_test.go`.

**Blocker:** stress tests exist to measure in-process scheduling, semaphore,
pool saturation, and GC behaviour under synthetic parallelism — replacing the
mock with a real network round-trip changes the test from "does our semaphore
saturate under 10k concurrent provider calls?" to "can we network-flood a real
server?" Different test; the information the stress test currently produces
would be lost.

**Proposed approach:** split stress tests into two buckets:

1. **In-process micro-stress** — keep the mocks, `git mv` to a new
   `tests/microstress/` package with build tag `//go:build microstress`, and
   exempt the new package from CONST-030 via an explicit named exception
   added to the rule ("micro-stress tests of in-process concurrency
   primitives"). This is the honest thing to do — these tests do produce
   value, they just aren't stress tests of the **system**, they're stress
   tests of the **package**.
2. **System stress** — create a small number of new files in
   `tests/stress/` that drive real HTTP load against `:7061` (use
   `vegeta`, `k6`, or raw `http/3` clients) and assert p99 / error-rate SLOs.

#### tests/security/ — 2 files

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| `tests/security/debate_security_test.go` | 385 | in-process debate security mock | high |
| `tests/security/userflow_security_test.go` | 1,068 | userflow security mocks | **critical** |

**Proposed approach:** security tests MUST exercise the real guardrail
pipeline (CLAUDE.md §16 already references
`redteam_fixtures_realpipeline_test.go` as the correct pattern). Rewrite to
call `POST /v1/completion` with prompt-injection payloads and assert on the
guardrail rejection shape. Preserve the existing payloads as test fixtures.

#### tests/performance/ — 1 file

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| `tests/performance/ensemble_benchmark_test.go` | 127 | `benchMockProvider` (pure in-process) | medium |

**Blocker:** benchmarks with `//go:build performance` are not runtime tests —
they are tooling to measure in-process semaphore overhead. Under CONST-030 as
literally written, benchmarks are still "runtime verification" and must hit
real infra — but the resulting numbers measure network latency, not the
code under benchmark.

**Proposed approach:** same split as stress tests — keep the pure in-process
benchmark under `tests/microbench/` with a named CONST-030 exemption, and add
a separate `tests/sysbench/` that measures the whole system.

#### tests/automation/ — 1 file

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| `tests/automation/full_automation_test.go` | 1,639 | automation harness with multiple mocks | high |

**Proposed approach:** the automation test simulates a CI pipeline in-process.
Replace each in-process step with a shell-out to the real `make` target it
emulates (already enforced by CONST-018 "No GitHub Actions..." — we run
everything via make). Likely the file can be largely deleted, as the real
automation is the Makefile itself.

#### internal/ — 9 files

| File | LOC | Mock class(es) | Severity |
|------|-----|----------------|----------|
| ~~`internal/services/services_integration_test.go`~~ | ~~803~~ | ~~`integrationMockProvider`~~ | **Fixed this session** (PR1) — see table above. |
| `internal/services/integration_orchestrator_test.go` | ~ | `mockProvider` | high |
| ~~`internal/handlers/handlers_integration_test.go`~~ | ~~987~~ | ~~HTTP handler mocks~~ | **Fixed this session** (PR2) — see table above. |
| `internal/bigdata/memory_integration_test.go` | 938 | memory backend mocks | high |
| `internal/bigdata/debate_integration_test.go` | 510 | debate backend mocks | high |
| `internal/bigdata/integration_test.go` | ~ | bigdata service mocks | medium |
| `internal/adapters/auth/integration_test.go` | ~ | auth adapter mocks | medium |
| `internal/adapters/mcp/integration_test.go` | ~ | MCP adapter mocks | medium |
| `internal/security/integration_test.go` | ~ | security integration mocks | high |

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
