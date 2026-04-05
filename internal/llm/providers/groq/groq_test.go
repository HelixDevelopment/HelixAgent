package groq

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
	p := NewProvider("gsk_test-key", "", "")
	assert.NotNil(t, p)
	assert.Equal(t, "gsk_test-key", p.apiKey)
	assert.Equal(t, GroqAPIURL, p.baseURL)
	assert.Equal(t, DefaultModel, p.model)
}

func TestNewProviderWithRetry(t *testing.T) {
	t.Parallel()
	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
	}

	p := NewProviderWithRetry("gsk_test-key", "https://custom.api.groq.com", "mixtral-8x7b-32768", retryConfig)

	assert.Equal(t, "gsk_test-key", p.apiKey)
	assert.Equal(t, "https://custom.api.groq.com", p.baseURL)
	assert.Equal(t, "mixtral-8x7b-32768", p.model)
	assert.Equal(t, 5, p.retryConfig.MaxRetries)
}

func TestProvider_Complete(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer gsk_test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		resp := Response{
			ID:      "chatcmpl-groq-123",
			Model:   "llama-3.3-70b-versatile",
			Choices: []Choice{{Index: 0, Message: Message{Role: "assistant", Content: "Hello from Groq!"}, FinishReason: "stop"}},
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
				PromptTime:       0.1,
				CompletionTime:   0.05,
				TotalTime:        0.15,
				QueueTime:        0.01,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewProvider("gsk_test-key", server.URL, "llama-3.3-70b-versatile")

	req := &models.LLMRequest{
		ID:     "req-123",
		Prompt: "You are a helpful assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello!"},
		},
		ModelParams: models.ModelParameters{
			Temperature: 0.7,
			MaxTokens:   100,
		},
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "chatcmpl-groq-123", resp.ID)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, "groq", resp.ProviderID)
	assert.Equal(t, "Groq", resp.ProviderName)
	assert.Equal(t, "Hello from Groq!", resp.Content)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 15, resp.TokensUsed)
	// Check Groq-specific timing metadata
	assert.NotNil(t, resp.Metadata["total_time"])
	assert.NotNil(t, resp.Metadata["queue_time"])
}

func TestProvider_Complete_WithTools(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody Request
		_ = json.NewDecoder(r.Body).Decode(&reqBody)

		assert.Len(t, reqBody.Tools, 1)
		assert.Equal(t, "function", reqBody.Tools[0].Type)
		assert.Equal(t, "search_web", reqBody.Tools[0].Function.Name)

		resp := Response{
			ID:    "chatcmpl-groq-456",
			Model: "llama-3.3-70b-versatile",
			Choices: []Choice{{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: "",
					ToolCalls: []ToolCall{{
						ID:   "call-groq-123",
						Type: "function",
						Function: FunctionCall{
							Name:      "search_web",
							Arguments: `{"query": "latest news"}`,
						},
					}},
				},
				FinishReason: "tool_calls",
			}},
			Usage: Usage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewProvider("gsk_test-key", server.URL, "llama-3.3-70b-versatile")

	req := &models.LLMRequest{
		ID:     "req-456",
		Prompt: "You are a search assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Search for latest news"},
		},
		ModelParams: models.ModelParameters{
			Temperature: 0.5,
		},
		Tools: []models.Tool{{
			Type: "function",
			Function: models.ToolFunction{
				Name:        "search_web",
				Description: "Search the web",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{"type": "string"},
					},
				},
			},
		}},
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "call-groq-123", resp.ToolCalls[0].ID)
	assert.Equal(t, "search_web", resp.ToolCalls[0].Function.Name)
	assert.Equal(t, "tool_calls", resp.FinishReason)
}

