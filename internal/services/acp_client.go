package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/models"
)

// LSPClient implements a real Language Server Protocol client
type LSPClient struct {
	servers      map[string]*LSPServerConnection
	capabilities map[string]*LSPCapabilities
	messageID    int
	mu           sync.RWMutex
	logger       *logrus.Logger
}

// LSPServerConnection represents a live connection to an LSP server
type LSPServerConnection struct {
	ID           string
	Name         string
	Language     string
	Transport    LSPTransport
	Capabilities *LSPCapabilities
	Workspace    string
	Connected    bool
	LastUsed     time.Time
	Files        map[string]*LSPFileInfo // URI -> file info
}

// LSPTransport defines the interface for LSP communication
type LSPTransport interface {
	Send(ctx context.Context, message interface{}) error
	Receive(ctx context.Context) (interface{}, error)
	Close() error
	IsConnected() bool
}

// StdioLSPTransport implements LSP transport over stdio
type StdioLSPTransport struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	scanner   *bufio.Scanner
	connected bool
	mu        sync.Mutex
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

// LSPCapabilities represents LSP server capabilities
type LSPCapabilities struct {
	TextDocumentSync                *TextDocumentSyncOptions `json:"textDocumentSync,omitempty"`
	CompletionProvider              *CompletionOptions       `json:"completionProvider,omitempty"`
	HoverProvider                   bool                     `json:"hoverProvider,omitempty"`
	SignatureHelpProvider           *SignatureHelpOptions    `json:"signatureHelpProvider,omitempty"`
	DefinitionProvider              bool                     `json:"definitionProvider,omitempty"`
	TypeDefinitionProvider          bool                     `json:"typeDefinitionProvider,omitempty"`
	ReferencesProvider              bool                     `json:"referencesProvider,omitempty"`
	DocumentSymbolProvider          bool                     `json:"documentSymbolProvider,omitempty"`
	CodeActionProvider              bool                     `json:"codeActionProvider,omitempty"`
	CodeLensProvider                *CodeLensOptions         `json:"codeLensProvider,omitempty"`
	DocumentFormattingProvider      bool                     `json:"documentFormattingProvider,omitempty"`
	DocumentRangeFormattingProvider bool                     `json:"documentRangeFormattingProvider,omitempty"`
	RenameProvider                  bool                     `json:"renameProvider,omitempty"`
}

// LSPFileInfo represents information about a file being edited
type LSPFileInfo struct {
	URI        string
	LanguageID string
	Version    int
	Content    string
	LastSync   time.Time
}

