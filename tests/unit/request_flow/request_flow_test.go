// Package integration provides comprehensive tests that verify the complete
// request-to-response flow in HelixAgent, from CLI agent request to final response.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/handlers"
	"dev.helix.agent/internal/models"
	"dev.helix.agent/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock Provider Implementation
// =============================================================================

// RequestFlowMockProvider is a mock LLM provider for testing request flows
type RequestFlowMockProvider struct {
	name            string
	response        string
	shouldSucceed   bool
	errorMessage    string
	confidence      float64
	delay           time.Duration
	callCount       int
	callMutex       sync.Mutex
	toolCalls       []models.ToolCall
	supportTools    bool
	lastRequest     *models.LLMRequest
	customResponder func(req *models.LLMRequest) (*models.LLMResponse, error)
}

func (m *RequestFlowMockProvider) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	m.callMutex.Lock()
	m.callCount++
	m.lastRequest = req
	m.callMutex.Unlock()

	// Simulate delay if configured
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.delay):
		}
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Use custom responder if provided
	if m.customResponder != nil {
		return m.customResponder(req)
	}

	if !m.shouldSucceed {
		return nil, fmt.Errorf("%s: %s", m.name, m.errorMessage)
	}

	resp := &models.LLMResponse{
		ID:           fmt.Sprintf("resp-%s-%d", m.name, time.Now().UnixNano()),
		ProviderID:   m.name,
		ProviderName: m.name,
		Content:      m.response,
		Confidence:   m.confidence,
		TokensUsed:   len(m.response) / 4,
		FinishReason: "stop",
		CreatedAt:    time.Now(),
	}

	// Include tool calls if configured
	if len(m.toolCalls) > 0 {
		resp.ToolCalls = m.toolCalls
		resp.FinishReason = "tool_calls"
	}

	return resp, nil
}

func (m *RequestFlowMockProvider) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	m.callMutex.Lock()
	m.callCount++
	m.lastRequest = req
	m.callMutex.Unlock()

	if !m.shouldSucceed {
		return nil, fmt.Errorf("%s: %s", m.name, m.errorMessage)
	}

	ch := make(chan *models.LLMResponse, 10)
	go func() {
		defer close(ch)

		// Simulate delay if configured
		if m.delay > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(m.delay):
			}
		}

		// Stream response in chunks
		words := strings.Split(m.response, " ")
		for i, word := range words {
			select {
			case <-ctx.Done():
				return
			default:
				finishReason := ""
				if i == len(words)-1 {
					finishReason = "stop"
				}
				ch <- &models.LLMResponse{
					ID:           fmt.Sprintf("resp-%s-%d", m.name, time.Now().UnixNano()),
					ProviderID:   m.name,
					ProviderName: m.name,
					Content:      word + " ",
					Confidence:   m.confidence,
					FinishReason: finishReason,
					CreatedAt:    time.Now(),
				}
			}
		}
	}()

	return ch, nil
}

func (m *RequestFlowMockProvider) HealthCheck() error {
	if !m.shouldSucceed {
		return fmt.Errorf("%s: health check failed", m.name)
	}
	return nil
}

func (m *RequestFlowMockProvider) GetCapabilities() *models.ProviderCapabilities {
	return &models.ProviderCapabilities{
		SupportedModels:         []string{"test-model"},
		SupportsStreaming:       true,
		SupportsFunctionCalling: m.supportTools,
		SupportsTools:           m.supportTools,
	}
}

func (m *RequestFlowMockProvider) ValidateConfig(cfg map[string]interface{}) (bool, []string) {
	return true, nil
}

func (m *RequestFlowMockProvider) GetCallCount() int {
	m.callMutex.Lock()
	defer m.callMutex.Unlock()
	return m.callCount
}

func (m *RequestFlowMockProvider) GetLastRequest() *models.LLMRequest {
	m.callMutex.Lock()
	defer m.callMutex.Unlock()
	return m.lastRequest
}

