package challenge

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"dev.helix.agent/internal/services"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestProviderAutoDiscovery tests the provider auto-discovery system
// This test suite ensures that providers are automatically detected from environment variables
//
// Run with: go test -v ./tests/challenge -run TestProviderAutoDiscovery -timeout 300s
func TestProviderAutoDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping provider auto-discovery in short mode (creates live provider registries)")  // SKIP-OK: #short-mode
	}

	t.Run("DiscoveryFromEnvironment", testDiscoveryFromEnvironment)
	t.Run("ProviderScoring", testProviderScoring)
	t.Run("BestProviderSelection", testBestProviderSelection)
	t.Run("BackwardCompatibility", testBackwardCompatibility)
}

// testDiscoveryFromEnvironment verifies that providers are discovered from environment variables
func testDiscoveryFromEnvironment(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Create a new ProviderDiscovery instance
	discovery := services.NewProviderDiscovery(logger, false)

	// Discover providers from environment
	discovered, err := discovery.DiscoverProviders()
	if err != nil {
		t.Fatalf("Failed to discover providers: %v", err)
	}

	t.Logf("Discovered %d providers from environment", len(discovered))

	// Verify that at least some providers are discovered (if API keys are set)
	// Note: This depends on environment configuration
	for _, provider := range discovered {
		t.Logf("  - %s (type: %s, env: %s)", provider.Name, provider.Type, provider.APIKeyEnvVar)

		// Verify provider has required fields
		if provider.Name == "" {
			t.Errorf("Provider has empty name")
		}
		if provider.Type == "" {
			t.Errorf("Provider %s has empty type", provider.Name)
		}
		if provider.APIKeyEnvVar == "" {
			t.Errorf("Provider %s has empty APIKeyEnvVar", provider.Name)
		}
	}

	// Get summary
	summary := discovery.Summary()
	t.Logf("Discovery summary: %d discovered, %d healthy, debate_ready=%v",
		summary["total_discovered"], summary["healthy"], summary["debate_ready"])
}

// testProviderScoring verifies the provider scoring system
func testProviderScoring(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Create discovery service
	discovery := services.NewProviderDiscovery(logger, false)

	// Discover providers
	discovered, _ := discovery.DiscoverProviders()
	if len(discovered) == 0 {
		t.Skip("No providers discovered from environment")  // SKIP-OK: #legacy-untriaged
	}

	// Verify providers and check scores
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Verify all providers
	discovery.VerifyAllProviders(ctx)

	// Check that working providers have scores
	for _, provider := range discovered {
		if provider.Verified {
			t.Logf("Provider %s verified with score: %.2f", provider.Name, provider.Score)

			// Verify score is within expected range (0-10)
			if provider.Score < 0 || provider.Score > 10 {
				t.Errorf("Provider %s has invalid score: %.2f (expected 0-10)", provider.Name, provider.Score)
			}

			// Verify score details are available
			scoreDetails := discovery.GetProviderScore(provider.Name)
			if scoreDetails != nil {
				t.Logf("  Score details: overall=%.2f, speed=%.2f, capabilities=%.2f",
					scoreDetails.OverallScore, scoreDetails.ResponseSpeed, scoreDetails.Capabilities)
			}
		}
	}
}

