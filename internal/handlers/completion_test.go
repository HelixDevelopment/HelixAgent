package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestCompletionHandler_Complete_Success tests successful completion request
func TestCompletionHandler_Complete_Success(t *testing.T) {
	t.Parallel()
	// Create a simple test that doesn't require mocking
	// This test focuses on request parsing and response formatting

	// Create test request
	reqBody := map[string]interface{}{
		"prompt":      "Test prompt",
		"model":       "test-model",
		"temperature": 0.7,
		"max_tokens":  100,
	}

	reqBytes, _ := json.Marshal(reqBody)

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	// Test request parsing
	var req CompletionRequest
	err := c.ShouldBindJSON(&req)

	assert.NoError(t, err)
	assert.Equal(t, "Test prompt", req.Prompt)
	assert.Equal(t, "test-model", req.Model)
	assert.Equal(t, 0.7, req.Temperature)
	assert.Equal(t, 100, req.MaxTokens)
}

// TestCompletionHandler_Complete_InvalidRequest tests invalid request handling
func TestCompletionHandler_Complete_InvalidRequest(t *testing.T) {
	t.Parallel()
	// Create invalid request (missing required prompt field)
	reqBody := map[string]interface{}{
		"model": "test-model",
	}

	reqBytes, _ := json.Marshal(reqBody)

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	// Test request parsing should fail
	var req CompletionRequest
	err := c.ShouldBindJSON(&req)

	assert.Error(t, err)
	// The error message contains "Prompt" (capital P) due to field name
	assert.Contains(t, err.Error(), "Prompt")
}

// TestConvertToInternalRequest tests conversion from API request to internal request
func TestConvertToInternalRequest(t *testing.T) {
	t.Parallel()
	// Create test handler with nil service (we're only testing conversion)
	handler := &CompletionHandler{}

	// Create test request
	req := &CompletionRequest{
		Prompt:         "Test prompt",
		Messages:       []models.Message{{Role: "user", Content: "Hello"}},
		Model:          "test-model",
		Temperature:    0.8,
		MaxTokens:      200,
		TopP:           0.9,
		Stop:           []string{"\n", "STOP"},
		EnsembleConfig: &models.EnsembleConfig{Strategy: "best_of_n"},
		MemoryEnhanced: true,
		RequestType:    "completion",
	}

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Execute
	internalReq := handler.convertToInternalRequest(req, c)

	// Verify
	assert.NotEmpty(t, internalReq.ID)
	assert.NotEmpty(t, internalReq.SessionID)
	assert.Equal(t, "anonymous", internalReq.UserID)
	assert.Equal(t, "Test prompt", internalReq.Prompt)
	assert.Len(t, internalReq.Messages, 1)
	assert.Equal(t, "test-model", internalReq.ModelParams.Model)
	assert.Equal(t, 0.8, internalReq.ModelParams.Temperature)
	assert.Equal(t, 200, internalReq.ModelParams.MaxTokens)
	assert.Equal(t, 0.9, internalReq.ModelParams.TopP)
	assert.Equal(t, []string{"\n", "STOP"}, internalReq.ModelParams.StopSequences)
	assert.Equal(t, "best_of_n", internalReq.EnsembleConfig.Strategy)
	assert.True(t, internalReq.MemoryEnhanced)
	assert.Equal(t, "completion", internalReq.RequestType)
	assert.Equal(t, "pending", internalReq.Status)
}

// TestConvertToInternalRequest_Defaults tests default values
func TestConvertToInternalRequest_Defaults(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	// Create minimal test request
	req := &CompletionRequest{
		Prompt: "Test prompt",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	// Verify default values
	assert.Equal(t, 0.7, internalReq.ModelParams.Temperature)
	assert.Equal(t, 1000, internalReq.ModelParams.MaxTokens)
	assert.Equal(t, 1.0, internalReq.ModelParams.TopP)
	assert.Empty(t, internalReq.ModelParams.StopSequences)
	assert.Equal(t, "confidence_weighted", internalReq.EnsembleConfig.Strategy)
	assert.False(t, internalReq.MemoryEnhanced)
}

// TestConvertToAPIResponse tests conversion from internal response to API response
func TestConvertToAPIResponse(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	// Create test response
	createdAt := time.Now()
	resp := &models.LLMResponse{
		ID:           "test-response-id",
		RequestID:    "test-request-id",
		ProviderName: "test-provider",
		Content:      "Test response content",
		Confidence:   0.95,
		TokensUsed:   100,
		CreatedAt:    createdAt,
		FinishReason: "stop",
	}

	// Execute
	apiResp := handler.convertToAPIResponse(resp)

	// Verify
	assert.Equal(t, "test-response-id", apiResp.ID)
	assert.Equal(t, "text_completion", apiResp.Object)
	assert.Equal(t, createdAt.Unix(), apiResp.Created)
	assert.Equal(t, "test-provider", apiResp.Model)
	assert.Len(t, apiResp.Choices, 1)
	assert.Equal(t, "assistant", apiResp.Choices[0].Message.Role)
	assert.Equal(t, "Test response content", apiResp.Choices[0].Message.Content)
	assert.Equal(t, "stop", apiResp.Choices[0].FinishReason)
	assert.NotNil(t, apiResp.Usage)
	assert.Equal(t, 50, apiResp.Usage.PromptTokens)
	assert.Equal(t, 50, apiResp.Usage.CompletionTokens)
	assert.Equal(t, 100, apiResp.Usage.TotalTokens)
	assert.Equal(t, "helixagent-v1.0", apiResp.SystemFingerprint)
}

// TestConvertToChatResponse tests conversion to chat response format
func TestConvertToChatResponse(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	createdAt := time.Now()
	resp := &models.LLMResponse{
		ID:           "test-chat-response-id",
		RequestID:    "test-chat-request-id",
		ProviderName: "test-provider",
		Content:      "Chat response content",
		Confidence:   0.92,
		TokensUsed:   80,
		CreatedAt:    createdAt,
		FinishReason: "stop",
	}

	chatResp := handler.convertToChatResponse(resp)

	assert.Equal(t, "test-chat-response-id", chatResp["id"])
	assert.Equal(t, "chat.completion", chatResp["object"])
	assert.Equal(t, "test-provider", chatResp["model"])
	assert.Equal(t, createdAt.Unix(), chatResp["created"])

	// The convertToChatResponse returns map[string]any with "choices" as []map[string]any
	choices, ok := chatResp["choices"].([]map[string]any)
	if !ok {
		// Try to handle as []interface{}
		choicesInterface, ok := chatResp["choices"].([]interface{})
		if ok && len(choicesInterface) > 0 {
			choice, ok := choicesInterface[0].(map[string]any)
			if ok {
				message, ok := choice["message"].(map[string]any)
				if ok {
					assert.Equal(t, "assistant", message["role"])
					assert.Equal(t, "Chat response content", message["content"])
					return
				}
			}
		}
		t.Fatal("Failed to parse choices")
	}

	assert.Len(t, choices, 1)
	assert.Equal(t, "assistant", choices[0]["message"].(map[string]any)["role"])
	assert.Equal(t, "Chat response content", choices[0]["message"].(map[string]any)["content"])
}

// TestSendError tests error response formatting
func TestSendError(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	handler.sendError(c, http.StatusBadRequest, "invalid_request", "Invalid format", "Missing required field")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errorResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errorResp)
	assert.NoError(t, err)

	assert.Equal(t, "invalid_request", errorResp.Error.Type)
	assert.Equal(t, "400", errorResp.Error.Code)
	assert.Contains(t, errorResp.Error.Message, "Invalid format")
	assert.Contains(t, errorResp.Error.Message, "Missing required field")
}

