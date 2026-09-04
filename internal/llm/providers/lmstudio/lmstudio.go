// Package lmstudio provides LM Studio integration
package lmstudio

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

// Provider implements LLMProvider for LM Studio
type Provider struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewProvider creates a new LM Studio provider
func NewProvider(baseURL, model string) *Provider {
	if baseURL == "" {
		baseURL = "http://localhost:1234"
	}

	return &Provider{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// Complete implements LLMProvider
func (p *Provider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	messages := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = map[string]string{
			"role":    m.Role,
			"content": m.Content,
		}
	}

	body := map[string]interface{}{
		"model":       p.model,
		"messages":    messages,
		"temperature": req.ModelParams.Temperature,
		"max_tokens":  req.ModelParams.MaxTokens,
		"top_p":       req.ModelParams.TopP,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		p.baseURL+"/v1/chat/completions",
		bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

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
		ProviderID:   "lmstudio",
		ProviderName: "LM Studio",
		// Record the REAL per-direction split, not just the total.
		// models.LLMResponse.TokenSplit reads these keys to build the
		// OpenAI-compatible `usage` envelope; without them the wire
		// response reports 0 for both directions even though LM Studio
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
	// LM Studio supports streaming via SSE
	// For now, fall back to non-streaming
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

	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/v1/models", nil)
	if err != nil {
		return err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %d", resp.StatusCode)
	}
	return nil
}

// GetCapabilities implements LLMProvider
func (p *Provider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels:         []string{p.model},
		SupportedFeatures:       []string{"chat", "completions"},
		SupportedRequestTypes:   []string{"chat"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: false,
		SupportsVision:          false,
		SupportsTools:           false,
		SupportsSearch:          false,
		SupportsReasoning:       true,
		SupportsCodeCompletion:  true,
		SupportsCodeAnalysis:    true,
		SupportsRefactoring:     false,
		Limits: models.ModelLimits{
			MaxTokens:             4096,
			MaxInputLength:        8192,
			MaxOutputLength:       4096,
			MaxConcurrentRequests: 10,
		},
		Metadata: map[string]string{
			"provider": "lmstudio",
			"type":     "local",
		},
	}
}

// ValidateConfig implements LLMProvider
func (p *Provider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	var errors []string
	if p.baseURL == "" {
		errors = append(errors, "base URL is required")
	}
	if p.model == "" {
		errors = append(errors, "model is required")
	}
	return len(errors) == 0, errors
}

// Ensure interface is implemented
var _ llm.LLMProvider = (*Provider)(nil)
