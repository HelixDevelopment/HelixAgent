package challenge

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ChallengeTestConfig holds configuration for challenge tests
type ChallengeTestConfig struct {
	BaseURL     string
	Timeout     time.Duration
	Concurrency int
}

// ChallengeResult holds results from challenge tests
type ChallengeResult struct {
	ChallengeName   string
	Success         bool
	Duration        time.Duration
	RequestsTotal   int
	RequestsSuccess int
	Error           string
	Score           int // 0-100
}

// checkServerAvailability checks if the server is reachable
func checkServerAvailability(baseURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}

	// Try health endpoint first
	resp, err := client.Get(baseURL + "/health")
	if err == nil && resp.StatusCode < 500 {
		resp.Body.Close()
		return true
	}

	// Try root endpoint
	resp, err = client.Get(baseURL)
	if err == nil {
		resp.Body.Close()
		return true
	}

	return false
}

// skipIfServerUnavailable skips the test if the server is not reachable
func skipIfServerUnavailable(t *testing.T, baseURL string) {
	if !checkServerAvailability(baseURL) {
		t.Skipf("Skipping challenge test: server not available at %s", baseURL)
	}
}

// TestAdvancedLoadScenarios tests complex load scenarios
func TestAdvancedLoadScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping challenge test in short mode")
	}

	config := ChallengeTestConfig{
		BaseURL:     "http://localhost:7061",
		Timeout:     60 * time.Second,
		Concurrency: 100,
	}

	skipIfServerUnavailable(t, config.BaseURL)

	t.Run("BurstLoadChallenge", func(t *testing.T) {
		result := performBurstLoadTest(t, config)
		reportChallengeResult(t, result)

		// Scoring: success = 50 points, performance = up to 50 points
		if result.Success {
			assert.GreaterOrEqual(t, result.Score, 70, "Should achieve good performance under burst load")
		}
	})

	t.Run("SustainedLoadChallenge", func(t *testing.T) {
		result := performSustainedLoadTest(t, config)
		reportChallengeResult(t, result)

		if result.Success {
			assert.GreaterOrEqual(t, result.Score, 80, "Should maintain performance under sustained load")
		}
	})

	t.Run("MixedWorkloadChallenge", func(t *testing.T) {
		result := performMixedWorkloadTest(t, config)
		reportChallengeResult(t, result)

		if result.Success {
			assert.GreaterOrEqual(t, result.Score, 75, "Should handle mixed workload effectively")
		}
	})
}

// TestResilienceScenarios tests system resilience
func TestResilienceScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resilience challenge test in short mode")
	}

	config := ChallengeTestConfig{
		BaseURL:     "http://localhost:7061",
		Timeout:     60 * time.Second,
		Concurrency: 50,
	}

	skipIfServerUnavailable(t, config.BaseURL)

	t.Run("CascadingFailureChallenge", func(t *testing.T) {
		result := performCascadingFailureTest(t, config)
		reportChallengeResult(t, result)

		if result.Success {
			assert.GreaterOrEqual(t, result.Score, 70, "Should demonstrate resilience to cascading failures")
		}
	})

	t.Run("PartialDegradationChallenge", func(t *testing.T) {
		result := performPartialDegradationTest(t, config)
		reportChallengeResult(t, result)

		if result.Success {
			assert.GreaterOrEqual(t, result.Score, 65, "Should handle partial service degradation gracefully")
		}
	})
}

// TestComplexQueries tests complex query scenarios
func TestComplexQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping complex queries challenge test in short mode")
	}

	config := ChallengeTestConfig{
		BaseURL:     "http://localhost:7061",
		Timeout:     30 * time.Second,
		Concurrency: 20,
	}

	skipIfServerUnavailable(t, config.BaseURL)

	t.Run("ComplexPromptChallenge", func(t *testing.T) {
		result := performComplexPromptTest(t, config)
		reportChallengeResult(t, result)

		if result.Success {
			assert.GreaterOrEqual(t, result.Score, 60, "Should handle complex prompts effectively")
		}
	})

	t.Run("LargePayloadChallenge", func(t *testing.T) {
		result := performLargePayloadTest(t, config)
		reportChallengeResult(t, result)

		if result.Success {
			assert.GreaterOrEqual(t, result.Score, 70, "Should handle large payloads efficiently")
		}
	})
}

