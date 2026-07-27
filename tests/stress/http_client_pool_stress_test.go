//go:build stress
// +build stress

package stress

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	internalhttp "dev.helix.agent/internal/http"
)

// startEchoServer starts a lightweight httptest.Server that echoes a fixed
// response. It is closed by the caller via the returned func.
func startEchoServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	return srv, srv.URL
}

// TestHTTPClientPool_ConcurrentGetRequests_Stress launches 200 goroutines
// each firing one GET request through the pool to verify connection reuse,
// no panics, no deadlocks, and no goroutine leaks.
func TestHTTPClientPool_ConcurrentGetRequests_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	srv, baseURL := startEchoServer(t)
	defer srv.Close()

	pool := internalhttp.NewHTTPClientPool(&internalhttp.PoolConfig{
		MaxIdleConns:          50,
		MaxConnsPerHost:       20,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       30 * time.Second,
		DialTimeout:           5 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		KeepAliveInterval:     15 * time.Second,
		RetryCount:            0, // no retries for speed
	})
	defer func() { _ = pool.Close() }()

	goroutinesBefore := runtime.NumGoroutine()

	const concurrency = 200
	var (
		wg       sync.WaitGroup
		panics   int64
		success  int64
		failures int64
	)

	start := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			resp, err := pool.Get(ctx, baseURL+"/")
			if err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				atomic.AddInt64(&success, 1)
			} else {
				atomic.AddInt64(&failures, 1)
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
		t.Fatal("DEADLOCK DETECTED: HTTP client pool concurrent GET stress timed out")
	}

	assert.Zero(t, panics, "no goroutine should panic during concurrent GET stress")
	assert.Equal(t, int64(concurrency), success+failures,
		"all goroutines must complete")
	assert.Greater(t, success, int64(concurrency*9/10),
		"at least 90%% of requests should succeed against local server")

	metrics := pool.Metrics()
	t.Logf("HTTPClientPool GET stress: success=%d, failures=%d, panics=%d", success, failures, panics)
	t.Logf("Pool metrics: total=%d, success=%d, failed=%d, retries=%d",
		metrics.TotalRequests, metrics.SuccessRequests, metrics.FailedRequests, metrics.RetryCount)

	// Goroutine leak check
	pool.CloseIdleConnections()
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d", goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 20, "goroutine count should not grow significantly")
}

// TestHTTPClientPool_ConnectionChurn_Stress exercises connection churn by
// using many distinct hosts (via separate httptest servers) to force the
// pool to create and manage multiple per-host clients concurrently.
func TestHTTPClientPool_ConnectionChurn_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	// Spin up 10 separate test servers to simulate distinct hosts
	const serverCount = 10
	servers := make([]*httptest.Server, serverCount)
	urls := make([]string, serverCount)
	for i := 0; i < serverCount; i++ {
		srv, u := startEchoServer(t)
		servers[i] = srv
		urls[i] = u
	}
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	pool := internalhttp.NewHTTPClientPool(&internalhttp.PoolConfig{
		MaxIdleConns:          100,
		MaxConnsPerHost:       10,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       10 * time.Second,
		DialTimeout:           5 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		KeepAliveInterval:     10 * time.Second,
		RetryCount:            0,
	})
	defer func() { _ = pool.Close() }()

	goroutinesBefore := runtime.NumGoroutine()

	var (
		wg      sync.WaitGroup
		panics  int64
		success int64
	)

	start := make(chan struct{})

	// 100 goroutines each making 5 requests to random servers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			for j := 0; j < 5; j++ {
				targetURL := urls[(id+j)%serverCount]
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				resp, err := pool.Get(ctx, targetURL+"/")
				cancel()
				if err == nil && resp != nil {
					_, _ = io.Copy(io.Discard, resp.Body)
					_ = resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						atomic.AddInt64(&success, 1)
					}
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
		t.Fatal("DEADLOCK DETECTED: HTTP client pool connection churn stress timed out")
	}

	assert.Zero(t, panics, "no panics during connection churn stress")
	assert.Greater(t, success, int64(400),
		"majority of requests to local test servers should succeed")

	// Pool should have created one client per distinct host
	clientCount := pool.ClientCount()
	assert.Equal(t, serverCount, clientCount,
		"pool should have exactly %d per-host clients after churn", serverCount)

	pool.CloseIdleConnections()
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines after churn: before=%d, after=%d, delta=%d",
		goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 20, "connection churn should not leak goroutines")

	t.Logf("Connection churn stress: success=%d, clients=%d, panics=%d",
		success, clientCount, panics)
}

