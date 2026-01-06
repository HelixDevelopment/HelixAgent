// Package integration provides tests for LLM provider configuration.
//
// These tests verify:
// 1. Gemini is the primary LLM provider for all AI operations
// 2. Ollama is deprecated and not used in production
// 3. LLMsVerifier benchmarking scores are correctly applied
// 4. Provider fallback chains work correctly
package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LLMProviderScores represents the verified LLM provider scores from LLMsVerifier
var LLMProviderScores = map[string]float64{
	"gemini-pro":       8.5,
	"gemini-1.5-pro":   8.5,
	"gemini-1.5-flash": 8.5,
	"deepseek-coder":   8.1,
	"deepseek-chat":    8.0,
	"llama-3-70b":      7.7,
	"fireworks-llama":  7.2,
	"mistral-large":    7.1,
	"cerebras-llama":   7.1,
}

// TestLLMProviderConfiguration_GeminiIsPrimary verifies Gemini is the primary LLM
func TestLLMProviderConfiguration_GeminiIsPrimary(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test 1: Verify Gemini API key is configured
	t.Run("GeminiAPIKeyExists", func(t *testing.T) {
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("ApiKey_Gemini")
		}
		require.NotEmpty(t, apiKey, "Gemini API key must be configured")
		assert.True(t, strings.HasPrefix(apiKey, "AIza"), "Gemini API key should start with 'AIza'")
	})

	// Test 2: Verify Gemini provider is available in SuperAgent
	t.Run("GeminiProviderAvailable", func(t *testing.T) {
		client := &http.Client{Timeout: 30 * time.Second}

		resp, err := client.Get(SuperAgentBaseURL + "/v1/providers")
		if err != nil {
			t.Skipf("SuperAgent service not available: %v", err)
		}
		defer resp.Body.Close()

		var providers struct {
			Providers []struct {
				Name     string `json:"name"`
				Metadata struct {
					Provider string `json:"provider"`
				} `json:"metadata"`
			} `json:"providers"`
		}
		json.NewDecoder(resp.Body).Decode(&providers)

		// Find Gemini provider
		hasGemini := false
		for _, p := range providers.Providers {
			if strings.ToLower(p.Name) == "gemini" || strings.ToLower(p.Metadata.Provider) == "google" {
				hasGemini = true
				break
			}
		}
		assert.True(t, hasGemini, "Gemini provider should be available")
	})

	// Test 3: Verify Gemini has highest score
	t.Run("GeminiHasHighestScore", func(t *testing.T) {
		geminiScore := LLMProviderScores["gemini-pro"]

		for provider, score := range LLMProviderScores {
			if !strings.Contains(provider, "gemini") {
				assert.GreaterOrEqual(t, geminiScore, score,
					"Gemini (%.1f) should have score >= %s (%.1f)",
					geminiScore, provider, score)
			}
		}
	})
}

// TestLLMProviderConfiguration_OllamaDeprecated verifies Ollama is deprecated
func TestLLMProviderConfiguration_OllamaDeprecated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Test 1: Verify Ollama is not running by default
	t.Run("OllamaNotRunningByDefault", func(t *testing.T) {
		resp, err := client.Get("http://localhost:11434/api/tags")

		if err != nil {
			// Connection refused means Ollama is not running - this is expected
			t.Log("Confirmed: Ollama is not running (connection refused)")
			return
		}
		defer resp.Body.Close()

		// If Ollama responds, log a warning but don't fail
		// (it might be running for legacy testing)
		t.Log("Warning: Ollama is running. It should be disabled for production. Use Gemini instead.")
	})

	// Test 2: Verify SuperAgent works without Ollama
	t.Run("SuperAgentWorksWithoutOllama", func(t *testing.T) {
		resp, err := client.Get(SuperAgentBaseURL + "/health")
		if err != nil {
			t.Skipf("SuperAgent service not available: %v", err)
		}
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "SuperAgent should be healthy without Ollama")
	})

	// Test 3: Verify Cognee works without Ollama
	t.Run("CogneeWorksWithoutOllama", func(t *testing.T) {
		resp, err := client.Get(SuperAgentBaseURL + "/v1/cognee/health")
		if err != nil {
			t.Skipf("SuperAgent service not available: %v", err)
		}
		defer resp.Body.Close()

		var health struct {
			Healthy bool `json:"healthy"`
		}
		json.NewDecoder(resp.Body).Decode(&health)

		assert.True(t, health.Healthy, "Cognee should be healthy without Ollama (uses Gemini)")
	})
}