func TestProvider_Complete_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"message": "Invalid request"}}`))
	}))
	defer server.Close()

	p := NewProviderWithRetry("gsk_test-key", server.URL, "llama-3.3-70b-versatile", RetryConfig{MaxRetries: 0})

	req := &models.LLMRequest{
		ID:       "req-err",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	ctx := context.Background()
	_, err := p.Complete(ctx, req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestProvider_CompleteStream(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		chunks := []string{
			`{"id":"chatcmpl-stream","choices":[{"delta":{"content":"Fast"}}]}`,
			`{"id":"chatcmpl-stream","choices":[{"delta":{"content":" inference"}}]}`,
			`{"id":"chatcmpl-stream","choices":[{"delta":{"content":" with Groq!"}}]}`,
			"[DONE]",
		}

		for _, chunk := range chunks {
			_, _ = w.Write([]byte("data: " + chunk + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := NewProvider("gsk_test-key", server.URL, "llama-3.3-70b-versatile")

	req := &models.LLMRequest{
		ID:       "req-stream",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	ctx := context.Background()
	ch, err := p.CompleteStream(ctx, req)

	require.NoError(t, err)

	var responses []*models.LLMResponse
	for resp := range ch {
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

	// Create provider and override the models URL check
	p := &Provider{
		apiKey:     "gsk_test-key",
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	// Test with direct HTTP call since HealthCheck uses hardcoded GroqModelsURL
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	httpReq, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	httpReq.Header.Set("Authorization", "Bearer gsk_test-key")

	resp, err := p.httpClient.Do(httpReq)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	p := NewProvider("gsk_test-key", "", "")
	caps := p.GetCapabilities()

	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsTools)
	assert.True(t, caps.SupportsVision)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsReasoning)
	assert.True(t, caps.SupportsCodeCompletion)
	assert.Contains(t, caps.SupportedModels, "llama-3.3-70b-versatile")
	assert.Contains(t, caps.SupportedModels, "mixtral-8x7b-32768")
	assert.Contains(t, caps.SupportedModels, "whisper-large-v3")
	assert.Contains(t, caps.SupportedFeatures, "fast_inference")
	assert.Contains(t, caps.SupportedFeatures, "audio_transcription")
	assert.Equal(t, 131072, caps.Limits.MaxTokens)
	assert.Equal(t, "true", caps.Metadata["fast_inference"])
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
			apiKey:     "gsk_valid-key-12345",
			wantValid:  true,
			wantErrLen: 0,
		},
		{
			name:       "Empty key",
			apiKey:     "",
			wantValid:  false,
			wantErrLen: 1,
		},
		{
			name:       "Invalid prefix",
			apiKey:     "sk-invalid-key",
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
	p := NewProvider("gsk_test-key", "", "llama-3.3-70b-versatile")

	req := &models.LLMRequest{
		ID:     "test-req",
		Prompt: "You are a helpful assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How fast is Groq?"},
		},
		ModelParams: models.ModelParameters{
			Model:         "mixtral-8x7b-32768",
			Temperature:   0.5,
			MaxTokens:     1000,
			TopP:          0.95,
			StopSequences: []string{"END"},
		},
	}

	apiReq := p.convertRequest(req)

	assert.Equal(t, "mixtral-8x7b-32768", apiReq.Model) // Model override
	assert.Len(t, apiReq.Messages, 4)                   // System + 3 messages
	assert.Equal(t, "system", apiReq.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant.", apiReq.Messages[0].Content)
	assert.Equal(t, 0.5, apiReq.Temperature)
	assert.Equal(t, 1000, apiReq.MaxTokens)
	assert.Equal(t, 0.95, apiReq.TopP)
	assert.Equal(t, []string{"END"}, apiReq.Stop)
}

func TestProvider_ConvertRequest_DefaultMaxTokens(t *testing.T) {
	t.Parallel()
	p := NewProvider("gsk_test-key", "", "")

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
	p := NewProvider("gsk_test-key", "", "")

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
			content:      "Groq provides incredibly fast inference, enabling real-time AI applications with sub-second response times.",
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
	p := NewProvider("gsk_test-key", "", "")

	// First attempt should be around initial delay (500ms for Groq)
	delay1 := p.calculateBackoff(1)
	assert.GreaterOrEqual(t, delay1, 450*time.Millisecond)
	assert.LessOrEqual(t, delay1, 600*time.Millisecond)

	// Second attempt should be roughly doubled
	delay2 := p.calculateBackoff(2)
	assert.GreaterOrEqual(t, delay2, 900*time.Millisecond)
	assert.LessOrEqual(t, delay2, 1200*time.Millisecond)
}

func TestProvider_GetSetModel(t *testing.T) {
	t.Parallel()
	p := NewProvider("gsk_test-key", "", "llama-3.3-70b-versatile")
	assert.Equal(t, "llama-3.3-70b-versatile", p.GetModel())

	p.SetModel("mixtral-8x7b-32768")
	assert.Equal(t, "mixtral-8x7b-32768", p.GetModel())
}

func TestProvider_GetName(t *testing.T) {
	t.Parallel()
	p := NewProvider("gsk_test-key", "", "")
	assert.Equal(t, "groq", p.GetName())
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

	p := NewProviderWithRetry("gsk_test-key", server.URL, "llama-3.3-70b-versatile", retryConfig)

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

	p := NewProvider("gsk_test-key", server.URL, "llama-3.3-70b-versatile")

	req := &models.LLMRequest{
		ID:       "req-cancel",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := p.Complete(ctx, req)
	assert.Error(t, err)
}

func TestDefaultRetryConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultRetryConfig()
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 500*time.Millisecond, cfg.InitialDelay) // Groq is fast
	assert.Equal(t, 30*time.Second, cfg.MaxDelay)
	assert.Equal(t, 2.0, cfg.Multiplier)
}

func TestProvider_ServerError_Retry(t *testing.T) {
	t.Parallel()
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resp := Response{
			ID:      "chatcmpl-recovered",
			Choices: []Choice{{Message: Message{Content: "Recovered"}, FinishReason: "stop"}},
			Usage:   Usage{TotalTokens: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	retryConfig := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 5 * time.Millisecond,
		MaxDelay:     50 * time.Millisecond,
		Multiplier:   2.0,
	}

	p := NewProviderWithRetry("gsk_test-key", server.URL, "llama-3.3-70b-versatile", retryConfig)

	req := &models.LLMRequest{
		ID:       "req-503",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "Recovered", resp.Content)
	assert.Equal(t, 2, attempts)
}

func TestProvider_TimingMetadata(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := Response{
			ID:      "chatcmpl-timing",
			Model:   "llama-3.3-70b-versatile",
			Choices: []Choice{{Message: Message{Content: "Fast!"}, FinishReason: "stop"}},
			Usage: Usage{
				PromptTokens:     5,
				CompletionTokens: 2,
				TotalTokens:      7,
				PromptTime:       0.05,
				CompletionTime:   0.02,
				TotalTime:        0.07,
				QueueTime:        0.005,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewProvider("gsk_test-key", server.URL, "llama-3.3-70b-versatile")

	req := &models.LLMRequest{
		ID:       "req-timing",
		Messages: []models.Message{{Role: "user", Content: "Hi"}},
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, 0.05, resp.Metadata["prompt_time"])
	assert.Equal(t, 0.02, resp.Metadata["completion_time"])
	assert.Equal(t, 0.07, resp.Metadata["total_time"])
	assert.Equal(t, 0.005, resp.Metadata["queue_time"])
}

// Benchmarks

func BenchmarkProvider_ConvertRequest(b *testing.B) {
	provider := NewProvider("test-key", "https://api.groq.com/openai/v1/chat/completions", "llama-3.1-70b-versatile")
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
	provider := NewProvider("test-key", "", "llama-3.1-70b-versatile")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.calculateConfidence("This is a detailed response with sufficient content to test confidence calculation.", "stop")
	}
}

func BenchmarkProvider_ConvertResponse(b *testing.B) {
	provider := NewProvider("test-key", "", "llama-3.1-70b-versatile")
	req := &models.LLMRequest{
		ID: "bench-request",
	}
	resp := &Response{
		ID:    "resp-1",
		Model: "llama-3.1-70b-versatile",
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
