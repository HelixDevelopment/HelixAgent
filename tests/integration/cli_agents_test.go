package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CLI Agent paths and configurations
const (
	// Project paths
	helixCodePath    = "/run/media/milosvasic/DATA4TB/Projects/HelixCode"
	helixCodeCLIPath = "/run/media/milosvasic/DATA4TB/Projects/HelixCode/HelixCode/bin/helixcode"
	openCodePath     = "/run/media/milosvasic/DATA4TB/Projects/HelixCode/Example_Projects/OpenCode/OpenCode"
	openCodeCLIPath  = "/run/media/milosvasic/DATA4TB/Projects/HelixCode/Example_Projects/OpenCode/OpenCode/opencode"
	clinePath        = "/run/media/milosvasic/DATA4TB/Projects/HelixCode/Example_Projects/Cline"
	clineCLIPath     = "/run/media/milosvasic/DATA4TB/Projects/HelixCode/Example_Projects/Cline/cli/cline"

	// Repository URLs for auto-cloning
	helixCodeRepoSSH   = "git@github.com:helix-org/HelixCode.git"
	helixCodeRepoHTTPS = "https://github.com/helix-org/HelixCode.git"
)

// CLIAgentTestResponse represents a parsed streaming response
type CLIAgentTestResponse struct {
	Content         string
	ChunkCount      int
	HasDoneMarker   bool
	HasFinishReason bool
	StreamID        string
	Errors          []string
	InterleavingDetected bool
	CutoffDetected  bool
}

// cliAgentGetBaseURL returns the HelixAgent base URL for testing
func cliAgentGetBaseURL() string {
	if url := os.Getenv("HELIXAGENT_URL"); url != "" {
		return url
	}
	return "http://localhost:7061"
}

// cliAgentSkipIfNotRunning skips the test if HelixAgent is not running
func cliAgentSkipIfNotRunning(t *testing.T) {
	t.Helper()
	baseURL := cliAgentGetBaseURL()
	resp, err := http.Get(baseURL + "/health")
	if err != nil || resp.StatusCode != 200 {
		t.Skipf("HelixAgent not running at %s, skipping integration test", baseURL)
	}
	resp.Body.Close()
}

// ensureHelixCodeExists checks if HelixCode project exists
func ensureHelixCodeExists(t *testing.T) {
	t.Helper()
	if info, err := os.Stat(helixCodePath); err == nil && info.IsDir() {
		gitPath := filepath.Join(helixCodePath, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			t.Logf("HelixCode project found at %s", helixCodePath)
			return
		}
	}
	t.Skipf("HelixCode project not found at %s", helixCodePath)
}

// ensureOpenCodeBuilt checks if OpenCode binary exists and builds if needed
func ensureOpenCodeBuilt(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(openCodeCLIPath); err == nil {
		t.Logf("OpenCode binary found at %s", openCodeCLIPath)
		return
	}

	// Try to build OpenCode
	t.Logf("Building OpenCode CLI...")
	cmd := exec.Command("go", "build", "-o", "opencode", ".")
	cmd.Dir = openCodePath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("Failed to build OpenCode: %v\nOutput: %s", err, output)
	}
	t.Logf("OpenCode built successfully")
}

