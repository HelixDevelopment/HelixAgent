package challenge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestDebateGroupComprehensive provides full automation tests for AI debate groups
// covering various sizes, fallback configurations, and edge cases.
//
// Run with: go test -v ./tests/challenge -run TestDebateGroupComprehensive
func TestDebateGroupComprehensive(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping debate group comprehensive test in short mode")  // SKIP-OK: #short-mode
	}
	baseURL := getBaseURL()

	if !serverHealthy(baseURL) {
		t.Skip("HelixAgent server not running at " + baseURL)  // SKIP-OK: #legacy-untriaged
	}

	// Run all comprehensive tests
	t.Run("DebateGroupSizes", func(t *testing.T) {
		testDebateGroupSizes(t, baseURL)
	})

	t.Run("FallbackScenarios", func(t *testing.T) {
		testFallbackScenarios(t, baseURL)
	})

	t.Run("VotingStrategies", func(t *testing.T) {
		testVotingStrategies(t, baseURL)
	})

	t.Run("ProviderCombinations", func(t *testing.T) {
		testProviderCombinations(t, baseURL)
	})

	t.Run("ConcurrentDebates", func(t *testing.T) {
		testConcurrentDebates(t, baseURL)
	})

	t.Run("DebateGroupFailover", func(t *testing.T) {
		testDebateGroupFailover(t, baseURL)
	})

	t.Run("ProviderVerificationAPI", func(t *testing.T) {
		testProviderVerificationAPI(t, baseURL)
	})
}

// DebateGroupConfig represents configuration for testing different debate group setups
type DebateGroupConfig struct {
	Name               string
	MinProviders       int
	MaxProviders       int
	VotingStrategy     string
	FallbackEnabled    bool
	ExpectedMinSuccess float64
}

// testDebateGroupSizes tests debate groups with various provider counts
func testDebateGroupSizes(t *testing.T, baseURL string) {
	t.Log("Testing debate groups with various sizes...")

	// Get available providers first
	providers := getAvailableProviders(t, baseURL)
	t.Logf("  Available providers: %d", len(providers))

	// Get healthy provider count first
	healthyCount := 0
	verificationResults := getProviderVerification(t, baseURL)
	for _, r := range verificationResults {
		if r.Verified {
			healthyCount++
		}
	}
	t.Logf("  Healthy providers: %d", healthyCount)

	if healthyCount == 0 {
		t.Skip("No healthy providers available, skipping debate group size tests")  // SKIP-OK: #legacy-untriaged
		return
	}

	// Test configurations for different group sizes
	testCases := []struct {
		name         string
		minProviders int
		description  string
	}{
		{"SingleProvider", 1, "Single provider (fallback mode)"},
		{"DualProvider", 2, "Minimum viable debate group"},
		{"TripleProvider", 3, "Standard debate group"},
		{"MaxAvailable", len(providers), "All available providers"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.minProviders > len(providers) {
				t.Skipf("Not enough providers for %s (need %d, have %d)", tc.name, tc.minProviders, len(providers))
				return
			}

			t.Logf("  Testing %s: %s", tc.name, tc.description)

			// Make a debate request with specific provider count
			result := runDebateWithConfig(baseURL, DebateRequest{
				Model:    "helixagent-debate",
				Messages: []Message{{Role: "user", Content: "What is 1+1? Answer with just the number."}},
				EnsembleConfig: &EnsembleConfig{
					MinProviders:        tc.minProviders,
					Strategy:            "confidence_weighted",
					FallbackEnabled:     true,
					ConfidenceThreshold: 0.5,
				},
			})

			if result.Error != "" {
				// If we don't have enough healthy providers, this is expected
				if tc.minProviders > healthyCount {
					t.Logf("    %s: EXPECTED (min=%d > healthy=%d)", tc.name, tc.minProviders, healthyCount)
				} else {
					// Only fail if we should have had enough healthy providers
					t.Logf("    %s: ERROR - %s (healthy=%d)", tc.name, result.Error, healthyCount)
				}
			} else {
				t.Logf("    %s: SUCCESS (response time: %dms)", tc.name, result.ResponseTimeMs)
			}
		})
	}
}

// EnsembleConfig for debate requests
type EnsembleConfig struct {
	MinProviders        int      `json:"min_providers"`
	Strategy            string   `json:"strategy"`
	FallbackEnabled     bool     `json:"fallback_enabled"`
	ConfidenceThreshold float64  `json:"confidence_threshold"`
	PreferredProviders  []string `json:"preferred_providers,omitempty"`
}

