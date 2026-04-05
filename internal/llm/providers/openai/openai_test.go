package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
)

func TestNewProvider(t *testing.T) {
	t.Parallel()
	p := NewProvider("test-key", "", "")
	assert.NotNil(t, p)
	assert.Equal(t, "test-key", p.apiKey)
	assert.Equal(t, OpenAIAPIURL, p.baseURL)
	assert.Equal(t, DefaultModel, p.model)
}

func TestNewProviderWithCustomValues(t *testing.T) {
	t.Parallel()
	p := NewProvider("test-key", "https://custom.api.com", "gpt-4")
	assert.Equal(t, "https://custom.api.com", p.baseURL)
	assert.Equal(t, "gpt-4", p.model)
}

func TestNewProviderWithRetry(t *testing.T) {
	t.Parallel()
	config := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
	}
	p := NewProviderWithRetry("test-key", "", "", config)
	assert.Equal(t, 5, p.retryConfig.MaxRetries)
	assert.Equal(t, 2*time.Second, p.retryConfig.InitialDelay)
}

func TestDefaultRetryConfig(t *testing.T) {
	t.Parallel()
	config := DefaultRetryConfig()
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, time.Second, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
}

func TestProvider_Complete(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-key")

		resp := Response{
			ID:    "chatcmpl-123",
			Model: "gpt-4o",
			Choices: []Choice{
				{
					Index: 0,
					Message: Message{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 8,
				TotalTokens:      18,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewProvider("test-key", server.URL, "gpt-4o")
	resp, err := p.Complete(context.Background(), &models.LLMRequest{
		ID: "req-123",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			Temperature: 0.7,
			MaxTokens:   100,
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "chatcmpl-123", resp.ID)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, "openai", resp.ProviderID)
	assert.Equal(t, "OpenAI", resp.ProviderName)
	assert.Equal(t, "Hello! How can I help you?", resp.Content)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 18, resp.TokensUsed)
}

func TestProvider_Complete_WithTools(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)

		assert.Len(t, req.Tools, 1)
		assert.Equal(t, "get_weather", req.Tools[0].Function.Name)

		resp := Response{
			ID:    "chatcmpl-456",
			Model: "gpt-4o",
			Choices: []Choice{
				{
					Index: 0,
					Message: Message{
						Role:    "assistant",
						Content: "",
						ToolCalls: []ToolCall{
							{
								ID:   "call_123",
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
			Usage: Usage{TotalTokens: 20},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewProvider("test-key", server.URL, "gpt-4o")
	resp, err := p.Complete(context.Background(), &models.LLMRequest{
		ID: "req-456",
		Messages: []models.Message{
			{Role: "user", Content: "What's the weather in SF?"},
		},
		Tools: []models.Tool{
			{
				Type: "function",
				Function: models.ToolFunction{
					Name:        "get_weather",
					Description: "Get weather for a location",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type":        "string",
								"description": "The city name",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "tool_calls", resp.FinishReason)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "get_weather", resp.ToolCalls[0].Function.Name)
	assert.Equal(t, "call_123", resp.ToolCalls[0].ID)
}

func TestProvider_Complete_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"message": "Invalid request"}}`))
	}))
	defer server.Close()

	p := NewProviderWithRetry("test-key", server.URL, "gpt-4o", RetryConfig{MaxRetries: 0})
	_, err := p.Complete(context.Background(), &models.LLMRequest{
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestProvider_Complete_WithSystemPrompt(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Verify system message was added
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "You are a helpful assistant.", req.Messages[0].Content)

		resp := Response{
			ID:      "chatcmpl-123",
			Model:   "gpt-4o",
			Choices: []Choice{{Message: Message{Content: "Hi!"}, FinishReason: "stop"}},
			Usage:   Usage{TotalTokens: 10},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewProvider("test-key", server.URL, "gpt-4o")
	_, err := p.Complete(context.Background(), &models.LLMRequest{
		Prompt:   "You are a helpful assistant.",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	})

	require.NoError(t, err)
}

func TestProvider_CompleteStream(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.True(t, req.Stream)

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		chunks := []string{
			`{"id":"chatcmpl-123","choices":[{"delta":{"content":"Hello"}}]}`,
			`{"id":"chatcmpl-123","choices":[{"delta":{"content":" World"}}]}`,
			"[DONE]",
		}

		for _, chunk := range chunks {
			_, _ = w.Write([]byte("data: " + chunk + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := NewProvider("test-key", server.URL, "gpt-4o")
	respChan, err := p.CompleteStream(context.Background(), &models.LLMRequest{
		ID:       "req-stream",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	})

	require.NoError(t, err)

	var responses []*models.LLMResponse
	for resp := range respChan {
		responses = append(responses, resp)
	}

	assert.GreaterOrEqual(t, len(responses), 1)
	lastResp := responses[len(responses)-1]
	assert.Equal(t, "stop", lastResp.FinishReason)
}

func TestProvider_HealthCheck(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": []}`))
	}))
	defer server.Close()

	// Create provider and verify it can be created
	p := NewProvider("test-key", server.URL, "gpt-4o")
	assert.NotNil(t, p)
}

func TestProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	p := NewProvider("test-key", "", "")
	caps := p.GetCapabilities()

	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsTools)
	assert.True(t, caps.SupportsVision)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsReasoning)
	assert.Contains(t, caps.SupportedModels, "gpt-4o")
	assert.Contains(t, caps.SupportedModels, "o1")
	assert.Contains(t, caps.SupportedFeatures, "streaming")
	assert.Contains(t, caps.SupportedFeatures, "tools")
	assert.Equal(t, 128000, caps.Limits.MaxTokens)
}