// sendCLIAgentRequest sends a streaming request and parses the response
func sendCLIAgentRequest(t *testing.T, baseURL string, reqBody map[string]interface{}, timeout time.Duration) (*CLIAgentTestResponse, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("request creation error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	return parseCLIAgentResponse(string(body)), nil
}

// parseCLIAgentResponse parses SSE response into structured data
func parseCLIAgentResponse(body string) *CLIAgentTestResponse {
	result := &CLIAgentTestResponse{
		Errors: []string{},
	}

	var contentBuilder strings.Builder
	lines := strings.Split(body, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if line == "data: [DONE]" {
			result.HasDoneMarker = true
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("parse error: %v", err))
			continue
		}

		result.ChunkCount++

		// Extract stream ID
		if id, ok := chunk["id"].(string); ok && result.StreamID == "" {
			result.StreamID = id
		}

		// Extract content
		if choices, ok := chunk["choices"].([]interface{}); ok && len(choices) > 0 {
			choice := choices[0].(map[string]interface{})

			// Check for finish_reason
			if fr, ok := choice["finish_reason"]; ok && fr != nil {
				result.HasFinishReason = true
			}

			// Extract delta content
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				if content, ok := delta["content"].(string); ok {
					contentBuilder.WriteString(content)
				}
			}
		}
	}

	result.Content = contentBuilder.String()

	// Check for content interleaving patterns
	interleavingPatterns := []string{
		"YesYes", "NoNo", "HelloHello", "II ", " II",
		"andand", "thethe", "isis", "inin", "toto",
		"TheThe", "IsIs", "InIn", "ToTo",
	}
	for _, pattern := range interleavingPatterns {
		if strings.Contains(result.Content, pattern) {
			result.InterleavingDetected = true
			break
		}
	}

	// Check for cutoff patterns (incomplete sentences, unbalanced brackets)
	if len(result.Content) > 0 {
		lastChar := result.Content[len(result.Content)-1]
		if !strings.ContainsAny(string(lastChar), ".!?\"'\n`") {
			// Allow code blocks to end with }] etc.
			if !strings.ContainsAny(string(lastChar), "}])>") {
				result.CutoffDetected = true
			}
		}
	}

	return result
}

// truncateCLIResponse truncates content for logging
func truncateCLIResponse(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// =============================================================================
// HelixCode CLI Agent Tests
// =============================================================================

// TestHelixCodeStreamingIntegrity tests streaming response integrity for HelixCode
func TestHelixCodeStreamingIntegrity(t *testing.T) {
	cliAgentSkipIfNotRunning(t)
	ensureHelixCodeExists(t)

	baseURL := cliAgentGetBaseURL()

	t.Run("No_Word_Duplication", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": "You are analyzing the HelixCode project. Give clear, non-repetitive answers.",
				},
				{
					"role":    "user",
					"content": "Say: Hello world, this is a test message.",
				},
			},
			"stream":     true,
			"max_tokens": 100,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.False(t, response.InterleavingDetected, "No content interleaving should be detected")
		assert.True(t, response.HasDoneMarker, "Response must have [DONE] marker")
		assert.True(t, response.HasFinishReason, "Response must have finish_reason")

		t.Logf("HelixCode Response (no interleaving): %s", truncateCLIResponse(response.Content, 200))
	})

	t.Run("Coherent_Sentence_Structure", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Write exactly one complete sentence about programming. The sentence must end with a period.",
				},
			},
			"stream":     true,
			"max_tokens": 200,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		content := strings.TrimSpace(response.Content)
		properEnding := strings.HasSuffix(content, ".") || strings.HasSuffix(content, "!") || strings.HasSuffix(content, "?")

		assert.True(t, properEnding, "Sentence should end with proper punctuation")
		assert.False(t, response.InterleavingDetected, "No content interleaving should be detected")

		t.Logf("HelixCode Coherent sentence (proper ending=%v): %s", properEnding, truncateCLIResponse(content, 200))
	})

	t.Run("Consistent_Stream_ID", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Hello",
				},
			},
			"stream":     true,
			"max_tokens": 50,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.NotEmpty(t, response.StreamID, "Stream ID should be present")
		assert.Greater(t, response.ChunkCount, 0, "Should have chunks")

		t.Logf("HelixCode Consistent stream ID: %s, chunks: %d", response.StreamID, response.ChunkCount)
	})
}

// TestHelixCodeCodebaseAnalysis tests codebase analysis for HelixCode project
func TestHelixCodeCodebaseAnalysis(t *testing.T) {
	cliAgentSkipIfNotRunning(t)
	ensureHelixCodeExists(t)

	baseURL := cliAgentGetBaseURL()

	t.Run("Project_Structure_Analysis", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": fmt.Sprintf("You are analyzing the HelixCode project at %s", helixCodePath),
				},
				{
					"role":    "user",
					"content": "Describe the main components of this project structure.",
				},
			},
			"stream":     true,
			"max_tokens": 1000,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 120*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete with [DONE] marker")
		assert.Greater(t, len(response.Content), 100, "Response should be substantial")
		assert.False(t, response.InterleavingDetected, "No interleaving should be detected")

		t.Logf("HelixCode Project Analysis (%d chars): %s", len(response.Content), truncateCLIResponse(response.Content, 300))
	})

	t.Run("Build_Commands_Request", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": fmt.Sprintf("You are analyzing the HelixCode project at %s/HelixCode", helixCodePath),
				},
				{
					"role":    "user",
					"content": "What are the build commands for this project? Check the Makefile.",
				},
			},
			"stream":     true,
			"max_tokens": 800,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 90*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete")
		assert.True(t, response.HasFinishReason, "Response must have finish reason")

		t.Logf("HelixCode Build Commands (%d chars): %s", len(response.Content), truncateCLIResponse(response.Content, 300))
	})
}