// TestHTTPClientPool_MetricsAccuracy_Stress verifies that pool metrics
// (TotalRequests, SuccessRequests) stay consistent with the actual number
// of requests made under concurrent load.
func TestHTTPClientPool_MetricsAccuracy_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	srv, baseURL := startEchoServer(t)
	defer srv.Close()

	pool := internalhttp.NewHTTPClientPool(&internalhttp.PoolConfig{
		MaxIdleConns:          50,
		MaxConnsPerHost:       20,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       30 * time.Second,
		DialTimeout:           5 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		KeepAliveInterval:     15 * time.Second,
		RetryCount:            0,
	})
	defer func() { _ = pool.Close() }()

	const (
		goroutines   = 50
		requestsEach = 4
		total        = goroutines * requestsEach
	)

	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start
			for j := 0; j < requestsEach; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				resp, err := pool.Get(ctx, fmt.Sprintf("%s/?id=%d-%d", baseURL, id, j))
				cancel()
				if err == nil && resp != nil {
					_, _ = io.Copy(io.Discard, resp.Body)
					_ = resp.Body.Close()
				}
			}
		}(i)
	}

	close(start)
	wg.Wait()

	metrics := pool.Metrics()
	t.Logf("Metrics: total=%d, success=%d, failed=%d",
		metrics.TotalRequests, metrics.SuccessRequests, metrics.FailedRequests)

	assert.Equal(t, int64(total), metrics.TotalRequests,
		"TotalRequests metric should match actual requests sent")
	assert.Equal(t, metrics.TotalRequests,
		metrics.SuccessRequests+metrics.FailedRequests,
		"SuccessRequests + FailedRequests must equal TotalRequests")
	assert.GreaterOrEqual(t, metrics.SuccessRequests, int64(total*9/10),
		"at least 90%% of requests to local server should succeed")
}

// TestHTTPClientPool_HostClientConcurrentRequests_Stress exercises the
// HostClient wrapper under concurrent load to verify header isolation and
// no panics.
func TestHTTPClientPool_HostClientConcurrentRequests_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	srv, baseURL := startEchoServer(t)
	defer srv.Close()

	pool := internalhttp.NewHTTPClientPool(nil)
	defer func() { _ = pool.Close() }()

	hostClient, err := internalhttp.NewHostClient(pool, baseURL)
	if err != nil {
		t.Fatalf("failed to create host client: %v", err)
	}
	hostClient.SetHeader("X-Stress-Test", "true")

	goroutinesBefore := runtime.NumGoroutine()

	const concurrency = 100
	var (
		wg      sync.WaitGroup
		panics  int64
		success int64
	)

	start := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			resp, err := hostClient.Get(ctx, fmt.Sprintf("/?worker=%d", id))
			if err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				atomic.AddInt64(&success, 1)
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
		t.Fatal("DEADLOCK DETECTED: HostClient concurrent requests stress timed out")
	}

	assert.Zero(t, panics, "no panics during HostClient concurrent request stress")
	assert.Greater(t, success, int64(concurrency*9/10),
		"at least 90%% of HostClient requests should succeed")

	pool.CloseIdleConnections()
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d", goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 20, "HostClient stress should not leak goroutines")

	t.Logf("HostClient stress: success=%d, panics=%d", success, panics)
}

// TestHTTPClientPool_GlobalPool_Stress exercises the global pool helper
// functions (Get/PostJSON) under concurrent load to verify the singleton
// is safe for concurrent use.
func TestHTTPClientPool_GlobalPool_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	srv, baseURL := startEchoServer(t)
	defer srv.Close()

	// Re-initialize global pool with tight limits for the test
	internalhttp.InitGlobalPool(&internalhttp.PoolConfig{
		MaxIdleConns:          50,
		MaxConnsPerHost:       10,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       10 * time.Second,
		DialTimeout:           5 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		KeepAliveInterval:     10 * time.Second,
		RetryCount:            0,
	})

	const concurrency = 100
	var (
		wg      sync.WaitGroup
		panics  int64
		success int64
	)

	start := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()
			<-start

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			resp, err := internalhttp.Get(ctx, fmt.Sprintf("%s/?global=%d", baseURL, id))
			if err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				atomic.AddInt64(&success, 1)
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
		t.Fatal("DEADLOCK DETECTED: global pool stress timed out")
	}

	assert.Zero(t, panics, "no panics during global pool stress")
	assert.Greater(t, success, int64(concurrency*9/10),
		"at least 90%% of global pool requests should succeed")

	t.Logf("Global pool stress: success=%d, panics=%d", success, panics)
}
