package deepseek

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/superagent/superagent/internal/models"
)

const (
	DeepSeekAPIURL = "https://api.deepseek.com/v1/chat/completions"
	DeepSeekModel  = "deepseek-coder"
)

type DeepSeekProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
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

func NewDeepSeekProvider(apiKey, baseURL, model string) *DeepSeekProvider {
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

	return DeepSeekRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: req.ModelParams.Temperature,
		MaxTokens:   req.ModelParams.MaxTokens,
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

	// Create HTTP request
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
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	return resp, nil
}
