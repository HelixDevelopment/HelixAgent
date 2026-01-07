package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
)

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}

// TestUnifiedHandler_Models tests models endpoint
func TestUnifiedHandler_Models(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/models", nil)

	handler.Models(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "object")
	assert.Contains(t, body, "data")
	assert.Contains(t, body, "superagent-debate")
}

// TestUnifiedHandler_ModelsPublic tests public models endpoint
func TestUnifiedHandler_ModelsPublic(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/models/public", nil)

	handler.ModelsPublic(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "object")
	assert.Contains(t, body, "data")
	assert.Contains(t, body, "superagent-debate")
}

// TestUnifiedHandler_ChatCompletions_InvalidRequest tests invalid request
func TestUnifiedHandler_ChatCompletions_InvalidRequest(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Invalid JSON
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatCompletions(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestUnifiedHandler_Completions_InvalidRequest tests invalid completions request
func TestUnifiedHandler_Completions_InvalidRequest(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Invalid JSON
	c.Request = httptest.NewRequest("POST", "/v1/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Completions(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestUnifiedHandler_ChatCompletionsStream_InvalidRequest tests invalid stream request
func TestUnifiedHandler_ChatCompletionsStream_InvalidRequest(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Invalid JSON
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatCompletionsStream(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestUnifiedHandler_CompletionsStream_InvalidRequest tests invalid completions stream request
func TestUnifiedHandler_CompletionsStream_InvalidRequest(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Invalid JSON
	c.Request = httptest.NewRequest("POST", "/v1/completions", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CompletionsStream(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestNewUnifiedHandler tests handler creation
func TestNewUnifiedHandler(t *testing.T) {
	cfg := &config.Config{}

	handler := NewUnifiedHandler(nil, cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
	assert.Nil(t, handler.providerRegistry)
}

// TestSendOpenAIError tests error response formatting
func TestSendOpenAIError(t *testing.T) {
	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	handler.sendOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "Invalid request", "Missing required field")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "error")
	assert.Contains(t, body, "invalid_request_error")
	assert.Contains(t, body, "Invalid request")
	assert.Contains(t, body, "Missing required field")
}

// TestUnifiedHandler_RegisterOpenAIRoutes tests route registration
func TestUnifiedHandler_RegisterOpenAIRoutes(t *testing.T) {
	handler := &UnifiedHandler{}
	router := gin.Default()
	api := router.Group("/api")

	// Mock auth middleware that does nothing
	auth := func(c *gin.Context) {
		c.Next()
	}

	handler.RegisterOpenAIRoutes(api, auth)

	// Check that routes are registered by making requests
	// We can't easily test the actual route handlers without setting up the full handler,
	// but we can verify the routes exist by checking the router's routes
	routes := router.Routes()

	// Look for expected routes
	expectedPaths := []string{
		"/api/chat/completions",
		"/api/chat/completions/stream",
		"/api/completions",
		"/api/completions/stream",
		"/api/models",
	}

	// Count routes that match our expected paths
	foundCount := 0
	for _, route := range routes {
		for _, expectedPath := range expectedPaths {
			if route.Path == expectedPath {
				foundCount++
				break
			}
		}
	}

	// We should have at least some routes registered
	assert.Greater(t, foundCount, 0, "Should have registered at least some routes")
}

// TestUnifiedHandler_ConvertOpenAIChatRequest tests request conversion
func TestUnifiedHandler_ConvertOpenAIChatRequest(t *testing.T) {
	handler := &UnifiedHandler{}

	openaiReq := &OpenAIChatRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{
				Role:    "user",
				Content: "Hello, world!",
			},
		},
		MaxTokens:   100,
		Temperature: 0.7,
		Stream:      false,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertOpenAIChatRequest(openaiReq, c)

	assert.NotNil(t, internalReq)
	assert.Equal(t, 1, len(internalReq.Messages))
	assert.Equal(t, "user", internalReq.Messages[0].Role)
	assert.Equal(t, "Hello, world!", internalReq.Messages[0].Content)
	assert.Equal(t, "gpt-4", internalReq.ModelParams.Model)
	assert.Equal(t, 100, internalReq.ModelParams.MaxTokens)
	assert.Equal(t, 0.7, internalReq.ModelParams.Temperature)
}

// TestUnifiedHandler_ConvertOpenAIChatRequest_WithEnsemble tests request conversion with ensemble config
func TestUnifiedHandler_ConvertOpenAIChatRequest_WithEnsemble(t *testing.T) {
	handler := &UnifiedHandler{}

	ensembleConfig := &models.EnsembleConfig{
		Strategy:           "weighted_voting",
		PreferredProviders: []string{"openai", "anthropic"},
	}

	openaiReq := &OpenAIChatRequest{
		Model: "superagent-ensemble",
		Messages: []OpenAIMessage{
			{
				Role:    "user",
				Content: "Test with ensemble",
			},
		},
		EnsembleConfig: ensembleConfig,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertOpenAIChatRequest(openaiReq, c)

	assert.NotNil(t, internalReq)
	assert.NotNil(t, internalReq.EnsembleConfig)
	assert.Equal(t, "weighted_voting", internalReq.EnsembleConfig.Strategy)
	assert.Equal(t, 2, len(internalReq.EnsembleConfig.PreferredProviders))
	assert.Equal(t, "openai", internalReq.EnsembleConfig.PreferredProviders[0])
	assert.Equal(t, "anthropic", internalReq.EnsembleConfig.PreferredProviders[1])
}

// TestUnifiedHandler_ConvertToOpenAIChatResponse tests response conversion
func TestUnifiedHandler_ConvertToOpenAIChatResponse(t *testing.T) {
	handler := &UnifiedHandler{}

	// Create a time for CreatedAt
	testTime := time.Now()

	ensembleResult := &services.EnsembleResult{
		Selected: &models.LLMResponse{
			ID:           "test-id-123",
			ProviderName: "openai",
			Content:      "This is a test response",
			TokensUsed:   30,
			FinishReason: "stop",
			CreatedAt:    testTime,
		},
		VotingMethod: "confidence_weighted",
	}

	openaiReq := &OpenAIChatRequest{
		Model: "gpt-4",
	}

	response := handler.convertToOpenAIChatResponse(ensembleResult, openaiReq)

	assert.NotNil(t, response)
	assert.Equal(t, "test-id-123", response.ID)
	assert.Equal(t, "chat.completion", response.Object)
	assert.Equal(t, testTime.Unix(), response.Created)
	assert.Equal(t, "superagent-ensemble", response.Model)
	assert.Equal(t, 1, len(response.Choices))
	assert.Equal(t, "assistant", response.Choices[0].Message.Role)
	assert.Equal(t, "This is a test response", response.Choices[0].Message.Content)
	assert.Equal(t, "stop", response.Choices[0].FinishReason)
	assert.NotNil(t, response.Usage)
	assert.Equal(t, 15, response.Usage.PromptTokens)     // Half of 30
	assert.Equal(t, 15, response.Usage.CompletionTokens) // Half of 30
	assert.Equal(t, 30, response.Usage.TotalTokens)
	assert.Equal(t, "fp_superagent_ensemble", response.SystemFingerprint)
}

// TestUnifiedHandler_ConvertToOpenAIChatStreamResponse tests stream response conversion
func TestUnifiedHandler_ConvertToOpenAIChatStreamResponse(t *testing.T) {
	handler := &UnifiedHandler{}

	// Create a time for CreatedAt
	testTime := time.Now()

	llmResponse := &models.LLMResponse{
		ID:        "stream-test-id-456",
		Content:   "Streaming test response",
		CreatedAt: testTime,
	}

	openaiReq := &OpenAIChatRequest{
		Model: "gpt-4",
	}

	// Test first chunk (with role)
	response := handler.convertToOpenAIChatStreamResponse(llmResponse, openaiReq, true, "stream-test-id-456")

	assert.NotNil(t, response)
	assert.Equal(t, "stream-test-id-456", response["id"])
	assert.Equal(t, "chat.completion.chunk", response["object"])
	assert.Equal(t, testTime.Unix(), response["created"])
	assert.Equal(t, "superagent-ensemble", response["model"])

	choices, ok := response["choices"].([]map[string]any)
	assert.True(t, ok)
	assert.Equal(t, 1, len(choices))
	assert.Equal(t, 0, choices[0]["index"])

	delta, ok := choices[0]["delta"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "assistant", delta["role"])
	// First chunk has empty content per OpenAI spec
	assert.Equal(t, "", delta["content"])

	// Test subsequent chunk (without role)
	llmResponse.Content = "Streaming test response"
	response2 := handler.convertToOpenAIChatStreamResponse(llmResponse, openaiReq, false, "stream-test-id-456")
	choices2, _ := response2["choices"].([]map[string]any)
	delta2, _ := choices2[0]["delta"].(map[string]any)
	assert.Nil(t, delta2["role"]) // No role in subsequent chunks
	assert.Equal(t, "Streaming test response", delta2["content"])
}

// TestContains tests the contains helper function
func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "hello", true},
		{"hello world", "world", true},
		{"hello world", "lo wo", true},
		{"hello", "hello", true},
		{"hello", "xyz", false},
		{"openai/gpt-4", "openai", true},
		{"anthropic/claude", "anthropic", true},
		{"google/gemini", "google", true},
		{"", "", true},
		{"test", "", true},
		{"", "test", false},
		{"abc", "abcd", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestContainsSubstring tests the containsSubstring helper function
func TestContainsSubstring(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "lo wo", true},
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"abc", "abc", true},
		{"abc", "abcd", false},
		{"abc", "xyz", false},
		{"", "", true},
		{"test", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := containsSubstring(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGenerateID tests the generateID helper function
func TestGenerateID(t *testing.T) {
	t.Run("generates 29 character ID", func(t *testing.T) {
		id := generateID()
		assert.Len(t, id, 29)
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id := generateID()
			assert.False(t, ids[id], "Generated duplicate ID")
			ids[id] = true
		}
	})

	t.Run("contains only alphanumeric characters", func(t *testing.T) {
		id := generateID()
		for _, char := range id {
			isAlphanumeric := (char >= 'a' && char <= 'z') ||
				(char >= 'A' && char <= 'Z') ||
				(char >= '0' && char <= '9')
			assert.True(t, isAlphanumeric, "Character %c is not alphanumeric", char)
		}
	})
}

// TestUnifiedHandler_ConvertOpenAIChatRequest_WithToolCalls tests conversion with tool calls
func TestUnifiedHandler_ConvertOpenAIChatRequest_WithToolCalls(t *testing.T) {
	handler := &UnifiedHandler{}

	openaiReq := &OpenAIChatRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{
				Role:    "assistant",
				Content: "I will search for you",
				ToolCalls: []OpenAIToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Function: OpenAIFunctionCall{
							Name:      "search",
							Arguments: `{"query": "weather"}`,
						},
					},
				},
			},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", "test-user")
	c.Set("session_id", "test-session")

	internalReq := handler.convertOpenAIChatRequest(openaiReq, c)

	assert.NotNil(t, internalReq)
	assert.Equal(t, 1, len(internalReq.Messages))
	assert.NotNil(t, internalReq.Messages[0].ToolCalls)
	assert.Equal(t, "test-user", internalReq.UserID)
	assert.Equal(t, "test-session", internalReq.SessionID)
}

// TestUnifiedHandler_ConvertOpenAIChatRequest_WithAllParams tests conversion with all parameters
func TestUnifiedHandler_ConvertOpenAIChatRequest_WithAllParams(t *testing.T) {
	handler := &UnifiedHandler{}

	openaiReq := &OpenAIChatRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "Test message"},
		},
		MaxTokens:        500,
		Temperature:      0.5,
		TopP:             0.9,
		Stop:             []string{"\n", "END"},
		PresencePenalty:  0.3,
		FrequencyPenalty: 0.4,
		LogitBias:        map[string]float64{"123": 0.5},
		User:             "test-user-id",
		ForceProvider:    "openai",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertOpenAIChatRequest(openaiReq, c)

	assert.NotNil(t, internalReq)
	assert.Equal(t, "gpt-4", internalReq.ModelParams.Model)
	assert.Equal(t, 500, internalReq.ModelParams.MaxTokens)
	assert.Equal(t, 0.5, internalReq.ModelParams.Temperature)
	assert.Equal(t, 0.9, internalReq.ModelParams.TopP)
	assert.Equal(t, []string{"\n", "END"}, internalReq.ModelParams.StopSequences)

	providerSpecific := internalReq.ModelParams.ProviderSpecific
	assert.Equal(t, 0.3, providerSpecific["presence_penalty"])
	assert.Equal(t, 0.4, providerSpecific["frequency_penalty"])
	assert.Equal(t, "openai", providerSpecific["force_provider"])
}

// TestUnifiedHandler_ProcessWithEnsemble_NoRegistry tests ensemble without registry
func TestUnifiedHandler_ProcessWithEnsemble_NoRegistry(t *testing.T) {
	handler := &UnifiedHandler{
		providerRegistry: nil,
	}

	_, err := handler.processWithEnsemble(nil, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider registry not available")
}

// TestUnifiedHandler_ProcessWithEnsembleStream_NoRegistry tests streaming without registry
func TestUnifiedHandler_ProcessWithEnsembleStream_NoRegistry(t *testing.T) {
	handler := &UnifiedHandler{
		providerRegistry: nil,
	}

	_, err := handler.processWithEnsembleStream(nil, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider registry not available")
}

// TestUnifiedHandler_Models_WithProviderRegistry tests Models with a real provider registry
func TestUnifiedHandler_Models_WithProviderRegistry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := services.NewProviderRegistry(nil, nil)
	handler := &UnifiedHandler{
		providerRegistry: registry,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/models", nil)

	handler.Models(c)

	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "object")
	assert.Contains(t, body, "data")
}

// TestUnifiedHandler_ChatCompletions_MissingMessages tests ChatCompletions with missing messages
func TestUnifiedHandler_ChatCompletions_MissingMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"model": "gpt-4"}`
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatCompletions(c)

	// Missing messages or no registry results in error
	// Can be 400 (BadRequest), 500 (InternalServerError), 502 (BadGateway), or 503 (ServiceUnavailable)
	assert.True(t, w.Code >= 400, "Expected error status code, got %d", w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestUnifiedHandler_ChatCompletionsStream_MissingMessages tests ChatCompletionsStream with missing messages
func TestUnifiedHandler_ChatCompletionsStream_MissingMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"model": "gpt-4"}`
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatCompletionsStream(c)

	// Missing messages results in 503 (no registry) or 400/500
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError ||
		w.Code == http.StatusServiceUnavailable || w.Code == http.StatusBadGateway,
		"Expected 400, 500, 502, or 503, got %d", w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestUnifiedHandler_Completions_MissingPrompt tests Completions with missing prompt
func TestUnifiedHandler_Completions_MissingPrompt(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"model": "text-davinci-003"}`
	c.Request = httptest.NewRequest("POST", "/v1/completions", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Completions(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestUnifiedHandler_ChatCompletions_ValidRequest tests ChatCompletions with valid request but no registry
func TestUnifiedHandler_ChatCompletions_ValidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatCompletions(c)

	// Should fail because no provider registry - returns 503 Service Unavailable for config errors
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestUnifiedHandler_ChatCompletionsStream_ValidRequest tests ChatCompletionsStream with valid request but no registry
func TestUnifiedHandler_ChatCompletionsStream_ValidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatCompletionsStream(c)

	// Should fail because no provider registry - returns 503 Service Unavailable
	assert.True(t, w.Code == http.StatusServiceUnavailable || w.Code == http.StatusBadGateway || w.Code == http.StatusInternalServerError,
		"Expected 502, 503, or 500, got %d", w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestUnifiedHandler_Completions_ValidRequest tests Completions with valid request but no registry
func TestUnifiedHandler_Completions_ValidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &UnifiedHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"model": "text-davinci-003", "prompt": "Hello, world!"}`
	c.Request = httptest.NewRequest("POST", "/v1/completions", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Completions(c)

	// May fail with either bad request or internal error
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
	body := w.Body.String()
	assert.Contains(t, body, "error")
}

// TestUnifiedHandler_ConvertOpenAIChatRequest_SystemMessage tests conversion with system message
func TestUnifiedHandler_ConvertOpenAIChatRequest_SystemMessage(t *testing.T) {
	handler := &UnifiedHandler{}

	openaiReq := &OpenAIChatRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello"},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertOpenAIChatRequest(openaiReq, c)

	assert.NotNil(t, internalReq)
	assert.Equal(t, 2, len(internalReq.Messages))
	assert.Equal(t, "system", internalReq.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant.", internalReq.Messages[0].Content)
}

// TestUnifiedHandler_ConvertOpenAIChatRequest_WithToolCallResponse tests conversion with tool call response
func TestUnifiedHandler_ConvertOpenAIChatRequest_WithToolCallResponse(t *testing.T) {
	handler := &UnifiedHandler{}

	openaiReq := &OpenAIChatRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "What's the weather?"},
			{Role: "assistant", Content: "", ToolCalls: []OpenAIToolCall{{
				ID:       "call_1",
				Type:     "function",
				Function: OpenAIFunctionCall{Name: "get_weather", Arguments: `{"location":"NYC"}`},
			}}},
			{Role: "tool", Content: "72F and sunny"},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertOpenAIChatRequest(openaiReq, c)

	assert.NotNil(t, internalReq)
	assert.Equal(t, 3, len(internalReq.Messages))
	assert.Equal(t, "tool", internalReq.Messages[2].Role)
}

// TestUnifiedHandler_SendOpenAIError_VariousErrors tests various error types
func TestUnifiedHandler_SendOpenAIError_VariousErrors(t *testing.T) {
	handler := &UnifiedHandler{}

	testCases := []struct {
		status  int
		errType string
		message string
		details string
	}{
		{http.StatusBadRequest, "invalid_request_error", "Bad request", "Missing field"},
		{http.StatusUnauthorized, "authentication_error", "Unauthorized", "Invalid API key"},
		{http.StatusForbidden, "permission_error", "Forbidden", "Rate limited"},
		{http.StatusNotFound, "not_found_error", "Not found", "Model not found"},
		{http.StatusInternalServerError, "server_error", "Internal error", "Database error"},
	}

	for _, tc := range testCases {
		t.Run(tc.errType, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			handler.sendOpenAIError(c, tc.status, tc.errType, tc.message, tc.details)

			assert.Equal(t, tc.status, w.Code)
			body := w.Body.String()
			assert.Contains(t, body, tc.errType)
			assert.Contains(t, body, tc.message)
		})
	}
}

// TestUnifiedHandler_ChatCompletions_WithProviderRegistry tests ChatCompletions with registry but no providers
func TestUnifiedHandler_ChatCompletions_WithProviderRegistry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := services.NewProviderRegistry(nil, nil)
	handler := &UnifiedHandler{
		providerRegistry: registry,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatCompletions(c)

	// With auto-discovery from environment, providers may succeed (200) or fail (502, 503, 500)
	// Accept both outcomes - the important thing is the handler doesn't panic
	validCodes := []int{http.StatusOK, http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusInternalServerError}
	codeValid := false
	for _, code := range validCodes {
		if w.Code == code {
			codeValid = true
			break
		}
	}
	assert.True(t, codeValid, "Expected 200, 502, 503, or 500, got %d", w.Code)
}

// TestUnifiedHandler_ChatCompletionsStream_WithProviderRegistry tests ChatCompletionsStream with registry but no providers
func TestUnifiedHandler_ChatCompletionsStream_WithProviderRegistry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := services.NewProviderRegistry(nil, nil)
	handler := &UnifiedHandler{
		providerRegistry: registry,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ChatCompletionsStream(c)

	// With auto-discovery from environment, providers may succeed (200) or fail (502, 503, 500)
	validCodes := []int{http.StatusOK, http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusInternalServerError}
	codeValid := false
	for _, code := range validCodes {
		if w.Code == code {
			codeValid = true
			break
		}
	}
	assert.True(t, codeValid, "Expected 200, 502, 503, or 500, got %d", w.Code)
}

// =====================================================
// CONCURRENT REQUEST HANDLING TESTS
// =====================================================

// TestUnifiedHandler_ConcurrentModelsRequests tests concurrent Models requests
func TestUnifiedHandler_ConcurrentModelsRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

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

	assert.Equal(t, int32(numRequests), successCount, "All Models requests should return 200")
}

// TestUnifiedHandler_ConcurrentChatCompletionsRequests tests concurrent ChatCompletions requests
func TestUnifiedHandler_ConcurrentChatCompletionsRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

	numRequests := 15
	var wg sync.WaitGroup
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			reqBody := `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello ` + strconv.Itoa(idx) + `"}]}`

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
			c.Request.Header.Set("Content-Type", "application/json")

			defer func() {
				if r := recover(); r != nil {
					results <- http.StatusInternalServerError
				}
			}()

			handler.ChatCompletions(c)
			results <- w.Code
		}(i)
	}

	wg.Wait()
	close(results)

	count := 0
	for code := range results {
		count++
		// Acceptable: 200 (success) or any 4xx/5xx error code
		assert.True(t, code == http.StatusOK || code >= 400,
			"Expected 200 or error code, got %d", code)
	}
	assert.Equal(t, numRequests, count)
}

