package deepseek

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/superagent/superagent/internal/models"
)

const (
	DeepSeekAPIURL = "https://api.deepseek.com/v1/chat/completions"
	DeepSeekModel  = "deepseek-coder"
)

type DeepSeekProvider struct {
	apiKey      string
	baseURL     string
	model       string
	httpClient  *http.Client
	retryConfig RetryConfig
}

// RetryConfig defines retry behavior for API calls
type RetryConfig struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

type DeepSeekRequest struct {
	Model       string            `json:"model"`
	Messages    []DeepSeekMessage `json:"messages"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	TopP        float64           `json:"top_p,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
	Stop        []string          `json:"stop,omitempty"`
}

type DeepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DeepSeekResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []DeepSeekChoice `json:"choices"`
	Usage   DeepSeekUsage    `json:"usage"`
}

type DeepSeekChoice struct {
	Index        int             `json:"index"`
	Message      DeepSeekMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

type DeepSeekUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type DeepSeekStreamResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []DeepSeekStreamChoice `json:"choices"`
}

type DeepSeekStreamChoice struct {
	Index        int             `json:"index"`
	Delta        DeepSeekMessage `json:"delta"`
	FinishReason *string         `json:"finish_reason"`
}

// DefaultRetryConfig returns sensible defaults for DeepSeek API retry behavior
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

func NewDeepSeekProvider(apiKey, baseURL, model string) *DeepSeekProvider {
	return NewDeepSeekProviderWithRetry(apiKey, baseURL, model, DefaultRetryConfig())
}

func NewDeepSeekProviderWithRetry(apiKey, baseURL, model string, retryConfig RetryConfig) *DeepSeekProvider {
	if baseURL == "" {
		baseURL = DeepSeekAPIURL
	}
	if model == "" {
		model = DeepSeekModel
	}

	return &DeepSeekProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		retryConfig: retryConfig,
	}
}

func (p *DeepSeekProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	startTime := time.Now()

	// Convert internal request to DeepSeek format
	dsReq := p.convertRequest(req)

	// Make API call
	resp, err := p.makeAPICall(ctx, dsReq)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek API call failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DeepSeek API error: %d - %s", resp.StatusCode, string(body))
	}

	var dsResp DeepSeekResponse
	if err := json.Unmarshal(body, &dsResp); err != nil {
		return nil, fmt.Errorf("failed to parse DeepSeek response: %w", err)
	}

	// Convert back to internal format
	return p.convertResponse(req, &dsResp, startTime), nil
}

func (p *DeepSeekProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	startTime := time.Now()

	// Convert internal request to DeepSeek format
	dsReq := p.convertRequest(req)
	dsReq.Stream = true

	// Make streaming API call
	resp, err := p.makeAPICall(ctx, dsReq)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek streaming API call failed: %w", err)
	}

	// Create response channel
	ch := make(chan *models.LLMResponse)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		reader := bufio.NewReader(resp.Body)
		var fullContent string

		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				// Send error response and exit
				errorResp := &models.LLMResponse{
					ID:             "stream-error-" + req.ID,
					RequestID:      req.ID,
					ProviderID:     "deepseek",
					ProviderName:   "DeepSeek",
					Content:        "",
					Confidence:     0.0,
					TokensUsed:     0,
					ResponseTime:   time.Since(startTime).Milliseconds(),
					FinishReason:   "error",
					Selected:       false,
					SelectionScore: 0.0,
					CreatedAt:      time.Now(),
				}
				ch <- errorResp
				return
			}

			// Skip empty lines and "data: " prefix
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data: ")) {
				continue
			}
			line = bytes.TrimPrefix(line, []byte("data: "))

			// Skip "[DONE]" marker
			if bytes.Equal(line, []byte("[DONE]")) {
				break
			}

			// Parse JSON
			var streamResp DeepSeekStreamResponse
			if err := json.Unmarshal(line, &streamResp); err != nil {
				continue // Skip malformed JSON
			}

			// Extract content
			if len(streamResp.Choices) > 0 {
				delta := streamResp.Choices[0].Delta.Content
				if delta != "" {
					fullContent += delta

					// Send chunk response
					chunkResp := &models.LLMResponse{
						ID:             streamResp.ID,
						RequestID:      req.ID,
						ProviderID:     "deepseek",
						ProviderName:   "DeepSeek",
						Content:        delta,
						Confidence:     0.8, // Default confidence for streaming
						TokensUsed:     1,   // Estimated
						ResponseTime:   time.Since(startTime).Milliseconds(),
						FinishReason:   "",
						Selected:       false,
						SelectionScore: 0.0,
						CreatedAt:      time.Now(),
					}
					ch <- chunkResp
				}

				// Check if stream is finished
				if streamResp.Choices[0].FinishReason != nil {
					break
				}
			}
		}

		// Send final response
		finalResp := &models.LLMResponse{
			ID:             "stream-final-" + req.ID,
			RequestID:      req.ID,
			ProviderID:     "deepseek",
			ProviderName:   "DeepSeek",
			Content:        "",
			Confidence:     0.8,
			TokensUsed:     len(fullContent) / 4, // Rough estimate
			ResponseTime:   time.Since(startTime).Milliseconds(),
			FinishReason:   "stop",
			Selected:       false,
			SelectionScore: 0.0,
			CreatedAt:      time.Now(),
		}
		ch <- finalResp
	}()

	return ch, nil
}

