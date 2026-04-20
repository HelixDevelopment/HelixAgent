package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInferProviderFromModel tests the dynamic model-to-provider inference
func TestInferProviderFromModel(t *testing.T) {
	testCases := []struct {
		name           string
		modelID        string
		expectedResult string
	}{
		// Claude/Anthropic models
		{"claude_3_5_sonnet", "claude-3-5-sonnet-20241022", "claude"},
		{"claude_3_opus", "claude-3-opus-20240229", "claude"},
		{"claude_instant", "claude-instant-1.2", "claude"},
		{"anthropic_model", "anthropic/some-model", "claude"},

		// OpenAI models
		{"gpt_4", "gpt-4", "openai"},
		{"gpt_4o", "gpt-4o", "openai"},
		{"gpt_4_turbo", "gpt-4-turbo-preview", "openai"},
		{"o1_model", "o1-preview", "openai"},
		{"o1_mini", "o1-mini", "openai"},

		// Gemini models
		{"gemini_pro", "gemini-pro", "gemini"},
		{"gemini_1_5_flash", "gemini-1.5-flash", "gemini"},
		{"gemini_2_0_flash", "gemini-2.0-flash", "gemini"},
		{"palm_model", "text-palm-001", "gemini"},

		// DeepSeek models
		{"deepseek_chat", "deepseek-chat", "deepseek"},
		{"deepseek_coder", "deepseek-coder", "deepseek"},
		{"deepseek_v3", "DeepSeek-V3", "deepseek"},

		// Mistral models
		{"mistral_large", "mistral-large-latest", "mistral"},
		{"mistral_small", "mistral-small-latest", "mistral"},
		{"codestral", "codestral-latest", "mistral"},
		{"mixtral", "mixtral-8x7b-32768", "mistral"},

		// Qwen models
		{"qwen_turbo", "qwen-turbo", "qwen"},
		{"qwen_plus", "qwen-plus", "qwen"},
		{"qwen_2_5", "Qwen2.5-72B-Instruct", "qwen"},

		// OpenRouter format models
		{"openrouter_claude", "anthropic/claude-3.5-sonnet", "claude"},
		{"openrouter_openai", "openai/gpt-4o", "openai"},
		{"openrouter_google", "google/gemini-pro", "gemini"},
		{"openrouter_meta", "meta-llama/llama-3.1-70b", "openrouter"},
		{"openrouter_generic", "some-provider/some-model", "openrouter"},

		// Llama models - should return empty (served by multiple providers)
		{"llama_model", "llama3.2", ""},
		{"llama_70b", "llama-3.1-70b-versatile", ""},

		// Provider-specific indicators
		{"groq_indicator", "groq-llama", "groq"},
		{"cerebras_indicator", "cerebras-model", "cerebras"},

		// Unknown models
		{"unknown_model", "some-random-model", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := inferProviderFromModel(tc.modelID)
			assert.Equal(t, tc.expectedResult, result, "Model %s should map to provider %s", tc.modelID, tc.expectedResult)
		})
	}
}

// TestDynamicScoringNoHardcodedValues tests that scoring is truly dynamic
func TestDynamicScoringNoHardcodedValues(t *testing.T) {
	t.Run("all_providers_start_with_neutral_baseline", func(t *testing.T) {
		// CRITICAL: All providers should start with the same baseline score
		// This ensures no provider is favored by hardcoded values
		providers := []string{
			"claude", "openai", "gemini", "deepseek", "mistral",
			"qwen", "groq", "cerebras", "openrouter", "ollama",
			"unknown_provider_xyz",
		}

		for _, provider := range providers {
			score := getBaseScoreForProvider(provider)
			assert.Equal(t, 5.0, score, "Provider %s should have neutral baseline score of 5.0", provider)
		}
	})

	t.Run("differentiation_comes_from_verification", func(t *testing.T) {
		// The baseline is the same, differentiation comes from:
		// 1. LLMsVerifier scores
		// 2. Response time during verification
		// 3. Capabilities detected
		baseScore := getBaseScoreForProvider("any-provider")
		assert.Equal(t, 5.0, baseScore, "All providers have same baseline")
	})
}

// TestLLMsVerifierScoreAdapterCreation tests adapter creation
func TestLLMsVerifierScoreAdapterCreation(t *testing.T) {
	t.Run("creates_with_nil_services", func(t *testing.T) {
		adapter := NewLLMsVerifierScoreAdapter(nil, nil, nil)
		require.NotNil(t, adapter)
		assert.NotNil(t, adapter.providerScores)
		assert.NotNil(t, adapter.modelScores)
	})

	t.Run("creates_with_custom_logger", func(t *testing.T) {
		log := logrus.New()
		adapter := NewLLMsVerifierScoreAdapter(nil, nil, log)
		require.NotNil(t, adapter)
		assert.Equal(t, log, adapter.log)
	})
}

