# HelixAgent Production Readiness Master Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring HelixAgent to production readiness: zero safety defects, 100% test coverage, full security scans, performance baselines, and production-complete documentation.

**Architecture:** 5-phase layered convergence — safety fixes first, then test coverage, then performance/security, then documentation, then final validation. Each phase has gate checks that must pass before proceeding.

**Tech Stack:** Go 1.25.3, Gin, PostgreSQL/pgx, Redis, testify, Prometheus/Grafana, OpenTelemetry, Docker/Podman, Snyk, SonarQube, gosec, quic-go, andybalholm/brotli

**Spec:** `docs/superpowers/specs/2026-04-05-production-readiness-master-spec.md`

**Resource Limits (MANDATORY):** All test/build commands: `GOMAXPROCS=2 nice -n 19 ionice -c 3`, go test with `-p 1`

---

## PHASE 1 — FOUNDATION HARDENING

---

### Task 1: Fix InstancePool Race Condition in Acquire() (S1)

**Files:**
- Modify: `internal/clis/pool.go:95-154`
- Test: `internal/clis/pool_test.go`

- [ ] **Step 1: Write failing race-detection test**

```go
// In internal/clis/pool_test.go
func TestInstancePool_Acquire_ConcurrentRace(t *testing.T) {
	pool := newTestPool(t, 10, 5) // maxActive=10, maxIdle=5
	defer pool.Close()

	var wg sync.WaitGroup
	errors := make([]error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			inst, err := pool.Acquire(ctx)
			if err != nil {
				errors[idx] = err
				return
			}
			time.Sleep(10 * time.Millisecond) // simulate work
			pool.Release(inst)
		}(i)
	}
	wg.Wait()

	// Count non-timeout errors — race conditions cause unexpected errors
	unexpectedErrors := 0
	for _, err := range errors {
		if err != nil && !strings.Contains(err.Error(), "timeout") {
			unexpectedErrors++
			t.Logf("Unexpected error: %v", err)
		}
	}
	assert.Equal(t, 0, unexpectedErrors, "should have no unexpected errors from race conditions")
}
```

- [ ] **Step 2: Run with race detector**

Run: `GOMAXPROCS=2 nice -n 19 go test -race -run TestInstancePool_Acquire_ConcurrentRace ./internal/clis/ -p 1 -v`
Expected: DATA RACE detected (or unexpected errors)

- [ ] **Step 3: Fix the RLock-to-Lock gap**

In `internal/clis/pool.go`, replace the Acquire method's locking pattern. The current code does RLock to check idle, then releases and takes full Lock — creating a window. Fix by using a single Lock:

```go
func (p *InstancePool) Acquire(ctx context.Context) (*AgentInstance, error) {
	// First try non-blocking from idle channel (no lock needed — channel is thread-safe)
	select {
	case inst := <-p.idleCh:
		p.mu.Lock()
		p.active[inst.ID] = inst
		p.mu.Unlock()
		return inst, nil
	default:
	}

	// Single lock for check-and-modify
	p.mu.Lock()
	// Try idle slice under lock
	if len(p.idle) > 0 {
		inst := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]
		p.active[inst.ID] = inst
		p.mu.Unlock()
		return inst, nil
	}

	// Check if we can create a new instance
	if len(p.active) < p.maxActive {
		p.mu.Unlock()
		// Create outside lock to avoid holding lock during I/O
		inst, err := p.factory()
		if err != nil {
			return nil, fmt.Errorf("factory error: %w", err)
		}
		p.mu.Lock()
		p.active[inst.ID] = inst
		p.mu.Unlock()
		return inst, nil
	}
	p.mu.Unlock()

	// Pool exhausted — wait with timeout
	select {
	case inst := <-p.idleCh:
		p.mu.Lock()
		p.active[inst.ID] = inst
		p.mu.Unlock()
		return inst, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("pool exhausted, context cancelled: %w", ctx.Err())
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("pool exhausted, timeout waiting for instance")
	}
}
```

- [ ] **Step 4: Run race test again**

Run: `GOMAXPROCS=2 nice -n 19 go test -race -run TestInstancePool_Acquire_ConcurrentRace ./internal/clis/ -p 1 -v`
Expected: PASS, no data races

- [ ] **Step 5: Run full pool test suite**

Run: `GOMAXPROCS=2 nice -n 19 go test -race ./internal/clis/ -p 1 -v`
Expected: All existing tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/clis/pool.go internal/clis/pool_test.go
git commit -m "fix(pool): eliminate race condition in Acquire() by using single Lock for check-and-modify

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Fix InstancePool Goroutine Leaks in Maintenance (M1, M2)

**Files:**
- Modify: `internal/clis/pool.go:280-400`
- Test: `internal/clis/pool_test.go`

- [ ] **Step 1: Write goroutine leak detection test**

```go
func TestInstancePool_CleanupExpired_NoGoroutineLeak(t *testing.T) {
	pool := newTestPool(t, 10, 5)

	// Get baseline goroutine count
	baseline := runtime.NumGoroutine()

	// Force cleanup cycle
	for i := 0; i < 20; i++ {
		inst, err := pool.Acquire(context.Background())
		require.NoError(t, err)
		pool.Release(inst)
	}

	// Close pool and wait
	err := pool.Close()
	require.NoError(t, err)

	// Allow goroutines to drain
	time.Sleep(500 * time.Millisecond)

	// Goroutine count should return to near baseline
	current := runtime.NumGoroutine()
	assert.LessOrEqual(t, current, baseline+5,
		"goroutine leak: baseline=%d, current=%d", baseline, current)
}
```

- [ ] **Step 2: Run to check for leaks**

Run: `GOMAXPROCS=2 nice -n 19 go test -run TestInstancePool_CleanupExpired_NoGoroutineLeak ./internal/clis/ -p 1 -v`
Expected: May FAIL if goroutines leak

- [ ] **Step 3: Fix goroutine tracking in cleanupExpired, ensureMinIdle, prewarm**

In `internal/clis/pool.go`, add WaitGroup tracking to all spawned goroutines:

```go
func (p *InstancePool) cleanupExpired() {
	p.mu.Lock()
	var toTerminate []*AgentInstance
	now := time.Now()
	remaining := make([]*AgentInstance, 0, len(p.idle))
	for _, inst := range p.idle {
		if now.Sub(inst.UpdatedAt) > p.maxLifetime {
			toTerminate = append(toTerminate, inst)
			atomic.AddInt64(&p.evicts, 1)
		} else {
			remaining = append(remaining, inst)
		}
	}
	p.idle = remaining
	p.mu.Unlock()

	for _, inst := range toTerminate {
		p.wg.Add(1) // Track the goroutine
		go func(i *AgentInstance) {
			defer p.wg.Done() // Ensure WaitGroup decremented
			p.terminateInstance(i)
		}(inst)
	}
}

func (p *InstancePool) ensureMinIdle() {
	p.mu.Lock()
	currentIdle := len(p.idle)
	needed := p.minIdle - currentIdle
	if needed <= 0 {
		p.mu.Unlock()
		return
	}
	if currentIdle+needed > p.maxIdle {
		needed = p.maxIdle - currentIdle
	}
	p.mu.Unlock() // Release lock BEFORE calling factory (fixes D1 deadlock too)

	for i := 0; i < needed; i++ {
		inst, err := p.factory()
		if err != nil {
			continue
		}
		select {
		case p.idleCh <- inst:
			p.mu.Lock()
			p.idle = append(p.idle, inst)
			p.mu.Unlock()
		default:
			p.wg.Add(1)
			go func(i *AgentInstance) {
				defer p.wg.Done()
				p.terminateInstance(i)
			}(inst)
		}
	}
}
```

- [ ] **Step 4: Run leak test**

Run: `GOMAXPROCS=2 nice -n 19 go test -run TestInstancePool_CleanupExpired_NoGoroutineLeak ./internal/clis/ -p 1 -v`
Expected: PASS

- [ ] **Step 5: Run full suite with race detector**

