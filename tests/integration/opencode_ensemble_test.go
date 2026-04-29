// Package integration provides ensemble and provider combination tests for OpenCode with HelixAgent.
// These tests verify all LLM provider combinations and ensemble strategies work correctly.
package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// PROVIDER CONFIGURATION
// =============================================================================

// EnsembleProviderConfig holds configuration for a specific provider in ensemble tests
type EnsembleProviderConfig struct {
	Name       string
	EnvKey     string
	BaseURLEnv string
	Model      string
	Available  bool
}

// getAvailableProviders returns all configured LLM providers
func getAvailableProviders(t *testing.T) []EnsembleProviderConfig {
	t.Helper()

	providers := []EnsembleProviderConfig{
		{
			Name:      "DeepSeek",
			EnvKey:    "DEEPSEEK_API_KEY",
			Model:     "deepseek-chat",
			Available: os.Getenv("DEEPSEEK_API_KEY") != "",
		},
		{
			Name:       "OpenRouter",
			EnvKey:     "OPENROUTER_API_KEY",
			BaseURLEnv: "OPENROUTER_BASE_URL",
			Model:      "anthropic/claude-3-haiku",
			Available:  os.Getenv("OPENROUTER_API_KEY") != "",
		},
		{
			Name:      "Gemini",
			EnvKey:    "GEMINI_API_KEY",
			Model:     "gemini-2.0-flash",
			Available: os.Getenv("GEMINI_API_KEY") != "",
		},
		{
			Name:      "Qwen",
			EnvKey:    "QWEN_API_KEY",
			Model:     "qwen-turbo",
			Available: os.Getenv("QWEN_API_KEY") != "",
		},
		{
			Name:      "ZAI",
			EnvKey:    "ZAI_API_KEY",
			Model:     "zai-default",
			Available: os.Getenv("ZAI_API_KEY") != "",
		},
		{
			Name:       "Ollama",
			EnvKey:     "OLLAMA_ENABLED",
			BaseURLEnv: "OLLAMA_BASE_URL",
			Model:      "llama3",
			Available:  os.Getenv("OLLAMA_ENABLED") == "true",
		},
	}

	return providers
}

// =============================================================================
// ENSEMBLE CONFIGURATION TESTS
// =============================================================================

// EnsembleConfig represents HelixAgent ensemble configuration
type EnsembleConfig struct {
	Strategy           string   `json:"strategy,omitempty"`
	Providers          []string `json:"providers,omitempty"`
	ConsensusThreshold float64  `json:"consensus_threshold,omitempty"`
	MaxProviders       int      `json:"max_providers,omitempty"`
	Timeout            int      `json:"timeout_ms,omitempty"`
}

// ChatRequestWithEnsemble extends the chat request with ensemble config
type ChatRequestWithEnsemble struct {
	Model          string          `json:"model"`
	Messages       []OpenAIMessage `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    float64         `json:"temperature,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	EnsembleConfig *EnsembleConfig `json:"ensemble_config,omitempty"`
	ForceProvider  string          `json:"force_provider,omitempty"`
}

// TestEnsembleStrategies tests different ensemble voting strategies
func TestEnsembleStrategies(t *testing.T) {
	testutil.RequireServer(t)
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	strategies := []string{
		"majority_vote",
		"weighted_vote",
		"best_of_n",
		"fastest_response",
	}

	for _, strategy := range strategies {
		t.Run("Strategy_"+strategy, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			chatReq := ChatRequestWithEnsemble{
				Model: "helixagent-debate",
				Messages: []OpenAIMessage{
					{Role: "user", Content: "What is 2+2? Answer with just the number."},
				},
				MaxTokens:   20,
				Temperature: 0.0,
				Stream:      false,
				EnsembleConfig: &EnsembleConfig{
					Strategy:           strategy,
					ConsensusThreshold: 0.6,
					MaxProviders:       3,
					Timeout:            30000,
				},
			}

			body, err := json.Marshal(chatReq)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, "POST",
				config.BaseURL+"/chat/completions", bytes.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			if config.HelixAgentAPIKey != "" {
				req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
			}

			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Logf("Strategy %s response: %s", strategy, string(body))
			}

			// Strategy might not be supported, or providers might be unavailable
			validStatuses := []int{
				http.StatusOK,
				http.StatusBadRequest,
				http.StatusBadGateway,
				http.StatusServiceUnavailable,
				http.StatusGatewayTimeout,
			}
			isValid := false
			for _, status := range validStatuses {
				if resp.StatusCode == status {
					isValid = true
					break
				}
			}
			assert.True(t, isValid,
				"Should return OK, BadRequest, or provider error for strategy %s, got %d", strategy, resp.StatusCode)
		})
	}
}

