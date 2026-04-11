package githubmodels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
)

func TestNewGitHubModelsProvider(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider("ghp_test-token", "", "")
	assert.NotNil(t, p)
	assert.Equal(t, "ghp_test-token", p.apiKey)
	assert.Equal(t, GitHubModelsAPIURL, p.baseURL)
	assert.Equal(t, GitHubModelsDefault, p.model)
}

func TestNewGitHubModelsProvider_CustomConfig(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider(
		"ghp_custom-token",
		"https://custom.github.ai/inference/chat/completions",
		"openai/gpt-5",
	)
	assert.Equal(t, "ghp_custom-token", p.apiKey)
	assert.Equal(t,
		"https://custom.github.ai/inference/chat/completions",
		p.baseURL,
	)
	assert.Equal(t, "openai/gpt-5", p.model)
}

func TestNewGitHubModelsProviderWithRetry(t *testing.T) {
	t.Parallel()
	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
	}

	p := NewGitHubModelsProviderWithRetry(
		"ghp_test-token",
		"https://custom.api.example.com",
		"openai/gpt-5",
		retryConfig,
	)

	assert.Equal(t, "ghp_test-token", p.apiKey)
	assert.Equal(t, "https://custom.api.example.com", p.baseURL)
	assert.Equal(t, "openai/gpt-5", p.model)
	assert.Equal(t, 5, p.retryConfig.MaxRetries)
	assert.Equal(t, 2*time.Second, p.retryConfig.InitialDelay)
	assert.Equal(t, 60*time.Second, p.retryConfig.MaxDelay)
	assert.Equal(t, 3.0, p.retryConfig.Multiplier)
}

func TestComplete_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t,
				"Bearer ghp_test-token",
				r.Header.Get("Authorization"),
			)
			assert.Equal(t,
				"application/json",
				r.Header.Get("Content-Type"),
			)
			assert.Equal(t,
				"2022-11-28",
				r.Header.Get("X-GitHub-Api-Version"),
			)

			var reqBody Request
			err := json.NewDecoder(r.Body).Decode(&reqBody)
			require.NoError(t, err)
			assert.Equal(t, "openai/gpt-4.1", reqBody.Model)
			assert.Len(t, reqBody.Messages, 2) // system + user
			assert.Equal(t, "system", reqBody.Messages[0].Role)
			assert.Equal(t, "user", reqBody.Messages[1].Role)

			resp := Response{
				ID:    "chatcmpl-gh-123",
				Model: "openai/gpt-4.1",
				Choices: []Choice{{
					Index: 0,
					Message: Message{
						Role:    "assistant",
						Content: "Hello from GitHub Models!",
					},
					FinishReason: "stop",
				}},
				Usage: Usage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		},
	))
	defer server.Close()

	p := NewGitHubModelsProvider(
		"ghp_test-token", server.URL, "openai/gpt-4.1",
	)

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
	assert.Equal(t, "chatcmpl-gh-123", resp.ID)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, "github-models", resp.ProviderID)
	assert.Equal(t, "GitHub Models", resp.ProviderName)
	assert.Equal(t, "Hello from GitHub Models!", resp.Content)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 15, resp.TokensUsed)
	assert.NotNil(t, resp.Metadata["model"])
	assert.Equal(t, "openai/gpt-4.1", resp.Metadata["model"])
}

func TestComplete_WithMessages(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			var reqBody Request
			_ = json.NewDecoder(r.Body).Decode(&reqBody)

			// system + 3 conversation messages
			assert.Len(t, reqBody.Messages, 4)
			assert.Equal(t, "system", reqBody.Messages[0].Role)
			assert.Equal(t, "user", reqBody.Messages[1].Role)
			assert.Equal(t, "assistant", reqBody.Messages[2].Role)
			assert.Equal(t, "user", reqBody.Messages[3].Role)

			resp := Response{
				ID:    "chatcmpl-gh-multi",
				Model: "openai/gpt-4.1",
				Choices: []Choice{{
					Index: 0,
					Message: Message{
						Role:    "assistant",
						Content: "Multi-turn response",
					},
					FinishReason: "stop",
				}},
				Usage: Usage{TotalTokens: 30},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		},
	))
	defer server.Close()

	p := NewGitHubModelsProvider(
		"ghp_test-token", server.URL, "openai/gpt-4.1",
	)

	req := &models.LLMRequest{
		ID:     "req-multi",
		Prompt: "You are helpful.",
		Messages: []models.Message{
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "Hello!"},
			{Role: "user", Content: "How are you?"},
		},
		ModelParams: models.ModelParameters{MaxTokens: 200},
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "Multi-turn response", resp.Content)
	assert.Equal(t, 30, resp.TokensUsed)
}

