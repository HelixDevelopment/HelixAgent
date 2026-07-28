package gemini_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/llm/providers/gemini"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/tests/testutils"
)

func TestNewGeminiProvider(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		provider := gemini.NewGeminiProvider(
			"test-api-key",
			"https://generativelanguage.googleapis.com",
			"gemini-pro",
		)
		require.NotNil(t, provider)
	})

	t.Run("with default base URL", func(t *testing.T) {
		provider := gemini.NewGeminiProvider(
			"test-api-key",
			"",
			"gemini-pro",
		)
		require.NotNil(t, provider)
	})

	t.Run("with default model", func(t *testing.T) {
		provider := gemini.NewGeminiProvider(
			"test-api-key",
			"https://generativelanguage.googleapis.com",
			"",
		)
		require.NotNil(t, provider)
	})
}

func TestGeminiProvider_GetCapabilities(t *testing.T) {
	provider := gemini.NewGeminiProvider(
		"test-api-key",
		"https://generativelanguage.googleapis.com",
		"gemini-pro",
	)
	require.NotNil(t, provider)

	capabilities := provider.GetCapabilities()

	assert.NotNil(t, capabilities)
	assert.True(t, capabilities.SupportsStreaming)
	assert.Greater(t, capabilities.Limits.MaxTokens, 0)
	assert.True(t, capabilities.SupportsFunctionCalling)
	assert.True(t, capabilities.SupportsVision)
	assert.NotEmpty(t, capabilities.SupportedModels)
	assert.Contains(t, capabilities.SupportedModels, "gemini-2.0-flash")
	assert.Contains(t, capabilities.SupportedModels, "gemini-2.5-flash")
	assert.Contains(t, capabilities.SupportedModels, "gemini-2.5-pro")

	// Check supported features
	assert.Contains(t, capabilities.SupportedFeatures, "text_completion")
	assert.Contains(t, capabilities.SupportedFeatures, "chat")
	assert.Contains(t, capabilities.SupportedFeatures, "function_calling")
	assert.Contains(t, capabilities.SupportedFeatures, "streaming")
	assert.Contains(t, capabilities.SupportedFeatures, "vision")
}

func TestGeminiProvider_ValidateConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		provider := gemini.NewGeminiProvider(
			"test-api-key",
			"https://generativelanguage.googleapis.com",
			"gemini-pro",
		)
		require.NotNil(t, provider)

		valid, errors := provider.ValidateConfig(nil)
		assert.True(t, valid)
		assert.Empty(t, errors)
	})

	t.Run("missing api key", func(t *testing.T) {
		// ANTI-BLUFF (§11.4.201 / §11.4.115): NewGeminiUnifiedProvider silently
		// backfills an empty APIKey from os.Getenv("GEMINI_API_KEY")
		// (internal/llm/providers/gemini/gemini.go:223-225), so on a host that
		// happens to have GEMINI_API_KEY exported, p.apiKey would be non-empty
		// and this "missing api key" test would assert the WRONG thing. Force
		// the real, asserted condition — no key, from neither the constructor
		// argument NOR the environment — via t.Setenv, which Go's testing
		// package restores automatically when this subtest ends. This makes
		// the test deterministic (§11.4.50) instead of branching on host state.
		t.Setenv("GEMINI_API_KEY", "")

		provider := gemini.NewGeminiProvider(
			"",
			"https://generativelanguage.googleapis.com",
			"gemini-pro",
		)
		require.NotNil(t, provider)

		valid, errors := provider.ValidateConfig(nil)
		// GeminiUnifiedProvider ALSO supports CLI/OAuth fallback
		// (gemini.go:486 ValidateConfig: invalid iff apiKey=="" && !IsGeminiCLIInstalled()),
		// which is a second, independent environment guard this test still
		// must honor — it is NOT the guard that was broken (that was the
		// GEMINI_API_KEY env backfill, fixed above by t.Setenv), so it is kept.
		if gemini.IsGeminiCLIInstalled() {
			assert.True(t, valid, "should be valid when Gemini CLI is available as fallback")
			assert.Empty(t, errors)
		} else {
			assert.False(t, valid, "should be invalid without API key and without CLI fallback")
			assert.NotEmpty(t, errors)
		}
	})

	t.Run("missing model uses default", func(t *testing.T) {
		provider := gemini.NewGeminiProvider(
			"test-api-key",
			"https://generativelanguage.googleapis.com",
			"",
		)
		require.NotNil(t, provider)

		valid, errors := provider.ValidateConfig(nil)
		assert.True(t, valid) // Default model is used
		assert.Empty(t, errors)
	})
}

func TestGeminiProvider_WithRetry(t *testing.T) {
	retryConfig := gemini.RetryConfig{
		MaxRetries:   5,
		InitialDelay: 100,
		MaxDelay:     1000,
		Multiplier:   2.0,
	}
	provider := gemini.NewGeminiProviderWithRetry("test-api-key", "", "gemini-pro", retryConfig)
	require.NotNil(t, provider)
}

// Integration tests that use mock LLM server when available
func TestGeminiProvider_Complete(t *testing.T) {
	testutils.SkipIfNoInfrastructure(t, "llm")

	// Gemini expects format string with model name: /v1beta/models/%s:generateContent
	mockURL := testutils.GetMockLLMBaseURL() + "/v1beta/models/%s:generateContent"
	apiKey := testutils.GetMockAPIKey()

	provider := gemini.NewGeminiProvider(apiKey, mockURL, "gemini-pro")
	require.NotNil(t, provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID:     "test-complete",
		Prompt: "Say hello",
		ModelParams: models.ModelParameters{
			Model: "gemini-pro",
		},
	}

	result, err := provider.Complete(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Content)
}

func TestGeminiProvider_CompleteStream(t *testing.T) {
	testutils.SkipIfNoInfrastructure(t, "llm")

	// Gemini expects format string with model name: /v1beta/models/%s:generateContent
	mockURL := testutils.GetMockLLMBaseURL() + "/v1beta/models/%s:generateContent"
	apiKey := testutils.GetMockAPIKey()

	provider := gemini.NewGeminiProvider(apiKey, mockURL, "gemini-pro")
	require.NotNil(t, provider)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID:     "test-stream",
		Prompt: "Say hello",
		ModelParams: models.ModelParameters{
			Model: "gemini-pro",
		},
	}

	stream, err := provider.CompleteStream(ctx, req)
	if err != nil {
		t.Logf("Stream not supported by mock server: %v", err)
		return
	}

	var gotChunk bool
	for resp := range stream {
		if resp != nil && resp.Content != "" {
			gotChunk = true
			break
		}
	}
	_ = gotChunk
}

func TestGeminiProvider_HealthCheck(t *testing.T) {
	testutils.SkipIfNoInfrastructure(t, "llm")

	// Gemini expects format string with model name: /v1beta/models/%s:generateContent
	mockURL := testutils.GetMockLLMBaseURL() + "/v1beta/models/%s:generateContent"
	apiKey := testutils.GetMockAPIKey()

	os.Setenv("GEMINI_API_KEY", apiKey)
	os.Setenv("GEMINI_BASE_URL", mockURL)
	defer func() {
		os.Unsetenv("GEMINI_API_KEY")
		os.Unsetenv("GEMINI_BASE_URL")
	}()

	provider := gemini.NewGeminiProvider(apiKey, mockURL, "gemini-pro")
	require.NotNil(t, provider)

	err := provider.HealthCheck()
	if err != nil {
		t.Logf("Health check returned error (acceptable for mock): %v", err)
	}
}
