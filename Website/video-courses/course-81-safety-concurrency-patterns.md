# Video Course 81: Safety & Concurrency Patterns

## Course Overview

**Duration:** 2.5 hours
**Level:** Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 69 (Concurrency Safety), Course 75 (Performance Tuning)

A rigorous treatment of Go concurrency safety in HelixAgent. This course covers the
complete set of concurrency fixes applied to the codebase: race condition prevention,
WaitGroup goroutine lifecycle tracking, `sync.Once` initialization, atomic counters,
and safe channel patterns. Every section shows the broken pattern alongside the fixed
pattern with an explanation of why the fix is correct.

---

## Learning Objectives

By the end of this course, you will be able to:

1. Identify race conditions in Go code using `go test -race` and structured review
2. Apply `sync.Mutex` and `sync.RWMutex` correctly to shared data structures
3. Implement the WaitGroup goroutine lifecycle pattern for graceful shutdown
4. Use `sync.Once` for safe lazy initialization under concurrent access
5. Write atomic counter operations without locks using `sync/atomic`
6. Design safe channel patterns that avoid deadlocks and goroutine leaks

---

## Module 1: Race Condition Prevention (30 min)

### Video 1.1: Detecting Race Conditions (15 min)

**Topics:**
- What is a race condition? Concurrent reads and writes without synchronisation
- The Go race detector: `go test -race ./...` — how it instruments memory accesses
- Interpreting race detector output: goroutine stacks, data address, access types
- Common HelixAgent race patterns found and fixed:
  - Shared provider registry map accessed from multiple goroutines
  - Cached model list written by discovery goroutine, read by handlers
  - Circuit breaker state counters incremented without atomics
- The `race_condition_challenge.sh` validation script

**Race Detector Output (before fix):**
```
==================
WARNING: DATA RACE
Write at 0x00c0001a4080 by goroutine 12:
  internal/llm/discovery.(*ModelCache).Refresh()
      internal/llm/discovery/cache.go:47

Previous read at 0x00c0001a4080 by goroutine 8:
  internal/handlers.(*DiscoveryHandler).ListModels()
      internal/handlers/discovery.go:83
==================
```

### Video 1.2: Fixing Races with Mutex (15 min)

**Topics:**
- `sync.Mutex` for exclusive write access: `Lock()` / `Unlock()` with `defer`
- `sync.RWMutex` for read-heavy data: `RLock()` / `RUnlock()` for reads, `Lock()` for writes
- Granularity: per-field mutex vs. per-struct mutex trade-offs
- Avoiding deadlock: never hold a lock while calling external code that may re-acquire it
- Before/after pattern for the ModelCache race fix

**Before (racy):**
```go
type ModelCache struct {
    models map[string][]Model
}

func (c *ModelCache) Refresh(provider string, models []Model) {
    c.models[provider] = models // write without lock
}

func (c *ModelCache) Get(provider string) []Model {
    return c.models[provider] // read without lock
}
```

**After (safe):**
```go
type ModelCache struct {
    mu     sync.RWMutex
    models map[string][]Model
}

func (c *ModelCache) Refresh(provider string, models []Model) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.models[provider] = models
}

func (c *ModelCache) Get(provider string) []Model {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.models[provider]
}
```

---

## Module 2: WaitGroup Goroutine Lifecycle (30 min)

### Video 2.1: The Goroutine Lifecycle Pattern (15 min)

**Topics:**
- Why goroutine leaks happen: goroutines that never terminate after the parent exits
- The WaitGroup pattern: `Add(1)` before launch, `defer Done()` inside goroutine, `Wait()` on shutdown
- Applying the pattern to HTTP handlers with background SSE goroutines
- Applying the pattern to model discovery refresh loops
- Applying the pattern to debate log tracking goroutines
- Key file: `docs/diagrams/src/goroutine-lifecycle.puml`

**WaitGroup Pattern:**
```go
type SSEHandler struct {
    wg     sync.WaitGroup
    cancel context.CancelFunc
}

func (h *SSEHandler) StreamEvents(w http.ResponseWriter, r *http.Request) {
    h.wg.Add(1)
    go func() {
        defer h.wg.Done()
        // stream SSE events until context is cancelled
        for {
            select {
            case <-r.Context().Done():
                return
            case event := <-h.eventCh:
                fmt.Fprintf(w, "data: %s\n\n", event)
            }
        }
    }()
}

func (h *SSEHandler) Shutdown() {
    h.cancel()      // signal all goroutines to stop
    h.wg.Wait()     // wait for all goroutines to finish
}
```

### Video 2.2: Applied WaitGroup Examples (15 min)

**Topics:**
- Cache invalidation goroutines: ensuring they complete before Redis disconnect
- ACP shutdown: waiting for in-flight message deliveries to complete
- Model refresh: waiting for ongoing discovery to finish before cache reset
- Goroutine lifecycle challenge: `challenges/scripts/goroutine_lifecycle_challenge.sh`
- Testing for leaks with `goleak`: `defer goleak.VerifyNone(t)`

**Leak Detection with goleak:**
```go
func TestSSEHandler_NoLeaks(t *testing.T) {
    defer goleak.VerifyNone(t)

    handler := NewSSEHandler()
    // simulate request lifecycle
    ctx, cancel := context.WithCancel(context.Background())
    go handler.StreamEvents(ctx)
    cancel()
    handler.Shutdown() // must drain all goroutines before test exits
}
```

---

## Module 3: sync.Once Initialization (30 min)

### Video 3.1: Safe Lazy Initialization (15 min)

