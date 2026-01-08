// Package integration provides integration tests for HelixAgent
package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"dev.helix.agent/internal/config"
)

// StreamingChunk represents an OpenAI-compatible streaming chunk
type StreamingChunk struct {
	ID                string                 `json:"id"`
	Object            string                 `json:"object"`
	Created           int64                  `json:"created"`
	Model             string                 `json:"model"`
	SystemFingerprint string                 `json:"system_fingerprint"`
	Choices           []StreamingChunkChoice `json:"choices"`
}

// StreamingChunkChoice represents a choice in a streaming chunk
type StreamingChunkChoice struct {
	Index        int                    `json:"index"`
	Delta        map[string]interface{} `json:"delta"`
	Logprobs     interface{}            `json:"logprobs"`
	FinishReason interface{}            `json:"finish_reason"`
}

// getBaseURL returns the HelixAgent base URL for testing
func getBaseURL() string {
	if url := os.Getenv("HELIXAGENT_URL"); url != "" {
		return url
	}
	return "http://localhost:7061"
}

// skipIfNotRunning skips the test if HelixAgent is not running
func skipIfNotRunning(t *testing.T) {
	baseURL := getBaseURL()
	resp, err := http.Get(baseURL + "/health")
	if err != nil || resp.StatusCode != 200 {
		t.Skipf("HelixAgent not running at %s, skipping integration test", baseURL)
	}
	resp.Body.Close()
}

// TestStreamingFormat_OpenCodeCompatibility tests that streaming responses are
// compatible with OpenCode's requirements
func TestStreamingFormat_OpenCodeCompatibility(t *testing.T) {
	skipIfNotRunning(t)

	baseURL := getBaseURL()

	reqBody := map[string]interface{}{
		"model": "helixagent-ensemble",
		"messages": []map[string]string{
			{"role": "user", "content": "Say hello in exactly 3 words"},
		},
		"stream": true,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	// Parse SSE stream
	reader := bufio.NewReader(resp.Body)
	chunks := []StreamingChunk{}
	var streamID string
	sawDone := false
	firstChunkHasRole := false
	subsequentChunksHaveRole := false

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			sawDone = true
			break
		}

		var chunk StreamingChunk
		err = json.Unmarshal([]byte(data), &chunk)
		require.NoError(t, err, "Failed to parse chunk: %s", data)

		if streamID == "" {
			streamID = chunk.ID
		}

		// Track if chunks have role
		if len(chunks) == 0 {
			if _, hasRole := chunk.Choices[0].Delta["role"]; hasRole {
				firstChunkHasRole = true
			}
		} else {
			if _, hasRole := chunk.Choices[0].Delta["role"]; hasRole {
				subsequentChunksHaveRole = true
			}
		}

		chunks = append(chunks, chunk)
	}

	// OpenCode Compatibility Checks
	t.Run("StreamID_Consistency", func(t *testing.T) {
		require.NotEmpty(t, chunks, "Should receive at least one chunk")
		for i, chunk := range chunks {
			assert.Equal(t, streamID, chunk.ID, "Chunk %d should have consistent stream ID", i)
		}
	})

	t.Run("FirstChunk_HasRole", func(t *testing.T) {
		assert.True(t, firstChunkHasRole, "First chunk must have role (OpenCode requirement)")
	})

	t.Run("SubsequentChunks_NoRole", func(t *testing.T) {
		assert.False(t, subsequentChunksHaveRole, "Subsequent chunks should not have role (OpenCode Issue #2840)")
	})

	t.Run("FinishReason_NullForIntermediate", func(t *testing.T) {
		for i := 0; i < len(chunks)-1; i++ {
			finishReason := chunks[i].Choices[0].FinishReason
			assert.Nil(t, finishReason, "Intermediate chunk %d should have finish_reason: null", i)
		}
	})

	t.Run("FinishReason_StopForFinal", func(t *testing.T) {
		require.NotEmpty(t, chunks, "Should have chunks")
		lastChunk := chunks[len(chunks)-1]
		finishReason, ok := lastChunk.Choices[0].FinishReason.(string)
		assert.True(t, ok, "Final chunk should have finish_reason as string")
		assert.Equal(t, "stop", finishReason, "Final chunk should have finish_reason: stop")
	})

	t.Run("SystemFingerprint_Present", func(t *testing.T) {
		for i, chunk := range chunks {
			assert.NotEmpty(t, chunk.SystemFingerprint, "Chunk %d should have system_fingerprint (OpenCode requirement)", i)
		}
	})

	t.Run("Logprobs_NullPresent", func(t *testing.T) {
		for i, chunk := range chunks {
			// Logprobs should be explicitly null (not missing)
			assert.Nil(t, chunk.Choices[0].Logprobs, "Chunk %d should have logprobs: null", i)
		}
	})

	t.Run("DoneMarker_Present", func(t *testing.T) {
		assert.True(t, sawDone, "Stream should end with [DONE] marker")
	})

	t.Run("NoEmptyToolCallsArray", func(t *testing.T) {
		for i, chunk := range chunks {
			toolCalls, hasToolCalls := chunk.Choices[0].Delta["tool_calls"]
			if hasToolCalls {
				// If tool_calls is present, it should not be an empty array
				if arr, ok := toolCalls.([]interface{}); ok {
					assert.NotEmpty(t, arr, "Chunk %d should not have empty tool_calls array (OpenCode Issue #4255)", i)
				}
			}
		}
	})
}

