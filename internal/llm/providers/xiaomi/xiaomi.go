package xiaomi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/transport"
)

const (
	XiaomiAPIURL       = "https://api.xiaomimimo.com/v1/chat/completions"
	XiaomiModelsURL    = "https://api.xiaomimimo.com/v1/models"
	XiaomiDefaultModel = "mimo-v2.5-pro"
)

type XiaomiProvider struct {
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

type XiaomiRequest struct {
	Model       string          `json:"model"`
	Messages    []XiaomiMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
	Tools       []XiaomiTool    `json:"tools,omitempty"`
	ToolChoice  interface{}     `json:"tool_choice,omitempty"`
}

type XiaomiMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []XiaomiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type XiaomiTool struct {
	Type     string         `json:"type"`
	Function XiaomiToolFunc `json:"function"`
}

type XiaomiToolFunc struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type XiaomiToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function XiaomiToolCallFunction `json:"function"`
}

type XiaomiToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type XiaomiResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []XiaomiChoice `json:"choices"`
	Usage   XiaomiUsage    `json:"usage"`
}

type XiaomiChoice struct {
	Index        int           `json:"index"`
	Message      XiaomiMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type XiaomiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type XiaomiStreamResponse struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []XiaomiStreamChoice `json:"choices"`
}

type XiaomiStreamChoice struct {
	Index        int           `json:"index"`
	Delta        XiaomiMessage `json:"delta"`
	FinishReason *string       `json:"finish_reason"`
}

// DefaultRetryConfig returns sensible defaults for Xiaomi API retry behavior
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

func NewXiaomiProvider(apiKey, baseURL, model string) *XiaomiProvider {
	return NewXiaomiProviderWithRetry(apiKey, baseURL, model, DefaultRetryConfig())
}

func NewXiaomiProviderWithRetry(apiKey, baseURL, model string, retryConfig RetryConfig) *XiaomiProvider {
	if baseURL == "" {
		baseURL = XiaomiAPIURL
	}
	if model == "" {
		model = XiaomiDefaultModel
	}

	return &XiaomiProvider{
		apiKey:      apiKey,
		baseURL:     baseURL,
		model:       model,
		httpClient:  transport.NewHTTP3Client(nil).HTTPClient(),
		retryConfig: retryConfig,
	}
}

func (p *XiaomiProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	startTime := time.Now()

	xReq := p.convertRequest(req)

	resp, err := p.makeAPICall(ctx, xReq)
	if err != nil {
		return nil, fmt.Errorf("Xiaomi MiMo API call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Xiaomi MiMo API error: %d - %s", resp.StatusCode, string(body))
	}

	var xResp XiaomiResponse
	if err := json.Unmarshal(body, &xResp); err != nil {
		return nil, fmt.Errorf("failed to parse Xiaomi MiMo response: %w", err)
	}

	return p.convertResponse(req, &xResp, startTime), nil
}

func (p *XiaomiProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	startTime := time.Now()

	xReq := p.convertRequest(req)
	xReq.Stream = true

	resp, err := p.makeAPICall(ctx, xReq)
	if err != nil {
		return nil, fmt.Errorf("Xiaomi MiMo streaming API call failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("Xiaomi MiMo API error: HTTP %d - failed to read response body: %v", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("Xiaomi MiMo API error: HTTP %d - %s", resp.StatusCode, string(body))
	}

	ch := make(chan *models.LLMResponse)

	go func() {
		defer func() { _ = resp.Body.Close() }()
		defer close(ch)

		reader := bufio.NewReader(resp.Body)
		var fullContent string

		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				errorResp := &models.LLMResponse{
					ID:           "stream-error-" + req.ID,
					RequestID:    req.ID,
					ProviderID:   "xiaomi",
					ProviderName: "Xiaomi MiMo",
					FinishReason: "error",
					CreatedAt:    time.Now(),
				}
				ch <- errorResp
				return
			}

			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data: ")) {
				continue
			}
			line = bytes.TrimPrefix(line, []byte("data: "))

			if bytes.Equal(line, []byte("[DONE]")) {
				break
			}

			var streamResp XiaomiStreamResponse
			if err := json.Unmarshal(line, &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) > 0 {
				delta := streamResp.Choices[0].Delta.Content
				if delta != "" {
					fullContent += delta

					chunkResp := &models.LLMResponse{
						ID:           streamResp.ID,
						RequestID:    req.ID,
						ProviderID:   "xiaomi",
						ProviderName: "Xiaomi MiMo",
						Content:      delta,
						Confidence:   0.8,
						TokensUsed:   1,
						ResponseTime: time.Since(startTime).Milliseconds(),
						CreatedAt:    time.Now(),
					}
					ch <- chunkResp
				}

				if streamResp.Choices[0].FinishReason != nil {
					break
				}
			}
		}

		finalResp := &models.LLMResponse{
			ID:           "stream-final-" + req.ID,
			RequestID:    req.ID,
			ProviderID:   "xiaomi",
			ProviderName: "Xiaomi MiMo",
			Content:      "",
			Confidence:   0.8,
			TokensUsed:   len(fullContent) / 4,
			ResponseTime: time.Since(startTime).Milliseconds(),
			FinishReason: "stop",
			CreatedAt:    time.Now(),
		}
		ch <- finalResp
	}()

	return ch, nil
}

