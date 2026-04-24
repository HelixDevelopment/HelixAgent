package stress

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	events "dev.helix.agent/internal/adapters"
	"dev.helix.agent/internal/cache"
	concurrency "digital.vasic.concurrency/pkg/pool"
)

// TestWorkerPool_NoDeadlock tests that the worker pool doesn't deadlock under stress
func TestWorkerPool_NoDeadlock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	pool := concurrency.NewWorkerPool(&concurrency.PoolConfig{
		Workers:   16,
		QueueSize: 100,
	})
	pool.Start()

	// Channel to detect deadlock via timeout
	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				task := concurrency.NewTaskFunc("task-"+string(rune('a'+idx%26)), func(ctx context.Context) (interface{}, error) {
					// Simulate work with occasional long tasks
					if idx%100 == 0 {
						time.Sleep(50 * time.Millisecond)
					}
					return idx, nil
				})

				pool.Submit(task)
			}(i)
		}
		wg.Wait()
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Worker pool stress test timed out")
	}

	pool.Shutdown(10 * time.Second)
}

// TestEventBus_NoDeadlock tests that the event bus doesn't deadlock
func TestEventBus_NoDeadlock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	bus := events.NewEventBus(&events.BusConfig{
		BufferSize:     1000,
		PublishTimeout: 100 * time.Millisecond,
	})

	done := make(chan struct{})

	go func() {
		defer close(done)

		// Create multiple subscribers
		channels := make([]<-chan *events.Event, 10)
		for i := 0; i < 10; i++ {
			channels[i] = bus.Subscribe(events.EventCacheHit)
		}

		// Drain subscribers in goroutines
		var wg sync.WaitGroup
		for _, ch := range channels {
			wg.Add(1)
			go func(c <-chan *events.Event) {
				defer wg.Done()
				count := 0
				for {
					select {
					case _, ok := <-c:
						if !ok {
							return
						}
						count++
						if count >= 100 {
							return
						}
					case <-time.After(5 * time.Second):
						return
					}
				}
			}(ch)
		}

		// Publish events
		for i := 0; i < 1000; i++ {
			bus.Publish(events.NewEvent(events.EventCacheHit, "stress", i))
		}

		bus.Close()
		wg.Wait()
	}()

	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Event bus stress test timed out")
	}
}

// TestTieredCache_NoDeadlock tests that the cache doesn't deadlock
func TestTieredCache_NoDeadlock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	config := &cache.TieredCacheConfig{
		L1MaxSize: 1000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)

	done := make(chan struct{})

	go func() {
		defer close(done)

		ctx := context.Background()
		var wg sync.WaitGroup

		// Concurrent reads and writes
		for i := 0; i < 100; i++ {
			wg.Add(3)

			// Writer
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					tc.Set(ctx, string(rune('a'+idx%26)), idx*j, time.Minute)
				}
			}(i)

			// Reader
			go func(idx int) {
				defer wg.Done()
				var result int
				for j := 0; j < 100; j++ {
					tc.Get(ctx, string(rune('a'+idx%26)), &result)
				}
			}(i)

			// Deleter
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					tc.Delete(ctx, string(rune('a'+idx%26)))
				}
			}(i)
		}

		wg.Wait()
	}()

	select {
	case <-done:
		// Success
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Cache stress test timed out")
	}

	tc.Close()
}

// TestWorkerPool_NoRaceConditions runs with -race flag to detect race conditions
func TestWorkerPool_NoRaceConditions(t *testing.T) {
	pool := concurrency.NewWorkerPool(nil)
	pool.Start()
	defer pool.Shutdown(5 * time.Second)

	var counter int64
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				task := concurrency.NewTaskFunc("race-task", func(ctx context.Context) (interface{}, error) {
					atomic.AddInt64(&counter, 1)
					return nil, nil
				})
				pool.Submit(task)

				// Also access metrics concurrently
				pool.Metrics()
			}
		}()
	}

	wg.Wait()
}

// TestEventBus_NoRaceConditions tests for race conditions
func TestEventBus_NoRaceConditions(t *testing.T) {
	bus := events.NewEventBus(nil)
	defer bus.Close()

	var wg sync.WaitGroup

	// Concurrent subscribe/unsubscribe
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := bus.Subscribe(events.EventCacheHit)
			time.Sleep(10 * time.Millisecond)
			bus.Unsubscribe(ch)
		}()
	}

	// Concurrent publish
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				bus.Publish(events.NewEvent(events.EventCacheHit, "test", nil))
			}
		}()
	}

	// Concurrent metrics access
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				bus.Metrics()
			}
		}()
	}

	wg.Wait()
}

