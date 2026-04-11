package huggingface

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	assert.Equal(t, HuggingFaceProURL, provider.baseURL)
	assert.Equal(t, DefaultModel, provider.model)
	assert.True(t, provider.usePro)
}

func TestNewProviderWithCustomURL(t *testing.T) {
	t.Parallel()
	customURL := "https://custom.huggingface.co/models/"
	provider := NewProvider("test-api-key", customURL, "mistralai/Mistral-7B-Instruct-v0.2")
	assert.Equal(t, customURL, provider.baseURL)
	assert.Equal(t, "mistralai/Mistral-7B-Instruct-v0.2", provider.model)
	assert.False(t, provider.usePro)
}

func TestNewProviderWithRetry(t *testing.T) {
	t.Parallel()
	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
	}
	provider := NewProviderWithRetry("test-key", "", "google/gemma-7b-it", retryConfig)
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

func TestCompletePro(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")

		var req ChatRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "meta-llama/Meta-Llama-3-8B-Instruct", req.Model)

		resp := ChatResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "meta-llama/Meta-Llama-3-8B-Instruct",
			Choices: []Choice{
				{
					Index:        0,
					Message:      Message{Role: "assistant", Content: "Hello from HuggingFace!"},
					FinishReason: "stop",
				},
			},
			Usage: Usage{
				PromptTokens:     15,
				CompletionTokens: 8,
				TotalTokens:      23,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewProvider("test-api-key", server.URL+"/v1/chat/completions", "meta-llama/Meta-Llama-3-8B-Instruct")
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
	assert.Equal(t, "chatcmpl-123", resp.ID)
	assert.Equal(t, "Hello from HuggingFace!", resp.Content)
	assert.Equal(t, "huggingface", resp.ProviderID)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 23, resp.TokensUsed)
}

func TestCompleteInference(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")

		var req InferenceRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Contains(t, req.Inputs, "user:")

		responses := []InferenceResponse{
			{GeneratedText: "Generated response from HuggingFace"},
		}
		_ = json.NewEncoder(w).Encode(responses)
	}))
	defer server.Close()

	// Non-pro provider (inference API)
	provider := NewProvider("test-api-key", server.URL+"/models/", "meta-llama/Meta-Llama-3-8B-Instruct")
	provider.usePro = false
	req := &models.LLMRequest{
		ID:       "req-inf",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, resp.Content, "Generated response")
	assert.Equal(t, "huggingface", resp.ProviderID)
}

func TestCompleteAPIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "Invalid API token"}`))
	}))
	defer server.Close()

	provider := NewProviderWithRetry("invalid-key", server.URL+"/v1/chat/completions", "", RetryConfig{MaxRetries: 0})
	req := &models.LLMRequest{
		ID:       "req-error",
		Messages: []models.Message{{Role: "user", Content: "Test"}},
	}

	_, err := provider.Complete(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestCompleteStream(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.True(t, req.Stream)

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		events := []string{
			`data: {"id":"chunk-1","choices":[{"delta":{"content":"Hello"}}]}`,
			`data: {"id":"chunk-2","choices":[{"delta":{"content":" from"}}]}`,
			`data: {"id":"chunk-3","choices":[{"delta":{"content":" HuggingFace"}}]}`,
			`data: [DONE]`,
		}

		for _, event := range events {
			_, _ = w.Write([]byte(event + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	provider := NewProvider("test-api-key", server.URL+"/v1/chat/completions", "")
	req := &models.LLMRequest{
		ID:       "req-stream",
		Messages: []models.Message{{Role: "user", Content: "Say hello"}},
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	require.NoError(t, err)

	var responses []*models.LLMResponse
	for resp := range ch {
		responses = append(responses, resp)
	}

	require.GreaterOrEqual(t, len(responses), 3)
	lastResp := responses[len(responses)-1]
	assert.Equal(t, "Hello from HuggingFace", lastResp.Content)
	assert.Equal(t, "stop", lastResp.FinishReason)
}

func TestCompleteStreamError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error": "Service unavailable"}`))
	}))
	defer server.Close()

	provider := NewProviderWithRetry("test-key", server.URL+"/v1/chat/completions", "", RetryConfig{MaxRetries: 0})
	req := &models.LLMRequest{
		ID:       "req-stream-error",
		Messages: []models.Message{{Role: "user", Content: "Test"}},
	}

	_, err := provider.CompleteStream(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestGetCapabilities(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	caps := provider.GetCapabilities()

	require.NotNil(t, caps)
	assert.Contains(t, caps.SupportedModels, "meta-llama/Llama-3.3-70B-Instruct")
	assert.Contains(t, caps.SupportedModels, "mistralai/Mistral-7B-Instruct-v0.3")
	assert.Contains(t, caps.SupportedModels, "google/gemma-2-9b-it")
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.Contains(t, caps.SupportedFeatures, "streaming")
	assert.Contains(t, caps.SupportedFeatures, "text_generation")
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsTools)
	assert.True(t, caps.SupportsVision)
	assert.Equal(t, 8192, caps.Limits.MaxTokens)
	assert.Equal(t, "huggingface", caps.Metadata["provider"])
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

func TestConvertChatRequest(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "meta-llama/Meta-Llama-3-8B-Instruct")
	req := &models.LLMRequest{
		ID:     "test-id",
		Prompt: "You are a coding assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
			{Role: "user", Content: "Help me"},
		},
		ModelParams: models.ModelParameters{
			Model:         "google/gemma-7b-it",
			Temperature:   0.8,
			MaxTokens:     2000,
			TopP:          0.9,
			StopSequences: []string{"END"},
		},
	}

	apiReq := provider.convertChatRequest(req)
	assert.Equal(t, "google/gemma-7b-it", apiReq.Model)
	assert.Len(t, apiReq.Messages, 4) // system + 3 messages
	assert.Equal(t, "system", apiReq.Messages[0].Role)
	assert.Equal(t, 0.8, apiReq.Temperature)
	assert.Equal(t, 2000, apiReq.MaxTokens)
	assert.Equal(t, []string{"END"}, apiReq.Stop)
}