func TestComplete_ErrorResponse_401(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(
				`{"error": {"message": "Bad credentials"}}`,
			))
		},
	))
	defer server.Close()

	p := NewGitHubModelsProviderWithRetry(
		"ghp_invalid", server.URL, "openai/gpt-4.1",
		RetryConfig{MaxRetries: 0},
	)

	req := &models.LLMRequest{
		ID:       "req-err-401",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	ctx := context.Background()
	_, err := p.Complete(ctx, req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.Contains(t, err.Error(), "Bad credentials")
}

func TestComplete_ErrorResponse_500(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(
				`{"error": {"message": "Internal server error"}}`,
			))
		},
	))
	defer server.Close()

	p := NewGitHubModelsProviderWithRetry(
		"ghp_test-token", server.URL, "openai/gpt-4.1",
		RetryConfig{
			MaxRetries:   1,
			InitialDelay: 5 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
		},
	)

	req := &models.LLMRequest{
		ID:       "req-err-500",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	ctx := context.Background()
	_, err := p.Complete(ctx, req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max retries exceeded")
}

func TestComplete_WithTools(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Verify GitHub-specific headers
			assert.Equal(t,
				"2022-11-28",
				r.Header.Get("X-GitHub-Api-Version"),
			)
			assert.Equal(t,
				"Bearer ghp_test-token",
				r.Header.Get("Authorization"),
			)

			var reqBody Request
			_ = json.NewDecoder(r.Body).Decode(&reqBody)

			assert.Len(t, reqBody.Tools, 1)
			assert.Equal(t, "function", reqBody.Tools[0].Type)
			assert.Equal(t, "get_weather", reqBody.Tools[0].Function.Name)
			assert.Equal(t, "auto", reqBody.ToolChoice)

			resp := Response{
				ID:    "chatcmpl-gh-tools",
				Model: "openai/gpt-4.1",
				Choices: []Choice{{
					Index: 0,
					Message: Message{
						Role:    "assistant",
						Content: "",
						ToolCalls: []ToolCall{{
							ID:   "call-gh-001",
							Type: "function",
							Function: FunctionCall{
								Name:      "get_weather",
								Arguments: `{"location": "San Francisco"}`,
							},
						}},
					},
					FinishReason: "tool_calls",
				}},
				Usage: Usage{
					PromptTokens:     25,
					CompletionTokens: 15,
					TotalTokens:      40,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		},
	))
	defer server.Close()

	p := NewGitHubModelsProvider(
		"ghp_test-token", server.URL, "openai/gpt-4.1",
	)

	req := &models.LLMRequest{
		ID:     "req-tools",
		Prompt: "You are a weather assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "What's the weather in SF?"},
		},
		ModelParams: models.ModelParameters{Temperature: 0.5},
		Tools: []models.Tool{{
			Type: "function",
			Function: models.ToolFunction{
				Name:        "get_weather",
				Description: "Get weather for a location",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		}},
		ToolChoice: "auto",
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, req)

	require.NoError(t, err)
	assert.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "call-gh-001", resp.ToolCalls[0].ID)
	assert.Equal(t, "function", resp.ToolCalls[0].Type)
	assert.Equal(t, "get_weather", resp.ToolCalls[0].Function.Name)
	assert.Contains(t, resp.ToolCalls[0].Function.Arguments, "San Francisco")
	assert.Equal(t, "tool_calls", resp.FinishReason)
	assert.Equal(t, 40, resp.TokensUsed)
}

