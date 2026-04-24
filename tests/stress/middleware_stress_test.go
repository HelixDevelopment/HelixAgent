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

func init() {
	// Enforce resource limits: stress tests must use at most 2 OS threads.
	runtime.GOMAXPROCS(2)
	gin.SetMode(gin.TestMode)
}

// buildRateLimitedRouter creates a Gin router with rate-limit middleware applied.
func buildRateLimitedRouter(rl *middleware.RateLimiter) *gin.Engine {
	r := gin.New()
	r.Use(rl.Middleware())
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

// TestMiddleware_RateLimiter_ConcurrentClients verifies that the rate limiter
// correctly allows and rejects requests under concurrent load from multiple
// clients (distinct IPs). Each client should be tracked independently.
func TestMiddleware_RateLimiter_ConcurrentClients(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	rl := middleware.NewRateLimiterWithConfig(nil, &middleware.RateLimitConfig{
		Requests: 20,
		Window:   time.Second,
		KeyFunc:  middleware.ByAPIKey,
	})

	router := buildRateLimitedRouter(rl)
	server := httptest.NewServer(router)
	defer server.Close()

	const numClients = 10
	const requestsPerClient = 30 // intentionally exceeds limit of 20

	var (
		allowed int64
		denied  int64
	)

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		for i := 0; i < numClients; i++ {
			wg.Add(1)
			go func(clientID int) {
				defer wg.Done()

				client := &http.Client{Timeout: 5 * time.Second}
				for j := 0; j < requestsPerClient; j++ {
					req, err := http.NewRequest(http.MethodGet, server.URL+"/ping", nil)
					if err != nil {
						continue
					}
					// Each client uses a distinct API key so buckets are independent
					req.Header.Set("X-API-Key", "client-key-"+string(rune('A'+clientID)))

					resp, err := client.Do(req)
					if err != nil {
						continue
					}
					resp.Body.Close()

					switch resp.StatusCode {
					case http.StatusOK:
						atomic.AddInt64(&allowed, 1)
					case http.StatusTooManyRequests:
						atomic.AddInt64(&denied, 1)
					}
				}
			}(i)
		}
		wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: middleware stress test timed out")
	}

	totalAllowed := atomic.LoadInt64(&allowed)
	totalDenied := atomic.LoadInt64(&denied)
	totalRequests := int64(numClients * requestsPerClient)

	t.Logf("Rate limiter stress test results:")
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Allowed:        %d", totalAllowed)
	t.Logf("  Denied:         %d", totalDenied)

	// Every request should have been classified
	assert.Equal(t, totalRequests, totalAllowed+totalDenied,
		"every request must be either allowed or denied")

	// At least some requests must have been allowed
	assert.Greater(t, totalAllowed, int64(0), "some requests must be allowed")

	// With a limit of 20 per second per key and 30 requests per client, at
	// least some must be denied across all clients.
	assert.Greater(t, totalDenied, int64(0), "some requests must be denied")
}

// TestMiddleware_RateLimiter_SingleKey_Enforcement verifies that a single key
// is correctly limited to the configured request count within a time window.
func TestMiddleware_RateLimiter_SingleKey_Enforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	const limit = 10

	rl := middleware.NewRateLimiterWithConfig(nil, &middleware.RateLimitConfig{
		Requests: limit,
		Window:   5 * time.Second, // large window so refill doesn't interfere
		KeyFunc:  middleware.ByAPIKey,
	})

	router := buildRateLimitedRouter(rl)
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	var (
		allowed int64
		denied  int64
	)

	// Send 2× limit requests from a single key sequentially
	const total = limit * 2
	for i := 0; i < total; i++ {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/ping", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("X-API-Key", "single-test-key")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
			allowed++
		case http.StatusTooManyRequests:
			denied++
		}
	}

	t.Logf("Single-key enforcement: allowed=%d, denied=%d (limit=%d)", allowed, denied, limit)

	// Exactly `limit` requests should be allowed; the rest denied.
	assert.Equal(t, int64(limit), allowed, "should allow exactly %d requests", limit)
	assert.Equal(t, int64(total-limit), denied, "should deny %d requests", total-limit)
}

// TestMiddleware_RateLimiter_BurstConcurrent verifies the limiter is race-free
// when many goroutines hit the same key simultaneously.
func TestMiddleware_RateLimiter_BurstConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	const limit = 50
	const goroutines = 20
	const requestsEach = 10

	rl := middleware.NewRateLimiterWithConfig(nil, &middleware.RateLimitConfig{
		Requests: limit,
		Window:   10 * time.Second,
		KeyFunc:  middleware.ByAPIKey,
	})

	router := buildRateLimitedRouter(rl)
	server := httptest.NewServer(router)
	defer server.Close()

	var (
		allowed int64
		denied  int64
	)

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				client := &http.Client{Timeout: 5 * time.Second}
				for j := 0; j < requestsEach; j++ {
					req, err := http.NewRequest(http.MethodGet, server.URL+"/ping", nil)
					if err != nil {
						continue
					}
					req.Header.Set("X-API-Key", "burst-shared-key")

					resp, err := client.Do(req)
					if err != nil {
						continue
					}
					resp.Body.Close()

					switch resp.StatusCode {
					case http.StatusOK:
						atomic.AddInt64(&allowed, 1)
					case http.StatusTooManyRequests:
						atomic.AddInt64(&denied, 1)
					}
				}
			}()
		}
		wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK DETECTED: burst concurrent test timed out")
	}

	totalAllowed := atomic.LoadInt64(&allowed)
	totalDenied := atomic.LoadInt64(&denied)

	t.Logf("Burst concurrent test: allowed=%d, denied=%d, limit=%d",
		totalAllowed, totalDenied, limit)

	// No more than limit requests should be allowed
	assert.LessOrEqual(t, totalAllowed, int64(limit),
		"allowed requests must not exceed the rate limit")

	// All requests accounted for
	assert.Equal(t, int64(goroutines*requestsEach), totalAllowed+totalDenied,
		"all requests must be classified")
}

// TestMiddleware_RateLimiter_RetryAfterHeader verifies that denied responses
// include the Retry-After header.
func TestMiddleware_RateLimiter_RetryAfterHeader(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}

	rl := middleware.NewRateLimiterWithConfig(nil, &middleware.RateLimitConfig{
		Requests: 1, // allow only 1 request
		Window:   10 * time.Second,
		KeyFunc:  middleware.ByAPIKey,
	})

	router := buildRateLimitedRouter(rl)
	server := httptest.NewServer(router)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	makeReq := func() *http.Response {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/ping", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("X-API-Key", "header-test-key")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		return resp
	}

	// First request: should be allowed
	resp1 := makeReq()
	resp1.Body.Close()
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	// Second request: should be denied with Retry-After header
	resp2 := makeReq()
	resp2.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp2.StatusCode)
	assert.NotEmpty(t, resp2.Header.Get("Retry-After"),
		"Retry-After header must be present on 429 response")
}
