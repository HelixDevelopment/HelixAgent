package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
)

// CompletionHandler handles LLM completion requests
type CompletionHandler struct {
	requestService *services.RequestService
}

// CompletionRequest represents the API request for completion
type CompletionRequest struct {
	Prompt         string                 `json:"prompt" binding:"required"`
	Messages       []models.Message       `json:"messages,omitempty"`
	Model          string                 `json:"model,omitempty"`
	Temperature    float64                `json:"temperature,omitempty"`
	MaxTokens      int                    `json:"max_tokens,omitempty"`
	TopP           float64                `json:"top_p,omitempty"`
	Stop           []string               `json:"stop,omitempty"`
	Stream         bool                   `json:"stream,omitempty"`
	EnsembleConfig *models.EnsembleConfig `json:"ensemble_config,omitempty"`
	MemoryEnhanced bool                   `json:"memory_enhanced,omitempty"`
	RequestType    string                 `json:"request_type,omitempty"`
}

// CompletionResponse represents the API response for completion
type CompletionResponse struct {
	ID                string             `json:"id"`
	Object            string             `json:"object"`
	Created           int64              `json:"created"`
	Model             string             `json:"model"`
	Choices           []CompletionChoice `json:"choices"`
	Usage             *CompletionUsage   `json:"usage,omitempty"`
	SystemFingerprint string             `json:"system_fingerprint"`
}

// CompletionChoice represents a choice in the completion response
type CompletionChoice struct {
	Index        int                 `json:"index"`
	Message      models.Message      `json:"message"`
	FinishReason string              `json:"finish_reason"`
	LogProbs     *CompletionLogProbs `json:"logprobs,omitempty"`
}

// CompletionUsage represents token usage information
type CompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CompletionLogProbs represents log probabilities
type CompletionLogProbs struct {
	Token       map[string]float64  `json:"token,omitempty"`
	TopLogprobs []CompletionLogProb `json:"top_logprobs,omitempty"`
	TextOffset  int                 `json:"text_offset,omitempty"`
}