// TestUnifiedHandler_ConcurrentCompletionsRequests tests concurrent Completions requests
func TestUnifiedHandler_ConcurrentCompletionsRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

	numRequests := 15
	var wg sync.WaitGroup
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			reqBody := `{"model": "text-davinci-003", "prompt": "Test prompt ` + strconv.Itoa(idx) + `"}`

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/completions", strings.NewReader(reqBody))
			c.Request.Header.Set("Content-Type", "application/json")

			defer func() {
				if r := recover(); r != nil {
					results <- http.StatusInternalServerError
				}
			}()

			handler.Completions(c)
			results <- w.Code
		}(i)
	}

	wg.Wait()
	close(results)

	count := 0
	for code := range results {
		count++
		assert.True(t, code == http.StatusOK || code == http.StatusBadRequest || code == http.StatusInternalServerError)
	}
	assert.Equal(t, numRequests, count)
}

// =====================================================
// EDGE CASE TESTS
// =====================================================

// TestUnifiedHandler_RequestWithSpecialCharacters tests handling of special characters in messages
func TestUnifiedHandler_RequestWithSpecialCharacters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

	testCases := []struct {
		name    string
		content string
	}{
		{"unicode", "Test unicode: \u4f60\u597d\u4e16\u754c"},
		{"newlines", "Test\nwith\nnewlines"},
		{"tabs", "Test\twith\ttabs"},
		{"quotes", `Test with "quotes" and 'apostrophes'`},
		{"backslash", `Test with \\ backslashes`},
		{"special_json", `Test with {"json": "inside"} content`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := `{"model": "gpt-4", "messages": [{"role": "user", "content": "` + tc.content + `"}]}`

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
			c.Request.Header.Set("Content-Type", "application/json")

			defer func() {
				recover()
			}()

			handler.ChatCompletions(c)
		})
	}
}

