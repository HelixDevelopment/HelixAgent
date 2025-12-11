package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/superagent/superagent/internal/models"
)

// LSPClient manages Language Server Protocol connections
type LSPClient struct {
	workspaceRoot    string
	languageID       string
	server           *LSPServer
	messageID        int
	mu               sync.RWMutex
	diagnostics      map[string][]*models.Diagnostic // URI -> diagnostics
	diagnosticsMu    sync.RWMutex
	notificationChan chan *LSPMessage
}

// LSPServer represents a running LSP server
type LSPServer struct {
	Process      *exec.Cmd
	Stdin        io.WriteCloser
	Stdout       io.ReadCloser
	Capabilities map[string]interface{}
	Initialized  bool
	LastHealth   time.Time
}

// LSPMessage represents a JSON-RPC message for LSP
type LSPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *LSPError   `json:"error,omitempty"`
}

// LSPError represents an LSP error
type LSPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// LSPRange represents a range in a text document
type LSPRange struct {
	Start models.Position `json:"start"`
	End   models.Position `json:"end"`
}

// LSPTextDocumentContentChangeEvent represents a change to a text document
type LSPTextDocumentContentChangeEvent struct {
	Range       *LSPRange `json:"range,omitempty"`
	RangeLength *int      `json:"rangeLength,omitempty"`
	Text        string    `json:"text"`
}

// LSPTextDocument represents a text document identifier
type LSPTextDocument struct {
	URI string `json:"uri"`
}

// LSPTextDocumentPosition represents a position in a text document
type LSPTextDocumentPosition struct {
	TextDocument LSPTextDocument `json:"textDocument"`
	Position     models.Position `json:"position"`
}

// NewLSPClient creates a new LSP client
func NewLSPClient(workspaceRoot, languageID string) *LSPClient {
	return &LSPClient{
		workspaceRoot:    workspaceRoot,
		languageID:       languageID,
		messageID:        1,
		diagnostics:      make(map[string][]*models.Diagnostic),
		notificationChan: make(chan *LSPMessage, 100),
	}
}

// StartServer starts the appropriate LSP server for the language
func (c *LSPClient) StartServer(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.server != nil {
		return fmt.Errorf("LSP server already running")
	}

	// Determine which server to start based on language
	serverConfig, err := c.getServerConfig()
	if err != nil {
		return fmt.Errorf("failed to get server config: %w", err)
	}

	// Start the server process
	server := &LSPServer{}
	if err := c.startServerProcess(server, serverConfig); err != nil {
		return fmt.Errorf("failed to start server process: %w", err)
	}

	// Initialize the server
	if err := c.initializeServer(ctx, server); err != nil {
		if server.Process.Process != nil {
			server.Process.Process.Kill()
		}
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	c.server = server

	// Start notification listener
	go c.listenForNotifications(server)

	log.Printf("Successfully started LSP server for %s", c.languageID)

	return nil
}

// getServerConfig returns the server configuration for the language
func (c *LSPClient) getServerConfig() (map[string]interface{}, error) {
	// Common LSP server configurations
	servers := map[string]map[string]interface{}{
		"go": {
			"command": []string{"gopls"},
			"args":    []string{},
		},
		"typescript": {
			"command": []string{"typescript-language-server", "--stdio"},
			"args":    []string{},
		},
		"javascript": {
			"command": []string{"typescript-language-server", "--stdio"},
			"args":    []string{},
		},
		"python": {
			"command": []string{"pylsp"},
			"args":    []string{},
		},
		"rust": {
			"command": []string{"rust-analyzer"},
			"args":    []string{},
		},
		"java": {
			"command": []string{"java", "-jar", "/path/to/jdt-language-server/plugins/org.eclipse.equinox.launcher.jar"},
			"args":    []string{"-configuration", "/path/to/jdt-language-server/config", "-data", "/tmp/jdt-data"},
		},
	}

	config, exists := servers[c.languageID]
	if !exists {
		return nil, fmt.Errorf("no LSP server configured for language: %s", c.languageID)
	}

	return config, nil
}

// startServerProcess starts the LSP server process
func (c *LSPClient) startServerProcess(server *LSPServer, config map[string]interface{}) error {
	command := config["command"].([]string)
	args := config["args"].([]string)

	cmd := exec.Command(command[0], append(command[1:], args...)...)
	cmd.Dir = c.workspaceRoot

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	server.Process = cmd
	server.Stdin = stdin
	server.Stdout = stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start LSP server: %w", err)
	}

	return nil
}

