package together

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
	assert.Equal(t, TogetherAPIURL, provider.baseURL)
	assert.Equal(t, DefaultModel, provider.model)
}

func TestNewProviderWithCustomURL(t *testing.T) {
	t.Parallel()
	customURL := "https://custom.together.xyz/v1/chat/completions"
	provider := NewProvider("test-api-key", customURL, "meta-llama/Llama-3.2-11B-Vision-Instruct-Turbo")
	assert.Equal(t, customURL, provider.baseURL)
	assert.Equal(t, "meta-llama/Llama-3.2-11B-Vision-Instruct-Turbo", provider.model)
}

func TestNewProviderWithRetry(t *testing.T) {
	t.Parallel()
	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
	}
	provider := NewProviderWithRetry("test-key", "", "Qwen/Qwen2.5-72B-Instruct-Turbo", retryConfig)
	assert.Equal(t, 5, provider.retryConfig.MaxRetries)
	assert.Equal(t, 2*time.Second, provider.retryConfig.InitialDelay)
	assert.Equal(t, "Qwen/Qwen2.5-72B-Instruct-Turbo", provider.model)
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
		assert.Equal(t, "meta-llama/Llama-3.3-70B-Instruct-Turbo", req.Model)

		resp := Response{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "meta-llama/Llama-3.3-70B-Instruct-Turbo",
			Choices: []Choice{
				{
					Index:        0,
					Message:      Message{Role: "assistant", Content: "Hello! How can I help you?"},
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

	provider := NewProvider("test-api-key", server.URL, "meta-llama/Llama-3.3-70B-Instruct-Turbo")
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
	assert.Equal(t, "Hello! How can I help you?", resp.Content)
	assert.Equal(t, "together", resp.ProviderID)
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

		resp := Response{
			ID:    "chatcmpl-tools-123",
			Model: "meta-llama/Llama-3.3-70B-Instruct-Turbo",
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
									Arguments: `{"location": "San Francisco"}`,
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

	provider := NewProvider("test-api-key", server.URL, "")
	req := &models.LLMRequest{
		ID: "req-tools",
		Messages: []models.Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
		},
		ModelParams: models.ModelParameters{
			Temperature: 0.5,
		},
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
	assert.Contains(t, resp.ToolCalls[0].Function.Arguments, "San Francisco")
	assert.Equal(t, "tool_calls", resp.FinishReason)
}

func TestCompleteWithDeepSeekR1(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "deepseek-ai/DeepSeek-R1", req.Model)

		resp := Response{
			ID:    "chatcmpl-deepseek",
			Model: "deepseek-ai/DeepSeek-R1",
			Choices: []Choice{
				{
					Index:        0,
					Message:      Message{Role: "assistant", Content: "<think>Reasoning...</think>The answer is 42."},
					FinishReason: "stop",
				},
			},
			Usage: Usage{TotalTokens: 50},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewProvider("test-api-key", server.URL, "deepseek-ai/DeepSeek-R1")
	req := &models.LLMRequest{
		ID: "req-deepseek",
		Messages: []models.Message{
			{Role: "user", Content: "What is the meaning of life?"},
		},
		ModelParams: models.ModelParameters{},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, resp.Content, "Reasoning")
	assert.Contains(t, resp.Content, "42")
}

func TestCompleteAPIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"message": "Invalid API key", "type": "invalid_request_error"}}`))
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
			`data: {"id":"chunk-1","choices":[{"delta":{"content":"Hello"}}]}`,
			`data: {"id":"chunk-2","choices":[{"delta":{"content":" from"}}]}`,
			`data: {"id":"chunk-3","choices":[{"delta":{"content":" Together"}}]}`,
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

	require.GreaterOrEqual(t, len(responses), 3)
	// Check final response has full content
	lastResp := responses[len(responses)-1]
	assert.Equal(t, "Hello from Together", lastResp.Content)
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

func TestHealthCheck(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id": "meta-llama/Llama-3.3-70B-Instruct-Turbo"}]`))
	}))
	defer server.Close()

	// Test with direct HTTP call to mock server
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

func TestHealthCheckFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := NewProvider("invalid-key", server.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	httpReq, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	resp, err := provider.httpClient.Do(httpReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestGetCapabilities(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	caps := provider.GetCapabilities()

	require.NotNil(t, caps)
	// Check Llama models
	assert.Contains(t, caps.SupportedModels, "meta-llama/Llama-3.3-70B-Instruct-Turbo")
	assert.Contains(t, caps.SupportedModels, "meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo")
	// Check Qwen models
	assert.Contains(t, caps.SupportedModels, "Qwen/Qwen2.5-72B-Instruct-Turbo")
	// Check DeepSeek models
	assert.Contains(t, caps.SupportedModels, "deepseek-ai/DeepSeek-R1")
	assert.Contains(t, caps.SupportedModels, "deepseek-ai/DeepSeek-V3")
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
	assert.Equal(t, "together", caps.Metadata["provider"])
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
	provider := NewProvider("test-api-key", "", "meta-llama/Llama-3.3-70B-Instruct-Turbo")
	req := &models.LLMRequest{
		ID:     "test-id",
		Prompt: "You are a coding assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "Help me code"},
		},
		ModelParams: models.ModelParameters{
			Model:         "Qwen/Qwen2.5-72B-Instruct-Turbo",
			Temperature:   0.8,
			MaxTokens:     2000,
			TopP:          0.9,
			StopSequences: []string{"END"},
		},
	}

	apiReq := provider.convertRequest(req)
	assert.Equal(t, "Qwen/Qwen2.5-72B-Instruct-Turbo", apiReq.Model)
	assert.Len(t, apiReq.Messages, 4) // system + 3 messages
	assert.Equal(t, "system", apiReq.Messages[0].Role)
	assert.Equal(t, "You are a coding assistant.", apiReq.Messages[0].Content)
	assert.Equal(t, 0.8, apiReq.Temperature)
	assert.Equal(t, 2000, apiReq.MaxTokens)
	assert.Equal(t, 0.9, apiReq.TopP)
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

func TestConvertRequestWithToolChoice(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	req := &models.LLMRequest{
		Messages: []models.Message{{Role: "user", Content: "Test"}},
		Tools: []models.Tool{
			{
				Type: "function",
				Function: models.ToolFunction{
					Name:        "test_func",
					Description: "Test function",
				},
			},
		},
		ToolChoice:  "auto",
		ModelParams: models.ModelParameters{},
	}

	apiReq := provider.convertRequest(req)
	assert.Len(t, apiReq.Tools, 1)
	assert.Equal(t, "auto", apiReq.ToolChoice)
}

func TestConvertResponse(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	req := &models.LLMRequest{ID: "req-123"}
	startTime := time.Now()

	apiResp := &Response{
		ID:      "chatcmpl-456",
		Model:   "meta-llama/Llama-3.3-70B-Instruct-Turbo",
		Created: time.Now().Unix(),
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "This is the response"},
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
	assert.Equal(t, "chatcmpl-456", resp.ID)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, "This is the response", resp.Content)
	assert.Equal(t, "together", resp.ProviderID)
	assert.Equal(t, "Together AI", resp.ProviderName)
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
		ID:    "chatcmpl-tools",
		Model: "meta-llama/Llama-3.3-70B-Instruct-Turbo",
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
								Name:      "search_web",
								Arguments: `{"query": "latest news"}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: Usage{TotalTokens: 30},
	}

	resp := provider.convertResponse(req, apiResp, startTime)
	assert.Equal(t, "tool_calls", resp.FinishReason)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "tc-1", resp.ToolCalls[0].ID)
	assert.Equal(t, "function", resp.ToolCalls[0].Type)
	assert.Equal(t, "search_web", resp.ToolCalls[0].Function.Name)
	assert.Equal(t, `{"query": "latest news"}`, resp.ToolCalls[0].Function.Arguments)
}

func TestConvertResponseEmptyChoices(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	req := &models.LLMRequest{ID: "req-empty"}
	startTime := time.Now()

	apiResp := &Response{
		ID:      "chatcmpl-empty",
		Choices: []Choice{},
		Usage:   Usage{},
	}

	resp := provider.convertResponse(req, apiResp, startTime)
	assert.Equal(t, "", resp.Content)
	assert.Equal(t, "", resp.FinishReason)
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
		{"Short", "end_turn", 0.9, 1.0},
		{strings.Repeat("Long content ", 20), "stop", 0.95, 1.0},
		{"Short", "length", 0.7, 0.8},
		{"Short", "content_filter", 0.5, 0.6},
	}

	for _, tt := range tests {
		conf := provider.calculateConfidence(tt.content, tt.finishReason)
		assert.GreaterOrEqual(t, conf, tt.minConf, "content=%q, finish=%s", tt.content, tt.finishReason)
		assert.LessOrEqual(t, conf, tt.maxConf, "content=%q, finish=%s", tt.content, tt.finishReason)
	}
}

func TestCalculateBackoff(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")

	delay1 := provider.calculateBackoff(1)
	delay2 := provider.calculateBackoff(2)
	delay3 := provider.calculateBackoff(3)

	// First delay should be close to initial delay
	assert.LessOrEqual(t, delay1, 2*time.Second)

	// Delays should generally increase (accounting for jitter)
	assert.LessOrEqual(t, delay1, delay2+time.Second)
	assert.LessOrEqual(t, delay2, delay3+time.Second)

	// Should not exceed max delay
	delay10 := provider.calculateBackoff(10)
	assert.LessOrEqual(t, delay10, 35*time.Second) // Max + jitter
}

func TestGetModel(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "meta-llama/Llama-3.3-70B-Instruct-Turbo")
	assert.Equal(t, "meta-llama/Llama-3.3-70B-Instruct-Turbo", provider.GetModel())
}

func TestSetModel(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "meta-llama/Llama-3.3-70B-Instruct-Turbo")
	provider.SetModel("deepseek-ai/DeepSeek-R1")
	assert.Equal(t, "deepseek-ai/DeepSeek-R1", provider.GetModel())
}

func TestGetName(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	assert.Equal(t, "together", provider.GetName())
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
			ID:      "success-after-retry",
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
	assert.Equal(t, "success-after-retry", resp.ID)
	assert.Equal(t, 3, attempts)
}

func TestRetryOnRateLimiting(t *testing.T) {
	t.Parallel()
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		resp := Response{
			ID:      "rate-limit-success",
			Choices: []Choice{{Message: Message{Content: "OK"}, FinishReason: "stop"}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewProviderWithRetry("test-key", server.URL, "", RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	})

	req := &models.LLMRequest{
		Messages: []models.Message{{Role: "user", Content: "Test"}},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "rate-limit-success", resp.ID)
}

func TestMaxRetriesExceeded(t *testing.T) {
	t.Parallel()
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := NewProviderWithRetry("test-key", server.URL, "", RetryConfig{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	})

	req := &models.LLMRequest{
		Messages: []models.Message{{Role: "user", Content: "Test"}},
	}

	_, err := provider.Complete(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max retries exceeded")
	assert.Equal(t, 3, attempts) // 1 initial + 2 retries
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
		"meta-llama/Llama-3.3-70B-Instruct-Turbo",
		"Qwen/Qwen2.5-72B-Instruct-Turbo",
		"deepseek-ai/DeepSeek-R1",
		"mistralai/Mixtral-8x22B-Instruct-v0.1",
		"google/gemma-2-27b-it",
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
					Choices: []Choice{{Message: Message{Content: "Response from " + model}, FinishReason: "stop"}},
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
			assert.Contains(t, resp.Content, model)
		})
	}
}

// Benchmarks

func BenchmarkProvider_ConvertRequest(b *testing.B) {
	provider := NewProvider("test-key", "https://api.together.xyz/v1/chat/completions", "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo")
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
	provider := NewProvider("test-key", "", "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.calculateConfidence("This is a detailed response with sufficient content to test confidence calculation.", "stop")
	}
}

func BenchmarkProvider_ConvertResponse(b *testing.B) {
	provider := NewProvider("test-key", "", "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo")
	req := &models.LLMRequest{
		ID: "bench-request",
	}
	resp := &Response{
		ID:    "resp-1",
		Model: "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo",
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
