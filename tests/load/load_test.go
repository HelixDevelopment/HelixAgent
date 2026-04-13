package load

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// loadMetrics tracks per-test metrics for load analysis.
type loadMetrics struct {
	totalRequests  int64
	successCount   int64
	failureCount   int64
	rejectedCount  int64
	totalLatencyNs int64
	maxLatencyNs   int64
	minLatencyNs   int64
	goroutinePeak  int64
	memAllocBytes  int64
}

func (m *loadMetrics) recordSuccess(latency time.Duration) {
	atomic.AddInt64(&m.totalRequests, 1)
	atomic.AddInt64(&m.successCount, 1)
	ns := latency.Nanoseconds()
	atomic.AddInt64(&m.totalLatencyNs, ns)
	for {
		cur := atomic.LoadInt64(&m.maxLatencyNs)
		if ns <= cur || atomic.CompareAndSwapInt64(&m.maxLatencyNs, cur, ns) {
			break
		}
	}
}

func (m *loadMetrics) recordFailure() {
	atomic.AddInt64(&m.totalRequests, 1)
	atomic.AddInt64(&m.failureCount, 1)
}

func (m *loadMetrics) recordRejected() {
	atomic.AddInt64(&m.totalRequests, 1)
	atomic.AddInt64(&m.rejectedCount, 1)
}

func (m *loadMetrics) avgLatency() time.Duration {
	total := atomic.LoadInt64(&m.totalLatencyNs)
	count := atomic.LoadInt64(&m.successCount)
	if count == 0 {
		return 0
	}
	return time.Duration(total / count)
}

func (m *loadMetrics) report(t *testing.T) {
	t.Logf("Load Test Results:")
	t.Logf("  Total requests:  %d", atomic.LoadInt64(&m.totalRequests))
	t.Logf("  Successful:      %d", atomic.LoadInt64(&m.successCount))
	t.Logf("  Failed:          %d", atomic.LoadInt64(&m.failureCount))
	t.Logf("  Rejected (503):  %d", atomic.LoadInt64(&m.rejectedCount))
	t.Logf("  Avg latency:     %v", m.avgLatency())
	t.Logf("  Max latency:     %v", time.Duration(atomic.LoadInt64(&m.maxLatencyNs)))
	t.Logf("  Goroutine peak:  %d", atomic.LoadInt64(&m.goroutinePeak))
}

// newTestRouter creates a Gin router with concurrency limiter and a handler
// that simulates work with the given latency.
func newTestRouter(maxInFlight int, handlerLatency time.Duration) *gin.Engine {
	r := gin.New()
	r.Use(middleware.ConcurrencyLimiter(maxInFlight))
	r.GET("/api/test", func(c *gin.Context) {
		time.Sleep(handlerLatency)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

// TestLoad_SustainedConstantRate sends requests at a constant rate for a fixed
// duration and validates that the server maintains acceptable latency and
// error rate under sustained load.
func TestLoad_SustainedConstantRate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	router := newTestRouter(50, 5*time.Millisecond)
	metrics := &loadMetrics{}

	concurrency := 10
	duration := 5 * time.Second
	deadline := time.Now().Add(duration)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				start := time.Now()
				req := httptest.NewRequest("GET", "/api/test", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				latency := time.Since(start)

				switch w.Code {
				case http.StatusOK:
					metrics.recordSuccess(latency)
				case http.StatusServiceUnavailable:
					metrics.recordRejected()
				default:
					metrics.recordFailure()
				}

				// Track goroutine count
				gr := int64(runtime.NumGoroutine())
				for {
					cur := atomic.LoadInt64(&metrics.goroutinePeak)
					if gr <= cur || atomic.CompareAndSwapInt64(&metrics.goroutinePeak, cur, gr) {
						break
					}
				}
			}
		}()
	}
	wg.Wait()
	metrics.report(t)

	total := atomic.LoadInt64(&metrics.totalRequests)
	require.Greater(t, total, int64(0), "should have processed requests")
	successRate := float64(atomic.LoadInt64(&metrics.successCount)) / float64(atomic.LoadInt64(&metrics.totalRequests))
	assert.Greater(t, successRate, 0.95, "success rate should be > 95%% under sustained load")
	assert.Less(t, metrics.avgLatency(), 100*time.Millisecond, "avg latency should be < 100ms")
}

