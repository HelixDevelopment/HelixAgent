package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/llm"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
)

// UnifiedHandler provides 100% OpenAI-compatible API with automatic ensemble support
type UnifiedHandler struct {
	providerRegistry  *services.ProviderRegistry
	config            *config.Config
	dialogueFormatter *services.DialogueFormatter
	debateTeamConfig  *services.DebateTeamConfig
	showDebateDialogue bool
}

// NewUnifiedHandler creates a new unified handler
func NewUnifiedHandler(registry *services.ProviderRegistry, cfg *config.Config) *UnifiedHandler {
	// Initialize dialogue formatter for AI debate presentation
	dialogueFormatter := services.NewDialogueFormatter(services.StyleTheater)

	// Create a default debate team config (can be replaced via SetDebateTeamConfig)
	// This ensures debate dialogue generation works even before full initialization
	var debateTeamConfig *services.DebateTeamConfig
	if registry != nil {
		debateTeamConfig = services.NewDebateTeamConfig(registry, nil, nil)
	}

	return &UnifiedHandler{
		providerRegistry:   registry,
		config:             cfg,
		dialogueFormatter:  dialogueFormatter,
		debateTeamConfig:   debateTeamConfig,
		showDebateDialogue: true, // Enable by default
	}
}

// SetDebateTeamConfig sets the debate team configuration and registers characters
// This should be called after the debate team has been properly initialized with discovery
func (h *UnifiedHandler) SetDebateTeamConfig(teamConfig *services.DebateTeamConfig) {
	h.debateTeamConfig = teamConfig

	// Register debate team members as dialogue characters
	if teamConfig != nil {
		members := teamConfig.GetAllLLMs()
		logrus.WithField("member_count", len(members)).Info("Registering debate team characters")
		for _, member := range members {
			if member != nil {
				char := h.dialogueFormatter.RegisterCharacter(member)
				logrus.WithFields(logrus.Fields{
					"position":  member.Position,
					"role":      member.Role,
					"provider":  member.ProviderName,
					"model":     member.ModelName,
					"char_name": char.Name,
				}).Info("Registered character for debate dialogue")
			}
		}
	}
}

// isToolResultProcessingTurn determines if the current request is specifically for processing
// tool results (continuation of tool execution) vs a new user request.
//
// CRITICAL: This function prevents infinite loops by distinguishing:
//   - Tool results from CURRENT turn → Process directly (return true)
//   - NEW user message → AI Debate (return false)
//
// The key insight: When the client sends back tool results, those results should
// be synthesized into a final response WITHOUT triggering a new debate cycle.
// Only genuinely NEW user messages should start a new debate.
func (h *UnifiedHandler) isToolResultProcessingTurn(messages []OpenAIMessage) bool {
	if len(messages) == 0 {
		return false
	}

	// Find the last non-system message to determine the conversation state
	var lastNonSystemIdx int = -1
	var lastUserIdx int = -1
	var lastToolIdx int = -1

	for i, msg := range messages {
		if msg.Role == "system" {
			continue
		}
		lastNonSystemIdx = i

		switch msg.Role {
		case "user":
			lastUserIdx = i
		case "tool":
			lastToolIdx = i
		}
	}

	// No non-system messages - not a tool result turn
	if lastNonSystemIdx == -1 {
		return false
	}

	// If the last message is from the user, this is a NEW user request → Use AI Debate
	if lastUserIdx == lastNonSystemIdx {
		logrus.WithFields(logrus.Fields{
			"last_user_idx":    lastUserIdx,
			"last_tool_idx":    lastToolIdx,
			"last_non_sys_idx": lastNonSystemIdx,
		}).Debug("New user request detected - will use AI Debate")
		return false
	}

	// If the last message is a TOOL result, this is tool processing → Direct synthesis
	// This PREVENTS infinite loops: debate generates tool_calls → tools execute →
	// results come back → synthesize response (NO new debate)
	if lastToolIdx == lastNonSystemIdx {
		logrus.WithFields(logrus.Fields{
			"last_tool_idx":    lastToolIdx,
			"last_non_sys_idx": lastNonSystemIdx,
		}).Debug("Tool result processing turn - will synthesize without new debate")
		return true // Process directly, don't start new debate
	}

	return false
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

// OpenAITool represents a tool definition in OpenAI format
// CRITICAL: This enables AI coding assistants (OpenCode, Claude Code, Qwen Code) to access codebases
type OpenAITool struct {
	Type     string           `json:"type"` // "function"
	Function OpenAIToolFunction `json:"function"`
}

// OpenAIToolFunction represents a function definition within a tool
type OpenAIToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Strict      *bool                  `json:"strict,omitempty"`
}

// OpenAIChatRequest represents OpenAI chat completion request
// IMPORTANT: Supports full OpenAI tool calling for AI coding assistants (OpenCode, Claude Code, Qwen Code)
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
	// OpenAI Tool Calling Support - CRITICAL for AI coding assistants to access codebase
	Tools      []OpenAITool `json:"tools,omitempty"`
	ToolChoice interface{}  `json:"tool_choice,omitempty"` // Can be "none", "auto", "required", or {"type":"function","function":{"name":"..."}}
	// Parallel tool calls support
	ParallelToolCalls *bool `json:"parallel_tool_calls,omitempty"`
	// HelixAgent extensions
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