// initializeServer performs LSP initialization handshake
func (c *LSPClient) initializeServer(ctx context.Context, server *LSPServer) error {
	initRequest := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "initialize",
		Params: map[string]interface{}{
			"processId":    nil,
			"rootUri":      fmt.Sprintf("file://%s", c.workspaceRoot),
			"capabilities": c.getClientCapabilities(),
		},
	}

	response, err := c.sendMessage(server, initRequest)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	if response.Error != nil {
		return fmt.Errorf("initialize error: %s", response.Error.Message)
	}

	// Store server capabilities
	if result, ok := response.Result.(map[string]interface{}); ok {
		if caps, ok := result["capabilities"].(map[string]interface{}); ok {
			server.Capabilities = caps
		}
	}

	// Send initialized notification
	initializedMsg := LSPMessage{
		JSONRPC: "2.0",
		Method:  "initialized",
		Params:  map[string]interface{}{},
	}

	if err := c.sendNotification(server, initializedMsg); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	server.Initialized = true
	return nil
}

// listenForNotifications listens for incoming notifications from the LSP server
func (c *LSPClient) listenForNotifications(server *LSPServer) {
	scanner := bufio.NewScanner(server.Stdout)

	for scanner.Scan() {
		header := scanner.Text()
		if header == "" {
			continue
		}

		var contentLength int
		if _, err := fmt.Sscanf(header, "Content-Length: %d", &contentLength); err != nil {
			continue
		}

		if !scanner.Scan() || scanner.Text() != "" {
			continue
		}

		content := make([]byte, contentLength)
		if _, err := io.ReadFull(server.Stdout, content); err != nil {
			continue
		}

		var message LSPMessage
		if err := json.Unmarshal(content, &message); err != nil {
			continue
		}

		// Handle notification (no ID)
		if message.ID == nil {
			c.handleNotification(&message)
		} else {
			// This is a response to a request, send it to the channel
			select {
			case c.notificationChan <- &message:
			default:
				// Channel full, drop message
			}
		}
	}
}

// handleNotification processes incoming LSP notifications
func (c *LSPClient) handleNotification(message *LSPMessage) {
	switch message.Method {
	case "textDocument/publishDiagnostics":
		c.handlePublishDiagnostics(message.Params)
	}
}

// handlePublishDiagnostics handles diagnostic notifications
func (c *LSPClient) handlePublishDiagnostics(params interface{}) {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return
	}

	uri, ok := paramsMap["uri"].(string)
	if !ok {
		return
	}

	diagnosticsData, ok := paramsMap["diagnostics"].([]interface{})
	if !ok {
		return
	}

	diagnostics := []*models.Diagnostic{}
	for _, diagData := range diagnosticsData {
		diagMap, ok := diagData.(map[string]interface{})
		if !ok {
			continue
		}

		diagnostic := &models.Diagnostic{}

		if rangeData, ok := diagMap["range"].(map[string]interface{}); ok {
			rng, err := c.parseRange(rangeData)
			if err == nil {
				diagnostic.Range = *rng
			}
		}

		if severity, ok := diagMap["severity"].(float64); ok {
			diagnostic.Severity = int(severity)
		}

		if code, ok := diagMap["code"].(string); ok {
			diagnostic.Code = code
		}

		if message, ok := diagMap["message"].(string); ok {
			diagnostic.Message = message
		}

		if source, ok := diagMap["source"].(string); ok {
			diagnostic.Source = source
		}

		diagnostics = append(diagnostics, diagnostic)
	}

	c.diagnosticsMu.Lock()
	c.diagnostics[uri] = diagnostics
	c.diagnosticsMu.Unlock()
}

// GetDiagnostics returns cached diagnostics for a file
func (c *LSPClient) GetDiagnostics(filePath string) []*models.Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	uri := fmt.Sprintf("file://%s", filePath)
	return c.diagnostics[uri]
}