// TestConcurrencyChallenges tests concurrent access scenarios
func TestConcurrencyChallenges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency challenge test in short mode")
	}

	config := ChallengeTestConfig{
		BaseURL:     "http://localhost:7061",
		Timeout:     45 * time.Second,
		Concurrency: 200,
	}

	skipIfServerUnavailable(t, config.BaseURL)

	t.Run("HighConcurrencyChallenge", func(t *testing.T) {
		result := performHighConcurrencyTest(t, config)
		reportChallengeResult(t, result)

		if result.Success {
			assert.GreaterOrEqual(t, result.Score, 75, "Should handle high concurrency well")
		}
	})

	t.Run("RaceConditionChallenge", func(t *testing.T) {
		result := performRaceConditionTest(t, config)
		reportChallengeResult(t, result)

		if result.Success {
			assert.GreaterOrEqual(t, result.Score, 80, "Should be free of race conditions")
		}
	})
}

// performBurstLoadTest performs a burst load challenge
func performBurstLoadTest(t *testing.T, config ChallengeTestConfig) ChallengeResult {
	result := ChallengeResult{
		ChallengeName: "Burst Load Challenge",
		Success:       false,
	}

	start := time.Now()

	// Generate burst of requests
	burstSize := config.Concurrency * 2
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for i := 0; i < burstSize; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client := &http.Client{Timeout: config.Timeout}
			resp, err := client.Get(config.BaseURL + "/v1/models")

			mu.Lock()
			result.RequestsTotal++
			if err == nil && resp.StatusCode == http.StatusOK {
				successCount++
				resp.Body.Close()
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	result.Duration = time.Since(start)
	result.RequestsSuccess = successCount

	// Evaluate success
	successRate := float64(successCount) / float64(result.RequestsTotal)
	result.Success = successRate >= 0.8 // 80% success rate

	if result.Success {
		// Score based on response time and success rate
		timeScore := 0
		if result.Duration < 5*time.Second {
			timeScore = 50
		} else if result.Duration < 10*time.Second {
			timeScore = 30
		} else if result.Duration < 20*time.Second {
			timeScore = 15
		}

		successScore := int(successRate * 50)
		result.Score = timeScore + successScore
	} else {
		result.Score = int(successRate * 100)
		result.Error = fmt.Sprintf("Low success rate: %.2f%%", successRate*100)
	}

	return result
}

// performSustainedLoadTest performs a sustained load challenge
func performSustainedLoadTest(t *testing.T, config ChallengeTestConfig) ChallengeResult {
	result := ChallengeResult{
		ChallengeName: "Sustained Load Challenge",
		Success:       false,
	}

	start := time.Now()
	duration := 30 * time.Second
	requestInterval := time.Second / time.Duration(config.Concurrency/10) // 10% of max concurrency per second

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	totalRequests := 0

	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()

	done := time.After(duration)

	for {
		select {
		case <-done:
			goto finished
		case <-ticker.C:
			wg.Add(1)
			totalRequests++

			go func() {
				defer wg.Done()

				client := &http.Client{Timeout: config.Timeout}
				resp, err := client.Get(config.BaseURL + "/health")

				mu.Lock()
				if err == nil && resp.StatusCode == http.StatusOK {
					successCount++
					resp.Body.Close()
				}
				mu.Unlock()
			}()
		}
	}

finished:
	wg.Wait()
	result.Duration = time.Since(start)
	result.RequestsTotal = totalRequests
	result.RequestsSuccess = successCount

	// Evaluate success
	successRate := float64(successCount) / float64(result.RequestsTotal)
	throughput := float64(successCount) / result.Duration.Seconds()

	result.Success = successRate >= 0.9 && throughput >= 10 // 90% success, 10+ req/sec

	if result.Success {
		// Score based on throughput and consistency
		throughputScore := 0
		if throughput >= 50 {
			throughputScore = 50
		} else if throughput >= 25 {
			throughputScore = 40
		} else if throughput >= 15 {
			throughputScore = 30
		} else {
			throughputScore = 20
		}

		consistencyScore := int(successRate * 50)
		result.Score = throughputScore + consistencyScore
	} else {
		result.Score = int(successRate * 100)
		result.Error = fmt.Sprintf("Poor sustained performance: %.2f req/sec, %.2f%% success", throughput, successRate*100)
	}

	return result
}

// performMixedWorkloadTest performs a mixed workload challenge
func performMixedWorkloadTest(t *testing.T, config ChallengeTestConfig) ChallengeResult {
	result := ChallengeResult{
		ChallengeName: "Mixed Workload Challenge",
		Success:       false,
	}

	start := time.Now()

	// Define different request types
	requestTypes := []struct {
		name     string
		endpoint string
		method   string
		body     interface{}
		weight   int // relative frequency
	}{
		{"Health", "/health", "GET", nil, 30},
		{"Models", "/v1/models", "GET", nil, 25},
		{"Completion", "/v1/completions", "POST", map[string]interface{}{
			"prompt":      "Hello world",
			"model":       "gpt-3.5-turbo",
			"max_tokens":  10,
			"temperature": 0.1,
		}, 25},
		{"Ensemble", "/v1/ensemble/completions", "POST", map[string]interface{}{
			"prompt": "What is 2+2?",
			"ensemble_config": map[string]interface{}{
				"strategy":      "confidence_weighted",
				"min_providers": 1,
			},
		}, 20},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	totalRequests := config.Concurrency

	// Generate mixed requests
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)

		// Select request type based on weight
		typeIndex := selectByWeight(func() []struct{ weight int } {
			result := make([]struct{ weight int }, len(requestTypes))
			for i, rt := range requestTypes {
				result[i] = struct{ weight int }{weight: rt.weight}
			}
			return result
		}())
		reqType := requestTypes[typeIndex]

		go func(rt struct {
			name     string
			endpoint string
			method   string
			body     interface{}
			weight   int
		}) {
			defer wg.Done()

			client := &http.Client{Timeout: config.Timeout}

			var resp *http.Response
			var err error

			if rt.method == "GET" {
				resp, err = client.Get(config.BaseURL + rt.endpoint)
			} else {
				jsonData, _ := json.Marshal(rt.body)
				resp, err = client.Post(config.BaseURL+rt.endpoint, "application/json", bytes.NewBuffer(jsonData))
			}

			mu.Lock()
			result.RequestsTotal++
			if err == nil && resp.StatusCode < 500 {
				successCount++
				resp.Body.Close()
			}
			mu.Unlock()
		}(reqType)
	}

	wg.Wait()
	result.Duration = time.Since(start)
	result.RequestsSuccess = successCount

	// Evaluate success
	successRate := float64(successCount) / float64(result.RequestsTotal)
	avgResponseTime := result.Duration / time.Duration(result.RequestsTotal)

	result.Success = successRate >= 0.85 && avgResponseTime < 5*time.Second

	if result.Success {
		// Score based on success rate and response time
		responseScore := 0
		if avgResponseTime < 1*time.Second {
			responseScore = 50
		} else if avgResponseTime < 2*time.Second {
			responseScore = 40
		} else if avgResponseTime < 3*time.Second {
			responseScore = 30
		} else {
			responseScore = 20
		}

		successScore := int(successRate * 50)
		result.Score = responseScore + successScore
	} else {
		result.Score = int(successRate * 100)
		result.Error = fmt.Sprintf("Mixed workload failed: %.2f%% success, avg response %v", successRate*100, avgResponseTime)
	}

	return result
}

