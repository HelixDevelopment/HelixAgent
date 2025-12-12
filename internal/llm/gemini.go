package llm

import (
	"context"
	"fmt"

	"github.com/superagent/superagent/internal/llm/providers/gemini"
	"github.com/superagent/superagent/internal/models"
)

// GeminiProvider wraps the complete Gemini provider implementation
type GeminiProvider struct {
	provider *gemini.GeminiProvider
}

func NewGeminiProvider(apiKey, baseURL, model string) *GeminiProvider {
	return &GeminiProvider{
		provider: gemini.NewGeminiProvider(apiKey, baseURL, model),
	}
}

func (g *GeminiProvider) Complete(req *models.LLMRequest) (*models.LLMResponse, error) {
	if g.provider == nil {
		return nil, fmt.Errorf("Gemini provider not initialized")
	}
	return g.provider.Complete(context.Background(), req)
}

func (g *GeminiProvider) HealthCheck() error {
	if g.provider == nil {
		return fmt.Errorf("Gemini provider not initialized")
	}
	return g.provider.HealthCheck()
}

func (g *GeminiProvider) GetCapabilities() *ProviderCapabilities {
	return &ProviderCapabilities{
		SupportedModels:         []string{"gemini-pro", "gemini-pro-vision"},
		SupportedFeatures:       []string{"streaming", "vision"},
		SupportedRequestTypes:   []string{"text_completion", "chat", "multimodal"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: false,
		SupportsVision:          true,
		Metadata:                map[string]string{},
	}
}

func (g *GeminiProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}
