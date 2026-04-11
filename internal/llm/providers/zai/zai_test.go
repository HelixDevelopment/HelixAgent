package zai

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

func TestNewZAIProvider(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		apiKey   string
		baseURL  string
		model    string
		expected *ZAIProvider
	}{
		{
			name:    "default values",
			apiKey:  "test-key",
			baseURL: "",
			model:   "",
			expected: &ZAIProvider{
				apiKey:  "test-key",
				baseURL: "https://api.z.ai/api/paas/v4",
				model:   "glm-4.5",
				httpClient: &http.Client{
					Timeout: 60 * time.Second,
				},
			},
		},
		{
			name:    "custom values",
			apiKey:  "custom-key",
			baseURL: "https://custom.z.ai/v2",
			model:   "custom-model",
			expected: &ZAIProvider{
				apiKey:  "custom-key",
				baseURL: "https://custom.z.ai/v2",
				model:   "custom-model",
				httpClient: &http.Client{
					Timeout: 60 * time.Second,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := NewZAIProvider(tt.apiKey, tt.baseURL, tt.model)
			assert.Equal(t, tt.expected.apiKey, provider.apiKey)
			assert.Equal(t, tt.expected.baseURL, provider.baseURL)
			assert.Equal(t, tt.expected.model, provider.model)
			assert.Equal(t, tt.expected.httpClient.Timeout, provider.httpClient.Timeout)
		})
	}
}

func TestZAIProvider_Complete(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req ZAIRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "glm-4-plus", req.Model)
		assert.Equal(t, "test prompt", req.Prompt)
		assert.False(t, req.Stream)
		assert.Equal(t, 0.7, req.Temperature)

		response := ZAIResponse{
			ID:      "resp-123",
			Object:  "text_completion",
			Created: time.Now().Unix(),
			Model:   "z-ai-base",
			Choices: []ZAIChoice{
				{
					Index:        0,
					Text:         "Test response",
					FinishReason: "stop",
				},
			},
			Usage: ZAIUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "glm-4-plus")

	req := &models.LLMRequest{
		ID:     "test-123",
		Prompt: "test prompt",
		ModelParams: models.ModelParameters{
			Temperature: 0.7,
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test-123", resp.RequestID)
	assert.Equal(t, "zai", resp.ProviderID)
	assert.Equal(t, "Z.AI", resp.ProviderName)
	assert.Equal(t, "Test response", resp.Content)
	assert.Equal(t, 0.80, resp.Confidence)
	assert.Equal(t, 15, resp.TokensUsed)
	assert.Equal(t, "stop", resp.FinishReason)
}

func TestZAIProvider_Complete_ChatFormat(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)

		var req ZAIRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Len(t, req.Messages, 2)
		assert.Equal(t, "user", req.Messages[0].Role)
		assert.Equal(t, "Hello", req.Messages[0].Content)

		response := ZAIResponse{
			ID:      "resp-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "z-ai-base",
			Choices: []ZAIChoice{
				{
					Index: 0,
					Message: ZAIMessage{
						Role:    "assistant",
						Content: "Chat response",
					},
					FinishReason: "stop",
				},
			},
			Usage: ZAIUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "glm-4-plus")

	req := &models.LLMRequest{
		ID: "test-123",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
		ModelParams: models.ModelParameters{
			Temperature: 0.7,
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Chat response", resp.Content)
	assert.Equal(t, "chat.completion", resp.Metadata["object"])
}

func TestZAIProvider_Complete_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ZAIError{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			}{
				Message: "Invalid request",
				Type:    "invalid_request_error",
				Code:    "400",
			},
		})
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "glm-4-plus")

	req := &models.LLMRequest{
		ID:     "test-123",
		Prompt: "test prompt",
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "Zhipu GLM API error")
}