Run: `GOMAXPROCS=2 nice -n 19 go test -race ./internal/clis/ -p 1 -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/clis/pool.go internal/clis/pool_test.go
git commit -m "fix(pool): track all spawned goroutines with WaitGroup to prevent leaks

Fixes goroutine leaks in cleanupExpired(), ensureMinIdle(), and prewarm().
Also fixes D1 deadlock by releasing lock before factory() call.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Fix WorkerPool SubmitAsync Race and Shutdown Leak (S2, M3, M4)

**Files:**
- Modify: `internal/ensemble/background/worker_pool.go:172-214, 334-364, 579-588`
- Test: `internal/ensemble/background/worker_pool_test.go`

- [ ] **Step 1: Write failing tests for submit race and shutdown leak**

```go
func TestWorkerPool_SubmitAsync_NoResultLoss(t *testing.T) {
	pool := newTestWorkerPool(t, 4)
	err := pool.Start(context.Background())
	require.NoError(t, err)
	defer pool.Stop()

	// Submit multiple tasks concurrently
	var wg sync.WaitGroup
	results := make([]*TaskResult, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			task := &clis.Task{ID: fmt.Sprintf("task-%d", idx), Input: "test"}
			resultCh := pool.SubmitAsync(task)
			select {
			case r := <-resultCh:
				results[idx] = r
			case <-time.After(10 * time.Second):
				t.Errorf("task-%d timed out", idx)
			}
		}(i)
	}
	wg.Wait()

	// All tasks should have results
	for i, r := range results {
		assert.NotNil(t, r, "task-%d should have result", i)
	}
}

func TestWorkerPool_Stop_NoGoroutineLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()

	pool := newTestWorkerPool(t, 4)
	err := pool.Start(context.Background())
	require.NoError(t, err)

	// Submit some work
	for i := 0; i < 10; i++ {
		pool.Submit(context.Background(), &clis.Task{ID: fmt.Sprintf("t-%d", i), Input: "test"})
	}

	err = pool.Stop()
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)
	current := runtime.NumGoroutine()
	assert.LessOrEqual(t, current, baseline+5,
		"goroutine leak after Stop: baseline=%d, current=%d", baseline, current)
}
```

- [ ] **Step 2: Run tests**

Run: `GOMAXPROCS=2 nice -n 19 go test -race -run "TestWorkerPool_SubmitAsync_NoResultLoss|TestWorkerPool_Stop_NoGoroutineLeak" ./internal/ensemble/background/ -p 1 -v`
Expected: May show race conditions or timeouts

- [ ] **Step 3: Fix SubmitAsync — replace spin loop with direct channel**

```go
func (wp *WorkerPool) SubmitAsync(task *clis.Task) <-chan *TaskResult {
	resultCh := make(chan *TaskResult, 1)

	go func() {
		defer close(resultCh)
		result, err := wp.Submit(wp.ctx, task)
		if err != nil {
			resultCh <- &TaskResult{
				TaskID: task.ID,
				Error:  err,
			}
			return
		}
		if result != nil {
			resultCh <- result
		}
	}()

	return resultCh
}
```

- [ ] **Step 4: Fix Stop — drain channels before closing, use context cancellation**

```go
func (wp *WorkerPool) Stop() error {
	wp.mu.Lock()
	if !wp.running {
		wp.mu.Unlock()
		return nil
	}
	wp.running = false
	wp.mu.Unlock()

	// Signal cancellation first — all goroutines check ctx.Done()
	wp.cancel()

	// Wait for all goroutines to finish (they'll exit via ctx.Done())
	wp.wg.Wait()

	// Now safe to close channels — no writers remain
	close(wp.taskQueue)
	close(wp.resultQueue)

	return nil
}
```

- [ ] **Step 5: Run tests**

Run: `GOMAXPROCS=2 nice -n 19 go test -race -run "TestWorkerPool_SubmitAsync|TestWorkerPool_Stop" ./internal/ensemble/background/ -p 1 -v`
Expected: All PASS

- [ ] **Step 6: Run full suite**

Run: `GOMAXPROCS=2 nice -n 19 go test -race ./internal/ensemble/background/ -p 1 -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ensemble/background/worker_pool.go internal/ensemble/background/worker_pool_test.go
git commit -m "fix(worker-pool): eliminate race in SubmitAsync and goroutine leak in Stop

Replace spin-loop result polling with direct channel per task.
Use context cancellation before closing channels to prevent write-to-closed panic.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Fix HTTPClientPool Double-Check Locking Race (S3)

**Files:**
- Modify: `internal/http/pool.go:152-177`
- Test: `internal/http/pool_test.go`

- [ ] **Step 1: Write concurrent GetClient test**

```go
func TestHTTPClientPool_GetClient_ConcurrentRace(t *testing.T) {
	pool := NewHTTPClientPool(nil)
	defer pool.Close()

	var wg sync.WaitGroup
	clients := make([]*http.Client, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// All goroutines request the same host — maximum contention
			clients[idx] = pool.GetClient("test-host.example.com")
		}(i)
	}
	wg.Wait()

	// All should get the same client instance
	for i := 1; i < 100; i++ {
		assert.Same(t, clients[0], clients[i],
			"client[%d] should be same instance as client[0]", i)
	}
}
```

- [ ] **Step 2: Run with race detector**

Run: `GOMAXPROCS=2 nice -n 19 go test -race -run TestHTTPClientPool_GetClient_ConcurrentRace ./internal/http/ -p 1 -v`
Expected: May detect data race

- [ ] **Step 3: Fix by holding write lock for full get-or-create**

```go
func (p *HTTPClientPool) GetClient(host string) *http.Client {
	// Fast path: read lock
	p.mu.RLock()
	client, exists := p.clients[host]
	p.mu.RUnlock()
	if exists {
		return client
	}

	// Slow path: write lock with double-check
	p.mu.Lock()
	defer p.mu.Unlock()

	// Re-check under write lock (another goroutine may have created it)
	if client, exists = p.clients[host]; exists {
		return client
	}

	// Create under write lock — no gap
	client = &http.Client{
		Transport: p.transport.Clone(),
		Timeout:   p.config.ResponseHeaderTimeout + p.config.DialTimeout,
	}
	p.clients[host] = client
	return client
}
```

- [ ] **Step 4: Run tests**

Run: `GOMAXPROCS=2 nice -n 19 go test -race ./internal/http/ -p 1 -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/http/pool.go internal/http/pool_test.go
git commit -m "fix(http-pool): fix double-check locking race in GetClient

Use Transport.Clone() for per-host clients and hold write lock for
full get-or-create to eliminate race window.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Fix EventBus Channel Close Race (M5)

**Files:**
- Modify: `internal/clis/event_bus.go:200-260`
- Test: `internal/clis/event_bus_test.go`

- [ ] **Step 1: Write close-while-sending test**

```go
func TestEventBus_Close_NoPanicDuringDispatch(t *testing.T) {
	eb := NewEventBus(context.Background())

	// Subscribe with a slow consumer
	sub := eb.Subscribe(EventTypeAll, "", nil)

	// Publish rapidly in background
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			eb.Publish(&Event{Type: EventTypeStatus, Data: fmt.Sprintf("event-%d", i)})
		}
	}()

	// Close while publishing — should not panic
	time.Sleep(10 * time.Millisecond)
	assert.NotPanics(t, func() {
		eb.Close()
	})

	wg.Wait()
	_ = sub // prevent unused warning
}
```

- [ ] **Step 2: Run test**

Run: `GOMAXPROCS=2 nice -n 19 go test -race -run TestEventBus_Close_NoPanicDuringDispatch ./internal/clis/ -p 1 -v -count=10`
Expected: May panic on closed channel

- [ ] **Step 3: Fix with atomic closed flag and sync.Once**

```go
// Add to EventBus struct:
//   closed    atomic.Bool
//   closeOnce sync.Once

func (eb *EventBus) sendToSub(sub *Subscription, event *Event) {
	if eb.closed.Load() {
		return // Don't send to closed bus
	}
	if sub.Filter != nil && !sub.Filter(event) {
		return
	}
	select {
	case sub.Ch <- event:
	default:
		// Channel full, skip (non-blocking)
	}
	if sub.Once {
		eb.Unsubscribe(sub)
	}
}

func (eb *EventBus) Close() error {
	var closeErr error
	eb.closeOnce.Do(func() {
		eb.closed.Store(true)
		eb.cancel()
		eb.wg.Wait() // Wait for dispatchLoop to exit

		eb.mu.Lock()
		defer eb.mu.Unlock()
		// Now safe to close subscriber channels — dispatcher is stopped
		for _, subs := range eb.subscribers {
			for _, sub := range subs {
				close(sub.Ch)
			}
		}
		for _, subs := range eb.topics {
			for _, sub := range subs {
				close(sub.Ch)
			}
		}
		for _, sub := range eb.wildcards {
			close(sub.Ch)
		}
	})
	return closeErr
}
```

- [ ] **Step 4: Run test**

Run: `GOMAXPROCS=2 nice -n 19 go test -race -run TestEventBus_Close_NoPanicDuringDispatch ./internal/clis/ -p 1 -v -count=10`
Expected: PASS, no panics

- [ ] **Step 5: Run full suite**

Run: `GOMAXPROCS=2 nice -n 19 go test -race ./internal/clis/ -p 1 -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/clis/event_bus.go internal/clis/event_bus_test.go
git commit -m "fix(event-bus): prevent panic on channel close race with atomic flag and sync.Once

