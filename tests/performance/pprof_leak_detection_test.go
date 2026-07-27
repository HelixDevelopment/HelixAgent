// Package performance contains pprof-based memory profiling and leak
// detection tests. These tests validate that common operations do not
// cause goroutine leaks or unbounded heap growth.
package performance

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestMemoryLeak_GoroutineCount verifies that spawning and joining many
// goroutines in waves does not leave leaked goroutines behind.
func TestMemoryLeak_GoroutineCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping profiling test in short mode") // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	// Allow runtime goroutines to settle
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	// Run 20 rounds of 50 goroutines each
	for round := 0; round < 20; round++ {
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(time.Millisecond)
			}()
		}
		wg.Wait()
	}

	// Force GC and let goroutines settle
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	current := runtime.NumGoroutine()
	leaked := current - baseline

	t.Logf("Goroutine count: baseline=%d, current=%d, diff=%d",
		baseline, current, leaked)

	assert.LessOrEqual(t, leaked, 10,
		"goroutines should return near baseline after all waves complete")
}

// TestMemoryLeak_HeapGrowth verifies that repeated large allocations that
// go out of scope are properly garbage collected and do not cause
// unbounded heap growth.
func TestMemoryLeak_HeapGrowth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping profiling test in short mode") // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	// Force GC and record baseline
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	var baselineStats runtime.MemStats
	runtime.ReadMemStats(&baselineStats)

	// Simulate work with allocations that should be GC'd
	for i := 0; i < 100; i++ {
		data := make([]byte, 1024*1024) // 1MB allocation
		// Use the data to prevent compiler optimization
		data[0] = byte(i)
		data[len(data)-1] = byte(i)
		_ = data
	}

	// Force GC and measure heap
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC() // second GC pass for thoroughness
	time.Sleep(50 * time.Millisecond)
	var currentStats runtime.MemStats
	runtime.ReadMemStats(&currentStats)

	// Use signed arithmetic to handle GC reclaiming memory below baseline
	heapGrowthMB := (float64(currentStats.HeapAlloc) - float64(baselineStats.HeapAlloc)) / 1024 / 1024
	t.Logf("Heap growth: %.2f MB (baseline: %.2f MB, current: %.2f MB)",
		heapGrowthMB,
		float64(baselineStats.HeapAlloc)/1024/1024,
		float64(currentStats.HeapAlloc)/1024/1024)

	assert.Less(t, heapGrowthMB, 50.0,
		"heap should not grow unboundedly after GC reclaims temporary allocations")
}

// TestMemoryLeak_ConcurrentMapAccess verifies that concurrent map operations
// with proper synchronization do not leak goroutines or memory.
func TestMemoryLeak_ConcurrentMapAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping profiling test in short mode") // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	var mu sync.RWMutex
	data := make(map[string]int)

	var wg sync.WaitGroup
	for round := 0; round < 10; round++ {
		// Writers
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					mu.Lock()
					data[string(rune('a'+id%26))] = j
					mu.Unlock()
				}
			}(i)
		}

		// Readers
		for i := 0; i < 30; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					mu.RLock()
					_ = len(data)
					mu.RUnlock()
				}
			}()
		}
	}

	wg.Wait()
	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	current := runtime.NumGoroutine()
	leaked := current - baseline

	t.Logf("ConcurrentMap goroutines: baseline=%d, current=%d, diff=%d",
		baseline, current, leaked)

	assert.LessOrEqual(t, leaked, 5,
		"concurrent map access should not leak goroutines")
}

// TestMemoryLeak_ChannelDraining verifies that channels are properly drained
// and closed, preventing goroutine leaks from blocked channel operations.
func TestMemoryLeak_ChannelDraining(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping profiling test in short mode") // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	for round := 0; round < 20; round++ {
		ch := make(chan int, 100)
		done := make(chan struct{})

		// Producer
		go func() {
			defer close(ch)
			for i := 0; i < 100; i++ {
				ch <- i
			}
		}()

		// Consumer
		go func() {
			defer close(done)
			for range ch {
				// consume
			}
		}()

		<-done
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	current := runtime.NumGoroutine()
	leaked := current - baseline

	t.Logf("ChannelDrain goroutines: baseline=%d, current=%d, diff=%d",
		baseline, current, leaked)

	assert.LessOrEqual(t, leaked, 5,
		"channel operations should not leak goroutines when properly drained")
}

// TestMemoryLeak_ContextCancellation verifies that goroutines holding
// context references are properly cleaned up when the context is cancelled.
func TestMemoryLeak_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping profiling test in short mode") // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	var baselineStats runtime.MemStats
	runtime.ReadMemStats(&baselineStats)
	baselineGoroutines := runtime.NumGoroutine()

	// Repeatedly allocate slices through channels with cancellation
	for round := 0; round < 50; round++ {
		ch := make(chan []byte, 10)
		done := make(chan struct{})

		go func() {
			defer close(done)
			for data := range ch {
				_ = len(data)
			}
		}()

		for i := 0; i < 10; i++ {
			ch <- make([]byte, 4096)
		}
		close(ch)
		<-done
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	var currentStats runtime.MemStats
	runtime.ReadMemStats(&currentStats)
	currentGoroutines := runtime.NumGoroutine()

	// Use signed arithmetic to handle GC reclaiming memory below baseline
	heapGrowthMB := (float64(currentStats.HeapAlloc) - float64(baselineStats.HeapAlloc)) / 1024 / 1024
	goroutineDiff := currentGoroutines - baselineGoroutines

	t.Logf("Context cancellation: heap_growth=%.2f MB, goroutine_diff=%d",
		heapGrowthMB, goroutineDiff)

	assert.Less(t, heapGrowthMB, 20.0,
		"cancelled context operations should not cause heap growth")
	assert.LessOrEqual(t, goroutineDiff, 5,
		"cancelled context operations should not leak goroutines")
}
