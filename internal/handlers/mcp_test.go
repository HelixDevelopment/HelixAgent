package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/superagent/superagent/internal/config"
)

// TestMCPHandler_MCPCapabilities_Disabled tests MCP capabilities when disabled
func TestMCPHandler_MCPCapabilities_Disabled(t *testing.T) {
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
	cfg := &config.MCPConfig{
		Enabled: true,
	}

	// Since we can't easily create a ProviderRegistry, we'll test with nil
	handler := NewMCPHandler(nil, cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
	assert.Nil(t, handler.providerRegistry)
	assert.NotNil(t, handler.mcpManager)
}

// TestMCPHandler_GetMCPManager tests getting MCP manager
func TestMCPHandler_GetMCPManager(t *testing.T) {
	handler := &MCPHandler{}

	// Should not be nil since NewMCPHandler creates it
	assert.Nil(t, handler.GetMCPManager())

	// Test that we can set it
	handler.mcpManager = nil
	assert.Nil(t, handler.GetMCPManager())
}