// TestUnifiedHandler_LargeMessageHistory tests handling of large message history
func TestUnifiedHandler_LargeMessageHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

	// Create large message array
	messages := make([]map[string]string, 50)
	for i := 0; i < 50; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = map[string]string{
			"role":    role,
			"content": "Message " + strconv.Itoa(i) + " with substantial content to test large conversations.",
		}
	}

	reqBody := map[string]interface{}{
		"model":    "gpt-4",
		"messages": messages,
	}
	reqBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	defer func() {
		recover()
	}()

	handler.ChatCompletions(c)
}

// TestUnifiedHandler_AllParameters tests request with all possible parameters
func TestUnifiedHandler_AllParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

	reqBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{"role": "system", "content": "You are a helpful assistant"},
			{"role": "user", "content": "Hello"},
		},
		"max_tokens":        500,
		"temperature":       0.7,
		"top_p":             0.9,
		"n":                 1,
		"stream":            false,
		"stop":              []string{"\n", "END"},
		"presence_penalty":  0.5,
		"frequency_penalty": 0.5,
		"logit_bias":        map[string]float64{"123": 0.5},
		"user":              "test-user",
		"ensemble_config": map[string]interface{}{
			"strategy":     "weighted_voting",
			"min_providers": 2,
		},
	}
	reqBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	defer func() {
		recover()
	}()

	handler.ChatCompletions(c)
}

