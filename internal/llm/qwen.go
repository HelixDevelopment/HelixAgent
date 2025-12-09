package llm

import (
	"time"

	"github.com/superagent/superagent/internal/models"
)

// QwenProvider stub
type QwenProvider struct{}

func (q *QwenProvider) Complete(req *models.LLMRequest) (*models.LLMResponse, error) {
	resp := &models.LLMResponse{
		ID:           "rsp-qwen-1",
		RequestID:    req.ID,
		ProviderName: "Qwen",
		Content:      "stub completion from Qwen",
		Confidence:   0.75,
		TokensUsed:   20,
		ResponseTime: 25,
		FinishReason: "stop",
		Metadata:     map[string]interface{}{},
		CreatedAt:    time.Now(),
	}
	return resp, nil
}

func (q *QwenProvider) HealthCheck() error { return nil }

func (q *QwenProvider) GetCapabilities() *ProviderCapabilities {
	return &ProviderCapabilities{
		SupportedModels:         []string{"qwen"},
		SupportedFeatures:       []string{"streaming"},
		SupportedRequestTypes:   []string{"code_generation"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: false,
		SupportsVision:          false,
		Metadata:                map[string]string{},
	}
}

func (q *QwenProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}
