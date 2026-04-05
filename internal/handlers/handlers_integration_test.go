package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	"dev.helix.agent/internal/skills"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupIntegrationTest creates a full integration test environment
func setupIntegrationTest() (*gin.Engine, map[string]interface{}) {
	gin.SetMode(gin.TestMode)

	// Create services
	ensemble := services.NewEnsembleService("best_of_n", 30*time.Second)
	requestService := services.NewRequestService("weighted", ensemble, nil)

	// Register mock providers
	requestService.RegisterProvider("mock-provider", &MockLLMProvider{
		name: "mock-provider",
		completeFunc: func(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
			return &models.LLMResponse{
				ID:           "resp-" + time.Now().Format("20060102150405"),
				Content:      "Mock integration response",
				ProviderName: "mock-provider",
				TokensUsed:   100,
				FinishReason: "stop",
				CreatedAt:    time.Now(),
			}, nil
		},
	})

	// Create handlers
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	completionHandler := NewCompletionHandler(requestService)
	debateHandler := NewDebateHandler(nil, nil, logger)
	mcpHandler := NewMCPHandler(nil, &config.MCPConfig{Enabled: true})

	// Setup router with all routes
	router := gin.New()

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// API v1 routes
	v1 := router.Group("/v1")
	{
		// Completion routes
		v1.POST("/completions", completionHandler.Complete)
		v1.POST("/completions/stream", completionHandler.CompleteStream)
		v1.POST("/chat/completions", completionHandler.Chat)
		v1.POST("/chat/completions/stream", completionHandler.ChatStream)
		v1.GET("/models", completionHandler.Models)

		// Debate routes
		debates := v1.Group("/debates")
		{
			debates.POST("", debateHandler.CreateDebate)
			debates.GET("", debateHandler.ListDebates)
			debates.GET("/:id", debateHandler.GetDebate)
			debates.GET("/:id/status", debateHandler.GetDebateStatus)
			debates.GET("/:id/results", debateHandler.GetDebateResults)
			debates.POST("/:id/approve", debateHandler.ApproveDebate)
			debates.POST("/:id/reject", debateHandler.RejectDebate)
			debates.GET("/:id/gates", debateHandler.GetDebateGates)
			debates.GET("/:id/audit", debateHandler.GetDebateAudit)
			debates.DELETE("/:id", debateHandler.DeleteDebate)
		}

		// MCP routes
		v1.GET("/mcp/capabilities", mcpHandler.MCPCapabilities)
		v1.GET("/mcp/tools", mcpHandler.MCPTools)
		v1.POST("/mcp/tools/call", mcpHandler.MCPToolsCall)
		v1.GET("/mcp/prompts", mcpHandler.MCPPrompts)
		v1.GET("/mcp/resources", mcpHandler.MCPResources)
	}

	handlers := map[string]interface{}{
		"completion": completionHandler,
		"debate":     debateHandler,
		"mcp":        mcpHandler,
	}

	return router, handlers
}

// TestIntegration_HealthEndpoint tests the health endpoint
func TestIntegration_HealthEndpoint(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
}