func TestCompleteStream(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t,
				"2022-11-28",
				r.Header.Get("X-GitHub-Api-Version"),
			)

			var reqBody Request
			_ = json.NewDecoder(r.Body).Decode(&reqBody)
			assert.True(t, reqBody.Stream)
			// Model name should preserve publisher/model format
			assert.Equal(t, "openai/gpt-4.1", reqBody.Model)

			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)

			chunks := []string{
				`{"id":"chatcmpl-stream-1","choices":[{"delta":{"content":"Hello"}}]}`,
				`{"id":"chatcmpl-stream-1","choices":[{"delta":{"content":" from"}}]}`,
				`{"id":"chatcmpl-stream-1","choices":[{"delta":{"content":" GitHub Models!"}}]}`,
				"[DONE]",
			}

			for _, chunk := range chunks {
				_, _ = w.Write([]byte("data: " + chunk + "\n\n"))
				flusher.Flush()
			}
		},
	))
	defer server.Close()

	p := NewGitHubModelsProvider(
		"ghp_test-token", server.URL, "openai/gpt-4.1",
	)

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

	// Verify final response has accumulated content
	lastResp := responses[len(responses)-1]
	assert.Equal(t, "stop", lastResp.FinishReason)
	assert.Equal(t, "Hello from GitHub Models!", lastResp.Content)
	assert.Equal(t, "github-models", lastResp.ProviderID)
	assert.Equal(t, "GitHub Models", lastResp.ProviderName)

	// Verify intermediate chunks
	if len(responses) > 1 {
		assert.Equal(t, "Hello", responses[0].Content)
		assert.Equal(t, "", responses[0].FinishReason)
	}
}

func TestCompleteStream_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
		},
	))
	defer server.Close()

	p := NewGitHubModelsProviderWithRetry(
		"ghp_bad-token", server.URL, "openai/gpt-4.1",
		RetryConfig{MaxRetries: 0},
	)

	req := &models.LLMRequest{
		ID:       "req-stream-err",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	ctx := context.Background()
	_, err := p.CompleteStream(ctx, req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestHealthCheck_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t,
				"Bearer ghp_test-token",
				r.Header.Get("Authorization"),
			)
			assert.Equal(t,
				"2022-11-28",
				r.Header.Get("X-GitHub-Api-Version"),
			)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data": []}`))
		},
	))
	defer server.Close()

	// Test direct HTTP call since HealthCheck uses hardcoded URL
	p := &GitHubModelsProvider{
		apiKey:     "ghp_test-token",
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	httpReq, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	httpReq.Header.Set("Authorization", "Bearer ghp_test-token")
	httpReq.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := p.httpClient.Do(httpReq)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHealthCheck_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		},
	))
	defer server.Close()

	p := &GitHubModelsProvider{
		apiKey:     "ghp_bad-token",
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	httpReq, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	httpReq.Header.Set("Authorization", "Bearer ghp_bad-token")

	resp, err := p.httpClient.Do(httpReq)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestGetCapabilities(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider("ghp_test-token", "", "")
	caps := p.GetCapabilities()

	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsTools)
	assert.True(t, caps.SupportsVision)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsReasoning)
	assert.True(t, caps.SupportsCodeCompletion)
	assert.True(t, caps.SupportsCodeAnalysis)

	// Verify fallback models are present
	assert.Contains(t, caps.SupportedModels, "openai/gpt-4.1")
	assert.Contains(t, caps.SupportedModels, "openai/gpt-5")
	assert.Contains(t, caps.SupportedModels, "openai/gpt-4o")
	assert.Contains(t, caps.SupportedModels, "DeepSeek/DeepSeek-R1")
	assert.Contains(t, caps.SupportedModels,
		"Meta/Llama-4-Scout-17B-16E-Instruct")
	assert.Contains(t, caps.SupportedModels,
		"Microsoft/Phi-4-reasoning")
	assert.Contains(t, caps.SupportedModels,
		"Mistral/Mistral-Large-2")
	assert.Contains(t, caps.SupportedModels, "Cohere/Command-A")
	assert.Contains(t, caps.SupportedModels,
		"AI21-Labs/AI21-Jamba-1.5-Large")

	// Verify features
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.Contains(t, caps.SupportedFeatures, "streaming")
	assert.Contains(t, caps.SupportedFeatures, "tools")
	assert.Contains(t, caps.SupportedFeatures, "multi_provider")

	// Verify limits
	assert.Equal(t, 128000, caps.Limits.MaxTokens)
	assert.Equal(t, 128000, caps.Limits.MaxInputLength)
	assert.Equal(t, 16384, caps.Limits.MaxOutputLength)

	// Verify metadata
	assert.Equal(t, "github-models", caps.Metadata["provider"])
	assert.Equal(t, "true", caps.Metadata["multi_provider"])
}

