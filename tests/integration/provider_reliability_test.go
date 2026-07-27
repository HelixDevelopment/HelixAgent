package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
)

// checkProvidersAvailable checks if any LLM providers are available
func checkProvidersAvailable(helixAgentURL string) (bool, string) {
	resp, err := makeCompletionRequest(helixAgentURL, "Say OK")
	if err != nil {
		return false, "connection error"
	}
	if resp.StatusCode == 502 && strings.Contains(resp.RawBody, "ALL_PROVIDERS_FAILED") {
		return false, "all providers failed"
	}
	return true, ""
}

// TestProviderReliability_ConsecutiveRequests validates that the API can handle
// consecutive requests without returning empty responses. This test was created
// after an incident where the OpenCode challenge failed because 23/25 tests
// returned empty responses after the first 2 requests succeeded.
//
// Root cause: Transient provider failures caused the circuit breaker to open,
// resulting in empty responses for subsequent requests.
//
// IMPORTANT: This test distinguishes between:
// - Provider unavailability (502 "ALL_PROVIDERS_FAILED") - infrastructure issue, test skips
// - Empty responses with 200 status - API bug, test fails
// - Proper responses - test passes
func TestProviderReliability_ConsecutiveRequests(t *testing.T) {
	testutil.RequireServer(t)
	helixAgentURL := getHelixAgentURL()

	t.Log("Testing consecutive API requests to ensure no empty responses...")

	// First, check if any providers are available
	initialResp, err := makeCompletionRequest(helixAgentURL, "Say OK")
	if err != nil {
		t.Skipf("Cannot reach HelixAgent API: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
	}
	if initialResp.StatusCode == 502 {
		// Check if it's "ALL_PROVIDERS_FAILED"
		if strings.Contains(initialResp.RawBody, "ALL_PROVIDERS_FAILED") {
			t.Skip("All LLM providers are unavailable - this is an infrastructure issue, not an API bug. Skipping reliability test.") // SKIP-OK: #legacy-untriaged
		}
	}

	// Use fewer requests in short mode to avoid test timeout
	numRequests := 10
	if testing.Short() {
		numRequests = 3
	}
	successCount := 0
	emptyResponseCount := 0
	providerUnavailableCount := 0
	errorCount := 0
	var failedResponses []string

	for i := 1; i <= numRequests; i++ {
		startTime := time.Now()
		prompt := fmt.Sprintf("What is %d + %d?", i, i)

		resp, err := makeCompletionRequest(helixAgentURL, prompt)
		duration := time.Since(startTime)

		if err != nil {
			errorCount++
			failedResponses = append(failedResponses, fmt.Sprintf("Request %d: error - %v", i, err))
			t.Logf("Request %d: ERROR - %v (%.2fs)", i, err, duration.Seconds())
			continue
		}

		// Check for provider unavailability (502)
		if resp.StatusCode == 502 {
			providerUnavailableCount++
			t.Logf("Request %d: PROVIDER_UNAVAILABLE, HTTP %d (%.2fs)", i, resp.StatusCode, duration.Seconds())
			continue
		}

		if resp.Content == "" && resp.StatusCode == 200 {
			// Empty response with 200 status is an API bug
			emptyResponseCount++
			failedResponses = append(failedResponses, fmt.Sprintf("Request %d: empty response (HTTP %d)", i, resp.StatusCode))
			t.Logf("Request %d: EMPTY response with HTTP 200 (%.2fs) - THIS IS A BUG", i, duration.Seconds())
		} else if resp.Content != "" {
			successCount++
			t.Logf("Request %d: SUCCESS, HTTP %d (%.2fs) - Response: %s", i, resp.StatusCode, duration.Seconds(), truncateProviderReliability(resp.Content, 50))
		}

		// Small delay to avoid overwhelming the server
		time.Sleep(500 * time.Millisecond)
	}

	t.Logf("Final Results: %d/%d successful, %d empty, %d provider unavailable, %d errors",
		successCount, numRequests, emptyResponseCount, providerUnavailableCount, errorCount)

	// If ALL requests got provider unavailability, skip the test
	if providerUnavailableCount == numRequests {
		t.Skip("All requests got provider unavailability (502) - this is an infrastructure issue, not an API bug") // SKIP-OK: #legacy-untriaged
	}

	// Fail if we get empty responses with 200 status - this should never happen
	if emptyResponseCount > 0 {
		t.Errorf("CRITICAL: Got %d empty responses with HTTP 200! This indicates an API bug.", emptyResponseCount)
		for _, failure := range failedResponses {
			t.Log("  - " + failure)
		}
	}

	// If we have some successful responses, require at least 80% success rate
	if successCount > 0 {
		successRate := float64(successCount) / float64(numRequests-providerUnavailableCount) * 100
		t.Logf("Success rate (excluding provider unavailability): %.1f%%", successRate)

		if successRate < 80 {
			t.Errorf("Success rate %.1f%% is below required 80%% threshold", successRate)
		}
	}
}

// TestProviderReliability_RapidRequests validates behavior under rapid request load
func TestProviderReliability_RapidRequests(t *testing.T) {
	testutil.RequireServer(t)
	helixAgentURL := getHelixAgentURL()

	// Check if providers are available first
	available, reason := checkProvidersAvailable(helixAgentURL)
	if !available {
		t.Skipf("LLM providers unavailable (%s) - this is an infrastructure issue, not an API bug (SKIP-OK: #unmarked-skip-needs-ticket)", reason)
	}

	t.Log("Testing rapid API requests...")

	// Send 5 requests in quick succession (no delay)
	numRequests := 5
	var wg sync.WaitGroup
	results := make(chan RequestResult, numRequests)

	for i := 1; i <= numRequests; i++ {
		wg.Add(1)
		go func(reqNum int) {
			defer wg.Done()
			prompt := fmt.Sprintf("What is the square root of %d?", reqNum*reqNum)
			startTime := time.Now()
			resp, err := makeCompletionRequest(helixAgentURL, prompt)
			duration := time.Since(startTime)

			results <- RequestResult{
				RequestNum: reqNum,
				Success:    err == nil && resp.Content != "" && resp.StatusCode == 200,
				StatusCode: resp.StatusCode,
				Duration:   duration,
				Error:      err,
				Content:    resp.Content,
				RawBody:    resp.RawBody,
			}
		}(i)
	}

	wg.Wait()
	close(results)

	successCount := 0
	emptyCount := 0
	providerUnavailableCount := 0
	timeoutCount := 0
	for result := range results {
		// Check for provider unavailability (502)
		if result.StatusCode == 502 && strings.Contains(result.RawBody, "ALL_PROVIDERS_FAILED") {
			providerUnavailableCount++
			t.Logf("Request %d: PROVIDER_UNAVAILABLE (%.2fs)", result.RequestNum, result.Duration.Seconds())
			continue
		}
		// Check for timeout errors (context deadline exceeded)
		if result.Error != nil && strings.Contains(result.Error.Error(), "deadline exceeded") {
			timeoutCount++
			t.Logf("Request %d: TIMEOUT (%.2fs)", result.RequestNum, result.Duration.Seconds())
			continue
		}
		if result.Success {
			successCount++
			t.Logf("Request %d: SUCCESS (%.2fs)", result.RequestNum, result.Duration.Seconds())
		} else if result.Content == "" && result.StatusCode == 200 {
			emptyCount++
			t.Logf("Request %d: EMPTY response with HTTP 200 - THIS IS A BUG", result.RequestNum)
		} else {
			t.Logf("Request %d: ERROR - %v (HTTP %d)", result.RequestNum, result.Error, result.StatusCode)
		}
	}

	t.Logf("Results: %d successful, %d empty (200), %d provider unavailable, %d timeout",
		successCount, emptyCount, providerUnavailableCount, timeoutCount)

	// If all requests got provider unavailability or timeouts, skip the test
	if providerUnavailableCount+timeoutCount == numRequests {
		t.Skip("All rapid requests failed due to provider unavailability or timeouts - this is an infrastructure issue, not an API bug") // SKIP-OK: #legacy-untriaged
	}

	// At least 4 out of 5 rapid requests should succeed (excluding provider unavailable and timeouts)
	availableRequests := numRequests - providerUnavailableCount - timeoutCount
	if availableRequests > 0 && successCount < availableRequests-1 {
		t.Errorf("Only %d/%d rapid requests succeeded (expected at least %d)", successCount, availableRequests, availableRequests-1)
	}

	// Empty responses with HTTP 200 are API bugs and should never happen
	if emptyCount > 0 {
		t.Errorf("Got %d empty responses with HTTP 200 - this is an API bug", emptyCount)
	}
}

// TestProviderReliability_CircuitBreakerRecovery validates that the system recovers
// after provider failures
func TestProviderReliability_CircuitBreakerRecovery(t *testing.T) {
	testutil.RequireServer(t)
	helixAgentURL := getHelixAgentURL()

	// Check if providers are available first
	available, reason := checkProvidersAvailable(helixAgentURL)
	if !available {
		t.Skipf("LLM providers unavailable (%s) - this is an infrastructure issue, not an API bug (SKIP-OK: #unmarked-skip-needs-ticket)", reason)
	}

	t.Log("Testing circuit breaker recovery behavior...")

	// Make initial request
	resp1, err := makeCompletionRequest(helixAgentURL, "What is 1+1?")
	if err != nil {
		t.Fatalf("Initial request failed: %v", err)
	}

	// Check if provider became unavailable
	if resp1.StatusCode == 502 && strings.Contains(resp1.RawBody, "ALL_PROVIDERS_FAILED") {
		t.Skip("LLM providers became unavailable during test - infrastructure issue") // SKIP-OK: #legacy-untriaged
	}

	t.Logf("Initial request: HTTP %d, Content length: %d", resp1.StatusCode, len(resp1.Content))

	// Wait a moment (simulate time passing)
	time.Sleep(2 * time.Second)

	// Make another request - should succeed even if circuit breaker was triggered
	resp2, err := makeCompletionRequest(helixAgentURL, "What is 2+2?")
	if err != nil {
		t.Errorf("Recovery request failed: %v", err)
		return
	}

	// Check if provider became unavailable
	if resp2.StatusCode == 502 && strings.Contains(resp2.RawBody, "ALL_PROVIDERS_FAILED") {
		t.Skip("LLM providers became unavailable during test - infrastructure issue") // SKIP-OK: #legacy-untriaged
	}

	if resp2.Content == "" && resp2.StatusCode == 200 {
		t.Error("Recovery request returned empty response with HTTP 200 - this is an API bug")
	} else if resp2.Content != "" {
		t.Logf("Recovery request: HTTP %d, Content length: %d - SUCCESS", resp2.StatusCode, len(resp2.Content))
	}
}

// TestAPIResponse_NonEmpty validates that API responses are never empty when the server is healthy
func TestAPIResponse_NonEmpty(t *testing.T) {
	testutil.RequireServer(t)
	helixAgentURL := getHelixAgentURL()

	// Check if providers are available first
	available, reason := checkProvidersAvailable(helixAgentURL)
	if !available {
		t.Skipf("LLM providers unavailable (%s) - this is an infrastructure issue, not an API bug (SKIP-OK: #unmarked-skip-needs-ticket)", reason)
	}

	testCases := []struct {
		name   string
		prompt string
	}{
		{"simple_math", "What is 2+2?"},
		{"factual", "What is the capital of France?"},
		{"code_generation", "Write a hello world function in Go"},
		{"explanation", "Explain what a REST API is"},
		{"knowledge", "What is Go used for?"},
	}

	providerUnavailableCount := 0
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := makeCompletionRequest(helixAgentURL, tc.prompt)

			if err != nil {
				// Allow 502/503/504 as transient provider errors (not API failures)
				if resp.StatusCode >= 502 && resp.StatusCode <= 504 {
					if strings.Contains(resp.RawBody, "ALL_PROVIDERS_FAILED") {
						providerUnavailableCount++
						t.Logf("Provider unavailable (HTTP %d) - infrastructure issue", resp.StatusCode)
					} else {
						t.Logf("Transient provider error (HTTP %d) - acceptable", resp.StatusCode)
					}
					return
				}
				t.Errorf("Request failed: %v", err)
				return
			}

			if resp.Content == "" && resp.StatusCode == 200 {
				t.Errorf("CRITICAL: Got empty response for prompt '%s' with HTTP 200 - this is an API bug!",
					tc.prompt)
			} else if resp.Content != "" {
				t.Logf("Response OK (%d chars)", len(resp.Content))
			}
		})
	}

	// If all tests got provider unavailability, skip the parent test
	if providerUnavailableCount == len(testCases) {
		t.Skip("All requests got provider unavailability - infrastructure issue") // SKIP-OK: #legacy-untriaged
	}
}