func TestProvider_ValidateConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		apiKey     string
		wantValid  bool
		wantErrLen int
	}{
		{
			name:       "Valid key",
			apiKey:     "sk-test-key-12345",
			wantValid:  true,
			wantErrLen: 0,
		},
		{
			name:       "Empty key",
			apiKey:     "",
			wantValid:  false,
			wantErrLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			p := NewProvider(tt.apiKey, "", "")
			valid, errs := p.ValidateConfig(nil)
			assert.Equal(t, tt.wantValid, valid)
			assert.Len(t, errs, tt.wantErrLen)
		})
	}
}

func TestProvider_ConvertRequest(t *testing.T) {
	t.Parallel()
	p := NewProvider("test-key", "", "gpt-4o")

	req := &models.LLMRequest{
		ID:     "test-req",
		Prompt: "You are a helpful assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
		ModelParams: models.ModelParameters{
			Model:         "gpt-4-turbo",
			Temperature:   0.8,
			MaxTokens:     500,
			TopP:          0.9,
			StopSequences: []string{"\n\n"},
		},
	}

	apiReq := p.convertRequest(req)

	assert.Equal(t, "gpt-4-turbo", apiReq.Model) // Model override
	assert.Len(t, apiReq.Messages, 4)            // System + 3 messages
	assert.Equal(t, "system", apiReq.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant.", apiReq.Messages[0].Content)
	assert.Equal(t, 0.8, apiReq.Temperature)
	assert.Equal(t, 500, apiReq.MaxTokens)
	assert.Equal(t, 0.9, apiReq.TopP)
	assert.Equal(t, []string{"\n\n"}, apiReq.Stop)
}

func TestProvider_ConvertRequest_DefaultMaxTokens(t *testing.T) {
	t.Parallel()
	p := NewProvider("test-key", "", "gpt-4o")

	req := &models.LLMRequest{
		ID:       "test-req",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
		ModelParams: models.ModelParameters{
			MaxTokens: 0, // Not set
		},
	}

	apiReq := p.convertRequest(req)
	assert.Equal(t, 4096, apiReq.MaxTokens) // Default value
}

func TestProvider_CalculateConfidence(t *testing.T) {
	t.Parallel()
	p := NewProvider("test-key", "", "")

	tests := []struct {
		name         string
		content      string
		finishReason string
		minConf      float64
		maxConf      float64
	}{
		{
			name:         "Stop finish",
			content:      "Short response",
			finishReason: "stop",
			minConf:      0.9,
			maxConf:      1.0,
		},
		{
			name:         "Length finish",
			content:      "Short",
			finishReason: "length",
			minConf:      0.7,
			maxConf:      0.8,
		},
		{
			name:         "Content filter",
			content:      "Filtered",
			finishReason: "content_filter",
			minConf:      0.5,
			maxConf:      0.6,
		},
		{
			name:         "Long content bonus",
			content:      "This is a much longer response that should get a small confidence bonus for having more content to evaluate.",
			finishReason: "stop",
			minConf:      0.95,
			maxConf:      1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			conf := p.calculateConfidence(tt.content, tt.finishReason)
			assert.GreaterOrEqual(t, conf, tt.minConf)
			assert.LessOrEqual(t, conf, tt.maxConf)
		})
	}
}

