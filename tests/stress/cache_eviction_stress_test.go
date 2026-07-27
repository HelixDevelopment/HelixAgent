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

// TestStress_Cache_LargeCapacityFill verifies that filling a large cache
// (10,000 entries) completes without panics or deadlocks, and that the
// reported cache size stays at or below the configured maximum.
func TestStress_Cache_LargeCapacityFill(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	const maxEntries = 10_000
	config := &cache.TieredCacheConfig{
		L1MaxSize: maxEntries,
		L1TTL:     5 * time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	ctx := context.Background()
	var panicCount int64

	func() {
		defer func() {
			if r := recover(); r != nil {
				atomic.AddInt64(&panicCount, 1)
			}
		}()

		for i := 0; i < maxEntries; i++ {
			key := fmt.Sprintf("large-fill-key-%d", i)
			value := fmt.Sprintf("value-%d-padding-%s", i, make([]byte, 64))
			tc.Set(ctx, key, value, 5*time.Minute)
		}
	}()

	assert.Zero(t, panicCount, "no panics during large cache fill")

	metrics := tc.Metrics()
	assert.LessOrEqual(t, metrics.L1Size, int64(maxEntries),
		"L1 cache size must not exceed configured maximum")

	t.Logf("Large capacity fill: L1_size=%d, L1_evictions=%d, panics=%d",
		metrics.L1Size, metrics.L1Evictions, panicCount)
}

// TestStress_Cache_ConcurrentReadWriteAtCapacity fills the cache to capacity
// then exercises 20 goroutines performing concurrent reads and writes while
// the cache is at or near maximum size. Validates that eviction under
// concurrent access does not cause panics, races, or unbounded growth.
func TestStress_Cache_ConcurrentReadWriteAtCapacity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	const maxEntries = 500
	config := &cache.TieredCacheConfig{
		L1MaxSize: maxEntries,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	ctx := context.Background()

	// Pre-fill cache to capacity
	for i := 0; i < maxEntries; i++ {
		tc.Set(ctx, fmt.Sprintf("prefill-%d", i), i, time.Minute)
	}

	initialMetrics := tc.Metrics()
	assert.LessOrEqual(t, initialMetrics.L1Size, int64(maxEntries),
		"pre-fill should not exceed max size")

	const goroutineCount = 20
	const opsPerGoroutine = 200

	var wg sync.WaitGroup
	var panicCount int64
	var writes, reads int64

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

			for j := 0; j < opsPerGoroutine; j++ {
				// Use overlapping key space to ensure evictions happen
				key := fmt.Sprintf("concurrent-capacity-%d", (id*opsPerGoroutine+j)%600)

				if j%3 == 0 {
					// Write new or overwrite existing
					tc.Set(ctx, key, id*j, time.Minute)
					atomic.AddInt64(&writes, 1)
				} else {
					// Read (may miss due to eviction)
					var result int
					tc.Get(ctx, key, &result)
					atomic.AddInt64(&reads, 1)
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
		t.Fatal("DEADLOCK DETECTED: concurrent read/write at capacity timed out")
	}

	finalMetrics := tc.Metrics()

	assert.Zero(t, panicCount, "no panics during concurrent read/write at capacity")
	assert.LessOrEqual(t, finalMetrics.L1Size, int64(maxEntries),
		"L1 size must stay bounded after concurrent access at capacity")

	t.Logf("Concurrent at capacity: writes=%d, reads=%d, "+
		"L1_size=%d (max=%d), evictions=%d, panics=%d",
		writes, reads, finalMetrics.L1Size, maxEntries,
		finalMetrics.L1Evictions, panicCount)
}

// TestStress_Cache_EvictionLatencyBounded measures eviction latency by timing
// individual Set operations that trigger evictions (cache is at capacity).
// Verifies that no single eviction operation takes longer than 50ms, which
// would indicate lock contention or O(n) eviction complexity.
func TestStress_Cache_EvictionLatencyBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	const maxEntries = 200
	config := &cache.TieredCacheConfig{
		L1MaxSize: maxEntries,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	ctx := context.Background()

	// Fill cache to capacity
	for i := 0; i < maxEntries; i++ {
		tc.Set(ctx, fmt.Sprintf("lat-prefill-%d", i), i, time.Minute)
	}

	// Now measure latency of eviction-triggering writes
	const samples = 1000
	var maxLatencyNs int64
	var totalLatencyNs int64

	for i := 0; i < samples; i++ {
		key := fmt.Sprintf("lat-overflow-%d", i)
		start := time.Now()
		tc.Set(ctx, key, i, time.Minute)
		elapsed := time.Since(start).Nanoseconds()

		totalLatencyNs += elapsed
		if elapsed > atomic.LoadInt64(&maxLatencyNs) {
			atomic.StoreInt64(&maxLatencyNs, elapsed)
		}
	}

	avgLatencyMs := float64(totalLatencyNs) / float64(samples) / 1e6
	maxLatencyMs := float64(atomic.LoadInt64(&maxLatencyNs)) / 1e6

	t.Logf("Eviction latency: avg=%.3fms, max=%.3fms, samples=%d",
		avgLatencyMs, maxLatencyMs, samples)

	assert.Less(t, maxLatencyMs, 50.0,
		"individual eviction should complete within 50ms")
	assert.Less(t, avgLatencyMs, 5.0,
		"average eviction latency should be under 5ms")

	// Verify size stayed bounded throughout
	finalMetrics := tc.Metrics()
	assert.LessOrEqual(t, finalMetrics.L1Size, int64(maxEntries),
		"cache size must stay bounded after overflow writes")
}

// TestStress_Cache_SizeBoundedUnderOverflow tests that when writes significantly
// exceed capacity, the cache does not grow indefinitely. Writes 5x more entries
// than capacity and verifies the reported size stays within bounds.
func TestStress_Cache_SizeBoundedUnderOverflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	const maxEntries = 100
	config := &cache.TieredCacheConfig{
		L1MaxSize: maxEntries,
		L1TTL:     time.Minute,
		EnableL1:  true,
		EnableL2:  false,
	}
	tc := cache.NewTieredCache(nil, config)
	defer tc.Close()

	ctx := context.Background()
	var panicCount int64

	const writes = maxEntries * 5
	const goroutineCount = 10

	var wg sync.WaitGroup
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

			for j := 0; j < writes/goroutineCount; j++ {
				key := fmt.Sprintf("overflow-%d-%d", id, j)
				tc.Set(ctx, key, id*j, time.Minute)
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
		t.Fatal("DEADLOCK DETECTED: cache overflow test timed out")
	}

	metrics := tc.Metrics()

	assert.Zero(t, panicCount, "no panics during cache overflow writes")
	assert.LessOrEqual(t, metrics.L1Size, int64(maxEntries),
		"cache size must not exceed configured maximum even after 5x overflow")
	assert.Greater(t, metrics.L1Evictions, int64(0),
		"evictions should have occurred when writing 5x capacity")

	t.Logf("Size bounded under overflow: L1_size=%d (max=%d), "+
		"evictions=%d, total_writes=%d, panics=%d",
		metrics.L1Size, maxEntries, metrics.L1Evictions, writes, panicCount)
}
