package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
)

// UnifiedHandler provides 100% OpenAI-compatible API with automatic ensemble support
type UnifiedHandler struct {
	providerRegistry *services.ProviderRegistry
	config           *config.Config
}

// NewUnifiedHandler creates a new unified handler
func NewUnifiedHandler(registry *services.ProviderRegistry, cfg *config.Config) *UnifiedHandler {
	return &UnifiedHandler{
		providerRegistry: registry,
		config:           cfg,
	}
}

// OpenAIModelsResponse represents OpenAI models API response
type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// OpenAIModel represents a model in OpenAI API format
type OpenAIModel struct {
	ID         string                  `json:"id"`
	Object     string                  `json:"object"`
	Created    int64                   `json:"created"`
	OwnedBy    string                  `json:"owned_by"`
	Permission []OpenAIModelPermission `json:"permission"`
	Root       string                  `json:"root"`
	Parent     *string                 `json:"parent"`
}

// OpenAIModelPermission represents model permissions in OpenAI format
type OpenAIModelPermission struct {
	ID                 string  `json:"id"`
	Object             string  `json:"object"`
	Created            int64   `json:"created"`
	AllowCreateEngine  bool    `json:"allow_create_engine"`
	AllowSampling      bool    `json:"allow_sampling"`
	AllowLogprobs      bool    `json:"allow_logprobs"`
	AllowSearchIndices bool    `json:"allow_search_indices"`
	AllowView          bool    `json:"allow_view"`
	AllowFineTuning    bool    `json:"allow_fine_tuning"`
	Organization       string  `json:"organization"`
	Group              *string `json:"group"`
	IsBlocking         bool    `json:"is_blocking"`
}

// OpenAIChatRequest represents OpenAI chat completion request
type OpenAIChatRequest struct {
	Model            string             `json:"model"`
	Messages         []OpenAIMessage    `json:"messages"`
	MaxTokens        int                `json:"max_tokens,omitempty"`
	Temperature      float64            `json:"temperature,omitempty"`
	TopP             float64            `json:"top_p,omitempty"`
	Stream           bool               `json:"stream,omitempty"`
	Stop             []string           `json:"stop,omitempty"`
	PresencePenalty  float64            `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64            `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]float64 `json:"logit_bias,omitempty"`
	User             string             `json:"user,omitempty"`
	// SuperAgent extensions
	EnsembleConfig *models.EnsembleConfig `json:"ensemble_config,omitempty"`
	ForceProvider  string                 `json:"force_provider,omitempty"`
}

// OpenAIMessage represents a message in OpenAI chat format
type OpenAIMessage struct {
	Role         string              `json:"role"`
	Content      string              `json:"content"`
	Name         *string             `json:"name,omitempty"`
	FunctionCall *OpenAIFunctionCall `json:"function_call,omitempty"`
	ToolCalls    []OpenAIToolCall    `json:"tool_calls,omitempty"`
	ToolCallID   string              `json:"tool_call_id,omitempty"`
}

// OpenAIFunctionCall represents function call in OpenAI format
type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// OpenAIToolCall represents tool call in OpenAI format
type OpenAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function OpenAIFunctionCall `json:"function"`
}