func TestZAIProvider_Complete_NoChoices(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ZAIResponse{
			ID:      "resp-123",
			Object:  "text_completion",
			Created: time.Now().Unix(),
			Model:   "z-ai-base",
			Choices: []ZAIChoice{}, // Empty choices
			Usage: ZAIUsage{
				PromptTokens:     10,
				CompletionTokens: 0,
				TotalTokens:      10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "glm-4-plus")

	req := &models.LLMRequest{
		ID:     "test-123",
		Prompt: "test prompt",
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "no choices returned")
}

func TestZAIProvider_CompleteStream(t *testing.T) {
	t.Parallel()
	// Create a mock SSE server for streaming responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		var req ZAIRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.True(t, req.Stream)

		// Send SSE streaming response
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter does not support flushing")
		}

		// Send chunk 1
		chunk1 := ZAIStreamResponse{
			ID:      "stream-123",
			Object:  "text_completion.chunk",
			Created: time.Now().Unix(),
			Model:   "z-ai-base",
			Choices: []ZAIStreamChoice{
				{Index: 0, Delta: ZAIStreamDelta{Content: "Hello "}, FinishReason: nil},
			},
		}
		data1, _ := json.Marshal(chunk1)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data1)
		flusher.Flush()

		// Send chunk 2
		chunk2 := ZAIStreamResponse{
			ID:      "stream-123",
			Object:  "text_completion.chunk",
			Created: time.Now().Unix(),
			Model:   "z-ai-base",
			Choices: []ZAIStreamChoice{
				{Index: 0, Delta: ZAIStreamDelta{Content: "World!"}, FinishReason: nil},
			},
		}
		data2, _ := json.Marshal(chunk2)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data2)
		flusher.Flush()

		// Send final chunk with finish reason
		finishReason := "stop"
		chunk3 := ZAIStreamResponse{
			ID:      "stream-123",
			Object:  "text_completion.chunk",
			Created: time.Now().Unix(),
			Model:   "z-ai-base",
			Choices: []ZAIStreamChoice{
				{Index: 0, Delta: ZAIStreamDelta{Content: ""}, FinishReason: &finishReason},
			},
		}
		data3, _ := json.Marshal(chunk3)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data3)
		flusher.Flush()

		// Send DONE marker
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "glm-4-plus")

	req := &models.LLMRequest{
		ID:     "test-123",
		Prompt: "test prompt",
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, ch)

	// Collect responses
	var responses []*models.LLMResponse
	for resp := range ch {
		responses = append(responses, resp)
	}

	// Verify we received chunks
	assert.GreaterOrEqual(t, len(responses), 1, "should receive at least one response")

	// Verify final response
	if len(responses) > 0 {
		lastResp := responses[len(responses)-1]
		assert.Equal(t, "test-123", lastResp.RequestID)
		assert.Equal(t, "zai", lastResp.ProviderID)
		assert.Equal(t, "Z.AI", lastResp.ProviderName)
	}
}

func TestZAIProvider_CompleteStream_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ZAIError{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			}{
				Message: "Invalid streaming request",
				Type:    "invalid_request_error",
				Code:    "400",
			},
		})
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "glm-4-plus")

	req := &models.LLMRequest{
		ID:     "test-123",
		Prompt: "test prompt",
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, ch)
	assert.Contains(t, err.Error(), "Invalid streaming request")
}

func TestZAIProvider_HealthCheck(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		responseStatus int
		expectError    bool
	}{
		{
			name:           "healthy",
			responseStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "unhealthy",
			responseStatus: http.StatusServiceUnavailable,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/models", r.URL.Path)
				assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
				w.WriteHeader(tt.responseStatus)
			}))
			defer server.Close()

			provider := NewZAIProvider("test-api-key", server.URL, "glm-4-plus")
			err := provider.HealthCheck()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestZAIProvider_HealthCheck_NetworkError(t *testing.T) {
	t.Parallel()
	provider := NewZAIProvider("test-api-key", "http://invalid-url:1234", "z-ai-base")
	err := provider.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check request failed")
}

func TestZAIProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	provider := NewZAIProvider("", "", "")
	caps := provider.GetCapabilities()

	assert.NotNil(t, caps)
	// GLM-4 series models
	assert.Contains(t, caps.SupportedModels, "glm-4-plus")
	assert.Contains(t, caps.SupportedModels, "glm-4")
	assert.Contains(t, caps.SupportedModels, "glm-4-flash")
	assert.Contains(t, caps.SupportedFeatures, "text_completion")
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.Contains(t, caps.SupportedFeatures, "function_calling")
	assert.Contains(t, caps.SupportedFeatures, "code_generation")
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsTools)
	assert.True(t, caps.SupportsVision) // GLM-4V supports vision
	assert.Equal(t, 8192, caps.Limits.MaxTokens)
	assert.Equal(t, 128000, caps.Limits.MaxInputLength) // 128K context
	assert.Equal(t, 20, caps.Limits.MaxConcurrentRequests)
	assert.Equal(t, "Zhipu AI (GLM)", caps.Metadata["provider"])
	assert.Equal(t, "v4", caps.Metadata["api_version"])
}

func TestZAIProvider_ValidateConfig(t *testing.T) {
	t.Parallel()
	// Note: NewZAIProvider sets defaults for baseURL and model,
	// so only API key validation will fail when not provided
	tests := []struct {
		name        string
		apiKey      string
		baseURL     string
		model       string
		expectValid bool
		expectedErr []string
	}{
		{
			name:        "valid config",
			apiKey:      "test-key",
			baseURL:     "https://api.z.ai/v1",
			model:       "z-ai-base",
			expectValid: true,
			expectedErr: []string{},
		},
		{
			name:        "missing API key",
			apiKey:      "",
			baseURL:     "https://api.z.ai/v1",
			model:       "z-ai-base",
			expectValid: false,
			expectedErr: []string{"API key is required"},
		},
		{
			name:        "empty base URL uses default",
			apiKey:      "test-key",
			baseURL:     "",
			model:       "z-ai-base",
			expectValid: true, // NewZAIProvider sets default baseURL
			expectedErr: []string{},
		},
		{
			name:        "empty model uses default",
			apiKey:      "test-key",
			baseURL:     "https://api.z.ai/v1",
			model:       "",
			expectValid: true, // NewZAIProvider sets default model
			expectedErr: []string{},
		},
		{
			name:        "missing API key with defaults",
			apiKey:      "",
			baseURL:     "",
			model:       "",
			expectValid: false,
			expectedErr: []string{"API key is required"}, // Only API key error since others have defaults
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := NewZAIProvider(tt.apiKey, tt.baseURL, tt.model)
			valid, errs := provider.ValidateConfig(nil)

			assert.Equal(t, tt.expectValid, valid)
			assert.Equal(t, len(tt.expectedErr), len(errs))
			for i, expectedErr := range tt.expectedErr {
				if i < len(errs) {
					assert.Contains(t, errs[i], expectedErr)
				}
			}
		})
	}
}

func TestZAIProvider_convertToZAIRequest(t *testing.T) {
	t.Parallel()
	provider := NewZAIProvider("test-key", "https://api.z.ai/v1", "z-ai-base")

	t.Run("completion format", func(t *testing.T) {
		t.Parallel()
		req := &models.LLMRequest{
			ID:     "test-123",
			Prompt: "test prompt",
			ModelParams: models.ModelParameters{
				Temperature:   0.7,
				MaxTokens:     100,
				TopP:          0.9,
				StopSequences: []string{"stop1", "stop2"},
			},
		}

		zaiReq := provider.convertToZAIRequest(req)

		assert.Equal(t, "z-ai-base", zaiReq.Model)
		assert.Equal(t, "test prompt", zaiReq.Prompt)
		assert.False(t, zaiReq.Stream)
		assert.Equal(t, 0.7, zaiReq.Temperature)
		assert.Equal(t, 100, zaiReq.MaxTokens)
		assert.Equal(t, 0.9, zaiReq.TopP)
		assert.Equal(t, []string{"stop1", "stop2"}, zaiReq.Stop)
		assert.Empty(t, zaiReq.Messages)
	})

	t.Run("chat format", func(t *testing.T) {
		t.Parallel()
		req := &models.LLMRequest{
			ID: "test-123",
			Messages: []models.Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "How are you?"},
			},
			ModelParams: models.ModelParameters{
				Temperature: 0.5,
			},
		}

		zaiReq := provider.convertToZAIRequest(req)

		assert.Equal(t, "z-ai-base", zaiReq.Model)
		assert.Empty(t, zaiReq.Prompt)
		assert.Len(t, zaiReq.Messages, 3)
		assert.Equal(t, "user", zaiReq.Messages[0].Role)
		assert.Equal(t, "Hello", zaiReq.Messages[0].Content)
		assert.Equal(t, "assistant", zaiReq.Messages[1].Role)
		assert.Equal(t, "Hi there!", zaiReq.Messages[1].Content)
		assert.Equal(t, "user", zaiReq.Messages[2].Role)
		assert.Equal(t, "How are you?", zaiReq.Messages[2].Content)
	})
}

