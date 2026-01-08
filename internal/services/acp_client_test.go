package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newACPTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

// MockACPTransport implements ACPTransport for testing
type MockACPTransport struct {
	connected    bool
	sendFunc     func(ctx context.Context, message interface{}) error
	receiveFunc  func(ctx context.Context) (interface{}, error)
	closeFunc    func() error
	sendCalls    []interface{}
	receiveCalls int
}

func NewMockACPTransport() *MockACPTransport {
	return &MockACPTransport{
		connected: true,
		sendCalls: make([]interface{}, 0),
	}
}

func (m *MockACPTransport) Send(ctx context.Context, message interface{}) error {
	m.sendCalls = append(m.sendCalls, message)
	if m.sendFunc != nil {
		return m.sendFunc(ctx, message)
	}
	return nil
}

func (m *MockACPTransport) Receive(ctx context.Context) (interface{}, error) {
	m.receiveCalls++
	if m.receiveFunc != nil {
		return m.receiveFunc(ctx)
	}
	// Return a mock initialize response
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]interface{}{
			"protocolVersion": "1.0.0",
			"capabilities":    map[string]interface{}{},
			"serverInfo":      map[string]string{"name": "mock-server"},
		},
	}, nil
}

func (m *MockACPTransport) Close() error {
	m.connected = false
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *MockACPTransport) IsConnected() bool {
	return m.connected
}

func TestNewACPDiscoveryClient(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	require.NotNil(t, client)
	assert.NotNil(t, client.agents)
	assert.Equal(t, 1, client.messageID)
}

func TestACPDiscoveryClient_ListAgents(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	t.Run("empty agents list", func(t *testing.T) {
		agents := client.ListAgents()
		assert.Empty(t, agents)
	})
}

func TestACPDiscoveryClient_HealthCheck(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	t.Run("empty health check", func(t *testing.T) {
		results := client.HealthCheck(context.Background())
		assert.Empty(t, results)
	})
}

func TestACPDiscoveryClient_GetAgentCapabilities_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	caps, err := client.GetAgentCapabilities("non-existent")
	assert.Error(t, err)
	assert.Nil(t, caps)
	assert.Contains(t, err.Error(), "not connected")
}

func TestACPDiscoveryClient_DisconnectAgent_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	err := client.DisconnectAgent("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestACPDiscoveryClient_ExecuteAction_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	result, err := client.ExecuteAction(context.Background(), "non-existent", "test", nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not connected")
}

func TestACPDiscoveryClient_GetAgentStatus_NotFound(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	status, err := client.GetAgentStatus(context.Background(), "non-existent")
	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "not found")
}

func TestACPDiscoveryClient_BroadcastAction_Empty(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	results := client.BroadcastAction(context.Background(), "test", nil)
	assert.Empty(t, results)
}

func TestACPDiscoveryClient_ConnectAgent_InvalidProtocol(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	err := client.ConnectAgent(context.Background(), "agent1", "Test Agent", "invalid://endpoint")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported endpoint protocol")
}

func TestACPAgentConnection_Structure(t *testing.T) {
	now := time.Now()
	connection := &ACPAgentConnection{
		ID:           "agent-123",
		Name:         "Test Agent",
		Transport:    nil,
		Capabilities: map[string]interface{}{"streaming": true},
		Connected:    true,
		LastUsed:     now,
	}

	assert.Equal(t, "agent-123", connection.ID)
	assert.Equal(t, "Test Agent", connection.Name)
	assert.True(t, connection.Connected)
	assert.Equal(t, true, connection.Capabilities["streaming"])
}

func TestACPMessage_Structure(t *testing.T) {
	message := &ACPMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  map[string]interface{}{"key": "value"},
		Result:  nil,
		Error:   nil,
	}

	assert.Equal(t, "2.0", message.JSONRPC)
	assert.Equal(t, 1, message.ID)
	assert.Equal(t, "initialize", message.Method)
}

func TestACPError_Structure(t *testing.T) {
	acpError := &ACPError{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    map[string]interface{}{"details": "error details"},
	}

	assert.Equal(t, -32600, acpError.Code)
	assert.Equal(t, "Invalid Request", acpError.Message)
}

func TestACPInitializeRequest_Structure(t *testing.T) {
	request := &ACPInitializeRequest{
		ProtocolVersion: "1.0.0",
		Capabilities:    map[string]interface{}{"streaming": true},
		ClientInfo: map[string]string{
			"name":    "test-client",
			"version": "1.0.0",
		},
	}

	assert.Equal(t, "1.0.0", request.ProtocolVersion)
	assert.Equal(t, true, request.Capabilities["streaming"])
	assert.Equal(t, "test-client", request.ClientInfo["name"])
}

func TestACPInitializeResult_Structure(t *testing.T) {
	result := &ACPInitializeResult{
		ProtocolVersion: "1.0.0",
		Capabilities:    map[string]interface{}{"tools": true},
		ServerInfo: map[string]string{
			"name":    "test-server",
			"version": "1.0.0",
		},
		Instructions: "Use tools for...",
	}

	assert.Equal(t, "1.0.0", result.ProtocolVersion)
	assert.Equal(t, "Use tools for...", result.Instructions)
}

func TestACPActionRequest_Structure(t *testing.T) {
	request := &ACPActionRequest{
		Action: "execute_tool",
		Params: map[string]interface{}{"tool": "calculator"},
		Context: map[string]interface{}{
			"session": "session-123",
		},
	}

	assert.Equal(t, "execute_tool", request.Action)
	assert.Equal(t, "calculator", request.Params["tool"])
}

func TestACPActionResult_Structure(t *testing.T) {
	result := &ACPActionResult{
		Success: true,
		Result:  map[string]interface{}{"output": "result"},
		Error:   "",
	}

	assert.True(t, result.Success)
	assert.Empty(t, result.Error)
}

func TestWebSocketACPTransport_Close(t *testing.T) {
	transport := &WebSocketACPTransport{
		conn:      nil,
		connected: true,
	}

	err := transport.Close()
	assert.NoError(t, err)
	assert.False(t, transport.connected)
}

func TestWebSocketACPTransport_IsConnected(t *testing.T) {
	t.Run("not connected", func(t *testing.T) {
		transport := &WebSocketACPTransport{
			conn:      nil,
			connected: false,
		}

		assert.False(t, transport.IsConnected())
	})
}

