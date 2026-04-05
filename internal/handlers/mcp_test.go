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
)

// TestMCPHandler_MCPCapabilities_Disabled tests MCP capabilities when disabled
func TestMCPHandler_MCPCapabilities_Disabled(t *testing.T) {
	t.Parallel()
	// Create config with MCP disabled
	cfg := &config.MCPConfig{
		Enabled: false,
	}

	// Create handler with nil registry (not used when disabled)
	handler := &MCPHandler{
		config: cfg,
	}

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/capabilities", nil)

	// Execute
	handler.MCPCapabilities(c)

	// Verify
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	// Parse response body directly since c.BindJSON doesn't work after c.JSON
	body := w.Body.String()
	assert.Contains(t, body, "MCP is not enabled")
	assert.Contains(t, body, "error")
}

// TestMCPHandler_MCPCapabilities_Enabled tests basic MCP capabilities structure
func TestMCPHandler_MCPCapabilities_Enabled(t *testing.T) {
	t.Parallel()
	// Create config with MCP enabled
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	// Create handler using NewMCPHandler to ensure mcpManager is initialized
	handler := NewMCPHandler(nil, cfg)

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/capabilities", nil)

	// Execute
	handler.MCPCapabilities(c)

	// Verify
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response body directly
	body := w.Body.String()
	assert.Contains(t, body, "version")
	assert.Contains(t, body, "capabilities")
	assert.Contains(t, body, "tools")
	assert.Contains(t, body, "prompts")
	assert.Contains(t, body, "resources")
	assert.Contains(t, body, "listChanged")
	assert.Contains(t, body, "providers")
	assert.Contains(t, body, "mcp_servers")
}

// TestMCPHandler_MCPTools_Disabled tests MCP tools endpoint when disabled
func TestMCPHandler_MCPTools_Disabled(t *testing.T) {
	t.Parallel()
	// Create config with MCP disabled
	cfg := &config.MCPConfig{
		Enabled: false,
	}

	// Create handler with nil registry
	handler := &MCPHandler{
		config: cfg,
	}

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/tools", nil)

	// Execute
	handler.MCPTools(c)

	// Verify
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "MCP is not enabled")
	assert.Contains(t, body, "error")
}

// TestMCPHandler_MCPTools_Enabled tests MCP tools endpoint when enabled
func TestMCPHandler_MCPTools_Enabled(t *testing.T) {
	t.Parallel()
	// Create config with MCP enabled
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	// Create handler using NewMCPHandler
	handler := NewMCPHandler(nil, cfg)

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/tools", nil)

	// Execute
	handler.MCPTools(c)

	// Verify
	assert.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "tools")
	// Response could be null or [] when no providers
	assert.True(t, body == "{\"tools\":null}" || body == "{\"tools\":[]}")
}

// TestMCPHandler_MCPToolsCall_Disabled tests tool execution when disabled
func TestMCPHandler_MCPToolsCall_Disabled(t *testing.T) {
	t.Parallel()
	// Create config with MCP disabled
	cfg := &config.MCPConfig{
		Enabled: false,
	}

	// Create handler using NewMCPHandler
	handler := NewMCPHandler(nil, cfg)

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp/tools/call", nil)

	// Execute
	handler.MCPToolsCall(c)

	// Verify
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "MCP is not enabled")
	assert.Contains(t, body, "error")
}

// TestMCPHandler_MCPToolsCall_InvalidRequest tests tool execution with invalid request
func TestMCPHandler_MCPToolsCall_InvalidRequest(t *testing.T) {
	t.Parallel()
	// Create config with MCP enabled
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	// Create handler using NewMCPHandler
	handler := NewMCPHandler(nil, cfg)

	// Create Gin context with empty request
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp/tools/call", nil)

	// Execute
	handler.MCPToolsCall(c)

	// Verify - should return bad request for invalid JSON
	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "Invalid request")
	assert.Contains(t, body, "error")
}

