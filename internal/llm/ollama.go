package llm

import (
	"context"
	"fmt"

	"github.com/superagent/superagent/internal/llm/providers/ollama"
	"github.com/superagent/superagent/internal/models"
)

// OllamaProvider wraps the complete Ollama provider implementation
type OllamaProvider struct {
	provider *ollama.OllamaProvider
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	return &OllamaProvider{
		provider: ollama.NewOllamaProvider(baseURL, model),
	}
}

func (o *OllamaProvider) Complete(req *models.LLMRequest) (*models.LLMResponse, error) {
	if o.provider == nil {
		return nil, fmt.Errorf("Ollama provider not initialized")
	}
	return o.provider.Complete(context.Background(), req)
}

func (o *OllamaProvider) HealthCheck() error {
	if o.provider == nil {
		return fmt.Errorf("Ollama provider not initialized")
	}
	return o.provider.HealthCheck()
}

func (o *OllamaProvider) GetCapabilities() *ProviderCapabilities {
	if o.provider == nil {
		return &ProviderCapabilities{
			SupportedModels:         []string{},
			SupportedFeatures:       []string{},
			SupportedRequestTypes:   []string{},
			SupportsStreaming:       false,
			SupportsFunctionCalling: false,
			SupportsVision:          false,
			Metadata:                map[string]string{},
		}
	}
	caps := o.provider.GetCapabilities()
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

func (o *OllamaProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	if o.provider == nil {
		return false, []string{"Ollama provider not initialized"}
	}
	return o.provider.ValidateConfig(config)
}
