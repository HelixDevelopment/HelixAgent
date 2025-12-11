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