// getClientCapabilities returns the client capabilities
func (c *LSPClient) getClientCapabilities() map[string]interface{} {
	return map[string]interface{}{
		"textDocument": map[string]interface{}{
			"completion": map[string]interface{}{
				"dynamicRegistration": false,
				"completionItem": map[string]interface{}{
					"snippetSupport":          true,
					"commitCharactersSupport": true,
					"documentationFormat":     []string{"markdown", "plaintext"},
					"deprecatedSupport":       true,
					"preselectSupport":        true,
				},
			},
			"hover": map[string]interface{}{
				"dynamicRegistration": false,
				"contentFormat":       []string{"markdown", "plaintext"},
			},
			"signatureHelp": map[string]interface{}{
				"dynamicRegistration": false,
				"signatureInformation": map[string]interface{}{
					"documentationFormat": []string{"markdown", "plaintext"},
				},
			},
			"definition": map[string]interface{}{
				"dynamicRegistration": false,
			},
			"references": map[string]interface{}{
				"dynamicRegistration": false,
			},
			"documentHighlight": map[string]interface{}{
				"dynamicRegistration": false,
			},
			"documentSymbol": map[string]interface{}{
				"dynamicRegistration": false,
			},
			"codeAction": map[string]interface{}{
				"dynamicRegistration": false,
				"codeActionLiteralSupport": map[string]interface{}{
					"codeActionKind": map[string]interface{}{
						"valueSet": []string{
							"quickfix",
							"refactor",
							"refactor.extract",
							"refactor.inline",
							"refactor.rewrite",
							"source",
							"source.organizeImports",
						},
					},
				},
			},
			"codeLens": map[string]interface{}{
				"dynamicRegistration": false,
			},
			"formatting": map[string]interface{}{
				"dynamicRegistration": false,
			},
			"rangeFormatting": map[string]interface{}{
				"dynamicRegistration": false,
			},
			"rename": map[string]interface{}{
				"dynamicRegistration": false,
			},
			"semanticTokens": map[string]interface{}{
				"dynamicRegistration": false,
				"requests": map[string]interface{}{
					"range": true,
					"full":  true,
				},
				"tokenTypes": []string{
					"namespace", "type", "class", "enum", "interface", "struct", "typeParameter",
					"parameter", "variable", "property", "enumMember", "event", "function", "method",
					"macro", "keyword", "modifier", "comment", "string", "number", "regexp", "operator",
				},
				"tokenModifiers": []string{
					"declaration", "definition", "readonly", "static", "deprecated", "abstract",
					"async", "modification", "documentation", "defaultLibrary",
				},
				"formats": []string{"relative"},
			},
		},
		"workspace": map[string]interface{}{
			"applyEdit": true,
			"workspaceEdit": map[string]interface{}{
				"documentChanges": true,
			},
			"didChangeConfiguration": map[string]interface{}{
				"dynamicRegistration": false,
			},
			"didChangeWatchedFiles": map[string]interface{}{
				"dynamicRegistration": false,
			},
			"symbol": map[string]interface{}{
				"dynamicRegistration": false,
			},
			"executeCommand": map[string]interface{}{
				"dynamicRegistration": false,
			},
		},
	}
}

// GetCodeIntelligence gets comprehensive code intelligence for a file
func (c *LSPClient) GetCodeIntelligence(ctx context.Context, filePath string, cursorPos *models.Position) (*models.CodeIntelligence, error) {
	c.mu.RLock()
	server := c.server
	c.mu.RUnlock()

	if server == nil || !server.Initialized {
		return nil, fmt.Errorf("LSP server not initialized")
	}

	intelligence := &models.CodeIntelligence{
		FilePath: filePath,
	}

	// Open the document
	if err := c.openDocument(server, filePath); err != nil {
		return nil, fmt.Errorf("failed to open document: %w", err)
	}

	// Get diagnostics
	diagnostics, err := c.getDiagnostics(server, filePath)
	if err != nil {
		log.Printf("Failed to get diagnostics: %v", err)
	}
	intelligence.Diagnostics = diagnostics

	if cursorPos != nil {
		// Get completions at cursor position
		completions, err := c.getCompletions(server, filePath, *cursorPos)
		if err != nil {
			log.Printf("Failed to get completions: %v", err)
		}
		intelligence.Completions = completions

		// Get hover information
		hover, err := c.getHover(server, filePath, *cursorPos)
		if err != nil {
			log.Printf("Failed to get hover: %v", err)
		}
		intelligence.Hover = hover

		// Get definitions
		definitions, err := c.getDefinitions(server, filePath, *cursorPos)
		if err != nil {
			log.Printf("Failed to get definitions: %v", err)
		}
		intelligence.Definitions = definitions

		// Get references
		references, err := c.getReferences(server, filePath, *cursorPos)
		if err != nil {
			log.Printf("Failed to get references: %v", err)
		}
		intelligence.References = references
	}

	// Get document symbols
	symbols, err := c.getDocumentSymbols(server, filePath)
	if err != nil {
		log.Printf("Failed to get document symbols: %v", err)
	}
	intelligence.Symbols = symbols

	// Get semantic tokens
	semanticTokens, err := c.getSemanticTokens(server, filePath)
	if err != nil {
		log.Printf("Failed to get semantic tokens: %v", err)
	}
	intelligence.SemanticTokens = semanticTokens

	return intelligence, nil
}