func TestHTTPACPTransport_Close(t *testing.T) {
	transport := &HTTPACPTransport{
		baseURL:   "http://localhost:7061",
		connected: true,
	}

	err := transport.Close()
	assert.NoError(t, err)
	assert.False(t, transport.connected)
}

func TestHTTPACPTransport_Send_NotConnected(t *testing.T) {
	transport := &HTTPACPTransport{
		baseURL:   "http://localhost:7061",
		connected: false,
	}

	err := transport.Send(context.Background(), map[string]interface{}{"test": "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestHTTPACPTransport_Receive(t *testing.T) {
	transport := &HTTPACPTransport{
		baseURL:   "http://localhost:7061",
		connected: true,
	}

	// HTTP transport doesn't support receive
	result, err := transport.Receive(context.Background())
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "does not support receive")
}

func TestWebSocketACPTransport_Send_NotConnected(t *testing.T) {
	transport := &WebSocketACPTransport{
		conn:      nil,
		connected: false,
	}

	err := transport.Send(context.Background(), map[string]interface{}{"test": "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestWebSocketACPTransport_Receive_NotConnected(t *testing.T) {
	transport := &WebSocketACPTransport{
		conn:      nil,
		connected: false,
	}

	result, err := transport.Receive(context.Background())
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not connected")
}

func BenchmarkACPClient_ListAgents(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	client := NewACPDiscoveryClient(log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.ListAgents()
	}
}

func BenchmarkACPClient_HealthCheck(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	client := NewACPDiscoveryClient(log)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.HealthCheck(ctx)
	}
}

// LSPClient Tests

func TestNewLSPClient(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	require.NotNil(t, client)
	assert.NotNil(t, client.servers)
	assert.NotNil(t, client.capabilities)
	assert.Equal(t, 1, client.messageID)
	assert.NotNil(t, client.logger)
}

func TestLSPClient_ListServers(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	servers := client.ListServers()
	assert.Len(t, servers, 0)
}

func TestLSPClient_GetServerCapabilities_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	caps, err := client.GetServerCapabilities("non-existent")
	assert.Error(t, err)
	assert.Nil(t, caps)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLSPClient_DisconnectServer_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	err := client.DisconnectServer("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLSPClient_OpenFile_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)
	ctx := context.Background()

	err := client.OpenFile(ctx, "non-existent", "file:///test.go", "go", "package main")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLSPClient_UpdateFile_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)
	ctx := context.Background()

	err := client.UpdateFile(ctx, "non-existent", "file:///test.go", "package main")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLSPClient_CloseFile_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)
	ctx := context.Background()

	err := client.CloseFile(ctx, "non-existent", "file:///test.go")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLSPClient_CloseFile_FileNotOpened(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Connected: true,
		Files:     make(map[string]*LSPFileInfo), // Empty files map
	}
	client.mu.Unlock()

	ctx := context.Background()
	err := client.CloseFile(ctx, "test-server", "file:///not-opened.go")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not opened")
}

func TestLSPClient_CloseFile_SendError(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.sendFunc = func(ctx context.Context, message interface{}) error {
		return errors.New("send failed")
	}

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Connected: true,
		Files: map[string]*LSPFileInfo{
			"file:///test.go": {URI: "file:///test.go"},
		},
	}
	client.mu.Unlock()

	ctx := context.Background()
	err := client.CloseFile(ctx, "test-server", "file:///test.go")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send didClose")
}

func TestLSPClient_GetCompletion_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)
	ctx := context.Background()

	result, err := client.GetCompletion(ctx, "non-existent", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLSPClient_GetHover_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)
	ctx := context.Background()

	result, err := client.GetHover(ctx, "non-existent", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLSPClient_GetDefinition_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)
	ctx := context.Background()

	result, err := client.GetDefinition(ctx, "non-existent", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLSPClient_HealthCheck(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)
	ctx := context.Background()

	results := client.HealthCheck(ctx)
	assert.NotNil(t, results)
	assert.Len(t, results, 0)
}

func TestLSPClient_GetDiagnostics(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)
	ctx := context.Background()

	// GetDiagnostics returns empty slice, doesn't require connection
	result, err := client.GetDiagnostics(ctx, "/test/file.go")
	assert.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestLSPClient_HandlePublishDiagnostics(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	t.Run("StoresDiagnostics", func(t *testing.T) {
		params := &ACPPublishDiagnosticsParams{
			URI: "file:///test/file.go",
			Diagnostics: []*ACPDiagnostic{
				{
					Range: Range{
						Start: Position{Line: 10, Character: 5},
						End:   Position{Line: 10, Character: 15},
					},
					Severity: 1,
					Code:     "E001",
					Source:   "golangci-lint",
					Message:  "unused variable 'x'",
				},
				{
					Range: Range{
						Start: Position{Line: 20, Character: 0},
						End:   Position{Line: 20, Character: 10},
					},
					Severity: 2,
					Code:     "W001",
					Source:   "golangci-lint",
					Message:  "deprecated function",
				},
			},
		}

		client.HandlePublishDiagnostics(params)

		ctx := context.Background()
		result, err := client.GetDiagnostics(ctx, "/test/file.go")
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "unused variable 'x'", result[0].Message)
		assert.Equal(t, 1, result[0].Severity)
		assert.Equal(t, "E001", result[0].Code)
	})

	t.Run("NilParams", func(t *testing.T) {
		client.HandlePublishDiagnostics(nil)
		// Should not panic
	})
}

