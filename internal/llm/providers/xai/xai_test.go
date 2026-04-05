package xai

import (
	"context"
	"encoding/json"
	"fmt"
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
	p := NewProvider("xai-test-key", "", "")
	assert.NotNil(t, p)
	assert.Equal(t, "xai-test-key", p.apiKey)
	assert.Equal(t, XAIAPIBaseURL, p.baseURL)
	assert.Equal(t, DefaultModel, p.model)
	assert.Equal(t, "us-east-1", p.region)
}

func TestNewProviderWithRegion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		region         string
		expectedURL    string
		expectedRegion string
	}{
		{
			name:           "US region",
			region:         "us-east-1",
			expectedURL:    XAIAPIBaseURL,
			expectedRegion: "us-east-1",
		},
		{
			name:           "EU region",
			region:         "eu-west-1",
			expectedURL:    XAIAPIEUBaseURL,
			expectedRegion: "eu-west-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			p := NewProviderWithRegion("xai-test-key", "grok-3", tt.region)
			assert.Equal(t, tt.expectedURL, p.baseURL)
			assert.Equal(t, tt.expectedRegion, p.region)
		})
	}
}

func TestNewProviderWithRetry(t *testing.T) {
	t.Parallel()
	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
	}

	p := NewProviderWithRetry("xai-test-key", "https://custom.api.x.ai/v1", "grok-4", "eu-west-1", retryConfig)

	assert.Equal(t, "xai-test-key", p.apiKey)
	assert.Equal(t, "https://custom.api.x.ai/v1", p.baseURL)
	assert.Equal(t, "grok-4", p.model)
	assert.Equal(t, "eu-west-1", p.region)
	assert.Equal(t, 5, p.retryConfig.MaxRetries)
}

func TestProvider_Complete(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/chat/completions")
		assert.Equal(t, "Bearer xai-test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		resp := Response{
			ID:      "chatcmpl-123",
			Model:   "grok-3-beta",
			Choices: []Choice{{Index: 0, Message: Message{Role: "assistant", Content: "Hello from Grok!"}, FinishReason: "stop"}},
			Usage:   Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewProvider("xai-test-key", server.URL, "grok-3-beta")

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
	assert.Equal(t, "chatcmpl-123", resp.ID)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, "xai", resp.ProviderID)
	assert.Equal(t, "xAI", resp.ProviderName)
	assert.Equal(t, "Hello from Grok!", resp.Content)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 15, resp.TokensUsed)
}

func TestProvider_Complete_WithTools(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody Request
		_ = json.NewDecoder(r.Body).Decode(&reqBody)

		assert.Len(t, reqBody.Tools, 1)
		assert.Equal(t, "function", reqBody.Tools[0].Type)
		assert.Equal(t, "get_weather", reqBody.Tools[0].Function.Name)

		resp := Response{
			ID:    "chatcmpl-456",
			Model: "grok-3",
			Choices: []Choice{{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: "",
					ToolCalls: []ToolCall{{
						ID:   "call-123",
						Type: "function",
						Function: FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location": "San Francisco"}`,
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

	p := NewProvider("xai-test-key", server.URL, "grok-3")

	req := &models.LLMRequest{
		ID:     "req-456",
		Prompt: "You are a weather assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
		},
		ModelParams: models.ModelParameters{
			Temperature: 0.5,
		},
		Tools: []models.Tool{{
			Type: "function",
			Function: models.ToolFunction{
				Name:        "get_weather",
				Description: "Get current weather",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{"type": "string"},
					},
				},
			},
		}},
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "call-123", resp.ToolCalls[0].ID)
	assert.Equal(t, "get_weather", resp.ToolCalls[0].Function.Name)
	assert.Equal(t, "tool_calls", resp.FinishReason)
}

func TestProvider_Complete_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"message": "Invalid request"}}`))
	}))
	defer server.Close()

	p := NewProviderWithRetry("xai-test-key", server.URL, "grok-3", "us-east-1", RetryConfig{MaxRetries: 0})

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
			`{"id":"chatcmpl-stream","choices":[{"delta":{"content":"Hello"}}]}`,
			`{"id":"chatcmpl-stream","choices":[{"delta":{"content":" from"}}]}`,
			`{"id":"chatcmpl-stream","choices":[{"delta":{"content":" Grok!"}}]}`,
			"[DONE]",
		}

		for _, chunk := range chunks {
			_, _ = w.Write([]byte("data: " + chunk + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	p := NewProvider("xai-test-key", server.URL, "grok-3")

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
	// Last response should have the full content
	lastResp := responses[len(responses)-1]
	assert.Equal(t, "stop", lastResp.FinishReason)
}

func TestProvider_HealthCheck(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/models")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": []}`))
	}))
	defer server.Close()

	p := NewProvider("xai-test-key", server.URL, "grok-3")
	err := p.HealthCheck()
	assert.NoError(t, err)
}

