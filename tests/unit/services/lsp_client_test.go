package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/superagent/superagent/internal/models"
	"github.com/superagent/superagent/internal/services"
)

func TestLSPClient_NewLSPClient(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	assert.NotNil(t, client)
}

func TestLSPClient_StartServer_UnsupportedLanguage(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "unsupported-language")

	ctx := context.Background()
	err := client.StartServer(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no LSP server configured for language")
}

func TestLSPClient_GetDiagnostics(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	// Test with no diagnostics
	diagnostics := client.GetDiagnostics("/test/file.go")
	assert.Empty(t, diagnostics)
}

func TestLSPClient_GetCodeIntelligence_ServerNotInitialized(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	ctx := context.Background()
	filePath := "/test/file.go"

	intelligence, err := client.GetCodeIntelligence(ctx, filePath, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
	assert.Nil(t, intelligence)
}

func TestLSPClient_GetWorkspaceSymbols_ServerNotInitialized(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	ctx := context.Background()
	symbols, err := client.GetWorkspaceSymbols(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
	assert.Nil(t, symbols)
}

func TestLSPClient_GetReferences_ServerNotInitialized(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	ctx := context.Background()
	position := models.Position{Line: 1, Character: 0}

	references, err := client.GetReferences(ctx, "/test/file.go", position, true)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
	assert.Nil(t, references)
}

func TestLSPClient_RenameSymbol_ServerNotInitialized(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	ctx := context.Background()
	position := models.Position{Line: 1, Character: 0}

	edit, err := client.RenameSymbol(ctx, "/test/file.go", position, "newName")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
	assert.Nil(t, edit)
}

func TestLSPClient_Shutdown_NoServer(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	ctx := context.Background()
	err := client.Shutdown(ctx)

	assert.NoError(t, err)
}

func TestLSPClient_HealthCheck_ServerNotInitialized(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	err := client.HealthCheck()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
}

func TestLSPClient_LSPMessageTypes(t *testing.T) {
	// Test LSPMessage type
	message := services.LSPMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "testMethod",
		Params:  map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, "2.0", message.JSONRPC)
	assert.Equal(t, 1, message.ID)
	assert.Equal(t, "testMethod", message.Method)
	assert.Equal(t, "value", message.Params.(map[string]interface{})["key"])

	// Test LSPError type
	lspError := services.LSPError{
		Code:    1,
		Message: "Test error",
		Data:    "Additional data",
	}

	assert.Equal(t, 1, lspError.Code)
	assert.Equal(t, "Test error", lspError.Message)
	assert.Equal(t, "Additional data", lspError.Data)

	// Test LSPRange type
	lspRange := services.LSPRange{
		Start: models.Position{Line: 1, Character: 2},
		End:   models.Position{Line: 3, Character: 4},
	}

	assert.Equal(t, 1, lspRange.Start.Line)
	assert.Equal(t, 2, lspRange.Start.Character)
	assert.Equal(t, 3, lspRange.End.Line)
	assert.Equal(t, 4, lspRange.End.Character)

	// Test LSPTextDocument type
	textDoc := services.LSPTextDocument{
		URI: "file:///test/file.go",
	}

	assert.Equal(t, "file:///test/file.go", textDoc.URI)

	// Test LSPTextDocumentPosition type
	textDocPos := services.LSPTextDocumentPosition{
		TextDocument: textDoc,
		Position:     models.Position{Line: 1, Character: 2},
	}

	assert.Equal(t, "file:///test/file.go", textDocPos.TextDocument.URI)
	assert.Equal(t, 1, textDocPos.Position.Line)
	assert.Equal(t, 2, textDocPos.Position.Character)

	// Test LSPTextDocumentContentChangeEvent type
	changeEvent := services.LSPTextDocumentContentChangeEvent{
		Range: &lspRange,
		Text:  "new text",
	}

	assert.Equal(t, &lspRange, changeEvent.Range)
	assert.Equal(t, "new text", changeEvent.Text)
}

func TestLSPClient_LSPServerType(t *testing.T) {
	server := services.LSPServer{
		Capabilities: map[string]interface{}{
			"test": "capability",
		},
		Initialized: true,
		LastHealth:  time.Now(),
	}

	assert.NotNil(t, server.Capabilities)
	assert.Equal(t, "capability", server.Capabilities["test"])
	assert.True(t, server.Initialized)
	assert.NotZero(t, server.LastHealth)
}

func TestLSPClient_StartServer_AlreadyRunning(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	// Mock server as already running
	client.Server = &services.LSPServer{}

	ctx := context.Background()
	err := client.StartServer(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server already running")
}

func TestLSPClient_StartServer_UnsupportedLanguage(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "unsupported-language")

	ctx := context.Background()
	err := client.StartServer(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no LSP server configured for language")
}

func TestLSPClient_GetDiagnostics(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	// Test with no diagnostics
	diagnostics := client.GetDiagnostics("/test/file.go")
	assert.Empty(t, diagnostics)

	// Test with cached diagnostics
	client.Diagnostics["file:///test/file.go"] = []*models.Diagnostic{
		{
			Range: models.Range{
				Start: models.Position{Line: 1, Character: 0},
				End:   models.Position{Line: 1, Character: 10},
			},
			Severity: 1,
			Code:     "test-code",
			Message:  "Test diagnostic",
			Source:   "test-source",
		},
	}

	diagnostics = client.GetDiagnostics("/test/file.go")
	assert.Len(t, diagnostics, 1)
	assert.Equal(t, "Test diagnostic", diagnostics[0].Message)
}

func TestLSPClient_GetCodeIntelligence_ServerNotInitialized(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	ctx := context.Background()
	filePath := "/test/file.go"

	intelligence, err := client.GetCodeIntelligence(ctx, filePath, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
	assert.Nil(t, intelligence)
}

func TestLSPClient_GetWorkspaceSymbols_ServerNotInitialized(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	ctx := context.Background()
	symbols, err := client.GetWorkspaceSymbols(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
	assert.Nil(t, symbols)
}

func TestLSPClient_GetReferences_ServerNotInitialized(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	ctx := context.Background()
	position := models.Position{Line: 1, Character: 0}

	references, err := client.GetReferences(ctx, "/test/file.go", position, true)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
	assert.Nil(t, references)
}

func TestLSPClient_RenameSymbol_ServerNotInitialized(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	ctx := context.Background()
	position := models.Position{Line: 1, Character: 0}

	edit, err := client.RenameSymbol(ctx, "/test/file.go", position, "newName")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
	assert.Nil(t, edit)
}

func TestLSPClient_Shutdown_NoServer(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	ctx := context.Background()
	err := client.Shutdown(ctx)

	assert.NoError(t, err)
}

func TestLSPClient_HealthCheck_ServerNotInitialized(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	err := client.HealthCheck()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
}

func TestLSPClient_ParsePosition(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	posData := map[string]interface{}{
		"line":      float64(10),
		"character": float64(20),
	}

	position, err := client.ParsePosition(posData)

	assert.NoError(t, err)
	assert.Equal(t, 10, position.Line)
	assert.Equal(t, 20, position.Character)
}

func TestLSPClient_ParseRange(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	rangeData := map[string]interface{}{
		"start": map[string]interface{}{
			"line":      float64(1),
			"character": float64(2),
		},
		"end": map[string]interface{}{
			"line":      float64(3),
			"character": float64(4),
		},
	}

	rng, err := client.ParseRange(rangeData)

	assert.NoError(t, err)
	assert.Equal(t, 1, rng.Start.Line)
	assert.Equal(t, 2, rng.Start.Character)
	assert.Equal(t, 3, rng.End.Line)
	assert.Equal(t, 4, rng.End.Character)
}

func TestLSPClient_ParseSingleLocation(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	locationData := map[string]interface{}{
		"uri": "file:///test/file.go",
		"range": map[string]interface{}{
			"start": map[string]interface{}{
				"line":      float64(1),
				"character": float64(2),
			},
			"end": map[string]interface{}{
				"line":      float64(3),
				"character": float64(4),
			},
		},
	}

	location, err := client.ParseSingleLocation(locationData)

	assert.NoError(t, err)
	assert.Equal(t, "file:///test/file.go", location.URI)
	assert.Equal(t, 1, location.Range.Start.Line)
	assert.Equal(t, 2, location.Range.Start.Character)
	assert.Equal(t, 3, location.Range.End.Line)
	assert.Equal(t, 4, location.Range.End.Character)
}

func TestLSPClient_ParseSingleSymbol(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	symbolData := map[string]interface{}{
		"name":          "TestSymbol",
		"kind":          float64(1),
		"containerName": "TestContainer",
		"location": map[string]interface{}{
			"uri": "file:///test/file.go",
			"range": map[string]interface{}{
				"start": map[string]interface{}{
					"line":      float64(1),
					"character": float64(2),
				},
				"end": map[string]interface{}{
					"line":      float64(3),
					"character": float64(4),
				},
			},
		},
	}

	symbol, err := client.ParseSingleSymbol(symbolData)

	assert.NoError(t, err)
	assert.Equal(t, "TestSymbol", symbol.Name)
	assert.Equal(t, 1, symbol.Kind)
	assert.Equal(t, "TestContainer", symbol.ContainerName)
	assert.Equal(t, "file:///test/file.go", symbol.Location.URI)
}

func TestLSPClient_GetClientCapabilities(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	capabilities := client.GetClientCapabilities()

	assert.NotNil(t, capabilities)
	assert.Contains(t, capabilities, "textDocument")
	assert.Contains(t, capabilities, "workspace")

	textDoc, ok := capabilities["textDocument"].(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, textDoc, "completion")
	assert.Contains(t, textDoc, "hover")
	assert.Contains(t, textDoc, "definition")
	assert.Contains(t, textDoc, "references")
	assert.Contains(t, textDoc, "documentSymbol")
	assert.Contains(t, textDoc, "semanticTokens")
}

func TestLSPClient_NextMessageID(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	// Initial ID should be 1
	id1 := client.NextMessageID()
	assert.Equal(t, 1, id1)

	// Next ID should increment
	id2 := client.NextMessageID()
	assert.Equal(t, 2, id2)

	id3 := client.NextMessageID()
	assert.Equal(t, 3, id3)
}

func TestLSPClient_SupportsCapability(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	server := &services.LSPServer{
		Capabilities: map[string]interface{}{
			"completionProvider": true,
			"hoverProvider":      true,
		},
	}

	assert.True(t, client.SupportsCapability(server, "completionProvider"))
	assert.True(t, client.SupportsCapability(server, "hoverProvider"))
	assert.False(t, client.SupportsCapability(server, "definitionProvider"))
}

func TestLSPClient_ReadFileContent(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	// Test with non-existent file
	content, err := client.ReadFileContent("/non/existent/file.go")
	assert.Error(t, err)
	assert.Empty(t, content)

	// Note: We can't test with real files in unit tests
	// This is just testing the function signature
}

func TestLSPClient_LSPMessageTypes(t *testing.T) {
	// Test LSPMessage type
	message := services.LSPMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "testMethod",
		Params:  map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, "2.0", message.JSONRPC)
	assert.Equal(t, 1, message.ID)
	assert.Equal(t, "testMethod", message.Method)
	assert.Equal(t, "value", message.Params.(map[string]interface{})["key"])

	// Test LSPError type
	lspError := services.LSPError{
		Code:    1,
		Message: "Test error",
		Data:    "Additional data",
	}

	assert.Equal(t, 1, lspError.Code)
	assert.Equal(t, "Test error", lspError.Message)
	assert.Equal(t, "Additional data", lspError.Data)

	// Test LSPRange type
	lspRange := services.LSPRange{
		Start: models.Position{Line: 1, Character: 2},
		End:   models.Position{Line: 3, Character: 4},
	}

	assert.Equal(t, 1, lspRange.Start.Line)
	assert.Equal(t, 2, lspRange.Start.Character)
	assert.Equal(t, 3, lspRange.End.Line)
	assert.Equal(t, 4, lspRange.End.Character)

	// Test LSPTextDocument type
	textDoc := services.LSPTextDocument{
		URI: "file:///test/file.go",
	}

	assert.Equal(t, "file:///test/file.go", textDoc.URI)

	// Test LSPTextDocumentPosition type
	textDocPos := services.LSPTextDocumentPosition{
		TextDocument: textDoc,
		Position:     models.Position{Line: 1, Character: 2},
	}

	assert.Equal(t, "file:///test/file.go", textDocPos.TextDocument.URI)
	assert.Equal(t, 1, textDocPos.Position.Line)
	assert.Equal(t, 2, textDocPos.Position.Character)

	// Test LSPTextDocumentContentChangeEvent type
	changeEvent := services.LSPTextDocumentContentChangeEvent{
		Range: &lspRange,
		Text:  "new text",
	}

	assert.Equal(t, &lspRange, changeEvent.Range)
	assert.Equal(t, "new text", changeEvent.Text)
}

func TestLSPClient_LSPServerType(t *testing.T) {
	server := services.LSPServer{
		Capabilities: map[string]interface{}{
			"test": "capability",
		},
		Initialized: true,
		LastHealth:  time.Now(),
	}

	assert.NotNil(t, server.Capabilities)
	assert.Equal(t, "capability", server.Capabilities["test"])
	assert.True(t, server.Initialized)
	assert.NotZero(t, server.LastHealth)
}