func TestLSPClient_ClearDiagnostics(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)
	ctx := context.Background()

	// Add diagnostics
	params := &ACPPublishDiagnosticsParams{
		URI: "file:///test/file.go",
		Diagnostics: []*ACPDiagnostic{
			{Message: "test error"},
		},
	}
	client.HandlePublishDiagnostics(params)

	// Verify they exist
	result, err := client.GetDiagnostics(ctx, "/test/file.go")
	assert.NoError(t, err)
	assert.Len(t, result, 1)

	// Clear diagnostics
	client.ClearDiagnostics("file:///test/file.go")

	// Verify they're gone
	result, err = client.GetDiagnostics(ctx, "/test/file.go")
	assert.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestLSPClient_ClearAllDiagnostics(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)
	ctx := context.Background()

	// Add diagnostics for multiple files
	params1 := &ACPPublishDiagnosticsParams{
		URI:         "file:///test/file1.go",
		Diagnostics: []*ACPDiagnostic{{Message: "error 1"}},
	}
	params2 := &ACPPublishDiagnosticsParams{
		URI:         "file:///test/file2.go",
		Diagnostics: []*ACPDiagnostic{{Message: "error 2"}},
	}
	client.HandlePublishDiagnostics(params1)
	client.HandlePublishDiagnostics(params2)

	// Verify they exist
	result1, _ := client.GetDiagnostics(ctx, "/test/file1.go")
	result2, _ := client.GetDiagnostics(ctx, "/test/file2.go")
	assert.Len(t, result1, 1)
	assert.Len(t, result2, 1)

	// Clear all diagnostics
	client.ClearAllDiagnostics()

	// Verify they're all gone
	result1, _ = client.GetDiagnostics(ctx, "/test/file1.go")
	result2, _ = client.GetDiagnostics(ctx, "/test/file2.go")
	assert.Len(t, result1, 0)
	assert.Len(t, result2, 0)
}

func TestACPDiagnostic_Structure(t *testing.T) {
	diag := &ACPDiagnostic{
		Range: Range{
			Start: Position{Line: 1, Character: 0},
			End:   Position{Line: 1, Character: 10},
		},
		Severity: 1,
		Code:     "E001",
		Source:   "linter",
		Message:  "error message",
		RelatedInformation: []ACPRelatedDiagnosticInfo{
			{
				Location: Location{URI: "file:///related.go"},
				Message:  "related info",
			},
		},
	}

	assert.Equal(t, 1, diag.Range.Start.Line)
	assert.Equal(t, 1, diag.Severity)
	assert.Equal(t, "E001", diag.Code)
	assert.Equal(t, "linter", diag.Source)
	assert.Equal(t, "error message", diag.Message)
	assert.Len(t, diag.RelatedInformation, 1)
	assert.Equal(t, "related info", diag.RelatedInformation[0].Message)
}

func TestLSPClient_GetCodeIntelligence_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)
	ctx := context.Background()

	result, err := client.GetCodeIntelligence(ctx, "/test/file.go", map[string]interface{}{})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not connected")
}

// LSP Types Structure Tests

func TestLSPServerConnection_Structure(t *testing.T) {
	now := time.Now()
	connection := &LSPServerConnection{
		ID:        "gopls-1",
		Name:      "Go Language Server",
		Language:  "go",
		Transport: nil,
		Capabilities: &LSPCapabilities{
			HoverProvider:      true,
			DefinitionProvider: true,
		},
		Workspace: "/workspace",
		Connected: true,
		LastUsed:  now,
		Files:     make(map[string]*LSPFileInfo),
	}

	assert.Equal(t, "gopls-1", connection.ID)
	assert.Equal(t, "Go Language Server", connection.Name)
	assert.Equal(t, "go", connection.Language)
	assert.True(t, connection.Connected)
	assert.True(t, connection.Capabilities.HoverProvider)
}

func TestLSPMessage_Structure(t *testing.T) {
	message := &LSPMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "textDocument/completion",
		Params:  map[string]interface{}{"uri": "file:///test.go"},
		Result:  nil,
		Error:   nil,
	}

	assert.Equal(t, "2.0", message.JSONRPC)
	assert.Equal(t, 1, message.ID)
	assert.Equal(t, "textDocument/completion", message.Method)
}

func TestLSPError_Structure(t *testing.T) {
	lspError := &LSPError{
		Code:    -32601,
		Message: "Method not found",
		Data:    map[string]interface{}{"details": "error details"},
	}

	assert.Equal(t, -32601, lspError.Code)
	assert.Equal(t, "Method not found", lspError.Message)
}

func TestLSPCapabilities_Structure(t *testing.T) {
	caps := &LSPCapabilities{
		HoverProvider:              true,
		DefinitionProvider:         true,
		TypeDefinitionProvider:     true,
		ReferencesProvider:         true,
		DocumentSymbolProvider:     true,
		CodeActionProvider:         true,
		DocumentFormattingProvider: true,
		RenameProvider:             true,
	}

	assert.True(t, caps.HoverProvider)
	assert.True(t, caps.DefinitionProvider)
	assert.True(t, caps.TypeDefinitionProvider)
	assert.True(t, caps.RenameProvider)
}

func TestLSPFileInfo_Structure(t *testing.T) {
	now := time.Now()
	fileInfo := &LSPFileInfo{
		URI:        "file:///test.go",
		LanguageID: "go",
		Version:    1,
		Content:    "package main",
		LastSync:   now,
	}

	assert.Equal(t, "file:///test.go", fileInfo.URI)
	assert.Equal(t, "go", fileInfo.LanguageID)
	assert.Equal(t, 1, fileInfo.Version)
	assert.Equal(t, "package main", fileInfo.Content)
}

func TestTextDocumentSyncOptions_Structure(t *testing.T) {
	options := &TextDocumentSyncOptions{
		OpenClose: true,
		Change:    2,
	}

	assert.True(t, options.OpenClose)
	assert.Equal(t, 2, options.Change)
}

func TestCompletionOptions_Structure(t *testing.T) {
	options := &CompletionOptions{
		TriggerCharacters: []string{".", ":", "<"},
		ResolveProvider:   true,
	}

	assert.Len(t, options.TriggerCharacters, 3)
	assert.True(t, options.ResolveProvider)
}

func TestSignatureHelpOptions_Structure(t *testing.T) {
	options := &SignatureHelpOptions{
		TriggerCharacters: []string{"(", ","},
	}

	assert.Len(t, options.TriggerCharacters, 2)
}

func TestCodeLensOptions_Structure(t *testing.T) {
	options := &CodeLensOptions{
		ResolveProvider: true,
	}

	assert.True(t, options.ResolveProvider)
}

func TestTextDocumentItem_Structure(t *testing.T) {
	item := TextDocumentItem{
		URI:        "file:///test.go",
		LanguageID: "go",
		Version:    1,
		Text:       "package main",
	}

	assert.Equal(t, "file:///test.go", item.URI)
	assert.Equal(t, "go", item.LanguageID)
	assert.Equal(t, 1, item.Version)
}

func TestVersionedTextDocumentIdentifier_Structure(t *testing.T) {
	identifier := VersionedTextDocumentIdentifier{
		URI:     "file:///test.go",
		Version: 2,
	}

	assert.Equal(t, "file:///test.go", identifier.URI)
	assert.Equal(t, 2, identifier.Version)
}