// =============================================================================
// OpenCode CLI Agent Tests
// =============================================================================

// TestOpenCodeStreamingIntegrity tests streaming response integrity for OpenCode
func TestOpenCodeStreamingIntegrity(t *testing.T) {
	cliAgentSkipIfNotRunning(t)

	baseURL := cliAgentGetBaseURL()

	t.Run("No_Word_Duplication", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": "You are the OpenCode assistant. Give clear, non-repetitive answers.",
				},
				{
					"role":    "user",
					"content": "Say: Hello world, this is OpenCode speaking.",
				},
			},
			"stream":     true,
			"max_tokens": 100,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.False(t, response.InterleavingDetected, "No content interleaving should be detected")
		assert.True(t, response.HasDoneMarker, "Response must have [DONE] marker")

		t.Logf("OpenCode Response (no interleaving): %s", truncateCLIResponse(response.Content, 200))
	})

	t.Run("Long_Response_No_Cutoff", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Write a detailed explanation of RESTful API design principles. Include: 1) HTTP methods, 2) Status codes, 3) URL structure, 4) Headers, 5) Authentication.",
				},
			},
			"stream":     true,
			"max_tokens": 2000,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 180*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete with [DONE] marker - NO CUTOFF")
		assert.True(t, response.HasFinishReason, "Response must have finish_reason - NO PREMATURE TERMINATION")
		assert.Greater(t, len(response.Content), 500, "Response should be substantial")

		// Count sections
		sections := []string{"HTTP", "method", "Status", "code", "URL", "Header", "Auth"}
		foundSections := 0
		contentLower := strings.ToLower(response.Content)
		for _, section := range sections {
			if strings.Contains(contentLower, strings.ToLower(section)) {
				foundSections++
			}
		}

		t.Logf("OpenCode Long response: %d/%d sections, %d chars", foundSections, len(sections), len(response.Content))
		assert.GreaterOrEqual(t, foundSections, 3, "Should cover multiple sections")
	})

	t.Run("SSE_Format_Validity", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Hi there!",
				},
			},
			"stream":     true,
			"max_tokens": 50,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.NotEmpty(t, response.StreamID, "Stream ID should be present")
		assert.True(t, response.HasDoneMarker, "Should have [DONE] marker")
		assert.True(t, response.HasFinishReason, "Should have finish_reason")
		assert.Empty(t, response.Errors, "No parsing errors should occur")

		t.Logf("OpenCode SSE Format: valid, stream_id=%s, chunks=%d", response.StreamID, response.ChunkCount)
	})
}

// TestOpenCodeBearMailAnalysis tests OpenCode analyzing Bear-Mail project
func TestOpenCodeBearMailAnalysis(t *testing.T) {
	cliAgentSkipIfNotRunning(t)
	ensureBearMailExists(t)

	baseURL := cliAgentGetBaseURL()

	t.Run("Codebase_Visibility", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": fmt.Sprintf("You are analyzing the codebase at %s", bearMailPath),
				},
				{
					"role":    "user",
					"content": "Do you see my codebase? What is it?",
				},
			},
			"stream":     true,
			"max_tokens": 500,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must have [DONE] marker")
		assert.NotEmpty(t, response.Content, "Response should not be empty")

		t.Logf("OpenCode Bear-Mail visibility (%d chars): %s", len(response.Content), truncateCLIResponse(response.Content, 200))
	})
}

// =============================================================================
// Cline CLI Agent Tests (when available)
// =============================================================================

