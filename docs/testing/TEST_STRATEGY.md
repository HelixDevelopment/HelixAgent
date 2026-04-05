# Test Strategy

**Date:** 2026-03-30
**Status:** Active

## Overview

HelixAgent maintains 14 distinct test types. Each type has a dedicated directory, build tag, and Makefile target. This document is the authoritative inventory of what each type covers, how to run it, and what resource constraints apply.

---

## Test Type Inventory

### 1. Unit Tests

**Directory:** `./internal/...` (co-located with production code)
**Build tag:** None (always included)
**Run:** `make test-unit` or `go test -short ./internal/...`
**Purpose:** Test individual functions and types in isolation. Mocks and stubs are permitted only here.
**Resource limits:** `GOMAXPROCS=2 nice -n 19 ionice -c 3` with `-p 1`

### 2. Integration Tests

**Directory:** `./tests/integration/`
**Build tag:** `integration`
**Run:** `make test-integration`
**Purpose:** Test interactions between internal packages and external services (PostgreSQL, Redis, Mock LLM). Requires infrastructure containers.
**Prerequisites:** `make test-infra-start`
**Resource limits:** `GOMAXPROCS=2 nice -n 19 ionice -c 3` with `-p 1`

### 3. End-to-End (E2E) Tests

**Directory:** `./tests/e2e/`
**Build tag:** `e2e`
**Run:** `make test-e2e`
**Purpose:** Test complete request flows through the running HelixAgent server. Validates HTTP endpoints, streaming responses, startup verification, and CLI agent behavior.
**Prerequisites:** Running HelixAgent server or `make test-infra-start`
**Resource limits:** `GOMAXPROCS=2 nice -n 19 ionice -c 3` with `-p 1`

### 4. Security Tests

**Directory:** `./tests/security/`
**Build tag:** `security`
**Run:** `make test-security`
**Purpose:** Test authentication, authorization, input validation, prompt injection resistance, and API security. Uses real HTTP requests against test server.
**Resource limits:** `GOMAXPROCS=2 nice -n 19 ionice -c 3` with `-p 1`

### 5. Stress Tests

**Directory:** `./tests/stress/`
**Build tag:** `stress`
**Run:** `make test-stress`
**Purpose:** Validate system behavior under sustained load. Tests goroutine leaks, memory growth, pool exhaustion, cache stampedes, circuit breaker cascades, and concurrent debate execution. Phase 2 (23 correctness tests) and Phase 3 (10 quantitative tests).
**Resource limits:** Strictly enforced: `GOMAXPROCS=2 nice -n 19 ionice -c 3` with `-p 1`. Exceeding limits has caused host system crashes.

### 6. Fuzz Tests

**Directory:** `./tests/fuzz/`
**Build tag:** `fuzz`
**Run:** `make test-fuzz` (corpus replay) or `go test -fuzz=FuzzName -fuzztime=30s ./tests/fuzz/`
**Purpose:** Discover panics and unexpected behavior by feeding random inputs to parsers, validators, and protocol handlers. 52 fuzz functions across 17 test files covering 10 target areas.
**Resource limits:** Fuzz tests are CPU-intensive; use `-fuzztime` to bound duration.

### 7. Penetration Tests

**Directory:** `./tests/pentest/`
**Build tag:** `pentest`
**Run:** `go test -tags=pentest -v ./tests/pentest/`
**Purpose:** Simulate attack scenarios: rate limit bypass, auth bypass, API key leakage, injection attacks, SSRF prevention, DDoS resistance. Uses real HTTP requests.
**Resource limits:** Standard limits apply.

### 8. Performance / Benchmark Tests

**Directory:** `./tests/performance/`
**Build tag:** None (uses `-bench` flag)
**Run:** `make test-bench` or `go test -bench=. -benchmem ./tests/performance/`
**Purpose:** Measure throughput, latency, and memory allocation for critical paths. Includes baseline regression detection (see `docs/performance/BASELINE_GUIDE.md`).
**Resource limits:** `GOMAXPROCS=2 nice -n 19` with `-p 1`

### 9. Chaos Tests

