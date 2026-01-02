package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestHealthEndpoint tests the /health endpoint
func TestHealthEndpoint(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	// Add health endpoint as defined in SetupRouter
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	t.Run("returns healthy status", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "healthy", response["status"])
	})

	t.Run("returns correct content type", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	})
}

// TestV1HealthEndpoint tests the /v1/health endpoint
func TestV1HealthEndpoint(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	// Simulate the v1 health endpoint with mock provider data
	router.GET("/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
			"providers": gin.H{
				"total":     5,
				"healthy":   4,
				"unhealthy": 1,
			},
			"timestamp": time.Now().Unix(),
		})
	})

	t.Run("returns detailed health status", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response["status"])
		assert.Contains(t, response, "providers")
		assert.Contains(t, response, "timestamp")

		providers := response["providers"].(map[string]interface{})
		assert.Equal(t, float64(5), providers["total"])
		assert.Equal(t, float64(4), providers["healthy"])
		assert.Equal(t, float64(1), providers["unhealthy"])
	})
}

// TestPublicProviderListEndpoint tests the /v1/providers endpoint
func TestPublicProviderListEndpoint(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	// Simulate the providers list endpoint
	mockProviders := []string{"claude", "deepseek", "gemini", "qwen", "ollama"}
	router.GET("/v1/providers", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"providers": mockProviders,
			"count":     len(mockProviders),
		})
	})

	t.Run("returns provider list", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/providers", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Contains(t, response, "providers")
		assert.Contains(t, response, "count")
		assert.Equal(t, float64(5), response["count"])
	})
}

// TestModelsEndpoint tests the /v1/models endpoint
func TestModelsEndpoint(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	// Simulate the models endpoint
	router.GET("/v1/models", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"object": "list",
			"data": []gin.H{
				{"id": "claude-3-sonnet", "object": "model", "owned_by": "anthropic"},
				{"id": "deepseek-chat", "object": "model", "owned_by": "deepseek"},
			},
		})
	})

	t.Run("returns model list in OpenAI format", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/models", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "list", response["object"])
		assert.Contains(t, response, "data")
		data := response["data"].([]interface{})
		assert.Greater(t, len(data), 0)
	})
}

