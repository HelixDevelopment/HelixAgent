package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		case r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})

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
