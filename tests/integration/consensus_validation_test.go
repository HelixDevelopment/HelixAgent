package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===============================================================================================
// CRITICAL CONSENSUS VALIDATION TESTS
// These tests MUST FAIL if the debate ensemble consensus section is empty
// ===============================================================================================

// consensusHelixAgentURL is the HelixAgent server URL for consensus tests
const consensusHelixAgentURL = "http://localhost:8100"

// consensusServerAvailable checks if HelixAgent server is available and responding properly
func consensusServerAvailable(t *testing.T) bool {
	t.Helper()
	if os.Getenv("HELIXAGENT_INTEGRATION_TESTS") != "1" {
		t.Logf("HELIXAGENT_INTEGRATION_TESTS not set - skipping integration test (acceptable)")
		return false
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(consensusHelixAgentURL + "/health")
	if err != nil {
		t.Logf("HelixAgent server not available at %s (acceptable)", consensusHelixAgentURL)
		return false
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Logf("HelixAgent server not healthy at %s (acceptable)", consensusHelixAgentURL)
		return false
	}
	return true
}

// TestConsensusNotEmpty_EndToEnd tests that the consensus section in the debate ensemble
// is NOT empty when making a real API request
// THIS TEST WILL FAIL if the consensus generation is broken
func TestConsensusNotEmpty_EndToEnd(t *testing.T) {
	testutil.RequireServer(t)

	var client *http.Client
	var resp *http.Response

	// Create a test request
	requestBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "What is 2+2?",
			},
		},
		"stream": true,
	}

	jsonBody, err := json.Marshal(requestBody)
	require.NoError(t, err)

	// Make the request with a timeout
	client = &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("POST", consensusHelixAgentURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Read the full response
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	fullResponse := string(body)

	// CRITICAL ASSERTIONS: These MUST pass for the debate ensemble to be working correctly
	// Note: API clients get Markdown format, terminals get ANSI format

	// 1. Must have the debate ensemble header (ANSI or Markdown)
	hasANSIHeader := strings.Contains(fullResponse, "HELIXAGENT AI DEBATE ENSEMBLE")
	hasMarkdownHeader := strings.Contains(fullResponse, "# HelixAgent AI Debate Ensemble")
	assert.True(t, hasANSIHeader || hasMarkdownHeader,
		"Response must contain debate ensemble header (ANSI or Markdown)")

	// 2. Must have the CONSENSUS section (ANSI or Markdown)
	hasANSIConsensus := strings.Contains(fullResponse, "CONSENSUS REACHED")
	hasMarkdownConsensus := strings.Contains(fullResponse, "## Consensus") || strings.Contains(fullResponse, "## Final Answer")
	assert.True(t, hasANSIConsensus || hasMarkdownConsensus,
		"Response must contain CONSENSUS section (ANSI or Markdown)")

	// 3. Must have the footer
	assert.Contains(t, fullResponse, "Powered by HelixAgent AI Debate Ensemble",
		"Response must contain footer")

	// 4. CRITICAL: There must be CONTENT between CONSENSUS and footer
	// This is the main test that validates the consensus is not empty
	// Look for both ANSI and Markdown formats
	consensusIndex := strings.Index(fullResponse, "CONSENSUS REACHED")
	if consensusIndex < 0 {
		consensusIndex = strings.Index(fullResponse, "## Consensus")
	}
	if consensusIndex < 0 {
		consensusIndex = strings.Index(fullResponse, "## Final Answer")
	}
	footerIndex := strings.Index(fullResponse, "Powered by HelixAgent AI Debate Ensemble")

	if consensusIndex >= 0 && footerIndex >= 0 && footerIndex > consensusIndex {
		consensusSection := fullResponse[consensusIndex:footerIndex]

		// The consensus section must have substantial content (not just whitespace/formatting)
		// Remove the header lines and check for actual content
		lines := strings.Split(consensusSection, "\n")
		contentLines := 0
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Skip empty lines and formatting lines (═, ─, #, etc.)
			if len(trimmed) > 5 && !strings.HasPrefix(trimmed, "═") && !strings.HasPrefix(trimmed, "─") &&
				!strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "---") &&
				!strings.Contains(trimmed, "CONSENSUS") && !strings.Contains(trimmed, "synthesized") {
				contentLines++
			}
		}

		assert.Greater(t, contentLines, 0,
			"CONSENSUS section must have actual content, not just headers. Got %d content lines in: %s",
			contentLines, consensusSection)
	} else {
		t.Fatalf("Could not find CONSENSUS section or footer in response. consensusIndex=%d, footerIndex=%d", consensusIndex, footerIndex)
	}

	// 5. Must have all 5 debate positions (ANSI or Markdown format)
	// ANSI: "THE ANALYST", Markdown: "**[Analyst]**" or "**Analyst**"
	positionPairs := []struct {
		ansi     string
		markdown string
	}{
		{"THE ANALYST", "Analyst"},
		{"THE PROPOSER", "Proposer"},
		{"THE CRITIC", "Critic"},
		{"THE SYNTHESIZER", "Synthesis"},
		{"THE MEDIATOR", "Mediator"},
	}
	for _, pair := range positionPairs {
		hasANSI := strings.Contains(fullResponse, pair.ansi)
		hasMarkdown := strings.Contains(fullResponse, pair.markdown)
		assert.True(t, hasANSI || hasMarkdown,
			"Response must contain position: %s (ANSI) or %s (Markdown)", pair.ansi, pair.markdown)
	}

	// 6. Each position must have a response (not "Unable to provide analysis")
	// For both formats, check that there's no error message in the position sections
	assert.NotContains(t, fullResponse, "Unable to provide analysis",
		"Response should not contain error fallback messages")
}

