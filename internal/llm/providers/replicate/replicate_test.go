package replicate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	assert.NotNil(t, provider)
	assert.Equal(t, "test-api-key", provider.apiKey)
	assert.Equal(t, ReplicateAPIURL, provider.baseURL)
	assert.Equal(t, DefaultModel, provider.model)
}

func TestNewProviderWithCustomURL(t *testing.T) {
	t.Parallel()
	customURL := "https://custom.replicate.com/v1/predictions"
	provider := NewProvider("test-api-key", customURL, "meta/llama-2-13b-chat")
	assert.Equal(t, customURL, provider.baseURL)
	assert.Equal(t, "meta/llama-2-13b-chat", provider.model)
}

func TestNewProviderWithRetry(t *testing.T) {
	t.Parallel()
	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
	}
	provider := NewProviderWithRetry("test-key", "", "meta/meta-llama-3-70b-instruct", retryConfig)
	assert.Equal(t, 5, provider.retryConfig.MaxRetries)
	assert.Equal(t, 2*time.Second, provider.retryConfig.InitialDelay)
}

func TestDefaultRetryConfig(t *testing.T) {
	t.Parallel()
	config := DefaultRetryConfig()
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, time.Second, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
}

func TestComplete(t *testing.T) {
	t.Parallel()
	var requestCount int32 = 0
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		count := atomic.LoadInt32(&requestCount)

		if r.Method == "POST" {
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")

			var req PredictionRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "meta/llama-2-70b-chat", req.Model)

			// Return initial response with URLs
			resp := PredictionResponse{
				ID:     "pred_123",
				Model:  "meta/llama-2-70b-chat",
				Status: "starting",
				URLs: &URLs{
					Get:    serverURL + "/pred_123",
					Cancel: serverURL + "/pred_123/cancel",
				},
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if r.Method == "GET" {
			// Polling request - return completed after a few attempts
			status := "processing"
			var output interface{} = nil
			if count > 2 {
				status = "succeeded"
				output = []string{"Hello", " from", " Replicate!"}
			}

			resp := PredictionResponse{
				ID:     "pred_123",
				Model:  "meta/llama-2-70b-chat",
				Status: status,
				Output: output,
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	serverURL = server.URL
	defer server.Close()

	provider := NewProvider("test-api-key", server.URL, "meta/llama-2-70b-chat")
	req := &models.LLMRequest{
		ID:     "req-1",
		Prompt: "You are a helpful assistant.",
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
	assert.Equal(t, "pred_123", resp.ID)
	assert.Contains(t, resp.Content, "Replicate")
	assert.Equal(t, "replicate", resp.ProviderID)
	assert.Equal(t, "succeeded", resp.FinishReason)
}

func TestCompleteAPIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail": "Invalid API token"}`))
	}))
	defer server.Close()

	provider := NewProviderWithRetry("invalid-key", server.URL, "", RetryConfig{MaxRetries: 0})
	req := &models.LLMRequest{
		ID:       "req-error",
		Messages: []models.Message{{Role: "user", Content: "Test"}},
	}

	_, err := provider.Complete(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestCompletePredictionFailed(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			resp := PredictionResponse{
				ID:     "pred_failed",
				Status: "starting",
				URLs: &URLs{
					Get: r.URL.String() + "/pred_failed",
				},
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp := PredictionResponse{
			ID:     "pred_failed",
			Status: "failed",
			Error:  "Model failed to generate response",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewProvider("test-key", server.URL, "")
	req := &models.LLMRequest{
		Messages: []models.Message{{Role: "user", Content: "Test"}},
	}

	_, err := provider.Complete(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed")
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results": []}`))
	}))
	defer server.Close()

	// Direct test with the server
	provider := NewProvider("test-api-key", server.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	httpReq, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	httpReq.Header.Set("Authorization", "Bearer test-api-key")
	resp, err := provider.httpClient.Do(httpReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGetCapabilities(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	caps := provider.GetCapabilities()

	require.NotNil(t, caps)
	assert.Contains(t, caps.SupportedModels, "meta/llama-2-70b-chat")
	assert.Contains(t, caps.SupportedModels, "meta/meta-llama-3-70b-instruct")
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.Contains(t, caps.SupportedFeatures, "streaming")
	assert.Contains(t, caps.SupportedFeatures, "image_generation")
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsTools)
	assert.True(t, caps.SupportsVision)
	assert.Equal(t, 4096, caps.Limits.MaxTokens)
	assert.Equal(t, "replicate", caps.Metadata["provider"])
	assert.Equal(t, "open_models", caps.Metadata["specialization"])
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		apiKey   string
		expected bool
	}{
		{"valid key", "test-api-key", true},
		{"empty key", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			provider := NewProvider(tt.apiKey, "", "")
			valid, errors := provider.ValidateConfig(nil)
			assert.Equal(t, tt.expected, valid)
			if !tt.expected {
				assert.NotEmpty(t, errors)
			}
		})
	}
}

func TestConvertRequest(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "meta/llama-2-70b-chat")
	req := &models.LLMRequest{
		ID:     "test-id",
		Prompt: "You are a coding assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
			{Role: "user", Content: "Help me"},
		},
		ModelParams: models.ModelParameters{
			Model:         "meta/meta-llama-3-8b-instruct",
			Temperature:   0.8,
			MaxTokens:     2000,
			TopP:          0.9,
			StopSequences: []string{"END", "STOP"},
		},
	}

	apiReq := provider.convertRequest(req)
	assert.Equal(t, "meta/meta-llama-3-8b-instruct", apiReq.Model)
	assert.Equal(t, "You are a coding assistant.", apiReq.Input.SystemPrompt)
	assert.Contains(t, apiReq.Input.Prompt, "[INST]")
	assert.Equal(t, 0.8, apiReq.Input.Temperature)
	assert.Equal(t, 2000, apiReq.Input.MaxNewTokens)
	assert.Equal(t, "END,STOP", apiReq.Input.StopSequences)
}