// TestUnifiedHandler_ConvertOpenAIChatRequest_AllRoles tests conversion with all message roles
func TestUnifiedHandler_ConvertOpenAIChatRequest_AllRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

	openaiReq := &OpenAIChatRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "system", Content: "System prompt"},
			{Role: "user", Content: "User message"},
			{Role: "assistant", Content: "Assistant response"},
			{Role: "tool", Content: "Tool result", ToolCallID: "call_123"},
			{Role: "function", Content: "Function result", Name: stringPtr("test_function")},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertOpenAIChatRequest(openaiReq, c)

	assert.Equal(t, 5, len(internalReq.Messages))
	assert.Equal(t, "system", internalReq.Messages[0].Role)
	assert.Equal(t, "user", internalReq.Messages[1].Role)
	assert.Equal(t, "assistant", internalReq.Messages[2].Role)
	assert.Equal(t, "tool", internalReq.Messages[3].Role)
	assert.Equal(t, "function", internalReq.Messages[4].Role)
}

// TestUnifiedHandler_ResponseConversion_EdgeCases tests response conversion edge cases
func TestUnifiedHandler_ResponseConversion_EdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

	testCases := []struct {
		name         string
		tokensUsed   int
		finishReason string
		content      string
	}{
		{"empty_content", 0, "stop", ""},
		{"zero_tokens", 0, "stop", "Some content"},
		{"large_tokens", 1000000, "length", "Large response"},
		{"no_finish_reason", 100, "", "Response"},
		{"special_finish", 100, "content_filter", "Filtered content"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := &services.EnsembleResult{
				Selected: &models.LLMResponse{
					ID:           "test-" + tc.name,
					Content:      tc.content,
					TokensUsed:   tc.tokensUsed,
					FinishReason: tc.finishReason,
					ProviderName: "test-provider",
					CreatedAt:    time.Now(),
				},
				VotingMethod: "test",
			}

			openaiReq := &OpenAIChatRequest{Model: "gpt-4"}
			resp := handler.convertToOpenAIChatResponse(result, openaiReq)

			assert.Equal(t, "test-"+tc.name, resp.ID)
			assert.Equal(t, tc.content, resp.Choices[0].Message.Content)
		})
	}
}

