package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/services"
)

func newTestCogneeLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return logger
}

func setupCogneeTestServer() (*httptest.Server, *services.CogneeService) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Root endpoint for fast health check (IsHealthy uses this now)
		case r.URL.Path == "/" || r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"message": "Hello, World, I am alive!"})

		// Auth endpoints for automatic authentication
		case r.URL.Path == "/api/v1/auth/register" && r.Method == "POST":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "test-user-id", "email": "test@test.com"})

		case r.URL.Path == "/api/v1/auth/login" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "test-token", "token_type": "bearer"})

		case r.URL.Path == "/api/v1/add" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "mem-123", "success": true})

		case r.URL.Path == "/api/v1/search" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{"content": "test result", "score": 0.95},
				},
			})

		case r.URL.Path == "/api/v1/cognify" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})

		case r.URL.Path == "/api/v1/insights" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"insights": []interface{}{
					map[string]interface{}{"type": "entity", "value": "test"},
				},
			})

		case r.URL.Path == "/api/v1/graph/completion" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"completions": []interface{}{"completion1", "completion2"},
			})

		case r.URL.Path == "/api/v1/code-pipeline/index" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"analysis": map[string]interface{}{
					"functions": []interface{}{"main", "helper"},
				},
			})

		case r.URL.Path == "/api/v1/datasets" && r.Method == "POST":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "ds-123", "name": "test"})

		case r.URL.Path == "/api/v1/datasets" && r.Method == "GET":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"datasets": []interface{}{
					map[string]interface{}{"id": "ds-1", "name": "default"},
				},
			})

		case r.URL.Path == "/api/v1/feedback" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})

		case r.URL.Path == "/api/v1/visualize" && r.Method == "GET":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"nodes": []interface{}{},
				"edges": []interface{}{},
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	logger := newTestCogneeLogger()
	config := &services.CogneeServiceConfig{
		Enabled:                true,
		BaseURL:                server.URL,
		AuthEmail:              "test@test.com",
		AuthPassword:           "testpassword",
		AutoCognify:            true,
		EnhancePrompts:         true,
		StoreResponses:         true,
		EnableGraphReasoning:   true,
		EnableFeedbackLoop:     true,
		EnableCodeIntelligence: true,
		Timeout:                10 * time.Second,
	}

	cogneeService := services.NewCogneeServiceWithConfig(config, logger)
	cogneeService.SetReady(true)

	return server, cogneeService
}

func TestNewCogneeAPIHandler(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	require.NotNil(t, handler)
	assert.NotNil(t, handler.cogneeService)
	assert.NotNil(t, handler.logger)
}

func TestCogneeAPIHandler_Health(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/cognee/health", nil)

	handler.Health(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "healthy")
}