// TestStreamingFormat_CrushCompatibility tests that streaming responses are
// compatible with Crush's requirements
func TestStreamingFormat_CrushCompatibility(t *testing.T) {
	skipIfNotRunning(t)

	baseURL := getBaseURL()

	reqBody := map[string]interface{}{
		"model": "helixagent-ensemble",
		"messages": []map[string]string{
			{"role": "user", "content": "Hi"},
		},
		"stream": true,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Crush Compatibility Checks
	t.Run("ContentType_TextEventStream", func(t *testing.T) {
		assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	})

	// Parse and verify structure
	reader := bufio.NewReader(resp.Body)
	var lastChunk StreamingChunk
	sawDone := false

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			sawDone = true
			break
		}

		err = json.Unmarshal([]byte(data), &lastChunk)
		require.NoError(t, err)
	}

	t.Run("Object_ChatCompletionChunk", func(t *testing.T) {
		assert.Equal(t, "chat.completion.chunk", lastChunk.Object)
	})

	t.Run("Model_Present", func(t *testing.T) {
		assert.NotEmpty(t, lastChunk.Model)
	})

	t.Run("DoneMarker_Present", func(t *testing.T) {
		assert.True(t, sawDone, "Crush requires [DONE] marker to terminate stream")
	})
}

// TestStreamingFormat_HelixCodeCompatibility tests that streaming responses are
// compatible with HelixCode's requirements
func TestStreamingFormat_HelixCodeCompatibility(t *testing.T) {
	skipIfNotRunning(t)

	baseURL := getBaseURL()

	reqBody := map[string]interface{}{
		"model": "helixagent-ensemble",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
		"stream": true,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// HelixCode Compatibility Checks (similar to OpenCode, OpenAI-compatible)
	reader := bufio.NewReader(resp.Body)
	chunks := []StreamingChunk{}
	sawDone := false

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			sawDone = true
			break
		}

		var chunk StreamingChunk
		err = json.Unmarshal([]byte(data), &chunk)
		require.NoError(t, err)
		chunks = append(chunks, chunk)
	}

	t.Run("ValidChunks", func(t *testing.T) {
		assert.NotEmpty(t, chunks, "Should receive chunks")
	})

	t.Run("ProperTermination", func(t *testing.T) {
		assert.True(t, sawDone, "Stream should end with [DONE]")
	})

	t.Run("ChoicesStructure", func(t *testing.T) {
		for _, chunk := range chunks {
			assert.Len(t, chunk.Choices, 1, "Should have exactly one choice")
			assert.Equal(t, 0, chunk.Choices[0].Index, "Choice index should be 0")
		}
	})
}