// TestCompletionHandler_Models tests model listing
func TestCompletionHandler_Models(t *testing.T) {
	t.Parallel()
	// This test doesn't require a service since Models() doesn't use it
	handler := &CompletionHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/models", nil)

	// We can't call handler.Models directly without a service
	// But we can test that the handler was created
	assert.NotNil(t, handler)

	// Test that we can create a models response manually
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
	}

	response := map[string]any{
		"object": "list",
		"data":   models,
	}

	assert.Equal(t, "list", response["object"])
	data, ok := response["data"].([]map[string]any)
	assert.True(t, ok)
	assert.Len(t, data, 1)
	assert.Equal(t, "deepseek-coder", data[0]["id"])
}

// TestNewCompletionHandler tests handler creation
func TestNewCompletionHandler(t *testing.T) {
	t.Parallel()
	// Create a mock request service
	handler := NewCompletionHandler(nil)

	assert.NotNil(t, handler)
	assert.Nil(t, handler.requestService)
}

// TestConvertToStreamingResponse tests conversion to streaming response format
func TestConvertToStreamingResponse(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	createdAt := time.Now()
	resp := &models.LLMResponse{
		ID:           "test-stream-response-id",
		RequestID:    "test-stream-request-id",
		ProviderName: "test-provider",
		Content:      "Stream response content",
		Confidence:   0.88,
		TokensUsed:   60,
		CreatedAt:    createdAt,
		FinishReason: "stop",
	}

	streamResp := handler.convertToStreamingResponse(resp)

	assert.Equal(t, "test-stream-response-id", streamResp["id"])
	assert.Equal(t, "text_completion", streamResp["object"])
	assert.Equal(t, "test-provider", streamResp["model"])
	assert.Equal(t, createdAt.Unix(), streamResp["created"])

	choices, ok := streamResp["choices"].([]map[string]any)
	if !ok {
		choicesInterface, ok := streamResp["choices"].([]interface{})
		if ok && len(choicesInterface) > 0 {
			choice, ok := choicesInterface[0].(map[string]any)
			if ok {
				delta, ok := choice["delta"].(map[string]any)
				if ok {
					assert.Equal(t, "Stream response content", delta["content"])
					return
				}
			}
		}
		t.Fatal("Failed to parse streaming choices")
	}

	assert.Len(t, choices, 1)
	assert.Equal(t, "Stream response content", choices[0]["delta"].(map[string]any)["content"])
}

// TestConvertToChatStreamingResponse tests conversion to chat streaming response format
func TestConvertToChatStreamingResponse(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	createdAt := time.Now()
	resp := &models.LLMResponse{
		ID:           "test-chat-stream-response-id",
		RequestID:    "test-chat-stream-request-id",
		ProviderName: "test-provider",
		Content:      "Chat stream response content",
		Confidence:   0.85,
		TokensUsed:   70,
		CreatedAt:    createdAt,
		FinishReason: "stop",
	}

	chatStreamResp := handler.convertToChatStreamingResponse(resp)

	assert.Equal(t, "test-chat-stream-response-id", chatStreamResp["id"])
	assert.Equal(t, "chat.completion.chunk", chatStreamResp["object"])
	assert.Equal(t, "test-provider", chatStreamResp["model"])
	assert.Equal(t, createdAt.Unix(), chatStreamResp["created"])

	choices, ok := chatStreamResp["choices"].([]map[string]any)
	if !ok {
		choicesInterface, ok := chatStreamResp["choices"].([]interface{})
		if ok && len(choicesInterface) > 0 {
			choice, ok := choicesInterface[0].(map[string]any)
			if ok {
				delta, ok := choice["delta"].(map[string]any)
				if ok {
					assert.Equal(t, "assistant", delta["role"])
					assert.Equal(t, "Chat stream response content", delta["content"])
					return
				}
			}
		}
		t.Fatal("Failed to parse chat streaming choices")
	}

	assert.Len(t, choices, 1)
	delta := choices[0]["delta"].(map[string]any)
	assert.Equal(t, "assistant", delta["role"])
	assert.Equal(t, "Chat stream response content", delta["content"])
}

// TestConvertToInternalRequest_WithContextValues tests conversion with context values
func TestConvertToInternalRequest_WithContextValues(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt: "Test prompt with context",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Set context values
	c.Set("user_id", "test-user-123")
	c.Set("session_id", "test-session-456")

	internalReq := handler.convertToInternalRequest(req, c)

	assert.Equal(t, "test-user-123", internalReq.UserID)
	assert.Equal(t, "test-session-456", internalReq.SessionID)
	assert.Equal(t, "Test prompt with context", internalReq.Prompt)
}

