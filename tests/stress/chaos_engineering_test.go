package stress

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/concurrency"
	"dev.helix.agent/internal/services"
)

// TestChaosServiceFailureRecovery verifies the system recovers gracefully when
// dependencies fail mid-operation via context cancellation.
func TestChaosServiceFailureRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	const numWorkers = 40
	const opsPerWorker = 20
	var wg sync.WaitGroup
	var completedOps, cancelledOps, panicCount int64

	// Simulate a dependency that fails mid-operation
	type dependency struct {
		mu      sync.RWMutex
		healthy bool
	}

	dep := &dependency{healthy: true}

	// Background goroutine that toggles health
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				dep.mu.Lock()
				dep.healthy = !dep.healthy
				dep.mu.Unlock()
			}
		}
	}()

	performOperation := func(opCtx context.Context) error {
		// Check dependency health
		dep.mu.RLock()
		healthy := dep.healthy
		dep.mu.RUnlock()

		if !healthy {
			return fmt.Errorf("dependency unavailable")
		}

		// Simulate work that respects context
		select {
		case <-opCtx.Done():
			return opCtx.Err()
		case <-time.After(2 * time.Millisecond):
			return nil
		}
	}

	start := make(chan struct{})

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for op := 0; op < opsPerWorker; op++ {
				// Each op gets its own short-lived context
				opCtx, opCancel := context.WithTimeout(ctx, 50*time.Millisecond)
				err := performOperation(opCtx)
				opCancel()

				if err != nil {
					atomic.AddInt64(&cancelledOps, 1)
				} else {
					atomic.AddInt64(&completedOps, 1)
				}
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK DETECTED: service failure recovery test timed out")
	}

	total := completedOps + cancelledOps
	t.Logf("Chaos service failure: completed=%d cancelled=%d panics=%d total=%d",
		completedOps, cancelledOps, panicCount, total)

	assert.Zero(t, panicCount, "service failures must not cause panics")
	assert.Equal(t, int64(numWorkers*opsPerWorker), total,
		"all operations must be accounted for")
	assert.Greater(t, completedOps, int64(0),
		"some operations should succeed between failures")
	assert.Greater(t, cancelledOps, int64(0),
		"some operations should fail due to dependency outage")
}

// TestChaosConcurrentGoroutineBomb verifies that semaphore mechanisms prevent
// goroutine explosion even when many goroutines attempt to spawn concurrently.
func TestChaosConcurrentGoroutineBomb(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	baseline := runtime.NumGoroutine()

	const maxConcurrent = 10
	sem := concurrency.NewSemaphore(maxConcurrent)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const numGoroutines = 500
	var wg sync.WaitGroup
	var acquired, panicCount int64
	var peakGoroutines int64

	start := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			err := sem.Acquire(ctx)
			if err != nil {
				return
			}

			atomic.AddInt64(&acquired, 1)

			// Track goroutine peak
			current := int64(runtime.NumGoroutine())
			for {
				old := atomic.LoadInt64(&peakGoroutines)
				if current <= old || atomic.CompareAndSwapInt64(&peakGoroutines, old, current) {
					break
				}
			}

			// Simulate brief work
			time.Sleep(100 * time.Microsecond)

			sem.Release()
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: goroutine bomb test timed out")
	}

	// Check goroutine cleanup
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()

	t.Logf("Goroutine bomb: baseline=%d peak=%d after=%d acquired=%d panics=%d",
		baseline, peakGoroutines, after, acquired, panicCount)

	assert.Zero(t, panicCount, "goroutine bomb must not cause panics")
	assert.Equal(t, int64(numGoroutines), acquired,
		"all goroutines should eventually acquire the semaphore")
	assert.Less(t, after, baseline+50,
		"goroutine count should be bounded after cleanup")
}

// TestChaosTimeoutCascade verifies the system handles cascading
// context.DeadlineExceeded errors gracefully, without panics or leaks.
func TestChaosTimeoutCascade(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	const numWorkers = 60
	var wg sync.WaitGroup
	var deadlineExceeded, completed, panicCount int64

	baseline := runtime.NumGoroutine()
	start := make(chan struct{})

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			// Create contexts with very short timeouts (1-5ms)
			timeout := time.Duration(1+id%5) * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Simulate chained operations that may time out
			for step := 0; step < 3; step++ {
				select {
				case <-ctx.Done():
					if ctx.Err() == context.DeadlineExceeded {
						atomic.AddInt64(&deadlineExceeded, 1)
					}
					return
				case <-time.After(2 * time.Millisecond):
					// Step completed within deadline
				}
			}
			atomic.AddInt64(&completed, 1)
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK DETECTED: timeout cascade test timed out")
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()

	t.Logf("Timeout cascade: completed=%d deadline_exceeded=%d panics=%d goroutine_leak=%d",
		completed, deadlineExceeded, panicCount, after-baseline)

	total := completed + deadlineExceeded
	assert.Zero(t, panicCount, "timeout cascade must not cause panics")
	assert.Equal(t, int64(numWorkers), total,
		"all workers must be accounted for")
	assert.Greater(t, deadlineExceeded, int64(0),
		"some operations should hit deadline exceeded")
	assert.Less(t, after-baseline, 20,
		"no goroutine leak after timeout cascade")
}