func TestConvertInferenceRequest(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "meta-llama/Meta-Llama-3-8B-Instruct")
	provider.usePro = false
	req := &models.LLMRequest{
		ID:     "test-id",
		Prompt: "You are helpful.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			Temperature: 0.7,
			MaxTokens:   500,
		},
	}

	apiReq := provider.convertInferenceRequest(req)
	assert.Contains(t, apiReq.Inputs, "System:")
	assert.Contains(t, apiReq.Inputs, "user:")
	assert.Equal(t, 500, apiReq.Parameters.MaxNewTokens)
	assert.Equal(t, 0.7, apiReq.Parameters.Temperature)
	assert.True(t, apiReq.Options.WaitForModel)
}

func TestConvertRequestDefaultMaxTokens(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	req := &models.LLMRequest{
		Messages:    []models.Message{{Role: "user", Content: "Test"}},
		ModelParams: models.ModelParameters{},
	}

	apiReq := provider.convertChatRequest(req)
	assert.Equal(t, 1024, apiReq.MaxTokens)
}

func TestConvertChatResponse(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	req := &models.LLMRequest{ID: "req-123"}
	startTime := time.Now()

	apiResp := &ChatResponse{
		ID:    "resp-456",
		Model: "meta-llama/Meta-Llama-3-8B-Instruct",
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "HuggingFace response"},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	resp := provider.convertChatResponse(req, apiResp, startTime)
	assert.Equal(t, "resp-456", resp.ID)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, "HuggingFace response", resp.Content)
	assert.Equal(t, "huggingface", resp.ProviderID)
	assert.Equal(t, 150, resp.TokensUsed)
	assert.Equal(t, "stop", resp.FinishReason)
}

func TestConvertInferenceResponse(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "test-model")
	req := &models.LLMRequest{ID: "req-inf"}
	startTime := time.Now()

	responses := []InferenceResponse{
		{GeneratedText: "Part 1 "},
		{GeneratedText: "Part 2"},
	}

	resp := provider.convertInferenceResponse(req, responses, startTime)
	assert.Equal(t, "Part 1 Part 2", resp.Content)
	assert.Equal(t, "huggingface", resp.ProviderID)
	assert.Equal(t, "stop", resp.FinishReason)
}

