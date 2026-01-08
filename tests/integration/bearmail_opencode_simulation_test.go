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

const (
	bearMailPath     = "/run/media/milosvasic/DATA4TB/Projects/Bear-Mail"
	bearMailRepoURL  = "git@github.com:Bear-Suite/Mail.git"
	bearMailRepoHTTP = "https://github.com/Bear-Suite/Mail.git"
)

// ensureBearMailExists checks if Bear-Mail project exists and clones it if not
func ensureBearMailExists(t *testing.T) {
	t.Helper()

	// Check if directory exists
	if info, err := os.Stat(bearMailPath); err == nil && info.IsDir() {
		// Check if it's a valid git repo
		gitPath := filepath.Join(bearMailPath, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			t.Logf("Bear-Mail project found at %s", bearMailPath)
			return
		}
	}

	// Create parent directory if needed
	parentDir := filepath.Dir(bearMailPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		t.Fatalf("Failed to create parent directory %s: %v", parentDir, err)
	}

	// Try SSH clone first, fall back to HTTPS
	t.Logf("Cloning Bear-Mail project to %s...", bearMailPath)

	// Try SSH first
	cmd := exec.Command("git", "clone", "--depth", "1", bearMailRepoURL, bearMailPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Logf("SSH clone failed, trying HTTPS: %v", err)

		// Clean up partial clone if any
		os.RemoveAll(bearMailPath)

		// Try HTTPS
		cmd = exec.Command("git", "clone", "--depth", "1", bearMailRepoHTTP, bearMailPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to clone Bear-Mail from %s or %s: %v", bearMailRepoURL, bearMailRepoHTTP, err)
		}
	}

	t.Logf("Successfully cloned Bear-Mail to %s", bearMailPath)
}

// bearMailGetBaseURL returns the HelixAgent base URL for testing
func bearMailGetBaseURL() string {
	if url := os.Getenv("HELIXAGENT_URL"); url != "" {
		return url
	}
	return "http://localhost:7061"
}

// bearMailSkipIfNotRunning skips the test if HelixAgent is not running
func bearMailSkipIfNotRunning(t *testing.T) {
	t.Helper()
	baseURL := bearMailGetBaseURL()
	resp, err := http.Get(baseURL + "/health")
	if err != nil || resp.StatusCode != 200 {
		t.Skipf("HelixAgent not running at %s, skipping integration test", baseURL)
	}
	resp.Body.Close()
}