Add closed atomic.Bool checked before every send.
Use sync.Once to ensure Close() is idempotent.
Wait for dispatchLoop to exit before closing subscriber channels.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Fix Stream Channel Drain on Context Cancel (M6)

**Files:**
- Modify: `internal/handlers/openai_compatible.go:2361-2423`
- Test: `internal/handlers/openai_compatible_test.go`

- [ ] **Step 1: Write cancellation drain test**

```go
func TestUnifiedHandler_ProcessStream_ContextCancel(t *testing.T) {
	handler := newTestUnifiedHandler(t)
	ctx, cancel := context.WithCancel(context.Background())

	req := &models.LLMRequest{Prompt: "test"}
	openaiReq := &OpenAIChatRequest{Messages: []OpenAIChatMessage{{Role: "user", Content: "test"}}}

	streamCh, err := handler.processWithComprehensiveStream(ctx, req, openaiReq)
	require.NoError(t, err)
	require.NotNil(t, streamCh)

	// Cancel immediately
	cancel()

	// Channel should eventually close without blocking forever
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	drained := false
	for {
		select {
		case _, ok := <-streamCh:
			if !ok {
				drained = true
			}
		case <-timer.C:
			t.Fatal("stream channel not drained after context cancel")
		}
		if drained {
			break
		}
	}
	assert.True(t, drained)
}
```

- [ ] **Step 2: Run test**

Run: `GOMAXPROCS=2 nice -n 19 go test -race -run TestUnifiedHandler_ProcessStream_ContextCancel ./internal/handlers/ -p 1 -v`
Expected: May timeout if channel doesn't drain

- [ ] **Step 3: Fix stream goroutine to check context**

In `processWithComprehensiveStream`, ensure the goroutine closes the channel on context cancellation:

```go
func (h *UnifiedHandler) processWithComprehensiveStream(
	ctx context.Context,
	req *models.LLMRequest,
	openaiReq *OpenAIChatRequest,
) (<-chan *models.LLMResponse, error) {
	streamChan := make(chan *models.LLMResponse, 100)

	go func() {
		defer close(streamChan) // Always close when goroutine exits

		// ... existing debate/stream setup ...

		select {
		case <-ctx.Done():
			// Context cancelled — drain and exit
			return
		default:
		}

		// ... existing stream processing with ctx.Done() checks in select ...
	}()

	return streamChan, nil
}
```

- [ ] **Step 4: Run test**

Run: `GOMAXPROCS=2 nice -n 19 go test -race -run TestUnifiedHandler_ProcessStream_ContextCancel ./internal/handlers/ -p 1 -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/openai_compatible.go internal/handlers/openai_compatible_test.go
git commit -m "fix(handlers): drain stream channel on context cancellation

Ensure processWithComprehensiveStream goroutine exits cleanly when
context is cancelled, preventing channel leak.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Fix Deadlock in ensureMinIdle Factory Call (D1)

**Note:** This was already addressed in Task 2 (the ensureMinIdle fix releases the lock before calling factory). Verify it works.

- [ ] **Step 1: Write deadlock detection test**

```go
func TestInstancePool_EnsureMinIdle_NoDeadlock(t *testing.T) {
	// Factory that takes time — if lock is held during factory, this deadlocks
	slowFactory := func() (*AgentInstance, error) {
		time.Sleep(100 * time.Millisecond)
		return &AgentInstance{ID: uuid.New().String()}, nil
	}

	pool := newTestPoolWithFactory(t, 10, 5, slowFactory)
	defer pool.Close()

	done := make(chan bool, 1)
	go func() {
		// This should complete without deadlock
		ctx := context.Background()
		inst, err := pool.Acquire(ctx)
		if err == nil {
			pool.Release(inst)
		}
		done <- true
	}()

	select {
	case <-done:
		// Good — no deadlock
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock detected: Acquire blocked for 10 seconds")
	}
}
```

- [ ] **Step 2: Run test**

Run: `GOMAXPROCS=2 nice -n 19 go test -run TestInstancePool_EnsureMinIdle_NoDeadlock ./internal/clis/ -p 1 -v`
Expected: PASS (Task 2 fix already released lock before factory)

- [ ] **Step 3: Commit**

```bash
git add internal/clis/pool_test.go
git commit -m "test(pool): add deadlock detection test for ensureMinIdle

Verifies that factory() is not called while holding mu.Lock.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Document Coordinator Lock Ordering (D2)

**Files:**
- Modify: `internal/ensemble/multi_instance/coordinator.go:20-40`

- [ ] **Step 1: Add lock ordering documentation**

```go
// Coordinator manages multi-instance ensemble sessions.
//
// Lock ordering (MUST be followed to prevent deadlocks):
//   1. Coordinator.mu (outermost)
//   2. EnsembleSession.mu
//   3. WorkerPool.mu
//   4. EventBus.mu (innermost)
//
// Never acquire a higher-numbered lock while holding a lower-numbered lock.
type Coordinator struct {
	// ... existing fields ...
}
```

- [ ] **Step 2: Add deadlock detector for debug builds**

Create `internal/ensemble/multi_instance/lockorder_debug.go`:

```go
//go:build debug

package multi_instance

import (
	"fmt"
	"runtime"
	"sync"
)

var (
	lockOrderMu sync.Mutex
	lockHolders = make(map[int64][]string) // goroutine -> held locks
)

func trackLock(name string) {
	id := goroutineID()
	lockOrderMu.Lock()
	defer lockOrderMu.Unlock()
	held := lockHolders[id]
	for _, h := range held {
		if lockOrder(h) > lockOrder(name) {
			panic(fmt.Sprintf("lock order violation: holding %s, acquiring %s", h, name))
		}
	}
	lockHolders[id] = append(held, name)
}

func trackUnlock(name string) {
	id := goroutineID()
	lockOrderMu.Lock()
	defer lockOrderMu.Unlock()
	held := lockHolders[id]
	for i, h := range held {
		if h == name {
			lockHolders[id] = append(held[:i], held[i+1:]...)
			return
		}
	}
}

func lockOrder(name string) int {
	switch name {
	case "coordinator":
		return 1
	case "session":
		return 2
	case "workerpool":
		return 3
	case "eventbus":
		return 4
	default:
		return 99
	}
}

func goroutineID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	var id int64
	fmt.Sscanf(string(buf[:n]), "goroutine %d ", &id)
	return id
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/ensemble/multi_instance/coordinator.go internal/ensemble/multi_instance/lockorder_debug.go
git commit -m "docs(coordinator): document lock ordering and add debug deadlock detector

Establishes 4-level lock ordering hierarchy.
Debug build tag enables runtime lock order violation detection.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: Fix Broken Benchmark Signature (B1)

**Files:**
- Modify: `tests/providers/provider_test.go:372-382`

- [ ] **Step 1: Fix the benchmark function signature**

```go
// BenchmarkProvider benchmarks providers using sub-benchmarks
func BenchmarkProvider(b *testing.B) {
	providers := getProviderTestConfigs()
	for _, provider := range providers {
		if os.Getenv(provider.APIKeyEnv) == "" {
			continue
		}
		for _, model := range provider.Models {
			b.Run(fmt.Sprintf("%s/%s", provider.Name, model.Name), func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					benchmarkProviderModel(b, provider, model)
				}
			})
		}
	}
}

func benchmarkProviderModel(b *testing.B, provider ProviderTestConfig, model ModelTestConfig) {
	// ... existing benchmark implementation ...
}
```

- [ ] **Step 2: Verify compilation**

Run: `GOMAXPROCS=2 nice -n 19 go test -run=^$ -bench=BenchmarkProvider -benchtime=1x ./tests/providers/ -p 1 -v`
Expected: Compiles and runs (may skip if no API keys set)

- [ ] **Step 3: Commit**

```bash
git add tests/providers/provider_test.go
git commit -m "fix(benchmark): correct BenchmarkProvider signature to use sub-benchmarks

Go benchmark functions must have signature func(b *testing.B).
Extracted provider/model iteration into sub-benchmarks.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: Dead Code Triage — Wire EnsembleHandler and CompletionHandler

**Files:**
- Modify: `internal/router/router.go`
- Modify: `internal/handlers/ensemble_handler.go` (verify route methods exist)
- Modify: `internal/handlers/completion.go` (verify route methods exist)

- [ ] **Step 1: Read ensemble_handler.go to find route methods**

Run: `grep -n "func (h \*EnsembleHandler)" internal/handlers/ensemble_handler.go | head -20`
Document all available handler methods.

- [ ] **Step 2: Read completion.go to find route methods**

Run: `grep -n "func (h \*CompletionHandler)" internal/handlers/completion.go | head -20`
Document all available handler methods.

- [ ] **Step 3: Register EnsembleHandler in router**

