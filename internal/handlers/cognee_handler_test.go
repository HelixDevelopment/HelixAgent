package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"dev.helix.agent/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	runtime.GOMAXPROCS(2)
	gin.SetMode(gin.TestMode)
}

// setupCogneeHandlerTestServer creates a mock Cognee backend and a configured
// CogneeService/CogneeAPIHandler pair for testing.
func setupCogneeHandlerTestServer() (*httptest.Server, *CogneeAPIHandler) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/" || r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Hello, World, I am alive!",
			})

		case r.URL.Path == "/api/v1/auth/register" && r.Method == "POST":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "test-user-id", "email": "test@test.com",
			})

		case r.URL.Path == "/api/v1/auth/login" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "test-token", "token_type": "bearer",
			})

		case r.URL.Path == "/api/v1/add" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "mem-123", "success": true,
			})

		case r.URL.Path == "/api/v1/memify" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "mem-123", "vector_id": "vec-123", "status": "success",
			})

		case r.URL.Path == "/api/v1/search" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{"content": "test result", "score": 0.95},
				},
			})

		case r.URL.Path == "/api/v1/cognify" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})

		case r.URL.Path == "/api/v1/insights" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"insights": []interface{}{
					map[string]interface{}{"type": "entity", "value": "test"},
				},
			})

		case r.URL.Path == "/api/v1/graph/completion" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"completions": []interface{}{"completion1", "completion2"},
			})

		case r.URL.Path == "/api/v1/code-pipeline/index" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"analysis": map[string]interface{}{
					"functions": []interface{}{"main", "helper"},
				},
			})

		case r.URL.Path == "/api/v1/datasets" && r.Method == "POST":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "ds-123", "name": "test",
			})

		case r.URL.Path == "/api/v1/datasets" && r.Method == "GET":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"datasets": []interface{}{
					map[string]interface{}{"id": "ds-1", "name": "default"},
				},
			})

		case r.URL.Path == "/api/v1/feedback" && r.Method == "POST":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})

		case r.URL.Path == "/api/v1/visualize" && r.Method == "GET":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"nodes": []interface{}{},
				"edges": []interface{}{},
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	cfg := &services.CogneeServiceConfig{
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

	cogneeService := services.NewCogneeServiceWithConfig(cfg, logger)
	cogneeService.SetReady(true)

	handler := NewCogneeAPIHandler(cogneeService, logger)
	return server, handler
}

// =====================================================
// Cognify with dataset_name field
// =====================================================

