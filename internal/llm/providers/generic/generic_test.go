package generic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGenericProvider(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pName   string
		apiKey  string
		baseURL string
		model   string
	}{
		{
			name:    "all fields set",
			pName:   "nvidia",
			apiKey:  "test-key-123",
			baseURL: "https://api.nvidia.com/v1/chat/completions",
			model:   "llama-3.1-70b",
		},
		{
			name:    "empty values",
			pName:   "",
			apiKey:  "",
			baseURL: "",
			model:   "",
		},
		{
			name:    "different provider",
			pName:   "sambanova",
			apiKey:  "sk-abc",
			baseURL: "https://api.sambanova.ai/v1/chat/completions",
			model:   "Meta-Llama-3-70B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			provider := NewGenericProvider(tt.pName, tt.apiKey, tt.baseURL, tt.model)
			require.NotNil(t, provider)
			assert.Equal(t, tt.pName, provider.name)
			assert.Equal(t, tt.apiKey, provider.apiKey)
			assert.Equal(t, tt.baseURL, provider.baseURL)
			assert.Equal(t, tt.model, provider.model)
			assert.NotNil(t, provider.httpClient)
			assert.Equal(t, DefaultTimeout, provider.httpClient.Timeout)
		})
	}
}

func TestComplete(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-abc123",
			"object": "chat.completion",
			"created": 1700000000,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "Hello from generic provider!"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 12, "completion_tokens": 8, "total_tokens": 20}
		}`))
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "test-key", server.URL, "test-model")
	req := &models.LLMRequest{
		ID: "req-001",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   256,
			Temperature: 0.7,
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "chatcmpl-abc123", resp.ID)
	assert.Equal(t, "req-001", resp.RequestID)
	assert.Equal(t, "test-provider", resp.ProviderID)
	assert.Equal(t, "test-provider", resp.ProviderName)
	assert.Equal(t, "Hello from generic provider!", resp.Content)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 0.9, resp.Confidence) // "stop" finish reason gives 0.9
	assert.Equal(t, 20, resp.TokensUsed)
	assert.GreaterOrEqual(t, resp.ResponseTime, int64(0))
	assert.False(t, resp.CreatedAt.IsZero())
}

func TestCompleteStream(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`data: {"id":"stream-1","object":"chat.completion.chunk","created":1700000000,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":""}]}`,
			`data: {"id":"stream-1","object":"chat.completion.chunk","created":1700000000,"model":"test-model","choices":[{"index":0,"delta":{"content":" World"},"finish_reason":""}]}`,
			`data: {"id":"stream-1","object":"chat.completion.chunk","created":1700000000,"model":"test-model","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":""}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			_, _ = fmt.Fprintf(w, "%s\n", chunk)
		}
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "test-key", server.URL, "test-model")
	req := &models.LLMRequest{
		ID: "req-stream-001",
		Messages: []models.Message{
			{Role: "user", Content: "Say hello"},
		},
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, ch)

	var responses []*models.LLMResponse
	for resp := range ch {
		responses = append(responses, resp)
	}

	// Should have delta chunks plus one final response
	require.GreaterOrEqual(t, len(responses), 2)

	// Find the final response (with finish_reason "stop")
	finalResp := responses[len(responses)-1]
	assert.Equal(t, "stop", finalResp.FinishReason)
	assert.Equal(t, "Hello World!", finalResp.Content)
	assert.Equal(t, "test-provider", finalResp.ProviderName)
	assert.Equal(t, "req-stream-001", finalResp.RequestID)
	assert.Equal(t, 0.85, finalResp.Confidence)

	// Verify delta chunks have partial content
	assert.Equal(t, "Hello", responses[0].Content)
	assert.Equal(t, " World", responses[1].Content)
	assert.Equal(t, "!", responses[2].Content)
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "hc-1",
			"choices": [{"message": {"content": "hi"}, "finish_reason": "stop"}],
			"usage": {"total_tokens": 2}
		}`))
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "test-key", server.URL, "test-model")
	err := provider.HealthCheck()
	assert.NoError(t, err)
}

func TestHealthCheckFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "test-key", server.URL, "test-model")
	err := provider.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
	assert.Contains(t, err.Error(), "status 500")
}

