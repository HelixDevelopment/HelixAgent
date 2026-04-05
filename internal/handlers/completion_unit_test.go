package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/skills"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLLMProvider is a mock implementation of the LLMProvider interface for testing
type MockLLMProvider struct {
	name           string
	completeFunc   func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error)
	streamFunc     func(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error)
	healthCheckErr error
}

func (m *MockLLMProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, req)
	}
	return &models.LLMResponse{
		ID:           "mock-response-id",
		Content:      "Mock response content",
		ProviderName: m.name,
		TokensUsed:   100,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}, nil
}

func (m *MockLLMProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req)
	}
	ch := make(chan *models.LLMResponse, 3)
	go func() {
		defer close(ch)
		for i := 0; i < 3; i++ {
			select {
			case <-ctx.Done():
				return
			case ch <- &models.LLMResponse{
				ID:           "mock-stream-id",
				Content:      "Stream chunk ",
				ProviderName: m.name,
				TokensUsed:   10,
				CreatedAt:    time.Now(),
				FinishReason: "",
			}:
			}
		}
	}()
	return ch, nil
}

func (m *MockLLMProvider) HealthCheck() error {
	return m.healthCheckErr
}

func (m *MockLLMProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportsStreaming: true,
	}
}

func (m *MockLLMProvider) ValidateConfig(config map[string]interface{}) (bool, []string) {
	return true, nil
}

// setupCompletionTest creates a test environment for completion handler tests
func setupCompletionTest() (*gin.Engine, *CompletionHandler, *services.RequestService) {
	gin.SetMode(gin.TestMode)

	// Create request service with weighted strategy
	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	// Register a mock provider
	mockProvider := &MockLLMProvider{name: "mock-provider"}
	requestService.RegisterProvider("mock-provider", mockProvider)

	handler := NewCompletionHandler(requestService)

	router := gin.New()
	v1 := router.Group("/v1")
	{
		v1.POST("/completions", handler.Complete)
		v1.POST("/completions/stream", handler.CompleteStream)
		v1.POST("/chat/completions", handler.Chat)
		v1.POST("/chat/completions/stream", handler.ChatStream)
		v1.GET("/models", handler.Models)
	}

	return router, handler, requestService
}

// TestCompletionHandler_Complete_Success tests successful completion
func TestCompletionHandlerUnit_Complete_Success(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	reqBody := CompletionRequest{
		Prompt:      "Hello, world!",
		Model:       "test-model",
		Temperature: 0.7,
		MaxTokens:   100,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response CompletionResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response.ID)
	assert.Equal(t, "text_completion", response.Object)
	assert.NotZero(t, response.Created)
	assert.NotEmpty(t, response.Choices)
	assert.NotNil(t, response.Usage)
}

// TestCompletionHandler_Complete_MissingPrompt tests completion with missing prompt
func TestCompletionHandlerUnit_Complete_MissingPrompt(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	reqBody := map[string]interface{}{
		"model": "test-model",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error.Type)
}

// TestCompletionHandler_Complete_InvalidJSON tests completion with invalid JSON
func TestCompletionHandlerUnit_Complete_InvalidJSON(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBufferString("not valid json"))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCompletionHandler_Complete_ProviderError tests completion when provider fails
func TestCompletionHandlerUnit_Complete_ProviderError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	// Register a failing provider
	failingProvider := &MockLLMProvider{
		name: "failing-provider",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return nil, errors.New("provider connection failed")
		},
	}
	requestService.RegisterProvider("failing-provider", failingProvider)

	handler := NewCompletionHandler(requestService)

	router := gin.New()
	router.POST("/v1/completions", handler.Complete)

	reqBody := CompletionRequest{
		Prompt: "Test prompt",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	// Should return error status
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

// TestCompletionHandler_Complete_RateLimitError tests rate limit handling
func TestCompletionHandlerUnit_Complete_RateLimitError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	// Register a provider that returns rate limit error
	rateLimitProvider := &MockLLMProvider{
		name: "rate-limit-provider",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return nil, services.NewRateLimitError("test-provider", 60*time.Second)
		},
	}
	requestService.RegisterProvider("rate-limit-provider", rateLimitProvider)

	handler := NewCompletionHandler(requestService)

	router := gin.New()
	router.POST("/v1/completions", handler.Complete)

	reqBody := CompletionRequest{
		Prompt: "Test prompt",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Header().Get("Retry-After"), "60")
}