In `internal/router/router.go`, add after existing handler registrations (follow the RAG handler pattern):

```go
// Ensemble orchestration endpoints
ensembleHandler := handlers.NewEnsembleHandler(/* dependencies from context */)
ensembleGroup := protected.Group("/ensemble")
{
	// Register all available route methods found in Step 1
}
```

- [ ] **Step 4: Register CompletionHandler in router**

```go
// Completion endpoints (alternative to OpenAI-compatible)
completionHandler := handlers.NewCompletionHandler(requestService, nil, intentRouter)
completionGroup := protected.Group("/completion")
{
	// Register all available route methods found in Step 2
}
```

- [ ] **Step 5: Verify compilation**

Run: `GOMAXPROCS=2 nice -n 19 go build ./...`
Expected: Clean compilation

- [ ] **Step 6: Run existing tests**

Run: `GOMAXPROCS=2 nice -n 19 go test ./internal/router/ -p 1 -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/router/router.go
git commit -m "feat(router): wire EnsembleHandler and CompletionHandler into router

Register previously unconnected handlers at /v1/ensemble/ and /v1/completion/.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: Dead Code Triage — Delete Redundant Handlers

**Files:**
- Modify: `internal/handlers/` (remove unused constructors)

- [ ] **Step 1: Verify NewDebateHandlerWithSkills is truly unused**

Run: `grep -rn "NewDebateHandlerWithSkills\|DebateHandlerWithSkills" --include="*.go" internal/ cmd/ tests/`
Expected: Only definition in handlers, no callers

- [ ] **Step 2: Verify NewProtocolSSEHandler is truly unused**

Run: `grep -rn "NewProtocolSSEHandler[^W]" --include="*.go" internal/ cmd/ tests/`
Expected: Only definition, no callers (WithACP version is used)

- [ ] **Step 3: Remove unused constructors**

Delete `NewDebateHandlerWithSkills` function and its type if standalone.
Delete `NewProtocolSSEHandler` function (keep `NewProtocolSSEHandlerWithACP`).

- [ ] **Step 4: Verify compilation**

Run: `GOMAXPROCS=2 nice -n 19 go build ./...`
Expected: Clean

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/
git commit -m "refactor(handlers): remove unused NewDebateHandlerWithSkills and NewProtocolSSEHandler

These constructors were never called. DebateHandler and ProtocolSSEHandlerWithACP
are the active versions.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: Dead Code Triage — Env Vars Cleanup

**Files:**
- Modify: `.env.example`
- Modify: `internal/config/config.go` (implement useful vars)

- [ ] **Step 1: Check if HelixMemory module reads HELIX_MEMORY_* vars**

Run: `grep -rn "HELIX_MEMORY_" --include="*.go" HelixMemory/`
Document which vars are read internally vs not.

- [ ] **Step 2: For vars read by HelixMemory — wire config bridge**

If HelixMemory reads these env vars internally, add pass-through documentation in `.env.example`. If not, remove them.

- [ ] **Step 3: Implement useful env vars in config.go**

For vars that SHOULD work but don't:
```go
// In config.go, add to Config loading:
if v := os.Getenv("ALLOWED_ORIGINS"); v != "" {
	cfg.AllowedOrigins = strings.Split(v, ",")
}
if v := os.Getenv("DB_SSL_MODE"); v != "" {
	cfg.Database.SSLMode = v
}
if v := os.Getenv("DEBUG_ENABLED"); v == "true" {
	cfg.DebugEnabled = true
}
if v := os.Getenv("RATE_LIMIT_RPM"); v != "" {
	rpm, _ := strconv.Atoi(v)
	cfg.RateLimitRPM = rpm
}
if v := os.Getenv("REDIS_DB"); v != "" {
	db, _ := strconv.Atoi(v)
	cfg.Redis.DB = db
}
```

- [ ] **Step 4: Remove vars with no backing implementation**

Remove from `.env.example` any `HELIX_MEMORY_*` vars not read anywhere, and `FEATURE_*` vars unless implementing feature flags.

- [ ] **Step 5: Verify and commit**

```bash
go build ./...
git add .env.example internal/config/config.go
git commit -m "chore(config): implement useful env vars, remove dead ones from .env.example

Wire ALLOWED_ORIGINS, DB_SSL_MODE, DEBUG_ENABLED, RATE_LIMIT_RPM, REDIS_DB.
Remove HELIX_MEMORY_* and FEATURE_* vars with no backing implementation.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 13: Phase 1 Gate Check

- [ ] **Step 1: Full build**

Run: `GOMAXPROCS=2 nice -n 19 go build ./...`
Expected: Clean

- [ ] **Step 2: Vet**

Run: `GOMAXPROCS=2 nice -n 19 go vet ./...`
Expected: Clean

- [ ] **Step 3: Unit tests**

Run: `GOMAXPROCS=2 nice -n 19 go test -short ./internal/... -p 1`
Expected: All PASS

- [ ] **Step 4: Race detection on fixed packages**

Run: `GOMAXPROCS=2 nice -n 19 go test -race ./internal/clis/... ./internal/ensemble/... ./internal/http/... ./internal/handlers/ -p 1`
Expected: All PASS, no races

- [ ] **Step 5: Commit gate check results**

```bash
git commit --allow-empty -m "chore: Phase 1 gate check passed — all safety fixes verified

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## PHASE 2 — COVERAGE FORTRESS

---

### Task 14: Add Tests for internal/output Package (T1)

**Files:**
- Create: `internal/output/pipeline_test.go`

- [ ] **Step 1: Write unit tests for Pipeline**

```go
package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPipeline(t *testing.T) {
	p := NewPipeline()
	require.NotNil(t, p)
	assert.NotEmpty(t, p.parsers, "should have default parsers")
	assert.NotEmpty(t, p.formatters, "should have default formatters")
	assert.NotEmpty(t, p.renderers, "should have default renderers")
}

func TestPipeline_Process_JSONInput(t *testing.T) {
	p := NewPipeline()
	result, err := p.Process(`{"key": "value"}`, "json", "raw", "terminal")
	require.NoError(t, err)
	assert.Contains(t, result, "key")
}

func TestPipeline_Process_MarkdownInput(t *testing.T) {
	p := NewPipeline()
	result, err := p.Process("# Hello\n\nWorld", "markdown", "raw", "terminal")
	require.NoError(t, err)
	assert.Contains(t, result, "Hello")
}

func TestPipeline_Process_UnknownParser(t *testing.T) {
	p := NewPipeline()
	_, err := p.Process("test", "nonexistent", "raw", "terminal")
	assert.Error(t, err, "should error on unknown parser")
}

func TestPipeline_Process_UnknownFormatter(t *testing.T) {
	p := NewPipeline()
	_, err := p.Process("test", "text", "nonexistent", "terminal")
	assert.Error(t, err, "should error on unknown formatter")
}

func TestPipeline_Process_UnknownRenderer(t *testing.T) {
	p := NewPipeline()
	_, err := p.Process("test", "text", "raw", "nonexistent")
	assert.Error(t, err, "should error on unknown renderer")
}

