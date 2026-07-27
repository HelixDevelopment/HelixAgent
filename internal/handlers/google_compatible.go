package handlers

// Google Gemini-compatible /v1beta/models/:model:generateContent translator
// (Finding #21).
//
// Gemini-protocol clients (Google's official Gemini SDKs, Vertex AI tooling
// pointed at custom endpoints, anything that calls
// /v1beta/models/<MODEL>:generateContent) expect Google's request shape.
// HelixAgent's native API is OpenAI-compatible. This file provides the
// translation shim that:
//   1. Accepts Google's Generate Content request shape
//   2. Converts to the internal OpenAIChatRequest
//   3. Delegates to UnifiedHandler.ChatCompletions in-process via a
//      capturing ResponseWriter (same dispatch pattern as the Anthropic
//      translator in anthropic_compatible.go)
//   4. Re-shapes the OpenAI response into Google's GenerateContentResponse
//
// Streaming is NOT implemented in this first cut; the streamGenerateContent
// route returns a clear 400 error so clients fail fast instead of dropping
// the stream= flag silently. SSE event-shape translation is a follow-up.
//
// Note: Gemini CLI specifically does not honor `--openai-base-url` for the
// default mode (it goes to Google's API directly), so the practical
// audience for this endpoint is programmatic clients that can override
// the base URL freely.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// GoogleGenerateContentRequest mirrors the Google Generative Language
// API request shape. Only the fields relevant for HelixAgent ensemble
// routing are modeled; extra fields are accepted but ignored.
type GoogleGenerateContentRequest struct {
	Contents          []GoogleContent          `json:"contents"`
	SystemInstruction *GoogleContent           `json:"systemInstruction,omitempty"`
	GenerationConfig  *GoogleGenerationConfig  `json:"generationConfig,omitempty"`
	SafetySettings    []map[string]interface{} `json:"safetySettings,omitempty"`
	// Tools intentionally omitted for the first cut. Function-calling
	// schema mapping (Google's functionDeclarations vs OpenAI's
	// parameters JSONSchema) requires a deeper translation pass.
	Tools []map[string]interface{} `json:"tools,omitempty"`
}

// GoogleContent is one turn in the conversation. Roles are "user" or
// "model" (Google's vocabulary; OpenAI uses "user" and "assistant").
type GoogleContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GooglePart `json:"parts"`
}

// GooglePart is one piece of a content turn. Only text parts are
// honored in this first cut; inlineData / functionCall / functionResponse
// blocks return an explicit error so the client can fall back.
type GooglePart struct {
	Text       string                 `json:"text,omitempty"`
	InlineData map[string]interface{} `json:"inlineData,omitempty"`
}