// openDocument opens a document for LSP operations
func (c *LSPClient) openDocument(server *LSPServer, filePath string) error {
	content, err := c.readFileContent(filePath)
	if err != nil {
		return err
	}

	openMsg := LSPMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":        fmt.Sprintf("file://%s", filePath),
				"languageId": c.languageID,
				"version":    1,
				"text":       content,
			},
		},
	}

	return c.sendNotification(server, openMsg)
}

// updateDocument sends a didChange notification for document updates
func (c *LSPClient) updateDocument(server *LSPServer, filePath string, changes []LSPTextDocumentContentChangeEvent) error {
	changeMsg := LSPMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/didChange",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":     fmt.Sprintf("file://%s", filePath),
				"version": 2, // Increment version
			},
			"contentChanges": changes,
		},
	}

	return c.sendNotification(server, changeMsg)
}

// closeDocument sends a didClose notification
func (c *LSPClient) closeDocument(server *LSPServer, filePath string) error {
	closeMsg := LSPMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/didClose",
		Params: map[string]interface{}{
			"textDocument": LSPTextDocument{URI: fmt.Sprintf("file://%s", filePath)},
		},
	}

	return c.sendNotification(server, closeMsg)
}

// getDiagnostics gets diagnostics for a file
func (c *LSPClient) getDiagnostics(server *LSPServer, filePath string) ([]*models.Diagnostic, error) {
	// Diagnostics are cached from notifications
	return c.GetDiagnostics(filePath), nil
}

// getCompletions gets completion items at a position
func (c *LSPClient) getCompletions(server *LSPServer, filePath string, position models.Position) ([]*models.CompletionItem, error) {
	if !c.supportsCapability(server, "completionProvider") {
		return nil, fmt.Errorf("server does not support completions")
	}

	request := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "textDocument/completion",
		Params: LSPTextDocumentPosition{
			TextDocument: LSPTextDocument{URI: fmt.Sprintf("file://%s", filePath)},
			Position:     position,
		},
	}

	response, err := c.sendMessage(server, request)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("completion error: %s", response.Error.Message)
	}

	return c.parseCompletionItems(response.Result)
}

// getHover gets hover information at a position
func (c *LSPClient) getHover(server *LSPServer, filePath string, position models.Position) (*models.HoverInfo, error) {
	if !c.supportsCapability(server, "hoverProvider") {
		return nil, fmt.Errorf("server does not support hover")
	}

	request := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "textDocument/hover",
		Params: LSPTextDocumentPosition{
			TextDocument: LSPTextDocument{URI: fmt.Sprintf("file://%s", filePath)},
			Position:     position,
		},
	}

	response, err := c.sendMessage(server, request)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("hover error: %s", response.Error.Message)
	}

	return c.parseHoverInfo(response.Result)
}

// getDefinitions gets definitions at a position
func (c *LSPClient) getDefinitions(server *LSPServer, filePath string, position models.Position) ([]*models.Location, error) {
	if !c.supportsCapability(server, "definitionProvider") {
		return nil, fmt.Errorf("server does not support definitions")
	}

	request := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "textDocument/definition",
		Params: LSPTextDocumentPosition{
			TextDocument: LSPTextDocument{URI: fmt.Sprintf("file://%s", filePath)},
			Position:     position,
		},
	}

	response, err := c.sendMessage(server, request)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("definition error: %s", response.Error.Message)
	}

	return c.parseLocations(response.Result)
}

// getReferences gets references at a position
func (c *LSPClient) getReferences(server *LSPServer, filePath string, position models.Position) ([]*models.Location, error) {
	if !c.supportsCapability(server, "referencesProvider") {
		return nil, fmt.Errorf("server does not support references")
	}

	request := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "textDocument/references",
		Params: map[string]interface{}{
			"textDocument": LSPTextDocument{URI: fmt.Sprintf("file://%s", filePath)},
			"position":     position,
			"context": map[string]interface{}{
				"includeDeclaration": true,
			},
		},
	}

	response, err := c.sendMessage(server, request)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("references error: %s", response.Error.Message)
	}

	return c.parseLocations(response.Result)
}

