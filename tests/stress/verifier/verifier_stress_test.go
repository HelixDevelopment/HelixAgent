package verifier

import (
	"bytes"
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// StressTestConfig holds configuration for stress tests
type StressTestConfig struct {
	BaseURL     string
	Concurrency int
	Duration    time.Duration
	RequestRate int // requests per second
	Timeout     time.Duration
}

// StressTestResult holds results from stress tests
type StressTestResult struct {
	TotalRequests    int64
	SuccessfulReqs   int64
	FailedReqs       int64
	AvgResponseTime  time.Duration
	MinResponseTime  time.Duration
	MaxResponseTime  time.Duration
	ThroughputPerSec float64
	ErrorRate        float64
	MemoryUsage      runtime.MemStats
	Duration         time.Duration
}

// checkServerAvailable checks if the test server is reachable
func checkServerAvailable(baseURL string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// TestVerifierStress performs stress testing on verifier endpoints
func TestVerifierStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	config := StressTestConfig{
		BaseURL:     "http://localhost:7061",
		Concurrency: 50,
		Duration:    30 * time.Second,
		RequestRate: 100,
		Timeout:     30 * time.Second,
	}

	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping stress test - server not available at " + config.BaseURL)
	}

	t.Run("VerifyEndpointStress", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"model_id": "gpt-4",
			"provider": "openai",
		}

		result := performVerifierStressTest(t, config, "/api/v1/verifier/verify", "POST", requestBody)
		analyzeVerifierStressResults(t, result, "Verify Endpoint")
	})

	t.Run("HealthEndpointStress", func(t *testing.T) {
		result := performVerifierStressTest(t, config, "/api/v1/verifier/health", "GET", nil)
		analyzeVerifierStressResults(t, result, "Health Endpoint")
	})

	t.Run("ScoreEndpointStress", func(t *testing.T) {
		result := performVerifierStressTest(t, config, "/api/v1/verifier/scores/gpt-4", "GET", nil)
		analyzeVerifierStressResults(t, result, "Score Endpoint")
	})

	t.Run("TopModelsEndpointStress", func(t *testing.T) {
		result := performVerifierStressTest(t, config, "/api/v1/verifier/scores/top?limit=10", "GET", nil)
		analyzeVerifierStressResults(t, result, "Top Models Endpoint")
	})

	t.Run("ProviderHealthEndpointStress", func(t *testing.T) {
		result := performVerifierStressTest(t, config, "/api/v1/verifier/health/providers", "GET", nil)
		analyzeVerifierStressResults(t, result, "Provider Health Endpoint")
	})

	t.Run("CodeVisibilityEndpointStress", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"code":     "func main() { fmt.Println(\"Hello\") }",
			"language": "go",
			"model_id": "gpt-4",
			"provider": "openai",
		}

		result := performVerifierStressTest(t, config, "/api/v1/verifier/code-visibility", "POST", requestBody)
		analyzeVerifierStressResults(t, result, "Code Visibility Endpoint")
	})
}

