package stress

import (
	"bytes"
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
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
	TotalRequests    int
	SuccessfulReqs   int
	FailedReqs       int
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

// TestHTTPStress performs HTTP stress testing
func TestHTTPStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	config := StressTestConfig{
		BaseURL:     "http://localhost:7061",
		Concurrency: 50,
		Duration:    30 * time.Second,
		RequestRate: 100, // 100 req/sec
		Timeout:     30 * time.Second,
	}

	// Skip if server is not available
	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping stress test - server not available at " + config.BaseURL)
	}

	t.Run("APIEndpointStress", func(t *testing.T) {
		result := performStressTest(t, config, "/v1/models", "GET", nil)
		analyzeStressResults(t, result, "Models Endpoint")
	})

	t.Run("CompletionEndpointStress", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"prompt":      "Hello world",
			"model":       "gpt-3.5-turbo",
			"max_tokens":  10,
			"temperature": 0.1,
		}

		result := performStressTest(t, config, "/v1/completions", "POST", requestBody)
		analyzeStressResults(t, result, "Completion Endpoint")
	})

	t.Run("EnsembleEndpointStress", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"prompt": "What is 2+2?",
			"ensemble_config": map[string]interface{}{
				"strategy":      "confidence_weighted",
				"min_providers": 1,
			},
		}

		result := performStressTest(t, config, "/v1/ensemble/completions", "POST", requestBody)
		analyzeStressResults(t, result, "Ensemble Endpoint")
	})

	t.Run("HealthEndpointStress", func(t *testing.T) {
		result := performStressTest(t, config, "/health", "GET", nil)
		analyzeStressResults(t, result, "Health Endpoint")
	})
}

// TestMemoryStress performs memory stress testing
func TestMemoryStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory stress test in short mode")
	}

	config := StressTestConfig{
		BaseURL:     "http://localhost:7061",
		Concurrency: 100,
		Duration:    60 * time.Second,
		RequestRate: 200, // 200 req/sec
		Timeout:     30 * time.Second,
	}

	// Skip if server is not available
	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping stress test - server not available at " + config.BaseURL)
	}

	t.Run("MemoryLeakDetection", func(t *testing.T) {
		var memStats []runtime.MemStats

		// Baseline memory measurement
		var baseline runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&baseline)
		memStats = append(memStats, baseline)

		// Run stress test
		result := performStressTest(t, config, "/v1/models", "GET", nil)

		// Post-test memory measurement
		var final runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&final)
		memStats = append(memStats, final)

		// Analyze memory usage
		memoryIncrease := final.Alloc - baseline.Alloc
		memoryIncreaseMB := float64(memoryIncrease) / 1024 / 1024

		t.Logf("Memory analysis:")
		t.Logf("  Baseline: %.2f MB", float64(baseline.Alloc)/1024/1024)
		t.Logf("  Final: %.2f MB", float64(final.Alloc)/1024/1024)
		t.Logf("  Increase: %.2f MB", memoryIncreaseMB)

		// Memory increase should be reasonable (less than 100MB for this test)
		assert.Less(t, memoryIncreaseMB, 100.0, "Memory increase should be reasonable")

		t.Logf("✅ Memory stress test completed: %v total requests", result.TotalRequests)
	})
}