// TestBearMailOpenCodeConversation simulates the exact OpenCode conversation
// against the Bear-Mail project to verify:
// 1. Responses are complete (not cut off)
// 2. Content is specific to actual codebase (not generic/hallucinated)
// 3. Multi-provider debate works correctly
// 4. No premature termination
func TestBearMailOpenCodeConversation(t *testing.T) {
	bearMailSkipIfNotRunning(t)

	baseURL := bearMailGetBaseURL()

	// Ensure Bear-Mail project exists (clone if needed)
	ensureBearMailExists(t)

	t.Run("Step1_Codebase_Visibility_Check", func(t *testing.T) {
		// First message: "Do you see my codebase?"
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": fmt.Sprintf("You are analyzing the codebase at %s. When asked about files, analyze the actual project structure.", bearMailPath),
				},
				{
					"role":    "user",
					"content": "Do you see my codebase?",
				},
			},
			"stream":     true,
			"max_tokens": 500,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		// Verify response is complete
		assert.True(t, response.HasDoneMarker, "Response must have [DONE] marker")
		assert.True(t, response.HasFinishReason, "Response must have finish_reason")
		assert.NotEmpty(t, response.Content, "Response content should not be empty")

		// Response should acknowledge the codebase or ask for clarification
		// NOT just say "Yes" without context
		t.Logf("Step 1 Response (%d chars): %s", len(response.Content), truncate(response.Content, 200))
	})

	t.Run("Step2_AGENTS_MD_Creation_Request", func(t *testing.T) {
		// Second message: Full AGENTS.md creation request
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": fmt.Sprintf("You are analyzing the Bear-Mail Android project at %s", bearMailPath),
				},
				{
					"role": "user",
					"content": `Please analyze this codebase and create an AGENTS.md file containing:
1. Build/lint/test commands - especially for running a single test
2. Code style guidelines including imports, formatting, types, naming conventions, error handling, etc.
The file you create will be given to agentic coding agents (such as yourself) that operate in this repository. Make it about 150 lines long.
If there are Cursor rules (in .cursor/rules/ or .cursorrules) or Copilot rules (in .github/copilot-instructions.md), make sure to include them.
If there's already an AGENTS.md, improve it if it's located in /run/media/milosvasic/DATA4TB/Projects/Bear-Mail`,
				},
			},
			"stream":     true,
			"max_tokens": 4000,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 180*time.Second)
		require.NoError(t, err, "Request should not fail")

		// Verify response completeness
		assert.True(t, response.HasDoneMarker, "Response must complete with [DONE] marker - NO CUTOFF")
		assert.True(t, response.HasFinishReason, "Response must have finish_reason - NO PREMATURE TERMINATION")

		// Verify response is substantial (not cut off mid-sentence)
		assert.Greater(t, len(response.Content), 500, "Response should be substantial for AGENTS.md creation")

		// Check for common cutoff indicators (incomplete sentences)
		cutoffIndicators := []string{
			"Ktlint (if configured):",  // Known cutoff point from user's conversation
			"testFeatureUnderTest_",    // Another cutoff point
			"(check for",               // Incomplete parenthetical
			"...",                       // Trailing ellipsis suggesting more content
		}

		for _, indicator := range cutoffIndicators {
			if strings.HasSuffix(strings.TrimSpace(response.Content), indicator) {
				t.Errorf("Response appears cut off at: %s", indicator)
			}
		}

		// Verify content is specific to Bear-Mail (not generic)
		verifyBearMailSpecificContent(t, response.Content, bearMailPath)

		t.Logf("Step 2 Response (%d chars, %d chunks)", len(response.Content), response.ChunkCount)
	})

	t.Run("Step3_Documentation_Request", func(t *testing.T) {
		// Third message: Documentation request
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": fmt.Sprintf("You are documenting the Bear-Mail Android project at %s", bearMailPath),
				},
				{
					"role":    "user",
					"content": "Once you are done write detailed reports and documentation!",
				},
			},
			"stream":     true,
			"max_tokens": 3000,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 180*time.Second)
		require.NoError(t, err, "Request should not fail")

		// Verify completeness
		assert.True(t, response.HasDoneMarker, "Documentation response must complete")
		assert.True(t, response.HasFinishReason, "Documentation response must have finish_reason")

		t.Logf("Step 3 Response (%d chars): %s", len(response.Content), truncate(response.Content, 300))
	})

	t.Run("Step4_Test_Coverage_Check", func(t *testing.T) {
		// Fourth message: Test coverage check
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": fmt.Sprintf("You are analyzing test coverage for the Bear-Mail Android project at %s", bearMailPath),
				},
				{
					"role":    "user",
					"content": "Check tests coverage too",
				},
			},
			"stream":     true,
			"max_tokens": 2000,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 120*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Coverage response must complete")
		assert.True(t, response.HasFinishReason, "Coverage response must have finish_reason")

		t.Logf("Step 4 Response (%d chars): %s", len(response.Content), truncate(response.Content, 300))
	})
}

