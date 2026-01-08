package verifier

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ChaosTestConfig holds configuration for chaos tests
type ChaosTestConfig struct {
	BaseURL     string
	Timeout     time.Duration
	Concurrency int
	Duration    time.Duration
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

// TestVerifierChaos performs chaos testing on verifier service
func TestVerifierChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	config := ChaosTestConfig{
		BaseURL:     "http://localhost:7061",
		Timeout:     30 * time.Second,
		Concurrency: 50,
		Duration:    20 * time.Second,
	}

	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping chaos test - server not available at " + config.BaseURL)
	}

	t.Run("RandomRequestPatterns", func(t *testing.T) {
		runRandomRequestPatterns(t, config)
	})

	t.Run("SlowClientSimulation", func(t *testing.T) {
		runSlowClientSimulation(t, config)
	})

	t.Run("ConnectionDraining", func(t *testing.T) {
		runConnectionDraining(t, config)
	})

	t.Run("MalformedRequestsStorm", func(t *testing.T) {
		runMalformedRequestsStorm(t, config)
	})

	t.Run("PartialRequestSimulation", func(t *testing.T) {
		runPartialRequestSimulation(t, config)
	})

	t.Run("RecoveryAfterFailure", func(t *testing.T) {
		runRecoveryAfterFailure(t, config)
	})
}

// runRandomRequestPatterns sends random combinations of requests
func runRandomRequestPatterns(t *testing.T, config ChaosTestConfig) {
	var wg sync.WaitGroup
	var successCount, failCount int64
	client := &http.Client{Timeout: config.Timeout}

	requestTypes := []func() (*http.Request, error){
		// Valid verify request
		func() (*http.Request, error) {
			body := map[string]interface{}{
				"model_id": "gpt-4",
				"provider": "openai",
			}
			jsonData, _ := json.Marshal(body)
			return http.NewRequest("POST", config.BaseURL+"/api/v1/verifier/verify", bytes.NewBuffer(jsonData))
		},
		// Valid health check
		func() (*http.Request, error) {
			return http.NewRequest("GET", config.BaseURL+"/api/v1/verifier/health", nil)
		},
		// Valid score request
		func() (*http.Request, error) {
			return http.NewRequest("GET", config.BaseURL+"/api/v1/verifier/scores/gpt-4", nil)
		},
		// Invalid model ID
		func() (*http.Request, error) {
			body := map[string]interface{}{
				"model_id": "",
				"provider": "openai",
			}
			jsonData, _ := json.Marshal(body)
			return http.NewRequest("POST", config.BaseURL+"/api/v1/verifier/verify", bytes.NewBuffer(jsonData))
		},
		// Invalid provider
		func() (*http.Request, error) {
			body := map[string]interface{}{
				"model_id": "gpt-4",
				"provider": "invalid-provider-xyz",
			}
			jsonData, _ := json.Marshal(body)
			return http.NewRequest("POST", config.BaseURL+"/api/v1/verifier/verify", bytes.NewBuffer(jsonData))
		},
		// Non-existent endpoint
		func() (*http.Request, error) {
			return http.NewRequest("GET", config.BaseURL+"/api/v1/verifier/nonexistent", nil)
		},
	}

	done := make(chan bool)
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-done:
					return
				default:
					// Pick random request type
					reqFunc := requestTypes[rand.Intn(len(requestTypes))]
					req, err := reqFunc()
					if err != nil {
						atomic.AddInt64(&failCount, 1)
						continue
					}

					if req.Method == "POST" {
						req.Header.Set("Content-Type", "application/json")
					}

					resp, err := client.Do(req)
					if err != nil {
						atomic.AddInt64(&failCount, 1)
						continue
					}
					resp.Body.Close()

					atomic.AddInt64(&successCount, 1)
				}
			}
		}()
	}

	time.Sleep(config.Duration)
	close(done)
	wg.Wait()

	t.Logf("Random patterns: %d successful, %d failed", successCount, failCount)
	assert.Greater(t, successCount, int64(0), "Should have some successful requests")
}