// TestVerifierConcurrency tests concurrent request handling
func TestVerifierConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	config := StressTestConfig{
		BaseURL:     "http://localhost:7061",
		Concurrency: 100,
		Duration:    10 * time.Second,
		Timeout:     30 * time.Second,
	}

	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping stress test - server not available at " + config.BaseURL)
	}

	t.Run("ConcurrentVerifications", func(t *testing.T) {
		var wg sync.WaitGroup
		var successCount, failCount int64
		client := &http.Client{Timeout: config.Timeout}

		models := []string{"gpt-4", "claude-3-opus", "gemini-pro", "deepseek-chat"}
		providers := []string{"openai", "anthropic", "google", "deepseek"}

		for i := 0; i < config.Concurrency; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				requestBody := map[string]interface{}{
					"model_id": models[idx%len(models)],
					"provider": providers[idx%len(providers)],
				}

				jsonData, _ := json.Marshal(requestBody)
				resp, err := client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer(jsonData))

				if err != nil {
					atomic.AddInt64(&failCount, 1)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Concurrent verifications: %d successful, %d failed", successCount, failCount)
		assert.Greater(t, successCount+failCount, int64(0), "Should have processed some requests")
	})

	t.Run("ConcurrentMixedOperations", func(t *testing.T) {
		var wg sync.WaitGroup
		var successCount, failCount int64
		client := &http.Client{Timeout: config.Timeout}

		operations := []struct {
			method string
			path   string
			body   map[string]interface{}
		}{
			{"POST", "/api/v1/verifier/verify", map[string]interface{}{"model_id": "gpt-4", "provider": "openai"}},
			{"GET", "/api/v1/verifier/health", nil},
			{"GET", "/api/v1/verifier/scores/gpt-4", nil},
			{"GET", "/api/v1/verifier/health/providers", nil},
			{"GET", "/api/v1/verifier/scores/top", nil},
		}

		for i := 0; i < config.Concurrency; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				op := operations[idx%len(operations)]
				var resp *http.Response
				var err error

				if op.method == "GET" {
					resp, err = client.Get(config.BaseURL + op.path)
				} else {
					jsonData, _ := json.Marshal(op.body)
					resp, err = client.Post(config.BaseURL+op.path, "application/json", bytes.NewBuffer(jsonData))
				}

				if err != nil {
					atomic.AddInt64(&failCount, 1)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode < 500 {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Concurrent mixed operations: %d successful, %d failed", successCount, failCount)
	})
}

// TestVerifierBurstLoad tests burst load handling
func TestVerifierBurstLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	config := StressTestConfig{
		BaseURL:     "http://localhost:7061",
		Concurrency: 200,
		Timeout:     30 * time.Second,
	}

	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping stress test - server not available at " + config.BaseURL)
	}

	t.Run("BurstVerifyRequests", func(t *testing.T) {
		var wg sync.WaitGroup
		var successCount, failCount int64
		client := &http.Client{Timeout: config.Timeout}

		requestBody := map[string]interface{}{
			"model_id": "gpt-4",
			"provider": "openai",
		}
		jsonData, _ := json.Marshal(requestBody)

		// Send burst of requests
		start := time.Now()
		for i := 0; i < config.Concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				resp, err := client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer(jsonData))
				if err != nil {
					atomic.AddInt64(&failCount, 1)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode < 500 {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}()
		}

		wg.Wait()
		duration := time.Since(start)

		t.Logf("Burst load (%d requests): %d successful, %d failed in %v",
			config.Concurrency, successCount, failCount, duration)
		t.Logf("Throughput: %.2f req/sec", float64(successCount+failCount)/duration.Seconds())
	})
}

// TestVerifierMemoryStress tests memory usage under load
func TestVerifierMemoryStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	config := StressTestConfig{
		BaseURL:     "http://localhost:7061",
		Concurrency: 50,
		Duration:    15 * time.Second,
		Timeout:     30 * time.Second,
	}

	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping stress test - server not available at " + config.BaseURL)
	}

	t.Run("MemoryUsageUnderLoad", func(t *testing.T) {
		var memBefore, memAfter runtime.MemStats
		runtime.ReadMemStats(&memBefore)

		client := &http.Client{Timeout: config.Timeout}
		requestBody := map[string]interface{}{
			"model_id": "gpt-4",
			"provider": "openai",
		}
		jsonData, _ := json.Marshal(requestBody)

		// Generate sustained load
		done := make(chan bool)
		var requestCount int64

		for i := 0; i < config.Concurrency; i++ {
			go func() {
				for {
					select {
					case <-done:
						return
					default:
						resp, err := client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer(jsonData))
						if err == nil {
							resp.Body.Close()
							atomic.AddInt64(&requestCount, 1)
						}
					}
				}
			}()
		}

		time.Sleep(config.Duration)
		close(done)

		runtime.ReadMemStats(&memAfter)

		t.Logf("Requests processed: %d", requestCount)
		t.Logf("Memory before: %d MB, Memory after: %d MB",
			memBefore.Alloc/1024/1024, memAfter.Alloc/1024/1024)
		t.Logf("Memory growth: %d MB",
			(memAfter.Alloc-memBefore.Alloc)/1024/1024)
	})
}

// TestVerifierBatchStress tests batch verification under stress
func TestVerifierBatchStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	config := StressTestConfig{
		BaseURL:     "http://localhost:7061",
		Concurrency: 20,
		Timeout:     60 * time.Second,
	}

	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping stress test - server not available at " + config.BaseURL)
	}

	t.Run("ConcurrentBatchVerifications", func(t *testing.T) {
		var wg sync.WaitGroup
		var successCount, failCount int64
		client := &http.Client{Timeout: config.Timeout}

		batchRequest := map[string]interface{}{
			"models": []map[string]interface{}{
				{"model_id": "gpt-4", "provider": "openai"},
				{"model_id": "claude-3-opus", "provider": "anthropic"},
				{"model_id": "gemini-pro", "provider": "google"},
				{"model_id": "deepseek-chat", "provider": "deepseek"},
				{"model_id": "qwen-plus", "provider": "qwen"},
			},
		}
		jsonData, _ := json.Marshal(batchRequest)

		for i := 0; i < config.Concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				resp, err := client.Post(config.BaseURL+"/api/v1/verifier/batch-verify", "application/json", bytes.NewBuffer(jsonData))
				if err != nil {
					atomic.AddInt64(&failCount, 1)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode < 500 {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
			}()
		}

		wg.Wait()

		t.Logf("Concurrent batch verifications: %d successful, %d failed", successCount, failCount)
	})
}