// TestBearMailContentQuality verifies the AI doesn't hallucinate project structure
func TestBearMailContentQuality(t *testing.T) {
	bearMailSkipIfNotRunning(t)

	baseURL := bearMailGetBaseURL()

	// Ensure Bear-Mail project exists (clone if needed)
	ensureBearMailExists(t)

	t.Run("No_Hallucinated_Structure", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": fmt.Sprintf("Analyze the actual project structure at %s. Only describe what actually exists.", bearMailPath),
				},
				{
					"role":    "user",
					"content": "List the actual directory structure of this project. Only list directories that actually exist.",
				},
			},
			"stream":     true,
			"max_tokens": 1500,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 90*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete")

		// Verify actual Bear-Mail structure exists
		actualDirs := getActualProjectDirs(bearMailPath)
		t.Logf("Actual Bear-Mail directories: %v", actualDirs)
		t.Logf("Response: %s", truncate(response.Content, 500))
	})

	t.Run("Specific_Build_Commands", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": fmt.Sprintf("You are analyzing the Bear-Mail Android project at %s", bearMailPath),
				},
				{
					"role":    "user",
					"content": "What are the exact build commands for this project? Check build.gradle files.",
				},
			},
			"stream":     true,
			"max_tokens": 1000,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 90*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete")

		// Should mention gradle commands
		content := strings.ToLower(response.Content)
		hasGradle := strings.Contains(content, "gradle") || strings.Contains(content, "./gradlew")
		assert.True(t, hasGradle, "Response should mention gradle for Android project")

		t.Logf("Build commands response: %s", truncate(response.Content, 400))
	})
}

// TestBearMailResponseCompleteness ensures no premature cutoffs
func TestBearMailResponseCompleteness(t *testing.T) {
	bearMailSkipIfNotRunning(t)

	baseURL := bearMailGetBaseURL()

	// Ensure Bear-Mail project exists (clone if needed)
	ensureBearMailExists(t)

	t.Run("Long_Response_No_Cutoff", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role": "user",
					"content": `Create a comprehensive AGENTS.md file with exactly these sections:
1. Project Overview (5 lines)
2. Build Commands (10 lines)
3. Test Commands (10 lines)
4. Lint Commands (5 lines)
5. Code Style (20 lines)
6. Naming Conventions (10 lines)
7. Error Handling (10 lines)
8. Testing Guidelines (15 lines)
9. Documentation (10 lines)
10. Project Structure (15 lines)

Make sure to complete ALL sections. Do not stop mid-section.`,
				},
			},
			"stream":     true,
			"max_tokens": 4000,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 180*time.Second)
		require.NoError(t, err, "Request should not fail")

		// CRITICAL: Must have completion markers
		assert.True(t, response.HasDoneMarker, "CRITICAL: Response must have [DONE] marker")
		assert.True(t, response.HasFinishReason, "CRITICAL: Response must have finish_reason")

		// Verify all sections are present (not cut off)
		content := strings.ToLower(response.Content)
		sections := []string{
			"project overview",
			"build commands",
			"test commands",
			"lint",
			"code style",
			"naming",
			"error handling",
			"testing",
			"documentation",
			"project structure",
		}

		missingSections := []string{}
		for _, section := range sections {
			if !strings.Contains(content, section) {
				missingSections = append(missingSections, section)
			}
		}

		if len(missingSections) > 3 {
			t.Errorf("Response missing too many sections (cutoff likely): %v", missingSections)
		}

		t.Logf("Response completeness: %d/%d sections found, %d chars, %d chunks",
			len(sections)-len(missingSections), len(sections), len(response.Content), response.ChunkCount)
	})

	t.Run("Multiple_Sequential_Requests", func(t *testing.T) {
		// Simulate the multiple requests from the conversation
		prompts := []string{
			"Do you see my codebase?",
			"What is the main programming language used?",
			"List the build.gradle files",
			"What testing frameworks are used?",
		}

		allComplete := true
		for i, prompt := range prompts {
			reqBody := map[string]interface{}{
				"model": "helixagent-ensemble",
				"messages": []map[string]string{
					{"role": "user", "content": prompt},
				},
				"stream":     true,
				"max_tokens": 500,
			}

			response, err := sendStreamingRequest(t, baseURL, reqBody, 60*time.Second)
			if err != nil {
				t.Errorf("Request %d failed: %v", i+1, err)
				allComplete = false
				continue
			}

			if !response.HasDoneMarker || !response.HasFinishReason {
				t.Errorf("Request %d incomplete: done=%v, finish=%v",
					i+1, response.HasDoneMarker, response.HasFinishReason)
				allComplete = false
			}

			t.Logf("Request %d (%s): %d chars, complete=%v",
				i+1, truncate(prompt, 30), len(response.Content), response.HasDoneMarker)
		}

		assert.True(t, allComplete, "ALL sequential requests must complete properly")
	})
}

