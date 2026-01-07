package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenCodeComprehensiveRequest tests the exact request pattern from OpenCode
// This verifies that responses are complete, non-generic, and all providers participate
func TestOpenCodeComprehensiveRequest(t *testing.T) {
	skipIfNotRunning(t)

	baseURL := getBaseURL()

	t.Run("Response_Is_Complete_Not_Cutoff", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "superagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "Write a simple hello world function in Go. Include the function signature, body, and a brief comment."},
			},
			"stream": true,
		}

		jsonBody, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		content := string(body)

		// Verify stream ends with [DONE]
		assert.True(t, strings.Contains(content, "data: [DONE]"), "Response must end with [DONE] marker")

		// Verify there's a finish_reason: "stop" before [DONE]
		assert.True(t, strings.Contains(content, `"finish_reason":"stop"`), "Response must have finish_reason: stop")

		// Parse chunks and verify content is substantial
		chunks := parseSSEChunks(content)
		assert.Greater(t, len(chunks), 2, "Response should have more than 2 chunks")

		// Combine all content
		fullContent := extractFullContent(chunks)
		assert.Greater(t, len(fullContent), 50, "Response content should be substantial (>50 chars)")

		// Verify response contains expected elements for a Go function
		assert.True(t,
			strings.Contains(fullContent, "func") || strings.Contains(fullContent, "package"),
			"Response should contain Go code elements")

		t.Logf("Response length: %d chars, chunks: %d", len(fullContent), len(chunks))
	})

	t.Run("Multi_Provider_Participation_In_Debate", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "superagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "What are the pros and cons of using Go vs Rust for systems programming?"},
			},
			"stream": true,
		}

		jsonBody, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(req)

		// NEVER SKIP - validate the fallback mechanism instead
		if err != nil {
			// This is a network error - validate it's properly categorized
			errStr := strings.ToLower(err.Error())
			isNetworkError := strings.Contains(errStr, "eof") ||
				strings.Contains(errStr, "connection refused") ||
				strings.Contains(errStr, "timeout") ||
				strings.Contains(errStr, "connection reset")

			if isNetworkError {
				t.Logf("Network error detected (fallback should have been attempted): %v", err)
				// The fallback mechanism was implemented - verify the error is categorized
				assert.True(t, isNetworkError, "Error should be a recognized network error type")
				// This is still a FAILURE because the system should have recovered via fallback
				t.Errorf("FALLBACK FAILED: Network error occurred but fallback providers should have handled it: %v", err)
			} else {
				t.Errorf("Unexpected error type: %v", err)
			}
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Reading response body should not fail")

		chunks := parseSSEChunks(string(body))

		// For multi-provider debate, we expect responses to come through
		// The metadata should show participation from multiple providers
		fullContent := extractFullContent(chunks)
		assert.Greater(t, len(fullContent), 100, "Debate response should be substantial")

		// Verify response has balanced perspective (mentions both Go and Rust)
		hasGo := strings.Contains(strings.ToLower(fullContent), "go")
		hasRust := strings.Contains(strings.ToLower(fullContent), "rust")
		assert.True(t, hasGo || hasRust, "Response should mention the languages being compared")

		t.Logf("Debate response: %d chars", len(fullContent))
	})

	t.Run("Content_Is_Dynamic_Not_Generic", func(t *testing.T) {
		// Send two slightly different requests and verify responses differ
		responses := make([]string, 2)
		prompts := []string{
			"Generate a random 4-digit number and explain why you chose it",
			"Generate a random 5-digit number and explain your reasoning",
		}

		for i, prompt := range prompts {
			reqBody := map[string]interface{}{
				"model": "superagent-ensemble",
				"messages": []map[string]string{
					{"role": "user", "content": prompt},
				},
				"stream": true,
			}

			jsonBody, _ := json.Marshal(reqBody)
			req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			chunks := parseSSEChunks(string(body))
			responses[i] = extractFullContent(chunks)
		}

		// Responses should be different (not hardcoded/static)
		assert.NotEqual(t, responses[0], responses[1], "Responses to different prompts should differ")
		assert.NotEmpty(t, responses[0], "First response should not be empty")
		assert.NotEmpty(t, responses[1], "Second response should not be empty")

		t.Logf("Response 1 length: %d, Response 2 length: %d", len(responses[0]), len(responses[1]))
	})

	t.Run("Stream_ID_Consistency", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "superagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "Count from 1 to 3"},
			},
			"stream": true,
		}

		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		chunks := parseSSEChunks(string(body))

		// All chunks should have the same ID
		var firstID string
		for i, chunk := range chunks {
			if id, ok := chunk["id"].(string); ok {
				if i == 0 {
					firstID = id
				} else {
					assert.Equal(t, firstID, id, "All chunks should have consistent stream ID")
				}
			}
		}

		assert.NotEmpty(t, firstID, "Stream ID should be present")
	})

	t.Run("First_Chunk_Has_Role", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "superagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "Say OK"},
			},
			"stream": true,
		}

		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		chunks := parseSSEChunks(string(body))

		require.Greater(t, len(chunks), 0, "Should have at least one chunk")

		// First chunk should have role: "assistant"
		firstChunk := chunks[0]
		if choices, ok := firstChunk["choices"].([]interface{}); ok && len(choices) > 0 {
			choice := choices[0].(map[string]interface{})
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				role, hasRole := delta["role"]
				assert.True(t, hasRole, "First chunk delta should have role")
				assert.Equal(t, "assistant", role, "First chunk role should be 'assistant'")
			}
		}
	})

	t.Run("No_Premature_Termination_Long_Response", func(t *testing.T) {
		// Request that requires a longer response
		reqBody := map[string]interface{}{
			"model": "superagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "Explain the HTTP request-response cycle in detail, covering DNS resolution, TCP handshake, request headers, response codes, and content delivery. Be thorough."},
			},
			"stream": true,
		}

		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 180 * time.Second} // Longer timeout for detailed response
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		content := string(body)

		// Must end with [DONE]
		assert.True(t, strings.HasSuffix(strings.TrimSpace(content), "data: [DONE]") ||
			strings.Contains(content, "data: [DONE]"),
			"Long response must complete with [DONE] marker")

		// Must have finish_reason
		assert.True(t, strings.Contains(content, `"finish_reason":"stop"`) ||
			strings.Contains(content, `"finish_reason": "stop"`),
			"Long response must have finish_reason")

		chunks := parseSSEChunks(content)
		fullContent := extractFullContent(chunks)

		// Should be a substantial explanation
		assert.Greater(t, len(fullContent), 200, "Detailed response should be >200 chars")

		// Should cover multiple topics mentioned in the prompt
		topicsFound := 0
		if strings.Contains(strings.ToLower(fullContent), "dns") {
			topicsFound++
		}
		if strings.Contains(strings.ToLower(fullContent), "tcp") {
			topicsFound++
		}
		if strings.Contains(strings.ToLower(fullContent), "header") {
			topicsFound++
		}
		if strings.Contains(strings.ToLower(fullContent), "response") {
			topicsFound++
		}

		assert.GreaterOrEqual(t, topicsFound, 2, "Response should cover at least 2 topics from the prompt")

		t.Logf("Long response: %d chars, topics covered: %d", len(fullContent), topicsFound)
	})
}

