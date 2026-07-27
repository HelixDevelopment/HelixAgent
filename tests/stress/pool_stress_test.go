//go:build stress
// +build stress

package stress

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"digital.vasic.concurrency/pkg/semaphore"
)

// TestPool_ConcurrentAcquireRelease_Stress validates that the semaphore
// Acquire/Release cycle is race-free under 200 concurrent goroutines.
// All goroutines acquire and immediately release weight=1. The test
// asserts no panics, near-zero error rate, and no goroutine leaks.
func TestPool_ConcurrentAcquireRelease_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const (
		concurrency = 200
		maxWeight   = 50 // Only 50 simultaneous holders at any time
	)

	sem := semaphore.New(maxWeight)

	var (
		wg      sync.WaitGroup
		panics  int64
		errors  int64
		success int64
	)

	goroutinesBefore := runtime.NumGoroutine()
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

			ctx, cancel := context.WithTimeout(
				context.Background(), 10*time.Second,
			)
			defer cancel()

			if err := sem.Acquire(ctx, 1); err != nil {
				atomic.AddInt64(&errors, 1)
				return
			}
			// Simulate brief critical section
			time.Sleep(time.Millisecond)
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

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: semaphore stress at concurrency 200 timed out")
	}

	assert.Zero(t, panics, "no goroutine should panic during semaphore stress")
	assert.Equal(t, int64(concurrency), success+errors,
		"all goroutines must complete")
	// Errors are only context cancellations — keep below 10%
	errorRate := float64(errors) / float64(concurrency)
	assert.Less(t, errorRate, 0.10,
		"error rate should be below 10%%, got %.2f%%", errorRate*100)

	// Verify full release: semaphore weight should be fully available
	assert.Equal(t, int64(maxWeight), sem.Available(),
		"semaphore should be fully released after all goroutines finish")

	// Allow goroutines to settle, then check for leaks
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d", goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 20, "goroutine count should not grow significantly")

	t.Logf("Pool stress: %d success, %d errors, %d panics (concurrency=%d)",
		success, errors, panics, concurrency)
}

// TestPool_AcquireRelease_NoDeadlock_Stress verifies that acquire + release
// under mixed weight values (1 and 5) does not deadlock or leave the semaphore
// in a permanently locked state.
func TestPool_AcquireRelease_NoDeadlock_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	const maxWeight = 100
	sem := semaphore.New(maxWeight)

	var wg sync.WaitGroup
	var panics int64
	start := make(chan struct{})

	// 100 lightweight goroutines acquire weight=1
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if sem.Acquire(ctx, 1) == nil {
				time.Sleep(500 * time.Microsecond)
				sem.Release(1)
			}
		}()
	}

	// 20 heavyweight goroutines acquire weight=5
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if sem.Acquire(ctx, 5) == nil {
				time.Sleep(time.Millisecond)
				sem.Release(5)
			}
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
		t.Fatal("DEADLOCK DETECTED: mixed-weight semaphore stress timed out")
	}

	assert.Zero(t, panics, "no panics should occur during mixed-weight stress")
	assert.Equal(t, int64(maxWeight), sem.Available(),
		"semaphore should be fully released after all goroutines complete")
}

// TestPool_TryAcquire_Contention_Stress verifies that TryAcquire returns
// false (not panic) when the semaphore is fully contested, and that the
// available weight stays consistent throughout.
func TestPool_TryAcquire_Contention_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	const maxWeight = 10
	sem := semaphore.New(maxWeight)

	var (
		wg       sync.WaitGroup
		panics   int64
		acquired int64
		rejected int64
	)

	start := make(chan struct{})

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			if sem.TryAcquire(1) {
				atomic.AddInt64(&acquired, 1)
				// Hold briefly then release
				time.Sleep(5 * time.Millisecond)
				sem.Release(1)
			} else {
				atomic.AddInt64(&rejected, 1)
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
		t.Fatal("DEADLOCK DETECTED: TryAcquire contention stress timed out")
	}

	assert.Zero(t, panics, "no panics during TryAcquire contention stress")
	assert.Equal(t, int64(200), acquired+rejected,
		"every goroutine must finish with either an acquire or a rejection")
	// Available weight must return to maxWeight
	assert.Equal(t, int64(maxWeight), sem.Available(),
		"semaphore must be fully released after all goroutines complete")

	t.Logf("TryAcquire stress: acquired=%d, rejected=%d, panics=%d",
		acquired, rejected, panics)
}

// TestPool_ContextCancellation_Stress verifies that cancelling a context
// while goroutines are blocked in Acquire results in a clean return of
// ctx.Err(), no leaks, and correct semaphore state afterwards.
func TestPool_ContextCancellation_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	goroutinesBefore := runtime.NumGoroutine()

	// Use a very small semaphore so most goroutines must block
	const maxWeight = 2
	sem := semaphore.New(maxWeight)

	// Pre-acquire all capacity so incoming goroutines must wait
	ctx0 := context.Background()
	if err := sem.Acquire(ctx0, maxWeight); err != nil {
		t.Fatalf("initial acquire failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	var cancelErrors int64
	var panics int64

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			err := sem.Acquire(ctx, 1)
			if err != nil {
				atomic.AddInt64(&cancelErrors, 1)
			} else {
				sem.Release(1)
			}
		}()
	}

	// Let context expire
	<-ctx.Done()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("goroutines did not exit after context cancellation")
	}

	// Release the pre-acquired capacity
	sem.Release(maxWeight)

	assert.Zero(t, panics, "no panics during context cancellation stress")
	assert.Greater(t, cancelErrors, int64(0),
		"at least some goroutines should have received context cancellation errors")

	// Semaphore must be fully available after cleanup
	assert.Equal(t, int64(maxWeight), sem.Available(),
		"semaphore should be fully available after context cancellation")

	// Goroutine leak check
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutine leak check: before=%d, after=%d, delta=%d",
		goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 10, "no significant goroutine leak after cancellation")

	t.Logf("Context cancellation stress: cancelled=%d, panics=%d", cancelErrors, panics)
}

// TestPool_Metrics_ConsistencyUnderStress verifies that Current() and Available()
// remain consistent (Current + Available == maxWeight) under concurrent load.
func TestPool_Metrics_ConsistencyUnderStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	const maxWeight = 20
	sem := semaphore.New(maxWeight)

	var (
		wg         sync.WaitGroup
		panics     int64
		violations int64
	)

	start := make(chan struct{})

	for i := 0; i < 150; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			for j := 0; j < 5; j++ {
				if err := sem.Acquire(ctx, 1); err != nil {
					return
				}

				// Check consistency while holding the semaphore
				curr := sem.Current()
				avail := sem.Available()
				if curr+avail != maxWeight {
					atomic.AddInt64(&violations, 1)
				}

				sem.Release(1)

				// Check consistency after release
				curr = sem.Current()
				avail = sem.Available()
				if curr+avail != maxWeight {
					atomic.AddInt64(&violations, 1)
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
		t.Fatal("DEADLOCK DETECTED: metrics consistency stress timed out")
	}

	assert.Zero(t, panics, "no panics during metrics consistency stress")
	assert.Zero(t, violations,
		fmt.Sprintf("Current()+Available() must always equal maxWeight=%d", maxWeight))
	assert.Equal(t, int64(maxWeight), sem.Available(),
		"semaphore must be fully available after all goroutines finish")
}