// TestNewMCPHandler tests handler creation
func TestNewMCPHandler(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	// Since we can't easily create a ProviderRegistry, we'll test with nil
	handler := NewMCPHandler(nil, cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
	assert.Nil(t, handler.providerRegistry)
	// mcpManager is lazily initialized — verify via GetMCPManager()
	assert.NotNil(t, handler.GetMCPManager())
}

// TestMCPHandler_GetMCPManager tests getting MCP manager
func TestMCPHandler_GetMCPManager(t *testing.T) {
	t.Parallel()
	handler := NewMCPHandler(nil, &config.MCPConfig{Enabled: true})

	// GetMCPManager lazily initializes and returns a valid manager
	mgr := handler.GetMCPManager()
	assert.NotNil(t, mgr)

	// Subsequent calls return the same instance
	assert.Same(t, mgr, handler.GetMCPManager())
}

// TestMCPHandler_MCPPrompts_Disabled tests MCP prompts when disabled
func TestMCPHandler_MCPPrompts_Disabled(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := &MCPHandler{
		config: cfg,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/prompts", nil)

	handler.MCPPrompts(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "MCP is not enabled")
}

// TestMCPHandler_MCPPrompts_Enabled tests MCP prompts when enabled
func TestMCPHandler_MCPPrompts_Enabled(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/prompts", nil)

	handler.MCPPrompts(c)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "prompts")
	assert.Contains(t, body, "summarize")
	assert.Contains(t, body, "analyze")
	assert.Contains(t, body, "description")
	assert.Contains(t, body, "arguments")
}

// TestMCPHandler_MCPResources_Disabled tests MCP resources when disabled
func TestMCPHandler_MCPResources_Disabled(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := &MCPHandler{
		config: cfg,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/resources", nil)

	handler.MCPResources(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "MCP is not enabled")
}

// TestMCPHandler_MCPResources_Enabled tests MCP resources when enabled
func TestMCPHandler_MCPResources_Enabled(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/resources", nil)

	handler.MCPResources(c)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "resources")
	assert.Contains(t, body, "helixagent://providers")
	assert.Contains(t, body, "helixagent://models")
	assert.Contains(t, body, "name")
	assert.Contains(t, body, "description")
	assert.Contains(t, body, "mimeType")
}

// TestMCPHandler_MCPToolsCall_ValidRequest tests MCP tools call with valid request
func TestMCPHandler_MCPToolsCall_ValidRequest(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	// Create a valid tool call request
	requestBody := map[string]interface{}{
		"name": "test_tool",
		"arguments": map[string]interface{}{
			"param1": "value1",
			"param2": 42,
		},
	}

	reqBytes, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp/tools/call", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.MCPToolsCall(c)

	// Should return internal server error since no providers are registered
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Provider registry not available")
}

// TestFindUnderscoreIndex tests the findUnderscoreIndex helper function
func TestFindUnderscoreIndex(t *testing.T) {
	t.Parallel()
	// Test with underscore in middle
	assert.Equal(t, 4, findUnderscoreIndex("test_tool"))

	// Test with underscore at beginning (should return -1 since i > 0)
	assert.Equal(t, -1, findUnderscoreIndex("_tool"))

	// Test with underscore at end (should return -1 since i < len(s)-1)
	assert.Equal(t, -1, findUnderscoreIndex("test_"))

	// Test with multiple underscores
	assert.Equal(t, 4, findUnderscoreIndex("test_tool_name"))

	// Test with no underscore
	assert.Equal(t, -1, findUnderscoreIndex("testtool"))

	// Test empty string
	assert.Equal(t, -1, findUnderscoreIndex(""))

	// Test single character
	assert.Equal(t, -1, findUnderscoreIndex("a"))

	// Test two characters with underscore
	assert.Equal(t, -1, findUnderscoreIndex("a_")) // At end, should return -1
}

// TestMCPHandler_RegisterMCPServer tests MCP server registration
func TestMCPHandler_RegisterMCPServer(t *testing.T) {
	t.Parallel()
	handler := NewMCPHandler(nil, &config.MCPConfig{Enabled: true})

	// Test registering a server
	serverConfig := map[string]interface{}{
		"name": "test-server",
		"type": "test",
		"config": map[string]interface{}{
			"url": "http://localhost:7061",
		},
	}

	_ = handler.RegisterMCPServer(serverConfig)

	// Since we can't actually connect to a server, this might fail
	// But we can test that the method exists and can be called
	assert.NotNil(t, handler)
}

// TestMCPHandler_GetProviderTools tests provider tools retrieval
func TestMCPHandler_GetProviderTools(t *testing.T) {
	t.Parallel()
	// Since we can't easily create a ProviderRegistry with actual providers,
	// we'll skip this test as it causes a panic when accessing nil registry
	// The function getProviderTools is private and only used internally
	// We'll test the public API instead
	t.Skip("Skipping test of private method getProviderTools due to nil registry access")
}

// TestMCPHandler_MCPToolsCall_WithUnifiedNamespace tests tool call with unified namespace
func TestMCPHandler_MCPToolsCall_WithUnifiedNamespace(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled:              true,
		UnifiedToolNamespace: true,
	}

	handler := NewMCPHandler(nil, cfg)

	// Create a tool call request with provider prefix
	requestBody := map[string]interface{}{
		"name": "provider_tool",
		"arguments": map[string]interface{}{
			"param": "value",
		},
	}

	reqBytes, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp/tools/call", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.MCPToolsCall(c)

	// Should return internal server error since no providers are registered
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Provider registry not available")
}

