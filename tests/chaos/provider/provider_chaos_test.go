// Package provider provides chaos tests for LLM provider handling.
package provider

import (
	"bytes"
	"encoding/json"
	"math/rand"
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

// TestProviderChaos_InvalidModelIDs sends requests with random invalid model IDs
// to verify graceful error handling and fallback.
func TestProviderChaos_InvalidModelIDs(t *testing.T) {
	testutil.RequireServer(t)

	invalidModels := []string{
		"", "nonexistent-model", "gpt-999999", "invalid/model",
		"../../../etc/passwd", "<script>alert(1)</script>",
		"model_" + randStr(100), "   ", "\x00\x01\x02",
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var errorCount, processedCount int64
	var wg sync.WaitGroup

	done := make(chan struct{})
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					model := invalidModels[rand.Intn(len(invalidModels))]
					body := map[string]interface{}{
						"model":    model,
						"messages": []map[string]string{{"role": "user", "content": "test"}},
					}
					data, _ := json.Marshal(body)

					req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")

					resp, err := client.Do(req)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}
					resp.Body.Close()
					atomic.AddInt64(&processedCount, 1)
				}
			}
		}()
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Invalid model IDs: %d processed (got response), %d errors (connection)", processedCount, errorCount)
	assert.True(t, checkAvailable(baseURL), "Server must survive invalid model ID chaos")
}

// TestProviderChaos_ConcurrentProviderRequests fires concurrent requests targeting
// different providers simultaneously.
func TestProviderChaos_ConcurrentProviderRequests(t *testing.T) {
	testutil.RequireServer(t)

	providers := []string{
		"openai", "claude", "gemini", "deepseek", "mistral",
		"openrouter", "groq", "cohere", "cerebras", "helixagent",
	}

	client := &http.Client{Timeout: 15 * time.Second}
	var wg sync.WaitGroup
	var successCount, failCount int64
	done := make(chan struct{})

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					provider := providers[rand.Intn(len(providers))]
					body := map[string]interface{}{
						"model":    provider + "/gpt-4",
						"messages": []map[string]string{{"role": "user", "content": "hello"}},
					}
					data, _ := json.Marshal(body)

					req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")

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

	time.Sleep(15 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Concurrent provider requests: %d responses received, %d connection errors", successCount, failCount)
	assert.True(t, checkAvailable(baseURL), "Server must survive concurrent provider chaos")
}

// TestProviderChaos_ProviderHealthUnderLoad checks provider health endpoint
// under high concurrent load.
func TestProviderChaos_ProviderHealthUnderLoad(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	var successCount, failCount int64
	done := make(chan struct{})

	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					resp, err := client.Get(baseURL + "/v1/monitoring/status")
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

	t.Logf("Provider health under load: %d successful, %d failed", successCount, failCount)
	assert.True(t, checkAvailable(baseURL), "Server should handle health check storm")
}

// TestProviderChaos_RecoveryAfterProviderExhaustion checks that the server recovers
// after a storm of failing provider requests.
func TestProviderChaos_RecoveryAfterProviderExhaustion(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 10 * time.Second}

	// Send burst of bad provider requests
	t.Log("Sending burst of invalid provider requests...")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := map[string]interface{}{
				"model":    "nonexistent_provider_xyz/gpt-9999",
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
		}()
	}
	wg.Wait()

	// Wait for recovery
	time.Sleep(2 * time.Second)

	// Verify server is still alive
	assert.True(t, checkAvailable(baseURL), "Server should recover after provider exhaustion")

	// Verify valid endpoints still respond
	resp, err := client.Get(baseURL + "/v1/models")
	if err == nil {
		resp.Body.Close()
		t.Logf("Recovery check: models endpoint returned %d", resp.StatusCode)
		assert.Less(t, resp.StatusCode, 500, "Models endpoint should not return 5xx after recovery")
	}
}

func randStr(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
