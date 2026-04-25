package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.agent/internal/llm/providers/cerebras"
	"dev.helix.agent/internal/llm/providers/claude"
	"dev.helix.agent/internal/llm/providers/deepseek"
	"dev.helix.agent/internal/llm/providers/gemini"
	"dev.helix.agent/internal/llm/providers/mistral"
	"dev.helix.agent/internal/llm/providers/qwen"
	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_AllProviders401Retry tests that all providers properly retry on 401 errors
func TestIntegration_AllProviders401Retry(t *testing.T) {
	testCases := []struct {
		name           string
		createProvider func(serverURL string) interface{}
		getComplete    func(provider interface{}, ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
		serverResponse map[string]any
	}{
		{
			name: "Mistral",
			createProvider: func(serverURL string) interface{} {
				return mistral.NewMistralProviderWithRetry("test-key", serverURL, "mistral-large-latest", mistral.RetryConfig{
					MaxRetries:   1,
					InitialDelay: 10 * time.Millisecond,
					MaxDelay:     100 * time.Millisecond,
					Multiplier:   2.0,
				})
			},
			getComplete: func(provider interface{}, ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return provider.(*mistral.MistralProvider).Complete(ctx, req)
			},
			serverResponse: map[string]any{
				"id":      "test-id",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"model":   "mistral-large-latest",
				"choices": []map[string]any{
					{
						"index": 0,
						"message": map[string]any{
							"role":    "assistant",
							"content": "Mistral response",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]any{
					"prompt_tokens":     10,
					"completion_tokens": 5,
					"total_tokens":      15,
				},
			},
		},
		{
			name: "DeepSeek",
			createProvider: func(serverURL string) interface{} {
				return deepseek.NewDeepSeekProviderWithRetry("test-key", serverURL, "deepseek-coder", deepseek.RetryConfig{
					MaxRetries:   1,
					InitialDelay: 10 * time.Millisecond,
					MaxDelay:     100 * time.Millisecond,
					Multiplier:   2.0,
				})
			},
			getComplete: func(provider interface{}, ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return provider.(*deepseek.DeepSeekProvider).Complete(ctx, req)
			},
			serverResponse: map[string]any{
				"id":      "test-id",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"model":   "deepseek-coder",
				"choices": []map[string]any{
					{
						"index": 0,
						"message": map[string]any{
							"role":    "assistant",
							"content": "DeepSeek response",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]any{
					"prompt_tokens":     10,
					"completion_tokens": 5,
					"total_tokens":      15,
				},
			},
		},
		{
			name: "Cerebras",
			createProvider: func(serverURL string) interface{} {
				return cerebras.NewCerebrasProviderWithRetry("test-key", serverURL, "llama-3.3-70b", cerebras.RetryConfig{
					MaxRetries:   1,
					InitialDelay: 10 * time.Millisecond,
					MaxDelay:     100 * time.Millisecond,
					Multiplier:   2.0,
				})
			},
			getComplete: func(provider interface{}, ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
				return provider.(*cerebras.CerebrasProvider).Complete(ctx, req)
			},
			serverResponse: map[string]any{
				"id":      "test-id",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"model":   "llama-3.3-70b",
				"choices": []map[string]any{
					{
						"index": 0,
						"message": map[string]any{
							"role":    "assistant",
							"content": "Cerebras response",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]any{
					"prompt_tokens":     10,
					"completion_tokens": 5,
					"total_tokens":      15,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var requestCount int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { // MOCK-OK: #provider-fault-injection
				count := atomic.AddInt32(&requestCount, 1)

				if count == 1 {
					// First request returns 401
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
					return
				}

				// Second request succeeds
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(tc.serverResponse)
			}))
			defer server.Close()

			provider := tc.createProvider(server.URL)
			req := &models.LLMRequest{
				ID: "test-request",
				Messages: []models.Message{
					{Role: "user", Content: "Hello"},
				},
				ModelParams: models.ModelParameters{
					MaxTokens:   100,
					Temperature: 0.7,
				},
			}

			ctx := context.Background()
			resp, err := tc.getComplete(provider, ctx, req)

			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount), "Should have made 2 requests (1 failed + 1 retry)")
		})
	}
}

// TestIntegration_ClaudeProvider401Retry tests Claude provider specifically
func TestIntegration_ClaudeProvider401Retry(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { // MOCK-OK: #provider-fault-injection
		count := atomic.AddInt32(&requestCount, 1)

		if count == 1 {
			// First request returns 401
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "Invalid API key",
					"type":    "authentication_error",
				},
			})
			return
		}

		// Second request succeeds
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":   "msg_test",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{
					"type": "text",
					"text": "Claude response",
				},
			},
			"model":         "claude-3-sonnet-20240229",
			"stop_reason":   "end_turn",
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		})
	}))
	defer server.Close()

	provider := claude.NewClaudeProviderWithRetry("test-key", server.URL, "claude-3-sonnet-20240229", claude.RetryConfig{
		MaxRetries:   1,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	})

	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}

	ctx := context.Background()
	resp, err := provider.Complete(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Claude response", resp.Content)
	assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount))
}