// TestConsensusHasSubstantiveContent validates that the consensus is not just filler text
func TestConsensusHasSubstantiveContent(t *testing.T) {
	testutil.RequireServer(t)

	var client *http.Client
	var resp *http.Response
	var err error

	// Create a test request with a specific question
	requestBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "What are the benefits of using Go for backend development?",
			},
		},
		"stream": true,
	}

	jsonBody, err := json.Marshal(requestBody)
	require.NoError(t, err)

	client = &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("POST", consensusHelixAgentURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	fullResponse := string(body)

	// Extract the consensus section content (ANSI or Markdown format)
	consensusIndex := strings.Index(fullResponse, "CONSENSUS REACHED")
	if consensusIndex < 0 {
		consensusIndex = strings.Index(fullResponse, "## Consensus")
	}
	if consensusIndex < 0 {
		consensusIndex = strings.Index(fullResponse, "## Final Answer")
	}
	footerIndex := strings.Index(fullResponse, "Powered by HelixAgent AI Debate Ensemble")

	require.True(t, consensusIndex >= 0, "Must have CONSENSUS section (ANSI or Markdown)")
	require.True(t, footerIndex > consensusIndex, "Footer must come after CONSENSUS")

	consensusSection := fullResponse[consensusIndex:footerIndex]

	// The consensus must reference the actual topic
	// For a question about Go, the consensus should mention relevant concepts
	relevantTerms := []string{"Go", "backend", "development", "concurrency", "performance", "goroutines", "compile", "type", "simple"}
	foundRelevant := 0
	for _, term := range relevantTerms {
		if strings.Contains(strings.ToLower(consensusSection), strings.ToLower(term)) {
			foundRelevant++
		}
	}

	assert.Greater(t, foundRelevant, 2,
		"Consensus should reference at least 3 relevant terms from the question. Found %d. Consensus: %s",
		foundRelevant, consensusSection)

	// Consensus should not be a generic fallback message
	genericMessages := []string{
		"could not be reached",
		"consensus was not achieved",
		"Unable to provide",
		"error occurred",
	}
	for _, msg := range genericMessages {
		assert.NotContains(t, consensusSection, msg,
			"Consensus should not contain generic error/fallback message: %s", msg)
	}
}

// TestAllDebatePositionsHaveRealResponses validates each position has actual LLM responses
func TestAllDebatePositionsHaveRealResponses(t *testing.T) {
	testutil.RequireServer(t)

	var client *http.Client
	var resp *http.Response
	var err error

	requestBody := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "Explain the concept of polymorphism in object-oriented programming.",
			},
		},
		"stream": true,
	}

	jsonBody, err := json.Marshal(requestBody)
	require.NoError(t, err)

	client = &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("POST", consensusHelixAgentURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	fullResponse := string(body)

	// All 5 positions must be present (ANSI or Markdown format)
	positionNames := []string{"Analyst", "Proposer", "Critic", "Synthesis", "Mediator"}
	for _, position := range positionNames {
		hasANSI := strings.Contains(fullResponse, "THE "+strings.ToUpper(position))
		hasMarkdown := strings.Contains(fullResponse, position)
		assert.True(t, hasANSI || hasMarkdown,
			"Response must contain position: %s (ANSI or Markdown)", position)
	}

	// The response must NOT contain the error fallback message
	assert.NotContains(t, fullResponse, "Unable to provide analysis",
		"Response should not contain error fallback messages")

	// The response must contain the query term (polymorphism) somewhere in the debate
	assert.Contains(t, strings.ToLower(fullResponse), "polymorphism",
		"Response should reference the query term 'polymorphism'")

	// The consensus section must exist and have content (ANSI or Markdown)
	hasANSIConsensus := strings.Contains(fullResponse, "CONSENSUS REACHED")
	hasMarkdownConsensus := strings.Contains(fullResponse, "## Consensus") || strings.Contains(fullResponse, "## Final Answer")
	assert.True(t, hasANSIConsensus || hasMarkdownConsensus,
		"Response must have CONSENSUS section (ANSI or Markdown)")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