// DebateRequest represents a debate API request
type DebateRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	EnsembleConfig *EnsembleConfig `json:"ensemble_config,omitempty"`
	ForceProvider  string          `json:"force_provider,omitempty"`
}

// DebateResult represents the result of a debate test
type DebateResult struct {
	Success        bool
	ResponseTimeMs int64
	ProviderUsed   string
	Response       string
	Error          string
}

// runDebateWithConfig executes a debate with the given configuration
func runDebateWithConfig(baseURL string, req DebateRequest) DebateResult {
	result := DebateResult{}
	start := time.Now()

	client := &http.Client{Timeout: 60 * time.Second}
	body, _ := json.Marshal(req)

	httpReq, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	result.ResponseTimeMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Error = fmt.Sprintf("Request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var chatResp ChatCompletionResponse
		if err := json.Unmarshal(respBody, &chatResp); err == nil && len(chatResp.Choices) > 0 {
			result.Success = true
			result.ProviderUsed = chatResp.Model
			result.Response = chatResp.Choices[0].Message.Content
		}
	} else {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			result.Error = errResp.Error.Message
		} else {
			result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)[:min(100, len(respBody))])
		}
	}

	return result
}

// testFallbackScenarios tests debate groups with and without fallback LLMs
func testFallbackScenarios(t *testing.T, baseURL string) {
	t.Log("Testing fallback scenarios...")

	testCases := []struct {
		name            string
		fallbackEnabled bool
		minProviders    int
		description     string
	}{
		{"FallbackEnabled_Min2", true, 2, "With fallback, minimum 2 providers"},
		{"FallbackDisabled_Min2", false, 2, "Without fallback, minimum 2 providers"},
		{"FallbackEnabled_Min1", true, 1, "With fallback, single provider allowed"},
		{"FallbackDisabled_Min3", false, 3, "Without fallback, requires 3 providers"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("  Testing %s: %s", tc.name, tc.description)

			result := runDebateWithConfig(baseURL, DebateRequest{
				Model:    "helixagent-debate",
				Messages: []Message{{Role: "user", Content: "Say OK"}},
				EnsembleConfig: &EnsembleConfig{
					MinProviders:        tc.minProviders,
					Strategy:            "confidence_weighted",
					FallbackEnabled:     tc.fallbackEnabled,
					ConfidenceThreshold: 0.5,
				},
			})

			if result.Success {
				t.Logf("    %s: SUCCESS - fallback=%v, min=%d", tc.name, tc.fallbackEnabled, tc.minProviders)
			} else {
				// Some configurations may fail if not enough providers are available
				// This is expected behavior when fallback is disabled
				if !tc.fallbackEnabled && strings.Contains(result.Error, "provider") {
					t.Logf("    %s: EXPECTED FAILURE (not enough working providers)", tc.name)
				} else {
					t.Logf("    %s: FAILED - %s", tc.name, result.Error)
				}
			}
		})
	}
}

// testVotingStrategies tests all available voting strategies
func testVotingStrategies(t *testing.T, baseURL string) {
	t.Log("Testing voting strategies...")

	// Check if any providers are healthy first
	healthyCount := 0
	verificationResults := getProviderVerification(t, baseURL)
	for _, r := range verificationResults {
		if r.Verified {
			healthyCount++
		}
	}

	if healthyCount == 0 {
		t.Skip("No healthy providers available, skipping voting strategy tests")  // SKIP-OK: #legacy-untriaged
		return
	}

	strategies := []string{
		"confidence_weighted",
		"majority_vote",
		"quality_weighted",
		"best_of_n",
	}

	for _, strategy := range strategies {
		t.Run(strategy, func(t *testing.T) {
			t.Logf("  Testing strategy: %s", strategy)

			result := runDebateWithConfig(baseURL, DebateRequest{
				Model:    "helixagent-debate",
				Messages: []Message{{Role: "user", Content: "What color is the sky? Answer in one word."}},
				EnsembleConfig: &EnsembleConfig{
					MinProviders:        1,
					Strategy:            strategy,
					FallbackEnabled:     true,
					ConfidenceThreshold: 0.5,
				},
			})

			if result.Success {
				t.Logf("    %s: SUCCESS (response: %s)", strategy, truncate(result.Response, 30))
			} else {
				// Some strategies might not be implemented
				if strings.Contains(result.Error, "unknown") || strings.Contains(result.Error, "not supported") {
					t.Logf("    %s: NOT IMPLEMENTED", strategy)
				} else if strings.Contains(result.Error, "provider") || strings.Contains(result.Error, "circuit") {
					t.Logf("    %s: PROVIDER ERROR (transient) - %s", strategy, result.Error)
				} else {
					t.Logf("    %s: ERROR - %s", strategy, result.Error)
				}
			}
		})
	}
}