func TestDidOpenTextDocumentParams_Structure(t *testing.T) {
	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///test.go",
			LanguageID: "go",
			Version:    1,
			Text:       "package main",
		},
	}

	assert.Equal(t, "file:///test.go", params.TextDocument.URI)
}

func TestDidChangeTextDocumentParams_Structure(t *testing.T) {
	params := DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     "file:///test.go",
			Version: 2,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: "package main\n\nfunc main() {}"},
		},
	}

	assert.Equal(t, 2, params.TextDocument.Version)
	assert.Len(t, params.ContentChanges, 1)
}

func TestCompletionParams_Structure(t *testing.T) {
	params := CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///test.go"},
		Position:     Position{Line: 10, Character: 5},
	}

	assert.Equal(t, "file:///test.go", params.TextDocument.URI)
	assert.Equal(t, 10, params.Position.Line)
	assert.Equal(t, 5, params.Position.Character)
}

func TestHoverParams_Structure(t *testing.T) {
	params := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///test.go"},
		Position:     Position{Line: 15, Character: 8},
	}

	assert.Equal(t, "file:///test.go", params.TextDocument.URI)
	assert.Equal(t, 15, params.Position.Line)
}

func TestDefinitionParams_Structure(t *testing.T) {
	params := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///test.go"},
		Position:     Position{Line: 20, Character: 12},
	}

	assert.Equal(t, "file:///test.go", params.TextDocument.URI)
	assert.Equal(t, 20, params.Position.Line)
}

func TestCompletionItem_Structure(t *testing.T) {
	item := CompletionItem{
		Label:         "Println",
		Kind:          3, // Function
		Detail:        "func(a ...interface{}) (n int, err error)",
		Documentation: "Println formats using the default formats...",
	}

	assert.Equal(t, "Println", item.Label)
	assert.Equal(t, 3, item.Kind)
	assert.Equal(t, "Println formats using the default formats...", item.Documentation)
}

func TestHover_Structure(t *testing.T) {
	result := Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: "```go\nfunc Println(a ...interface{}) (n int, err error)\n```",
		},
		Range: &Range{
			Start: Position{Line: 10, Character: 0},
			End:   Position{Line: 10, Character: 7},
		},
	}

	assert.Equal(t, "markdown", result.Contents.Kind)
	assert.Equal(t, 10, result.Range.Start.Line)
}

func TestLocation_Structure(t *testing.T) {
	location := Location{
		URI: "file:///src/main.go",
		Range: Range{
			Start: Position{Line: 5, Character: 0},
			End:   Position{Line: 5, Character: 10},
		},
	}

	assert.Equal(t, "file:///src/main.go", location.URI)
	assert.Equal(t, 5, location.Range.Start.Line)
}

func TestRange_Structure(t *testing.T) {
	r := Range{
		Start: Position{Line: 1, Character: 0},
		End:   Position{Line: 10, Character: 20},
	}

	assert.Equal(t, 1, r.Start.Line)
	assert.Equal(t, 10, r.End.Line)
}

func TestPosition_Structure(t *testing.T) {
	pos := Position{
		Line:      42,
		Character: 15,
	}

	assert.Equal(t, 42, pos.Line)
	assert.Equal(t, 15, pos.Character)
}

// StdioLSPTransport Tests

func TestStdioLSPTransport_IsConnected(t *testing.T) {
	transport := &StdioLSPTransport{
		connected: true,
	}

	assert.True(t, transport.IsConnected())

	transport.connected = false
	assert.False(t, transport.IsConnected())
}

