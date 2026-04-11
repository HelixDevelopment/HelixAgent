package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	verifier "dev.helix.agent/internal/verifier"
)

func TestNewExtendedProvidersAdapter(t *testing.T) {
	t.Parallel()
	t.Run("creates adapter with default config", func(t *testing.T) {
		adapter := NewExtendedProvidersAdapter(nil)
		assert.NotNil(t, adapter)
		assert.NotNil(t, adapter.config)
		assert.Equal(t, 30*time.Second, adapter.config.VerificationTimeout)
	})

	t.Run("creates adapter with custom config", func(t *testing.T) {
		t.Parallel()
		config := &ExtendedProviderConfig{
			VerificationTimeout:        60 * time.Second,
			MaxConcurrentVerifications: 10,
		}
		adapter := NewExtendedProvidersAdapter(config)
		assert.NotNil(t, adapter)
		assert.Equal(t, 60*time.Second, adapter.config.VerificationTimeout)
		assert.Equal(t, 10, adapter.config.MaxConcurrentVerifications)
	})
}

func TestExtendedProvidersAdapter_VerifyProvider(t *testing.T) {
	t.Parallel()
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "test-id",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "4"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`))
	}))
	defer server.Close()

	adapter := NewExtendedProvidersAdapter(DefaultExtendedProviderConfig())

	t.Run("verifies provider successfully", func(t *testing.T) {
		t.Parallel()
		req := &ProviderVerificationRequest{
			ProviderID:   "test-provider",
			ProviderName: "Test Provider",
			APIKey:       "test-key",
			BaseURL:      server.URL,
			Models:       []string{"test-model"},
			AuthType:     verifier.AuthTypeAPIKey,
			Tier:         2,
			Priority:     3,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		provider, err := adapter.VerifyProvider(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.True(t, provider.Verified)
		assert.Equal(t, "test-provider", provider.ID)
		assert.Len(t, provider.Models, 1)
	})

	t.Run("handles API error gracefully", func(t *testing.T) {
		t.Parallel()
		errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "internal server error"}`))
		}))
		defer errorServer.Close()

		req := &ProviderVerificationRequest{
			ProviderID:   "error-provider",
			ProviderName: "Error Provider",
			APIKey:       "test-key",
			BaseURL:      errorServer.URL,
			Models:       []string{"test-model"},
			AuthType:     verifier.AuthTypeAPIKey,
			Tier:         2,
			Priority:     3,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		provider, err := adapter.VerifyProvider(ctx, req)
		require.NoError(t, err) // Provider verification doesn't fail, but provider is not verified
		assert.NotNil(t, provider)
		assert.False(t, provider.Verified)
		assert.Equal(t, verifier.StatusFailed, provider.Status)
	})
}

func TestExtendedProvidersAdapter_CalculateProviderScore(t *testing.T) {
	t.Parallel()
	adapter := NewExtendedProvidersAdapter(nil)

	t.Run("returns zero for no models", func(t *testing.T) {
		t.Parallel()
		req := &ProviderVerificationRequest{Tier: 2}
		score := adapter.calculateProviderScore(nil, req)
		assert.Equal(t, 0.0, score)
	})

	t.Run("calculates score based on models", func(t *testing.T) {
		t.Parallel()
		models := []verifier.UnifiedModel{
			{ID: "model-1", Score: 8.0},
			{ID: "model-2", Score: 9.0},
		}
		req := &ProviderVerificationRequest{Tier: 2}
		score := adapter.calculateProviderScore(models, req)
		assert.Greater(t, score, 8.0)
	})

	t.Run("applies tier bonus", func(t *testing.T) {
		t.Parallel()
		models := []verifier.UnifiedModel{
			{ID: "model-1", Score: 7.0},
		}

		tier1Req := &ProviderVerificationRequest{Tier: 1}
		tier5Req := &ProviderVerificationRequest{Tier: 5}

		tier1Score := adapter.calculateProviderScore(models, tier1Req)
		tier5Score := adapter.calculateProviderScore(models, tier5Req)

		assert.Greater(t, tier1Score, tier5Score)
	})
}

