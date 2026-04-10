# User Manual 46: Performance Optimization Guide

**Version:** 1.0
**Last Updated:** April 10, 2026
**Audience:** Developers, Performance Engineers, System Architects

---

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Lazy Loading and Lazy Initialization](#lazy-loading-and-lazy-initialization)
4. [Semaphore-Based Concurrency Limiting](#semaphore-based-concurrency-limiting)
5. [sync.Pool Buffer Reuse](#syncpool-buffer-reuse)
6. [Prometheus Metrics Instrumentation](#prometheus-metrics-instrumentation)
7. [Health Check Parallelization](#health-check-parallelization)
8. [HTTP/3 QUIC Performance](#http3-quic-performance)
9. [Worker Pool Tuning](#worker-pool-tuning)
10. [Resource Monitoring](#resource-monitoring)
11. [Optimization Checklist](#optimization-checklist)
12. [Troubleshooting](#troubleshooting)

---

## Overview

HelixAgent processes requests through 43 LLM providers, an ensemble
debate system, and multiple middleware layers. Performance optimization
is essential for maintaining low latency and high throughput while
respecting the mandatory 30-40% host resource limit imposed by the
project Constitution.

This guide covers the concrete performance patterns used throughout
HelixAgent and provides step-by-step instructions for measuring,
tuning, and monitoring each one.

### Performance Architecture

```
Request Flow with Optimization Points
======================================

[Client] --HTTP/3+Brotli--> [Gin Router]
                               |
                    [Rate Limiter] (token bucket)
                               |
                    [Auth Middleware] (JWT cache)
                               |
                    [Handler] ---> [sync.Pool buffers]
                       |
              [Ensemble Orchestrator]
                  |         |         |
          [Semaphore: max 5 concurrent]
                  |         |         |
             [Provider A] [Provider B] [Provider C]
                  |         |         |
          [Response Cache] (Redis L2 + in-memory L1)
                  |
          [Early Termination on Consensus]
                  |
          [Brotli Compress] --> [Client]
```

---

## Prerequisites

- HelixAgent source tree with all submodules initialized
- Go 1.25+ with `go test -bench` support
- Docker or Podman for infrastructure containers
- Infrastructure running: `./bin/helixagent` (auto-boots containers)
- `curl` 7.x+ for API testing
- `pprof` (bundled with Go) for profiling
- Prometheus (optional) for metrics collection

Set up environment variables:

```bash
export HELIX_KEY="your-helixagent-api-key"
export HELIX_URL="http://localhost:7061"
```

---

## Lazy Loading and Lazy Initialization

Lazy loading defers initialization of expensive resources (HTTP
clients, database connections, provider registrations) until they are
first needed. This reduces startup time and avoids allocating resources
that may never be used.

### Step 1: Identify Eager Initialization

Search for `init()` functions and global variable initializations that
perform expensive work:

```bash
grep -rn "func init()" internal/ --include="*.go" | head -20
```

### Step 2: Convert to sync.Once Pattern

Replace `init()` with lazy initialization using `sync.Once`:

```go
// BEFORE: Eager initialization
var httpClient *http.Client

func init() {
    httpClient = &http.Client{
        Timeout:   30 * time.Second,
        Transport: buildTransport(), // Expensive
    }
}

// AFTER: Lazy initialization
var (
    httpClient     *http.Client
    httpClientOnce sync.Once
)

func getHTTPClient() *http.Client {
    httpClientOnce.Do(func() {
        httpClient = &http.Client{
            Timeout:   30 * time.Second,
            Transport: buildTransport(),
        }
    })
    return httpClient
}
```

### Step 3: Handle Initialization Errors

For resources that can fail during initialization:

```go
var (
    dbPool     *pgxpool.Pool
    dbPoolOnce sync.Once
    dbPoolErr  error
)

func getDBPool() (*pgxpool.Pool, error) {
    dbPoolOnce.Do(func() {
        config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
        if err != nil {
            dbPoolErr = fmt.Errorf("parse database config: %w", err)
            return
        }
        dbPool, dbPoolErr = pgxpool.NewWithConfig(
            context.Background(), config,
        )
    })
    return dbPool, dbPoolErr
}
```

### Step 4: Measure Startup Improvement

Run the startup benchmark before and after converting to lazy loading:

```bash
# Measure startup time
time ./bin/helixagent --dry-run 2>/dev/null

# Run lazy loading benchmarks
go test -bench=BenchmarkLazyInit -benchmem ./internal/...
```

### HelixAgent Lazy Loading Savings

| Component | Before (init) | After (lazy) | Savings |
|-----------|---------------|--------------|---------|
| HTTP clients (43 providers) | ~500ms | 0ms (deferred) | ~500ms |
| Database connection pool | ~200ms | 0ms (deferred) | ~200ms |
| Redis client | ~100ms | 0ms (deferred) | ~100ms |
| Provider registry | ~300ms | 0ms (deferred) | ~300ms |
| Model discovery cache | ~2s | 0ms (deferred) | ~2s |
| Formatter registry | ~400ms | 0ms (deferred) | ~400ms |

---

## Semaphore-Based Concurrency Limiting

The ensemble system calls multiple LLM providers in parallel. Without
limiting, this can exhaust connections, trigger provider rate limits,
and overload the host. A channel-based semaphore bounds the number of
concurrent operations.

### Step 1: Understand the Semaphore Pattern

```go
type EnsembleOrchestrator struct {
    semaphore chan struct{}
}

func NewEnsembleOrchestrator(maxConcurrent int) *EnsembleOrchestrator {
    return &EnsembleOrchestrator{
        semaphore: make(chan struct{}, maxConcurrent),
    }
}

func (e *EnsembleOrchestrator) CallProvider(
    ctx context.Context, provider Provider,
) (*Response, error) {
    // Acquire semaphore slot (blocks if full)
    select {
    case e.semaphore <- struct{}{}:
        // Slot acquired
    case <-ctx.Done():
        return nil, ctx.Err()
    }
    // Release slot when done
    defer func() { <-e.semaphore }()

    return provider.Complete(ctx)
}
```

### Step 2: Configure the Concurrency Limit

The default limit is 5 concurrent provider calls. Adjust via
environment variable:

```bash
# Increase to 8 for high-throughput scenarios
export ENSEMBLE_MAX_CONCURRENT=8

# Decrease to 3 for resource-constrained hosts
export ENSEMBLE_MAX_CONCURRENT=3
```

### Step 3: Use Weighted Semaphores for Heterogeneous Costs

When providers have different resource costs, use a weighted semaphore:

```go
import "golang.org/x/sync/semaphore"

sem := semaphore.NewWeighted(10) // Total weight budget: 10

// Heavy provider: weight 3
sem.Acquire(ctx, 3)
defer sem.Release(3)

// Light provider: weight 1
sem.Acquire(ctx, 1)
defer sem.Release(1)
```

### Step 4: Monitor Semaphore Utilization

Check the current concurrency via the monitoring endpoint:

```bash
curl -s "$HELIX_URL/v1/monitoring/status" \
  -H "Authorization: Bearer $HELIX_KEY" | jq '.ensemble'
```

---

## sync.Pool Buffer Reuse

`sync.Pool` recycles allocated objects (byte buffers, JSON encoders,
response writers) to reduce garbage collection pressure. This is
critical for high-throughput request processing.

### Step 1: Create a Buffer Pool

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func processRequest(data []byte) ([]byte, error) {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()

    buf.Write(data)
    // ... process ...
    return buf.Bytes(), nil
}
```

### Step 2: Pool JSON Encoders

```go
var encoderPool = sync.Pool{
    New: func() interface{} {
        return json.NewEncoder(io.Discard)
    },
}

func encodeResponse(w io.Writer, v interface{}) error {
    enc := encoderPool.Get().(*json.Encoder)
    defer encoderPool.Put(enc)

    enc.Reset(w)
    return enc.Encode(v)
}
```

### Step 3: Benchmark Pool Effectiveness

```bash
go test -bench=BenchmarkBufferPool -benchmem ./internal/handlers/...
```

Expected improvement: 60-80% reduction in allocations per request.

### Step 4: Avoid Common sync.Pool Mistakes

- Always call `Reset()` before returning objects to the pool
- Never store pointers to pooled objects beyond the request scope
- Do not assume objects survive between GC cycles
- Size pools for the 90th percentile workload, not the maximum

---

## Prometheus Metrics Instrumentation

HelixAgent exposes Prometheus metrics on port 9090 for monitoring
request latency, provider health, cache hit rates, and resource usage.

### Step 1: Verify Prometheus Configuration

```yaml
# configs/production.yaml
monitoring:
  prometheus:
    enabled: true
    port: 9090
```

### Step 2: Query Key Metrics

```bash
# Request latency histogram
curl -s http://localhost:9090/metrics | grep helixagent_request_duration

# Provider response times
curl -s http://localhost:9090/metrics | grep helixagent_provider_latency

# Cache hit/miss ratio
curl -s http://localhost:9090/metrics | grep helixagent_cache_hits

# Active goroutines
curl -s http://localhost:9090/metrics | grep go_goroutines

# Memory usage
curl -s http://localhost:9090/metrics | grep go_memstats_alloc_bytes
```

### Step 3: Add Custom Metrics

Register counters, histograms, and gauges in your code:

```go
import "github.com/prometheus/client_golang/prometheus"

var requestDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "helixagent_request_duration_seconds",
        Help:    "Request duration in seconds",
        Buckets: prometheus.DefBuckets,
    },
    []string{"method", "endpoint", "status"},
)

func init() {
    prometheus.MustRegister(requestDuration)
}

// In your handler:
timer := prometheus.NewTimer(requestDuration.WithLabelValues(
    "POST", "/v1/completions", "200",
))
defer timer.ObserveDuration()
```

### Step 4: Set Up Alerting Thresholds

Monitor these metrics for performance degradation:

| Metric | Warning Threshold | Critical Threshold |
|--------|-------------------|-------------------|
| Request P99 latency | > 5s | > 15s |
| Provider error rate | > 5% | > 20% |
| Cache hit rate | < 70% | < 50% |
| Goroutine count | > 5000 | > 10000 |
| Memory usage | > 2 GB | > 4 GB |

---

## Health Check Parallelization

HelixAgent checks the health of 43 providers, databases, caches, and
external services. Running health checks sequentially takes too long.
Parallel execution with bounded concurrency is essential.

### Step 1: Understand the Parallel Health Check Pattern

```go
func (h *HealthChecker) CheckAll(ctx context.Context) []HealthResult {
    var (
        wg      sync.WaitGroup
        mu      sync.Mutex
        results []HealthResult
    )

    sem := make(chan struct{}, 10) // Max 10 concurrent checks

    for _, svc := range h.services {
        wg.Add(1)
        go func(s Service) {
            defer wg.Done()

            sem <- struct{}{}        // Acquire
            defer func() { <-sem }() // Release

            result := s.HealthCheck(ctx)
            mu.Lock()
            results = append(results, result)
            mu.Unlock()
        }(svc)
    }

    wg.Wait()
    return results
}
```

### Step 2: Query Health Status

```bash
# Full health check (all services)
curl -s "$HELIX_URL/v1/health" \
  -H "Authorization: Bearer $HELIX_KEY" | jq .

# Provider-specific health
curl -s "$HELIX_URL/v1/monitoring/provider-health" \
  -H "Authorization: Bearer $HELIX_KEY" | jq .

# Circuit breaker status
curl -s "$HELIX_URL/v1/monitoring/circuit-breakers" \
  -H "Authorization: Bearer $HELIX_KEY" | jq .
```

### Step 3: Force a Health Check Refresh

```bash
curl -s -X POST "$HELIX_URL/v1/monitoring/force-health-check" \
  -H "Authorization: Bearer $HELIX_KEY" | jq .
```

### Step 4: Tune Health Check Intervals

Configure check intervals and timeouts:

```yaml
health:
  check_interval: 30s      # How often to run checks
  check_timeout: 5s         # Per-service timeout
  max_concurrent_checks: 10 # Parallel check limit
  failure_threshold: 3      # Failures before marking unhealthy
  success_threshold: 1      # Successes before marking healthy
```

---

## HTTP/3 QUIC Performance

HelixAgent uses HTTP/3 (QUIC) as the primary transport protocol with
Brotli compression. QUIC reduces connection establishment latency
through 0-RTT handshakes and eliminates head-of-line blocking.

### Step 1: Verify HTTP/3 Is Active

```bash
# Check server capabilities
curl -s "$HELIX_URL/v1/health" \
  -H "Authorization: Bearer $HELIX_KEY" | jq '.transport'
```

### Step 2: Understand the Transport Stack

```
Transport Priority:
  1. HTTP/3 (QUIC) -- Primary (quic-go/quic-go)
  2. HTTP/2         -- Fallback when QUIC unavailable

Compression Priority:
  1. Brotli         -- Primary (andybalholm/brotli)
  2. gzip           -- Fallback
```

### Step 3: Configure QUIC Parameters

```yaml
server:
  http3:
    enabled: true
    max_streams: 256           # Max concurrent streams per connection
    idle_timeout: 30s          # Connection idle timeout
    max_incoming_streams: 256  # Max incoming unidirectional streams
```

### Step 4: Test Compression Effectiveness

```bash
# Request with Brotli accept header
curl -s -H "Accept-Encoding: br" \
  -H "Authorization: Bearer $HELIX_KEY" \
  "$HELIX_URL/v1/health" \
  -o /dev/null -w "Size: %{size_download} bytes\n"

# Request without compression
curl -s -H "Accept-Encoding: identity" \
  -H "Authorization: Bearer $HELIX_KEY" \
  "$HELIX_URL/v1/health" \
  -o /dev/null -w "Size: %{size_download} bytes\n"
```

### Step 5: Monitor QUIC Connection Metrics

```bash
# QUIC-specific metrics
curl -s http://localhost:9090/metrics | grep quic_

# Connection migration events
curl -s http://localhost:9090/metrics | grep helixagent_quic_migrations
```

---

## Worker Pool Tuning

HelixAgent uses worker pools for background tasks (cache invalidation,
model refresh, debate log tracking, notification delivery). Proper
tuning prevents goroutine leaks and ensures timely task completion.

### Step 1: Understand the Worker Pool Architecture

```
Worker Pool Architecture
========================

[Task Queue] --> [Dispatcher] --> [Worker 1]
                              --> [Worker 2]
                              --> [Worker 3]
                              --> [Worker N]
                                     |
                              [Result Channel]
                                     |
                              [Result Handler]
```

### Step 2: Configure Pool Size

```yaml
background:
  worker_pool_size: 4       # Number of worker goroutines
  task_queue_size: 1000     # Buffered channel capacity
  task_timeout: 30s         # Per-task timeout
  shutdown_timeout: 10s     # Grace period on shutdown
```

Environment variable override:

```bash
export BACKGROUND_WORKER_POOL_SIZE=8
export BACKGROUND_TASK_QUEUE_SIZE=2000
```

### Step 3: Monitor Worker Pool Utilization

```bash
# Background task status
curl -s "$HELIX_URL/v1/tasks" \
  -H "Authorization: Bearer $HELIX_KEY" | jq '.pool_stats'

# Active workers and queue depth
curl -s "$HELIX_URL/v1/monitoring/status" \
  -H "Authorization: Bearer $HELIX_KEY" | jq '.background_tasks'
```

### Step 4: Verify Graceful Shutdown

Worker pools use `sync.WaitGroup` to ensure all in-flight tasks
complete before process exit:

```go
func (p *WorkerPool) Shutdown() {
    p.cancel()         // Signal workers to stop
    p.wg.Wait()        // Wait for all workers to finish
    close(p.taskQueue) // Close the task channel
}
```

Verify shutdown behavior:

```bash
# Send SIGTERM and observe clean shutdown
kill -TERM $(pgrep helixagent)

# Check logs for "all workers stopped" message
grep "workers stopped" /tmp/helixagent-server.log
```

---

## Resource Monitoring

### Step 1: Enable pprof Profiling

pprof is enabled by default on the debug port:

```bash
# CPU profile (30 seconds)
curl -s "http://localhost:6060/debug/pprof/profile?seconds=30" \
  -o cpu.prof
go tool pprof -http=:8080 cpu.prof

# Memory profile
curl -s "http://localhost:6060/debug/pprof/heap" -o heap.prof
go tool pprof -http=:8080 heap.prof

# Goroutine dump
curl -s "http://localhost:6060/debug/pprof/goroutine?debug=2"

# Block profile (contention)
curl -s "http://localhost:6060/debug/pprof/block" -o block.prof
go tool pprof -http=:8080 block.prof
```

### Step 2: Monitor Runtime Metrics

```bash
# System resource usage
curl -s "$HELIX_URL/v1/monitoring/status" \
  -H "Authorization: Bearer $HELIX_KEY" | jq '.system'
```

### Step 3: Run Benchmarks

```bash
# All benchmarks with memory allocation stats
nice -n 19 ionice -c 3 \
  go test -bench=. -benchmem -count=3 ./internal/... 2>&1 \
  | tee benchmark-results.txt

# Specific component benchmark
go test -bench=BenchmarkEnsemble -benchmem ./internal/llm/...

# Compare benchmark results across runs
go install golang.org/x/perf/cmd/benchstat@latest
benchstat old.txt new.txt
```

### Step 4: Set Resource Limits

All test and benchmark execution must respect the 30-40% resource cap:

```bash
# Resource-limited benchmark run
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -p 1 -bench=. -benchmem ./internal/...
```

---

## Optimization Checklist

### Before Deployment

- [ ] All `init()` functions converted to lazy loading where appropriate
- [ ] Semaphore limits configured for ensemble concurrency
- [ ] sync.Pool used for frequently allocated buffers
- [ ] Prometheus metrics registered for key operations
- [ ] Health checks run in parallel with bounded concurrency
- [ ] HTTP/3 and Brotli compression enabled
- [ ] Worker pool sizes tuned for workload
- [ ] pprof endpoint accessible for debugging

### Ongoing Monitoring

- [ ] Request P99 latency below 5 seconds
- [ ] Cache hit rate above 70%
- [ ] Goroutine count stable (no leaks)
- [ ] Memory usage within expected bounds
- [ ] No semaphore starvation (check wait times)
- [ ] Worker pool queue depth not growing unbounded
- [ ] Circuit breakers not in half-open state excessively

---

## Troubleshooting

### High request latency

**Symptom:** P99 latency exceeds 10 seconds.

**Diagnosis:**

```bash
# Check provider response times
curl -s "$HELIX_URL/v1/monitoring/provider-health" \
  -H "Authorization: Bearer $HELIX_KEY" | jq '.providers[] | {name, latency_ms}'

# Check for semaphore contention
curl -s http://localhost:9090/metrics | grep semaphore_wait
```

**Possible fixes:**

1. Increase `ENSEMBLE_MAX_CONCURRENT` if providers are fast
2. Enable early termination on consensus
3. Reduce the number of debate participants
4. Check if a slow provider is dragging down the ensemble

### Memory growing continuously

**Symptom:** `go_memstats_alloc_bytes` keeps increasing over time.

**Diagnosis:**

```bash
# Take heap profiles at two points in time
curl -s "http://localhost:6060/debug/pprof/heap" -o heap1.prof
# Wait 5 minutes
curl -s "http://localhost:6060/debug/pprof/heap" -o heap2.prof

# Compare
go tool pprof -base heap1.prof heap2.prof
```

**Possible fixes:**

1. Check for goroutine leaks: `curl http://localhost:6060/debug/pprof/goroutine?debug=2`
2. Verify sync.Pool objects are being returned after use
3. Check for unbounded caches without TTL eviction
4. Review response body close patterns (`defer resp.Body.Close()`)

### Goroutine count keeps increasing

**Symptom:** `go_goroutines` metric grows without bound.

**Diagnosis:**

```bash
# Dump all goroutines with stack traces
curl -s "http://localhost:6060/debug/pprof/goroutine?debug=2" \
  | head -200
```

**Possible fixes:**

1. Ensure all goroutines have a cancellation path via `context.Context`
2. Verify `sync.WaitGroup` lifecycle: `Add(1)` before launch, `defer Done()` inside
3. Check for blocked channel operations without timeouts
4. Review SSE and WebSocket handlers for proper cleanup

### Cache hit rate is low

**Symptom:** `helixagent_cache_hits` / total requests is below 50%.

**Diagnosis:**

```bash
curl -s http://localhost:9090/metrics \
  | grep -E "helixagent_cache_(hits|misses)"
```

**Possible fixes:**

1. Increase cache TTL for stable responses
2. Verify cache key generation includes the right parameters
3. Check Redis connectivity and memory limits
4. Review L1 (in-memory) vs L2 (Redis) cache configuration

### Worker pool tasks backing up

**Symptom:** Task queue depth grows, tasks are delayed.

**Diagnosis:**

```bash
curl -s "$HELIX_URL/v1/tasks" \
  -H "Authorization: Bearer $HELIX_KEY" \
  | jq '.pool_stats.queue_depth'
```

**Possible fixes:**

1. Increase `BACKGROUND_WORKER_POOL_SIZE`
2. Check for tasks with long execution times
3. Verify task timeouts are configured
4. Review whether all queued tasks are necessary

---

## Related Resources

- [User Manual 11: Performance Tuning](11-performance-tuning.md)
- [User Manual 18: Performance Monitoring](18-performance-monitoring.md)
- [User Manual 19: Concurrency Patterns](19-concurrency-patterns.md)
- [User Manual 33: Performance Optimization Guide](33-performance-optimization-guide.md)
- [Video Course 75: Performance Tuning](../video-courses/course-75-performance-tuning.md)
- [Video Course 56: Performance Optimization](../video-courses/video-course-56-performance-optimization.md)

---

**Document Version**: 1.0
**Last Updated**: April 10, 2026