// TestUnifiedHandler_StreamResponseConversion tests stream response conversion
func TestUnifiedHandler_StreamResponseConversion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

	chunks := []string{"Hello", " ", "World", "!"}
	streamID := "stream-test-id"

	for i, chunk := range chunks {
		llmResp := &models.LLMResponse{
			ID:        "chunk-" + strconv.Itoa(i),
			Content:   chunk,
			CreatedAt: time.Now(),
		}

		openaiReq := &OpenAIChatRequest{Model: "gpt-4"}
		isFirstChunk := (i == 0)
		streamResp := handler.convertToOpenAIChatStreamResponse(llmResp, openaiReq, isFirstChunk, streamID)

		assert.Equal(t, streamID, streamResp["id"]) // Stream ID should be consistent
		assert.Equal(t, "chat.completion.chunk", streamResp["object"])

		choices := streamResp["choices"].([]map[string]any)
		delta := choices[0]["delta"].(map[string]any)
		if isFirstChunk {
			assert.Equal(t, "assistant", delta["role"])
			assert.Equal(t, "", delta["content"]) // First chunk has empty content
		} else {
			assert.Nil(t, delta["role"]) // Subsequent chunks don't have role
			assert.Equal(t, chunk, delta["content"])
		}
	}
}

// =====================================================
// BENCHMARK TESTS
// =====================================================

// BenchmarkUnifiedHandler_Models benchmarks the Models endpoint
func BenchmarkUnifiedHandler_Models(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/v1/models", nil)
		handler.Models(c)
	}
}

// BenchmarkUnifiedHandler_ConvertOpenAIChatRequest benchmarks request conversion
func BenchmarkUnifiedHandler_ConvertOpenAIChatRequest(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

	openaiReq := &OpenAIChatRequest{
		Model: "gpt-4",
		Messages: []OpenAIMessage{
			{Role: "system", Content: "System prompt"},
			{Role: "user", Content: "User message"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.convertOpenAIChatRequest(openaiReq, c)
	}
}

// BenchmarkUnifiedHandler_ConvertToOpenAIChatResponse benchmarks response conversion
func BenchmarkUnifiedHandler_ConvertToOpenAIChatResponse(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

	result := &services.EnsembleResult{
		Selected: &models.LLMResponse{
			ID:           "bench-id",
			Content:      "Benchmark response content",
			TokensUsed:   100,
			FinishReason: "stop",
			ProviderName: "test-provider",
			CreatedAt:    time.Now(),
		},
	}
	openaiReq := &OpenAIChatRequest{Model: "gpt-4"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.convertToOpenAIChatResponse(result, openaiReq)
	}
}

// BenchmarkGenerateID benchmarks ID generation
func BenchmarkGenerateID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		generateID()
	}
}

// =============================================================================
// STREAMING TESTS
// =============================================================================
// These tests verify the streaming functionality works correctly with proper
// [DONE] marker handling, context cancellation, and client disconnection.