func TestStdioLSPTransport_Send_NotConnected(t *testing.T) {
	transport := &StdioLSPTransport{
		connected: false,
	}
	ctx := context.Background()

	err := transport.Send(ctx, map[string]string{"test": "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestStdioLSPTransport_Receive_NotConnected(t *testing.T) {
	transport := &StdioLSPTransport{
		connected: false,
	}
	ctx := context.Background()

	result, err := transport.Receive(ctx)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not connected")
}

func TestStdioLSPTransport_Close(t *testing.T) {
	transport := &StdioLSPTransport{
		connected: true,
		stdin:     nil,
		cmd:       nil,
	}

	err := transport.Close()
	assert.NoError(t, err)
	assert.False(t, transport.connected)
}

// LSPClient Benchmarks

func BenchmarkLSPClient_ListServers(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	client := NewLSPClient(log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.ListServers()
	}
}

func BenchmarkLSPClient_HealthCheck(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	client := NewLSPClient(log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.HealthCheck(ctx)
	}
}

// Tests for LSP helper functions

func TestConvertCompletionList(t *testing.T) {
	t.Run("nil list returns nil", func(t *testing.T) {
		result := convertCompletionList(nil)
		assert.Nil(t, result)
	})

	t.Run("empty list returns empty slice", func(t *testing.T) {
		list := &CompletionList{
			IsIncomplete: false,
			Items:        []CompletionItem{},
		}
		result := convertCompletionList(list)
		require.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("converts completion items correctly", func(t *testing.T) {
		list := &CompletionList{
			IsIncomplete: true,
			Items: []CompletionItem{
				{
					Label:         "Println",
					Kind:          3, // Function
					Detail:        "func(a ...interface{}) (n int, err error)",
					Documentation: "Prints to stdout",
				},
				{
					Label:         "Printf",
					Kind:          3,
					Detail:        "func(format string, a ...interface{}) (n int, err error)",
					Documentation: "Formatted print",
				},
			},
		}
		result := convertCompletionList(list)
		require.NotNil(t, result)
		assert.Len(t, result, 2)

		// Check first item
		assert.Equal(t, "Println", result[0].Label)
		assert.Equal(t, 3, result[0].Kind)
		assert.Equal(t, "func(a ...interface{}) (n int, err error)", result[0].Detail)

		// Check second item
		assert.Equal(t, "Printf", result[1].Label)
		assert.Equal(t, 3, result[1].Kind)
	})

	t.Run("handles single item", func(t *testing.T) {
		list := &CompletionList{
			IsIncomplete: false,
			Items: []CompletionItem{
				{
					Label:  "TestFunc",
					Kind:   3,
					Detail: "test function",
				},
			},
		}
		result := convertCompletionList(list)
		require.NotNil(t, result)
		assert.Len(t, result, 1)
		assert.Equal(t, "TestFunc", result[0].Label)
	})
}

func TestConvertHover(t *testing.T) {
	t.Run("nil hover returns nil", func(t *testing.T) {
		result := convertHover(nil)
		assert.Nil(t, result)
	})

	t.Run("converts hover correctly", func(t *testing.T) {
		hover := &Hover{
			Contents: MarkupContent{
				Kind:  "markdown",
				Value: "```go\nfunc Println(a ...interface{}) (n int, err error)\n```",
			},
			Range: &Range{
				Start: Position{Line: 10, Character: 0},
				End:   Position{Line: 10, Character: 7},
			},
		}
		result := convertHover(hover)
		require.NotNil(t, result)
		assert.Equal(t, "```go\nfunc Println(a ...interface{}) (n int, err error)\n```", result.Content)
	})

	t.Run("converts hover with empty content", func(t *testing.T) {
		hover := &Hover{
			Contents: MarkupContent{
				Kind:  "plaintext",
				Value: "",
			},
		}
		result := convertHover(hover)
		require.NotNil(t, result)
		assert.Equal(t, "", result.Content)
	})

	t.Run("converts hover without range", func(t *testing.T) {
		hover := &Hover{
			Contents: MarkupContent{
				Kind:  "plaintext",
				Value: "Simple hover text",
			},
			Range: nil,
		}
		result := convertHover(hover)
		require.NotNil(t, result)
		assert.Equal(t, "Simple hover text", result.Content)
	})
}

func TestConvertLocation(t *testing.T) {
	t.Run("nil location returns nil", func(t *testing.T) {
		result := convertLocation(nil)
		assert.Nil(t, result)
	})

	t.Run("converts location correctly", func(t *testing.T) {
		loc := &Location{
			URI: "file:///workspace/main.go",
			Range: Range{
				Start: Position{Line: 10, Character: 5},
				End:   Position{Line: 10, Character: 15},
			},
		}
		result := convertLocation(loc)
		require.NotNil(t, result)
		assert.Equal(t, "file:///workspace/main.go", result.URI)
		assert.Equal(t, 10, result.Range.Start.Line)
		assert.Equal(t, 5, result.Range.Start.Character)
		assert.Equal(t, 10, result.Range.End.Line)
		assert.Equal(t, 15, result.Range.End.Character)
	})

	t.Run("converts location with zero positions", func(t *testing.T) {
		loc := &Location{
			URI: "file:///test.go",
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 0, Character: 0},
			},
		}
		result := convertLocation(loc)
		require.NotNil(t, result)
		assert.Equal(t, "file:///test.go", result.URI)
		assert.Equal(t, 0, result.Range.Start.Line)
		assert.Equal(t, 0, result.Range.Start.Character)
	})

	t.Run("converts location with large line numbers", func(t *testing.T) {
		loc := &Location{
			URI: "file:///large-file.go",
			Range: Range{
				Start: Position{Line: 10000, Character: 100},
				End:   Position{Line: 10050, Character: 0},
			},
		}
		result := convertLocation(loc)
		require.NotNil(t, result)
		assert.Equal(t, 10000, result.Range.Start.Line)
		assert.Equal(t, 100, result.Range.Start.Character)
		assert.Equal(t, 10050, result.Range.End.Line)
	})
}

// Benchmarks for helper functions

func BenchmarkConvertCompletionList(b *testing.B) {
	list := &CompletionList{
		IsIncomplete: false,
		Items: []CompletionItem{
			{Label: "Println", Kind: 3, Detail: "func"},
			{Label: "Printf", Kind: 3, Detail: "func"},
			{Label: "Print", Kind: 3, Detail: "func"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = convertCompletionList(list)
	}
}

func BenchmarkConvertHover(b *testing.B) {
	hover := &Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: "```go\nfunc Println(a ...interface{}) (n int, err error)\n```",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = convertHover(hover)
	}
}

func BenchmarkConvertLocation(b *testing.B) {
	loc := &Location{
		URI: "file:///workspace/main.go",
		Range: Range{
			Start: Position{Line: 10, Character: 5},
			End:   Position{Line: 10, Character: 15},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = convertLocation(loc)
	}
}

// LSPClient helper function tests

func TestLSPClient_NextMessageID(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	id1 := client.nextMessageID()
	id2 := client.nextMessageID()
	id3 := client.nextMessageID()

	// Test that IDs increment sequentially
	assert.Equal(t, id1+1, id2)
	assert.Equal(t, id2+1, id3)
	assert.Greater(t, id1, 0)
}

func TestLSPClient_UnmarshalMessage(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	t.Run("unmarshal marshal error", func(t *testing.T) {
		// Channels cannot be marshaled to JSON
		data := make(chan int)
		var message LSPMessage
		err := client.unmarshalMessage(data, &message)
		assert.Error(t, err)
	})

	t.Run("unmarshal valid message", func(t *testing.T) {
		data := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"method":  "textDocument/completion",
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test.go",
				},
			},
		}

		var message LSPMessage
		err := client.unmarshalMessage(data, &message)
		require.NoError(t, err)
		assert.Equal(t, "2.0", message.JSONRPC)
		assert.Equal(t, "textDocument/completion", message.Method)
	})

	t.Run("unmarshal response message", func(t *testing.T) {
		data := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"result": map[string]interface{}{
				"items": []interface{}{},
			},
		}

		var message LSPMessage
		err := client.unmarshalMessage(data, &message)
		require.NoError(t, err)
		assert.Equal(t, "2.0", message.JSONRPC)
		assert.NotNil(t, message.Result)
	})
}

