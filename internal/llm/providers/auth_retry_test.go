package providers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// TestAuthRetry_MistralProvider tests that Mistral provider retries on 401
func TestAuthRetry_MistralProvider(t *testing.T) {
	t.Parallel()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		if count == 1 {
			// First request returns 401
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"detail": "Unauthorized",
			})
			return
		}

		// Second request succeeds
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
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

	retryConfig := mistral.RetryConfig{
		MaxRetries:   1,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	provider := mistral.NewMistralProviderWithRetry("test-api-key", server.URL, "mistral-large-latest", retryConfig)

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
	assert.Equal(t, "Test response", resp.Content)
	assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount), "Should have made 2 requests (1 failed + 1 retry)")
}

// TestAuthRetry_ClaudeProvider tests that Claude provider retries on 401
func TestAuthRetry_ClaudeProvider(t *testing.T) {
	t.Parallel()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		if count == 1 {
			// First request returns 401
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "Invalid API key",
					"type":    "authentication_error",
				},
			})
			return
		}

		// Second request succeeds
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":   "msg_test",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{
					"type": "text",
					"text": "Test response from Claude",
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

	retryConfig := claude.RetryConfig{
		MaxRetries:   1,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	provider := claude.NewClaudeProviderWithRetry("test-api-key", server.URL, "claude-3-sonnet-20240229", retryConfig)

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
	assert.Equal(t, "Test response from Claude", resp.Content)
	assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount), "Should have made 2 requests (1 failed + 1 retry)")
}

// TestAuthRetry_DeepSeekProvider tests that DeepSeek provider retries on 401
func TestAuthRetry_DeepSeekProvider(t *testing.T) {
	t.Parallel()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		if count == 1 {
			// First request returns 401
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": "unauthorized",
			})
			return
		}

		// Second request succeeds
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "deepseek-coder",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Test response from DeepSeek",
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

	retryConfig := deepseek.RetryConfig{
		MaxRetries:   1,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	provider := deepseek.NewDeepSeekProviderWithRetry("test-api-key", server.URL, "deepseek-coder", retryConfig)

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
	assert.Equal(t, "Test response from DeepSeek", resp.Content)
	assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount), "Should have made 2 requests (1 failed + 1 retry)")
}

// TestAuthRetry_GeminiProvider tests that Gemini provider retries on 401
func TestAuthRetry_GeminiProvider(t *testing.T) {
	t.Parallel()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		if count == 1 {
			// First request returns 401
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{"text": "Test response from Gemini"},
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

	retryConfig := gemini.RetryConfig{
		MaxRetries:   1,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	// Gemini provider uses URL with model placeholder, so we need to adjust
	baseURL := server.URL + "/v1beta/models/%s:generateContent"
	provider := gemini.NewGeminiProviderWithRetry("test-api-key", baseURL, "gemini-2.0-flash", retryConfig)

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
	assert.Equal(t, "Test response from Gemini", resp.Content)
	assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount), "Should have made 2 requests (1 failed + 1 retry)")
}

// TestAuthRetry_CerebrasProvider tests that Cerebras provider retries on 401
func TestAuthRetry_CerebrasProvider(t *testing.T) {
	t.Parallel()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		if count == 1 {
			// First request returns 401
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "llama-3.3-70b",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Test response from Cerebras",
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

	retryConfig := cerebras.RetryConfig{
		MaxRetries:   1,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	provider := cerebras.NewCerebrasProviderWithRetry("test-api-key", server.URL, "llama-3.3-70b", retryConfig)

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
	assert.Equal(t, "Test response from Cerebras", resp.Content)
	assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount), "Should have made 2 requests (1 failed + 1 retry)")
}

// TestAuthRetry_QwenProvider tests that Qwen provider retries on 401
func TestAuthRetry_QwenProvider(t *testing.T) {
	t.Parallel()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		if count == 1 {
			// First request returns 401
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "qwen-turbo",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Test response from Qwen",
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

	retryConfig := qwen.RetryConfig{
		MaxRetries:   1,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	provider := qwen.NewQwenProviderWithRetry("test-api-key", server.URL, "qwen-turbo", retryConfig)

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
	assert.Equal(t, "Test response from Qwen", resp.Content)
	assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount), "Should have made 2 requests (1 failed + 1 retry)")
}

// TestAuthRetry_NoInfiniteLoop tests that 401 retry doesn't cause infinite loop
func TestAuthRetry_NoInfiniteLoop(t *testing.T) {
	t.Parallel()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		// Always return 401
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"detail": "Unauthorized",
		})
	}))
	defer server.Close()

	retryConfig := mistral.RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	provider := mistral.NewMistralProviderWithRetry("test-api-key", server.URL, "mistral-large-latest", retryConfig)

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
	_, err := provider.Complete(ctx, req)

	require.Error(t, err)
	// Should only retry once for 401 (2 total requests), not follow maxRetries
	assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount), "Should have made exactly 2 requests (1 original + 1 auth retry)")
}

// TestAuthRetry_ContextCancellation tests that context cancellation works during auth retry
func TestAuthRetry_ContextCancellation(t *testing.T) {
	t.Parallel()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		// First request returns 401
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"detail": "Unauthorized",
		})
	}))
	defer server.Close()

	retryConfig := mistral.RetryConfig{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond, // Longer delay to give time for cancellation
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	provider := mistral.NewMistralProviderWithRetry("test-api-key", server.URL, "mistral-large-latest", retryConfig)

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

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := provider.Complete(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

// TestAuthRetry_SuccessFirstTry tests that successful first request doesn't trigger retry
func TestAuthRetry_SuccessFirstTry(t *testing.T) {
	t.Parallel()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		// First request succeeds
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
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

	retryConfig := mistral.RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	provider := mistral.NewMistralProviderWithRetry("test-api-key", server.URL, "mistral-large-latest", retryConfig)

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
	assert.Equal(t, int32(1), atomic.LoadInt32(&requestCount), "Should have made exactly 1 request")
}

// TestAuthRetry_403NotRetried tests that 403 errors are not retried
func TestAuthRetry_403NotRetried(t *testing.T) {
	t.Parallel()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		// Return 403 Forbidden
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"detail": "Forbidden",
		})
	}))
	defer server.Close()

	retryConfig := mistral.RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	provider := mistral.NewMistralProviderWithRetry("test-api-key", server.URL, "mistral-large-latest", retryConfig)

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
	_, err := provider.Complete(ctx, req)

	require.Error(t, err)
	// 403 should not be retried (only 401)
	assert.Equal(t, int32(1), atomic.LoadInt32(&requestCount), "Should have made exactly 1 request (no retry for 403)")
}

// TestAuthRetry_RetryableStatusCodesStillWork tests that 429/5xx retries still work
func TestAuthRetry_RetryableStatusCodesStillWork(t *testing.T) {
	t.Parallel()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		if count == 1 {
			// First request returns 429 (rate limited)
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"detail": "Rate limited",
			})
			return
		}

		if count == 2 {
			// Second request returns 503 (service unavailable)
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"detail": "Service temporarily unavailable",
			})
			return
		}

		// Third request succeeds
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
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

	retryConfig := mistral.RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	provider := mistral.NewMistralProviderWithRetry("test-api-key", server.URL, "mistral-large-latest", retryConfig)

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
	assert.Equal(t, "Test response", resp.Content)
	assert.Equal(t, int32(3), atomic.LoadInt32(&requestCount), "Should have made 3 requests (2 retryable errors + 1 success)")
}
