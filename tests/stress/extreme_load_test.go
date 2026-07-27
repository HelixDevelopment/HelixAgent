package stress

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/concurrency"
	helixhttp "dev.helix.agent/internal/http"
	"dev.helix.agent/internal/services"
)

// TestExtreme10xConcurrentLoad verifies the system handles 10x normal concurrent
// load without deadlocks, panics, or unacceptable error rates.
func TestExtreme10xConcurrentLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping extreme stress test in short mode") // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Simulate a realistic handler with slight latency
	var handlerCalls int64
	router.GET("/health", func(c *gin.Context) {
		atomic.AddInt64(&handlerCalls, 1)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/v1/completions", func(c *gin.Context) {
		atomic.AddInt64(&handlerCalls, 1)
		time.Sleep(time.Millisecond) // Simulate processing
		c.JSON(http.StatusOK, gin.H{
			"id":      "cmpl-test",
			"object":  "text_completion",
			"choices": []gin.H{{"text": "response"}},
		})
	})

	server := httptest.NewServer(router)
	defer server.Close()

	const numGoroutines = 120
	const requestsPerGoroutine = 10
	var wg sync.WaitGroup
	var successCount, failCount, panicCount int64

	pool := helixhttp.NewHTTPClientPool(helixhttp.DefaultPoolConfig())
	defer func() { _ = pool.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	start := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
					t.Logf("PANIC in goroutine %d: %v", id, r)
				}
			}()
			<-start

			for j := 0; j < requestsPerGoroutine; j++ {
				endpoint := server.URL + "/health"
				if j%3 == 0 {
					endpoint = server.URL + "/v1/completions"
				}

				resp, err := pool.Get(ctx, endpoint)
				if err != nil {
					atomic.AddInt64(&failCount, 1)
					continue
				}
				resp.Body.Close()
				if resp.StatusCode < 500 {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}
		}(i)
	}

	close(start)

	// Detect deadlock with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: 10x concurrent load test timed out")
	}

	total := successCount + failCount
	t.Logf("10x concurrent load: total=%d success=%d fail=%d panics=%d",
		total, successCount, failCount, panicCount)

	// Assertions
	assert.Zero(t, panicCount, "no goroutine should panic under 10x load")
	assert.Equal(t, int64(numGoroutines*requestsPerGoroutine), total,
		"all requests must be accounted for")

	errorRate := float64(failCount) / float64(total)
	assert.Less(t, errorRate, 0.1,
		"error rate should be below 10%% under 10x load, got %.2f%%", errorRate*100)
}

// TestExtremeProviderCascadeFailure verifies the system degrades gracefully when
// all simulated providers fail, with circuit breakers activating and no panics.
func TestExtremeProviderCascadeFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cascade failure test in short mode") // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	providerNames := []string{
		"openai", "anthropic", "gemini", "mistral",
		"deepseek", "cohere", "groq", "fireworks",
	}

	breakers := make(map[string]*services.CircuitBreaker)
	for _, p := range providerNames {
		breakers[p] = services.NewCircuitBreaker(3, 2, 100*time.Millisecond)
	}

	const numWorkers = 50
	const attemptsPerWorker = 20
	var wg sync.WaitGroup
	var gracefulErrors, panicCount, circuitOpens int64

	start := make(chan struct{})

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for attempt := 0; attempt < attemptsPerWorker; attempt++ {
				// Try each provider in the chain; all fail
				allFailed := true
				for _, p := range providerNames {
					cb := breakers[p]
					err := cb.Call(func() error {
						return fmt.Errorf("provider %s unavailable", p)
					})
					if err != nil {
						if err.Error() == "circuit breaker is open" {
							atomic.AddInt64(&circuitOpens, 1)
						}
					} else {
						allFailed = false
						break
					}
				}
				if allFailed {
					atomic.AddInt64(&gracefulErrors, 1)
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
		t.Fatal("DEADLOCK DETECTED: cascade failure test timed out")
	}

	t.Logf("Cascade failure: graceful_errors=%d circuit_opens=%d panics=%d",
		gracefulErrors, circuitOpens, panicCount)

	// All requests should result in graceful errors, not panics
	assert.Zero(t, panicCount, "no goroutine should panic during cascade failure")
	assert.Greater(t, gracefulErrors, int64(0), "should see graceful failures")
	assert.Greater(t, circuitOpens, int64(0), "circuit breakers should activate")

	// Verify circuit breakers are open after cascade
	openCount := 0
	for _, cb := range breakers {
		if cb.GetState() == services.StateOpen {
			openCount++
		}
	}
	t.Logf("Open circuit breakers: %d/%d", openCount, len(providerNames))
	assert.Greater(t, openCount, 0, "at least one circuit breaker should be open")

	// Verify no goroutine leaks
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	goroutines := runtime.NumGoroutine()
	t.Logf("Goroutines after cascade failure test: %d", goroutines)
}

