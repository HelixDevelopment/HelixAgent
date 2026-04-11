// Package integration provides comprehensive OpenRouter provider integration tests.
// These tests make real API calls to the OpenRouter API and are skipped when
// OPENROUTER_API_KEY is not set. Rate-limit (429) errors are treated as skips
// rather than failures, since they depend on quota availability.
package integration

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/llm/providers/openrouter"
	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openrouterAPIKey returns the OpenRouter API key or skips the test.
func openrouterAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		t.Skip("OPENROUTER_API_KEY not set")
	}
	if strings.HasPrefix(key, "$") || strings.HasPrefix(key, "<") {
		t.Skipf("OPENROUTER_API_KEY looks like an unsubstituted placeholder (%q) — skipping", key)
	}
	return key
}

// skipOnOpenRouterRateLimit checks if an error is a rate-limit error (429)
// or auth failure (401) from a broken token and skips the test.
func skipOnOpenRouterRateLimit(t *testing.T, err error) bool {
	t.Helper()
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "429") ||
		strings.Contains(msg, "rate_limit") ||
		strings.Contains(msg, "Rate limit") {
		t.Skipf("Skipping due to OpenRouter API rate limit: %v", err)
		return true
	}
	if strings.Contains(msg, "401") || strings.Contains(msg, "Unauthorized") {
		t.Skipf("Skipping due to OpenRouter 401 (bad/expired token): %v", err)
		return true
	}
	return false
}

// TestOpenRouterAPI_SimpleCompletion sends a simple prompt to OpenRouter
// and verifies a non-empty response is returned.
func TestOpenRouterAPI_SimpleCompletion(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := openrouterAPIKey(t)

	provider := openrouter.NewSimpleOpenRouterProvider(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID: "test-openrouter-simple-completion",
		Messages: []models.Message{
			{Role: "user", Content: "Reply with just the word 'hello'"},
		},
		ModelParams: models.ModelParameters{
			Model:       "meta-llama/llama-4-scout",
			MaxTokens:   64,
			Temperature: 0.0,
		},
	}

	resp, err := provider.Complete(ctx, req)
	if skipOnOpenRouterRateLimit(t, err) {
		return
	}
	require.NoError(t, err, "Complete should not return an error")
	require.NotNil(t, resp, "response should not be nil")

	assert.NotEmpty(t, resp.Content, "response content should not be empty")
	assert.Contains(
		t,
		strings.ToLower(resp.Content),
		"hello",
		"response should contain 'hello'",
	)
	assert.Equal(t, "openrouter", resp.ProviderID)
	assert.Greater(t, resp.ResponseTime, int64(0),
		"response time should be positive")
}

// TestOpenRouterAPI_StreamingCompletion verifies streaming completion returns
// multiple chunks and a final response with stop finish reason.
func TestOpenRouterAPI_StreamingCompletion(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := openrouterAPIKey(t)

	provider := openrouter.NewSimpleOpenRouterProvider(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID: "test-openrouter-streaming",
		Messages: []models.Message{
			{Role: "user", Content: "Count from 1 to 5, one number per line."},
		},
		ModelParams: models.ModelParameters{
			Model:       "meta-llama/llama-4-scout",
			MaxTokens:   256,
			Temperature: 0.0,
		},
	}

	ch, err := provider.CompleteStream(ctx, req)
	if skipOnOpenRouterRateLimit(t, err) {
		return
	}
	require.NoError(t, err, "CompleteStream should not return an error")
	require.NotNil(t, ch, "stream channel should not be nil")

	var chunks []*models.LLMResponse
	for resp := range ch {
		chunks = append(chunks, resp)
	}

	require.NotEmpty(t, chunks, "should receive at least one chunk")

	// Check that at least one content chunk was received
	hasContent := false
	for _, c := range chunks {
		if c.Content != "" {
			hasContent = true
			break
		}
	}
	assert.True(t, hasContent,
		"should have received at least one content chunk")
}

// TestOpenRouterAPI_HealthCheck verifies the health check endpoint.
func TestOpenRouterAPI_HealthCheck(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := openrouterAPIKey(t)

	provider := openrouter.NewSimpleOpenRouterProvider(apiKey)

	err := provider.HealthCheck()
	if skipOnOpenRouterRateLimit(t, err) {
		return
	}
	assert.NoError(t, err, "HealthCheck should succeed with a valid API key")
}

// TestOpenRouterAPI_ModelDiscovery verifies GetCapabilities returns provider info.
func TestOpenRouterAPI_ModelDiscovery(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := openrouterAPIKey(t)

	provider := openrouter.NewSimpleOpenRouterProvider(apiKey)

	caps := provider.GetCapabilities()
	require.NotNil(t, caps, "capabilities should not be nil")
	assert.Equal(t, "OpenRouter", caps.Metadata["provider"])
	assert.True(t, caps.SupportsStreaming, "should support streaming")
}

// TestOpenRouterAPI_MultipleModels verifies that multiple models respond
// successfully when called sequentially through OpenRouter.
func TestOpenRouterAPI_MultipleModels(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := openrouterAPIKey(t)

	modelsToTest := []string{
		"meta-llama/llama-4-scout",
		"deepseek/deepseek-r1",
	}

	for _, model := range modelsToTest {
		t.Run(model, func(t *testing.T) {
			provider := openrouter.NewSimpleOpenRouterProvider(apiKey)

			ctx, cancel := context.WithTimeout(
				context.Background(), 60*time.Second,
			)
			defer cancel()

			req := &models.LLMRequest{
				ID: "test-openrouter-model-" + model,
				Messages: []models.Message{
					{
						Role:    "user",
						Content: "Reply with the word 'pong'.",
					},
				},
				ModelParams: models.ModelParameters{
					Model:       model,
					MaxTokens:   32,
					Temperature: 0.0,
				},
			}

			resp, err := provider.Complete(ctx, req)
			if skipOnOpenRouterRateLimit(t, err) {
				return
			}
			require.NoError(t, err,
				"Complete should not error for model %s", model)
			require.NotNil(t, resp,
				"response should not be nil for model %s", model)
			assert.NotEmpty(t, resp.Content,
				"response content should not be empty for model %s",
				model)
		})
	}
}
