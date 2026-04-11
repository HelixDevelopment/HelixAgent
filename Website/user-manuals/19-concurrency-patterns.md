# User Manual 19: Concurrency Patterns

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Concurrency Architecture in HelixAgent](#concurrency-architecture-in-helixagent)
4. [Mutex and RWMutex](#mutex-and-rwmutex)
5. [Channels and Select](#channels-and-select)
6. [sync.Once and Lazy Initialization](#synconce-and-lazy-initialization)
7. [Worker Pools](#worker-pools)
8. [Semaphores](#semaphores)
9. [Circuit Breakers](#circuit-breakers)
10. [Rate Limiters](#rate-limiters)
11. [Context Cancellation and Timeouts](#context-cancellation-and-timeouts)
12. [Patterns Used in Ensemble and Debate](#patterns-used-in-ensemble-and-debate)
13. [Best Practices](#best-practices)
14. [Testing Concurrent Code](#testing-concurrent-code)
15. [Troubleshooting](#troubleshooting)
16. [Related Resources](#related-resources)

## Overview

HelixAgent is a highly concurrent system that orchestrates parallel requests across 47+ LLM providers, runs multi-round debate sessions with ensemble voting, manages connection pools to PostgreSQL and Redis, and processes background tasks. This manual covers the concurrency patterns used throughout the codebase and how to apply them correctly.

The Concurrency module (`digital.vasic.concurrency`) provides reusable primitives: worker pools, priority queues, rate limiters (token bucket and sliding window), circuit breakers, semaphores, and resource monitoring. These are used across the HelixAgent core and all extracted modules.

## Prerequisites

- Familiarity with Go goroutines, channels, and the `sync` package
- Understanding of `context.Context` for cancellation and deadlines
- Go 1.24+ (required by HelixAgent)
- Access to the Concurrency module source: `Concurrency/`

## Concurrency Architecture in HelixAgent

```
                    +---------------------------+
                    |     HTTP Server (Gin)     |
                    |  (goroutine per request)  |
                    +------------+--------------+
                                 |
                    +------------v--------------+
                    |   Rate Limiter Middleware  |
                    |  (token bucket / sliding)  |
                    +------------+--------------+
                                 |
              +------------------+------------------+
              |                  |                   |
     +--------v------+  +-------v-------+  +--------v--------+
     | Provider Pool  |  | Debate Engine |  | Background Tasks |
     | (semaphore)    |  | (worker pool) |  | (task queue)     |
     +--------+------+  +-------+-------+  +--------+--------+
              |                  |                   |
     +--------v------+  +-------v-------+  +--------v--------+
     | Circuit Breaker|  | Ensemble Vote |  | Resource Monitor |
     | (per provider) |  | (fan-out/in)  |  | (periodic check) |
     +---------------+  +---------------+  +-----------------+
```

## Mutex and RWMutex

### sync.Mutex

Use `sync.Mutex` when both reads and writes must be serialized:

```go
type ProviderRegistry struct {
    mu        sync.Mutex
    providers map[string]LLMProvider
}

func (r *ProviderRegistry) Register(name string, p LLMProvider) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.providers[name] = p
}

func (r *ProviderRegistry) Get(name string) (LLMProvider, bool) {
    r.mu.Lock()
    defer r.mu.Unlock()
    p, ok := r.providers[name]
    return p, ok
}
```

### sync.RWMutex

Use `sync.RWMutex` when reads significantly outnumber writes, such as in configuration or cache lookups:

```go
type ConfigStore struct {
    mu     sync.RWMutex
    config map[string]string
}

func (c *ConfigStore) Get(key string) string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.config[key]
}

func (c *ConfigStore) Set(key, value string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.config[key] = value
}
```

### Double-Checked Locking

HelixAgent uses double-checked locking for the HTTP global pool (see `internal/http/`):

```go
var (
    pool     *ConnectionPool
    poolOnce sync.Mutex
)

func GlobalPool() *ConnectionPool {
    if p := atomicLoadPool(); p != nil {
        return p // fast path, no lock
    }
    poolOnce.Lock()
    defer poolOnce.Unlock()
    if pool == nil {
        pool = newConnectionPool()
    }
    return pool
}
```

## Channels and Select

### Fan-Out / Fan-In for Parallel Provider Queries

```go
func queryProviders(ctx context.Context, providers []LLMProvider, req *LLMRequest) []*LLMResponse {
    results := make(chan *LLMResponse, len(providers))

    // Fan-out: launch all providers in parallel
    for _, p := range providers {
        go func(provider LLMProvider) {
            resp, err := provider.Complete(ctx, req)
            if err != nil {
                results <- nil
                return
            }
            results <- resp
        }(p)
    }

    // Fan-in: collect all results
    var responses []*LLMResponse
    for range providers {
        if r := <-results; r != nil {
            responses = append(responses, r)
        }
    }
    return responses
}
```

### Select with Timeout

```go
func queryWithTimeout(ctx context.Context, provider LLMProvider, req *LLMRequest) (*LLMResponse, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    resultCh := make(chan *LLMResponse, 1)
    errCh := make(chan error, 1)

    go func() {
        resp, err := provider.Complete(ctx, req)
        if err != nil {
            errCh <- err
            return
        }
        resultCh <- resp
    }()

    select {
    case resp := <-resultCh:
        return resp, nil
    case err := <-errCh:
        return nil, err
    case <-ctx.Done():
        return nil, fmt.Errorf("provider query timed out: %w", ctx.Err())
    }
}
```

## sync.Once and Lazy Initialization

HelixAgent uses lazy initialization extensively to avoid startup overhead:

```go
var (
    defaultRegistry     *ToolRegistry
    defaultRegistryOnce sync.Once
)

// GetDefaultToolRegistry returns the lazily-initialized tool registry.
func GetDefaultToolRegistry() *ToolRegistry {
    defaultRegistryOnce.Do(func() {
        defaultRegistry = &ToolRegistry{
            tools: make(map[string]Tool),
        }
        defaultRegistry.registerBuiltins()
    })
    return defaultRegistry
}
```

This pattern ensures thread-safe, one-time initialization regardless of how many goroutines call the function concurrently.

## Worker Pools

### Basic Worker Pool

```go
type WorkerPool struct {
    workers int
    jobs    chan func()
    wg      sync.WaitGroup
}

func NewWorkerPool(workers, queueSize int) *WorkerPool {
    return &WorkerPool{
        workers: workers,
        jobs:    make(chan func(), queueSize),
    }
}

func (p *WorkerPool) Start(ctx context.Context) {
    for i := 0; i < p.workers; i++ {
        p.wg.Add(1)
        go func() {
            defer p.wg.Done()
            for {
                select {
                case job, ok := <-p.jobs:
                    if !ok {
                        return
                    }
                    job()
                case <-ctx.Done():
                    return
                }
            }
        }()
    }
}

func (p *WorkerPool) Submit(job func()) {
    p.jobs <- job
}

func (p *WorkerPool) Shutdown() {
    close(p.jobs)
    p.wg.Wait()
}
```

### Priority Worker Pool

The Concurrency module provides a priority-based worker pool where high-priority tasks (such as debate convergence checks) execute before low-priority tasks (such as cache warming):

```go
pool := concurrency.NewPriorityPool(concurrency.PriorityPoolConfig{
    Workers:   4,
    QueueSize: 100,
})
pool.Start(ctx)

pool.Submit(concurrency.PriorityHigh, func() {
    // Critical task: debate round execution
})
pool.Submit(concurrency.PriorityLow, func() {
    // Background task: metrics aggregation
})
```

## Semaphores

Semaphores limit the number of concurrent operations, preventing resource exhaustion when calling external APIs:

```go
type Semaphore struct {
    ch chan struct{}
}

func NewSemaphore(maxConcurrency int) *Semaphore {
    return &Semaphore{ch: make(chan struct{}, maxConcurrency)}
}

func (s *Semaphore) Acquire(ctx context.Context) error {
    select {
    case s.ch <- struct{}{}:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (s *Semaphore) Release() {
    <-s.ch
}
```

Usage in the debate performance optimizer:

```go
// Limit to 5 concurrent LLM calls
sem := NewSemaphore(5)

for _, provider := range activeProviders {
    go func(p LLMProvider) {
        if err := sem.Acquire(ctx); err != nil {
            return
        }
        defer sem.Release()
        p.Complete(ctx, req)
    }(provider)
}
```

## Circuit Breakers

Circuit breakers protect the system from cascading failures when LLM providers become unresponsive:

```go
type CircuitBreaker struct {
    mu          sync.Mutex
    state       State // Closed, HalfOpen, Open
    failures    int
    threshold   int
    resetAfter  time.Duration
    lastFailure time.Time
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
    cb.mu.Lock()
    if cb.state == Open {
        if time.Since(cb.lastFailure) > cb.resetAfter {
            cb.state = HalfOpen
        } else {
            cb.mu.Unlock()
            return ErrCircuitOpen
        }
    }
    cb.mu.Unlock()

    err := fn()

    cb.mu.Lock()
    defer cb.mu.Unlock()
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        if cb.failures >= cb.threshold {
            cb.state = Open
        }
        return err
    }
    cb.failures = 0
    cb.state = Closed
    return nil
}
```

Each of the 22+ providers has its own circuit breaker. View states via:

```bash
curl -s http://localhost:7061/v1/monitoring/circuit-breakers | jq .
```

## Rate Limiters

### Token Bucket

Limits burst traffic while allowing sustained throughput:

```go
limiter := concurrency.NewTokenBucketLimiter(concurrency.TokenBucketConfig{
    Rate:       100, // 100 requests per second
    BurstSize:  200, // allow bursts up to 200
})

if !limiter.Allow() {
    return ErrRateLimited
}
```

### Sliding Window

Provides precise per-interval counting without burst allowances:

```go
limiter := concurrency.NewSlidingWindowLimiter(concurrency.SlidingWindowConfig{
    Window:    time.Minute,
    MaxCount:  600, // 600 requests per minute
    Precision: time.Second,
})
```

Both implementations support Redis-backed distributed rate limiting for multi-instance deployments.

## Context Cancellation and Timeouts

Every function that performs I/O or calls external services must accept `context.Context` as its first parameter:

```go
func (s *EnsembleService) Execute(ctx context.Context, req *EnsembleRequest) (*EnsembleResponse, error) {
    // Set a hard timeout for the entire ensemble operation
    ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
    defer cancel()

    // Pass context down to all provider calls
    responses, err := s.queryAllProviders(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("ensemble query failed: %w", err)
    }

    // Context cancellation propagates to all in-flight goroutines
    return s.vote(ctx, responses)
}
```

Use `context.WithCancel` for early termination on consensus detection in debates:

```go
ctx, cancel := context.WithCancel(parentCtx)
defer cancel()

for round := 0; round < maxRounds; round++ {
    result := executeRound(ctx, round)
    if result.ConsensusReached {
        cancel() // Stop all remaining provider calls
        return result, nil
    }
}
```

## Patterns Used in Ensemble and Debate

### Parallel Provider Execution with Semaphore

The debate performance optimizer runs LLM calls in parallel with a configurable concurrency limit:

```go
func (o *Optimizer) ExecuteParallel(ctx context.Context, providers []LLMProvider, req *LLMRequest) []*LLMResponse {
    sem := make(chan struct{}, o.maxConcurrency)
    var mu sync.Mutex
    var results []*LLMResponse

    var wg sync.WaitGroup
    for _, p := range providers {
        wg.Add(1)
        go func(provider LLMProvider) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()

            resp, err := provider.Complete(ctx, req)
            if err != nil {
                return
            }
            mu.Lock()
            results = append(results, resp)
            mu.Unlock()
        }(p)
    }
    wg.Wait()
    return results
}
```

### Early Termination on Consensus

```go
func (e *EnsembleEngine) executeWithEarlyTermination(ctx context.Context, providers []LLMProvider, req *LLMRequest) *LLMResponse {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    results := make(chan *LLMResponse, len(providers))

    for _, p := range providers {
        go func(provider LLMProvider) {
            resp, _ := provider.Complete(ctx, req)
            if resp != nil {
                results <- resp
            }
        }(p)
    }

    var collected []*LLMResponse
    for i := 0; i < len(providers); i++ {
        select {
        case r := <-results:
            collected = append(collected, r)
            if consensus := checkConsensus(collected); consensus != nil {
                cancel() // stop remaining providers
                return consensus
            }
        case <-ctx.Done():
            return bestFromCollected(collected)
        }
    }
    return vote(collected)
}
```

## Best Practices

1. **Always use `context.Context`** -- Pass context as the first parameter to all functions involving I/O. Never use `context.Background()` in production request paths.

2. **Prefer `sync.RWMutex` for read-heavy data** -- Provider registries, configuration stores, and cache lookups benefit from concurrent readers.

3. **Close channels from the sender** -- The goroutine that writes to a channel should be the one to close it. Never close from the receiver side.

4. **Use `defer` for unlocking** -- Always `defer mu.Unlock()` immediately after `mu.Lock()` to prevent deadlocks from early returns or panics.

5. **Bound all goroutines** -- Never launch unbounded goroutines. Use worker pools or semaphores to limit concurrency. HelixAgent resource limits: `GOMAXPROCS=2` for tests, configurable concurrency for production.

6. **Avoid goroutine leaks** -- Every goroutine must have a termination condition: context cancellation, channel close, or explicit done signal.

7. **Prefer buffered channels for known sizes** -- When the number of results is known (e.g., number of providers), use a buffered channel of that size to prevent goroutine blocking.

8. **Use `sync.WaitGroup` to await goroutine completion** -- Before returning results from fan-out operations, wait for all goroutines to finish.

## Testing Concurrent Code

### Race Detection

```bash
# Run all tests with the race detector
make test-race

# Run a specific test with race detection
go test -race -v -run TestEnsembleParallel ./internal/services/...
```

### Stress Testing

```bash
# Run stress tests (limited to 30-40% resources)
GOMAXPROCS=2 nice -n 19 ionice -c 3 go test -v -p 1 -run TestStress ./tests/stress/...
```

### Testing Channel Behavior

```go
func TestWorkerPool_Shutdown(t *testing.T) {
    pool := NewWorkerPool(4, 10)
    ctx, cancel := context.WithCancel(context.Background())
    pool.Start(ctx)

    var count int64
    for i := 0; i < 100; i++ {
        pool.Submit(func() {
            atomic.AddInt64(&count, 1)
        })
    }

    cancel()
    pool.Shutdown()
    assert.Equal(t, int64(100), atomic.LoadInt64(&count))
}
```

## Troubleshooting

### Goroutine Leak Detected

**Symptom:** `helixagent_goroutines` metric increases over time without decreasing.

**Solutions:**
1. Run with `GODEBUG=gctrace=1` to observe GC and goroutine counts
2. Use `pprof` to inspect goroutines: `curl http://localhost:7061/debug/pprof/goroutine?debug=2`
3. Check for missing context cancellation in provider calls
4. Verify all channels are closed after use

### Deadlock on Mutex

**Symptom:** Application hangs; requests time out.

**Solutions:**
1. Ensure `defer mu.Unlock()` is always used after `Lock()`
2. Never hold two locks in different orders across goroutines
3. Use `go test -race` to detect data races (which often accompany deadlocks)
4. Check for recursive locking (Go mutexes are not reentrant)

### Channel Send Blocks Forever

**Symptom:** Goroutine blocked on channel send, visible in pprof stack trace.

**Solutions:**
1. Use buffered channels with adequate capacity
2. Ensure the receiving side is always active or the channel is properly drained
3. Use `select` with a timeout or context cancellation as a safety valve

## WaitGroup Lifecycle Pattern for Handlers

All HTTP handlers and background services in HelixAgent that launch goroutines must implement the WaitGroup lifecycle pattern for graceful shutdown. This pattern was applied across 9 components during the v1.3.1 remediation.

### Pattern Structure

```go
type Service struct {
    wg     sync.WaitGroup
    ctx    context.Context
    cancel context.CancelFunc
}

func NewService() *Service {
    ctx, cancel := context.WithCancel(context.Background())
    return &Service{ctx: ctx, cancel: cancel}
}

func (s *Service) Start() {
    s.wg.Add(1)           // MUST be before go func()
    go func() {
        defer s.wg.Done() // Signals completion
        for {
            select {
            case <-s.ctx.Done():
                return    // Exit on cancellation
            case work := <-s.workCh:
                s.process(work)
            }
        }
    }()
}

func (s *Service) Shutdown() {
    s.cancel()   // Signal goroutine to stop
    s.wg.Wait()  // Block until goroutine exits
}
```

### Where This Pattern Is Applied

| Component | File | Goroutine Purpose |
|---|---|---|
| SSE Handler | `internal/handlers/sse_handler.go` | Event broadcasting |
| Cache Invalidation | `internal/cache/invalidation.go` | Periodic cache cleanup |
| Model Refresh | `internal/llm/discovery/` | Background model list refresh |
| Debate Log Tracker | `internal/services/debate_service.go` | Debate round logging |
| ACP Shutdown | `internal/handlers/acp_handler.go` | Graceful connection draining |

### Rules

1. `wg.Add(1)` MUST be called before `go func()`, never inside it
2. `defer wg.Done()` MUST be the first line inside the goroutine
3. The goroutine MUST listen on `ctx.Done()` in a `select` statement
4. `Shutdown()` MUST call `cancel()` before `wg.Wait()` to avoid deadlock

## Deduplication Pattern

The deduplication pattern prevents multiple goroutines from performing the same background operation simultaneously. This is used for model refresh and cache warming.

```go
type Deduplicator struct {
    mu      sync.Mutex
    running bool
    wg      sync.WaitGroup
}

func (d *Deduplicator) TryStart() bool {
    d.mu.Lock()
    defer d.mu.Unlock()
    if d.running {
        return false // Already in progress
    }
    d.running = true
    d.wg.Add(1)
    return true
}

func (d *Deduplicator) Finish() {
    d.mu.Lock()
    d.running = false
    d.mu.Unlock()
    d.wg.Done()
}

func (d *Deduplicator) Wait() {
    d.wg.Wait()
}
```

Usage: call `TryStart()` before launching a refresh goroutine. If it returns `false`, another refresh is already in flight -- skip silently. The caller goroutine calls `defer d.Finish()` when done.

## SpecAdapter No-Op Pattern

When an extracted module (e.g., HelixSpecifier) may not be compiled in (build tag `nohelixspecifier`), the adapter must handle nil receivers safely:

```go
func (a *SpecAdapter) ProcessRequest(ctx context.Context, req *Request) (*Response, error) {
    if a == nil || a.engine == nil {
        return nil, nil // No-op when module not available
    }
    return a.engine.Process(ctx, req)
}
```

All adapter methods must check for nil before delegating to the underlying module.

## Related Resources

- [User Manual 18: Performance Monitoring](18-performance-monitoring.md) -- Monitoring goroutine and pool metrics
- [User Manual 20: Testing Strategies](20-testing-strategies.md) -- Race detection and stress testing
- [Video Course 61: Goroutine Safety](../video-courses/video-course-61-goroutine-safety.md) -- Deep dive on lifecycle patterns
- Goroutine lifecycle diagram: `docs/diagrams/src/goroutine-lifecycle.puml`
- Concurrency module source: `Concurrency/`
- Debate performance optimizer: `internal/services/debate_performance_optimizer.go`
- Background task system: `internal/background/`
- Go concurrency documentation: https://go.dev/doc/effective_go#concurrency