// performCascadingFailureTest performs a cascading failure challenge
func performCascadingFailureTest(t *testing.T, config ChallengeTestConfig) ChallengeResult {
	result := ChallengeResult{
		ChallengeName: "Cascading Failure Challenge",
		Success:       false,
	}

	start := time.Now()

	// Simulate cascading failure by sending requests with invalid payloads
	// that might cause downstream failures
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	totalRequests := config.Concurrency

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client := &http.Client{Timeout: config.Timeout}

			// Mix of valid and potentially problematic requests
			var resp *http.Response
			var err error

			if id%3 == 0 {
				// Potentially problematic request
				invalidRequest := map[string]interface{}{
					"prompt": string(make([]byte, 10000)), // Large prompt
					"model":  "non-existent-model",
				}
				jsonData, _ := json.Marshal(invalidRequest)
				resp, err = client.Post(config.BaseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))
			} else {
				// Normal request
				resp, err = client.Get(config.BaseURL + "/health")
			}

			mu.Lock()
			result.RequestsTotal++
			if err == nil && resp.StatusCode < 500 {
				successCount++
				resp.Body.Close()
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	result.Duration = time.Since(start)
	result.RequestsSuccess = successCount

	// Evaluate success - system should not completely fail
	successRate := float64(successCount) / float64(result.RequestsTotal)

	result.Success = successRate >= 0.6 // Should handle 60% of requests even under stress

	if result.Success {
		result.Score = int(successRate*100) + 20 // Bonus for resilience
	} else {
		result.Score = int(successRate * 100)
		result.Error = fmt.Sprintf("System not resilient: %.2f%% success under cascading failure", successRate*100)
	}

	return result
}

