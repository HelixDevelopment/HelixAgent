package publicai

import (
	"context"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestPublicAIProvider_NewProvider(t *testing.T) {
	t.Parallel()
	provider := NewPublicAIProvider("test-api-key", "", "")
	assert.NotNil(t, provider)
	assert.Equal(t, "test-api-key", provider.apiKey)
	assert.Equal(t, PublicAIAPIURL, provider.baseURL)
	assert.Equal(t, PublicAIModel, provider.model)
}

func TestPublicAIProvider_NewProviderWithCustomConfig(t *testing.T) {
	t.Parallel()
	customURL := "https://custom.publicai.co/v1/chat/completions"
	customModel := "swiss-ai/custom-model"
	provider := NewPublicAIProvider("test-key", customURL, customModel)

	assert.Equal(t, customURL, provider.baseURL)
	assert.Equal(t, customModel, provider.model)
}

func TestPublicAIProvider_DefaultRetryConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultRetryConfig()

	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 2*time.Second, cfg.InitialDelay)
	assert.Equal(t, 30*time.Second, cfg.MaxDelay)
	assert.Equal(t, 2.0, cfg.Multiplier)
}

func TestPublicAIProvider_ConvertRequest(t *testing.T) {
	t.Parallel()
	provider := NewPublicAIProvider("test-key", "", "")

	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		Prompt: "You are a helpful assistant",
		ModelParams: models.ModelParameters{
			MaxTokens:   1000,
			Temperature: 0.7,
			TopP:        0.95,
		},
	}

	publicaiReq := provider.convertRequest(req)

	assert.Equal(t, PublicAIModel, publicaiReq.Model)
	assert.Len(t, publicaiReq.Messages, 2)
	assert.Equal(t, "system", publicaiReq.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant", publicaiReq.Messages[0].Content)
	assert.Equal(t, "user", publicaiReq.Messages[1].Role)
	assert.Equal(t, "Hello", publicaiReq.Messages[1].Content)
	assert.Equal(t, 1000, publicaiReq.MaxTokens)
	assert.Equal(t, 0.7, publicaiReq.Temperature)
	assert.Equal(t, 0.95, publicaiReq.TopP)
	assert.False(t, publicaiReq.Stream)
}

func TestPublicAIProvider_ConvertRequestDefaultValues(t *testing.T) {
	t.Parallel()
	provider := NewPublicAIProvider("test-key", "", "")

	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{},
	}

	publicaiReq := provider.convertRequest(req)

	assert.Equal(t, PublicAIMaxOutput, publicaiReq.MaxTokens)
	assert.Equal(t, PublicAIRecommendedTemp, publicaiReq.Temperature)
	assert.Equal(t, PublicAIRecommendedTopP, publicaiReq.TopP)
}

func TestPublicAIProvider_ConvertRequestMaxTokensCap(t *testing.T) {
	t.Parallel()
	provider := NewPublicAIProvider("test-key", "", "")

	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		ModelParams: models.ModelParameters{
			MaxTokens: 50000,
		},
	}

	publicaiReq := provider.convertRequest(req)

	assert.Equal(t, PublicAIMaxOutput, publicaiReq.MaxTokens)
}

func TestPublicAIProvider_CalculateConfidence(t *testing.T) {
	t.Parallel()
	provider := NewPublicAIProvider("test-key", "", "")

	tests := []struct {
		name         string
		content      string
		finishReason string
		expectedMin  float64
		expectedMax  float64
	}{
		{"stop with content", "This is a response that is quite long", "stop", 0.9, 1.0},
		{"stop short", "Hi", "stop", 0.85, 1.0},
		{"length", "This is a longer response", "length", 0.7, 0.9},
		{"no finish reason", "Content here", "", 0.8, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			confidence := provider.calculateConfidence(tt.content, tt.finishReason)
			assert.GreaterOrEqual(t, confidence, tt.expectedMin)
			assert.LessOrEqual(t, confidence, tt.expectedMax)
		})
	}
}

