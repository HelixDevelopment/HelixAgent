# Stress Testing Guide

**Date:** 2026-03-30
**Status:** Active

## Overview

HelixAgent's stress test suite validates system behavior under sustained load, resource exhaustion, and concurrent access. The suite is organized in `tests/stress/` with 63 test files, split across two phases: Phase 2 correctness tests (23 tests proving the system behaves correctly under load) and Phase 3 quantitative tests (10 tests measuring throughput, latency, and capacity limits).

All stress tests use the `//go:build stress` build tag.

---

## Phase 2: Correctness Under Load (23 Tests)

These tests verify that the system produces correct results even when pushed to its limits. They do not measure performance -- they assert functional correctness under concurrent access.

| Test File | What It Validates |
|-----------|-------------------|
| `pool_stress_test.go` | CLI process pool returns correct processes under contention |
| `worker_pool_stress_test.go` | Worker pool task completion and result correctness |
| `worker_pool_overload_stress_test.go` | Worker pool behavior when queue is full |
| `http_client_pool_stress_test.go` | HTTP pool returns valid clients per-host under races |
| `http_pool_exhaustion_stress_test.go` | HTTP pool graceful degradation when exhausted |
| `event_bus_stress_test.go` | Event delivery correctness under subscriber churn |
| `event_bus_flood_stress_test.go` | No lost events under sustained high-rate publishing |
| `ensemble_correctness_stress_test.go` | Ensemble voting produces consistent results |
| `ensemble_failure_stress_test.go` | Ensemble fallback chain correctness |
| `circuit_breaker_stress_test.go` | Circuit breaker state transitions are correct |
| `circuit_breaker_cascade_stress_test.go` | Cascading failures are isolated |
| `circuit_breaker_storm_stress_test.go` | Breaker storm does not corrupt state |
| `cache_stress_test.go` | Cache returns correct values under concurrent reads/writes |
| `cache_concurrent_stress_test.go` | Cache operations are linearizable |
| `cache_eviction_stress_test.go` | Eviction policy correctness under memory pressure |
| `cache_stampede_stress_test.go` | Cache stampede protection works (single-flight) |
| `concurrent_debate_stress_test.go` | Multiple concurrent debates produce valid results |
| `debate_concurrency_stress_test.go` | Debate state machine transitions are safe |
| `goroutine_leak_stress_test.go` | No goroutine growth over sustained operation |
| `memory_growth_stress_test.go` | Memory usage stays bounded |
| `memory_leak_stress_test.go` | No objects retained past their lifecycle |
| `pool_saturation_stress_test.go` | Pool saturation does not cause deadlocks |
| `race_condition_stress_test.go` | Known race-prone code paths are safe |

### Phase 3: Quantitative Stress (10 Tests)

These tests measure system capacity and detect regressions in throughput, latency, and resource consumption.

| Test File | What It Measures |
|-----------|-----------------|
| `streaming_backpressure_quantitative_stress_test.go` | Streaming throughput under backpressure (events/sec) |
| `ensemble_all_timeout_stress_test.go` | Timeout handling when all providers are slow |
| `deadlock_detection_stress_test.go` | Time to detect and recover from deadlock conditions |
| `db_pool_exhaustion_stress_test.go` | Database pool behavior at max connections |
| `provider_concurrent_stress_test.go` | Provider request throughput under concurrency |
| `provider_failover_stress_test.go` | Failover latency and correctness |
| `provider_fallback_stress_test.go` | Fallback chain traversal speed |
| `rate_limiter_stress_test.go` | Rate limiter accuracy under burst traffic |
| `streaming_backpressure_stress_test.go` | Backpressure mechanism activation threshold |
| `streaming_storm_stress_test.go` | Streaming stability under connection storms |

### Additional Stress Tests (30 Files)

The remaining 30 test files cover specific subsystems:

- **API:** `api_stress_test.go`, `handlers_stress_test.go`, `handler_concurrent_stress_test.go`, `new_handlers_stress_test.go`
- **Memory:** `memory_stress_test.go`, `memory_system_stress_test.go`, `memory_pressure_stress_test.go`
- **Debate:** `debate_stress_test.go`, `debate_concurrent_stress_test.go`
- **Infrastructure:** `health_check_stress_test.go`, `boot_manager_concurrent_stress_test.go`
- **Providers:** `provider_stress_test.go`, `provider_registry_stress_test.go`
- **Middleware:** `middleware_stress_test.go`
- **Formatters:** `formatters_stress_test.go`
- **MCP:** `mcp_adapter_stress_test.go`
- **Ensemble:** `ensemble_stress_test.go`, `agentic_ensemble_stress_test.go`
- **Lifecycle:** `goroutine_stability_stress_test.go`, `goroutine_leak_test.go`, `concurrency_stress_test.go`, `concurrency_safety_test.go`
- **Other:** `websocket_stress_test.go`, `extreme_load_test.go`, `chaos_engineering_test.go`, `bigdata_stress_test.go`, `stress_test.go`, `userflow_stress_test.go`, `qa_vision_stress_test.go`

---

## Running Stress Tests

### Full Suite

```bash
make test-stress
```

This runs all 63 stress test files with proper resource limits.

### Single Test

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -tags=stress -v -run TestPoolSaturation \
  ./tests/stress/ -p 1 -timeout=300s
```

### With Race Detector

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -tags=stress -race -v \
  ./tests/stress/ -p 1 -timeout=600s
```

---

## Resource Limit Enforcement

Stress tests are the most resource-intensive test type. The Constitution requires strict limits to protect the host system.

**Mandatory flags:**
```bash
GOMAXPROCS=2        # Limit Go scheduler to 2 OS threads
nice -n 19          # Lowest CPU scheduling priority
ionice -c 3         # Idle I/O scheduling class
-p 1                # One test package at a time
-timeout=300s       # 5-minute hard timeout per package
```

**Why this matters:** The host machine runs mission-critical processes. Stress tests that consumed excessive resources have previously caused system crashes and forced hard resets. These limits are non-negotiable.

**Container limits (when running in containers):**
```bash
--memory=2g --cpus=2
```

---

## Interpreting Results

### Correctness Tests

- **PASS:** The system is functionally correct under the tested load level.
- **FAIL with data race:** A race condition exists. Check the race detector output for the exact goroutine stacks. See `docs/development/SAFETY_FIXES.md` for the fix pattern.
- **FAIL with timeout:** The system is too slow under load or has a deadlock. Check for lock contention with `go tool pprof`.
- **FAIL with goroutine leak:** A goroutine is not being cleaned up. Compare goroutine counts before/after the test.

### Quantitative Tests

- Results are printed as structured output with measurements.
- Compare against previous runs to detect regressions.
- Key metrics: operations/second, p99 latency, memory delta, goroutine delta.

---

## Adding New Stress Tests

1. Create a file in `tests/stress/` with the `//go:build stress` build tag
2. Name it `<subsystem>_stress_test.go`
3. Use `testing.T` (not `testing.B` -- benchmarks go in `tests/performance/`)
4. Record goroutine count at start and end; assert no growth
5. Use `context.WithTimeout` to bound the test duration
6. Document the resource consumption expectation in a comment

---

## Cross-References

- Test strategy: `docs/testing/TEST_STRATEGY.md`
- Fuzz testing: `docs/testing/FUZZ_TESTING_GUIDE.md`
- Safety fixes: `docs/development/SAFETY_FIXES.md`
- Resource limits challenge: `./challenges/scripts/resource_limits_challenge.sh`
