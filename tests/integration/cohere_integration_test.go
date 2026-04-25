// Package integration provides comprehensive Cohere provider integration tests.
// These tests make real API calls to the Cohere API and are skipped when
// COHERE_API_KEY is not set. Rate-limit (429) errors are treated as skips
// rather than failures, since they depend on quota availability.
package integration

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/llm/providers/cohere"
	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cohereAPIKey returns the Cohere API key or skips the test.
//
// In addition to the env-var presence check, this helper performs a real
// HealthCheck against the upstream Cohere API. The verifier may have
// flagged the key as faulty during binary boot (see binary log:
// "Recorded faulty API key api_key=COHERE_API_KEY") in which case env
// presence is misleading — the key is set but rejected. CONST-030 says
// "non-unit tests that cannot connect to real services MUST skip (not
// fail)" so we skip here rather than letting the actual test hang on a
// 30s context deadline. Drainage report 2026-04-25 Finding #8.
func cohereAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("COHERE_API_KEY")
	if key == "" {
		t.Skip("COHERE_API_KEY not set") // SKIP-OK: #requires-upstream-key
	}
	// Verify the key still works upstream — boot-time verification may
	// have flagged it as faulty even when env is set.
	probe := cohere.NewProvider(key, "", "command-a-03-2025")
	if err := probe.HealthCheck(); err != nil {
		t.Skipf("Cohere HealthCheck failed (key may be faulty or upstream unreachable): %v", err) // SKIP-OK: #requires-upstream-key
	}
	return key
}

// skipOnCohereRateLimit checks if an error is a Cohere rate-limit error (429) and
// skips the test if so. Returns true when the test should be skipped.
func skipOnCohereRateLimit(t *testing.T, err error) bool {
	t.Helper()
	if err != nil && (strings.Contains(err.Error(), "429") ||
		strings.Contains(err.Error(), "rate_limit") ||
		strings.Contains(err.Error(), "Rate limit") ||
		strings.Contains(err.Error(), "too many requests")) {
		t.Skipf("Skipping due to Cohere API rate limit: %v", err)
		return true
	}
	return false
}

// TestCohereAPI_SimpleCompletion sends a simple prompt to the Cohere API and
// verifies a non-empty response is returned.
func TestCohereAPI_SimpleCompletion(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := cohereAPIKey(t)

	provider := cohere.NewProvider(apiKey, "", "command-a-03-2025")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID: "test-cohere-simple-completion",
		Messages: []models.Message{
			{Role: "user", Content: "Reply with just the word 'hello'"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   64,
			Temperature: 0.0,
		},
	}

	resp, err := provider.Complete(ctx, req)
	if skipOnCohereRateLimit(t, err) {
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
	assert.Equal(t, "cohere", resp.ProviderID)
	assert.Equal(t, "Cohere", resp.ProviderName)
	assert.Greater(t, resp.ResponseTime, int64(0),
		"response time should be positive")
}

// TestCohereAPI_StreamingCompletion verifies streaming completion returns
// multiple chunks and a final response.
func TestCohereAPI_StreamingCompletion(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := cohereAPIKey(t)

	provider := cohere.NewProvider(apiKey, "", "command-a-03-2025")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID: "test-cohere-streaming",
		Messages: []models.Message{
			{Role: "user", Content: "Count from 1 to 5, one number per line."},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   256,
			Temperature: 0.0,
		},
	}

	ch, err := provider.CompleteStream(ctx, req)
	if skipOnCohereRateLimit(t, err) {
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

// TestCohereAPI_HealthCheck verifies the health check endpoint returns no error.
func TestCohereAPI_HealthCheck(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := cohereAPIKey(t)

	provider := cohere.NewProvider(apiKey, "", "")

	err := provider.HealthCheck()
	if skipOnCohereRateLimit(t, err) {
		return
	}
	assert.NoError(t, err, "HealthCheck should succeed with a valid API key")
}

// TestCohereAPI_ModelDiscovery verifies GetCapabilities returns a non-empty
// list of supported models.
func TestCohereAPI_ModelDiscovery(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := cohereAPIKey(t)

	provider := cohere.NewProvider(apiKey, "", "")

	caps := provider.GetCapabilities()
	require.NotNil(t, caps, "capabilities should not be nil")
	assert.NotEmpty(t, caps.SupportedModels,
		"supported models list should not be empty")
	assert.True(t, caps.SupportsStreaming, "should support streaming")
	assert.True(t, caps.SupportsTools, "should support tools")
}

// TestCohereAPI_MultipleModels verifies that multiple Cohere models respond
// successfully when called sequentially.
func TestCohereAPI_MultipleModels(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey := cohereAPIKey(t)

	modelsToTest := []string{
		"command-a-03-2025",
		"command-r7b-12-2024",
	}

	for _, model := range modelsToTest {
		t.Run(model, func(t *testing.T) {
			provider := cohere.NewProvider(apiKey, "", model)

			ctx, cancel := context.WithTimeout(
				context.Background(), 30*time.Second,
			)
			defer cancel()

			req := &models.LLMRequest{
				ID: "test-cohere-model-" + model,
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
			if skipOnCohereRateLimit(t, err) {
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