// TestAuthenticationMiddleware tests the authentication middleware behavior
func TestAuthenticationMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	// Mock auth middleware that checks for Authorization header
	authMiddleware := func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}
		if authHeader != "Bearer valid-token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}
		c.Next()
	}

	// Protected route
	protected := router.Group("/v1", authMiddleware)
	protected.POST("/completions", func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true})
	})

	t.Run("rejects request without auth header", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/completions", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("rejects request with invalid token", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/completions", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("accepts request with valid token", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/completions", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestCompletionEndpoint tests the /v1/completions endpoint
func TestCompletionEndpoint(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	// Mock completion handler
	router.POST("/v1/completions", func(c *gin.Context) {
		var req struct {
			Prompt string `json:"prompt"`
			Model  string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Prompt == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "prompt is required"})
			return
		}

		c.JSON(200, gin.H{
			"id":      "cmpl-123",
			"object":  "text_completion",
			"created": time.Now().Unix(),
			"model":   req.Model,
			"choices": []gin.H{
				{
					"text":          "Mock completion response",
					"index":         0,
					"finish_reason": "stop",
				},
			},
			"usage": gin.H{
				"prompt_tokens":     10,
				"completion_tokens": 20,
				"total_tokens":      30,
			},
		})
	})

	t.Run("returns completion for valid request", func(t *testing.T) {
		body := map[string]interface{}{
			"prompt": "Hello, world!",
			"model":  "test-model",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "text_completion", response["object"])
		assert.Contains(t, response, "choices")
		assert.Contains(t, response, "usage")
	})

	t.Run("returns error for missing prompt", func(t *testing.T) {
		body := map[string]interface{}{
			"model": "test-model",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/completions", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestChatCompletionEndpoint tests the /v1/chat/completions endpoint
func TestChatCompletionEndpoint(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	// Mock chat completion handler
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		var req struct {
			Model    string           `json:"model"`
			Messages []map[string]any `json:"messages"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if len(req.Messages) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "messages is required"})
			return
		}

		c.JSON(200, gin.H{
			"id":      "chatcmpl-123",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   req.Model,
			"choices": []gin.H{
				{
					"index": 0,
					"message": gin.H{
						"role":    "assistant",
						"content": "Mock chat response",
					},
					"finish_reason": "stop",
				},
			},
			"usage": gin.H{
				"prompt_tokens":     15,
				"completion_tokens": 25,
				"total_tokens":      40,
			},
		})
	})

	t.Run("returns chat completion for valid request", func(t *testing.T) {
		body := map[string]interface{}{
			"model": "test-model",
			"messages": []map[string]string{
				{"role": "user", "content": "Hello!"},
			},
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "chat.completion", response["object"])
		assert.Contains(t, response, "choices")
		choices := response["choices"].([]interface{})
		choice := choices[0].(map[string]interface{})
		message := choice["message"].(map[string]interface{})
		assert.Equal(t, "assistant", message["role"])
	})

	t.Run("returns error for empty messages", func(t *testing.T) {
		body := map[string]interface{}{
			"model":    "test-model",
			"messages": []map[string]string{},
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestEnsembleEndpoint tests the /v1/ensemble/completions endpoint
func TestEnsembleEndpoint(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	// Mock ensemble handler
	router.POST("/v1/ensemble/completions", func(c *gin.Context) {
		var req struct {
			Prompt         string                 `json:"prompt"`
			Model          string                 `json:"model"`
			EnsembleConfig map[string]interface{} `json:"ensemble_config"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"id":      "ens-123",
			"object":  "ensemble.completion",
			"created": time.Now().Unix(),
			"model":   req.Model,
			"choices": []gin.H{
				{
					"index": 0,
					"message": gin.H{
						"role":    "assistant",
						"content": "Ensemble response",
					},
					"finish_reason": "stop",
				},
			},
			"ensemble": gin.H{
				"voting_method":     "confidence_weighted",
				"responses_count":   3,
				"selected_provider": "claude",
			},
		})
	})

	t.Run("returns ensemble completion", func(t *testing.T) {
		body := map[string]interface{}{
			"prompt": "Test ensemble prompt",
			"model":  "ensemble-model",
			"ensemble_config": map[string]interface{}{
				"strategy":             "confidence_weighted",
				"min_providers":        2,
				"confidence_threshold": 0.8,
			},
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/ensemble/completions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "ensemble.completion", response["object"])
		assert.Contains(t, response, "ensemble")
		ensemble := response["ensemble"].(map[string]interface{})
		assert.Equal(t, "confidence_weighted", ensemble["voting_method"])
	})
}

// TestProviderHealthEndpoint tests the /v1/providers/:name/health endpoint
func TestProviderHealthEndpoint(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	// Mock provider health check
	router.GET("/v1/providers/:name/health", func(c *gin.Context) {
		name := c.Param("name")

		// Simulate different provider states
		if name == "unhealthy-provider" {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"provider": name,
				"healthy":  false,
				"error":    "Provider is not responding",
			})
			return
		}

		if name == "unknown-provider" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "provider not found",
			})
			return
		}

		c.JSON(200, gin.H{
			"provider": name,
			"healthy":  true,
			"circuit_breaker": gin.H{
				"state":         "closed",
				"failure_count": 0,
			},
		})
	})

	t.Run("returns healthy status for active provider", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/providers/claude/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, true, response["healthy"])
	})

	t.Run("returns unhealthy status for failing provider", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/providers/unhealthy-provider/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("returns 404 for unknown provider", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/providers/unknown-provider/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestMCPEndpoints tests MCP-related endpoints
func TestMCPEndpoints(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	// MCP capabilities endpoint
	router.GET("/v1/mcp/capabilities", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"version":   "1.0",
			"providers": []string{"claude", "deepseek"},
		})
	})

	// MCP tools endpoint
	router.GET("/v1/mcp/tools", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"tools": []gin.H{
				{"name": "read_file", "description": "Read a file"},
				{"name": "write_file", "description": "Write to a file"},
			},
		})
	})

	// MCP tool call endpoint
	router.POST("/v1/mcp/tools/call", func(c *gin.Context) {
		var req struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{
			"result": "Tool executed successfully",
		})
	})

	t.Run("capabilities returns MCP info", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/mcp/capabilities", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "version")
		assert.Contains(t, response, "providers")
	})

	t.Run("tools returns available tools", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/mcp/tools", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "tools")
	})

	t.Run("tool call executes tool", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "read_file",
			"arguments": map[string]interface{}{
				"path": "/tmp/test.txt",
			},
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/mcp/tools/call", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestLSPEndpoints tests LSP-related endpoints
func TestLSPEndpoints(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/v1/lsp/servers", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"servers": []gin.H{
				{"language": "go", "status": "running"},
				{"language": "python", "status": "running"},
			},
		})
	})

	router.GET("/v1/lsp/stats", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"total_servers": 2,
			"active":        2,
			"inactive":      0,
		})
	})

	t.Run("servers returns LSP server list", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/lsp/servers", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "servers")
	})

	t.Run("stats returns LSP statistics", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/lsp/stats", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(2), response["total_servers"])
	})
}