// runSlowClientSimulation simulates slow/laggy clients
func runSlowClientSimulation(t *testing.T, config ChaosTestConfig) {
	var wg sync.WaitGroup
	var successCount, timeoutCount int64

	slowClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 2 * time.Second,
		},
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-done:
					return
				default:
					// Add random delay before each request
					time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

					body := map[string]interface{}{
						"model_id": "gpt-4",
						"provider": "openai",
					}
					jsonData, _ := json.Marshal(body)

					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					req, _ := http.NewRequestWithContext(ctx, "POST", config.BaseURL+"/api/v1/verifier/verify", bytes.NewBuffer(jsonData))
					req.Header.Set("Content-Type", "application/json")

					resp, err := slowClient.Do(req)
					cancel()

					if err != nil {
						atomic.AddInt64(&timeoutCount, 1)
						continue
					}

					// Slow read of response
					time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()

					atomic.AddInt64(&successCount, 1)
				}
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Slow clients: %d successful, %d timeouts", successCount, timeoutCount)
}

// runConnectionDraining tests behavior when connections are abruptly closed
func runConnectionDraining(t *testing.T, config ChaosTestConfig) {
	var wg sync.WaitGroup
	var successCount, failCount int64

	done := make(chan bool)
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-done:
					return
				default:
					// Create new client for each request to simulate connection churn
					client := &http.Client{
						Timeout: 5 * time.Second,
						Transport: &http.Transport{
							DisableKeepAlives: true,
						},
					}

					body := map[string]interface{}{
						"model_id": "gpt-4",
						"provider": "openai",
					}
					jsonData, _ := json.Marshal(body)

					resp, err := client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer(jsonData))
					if err != nil {
						atomic.AddInt64(&failCount, 1)
						continue
					}
					resp.Body.Close()

					atomic.AddInt64(&successCount, 1)
				}
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Connection draining: %d successful, %d failed", successCount, failCount)
}

// runMalformedRequestsStorm sends malformed requests
func runMalformedRequestsStorm(t *testing.T, config ChaosTestConfig) {
	var wg sync.WaitGroup
	var processedCount, errorCount int64
	client := &http.Client{Timeout: config.Timeout}

	malformedPayloads := []string{
		`{"model_id": }`,
		`{"model_id": "test", "provider": null}`,
		`{invalid json}`,
		`<xml>not json</xml>`,
		`{"model_id": ["array", "not", "string"]}`,
		`{"model_id": 12345}`,
		``,
		`null`,
		`[]`,
		`{"nested": {"deep": {"model_id": "test"}}}`,
	}

	done := make(chan bool)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-done:
					return
				default:
					payload := malformedPayloads[rand.Intn(len(malformedPayloads))]

					resp, err := client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer([]byte(payload)))
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}
					resp.Body.Close()

					// Server should handle gracefully (400/500, not crash)
					atomic.AddInt64(&processedCount, 1)
				}
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Malformed requests: %d processed, %d errors", processedCount, errorCount)

	// Server should still be alive after malformed requests
	assert.True(t, checkServerAvailable(config.BaseURL, 5*time.Second), "Server should survive malformed requests")
}

// runPartialRequestSimulation simulates requests that are aborted mid-flight
func runPartialRequestSimulation(t *testing.T, config ChaosTestConfig) {
	var wg sync.WaitGroup
	var cancelledCount, completedCount int64

	done := make(chan bool)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-done:
					return
				default:
					client := &http.Client{
						Timeout: 10 * time.Second,
					}

					body := map[string]interface{}{
						"model_id": "gpt-4",
						"provider": "openai",
					}
					jsonData, _ := json.Marshal(body)

					// Create context with random timeout
					timeout := time.Duration(rand.Intn(100)+10) * time.Millisecond
					ctx, cancel := context.WithTimeout(context.Background(), timeout)

					req, _ := http.NewRequestWithContext(ctx, "POST", config.BaseURL+"/api/v1/verifier/verify", bytes.NewBuffer(jsonData))
					req.Header.Set("Content-Type", "application/json")

					resp, err := client.Do(req)
					cancel()

					if err != nil {
						atomic.AddInt64(&cancelledCount, 1)
						continue
					}
					resp.Body.Close()

					atomic.AddInt64(&completedCount, 1)
				}
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Partial requests: %d completed, %d cancelled", completedCount, cancelledCount)

	// Server should still be alive
	assert.True(t, checkServerAvailable(config.BaseURL, 5*time.Second), "Server should survive partial requests")
}