// TestLoad_SpikeTraffic simulates a sudden 10x traffic spike and validates
// that the server handles it gracefully — rejects excess requests with 503
// rather than crashing or hanging.
func TestLoad_SpikeTraffic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	// Low concurrency limit to force rejections during spike
	router := newTestRouter(10, 10*time.Millisecond)
	metrics := &loadMetrics{}

	// Phase 1: Normal load (5 concurrent for 2s)
	normalDuration := 2 * time.Second
	normalDeadline := time.Now().Add(normalDuration)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(normalDeadline) {
				req := httptest.NewRequest("GET", "/api/test", nil)
				w := httptest.NewRecorder()
				start := time.Now()
				router.ServeHTTP(w, req)
				if w.Code == http.StatusOK {
					metrics.recordSuccess(time.Since(start))
				} else {
					metrics.recordRejected()
				}
			}
		}()
	}
	wg.Wait()

	normalSuccess := atomic.LoadInt64(&metrics.successCount)
	t.Logf("Phase 1 (normal): %d successful requests", normalSuccess)

	// Phase 2: Spike — 50 concurrent goroutines for 2s
	spikeMetrics := &loadMetrics{}
	spikeDeadline := time.Now().Add(2 * time.Second)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(spikeDeadline) {
				req := httptest.NewRequest("GET", "/api/test", nil)
				w := httptest.NewRecorder()
				start := time.Now()
				router.ServeHTTP(w, req)
				switch w.Code {
				case http.StatusOK:
					spikeMetrics.recordSuccess(time.Since(start))
				case http.StatusServiceUnavailable:
					spikeMetrics.recordRejected()
				default:
					spikeMetrics.recordFailure()
				}
			}
		}()
	}
	wg.Wait()
	spikeMetrics.report(t)

	// During spike, some requests should be rejected (503) but no failures (5xx other than 503)
	assert.Equal(t, int64(0), atomic.LoadInt64(&spikeMetrics.failureCount),
		"no unexpected failures during spike — only 503 rejections expected")
	assert.Greater(t, atomic.LoadInt64(&spikeMetrics.rejectedCount), int64(0),
		"concurrency limiter should reject some requests during spike")
	assert.Greater(t, atomic.LoadInt64(&spikeMetrics.successCount), int64(0),
		"some requests should still succeed during spike")
}

// TestLoad_SoakTest runs a low-rate, extended test checking for resource leaks
// (goroutine accumulation, memory growth).
func TestLoad_SoakTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	router := newTestRouter(50, 1*time.Millisecond)

	// Record initial state
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	initialGoroutines := runtime.NumGoroutine()

	// Run low-rate requests for 5 seconds
	duration := 5 * time.Second
	deadline := time.Now().Add(duration)
	requestCount := int64(0)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				req := httptest.NewRequest("GET", "/api/test", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				atomic.AddInt64(&requestCount, 1)
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}
	wg.Wait()

	// Record final state
	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	finalGoroutines := runtime.NumGoroutine()

	t.Logf("Soak Test Results:")
	t.Logf("  Requests processed: %d", requestCount)
	t.Logf("  Initial goroutines: %d", initialGoroutines)
	t.Logf("  Final goroutines:   %d", finalGoroutines)
	t.Logf("  Initial alloc:      %d MB", initialMem.Alloc/1024/1024)
	t.Logf("  Final alloc:        %d MB", finalMem.Alloc/1024/1024)

	goroutineDelta := finalGoroutines - initialGoroutines
	assert.LessOrEqual(t, goroutineDelta, 5,
		"goroutine count should not grow significantly — possible leak if delta > 5")
	assert.Greater(t, requestCount, int64(100),
		"should process a reasonable number of requests in 5 seconds")
}

// TestLoad_GoroutineLeakDetection verifies that processing many requests
// does not leak goroutines.
func TestLoad_GoroutineLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	router := newTestRouter(20, 1*time.Millisecond)

	// Warmup
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	runtime.GC()
	baseGoroutines := runtime.NumGoroutine()

	// Send many requests
	var wg sync.WaitGroup
	for batch := 0; batch < 5; batch++ {
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest("GET", "/api/test", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}()
		}
		wg.Wait()
	}

	// Let goroutines settle
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	finalGoroutines := runtime.NumGoroutine()

	t.Logf("Base goroutines: %d, Final: %d, Delta: %d",
		baseGoroutines, finalGoroutines, finalGoroutines-baseGoroutines)

	assert.LessOrEqual(t, finalGoroutines-baseGoroutines, 5,
		fmt.Sprintf("goroutine leak detected: base=%d final=%d", baseGoroutines, finalGoroutines))
}