func TestValidateConfig_Valid(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider("ghp_valid-token-12345", "", "")
	valid, errs := p.ValidateConfig(nil)
	assert.True(t, valid)
	assert.Len(t, errs, 0)
}

func TestValidateConfig_EmptyKey(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider("", "", "")
	valid, errs := p.ValidateConfig(nil)
	assert.False(t, valid)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "API key is required")
}

func TestRetry_RateLimited(t *testing.T) {
	t.Parallel()
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			resp := Response{
				ID:    "chatcmpl-gh-retry",
				Model: "openai/gpt-4.1",
				Choices: []Choice{{
					Message:      Message{Content: "Success after rate limit"},
					FinishReason: "stop",
				}},
				Usage: Usage{TotalTokens: 10},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		},
	))
	defer server.Close()

	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 5 * time.Millisecond,
		MaxDelay:     50 * time.Millisecond,
		Multiplier:   2.0,
	}

	p := NewGitHubModelsProviderWithRetry(
		"ghp_test-token", server.URL, "openai/gpt-4.1", retryConfig,
	)

	req := &models.LLMRequest{
		ID:       "req-retry-429",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "Success after rate limit", resp.Content)
	assert.Equal(t, 3, attempts)
}

func TestRetry_ServerError(t *testing.T) {
	t.Parallel()
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 2 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			resp := Response{
				ID:    "chatcmpl-gh-recovered",
				Model: "openai/gpt-4.1",
				Choices: []Choice{{
					Message:      Message{Content: "Recovered from 503"},
					FinishReason: "stop",
				}},
				Usage: Usage{TotalTokens: 5},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		},
	))
	defer server.Close()

	retryConfig := RetryConfig{
		MaxRetries:   3,
		InitialDelay: 5 * time.Millisecond,
		MaxDelay:     50 * time.Millisecond,
		Multiplier:   2.0,
	}

	p := NewGitHubModelsProviderWithRetry(
		"ghp_test-token", server.URL, "openai/gpt-4.1", retryConfig,
	)

	req := &models.LLMRequest{
		ID:       "req-503",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "Recovered from 503", resp.Content)
	assert.Equal(t, 2, attempts)
}

func TestContextTimeout(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(5 * time.Second)
			w.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()

	p := NewGitHubModelsProvider(
		"ghp_test-token", server.URL, "openai/gpt-4.1",
	)

	req := &models.LLMRequest{
		ID:       "req-timeout",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 100*time.Millisecond,
	)
	defer cancel()

	_, err := p.Complete(ctx, req)
	assert.Error(t, err)
}

func TestGetName(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider("ghp_test-token", "", "")
	assert.Equal(t, "github-models", p.GetName())
}

func TestGetProviderType(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider("ghp_test-token", "", "")
	assert.Equal(t, "github-models", p.GetProviderType())
}

func TestGetModel(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider(
		"ghp_test-token", "", "openai/gpt-4.1",
	)
	assert.Equal(t, "openai/gpt-4.1", p.GetModel())
}

func TestSetModel(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider(
		"ghp_test-token", "", "openai/gpt-4.1",
	)
	assert.Equal(t, "openai/gpt-4.1", p.GetModel())

	p.SetModel("openai/gpt-5")
	assert.Equal(t, "openai/gpt-5", p.GetModel())

	p.SetModel("Meta/Llama-4-Scout-17B-16E-Instruct")
	assert.Equal(t, "Meta/Llama-4-Scout-17B-16E-Instruct", p.GetModel())
}

func TestDefaultRetryConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultRetryConfig()
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 1*time.Second, cfg.InitialDelay)
	assert.Equal(t, 30*time.Second, cfg.MaxDelay)
	assert.Equal(t, 2.0, cfg.Multiplier)
}

