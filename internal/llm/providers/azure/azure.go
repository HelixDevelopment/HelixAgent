// Package azure provides Azure OpenAI integration
package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
)

// Provider implements LLMProvider for Azure OpenAI
type Provider struct {
	endpoint   string
	deployment string
	apiKey     string
	apiVersion string
	client     *http.Client
}

// NewProvider creates a new Azure OpenAI provider
func NewProvider(endpoint, deployment, apiKey string) *Provider {
	return &Provider{
		endpoint:   endpoint,
		deployment: deployment,
		apiKey:     apiKey,
		apiVersion: "2024-02-01",
		client:     &http.Client{Timeout: 120 * time.Second},
	}
}

// Complete implements LLMProvider
func (p *Provider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		p.endpoint, p.deployment, p.apiVersion)

	messages := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = map[string]string{
			"role":    m.Role,
			"content": m.Content,
		}
	}

	body := map[string]interface{}{
		"messages":    messages,
		"temperature": req.ModelParams.Temperature,
		"max_tokens":  req.ModelParams.MaxTokens,
		"top_p":       req.ModelParams.TopP,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("api-key", p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &models.LLMResponse{
		Content:      result.Choices[0].Message.Content,
		FinishReason: result.Choices[0].FinishReason,
		TokensUsed:   result.Usage.TotalTokens,
		ProviderID:   "azure-openai",
		ProviderName: "Azure OpenAI",
		// Record the REAL per-direction split, not just the total.
		// models.LLMResponse.TokenSplit reads these keys to build the
		// OpenAI-compatible `usage` envelope; without them the wire
		// response reports 0 for both directions even though Azure
		// measured and returned them here.
		Metadata: map[string]any{
			"prompt_tokens":     result.Usage.PromptTokens,
			"completion_tokens": result.Usage.CompletionTokens,
			"total_tokens":      result.Usage.TotalTokens,
		},
	}, nil
}

// CompleteStream implements LLMProvider
func (p *Provider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	resp, err := p.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	ch := make(chan *models.LLMResponse, 1)
	ch <- resp
	close(ch)
	return ch, nil
}

// HealthCheck implements LLMProvider
func (p *Provider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/openai/deployments/%s?api-version=%s",
		p.endpoint, p.deployment, p.apiVersion)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("api-key", p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}

	return fmt.Errorf("health check returned status %d", resp.StatusCode)
}

// GetCapabilities implements LLMProvider
func (p *Provider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels:         []string{p.deployment},
		SupportedFeatures:       []string{"chat", "streaming", "function_calling", "vision"},
		SupportedRequestTypes:   []string{"chat", "completion"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: true,
		SupportsVision:          true,
		SupportsTools:           true,
		SupportsSearch:          false,
		SupportsReasoning:       true,
		SupportsCodeCompletion:  true,
		SupportsCodeAnalysis:    true,
		SupportsRefactoring:     true,
		Limits: models.ModelLimits{
			MaxTokens:             128000,
			MaxInputLength:        128000,
			MaxOutputLength:       4096,
			MaxConcurrentRequests: 100,
		},
		Metadata: map[string]string{
			"provider":   "azure-openai",
			"deployment": p.deployment,
		},
	}
}

// ValidateConfig implements LLMProvider
func (p *Provider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	var errors []string

	if endpoint, ok := config["endpoint"].(string); !ok || endpoint == "" {
		errors = append(errors, "endpoint is required")
	}

	if deployment, ok := config["deployment"].(string); !ok || deployment == "" {
		errors = append(errors, "deployment is required")
	}

	if apiKey, ok := config["api_key"].(string); !ok || apiKey == "" {
		errors = append(errors, "api_key is required")
	}

	return len(errors) == 0, errors
}

// Ensure interface is implemented
var _ llm.LLMProvider = (*Provider)(nil)