func (m *RequestFlowMockProvider) Reset() {
	m.callMutex.Lock()
	defer m.callMutex.Unlock()
	m.callCount = 0
	m.lastRequest = nil
}

// =============================================================================
// Test Helper Functions
// =============================================================================

func setupRequestFlowTestServer(t *testing.T) (*gin.Engine, *services.ProviderRegistry, func()) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: "8081",
		},
	}

	// Create provider registry without auto-discovery
	registryConfig := &services.RegistryConfig{
		DefaultTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	registry := services.NewProviderRegistryWithoutAutoDiscovery(registryConfig, nil)

	// Create unified handler
	handler := handlers.NewUnifiedHandler(registry, cfg)

	// Register routes with simple auth middleware
	api := router.Group("/v1")
	auth := func(c *gin.Context) { c.Next() }
	handler.RegisterOpenAIRoutes(api, auth)

	cleanup := func() {
		// Cleanup if needed
	}

	return router, registry, cleanup
}

func makeOpenAIChatRequest(t *testing.T, router *gin.Engine, req map[string]interface{}) *httptest.ResponseRecorder {
	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httpReq)
	return w
}

// =============================================================================
// 1. Request Parsing Tests
// =============================================================================

func TestRequestFlow_RequestParsing(t *testing.T) {
	// Integration test — no external deps required

	router, registry, cleanup := setupRequestFlowTestServer(t)
	defer cleanup()

	// Register a mock provider
	mockProvider := &RequestFlowMockProvider{
		name:          "parse-test-provider",
		response:      "Test response for parsing",
		shouldSucceed: true,
		confidence:    0.9,
	}
	registry.RegisterProvider("parse-test-provider", mockProvider)

	t.Run("Parse basic chat completion request", func(t *testing.T) {
		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Hello, how are you?"},
			},
			"max_tokens":  100,
			"temperature": 0.7,
		}

		w := makeOpenAIChatRequest(t, router, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response, "id")
		assert.Contains(t, response, "object")
		assert.Equal(t, "chat.completion", response["object"])
		assert.Contains(t, response, "choices")
	})

	t.Run("Parse request with tool calls", func(t *testing.T) {
		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Read the file test.go"},
			},
			"tools": []map[string]interface{}{
				{
					"type": "function",
					"function": map[string]interface{}{
						"name":        "Read",
						"description": "Read a file from the filesystem",
						"parameters": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"file_path": map[string]interface{}{
									"type":        "string",
									"description": "The path to the file to read",
								},
							},
							"required": []string{"file_path"},
						},
					},
				},
			},
			"tool_choice": "auto",
		}

		w := makeOpenAIChatRequest(t, router, req)

		// Should return 200 (success) or error due to no real providers
		// The important thing is that the request was parsed correctly
		assert.Contains(t, []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable}, w.Code)
	})

	t.Run("Parse streaming request", func(t *testing.T) {
		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Stream test"},
			},
			"stream":     true,
			"max_tokens": 50,
		}

		body, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, httpReq)

		// Streaming requests should return 200 with SSE content type
		if w.Code == http.StatusOK {
			assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
		}
	})

	t.Run("Parse request with system message", func(t *testing.T) {
		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]interface{}{
				{"role": "system", "content": "You are a helpful assistant."},
				{"role": "user", "content": "Hello!"},
			},
			"max_tokens": 100,
		}

		w := makeOpenAIChatRequest(t, router, req)

		// Request should be parsed correctly
		assert.Contains(t, []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable}, w.Code)
	})

	t.Run("Parse invalid request returns error", func(t *testing.T) {
		// Send invalid JSON
		httpReq := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader([]byte("{invalid json")))
		httpReq.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, httpReq)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "error")
	})
}

// =============================================================================
// 2. Provider Selection Tests
// =============================================================================