// TestIntegration_GeminiProvider401Retry tests Gemini provider specifically
func TestIntegration_GeminiProvider401Retry(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { // MOCK-OK: #provider-fault-injection
		count := atomic.AddInt32(&requestCount, 1)

		if count == 1 {
			// First request returns 401
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    401,
					"message": "Invalid API key",
					"status":  "UNAUTHENTICATED",
				},
			})
			return
		}

		// Second request succeeds
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{"text": "Gemini response"},
						},
						"role": "model",
					},
					"finishReason": "STOP",
					"index":        0,
				},
			},
			"usageMetadata": map[string]any{
				"promptTokenCount":     10,
				"candidatesTokenCount": 5,
				"totalTokenCount":      15,
			},
		})
	}))
	defer server.Close()

	baseURL := server.URL + "/v1beta/models/%s:generateContent"
	provider := gemini.NewGeminiProviderWithRetry("test-key", baseURL, "gemini-2.0-flash", gemini.RetryConfig{
		MaxRetries:   1,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	})

	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}

	ctx := context.Background()
	resp, err := provider.Complete(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Gemini response", resp.Content)
	assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount))
}

// TestIntegration_QwenProvider401Retry tests Qwen provider specifically
func TestIntegration_QwenProvider401Retry(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { // MOCK-OK: #provider-fault-injection
		count := atomic.AddInt32(&requestCount, 1)

		if count == 1 {
			// First request returns 401
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "Invalid API key",
					"type":    "authentication_error",
					"code":    "invalid_api_key",
				},
			})
			return
		}

		// Second request succeeds
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "qwen-turbo",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Qwen response",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
	}))
	defer server.Close()

	provider := qwen.NewQwenProviderWithRetry("test-key", server.URL, "qwen-turbo", qwen.RetryConfig{
		MaxRetries:   1,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	})

	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}

	ctx := context.Background()
	resp, err := provider.Complete(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Qwen response", resp.Content)
	assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount))
}

// TestIntegration_ConcurrentAuth401Retry tests concurrent requests with 401 retry
func TestIntegration_ConcurrentAuth401Retry(t *testing.T) {
	// Total request count - first 5 requests fail with 401, rest succeed
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { // MOCK-OK: #provider-fault-injection
		count := atomic.AddInt32(&requestCount, 1)

		// First 5 requests fail with 401 (one from each goroutine)
		// Retries (requests 6-10) succeed
		if count <= 5 {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}

		// All retry requests succeed
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "test-id",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "mistral-large-latest",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Test response",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
	}))
	defer server.Close()

	provider := mistral.NewMistralProviderWithRetry("test-key", server.URL, "mistral-large-latest", mistral.RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     50 * time.Millisecond,
		Multiplier:   2.0,
	})

	// Run 5 concurrent requests
	var successfulResponses int32
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := &models.LLMRequest{
				ID: "test-request",
				Messages: []models.Message{
					{Role: "user", Content: "Hello"},
				},
				ModelParams: models.ModelParameters{
					MaxTokens:   100,
					Temperature: 0.7,
				},
			}

			ctx := context.Background()
			resp, err := provider.Complete(ctx, req)
			if err == nil && resp != nil {
				atomic.AddInt32(&successfulResponses, 1)
			}
		}()
	}

	wg.Wait()

	// All 5 requests should succeed (each gets 401 once, then succeeds on retry)
	assert.Equal(t, int32(5), atomic.LoadInt32(&successfulResponses), "All 5 concurrent requests should succeed with retry")
	// Total requests should be ~10 (5 initial + 5 retries)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&requestCount), int32(10), "Should have made at least 10 total requests")
}

// TestIntegration_AuthRetryHeaderPropagation tests that auth headers are properly sent on retry
func TestIntegration_AuthRetryHeaderPropagation(t *testing.T) {
	var requestHeaders []http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { // MOCK-OK: #provider-contract-test
		// Capture headers from each request
		headers := r.Header.Clone()
		requestHeaders = append(requestHeaders, headers)

		if len(requestHeaders) == 1 {
			// First request returns 401
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}

		// Second request succeeds
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "test-id",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "mistral-large-latest",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Test response",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
	}))
	defer server.Close()

	provider := mistral.NewMistralProviderWithRetry("test-api-key", server.URL, "mistral-large-latest", mistral.RetryConfig{
		MaxRetries:   1,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	})

	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   100,
			Temperature: 0.7,
		},
	}

	ctx := context.Background()
	resp, err := provider.Complete(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, requestHeaders, 2, "Should have 2 requests")

	// Both requests should have Authorization header
	for i, headers := range requestHeaders {
		assert.Equal(t, "Bearer test-api-key", headers.Get("Authorization"), "Request %d should have auth header", i+1)
		assert.Equal(t, "application/json", headers.Get("Content-Type"), "Request %d should have content type", i+1)
	}
}