// OpenAIChatResponse represents OpenAI chat completion response
type OpenAIChatResponse struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []OpenAIChoice `json:"choices"`
	Usage             *OpenAIUsage   `json:"usage,omitempty"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
}

// OpenAIChoice represents a choice in OpenAI response
type OpenAIChoice struct {
	Index        int             `json:"index"`
	Message      OpenAIMessage   `json:"message"`
	FinishReason string          `json:"finish_reason"`
	Logprobs     *OpenAILogprobs `json:"logprobs,omitempty"`
}

// OpenAIUsage represents token usage in OpenAI format
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAILogprobs represents log probabilities in OpenAI format
type OpenAILogprobs struct {
	TokenOffset map[string]float64 `json:"token_offset,omitempty"`
	TopLogprobs []OpenAILogprob    `json:"top_logprobs,omitempty"`
	TextOffset  int                `json:"text_offset,omitempty"`
}

// OpenAILogprob represents a log probability entry
type OpenAILogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []byte  `json:"bytes,omitempty"`
	Offset  int     `json:"offset,omitempty"`
}

// RegisterOpenAIRoutes registers OpenAI-compatible routes
func (h *UnifiedHandler) RegisterOpenAIRoutes(r *gin.RouterGroup, auth gin.HandlerFunc) {
	// Apply auth middleware to protected routes
	protected := r.Group("").Use(auth)

	{
		// Chat completions - main endpoint for AI agents like OpenCode/Crush
		protected.POST("/chat/completions", h.ChatCompletions)
		protected.POST("/chat/completions/stream", h.ChatCompletionsStream)

		// Completions - legacy text completions
		protected.POST("/completions", h.Completions)
		protected.POST("/completions/stream", h.CompletionsStream)

		// Models endpoint - exposes all available models from all providers
		protected.GET("/models", h.Models)
	}
}

// ChatCompletions handles OpenAI chat completions with automatic ensemble
// Supports both streaming and non-streaming modes based on the "stream" parameter
func (h *UnifiedHandler) ChatCompletions(c *gin.Context) {
	// Parse OpenAI request
	var req OpenAIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendOpenAIError(c, http.StatusBadRequest, "invalid_request", "Invalid request format", err.Error())
		return
	}

	// If streaming is requested, handle with SSE streaming
	if req.Stream {
		h.handleStreamingChatCompletions(c, &req)
		return
	}

	// Convert to internal request format
	internalReq := h.convertOpenAIChatRequest(&req, c)

	// Process with ensemble for best results
	result, err := h.processWithEnsemble(c.Request.Context(), internalReq, &req)
	if err != nil {
		h.sendCategorizedError(c, err)
		return
	}

	// Convert to OpenAI response format
	response := h.convertToOpenAIChatResponse(result, &req)

	c.JSON(http.StatusOK, response)
}

// handleStreamingChatCompletions handles streaming chat completions with SSE
func (h *UnifiedHandler) handleStreamingChatCompletions(c *gin.Context, req *OpenAIChatRequest) {
	// Convert to internal request format
	internalReq := h.convertOpenAIChatRequest(req, c)

	// Create a cancellable context with timeout for the stream
	// Maximum 120 seconds to prevent endless loops (OpenCode fix)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	// Process with ensemble streaming
	streamChan, err := h.processWithEnsembleStream(ctx, internalReq, req)
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

	// Stream responses
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		h.sendOpenAIError(c, http.StatusInternalServerError, "internal_error", "Streaming not supported", "")
		return
	}

	// Track streaming state for OpenCode/Crush/HelixCode compatibility
	chunksSent := 0
	isFirstChunk := true
	sentFinalChunk := false // Track if we've already sent a finish_reason chunk
	streamID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()) // Consistent ID across all chunks

	// Client disconnect detection - safely get CloseNotify channel
	// Note: In test environments, httptest.ResponseRecorder doesn't support CloseNotify
	// but gin's wrapper implements it, causing a panic when delegating. Use recover.
	var clientGone <-chan bool
	dummyChan := make(chan bool)
	clientGone = dummyChan
	func() {
		defer func() {
			if r := recover(); r != nil {
				// CloseNotify not supported (test environment), use dummy channel
			}
		}()
		if cn, ok := c.Writer.(http.CloseNotifier); ok {
			clientGone = cn.CloseNotify()
		}
	}()

	// Idle timeout ticker - if no data for 30 seconds, exit
	idleTimeout := time.NewTicker(30 * time.Second)
	defer idleTimeout.Stop()

	// Send first chunk with role (required by OpenCode)
	firstChunkResp := &models.LLMResponse{ID: streamID, CreatedAt: time.Now()}
	firstChunk := h.convertToOpenAIChatStreamResponse(firstChunkResp, req, true, streamID)
	if firstData, err := json.Marshal(firstChunk); err == nil {
		c.Writer.Write([]byte("data: "))
		c.Writer.Write(firstData)
		c.Writer.Write([]byte("\n\n"))
		flusher.Flush()
	}
	isFirstChunk = false

StreamLoop:
	for {
		select {
		case <-clientGone:
			// Client disconnected, stop streaming
			cancel()
			return
		case <-ctx.Done():
			// Context cancelled or timed out, exit gracefully
			break StreamLoop
		case <-idleTimeout.C:
			// No data received for 30 seconds, break to send DONE
			break StreamLoop
		case response, ok := <-streamChan:
			if !ok {
				// Channel closed, stream complete
				break StreamLoop
			}

			// Reset idle timeout on data received
			idleTimeout.Reset(30 * time.Second)

			// Skip empty content chunks (Issue #2840 fix for OpenCode)
			if response.Content == "" && response.FinishReason == "" {
				continue
			}

			// Track if this response already has finish_reason
			if response.FinishReason != "" {
				sentFinalChunk = true
			}

			// Convert to streaming format with consistent streamID
			streamResp := h.convertToOpenAIChatStreamResponse(response, req, isFirstChunk, streamID)

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
			chunksSent++
		}
	}

	// Only send final chunk if we haven't already sent one with finish_reason
	if !sentFinalChunk {
		finalChunk := map[string]any{
			"id":                 streamID, // Same ID as all other chunks
			"object":             "chat.completion.chunk",
			"created":            time.Now().Unix(),
			"model":              "superagent-ensemble",
			"system_fingerprint": "fp_superagent_v1",
			"choices": []map[string]any{
				{
					"index":         0,
					"delta":         map[string]any{},
					"logprobs":      nil,
					"finish_reason": "stop",
				},
			},
		}
		finalData, _ := json.Marshal(finalChunk)
		c.Writer.Write([]byte("data: "))
		c.Writer.Write(finalData)
		c.Writer.Write([]byte("\n\n"))
		flusher.Flush()
	}

	// Always send [DONE] to properly close the stream
	c.Writer.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

// ChatCompletionsStream handles streaming OpenAI chat completions
func (h *UnifiedHandler) ChatCompletionsStream(c *gin.Context) {
	// Parse OpenAI request
	var req OpenAIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendOpenAIError(c, http.StatusBadRequest, "invalid_request", "Invalid request format", err.Error())
		return
	}

	// Force streaming mode
	req.Stream = true

	// Convert to internal request format
	internalReq := h.convertOpenAIChatRequest(&req, c)

	// Create a cancellable context with timeout for the stream
	// Maximum 120 seconds to prevent endless loops (OpenCode fix)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	// Process with ensemble streaming
	streamChan, err := h.processWithEnsembleStream(ctx, internalReq, &req)
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
		h.sendOpenAIError(c, http.StatusInternalServerError, "internal_error", "Streaming not supported", "")
		return
	}

	// Track streaming state for OpenCode/Crush/HelixCode compatibility
	isFirstChunk := true
	sentFinalChunk := false // Track if we've already sent a finish_reason chunk
	streamID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()) // Consistent ID across all chunks

	// Client disconnect detection - safely get CloseNotify channel
	// Note: In test environments, httptest.ResponseRecorder doesn't support CloseNotify
	// but gin's wrapper implements it, causing a panic when delegating. Use recover.
	var clientGone <-chan bool
	dummyChan := make(chan bool)
	clientGone = dummyChan
	func() {
		defer func() {
			if r := recover(); r != nil {
				// CloseNotify not supported (test environment), use dummy channel
			}
		}()
		if cn, ok := c.Writer.(http.CloseNotifier); ok {
			clientGone = cn.CloseNotify()
		}
	}()

	// Idle timeout ticker - if no data for 30 seconds, exit
	idleTimeout := time.NewTicker(30 * time.Second)
	defer idleTimeout.Stop()

	// Send first chunk with role (required by OpenCode)
	firstChunkResp := &models.LLMResponse{ID: streamID, CreatedAt: time.Now()}
	firstChunk := h.convertToOpenAIChatStreamResponse(firstChunkResp, &req, true, streamID)
	if firstData, err := json.Marshal(firstChunk); err == nil {
		c.Writer.Write([]byte("data: "))
		c.Writer.Write(firstData)
		c.Writer.Write([]byte("\n\n"))
		flusher.Flush()
	}
	isFirstChunk = false

StreamLoop:
	for {
		select {
		case <-clientGone:
			// Client disconnected, stop streaming
			cancel()
			return
		case <-ctx.Done():
			// Context cancelled or timed out, exit gracefully
			break StreamLoop
		case <-idleTimeout.C:
			// No data received for 30 seconds, break to send DONE
			break StreamLoop
		case response, ok := <-streamChan:
			if !ok {
				// Channel closed, stream complete
				break StreamLoop
			}

			// Reset idle timeout on data received
			idleTimeout.Reset(30 * time.Second)

			// Skip empty content chunks (Issue #2840 fix for OpenCode)
			if response.Content == "" && response.FinishReason == "" {
				continue
			}

			// Track if this response already has finish_reason
			if response.FinishReason != "" {
				sentFinalChunk = true
			}

			// Convert to streaming format with consistent streamID
			streamResp := h.convertToOpenAIChatStreamResponse(response, &req, isFirstChunk, streamID)

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

	// Only send final chunk if we haven't already sent one with finish_reason
	if !sentFinalChunk {
		finalChunk := map[string]any{
			"id":                 streamID, // Same ID as all other chunks
			"object":             "chat.completion.chunk",
			"created":            time.Now().Unix(),
			"model":              "superagent-ensemble",
			"system_fingerprint": "fp_superagent_v1",
			"choices": []map[string]any{
				{
					"index":         0,
					"delta":         map[string]any{},
					"logprobs":      nil,
					"finish_reason": "stop",
				},
			},
		}
		finalData, _ := json.Marshal(finalChunk)
		c.Writer.Write([]byte("data: "))
		c.Writer.Write(finalData)
		c.Writer.Write([]byte("\n\n"))
		flusher.Flush()
	}

	// Always send [DONE] to properly close the stream
	c.Writer.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

// Completions handles legacy text completions
func (h *UnifiedHandler) Completions(c *gin.Context) {
	// Convert to chat format and process
	var req struct {
		Model       string  `json:"model"`
		Prompt      string  `json:"prompt"`
		MaxTokens   int     `json:"max_tokens,omitempty"`
		Temperature float64 `json:"temperature,omitempty"`
		Stream      bool    `json:"stream,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendOpenAIError(c, http.StatusBadRequest, "invalid_request", "Invalid request format", err.Error())
		return
	}

	// Convert to chat request and call directly
	h.ChatCompletions(c)
}

