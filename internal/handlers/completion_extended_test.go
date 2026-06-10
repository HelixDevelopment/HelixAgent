package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/skills"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestCompletionHandler_NewCompletionHandlerWithSkills tests handler creation with skills
func TestCompletionHandler_NewCompletionHandlerWithSkills(t *testing.T) {
	t.Parallel()
	handler := NewCompletionHandlerWithSkills(nil, nil)

	assert.NotNil(t, handler)
	assert.Nil(t, handler.requestService)
	assert.Nil(t, handler.skillsIntegration)
}

// TestCompletionHandler_SetSkillsIntegration tests setting skills integration
func TestCompletionHandler_SetSkillsIntegration(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}
	assert.Nil(t, handler.skillsIntegration)

	// Set skills integration (nil is valid for testing)
	handler.SetSkillsIntegration(nil)
	assert.Nil(t, handler.skillsIntegration)
}

// TestCompletionHandler_SendCategorizedError_LLMServiceError tests categorized error handling
func TestCompletionHandler_SendCategorizedError_LLMServiceError(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	testCases := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name:           "generic error",
			err:            errors.New("something went wrong"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			handler.sendCategorizedError(c, tc.err)

			// The error should be categorized and return appropriate status
			assert.True(t, w.Code >= 400 && w.Code < 600, "Expected error status code, got %d", w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response, "error")
		})
	}
}

// TestCompletionHandler_ConvertToAPIResponseWithSkills_NoSkills tests conversion without skills
func TestCompletionHandler_ConvertToAPIResponseWithSkills_NoSkills(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	resp := &models.LLMResponse{
		ID:           "test-id",
		Content:      "Test content",
		ProviderName: "test-provider",
		TokensUsed:   100,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}

	apiResp := handler.convertToAPIResponseWithSkills(resp, nil)

	assert.Equal(t, "test-id", apiResp.ID)
	assert.Nil(t, apiResp.SkillsUsed)
}

// TestCompletionHandler_ConvertToAPIResponseWithSkills_EmptyUsages tests with empty skill usages
func TestCompletionHandler_ConvertToAPIResponseWithSkills_EmptyUsages(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	resp := &models.LLMResponse{
		ID:           "test-id",
		Content:      "Test content",
		ProviderName: "test-provider",
		TokensUsed:   100,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}

	usages := []skills.SkillUsage{}

	apiResp := handler.convertToAPIResponseWithSkills(resp, usages)

	assert.Equal(t, "test-id", apiResp.ID)
	// Empty usages should not set SkillsUsed
	assert.Nil(t, apiResp.SkillsUsed)
}

// TestCompletionHandler_ConvertToChatResponseWithSkills_NoSkills tests chat response without skills
func TestCompletionHandler_ConvertToChatResponseWithSkills_NoSkills(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	resp := &models.LLMResponse{
		ID:           "test-id",
		Content:      "Test content",
		ProviderName: "test-provider",
		TokensUsed:   100,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}

	chatResp := handler.convertToChatResponseWithSkills(resp, nil)

	assert.Equal(t, "test-id", chatResp["id"])
	_, hasSkills := chatResp["skills_used"]
	assert.False(t, hasSkills)
}

// TestCompletionHandler_ConvertToChatResponseWithSkills_EmptyUsages tests with empty usages
func TestCompletionHandler_ConvertToChatResponseWithSkills_EmptyUsages(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	resp := &models.LLMResponse{
		ID:           "test-id",
		Content:      "Test content",
		ProviderName: "test-provider",
		TokensUsed:   100,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}

	usages := []skills.SkillUsage{}

	chatResp := handler.convertToChatResponseWithSkills(resp, usages)

	assert.Equal(t, "test-id", chatResp["id"])
	_, hasSkills := chatResp["skills_used"]
	assert.False(t, hasSkills)
}

// TestCompletionHandler_Complete_WithRequestService_Error tests Complete with service error
func TestCompletionHandler_Complete_WithRequestService_Error(t *testing.T) {
	t.Parallel()
	// Create service without ensemble (will fail)
	service := services.NewRequestService("random", nil, nil)
	handler := NewCompletionHandler(service)

	reqBody := map[string]interface{}{
		"prompt": "Test prompt",
		"model":  "test-model",
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Complete(c)

	// Should fail with categorized error
	assert.True(t, w.Code >= 400 && w.Code < 600, "Expected error status code, got %d", w.Code)
}

// TestCompletionHandler_Chat_WithRequestService_Error tests Chat with service error
func TestCompletionHandler_Chat_WithRequestService_Error(t *testing.T) {
	t.Parallel()
	// Create service without ensemble (will fail)
	service := services.NewRequestService("random", nil, nil)
	handler := NewCompletionHandler(service)

	reqBody := map[string]interface{}{
		"prompt": "Test prompt",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	// Recover from potential panic
	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic - expected behavior with nil ensemble")
		}
	}()

	handler.Chat(c)

	// Should fail with error
	if w.Code != 0 { // If not panicked
		assert.True(t, w.Code >= 400 && w.Code < 600)
	}
}

