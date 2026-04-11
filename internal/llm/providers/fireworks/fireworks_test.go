package fireworks

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
	assert.Equal(t, FireworksAPIURL, provider.baseURL)
	assert.Equal(t, DefaultModel, provider.model)
}

func TestNewProviderWithCustomURL(t *testing.T) {
	t.Parallel()
	customURL := "https://custom.fireworks.ai/inference/v1/chat/completions"
	provider := NewProvider("test-api-key", customURL, "accounts/fireworks/models/llama-v3p1-8b-instruct")
	assert.Equal(t, customURL, provider.baseURL)
	assert.Equal(t, "accounts/fireworks/models/llama-v3p1-8b-instruct", provider.model)
}

func TestNewProviderWithRetry(t *testing.T) {
	t.Parallel()
	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
	}
	provider := NewProviderWithRetry("test-key", "", "accounts/fireworks/models/deepseek-r1", retryConfig)
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")

		var req Request
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "accounts/fireworks/models/llama-v3p1-70b-instruct", req.Model)

		resp := Response{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "accounts/fireworks/models/llama-v3p1-70b-instruct",
			Choices: []Choice{
				{
					Index:        0,
					Message:      Message{Role: "assistant", Content: "Fireworks AI response!"},
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

	provider := NewProvider("test-api-key", server.URL, "accounts/fireworks/models/llama-v3p1-70b-instruct")
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
	assert.Equal(t, "Fireworks AI response!", resp.Content)
	assert.Equal(t, "fireworks", resp.ProviderID)
	assert.Equal(t, "Fireworks AI", resp.ProviderName)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 23, resp.TokensUsed)
}

func TestCompleteWithTools(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Len(t, req.Tools, 1)
		assert.Equal(t, "function", req.Tools[0].Type)
		assert.Equal(t, "get_weather", req.Tools[0].Function.Name)
		assert.Equal(t, "auto", req.ToolChoice)

		resp := Response{
			ID:    "chatcmpl-tools",
			Model: "accounts/fireworks/models/firefunction-v2",
			Choices: []Choice{
				{
					Index: 0,
					Message: Message{
						Role: "assistant",
						ToolCalls: []ToolCall{
							{
								ID:   "call-1",
								Type: "function",
								Function: FunctionCall{
									Name:      "get_weather",
									Arguments: `{"location": "Tokyo"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: Usage{TotalTokens: 30},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewProvider("test-api-key", server.URL, "accounts/fireworks/models/firefunction-v2")
	req := &models.LLMRequest{
		ID: "req-tools",
		Messages: []models.Message{
			{Role: "user", Content: "What's the weather in Tokyo?"},
		},
		ModelParams: models.ModelParameters{},
		Tools: []models.Tool{
			{
				Type: "function",
				Function: models.ToolFunction{
					Name:        "get_weather",
					Description: "Get current weather",
					Parameters:  map[string]any{"type": "object"},
				},
			},
		},
		ToolChoice: "auto",
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "call-1", resp.ToolCalls[0].ID)
	assert.Equal(t, "get_weather", resp.ToolCalls[0].Function.Name)
	assert.Contains(t, resp.ToolCalls[0].Function.Arguments, "Tokyo")
	assert.Equal(t, "tool_calls", resp.FinishReason)
}

func TestCompleteWithDeepSeekR1(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "accounts/fireworks/models/deepseek-r1", req.Model)

		resp := Response{
			ID:    "chatcmpl-deepseek",
			Model: "accounts/fireworks/models/deepseek-r1",
			Choices: []Choice{
				{
					Index:        0,
					Message:      Message{Role: "assistant", Content: "<think>reasoning here</think>The answer."},
					FinishReason: "stop",
				},
			},
			Usage: Usage{TotalTokens: 50},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewProvider("test-api-key", server.URL, "accounts/fireworks/models/deepseek-r1")
	req := &models.LLMRequest{
		ID:          "req-reasoning",
		Messages:    []models.Message{{Role: "user", Content: "Solve a complex problem"}},
		ModelParams: models.ModelParameters{},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, resp.Content, "reasoning")
}

func TestCompleteAPIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
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

func TestCompleteStream(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.True(t, req.Stream)

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		events := []string{
			`data: {"id":"chunk-1","choices":[{"delta":{"content":"Fast"}}]}`,
			`data: {"id":"chunk-2","choices":[{"delta":{"content":" inference"}}]}`,
			`data: {"id":"chunk-3","choices":[{"delta":{"content":" from"}}]}`,
			`data: {"id":"chunk-4","choices":[{"delta":{"content":" Fireworks"}}]}`,
			`data: [DONE]`,
		}

		for _, event := range events {
			_, _ = w.Write([]byte(event + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	provider := NewProvider("test-api-key", server.URL, "")
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

	require.GreaterOrEqual(t, len(responses), 4)
	lastResp := responses[len(responses)-1]
	assert.Equal(t, "Fast inference from Fireworks", lastResp.Content)
	assert.Equal(t, "stop", lastResp.FinishReason)
}

func TestCompleteStreamError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error": "Service unavailable"}`))
	}))
	defer server.Close()

	provider := NewProviderWithRetry("test-key", server.URL, "", RetryConfig{MaxRetries: 0})
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
	// Check Llama models
	assert.Contains(t, caps.SupportedModels, "accounts/fireworks/models/llama-v3p1-405b-instruct")
	assert.Contains(t, caps.SupportedModels, "accounts/fireworks/models/llama-v3p1-70b-instruct")
	// Check DeepSeek models
	assert.Contains(t, caps.SupportedModels, "accounts/fireworks/models/deepseek-r1")
	// Check function calling model
	assert.Contains(t, caps.SupportedModels, "accounts/fireworks/models/firefunction-v2")
	// Check features
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.Contains(t, caps.SupportedFeatures, "streaming")
	assert.Contains(t, caps.SupportedFeatures, "tools")
	assert.Contains(t, caps.SupportedFeatures, "vision")
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsTools)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsVision)
	assert.Equal(t, 131072, caps.Limits.MaxTokens)
	assert.Equal(t, "fireworks", caps.Metadata["provider"])
	assert.Equal(t, "fast_inference", caps.Metadata["specialization"])
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
	provider := NewProvider("test-api-key", "", "accounts/fireworks/models/llama-v3p1-70b-instruct")
	req := &models.LLMRequest{
		ID:     "test-id",
		Prompt: "You are a helpful assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
			{Role: "user", Content: "Help me"},
		},
		ModelParams: models.ModelParameters{
			Model:         "accounts/fireworks/models/llama-v3p1-8b-instruct",
			Temperature:   0.8,
			MaxTokens:     2000,
			TopP:          0.9,
			StopSequences: []string{"END"},
		},
	}

	apiReq := provider.convertRequest(req)
	assert.Equal(t, "accounts/fireworks/models/llama-v3p1-8b-instruct", apiReq.Model)
	assert.Len(t, apiReq.Messages, 4) // system + 3 messages
	assert.Equal(t, "system", apiReq.Messages[0].Role)
	assert.Equal(t, 0.8, apiReq.Temperature)
	assert.Equal(t, 2000, apiReq.MaxTokens)
	assert.Equal(t, []string{"END"}, apiReq.Stop)
}