// CompletionsStream handles legacy streaming text completions
func (h *UnifiedHandler) CompletionsStream(c *gin.Context) {
	h.Completions(c) // Reuse completions logic
}

// Models returns only the SuperAgent virtual model
// SuperAgent exposes a single unified model that internally uses AI debate ensemble
// Backend provider models are implementation details and not exposed to clients
func (h *UnifiedHandler) Models(c *gin.Context) {
	// SuperAgent exposes only ONE virtual model: superagent-debate
	// This model internally uses AI debate ensemble with multiple LLMs
	superagentModel := OpenAIModel{
		ID:      "superagent-debate",
		Object:  "model",
		Created: time.Now().Unix(),
		OwnedBy: "superagent",
		Permission: []OpenAIModelPermission{
			{
				ID:                 "superagent-debate-permission",
				Object:             "model_permission",
				Created:            time.Now().Unix(),
				AllowCreateEngine:  true,
				AllowSampling:      true,
				AllowLogprobs:      true,
				AllowSearchIndices: true,
				AllowView:          true,
				AllowFineTuning:    false,
				Organization:       "superagent",
				IsBlocking:         false,
			},
		},
		Root:   "superagent-debate",
		Parent: nil,
	}

	response := OpenAIModelsResponse{
		Object: "list",
		Data:   []OpenAIModel{superagentModel},
	}

	c.JSON(http.StatusOK, response)
}

