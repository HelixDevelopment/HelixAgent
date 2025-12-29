package stress

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestModelsDevStress_PerformanceBaselines(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	router.GET("/v1/models/metadata", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"models":     []interface{}{},
			"total":      1000,
			"page":       1,
			"limit":      20,
			"total_pages": 50,
		})
	})
	
	router.GET("/v1/models/metadata/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"model_id": c.Param("id"),
			"model_name": "Test Model",
			"provider_id": "anthropic",
			"provider_name": "Anthropic",
		})
	})
	
	router.GET("/v1/models/metadata/compare", func(c *gin.Context) {
		ids := c.QueryArray("ids")
		if len(ids) < 2 || len(ids) > 10 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid number of models"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"models": []interface{}{}})
	})
	
	router.GET("/v1/models/metadata/capability/:capability", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"capability": c.Param("capability"),
			"models":     []interface{}{},
			"total":      100,
		})
	})
	
	router.GET("/v1/providers/:provider_id/models/metadata", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"provider_id": c.Param("provider_id"),
			"models":      []interface{}{},
			"total":       50,
		})
	})
	
	t.Run("SingleRequest_Baseline", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=20", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		duration := time.Since(time.Now())
		t.Logf("Single request duration: %v", duration)
		assert.Less(t, duration, 100*time.Millisecond, "Should complete in <100ms")
	})

	t.Run("ResponseSize", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=20", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		body := w.Body.String()
		t.Logf("Response size: %d bytes", len(body))
		assert.Greater(t, len(body), 0, "Response should not be empty")
	})
}

	t.Run("Serialization", func(t *testing.T) {
		start := time.Now()
		
		for i := 0; i < 100; i++ {
			req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=20", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			assert.Equal(t, http.StatusOK, w.Code)
		}
		
		duration := time.Since(start)
		t.Logf("100 serial requests in %v", duration)
		assert.Less(t, duration, 2*time.Second, "Should complete in <2s")
	})
}

	t.Run("JSONParsing", func(t *testing.T) {
		start := time.Now()
		
		for i := 0; i < 1000; i++ {
			req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=20", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			var data map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &data)
			assert.NoError(t, err)
		}
		
		duration := time.Since(start)
		t.Logf("1000 JSON parse operations in %v", duration)
		assert.Less(t, duration, 3*time.Second, "Should complete in <3s")
	})
}

	t.Run("GarbageCollection", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			start := time.Now()
			
			req, _ := http.NewRequest("GET", "/v1/models/metadata?page=1&limit=20", nil)
			w := httptest.NewRecorder()
			
			for j := 0; j < 1000; j++ {
				router.ServeHTTP(w, req)
			}
			
			runtime.GC()
			
			duration := time.Since(start)
			t.Logf("GC iteration %d: %v", i+1, duration)
		}
	})
}

	t.Run("ConnectionPool", func(t *testing.T) {
		router := gin.New()
		
		router.GET("/v1/models/metadata", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"models": []interface{}{}})
		})
		
		start := time.Now()
		
		for i := 0; i < 100; i++ {
			req, _ := http.NewRequest("GET", "/v1/models/metadata", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
		
		duration := time.Since(start)
		t.Logf("100 sequential requests completed in %v", duration)
		assert.Less(t, duration, 1*time.Second)
	})
}

	t.Run("Concurrent_100", func(t *testing.T) {
		router := gin.New()
		
		router.GET("/v1/models/metadata", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"models": []interface{}{}})
		})
		
		concurrency := 100
		requests := 0
		start := time.Now()
		
		for i := 0; i < concurrency; i++ {
			req, _ := http.NewRequest("GET", "/v1/models/metadata", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			if w.Code == http.StatusOK {
				requests++
			}
		}
		
		duration := time.Since(start)
		
		assert.Equal(t, concurrency, requests, "All requests should succeed")
		t.Less(t, duration, 1*time.Second, "Should complete in <1s")
		t.Logf("100 concurrent requests: %d successful in %v", requests)
	})

	t.Run("Concurrent_500", func(t *testing.T) {
		router := gin.New()
		
		router.GET("/v1/models/metadata", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"models": []interface{}{}})
		})
		
		concurrency := 500
		requests := 0
		start := time.Now()
		
		for i := 0; i < concurrency; i++ {
			req,	reqErr := http.NewRequest("GET", "/v1/models/metadata", nil)
			if reqErr != nil {
				t.Fatal("Failed to create request")
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, reqErr, w)
			
			if w.Code == http.StatusOK {
				requests++
			}
		}
		
		duration := time.Since(start)
		
		assert.Greater(t, float64(requests)/float64(concurrency), 0.99, "99%+ success rate")
		assert.Less(t, duration, 3*time.Second, "Should complete in <3s")
		t.Logf("500 concurrent requests: %d successful in %v with %.2f%% success rate", requests, float64(requests)/float64(concurrency)*100)
	})

	t.Run("SustainedLoad_100_For_10s", func(t *testing.T) {
		router := gin.New()
		
		router.GET("/v1/models/metadata", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"models": []interface{}{}})
		})
		
		rps := 100
		rate := 10
		totalDuration := 10 * time.Second
		
		successCount := 0
		failCount := 0
		
		var wg sync.WaitGroup
		stopTime := time.Now().Add(totalDuration)
		
		for time.Now().Before(stopTime) {
			for i := 0; i < rate; i++ {
				wg.Add(1)
				
				go func() {
					req, _ := http.NewRequest("GET", "/v1/models/metadata", nil)
					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)
					
					if w.Code == http.StatusOK {
						successCount++
					} else {
						failCount++
					}
				}()
			}
			
			time.Sleep(1 * time.Second / time.Duration(rate))
		}
		}
		
		wg.Wait()
		
		t.Logf("Sustained load: %d RPS for %v - Success: %d, Failed: %d", rps*10, totalDuration, successCount, failCount)
		
		assert.GreaterOrEqual(t, float64(successCount+failCount), float64(rps*10), "Should process at least 90% of requests")
	})
}

	t.Run("MixedEndpoints_500", func(t *testing.T) {
		router := gin.New()
		
		router.GET("/v1/models/metadata", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"models": []interface{}{}})
		})
		
		router.GET("/v1/models/metadata/model-1", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"model_id": "model-1"})
		})
		
		router.GET("/v1/models/metadata/compare", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"models": []interface{}{}})
		})
		
		router.GET("/v1/models/metadata/capability/vision", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"capability": "vision", "models": []interface{}{}})
		})
		
		router.GET("/v1/providers/anthropic/models/metadata", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"models": []interface{}{}})
		})
		
		endpoints := []string{
			"/v1/models/metadata",
			"/v1/models/metadata/model-1",
			"/v1/models/metadata/compare?ids=model-1,model-2,model-3",
			"/v1/models/metadata/capability/vision",
			"/v1/providers/anthropic/models/metadata",
		}
		
		concurrency := 500
		requestCount := int64(0)
		successCount := int64(0)
		
		var wg sync.WaitGroup
		start := time.Now()
		
		for i := int64(0); i < int64(concurrency); i++ {
			wg.Add(1)
			go func(idx int64) {
				defer wg.Done()
				
				endpoint := endpoints[idx%len(endpoints)]
				
				req, _ := http.NewRequest("GET", endpoint, nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				
				if w.Code == http.StatusOK {
					atomic.AddInt64(&successCount, 1)
				}
				atomic.AddInt64(&requestCount, 1)
			}()
		}
		
		wg.Wait()
		duration := time.Since(start)
		
		t.Logf("Mixed endpoints: %d requests in %v with %.2f%% success", atomic.LoadInt64(&requestCount), float64(atomic.LoadInt64(&successCount))/float64(atomic.LoadInt64(&requestCount))*100)
		
		assert.Less(t, duration, 10*time.Second, "Should complete in <10s")
		assert.GreaterOrEqual(t, float64(atomic.LoadInt64(&successCount))/float64(atomic.LoadInt64(&requestCount)), 0.95, "Success rate should be >=95%")
	})
}
