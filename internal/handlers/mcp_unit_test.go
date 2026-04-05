package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMCPUnitTest creates a test environment for MCP handler tests
func setupMCPUnitTest() (*gin.Engine, *MCPHandler) {
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled:              true,
		UnifiedToolNamespace: true,
	}

	handler := NewMCPHandler(nil, cfg)

	router := gin.New()
	v1 := router.Group("/v1")
	{
		v1.GET("/mcp/capabilities", handler.MCPCapabilities)
		v1.GET("/mcp/tools", handler.MCPTools)
		v1.POST("/mcp/tools/call", handler.MCPToolsCall)
		v1.GET("/mcp/prompts", handler.MCPPrompts)
		v1.GET("/mcp/resources", handler.MCPResources)
		v1.GET("/mcp/tools/search", handler.MCPToolSearch)
		v1.POST("/mcp/tools/search", handler.MCPToolSearch)
		v1.GET("/mcp/adapters/search", handler.MCPAdapterSearch)
		v1.POST("/mcp/adapters/search", handler.MCPAdapterSearch)
		v1.GET("/mcp/tools/suggestions", handler.MCPToolSuggestions)
		v1.GET("/mcp/categories", handler.MCPCategories)
		v1.GET("/mcp/stats", handler.MCPStats)
	}

	return router, handler
}

// TestMCPHandler_MCPCapabilities_Success tests successful capabilities request
func TestMCPHandlerUnit_MCPCapabilities_Success(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/capabilities", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", response["version"])
	assert.Contains(t, response, "capabilities")
	assert.Contains(t, response, "providers")
	assert.Contains(t, response, "mcp_servers")
}

// TestMCPHandler_MCPCapabilities_Disabled tests capabilities when MCP is disabled
func TestMCPHandlerUnit_MCPCapabilities_Disabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := NewMCPHandler(nil, cfg)

	router := gin.New()
	router.GET("/v1/mcp/capabilities", handler.MCPCapabilities)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/capabilities", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "MCP is not enabled", response["error"])
}

// TestMCPHandler_MCPTools_Success tests successful tools request
func TestMCPHandlerUnit_MCPTools_Success(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "tools")
}

// TestMCPHandler_MCPTools_Disabled tests tools when MCP is disabled
func TestMCPHandlerUnit_MCPTools_Disabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := NewMCPHandler(nil, cfg)

	router := gin.New()
	router.GET("/v1/mcp/tools", handler.MCPTools)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestMCPHandler_MCPToolsCall_Success tests successful tool execution
func TestMCPHandlerUnit_MCPToolsCall_Success(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled:              true,
		UnifiedToolNamespace: true,
	}

	// Create provider registry
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	handler := &MCPHandler{
		config:           cfg,
		providerRegistry: registry,
		mcpManager:       services.NewMCPManager(nil, nil, logrus.New()),
		logger:           logrus.New(),
	}

	router := gin.New()
	router.POST("/v1/mcp/tools/call", handler.MCPToolsCall)

	reqBody := map[string]interface{}{
		"name": "provider_tool",
		"arguments": map[string]interface{}{
			"param1": "value1",
			"param2": 42,
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "result")
	assert.Contains(t, response, "arguments")
}

// TestMCPHandler_MCPToolsCall_Disabled tests tool execution when MCP is disabled
func TestMCPHandlerUnit_MCPToolsCall_Disabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := NewMCPHandler(nil, cfg)

	router := gin.New()
	router.POST("/v1/mcp/tools/call", handler.MCPToolsCall)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestMCPHandler_MCPToolsCall_InvalidJSON tests tool execution with invalid JSON