func TestPipeline_RegisterParser(t *testing.T) {
	p := NewPipeline()
	p.RegisterParser("custom", &testParser{})
	result, err := p.Process("test", "custom", "raw", "terminal")
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestPipeline_ProcessStream(t *testing.T) {
	p := NewPipeline()
	ch := make(chan string, 3)
	ch <- "chunk1"
	ch <- "chunk2"
	close(ch)

	results, err := p.ProcessStream(ch, "text", "raw", "terminal")
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

type testParser struct{}

func (tp *testParser) Parse(input string) (interface{}, error) {
	return input, nil
}
```

- [ ] **Step 2: Run tests**

Run: `GOMAXPROCS=2 nice -n 19 go test ./internal/output/ -p 1 -v`
Expected: All PASS

- [ ] **Step 3: Commit**

```bash
git add internal/output/pipeline_test.go
git commit -m "test(output): add unit tests for Pipeline package

Covers NewPipeline, Process (JSON/markdown/text), error paths,
custom parser registration, and stream processing.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 15: Add Tests for internal/containers Package (T2)

**Files:**
- Create: `internal/containers/lazy_integration_test.go`
- Create: `internal/containers/logger_adapter_test.go`

- [ ] **Step 1: Write tests for LazyOrchestrator**

```go
package containers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLazyOrchestrator(t *testing.T) {
	orch := NewLazyOrchestrator(nil, nil, nil, "/tmp/test-workdir")
	require.NotNil(t, orch)
	assert.NotNil(t, orch.services)
	assert.NotNil(t, orch.started)
	assert.NotNil(t, orch.failed)
}

func TestLazyOrchestrator_RegisterService(t *testing.T) {
	orch := NewLazyOrchestrator(nil, nil, nil, "/tmp/test-workdir")

	svc := &ServiceDefinition{
		Name:        "test-service",
		ComposeFile: "docker-compose.yml",
		Required:    false,
		Description: "Test service",
	}

	err := orch.RegisterService(svc)
	require.NoError(t, err)

	services := orch.ListServices()
	assert.Contains(t, services, "test-service")
}

func TestLazyOrchestrator_RegisterService_Duplicate(t *testing.T) {
	orch := NewLazyOrchestrator(nil, nil, nil, "/tmp/test-workdir")

	svc := &ServiceDefinition{Name: "dup-service"}
	err := orch.RegisterService(svc)
	require.NoError(t, err)

	err = orch.RegisterService(svc)
	assert.Error(t, err, "duplicate registration should error")
}

func TestLazyOrchestrator_GetServiceStatus_Unknown(t *testing.T) {
	orch := NewLazyOrchestrator(nil, nil, nil, "/tmp/test-workdir")
	status := orch.GetServiceStatus("nonexistent")
	assert.Equal(t, "unknown", status.State)
}

func TestLazyOrchestrator_ListServices_Empty(t *testing.T) {
	orch := NewLazyOrchestrator(nil, nil, nil, "/tmp/test-workdir")
	services := orch.ListServices()
	assert.Empty(t, services)
}
```

- [ ] **Step 2: Write tests for logger adapter**

Read `internal/containers/logger_adapter.go` and write corresponding tests based on its interface.

- [ ] **Step 3: Run tests**

Run: `GOMAXPROCS=2 nice -n 19 go test ./internal/containers/ -p 1 -v`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/containers/lazy_integration_test.go internal/containers/logger_adapter_test.go
git commit -m "test(containers): add unit tests for LazyOrchestrator and logger adapter

Covers constructor, service registration, duplicate detection,
status queries, and listing.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 16: Add Tests for Missing Handlers (H1-H3, H5)

**Files:**
- Create: `internal/handlers/browser_handler_test.go`
- Create: `internal/handlers/ensemble_handler_test.go`
- Create: `internal/handlers/search_handler_test.go`
- Create: `internal/handlers/verifier_types_test.go`

- [ ] **Step 1: Read each handler file to understand the API**

Run: `grep -n "func (h \*BrowserHandler)" internal/handlers/browser_handler.go | head -10`
Run: `grep -n "func (h \*EnsembleHandler)" internal/handlers/ensemble_handler.go | head -10`
Run: `grep -n "func (h \*SearchHandler)" internal/handlers/search_handler.go | head -10`

- [ ] **Step 2: Write handler tests following existing test patterns**

Each test file should follow the project's table-driven test pattern:

```go
func TestBrowserHandler_HandleRequest(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
	}{
		{"valid request", "POST", "/v1/browser/navigate", `{"url":"https://example.com"}`, http.StatusOK},
		{"missing url", "POST", "/v1/browser/navigate", `{}`, http.StatusBadRequest},
		{"invalid json", "POST", "/v1/browser/navigate", `{invalid`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			handler := NewBrowserHandler(nil) // nil manager for unit test
			handler.HandleNavigate(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
```

Repeat similar pattern for EnsembleHandler and SearchHandler.

- [ ] **Step 3: Write verifier_types serialization tests**

```go
func TestVerifierTypes_JSONRoundTrip(t *testing.T) {
	// Read the types from verifier_types.go and test JSON marshal/unmarshal
	original := VerificationResult{
		// ... fill with sample data ...
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded VerificationResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}
```

- [ ] **Step 4: Run tests**

Run: `GOMAXPROCS=2 nice -n 19 go test ./internal/handlers/ -p 1 -v -run "TestBrowserHandler|TestEnsembleHandler|TestSearchHandler|TestVerifierTypes"`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/*_test.go
git commit -m "test(handlers): add tests for browser, ensemble, search handlers and verifier types

Covers request validation, error handling, JSON serialization round-trips.
All 5 previously untested handlers now have test coverage.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 17: Implement Fuzz Test Functions (F1-F10)

**Files:**
- Modify: existing files in `tests/fuzz/`

- [ ] **Step 1: List existing fuzz test files**

Run: `ls tests/fuzz/`

- [ ] **Step 2: Add real Fuzz functions to each file**

For each existing fuzz test file, add a proper `Fuzz*` function. Example for API input parsing:

```go
func FuzzAPIInputParsing(f *testing.F) {
	// Seed corpus
	f.Add(`{"prompt": "hello"}`)
	f.Add(`{"messages": [{"role": "user", "content": "test"}]}`)
	f.Add(`{}`)
	f.Add(``)
	f.Add(`{"prompt": "` + strings.Repeat("a", 10000) + `"}`)

	f.Fuzz(func(t *testing.T, input string) {
		var req OpenAIChatRequest
		err := json.Unmarshal([]byte(input), &req)
		if err != nil {
			return // Invalid JSON is expected — just don't panic
		}
		// Validate doesn't panic
		_ = req.Validate()
	})
}
```

Repeat for each of the 10 targets (F1-F10) specified in the spec.

- [ ] **Step 3: Verify all fuzz functions compile**

Run: `GOMAXPROCS=2 nice -n 19 go test -tags=fuzz -run=^$ -fuzz=. -fuzztime=1x ./tests/fuzz/ -p 1`
Expected: Compiles and runs (1 iteration each)

- [ ] **Step 4: Run fuzz for 30 seconds each**

Run: `GOMAXPROCS=2 nice -n 19 go test -tags=fuzz -fuzz=FuzzAPIInputParsing -fuzztime=30s ./tests/fuzz/ -p 1`
Expected: No crashes

- [ ] **Step 5: Commit**

```bash
git add tests/fuzz/
git commit -m "test(fuzz): implement 10 real Fuzz functions for critical input paths

Covers API input, provider responses, MCP messages, prompts, config,
tool schemas, debate messages, embeddings, memory queries, auth tokens.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 18: Add Phase 2 Stress Tests (correctness-focused)

**Files:**
- Create: `tests/stress/pool_stress_test.go`
- Create: `tests/stress/worker_pool_stress_test.go`
- Create: `tests/stress/event_bus_stress_test.go`
- Create: `tests/stress/ensemble_stress_test.go`
- Create: `tests/stress/http_client_pool_stress_test.go`

- [ ] **Step 1: Write pool stress test**

```go
//go:build stress

package stress

import (
	"context"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/clis"
	"github.com/stretchr/testify/assert"
)

func TestPool_ConcurrentAcquireRelease_Stress(t *testing.T) {
	pool := newStressTestPool(t, 20, 10) // maxActive=20, maxIdle=10
	defer pool.Close()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			inst, err := pool.Acquire(ctx)
			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
				return
			}
			time.Sleep(5 * time.Millisecond)
			pool.Release(inst)
		}()
	}
	wg.Wait()

	timeoutCount := 0
	for _, err := range errors {
		if err != nil {
			timeoutCount++
		}
	}
	// Some timeouts are expected under extreme load, but no panics/races
	t.Logf("Completed: %d/%d (timeouts: %d)", 200-timeoutCount, 200, timeoutCount)
	assert.Less(t, timeoutCount, 50, "too many timeouts suggests a bottleneck")
}
```

- [ ] **Step 2: Write similar tests for worker_pool, event_bus, ensemble, http_pool**

Follow the same pattern for each: concurrent operations, verify no panics/races, measure timeout rate.

- [ ] **Step 3: Run all stress tests with race detector**

Run: `GOMAXPROCS=2 nice -n 19 go test -tags=stress -race ./tests/stress/ -p 1 -v -timeout=300s`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add tests/stress/
git commit -m "test(stress): add correctness-focused stress tests for fixed safety code

Validates pool, worker_pool, event_bus, ensemble, http_pool under
concurrent load. Proves Phase 1 safety fixes hold under stress.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 19: Create Phase 2 Challenge Scripts (C1-C10)

**Files:**
- Create: `challenges/scripts/output_pipeline_challenge.sh`
- Create: `challenges/scripts/container_lazy_loading_challenge.sh`
- Create: `challenges/scripts/ensemble_handler_challenge.sh`
- Create: `challenges/scripts/browser_handler_challenge.sh`
- Create: `challenges/scripts/search_handler_challenge.sh`
- Create: `challenges/scripts/fuzz_test_validation_challenge.sh`
- Create: `challenges/scripts/feature_flag_challenge.sh`
- Create: `challenges/scripts/env_var_completeness_challenge.sh`
- Create: `challenges/scripts/safety_regression_challenge.sh`
- Create: `challenges/scripts/test_type_completeness_challenge.sh`

- [ ] **Step 1: Create challenge scripts following existing pattern**

Read one existing challenge for the pattern:

Run: `head -40 challenges/scripts/coverage_gate_challenge.sh`

Then create each challenge following the same pattern (PASS/FAIL counters, section headers, test assertions). Example for safety_regression_challenge.sh:

```bash
#!/bin/bash
# Safety Regression Challenge
# Validates that all Phase 1 safety fixes remain effective
set -euo pipefail

PASS=0
FAIL=0
TOTAL=0

check() {
    TOTAL=$((TOTAL + 1))
    if [ "$1" -eq 0 ]; then
        PASS=$((PASS + 1))
        echo "  PASS: $2"
    else
        FAIL=$((FAIL + 1))
        echo "  FAIL: $2"
    fi
}

echo "=== Safety Regression Challenge ==="
echo ""

echo "--- Section 1: Race Detection ---"
GOMAXPROCS=2 nice -n 19 go test -race ./internal/clis/ -p 1 -short -count=1 2>&1 | tail -1
check $? "Pool package race-free"

GOMAXPROCS=2 nice -n 19 go test -race ./internal/ensemble/background/ -p 1 -short -count=1 2>&1 | tail -1
check $? "WorkerPool race-free"

# ... more checks ...

echo ""
echo "=== Results: $PASS/$TOTAL passed, $FAIL failed ==="
exit $FAIL
```

- [ ] **Step 2: Make all scripts executable**

Run: `chmod +x challenges/scripts/output_pipeline_challenge.sh challenges/scripts/container_lazy_loading_challenge.sh challenges/scripts/ensemble_handler_challenge.sh challenges/scripts/browser_handler_challenge.sh challenges/scripts/search_handler_challenge.sh challenges/scripts/fuzz_test_validation_challenge.sh challenges/scripts/feature_flag_challenge.sh challenges/scripts/env_var_completeness_challenge.sh challenges/scripts/safety_regression_challenge.sh challenges/scripts/test_type_completeness_challenge.sh`

- [ ] **Step 3: Run each challenge**

Run each and verify they pass.

- [ ] **Step 4: Commit**

```bash
git add challenges/scripts/
git commit -m "test(challenges): add 10 Phase 2 challenge scripts

Covers output pipeline, container lazy loading, ensemble/browser/search handlers,
fuzz validation, feature flags, env vars, safety regression, test completeness.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 20: Phase 2 Gate Check

- [ ] **Step 1: Full test suite**

Run: `GOMAXPROCS=2 nice -n 19 go test ./... -p 1 -short`
Expected: All PASS

- [ ] **Step 2: Race detection**

Run: `GOMAXPROCS=2 nice -n 19 go test -race ./... -p 1 -short`
Expected: Clean

- [ ] **Step 3: Coverage check**

Run: `GOMAXPROCS=2 nice -n 19 go test -coverprofile=coverage.out ./internal/... -p 1 -short && go tool cover -func=coverage.out | tail -1`
Expected: >= 95%

- [ ] **Step 4: Commit gate check**

```bash
git commit --allow-empty -m "chore: Phase 2 gate check passed — coverage fortress built

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## PHASE 3 — PERFORMANCE & SECURITY

---

### Task 21: Create Performance Baseline Framework

**Files:**
- Create: `benchmarks/baselines/` directory
- Create: `tests/performance/baseline_regression_test.go`
- Modify: `Makefile` (add benchmark-baseline and benchmark-check targets)

- [ ] **Step 1: Create baseline capture script**

Create `scripts/benchmark-baseline.sh`:

```bash
#!/bin/bash
# Capture performance baselines
set -euo pipefail

BASEDIR="benchmarks/baselines"
mkdir -p "$BASEDIR"

echo "Capturing handler baselines..."
GOMAXPROCS=2 nice -n 19 go test -bench=. -benchmem -count=5 \
  ./internal/handlers/ -p 1 -run=^$ \
  | tee "$BASEDIR/handlers.txt"

echo "Capturing pool baselines..."
GOMAXPROCS=2 nice -n 19 go test -bench=. -benchmem -count=5 \
  ./internal/clis/ -p 1 -run=^$ \
  | tee "$BASEDIR/pool.txt"

echo "Capturing ensemble baselines..."
GOMAXPROCS=2 nice -n 19 go test -bench=. -benchmem -count=5 \
  ./internal/ensemble/... -p 1 -run=^$ \
  | tee "$BASEDIR/ensemble.txt"

echo "Baselines captured at $BASEDIR/"
```

- [ ] **Step 2: Create regression test**

```go
//go:build performance

package performance

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseline_HandlersExist(t *testing.T) {
	_, err := os.Stat("../../benchmarks/baselines/handlers.txt")
	assert.NoError(t, err, "handler baselines must exist — run: make benchmark-baseline")
}

// Regression tests compare current benchmark output against baselines
// using benchstat or custom parsing
```

- [ ] **Step 3: Add Makefile targets**

```makefile
benchmark-baseline:
	@bash scripts/benchmark-baseline.sh

benchmark-check:
	@echo "Comparing against baselines..."
	@GOMAXPROCS=2 nice -n 19 go test -bench=. -benchmem -count=5 \
		./internal/handlers/ -p 1 -run=^$ > /tmp/bench-current.txt
	@benchstat benchmarks/baselines/handlers.txt /tmp/bench-current.txt
```

- [ ] **Step 4: Capture initial baselines**

Run: `make benchmark-baseline`

- [ ] **Step 5: Commit**

```bash
git add benchmarks/ scripts/benchmark-baseline.sh tests/performance/baseline_regression_test.go Makefile
git commit -m "feat(perf): add performance baseline framework with regression detection

Captures handler/pool/ensemble benchmarks. Makefile targets for
baseline capture and regression checking via benchstat.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 22: Create Phase 3 Stress Tests (ST1-ST10)

**Files:**
- Create: `tests/stress/pool_saturation_stress_test.go`
- Create: `tests/stress/worker_pool_overload_stress_test.go`
- Create: `tests/stress/event_bus_flood_stress_test.go`
- Create: `tests/stress/http_pool_exhaustion_stress_test.go`
- Create: `tests/stress/ensemble_all_timeout_stress_test.go`
- Create: `tests/stress/memory_growth_stress_test.go`
- Create: `tests/stress/goroutine_leak_stress_test.go`
- Create: `tests/stress/concurrent_debate_stress_test.go`
- Create: `tests/stress/streaming_backpressure_stress_test.go`
- Create: `tests/stress/circuit_breaker_cascade_stress_test.go`

- [ ] **Step 1: Write quantitative stress tests**

Each test has a specific numeric target. Example:

```go
//go:build stress

func TestPoolSaturation_1000Concurrent(t *testing.T) {
	pool := newStressTestPool(t, 50, 25)
	defer pool.Close()

	var wg sync.WaitGroup
	var successCount, timeoutCount int64

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			inst, err := pool.Acquire(ctx)
			if err != nil {
				atomic.AddInt64(&timeoutCount, 1)
				return
			}
			atomic.AddInt64(&successCount, 1)
			time.Sleep(10 * time.Millisecond)
			pool.Release(inst)
		}()
	}
	wg.Wait()

	t.Logf("Success: %d, Timeout: %d", successCount, timeoutCount)
	assert.Greater(t, successCount, int64(900), "at least 90% should succeed")
}

func TestMemoryGrowth_10KRequests(t *testing.T) {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	// Simulate 10K requests
	for i := 0; i < 10000; i++ {
		// ... create and process request ...
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	growth := float64(after.HeapAlloc-before.HeapAlloc) / float64(before.HeapAlloc) * 100
	t.Logf("Heap growth: %.1f%%", growth)
	assert.Less(t, growth, 15.0, "heap should not grow more than 15%%")
}
```

- [ ] **Step 2: Run all stress tests**

Run: `GOMAXPROCS=2 nice -n 19 go test -tags=stress ./tests/stress/ -p 1 -v -timeout=600s`
Expected: All PASS

- [ ] **Step 3: Commit**

```bash
git add tests/stress/
git commit -m "test(stress): add 10 quantitative resilience stress tests

Pool saturation (1000 concurrent), worker overload (10x capacity),
event bus flood (10K/s), memory growth (10K req), goroutine leaks,
circuit breaker cascade, streaming backpressure.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 23: Run Security Scans and Fix Findings

**Files:**
- Modify: Various (based on scan findings)
- Modify: `.snyk` and `.gosec.yml` (justified exclusions)

- [ ] **Step 1: Run gosec**

Run: `GOMAXPROCS=2 nice -n 19 make security-scan-gosec 2>&1 | tail -20`
Document all findings.

- [ ] **Step 2: Run go vet and staticcheck**

Run: `GOMAXPROCS=2 nice -n 19 make security-scan-go 2>&1 | tail -20`
Document all findings.

- [ ] **Step 3: Fix all critical/high findings**

Apply fixes for each finding. Add justified exclusions to `.gosec.yml` for false positives.

- [ ] **Step 4: Verify clean scan**

Run: `GOMAXPROCS=2 nice -n 19 make security-scan-gosec`
Expected: Clean (only excluded rules)

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "security: fix all critical/high gosec and staticcheck findings

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 24: Create Grafana Dashboards

**Files:**
- Create: `docker/monitoring/grafana/dashboards/api-overview.json`
- Create: `docker/monitoring/grafana/dashboards/provider-health.json`
- Create: `docker/monitoring/grafana/dashboards/resource-utilization.json`
- Create: `docker/monitoring/grafana/dashboards/cache-performance.json`
- Create: `docker/monitoring/grafana/dashboards/ensemble-performance.json`
- Create: `docker/monitoring/grafana/dashboards/mcp-adapters.json`
- Create: `docker/monitoring/grafana/dashboards/security-status.json`
- Create: `docker/monitoring/grafana/datasources/prometheus.yml`

- [ ] **Step 1: Create Prometheus datasource config**

```yaml
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: false
```

- [ ] **Step 2: Create API overview dashboard**

Create a Grafana dashboard JSON with panels for: request rate (rate(http_requests_total[5m])), error rate, p50/p95/p99 latency, active connections. Use the Grafana dashboard JSON format.

- [ ] **Step 3: Create remaining 6 dashboards**

Each dashboard follows the same JSON structure with appropriate Prometheus queries matching the metrics exposed by HelixAgent.

- [ ] **Step 4: Commit**

```bash
git add docker/monitoring/grafana/
git commit -m "feat(monitoring): add 7 Grafana dashboards and Prometheus datasource

API overview, provider health, ensemble performance, resource utilization,
cache performance, MCP adapters, security status.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 25: Implement Lazy Loading Improvements (L1-L7)

**Files:**
- Modify: `internal/handlers/openai_compatible.go` (L1)
- Modify: `internal/mcp/adapters/registry.go` (L3)
- Modify: `internal/formatters/registry.go` (L4)
- Modify: `internal/clis/pool.go` (L5)
- Modify: `cmd/helixagent/main.go` (L6)

- [ ] **Step 1: Add sync.Once lazy init for debate orchestrator (L1)**

```go
// In UnifiedHandler struct, add:
//   debateOnce sync.Once

func (h *UnifiedHandler) getDebateService() *services.DebateService {
	h.debateOnce.Do(func() {
		// Initialize debate service on first use
		if h.debateService == nil {
			h.debateService = services.NewDebateService(/* config */)
		}
	})
	return h.debateService
}
```

- [ ] **Step 2: Add configurable pool timeout (L5)**

```go
// In PoolConfig, add:
//   AcquireTimeout time.Duration