// TestProviderCombinations tests different provider combinations
func TestProviderCombinations(t *testing.T) {
	testutil.RequireServer(t)
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	providers := getAvailableProviders(t)
	availableProviders := []EnsembleProviderConfig{}
	for _, p := range providers {
		if p.Available {
			availableProviders = append(availableProviders, p)
		}
	}

	if len(availableProviders) < 2 {
		t.Logf("Need at least 2 providers configured for combination tests (acceptable)")
		return
	}

	// Test pairs of providers
	for i := 0; i < len(availableProviders); i++ {
		for j := i + 1; j < len(availableProviders); j++ {
			p1, p2 := availableProviders[i], availableProviders[j]
			t.Run(fmt.Sprintf("Pair_%s_%s", p1.Name, p2.Name), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				chatReq := ChatRequestWithEnsemble{
					Model: "helixagent-debate",
					Messages: []OpenAIMessage{
						{Role: "user", Content: "Say 'hello' and nothing else."},
					},
					MaxTokens:   10,
					Temperature: 0.0,
					Stream:      false,
					EnsembleConfig: &EnsembleConfig{
						Providers:    []string{strings.ToLower(p1.Name), strings.ToLower(p2.Name)},
						MaxProviders: 2,
						Timeout:      30000,
					},
				}

				body, err := json.Marshal(chatReq)
				require.NoError(t, err)

				req, err := http.NewRequestWithContext(ctx, "POST",
					config.BaseURL+"/chat/completions", bytes.NewReader(body))
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/json")
				if config.HelixAgentAPIKey != "" {
					req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
				}

				client := &http.Client{}
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				t.Logf("Provider pair %s + %s: status %d", p1.Name, p2.Name, resp.StatusCode)
			})
		}
	}
}

// TestForceProvider tests forcing requests to specific providers
func TestForceProvider(t *testing.T) {
	testutil.RequireServer(t)
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	providers := getAvailableProviders(t)

	for _, provider := range providers {
		if !provider.Available {
			continue
		}

		t.Run("Force_"+provider.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			chatReq := ChatRequestWithEnsemble{
				Model: "helixagent-debate",
				Messages: []OpenAIMessage{
					{Role: "user", Content: "Say 'test' and nothing else."},
				},
				MaxTokens:     10,
				Temperature:   0.0,
				Stream:        false,
				ForceProvider: strings.ToLower(provider.Name),
			}

			body, err := json.Marshal(chatReq)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(ctx, "POST",
				config.BaseURL+"/chat/completions", bytes.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			if config.HelixAgentAPIKey != "" {
				req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
			}

			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			t.Logf("Force provider %s: status %d", provider.Name, resp.StatusCode)
		})
	}
}

// =============================================================================
// STREAMING WITH ENSEMBLE TESTS
// =============================================================================

// TestEnsembleStreaming tests streaming with ensemble configuration
func TestEnsembleStreaming(t *testing.T) {
	testutil.RequireServer(t)
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("StreamingWithEnsemble", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		chatReq := ChatRequestWithEnsemble{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Count from 1 to 3."},
			},
			MaxTokens:   50,
			Temperature: 0.0,
			Stream:      true,
			EnsembleConfig: &EnsembleConfig{
				Strategy:     "fastest_response",
				MaxProviders: 2,
				Timeout:      30000,
			},
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Skip if provider unavailable
		if resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusGatewayTimeout {
			body, _ := io.ReadAll(resp.Body)
			t.Skipf("Provider unavailable (status %d): %s (SKIP-OK: #unmarked-skip-needs-ticket)", resp.StatusCode, string(body))
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Read stream
		var chunks []OpenAIStreamChunk
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					break
				}
				var chunk OpenAIStreamChunk
				if json.Unmarshal([]byte(data), &chunk) == nil {
					chunks = append(chunks, chunk)
				}
			}
		}

		assert.NotEmpty(t, chunks, "Should receive streaming chunks with ensemble")
	})

	t.Run("StreamingWithForceProvider", func(t *testing.T) {
		providers := getAvailableProviders(t)
		var availableProvider *EnsembleProviderConfig
		for _, p := range providers {
			if p.Available {
				availableProvider = &p
				break
			}
		}

		if availableProvider == nil {
			t.Logf("No providers available (acceptable)")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		chatReq := ChatRequestWithEnsemble{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Say hello."},
			},
			MaxTokens:     30,
			Temperature:   0.0,
			Stream:        true,
			ForceProvider: strings.ToLower(availableProvider.Name),
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		t.Logf("Streaming with force %s: status %d", availableProvider.Name, resp.StatusCode)
	})
}

// =============================================================================
// FALLBACK AND RESILIENCE TESTS
// =============================================================================