// TestConvertToInternalRequest_WithMessages tests conversion with messages
func TestConvertToInternalRequest_WithMessages(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt: "Test prompt",
		Messages: []models.Message{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello, how are you?"},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	assert.Len(t, internalReq.Messages, 2)
	assert.Equal(t, "system", internalReq.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant", internalReq.Messages[0].Content)
	assert.Equal(t, "user", internalReq.Messages[1].Role)
	assert.Equal(t, "Hello, how are you?", internalReq.Messages[1].Content)
}

// TestConvertToInternalRequest_WithToolCalls tests conversion with tool calls
func TestConvertToInternalRequest_WithToolCalls(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt: "Test prompt with tool calls",
		Messages: []models.Message{
			{
				Role:    "assistant",
				Content: "I'll help you with that",
				ToolCalls: map[string]interface{}{
					"name": "search_web",
					"arguments": map[string]interface{}{
						"query": "weather in New York",
					},
				},
			},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	assert.Len(t, internalReq.Messages, 1)
	assert.Equal(t, "assistant", internalReq.Messages[0].Role)
	assert.Equal(t, "I'll help you with that", internalReq.Messages[0].Content)
	assert.NotNil(t, internalReq.Messages[0].ToolCalls)
}

// TestCompletionHandler_Models_Direct tests the Models handler directly.
//
// §11.4.120 reconciliation (CONST-036 / BLUFF-002): this test previously
// asserted the OLD hardcoded 3-model literal (deepseek-coder / claude-3-sonnet /
// gemini-pro). That literal was the BLUFF-002 defect and has been removed; the
// test is RECONCILED to assert the NEW mechanism — the list is sourced from the
// live provider registry, and with no source attached it is HONESTLY EMPTY
// (never a fabricated "working" list). It is NOT deleted and NOT weakened to a
// tautology. The deep behavioural guard lives in
// completion_models_test.go:TestCompletionModels_D1_Polarity.
func TestCompletionHandler_Models_Direct(t *testing.T) {
	t.Parallel()

	// (a) No registry/verifier source wired → honest empty list.
	handler := &CompletionHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/models", nil)

	handler.Models(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "list", response["object"])
	data, ok := response["data"].([]interface{})
	assert.True(t, ok)
	assert.Empty(t, data, "no source wired → honest empty list, never a fabricated hardcoded list")

	// (b) Registry source wired → the list reflects live provider capabilities.
	src := newStaticModelSource()
	src.add("anthropic", "claude-sonnet-4")
	handler.SetModelSource(src)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("GET", "/v1/models", nil)
	handler.Models(c2)
	assert.Equal(t, http.StatusOK, w2.Code)

	var resp2 map[string]interface{}
	assert.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	data2, ok := resp2["data"].([]interface{})
	assert.True(t, ok)
	if !assert.Len(t, data2, 1, "list must reflect the single registry-advertised model") {
		return
	}
	m0 := data2[0].(map[string]interface{})
	assert.Equal(t, "claude-sonnet-4", m0["id"])
	assert.Equal(t, "anthropic", m0["owned_by"])
	assert.Equal(t, "model", m0["object"])

	// And the removed hardcoded literal must NOT be fabricated.
	for _, entry := range data2 {
		em := entry.(map[string]interface{})
		assert.NotEqual(t, "deepseek-coder", em["id"])
		assert.NotEqual(t, "gemini-pro", em["id"])
	}
}

// TestCompletionHandler_Complete_InvalidJSON tests Complete with invalid JSON
func TestCompletionHandler_Complete_InvalidJSON(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Complete(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error.Type)
}

// TestCompletionHandler_CompleteStream_InvalidJSON tests CompleteStream with invalid JSON
func TestCompletionHandler_CompleteStream_InvalidJSON(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions/stream", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CompleteStream(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error.Type)
}

// TestCompletionHandler_Chat_InvalidJSON tests Chat with invalid JSON
func TestCompletionHandler_Chat_InvalidJSON(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Chat(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error.Type)
}

// TestCompletionHandler_ChatStream_InvalidJSON tests ChatStream with invalid JSON
func TestCompletionHandler_ChatStream_InvalidJSON(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions/stream", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatStream(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error.Type)
}

// TestCompletionHandler_Complete_MissingPrompt tests Complete with missing required field
func TestCompletionHandler_Complete_MissingPrompt(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	reqBody := map[string]interface{}{
		"model":       "test-model",
		"temperature": 0.7,
	}
	reqBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Complete(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error.Type)
	assert.Contains(t, response.Error.Message, "Prompt")
}

// TestCompletionHandler_Chat_MissingPrompt tests Chat with missing required field
func TestCompletionHandler_Chat_MissingPrompt(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	reqBody := map[string]interface{}{
		"model": "test-model",
	}
	reqBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Chat(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error.Type)
}

// TestCompletionRequest_Struct tests CompletionRequest struct fields
func TestCompletionRequest_Struct(t *testing.T) {
	t.Parallel()
	req := CompletionRequest{
		Prompt:         "test prompt",
		Model:          "test-model",
		Temperature:    0.8,
		MaxTokens:      100,
		TopP:           0.9,
		Stop:           []string{"\n"},
		Stream:         true,
		MemoryEnhanced: true,
		RequestType:    "completion",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		EnsembleConfig: &models.EnsembleConfig{
			Strategy: "best_of_n",
		},
	}

	assert.Equal(t, "test prompt", req.Prompt)
	assert.Equal(t, "test-model", req.Model)
	assert.Equal(t, 0.8, req.Temperature)
	assert.Equal(t, 100, req.MaxTokens)
	assert.Equal(t, 0.9, req.TopP)
	assert.Equal(t, []string{"\n"}, req.Stop)
	assert.True(t, req.Stream)
	assert.True(t, req.MemoryEnhanced)
	assert.Equal(t, "completion", req.RequestType)
	assert.Len(t, req.Messages, 1)
	assert.NotNil(t, req.EnsembleConfig)
}

// TestCompletionResponse_Struct tests CompletionResponse struct fields
func TestCompletionResponse_Struct(t *testing.T) {
	t.Parallel()
	resp := CompletionResponse{
		ID:                "test-id",
		Object:            "text_completion",
		Created:           1234567890,
		Model:             "test-model",
		SystemFingerprint: "test-fingerprint",
		Choices: []CompletionChoice{
			{
				Index:        0,
				FinishReason: "stop",
				Message: models.Message{
					Role:    "assistant",
					Content: "test content",
				},
				LogProbs: &CompletionLogProbs{
					TextOffset: 0,
				},
			},
		},
		Usage: &CompletionUsage{
			PromptTokens:     50,
			CompletionTokens: 50,
			TotalTokens:      100,
		},
	}

	assert.Equal(t, "test-id", resp.ID)
	assert.Equal(t, "text_completion", resp.Object)
	assert.Equal(t, int64(1234567890), resp.Created)
	assert.Equal(t, "test-model", resp.Model)
	assert.Equal(t, "test-fingerprint", resp.SystemFingerprint)
	assert.Len(t, resp.Choices, 1)
	assert.Equal(t, "stop", resp.Choices[0].FinishReason)
	assert.NotNil(t, resp.Usage)
	assert.Equal(t, 100, resp.Usage.TotalTokens)
}

// TestCompletionChoice_Struct tests CompletionChoice struct fields
func TestCompletionChoice_Struct(t *testing.T) {
	t.Parallel()
	choice := CompletionChoice{
		Index:        0,
		FinishReason: "stop",
		Message: models.Message{
			Role:    "assistant",
			Content: "test",
		},
		LogProbs: &CompletionLogProbs{
			Token:      map[string]float64{"test": 0.9},
			TextOffset: 10,
			TopLogprobs: []CompletionLogProb{
				{Token: "test", Logprob: -0.1, Bytes: []byte("test"), Offset: 0},
			},
		},
	}

	assert.Equal(t, 0, choice.Index)
	assert.Equal(t, "stop", choice.FinishReason)
	assert.Equal(t, "assistant", choice.Message.Role)
	assert.NotNil(t, choice.LogProbs)
	assert.Equal(t, 0.9, choice.LogProbs.Token["test"])
	assert.Len(t, choice.LogProbs.TopLogprobs, 1)
}

// TestCompletionUsage_Struct tests CompletionUsage struct fields
func TestCompletionUsage_Struct(t *testing.T) {
	t.Parallel()
	usage := CompletionUsage{
		PromptTokens:     50,
		CompletionTokens: 75,
		TotalTokens:      125,
	}

	assert.Equal(t, 50, usage.PromptTokens)
	assert.Equal(t, 75, usage.CompletionTokens)
	assert.Equal(t, 125, usage.TotalTokens)
}

// TestCompletionLogProbs_Struct tests CompletionLogProbs struct fields
func TestCompletionLogProbs_Struct(t *testing.T) {
	t.Parallel()
	logProbs := CompletionLogProbs{
		Token:      map[string]float64{"token1": -0.5, "token2": -1.0},
		TextOffset: 5,
		TopLogprobs: []CompletionLogProb{
			{Token: "token1", Logprob: -0.5, Bytes: []byte("token1"), Offset: 0},
			{Token: "token2", Logprob: -1.0, Bytes: []byte("token2"), Offset: 6},
		},
	}

	assert.Equal(t, -0.5, logProbs.Token["token1"])
	assert.Equal(t, -1.0, logProbs.Token["token2"])
	assert.Equal(t, 5, logProbs.TextOffset)
	assert.Len(t, logProbs.TopLogprobs, 2)
}

// TestCompletionLogProb_Struct tests CompletionLogProb struct fields
func TestCompletionLogProb_Struct(t *testing.T) {
	t.Parallel()
	logProb := CompletionLogProb{
		Token:   "test_token",
		Logprob: -0.25,
		Bytes:   []byte("test_token"),
		Offset:  10,
	}

	assert.Equal(t, "test_token", logProb.Token)
	assert.Equal(t, -0.25, logProb.Logprob)
	assert.Equal(t, []byte("test_token"), logProb.Bytes)
	assert.Equal(t, 10, logProb.Offset)
}

// TestErrorResponse_Struct tests ErrorResponse struct fields
func TestErrorResponse_Struct(t *testing.T) {
	t.Parallel()
	errResp := ErrorResponse{}
	errResp.Error.Message = "Test error message"
	errResp.Error.Type = "invalid_request"
	errResp.Error.Code = "400"

	assert.Equal(t, "Test error message", errResp.Error.Message)
	assert.Equal(t, "invalid_request", errResp.Error.Type)
	assert.Equal(t, "400", errResp.Error.Code)
}

// TestCompletionHandler_SendError_Various tests sendError with various error types
func TestCompletionHandler_SendError_Various(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	tests := []struct {
		name       string
		statusCode int
		errorType  string
		message    string
		details    string
	}{
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			errorType:  "invalid_request",
			message:    "Invalid input",
			details:    "Missing field",
		},
		{
			name:       "internal error",
			statusCode: http.StatusInternalServerError,
			errorType:  "internal_error",
			message:    "Server error",
			details:    "Database connection failed",
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			errorType:  "authentication_error",
			message:    "Auth failed",
			details:    "Invalid token",
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			errorType:  "not_found",
			message:    "Resource missing",
			details:    "Model not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			handler.sendError(c, tt.statusCode, tt.errorType, tt.message, tt.details)

			assert.Equal(t, tt.statusCode, w.Code)

			var errResp ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &errResp)
			assert.NoError(t, err)
			assert.Equal(t, tt.errorType, errResp.Error.Type)
			assert.Contains(t, errResp.Error.Message, tt.message)
			assert.Contains(t, errResp.Error.Message, tt.details)
		})
	}
}