**Directory:** `./tests/chaos/`
**Build tag:** `chaos`
**Run:** `make test-chaos`
**Purpose:** Inject failures (network partitions, process kills, resource exhaustion) and verify graceful degradation. Subdirectories cover: agentic, API, auth, circuit breaker, core, ensemble, fault injection, MCP, memory, provider, rate limiter, resilience, streaming, verifier.
**Resource limits:** Standard limits apply.

### 10. Compliance Tests

**Directory:** `./tests/compliance/`
**Build tag:** `compliance`
**Run:** `go test -tags=compliance -v ./tests/compliance/`
**Purpose:** Verify adherence to project standards: API response formats, authentication contracts, constitution rules, error formats, HTTP compliance, logging formats, module structure, provider configuration, rate limit behavior, system-level invariants.
**Resource limits:** Lightweight; standard limits apply.

### 11. Monitoring Tests

**Directory:** `./tests/monitoring/`
**Build tag:** `monitoring`
**Run:** `go test -tags=monitoring -v ./tests/monitoring/`
**Purpose:** Validate Prometheus metrics exposure, pprof endpoint gating, and dashboard query correctness.
**Resource limits:** Standard limits apply.

### 12. Automation Tests

**Directory:** `./tests/automation/`
**Build tag:** `automation`
**Run:** `go test -tags=automation -v ./tests/automation/`
**Purpose:** End-to-end automation workflows: build pipelines, full-system automation, agentic ensemble automation.
**Resource limits:** Standard limits apply.

### 13. Race Detection

**Build tag:** None (uses `-race` flag)
**Run:** `make test-race` or `go test -race ./internal/...`
**Purpose:** Run the Go race detector across all internal packages. Catches data races missed by regular tests.
**Resource limits:** Race detection has 5-10x overhead; use `GOMAXPROCS=2`.

### 14. Challenge Tests

**Directory:** `./tests/challenge/`
**Build tag:** `challenge`
**Run:** `make test-chaos` or `go test -tags=challenge -v ./tests/challenge/`
**Purpose:** Validate complex multi-component scenarios: debate groups, provider autodiscovery, single-provider debate, AI debate maximal challenge.
**Resource limits:** Standard limits apply.

---

## Build Tags Summary

| Tag | Directory | Makefile Target |
|-----|-----------|----------------|
| (none) | `./internal/...` | `make test-unit` |
| `integration` | `./tests/integration/` | `make test-integration` |
| `e2e` | `./tests/e2e/` | `make test-e2e` |
| `security` | `./tests/security/` | `make test-security` |
| `stress` | `./tests/stress/` | `make test-stress` |
| `fuzz` | `./tests/fuzz/` | `make test-fuzz` |
| `pentest` | `./tests/pentest/` | Manual |
| `chaos` | `./tests/chaos/` | `make test-chaos` |
| `compliance` | `./tests/compliance/` | Manual |
| `monitoring` | `./tests/monitoring/` | Manual |
| `automation` | `./tests/automation/` | Manual |
| `challenge` | `./tests/challenge/` | `make test-chaos` |

---

## Resource Limit Enforcement

All test execution MUST be limited to 30-40% of host resources. This is a Constitution-level requirement (Priority 1). The host runs mission-critical processes; exceeding these limits has caused system crashes and forced resets.

**Mandatory flags:**
```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -p 1 -timeout=300s ...
```

**Container limits (when applicable):**
```bash
--memory=2g --cpus=2
```

**Validation:** `./challenges/scripts/resource_limits_challenge.sh`

---

## Running All Tests

```bash
# Quick: unit tests only
make test-unit

# Standard: all test types with infrastructure
make test-with-infra

# Full validation (all types + challenges + benchmarks)
make ci-validate-all
```

## Cross-References

- Fuzz details: `docs/testing/FUZZ_TESTING_GUIDE.md`
- Stress details: `docs/testing/STRESS_TESTING_GUIDE.md`
- Benchmark baselines: `docs/performance/BASELINE_GUIDE.md`
- Security scanning: `docs/security/SCANNING_GUIDE.md`