// testBestProviderSelection verifies the best provider selection for debate groups
func testBestProviderSelection(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	discovery := services.NewProviderDiscovery(logger, false)

	// Discover and verify providers
	discovered, _ := discovery.DiscoverProviders()
	if len(discovered) == 0 {
		t.Skip("No providers discovered from environment")  // SKIP-OK: #legacy-untriaged
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	discovery.VerifyAllProviders(ctx)

	// Test GetBestProviders with various counts
	testCases := []struct {
		name         string
		count        int
		minExpected  int
		expectSorted bool
	}{
		{"Top 3 providers", 3, 0, true},
		{"Top 5 providers", 5, 0, true},
		{"All providers", 0, 0, true}, // 0 means all
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			best := discovery.GetBestProviders(tc.count)
			t.Logf("GetBestProviders(%d) returned %d providers", tc.count, len(best))

			if len(best) < tc.minExpected {
				t.Errorf("Expected at least %d providers, got %d", tc.minExpected, len(best))
			}

			// Verify providers are sorted by score (descending)
			if tc.expectSorted && len(best) > 1 {
				for i := 1; i < len(best); i++ {
					if best[i].Score > best[i-1].Score {
						t.Errorf("Providers not sorted by score: %s (%.2f) > %s (%.2f)",
							best[i].Name, best[i].Score, best[i-1].Name, best[i-1].Score)
					}
				}
			}

			// Log the best providers
			for i, p := range best {
				t.Logf("  #%d: %s (score: %.2f)", i+1, p.Name, p.Score)
			}
		})
	}

	// Test GetDebateGroupProviders
	t.Run("DebateGroupProviders", func(t *testing.T) {
		debateProviders := discovery.GetDebateGroupProviders(2, 5)
		t.Logf("GetDebateGroupProviders(2, 5) returned %d providers", len(debateProviders))
		// Validate returned providers have required fields populated
		for _, p := range debateProviders {
			assert.NotEmpty(t, p.Name, "provider name must not be empty")
			assert.GreaterOrEqual(t, p.Score, 0.0, "provider score must be non-negative")
			t.Logf("  - %s (score: %.2f, verified: %v)", p.Name, p.Score, p.Verified)
		}
	})
}

// testBackwardCompatibility verifies that config files still work alongside auto-discovery
func testBackwardCompatibility(t *testing.T) {
	// Create registry config (simulating config file)
	cfg := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		CircuitBreaker: services.CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 5,
			RecoveryTimeout:  60 * time.Second,
			SuccessThreshold: 2,
		},
		Providers: make(map[string]*services.ProviderConfig),
	}

	// Add a provider via config (simulating config file)
	if apiKey := os.Getenv("DEEPSEEK_API_KEY"); apiKey != "" {
		cfg.Providers["deepseek"] = &services.ProviderConfig{
			Name:    "deepseek",
			Type:    "deepseek",
			Enabled: true,
			APIKey:  apiKey,
			BaseURL: "https://api.deepseek.com",
			Models: []services.ModelConfig{{
				ID:      "deepseek-chat",
				Enabled: true,
			}},
		}
	}

	// Create registry (this will trigger auto-discovery)
	registry := services.NewProviderRegistry(cfg, nil)

	// Verify config-based provider is registered
	providers := registry.ListProviders()
	t.Logf("Registry has %d providers after initialization", len(providers))

	for _, name := range providers {
		t.Logf("  - %s", name)
	}

	// Verify auto-discovery is available
	discovery := registry.GetDiscovery()
	if discovery == nil {
		t.Log("Auto-discovery is not initialized (may be disabled)")
	} else {
		t.Log("Auto-discovery is initialized and available")

		// Get discovery summary
		summary := discovery.Summary()
		t.Logf("Auto-discovery summary: %v discovered providers", summary["total_discovered"])
	}

	// Test that config providers take precedence
	if _, err := registry.GetProvider("deepseek"); err != nil && os.Getenv("DEEPSEEK_API_KEY") != "" {
		t.Log("DeepSeek provider not available (config-based registration)")
	}
}

// TestProviderAutoDiscoveryAPI tests the HTTP endpoints for auto-discovery
// This test requires a running HelixAgent server
//
// Run with: go test -v ./tests/challenge -run TestProviderAutoDiscoveryAPI -timeout 300s
func TestProviderAutoDiscoveryAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping auto-discovery API test in short mode (requires live server)")  // SKIP-OK: #short-mode
	}

	baseURL := getBaseURL()

	// Skip if server is not running
	if !serverHealthy(baseURL) {
		t.Skip("HelixAgent server not running at " + baseURL)  // SKIP-OK: #legacy-untriaged
	}

	t.Run("GetDiscoverySummary", func(t *testing.T) {
		testGetDiscoverySummary(t, baseURL)
	})

	t.Run("DiscoverAndVerify", func(t *testing.T) {
		testDiscoverAndVerify(t, baseURL)
	})

	t.Run("GetBestProvidersAPI", func(t *testing.T) {
		testGetBestProvidersAPI(t, baseURL)
	})

	t.Run("ReDiscoverProviders", func(t *testing.T) {
		testReDiscoverProviders(t, baseURL)
	})
}