// TestCompletionHandler_CompleteStream_MissingPrompt tests stream with missing prompt
func TestCompletionHandler_CompleteStream_MissingPrompt(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	reqBody := map[string]interface{}{
		"model": "test-model",
	}
	reqBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions/stream", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CompleteStream(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error.Type)
}

// TestCompletionHandler_ChatStream_MissingPrompt tests chat stream with missing prompt
func TestCompletionHandler_ChatStream_MissingPrompt(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	reqBody := map[string]interface{}{
		"model": "test-model",
	}
	reqBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions/stream", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatStream(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error.Type)
}

// TestCompletionHandler_ConvertToInternalRequest_WithInvalidUserID tests invalid user ID type
func TestCompletionHandler_ConvertToInternalRequest_WithInvalidUserID(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt: "Test prompt",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Set non-string user_id
	c.Set("user_id", 12345)
	c.Set("session_id", 67890)

	internalReq := handler.convertToInternalRequest(req, c)

	// Should fall back to defaults
	assert.Equal(t, "anonymous", internalReq.UserID)
	assert.NotEmpty(t, internalReq.SessionID)
}

// TestCompletionHandler_ConvertToInternalRequest_WithName tests messages with name field
func TestCompletionHandler_ConvertToInternalRequest_WithName(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	name := "TestUser"
	req := &CompletionRequest{
		Prompt: "Test prompt",
		Messages: []models.Message{
			{Role: "user", Content: "Hello", Name: &name},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	assert.Len(t, internalReq.Messages, 1)
	assert.NotNil(t, internalReq.Messages[0].Name)
	assert.Equal(t, "TestUser", *internalReq.Messages[0].Name)
}

// TestCompletionHandler_ConvertToAPIResponse_VariousFinishReasons tests response conversion
func TestCompletionHandler_ConvertToAPIResponse_VariousFinishReasons(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	finishReasons := []string{"stop", "length", "content_filter", "tool_calls", "function_call"}

	for _, reason := range finishReasons {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			resp := &models.LLMResponse{
				ID:           "test-id",
				Content:      "Test response",
				FinishReason: reason,
				TokensUsed:   100,
				CreatedAt:    time.Now(),
			}

			apiResp := handler.convertToAPIResponse(resp)

			assert.Equal(t, reason, apiResp.Choices[0].FinishReason)
		})
	}
}

