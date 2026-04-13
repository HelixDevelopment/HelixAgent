//go:build stress
// +build stress

package stress

import (
	"context"
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

// TestHTTPPoolExhaustion verifies that when all pool connections are busy,
// new requests are handled gracefully via circuit breaker timeout rather
// than blocking forever or panicking. The test uses a slow server to hold
// connections open while a wave of new requests arrives.
func TestHTTPPoolExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)

	const (
		maxConnsPerHost = 5 // tight pool to force exhaustion quickly
		holdDuration    = 200 * time.Millisecond
		requestTimeout  = 150 * time.Millisecond // shorter than hold — causes timeout
		totalRequests   = 50
	)

	// Slow server: holds each connection open for holdDuration.
	var activeConns int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&activeConns, 1)
		defer atomic.AddInt64(&activeConns, -1)
		time.Sleep(holdDuration)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	pool := internalhttp.NewHTTPClientPool(&internalhttp.PoolConfig{
		MaxIdleConns:          maxConnsPerHost,
		MaxConnsPerHost:       maxConnsPerHost,
		MaxIdleConnsPerHost:   maxConnsPerHost,
		IdleConnTimeout:       10 * time.Second,
		DialTimeout:           2 * time.Second,
		TLSHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: requestTimeout,
		ExpectContinueTimeout: 500 * time.Millisecond,
		KeepAliveInterval:     10 * time.Second,
		RetryCount:            0,
	})
	defer func() { _ = pool.Close() }()

	goroutinesBefore := runtime.NumGoroutine()

	var (
		wg        sync.WaitGroup
		succeeded int64
		timedOut  int64
		panics    int64
	)

	start := make(chan struct{})

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panics, 1)
				}
			}()

			<-start

			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			defer cancel()

			resp, err := pool.Get(ctx, srv.URL+"/")
			if err != nil {
				// Timeout or connection refused — graceful handling
				atomic.AddInt64(&timedOut, 1)
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				atomic.AddInt64(&succeeded, 1)
			} else {
				atomic.AddInt64(&timedOut, 1)
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Deadlock detection: 30s timeout
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: HTTP pool exhaustion test timed out after 30s")
	}

	assert.Zero(t, panics, "no panics when HTTP pool is exhausted")

	// Key assertion: all requests must complete (no hung goroutines).
	assert.Equal(t, int64(totalRequests), succeeded+timedOut,
		"all requests must complete — either succeed or timeout gracefully")

	// With exhaustion, we expect at least some timeouts (pool was undersized vs load).
	// But the system must not deadlock or panic.
	t.Logf("HTTP pool exhaustion: total=%d, succeeded=%d, timedOut=%d, panics=%d",
		totalRequests, succeeded, timedOut, panics)

	// Goroutine leak check
	pool.CloseIdleConnections()
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines: before=%d, after=%d, delta=%d", goroutinesBefore, goroutinesAfter, leaked)
	assert.Less(t, leaked, 20, "no goroutine leak after HTTP pool exhaustion")
}