func TestConvertRequestDefaultMaxTokens(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	req := &models.LLMRequest{
		Messages:    []models.Message{{Role: "user", Content: "Test"}},
		ModelParams: models.ModelParameters{},
	}

	apiReq := provider.convertRequest(req)
	assert.Equal(t, 4096, apiReq.MaxTokens)
}

func TestConvertResponse(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	req := &models.LLMRequest{ID: "req-123"}
	startTime := time.Now()

	apiResp := &Response{
		ID:    "resp-456",
		Model: "accounts/fireworks/models/llama-v3p1-70b-instruct",
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "Fireworks response"},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	resp := provider.convertResponse(req, apiResp, startTime)
	assert.Equal(t, "resp-456", resp.ID)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, "Fireworks response", resp.Content)
	assert.Equal(t, "fireworks", resp.ProviderID)
	assert.Equal(t, "Fireworks AI", resp.ProviderName)
	assert.Equal(t, 150, resp.TokensUsed)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 100, resp.Metadata["prompt_tokens"])
	assert.Equal(t, 50, resp.Metadata["completion_tokens"])
}

func TestConvertResponseWithToolCalls(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	req := &models.LLMRequest{ID: "req-tools"}
	startTime := time.Now()

	apiResp := &Response{
		ID:    "resp-tools",
		Model: "accounts/fireworks/models/firefunction-v2",
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role: "assistant",
					ToolCalls: []ToolCall{
						{
							ID:   "tc-1",
							Type: "function",
							Function: FunctionCall{
								Name:      "search_db",
								Arguments: `{"query": "test"}`,
							},
						},
					},
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{TotalTokens: 30},
	}

	resp := provider.convertResponse(req, apiResp, startTime)
	assert.Equal(t, "tool_calls", resp.FinishReason)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "tc-1", resp.ToolCalls[0].ID)
	assert.Equal(t, "search_db", resp.ToolCalls[0].Function.Name)
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
		{"Short", "content_filter", 0.5, 0.6},
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
	provider := NewProvider("test-api-key", "", "accounts/fireworks/models/llama-v3p1-70b-instruct")
	assert.Equal(t, "accounts/fireworks/models/llama-v3p1-70b-instruct", provider.GetModel())
}

