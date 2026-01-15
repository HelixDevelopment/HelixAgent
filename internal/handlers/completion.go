package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/skills"
)

// CompletionHandler handles LLM completion requests
type CompletionHandler struct {
	requestService    *services.RequestService
	skillsIntegration *skills.Integration
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
	ID                string                    `json:"id"`
	Object            string                    `json:"object"`
	Created           int64                     `json:"created"`
	Model             string                    `json:"model"`
	Choices           []CompletionChoice        `json:"choices"`
	Usage             *CompletionUsage          `json:"usage,omitempty"`
	SystemFingerprint string                    `json:"system_fingerprint"`
	SkillsUsed        *skills.SkillsUsedMetadata `json:"skills_used,omitempty"`
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

// NewCompletionHandlerWithSkills creates a completion handler with Skills integration
func NewCompletionHandlerWithSkills(requestService *services.RequestService, skillsIntegration *skills.Integration) *CompletionHandler {
	return &CompletionHandler{
		requestService:    requestService,
		skillsIntegration: skillsIntegration,
	}
}

// SetSkillsIntegration sets the Skills integration for the handler
func (h *CompletionHandler) SetSkillsIntegration(integration *skills.Integration) {
	h.skillsIntegration = integration
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

	// Process Skills matching if integration is available
	var skillsCtx *skills.RequestContext
	if h.skillsIntegration != nil {
		var err error
		skillsCtx, err = h.skillsIntegration.ProcessRequest(c.Request.Context(), internalReq.ID, req.Prompt)
		if err == nil && skillsCtx != nil && len(skillsCtx.Instructions) > 0 {
			// Enhance prompt with skill instructions
			internalReq.Prompt = h.skillsIntegration.EnhancePromptWithSkills(internalReq.Prompt, skillsCtx)
		}
	}

	// Process request
	response, err := h.requestService.ProcessRequest(c.Request.Context(), internalReq)
	if err != nil {
		// Complete Skills tracking with error
		if h.skillsIntegration != nil && skillsCtx != nil {
			h.skillsIntegration.CompleteRequest(internalReq.ID, false, err.Error())
		}
		h.sendCategorizedError(c, err)
		return
	}

	// Complete Skills tracking and get usage
	var skillsUsages []skills.SkillUsage
	if h.skillsIntegration != nil && skillsCtx != nil {
		skillsUsages = h.skillsIntegration.CompleteRequest(internalReq.ID, true, "")
	}

	// Convert to API response format with Skills metadata
	apiResp := h.convertToAPIResponseWithSkills(response, skillsUsages)

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

	// Create a cancellable context for the stream
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Process streaming request
	streamChan, err := h.requestService.ProcessRequestStream(ctx, internalReq)
	if err != nil {
		h.sendCategorizedError(c, err)
		return
	}

	// Set streaming headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	// Get flusher for proper SSE streaming
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		h.sendError(c, http.StatusInternalServerError, "internal_error", "Streaming not supported", "")
		return
	}

	// Track client disconnection
	clientGone := c.Writer.CloseNotify()

StreamLoop:
	for {
		select {
		case <-clientGone:
			// Client disconnected, stop streaming
			cancel()
			return
		case <-ctx.Done():
			// Context cancelled, exit gracefully
			break StreamLoop
		case response, ok := <-streamChan:
			if !ok {
				// Channel closed, stream complete
				break StreamLoop
			}

			// Convert to streaming format
			streamResp := h.convertToStreamingResponse(response)

			// Send as Server-Sent Events
			data, err := json.Marshal(streamResp)
			if err != nil {
				continue // Skip malformed response
			}

			// Write data with error handling
			if _, err := c.Writer.Write([]byte("data: ")); err != nil {
				cancel()
				return
			}
			if _, err := c.Writer.Write(data); err != nil {
				cancel()
				return
			}
			if _, err := c.Writer.Write([]byte("\n\n")); err != nil {
				cancel()
				return
			}
			flusher.Flush()
		}
	}

	// Always send final event to properly close the stream
	c.Writer.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
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

	// Get user input for Skills matching (from messages or prompt)
	userInput := req.Prompt
	if len(req.Messages) > 0 {
		// Use the last user message for skill matching
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				userInput = req.Messages[i].Content
				break
			}
		}
	}

	// Process Skills matching if integration is available
	var skillsCtx *skills.RequestContext
	if h.skillsIntegration != nil {
		var err error
		skillsCtx, err = h.skillsIntegration.ProcessRequest(c.Request.Context(), internalReq.ID, userInput)
		if err == nil && skillsCtx != nil && len(skillsCtx.Instructions) > 0 {
			// Enhance prompt with skill instructions
			if internalReq.Prompt != "" {
				internalReq.Prompt = h.skillsIntegration.EnhancePromptWithSkills(internalReq.Prompt, skillsCtx)
			}
		}
	}

	// Process request
	response, err := h.requestService.ProcessRequest(c.Request.Context(), internalReq)
	if err != nil {
		// Complete Skills tracking with error
		if h.skillsIntegration != nil && skillsCtx != nil {
			h.skillsIntegration.CompleteRequest(internalReq.ID, false, err.Error())
		}
		h.sendError(c, http.StatusInternalServerError, "internal_error", "Failed to process chat request", err.Error())
		return
	}

	// Complete Skills tracking and get usage
	var skillsUsages []skills.SkillUsage
	if h.skillsIntegration != nil && skillsCtx != nil {
		skillsUsages = h.skillsIntegration.CompleteRequest(internalReq.ID, true, "")
	}

	// Convert to chat API response format with Skills metadata
	apiResp := h.convertToChatResponseWithSkills(response, skillsUsages)

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

	// Create a cancellable context for the stream
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Process streaming request
	streamChan, err := h.requestService.ProcessRequestStream(ctx, internalReq)
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "internal_error", "Failed to process streaming chat request", err.Error())
		return
	}

	// Set streaming headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	// Get flusher for proper SSE streaming
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		h.sendError(c, http.StatusInternalServerError, "internal_error", "Streaming not supported", "")
		return
	}

	// Track client disconnection
	clientGone := c.Writer.CloseNotify()