func TestLSPClient_UnmarshalResult(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	t.Run("unmarshal marshal error", func(t *testing.T) {
		// Channels cannot be marshaled to JSON
		result := make(chan int)
		var target map[string]interface{}
		err := client.unmarshalResult(result, &target)
		assert.Error(t, err)
	})

	t.Run("unmarshal completion result", func(t *testing.T) {
		result := map[string]interface{}{
			"isIncomplete": true,
			"items": []interface{}{
				map[string]interface{}{
					"label":  "Println",
					"kind":   float64(3),
					"detail": "func(a ...interface{}) (n int, err error)",
				},
			},
		}

		var target struct {
			IsIncomplete bool `json:"isIncomplete"`
			Items        []struct {
				Label  string  `json:"label"`
				Kind   float64 `json:"kind"`
				Detail string  `json:"detail"`
			} `json:"items"`
		}

		err := client.unmarshalResult(result, &target)
		require.NoError(t, err)
		assert.True(t, target.IsIncomplete)
		assert.Len(t, target.Items, 1)
		assert.Equal(t, "Println", target.Items[0].Label)
	})

	t.Run("unmarshal hover result", func(t *testing.T) {
		result := map[string]interface{}{
			"contents": map[string]interface{}{
				"kind":  "markdown",
				"value": "```go\nfunc main()\n```",
			},
		}

		var target struct {
			Contents struct {
				Kind  string `json:"kind"`
				Value string `json:"value"`
			} `json:"contents"`
		}

		err := client.unmarshalResult(result, &target)
		require.NoError(t, err)
		assert.Equal(t, "markdown", target.Contents.Kind)
		assert.Contains(t, target.Contents.Value, "func main()")
	})
}

// MockLSPTransport implements LSPTransport for testing
type MockLSPTransport struct {
	connected    bool
	sendFunc     func(ctx context.Context, message interface{}) error
	receiveFunc  func(ctx context.Context) (interface{}, error)
	closeFunc    func() error
	sendCalls    []interface{}
	receiveCalls int
}

func NewMockLSPTransport() *MockLSPTransport {
	return &MockLSPTransport{
		connected: true,
		sendCalls: make([]interface{}, 0),
	}
}

func (m *MockLSPTransport) Send(ctx context.Context, message interface{}) error {
	m.sendCalls = append(m.sendCalls, message)
	if m.sendFunc != nil {
		return m.sendFunc(ctx, message)
	}
	return nil
}

func (m *MockLSPTransport) Receive(ctx context.Context) (interface{}, error) {
	m.receiveCalls++
	if m.receiveFunc != nil {
		return m.receiveFunc(ctx)
	}
	// Return a mock successful response
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      float64(1),
		"result":  map[string]interface{}{},
	}, nil
}

func (m *MockLSPTransport) Close() error {
	m.connected = false
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *MockLSPTransport) IsConnected() bool {
	return m.connected
}

// Tests for LSPClient with connected mock servers

func TestLSPClient_DisconnectServer_Connected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()

	// Add a mock server connection
	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Name:      "Test LSP Server",
		Language:  "go",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			HoverProvider:      true,
			DefinitionProvider: true,
		},
		Connected: true,
		Files:     make(map[string]*LSPFileInfo),
	}
	client.mu.Unlock()

	err := client.DisconnectServer("test-server")
	require.NoError(t, err)

	// Verify server was removed
	client.mu.RLock()
	_, exists := client.servers["test-server"]
	client.mu.RUnlock()
	assert.False(t, exists)

	// Verify shutdown and exit messages were sent
	assert.GreaterOrEqual(t, len(mockTransport.sendCalls), 2)
}

func TestLSPClient_OpenFile_Connected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:           "test-server",
		Name:         "Test LSP Server",
		Language:     "go",
		Transport:    mockTransport,
		Capabilities: &LSPCapabilities{},
		Connected:    true,
		Files:        make(map[string]*LSPFileInfo),
	}
	client.mu.Unlock()

	ctx := context.Background()
	err := client.OpenFile(ctx, "test-server", "file:///test.go", "go", "package main")
	require.NoError(t, err)

	// Verify file was stored
	client.mu.RLock()
	server := client.servers["test-server"]
	fileInfo := server.Files["file:///test.go"]
	client.mu.RUnlock()

	assert.NotNil(t, fileInfo)
	assert.Equal(t, "file:///test.go", fileInfo.URI)
	assert.Equal(t, "go", fileInfo.LanguageID)
	assert.Equal(t, "package main", fileInfo.Content)

	// Verify didOpen was sent
	assert.Len(t, mockTransport.sendCalls, 1)
}

func TestLSPClient_UpdateFile_Connected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:           "test-server",
		Name:         "Test LSP Server",
		Language:     "go",
		Transport:    mockTransport,
		Capabilities: &LSPCapabilities{},
		Connected:    true,
		Files: map[string]*LSPFileInfo{
			"file:///test.go": {
				URI:        "file:///test.go",
				LanguageID: "go",
				Version:    1,
				Content:    "package main",
			},
		},
	}
	client.mu.Unlock()

	ctx := context.Background()
	err := client.UpdateFile(ctx, "test-server", "file:///test.go", "package main\n\nfunc main() {}")
	require.NoError(t, err)

	// Verify file was updated
	client.mu.RLock()
	server := client.servers["test-server"]
	fileInfo := server.Files["file:///test.go"]
	client.mu.RUnlock()

	assert.Equal(t, 2, fileInfo.Version)
	assert.Equal(t, "package main\n\nfunc main() {}", fileInfo.Content)
}

func TestLSPClient_UpdateFile_FileNotOpen(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Connected: true,
		Files:     make(map[string]*LSPFileInfo),
	}
	client.mu.Unlock()

	ctx := context.Background()
	err := client.UpdateFile(ctx, "test-server", "file:///nonexistent.go", "content")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not open")
}

func TestLSPClient_CloseFile_Connected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:           "test-server",
		Name:         "Test LSP Server",
		Language:     "go",
		Transport:    mockTransport,
		Capabilities: &LSPCapabilities{},
		Connected:    true,
		Files: map[string]*LSPFileInfo{
			"file:///test.go": {
				URI:        "file:///test.go",
				LanguageID: "go",
				Version:    1,
				Content:    "package main",
			},
		},
	}
	client.mu.Unlock()

	ctx := context.Background()
	err := client.CloseFile(ctx, "test-server", "file:///test.go")
	require.NoError(t, err)

	// Verify file was removed
	client.mu.RLock()
	server := client.servers["test-server"]
	_, exists := server.Files["file:///test.go"]
	client.mu.RUnlock()
	assert.False(t, exists)
}

func TestLSPClient_GetCompletion_ServerDoesNotSupportCompletion(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			CompletionProvider: nil, // No completion support
		},
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetCompletion(ctx, "test-server", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "does not support completion")
}