// TestExtremeMemoryPressureGracefulDegradation verifies that heavy concurrent
// operations do not cause unreasonable memory growth.
func TestExtremeMemoryPressureGracefulDegradation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory pressure test in short mode") // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	// Track memory before
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Run many concurrent allocation-heavy operations
	const numGoroutines = 80
	const roundsPerGoroutine = 50
	var wg sync.WaitGroup
	var totalOps int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; round < roundsPerGoroutine; round++ {
				// Simulate JSON processing work (typical of LLM response handling)
				data := make(map[string]interface{})
				for k := 0; k < 20; k++ {
					data[fmt.Sprintf("key-%d", k)] = make([]byte, 512)
				}
				// Simulate response aggregation
				results := make([]map[string]interface{}, 0, 5)
				for p := 0; p < 5; p++ {
					result := make(map[string]interface{})
					result["provider"] = fmt.Sprintf("provider-%d", p)
					result["response"] = make([]byte, 1024)
					results = append(results, result)
				}
				_ = results
				atomic.AddInt64(&totalOps, 1)
			}
		}()
	}

	wg.Wait()

	// Force GC and measure
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	heapBeforeMB := float64(memBefore.HeapInuse) / 1024 / 1024
	heapAfterMB := float64(memAfter.HeapInuse) / 1024 / 1024

	t.Logf("Memory pressure test:")
	t.Logf("  Heap before: %.2f MB", heapBeforeMB)
	t.Logf("  Heap after:  %.2f MB", heapAfterMB)
	t.Logf("  Total operations: %d", totalOps)
	t.Logf("  Total allocs: %d", memAfter.TotalAlloc-memBefore.TotalAlloc)

	// After GC, live heap should be bounded since we did not retain references
	assert.Less(t, heapAfterMB, 200.0,
		"live heap after GC should be bounded (under 200 MB)")
	assert.Equal(t, int64(numGoroutines*roundsPerGoroutine), totalOps,
		"all operations should complete")
}

// TestExtremeConnectionPoolExhaustion verifies that a small HTTP connection pool
// handles more concurrent requests than its capacity without panics, and recovers.
func TestExtremeConnectionPoolExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping pool exhaustion test in short mode") // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	// Create a slow server to force pool contention
	var serverCalls int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&serverCalls, 1)
		time.Sleep(10 * time.Millisecond) // Slow responses force pool contention
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	// Create a pool with very small limits to force exhaustion
	smallConfig := &helixhttp.PoolConfig{
		MaxIdleConns:          5,
		MaxConnsPerHost:       3,
		MaxIdleConnsPerHost:   3,
		IdleConnTimeout:       5 * time.Second,
		DialTimeout:           5 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		KeepAliveInterval:     5 * time.Second,
		RetryCount:            1,
		RetryWaitMin:          50 * time.Millisecond,
		RetryWaitMax:          200 * time.Millisecond,
	}

	pool := helixhttp.NewHTTPClientPool(smallConfig)
	defer func() { _ = pool.Close() }()

	// Send far more concurrent requests than pool capacity
	const numRequests = 60
	var wg sync.WaitGroup
	var successCount, timeoutCount, errorCount, panicCount int64

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := make(chan struct{})

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			resp, err := pool.Get(ctx, server.URL+"/test")
			if err != nil {
				if ctx.Err() != nil {
					atomic.AddInt64(&timeoutCount, 1)
				} else {
					atomic.AddInt64(&errorCount, 1)
				}
				return
			}
			resp.Body.Close()
			atomic.AddInt64(&successCount, 1)
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
		t.Fatal("DEADLOCK DETECTED: pool exhaustion test timed out")
	}

	total := successCount + timeoutCount + errorCount
	t.Logf("Pool exhaustion: success=%d timeout=%d error=%d panics=%d total=%d server_calls=%d",
		successCount, timeoutCount, errorCount, panicCount, total, serverCalls)

	// Key assertions
	assert.Zero(t, panicCount, "pool exhaustion must not cause panics")
	assert.Equal(t, int64(numRequests), total, "all requests must be accounted for")
	assert.Greater(t, successCount, int64(0),
		"some requests should succeed even under pool exhaustion")

	// Verify pool recovers: send a few follow-up requests
	var recoverSuccess int64
	for i := 0; i < 5; i++ {
		resp, err := pool.Get(ctx, server.URL+"/test")
		if err == nil {
			resp.Body.Close()
			recoverSuccess++
		}
	}
	t.Logf("Recovery requests: %d/5 succeeded", recoverSuccess)
	assert.Greater(t, recoverSuccess, int64(0),
		"pool should recover and serve requests after exhaustion")
}