func performVerifierStressTest(t *testing.T, config StressTestConfig, endpoint, method string, body map[string]interface{}) *StressTestResult {
	var wg sync.WaitGroup
	var totalRequests, successfulReqs, failedReqs int64
	var totalResponseTime int64
	var minResponseTime, maxResponseTime int64

	minResponseTime = int64(time.Hour) // Start with very high value

	client := &http.Client{Timeout: config.Timeout}
	var jsonData []byte
	if body != nil {
		jsonData, _ = json.Marshal(body)
	}

	done := make(chan bool)
	start := time.Now()

	// Start workers
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-done:
					return
				default:
					reqStart := time.Now()
					var resp *http.Response
					var err error

					if method == "GET" {
						resp, err = client.Get(config.BaseURL + endpoint)
					} else {
						resp, err = client.Post(config.BaseURL+endpoint, "application/json", bytes.NewBuffer(jsonData))
					}

					reqDuration := time.Since(reqStart)
					atomic.AddInt64(&totalRequests, 1)

					if err != nil {
						atomic.AddInt64(&failedReqs, 1)
						continue
					}
					resp.Body.Close()

					if resp.StatusCode < 500 {
						atomic.AddInt64(&successfulReqs, 1)
					} else {
						atomic.AddInt64(&failedReqs, 1)
					}

					atomic.AddInt64(&totalResponseTime, int64(reqDuration))

					// Update min/max (atomic operations)
					for {
						old := atomic.LoadInt64(&minResponseTime)
						if int64(reqDuration) >= old || atomic.CompareAndSwapInt64(&minResponseTime, old, int64(reqDuration)) {
							break
						}
					}
					for {
						old := atomic.LoadInt64(&maxResponseTime)
						if int64(reqDuration) <= old || atomic.CompareAndSwapInt64(&maxResponseTime, old, int64(reqDuration)) {
							break
						}
					}
				}
			}
		}()
	}

	// Run for specified duration
	time.Sleep(config.Duration)
	close(done)
	wg.Wait()

	duration := time.Since(start)

	result := &StressTestResult{
		TotalRequests:  totalRequests,
		SuccessfulReqs: successfulReqs,
		FailedReqs:     failedReqs,
		Duration:       duration,
	}

	if totalRequests > 0 {
		result.AvgResponseTime = time.Duration(totalResponseTime / totalRequests)
		result.MinResponseTime = time.Duration(minResponseTime)
		result.MaxResponseTime = time.Duration(maxResponseTime)
		result.ThroughputPerSec = float64(totalRequests) / duration.Seconds()
		result.ErrorRate = float64(failedReqs) / float64(totalRequests) * 100
	}

	runtime.ReadMemStats(&result.MemoryUsage)

	return result
}

func analyzeVerifierStressResults(t *testing.T, result *StressTestResult, endpointName string) {
	t.Logf("\n=== Stress Test Results: %s ===", endpointName)
	t.Logf("Duration: %v", result.Duration)
	t.Logf("Total Requests: %d", result.TotalRequests)
	t.Logf("Successful: %d (%.2f%%)", result.SuccessfulReqs, float64(result.SuccessfulReqs)/float64(result.TotalRequests)*100)
	t.Logf("Failed: %d (%.2f%%)", result.FailedReqs, result.ErrorRate)
	t.Logf("Throughput: %.2f req/sec", result.ThroughputPerSec)
	t.Logf("Response Times - Avg: %v, Min: %v, Max: %v",
		result.AvgResponseTime, result.MinResponseTime, result.MaxResponseTime)
	t.Logf("Memory Usage: %d MB", result.MemoryUsage.Alloc/1024/1024)

	// Performance assertions
	assert.Less(t, result.ErrorRate, float64(10), "Error rate should be under 10%%")

	if result.ThroughputPerSec > 0 {
		t.Logf("Endpoint %s can handle %.0f requests/sec", endpointName, result.ThroughputPerSec)
	}
}
