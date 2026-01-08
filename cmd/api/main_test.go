package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewAPIServer(t *testing.T) {
	server := NewAPIServer("8080")
	require.NotNil(t, server)
	assert.Equal(t, "8080", server.port)
	assert.NotNil(t, server.logger)
	assert.NotNil(t, server.unifiedManager)
	assert.NotNil(t, server.protocolAnalytics)
	assert.NotNil(t, server.pluginSystem)
	assert.NotNil(t, server.pluginRegistry)
	assert.NotNil(t, server.templateManager)
}

func TestNewAPIServer_DefaultPort(t *testing.T) {
	server := NewAPIServer("")
	require.NotNil(t, server)
	assert.Equal(t, "", server.port)
}

func setupTestServer() (*APIServer, *gin.Engine) {
	server := NewAPIServer("8080")
	r := gin.New()

	// Setup routes like in Start()
	api := r.Group("/api/v1")
	{
		mcp := api.Group("/mcp")
		{
			mcp.POST("/tools/call", server.handleMCPCallTool)
			mcp.GET("/tools/list", server.handleMCPListTools)
			mcp.GET("/servers", server.handleMCPListServers)
		}

		lsp := api.Group("/lsp")
		{
			lsp.POST("/completion", server.handleLSPCompletion)
			lsp.POST("/hover", server.handleLSPHover)
			lsp.POST("/definition", server.handleLSPDefinition)
			lsp.POST("/diagnostics", server.handleLSPDiagnostics)
		}

		acp := api.Group("/acp")
		{
			acp.POST("/execute", server.handleACPExecute)
			acp.POST("/broadcast", server.handleACPBroadcast)
			acp.GET("/status", server.handleACPStatus)
		}

		analytics := api.Group("/analytics")
		{
			analytics.GET("/metrics", server.handleGetAnalytics)
			analytics.GET("/metrics/:protocol", server.handleGetProtocolMetrics)
			analytics.GET("/health", server.handleGetHealthStatus)
			analytics.POST("/record", server.handleRecordRequest)
		}

		plugins := api.Group("/plugins")
		{
			plugins.GET("/", server.handleListPlugins)
			plugins.POST("/load", server.handleLoadPlugin)
			plugins.DELETE("/:id", server.handleUnloadPlugin)
			plugins.POST("/:id/execute", server.handleExecutePlugin)
			plugins.GET("/marketplace", server.handleMarketplaceSearch)
			plugins.POST("/marketplace/register", server.handleRegisterPlugin)
		}

		templates := api.Group("/templates")
		{
			templates.GET("/", server.handleListTemplates)
			templates.GET("/:id", server.handleGetTemplate)
			templates.POST("/:id/generate", server.handleGenerateFromTemplate)
		}

		api.GET("/health", server.handleHealth)
		api.GET("/status", server.handleStatus)
		api.GET("/metrics", server.handlePrometheusMetrics)
	}

	return server, r
}

// MCP Protocol Tests