// getDocumentSymbols gets symbols in a document
func (c *LSPClient) getDocumentSymbols(server *LSPServer, filePath string) ([]*models.SymbolInfo, error) {
	if !c.supportsCapability(server, "documentSymbolProvider") {
		return nil, fmt.Errorf("server does not support document symbols")
	}

	request := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "textDocument/documentSymbol",
		Params: map[string]interface{}{
			"textDocument": LSPTextDocument{URI: fmt.Sprintf("file://%s", filePath)},
		},
	}

	response, err := c.sendMessage(server, request)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("document symbol error: %s", response.Error.Message)
	}

	return c.parseSymbols(response.Result)
}

// getSemanticTokens gets semantic tokens for a document
func (c *LSPClient) getSemanticTokens(server *LSPServer, filePath string) (*models.SemanticTokens, error) {
	if !c.supportsCapability(server, "semanticTokensProvider") {
		return nil, fmt.Errorf("server does not support semantic tokens")
	}

	request := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "textDocument/semanticTokens/full",
		Params: map[string]interface{}{
			"textDocument": LSPTextDocument{URI: fmt.Sprintf("file://%s", filePath)},
		},
	}

	response, err := c.sendMessage(server, request)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("semantic tokens error: %s", response.Error.Message)
	}

	return c.parseSemanticTokens(response.Result)
}

// parseSemanticTokens parses semantic tokens response
func (c *LSPClient) parseSemanticTokens(result interface{}) (*models.SemanticTokens, error) {
	tokens := &models.SemanticTokens{}

	if resultMap, ok := result.(map[string]interface{}); ok {
		if data, ok := resultMap["data"].([]interface{}); ok {
			tokens.Data = make([]int, len(data))
			for i, val := range data {
				if num, ok := val.(float64); ok {
					tokens.Data[i] = int(num)
				}
			}
		}
	}

	return tokens, nil
}

// GetWorkspaceSymbols gets symbols from the entire workspace
func (c *LSPClient) GetWorkspaceSymbols(ctx context.Context) ([]*models.SymbolInfo, error) {
	c.mu.RLock()
	server := c.server
	c.mu.RUnlock()

	if server == nil || !server.Initialized {
		return nil, fmt.Errorf("LSP server not initialized")
	}

	if !c.supportsCapability(server, "workspaceSymbolProvider") {
		return nil, fmt.Errorf("server does not support workspace symbols")
	}

	request := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "workspace/symbol",
		Params: map[string]interface{}{
			"query": "",
		},
	}

	response, err := c.sendMessage(server, request)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("workspace symbol error: %s", response.Error.Message)
	}

	return c.parseSymbols(response.Result)
}

// GetReferences gets all references to a symbol at a position across the workspace
func (c *LSPClient) GetReferences(ctx context.Context, filePath string, position models.Position, includeDeclaration bool) ([]*models.Location, error) {
	c.mu.RLock()
	server := c.server
	c.mu.RUnlock()

	if server == nil || !server.Initialized {
		return nil, fmt.Errorf("LSP server not initialized")
	}

	if !c.supportsCapability(server, "referencesProvider") {
		return nil, fmt.Errorf("server does not support references")
	}

	request := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "textDocument/references",
		Params: map[string]interface{}{
			"textDocument": LSPTextDocument{URI: fmt.Sprintf("file://%s", filePath)},
			"position":     position,
			"context": map[string]interface{}{
				"includeDeclaration": includeDeclaration,
			},
		},
	}

	response, err := c.sendMessage(server, request)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("references error: %s", response.Error.Message)
	}

	return c.parseLocations(response.Result)
}

// RenameSymbol performs a workspace-wide rename of a symbol
func (c *LSPClient) RenameSymbol(ctx context.Context, filePath string, position models.Position, newName string) (*models.WorkspaceEdit, error) {
	c.mu.RLock()
	server := c.server
	c.mu.RUnlock()

	if server == nil || !server.Initialized {
		return nil, fmt.Errorf("LSP server not initialized")
	}

	if !c.supportsCapability(server, "renameProvider") {
		return nil, fmt.Errorf("server does not support rename")
	}

	request := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "textDocument/rename",
		Params: map[string]interface{}{
			"textDocument": LSPTextDocument{URI: fmt.Sprintf("file://%s", filePath)},
			"position":     position,
			"newName":      newName,
		},
	}

	response, err := c.sendMessage(server, request)
	if err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("rename error: %s", response.Error.Message)
	}

	return c.parseWorkspaceEdit(response.Result)
}