func TestLSPClient_GetCompletion_WithSupport(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"result": map[string]interface{}{
				"isIncomplete": false,
				"items": []interface{}{
					map[string]interface{}{
						"label":  "Println",
						"kind":   float64(3),
						"detail": "func(a ...interface{}) (n int, err error)",
					},
				},
			},
		}, nil
	}

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			CompletionProvider: &CompletionOptions{
				TriggerCharacters: []string{"."},
			},
		},
		Connected: true,
		LastUsed:  time.Now(),
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetCompletion(ctx, "test-server", "file:///test.go", 10, 5)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestLSPClient_GetCompletion_SendError(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.sendFunc = func(ctx context.Context, message interface{}) error {
		return errors.New("send failed")
	}

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			CompletionProvider: &CompletionOptions{},
		},
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetCompletion(ctx, "test-server", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to send completion request")
}

func TestLSPClient_GetCompletion_ReceiveError(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("receive failed")
	}

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			CompletionProvider: &CompletionOptions{},
		},
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetCompletion(ctx, "test-server", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to receive completion response")
}

func TestLSPClient_GetCompletion_ErrorResponse(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"error": map[string]interface{}{
				"code":    float64(-32600),
				"message": "Invalid Request",
			},
		}, nil
	}

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			CompletionProvider: &CompletionOptions{},
		},
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetCompletion(ctx, "test-server", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "completion error")
}

func TestLSPClient_GetHover_ServerDoesNotSupportHover(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			HoverProvider: false, // No hover support
		},
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetHover(ctx, "test-server", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "does not support hover")
}

func TestLSPClient_GetHover_WithSupport(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"result": map[string]interface{}{
				"contents": map[string]interface{}{
					"kind":  "markdown",
					"value": "```go\nfunc Println(...)\n```",
				},
			},
		}, nil
	}

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			HoverProvider: true,
		},
		Connected: true,
		LastUsed:  time.Now(),
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetHover(ctx, "test-server", "file:///test.go", 10, 5)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestLSPClient_GetHover_SendError(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.sendFunc = func(ctx context.Context, message interface{}) error {
		return errors.New("send failed")
	}

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			HoverProvider: true,
		},
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetHover(ctx, "test-server", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to send hover request")
}

func TestLSPClient_GetHover_ReceiveError(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("receive failed")
	}

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			HoverProvider: true,
		},
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetHover(ctx, "test-server", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to receive hover response")
}

func TestLSPClient_GetHover_ErrorResponse(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"error": map[string]interface{}{
				"code":    float64(-32600),
				"message": "Invalid Request",
			},
		}, nil
	}

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			HoverProvider: true,
		},
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetHover(ctx, "test-server", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "hover error")
}

func TestLSPClient_GetDefinition_ServerDoesNotSupportDefinition(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			DefinitionProvider: false, // No definition support
		},
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetDefinition(ctx, "test-server", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "does not support definition")
}

func TestLSPClient_GetDefinition_WithSupport(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"result": map[string]interface{}{
				"uri": "file:///src/main.go",
				"range": map[string]interface{}{
					"start": map[string]interface{}{
						"line":      float64(5),
						"character": float64(0),
					},
					"end": map[string]interface{}{
						"line":      float64(5),
						"character": float64(10),
					},
				},
			},
		}, nil
	}

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			DefinitionProvider: true,
		},
		Connected: true,
		LastUsed:  time.Now(),
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetDefinition(ctx, "test-server", "file:///test.go", 10, 5)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestLSPClient_GetDefinition_SendError(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.sendFunc = func(ctx context.Context, message interface{}) error {
		return errors.New("send failed")
	}

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			DefinitionProvider: true,
		},
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetDefinition(ctx, "test-server", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to send definition request")
}

func TestLSPClient_GetDefinition_ReceiveError(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("receive failed")
	}

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			DefinitionProvider: true,
		},
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetDefinition(ctx, "test-server", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to receive definition response")
}

func TestLSPClient_GetDefinition_ErrorResponse(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"error": map[string]interface{}{
				"code":    float64(-32600),
				"message": "Invalid Request",
			},
		}, nil
	}

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			DefinitionProvider: true,
		},
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetDefinition(ctx, "test-server", "file:///test.go", 10, 5)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "definition error")
}

func TestLSPClient_ListServers_WithServers(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	client.mu.Lock()
	client.servers["server-1"] = &LSPServerConnection{
		ID:        "server-1",
		Name:      "Server 1",
		Language:  "go",
		Connected: true,
	}
	client.servers["server-2"] = &LSPServerConnection{
		ID:        "server-2",
		Name:      "Server 2",
		Language:  "python",
		Connected: true,
	}
	client.mu.Unlock()

	servers := client.ListServers()
	assert.Len(t, servers, 2)
}

func TestLSPClient_GetServerCapabilities_Connected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:   "test-server",
		Name: "Test Server",
		Capabilities: &LSPCapabilities{
			HoverProvider:      true,
			DefinitionProvider: true,
			CompletionProvider: &CompletionOptions{
				TriggerCharacters: []string{"."},
			},
		},
		Connected: true,
	}
	client.mu.Unlock()

	caps, err := client.GetServerCapabilities("test-server")
	require.NoError(t, err)
	assert.NotNil(t, caps)
	assert.True(t, caps.HoverProvider)
	assert.True(t, caps.DefinitionProvider)
}

func TestLSPClient_HealthCheck_WithServers(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.connected = true

	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	results := client.HealthCheck(ctx)

	assert.Len(t, results, 1)
	assert.Contains(t, results, "test-server")
	assert.Equal(t, true, results["test-server"])
}