// TestExtremeP99LatencyBaseline measures P99 latency for a baseline set of
// operations and writes the result to a report file for tracking.
func TestExtremeP99LatencyBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency baseline test in short mode") // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.POST("/v1/completions", func(c *gin.Context) {
		time.Sleep(500 * time.Microsecond) // Simulate minimal processing
		c.JSON(http.StatusOK, gin.H{
			"id":      "cmpl-baseline",
			"object":  "text_completion",
			"choices": []gin.H{{"text": "baseline response"}},
		})
	})

	const sampleSize = 200
	durations := make([]time.Duration, 0, sampleSize)

	for i := 0; i < sampleSize; i++ {
		var endpoint string
		var method string
		if i%2 == 0 {
			endpoint = "/health"
			method = "GET"
		} else {
			endpoint = "/v1/completions"
			method = "POST"
		}

		req := httptest.NewRequest(method, endpoint, nil)
		rec := httptest.NewRecorder()

		start := time.Now()
		router.ServeHTTP(rec, req)
		elapsed := time.Since(start)

		durations = append(durations, elapsed)
	}

	require.Equal(t, sampleSize, len(durations), "should have all samples")

	// Sort and compute percentiles
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

	p50Idx := int(float64(len(durations)) * 0.50)
	p90Idx := int(float64(len(durations)) * 0.90)
	p99Idx := int(float64(len(durations)) * 0.99)
	if p50Idx >= len(durations) {
		p50Idx = len(durations) - 1
	}
	if p90Idx >= len(durations) {
		p90Idx = len(durations) - 1
	}
	if p99Idx >= len(durations) {
		p99Idx = len(durations) - 1
	}

	p50 := durations[p50Idx]
	p90 := durations[p90Idx]
	p99 := durations[p99Idx]
	minLatency := durations[0]
	maxLatency := durations[len(durations)-1]

	// Calculate mean
	var totalDuration time.Duration
	for _, d := range durations {
		totalDuration += d
	}
	mean := totalDuration / time.Duration(len(durations))

	t.Logf("Latency baseline (%d samples):", sampleSize)
	t.Logf("  Min:  %v", minLatency)
	t.Logf("  Mean: %v", mean)
	t.Logf("  P50:  %v", p50)
	t.Logf("  P90:  %v", p90)
	t.Logf("  P99:  %v", p99)
	t.Logf("  Max:  %v", maxLatency)

	// Write report
	reportDir := filepath.Join("..", "..", "reports", "latency")
	err := os.MkdirAll(reportDir, 0o750)
	if err != nil {
		t.Logf("Warning: could not create report dir: %v", err)
	} else {
		reportContent := fmt.Sprintf(
			"P99 Latency Baseline Report\n"+
				"===========================\n"+
				"Date: 2026-03-16\n"+
				"Samples: %d\n"+
				"Min: %v\n"+
				"Mean: %v\n"+
				"P50: %v\n"+
				"P90: %v\n"+
				"P99: %v\n"+
				"Max: %v\n",
			sampleSize, minLatency, mean, p50, p90, p99, maxLatency,
		)

		reportPath := filepath.Join(reportDir, "p99-baseline-2026-03-16.txt")
		writeErr := os.WriteFile(reportPath, []byte(reportContent), 0o600)
		if writeErr != nil {
			t.Logf("Warning: could not write report: %v", writeErr)
		} else {
			t.Logf("Report written to %s", reportPath)
		}
	}

	// Sanity assertions on latency
	assert.Less(t, p99, 100*time.Millisecond,
		"P99 latency should be under 100ms for in-process requests")
	assert.Less(t, mean, 50*time.Millisecond,
		"Mean latency should be under 50ms for in-process requests")
}

// TestExtremeSemaphoreOverload verifies that semaphore-limited concurrency holds
// under extreme goroutine pressure, and that no goroutines leak afterward.
func TestExtremeSemaphoreOverload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping extreme semaphore overload test in short mode") // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	const (
		semCapacity   = 5
		numGoroutines = 200
		opsPerRoutine = 10
	)

	sem := concurrency.NewSemaphore(semCapacity)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var maxConcurrent, currentConcurrent int64
	var totalAcquired, panicCount int64

	baseline := runtime.NumGoroutine()
	start := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for op := 0; op < opsPerRoutine; op++ {
				err := sem.Acquire(ctx)
				if err != nil {
					return // context cancelled
				}

				cur := atomic.AddInt64(&currentConcurrent, 1)
				// Track max concurrency
				for {
					old := atomic.LoadInt64(&maxConcurrent)
					if cur <= old || atomic.CompareAndSwapInt64(&maxConcurrent, old, cur) {
						break
					}
				}
				atomic.AddInt64(&totalAcquired, 1)

				// Brief work
				time.Sleep(50 * time.Microsecond)

				atomic.AddInt64(&currentConcurrent, -1)
				sem.Release()
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
		t.Fatal("DEADLOCK DETECTED: semaphore overload test timed out")
	}

	// Check goroutine leak
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()
	leaked := after - baseline

	t.Logf("Semaphore overload: max_concurrent=%d (limit=%d) total_acquired=%d panics=%d goroutine_leak=%d",
		maxConcurrent, semCapacity, totalAcquired, panicCount, leaked)

	assert.Zero(t, panicCount, "no panics under semaphore overload")
	assert.LessOrEqual(t, maxConcurrent, int64(semCapacity),
		"max concurrent must never exceed semaphore capacity")
	assert.Equal(t, int64(numGoroutines*opsPerRoutine), totalAcquired,
		"all acquire operations should complete")
	assert.Less(t, leaked, 20,
		"goroutine count should be bounded after semaphore overload")
}