// parseWorkspaceEdit parses a workspace edit response
func (c *LSPClient) parseWorkspaceEdit(result interface{}) (*models.WorkspaceEdit, error) {
	edit := &models.WorkspaceEdit{}

	if resultMap, ok := result.(map[string]interface{}); ok {
		if changes, ok := resultMap["changes"].(map[string]interface{}); ok {
			edit.Changes = make(map[string][]*models.TextEdit)
			for uri, edits := range changes {
				if editsArray, ok := edits.([]interface{}); ok {
					textEdits := make([]*models.TextEdit, len(editsArray))
					for i, editData := range editsArray {
						if editMap, ok := editData.(map[string]interface{}); ok {
							textEdit := &models.TextEdit{}
							if rangeData, ok := editMap["range"].(map[string]interface{}); ok {
								if rng, err := c.parseRange(rangeData); err == nil {
									textEdit.Range = *rng
								}
							}
							if newText, ok := editMap["newText"].(string); ok {
								textEdit.NewText = newText
							}
							textEdits[i] = textEdit
						}
					}
					edit.Changes[uri] = textEdits
				}
			}
		}
	}

	return edit, nil
}

// Helper methods for parsing LSP responses
func (c *LSPClient) parseCompletionItems(result interface{}) ([]*models.CompletionItem, error) {
	// Simplified parsing - would need full implementation
	items := []*models.CompletionItem{}
	if resultMap, ok := result.(map[string]interface{}); ok {
		if itemsArray, ok := resultMap["items"].([]interface{}); ok {
			for _, item := range itemsArray {
				if itemMap, ok := item.(map[string]interface{}); ok {
					completion := &models.CompletionItem{
						Label: itemMap["label"].(string),
						Kind:  int(itemMap["kind"].(float64)),
					}
					if detail, ok := itemMap["detail"].(string); ok {
						completion.Detail = detail
					}
					items = append(items, completion)
				}
			}
		}
	}
	return items, nil
}

func (c *LSPClient) parseHoverInfo(result interface{}) (*models.HoverInfo, error) {
	hover := &models.HoverInfo{}
	if resultMap, ok := result.(map[string]interface{}); ok {
		if contents, ok := resultMap["contents"].(map[string]interface{}); ok {
			if value, ok := contents["value"].(string); ok {
				hover.Content = value
			}
		}
	}
	return hover, nil
}

func (c *LSPClient) parseLocations(result interface{}) ([]*models.Location, error) {
	locations := []*models.Location{}

	if result == nil {
		return locations, nil
	}

	// Handle single location
	if locationMap, ok := result.(map[string]interface{}); ok {
		location, err := c.parseSingleLocation(locationMap)
		if err != nil {
			return nil, err
		}
		locations = append(locations, location)
		return locations, nil
	}

	// Handle array of locations
	if locationsArray, ok := result.([]interface{}); ok {
		for _, item := range locationsArray {
			if locationMap, ok := item.(map[string]interface{}); ok {
				location, err := c.parseSingleLocation(locationMap)
				if err != nil {
					continue // Skip invalid locations
				}
				locations = append(locations, location)
			}
		}
	}

	return locations, nil
}

func (c *LSPClient) parseSingleLocation(locationMap map[string]interface{}) (*models.Location, error) {
	location := &models.Location{}

	// Parse URI
	if uri, ok := locationMap["uri"].(string); ok {
		location.URI = uri
	}

	// Parse range
	if rangeData, ok := locationMap["range"].(map[string]interface{}); ok {
		rng, err := c.parseRange(rangeData)
		if err != nil {
			return nil, err
		}
		location.Range = *rng
	}

	return location, nil
}

func (c *LSPClient) parseRange(rangeData map[string]interface{}) (*models.Range, error) {
	rng := &models.Range{}

	if startData, ok := rangeData["start"].(map[string]interface{}); ok {
		start, err := c.parsePosition(startData)
		if err != nil {
			return nil, err
		}
		rng.Start = *start
	}

	if endData, ok := rangeData["end"].(map[string]interface{}); ok {
		end, err := c.parsePosition(endData)
		if err != nil {
			return nil, err
		}
		rng.End = *end
	}

	return rng, nil
}