func testGetDiscoverySummary(t *testing.T, baseURL string) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(baseURL + "/v1/providers/discovery")
	if err != nil {
		t.Fatalf("Failed to get discovery summary: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	t.Logf("Discovery summary response: %v", result)

	// Verify response structure
	if _, ok := result["total_discovered"]; ok {
		t.Logf("  Total discovered: %v", result["total_discovered"])
	}
	if _, ok := result["healthy"]; ok {
		t.Logf("  Healthy: %v", result["healthy"])
	}
	if _, ok := result["debate_ready"]; ok {
		t.Logf("  Debate ready: %v", result["debate_ready"])
	}
}

func testDiscoverAndVerify(t *testing.T, baseURL string) {
	client := &http.Client{Timeout: 120 * time.Second} // Longer timeout for verification

	resp, err := client.Post(baseURL+"/v1/providers/discover", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to trigger discovery: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("Unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	t.Logf("Discover and verify response: %v", result)

	if summary, ok := result["summary"].(map[string]interface{}); ok {
		t.Logf("  Summary: healthy=%v, rate_limited=%v, debate_ready=%v",
			summary["healthy"], summary["rate_limited"], summary["debate_ready"])
	}
}

func testGetBestProvidersAPI(t *testing.T, baseURL string) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Test default parameters
	resp, err := client.Get(baseURL + "/v1/providers/best")
	if err != nil {
		t.Fatalf("Failed to get best providers: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	t.Logf("Best providers response: count=%v, debate_ready=%v",
		result["count"], result["debate_group_ready"])

	if providers, ok := result["best_providers"].([]interface{}); ok {
		for i, p := range providers {
			if provider, ok := p.(map[string]interface{}); ok {
				t.Logf("  #%d: %v (score: %v)", i+1, provider["name"], provider["score"])
			}
		}
	}

	// Test with custom parameters
	resp2, err := client.Get(baseURL + "/v1/providers/best?min=2&max=3")
	if err != nil {
		t.Fatalf("Failed to get best providers with params: %v", err)
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	var result2 map[string]interface{}
	if err := json.Unmarshal(body2, &result2); err == nil {
		t.Logf("Best providers (min=2, max=3): count=%v", result2["count"])
	}
}

func testReDiscoverProviders(t *testing.T, baseURL string) {
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Post(baseURL+"/v1/providers/rediscover", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to trigger re-discovery: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("Unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	t.Logf("Re-discover response: discovered=%v, newly_registered=%v, total=%v",
		result["discovered"], result["newly_registered"], result["total_providers"])
}

// TestProviderAutoDiscoveryIntegration tests the complete integration
// This test verifies that auto-discovered providers work correctly in the ensemble
//
// Run with: go test -v ./tests/challenge -run TestProviderAutoDiscoveryIntegration -timeout 300s
func TestProviderAutoDiscoveryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping auto-discovery integration test in short mode (requires live server and providers)")  // SKIP-OK: #short-mode
	}

	baseURL := getBaseURL()

	// Skip if server is not running
	if !serverHealthy(baseURL) {
		t.Skip("HelixAgent server not running at " + baseURL)  // SKIP-OK: #legacy-untriaged
	}

	client := &http.Client{Timeout: 120 * time.Second}

	// Step 1: Trigger discovery and verification
	t.Log("Step 1: Triggering provider discovery and verification...")
	discoverResp, err := client.Post(baseURL+"/v1/providers/discover", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to trigger discovery: %v", err)
	}
	defer discoverResp.Body.Close()
	discoverBody, _ := io.ReadAll(discoverResp.Body)

	var discoverResult map[string]interface{}
	json.Unmarshal(discoverBody, &discoverResult)
	t.Logf("Discovery result: %v", discoverResult)

	// Step 2: Get best providers
	t.Log("Step 2: Getting best providers for debate group...")
	bestResp, err := client.Get(baseURL + "/v1/providers/best?min=2&max=5")
	if err != nil {
		t.Fatalf("Failed to get best providers: %v", err)
	}
	defer bestResp.Body.Close()
	bestBody, _ := io.ReadAll(bestResp.Body)

	var bestResult map[string]interface{}
	json.Unmarshal(bestBody, &bestResult)
	t.Logf("Best providers: %v", bestResult)

	// Step 3: Verify ensemble can process a request (if providers are available)
	debateReady := false
	if ready, ok := bestResult["debate_group_ready"].(bool); ok {
		debateReady = ready
	}

	if !debateReady {
		t.Log("Step 3: Skipping ensemble test - not enough providers for debate group")
		return
	}

	t.Log("Step 3: Testing ensemble with auto-discovered providers...")
	ensembleReq := map[string]interface{}{
		"model": "helixagent-debate",
		"messages": []map[string]string{
			{"role": "user", "content": "Say 'Auto-discovery integration test successful'"},
		},
	}
	reqBody, _ := json.Marshal(ensembleReq)

	ensembleResp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Logf("Ensemble request failed: %v", err)
		return
	}
	defer ensembleResp.Body.Close()

	if ensembleResp.StatusCode == http.StatusOK {
		t.Log("Step 3: Ensemble request successful!")

		var ensembleResult map[string]interface{}
		ensembleBody, _ := io.ReadAll(ensembleResp.Body)
		json.Unmarshal(ensembleBody, &ensembleResult)

		if choices, ok := ensembleResult["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if msg, ok := choice["message"].(map[string]interface{}); ok {
					content := msg["content"]
					if len(content.(string)) > 50 {
						content = content.(string)[:50] + "..."
					}
					t.Logf("Response: %v", content)
				}
			}
		}
	} else {
		ensembleBody, _ := io.ReadAll(ensembleResp.Body)
		t.Logf("Step 3: Ensemble request returned status %d: %s", ensembleResp.StatusCode, string(ensembleBody))
	}
}