func TestProvider_CalculateBackoff(t *testing.T) {
	t.Parallel()
	p := NewProvider("test-key", "", "")

	// First attempt should be around initial delay
	delay1 := p.calculateBackoff(1)
	assert.GreaterOrEqual(t, delay1, 900*time.Millisecond)
	assert.LessOrEqual(t, delay1, 1200*time.Millisecond)

	// Second attempt should be roughly doubled
	delay2 := p.calculateBackoff(2)
	assert.GreaterOrEqual(t, delay2, 1800*time.Millisecond)
	assert.LessOrEqual(t, delay2, 2400*time.Millisecond)
}

func TestProvider_GetSetModel(t *testing.T) {
	t.Parallel()
	p := NewProvider("test-key", "", "gpt-4")
	assert.Equal(t, "gpt-4", p.GetModel())

	p.SetModel("gpt-4o")
	assert.Equal(t, "gpt-4o", p.GetModel())
}

func TestProvider_GetName(t *testing.T) {
	t.Parallel()
	p := NewProvider("test-key", "", "")
	assert.Equal(t, "openai", p.GetName())
}

func TestProvider_SetOrganization(t *testing.T) {
	t.Parallel()
	p := NewProvider("test-key", "", "")
	assert.Empty(t, p.organization)

	p.SetOrganization("org-123")
	assert.Equal(t, "org-123", p.organization)
}

func TestProvider_Retry(t *testing.T) {
	t.Parallel()
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		resp := Response{
			ID:      "chatcmpl-retry",
			Choices: []Choice{{Message: Message{Content: "Success after retry"}, FinishReason: "stop"}},
			Usage:   Usage{TotalTokens: 10},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	p := NewProviderWithRetry("test-key", server.URL, "gpt-4o", retryConfig)

	req := &models.LLMRequest{
		ID:       "req-retry",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "Success after retry", resp.Content)
	assert.Equal(t, 3, attempts)
}

func TestProvider_ContextCancellation(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewProvider("test-key", server.URL, "gpt-4o")

	req := &models.LLMRequest{
		ID:       "req-cancel",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := p.Complete(ctx, req)
	assert.Error(t, err)
}

// Benchmarks

func BenchmarkProvider_ConvertRequest(b *testing.B) {
	provider := NewProvider("test-key", "https://api.openai.com/v1/chat/completions", "gpt-4o")
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
	provider := NewProvider("test-key", "", "gpt-4o")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.calculateConfidence("This is a detailed response with sufficient content to test confidence calculation.", "stop")
	}
}

func BenchmarkProvider_ConvertResponse(b *testing.B) {
	provider := NewProvider("test-key", "", "gpt-4o")
	req := &models.LLMRequest{
		ID: "bench-request",
	}
	resp := &Response{
		ID:    "resp-1",
		Model: "gpt-4o",
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
