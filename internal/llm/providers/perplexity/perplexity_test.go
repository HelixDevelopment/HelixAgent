package perplexity

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
	assert.Equal(t, PerplexityAPIURL, provider.baseURL)
	assert.Equal(t, DefaultModel, provider.model)
}

func TestNewProviderWithCustomURL(t *testing.T) {
	t.Parallel()
	customURL := "https://custom.perplexity.ai/chat/completions"
	provider := NewProvider("test-api-key", customURL, "llama-3.1-sonar-small-128k-online")
	assert.Equal(t, customURL, provider.baseURL)
	assert.Equal(t, "llama-3.1-sonar-small-128k-online", provider.model)
}

func TestNewProviderWithRetry(t *testing.T) {
	t.Parallel()
	retryConfig := RetryConfig{
		MaxRetries:   5,
		InitialDelay: 2 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   3.0,
	}
	provider := NewProviderWithRetry("test-key", "", "llama-3.1-sonar-huge-128k-online", retryConfig)
	assert.Equal(t, 5, provider.retryConfig.MaxRetries)
	assert.Equal(t, 2*time.Second, provider.retryConfig.InitialDelay)
	assert.Equal(t, "llama-3.1-sonar-huge-128k-online", provider.model)
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
		assert.Equal(t, "llama-3.1-sonar-large-128k-online", req.Model)

		resp := Response{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "llama-3.1-sonar-large-128k-online",
			Choices: []Choice{
				{
					Index:        0,
					Message:      Message{Role: "assistant", Content: "Based on my search, here is the answer."},
					FinishReason: "stop",
				},
			},
			Usage: Usage{
				PromptTokens:     15,
				CompletionTokens: 12,
				TotalTokens:      27,
			},
			Citations: []string{"https://example.com/source1", "https://example.com/source2"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewProvider("test-api-key", server.URL, "llama-3.1-sonar-large-128k-online")
	req := &models.LLMRequest{
		ID:     "req-1",
		Prompt: "You are a helpful assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Search for the latest news about AI"},
		},
		ModelParams: models.ModelParameters{
			Temperature: 0.7,
			MaxTokens:   1000,
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "chatcmpl-123", resp.ID)
	assert.Contains(t, resp.Content, "Based on my search")
	assert.Equal(t, "perplexity", resp.ProviderID)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 27, resp.TokensUsed)
	// Check citations in metadata
	assert.NotNil(t, resp.Metadata["citations"])
}

func TestCompleteWithSearchFilters(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, []string{"wikipedia.org", "bbc.com"}, req.SearchDomainFilter)
		assert.Equal(t, "day", req.SearchRecencyFilter)

		resp := Response{
			ID:      "chatcmpl-filtered",
			Choices: []Choice{{Message: Message{Content: "Filtered search results"}, FinishReason: "stop"}},
			Usage:   Usage{TotalTokens: 20},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewProvider("test-api-key", server.URL, "")
	req := &models.LLMRequest{
		ID:       "req-filtered",
		Messages: []models.Message{{Role: "user", Content: "Search within specific domains"}},
		ModelParams: models.ModelParameters{
			ProviderSpecific: map[string]any{
				"search_domain_filter":  []string{"wikipedia.org", "bbc.com"},
				"search_recency_filter": "day",
			},
		},
	}

	resp, err := provider.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "Filtered search results", resp.Content)
}

func TestCompleteAPIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"message": "Invalid API key"}}`))
	}))
	defer server.Close()

	provider := NewProviderWithRetry("invalid-key", server.URL, "", RetryConfig{MaxRetries: 0})
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
			`data: {"id":"chunk-1","choices":[{"delta":{"content":"Here"}}]}`,
			`data: {"id":"chunk-2","choices":[{"delta":{"content":" is"}}]}`,
			`data: {"id":"chunk-3","choices":[{"delta":{"content":" the"}}],"citations":["https://example.com"]}`,
			`data: {"id":"chunk-4","choices":[{"delta":{"content":" answer"}}]}`,
			`data: [DONE]`,
		}

		for _, event := range events {
			_, _ = w.Write([]byte(event + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	provider := NewProvider("test-api-key", server.URL, "")
	req := &models.LLMRequest{
		ID:       "req-stream",
		Messages: []models.Message{{Role: "user", Content: "Search for something"}},
	}

	ch, err := provider.CompleteStream(context.Background(), req)
	require.NoError(t, err)

	var responses []*models.LLMResponse
	for resp := range ch {
		responses = append(responses, resp)
	}

	require.GreaterOrEqual(t, len(responses), 4)
	lastResp := responses[len(responses)-1]
	assert.Equal(t, "Here is the answer", lastResp.Content)
	assert.Equal(t, "stop", lastResp.FinishReason)
}

func TestCompleteStreamError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error": "Service unavailable"}`))
	}))
	defer server.Close()

	provider := NewProviderWithRetry("test-key", server.URL, "", RetryConfig{MaxRetries: 0})
	req := &models.LLMRequest{
		ID:       "req-stream-error",
		Messages: []models.Message{{Role: "user", Content: "Test"}},
	}

	_, err := provider.CompleteStream(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestGetCapabilities(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	caps := provider.GetCapabilities()

	require.NotNil(t, caps)
	assert.Contains(t, caps.SupportedModels, "llama-3.1-sonar-large-128k-online")
	assert.Contains(t, caps.SupportedModels, "llama-3.1-sonar-small-128k-online")
	assert.Contains(t, caps.SupportedFeatures, "chat")
	assert.Contains(t, caps.SupportedFeatures, "streaming")
	assert.Contains(t, caps.SupportedFeatures, "online_search")
	assert.Contains(t, caps.SupportedFeatures, "citations")
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.SupportsTools) // Perplexity doesn't support tools
	assert.Equal(t, 128000, caps.Limits.MaxTokens)
	assert.Equal(t, "perplexity", caps.Metadata["provider"])
	assert.Equal(t, "search", caps.Metadata["specialization"])
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
	provider := NewProvider("test-api-key", "", "llama-3.1-sonar-large-128k-online")
	req := &models.LLMRequest{
		ID:     "test-id",
		Prompt: "You are a research assistant.",
		Messages: []models.Message{
			{Role: "user", Content: "Search for AI news"},
		},
		ModelParams: models.ModelParameters{
			Model:       "llama-3.1-sonar-huge-128k-online",
			Temperature: 0.5,
			MaxTokens:   2000,
			TopP:        0.9,
		},
	}

	apiReq := provider.convertRequest(req)
	assert.Equal(t, "llama-3.1-sonar-huge-128k-online", apiReq.Model)
	assert.Len(t, apiReq.Messages, 2) // system + user
	assert.Equal(t, "system", apiReq.Messages[0].Role)
	assert.Equal(t, 0.5, apiReq.Temperature)
	assert.Equal(t, 2000, apiReq.MaxTokens)
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
		ID:    "resp-456",
		Model: "llama-3.1-sonar-large-128k-online",
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "Search result"},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     50,
			CompletionTokens: 25,
			TotalTokens:      75,
		},
		Citations: []string{"https://source1.com", "https://source2.com"},
	}

	resp := provider.convertResponse(req, apiResp, startTime)
	assert.Equal(t, "resp-456", resp.ID)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, "Search result", resp.Content)
	assert.Equal(t, "perplexity", resp.ProviderID)
	assert.Equal(t, 75, resp.TokensUsed)
	assert.Equal(t, "stop", resp.FinishReason)
	// Check citations
	citations := resp.Metadata["citations"].([]string)
	assert.Len(t, citations, 2)
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
		{"Short", "stop", 0.9, 1.0},
		{strings.Repeat("Long content ", 20), "stop", 0.95, 1.0},
		{"Short", "length", 0.7, 0.8},
		{"Short", "content_filter", 0.5, 0.6},
	}

	for _, tt := range tests {
		conf := provider.calculateConfidence(tt.content, tt.finishReason)
		assert.GreaterOrEqual(t, conf, tt.minConf)
		assert.LessOrEqual(t, conf, tt.maxConf)
	}
}

func TestCalculateBackoff(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")

	delay1 := provider.calculateBackoff(1)
	delay2 := provider.calculateBackoff(2)

	assert.LessOrEqual(t, delay1, 2*time.Second)
	assert.LessOrEqual(t, delay1, delay2+time.Second)

	delay10 := provider.calculateBackoff(10)
	assert.LessOrEqual(t, delay10, 35*time.Second)
}

func TestGetModel(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "llama-3.1-sonar-large-128k-online")
	assert.Equal(t, "llama-3.1-sonar-large-128k-online", provider.GetModel())
}

func TestSetModel(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "llama-3.1-sonar-large-128k-online")
	provider.SetModel("llama-3.1-sonar-huge-128k-online")
	assert.Equal(t, "llama-3.1-sonar-huge-128k-online", provider.GetModel())
}

func TestGetName(t *testing.T) {
	t.Parallel()
	provider := NewProvider("test-api-key", "", "")
	assert.Equal(t, "perplexity", provider.GetName())
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
			ID:      "success",
			Choices: []Choice{{Message: Message{Content: "Success"}, FinishReason: "stop"}},
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

func TestOnlineVsChatModels(t *testing.T) {
	t.Parallel()
	testModels := []string{
		"llama-3.1-sonar-small-128k-online",
		"llama-3.1-sonar-large-128k-online",
		"llama-3.1-sonar-small-128k-chat",
		"llama-3.1-70b-instruct",
	}

	for _, model := range testModels {
		t.Run(model, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req Request
				_ = json.NewDecoder(r.Body).Decode(&req)
				assert.Equal(t, model, req.Model)

				resp := Response{
					ID:      "test-" + model,
					Model:   model,
					Choices: []Choice{{Message: Message{Content: "Response from " + model}, FinishReason: "stop"}},
				}
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			provider := NewProvider("test-key", server.URL, model)
			req := &models.LLMRequest{
				Messages:    []models.Message{{Role: "user", Content: "Test"}},
				ModelParams: models.ModelParameters{},
			}

			resp, err := provider.Complete(context.Background(), req)
			require.NoError(t, err)
			assert.Contains(t, resp.Content, model)
		})
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkPerplexityProvider_ConvertRequest(b *testing.B) {
	provider := NewProvider("test-key", "", "")
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

func BenchmarkPerplexityProvider_CalculateConfidence(b *testing.B) {
	provider := NewProvider("test-key", "", "")
	content := "This is a sample response from the Perplexity model for confidence scoring."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.calculateConfidence(content, "stop")
	}
}

func BenchmarkPerplexityProvider_GetCapabilities(b *testing.B) {
	provider := NewProvider("test-key", "", "")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.GetCapabilities()
	}
}

func BenchmarkPerplexityProvider_ConvertResponse(b *testing.B) {
	provider := NewProvider("test-key", "", "sonar-pro")
	req := &models.LLMRequest{
		ID: "bench-request",
	}
	resp := &Response{
		ID:    "resp-1",
		Model: "sonar-pro",
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: "assistant", Content: "This is a benchmark response with enough content."},
				FinishReason: "stop",
			},
		},
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
