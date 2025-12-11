package llm

import (
	"context"

	"github.com/superagent/superagent/internal/llm/providers/deepseek"
	"github.com/superagent/superagent/internal/models"
)

// DeepSeekProvider wraps the complete DeepSeek provider implementation
type DeepSeekProvider struct {
	provider *deepseek.DeepSeekProvider
}

func NewDeepSeekProvider(apiKey, baseURL, model string) *DeepSeekProvider {
	return &DeepSeekProvider{
		provider: deepseek.NewDeepSeekProvider(apiKey, baseURL, model),
	}
}

func (d *DeepSeekProvider) Complete(req *models.LLMRequest) (*models.LLMResponse, error) {
	return d.provider.Complete(context.Background(), req)
}

func (d *DeepSeekProvider) HealthCheck() error {
	return d.provider.HealthCheck()
}

func (d *DeepSeekProvider) GetCapabilities() *ProviderCapabilities {
	return &ProviderCapabilities{
		SupportedModels:         []string{"deepseek-coder", "deepseek-chat"},
		SupportedFeatures:       []string{"streaming", "coding", "reasoning"},
		SupportedRequestTypes:   []string{"code_generation", "text_completion"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: false,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsSearch:          false,
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
		Metadata: map[string]string{},
	}
}

func (d *DeepSeekProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}