// TestConnectionStress performs connection stress testing
func TestConnectionStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping connection stress test in short mode")
	}

	baseURL := "http://localhost:7061"
	// Skip if server is not available
	if !checkServerAvailable(baseURL, 5*time.Second) {
		t.Skip("Skipping stress test - server not available at " + baseURL)
	}

	t.Run("MaxConnections", func(t *testing.T) {
		maxConnections := 1000
		var wg sync.WaitGroup
		var mu sync.Mutex
		successful := 0
		failed := 0

		start := time.Now()

		for i := 0; i < maxConnections; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				client := &http.Client{
					Timeout: 10 * time.Second,
				}

				resp, err := client.Get(baseURL + "/health")
				mu.Lock()
				if err == nil && resp.StatusCode == http.StatusOK {
					successful++
					resp.Body.Close()
				} else {
					failed++
				}
				mu.Unlock()
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		successRate := float64(successful) / float64(maxConnections) * 100
		t.Logf("Connection stress test results:")
		t.Logf("  Total connections: %d", maxConnections)
		t.Logf("  Successful: %d", successful)
		t.Logf("  Failed: %d", failed)
		t.Logf("  Success rate: %.2f%%", successRate)
		t.Logf("  Duration: %v", duration)

		// Should handle at least 80% of connections
		assert.GreaterOrEqual(t, successRate, 80.0, "Should handle at least 80%% of connections")
	})

	t.Run("ConnectionReuse", func(t *testing.T) {
		baseURL := "http://localhost:7061"
		iterations := 100

		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		start := time.Now()
		successful := 0

		for i := 0; i < iterations; i++ {
			resp, err := client.Get(baseURL + "/health")
			if err == nil && resp.StatusCode == http.StatusOK {
				successful++
				resp.Body.Close()
			}
		}

		duration := time.Since(start)
		avgTime := duration / time.Duration(iterations)

		t.Logf("Connection reuse test results:")
		t.Logf("  Iterations: %d", iterations)
		t.Logf("  Successful: %d", successful)
		t.Logf("  Total duration: %v", duration)
		t.Logf("  Average per request: %v", avgTime)

		// Connection reuse should improve performance
		assert.Less(t, avgTime, 100*time.Millisecond, "Average request time should be low with connection reuse")
	})
}

// TestLoadGradual performs gradual load increase testing
func TestLoadGradual(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping gradual load test in short mode")
	}

	// Skip if server is not available
	if !checkServerAvailable("http://localhost:7061", 5*time.Second) {
		t.Skip("Skipping stress test - server not available")
	}

	stages := []struct {
		concurrency int
		duration    time.Duration
		name        string
	}{
		{10, 10 * time.Second, "Low Load"},
		{25, 10 * time.Second, "Medium Load"},
		{50, 10 * time.Second, "High Load"},
		{100, 10 * time.Second, "Peak Load"},
	}

	var results []StressTestResult

	for _, stage := range stages {
		config := StressTestConfig{
			BaseURL:     "http://localhost:7061",
			Concurrency: stage.concurrency,
			Duration:    stage.duration,
			RequestRate: stage.concurrency * 2, // 2x concurrent connections
			Timeout:     30 * time.Second,
		}

		t.Run(stage.name, func(t *testing.T) {
			result := performStressTest(t, config, "/health", "GET", nil)
			results = append(results, result)

			t.Logf("✅ %s completed: %d requests, %.2f%% success rate",
				stage.name, result.TotalRequests, (1-result.ErrorRate)*100)
		})
	}

	// Analyze performance degradation
	if len(results) > 1 {
		t.Run("PerformanceDegradationAnalysis", func(t *testing.T) {
			baselineThroughput := results[0].ThroughputPerSec
			peakThroughput := results[len(results)-1].ThroughputPerSec

			degradation := (baselineThroughput - peakThroughput) / baselineThroughput * 100

			t.Logf("Performance degradation analysis:")
			t.Logf("  Baseline throughput: %.2f req/sec", baselineThroughput)
			t.Logf("  Peak throughput: %.2f req/sec", peakThroughput)
			t.Logf("  Degradation: %.2f%%", degradation)

			// Performance degradation should be reasonable
			assert.Less(t, degradation, 50.0, "Performance degradation should be less than 50%%")
		})
	}
}

