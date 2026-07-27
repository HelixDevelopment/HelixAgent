package stress

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestStress_APIHealthEndpoint_ConcurrentAccess fires 50 concurrent requests
// at the health endpoint and verifies no panics occur, all responses are
// valid JSON, and goroutine count remains stable afterward.
func TestStress_APIHealthEndpoint_ConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().UnixMilli(),
			"version":   "1.0.0",
		})
	})

	const concurrency = 50

	// Goroutine baseline
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var successCount, failCount, panicCount int64
	var invalidJSONCount int64

	start := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/health", nil)
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				atomic.AddInt64(&successCount, 1)

				// Validate response is valid JSON
				var result map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
					atomic.AddInt64(&invalidJSONCount, 1)
				}
			} else {
				atomic.AddInt64(&failCount, 1)
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
		t.Fatal("DEADLOCK DETECTED: API health endpoint stress test timed out")
	}

	// Check goroutine leak
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	assert.Zero(t, panicCount, "no panics should occur under 50 concurrent health requests")
	assert.Zero(t, failCount, "all health requests should succeed")
	assert.Zero(t, invalidJSONCount, "all responses must be valid JSON")
	assert.Equal(t, int64(concurrency), successCount, "all requests must complete")
	assert.Less(t, leaked, 10,
		"goroutine count should not grow significantly after concurrent API access")
	t.Logf("API health stress: success=%d, fail=%d, panics=%d, "+
		"invalidJSON=%d, goroutine_leak=%d",
		successCount, failCount, panicCount, invalidJSONCount, leaked)
}

// TestStress_APIModelsEndpoint_ConcurrentAccess exercises the models listing
// endpoint with 50 concurrent goroutines to verify consistent results and
// no data races in model serialization.
func TestStress_APIModelsEndpoint_ConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	models := []gin.H{
		{"id": "gpt-4", "provider": "openai"},
		{"id": "claude-3-opus", "provider": "anthropic"},
		{"id": "gemini-pro", "provider": "google"},
		{"id": "deepseek-chat", "provider": "deepseek"},
		{"id": "mistral-large", "provider": "mistral"},
	}

	router.GET("/v1/models", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"object": "list",
			"data":   models,
		})
	})

	const concurrency = 50

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var successCount, panicCount int64
	var inconsistentCount int64

	start := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/models", nil)
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				atomic.AddInt64(&successCount, 1)

				var result map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &result); err == nil {
					data, ok := result["data"].([]interface{})
					if !ok || len(data) != len(models) {
						atomic.AddInt64(&inconsistentCount, 1)
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
		t.Fatal("DEADLOCK DETECTED: API models endpoint stress test timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	assert.Zero(t, panicCount, "no panics under concurrent models access")
	assert.Equal(t, int64(concurrency), successCount, "all model requests should succeed")
	assert.Zero(t, inconsistentCount, "all responses should have consistent model count")
	assert.Less(t, leaked, 10, "no goroutine leak after models stress test")
	t.Logf("API models stress: success=%d, panics=%d, inconsistent=%d, goroutine_leak=%d",
		successCount, panicCount, inconsistentCount, leaked)
}

// TestStress_APIChatCompletions_ConcurrentJSON validates that the chat
// completions endpoint handles 50 concurrent POST requests with varied
// payloads without panics, returning valid JSON for every request.
func TestStress_APIChatCompletions_ConcurrentJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"id":      "chatcmpl-stress",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"choices": []gin.H{
				{
					"index": 0,
					"message": gin.H{
						"role":    "assistant",
						"content": "Stress test response",
					},
					"finish_reason": "stop",
				},
			},
		})
	})

	const concurrency = 50
	payloads := []string{
		`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`,
		`{"model":"claude-3","messages":[{"role":"system","content":"be helpful"},{"role":"user","content":"test"}]}`,
		`{"model":"helixagent-debate","messages":[{"role":"user","content":"` +
			strings.Repeat("a", 500) + `"}]}`,
		`{"model":"deepseek","messages":[{"role":"user","content":"short"}],"temperature":0.5}`,
	}

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var successCount, panicCount, invalidJSONCount int64

	start := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			payload := payloads[id%len(payloads)]
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/chat/completions",
				strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				atomic.AddInt64(&successCount, 1)
				var result map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
					atomic.AddInt64(&invalidJSONCount, 1)
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
		t.Fatal("DEADLOCK DETECTED: chat completions stress test timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	assert.Zero(t, panicCount, "no panics under concurrent chat completions")
	assert.Equal(t, int64(concurrency), successCount, "all chat completion requests succeed")
	assert.Zero(t, invalidJSONCount, "all chat responses must be valid JSON")
	assert.Less(t, leaked, 10, "no goroutine leaks after chat completions stress")
	t.Logf("Chat completions stress: success=%d, panics=%d, invalidJSON=%d, goroutine_leak=%d",
		successCount, panicCount, invalidJSONCount, leaked)
}

