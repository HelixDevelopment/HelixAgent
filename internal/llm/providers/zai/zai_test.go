package zai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/models"
)

func TestNewZAIProvider(t *testing.T) {
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
				baseURL: "https://api.z.ai/v1",
				model:   "z-ai-base",
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
			provider := NewZAIProvider(tt.apiKey, tt.baseURL, tt.model)
			assert.Equal(t, tt.expected.apiKey, provider.apiKey)
			assert.Equal(t, tt.expected.baseURL, provider.baseURL)
			assert.Equal(t, tt.expected.model, provider.model)
			assert.Equal(t, tt.expected.httpClient.Timeout, provider.httpClient.Timeout)
		})
	}
}

func TestZAIProvider_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req ZAIRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "z-ai-base", req.Model)
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
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "z-ai-base")

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
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "z-ai-base")

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ZAIError{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    int    `json:"code"`
			}{
				Message: "Invalid request",
				Type:    "invalid_request_error",
				Code:    400,
			},
		})
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "z-ai-base")

	req := &models.LLMRequest{
		ID:     "test-123",
		Prompt: "test prompt",
	}

	resp, err := provider.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "Z.AI API error: Invalid request")
}

func TestZAIProvider_Complete_NoChoices(t *testing.T) {
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
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "z-ai-base")

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
	provider := NewZAIProvider("test-api-key", "https://api.z.ai/v1", "z-ai-base")

	req := &models.LLMRequest{
		ID:     "test-123",
		Prompt: "test prompt",
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, ch)
	assert.Contains(t, err.Error(), "streaming not yet implemented")
}

func TestZAIProvider_HealthCheck(t *testing.T) {
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/models", r.URL.Path)
				assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
				w.WriteHeader(tt.responseStatus)
			}))
			defer server.Close()

			provider := NewZAIProvider("test-api-key", server.URL, "z-ai-base")
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
	provider := NewZAIProvider("test-api-key", "http://invalid-url:1234", "z-ai-base")
	err := provider.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check request failed")
}

func TestZAIProvider_GetCapabilities(t *testing.T) {
	provider := NewZAIProvider("", "", "")
	caps := provider.GetCapabilities()

	assert.NotNil(t, caps)
	assert.Contains(t, caps.SupportedModels, "z-ai-base")
	assert.Contains(t, caps.SupportedModels, "z-ai-pro")
	assert.Contains(t, caps.SupportedFeatures, "text_completion")
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.False(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsFunctionCalling)
	assert.False(t, caps.SupportsVision)
	assert.Equal(t, 4096, caps.Limits.MaxTokens)
	assert.Equal(t, 8192, caps.Limits.MaxInputLength)
	assert.Equal(t, 10, caps.Limits.MaxConcurrentRequests)
	assert.Equal(t, "Z.AI", caps.Metadata["provider"])
	assert.Equal(t, "v1", caps.Metadata["api_version"])
}

func TestZAIProvider_ValidateConfig(t *testing.T) {
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
			name:        "missing base URL",
			apiKey:      "test-key",
			baseURL:     "",
			model:       "z-ai-base",
			expectValid: false,
			expectedErr: []string{"base URL is required"},
		},
		{
			name:        "missing model",
			apiKey:      "test-key",
			baseURL:     "https://api.z.ai/v1",
			model:       "",
			expectValid: false,
			expectedErr: []string{"model is required"},
		},
		{
			name:        "missing all",
			apiKey:      "",
			baseURL:     "",
			model:       "",
			expectValid: false,
			expectedErr: []string{"API key is required", "base URL is required", "model is required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	provider := NewZAIProvider("test-key", "https://api.z.ai/v1", "z-ai-base")

	t.Run("completion format", func(t *testing.T) {
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
	provider := NewZAIProvider("test-key", "https://api.z.ai/v1", "z-ai-base")

	t.Run("text completion response", func(t *testing.T) {
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
		assert.Contains(t, err.Error(), "no content found")
	})
}

func TestZAIProvider_makeRequest(t *testing.T) {
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
					json.NewEncoder(w).Encode(tt.response)
				} else if tt.responseStatus != http.StatusOK {
					w.Write([]byte("Bad Request"))
				}
			}))
			defer server.Close()

			provider := NewZAIProvider("test-api-key", server.URL, "z-ai-base")
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "z-ai-base")

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
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewZAIProvider("test-api-key", server.URL, "z-ai-base")
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
