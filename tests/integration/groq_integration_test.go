// Package integration provides comprehensive Groq provider integration tests.
// These tests make real API calls to the Groq API and are skipped when
// GROQ_API_KEY is not set. Rate-limit (429) errors are treated as skips
// rather than failures, since they depend on quota availability.
package integration

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/llm/providers/groq"
	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groqAPIKey returns the Groq API key or skips the test.
func groqAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		t.Skip("GROQ_API_KEY not set")  // SKIP-OK: #requires-upstream-key
	}
	return key
}

// skipOnGroqRateLimit checks if an error is a Groq rate-limit error (429) and
// skips the test if so. Returns true when the test should be skipped.
func skipOnGroqRateLimit(t *testing.T, err error) bool {
	t.Helper()
	if err != nil && (strings.Contains(err.Error(), "429") ||
		strings.Contains(err.Error(), "rate_limit") ||
		strings.Contains(err.Error(), "Rate limit")) {
		t.Skipf("Skipping due to Groq API rate limit: %v", err)
		return true
	}
	return false
}

// TestGroqAPI_SimpleCompletion sends a simple prompt to the Groq API and
// verifies a non-empty response is returned.
func TestGroqAPI_SimpleCompletion(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := groqAPIKey(t)

	provider := groq.NewProvider(apiKey, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID: "test-groq-simple-completion",
		Messages: []models.Message{
			{Role: "user", Content: "Reply with just the word 'hello'"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   64,
			Temperature: 0.0,
		},
	}

	resp, err := provider.Complete(ctx, req)
	if skipOnGroqRateLimit(t, err) {
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
	assert.Equal(t, "groq", resp.ProviderID)
	assert.Equal(t, "Groq", resp.ProviderName)
	assert.Greater(t, resp.ResponseTime, int64(0),
		"response time should be positive")
}

// TestGroqAPI_StreamingCompletion verifies streaming completion returns
// multiple chunks and a final response with stop finish reason.
func TestGroqAPI_StreamingCompletion(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := groqAPIKey(t)

	provider := groq.NewProvider(apiKey, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID: "test-groq-streaming",
		Messages: []models.Message{
			{Role: "user", Content: "Count from 1 to 5, one number per line."},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   256,
			Temperature: 0.0,
		},
	}

	ch, err := provider.CompleteStream(ctx, req)
	if skipOnGroqRateLimit(t, err) {
		return
	}
	require.NoError(t, err, "CompleteStream should not return an error")
	require.NotNil(t, ch, "stream channel should not be nil")

	var chunks []*models.LLMResponse
	for resp := range ch {
		chunks = append(chunks, resp)
	}

	require.NotEmpty(t, chunks, "should receive at least one chunk")

	// The last chunk should have finish_reason "stop"
	lastChunk := chunks[len(chunks)-1]
	assert.Equal(t, "stop", lastChunk.FinishReason,
		"last chunk should have stop finish reason")

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

// TestGroqAPI_HealthCheck verifies the health check endpoint returns no error.
func TestGroqAPI_HealthCheck(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := groqAPIKey(t)

	provider := groq.NewProvider(apiKey, "", "")

	err := provider.HealthCheck()
	if skipOnGroqRateLimit(t, err) {
		return
	}
	assert.NoError(t, err, "HealthCheck should succeed with a valid API key")
}

// TestGroqAPI_ModelDiscovery verifies GetCapabilities returns a non-empty
// list of supported models.
func TestGroqAPI_ModelDiscovery(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := groqAPIKey(t)

	provider := groq.NewProvider(apiKey, "", "")

	caps := provider.GetCapabilities()
	require.NotNil(t, caps, "capabilities should not be nil")
	assert.NotEmpty(t, caps.SupportedModels,
		"supported models list should not be empty")
	assert.True(t, caps.SupportsStreaming, "should support streaming")
	assert.True(t, caps.SupportsTools, "should support tools")
}

// TestGroqAPI_MultipleModels verifies that multiple Groq models respond
// successfully when called sequentially.
func TestGroqAPI_MultipleModels(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := groqAPIKey(t)

	modelsToTest := []string{
		"llama-3.3-70b-versatile",
		"llama-3.1-8b-instant",
	}

	for _, model := range modelsToTest {
		t.Run(model, func(t *testing.T) {
			provider := groq.NewProvider(apiKey, "", model)

			ctx, cancel := context.WithTimeout(
				context.Background(), 30*time.Second,
			)
			defer cancel()

			req := &models.LLMRequest{
				ID: "test-groq-model-" + model,
				Messages: []models.Message{
					{
						Role:    "user",
						Content: "Reply with the word 'pong'.",
					},
				},
				ModelParams: models.ModelParameters{
					MaxTokens:   32,
					Temperature: 0.0,
				},
			}

			resp, err := provider.Complete(ctx, req)
			if skipOnGroqRateLimit(t, err) {
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
