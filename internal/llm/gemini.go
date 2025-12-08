package llm

import (
	"github.com/superagent/superagent/internal/models"
	"time"
)

// GeminiProvider stub
type GeminiProvider struct{}

func (g *GeminiProvider) Complete(req *models.LLMRequest) (*models.LLMResponse, error) {
	resp := &models.LLMResponse{
		ID:           "rsp-gemini-1",
		RequestID:    req.ID,
		ProviderName: "Gemini",
		Content:      "stub completion from Gemini",
		Confidence:   0.8,
		TokensUsed:   30,
		ResponseTime: 30,
		FinishReason: "stop",
		Metadata:     map[string]interface{}{},
		CreatedAt:    time.Now(),
	}
	return resp, nil
}

func (g *GeminiProvider) HealthCheck() error { return nil }

func (g *GeminiProvider) GetCapabilities() *ProviderCapabilities {
	return &ProviderCapabilities{
		SupportedModels:         []string{"gemini"},
		SupportedFeatures:       []string{"streaming"},
		SupportedRequestTypes:   []string{"code_generation"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: false,
		SupportsVision:          false,
		Metadata:                map[string]string{},
	}
}

func (g *GeminiProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}