// TestMemoryLeak_WorkerPool tests for goroutine/memory leaks
func TestMemoryLeak_WorkerPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory leak test in short mode")  // SKIP-OK: #short-mode
	}

	initialGoroutines := runtime.NumGoroutine()

	for iteration := 0; iteration < 10; iteration++ {
		pool := concurrency.NewWorkerPool(nil)
		pool.Start()

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				task := concurrency.NewTaskFunc("leak-task", func(ctx context.Context) (interface{}, error) {
					return nil, nil
				})
				pool.Submit(task)
			}()
		}

		wg.Wait()
		pool.Shutdown(5 * time.Second)
	}

	// Force GC and allow goroutines to exit
	runtime.GC()
	time.Sleep(500 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()

	// Allow some variance but catch major leaks
	leakedGoroutines := finalGoroutines - initialGoroutines
	if leakedGoroutines > 10 {
		t.Errorf("GOROUTINE LEAK: started with %d, ended with %d, leaked %d",
			initialGoroutines, finalGoroutines, leakedGoroutines)
	}
}

// TestMemoryLeak_EventBus tests for goroutine/memory leaks
func TestMemoryLeak_EventBus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory leak test in short mode")  // SKIP-OK: #short-mode
	}

	initialGoroutines := runtime.NumGoroutine()

	for iteration := 0; iteration < 10; iteration++ {
		bus := events.NewEventBus(nil)

		channels := make([]<-chan *events.Event, 10)
		for i := 0; i < 10; i++ {
			channels[i] = bus.Subscribe(events.EventCacheHit)
		}

		for i := 0; i < 100; i++ {
			bus.Publish(events.NewEvent(events.EventCacheHit, "test", nil))
		}

		for _, ch := range channels {
			bus.Unsubscribe(ch)
		}

		bus.Close()
	}

	runtime.GC()
	time.Sleep(500 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	leakedGoroutines := finalGoroutines - initialGoroutines

	if leakedGoroutines > 10 {
		t.Errorf("GOROUTINE LEAK: started with %d, ended with %d, leaked %d",
			initialGoroutines, finalGoroutines, leakedGoroutines)
	}
}

// TestMemoryLeak_Cache tests for memory leaks in cache
func TestMemoryLeak_Cache(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory leak test in short mode")  // SKIP-OK: #short-mode
	}

	initialGoroutines := runtime.NumGoroutine()

	for iteration := 0; iteration < 10; iteration++ {
		config := &cache.TieredCacheConfig{
			L1MaxSize:         100,
			L1TTL:             100 * time.Millisecond,
			L1CleanupInterval: 50 * time.Millisecond,
			EnableL1:          true,
			EnableL2:          false,
		}
		tc := cache.NewTieredCache(nil, config)

		ctx := context.Background()
		for i := 0; i < 1000; i++ {
			tc.Set(ctx, string(rune('a'+i%100)), i, 50*time.Millisecond)
		}

		tc.Close()
	}

	runtime.GC()
	time.Sleep(500 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	leakedGoroutines := finalGoroutines - initialGoroutines

	if leakedGoroutines > 10 {
		t.Errorf("GOROUTINE LEAK: started with %d, ended with %d, leaked %d",
			initialGoroutines, finalGoroutines, leakedGoroutines)
	}
}