StreamLoop:
	for {
		select {
		case <-clientGone:
			// Client disconnected, stop streaming
			cancel()
			return
		case <-ctx.Done():
			// Context cancelled, exit gracefully
			break StreamLoop
		case response, ok := <-streamChan:
			if !ok {
				// Channel closed, stream complete
				break StreamLoop
			}

			// Convert to streaming chat format
			streamResp := h.convertToChatStreamingResponse(response)

			// Send as Server-Sent Events
			data, err := json.Marshal(streamResp)
			if err != nil {
				continue // Skip malformed response
			}

			// Write data with error handling
			if _, err := c.Writer.Write([]byte("data: ")); err != nil {
				cancel()
				return
			}
			if _, err := c.Writer.Write(data); err != nil {
				cancel()
				return
			}
			if _, err := c.Writer.Write([]byte("\n\n")); err != nil {
				cancel()
				return
			}
			flusher.Flush()
		}
	}

	// Always send final event to properly close the stream
	c.Writer.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
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
		SystemFingerprint: "helixagent-v1.0",
	}
}

// convertToAPIResponseWithSkills converts response with Skills usage metadata
func (h *CompletionHandler) convertToAPIResponseWithSkills(resp *models.LLMResponse, usages []skills.SkillUsage) *CompletionResponse {
	apiResp := h.convertToAPIResponse(resp)
	if h.skillsIntegration != nil && len(usages) > 0 {
		apiResp.SkillsUsed = h.skillsIntegration.BuildSkillsUsedSection(usages)
	}
	return apiResp
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

// convertToChatResponseWithSkills converts chat response with Skills usage metadata
func (h *CompletionHandler) convertToChatResponseWithSkills(resp *models.LLMResponse, usages []skills.SkillUsage) map[string]any {
	apiResp := h.convertToChatResponse(resp)
	if h.skillsIntegration != nil && len(usages) > 0 {
		skillsMetadata := h.skillsIntegration.BuildSkillsUsedSection(usages)
		if skillsMetadata != nil {
			apiResp["skills_used"] = skillsMetadata
		}
	}
	return apiResp
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

// sendCategorizedError sends an error response with proper HTTP status code based on error category
func (h *CompletionHandler) sendCategorizedError(c *gin.Context, err error) {
	// Check if it's already a categorized LLM service error
	if llmErr, ok := err.(*services.LLMServiceError); ok {
		response := llmErr.ToOpenAIError()
		if llmErr.RetryAfter > 0 {
			c.Header("Retry-After", fmt.Sprintf("%d", int(llmErr.RetryAfter.Seconds())))
		}
		c.JSON(llmErr.HTTPStatus, response)
		return
	}

	// Categorize unknown errors
	categorized := services.CategorizeError(err, "unknown")
	response := categorized.ToOpenAIError()
	if categorized.RetryAfter > 0 {
		c.Header("Retry-After", fmt.Sprintf("%d", int(categorized.RetryAfter.Seconds())))
	}
	c.JSON(categorized.HTTPStatus, response)
}
