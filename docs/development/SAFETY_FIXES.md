# Safety Fixes (Phase 1)

**Date:** 2026-03-30
**Status:** Complete
**Total Fixes:** 11 (3 race conditions, 6 memory leaks, 2 deadlocks)

## Overview

Phase 1 of the stability effort identified and resolved 11 concurrency and memory safety issues across the HelixAgent codebase. Every fix has corresponding test coverage to prevent regression.

---

## Race Conditions (S1-S3)

### S1: CLI Process Pool Race

**File:** `internal/clis/pool.go`
**Problem:** The CLI process pool used a plain `map` to track active processes. Concurrent `Acquire()` and `Release()` calls from multiple goroutines caused data races on the map, leading to intermittent panics under load.
**Fix:** Replaced the plain `map` with a `sync.RWMutex`-guarded map. All map reads use `RLock()` and all writes use `Lock()`. Added `atomic.Int64` for pool-level counters (active count, total acquired, total released).
**Before:**
```go
type Pool struct {
    processes map[string]*Process
}
func (p *Pool) Acquire(id string) { p.processes[id] = proc }
```
**After:**
```go
type Pool struct {
    mu        sync.RWMutex
    processes map[string]*Process
}
func (p *Pool) Acquire(id string) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.processes[id] = proc
}
```
**Test coverage:** `internal/clis/pool_test.go` -- concurrent acquire/release with `-race` flag.

### S2: Ensemble Worker Pool Race

**File:** `internal/ensemble/background/worker_pool.go`
**Problem:** Worker pool metrics (`tasksCompleted`, `tasksFailed`, `activeWorkers`) were incremented with `++` from multiple goroutines without synchronization.
**Fix:** Converted all counters to `atomic.Int64` and replaced `++` with `Add(1)` / `Add(-1)`.
**Before:**
```go
type WorkerPoolMetrics struct {
    TasksCompleted int64
}
w.metrics.TasksCompleted++
```
**After:**
```go
type WorkerPoolMetrics struct {
    TasksCompleted atomic.Int64
}
w.metrics.TasksCompleted.Add(1)
```
**Test coverage:** `internal/ensemble/background/worker_pool_test.go`, `tests/stress/worker_pool_stress_test.go`, `tests/stress/worker_pool_overload_stress_test.go`.

### S3: HTTP Client Pool Race

**File:** `internal/http/pool.go`
**Problem:** Per-host client caching used a plain map. When multiple providers made simultaneous requests to different hosts, concurrent map writes triggered Go runtime panics.
**Fix:** Added `sync.RWMutex` protection around the per-host client map. Pool-level metrics use `atomic.Int64`. Lazy initialization of the global singleton uses `sync.Once`.
**Test coverage:** `internal/http/http_test.go`, `tests/stress/http_client_pool_stress_test.go`, `tests/stress/http_pool_exhaustion_stress_test.go`.

---

## Memory Leaks (M1-M6)

### M1: Goroutine Tracking Leak

**File:** `internal/services/concurrency_alert_manager.go`
**Problem:** Background goroutines spawned for monitoring were tracked but never cleaned up when their parent context was cancelled. Over time, stale entries accumulated in the tracking map.
**Fix:** Added `context.Done()` listener in each goroutine that removes its entry from the tracking map on cancellation. Added `Shutdown()` method that calls `cancel()` + `WaitGroup.Wait()`.
**Test coverage:** `internal/services/concurrency_alert_manager_test.go`.

### M2: Channel Drain on SSE Disconnect

**File:** `internal/handlers/protocol_sse.go`
**Problem:** When an SSE client disconnected mid-stream, the response channel continued to buffer events. The sending goroutine blocked indefinitely on the full channel, leaking the goroutine.
**Fix:** Added a select-based drain loop that empties the channel when the client context is cancelled. The sending goroutine detects cancellation via `ctx.Done()` and exits cleanly.
**Test coverage:** `internal/handlers/protocol_sse_test.go`, `tests/stress/streaming_backpressure_stress_test.go`.