// TestIntegration_CompleteFlow tests the complete completion flow
func TestIntegration_CompleteFlow(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	reqBody := CompletionRequest{
		Prompt:      "What is the capital of France?",
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
	assert.NotEmpty(t, response.Choices)
	assert.NotNil(t, response.Usage)
}

// TestIntegration_ChatFlow tests the chat completion flow
func TestIntegration_ChatFlow(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	reqBody := CompletionRequest{
		Prompt: "Hello",
		Messages: []models.Message{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello, how are you?"},
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

// TestIntegration_ModelsEndpoint tests the models endpoint
func TestIntegration_ModelsEndpoint(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "list", response["object"])
	assert.Contains(t, response, "data")

	data := response["data"].([]interface{})
	assert.GreaterOrEqual(t, len(data), 3)
}

// TestIntegration_DebateCreateAndRetrieve tests debate creation and retrieval
func TestIntegration_DebateCreateAndRetrieve(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	// Create debate
	createBody := CreateDebateRequest{
		Topic: "Should AI be regulated?",
		Participants: []ParticipantConfigRequest{
			{Name: "Advocate", Role: "proposer"},
			{Name: "Skeptic", Role: "critic"},
		},
		MaxRounds: 3,
	}
	body, _ := json.Marshal(createBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var createResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &createResponse)
	require.NoError(t, err)
	assert.Contains(t, createResponse, "debate_id")
	assert.Equal(t, "pending", createResponse["status"])

	debateID := createResponse["debate_id"].(string)

	// Retrieve debate
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/v1/debates/"+debateID, nil)

	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var getResponse map[string]interface{}
	err = json.Unmarshal(w2.Body.Bytes(), &getResponse)
	require.NoError(t, err)
	assert.Equal(t, debateID, getResponse["debate_id"])
	assert.Equal(t, "Should AI be regulated?", getResponse["topic"])
}

// TestIntegration_DebateStatusFlow tests debate status flow
func TestIntegration_DebateStatusFlow(t *testing.T) {
	t.Parallel()
	router, handlers := setupIntegrationTest()
	debateHandler := handlers["debate"].(*DebateHandler)

	// Create debate
	createBody := CreateDebateRequest{
		Topic: "Test debate",
		Participants: []ParticipantConfigRequest{
			{Name: "A"},
			{Name: "B"},
		},
	}
	body, _ := json.Marshal(createBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	var createResponse map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &createResponse)
	debateID := createResponse["debate_id"].(string)

	// Get status
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/v1/debates/"+debateID+"/status", nil)
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var statusResponse map[string]interface{}
	err := json.Unmarshal(w2.Body.Bytes(), &statusResponse)
	require.NoError(t, err)
	assert.Equal(t, debateID, statusResponse["debate_id"])
	assert.Contains(t, statusResponse, "status")
	assert.Contains(t, statusResponse, "start_time")

	// Manually update debate status for testing
	debateHandler.mu.Lock()
	if state, exists := debateHandler.activeDebates[debateID]; exists {
		state.Status = "paused"
	}
	debateHandler.mu.Unlock()

	// Test approve
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/v1/debates/"+debateID+"/approve", nil)
	router.ServeHTTP(w3, req3)

	assert.Equal(t, http.StatusOK, w3.Code)
}

// TestIntegration_DebateListAndDelete tests debate listing and deletion
func TestIntegration_DebateListAndDelete(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	// Create multiple debates
	for i := 0; i < 3; i++ {
		createBody := CreateDebateRequest{
			Topic: "Test debate " + string(rune('A'+i)),
			Participants: []ParticipantConfigRequest{
				{Name: "Participant 1"},
				{Name: "Participant 2"},
			},
		}
		body, _ := json.Marshal(createBody)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusAccepted, w.Code)
	}

	// List debates
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/debates", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var listResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &listResponse)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, int(listResponse["count"].(float64)), 3)

	debates := listResponse["debates"].([]interface{})
	assert.GreaterOrEqual(t, len(debates), 3)
}

// TestIntegration_MCPFlow tests MCP endpoints flow
func TestIntegration_MCPFlow(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	// Test capabilities
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/capabilities", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var capabilities map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &capabilities)
	require.NoError(t, err)
	assert.Contains(t, capabilities, "version")
	assert.Contains(t, capabilities, "capabilities")

	// Test tools
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools", nil)
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var tools map[string]interface{}
	err = json.Unmarshal(w2.Body.Bytes(), &tools)
	require.NoError(t, err)
	assert.Contains(t, tools, "tools")

	// Test prompts
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/v1/mcp/prompts", nil)
	router.ServeHTTP(w3, req3)

	assert.Equal(t, http.StatusOK, w3.Code)

	var prompts map[string]interface{}
	err = json.Unmarshal(w3.Body.Bytes(), &prompts)
	require.NoError(t, err)
	assert.Contains(t, prompts, "prompts")

	// Test resources
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodGet, "/v1/mcp/resources", nil)
	router.ServeHTTP(w4, req4)

	assert.Equal(t, http.StatusOK, w4.Code)

	var resources map[string]interface{}
	err = json.Unmarshal(w4.Body.Bytes(), &resources)
	require.NoError(t, err)
	assert.Contains(t, resources, "resources")
}

// TestIntegration_InvalidRoutes tests invalid routes
func TestIntegration_InvalidRoutes(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	invalidRoutes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/v1/debates/non-existent", ""},
		{http.MethodGet, "/v1/debates/non-existent/status", ""},
		{http.MethodGet, "/v1/debates/non-existent/results", ""},
		{http.MethodDelete, "/v1/debates/non-existent", ""},
		{http.MethodPost, "/v1/debates/non-existent/approve", ""},
		{http.MethodPost, "/v1/debates/non-existent/reject", ""},
		{http.MethodPost, "/v1/completions", "invalid json"},
		{http.MethodPost, "/v1/chat/completions", "invalid json"},
	}

	for _, route := range invalidRoutes {
		t.Run(route.method+"_"+route.path, func(t *testing.T) {
				t.Parallel()
			var body *bytes.Buffer
			if route.body != "" {
				body = bytes.NewBufferString(route.body)
			} else {
				body = &bytes.Buffer{}
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest(route.method, route.path, body)
			if route.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(w, req)

			// Should return 4xx error, not 200
			assert.NotEqual(t, http.StatusOK, w.Code)
		})
	}
}

// TestIntegration_HTTPMethods tests various HTTP methods
func TestIntegration_HTTPMethods(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	// Test GET
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test POST with valid body
	reqBody := CompletionRequest{
		Prompt: "Test",
	}
	body, _ := json.Marshal(reqBody)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Test DELETE
	createBody := CreateDebateRequest{
		Topic: "Delete test",
		Participants: []ParticipantConfigRequest{
			{Name: "A"},
			{Name: "B"},
		},
	}
	body, _ = json.Marshal(createBody)

	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req3.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w3, req3)

	var createResp map[string]interface{}
	json.Unmarshal(w3.Body.Bytes(), &createResp)
	debateID := createResp["debate_id"].(string)

	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodDelete, "/v1/debates/"+debateID, nil)
	router.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusOK, w4.Code)
}