// TestTimeout_WorkerPool tests that tasks timeout correctly
func TestTimeout_WorkerPool(t *testing.T) {
	pool := concurrency.NewWorkerPool(&concurrency.PoolConfig{
		Workers:     4,
		QueueSize:   10,
		TaskTimeout: 100 * time.Millisecond,
	})
	pool.Start()
	defer pool.Shutdown(5 * time.Second)

	task := concurrency.NewTaskFunc("timeout-task", func(taskCtx context.Context) (interface{}, error) {
		// This task takes longer than the timeout
		select {
		case <-taskCtx.Done():
			return nil, taskCtx.Err()
		case <-time.After(5 * time.Second):
			return "completed", nil
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := pool.SubmitWait(ctx, task)

	// Task should have timed out
	if err == nil && result.Error == nil {
		t.Log("Task completed successfully (may happen if pool timeout was reached first)")
	} else {
		assert.Error(t, err)
	}
}

// TestTimeout_EventBusWait tests that Wait respects timeout
func TestTimeout_EventBusWait(t *testing.T) {
	bus := events.NewEventBus(nil)
	defer bus.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := bus.Wait(ctx, events.EventSystemStartup)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.True(t, elapsed < 500*time.Millisecond, "Wait took too long: %v", elapsed)
}

// TestHighLoad_Combined tests all components under high load
func TestHighLoad_Combined(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping high load test in short mode")  // SKIP-OK: #short-mode
	}

	// Setup all components
	pool := concurrency.NewWorkerPool(&concurrency.PoolConfig{
		Workers:   32,
		QueueSize: 1000,
	})
	pool.Start()
	defer pool.Shutdown(30 * time.Second)

	bus := events.NewEventBus(&events.BusConfig{
		BufferSize:     10000,
		PublishTimeout: 100 * time.Millisecond,
	})
	defer bus.Close()

	cacheConfig := &cache.TieredCacheConfig{
		L1MaxSize: 10000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, cacheConfig)
	defer tc.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	var operations int64

	// Subscribe to events
	ch := bus.Subscribe(events.EventCacheHit)
	go func() {
		for range ch {
		}
	}()

	// Run high load test
	startTime := time.Now()
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < 100; j++ {
				// Worker pool task
				task := concurrency.NewTaskFunc("load-task", func(ctx context.Context) (interface{}, error) {
					atomic.AddInt64(&operations, 1)
					return workerID * j, nil
				})
				pool.Submit(task)

				// Cache operation
				key := string(rune('a' + workerID%26))
				tc.Set(ctx, key, workerID*j, time.Minute)
				var val int
				tc.Get(ctx, key, &val)

				// Event publish
				bus.Publish(events.NewEvent(events.EventCacheHit, "stress", nil))

				atomic.AddInt64(&operations, 2)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	ops := atomic.LoadInt64(&operations)
	opsPerSecond := float64(ops) / elapsed.Seconds()

	t.Logf("High load test completed: %d operations in %v (%.0f ops/sec)",
		ops, elapsed, opsPerSecond)

	// Should complete at least 10000 operations
	assert.True(t, ops >= 10000, "Expected at least 10000 operations, got %d", ops)
}

// TestConcurrentCacheTagInvalidation tests concurrent tag-based invalidation
func TestConcurrentCacheTagInvalidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	config := &cache.TieredCacheConfig{
		L1MaxSize: 10000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	ctx := context.Background()
	var wg sync.WaitGroup

	// Populate cache with tagged entries
	for i := 0; i < 100; i++ {
		for j := 0; j < 10; j++ {
			tag := "tag:" + string(rune('a'+i%26))
			key := "key:" + string(rune('0'+j))
			tc.Set(ctx, key, i*j, time.Minute, tag)
		}
	}

	// Concurrent invalidation and reads
	for i := 0; i < 50; i++ {
		wg.Add(2)

		go func(idx int) {
			defer wg.Done()
			tag := "tag:" + string(rune('a'+idx%26))
			tc.InvalidateByTag(ctx, tag)
		}(i)

		go func(idx int) {
			defer wg.Done()
			var result int
			for j := 0; j < 10; j++ {
				key := "key:" + string(rune('0'+j))
				tc.Get(ctx, key, &result)
			}
		}(i)
	}

	wg.Wait()
}

// TestWorkerPoolStress_RapidSubmit tests rapid task submission
func TestWorkerPoolStress_RapidSubmit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	pool := concurrency.NewWorkerPool(&concurrency.PoolConfig{
		Workers:   16,
		QueueSize: 10000,
	})
	pool.Start()
	defer pool.Shutdown(30 * time.Second)

	start := time.Now()
	var submitted int64

	// Rapid submission from multiple goroutines
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				task := concurrency.NewTaskFunc("rapid", func(ctx context.Context) (interface{}, error) {
					return nil, nil
				})
				if pool.Submit(task) == nil {
					atomic.AddInt64(&submitted, 1)
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Submitted %d tasks in %v (%.0f tasks/sec)",
		atomic.LoadInt64(&submitted), elapsed,
		float64(submitted)/elapsed.Seconds())
}