// TestStress_APIMultiEndpoint_RapidAlternation rapidly alternates between
// multiple API endpoints from 50 concurrent goroutines, verifying that
// router-level concurrency remains safe and response times stay bounded.
func TestStress_APIMultiEndpoint_RapidAlternation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/v1/models", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"object": "list", "data": []string{"model-a", "model-b"}})
	})
	router.GET("/v1/monitoring/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "operational"})
	})
	router.GET("/v1/discovery/providers", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"providers": []string{"openai", "anthropic"}})
	})

	endpoints := []string{
		"/health",
		"/v1/models",
		"/v1/monitoring/status",
		"/v1/discovery/providers",
	}

	const concurrency = 50
	const requestsPerWorker = 20

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var totalSuccess, panicCount int64
	var maxLatencyNs int64

	start := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < requestsPerWorker; j++ {
				endpoint := endpoints[(id+j)%len(endpoints)]
				startTime := time.Now()

				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", endpoint, nil)
				router.ServeHTTP(w, req)

				latency := time.Since(startTime).Nanoseconds()

				if w.Code == http.StatusOK {
					atomic.AddInt64(&totalSuccess, 1)
				}

				// Track max latency via CAS loop
				for {
					current := atomic.LoadInt64(&maxLatencyNs)
					if latency <= current {
						break
					}
					if atomic.CompareAndSwapInt64(&maxLatencyNs, current, latency) {
						break
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
		t.Fatal("DEADLOCK DETECTED: multi-endpoint alternation stress test timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	expectedTotal := int64(concurrency * requestsPerWorker)
	maxLatencyMs := float64(atomic.LoadInt64(&maxLatencyNs)) / 1e6

	assert.Zero(t, panicCount, "no panics during multi-endpoint alternation")
	assert.Equal(t, expectedTotal, totalSuccess,
		"all multi-endpoint requests should succeed")
	assert.Less(t, maxLatencyMs, 1000.0,
		"max response latency should be under 1s for in-process handlers")
	assert.Less(t, leaked, 10, "no goroutine leaks after multi-endpoint stress")
	t.Logf("Multi-endpoint stress: total=%d, panics=%d, "+
		"max_latency=%.2fms, goroutine_leak=%d",
		totalSuccess, panicCount, maxLatencyMs, leaked)
}

// TestStress_APIErrorPaths_ConcurrentNotFound verifies that the router
// handles concurrent requests to nonexistent endpoints gracefully (404)
// without panics or goroutine leaks.
func TestStress_APIErrorPaths_ConcurrentNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	const concurrency = 50

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	var wg sync.WaitGroup
	var notFoundCount, panicCount int64

	start := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			path := fmt.Sprintf("/nonexistent/path/%d", id)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", path, nil)
			router.ServeHTTP(w, req)

			if w.Code == http.StatusNotFound {
				atomic.AddInt64(&notFoundCount, 1)
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
		t.Fatal("DEADLOCK DETECTED: error path stress test timed out")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	leaked := goroutinesAfter - goroutinesBefore

	assert.Zero(t, panicCount, "no panics on concurrent 404 requests")
	assert.Equal(t, int64(concurrency), notFoundCount,
		"all nonexistent-path requests should return 404")
	assert.Less(t, leaked, 10, "no goroutine leaks after error path stress")
	t.Logf("Error path stress: 404s=%d, panics=%d, goroutine_leak=%d",
		notFoundCount, panicCount, leaked)
}