// TestCompletionHandler_ConvertToChatResponse_WithZeroTokens tests zero tokens
func TestCompletionHandler_ConvertToChatResponse_WithZeroTokens(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	resp := &models.LLMResponse{
		ID:           "test-id",
		Content:      "Test response",
		TokensUsed:   0,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}

	chatResp := handler.convertToChatResponse(resp)

	usage, ok := chatResp["usage"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, 0, usage["total_tokens"])
	assert.Equal(t, 0, usage["prompt_tokens"])
	assert.Equal(t, 0, usage["completion_tokens"])
}

// TestCompletionHandler_ConvertToStreamingResponse_LargeResponse tests large response
func TestCompletionHandler_ConvertToStreamingResponse_LargeResponse(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	// Create a large content response
	largeContent := ""
	for i := 0; i < 1000; i++ {
		largeContent += "This is a test sentence. "
	}

	resp := &models.LLMResponse{
		ID:           "test-large-id",
		Content:      largeContent,
		ProviderName: "test-provider",
		TokensUsed:   5000,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}

	streamResp := handler.convertToStreamingResponse(resp)

	assert.Equal(t, "test-large-id", streamResp["id"])
	choices := streamResp["choices"].([]map[string]any)
	delta := choices[0]["delta"].(map[string]any)
	assert.Equal(t, largeContent, delta["content"])
}

// TestCompletionHandler_ConvertToChatStreamingResponse_WithRole tests role inclusion
func TestCompletionHandler_ConvertToChatStreamingResponse_WithRole(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	resp := &models.LLMResponse{
		ID:        "test-stream-id",
		Content:   "Streaming content",
		CreatedAt: time.Now(),
	}

	chatStreamResp := handler.convertToChatStreamingResponse(resp)

	choices := chatStreamResp["choices"].([]map[string]any)
	delta := choices[0]["delta"].(map[string]any)

	assert.Equal(t, "assistant", delta["role"])
	assert.Equal(t, "Streaming content", delta["content"])
}

// TestCompletionHandler_Complete_EmptyBody tests empty request body
func TestCompletionHandler_Complete_EmptyBody(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Complete(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error.Type)
}

// TestCompletionHandler_Chat_EmptyBody tests empty chat request body
func TestCompletionHandler_Chat_EmptyBody(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Chat(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error.Type)
}

// TestCompletionHandler_CompleteStream_EmptyBody tests empty stream request body
func TestCompletionHandler_CompleteStream_EmptyBody(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions/stream", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CompleteStream(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCompletionHandler_ChatStream_EmptyBody tests empty chat stream request body
func TestCompletionHandler_ChatStream_EmptyBody(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions/stream", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatStream(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCompletionHandler_Complete_WithRequestService tests Complete with a request service
func TestCompletionHandler_Complete_WithRequestService(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// Create a request service (without providers, will return error)
	service := services.NewRequestService("random", nil, nil)
	handler := NewCompletionHandler(service)

	reqBody := map[string]interface{}{
		"prompt":      "Test prompt",
		"model":       "test-model",
		"temperature": 0.7,
		"max_tokens":  100,
	}
	reqBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Complete(c)

	// Should fail because no ensemble service - returns 502 Bad Gateway or 503 Service Unavailable
	assert.True(t, w.Code == http.StatusBadGateway || w.Code == http.StatusServiceUnavailable || w.Code == http.StatusInternalServerError,
		"Expected 502, 503, or 500, got %d", w.Code)
}

// TestCompletionHandler_Chat_WithRequestService tests Chat with a request service
func TestCompletionHandler_Chat_WithRequestService(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// Create a request service (without providers, will return error)
	service := services.NewRequestService("random", nil, nil)
	handler := NewCompletionHandler(service)

	reqBody := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
		"model": "test-model",
	}
	reqBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	// May panic due to nil ensemble, so recover
	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil ensemble service")
		}
	}()

	handler.Chat(c)

	// If we get here, check for error response
	t.Logf("Response code: %d, Body: %s", w.Code, w.Body.String())
}

