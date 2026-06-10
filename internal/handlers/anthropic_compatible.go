package handlers

// Anthropic-compatible /v1/messages translator (Finding #20).
//
// Claude Code and other Anthropic-protocol clients POST to /v1/messages
// expecting the Messages API shape; HelixAgent's native API is
// OpenAI-compatible. This file provides a translation shim that:
//   1. Accepts the Anthropic Messages request shape
//   2. Converts to the internal OpenAIChatRequest
//   3. Delegates to UnifiedHandler.ChatCompletions for ensemble routing
//   4. Captures the JSON response and re-shapes it into Anthropic's
//      response envelope before sending to the client
//
// Streaming is NOT implemented in this first cut; clients that send
// "stream": true currently receive a non-streaming completion. SSE
// translation (Anthropic's message_start / content_block_delta /
// message_stop event sequence) is a follow-up.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// AnthropicMessageRequest mirrors the Anthropic Messages API request shape.
// Only the fields relevant for HelixAgent ensemble routing are modelled;
// extra fields are accepted but ignored.
type AnthropicMessageRequest struct {
	Model         string                 `json:"model"`
	Messages      []AnthropicMessage     `json:"messages"`
	System        any                    `json:"system,omitempty"` // string OR []AnthropicContentBlock
	MaxTokens     int                    `json:"max_tokens"`
	Temperature   float64                `json:"temperature,omitempty"`
	TopP          float64                `json:"top_p,omitempty"`
	StopSequences []string               `json:"stop_sequences,omitempty"`
	Stream        bool                   `json:"stream,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	// Tools/ToolChoice intentionally omitted for the first cut — they
	// require a deeper schema mapping (Anthropic's input_schema vs
	// OpenAI's parameters JSONSchema). The translator returns a clear
	// error if tools are present so clients fail fast rather than
	// silently dropping the tool definitions.
	Tools []map[string]interface{} `json:"tools,omitempty"`
}

// AnthropicMessage is a single turn in the Anthropic conversation.
// Content can be a plain string OR an array of content blocks; the
// translator normalizes both shapes to a flat string for OpenAI.
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// AnthropicContentBlock is one element of a content-block array. The
// translator only honors text blocks for the first cut — tool_use and
// tool_result blocks return an explicit "not yet supported" error.
type AnthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AnthropicMessageResponse mirrors the Anthropic Messages API response
// envelope. Only the fields HelixAgent can populate from an OpenAI-shaped
// completion are filled.
type AnthropicMessageResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"` // always "message"
	Role         string                  `json:"role"` // always "assistant"
	Content      []AnthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason,omitempty"`
	StopSequence *string                 `json:"stop_sequence,omitempty"`
	Usage        AnthropicUsage          `json:"usage"`
}

// AnthropicUsage counts input + output tokens. HelixAgent uses
// best-effort token counts derived from the upstream provider response.
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicErrorResponse is the standard error shape Claude Code expects.
type AnthropicErrorResponse struct {
	Type  string         `json:"type"` // always "error"
	Error AnthropicError `json:"error"`
}

// AnthropicError is the inner error object.
type AnthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// RegisterAnthropicRoutes registers the /v1/messages translator.
// Keep separate from RegisterOpenAIRoutes so the router wiring stays
// explicit about which protocol surface is being exposed.
func (h *UnifiedHandler) RegisterAnthropicRoutes(r *gin.RouterGroup, auth gin.HandlerFunc) {
	protected := r.Group("").Use(auth)
	protected.POST("/messages", h.AnthropicMessages)
}