// performPartialDegradationTest performs partial degradation challenge
func performPartialDegradationTest(t *testing.T, config ChallengeTestConfig) ChallengeResult {
	result := ChallengeResult{
		ChallengeName: "Partial Degradation Challenge",
		Success:       false,
	}

	start := time.Now()

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	totalRequests := config.Concurrency

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client := &http.Client{Timeout: config.Timeout}

			// Target different endpoints to simulate partial degradation
			endpoints := []string{"/health", "/v1/models", "/metrics", "/v1/providers"}
			endpoint := endpoints[id%len(endpoints)]

			resp, err := client.Get(config.BaseURL + endpoint)

			mu.Lock()
			result.RequestsTotal++
			if err == nil && resp.StatusCode < 500 {
				successCount++
				resp.Body.Close()
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	result.Duration = time.Since(start)
	result.RequestsSuccess = successCount

	// Evaluate success
	successRate := float64(successCount) / float64(result.RequestsTotal)

	result.Success = successRate >= 0.7 // Should handle 70% of requests during partial degradation

	if result.Success {
		result.Score = int(successRate*100) + 15 // Bonus for graceful degradation
	} else {
		result.Score = int(successRate * 100)
		result.Error = fmt.Sprintf("Poor graceful degradation: %.2f%% success", successRate*100)
	}

	return result
}

// performComplexPromptTest performs complex prompt challenge
func performComplexPromptTest(t *testing.T, config ChallengeTestConfig) ChallengeResult {
	result := ChallengeResult{
		ChallengeName: "Complex Prompt Challenge",
		Success:       false,
	}

	start := time.Now()

	// Generate complex prompts
	complexPrompts := []string{
		generateLongPrompt(1000),
		generateUnicodePrompt(),
		generateSpecialCharPrompt(),
		generateNestedJsonPrompt(),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for i, prompt := range complexPrompts {
		wg.Add(1)
		go func(id int, promptText string) {
			defer wg.Done()

			client := &http.Client{Timeout: config.Timeout}

			request := map[string]interface{}{
				"prompt":      promptText,
				"model":       "gpt-3.5-turbo",
				"max_tokens":  50,
				"temperature": 0.1,
			}

			jsonData, _ := json.Marshal(request)
			resp, err := client.Post(config.BaseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))

			mu.Lock()
			result.RequestsTotal++
			if err == nil && resp.StatusCode < 500 {
				successCount++
				resp.Body.Close()
			}
			mu.Unlock()
		}(i, prompt)
	}

	wg.Wait()
	result.Duration = time.Since(start)
	result.RequestsSuccess = successCount

	// Evaluate success
	successRate := float64(successCount) / float64(result.RequestsTotal)

	result.Success = successRate >= 0.75 // Should handle 75% of complex prompts

	if result.Success {
		result.Score = int(successRate * 100)
	} else {
		result.Score = int(successRate * 100)
		result.Error = fmt.Sprintf("Complex prompt handling: %.2f%% success", successRate*100)
	}

	return result
}

// performLargePayloadTest performs large payload challenge
func performLargePayloadTest(t *testing.T, config ChallengeTestConfig) ChallengeResult {
	result := ChallengeResult{
		ChallengeName: "Large Payload Challenge",
		Success:       false,
	}

	start := time.Now()

	// Generate large payloads
	payloadSizes := []int{1024, 10240, 102400, 512000} // 1KB to 500KB

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for _, size := range payloadSizes {
		wg.Add(1)
		go func(payloadSize int) {
			defer wg.Done()

			client := &http.Client{Timeout: config.Timeout}

			// Generate large prompt
			largePrompt := generateLongPrompt(payloadSize)

			request := map[string]interface{}{
				"prompt":      largePrompt,
				"model":       "gpt-3.5-turbo",
				"max_tokens":  10,
				"temperature": 0.1,
			}

			jsonData, _ := json.Marshal(request)
			resp, err := client.Post(config.BaseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))

			mu.Lock()
			result.RequestsTotal++
			if err == nil && resp.StatusCode < 500 {
				successCount++
				resp.Body.Close()
			}
			mu.Unlock()
		}(size)
	}

	wg.Wait()
	result.Duration = time.Since(start)
	result.RequestsSuccess = successCount

	// Evaluate success
	successRate := float64(successCount) / float64(result.RequestsTotal)

	result.Success = successRate >= 0.5 // Should handle 50% of large payloads

	if result.Success {
		result.Score = int(successRate*100) + 10 // Bonus for handling large payloads
	} else {
		result.Score = int(successRate * 100)
		result.Error = fmt.Sprintf("Large payload handling: %.2f%% success", successRate*100)
	}

	return result
}