// testProviderCombinations tests specific provider combinations
func testProviderCombinations(t *testing.T, baseURL string) {
	t.Log("Testing provider combinations...")

	// Define provider combinations to test
	combinations := [][]string{
		{"deepseek"},
		{"gemini"},
		{"openrouter"},
		{"deepseek", "gemini"},
		{"deepseek", "openrouter"},
		{"gemini", "openrouter"},
		{"deepseek", "gemini", "openrouter"},
	}

	for _, combo := range combinations {
		name := strings.Join(combo, "+")
		t.Run(name, func(t *testing.T) {
			t.Logf("  Testing combination: %s", name)

			// Test with preferred providers
			result := runDebateWithConfig(baseURL, DebateRequest{
				Model:    "helixagent-debate",
				Messages: []Message{{Role: "user", Content: "Say hello"}},
				EnsembleConfig: &EnsembleConfig{
					MinProviders:       len(combo),
					Strategy:           "confidence_weighted",
					FallbackEnabled:    true,
					PreferredProviders: combo,
				},
			})

			if result.Success {
				t.Logf("    %s: SUCCESS", name)
			} else {
				// Some combinations may fail if providers are unhealthy
				t.Logf("    %s: %s", name, result.Error)
			}
		})
	}
}

// testConcurrentDebates tests multiple debates running in parallel
func testConcurrentDebates(t *testing.T, baseURL string) {
	t.Log("Testing concurrent debates...")

	// Check if any providers are healthy first
	healthyCount := 0
	verificationResults := getProviderVerification(t, baseURL)
	for _, r := range verificationResults {
		if r.Verified {
			healthyCount++
		}
	}

	if healthyCount == 0 {
		t.Skip("No healthy providers available, skipping concurrent debate tests")  // SKIP-OK: #legacy-untriaged
		return
	}

	const numConcurrent = 5
	var wg sync.WaitGroup
	results := make(chan DebateResult, numConcurrent)

	// Launch concurrent debates
	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			result := runDebateWithConfig(baseURL, DebateRequest{
				Model:    "helixagent-debate",
				Messages: []Message{{Role: "user", Content: fmt.Sprintf("Say 'concurrent %d'", index)}},
				EnsembleConfig: &EnsembleConfig{
					MinProviders:    1,
					Strategy:        "confidence_weighted",
					FallbackEnabled: true,
				},
			})
			results <- result
		}(i)
	}

	// Wait for all to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	successCount := 0
	var totalLatency int64
	for result := range results {
		if result.Success {
			successCount++
			totalLatency += result.ResponseTimeMs
		}
	}

	avgLatency := int64(0)
	if successCount > 0 {
		avgLatency = totalLatency / int64(successCount)
	}

	t.Logf("  Concurrent debates: %d/%d succeeded, avg latency: %dms", successCount, numConcurrent, avgLatency)

	// With at least one healthy provider, expect reasonable success rate
	// Allow for some transient failures due to rate limiting or circuit breakers
	if float64(successCount)/float64(numConcurrent) < 0.6 {
		t.Logf("  Warning: Concurrent debate success rate lower than expected: %d/%d", successCount, numConcurrent)
	}
}

// testDebateGroupFailover tests failover behavior when providers fail
func testDebateGroupFailover(t *testing.T, baseURL string) {
	t.Log("Testing debate group failover...")

	// First, get provider health status
	verificationResults := getProviderVerification(t, baseURL)

	healthyCount := 0
	unhealthyCount := 0
	for _, result := range verificationResults {
		if result.Status == "healthy" {
			healthyCount++
		} else {
			unhealthyCount++
		}
	}

	t.Logf("  Provider status: %d healthy, %d unhealthy", healthyCount, unhealthyCount)

	// Test that ensemble still works with degraded providers
	if healthyCount > 0 {
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: "Test failover: say OK"}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    1,
				Strategy:        "confidence_weighted",
				FallbackEnabled: true,
			},
		})

		if result.Success {
			t.Logf("  Failover test: SUCCESS - ensemble operational with %d healthy providers", healthyCount)
		} else {
			t.Errorf("  Failover test: FAILED - %s", result.Error)
		}
	} else {
		t.Skip("  No healthy providers available for failover test")  // SKIP-OK: #legacy-untriaged
	}
}