func TestExtendedProvidersAdapter_CalculateModelScore(t *testing.T) {
	t.Parallel()
	adapter := NewExtendedProvidersAdapter(nil)

	t.Run("rewards low latency", func(t *testing.T) {
		t.Parallel()
		req := &ProviderVerificationRequest{Tier: 2}
		testResults := map[string]bool{"basic_completion": true}

		fastScore := adapter.calculateModelScore(200*time.Millisecond, testResults, req)
		slowScore := adapter.calculateModelScore(5*time.Second, testResults, req)

		assert.Greater(t, fastScore, slowScore)
	})

	t.Run("rewards passing tests", func(t *testing.T) {
		t.Parallel()
		req := &ProviderVerificationRequest{Tier: 2}

		moreTests := map[string]bool{
			"basic_completion": true,
			"code_visibility":  true,
			"json_mode":        true,
		}
		fewerTests := map[string]bool{
			"basic_completion": true,
		}

		moreScore := adapter.calculateModelScore(500*time.Millisecond, moreTests, req)
		fewerScore := adapter.calculateModelScore(500*time.Millisecond, fewerTests, req)

		assert.Greater(t, moreScore, fewerScore)
	})
}

func TestExtendedProvidersAdapter_InferCapabilities(t *testing.T) {
	t.Parallel()
	adapter := NewExtendedProvidersAdapter(nil)

	t.Run("infers vision capability", func(t *testing.T) {
		t.Parallel()
		caps := adapter.inferCapabilities("grok", "grok-2-vision")
		assert.Contains(t, caps, "vision")
	})

	t.Run("infers function calling", func(t *testing.T) {
		t.Parallel()
		caps := adapter.inferCapabilities("together", "some-model")
		assert.Contains(t, caps, "function_calling")
		assert.Contains(t, caps, "tools")
	})

	t.Run("infers web search for perplexity", func(t *testing.T) {
		t.Parallel()
		caps := adapter.inferCapabilities("perplexity", "sonar-online")
		assert.Contains(t, caps, "web_search")
		assert.Contains(t, caps, "realtime_info")
	})

	t.Run("infers code generation", func(t *testing.T) {
		t.Parallel()
		caps := adapter.inferCapabilities("any", "codestral-latest")
		assert.Contains(t, caps, "code_generation")
	})

	t.Run("always includes base capabilities", func(t *testing.T) {
		t.Parallel()
		caps := adapter.inferCapabilities("any", "any-model")
		assert.Contains(t, caps, "text_completion")
		assert.Contains(t, caps, "chat")
		assert.Contains(t, caps, "streaming")
	})
}

func TestExtendedProvidersAdapter_GetVerifiedProviders(t *testing.T) {
	t.Parallel()
	adapter := NewExtendedProvidersAdapter(nil)

	t.Run("returns empty map initially", func(t *testing.T) {
		t.Parallel()
		providers := adapter.GetVerifiedProviders()
		assert.Empty(t, providers)
	})
}

func TestGetModelDisplayNameExt(t *testing.T) {
	t.Parallel()
	t.Run("removes provider prefix", func(t *testing.T) {
		name := getModelDisplayNameExt("meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo")
		assert.NotContains(t, name, "meta-llama/")
	})

	t.Run("cleans up suffixes", func(t *testing.T) {
		t.Parallel()
		name := getModelDisplayNameExt("Model-Instruct-Turbo")
		assert.Contains(t, name, "Instruct")
		assert.NotContains(t, name, "-Turbo")
	})
}

func TestCountPassedTests(t *testing.T) {
	t.Parallel()
	t.Run("counts correctly", func(t *testing.T) {
		results := map[string]bool{
			"test1": true,
			"test2": false,
			"test3": true,
			"test4": true,
		}
		assert.Equal(t, 3, countPassedTests(results))
	})

	t.Run("handles empty map", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, 0, countPassedTests(map[string]bool{}))
	})

	t.Run("handles all false", func(t *testing.T) {
		t.Parallel()
		results := map[string]bool{
			"test1": false,
			"test2": false,
		}
		assert.Equal(t, 0, countPassedTests(results))
	})

	t.Run("handles all true", func(t *testing.T) {
		t.Parallel()
		results := map[string]bool{
			"test1": true,
			"test2": true,
		}
		assert.Equal(t, 2, countPassedTests(results))
	})
}

func TestDefaultExtendedProviderConfig(t *testing.T) {
	t.Parallel()
	config := DefaultExtendedProviderConfig()

	assert.Equal(t, 30*time.Second, config.VerificationTimeout)
	assert.Equal(t, 10*time.Second, config.HealthCheckTimeout)
	assert.Equal(t, 5, config.MaxConcurrentVerifications)
	assert.Equal(t, 2, config.RetryAttempts)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
	assert.Equal(t, 5.0, config.MinScoreThreshold)
}