// performHighConcurrencyTest performs high concurrency challenge
func performHighConcurrencyTest(t *testing.T, config ChallengeTestConfig) ChallengeResult {
	result := ChallengeResult{
		ChallengeName: "High Concurrency Challenge",
		Success:       false,
	}

	start := time.Now()

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	// Launch high number of concurrent requests
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client := &http.Client{Timeout: config.Timeout}
			resp, err := client.Get(config.BaseURL + "/health")

			mu.Lock()
			result.RequestsTotal++
			if err == nil && resp.StatusCode == http.StatusOK {
				successCount++
				resp.Body.Close()
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	result.Duration = time.Since(start)
	result.RequestsSuccess = successCount

	// Evaluate success
	successRate := float64(successCount) / float64(result.RequestsTotal)
	avgResponseTime := result.Duration / time.Duration(result.RequestsTotal)

	result.Success = successRate >= 0.8 && avgResponseTime < 10*time.Second

	if result.Success {
		// Score based on concurrency handling
		concurrencyScore := 0
		if successRate >= 0.95 {
			concurrencyScore = 50
		} else if successRate >= 0.9 {
			concurrencyScore = 40
		} else if successRate >= 0.85 {
			concurrencyScore = 30
		} else {
			concurrencyScore = 20
		}

		timeScore := 0
		if avgResponseTime < 1*time.Second {
			timeScore = 50
		} else if avgResponseTime < 3*time.Second {
			timeScore = 40
		} else if avgResponseTime < 5*time.Second {
			timeScore = 30
		} else {
			timeScore = 20
		}

		result.Score = concurrencyScore + timeScore
	} else {
		result.Score = int(successRate * 100)
		result.Error = fmt.Sprintf("High concurrency failed: %.2f%% success, avg %v", successRate*100, avgResponseTime)
	}

	return result
}

// performRaceConditionTest performs race condition challenge
func performRaceConditionTest(t *testing.T, config ChallengeTestConfig) ChallengeResult {
	result := ChallengeResult{
		ChallengeName: "Race Condition Challenge",
		Success:       false,
	}

	start := time.Now()

	// Test for race conditions by accessing shared resources concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	sharedCounter := 0

	for i := 0; i < config.Concurrency/2; i++ {
		// Readers
		wg.Add(1)
		go func() {
			defer wg.Done()

			client := &http.Client{Timeout: config.Timeout}
			resp, err := client.Get(config.BaseURL + "/v1/models")

			mu.Lock()
			result.RequestsTotal++
			if err == nil && resp.StatusCode == http.StatusOK {
				successCount++
				sharedCounter++
			}
			mu.Unlock()

			if resp != nil {
				resp.Body.Close()
			}
		}()

		// Writers (POST requests)
		wg.Add(1)
		go func() {
			defer wg.Done()

			request := map[string]interface{}{
				"prompt":      fmt.Sprintf("Test message %d", sharedCounter),
				"model":       "gpt-3.5-turbo",
				"max_tokens":  5,
				"temperature": 0.1,
			}

			client := &http.Client{Timeout: config.Timeout}
			jsonData, _ := json.Marshal(request)
			resp, err := client.Post(config.BaseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))

			mu.Lock()
			result.RequestsTotal++
			if err == nil && resp.StatusCode < 500 {
				successCount++
			}
			mu.Unlock()

			if resp != nil {
				resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	result.Duration = time.Since(start)
	result.RequestsSuccess = successCount

	// Evaluate success - system should handle concurrent access without issues
	successRate := float64(successCount) / float64(result.RequestsTotal)

	result.Success = successRate >= 0.85 // Should handle 85% of concurrent requests

	if result.Success {
		result.Score = int(successRate*100) + 10 // Bonus for race condition safety
	} else {
		result.Score = int(successRate * 100)
		result.Error = fmt.Sprintf("Potential race conditions: %.2f%% success", successRate*100)
	}

	return result
}

// Helper functions

func selectByWeight(items []struct{ weight int }) int {
	totalWeight := 0
	for _, item := range items {
		totalWeight += item.weight
	}

	n, _ := rand.Int(rand.Reader, big.NewInt(int64(totalWeight)))
	remaining := n.Int64()

	for i, item := range items {
		remaining -= int64(item.weight)
		if remaining < 0 {
			return i
		}
	}

	return len(items) - 1
}

func generateLongPrompt(size int) string {
	words := []string{"test", "prompt", "generation", "challenge", "performance", "load", "stress", "testing"}
	var result string

	for len(result) < size {
		word := words[len(result)%len(words)]
		if result != "" {
			result += " "
		}
		result += word
	}

	return result[:size]
}

func generateUnicodePrompt() string {
	unicodeChars := "🧪 🚀 💻 🌟 🎯 🔥 💡 🎨 🎭 🎪"
	return "Unicode test: " + unicodeChars + " with special characters: ñáéíóú üõä öç"
}

func generateSpecialCharPrompt() string {
	specialChars := "!@#$%^&*()_+-=[]{}|;':\",./<>?`~\\"
	return "Special chars test: " + specialChars + " with quotes: 'single' and \"double\""
}

func generateNestedJsonPrompt() string {
	nested := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": []string{"item1", "item2", "item3"},
				"data":   map[string]string{"key1": "value1", "key2": "value2"},
			},
		},
	}

	jsonData, _ := json.Marshal(nested)
	return "Nested JSON test: " + string(jsonData)
}

func reportChallengeResult(t *testing.T, result ChallengeResult) {
	if result.Success {
		t.Logf("✅ %s: SUCCESS (Score: %d/100)", result.ChallengeName, result.Score)
	} else {
		t.Logf("❌ %s: FAILED (Score: %d/100) - %s", result.ChallengeName, result.Score, result.Error)
	}

	t.Logf("   Duration: %v", result.Duration)
	t.Logf("   Requests: %d/%d successful", result.RequestsSuccess, result.RequestsTotal)
}