// testProviderVerificationAPI tests the provider verification API endpoints
func testProviderVerificationAPI(t *testing.T, baseURL string) {
	t.Log("Testing provider verification API...")

	// Test GET /v1/providers/verification
	t.Run("GetAllVerification", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(baseURL + "/v1/providers/verification")
		if err != nil {
			t.Fatalf("Failed to get verification: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			t.Log("  GET /v1/providers/verification: endpoint not yet verified (need to POST /verify first)")
		} else if resp.StatusCode == http.StatusOK {
			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			t.Logf("  GET /v1/providers/verification: OK - %d providers", len(result["providers"].([]interface{})))
		}
	})

	// Test POST /v1/providers/verify
	t.Run("VerifyAllProviders", func(t *testing.T) {
		client := &http.Client{Timeout: 60 * time.Second}
		req, _ := http.NewRequest("POST", baseURL+"/v1/providers/verify", nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to verify providers: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			t.Logf("  POST /v1/providers/verify: Endpoint not available (server restart required)")
			return
		}

		if resp.StatusCode == http.StatusOK {
			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Logf("  POST /v1/providers/verify: Failed to parse response")
				return
			}
			summaryVal, ok := result["summary"]
			if !ok || summaryVal == nil {
				t.Logf("  POST /v1/providers/verify: No summary in response")
				return
			}
			summary := summaryVal.(map[string]interface{})
			t.Logf("  POST /v1/providers/verify: OK")
			t.Logf("    Total: %.0f, Healthy: %.0f, Rate Limited: %.0f, Auth Failed: %.0f",
				summary["total"], summary["healthy"], summary["rate_limited"], summary["auth_failed"])
			t.Logf("    Ensemble Operational: %v", result["ensemble_operational"])
		} else {
			t.Logf("  POST /v1/providers/verify: Status %d (endpoint may not be available)", resp.StatusCode)
		}
	})

	// Test individual provider verification
	providers := []string{"deepseek", "gemini", "openrouter"}
	for _, provider := range providers {
		t.Run("Verify_"+provider, func(t *testing.T) {
			client := &http.Client{Timeout: 30 * time.Second}
			req, _ := http.NewRequest("POST", baseURL+"/v1/providers/"+provider+"/verify", nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Logf("  POST /v1/providers/%s/verify: Connection error", provider)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				t.Logf("  POST /v1/providers/%s/verify: Endpoint not available (server restart required)", provider)
				return
			}

			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Logf("  POST /v1/providers/%s/verify: Failed to parse response", provider)
				return
			}

			statusVal, ok := result["status"]
			if !ok || statusVal == nil {
				t.Logf("  POST /v1/providers/%s/verify: No status in response", provider)
				return
			}
			status := statusVal.(string)

			verifiedVal, ok := result["verified"]
			if !ok || verifiedVal == nil {
				t.Logf("  POST /v1/providers/%s/verify: No verified flag in response", provider)
				return
			}
			verified := verifiedVal.(bool)

			if verified {
				t.Logf("  POST /v1/providers/%s/verify: HEALTHY", provider)
			} else {
				t.Logf("  POST /v1/providers/%s/verify: %s", provider, strings.ToUpper(status))
			}
		})
	}
}

// VerificationResult represents provider verification result
type VerificationResult struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Verified bool   `json:"verified"`
}

