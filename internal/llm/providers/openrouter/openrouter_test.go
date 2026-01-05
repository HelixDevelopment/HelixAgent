package openrouter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/models"
)

func TestNewSimpleOpenRouterProvider(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		want   *SimpleOpenRouterProvider
	}{
		{
			name:   "valid api key",
			apiKey: "test-api-key-123",
			want: &SimpleOpenRouterProvider{
				apiKey:  "test-api-key-123",
				baseURL: defaultBaseURL,
				client: &http.Client{
					Timeout: 60 * time.Second,
				},
			},
		},
		{
			name:   "empty api key",
			apiKey: "",
			want: &SimpleOpenRouterProvider{
				apiKey:  "",
				baseURL: defaultBaseURL,
				client: &http.Client{
					Timeout: 60 * time.Second,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSimpleOpenRouterProvider(tt.apiKey)
			assert.Equal(t, tt.want.apiKey, got.apiKey)
			assert.Equal(t, tt.want.baseURL, got.baseURL)
			assert.NotNil(t, got.client)
			assert.Equal(t, 60*time.Second, got.client.Timeout)
		})
	}
}

func TestNewSimpleOpenRouterProviderWithBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		baseURL string
		want    *SimpleOpenRouterProvider
	}{
		{
			name:    "custom base URL",
			apiKey:  "test-key",
			baseURL: "https://custom.example.com",
			want: &SimpleOpenRouterProvider{
				apiKey:  "test-key",
				baseURL: "https://custom.example.com",
				client: &http.Client{
					Timeout: 60 * time.Second,
				},
			},
		},
		{
			name:    "empty base URL defaults to standard URL",
			apiKey:  "test-key",
			baseURL: "",
			want: &SimpleOpenRouterProvider{
				apiKey:  "test-key",
				baseURL: "https://openrouter.ai/api/v1", // Defaults to standard URL when empty
				client: &http.Client{
					Timeout: 60 * time.Second,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSimpleOpenRouterProviderWithBaseURL(tt.apiKey, tt.baseURL)
			assert.Equal(t, tt.want.apiKey, got.apiKey)
			assert.Equal(t, tt.want.baseURL, got.baseURL)
			assert.NotNil(t, got.client)
			assert.Equal(t, 60*time.Second, got.client.Timeout)
		})
	}
}

func TestSimpleOpenRouterProvider_Complete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "superagent", r.Header.Get("HTTP-Referer"))

		var reqBody struct {
			Model       string           `json:"model"`
			Messages    []models.Message `json:"messages"`
			Prompt      string           `json:"prompt,omitempty"`
			MaxTokens   int              `json:"max_tokens,omitempty"`
			Temperature float64          `json:"temperature,omitempty"`
		}

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &reqBody)
		require.NoError(t, err)

		assert.Equal(t, "openrouter/anthropic/claude-3.5-sonnet", reqBody.Model)
		assert.Equal(t, "Hello, how are you?", reqBody.Prompt)
		assert.Equal(t, 1000, reqBody.MaxTokens)
		assert.Equal(t, 0.7, reqBody.Temperature)

		response := map[string]interface{}{
			"id": "chatcmpl-123",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "I'm doing well, thank you for asking!",
					},
				},
			},
			"created": 1677858242,
			"model":   "openrouter/anthropic/claude-3.5-sonnet",
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 20,
				"total_tokens":      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewSimpleOpenRouterProviderWithBaseURL("test-api-key", server.URL)
	provider.client = server.Client()

	req := &models.LLMRequest{
		ID: "test-req-123",
		ModelParams: models.ModelParameters{
			Model:       "openrouter/anthropic/claude-3.5-sonnet",
			MaxTokens:   1000,
			Temperature: 0.7,
		},
		Prompt: "Hello, how are you?",
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "chatcmpl-123", resp.ID)
	assert.Equal(t, "test-req-123", resp.RequestID)
	assert.Equal(t, "openrouter", resp.ProviderID)
	assert.Equal(t, "OpenRouter", resp.ProviderName)
	assert.Equal(t, "I'm doing well, thank you for asking!", resp.Content)
	assert.Equal(t, 0.85, resp.Confidence) // Default confidence for OpenRouter
	assert.Equal(t, 30, resp.TokensUsed)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, "openrouter/anthropic/claude-3.5-sonnet", resp.Metadata["model"])
	assert.Equal(t, "openrouter", resp.Metadata["provider"])
}

