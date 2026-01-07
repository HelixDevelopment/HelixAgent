// Package integration provides integration tests for streaming functionality
// specifically designed to verify OpenCode compatibility.
package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreaming_OpenCode_DoneMarker tests that the streaming endpoint
// properly sends the [DONE] marker at the end of the stream.
// This is critical for OpenCode compatibility.
func TestStreaming_OpenCode_DoneMarker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := &http.Client{Timeout: 120 * time.Second}

	// Test multiple requests to ensure consistent behavior
	for i := 0; i < 3; i++ {
		t.Run(strings.Repeat("request_", 1)+string(rune('A'+i)), func(t *testing.T) {
			payload := map[string]interface{}{
				"model": "superagent-debate",
				"messages": []map[string]string{
					{"role": "user", "content": "Say hi"},
				},
				"stream":     true,
				"max_tokens": 20,
			}
			body, _ := json.Marshal(payload)

			req, err := http.NewRequest("POST", SuperAgentBaseURL+"/v1/chat/completions", bytes.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				t.Skipf("SuperAgent not available: %v", err)
			}
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK")
			require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"), "Expected SSE content type")

			// Read all chunks
			reader := bufio.NewReader(resp.Body)
			chunks := []string{}
			hasDone := false

			for {
				line, err := reader.ReadBytes('\n')
				if err == io.EOF {
					break
				}
				require.NoError(t, err)

				lineStr := strings.TrimSpace(string(line))
				if lineStr == "" {
					continue
				}

				if lineStr == "data: [DONE]" {
					hasDone = true
					break
				}

				if strings.HasPrefix(lineStr, "data: ") {
					chunks = append(chunks, lineStr)
				}
			}

			assert.True(t, hasDone, "Stream must end with [DONE] marker")
			assert.Greater(t, len(chunks), 0, "Should have received at least one chunk")
		})

		// Small delay between requests
		time.Sleep(1 * time.Second)
	}
}

// TestStreaming_OpenCode_ChunkFormat tests that streaming chunks are
// properly formatted according to OpenAI's SSE specification.
func TestStreaming_OpenCode_ChunkFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := &http.Client{Timeout: 60 * time.Second}

	payload := map[string]interface{}{
		"model": "superagent-debate",
		"messages": []map[string]string{
			{"role": "user", "content": "Count: 1, 2, 3"},
		},
		"stream":     true,
		"max_tokens": 30,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", SuperAgentBaseURL+"/v1/chat/completions", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("SuperAgent not available: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Request failed with status %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		lineStr := strings.TrimSpace(string(line))
		if lineStr == "" {
			continue
		}

		if lineStr == "data: [DONE]" {
			break
		}

		if strings.HasPrefix(lineStr, "data: ") {
			// Verify JSON structure
			jsonData := lineStr[6:]
			var chunk map[string]interface{}
			err := json.Unmarshal([]byte(jsonData), &chunk)
			require.NoError(t, err, "Chunk must be valid JSON")

			// Verify required fields
			assert.Contains(t, chunk, "id", "Chunk must have 'id' field")
			assert.Contains(t, chunk, "object", "Chunk must have 'object' field")
			assert.Equal(t, "chat.completion.chunk", chunk["object"], "Object must be 'chat.completion.chunk'")
			assert.Contains(t, chunk, "choices", "Chunk must have 'choices' field")

			// Verify choices structure
			choices, ok := chunk["choices"].([]interface{})
			require.True(t, ok, "choices must be an array")
			require.Greater(t, len(choices), 0, "choices must not be empty")

			choice := choices[0].(map[string]interface{})
			assert.Contains(t, choice, "delta", "Choice must have 'delta' field")
		}
	}
}

// TestStreaming_OpenCode_NoInfiniteLoop tests that the stream properly
// terminates and doesn't loop infinitely (the bug that was reported).
func TestStreaming_OpenCode_NoInfiniteLoop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := &http.Client{Timeout: 90 * time.Second}

	payload := map[string]interface{}{
		"model": "superagent-debate",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
		"stream":     true,
		"max_tokens": 50,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", SuperAgentBaseURL+"/v1/chat/completions", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("SuperAgent not available: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Request failed with status %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)

	// Track chunks and look for duplicates
	seenContent := make(map[string]int)
	maxChunks := 500 // Safety limit
	chunkCount := 0
	hasDone := false

	for chunkCount < maxChunks {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		lineStr := strings.TrimSpace(string(line))
		if lineStr == "" {
			continue
		}

		if lineStr == "data: [DONE]" {
			hasDone = true
			break
		}

		if strings.HasPrefix(lineStr, "data: ") {
			chunkCount++

			// Track content for duplicate detection
			var chunk map[string]interface{}
			if json.Unmarshal([]byte(lineStr[6:]), &chunk) == nil {
				if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
					if choice, ok := choices[0].(map[string]interface{}); ok {
						if delta, ok := choice["delta"].(map[string]interface{}); ok {
							if content, ok := delta["content"].(string); ok && content != "" {
								seenContent[content]++
								// Alert if same content appears too many times
								if seenContent[content] > 10 {
									t.Errorf("Content '%s' appeared %d times - possible infinite loop", content, seenContent[content])
								}
							}
						}
					}
				}
			}
		}
	}

	assert.True(t, hasDone, "Stream must complete with [DONE] marker")
	assert.Less(t, chunkCount, maxChunks, "Stream should not hit the chunk limit (infinite loop)")
}

// TestStreaming_OpenCode_Headers tests that the response has correct
// SSE headers for proper client handling.
func TestStreaming_OpenCode_Headers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := &http.Client{Timeout: 30 * time.Second}

	payload := map[string]interface{}{
		"model": "superagent-debate",
		"messages": []map[string]string{
			{"role": "user", "content": "Hi"},
		},
		"stream":     true,
		"max_tokens": 10,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", SuperAgentBaseURL+"/v1/chat/completions", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("SuperAgent not available: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Request failed with status %d", resp.StatusCode)
	}

	// Verify SSE headers
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"), "Content-Type must be text/event-stream")
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"), "Cache-Control must be no-cache")
	assert.Equal(t, "keep-alive", resp.Header.Get("Connection"), "Connection must be keep-alive")

	// X-Accel-Buffering is optional but recommended for nginx
	if xAccel := resp.Header.Get("X-Accel-Buffering"); xAccel != "" {
		assert.Equal(t, "no", xAccel, "X-Accel-Buffering should be 'no' if present")
	}
}
