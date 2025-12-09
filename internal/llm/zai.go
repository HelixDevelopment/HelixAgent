package llm

import (
	"time"

	"github.com/superagent/superagent/internal/models"
)

// ZaiProvider stub
type ZaiProvider struct{}

func (z *ZaiProvider) Complete(req *models.LLMRequest) (*models.LLMResponse, error) {
	resp := &models.LLMResponse{
		ID:           "rsp-zai-1",
		RequestID:    req.ID,
		ProviderName: "ZAI",
		Content:      "stub completion from ZAI",
		Confidence:   0.6,
		TokensUsed:   15,
		ResponseTime: 15,
		FinishReason: "stop",
		Metadata:     map[string]interface{}{},
		CreatedAt:    time.Now(),
	}
	return resp, nil
}

func (z *ZaiProvider) HealthCheck() error { return nil }

func (z *ZaiProvider) GetCapabilities() *ProviderCapabilities {
	return &ProviderCapabilities{
		SupportedModels:         []string{"zai"},
		SupportedFeatures:       []string{"streaming"},
		SupportedRequestTypes:   []string{"code_generation"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: false,
		SupportsVision:          false,
		Metadata:                map[string]string{},
	}
}

func (z *ZaiProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}