// TestScoreAdapterGetProviderScore tests dynamic score retrieval
func TestScoreAdapterGetProviderScore(t *testing.T) {
	adapter := NewLLMsVerifierScoreAdapter(nil, nil, nil)

	t.Run("returns_false_for_unknown_provider", func(t *testing.T) {
		score, found := adapter.GetProviderScore("unknown-provider")
		assert.False(t, found)
		assert.Equal(t, 0.0, score)
	})

	t.Run("returns_score_after_update", func(t *testing.T) {
		adapter.UpdateScore("test-provider", "test-model", 8.5)

		score, found := adapter.GetProviderScore("test-provider")
		assert.True(t, found)
		assert.Equal(t, 8.5, score)
	})

	t.Run("normalizes_scores_above_10", func(t *testing.T) {
		// LLMsVerifier uses 0-100 scale, adapter normalizes to 0-10
		adapter.providerScores.Put("high-score-provider", 85.0)

		score, found := adapter.GetProviderScore("high-score-provider")
		assert.True(t, found)
		assert.Equal(t, 8.5, score) // 85/10 = 8.5
	})
}

// TestScoreAdapterGetModelScore tests model-specific score retrieval
func TestScoreAdapterGetModelScore(t *testing.T) {
	adapter := NewLLMsVerifierScoreAdapter(nil, nil, nil)

	t.Run("returns_false_for_unknown_model", func(t *testing.T) {
		score, found := adapter.GetModelScore("unknown-model-xyz")
		assert.False(t, found)
		assert.Equal(t, 0.0, score)
	})

	t.Run("returns_score_after_update", func(t *testing.T) {
		adapter.UpdateScore("provider", "specific-model", 9.2)

		score, found := adapter.GetModelScore("specific-model")
		assert.True(t, found)
		assert.Equal(t, 9.2, score)
	})
}

// TestScoreAdapterUpdateScore tests score updates
func TestScoreAdapterUpdateScore(t *testing.T) {
	adapter := NewLLMsVerifierScoreAdapter(nil, nil, nil)

	t.Run("updates_both_model_and_provider_scores", func(t *testing.T) {
		adapter.UpdateScore("test-provider", "test-model", 7.5)

		modelScore, modelFound := adapter.GetModelScore("test-model")
		providerScore, providerFound := adapter.GetProviderScore("test-provider")

		assert.True(t, modelFound)
		assert.True(t, providerFound)
		assert.Equal(t, 7.5, modelScore)
		assert.Equal(t, 7.5, providerScore)
	})

	t.Run("keeps_higher_provider_score", func(t *testing.T) {
		adapter.UpdateScore("multi-model-provider", "model-1", 8.0)
		adapter.UpdateScore("multi-model-provider", "model-2", 9.0)
		adapter.UpdateScore("multi-model-provider", "model-3", 7.0)

		score, found := adapter.GetProviderScore("multi-model-provider")
		assert.True(t, found)
		assert.Equal(t, 9.0, score, "Provider score should be the highest model score")
	})
}

// TestGetAllProviderScores tests bulk score retrieval
func TestGetAllProviderScores(t *testing.T) {
	adapter := NewLLMsVerifierScoreAdapter(nil, nil, nil)

	adapter.UpdateScore("provider-a", "model-a", 8.0)
	adapter.UpdateScore("provider-b", "model-b", 7.5)
	adapter.UpdateScore("provider-c", "model-c", 9.0)

	scores := adapter.GetAllProviderScores()

	assert.Len(t, scores, 3)
	assert.Equal(t, 8.0, scores["provider-a"])
	assert.Equal(t, 7.5, scores["provider-b"])
	assert.Equal(t, 9.0, scores["provider-c"])
}

// TestGetBestProvider tests best provider selection
func TestGetBestProvider(t *testing.T) {
	adapter := NewLLMsVerifierScoreAdapter(nil, nil, nil)

	t.Run("returns_empty_when_no_scores", func(t *testing.T) {
		provider, score := adapter.GetBestProvider()
		assert.Empty(t, provider)
		assert.Equal(t, 0.0, score)
	})

	t.Run("returns_highest_scoring_provider", func(t *testing.T) {
		adapter.UpdateScore("provider-low", "model-1", 6.0)
		adapter.UpdateScore("provider-high", "model-2", 9.5)
		adapter.UpdateScore("provider-mid", "model-3", 7.5)

		provider, score := adapter.GetBestProvider()
		assert.Equal(t, "provider-high", provider)
		assert.Equal(t, 9.5, score)
	})
}