// LSP request/response types
type InitializeRequest struct {
	ProcessID    *int               `json:"processId,omitempty"`
	RootURI      string             `json:"rootUri,omitempty"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

type ClientCapabilities struct {
	TextDocument TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

type TextDocumentClientCapabilities struct {
	Completion CompletionCapability `json:"completion,omitempty"`
}

type CompletionCapability struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type InitializeResult struct {
	Capabilities LSPCapabilities `json:"capabilities"`
}

type TextDocumentSyncOptions struct {
	OpenClose bool `json:"openClose,omitempty"`
	Change    int  `json:"change,omitempty"`
}

type CompletionOptions struct {
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type SignatureHelpOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type CodeLensOptions struct {
	ResolveProvider bool `json:"resolveProvider,omitempty"`
}

// LSP operation request types
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type DefinitionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// Common LSP types
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength *int   `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// LSP response types
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type CompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind,omitempty"`
	Detail        string `json:"detail,omitempty"`
	Documentation string `json:"documentation,omitempty"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// NewLSPClient creates a new LSP client
func NewLSPClient(logger *logrus.Logger) *LSPClient {
	return &LSPClient{
		servers:      make(map[string]*LSPServerConnection),
		capabilities: make(map[string]*LSPCapabilities),
		messageID:    1,
		logger:       logger,
	}
}

// ConnectServer connects to an LSP server
func (c *LSPClient) ConnectServer(ctx context.Context, serverID, name, language, command string, args []string, workspace string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.servers[serverID]; exists {
		return fmt.Errorf("LSP server %s already connected", serverID)
	}

	// Create transport
	transport, err := c.createStdioTransport(command, args)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	connection := &LSPServerConnection{
		ID:        serverID,
		Name:      name,
		Language:  language,
		Transport: transport,
		Workspace: workspace,
		Connected: true,
		LastUsed:  time.Now(),
		Files:     make(map[string]*LSPFileInfo),
	}

	// Initialize the server
	if err := c.initializeServer(ctx, connection); err != nil {
		transport.Close()
		return fmt.Errorf("failed to initialize LSP server: %w", err)
	}

	c.servers[serverID] = connection
	c.logger.WithFields(logrus.Fields{
		"serverId": serverID,
		"language": language,
	}).Info("Connected to LSP server")

	return nil
}

// DisconnectServer disconnects from an LSP server
func (c *LSPClient) DisconnectServer(serverID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	connection, exists := c.servers[serverID]
	if !exists {
		return fmt.Errorf("LSP server %s not connected", serverID)
	}

	// Send shutdown request
	shutdownReq := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "shutdown",
		Params:  nil,
	}

	if err := connection.Transport.Send(context.Background(), shutdownReq); err != nil {
		c.logger.WithError(err).Warn("Failed to send shutdown request")
	}

	// Send exit notification
	exitNotification := LSPMessage{
		JSONRPC: "2.0",
		Method:  "exit",
		Params:  nil,
	}

	connection.Transport.Send(context.Background(), exitNotification)

	if err := connection.Transport.Close(); err != nil {
		c.logger.WithError(err).Warn("Error closing LSP transport")
	}

	delete(c.servers, serverID)

	c.logger.WithField("serverId", serverID).Info("Disconnected from LSP server")
	return nil
}

// OpenFile opens a file for LSP operations
func (c *LSPClient) OpenFile(ctx context.Context, serverID, uri, languageID, content string) error {
	c.mu.RLock()
	connection, exists := c.servers[serverID]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("LSP server %s not connected", serverID)
	}

	// Store file info
	fileInfo := &LSPFileInfo{
		URI:        uri,
		LanguageID: languageID,
		Version:    1,
		Content:    content,
		LastSync:   time.Now(),
	}
	connection.Files[uri] = fileInfo

	// Send didOpen notification
	didOpenParams := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    1,
			Text:       content,
		},
	}

	didOpenMsg := LSPMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params:  didOpenParams,
	}

	if err := connection.Transport.Send(ctx, didOpenMsg); err != nil {
		return fmt.Errorf("failed to send didOpen notification: %w", err)
	}

	connection.LastUsed = time.Now()
	return nil
}

// UpdateFile updates file content
func (c *LSPClient) UpdateFile(ctx context.Context, serverID, uri, content string) error {
	c.mu.RLock()
	connection, exists := c.servers[serverID]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("LSP server %s not connected", serverID)
	}

	fileInfo, exists := connection.Files[uri]
	if !exists {
		return fmt.Errorf("file %s not opened", uri)
	}

	fileInfo.Version++
	fileInfo.Content = content
	fileInfo.LastSync = time.Now()

	// Send didChange notification
	didChangeParams := DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     uri,
			Version: fileInfo.Version,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{
				Text: content,
			},
		},
	}

	didChangeMsg := LSPMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/didChange",
		Params:  didChangeParams,
	}

	if err := connection.Transport.Send(ctx, didChangeMsg); err != nil {
		return fmt.Errorf("failed to send didChange notification: %w", err)
	}

	connection.LastUsed = time.Now()
	return nil
}

// CloseFile closes a file
func (c *LSPClient) CloseFile(ctx context.Context, serverID, uri string) error {
	c.mu.RLock()
	connection, exists := c.servers[serverID]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("LSP server %s not connected", serverID)
	}

	if _, exists := connection.Files[uri]; !exists {
		return fmt.Errorf("file %s not opened", uri)
	}

	delete(connection.Files, uri)

	// Send didClose notification
	didCloseParams := map[string]interface{}{
		"textDocument": TextDocumentIdentifier{URI: uri},
	}

	didCloseMsg := LSPMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/didClose",
		Params:  didCloseParams,
	}

	if err := connection.Transport.Send(ctx, didCloseMsg); err != nil {
		return fmt.Errorf("failed to send didClose notification: %w", err)
	}

	connection.LastUsed = time.Now()
	return nil
}

// GetCompletion requests completion at a position
func (c *LSPClient) GetCompletion(ctx context.Context, serverID, uri string, line, character int) (*CompletionList, error) {
	c.mu.RLock()
	connection, exists := c.servers[serverID]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("LSP server %s not connected", serverID)
	}

	if connection.Capabilities.CompletionProvider == nil {
		return nil, fmt.Errorf("server does not support completion")
	}

	completionParams := CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	completionReq := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "textDocument/completion",
		Params:  completionParams,
	}

	if err := connection.Transport.Send(ctx, completionReq); err != nil {
		return nil, fmt.Errorf("failed to send completion request: %w", err)
	}

	response, err := connection.Transport.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive completion response: %w", err)
	}

	var completionMsg LSPMessage
	if err := c.unmarshalMessage(response, &completionMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal completion response: %w", err)
	}

	if completionMsg.Error != nil {
		return nil, fmt.Errorf("completion error: %s", completionMsg.Error.Message)
	}

	var completionList CompletionList
	if err := c.unmarshalResult(completionMsg.Result, &completionList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal completion result: %w", err)
	}

	connection.LastUsed = time.Now()
	return &completionList, nil
}

// GetHover requests hover information at a position
func (c *LSPClient) GetHover(ctx context.Context, serverID, uri string, line, character int) (*Hover, error) {
	c.mu.RLock()
	connection, exists := c.servers[serverID]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("LSP server %s not connected", serverID)
	}

	if !connection.Capabilities.HoverProvider {
		return nil, fmt.Errorf("server does not support hover")
	}

	hoverParams := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	hoverReq := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "textDocument/hover",
		Params:  hoverParams,
	}

	if err := connection.Transport.Send(ctx, hoverReq); err != nil {
		return nil, fmt.Errorf("failed to send hover request: %w", err)
	}

	response, err := connection.Transport.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive hover response: %w", err)
	}

	var hoverMsg LSPMessage
	if err := c.unmarshalMessage(response, &hoverMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hover response: %w", err)
	}

	if hoverMsg.Error != nil {
		return nil, fmt.Errorf("hover error: %s", hoverMsg.Error.Message)
	}

	var hover Hover
	if err := c.unmarshalResult(hoverMsg.Result, &hover); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hover result: %w", err)
	}

	connection.LastUsed = time.Now()
	return &hover, nil
}

// GetDefinition finds the definition of a symbol
func (c *LSPClient) GetDefinition(ctx context.Context, serverID, uri string, line, character int) (*Location, error) {
	c.mu.RLock()
	connection, exists := c.servers[serverID]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("LSP server %s not connected", serverID)
	}

	if !connection.Capabilities.DefinitionProvider {
		return nil, fmt.Errorf("server does not support definition")
	}

	definitionParams := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	definitionReq := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "textDocument/definition",
		Params:  definitionParams,
	}

	if err := connection.Transport.Send(ctx, definitionReq); err != nil {
		return nil, fmt.Errorf("failed to send definition request: %w", err)
	}

	response, err := connection.Transport.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive definition response: %w", err)
	}

	var definitionMsg LSPMessage
	if err := c.unmarshalMessage(response, &definitionMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal definition response: %w", err)
	}

	if definitionMsg.Error != nil {
		return nil, fmt.Errorf("definition error: %s", definitionMsg.Error.Message)
	}

	var location Location
	if err := c.unmarshalResult(definitionMsg.Result, &location); err != nil {
		return nil, fmt.Errorf("failed to unmarshal definition result: %w", err)
	}

	connection.LastUsed = time.Now()
	return &location, nil
}

// ListServers returns all connected LSP servers
func (c *LSPClient) ListServers() []*LSPServerConnection {
	c.mu.RLock()
	defer c.mu.RUnlock()

	servers := make([]*LSPServerConnection, 0, len(c.servers))
	for _, server := range c.servers {
		servers = append(servers, server)
	}

	return servers
}

// GetServerCapabilities returns capabilities for a server
func (c *LSPClient) GetServerCapabilities(serverID string) (*LSPCapabilities, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	connection, exists := c.servers[serverID]
	if !exists {
		return nil, fmt.Errorf("LSP server %s not connected", serverID)
	}

	return connection.Capabilities, nil
}

// HealthCheck performs health checks on all connected servers
func (c *LSPClient) HealthCheck(ctx context.Context) map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make(map[string]bool)
	for serverID, connection := range c.servers {
		results[serverID] = connection.Transport.IsConnected()
	}

	return results
}

// StartServer starts a default LSP server (for integration orchestrator)
func (c *LSPClient) StartServer(ctx context.Context) error {
	// Start a default Go LSP server
	return c.ConnectServer(ctx, "default-go", "gopls", "go", "gopls", []string{}, "/tmp")
}

// GetDiagnostics provides diagnostics for a file
func (c *LSPClient) GetDiagnostics(ctx context.Context, filePath string) ([]*models.Diagnostic, error) {
	// For this implementation, we'll return empty diagnostics
	// In a real implementation, this would query the LSP server for diagnostics
	return []*models.Diagnostic{}, nil
}

// GetCodeIntelligence provides code intelligence for a file
func (c *LSPClient) GetCodeIntelligence(ctx context.Context, filePath string, options map[string]interface{}) (*models.CodeIntelligence, error) {
	serverID := "default-go"
	uri := fmt.Sprintf("file://%s", filePath)

	// Open the file first
	content := "" // Would need to read file content
	if err := c.OpenFile(ctx, serverID, uri, "go", content); err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Get completions at position 0,0 as example
	completions, err := c.GetCompletion(ctx, serverID, uri, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get completions: %w", err)
	}

	// Get hover info
	hover, err := c.GetHover(ctx, serverID, uri, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get hover: %w", err)
	}

	// Get definition
	definition, err := c.GetDefinition(ctx, serverID, uri, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get definition: %w", err)
	}

	return &models.CodeIntelligence{
		FilePath:    filePath,
		Completions: convertCompletionList(completions),
		Hover:       convertHover(hover),
		Definitions: []*models.Location{convertLocation(definition)},
	}, nil
}

// Helper functions to convert LSP types to models types
func convertCompletionList(list *CompletionList) []*models.CompletionItem {
	if list == nil {
		return nil
	}
	items := make([]*models.CompletionItem, len(list.Items))
	for i, item := range list.Items {
		items[i] = &models.CompletionItem{
			Label:  item.Label,
			Kind:   item.Kind,
			Detail: item.Detail,
		}
	}
	return items
}

func convertHover(hover *Hover) *models.HoverInfo {
	if hover == nil {
		return nil
	}
	return &models.HoverInfo{
		Content: hover.Contents.Value,
	}
}

func convertLocation(loc *Location) *models.Location {
	if loc == nil {
		return nil
	}
	return &models.Location{
		URI: loc.URI,
		Range: models.Range{
			Start: models.Position{
				Line:      loc.Range.Start.Line,
				Character: loc.Range.Start.Character,
			},
			End: models.Position{
				Line:      loc.Range.End.Line,
				Character: loc.Range.End.Character,
			},
		},
	}
}

// Private methods

func (c *LSPClient) createStdioTransport(command string, args []string) (LSPTransport, error) {
	cmd := exec.Command(command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}

	return &StdioLSPTransport{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		scanner:   bufio.NewScanner(stdout),
		connected: true,
	}, nil
}

func (c *LSPClient) initializeServer(ctx context.Context, connection *LSPServerConnection) error {
	initializeParams := InitializeRequest{
		ProcessID: nil,
		RootURI:   fmt.Sprintf("file://%s", connection.Workspace),
		Capabilities: ClientCapabilities{
			TextDocument: TextDocumentClientCapabilities{
				Completion: CompletionCapability{
					DynamicRegistration: false,
				},
			},
		},
	}

	initializeReq := LSPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "initialize",
		Params:  initializeParams,
	}

	if err := connection.Transport.Send(ctx, initializeReq); err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	response, err := connection.Transport.Receive(ctx)
	if err != nil {
		return fmt.Errorf("failed to receive initialize response: %w", err)
	}

	var initializeMsg LSPMessage
	if err := c.unmarshalMessage(response, &initializeMsg); err != nil {
		return fmt.Errorf("failed to unmarshal initialize response: %w", err)
	}

	if initializeMsg.Error != nil {
		return fmt.Errorf("initialize failed: %s", initializeMsg.Error.Message)
	}

	var result InitializeResult
	if err := c.unmarshalResult(initializeMsg.Result, &result); err != nil {
		return fmt.Errorf("failed to unmarshal initialize result: %w", err)
	}

	connection.Capabilities = &result.Capabilities

	// Send initialized notification
	initializedNotification := LSPMessage{
		JSONRPC: "2.0",
		Method:  "initialized",
		Params:  map[string]interface{}{},
	}

	if err := connection.Transport.Send(ctx, initializedNotification); err != nil {
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}

	return nil
}

func (c *LSPClient) nextMessageID() int {
	c.messageID++
	return c.messageID
}

func (c *LSPClient) unmarshalMessage(data interface{}, message *LSPMessage) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, message)
}

func (c *LSPClient) unmarshalResult(result interface{}, target interface{}) error {
	jsonData, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}

// StdioLSPTransport implementation

func (t *StdioLSPTransport) Send(ctx context.Context, message interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return fmt.Errorf("transport not connected")
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// LSP uses Content-Length headers
	contentLength := len(jsonData)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", contentLength)

	if _, err := t.stdin.Write([]byte(header)); err != nil {
		t.connected = false
		return err
	}

	if _, err := t.stdin.Write(jsonData); err != nil {
		t.connected = false
		return err
	}

	return nil
}

func (t *StdioLSPTransport) Receive(ctx context.Context) (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil, fmt.Errorf("transport not connected")
	}

	// Read Content-Length header
	if !t.scanner.Scan() {
		t.connected = false
		if err := t.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}

	headerLine := t.scanner.Text()
	if !strings.HasPrefix(headerLine, "Content-Length: ") {
		return nil, fmt.Errorf("invalid LSP header: %s", headerLine)
	}

	// Parse content length (simplified - should handle parsing better)
	contentLengthStr := strings.TrimPrefix(headerLine, "Content-Length: ")
	contentLength := 0
	fmt.Sscanf(contentLengthStr, "%d", &contentLength)

	// Skip empty line
	if !t.scanner.Scan() {
		return nil, fmt.Errorf("expected empty line after header")
	}

	// Read content
	content := make([]byte, contentLength)
	if _, err := io.ReadFull(t.stdout, content); err != nil {
		t.connected = false
		return nil, err
	}

	var message interface{}
	if err := json.Unmarshal(content, &message); err != nil {
		return nil, err
	}

	return message, nil
}

func (t *StdioLSPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connected = false

	if t.stdin != nil {
		t.stdin.Close()
	}

	if t.cmd != nil && t.cmd.Process != nil {
		return t.cmd.Process.Kill()
	}

	return nil
}

func (t *StdioLSPTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}