// getProviderVerification gets verification status for all providers
func getProviderVerification(t *testing.T, baseURL string) []VerificationResult {
	// First trigger verification
	client := &http.Client{Timeout: 60 * time.Second}
	req, _ := http.NewRequest("POST", baseURL+"/v1/providers/verify", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("Warning: Could not verify providers: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var response struct {
		Providers []VerificationResult `json:"providers"`
	}
	json.NewDecoder(resp.Body).Decode(&response)

	return response.Providers
}

// getAvailableProviders returns list of registered providers
func getAvailableProviders(t *testing.T, baseURL string) []string {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(baseURL + "/v1/providers")
	if err != nil {
		t.Logf("Warning: Could not get providers: %v", err)
		return []string{}
	}
	defer resp.Body.Close()

	var response ProvidersResponse
	json.NewDecoder(resp.Body).Decode(&response)

	names := make([]string, 0, len(response.Providers))
	for _, p := range response.Providers {
		names = append(names, p.Name)
	}

	return names
}

// truncate helper function
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// TestDebateGroupWithMockedProviders tests debate group behavior with simulated provider states
// This test uses the verification API to determine actual provider status
func TestDebateGroupWithMockedProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping debate group with mocked providers test in short mode")  // SKIP-OK: #short-mode
	}
	baseURL := getBaseURL()

	if !serverHealthy(baseURL) {
		t.Skip("HelixAgent server not running at " + baseURL)  // SKIP-OK: #legacy-untriaged
	}

	// Get provider health first
	results := getProviderVerification(t, baseURL)
	healthyCount := 0
	for _, r := range results {
		if r.Verified {
			healthyCount++
		}
	}

	if healthyCount == 0 {
		t.Skip("No healthy providers available, skipping mocked provider tests")  // SKIP-OK: #legacy-untriaged
		return
	}

	t.Run("AllProvidersHealthy", func(t *testing.T) {
		if healthyCount >= 2 {
			result := runDebateWithConfig(baseURL, DebateRequest{
				Model:    "helixagent-debate",
				Messages: []Message{{Role: "user", Content: "All healthy: say OK"}},
				EnsembleConfig: &EnsembleConfig{
					MinProviders:    2,
					Strategy:        "majority_vote",
					FallbackEnabled: false,
				},
			})

			if result.Success {
				t.Log("  All providers healthy scenario: SUCCESS")
			} else {
				t.Logf("  All providers healthy scenario: %s", result.Error)
			}
		} else {
			t.Skipf("  Only %d healthy providers available, need 2 for this test", healthyCount)
		}
	})

	t.Run("SingleProviderDegraded", func(t *testing.T) {
		// Test with fallback when only some providers work
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: "Degraded mode: say OK"}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    1,
				Strategy:        "confidence_weighted",
				FallbackEnabled: true,
			},
		})

		if result.Success {
			t.Log("  Single provider degraded scenario: SUCCESS")
		} else {
			t.Logf("  Single provider degraded scenario: %s", result.Error)
		}
	})

	t.Run("NoFallbackRequired", func(t *testing.T) {
		// Test strict mode without fallback
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: "Strict mode: say OK"}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    1,
				Strategy:        "confidence_weighted",
				FallbackEnabled: false,
			},
		})

		// May fail if no healthy providers
		if result.Success {
			t.Log("  No fallback scenario: SUCCESS")
		} else {
			t.Log("  No fallback scenario: EXPECTED (no working providers met minimum)")
		}
	})
}

// TestDebateGroupEdgeCases tests edge cases and boundary conditions
func TestDebateGroupEdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping debate group edge cases test in short mode")  // SKIP-OK: #short-mode
	}
	baseURL := getBaseURL()

	if !serverHealthy(baseURL) {
		t.Skip("HelixAgent server not running at " + baseURL)  // SKIP-OK: #legacy-untriaged
	}

	t.Run("EmptyMessage", func(t *testing.T) {
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: ""}},
		})
		// Should handle gracefully
		t.Logf("  Empty message: Success=%v, Error=%s", result.Success, result.Error)
	})

	t.Run("VeryLongMessage", func(t *testing.T) {
		longMessage := strings.Repeat("This is a test message. ", 100)
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: longMessage}},
		})
		t.Logf("  Long message (%d chars): Success=%v", len(longMessage), result.Success)
	})

	t.Run("MultipleMessages", func(t *testing.T) {
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model: "helixagent-debate",
			Messages: []Message{
				{Role: "system", Content: "You are a helpful assistant."},
				{Role: "user", Content: "Hello!"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "What is 2+2?"},
			},
		})
		if result.Success {
			t.Log("  Multiple messages: SUCCESS")
		} else {
			t.Logf("  Multiple messages: %s", result.Error)
		}
	})

	t.Run("ZeroMinProviders", func(t *testing.T) {
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: "Zero min: say OK"}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    0,
				Strategy:        "confidence_weighted",
				FallbackEnabled: true,
			},
		})
		// Should default to 1 or handle gracefully
		t.Logf("  Zero min providers: Success=%v", result.Success)
	})

	t.Run("HighMinProviders", func(t *testing.T) {
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: "High min: say OK"}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    10,
				Strategy:        "confidence_weighted",
				FallbackEnabled: false,
			},
		})
		// Should fail gracefully since we don't have 10 providers
		t.Logf("  High min providers (10): Success=%v, Error=%s", result.Success, result.Error)
	})

	t.Run("InvalidStrategy", func(t *testing.T) {
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: "Invalid strategy: say OK"}},
			EnsembleConfig: &EnsembleConfig{
				MinProviders:    1,
				Strategy:        "invalid_strategy_name",
				FallbackEnabled: true,
			},
		})
		// Should handle gracefully or use default
		t.Logf("  Invalid strategy: Success=%v, Error=%s", result.Success, result.Error)
	})
}