// TestConfigGenerator_AllAgents tests that config generator produces valid configs for all agents
func TestConfigGenerator_AllAgents(t *testing.T) {
	gen := config.NewConfigGenerator(
		"http://localhost:7061/v1",
		"test-api-key",
		"helixagent-ensemble",
	)
	gen.SetTimeout(120).SetMaxTokens(8192)

	validator := config.NewConfigValidator()

	t.Run("OpenCode", func(t *testing.T) {
		jsonData, err := gen.GenerateJSON(config.AgentTypeOpenCode)
		require.NoError(t, err)

		result, err := validator.ValidateJSON(config.AgentTypeOpenCode, jsonData)
		require.NoError(t, err)
		assert.True(t, result.Valid, "OpenCode config should be valid: %v", result.Errors)

		// Verify structure
		var cfg map[string]interface{}
		err = json.Unmarshal(jsonData, &cfg)
		require.NoError(t, err)

		assert.Contains(t, cfg, "$schema")
		assert.Contains(t, cfg, "provider")

		provider := cfg["provider"].(map[string]interface{})
		assert.Contains(t, provider, "helixagent")
	})

	t.Run("Crush", func(t *testing.T) {
		jsonData, err := gen.GenerateJSON(config.AgentTypeCrush)
		require.NoError(t, err)

		result, err := validator.ValidateJSON(config.AgentTypeCrush, jsonData)
		require.NoError(t, err)
		assert.True(t, result.Valid, "Crush config should be valid: %v", result.Errors)

		// Verify structure
		var cfg map[string]interface{}
		err = json.Unmarshal(jsonData, &cfg)
		require.NoError(t, err)

		assert.Contains(t, cfg, "providers")
		providers := cfg["providers"].(map[string]interface{})
		assert.Contains(t, providers, "helixagent")

		sa := providers["helixagent"].(map[string]interface{})
		assert.Equal(t, "openai-compat", sa["type"])
	})

	t.Run("HelixCode", func(t *testing.T) {
		jsonData, err := gen.GenerateJSON(config.AgentTypeHelixCode)
		require.NoError(t, err)

		result, err := validator.ValidateJSON(config.AgentTypeHelixCode, jsonData)
		require.NoError(t, err)
		assert.True(t, result.Valid, "HelixCode config should be valid: %v", result.Errors)

		// Verify structure
		var cfg map[string]interface{}
		err = json.Unmarshal(jsonData, &cfg)
		require.NoError(t, err)

		assert.Contains(t, cfg, "providers")
		assert.Contains(t, cfg, "settings")

		settings := cfg["settings"].(map[string]interface{})
		assert.Equal(t, "helixagent", settings["default_provider"])
		assert.True(t, settings["streaming_enabled"].(bool))
	})
}

// TestAgentStreamingTimeout tests that streaming doesn't hang indefinitely
// (Renamed from TestStreamingTimeout to avoid conflict with provider_streaming_test.go)
func TestAgentStreamingTimeout(t *testing.T) {
	skipIfNotRunning(t)

	baseURL := getBaseURL()

	reqBody := map[string]interface{}{
		"model": "helixagent-ensemble",
		"messages": []map[string]string{
			{"role": "user", "content": "Count to 3"},
		},
		"stream": true,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Use a shorter timeout to test that stream completes properly
	client := &http.Client{Timeout: 30 * time.Second}

	start := time.Now()
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Read all response
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	elapsed := time.Since(start)

	// Stream should complete in reasonable time
	assert.Less(t, elapsed, 30*time.Second, "Streaming should complete within timeout")

	// Should end with [DONE]
	assert.Contains(t, string(body), "[DONE]", "Stream should end with [DONE] marker")
}

// TestNonStreamingCompatibility tests that non-streaming requests still work
func TestNonStreamingCompatibility(t *testing.T) {
	skipIfNotRunning(t)

	baseURL := getBaseURL()

	reqBody := map[string]interface{}{
		"model": "helixagent-ensemble",
		"messages": []map[string]string{
			{"role": "user", "content": "Say hi in one word"},
		},
		"stream": false,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	// Verify non-streaming response structure
	assert.Contains(t, result, "id")
	assert.Contains(t, result, "object")
	assert.Equal(t, "chat.completion", result["object"])
	assert.Contains(t, result, "choices")

	choices := result["choices"].([]interface{})
	require.Len(t, choices, 1)

	choice := choices[0].(map[string]interface{})
	assert.Contains(t, choice, "message")
	assert.Equal(t, "stop", choice["finish_reason"])

	message := choice["message"].(map[string]interface{})
	assert.Equal(t, "assistant", message["role"])
	assert.NotEmpty(t, message["content"])
}

// BenchmarkStreamingThroughput benchmarks streaming response throughput
func BenchmarkStreamingThroughput(b *testing.B) {
	baseURL := getBaseURL()

	// Check if running
	resp, err := http.Get(baseURL + "/health")
	if err != nil || resp.StatusCode != 200 {
		b.Skip("HelixAgent not running")
	}
	resp.Body.Close()

	reqBody := map[string]interface{}{
		"model": "helixagent-ensemble",
		"messages": []map[string]string{
			{"role": "user", "content": "Hi"},
		},
		"stream": true,
	}

	jsonBody, _ := json.Marshal(reqBody)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}

		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