func TestProvider_HealthCheck_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	p := NewProvider("invalid-key", server.URL, "grok-3")
	err := p.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	p := NewProviderWithRegion("xai-test-key", "grok-3", "eu-west-1")
	caps := p.GetCapabilities()

	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsTools)
	assert.True(t, caps.SupportsVision)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsReasoning)
	assert.Contains(t, caps.SupportedModels, "grok-3")
	assert.Contains(t, caps.SupportedModels, "grok-4")
	assert.Contains(t, caps.SupportedFeatures, "web_search")
	assert.Contains(t, caps.SupportedFeatures, "x_search")
	assert.Equal(t, 2000000, caps.Limits.MaxTokens)
	assert.Equal(t, "eu-west-1", caps.Metadata["region"])
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
			apiKey:     "xai-valid-key-12345",
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
	p := NewProvider("xai-test-key", "", "grok-3-beta")

	req := &models.LLMRequest{
		ID:     "test-req",
		Prompt: "You are a helpful assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
		ModelParams: models.ModelParameters{
			Model:         "grok-4",
			Temperature:   0.8,
			MaxTokens:     500,
			TopP:          0.9,
			StopSequences: []string{"\n\n"},
		},
	}

	apiReq := p.convertRequest(req)

	assert.Equal(t, "grok-4", apiReq.Model) // Model override
	assert.Len(t, apiReq.Messages, 4)       // System + 3 messages
	assert.Equal(t, "system", apiReq.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant.", apiReq.Messages[0].Content)
	assert.Equal(t, 0.8, apiReq.Temperature)
	assert.Equal(t, 500, apiReq.MaxTokens)
	assert.Equal(t, 0.9, apiReq.TopP)
	assert.Equal(t, []string{"\n\n"}, apiReq.Stop)
}

func TestProvider_CalculateConfidence(t *testing.T) {
	t.Parallel()
	p := NewProvider("xai-test-key", "", "")

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
	p := NewProvider("xai-test-key", "", "")

	// First attempt should be around initial delay
	delay1 := p.calculateBackoff(1)
	assert.GreaterOrEqual(t, delay1, 900*time.Millisecond)
	assert.LessOrEqual(t, delay1, 1200*time.Millisecond)

	// Second attempt should be roughly doubled
	delay2 := p.calculateBackoff(2)
	assert.GreaterOrEqual(t, delay2, 1800*time.Millisecond)
	assert.LessOrEqual(t, delay2, 2400*time.Millisecond)
}

func TestProvider_SetRegion(t *testing.T) {
	t.Parallel()
	p := NewProvider("xai-test-key", "", "")
	assert.Equal(t, "us-east-1", p.region)
	assert.Equal(t, XAIAPIBaseURL, p.baseURL)

	p.SetRegion("eu-west-1")
	assert.Equal(t, "eu-west-1", p.region)
	assert.Equal(t, XAIAPIEUBaseURL, p.baseURL)

	p.SetRegion("us-east-1")
	assert.Equal(t, "us-east-1", p.region)
	assert.Equal(t, XAIAPIBaseURL, p.baseURL)
}

func TestProvider_GetSetModel(t *testing.T) {
	t.Parallel()
	p := NewProvider("xai-test-key", "", "grok-3")
	assert.Equal(t, "grok-3", p.GetModel())

	p.SetModel("grok-4")
	assert.Equal(t, "grok-4", p.GetModel())
}

func TestProvider_GetName(t *testing.T) {
	t.Parallel()
	p := NewProvider("xai-test-key", "", "")
	assert.Equal(t, "xai", p.GetName())
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

	p := NewProviderWithRetry("xai-test-key", server.URL, "grok-3", "us-east-1", retryConfig)

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

	p := NewProvider("xai-test-key", server.URL, "grok-3")

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
	assert.Equal(t, 1*time.Second, cfg.InitialDelay)
	assert.Equal(t, 30*time.Second, cfg.MaxDelay)
	assert.Equal(t, 2.0, cfg.Multiplier)
}

// Helper function to avoid fmt shadowing
func errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkXAIProvider_ConvertRequest(b *testing.B) {
	provider := NewProvider("test-key", "", "")
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

func BenchmarkXAIProvider_CalculateConfidence(b *testing.B) {
	provider := NewProvider("test-key", "", "")
	content := "This is a sample response from the xAI model for confidence scoring."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.calculateConfidence(content, "stop")
	}
}

func BenchmarkXAIProvider_GetCapabilities(b *testing.B) {
	provider := NewProvider("test-key", "", "")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.GetCapabilities()
	}
}

func BenchmarkXAIProvider_ConvertResponse(b *testing.B) {
	provider := NewProvider("test-key", "", "grok-2")
	req := &models.LLMRequest{
		ID: "bench-request",
	}
	resp := &Response{
		ID:    "resp-1",
		Model: "grok-2",
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