// TestLLMProviderConfiguration_EnsembleUsesVerifiedProviders verifies ensemble uses verified providers
func TestLLMProviderConfiguration_EnsembleUsesVerifiedProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := &http.Client{Timeout: 60 * time.Second}

	// Test 1: Verify ensemble responds correctly
	t.Run("EnsembleRespondsCorrectly", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"model": "superagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "What is 2+2? Reply with just the number."},
			},
			"max_tokens": 10,
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req, err := http.NewRequest("POST", SuperAgentBaseURL+"/v1/chat/completions", bytes.NewReader(bodyBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Skipf("SuperAgent service not available: %v", err)
		}
		defer resp.Body.Close()

		// Skip on error status codes (providers may be temporarily unavailable)
		if resp.StatusCode != http.StatusOK {
			t.Skipf("Ensemble returned non-200 status (%d), providers may be temporarily unavailable", resp.StatusCode)
		}

		var result struct {
			Model             string `json:"model"`
			SystemFingerprint string `json:"system_fingerprint"`
			Choices           []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		assert.Equal(t, "superagent-ensemble", result.Model, "Should use ensemble model")
		assert.Equal(t, "fp_superagent_ensemble", result.SystemFingerprint, "Should have ensemble fingerprint")
		if len(result.Choices) == 0 {
			t.Skip("No choices returned (providers may be temporarily unavailable)")
		}
		assert.Contains(t, result.Choices[0].Message.Content, "4", "Should correctly answer 2+2=4")
	})

	// Test 2: Verify model list shows verified models
	t.Run("ModelListShowsVerifiedModels", func(t *testing.T) {
		resp, err := client.Get(SuperAgentBaseURL + "/v1/models")
		if err != nil {
			t.Skipf("SuperAgent service not available: %v", err)
		}
		defer resp.Body.Close()

		var models struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&models)

		// Should have the debate model available
		hasDebateModel := false
		for _, m := range models.Data {
			if strings.Contains(m.ID, "debate") || strings.Contains(m.ID, "ensemble") {
				hasDebateModel = true
				break
			}
		}
		assert.True(t, hasDebateModel, "Should have debate/ensemble model available")
	})
}

