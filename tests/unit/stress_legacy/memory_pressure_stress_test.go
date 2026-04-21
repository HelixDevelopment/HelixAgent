// Previously tests/stress/... — demoted to unit package under CONST-030.
// Stress tests of in-process concurrency primitives (semaphore / pool /
// cache saturation, circuit-breaker storms, goroutine growth) that
// exercise local mock providers and invokers. Keeping the mocks in the
// unit tree per the audit's Pattern-4 disposition for stress tests.
// PR33 of the CONST-030 compliance campaign.
package stress_legacy_test

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/cache"
)

// heapAllocMB returns the current heap allocation in megabytes.
func heapAllocMB() float64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return float64(ms.HeapAlloc) / (1024 * 1024)
}

// TestStress_MemoryPressure_CacheWriteBurst verifies that executing 500 cache
// write operations does not permanently grow the heap by more than 50% from
// the post-GC baseline. A forced GC after the burst allows the allocator to
// reclaim short-lived objects before the final measurement.
func TestStress_MemoryPressure_CacheWriteBurst(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	// Establish a clean baseline after GC
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineMB := heapAllocMB()

	config := &cache.TieredCacheConfig{
		L1MaxSize: 2000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	ctx := context.Background()

	// Execute 500 cache write operations with varied payload sizes
	const operations = 500
	for i := 0; i < operations; i++ {
		key := fmt.Sprintf("mem-pressure-key-%d", i)
		// Vary payload size: small, medium, large
		var value interface{}
		switch i % 3 {
		case 0:
			value = fmt.Sprintf("small-value-%d", i)
		case 1:
			value = make([]byte, 1024) // 1 KB
		case 2:
			value = make([]byte, 4096) // 4 KB
		}
		tc.Set(ctx, key, value, time.Minute)
	}

	// Force GC to reclaim transient allocations
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	afterMB := heapAllocMB()

	growthFactor := afterMB / baselineMB
	t.Logf("Cache write burst memory: baseline=%.2fMB, after=%.2fMB, "+
		"growth=%.2fx (%d operations)",
		baselineMB, afterMB, growthFactor, operations)

	// Heap should not grow by more than 50% from baseline after GC
	assert.Less(t, growthFactor, 1.5,
		"heap should not grow by more than 50%% after 500 cache write operations")
}

// TestStress_MemoryPressure_ConcurrentOperations measures heap growth during
// concurrent cache operations across 20 goroutines (read + write mix).
// Validates that concurrent access patterns do not cause runaway allocation.
func TestStress_MemoryPressure_ConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineMB := heapAllocMB()

	config := &cache.TieredCacheConfig{
		L1MaxSize: 1000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	ctx := context.Background()

	const goroutineCount = 20
	const opsPerGoroutine = 25 // 20 * 25 = 500 total

	var wg sync.WaitGroup
	var totalOps atomic.Int64

	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start

			for j := 0; j < opsPerGoroutine; j++ {
				key := fmt.Sprintf("concurrent-mem-%d-%d", id, j%50)
				if j%2 == 0 {
					tc.Set(ctx, key, make([]byte, 512), time.Minute)
				} else {
					var result []byte
					tc.Get(ctx, key, &result)
				}
				totalOps.Add(1)
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: concurrent memory pressure test timed out")
	}

	assert.Equal(t, int64(goroutineCount*opsPerGoroutine), totalOps.Load(),
		"all operations should complete")

	// Force GC and measure final heap
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	afterMB := heapAllocMB()

	growthFactor := afterMB / baselineMB
	t.Logf("Concurrent operations memory: baseline=%.2fMB, after=%.2fMB, "+
		"growth=%.2fx (%d ops across %d goroutines)",
		baselineMB, afterMB, growthFactor, totalOps.Load(), goroutineCount)

	assert.Less(t, growthFactor, 1.5,
		"heap should not grow by more than 50%% after concurrent cache operations")
}

// TestStress_MemoryPressure_AllocFreePattern verifies that a repeated
// alloc-use-release pattern (simulating request processing) does not cause
// heap growth ratcheting. After 500 iterations each allocating and discarding
// temporary buffers, the heap should stay near baseline.
func TestStress_MemoryPressure_AllocFreePattern(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineMB := heapAllocMB()

	const iterations = 500
	var sink []byte // prevent compiler optimisation

	for i := 0; i < iterations; i++ {
		// Simulate request-scoped allocation: allocate, use, discard
		buf := make([]byte, 8192) // 8 KB per iteration
		for j := range buf {
			buf[j] = byte(i + j)
		}
		sink = buf[:1] // keep alive briefly to prevent optimisation

		// Simulate cache key construction
		key := fmt.Sprintf("alloc-free-key-%d", i%100)
		_ = key

		// Simulate response struct allocation
		type responseStub struct {
			Content  string
			Metadata map[string]interface{}
			Tags     []string
		}
		resp := &responseStub{
			Content:  fmt.Sprintf("response-%d", i),
			Metadata: map[string]interface{}{"iteration": i},
			Tags:     []string{"tag-a", "tag-b"},
		}
		_ = resp
	}
	_ = sink

	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	afterMB := heapAllocMB()

	growthFactor := afterMB / baselineMB
	t.Logf("Alloc-free pattern memory: baseline=%.2fMB, after=%.2fMB, "+
		"growth=%.2fx (%d iterations)",
		baselineMB, afterMB, growthFactor, iterations)

	assert.Less(t, growthFactor, 1.5,
		"heap should not grow by more than 50%% after alloc-free pattern")
}

// TestStress_MemoryPressure_MultipleGCCycles verifies that running multiple
// GC cycles during concurrent cache operations does not cause heap instability
// or expose use-after-free bugs (which would manifest as crashes or wrong values).
func TestStress_MemoryPressure_MultipleGCCycles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GOMAXPROCS(2)

	config := &cache.TieredCacheConfig{
		L1MaxSize: 500,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	ctx := context.Background()

	var wg sync.WaitGroup
	var panicCount atomic.Int64
	var opsCompleted atomic.Int64

	// Background GC trigger goroutine
	gcDone := make(chan struct{})
	go func() {
		defer close(gcDone)
		for i := 0; i < 10; i++ {
			runtime.GC()
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Concurrent cache operations during GC cycles
	const goroutineCount = 30
	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
				}
			}()
			<-start

			for j := 0; j < 20; j++ {
				key := fmt.Sprintf("gc-cycle-key-%d-%d", id, j)
				value := fmt.Sprintf("gc-cycle-value-%d-%d", id, j)

				tc.Set(ctx, key, value, time.Minute)

				var result string
				tc.Get(ctx, key, &result)

				opsCompleted.Add(1)
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: GC cycle stress test timed out")
	}

	<-gcDone

	assert.Zero(t, panicCount.Load(), "no panics during concurrent GC cycles")
	assert.Equal(t, int64(goroutineCount*20), opsCompleted.Load(),
		"all cache operations should complete during GC pressure")

	t.Logf("Multiple GC cycles: ops=%d, panics=%d", opsCompleted.Load(), panicCount.Load())
}

// TestStress_MemoryPressure_MemStatsReadsDuringLoad verifies that calling
// runtime.ReadMemStats concurrently with cache operations does not cause
// data races or abnormal blocking (ReadMemStats does a stop-the-world).
func TestStress_MemoryPressure_MemStatsReadsDuringLoad(t *testing.T) {
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
	defer tc.Close()

	ctx := context.Background()

	var wg sync.WaitGroup
	var panicCount atomic.Int64

	opsDone := make(chan struct{})

	// Cache operations goroutines
	const opsGoroutines = 20
	start := make(chan struct{})

	for i := 0; i < opsGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
				}
			}()
			<-start

			for j := 0; j < 25; j++ {
				key := fmt.Sprintf("memstats-key-%d-%d", id, j)
				tc.Set(ctx, key, id*j, time.Minute)
				var result int
				tc.Get(ctx, key, &result)
			}
		}(i)
	}

	// MemStats reader goroutine — runs concurrently with cache ops
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				panicCount.Add(1)
			}
		}()
		<-start

		var ms runtime.MemStats
		reads := 0
		for {
			select {
			case <-opsDone:
				return
			default:
			}
			runtime.ReadMemStats(&ms)
			reads++
			// Small yield to avoid monopolising stop-the-world
			time.Sleep(5 * time.Millisecond)
		}
	}()

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	// Signal MemStats reader to stop once cache ops finish
	go func() {
		// Wait a bit to let cache ops start, then signal done
		time.Sleep(50 * time.Millisecond)
		close(opsDone)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: MemStats reads during load timed out")
	}

	assert.Zero(t, panicCount.Load(),
		"no panics when ReadMemStats is called during concurrent cache load")

	t.Logf("MemStats reads during load: panics=%d", panicCount.Load())
}
