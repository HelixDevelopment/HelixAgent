package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
)

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
	assert.Contains(t, body, "superagent-ensemble")
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
	assert.Contains(t, body, "superagent-ensemble")
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