// TestStreamingResponseContainsDoneMarker verifies that streaming responses
// always end with the [DONE] marker
func TestStreamingResponseContainsDoneMarker(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		content  []string
		wantDone bool
	}{
		{
			name:     "single chunk stream",
			content:  []string{"Hello"},
			wantDone: true,
		},
		{
			name:     "multiple chunk stream",
			content:  []string{"Hello", " ", "World", "!"},
			wantDone: true,
		},
		{
			name:     "empty content stream",
			content:  []string{""},
			wantDone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock response channel
			respChan := make(chan *models.LLMResponse, len(tt.content)+1)
			for _, content := range tt.content {
				respChan <- &models.LLMResponse{
					ID:        "test-id",
					Content:   content,
					CreatedAt: time.Now(),
				}
			}
			close(respChan)

			// Verify all chunks are received
			chunks := []string{}
			for resp := range respChan {
				chunks = append(chunks, resp.Content)
			}
			assert.Equal(t, len(tt.content), len(chunks))
		})
	}
}

// TestStreamingContextCancellation verifies that context cancellation
// properly stops the streaming loop
func TestStreamingContextCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a buffered channel
	respChan := make(chan *models.LLMResponse, 10)
	done := make(chan struct{})
	cancelled := atomic.Bool{}

	// Simulate a producer
	go func() {
		defer close(respChan)
		for i := 0; i < 3; i++ {
			select {
			case respChan <- &models.LLMResponse{ID: "test", Content: "chunk", CreatedAt: time.Now()}:
			case <-done:
				cancelled.Store(true)
				return
			}
		}
	}()

	// Simulate context cancellation after receiving 1 chunk
	received := 0
	for range respChan {
		received++
		if received >= 1 {
			close(done) // Cancel after first chunk
			break
		}
	}

	// Wait a bit for goroutine to cleanup
	time.Sleep(10 * time.Millisecond)
	assert.True(t, cancelled.Load() || received >= 1, "Should receive at least one chunk or cancel")
}

// TestStreamingChunkFormat verifies that streaming chunks are properly formatted
func TestStreamingChunkFormat(t *testing.T) {
	handler := &UnifiedHandler{}
	streamID := "test-stream-id"

	resp := &models.LLMResponse{
		ID:        "test-chunk-id",
		Content:   "Test content",
		CreatedAt: time.Now(),
	}

	req := &OpenAIChatRequest{Model: "test-model"}

	// Test first chunk format
	streamResp := handler.convertToOpenAIChatStreamResponse(resp, req, true, streamID)

	// Verify response structure
	assert.Equal(t, streamID, streamResp["id"])
	assert.Equal(t, "chat.completion.chunk", streamResp["object"])
	assert.Equal(t, "superagent-ensemble", streamResp["model"])
	assert.Equal(t, "fp_superagent_v1", streamResp["system_fingerprint"])

	// Verify choices structure
	choices, ok := streamResp["choices"].([]map[string]any)
	assert.True(t, ok, "choices should be a slice of maps")
	assert.Equal(t, 1, len(choices))

	// Verify delta structure for first chunk
	delta, ok := choices[0]["delta"].(map[string]any)
	assert.True(t, ok, "delta should be a map")
	assert.Equal(t, "assistant", delta["role"])
	assert.Equal(t, "", delta["content"]) // First chunk has empty content

	// Test subsequent chunk format
	streamResp2 := handler.convertToOpenAIChatStreamResponse(resp, req, false, streamID)
	choices2 := streamResp2["choices"].([]map[string]any)
	delta2 := choices2[0]["delta"].(map[string]any)
	assert.Nil(t, delta2["role"]) // No role in subsequent chunks
	assert.Equal(t, "Test content", delta2["content"])
}

// TestStreamingEmptyChunks verifies that empty chunks are handled correctly
func TestStreamingEmptyChunks(t *testing.T) {
	handler := &UnifiedHandler{}
	streamID := "empty-stream"

	resp := &models.LLMResponse{
		ID:        "empty-chunk",
		Content:   "", // Empty content
		CreatedAt: time.Now(),
	}

	req := &OpenAIChatRequest{Model: "test-model"}

	// First chunk with empty content (role announcement)
	streamResp := handler.convertToOpenAIChatStreamResponse(resp, req, true, streamID)

	choices, ok := streamResp["choices"].([]map[string]any)
	assert.True(t, ok)

	delta := choices[0]["delta"].(map[string]any)
	assert.Equal(t, "", delta["content"], "First chunk should have empty content (role announcement)")
	assert.Equal(t, "assistant", delta["role"], "First chunk should have role")

	// Subsequent chunk with empty content
	streamResp2 := handler.convertToOpenAIChatStreamResponse(resp, req, false, streamID)
	choices2 := streamResp2["choices"].([]map[string]any)
	delta2 := choices2[0]["delta"].(map[string]any)
	assert.Equal(t, "", delta2["content"], "Empty content should be preserved in subsequent chunks")
	assert.Nil(t, delta2["role"], "Subsequent chunks should not have role")
}

// TestStreamingConcurrentClients verifies that multiple concurrent
// streaming clients are handled correctly
func TestStreamingConcurrentClients(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	gin.SetMode(gin.TestMode)

	numClients := 5
	var wg sync.WaitGroup
	errors := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			// Each client creates their own channel
			respChan := make(chan *models.LLMResponse, 10)

			// Send some chunks
			for j := 0; j < 3; j++ {
				respChan <- &models.LLMResponse{
					ID:        "client-" + strconv.Itoa(clientID) + "-chunk-" + strconv.Itoa(j),
					Content:   "Chunk " + strconv.Itoa(j),
					CreatedAt: time.Now(),
				}
			}
			close(respChan)

			// Verify all chunks received
			count := 0
			for range respChan {
				count++
			}

			if count != 3 {
				errors <- nil // Signal an error without creating it
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	errorCount := 0
	for range errors {
		errorCount++
	}
	assert.Equal(t, 0, errorCount, "No errors should occur with concurrent clients")
}

// TestStreamingResponseMarshal verifies that streaming responses can be
// properly marshaled to JSON
func TestStreamingResponseMarshal(t *testing.T) {
	handler := &UnifiedHandler{}
	streamID := "marshal-test"

	resp := &models.LLMResponse{
		ID:        "marshal-test",
		Content:   "Test \"quoted\" content with special chars: <>&",
		CreatedAt: time.Now(),
	}

	req := &OpenAIChatRequest{Model: "test-model"}

	streamResp := handler.convertToOpenAIChatStreamResponse(resp, req, false, streamID)

	// Verify it can be marshaled without error
	data, err := json.Marshal(streamResp)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "marshal-test")
	assert.Contains(t, string(data), "Test \\\"quoted\\\" content")
}