func TestZAIProvider_convertFromZAIResponse(t *testing.T) {
	t.Parallel()
	provider := NewZAIProvider("test-key", "https://api.z.ai/v1", "z-ai-base")

	t.Run("text completion response", func(t *testing.T) {
		t.Parallel()
		zaiResp := &ZAIResponse{
			ID:      "resp-123",
			Object:  "text_completion",
			Created: time.Now().Unix(),
			Model:   "z-ai-base",
			Choices: []ZAIChoice{
				{
					Index:        0,
					Text:         "Test response",
					FinishReason: "stop",
				},
			},
			Usage: ZAIUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		resp, err := provider.convertFromZAIResponse(zaiResp, "test-123")
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "test-123", resp.RequestID)
		assert.Equal(t, "zai", resp.ProviderID)
		assert.Equal(t, "Z.AI", resp.ProviderName)
		assert.Equal(t, "Test response", resp.Content)
		assert.Equal(t, 0.80, resp.Confidence)
		assert.Equal(t, 15, resp.TokensUsed)
		assert.Equal(t, "stop", resp.FinishReason)
		assert.Equal(t, "z-ai-base", resp.Metadata["model"])
		assert.Equal(t, "text_completion", resp.Metadata["object"])
		assert.Equal(t, 10, resp.Metadata["prompt_tokens"])
		assert.Equal(t, 5, resp.Metadata["completion_tokens"])
	})

	t.Run("chat completion response", func(t *testing.T) {
		t.Parallel()
		zaiResp := &ZAIResponse{
			ID:      "resp-456",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "z-ai-pro",
			Choices: []ZAIChoice{
				{
					Index: 0,
					Message: ZAIMessage{
						Role:    "assistant",
						Content: "Chat response",
					},
					FinishReason: "stop",
				},
			},
			Usage: ZAIUsage{
				PromptTokens:     20,
				CompletionTokens: 10,
				TotalTokens:      30,
			},
		}

		resp, err := provider.convertFromZAIResponse(zaiResp, "test-456")
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "Chat response", resp.Content)
		assert.Equal(t, "chat.completion", resp.Metadata["object"])
	})

	t.Run("no content in response", func(t *testing.T) {
		t.Parallel()
		zaiResp := &ZAIResponse{
			ID:      "resp-789",
			Object:  "text_completion",
			Created: time.Now().Unix(),
			Model:   "z-ai-base",
			Choices: []ZAIChoice{
				{
					Index:        0,
					Text:         "", // Empty text
					FinishReason: "stop",
				},
			},
			Usage: ZAIUsage{
				PromptTokens:     5,
				CompletionTokens: 0,
				TotalTokens:      5,
			},
		}

		resp, err := provider.convertFromZAIResponse(zaiResp, "test-789")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "no content or tool_calls found")
	})
}

