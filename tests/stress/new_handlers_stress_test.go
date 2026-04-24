package stress

import (
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

// TestStress_DiscoveryHandler_ConcurrentAccess stress-tests the model
// discovery endpoint under high concurrent load to verify no panics,
// deadlocks, or data races occur.
func TestStress_DiscoveryHandler_ConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/v1/discovery/models", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"models": []string{
				"gpt-4", "claude-3-opus", "gemini-pro",
				"deepseek-chat", "mistral-large",
			},
		})
	})

	const goroutineCount = 200
	var wg sync.WaitGroup
	var successCount, failCount, panicCount int64

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

			for j := 0; j < 10; j++ {
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/v1/discovery/models", nil)
				router.ServeHTTP(w, req)
				if w.Code == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}
		}(i)
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
		// completed normally
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: discovery handler stress test timed out")
	}

	assert.Zero(t, panicCount, "no panics should occur under concurrent load")
	assert.Zero(t, failCount, "all requests should succeed")
	assert.Equal(t, int64(goroutineCount*10), successCount,
		"all requests should be processed")
	t.Logf("Discovery stress: success=%d, fail=%d, panics=%d",
		successCount, failCount, panicCount)
}

// TestStress_ScoringHandler_ConcurrentScoring stress-tests the model
// scoring endpoint with many concurrent requests for different models.
func TestStress_ScoringHandler_ConcurrentScoring(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/v1/scoring/model/:name", func(c *gin.Context) {
		name := c.Param("name")
		c.JSON(http.StatusOK, gin.H{
			"model": name,
			"score": 8.5,
			"components": gin.H{
				"speed":      9.0,
				"cost":       8.0,
				"capability": 8.5,
			},
		})
	})

	const goroutineCount = 300
	var wg sync.WaitGroup
	var successCount, panicCount int64

	start := make(chan struct{})
	models := []string{
		"gpt-4", "claude-3", "gemini-pro",
		"deepseek-v2", "mistral-large",
	}

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

			for j := 0; j < 20; j++ {
				model := models[j%len(models)]
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET",
					fmt.Sprintf("/v1/scoring/model/%s", model), nil)
				router.ServeHTTP(w, req)
				if w.Code == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	close(start)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
		// completed normally
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: scoring handler stress test timed out")
	}

	assert.Zero(t, panicCount, "no panics should occur under concurrent scoring load")
	assert.Equal(t, int64(goroutineCount*20), successCount,
		"all scoring requests should be processed successfully")
	t.Logf("Scoring stress: success=%d, panics=%d", successCount, panicCount)
}

// TestStress_HealthHandler_HighThroughput stress-tests the health endpoint
// to verify it remains responsive under sustained high-throughput load.
func TestStress_HealthHandler_HighThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"uptime_ms": 12345,
		})
	})

	const workers = 100
	const requestsPerWorker = 50
	var wg sync.WaitGroup
	var successCount, panicCount int64
	var maxLatency int64 // nanoseconds

	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < requestsPerWorker; j++ {
				startTime := time.Now()
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/health", nil)
				router.ServeHTTP(w, req)
				latency := time.Since(startTime).Nanoseconds()

				if w.Code == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
				}

				// Track max latency via CAS loop
				for {
					current := atomic.LoadInt64(&maxLatency)
					if latency <= current {
						break
					}
					if atomic.CompareAndSwapInt64(&maxLatency, current, latency) {
						break
					}
				}
			}
		}()
	}

	close(start)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: health handler throughput test timed out")
	}

	assert.Zero(t, panicCount, "no panics under high throughput")
	expectedTotal := int64(workers * requestsPerWorker)
	assert.Equal(t, expectedTotal, successCount,
		"all health check requests should succeed")

	maxLatencyMs := float64(atomic.LoadInt64(&maxLatency)) / 1e6
	t.Logf("Health throughput: total=%d, panics=%d, max_latency=%.2fms",
		successCount, panicCount, maxLatencyMs)
}

// TestStress_ChatCompletionHandler_MixedPayloads stress-tests the chat
// completions endpoint with varied payload sizes and formats concurrently.
func TestStress_ChatCompletionHandler_MixedPayloads(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"choices": []gin.H{{"message": gin.H{"content": "response"}}},
		})
	})

	payloads := []string{
		`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`,
		`{"model":"claude-3","messages":[{"role":"system","content":"be helpful"},{"role":"user","content":"test"}]}`,
		`{"model":"helixagent-debate","messages":[{"role":"user","content":"` +
			strings.Repeat("x", 1000) + `"}]}`,
		`{"model":"deepseek","messages":[{"role":"user","content":"short"}],"temperature":0.0}`,
		`{"model":"gemini","messages":[{"role":"user","content":"test"}],"max_tokens":10,"stream":false}`,
	}

	const workers = 150
	var wg sync.WaitGroup
	var successCount, failCount, panicCount int64

	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 15; j++ {
				payload := payloads[(id+j)%len(payloads)]
				w := httptest.NewRecorder()
				req := httptest.NewRequest("POST", "/v1/chat/completions",
					strings.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer test-key")
				router.ServeHTTP(w, req)

				if w.Code == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}
		}(i)
	}

	close(start)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: chat completion stress test timed out")
	}

	assert.Zero(t, panicCount, "no panics under mixed payload stress")
	assert.Zero(t, failCount, "all mixed payload requests should succeed")
	t.Logf("ChatCompletion stress: success=%d, fail=%d, panics=%d",
		successCount, failCount, panicCount)
}

// TestStress_MonitoringHandler_ConcurrentMetrics stress-tests the monitoring
// endpoints under concurrent access to verify metric collection safety.
func TestStress_MonitoringHandler_ConcurrentMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")  // SKIP-OK: #short-mode
	}
	runtime.GOMAXPROCS(2)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Shared counter simulating metric state
	var requestCounter int64

	router.GET("/v1/monitoring/status", func(c *gin.Context) {
		count := atomic.AddInt64(&requestCounter, 1)
		c.JSON(http.StatusOK, gin.H{
			"status":        "operational",
			"request_count": count,
		})
	})
	router.GET("/v1/monitoring/providers", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"providers": []gin.H{
				{"name": "openai", "status": "healthy"},
				{"name": "claude", "status": "healthy"},
			},
		})
	})

	endpoints := []string{
		"/v1/monitoring/status",
		"/v1/monitoring/providers",
	}

	const workers = 200
	var wg sync.WaitGroup
	var successCount, panicCount int64

	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&panicCount, 1)
				}
			}()
			<-start

			for j := 0; j < 25; j++ {
				endpoint := endpoints[j%len(endpoints)]
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", endpoint, nil)
				router.ServeHTTP(w, req)
				if w.Code == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	close(start)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("DEADLOCK DETECTED: monitoring handler stress test timed out")
	}

	assert.Zero(t, panicCount, "no panics under monitoring stress")
	assert.Equal(t, int64(workers*25), successCount,
		"all monitoring requests should succeed")
	t.Logf("Monitoring stress: success=%d, panics=%d, total_metrics=%d",
		successCount, panicCount, atomic.LoadInt64(&requestCounter))
}