func TestPublicAIProvider_GetCapabilities(t *testing.T) {
	t.Parallel()
	provider := NewPublicAIProvider("test-key", "", "")
	caps := provider.GetCapabilities()

	assert.NotNil(t, caps)
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsCodeCompletion)
	assert.True(t, caps.SupportsCodeAnalysis)
	assert.True(t, caps.SupportsRefactoring)
	assert.False(t, caps.SupportsFunctionCalling)
	assert.False(t, caps.SupportsVision)
	assert.False(t, caps.SupportsTools)
	assert.Equal(t, PublicAIMaxOutput, caps.Limits.MaxTokens)
	assert.Equal(t, PublicAIMaxContext, caps.Limits.MaxInputLength)
	assert.Contains(t, caps.Metadata, "provider")
	assert.Contains(t, caps.Metadata, "model_family")
}

func TestPublicAIProvider_ValidateConfig(t *testing.T) {
	t.Parallel()
	provider := NewPublicAIProvider("test-key", "https://api.publicai.co/v1", "swiss-ai/apertus-8b-instruct")

	valid, errors := provider.ValidateConfig(nil)
	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestPublicAIProvider_ValidateConfigMissingAPIKey(t *testing.T) {
	t.Parallel()
	provider := NewPublicAIProvider("", "https://api.publicai.co/v1", "swiss-ai/apertus-8b-instruct")

	valid, errors := provider.ValidateConfig(nil)
	assert.False(t, valid)
	assert.Contains(t, errors, "API key is required")
}

func TestPublicAIProvider_ValidateConfigMissingBaseURL(t *testing.T) {
	t.Parallel()
	provider := &PublicAIProvider{
		apiKey:  "test-key",
		baseURL: "",
		model:   "swiss-ai/apertus-8b-instruct",
	}

	valid, errors := provider.ValidateConfig(nil)
	assert.False(t, valid)
	assert.Contains(t, errors, "base URL is required")
}

func TestPublicAIProvider_ValidateConfigMissingModel(t *testing.T) {
	t.Parallel()
	provider := &PublicAIProvider{
		apiKey:  "test-key",
		baseURL: "https://api.publicai.co/v1",
		model:   "",
	}

	valid, errors := provider.ValidateConfig(nil)
	assert.False(t, valid)
	assert.Contains(t, errors, "model is required")
}

func TestPublicAIProvider_IsRetryableStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		statusCode int
		expected   bool
	}{
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
		{200, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.statusCode)), func(t *testing.T) {
				t.Parallel()
			result := isRetryableStatus(tt.statusCode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPublicAIProvider_IsAuthRetryableStatus(t *testing.T) {
	t.Parallel()
	assert.True(t, isAuthRetryableStatus(401))
	assert.False(t, isAuthRetryableStatus(403))
	assert.False(t, isAuthRetryableStatus(500))
}

func TestPublicAIProvider_Min(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 5, min(5, 10))
	assert.Equal(t, 3, min(10, 3))
	assert.Equal(t, 7, min(7, 7))
}

func TestPublicAIProvider_Complete_InvalidAPIKey(t *testing.T) {
	t.Parallel()
	provider := NewPublicAIProvider("invalid-key", "", "")

	req := &models.LLMRequest{
		ID: "test-request",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := provider.Complete(ctx, req)
	assert.Error(t, err)
}

func TestPublicAIProvider_HealthCheck_InvalidAPIKey(t *testing.T) {
	t.Parallel()
	provider := NewPublicAIProvider("invalid-key", "", "")

	err := provider.HealthCheck()
	assert.Error(t, err)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkPublicAIProvider_ConvertRequest(b *testing.B) {
	provider := NewPublicAIProvider("test-key", "", "")
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

func BenchmarkPublicAIProvider_GetCapabilities(b *testing.B) {
	provider := NewPublicAIProvider("test-key", "", "")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.GetCapabilities()
	}
}