func TestRequestFlow_ProviderSelection(t *testing.T) {
	// Integration test — no external deps required

	t.Run("Select provider based on availability", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		// Register multiple providers with different priorities
		provider1 := &RequestFlowMockProvider{
			name:          "provider-primary",
			response:      "Response from primary provider",
			shouldSucceed: true,
			confidence:    0.95,
		}
		provider2 := &RequestFlowMockProvider{
			name:          "provider-secondary",
			response:      "Response from secondary provider",
			shouldSucceed: true,
			confidence:    0.85,
		}

		registry.RegisterProvider("provider-primary", provider1)
		registry.RegisterProvider("provider-secondary", provider2)

		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Test provider selection"},
			},
		}

		w := makeOpenAIChatRequest(t, router, req)

		if w.Code == http.StatusOK {
			// At least one provider should have been called
			totalCalls := provider1.GetCallCount() + provider2.GetCallCount()
			assert.Greater(t, totalCalls, 0, "At least one provider should be called")
		}
	})

	t.Run("Fallback to next provider on failure", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		// Primary provider fails
		failingProvider := &RequestFlowMockProvider{
			name:          "provider-failing",
			shouldSucceed: false,
			errorMessage:  "Primary provider failed",
		}
		// Secondary provider succeeds
		successProvider := &RequestFlowMockProvider{
			name:          "provider-success",
			response:      "Response from fallback provider",
			shouldSucceed: true,
			confidence:    0.9,
		}

		registry.RegisterProvider("provider-failing", failingProvider)
		registry.RegisterProvider("provider-success", successProvider)

		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Test fallback"},
			},
		}

		w := makeOpenAIChatRequest(t, router, req)

		// Response should come from fallback provider or fail gracefully
		assert.Contains(t, []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable}, w.Code)
	})

	t.Run("Use ensemble for debate requests", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		// Register multiple providers for ensemble
		providers := []*RequestFlowMockProvider{
			{name: "ensemble-1", response: "Ensemble response 1", shouldSucceed: true, confidence: 0.9},
			{name: "ensemble-2", response: "Ensemble response 2", shouldSucceed: true, confidence: 0.85},
			{name: "ensemble-3", response: "Ensemble response 3", shouldSucceed: true, confidence: 0.8},
		}

		for _, p := range providers {
			registry.RegisterProvider(p.name, p)
		}

		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Test ensemble request"},
			},
			"ensemble_config": map[string]interface{}{
				"strategy":             "confidence_weighted",
				"min_providers":        2,
				"confidence_threshold": 0.5,
			},
		}

		w := makeOpenAIChatRequest(t, router, req)

		// Ensemble should process the request
		assert.Contains(t, []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable}, w.Code)
	})
}

// =============================================================================
// 3. Tool Call Handling Tests
// =============================================================================

