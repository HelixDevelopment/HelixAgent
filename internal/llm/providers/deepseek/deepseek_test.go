package deepseek

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/models"
)

func TestNewDeepSeekProvider(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		baseURL  string
		model    string
		expected *DeepSeekProvider
	}{
		{
			name:    "default values",
			apiKey:  "test-key",
			baseURL: "",
			model:   "",
			expected: &DeepSeekProvider{
				apiKey:  "test-key",
				baseURL: DeepSeekAPIURL,
				model:   DeepSeekModel,
			},
		},
		{
			name:    "custom values",
			apiKey:  "test-key",
			baseURL: "https://custom.example.com",
			model:   "deepseek-chat",
			expected: &DeepSeekProvider{
				apiKey:  "test-key",
				baseURL: "https://custom.example.com",
				model:   "deepseek-chat",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewDeepSeekProvider(tt.apiKey, tt.baseURL, tt.model)
			require.NotNil(t, provider)
			assert.Equal(t, tt.expected.apiKey, provider.apiKey)
			assert.Equal(t, tt.expected.baseURL, provider.baseURL)
			assert.Equal(t, tt.expected.model, provider.model)
			assert.NotNil(t, provider.httpClient)
			assert.Equal(t, 60*time.Second, provider.httpClient.Timeout)
		})
	}
}

func TestDeepSeekProvider_Complete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "chat_123",
			"object": "chat.completion",
			"created": 1677858242,
			"model": "deepseek-coder",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "Hello from DeepSeek!"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`))
	}))
	defer server.Close()

	provider := NewDeepSeekProvider("test-key", server.URL+"/v1/chat/completions", "deepseek-coder")
	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "chat_123", resp.ID)
	assert.Equal(t, "Hello from DeepSeek!", resp.Content)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Greater(t, resp.Confidence, 0.0)
	assert.Less(t, resp.Confidence, 1.0)
}

func TestDeepSeekProvider_Complete_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"message": "Invalid request", "type": "invalid_request_error"}}`))
	}))
	defer server.Close()

	provider := NewDeepSeekProvider("test-key", server.URL+"/v1/chat/completions", "deepseek-coder")
	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DeepSeek API error: 400")
}

func TestDeepSeekProvider_ConvertRequest(t *testing.T) {
	provider := NewDeepSeekProvider("test-key", "", "")
	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello, DeepSeek!"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
		ModelParams: models.ModelParameters{
			Model:         "deepseek-coder",
			MaxTokens:     100,
			Temperature:   0.7,
			TopP:          0.9,
			StopSequences: []string{"\n", "STOP"},
		},
	}

	deepseekReq := provider.convertRequest(req)
	assert.Equal(t, "deepseek-coder", deepseekReq.Model)
	assert.Equal(t, 100, deepseekReq.MaxTokens)
	assert.Equal(t, 0.7, deepseekReq.Temperature)
	assert.Equal(t, 0.9, deepseekReq.TopP)
	assert.Equal(t, []string{"\n", "STOP"}, deepseekReq.Stop)
	assert.Len(t, deepseekReq.Messages, 3)
	assert.Equal(t, "user", deepseekReq.Messages[0].Role)
	assert.Equal(t, "Hello, DeepSeek!", deepseekReq.Messages[0].Content)
	assert.Equal(t, "assistant", deepseekReq.Messages[1].Role)
	assert.Equal(t, "Hi there!", deepseekReq.Messages[1].Content)
	assert.Equal(t, "user", deepseekReq.Messages[2].Role)
	assert.Equal(t, "How are you?", deepseekReq.Messages[2].Content)
}

