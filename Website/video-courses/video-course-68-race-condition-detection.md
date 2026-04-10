# Video Course 68: Race Condition Detection and Prevention

## Course Overview

**Duration:** 2 hours
**Level:** Advanced
**Prerequisites:** Course 01 (Fundamentals), Course 61 (Goroutine Safety), Course 69 (Concurrency Safety)

Detect, diagnose, and prevent race conditions in Go applications. Learn to use the Go race detector, atomic operations, proper mutex patterns, and the WaitGroup lifecycle pattern used throughout HelixAgent for goroutine safety.

---

## Learning Objectives

By the end of this course, you will be able to:

1. Run the Go race detector and interpret its output
2. Identify common race condition patterns in Go code
3. Convert plain variables to atomic operations where needed
4. Implement the WaitGroup lifecycle pattern for background goroutines
5. Apply context-based cancellation for graceful shutdown
6. Write concurrent stress tests that expose race conditions

---

## Module 1: Understanding Race Conditions (20 min)

### Video 1.1: What Is a Data Race (10 min)

**Topics:**
- Definition: concurrent unsynchronized access to shared memory
- Read-write races vs write-write races
- Why race conditions are intermittent and hard to reproduce
- Real example: HelixAgent worker pool `started` flag (was `bool`, now `atomic int32`)

### Video 1.2: The Go Race Detector (10 min)

**Topics:**
- Enabling: `go test -race ./...`
- How it works: compiler instrumentation at build time
- Performance overhead (2-10x slower)
- Reading race detector output: goroutine stacks, memory addresses
- Resource-limited usage: `GOMAXPROCS=2 go test -race -p 1`

---

## Module 2: Atomic Operations (25 min)

### Video 2.1: When to Use Atomics vs Mutexes (10 min)

**Topics:**
- Simple flags and counters: use `sync/atomic`
- Complex state transitions: use `sync.Mutex`
- `atomic.LoadInt32` / `atomic.StoreInt32` for boolean flags
- `atomic.AddInt64` for counters
- `atomic.CompareAndSwapInt64` for min/max tracking

**Code Example — Before (race-prone):**
```go
type Pool struct {
    started bool  // accessed from multiple goroutines
}
```

**After (race-safe):**
```go
type Pool struct {
    started int32  // use atomic.LoadInt32/StoreInt32
}

func (p *Pool) IsStarted() bool {
    return atomic.LoadInt32(&p.started) == 1
}
```

### Video 2.2: HelixAgent Atomic Patterns (15 min)

**Topics:**
- Worker pool: `started`, `scaling`, `activeCount` fields
- Load test metrics: `totalRequests`, `successCount` with atomic adds
- Worker state: `status int32` with atomic load/store
- `stopChanDone int32` flag to prevent double-close

---

## Module 3: The WaitGroup Lifecycle Pattern (30 min)

### Video 3.1: Pattern Overview (10 min)

**Topics:**
- Five-step pattern used across HelixAgent:
  1. `wg.Add(1)` before goroutine launch
  2. `go func()` starts the goroutine
  3. `defer wg.Done()` inside the goroutine
  4. `cancel()` signals goroutine to stop
  5. `wg.Wait()` in `Stop()` ensures completion

### Video 3.2: Rate Limiter Implementation (10 min)

**Topics:**
- `RateLimiter` struct: `ctx`, `cancel`, `wg` fields
- `NewRateLimiter()`: `context.WithCancel` + `wg.Add(1)` + `go cleanup()`
- `cleanupExpiredBuckets()`: `defer wg.Done()` + `select { case <-ctx.Done(): return }`
- `Stop()`: `cancel()` + `wg.Wait()`

### Video 3.3: Memory Service Implementation (10 min)

**Topics:**
- `MemoryService` struct: `stopCh`, `stopped`, `wg` fields
- Channel-based signaling vs context-based cancellation
- `Stop()` mutex protection for idempotent shutdown
- Unlock before `wg.Wait()` to prevent deadlock

---

## Module 4: Mutex and Lock Ordering (25 min)

### Video 4.1: Nested Lock Deadlock Prevention (15 min)

**Topics:**
- Deadlock scenario: Lock A then Lock B vs Lock B then Lock A
- Consistent lock ordering across all code paths
- HelixAgent example: rate limiter outer `rl.mu` then inner `bucket.mu`
- Never hold a mutex across a blocking call

### Video 4.2: RWMutex Patterns (10 min)

**Topics:**
- `RLock` for concurrent reads, `Lock` for exclusive writes
- SSE handler: `clientsMu sync.RWMutex` for client map
- Nested map synchronization: both levels under same lock
- `sync.Once` for one-time initialization (worker stop)

---

## Module 5: Writing Concurrent Tests (20 min)

### Video 5.1: Table-Driven Concurrent Tests (10 min)

**Topics:**
- Multiple goroutines accessing shared state
- `sync.WaitGroup` for test coordination
- Verifying final state after concurrent operations
- Using `t.Parallel()` safely

**Code Example:**
```go
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
    rl := NewRateLimiter(nil)
    defer rl.Stop()

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            key := fmt.Sprintf("client-%d", n%10)
            rl.checkInMemory(key, rl.defaultCfg)
        }(i)
    }
    wg.Wait()
}
```

### Video 5.2: Race Detector in CI (10 min)

**Topics:**
- `make test-race` target
- Resource limits: `-race` increases memory usage 5-10x
- Selective race testing for critical paths
- Memory safety challenge script validation

---

## Exercises

1. Run `go test -race ./internal/middleware/` and verify zero race warnings
2. Find a plain `bool` field accessed from goroutines and convert to atomic
3. Add a `Stop()` method with WaitGroup to a service missing one
4. Write a concurrent test that would fail without proper synchronization

---

## Summary

Race conditions are among the most dangerous bugs in concurrent software. HelixAgent prevents them through three primary patterns: atomic operations for simple flags/counters, the WaitGroup lifecycle pattern for background goroutines, and consistent mutex ordering for complex state. The Go race detector validates these patterns, and the memory safety challenge script provides automated regression testing.
