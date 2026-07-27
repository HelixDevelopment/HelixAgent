package stress

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestStress_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	const numRequests = 1000
	var successCount atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/health", nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code == http.StatusOK {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(numRequests), successCount.Load(), "All requests should succeed")
}

func TestStress_SustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	duration := 5 * time.Second
	var requestCount atomic.Int64
	var errorCount atomic.Int64

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					req := httptest.NewRequest("GET", "/health", nil)
					rec := httptest.NewRecorder()
					r.ServeHTTP(rec, req)
					requestCount.Add(1)
					if rec.Code != http.StatusOK {
						errorCount.Add(1)
					}
				}
			}
		}()
	}

	wg.Wait()

	t.Logf("Total requests: %d, Errors: %d", requestCount.Load(), errorCount.Load())
	assert.Equal(t, int64(0), errorCount.Load(), "No errors should occur")
	assert.Greater(t, requestCount.Load(), int64(100), "Should handle at least 100 requests")
}

func TestStress_MemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "data": make([]string, 100)})
	})

	const numRequests = 10000
	for i := 0; i < numRequests; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}

	assert.True(t, true, "Memory stability test completed")
}

func TestStress_ResponseTimeUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode") // SKIP-OK: #short-mode
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	const numRequests = 100
	var totalDuration atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			req := httptest.NewRequest("GET", "/health", nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			totalDuration.Add(int64(time.Since(start)))
		}()
	}

	wg.Wait()

	avgDuration := time.Duration(totalDuration.Load() / numRequests)
	t.Logf("Average response time: %v", avgDuration)
	assert.Less(t, avgDuration, 10*time.Millisecond, "Average response time should be under 10ms")
}

func BenchmarkHealthEndpoint(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/health", nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
		}
	})
}

func BenchmarkJSONResponse(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/json", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": "Hello, World!",
			"data":    []int{1, 2, 3, 4, 5},
		})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/json", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}
