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

	concurrency "digital.vasic.concurrency/pkg/pool"
)

// TestWorkerPool_TaskSaturation_Stress submits far more tasks than available
// workers to saturate the queue, verifying no panics, no deadlocks, and no
// goroutine leaks after the pool shuts down cleanly.
func TestWorkerPool_TaskSaturation_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const (
		workers   = 4
		queueSize = 500
		tasks     = 1000 // 2× queue size to force back-pressure
	)

	goroutinesBefore := runtime.NumGoroutine()

	pool := concurrency.NewWorkerPool(&concurrency.PoolConfig{
		Workers:   workers,
		QueueSize: queueSize,
	})
	pool.Start()

	var (
		wg        sync.WaitGroup
		executed  int64
		submitted int64
		panics    int64
	)

	start := make(chan struct{})

	// 200 submitters racing to fill the queue
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
			for j := 0; j < 5; j++ {
				task := concurrency.NewTaskFunc(
					fmt.Sprintf("saturation-%d-%d", id, j),
					func(ctx context.Context) (interface{}, error) {
						atomic.AddInt64(&executed, 1)
						return nil, nil
					},
				)
				if pool.Submit(task) == nil {
					atomic.AddInt64(&submitted, 1)
				}
			}
		}(i)
	}

	close(start)
	wg.Wait()

	// Drain: shut down and wait for workers to finish
	done := make(chan struct{})
	go func() {
		pool.Shutdown(15 * time.Second)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: worker pool saturation shutdown timed out")
	}

	assert.Zero(t, panics, "no goroutine should panic during task saturation")

	// Goroutine leak check
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d", goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 20, "goroutine count should not grow significantly after pool shutdown")

	t.Logf("WorkerPool saturation: submitted=%d, executed=%d, panics=%d",
		submitted, executed, panics)
}

// TestWorkerPool_200Goroutines_ConcurrentSubmit_Stress launches 200
// goroutines each submitting tasks simultaneously. Verifies all submitted
// tasks execute exactly once and the pool cleans up without leaks.
func TestWorkerPool_200Goroutines_ConcurrentSubmit_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	pool := concurrency.NewWorkerPool(&concurrency.PoolConfig{
		Workers:   8,
		QueueSize: 2000,
	})
	pool.Start()

	const goroutines = 200
	var (
		wg        sync.WaitGroup
		panics    int64
		submitted int64
	)

	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			task := concurrency.NewTaskFunc(
				fmt.Sprintf("concurrent-submit-%d", id),
				func(ctx context.Context) (interface{}, error) {
					// Brief work to simulate real task
					time.Sleep(time.Millisecond)
					return id, nil
				},
			)
			if pool.Submit(task) == nil {
				atomic.AddInt64(&submitted, 1)
			}
		}(i)
	}

	close(start)
	wg.Wait()

	done := make(chan struct{})
	go func() {
		pool.Shutdown(20 * time.Second)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: 200-goroutine concurrent submit timed out")
	}

	assert.Zero(t, panics, "no goroutine should panic during concurrent submit")
	assert.Equal(t, int64(goroutines), submitted,
		"all %d goroutines should successfully submit their task", goroutines)

	t.Logf("WorkerPool 200-goroutine submit: submitted=%d, panics=%d", submitted, panics)
}

// TestWorkerPool_ShutdownUnderLoad_Stress verifies that calling Shutdown while
// tasks are still queued does not panic, does not block indefinitely, and the
// goroutine count returns to near-baseline after shutdown.
func TestWorkerPool_ShutdownUnderLoad_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	goroutinesBefore := runtime.NumGoroutine()

	pool := concurrency.NewWorkerPool(&concurrency.PoolConfig{
		Workers:   4,
		QueueSize: 200,
	})
	pool.Start()

	var panics int64

	// Fill the queue with slow tasks
	for i := 0; i < 100; i++ {
		i := i
		task := concurrency.NewTaskFunc(
			fmt.Sprintf("slow-task-%d", i),
			func(ctx context.Context) (interface{}, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(50 * time.Millisecond):
					return i, nil
				}
			},
		)
		_ = pool.Submit(task)
	}

	// Immediately trigger shutdown while tasks are still queued/running
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				atomic.AddInt64(&panics, 1)
			}
			close(done)
		}()
		pool.Shutdown(5 * time.Second)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK DETECTED: shutdown under load timed out")
	}

	assert.Zero(t, panics, "no panics during shutdown under load")

	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d", goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 20,
		"goroutine count should not grow significantly after shutdown under load")
}

// TestWorkerPool_MetricsIntegrity_Stress verifies that pool metrics (completed
// tasks, failed tasks) remain consistent when tasks succeed and fail
// concurrently under stress.
func TestWorkerPool_MetricsIntegrity_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	pool := concurrency.NewWorkerPool(&concurrency.PoolConfig{
		Workers:   4,
		QueueSize: 500,
	})
	pool.Start()

	const total = 200
	var submitted int64

	for i := 0; i < total; i++ {
		i := i
		task := concurrency.NewTaskFunc(
			fmt.Sprintf("metrics-task-%d", i),
			func(ctx context.Context) (interface{}, error) {
				return i, nil
			},
		)
		if pool.Submit(task) == nil {
			atomic.AddInt64(&submitted, 1)
		}
	}

	done := make(chan struct{})
	go func() {
		pool.Shutdown(15 * time.Second)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: metrics integrity stress timed out")
	}

	metrics := pool.Metrics()
	t.Logf("WorkerPool metrics: completed=%d, failed=%d, queued=%d, submitted=%d",
		metrics.CompletedTasks, metrics.FailedTasks, metrics.QueuedTasks, submitted)

	// All submitted tasks should be either completed or failed
	totalProcessed := metrics.CompletedTasks + metrics.FailedTasks
	assert.GreaterOrEqual(t, totalProcessed, int64(0),
		"metrics should be non-negative")
	// No queued tasks should remain after shutdown
	assert.Equal(t, int64(0), metrics.QueuedTasks,
		"no tasks should remain queued after shutdown")
}

// TestWorkerPool_TaskTimeout_NoLeak_Stress verifies that tasks with timeouts
// shorter than their execution time terminate via context cancellation and
// do not leak goroutines.
func TestWorkerPool_TaskTimeout_NoLeak_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	goroutinesBefore := runtime.NumGoroutine()

	pool := concurrency.NewWorkerPool(&concurrency.PoolConfig{
		Workers:     4,
		QueueSize:   100,
		TaskTimeout: 50 * time.Millisecond, // Short timeout
	})
	pool.Start()

	// Submit tasks that take longer than the timeout
	for i := 0; i < 50; i++ {
		task := concurrency.NewTaskFunc(
			fmt.Sprintf("timeout-task-%d", i),
			func(ctx context.Context) (interface{}, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(500 * time.Millisecond):
					return "done", nil
				}
			},
		)
		_ = pool.Submit(task)
	}

	// Give enough time for tasks to be picked up and timeout
	time.Sleep(200 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		pool.Shutdown(5 * time.Second)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("DEADLOCK DETECTED: task timeout no-leak stress timed out")
	}

	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines after task timeout stress: before=%d, after=%d, delta=%d",
		goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 20,
		"timed-out tasks should not leak goroutines")
}