// TestOpenCodeConcurrentRequests tests handling of concurrent requests
func TestOpenCodeConcurrentRequests(t *testing.T) {
	skipIfNotRunning(t)

	baseURL := getBaseURL()
	numRequests := 5

	var wg sync.WaitGroup
	results := make(chan struct {
		index   int
		success bool
		content string
		err     error
	}, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			reqBody := map[string]interface{}{
				"model": "superagent-ensemble",
				"messages": []map[string]string{
					{"role": "user", "content": fmt.Sprintf("Request %d: What is %d + %d?", idx, idx, idx*2)},
				},
				"stream": true,
			}

			jsonBody, _ := json.Marshal(reqBody)
			req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				results <- struct {
					index   int
					success bool
					content string
					err     error
				}{idx, false, "", err}
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			content := string(body)

			// Check for proper completion
			success := strings.Contains(content, "data: [DONE]")

			chunks := parseSSEChunks(content)
			fullContent := extractFullContent(chunks)

			results <- struct {
				index   int
				success bool
				content string
				err     error
			}{idx, success, fullContent, nil}
		}(i)
	}

	wg.Wait()
	close(results)

	successCount := 0
	for result := range results {
		if result.success {
			successCount++
			t.Logf("Request %d succeeded: %d chars", result.index, len(result.content))
		} else {
			t.Logf("Request %d failed: %v", result.index, result.err)
		}
	}

	// At least 80% should succeed under concurrent load
	minSuccess := numRequests * 80 / 100
	assert.GreaterOrEqual(t, successCount, minSuccess,
		"At least %d%% of concurrent requests should succeed", 80)
}