// In Acquire(), replace hardcoded 30s:
timeout := p.config.AcquireTimeout
if timeout == 0 {
	timeout = 30 * time.Second
}
```

- [ ] **Step 3: Add GOMAXPROCS to main.go (L6)**

```go
import "runtime"

func main() {
	// Respect container CPU limits
	if v := os.Getenv("GOMAXPROCS"); v == "" {
		// Auto-detect: use container CPU quota if available
		runtime.GOMAXPROCS(0) // 0 = use GOMAXPROCS env or all CPUs
	}
	// ... existing main code ...
}
```

- [ ] **Step 4: Verify compilation and test**

Run: `GOMAXPROCS=2 nice -n 19 go build ./... && go test ./internal/handlers/ ./internal/clis/ -p 1 -short`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/openai_compatible.go internal/clis/pool.go cmd/helixagent/main.go
git commit -m "perf: implement lazy loading improvements and configurable timeouts

Defer debate orchestrator init to first use (sync.Once).
Make pool acquire timeout configurable. Add GOMAXPROCS awareness.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 26: Phase 3 Challenge Scripts and Gate Check

**Files:**
- Create: `challenges/scripts/performance_baseline_challenge.sh`
- Create: `challenges/scripts/stress_resilience_challenge.sh`
- Create: `challenges/scripts/security_scan_results_challenge.sh`
- Create: `challenges/scripts/grafana_dashboard_content_challenge.sh`
- Create: `challenges/scripts/lazy_loading_comprehensive_challenge.sh`
- Create: `challenges/scripts/brotli_compression_challenge.sh`

- [ ] **Step 1: Create 6 challenge scripts**

Following the existing challenge pattern. Each validates its Phase 3 area.

- [ ] **Step 2: Run all Phase 3 challenges**

- [ ] **Step 3: Phase 3 gate check**

Run: Full stress test suite, security scan, benchmark regression, race detection.

- [ ] **Step 4: Commit**

```bash
git add challenges/scripts/
git commit -m "test(challenges): add 6 Phase 3 challenge scripts

