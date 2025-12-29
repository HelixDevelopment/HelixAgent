package qwen

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

func TestNewQwenProvider(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		baseURL string
		model   string
		want    *QwenProvider
	}{
		{
			name:    "all parameters provided",
			apiKey:  "test-api-key-123",
			baseURL: "https://custom.example.com",
			model:   "qwen-max",
			want: &QwenProvider{
				apiKey:  "test-api-key-123",
				baseURL: "https://custom.example.com",
				model:   "qwen-max",
				httpClient: &http.Client{
					Timeout: 60 * time.Second,
				},
			},
		},
		{
			name:    "default parameters",
			apiKey:  "test-key",
			baseURL: "",
			model:   "",
			want: &QwenProvider{
				apiKey:  "test-key",
				baseURL: "https://dashscope.aliyuncs.com/api/v1",
				model:   "qwen-turbo",
				httpClient: &http.Client{
					Timeout: 60 * time.Second,
				},
			},
		},
		{
			name:    "empty api key",
			apiKey:  "",
			baseURL: "https://api.example.com",
			model:   "qwen-plus",
			want: &QwenProvider{
				apiKey:  "",
				baseURL: "https://api.example.com",
				model:   "qwen-plus",
				httpClient: &http.Client{
					Timeout: 60 * time.Second,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewQwenProvider(tt.apiKey, tt.baseURL, tt.model)
			assert.Equal(t, tt.want.apiKey, got.apiKey)
			assert.Equal(t, tt.want.baseURL, got.baseURL)
			assert.Equal(t, tt.want.model, got.model)
			assert.NotNil(t, got.httpClient)
			assert.Equal(t, 60*time.Second, got.httpClient.Timeout)
		})
	}
}

func TestQwenProvider_Complete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/services/aigc/text-generation/generation", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

		var reqBody QwenRequest
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &reqBody)
		require.NoError(t, err)

		assert.Equal(t, "qwen-turbo", reqBody.Model)
		assert.False(t, reqBody.Stream)
		assert.Equal(t, 0.7, reqBody.Temperature)
		assert.Equal(t, 1000, reqBody.MaxTokens)
		assert.Len(t, reqBody.Messages, 1)
		assert.Equal(t, "system", reqBody.Messages[0].Role)
		assert.Equal(t, "Hello, how are you?", reqBody.Messages[0].Content)

		response := QwenResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "qwen-turbo",
			Choices: []QwenChoice{
				{
					Index: 0,
					Message: QwenMessage{
						Role:    "assistant",
						Content: "I'm doing well, thank you for asking!",
					},
					FinishReason: "stop",
				},
			},
			Usage: QwenUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewQwenProvider("test-api-key", server.URL, "qwen-turbo")
	provider.httpClient = server.Client()

	req := &models.LLMRequest{
		ID: "test-req-123",
		ModelParams: models.ModelParameters{
			Model:       "qwen-turbo",
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
	assert.Equal(t, "qwen", resp.ProviderID)
	assert.Equal(t, "Qwen", resp.ProviderName)
	assert.Equal(t, "I'm doing well, thank you for asking!", resp.Content)
	assert.Equal(t, 0.85, resp.Confidence)
	assert.Equal(t, 30, resp.TokensUsed)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, "qwen-turbo", resp.Metadata["model"])
	assert.Equal(t, "chat.completion", resp.Metadata["object"])
	assert.Equal(t, 10, resp.Metadata["prompt_tokens"])
	assert.Equal(t, 20, resp.Metadata["completion_tokens"])
}

