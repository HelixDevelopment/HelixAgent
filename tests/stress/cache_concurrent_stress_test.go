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

	"dev.helix.agent/internal/cache"
)

// TestStress_Cache_ConcurrentSetGetDelete exercises 50 goroutines performing
// simultaneous Set, Get, and Delete operations on the same key space to
// verify that the cache's internal locking prevents panics and corruption.
func TestStress_Cache_ConcurrentSetGetDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	config := &cache.TieredCacheConfig{
		L1MaxSize: 5000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	const goroutineCount = 50
	ctx := context.Background()

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var panicCount int64
	var setOps, getOps, deleteOps int64

	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 100; j++ {
				// Use overlapping key space to maximize contention
				key := fmt.Sprintf("stress-key-%d", (id*7+j)%100)

				switch j % 3 {
				case 0: // Set
					tc.Set(ctx, key, fmt.Sprintf("value-%d-%d", id, j), time.Minute)
					atomic.AddInt64(&setOps, 1)
				case 1: // Get
					var result string
					tc.Get(ctx, key, &result)
					atomic.AddInt64(&getOps, 1)
				case 2: // Delete
					tc.Delete(ctx, key)
					atomic.AddInt64(&deleteOps, 1)
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
		t.Fatal("DEADLOCK DETECTED: cache concurrent set/get/delete timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	totalOps := setOps + getOps + deleteOps
	assert.Zero(t, panicCount,
		"no panics under 50-goroutine concurrent cache access")
	assert.Equal(t, int64(goroutineCount*100), totalOps,
		"all operations should complete")
	assert.Less(t, leaked, 10,
		"goroutine count should remain stable after cache stress")
	t.Logf("Cache set/get/delete stress: sets=%d, gets=%d, deletes=%d, "+
		"total=%d, panics=%d, goroutine_leak=%d",
		setOps, getOps, deleteOps, totalOps, panicCount, leaked)
}

// TestStress_Cache_UserKeyMapConcurrency specifically targets the userKey
// mapping within the cache by having 50 goroutines simultaneously access
// keys with identical prefixes but different suffixes, creating high
// contention on the internal map structures.
func TestStress_Cache_UserKeyMapConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	config := &cache.TieredCacheConfig{
		L1MaxSize: 10000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	const goroutineCount = 50
	ctx := context.Background()

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var panicCount int64
	var totalOps int64

	start := make(chan struct{})

	// All goroutines target overlapping key prefixes to maximize map contention
	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			// Use a shared key prefix to stress the same map bucket
			prefix := "shared-prefix"
			for j := 0; j < 200; j++ {
				key := fmt.Sprintf("%s:%d:%d", prefix, id%10, j%50)

				// Rapid set-get-delete cycle on the same key
				tc.Set(ctx, key, j, time.Minute)
				var result int
				tc.Get(ctx, key, &result)
				if j%5 == 0 {
					tc.Delete(ctx, key)
				}
				atomic.AddInt64(&totalOps, 1)
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
		t.Fatal("DEADLOCK DETECTED: cache userKey map concurrency timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	assert.Zero(t, panicCount,
		"no panics during userKey map concurrent access")
	assert.Equal(t, int64(goroutineCount*200), totalOps,
		"all key-map operations should complete")
	assert.Less(t, leaked, 10,
		"goroutines should not leak after userKey map stress")

	// Verify cache is still functional after stress
	tc.Set(ctx, "post-stress-key", "alive", time.Minute)
	var postResult string
	found, err := tc.Get(ctx, "post-stress-key", &postResult)
	assert.NoError(t, err, "cache get should not error after stress")
	assert.True(t, found, "cache should still be functional after stress")
	assert.Equal(t, "alive", postResult)

	t.Logf("Cache userKey map stress: ops=%d, panics=%d, goroutine_leak=%d",
		totalOps, panicCount, leaked)
}

// TestStress_Cache_MemoryStabilityUnderLoad writes and reads a large volume
// of data through the cache to verify that memory usage remains bounded and
// no leaks occur when entries are continuously cycled.
func TestStress_Cache_MemoryStabilityUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	config := &cache.TieredCacheConfig{
		L1MaxSize: 500, // Small to force evictions
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	ctx := context.Background()

	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	const goroutineCount = 50

	var wg sync.WaitGroup
	var panicCount int64

	start := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 200; j++ {
				key := fmt.Sprintf("mem-stress-%d-%d", id, j)
				// Write medium-sized values to stress memory
				value := fmt.Sprintf("value-%d-%d-%s", id, j,
					"padding-data-for-memory-pressure")
				tc.Set(ctx, key, value, time.Minute)

				var result string
				tc.Get(ctx, key, &result)
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
		t.Fatal("DEADLOCK DETECTED: cache memory stability test timed out")
	}

	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	// Use signed arithmetic to handle GC reclaiming memory (after < before)
	heapGrowthMB := float64(int64(memAfter.HeapInuse)-int64(memBefore.HeapInuse)) / 1024 / 1024

	assert.Zero(t, panicCount, "no panics during memory stability stress")
	assert.Less(t, heapGrowthMB, 200.0,
		"heap growth should be bounded during cache cycling")

	// Verify cache size is within bounds
	metrics := tc.Metrics()
	assert.LessOrEqual(t, metrics.L1Size, int64(config.L1MaxSize),
		"cache size should not exceed max after stress")

	t.Logf("Cache memory stress: heap_growth=%.2fMB, L1_size=%d, "+
		"L1_evictions=%d, panics=%d",
		heapGrowthMB, metrics.L1Size, metrics.L1Evictions, panicCount)
}
