// Package streaming provides chaos tests for SSE and streaming responses.
package streaming

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
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

// TestStreamingChaos_AbruptDisconnect opens streaming connections and drops them
// at random points to verify the server handles client disconnections.
func TestStreamingChaos_AbruptDisconnect(t *testing.T) {
	testutil.RequireServer(t)

	var wg sync.WaitGroup
	var attemptCount, disconnectCount int64
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
					atomic.AddInt64(&attemptCount, 1)

					body := map[string]interface{}{
						"model":  "helixagent",
						"stream": true,
						"messages": []map[string]string{
							{"role": "user", "content": "write a long story"},
						},
					}
					data, _ := json.Marshal(body)

					client := &http.Client{Timeout: 30 * time.Second}
					req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")
					req.Header.Set("Accept", "text/event-stream")

					resp, err := client.Do(req)
					if err != nil {
						continue
					}

					// Read for a random short duration then disconnect
					readDuration := time.Duration(rand.Intn(300)+50) * time.Millisecond
					readDone := make(chan struct{})
					go func() {
						defer close(readDone)
						scanner := bufio.NewScanner(resp.Body)
						for scanner.Scan() {
							// Just read without processing
						}
					}()

					// Abort after short time
					time.Sleep(readDuration)
					resp.Body.Close()
					<-readDone
					atomic.AddInt64(&disconnectCount, 1)
				}
			}
		}()
	}

	time.Sleep(15 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Abrupt disconnect: %d attempts, %d disconnections", attemptCount, disconnectCount)
	assert.True(t, checkAvailable(baseURL), "Server should survive abrupt stream disconnections")
}

// TestStreamingChaos_SlowConsumer tests server behavior with a slow consumer
// that reads very slowly from the SSE stream.
func TestStreamingChaos_SlowConsumer(t *testing.T) {
	testutil.RequireServer(t)

	var wg sync.WaitGroup
	var successCount int64

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			body := map[string]interface{}{
				"model":  "helixagent",
				"stream": true,
				"messages": []map[string]string{
					{"role": "user", "content": "respond with exactly 5 words"},
				},
			}
			data, _ := json.Marshal(body)

			client := &http.Client{Timeout: 30 * time.Second}
			req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-chaos-key")
			req.Header.Set("Accept", "text/event-stream")

			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			// Read very slowly: 100 bytes at a time with delays
			buf := make([]byte, 100)
			for {
				n, err := resp.Body.Read(buf)
				if n > 0 {
					time.Sleep(50 * time.Millisecond) // slow reader
				}
				if err == io.EOF || err != nil {
					break
				}
			}
			atomic.AddInt64(&successCount, 1)
		}()
	}

	wg.Wait()

	t.Logf("Slow consumer: %d streams completed", successCount)
	assert.True(t, checkAvailable(baseURL), "Server should handle slow consumers")
}

// TestStreamingChaos_ConcurrentStreams opens many concurrent SSE streams.
func TestStreamingChaos_ConcurrentStreams(t *testing.T) {
	testutil.RequireServer(t)

	var wg sync.WaitGroup
	var openedCount, errorCount int64

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			body := map[string]interface{}{
				"model":  "helixagent",
				"stream": true,
				"messages": []map[string]string{
					{"role": "user", "content": "say hello"},
				},
			}
			data, _ := json.Marshal(body)

			client := &http.Client{Timeout: 20 * time.Second}
			req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-chaos-key")
			req.Header.Set("Accept", "text/event-stream")

			resp, err := client.Do(req)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				return
			}
			defer resp.Body.Close()
			atomic.AddInt64(&openedCount, 1)

			// Drain the stream
			io.Copy(io.Discard, resp.Body)
		}()
	}

	wg.Wait()

	t.Logf("Concurrent streams: %d opened, %d errors", openedCount, errorCount)
	assert.True(t, checkAvailable(baseURL), "Server should handle concurrent stream chaos")
}

// TestStreamingChaos_MixedStreamingAndNonStreaming sends interleaved
// streaming and non-streaming requests concurrently.
func TestStreamingChaos_MixedStreamingAndNonStreaming(t *testing.T) {
	testutil.RequireServer(t)

	var wg sync.WaitGroup
	var streamCount, nonStreamCount int64
	done := make(chan struct{})

	// Streaming goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					body := map[string]interface{}{
						"model":  "helixagent",
						"stream": true,
						"messages": []map[string]string{
							{"role": "user", "content": "hello"},
						},
					}
					data, _ := json.Marshal(body)
					client := &http.Client{Timeout: 10 * time.Second}
					req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")
					resp, err := client.Do(req)
					if err == nil {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
						atomic.AddInt64(&streamCount, 1)
					}
				}
			}
		}()
	}

	// Non-streaming goroutines
	for i := 0; i < 5; i++ {
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
					client := &http.Client{Timeout: 10 * time.Second}
					req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(data))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test-chaos-key")
					resp, err := client.Do(req)
					if err == nil {
						resp.Body.Close()
						atomic.AddInt64(&nonStreamCount, 1)
					}
				}
			}
		}()
	}

	time.Sleep(15 * time.Second)
	close(done)
	wg.Wait()

	t.Logf("Mixed stream/non-stream: %d streaming, %d non-streaming", streamCount, nonStreamCount)
	assert.True(t, checkAvailable(baseURL), "Server should handle mixed streaming chaos")
}
