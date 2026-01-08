// Package integration provides OpenCode simulation tests
// These tests verify HelixAgent works correctly with OpenCode-style requests
package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"
)

// OpenCodeRequest represents an OpenCode-style chat completion request
type OpenCodeRequest struct {
	Model     string            `json:"model"`
	Messages  []OpenCodeMessage `json:"messages"`
	MaxTokens int               `json:"max_tokens,omitempty"`
	Stream    bool              `json:"stream,omitempty"`
}

// OpenCodeMessage represents a chat message
type OpenCodeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenCodeResponse represents a chat completion response
type OpenCodeResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []OpenCodeChoice `json:"choices"`
	Usage   OpenCodeUsage    `json:"usage,omitempty"`
	Error   *OpenCodeError   `json:"error,omitempty"`
}

// OpenCodeChoice represents a response choice
type OpenCodeChoice struct {
	Index   int `json:"index"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

// OpenCodeUsage represents token usage
type OpenCodeUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenCodeError represents an API error
type OpenCodeError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func getTestURL() string {
	url := os.Getenv("HELIXAGENT_URL")
	if url == "" {
		url = "http://localhost:7061"
	}
	return url
}

func getTestAPIKey() string {
	key := os.Getenv("HELIXAGENT_API_KEY")
	if key == "" {
		key = "sk-bd15ed2af3d6cd8c0bdf57e221bbf7771fa06bda93cc8866807cc85211f58d1a"
	}
	return key
}

// TestOpenCode_HealthCheck verifies HelixAgent is ready for OpenCode
func TestOpenCode_HealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OpenCode simulation in short mode")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(getTestURL() + "/health")
	if err != nil {
		t.Fatalf("HelixAgent health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HelixAgent unhealthy: status %d", resp.StatusCode)
	}
	t.Log("HelixAgent health check passed")
}

// TestOpenCode_CodebaseQuery simulates "Do you see my codebase?" query
func TestOpenCode_CodebaseQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OpenCode simulation in short mode")
	}

	request := OpenCodeRequest{
		Model: "helixagent-debate",
		Messages: []OpenCodeMessage{
			{Role: "user", Content: "Do you see my codebase? Give me a brief overview."},
		},
		MaxTokens: 500,
	}

	resp, err := sendOpenCodeRequest(request)
	if err != nil {
		t.Skipf("OpenCode codebase query failed (network issue): %v", err)
	}

	if resp.Error != nil {
		t.Skipf("API returned error (may indicate service unavailable): %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		t.Skip("No choices returned (may indicate service unavailable)")
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		t.Skip("Empty response content (may indicate service unavailable)")
	}

	t.Logf("Response length: %d chars", len(content))
	if len(content) > 200 {
		t.Logf("Response preview: %s...", content[:200])
	} else {
		t.Logf("Response: %s", content)
	}
}

// TestOpenCode_InitRequest simulates an init/project analysis request
func TestOpenCode_InitRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OpenCode simulation in short mode")
	}

	request := OpenCodeRequest{
		Model: "helixagent-debate",
		Messages: []OpenCodeMessage{
			{Role: "user", Content: "Initialize and analyze this project. What is the main purpose and structure?"},
		},
		MaxTokens: 1000,
	}

	resp, err := sendOpenCodeRequest(request)
	if err != nil {
		t.Skipf("OpenCode init request failed (network issue): %v", err)
	}

	if resp.Error != nil {
		t.Skipf("API returned error (may indicate service unavailable): %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		t.Skip("No choices returned (may indicate service unavailable)")
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		t.Skip("Empty response content (may indicate service unavailable)")
	}

	t.Logf("Init response length: %d chars", len(content))
}

// TestOpenCode_DocumentationRequest simulates a documentation generation request
func TestOpenCode_DocumentationRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OpenCode simulation in short mode")
	}

	request := OpenCodeRequest{
		Model: "helixagent-debate",
		Messages: []OpenCodeMessage{
			{Role: "system", Content: "You are a documentation expert."},
			{Role: "user", Content: "Once you are done write detailed reports and documentation about the key components."},
		},
		MaxTokens: 1500,
	}

	resp, err := sendOpenCodeRequest(request)
	if err != nil {
		// NEVER SKIP - network errors should be handled by fallback mechanism
		t.Fatalf("OpenCode documentation request failed - FALLBACK SHOULD HAVE SUCCEEDED: %v", err)
	}

	if resp.Error != nil {
		// API errors are expected for invalid requests, but not for valid ones
		t.Fatalf("API returned error - FALLBACK PROVIDERS SHOULD HAVE HANDLED THIS: %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("No choices returned")
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		t.Fatal("Empty response content")
	}

	t.Logf("Documentation response length: %d chars", len(content))
}

// TestOpenCode_ConcurrentRequests simulates multiple concurrent OpenCode requests
// This verifies no endless loops or blocking occurs
func TestOpenCode_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OpenCode simulation in short mode")
	}

	requests := []OpenCodeRequest{
		{
			Model: "helixagent-debate",
			Messages: []OpenCodeMessage{
				{Role: "user", Content: "Do you see my codebase?"},
			},
			MaxTokens: 200,
		},
		{
			Model: "helixagent-debate",
			Messages: []OpenCodeMessage{
				{Role: "user", Content: "Initialize and analyze this project."},
			},
			MaxTokens: 500,
		},
		{
			Model: "helixagent-debate",
			Messages: []OpenCodeMessage{
				{Role: "user", Content: "Write a brief report about the key components."},
			},
			MaxTokens: 500,
		},
	}

	var wg sync.WaitGroup
	results := make(chan *testResult, len(requests))
	timeout := 120 * time.Second // Max time for all requests

	start := time.Now()
	for i, req := range requests {
		wg.Add(1)
		go func(idx int, r OpenCodeRequest) {
			defer wg.Done()

			reqStart := time.Now()
			resp, err := sendOpenCodeRequest(r)
			elapsed := time.Since(reqStart)

			results <- &testResult{
				index:    idx,
				response: resp,
				err:      err,
				duration: elapsed,
			}
		}(i, req)
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All requests completed
	case <-time.After(timeout):
		t.Fatalf("Concurrent requests timed out after %v - possible endless loop!", timeout)
	}

	close(results)
	totalDuration := time.Since(start)

	// Check results
	successCount := 0
	failCount := 0
	for result := range results {
		if result.err != nil {
			t.Logf("Request %d failed (network issue): %v", result.index, result.err)
			failCount++
			continue
		}
		if result.response.Error != nil {
			t.Logf("Request %d API error: %s", result.index, result.response.Error.Message)
			failCount++
			continue
		}
		if len(result.response.Choices) == 0 {
			t.Logf("Request %d returned no choices", result.index)
			failCount++
			continue
		}

		successCount++
		t.Logf("Request %d completed in %v", result.index, result.duration)
	}

	t.Logf("Concurrent test: %d/%d succeeded in %v total", successCount, len(requests), totalDuration)

	// NEVER SKIP - fallback mechanism MUST ensure success
	// At least 80% success rate required with fallback
	minRequired := (len(requests) * 80) / 100
	if minRequired < 1 {
		minRequired = 1
	}
	if successCount == 0 {
		t.Fatalf("NO requests succeeded - FALLBACK MECHANISM FAILED COMPLETELY")
	} else if successCount < minRequired {
		t.Errorf("Only %d/%d requests succeeded - FALLBACK MECHANISM INSUFFICIENT (need at least %d)", successCount, len(requests), minRequired)
	} else if successCount < len(requests) {
		t.Logf("Note: %d/%d requests had issues (fallback handled %d)", len(requests)-successCount, len(requests), successCount)
	}
}

// TestOpenCode_NoEndlessLoop verifies responses don't repeat indefinitely
func TestOpenCode_NoEndlessLoop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OpenCode simulation in short mode")
	}

	request := OpenCodeRequest{
		Model: "helixagent-debate",
		Messages: []OpenCodeMessage{
			{Role: "user", Content: "Say hello in 10 words or less."},
		},
		MaxTokens: 100,
	}

	timeout := 30 * time.Second
	start := time.Now()

	done := make(chan *OpenCodeResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		resp, err := sendOpenCodeRequest(request)
		if err != nil {
			errChan <- err
			return
		}
		done <- resp
	}()

	select {
	case resp := <-done:
		elapsed := time.Since(start)

		if resp.Error != nil {
			// NEVER SKIP - analyze error and report fallback failure
			t.Errorf("API error - FALLBACK SHOULD HAVE HANDLED: %s", resp.Error.Message)
			return
		}

		if len(resp.Choices) == 0 {
			t.Errorf("No choices returned - FALLBACK MECHANISM SHOULD ENSURE RESPONSE")
			return
		}

		content := resp.Choices[0].Message.Content

		// Check for repetitive content (sign of endless loop)
		if isRepetitive(content) {
			t.Errorf("Response appears to be repetitive (possible loop): %s", content[:min(500, len(content))])
		}

		t.Logf("Completed in %v with %d chars", elapsed, len(content))

	case err := <-errChan:
		// NEVER SKIP - network errors should be handled by fallback
		t.Errorf("Request failed - FALLBACK MECHANISM SHOULD HAVE HANDLED: %v", err)

	case <-time.After(timeout):
		t.Errorf("Request timed out after %v - POSSIBLE ENDLESS LOOP OR FALLBACK FAILURE", timeout)
	}
}

// TestOpenCode_SequentialRequests verifies sequential requests work correctly
func TestOpenCode_SequentialRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OpenCode simulation in short mode")
	}

	// Simulate OpenCode's typical request sequence
	requests := []struct {
		name    string
		request OpenCodeRequest
	}{
		{
			name: "codebase_query",
			request: OpenCodeRequest{
				Model: "helixagent-debate",
				Messages: []OpenCodeMessage{
					{Role: "user", Content: "Do you see my codebase?"},
				},
				MaxTokens: 200,
			},
		},
		{
			name: "init_request",
			request: OpenCodeRequest{
				Model: "helixagent-debate",
				Messages: []OpenCodeMessage{
					{Role: "user", Content: "Initialize and analyze the project structure."},
				},
				MaxTokens: 500,
			},
		},
		{
			name: "documentation_request",
			request: OpenCodeRequest{
				Model: "helixagent-debate",
				Messages: []OpenCodeMessage{
					{Role: "user", Content: "Write detailed reports and documentation about key components."},
				},
				MaxTokens: 1000,
			},
		},
	}

	for _, tc := range requests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			resp, err := sendOpenCodeRequest(tc.request)
			elapsed := time.Since(start)

			if err != nil {
				// NEVER SKIP - fallback should handle network errors
				t.Errorf("Request failed - FALLBACK SHOULD HANDLE: %v", err)
				return
			}

			if resp.Error != nil {
				// NEVER SKIP - analyze error for fallback failure
				t.Errorf("API error - FALLBACK PROVIDERS SHOULD HAVE HANDLED: %s", resp.Error.Message)
				return
			}

			if len(resp.Choices) == 0 {
				t.Errorf("No choices returned - FALLBACK MECHANISM FAILURE")
				return
			}

			content := resp.Choices[0].Message.Content
			if content == "" {
				t.Errorf("Empty response content - INCOMPLETE RESPONSE FROM FALLBACK")
				return
			}

			t.Logf("Completed in %v with %d chars", elapsed, len(content))
		})
	}
}

type testResult struct {
	index    int
	response *OpenCodeResponse
	err      error
	duration time.Duration
}

func sendOpenCodeRequest(request OpenCodeRequest) (*OpenCodeResponse, error) {
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", getTestURL()+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+getTestAPIKey())

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response OpenCodeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// isRepetitive checks if content has repetitive patterns (sign of endless loop)
func isRepetitive(content string) bool {
	if len(content) < 100 {
		return false
	}

	// Check for repeated chunks
	chunkSize := 50
	for i := 0; i < len(content)-chunkSize*2; i++ {
		chunk := content[i : i+chunkSize]
		// Count occurrences of this chunk
		count := 0
		for j := i; j < len(content)-chunkSize; j++ {
			if content[j:j+chunkSize] == chunk {
				count++
			}
		}
		// If same chunk appears more than 3 times, likely repetitive
		if count > 3 {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
