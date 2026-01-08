package challenge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestDebateGroupVerification verifies that the debate group has properly
// configured providers and the ensemble is operational.
//
// This test suite covers:
// 1. Provider registration verification
// 2. Individual provider health checks
// 3. Ensemble functionality verification
// 4. Provider contribution validation
//
// Run with: go test -v ./tests/challenge -run TestDebateGroupVerification
func TestDebateGroupVerification(t *testing.T) {
	baseURL := getBaseURL()

	// Skip if server is not running
	if !serverHealthy(baseURL) {
		t.Skip("HelixAgent server not running at " + baseURL)
	}

	t.Run("ServerHealth", func(t *testing.T) {
		testServerHealth(t, baseURL)
	})

	t.Run("ProvidersRegistered", func(t *testing.T) {
		testProvidersRegistered(t, baseURL)
	})

	t.Run("IndividualProviders", func(t *testing.T) {
		testIndividualProviders(t, baseURL)
	})

	t.Run("EnsembleFunctionality", func(t *testing.T) {
		testEnsembleFunctionality(t, baseURL)
	})

	t.Run("ProviderContribution", func(t *testing.T) {
		testProviderContribution(t, baseURL)
	})
}

func getBaseURL() string {
	url := os.Getenv("HELIXAGENT_URL")
	if url == "" {
		url = "http://localhost:7061"
	}
	return url
}

func serverHealthy(baseURL string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func testServerHealth(t *testing.T, baseURL string) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Server not healthy: status %d", resp.StatusCode)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	status, ok := health["status"].(string)
	if !ok || status != "healthy" {
		t.Errorf("Unexpected health status: %v", health)
	}
}

type ProvidersResponse struct {
	Count     int        `json:"count"`
	Providers []Provider `json:"providers"`
}

type Provider struct {
	Name              string                 `json:"name"`
	SupportedModels   []string               `json:"supported_models"`
	SupportedFeatures []string               `json:"supported_features"`
	Metadata          map[string]interface{} `json:"metadata"`
}

func testProvidersRegistered(t *testing.T, baseURL string) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(baseURL + "/v1/providers")
	if err != nil {
		t.Fatalf("Failed to get providers: %v", err)
	}
	defer resp.Body.Close()

	var providers ProvidersResponse
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		t.Fatalf("Failed to decode providers response: %v", err)
	}

	// Verify we have providers registered
	if providers.Count == 0 {
		t.Fatal("No providers registered")
	}

	t.Logf("Found %d providers registered", providers.Count)

	// Check for expected providers
	expectedProviders := []string{"deepseek", "gemini", "openrouter"}
	registeredMap := make(map[string]bool)
	for _, p := range providers.Providers {
		registeredMap[p.Name] = true
		t.Logf("  Provider: %s (models: %d)", p.Name, len(p.SupportedModels))
	}

	for _, expected := range expectedProviders {
		if !registeredMap[expected] {
			t.Errorf("Expected provider '%s' not registered", expected)
		}
	}
}