// TestProviderDiscoveryMapping tests the provider mapping system
func TestProviderDiscoveryMapping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping provider discovery mapping in short mode (creates live provider discoveries)")  // SKIP-OK: #short-mode
	}

	// Test that known API key environment variables are properly mapped
	knownMappings := []struct {
		envVar           string
		expectedProvider string
		expectedType     string
	}{
		{"ANTHROPIC_API_KEY", "claude", "claude"},
		{"CLAUDE_API_KEY", "claude", "claude"},
		{"OPENAI_API_KEY", "openai", "openai"},
		{"GEMINI_API_KEY", "gemini", "gemini"},
		{"DEEPSEEK_API_KEY", "deepseek", "deepseek"},
		{"MISTRAL_API_KEY", "mistral", "mistral"},
		{"OPENROUTER_API_KEY", "openrouter", "openrouter"},
		{"GROQ_API_KEY", "groq", "groq"},
		{"CEREBRAS_API_KEY", "cerebras", "cerebras"},
		{"SAMBANOVA_API_KEY", "sambanova", "sambanova"},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Suppress info logs

	for _, tc := range knownMappings {
		t.Run(tc.envVar, func(t *testing.T) {
			// Temporarily set the env var with a test value
			originalValue := os.Getenv(tc.envVar)
			os.Setenv(tc.envVar, "test-api-key-for-mapping-test")

			// Create discovery and discover
			discovery := services.NewProviderDiscovery(logger, false)
			discovered, err := discovery.DiscoverProviders()

			// Restore original value
			if originalValue == "" {
				os.Unsetenv(tc.envVar)
			} else {
				os.Setenv(tc.envVar, originalValue)
			}

			if err != nil {
				t.Fatalf("Discovery failed: %v", err)
			}

			// Find the discovered provider
			var found *services.DiscoveredProvider
			for _, p := range discovered {
				if p.Name == tc.expectedProvider {
					found = p
					break
				}
			}

			if found == nil {
				t.Errorf("Expected to discover provider %s from %s, but it was not found", tc.expectedProvider, tc.envVar)
				return
			}

			if found.Type != tc.expectedType {
				t.Errorf("Expected provider type %s, got %s", tc.expectedType, found.Type)
			}

			t.Logf("✓ %s -> %s (type: %s)", tc.envVar, found.Name, found.Type)
		})
	}
}