func TestGetCapabilities(t *testing.T) {
	t.Parallel()
	provider := NewGenericProvider("nvidia", "key", "https://api.nvidia.com/v1/chat/completions", "llama-3.1-70b")
	caps := provider.GetCapabilities()

	require.NotNil(t, caps)

	// Supported models should contain only the configured model
	assert.Equal(t, []string{"llama-3.1-70b"}, caps.SupportedModels)

	// Supported features
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.Contains(t, caps.SupportedFeatures, "streaming")

	// Supported request types
	assert.Contains(t, caps.SupportedRequestTypes, "chat")

	// Boolean capabilities
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsFunctionCalling)
	assert.False(t, caps.SupportsVision)
	assert.False(t, caps.SupportsTools)

	// Limits
	assert.Equal(t, MaxTokensCap, caps.Limits.MaxTokens)
	assert.Equal(t, MaxTokensCap, caps.Limits.MaxInputLength)
	assert.Equal(t, MaxTokensCap, caps.Limits.MaxOutputLength)
	assert.Equal(t, 10, caps.Limits.MaxConcurrentRequests)

	// Metadata
	assert.Equal(t, "nvidia", caps.Metadata["provider"])
	assert.Equal(t, "generic_openai_compatible", caps.Metadata["type"])
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()
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
			apiKey:       "sk-test",
			baseURL:      "https://api.example.com/v1/chat/completions",
			model:        "llama-3.1-70b",
			expectValid:  true,
			expectErrLen: 0,
		},
		{
			name:         "missing API key",
			apiKey:       "",
			baseURL:      "https://api.example.com/v1/chat/completions",
			model:        "llama-3.1-70b",
			expectValid:  false,
			expectErrLen: 1,
		},
		{
			name:         "missing base URL",
			apiKey:       "sk-test",
			baseURL:      "",
			model:        "llama-3.1-70b",
			expectValid:  false,
			expectErrLen: 1,
		},
		{
			name:         "missing model",
			apiKey:       "sk-test",
			baseURL:      "https://api.example.com/v1/chat/completions",
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
		{
			name:         "only API key provided",
			apiKey:       "sk-test",
			baseURL:      "",
			model:        "",
			expectValid:  false,
			expectErrLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			provider := &Provider{
				apiKey:  tt.apiKey,
				baseURL: tt.baseURL,
				model:   tt.model,
			}

			valid, errs := provider.ValidateConfig(nil)
			assert.Equal(t, tt.expectValid, valid)
			assert.Len(t, errs, tt.expectErrLen)

			if tt.apiKey == "" && !valid {
				assert.Contains(t, errs, "API key is required")
			}
			if tt.baseURL == "" && !valid {
				assert.Contains(t, errs, "base URL is required")
			}
			if tt.model == "" && !valid {
				assert.Contains(t, errs, "model is required")
			}
		})
	}
}

func TestConvertRequest(t *testing.T) {
	t.Parallel()
	provider := NewGenericProvider("test-provider", "key", "https://api.example.com", "default-model")
	req := &models.LLMRequest{
		ID: "req-001",
		Messages: []models.Message{
			{Role: "user", Content: "Hello, how are you?"},
			{Role: "assistant", Content: "I am fine, thanks!"},
			{Role: "user", Content: "What can you do?"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:     512,
			Temperature:   0.8,
			TopP:          0.95,
			StopSequences: []string{"\n\n", "END"},
		},
	}

	apiReq := provider.convertRequest(req)

	assert.Equal(t, "default-model", apiReq.Model)
	assert.Equal(t, 512, apiReq.MaxTokens)
	assert.Equal(t, 0.8, apiReq.Temperature)
	assert.Equal(t, 0.95, apiReq.TopP)
	assert.Equal(t, []string{"\n\n", "END"}, apiReq.Stop)

	// No system prompt, so messages should match directly
	require.Len(t, apiReq.Messages, 3)
	assert.Equal(t, "user", apiReq.Messages[0].Role)
	assert.Equal(t, "Hello, how are you?", apiReq.Messages[0].Content)
	assert.Equal(t, "assistant", apiReq.Messages[1].Role)
	assert.Equal(t, "I am fine, thanks!", apiReq.Messages[1].Content)
	assert.Equal(t, "user", apiReq.Messages[2].Role)
	assert.Equal(t, "What can you do?", apiReq.Messages[2].Content)
}

func TestConvertRequest_DefaultMaxTokens(t *testing.T) {
	t.Parallel()
	provider := NewGenericProvider("test-provider", "key", "https://api.example.com", "model")
	req := &models.LLMRequest{
		ID: "req-001",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens: 0, // Should default to DefaultMaxTokens
		},
	}

	apiReq := provider.convertRequest(req)
	assert.Equal(t, DefaultMaxTokens, apiReq.MaxTokens)
}

