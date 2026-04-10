# User Manual 47: Stress Testing Guide

**Version:** 1.0
**Last Updated:** April 10, 2026
**Audience:** QA Engineers, Performance Engineers, Developers

---

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Running Stress Tests](#running-stress-tests)
4. [Resource Limits](#resource-limits)
5. [Sustained Load Testing](#sustained-load-testing)
6. [Spike Testing](#spike-testing)
7. [Soak Testing](#soak-testing)
8. [Memory Leak Detection](#memory-leak-detection)
9. [Goroutine Leak Detection](#goroutine-leak-detection)
10. [Circuit Breaker Testing](#circuit-breaker-testing)
11. [Database Pool Exhaustion Testing](#database-pool-exhaustion-testing)
12. [Stress Test Suite Reference](#stress-test-suite-reference)
13. [Configuration Reference](#configuration-reference)
14. [Troubleshooting](#troubleshooting)

---

## Overview

HelixAgent mandates comprehensive stress testing to verify that the
system remains responsive and cannot be overloaded or broken under
adverse conditions. The stress test suite contains 60+ test files
covering concurrency safety, memory pressure, goroutine leaks,
circuit breaker cascades, cache stampedes, database pool exhaustion,
streaming backpressure, and extreme load scenarios.

All stress tests are resource-limited to 30-40% of host resources
per the project Constitution. The host machine runs mission-critical
processes; exceeding these limits has historically caused system
crashes and forced resets.

### Stress Test Architecture

```
Stress Test Categories
======================

[Sustained Load]     -- Constant high traffic for extended periods
[Spike Testing]      -- Sudden traffic bursts
[Soak Testing]       -- Low-moderate traffic over hours
[Memory Pressure]    -- Force memory allocation patterns
[Goroutine Stress]   -- Concurrent goroutine creation/destruction
[Circuit Breaker]    -- Cascading provider failures
[Pool Exhaustion]    -- Database/HTTP connection pool limits
[Cache Stampede]     -- Simultaneous cache misses
[Streaming Storm]    -- SSE/WebSocket flood
[Backpressure]       -- Slow consumer scenarios
```

---

## Prerequisites

- HelixAgent built: `make build`
- HelixAgent running: `./bin/helixagent` (auto-boots infrastructure
  containers including PostgreSQL, Redis, Mock LLM)
- Go 1.25+ with `go test` available
- Docker or Podman for infrastructure containers
- `curl` 7.x+ for API-level stress testing
- `pprof` (bundled with Go) for profiling during tests
- At least 4 GB of free memory

Set up environment variables for infrastructure access:

```bash
export DB_HOST=localhost
export DB_PORT=15432
export DB_USER=helixagent
export DB_PASSWORD=helixagent123
export DB_NAME=helixagent_db
export REDIS_HOST=localhost
export REDIS_PORT=16379
export REDIS_PASSWORD=helixagent123
export HELIX_KEY="your-helixagent-api-key"
export HELIX_URL="http://localhost:7061"
```

---

## Running Stress Tests

### Step 1: Run the Full Stress Test Suite

```bash
make test-stress
```

This executes all tests in `tests/stress/` with resource limits
applied automatically (`GOMAXPROCS=2`, `nice -n 19`, `ionice -c 3`,
`-p 1`).

### Step 2: Run a Specific Stress Test

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -run TestConcurrencySafety \
  ./tests/stress/concurrency_safety_test.go
```

### Step 3: Run Stress Tests by Category

```bash
# Memory-related stress tests
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run "TestMemory" ./tests/stress/...

# Circuit breaker stress tests
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run "TestCircuitBreaker" ./tests/stress/...

# Cache stress tests
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run "TestCache" ./tests/stress/...

# Database pool stress tests
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run "TestDBPool" ./tests/stress/...
```

### Step 4: Run Verifier-Specific Stress Tests

```bash
make verifier-test-stress
```

This runs tests in `tests/stress/verifier/` with a 15-minute timeout.

### Step 5: Run with Race Detection

Combine stress testing with Go's race detector for maximum coverage:

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -race -timeout 600s ./tests/stress/...
```

Note: Race detection adds significant overhead (2-10x slowdown). Use
longer timeouts when combining with stress tests.

---

## Resource Limits

All stress test execution must respect the mandatory 30-40% host
resource cap. The Makefile enforces this automatically, but manual
runs must apply limits explicitly.

### Mandatory Resource Flags

| Flag | Value | Purpose |
|------|-------|---------|
| `GOMAXPROCS` | `2` | Limits Go runtime to 2 OS threads |
| `nice -n 19` | lowest priority | CPU scheduling priority |
| `ionice -c 3` | idle class | I/O scheduling priority |
| `-p 1` | 1 package at a time | Sequential package execution |

### Full Command Template

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  [TEST_FLAGS] ./tests/stress/...
```

### Container Resource Limits

When stress tests start containers, apply memory and CPU limits:

```yaml
services:
  stress-target:
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
        reservations:
          cpus: '1.0'
          memory: 1G
```

---

## Sustained Load Testing

Sustained load tests send a constant rate of requests over an extended
period to verify that HelixAgent maintains stable latency, memory
usage, and goroutine counts.

### Step 1: Run the Sustained Load Test

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 600s \
  -run TestSustainedLoad ./tests/stress/extreme_load_test.go
```

### Step 2: Generate Sustained Load via API

Use `curl` in a loop to simulate sustained traffic:

```bash
# Send 100 requests at 2 requests/second
for i in $(seq 1 100); do
  curl -s -X POST "$HELIX_URL/v1/completions" \
    -H "Authorization: Bearer $HELIX_KEY" \
    -H "Content-Type: application/json" \
    -d '{
      "model": "helixagent-debate",
      "messages": [{"role": "user", "content": "What is 2+2?"}],
      "max_tokens": 50
    }' -o /dev/null -w "Request $i: %{http_code} %{time_total}s\n"
  sleep 0.5
done
```

### Step 3: Monitor During Sustained Load

While the test runs, observe system metrics:

```bash
# Goroutine count (should remain stable)
curl -s http://localhost:9090/metrics | grep go_goroutines

# Memory allocation (should not grow linearly)
curl -s http://localhost:9090/metrics | grep go_memstats_alloc_bytes

# Request latency (P99 should not degrade)
curl -s http://localhost:9090/metrics \
  | grep helixagent_request_duration
```

### Step 4: Validate Stability Criteria

A sustained load test passes when:

- P99 latency does not increase by more than 20% over the test duration
- Memory usage remains within 2x of the baseline
- Goroutine count remains within 10% of the starting value
- Zero HTTP 5xx errors
- No circuit breakers tripped

---

## Spike Testing

Spike tests send sudden bursts of traffic to verify that HelixAgent
handles sudden load increases without crashing or degrading.

### Step 1: Run the Spike Test

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run TestSpikeLoad ./tests/stress/extreme_load_test.go
```

### Step 2: Simulate a Spike via API

```bash
# Baseline: 1 req/s for 10 seconds
for i in $(seq 1 10); do
  curl -s -X POST "$HELIX_URL/v1/completions" \
    -H "Authorization: Bearer $HELIX_KEY" \
    -H "Content-Type: application/json" \
    -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"ping"}],"max_tokens":10}' \
    -o /dev/null -w "Baseline $i: %{http_code} %{time_total}s\n"
  sleep 1
done

# Spike: 20 concurrent requests
echo "--- SPIKE ---"
for i in $(seq 1 20); do
  curl -s -X POST "$HELIX_URL/v1/completions" \
    -H "Authorization: Bearer $HELIX_KEY" \
    -H "Content-Type: application/json" \
    -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"ping"}],"max_tokens":10}' \
    -o /dev/null -w "Spike $i: %{http_code} %{time_total}s\n" &
done
wait

# Recovery: 1 req/s for 10 seconds
echo "--- RECOVERY ---"
for i in $(seq 1 10); do
  curl -s -X POST "$HELIX_URL/v1/completions" \
    -H "Authorization: Bearer $HELIX_KEY" \
    -H "Content-Type: application/json" \
    -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"ping"}],"max_tokens":10}' \
    -o /dev/null -w "Recovery $i: %{http_code} %{time_total}s\n"
  sleep 1
done
```

### Step 3: Validate Spike Recovery

A spike test passes when:

- All spike requests receive a response (200 or 429 rate-limited)
- Recovery phase latency returns to within 30% of baseline
- No goroutine leaks after the spike
- Rate limiter engages correctly during the spike

---

## Soak Testing

Soak tests run at low-to-moderate load for extended periods (hours) to
detect slow resource leaks, gradual memory growth, and time-dependent
bugs.

### Step 1: Run the Soak Test

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 3600s \
  -run TestSoakTest ./tests/stress/memory_growth_stress_test.go
```

### Step 2: Manual Soak Test via API

```bash
# Run for 1 hour at 0.5 req/s
DURATION=3600
INTERVAL=2
START=$(date +%s)

while [ $(($(date +%s) - START)) -lt $DURATION ]; do
  curl -s -X POST "$HELIX_URL/v1/completions" \
    -H "Authorization: Bearer $HELIX_KEY" \
    -H "Content-Type: application/json" \
    -d '{"model":"helixagent-debate","messages":[{"role":"user","content":"test"}],"max_tokens":10}' \
    -o /dev/null -w "%{http_code} %{time_total}s\n"
  sleep $INTERVAL
done
```

### Step 3: Collect Profiles at Intervals

Take snapshots every 15 minutes during the soak:

```bash
# Heap profile at start
curl -s "http://localhost:6060/debug/pprof/heap" -o heap-t0.prof

# ... 15 minutes later ...
curl -s "http://localhost:6060/debug/pprof/heap" -o heap-t15.prof

# ... 30 minutes later ...
curl -s "http://localhost:6060/debug/pprof/heap" -o heap-t30.prof

# Compare growth
go tool pprof -base heap-t0.prof heap-t30.prof
```

### Step 4: Validate Soak Results

A soak test passes when:

- Memory usage at end is within 1.5x of the starting value
- Goroutine count at end is within 5% of the starting value
- No HTTP 5xx errors over the entire duration
- Database connection pool is not exhausted
- Redis connection pool is not exhausted

---

## Memory Leak Detection

### Step 1: Run Memory Leak Tests

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run TestMemoryLeak ./tests/stress/memory_leak_stress_test.go
```

### Step 2: Run Memory Pressure Tests

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run TestMemoryPressure ./tests/stress/memory_pressure_stress_test.go
```

### Step 3: Run Memory Growth Tests

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 600s \
  -run TestMemoryGrowth ./tests/stress/memory_growth_stress_test.go
```

### Step 4: Profile Memory During Tests

```bash
# Take heap profile before test
curl -s "http://localhost:6060/debug/pprof/heap" -o before.prof

# Run the test
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run TestMemoryLeak ./tests/stress/memory_leak_stress_test.go

# Take heap profile after test
curl -s "http://localhost:6060/debug/pprof/heap" -o after.prof

# Compare allocations
go tool pprof -base before.prof -top after.prof
```

### Step 5: Detect Leaks in Code

Common memory leak patterns in Go:

| Pattern | Symptom | Fix |
|---------|---------|-----|
| Unclosed response bodies | `http.Client` connections grow | `defer resp.Body.Close()` |
| Unbounded maps/slices | Heap grows linearly | Add TTL eviction or size cap |
| Goroutine leaks | Goroutine count grows | Use `context.Context` cancellation |
| Ticker not stopped | Timer goroutines accumulate | `defer ticker.Stop()` |
| Channel not closed | Blocked receivers accumulate | Close channels when done |

### Step 6: Verify with runtime.ReadMemStats

```go
func TestNoMemoryLeak(t *testing.T) {
    var before, after runtime.MemStats
    runtime.ReadMemStats(&before)

    // Run workload
    for i := 0; i < 10000; i++ {
        processRequest(ctx)
    }

    runtime.GC()
    runtime.ReadMemStats(&after)

    growth := after.HeapInuse - before.HeapInuse
    assert.Less(t, growth, uint64(10*1024*1024),
        "heap should not grow more than 10MB")
}
```

---

## Goroutine Leak Detection

### Step 1: Run Goroutine Leak Tests

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run TestGoroutineLeak ./tests/stress/goroutine_leak_stress_test.go
```

### Step 2: Run Goroutine Stability Tests

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run TestGoroutineStability \
  ./tests/stress/goroutine_stability_stress_test.go
```

### Step 3: Dump Active Goroutines

```bash
# Full goroutine dump with stack traces
curl -s "http://localhost:6060/debug/pprof/goroutine?debug=2" \
  | head -500

# Count goroutines by function
curl -s "http://localhost:6060/debug/pprof/goroutine?debug=1" \
  | grep -E "^[0-9]+ @" | sort -rn | head -20
```

### Step 4: Detect Leaks with goleak

Use Uber's `goleak` library in tests:

```go
import "go.uber.org/goleak"

func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}

func TestNoGoroutineLeak(t *testing.T) {
    defer goleak.VerifyNone(t)

    // Run the operation under test
    svc := NewService()
    svc.Start()
    svc.ProcessRequest(ctx, req)
    svc.Stop()

    // goleak verifies no extra goroutines remain
}
```

### Step 5: Verify WaitGroup Lifecycle

All background goroutines in HelixAgent follow the WaitGroup lifecycle
pattern:

```go
// Correct pattern
func (h *Handler) StartBackground() {
    h.wg.Add(1)
    go func() {
        defer h.wg.Done()
        for {
            select {
            case <-h.ctx.Done():
                return
            case task := <-h.tasks:
                h.process(task)
            }
        }
    }()
}

func (h *Handler) Shutdown() {
    h.cancel()   // Signal goroutines to stop
    h.wg.Wait()  // Wait for all to finish
}
```

---

## Circuit Breaker Testing

### Step 1: Run Circuit Breaker Stress Tests

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run TestCircuitBreaker ./tests/stress/circuit_breaker_stress_test.go
```

### Step 2: Run Cascade Failure Tests

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run TestCircuitBreakerCascade \
  ./tests/stress/circuit_breaker_cascade_stress_test.go
```

### Step 3: Run Circuit Breaker Storm Tests

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run TestCircuitBreakerStorm \
  ./tests/stress/circuit_breaker_storm_stress_test.go
```

### Step 4: Monitor Circuit Breaker State via API

```bash
# View all circuit breaker states
curl -s "$HELIX_URL/v1/monitoring/circuit-breakers" \
  -H "Authorization: Bearer $HELIX_KEY" | jq .

# Expected states: closed (healthy), open (failing), half-open (testing)
```

### Step 5: Force a Circuit Breaker Reset

```bash
curl -s -X POST "$HELIX_URL/v1/monitoring/reset-circuits" \
  -H "Authorization: Bearer $HELIX_KEY" | jq .
```

### Step 6: Validate Circuit Breaker Behavior

A circuit breaker test passes when:

- Breaker opens after the configured failure threshold (e.g., 5 failures)
- Open breaker rejects requests immediately (fast-fail)
- Half-open state allows a single probe request through
- Successful probe transitions breaker back to closed
- Cascading failures do not bring down the entire system
- Fallback providers are invoked when primary providers are open

---

## Database Pool Exhaustion Testing

### Step 1: Run Pool Exhaustion Tests

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run TestDBPoolExhaustion \
  ./tests/stress/db_pool_exhaustion_stress_test.go
```

### Step 2: Run Connection Pool Stress Tests

```bash
# HTTP client pool exhaustion
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run TestHTTPPoolExhaustion \
  ./tests/stress/http_pool_exhaustion_stress_test.go

# Worker pool overload
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run TestWorkerPoolOverload \
  ./tests/stress/worker_pool_overload_stress_test.go

# Pool saturation
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -v -p 1 -timeout 300s \
  -run TestPoolSaturation \
  ./tests/stress/pool_saturation_stress_test.go
```

### Step 3: Monitor Pool Metrics

```bash
# Database connection pool stats
curl -s "$HELIX_URL/v1/monitoring/status" \
  -H "Authorization: Bearer $HELIX_KEY" \
  | jq '.database.pool'

# Redis connection pool stats
curl -s "$HELIX_URL/v1/monitoring/status" \
  -H "Authorization: Bearer $HELIX_KEY" \
  | jq '.redis.pool'

# HTTP client pool stats
curl -s http://localhost:9090/metrics \
  | grep helixagent_http_pool
```

### Step 4: Configure Pool Limits

```yaml
database:
  pool:
    max_conns: 25           # Maximum connections
    min_conns: 5            # Minimum idle connections
    max_conn_lifetime: 1h   # Connection lifetime
    max_conn_idle_time: 30m # Idle connection timeout
    health_check_period: 1m # Health check interval

redis:
  pool:
    pool_size: 20           # Maximum connections
    min_idle_conns: 5       # Minimum idle connections
    pool_timeout: 5s        # Wait time for a free connection
```

### Step 5: Validate Pool Behavior Under Stress

A pool exhaustion test passes when:

- Requests that exceed pool capacity receive a clear error (not a hang)
- Pool recovers after load decreases (connections are returned)
- No connection leaks (pool size returns to baseline)
- Timeout errors are properly categorized in error responses
- Health checks continue to function even under pool pressure

---

## Stress Test Suite Reference

The full stress test suite in `tests/stress/` contains 60+ test files:

| Test File | Focus Area |
|-----------|------------|
| `stress_test.go` | General stress test harness |
| `api_stress_test.go` | API endpoint stress |
| `ensemble_stress_test.go` | Ensemble orchestration under load |
| `debate_stress_test.go` | Debate system stress |
| `debate_concurrent_stress_test.go` | Concurrent debate sessions |
| `debate_concurrency_stress_test.go` | Debate concurrency safety |
| `concurrent_debate_stress_test.go` | Parallel debate execution |
| `cache_stress_test.go` | Cache operations under load |
| `cache_concurrent_stress_test.go` | Concurrent cache access |
| `cache_stampede_stress_test.go` | Simultaneous cache misses |
| `cache_eviction_stress_test.go` | Cache eviction under pressure |
| `circuit_breaker_stress_test.go` | Circuit breaker behavior |
| `circuit_breaker_storm_stress_test.go` | Mass circuit breaker trips |
| `circuit_breaker_cascade_stress_test.go` | Cascading failures |
| `memory_stress_test.go` | Memory allocation patterns |
| `memory_leak_stress_test.go` | Memory leak detection |
| `memory_pressure_stress_test.go` | Low-memory conditions |
| `memory_growth_stress_test.go` | Long-term memory growth |
| `memory_system_stress_test.go` | Memory subsystem stress |
| `goroutine_leak_test.go` | Goroutine leak detection |
| `goroutine_leak_stress_test.go` | Extended goroutine leak tests |
| `goroutine_stability_stress_test.go` | Goroutine count stability |
| `concurrency_stress_test.go` | General concurrency stress |
| `concurrency_safety_test.go` | Race condition detection |
| `db_pool_exhaustion_stress_test.go` | Database pool limits |
| `http_pool_exhaustion_stress_test.go` | HTTP connection pool limits |
| `http_client_pool_stress_test.go` | HTTP client pool stress |
| `pool_stress_test.go` | Generic pool stress |
| `pool_saturation_stress_test.go` | Pool saturation behavior |
| `worker_pool_stress_test.go` | Worker pool under load |
| `worker_pool_overload_stress_test.go` | Worker pool overflow |
| `event_bus_stress_test.go` | EventBus throughput |
| `event_bus_flood_stress_test.go` | EventBus flood handling |
| `streaming_backpressure_stress_test.go` | Streaming backpressure |
| `streaming_backpressure_quantitative_stress_test.go` | Quantitative backpressure |
| `streaming_storm_stress_test.go` | SSE/WebSocket flood |
| `provider_stress_test.go` | Provider stress |
| `provider_concurrent_stress_test.go` | Concurrent provider calls |
| `provider_failover_stress_test.go` | Provider failover chains |
| `provider_fallback_stress_test.go` | Fallback behavior |
| `provider_registry_stress_test.go` | Registry under load |
| `handlers_stress_test.go` | HTTP handler stress |
| `handler_concurrent_stress_test.go` | Concurrent handler access |
| `new_handlers_stress_test.go` | New endpoint handlers |
| `middleware_stress_test.go` | Middleware chain stress |
| `rate_limiter_stress_test.go` | Rate limiter accuracy |
| `websocket_stress_test.go` | WebSocket connections |
| `mcp_adapter_stress_test.go` | MCP adapter stress |
| `health_check_stress_test.go` | Health check concurrency |
| `deadlock_detection_stress_test.go` | Deadlock detection |
| `race_condition_stress_test.go` | Race condition tests |
| `extreme_load_test.go` | Extreme load scenarios |
| `chaos_engineering_test.go` | Chaos engineering |
| `formatters_stress_test.go` | Formatter stress |
| `bigdata_stress_test.go` | BigData pipeline stress |
| `boot_manager_concurrent_stress_test.go` | Boot manager concurrency |
| `ensemble_failure_stress_test.go` | Ensemble failure handling |
| `ensemble_correctness_stress_test.go` | Ensemble correctness |
| `ensemble_all_timeout_stress_test.go` | All-provider timeout |
| `userflow_stress_test.go` | User flow stress |
| `agentic_ensemble_stress_test.go` | Agentic ensemble stress |
| `qa_vision_stress_test.go` | QA vision stress |

---

## Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GOMAXPROCS` | `2` | Maximum OS threads for Go runtime |
| `ENSEMBLE_MAX_CONCURRENT` | `5` | Max concurrent provider calls |
| `BACKGROUND_WORKER_POOL_SIZE` | `4` | Background worker count |
| `BACKGROUND_TASK_QUEUE_SIZE` | `1000` | Task queue buffer size |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `15432` | PostgreSQL port (test infra) |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `16379` | Redis port (test infra) |

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make test-stress` | Run full stress test suite |
| `make verifier-test-stress` | Run verifier stress tests |
| `make test-race` | Run tests with race detector |
| `make test-bench` | Run benchmarks |
| `make test-fuzz` | Run fuzz tests (corpus replay) |
| `make test-chaos` | Run chaos/challenge tests |

### Timeout Guidelines

| Test Category | Recommended Timeout |
|---------------|-------------------|
| Individual stress test | `300s` (5 minutes) |
| Full stress suite | `900s` (15 minutes) |
| Soak test | `3600s` (1 hour) |
| Stress + race detection | `600s` (10 minutes) |
| Verifier stress | `900s` (15 minutes) |

---

## Troubleshooting

### Stress test killed by OOM

**Symptom:** Test process is killed by the Linux OOM killer.

**Diagnosis:**

```bash
# Check kernel logs for OOM events
dmesg | grep -i "out of memory" | tail -5

# Check current memory usage
free -h
```

**Possible fixes:**

1. Reduce the number of concurrent goroutines in the test
2. Lower `GOMAXPROCS` from 2 to 1
3. Close other memory-intensive applications
4. Add `runtime.GC()` calls between test phases
5. Ensure `sync.Pool` objects are being returned

### Stress test times out

**Symptom:** Test exceeds the configured timeout and is killed.

**Diagnosis:**

```bash
# Check for deadlocks
curl -s "http://localhost:6060/debug/pprof/goroutine?debug=2" \
  | grep -A 5 "semacquire\|chan receive\|select"
```

**Possible fixes:**

1. Increase the timeout: `-timeout 600s`
2. Check for deadlocks in channel operations
3. Verify all goroutines have cancellation paths
4. Reduce the test workload size

### Database connection errors during stress

**Symptom:** `too many connections` or `connection pool exhausted`.

**Diagnosis:**

```bash
# Check PostgreSQL connection count
PGPASSWORD=helixagent123 psql -h localhost -p 15432 -U helixagent \
  -d helixagent_db -c "SELECT count(*) FROM pg_stat_activity;"
```

**Possible fixes:**

1. Increase `max_conns` in the pool configuration
2. Reduce concurrent test goroutines
3. Ensure all database connections are properly closed with `defer`
4. Add connection timeouts to prevent hanging connections

### Race condition detected

**Symptom:** `-race` flag reports data races.

**Diagnosis:** The race detector output shows the conflicting goroutines
and stack traces. Read the output carefully to identify the shared
variable.

**Possible fixes:**

1. Protect shared data with `sync.Mutex` or `sync.RWMutex`
2. Use `atomic` operations for counters and flags
3. Use channels instead of shared memory
4. Ensure map access is synchronized

### Goroutine count does not return to baseline

**Symptom:** After the stress test completes, goroutine count is higher
than the starting value.

**Diagnosis:**

```bash
# Dump goroutines and look for leaked ones
curl -s "http://localhost:6060/debug/pprof/goroutine?debug=2" \
  | grep -B 2 "created by"
```

**Possible fixes:**

1. Verify `context.Cancel()` is called for all created contexts
2. Check that `ticker.Stop()` is called for all tickers
3. Ensure all channels are eventually closed or drained
4. Add `defer wg.Done()` inside every goroutine launched with `wg.Add(1)`

### Circuit breakers stuck in open state

**Symptom:** Providers remain unavailable after the test completes.

**Fix:** Reset all circuit breakers:

```bash
curl -s -X POST "$HELIX_URL/v1/monitoring/reset-circuits" \
  -H "Authorization: Bearer $HELIX_KEY" | jq .
```

### Infrastructure containers not responding

**Symptom:** Tests fail with connection refused errors to PostgreSQL
or Redis.

**Fix:** Verify infrastructure is running:

```bash
# Check container status
docker ps --filter "name=helixagent"

# Check PostgreSQL
curl -s "$HELIX_URL/v1/health" \
  -H "Authorization: Bearer $HELIX_KEY" \
  | jq '.services.postgresql'

# Check Redis
curl -s "$HELIX_URL/v1/health" \
  -H "Authorization: Bearer $HELIX_KEY" \
  | jq '.services.redis'
```

If containers are down, restart HelixAgent which will auto-boot them:

```bash
make build && ./bin/helixagent
```

---

## Related Resources

- [User Manual 20: Testing Strategies](20-testing-strategies.md)
- [User Manual 19: Concurrency Patterns](19-concurrency-patterns.md)
- [User Manual 46: Performance Optimization Guide](46-performance-optimization-guide.md)
- [User Manual 31: Fuzz Testing Guide](31-fuzz-testing-guide.md)
- [Video Course 57: Stress Testing Guide](../video-courses/video-course-57-stress-testing-guide.md)
- [Video Course 58: Chaos Engineering](../video-courses/video-course-58-chaos-engineering.md)

---

**Document Version**: 1.0
**Last Updated**: April 10, 2026