// TestAPIResponse_ResponseTime validates that response times are reasonable
func TestAPIResponse_ResponseTime(t *testing.T) {
	testutil.RequireServer(t)
	helixAgentURL := getHelixAgentURL()

	// Check if providers are available first
	available, reason := checkProvidersAvailable(helixAgentURL)
	if !available {
		t.Skipf("LLM providers unavailable (%s) - this is an infrastructure issue, not an API bug (SKIP-OK: #unmarked-skip-needs-ticket)", reason)
	}

	t.Log("Testing API response times...")

	startTime := time.Now()
	resp, err := makeCompletionRequest(helixAgentURL, "What is 5+5?")
	duration := time.Since(startTime)

	if err != nil {
		t.Errorf("Request failed: %v", err)
		return
	}

	// Check if provider became unavailable
	if resp.StatusCode == 502 && strings.Contains(resp.RawBody, "ALL_PROVIDERS_FAILED") {
		t.Skip("LLM providers became unavailable during test - infrastructure issue") // SKIP-OK: #legacy-untriaged
	}

	// Response time should be under 60 seconds (generous for LLM)
	if duration > 60*time.Second {
		t.Errorf("Response time %.2fs exceeds 60 second threshold", duration.Seconds())
	}

	// Response time under 100ms with empty response and HTTP 200 indicates an API bug
	if duration < 100*time.Millisecond && resp.Content == "" && resp.StatusCode == 200 {
		t.Error("Very fast empty response (< 100ms) with HTTP 200 indicates an API bug")
	}

	t.Logf("Response time: %.2fs, content length: %d, HTTP status: %d", duration.Seconds(), len(resp.Content), resp.StatusCode)
}

// Helper types and functions

type CompletionResponse struct {
	StatusCode int
	Content    string
	RawBody    string
}

type RequestResult struct {
	RequestNum int
	Success    bool
	StatusCode int
	Duration   time.Duration
	Error      error
	Content    string
	RawBody    string
}

func getHelixAgentURL() string {
	// Default to localhost:8100
	return "http://localhost:8100"
}

func isHelixAgentRunning(baseURL string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func makeCompletionRequest(baseURL, prompt string) (CompletionResponse, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	requestBody := fmt.Sprintf(`{
		"model": "helixagent-debate",
		"messages": [{"role": "user", "content": %q}],
		"max_tokens": 100
	}`, prompt)

	req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions",
		strings.NewReader(requestBody))
	if err != nil {
		return CompletionResponse{}, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return CompletionResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CompletionResponse{StatusCode: resp.StatusCode}, err
	}

	// Parse JSON response to extract content
	var result CompletionResponse
	result.StatusCode = resp.StatusCode
	result.RawBody = string(body)

	var jsonResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &jsonResp); err == nil {
		if len(jsonResp.Choices) > 0 {
			result.Content = jsonResp.Choices[0].Message.Content
		}
	}

	return result, nil
}

func truncateProviderReliability(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
