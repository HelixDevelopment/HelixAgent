package qwen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/superagent/superagent/internal/models"
)

// QwenProvider implements the LLMProvider interface for Alibaba Cloud Qwen
type QwenProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// QwenRequest represents a request to the Qwen API
type QwenRequest struct {
	Model       string        `json:"model"`
	Messages    []QwenMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
}

// QwenMessage represents a message in the Qwen API format
type QwenMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// QwenResponse represents a response from the Qwen API
type QwenResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []QwenChoice `json:"choices"`
	Usage   QwenUsage    `json:"usage"`
}

// QwenChoice represents a choice in the Qwen response
type QwenChoice struct {
	Index        int         `json:"index"`
	Message      QwenMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// QwenUsage represents token usage in the Qwen response
type QwenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// QwenError represents an error from the Qwen API
type QwenError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// NewQwenProvider creates a new Qwen provider instance
func NewQwenProvider(apiKey, baseURL, model string) *QwenProvider {
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/api/v1"
	}
	if model == "" {
		model = "qwen-turbo"
	}

	return &QwenProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Complete implements the LLMProvider interface
func (q *QwenProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	// Convert internal request to Qwen format
	qwenReq := q.convertToQwenRequest(req)

	// Make API call
	resp, err := q.makeRequest(ctx, qwenReq)
	if err != nil {
		return nil, fmt.Errorf("failed to complete request: %w", err)
	}

	// Convert response back to internal format
	return q.convertFromQwenResponse(resp, req.ID)
}

// CompleteStream implements streaming completion (placeholder for now)
func (q *QwenProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	// Qwen supports streaming, but implementing it would require more complex handling
	// For now, return an error indicating streaming is not supported
	return nil, fmt.Errorf("streaming not yet implemented for Qwen provider")
}

// HealthCheck implements health checking for the Qwen provider
func (q *QwenProvider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Simple health check - try to get models list
	req, err := http.NewRequestWithContext(ctx, "GET", q.baseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+q.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// GetCapabilities returns the capabilities of the Qwen provider
func (q *QwenProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels: []string{
			"qwen-turbo",
			"qwen-plus",
			"qwen-max",
			"qwen-max-longcontext",
		},
		SupportedFeatures: []string{
			"text_completion",
			"chat",
			"function_calling",
		},
		SupportedRequestTypes: []string{
			"text_completion",
			"chat",
		},
		SupportsStreaming:       true,
		SupportsFunctionCalling: true,
		SupportsVision:          false,
		SupportsTools:           true,
		SupportsSearch:          false,
		SupportsReasoning:       true,
		SupportsCodeCompletion:  true,
		SupportsCodeAnalysis:    true,
		SupportsRefactoring:     false,
		Limits: models.ModelLimits{
			MaxTokens:             6000,
			MaxInputLength:        30000,
			MaxOutputLength:       2000,
			MaxConcurrentRequests: 50,
		},
		Metadata: map[string]string{
			"provider":     "Alibaba Cloud",
			"model_family": "Qwen",
			"api_version":  "v1",
		},
	}
}

// ValidateConfig validates the provider configuration
func (q *QwenProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	var errors []string

	if q.apiKey == "" {
		errors = append(errors, "API key is required")
	}

	if q.baseURL == "" {
		errors = append(errors, "base URL is required")
	}

	if q.model == "" {
		errors = append(errors, "model is required")
	}

	return len(errors) == 0, errors
}

// convertToQwenRequest converts internal request format to Qwen API format
func (q *QwenProvider) convertToQwenRequest(req *models.LLMRequest) *QwenRequest {
	messages := make([]QwenMessage, 0, len(req.Messages))

	// Add system message if present in prompt
	if req.Prompt != "" {
		messages = append(messages, QwenMessage{
			Role:    "system",
			Content: req.Prompt,
		})
	}

	// Convert internal messages
	for _, msg := range req.Messages {
		messages = append(messages, QwenMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return &QwenRequest{
		Model:       q.model,
		Messages:    messages,
		Stream:      false,
		Temperature: req.ModelParams.Temperature,
		MaxTokens:   req.ModelParams.MaxTokens,
		TopP:        req.ModelParams.TopP,
		Stop:        req.ModelParams.StopSequences,
	}
}

// convertFromQwenResponse converts Qwen API response to internal format
func (q *QwenProvider) convertFromQwenResponse(resp *QwenResponse, requestID string) (*models.LLMResponse, error) {
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from Qwen API")
	}

	choice := resp.Choices[0]

	return &models.LLMResponse{
		ID:           resp.ID,
		RequestID:    requestID,
		ProviderID:   "qwen",
		ProviderName: "Qwen",
		Content:      choice.Message.Content,
		Confidence:   0.85, // Qwen doesn't provide confidence scores
		TokensUsed:   resp.Usage.TotalTokens,
		ResponseTime: time.Now().UnixMilli() - (resp.Created * 1000),
		FinishReason: choice.FinishReason,
		Metadata: map[string]interface{}{
			"model":             resp.Model,
			"object":            resp.Object,
			"prompt_tokens":     resp.Usage.PromptTokens,
			"completion_tokens": resp.Usage.CompletionTokens,
		},
		CreatedAt: time.Now(),
	}, nil
}

// makeRequest sends a request to the Qwen API
func (q *QwenProvider) makeRequest(ctx context.Context, req *QwenRequest) (*QwenResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", q.baseURL+"/services/aigc/text-generation/generation", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+q.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var qwenErr QwenError
		if err := json.Unmarshal(body, &qwenErr); err == nil && qwenErr.Error.Message != "" {
			return nil, fmt.Errorf("Qwen API error: %s (%s)", qwenErr.Error.Message, qwenErr.Error.Type)
		}
		return nil, fmt.Errorf("Qwen API returned status %d: %s", resp.StatusCode, string(body))
	}

	var qwenResp QwenResponse
	if err := json.Unmarshal(body, &qwenResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &qwenResp, nil
}