func TestQwenProvider_Complete_WithMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody QwenRequest
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &reqBody)
		require.NoError(t, err)

		assert.Len(t, reqBody.Messages, 3)
		assert.Equal(t, "system", reqBody.Messages[0].Role)
		assert.Equal(t, "You are a helpful assistant", reqBody.Messages[0].Content)
		assert.Equal(t, "user", reqBody.Messages[1].Role)
		assert.Equal(t, "Hello", reqBody.Messages[1].Content)
		assert.Equal(t, "assistant", reqBody.Messages[2].Role)
		assert.Equal(t, "Hi there!", reqBody.Messages[2].Content)

		response := QwenResponse{
			ID:      "chatcmpl-messages",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "qwen-turbo",
			Choices: []QwenChoice{
				{
					Index: 0,
					Message: QwenMessage{
						Role:    "assistant",
						Content: "How can I help you today?",
					},
					FinishReason: "stop",
				},
			},
			Usage: QwenUsage{
				PromptTokens:     15,
				CompletionTokens: 25,
				TotalTokens:      40,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewQwenProvider("test-api-key", server.URL, "qwen-turbo")
	provider.httpClient = server.Client()

	req := &models.LLMRequest{
		ID: "test-req-messages",
		ModelParams: models.ModelParameters{
			Model: "qwen-turbo",
		},
		Prompt: "You are a helpful assistant",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "How can I help you today?", resp.Content)
	assert.Equal(t, 40, resp.TokensUsed)
}

func TestQwenProvider_Complete_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := QwenError{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			}{
				Message: "Invalid API key",
				Type:    "authentication_error",
				Code:    "401",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewQwenProvider("invalid-api-key", server.URL, "qwen-turbo")
	provider.httpClient = server.Client()

	req := &models.LLMRequest{
		ID: "test-req-456",
		ModelParams: models.ModelParameters{
			Model: "qwen-turbo",
		},
		Prompt: "Test prompt",
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "Qwen API error: Invalid API key (authentication_error)")
}

func TestQwenProvider_Complete_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := QwenResponse{
			ID:      "chatcmpl-789",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "qwen-turbo",
			Choices: []QwenChoice{},
			Usage: QwenUsage{
				PromptTokens:     5,
				CompletionTokens: 0,
				TotalTokens:      5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewQwenProvider("test-api-key", server.URL, "qwen-turbo")
	provider.httpClient = server.Client()

	req := &models.LLMRequest{
		ID: "test-req-789",
		ModelParams: models.ModelParameters{
			Model: "qwen-turbo",
		},
		Prompt: "Test prompt",
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "no choices returned from Qwen API")
}

func TestQwenProvider_Complete_NetworkError(t *testing.T) {
	provider := NewQwenProvider("test-api-key", "https://invalid-url-that-does-not-exist.example.com", "qwen-turbo")
	// Create a client that will fail quickly
	provider.httpClient = &http.Client{
		Timeout: 1 * time.Millisecond,
	}

	req := &models.LLMRequest{
		ID: "test-req-network",
		ModelParams: models.ModelParameters{
			Model: "qwen-turbo",
		},
		Prompt: "Test prompt",
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to complete request")
}

func TestQwenProvider_Complete_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	provider := NewQwenProvider("test-api-key", server.URL, "qwen-turbo")
	provider.httpClient = server.Client()

	req := &models.LLMRequest{
		ID: "test-req-json",
		ModelParams: models.ModelParameters{
			Model: "qwen-turbo",
		},
		Prompt: "Test prompt",
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to unmarshal response")
}

func TestQwenProvider_CompleteStream(t *testing.T) {
	t.Skip("Streaming test needs proper server setup - TODO: implement later")
}

func TestQwenProvider_HealthCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/models", r.URL.Path)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"id": "qwen-turbo"}]}`))
	}))
	defer server.Close()

	provider := NewQwenProvider("test-api-key", server.URL, "qwen-turbo")
	provider.httpClient = server.Client()

	err := provider.HealthCheck()
	assert.NoError(t, err)
}

func TestQwenProvider_HealthCheck_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Invalid API key"}`))
	}))
	defer server.Close()

	provider := NewQwenProvider("invalid-api-key", server.URL, "qwen-turbo")
	provider.httpClient = server.Client()

	err := provider.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed with status: 401")
}

func TestQwenProvider_HealthCheck_EmptyAPIKey(t *testing.T) {
	provider := NewQwenProvider("", "https://api.example.com", "qwen-turbo")
	// Create a client that will fail quickly
	provider.httpClient = &http.Client{
		Timeout: 1 * time.Millisecond,
	}

	err := provider.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check request failed")
}