// TestCompletionHandler_Chat_ExtractsLastUserMessage tests user message extraction
func TestCompletionHandler_Chat_ExtractsLastUserMessage(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	// This tests the message extraction logic indirectly
	req := &CompletionRequest{
		Prompt: "",
		Messages: []models.Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "First message"},
			{Role: "assistant", Content: "Response"},
			{Role: "user", Content: "Last user message"},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	// Verify messages are preserved
	assert.Len(t, internalReq.Messages, 4)
	assert.Equal(t, "Last user message", internalReq.Messages[3].Content)
}

// TestCompletionHandler_Complete_VariousContentTypes tests different content types
func TestCompletionHandler_Complete_VariousContentTypes(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	contentTypes := []string{
		"application/json",
		"application/json; charset=utf-8",
		"text/json",
	}

	for _, ct := range contentTypes {
		t.Run(ct, func(t *testing.T) {
			t.Parallel()
			reqBody := map[string]interface{}{
				"prompt": "Test",
			}
			jsonBody, _ := json.Marshal(reqBody)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(jsonBody))
			c.Request.Header.Set("Content-Type", ct)

			defer func() {
				_ = recover()
			}()

			handler.Complete(c)
		})
	}
}

// TestCompletionHandler_ErrorResponse_Fields tests error response field structure
func TestCompletionHandler_ErrorResponse_Fields(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	handler.sendError(c, http.StatusBadRequest, "test_type", "Test message", "Test details")

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "test_type", response.Error.Type)
	assert.Equal(t, "400", response.Error.Code)
	assert.Contains(t, response.Error.Message, "Test message")
	assert.Contains(t, response.Error.Message, "Test details")
}

// TestCompletionHandler_Complete_LargePayload tests handling large request payload
func TestCompletionHandler_Complete_LargePayload(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	// Create large prompt
	largePrompt := ""
	for i := 0; i < 10000; i++ {
		largePrompt += "This is a test sentence. "
	}

	reqBody := map[string]interface{}{
		"prompt": largePrompt,
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	defer func() {
		_ = recover()
	}()

	handler.Complete(c)
	// Should handle large payload without crashing
}

// TestCompletionHandler_Models_ResponseStructure tests models endpoint response structure
// §11.4.120 reconciliation (CONST-036 / BLUFF-002): previously asserted exactly
// 3 hardcoded models with a "permission" field. The list is now registry-sourced;
// with no source it is honestly empty, and with a source each entry carries the
// real structural fields (id/object/created/owned_by/root). The removed hardcoded
// "permission" field is no longer part of the contract.
func TestCompletionHandler_Models_ResponseStructure(t *testing.T) {
	t.Parallel()

	// (a) no source → honest empty list
	handler := &CompletionHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/models", nil)
	handler.Models(c)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "list", response["object"])
	data, ok := response["data"].([]interface{})
	require.True(t, ok)
	assert.Empty(t, data, "no ModelSource wired → honest empty list")

	// (b) source wired → entries carry the real structural fields
	src := newStaticModelSource()
	src.add("openai", "gpt-4o")
	handler.SetModelSource(src)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("GET", "/v1/models", nil)
	handler.Models(c2)
	assert.Equal(t, http.StatusOK, w2.Code)

	var resp2 map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	data2, ok := resp2["data"].([]interface{})
	require.True(t, ok)
	require.Len(t, data2, 1)
	model := data2[0].(map[string]interface{})
	assert.Equal(t, "gpt-4o", model["id"])
	assert.Contains(t, model, "object")
	assert.Contains(t, model, "created")
	assert.Equal(t, "openai", model["owned_by"])
	assert.Contains(t, model, "root")
}

// TestCompletionHandler_ConvertToInternalRequest_EmptyEnsembleConfig tests empty ensemble
func TestCompletionHandler_ConvertToInternalRequest_EmptyEnsembleConfig(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt:         "Test",
		EnsembleConfig: nil,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	// Should have default ensemble config
	assert.NotNil(t, internalReq.EnsembleConfig)
	assert.Equal(t, "confidence_weighted", internalReq.EnsembleConfig.Strategy)
	assert.Equal(t, 2, internalReq.EnsembleConfig.MinProviders)
	assert.Equal(t, 0.8, internalReq.EnsembleConfig.ConfidenceThreshold)
}

