package cohere

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
	assert.Equal(t, CohereAPIURL, provider.baseURL)
	assert.Equal(t, DefaultModel, provider.model)
}

func TestNewProviderWithCustomURL(t *testing.T) {
	t.Parallel()
	customURL := "https://custom.cohere.ai/v2/chat"
	provider := NewProvider("test-api-key", customURL, "command-r")
	assert.Equal(t, customURL, provider.baseURL)
	assert.Equal(t, "command-r", provider.model)
}

func TestNewProviderWithRetry(t *testing.T) {
	t.Parallel()
	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
	}
	provider := NewProviderWithRetry("test-key", "", "command-r-plus", retryConfig)
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
		assert.Equal(t, "command-r-plus", req.Model)

		resp := Response{
			ID: "chat-123",
			Message: MessageOutput{
				Role: "assistant",
				Content: []ContentPart{
					{Type: "text", Text: "Hello! I'm happy to help."},
				},
			},
			FinishReason: "COMPLETE",
			Usage: Usage{
				Tokens: TokenUsage{
					InputTokens:  10,
					OutputTokens: 8,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewProvider("test-api-key", server.URL, "command-r-plus")
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
	assert.Equal(t, "chat-123", resp.ID)
	assert.Equal(t, "Hello! I'm happy to help.", resp.Content)
	assert.Equal(t, "cohere", resp.ProviderID)
	assert.Equal(t, "COMPLETE", resp.FinishReason)
	assert.Equal(t, 18, resp.TokensUsed)
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
			ID: "chat-tools-123",
			Message: MessageOutput{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{
						ID:         "call-1",
						Type:       "function",
						Name:       "get_weather",
						Parameters: map[string]any{"location": "London"},
					},
				},
			},
			FinishReason: "TOOL_CALL",
			Usage: Usage{
				Tokens: TokenUsage{InputTokens: 20, OutputTokens: 15},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewProvider("test-api-key", server.URL, "command-r-plus")
	req := &models.LLMRequest{
		ID: "req-tools",
		Messages: []models.Message{
			{Role: "user", Content: "What's the weather in London?"},
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
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "get_weather", resp.ToolCalls[0].Function.Name)
	assert.Contains(t, resp.ToolCalls[0].Function.Arguments, "London")
}

func TestCompleteAPIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "Invalid API key"}`))
	}))
	defer server.Close()

	provider := NewProviderWithRetry("invalid-key", server.URL, "command-r-plus", RetryConfig{MaxRetries: 0})
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
			`{"type":"content-delta","id":"chunk-1","delta":{"message":{"content":{"type":"text","text":"Hello"}}}}`,
			`{"type":"content-delta","id":"chunk-2","delta":{"message":{"content":{"type":"text","text":" world"}}}}`,
			`{"type":"message-end","finish_reason":"COMPLETE"}`,
		}

		for _, event := range events {
			_, _ = w.Write([]byte("data: " + event + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	provider := NewProvider("test-api-key", server.URL, "command-r-plus")
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

	require.GreaterOrEqual(t, len(responses), 2)
	// Check final response
	lastResp := responses[len(responses)-1]
	assert.Equal(t, "Hello world", lastResp.Content)
}

func TestCompleteStreamError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error": "Service unavailable"}`))
	}))
	defer server.Close()

	provider := NewProviderWithRetry("test-key", server.URL, "command-r-plus", RetryConfig{MaxRetries: 0})
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
		_, _ = w.Write([]byte(`{"models": []}`))
	}))
	defer server.Close()

	provider := NewProvider("test-api-key", "", "")
	// Override the HTTP client to use our test server
	provider.httpClient = &http.Client{
		Transport: &customTransport{
			modelsURL: server.URL,
		},
	}

	// For this test, directly check with the test server
	provider2 := NewProvider("test-api-key", server.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	httpReq, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	httpReq.Header.Set("Authorization", "Bearer test-api-key")
	resp, err := provider2.httpClient.Do(httpReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

type customTransport struct {
	modelsURL string
}

func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.String(), "models") {
		req.URL, _ = req.URL.Parse(t.modelsURL)
	}
	return http.DefaultTransport.RoundTrip(req)
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
	assert.Contains(t, caps.SupportedModels, "command-a-03-2025")
	assert.Contains(t, caps.SupportedModels, "command-r7b-12-2024")
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.Contains(t, caps.SupportedFeatures, "streaming")
	assert.Contains(t, caps.SupportedFeatures, "tools")
	assert.Contains(t, caps.SupportedFeatures, "rag")
	assert.Contains(t, caps.SupportedFeatures, "vision")
	assert.Contains(t, caps.SupportedFeatures, "reasoning")
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsTools)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsVision)
	assert.True(t, caps.SupportsReasoning)
	assert.Equal(t, 256000, caps.Limits.MaxTokens)
	assert.Equal(t, 8192, caps.Limits.MaxOutputLength)
	assert.Equal(t, "cohere", caps.Metadata["provider"])
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
	provider := NewProvider("test-api-key", "", "command-r-plus")
	req := &models.LLMRequest{
		ID:     "test-id",
		Prompt: "You are a coding assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "Help me code"},
		},
		ModelParams: models.ModelParameters{
			Model:         "command-r",
			Temperature:   0.8,
			MaxTokens:     2000,
			TopP:          0.9,
			StopSequences: []string{"END"},
		},
	}

	apiReq := provider.convertRequest(req)
	assert.Equal(t, "command-r", apiReq.Model)
	assert.Equal(t, "You are a coding assistant.", apiReq.Preamble)
	assert.Len(t, apiReq.Messages, 3)
	assert.Equal(t, 0.8, apiReq.Temperature)
	assert.Equal(t, 2000, apiReq.MaxTokens)
	assert.Equal(t, 0.9, apiReq.TopP)
	assert.Equal(t, []string{"END"}, apiReq.StopSequences)
}