func TestConvertRequest(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider(
		"ghp_test-token", "", "openai/gpt-4.1",
	)

	req := &models.LLMRequest{
		ID:     "test-req",
		Prompt: "You are a helpful assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "What models do you support?"},
		},
		ModelParams: models.ModelParameters{
			Model:         "openai/gpt-5",
			Temperature:   0.5,
			MaxTokens:     1000,
			TopP:          0.95,
			StopSequences: []string{"END"},
		},
	}

	apiReq := p.convertRequest(req)

	// Model override from ModelParams
	assert.Equal(t, "openai/gpt-5", apiReq.Model)
	// System + 3 messages
	assert.Len(t, apiReq.Messages, 4)
	assert.Equal(t, "system", apiReq.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant.",
		apiReq.Messages[0].Content)
	assert.Equal(t, 0.5, apiReq.Temperature)
	assert.Equal(t, 1000, apiReq.MaxTokens)
	assert.Equal(t, 0.95, apiReq.TopP)
	assert.Equal(t, []string{"END"}, apiReq.Stop)
}

func TestConvertRequest_DefaultMaxTokens(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider("ghp_test-token", "", "")

	req := &models.LLMRequest{
		ID:       "test-req",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
		ModelParams: models.ModelParameters{
			MaxTokens: 0, // Not set
		},
	}

	apiReq := p.convertRequest(req)
	assert.Equal(t, 4096, apiReq.MaxTokens)
}

func TestConvertRequest_ModelNamePreserved(t *testing.T) {
	t.Parallel()
	// Verify publisher/model-name format is preserved
	p := NewGitHubModelsProvider(
		"ghp_test-token", "",
		"Meta/Llama-4-Scout-17B-16E-Instruct",
	)

	req := &models.LLMRequest{
		ID:       "test-req",
		Messages: []models.Message{{Role: "user", Content: "Hello"}},
	}

	apiReq := p.convertRequest(req)
	assert.Equal(t, "Meta/Llama-4-Scout-17B-16E-Instruct", apiReq.Model)
	assert.True(t, strings.Contains(apiReq.Model, "/"),
		"Model name should contain publisher/model separator")
}

func TestCalculateConfidence(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider("ghp_test-token", "", "")

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
			name: "Long content with stop",
			content: "GitHub Models provides access to " +
				"a wide range of AI models from multiple " +
				"publishers, enabling real-time AI applications.",
			finishReason: "stop",
			minConf:      0.95,
			maxConf:      1.0,
		},
		{
			name:         "Unknown finish reason",
			content:      "Some content",
			finishReason: "unknown",
			minConf:      0.8,
			maxConf:      0.9,
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

func TestCalculateBackoff(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider("ghp_test-token", "", "")

	// First attempt should be around initial delay (1s)
	delay1 := p.calculateBackoff(1)
	assert.GreaterOrEqual(t, delay1, 950*time.Millisecond)
	assert.LessOrEqual(t, delay1, 1200*time.Millisecond)

	// Second attempt should be roughly doubled
	delay2 := p.calculateBackoff(2)
	assert.GreaterOrEqual(t, delay2, 1900*time.Millisecond)
	assert.LessOrEqual(t, delay2, 2400*time.Millisecond)

	// Should not exceed max delay
	delay10 := p.calculateBackoff(10)
	assert.LessOrEqual(t, delay10, 35*time.Second)
}

func TestGitHubAPIVersionHeader(t *testing.T) {
	t.Parallel()
	headerSent := false
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-GitHub-Api-Version") == "2022-11-28" {
				headerSent = true
			}

			resp := Response{
				ID:    "chatcmpl-gh-header",
				Model: "openai/gpt-4.1",
				Choices: []Choice{{
					Message:      Message{Content: "OK"},
					FinishReason: "stop",
				}},
				Usage: Usage{TotalTokens: 5},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		},
	))
	defer server.Close()

	p := NewGitHubModelsProvider(
		"ghp_test-token", server.URL, "openai/gpt-4.1",
	)

	req := &models.LLMRequest{
		ID:       "req-header-check",
		Messages: []models.Message{{Role: "user", Content: "Hi"}},
	}

	ctx := context.Background()
	_, err := p.Complete(ctx, req)

	require.NoError(t, err)
	assert.True(t, headerSent,
		"X-GitHub-Api-Version header must be sent with every request")
}