// AnthropicMessages is the /v1/messages handler. It translates the
// request to OpenAI shape, delegates to ChatCompletions via an internal
// re-dispatch, captures the JSON response, and re-shapes it into the
// Anthropic envelope.
func (h *UnifiedHandler) AnthropicMessages(c *gin.Context) {
	logrus.Info("[ENTRY] AnthropicMessages handler called")

	var req AnthropicMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		sendAnthropicError(c, http.StatusBadRequest, "invalid_request_error",
			"Invalid request body: "+err.Error())
		return
	}

	if len(req.Tools) > 0 {
		sendAnthropicError(c, http.StatusBadRequest, "invalid_request_error",
			"Anthropic tools are not yet supported by HelixAgent's /v1/messages "+
				"translator (Finding #20). Use /v1/chat/completions with OpenAI tool "+
				"format, or contact maintainers if you need this.")
		return
	}

	if req.Stream {
		sendAnthropicError(c, http.StatusBadRequest, "invalid_request_error",
			"Streaming is not yet supported by HelixAgent's /v1/messages "+
				"translator. Set stream=false or use /v1/chat/completions for "+
				"OpenAI-style SSE.")
		return
	}

	openaiReq, err := translateAnthropicToOpenAI(&req)
	if err != nil {
		sendAnthropicError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	openaiResp, statusCode, err := h.dispatchToChatCompletions(c, openaiReq)
	if err != nil {
		sendAnthropicError(c, http.StatusInternalServerError, "api_error",
			"Failed to dispatch to chat completions: "+err.Error())
		return
	}
	if statusCode != http.StatusOK {
		// Pass through upstream error with Anthropic-shaped envelope.
		sendAnthropicError(c, statusCode, "api_error",
			fmt.Sprintf("Upstream chat completions returned status %d", statusCode))
		return
	}

	anthropicResp := translateOpenAIToAnthropic(openaiResp, req.Model)
	c.JSON(http.StatusOK, anthropicResp)
}

// translateAnthropicToOpenAI converts an Anthropic request to the
// internal OpenAI chat shape. Returns an error if the request shape
// is unsupported (e.g. content blocks of unrecognized types).
func translateAnthropicToOpenAI(req *AnthropicMessageRequest) (*OpenAIChatRequest, error) {
	out := &OpenAIChatRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.StopSequences,
		Stream:      false, // already rejected above
	}

	if sys := flattenAnthropicSystem(req.System); sys != "" {
		out.Messages = append(out.Messages, OpenAIMessage{
			Role:    "system",
			Content: sys,
		})
	}

	for i, m := range req.Messages {
		text, err := flattenAnthropicMessageContent(m.Content)
		if err != nil {
			return nil, fmt.Errorf("messages[%d]: %w", i, err)
		}
		out.Messages = append(out.Messages, OpenAIMessage{
			Role:    m.Role,
			Content: text,
		})
	}

	return out, nil
}