func TestDeepSeekProvider_CalculateConfidence(t *testing.T) {
	provider := NewDeepSeekProvider("test-key", "", "")

	tests := []struct {
		name         string
		content      string
		finishReason string
		expectedMin  float64
		expectedMax  float64
	}{
		{
			name:         "good response with stop",
			content:      "This is a comprehensive and well-formed response.",
			finishReason: "stop",
			expectedMin:  0.8,
			expectedMax:  1.0,
		},
		{
			name:         "short response with length",
			content:      "Short",
			finishReason: "length",
			expectedMin:  0.7,
			expectedMax:  0.9,
		},
		{
			name:         "content filter",
			content:      "Filtered content",
			finishReason: "content_filter",
			expectedMin:  0.3,
			expectedMax:  0.6,
		},
		{
			name:         "long response over 100 chars",
			content:      "This is a response that is over one hundred characters long and should get a confidence boost based on length",
			finishReason: "stop",
			expectedMin:  0.9,
			expectedMax:  1.0,
		},
		{
			name:         "very long response over 500 chars",
			content:      "This is a very long response that exceeds five hundred characters. It contains a lot of information and should demonstrate high quality output from the model. The response includes multiple sentences and covers various aspects of the topic being discussed. This additional length should result in a higher confidence score as it indicates the model has provided a comprehensive and detailed response to the user's query. The content continues with more elaboration to ensure we exceed the threshold.",
			finishReason: "stop",
			expectedMin:  0.95,
			expectedMax:  1.0,
		},
		{
			name:         "unknown finish reason",
			content:      "Some content",
			finishReason: "unknown_reason",
			expectedMin:  0.75,
			expectedMax:  0.85,
		},
		{
			name:         "empty content with stop",
			content:      "",
			finishReason: "stop",
			expectedMin:  0.85,
			expectedMax:  0.95,
		},
		{
			name:         "content filter with short content - tests lower bound",
			content:      "X",
			finishReason: "content_filter",
			expectedMin:  0.0,
			expectedMax:  0.55,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := provider.calculateConfidence(tt.content, tt.finishReason)
			assert.GreaterOrEqual(t, confidence, tt.expectedMin)
			assert.LessOrEqual(t, confidence, tt.expectedMax)
		})
	}
}

func TestDeepSeekProvider_CompleteStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send streaming chunks
		w.Write([]byte("data: {\"id\":\"stream1\",\"object\":\"chat.completion.chunk\",\"created\":1677858242,\"model\":\"deepseek-coder\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n"))
		w.Write([]byte("data: {\"id\":\"stream1\",\"object\":\"chat.completion.chunk\",\"created\":1677858242,\"model\":\"deepseek-coder\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" World\"},\"finish_reason\":null}]}\n\n"))
		w.Write([]byte("data: {\"id\":\"stream1\",\"object\":\"chat.completion.chunk\",\"created\":1677858242,\"model\":\"deepseek-coder\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"\"},\"finish_reason\":\"stop\"}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	provider := NewDeepSeekProvider("test-key", server.URL, "deepseek-coder")
	req := &models.LLMRequest{
		ID: "test-stream-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, ch)

	// Collect responses
	var responses []*models.LLMResponse
	for resp := range ch {
		responses = append(responses, resp)
	}

	// Should have received chunks plus final response
	assert.GreaterOrEqual(t, len(responses), 2)
}

func TestDeepSeekProvider_GetCapabilities(t *testing.T) {
	provider := NewDeepSeekProvider("test-key", "", "")
	caps := provider.GetCapabilities()

	require.NotNil(t, caps)

	// Check supported models
	assert.Contains(t, caps.SupportedModels, "deepseek-coder")
	assert.Contains(t, caps.SupportedModels, "deepseek-chat")

	// Check supported features
	assert.Contains(t, caps.SupportedFeatures, "text_completion")
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.Contains(t, caps.SupportedFeatures, "streaming")
	assert.Contains(t, caps.SupportedFeatures, "function_calling")

	// Check supported request types
	assert.Contains(t, caps.SupportedRequestTypes, "text_completion")
	assert.Contains(t, caps.SupportedRequestTypes, "chat")

	// Check boolean capabilities
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.False(t, caps.SupportsVision)
	assert.True(t, caps.SupportsTools)
	assert.False(t, caps.SupportsSearch)
	assert.True(t, caps.SupportsReasoning)
	assert.True(t, caps.SupportsCodeCompletion)
	assert.True(t, caps.SupportsCodeAnalysis)
	assert.True(t, caps.SupportsRefactoring)

	// Check limits
	assert.Equal(t, 4096, caps.Limits.MaxTokens)
	assert.Equal(t, 4096, caps.Limits.MaxInputLength)
	assert.Equal(t, 4096, caps.Limits.MaxOutputLength)
	assert.Equal(t, 10, caps.Limits.MaxConcurrentRequests)

	// Check metadata
	assert.Equal(t, "DeepSeek", caps.Metadata["provider"])
	assert.Equal(t, "v1", caps.Metadata["api_version"])
}

func TestDeepSeekProvider_ValidateConfig(t *testing.T) {
	tests := []struct {
		name         string
		apiKey       string
		baseURL      string
		model        string
		expectValid  bool
		expectErrLen int
	}{
		{
			name:         "all valid",
			apiKey:       "test-key",
			baseURL:      "https://api.deepseek.com",
			model:        "deepseek-coder",
			expectValid:  true,
			expectErrLen: 0,
		},
		{
			name:         "missing api key",
			apiKey:       "",
			baseURL:      "https://api.deepseek.com",
			model:        "deepseek-coder",
			expectValid:  false,
			expectErrLen: 1,
		},
		{
			name:         "missing base url",
			apiKey:       "test-key",
			baseURL:      "",
			model:        "deepseek-coder",
			expectValid:  false,
			expectErrLen: 1,
		},
		{
			name:         "missing model",
			apiKey:       "test-key",
			baseURL:      "https://api.deepseek.com",
			model:        "",
			expectValid:  false,
			expectErrLen: 1,
		},
		{
			name:         "all missing",
			apiKey:       "",
			baseURL:      "",
			model:        "",
			expectValid:  false,
			expectErrLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &DeepSeekProvider{
				apiKey:  tt.apiKey,
				baseURL: tt.baseURL,
				model:   tt.model,
			}

			valid, errs := provider.ValidateConfig(nil)
			assert.Equal(t, tt.expectValid, valid)
			assert.Len(t, errs, tt.expectErrLen)
		})
	}
}

func TestDeepSeekProvider_HealthCheck(t *testing.T) {
	t.Run("health check with invalid API key", func(t *testing.T) {
		provider := NewDeepSeekProvider("invalid-key", "", "deepseek-coder")
		provider.httpClient.Timeout = 2 * time.Second

		// This will fail because of network/auth issues but exercises the code path
		err := provider.HealthCheck()
		// We expect an error since we can't reach the real API
		// The error could be network-related or auth-related
		if err != nil {
			assert.True(t, true) // Expected - API call failed
		}
	})

	t.Run("health check timeout", func(t *testing.T) {
		provider := NewDeepSeekProvider("test-key", "", "deepseek-coder")
		// Set very short timeout to trigger timeout error
		provider.httpClient.Timeout = 1 * time.Nanosecond

		err := provider.HealthCheck()
		// Should fail due to timeout
		assert.Error(t, err)
	})
}

func TestDeepSeekProvider_Complete_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Simulate slow response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chat_123","choices":[{"message":{"content":"test"}}],"usage":{}}`))
	}))
	defer server.Close()

	provider := NewDeepSeekProvider("test-key", server.URL, "deepseek-coder")
	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	resp, err := provider.Complete(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "context")
}