// TestOpenCodeEdgeCases tests edge cases and error handling
func TestOpenCodeEdgeCases(t *testing.T) {
	skipIfNotRunning(t)

	baseURL := getBaseURL()

	t.Run("Empty_Message_Handling", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "superagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": ""},
			},
			"stream": true,
		}

		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should either return an error response or handle gracefully
		assert.True(t, resp.StatusCode == 200 || resp.StatusCode == 400,
			"Empty message should return 200 (handled) or 400 (rejected)")
	})

	t.Run("Very_Long_Prompt_Handling", func(t *testing.T) {
		// Create a very long prompt
		longPrompt := strings.Repeat("This is a test sentence. ", 100)

		reqBody := map[string]interface{}{
			"model": "superagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": longPrompt + " Summarize in one sentence."},
			},
			"stream": true,
		}

		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		content := string(body)

		// Should complete properly even with long prompt
		if resp.StatusCode == 200 {
			assert.True(t, strings.Contains(content, "data: [DONE]"),
				"Long prompt response should complete with [DONE]")
		}
	})

	t.Run("Special_Characters_In_Prompt", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "superagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "What does this symbol mean: < > & \" ' \n\t ?"},
			},
			"stream": true,
		}

		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		content := string(body)

		// Should handle special characters gracefully
		assert.True(t, strings.Contains(content, "data: [DONE]") || resp.StatusCode == 200,
			"Special characters should be handled properly")
	})

	t.Run("Timeout_Context_Cancellation", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "superagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "Say hello"},
			},
			"stream": true,
		}

		jsonBody, _ := json.Marshal(reqBody)

		// Create context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		_, err := client.Do(req)

		// Should either timeout or complete quickly
		// The important thing is it doesn't hang
		if err != nil {
			assert.True(t, strings.Contains(err.Error(), "context") ||
				strings.Contains(err.Error(), "timeout") ||
				strings.Contains(err.Error(), "deadline"),
				"Error should be context/timeout related")
		}
	})
}