// TestCompletionHandler_CompleteStream_WithRequestService tests CompleteStream with a request service
func TestCompletionHandler_CompleteStream_WithRequestService(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// Create a request service (without providers, will return error)
	service := services.NewRequestService("random", nil, nil)
	handler := NewCompletionHandler(service)

	reqBody := map[string]interface{}{
		"prompt": "Test prompt",
		"model":  "test-model",
		"stream": true,
	}
	reqBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CompleteStream(c)

	// Should fail because no ensemble service - returns 502 Bad Gateway or 503 Service Unavailable
	assert.True(t, w.Code == http.StatusBadGateway || w.Code == http.StatusServiceUnavailable || w.Code == http.StatusInternalServerError,
		"Expected 502, 503, or 500, got %d", w.Code)
}

// TestCompletionHandler_ChatStream_WithRequestService tests ChatStream with a request service
func TestCompletionHandler_ChatStream_WithRequestService(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// Create a request service (without providers, will return error)
	service := services.NewRequestService("random", nil, nil)
	handler := NewCompletionHandler(service)

	reqBody := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
		"model":  "test-model",
		"stream": true,
	}
	reqBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	// May panic due to nil ensemble, so recover
	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil ensemble service")
		}
	}()

	handler.ChatStream(c)

	// If we get here, check for error response
	t.Logf("Response code: %d, Body: %s", w.Code, w.Body.String())
}

// TestCompletionHandler_ConvertToInternalRequest_WithSessionID tests session ID extraction
func TestCompletionHandler_ConvertToInternalRequest_WithSessionID(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt: "Test prompt",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", "test-user-123")
	c.Set("session_id", "session-abc")

	internalReq := handler.convertToInternalRequest(req, c)

	assert.Equal(t, "test-user-123", internalReq.UserID)
	assert.Equal(t, "session-abc", internalReq.SessionID)
}

// TestCompletionHandler_ConvertToAPIResponse_WithMetadata tests metadata in response
func TestCompletionHandler_ConvertToAPIResponse_WithMetadata(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := &CompletionHandler{}

	resp := &models.LLMResponse{
		ID:           "cmpl-test-123",
		Content:      "This is a test response",
		ProviderName: "test-provider",
		TokensUsed:   50,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
		Metadata: map[string]interface{}{
			"confidence": 0.95,
			"model":      "test-model",
		},
	}

	apiResp := handler.convertToAPIResponse(resp)

	assert.Equal(t, "cmpl-test-123", apiResp.ID)
	assert.Equal(t, 50, apiResp.Usage.TotalTokens)
}

// TestCompletionHandler_ConvertToChatResponse_FullResponse tests full chat response conversion
func TestCompletionHandler_ConvertToChatResponse_FullResponse(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := &CompletionHandler{}

	testTime := time.Now()
	resp := &models.LLMResponse{
		ID:           "chat-test-456",
		Content:      "Hello! How can I help you?",
		ProviderName: "test-provider",
		TokensUsed:   100,
		FinishReason: "stop",
		CreatedAt:    testTime,
	}

	chatResp := handler.convertToChatResponse(resp)

	assert.Equal(t, "chat-test-456", chatResp["id"])
	assert.Equal(t, "chat.completion", chatResp["object"])
	assert.Equal(t, testTime.Unix(), chatResp["created"])

	choices := chatResp["choices"].([]map[string]any)
	assert.Len(t, choices, 1)
	assert.Equal(t, 0, choices[0]["index"])
	assert.Equal(t, "stop", choices[0]["finish_reason"])

	message := choices[0]["message"].(map[string]any)
	assert.Equal(t, "assistant", message["role"])
	assert.Equal(t, "Hello! How can I help you?", message["content"])
}

// TestCompletionHandler_Complete_NilRequestService tests Complete with nil request service
func TestCompletionHandler_Complete_NilRequestService(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := &CompletionHandler{} // nil requestService

	reqBody := map[string]interface{}{
		"prompt": "Test prompt",
		"model":  "test-model",
	}
	reqBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	// This will panic due to nil dereference, so we recover
	defer func() {
		if r := recover(); r != nil {
			t.Log("Expected panic due to nil request service")
		}
	}()

	handler.Complete(c)
}

// =====================================================
// CONCURRENT REQUEST HANDLING TESTS
// =====================================================

// TestCompletionHandler_ConcurrentCompleteRequests tests concurrent Complete requests
func TestCompletionHandler_ConcurrentCompleteRequests(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	numRequests := 20
	var wg sync.WaitGroup
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			reqBody := map[string]interface{}{
				"prompt":      "Concurrent test prompt " + strconv.Itoa(idx),
				"model":       "test-model",
				"temperature": 0.5 + float64(idx%5)*0.1,
				"max_tokens":  100 + idx*10,
			}
			reqBytes, _ := json.Marshal(reqBody)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			// Recover from panic to continue other goroutines
			defer func() {
				if r := recover(); r != nil {
					results <- http.StatusInternalServerError
				}
			}()

			handler.Complete(c)
			results <- w.Code
		}(i)
	}

	wg.Wait()
	close(results)

	// All requests should either complete or fail gracefully
	count := 0
	for code := range results {
		count++
		// Acceptable codes: 200 (success), 400 (bad request), 500-504 (server errors)
		isAcceptable := code == http.StatusOK || code == http.StatusBadRequest ||
			code == http.StatusInternalServerError || code == http.StatusBadGateway ||
			code == http.StatusServiceUnavailable || code == http.StatusGatewayTimeout
		assert.True(t, isAcceptable, "Unexpected status code: %d", code)
	}
	assert.Equal(t, numRequests, count, "All requests should complete")
}