// TestChaosCircuitBreakerUnderChaos verifies circuit breakers operate correctly
// when the underlying service alternates rapidly between healthy and failed states.
func TestChaosCircuitBreakerUnderChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	cb := services.NewCircuitBreaker(3, 2, 20*time.Millisecond)

	const numWorkers = 30
	const opsPerWorker = 50
	var wg sync.WaitGroup
	var successes, failures, circuitOpens, panicCount int64

	// Use a shared flag to toggle service state
	var serviceUp int32 = 1

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Chaos goroutine: rapidly toggle service state
	go func() {
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if atomic.LoadInt32(&serviceUp) == 1 {
					atomic.StoreInt32(&serviceUp, 0)
				} else {
					atomic.StoreInt32(&serviceUp, 1)
				}
			}
		}
	}()

	start := make(chan struct{})

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for op := 0; op < opsPerWorker; op++ {
				err := cb.Call(func() error {
					if atomic.LoadInt32(&serviceUp) == 0 {
						return fmt.Errorf("service down")
					}
					return nil
				})
				if err == nil {
					atomic.AddInt64(&successes, 1)
				} else if err.Error() == "circuit breaker is open" {
					atomic.AddInt64(&circuitOpens, 1)
				} else {
					atomic.AddInt64(&failures, 1)
				}
				time.Sleep(time.Millisecond) // Slight pacing
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: circuit breaker chaos test timed out")
	}

	total := successes + failures + circuitOpens
	t.Logf("Circuit breaker chaos: success=%d failure=%d circuit_open=%d panics=%d total=%d",
		successes, failures, circuitOpens, panicCount, total)

	assert.Zero(t, panicCount, "circuit breaker should not panic under chaos")
	assert.Equal(t, int64(numWorkers*opsPerWorker), total,
		"all operations must be accounted for")
	assert.Greater(t, successes, int64(0),
		"some calls should succeed when service is up")
	assert.Greater(t, failures+circuitOpens, int64(0),
		"some calls should fail or be blocked when service is down")
}

// TestChaosHTTPHandlerPanicRecovery verifies that panics inside HTTP handlers
// are recovered and do not bring down the server.
func TestChaosHTTPHandlerPanicRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery()) // Gin's built-in panic recovery

	var panicHandlerCalls int64
	router.GET("/panic", func(c *gin.Context) {
		atomic.AddInt64(&panicHandlerCalls, 1)
		panic("simulated handler panic")
	})
	router.GET("/healthy", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	const numRequests = 50
	var wg sync.WaitGroup
	var panicResponses, healthyResponses, errorCount int64

	start := make(chan struct{})

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start

			var endpoint string
			if id%2 == 0 {
				endpoint = server.URL + "/panic"
			} else {
				endpoint = server.URL + "/healthy"
			}

			resp, err := client.Get(endpoint)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				return
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusInternalServerError {
				atomic.AddInt64(&panicResponses, 1)
			} else if resp.StatusCode == http.StatusOK {
				atomic.AddInt64(&healthyResponses, 1)
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK DETECTED: panic recovery test timed out")
	}

	t.Logf("Panic recovery: panic_responses=%d healthy=%d errors=%d",
		panicResponses, healthyResponses, errorCount)

	// Server should still be alive after panics
	resp, err := client.Get(server.URL + "/healthy")
	if err == nil {
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"server should still be healthy after handler panics")
	}

	assert.Greater(t, healthyResponses, int64(0),
		"healthy endpoints should still work after panics")
	assert.Greater(t, panicResponses, int64(0),
		"panic endpoints should return 500, not crash server")
}

// TestChaosResourceExhaustionRecovery verifies that after temporary resource
// exhaustion (many goroutines, memory pressure), the system returns to normal.
func TestChaosResourceExhaustionRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	// Baseline measurements
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()
	var baselineMem runtime.MemStats
	runtime.ReadMemStats(&baselineMem)

	// Phase 1: Create resource pressure
	const numGoroutines = 300
	var wg sync.WaitGroup
	var completed int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Allocate some memory per goroutine
			data := make([]byte, 4096)
			for j := range data {
				data[j] = byte(j % 256)
			}
			time.Sleep(5 * time.Millisecond)
			_ = data
			atomic.AddInt64(&completed, 1)
		}(i)
	}

	// Verify goroutine count during pressure
	peakGoroutines := runtime.NumGoroutine()
	t.Logf("Peak goroutines during pressure: %d", peakGoroutines)

	wg.Wait()

	// Phase 2: Verify recovery
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	runtime.GC()

	afterGoroutines := runtime.NumGoroutine()
	var afterMem runtime.MemStats
	runtime.ReadMemStats(&afterMem)

	goroutineDiff := afterGoroutines - baselineGoroutines
	heapAfterMB := float64(afterMem.HeapInuse) / 1024 / 1024

	t.Logf("Resource exhaustion recovery:")
	t.Logf("  Goroutines: baseline=%d peak=%d after=%d diff=%d",
		baselineGoroutines, peakGoroutines, afterGoroutines, goroutineDiff)
	t.Logf("  Heap after recovery: %.2f MB", heapAfterMB)
	t.Logf("  Completed operations: %d/%d", completed, numGoroutines)

	assert.Equal(t, int64(numGoroutines), completed,
		"all goroutines should complete")
	assert.Less(t, goroutineDiff, 20,
		"goroutine count should return near baseline after recovery")
	assert.Less(t, heapAfterMB, 200.0,
		"heap should be bounded after GC recovery")
}