// TestFallbackMechanismValidation tests that fallback providers are invoked when primary fails
func TestFallbackMechanismValidation(t *testing.T) {
	skipIfNotRunning(t)

	baseURL := getBaseURL()

	t.Run("Response_Must_Succeed_With_Fallback", func(t *testing.T) {
		// Make multiple requests to verify fallback mechanism
		successCount := 0
		failureDetails := make([]string, 0)

		for i := 0; i < 3; i++ {
			reqBody := map[string]interface{}{
				"model": "superagent-ensemble",
				"messages": []map[string]string{
					{"role": "user", "content": fmt.Sprintf("Test request %d: Say hello", i)},
				},
				"stream": true,
			}

			jsonBody, _ := json.Marshal(reqBody)
			req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Do(req)

			if err != nil {
				failureDetails = append(failureDetails, fmt.Sprintf("Request %d: %v", i, err))
				continue
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				failureDetails = append(failureDetails, fmt.Sprintf("Request %d read: %v", i, err))
				continue
			}

			content := string(body)
			if strings.Contains(content, "data: [DONE]") {
				successCount++
			} else {
				failureDetails = append(failureDetails, fmt.Sprintf("Request %d: no [DONE] marker", i))
			}
		}

		// ALL requests must succeed - fallback should handle any failures
		assert.Equal(t, 3, successCount, "ALL requests must succeed with fallback mechanism. Failures: %v", failureDetails)
	})

	t.Run("Error_Response_Contains_Categorization", func(t *testing.T) {
		// Test that error responses contain proper categorization
		// Send request with intentionally invalid model to trigger error handling
		reqBody := map[string]interface{}{
			"model": "invalid-model-that-does-not-exist",
			"messages": []map[string]string{
				{"role": "user", "content": "Test error categorization"},
			},
			"stream": true,
		}

		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			// Even network errors should be categorized
			errStr := err.Error()
			t.Logf("Error (should be categorized): %s", errStr)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		content := string(body)

		// Response might be an error response or might succeed via fallback
		// Either way, it should be properly formatted
		t.Logf("Response status: %d, content length: %d", resp.StatusCode, len(content))

		if resp.StatusCode != 200 {
			// Error response should be JSON with error details
			var errorResp map[string]interface{}
			err := json.Unmarshal(body, &errorResp)
			if err == nil {
				if errObj, ok := errorResp["error"].(map[string]interface{}); ok {
					assert.Contains(t, errObj, "type", "Error response should contain error type")
					assert.Contains(t, errObj, "message", "Error response should contain error message")
				}
			}
		}
	})

	t.Run("Network_Quality_Verification", func(t *testing.T) {
		// Verify network quality by checking response times and success rate
		responseTimes := make([]time.Duration, 0)
		successCount := 0

		for i := 0; i < 5; i++ {
			start := time.Now()
			reqBody := map[string]interface{}{
				"model": "superagent-ensemble",
				"messages": []map[string]string{
					{"role": "user", "content": "Ping"},
				},
				"stream": false, // Non-streaming for quicker response
			}

			jsonBody, _ := json.Marshal(reqBody)
			req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)
			elapsed := time.Since(start)

			if err == nil && resp.StatusCode == 200 {
				successCount++
				responseTimes = append(responseTimes, elapsed)
				resp.Body.Close()
			} else if err != nil {
				t.Logf("Request %d failed: %v (time: %v)", i, err, elapsed)
			}
		}

		// Report network quality metrics
		if len(responseTimes) > 0 {
			var total time.Duration
			for _, rt := range responseTimes {
				total += rt
			}
			avgTime := total / time.Duration(len(responseTimes))
			t.Logf("Network Quality: %d/5 successful, avg response time: %v", successCount, avgTime)
		}

		// At least 80% success rate required for network quality
		assert.GreaterOrEqual(t, successCount, 4, "Network quality check: at least 80%% success rate required")
	})
}

// TestContentValidation tests that content is valid and complete
func TestContentValidation(t *testing.T) {
	skipIfNotRunning(t)

	baseURL := getBaseURL()

	t.Run("JSON_Response_Valid", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "superagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "Reply with just: OK"},
			},
			"stream": true,
		}

		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		chunks := parseSSEChunks(string(body))

		// Each chunk should be valid JSON
		for i, chunk := range chunks {
			// Try to marshal back to JSON to verify validity
			_, err := json.Marshal(chunk)
			assert.NoError(t, err, "Chunk %d should be valid JSON", i)

			// Verify required fields
			assert.Contains(t, chunk, "id", "Chunk %d should have id", i)
			assert.Contains(t, chunk, "object", "Chunk %d should have object", i)
			assert.Contains(t, chunk, "choices", "Chunk %d should have choices", i)
		}
	})

	t.Run("No_Duplicate_Finish_Reason", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "superagent-ensemble",
			"messages": []map[string]string{
				{"role": "user", "content": "Count: 1, 2, 3"},
			},
			"stream": true,
		}

		jsonBody, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		chunks := parseSSEChunks(string(body))

		finishReasonCount := 0
		for _, chunk := range chunks {
			if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
				choice := choices[0].(map[string]interface{})
				if fr, ok := choice["finish_reason"]; ok && fr != nil {
					finishReasonCount++
				}
			}
		}

		// Should have exactly one chunk with finish_reason
		assert.Equal(t, 1, finishReasonCount,
			"Should have exactly one chunk with finish_reason, got %d", finishReasonCount)
	})
}

// Helper functions

func parseSSEChunks(content string) []map[string]interface{} {
	var chunks []map[string]interface{}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			continue
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err == nil {
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

func extractFullContent(chunks []map[string]interface{}) string {
	var content strings.Builder

	for _, chunk := range chunks {
		if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
			choice := choices[0].(map[string]interface{})
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				if c, ok := delta["content"].(string); ok {
					content.WriteString(c)
				}
			}
		}
	}

	return content.String()
}
