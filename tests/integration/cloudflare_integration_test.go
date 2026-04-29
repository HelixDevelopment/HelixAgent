// Package integration provides comprehensive Cloudflare Workers AI provider
// integration tests. These tests make real API calls to the Cloudflare API
// and are skipped when CLOUDFLARE_API_KEY or CLOUDFLARE_ACCOUNT_ID is not set.
// Rate-limit (429) errors are treated as skips rather than failures, since
// they depend on quota availability.
package integration

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/llm/providers/cloudflare"
	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cloudflareCredentials returns the Cloudflare API key and account ID or
// skips the test when either is missing.
func cloudflareCredentials(t *testing.T) (string, string) {
	t.Helper()
	apiKey := os.Getenv("CLOUDFLARE_API_KEY")
	if apiKey == "" {
		t.Skip("CLOUDFLARE_API_KEY not set")  // SKIP-OK: #requires-upstream-key
	}
	accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	if accountID == "" {
		t.Skip("CLOUDFLARE_ACCOUNT_ID not set")  // SKIP-OK: #legacy-untriaged
	}
	return apiKey, accountID
}

// skipOnCloudflareRateLimit checks if an error is a Cloudflare rate-limit
// error (429) and skips the test if so. Returns true when the test should
// be skipped.
func skipOnCloudflareRateLimit(t *testing.T, err error) bool {
	t.Helper()
	if err != nil && (strings.Contains(err.Error(), "429") ||
		strings.Contains(err.Error(), "rate_limit") ||
		strings.Contains(err.Error(), "Rate limit")) {
		t.Skipf("Skipping due to Cloudflare API rate limit: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		return true
	}
	return false
}

// TestCloudflareAPI_SimpleCompletion sends a simple prompt to the Cloudflare
// Workers AI API and verifies a non-empty response is returned.
func TestCloudflareAPI_SimpleCompletion(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey, accountID := cloudflareCredentials(t)

	provider := cloudflare.NewCloudflareProvider(apiKey, accountID, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID: "test-cloudflare-simple-completion",
		Messages: []models.Message{
			{Role: "user", Content: "Reply with just the word 'hello'"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   64,
			Temperature: 0.0,
		},
	}

	resp, err := provider.Complete(ctx, req)
	if skipOnCloudflareRateLimit(t, err) {
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
	assert.Equal(t, "cloudflare", resp.ProviderID)
	assert.Equal(t, "Cloudflare", resp.ProviderName)
	assert.Greater(t, resp.ResponseTime, int64(0),
		"response time should be positive")
}

// TestCloudflareAPI_StreamingCompletion verifies streaming completion returns
// multiple chunks and a final response with stop finish reason.
func TestCloudflareAPI_StreamingCompletion(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey, accountID := cloudflareCredentials(t)

	provider := cloudflare.NewCloudflareProvider(apiKey, accountID, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &models.LLMRequest{
		ID: "test-cloudflare-streaming",
		Messages: []models.Message{
			{Role: "user", Content: "Count from 1 to 5, one number per line."},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   256,
			Temperature: 0.0,
		},
	}

	ch, err := provider.CompleteStream(ctx, req)
	if skipOnCloudflareRateLimit(t, err) {
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

// TestCloudflareAPI_HealthCheck verifies the health check endpoint returns
// no error when valid credentials are provided.
func TestCloudflareAPI_HealthCheck(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey, accountID := cloudflareCredentials(t)

	provider := cloudflare.NewCloudflareProvider(apiKey, accountID, "", "")

	err := provider.HealthCheck()
	if skipOnCloudflareRateLimit(t, err) {
		return
	}
	assert.NoError(t, err, "HealthCheck should succeed with valid credentials")
}

// TestCloudflareAPI_ModelDiscovery verifies GetCapabilities returns a
// non-empty list of supported models and correct provider metadata.
func TestCloudflareAPI_ModelDiscovery(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey, accountID := cloudflareCredentials(t)

	provider := cloudflare.NewCloudflareProvider(apiKey, accountID, "", "")

	caps := provider.GetCapabilities()
	require.NotNil(t, caps, "capabilities should not be nil")
	assert.NotEmpty(t, caps.SupportedModels,
		"supported models list should not be empty")
	assert.True(t, caps.SupportsStreaming, "should support streaming")
}

// TestCloudflareAPI_MultipleModels verifies that multiple Cloudflare Workers
// AI models respond successfully when called sequentially.
func TestCloudflareAPI_MultipleModels(t *testing.T) {
	runtime.GOMAXPROCS(2)
	apiKey, accountID := cloudflareCredentials(t)

	modelsToTest := []string{
		"@cf/meta/llama-3.1-8b-instruct",
		"@cf/mistral/mistral-7b-instruct",
	}

	for _, model := range modelsToTest {
		t.Run(model, func(t *testing.T) {
			provider := cloudflare.NewCloudflareProvider(
				apiKey, accountID, "", model,
			)

			ctx, cancel := context.WithTimeout(
				context.Background(), 60*time.Second,
			)
			defer cancel()

			req := &models.LLMRequest{
				ID: "test-cloudflare-model-" + model,
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
			if skipOnCloudflareRateLimit(t, err) {
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