func (p *XiaomiProvider) convertRequest(req *models.LLMRequest) XiaomiRequest {
	messages := make([]XiaomiMessage, 0, len(req.Messages)+1)

	if req.Prompt != "" {
		messages = append(messages, XiaomiMessage{
			Role:    "system",
			Content: req.Prompt,
		})
	}

	for _, msg := range req.Messages {
		xMsg := XiaomiMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		if len(msg.AssistantToolCalls) > 0 {
			xMsg.ToolCalls = make([]XiaomiToolCall, 0, len(msg.AssistantToolCalls))
			for _, tc := range msg.AssistantToolCalls {
				xMsg.ToolCalls = append(xMsg.ToolCalls, XiaomiToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: XiaomiToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
		}
		messages = append(messages, xMsg)
	}

	maxTokens := req.ModelParams.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	} else if maxTokens > 8192 {
		maxTokens = 8192
	}

	xReq := XiaomiRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: req.ModelParams.Temperature,
		MaxTokens:   maxTokens,
		TopP:        req.ModelParams.TopP,
		Stream:      false,
		Stop:        req.ModelParams.StopSequences,
	}

	if len(req.Tools) > 0 {
		xReq.Tools = make([]XiaomiTool, len(req.Tools))
		for i, tool := range req.Tools {
			xReq.Tools[i] = XiaomiTool{
				Type: tool.Type,
				Function: XiaomiToolFunc{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			}
		}
		if req.ToolChoice != "" {
			xReq.ToolChoice = req.ToolChoice
		}
	}

	return xReq
}

func (p *XiaomiProvider) convertResponse(req *models.LLMRequest, xResp *XiaomiResponse, startTime time.Time) *models.LLMResponse {
	var content string
	var finishReason string
	var toolCalls []models.ToolCall

	if len(xResp.Choices) > 0 {
		content = xResp.Choices[0].Message.Content
		finishReason = xResp.Choices[0].FinishReason

		if len(xResp.Choices[0].Message.ToolCalls) > 0 {
			toolCalls = make([]models.ToolCall, len(xResp.Choices[0].Message.ToolCalls))
			for i, tc := range xResp.Choices[0].Message.ToolCalls {
				toolCalls[i] = models.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: models.ToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
			if finishReason == "" || finishReason == "stop" {
				finishReason = "tool_calls"
			}
		}
	}

	confidence := p.calculateConfidence(content, finishReason)

	return &models.LLMResponse{
		ID:           xResp.ID,
		RequestID:    req.ID,
		ProviderID:   "xiaomi",
		ProviderName: "Xiaomi MiMo",
		Content:      content,
		Confidence:   confidence,
		TokensUsed:   xResp.Usage.TotalTokens,
		ResponseTime: time.Since(startTime).Milliseconds(),
		FinishReason: finishReason,
		ToolCalls:    toolCalls,
		Metadata: map[string]any{
			"model":             xResp.Model,
			"prompt_tokens":     xResp.Usage.PromptTokens,
			"completion_tokens": xResp.Usage.CompletionTokens,
		},
		CreatedAt: time.Now(),
	}
}

func (p *XiaomiProvider) calculateConfidence(content, finishReason string) float64 {
	confidence := 0.8

	switch finishReason {
	case "stop":
		confidence += 0.1
	case "length":
		confidence -= 0.1
	case "content_filter":
		confidence -= 0.3
	}

	if len(content) > 100 {
		confidence += 0.05
	}
	if len(content) > 500 {
		confidence += 0.05
	}

	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

func (p *XiaomiProvider) makeAPICall(ctx context.Context, req XiaomiRequest) (*http.Response, error) {
	return p.makeAPICallWithAuthRetry(ctx, req, true)
}

func (p *XiaomiProvider) makeAPICallWithAuthRetry(ctx context.Context, req XiaomiRequest, allowAuthRetry bool) (*http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	delay := p.retryConfig.InitialDelay

	for attempt := 0; attempt <= p.retryConfig.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewBuffer(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
		httpReq.Header.Set("User-Agent", "helix_agent/1.0")

		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			if attempt < p.retryConfig.MaxRetries {
				p.waitWithJitter(ctx, delay)
				delay = p.nextDelay(delay)
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode == http.StatusUnauthorized && allowAuthRetry {
			_ = resp.Body.Close()
			authRetryDelay := 500 * time.Millisecond
			p.waitWithJitter(ctx, authRetryDelay)
			return p.makeAPICallWithAuthRetry(ctx, req, false)
		}

		if isRetryableStatus(resp.StatusCode) && attempt < p.retryConfig.MaxRetries {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: retryable error", resp.StatusCode)
			p.waitWithJitter(ctx, delay)
			delay = p.nextDelay(delay)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("all %d retry attempts failed: %w", p.retryConfig.MaxRetries+1, lastErr)
}

func isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (p *XiaomiProvider) waitWithJitter(ctx context.Context, delay time.Duration) {
	jitter := time.Duration(rand.Float64() * 0.1 * float64(delay)) // #nosec G404
	select {
	case <-ctx.Done():
	case <-time.After(delay + jitter):
	}
}

func (p *XiaomiProvider) nextDelay(currentDelay time.Duration) time.Duration {
	nextDelay := time.Duration(float64(currentDelay) * p.retryConfig.Multiplier)
	if nextDelay > p.retryConfig.MaxDelay {
		nextDelay = p.retryConfig.MaxDelay
	}
	return nextDelay
}

// GetCapabilities returns the capabilities of the Xiaomi MiMo provider
func (p *XiaomiProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels: []string{
			"mimo-v2.5-pro",
			"mimo-v2.5",
			"mimo-v2-flash",
			"mimo-v2.5-asr",
			"mimo-v2.5-tts",
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
			"provider":     "Xiaomi MiMo",
			"model_family": "MiMo",
			"api_version":  "v1",
		},
	}
}

// ValidateConfig validates the provider configuration
func (p *XiaomiProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
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

// HealthCheck implements health checking for the Xiaomi MiMo provider
func (p *XiaomiProvider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	modelsURL := strings.TrimSuffix(p.baseURL, "/v1/chat/completions") + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}