func TestSetModel(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "accounts/fireworks/models/llama-v3p1-70b-instruct")
	provider.SetModel("accounts/fireworks/models/deepseek-r1")
	assert.Equal(t, "accounts/fireworks/models/deepseek-r1", provider.GetModel())
}

func TestGetName(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	assert.Equal(t, "fireworks", provider.GetName())
}

func TestRetryOnServerError(t *testing.T) {
	t.Parallel()
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := Response{
			ID:      "success",
			Choices: []Choice{{Message: Message{Content: "Success"}, FinishReason: "stop"}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewProviderWithRetry("test-key", server.URL, "", RetryConfig{
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

	provider := NewProvider("test-key", server.URL, "")
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
		"accounts/fireworks/models/llama-v3p1-70b-instruct",
		"accounts/fireworks/models/deepseek-r1",
		"accounts/fireworks/models/qwen2p5-72b-instruct",
		"accounts/fireworks/models/firefunction-v2",
	}

	for _, model := range testModels {
		t.Run(model, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req Request
				_ = json.NewDecoder(r.Body).Decode(&req)
				assert.Equal(t, model, req.Model)

				resp := Response{
					ID:      "test-" + model,
					Model:   model,
					Choices: []Choice{{Message: Message{Content: "Response from model"}, FinishReason: "stop"}},
				}
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			provider := NewProvider("test-key", server.URL, model)
			req := &models.LLMRequest{
				Messages:    []models.Message{{Role: "user", Content: "Test"}},
				ModelParams: models.ModelParameters{},
			}

			resp, err := provider.Complete(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, "Response from model", resp.Content)
		})
	}
}

// Benchmarks

func BenchmarkProvider_ConvertRequest(b *testing.B) {
	provider := NewProvider("test-key", "https://api.fireworks.ai/inference/v1/chat/completions", "accounts/fireworks/models/llama-v3p1-70b-instruct")
	req := &models.LLMRequest{
		ID:     "bench-request",
		Prompt: "You are a helpful assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello, how are you?"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   100,
			Temperature: 0.7,
			TopP:        0.9,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.convertRequest(req)
	}
}

func BenchmarkProvider_CalculateConfidence(b *testing.B) {
	provider := NewProvider("test-key", "", "accounts/fireworks/models/llama-v3p1-70b-instruct")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.calculateConfidence("This is a detailed response with sufficient content to test confidence calculation.", "stop")
	}
}

func BenchmarkProvider_ConvertResponse(b *testing.B) {
	provider := NewProvider("test-key", "", "accounts/fireworks/models/llama-v3p1-70b-instruct")
	req := &models.LLMRequest{
		ID: "bench-request",
	}
	resp := &Response{
		ID:    "resp-1",
		Model: "accounts/fireworks/models/llama-v3p1-70b-instruct",
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "This is a benchmark response with enough content."},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     50,
			CompletionTokens: 30,
			TotalTokens:      80,
		},
	}
	startTime := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.convertResponse(req, resp, startTime)
	}
}