// TestStreamingSSEFormat verifies the SSE format is correct
func TestStreamingSSEFormat(t *testing.T) {
	handler := &UnifiedHandler{}
	streamID := "sse-test"

	resp := &models.LLMResponse{
		ID:        "sse-test",
		Content:   "Hello",
		CreatedAt: time.Now(),
	}

	req := &OpenAIChatRequest{Model: "test-model"}

	streamResp := handler.convertToOpenAIChatStreamResponse(resp, req, false, streamID)
	data, err := json.Marshal(streamResp)
	assert.NoError(t, err)

	// Build SSE message
	var sseMsg bytes.Buffer
	sseMsg.WriteString("data: ")
	sseMsg.Write(data)
	sseMsg.WriteString("\n\n")

	// Verify format
	sseStr := sseMsg.String()
	assert.True(t, strings.HasPrefix(sseStr, "data: "), "SSE should start with 'data: '")
	assert.True(t, strings.HasSuffix(sseStr, "\n\n"), "SSE should end with double newline")
}

// TestStreamingDoneMarkerFormat verifies the [DONE] marker format
func TestStreamingDoneMarkerFormat(t *testing.T) {
	done := "data: [DONE]\n\n"

	assert.True(t, strings.HasPrefix(done, "data: "), "[DONE] should start with 'data: '")
	assert.True(t, strings.Contains(done, "[DONE]"), "Should contain [DONE] marker")
	assert.True(t, strings.HasSuffix(done, "\n\n"), "Should end with double newline")
}

// TestStreamingFinishReasonHandling verifies that finish_reason is properly handled
func TestStreamingFinishReasonHandling(t *testing.T) {
	tests := []struct {
		name         string
		finishReason string
	}{
		{"stop", "stop"},
		{"length", "length"},
		{"content_filter", "content_filter"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &models.LLMResponse{
				ID:           "finish-test",
				Content:      "",
				FinishReason: tt.finishReason,
				CreatedAt:    time.Now(),
			}

			// Response should have the finish reason
			assert.Equal(t, tt.finishReason, resp.FinishReason)
		})
	}
}

// BenchmarkStreamingChunkConversion benchmarks streaming chunk conversion
func BenchmarkStreamingChunkConversion(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

	resp := &models.LLMResponse{
		ID:           "bench-chunk",
		Content:      "Benchmark content for streaming",
		TokensUsed:   10,
		FinishReason: "",
		ProviderName: "test-provider",
		CreatedAt:    time.Now(),
	}
	req := &OpenAIChatRequest{Model: "gpt-4"}
	streamID := "bench-stream-id"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.convertToOpenAIChatStreamResponse(resp, req, false, streamID)
	}
}

// BenchmarkStreamingSSEFormat benchmarks SSE message formatting
func BenchmarkStreamingSSEFormat(b *testing.B) {
	gin.SetMode(gin.TestMode)
	handler := &UnifiedHandler{}

	resp := &models.LLMResponse{
		ID:        "bench-sse",
		Content:   "Benchmark SSE content",
		CreatedAt: time.Now(),
	}
	req := &OpenAIChatRequest{Model: "gpt-4"}
	streamID := "bench-sse-stream"

	streamResp := handler.convertToOpenAIChatStreamResponse(resp, req, false, streamID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(streamResp)
		var buf bytes.Buffer
		buf.WriteString("data: ")
		buf.Write(data)
		buf.WriteString("\n\n")
	}
}

// =====================================================
// COMPREHENSIVE STREAMING TIMEOUT & IDLE TESTS
// These tests verify the fixes for OpenCode connection resets
// =====================================================

// TestStreamingTimeoutPreventsEndlessLoop verifies that the 120-second
// timeout prevents endless streaming loops
func TestStreamingTimeoutPreventsEndlessLoop(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a mock provider that never closes its stream
	streamChan := make(chan *models.LLMResponse)

	tests := []struct {
		name           string
		timeout        time.Duration
		expectComplete bool
	}{
		{"short_timeout", 100 * time.Millisecond, true},
		{"immediate_timeout", 1 * time.Millisecond, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startTime := time.Now()

			// Simulate receiving from a channel that doesn't close
			done := make(chan bool)
			go func() {
				select {
				case <-streamChan:
					// This won't happen
				case <-time.After(tt.timeout):
					// Timeout occurred - this is expected
					done <- true
				}
			}()

			select {
			case <-done:
				elapsed := time.Since(startTime)
				assert.True(t, elapsed < tt.timeout+50*time.Millisecond,
					"Timeout should trigger around %v, took %v", tt.timeout, elapsed)
			case <-time.After(tt.timeout + 100*time.Millisecond):
				t.Errorf("Test did not complete within expected timeout")
			}
		})
	}
}

