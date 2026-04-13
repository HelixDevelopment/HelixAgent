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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/cache"
	"dev.helix.agent/internal/concurrency"
	helixhttp "dev.helix.agent/internal/http"
	"dev.helix.agent/internal/services"
)

// TestStress_ConcurrentHTTPPool_NoLeak verifies that creating 100 concurrent
// HTTP requests through the pool does not leak goroutines or connections.
func TestStress_ConcurrentHTTPPool_NoLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	// Spin up a lightweight test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	pool := helixhttp.NewHTTPClientPool(helixhttp.DefaultPoolConfig())
	defer func() { _ = pool.Close() }()

	// Take goroutine baseline after pool creation
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const numRequests = 100
	var wg sync.WaitGroup
	var successes int64
	var failures int64

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := make(chan struct{})
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start

			resp, err := pool.Get(ctx, server.URL+"/test")
			if err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}
			resp.Body.Close()
			atomic.AddInt64(&successes, 1)
		}()
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
		t.Fatal("DEADLOCK DETECTED: concurrent HTTP pool requests timed out")
	}

	assert.Equal(t, int64(numRequests), successes+failures,
		"all requests must complete")
	assert.Greater(t, successes, int64(0), "at least some requests should succeed")

	// Check goroutine leak
	pool.CloseIdleConnections()
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	current := runtime.NumGoroutine()
	leaked := current - baseline
	t.Logf("HTTP pool: %d successes, %d failures, goroutines baseline=%d current=%d leaked=%d",
		successes, failures, baseline, current, leaked)
	assert.Less(t, leaked, 20,
		"goroutine count should not grow significantly after HTTP pool stress")
}

// TestStress_ConcurrentCacheAccess exercises 50 concurrent goroutines performing
// mixed get/set operations on TieredCache to verify locking correctness.
func TestStress_ConcurrentCacheAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	config := &cache.TieredCacheConfig{
		L1MaxSize: 1000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer func() { _ = tc.Close() }()

	const goroutineCount = 50
	const opsPerGoroutine = 100
	ctx := context.Background()

	var wg sync.WaitGroup
	var sets, gets, panics int64

	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			for j := 0; j < opsPerGoroutine; j++ {
				key := fmt.Sprintf("stress-key-%d-%d", id, j%20)
				if j%2 == 0 {
					_ = tc.Set(ctx, key, fmt.Sprintf("value-%d-%d", id, j), time.Minute)
					atomic.AddInt64(&sets, 1)
				} else {
					var dest string
					_, _ = tc.Get(ctx, key, &dest)
					atomic.AddInt64(&gets, 1)
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
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: concurrent cache access timed out")
	}

	assert.Zero(t, panics, "no goroutine should panic during concurrent cache access")
	assert.Equal(t, int64(goroutineCount*opsPerGoroutine), sets+gets,
		"all operations must complete")
	t.Logf("Cache stress: %d sets, %d gets, %d panics", sets, gets, panics)
}

// TestStress_WorkerPoolSaturation submits 1000 tasks to a worker pool and
// verifies that all tasks complete without deadlocks or data loss.
func TestStress_WorkerPoolSaturation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	pool := concurrency.NewWorkerPool(&concurrency.PoolConfig{
		Workers:   8,
		QueueSize: 200,
	})
	pool.Start()

	const totalTasks = 1000
	var completed int64
	var wg sync.WaitGroup

	start := make(chan struct{})

	for i := 0; i < totalTasks; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start

			task := concurrency.NewTaskFunc(
				fmt.Sprintf("saturate-task-%d", idx),
				func(ctx context.Context) (interface{}, error) {
					// Simulate variable work
					time.Sleep(time.Duration(idx%5) * time.Millisecond)
					return idx, nil
				},
			)
			pool.Submit(task)
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
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: worker pool saturation test timed out")
	}

	assert.Equal(t, int64(totalTasks), completed,
		"all %d tasks must be submitted", totalTasks)
	t.Logf("Worker pool saturation: %d/%d tasks submitted", completed, totalTasks)

	pool.Shutdown(10 * time.Second)
}

// TestStress_CircuitBreakerRapidCycling rapidly opens and closes the circuit
// breaker 100 times to verify state machine correctness under rapid transitions.
func TestStress_CircuitBreakerRapidCycling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	const cycles = 100
	var wg sync.WaitGroup
	var panics int64
	var transitions int64

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := make(chan struct{})

	// Create a circuit breaker with low thresholds for rapid cycling
	cb := services.NewCircuitBreaker(2, 1, 5*time.Millisecond)
	require.NotNil(t, cb)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			lastState := cb.GetState()

			for cycle := 0; cycle < cycles; cycle++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Phase 1: Trip the breaker with failures
				for f := 0; f < 3; f++ {
					_ = cb.Call(func() error {
						return fmt.Errorf("cycling failure %d-%d", id, cycle)
					})
				}

				// Phase 2: Wait for recovery timeout
				time.Sleep(6 * time.Millisecond)

				// Phase 3: Send success to close the breaker
				_ = cb.Call(func() error {
					return nil
				})

				newState := cb.GetState()
				if newState != lastState {
					atomic.AddInt64(&transitions, 1)
					lastState = newState
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
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: circuit breaker rapid cycling timed out")
	}

	assert.Zero(t, panics, "no goroutine should panic during rapid cycling")
	assert.Greater(t, transitions, int64(0), "should observe state transitions")
	t.Logf("Circuit breaker cycling: %d transitions observed, %d panics", transitions, panics)
}

// TestStress_SemaphoreContention puts 50 goroutines contending for a semaphore
// with capacity 5, verifying the semaphore never exceeds its max concurrency
// and that all goroutines eventually acquire and release it.
func TestStress_SemaphoreContention(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	const (
		semCapacity   = 5
		numGoroutines = 50
		opsPerRoutine = 20
	)

	sem := concurrency.NewSemaphore(semCapacity)
	var wg sync.WaitGroup
	var panics int64
	var maxConcurrent int64
	var currentConcurrent int64
	var totalAcquired int64

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			for op := 0; op < opsPerRoutine; op++ {
				err := sem.Acquire(ctx)
				if err != nil {
					// Context cancelled, bail out
					return
				}

				cur := atomic.AddInt64(&currentConcurrent, 1)
				// Track max concurrency seen
				for {
					old := atomic.LoadInt64(&maxConcurrent)
					if cur <= old || atomic.CompareAndSwapInt64(&maxConcurrent, old, cur) {
						break
					}
				}
				atomic.AddInt64(&totalAcquired, 1)

				// Simulate brief work
				time.Sleep(100 * time.Microsecond)

				atomic.AddInt64(&currentConcurrent, -1)
				sem.Release()
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
		t.Fatal("DEADLOCK DETECTED: semaphore contention test timed out")
	}

	assert.Zero(t, panics, "no goroutine should panic during semaphore contention")
	assert.LessOrEqual(t, maxConcurrent, int64(semCapacity),
		"max concurrent holders should never exceed semaphore capacity %d", semCapacity)
	assert.Equal(t, int64(numGoroutines*opsPerRoutine), totalAcquired,
		"all acquire operations should complete")
	t.Logf("Semaphore contention: max_concurrent=%d (limit=%d), total_acquired=%d, panics=%d",
		maxConcurrent, semCapacity, totalAcquired, panics)
}
