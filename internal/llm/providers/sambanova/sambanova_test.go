package sambanova

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

func TestNewProvider(t *testing.T) {
	t.Parallel()
	provider := NewSambaNovaProvider("test-api-key", "", "")
	assert.NotNil(t, provider)
	assert.Equal(t, "test-api-key", provider.apiKey)
	assert.Equal(t, "https://api.sambanova.ai/v1/chat/completions", provider.baseURL)
	assert.Equal(t, "Meta-Llama-3.1-8B-Instruct", provider.model)
}

func TestNewProviderWithCustomURL(t *testing.T) {
	t.Parallel()
	customURL := "https://custom.api.com/v1/chat/completions"
	provider := NewSambaNovaProvider("test-key", customURL, "custom-model")
	assert.Equal(t, customURL, provider.baseURL)
	assert.Equal(t, "custom-model", provider.model)
}

func TestNewProviderWithRetry(t *testing.T) {
	t.Parallel()
	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
	}
	provider := NewSambaNovaProviderWithRetry("test-key", "", "", retryConfig)
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
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer")

		resp := map[string]interface{}{
			"id":      "resp_123",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "test-model",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"message":       map[string]string{"role": "assistant", "content": "Hello!"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewSambaNovaProvider("test-api-key", server.URL, "")
	req := &models.LLMRequest{
		ID:       "req-1",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
		ModelParams: models.ModelParameters{
			Temperature: 0.7,
			MaxTokens:   1000,
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "resp_123", resp.ID)
	assert.Contains(t, resp.Content, "Hello")
	assert.Equal(t, "sambanova", resp.ProviderID)
}

func TestCompleteWithError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		resp := map[string]interface{}{
			"error": map[string]string{"message": "Invalid API key"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewSambaNovaProvider("invalid-key", server.URL, "")
	req := &models.LLMRequest{ID: "req-1", Messages: []models.Message{{Role: "user", Content: "Hi"}}}

	_, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
}

func TestCompleteStream(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			"data: {\"id\":\"stream-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n",
			"data: {\"id\":\"stream-2\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"delta\":{\"content\":\" there\"}}]}\n\n",
			"data: [DONE]\n\n",
		}

		for _, chunk := range chunks {
			_, _ = w.Write([]byte(chunk))
		}
	}))
	defer server.Close()

	provider := NewSambaNovaProvider("test-key", server.URL, "")
	req := &models.LLMRequest{ID: "stream-req", Messages: []models.Message{{Role: "user", Content: "Hi"}}}

	ch, err := provider.CompleteStream(context.Background(), req)
	require.NoError(t, err)

	var responses int
	for range ch {
		responses++
	}
	assert.GreaterOrEqual(t, responses, 1)
}

func TestGetCapabilities(t *testing.T) {
	t.Parallel()
	provider := NewSambaNovaProvider("test-key", "", "")
	caps := provider.GetCapabilities()

	assert.NotNil(t, caps)
	assert.NotEmpty(t, caps.SupportedModels)
	assert.Contains(t, caps.SupportedFeatures, "text_completion")
	assert.True(t, caps.SupportsStreaming)
	assert.Equal(t, 4096, caps.Limits.MaxTokens)
	assert.Equal(t, 131072, caps.Limits.MaxInputLength)
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		apiKey    string
		wantValid bool
	}{
		{name: "valid config", apiKey: "test-key", wantValid: true},
		{name: "missing api key", apiKey: "", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			provider := NewSambaNovaProvider(tt.apiKey, "", "")
			valid, errs := provider.ValidateConfig(nil)
			assert.Equal(t, tt.wantValid, valid)
			if !tt.wantValid {
				assert.NotEmpty(t, errs)
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping health check test in short mode")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"object": "list",
			"data":   []interface{}{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewSambaNovaProvider("test-key", server.URL, "")
	err := provider.HealthCheck()
	assert.NoError(t, err)
}

func TestHealthCheckWithError(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping health check test in short mode")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	provider := NewSambaNovaProvider("test-key", server.URL, "")
	err := provider.HealthCheck()
	assert.Error(t, err)
}

// Benchmarks

func BenchmarkSambaNovaProvider_ConvertRequest(b *testing.B) {
	provider := NewSambaNovaProvider("test-key", "", "")
	req := &models.LLMRequest{
		ID:     "bench-request",
		Prompt: "You are a helpful assistant.",
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

func BenchmarkSambaNovaProvider_ConvertResponse(b *testing.B) {
	provider := NewSambaNovaProvider("test-key", "", "")
	req := &models.LLMRequest{ID: "bench-request"}
	startTime := time.Now()
	sResp := &SambaNovaResponse{
		ID:    "resp-456",
		Model: "Meta-Llama-3.1-8B-Instruct",
		Choices: []SambaNovaChoice{
			{
				Index:        0,
				Message:      SambaNovaMessage{Role: "assistant", Content: "Benchmark response content"},
				FinishReason: "stop",
			},
		},
		Usage: SambaNovaUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.convertResponse(req, sResp, startTime)
	}
}
