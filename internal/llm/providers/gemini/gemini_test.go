package gemini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildPromptFromRequest converts an LLMRequest to a plain text prompt
// following the same format used by GeminiACPProvider.Complete
func buildPromptFromRequest(req *models.LLMRequest) string {
	var promptBuilder strings.Builder
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			promptBuilder.WriteString("System: ")
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n\n")
		case "user":
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n")
		case "assistant":
			promptBuilder.WriteString("Assistant: ")
			promptBuilder.WriteString(msg.Content)
			promptBuilder.WriteString("\n\n")
		}
	}

	prompt := promptBuilder.String()
	if prompt == "" && req.Prompt != "" {
		prompt = req.Prompt
	}
	return prompt
}

// ==============================================================================
// Unified Provider Tests
// ==============================================================================

func TestNewGeminiProvider(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		apiKey  string
		baseURL string
		model   string
	}{
		{
			name:    "all parameters provided",
			apiKey:  "test-api-key-123",
			baseURL: "https://custom.example.com/v1beta/models/%s:generateContent",
			model:   "gemini-ultra",
		},
		{
			name:    "default parameters",
			apiKey:  "test-key",
			baseURL: "",
			model:   "",
		},
		{
			name:    "explicit api key override",
			apiKey:  "explicit-key-override",
			baseURL: "https://api.example.com/v1beta/models/%s:generateContent",
			model:   "gemini-pro",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			got := NewGeminiProvider(tt.apiKey, tt.baseURL, tt.model)
			assert.NotNil(t, got)

			// When explicit key provided, it should be used
			if tt.apiKey != "" {
				assert.Equal(t, tt.apiKey, got.apiKey)
			}

			if tt.model != "" {
				assert.Equal(t, tt.model, got.model)
			} else {
				assert.Equal(t, GeminiDefaultModel, got.model)
			}

			// API provider created when any API key is available (explicit or env)
			if got.apiKey != "" {
				assert.NotNil(t, got.apiProvider)
			}
		})
	}
}

func TestNewGeminiUnifiedProvider(t *testing.T) {
	t.Parallel()
	config := DefaultGeminiUnifiedConfig()
	config.APIKey = "test-key"
	config.Model = "gemini-2.5-pro"

	p := NewGeminiUnifiedProvider(config)
	assert.NotNil(t, p)
	assert.Equal(t, "test-key", p.apiKey)
	assert.Equal(t, "gemini-2.5-pro", p.model)
	assert.Equal(t, "auto", p.preferredMethod)
	assert.NotNil(t, p.apiProvider)
}

func TestDefaultGeminiUnifiedConfig(t *testing.T) {
	t.Parallel()
	config := DefaultGeminiUnifiedConfig()
	assert.Equal(t, GeminiDefaultModel, config.Model)
	assert.Equal(t, 180*time.Second, config.Timeout)
	assert.Equal(t, 8192, config.MaxTokens)
	assert.Equal(t, "auto", config.PreferredMethod)
}

func TestGeminiUnifiedProvider_GetName(t *testing.T) {
	t.Parallel()
	p := NewGeminiProvider("key", "", "")
	assert.Equal(t, "gemini", p.GetName())
}

func TestGeminiUnifiedProvider_GetProviderType(t *testing.T) {
	t.Parallel()
	p := NewGeminiProvider("key", "", "")
	assert.Equal(t, "gemini", p.GetProviderType())
}

func TestGeminiUnifiedProvider_SetModel(t *testing.T) {
	t.Parallel()
	p := NewGeminiProvider("key", "", "gemini-2.5-flash")
	assert.Equal(t, "gemini-2.5-flash", p.GetCurrentModel())

	p.SetModel("gemini-2.5-pro")
	assert.Equal(t, "gemini-2.5-pro", p.GetCurrentModel())
}

func TestGeminiUnifiedProvider_GetAvailableAccessMethods(t *testing.T) {
	t.Parallel()
	p := NewGeminiProvider("test-key", "", "")
	methods := p.GetAvailableAccessMethods()
	// Should at least have API when key is provided
	assert.Contains(t, methods, "api")
}

func TestGeminiUnifiedProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	p := NewGeminiProvider("test-key", "", "")
	caps := p.GetCapabilities()

	require.NotNil(t, caps)

	// Check supported models include all generations
	assert.Contains(t, caps.SupportedModels, "gemini-2.5-flash")
	assert.Contains(t, caps.SupportedModels, "gemini-2.5-pro")
	assert.Contains(t, caps.SupportedModels, "gemini-3-pro-preview")
	assert.Contains(t, caps.SupportedModels, "gemini-3.1-pro-preview")

	// Check features
	assert.Contains(t, caps.SupportedFeatures, "extended_thinking")
	assert.Contains(t, caps.SupportedFeatures, "google_search_grounding")
	assert.Contains(t, caps.SupportedFeatures, "streaming")

	// Check booleans
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsVision)
	assert.True(t, caps.SupportsTools)
	assert.True(t, caps.SupportsSearch)
	assert.True(t, caps.SupportsReasoning)

	// Check limits
	assert.Equal(t, 65536, caps.Limits.MaxTokens)
	assert.Equal(t, 10, caps.Limits.MaxConcurrentRequests)

	// Check metadata
	assert.Equal(t, "Google", caps.Metadata["provider"])
	assert.Equal(t, "Gemini", caps.Metadata["model_family"])
}

func TestGeminiUnifiedProvider_ValidateConfig(t *testing.T) {
	t.Parallel()
	t.Run("valid with API key", func(t *testing.T) {
		p := NewGeminiProvider("test-key", "", "")
		valid, issues := p.ValidateConfig(nil)
		assert.True(t, valid)
		assert.Empty(t, issues)
	})

	t.Run("without key uses CLI or env fallback", func(t *testing.T) {
			t.Parallel()
		// This test is environment-dependent: if GEMINI_API_KEY is set in env
		// or Gemini CLI is installed, validation will pass
		config := GeminiUnifiedConfig{
			Model:           GeminiDefaultModel,
			Timeout:         180 * time.Second,
			MaxTokens:       8192,
			PreferredMethod: "auto",
			// Explicitly no API key
		}
		p := NewGeminiUnifiedProvider(config)
		valid, issues := p.ValidateConfig(nil)
		if p.apiKey != "" || IsGeminiCLIInstalled() {
			assert.True(t, valid)
			assert.Empty(t, issues)
		} else {
			assert.False(t, valid)
			assert.NotEmpty(t, issues)
		}
	})
}

func TestGeminiProviderInfo(t *testing.T) {
	t.Parallel()
	info := GetGeminiProviderInfo()
	assert.Equal(t, "gemini", info["id"])
	assert.Equal(t, "Gemini (Google)", info["name"])
	assert.True(t, info["supports_streaming"].(bool))
	assert.True(t, info["supports_tools"].(bool))
	assert.True(t, info["supports_search"].(bool))

	accessMethods := info["access_methods"].([]string)
	assert.Contains(t, accessMethods, "api")
	assert.Contains(t, accessMethods, "cli")
	assert.Contains(t, accessMethods, "acp")
}

func TestGetAllGeminiModels(t *testing.T) {
	t.Parallel()
	models := getAllGeminiModels()
	assert.GreaterOrEqual(t, len(models), 7)
	assert.Contains(t, models, "gemini-2.0-flash")
	assert.Contains(t, models, "gemini-2.5-flash")
	assert.Contains(t, models, "gemini-2.5-pro")
	assert.Contains(t, models, "gemini-3-pro-preview")
	assert.Contains(t, models, "gemini-3.1-pro-preview")
	assert.Contains(t, models, "gemini-embedding-001")
}

func TestBuildPromptFromRequest(t *testing.T) {
	t.Parallel()
	t.Run("from messages", func(t *testing.T) {
		req := &models.LLMRequest{
			Messages: []models.Message{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there"},
			},
		}
		prompt := buildPromptFromRequest(req)
		assert.Contains(t, prompt, "System: You are helpful")
		assert.Contains(t, prompt, "Hello")
		assert.Contains(t, prompt, "Assistant: Hi there")
	})

	t.Run("from prompt field", func(t *testing.T) {
			t.Parallel()
		req := &models.LLMRequest{
			Prompt: "Direct prompt",
		}
		prompt := buildPromptFromRequest(req)
		assert.Equal(t, "Direct prompt", prompt)
	})

	t.Run("empty request", func(t *testing.T) {
			t.Parallel()
		req := &models.LLMRequest{}
		prompt := buildPromptFromRequest(req)
		assert.Empty(t, prompt)
	})
}

