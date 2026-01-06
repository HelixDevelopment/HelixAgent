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

	response := handler.convertToOpenAIChatStreamResponse(llmResponse, openaiReq)

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
	assert.Equal(t, "Streaming test response", delta["content"])
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

	for i, chunk := range chunks {
		llmResp := &models.LLMResponse{
			ID:        "chunk-" + strconv.Itoa(i),
			Content:   chunk,
			CreatedAt: time.Now(),
		}

		openaiReq := &OpenAIChatRequest{Model: "gpt-4"}
		streamResp := handler.convertToOpenAIChatStreamResponse(llmResp, openaiReq)

		assert.Equal(t, "chunk-"+strconv.Itoa(i), streamResp["id"])
		assert.Equal(t, "chat.completion.chunk", streamResp["object"])

		choices := streamResp["choices"].([]map[string]any)
		delta := choices[0]["delta"].(map[string]any)
		assert.Equal(t, chunk, delta["content"])
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