func TestConvertRequest_MaxTokensCap(t *testing.T) {
	t.Parallel()
	provider := NewGenericProvider("test-provider", "key", "https://api.example.com", "model")
	req := &models.LLMRequest{
		ID: "req-001",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens: 999999, // Exceeds MaxTokensCap, should be capped
		},
	}

	apiReq := provider.convertRequest(req)
	assert.Equal(t, MaxTokensCap, apiReq.MaxTokens)
}

func TestConvertRequest_NegativeMaxTokens(t *testing.T) {
	t.Parallel()
	provider := NewGenericProvider("test-provider", "key", "https://api.example.com", "model")
	req := &models.LLMRequest{
		ID: "req-001",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens: -1, // Negative should default to DefaultMaxTokens
		},
	}

	apiReq := provider.convertRequest(req)
	assert.Equal(t, DefaultMaxTokens, apiReq.MaxTokens)
}

func TestConvertRequestWithSystemPrompt(t *testing.T) {
	t.Parallel()
	provider := NewGenericProvider("test-provider", "key", "https://api.example.com", "model")
	req := &models.LLMRequest{
		ID:     "req-001",
		Prompt: "You are a helpful coding assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Write hello world in Go"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   100,
			Temperature: 0.5,
		},
	}

	apiReq := provider.convertRequest(req)

	// System prompt should be prepended as the first message
	require.Len(t, apiReq.Messages, 2)
	assert.Equal(t, "system", apiReq.Messages[0].Role)
	assert.Equal(t, "You are a helpful coding assistant.", apiReq.Messages[0].Content)
	assert.Equal(t, "user", apiReq.Messages[1].Role)
	assert.Equal(t, "Write hello world in Go", apiReq.Messages[1].Content)
}

func TestCompleteAPIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"message": "Invalid model specified", "type": "invalid_request_error", "code": "model_not_found"}}`))
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "test-key", server.URL, "bad-model")
	req := &models.LLMRequest{
		ID: "req-err-001",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test-provider")
	assert.Contains(t, err.Error(), "API error: 400")
	assert.Contains(t, err.Error(), "Invalid model specified")
}

func TestCompleteTimeout(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"late","choices":[{"message":{"content":"too late"}}]}`))
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "test-key", server.URL, "test-model")
	req := &models.LLMRequest{
		ID: "req-timeout-001",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	// Cancel the context before making the request
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := provider.Complete(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "context")
}

func TestStreamDone(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send a single chunk followed by [DONE]
		_, _ = fmt.Fprint(w, "data: {\"id\":\"s1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Done test\"},\"finish_reason\":\"\"}]}\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n")
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "test-key", server.URL, "test-model")
	req := &models.LLMRequest{
		ID: "req-done-001",
		Messages: []models.Message{
			{Role: "user", Content: "Test done signal"},
		},
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, ch)

	var responses []*models.LLMResponse
	for resp := range ch {
		responses = append(responses, resp)
	}

	// Should have the delta chunk and the final [DONE] response
	require.GreaterOrEqual(t, len(responses), 2)

	// The final response should have accumulated content and finish_reason "stop"
	finalResp := responses[len(responses)-1]
	assert.Equal(t, "stop", finalResp.FinishReason)
	assert.Equal(t, "Done test", finalResp.Content)
	assert.Contains(t, finalResp.ID, "stream-final-")
	assert.Equal(t, "req-done-001", finalResp.RequestID)
}