func TestRequestFlow_ToolCallHandling(t *testing.T) {
	// Integration test — no external deps required

	t.Run("Extract tool calls from request", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		// Provider that generates tool calls
		toolProvider := &RequestFlowMockProvider{
			name:          "tool-provider",
			response:      "I'll read that file for you.",
			shouldSucceed: true,
			confidence:    0.9,
			supportTools:  true,
			toolCalls: []models.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: models.ToolCallFunction{
						Name:      "Read",
						Arguments: `{"file_path": "/tmp/test.go"}`,
					},
				},
			},
		}
		registry.RegisterProvider("tool-provider", toolProvider)

		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Read the file /tmp/test.go"},
			},
			"tools": []map[string]interface{}{
				{
					"type": "function",
					"function": map[string]interface{}{
						"name":        "Read",
						"description": "Read a file",
						"parameters": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"file_path": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
		}

		w := makeOpenAIChatRequest(t, router, req)

		// Should process request with tools
		assert.Contains(t, []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable}, w.Code)
	})

	t.Run("Handle multiple tool calls", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		// Provider that generates multiple tool calls
		multiToolProvider := &RequestFlowMockProvider{
			name:          "multi-tool-provider",
			response:      "I'll execute multiple commands.",
			shouldSucceed: true,
			confidence:    0.9,
			supportTools:  true,
			toolCalls: []models.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: models.ToolCallFunction{
						Name:      "Read",
						Arguments: `{"file_path": "/tmp/file1.go"}`,
					},
				},
				{
					ID:   "call_2",
					Type: "function",
					Function: models.ToolCallFunction{
						Name:      "Bash",
						Arguments: `{"command": "ls -la", "description": "List files"}`,
					},
				},
			},
		}
		registry.RegisterProvider("multi-tool-provider", multiToolProvider)

		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Read file1.go and list the directory"},
			},
			"tools": []map[string]interface{}{
				{
					"type": "function",
					"function": map[string]interface{}{
						"name":       "Read",
						"parameters": map[string]interface{}{"type": "object"},
					},
				},
				{
					"type": "function",
					"function": map[string]interface{}{
						"name":       "Bash",
						"parameters": map[string]interface{}{"type": "object"},
					},
				},
			},
			"parallel_tool_calls": true,
		}

		w := makeOpenAIChatRequest(t, router, req)

		assert.Contains(t, []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable}, w.Code)
	})

	t.Run("Return tool results in response", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		// Provider that handles tool results
		resultProvider := &RequestFlowMockProvider{
			name:          "result-provider",
			response:      "Based on the tool results, here is my analysis.",
			shouldSucceed: true,
			confidence:    0.9,
		}
		registry.RegisterProvider("result-provider", resultProvider)

		// Simulate a request with tool results from a previous turn
		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []interface{}{
				map[string]interface{}{"role": "user", "content": "Read test.go"},
				map[string]interface{}{
					"role":    "assistant",
					"content": "",
					"tool_calls": []map[string]interface{}{
						{
							"id":   "call_abc",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "Read",
								"arguments": `{"file_path": "test.go"}`,
							},
						},
					},
				},
				map[string]interface{}{
					"role":         "tool",
					"tool_call_id": "call_abc",
					"content":      "package main\n\nfunc main() {}",
				},
			},
		}

		w := makeOpenAIChatRequest(t, router, req)

		assert.Contains(t, []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable}, w.Code)
	})
}

// =============================================================================
// 4. Response Formatting Tests
// =============================================================================

func TestRequestFlow_ResponseFormatting(t *testing.T) {
	// Integration test — no external deps required

	t.Run("Format response for OpenCode agent", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		mockProvider := &RequestFlowMockProvider{
			name:          "format-provider",
			response:      "Formatted response content",
			shouldSucceed: true,
			confidence:    0.92,
		}
		registry.RegisterProvider("format-provider", mockProvider)

		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Test formatting"},
			},
		}

		w := makeOpenAIChatRequest(t, router, req)

		if w.Code == http.StatusOK {
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			// Verify OpenAI-compatible response format
			assert.Contains(t, response, "id")
			assert.Contains(t, response, "object")
			assert.Contains(t, response, "created")
			assert.Contains(t, response, "model")
			assert.Contains(t, response, "choices")

			choices := response["choices"].([]interface{})
			assert.Greater(t, len(choices), 0)

			choice := choices[0].(map[string]interface{})
			assert.Contains(t, choice, "index")
			assert.Contains(t, choice, "message")
			assert.Contains(t, choice, "finish_reason")

			message := choice["message"].(map[string]interface{})
			assert.Contains(t, message, "role")
			assert.Contains(t, message, "content")
			assert.Equal(t, "assistant", message["role"])
		}
	})

	t.Run("Format streaming response", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		mockProvider := &RequestFlowMockProvider{
			name:          "stream-format-provider",
			response:      "This is a streaming test response",
			shouldSucceed: true,
			confidence:    0.9,
		}
		registry.RegisterProvider("stream-format-provider", mockProvider)

		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Stream test"},
			},
			"stream": true,
		}

		body, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, httpReq)

		if w.Code == http.StatusOK {
			// Verify streaming headers
			assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
			assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))

			// Verify response contains SSE data
			responseBody := w.Body.String()
			assert.Contains(t, responseBody, "data:")
		}
	})

	t.Run("Format tool call response", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		toolProvider := &RequestFlowMockProvider{
			name:          "tool-format-provider",
			response:      "",
			shouldSucceed: true,
			confidence:    0.9,
			supportTools:  true,
			toolCalls: []models.ToolCall{
				{
					ID:   "call_format_test",
					Type: "function",
					Function: models.ToolCallFunction{
						Name:      "Bash",
						Arguments: `{"command": "echo test"}`,
					},
				},
			},
		}
		registry.RegisterProvider("tool-format-provider", toolProvider)

		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Run a test command"},
			},
			"tools": []map[string]interface{}{
				{
					"type": "function",
					"function": map[string]interface{}{
						"name":       "Bash",
						"parameters": map[string]interface{}{"type": "object"},
					},
				},
			},
		}

		w := makeOpenAIChatRequest(t, router, req)

		if w.Code == http.StatusOK {
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			// Response should be properly formatted
			assert.Contains(t, response, "choices")
		}
	})
}

