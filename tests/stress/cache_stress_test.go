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

// TestCache_HighConcurrentAccess exercises the TieredCache with 500 goroutines
// performing concurrent Set, Get, and Delete operations. This verifies that
// the cache's internal locking prevents panics and data corruption.
func TestCache_HighConcurrentAccess(t *testing.T) {
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

	const goroutineCount = 500
	ctx := context.Background()

	var wg sync.WaitGroup
	var panics int64
	var sets int64
	var gets int64
	var deletes int64

	startSignal := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-startSignal

			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key-%d-%d", id%50, j%20)

				switch j % 3 {
				case 0: // Set
					tc.Set(ctx, key, fmt.Sprintf("value-%d-%d", id, j), time.Minute)
					atomic.AddInt64(&sets, 1)

				case 1: // Get
					var result string
					tc.Get(ctx, key, &result)
					atomic.AddInt64(&gets, 1)

				case 2: // Delete
					tc.Delete(ctx, key)
					atomic.AddInt64(&deletes, 1)
				}
			}
		}(i)
	}

	close(startSignal)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: high concurrent cache access timed out")
	}

	assert.Zero(t, panics,
		"no goroutine should panic under 500-goroutine cache access")

	totalOps := sets + gets + deletes
	t.Logf("High concurrent access: sets=%d, gets=%d, deletes=%d, "+
		"total=%d, panics=%d",
		sets, gets, deletes, totalOps, panics)

	assert.Equal(t, int64(goroutineCount*100), totalOps,
		"all operations should complete")
}

// TestCache_EvictionUnderPressure tests that the cache correctly evicts
// entries when the L1 max size is exceeded, and does so without panics
// or deadlocks under concurrent access.
func TestCache_EvictionUnderPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	// Small cache to force frequent evictions
	config := &cache.TieredCacheConfig{
		L1MaxSize: 100,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	const goroutineCount = 100
	ctx := context.Background()

	var wg sync.WaitGroup
	var panics int64
	var totalSets int64

	startSignal := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-startSignal

			// Each goroutine writes 50 unique keys, far exceeding the
			// cache capacity of 100 total
			for j := 0; j < 50; j++ {
				key := fmt.Sprintf("evict-%d-%d", id, j)
				tc.Set(ctx, key, id*1000+j, time.Minute)
				atomic.AddInt64(&totalSets, 1)

				// Interleave reads to ensure eviction doesn't break reads
				var result int
				tc.Get(ctx, key, &result)
			}
		}(i)
	}

	close(startSignal)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: cache eviction under pressure timed out")
	}

	assert.Zero(t, panics,
		"no goroutine should panic during eviction pressure")

	// The cache should have metrics reflecting evictions
	metrics := tc.Metrics()
	t.Logf("Eviction under pressure: total_sets=%d, L1_size=%d, "+
		"L1_evictions=%d, panics=%d",
		totalSets, metrics.L1Size, metrics.L1Evictions, panics)

	// Cache size should be at or below max
	assert.LessOrEqual(t, metrics.L1Size, int64(config.L1MaxSize),
		"L1 cache size should not exceed max")
}

// TestCache_NoDeadlocksUnderMixedOperations exercises the cache with
// a mix of Set, Get, Delete, InvalidateByTag, and Metrics calls from
// many goroutines to detect potential deadlocks from lock ordering issues.
func TestCache_NoDeadlocksUnderMixedOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	config := &cache.TieredCacheConfig{
		L1MaxSize: 2000,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	const goroutineCount = 200
	ctx := context.Background()

	var wg sync.WaitGroup
	var panics int64

	startSignal := make(chan struct{})

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-startSignal

			tag := fmt.Sprintf("tag-%d", id%10)

			for j := 0; j < 50; j++ {
				key := fmt.Sprintf("mixed-%d-%d", id%30, j%15)

				switch j % 5 {
				case 0: // Set with tag
					tc.Set(ctx, key, id*j, time.Minute, tag)

				case 1: // Get
					var result int
					tc.Get(ctx, key, &result)

				case 2: // Delete
					tc.Delete(ctx, key)

				case 3: // Invalidate by tag
					tc.InvalidateByTag(ctx, tag)

				case 4: // Metrics
					_ = tc.Metrics()
				}
			}
		}(i)
	}

	close(startSignal)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: mixed cache operations timed out after 30s")
	}

	assert.Zero(t, panics,
		"no goroutine should panic during mixed cache operations")
	t.Logf("No deadlocks under mixed ops: %d goroutines completed", goroutineCount)
}