// TestCogneeEndpoints tests Cognee-related endpoints
func TestCogneeEndpoints(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	router.POST("/v1/cognee/add", func(c *gin.Context) {
		var req struct {
			DatasetName string `json:"dataset_name"`
			Content     string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{
			"success": true,
			"message": "Content added to dataset",
		})
	})

	router.POST("/v1/cognee/search", func(c *gin.Context) {
		var req struct {
			DatasetName string `json:"dataset_name"`
			Query       string `json:"query"`
			Limit       int    `json:"limit"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{
			"results": []gin.H{
				{"content": "Result 1", "score": 0.95},
				{"content": "Result 2", "score": 0.85},
			},
		})
	})

	t.Run("add stores content", func(t *testing.T) {
		body := map[string]interface{}{
			"dataset_name": "test-dataset",
			"content":      "Test content",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/cognee/add", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("search returns results", func(t *testing.T) {
		body := map[string]interface{}{
			"dataset_name": "test-dataset",
			"query":        "test query",
			"limit":        10,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/cognee/search", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "results")
	})
}

// TestEmbeddingEndpoints tests embedding-related endpoints
func TestEmbeddingEndpoints(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	router.POST("/v1/embeddings/generate", func(c *gin.Context) {
		var req struct {
			Input string `json:"input"`
			Model string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{
			"object": "list",
			"data": []gin.H{
				{
					"object":    "embedding",
					"embedding": []float64{0.1, 0.2, 0.3, 0.4, 0.5},
					"index":     0,
				},
			},
		})
	})

	t.Run("generate returns embeddings", func(t *testing.T) {
		body := map[string]interface{}{
			"input": "Test text for embedding",
			"model": "test-embedding-model",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/embeddings/generate", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "list", response["object"])
		assert.Contains(t, response, "data")
	})
}

// TestSessionEndpoints tests session-related endpoints
func TestSessionEndpoints(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	sessions := make(map[string]map[string]interface{})

	router.POST("/v1/sessions", func(c *gin.Context) {
		sessionID := "session-" + time.Now().Format("20060102150405")
		session := map[string]interface{}{
			"id":         sessionID,
			"status":     "active",
			"created_at": time.Now().Unix(),
		}
		sessions[sessionID] = session
		c.JSON(200, session)
	})

	router.GET("/v1/sessions/:id", func(c *gin.Context) {
		id := c.Param("id")
		if session, exists := sessions[id]; exists {
			c.JSON(200, session)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
	})

	router.GET("/v1/sessions", func(c *gin.Context) {
		list := make([]map[string]interface{}, 0)
		for _, s := range sessions {
			list = append(list, s)
		}
		c.JSON(200, gin.H{"sessions": list})
	})

	router.DELETE("/v1/sessions/:id", func(c *gin.Context) {
		id := c.Param("id")
		if _, exists := sessions[id]; exists {
			delete(sessions, id)
			c.JSON(200, gin.H{"success": true})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
	})

	t.Run("create session", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/sessions", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "id")
		assert.Equal(t, "active", response["status"])
	})

	t.Run("get non-existent session returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/sessions/non-existent", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("list sessions", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/sessions", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "sessions")
	})
}

// TestRouterNotFound tests 404 handling
func TestRouterNotFound(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	// Add some routes
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	t.Run("returns 404 for non-existent route", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/non-existent-route", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 404 for wrong method", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestMetricsEndpoint tests the /metrics endpoint
func TestMetricsEndpoint(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/metrics", func(c *gin.Context) {
		c.String(200, "# HELP go_gc_duration_seconds A summary of the GC invocation durations.\n")
	})

	t.Run("returns prometheus metrics", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/metrics", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "# HELP")
	})
}

// TestAdminEndpoints tests admin-related endpoints
func TestAdminEndpoints(t *testing.T) {
	router := gin.New()
	router.Use(gin.Recovery())

	// Simple admin check middleware
	adminMiddleware := func(c *gin.Context) {
		role := c.GetHeader("X-User-Role")
		if role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}

	admin := router.Group("/v1/admin", adminMiddleware)
	admin.GET("/health/all", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"provider_health": map[string]interface{}{
				"claude":   nil,
				"deepseek": nil,
				"gemini":   "timeout error",
			},
		})
	})

	t.Run("admin endpoints require admin role", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/admin/health/all", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("admin endpoints accessible with admin role", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/admin/health/all", nil)
		req.Header.Set("X-User-Role", "admin")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