func TestQwenProvider_GetCapabilities(t *testing.T) {
	provider := NewQwenProvider("test-api-key", "https://api.example.com", "qwen-turbo")
	caps := provider.GetCapabilities()

	assert.NotNil(t, caps)
	assert.NotEmpty(t, caps.SupportedModels)
	assert.Contains(t, caps.SupportedModels, "qwen-turbo")
	assert.Contains(t, caps.SupportedModels, "qwen-plus")
	assert.Contains(t, caps.SupportedModels, "qwen-max")
	assert.Contains(t, caps.SupportedModels, "qwen-max-longcontext")

	assert.Contains(t, caps.SupportedFeatures, "text_completion")
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.Contains(t, caps.SupportedFeatures, "function_calling")

	assert.Contains(t, caps.SupportedRequestTypes, "text_completion")
	assert.Contains(t, caps.SupportedRequestTypes, "chat")

	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.False(t, caps.SupportsVision)
	assert.True(t, caps.SupportsTools)
	assert.False(t, caps.SupportsSearch)
	assert.True(t, caps.SupportsReasoning)
	assert.True(t, caps.SupportsCodeCompletion)
	assert.True(t, caps.SupportsCodeAnalysis)
	assert.False(t, caps.SupportsRefactoring)

	assert.Equal(t, 6000, caps.Limits.MaxTokens)
	assert.Equal(t, 30000, caps.Limits.MaxInputLength)
	assert.Equal(t, 2000, caps.Limits.MaxOutputLength)
	assert.Equal(t, 50, caps.Limits.MaxConcurrentRequests)

	assert.Equal(t, "Alibaba Cloud", caps.Metadata["provider"])
	assert.Equal(t, "Qwen", caps.Metadata["model_family"])
	assert.Equal(t, "v1", caps.Metadata["api_version"])
}

func TestQwenProvider_ValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		baseURL   string
		model     string
		config    map[string]interface{}
		wantValid bool
		wantErrs  []string
	}{
		{
			name:      "valid config",
			apiKey:    "test-api-key",
			baseURL:   "https://api.example.com",
			model:     "qwen-turbo",
			config:    map[string]interface{}{},
			wantValid: true,
			wantErrs:  nil,
		},
		{
			name:      "empty api key",
			apiKey:    "",
			baseURL:   "https://api.example.com",
			model:     "qwen-turbo",
			config:    map[string]interface{}{},
			wantValid: false,
			wantErrs:  []string{"API key is required"},
		},
		{
			name:      "empty base URL - gets default",
			apiKey:    "test-api-key",
			baseURL:   "",
			model:     "qwen-turbo",
			config:    map[string]interface{}{},
			wantValid: true, // Empty base URL gets default value
			wantErrs:  nil,
		},
		{
			name:      "empty model - gets default",
			apiKey:    "test-api-key",
			baseURL:   "https://api.example.com",
			model:     "",
			config:    map[string]interface{}{},
			wantValid: true, // Empty model gets default value
			wantErrs:  nil,
		},
		{
			name:      "only api key error",
			apiKey:    "",
			baseURL:   "",
			model:     "",
			config:    map[string]interface{}{},
			wantValid: false,
			wantErrs:  []string{"API key is required"}, // Only API key error since baseURL and model get defaults
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewQwenProvider(tt.apiKey, tt.baseURL, tt.model)
			valid, errs := provider.ValidateConfig(tt.config)
			assert.Equal(t, tt.wantValid, valid)
			assert.Equal(t, tt.wantErrs, errs)
		})
	}
}

func TestQwenProvider_Complete_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)
		response := QwenResponse{
			ID:      "chatcmpl-slow",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "qwen-turbo",
			Choices: []QwenChoice{
				{
					Index: 0,
					Message: QwenMessage{
						Role:    "assistant",
						Content: "Slow response",
					},
					FinishReason: "stop",
				},
			},
			Usage: QwenUsage{
				PromptTokens:     5,
				CompletionTokens: 10,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewQwenProvider("test-api-key", server.URL, "qwen-turbo")
	provider.httpClient = server.Client()

	req := &models.LLMRequest{
		ID: "test-req-timeout",
		ModelParams: models.ModelParameters{
			Model: "qwen-turbo",
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

func TestQwenProvider_Complete_WithStopSequences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody QwenRequest
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &reqBody)
		require.NoError(t, err)

		assert.Equal(t, []string{"STOP", "END"}, reqBody.Stop)

		response := QwenResponse{
			ID:      "chatcmpl-stop",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "qwen-turbo",
			Choices: []QwenChoice{
				{
					Index: 0,
					Message: QwenMessage{
						Role:    "assistant",
						Content: "Response with stop sequences",
					},
					FinishReason: "stop",
				},
			},
			Usage: QwenUsage{
				PromptTokens:     8,
				CompletionTokens: 12,
				TotalTokens:      20,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewQwenProvider("test-api-key", server.URL, "qwen-turbo")
	provider.httpClient = server.Client()

	req := &models.LLMRequest{
		ID: "test-req-stop",
		ModelParams: models.ModelParameters{
			Model:         "qwen-turbo",
			StopSequences: []string{"STOP", "END"},
			Temperature:   0.8,
			TopP:          0.9,
			MaxTokens:     500,
		},
		Prompt: "Generate text but stop when you see STOP or END",
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Response with stop sequences", resp.Content)
	assert.Equal(t, 20, resp.TokensUsed)
}