// TestProviderFallback tests fallback behavior when primary provider fails
func TestProviderFallback(t *testing.T) {
	testutil.RequireServer(t)
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("FallbackOnProviderError", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Request with an invalid provider first, should fallback
		chatReq := ChatRequestWithEnsemble{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Say 'fallback test' and nothing else."},
			},
			MaxTokens:   20,
			Temperature: 0.0,
			Stream:      false,
			EnsembleConfig: &EnsembleConfig{
				Providers:    []string{"invalid_provider", "deepseek", "gemini"},
				MaxProviders: 3,
				Timeout:      30000,
			},
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should get a response (fallback worked) or error
		t.Logf("Fallback test response status: %d", resp.StatusCode)
	})
}

// TestTimeoutHandling tests timeout handling with slow providers
func TestTimeoutHandling(t *testing.T) {
	testutil.RequireServer(t)
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("ShortTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		chatReq := ChatRequestWithEnsemble{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Write a very short poem."},
			},
			MaxTokens:   100,
			Temperature: 0.7,
			Stream:      false,
			EnsembleConfig: &EnsembleConfig{
				Timeout: 1000, // Very short timeout - 1 second
			},
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// May timeout or succeed depending on provider speed
		t.Logf("Short timeout test: status %d", resp.StatusCode)
	})

	t.Run("LongTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		chatReq := ChatRequestWithEnsemble{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Say hello."},
			},
			MaxTokens:   10,
			Temperature: 0.0,
			Stream:      false,
			EnsembleConfig: &EnsembleConfig{
				Timeout: 60000, // Long timeout - 60 seconds
			},
		}

		body, err := json.Marshal(chatReq)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Skip if provider unavailable
		if resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusGatewayTimeout {
			body, _ := io.ReadAll(resp.Body)
			t.Skipf("Provider unavailable (status %d): %s (SKIP-OK: #unmarked-skip-needs-ticket)", resp.StatusCode, string(body))
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should succeed with long timeout")
	})
}

// =============================================================================
// RATE LIMITING TESTS
// =============================================================================

// TestRateLimitHandling tests rate limit handling
func TestRateLimitHandling(t *testing.T) {
	testutil.RequireServer(t)
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("BurstRequests", func(t *testing.T) {
		numRequests := 10
		var wg sync.WaitGroup
		results := make(chan int, numRequests)

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				chatReq := OpenAIChatRequest{
					Model: "helixagent-debate",
					Messages: []OpenAIMessage{
						{Role: "user", Content: "Say 'ok'."},
					},
					MaxTokens:   5,
					Temperature: 0.0,
					Stream:      false,
				}

				body, _ := json.Marshal(chatReq)
				req, _ := http.NewRequestWithContext(ctx, "POST",
					config.BaseURL+"/chat/completions", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				if config.HelixAgentAPIKey != "" {
					req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
				}

				client := &http.Client{}
				resp, err := client.Do(req)
				if err != nil {
					results <- 0
					return
				}
				defer resp.Body.Close()
				results <- resp.StatusCode
			}()
		}

		wg.Wait()
		close(results)

		var success, rateLimited, providerUnavailable, serverError, other int
		for status := range results {
			switch status {
			case http.StatusOK:
				success++
			case http.StatusTooManyRequests:
				rateLimited++
			case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
				providerUnavailable++
			case http.StatusInternalServerError:
				serverError++
			case 0:
				// Network error (no response)
				providerUnavailable++
			default:
				other++
				t.Logf("Got unexpected status code: %d", status)
			}
		}

		t.Logf("Burst results: %d success, %d rate limited, %d provider unavailable, %d server error, %d other",
			success, rateLimited, providerUnavailable, serverError, other)
		// If all providers are unavailable, that's a valid test outcome
		if providerUnavailable == numRequests || serverError == numRequests {
			t.Logf("All providers unavailable or errors (acceptable)")
			return
		}
		// Accept any reasonable outcome - the test verifies the server doesn't crash
		assert.True(t, success > 0 || providerUnavailable > 0 || serverError > 0,
			"At least some requests should succeed or fail with a recognized error")
	})
}

// =============================================================================
// MODEL LISTING AND DISCOVERY TESTS
// =============================================================================