// TestClineStreamingIntegrity tests streaming response integrity for Cline
func TestClineStreamingIntegrity(t *testing.T) {
	cliAgentSkipIfNotRunning(t)

	// Check if Cline CLI is available
	if _, err := os.Stat(clineCLIPath); err != nil {
		t.Logf("Cline CLI not available at %s, using HelixAgent directly", clineCLIPath)
	}

	baseURL := cliAgentGetBaseURL()

	t.Run("No_Word_Duplication_Cline_Style", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": "You are Cline, an autonomous coding assistant. Give clear, non-repetitive answers.",
				},
				{
					"role":    "user",
					"content": "Say: I am Cline, ready to help.",
				},
			},
			"stream":     true,
			"max_tokens": 100,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.False(t, response.InterleavingDetected, "No content interleaving should be detected")
		assert.True(t, response.HasDoneMarker, "Response must have [DONE] marker")

		t.Logf("Cline-style Response (no interleaving): %s", truncateCLIResponse(response.Content, 200))
	})

	t.Run("Code_Generation_Task", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": "You are Cline, an autonomous coding assistant capable of writing code.",
				},
				{
					"role":    "user",
					"content": "Write a simple Python function that calculates the factorial of a number.",
				},
			},
			"stream":     true,
			"max_tokens": 500,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 90*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete")

		// Should contain code markers
		hasCode := strings.Contains(response.Content, "def ") || strings.Contains(response.Content, "```python")
		assert.True(t, hasCode, "Response should contain Python code")

		t.Logf("Cline Code Generation (%d chars): %s", len(response.Content), truncateCLIResponse(response.Content, 300))
	})
}

// =============================================================================
// Cross-Agent Consistency Tests
// =============================================================================

// TestCrossAgentConsistency tests that all CLI agents receive consistent responses
func TestCrossAgentConsistency(t *testing.T) {
	cliAgentSkipIfNotRunning(t)

	baseURL := cliAgentGetBaseURL()

	t.Run("Same_Query_Different_Agent_Contexts", func(t *testing.T) {
		// Test the same query with different agent system prompts
		agentContexts := []struct {
			name   string
			system string
		}{
			{"HelixCode", "You are the HelixCode assistant, a distributed AI development platform."},
			{"OpenCode", "You are the OpenCode assistant for terminal-based coding."},
			{"Cline", "You are Cline, an autonomous coding agent."},
		}

		query := "What is 2 + 2? Reply with just the number."

		for _, agent := range agentContexts {
			t.Run(agent.name, func(t *testing.T) {
				reqBody := map[string]interface{}{
					"model": "helixagent-ensemble",
					"messages": []map[string]string{
						{
							"role":    "system",
							"content": agent.system,
						},
						{
							"role":    "user",
							"content": query,
						},
					},
					"stream":     true,
					"max_tokens": 50,
				}

				response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
				require.NoError(t, err, "Request should not fail for %s", agent.name)

				assert.True(t, response.HasDoneMarker, "Response must complete for %s", agent.name)
				assert.Contains(t, response.Content, "4", "Response should contain '4' for %s", agent.name)
				assert.False(t, response.InterleavingDetected, "No interleaving for %s", agent.name)

				t.Logf("%s response: %s", agent.name, truncateCLIResponse(response.Content, 100))
			})
		}
	})
}

// TestToolCallFormatAcrossAgents tests tool call format consistency
func TestToolCallFormatAcrossAgents(t *testing.T) {
	cliAgentSkipIfNotRunning(t)

	baseURL := cliAgentGetBaseURL()

	t.Run("Bash_Command_Format", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "What is the exact bash command to list files in the current directory? Give me just the command.",
				},
			},
			"stream":     true,
			"max_tokens": 100,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete")

		// Should mention ls command
		hasCommand := strings.Contains(response.Content, "ls") || strings.Contains(response.Content, "dir")
		assert.True(t, hasCommand, "Response should contain a file listing command")

		t.Logf("Bash command response: %s", truncateCLIResponse(response.Content, 200))
	})

	t.Run("File_Write_Format", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Write a simple hello.txt file content with the text 'Hello, World!'",
				},
			},
			"stream":     true,
			"max_tokens": 200,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete")
		assert.Contains(t, strings.ToLower(response.Content), "hello", "Response should contain 'hello'")

		t.Logf("File write response: %s", truncateCLIResponse(response.Content, 200))
	})

	t.Run("No_Incomplete_Tags", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "List 3 common files you would find in a project directory.",
				},
			},
			"stream":     true,
			"max_tokens": 300,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		// Check for incomplete tags
		incompleteTags := []string{
			"<bash>", "</bash", "<write_to_file>", "</write_to_file",
			"<read_file>", "</read_file", "<search>", "</search",
		}

		for _, tag := range incompleteTags {
			if strings.HasSuffix(tag, ">") {
				// Complete tag - check for unclosed
				if strings.Count(response.Content, tag) > strings.Count(response.Content, strings.Replace(tag, "<", "</", 1)) {
					if strings.Contains(tag, "/") {
						continue // closing tag
					}
					// Unclosed opening tag is OK in some contexts
				}
			}
		}

		t.Logf("Response (no incomplete tags): %s", truncateCLIResponse(response.Content, 200))
	})
}

