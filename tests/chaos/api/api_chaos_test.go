// Package api provides chaos tests for the main HelixAgent API endpoints.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
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

// TestAPIChaos_CompletionEndpoint hammers the completion endpoint with concurrent
// valid and invalid requests to verify resilience.
func TestAPIChaos_CompletionEndpoint(t *testing.T) {
	testutil.RequireServer(t)

	var successCount, failCount int64
	var wg sync.WaitGroup
	done := make(chan struct{})

	client := &http.Client{Timeout: 10 * time.Second}

	for i := 0; i < 20; i++ {
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

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Completion endpoint: %d responses, %d errors", successCount, failCount)
	// Server must remain alive
	assert.True(t, checkAvailable(baseURL), "Server should remain responsive after chaos")
}

// TestAPIChaos_LargePayloads sends increasingly large payloads to test request size handling.
func TestAPIChaos_LargePayloads(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 15 * time.Second}
	sizes := []int{1024, 10 * 1024, 100 * 1024, 1024 * 1024}

	for _, size := range sizes {
		content := strings.Repeat("x", size)
		body := map[string]interface{}{
			"model":    "helixagent",
			"messages": []map[string]string{{"role": "user", "content": content}},
		}
		data, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-chaos-key")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Large payload (%d bytes) error: %v", size, err)
			continue
		}
		resp.Body.Close()
		t.Logf("Large payload (%d bytes): status %d", size, resp.StatusCode)
	}

	assert.True(t, checkAvailable(baseURL), "Server should survive large payloads")
}

// TestAPIChaos_ConcurrentMixedRequests sends a storm of mixed valid/invalid requests.
func TestAPIChaos_ConcurrentMixedRequests(t *testing.T) {
	testutil.RequireServer(t)

	type reqFunc func() *http.Request

	makeValid := func() *http.Request {
		body := map[string]interface{}{
			"model":    "helixagent",
			"messages": []map[string]string{{"role": "user", "content": "test"}},
		}
		data, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-chaos-key")
		return req
	}

	makeInvalidJSON := func() *http.Request {
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer([]byte(`{bad json`)))
		req.Header.Set("Content-Type", "application/json")
		return req
	}

	makeNoAuth := func() *http.Request {
		body := map[string]interface{}{"model": "helixagent"}
		data, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		return req
	}

	makeWrongMethod := func() *http.Request {
		req, _ := http.NewRequest("DELETE", baseURL+"/v1/chat/completions", nil)
		return req
	}

	requestFuncs := []reqFunc{makeValid, makeInvalidJSON, makeNoAuth, makeWrongMethod}

	var wg sync.WaitGroup
	var processedCount int64
	done := make(chan struct{})
	client := &http.Client{Timeout: 5 * time.Second}

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					fn := requestFuncs[rand.Intn(len(requestFuncs))]
					resp, err := client.Do(fn())
					if err == nil {
						resp.Body.Close()
						atomic.AddInt64(&processedCount, 1)
					}
				}
			}
		}()
	}

	time.Sleep(15 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Mixed requests: %d total processed", processedCount)
	assert.True(t, checkAvailable(baseURL), "Server should survive mixed request chaos")
}

// TestAPIChaos_ModelListChaos rapid fires model list requests.
func TestAPIChaos_ModelListChaos(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	var successCount, failCount int64
	done := make(chan struct{})

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					resp, err := client.Get(baseURL + "/v1/models")
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

	t.Logf("Model list chaos: %d successful, %d failed", successCount, failCount)
	assert.Greater(t, successCount, int64(0), "Should handle model list requests")
	assert.True(t, checkAvailable(baseURL), "Server should survive model list chaos")
}

// TestAPIChaos_ContextCancellation tests request cancellation mid-flight.
func TestAPIChaos_ContextCancellation(t *testing.T) {
	testutil.RequireServer(t)

	var cancelledCount, completedCount int64
	var wg sync.WaitGroup
	done := make(chan struct{})

	client := &http.Client{Timeout: 10 * time.Second}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					timeout := time.Duration(rand.Intn(200)+10) * time.Millisecond
					ctx, cancel := context.WithTimeout(context.Background(), timeout)

					body := map[string]interface{}{
						"model":    "helixagent",
						"messages": []map[string]string{{"role": "user", "content": fmt.Sprintf("test %d", rand.Int())}},
					}
					data, _ := json.Marshal(body)

					req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")

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

	time.Sleep(15 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Context cancellation: %d completed, %d cancelled", completedCount, cancelledCount)
	assert.True(t, checkAvailable(baseURL), "Server should survive context cancellation chaos")
}