### M3: Event Bus Subscriber Leak

**File:** `internal/clis/event_bus.go`
**Problem:** Event bus subscribers registered for specific topics were never unsubscribed when the owning component shut down. This caused the event bus to hold references to dead subscribers.
**Fix:** Implemented a `Close()` method on subscribers that unregisters them from the event bus. Components call `defer subscriber.Close()` immediately after registration. Added finalizer-based leak detection in tests.
**Test coverage:** `tests/stress/event_bus_stress_test.go`, `tests/stress/event_bus_flood_stress_test.go`.

### M4: Circuit Breaker Timer Leak

**File:** `internal/llm/circuit_breaker.go`
**Problem:** Each state transition in the circuit breaker created a new `time.Timer` for the recovery window. Old timers were not stopped before creating new ones, causing timer goroutine accumulation.
**Fix:** Store the active timer reference and call `timer.Stop()` before creating a replacement. Check the timer channel drain on stop to avoid channel leak.
**Test coverage:** `internal/llm/circuit_breaker_lifecycle_test.go`.

### M5: Discovery Handler Cache Growth

**File:** `internal/handlers/discovery_handler.go`
**Problem:** Model discovery results were cached per-provider with no TTL enforcement on stale entries. Providers that were removed from the registry left orphaned cache entries.
**Fix:** Added a background cache cleanup goroutine (runs every 10 minutes) that removes entries for providers no longer in the registry. Cache entries have a 1-hour TTL.
**Test coverage:** Unit tests in the handler test file; verified by `tests/stress/memory_growth_stress_test.go`.

### M6: Background Task Handler Leak

**File:** `internal/handlers/background_task_handler.go`
**Problem:** Completed background tasks were stored indefinitely in the in-memory task store, causing unbounded memory growth under sustained load.
**Fix:** Added configurable retention policy (default: 1 hour for completed tasks, 24 hours for failed tasks). Background cleanup goroutine runs on a ticker and respects graceful shutdown.
**Test coverage:** `internal/handlers/background_task_handler_test.go`.

---

## Deadlocks (D1-D2)

### D1: Lock Ordering Violation

**File:** `internal/concurrency/deadlock/detector.go`
**Problem:** Two mutexes were acquired in inconsistent order across code paths: path A locked `providerMu` then `metricsMu`, while path B locked `metricsMu` then `providerMu`. Under contention this caused classic ABBA deadlocks.
**Fix:** Established a strict lock ordering convention: always acquire `providerMu` before `metricsMu`. Refactored path B to follow this order. Added a comment documenting the ordering contract.
**Test coverage:** `internal/concurrency/deadlock/detector_test.go`, `tests/stress/deadlock_detection_stress_test.go`.

### D2: Factory Call Inside Lock

**File:** `internal/services/provider_registry.go`
**Problem:** Provider instantiation (which may call out to external APIs for health checks) was performed while holding the registry write lock. If the health check timed out, the lock was held for the full timeout duration, blocking all other registry operations.
**Fix:** Moved provider instantiation outside the lock. The factory call runs unlocked, and only the insertion into the map acquires the write lock. A double-check pattern prevents duplicate registration between unlock and re-lock.
**Test coverage:** `internal/services/provider_registry_test.go`, `tests/stress/provider_registry_stress_test.go`.

---

## Verification

All fixes are validated by:

1. **Unit tests** with `-race` flag (`go test -race ./internal/...`)
2. **Stress tests** in `tests/stress/` (build tag `stress`)
3. **Challenge script:** `./challenges/scripts/race_condition_challenge.sh`
4. **Challenge script:** `./challenges/scripts/concurrency_safety_comprehensive_challenge.sh`
5. **Challenge script:** `./challenges/scripts/goroutine_lifecycle_challenge.sh`

See also: `docs/memory_safety/PHASE4_MEMORY_SAFETY_REPORT.md` for the Phase 4 extended analysis.