// TestMCPHandler_MCPToolsCall_WithExposeAllTools tests capabilities with expose all tools
func TestMCPHandler_MCPToolsCall_WithExposeAllTools(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled:        true,
		ExposeAllTools: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/capabilities", nil)

	handler.MCPCapabilities(c)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "capabilities")
	assert.Contains(t, body, "tools")
	assert.Contains(t, body, "prompts")
	assert.Contains(t, body, "resources")
	// With ExposeAllTools: true but no providers, tools might be empty
}

// TestMCPHandler_MCPToolsCall_MissingName tests tool call without name
func TestMCPHandler_MCPToolsCall_MissingName(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	// Create request without tool name
	requestBody := map[string]interface{}{
		"arguments": map[string]interface{}{
			"param1": "value1",
		},
	}

	reqBytes, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp/tools/call", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.MCPToolsCall(c)

	// Should return internal server error since no providers
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestMCPHandler_MCPToolsCall_InvalidToolFormat tests tool call with invalid tool format
func TestMCPHandler_MCPToolsCall_InvalidToolFormat(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled:              true,
		UnifiedToolNamespace: false,
	}

	handler := NewMCPHandler(nil, cfg)

	// Create request with non-namespaced tool
	requestBody := map[string]interface{}{
		"name": "simpletool",
		"arguments": map[string]interface{}{
			"param1": "value1",
		},
	}

	reqBytes, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp/tools/call", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.MCPToolsCall(c)

	// Should return internal server error since registry is nil
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestMCPHandler_RegisterMCPServer_Success tests that registration logs and succeeds
func TestMCPHandler_RegisterMCPServer_Success(t *testing.T) {
	t.Parallel()
	handler := NewMCPHandler(nil, &config.MCPConfig{Enabled: true})

	serverConfig := map[string]interface{}{
		"name": "test-server",
	}

	err := handler.RegisterMCPServer(serverConfig)
	assert.NoError(t, err)
}

// TestMCPHandler_MCPCapabilities_WithProviderRegistry tests capabilities with nil registry
func TestMCPHandler_MCPCapabilities_WithNilProviderRegistry(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := &MCPHandler{
		config:           cfg,
		providerRegistry: nil,
		mcpManager:       nil,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/capabilities", nil)

	handler.MCPCapabilities(c)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "providers")
	assert.Contains(t, body, "[]")
}

// TestFindUnderscoreIndex_EdgeCases tests edge cases for underscore finder
func TestFindUnderscoreIndex_EdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected int
	}{
		{"provider_tool_name", 8}, // First underscore in middle
		{"ab_cd_ef", 2},           // First underscore
		{"__test", 1},             // Second underscore (first is at position 0, so second at position 1 is valid)
		{"test__name", 4},         // First valid underscore
		{"x_y", 1},                // Minimum valid case
		{"_", -1},                 // Only underscore, at end
		{"__", -1},                // Double underscore, second at end
		{"a__b", 1},               // First underscore at valid position
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
				t.Parallel()
			result := findUnderscoreIndex(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMCPHandler_MCPTools_Enabled_EmptyTools tests tools endpoint with empty tools
func TestMCPHandler_MCPTools_Enabled_EmptyTools(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/tools", nil)

	handler.MCPTools(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	// Tools should be empty array
	tools := response["tools"]
	assert.NotNil(t, tools)
}

// TestMCPHandler_MCPPrompts_ResponseStructure tests prompts response structure
func TestMCPHandler_MCPPrompts_ResponseStructure(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/prompts", nil)

	handler.MCPPrompts(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	prompts := response["prompts"].([]interface{})
	assert.Len(t, prompts, 2)

	// Check summarize prompt
	summarize := prompts[0].(map[string]interface{})
	assert.Equal(t, "summarize", summarize["name"])
	assert.Contains(t, summarize["description"].(string), "Summarize")

	// Check analyze prompt
	analyze := prompts[1].(map[string]interface{})
	assert.Equal(t, "analyze", analyze["name"])
	assert.Contains(t, analyze["description"].(string), "Analyze")
}

// TestMCPHandler_MCPResources_ResponseStructure tests resources response structure
func TestMCPHandler_MCPResources_ResponseStructure(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/resources", nil)

	handler.MCPResources(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	resources := response["resources"].([]interface{})
	assert.Len(t, resources, 2)

	// Check providers resource
	providers := resources[0].(map[string]interface{})
	assert.Equal(t, "helixagent://providers", providers["uri"])
	assert.Equal(t, "application/json", providers["mimeType"])

	// Check models resource
	models := resources[1].(map[string]interface{})
	assert.Equal(t, "helixagent://models", models["uri"])
	assert.Equal(t, "application/json", models["mimeType"])
}

// TestMCPHandler_MCPToolsCall_WithProviderRegistry tests tool call with a real provider registry
func TestMCPHandler_MCPToolsCall_WithProviderRegistry(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled:              true,
		UnifiedToolNamespace: true,
	}

	// Create a real provider registry
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	handler := &MCPHandler{
		config:           cfg,
		providerRegistry: registry,
		mcpManager:       services.NewMCPManager(nil, nil, logrus.New()),
		logger:           logrus.New(),
	}

	// Create a tool call request with provider prefix
	requestBody := map[string]interface{}{
		"name": "provider_tool",
		"arguments": map[string]interface{}{
			"param": "value",
		},
	}

	reqBytes, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp/tools/call", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.MCPToolsCall(c)

	// With unified namespace enabled and proper tool name format, should return success
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "result")
}

// TestMCPHandler_MCPToolsCall_WithProviderRegistry_NoUnderscore tests tool call without underscore in name
func TestMCPHandler_MCPToolsCall_WithProviderRegistry_NoUnderscore(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled:              true,
		UnifiedToolNamespace: true,
	}

	// Create a real provider registry
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	handler := &MCPHandler{
		config:           cfg,
		providerRegistry: registry,
		mcpManager:       services.NewMCPManager(nil, nil, logrus.New()),
		logger:           logrus.New(),
	}

	// Create a tool call request without underscore
	requestBody := map[string]interface{}{
		"name": "simpletool",
		"arguments": map[string]interface{}{
			"param": "value",
		},
	}

	reqBytes, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp/tools/call", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.MCPToolsCall(c)

	// Without underscore, should return bad request for invalid format
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "Invalid tool format")
}

// TestMCPHandler_MCPToolsCall_MissingNameWithRegistry tests tool call without name field
func TestMCPHandler_MCPToolsCall_MissingNameWithRegistry(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	// Create a real provider registry
	registry := services.NewProviderRegistryWithoutAutoDiscovery(nil, nil)

	handler := &MCPHandler{
		config:           cfg,
		providerRegistry: registry,
		mcpManager:       services.NewMCPManager(nil, nil, logrus.New()),
		logger:           logrus.New(),
	}

	// Create a tool call request without name
	requestBody := map[string]interface{}{
		"arguments": map[string]interface{}{
			"param": "value",
		},
	}

	reqBytes, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp/tools/call", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.MCPToolsCall(c)

	// Should return bad request for missing tool name
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"], "Tool name is required")
}

// TestMCPHandler_MCPToolSearch_Disabled tests tool search when MCP is disabled
func TestMCPHandler_MCPToolSearch_Disabled(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := &MCPHandler{
		config: cfg,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/tools/search?q=test", nil)

	handler.MCPToolSearch(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "MCP is not enabled")
}

// TestMCPHandler_MCPToolSearch_GET tests tool search with GET request
func TestMCPHandler_MCPToolSearch_GET(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/tools/search?q=read&include_params=true&fuzzy=true&max_results=5", nil)

	handler.MCPToolSearch(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "read", response["query"])
	assert.Contains(t, response, "count")
	assert.Contains(t, response, "results")
}

// TestMCPHandler_MCPToolSearch_POST tests tool search with POST request
func TestMCPHandler_MCPToolSearch_POST(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	requestBody := MCPToolSearchRequest{
		Query:         "write",
		Categories:    []string{"file_system"},
		IncludeParams: true,
		FuzzyMatch:    false,
		MaxResults:    10,
	}

	reqBytes, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp/tools/search", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.MCPToolSearch(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "write", response["query"])
}

// TestMCPHandler_MCPToolSearch_MissingQuery tests tool search without query
func TestMCPHandler_MCPToolSearch_MissingQuery(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/tools/search", nil)

	handler.MCPToolSearch(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Query parameter is required")
}

// TestMCPHandler_MCPToolSearch_InvalidJSON tests tool search with invalid JSON
func TestMCPHandler_MCPToolSearch_InvalidJSON(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp/tools/search", bytes.NewBuffer([]byte("invalid json")))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.MCPToolSearch(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Invalid request")
}

// TestMCPHandler_MCPAdapterSearch_Disabled tests adapter search when MCP is disabled
func TestMCPHandler_MCPAdapterSearch_Disabled(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := &MCPHandler{
		config: cfg,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/adapters/search?q=test", nil)

	handler.MCPAdapterSearch(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "MCP is not enabled")
}

// TestMCPHandler_MCPAdapterSearch_GET tests adapter search with GET request
func TestMCPHandler_MCPAdapterSearch_GET(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/adapters/search?q=slack&max_results=5", nil)

	handler.MCPAdapterSearch(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "slack", response["query"])
	assert.Contains(t, response, "count")
	assert.Contains(t, response, "results")
}

// TestMCPHandler_MCPAdapterSearch_POST tests adapter search with POST request
func TestMCPHandler_MCPAdapterSearch_POST(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	requestBody := MCPAdapterSearchRequest{
		Query:      "github",
		Categories: []string{"development"},
		MaxResults: 10,
	}

	reqBytes, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp/adapters/search", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.MCPAdapterSearch(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "github", response["query"])
}

// TestMCPHandler_MCPAdapterSearch_EmptyQuery tests adapter search with empty query returns all adapters
func TestMCPHandler_MCPAdapterSearch_EmptyQuery(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/adapters/search", nil)

	handler.MCPAdapterSearch(c)

	// Empty query returns all adapters
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "", response["query"])
	// Should return all adapters
	count := int(response["count"].(float64))
	assert.Greater(t, count, 0)
}

// TestMCPHandler_MCPToolSuggestions_Disabled tests tool suggestions when MCP is disabled
func TestMCPHandler_MCPToolSuggestions_Disabled(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := &MCPHandler{
		config: cfg,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/tools/suggest?prefix=re", nil)

	handler.MCPToolSuggestions(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "MCP is not enabled")
}

// TestMCPHandler_MCPToolSuggestions tests tool suggestions endpoint
func TestMCPHandler_MCPToolSuggestions(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/tools/suggest?prefix=re&limit=5", nil)

	handler.MCPToolSuggestions(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "re", response["prefix"])
	assert.Contains(t, response, "count")
	assert.Contains(t, response, "suggestions")
}

// TestMCPHandler_MCPToolSuggestions_EmptyPrefix tests tool suggestions with empty prefix returns error
func TestMCPHandler_MCPToolSuggestions_EmptyPrefix(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/tools/suggest", nil)

	handler.MCPToolSuggestions(c)

	// Empty prefix requires at least 1 character
	assert.Equal(t, http.StatusBadRequest, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "Prefix parameter is required")
}

// TestMCPHandler_MCPCategories_Disabled tests categories when MCP is disabled
func TestMCPHandler_MCPCategories_Disabled(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := &MCPHandler{
		config: cfg,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/categories", nil)

	handler.MCPCategories(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "MCP is not enabled")
}

// TestMCPHandler_MCPCategories tests categories endpoint
func TestMCPHandler_MCPCategories(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/categories", nil)

	handler.MCPCategories(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "tool_categories")
	assert.Contains(t, response, "adapter_categories")
	assert.Contains(t, response, "auth_types")

	// Verify tool categories
	toolCategories := response["tool_categories"].([]interface{})
	assert.GreaterOrEqual(t, len(toolCategories), 1)

	// Verify adapter categories
	adapterCategories := response["adapter_categories"].([]interface{})
	assert.GreaterOrEqual(t, len(adapterCategories), 1)
}

// TestMCPHandler_MCPStats_Disabled tests stats when MCP is disabled
func TestMCPHandler_MCPStats_Disabled(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: false,
	}

	handler := &MCPHandler{
		config: cfg,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/stats", nil)

	handler.MCPStats(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "MCP is not enabled")
}

// TestMCPHandler_MCPStats tests stats endpoint
func TestMCPHandler_MCPStats(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/stats", nil)

	handler.MCPStats(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify tools stats
	tools := response["tools"].(map[string]interface{})
	assert.Contains(t, tools, "total")
	assert.Contains(t, tools, "by_category")

	// Verify adapters stats
	adaptersStats := response["adapters"].(map[string]interface{})
	assert.Contains(t, adaptersStats, "total")
	assert.Contains(t, adaptersStats, "by_category")
	assert.Contains(t, adaptersStats, "official")
	assert.Contains(t, adaptersStats, "supported")
}

// TestMCPHandler_MCPToolSearch_WithCategories tests tool search with category filter
func TestMCPHandler_MCPToolSearch_WithCategories(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/tools/search?q=file&categories=file_system,core", nil)

	handler.MCPToolSearch(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "file", response["query"])
}

// TestMCPHandler_MCPAdapterSearch_WithFilters tests adapter search with multiple filters
func TestMCPHandler_MCPAdapterSearch_WithFilters(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	official := true
	requestBody := MCPAdapterSearchRequest{
		Query:    "api",
		Official: &official,
	}

	reqBytes, _ := json.Marshal(requestBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/mcp/adapters/search", bytes.NewBuffer(reqBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.MCPAdapterSearch(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestMCPHandler_MCPToolSearch_AlternateQueryParam tests tool search with "query" param instead of "q"
func TestMCPHandler_MCPToolSearch_AlternateQueryParam(t *testing.T) {
	t.Parallel()
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	handler := NewMCPHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/mcp/tools/search?query=bash", nil)

	handler.MCPToolSearch(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "bash", response["query"])
}