func TestCalculateConfidence(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")

	tests := []struct {
		content      string
		finishReason string
		minConf      float64
		maxConf      float64
	}{
		{"Short", "stop", 0.9, 1.0},
		{strings.Repeat("Long content ", 20), "stop", 0.95, 1.0},
		{"Short", "length", 0.7, 0.8},
		{"Short", "eos_token", 0.9, 1.0},
	}

	for _, tt := range tests {
		conf := provider.calculateConfidence(tt.content, tt.finishReason)
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
	provider := NewProvider("test-api-key", "", "meta-llama/Meta-Llama-3-8B-Instruct")
	assert.Equal(t, "meta-llama/Meta-Llama-3-8B-Instruct", provider.GetModel())
}

func TestSetModel(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "meta-llama/Meta-Llama-3-8B-Instruct")
	provider.SetModel("google/gemma-7b-it")
	assert.Equal(t, "google/gemma-7b-it", provider.GetModel())
}

func TestGetName(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	assert.Equal(t, "huggingface", provider.GetName())
}

func TestRetryOnServerError(t *testing.T) {
	t.Parallel()
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resp := ChatResponse{
			ID:      "success",
			Choices: []Choice{{Message: Message{Content: "Success"}, FinishReason: "stop"}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewProviderWithRetry("test-key", server.URL+"/v1/chat/completions", "", RetryConfig{
		MaxRetries:   5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	})

	req := &models.LLMRequest{
		Messages: []models.Message{{Role: "user", Content: "Test"}},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "success", resp.ID)
	assert.Equal(t, 3, attempts)
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	provider := NewProvider("test-key", server.URL+"/v1/chat/completions", "")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := &models.LLMRequest{
		Messages: []models.Message{{Role: "user", Content: "Test"}},
	}

	_, err := provider.Complete(ctx, req)
	require.Error(t, err)
}

func TestMultipleModels(t *testing.T) {
	t.Parallel()
	testModels := []string{
		"meta-llama/Meta-Llama-3-8B-Instruct",
		"mistralai/Mistral-7B-Instruct-v0.2",
		"google/gemma-7b-it",
	}

	for _, model := range testModels {
		t.Run(model, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req ChatRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				assert.Equal(t, model, req.Model)

				resp := ChatResponse{
					ID:      "test-" + model,
					Model:   model,
					Choices: []Choice{{Message: Message{Content: "Response from " + model}, FinishReason: "stop"}},
				}
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			provider := NewProvider("test-key", server.URL+"/v1/chat/completions", model)
			req := &models.LLMRequest{
				Messages:    []models.Message{{Role: "user", Content: "Test"}},
				ModelParams: models.ModelParameters{},
			}

			resp, err := provider.Complete(context.Background(), req)
			require.NoError(t, err)
			assert.Contains(t, resp.Content, model)
		})
	}
}

// Benchmarks

func BenchmarkProvider_ConvertChatRequest(b *testing.B) {
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
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.convertChatRequest(req)
	}
}

func BenchmarkProvider_ConvertInferenceRequest(b *testing.B) {
	provider := NewProvider("test-key", "", "")
	provider.usePro = false
	req := &models.LLMRequest{
		ID:     "bench-request",
		Prompt: "You are helpful.",
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
		provider.convertInferenceRequest(req)
	}
}

func BenchmarkProvider_ConvertChatResponse(b *testing.B) {
	provider := NewProvider("test-key", "", "")
	req := &models.LLMRequest{ID: "bench-request"}
	startTime := time.Now()
	apiResp := &ChatResponse{
		ID:    "resp-456",
		Model: "meta-llama/Meta-Llama-3-8B-Instruct",
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "Benchmark response content"},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.convertChatResponse(req, apiResp, startTime)
	}
}

func BenchmarkProvider_ConvertInferenceResponse(b *testing.B) {
	provider := NewProvider("test-key", "", "test-model")
	req := &models.LLMRequest{ID: "bench-request"}
	startTime := time.Now()
	responses := []InferenceResponse{
		{GeneratedText: "Part 1 "},
		{GeneratedText: "Part 2"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.convertInferenceResponse(req, responses, startTime)
	}
}

func BenchmarkProvider_CalculateConfidence(b *testing.B) {
	provider := NewProvider("test-key", "", "")
	content := strings.Repeat("Benchmark content ", 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.calculateConfidence(content, "stop")
	}
}

func BenchmarkProvider_CalculateBackoff(b *testing.B) {
	provider := NewProvider("test-key", "", "")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.calculateBackoff(3)
	}
}