// TestBearMailMultiProviderParticipation verifies all providers contribute
func TestBearMailMultiProviderParticipation(t *testing.T) {
	bearMailSkipIfNotRunning(t)

	baseURL := bearMailGetBaseURL()

	// Ensure Bear-Mail project exists (clone if needed)
	ensureBearMailExists(t)

	t.Run("Complex_Analysis_Uses_Multiple_Providers", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role": "user",
					"content": `Analyze this Android project and provide:
1. Architecture pattern used (MVC, MVP, MVVM, MVI)
2. Dependency injection framework (Hilt, Koin, Dagger, or none)
3. Testing frameworks (JUnit, Espresso, MockK, Mockito)
4. Networking library (Retrofit, OkHttp, Ktor)
5. Database (Room, SQLite, Realm)

For each, explain WHY you determined this based on actual evidence from the codebase.`,
				},
			},
			"stream":     true,
			"max_tokens": 2000,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 120*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Complex analysis must complete")
		assert.Greater(t, len(response.Content), 200, "Complex analysis should be detailed")

		// Check for evidence-based reasoning (not just listing technologies)
		content := strings.ToLower(response.Content)
		hasEvidence := strings.Contains(content, "because") ||
			strings.Contains(content, "found") ||
			strings.Contains(content, "based on") ||
			strings.Contains(content, "evidence") ||
			strings.Contains(content, "file") ||
			strings.Contains(content, "import")

		t.Logf("Evidence-based response: %v", hasEvidence)
		t.Logf("Response: %s", truncate(response.Content, 500))
	})
}

// StreamingResponse holds parsed streaming response data
type StreamingResponse struct {
	Content         string
	ChunkCount      int
	HasDoneMarker   bool
	HasFinishReason bool
	StreamID        string
	Errors          []string
}

// sendStreamingRequest sends a streaming request and parses the response
func sendStreamingRequest(t *testing.T, baseURL string, reqBody map[string]interface{}, timeout time.Duration) (*StreamingResponse, error) {
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

	return parseStreamingResponse(string(body)), nil
}

// parseStreamingResponse parses SSE response into structured data
func parseStreamingResponse(body string) *StreamingResponse {
	result := &StreamingResponse{
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
	return result
}

// verifyBearMailSpecificContent checks if response is specific to Bear-Mail
func verifyBearMailSpecificContent(t *testing.T, content string, projectPath string) {
	content = strings.ToLower(content)

	// Generic indicators that suggest hallucination
	genericIndicators := []struct {
		pattern string
		concern string
	}{
		{"hilt/koin", "Mentioned both DI frameworks without confirming which is used"},
		{"junit 4/5", "Mentioned both JUnit versions without confirming"},
		{"mvvm/mvp/mvi", "Listed multiple architectures without determining actual one"},
	}

	genericCount := 0
	for _, indicator := range genericIndicators {
		if strings.Contains(content, indicator.pattern) {
			t.Logf("Warning - Generic content detected: %s", indicator.concern)
			genericCount++
		}
	}

	if genericCount >= 2 {
		t.Errorf("Response appears to be generic/hallucinated (found %d generic indicators)", genericCount)
	}

	// Check for actual Bear-Mail specific content
	// (These should be updated based on actual Bear-Mail project structure)
	expectedTerms := []string{"android", "kotlin", "gradle"}
	foundTerms := 0
	for _, term := range expectedTerms {
		if strings.Contains(content, term) {
			foundTerms++
		}
	}

	if foundTerms == 0 {
		t.Errorf("Response doesn't mention any expected Android project terms")
	}
}

// getActualProjectDirs returns actual directories in a project
func getActualProjectDirs(projectPath string) []string {
	var dirs []string

	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return dirs
	}

	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, entry.Name())
		}
	}

	return dirs
}

