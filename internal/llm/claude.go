package llm

import (
	"context"

	"github.com/superagent/superagent/internal/llm/providers/claude"
	"github.com/superagent/superagent/internal/models"
)

// ClaudeProvider wraps the complete Claude provider implementation
type ClaudeProvider struct {
	provider *claude.ClaudeProvider
}

func NewClaudeProvider(apiKey, baseURL, model string) *ClaudeProvider {
	return &ClaudeProvider{
		provider: claude.NewClaudeProvider(apiKey, baseURL, model),
	}
}

func (c *ClaudeProvider) Complete(req *models.LLMRequest) (*models.LLMResponse, error) {
	return c.provider.Complete(context.Background(), req)
}

func (c *ClaudeProvider) HealthCheck() error {
	return c.provider.HealthCheck()
}

func (c *ClaudeProvider) GetCapabilities() *ProviderCapabilities {
	return &ProviderCapabilities{
		SupportedModels:         []string{"claude-3-sonnet-20240229", "claude-3-opus-20240229"},
		SupportedFeatures:       []string{"streaming", "function_calling"},
		SupportedRequestTypes:   []string{"text_completion", "chat"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: true,
		SupportsVision:          false,
		Metadata:                map[string]string{},
	}
}

func (c *ClaudeProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}