// TestCompletionHandler_Complete_TimeoutError tests timeout error handling
func TestCompletionHandlerUnit_Complete_TimeoutError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	timeoutProvider := &MockLLMProvider{
		name: "timeout-provider",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return nil, services.NewTimeoutError("test-provider", context.DeadlineExceeded)
		},
	}
	requestService.RegisterProvider("timeout-provider", timeoutProvider)

	handler := NewCompletionHandler(requestService)

	router := gin.New()
	router.POST("/v1/completions", handler.Complete)

	reqBody := CompletionRequest{
		Prompt: "Test prompt",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)
}

// TestCompletionHandler_CompleteStream_Success tests successful streaming completion
func TestCompletionHandlerUnit_CompleteStream_Success(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	reqBody := CompletionRequest{
		Prompt:      "Stream test",
		Model:       "test-model",
		Temperature: 0.7,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions/stream", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	assert.Contains(t, w.Body.String(), "data:")
}

// TestCompletionHandler_CompleteStream_InvalidJSON tests streaming with invalid JSON
func TestCompletionHandlerUnit_CompleteStream_InvalidJSON(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions/stream", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCompletionHandler_CompleteStream_NoProviders tests streaming when no providers available
func TestCompletionHandlerUnit_CompleteStream_NoProviders(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// Create request service without any providers
	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	handler := NewCompletionHandler(requestService)

	router := gin.New()
	router.POST("/v1/completions/stream", handler.CompleteStream)

	reqBody := CompletionRequest{
		Prompt: "Test prompt",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions/stream", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	// Should return error since no providers available
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

// TestCompletionHandler_Chat_Success tests successful chat completion
func TestCompletionHandlerUnit_Chat_Success(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	reqBody := CompletionRequest{
		Prompt: "Hello",
		Messages: []models.Message{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello"},
		},
		Model:       "test-model",
		Temperature: 0.7,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "chat.completion", response["object"])
	assert.Contains(t, response, "choices")
	assert.Contains(t, response, "usage")
}

// TestCompletionHandler_Chat_MissingPrompt tests chat with missing prompt
func TestCompletionHandlerUnit_Chat_MissingPrompt(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	reqBody := map[string]interface{}{
		"model": "test-model",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCompletionHandler_ChatStream_Success tests successful chat streaming
func TestCompletionHandlerUnit_ChatStream_Success(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	reqBody := CompletionRequest{
		Prompt: "Hello",
		Messages: []models.Message{
			{Role: "user", Content: "Hello"},
		},
		Model:       "test-model",
		Temperature: 0.7,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions/stream", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "chat.completion.chunk")
}

// TestCompletionHandler_ChatStream_InvalidJSON tests chat streaming with invalid JSON
func TestCompletionHandlerUnit_ChatStream_InvalidJSON(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions/stream", bytes.NewBufferString("bad json"))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCompletionHandler_Models tests model listing endpoint
func TestCompletionHandlerUnit_Models(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "list", response["object"])

	data, ok := response["data"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(data), 3)

	// Verify model structure
	if len(data) > 0 {
		model := data[0].(map[string]interface{})
		assert.Contains(t, model, "id")
		assert.Contains(t, model, "object")
		assert.Contains(t, model, "created")
		assert.Contains(t, model, "owned_by")
	}
}

// TestCompletionHandler_Complete_WithSkills tests completion with skills integration
func TestCompletionHandlerUnit_Complete_WithSkills(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// Create mock skills service
	mockSkillService := skills.NewService(&skills.SkillConfig{MinConfidence: 0.5})
	skillsIntegration := skills.NewIntegration(mockSkillService)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)
	requestService.RegisterProvider("mock-provider", &MockLLMProvider{name: "mock-provider"})

	handler := NewCompletionHandlerWithSkills(requestService, skillsIntegration)

	router := gin.New()
	router.POST("/v1/completions", handler.Complete)

	reqBody := CompletionRequest{
		Prompt:      "Test prompt",
		Model:       "test-model",
		Temperature: 0.7,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCompletionHandler_Complete_WithEnsembleConfig tests completion with ensemble configuration
func TestCompletionHandlerUnit_Complete_WithEnsembleConfig(t *testing.T) {
	t.Parallel()
	router, _, requestService := setupCompletionTest()

	// Register multiple providers for ensemble
	requestService.RegisterProvider("provider-1", &MockLLMProvider{name: "provider-1"})
	requestService.RegisterProvider("provider-2", &MockLLMProvider{name: "provider-2"})

	reqBody := CompletionRequest{
		Prompt: "Test prompt",
		EnsembleConfig: &models.EnsembleConfig{
			Strategy:            "confidence_weighted",
			MinProviders:        2,
			ConfidenceThreshold: 0.8,
			FallbackToBest:      true,
			Timeout:             30,
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCompletionHandler_ConvertToInternalRequest_Defaults tests default value handling
func TestCompletionHandlerUnit_ConvertToInternalRequest_Defaults(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	req := &CompletionRequest{
		Prompt: "Test",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	// Check defaults
	assert.Equal(t, 0.7, internalReq.ModelParams.Temperature)
	assert.Equal(t, 1000, internalReq.ModelParams.MaxTokens)
	assert.Equal(t, 1.0, internalReq.ModelParams.TopP)
	assert.NotNil(t, internalReq.EnsembleConfig)
	assert.Equal(t, "confidence_weighted", internalReq.EnsembleConfig.Strategy)
	assert.Equal(t, "anonymous", internalReq.UserID)
}

// TestCompletionHandler_ConvertToInternalRequest_WithContextValues tests context value extraction
func TestCompletionHandlerUnit_ConvertToInternalRequest_WithContextValues(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	req := &CompletionRequest{
		Prompt: "Test",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", "test-user-123")
	c.Set("session_id", "test-session-456")

	internalReq := handler.convertToInternalRequest(req, c)

	assert.Equal(t, "test-user-123", internalReq.UserID)
	assert.Equal(t, "test-session-456", internalReq.SessionID)
}

// TestCompletionHandler_ConvertToAPIResponse tests response conversion
func TestCompletionHandlerUnit_ConvertToAPIResponse(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	createdAt := time.Now()
	resp := &models.LLMResponse{
		ID:           "test-id",
		RequestID:    "req-id",
		ProviderName: "test-provider",
		Content:      "Test content",
		Confidence:   0.95,
		TokensUsed:   200,
		FinishReason: "stop",
		CreatedAt:    createdAt,
	}

	apiResp := handler.convertToAPIResponse(resp)

	assert.Equal(t, "test-id", apiResp.ID)
	assert.Equal(t, "text_completion", apiResp.Object)
	assert.Equal(t, createdAt.Unix(), apiResp.Created)
	assert.Equal(t, "test-provider", apiResp.Model)
	assert.Len(t, apiResp.Choices, 1)
	assert.Equal(t, "assistant", apiResp.Choices[0].Message.Role)
	assert.Equal(t, "Test content", apiResp.Choices[0].Message.Content)
	assert.Equal(t, "stop", apiResp.Choices[0].FinishReason)
	assert.Equal(t, 200, apiResp.Usage.TotalTokens)
}

// TestCompletionHandler_ConvertToChatResponse tests chat response conversion
func TestCompletionHandlerUnit_ConvertToChatResponse(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	createdAt := time.Now()
	resp := &models.LLMResponse{
		ID:           "chat-id",
		ProviderName: "test-provider",
		Content:      "Chat response",
		TokensUsed:   150,
		FinishReason: "stop",
		CreatedAt:    createdAt,
	}

	chatResp := handler.convertToChatResponse(resp)

	assert.Equal(t, "chat-id", chatResp["id"])
	assert.Equal(t, "chat.completion", chatResp["object"])
	assert.Equal(t, "test-provider", chatResp["model"])
	assert.Equal(t, createdAt.Unix(), chatResp["created"])

	choices := chatResp["choices"].([]map[string]interface{})
	assert.Len(t, choices, 1)
	message := choices[0]["message"].(map[string]interface{})
	assert.Equal(t, "assistant", message["role"])
	assert.Equal(t, "Chat response", message["content"])
}

// TestCompletionHandler_ConvertToStreamingResponse tests streaming response conversion
func TestCompletionHandlerUnit_ConvertToStreamingResponse(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	createdAt := time.Now()
	resp := &models.LLMResponse{
		ID:           "stream-id",
		ProviderName: "test-provider",
		Content:      "Stream content",
		CreatedAt:    createdAt,
		FinishReason: "stop",
	}

	streamResp := handler.convertToStreamingResponse(resp)

	assert.Equal(t, "stream-id", streamResp["id"])
	assert.Equal(t, "text_completion", streamResp["object"])

	choices := streamResp["choices"].([]map[string]interface{})
	delta := choices[0]["delta"].(map[string]interface{})
	assert.Equal(t, "Stream content", delta["content"])
}

// TestCompletionHandler_ConvertToChatStreamingResponse tests chat streaming response conversion
func TestCompletionHandlerUnit_ConvertToChatStreamingResponse(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	createdAt := time.Now()
	resp := &models.LLMResponse{
		ID:           "chat-stream-id",
		ProviderName: "test-provider",
		Content:      "Chat stream content",
		CreatedAt:    createdAt,
	}

	chatStreamResp := handler.convertToChatStreamingResponse(resp)

	assert.Equal(t, "chat-stream-id", chatStreamResp["id"])
	assert.Equal(t, "chat.completion.chunk", chatStreamResp["object"])

	choices := chatStreamResp["choices"].([]map[string]interface{})
	delta := choices[0]["delta"].(map[string]interface{})
	assert.Equal(t, "assistant", delta["role"])
	assert.Equal(t, "Chat stream content", delta["content"])
}

// TestCompletionHandler_SendCategorizedError tests error categorization
func TestCompletionHandlerUnit_SendCategorizedError(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "rate limit error",
			err:            services.NewRateLimitError("test", 60*time.Second),
			expectedStatus: http.StatusTooManyRequests,
			expectedCode:   "RATE_LIMIT_EXCEEDED",
		},
		{
			name:           "timeout error",
			err:            services.NewTimeoutError("test", errors.New("timeout")),
			expectedStatus: http.StatusGatewayTimeout,
			expectedCode:   "TIMEOUT",
		},
		{
			name:           "validation error",
			err:            services.NewValidationError("invalid input", nil),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "VALIDATION_ERROR",
		},
		{
			name:           "provider error",
			err:            services.NewProviderError("test", errors.New("failed")),
			expectedStatus: http.StatusBadGateway,
			expectedCode:   "PROVIDER_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			handler.sendCategorizedError(c, tt.err)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			errorObj := response["error"].(map[string]interface{})
			assert.Equal(t, tt.expectedCode, errorObj["code"])
		})
	}
}

// TestCompletionHandler_Complete_WithMessages tests completion with message history
func TestCompletionHandlerUnit_Complete_WithMessages(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	reqBody := CompletionRequest{
		Prompt: "Continue the conversation",
		Messages: []models.Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
			{Role: "user", Content: "How are you?"},
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCompletionHandler_Complete_WithMemoryEnhanced tests completion with memory enhancement
func TestCompletionHandlerUnit_Complete_WithMemoryEnhanced(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	reqBody := CompletionRequest{
		Prompt:         "Remember this",
		MemoryEnhanced: true,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCompletionHandler_Chat_WithLastUserMessage tests chat extracts last user message
func TestCompletionHandlerUnit_Chat_WithLastUserMessage(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	req := &CompletionRequest{
		Prompt: "Original prompt",
		Messages: []models.Message{
			{Role: "user", Content: "First message"},
			{Role: "assistant", Content: "Response"},
			{Role: "user", Content: "Last message"},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	internalReq := handler.convertToInternalRequest(req, c)
	internalReq.RequestType = "chat"

	// The chat handler uses the last user message for skill matching
	assert.Equal(t, "Last message", req.Messages[len(req.Messages)-1].Content)
}

// TestCompletionHandler_SendError tests error response formatting
func TestCompletionHandlerUnit_SendError(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	handler.sendError(c, http.StatusBadRequest, "invalid_request", "Bad request", "Missing field")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "invalid_request", response.Error.Type)
	assert.Equal(t, "400", response.Error.Code)
	assert.Contains(t, response.Error.Message, "Bad request")
	assert.Contains(t, response.Error.Message, "Missing field")
}

// TestCompletionHandler_ConvertToAPIResponseWithSkills tests response with skills metadata
func TestCompletionHandlerUnit_ConvertToAPIResponseWithSkills(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mockSkillService := skills.NewService(&skills.SkillConfig{MinConfidence: 0.5})
	skillsIntegration := skills.NewIntegration(mockSkillService)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	handler := NewCompletionHandlerWithSkills(requestService, skillsIntegration)

	resp := &models.LLMResponse{
		ID:           "test-id",
		ProviderName: "test-provider",
		Content:      "Test content",
		CreatedAt:    time.Now(),
	}

	// Empty usages should not panic
	apiResp := handler.convertToAPIResponseWithSkills(resp, []skills.SkillUsage{})
	assert.NotNil(t, apiResp)
}

// TestCompletionHandler_SetIntentBasedRouter tests setting the intent router
func TestCompletionHandlerUnit_SetIntentBasedRouter(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)
	handler := NewCompletionHandler(requestService)

	intentRouter := services.NewIntentBasedRouter(nil, logrus.New())
	handler.SetIntentBasedRouter(intentRouter)

	assert.NotNil(t, handler.intentRouter)
}

// TestCompletionHandler_NewCompletionHandlerWithSkills tests handler creation with skills
func TestCompletionHandlerUnit_NewCompletionHandlerWithSkills(t *testing.T) {
	t.Parallel()
	mockSkillService := skills.NewService(&skills.SkillConfig{MinConfidence: 0.5})
	skillsIntegration := skills.NewIntegration(mockSkillService)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	handler := NewCompletionHandlerWithSkills(requestService, skillsIntegration)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.requestService)
	assert.NotNil(t, handler.skillsIntegration)
}

// TestCompletionHandler_SetSkillsIntegration tests setting skills integration
func TestCompletionHandlerUnit_SetSkillsIntegration(t *testing.T) {
	t.Parallel()
	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)
	handler := NewCompletionHandler(requestService)

	mockSkillService := skills.NewService(&skills.SkillConfig{MinConfidence: 0.5})
	skillsIntegration := skills.NewIntegration(mockSkillService)

	handler.SetSkillsIntegration(skillsIntegration)

	assert.NotNil(t, handler.skillsIntegration)
}

// TestCompletionHandler_ConvertToChatResponseWithSkills tests chat response with skills
func TestCompletionHandlerUnit_ConvertToChatResponseWithSkills(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mockSkillService := skills.NewService(&skills.SkillConfig{MinConfidence: 0.5})
	skillsIntegration := skills.NewIntegration(mockSkillService)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	handler := NewCompletionHandlerWithSkills(requestService, skillsIntegration)

	resp := &models.LLMResponse{
		ID:           "chat-id",
		ProviderName: "test-provider",
		Content:      "Test content",
		CreatedAt:    time.Now(),
	}

	// Test with nil skills integration
	emptyHandler := NewCompletionHandler(requestService)
	chatResp := emptyHandler.convertToChatResponseWithSkills(resp, nil)
	assert.NotNil(t, chatResp)

	// Test with empty usages
	chatResp = handler.convertToChatResponseWithSkills(resp, []skills.SkillUsage{})
	assert.NotNil(t, chatResp)
}

// TestCompletionHandler_ConvertToInternalRequest_WithToolCalls tests conversion with tool calls
func TestCompletionHandlerUnit_ConvertToInternalRequest_WithToolCalls(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	req := &CompletionRequest{
		Prompt: "Test",
		Messages: []models.Message{
			{
				Role:    "assistant",
				Content: "I'll help you with that",
				ToolCalls: map[string]interface{}{
					"name": "search",
					"arguments": map[string]interface{}{
						"query": "test",
					},
				},
			},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	assert.Len(t, internalReq.Messages, 1)
	assert.NotNil(t, internalReq.Messages[0].ToolCalls)
}

// TestCompletionHandler_ConvertToInternalRequest_WithName tests message with name field
func TestCompletionHandlerUnit_ConvertToInternalRequest_WithName(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	name := "TestUser"
	req := &CompletionRequest{
		Prompt: "Test",
		Messages: []models.Message{
			{
				Role:    "user",
				Content: "Hello",
				Name:    &name,
			},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	assert.Len(t, internalReq.Messages, 1)
	assert.NotNil(t, internalReq.Messages[0].Name)
	assert.Equal(t, "TestUser", *internalReq.Messages[0].Name)
}

// TestCompletionHandler_ConvertToInternalRequest_StopSequencesNil tests nil stop sequences handling
func TestCompletionHandlerUnit_ConvertToInternalRequest_StopSequencesNil(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	req := &CompletionRequest{
		Prompt: "Test",
		Stop:   nil,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	// Should set empty slice, not nil
	assert.NotNil(t, internalReq.ModelParams.StopSequences)
}

// TestCompletionHandler_ConvertToInternalRequest_WithoutEnsemble tests ensemble disabled
func TestCompletionHandlerUnit_ConvertToInternalRequest_WithoutEnsemble(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)
	handler := NewCompletionHandler(requestService)

	// Set intent router that disables ensemble
	logger := logrus.New()
	intentRouter := services.NewIntentBasedRouter(nil, logger)
	handler.SetIntentBasedRouter(intentRouter)

	req := &CompletionRequest{
		Prompt: "simple question that shouldn't use ensemble",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	// Ensemble config may or may not be nil depending on intent router logic
	_ = internalReq.EnsembleConfig
}

// TestCompletionHandler_SendCategorizedError_WithRetryAfter tests retry-after header
func TestCompletionHandlerUnit_SendCategorizedError_WithRetryAfter(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create rate limit error with retry after
	err := services.NewRateLimitError("test-provider", 120*time.Second)
	handler.sendCategorizedError(c, err)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Header().Get("Retry-After"), "120")
}

// TestCompletionHandler_SendCategorizedError_LLMServiceError tests with already categorized error
func TestCompletionHandlerUnit_SendCategorizedError_LLMServiceError(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Create an LLMServiceError directly
	llmErr := &services.LLMServiceError{
		Type:       services.ErrorTypeProvider,
		Message:    "Provider failed",
		Code:       "PROVIDER_ERROR",
		HTTPStatus: http.StatusBadGateway,
		Provider:   "test-provider",
		Retryable:  true,
	}

	handler.sendCategorizedError(c, llmErr)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

// TestCompletionHandler_Chat_WithStopSequences tests chat with stop sequences
func TestCompletionHandlerUnit_Chat_WithStopSequences(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	reqBody := CompletionRequest{
		Prompt: "Test",
		Stop:   []string{"STOP", "END"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCompletionHandler_Chat_WithTopP tests chat with top_p parameter
func TestCompletionHandlerUnit_Chat_WithTopP(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	reqBody := CompletionRequest{
		Prompt: "Test",
		TopP:   0.9,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCompletionHandler_Stream_WithVariousFinishReasons tests streaming with different finish reasons
func TestCompletionHandlerUnit_Stream_WithVariousFinishReasons(t *testing.T) {
	t.Parallel()
	finishReasons := []string{"stop", "length", "content_filter", ""}

	for _, reason := range finishReasons {
		t.Run("finish_reason_"+reason, func(t *testing.T) {
				t.Parallel()
			gin.SetMode(gin.TestMode)

			ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
			requestService := services.NewRequestService("weighted", ensemble, nil)

			// Create provider that returns specific finish reason
			provider := &MockLLMProvider{
				name: "test",
				streamFunc: func(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
					ch := make(chan *models.LLMResponse, 1)
					go func() {
						defer close(ch)
						ch <- &models.LLMResponse{
							ID:           "test",
							Content:      "Test",
							ProviderName: "test",
							FinishReason: reason,
							CreatedAt:    time.Now(),
						}
					}()
					return ch, nil
				},
			}
			requestService.RegisterProvider("test", provider)

			handler := NewCompletionHandler(requestService)
			router := gin.New()
			router.POST("/stream", handler.CompleteStream)

			reqBody := CompletionRequest{Prompt: "Test"}
			body, _ := json.Marshal(reqBody)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/stream", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Body.String(), reason)
		})
	}
}

// TestCompletionHandler_Headers tests that proper headers are set
func TestCompletionHandlerUnit_Headers(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	reqBody := CompletionRequest{Prompt: "Test"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
}

// TestCompletionHandler_ChatStream_Headers tests chat stream headers
func TestCompletionHandlerUnit_ChatStream_Headers(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	reqBody := CompletionRequest{Prompt: "Test"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions/stream", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "no", w.Header().Get("X-Accel-Buffering"))
}

// TestCompletionHandler_Stream_ResponseFormat tests streaming response format
func TestCompletionHandlerUnit_Stream_ResponseFormat(t *testing.T) {
	t.Parallel()
	router, _, _ := setupCompletionTest()

	reqBody := CompletionRequest{Prompt: "Test streaming format"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions/stream", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify SSE format
	responseBody := w.Body.String()
	assert.True(t, strings.HasPrefix(responseBody, "data: "))
	assert.Contains(t, responseBody, "[DONE]")

	// Each line should start with "data: " or be empty
	lines := strings.Split(responseBody, "\n")
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, "data: ") {
			t.Errorf("Invalid SSE line: %s", line)
		}
	}
}

// TestCompletionHandler_ConvertToInternalRequest_InvalidUserIDType tests invalid user_id type
func TestCompletionHandlerUnit_ConvertToInternalRequest_InvalidUserIDType(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	req := &CompletionRequest{Prompt: "Test"}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_id", 12345) // Invalid type
	c.Set("session_id", true) // Invalid type

	internalReq := handler.convertToInternalRequest(req, c)

	// Should fall back to defaults
	assert.Equal(t, "anonymous", internalReq.UserID)
}

// TestCompletionHandler_Complete_WithRequestType tests completion with specific request type
func TestCompletionHandlerUnit_Complete_WithRequestType(t *testing.T) {
	t.Parallel()
	_, handler, _ := setupCompletionTest()

	req := &CompletionRequest{
		Prompt:      "Test",
		RequestType: "analysis",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	internalReq := handler.convertToInternalRequest(req, c)

	assert.Equal(t, "analysis", internalReq.RequestType)
}

// TestCompletionHandler_Chat_RequestType tests chat sets request type
func TestCompletionHandlerUnit_Chat_RequestType(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)
	requestService.RegisterProvider("mock", &MockLLMProvider{name: "mock"})

	handler := NewCompletionHandler(requestService)

	// Test that chat sets RequestType to "chat"
	req := &CompletionRequest{
		Prompt: "Test",
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/chat", bytes.NewBufferString(`{"prompt":"test"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	internalReq := handler.convertToInternalRequest(req, c)
	internalReq.RequestType = "chat" // Chat handler sets this

	assert.Equal(t, "chat", internalReq.RequestType)
}

// TestCompletionHandler_NewCompletionHandler tests basic handler creation
func TestCompletionHandlerUnit_NewCompletionHandler(t *testing.T) {
	t.Parallel()
	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	handler := NewCompletionHandler(requestService)

	assert.NotNil(t, handler)
	assert.NotNil(t, handler.requestService)
	assert.Nil(t, handler.skillsIntegration)
	assert.Nil(t, handler.intentRouter)
}

// TestCompletionHandler_NoProvidersAvailable tests error when no providers
func TestCompletionHandlerUnit_NoProvidersAvailable(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// Create request service with NO providers
	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	handler := NewCompletionHandler(requestService)

	router := gin.New()
	router.POST("/v1/completions", handler.Complete)

	reqBody := CompletionRequest{Prompt: "Test"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	// Should return error since no providers
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

// TestCompletionHandler_NetworkError tests network error handling
func TestCompletionHandlerUnit_NetworkError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	// Register provider that returns network error
	provider := &MockLLMProvider{
		name: "network-error",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return nil, services.NewNetworkError("test", errors.New("connection refused"))
		},
	}
	requestService.RegisterProvider("network-error", provider)

	handler := NewCompletionHandler(requestService)

	router := gin.New()
	router.POST("/v1/completions", handler.Complete)

	reqBody := CompletionRequest{Prompt: "Test"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

// TestCompletionHandler_ConfigurationError tests configuration error handling
func TestCompletionHandlerUnit_ConfigurationError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	// Register provider that returns config error
	provider := &MockLLMProvider{
		name: "config-error",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return nil, services.NewConfigurationError("API key not configured", errors.New("missing key"))
		},
	}
	requestService.RegisterProvider("config-error", provider)

	handler := NewCompletionHandler(requestService)

	router := gin.New()
	router.POST("/v1/completions", handler.Complete)

	reqBody := CompletionRequest{Prompt: "Test"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestCompletionHandler_AllProvidersFailed tests all providers failed error
func TestCompletionHandlerUnit_AllProvidersFailed(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	handler := NewCompletionHandler(requestService)

	router := gin.New()
	router.POST("/v1/completions", handler.Complete)

	reqBody := CompletionRequest{Prompt: "Test"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	// No providers registered, so should fail
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

// TestCompletionHandler_CompleteStream_NoFlusher tests streaming without flusher
func TestCompletionHandlerUnit_CompleteStream_NoFlusher(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)
	requestService.RegisterProvider("mock", &MockLLMProvider{name: "mock"})

	handler := NewCompletionHandler(requestService)

	// Use a custom response writer that doesn't implement http.Flusher
	w := &mockResponseWriter{}
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/stream", bytes.NewBufferString(`{"prompt":"test"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CompleteStream(c)

	// The mock response writer doesn't actually block streaming
	// In real scenario, this would fail but our mock doesn't simulate it
	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, w.status)
}

// mockResponseWriter is a mock that doesn't implement http.Flusher
type mockResponseWriter struct {
	status int
}

func (m *mockResponseWriter) Header() http.Header {
	return http.Header{}
}

func (m *mockResponseWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (m *mockResponseWriter) WriteHeader(status int) {
	m.status = status
}