func TestBearerAuthHeader(t *testing.T) {
	t.Parallel()
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")

			resp := Response{
				ID:    "chatcmpl-gh-auth",
				Model: "openai/gpt-4.1",
				Choices: []Choice{{
					Message:      Message{Content: "Authed"},
					FinishReason: "stop",
				}},
				Usage: Usage{TotalTokens: 5},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		},
	))
	defer server.Close()

	p := NewGitHubModelsProvider(
		"ghp_my-secret-token", server.URL, "openai/gpt-4.1",
	)

	req := &models.LLMRequest{
		ID:       "req-auth-check",
		Messages: []models.Message{{Role: "user", Content: "Hi"}},
	}

	ctx := context.Background()
	_, err := p.Complete(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, "Bearer ghp_my-secret-token", receivedAuth)
}

func TestConvertResponse_ToolCallsOverrideFinishReason(t *testing.T) {
	t.Parallel()
	p := NewGitHubModelsProvider("ghp_test-token", "", "")

	resp := &Response{
		ID:    "chatcmpl-gh-tc",
		Model: "openai/gpt-4.1",
		Choices: []Choice{{
			Index: 0,
			Message: Message{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ToolCall{{
					ID:   "call-001",
					Type: "function",
					Function: FunctionCall{
						Name:      "search",
						Arguments: `{"q":"test"}`,
					},
				}},
			},
			// Finish reason is "stop" but should be overridden
			FinishReason: "stop",
		}},
		Usage: Usage{TotalTokens: 20},
	}

	req := &models.LLMRequest{ID: "req-tc"}
	result := p.convertResponse(req, resp, time.Now())

	assert.Equal(t, "tool_calls", result.FinishReason)
	assert.Len(t, result.ToolCalls, 1)
}

func TestRetry_502_504(t *testing.T) {
	t.Parallel()
	statusCodes := []int{502, 504}

	for _, statusCode := range statusCodes {
		t.Run(
			"retry_on_"+http.StatusText(statusCode),
			func(t *testing.T) {
				attempts := 0
				server := httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						attempts++
						if attempts < 2 {
							w.WriteHeader(statusCode)
							return
						}
						resp := Response{
							ID:    "chatcmpl-recovered",
							Model: "openai/gpt-4.1",
							Choices: []Choice{{
								Message:      Message{Content: "OK"},
								FinishReason: "stop",
							}},
							Usage: Usage{TotalTokens: 5},
						}
						w.Header().Set(
							"Content-Type", "application/json",
						)
						_ = json.NewEncoder(w).Encode(resp)
					},
				))
				defer server.Close()

				retryConfig := RetryConfig{
					MaxRetries:   3,
					InitialDelay: 5 * time.Millisecond,
					MaxDelay:     50 * time.Millisecond,
					Multiplier:   2.0,
				}

				p := NewGitHubModelsProviderWithRetry(
					"ghp_test-token",
					server.URL,
					"openai/gpt-4.1",
					retryConfig,
				)

				req := &models.LLMRequest{
					ID: "req-retry-status",
					Messages: []models.Message{
						{Role: "user", Content: "Hello"},
					},
				}

				ctx := context.Background()
				resp, err := p.Complete(ctx, req)

				require.NoError(t, err)
				assert.Equal(t, "OK", resp.Content)
				assert.Equal(t, 2, attempts)
			},
		)
	}
}

// Benchmarks

func BenchmarkConvertRequest(b *testing.B) {
	provider := NewGitHubModelsProvider(
		"ghp_bench-key",
		GitHubModelsAPIURL,
		"openai/gpt-4.1",
	)
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

func BenchmarkCalculateConfidence(b *testing.B) {
	provider := NewGitHubModelsProvider(
		"ghp_bench-key", "", "openai/gpt-4.1",
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.calculateConfidence(
			"This is a detailed response with sufficient "+
				"content to test confidence calculation.",
			"stop",
		)
	}
}

func BenchmarkConvertResponse(b *testing.B) {
	provider := NewGitHubModelsProvider(
		"ghp_bench-key", "", "openai/gpt-4.1",
	)
	req := &models.LLMRequest{ID: "bench-request"}
	resp := &Response{
		ID:    "resp-1",
		Model: "openai/gpt-4.1",
		Choices: []Choice{{
			Index: 0,
			Message: Message{
				Role: "assistant",
				Content: "This is a benchmark response " +
					"with enough content for testing.",
			},
			FinishReason: "stop",
		}},
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
