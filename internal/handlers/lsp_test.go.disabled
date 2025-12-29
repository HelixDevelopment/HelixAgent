package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/models"
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

// TestLSPHandler_LSPCompletion_Disabled tests LSP completion when disabled
func TestLSPHandler_LSPCompletion_Disabled(t *testing.T) {
	cfg := &config.LSPConfig{
		Enabled: false,
	}

	handler := &LSPHandler{
		config: cfg,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/lsp/completion", nil)

	handler.LSPCompletion(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "LSP is not enabled")
}

// TestLSPHandler_LSPCompletion_InvalidRequest tests LSP completion with invalid request
func TestLSPHandler_LSPCompletion_InvalidRequest(t *testing.T) {
	cfg := &config.LSPConfig{
		Enabled: true,
	}

	handler := &LSPHandler{
		config: cfg,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/lsp/completion", nil)

	handler.LSPCompletion(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Invalid request")
}

// TestLSPHandler_LSPHover_Disabled tests LSP hover when disabled
func TestLSPHandler_LSPHover_Disabled(t *testing.T) {
	cfg := &config.LSPConfig{
		Enabled: false,
	}

	handler := &LSPHandler{
		config: cfg,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/lsp/hover", nil)

	handler.LSPHover(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "LSP is not enabled")
}

// TestLSPHandler_LSPHover_InvalidRequest tests LSP hover with invalid request
func TestLSPHandler_LSPHover_InvalidRequest(t *testing.T) {
	cfg := &config.LSPConfig{
		Enabled: true,
	}

	handler := &LSPHandler{
		config: cfg,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/lsp/hover", nil)

	handler.LSPHover(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Invalid request")
}

// TestLSPHandler_LSPCodeActions_Disabled tests LSP code actions when disabled
func TestLSPHandler_LSPCodeActions_Disabled(t *testing.T) {
	cfg := &config.LSPConfig{
		Enabled: false,
	}

	handler := &LSPHandler{
		config: cfg,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/lsp/codeActions", nil)

	handler.LSPCodeActions(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "LSP is not enabled")
}

// TestLSPHandler_LSPCodeActions_InvalidRequest tests LSP code actions with invalid request
func TestLSPHandler_LSPCodeActions_InvalidRequest(t *testing.T) {
	cfg := &config.LSPConfig{
		Enabled: true,
	}

	handler := &LSPHandler{
		config: cfg,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/lsp/codeActions", nil)

	handler.LSPCodeActions(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Invalid request")
}

// TestBuildCompletionPrompt tests completion prompt building
func TestBuildCompletionPrompt(t *testing.T) {
	handler := &LSPHandler{}

	lines := []string{
		"package main",
		"",
		"func main() {",
		"    fmt.Println(\"Hello, World\")",
		"}",
	}

	prompt := handler.buildCompletionPrompt(lines, 3, 4)

	assert.Contains(t, string(prompt), "Complete the code at position line 3, character 4")
	// The function shows lines 1-5 (line 3 ± 2)
	assert.Contains(t, string(prompt), "1: ")
	assert.Contains(t, string(prompt), "2: func main() {")
	assert.Contains(t, string(prompt), "3:     fmt.Println(\"Hello, World\")")
	assert.Contains(t, string(prompt), "4: }")
	assert.Contains(t, string(prompt), "Provide 3-5 concise completion options")
}

// TestBuildCompletionPrompt_EdgeCases tests completion prompt building with edge cases
func TestBuildCompletionPrompt_EdgeCases(t *testing.T) {
	handler := &LSPHandler{}

	// Test with empty lines
	prompt1 := handler.buildCompletionPrompt([]string{}, 0, 0)
	assert.Contains(t, string(prompt1), "Complete the code at position line 0, character 0")

	// Test with single line
	lines2 := []string{"single line"}
	prompt2 := handler.buildCompletionPrompt(lines2, 0, 5)
	assert.Contains(t, string(prompt2), "0: single line")

	// Test with line out of bounds
	lines3 := []string{"line1", "line2"}
	prompt3 := handler.buildCompletionPrompt(lines3, 10, 5)
	assert.Contains(t, string(prompt3), "Complete the code at position line 10, character 5")
}

// TestBuildHoverPrompt tests hover prompt building
func TestBuildHoverPrompt(t *testing.T) {
	handler := &LSPHandler{}

	lines := []string{
		"package main",
		"import \"fmt\"",
		"func main() {",
		"    message := \"Hello\"",
		"    fmt.Println(message)",
		"}",
	}

	prompt := handler.buildHoverPrompt(lines, 3, 12)

	assert.Contains(t, string(prompt), "Explain the code element at line 3, character 12")
	assert.Contains(t, string(prompt), "message := \"Hello\"")
	assert.Contains(t, string(prompt), "Provide a brief, clear explanation")
}

// TestBuildHoverPrompt_EdgeCases tests hover prompt building with edge cases
func TestBuildHoverPrompt_EdgeCases(t *testing.T) {
	handler := &LSPHandler{}

	// Test with empty lines
	prompt1 := handler.buildHoverPrompt([]string{}, 0, 0)
	assert.Contains(t, string(prompt1), "Explain the code element at the specified position")

	// Test with line out of bounds
	lines2 := []string{"line1"}
	prompt2 := handler.buildHoverPrompt(lines2, 5, 2)
	assert.Contains(t, string(prompt2), "Explain the code element at the specified position")
}

// TestBuildCodeActionPrompt tests code action prompt building
func TestBuildCodeActionPrompt(t *testing.T) {
	handler := &LSPHandler{}

	lines := []string{
		"package main",
		"",
		"func calculate(x int, y int) int {",
		"    result := x + y",
		"    return result",
		"}",
	}

	var range_ struct {
		Start struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"start"`
		End struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"end"`
	}
	range_.Start.Line = 2
	range_.Start.Character = 0
	range_.End.Line = 5
	range_.End.Character = 1

	prompt := handler.buildCodeActionPrompt(lines, range_)

	assert.Contains(t, string(prompt), "Suggest code improvements for this range (lines 2-5)")
	assert.Contains(t, string(prompt), "func calculate(x int, y int) int {")
	assert.Contains(t, string(prompt), "    result := x + y")
	assert.Contains(t, string(prompt), "    return result")
	assert.Contains(t, string(prompt), "}")
	assert.Contains(t, string(prompt), "Provide 3-5 specific code actions")
}

// TestConvertToLSPCapabilities tests LSP capabilities conversion
func TestConvertToLSPCapabilities(t *testing.T) {
	handler := &LSPHandler{}

	// Test with minimal capabilities
	caps := &models.ProviderCapabilities{
		SupportsCodeCompletion: true,
		SupportsCodeAnalysis:   false,
		SupportsRefactoring:    false,
		SupportsTools:          false,
		SupportsSearch:         false,
		SupportsReasoning:      false,
	}

	lspCaps := handler.convertToLSPCapabilities("test-provider", caps)

	assert.NotNil(t, lspCaps)
	assert.Contains(t, lspCaps, "textDocumentSync")
	assert.Contains(t, lspCaps, "completionProvider")
	assert.NotContains(t, lspCaps, "hoverProvider")
	assert.NotContains(t, lspCaps, "definitionProvider")

	// Test with full capabilities
	fullCaps := &models.ProviderCapabilities{
		SupportsCodeCompletion: true,
		SupportsCodeAnalysis:   true,
		SupportsRefactoring:    true,
		SupportsTools:          true,
		SupportsSearch:         true,
		SupportsReasoning:      true,
	}

	fullLspCaps := handler.convertToLSPCapabilities("full-provider", fullCaps)

	assert.Contains(t, fullLspCaps, "hoverProvider")
	assert.Contains(t, fullLspCaps, "definitionProvider")
	assert.Contains(t, fullLspCaps, "codeActionProvider")
	assert.Contains(t, fullLspCaps, "renameProvider")
}

// TestConvertToCompletionItems tests completion items conversion
func TestConvertToCompletionItems(t *testing.T) {
	handler := &LSPHandler{}

	content := "Example completion content"
	completionItems := handler.convertToCompletionItems(content, 1, 5)

	assert.NotNil(t, completionItems)
	assert.Len(t, completionItems, 1)

	item, ok := completionItems[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Example completion", item["label"])
	assert.Equal(t, 1, item["kind"]) // Text
	assert.Equal(t, content, item["insertText"])
	assert.Equal(t, "Suggested by SuperAgent ensemble", item["detail"])

	doc, ok := item["documentation"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "markdown", doc["kind"])
	assert.Equal(t, "Code completion generated using multiple LLM providers", doc["value"])
}

// TestConvertToCodeActions tests code actions conversion
func TestConvertToCodeActions(t *testing.T) {
	handler := &LSPHandler{}

	content := "Suggested refactoring code"
	var range_ interface{} = map[string]interface{}{
		"start": map[string]interface{}{"line": 1, "character": 0},
		"end":   map[string]interface{}{"line": 2, "character": 10},
	}

	codeActions := handler.convertToCodeActions(content, range_)

	assert.NotNil(t, codeActions)
	actions, ok := codeActions.([]interface{})
	assert.True(t, ok)
	assert.Len(t, actions, 1)

	action, ok := actions[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Suggested refactoring", action["title"])
	assert.Equal(t, "refactor", action["kind"])

	edit, ok := action["edit"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, range_, edit["range"])
	assert.Equal(t, content, edit["newText"])
}