// CompletionLogProb represents a log probability entry
type CompletionLogProb struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []byte  `json:"bytes,omitempty"`
	Offset  int     `json:"offset,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func NewCompletionHandler(requestService *services.RequestService) *CompletionHandler {
	return &CompletionHandler{
		requestService: requestService,
	}
}

// Complete handles non-streaming completion requests
func (h *CompletionHandler) Complete(c *gin.Context) {
	// Parse request
	var req CompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, http.StatusBadRequest, "invalid_request", "Invalid request format", err.Error())
		return
	}

	// Convert to internal request format
	internalReq := h.convertToInternalRequest(&req, c)

	// Process request
	response, err := h.requestService.ProcessRequest(c.Request.Context(), internalReq)
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "internal_error", "Failed to process request", err.Error())
		return
	}

	// Convert to API response format
	apiResp := h.convertToAPIResponse(response)

	c.JSON(http.StatusOK, apiResp)
}

// CompleteStream handles streaming completion requests
func (h *CompletionHandler) CompleteStream(c *gin.Context) {
	// Parse request
	var req CompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, http.StatusBadRequest, "invalid_request", "Invalid request format", err.Error())
		return
	}

	// Force streaming mode
	req.Stream = true

	// Convert to internal request format
	internalReq := h.convertToInternalRequest(&req, c)

	// Process streaming request
	streamChan, err := h.requestService.ProcessRequestStream(c.Request.Context(), internalReq)
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "internal_error", "Failed to process streaming request", err.Error())
		return
	}

	// Set streaming headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Stream responses
	for response := range streamChan {
		// Convert to streaming format
		streamResp := h.convertToStreamingResponse(response)

		// Send as Server-Sent Events
		data, _ := json.Marshal(streamResp)
		c.Writer.Write([]byte("data: "))
		c.Writer.Write(data)
		c.Writer.Write([]byte("\n\n"))
		c.Writer.Flush()
	}

	// Send final event
	c.Writer.Write([]byte("data: [DONE]\n\n"))
	c.Writer.Flush()
}

// Chat handles chat-style completion requests
func (h *CompletionHandler) Chat(c *gin.Context) {
	// Parse request
	var req CompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, http.StatusBadRequest, "invalid_request", "Invalid request format", err.Error())
		return
	}

	// Convert to internal request format
	internalReq := h.convertToInternalRequest(&req, c)
	internalReq.RequestType = "chat"

	// Process request
	response, err := h.requestService.ProcessRequest(c.Request.Context(), internalReq)
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "internal_error", "Failed to process chat request", err.Error())
		return
	}

	// Convert to chat API response format
	apiResp := h.convertToChatResponse(response)

	c.JSON(http.StatusOK, apiResp)
}

// ChatStream handles streaming chat requests
func (h *CompletionHandler) ChatStream(c *gin.Context) {
	// Parse request
	var req CompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, http.StatusBadRequest, "invalid_request", "Invalid request format", err.Error())
		return
	}

	// Force streaming mode
	req.Stream = true

	// Convert to internal request format
	internalReq := h.convertToInternalRequest(&req, c)
	internalReq.RequestType = "chat"

	// Process streaming request
	streamChan, err := h.requestService.ProcessRequestStream(c.Request.Context(), internalReq)
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "internal_error", "Failed to process streaming chat request", err.Error())
		return
	}

	// Set streaming headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Stream responses
	for response := range streamChan {
		// Convert to streaming chat format
		streamResp := h.convertToChatStreamingResponse(response)

		// Send as Server-Sent Events
		data, _ := json.Marshal(streamResp)
		c.Writer.Write([]byte("data: "))
		c.Writer.Write(data)
		c.Writer.Write([]byte("\n\n"))
		c.Writer.Flush()
	}

	// Send final event
	c.Writer.Write([]byte("data: [DONE]\n\n"))
	c.Writer.Flush()
}

// Models handles model listing requests
func (h *CompletionHandler) Models(c *gin.Context) {
	// Create model list response
	models := []map[string]any{
		{
			"id":         "deepseek-coder",
			"object":     "model",
			"created":    time.Now().Unix(),
			"owned_by":   "deepseek",
			"permission": "code_generation",
			"root":       "deepseek",
			"parent":     nil,
		},
		{
			"id":         "claude-3-sonnet-20240229",
			"object":     "model",
			"created":    time.Now().Unix(),
			"owned_by":   "anthropic",
			"permission": "reasoning",
			"root":       "claude",
			"parent":     nil,
		},
		{
			"id":         "gemini-pro",
			"object":     "model",
			"created":    time.Now().Unix(),
			"owned_by":   "google",
			"permission": "multimodal",
			"root":       "gemini",
			"parent":     nil,
		},
	}

	response := map[string]any{
		"object": "list",
		"data":   models,
	}

	c.JSON(http.StatusOK, response)
}

// Helper methods

func (h *CompletionHandler) convertToInternalRequest(req *CompletionRequest, c *gin.Context) *models.LLMRequest {
	// Generate request ID
	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())

	// Get user ID from context (set by auth middleware)
	userID := "anonymous"
	if userIDValue, exists := c.Get("user_id"); exists {
		if uid, ok := userIDValue.(string); ok {
			userID = uid
		}
	}

	// Get session ID from context
	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())
	if sessionIDValue, exists := c.Get("session_id"); exists {
		if sid, ok := sessionIDValue.(string); ok {
			sessionID = sid
		}
	}

	// Convert messages
	messages := make([]models.Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, models.Message{
			Role:      msg.Role,
			Content:   msg.Content,
			Name:      msg.Name,
			ToolCalls: msg.ToolCalls,
		})
	}

	// Set default values
	temperature := req.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1000
	}

	topP := req.TopP
	if topP == 0 {
		topP = 1.0
	}

	stopSequences := req.Stop
	if stopSequences == nil {
		stopSequences = []string{}
	}

	// Create ensemble config if not provided
	ensembleConfig := req.EnsembleConfig
	if ensembleConfig == nil {
		ensembleConfig = &models.EnsembleConfig{
			Strategy:            "confidence_weighted",
			MinProviders:        2,
			ConfidenceThreshold: 0.8,
			FallbackToBest:      true,
			Timeout:             30,
			PreferredProviders:  []string{},
		}
	}

	return &models.LLMRequest{
		ID:        requestID,
		SessionID: sessionID,
		UserID:    userID,
		Prompt:    req.Prompt,
		Messages:  messages,
		ModelParams: models.ModelParameters{
			Model:            req.Model,
			Temperature:      temperature,
			MaxTokens:        maxTokens,
			TopP:             topP,
			StopSequences:    stopSequences,
			ProviderSpecific: map[string]any{},
		},
		EnsembleConfig: ensembleConfig,
		MemoryEnhanced: req.MemoryEnhanced,
		Memory:         map[string]string{},
		Status:         "pending",
		CreatedAt:      time.Now(),
		RequestType:    req.RequestType,
	}
}

func (h *CompletionHandler) convertToAPIResponse(resp *models.LLMResponse) *CompletionResponse {
	return &CompletionResponse{
		ID:      resp.ID,
		Object:  "text_completion",
		Created: resp.CreatedAt.Unix(),
		Model:   resp.ProviderName,
		Choices: []CompletionChoice{
			{
				Index: 0,
				Message: models.Message{
					Role:    "assistant",
					Content: resp.Content,
				},
				FinishReason: resp.FinishReason,
			},
		},
		Usage: &CompletionUsage{
			PromptTokens:     resp.TokensUsed / 2, // Estimate
			CompletionTokens: resp.TokensUsed / 2, // Estimate
			TotalTokens:      resp.TokensUsed,
		},
		SystemFingerprint: "superagent-v1.0",
	}
}

func (h *CompletionHandler) convertToStreamingResponse(resp *models.LLMResponse) map[string]any {
	return map[string]any{
		"id":      resp.ID,
		"object":  "text_completion",
		"created": resp.CreatedAt.Unix(),
		"model":   resp.ProviderName,
		"choices": []map[string]any{
			{
				"index": 0,
				"delta": map[string]any{
					"content": resp.Content,
				},
				"finish_reason": resp.FinishReason,
			},
		},
	}
}

func (h *CompletionHandler) convertToChatResponse(resp *models.LLMResponse) map[string]any {
	return map[string]any{
		"id":      resp.ID,
		"object":  "chat.completion",
		"created": resp.CreatedAt.Unix(),
		"model":   resp.ProviderName,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": resp.Content,
				},
				"finish_reason": resp.FinishReason,
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     resp.TokensUsed / 2,
			"completion_tokens": resp.TokensUsed / 2,
			"total_tokens":      resp.TokensUsed,
		},
	}
}

func (h *CompletionHandler) convertToChatStreamingResponse(resp *models.LLMResponse) map[string]any {
	return map[string]any{
		"id":      resp.ID,
		"object":  "chat.completion.chunk",
		"created": resp.CreatedAt.Unix(),
		"model":   resp.ProviderName,
		"choices": []map[string]any{
			{
				"index": 0,
				"delta": map[string]any{
					"role":    "assistant",
					"content": resp.Content,
				},
			},
		},
	}
}

func (h *CompletionHandler) sendError(c *gin.Context, statusCode int, errorType, message, details string) {
	errorResp := ErrorResponse{
		Error: struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		}{
			Message: fmt.Sprintf("%s: %s", message, details),
			Type:    errorType,
			Code:    strconv.Itoa(statusCode),
		},
	}

	c.JSON(statusCode, errorResp)
}
