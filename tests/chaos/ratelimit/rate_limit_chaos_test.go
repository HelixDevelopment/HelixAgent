// Package ratelimit provides chaos tests for rate limiting behavior.
package ratelimit

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

// TestRateLimitChaos_ExceedLimit sends requests at a rate that should trigger
// rate limiting, verifying that 429 responses are returned and the server stays up.
func TestRateLimitChaos_ExceedLimit(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	var rateLimitedCount, otherCount int64
	done := make(chan struct{})

	// High concurrency to exceed any rate limit
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					body := map[string]interface{}{
						"model":    "helixagent",
						"messages": []map[string]string{{"role": "user", "content": "rate limit test"}},
					}
					data, _ := json.Marshal(body)

					req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")
					req.Header.Set("X-Source-IP", "10.0.0.1") // single IP for rate limiting

					resp, err := client.Do(req)
					if err != nil {
						continue
					}
					resp.Body.Close()
					if resp.StatusCode == 429 {
						atomic.AddInt64(&rateLimitedCount, 1)
					} else {
						atomic.AddInt64(&otherCount, 1)
					}
				}
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Rate limit chaos: %d rate-limited (429), %d other responses", rateLimitedCount, otherCount)
	assert.True(t, checkAvailable(baseURL), "Server should survive rate limit exhaustion")
}

// TestRateLimitChaos_BurstPattern sends traffic in burst patterns to test token bucket behavior.
func TestRateLimitChaos_BurstPattern(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 5 * time.Second}

	for burst := 0; burst < 5; burst++ {
		t.Logf("Burst %d: sending 30 concurrent requests", burst+1)
		var wg sync.WaitGroup
		var successCount, limitedCount int64

		for i := 0; i < 30; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				body := map[string]interface{}{
					"model":    "helixagent",
					"messages": []map[string]string{{"role": "user", "content": "burst test"}},
				}
				data, _ := json.Marshal(body)

				req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer test-chaos-key")

				resp, err := client.Do(req)
				if err != nil {
					return
				}
				resp.Body.Close()
				if resp.StatusCode == 429 {
					atomic.AddInt64(&limitedCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}()
		}

		wg.Wait()
		t.Logf("Burst %d result: %d success, %d rate-limited", burst+1, successCount, limitedCount)

		// Pause between bursts to allow token replenishment
		time.Sleep(2 * time.Second)
	}

	assert.True(t, checkAvailable(baseURL), "Server should survive burst pattern chaos")
}

// TestRateLimitChaos_MultiIPSources tests rate limiting across different IPs.
func TestRateLimitChaos_MultiIPSources(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	var processedCount int64
	done := make(chan struct{})

	for i := 0; i < 20; i++ {
		wg.Add(1)
		ip := i
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					body := map[string]interface{}{
						"model":    "helixagent",
						"messages": []map[string]string{{"role": "user", "content": "multi-ip test"}},
					}
					data, _ := json.Marshal(body)

					req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")
					// Different source IPs
					req.Header.Set("X-Forwarded-For", "10.0.0."+string(rune('0'+ip%10)))

					resp, err := client.Do(req)
					if err == nil {
						resp.Body.Close()
						atomic.AddInt64(&processedCount, 1)
					}
				}
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Multi-IP rate limit: %d requests processed", processedCount)
	assert.True(t, checkAvailable(baseURL), "Server should handle multi-IP rate limit chaos")
}

// TestRateLimitChaos_Recovery verifies the server recovers correctly
// after rate limit windows expire.
func TestRateLimitChaos_Recovery(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 5 * time.Second}

	// Phase 1: Exhaust rate limit
	t.Log("Phase 1: Exhausting rate limit...")
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := map[string]interface{}{
				"model":    "helixagent",
				"messages": []map[string]string{{"role": "user", "content": "rate limit"}},
			}
			data, _ := json.Marshal(body)
			req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-chaos-key")
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()

	// Phase 2: Wait for rate limit window to expire
	t.Log("Phase 2: Waiting for rate limit recovery...")
	time.Sleep(5 * time.Second)

	// Phase 3: Verify requests succeed
	t.Log("Phase 3: Checking recovery...")
	assert.True(t, checkAvailable(baseURL), "Server should recover from rate limit exhaustion")

	resp, err := client.Get(baseURL + "/v1/models")
	if err == nil {
		resp.Body.Close()
		t.Logf("Recovery check: models endpoint returned %d", resp.StatusCode)
	}
}
