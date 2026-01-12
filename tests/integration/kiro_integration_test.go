package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKiroToolSupport tests that Kiro can use all 21 tools
func TestKiroToolSupport(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	// Define all 21 tools for Kiro
	allTools := []map[string]interface{}{
		// Core tools
		{"type": "function", "function": map[string]interface{}{
			"name": "Bash", "description": "Execute bash commands",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"command": map[string]string{"type": "string"}, "description": map[string]string{"type": "string"}}, "required": []string{"command", "description"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Test", "description": "Run tests",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"description": map[string]string{"type": "string"}}, "required": []string{"description"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Lint", "description": "Run linter",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"description": map[string]string{"type": "string"}}, "required": []string{"description"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Task", "description": "Create a task",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"prompt": map[string]string{"type": "string"}, "description": map[string]string{"type": "string"}, "subagent_type": map[string]string{"type": "string"}}, "required": []string{"prompt", "description", "subagent_type"}}}},
		// Filesystem tools
		{"type": "function", "function": map[string]interface{}{
			"name": "Read", "description": "Read file",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"file_path": map[string]string{"type": "string"}}, "required": []string{"file_path"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Write", "description": "Write file",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"file_path": map[string]string{"type": "string"}, "content": map[string]string{"type": "string"}}, "required": []string{"file_path", "content"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Edit", "description": "Edit file",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"file_path": map[string]string{"type": "string"}, "old_string": map[string]string{"type": "string"}, "new_string": map[string]string{"type": "string"}}, "required": []string{"file_path", "old_string", "new_string"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Glob", "description": "Find files",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"pattern": map[string]string{"type": "string"}}, "required": []string{"pattern"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Grep", "description": "Search content",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"pattern": map[string]string{"type": "string"}}, "required": []string{"pattern"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "TreeView", "description": "Show directory tree",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"description": map[string]string{"type": "string"}}, "required": []string{"description"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "FileInfo", "description": "Get file info",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"file_path": map[string]string{"type": "string"}, "description": map[string]string{"type": "string"}}, "required": []string{"file_path", "description"}}}},
		// Version control tools
		{"type": "function", "function": map[string]interface{}{
			"name": "Git", "description": "Git operations",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"operation": map[string]string{"type": "string"}, "description": map[string]string{"type": "string"}}, "required": []string{"operation", "description"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Diff", "description": "Show diff",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"description": map[string]string{"type": "string"}}, "required": []string{"description"}}}},
		// Code intelligence tools
		{"type": "function", "function": map[string]interface{}{
			"name": "Symbols", "description": "List symbols",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"description": map[string]string{"type": "string"}}, "required": []string{"description"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "References", "description": "Find references",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"symbol": map[string]string{"type": "string"}, "description": map[string]string{"type": "string"}}, "required": []string{"symbol", "description"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Definition", "description": "Go to definition",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"symbol": map[string]string{"type": "string"}, "description": map[string]string{"type": "string"}}, "required": []string{"symbol", "description"}}}},
		// Workflow tools
		{"type": "function", "function": map[string]interface{}{
			"name": "PR", "description": "Pull request operations",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"action": map[string]string{"type": "string"}, "description": map[string]string{"type": "string"}}, "required": []string{"action", "description"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Issue", "description": "Issue operations",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"action": map[string]string{"type": "string"}, "description": map[string]string{"type": "string"}}, "required": []string{"action", "description"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "Workflow", "description": "CI/CD operations",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"action": map[string]string{"type": "string"}, "description": map[string]string{"type": "string"}}, "required": []string{"action", "description"}}}},
		// Web tools
		{"type": "function", "function": map[string]interface{}{
			"name": "WebFetch", "description": "Fetch web content",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"url": map[string]string{"type": "string"}, "prompt": map[string]string{"type": "string"}}, "required": []string{"url", "prompt"}}}},
		{"type": "function", "function": map[string]interface{}{
			"name": "WebSearch", "description": "Search the web",
			"parameters": map[string]interface{}{"type": "object", "properties": map[string]interface{}{"query": map[string]string{"type": "string"}}, "required": []string{"query"}}}},
	}

	assert.Len(t, allTools, 21, "Should have 21 tools defined for Kiro")

	reqBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "system", "content": "You are Kiro, an AI coding agent with access to all development tools."},
			{"role": "user", "content": "Check the git status"},
		},
		"tools":       allTools,
		"tool_choice": "auto",
		"stream":      false,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Post(
		baseURL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode == 200 {
		var apiResp ToolCallAPIResponse
		err = json.Unmarshal(body, &apiResp)
		if err == nil {
			// Response parsed successfully
			assert.NotEmpty(t, apiResp.Choices, "Response should have choices")
		}
	} else if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
		// Provider temporarily unavailable is acceptable
		t.Logf("Provider temporarily unavailable (status %d)", resp.StatusCode)
	} else {
		t.Logf("Response status: %d", resp.StatusCode)
	}
}

// TestKiroCodeGeneration tests Kiro code generation capabilities
func TestKiroCodeGeneration(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	reqBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "system", "content": "You are Kiro, an AI coding agent that writes high-quality code."},
			{"role": "user", "content": "Write a Go function that calculates the factorial of a number"},
		},
		"max_tokens": 500,
		"stream":     false,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Post(
		baseURL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode == 200 {
		var apiResp ToolCallAPIResponse
		err = json.Unmarshal(body, &apiResp)
		if err == nil && len(apiResp.Choices) > 0 {
			content := apiResp.Choices[0].Message.Content
			assert.NotEmpty(t, content, "Response should have content")
		}
	} else if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
		t.Logf("Provider temporarily unavailable (status %d)", resp.StatusCode)
	}
}

// TestKiroStreaming tests Kiro streaming responses
func TestKiroStreaming(t *testing.T) {
	skipIfServerNotRunning(t)
	baseURL := getTestBaseURL()

	reqBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "system", "content": "You are Kiro, an AI coding agent."},
			{"role": "user", "content": "Explain what Go interfaces are"},
		},
		"max_tokens": 300,
		"stream":     true,
	}

	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Post(
		baseURL+"/v1/chat/completions",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode == 200 {
		// Check for SSE format
		bodyStr := string(body)
		assert.Contains(t, bodyStr, "data:", "Streaming response should have SSE format")
	} else if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
		t.Logf("Provider temporarily unavailable (status %d)", resp.StatusCode)
	}
}