func TestSimpleOpenRouterProvider_Complete_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid API key",
				"type":    "authentication_error",
				"code":    401,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewSimpleOpenRouterProviderWithBaseURL("invalid-api-key", server.URL)
	provider.client = server.Client()

	req := &models.LLMRequest{
		ID: "test-req-456",
		ModelParams: models.ModelParameters{
			Model: "openrouter/anthropic/claude-3.5-sonnet",
		},
		Prompt: "Test prompt",
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "OpenRouter API error: Invalid API key")
}

func TestSimpleOpenRouterProvider_Complete_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"id":      "chatcmpl-789",
			"choices": []map[string]interface{}{},
			"created": 1677858242,
			"model":   "openrouter/anthropic/claude-3.5-sonnet",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewSimpleOpenRouterProviderWithBaseURL("test-api-key", server.URL)
	provider.client = server.Client()

	req := &models.LLMRequest{
		ID: "test-req-789",
		ModelParams: models.ModelParameters{
			Model: "openrouter/anthropic/claude-3.5-sonnet",
		},
		Prompt: "Test prompt",
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "no choices in OpenRouter response")
}

func TestSimpleOpenRouterProvider_Complete_NetworkError(t *testing.T) {
	provider := NewSimpleOpenRouterProvider("test-api-key")
	// Create a client that will fail
	provider.client = &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	req := &models.LLMRequest{
		ID: "test-req-network",
		ModelParams: models.ModelParameters{
			Model: "openrouter/anthropic/claude-3.5-sonnet",
		},
		Prompt: "Test prompt",
	}

	// Use a context that will timeout immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
	defer cancel()

	resp, err := provider.Complete(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestSimpleOpenRouterProvider_Complete_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	provider := NewSimpleOpenRouterProviderWithBaseURL("test-api-key", server.URL)
	provider.client = server.Client()

	req := &models.LLMRequest{
		ID: "test-req-json",
		ModelParams: models.ModelParameters{
			Model: "openrouter/anthropic/claude-3.5-sonnet",
		},
		Prompt: "Test prompt",
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to decode OpenRouter response")
}

func TestSimpleOpenRouterProvider_CompleteStream(t *testing.T) {
	// Create a mock server that simulates SSE streaming
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		// Send SSE data chunks
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("server doesn't support flushing")
		}

		// Send a single chunk and the done message
		w.Write([]byte("data: {\"id\":\"stream-1\",\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"))
		flusher.Flush()
		time.Sleep(50 * time.Millisecond) // Give client time to process
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
		time.Sleep(50 * time.Millisecond) // Keep connection open briefly
	}))
	defer server.Close()

	provider := NewSimpleOpenRouterProvider("test-api-key")
	provider.baseURL = server.URL

	req := &models.LLMRequest{
		ID: "test-req-stream",
		ModelParams: models.ModelParameters{
			Model: "openrouter/anthropic/claude-3.5-sonnet",
		},
		Prompt: "Test streaming prompt",
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, ch)

	// Read from channel with timeout
	responses := make([]*models.LLMResponse, 0)
	timeout := time.After(2 * time.Second)
	done := false

	for !done {
		select {
		case resp, ok := <-ch:
			if !ok {
				done = true
				break
			}
			if resp != nil {
				responses = append(responses, resp)
			}
		case <-timeout:
			done = true
		}
	}

	assert.NotEmpty(t, responses, "expected at least one response")
}

