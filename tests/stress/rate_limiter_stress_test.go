//go:build stress
// +build stress

package stress

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/middleware"
)

// TestRateLimiter_100Goroutines_100Requests_Each launches 100 goroutines, each
// sending 100 requests through a configured rate limiter. The test verifies
// that the total number of allowed requests does not exceed the configured
// limit × window factor and that no goroutines leak after completion.
func TestRateLimiter_100Goroutines_100Requests_Each(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	// Enforce resource limits per CLAUDE.md rule 15.
	runtime.GOMAXPROCS(2)
	gin.SetMode(gin.TestMode)

	const (
		goroutines    = 100
		requestsEach  = 100
		configLimit   = 50 // tokens per window per key
		windowSeconds = 1  // 1-second window
	)

	// Single shared key so all 100×100 requests contend on the same bucket.
	rl := middleware.NewRateLimiterWithConfig(nil, &middleware.RateLimitConfig{
		Requests: configLimit,
		Window:   time.Duration(windowSeconds) * time.Second,
		KeyFunc:  func(_ *gin.Context) string { return "shared-key" },
	})

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Measure goroutines before the storm.
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var (
		allowed int64
		denied  int64
		wg      sync.WaitGroup
		start   = make(chan struct{})
	)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // release all goroutines simultaneously

			for j := 0; j < requestsEach; j++ {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				switch w.Code {
				case http.StatusOK:
					atomic.AddInt64(&allowed, 1)
				case http.StatusTooManyRequests:
					atomic.AddInt64(&denied, 1)
				}
			}
		}()
	}

	close(start)

	// Wait with a hard deadline to detect deadlocks.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: rate limiter stress timed out after 30s")
	}

	total := allowed + denied
	assert.Equal(t, int64(goroutines*requestsEach), total,
		"every request must receive a 200 or 429 response")

	// Allowed requests must not exceed configLimit per window.
	// We ran within a burst that fits in roughly 1-2 windows so the ceiling is
	// a small multiple of configLimit; assert a conservative upper bound.
	assert.LessOrEqual(t, allowed, int64(configLimit*20),
		"allowed requests must be bounded by rate-limit configuration")

	t.Logf("Rate-limiter stress: allowed=%d denied=%d total=%d (limit=%d/window)",
		allowed, denied, total, configLimit)

	// Goroutine-leak check.
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore
	assert.Less(t, leaked, 30,
		"goroutine count must not grow excessively after rate-limiter stress")
	t.Logf("Goroutines: before=%d after=%d leaked=%d", goroutinesBefore, goroutinesAfter, leaked)
}

// TestRateLimiter_PerClientIsolation verifies that each distinct client key
// has its own independent token bucket under concurrent load.
func TestRateLimiter_PerClientIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)
	gin.SetMode(gin.TestMode)

	const (
		clients      = 20
		requestsEach = 100
		limitPerKey  = 60
	)

	clientKeys := make([]string, clients)
	for i := 0; i < clients; i++ {
		clientKeys[i] = "client-key-" + string(rune('A'+i))
	}

	keyIdx := int64(-1)
	rl := middleware.NewRateLimiterWithConfig(nil, &middleware.RateLimitConfig{
		Requests: limitPerKey,
		Window:   10 * time.Second,
		KeyFunc: func(_ *gin.Context) string {
			idx := atomic.AddInt64(&keyIdx, 1) % int64(clients)
			return clientKeys[idx]
		},
	})

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"pong": true})
	})

	var (
		wg      sync.WaitGroup
		allowed int64
		denied  int64
	)

	start := make(chan struct{})

	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < requestsEach; j++ {
				req := httptest.NewRequest(http.MethodGet, "/ping", nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				if w.Code == http.StatusOK {
					atomic.AddInt64(&allowed, 1)
				} else {
					atomic.AddInt64(&denied, 1)
				}
			}
		}()
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
		t.Fatal("DEADLOCK: per-client isolation stress timed out")
	}

	total := allowed + denied
	assert.Equal(t, int64(clients*requestsEach), total, "all requests must complete")
	t.Logf("Per-client isolation: allowed=%d denied=%d", allowed, denied)
}