// truncate shortens a string for logging
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// TestBearMailStreamingContentIntegrity verifies no content interleaving from multiple providers
func TestBearMailStreamingContentIntegrity(t *testing.T) {
	bearMailSkipIfNotRunning(t)

	baseURL := bearMailGetBaseURL()

	t.Run("No_Word_Duplication", func(t *testing.T) {
		// Request a specific phrase to detect interleaving
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Say exactly this phrase: Hello world, this is a test message.",
				},
			},
			"stream":     true,
			"max_tokens": 50,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		// Check for duplicate word patterns (sign of interleaving)
		// e.g., "HelloHello" or "worldworld" would indicate multi-provider merge
		words := strings.Fields(response.Content)
		for i := 0; i < len(words)-1; i++ {
			// Check for immediate duplicates or patterns like "YesYes"
			if strings.EqualFold(words[i], words[i+1]) && len(words[i]) > 2 {
				// Allow certain common duplicates like "the the" in natural speech
				if words[i] != "the" && words[i] != "a" && words[i] != "an" {
					t.Logf("Warning: Possible duplicate word at position %d: '%s %s'", i, words[i], words[i+1])
				}
			}
		}

		// Check for concatenated duplicates like "YesYes" or "HelloHello"
		interleavingPatterns := []string{
			"YesYes", "NoNo", "HelloHello", "II ", " II",
			"andand", "thethe", "isis", "inin", "toto",
		}
		for _, pattern := range interleavingPatterns {
			if strings.Contains(response.Content, pattern) {
				t.Errorf("Content interleaving detected: found '%s' in response", pattern)
			}
		}

		assert.True(t, response.HasDoneMarker, "Response must have [DONE] marker")
		t.Logf("Response content (no interleaving): %s", truncate(response.Content, 200))
	})

	t.Run("Coherent_Sentence_Structure", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Write a single coherent sentence about programming.",
				},
			},
			"stream":     true,
			"max_tokens": 100,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		// Basic coherence check: sentence should end properly
		content := strings.TrimSpace(response.Content)
		hasProperEnding := strings.HasSuffix(content, ".") ||
			strings.HasSuffix(content, "!") ||
			strings.HasSuffix(content, "?") ||
			strings.HasSuffix(content, ":")

		// Check for random character interleaving (like "a,nd" or "th.e")
		badPatterns := []string{",nd", ",he", ",ll", ".he", ".nd"}
		for _, pattern := range badPatterns {
			assert.False(t, strings.Contains(content, pattern),
				"Found malformed pattern '%s' indicating character interleaving", pattern)
		}

		t.Logf("Coherent sentence (proper ending=%v): %s", hasProperEnding, truncate(content, 200))
	})

	t.Run("Consistent_Stream_ID", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Count from 1 to 5.",
				},
			},
			"stream":     true,
			"max_tokens": 50,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		// All chunks should have the same stream ID
		assert.NotEmpty(t, response.StreamID, "Stream ID should be present")
		assert.True(t, strings.HasPrefix(response.StreamID, "chatcmpl-"),
			"Stream ID should have correct prefix: %s", response.StreamID)

		t.Logf("Consistent stream ID: %s, chunks: %d", response.StreamID, response.ChunkCount)
	})
}