func TestLSPClient_GetCodeIntelligence_Connected(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	// Track which methods have been called
	callCount := 0

	mockTransport := NewMockLSPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		callCount++
		if callCount == 1 {
			// First call: OpenFile (textDocument/didOpen)
			return map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      float64(1),
				"result":  nil,
			}, nil
		} else if callCount == 2 {
			// Second call: GetCompletion
			return map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      float64(2),
				"result": map[string]interface{}{
					"isIncomplete": false,
					"items": []interface{}{
						map[string]interface{}{
							"label":  "testCompletion",
							"kind":   float64(1),
							"detail": "Test completion item",
						},
					},
				},
			}, nil
		} else if callCount == 3 {
			// Third call: GetHover
			return map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      float64(3),
				"result": map[string]interface{}{
					"contents": "Hover content",
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": float64(0), "character": float64(0)},
						"end":   map[string]interface{}{"line": float64(0), "character": float64(10)},
					},
				},
			}, nil
		}
		// Fourth call: GetDefinition
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(4),
			"result": map[string]interface{}{
				"uri": "file:///test/definition.go",
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": float64(10), "character": float64(5)},
					"end":   map[string]interface{}{"line": float64(10), "character": float64(15)},
				},
			},
		}, nil
	}

	// Must use "default-go" as the serverID since that's what GetCodeIntelligence uses
	client.mu.Lock()
	client.servers["default-go"] = &LSPServerConnection{
		ID:        "default-go",
		Transport: mockTransport,
		Capabilities: &LSPCapabilities{
			HoverProvider:      true,
			DefinitionProvider: true,
			CompletionProvider: &CompletionOptions{},
		},
		Connected: true,
		Files:     make(map[string]*LSPFileInfo),
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.GetCodeIntelligence(ctx, "/test/file.go", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "/test/file.go", result.FilePath)
	assert.NotNil(t, result.Completions)
	assert.NotNil(t, result.Hover)
	assert.NotNil(t, result.Definitions)
}

// Tests for ACPClient with mock agents

func TestACPDiscoveryClient_GetAgentCapabilities_Connected(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	mockTransport := NewMockACPTransport()

	client.mu.Lock()
	client.agents["test-agent"] = &ACPAgentConnection{
		ID:        "test-agent",
		Name:      "Test Agent",
		Transport: mockTransport,
		Capabilities: map[string]interface{}{
			"streaming": true,
			"tools":     []string{"calculator", "search"},
		},
		Connected: true,
	}
	client.mu.Unlock()

	caps, err := client.GetAgentCapabilities("test-agent")
	require.NoError(t, err)
	assert.NotNil(t, caps)
	assert.Equal(t, true, caps["streaming"])
}

func TestACPDiscoveryClient_ExecuteAction_Connected(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	mockTransport := NewMockACPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"result": map[string]interface{}{
				"success": true,
				"result":  "action completed",
			},
		}, nil
	}

	client.mu.Lock()
	client.agents["test-agent"] = &ACPAgentConnection{
		ID:           "test-agent",
		Name:         "Test Agent",
		Transport:    mockTransport,
		Capabilities: map[string]interface{}{},
		Connected:    true,
		LastUsed:     time.Now(),
	}
	client.mu.Unlock()

	ctx := context.Background()
	result, err := client.ExecuteAction(ctx, "test-agent", "test-action", map[string]interface{}{
		"param1": "value1",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestACPDiscoveryClient_ListAgents_WithAgents(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	client.mu.Lock()
	client.agents["agent-1"] = &ACPAgentConnection{
		ID:        "agent-1",
		Name:      "Agent 1",
		Connected: true,
	}
	client.agents["agent-2"] = &ACPAgentConnection{
		ID:        "agent-2",
		Name:      "Agent 2",
		Connected: true,
	}
	client.mu.Unlock()

	agents := client.ListAgents()
	assert.Len(t, agents, 2)
}

func TestACPDiscoveryClient_HealthCheck_WithAgents(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	mockTransport := NewMockACPTransport()
	mockTransport.connected = true

	client.mu.Lock()
	client.agents["test-agent"] = &ACPAgentConnection{
		ID:        "test-agent",
		Transport: mockTransport,
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	results := client.HealthCheck(ctx)

	assert.Len(t, results, 1)
	assert.Contains(t, results, "test-agent")
	assert.Equal(t, true, results["test-agent"])
}

func TestACPDiscoveryClient_GetAgentStatus_Connected(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	now := time.Now()
	client.mu.Lock()
	client.agents["test-agent"] = &ACPAgentConnection{
		ID:        "test-agent",
		Name:      "Test Agent",
		Connected: true,
		LastUsed:  now,
		Capabilities: map[string]interface{}{
			"tools": true,
		},
	}
	client.mu.Unlock()

	ctx := context.Background()
	status, err := client.GetAgentStatus(ctx, "test-agent")
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, "test-agent", status["id"])
	assert.Equal(t, "Test Agent", status["name"])
	assert.Equal(t, true, status["connected"])
}

func TestACPDiscoveryClient_DisconnectAgent_Connected(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPDiscoveryClient(log)

	mockTransport := NewMockACPTransport()
	mockTransport.connected = true

	client.mu.Lock()
	client.agents["test-agent"] = &ACPAgentConnection{
		ID:        "test-agent",
		Name:      "Test Agent",
		Transport: mockTransport,
		Connected: true,
	}
	client.mu.Unlock()

	err := client.DisconnectAgent("test-agent")
	require.NoError(t, err)

	// Verify agent was removed
	client.mu.RLock()
	_, exists := client.agents["test-agent"]
	client.mu.RUnlock()
	assert.False(t, exists)
}

func TestLSPClient_DisconnectServer_ShutdownError(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.sendFunc = func(ctx context.Context, message interface{}) error {
		return errors.New("send error")
	}

	// Add a mock server connection
	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Name:      "Test LSP Server",
		Language:  "go",
		Transport: mockTransport,
		Connected: true,
		Files:     make(map[string]*LSPFileInfo),
	}
	client.mu.Unlock()

	// Should still succeed even if shutdown send fails
	err := client.DisconnectServer("test-server")
	require.NoError(t, err)

	// Verify server was removed
	client.mu.RLock()
	_, exists := client.servers["test-server"]
	client.mu.RUnlock()
	assert.False(t, exists)
}

func TestLSPClient_DisconnectServer_CloseError(t *testing.T) {
	log := newACPTestLogger()
	client := NewLSPClient(log)

	mockTransport := NewMockLSPTransport()
	mockTransport.closeFunc = func() error {
		return errors.New("close error")
	}

	// Add a mock server connection
	client.mu.Lock()
	client.servers["test-server"] = &LSPServerConnection{
		ID:        "test-server",
		Name:      "Test LSP Server",
		Language:  "go",
		Transport: mockTransport,
		Connected: true,
		Files:     make(map[string]*LSPFileInfo),
	}
	client.mu.Unlock()

	// Should still succeed even if close fails
	err := client.DisconnectServer("test-server")
	require.NoError(t, err)

	// Verify server was removed
	client.mu.RLock()
	_, exists := client.servers["test-server"]
	client.mu.RUnlock()
	assert.False(t, exists)
}