// ModelsPublic is public version of models endpoint
func (h *UnifiedHandler) ModelsPublic(c *gin.Context) {
	h.Models(c) // Same implementation
}

// Helper methods

func (h *UnifiedHandler) convertOpenAIChatRequest(req *OpenAIChatRequest, c *gin.Context) *models.LLMRequest {
	// Generate request ID
	requestID := fmt.Sprintf("openai_%d", time.Now().UnixNano())

	// Get user info from context
	userID := "anonymous"
	if uid, exists := c.Get("user_id"); exists {
		if uidStr, ok := uid.(string); ok {
			userID = uidStr
		}
	}

	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())
	if sid, exists := c.Get("session_id"); exists {
		if sidStr, ok := sid.(string); ok {
			sessionID = sidStr
		}
	}

	// Convert messages
	messages := make([]models.Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		toolCalls := make(map[string]interface{})
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				toolCalls[tc.ID] = map[string]interface{}{
					"type":     tc.Type,
					"function": tc.Function,
				}
			}
		}

		messages = append(messages, models.Message{
			Role:      msg.Role,
			Content:   msg.Content,
			Name:      msg.Name,
			ToolCalls: toolCalls,
		})
	}

	// Set default ensemble config if not provided
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
		Prompt:    "", // System prompt should be in messages
		Messages:  messages,
		ModelParams: models.ModelParameters{
			Model:         req.Model,
			Temperature:   req.Temperature,
			MaxTokens:     req.MaxTokens,
			TopP:          req.TopP,
			StopSequences: req.Stop,
			ProviderSpecific: map[string]any{
				"presence_penalty":  req.PresencePenalty,
				"frequency_penalty": req.FrequencyPenalty,
				"logit_bias":        req.LogitBias,
				"user":              req.User,
				"force_provider":    req.ForceProvider,
			},
		},
		EnsembleConfig: ensembleConfig,
		MemoryEnhanced: true, // Enable memory for all requests
		Memory:         map[string]string{},
		Status:         "pending",
		CreatedAt:      time.Now(),
		RequestType:    "openai_chat",
	}
}