func TestStreamError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send one chunk then close connection abruptly (simulates read error)
		_, _ = fmt.Fprint(w, "data: {\"id\":\"s1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Partial\"},\"finish_reason\":\"\"}]}\n")
		// The handler returns, closing the connection which causes an EOF
		// The goroutine in CompleteStream will hit io.EOF on ReadBytes
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "test-key", server.URL, "test-model")
	req := &models.LLMRequest{
		ID: "req-stream-err-001",
		Messages: []models.Message{
			{Role: "user", Content: "Partial stream"},
		},
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, ch)

	var responses []*models.LLMResponse
	for resp := range ch {
		responses = append(responses, resp)
	}

	// Should have at least the partial chunk; channel closes on EOF
	require.GreaterOrEqual(t, len(responses), 1)
	assert.Equal(t, "Partial", responses[0].Content)
}

func TestStreamAPIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": {"message": "Rate limit exceeded"}}`))
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "test-key", server.URL, "test-model")
	req := &models.LLMRequest{
		ID: "req-stream-api-err",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	assert.Nil(t, ch)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stream API error: 429")
}

func TestModelOverride(t *testing.T) {
	t.Parallel()
	var receivedModel string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var apiReq Request
		err := decodeJSONBody(r, &apiReq)
		if err == nil {
			receivedModel = apiReq.Model
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "override-1",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "response"},
				"finish_reason": "stop"
			}],
			"usage": {"total_tokens": 10}
		}`))
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "test-key", server.URL, "default-model")

	// Request with ModelParams.Model override
	req := &models.LLMRequest{
		ID: "req-override-001",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			Model:     "override-model",
			MaxTokens: 100,
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// The server should have received the overridden model
	assert.Equal(t, "override-model", receivedModel)

	// Also verify via convertRequest directly
	apiReq := provider.convertRequest(req)
	assert.Equal(t, "override-model", apiReq.Model)
}

func TestModelOverride_EmptyDoesNotOverride(t *testing.T) {
	t.Parallel()
	provider := NewGenericProvider("test-provider", "key", "https://api.example.com", "default-model")
	req := &models.LLMRequest{
		ID: "req-001",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			Model: "", // Empty should not override
		},
	}

	apiReq := provider.convertRequest(req)
	assert.Equal(t, "default-model", apiReq.Model)
}

func TestComplete_FinishReasonConfidence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		finishReason       string
		expectedConfidence float64
	}{
		{
			name:               "stop gives 0.9",
			finishReason:       "stop",
			expectedConfidence: 0.9,
		},
		{
			name:               "length gives 0.75",
			finishReason:       "length",
			expectedConfidence: 0.75,
		},
		{
			name:               "other gives 0.85",
			finishReason:       "content_filter",
			expectedConfidence: 0.85,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, `{
					"id": "conf-%s",
					"choices": [{
						"index": 0,
						"message": {"role": "assistant", "content": "test"},
						"finish_reason": "%s"
					}],
					"usage": {"total_tokens": 5}
				}`, tt.finishReason, tt.finishReason)
			}))
			defer server.Close()

			provider := NewGenericProvider("test-provider", "key", server.URL, "model")
			req := &models.LLMRequest{
				ID: "req-conf",
				Messages: []models.Message{
					{Role: "user", Content: "Hi"},
				},
			}

			resp, err := provider.Complete(context.Background(), req)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedConfidence, resp.Confidence)
		})
	}
}

func TestComplete_EmptyChoices(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "empty-choices",
			"choices": [],
			"usage": {"total_tokens": 3}
		}`))
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "key", server.URL, "model")
	req := &models.LLMRequest{
		ID: "req-empty",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "", resp.Content)
	assert.Equal(t, "", resp.FinishReason)
	assert.Equal(t, 0.85, resp.Confidence) // Default confidence for unknown finish reason
}

func TestComplete_NoUsage(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "no-usage",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "hi"},
				"finish_reason": "stop"
			}]
		}`))
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "key", server.URL, "model")
	req := &models.LLMRequest{
		ID: "req-no-usage",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 0, resp.TokensUsed) // No usage data
}

func TestComplete_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json response`))
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "key", server.URL, "model")
	req := &models.LLMRequest{
		ID: "req-invalid-json",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to parse response")
}

func TestComplete_AuthorizationHeader(t *testing.T) {
	t.Parallel()
	var capturedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"auth-test","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "my-secret-key", server.URL, "model")
	req := &models.LLMRequest{
		ID: "req-auth",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "Bearer my-secret-key", capturedAuth)
}

