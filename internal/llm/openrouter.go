package llm

import (
	"context"

	"github.com/superagent/superagent/internal/llm/providers/openrouter"
	"github.com/superagent/superagent/internal/models"
)

// OpenRouterProvider wraps the OpenRouter provider implementation
type OpenRouterProvider struct {
	provider *openrouter.SimpleOpenRouterProvider
}

func NewOpenRouterProvider(apiKey string) *OpenRouterProvider {
	return &OpenRouterProvider{
		provider: openrouter.NewSimpleOpenRouterProvider(apiKey),
	}
}

func (o *OpenRouterProvider) Complete(req *models.LLMRequest) (*models.LLMResponse, error) {
	return o.provider.Complete(context.Background(), req)
}

func (o *OpenRouterProvider) HealthCheck() error {
	return o.provider.HealthCheck()
}

func (o *OpenRouterProvider) GetCapabilities() *ProviderCapabilities {
	return &ProviderCapabilities{
		SupportedModels: []string{
			"x-ai/grok-4",
			"x-ai/grok-4-mini",
			"google/gemini-2.0-flash-exp",
			"google/gemini-2.0-flash-thinking-exp",
			"anthropic/claude-3.5-sonnet",
			"anthropic/claude-3.5-haiku",
			"openai/gpt-4o",
			"openai/gpt-4o-mini",
			"meta-llama/llama-3.1-405b-instruct",
			"meta-llama/llama-3.1-70b-instruct",
			"meta-llama/llama-3.1-8b-instruct",
		},
		SupportedFeatures: []string{
			"text-generation",
			"code-generation",
			"reasoning",
			"function-calling",
			"multi-turn",
		},
		SupportedRequestTypes: []string{
			"chat",
			"completion",
		},
		SupportsStreaming:       true,
		SupportsFunctionCalling: true,
		SupportsVision:          false,
		Metadata: map[string]string{
			"provider": "openrouter",
			"models":   "100+ models available",
		},
	}
}

func (o *OpenRouterProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	var warnings []string

	// Check required fields
	if apiKey, ok := config["api_key"]; !ok || apiKey.(string) == "" {
		return false, []string{"API key is required"}
	}

	// Optional warnings
	if _, ok := config["base_url"]; ok {
		warnings = append(warnings, "Custom base URL may not be supported by OpenRouter")
	}

	return true, warnings
}