func TestSimpleOpenRouterProvider_HealthCheck(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
	}{
		{
			name:    "valid api key",
			apiKey:  "test-api-key",
			wantErr: false,
		},
		{
			name:    "empty api key",
			apiKey:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewSimpleOpenRouterProvider(tt.apiKey)
			err := provider.HealthCheck()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "API key is required")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSimpleOpenRouterProvider_GetCapabilities(t *testing.T) {
	provider := NewSimpleOpenRouterProvider("test-api-key")
	caps := provider.GetCapabilities()

	assert.NotNil(t, caps)
	assert.NotEmpty(t, caps.SupportedModels)
	assert.Contains(t, caps.SupportedModels, "openrouter/anthropic/claude-3.5-sonnet")
	assert.Contains(t, caps.SupportedModels, "openrouter/openai/gpt-4o")
	assert.Contains(t, caps.SupportedModels, "openrouter/google/gemini-pro")

	assert.Contains(t, caps.SupportedFeatures, "text_completion")
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.Contains(t, caps.SupportedFeatures, "multi_model_routing")
	assert.Contains(t, caps.SupportedFeatures, "cost_optimization")

	assert.Contains(t, caps.SupportedRequestTypes, "text_completion")
	assert.Contains(t, caps.SupportedRequestTypes, "chat")

	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsFunctionCalling)
	assert.False(t, caps.SupportsVision)
	assert.True(t, caps.SupportsTools)
	assert.True(t, caps.SupportsSearch)
	assert.True(t, caps.SupportsReasoning)
	assert.True(t, caps.SupportsCodeCompletion)
	assert.True(t, caps.SupportsCodeAnalysis)
	assert.True(t, caps.SupportsRefactoring)

	assert.Equal(t, 200000, caps.Limits.MaxTokens)
	assert.Equal(t, 200000, caps.Limits.MaxInputLength)
	assert.Equal(t, 8192, caps.Limits.MaxOutputLength)
	assert.Equal(t, 10, caps.Limits.MaxConcurrentRequests)

	assert.Equal(t, "OpenRouter", caps.Metadata["provider"])
	assert.Equal(t, "v1", caps.Metadata["api_version"])
	assert.Equal(t, "basic", caps.Metadata["routing"])
	assert.Equal(t, "true", caps.Metadata["multi_tenancy"])
}

func TestSimpleOpenRouterProvider_ValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		config    map[string]interface{}
		wantValid bool
		wantErrs  []string
	}{
		{
			name:      "valid config with api key",
			apiKey:    "test-api-key",
			config:    map[string]interface{}{},
			wantValid: true,
			wantErrs:  nil,
		},
		{
			name:      "empty api key",
			apiKey:    "",
			config:    map[string]interface{}{},
			wantValid: false,
			wantErrs:  []string{"api_key is required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewSimpleOpenRouterProvider(tt.apiKey)
			valid, errs := provider.ValidateConfig(tt.config)
			assert.Equal(t, tt.wantValid, valid)
			assert.Equal(t, tt.wantErrs, errs)
		})
	}
}

func TestSimpleOpenRouterProvider_Complete_WithMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody struct {
			Messages []models.Message `json:"messages"`
		}

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &reqBody)
		require.NoError(t, err)

		assert.Len(t, reqBody.Messages, 2)
		assert.Equal(t, "user", reqBody.Messages[0].Role)
		assert.Equal(t, "Hello", reqBody.Messages[0].Content)
		assert.Equal(t, "assistant", reqBody.Messages[1].Role)
		assert.Equal(t, "Hi there!", reqBody.Messages[1].Content)

		response := map[string]interface{}{
			"id": "chatcmpl-messages",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "How can I help you today?",
					},
				},
			},
			"created": 1677858242,
			"model":   "openrouter/anthropic/claude-3.5-sonnet",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewSimpleOpenRouterProviderWithBaseURL("test-api-key", server.URL)
	provider.client = server.Client()

	req := &models.LLMRequest{
		ID: "test-req-messages",
		ModelParams: models.ModelParameters{
			Model: "openrouter/anthropic/claude-3.5-sonnet",
		},
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "How can I help you today?", resp.Content)
}

func TestSimpleOpenRouterProvider_Complete_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)
		response := map[string]interface{}{
			"id": "chatcmpl-slow",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Slow response",
					},
				},
			},
			"created": 1677858242,
			"model":   "openrouter/anthropic/claude-3.5-sonnet",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewSimpleOpenRouterProviderWithBaseURL("test-api-key", server.URL)
	provider.client = server.Client()

	req := &models.LLMRequest{
		ID: "test-req-timeout",
		ModelParams: models.ModelParameters{
			Model: "openrouter/anthropic/claude-3.5-sonnet",
		},
		Prompt: "Test prompt",
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	resp, err := provider.Complete(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}
