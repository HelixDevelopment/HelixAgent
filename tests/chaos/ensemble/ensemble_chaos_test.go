// Package ensemble provides chaos tests for the ensemble/debate system.
package ensemble

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

// TestEnsembleChaos_ConcurrentDebates fires many concurrent debate requests
// to verify the ensemble system handles parallelism correctly.
func TestEnsembleChaos_ConcurrentDebates(t *testing.T) {
	testutil.RequireServer(t)

	var wg sync.WaitGroup
	var successCount, failCount int64
	done := make(chan struct{})
	client := &http.Client{Timeout: 30 * time.Second}

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
						"model":    "helixagent-debate",
						"messages": []map[string]string{{"role": "user", "content": "what is 2+2"}},
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
		}(i)
	}

	time.Sleep(15 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Concurrent debates: %d responses, %d errors", successCount, failCount)
	assert.True(t, checkAvailable(baseURL), "Server must survive concurrent debate chaos")
}

// TestEnsembleChaos_LargeContext sends debate requests with large context windows.
func TestEnsembleChaos_LargeContext(t *testing.T) {
	testutil.RequireServer(t)

	client := &http.Client{Timeout: 60 * time.Second}

	// Build a conversation with large context
	messages := make([]map[string]string, 20)
	for i := 0; i < 20; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = map[string]string{
			"role":    role,
			"content": buildLargeContent(500),
		}
	}

	body := map[string]interface{}{
		"model":    "helixagent",
		"messages": messages,
	}
	data, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-chaos-key")

	resp, err := client.Do(req)
	if err != nil {
		t.Logf("Large context request error: %v", err)
	} else {
		resp.Body.Close()
		t.Logf("Large context request: status %d", resp.StatusCode)
	}

	assert.True(t, checkAvailable(baseURL), "Server should handle large context")
}

// TestEnsembleChaos_RapidModelSwitching rapidly switches between ensemble models.
func TestEnsembleChaos_RapidModelSwitching(t *testing.T) {
	testutil.RequireServer(t)

	models := []string{
		"helixagent", "helixagent-debate", "helixagent-fast",
		"helixagent/helixagent-debate", "helixagent/ensemble",
	}

	client := &http.Client{Timeout: 15 * time.Second}
	var wg sync.WaitGroup
	var processedCount int64
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
					model := models[rand.Intn(len(models))]
					body := map[string]interface{}{
						"model":    model,
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

	time.Sleep(15 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Rapid model switching: %d requests processed", processedCount)
	assert.True(t, checkAvailable(baseURL), "Server should handle rapid model switching")
}

// TestEnsembleChaos_AbortedDebates tests behavior when debate requests are aborted.
func TestEnsembleChaos_AbortedDebates(t *testing.T) {
	testutil.RequireServer(t)

	var wg sync.WaitGroup
	var abortedCount int64
	done := make(chan struct{})

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
						"model":    "helixagent-debate",
						"messages": []map[string]string{{"role": "user", "content": "explain quantum physics"}},
					}
					data, _ := json.Marshal(body)

					// Short timeout to abort in-flight
					shortTimeout := time.Duration(rand.Intn(200)+50) * time.Millisecond
					client := &http.Client{Timeout: shortTimeout}
					req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")

					resp, err := client.Do(req)
					if err != nil {
						atomic.AddInt64(&abortedCount, 1)
					} else {
						resp.Body.Close()
					}

					time.Sleep(50 * time.Millisecond)
				}
			}
		}()
	}

	time.Sleep(15 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Aborted debates: %d aborted requests", abortedCount)
	assert.True(t, checkAvailable(baseURL), "Server should survive aborted debates")
}

func buildLargeContent(words int) string {
	wordList := []string{"the", "quick", "brown", "fox", "jumps", "over", "lazy", "dog"}
	result := make([]string, words)
	for i := range result {
		result[i] = wordList[rand.Intn(len(wordList))]
	}
	content := ""
	for _, w := range result {
		content += w + " "
	}
	return content
}