// flattenAnthropicSystem normalizes the `system` field, which may be
// either a string or an array of content blocks.
func flattenAnthropicSystem(sys any) string {
	if sys == nil {
		return ""
	}
	switch v := sys.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, blk := range v {
			if m, ok := blk.(map[string]interface{}); ok {
				if t, _ := m["type"].(string); t == "text" {
					if txt, _ := m["text"].(string); txt != "" {
						parts = append(parts, txt)
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// flattenAnthropicMessageContent normalizes a per-message content
// field, which may be string OR []AnthropicContentBlock OR
// []map[string]interface{} (when JSON-decoded into `any`).
func flattenAnthropicMessageContent(content any) (string, error) {
	if content == nil {
		return "", nil
	}
	switch v := content.(type) {
	case string:
		return v, nil
	case []interface{}:
		var parts []string
		for _, blk := range v {
			m, ok := blk.(map[string]interface{})
			if !ok {
				continue
			}
			t, _ := m["type"].(string)
			switch t {
			case "text":
				if txt, _ := m["text"].(string); txt != "" {
					parts = append(parts, txt)
				}
			case "tool_use", "tool_result":
				return "", fmt.Errorf(
					"content block type %q is not yet supported by /v1/messages translator", t)
			}
		}
		return strings.Join(parts, "\n"), nil
	}
	return "", fmt.Errorf("unsupported content shape %T", content)
}

// dispatchToChatCompletions invokes ChatCompletions in-process by
// rebuilding a Gin context that wraps a captured ResponseWriter. This
// avoids a recursive HTTP round-trip while still going through the
// full ensemble/orchestrator pipeline that ChatCompletions provides.
func (h *UnifiedHandler) dispatchToChatCompletions(
	src *gin.Context, openaiReq *OpenAIChatRequest,
) ([]byte, int, error) {
	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal openai request: %w", err)
	}

	// Build an inner Gin context with the OpenAI-shaped body.
	innerReq, err := http.NewRequestWithContext(
		src.Request.Context(),
		http.MethodPost,
		"/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("build inner request: %w", err)
	}
	innerReq.Header = src.Request.Header.Clone()
	innerReq.Header.Set("Content-Type", "application/json")
	innerReq.ContentLength = int64(len(body))

	rec := newCapturingResponseWriter()
	innerCtx, _ := gin.CreateTestContext(rec)
	innerCtx.Request = innerReq

	h.ChatCompletions(innerCtx)

	return rec.Body(), rec.StatusCode(), nil
}

// translateOpenAIToAnthropic re-shapes an OpenAI chat completion JSON
// blob into the Anthropic Messages response envelope.
func translateOpenAIToAnthropic(openaiResp []byte, requestedModel string) *AnthropicMessageResponse {
	var openai struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(openaiResp, &openai)

	content := ""
	finish := "end_turn"
	if len(openai.Choices) > 0 {
		content = openai.Choices[0].Message.Content
		finish = mapFinishReason(openai.Choices[0].FinishReason)
	}

	id := openai.ID
	if id == "" {
		// D-20: uuid suffix makes the fallback message ID unique-by-construction;
		// the bare UnixNano suffix collided within one coarse clock tick.
		id = fmt.Sprintf("msg_%d_%s", time.Now().UnixNano(), uuid.New().String()[:8])
	}
	model := openai.Model
	if model == "" {
		model = requestedModel
	}

	return &AnthropicMessageResponse{
		ID:    id,
		Type:  "message",
		Role:  "assistant",
		Model: model,
		Content: []AnthropicContentBlock{
			{Type: "text", Text: content},
		},
		StopReason: finish,
		Usage: AnthropicUsage{
			InputTokens:  openai.Usage.PromptTokens,
			OutputTokens: openai.Usage.CompletionTokens,
		},
	}
}

// mapFinishReason translates OpenAI's finish_reason to Anthropic's
// stop_reason vocabulary. "stop" → "end_turn"; "length" → "max_tokens";
// unknown → "end_turn" (best-effort default).
func mapFinishReason(openaiReason string) string {
	switch openaiReason {
	case "stop", "":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls", "function_call":
		return "tool_use"
	default:
		return "end_turn"
	}
}

// sendAnthropicError writes an Anthropic-shaped error envelope.
func sendAnthropicError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, AnthropicErrorResponse{
		Type: "error",
		Error: AnthropicError{
			Type:    errType,
			Message: message,
		},
	})
}

// capturingResponseWriter implements http.ResponseWriter for the
// in-process dispatch in dispatchToChatCompletions. Captures status +
// body without sending anything over the wire.
type capturingResponseWriter struct {
	header     http.Header
	body       *bytes.Buffer
	statusCode int
}

func newCapturingResponseWriter() *capturingResponseWriter {
	return &capturingResponseWriter{
		header:     make(http.Header),
		body:       &bytes.Buffer{},
		statusCode: http.StatusOK,
	}
}

func (w *capturingResponseWriter) Header() http.Header { return w.header }
func (w *capturingResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}
func (w *capturingResponseWriter) WriteHeader(code int) { w.statusCode = code }
func (w *capturingResponseWriter) Body() []byte         { return w.body.Bytes() }
func (w *capturingResponseWriter) StatusCode() int      { return w.statusCode }

// drainReader is a small helper for tests to compare captured bytes.
var _ io.Reader = (*bytes.Reader)(nil)