Performance baselines, stress resilience, security scan results,
Grafana dashboards, lazy loading, Brotli compression.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## PHASE 4 — DOCUMENTATION & CONTENT

---

### Task 27: Resolve TODO/FIXME Markers in Documentation

**Files:**
- Modify: Multiple files across `docs/`
- Create: `docs/TODO_RESOLUTION_LOG.md`

- [ ] **Step 1: Generate TODO inventory**

Run: `grep -rn "TODO\|FIXME" docs/ --include="*.md" | wc -l`
Run: `grep -rn "TODO\|FIXME" docs/ --include="*.md" > /tmp/todo-inventory.txt`

- [ ] **Step 2: Batch-resolve by category**

Process in batches:
1. Phase status markers — update to actual verified status
2. Test structure TODOs — remove (Phase 2 created the tests)
3. Code examples — fill with real code from the codebase
4. API endpoints — verify against `internal/router/router.go`
5. Feature descriptions — write accurate descriptions
6. Broken links — fix or remove

- [ ] **Step 3: Create resolution log**

```markdown
# TODO Resolution Log

## Summary
- Total TODOs resolved: [count]
- Resolved by category:
  - Phase status: [count]
  - Test structure: [count]
  - Code examples: [count]
  - API endpoints: [count]
  - Feature descriptions: [count]
  - Broken links: [count]
  - Removed duplicates: [count]
```

- [ ] **Step 4: Verify zero remaining**

Run: `grep -rn "TODO\|FIXME" docs/ --include="*.md" | grep -v archive/ | wc -l`
Expected: 0

- [ ] **Step 5: Commit**

```bash
git add docs/
git commit -m "docs: resolve all 3,208 TODO/FIXME markers in documentation

See docs/TODO_RESOLUTION_LOG.md for resolution details by category.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 28: Archive Conflicting Status Reports

**Files:**
- Move: `ACTUALLY_UNFINISHED.md`, `100_PERCENT_COMPLETION_REPORT.md`, `COMPLETION_STATUS.md`, `FINAL_STATUS.md`, `UNFINISHED_WORK.md` to `docs/archive/status-history/`
- Create: `docs/PROJECT_STATUS.md`

- [ ] **Step 1: Create archive directory and move files**

Run: `mkdir -p docs/archive/status-history/`
Move all conflicting status files to archive.

- [ ] **Step 2: Create authoritative status document**

Write `docs/PROJECT_STATUS.md` with current verified state from Phases 1-3.

- [ ] **Step 3: Commit**

```bash
git add docs/
git commit -m "docs: create authoritative PROJECT_STATUS.md, archive conflicting reports

Single source of truth replaces 10+ conflicting completion reports.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 29: Expand 4 Stub User Guides

**Files:**
- Modify: `Website/user-manuals/34-agentic-workflows-guide.md`
- Modify: `Website/user-manuals/35-llmops-experimentation-guide.md`
- Modify: `Website/user-manuals/36-planning-algorithms-guide.md`
- Modify: `Website/user-manuals/37-benchmark-guide.md`

- [ ] **Step 1: Read agentic code for guide content**

Run: `grep -rn "func.*Workflow\|func.*Node\|func.*Execute" internal/agentic/ | head -20`
Use actual code to write the guide.

- [ ] **Step 2: Expand each guide to 15-20KB**

Each guide should include:
- Introduction and concepts
- Architecture overview with diagram
- Step-by-step setup
- Configuration reference (every option)
- 3-5 complete code examples
- Troubleshooting section
- FAQ

- [ ] **Step 3: Verify guide sizes**

Run: `wc -c Website/user-manuals/34-*.md Website/user-manuals/35-*.md Website/user-manuals/36-*.md Website/user-manuals/37-*.md`
Expected: Each >= 15,000 bytes

- [ ] **Step 4: Commit**

