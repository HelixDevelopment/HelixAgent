// Package memory provides chaos tests for the memory/retrieval endpoints.
package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// TestMemoryChaos_ConcurrentReadWrites fires concurrent memory store and retrieve
// operations to stress test the memory system.
func TestMemoryChaos_ConcurrentReadWrites(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 10 * time.Second}
	var writeCount, readCount, errorCount int64
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					body := map[string]interface{}{
						"content":  fmt.Sprintf("chaos memory entry %d", rand.Int()),
						"user_id":  fmt.Sprintf("chaos-user-%d", idx),
						"metadata": map[string]string{"type": "chaos", "iteration": fmt.Sprintf("%d", rand.Int())},
					}
					data, _ := json.Marshal(body)
					req, _ := http.NewRequest("POST", baseURL+"/v1/memory", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")

					resp, err := client.Do(req)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}
					resp.Body.Close()
					atomic.AddInt64(&writeCount, 1)
				}
			}
		}(i)
	}

	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					body := map[string]interface{}{
						"query":   fmt.Sprintf("chaos test query %d", rand.Int()),
						"user_id": fmt.Sprintf("chaos-user-%d", idx%10),
						"limit":   10,
					}
					data, _ := json.Marshal(body)
					req, _ := http.NewRequest("POST", baseURL+"/v1/memory/search", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")

					resp, err := client.Do(req)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}
					resp.Body.Close()
					atomic.AddInt64(&readCount, 1)
				}
			}
		}(i)
	}

	time.Sleep(15 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Memory chaos: %d writes, %d reads, %d errors", writeCount, readCount, errorCount)
	assert.True(t, checkAvailable(baseURL), "Server must survive memory concurrent chaos")
}

// TestMemoryChaos_MemoryFlood floods the memory endpoint with write requests.
func TestMemoryChaos_MemoryFlood(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 5 * time.Second}
	var processedCount, errorCount int64
	var wg sync.WaitGroup
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
					body := map[string]interface{}{
						"content": generateRandomContent(),
						"user_id": fmt.Sprintf("flood-user-%d", rand.Intn(100)),
					}
					data, _ := json.Marshal(body)
					req, _ := http.NewRequest("POST", baseURL+"/v1/memory", bytes.NewBuffer(data))
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

	t.Logf("Memory flood: %d processed, %d connection errors", processedCount, errorCount)
	assert.True(t, checkAvailable(baseURL), "Server should survive memory flood")
}

// TestMemoryChaos_InvalidMemoryRequests sends malformed memory requests.
func TestMemoryChaos_InvalidMemoryRequests(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 5 * time.Second}
	var processedCount int64
	var wg sync.WaitGroup

	invalidPayloads := []string{
		`{}`,
		`{"content": null}`,
		`{"content": ""}`,
		`{"user_id": null}`,
		`{invalid json}`,
		`{"content": ` + fmt.Sprintf("%q", make([]byte, 100000)) + `}`,
		`null`,
		`[]`,
	}

	for _, payload := range invalidPayloads {
		for i := 0; i < 3; i++ {
			wg.Add(1)
			p := payload
			go func() {
				defer wg.Done()
				req, _ := http.NewRequest("POST", baseURL+"/v1/memory", bytes.NewBuffer([]byte(p)))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer test-chaos-key")
				resp, err := client.Do(req)
				if err == nil {
					resp.Body.Close()
					atomic.AddInt64(&processedCount, 1)
				}
			}()
		}
	}

	wg.Wait()
	t.Logf("Invalid memory requests: %d got server responses", processedCount)
	assert.True(t, checkAvailable(baseURL), "Server should handle invalid memory requests gracefully")
}

// TestMemoryChaos_DeleteUnderLoad deletes memory entries while writes happen concurrently.
func TestMemoryChaos_DeleteUnderLoad(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 5 * time.Second}
	var writeCount, deleteCount int64
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Writers
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
						"content": "delete chaos test",
						"user_id": "delete-chaos-user",
					}
					data, _ := json.Marshal(body)
					req, _ := http.NewRequest("POST", baseURL+"/v1/memory", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")
					resp, err := client.Do(req)
					if err == nil {
						resp.Body.Close()
						atomic.AddInt64(&writeCount, 1)
					}
				}
			}
		}()
	}

	// Deleters
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					// Try deleting by random IDs
					memID := fmt.Sprintf("chaos-mem-%d", rand.Int())
					req, _ := http.NewRequest("DELETE", baseURL+"/v1/memory/"+memID, nil)
					req.Header.Set("Authorization", "Bearer test-chaos-key")
					resp, err := client.Do(req)
					if err == nil {
						resp.Body.Close()
						atomic.AddInt64(&deleteCount, 1)
					}
				}
			}
		}(i)
	}

	time.Sleep(10 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Memory delete under load: %d writes, %d deletes", writeCount, deleteCount)
	assert.True(t, checkAvailable(baseURL), "Server should survive concurrent memory writes and deletes")
}

func generateRandomContent() string {
	words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "chaos", "test", "memory"}
	count := rand.Intn(20) + 5
	result := ""
	for i := 0; i < count; i++ {
		result += words[rand.Intn(len(words))] + " "
	}
	return result
}
