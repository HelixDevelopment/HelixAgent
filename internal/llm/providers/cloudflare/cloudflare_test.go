package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCloudflareProvider(t *testing.T) {
	t.Parallel()
	provider := NewCloudflareProvider("test-api-key", "test-account-id", "", "")
	assert.NotNil(t, provider)
	assert.Equal(t, "test-api-key", provider.apiKey)
	assert.Equal(t, "test-account-id", provider.accountID)
	assert.Equal(t, CloudflareModel, provider.model)
}

func TestNewCloudflareProviderWithCustomURL(t *testing.T) {
	t.Parallel()
	customURL := "https://custom.cloudflare.com/v1/chat/completions"
	provider := NewCloudflareProvider("test-key", "test-account", customURL, "@cf/custom/model")
	assert.Equal(t, customURL, provider.baseURL)
	assert.Equal(t, "@cf/custom/model", provider.model)
}

func TestNewCloudflareProviderWithRetry(t *testing.T) {
	t.Parallel()
	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
	}
	provider := NewCloudflareProviderWithRetry("test-key", "test-account", "", "", retryConfig)
	assert.Equal(t, 5, provider.retryConfig.MaxRetries)
	assert.Equal(t, 2*time.Second, provider.retryConfig.InitialDelay)
}

func TestDefaultRetryConfig(t *testing.T) {
	t.Parallel()
	config := DefaultRetryConfig()
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
}