```bash
git add Website/user-manuals/
git commit -m "docs: expand 4 stub user guides to full production length (15-20KB each)

Agentic workflows, LLMOps experimentation, planning algorithms,
benchmarking guides — all with code examples and configuration references.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 30: Create New Documentation Files

**Files:**
- Create: `docs/development/SAFETY_FIXES.md`
- Create: `docs/development/DEAD_CODE_AUDIT.md`
- Create: `docs/testing/TEST_STRATEGY.md`
- Create: `docs/testing/FUZZ_TESTING_GUIDE.md`
- Create: `docs/testing/STRESS_TESTING_GUIDE.md`
- Create: `docs/performance/BASELINE_GUIDE.md`
- Create: `docs/security/SCANNING_GUIDE.md`
- Create: `docs/monitoring/DASHBOARD_GUIDE.md`
- Create: `docs/architecture/LAZY_LOADING_PATTERNS.md`
- Create: `docs/configuration/FEATURE_FLAGS.md`
- Create: `docs/diagrams/INDEX.md`
- Create: `docs/development/RELEASE_CHECKLIST.md`

- [ ] **Step 1: Create each documentation file**

Each file should be comprehensive (5-15KB) with:
- Purpose and scope
- Detailed instructions
- Code examples where applicable
- Cross-references to related docs

- [ ] **Step 2: Create new diagrams**

Create `docs/diagrams/src/test-pyramid.mermaid`, `monitoring-flow.mermaid`, `security-scanning.mermaid`.

- [ ] **Step 3: Update existing diagrams**

Update architecture-overview, goroutine-lifecycle, provider-flow with Phase 1-3 changes.

- [ ] **Step 4: Commit**

```bash
git add docs/
git commit -m "docs: add 12 new documentation files and 3 diagrams

Safety fixes, dead code audit, test strategy, fuzz/stress testing,
baselines, scanning, dashboards, lazy loading, feature flags, release checklist.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 31: Update Video Courses and Website

**Files:**
- Create: `Website/video-courses/course-77-*.md` through `course-84-*.md`
- Modify: `Website/video-courses/` (update courses 1-76)
- Modify: `Website/README.md`
- Modify: `Website/user-manuals/` (verify all current)

- [ ] **Step 1: Create 8 new video course scripts**

Each course script includes:
- Title and description
- Prerequisites
- Estimated duration
- Section outline (4-8 sections)
- Talking points per section
- Code demos to show
- Key takeaways

- [ ] **Step 2: Update courses 1-76**

Verify code examples, endpoint URLs, feature descriptions are current.

- [ ] **Step 3: Update Website README.md**

Update feature count, provider count, module count, test counts.

- [ ] **Step 4: Commit**

```bash
git add Website/
git commit -m "docs(website): add 8 new video courses (77-84), update existing 76, refresh website

New courses: agentic workflows, LLMOps, planning, benchmarking,
safety patterns, performance tuning, security scanning, monitoring.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 32: Synchronize Constitution Documents

**Files:**
- Modify: `CLAUDE.md`
- Modify: `AGENTS.md`
- Modify: `CONSTITUTION.json`

- [ ] **Step 1: Compare documents for inconsistencies**

Run diff analysis between CLAUDE.md, AGENTS.md, and CONSTITUTION.json.

- [ ] **Step 2: Update all three with Phase 1-3 additions**

Add: new endpoints (ensemble, completion), new challenge scripts, updated test counts, lazy loading patterns, performance baselines, Grafana dashboards.

- [ ] **Step 3: Verify synchronization**

Run: `diff <(grep -oP '(?<=\*\*).+?(?=\*\*)' CLAUDE.md | sort) <(grep -oP '(?<=\*\*).+?(?=\*\*)' AGENTS.md | sort)`
Expected: Minimal differences (format-only)

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md AGENTS.md CONSTITUTION.json
git commit -m "docs: synchronize CLAUDE.md, AGENTS.md, and CONSTITUTION.json

Updated with Phase 1-3 capabilities, new endpoints, challenge counts,
test type inventory, and performance infrastructure.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 33: Phase 4 Gate Check

- [ ] **Step 1: Zero TODOs in docs**

Run: `grep -rn "TODO\|FIXME" docs/ --include="*.md" | grep -v archive/ | wc -l`
Expected: 0

- [ ] **Step 2: Guide sizes**

Run: `wc -c Website/user-manuals/34-*.md Website/user-manuals/35-*.md Website/user-manuals/36-*.md Website/user-manuals/37-*.md`
Expected: All >= 15,000

- [ ] **Step 3: New docs exist**

Run: `ls docs/development/SAFETY_FIXES.md docs/testing/TEST_STRATEGY.md docs/performance/BASELINE_GUIDE.md docs/diagrams/INDEX.md`
Expected: All exist

- [ ] **Step 4: Commit**

```bash
git commit --allow-empty -m "chore: Phase 4 gate check passed — documentation production-complete

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## PHASE 5 — FINAL VALIDATION

---

### Task 34: Run All Challenges

- [ ] **Step 1: Run master challenge suite**

Run: `GOMAXPROCS=2 nice -n 19 ./challenges/scripts/run_all_challenges.sh 2>&1 | tee /tmp/challenge-results.txt`
Expected: All pass (exit code 0)

- [ ] **Step 2: Run new Phase 2 challenges**

Run each of the 10 new challenges individually and verify pass.

- [ ] **Step 3: Run new Phase 3 challenges**

Run each of the 6 new challenges individually and verify pass.

- [ ] **Step 4: Document results**

Create `challenges/results/final-validation/` with timestamped results.

---

### Task 35: Run Full Test Suite

- [ ] **Step 1: Unit tests**

Run: `GOMAXPROCS=2 nice -n 19 go test -short ./internal/... -p 1`

- [ ] **Step 2: Integration tests**

Run: `GOMAXPROCS=2 nice -n 19 go test -tags=integration ./tests/integration/... -p 1`

- [ ] **Step 3: Security tests**

Run: `GOMAXPROCS=2 nice -n 19 go test -tags=security ./tests/security/... -p 1`

- [ ] **Step 4: Race detection**

Run: `GOMAXPROCS=2 nice -n 19 go test -race ./... -p 1 -short`

- [ ] **Step 5: Coverage report**

Run: `GOMAXPROCS=2 nice -n 19 go test -coverprofile=coverage.out ./internal/... -p 1 -short && go tool cover -func=coverage.out | tail -1`
Expected: >= 95%

- [ ] **Step 6: Benchmark regression**

Run: `make benchmark-check`
Expected: All within 15% of baselines

---

### Task 36: Zero-Broken Verification

- [ ] **Step 1: No unconditional skips**

Run: `grep -rn "t.Skip(" --include="*_test.go" internal/ | grep -v "short\|testing.Short\|integration\|infra\|os.Getenv"`
Expected: 0 unconditional skips

- [ ] **Step 2: No orphaned packages**

Run: Verify every package in `internal/` is imported somewhere.

- [ ] **Step 3: No dead env vars**

Run: Compare `.env.example` against `grep -rn "os.Getenv" internal/ cmd/`
Expected: Every var in .env.example is read

- [ ] **Step 4: Constitution sync verified**

Confirm CLAUDE.md, AGENTS.md, CONSTITUTION.json are consistent.

---

### Task 37: Create Master Validation Challenge

**Files:**
- Create: `challenges/scripts/full_validation_challenge.sh`

- [ ] **Step 1: Create the master challenge**

Combines all zero-broken checks into a single script:

```bash
#!/bin/bash
# Full Validation Challenge — Master Gate Check
# Runs ALL Phase 5 zero-broken checks
set -euo pipefail
# ... implement all 5.3 checks from the spec ...
```

- [ ] **Step 2: Run it**

Run: `GOMAXPROCS=2 nice -n 19 ./challenges/scripts/full_validation_challenge.sh`
Expected: All PASS

- [ ] **Step 3: Final commit**

```bash
git add challenges/scripts/full_validation_challenge.sh
git commit -m "feat: add master validation challenge — Phase 5 final gate

Combines all zero-broken verification checks: no skips, no orphans,
no dead code, no unregistered handlers, full coverage, synced docs.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## TASK SUMMARY

| Phase | Tasks | Key Deliverables |
|-------|-------|-----------------|
| Phase 1 | Tasks 1-13 | 11 safety fixes, 1 build fix, dead code triage, gate check |
| Phase 2 | Tasks 14-20 | Missing tests, fuzz functions, 10 challenges, gate check |
| Phase 3 | Tasks 21-26 | Baselines, stress tests, security scans, dashboards, lazy loading |
| Phase 4 | Tasks 27-33 | 3,208 TODOs resolved, guides expanded, courses updated, constitution synced |
| Phase 5 | Tasks 34-37 | All challenges, all test types, zero-broken verification, master challenge |

**Total: 37 tasks across 5 phases**