func TestGeminiAllModelsComprehensive(t *testing.T) {
	t.Parallel()
	t.Run("returns all expected models", func(t *testing.T) {
		allModels := getAllGeminiModels()

		expectedModels := []string{
			"gemini-3.1-pro-preview",
			"gemini-3-pro-preview",
			"gemini-3-flash-preview",
			"gemini-2.5-pro",
			"gemini-2.5-flash",
			"gemini-2.5-flash-lite",
			"gemini-2.0-flash",
			"gemini-embedding-001",
		}

		for _, expected := range expectedModels {
			assert.Contains(t, allModels, expected,
				"getAllGeminiModels should contain %s", expected)
		}
	})

	t.Run("model names follow gemini pattern", func(t *testing.T) {
			t.Parallel()
		allModels := getAllGeminiModels()

		for _, model := range allModels {
			assert.True(t,
				len(model) > 6 && model[:6] == "gemini",
				"model %q should start with 'gemini'", model)
		}
	})

	t.Run("no duplicate models", func(t *testing.T) {
			t.Parallel()
		allModels := getAllGeminiModels()
		seen := make(map[string]bool, len(allModels))

		for _, model := range allModels {
			assert.False(t, seen[model],
				"model %q appears more than once", model)
			seen[model] = true
		}
	})

	t.Run("embedding model is included", func(t *testing.T) {
			t.Parallel()
		allModels := getAllGeminiModels()
		assert.Contains(t, allModels, "gemini-embedding-001",
			"embedding model should be present")
	})

	t.Run("minimum model count", func(t *testing.T) {
			t.Parallel()
		allModels := getAllGeminiModels()
		assert.GreaterOrEqual(t, len(allModels), 7,
			"should have at least 7 models")
	})

	t.Run("thinking models are subset of all models", func(t *testing.T) {
			t.Parallel()
		allModels := getAllGeminiModels()
		allModelSet := make(map[string]bool, len(allModels))
		for _, m := range allModels {
			allModelSet[m] = true
		}

		for model := range thinkingModels {
			assert.True(t, allModelSet[model],
				"thinking model %q should be in getAllGeminiModels()", model)
		}
	})
}

func TestGeminiUnifiedProvider_FallbackChain(t *testing.T) {
	t.Parallel()
	t.Run("reports all methods failed when API returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": {"code": 500, "message": "Internal error"}}`))
		}))
		defer server.Close()

		config := GeminiUnifiedConfig{
			APIKey:          "test-key",
			BaseURL:         server.URL + "/v1beta/models/%s:generateContent",
			Model:           "gemini-2.5-flash",
			Timeout:         5 * time.Second,
			MaxTokens:       100,
			PreferredMethod: "auto",
		}
		p := NewGeminiUnifiedProvider(config)
		// Override HTTP client on the API provider to use test server
		p.apiProvider.httpClient = server.Client()
		// Override retry config to speed up test
		p.apiProvider.retryConfig = RetryConfig{
			MaxRetries:   0,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   1.0,
		}

		req := &models.LLMRequest{
			ID:     "fallback-test",
			Prompt: "Test",
			ModelParams: models.ModelParameters{
				MaxTokens: 100,
			},
		}

		resp, err := p.Complete(context.Background(), req)
		assert.Error(t, err, "should fail when all methods fail")
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "all Gemini access methods failed",
			"error should mention fallback failure")
	})

	t.Run("API-only mode fails gracefully", func(t *testing.T) {
			t.Parallel()
		config := GeminiUnifiedConfig{
			APIKey:          "bad-key",
			BaseURL:         "http://localhost:1/v1beta/models/%s:generateContent",
			Model:           "gemini-2.5-flash",
			Timeout:         2 * time.Second,
			MaxTokens:       100,
			PreferredMethod: "api",
		}
		p := NewGeminiUnifiedProvider(config)
		p.apiProvider.httpClient = &http.Client{Timeout: 100 * time.Millisecond}
		p.apiProvider.retryConfig = RetryConfig{
			MaxRetries:   0,
			InitialDelay: 1 * time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   1.0,
		}

		req := &models.LLMRequest{
			ID:     "api-only-fail",
			Prompt: "Test",
			ModelParams: models.ModelParameters{
				MaxTokens: 100,
			},
		}

		resp, err := p.Complete(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

// ==============================================================================
// Benchmarks
// ==============================================================================

func BenchmarkBuildPromptFromRequest(b *testing.B) {
	req := &models.LLMRequest{
		Prompt: "System context",
		Messages: []models.Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
			{Role: "user", Content: "How are you?"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildPromptFromRequest(req)
	}
}