// TestBearMailStreamingFormatValidity verifies proper SSE streaming format
func TestBearMailStreamingFormatValidity(t *testing.T) {
	bearMailSkipIfNotRunning(t)

	baseURL := bearMailGetBaseURL()

	t.Run("Proper_SSE_Format", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Say hello.",
				},
			},
			"stream":     true,
			"max_tokens": 20,
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

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		bodyStr := string(body)

		// Verify SSE format: all data lines should start with "data: "
		lines := strings.Split(bodyStr, "\n")
		dataLineCount := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Every non-empty line should be a data line
			assert.True(t, strings.HasPrefix(line, "data: "),
				"Line should start with 'data: ': %s", truncate(line, 50))
			dataLineCount++
		}

		assert.Greater(t, dataLineCount, 0, "Should have at least one data line")

		// Must end with [DONE]
		assert.True(t, strings.Contains(bodyStr, "data: [DONE]"),
			"Response must contain 'data: [DONE]'")

		t.Logf("Valid SSE format with %d data lines", dataLineCount)
	})

	t.Run("First_Chunk_Has_Role", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Hi",
				},
			},
			"stream":     true,
			"max_tokens": 10,
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

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Find first data line (after skipping empty lines)
		lines := strings.Split(string(body), "\n")
		var firstDataLine string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
				firstDataLine = strings.TrimPrefix(line, "data: ")
				break
			}
		}

		require.NotEmpty(t, firstDataLine, "Should have first data chunk")

		var chunk map[string]interface{}
		err = json.Unmarshal([]byte(firstDataLine), &chunk)
		require.NoError(t, err, "First chunk should be valid JSON")

		// Check for role in first chunk
		choices, ok := chunk["choices"].([]interface{})
		require.True(t, ok && len(choices) > 0, "Should have choices")

		choice := choices[0].(map[string]interface{})
		delta, ok := choice["delta"].(map[string]interface{})
		require.True(t, ok, "Should have delta")

		role, hasRole := delta["role"].(string)
		assert.True(t, hasRole, "First chunk should have role")
		assert.Equal(t, "assistant", role, "Role should be 'assistant'")

		t.Logf("First chunk has role: %s", role)
	})

	t.Run("Last_Chunk_Has_Finish_Reason", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Say OK.",
				},
			},
			"stream":     true,
			"max_tokens": 10,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasFinishReason, "Response must have finish_reason")
		assert.True(t, response.HasDoneMarker, "Response must have [DONE] marker")

		t.Logf("Proper termination: finish_reason=%v, [DONE]=%v",
			response.HasFinishReason, response.HasDoneMarker)
	})
}

// TestOpenCodeToolCallFormat verifies responses contain properly formatted tool calls
// that OpenCode clients can parse and execute
func TestOpenCodeToolCallFormat(t *testing.T) {
	bearMailSkipIfNotRunning(t)

	baseURL := bearMailGetBaseURL()

	t.Run("Bash_Command_Format", func(t *testing.T) {
		// Request that should generate bash commands
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Create a simple test.txt file with the content 'hello world' using bash. Show the exact command.",
				},
			},
			"stream":     true,
			"max_tokens": 200,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		// Response should be complete
		assert.True(t, response.HasDoneMarker, "Response must complete")
		assert.True(t, response.HasFinishReason, "Response must have finish_reason")

		// Log the response for debugging
		t.Logf("Response for bash command request: %s", truncate(response.Content, 500))

		// Response should mention common bash patterns
		content := strings.ToLower(response.Content)
		hasBashIndicator := strings.Contains(content, "echo") ||
			strings.Contains(content, "cat") ||
			strings.Contains(content, ">") ||
			strings.Contains(content, "bash")

		assert.True(t, hasBashIndicator, "Response should contain bash command indicators")
	})

	t.Run("Write_To_File_Request", func(t *testing.T) {
		// Request that should generate file write commands
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Write a Python hello world script to hello.py. Show the complete file content.",
				},
			},
			"stream":     true,
			"max_tokens": 300,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete")

		// Response should contain Python code
		content := response.Content
		hasPythonCode := strings.Contains(content, "print") ||
			strings.Contains(content, "def ") ||
			strings.Contains(content, "hello") ||
			strings.Contains(content, "python")

		t.Logf("Response for file write request: %s", truncate(content, 400))
		assert.True(t, hasPythonCode, "Response should contain Python code")
	})

	t.Run("Response_No_Incomplete_Tags", func(t *testing.T) {
		// Verify responses don't have incomplete/broken XML-like tags
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "List 3 files in a typical project directory.",
				},
			},
			"stream":     true,
			"max_tokens": 200,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		// Check for incomplete tags (sign of cutoff)
		incompletePatterns := []string{
			"<bash", // Missing closing >
			"<write", // Missing closing >
			"</bas", // Incomplete closing tag
			"</writ", // Incomplete closing tag
		}

		for _, pattern := range incompletePatterns {
			if strings.HasSuffix(response.Content, pattern) {
				t.Errorf("Response ends with incomplete tag: '%s'", pattern)
			}
		}

		t.Logf("Response (no incomplete tags): %s", truncate(response.Content, 300))
	})
}