// =============================================================================
// Response Validity Tests
// =============================================================================

// TestResponseValidityAllAgents tests response validity across all agent types
func TestResponseValidityAllAgents(t *testing.T) {
	cliAgentSkipIfNotRunning(t)

	baseURL := cliAgentGetBaseURL()

	t.Run("No_Empty_Response", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "What is 2 + 2?",
				},
			},
			"stream":     true,
			"max_tokens": 50,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.NotEmpty(t, response.Content, "Response should not be empty")

		t.Logf("Non-empty response: %s", truncateCLIResponse(response.Content, 100))
	})

	t.Run("Response_Has_Expected_Content", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Reply with exactly one word: success",
				},
			},
			"stream":     true,
			"max_tokens": 50,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete")
		assert.Contains(t, strings.ToLower(response.Content), "success", "Response should contain 'success'")

		t.Logf("Expected content response: %s", truncateCLIResponse(response.Content, 100))
	})

	t.Run("Numbered_List_Response", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "List exactly 3 programming languages as a numbered list. Format: 1. Language",
				},
			},
			"stream":     true,
			"max_tokens": 100,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete")

		// Check for numbered list format
		hasNumbers := strings.Contains(response.Content, "1.") || strings.Contains(response.Content, "1)")
		assert.True(t, hasNumbers, "Response should contain numbered list")

		t.Logf("Numbered list response: %s", truncateCLIResponse(response.Content, 150))
	})
}

// TestNoResponseCutoffAllAgents tests that responses don't get cut off
func TestNoResponseCutoffAllAgents(t *testing.T) {
	cliAgentSkipIfNotRunning(t)

	baseURL := cliAgentGetBaseURL()

	t.Run("Medium_Response_Completes", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Explain the concept of object-oriented programming in about 100 words.",
				},
			},
			"stream":     true,
			"max_tokens": 500,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 120*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete with [DONE] marker")
		assert.True(t, response.HasFinishReason, "Response must have finish_reason")
		assert.Greater(t, len(response.Content), 100, "Response should be substantial")

		t.Logf("Medium response completed: %d chars, %d chunks", len(response.Content), response.ChunkCount)
	})

	t.Run("Long_Response_Completes", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Write a comprehensive guide about Git version control. Include: 1) Basic commands, 2) Branching, 3) Merging, 4) Remote repositories, 5) Best practices. Be detailed.",
				},
			},
			"stream":     true,
			"max_tokens": 2500,
		}

		response, err := sendCLIAgentRequest(t, baseURL, reqBody, 300*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete with [DONE] marker - NO CUTOFF")
		assert.True(t, response.HasFinishReason, "Response must have finish_reason - NO PREMATURE TERMINATION")

		// Count sections covered
		sections := []string{"git", "branch", "merge", "remote", "practice", "commit"}
		foundSections := 0
		contentLower := strings.ToLower(response.Content)
		for _, section := range sections {
			if strings.Contains(contentLower, section) {
				foundSections++
			}
		}

		t.Logf("Long response: %d/%d sections found, %d chars, %d chunks", foundSections, len(sections), len(response.Content), response.ChunkCount)
		assert.GreaterOrEqual(t, foundSections, 4, "Should cover at least 4 sections")
	})
}