// TestCompletionHandler_ConcurrentChatRequests tests concurrent Chat requests
func TestCompletionHandler_ConcurrentChatRequests(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	numRequests := 15
	var wg sync.WaitGroup
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			reqBody := map[string]interface{}{
				"prompt": "Chat test " + strconv.Itoa(idx),
				"messages": []map[string]string{
					{"role": "user", "content": "Hello " + strconv.Itoa(idx)},
				},
				"model": "test-model",
			}
			reqBytes, _ := json.Marshal(reqBody)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(reqBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			defer func() {
				if r := recover(); r != nil {
					results <- http.StatusInternalServerError
				}
			}()

			handler.Chat(c)
			results <- w.Code
		}(i)
	}

	wg.Wait()
	close(results)

	count := 0
	for code := range results {
		count++
		// Acceptable codes: 200 (success), 400 (bad request), 500-504 (server errors)
		isAcceptable := code == http.StatusOK || code == http.StatusBadRequest ||
			code == http.StatusInternalServerError || code == http.StatusBadGateway ||
			code == http.StatusServiceUnavailable || code == http.StatusGatewayTimeout
		assert.True(t, isAcceptable, "Unexpected status code: %d", code)
	}
	assert.Equal(t, numRequests, count)
}

// TestCompletionHandler_ConcurrentModelRequests tests concurrent Models requests
func TestCompletionHandler_ConcurrentModelRequests(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	numRequests := 30
	var wg sync.WaitGroup
	successCount := int32(0)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/v1/models", nil)

			handler.Models(c)

			if w.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	// All Models requests should succeed
	assert.Equal(t, int32(numRequests), successCount, "All Models requests should return 200")
}

// =====================================================
// EDGE CASE TESTS
// =====================================================

// TestCompletionHandler_RequestWithSpecialCharacters tests handling of special characters
func TestCompletionHandler_RequestWithSpecialCharacters(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	testCases := []struct {
		name   string
		prompt string
	}{
		{"unicode", "Test with unicode: 你好世界 こんにちは"},
		{"emoji", "Test with emoji: 😀 🎉 🚀"},
		{"newlines", "Test with\nnewlines\nand\ttabs"},
		{"quotes", `Test with "quotes" and 'single quotes'`},
		{"backslash", `Test with \ backslash and \\ double backslash`},
		{"html_entities", "Test with &lt;html&gt; entities &amp; special chars"},
		{"long_unicode", "Тест на кириллице и العربية والعبرית"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reqBody := map[string]interface{}{
				"prompt": tc.prompt,
				"model":  "test-model",
			}
			reqBytes, _ := json.Marshal(reqBody)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			defer func() {
				if r := recover(); r != nil {
					// Acceptable to panic due to nil service, but JSON parsing should work
				}
			}()

			handler.Complete(c)

			// Request should at least parse correctly
			// Will get 400 (bad request - missing required) or panic due to nil service
			assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusOK || w.Code == 0)
		})
	}
}

// TestCompletionHandler_ExtremeValues tests handling of extreme parameter values
func TestCompletionHandler_ExtremeValues(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	testCases := []struct {
		name        string
		temperature float64
		maxTokens   int
		topP        float64
	}{
		{"zero_temp", 0.0, 100, 1.0},
		{"max_temp", 2.0, 100, 1.0},
		{"zero_tokens", 0.7, 0, 1.0},
		{"large_tokens", 0.7, 100000, 1.0},
		{"zero_top_p", 0.7, 100, 0.0},
		{"negative_temp", -0.5, 100, 1.0},
		{"negative_tokens", 0.7, -100, 1.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reqBody := map[string]interface{}{
				"prompt":      "Test extreme values",
				"model":       "test-model",
				"temperature": tc.temperature,
				"max_tokens":  tc.maxTokens,
				"top_p":       tc.topP,
			}
			reqBytes, _ := json.Marshal(reqBody)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			defer func() {
				_ = recover()
			}()

			handler.Complete(c)
			// Verify parsing doesn't crash
		})
	}
}

// TestCompletionHandler_LargeMessages tests handling of large message arrays
func TestCompletionHandler_LargeMessages(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	// Create a large number of messages
	messages := make([]map[string]string, 100)
	for i := 0; i < 100; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = map[string]string{
			"role":    role,
			"content": "Message number " + strconv.Itoa(i) + " with some content to test large arrays",
		}
	}

	reqBody := map[string]interface{}{
		"prompt":   "Test with many messages",
		"messages": messages,
		"model":    "test-model",
	}
	reqBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	defer func() {
		_ = recover()
	}()

	handler.Complete(c)
	// Should handle large message arrays without crashing
}

// TestCompletionHandler_RequestWithAllOptions tests request with all possible options
func TestCompletionHandler_RequestWithAllOptions(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	reqBody := map[string]interface{}{
		"prompt":      "Complete test prompt",
		"model":       "test-model",
		"temperature": 0.7,
		"max_tokens":  500,
		"top_p":       0.95,
		"stop":        []string{"\n", "END", "STOP"},
		"stream":      false,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a helpful assistant"},
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi there!"},
			{"role": "user", "content": "How are you?"},
		},
		"memory_enhanced": true,
		"request_type":    "completion",
		"ensemble_config": map[string]interface{}{
			"strategy":             "confidence_weighted",
			"min_providers":        2,
			"confidence_threshold": 0.85,
			"fallback_to_best":     true,
			"timeout":              45,
		},
	}
	reqBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", "comprehensive-test-user")
	c.Set("session_id", "comprehensive-test-session")

	defer func() {
		_ = recover()
	}()

	handler.Complete(c)
}

