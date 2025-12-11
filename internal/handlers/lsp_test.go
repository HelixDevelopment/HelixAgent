package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/superagent/superagent/internal/config"
)

// TestLSPHandler_LSPCapabilities_Disabled tests LSP capabilities when disabled
func TestLSPHandler_LSPCapabilities_Disabled(t *testing.T) {
	// Create config with LSP disabled
	cfg := &config.LSPConfig{
		Enabled: false,
	}

	// Create handler with nil registry (not used when disabled)
	handler := &LSPHandler{
		config: cfg,
	}

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/lsp/capabilities", nil)

	// Execute
	handler.LSPCapabilities(c)

	// Verify
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	// Parse response body directly since c.BindJSON doesn't work after c.JSON
	body := w.Body.String()
	assert.Contains(t, body, "LSP is not enabled")
	assert.Contains(t, body, "error")
}

// TestLSPHandler_LSPCapabilities_Enabled tests basic LSP capabilities structure
func TestLSPHandler_LSPCapabilities_Enabled(t *testing.T) {
	// Create config with LSP enabled
	cfg := &config.LSPConfig{
		Enabled: true,
	}

	// Create handler
	handler := &LSPHandler{
		config: cfg,
	}

	// Create Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/lsp/capabilities", nil)

	// Execute
	handler.LSPCapabilities(c)

	// Verify
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response body directly since c.BindJSON doesn't work after c.JSON
	body := w.Body.String()
	assert.Contains(t, body, "textDocumentSync")
	assert.Contains(t, body, "completionProvider")
	assert.Contains(t, body, "hoverProvider")
	assert.Contains(t, body, "definitionProvider")
	assert.Contains(t, body, "openClose")
	assert.Contains(t, body, "change")
	assert.Contains(t, body, "resolveProvider")
	assert.Contains(t, body, "triggerCharacters")
	assert.Contains(t, body, ".")
	assert.Contains(t, body, "(")
	assert.Contains(t, body, "[")
}

// TestNewLSPHandler tests handler creation
func TestNewLSPHandler(t *testing.T) {
	cfg := &config.LSPConfig{
		Enabled: true,
	}

	// Since we can't easily create a ProviderRegistry, we'll test with nil
	handler := NewLSPHandler(nil, cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
	assert.Nil(t, handler.providerRegistry)
}

// TestLSPHandler_GetLSPClient tests getting LSP client
func TestLSPHandler_GetLSPClient(t *testing.T) {
	handler := &LSPHandler{}

	// Initially should be nil
	assert.Nil(t, handler.GetLSPClient())

	// Test that we can set it (though InitializeLSP would normally do this)
	// This is just to verify the getter works
	handler.lspClient = nil // Keeping it nil for test
	assert.Nil(t, handler.GetLSPClient())
}