// runRecoveryAfterFailure tests server recovery after induced failures
func runRecoveryAfterFailure(t *testing.T, config ChaosTestConfig) {
	client := &http.Client{Timeout: config.Timeout}

	// First, send a burst of bad requests
	t.Log("Sending burst of bad requests...")
	for i := 0; i < 100; i++ {
		client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer([]byte(`{invalid}`)))
	}

	// Wait a moment
	time.Sleep(2 * time.Second)

	// Check recovery - server should still respond normally
	t.Log("Checking recovery...")
	successCount := 0
	for i := 0; i < 10; i++ {
		body := map[string]interface{}{
			"model_id": "gpt-4",
			"provider": "openai",
		}
		jsonData, _ := json.Marshal(body)

		resp, err := client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer(jsonData))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				successCount++
			}
		}
	}

	t.Logf("Recovery check: %d/10 requests succeeded", successCount)
	assert.Greater(t, successCount, 0, "Server should recover and process valid requests")
}

// TestVerifierCircuitBreaker tests circuit breaker behavior
func TestVerifierCircuitBreaker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	config := ChaosTestConfig{
		BaseURL: "http://localhost:7061",
		Timeout: 30 * time.Second,
	}

	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping chaos test - server not available at " + config.BaseURL)
	}

	t.Run("CircuitBreakerTrip", func(t *testing.T) {
		client := &http.Client{Timeout: config.Timeout}

		// Send requests to non-configured provider to trip circuit breaker
		for i := 0; i < 20; i++ {
			body := map[string]interface{}{
				"model_id": "nonexistent-model",
				"provider": "nonexistent-provider",
			}
			jsonData, _ := json.Marshal(body)
			resp, err := client.Post(config.BaseURL+"/api/v1/verifier/verify", "application/json", bytes.NewBuffer(jsonData))
			if err == nil {
				resp.Body.Close()
			}
		}

		// Check provider health to see if circuit breaker was triggered
		resp, err := client.Get(config.BaseURL + "/api/v1/verifier/health/providers")
		if err == nil {
			defer resp.Body.Close()
			var health map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&health)
			t.Logf("Provider health after failures: %+v", health)
		}
	})
}

// TestVerifierResourceExhaustion tests behavior under resource pressure
func TestVerifierResourceExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	config := ChaosTestConfig{
		BaseURL:     "http://localhost:7061",
		Timeout:     30 * time.Second,
		Concurrency: 100,
	}

	if !checkServerAvailable(config.BaseURL, 5*time.Second) {
		t.Skip("Skipping chaos test - server not available at " + config.BaseURL)
	}

	t.Run("MaxConnections", func(t *testing.T) {
		var wg sync.WaitGroup
		var successCount, failCount int64
		connections := make([]*http.Response, 0, config.Concurrency)
		var mu sync.Mutex

		// Try to open many connections simultaneously
		for i := 0; i < config.Concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				client := &http.Client{
					Timeout: 5 * time.Second,
					Transport: &http.Transport{
						DisableKeepAlives: false,
						MaxIdleConns:      1,
					},
				}

				resp, err := client.Get(config.BaseURL + "/api/v1/verifier/health")
				if err != nil {
					atomic.AddInt64(&failCount, 1)
					return
				}

				mu.Lock()
				connections = append(connections, resp)
				mu.Unlock()

				atomic.AddInt64(&successCount, 1)
			}()
		}

		wg.Wait()

		t.Logf("Max connections test: %d successful, %d failed", successCount, failCount)

		// Close all connections
		for _, conn := range connections {
			conn.Body.Close()
		}

		// Server should still be responsive
		time.Sleep(1 * time.Second)
		assert.True(t, checkServerAvailable(config.BaseURL, 5*time.Second), "Server should recover from connection exhaustion")
	})
}