func TestCogneeAPIHandler_AddMemory(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	t.Run("successful add", func(t *testing.T) {
		body := map[string]interface{}{
			"content": "Test memory content",
			"dataset": "default",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/memory", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.AddMemory(c)

		// AddMemory returns 201 Created
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("missing content", func(t *testing.T) {
		body := map[string]interface{}{
			"dataset": "default",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/memory", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.AddMemory(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestCogneeAPIHandler_SearchMemory(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	t.Run("successful search", func(t *testing.T) {
		body := map[string]interface{}{
			"query":   "test query",
			"dataset": "default",
			"limit":   10,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/search", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.SearchMemory(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing query", func(t *testing.T) {
		body := map[string]interface{}{
			"dataset": "default",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/search", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.SearchMemory(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestCogneeAPIHandler_Cognify(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	body := map[string]interface{}{
		"content": "Content to cognify",
		"dataset": "default",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/cognify", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Cognify(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCogneeAPIHandler_GetInsights(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	body := map[string]interface{}{
		"query":   "insights query",
		"dataset": "default",
		"depth":   3,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/insights", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.GetInsights(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCogneeAPIHandler_ProcessCode(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	t.Run("successful code processing", func(t *testing.T) {
		body := map[string]interface{}{
			"code":     "func main() { println(\"hello\") }",
			"language": "go",
			"dataset":  "code",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/code", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.ProcessCode(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing code", func(t *testing.T) {
		body := map[string]interface{}{
			"language": "go",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/code", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.ProcessCode(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestCogneeAPIHandler_ProvideFeedback(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	body := map[string]interface{}{
		"query_id":  "query-123",
		"query":     "What is AI?",
		"response":  "AI is artificial intelligence",
		"relevance": 0.95,
		"approved":  true,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/feedback", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ProvideFeedback(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCogneeAPIHandler_Stats(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/cognee/stats", nil)

	handler.Stats(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	// Stats returns flat structure with various metrics
	assert.Contains(t, response, "total_memories_stored")
	assert.Contains(t, response, "total_searches")
}

func TestCogneeAPIHandler_RegisterRoutes(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")

	handler.RegisterRoutes(api)

	// Verify routes are registered
	routes := router.Routes()
	expectedPaths := []string{
		"/api/v1/cognee/health",
		"/api/v1/cognee/memory",
		"/api/v1/cognee/search",
		"/api/v1/cognee/cognify",
		"/api/v1/cognee/stats",
	}

	registeredPaths := make(map[string]bool)
	for _, route := range routes {
		registeredPaths[route.Path] = true
	}

	for _, path := range expectedPaths {
		assert.True(t, registeredPaths[path], "Expected route %s to be registered", path)
	}
}

func TestCogneeAPIHandler_CreateDataset(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	body := map[string]interface{}{
		"name":        "test-dataset",
		"description": "Test dataset for unit testing",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/datasets", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateDataset(c)

	// CreateDataset returns 201 Created
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCogneeAPIHandler_ListDatasets(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/cognee/datasets", nil)

	handler.ListDatasets(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCogneeAPIHandler_VisualizeGraph(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/cognee/graph?dataset=default", nil)

	handler.VisualizeGraph(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Benchmark tests
func BenchmarkCogneeAPIHandler_Health(b *testing.B) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewCogneeAPIHandler(cogneeService, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/cognee/health", nil)
		handler.Health(c)
	}
}

func BenchmarkCogneeAPIHandler_SearchMemory(b *testing.B) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewCogneeAPIHandler(cogneeService, logger)

	body := map[string]interface{}{
		"query":   "test",
		"dataset": "default",
	}
	jsonBody, _ := json.Marshal(body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/search", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")
		handler.SearchMemory(c)
	}
}

// Test helper functions
func TestGetIntParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("returns value when param exists", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test?limit=42", nil)

		result := getIntParam(c, "limit", 10)
		assert.Equal(t, 42, result)
	})

	t.Run("returns default when param missing", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)

		result := getIntParam(c, "limit", 10)
		assert.Equal(t, 10, result)
	})

	t.Run("returns default when param invalid", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test?limit=not-a-number", nil)

		result := getIntParam(c, "limit", 10)
		assert.Equal(t, 10, result)
	})

	t.Run("handles zero value", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test?offset=0", nil)

		result := getIntParam(c, "offset", 5)
		assert.Equal(t, 0, result)
	})

	t.Run("handles negative value", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test?page=-1", nil)

		result := getIntParam(c, "page", 1)
		assert.Equal(t, -1, result)
	})
}

func TestCogneeAPIHandler_GetConfig(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/cognee/config", nil)

	handler.GetConfig(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "enabled")
	assert.Contains(t, response, "base_url")
	assert.Contains(t, response, "auto_cognify")
	assert.Contains(t, response, "enhance_prompts")
	assert.Contains(t, response, "max_context_size")
}

func TestCogneeAPIHandler_DeleteDataset(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	t.Run("delete existing dataset", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("DELETE", "/cognee/datasets/test-dataset", nil)
		c.Params = gin.Params{{Key: "name", Value: "test-dataset"}}

		handler.DeleteDataset(c)

		// Service will attempt to delete - verify it processes the request
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
	})

	t.Run("delete with empty name", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("DELETE", "/cognee/datasets/", nil)
		c.Params = gin.Params{{Key: "name", Value: ""}}

		handler.DeleteDataset(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestCogneeAPIHandler_GetGraphCompletion(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	t.Run("successful graph completion", func(t *testing.T) {
		body := map[string]interface{}{
			"query":   "test graph query",
			"dataset": "default",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/graph/complete", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GetGraphCompletion(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing query", func(t *testing.T) {
		body := map[string]interface{}{
			"dataset": "default",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/graph/complete", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GetGraphCompletion(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestCogneeAPIHandler_EnsureRunning(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/start", nil)

	handler.EnsureRunning(c)

	// EnsureRunning may succeed or fail depending on Docker availability
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
}

// TestCogneeAPIHandler_Cognify_ErrorPaths tests error handling in Cognify
func TestCogneeAPIHandler_Cognify_ErrorPaths(t *testing.T) {
	logger := newTestCogneeLogger()
	logger.SetLevel(logrus.PanicLevel)

	t.Run("invalid JSON body", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/cognify", bytes.NewBufferString("not json"))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Cognify(c)

		// Cognify handles invalid JSON gracefully and uses default empty datasets
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("service returns error", func(t *testing.T) {
		// Create server that returns error for cognify
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/cognify" {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "cognify failed"})
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &services.CogneeServiceConfig{
			Enabled: true,
			BaseURL: server.URL,
		}
		cogneeService := services.NewCogneeServiceWithConfig(config, logger)
		cogneeService.SetReady(true)

		handler := NewCogneeAPIHandler(cogneeService, logger)

		body := map[string]interface{}{"datasets": []string{"test"}}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/cognify", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Cognify(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// TestCogneeAPIHandler_GetInsights_ErrorPaths tests error handling in GetInsights
func TestCogneeAPIHandler_GetInsights_ErrorPaths(t *testing.T) {
	logger := newTestCogneeLogger()
	logger.SetLevel(logrus.PanicLevel)

	t.Run("invalid JSON body", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/insights", bytes.NewBufferString("{broken"))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GetInsights(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing required query field", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		body := map[string]interface{}{"datasets": []string{"test"}}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/insights", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GetInsights(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("service returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/insights" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &services.CogneeServiceConfig{
			Enabled:              true,
			BaseURL:              server.URL,
			EnableGraphReasoning: true,
		}
		cogneeService := services.NewCogneeServiceWithConfig(config, logger)
		cogneeService.SetReady(true)

		handler := NewCogneeAPIHandler(cogneeService, logger)

		body := map[string]interface{}{"query": "test query", "datasets": []string{"test"}}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/insights", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GetInsights(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// TestCogneeAPIHandler_CreateDataset_ErrorPaths tests error handling in CreateDataset
func TestCogneeAPIHandler_CreateDataset_ErrorPaths(t *testing.T) {
	logger := newTestCogneeLogger()
	logger.SetLevel(logrus.PanicLevel)

	t.Run("invalid JSON body", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/datasets", bytes.NewBufferString("not json"))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.CreateDataset(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing required name field", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		body := map[string]interface{}{"description": "test dataset"}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/datasets", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.CreateDataset(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("service returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/datasets" && r.Method == "POST" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &services.CogneeServiceConfig{
			Enabled: true,
			BaseURL: server.URL,
		}
		cogneeService := services.NewCogneeServiceWithConfig(config, logger)
		cogneeService.SetReady(true)

		handler := NewCogneeAPIHandler(cogneeService, logger)

		body := map[string]interface{}{"name": "test-dataset"}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/datasets", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.CreateDataset(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// TestCogneeAPIHandler_ListDatasets_ErrorPaths tests error handling in ListDatasets
func TestCogneeAPIHandler_ListDatasets_ErrorPaths(t *testing.T) {
	logger := newTestCogneeLogger()
	logger.SetLevel(logrus.PanicLevel)

	t.Run("service returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/datasets" && r.Method == "GET" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &services.CogneeServiceConfig{
			Enabled: true,
			BaseURL: server.URL,
		}
		cogneeService := services.NewCogneeServiceWithConfig(config, logger)
		cogneeService.SetReady(true)

		handler := NewCogneeAPIHandler(cogneeService, logger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/cognee/datasets", nil)

		handler.ListDatasets(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// TestCogneeAPIHandler_ProvideFeedback_ErrorPaths tests error handling in ProvideFeedback
func TestCogneeAPIHandler_ProvideFeedback_ErrorPaths(t *testing.T) {
	logger := newTestCogneeLogger()
	logger.SetLevel(logrus.PanicLevel)

	t.Run("invalid JSON body", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/feedback", bytes.NewBufferString("invalid"))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.ProvideFeedback(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing required fields", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		body := map[string]interface{}{"query_id": "123"}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/feedback", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.ProvideFeedback(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("successful with all required fields", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		body := map[string]interface{}{
			"query_id":  "123",
			"query":     "test query",
			"response":  "test response",
			"relevance": 0.9,
			"approved":  true,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/feedback", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.ProvideFeedback(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestCogneeAPIHandler_Health_Unhealthy tests Health when service is unhealthy
func TestCogneeAPIHandler_Health_Unhealthy(t *testing.T) {
	logger := newTestCogneeLogger()
	logger.SetLevel(logrus.PanicLevel)

	// Create server that doesn't respond to health checks
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := &services.CogneeServiceConfig{
		Enabled: true,
		BaseURL: server.URL,
	}
	cogneeService := services.NewCogneeServiceWithConfig(config, logger)
	// Don't set ready - service will be unhealthy

	handler := NewCogneeAPIHandler(cogneeService, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/cognee/health", nil)

	handler.Health(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", response["status"])
	assert.Equal(t, false, response["healthy"])
}

// TestCogneeAPIHandler_AddMemory_ErrorPaths tests error handling in AddMemory
func TestCogneeAPIHandler_AddMemory_ErrorPaths(t *testing.T) {
	logger := newTestCogneeLogger()
	logger.SetLevel(logrus.PanicLevel)

	t.Run("invalid JSON body", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/memory", bytes.NewBufferString("{bad"))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.AddMemory(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("service returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/add" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &services.CogneeServiceConfig{
			Enabled: true,
			BaseURL: server.URL,
		}
		cogneeService := services.NewCogneeServiceWithConfig(config, logger)
		cogneeService.SetReady(true)

		handler := NewCogneeAPIHandler(cogneeService, logger)

		body := map[string]interface{}{"content": "test content"}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/memory", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.AddMemory(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// TestCogneeAPIHandler_SearchMemory_ErrorPaths tests error handling in SearchMemory
func TestCogneeAPIHandler_SearchMemory_ErrorPaths(t *testing.T) {
	logger := newTestCogneeLogger()
	logger.SetLevel(logrus.PanicLevel)

	t.Run("invalid JSON body", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/search", bytes.NewBufferString("invalid"))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.SearchMemory(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("successful search with dataset", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		body := map[string]interface{}{
			"query":   "test query",
			"dataset": "test-dataset",
			"limit":   5,
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/search", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.SearchMemory(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestCogneeAPIHandler_ProcessCode_ErrorPaths tests error handling in ProcessCode
func TestCogneeAPIHandler_ProcessCode_ErrorPaths(t *testing.T) {
	logger := newTestCogneeLogger()
	logger.SetLevel(logrus.PanicLevel)

	t.Run("invalid JSON body", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/code", bytes.NewBufferString("{bad"))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.ProcessCode(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("service returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/code-pipeline/index" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &services.CogneeServiceConfig{
			Enabled:                true,
			BaseURL:                server.URL,
			EnableCodeIntelligence: true,
		}
		cogneeService := services.NewCogneeServiceWithConfig(config, logger)
		cogneeService.SetReady(true)

		handler := NewCogneeAPIHandler(cogneeService, logger)

		body := map[string]interface{}{"code": "func main() {}", "language": "go"}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/code", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.ProcessCode(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// TestCogneeAPIHandler_VisualizeGraph_ErrorPath tests error handling in VisualizeGraph
func TestCogneeAPIHandler_VisualizeGraph_ErrorPath(t *testing.T) {
	logger := newTestCogneeLogger()
	logger.SetLevel(logrus.PanicLevel)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/visualize" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &services.CogneeServiceConfig{
		Enabled: true,
		BaseURL: server.URL,
	}
	cogneeService := services.NewCogneeServiceWithConfig(config, logger)
	cogneeService.SetReady(true)

	handler := NewCogneeAPIHandler(cogneeService, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/cognee/graph/visualize?dataset=test", nil)

	handler.VisualizeGraph(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestCogneeAPIHandler_NilLogger tests handler creation with nil logger
func TestCogneeAPIHandler_NilLogger(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	handler := NewCogneeAPIHandler(cogneeService, nil)

	require.NotNil(t, handler)
	assert.NotNil(t, handler.logger)
}

// TestCogneeAPIHandler_EnsureRunning_Success tests successful EnsureRunning
func TestCogneeAPIHandler_EnsureRunning_Success(t *testing.T) {
	logger := newTestCogneeLogger()
	logger.SetLevel(logrus.PanicLevel)

	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	handler := NewCogneeAPIHandler(cogneeService, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/start", nil)

	handler.EnsureRunning(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["success"])
	assert.Contains(t, response["message"].(string), "started successfully")
}

// TestCogneeAPIHandler_DeleteDataset_EmptyName tests DeleteDataset with empty name
func TestCogneeAPIHandler_DeleteDataset_EmptyName(t *testing.T) {
	logger := newTestCogneeLogger()
	logger.SetLevel(logrus.PanicLevel)

	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	handler := NewCogneeAPIHandler(cogneeService, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/cognee/datasets/", nil)
	c.Params = gin.Params{{Key: "name", Value: ""}}

	handler.DeleteDataset(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "dataset name is required")
}

// TestCogneeAPIHandler_GetGraphCompletion_ErrorPaths tests error handling
func TestCogneeAPIHandler_GetGraphCompletion_ErrorPaths(t *testing.T) {
	logger := newTestCogneeLogger()
	logger.SetLevel(logrus.PanicLevel)

	t.Run("invalid JSON body", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/graph/completion", bytes.NewBufferString("{bad json"))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GetGraphCompletion(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing required query", func(t *testing.T) {
		server, cogneeService := setupCogneeTestServer()
		defer server.Close()

		handler := NewCogneeAPIHandler(cogneeService, logger)

		body := map[string]interface{}{"datasets": []string{"test"}}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/graph/completion", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.GetGraphCompletion(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestGetFloatParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("returns value when param exists", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test?score=0.95", nil)

		result := getFloatParam(c, "score", 0.5)
		assert.Equal(t, 0.95, result)
	})

	t.Run("returns default when param missing", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)

		result := getFloatParam(c, "score", 0.5)
		assert.Equal(t, 0.5, result)
	})

	t.Run("returns default when param invalid", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test?score=not-a-float", nil)

		result := getFloatParam(c, "score", 0.5)
		assert.Equal(t, 0.5, result)
	})

	t.Run("handles zero value", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test?threshold=0.0", nil)

		result := getFloatParam(c, "threshold", 0.7)
		assert.Equal(t, 0.0, result)
	})

	t.Run("handles large float value", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test?value=1234567.89", nil)

		result := getFloatParam(c, "value", 0.0)
		assert.Equal(t, 1234567.89, result)
	})

	t.Run("handles scientific notation", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test?value=1.5e-3", nil)

		result := getFloatParam(c, "value", 0.0)
		assert.Equal(t, 0.0015, result)
	})
}

// =====================================================
// CONCURRENT REQUEST HANDLING TESTS
// =====================================================

// TestCogneeAPIHandler_ConcurrentHealthRequests tests concurrent Health requests
func TestCogneeAPIHandler_ConcurrentHealthRequests(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	numRequests := 30
	var wg sync.WaitGroup
	successCount := int32(0)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/cognee/health", nil)

			handler.Health(c)

			if w.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int32(numRequests), successCount, "All Health requests should return 200")
}

// TestCogneeAPIHandler_ConcurrentSearchRequests tests concurrent SearchMemory requests
func TestCogneeAPIHandler_ConcurrentSearchRequests(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	numRequests := 20
	var wg sync.WaitGroup
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			body := map[string]interface{}{
				"query":   "concurrent search " + strconv.Itoa(idx),
				"dataset": "default",
				"limit":   5,
			}
			jsonBody, _ := json.Marshal(body)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/cognee/search", bytes.NewReader(jsonBody))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.SearchMemory(c)
			results <- w.Code
		}(i)
	}

	wg.Wait()
	close(results)

	count := 0
	successCount := 0
	for code := range results {
		count++
		if code == http.StatusOK {
			successCount++
		}
	}

	assert.Equal(t, numRequests, count, "All requests should complete")
	assert.Equal(t, numRequests, successCount, "All concurrent search requests should succeed")
}

// TestCogneeAPIHandler_ConcurrentAddMemoryRequests tests concurrent AddMemory requests
func TestCogneeAPIHandler_ConcurrentAddMemoryRequests(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	numRequests := 15
	var wg sync.WaitGroup
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			body := map[string]interface{}{
				"content": "Concurrent memory content " + strconv.Itoa(idx),
				"dataset": "test-concurrent",
			}
			jsonBody, _ := json.Marshal(body)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/cognee/memory", bytes.NewReader(jsonBody))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.AddMemory(c)
			results <- w.Code
		}(i)
	}

	wg.Wait()
	close(results)

	count := 0
	successCount := 0
	for code := range results {
		count++
		if code == http.StatusCreated {
			successCount++
		}
	}

	assert.Equal(t, numRequests, count)
	assert.Equal(t, numRequests, successCount, "All concurrent add memory requests should succeed")
}

// TestCogneeAPIHandler_ConcurrentStatsRequests tests concurrent Stats requests
func TestCogneeAPIHandler_ConcurrentStatsRequests(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	numRequests := 25
	var wg sync.WaitGroup
	successCount := int32(0)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/cognee/stats", nil)

			handler.Stats(c)

			if w.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int32(numRequests), successCount)
}

// =====================================================
// EDGE CASE TESTS
// =====================================================

// TestCogneeAPIHandler_RequestWithSpecialContent tests handling of special content
func TestCogneeAPIHandler_RequestWithSpecialContent(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	testCases := []struct {
		name    string
		content string
	}{
		{"unicode", "Test content with unicode: \u4f60\u597d\u4e16\u754c"},
		{"emoji", "Test content with emoji: testing"},
		{"newlines", "Test content\nwith\nnewlines"},
		{"tabs", "Test content\twith\ttabs"},
		{"quotes", `Test content with "quotes" and 'apostrophes'`},
		{"code", "func main() { fmt.Println(\"Hello, World!\") }"},
		{"sql", "SELECT * FROM users WHERE id = 1; DROP TABLE users;"},
		{"html", "<script>alert('XSS')</script>"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]interface{}{
				"content": tc.content,
				"dataset": "test",
			}
			jsonBody, _ := json.Marshal(body)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/cognee/memory", bytes.NewReader(jsonBody))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.AddMemory(c)

			assert.Equal(t, http.StatusCreated, w.Code, "Should handle special content: %s", tc.name)
		})
	}
}

// TestCogneeAPIHandler_LargeContent tests handling of large content
func TestCogneeAPIHandler_LargeContent(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	// Create large content
	largeContent := ""
	for i := 0; i < 1000; i++ {
		largeContent += "This is a test sentence to create large content. "
	}

	body := map[string]interface{}{
		"content": largeContent,
		"dataset": "test",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/memory", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.AddMemory(c)

	assert.Equal(t, http.StatusCreated, w.Code, "Should handle large content")
}

// TestCogneeAPIHandler_EmptyContent tests handling of empty content
func TestCogneeAPIHandler_EmptyContent(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	body := map[string]interface{}{
		"content": "",
		"dataset": "test",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/memory", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.AddMemory(c)

	// Empty content should be rejected
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCogneeAPIHandler_SearchWithVariousLimits tests search with various limit values
func TestCogneeAPIHandler_SearchWithVariousLimits(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	limits := []int{0, 1, 5, 10, 50, 100, 1000}

	for _, limit := range limits {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			body := map[string]interface{}{
				"query":   "test query",
				"dataset": "default",
				"limit":   limit,
			}
			jsonBody, _ := json.Marshal(body)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/cognee/search", bytes.NewReader(jsonBody))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.SearchMemory(c)

			assert.Equal(t, http.StatusOK, w.Code, "Should handle limit: %d", limit)
		})
	}
}

// TestCogneeAPIHandler_ProcessCodeAllLanguages tests code processing with various languages
func TestCogneeAPIHandler_ProcessCodeAllLanguages(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	languages := []struct {
		name string
		code string
	}{
		{"go", "package main\n\nfunc main() {}"},
		{"python", "def main():\n    print('hello')"},
		{"javascript", "function main() { console.log('hello'); }"},
		{"typescript", "const main = (): void => { console.log('hello'); };"},
		{"java", "public class Main { public static void main(String[] args) {} }"},
		{"rust", "fn main() { println!(\"hello\"); }"},
		{"c", "int main() { return 0; }"},
		{"cpp", "#include <iostream>\nint main() { std::cout << \"hello\"; return 0; }"},
	}

	for _, lang := range languages {
		t.Run(lang.name, func(t *testing.T) {
			body := map[string]interface{}{
				"code":     lang.code,
				"language": lang.name,
				"dataset":  "code-test",
			}
			jsonBody, _ := json.Marshal(body)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/cognee/code", bytes.NewReader(jsonBody))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.ProcessCode(c)

			assert.Equal(t, http.StatusOK, w.Code, "Should handle language: %s", lang.name)
		})
	}
}

// TestCogneeAPIHandler_InsightsWithAllOptions tests insights with all options
func TestCogneeAPIHandler_InsightsWithAllOptions(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	body := map[string]interface{}{
		"query":    "comprehensive insights query",
		"datasets": []string{"default", "custom", "code"},
		"limit":    20,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/insights", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.GetInsights(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestCogneeAPIHandler_FeedbackVariousScores tests feedback with various relevance scores
func TestCogneeAPIHandler_FeedbackVariousScores(t *testing.T) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := newTestCogneeLogger()
	handler := NewCogneeAPIHandler(cogneeService, logger)

	scores := []float64{0.0, 0.25, 0.5, 0.75, 1.0}

	for _, score := range scores {
		t.Run(strconv.FormatFloat(score, 'f', 2, 64), func(t *testing.T) {
			body := map[string]interface{}{
				"query_id":  "test-query-" + strconv.FormatFloat(score, 'f', 2, 64),
				"query":     "test query",
				"response":  "test response",
				"relevance": score,
				"approved":  score >= 0.5,
			}
			jsonBody, _ := json.Marshal(body)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/cognee/feedback", bytes.NewReader(jsonBody))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.ProvideFeedback(c)

			assert.Equal(t, http.StatusOK, w.Code, "Should handle relevance: %f", score)
		})
	}
}

// =====================================================
// ADDITIONAL BENCHMARK TESTS
// =====================================================

// BenchmarkCogneeAPIHandler_AddMemory benchmarks AddMemory
func BenchmarkCogneeAPIHandler_AddMemory(b *testing.B) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewCogneeAPIHandler(cogneeService, logger)

	body := map[string]interface{}{
		"content": "Benchmark memory content for testing performance",
		"dataset": "benchmark",
	}
	jsonBody, _ := json.Marshal(body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/memory", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")
		handler.AddMemory(c)
	}
}

// BenchmarkCogneeAPIHandler_Stats benchmarks Stats
func BenchmarkCogneeAPIHandler_Stats(b *testing.B) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewCogneeAPIHandler(cogneeService, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/cognee/stats", nil)
		handler.Stats(c)
	}
}

// BenchmarkCogneeAPIHandler_GetConfig benchmarks GetConfig
func BenchmarkCogneeAPIHandler_GetConfig(b *testing.B) {
	server, cogneeService := setupCogneeTestServer()
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	handler := NewCogneeAPIHandler(cogneeService, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/cognee/config", nil)
		handler.GetConfig(c)
	}
}
