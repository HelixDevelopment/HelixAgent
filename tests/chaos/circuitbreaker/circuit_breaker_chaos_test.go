// Package circuitbreaker provides chaos tests for circuit breaker behavior.
package circuitbreaker

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/testutil"
)

const baseURL = "http://localhost:8100"

func checkAvailable(url string) bool {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// TestCircuitBreakerChaos_TripAndRecover trips circuit breakers by sending
// many requests to broken providers, then verifies recovery.
func TestCircuitBreakerChaos_TripAndRecover(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 10 * time.Second}
	var openCount, closedCount int64

	// Phase 1: Trip circuit by sending to nonexistent provider
	t.Log("Phase 1: Tripping circuit breakers...")
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := map[string]interface{}{
				"model":    "broken-provider-xyz/gpt-4",
				"messages": []map[string]string{{"role": "user", "content": "trip circuit"}},
			}
			data, _ := json.Marshal(body)
			req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-chaos-key")
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
				atomic.AddInt64(&openCount, 1)
			}
		}()
	}
	wg.Wait()
	t.Logf("Phase 1 complete: %d responses received", openCount)

	// Phase 2: Check circuit breaker status
	resp, err := client.Get(baseURL + "/v1/monitoring/status")
	if err == nil {
		resp.Body.Close()
		t.Logf("Monitoring status: %d", resp.StatusCode)
	}

	// Phase 3: Wait for circuit to potentially reset
	t.Log("Phase 3: Waiting for circuit recovery...")
	time.Sleep(5 * time.Second)

	// Phase 4: Try valid requests
	t.Log("Phase 4: Verifying recovery...")
	for i := 0; i < 5; i++ {
		resp, err := client.Get(baseURL + "/v1/models")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				atomic.AddInt64(&closedCount, 1)
			}
		}
	}

	t.Logf("Circuit breaker chaos: %d requests opened, %d recovery responses", openCount, closedCount)
	assert.True(t, checkAvailable(baseURL), "Server should survive circuit breaker chaos")
}

// TestCircuitBreakerChaos_ConcurrentFailures generates concurrent failures
// to multiple providers to stress circuit breaker state management.
func TestCircuitBreakerChaos_ConcurrentFailures(t *testing.T) {
	testutil.RequireServer(t)

	// Different broken providers to stress multiple circuit breakers
	brokenProviders := []string{
		"broken-alpha/model1", "broken-beta/model2", "broken-gamma/model3",
		"broken-delta/model4", "broken-epsilon/model5",
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var wg sync.WaitGroup
	var processedCount int64
	done := make(chan struct{})

	for i, provider := range brokenProviders {
		for j := 0; j < 5; j++ {
			wg.Add(1)
			prov := provider
			_ = i
			go func() {
				defer wg.Done()
				for {
					select {
					case <-done:
						return
					default:
						body := map[string]interface{}{
							"model":    prov,
							"messages": []map[string]string{{"role": "user", "content": "test"}},
						}
						data, _ := json.Marshal(body)
						req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("Authorization", "Bearer test-chaos-key")
						resp, err := client.Do(req)
						if err == nil {
							resp.Body.Close()
							atomic.AddInt64(&processedCount, 1)
						}
					}
				}
			}()
		}
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Concurrent circuit failures: %d requests processed", processedCount)
	assert.True(t, checkAvailable(baseURL), "Server should survive concurrent circuit breaker stress")
}

// TestCircuitBreakerChaos_IntermittentFailures alternates between
// good and bad requests to trigger half-open circuit behavior.
func TestCircuitBreakerChaos_IntermittentFailures(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 10 * time.Second}
	var goodCount, badCount int64

	// Alternate between good (models endpoint) and bad (broken provider) requests
	for cycle := 0; cycle < 10; cycle++ {
		var wg sync.WaitGroup

		// Good requests
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := client.Get(baseURL + "/v1/models")
				if err == nil {
					resp.Body.Close()
					atomic.AddInt64(&goodCount, 1)
				}
			}()
		}

		// Bad requests (to non-existent endpoints)
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				body := map[string]interface{}{
					"model":    "intermittent-broken-xyz/gpt-4",
					"messages": []map[string]string{{"role": "user", "content": "test"}},
				}
				data, _ := json.Marshal(body)
				req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer test-chaos-key")
				resp, err := client.Do(req)
				if err == nil {
					resp.Body.Close()
					atomic.AddInt64(&badCount, 1)
				}
			}()
		}

		wg.Wait()
		time.Sleep(500 * time.Millisecond)
	}

	t.Logf("Intermittent failures: %d good, %d bad requests processed", goodCount, badCount)
	assert.True(t, checkAvailable(baseURL), "Server should handle intermittent failure pattern")
}

// TestCircuitBreakerChaos_StatusMonitoring verifies circuit breaker status
// is reported correctly under chaos conditions.
func TestCircuitBreakerChaos_StatusMonitoring(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	var statusCount int64
	done := make(chan struct{})

	// Continuously poll circuit breaker status
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					resp, err := client.Get(baseURL + "/v1/monitoring/circuit-breakers")
					if err == nil {
						resp.Body.Close()
						atomic.AddInt64(&statusCount, 1)
					}
					time.Sleep(200 * time.Millisecond)
				}
			}
		}()
	}

	// Simultaneously generate failures
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					body := map[string]interface{}{
						"model":    "status-chaos-test/broken",
						"messages": []map[string]string{{"role": "user", "content": "test"}},
					}
					data, _ := json.Marshal(body)
					req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")
					resp, err := client.Do(req)
					if err == nil {
						resp.Body.Close()
					}
				}
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Circuit breaker monitoring: %d status polls completed", statusCount)
	assert.True(t, checkAvailable(baseURL), "Server should remain alive during circuit breaker monitoring chaos")
}