func TestZAIProvider_makeRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		request        *ZAIRequest
		response       *ZAIResponse
		responseStatus int
		expectError    bool
	}{
		{
			name: "successful completion request",
			request: &ZAIRequest{
				Model:  "z-ai-base",
				Prompt: "test",
				Stream: false,
			},
			response: &ZAIResponse{
				ID:      "resp-123",
				Object:  "text_completion",
				Created: time.Now().Unix(),
				Model:   "z-ai-base",
				Choices: []ZAIChoice{
					{
						Index:        0,
						Text:         "test response",
						FinishReason: "stop",
					},
				},
				Usage: ZAIUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
			responseStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name: "successful chat request",
			request: &ZAIRequest{
				Model: "z-ai-base",
				Messages: []ZAIMessage{
					{Role: "user", Content: "hello"},
				},
				Stream: false,
			},
			response: &ZAIResponse{
				ID:      "resp-456",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "z-ai-base",
				Choices: []ZAIChoice{
					{
						Index: 0,
						Message: ZAIMessage{
							Role:    "assistant",
							Content: "chat response",
						},
						FinishReason: "stop",
					},
				},
				Usage: ZAIUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
			responseStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name: "API error",
			request: &ZAIRequest{
				Model:  "z-ai-base",
				Prompt: "test",
				Stream: false,
			},
			response:       nil,
			responseStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Check endpoint based on request type
				if len(tt.request.Messages) > 0 {
					assert.Equal(t, "/chat/completions", r.URL.Path)
				} else {
					assert.Equal(t, "/completions", r.URL.Path)
				}

				var req ZAIRequest
				err := json.NewDecoder(r.Body).Decode(&req)
				require.NoError(t, err)

				assert.Equal(t, tt.request.Model, req.Model)
				assert.Equal(t, tt.request.Stream, req.Stream)

				w.WriteHeader(tt.responseStatus)
				if tt.responseStatus == http.StatusOK && tt.response != nil {
					_ = json.NewEncoder(w).Encode(tt.response)
				} else if tt.responseStatus != http.StatusOK {
					_, _ = w.Write([]byte("Bad Request"))
				}
			}))
			defer server.Close()

			provider := NewZAIProvider("test-api-key", server.URL, "glm-4-plus")
			resp, err := provider.makeRequest(context.Background(), tt.request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.response.ID, resp.ID)
				assert.Equal(t, tt.response.Model, resp.Model)
			}
		})
	}
}

func TestZAIProvider_makeRequest_InvalidJSON(t *testing.T) {
	t.Parallel()
	provider := NewZAIProvider("test-api-key", "https://api.z.ai/v1", "z-ai-base")

	// Create an invalid request that can't be marshaled
	req := &ZAIRequest{
		Model:      "z-ai-base",
		Parameters: make(map[string]interface{}),
	}
	req.Parameters["invalid"] = make(chan int) // Channels can't be marshaled to JSON

	_, err := provider.makeRequest(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal request")
}

func TestZAIProvider_makeRequest_NetworkError(t *testing.T) {
	t.Parallel()
	provider := NewZAIProvider("test-api-key", "http://invalid-url:1234", "z-ai-base")

	req := &ZAIRequest{
		Model:  "z-ai-base",
		Prompt: "test",
	}

	_, err := provider.makeRequest(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP request failed")
}

func TestZAIProvider_makeRequest_InvalidResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "glm-4-plus")

	req := &ZAIRequest{
		Model:  "z-ai-base",
		Prompt: "test",
	}

	_, err := provider.makeRequest(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal response")
}

// Benchmark tests
func BenchmarkZAIProvider_Complete(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ZAIResponse{
			ID:      "resp-123",
			Object:  "text_completion",
			Created: time.Now().Unix(),
			Model:   "z-ai-base",
			Choices: []ZAIChoice{
				{
					Index:        0,
					Text:         "Test response",
					FinishReason: "stop",
				},
			},
			Usage: ZAIUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "glm-4-plus")
	req := &models.LLMRequest{
		ID:     "test-123",
		Prompt: "test prompt",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = provider.Complete(context.Background(), req)
	}
}

func BenchmarkZAIProvider_GetCapabilities(b *testing.B) {
	provider := NewZAIProvider("", "", "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.GetCapabilities()
	}
}

func BenchmarkZAIProvider_convertToZAIRequest(b *testing.B) {
	provider := NewZAIProvider("test-key", "https://api.z.ai/v1", "z-ai-base")
	req := &models.LLMRequest{
		ID:     "test-123",
		Prompt: "test prompt",
		ModelParams: models.ModelParameters{
			Temperature: 0.7,
			MaxTokens:   100,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.convertToZAIRequest(req)
	}
}

func TestZAIProvider_ZhipuErrorCodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		errorCode     string
		errorMsg      string
		expectContain string
	}{
		{
			name:          "insufficient balance",
			errorCode:     "1113",
			errorMsg:      "余额不足或无可用资源包,请充值",
			expectContain: "insufficient balance",
		},
		{
			name:          "model not found",
			errorCode:     "1211",
			errorMsg:      "模型不存在，请检查模型代码",
			expectContain: "not found",
		},
		{
			name:          "unauthorized",
			errorCode:     "401",
			errorMsg:      "令牌已过期",
			expectContain: "API key expired",
		},
		{
			name:          "rate limited",
			errorCode:     "1301",
			errorMsg:      "请求频率过高",
			expectContain: "rate limited",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(ZAIError{
					Error: struct {
						Message string `json:"message"`
						Type    string `json:"type"`
						Code    string `json:"code"`
					}{
						Message: tt.errorMsg,
						Type:    "api_error",
						Code:    tt.errorCode,
					},
				})
			}))
			defer server.Close()

			provider := NewZAIProvider("test-api-key", server.URL, "glm-4.5")
			req := &models.LLMRequest{
				ID:     "test-123",
				Prompt: "test prompt",
			}

			_, err := provider.Complete(context.Background(), req)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectContain)
		})
	}
}