func (h *UnifiedHandler) processWithEnsemble(ctx context.Context, req *models.LLMRequest, openaiReq *OpenAIChatRequest) (*services.EnsembleResult, error) {
	// Check if provider registry is available
	if h.providerRegistry == nil {
		return nil, services.NewConfigurationError("provider registry not available", nil)
	}

	// Get ensemble service
	ensembleService := h.providerRegistry.GetEnsembleService()
	if ensembleService == nil {
		return nil, services.NewConfigurationError("ensemble service not available", nil)
	}

	// If specific provider requested, try to use it
	if openaiReq.ForceProvider != "" {
		provider, err := h.providerRegistry.GetProvider(openaiReq.ForceProvider)
		if err == nil {
			// Use single provider
			response, err := provider.Complete(ctx, req)
			if err != nil {
				// Categorize the provider error
				return nil, services.CategorizeError(err, openaiReq.ForceProvider)
			}

			// Create ensemble result with single provider
			return &services.EnsembleResult{
				Responses:    []*models.LLMResponse{response},
				Selected:     response,
				VotingMethod: "single_provider",
				Scores:       map[string]float64{response.ID: 1.0},
				Metadata: map[string]any{
					"forced_provider": openaiReq.ForceProvider,
					"total_providers": 1,
				},
			}, nil
		}
	}

	// Use ensemble for best results
	return ensembleService.RunEnsemble(ctx, req)
}

func (h *UnifiedHandler) processWithEnsembleStream(ctx context.Context, req *models.LLMRequest, openaiReq *OpenAIChatRequest) (<-chan *models.LLMResponse, error) {
	// Check if provider registry is available
	if h.providerRegistry == nil {
		return nil, services.NewConfigurationError("provider registry not available", nil)
	}

	// Get ensemble service
	ensembleService := h.providerRegistry.GetEnsembleService()
	if ensembleService == nil {
		return nil, services.NewConfigurationError("ensemble service not available", nil)
	}

	// For streaming, we'll use the first available provider
	// In a more sophisticated implementation, we could merge streams
	return ensembleService.RunEnsembleStream(ctx, req)
}