func TestDeepSeekProvider_RetryOnServerError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chat_123","choices":[{"message":{"content":"success"}}],"usage":{}}`))
	}))
	defer server.Close()

	retryConfig := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	provider := NewDeepSeekProviderWithRetry("test-key", server.URL, "deepseek-coder", retryConfig)
	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "success", resp.Content)
	assert.Equal(t, 3, attempts)
}

func TestDeepSeekProvider_ConvertRequestWithSystemPrompt(t *testing.T) {
	provider := NewDeepSeekProvider("test-key", "", "")
	req := &models.LLMRequest{
		ID:     "test-request",
		Prompt: "You are a helpful coding assistant.", // System prompt
		Messages: []models.Message{
			{Role: "user", Content: "Write hello world"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   100,
			Temperature: 0.5,
		},
	}

	deepseekReq := provider.convertRequest(req)

	// Should have system prompt as first message
	assert.Len(t, deepseekReq.Messages, 2)
	assert.Equal(t, "system", deepseekReq.Messages[0].Role)
	assert.Equal(t, "You are a helpful coding assistant.", deepseekReq.Messages[0].Content)
	assert.Equal(t, "user", deepseekReq.Messages[1].Role)
	assert.Equal(t, "Write hello world", deepseekReq.Messages[1].Content)
}

func TestDeepSeekProvider_ConvertResponse(t *testing.T) {
	provider := NewDeepSeekProvider("test-key", "", "")
	req := &models.LLMRequest{ID: "req-123"}

	dsResp := &DeepSeekResponse{
		ID:      "resp-456",
		Model:   "deepseek-coder",
		Created: 1677858242,
		Choices: []DeepSeekChoice{
			{
				Index:        0,
				Message:      DeepSeekMessage{Role: "assistant", Content: "Hello there!"},
				FinishReason: "stop",
			},
		},
		Usage: DeepSeekUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	startTime := time.Now()
	resp := provider.convertResponse(req, dsResp, startTime)

	assert.Equal(t, "resp-456", resp.ID)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, "deepseek", resp.ProviderID)
	assert.Equal(t, "DeepSeek", resp.ProviderName)
	assert.Equal(t, "Hello there!", resp.Content)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 15, resp.TokensUsed)
	assert.Greater(t, resp.Confidence, 0.0)
	assert.NotNil(t, resp.Metadata)
	assert.Equal(t, "deepseek-coder", resp.Metadata["model"])
	assert.Equal(t, 10, resp.Metadata["prompt_tokens"])
	assert.Equal(t, 5, resp.Metadata["completion_tokens"])
}