// performStressTest executes a stress test with the given configuration
func performStressTest(t *testing.T, config StressTestConfig, endpoint, method string, body interface{}) StressTestResult {
	var wg sync.WaitGroup
	var mu sync.Mutex
	result := StressTestResult{
		MinResponseTime: time.Hour, // Initialize to high value
	}

	start := time.Now()

	// Request rate limiter
	rateLimiter := time.NewTicker(time.Second / time.Duration(config.RequestRate))
	defer rateLimiter.Stop()

	// Memory measurement before test
	runtime.ReadMemStats(&result.MemoryUsage)

	// Channel to collect results
	responseTimes := make(chan time.Duration, config.Concurrency*100)

	// Worker function
	worker := func() {
		defer wg.Done()

		client := &http.Client{Timeout: config.Timeout}
		reqStart := time.Now()

		var reqBody *bytes.Buffer
		if body != nil {
			jsonData, err := json.Marshal(body)
			if err != nil {
				mu.Lock()
				result.FailedReqs++
				mu.Unlock()
				return
			}
			reqBody = bytes.NewBuffer(jsonData)
		} else {
			reqBody = bytes.NewBuffer(nil)
		}

		req, err := http.NewRequest(method, config.BaseURL+endpoint, reqBody)
		if err != nil {
			mu.Lock()
			result.FailedReqs++
			mu.Unlock()
			return
		}

		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(req)
		responseTime := time.Since(reqStart)

		mu.Lock()
		result.TotalRequests++
		if err == nil {
			if resp.StatusCode < 500 { // Treat 5xx as failures
				result.SuccessfulReqs++
				responseTimes <- responseTime
				if responseTime < result.MinResponseTime {
					result.MinResponseTime = responseTime
				}
				if responseTime > result.MaxResponseTime {
					result.MaxResponseTime = responseTime
				}
			} else {
				result.FailedReqs++
			}
			resp.Body.Close()
		} else {
			result.FailedReqs++
		}
		mu.Unlock()
	}

	// Run stress test for specified duration
	timeout := time.After(config.Duration)

	for {
		select {
		case <-timeout:
			goto done
		case <-rateLimiter.C:
			if result.TotalRequests >= config.Concurrency*100 { // Safeguard
				continue
			}

			wg.Add(1)
			go worker()
		}
	}

done:
	wg.Wait()
	close(responseTimes)

	// Calculate final statistics
	result.Duration = time.Since(start)

	// Calculate average response time
	var totalTime time.Duration
	count := 0
	for rt := range responseTimes {
		totalTime += rt
		count++
	}

	if count > 0 {
		result.AvgResponseTime = totalTime / time.Duration(count)
	}

	// Calculate throughput and error rate
	if result.Duration.Seconds() > 0 {
		result.ThroughputPerSec = float64(result.TotalRequests) / result.Duration.Seconds()
	}
	if result.TotalRequests > 0 {
		result.ErrorRate = float64(result.FailedReqs) / float64(result.TotalRequests)
	}

	return result
}

// analyzeStressResults analyzes and reports stress test results
func analyzeStressResults(t *testing.T, result StressTestResult, testName string) {
	t.Logf("%s Stress Test Results:", testName)
	t.Logf("  Total Requests: %d", result.TotalRequests)
	t.Logf("  Successful: %d", result.SuccessfulReqs)
	t.Logf("  Failed: %d", result.FailedReqs)
	t.Logf("  Success Rate: %.2f%%", (1-result.ErrorRate)*100)
	t.Logf("  Throughput: %.2f req/sec", result.ThroughputPerSec)
	t.Logf("  Avg Response Time: %v", result.AvgResponseTime)
	t.Logf("  Min Response Time: %v", result.MinResponseTime)
	t.Logf("  Max Response Time: %v", result.MaxResponseTime)
	t.Logf("  Duration: %v", result.Duration)

	// Assertions for production readiness
	assert.Greater(t, result.TotalRequests, 0, "Should handle some requests")
	assert.Greater(t, result.ThroughputPerSec, 10.0, "Should handle at least 10 req/sec")
	assert.Less(t, result.ErrorRate, 0.5, "Error rate should be less than 50%")

	if result.AvgResponseTime > 0 {
		assert.Less(t, result.AvgResponseTime, 30*time.Second, "Average response time should be reasonable")
	}
}
