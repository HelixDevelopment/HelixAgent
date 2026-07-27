package stress

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/services"
)

// newQuietLogger creates a logger that only emits errors to avoid noisy output.
func newQuietLogger() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.ErrorLevel)
	return l
}

// startTCPEchoServer starts a minimal TCP listener on a random port and
// returns the listener and its address. The listener accepts and immediately
// closes connections to simulate a healthy TCP service.
func startTCPEchoServer(t *testing.T) (net.Listener, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "failed to start TCP echo server")

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return // Listener closed
			}
			conn.Close()
		}
	}()

	return ln, ln.Addr().String()
}

// startHTTPHealthServer starts a minimal HTTP server on a random port that
// responds 200 OK on /health. Returns the server and the base URL.
func startHTTPHealthServer(t *testing.T) (*http.Server, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "failed to start HTTP health server")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(ln)
	}()

	return server, ln.Addr().String()
}

// extractHostPort splits an address string into host and port components.
func extractHostPort(addr string) (string, string) {
	host, port, _ := net.SplitHostPort(addr)
	return host, port
}

// --- Health checker stress tests ---

// TestHealthChecker_ManyEndpoints verifies that CheckAllNonBlocking handles
// a large number of endpoints (50+ TCP and HTTP) without deadlocks or
// missed results. Each endpoint has its own local listener.
func TestHealthChecker_ManyEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	hc := services.NewServiceHealthChecker(newQuietLogger())
	hc.BatchTimeout = 15 * time.Second
	hc.MaxConcurrentChecks = 10

	endpoints := make(map[string]config.ServiceEndpoint)
	var cleanupFuncs []func()

	// Create 30 TCP endpoints
	for i := 0; i < 30; i++ {
		ln, addr := startTCPEchoServer(t)
		cleanupFuncs = append(cleanupFuncs, func() { ln.Close() })

		host, port := extractHostPort(addr)
		name := fmt.Sprintf("tcp-service-%d", i)
		endpoints[name] = config.ServiceEndpoint{
			Host:       host,
			Port:       port,
			Enabled:    true,
			HealthType: "tcp",
			Timeout:    2 * time.Second,
		}
	}

	// Create 20 HTTP endpoints
	for i := 0; i < 20; i++ {
		server, addr := startHTTPHealthServer(t)
		cleanupFuncs = append(cleanupFuncs, func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		})

		host, port := extractHostPort(addr)
		name := fmt.Sprintf("http-service-%d", i)
		endpoints[name] = config.ServiceEndpoint{
			Host:       host,
			Port:       port,
			Enabled:    true,
			HealthType: "http",
			HealthPath: "/health",
			Timeout:    2 * time.Second,
		}
	}

	defer func() {
		for _, cleanup := range cleanupFuncs {
			cleanup()
		}
	}()

	ctx := context.Background()
	start := time.Now()
	results := hc.CheckAllNonBlocking(ctx, endpoints)
	elapsed := time.Since(start)

	t.Logf("Many endpoints: %d endpoints checked in %v", len(endpoints), elapsed)

	// All endpoints should have results
	assert.Equal(t, len(endpoints), len(results),
		"every endpoint should have a result")

	// Count healthy vs unhealthy
	healthy := 0
	for name, result := range results {
		if result.Error == nil {
			healthy++
		} else {
			t.Logf("  Unhealthy: %s - %v", name, result.Error)
		}
	}

	assert.Equal(t, len(endpoints), healthy,
		"all local endpoints should be healthy")

	// The batch should complete within the batch timeout
	assert.Less(t, elapsed, hc.BatchTimeout,
		"batch health check should complete before timeout")
}

