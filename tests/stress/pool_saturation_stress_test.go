//go:build stress
// +build stress

package stress

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"digital.vasic.concurrency/pkg/semaphore"
)

// TestPoolSaturation_1000Concurrent launches 1000 goroutines that all compete
// for a bounded semaphore pool. It asserts a >= 90% success rate and verifies
// no deadlocks occur. The semaphore is large enough to handle the load but
// bounded so some goroutines may timeout — the test validates graceful handling.
func TestPoolSaturation_1000Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const (
		concurrency = 1000
		poolSize    = 200 // large enough to allow 90% success
	)

	sem := semaphore.New(poolSize)

	goroutinesBefore := runtime.NumGoroutine()

	var (
		wg      sync.WaitGroup
		success int64
		failed  int64
		panics  int64
	)

	start := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-start

			// Each goroutine gets 10s to acquire — gracefully timeout rather than deadlock.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := sem.Acquire(ctx, 1); err != nil {
				atomic.AddInt64(&failed, 1)
				return
			}
			// Simulate brief work in the critical section
			time.Sleep(2 * time.Millisecond)
			sem.Release(1)
			atomic.AddInt64(&success, 1)
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Deadlock detection: 30s total timeout
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: 1000-goroutine pool saturation timed out after 30s")
	}

	assert.Zero(t, panics, "no goroutine should panic during pool saturation")
	assert.Equal(t, int64(concurrency), success+failed,
		"all goroutines must complete (no hangs)")

	// Numeric threshold: >= 90% success rate
	successRate := float64(success) / float64(concurrency)
	assert.GreaterOrEqual(t, successRate, 0.90,
		"success rate must be >= 90%%, got %.2f%% (%d/%d)", successRate*100, success, concurrency)

	// Semaphore must be fully released
	assert.Equal(t, int64(poolSize), sem.Available(),
		"semaphore must be fully released after all goroutines complete")

	// Goroutine leak check
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d", goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 20, "no significant goroutine leak after pool saturation")

	t.Logf("Pool saturation 1000-concurrent: success=%d (%.1f%%), failed=%d, panics=%d",
		success, successRate*100, failed, panics)
}