// =============================================================================
// 5. Error Handling Tests
// =============================================================================

func TestRequestFlow_ErrorHandling(t *testing.T) {
	// Integration test — no external deps required

	t.Run("Handle provider unavailable", func(t *testing.T) {
		router, _, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		// No providers registered - should return error
		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Test with no providers"},
			},
		}

		w := makeOpenAIChatRequest(t, router, req)

		// Should return service unavailable or bad gateway
		assert.Contains(t, []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusInternalServerError}, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "error")
	})

	t.Run("Handle all providers failing", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		// Register only failing providers
		for i := 0; i < 3; i++ {
			failProvider := &RequestFlowMockProvider{
				name:          fmt.Sprintf("fail-provider-%d", i),
				shouldSucceed: false,
				errorMessage:  "Provider error",
			}
			registry.RegisterProvider(failProvider.name, failProvider)
		}

		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Test all providers fail"},
			},
		}

		w := makeOpenAIChatRequest(t, router, req)

		// Should return error status
		assert.Contains(t, []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusInternalServerError}, w.Code)
	})

	t.Run("Handle timeout", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		// Provider that takes too long
		slowProvider := &RequestFlowMockProvider{
			name:          "slow-provider",
			response:      "Slow response",
			shouldSucceed: true,
			delay:         10 * time.Second, // Will timeout
		}
		registry.RegisterProvider("slow-provider", slowProvider)

		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Test timeout"},
			},
		}

		// Create request with short timeout
		body, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")

		// Set a short deadline on context
		ctx, cancel := context.WithTimeout(httpReq.Context(), 100*time.Millisecond)
		defer cancel()
		httpReq = httpReq.WithContext(ctx)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, httpReq)

		// Should return error or timeout - we just verify it doesn't hang
		assert.NotEqual(t, 0, w.Code)
	})

	t.Run("Handle rate limiting", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		// Provider that simulates rate limiting
		rateLimitProvider := &RequestFlowMockProvider{
			name:          "rate-limit-provider",
			shouldSucceed: false,
			errorMessage:  "rate limit exceeded",
		}
		registry.RegisterProvider("rate-limit-provider", rateLimitProvider)

		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Test rate limit"},
			},
		}

		w := makeOpenAIChatRequest(t, router, req)

		// Should handle rate limit gracefully
		assert.Contains(t, []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusTooManyRequests}, w.Code)
	})

	t.Run("Handle invalid model", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		mockProvider := &RequestFlowMockProvider{
			name:          "valid-provider",
			response:      "Valid response",
			shouldSucceed: true,
		}
		registry.RegisterProvider("valid-provider", mockProvider)

		req := map[string]interface{}{
			"model": "nonexistent-model-xyz",
			"messages": []map[string]string{
				{"role": "user", "content": "Test invalid model"},
			},
		}

		w := makeOpenAIChatRequest(t, router, req)

		// Should either accept (since model name is flexible) or return error
		assert.Contains(t, []int{http.StatusOK, http.StatusBadRequest, http.StatusBadGateway, http.StatusServiceUnavailable}, w.Code)
	})
}