// GoogleGenerationConfig carries sampling / output knobs.
type GoogleGenerationConfig struct {
	Temperature     float64  `json:"temperature,omitempty"`
	TopP            float64  `json:"topP,omitempty"`
	TopK            int      `json:"topK,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
	CandidateCount  int      `json:"candidateCount,omitempty"`
}

// GoogleGenerateContentResponse mirrors the Google API response envelope.
type GoogleGenerateContentResponse struct {
	Candidates    []GoogleCandidate    `json:"candidates"`
	UsageMetadata *GoogleUsageMetadata `json:"usageMetadata,omitempty"`
	ModelVersion  string               `json:"modelVersion,omitempty"`
}

// GoogleCandidate is one model response candidate.
type GoogleCandidate struct {
	Content      GoogleContent `json:"content"`
	FinishReason string        `json:"finishReason,omitempty"`
	Index        int           `json:"index"`
}

// GoogleUsageMetadata carries token counts.
type GoogleUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// GoogleErrorResponse is Google's standard error envelope.
type GoogleErrorResponse struct {
	Error GoogleError `json:"error"`
}

// GoogleError is the inner error object.
type GoogleError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// RegisterGoogleRoutes wires the /v1beta/models/:modelAction route
// that Google clients (Gemini SDK, Vertex AI tooling) hit when their
// base URL points at HelixAgent. Registered on the ROOT engine (not
// the /v1 protected group) because Google's URL contract is
// /v1beta/models/<MODEL>:<METHOD> — completely outside HelixAgent's
// /v1 namespace.
//
// Gin captures the entire trailing segment including the colon-method
// as the modelAction param value; splitGoogleModelAction recovers
// model + method from it.
func (h *UnifiedHandler) RegisterGoogleRoutes(engine *gin.Engine, auth gin.HandlerFunc) {
	group := engine.Group("/v1beta").Use(auth)
	group.POST("/models/:modelAction", h.GoogleGenerateContent)
}

// GoogleGenerateContent is the /generateContent handler. It dispatches
// streaming requests to a 400-not-supported error and otherwise
// translates → ChatCompletions → re-shapes.
func (h *UnifiedHandler) GoogleGenerateContent(c *gin.Context) {
	logrus.Info("[ENTRY] GoogleGenerateContent handler called")

	model, method := splitGoogleModelAction(c.Param("modelAction"), c.Request.URL.Path)
	if strings.HasPrefix(method, "stream") {
		sendGoogleError(c, http.StatusBadRequest, "INVALID_ARGUMENT",
			"streaming generateContent is not yet supported by HelixAgent's "+
				"/v1beta translator (Finding #21). Use the non-streaming variant "+
				"or the OpenAI-compatible /v1/chat/completions endpoint with "+
				"stream=true.")
		return
	}

	var req GoogleGenerateContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		sendGoogleError(c, http.StatusBadRequest, "INVALID_ARGUMENT",
			"Invalid request body: "+err.Error())
		return
	}

	if len(req.Tools) > 0 {
		sendGoogleError(c, http.StatusBadRequest, "INVALID_ARGUMENT",
			"Google function-calling tools are not yet supported by HelixAgent's "+
				"/v1beta translator. Use /v1/chat/completions with OpenAI tool "+
				"format instead.")
		return
	}

	openaiReq, err := translateGoogleToOpenAI(&req, model)
	if err != nil {
		sendGoogleError(c, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}

	openaiResp, statusCode, err := h.dispatchToChatCompletions(c, openaiReq)
	if err != nil {
		sendGoogleError(c, http.StatusInternalServerError, "INTERNAL",
			"Failed to dispatch to chat completions: "+err.Error())
		return
	}
	if statusCode != http.StatusOK {
		sendGoogleError(c, statusCode, "INTERNAL",
			fmt.Sprintf("Upstream chat completions returned status %d", statusCode))
		return
	}

	googleResp := translateOpenAIToGoogle(openaiResp, model)
	c.JSON(http.StatusOK, googleResp)
}

// splitGoogleModelAction extracts the model name and action method from
// a Google-style `<MODEL>:<METHOD>` path segment. Falls back to deriving
// from the URL path when the param is just a model name (i.e. when the
// route already encoded the method in the path).
func splitGoogleModelAction(param, fullPath string) (model, method string) {
	if idx := strings.Index(param, ":"); idx > 0 {
		return param[:idx], param[idx+1:]
	}
	// Param has no colon — derive method from URL path suffix.
	if strings.HasSuffix(fullPath, "/streamGenerateContent") {
		return param, "streamGenerateContent"
	}
	if strings.HasSuffix(fullPath, "/generateContent") {
		return param, "generateContent"
	}
	return param, ""
}

// translateGoogleToOpenAI converts a Google request to the internal
// OpenAI chat shape.
func translateGoogleToOpenAI(req *GoogleGenerateContentRequest, model string) (*OpenAIChatRequest, error) {
	out := &OpenAIChatRequest{
		Model:  model,
		Stream: false, // streaming rejected before we get here
	}
	if req.GenerationConfig != nil {
		out.MaxTokens = req.GenerationConfig.MaxOutputTokens
		out.Temperature = req.GenerationConfig.Temperature
		out.TopP = req.GenerationConfig.TopP
		out.Stop = req.GenerationConfig.StopSequences
	}

	if req.SystemInstruction != nil {
		text, err := flattenGoogleParts(req.SystemInstruction.Parts)
		if err != nil {
			return nil, fmt.Errorf("systemInstruction: %w", err)
		}
		if text != "" {
			out.Messages = append(out.Messages, OpenAIMessage{
				Role:    "system",
				Content: text,
			})
		}
	}

	for i, c := range req.Contents {
		text, err := flattenGoogleParts(c.Parts)
		if err != nil {
			return nil, fmt.Errorf("contents[%d]: %w", i, err)
		}
		out.Messages = append(out.Messages, OpenAIMessage{
			Role:    googleToOpenAIRole(c.Role),
			Content: text,
		})
	}

	return out, nil
}

// flattenGoogleParts joins all text parts of a Google content turn,
// rejecting inlineData (image/audio) which can't be expressed in the
// OpenAI text-only request used here.
func flattenGoogleParts(parts []GooglePart) (string, error) {
	var out []string
	for _, p := range parts {
		if len(p.InlineData) > 0 {
			return "", fmt.Errorf(
				"inlineData parts (images/audio) not supported by /v1beta translator")
		}
		if p.Text != "" {
			out = append(out, p.Text)
		}
	}
	return strings.Join(out, "\n"), nil
}

// googleToOpenAIRole maps Google's role vocabulary to OpenAI's. Google
// uses "model" for the assistant's turn; OpenAI uses "assistant".
// Empty role defaults to "user" (the most common case for one-shot
// generateContent requests).
func googleToOpenAIRole(googleRole string) string {
	switch strings.ToLower(googleRole) {
	case "model":
		return "assistant"
	case "user", "":
		return "user"
	default:
		return googleRole
	}
}

// openaiToGoogleRole is the inverse — assistant→model.
func openaiToGoogleRole(openaiRole string) string {
	if strings.ToLower(openaiRole) == "assistant" {
		return "model"
	}
	return openaiRole
}

// translateOpenAIToGoogle reshapes an OpenAI completion JSON into the
// Google GenerateContentResponse envelope.
func translateOpenAIToGoogle(openaiResp []byte, requestedModel string) *GoogleGenerateContentResponse {
	var openai struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
			Index        int    `json:"index"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(openaiResp, &openai)

	model := openai.Model
	if model == "" {
		model = requestedModel
	}

	candidates := make([]GoogleCandidate, 0, len(openai.Choices))
	for _, ch := range openai.Choices {
		candidates = append(candidates, GoogleCandidate{
			Index: ch.Index,
			Content: GoogleContent{
				Role:  openaiToGoogleRole(ch.Message.Role),
				Parts: []GooglePart{{Text: ch.Message.Content}},
			},
			FinishReason: mapOpenAIFinishReasonToGoogle(ch.FinishReason),
		})
	}
	if len(candidates) == 0 {
		// Always emit at least one candidate so clients that index into
		// candidates[0] don't crash on a degenerate upstream response.
		candidates = append(candidates, GoogleCandidate{
			Content: GoogleContent{
				Role:  "model",
				Parts: []GooglePart{{Text: ""}},
			},
			FinishReason: "STOP",
		})
	}

	return &GoogleGenerateContentResponse{
		Candidates: candidates,
		UsageMetadata: &GoogleUsageMetadata{
			PromptTokenCount:     openai.Usage.PromptTokens,
			CandidatesTokenCount: openai.Usage.CompletionTokens,
			TotalTokenCount:      openai.Usage.TotalTokens,
		},
		ModelVersion: model,
	}
}

// mapOpenAIFinishReasonToGoogle translates OpenAI's finish_reason
// vocabulary to Google's UPPER_SNAKE_CASE form.
func mapOpenAIFinishReasonToGoogle(openaiReason string) string {
	switch openaiReason {
	case "stop", "":
		return "STOP"
	case "length":
		return "MAX_TOKENS"
	case "content_filter":
		return "SAFETY"
	case "tool_calls", "function_call":
		return "STOP" // Google doesn't have a direct tool-use stop reason in v1beta
	default:
		return "STOP"
	}
}

// sendGoogleError writes a Google-shaped error envelope.
func sendGoogleError(c *gin.Context, status int, statusCode, message string) {
	c.JSON(status, GoogleErrorResponse{
		Error: GoogleError{
			Code:    status,
			Message: message,
			Status:  statusCode,
		},
	})
}
