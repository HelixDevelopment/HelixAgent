// Package api provides Messages API implementation for Claude Code integration.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Message represents a message in the conversation
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []ContentBlock
}

// ContentBlock represents a content block in a message
type ContentBlock struct {
	Type string `json:"type"`

	// For text type
	Text string `json:"text,omitempty"`

	// For image type
	Source *ImageSource `json:"source,omitempty"`

	// For tool_use type
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// For tool_result type
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   interface{} `json:"content,omitempty"`
	IsError   bool        `json:"is_error,omitempty"`
}

// ImageSource represents an image source
type ImageSource struct {
	Type      string `json:"type"`                 // "base64" or "url"
	MediaType string `json:"media_type,omitempty"` // e.g., "image/jpeg"
	Data      string `json:"data,omitempty"`       // base64 encoded
	URL       string `json:"url,omitempty"`
}

// Tool represents a tool definition
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolChoice represents tool choice configuration
type ToolChoice struct {
	Type string `json:"type"`           // "auto", "any", or "tool"
	Name string `json:"name,omitempty"` // required if type is "tool"
}

// MessageRequest represents a request to the Messages API
type MessageRequest struct {
	Model       string      `json:"model"`
	MaxTokens   int         `json:"max_tokens"`
	Messages    []Message   `json:"messages"`
	System      string      `json:"system,omitempty"`
	Tools       []Tool      `json:"tools,omitempty"`
	ToolChoice  interface{} `json:"tool_choice,omitempty"` // "auto", "any", or ToolChoice
	Temperature *float64    `json:"temperature,omitempty"`
	TopP        *float64    `json:"top_p,omitempty"`
	TopK        *int        `json:"top_k,omitempty"`
	Stream      bool        `json:"stream,omitempty"`
	Metadata    *Metadata   `json:"metadata,omitempty"`
}

// Metadata represents request metadata
type Metadata struct {
	UserID string `json:"user_id,omitempty"`
}

// MessageResponse represents a non-streaming response from the Messages API
type MessageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Model        string         `json:"model"`
	Content      []ContentBlock `json:"content"`
	StopReason   *string        `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        Usage          `json:"usage"`
}

// Usage represents token usage information
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamEvent represents a streaming event from the Messages API
type StreamEvent struct {
	Type         string           `json:"type"`
	Message      *MessageResponse `json:"message,omitempty"`
	Index        int              `json:"index,omitempty"`
	ContentBlock *ContentBlock    `json:"content_block,omitempty"`
	Delta        *StreamDelta     `json:"delta,omitempty"`
}

// StreamDelta represents a delta in a streaming event
type StreamDelta struct {
	Type         string `json:"type"`
	Text         string `json:"text,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

// Available Claude models
const (
	ModelClaudeOpus4_1   = "claude-opus-4-1-20251001"
	ModelClaudeSonnet4_5 = "claude-sonnet-4-5-20251001"
	ModelClaudeSonnet4_6 = "claude-sonnet-4-6-20251001"
	ModelClaudeHaiku4_5  = "claude-haiku-4-5-20251001"
)

// CreateMessage sends a message request to the Anthropic API
func (c *Client) CreateMessage(ctx context.Context, req *MessageRequest) (*MessageResponse, error) {
	resp, err := c.doRequest(ctx, "POST", "/v1/messages", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, handleErrorResponse(resp)
	}

	var result MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// CreateMessageStream sends a streaming message request to the Anthropic API
func (c *Client) CreateMessageStream(ctx context.Context, req *MessageRequest) (<-chan StreamEvent, <-chan error) {
	events := make(chan StreamEvent)
	errors := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errors)

		req.Stream = true

		resp, err := c.doRequest(ctx, "POST", "/v1/messages", req)
		if err != nil {
			errors <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			errors <- handleErrorResponse(resp)
			return
		}

		reader := NewEventStreamReader(resp.Body)

		for {
			eventType, data, err := reader.ReadEvent()
			if err != nil {
				if err != io.EOF {
					errors <- fmt.Errorf("read stream event: %w", err)
				}
				return
			}

			// Skip ping events
			if eventType == "ping" {
				continue
			}

			var event StreamEvent
			if err := json.Unmarshal(data, &event); err != nil {
				errors <- fmt.Errorf("unmarshal event: %w", err)
				return
			}

			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return events, errors
}

// CreateMessageWithTools sends a message with tool definitions
func (c *Client) CreateMessageWithTools(ctx context.Context, model, system string, messages []Message, tools []Tool, maxTokens int) (*MessageResponse, error) {
	req := &MessageRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Messages:  messages,
		System:    system,
		Tools:     tools,
	}

	return c.CreateMessage(ctx, req)
}

// SimpleMessage sends a simple text message
func (c *Client) SimpleMessage(ctx context.Context, model, userMessage string, maxTokens int) (*MessageResponse, error) {
	req := &MessageRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Messages: []Message{
			{
				Role:    "user",
				Content: userMessage,
			},
		},
	}

	return c.CreateMessage(ctx, req)
}

// CollectStreamContent collects all text content from a streaming response
func CollectStreamContent(events <-chan StreamEvent, errors <-chan error) (string, error) {
	var content strings.Builder

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return content.String(), nil
			}

			switch event.Type {
			case "content_block_delta":
				if event.Delta != nil && event.Delta.Type == "text_delta" {
					content.WriteString(event.Delta.Text)
				}
			}

		case err := <-errors:
			if err != nil {
				return content.String(), err
			}
		}
	}
}

// Helper function to create text content
type textBuilder struct{}

func TextContent(text string) interface{} {
	return text
}

func TextBlocks(blocks ...ContentBlock) interface{} {
	return blocks
}

// CreateImageBlock creates an image content block from base64 data
func CreateImageBlock(mediaType, base64Data string) ContentBlock {
	return ContentBlock{
		Type: "image",
		Source: &ImageSource{
			Type:      "base64",
			MediaType: mediaType,
			Data:      base64Data,
		},
	}
}

// CreateToolUseBlock creates a tool use content block
func CreateToolUseBlock(id, name string, input interface{}) ContentBlock {
	inputJSON, _ := json.Marshal(input)
	return ContentBlock{
		Type:  "tool_use",
		ID:    id,
		Name:  name,
		Input: inputJSON,
	}
}

// CreateToolResultBlock creates a tool result content block
func CreateToolResultBlock(toolUseID string, content interface{}, isError bool) ContentBlock {
	return ContentBlock{
		Type:      "tool_result",
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	}
}