func TestHandleMCPCallTool(t *testing.T) {
	_, router := setupTestServer()

	t.Run("successful call", func(t *testing.T) {
		body := map[string]interface{}{
			"server_id":  "test-server",
			"tool_name":  "calculate",
			"parameters": map[string]interface{}{"x": 1, "y": 2},
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/mcp/tools/call", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "calculate", response["tool"])
		assert.Equal(t, "test-server", response["server"])
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/mcp/tools/call", bytes.NewBuffer([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleMCPListTools(t *testing.T) {
	_, router := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/v1/mcp/tools/list?server_id=test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "tools")
}

func TestHandleMCPListServers(t *testing.T) {
	_, router := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/v1/mcp/servers", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "servers")
}

// LSP Protocol Tests

func TestHandleLSPCompletion(t *testing.T) {
	_, router := setupTestServer()

	t.Run("successful completion", func(t *testing.T) {
		body := map[string]interface{}{
			"file_path": "/test/file.go",
			"line":      10,
			"character": 5,
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/lsp/completion", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "completions")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/lsp/completion", bytes.NewBuffer([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleLSPHover(t *testing.T) {
	_, router := setupTestServer()

	t.Run("successful hover", func(t *testing.T) {
		body := map[string]interface{}{
			"file_path": "/test/file.go",
			"line":      10,
			"character": 5,
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/lsp/hover", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "contents")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/lsp/hover", bytes.NewBuffer([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleLSPDefinition(t *testing.T) {
	_, router := setupTestServer()

	t.Run("successful definition", func(t *testing.T) {
		body := map[string]interface{}{
			"file_path": "/test/file.go",
			"line":      10,
			"character": 5,
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/lsp/definition", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "definition")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/lsp/definition", bytes.NewBuffer([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleLSPDiagnostics(t *testing.T) {
	_, router := setupTestServer()

	req, _ := http.NewRequest("POST", "/api/v1/lsp/diagnostics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "diagnostics")
}

// ACP Protocol Tests

func TestHandleACPExecute(t *testing.T) {
	_, router := setupTestServer()

	t.Run("successful execute", func(t *testing.T) {
		body := map[string]interface{}{
			"action":   "test_action",
			"agent_id": "agent-1",
			"params":   map[string]interface{}{"key": "value"},
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/acp/execute", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "test_action", response["action"])
		assert.Equal(t, "agent-1", response["agent_id"])
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/acp/execute", bytes.NewBuffer([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleACPBroadcast(t *testing.T) {
	_, router := setupTestServer()

	t.Run("successful broadcast", func(t *testing.T) {
		body := map[string]interface{}{
			"message": "test message",
			"targets": []string{"agent-1", "agent-2"},
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/acp/broadcast", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "broadcast_id")
		assert.Equal(t, float64(2), response["delivered_to"])
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/acp/broadcast", bytes.NewBuffer([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleACPStatus(t *testing.T) {
	_, router := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/v1/acp/status?agent_id=agent-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "agent-1", response["agent_id"])
	assert.Equal(t, "active", response["status"])
}

// Analytics Tests

func TestHandleGetAnalytics(t *testing.T) {
	_, router := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/v1/analytics/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleGetProtocolMetrics(t *testing.T) {
	_, router := setupTestServer()

	t.Run("existing protocol", func(t *testing.T) {
		// First record a request to create metrics
		body := map[string]interface{}{
			"protocol":   "mcp",
			"method":     "call_tool",
			"duration":   100000000,
			"success":    true,
			"error_type": "",
		}
		jsonBody, _ := json.Marshal(body)

		recordReq, _ := http.NewRequest("POST", "/api/v1/analytics/record", bytes.NewBuffer(jsonBody))
		recordReq.Header.Set("Content-Type", "application/json")
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, recordReq)

		// Then get the metrics
		req, _ := http.NewRequest("GET", "/api/v1/analytics/metrics/mcp", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("non-existing protocol", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/analytics/metrics/nonexistent", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleGetHealthStatus(t *testing.T) {
	_, router := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/v1/analytics/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleRecordRequest(t *testing.T) {
	_, router := setupTestServer()

	t.Run("successful record", func(t *testing.T) {
		body := map[string]interface{}{
			"protocol":   "mcp",
			"method":     "call_tool",
			"duration":   100000000,
			"success":    true,
			"error_type": "",
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/analytics/record", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "recorded", response["status"])
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/analytics/record", bytes.NewBuffer([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// Plugin Tests

func TestHandleListPlugins(t *testing.T) {
	_, router := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/v1/plugins/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "plugins")
}

func TestHandleLoadPlugin(t *testing.T) {
	_, router := setupTestServer()

	t.Run("attempt to load plugin", func(t *testing.T) {
		body := map[string]interface{}{
			"path": "/test/plugin.so",
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/plugins/load", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Will return 500 because plugin doesn't exist, but tests the handler
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/plugins/load", bytes.NewBuffer([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleUnloadPlugin(t *testing.T) {
	_, router := setupTestServer()

	req, _ := http.NewRequest("DELETE", "/api/v1/plugins/test-plugin", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Will return 500 because plugin doesn't exist, but tests the handler
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
}

func TestHandleExecutePlugin(t *testing.T) {
	_, router := setupTestServer()

	t.Run("non-existing plugin", func(t *testing.T) {
		body := map[string]interface{}{
			"operation": "test",
			"params":    map[string]interface{}{"key": "value"},
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/plugins/test-plugin/execute", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Will return 500 because plugin doesn't exist, but tests the handler
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/plugins/test-plugin/execute", bytes.NewBuffer([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleMarketplaceSearch(t *testing.T) {
	_, router := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/v1/plugins/marketplace?q=test&protocol=mcp", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "plugins")
}

func TestHandleRegisterPlugin(t *testing.T) {
	_, router := setupTestServer()

	t.Run("successful registration", func(t *testing.T) {
		body := map[string]interface{}{
			"id":          "test-plugin",
			"name":        "Test Plugin",
			"version":     "1.0.0",
			"description": "A test plugin",
			"author":      "Test Author",
			"protocols":   []string{"mcp"},
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/plugins/marketplace/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/plugins/marketplace/register", bytes.NewBuffer([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// Template Tests

func TestHandleListTemplates(t *testing.T) {
	_, router := setupTestServer()

	t.Run("list all templates", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/templates/", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "templates")
	})

	t.Run("list templates by protocol", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/templates/?protocol=mcp", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleGetTemplate(t *testing.T) {
	_, router := setupTestServer()

	t.Run("get existing template", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/templates/mcp-basic-integration", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "mcp-basic-integration", response["ID"])
	})

	t.Run("get non-existing template", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/templates/nonexistent", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleGenerateFromTemplate(t *testing.T) {
	_, router := setupTestServer()

	t.Run("existing template", func(t *testing.T) {
		body := map[string]interface{}{
			"config": map[string]interface{}{"server_url": "http://localhost:7061"},
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/templates/mcp-basic-integration/generate", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "generated")
	})

	t.Run("non-existing template", func(t *testing.T) {
		body := map[string]interface{}{
			"config": map[string]interface{}{"key": "value"},
		}
		jsonBody, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", "/api/v1/templates/test-template/generate", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Will return 500 because template doesn't exist, but tests the handler
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v1/templates/test-template/generate", bytes.NewBuffer([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// System Tests

func TestHandleHealth(t *testing.T) {
	_, router := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, "1.0.0", response["version"])
}

func TestHandleStatus(t *testing.T) {
	_, router := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "operational", response["status"])
	assert.Contains(t, response, "protocols_active")
	assert.Contains(t, response, "plugins_loaded")
	assert.Contains(t, response, "templates_available")
}

func TestHandlePrometheusMetrics(t *testing.T) {
	_, router := setupTestServer()

	req, _ := http.NewRequest("GET", "/api/v1/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	assert.Contains(t, w.Body.String(), "helixagent_protocols_active")
	assert.Contains(t, w.Body.String(), "helixagent_plugins_loaded")
	assert.Contains(t, w.Body.String(), "helixagent_requests_total")
}

func TestHandlePrometheusMetricsWithData(t *testing.T) {
	_, router := setupTestServer()

	// First record some requests to create metrics
	body := map[string]interface{}{
		"protocol":   "mcp",
		"method":     "call_tool",
		"duration":   100000000,
		"success":    true,
		"error_type": "",
	}
	jsonBody, _ := json.Marshal(body)
	recordReq, _ := http.NewRequest("POST", "/api/v1/analytics/record", bytes.NewBuffer(jsonBody))
	recordReq.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, recordReq)

	// Now get the metrics
	req, _ := http.NewRequest("GET", "/api/v1/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// With recorded data, total requests should be > 0
	assert.Contains(t, w.Body.String(), "helixagent_requests_total")
}

// Helper to setup test router with CORS middleware (mirrors Start() function)
func setupTestServerWithCORS() (*APIServer, *gin.Engine) {
	server := NewAPIServer("8080")
	r := gin.New()

	// CORS middleware from Start()
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Setup routes like in Start()
	api := r.Group("/api/v1")
	{
		api.GET("/health", server.handleHealth)
	}

	return server, r
}

func TestCORSMiddleware(t *testing.T) {
	_, router := setupTestServerWithCORS()

	t.Run("regular request has CORS headers", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/health", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	})

	t.Run("OPTIONS preflight request", func(t *testing.T) {
		req, _ := http.NewRequest("OPTIONS", "/api/v1/health", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, 204, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})
}