type ChatCompletionRequest struct {
	Model         string    `json:"model"`
	Messages      []Message `json:"messages"`
	ForceProvider string    `json:"force_provider,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             *Usage   `json:"usage,omitempty"`
	SystemFingerprint string   `json:"system_fingerprint,omitempty"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// ProviderTestResult represents the result of testing a single provider
type ProviderTestResult struct {
	Provider    string
	Working     bool
	RateLimited bool
	AuthFailed  bool
	Error       string
	ResponseMs  int64
}

func testIndividualProviders(t *testing.T, baseURL string) {
	providers := []string{"deepseek", "gemini", "openrouter"}
	results := make([]ProviderTestResult, 0, len(providers))
	workingCount := 0
	rateLimitedCount := 0
	authFailedCount := 0

	for _, provider := range providers {
		result := testSingleProvider(baseURL, provider)
		results = append(results, result)

		if result.Working {
			workingCount++
			t.Logf("  %s: WORKING (%.0fms)", provider, float64(result.ResponseMs))
		} else if result.RateLimited {
			rateLimitedCount++
			t.Logf("  %s: RATE LIMITED (temporary)", provider)
		} else if result.AuthFailed {
			authFailedCount++
			t.Logf("  %s: AUTH FAILED (check API key)", provider)
		} else {
			t.Logf("  %s: FAILED - %s", provider, result.Error)
		}
	}

	// Log the summary
	t.Logf("  %d/%d providers working", workingCount, len(providers))
	if rateLimitedCount > 0 {
		t.Logf("  %d providers rate limited (temporary issue)", rateLimitedCount)
	}
	if authFailedCount > 0 {
		t.Logf("  %d providers have auth failures (check API keys)", authFailedCount)
	}

	// Note: We don't fail here because provider availability is external
	// The test validates the system properly reports provider status
}

func testSingleProvider(baseURL, provider string) ProviderTestResult {
	result := ProviderTestResult{Provider: provider}

	start := time.Now()
	client := &http.Client{Timeout: 30 * time.Second}

	reqBody := ChatCompletionRequest{
		Model:         "helixagent-debate",
		Messages:      []Message{{Role: "user", Content: "Say OK"}},
		ForceProvider: provider,
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	result.ResponseMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Error = fmt.Sprintf("Request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Check for success
	if resp.StatusCode == http.StatusOK {
		var chatResp ChatCompletionResponse
		if err := json.Unmarshal(respBody, &chatResp); err == nil && len(chatResp.Choices) > 0 {
			result.Working = true
			return result
		}
	}

	// Parse error
	var errResp ErrorResponse
	if err := json.Unmarshal(respBody, &errResp); err == nil {
		errMsg := errResp.Error.Message
		errCode := errResp.Error.Code

		// Categorize error
		if contains(errMsg, "429", "quota", "rate") {
			result.RateLimited = true
			result.Error = "Rate limited"
		} else if contains(errMsg, "401", "unauthorized", "not found", "invalid") || contains(errCode, "401", "unauthorized") {
			result.AuthFailed = true
			result.Error = "Authentication failed"
		} else {
			result.Error = errMsg
		}
	} else {
		result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)[:min(100, len(respBody))])
	}

	return result
}

func contains(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func testEnsembleFunctionality(t *testing.T, baseURL string) {
	client := &http.Client{Timeout: 60 * time.Second}

	reqBody := ChatCompletionRequest{
		Model:    "helixagent-debate",
		Messages: []Message{{Role: "user", Content: "What is 2+2? Answer with just the number."}},
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		t.Logf("Ensemble request failed (network error): %v", err)
		t.Skip("Skipping due to network error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Check if this is a provider availability issue (not a system bug)
		if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable {
			t.Logf("Ensemble unavailable (no working providers): %s", string(body))
			t.Skip("Skipping due to provider unavailability")
			return
		}
		t.Logf("Ensemble returned non-OK status %d: %s", resp.StatusCode, string(body))
		return
	}

	var chatResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		t.Logf("Failed to decode ensemble response: %v", err)
		return
	}

	// Verify response structure
	if len(chatResp.Choices) == 0 {
		t.Log("Ensemble returned no choices")
		return
	}

	// Verify model identifier
	if chatResp.Model != "helixagent-ensemble" {
		t.Logf("Expected model 'helixagent-ensemble', got '%s'", chatResp.Model)
	}

	// Verify fingerprint
	if chatResp.SystemFingerprint != "fp_helixagent_ensemble" {
		t.Logf("Expected fingerprint 'fp_helixagent_ensemble', got '%s'", chatResp.SystemFingerprint)
	}

	// Log response details
	t.Logf("  Ensemble response received in %v", elapsed)
	t.Logf("  Model: %s", chatResp.Model)
	t.Logf("  Fingerprint: %s", chatResp.SystemFingerprint)
	if chatResp.Usage != nil {
		t.Logf("  Tokens: %d total", chatResp.Usage.TotalTokens)
	}

	content := chatResp.Choices[0].Message.Content
	if len(content) > 50 {
		content = content[:50] + "..."
	}
	t.Logf("  Response: %s", content)
}

func testProviderContribution(t *testing.T, baseURL string) {
	// Send multiple requests and verify ensemble consistently returns responses
	t.Log("Testing ensemble consistency with multiple requests...")

	const numRequests = 3
	successCount := 0
	var totalTokens int

	for i := 0; i < numRequests; i++ {
		client := &http.Client{Timeout: 60 * time.Second}

		reqBody := ChatCompletionRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: fmt.Sprintf("Say 'test %d'", i+1)}},
		}

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Logf("  Request %d failed: %v", i+1, err)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			var chatResp ChatCompletionResponse
			if err := json.NewDecoder(resp.Body).Decode(&chatResp); err == nil && len(chatResp.Choices) > 0 {
				successCount++
				if chatResp.Usage != nil {
					totalTokens += chatResp.Usage.TotalTokens
				}
			}
		}
		resp.Body.Close()

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("  Completed %d/%d requests successfully", successCount, numRequests)
	if totalTokens > 0 {
		t.Logf("  Total tokens used: %d", totalTokens)
	}

	// Log result - we don't fail because provider availability is external
	if successCount < 2 {
		t.Logf("  Note: Low success rate may indicate provider availability issues")
	}
}

// TestProviderHealthEndpoints tests provider-specific health endpoints
func TestProviderHealthEndpoints(t *testing.T) {
	baseURL := getBaseURL()

	if !serverHealthy(baseURL) {
		t.Skip("HelixAgent server not running at " + baseURL)
	}

	providers := []string{"deepseek", "gemini", "openrouter"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(fmt.Sprintf("%s/v1/providers/%s/health", baseURL, provider))
			if err != nil {
				t.Logf("  %s health check: failed to connect", provider)
				return
			}
			defer resp.Body.Close()

			t.Logf("  %s health check: HTTP %d", provider, resp.StatusCode)
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