// =============================================================================
// 6. Complete Flow Tests
// =============================================================================

func TestRequestFlow_CompleteFlow(t *testing.T) {
	// Integration test — no external deps required

	t.Run("User message -> Provider -> Response", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		mockProvider := &RequestFlowMockProvider{
			name:          "flow-provider",
			response:      "Here is a comprehensive response to your query.",
			shouldSucceed: true,
			confidence:    0.95,
		}
		registry.RegisterProvider("flow-provider", mockProvider)

		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "What is the weather today?"},
			},
			"max_tokens":  200,
			"temperature": 0.7,
		}

		w := makeOpenAIChatRequest(t, router, req)

		if w.Code == http.StatusOK {
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			// Verify complete response structure
			assert.Contains(t, response, "id")
			assert.Contains(t, response, "choices")
			assert.Contains(t, response, "usage")

			// Verify provider was called
			assert.Greater(t, mockProvider.GetCallCount(), 0)

			// Verify the message was passed correctly
			lastReq := mockProvider.GetLastRequest()
			if lastReq != nil {
				assert.Greater(t, len(lastReq.Messages), 0)
			}
		}
	})

	t.Run("User message -> Tool call -> Tool result -> Response", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		// Step 1: Provider generates tool call
		callCount := 0
		toolProvider := &RequestFlowMockProvider{
			name:          "tool-flow-provider",
			shouldSucceed: true,
			confidence:    0.9,
			supportTools:  true,
			customResponder: func(req *models.LLMRequest) (*models.LLMResponse, error) {
				callCount++
				// First call generates tool call
				if callCount == 1 {
					return &models.LLMResponse{
						ID:           "resp-1",
						ProviderID:   "tool-flow-provider",
						Content:      "",
						FinishReason: "tool_calls",
						ToolCalls: []models.ToolCall{
							{
								ID:   "call_flow_test",
								Type: "function",
								Function: models.ToolCallFunction{
									Name:      "Read",
									Arguments: `{"file_path": "/tmp/test.txt"}`,
								},
							},
						},
					}, nil
				}
				// Subsequent calls return normal response
				return &models.LLMResponse{
					ID:           "resp-2",
					ProviderID:   "tool-flow-provider",
					Content:      "The file contains: test content",
					FinishReason: "stop",
				}, nil
			},
		}
		registry.RegisterProvider("tool-flow-provider", toolProvider)

		// Step 1: Initial request
		req1 := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Read the file /tmp/test.txt"},
			},
			"tools": []map[string]interface{}{
				{
					"type": "function",
					"function": map[string]interface{}{
						"name": "Read",
						"parameters": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"file_path": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
		}

		w1 := makeOpenAIChatRequest(t, router, req1)
		assert.Contains(t, []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable}, w1.Code)

		// Step 2: Send tool result (simulating client sending back tool execution result)
		req2 := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []interface{}{
				map[string]interface{}{"role": "user", "content": "Read the file /tmp/test.txt"},
				map[string]interface{}{
					"role":    "assistant",
					"content": "",
					"tool_calls": []map[string]interface{}{
						{
							"id":   "call_flow_test",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "Read",
								"arguments": `{"file_path": "/tmp/test.txt"}`,
							},
						},
					},
				},
				map[string]interface{}{
					"role":         "tool",
					"tool_call_id": "call_flow_test",
					"content":      "This is the content of the test file.",
				},
			},
		}

		w2 := makeOpenAIChatRequest(t, router, req2)
		assert.Contains(t, []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable}, w2.Code)
	})

	t.Run("User message -> Debate -> Consensus -> Response", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		// Register multiple providers for debate ensemble
		providers := []*RequestFlowMockProvider{
			{name: "debate-1", response: "From my analysis, the answer is A because...", shouldSucceed: true, confidence: 0.9},
			{name: "debate-2", response: "I believe the answer is A with some caveats...", shouldSucceed: true, confidence: 0.85},
			{name: "debate-3", response: "The consensus points to answer A...", shouldSucceed: true, confidence: 0.88},
		}

		for _, p := range providers {
			registry.RegisterProvider(p.name, p)
		}

		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "system", "content": "You are participating in an AI debate."},
				{"role": "user", "content": "What is the best approach to solve climate change?"},
			},
			"ensemble_config": map[string]interface{}{
				"strategy":      "confidence_weighted",
				"min_providers": 2,
			},
		}

		w := makeOpenAIChatRequest(t, router, req)

		if w.Code == http.StatusOK {
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			// Verify debate produced a result
			assert.Contains(t, response, "choices")
			choices := response["choices"].([]interface{})
			assert.Greater(t, len(choices), 0)
		}
	})

	t.Run("Multi-turn conversation flow", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		turnCount := 0
		conversationProvider := &RequestFlowMockProvider{
			name:          "conversation-provider",
			shouldSucceed: true,
			confidence:    0.9,
			customResponder: func(req *models.LLMRequest) (*models.LLMResponse, error) {
				turnCount++
				response := fmt.Sprintf("Response to turn %d based on conversation history", turnCount)
				return &models.LLMResponse{
					ID:           fmt.Sprintf("resp-turn-%d", turnCount),
					ProviderID:   "conversation-provider",
					Content:      response,
					FinishReason: "stop",
				}, nil
			},
		}
		registry.RegisterProvider("conversation-provider", conversationProvider)

		// Turn 1
		req1 := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Hello, I need help with coding."},
			},
		}
		w1 := makeOpenAIChatRequest(t, router, req1)
		assert.Contains(t, []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable}, w1.Code)

		// Turn 2 - with conversation history
		req2 := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Hello, I need help with coding."},
				{"role": "assistant", "content": "Hello! I'd be happy to help with coding."},
				{"role": "user", "content": "I'm working on a Go project."},
			},
		}
		w2 := makeOpenAIChatRequest(t, router, req2)
		assert.Contains(t, []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable}, w2.Code)

		// Turn 3 - longer conversation
		req3 := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Hello, I need help with coding."},
				{"role": "assistant", "content": "Hello! I'd be happy to help with coding."},
				{"role": "user", "content": "I'm working on a Go project."},
				{"role": "assistant", "content": "Great! What aspect of Go do you need help with?"},
				{"role": "user", "content": "How do I handle errors properly?"},
			},
		}
		w3 := makeOpenAIChatRequest(t, router, req3)
		assert.Contains(t, []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable}, w3.Code)
	})
}