func TestHealthCheck_NetworkError(t *testing.T) {
	t.Parallel()
	// Use an unreachable address to trigger a network error
	provider := NewGenericProvider("test-provider", "key", "http://127.0.0.1:1", "model")
	provider.httpClient.Timeout = 100 * time.Millisecond

	err := provider.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

func TestConvertRequest_EmptyMessages(t *testing.T) {
	t.Parallel()
	provider := NewGenericProvider("test-provider", "key", "https://api.example.com", "model")
	req := &models.LLMRequest{
		ID:       "req-empty-msgs",
		Messages: []models.Message{},
		ModelParams: models.ModelParameters{
			MaxTokens: 100,
		},
	}

	apiReq := provider.convertRequest(req)
	assert.Empty(t, apiReq.Messages)
	assert.Equal(t, "model", apiReq.Model)
	assert.Equal(t, 100, apiReq.MaxTokens)
}

func TestConvertRequest_SystemPromptWithNoMessages(t *testing.T) {
	t.Parallel()
	provider := NewGenericProvider("test-provider", "key", "https://api.example.com", "model")
	req := &models.LLMRequest{
		ID:       "req-sys-only",
		Prompt:   "You are a helpful assistant.",
		Messages: []models.Message{},
	}

	apiReq := provider.convertRequest(req)
	require.Len(t, apiReq.Messages, 1)
	assert.Equal(t, "system", apiReq.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant.", apiReq.Messages[0].Content)
}

func TestCompleteStream_ContextCancellation(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "test-key", server.URL, "test-model")
	req := &models.LLMRequest{
		ID: "req-cancel",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	ch, err := provider.CompleteStream(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, ch)
	assert.Contains(t, err.Error(), "context")
}

func TestStreamSkipsNonDataLines(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Mix data lines with non-data lines (comments, empty lines, event types)
		_, _ = fmt.Fprint(w, ": this is a comment\n")
		_, _ = fmt.Fprint(w, "event: message\n")
		_, _ = fmt.Fprint(w, "\n")
		_, _ = fmt.Fprint(w, "data: {\"id\":\"s1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":\"\"}]}\n")
		_, _ = fmt.Fprint(w, "\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n")
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "test-key", server.URL, "test-model")
	req := &models.LLMRequest{
		ID: "req-skip-001",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, ch)

	var responses []*models.LLMResponse
	for resp := range ch {
		responses = append(responses, resp)
	}

	// Should have the delta chunk and the final [DONE] response
	require.GreaterOrEqual(t, len(responses), 2)
	assert.Equal(t, "Hello", responses[0].Content)
	assert.Equal(t, "stop", responses[len(responses)-1].FinishReason)
}

func TestStreamSkipsMalformedJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		_, _ = fmt.Fprint(w, "data: {not valid json}\n")
		_, _ = fmt.Fprint(w, "data: {\"id\":\"s1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Valid\"},\"finish_reason\":\"\"}]}\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n")
	}))
	defer server.Close()

	provider := NewGenericProvider("test-provider", "test-key", server.URL, "test-model")
	req := &models.LLMRequest{
		ID: "req-malformed",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	require.NoError(t, err)

	var responses []*models.LLMResponse
	for resp := range ch {
		responses = append(responses, resp)
	}

	// Malformed JSON should be silently skipped
	require.GreaterOrEqual(t, len(responses), 2)
	assert.Equal(t, "Valid", responses[0].Content)
}

// decodeJSONBody is a helper for tests to decode JSON request bodies.
func decodeJSONBody(r *http.Request, target interface{}) error {
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(target)
}

func BenchmarkConvertRequest(b *testing.B) {
	provider := NewGenericProvider("bench-provider", "key", "https://api.example.com", "model")
	req := &models.LLMRequest{
		ID:     "bench-req",
		Prompt: "You are a helpful assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "What is Go?"},
		},
		ModelParams: models.ModelParameters{
			Model:         "custom-model",
			MaxTokens:     2048,
			Temperature:   0.7,
			TopP:          0.9,
			StopSequences: []string{"\n"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.convertRequest(req)
	}
}

func BenchmarkComplete(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"bench","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"total_tokens":5}}`))
	}))
	defer server.Close()

	provider := NewGenericProvider("bench-provider", "key", server.URL, "model")
	req := &models.LLMRequest{
		ID: "bench-req",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = provider.Complete(context.Background(), req)
	}
}
