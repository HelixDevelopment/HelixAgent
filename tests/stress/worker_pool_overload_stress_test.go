//go:build stress
// +build stress

package stress

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	concurrency "digital.vasic.concurrency/pkg/pool"
)

// TestWorkerPoolOverload_10xCapacity submits tasks at 10x the queue capacity
// and asserts that backpressure works correctly: no goroutine explosion, no
// panics, and the pool remains stable after shutdown.
func TestWorkerPoolOverload_10xCapacity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const (
		workers       = 4
		queueCapacity = 100
		// Submit 10x the queue capacity
		totalTasks = queueCapacity * 10
	)

	goroutinesBefore := runtime.NumGoroutine()

	pool := concurrency.NewWorkerPool(&concurrency.PoolConfig{
		Workers:   workers,
		QueueSize: queueCapacity,
	})
	pool.Start()

	var (
		submitted int64
		rejected  int64
		executed  int64
		panics    int64
	)

	// Submit 10x capacity sequentially to guarantee backpressure activation.
	// The pool must apply backpressure (reject overflow) rather than spawning
	// unbounded goroutines.
	for i := 0; i < totalTasks; i++ {
		i := i
		func() {
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			task := concurrency.NewTaskFunc(
				fmt.Sprintf("overload-task-%d", i),
				func(ctx context.Context) (interface{}, error) {
					atomic.AddInt64(&executed, 1)
					return i, nil
				},
			)
			if pool.Submit(task) == nil {
				atomic.AddInt64(&submitted, 1)
			} else {
				atomic.AddInt64(&rejected, 1)
			}
		}()
	}

	// Measure goroutine count immediately after submissions to catch explosion.
	goroutinesAfterSubmit := runtime.NumGoroutine()

	done := make(chan struct{})
	go func() {
		pool.Shutdown(20 * time.Second)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: worker pool 10x overload shutdown timed out")
	}

	assert.Zero(t, panics, "no panics during 10x overload")

	// Backpressure assertion: goroutine count during overload must not explode.
	// Pool has `workers` worker goroutines — allow some overhead but not 10x growth.
	goroutineGrowth := goroutinesAfterSubmit - goroutinesBefore
	t.Logf("Goroutine growth during 10x overload: +%d (before=%d, during=%d)",
		goroutineGrowth, goroutinesBefore, goroutinesAfterSubmit)
	assert.Less(t, goroutineGrowth, 50,
		"goroutine count must not explode during 10x overload (backpressure must work)")

	// Some tasks must have been rejected (backpressure active)
	assert.Greater(t, rejected, int64(0),
		"backpressure must reject some tasks when queue is full (10x capacity)")

	// Total accounting: submitted + rejected == totalTasks
	assert.Equal(t, int64(totalTasks), submitted+rejected,
		"every task attempt must either be submitted or rejected")

	// Goroutine leak check after shutdown
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d", goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 20, "no goroutine leak after 10x overload pool shutdown")

	t.Logf("10x overload: total=%d, submitted=%d, rejected=%d, executed=%d, panics=%d",
		totalTasks, submitted, rejected, executed, panics)
}
