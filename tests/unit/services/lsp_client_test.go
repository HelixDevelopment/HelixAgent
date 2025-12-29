package services_test

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/services"
)

func TestLSPClient_Basic(t *testing.T) {
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

func TestLSPClient_GetDiagnostics_Empty(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")
	diagnostics := client.GetDiagnostics("/test/file.go")
	assert.Empty(t, diagnostics)
}

func TestLSPClient_GetCodeIntelligence_NoServer(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	ctx := context.Background()
	intelligence, err := client.GetCodeIntelligence(ctx, "/test/file.go", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
	assert.Nil(t, intelligence)
}

func TestLSPClient_GetWorkspaceSymbols_NoServer(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	ctx := context.Background()
	symbols, err := client.GetWorkspaceSymbols(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
	assert.Nil(t, symbols)
}

func TestLSPClient_GetReferences_NoServer(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	ctx := context.Background()
	position := models.Position{Line: 1, Character: 0}
	references, err := client.GetReferences(ctx, "/test/file.go", position, true)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
	assert.Nil(t, references)
}

func TestLSPClient_RenameSymbol_NoServer(t *testing.T) {
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

func TestLSPClient_HealthCheck_NoServer(t *testing.T) {
	client := services.NewLSPClient("/test/workspace", "go")

	err := client.HealthCheck()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LSP server not initialized")
}

func TestLSPClient_TypeDefinitions(t *testing.T) {
	// Test LSPMessage
	message := services.LSPMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
	}
	assert.Equal(t, "2.0", message.JSONRPC)
	assert.Equal(t, 1, message.ID)
	assert.Equal(t, "test", message.Method)

	// Test LSPError
	lspError := services.LSPError{
		Code:    1,
		Message: "error",
	}
	assert.Equal(t, 1, lspError.Code)
	assert.Equal(t, "error", lspError.Message)

	// Test LSPRange
	lspRange := services.LSPRange{
		Start: models.Position{Line: 1, Character: 2},
		End:   models.Position{Line: 3, Character: 4},
	}
	assert.Equal(t, 1, lspRange.Start.Line)
	assert.Equal(t, 2, lspRange.Start.Character)
	assert.Equal(t, 3, lspRange.End.Line)
	assert.Equal(t, 4, lspRange.End.Character)

	// Test LSPTextDocument
	textDoc := services.LSPTextDocument{
		URI: "file:///test.go",
	}
	assert.Equal(t, "file:///test.go", textDoc.URI)

	// Test LSPTextDocumentPosition
	textDocPos := services.LSPTextDocumentPosition{
		TextDocument: textDoc,
		Position:     models.Position{Line: 5, Character: 6},
	}
	assert.Equal(t, "file:///test.go", textDocPos.TextDocument.URI)
	assert.Equal(t, 5, textDocPos.Position.Line)
	assert.Equal(t, 6, textDocPos.Position.Character)

	// Test LSPTextDocumentContentChangeEvent
	changeEvent := services.LSPTextDocumentContentChangeEvent{
		Range: &lspRange,
		Text:  "new text",
	}
	assert.Equal(t, &lspRange, changeEvent.Range)
	assert.Equal(t, "new text", changeEvent.Text)
}

func TestLSPServerConnection_Type(t *testing.T) {
	server := services.LSPServerConnection{
		Capabilities: map[string]interface{}{
			"test": "value",
		},
		Initialized: true,
		LastHealth:  time.Now(),
	}

	assert.NotNil(t, server.Capabilities)
	assert.Equal(t, "value", server.Capabilities["test"])
	assert.True(t, server.Initialized)
	assert.NotZero(t, server.LastHealth)
}