// TestIntegration_ContentTypes tests various content types
func TestIntegration_ContentTypes(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	// JSON content type
	reqBody := CompletionRequest{Prompt: "Test"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	// Without content type (should still work for some endpoints)
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

// TestIntegration_MiddlewareNotFound tests not found handling
func TestIntegration_MiddlewareNotFound(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/non-existent-route", nil)
	router.ServeHTTP(w, req)

	// Gin returns 404 for non-existent routes
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestIntegration_ConcurrentRequests tests handling concurrent requests
func TestIntegration_ConcurrentRequests(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	done := make(chan bool, 10)

	// Send 10 concurrent requests
	for i := 0; i < 10; i++ {
		go func() {
			reqBody := CompletionRequest{Prompt: "Concurrent test"}
			body, _ := json.Marshal(reqBody)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			// All should succeed
			if w.Code == http.StatusOK {
				done <- true
			} else {
				done <- false
			}
		}()
	}

	// Wait for all requests
	successCount := 0
	for i := 0; i < 10; i++ {
		if <-done {
			successCount++
		}
	}

	assert.Equal(t, 10, successCount)
}

// TestIntegration_ResponseHeaders tests response headers
func TestIntegration_ResponseHeaders(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	// Test JSON response headers
	reqBody := CompletionRequest{Prompt: "Test"}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	// Test streaming response headers
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/v1/completions/stream", bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, "text/event-stream", w2.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w2.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w2.Header().Get("Connection"))
}

// TestIntegration_LargePayload tests handling of large payloads
func TestIntegration_LargePayload(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	// Create a large prompt
	largePrompt := ""
	for i := 0; i < 1000; i++ {
		largePrompt += "This is a test sentence. "
	}

	reqBody := CompletionRequest{
		Prompt:    largePrompt,
		MaxTokens: 500,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestIntegration_ComplexDebateFlow tests a complex debate workflow
func TestIntegration_ComplexDebateFlow(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	// Create debate with all options
	createBody := CreateDebateRequest{
		DebateID: "integration-debate",
		Topic:    "Complex integration test debate",
		Participants: []ParticipantConfigRequest{
			{
				ParticipantID: "p1",
				Name:          "Expert",
				Role:          "analyst",
				LLMProvider:   "mock",
				LLMModel:      "test-model",
				Weight:        1.5,
			},
			{
				ParticipantID: "p2",
				Name:          "Critic",
				Role:          "reviewer",
				LLMProvider:   "mock",
				LLMModel:      "test-model",
				Weight:        1.0,
			},
		},
		MaxRounds:                 5,
		Timeout:                   600,
		Strategy:                  "consensus",
		EnableCognee:              true,
		EnableMultiPassValidation: true,
		ValidationConfig: &ValidationConfigRequest{
			EnableValidation:    true,
			EnablePolish:        true,
			ValidationTimeout:   120,
			PolishTimeout:       60,
			MinConfidenceToSkip: 0.85,
			MaxValidationRounds: 3,
			ShowPhaseIndicators: true,
		},
		Metadata: map[string]interface{}{
			"category":    "integration_test",
			"priority":    "high",
			"test":        true,
		},
	}
	body, _ := json.Marshal(createBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var createResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &createResponse)
	require.NoError(t, err)
	assert.Equal(t, "integration-debate", createResponse["debate_id"])

	// Get debate
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/v1/debates/integration-debate", nil)
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var getResponse map[string]interface{}
	err = json.Unmarshal(w2.Body.Bytes(), &getResponse)
	require.NoError(t, err)
	assert.Equal(t, "Complex integration test debate", getResponse["topic"])
	assert.Equal(t, float64(5), getResponse["max_rounds"])
	assert.Equal(t, true, getResponse["enable_multi_pass_validation"])

	// Get status
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/v1/debates/integration-debate/status", nil)
	router.ServeHTTP(w3, req3)

	assert.Equal(t, http.StatusOK, w3.Code)

	// Get gates
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodGet, "/v1/debates/integration-debate/gates", nil)
	router.ServeHTTP(w4, req4)

	assert.Equal(t, http.StatusOK, w4.Code)

	// Get audit
	w5 := httptest.NewRecorder()
	req5 := httptest.NewRequest(http.MethodGet, "/v1/debates/integration-debate/audit", nil)
	router.ServeHTTP(w5, req5)

	assert.Equal(t, http.StatusOK, w5.Code)

	// Delete debate
	w6 := httptest.NewRecorder()
	req6 := httptest.NewRequest(http.MethodDelete, "/v1/debates/integration-debate", nil)
	router.ServeHTTP(w6, req6)

	assert.Equal(t, http.StatusOK, w6.Code)
}

// TestIntegration_ErrorResponseFormat tests error response format consistency
func TestIntegration_ErrorResponseFormat(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	testCases := []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
	}{
		{
			name:           "invalid completion request",
			method:         http.MethodPost,
			path:           "/v1/completions",
			body:           `{"invalid": "request"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid debate request",
			method:         http.MethodPost,
			path:           "/v1/debates",
			body:           `{"invalid": "request"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "debate not found",
			method:         http.MethodGet,
			path:           "/v1/debates/non-existent-id",
			body:           "",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			var body *bytes.Buffer
			if tc.body != "" {
				body = bytes.NewBufferString(tc.body)
			} else {
				body = &bytes.Buffer{}
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, body)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Contains(t, response, "error")
		})
	}
}

// TestIntegration_RequestResponseCycle tests complete request-response cycles
func TestIntegration_RequestResponseCycle(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	// Test 1: Completion request-response
	t.Run("completion cycle", func(t *testing.T) {
			t.Parallel()
		reqBody := CompletionRequest{
			Prompt:      "What is 2+2?",
			Temperature: 0.5,
			MaxTokens:   50,
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
		assert.NotEmpty(t, response.Choices)
	})

	// Test 2: Debate request-response
	t.Run("debate cycle", func(t *testing.T) {
			t.Parallel()
		reqBody := CreateDebateRequest{
			Topic: "Test topic",
			Participants: []ParticipantConfigRequest{
				{Name: "A"},
				{Name: "B"},
			},
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/debates", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "debate_id")
		assert.Equal(t, "pending", response["status"])
	})

	// Test 3: MCP request-response
	t.Run("mcp cycle", func(t *testing.T) {
			t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/mcp/capabilities", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "version")
	})
}

// TestIntegration_URLRouting tests URL routing patterns
func TestIntegration_URLRouting(t *testing.T) {
	t.Parallel()
	router, _ := setupIntegrationTest()

	// Test completion routes
	t.Run("POST_/v1/completions", func(t *testing.T) {
			t.Parallel()
		reqBody := CompletionRequest{Prompt: "Test"}
		b, _ := json.Marshal(reqBody)
		body := bytes.NewBuffer(b)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/completions", body)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GET_/v1/models", func(t *testing.T) {
			t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("POST_/v1_debates", func(t *testing.T) {
			t.Parallel()
		reqBody := CreateDebateRequest{
			Topic: "Test",
			Participants: []ParticipantConfigRequest{
				{Name: "A"},
				{Name: "B"},
			},
		}
		b, _ := json.Marshal(reqBody)
		body := bytes.NewBuffer(b)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/debates", body)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
	})

	t.Run("GET_/v1_debates", func(t *testing.T) {
			t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/debates", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestIntegration_JSONMarshaling tests JSON marshaling/unmarshaling
func TestIntegration_JSONMarshaling(t *testing.T) {
	t.Parallel()
	// Test CompletionRequest marshaling
	t.Run("completion request", func(t *testing.T) {
			t.Parallel()
		req := CompletionRequest{
			Prompt:      "Test",
			Model:       "test-model",
			Temperature: 0.7,
			MaxTokens:   100,
			TopP:        0.9,
			Stop:        []string{"STOP", "END"},
			Stream:      false,
			Messages: []models.Message{
				{Role: "user", Content: "Hello"},
			},
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled CompletionRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Prompt, unmarshaled.Prompt)
		assert.Equal(t, req.Model, unmarshaled.Model)
		assert.Equal(t, req.Temperature, unmarshaled.Temperature)
		assert.Equal(t, req.MaxTokens, unmarshaled.MaxTokens)
		assert.Equal(t, len(req.Stop), len(unmarshaled.Stop))
	})

	// Test CreateDebateRequest marshaling
	t.Run("debate request", func(t *testing.T) {
			t.Parallel()
		req := CreateDebateRequest{
			Topic: "Test",
			Participants: []ParticipantConfigRequest{
				{Name: "A"},
				{Name: "B"},
			},
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled CreateDebateRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Topic, unmarshaled.Topic)
		assert.Len(t, unmarshaled.Participants, 2)
	})
}

// Mock for skills integration testing
type mockSkillsService struct{}

func (m *mockSkillsService) FindSkills(ctx interface{}, userInput string) ([]*skills.SkillMatch, error) {
	return []*skills.SkillMatch{}, nil
}

func (m *mockSkillsService) GetConfig() *skills.SkillConfig {
	return &skills.SkillConfig{MinConfidence: 0.5}
}

func (m *mockSkillsService) StartSkillExecution(requestID string, skill *skills.Skill, match *skills.SkillMatch) *skills.SkillUsage {
	return &skills.SkillUsage{}
}

func (m *mockSkillsService) CompleteSkillExecution(requestID string, success bool, errorMsg string) *skills.SkillUsage {
	return &skills.SkillUsage{}
}