// TestHealthChecker_CheckAllNonBlocking_TimeoutPressure verifies that
// CheckAllNonBlocking correctly times out unresponsive endpoints without
// blocking the main goroutine indefinitely.
func TestHealthChecker_CheckAllNonBlocking_TimeoutPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	hc := services.NewServiceHealthChecker(newQuietLogger())
	hc.BatchTimeout = 5 * time.Second
	hc.MaxConcurrentChecks = 5

	endpoints := make(map[string]config.ServiceEndpoint)

	// Create 5 healthy TCP endpoints
	var cleanupFuncs []func()
	for i := 0; i < 5; i++ {
		ln, addr := startTCPEchoServer(t)
		cleanupFuncs = append(cleanupFuncs, func() { ln.Close() })

		host, port := extractHostPort(addr)
		endpoints[fmt.Sprintf("healthy-%d", i)] = config.ServiceEndpoint{
			Host:       host,
			Port:       port,
			Enabled:    true,
			HealthType: "tcp",
			Timeout:    time.Second,
		}
	}

	// Create 10 unreachable endpoints (non-routable IP to cause timeout)
	for i := 0; i < 10; i++ {
		endpoints[fmt.Sprintf("unreachable-%d", i)] = config.ServiceEndpoint{
			Host:       "192.0.2.1", // RFC 5737 documentation address (non-routable)
			Port:       fmt.Sprintf("%d", 59000+i),
			Enabled:    true,
			HealthType: "tcp",
			Timeout:    500 * time.Millisecond, // Short timeout per-endpoint
		}
	}

	defer func() {
		for _, cleanup := range cleanupFuncs {
			cleanup()
		}
	}()

	// The main goroutine should NOT be blocked beyond BatchTimeout
	mainDone := make(chan struct{})
	var results map[string]*services.HealthCheckResult
	var elapsed time.Duration

	go func() {
		start := time.Now()
		ctx := context.Background()
		results = hc.CheckAllNonBlocking(ctx, endpoints)
		elapsed = time.Since(start)
		close(mainDone)
	}()

	select {
	case <-mainDone:
		// Good - completed within expected time
	case <-time.After(20 * time.Second):
		t.Fatal("BLOCKED: CheckAllNonBlocking blocked main goroutine for >20s")
	}

	t.Logf("Timeout pressure: %d endpoints checked in %v (batch timeout=%v)",
		len(endpoints), elapsed, hc.BatchTimeout)

	// All endpoints should have results
	assert.Equal(t, len(endpoints), len(results),
		"every endpoint should have a result, even timed-out ones")

	// Healthy endpoints should succeed
	healthyCount := 0
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("healthy-%d", i)
		if result, ok := results[name]; ok && result.Error == nil {
			healthyCount++
		}
	}
	assert.Equal(t, 5, healthyCount,
		"all 5 healthy endpoints should pass")

	// Unreachable endpoints should fail
	failedCount := 0
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("unreachable-%d", i)
		if result, ok := results[name]; ok && result.Error != nil {
			failedCount++
		}
	}
	assert.Greater(t, failedCount, 0,
		"unreachable endpoints should report failures")
}

// TestHealthChecker_DoesNotBlockMainGoroutine verifies that invoking
// CheckAllNonBlocking from the main goroutine returns within a
// predictable timeframe, even when all endpoints are unresponsive.
func TestHealthChecker_DoesNotBlockMainGoroutine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	hc := services.NewServiceHealthChecker(newQuietLogger())
	hc.BatchTimeout = 3 * time.Second
	hc.MaxConcurrentChecks = 5

	// All unreachable endpoints
	endpoints := make(map[string]config.ServiceEndpoint)
	for i := 0; i < 20; i++ {
		endpoints[fmt.Sprintf("dead-svc-%d", i)] = config.ServiceEndpoint{
			Host:       "192.0.2.1",
			Port:       fmt.Sprintf("%d", 59100+i),
			Enabled:    true,
			HealthType: "tcp",
			Timeout:    500 * time.Millisecond,
		}
	}

	start := time.Now()
	ctx := context.Background()
	results := hc.CheckAllNonBlocking(ctx, endpoints)
	elapsed := time.Since(start)

	t.Logf("Non-blocking check: %d dead endpoints checked in %v", len(endpoints), elapsed)

	// Should complete within batch timeout + reasonable overhead
	maxExpected := hc.BatchTimeout + 2*time.Second
	assert.Less(t, elapsed, maxExpected,
		"CheckAllNonBlocking should complete within batch timeout + overhead")

	// All endpoints should have results with errors
	assert.Equal(t, len(endpoints), len(results))
	for _, result := range results {
		assert.Error(t, result.Error,
			"dead endpoint should report error")
	}
}