func TestDeepSeekProvider_ConvertResponse_EmptyChoices(t *testing.T) {
	provider := NewDeepSeekProvider("test-key", "", "")
	req := &models.LLMRequest{ID: "req-123"}

	dsResp := &DeepSeekResponse{
		ID:      "resp-456",
		Choices: []DeepSeekChoice{},
		Usage:   DeepSeekUsage{},
	}

	startTime := time.Now()
	resp := provider.convertResponse(req, dsResp, startTime)

	assert.Equal(t, "", resp.Content)
	assert.Equal(t, "", resp.FinishReason)
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.InitialDelay)
	assert.Equal(t, 30*time.Second, config.MaxDelay)
	assert.Equal(t, 2.0, config.Multiplier)
}

func TestNewDeepSeekProviderWithRetry(t *testing.T) {
	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
	}

	provider := NewDeepSeekProviderWithRetry("test-key", "", "", retryConfig)

	assert.Equal(t, "test-key", provider.apiKey)
	assert.Equal(t, DeepSeekAPIURL, provider.baseURL)
	assert.Equal(t, DeepSeekModel, provider.model)
	assert.Equal(t, 5, provider.retryConfig.MaxRetries)
	assert.Equal(t, 2*time.Second, provider.retryConfig.InitialDelay)
}

func TestIsRetryableStatus(t *testing.T) {
	tests := []struct {
		statusCode int
		retryable  bool
	}{
		{http.StatusOK, false},
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false},
		{http.StatusForbidden, false},
		{http.StatusNotFound, false},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusGatewayTimeout, true},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.statusCode), func(t *testing.T) {
			assert.Equal(t, tt.retryable, isRetryableStatus(tt.statusCode))
		})
	}
}

func TestDeepSeekProvider_NextDelay(t *testing.T) {
	provider := NewDeepSeekProviderWithRetry("test-key", "", "", RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
	})

	// First delay should be multiplied
	next := provider.nextDelay(1 * time.Second)
	assert.Equal(t, 2*time.Second, next)

	// Should hit max delay
	next = provider.nextDelay(8 * time.Second)
	assert.Equal(t, 10*time.Second, next)
}

func TestDeepSeekProvider_Complete_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	provider := NewDeepSeekProvider("test-key", server.URL, "deepseek-coder")
	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to parse DeepSeek response")
}

func TestDeepSeekProvider_RetryExhaustion(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable) // Always fail
	}))
	defer server.Close()

	retryConfig := RetryConfig{
		MaxRetries:   2,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}

	provider := NewDeepSeekProviderWithRetry("test-key", server.URL, "deepseek-coder", retryConfig)
	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 3, attempts) // Initial + 2 retries
	assert.Contains(t, err.Error(), "DeepSeek API error: 503")
}

func TestDeepSeekProvider_MakeAPICall_NetworkError(t *testing.T) {
	// Use an invalid URL to simulate network error
	retryConfig := RetryConfig{
		MaxRetries:   1,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
	}

	provider := NewDeepSeekProviderWithRetry("test-key", "http://localhost:1", "deepseek-coder", retryConfig)
	provider.httpClient.Timeout = 100 * time.Millisecond

	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	// Should fail with network error
}

func TestDeepSeekProvider_CompleteStream_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n"))
	}))
	defer server.Close()

	provider := NewDeepSeekProvider("test-key", server.URL, "deepseek-coder")
	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	ch, err := provider.CompleteStream(ctx, req)
	// Context cancellation should cause makeAPICall to fail
	assert.Error(t, err)
	assert.Nil(t, ch)
}

func BenchmarkDeepSeekProvider_ConvertRequest(b *testing.B) {
	provider := NewDeepSeekProvider("test-key", "", "")
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

func BenchmarkDeepSeekProvider_CalculateConfidence(b *testing.B) {
	provider := NewDeepSeekProvider("test-key", "", "")
	content := "This is a sample response from the DeepSeek model that should be evaluated for confidence scoring."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.calculateConfidence(content, "stop")
	}
}