// StreamingToolCall represents a tool call for streaming responses with Index
// CRITICAL: This enables actual tool execution by AI coding assistants like OpenCode
type StreamingToolCall struct {
	Index    int                `json:"index"`
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

	// Check if this is a tool result processing turn vs a new user request
	// CRITICAL: Tool results must be processed directly to prevent infinite loops
	// Only NEW user messages should trigger a fresh AI Debate cycle
	isToolResultTurn := h.isToolResultProcessingTurn(req.Messages)

	if isToolResultTurn {
		// Process tool results directly - synthesize into final response
		// This prevents: debate → tool_calls → results → debate → tool_calls... (infinite loop)
		logrus.Info("Processing tool results - synthesizing without new debate")
		toolResultResponse, err := h.processToolResultsWithLLM(c.Request.Context(), &req)
		if err != nil {
			logrus.WithError(err).Warn("Failed to process tool results with LLM")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"message": "Failed to process tool results",
					"type":    "server_error",
					"code":    "TOOL_RESULT_PROCESSING_FAILED",
				},
			})
			return
		}

		// Return as OpenAI-compatible response
		response := OpenAIChatResponse{
			ID:                "chatcmpl-tool-" + fmt.Sprintf("%d", time.Now().UnixNano()),
			Object:            "chat.completion",
			Created:           time.Now().Unix(),
			Model:             "helixagent-ensemble",
			SystemFingerprint: "fp_helixagent_v1",
			Choices: []OpenAIChoice{
				{
					Index: 0,
					Message: OpenAIMessage{
						Role:    "assistant",
						Content: toolResultResponse,
					},
					FinishReason: "stop",
				},
			},
			Usage: &OpenAIUsage{
				PromptTokens:     len(toolResultResponse) / 4,
				CompletionTokens: len(toolResultResponse) / 4,
				TotalTokens:      len(toolResultResponse) / 2,
			},
		}
		c.JSON(http.StatusOK, response)
		return
	}

	// NEW user request → Full AI Debate ensemble
	logrus.Info("New user request - initiating AI Debate")

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
	// Maximum 420 seconds (7 min) to allow for multiple provider fallbacks during tool result processing
	// With 6 providers at 60 seconds each, we need at least 360 seconds plus buffer
	ctx, cancel := context.WithTimeout(c.Request.Context(), 420*time.Second)
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

	// Check if this is a tool result processing turn vs a new user request
	// CRITICAL: Tool results must be processed directly to prevent infinite loops
	isToolResultTurn := h.isToolResultProcessingTurn(req.Messages)

	if isToolResultTurn {
		// Process tool results directly - synthesize into final response
		// This prevents: debate → tool_calls → results → debate → tool_calls... (infinite loop)
		logrus.Info("Streaming tool results - synthesizing without new debate")
		toolResultResponse, err := h.processToolResultsWithLLM(ctx, req)
		if err != nil {
			logrus.WithError(err).Warn("Failed to process tool results with LLM, using fallback response")
			fallbackResponse := h.generateFallbackToolResultsResponse(req)
			if fallbackResponse != "" {
				toolResultResponse = fallbackResponse
				err = nil
			} else {
				errContent := fmt.Sprintf("I encountered an issue processing the tool results: %v", err)
				errChunk := map[string]any{
					"id":                 streamID,
					"object":             "chat.completion.chunk",
					"created":            time.Now().Unix(),
					"model":              "helixagent-ensemble",
					"system_fingerprint": "fp_helixagent_v1",
					"choices": []map[string]any{
						{
							"index":         0,
							"delta":         map[string]any{"content": errContent},
							"logprobs":      nil,
							"finish_reason": nil,
						},
					},
				}
				if errData, err := json.Marshal(errChunk); err == nil {
					c.Writer.Write([]byte("data: "))
					c.Writer.Write(errData)
					c.Writer.Write([]byte("\n\n"))
					flusher.Flush()
				}
			}
		}
		if err == nil && toolResultResponse != "" {
			responseChunk := map[string]any{
				"id":                 streamID,
				"object":             "chat.completion.chunk",
				"created":            time.Now().Unix(),
				"model":              "helixagent-ensemble",
				"system_fingerprint": "fp_helixagent_v1",
				"choices": []map[string]any{
					{
						"index":         0,
						"delta":         map[string]any{"content": toolResultResponse},
						"logprobs":      nil,
						"finish_reason": nil,
					},
				},
			}
			if respData, err := json.Marshal(responseChunk); err == nil {
				c.Writer.Write([]byte("data: "))
				c.Writer.Write(respData)
				c.Writer.Write([]byte("\n\n"))
				flusher.Flush()
			}
		}

		// Send finish chunk and end
		finishChunk := map[string]any{
			"id":                 streamID,
			"object":             "chat.completion.chunk",
			"created":            time.Now().Unix(),
			"model":              "helixagent-ensemble",
			"system_fingerprint": "fp_helixagent_v1",
			"choices": []map[string]any{
				{
					"index":         0,
					"delta":         map[string]any{},
					"logprobs":      nil,
					"finish_reason": "stop",
				},
			},
		}
		if finishData, err := json.Marshal(finishChunk); err == nil {
			c.Writer.Write([]byte("data: "))
			c.Writer.Write(finishData)
			c.Writer.Write([]byte("\n\n"))
			flusher.Flush()
		}

		c.Writer.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
		return
	}

	// NEW user request → Full AI Debate ensemble
	logrus.Info("New streaming request - initiating AI Debate")

	// Stream AI Debate dialogue introduction before the actual response
	if h.showDebateDialogue && h.dialogueFormatter != nil && h.debateTeamConfig != nil {
		// Extract topic from the last user message
		topic := "User Query"
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				topic = req.Messages[i].Content
				break
			}
		}

		// Generate and stream debate dialogue introduction
		dialogueIntro := h.generateDebateDialogueIntroduction(topic)
		if dialogueIntro != "" {
			// Stream introduction in chunks for better rendering
			for _, line := range strings.Split(dialogueIntro, "\n") {
				if line == "" {
					line = "\n"
				} else {
					line = line + "\n"
				}
				introChunk := map[string]any{
					"id":                 streamID,
					"object":             "chat.completion.chunk",
					"created":            time.Now().Unix(),
					"model":              "helixagent-ensemble",
					"system_fingerprint": "fp_helixagent_v1",
					"choices": []map[string]any{
						{
							"index":         0,
							"delta":         map[string]any{"content": line},
							"logprobs":      nil,
							"finish_reason": nil,
						},
					},
				}
				if introData, err := json.Marshal(introChunk); err == nil {
					c.Writer.Write([]byte("data: "))
					c.Writer.Write(introData)
					c.Writer.Write([]byte("\n\n"))
					flusher.Flush()
				}
				// Small delay for visual effect
				time.Sleep(5 * time.Millisecond)
			}
			chunksSent++
		}

		// Stream REAL debate responses for each position (actual LLM calls)
		positions := []services.DebateTeamPosition{
			services.PositionAnalyst,
			services.PositionProposer,
			services.PositionCritic,
			services.PositionSynthesis,
			services.PositionMediator,
		}
		previousResponses := make(map[services.DebateTeamPosition]string)

		for _, pos := range positions {
			// First, stream the header (character name and role)
			debateHeader := h.generateDebateDialogueResponse(pos, topic)
			if debateHeader != "" {
				headerChunk := map[string]any{
					"id":                 streamID,
					"object":             "chat.completion.chunk",
					"created":            time.Now().Unix(),
					"model":              "helixagent-ensemble",
					"system_fingerprint": "fp_helixagent_v1",
					"choices": []map[string]any{
						{
							"index":         0,
							"delta":         map[string]any{"content": debateHeader},
							"logprobs":      nil,
							"finish_reason": nil,
						},
					},
				}
				if headerData, err := json.Marshal(headerChunk); err == nil {
					c.Writer.Write([]byte("data: "))
					c.Writer.Write(headerData)
					c.Writer.Write([]byte("\n\n"))
					flusher.Flush()
				}
			}

			// Now get the REAL response from the LLM for this position
			// CRITICAL: Pass tools so LLM knows about available coding assistant capabilities
			realResponse, err := h.generateRealDebateResponse(ctx, pos, topic, previousResponses, req.Tools)
			if err != nil {
				// Log error but continue with fallback message
				logrus.WithError(err).WithField("position", pos).Warn("Failed to get real debate response, using fallback")
				realResponse = "Unable to provide analysis at this time."
			}

			// Store for context in subsequent positions
			previousResponses[pos] = realResponse

			// Stream the actual LLM response
			responseContent := realResponse + "\"\n"
			responseChunk := map[string]any{
				"id":                 streamID,
				"object":             "chat.completion.chunk",
				"created":            time.Now().Unix(),
				"model":              "helixagent-ensemble",
				"system_fingerprint": "fp_helixagent_v1",
				"choices": []map[string]any{
					{
						"index":         0,
						"delta":         map[string]any{"content": responseContent},
						"logprobs":      nil,
						"finish_reason": nil,
					},
				},
			}
			if responseData, err := json.Marshal(responseChunk); err == nil {
				c.Writer.Write([]byte("data: "))
				c.Writer.Write(responseData)
				c.Writer.Write([]byte("\n\n"))
				flusher.Flush()
			}
			chunksSent++
		}

		// Stream conclusion
		conclusion := h.generateDebateDialogueConclusion()
		if conclusion != "" {
			conclusionChunk := map[string]any{
				"id":                 streamID,
				"object":             "chat.completion.chunk",
				"created":            time.Now().Unix(),
				"model":              "helixagent-ensemble",
				"system_fingerprint": "fp_helixagent_v1",
				"choices": []map[string]any{
					{
						"index":         0,
						"delta":         map[string]any{"content": conclusion},
						"logprobs":      nil,
						"finish_reason": nil,
					},
				},
			}
			if conclusionData, err := json.Marshal(conclusionChunk); err == nil {
				c.Writer.Write([]byte("data: "))
				c.Writer.Write(conclusionData)
				c.Writer.Write([]byte("\n\n"))
				flusher.Flush()
			}
		}

		// CRITICAL FIX: Generate FINAL SYNTHESIS based on all debate responses
		// This is the actual "consensus" that combines all 5 perspectives
		// IMPORTANT: Pass tools so synthesis knows about available coding assistant capabilities
		synthesisResponse, synthesisErr := h.generateFinalSynthesis(ctx, topic, previousResponses, req.Tools)
		if synthesisErr != nil {
			logrus.WithError(synthesisErr).Warn("Failed to generate final synthesis")
			synthesisResponse = "Based on the debate, a consensus could not be reached. Please consider the individual perspectives above."
		}

		// Stream the synthesis response as the consensus content
		if synthesisResponse != "" {
			// Stream the synthesis in chunks for better rendering
			synthesisChunk := map[string]any{
				"id":                 streamID,
				"object":             "chat.completion.chunk",
				"created":            time.Now().Unix(),
				"model":              "helixagent-ensemble",
				"system_fingerprint": "fp_helixagent_v1",
				"choices": []map[string]any{
					{
						"index":         0,
						"delta":         map[string]any{"content": synthesisResponse + "\n"},
						"logprobs":      nil,
						"finish_reason": nil,
					},
				},
			}
			if synthesisData, err := json.Marshal(synthesisChunk); err == nil {
				c.Writer.Write([]byte("data: "))
				c.Writer.Write(synthesisData)
				c.Writer.Write([]byte("\n\n"))
				flusher.Flush()
			}
			chunksSent++
		}

		// ACTION PHASE: If tools are available, make one final call that can return actual tool_calls
		// This allows the AI coding assistant to actually USE the tools, not just talk about them
		if len(req.Tools) > 0 {
			actionToolCalls := h.generateActionToolCalls(ctx, topic, synthesisResponse, req.Tools, previousResponses)
			if len(actionToolCalls) > 0 {
				// Stream the tool calls to the client
				for _, toolCall := range actionToolCalls {
					toolCallChunk := map[string]any{
						"id":                 streamID,
						"object":             "chat.completion.chunk",
						"created":            time.Now().Unix(),
						"model":              "helixagent-ensemble",
						"system_fingerprint": "fp_helixagent_v1",
						"choices": []map[string]any{
							{
								"index": 0,
								"delta": map[string]any{
									"tool_calls": []map[string]any{
										{
											"index": toolCall.Index,
											"id":    toolCall.ID,
											"type":  "function",
											"function": map[string]any{
												"name":      toolCall.Function.Name,
												"arguments": toolCall.Function.Arguments,
											},
										},
									},
								},
								"logprobs":      nil,
								"finish_reason": nil,
							},
						},
					}
					if toolCallData, err := json.Marshal(toolCallChunk); err == nil {
						c.Writer.Write([]byte("data: "))
						c.Writer.Write(toolCallData)
						c.Writer.Write([]byte("\n\n"))
						flusher.Flush()
					}
				}
				chunksSent++

				// Send finish_reason: tool_calls
				finishChunk := map[string]any{
					"id":                 streamID,
					"object":             "chat.completion.chunk",
					"created":            time.Now().Unix(),
					"model":              "helixagent-ensemble",
					"system_fingerprint": "fp_helixagent_v1",
					"choices": []map[string]any{
						{
							"index":         0,
							"delta":         map[string]any{},
							"logprobs":      nil,
							"finish_reason": "tool_calls",
						},
					},
				}
				if finishData, err := json.Marshal(finishChunk); err == nil {
					c.Writer.Write([]byte("data: "))
					c.Writer.Write(finishData)
					c.Writer.Write([]byte("\n\n"))
					flusher.Flush()
				}
				sentFinalChunk = true

				// CRITICAL: After sending tool_calls, immediately end the response
				// The client will execute the tools and send another request with results
				// Do NOT send any more content or footer - it confuses the tool calling protocol
				c.Writer.Write([]byte("data: [DONE]\n\n"))
				flusher.Flush()
				return
			}
		}
	}

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

	// Send response footer before final chunk (if debate dialogue was shown)
	if h.showDebateDialogue {
		footer := h.generateResponseFooter()
		if footer != "" {
			footerChunk := map[string]any{
				"id":                 streamID,
				"object":             "chat.completion.chunk",
				"created":            time.Now().Unix(),
				"model":              "helixagent-ensemble",
				"system_fingerprint": "fp_helixagent_v1",
				"choices": []map[string]any{
					{
						"index":         0,
						"delta":         map[string]any{"content": footer},
						"logprobs":      nil,
						"finish_reason": nil,
					},
				},
			}
			if footerData, err := json.Marshal(footerChunk); err == nil {
				c.Writer.Write([]byte("data: "))
				c.Writer.Write(footerData)
				c.Writer.Write([]byte("\n\n"))
				flusher.Flush()
			}
		}
	}

	// Only send final chunk if we haven't already sent one with finish_reason
	if !sentFinalChunk {
		finalChunk := map[string]any{
			"id":                 streamID, // Same ID as all other chunks
			"object":             "chat.completion.chunk",
			"created":            time.Now().Unix(),
			"model":              "helixagent-ensemble",
			"system_fingerprint": "fp_helixagent_v1",
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
	// Maximum 420 seconds (7 min) to allow for multiple provider fallbacks during tool result processing
	// With 6 providers at 60 seconds each, we need at least 360 seconds plus buffer
	ctx, cancel := context.WithTimeout(c.Request.Context(), 420*time.Second)
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
			"model":              "helixagent-ensemble",
			"system_fingerprint": "fp_helixagent_v1",
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

// Models returns only the HelixAgent virtual model
// HelixAgent exposes a single unified model that internally uses AI debate ensemble
// Backend provider models are implementation details and not exposed to clients
func (h *UnifiedHandler) Models(c *gin.Context) {
	// HelixAgent exposes only ONE virtual model: helixagent-debate
	// This model internally uses AI debate ensemble with multiple LLMs
	helixagentModel := OpenAIModel{
		ID:      "helixagent-debate",
		Object:  "model",
		Created: time.Now().Unix(),
		OwnedBy: "helixagent",
		Permission: []OpenAIModelPermission{
			{
				ID:                 "helixagent-debate-permission",
				Object:             "model_permission",
				Created:            time.Now().Unix(),
				AllowCreateEngine:  true,
				AllowSampling:      true,
				AllowLogprobs:      true,
				AllowSearchIndices: true,
				AllowView:          true,
				AllowFineTuning:    false,
				Organization:       "helixagent",
				IsBlocking:         false,
			},
		},
		Root:   "helixagent-debate",
		Parent: nil,
	}

	response := OpenAIModelsResponse{
		Object: "list",
		Data:   []OpenAIModel{helixagentModel},
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

	// Convert tools to provider-specific format
	// CRITICAL: This enables tool calling for AI coding assistants
	var toolsData interface{}
	if len(req.Tools) > 0 {
		toolsData = req.Tools
		logrus.WithField("tool_count", len(req.Tools)).Debug("Passing tools to LLM request")
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
				"presence_penalty":    req.PresencePenalty,
				"frequency_penalty":   req.FrequencyPenalty,
				"logit_bias":          req.LogitBias,
				"user":                req.User,
				"force_provider":      req.ForceProvider,
				"tools":               toolsData,
				"tool_choice":         req.ToolChoice,
				"parallel_tool_calls": req.ParallelToolCalls,
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
		Model:             "helixagent-ensemble", // Always show ensemble model
		Choices:           []OpenAIChoice{choice},
		Usage:             usage,
		SystemFingerprint: "fp_helixagent_ensemble",
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
		"model":              "helixagent-ensemble",
		"system_fingerprint": "fp_helixagent_v1",
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

// generateDebateDialogueIntroduction creates the AI debate team conversation introduction
// This is displayed before the final response to show how the AI debate ensemble works
func (h *UnifiedHandler) generateDebateDialogueIntroduction(topic string) string {
	if h.dialogueFormatter == nil || h.debateTeamConfig == nil {
		return ""
	}

	var sb strings.Builder

	// Header
	sb.WriteString("\n")
	sb.WriteString("╔══════════════════════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                      🎭 HELIXAGENT AI DEBATE ENSEMBLE 🎭                      ║\n")
	sb.WriteString("╠══════════════════════════════════════════════════════════════════════════════╣\n")
	sb.WriteString("║  Five AI minds deliberate to synthesize the optimal response.                ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════════════════════════╝\n\n")

	// Topic
	sb.WriteString("📋 TOPIC: ")
	if len(topic) > 70 {
		sb.WriteString(topic[:70])
		sb.WriteString("...")
	} else {
		sb.WriteString(topic)
	}
	sb.WriteString("\n\n")

	// Cast of characters
	sb.WriteString("═══════════════════════════════════════════════════════════════════════════════\n")
	sb.WriteString("                              DRAMATIS PERSONAE\n")
	sb.WriteString("═══════════════════════════════════════════════════════════════════════════════\n\n")

	members := h.debateTeamConfig.GetAllLLMs()
	for _, member := range members {
		if member == nil {
			continue
		}
		char := h.dialogueFormatter.GetCharacter(member.Position)
		if char != nil {
			sb.WriteString(fmt.Sprintf("  %s  %-15s │ %s (%s)\n",
				char.Avatar,
				char.Name,
				member.ModelName,
				member.ProviderName))
		}
	}

	sb.WriteString("\n═══════════════════════════════════════════════════════════════════════════════\n")
	sb.WriteString("                               THE DELIBERATION\n")
	sb.WriteString("═══════════════════════════════════════════════════════════════════════════════\n\n")

	return sb.String()
}

// generateDebateDialogueResponse creates a debate response header for a position
// Note: This only generates the header. Use generateRealDebateResponse for actual LLM calls.
func (h *UnifiedHandler) generateDebateDialogueResponse(position services.DebateTeamPosition, topic string) string {
	if h.dialogueFormatter == nil {
		return ""
	}

	char := h.dialogueFormatter.GetCharacter(position)
	if char == nil {
		return ""
	}

	// Generate just the header - actual content comes from generateRealDebateResponse
	return fmt.Sprintf("\n%s %s:\n    \"", char.Avatar, char.Name)
}

// generateRealDebateResponse calls the actual LLM for a position and returns the real response
// It tries the primary provider first, then automatically falls back to fallback providers if primary fails
// The tools parameter is passed to inform the LLM about available coding assistant capabilities
func (h *UnifiedHandler) generateRealDebateResponse(ctx context.Context, position services.DebateTeamPosition, topic string, previousResponses map[services.DebateTeamPosition]string, tools []OpenAITool) (string, error) {
	if h.providerRegistry == nil || h.debateTeamConfig == nil {
		return "", fmt.Errorf("provider registry or debate team config not available")
	}

	// Get the team member assigned to this position
	member := h.debateTeamConfig.GetTeamMember(position)
	if member == nil {
		return "", fmt.Errorf("no LLM assigned to position %d", position)
	}

	// Build a role-specific system prompt with tool awareness (same for all attempts)
	// CRITICAL: Pass tools so LLM knows about available coding assistant capabilities
	systemPrompt := h.buildDebateRoleSystemPromptWithTools(position, member.Role, tools)

	// Build context from previous responses
	contextStr := ""
	if len(previousResponses) > 0 {
		contextStr = "\n\nPrevious contributions from other team members:\n"
		positionOrder := []services.DebateTeamPosition{
			services.PositionAnalyst,
			services.PositionProposer,
			services.PositionCritic,
			services.PositionSynthesis,
			services.PositionMediator,
		}
		for _, pos := range positionOrder {
			if resp, ok := previousResponses[pos]; ok && resp != "" {
				char := h.dialogueFormatter.GetCharacter(pos)
				if char != nil {
					contextStr += fmt.Sprintf("- %s: %s\n", char.Name, resp)
				}
			}
		}
	}

	// Create the base user prompt
	userPrompt := fmt.Sprintf("Topic: %s%s\n\nProvide your analysis in 2-3 sentences, focused on your role.", topic, contextStr)

	// Try the primary member and all fallbacks in the chain
	currentMember := member
	attemptNum := 0
	maxAttempts := 5 // Primary + up to 4 fallbacks
	var lastErr error

	for currentMember != nil && attemptNum < maxAttempts {
		attemptNum++

		// Get the provider for this member
		provider, providerErr := h.getProviderForMember(currentMember)
		if providerErr != nil {
			logrus.WithFields(logrus.Fields{
				"position": position,
				"provider": currentMember.ProviderName,
				"model":    currentMember.ModelName,
				"attempt":  attemptNum,
				"is_oauth": currentMember.IsOAuth,
			}).WithError(providerErr).Warn("Failed to get provider, trying fallback")
			lastErr = providerErr
			currentMember = currentMember.Fallback
			continue
		}

		// Create the request with this member's model
		llmReq := &models.LLMRequest{
			ID:        fmt.Sprintf("debate-%d-%d-%d", position, attemptNum, time.Now().UnixNano()),
			SessionID: "debate-session",
			Prompt:    userPrompt,
			Messages: []models.Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
			ModelParams: models.ModelParameters{
				Model:       currentMember.ModelName,
				Temperature: 0.7,
				MaxTokens:   256, // Keep responses concise for debate
			},
		}

		// Call the LLM with a timeout
		llmCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		resp, err := provider.Complete(llmCtx, llmReq)
		cancel()

		if err != nil {
			logrus.WithFields(logrus.Fields{
				"position": position,
				"provider": currentMember.ProviderName,
				"model":    currentMember.ModelName,
				"attempt":  attemptNum,
				"is_oauth": currentMember.IsOAuth,
			}).WithError(err).Warn("LLM call failed, trying fallback")
			lastErr = err
			currentMember = currentMember.Fallback
			continue
		}

		// Success! Clean up the response
		content := strings.TrimSpace(resp.Content)
		content = strings.Trim(content, "\"")

		if attemptNum > 1 {
			logrus.WithFields(logrus.Fields{
				"position": position,
				"provider": currentMember.ProviderName,
				"model":    currentMember.ModelName,
				"attempt":  attemptNum,
			}).Info("Debate response succeeded with fallback provider")
		}

		return content, nil
	}

	// All attempts failed
	return "", fmt.Errorf("all providers failed for position %d after %d attempts, last error: %w", position, attemptNum, lastErr)
}

// getProviderForMember retrieves the LLM provider for a debate team member
func (h *UnifiedHandler) getProviderForMember(member *services.DebateTeamMember) (llm.LLMProvider, error) {
	if member.Provider != nil {
		// Use the embedded provider (from DebateTeamMember)
		return member.Provider, nil
	}

	// Fall back to registry lookup
	regProvider, regErr := h.providerRegistry.GetProvider(member.ProviderName)
	if regErr != nil {
		return nil, fmt.Errorf("provider %s not found: %w", member.ProviderName, regErr)
	}

	// Convert services.LLMProvider to llm.LLMProvider
	if llmProv, ok := regProvider.(llm.LLMProvider); ok {
		return llmProv, nil
	}

	return nil, fmt.Errorf("provider %s does not implement llm.LLMProvider", member.ProviderName)
}

// buildDebateRoleSystemPrompt creates a system prompt based on the debate role
// IMPORTANT: This prompt includes context about the AI coding assistant capabilities
// so debate positions understand they have access to the user's codebase through tools
func (h *UnifiedHandler) buildDebateRoleSystemPrompt(position services.DebateTeamPosition, role services.DebateRole) string {
	return h.buildDebateRoleSystemPromptWithTools(position, role, nil)
}

// buildDebateRoleSystemPromptWithTools creates a system prompt with specific tool information
// This is CRITICAL for AI coding assistants - the LLM needs to know what tools are available
func (h *UnifiedHandler) buildDebateRoleSystemPromptWithTools(position services.DebateTeamPosition, role services.DebateRole, tools []OpenAITool) string {
	// Build tool description if tools are available
	toolDescription := ""
	if len(tools) > 0 {
		toolNames := make([]string, 0, len(tools))
		for _, tool := range tools {
			if tool.Function.Name != "" {
				toolNames = append(toolNames, tool.Function.Name)
			}
		}
		if len(toolNames) > 0 {
			toolDescription = fmt.Sprintf(`
AVAILABLE TOOLS (these are available through the coding assistant interface):
- %s

The user's AI coding assistant (Claude Code, OpenCode, Qwen Code, etc.) has access to these tools.
When the user asks about their codebase, YOU CAN AND SHOULD provide specific guidance because
the tools above allow reading files, searching code, editing files, and running commands.
`, strings.Join(toolNames, "\n- "))
		}
	}

	// Context about being part of an AI coding assistant with tool access
	contextPrompt := fmt.Sprintf(`You are part of HelixAgent, an AI coding assistant that provides responses through an AI Debate Ensemble.

IMPORTANT CONTEXT:
- You are integrated with AI coding tools like Claude Code, OpenCode, and Qwen Code
- The user's coding assistant HAS FULL ACCESS to their codebase through tools
- The tools can: Read files, Write/Edit files, Search code (grep), List files (glob), Execute shell commands
- When the user asks about their code, the assistant CAN see and access their files
- NEVER say "I cannot see your codebase" - the tools handle file access
- Provide SPECIFIC, ACTIONABLE coding advice as if you can see their code
- If asked about specific files or code, give concrete guidance%s

You are participating in an AI debate ensemble. Provide a concise, focused response in 2-3 sentences. `, toolDescription)

	switch role {
	case services.RoleAnalyst:
		return contextPrompt + "Your role is THE ANALYST: Break down the problem systematically, identify key factors, and provide data-driven insights. For coding questions, analyze code structure, patterns, and potential issues."
	case services.RoleProposer:
		return contextPrompt + "Your role is THE PROPOSER: Present actionable solutions and approaches, focusing on practical implementation. For coding questions, suggest specific code changes or approaches."
	case services.RoleCritic:
		return contextPrompt + "Your role is THE CRITIC: Challenge assumptions, identify potential issues, edge cases, and weaknesses in proposed approaches. For coding questions, identify potential bugs, security issues, or performance concerns."
	case services.RoleSynthesis:
		return contextPrompt + "Your role is THE SYNTHESIZER: Combine insights from different perspectives into a cohesive understanding. For coding questions, synthesize the best approach from all perspectives."
	case services.RoleMediator:
		return contextPrompt + "Your role is THE MEDIATOR: Balance different viewpoints and guide toward consensus. For coding questions, recommend the most balanced and practical solution."
	default:
		return contextPrompt + "Contribute your perspective on the topic."
	}
}

// generateDebateDialogueConclusion creates the conclusion section after debate
func (h *UnifiedHandler) generateDebateDialogueConclusion() string {
	var sb strings.Builder

	sb.WriteString("\n═══════════════════════════════════════════════════════════════════════════════\n")
	sb.WriteString("                              📜 CONSENSUS REACHED 📜\n")
	sb.WriteString("═══════════════════════════════════════════════════════════════════════════════\n")
	sb.WriteString("  The AI Debate Ensemble has synthesized the following response:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────────────────────\n\n")

	return sb.String()
}

// generateFinalSynthesis creates the final synthesized response based on all debate contributions
// This is the CRITICAL function that produces the actual CONSENSUS content
// IMPORTANT: tools parameter informs the synthesis about available coding assistant capabilities
func (h *UnifiedHandler) generateFinalSynthesis(ctx context.Context, topic string, debateResponses map[services.DebateTeamPosition]string, tools []OpenAITool) (string, error) {
	if h.providerRegistry == nil || h.debateTeamConfig == nil {
		return "", fmt.Errorf("provider registry or debate team config not available")
	}

	// Build tool description if tools are available
	toolDescription := ""
	if len(tools) > 0 {
		toolNames := make([]string, 0, len(tools))
		for _, tool := range tools {
			if tool.Function.Name != "" {
				toolNames = append(toolNames, tool.Function.Name)
			}
		}
		if len(toolNames) > 0 {
			toolDescription = fmt.Sprintf(`
AVAILABLE TOOLS (the user's coding assistant has access to these):
- %s
`, strings.Join(toolNames, "\n- "))
		}
	}

	// Build synthesis prompt that includes all debate contributions
	// IMPORTANT: Include context about AI coding assistant capabilities
	var promptBuilder strings.Builder
	promptBuilder.WriteString(fmt.Sprintf(`You are part of HelixAgent, an AI coding assistant that provides responses through an AI Debate Ensemble.

IMPORTANT CONTEXT:
- You are integrated with AI coding tools like Claude Code, OpenCode, and Qwen Code
- The user's coding assistant HAS FULL ACCESS to their codebase through tools
- Tools available: Read files, Write/Edit files, Search code (grep), List files (glob), Execute shell commands
- When the user asks about their code, the assistant CAN see and access their files
- NEVER say "I cannot see your codebase" - the tools handle file access
- Provide SPECIFIC, ACTIONABLE coding advice%s

You are tasked with synthesizing a final consensus response based on a multi-perspective AI debate.

`, toolDescription))
	promptBuilder.WriteString("ORIGINAL TOPIC/QUESTION:\n")
	promptBuilder.WriteString(topic)
	promptBuilder.WriteString("\n\nDEBATE CONTRIBUTIONS:\n\n")

	// Add each position's contribution
	positionNames := map[services.DebateTeamPosition]string{
		services.PositionAnalyst:   "THE ANALYST",
		services.PositionProposer:  "THE PROPOSER",
		services.PositionCritic:    "THE CRITIC",
		services.PositionSynthesis: "THE SYNTHESIZER",
		services.PositionMediator:  "THE MEDIATOR",
	}

	positionOrder := []services.DebateTeamPosition{
		services.PositionAnalyst,
		services.PositionProposer,
		services.PositionCritic,
		services.PositionSynthesis,
		services.PositionMediator,
	}

	for _, pos := range positionOrder {
		if response, ok := debateResponses[pos]; ok && response != "" {
			promptBuilder.WriteString(fmt.Sprintf("%s:\n%s\n\n", positionNames[pos], response))
		}
	}

	promptBuilder.WriteString("TASK:\n")
	promptBuilder.WriteString("Based on the above debate, provide a FINAL SYNTHESIZED RESPONSE that:\n")
	promptBuilder.WriteString("1. Directly answers the original topic/question\n")
	promptBuilder.WriteString("2. Incorporates the strongest insights from each perspective\n")
	promptBuilder.WriteString("3. Addresses concerns raised by the critic where valid\n")
	promptBuilder.WriteString("4. Provides a clear, actionable, and balanced conclusion\n")
	promptBuilder.WriteString("5. For coding questions, provide specific, actionable guidance the user can apply\n\n")
	promptBuilder.WriteString("Your response should be comprehensive yet concise (2-4 paragraphs). Focus on providing genuine value to the user. Remember: the user has access to their codebase through integrated tools - provide practical advice they can immediately apply.")

	synthesisPrompt := promptBuilder.String()

	// Get the Mediator's provider for synthesis (it's designed for consensus-building)
	mediatorMember := h.debateTeamConfig.GetTeamMember(services.PositionMediator)
	if mediatorMember == nil {
		// Fallback to any available provider
		mediatorMember = h.debateTeamConfig.GetTeamMember(services.PositionSynthesis)
	}
	if mediatorMember == nil {
		return "", fmt.Errorf("no suitable provider found for synthesis")
	}

	// Get provider using the same fallback mechanism as debate responses
	provider, err := h.getProviderForMember(mediatorMember)
	if err != nil {
		// Try fallbacks
		currentMember := mediatorMember.Fallback
		for currentMember != nil {
			provider, err = h.getProviderForMember(currentMember)
			if err == nil {
				break
			}
			currentMember = currentMember.Fallback
		}
		if provider == nil {
			return "", fmt.Errorf("all providers failed for synthesis: %w", err)
		}
	}

	// Create the synthesis request
	synthesisReq := &models.LLMRequest{
		Messages: []models.Message{
			{
				Role:    "system",
				Content: "You are a skilled synthesizer who combines multiple perspectives into clear, actionable conclusions.",
			},
			{
				Role:    "user",
				Content: synthesisPrompt,
			},
		},
		ModelParams: models.ModelParameters{
			MaxTokens:   1024,
			Temperature: 0.7,
		},
	}

	// Call the LLM with timeout
	synthesisCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := provider.Complete(synthesisCtx, synthesisReq)
	if err != nil {
		logrus.WithError(err).WithField("provider", mediatorMember.ProviderName).Warn("Synthesis provider failed, trying fallback")

		// Try fallback providers
		currentMember := mediatorMember.Fallback
		for currentMember != nil && err != nil {
			provider, provErr := h.getProviderForMember(currentMember)
			if provErr != nil {
				currentMember = currentMember.Fallback
				continue
			}

			resp, err = provider.Complete(synthesisCtx, synthesisReq)
			if err == nil {
				logrus.WithField("provider", currentMember.ProviderName).Info("Synthesis succeeded with fallback provider")
				break
			}

			logrus.WithError(err).WithField("provider", currentMember.ProviderName).Warn("Fallback synthesis provider also failed")
			currentMember = currentMember.Fallback
		}

		if err != nil {
			return "", fmt.Errorf("all synthesis providers failed: %w", err)
		}
	}

	return strings.TrimSpace(resp.Content), nil
}

// generateResponseFooter creates the finalization footer for every response
func (h *UnifiedHandler) generateResponseFooter() string {
	var sb strings.Builder

	sb.WriteString("\n\n")
	sb.WriteString("───────────────────────────────────────────────────────────────────────────────\n")
	sb.WriteString("                     ✨ Powered by HelixAgent AI Debate Ensemble ✨\n")
	sb.WriteString("                  Synthesized from 5 AI perspectives for optimal results\n")
	sb.WriteString("───────────────────────────────────────────────────────────────────────────────\n")

	return sb.String()
}

// generateActionToolCalls analyzes the debate synthesis and generates actual tool calls
// CRITICAL: This function enables AI coding assistants to execute tools, not just talk about them
// It analyzes the topic/question and synthesis to determine what tools should be called
func (h *UnifiedHandler) generateActionToolCalls(ctx context.Context, topic string, synthesis string, tools []OpenAITool, previousResponses map[services.DebateTeamPosition]string) []StreamingToolCall {
	if len(tools) == 0 {
		return nil
	}

	// Build a map of available tools by name
	availableTools := make(map[string]OpenAITool)
	for _, tool := range tools {
		if tool.Type == "function" {
			availableTools[tool.Function.Name] = tool
		}
	}

	var toolCalls []StreamingToolCall
	topicLower := strings.ToLower(topic)
	synthesisLower := strings.ToLower(synthesis)

	// Determine what tools to call based on topic and synthesis analysis
	// This enables the AI to actually take action, not just discuss what it could do

	// Case 1: Questions about seeing/accessing the codebase - use glob or read
	if containsAny(topicLower, []string{"see my codebase", "access the codebase", "view the codebase", "look at my code", "see my code", "access my code"}) {
		if tool, ok := availableTools["glob"]; ok {
			toolCalls = append(toolCalls, StreamingToolCall{
				Index: len(toolCalls),
				ID:    fmt.Sprintf("call_%s", generateToolCallID()),
				Type:  "function",
				Function: OpenAIFunctionCall{
					Name:      tool.Function.Name,
					Arguments: `{"pattern": "**/*", "path": "."}`,
				},
			})
		} else if tool, ok := availableTools["Glob"]; ok {
			toolCalls = append(toolCalls, StreamingToolCall{
				Index: len(toolCalls),
				ID:    fmt.Sprintf("call_%s", generateToolCallID()),
				Type:  "function",
				Function: OpenAIFunctionCall{
					Name:      tool.Function.Name,
					Arguments: `{"pattern": "**/*"}`,
				},
			})
		}
	}

	// Case 2: Questions about structure/architecture - use glob to explore
	if containsAny(topicLower, []string{"structure", "architecture", "layout", "organization", "how is.*organized"}) {
		if tool, ok := availableTools["glob"]; ok {
			toolCalls = append(toolCalls, StreamingToolCall{
				Index: len(toolCalls),
				ID:    fmt.Sprintf("call_%s", generateToolCallID()),
				Type:  "function",
				Function: OpenAIFunctionCall{
					Name:      tool.Function.Name,
					Arguments: `{"pattern": "**/*.go"}`,
				},
			})
		} else if tool, ok := availableTools["Glob"]; ok {
			toolCalls = append(toolCalls, StreamingToolCall{
				Index: len(toolCalls),
				ID:    fmt.Sprintf("call_%s", generateToolCallID()),
				Type:  "function",
				Function: OpenAIFunctionCall{
					Name:      tool.Function.Name,
					Arguments: `{"pattern": "**/*.go"}`,
				},
			})
		}
	}

	// Case 3: Search for specific content - use grep
	if containsAny(topicLower, []string{"search for", "find", "where is", "look for", "locate"}) {
		// Extract what to search for from the topic
		searchTerm := extractSearchTerm(topic)
		if searchTerm != "" {
			if tool, ok := availableTools["grep"]; ok {
				toolCalls = append(toolCalls, StreamingToolCall{
					Index: len(toolCalls),
					ID:    fmt.Sprintf("call_%s", generateToolCallID()),
					Type:  "function",
					Function: OpenAIFunctionCall{
						Name:      tool.Function.Name,
						Arguments: fmt.Sprintf(`{"pattern": "%s"}`, escapeJSONString(searchTerm)),
					},
				})
			} else if tool, ok := availableTools["Grep"]; ok {
				toolCalls = append(toolCalls, StreamingToolCall{
					Index: len(toolCalls),
					ID:    fmt.Sprintf("call_%s", generateToolCallID()),
					Type:  "function",
					Function: OpenAIFunctionCall{
						Name:      tool.Function.Name,
						Arguments: fmt.Sprintf(`{"pattern": "%s"}`, escapeJSONString(searchTerm)),
					},
				})
			}
		}
	}

	// Case 4: Read a specific file - use read
	if containsAny(topicLower, []string{"read ", "show me ", "what's in ", "contents of ", "open "}) {
		// Extract file path from the topic
		filePath := extractFilePath(topic)
		if filePath != "" {
			if tool, ok := availableTools["read"]; ok {
				toolCalls = append(toolCalls, StreamingToolCall{
					Index: len(toolCalls),
					ID:    fmt.Sprintf("call_%s", generateToolCallID()),
					Type:  "function",
					Function: OpenAIFunctionCall{
						Name:      tool.Function.Name,
						Arguments: fmt.Sprintf(`{"file_path": "%s"}`, escapeJSONString(filePath)),
					},
				})
			} else if tool, ok := availableTools["Read"]; ok {
				toolCalls = append(toolCalls, StreamingToolCall{
					Index: len(toolCalls),
					ID:    fmt.Sprintf("call_%s", generateToolCallID()),
					Type:  "function",
					Function: OpenAIFunctionCall{
						Name:      tool.Function.Name,
						Arguments: fmt.Sprintf(`{"file_path": "%s"}`, escapeJSONString(filePath)),
					},
				})
			}
		}
	}

	// Case 5: Execute command - use shell/bash
	if containsAny(topicLower, []string{"run ", "execute ", "command "}) {
		command := extractCommand(topic)
		if command != "" {
			if tool, ok := availableTools["shell"]; ok {
				toolCalls = append(toolCalls, StreamingToolCall{
					Index: len(toolCalls),
					ID:    fmt.Sprintf("call_%s", generateToolCallID()),
					Type:  "function",
					Function: OpenAIFunctionCall{
						Name:      tool.Function.Name,
						Arguments: fmt.Sprintf(`{"command": "%s"}`, escapeJSONString(command)),
					},
				})
			} else if tool, ok := availableTools["Bash"]; ok {
				toolCalls = append(toolCalls, StreamingToolCall{
					Index: len(toolCalls),
					ID:    fmt.Sprintf("call_%s", generateToolCallID()),
					Type:  "function",
					Function: OpenAIFunctionCall{
						Name:      tool.Function.Name,
						Arguments: fmt.Sprintf(`{"command": "%s"}`, escapeJSONString(command)),
					},
				})
			}
		}
	}

	// Case 6: Create/Write file - use Write tool
	// CRITICAL: This enables the AI to actually CREATE files like AGENTS.md
	if containsAny(topicLower, []string{"create ", "write ", "generate ", "make ", "add "}) &&
	   containsAny(topicLower, []string{".md", ".txt", ".json", ".yaml", ".yml", ".go", ".py", ".js", ".ts", "file", "document"}) {
		// Extract the file path from the topic
		filePath := extractCreateFilePath(topic)
		if filePath != "" {
			// Check for Write tool (case-insensitive)
			if tool, ok := availableTools["write"]; ok {
				// Extract or generate content from the synthesis
				content := extractFileContent(synthesis, filePath, topic)
				toolCalls = append(toolCalls, StreamingToolCall{
					Index: len(toolCalls),
					ID:    fmt.Sprintf("call_%s", generateToolCallID()),
					Type:  "function",
					Function: OpenAIFunctionCall{
						Name:      tool.Function.Name,
						Arguments: fmt.Sprintf(`{"file_path": "%s", "content": "%s"}`, escapeJSONString(filePath), escapeJSONString(content)),
					},
				})
			} else if tool, ok := availableTools["Write"]; ok {
				content := extractFileContent(synthesis, filePath, topic)
				toolCalls = append(toolCalls, StreamingToolCall{
					Index: len(toolCalls),
					ID:    fmt.Sprintf("call_%s", generateToolCallID()),
					Type:  "function",
					Function: OpenAIFunctionCall{
						Name:      tool.Function.Name,
						Arguments: fmt.Sprintf(`{"file_path": "%s", "content": "%s"}`, escapeJSONString(filePath), escapeJSONString(content)),
					},
				})
			}
		}
	}

	// Case 7: Analysis of synthesis - if synthesis mentions specific tools, try to call them
	// This handles cases where the debate concluded that a specific tool should be used
	for toolName, tool := range availableTools {
		toolNameLower := strings.ToLower(toolName)
		if strings.Contains(synthesisLower, "use "+toolNameLower) ||
		   strings.Contains(synthesisLower, "call "+toolNameLower) ||
		   strings.Contains(synthesisLower, "invoke "+toolNameLower) ||
		   strings.Contains(synthesisLower, toolNameLower+" tool") {
			// Check if we haven't already added this tool
			alreadyAdded := false
			for _, tc := range toolCalls {
				if tc.Function.Name == toolName {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				// Try to extract arguments from synthesis context
				args := extractToolArguments(toolName, synthesis)
				toolCalls = append(toolCalls, StreamingToolCall{
					Index: len(toolCalls),
					ID:    fmt.Sprintf("call_%s", generateToolCallID()),
					Type:  "function",
					Function: OpenAIFunctionCall{
						Name:      tool.Function.Name,
						Arguments: args,
					},
				})
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"topic":           topic[:min(50, len(topic))],
		"tool_count":      len(toolCalls),
		"available_tools": len(availableTools),
	}).Debug("Generated action tool calls from debate synthesis")

	return toolCalls
}

// containsAny checks if text contains any of the patterns
func containsAny(text string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}

// generateToolCallID generates a unique ID for a tool call
func generateToolCallID() string {
	// Generate a random ID using timestamp and random number
	return fmt.Sprintf("%d%04d", time.Now().UnixNano()%1000000, rand.Intn(10000))
}

// escapeJSONString escapes a string for safe inclusion in JSON
func escapeJSONString(s string) string {
	// Basic JSON escaping
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// extractSearchTerm extracts the search term from a query
func extractSearchTerm(topic string) string {
	// Simple extraction - look for patterns like "search for X" or "find X"
	patterns := []string{
		"search for ",
		"find ",
		"look for ",
		"locate ",
		"where is ",
	}
	topicLower := strings.ToLower(topic)
	for _, pattern := range patterns {
		idx := strings.Index(topicLower, pattern)
		if idx != -1 {
			remaining := topic[idx+len(pattern):]
			// Take until end of sentence or special character
			endIdx := strings.IndexAny(remaining, ".,?!;:")
			if endIdx != -1 {
				return strings.TrimSpace(remaining[:endIdx])
			}
			return strings.TrimSpace(remaining)
		}
	}
	return ""
}

// extractFilePath extracts a file path from a query
func extractFilePath(topic string) string {
	// Look for file path patterns
	patterns := []string{
		"read ",
		"show me ",
		"what's in ",
		"contents of ",
		"open ",
		"file ",
	}
	topicLower := strings.ToLower(topic)
	for _, pattern := range patterns {
		idx := strings.Index(topicLower, pattern)
		if idx != -1 {
			remaining := topic[idx+len(pattern):]
			// Take until whitespace or end
			endIdx := strings.IndexAny(remaining, " \t\n")
			if endIdx != -1 {
				return strings.TrimSpace(remaining[:endIdx])
			}
			return strings.TrimSpace(remaining)
		}
	}
	return ""
}

// extractCommand extracts a command from a query
func extractCommand(topic string) string {
	// Look for command patterns
	patterns := []string{
		"run ",
		"execute ",
		"command ",
	}
	topicLower := strings.ToLower(topic)
	for _, pattern := range patterns {
		idx := strings.Index(topicLower, pattern)
		if idx != -1 {
			remaining := topic[idx+len(pattern):]
			// Take until end of line
			endIdx := strings.Index(remaining, "\n")
			if endIdx != -1 {
				return strings.TrimSpace(remaining[:endIdx])
			}
			return strings.TrimSpace(remaining)
		}
	}
	return ""
}

// extractCreateFilePath extracts a file path from a create/write/generate request
func extractCreateFilePath(topic string) string {
	// Look for file creation patterns
	patterns := []string{
		"create ",
		"write ",
		"generate ",
		"make ",
		"add ",
	}
	topicLower := strings.ToLower(topic)

	for _, pattern := range patterns {
		idx := strings.Index(topicLower, pattern)
		if idx != -1 {
			remaining := topic[idx+len(pattern):]
			// Look for file path-like patterns (words with extensions or paths)
			words := strings.Fields(remaining)
			for _, word := range words {
				// Clean up the word (remove quotes, commas, etc.)
				word = strings.Trim(word, "\"'`,.:;!?")
				// Check if it looks like a file path or has an extension
				if strings.Contains(word, ".") || strings.Contains(word, "/") {
					// Handle relative paths and filenames
					if !strings.HasPrefix(word, "/") && !strings.HasPrefix(word, "./") {
						// Add ./ for relative paths if it doesn't start with a path
						if !strings.Contains(word, "/") {
							return "./" + word
						}
					}
					return word
				}
				// Special handling for common file names without context
				wordLower := strings.ToLower(word)
				if wordLower == "agents" || wordLower == "readme" || wordLower == "changelog" {
					// Check if next word has extension
					wordsIdx := strings.Index(remaining, word)
					if wordsIdx != -1 {
						afterWord := remaining[wordsIdx+len(word):]
						if strings.HasPrefix(strings.TrimSpace(strings.ToLower(afterWord)), ".md") {
							return "./" + word + ".md"
						}
					}
					return "./" + word + ".md"
				}
			}
		}
	}

	// Also look for explicit file mentions
	filePatterns := []string{"file ", "named ", "called "}
	for _, pattern := range filePatterns {
		idx := strings.Index(topicLower, pattern)
		if idx != -1 {
			remaining := topic[idx+len(pattern):]
			words := strings.Fields(remaining)
			if len(words) > 0 {
				word := strings.Trim(words[0], "\"'`,.:;!?")
				if strings.Contains(word, ".") {
					if !strings.HasPrefix(word, "/") && !strings.HasPrefix(word, "./") {
						return "./" + word
					}
					return word
				}
			}
		}
	}

	return ""
}

// extractFileContent extracts or generates content for a file from the synthesis
func extractFileContent(synthesis, filePath, topic string) string {
	// Check if synthesis already contains markdown-like content that could be file content
	// Look for code blocks or structured content in the synthesis

	// First, try to find content between code blocks
	if idx := strings.Index(synthesis, "```"); idx != -1 {
		afterStart := synthesis[idx+3:]
		// Skip the language identifier if present
		if nlIdx := strings.Index(afterStart, "\n"); nlIdx != -1 {
			afterStart = afterStart[nlIdx+1:]
		}
		if endIdx := strings.Index(afterStart, "```"); endIdx != -1 {
			return strings.TrimSpace(afterStart[:endIdx])
		}
	}

	// If the synthesis describes what should be in the file, use the synthesis as basis
	// Look for file description patterns
	descPatterns := []string{
		"the file should contain",
		"should include",
		"will contain",
		"should have",
		"content should be",
		"document should describe",
	}

	synthesisLower := strings.ToLower(synthesis)
	for _, pattern := range descPatterns {
		if idx := strings.Index(synthesisLower, pattern); idx != -1 {
			// Use the synthesis from this point as a guide
			break
		}
	}

	// Generate appropriate default content based on file type
	filePathLower := strings.ToLower(filePath)
	topicLower := strings.ToLower(topic)

	if strings.HasSuffix(filePathLower, "agents.md") || strings.Contains(topicLower, "agents") {
		return generateAgentsMDContent(synthesis, topic)
	}

	if strings.HasSuffix(filePathLower, "readme.md") {
		return generateReadmeMDContent(synthesis, topic)
	}

	// Default: use a cleaned-up version of the synthesis
	// Remove conversational phrases and format as document content
	content := cleanSynthesisForFile(synthesis)
	if content == "" {
		content = fmt.Sprintf("# %s\n\nGenerated based on: %s\n\n%s",
			strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)),
			topic,
			synthesis)
	}

	return content
}

// generateAgentsMDContent generates content for an AGENTS.md file
func generateAgentsMDContent(synthesis, topic string) string {
	var content strings.Builder

	content.WriteString("# AGENTS.md\n\n")
	content.WriteString("This file provides guidance to AI coding agents working with this codebase.\n\n")

	content.WriteString("## Project Overview\n\n")
	content.WriteString("Based on analysis of the codebase:\n\n")

	// Extract relevant information from synthesis
	if synthesis != "" {
		// Clean and format the synthesis
		cleaned := cleanSynthesisForFile(synthesis)
		if cleaned != "" {
			content.WriteString(cleaned)
			content.WriteString("\n\n")
		}
	}

	content.WriteString("## Key Guidelines\n\n")
	content.WriteString("- Follow existing code patterns and conventions\n")
	content.WriteString("- Maintain consistent formatting and style\n")
	content.WriteString("- Write clear, concise code with appropriate comments\n")
	content.WriteString("- Update tests when modifying functionality\n\n")

	content.WriteString("## Important Files\n\n")
	content.WriteString("- See project structure for key entry points\n")
	content.WriteString("- Configuration files in project root\n\n")

	return content.String()
}

// generateReadmeMDContent generates content for a README.md file
func generateReadmeMDContent(synthesis, topic string) string {
	var content strings.Builder

	content.WriteString("# Project\n\n")
	content.WriteString("## Description\n\n")

	if synthesis != "" {
		cleaned := cleanSynthesisForFile(synthesis)
		if cleaned != "" {
			content.WriteString(cleaned)
			content.WriteString("\n\n")
		}
	}

	content.WriteString("## Getting Started\n\n")
	content.WriteString("TODO: Add installation and usage instructions\n\n")

	content.WriteString("## Contributing\n\n")
	content.WriteString("TODO: Add contribution guidelines\n\n")

	return content.String()
}

// cleanSynthesisForFile cleans synthesis text for use as file content
func cleanSynthesisForFile(synthesis string) string {
	// Remove common conversational phrases
	removePatterns := []string{
		"Based on my analysis",
		"I would suggest",
		"I recommend",
		"Let me explain",
		"Here's what I found",
		"In summary",
		"To summarize",
		"The consensus is",
	}

	result := synthesis
	for _, pattern := range removePatterns {
		result = strings.ReplaceAll(result, pattern, "")
	}

	// Clean up multiple newlines
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(result)
}

// extractToolArguments attempts to extract appropriate arguments for a tool from context
func extractToolArguments(toolName string, context string) string {
	toolNameLower := strings.ToLower(toolName)
	switch toolNameLower {
	case "glob", "Glob":
		// Default glob pattern
		return `{"pattern": "**/*"}`
	case "grep", "Grep":
		// Try to extract a search pattern from context
		return `{"pattern": ".*"}`
	case "read", "Read":
		return `{"file_path": "README.md"}`
	case "ls":
		return `{"path": "."}`
	default:
		return "{}"
	}
}

// processToolResultsWithLLM processes tool results by making a direct LLM call
// CRITICAL: This enables proper handling of tool results from CLI agents like OpenCode
// Instead of just acknowledging, it actually generates useful insights from the tool output
func (h *UnifiedHandler) processToolResultsWithLLM(ctx context.Context, req *OpenAIChatRequest) (string, error) {
	// Build system prompt for tool result processing
	systemPrompt := `You are HelixAgent, an expert AI coding assistant with FULL ACCESS to the user's codebase through tools.

CRITICAL INSTRUCTIONS:
1. You have just received TOOL RESULTS from executing tools on the user's codebase
2. You MUST analyze these results and provide SPECIFIC, ACTIONABLE insights
3. If the user asked you to CREATE or MODIFY files, you MUST use the appropriate tools (Write, Edit) to do so
4. If you see file listings (from Glob), describe the project structure and key files
5. If you see file contents (from Read), analyze the code and provide insights
6. If you see search results (from Grep), explain what was found and where
7. NEVER say "I cannot see your codebase" - you CAN and HAVE seen it via tools
8. Be SPECIFIC - reference actual file names, line numbers, and code patterns from the results
9. If the task requires WRITING code or files, DO IT - don't just describe what you would do

When asked to create files like AGENTS.md:
- Actually CREATE the file using the Write tool
- Include all requested information based on what you found in the codebase
- Follow the user's specifications exactly`

	// Convert messages to internal format for LLM call
	var messages []models.Message
	messages = append(messages, models.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add all conversation messages including tool results
	// IMPORTANT: Convert tool messages to user messages to avoid provider-specific format issues
	for _, msg := range req.Messages {
		// Skip system messages (we already added our own)
		if msg.Role == "system" {
			continue
		}

		message := models.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Convert tool result messages to user messages with context
		// This avoids provider-specific tool message format requirements
		if msg.Role == "tool" || msg.ToolCallID != "" {
			message.Role = "user"
			message.Content = fmt.Sprintf("TOOL EXECUTION RESULT:\n```\n%s\n```\n\nPlease analyze this tool output and provide specific insights based on the data.", msg.Content)
		}

		// Convert assistant messages with tool_calls to plain assistant messages
		// Include the tool call info in the content
		if len(msg.ToolCalls) > 0 {
			var toolCallsStr strings.Builder
			if msg.Content != "" {
				toolCallsStr.WriteString(msg.Content)
				toolCallsStr.WriteString("\n\n")
			}
			toolCallsStr.WriteString("I executed the following tools:\n")
			for _, tc := range msg.ToolCalls {
				toolCallsStr.WriteString(fmt.Sprintf("- %s with arguments: %s\n", tc.Function.Name, tc.Function.Arguments))
			}
			message.Content = toolCallsStr.String()
		}

		messages = append(messages, message)
	}

	// Create internal request with ModelParams
	internalReq := &models.LLMRequest{
		Messages: messages,
		ModelParams: models.ModelParameters{
			MaxTokens:   4096,
			Temperature: 0.7,
		},
	}

	// Add tools if available so LLM can make additional tool calls if needed
	if len(req.Tools) > 0 {
		internalReq.ModelParams.ProviderSpecific = map[string]interface{}{
			"tools": req.Tools,
		}
	}

	// Get providers dynamically ordered by LLMsVerifier scores (highest first)
	// CRITICAL: NO hardcoded fallback chains - all ordering is based on real verification results
	providers := h.providerRegistry.ListProvidersOrderedByScore()
	if len(providers) == 0 {
		return "", fmt.Errorf("no providers available - check your configuration")
	}

	logrus.WithField("providers_ordered", providers).Debug("Using dynamically ordered providers for tool result processing")

	var lastErr error
	var successfulProvider string

	for _, providerName := range providers {
		// Check if parent context is already cancelled
		select {
		case <-ctx.Done():
			logrus.Warn("Parent context cancelled during tool result processing")
			// Return what we have or a helpful message
			if lastErr != nil {
				return "", fmt.Errorf("context cancelled while processing tool results: %w", lastErr)
			}
			return "", fmt.Errorf("context cancelled before tool results could be processed")
		default:
		}

		provider, err := h.providerRegistry.GetProvider(providerName)
		if err != nil || provider == nil {
			logrus.WithField("provider", providerName).Debug("Provider not available for tool results")
			continue
		}

		// Create a per-provider timeout context (60 seconds per provider)
		// This prevents one slow provider from exhausting the entire timeout
		providerCtx, providerCancel := context.WithTimeout(context.Background(), 60*time.Second)

		logrus.WithField("provider", providerName).Debug("Attempting tool result processing")

		resp, err := provider.Complete(providerCtx, internalReq)
		providerCancel() // Always cancel to release resources

		if err != nil {
			logrus.WithError(err).WithField("provider", providerName).Debug("Tool result LLM call failed, trying next")
			lastErr = err
			continue
		}

		if resp == nil || resp.Content == "" {
			logrus.WithField("provider", providerName).Debug("Empty response from provider, trying next")
			lastErr = fmt.Errorf("empty response from %s", providerName)
			continue
		}

		successfulProvider = providerName
		logrus.WithFields(logrus.Fields{
			"provider":    providerName,
			"content_len": len(resp.Content),
		}).Info("Tool result processed successfully")

		// CRITICAL: Check for embedded function calls in the response
		// Some LLMs output <function=write>...</function> style text instead of proper tool_calls
		// We need to parse and execute these embedded calls
		content, executedTools := h.processEmbeddedFunctionCalls(ctx, resp.Content)
		if len(executedTools) > 0 {
			logrus.WithFields(logrus.Fields{
				"tools_executed": len(executedTools),
				"provider":       providerName,
			}).Info("Executed embedded function calls from LLM response")
		}

		return content, nil
	}

	// If all providers failed, return a helpful error message
	logrus.WithError(lastErr).WithFields(logrus.Fields{
		"successful_provider": successfulProvider,
		"providers_tried":     len(providers),
	}).Error("All providers failed to process tool results")
	if lastErr != nil {
		return "", fmt.Errorf("all %d dynamically-ordered providers failed to process tool results (last error: %w). Check provider API keys and LLMsVerifier scores", len(providers), lastErr)
	}
	return "", fmt.Errorf("no providers available to process tool results - check your configuration and LLMsVerifier")
}

// EmbeddedFunctionCall represents a function call parsed from LLM response text
type EmbeddedFunctionCall struct {
	Name       string
	Parameters map[string]string
	RawContent string
}

// processEmbeddedFunctionCalls detects and executes embedded function calls in LLM responses
// This handles cases where LLMs output <function=write>...</function> style text instead of proper tool_calls
func (h *UnifiedHandler) processEmbeddedFunctionCalls(ctx context.Context, content string) (string, []string) {
	var executedTools []string

	// Parse embedded function calls from the content
	calls := parseEmbeddedFunctionCalls(content)
	if len(calls) == 0 {
		return content, executedTools
	}

	// Execute each embedded function call
	cleanedContent := content
	for _, call := range calls {
		result, err := h.executeEmbeddedFunctionCall(ctx, call)
		if err != nil {
			logrus.WithError(err).WithField("function", call.Name).Warn("Failed to execute embedded function call")
			// Replace the function call with an error message
			errorMsg := fmt.Sprintf("\n\n⚠️ Failed to execute %s: %v\n\n", call.Name, err)
			cleanedContent = strings.Replace(cleanedContent, call.RawContent, errorMsg, 1)
		} else {
			// Replace the function call with a success message
			successMsg := fmt.Sprintf("\n\n✅ Successfully executed %s: %s\n\n", call.Name, result)
			cleanedContent = strings.Replace(cleanedContent, call.RawContent, successMsg, 1)
			executedTools = append(executedTools, call.Name)
		}
	}

	return cleanedContent, executedTools
}

// parseEmbeddedFunctionCalls parses function calls from LLM response text
// Supports multiple formats:
// - <function=write><parameter=path>...</parameter><parameter=content>...</parameter></function>
// - <function_call name="write"><path>...</path><content>...</content></function_call>
// - ```function:write ... ```
func parseEmbeddedFunctionCalls(content string) []EmbeddedFunctionCall {
	var calls []EmbeddedFunctionCall

	// Pattern 1: <function=name>...<parameter=key>value</parameter>...</function>
	funcPattern := regexp.MustCompile(`(?s)<function=(\w+)>(.*?)</function>`)
	funcMatches := funcPattern.FindAllStringSubmatch(content, -1)
	for _, match := range funcMatches {
		if len(match) >= 3 {
			call := EmbeddedFunctionCall{
				Name:       match[1],
				Parameters: make(map[string]string),
				RawContent: match[0],
			}
			// Parse parameters
			paramPattern := regexp.MustCompile(`(?s)<parameter=(\w+)>(.*?)</parameter>`)
			paramMatches := paramPattern.FindAllStringSubmatch(match[2], -1)
			for _, pm := range paramMatches {
				if len(pm) >= 3 {
					call.Parameters[pm[1]] = strings.TrimSpace(pm[2])
				}
			}
			calls = append(calls, call)
		}
	}

	// Pattern 2: <function_call name="...">...</function_call>
	fcPattern := regexp.MustCompile(`(?s)<function_call\s+name="(\w+)">(.*?)</function_call>`)
	fcMatches := fcPattern.FindAllStringSubmatch(content, -1)
	for _, match := range fcMatches {
		if len(match) >= 3 {
			call := EmbeddedFunctionCall{
				Name:       match[1],
				Parameters: make(map[string]string),
				RawContent: match[0],
			}
			// Parse inner elements as parameters using predefined tag names
			// Go's regexp doesn't support backreferences
			innerContent := match[2]
			paramTags := []string{"path", "file_path", "filepath", "content", "data", "text", "pattern", "old_string", "new_string", "command"}
			for _, paramTag := range paramTags {
				paramPattern := regexp.MustCompile(`(?s)<` + paramTag + `>(.*?)</` + paramTag + `>`)
				paramMatches := paramPattern.FindStringSubmatch(innerContent)
				if len(paramMatches) >= 2 {
					call.Parameters[paramTag] = strings.TrimSpace(paramMatches[1])
				}
			}
			calls = append(calls, call)
		}
	}

	// Pattern 3: Simple XML format with Write/Edit/Read tags
	// <Write><file_path>...</file_path><content>...</content></Write>
	// Go's regexp doesn't support backreferences, so we check each tag separately
	simpleTags := []string{"Write", "Edit", "Read", "Glob", "Grep", "Bash"}
	for _, tag := range simpleTags {
		tagPattern := regexp.MustCompile(`(?s)<` + tag + `>(.*?)</` + tag + `>`)
		tagMatches := tagPattern.FindAllStringSubmatch(content, -1)
		for _, match := range tagMatches {
			if len(match) >= 2 {
				call := EmbeddedFunctionCall{
					Name:       strings.ToLower(tag),
					Parameters: make(map[string]string),
					RawContent: match[0],
				}
				// Parse inner elements as parameters using a simple approach
				// Match <tagname>content</tagname> patterns
				innerContent := match[1]
				paramTags := []string{"file_path", "filepath", "path", "content", "data", "text", "pattern", "old_string", "new_string", "command"}
				for _, paramTag := range paramTags {
					paramPattern := regexp.MustCompile(`(?s)<` + paramTag + `>(.*?)</` + paramTag + `>`)
					paramMatches := paramPattern.FindStringSubmatch(innerContent)
					if len(paramMatches) >= 2 {
						call.Parameters[paramTag] = strings.TrimSpace(paramMatches[1])
					}
				}
				calls = append(calls, call)
			}
		}
	}

	return calls
}

// executeEmbeddedFunctionCall executes a parsed function call
func (h *UnifiedHandler) executeEmbeddedFunctionCall(ctx context.Context, call EmbeddedFunctionCall) (string, error) {
	funcName := strings.ToLower(call.Name)

	switch funcName {
	case "write":
		return h.executeWriteFunction(ctx, call)
	case "edit":
		return h.executeEditFunction(ctx, call)
	case "read":
		return h.executeReadFunction(ctx, call)
	case "glob":
		return h.executeGlobFunction(ctx, call)
	case "grep":
		return h.executeGrepFunction(ctx, call)
	case "bash", "shell":
		return h.executeBashFunction(ctx, call)
	default:
		return "", fmt.Errorf("unsupported function: %s", call.Name)
	}
}

// executeWriteFunction writes a file to disk
func (h *UnifiedHandler) executeWriteFunction(ctx context.Context, call EmbeddedFunctionCall) (string, error) {
	// Get file path - try various parameter names
	filePath := getParam(call.Parameters, "path", "file_path", "filepath", "file")
	if filePath == "" {
		return "", fmt.Errorf("missing file path parameter")
	}

	// Get content
	content := getParam(call.Parameters, "content", "data", "text", "body")
	if content == "" {
		return "", fmt.Errorf("missing content parameter")
	}

	// Resolve relative paths
	if !filepath.IsAbs(filePath) {
		// Get current working directory or use a default
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		filePath = filepath.Join(cwd, filePath)
	}

	// Create parent directories if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write the file
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	logrus.WithField("file", filePath).Info("Successfully wrote file from embedded function call")
	return fmt.Sprintf("File written: %s (%d bytes)", filePath, len(content)), nil
}

// executeEditFunction edits an existing file
func (h *UnifiedHandler) executeEditFunction(ctx context.Context, call EmbeddedFunctionCall) (string, error) {
	filePath := getParam(call.Parameters, "path", "file_path", "filepath", "file")
	if filePath == "" {
		return "", fmt.Errorf("missing file path parameter")
	}

	oldString := getParam(call.Parameters, "old_string", "old", "search", "find")
	newString := getParam(call.Parameters, "new_string", "new", "replace", "replacement")

	if oldString == "" {
		return "", fmt.Errorf("missing old_string parameter")
	}

	// Resolve path
	if !filepath.IsAbs(filePath) {
		cwd, _ := os.Getwd()
		filePath = filepath.Join(cwd, filePath)
	}

	// Read existing content
	existingContent, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Replace content
	newContent := strings.Replace(string(existingContent), oldString, newString, -1)

	// Write back
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return fmt.Sprintf("File edited: %s", filePath), nil
}

// executeReadFunction reads a file (returns content for display, not execution)
func (h *UnifiedHandler) executeReadFunction(ctx context.Context, call EmbeddedFunctionCall) (string, error) {
	filePath := getParam(call.Parameters, "path", "file_path", "filepath", "file")
	if filePath == "" {
		return "", fmt.Errorf("missing file path parameter")
	}

	if !filepath.IsAbs(filePath) {
		cwd, _ := os.Getwd()
		filePath = filepath.Join(cwd, filePath)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return fmt.Sprintf("Read %d bytes from %s", len(content), filePath), nil
}

// executeGlobFunction finds files matching a pattern
func (h *UnifiedHandler) executeGlobFunction(ctx context.Context, call EmbeddedFunctionCall) (string, error) {
	pattern := getParam(call.Parameters, "pattern", "glob", "path")
	if pattern == "" {
		return "", fmt.Errorf("missing pattern parameter")
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob failed: %w", err)
	}

	return fmt.Sprintf("Found %d files matching %s", len(matches), pattern), nil
}

// executeGrepFunction searches for patterns in files
func (h *UnifiedHandler) executeGrepFunction(ctx context.Context, call EmbeddedFunctionCall) (string, error) {
	pattern := getParam(call.Parameters, "pattern", "search", "query", "regex")
	if pattern == "" {
		return "", fmt.Errorf("missing pattern parameter")
	}

	return fmt.Sprintf("Search pattern registered: %s (grep not fully implemented for embedded calls)", pattern), nil
}

// executeBashFunction executes a shell command (with safety restrictions)
func (h *UnifiedHandler) executeBashFunction(ctx context.Context, call EmbeddedFunctionCall) (string, error) {
	// For security, we don't execute arbitrary bash commands from embedded function calls
	command := getParam(call.Parameters, "command", "cmd", "script")
	return "", fmt.Errorf("bash execution from embedded function calls is disabled for security - command was: %s", command)
}

// getParam gets a parameter value by trying multiple possible keys
func getParam(params map[string]string, keys ...string) string {
	for _, key := range keys {
		if val, ok := params[key]; ok && val != "" {
			return val
		}
		// Also try lowercase version
		if val, ok := params[strings.ToLower(key)]; ok && val != "" {
			return val
		}
	}
	return ""
}

// generateFallbackToolResultsResponse generates a fallback response when LLM processing fails
// This extracts useful information from the tool results and generates a helpful message
func (h *UnifiedHandler) generateFallbackToolResultsResponse(req *OpenAIChatRequest) string {
	var response strings.Builder
	response.WriteString("I received the tool results. Here's what I found:\n\n")

	// Process each message looking for tool results
	for _, msg := range req.Messages {
		if msg.Role == "tool" || msg.ToolCallID != "" {
			content := msg.Content
			if content == "" {
				continue
			}

			// Analyze the content to provide useful information
			lines := strings.Split(content, "\n")
			if len(lines) > 0 {
				// Check if it looks like a file listing (from Glob)
				if strings.Contains(content, ".go") || strings.Contains(content, ".ts") || strings.Contains(content, ".py") {
					response.WriteString("**File Listing:**\n")
					fileCount := 0
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if line != "" && (strings.Contains(line, ".") || strings.HasPrefix(line, "/")) {
							fileCount++
							if fileCount <= 20 { // Limit to first 20 files
								response.WriteString(fmt.Sprintf("- %s\n", line))
							}
						}
					}
					if fileCount > 20 {
						response.WriteString(fmt.Sprintf("- ... and %d more files\n", fileCount-20))
					}
					response.WriteString("\n")
				} else if len(content) < 2000 {
					// For smaller content, include it directly
					response.WriteString("**Tool Output:**\n```\n")
					response.WriteString(content)
					response.WriteString("\n```\n\n")
				} else {
					// For large content, summarize
					response.WriteString(fmt.Sprintf("**Tool Output:** (Received %d bytes of data)\n\n", len(content)))
				}
			}
		}

		// Look for the original user request to understand what was asked
		if msg.Role == "user" && !strings.Contains(msg.Content, "TOOL EXECUTION RESULT") {
			// This is likely the original user question
			userRequest := strings.ToLower(msg.Content)

			// Check if user asked to create a file
			if strings.Contains(userRequest, "create") && strings.Contains(userRequest, "agents.md") {
				response.WriteString("\n**Note:** To create the AGENTS.md file, I would need the LLM providers to be available to generate the content. ")
				response.WriteString("Please check your API configurations and try again.\n")
			}
		}
	}

	if response.Len() == 0 {
		return "" // No useful fallback could be generated
	}

	return response.String()
}