// TestRefreshScores tests score refresh mechanism
func TestRefreshScores(t *testing.T) {
	adapter := NewLLMsVerifierScoreAdapter(nil, nil, nil)

	t.Run("respects_refresh_interval", func(t *testing.T) {
		adapter.lastRefresh = time.Now()

		err := adapter.RefreshScores(context.Background())
		assert.NoError(t, err)
		// No actual refresh happens because interval not elapsed
	})

	t.Run("allows_refresh_after_interval", func(t *testing.T) {
		adapter.lastRefresh = time.Now().Add(-10 * time.Minute)
		adapter.refreshInterval = 5 * time.Minute

		err := adapter.RefreshScores(context.Background())
		assert.NoError(t, err)
	})
}

// TestKnownModelsFromVerifications tests dynamic model discovery
func TestKnownModelsFromVerifications(t *testing.T) {
	adapter := NewLLMsVerifierScoreAdapter(nil, nil, nil)

	t.Run("returns_nil_without_verification_service", func(t *testing.T) {
		models := adapter.getKnownModelsFromVerifications()
		assert.Nil(t, models)
	})
}

// TestDynamicModelInference validates that model inference works correctly
func TestDynamicModelInference(t *testing.T) {
	t.Run("inference_is_case_insensitive", func(t *testing.T) {
		tests := []struct {
			model    string
			expected string
		}{
			{"CLAUDE-3-SONNET", "claude"},
			{"Claude-3-Opus", "claude"},
			{"GPT-4", "openai"},
			{"Gpt-4o", "openai"},
			{"GEMINI-PRO", "gemini"},
			{"DeepSeek-Chat", "deepseek"},
		}

		for _, tc := range tests {
			result := inferProviderFromModel(tc.model)
			assert.Equal(t, tc.expected, result, "Model %s should infer provider %s", tc.model, tc.expected)
		}
	})

	t.Run("handles_versioned_model_names", func(t *testing.T) {
		versionedModels := []struct {
			model    string
			expected string
		}{
			{"claude-3-5-sonnet-20241022", "claude"},
			{"claude-3-opus-20240229", "claude"},
			{"gpt-4-turbo-2024-04-09", "openai"},
			{"gemini-1.5-flash-001", "gemini"},
			{"mistral-large-2402", "mistral"},
		}

		for _, tc := range versionedModels {
			result := inferProviderFromModel(tc.model)
			assert.Equal(t, tc.expected, result, "Versioned model %s should infer provider %s", tc.model, tc.expected)
		}
	})
}

// TestProviderOrderingIsDynamic validates that provider ordering is dynamic
func TestProviderOrderingIsDynamic(t *testing.T) {
	t.Run("ordering_based_on_scores_not_hardcoded", func(t *testing.T) {
		// This test validates that provider ordering comes from scores
		// and not from a hardcoded list

		adapter := NewLLMsVerifierScoreAdapter(nil, nil, nil)

		// Set up scores in non-alphabetical, non-tier order
		adapter.UpdateScore("ollama", "llama", 9.9)      // Usually lowest priority
		adapter.UpdateScore("openrouter", "router", 1.0) // Usually high priority
		adapter.UpdateScore("claude", "sonnet", 5.0)     // Usually top priority

		// Get best provider - should be based on actual score
		best, score := adapter.GetBestProvider()
		assert.Equal(t, "ollama", best, "Best provider should be based on score, not hardcoded priority")
		assert.Equal(t, 9.9, score)

		// Validate all scores are retrievable
		allScores := adapter.GetAllProviderScores()
		assert.Equal(t, 9.9, allScores["ollama"])
		assert.Equal(t, 1.0, allScores["openrouter"])
		assert.Equal(t, 5.0, allScores["claude"])
	})
}

// Ensure the adapter implements the interface
func TestLLMsVerifierScoreAdapterInterface(t *testing.T) {
	var _ LLMsVerifierScoreProvider = (*LLMsVerifierScoreAdapter)(nil)
}

// =============================================================================
// Additional Tests for Uncovered Functions
// =============================================================================

func TestLLMsVerifierScoreAdapter_GetVerificationService(t *testing.T) {
	adapter := NewLLMsVerifierScoreAdapter(nil, nil, nil)

	// Without verification service, should return nil
	svc := adapter.GetVerificationService()
	assert.Nil(t, svc)
}

func TestLLMsVerifierScoreAdapter_GetScoringService(t *testing.T) {
	adapter := NewLLMsVerifierScoreAdapter(nil, nil, nil)

	// Without scoring service, should return nil
	svc := adapter.GetScoringService()
	assert.Nil(t, svc)
}

func TestLLMsVerifierScoreAdapter_IsInitialized(t *testing.T) {
	t.Run("not initialized without services", func(t *testing.T) {
		adapter := NewLLMsVerifierScoreAdapter(nil, nil, nil)
		assert.False(t, adapter.IsInitialized())
	})
}
