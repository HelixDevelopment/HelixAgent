package claude

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
	ClaudeAPIURL = "https://api.anthropic.com/v1/messages"
	ClaudeModel  = "claude-3-sonnet-20240229"
)

type ClaudeProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

type ClaudeRequest struct {
	Model         string          `json:"model"`
	MaxTokens     int             `json:"max_tokens,omitempty"`
	Temperature   float64         `json:"temperature,omitempty"`
	TopP          float64         `json:"top_p,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	Messages      []ClaudeMessage `json:"messages"`
	System        string          `json:"system,omitempty"`
}

type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeResponse struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Content      []ClaudeContent `json:"content"`
	Model        string          `json:"model"`
	StopReason   *string         `json:"stop_reason"`
	StopSequence *string         `json:"stop_sequence"`
	Usage        ClaudeUsage     `json:"usage"`
}

type ClaudeContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ClaudeStreamResponse struct {
	Type    string          `json:"type"`
	Message *ClaudeResponse `json:"message,omitempty"`
	Delta   *ClaudeDelta    `json:"delta,omitempty"`
	Usage   *ClaudeUsage    `json:"usage,omitempty"`
}

type ClaudeDelta struct {
	Type         string `json:"type"`
	Text         string `json:"text,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

func NewClaudeProvider(apiKey, baseURL, model string) *ClaudeProvider {
	if baseURL == "" {
		baseURL = ClaudeAPIURL
	}
	if model == "" {
		model = ClaudeModel
	}

	return &ClaudeProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *ClaudeProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	startTime := time.Now()

	// Convert internal request to Claude format
	claudeReq := p.convertRequest(req)

	// Make API call
	resp, err := p.makeAPICall(ctx, claudeReq)
	if err != nil {
		return nil, fmt.Errorf("Claude API call failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Claude API error: %d - %s", resp.StatusCode, string(body))
	}

	var claudeResp ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, fmt.Errorf("failed to parse Claude response: %w", err)
	}

	// Convert back to internal format
	return p.convertResponse(req, &claudeResp, startTime), nil
}

func (p *ClaudeProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	startTime := time.Now()

	// Convert internal request to Claude format
	claudeReq := p.convertRequest(req)
	claudeReq.Stream = true

	// Make streaming API call
	resp, err := p.makeAPICall(ctx, claudeReq)
	if err != nil {
		return nil, fmt.Errorf("Claude streaming API call failed: %w", err)
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
					ProviderID:     "claude",
					ProviderName:   "Claude",
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
			var streamResp ClaudeStreamResponse
			if err := json.Unmarshal(line, &streamResp); err != nil {
				continue // Skip malformed JSON
			}

			// Handle different event types
			switch streamResp.Type {
			case "content_block_delta":
				if streamResp.Delta != nil && streamResp.Delta.Type == "text_delta" {
					delta := streamResp.Delta.Text
					if delta != "" {
						fullContent += delta

						// Send chunk response
						chunkResp := &models.LLMResponse{
							ID:             "claude-stream-" + req.ID,
							RequestID:      req.ID,
							ProviderID:     "claude",
							ProviderName:   "Claude",
							Content:        delta,
							Confidence:     0.9, // High confidence for Claude
							TokensUsed:     1,   // Estimated
							ResponseTime:   time.Since(startTime).Milliseconds(),
							FinishReason:   "",
							Selected:       false,
							SelectionScore: 0.0,
							CreatedAt:      time.Now(),
						}
						ch <- chunkResp
					}
				}
			case "message_stop":
				// Stream finished
				break
			}
		}

		// Send final response
		finalResp := &models.LLMResponse{
			ID:             "claude-final-" + req.ID,
			RequestID:      req.ID,
			ProviderID:     "claude",
			ProviderName:   "Claude",
			Content:        "",
			Confidence:     0.9,
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

func (p *ClaudeProvider) convertRequest(req *models.LLMRequest) ClaudeRequest {
	// Convert messages
	messages := make([]ClaudeMessage, 0, len(req.Messages))

	// Add conversation messages (Claude doesn't use system prompt in messages)
	for _, msg := range req.Messages {
		if msg.Role != "system" {
			messages = append(messages, ClaudeMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Extract system message if present
	var systemPrompt string
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
			break
		}
	}

	return ClaudeRequest{
		Model:         p.model,
		MaxTokens:     req.ModelParams.MaxTokens,
		Temperature:   req.ModelParams.Temperature,
		TopP:          req.ModelParams.TopP,
		StopSequences: req.ModelParams.StopSequences,
		Stream:        false,
		Messages:      messages,
		System:        systemPrompt,
	}
}

func (p *ClaudeProvider) convertResponse(req *models.LLMRequest, claudeResp *ClaudeResponse, startTime time.Time) *models.LLMResponse {
	var content string
	var finishReason string

	if len(claudeResp.Content) > 0 {
		content = claudeResp.Content[0].Text
	}

	if claudeResp.StopReason != nil {
		finishReason = *claudeResp.StopReason
	}

	// Calculate confidence based on finish reason and response quality
	confidence := p.calculateConfidence(content, finishReason)

	return &models.LLMResponse{
		ID:           claudeResp.ID,
		RequestID:    req.ID,
		ProviderID:   "claude",
		ProviderName: "Claude",
		Content:      content,
		Confidence:   confidence,
		TokensUsed:   claudeResp.Usage.OutputTokens,
		ResponseTime: time.Since(startTime).Milliseconds(),
		FinishReason: finishReason,
		Metadata: map[string]any{
			"model":        claudeResp.Model,
			"input_tokens": claudeResp.Usage.InputTokens,
			"type":         claudeResp.Type,
		},
		Selected:       false,
		SelectionScore: 0.0,
		CreatedAt:      time.Now(),
	}
}

func (p *ClaudeProvider) calculateConfidence(content, finishReason string) float64 {
	confidence := 0.9 // High base confidence for Claude

	// Adjust based on finish reason
	switch finishReason {
	case "end_turn":
		confidence += 0.05
	case "max_tokens":
		confidence -= 0.1
	case "stop_sequence":
		confidence += 0.02
	}

	// Adjust based on content length and quality
	if len(content) > 50 {
		confidence += 0.02
	}
	if len(content) > 200 {
		confidence += 0.02
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

func (p *ClaudeProvider) makeAPICall(ctx context.Context, req ClaudeRequest) (*http.Response, error) {
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
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("User-Agent", "SuperAgent/1.0")

	// Make request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	return resp, nil
}