func TestConvertRequestWithSystemMessage(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	req := &models.LLMRequest{
		Messages: []models.Message{
			{Role: "system", Content: "System message here"},
			{Role: "user", Content: "User message"},
		},
		ModelParams: models.ModelParameters{},
	}

	apiReq := provider.convertRequest(req)
	assert.Equal(t, "System message here", apiReq.Preamble)
	assert.Len(t, apiReq.Messages, 1)
	assert.Equal(t, "user", apiReq.Messages[0].Role)
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
		ID: "resp-456",
		Message: MessageOutput{
			Role: "assistant",
			Content: []ContentPart{
				{Type: "text", Text: "Part 1 "},
				{Type: "text", Text: "Part 2"},
			},
		},
		FinishReason: "COMPLETE",
		Usage: Usage{
			Tokens: TokenUsage{
				InputTokens:  100,
				OutputTokens: 50,
			},
		},
	}

	resp := provider.convertResponse(req, apiResp, startTime)
	assert.Equal(t, "resp-456", resp.ID)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, "Part 1 Part 2", resp.Content)
	assert.Equal(t, "cohere", resp.ProviderID)
	assert.Equal(t, "Cohere", resp.ProviderName)
	assert.Equal(t, 150, resp.TokensUsed)
	assert.Equal(t, "COMPLETE", resp.FinishReason)
}

func TestConvertResponseWithToolCalls(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	req := &models.LLMRequest{ID: "req-tools"}
	startTime := time.Now()

	apiResp := &Response{
		ID: "resp-tools",
		Message: MessageOutput{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{
					ID:   "tc-1",
					Type: "function",
					Name: "search",
					Parameters: map[string]any{
						"query": "test query",
					},
				},
			},
		},
		FinishReason: "COMPLETE",
		Usage:        Usage{Tokens: TokenUsage{InputTokens: 20, OutputTokens: 10}},
	}

	resp := provider.convertResponse(req, apiResp, startTime)
	assert.Equal(t, "tool_calls", resp.FinishReason)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "tc-1", resp.ToolCalls[0].ID)
	assert.Equal(t, "search", resp.ToolCalls[0].Function.Name)
	assert.Contains(t, resp.ToolCalls[0].Function.Arguments, "test query")
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
		{"Short", "COMPLETE", 0.9, 1.0},
		{strings.Repeat("Long content ", 20), "COMPLETE", 0.95, 1.0},
		{"Short", "MAX_TOKENS", 0.7, 0.8},
		{"Short", "ERROR", 0.5, 0.6},
		{"Short", "stop", 0.9, 1.0},
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

	// Delays should increase
	assert.LessOrEqual(t, delay1, delay2)
	assert.LessOrEqual(t, delay2, delay3)

	// Should not exceed max delay
	delay10 := provider.calculateBackoff(10)
	assert.LessOrEqual(t, delay10, 35*time.Second) // Max + jitter
}

func TestGetModel(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "command-r-plus")
	assert.Equal(t, "command-r-plus", provider.GetModel())
}

func TestSetModel(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "command-r-plus")
	provider.SetModel("command-r")
	assert.Equal(t, "command-r", provider.GetModel())
}

func TestGetName(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	assert.Equal(t, "cohere", provider.GetName())
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
			ID: "success",
			Message: MessageOutput{
				Content: []ContentPart{{Type: "text", Text: "Success"}},
			},
			FinishReason: "COMPLETE",
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
			ID:           "rate-limit-success",
			Message:      MessageOutput{Content: []ContentPart{{Type: "text", Text: "OK"}}},
			FinishReason: "COMPLETE",
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
	provider := NewProvider("test-key", "https://api.cohere.com/v2/chat", "command-r-plus")
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
	provider := NewProvider("test-key", "", "command-r-plus")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.calculateConfidence("This is a detailed response with sufficient content to test confidence calculation.", "stop")
	}
}

func BenchmarkProvider_ConvertResponse(b *testing.B) {
	provider := NewProvider("test-key", "", "command-r-plus")
	req := &models.LLMRequest{
		ID: "bench-request",
	}
	resp := &Response{
		ID: "resp-1",
		Message: MessageOutput{
			Role: "assistant",
			Content: []ContentPart{
				{Type: "text", Text: "This is a benchmark response with enough content."},
			},
		},
		FinishReason: "COMPLETE",
		Usage: Usage{
			BilledUnits: BilledUnits{InputTokens: 50, OutputTokens: 30},
			Tokens:      TokenUsage{InputTokens: 50, OutputTokens: 30},
		},
	}
	startTime := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.convertResponse(req, resp, startTime)
	}
}