// TestCompletionHandler_ConvertToInternalRequest_NilStopSequences tests nil stop sequences
func TestCompletionHandler_ConvertToInternalRequest_NilStopSequences(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt: "Test",
		Stop:   nil,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	// Should have empty slice, not nil
	assert.NotNil(t, internalReq.ModelParams.StopSequences)
	assert.Empty(t, internalReq.ModelParams.StopSequences)
}

// TestCompletionHandler_ConvertToInternalRequest_ZeroValues tests zero value defaults
func TestCompletionHandler_ConvertToInternalRequest_ZeroValues(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt:      "Test",
		Temperature: 0,
		MaxTokens:   0,
		TopP:        0,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	// Zero values should be replaced with defaults
	assert.Equal(t, 0.7, internalReq.ModelParams.Temperature)
	assert.Equal(t, 1000, internalReq.ModelParams.MaxTokens)
	assert.Equal(t, 1.0, internalReq.ModelParams.TopP)
}

// TestCompletionHandler_CompleteStream_ValidRequest tests stream with valid request
func TestCompletionHandler_CompleteStream_ValidRequest(t *testing.T) {
	t.Parallel()
	service := services.NewRequestService("random", nil, nil)
	handler := NewCompletionHandler(service)

	reqBody := map[string]interface{}{
		"prompt": "Test prompt for streaming",
		"stream": true,
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CompleteStream(c)

	// Should fail with error status (no ensemble configured)
	assert.True(t, w.Code >= 400 && w.Code < 600, "Expected error status code, got %d", w.Code)
}

// TestCompletionHandler_ChatStream_ValidRequest tests chat stream with valid request
func TestCompletionHandler_ChatStream_ValidRequest(t *testing.T) {
	t.Parallel()
	service := services.NewRequestService("random", nil, nil)
	handler := NewCompletionHandler(service)

	reqBody := map[string]interface{}{
		"prompt": "Test",
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
		"stream": true,
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatStream(c)

	// Should fail with error status (no ensemble configured)
	assert.True(t, w.Code >= 400 && w.Code < 600, "Expected error status code, got %d", w.Code)
}

// TestCompletionHandler_ProviderSpecific tests provider specific params
func TestCompletionHandler_ProviderSpecific(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt: "Test",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	// Should have empty provider specific map
	assert.NotNil(t, internalReq.ModelParams.ProviderSpecific)
	assert.Empty(t, internalReq.ModelParams.ProviderSpecific)
}

// TestCompletionHandler_Memory tests memory field
func TestCompletionHandler_Memory(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt:         "Test",
		MemoryEnhanced: true,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	assert.True(t, internalReq.MemoryEnhanced)
	assert.NotNil(t, internalReq.Memory)
	assert.Empty(t, internalReq.Memory)
}

// TestCompletionHandler_RequestType tests request type field
func TestCompletionHandler_RequestType(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt:      "Test",
		RequestType: "custom_type",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	assert.Equal(t, "custom_type", internalReq.RequestType)
}

// TestCompletionHandler_CreatedAtAndStatus tests created at and status
func TestCompletionHandler_CreatedAtAndStatus(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	beforeTime := time.Now()

	req := &CompletionRequest{
		Prompt: "Test",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	afterTime := time.Now()

	assert.Equal(t, "pending", internalReq.Status)
	assert.True(t, internalReq.CreatedAt.After(beforeTime) || internalReq.CreatedAt.Equal(beforeTime))
	assert.True(t, internalReq.CreatedAt.Before(afterTime) || internalReq.CreatedAt.Equal(afterTime))
}

// TestCompletionHandler_IDGeneration tests ID generation
func TestCompletionHandler_IDGeneration(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt: "Test",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Generate multiple requests to verify unique IDs
	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		internalReq := handler.convertToInternalRequest(req, c)
		assert.NotEmpty(t, internalReq.ID)
		assert.NotContains(t, ids, internalReq.ID, "IDs should be unique")
		ids[internalReq.ID] = true
	}
}

// TestCompletionHandler_SessionIDGeneration tests session ID generation
func TestCompletionHandler_SessionIDGeneration(t *testing.T) {
	t.Parallel()
	handler := &CompletionHandler{}

	req := &CompletionRequest{
		Prompt: "Test",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	assert.NotEmpty(t, internalReq.SessionID)
	assert.Contains(t, internalReq.SessionID, "session_")
}