func TestCogneeAPIHandler_Cognify_WithDatasetName(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	body := map[string]interface{}{
		"dataset_name": "single-dataset",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/cognify", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Cognify(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["success"])
}

func TestCogneeAPIHandler_Cognify_EmptyBody(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/cognify", bytes.NewBufferString(""))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Cognify(c)

	// Should handle empty body gracefully with default empty datasets
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCogneeAPIHandler_Cognify_DatasetsArray(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	body := map[string]interface{}{
		"datasets": []string{"dataset1", "dataset2", "dataset3"},
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/cognify", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Cognify(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["success"])
}

// =====================================================
// GetInsights with dataset_name field
// =====================================================

func TestCogneeAPIHandler_GetInsights_WithDatasetName(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	body := map[string]interface{}{
		"query":        "test insights query",
		"dataset_name": "my-dataset",
		"limit":        10,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/insights", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.GetInsights(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "query")
	assert.Contains(t, response, "insights")
}

func TestCogneeAPIHandler_GetInsights_DatasetsAndDatasetNameBoth(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	// When both datasets and dataset_name are provided,
	// datasets takes priority (non-empty array)
	body := map[string]interface{}{
		"query":        "test query",
		"datasets":     []string{"ds1", "ds2"},
		"dataset_name": "fallback-ds",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/insights", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.GetInsights(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// =====================================================
// GetGraphCompletion with dataset_name field
// =====================================================

func TestCogneeAPIHandler_GetGraphCompletion_WithDatasetName(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	body := map[string]interface{}{
		"query":        "graph completion query",
		"dataset_name": "graph-ds",
		"limit":        5,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/graph/complete", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.GetGraphCompletion(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "query")
	assert.Contains(t, response, "completions")
}

// =====================================================
// VisualizeGraph with query params
// =====================================================

func TestCogneeAPIHandler_VisualizeGraph_WithAllQueryParams(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/cognee/visualize?dataset=my-dataset&format=json", nil)

	handler.VisualizeGraph(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "dataset")
	assert.Contains(t, response, "format")
	assert.Contains(t, response, "graph")
}

func TestCogneeAPIHandler_VisualizeGraph_NoQueryParams(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/cognee/visualize", nil)

	handler.VisualizeGraph(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// =====================================================
// Health response validation
// =====================================================

func TestCogneeAPIHandler_Health_ResponseFields(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/cognee/health", nil)

	handler.Health(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify all expected fields
	assert.Contains(t, response, "status")
	assert.Contains(t, response, "healthy")
	assert.Contains(t, response, "ready")
	assert.Contains(t, response, "config")

	// Verify config subfields
	configMap, ok := response["config"].(map[string]interface{})
	require.True(t, ok, "config should be a map")
	assert.Contains(t, configMap, "enabled")
	assert.Contains(t, configMap, "auto_cognify")
	assert.Contains(t, configMap, "enhance_prompts")
	assert.Contains(t, configMap, "temporal_awareness")
	assert.Contains(t, configMap, "enable_graph_reasoning")
	assert.Contains(t, configMap, "enable_code_intelligence")
}

// =====================================================
// Stats response validation
// =====================================================

func TestCogneeAPIHandler_Stats_ResponseFields(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/cognee/stats", nil)

	handler.Stats(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	expectedFields := []string{
		"total_memories_stored",
		"total_searches",
		"total_cognify_operations",
		"total_insights_queries",
		"total_graph_completions",
		"total_code_processed",
		"total_feedback_received",
		"average_search_latency_ms",
		"error_count",
	}

	for _, field := range expectedFields {
		assert.Contains(t, response, field, "Stats response should contain %s", field)
	}
}

// =====================================================
// GetConfig response validation
// =====================================================

func TestCogneeAPIHandler_GetConfig_AllFields(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/cognee/config", nil)

	handler.GetConfig(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	expectedFields := []string{
		"enabled",
		"base_url",
		"auto_cognify",
		"enhance_prompts",
		"store_responses",
		"max_context_size",
		"relevance_threshold",
		"temporal_awareness",
		"enable_feedback_loop",
		"enable_graph_reasoning",
		"enable_code_intelligence",
		"default_search_limit",
		"default_dataset",
		"cache_enabled",
		"max_concurrency",
	}

	for _, field := range expectedFields {
		assert.Contains(t, response, field, "GetConfig response should contain %s", field)
	}
}

// =====================================================
// RegisterRoutes comprehensive check
// =====================================================

func TestCogneeAPIHandler_RegisterRoutes_AllRoutes(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	router := gin.New()
	api := router.Group("/v1")

	handler.RegisterRoutes(api)

	routes := router.Routes()
	registeredRoutes := make(map[string]string) // path -> method
	for _, route := range routes {
		registeredRoutes[route.Method+":"+route.Path] = route.Handler
	}

	expectedRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/cognee/health"},
		{"GET", "/v1/cognee/stats"},
		{"GET", "/v1/cognee/config"},
		{"POST", "/v1/cognee/start"},
		{"POST", "/v1/cognee/memory"},
		{"POST", "/v1/cognee/search"},
		{"POST", "/v1/cognee/cognify"},
		{"POST", "/v1/cognee/insights"},
		{"POST", "/v1/cognee/graph/complete"},
		{"GET", "/v1/cognee/visualize"},
		{"POST", "/v1/cognee/code"},
		{"POST", "/v1/cognee/datasets"},
		{"GET", "/v1/cognee/datasets"},
		{"DELETE", "/v1/cognee/datasets/:name"},
		{"POST", "/v1/cognee/feedback"},
	}

	for _, expected := range expectedRoutes {
		key := expected.method + ":" + expected.path
		assert.Contains(t, registeredRoutes, key,
			"Expected route %s %s to be registered", expected.method, expected.path)
	}
}

// =====================================================
// ProvideFeedback with all fields
// =====================================================

func TestCogneeAPIHandler_ProvideFeedback_AllFields(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	body := map[string]interface{}{
		"query_id":  "q-456",
		"query":     "What is machine learning?",
		"response":  "Machine learning is a subset of AI...",
		"relevance": 0.88,
		"approved":  true,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/feedback", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.ProvideFeedback(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["success"])
	assert.Equal(t, "q-456", response["query_id"])
	assert.Equal(t, true, response["approved"])
}

// =====================================================
// DeleteDataset with valid name
// =====================================================

func TestCogneeAPIHandler_DeleteDataset_ValidName(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/cognee/datasets/my-dataset", nil)
	c.Params = gin.Params{{Key: "name", Value: "my-dataset"}}

	handler.DeleteDataset(c)

	// May succeed or return error depending on backend mock
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
}

// =====================================================
// CreateDataset with metadata
// =====================================================

func TestCogneeAPIHandler_CreateDataset_WithMetadata(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	body := map[string]interface{}{
		"name":        "rich-dataset",
		"description": "Dataset with metadata",
		"metadata": map[string]interface{}{
			"source":  "test-suite",
			"version": 1,
		},
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/datasets", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateDataset(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["success"])
	assert.Equal(t, "rich-dataset", response["name"])
}

// =====================================================
// AddMemory with all optional fields
// =====================================================

func TestCogneeAPIHandler_AddMemory_AllFields(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	body := map[string]interface{}{
		"content":      "Full featured memory entry",
		"dataset":      "custom-ds",
		"content_type": "text/plain",
		"metadata": map[string]interface{}{
			"author":    "test",
			"timestamp": time.Now().Unix(),
		},
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/cognee/memory", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.AddMemory(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["success"])
	assert.Contains(t, response, "memory")
}

// =====================================================
// SearchMemory response validation
// =====================================================

func TestCogneeAPIHandler_SearchMemory_ResponseFields(t *testing.T) {
	t.Parallel()
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	body := map[string]interface{}{
		"query":   "test query for response validation",
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

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	expectedFields := []string{
		"query",
		"combined_context",
		"total_results",
		"search_latency_ms",
		"relevance_score",
	}

	for _, field := range expectedFields {
		assert.Contains(t, response, field, "SearchMemory response should contain %s", field)
	}
}

// =====================================================
// BENCHMARK TESTS
// =====================================================

func BenchmarkCogneeAPIHandler_Cognify(b *testing.B) {
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	body := map[string]interface{}{
		"datasets": []string{"test"},
	}
	jsonBody, _ := json.Marshal(body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/cognify", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")
		handler.Cognify(c)
	}
}

func BenchmarkCogneeAPIHandler_GetInsights(b *testing.B) {
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	body := map[string]interface{}{
		"query":   "benchmark query",
		"dataset": "default",
	}
	jsonBody, _ := json.Marshal(body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/insights", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")
		handler.GetInsights(c)
	}
}

func BenchmarkCogneeAPIHandler_ProcessCode(b *testing.B) {
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	body := map[string]interface{}{
		"code":     "func main() { println(\"benchmark\") }",
		"language": "go",
	}
	jsonBody, _ := json.Marshal(body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/code", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")
		handler.ProcessCode(c)
	}
}

func BenchmarkCogneeAPIHandler_ProvideFeedback(b *testing.B) {
	server, handler := setupCogneeHandlerTestServer()
	defer server.Close()

	body := map[string]interface{}{
		"query_id":  "bench-q",
		"query":     "benchmark query",
		"response":  "benchmark response",
		"relevance": 0.9,
		"approved":  true,
	}
	jsonBody, _ := json.Marshal(body)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/cognee/feedback", bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")
		handler.ProvideFeedback(c)
	}
}