func TestZAIProvider_CurrentGLMModels(t *testing.T) {
	t.Parallel()
	provider := NewZAIProvider("", "", "")
	caps := provider.GetCapabilities()

	currentModels := []string{
		"glm-5",
		"glm-4.7",
		"glm-4.6",
		"glm-4.5",
		"glm-4.5-air",
	}

	for _, model := range currentModels {
		assert.Contains(t, caps.SupportedModels, model, "Expected %s to be in supported models", model)
	}
}

func TestZAIProvider_RetryLogic(t *testing.T) {
	t.Parallel()
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		response := ZAIResponse{
			ID:      "resp-123",
			Object:  "text_completion",
			Created: time.Now().Unix(),
			Model:   "glm-4.5",
			Choices: []ZAIChoice{
				{
					Index:        0,
					Text:         "Success after retry",
					FinishReason: "stop",
				},
			},
			Usage: ZAIUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	retryConfig := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}
	provider := NewZAIProviderWithRetry("test-api-key", server.URL, "glm-4.5", retryConfig)

	req := &models.LLMRequest{
		ID:     "test-123",
		Prompt: "test prompt",
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Success after retry", resp.Content)
	assert.GreaterOrEqual(t, attempts, 3, "Should have retried at least 3 times")
}

func TestZAIProvider_ContextCancellation(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "glm-4.5")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := &models.LLMRequest{
		ID:     "test-123",
		Prompt: "test prompt",
	}

	_, err := provider.Complete(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}

func TestZAIProvider_ToolCalls(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ZAIResponse{
			ID:      "resp-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "glm-4.5",
			Choices: []ZAIChoice{
				{
					Index: 0,
					Message: ZAIMessage{
						Role:    "assistant",
						Content: "",
						ToolCalls: []ZAIToolCall{
							{
								ID:   "call-123",
								Type: "function",
								Function: ZAIToolCallFunction{
									Name:      "get_weather",
									Arguments: `{"location": "Beijing"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: ZAIUsage{
				PromptTokens:     20,
				CompletionTokens: 10,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "glm-4.5")

	req := &models.LLMRequest{
		ID: "test-123",
		Messages: []models.Message{
			{Role: "user", Content: "What's the weather in Beijing?"},
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
							"location": map[string]string{"type": "string"},
						},
					},
				},
			},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "call-123", resp.ToolCalls[0].ID)
	assert.Equal(t, "get_weather", resp.ToolCalls[0].Function.Name)
	assert.Equal(t, `{"location": "Beijing"}`, resp.ToolCalls[0].Function.Arguments)
}