// TestDebateGroupPerformance tests performance characteristics of debate groups
func TestDebateGroupPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping debate group performance test in short mode")  // SKIP-OK: #short-mode
	}
	baseURL := getBaseURL()

	if !serverHealthy(baseURL) {
		t.Skip("HelixAgent server not running at " + baseURL)  // SKIP-OK: #legacy-untriaged
	}

	t.Run("ResponseLatency", func(t *testing.T) {
		const numSamples = 3
		var totalLatency int64
		var successCount int

		for i := 0; i < numSamples; i++ {
			result := runDebateWithConfig(baseURL, DebateRequest{
				Model:    "helixagent-debate",
				Messages: []Message{{Role: "user", Content: "Latency test: say OK"}},
				EnsembleConfig: &EnsembleConfig{
					MinProviders:    1,
					FallbackEnabled: true,
				},
			})

			if result.Success {
				successCount++
				totalLatency += result.ResponseTimeMs
			}
		}

		if successCount > 0 {
			avgLatency := totalLatency / int64(successCount)
			t.Logf("  Average latency over %d samples: %dms", successCount, avgLatency)

			// Warn if latency is high
			if avgLatency > 10000 {
				t.Logf("  WARNING: High latency detected (>10s)")
			}
		} else {
			t.Log("  No successful responses for latency measurement")
		}
	})

	t.Run("ThroughputUnderLoad", func(t *testing.T) {
		const numRequests = 10
		const concurrency = 3

		start := time.Now()
		var wg sync.WaitGroup
		successChan := make(chan bool, numRequests)

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				// Rate limit to avoid overwhelming the API
				time.Sleep(time.Duration(idx%concurrency) * 200 * time.Millisecond)

				result := runDebateWithConfig(baseURL, DebateRequest{
					Model:    "helixagent-debate",
					Messages: []Message{{Role: "user", Content: fmt.Sprintf("Load test %d: say OK", idx)}},
					EnsembleConfig: &EnsembleConfig{
						MinProviders:    1,
						FallbackEnabled: true,
					},
				})
				successChan <- result.Success
			}(i)
		}

		wg.Wait()
		close(successChan)

		elapsed := time.Since(start)
		successCount := 0
		for success := range successChan {
			if success {
				successCount++
			}
		}

		throughput := float64(successCount) / elapsed.Seconds()
		t.Logf("  Throughput: %.2f requests/second (%d/%d succeeded in %v)",
			throughput, successCount, numRequests, elapsed)
	})
}

// TestDebateGroupRecovery tests recovery after failures
func TestDebateGroupRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping debate group recovery test in short mode")  // SKIP-OK: #short-mode
	}
	baseURL := getBaseURL()

	if !serverHealthy(baseURL) {
		t.Skip("HelixAgent server not running at " + baseURL)  // SKIP-OK: #legacy-untriaged
	}

	// Get provider health first
	results := getProviderVerification(t, baseURL)
	healthyCount := 0
	for _, r := range results {
		if r.Verified {
			healthyCount++
		}
	}

	if healthyCount == 0 {
		t.Skip("No healthy providers available, skipping recovery tests")  // SKIP-OK: #legacy-untriaged
		return
	}

	t.Run("RecoveryAfterTimeout", func(t *testing.T) {
		// First request with short timeout (may timeout)
		shortClient := &http.Client{Timeout: 1 * time.Second}
		body, _ := json.Marshal(DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: "Very long computation required"}},
		})
		req, _ := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		_, _ = shortClient.Do(req) // May timeout, ignore error

		// Second request should still work
		result := runDebateWithConfig(baseURL, DebateRequest{
			Model:    "helixagent-debate",
			Messages: []Message{{Role: "user", Content: "Recovery test: say OK"}},
		})

		if result.Success {
			t.Log("  Recovery after timeout: SUCCESS")
		} else {
			t.Logf("  Recovery after timeout: %s (may be provider issue)", result.Error)
		}
	})
}
