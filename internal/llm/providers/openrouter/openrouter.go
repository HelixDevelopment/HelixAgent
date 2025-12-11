package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/superagent/superagent/internal/models"
)

const (
	BaseURL = "https://openrouter.ai/api/v1"
)

// SimpleOpenRouterProvider implements LLM provider interface for OpenRouter
type SimpleOpenRouterProvider struct {
	apiKey string
	client *http.Client
}

// NewSimpleOpenRouterProvider creates a new OpenRouter provider
func NewSimpleOpenRouterProvider(apiKey string) *SimpleOpenRouterProvider {
	return &SimpleOpenRouterProvider{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Complete implements LLM provider interface
func (p *SimpleOpenRouterProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	// Use provided context or create timeout context
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
	}

	// Convert to OpenRouter format
	type OpenRouterRequest struct {
		Model       string           `json:"model"`
		Messages    []models.Message `json:"messages"`
		Prompt      string           `json:"prompt,omitempty"`
		MaxTokens   int              `json:"max_tokens,omitempty"`
		Temperature float64          `json:"temperature,omitempty"`
	}

	orReq := OpenRouterRequest{
		Model:       req.ModelParams.Model,
		Messages:    req.Messages,
		Prompt:      req.Prompt,
		MaxTokens:   req.ModelParams.MaxTokens,
		Temperature: req.ModelParams.Temperature,
	}

	// Make request
	jsonData, err := json.Marshal(orReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenRouter request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenRouter request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("HTTP-Referer", "superagent")

	// Make request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var orResp struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Usage   *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage,omitempty"`
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    int    `json:"code,omitempty"`
		} `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&orResp); err != nil {
		return nil, fmt.Errorf("failed to decode OpenRouter response: %w", err)
	}

	if orResp.Error != nil {
		return nil, fmt.Errorf("OpenRouter API error: %s", orResp.Error.Message)
	}

	// Convert to internal response format
	if len(orResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in OpenRouter response")
	}

	choice := orResp.Choices[0]
	response := &models.LLMResponse{
		ID:           orResp.ID,
		RequestID:    req.ID,
		ProviderID:   "openrouter",
		ProviderName: "OpenRouter",
		Content:      choice.Message.Content,
		Confidence:   0.85, // OpenRouter doesn't provide confidence
		TokensUsed:   0,
		ResponseTime: time.Now().UnixMilli(),
		FinishReason: "stop",
		Metadata: map[string]any{
			"model":    orResp.Model,
			"provider": "openrouter",
		},
		Selected:       false,
		SelectionScore: 0.0,
		CreatedAt:      time.Now(),
	}

	if orResp.Usage != nil {
		response.TokensUsed = orResp.Usage.TotalTokens
	}

	return response, nil
}

// CompleteStream implements streaming completion
func (p *SimpleOpenRouterProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ch := make(chan *models.LLMResponse, 1)

	go func() {
		defer close(ch)

		// OpenRouter streaming not implemented in this simple version
		ch <- &models.LLMResponse{
			ID:           "stream-not-supported",
			RequestID:    req.ID,
			ProviderID:   "openrouter",
			ProviderName: "OpenRouter",
			Content:      "Streaming not supported by OpenRouter provider",
			FinishReason: "error",
			CreatedAt:    time.Now(),
		}
	}()

	return ch, nil
}

// HealthCheck implements provider health monitoring
func (p *SimpleOpenRouterProvider) HealthCheck() error {
	// OpenRouter doesn't have a specific health check endpoint
	if p.apiKey == "" {
		return fmt.Errorf("OpenRouter API key is required for health check")
	}
	return nil
}

// GetCapabilities returns provider capabilities
func (p *SimpleOpenRouterProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels: []string{
			"openrouter/anthropic/claude-3.5-sonnet",
			"openrouter/openai/gpt-4o",
			"openrouter/google/gemini-pro",
			"openrouter/meta-llama/llama-3.1-405b",
			"openrouter/mistralai/mistral-large",
			"openrouter/meta-llama/llama-3.1-70b",
			"openrouter/perplexity-70b",
			"openwizard/cohere-2",
			"openwizard/palm-2-chat-bison",
			"openwizard/gemma-2-7b",
			"openwizard/gemma-1.5-pro",
			"openwizard/dbrx-instruct",
			"openwizard/dbrx-small",
			"openwizard/llava-2",
			"openwizard/code-llama-2",
			"openwizard/qwen-1.8b-chat",
			"openwizard/qwen-1.8b-code",
			"openwizard/zephyr-7b-alpha",
			"openrouter/deepseek-v2-lite",
			"openrouter/deepseek-coder-v2-lite",
			"openwizard/nous-hermes-2",
			"openwizard/nous-hermes-2-predetermined",
			"openwizard/seamless-emb",
			"openwizard/command-r",
			"openwizard/vicuna-1.3",
			"openwizard/unitree-2",
			"openwizard/vicuna-2.0",
			"openwizard/yi-34b",
			"openwizard/yi-6b",
			"openwizard/yi-34b-200k",
			"openrouter/segway-3.5b",
			"openrouter/segway-3.5b-16k",
			"openwizard/gpt-4o",
			"openwizard/gpt-4-turbo",
			"openwizard/gpt-4-32k",
			"openrouter/gpt-4-vision-preview",
			"openwizard/o1-preview",
			"openwizard/grande-3",
			"openwizard/grande-3-instruct",
			"openwizard/yi-6b",
			"openwizard/mistral-7b",
			"openwizard/mixtral-8x7b",
			"openwizard/mixtral-8x22b",
			"openwizard/pixtral-12b",
			"openwizard/starcoder2-15b",
			"openwizard/starcoder2-13b",
		},
		SupportedFeatures: []string{
			"text_completion",
			"chat",
			"multi_model_routing",
			"cost_optimization",
		},
		SupportedRequestTypes: []string{
			"text_completion",
			"chat",
		},
		SupportsStreaming:       true,
		SupportsFunctionCalling: false,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsSearch:          true,
		SupportsReasoning:       true,
		SupportsCodeCompletion:  true,
		SupportsCodeAnalysis:    true,
		SupportsRefactoring:     true,
		Limits: models.ModelLimits{
			MaxTokens:             200000,
			MaxInputLength:        200000,
			MaxOutputLength:       8192,
			MaxConcurrentRequests: 10,
		},
		Metadata: map[string]string{
			"provider":      "OpenRouter",
			"api_version":   "v1",
			"routing":       "basic",
			"multi_tenancy": "true",
		},
	}
}

// ValidateConfig validates provider configuration
func (p *SimpleOpenRouterProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	if p.apiKey == "" {
		return false, []string{"api_key is required"}
	}
	return true, nil
}