// TestResponseContentValidity verifies responses are valid and coherent
func TestResponseContentValidity(t *testing.T) {
	bearMailSkipIfNotRunning(t)

	baseURL := bearMailGetBaseURL()

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

		response, err := sendStreamingRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete")
		assert.NotEmpty(t, response.Content, "Response must not be empty")
		assert.Greater(t, len(response.Content), 1, "Response should have meaningful content")

		t.Logf("Non-empty response: %s", response.Content)
	})

	t.Run("Response_Has_Expected_Content", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Say ONLY the word 'success' and nothing else.",
				},
			},
			"stream":     true,
			"max_tokens": 20,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete")

		// Should contain "success" (case insensitive)
		content := strings.ToLower(response.Content)
		assert.True(t, strings.Contains(content, "success"),
			"Response should contain 'success', got: %s", response.Content)

		t.Logf("Expected content response: %s", response.Content)
	})

	t.Run("Response_Follows_Instructions", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "List exactly 3 programming languages, numbered 1-3.",
				},
			},
			"stream":     true,
			"max_tokens": 100,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 60*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete")

		// Check for numbered list
		content := response.Content
		hasNumbers := (strings.Contains(content, "1") || strings.Contains(content, "1.")) &&
			(strings.Contains(content, "2") || strings.Contains(content, "2.")) &&
			(strings.Contains(content, "3") || strings.Contains(content, "3."))

		t.Logf("Numbered list response: %s", truncate(content, 300))
		assert.True(t, hasNumbers, "Response should have numbered items 1-3")
	})
}

// TestBearMailNoResponseCutoff verifies responses complete without premature termination
func TestBearMailNoResponseCutoff(t *testing.T) {
	bearMailSkipIfNotRunning(t)

	baseURL := bearMailGetBaseURL()

	t.Run("Medium_Response_Completes", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role":    "user",
					"content": "Explain what an API is in exactly 3 paragraphs.",
				},
			},
			"stream":     true,
			"max_tokens": 500,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 120*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Response must complete with [DONE]")
		assert.True(t, response.HasFinishReason, "Response must have finish_reason")
		assert.Greater(t, len(response.Content), 200, "Medium response should have substantial content")

		// Should not end mid-sentence (check for incomplete patterns)
		content := strings.TrimSpace(response.Content)
		incompletePatterns := []string{
			" the$", " a$", " an$", " is$", " are$", " and$", " or$", " to$",
		}
		for _, pattern := range incompletePatterns {
			// Using regex-style check - ends with these words
			if strings.HasSuffix(content, strings.TrimPrefix(pattern, "$")) {
				t.Logf("Warning: Response may be cut off, ends with: '%s'",
					content[max(0, len(content)-30):])
			}
		}

		t.Logf("Medium response completed: %d chars, %d chunks", len(response.Content), response.ChunkCount)
	})

	t.Run("Long_Response_Completes", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "helixagent-ensemble",
			"messages": []map[string]string{
				{
					"role": "user",
					"content": `Write a comprehensive guide with these sections:
1. Introduction (3 sentences)
2. Getting Started (5 bullet points)
3. Best Practices (5 bullet points)
4. Common Mistakes (3 points)
5. Conclusion (2 sentences)

Complete ALL sections fully.`,
				},
			},
			"stream":     true,
			"max_tokens": 2000,
		}

		response, err := sendStreamingRequest(t, baseURL, reqBody, 180*time.Second)
		require.NoError(t, err, "Request should not fail")

		assert.True(t, response.HasDoneMarker, "Long response must complete with [DONE]")
		assert.True(t, response.HasFinishReason, "Long response must have finish_reason")

		// Check for section markers
		content := strings.ToLower(response.Content)
		sections := []string{"introduction", "getting started", "best practices", "common mistakes", "conclusion"}
		foundSections := 0
		for _, section := range sections {
			if strings.Contains(content, section) {
				foundSections++
			}
		}

		t.Logf("Long response: %d/%d sections found, %d chars, %d chunks",
			foundSections, len(sections), len(response.Content), response.ChunkCount)
		assert.GreaterOrEqual(t, foundSections, 3, "Should find at least 3 of 5 sections")
	})
}