**Topics:**
- The double-checked locking anti-pattern in Go: why it does not work without `sync.Once`
- `sync.Once.Do`: guarantees the function runs exactly once, even under concurrent calls
- Error propagation with `sync.Once`: the "once with error" pattern
- Where HelixAgent uses `sync.Once`: provider registry, formatter registry, MCP adapters
- Key file pattern: `initOnce sync.Once`, `initErr error` fields on structs

**sync.Once with Error:**
```go
type FormatterRegistry struct {
    initOnce sync.Once
    initErr  error
    registry map[string]Formatter
}

func (r *FormatterRegistry) Init() error {
    r.initOnce.Do(func() {
        r.registry, r.initErr = loadFormatters()
    })
    return r.initErr
}

func (r *FormatterRegistry) Get(lang string) (Formatter, error) {
    if err := r.Init(); err != nil {
        return nil, fmt.Errorf("formatter registry init: %w", err)
    }
    f, ok := r.registry[lang]
    if !ok {
        return nil, fmt.Errorf("no formatter for %s", lang)
    }
    return f, nil
}
```

### Video 3.2: Resettable Once and Testing (15 min)

**Topics:**
- `sync.Once` cannot be reset — design implications for tests and hot-reload
- Resettable initialisation: store a pointer to `sync.Once` and replace it on reset
- Testing lazy initialisation: verifying `Init()` is called exactly once under 100 goroutines
- The lazy loading challenge: `challenges/scripts/lazy_loading_validation_challenge.sh`

**Resettable Once Pattern:**
```go
type ResettableRegistry struct {
    mu   sync.Mutex
    once *sync.Once
    data map[string]string
}

func (r *ResettableRegistry) Reset() {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.once = &sync.Once{} // replace the Once, allowing re-initialisation
    r.data = nil
}
```

---

## Module 4: Atomic Operations (20 min)

### Video 4.1: sync/atomic for Counters and Flags (10 min)

**Topics:**
- When to prefer `sync/atomic` over `sync.Mutex`: simple counters, boolean flags
- `atomic.AddInt64`, `atomic.LoadInt64`, `atomic.StoreInt64`
- `atomic.CompareAndSwapInt64`: lock-free state transitions
- Circuit breaker failure counter: atomic increment without lock contention
- Request counter in rate limiter: atomic load/add for hot path

**Atomic Counter:**
```go
type CircuitBreaker struct {
    failures int64 // accessed atomically
    state    int32 // 0=closed, 1=open, 2=half-open
}

func (cb *CircuitBreaker) RecordFailure() {
    count := atomic.AddInt64(&cb.failures, 1)
    if count >= int64(cb.threshold) {
        atomic.StoreInt32(&cb.state, 1) // open
    }
}

func (cb *CircuitBreaker) IsOpen() bool {
    return atomic.LoadInt32(&cb.state) == 1
}
```

### Video 4.2: Atomic Pointer Swaps (10 min)

**Topics:**
- `atomic.Pointer[T]` (Go 1.19+): lock-free pointer swaps for hot-path config updates
- Use case: swapping the active provider list without locking the serving path
- Consistency caveat: atomic pointer swap is not a transaction; only swap complete structs
- When to avoid atomics: complex data structures with multiple related fields (use mutex)

---

## Module 5: Safe Channel Patterns (20 min)

### Video 5.1: Avoiding Deadlocks and Panics (10 min)

**Topics:**
- Sending on a closed channel: panic — always close from the sender side
- The "done channel" pattern: signal goroutine termination without data
- Buffered vs. unbuffered channels: when to use each
- Select with default: non-blocking sends to avoid goroutine blocking
- Fan-out: one input channel, multiple worker goroutines reading from it

**Safe Send Pattern:**
```go
func safeSend(ch chan<- Event, event Event) (sent bool) {
    defer func() {
        if recover() != nil {
            sent = false // channel was closed
        }
    }()
    ch <- event
    return true
}
```

### Video 5.2: Channel-Based Worker Pools (10 min)

**Topics:**
- Bounded worker pool: `make(chan struct{}, N)` as a semaphore
- Draining channels on shutdown: range over channel until closed
- Context propagation through channels: select on `ctx.Done()` and data channel
- Concurrency safety challenge: `challenges/scripts/concurrency_safety_comprehensive_challenge.sh`

**Semaphore Worker Pool:**
```go
sem := make(chan struct{}, maxWorkers)

for _, task := range tasks {
    sem <- struct{}{} // acquire
    go func(t Task) {
        defer func() { <-sem }() // release
        process(ctx, t)
    }(task)
}

// drain: wait for all workers to finish
for i := 0; i < cap(sem); i++ {
    sem <- struct{}{}
}
```

---

## Key Takeaways

- Always run `go test -race ./...` before committing; the race detector catches data
  races that are invisible in normal test runs.
- The WaitGroup goroutine lifecycle pattern is mandatory for every background goroutine
  in HelixAgent — it prevents leaks and ensures clean shutdown.
- `sync.Once` is the correct tool for lazy initialization; never implement double-checked
  locking manually in Go.
- Prefer `sync/atomic` for simple counters and flags on hot paths; use `sync.Mutex` for
  anything more complex.
- Always close channels from the sender side; use context cancellation to signal
  goroutine termination rather than closing shared channels.

---

## Related Courses

- **Course 69: Concurrency Safety** — Overview of the concurrency patterns module
- **Course 75: Performance Tuning** — Benchmarking and profiling concurrent code
- **Course 82: Performance Tuning and Baselines** — Measuring the impact of concurrency fixes
- **Course 84: Monitoring, Dashboards and Alerting** — Observing goroutine counts in production
