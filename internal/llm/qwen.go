package llm

import (
	"context"
	"fmt"

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
	if q.provider == nil {
		return nil, fmt.Errorf("qwen provider not initialized")
	}
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	return q.provider.Complete(context.Background(), req)
}

func (q *QwenProvider) HealthCheck() error {
	if q.provider == nil {
		return fmt.Errorf("qwen provider not initialized")
	}
	return q.provider.HealthCheck()
}

func (q *QwenProvider) GetCapabilities() *ProviderCapabilities {
	return &ProviderCapabilities{
		SupportedModels:         []string{"qwen-turbo", "qwen-plus", "qwen-max"},
		SupportedFeatures:       []string{"streaming", "function_calling", "vision"},
		SupportedRequestTypes:   []string{"text_completion", "chat", "image_analysis"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: true,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsSearch:          false,
		SupportsReasoning:       true,
		SupportsCodeCompletion:  true,
		SupportsCodeAnalysis:    true,
		SupportsRefactoring:     true,
		Limits: ModelLimits{
			MaxTokens:             8192,
			MaxInputLength:        8192,
			MaxOutputLength:       4096,
			MaxConcurrentRequests: 5,
		},
		Metadata: map[string]string{
			"provider": "qwen",
			"version":  "1.0",
		},
	}
}

func (q *QwenProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}