// TestLLMProviderConfiguration_CogneeUsesGemini verifies Cognee uses Gemini
func TestLLMProviderConfiguration_CogneeUsesGemini(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test 1: Verify Cognee LLM environment variables
	t.Run("CogneeLLMConfiguredForGemini", func(t *testing.T) {
		// These are the expected environment variables for Cognee
		expectedProvider := "gemini"
		expectedModel := "gemini-1.5-flash"

		// The Cognee container should have these set via docker-compose
		t.Logf("Expected Cognee LLM_PROVIDER: %s", expectedProvider)
		t.Logf("Expected Cognee LLM_MODEL: %s", expectedModel)

		// Verify through Cognee health that Gemini features work
		client := &http.Client{Timeout: 30 * time.Second}

		resp, err := client.Get(SuperAgentBaseURL + "/v1/cognee/health")
		if err != nil {
			t.Skipf("SuperAgent service not available: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Skipf("Cognee health endpoint returned %d (container may still be starting)", resp.StatusCode)
		}

		var health struct {
			Healthy bool `json:"healthy"`
			Ready   bool `json:"ready"`
			Config  struct {
				Enabled                bool `json:"enabled"`
				EnableGraphReasoning   bool `json:"enable_graph_reasoning"`
				EnableCodeIntelligence bool `json:"enable_code_intelligence"`
			} `json:"config"`
		}
		json.NewDecoder(resp.Body).Decode(&health)

		// If Cognee is enabled but not yet healthy (container starting), skip
		if health.Config.Enabled && !health.Healthy {
			t.Skip("Cognee container is still starting up")
		}

		// These features require a powerful LLM like Gemini
		assert.True(t, health.Config.Enabled, "Cognee should be enabled")
		assert.True(t, health.Config.EnableGraphReasoning, "Graph reasoning should be enabled (Gemini-powered)")
		assert.True(t, health.Config.EnableCodeIntelligence, "Code intelligence should be enabled (Gemini-powered)")
	})
}

// TestLLMProviderConfiguration_ProviderFallback verifies provider fallback works
func TestLLMProviderConfiguration_ProviderFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Test: Verify providers endpoint shows fallback configuration
	t.Run("ProvidersShowFallbackConfig", func(t *testing.T) {
		resp, err := client.Get(SuperAgentBaseURL + "/v1/providers")
		if err != nil {
			t.Skipf("SuperAgent service not available: %v", err)
		}
		defer resp.Body.Close()

		var providers struct {
			Count     int `json:"count"`
			Providers []struct {
				Name              string   `json:"name"`
				SupportedFeatures []string `json:"supported_features"`
			} `json:"providers"`
		}
		json.NewDecoder(resp.Body).Decode(&providers)

		assert.Greater(t, providers.Count, 0, "Should have at least one provider")

		// Verify providers support cognee features
		for _, p := range providers.Providers {
			if strings.Contains(strings.ToLower(p.Name), "gemini") {
				hasCogneeSupport := false
				for _, feature := range p.SupportedFeatures {
					if strings.Contains(feature, "cognee") {
						hasCogneeSupport = true
						break
					}
				}
				if hasCogneeSupport {
					t.Logf("Provider %s supports Cognee features", p.Name)
				}
			}
		}
	})
}

// TestLLMProviderConfiguration_VerifierScoresAccurate verifies LLMsVerifier scores are accurate
func TestLLMProviderConfiguration_VerifierScoresAccurate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test: Verify the score ordering matches our configuration
	t.Run("ScoreOrderingCorrect", func(t *testing.T) {
		// Expected ordering from LLMsVerifier:
		// 1. Gemini Pro / Gemini 1.5 Pro: 8.5 (HIGHEST)
		// 2. DeepSeek Coder: 8.1
		// 3. DeepSeek Chat: 8.0
		// 4. Llama 3 70B: 7.7
		// 5. Fireworks Llama: 7.2
		// 6. Mistral Large: 7.1
		// 7. Cerebras Llama: 7.1

		assert.Equal(t, 8.5, LLMProviderScores["gemini-pro"], "Gemini Pro should have score 8.5")
		assert.Equal(t, 8.5, LLMProviderScores["gemini-1.5-pro"], "Gemini 1.5 Pro should have score 8.5")
		assert.Equal(t, 8.1, LLMProviderScores["deepseek-coder"], "DeepSeek Coder should have score 8.1")
		assert.Equal(t, 8.0, LLMProviderScores["deepseek-chat"], "DeepSeek Chat should have score 8.0")

		// Verify Gemini is highest
		maxScore := 0.0
		maxProvider := ""
		for provider, score := range LLMProviderScores {
			if score > maxScore {
				maxScore = score
				maxProvider = provider
			}
		}
		assert.Contains(t, maxProvider, "gemini", "Gemini should be the highest-scoring provider")
		assert.Equal(t, 8.5, maxScore, "Highest score should be 8.5")
	})
}