func TestConvertRequestDefaultMaxTokens(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	req := &models.LLMRequest{
		Messages:    []models.Message{{Role: "user", Content: "Test"}},
		ModelParams: models.ModelParameters{},
	}

	apiReq := provider.convertRequest(req)
	assert.Equal(t, 512, apiReq.Input.MaxNewTokens)
}

func TestExtractOutput(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")

	tests := []struct {
		name     string
		output   interface{}
		expected string
	}{
		{"string output", "Hello world", "Hello world"},
		{"array output", []interface{}{"Hello", " ", "world"}, "Hello world"},
		{"nil output", nil, ""},
		{"map output", map[string]string{"text": "hello"}, `{"text":"hello"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			result := provider.extractOutput(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateConfidence(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")

	tests := []struct {
		content string
		status  string
		minConf float64
		maxConf float64
	}{
		{"Short", "succeeded", 0.9, 1.0},
		{strings.Repeat("Long content ", 20), "succeeded", 0.95, 1.0},
		{"Short", "failed", 0.5, 0.6},
		{"Short", "canceled", 0.6, 0.7},
	}

	for _, tt := range tests {
		conf := provider.calculateConfidence(tt.content, tt.status)
		assert.GreaterOrEqual(t, conf, tt.minConf)
		assert.LessOrEqual(t, conf, tt.maxConf)
	}
}

func TestCalculateBackoff(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")

	delay1 := provider.calculateBackoff(1)
	delay2 := provider.calculateBackoff(2)

	assert.LessOrEqual(t, delay1, 2*time.Second)
	assert.LessOrEqual(t, delay1, delay2+time.Second)

	delay10 := provider.calculateBackoff(10)
	assert.LessOrEqual(t, delay10, 35*time.Second)
}

func TestGetModel(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "meta/llama-2-70b-chat")
	assert.Equal(t, "meta/llama-2-70b-chat", provider.GetModel())
}

func TestSetModel(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "meta/llama-2-70b-chat")
	provider.SetModel("meta/meta-llama-3-70b-instruct")
	assert.Equal(t, "meta/meta-llama-3-70b-instruct", provider.GetModel())
}

func TestGetName(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	assert.Equal(t, "replicate", provider.GetName())
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	provider := NewProvider("test-key", server.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := &models.LLMRequest{
		Messages: []models.Message{{Role: "user", Content: "Test"}},
	}

	_, err := provider.Complete(ctx, req)
	require.Error(t, err)
}

// Benchmarks

func BenchmarkProvider_ConvertRequest(b *testing.B) {
	provider := NewProvider("test-key", "", "")
	req := &models.LLMRequest{
		ID:     "bench-request",
		Prompt: "You are a helpful assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi"},
			{Role: "user", Content: "How are you?"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:     100,
			Temperature:   0.7,
			StopSequences: []string{"END", "STOP"},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.convertRequest(req)
	}
}

func BenchmarkProvider_ExtractOutput(b *testing.B) {
	provider := NewProvider("test-key", "", "")
	output := []interface{}{"Hello", " from", " Replicate", "!"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.extractOutput(output)
	}
}

func BenchmarkProvider_CalculateConfidence(b *testing.B) {
	provider := NewProvider("test-key", "", "")
	content := strings.Repeat("Benchmark content ", 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.calculateConfidence(content, "succeeded")
	}
}

func BenchmarkProvider_CalculateBackoff(b *testing.B) {
	provider := NewProvider("test-key", "", "")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.calculateBackoff(3)
	}
}

func BenchmarkProvider_ConvertResponse(b *testing.B) {
	provider := NewProvider("test-key", "", "")
	req := &models.LLMRequest{ID: "bench-request"}
	startTime := time.Now()
	resp := &PredictionResponse{
		ID:     "pred_123",
		Model:  "meta/llama-2-70b-chat",
		Status: "succeeded",
		Output: []interface{}{"Hello", " from", " Replicate"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.convertResponse(req, resp, startTime)
	}
}