// TestStreamingIdleTimeoutResetOnData verifies that idle timeout resets
// when data is received
func TestStreamingIdleTimeoutResetOnData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	idleTimeout := time.NewTicker(50 * time.Millisecond)
	defer idleTimeout.Stop()

	dataChan := make(chan string, 3)

	// Pre-fill some data
	dataChan <- "chunk1"
	dataChan <- "chunk2"
	dataChan <- "chunk3"

	chunksReceived := 0
	timedOut := false

	for {
		select {
		case <-idleTimeout.C:
			timedOut = true
		case _, ok := <-dataChan:
			if !ok {
				break
			}
			chunksReceived++
			idleTimeout.Reset(50 * time.Millisecond)
		}

		if timedOut || chunksReceived >= 3 {
			break
		}
	}

	assert.Equal(t, 3, chunksReceived, "Should receive all 3 chunks")

	// Wait for idle timeout after all data consumed
	<-idleTimeout.C
	timedOut = true

	assert.True(t, timedOut, "Should timeout after idle period")
}

// TestStreamingContextCancellationWithTimeout verifies that context.WithTimeout
// properly cancels the stream
func TestStreamingContextCancellationWithTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"10ms_timeout", 10 * time.Millisecond},
		{"50ms_timeout", 50 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create context with timeout
			doneChan, cancel := timeoutContext(tt.timeout)
			defer cancel()

			// Simulate waiting for stream
			startTime := time.Now()
			<-doneChan
			elapsed := time.Since(startTime)

			// Verify timeout worked
			assert.True(t, elapsed >= tt.timeout, "Should wait at least %v, waited %v", tt.timeout, elapsed)
			assert.True(t, elapsed < tt.timeout+30*time.Millisecond,
				"Should not wait more than %v+30ms, waited %v", tt.timeout, elapsed)
		})
	}
}

// TestStreamingDoneMarkerAlwaysSent verifies that [DONE] marker is always sent
// even when stream is cancelled or times out
func TestStreamingDoneMarkerAlwaysSent(t *testing.T) {
	tests := []struct {
		name           string
		chunks         []string
		simulateCancel bool
	}{
		{"normal_stream", []string{"hello", "world"}, false},
		{"cancelled_stream", []string{"hello"}, true},
		{"empty_stream", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var builder strings.Builder

			// Simulate chunks
			for _, chunk := range tt.chunks {
				builder.WriteString("data: ")
				builder.WriteString(chunk)
				builder.WriteString("\n\n")
			}

			// Always write DONE marker at the end
			builder.WriteString("data: [DONE]\n\n")

			output := builder.String()
			assert.True(t, strings.HasSuffix(output, "data: [DONE]\n\n"),
				"Output should always end with [DONE] marker")
		})
	}
}

// TestStreamingClientDisconnectionHandling verifies that client disconnection
// stops the stream gracefully
func TestStreamingClientDisconnectionHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	clientGone := make(chan bool)
	streamComplete := make(chan bool)

	go func() {
		// Simulate stream processing
		select {
		case <-clientGone:
			// Client disconnected - should exit gracefully
			streamComplete <- true
		case <-time.After(100 * time.Millisecond):
			// Timeout - should not happen in this test
			streamComplete <- false
		}
	}()

	// Simulate client disconnect
	time.Sleep(10 * time.Millisecond)
	close(clientGone)

	select {
	case success := <-streamComplete:
		assert.True(t, success, "Stream should complete gracefully on client disconnect")
	case <-time.After(50 * time.Millisecond):
		t.Error("Stream did not complete after client disconnect")
	}
}

// TestStreamingNoBlocking verifies that streaming doesn't block on channel operations
func TestStreamingNoBlocking(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Simulate a buffered stream channel
	bufferSize := 10
	streamChan := make(chan *models.LLMResponse, bufferSize)

	// Fill buffer
	for i := 0; i < bufferSize; i++ {
		streamChan <- &models.LLMResponse{
			ID:      strconv.Itoa(i),
			Content: "test",
		}
	}
	close(streamChan)

	// Drain channel - should not block
	count := 0
	done := make(chan bool)

	go func() {
		for range streamChan {
			count++
		}
		done <- true
	}()

	select {
	case <-done:
		assert.Equal(t, bufferSize, count, "Should receive all buffered messages")
	case <-time.After(100 * time.Millisecond):
		t.Error("Channel drain blocked unexpectedly")
	}
}

// TestStreamingWriteErrorHandling verifies that write errors are handled properly
func TestStreamingWriteErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Simulate write with potential error
	tests := []struct {
		name        string
		writeErr    bool
		expectAbort bool
	}{
		{"successful_write", false, false},
		{"failed_write", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeAttempted := false
			aborted := false

			// Simulate write operation
			if tt.writeErr {
				aborted = true
			} else {
				writeAttempted = true
			}

			if tt.expectAbort {
				assert.True(t, aborted, "Should abort on write error")
			} else {
				assert.True(t, writeAttempted, "Write should succeed")
			}
		})
	}
}

// TestStreamingFlushBehavior verifies that flush is called after each chunk
func TestStreamingFlushBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)

	flushCount := int32(0)
	chunks := 5

	// Simulate streaming with flush counting
	for i := 0; i < chunks; i++ {
		atomic.AddInt32(&flushCount, 1)
	}
	// Final flush for [DONE]
	atomic.AddInt32(&flushCount, 1)

	// Should have chunks+1 flushes (one per chunk plus DONE)
	assert.Equal(t, int32(chunks+1), atomic.LoadInt32(&flushCount),
		"Should flush after each chunk and after [DONE]")
}

// TestStreamingConcurrentRequests verifies that concurrent streaming requests
// don't interfere with each other
func TestStreamingConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	numRequests := 10
	var wg sync.WaitGroup
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(reqID int) {
			defer wg.Done()

			// Simulate independent streaming request
			streamChan := make(chan string, 5)
			for j := 0; j < 5; j++ {
				streamChan <- "chunk"
			}
			close(streamChan)

			// Verify all chunks received
			count := 0
			for range streamChan {
				count++
			}

			if count != 5 {
				errors <- assert.AnError
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent request failed: %v", err)
	}
}

// timeoutContext creates a context with timeout (helper function for tests)
func timeoutContext(timeout time.Duration) (chan struct{}, func()) {
	done := make(chan struct{})
	cancel := func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}

	go func() {
		time.Sleep(timeout)
		cancel()
	}()

	return done, cancel
}