func (p *DeepSeekProvider) convertRequest(req *models.LLMRequest) DeepSeekRequest {
	// Convert messages
	messages := make([]DeepSeekMessage, 0, len(req.Messages)+1)

	// Add system prompt if present
	if req.Prompt != "" {
		messages = append(messages, DeepSeekMessage{
			Role:    "system",
			Content: req.Prompt,
		})
	}

	// Add conversation messages
	for _, msg := range req.Messages {
		messages = append(messages, DeepSeekMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Cap max_tokens to DeepSeek's limit (8192)
	maxTokens := req.ModelParams.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096 // Default
	} else if maxTokens > 8192 {
		maxTokens = 8192 // DeepSeek's max limit
	}

	return DeepSeekRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: req.ModelParams.Temperature,
		MaxTokens:   maxTokens,
		TopP:        req.ModelParams.TopP,
		Stream:      false,
		Stop:        req.ModelParams.StopSequences,
	}
}

func (p *DeepSeekProvider) convertResponse(req *models.LLMRequest, dsResp *DeepSeekResponse, startTime time.Time) *models.LLMResponse {
	var content string
	var finishReason string

	if len(dsResp.Choices) > 0 {
		content = dsResp.Choices[0].Message.Content
		finishReason = dsResp.Choices[0].FinishReason
	}

	// Calculate confidence based on finish reason and response quality
	confidence := p.calculateConfidence(content, finishReason)

	return &models.LLMResponse{
		ID:           dsResp.ID,
		RequestID:    req.ID,
		ProviderID:   "deepseek",
		ProviderName: "DeepSeek",
		Content:      content,
		Confidence:   confidence,
		TokensUsed:   dsResp.Usage.TotalTokens,
		ResponseTime: time.Since(startTime).Milliseconds(),
		FinishReason: finishReason,
		Metadata: map[string]any{
			"model":             dsResp.Model,
			"prompt_tokens":     dsResp.Usage.PromptTokens,
			"completion_tokens": dsResp.Usage.CompletionTokens,
		},
		Selected:       false,
		SelectionScore: 0.0,
		CreatedAt:      time.Now(),
	}
}

func (p *DeepSeekProvider) calculateConfidence(content, finishReason string) float64 {
	confidence := 0.8 // Base confidence

	// Adjust based on finish reason
	switch finishReason {
	case "stop":
		confidence += 0.1
	case "length":
		confidence -= 0.1
	case "content_filter":
		confidence -= 0.3
	}

	// Adjust based on content length
	if len(content) > 100 {
		confidence += 0.05
	}
	if len(content) > 500 {
		confidence += 0.05
	}

	// Ensure confidence is within bounds
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

func (p *DeepSeekProvider) makeAPICall(ctx context.Context, req DeepSeekRequest) (*http.Response, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	delay := p.retryConfig.InitialDelay

	for attempt := 0; attempt <= p.retryConfig.MaxRetries; attempt++ {
		// Check context before making request
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		// Create HTTP request (fresh for each attempt)
		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewBuffer(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
		httpReq.Header.Set("User-Agent", "SuperAgent/1.0")

		// Make request
		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			// Retry on network errors
			if attempt < p.retryConfig.MaxRetries {
				p.waitWithJitter(ctx, delay)
				delay = p.nextDelay(delay)
				continue
			}
			return nil, lastErr
		}

		// Check for retryable status codes
		if isRetryableStatus(resp.StatusCode) && attempt < p.retryConfig.MaxRetries {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: retryable error", resp.StatusCode)
			p.waitWithJitter(ctx, delay)
			delay = p.nextDelay(delay)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("all %d retry attempts failed: %w", p.retryConfig.MaxRetries+1, lastErr)
}

// isRetryableStatus returns true for HTTP status codes that warrant a retry
func isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429 - Rate limited
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// waitWithJitter waits for the specified duration plus random jitter
func (p *DeepSeekProvider) waitWithJitter(ctx context.Context, delay time.Duration) {
	// Add 10% jitter
	jitter := time.Duration(rand.Float64() * 0.1 * float64(delay))
	select {
	case <-ctx.Done():
	case <-time.After(delay + jitter):
	}
}

// nextDelay calculates the next delay using exponential backoff
func (p *DeepSeekProvider) nextDelay(currentDelay time.Duration) time.Duration {
	nextDelay := time.Duration(float64(currentDelay) * p.retryConfig.Multiplier)
	if nextDelay > p.retryConfig.MaxDelay {
		nextDelay = p.retryConfig.MaxDelay
	}
	return nextDelay
}

// GetCapabilities returns the capabilities of the DeepSeek provider
func (p *DeepSeekProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels: []string{
			"deepseek-coder",
			"deepseek-chat",
		},
		SupportedFeatures: []string{
			"text_completion",
			"chat",
			"function_calling",
			"streaming",
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
		SupportsRefactoring:     true,
		Limits: models.ModelLimits{
			MaxTokens:             4096,
			MaxInputLength:        4096,
			MaxOutputLength:       4096,
			MaxConcurrentRequests: 10,
		},
		Metadata: map[string]string{
			"provider":     "DeepSeek",
			"model_family": "DeepSeek",
			"api_version":  "v1",
		},
	}
}

// ValidateConfig validates the provider configuration
func (p *DeepSeekProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	var errors []string

	if p.apiKey == "" {
		errors = append(errors, "API key is required")
	}

	if p.baseURL == "" {
		errors = append(errors, "base URL is required")
	}

	if p.model == "" {
		errors = append(errors, "model is required")
	}

	return len(errors) == 0, errors
}

// HealthCheck implements health checking for the DeepSeek provider
func (p *DeepSeekProvider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Simple health check - try to get models list
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.deepseek.com/models", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}
