package llm

import (
	"context"

	"github.com/superagent/superagent/internal/llm/providers/qwen"
	"github.com/superagent/superagent/internal/models"
)

// QwenProvider wraps the complete Qwen provider implementation
type QwenProvider struct {
	provider *qwen.QwenProvider
}

func NewQwenProvider(apiKey, baseURL, model string) *QwenProvider {
	return &QwenProvider{
		provider: qwen.NewQwenProvider(apiKey, baseURL, model),
	}
}

func (q *QwenProvider) Complete(req *models.LLMRequest) (*models.LLMResponse, error) {
	return q.provider.Complete(context.Background(), req)
}

func (q *QwenProvider) HealthCheck() error {
	return q.provider.HealthCheck()
}

func (q *QwenProvider) GetCapabilities() *ProviderCapabilities {
	caps := q.provider.GetCapabilities()
	return &ProviderCapabilities{
		SupportedModels:         caps.SupportedModels,
		SupportedFeatures:       caps.SupportedFeatures,
		SupportedRequestTypes:   caps.SupportedRequestTypes,
		SupportsStreaming:       caps.SupportsStreaming,
		SupportsFunctionCalling: caps.SupportsFunctionCalling,
		SupportsVision:          caps.SupportsVision,
		Metadata:                caps.Metadata,
	}
}

func (q *QwenProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return q.provider.ValidateConfig(config)
}