func TestComplete(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

		var req CloudflareRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "@cf/meta/llama-3.3-70b-instruct-fp8-fast", req.Model)

		resp := CloudflareResponse{
			Success: true,
			Result: CloudflareResult{
				Response: "Hello! I'm an AI assistant.",
				ID:       "resp_123",
				Model:    "@cf/meta/llama-3.3-70b-instruct-fp8-fast",
				Usage: CloudflareUsage{
					PromptTokens:     10,
					CompletionTokens: 8,
					TotalTokens:      18,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewCloudflareProvider("test-api-key", "test-account", server.URL, "")
	req := &models.LLMRequest{
		ID:     "req-1",
		Prompt: "You are helpful.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello!"},
		},
		ModelParams: models.ModelParameters{
			Temperature: 0.7,
			MaxTokens:   1000,
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "resp_123", resp.ID)
	assert.Contains(t, resp.Content, "AI assistant")
	assert.Equal(t, "cloudflare", resp.ProviderID)
	assert.Equal(t, "Cloudflare", resp.ProviderName)
	assert.Equal(t, 18, resp.TokensUsed)
}

func TestCompleteWithErrors(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := CloudflareResponse{
			Success: false,
			Errors: []CloudflareError{
				{Code: 400, Message: "Invalid request"},
			},
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewCloudflareProvider("test-key", "test-account", server.URL, "")
	req := &models.LLMRequest{
		ID:       "req-1",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	_, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid request")
}

func TestCompleteStream(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`{"response":"Hello","id":"stream-1","model":"@cf/meta/llama-3.3-70b-instruct-fp8-fast"}`,
			`{"response":" there","id":"stream-2","model":"@cf/meta/llama-3.3-70b-instruct-fp8-fast"}`,
			`{"response":"!","id":"stream-3","model":"@cf/meta/llama-3.3-70b-instruct-fp8-fast","done":true}`,
		}

		for _, chunk := range chunks {
			_, _ = w.Write([]byte("data: " + chunk + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	provider := NewCloudflareProvider("test-key", "test-account", server.URL, "")
	req := &models.LLMRequest{
		ID:       "stream-req",
		Messages: []models.Message{{Role: "user", Content: "Hi"}},
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	require.NoError(t, err)

	var responses []*models.LLMResponse
	for resp := range ch {
		responses = append(responses, resp)
	}

	assert.GreaterOrEqual(t, len(responses), 2)
}

func TestGetCapabilities(t *testing.T) {
	t.Parallel()
	provider := NewCloudflareProvider("test-key", "test-account", "", "")
	caps := provider.GetCapabilities()

	assert.NotNil(t, caps)
	assert.Contains(t, caps.SupportedModels, CloudflareModel)
	assert.Contains(t, caps.SupportedFeatures, "text_completion")
	assert.Contains(t, caps.SupportedFeatures, "streaming")
	assert.True(t, caps.SupportsStreaming)
	assert.Equal(t, CloudflareMaxContext, caps.Limits.MaxTokens)
	assert.Equal(t, CloudflareMaxContext, caps.Limits.MaxInputLength)
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		apiKey    string
		accountID string
		wantValid bool
		wantErrs  int
	}{
		{
			name:      "valid config",
			apiKey:    "test-key",
			accountID: "test-account",
			wantValid: true,
			wantErrs:  0,
		},
		{
			name:      "missing api key",
			apiKey:    "",
			accountID: "test-account",
			wantValid: false,
			wantErrs:  1,
		},
		{
			name:      "missing account id",
			apiKey:    "test-key",
			accountID: "",
			wantValid: false,
			wantErrs:  1,
		},
		{
			name:      "missing both",
			apiKey:    "",
			accountID: "",
			wantValid: false,
			wantErrs:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := NewCloudflareProvider(tt.apiKey, tt.accountID, "", "")
			valid, errs := provider.ValidateConfig(nil)
			assert.Equal(t, tt.wantValid, valid)
			assert.Len(t, errs, tt.wantErrs)
		})
	}
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping health check test in short mode - requires real Cloudflare API") // SKIP-OK: #short-mode
	}

	provider := NewCloudflareProvider("test-key", "test-account", "", "")
	err := provider.HealthCheck()
	// In short mode we skip, otherwise we accept either success or error
	_ = err
}

func TestHealthCheckNoAccountID(t *testing.T) {
	t.Parallel()
	provider := NewCloudflareProvider("test-key", "", "", "")
	err := provider.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "account ID is required")
}

func TestConvertRequest(t *testing.T) {
	t.Parallel()
	provider := NewCloudflareProvider("test-key", "test-account", "", "")
	req := &models.LLMRequest{
		Prompt: "You are helpful.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
		},
		ModelParams: models.ModelParameters{
			Temperature: 0.8,
			MaxTokens:   500,
			TopP:        0.9,
		},
	}

	cReq := provider.convertRequest(req)

	assert.Equal(t, CloudflareModel, cReq.Model)
	assert.Len(t, cReq.Messages, 3) // system + 2 messages
	assert.Equal(t, "system", cReq.Messages[0].Role)
	assert.Equal(t, "user", cReq.Messages[1].Role)
	assert.Equal(t, 0.8, cReq.Temperature)
	assert.Equal(t, 500, cReq.MaxTokens)
	assert.Equal(t, 0.9, cReq.TopP)
}

func TestConvertResponse(t *testing.T) {
	t.Parallel()
	provider := NewCloudflareProvider("test-key", "test-account", "", "")
	startTime := time.Now()

	cResp := &CloudflareResponse{
		Success: true,
		Result: CloudflareResult{
			Response: "Test response",
			ID:       "resp-123",
			Model:    "@cf/meta/llama-3.3-70b-instruct-fp8-fast",
			Usage: CloudflareUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
	}

	req := &models.LLMRequest{ID: "req-123"}
	resp := provider.convertResponse(req, cResp, startTime)

	assert.Equal(t, "resp-123", resp.ID)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, "cloudflare", resp.ProviderID)
	assert.Equal(t, "Cloudflare", resp.ProviderName)
	assert.Equal(t, "Test response", resp.Content)
	assert.Equal(t, 15, resp.TokensUsed)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.GreaterOrEqual(t, resp.Confidence, 0.8)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkCloudflareProvider_ConvertRequest(b *testing.B) {
	provider := NewCloudflareProvider("test-key", "test-account", "", "")
	req := &models.LLMRequest{
		ID: "bench-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi"},
			{Role: "user", Content: "How are you?"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.convertRequest(req)
	}
}

func BenchmarkCloudflareProvider_GetCapabilities(b *testing.B) {
	provider := NewCloudflareProvider("test-key", "test-account", "", "")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.GetCapabilities()
	}
}