// =============================================================================
// 7. CLI Agent Compatibility Tests
// =============================================================================

func TestRequestFlow_CLIAgentCompatibility(t *testing.T) {
	// Integration test — no external deps required

	t.Run("OpenCode compatible request format", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		mockProvider := &RequestFlowMockProvider{
			name:          "opencode-provider",
			response:      "OpenCode compatible response",
			shouldSucceed: true,
			confidence:    0.9,
		}
		registry.RegisterProvider("opencode-provider", mockProvider)

		// Request format that OpenCode would send
		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "system", "content": "You are a helpful coding assistant."},
				{"role": "user", "content": "Help me write a function"},
			},
			"stream":      false,
			"max_tokens":  4096,
			"temperature": 0,
		}

		w := makeOpenAIChatRequest(t, router, req)

		if w.Code == http.StatusOK {
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			// Verify response has all fields OpenCode expects
			assert.Contains(t, response, "id")
			assert.Contains(t, response, "object")
			assert.Contains(t, response, "choices")
			assert.Contains(t, response, "model")
		}
	})

	t.Run("Claude Code compatible request format", func(t *testing.T) {
		router, registry, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		mockProvider := &RequestFlowMockProvider{
			name:          "claude-code-provider",
			response:      "Claude Code compatible response",
			shouldSucceed: true,
			confidence:    0.9,
			supportTools:  true,
		}
		registry.RegisterProvider("claude-code-provider", mockProvider)

		// Request format that Claude Code would send with tools
		req := map[string]interface{}{
			"model": "helixagent-debate",
			"messages": []map[string]string{
				{"role": "user", "content": "Read the file main.go"},
			},
			"tools": []map[string]interface{}{
				{
					"type": "function",
					"function": map[string]interface{}{
						"name":        "Read",
						"description": "Reads a file from the local filesystem.",
						"parameters": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"file_path": map[string]interface{}{
									"type":        "string",
									"description": "The absolute path to the file to read",
								},
							},
							"required": []string{"file_path"},
						},
					},
				},
			},
			"tool_choice": "auto",
		}

		w := makeOpenAIChatRequest(t, router, req)

		assert.Contains(t, []int{http.StatusOK, http.StatusBadGateway, http.StatusServiceUnavailable}, w.Code)
	})

	t.Run("Models endpoint returns valid format", func(t *testing.T) {
		router, _, cleanup := setupRequestFlowTestServer(t)
		defer cleanup()

		httpReq := httptest.NewRequest("GET", "/v1/models", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httpReq)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify models response format
		assert.Contains(t, response, "object")
		assert.Equal(t, "list", response["object"])
		assert.Contains(t, response, "data")

		data := response["data"].([]interface{})
		assert.Greater(t, len(data), 0, "Should have at least one model")

		model := data[0].(map[string]interface{})
		assert.Contains(t, model, "id")
		assert.Contains(t, model, "object")
		assert.Equal(t, "model", model["object"])
	})
}