func (c *LSPClient) parsePosition(posData map[string]interface{}) (*models.Position, error) {
	pos := &models.Position{}

	if line, ok := posData["line"].(float64); ok {
		pos.Line = int(line)
	}

	if character, ok := posData["character"].(float64); ok {
		pos.Character = int(character)
	}

	return pos, nil
}

func (c *LSPClient) parseSymbols(result interface{}) ([]*models.SymbolInfo, error) {
	symbols := []*models.SymbolInfo{}

	if result == nil {
		return symbols, nil
	}

	// Handle array of symbols
	if symbolsArray, ok := result.([]interface{}); ok {
		for _, item := range symbolsArray {
			if symbolMap, ok := item.(map[string]interface{}); ok {
				symbol, err := c.parseSingleSymbol(symbolMap)
				if err != nil {
					continue // Skip invalid symbols
				}
				symbols = append(symbols, symbol)
			}
		}
	}

	return symbols, nil
}

func (c *LSPClient) parseSingleSymbol(symbolMap map[string]interface{}) (*models.SymbolInfo, error) {
	symbol := &models.SymbolInfo{}

	if name, ok := symbolMap["name"].(string); ok {
		symbol.Name = name
	}

	if kind, ok := symbolMap["kind"].(float64); ok {
		symbol.Kind = int(kind)
	}

	if containerName, ok := symbolMap["containerName"].(string); ok {
		symbol.ContainerName = containerName
	}

	// Parse location
	if locationData, ok := symbolMap["location"].(map[string]interface{}); ok {
		location, err := c.parseSingleLocation(locationData)
		if err != nil {
			return nil, err
		}
		symbol.Location = *location
	}

	// Parse children recursively
	if childrenData, ok := symbolMap["children"].([]interface{}); ok {
		children := []*models.SymbolInfo{}
		for _, childItem := range childrenData {
			if childMap, ok := childItem.(map[string]interface{}); ok {
				child, err := c.parseSingleSymbol(childMap)
				if err != nil {
					continue
				}
				children = append(children, child)
			}
		}
		symbol.Children = children
	}

	return symbol, nil
}

func (c *LSPClient) supportsCapability(server *LSPServer, capability string) bool {
	_, exists := server.Capabilities[capability]
	return exists
}

func (c *LSPClient) readFileContent(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// sendMessage sends a JSON-RPC request and waits for response
func (c *LSPClient) sendMessage(server *LSPServer, message LSPMessage) (*LSPMessage, error) {
	data, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	if _, err := fmt.Fprintf(server.Stdin, "Content-Length: %d\r\n\r\n%s", len(data), data); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Wait for response on the notification channel
	timeout := time.After(30 * time.Second)
	for {
		select {
		case response := <-c.notificationChan:
			if response.ID != nil && response.ID == message.ID {
				return response, nil
			}
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for response")
		}
	}
}

// sendNotification sends a JSON-RPC notification
func (c *LSPClient) sendNotification(server *LSPServer, message LSPMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	_, err = fmt.Fprintf(server.Stdin, "Content-Length: %d\r\n\r\n%s", len(data), data)
	return err
}

// nextMessageID returns the next message ID
func (c *LSPClient) nextMessageID() int {
	id := c.messageID
	c.messageID++
	return id
}

// Shutdown gracefully shuts down the LSP server
func (c *LSPClient) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.server == nil {
		return nil
	}

	// Send shutdown request
	shutdownMsg := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "shutdown",
		Params:  map[string]interface{}{},
	}

	if _, err := c.sendMessage(c.server, shutdownMsg); err != nil {
		log.Printf("Shutdown request failed: %v", err)
	}

	// Send exit notification
	exitMsg := LSPMessage{
		JSONRPC: "2.0",
		Method:  "exit",
		Params:  map[string]interface{}{},
	}

	c.sendNotification(c.server, exitMsg)

	// Kill the process
	if c.server.Process != nil && c.server.Process.Process != nil {
		return c.server.Process.Process.Kill()
	}

	return nil
}

// HealthCheck performs health check on the LSP server
func (c *LSPClient) HealthCheck() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.server == nil || !c.server.Initialized {
		return fmt.Errorf("LSP server not initialized")
	}

	// Simple health check - could be enhanced
	c.server.LastHealth = time.Now()
	return nil
}