func (h *UnifiedHandler) convertToOpenAIChatResponse(result *services.EnsembleResult, req *OpenAIChatRequest) *OpenAIChatResponse {
	selected := result.Selected

	// Convert usage information
	usage := &OpenAIUsage{
		PromptTokens:     selected.TokensUsed / 2, // Estimate
		CompletionTokens: selected.TokensUsed / 2, // Estimate
		TotalTokens:      selected.TokensUsed,
	}

	// Create choice
	choice := OpenAIChoice{
		Index: 0,
		Message: OpenAIMessage{
			Role:    "assistant",
			Content: selected.Content,
		},
		FinishReason: selected.FinishReason,
	}

	return &OpenAIChatResponse{
		ID:                selected.ID,
		Object:            "chat.completion",
		Created:           selected.CreatedAt.Unix(),
		Model:             "superagent-ensemble", // Always show ensemble model
		Choices:           []OpenAIChoice{choice},
		Usage:             usage,
		SystemFingerprint: "fp_superagent_ensemble",
	}
}

// convertToOpenAIChatStreamResponse converts an LLM response to OpenAI streaming format
// isFirstChunk: true for the first chunk (includes role), false for subsequent chunks
// streamID: consistent ID across all chunks in the stream
func (h *UnifiedHandler) convertToOpenAIChatStreamResponse(resp *models.LLMResponse, req *OpenAIChatRequest, isFirstChunk bool, streamID string) map[string]any {
	// Build delta based on chunk type
	// Per OpenAI spec: role only in first chunk, content in subsequent chunks
	delta := map[string]any{}

	if isFirstChunk {
		// First chunk: include role, empty content
		delta["role"] = "assistant"
		delta["content"] = ""
	} else if resp.FinishReason != "" && resp.Content == "" {
		// Final chunk with NO content: empty delta (finish_reason indicates completion)
		// IMPORTANT: Only empty delta if there's no content to send
	} else if resp.FinishReason != "" && resp.Content != "" {
		// Final chunk WITH content: include both content AND finish_reason
		// CRITICAL FIX: Don't lose content when finish_reason is present!
		delta["content"] = resp.Content
	} else {
		// Content chunk: only include content (no role)
		delta["content"] = resp.Content
	}

	// Build choice with proper finish_reason
	// Critical for OpenCode/Crush/HelixCode compatibility
	choice := map[string]any{
		"index":    0,
		"delta":    delta,
		"logprobs": nil, // Required field per OpenAI spec
	}

	// finish_reason: null for intermediate, "stop" for final
	if resp.FinishReason != "" {
		choice["finish_reason"] = resp.FinishReason
	} else {
		choice["finish_reason"] = nil
	}

	// Use consistent stream ID across all chunks
	id := streamID
	if id == "" {
		id = resp.ID
	}

	return map[string]any{
		"id":                 id,
		"object":             "chat.completion.chunk",
		"created":            resp.CreatedAt.Unix(),
		"model":              "superagent-ensemble",
		"system_fingerprint": "fp_superagent_v1",
		"choices":            []map[string]any{choice},
	}
}

func (h *UnifiedHandler) sendOpenAIError(c *gin.Context, statusCode int, errorType, message, details string) {
	errorResp := map[string]any{
		"error": map[string]any{
			"message": fmt.Sprintf("%s: %s", message, details),
			"type":    errorType,
			"code":    statusCode,
		},
	}

	c.JSON(statusCode, errorResp)
}

// sendCategorizedError handles LLMServiceError with proper HTTP status codes
func (h *UnifiedHandler) sendCategorizedError(c *gin.Context, err error) {
	// Check if it's a categorized LLM service error
	if llmErr, ok := err.(*services.LLMServiceError); ok {
		response := llmErr.ToOpenAIError()

		// Add retry-after header if applicable
		if llmErr.RetryAfter > 0 {
			c.Header("Retry-After", fmt.Sprintf("%d", int(llmErr.RetryAfter.Seconds())))
		}

		c.JSON(llmErr.HTTPStatus, response)
		return
	}

	// Categorize unknown errors
	categorized := services.CategorizeError(err, "unknown")
	if categorized != nil {
		response := categorized.ToOpenAIError()
		if categorized.RetryAfter > 0 {
			c.Header("Retry-After", fmt.Sprintf("%d", int(categorized.RetryAfter.Seconds())))
		}
		c.JSON(categorized.HTTPStatus, response)
		return
	}

	// Fallback to generic 500 error
	h.sendOpenAIError(c, http.StatusInternalServerError, "internal_error", "An unexpected error occurred", err.Error())
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// generateID generates a random ID for OpenAI compatibility
func generateID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 29)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