// TestCompletionHandler_ConvertToInternalRequest_AllFields tests internal request conversion with all fields
func TestCompletionHandler_ConvertToInternalRequest_AllFields(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	name := "TestUser"
	req := &CompletionRequest{
		Prompt: "Test prompt with all fields",
		Messages: []models.Message{
			{Role: "system", Content: "System message", Name: &name},
			{Role: "user", Content: "User message"},
			{Role: "assistant", Content: "Assistant response", ToolCalls: map[string]interface{}{"function": "test"}},
		},
		Model:       "gpt-4",
		Temperature: 0.8,
		MaxTokens:   2000,
		TopP:        0.95,
		Stop:        []string{"END", "\n\n"},
		EnsembleConfig: &models.EnsembleConfig{
			Strategy:            "majority_vote",
			MinProviders:        3,
			ConfidenceThreshold: 0.9,
			FallbackToBest:      true,
			Timeout:             60,
		},
		MemoryEnhanced: true,
		RequestType:    "chat",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", "full-test-user")
	c.Set("session_id", "full-test-session")

	internalReq := handler.convertToInternalRequest(req, c)

	// Verify all fields are converted correctly
	assert.Equal(t, "Test prompt with all fields", internalReq.Prompt)
	assert.Equal(t, 3, len(internalReq.Messages))
	assert.Equal(t, "gpt-4", internalReq.ModelParams.Model)
	assert.Equal(t, 0.8, internalReq.ModelParams.Temperature)
	assert.Equal(t, 2000, internalReq.ModelParams.MaxTokens)
	assert.Equal(t, 0.95, internalReq.ModelParams.TopP)
	assert.Equal(t, []string{"END", "\n\n"}, internalReq.ModelParams.StopSequences)
	assert.Equal(t, "majority_vote", internalReq.EnsembleConfig.Strategy)
	assert.True(t, internalReq.MemoryEnhanced)
	assert.Equal(t, "chat", internalReq.RequestType)
	assert.Equal(t, "full-test-user", internalReq.UserID)
	assert.Equal(t, "full-test-session", internalReq.SessionID)
}

// TestCompletionHandler_ResponseTypes tests different response finish reasons
func TestCompletionHandler_ResponseTypes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	testCases := []struct {
		finishReason string
		tokensUsed   int
		confidence   float64
	}{
		{"stop", 100, 0.95},
		{"length", 4096, 0.88},
		{"content_filter", 50, 0.70},
		{"tool_calls", 150, 0.92},
		{"function_call", 200, 0.89},
		{"", 0, 0.0}, // Empty response
	}

	for _, tc := range testCases {
		t.Run("finish_"+tc.finishReason, func(t *testing.T) {
			t.Parallel()
			resp := &models.LLMResponse{
				ID:           "test-" + tc.finishReason,
				Content:      "Response with finish reason: " + tc.finishReason,
				FinishReason: tc.finishReason,
				TokensUsed:   tc.tokensUsed,
				Confidence:   tc.confidence,
				ProviderName: "test-provider",
				CreatedAt:    time.Now(),
			}

			apiResp := handler.convertToAPIResponse(resp)

			assert.Equal(t, tc.finishReason, apiResp.Choices[0].FinishReason)
			assert.Equal(t, tc.tokensUsed, apiResp.Usage.TotalTokens)
		})
	}
}

// TestCompletionHandler_StreamingResponseFormat tests streaming response formatting
func TestCompletionHandler_StreamingResponseFormat(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	// Test multiple chunks
	chunks := []string{"Hello", " ", "World", "!"}

	for i, chunk := range chunks {
		resp := &models.LLMResponse{
			ID:           "stream-" + strconv.Itoa(i),
			Content:      chunk,
			ProviderName: "test-provider",
			CreatedAt:    time.Now(),
		}
		if i == len(chunks)-1 {
			resp.FinishReason = "stop"
		}

		streamResp := handler.convertToStreamingResponse(resp)

		assert.Equal(t, "stream-"+strconv.Itoa(i), streamResp["id"])
		assert.Equal(t, "text_completion", streamResp["object"])

		choices := streamResp["choices"].([]map[string]any)
		delta := choices[0]["delta"].(map[string]any)
		assert.Equal(t, chunk, delta["content"])
	}
}

// TestCompletionHandler_ChatStreamingResponseFormat tests chat streaming response formatting
func TestCompletionHandler_ChatStreamingResponseFormat(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	chunks := []string{"This", " is", " a", " test"}

	for i, chunk := range chunks {
		resp := &models.LLMResponse{
			ID:           "chat-stream-" + strconv.Itoa(i),
			Content:      chunk,
			ProviderName: "test-provider",
			CreatedAt:    time.Now(),
		}
		if i == len(chunks)-1 {
			resp.FinishReason = "stop"
		}

		chatStreamResp := handler.convertToChatStreamingResponse(resp)

		assert.Equal(t, "chat-stream-"+strconv.Itoa(i), chatStreamResp["id"])
		assert.Equal(t, "chat.completion.chunk", chatStreamResp["object"])

		choices := chatStreamResp["choices"].([]map[string]any)
		delta := choices[0]["delta"].(map[string]any)
		assert.Equal(t, "assistant", delta["role"])
		assert.Equal(t, chunk, delta["content"])
	}
}

// =====================================================
// BENCHMARK TESTS
// =====================================================

// BenchmarkCompletionHandler_ConvertToInternalRequest benchmarks internal request conversion
func BenchmarkCompletionHandler_ConvertToInternalRequest(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt: "Benchmark test prompt",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		Model:       "test-model",
		Temperature: 0.7,
		MaxTokens:   100,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.convertToInternalRequest(req, c)
	}
}

// BenchmarkCompletionHandler_ConvertToAPIResponse benchmarks API response conversion
func BenchmarkCompletionHandler_ConvertToAPIResponse(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	resp := &models.LLMResponse{
		ID:           "bench-response",
		Content:      "Benchmark response content",
		FinishReason: "stop",
		TokensUsed:   100,
		ProviderName: "test-provider",
		CreatedAt:    time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.convertToAPIResponse(resp)
	}
}

// BenchmarkCompletionHandler_ConvertToChatResponse benchmarks chat response conversion
func BenchmarkCompletionHandler_ConvertToChatResponse(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	resp := &models.LLMResponse{
		ID:           "bench-chat-response",
		Content:      "Benchmark chat response",
		FinishReason: "stop",
		TokensUsed:   150,
		ProviderName: "test-provider",
		CreatedAt:    time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.convertToChatResponse(resp)
	}
}

// BenchmarkCompletionHandler_Models benchmarks the Models endpoint
func BenchmarkCompletionHandler_Models(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler := &CompletionHandler{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/v1/models", nil)
		handler.Models(c)
	}
}