func TestMCPHandlerUnit_MCPToolsCall_InvalidJSON(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestMCPHandler_MCPToolsCall_MissingName tests tool execution without name
func TestMCPHandlerUnit_MCPToolsCall_MissingName(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled:              true,
		UnifiedToolNamespace: true,
	}

	// Create provider registry
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	handler := &MCPHandler{
		config:           cfg,
		providerRegistry: registry,
		mcpManager:       services.NewMCPManager(nil, nil, logrus.New()),
		logger:           logrus.New(),
	}

	router := gin.New()
	router.POST("/v1/mcp/tools/call", handler.MCPToolsCall)

	reqBody := map[string]interface{}{
		"arguments": map[string]interface{}{
			"param": "value",
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Tool name is required")
}

// TestMCPHandler_MCPToolsCall_NoProviderRegistry tests tool execution without provider registry
func TestMCPHandlerUnit_MCPToolsCall_NoProviderRegistry(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	reqBody := map[string]interface{}{
		"name": "test_tool",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Provider registry not available")
}

// TestMCPHandler_MCPToolsCall_InvalidToolFormat tests tool execution with invalid format
func TestMCPHandlerUnit_MCPToolsCall_InvalidToolFormat(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled:              true,
		UnifiedToolNamespace: false, // Disabled
	}

	// Create provider registry
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	handler := &MCPHandler{
		config:           cfg,
		providerRegistry: registry,
		mcpManager:       services.NewMCPManager(nil, nil, logrus.New()),
		logger:           logrus.New(),
	}

	router := gin.New()
	router.POST("/v1/mcp/tools/call", handler.MCPToolsCall)

	reqBody := map[string]interface{}{
		"name": "simpletool", // No underscore
		"arguments": map[string]interface{}{
			"param": "value",
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Invalid tool format")
}

// TestMCPHandler_MCPPrompts_Success tests successful prompts request
func TestMCPHandlerUnit_MCPPrompts_Success(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/prompts", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "prompts")

	prompts := response["prompts"].([]interface{})
	assert.Len(t, prompts, 2)

	// Check summarize prompt
	summarize := prompts[0].(map[string]interface{})
	assert.Equal(t, "summarize", summarize["name"])
	assert.Contains(t, summarize["description"], "Summarize")

	// Check analyze prompt
	analyze := prompts[1].(map[string]interface{})
	assert.Equal(t, "analyze", analyze["name"])
}

// TestMCPHandler_MCPPrompts_Disabled tests prompts when MCP is disabled
func TestMCPHandlerUnit_MCPPrompts_Disabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := NewMCPHandler(nil, cfg)

	router := gin.New()
	router.GET("/v1/mcp/prompts", handler.MCPPrompts)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/prompts", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestMCPHandler_MCPResources_Success tests successful resources request
func TestMCPHandlerUnit_MCPResources_Success(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/resources", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "resources")

	resources := response["resources"].([]interface{})
	assert.Len(t, resources, 2)

	// Check providers resource
	providers := resources[0].(map[string]interface{})
	assert.Equal(t, "helixagent://providers", providers["uri"])
	assert.Equal(t, "application/json", providers["mimeType"])

	// Check models resource
	models := resources[1].(map[string]interface{})
	assert.Equal(t, "helixagent://models", models["uri"])
}

// TestMCPHandler_MCPResources_Disabled tests resources when MCP is disabled
func TestMCPHandlerUnit_MCPResources_Disabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := NewMCPHandler(nil, cfg)

	router := gin.New()
	router.GET("/v1/mcp/resources", handler.MCPResources)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/resources", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestMCPHandler_MCPToolSearch_GET tests tool search with GET request
func TestMCPHandlerUnit_MCPToolSearch_GET(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools/search?q=read&categories=file_system&include_params=true&fuzzy=true&max_results=5", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "read", response["query"])
	assert.Contains(t, response, "count")
	assert.Contains(t, response, "results")
}

// TestMCPHandler_MCPToolSearch_POST tests tool search with POST request
func TestMCPHandlerUnit_MCPToolSearch_POST(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	reqBody := MCPToolSearchRequest{
		Query:         "write",
		Categories:    []string{"file_system"},
		IncludeParams: true,
		FuzzyMatch:    false,
		MaxResults:    10,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "write", response["query"])
}

// TestMCPHandler_MCPToolSearch_Disabled tests tool search when MCP is disabled
func TestMCPHandlerUnit_MCPToolSearch_Disabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := NewMCPHandler(nil, cfg)

	router := gin.New()
	router.GET("/v1/mcp/tools/search", handler.MCPToolSearch)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools/search?q=test", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestMCPHandler_MCPToolSearch_MissingQuery tests tool search without query
func TestMCPHandlerUnit_MCPToolSearch_MissingQuery(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools/search", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Query parameter is required")
}

// TestMCPHandler_MCPToolSearch_InvalidJSON tests tool search with invalid JSON
func TestMCPHandlerUnit_MCPToolSearch_InvalidJSON(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/search", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestMCPHandler_MCPAdapterSearch_GET tests adapter search with GET request
func TestMCPHandlerUnit_MCPAdapterSearch_GET(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/adapters/search?q=slack&max_results=5", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "slack", response["query"])
	assert.Contains(t, response, "count")
	assert.Contains(t, response, "results")
}

// TestMCPHandler_MCPAdapterSearch_POST tests adapter search with POST request
func TestMCPHandlerUnit_MCPAdapterSearch_POST(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	reqBody := MCPAdapterSearchRequest{
		Query:      "github",
		Categories: []string{"development"},
		MaxResults: 10,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/adapters/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "github", response["query"])
}

// TestMCPHandler_MCPAdapterSearch_Disabled tests adapter search when MCP is disabled
func TestMCPHandlerUnit_MCPAdapterSearch_Disabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := NewMCPHandler(nil, cfg)

	router := gin.New()
	router.GET("/v1/mcp/adapters/search", handler.MCPAdapterSearch)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/adapters/search?q=test", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestMCPHandler_MCPAdapterSearch_EmptyQuery tests adapter search with empty query
func TestMCPHandlerUnit_MCPAdapterSearch_EmptyQuery(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/adapters/search", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "", response["query"])
	// Should return all adapters
	count := int(response["count"].(float64))
	assert.Greater(t, count, 0)
}

// TestMCPHandler_MCPToolSuggestions_Success tests successful tool suggestions
func TestMCPHandlerUnit_MCPToolSuggestions_Success(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools/suggestions?prefix=read&max=5", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "read", response["prefix"])
	assert.Contains(t, response, "count")
	assert.Contains(t, response, "suggestions")
}

// TestMCPHandler_MCPToolSuggestions_MissingPrefix tests suggestions without prefix
func TestMCPHandlerUnit_MCPToolSuggestions_MissingPrefix(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools/suggestions", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["error"], "Prefix parameter is required")
}

// TestMCPHandler_MCPToolSuggestions_Disabled tests suggestions when MCP is disabled
func TestMCPHandlerUnit_MCPToolSuggestions_Disabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := NewMCPHandler(nil, cfg)

	router := gin.New()
	router.GET("/v1/mcp/tools/suggestions", handler.MCPToolSuggestions)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools/suggestions?prefix=test", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestMCPHandler_MCPCategories_Success tests successful categories request
func TestMCPHandlerUnit_MCPCategories_Success(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/categories", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "tool_categories")
	assert.Contains(t, response, "adapter_categories")
	assert.Contains(t, response, "auth_types")
}

// TestMCPHandler_MCPCategories_Disabled tests categories when MCP is disabled
func TestMCPHandlerUnit_MCPCategories_Disabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := NewMCPHandler(nil, cfg)

	router := gin.New()
	router.GET("/v1/mcp/categories", handler.MCPCategories)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/categories", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestMCPHandler_MCPStats_Success tests successful stats request
func TestMCPHandlerUnit_MCPStats_Success(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/stats", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "tools")
	assert.Contains(t, response, "adapters")

	tools := response["tools"].(map[string]interface{})
	assert.Contains(t, tools, "total")
	assert.Contains(t, tools, "by_category")

	adapters := response["adapters"].(map[string]interface{})
	assert.Contains(t, adapters, "total")
	assert.Contains(t, adapters, "by_category")
	assert.Contains(t, adapters, "official")
	assert.Contains(t, adapters, "supported")
}

// TestMCPHandler_MCPStats_Disabled tests stats when MCP is disabled
func TestMCPHandlerUnit_MCPStats_Disabled(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := NewMCPHandler(nil, cfg)

	router := gin.New()
	router.GET("/v1/mcp/stats", handler.MCPStats)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/stats", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestMCPHandler_NewMCPHandler tests handler creation
func TestMCPHandlerUnit_NewMCPHandler(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
	assert.Nil(t, handler.providerRegistry)
	assert.NotNil(t, handler.GetMCPManager())
}

// TestMCPHandler_GetMCPManager tests getting MCP manager
func TestMCPHandlerUnit_GetMCPManager(t *testing.T) {
	t.Parallel()
	handler := NewMCPHandler(nil, &config.MCPConfig{Enabled: true})

	// GetMCPManager lazily initializes and returns a valid manager
	mgr := handler.GetMCPManager()
	assert.NotNil(t, mgr)

	// Subsequent calls return the same instance
	assert.Same(t, mgr, handler.GetMCPManager())
}

// TestMCPHandler_RegisterMCPServer tests MCP server registration
func TestMCPHandlerUnit_RegisterMCPServer(t *testing.T) {
	t.Parallel()
	handler := NewMCPHandler(nil, &config.MCPConfig{Enabled: true})

	serverConfig := map[string]interface{}{
		"name": "test-server",
		"type": "test",
		"config": map[string]interface{}{
			"url": "http://localhost:7061",
		},
	}

	err := handler.RegisterMCPServer(serverConfig)
	assert.NoError(t, err)
}

// TestFindUnderscoreIndex tests the findUnderscoreIndex helper function
func TestFindUnderscoreIndexUnit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected int
	}{
		{"test_tool", 4},               // First underscore in middle
		{"provider_tool_name", 8},      // First underscore
		{"ab_cd_ef", 2},                // First underscore
		{"__test", 1},                  // Second underscore (first is at position 0)
		{"test__name", 4},              // First valid underscore
		{"x_y", 1},                     // Minimum valid case
		{"_", -1},                      // Only underscore, at start/end
		{"__", -1},                     // Double underscore, both invalid
		{"a__b", 1},                    // First underscore at valid position
		{"testtool", -1},               // No underscore
		{"", -1},                       // Empty string
		{"a", -1},                      // Single character
		{"a_", -1},                     // Underscore at end
		{"_a", -1},                     // Underscore at start
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
				t.Parallel()
			result := findUnderscoreIndex(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMCPHandler_MCPCapabilities_WithProviderRegistry tests capabilities with provider registry
func TestMCPHandlerUnit_MCPCapabilities_WithProviderRegistry(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled: true,
	}

	// Create provider registry
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	handler := &MCPHandler{
		config:           cfg,
		providerRegistry: registry,
		mcpManager:       services.NewMCPManager(nil, nil, logrus.New()),
		logger:           logrus.New(),
	}

	router := gin.New()
	router.GET("/v1/mcp/capabilities", handler.MCPCapabilities)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/capabilities", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "providers")
	assert.Contains(t, response, "mcp_servers")
}

// TestMCPHandler_MCPToolsCall_WithUnifiedNamespace tests tool call with unified namespace
func TestMCPHandlerUnit_MCPToolsCall_WithUnifiedNamespace(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled:              true,
		UnifiedToolNamespace: true,
	}

	// Create provider registry
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	handler := &MCPHandler{
		config:           cfg,
		providerRegistry: registry,
		mcpManager:       services.NewMCPManager(nil, nil, logrus.New()),
		logger:           logrus.New(),
	}

	router := gin.New()
	router.POST("/v1/mcp/tools/call", handler.MCPToolsCall)

	reqBody := map[string]interface{}{
		"name": "claude_complete",
		"arguments": map[string]interface{}{
			"prompt": "Hello",
		},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "result")
}

// TestMCPHandler_MCPToolsCall_WithNilProviderRegistry tests tool call with nil registry
func TestMCPHandlerUnit_MCPToolsCall_WithNilProviderRegistry(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.MCPConfig{
		Enabled:              true,
		UnifiedToolNamespace: true,
	}

	handler := &MCPHandler{
		config:           cfg,
		providerRegistry: nil,
		mcpManager:       services.NewMCPManager(nil, nil, logrus.New()),
		logger:           logrus.New(),
	}

	router := gin.New()
	router.POST("/v1/mcp/tools/call", handler.MCPToolsCall)

	reqBody := map[string]interface{}{
		"name": "test_tool",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestMCPHandler_MCPToolSearch_GET_WithAllParams tests tool search with all query params
func TestMCPHandlerUnit_MCPToolSearch_GET_WithAllParams(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools/search?q=read&query=write&categories=core,file_system&include_params=true&fuzzy=true&max_results=10", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	// The first 'q' parameter should be used
	assert.Equal(t, "read", response["query"])
}

// TestMCPHandler_MCPAdapterSearch_WithFilters tests adapter search with filters
func TestMCPHandlerUnit_MCPAdapterSearch_WithFilters(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/adapters/search?q=github&categories=development&auth_types=api_key&official=true&supported=true&max_results=5", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "github", response["query"])
	assert.Contains(t, response, "results")
}

// TestMCPHandler_MCPAdapterSearch_InvalidJSON tests adapter search with invalid JSON
func TestMCPHandlerUnit_MCPAdapterSearch_InvalidJSON(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/adapters/search", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	// Empty body or invalid JSON should not cause panic
	// The handler handles this gracefully
}

// TestMCPHandler_MCPToolSuggestions_DefaultMax tests suggestions with default max
func TestMCPHandlerUnit_MCPToolSuggestions_DefaultMax(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools/suggestions?prefix=search", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "search", response["prefix"])
	assert.Contains(t, response, "suggestions")
}

// TestMCPHandler_MCPToolSuggestions_InvalidMax tests suggestions with invalid max
func TestMCPHandlerUnit_MCPToolSuggestions_InvalidMax(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools/suggestions?prefix=search&max=invalid", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	// Should use default max value
	assert.Equal(t, "search", response["prefix"])
}

// TestMCPHandler_MCPTools_EmptyResponse tests tools endpoint returns empty array
func TestMCPHandlerUnit_MCPTools_EmptyResponse(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/tools", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "tools")
}

// TestMCPHandler_MCPResources_ResponseStructure tests resources response structure
func TestMCPHandlerUnit_MCPResources_ResponseStructure(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/resources", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	resources := response["resources"].([]interface{})
	require.Len(t, resources, 2)

	// Check structure of each resource
	for _, r := range resources {
		resource := r.(map[string]interface{})
		assert.Contains(t, resource, "uri")
		assert.Contains(t, resource, "name")
		assert.Contains(t, resource, "description")
		assert.Contains(t, resource, "mimeType")
	}
}

// TestMCPHandler_MCPPrompts_ResponseStructure tests prompts response structure
func TestMCPHandlerUnit_MCPPrompts_ResponseStructure(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/prompts", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	prompts := response["prompts"].([]interface{})
	require.Len(t, prompts, 2)

	// Check structure of each prompt
	for _, p := range prompts {
		prompt := p.(map[string]interface{})
		assert.Contains(t, prompt, "name")
		assert.Contains(t, prompt, "description")
		assert.Contains(t, prompt, "arguments")
	}
}

// TestMCPHandler_MCPCapabilities_ResponseStructure tests capabilities response structure
func TestMCPHandlerUnit_MCPCapabilities_ResponseStructure(t *testing.T) {
	t.Parallel()
	router, _ := setupMCPUnitTest()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/mcp/capabilities", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "version")
	assert.Contains(t, response, "capabilities")
	assert.Contains(t, response, "providers")
	assert.Contains(t, response, "mcp_servers")

	capabilities := response["capabilities"].(map[string]interface{})
	assert.Contains(t, capabilities, "tools")
	assert.Contains(t, capabilities, "prompts")
	assert.Contains(t, capabilities, "resources")

	tools := capabilities["tools"].(map[string]interface{})
	assert.Contains(t, tools, "listChanged")
}