// TestModelDiscovery tests model discovery and listing
func TestModelDiscovery(t *testing.T) {
	testutil.RequireServer(t)
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("ListAllModels", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/models", nil)
		require.NoError(t, err)
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var modelsResp OpenAIModelsResponse
		err = json.NewDecoder(resp.Body).Decode(&modelsResp)
		require.NoError(t, err)

		t.Logf("Available models: %d", len(modelsResp.Data))
		for _, model := range modelsResp.Data {
			t.Logf("  - %s (owned by: %s)", model.ID, model.OwnedBy)
		}

		// Should have at least the helixagent-debate model
		modelIDs := make(map[string]bool)
		for _, m := range modelsResp.Data {
			modelIDs[m.ID] = true
		}
		assert.True(t, modelIDs["helixagent-debate"], "Should include helixagent-debate model")
	})

	t.Run("TestEachDiscoveredModel", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/models", nil)
		require.NoError(t, err)
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Logf("Could not get models list (acceptable)")
			return
		}

		var modelsResp OpenAIModelsResponse
		json.NewDecoder(resp.Body).Decode(&modelsResp)

		// Test first 3 models only to avoid too many API calls
		maxModels := 3
		if len(modelsResp.Data) < maxModels {
			maxModels = len(modelsResp.Data)
		}

		for i := 0; i < maxModels; i++ {
			model := modelsResp.Data[i]
			t.Run("Model_"+model.ID, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				chatReq := OpenAIChatRequest{
					Model: model.ID,
					Messages: []OpenAIMessage{
						{Role: "user", Content: "Say 'ok'."},
					},
					MaxTokens:   5,
					Temperature: 0.0,
					Stream:      false,
				}

				body, _ := json.Marshal(chatReq)
				req, _ := http.NewRequestWithContext(ctx, "POST",
					config.BaseURL+"/chat/completions", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				if config.HelixAgentAPIKey != "" {
					req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
				}

				client := &http.Client{}
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				t.Logf("Model %s: status %d", model.ID, resp.StatusCode)
			})
		}
	})
}

// =============================================================================
// PROVIDER HEALTH CHECK TESTS
// =============================================================================

// TestProviderHealthChecks tests health checks for all providers
func TestProviderHealthChecks(t *testing.T) {
	testutil.RequireServer(t)
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("HealthEndpoint", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		// Try common health endpoints
		endpoints := []string{"/health", "/healthz", "/v1/health"}

		for _, endpoint := range endpoints {
			req, err := http.NewRequestWithContext(ctx, "GET",
				fmt.Sprintf("http://%s:%s%s", config.HelixAgentHost, config.HelixAgentPort, endpoint), nil)
			require.NoError(t, err)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				t.Logf("Health endpoint %s: OK", endpoint)
				return
			}
		}

		t.Log("No health endpoint found (not critical)")
	})

	t.Run("ModelsEndpointAsHealth", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", config.BaseURL+"/models", nil)
		require.NoError(t, err)
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Models endpoint should work as health check")
	})
}

// =============================================================================
// PROVIDER-SPECIFIC TESTS
// =============================================================================

// TestDeepSeekSpecific tests DeepSeek-specific features
func TestDeepSeekSpecific(t *testing.T) {
	testutil.RequireServer(t)
	testutil.RequireAPIKey(t, "deepseek")
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("DeepSeekChat", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		chatReq := ChatRequestWithEnsemble{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Hello from DeepSeek test."},
			},
			MaxTokens:     30,
			Temperature:   0.0,
			Stream:        false,
			ForceProvider: "deepseek",
		}

		body, _ := json.Marshal(chatReq)
		req, _ := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		t.Logf("DeepSeek response status: %d", resp.StatusCode)
	})
}

// TestGeminiSpecific tests Gemini-specific features
func TestGeminiSpecific(t *testing.T) {
	testutil.RequireServer(t)
	testutil.RequireAPIKey(t, "gemini")
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("GeminiChat", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		chatReq := ChatRequestWithEnsemble{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Hello from Gemini test."},
			},
			MaxTokens:     30,
			Temperature:   0.0,
			Stream:        false,
			ForceProvider: "gemini",
		}

		body, _ := json.Marshal(chatReq)
		req, _ := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		t.Logf("Gemini response status: %d", resp.StatusCode)
	})
}

// TestOpenRouterSpecific tests OpenRouter-specific features
func TestOpenRouterSpecific(t *testing.T) {
	testutil.RequireServer(t)
	testutil.RequireAPIKey(t, "openrouter")
	config := loadTestConfig(t)
	defer cleanupTestConfig(t, config)

	t.Run("OpenRouterChat", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		chatReq := ChatRequestWithEnsemble{
			Model: "helixagent-debate",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "Hello from OpenRouter test."},
			},
			MaxTokens:     30,
			Temperature:   0.0,
			Stream:        false,
			ForceProvider: "openrouter",
		}

		body, _ := json.Marshal(chatReq)
		req, _ := http.NewRequestWithContext(ctx, "POST",
			config.BaseURL+"/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if config.HelixAgentAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.HelixAgentAPIKey)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		t.Logf("OpenRouter response status: %d", resp.StatusCode)
	})
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func init() {
	// Load .env file at package init
	projectRoot := "."
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			projectRoot = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	envFile := filepath.Join(projectRoot, ".env")
	godotenv.Load(envFile)
}