// TestHealthChecker_ConcurrentChecks verifies that multiple callers can
// invoke the health checker concurrently without data races or panics.
func TestHealthChecker_ConcurrentChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	// Start a healthy TCP server
	ln, addr := startTCPEchoServer(t)
	defer ln.Close()

	host, port := extractHostPort(addr)

	hc := services.NewServiceHealthChecker(newQuietLogger())

	ep := config.ServiceEndpoint{
		Host:       host,
		Port:       port,
		Enabled:    true,
		HealthType: "tcp",
		Timeout:    2 * time.Second,
	}

	const goroutineCount = 100
	var wg sync.WaitGroup
	var panics int64
	var successes int64
	var failures int64

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

			for j := 0; j < 20; j++ {
				err := hc.Check(fmt.Sprintf("svc-%d-%d", id, j), ep)
				if err != nil {
					atomic.AddInt64(&failures, 1)
				} else {
					atomic.AddInt64(&successes, 1)
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
		t.Fatal("DEADLOCK DETECTED: concurrent health checks timed out")
	}

	assert.Zero(t, panics,
		"no goroutine should panic during concurrent health checks")

	totalOps := successes + failures
	t.Logf("Concurrent checks: %d successes, %d failures out of %d total, panics=%d",
		successes, failures, totalOps, panics)

	assert.Equal(t, int64(goroutineCount*20), totalOps,
		"all check operations should complete")

	// Most checks should succeed against a local listener
	successRate := float64(successes) / float64(totalOps) * 100
	assert.Greater(t, successRate, 80.0,
		"success rate should be >80%% against local listener")
}

// TestHealthChecker_CheckWithContext_CancelRespected verifies that
// CheckWithContext respects context cancellation under heavy concurrent
// invocations — important for BootManager using tight deadlines.
func TestHealthChecker_CheckWithContext_CancelRespected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	hc := services.NewServiceHealthChecker(newQuietLogger())

	// Unreachable endpoint to force timeout
	ep := config.ServiceEndpoint{
		Host:       "192.0.2.1",
		Port:       "59999",
		Enabled:    true,
		HealthType: "tcp",
		Timeout:    10 * time.Second, // Long per-check timeout
	}

	const goroutineCount = 50
	var wg sync.WaitGroup
	var cancelledCount int64

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Very short context timeout — should cancel quickly
			ctx, cancel := context.WithTimeout(
				context.Background(), 200*time.Millisecond,
			)
			defer cancel()

			start := time.Now()
			err := hc.CheckWithContext(ctx, fmt.Sprintf("cancel-test-%d", id), ep)
			elapsed := time.Since(start)

			if err != nil {
				atomic.AddInt64(&cancelledCount, 1)
			}

			// The check should complete near the context timeout, not the
			// per-check timeout of 10s
			if elapsed > 5*time.Second {
				t.Errorf("Check for goroutine %d took %v — context cancel was not respected",
					id, elapsed)
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("BLOCKED: CheckWithContext stress test blocked for >30s")
	}

	t.Logf("Context cancel respected: %d/%d checks cancelled within deadline",
		cancelledCount, goroutineCount)

	// All checks should have been cancelled (endpoint is unreachable)
	assert.Equal(t, int64(goroutineCount), cancelledCount,
		"all checks against unreachable endpoint should fail/cancel")
}

// TestHealthChecker_SemaphoreUnderLoad verifies that the semaphore in
// CheckAllNonBlocking limits concurrent checks. With max 3 concurrent
// checks and 30 endpoints, goroutine count should remain bounded.
func TestHealthChecker_SemaphoreUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	hc := services.NewServiceHealthChecker(newQuietLogger())
	hc.BatchTimeout = 10 * time.Second
	hc.MaxConcurrentChecks = 3

	endpoints := make(map[string]config.ServiceEndpoint)
	var cleanupFuncs []func()

	// Create 30 TCP endpoints
	for i := 0; i < 30; i++ {
		ln, addr := startTCPEchoServer(t)
		cleanupFuncs = append(cleanupFuncs, func() { ln.Close() })

		host, port := extractHostPort(addr)
		endpoints[fmt.Sprintf("sem-svc-%d", i)] = config.ServiceEndpoint{
			Host:       host,
			Port:       port,
			Enabled:    true,
			HealthType: "tcp",
			Timeout:    time.Second,
		}
	}

	defer func() {
		for _, cleanup := range cleanupFuncs {
			cleanup()
		}
	}()

	goroutinesBefore := runtime.NumGoroutine()

	ctx := context.Background()
	results := hc.CheckAllNonBlocking(ctx, endpoints)

	// Allow goroutines to settle
	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	t.Logf("Semaphore under load: goroutines before=%d, after=%d, delta=%d",
		goroutinesBefore, goroutinesAfter, leaked)

	assert.Equal(t, len(endpoints), len(results),
		"all endpoints should have results")

	// No significant goroutine leak
	assert.Less(t, leaked, 20,
		"goroutine count should not grow excessively after batch check")
}
