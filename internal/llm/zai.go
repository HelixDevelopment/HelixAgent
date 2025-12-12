package llm

import (
	"context"
	"fmt"

	"github.com/superagent/superagent/internal/llm/providers/zai"
	"github.com/superagent/superagent/internal/models"
)

// ZaiProvider wraps the complete Z.AI provider implementation
type ZaiProvider struct {
	provider *zai.ZAIProvider
}

func NewZaiProvider(apiKey, baseURL, model string) *ZaiProvider {
	return &ZaiProvider{
		provider: zai.NewZAIProvider(apiKey, baseURL, model),
	}
}

func (z *ZaiProvider) Complete(req *models.LLMRequest) (*models.LLMResponse, error) {
	if z.provider == nil {
		return nil, fmt.Errorf("zai provider not initialized")
	}
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	return z.provider.Complete(context.Background(), req)
}

func (z *ZaiProvider) HealthCheck() error {
	if z.provider == nil {
		return fmt.Errorf("zai provider not initialized")
	}
	return z.provider.HealthCheck()
}

func (z *ZaiProvider) GetCapabilities() *ProviderCapabilities {
	return &ProviderCapabilities{
		SupportedModels:         []string{"zephyr", "mistral", "llama"},
		SupportedFeatures:       []string{"streaming", "function_calling"},
		SupportedRequestTypes:   []string{"text_completion", "chat"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: true,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsSearch:          true,
		SupportsReasoning:       true,
		SupportsCodeCompletion:  true,
		SupportsCodeAnalysis:    true,
		SupportsRefactoring:     true,
		Limits: ModelLimits{
			MaxTokens:             4096,
			MaxInputLength:        4096,
			MaxOutputLength:       2048,
			MaxConcurrentRequests: 10,
		},
		Metadata: map[string]string{
			"provider": "zai",
			"version":  "1.0",
		},
	}
}

func (z *ZaiProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}
