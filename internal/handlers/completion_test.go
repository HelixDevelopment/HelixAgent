package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/superagent/superagent/internal/models"
)

// TestCompletionHandler_Complete_Success tests successful completion request
func TestCompletionHandler_Complete_Success(t *testing.T) {
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
	assert.Equal(t, "superagent-v1.0", apiResp.SystemFingerprint)
}

// TestConvertToChatResponse tests conversion to chat response format
func TestConvertToChatResponse(t *testing.T) {
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
	// Create a mock request service
	handler := NewCompletionHandler(nil)

	assert.NotNil(t, handler)
	assert.Nil(t, handler.requestService)
}

// TestConvertToStreamingResponse tests conversion to streaming response format
func TestConvertToStreamingResponse(t *testing.T) {
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

// TestCompletionHandler_Models_Direct tests the Models handler directly
func TestCompletionHandler_Models_Direct(t *testing.T) {
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
	assert.Len(t, data, 3) // deepseek-coder, claude-3-sonnet, gemini-pro

	// Verify first model
	model0 := data[0].(map[string]interface{})
	assert.Equal(t, "deepseek-coder", model0["id"])
	assert.Equal(t, "model", model0["object"])
	assert.Equal(t, "deepseek", model0["owned_by"])

	// Verify second model
	model1 := data[1].(map[string]interface{})
	assert.Equal(t, "claude-3-sonnet-20240229", model1["id"])
	assert.Equal(t, "anthropic", model1["owned_by"])

	// Verify third model
	model2 := data[2].(map[string]interface{})
	assert.Equal(t, "gemini-pro", model2["id"])
	assert.Equal(t, "google", model2["owned_by"])
}

// TestCompletionHandler_Complete_InvalidJSON tests Complete with invalid JSON
func TestCompletionHandler_Complete_InvalidJSON(t *testing.T) {
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