// TestCache_TTLExpirationUnderConcurrency tests that TTL expiration
// works correctly when many goroutines are writing keys with short TTLs
// while others are reading. Ensures expiration doesn't cause races.
func TestCache_TTLExpirationUnderConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	config := &cache.TieredCacheConfig{
		L1MaxSize:         1000,
		L1TTL:             50 * time.Millisecond,
		L1CleanupInterval: 25 * time.Millisecond,
		EnableL1:          true,
		EnableL2:          false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	const goroutineCount = 100
	ctx := context.Background()

	var wg sync.WaitGroup
	var panics int64
	var writeOps int64
	var readOps int64

	startSignal := make(chan struct{})

	// Writers: set keys with very short TTL
	for i := 0; i < goroutineCount/2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-startSignal

			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("ttl-%d-%d", id%20, j%10)
				ttl := time.Duration(10+j%40) * time.Millisecond
				tc.Set(ctx, key, id*j, ttl)
				atomic.AddInt64(&writeOps, 1)

				// Small delay to let some keys expire between writes
				if j%10 == 0 {
					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}

	// Readers: read keys (some will be expired)
	for i := 0; i < goroutineCount/2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-startSignal

			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("ttl-%d-%d", id%20, j%10)
				var result int
				tc.Get(ctx, key, &result)
				atomic.AddInt64(&readOps, 1)
			}
		}(i)
	}

	close(startSignal)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: TTL expiration under concurrency timed out")
	}

	assert.Zero(t, panics,
		"no goroutine should panic during TTL expiration")

	t.Logf("TTL expiration under concurrency: writes=%d, reads=%d, panics=%d",
		writeOps, readOps, panics)

	// All operations should have completed
	assert.Equal(t, int64(goroutineCount/2*100), writeOps,
		"all write operations should complete")
	assert.Equal(t, int64(goroutineCount/2*100), readOps,
		"all read operations should complete")
}

// TestCache_MetricsConcurrency verifies that reading cache metrics
// concurrently with cache operations does not cause panics or races.
func TestCache_MetricsConcurrency(t *testing.T) {
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

	ctx := context.Background()
	var wg sync.WaitGroup
	var panics int64

	startSignal := make(chan struct{})

	// Cache operation goroutines
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-startSignal

			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("metric-test-%d", id*100+j)
				tc.Set(ctx, key, id*j, time.Minute)
				var val int
				tc.Get(ctx, key, &val)
			}
		}(i)
	}

	// Metrics reader goroutines
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-startSignal

			for j := 0; j < 200; j++ {
				m := tc.Metrics()
				// Access fields to ensure no nil dereference
				_ = m.L1Hits
				_ = m.L1Misses
				_ = m.L1Size
				_ = m.L1Evictions
			}
		}()
	}

	close(startSignal)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: metrics concurrency timed out")
	}

	assert.Zero(t, panics,
		"no goroutine should panic during concurrent metrics access")
}

// TestCache_NoGoroutineLeaksAfterClose verifies that creating and closing
// many TieredCache instances does not leak goroutines. This catches
// issues with background cleanup goroutines not being properly stopped.
func TestCache_NoGoroutineLeaksAfterClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	for iteration := 0; iteration < 20; iteration++ {
		config := &cache.TieredCacheConfig{
			L1MaxSize:         500,
			L1TTL:             100 * time.Millisecond,
			L1CleanupInterval: 50 * time.Millisecond,
			EnableL1:          true,
			EnableL2:          false,
		}
		tc := cache.NewTieredCache(nil, config)

		ctx := context.Background()
		for i := 0; i < 100; i++ {
			tc.Set(ctx, fmt.Sprintf("leak-key-%d", i), i, 50*time.Millisecond)
		}

		tc.Close()
	}

	runtime.GC()
	time.Sleep(500 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	leaked := finalGoroutines - initialGoroutines

	t.Logf("Cache goroutine leak check: initial=%d, final=%d, leaked=%d",
		initialGoroutines, finalGoroutines, leaked)

	assert.Less(t, leaked, 20,
		"goroutine count should not grow significantly after 20 cache create/close cycles")
}
