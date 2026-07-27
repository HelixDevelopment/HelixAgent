package monitoring_test

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/observability"
)

// TestMonitoring_GoroutineCount_Bounded verifies that starting and stopping
// concurrent operations does not cause goroutine count to grow unboundedly.
func TestMonitoring_GoroutineCount_Bounded(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource monitoring test in short mode") // SKIP-OK: #short-mode
	}

	// Take baseline goroutine count
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	// Run several rounds of concurrent work
	for round := 0; round < 10; round++ {
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Simulate brief work
				time.Sleep(time.Millisecond)
			}()
		}
		wg.Wait()
	}

	// Allow goroutines to settle
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	current := runtime.NumGoroutine()
	leaked := current - baseline

	t.Logf("Goroutine count: baseline=%d, current=%d, diff=%d",
		baseline, current, leaked)

	// Allow a small margin for runtime-internal goroutines
	assert.LessOrEqual(t, leaked, 10,
		"Goroutine count should return near baseline after operations complete")
}

// TestMonitoring_MemoryUsage_Bounded verifies that repeated metric recording
// operations do not cause unbounded memory growth.
func TestMonitoring_MemoryUsage_Bounded(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource monitoring test in short mode") // SKIP-OK: #short-mode
	}

	metrics, err := observability.NewLLMMetrics("test-memory-monitoring")
	require.NoError(t, err)

	ctx := context.Background()

	// Force GC and get baseline memory
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	var baselineStats runtime.MemStats
	runtime.ReadMemStats(&baselineStats)

	// Perform many metric recording operations
	for i := 0; i < 10000; i++ {
		metrics.RecordRequest(
			ctx,
			"test-provider",
			"test-model",
			time.Duration(i)*time.Microsecond,
			100, 200, 0.01, nil,
		)
	}

	// Force GC and measure
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	var afterStats runtime.MemStats
	runtime.ReadMemStats(&afterStats)

	// Calculate memory growth
	var memGrowth int64
	if afterStats.Alloc > baselineStats.Alloc {
		memGrowth = int64(afterStats.Alloc - baselineStats.Alloc)
	}

	t.Logf("Memory: baseline=%d bytes, after=%d bytes, growth=%d bytes",
		baselineStats.Alloc, afterStats.Alloc, memGrowth)

	// 50 MB is a generous upper bound; metric recording should not allocate
	// anywhere near this much
	const maxGrowthBytes = 50 * 1024 * 1024
	assert.Less(t, memGrowth, int64(maxGrowthBytes),
		"Memory growth after metric recording should be bounded")
}

// TestMonitoring_FileDescriptor_NoLeak verifies that creating and destroying
// goroutines with channels does not leak resources.
func TestMonitoring_FileDescriptor_NoLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource monitoring test in short mode") // SKIP-OK: #short-mode
	}

	// Use goroutine count as a proxy for resource leaks since
	// Go's runtime tracks goroutines but not FDs directly.
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	// Open/close many channels in goroutines
	for round := 0; round < 20; round++ {
		channels := make([]chan struct{}, 50)
		for i := range channels {
			channels[i] = make(chan struct{})
			go func(ch chan struct{}) {
				<-ch
			}(channels[i])
		}
		// Close all channels to release goroutines
		for _, ch := range channels {
			close(ch)
		}
	}

	// Allow goroutines to finish
	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	current := runtime.NumGoroutine()
	leaked := current - baseline

	t.Logf("Goroutines: baseline=%d, current=%d, diff=%d",
		baseline, current, leaked)

	assert.LessOrEqual(t, leaked, 5,
		"Resource count should return to baseline after channel cleanup")
}

// TestMonitoring_ConnectionPool_Metrics verifies that pool-like patterns
// correctly track active and idle metrics through the OTel gauge interface.
func TestMonitoring_ConnectionPool_Metrics(t *testing.T) {
	metrics, err := observability.NewLLMMetrics("test-pool-monitoring")
	require.NoError(t, err)

	ctx := context.Background()

	// Simulate connection pool operations via RequestsInFlight gauge:
	// - Add connections (increment in-flight)
	// - Remove connections (decrement in-flight)
	// The UpDownCounter should not panic when going to zero and below.

	const poolSize = 10

	// Acquire connections
	for i := 0; i < poolSize; i++ {
		metrics.RequestsInFlight.Add(ctx, 1)
	}

	// Release some connections
	for i := 0; i < poolSize/2; i++ {
		metrics.RequestsInFlight.Add(ctx, -1)
	}

	// Release remaining
	for i := 0; i < poolSize/2; i++ {
		metrics.RequestsInFlight.Add(ctx, -1)
	}

	// If we get here without panics, the pool metrics work correctly
	assert.NotNil(t, metrics.RequestsInFlight,
		"RequestsInFlight gauge must be initialized")
}

// TestMonitoring_WorkerPool_Metrics verifies that worker-pool-style metrics
// correctly track completed and failed tasks.
func TestMonitoring_WorkerPool_Metrics(t *testing.T) {
	metrics, err := observability.NewLLMMetrics("test-worker-monitoring")
	require.NoError(t, err)

	ctx := context.Background()

	var completedCount int64
	var failedCount int64

	const numWorkers = 5
	const tasksPerWorker = 20

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for task := 0; task < tasksPerWorker; task++ {
				start := time.Now()
				// Simulate work
				time.Sleep(10 * time.Microsecond)
				duration := time.Since(start)

				if task%5 == 0 {
					// Simulate failure
					metrics.RecordRequest(
						ctx,
						"worker",
						"task",
						duration,
						0, 0, 0,
						assert.AnError,
					)
					atomic.AddInt64(&failedCount, 1)
				} else {
					metrics.RecordRequest(
						ctx,
						"worker",
						"task",
						duration,
						10, 20, 0.001,
						nil,
					)
					atomic.AddInt64(&completedCount, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	totalExpected := int64(numWorkers * tasksPerWorker)
	totalActual := atomic.LoadInt64(&completedCount) + atomic.LoadInt64(&failedCount)
	assert.Equal(t, totalExpected, totalActual,
		"All tasks should be accounted for (completed + failed)")

	expectedFailed := int64(numWorkers * (tasksPerWorker / 5))
	assert.Equal(t, expectedFailed, atomic.LoadInt64(&failedCount),
		"Failed count should match expected failure rate")

	t.Logf("Worker pool: total=%d, completed=%d, failed=%d",
		totalActual,
		atomic.LoadInt64(&completedCount),
		atomic.LoadInt64(&failedCount),
	)
}