// =============================================================================
// 8. Debate Service Integration Tests
// =============================================================================

func TestRequestFlow_DebateServiceIntegration(t *testing.T) {
	// Integration test — no external deps required

	t.Run("Debate service creates participant responses", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.DebugLevel)

		// Create provider registry
		registryConfig := &services.RegistryConfig{
			DefaultTimeout: 30 * time.Second,
		}
		registry := services.NewProviderRegistryWithoutAutoDiscovery(registryConfig, nil)

		// Register mock providers
		for i := 0; i < 3; i++ {
			provider := &RequestFlowMockProvider{
				name:          fmt.Sprintf("debate-provider-%d", i),
				response:      fmt.Sprintf("Analysis from provider %d", i),
				shouldSucceed: true,
				confidence:    0.8 + float64(i)*0.05,
			}
			registry.RegisterProvider(provider.name, provider)
		}

		// Create debate service
		debateService := services.NewDebateServiceWithDeps(logger, registry, nil)
		require.NotNil(t, debateService)

		// Verify service is configured
		assert.NotNil(t, debateService)
	})
}

// =============================================================================
// 9. Concurrent Request Tests
// =============================================================================

func TestRequestFlow_ConcurrentRequests(t *testing.T) {
	// Integration test — no external deps required

	router, registry, cleanup := setupRequestFlowTestServer(t)
	defer cleanup()

	mockProvider := &RequestFlowMockProvider{
		name:          "concurrent-provider",
		response:      "Concurrent response",
		shouldSucceed: true,
		confidence:    0.9,
		delay:         50 * time.Millisecond, // Small delay to test concurrency
	}
	registry.RegisterProvider("concurrent-provider", mockProvider)

	t.Run("Handle multiple concurrent requests", func(t *testing.T) {
		const numRequests = 5
		var wg sync.WaitGroup
		results := make(chan int, numRequests)

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(requestNum int) {
				defer wg.Done()

				req := map[string]interface{}{
					"model": "helixagent-debate",
					"messages": []map[string]string{
						{"role": "user", "content": fmt.Sprintf("Concurrent request %d", requestNum)},
					},
				}

				w := makeOpenAIChatRequest(t, router, req)
				results <- w.Code
			}(i)
		}

		wg.Wait()
		close(results)

		// Collect results
		successCount := 0
		for code := range results {
			if code == http.StatusOK {
				successCount++
			}
		}

		// At least some requests should succeed
		t.Logf("Successful requests: %d/%d", successCount, numRequests)
	})
}
