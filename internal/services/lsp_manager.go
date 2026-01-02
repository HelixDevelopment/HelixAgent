package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/database"
)

// LSPManager handles LSP (Language Server Protocol) operations
type LSPManager struct {
	repo        *database.ModelMetadataRepository
	cache       CacheInterface
	log         *logrus.Logger
	connections map[string]*LSPConnection
	servers     map[string]*LSPServer
	mu          sync.RWMutex
	messageID   int64
	config      *LSPConfig
}

// LSPConfig holds configuration for the LSP manager
type LSPConfig struct {
	// ServerConfigs maps server ID to configuration
	ServerConfigs map[string]LSPServerConfig `json:"serverConfigs"`
	// DefaultWorkspace is the default workspace path
	DefaultWorkspace string `json:"defaultWorkspace"`
	// RequestTimeout is the timeout for LSP requests
	RequestTimeout time.Duration `json:"requestTimeout"`
	// InitTimeout is the timeout for server initialization
	InitTimeout time.Duration `json:"initTimeout"`
	// EnableCaching enables response caching
	EnableCaching bool `json:"enableCaching"`
	// BinarySearchPaths lists paths to search for LSP binaries
	BinarySearchPaths []string `json:"binarySearchPaths"`
}

// LSPServerConfig holds configuration for a specific LSP server
type LSPServerConfig struct {
	Command      string   `json:"command"`
	Args         []string `json:"args"`
	RootURI      string   `json:"rootUri"`
	WorkspaceDir string   `json:"workspaceDir"`
	Enabled      bool     `json:"enabled"`
}

// DefaultLSPConfig returns the default LSP configuration
func DefaultLSPConfig() *LSPConfig {
	return &LSPConfig{
		ServerConfigs: map[string]LSPServerConfig{
			"gopls": {
				Command: "gopls",
				Args:    []string{"serve"},
				Enabled: true,
			},
			"rust-analyzer": {
				Command: "rust-analyzer",
				Args:    []string{},
				Enabled: true,
			},
			"pylsp": {
				Command: "pylsp",
				Args:    []string{},
				Enabled: true,
			},
			"ts-language-server": {
				Command: "typescript-language-server",
				Args:    []string{"--stdio"},
				Enabled: true,
			},
		},
		DefaultWorkspace:  "/workspace",
		RequestTimeout:    30 * time.Second,
		InitTimeout:       10 * time.Second,
		EnableCaching:     true,
		BinarySearchPaths: []string{"/usr/bin", "/usr/local/bin", "/opt/bin"},
	}
}

// LSPConnection represents a live connection to an LSP server process
type LSPConnection struct {
	ServerID     string
	Server       *LSPServer
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       io.ReadCloser
	stderr       io.ReadCloser
	scanner      *bufio.Reader
	connected    bool
	initialized  bool
	capabilities *LSPServerCapabilities
	mu           sync.Mutex
	lastUsed     time.Time
}

// LSPServerCapabilities represents the capabilities of an LSP server
type LSPServerCapabilities struct {
	CompletionProvider     bool `json:"completionProvider"`
	HoverProvider          bool `json:"hoverProvider"`
	DefinitionProvider     bool `json:"definitionProvider"`
	ReferencesProvider     bool `json:"referencesProvider"`
	DiagnosticProvider     bool `json:"diagnosticProvider"`
	CodeActionProvider     bool `json:"codeActionProvider"`
	DocumentFormattingProvider bool `json:"documentFormattingProvider"`
}

// LSPServer represents an LSP server configuration
type LSPServer struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Language     string          `json:"language"`
	Command      string          `json:"command"`
	Args         []string        `json:"args,omitempty"`
	Enabled      bool            `json:"enabled"`
	Workspace    string          `json:"workspace"`
	LastSync     *time.Time      `json:"lastSync"`
	Capabilities []LSPCapability `json:"capabilities"`
	BinaryPath   string          `json:"binaryPath,omitempty"`
	Available    bool            `json:"available"`
}

// LSPCapability represents a capability of an LSP server
type LSPCapability struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// LSPRequest represents a request to an LSP server
type LSPRequest struct {
	ServerID string                 `json:"serverId"`
	Method   string                 `json:"method"`
	Params   map[string]interface{} `json:"params"`
	Text     string                 `json:"text,omitempty"`
	FileURI  string                 `json:"fileUri,omitempty"`
	Position LSPPosition            `json:"position,omitempty"`
}

// LSPPosition represents a position in a file for LSP operations
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// LSPResponse represents a response from an LSP server
type LSPResponse struct {
	Success   bool        `json:"success"`
	Result    interface{} `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// JSON-RPC 2.0 message types for LSP

// LSPJSONRPCRequest represents a JSON-RPC 2.0 request
type LSPJSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// LSPJSONRPCResponse represents a JSON-RPC 2.0 response
type LSPJSONRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      int64            `json:"id,omitempty"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *LSPJSONRPCError `json:"error,omitempty"`
}

// LSPJSONRPCError represents a JSON-RPC 2.0 error
type LSPJSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// LSPJSONRPCNotification represents a JSON-RPC 2.0 notification (no ID)
type LSPJSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// LSP initialize params
type LSPInitializeParams struct {
	ProcessID             int                      `json:"processId"`
	RootURI               string                   `json:"rootUri"`
	Capabilities          LSPClientCapabilities    `json:"capabilities"`
	InitializationOptions map[string]interface{}   `json:"initializationOptions,omitempty"`
	WorkspaceFolders      []LSPWorkspaceFolder     `json:"workspaceFolders,omitempty"`
}

// LSPClientCapabilities represents client capabilities
type LSPClientCapabilities struct {
	TextDocument *LSPTextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Workspace    *LSPWorkspaceClientCapabilities    `json:"workspace,omitempty"`
}

// LSPTextDocumentClientCapabilities represents text document capabilities
type LSPTextDocumentClientCapabilities struct {
	Completion    *LSPCompletionClientCapabilities    `json:"completion,omitempty"`
	Hover         *LSPHoverClientCapabilities         `json:"hover,omitempty"`
	Definition    *LSPDefinitionClientCapabilities    `json:"definition,omitempty"`
	References    *LSPReferencesClientCapabilities    `json:"references,omitempty"`
	CodeAction    *LSPCodeActionClientCapabilities    `json:"codeAction,omitempty"`
	Diagnostics   *LSPDiagnosticsClientCapabilities   `json:"diagnostics,omitempty"`
}

// LSPCompletionClientCapabilities represents completion capabilities
type LSPCompletionClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	CompletionItem      *LSPCompletionItemCapabilities `json:"completionItem,omitempty"`
}

// LSPCompletionItemCapabilities represents completion item capabilities
type LSPCompletionItemCapabilities struct {
	SnippetSupport          bool `json:"snippetSupport,omitempty"`
	CommitCharactersSupport bool `json:"commitCharactersSupport,omitempty"`
	DocumentationFormat     []string `json:"documentationFormat,omitempty"`
}

// LSPHoverClientCapabilities represents hover capabilities
type LSPHoverClientCapabilities struct {
	DynamicRegistration bool     `json:"dynamicRegistration,omitempty"`
	ContentFormat       []string `json:"contentFormat,omitempty"`
}

// LSPDefinitionClientCapabilities represents definition capabilities
type LSPDefinitionClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

// LSPReferencesClientCapabilities represents references capabilities
type LSPReferencesClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// LSPCodeActionClientCapabilities represents code action capabilities
type LSPCodeActionClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// LSPDiagnosticsClientCapabilities represents diagnostics capabilities
type LSPDiagnosticsClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// LSPWorkspaceClientCapabilities represents workspace capabilities
type LSPWorkspaceClientCapabilities struct {
	ApplyEdit              bool `json:"applyEdit,omitempty"`
	WorkspaceFolders       bool `json:"workspaceFolders,omitempty"`
	Configuration          bool `json:"configuration,omitempty"`
}

// LSPWorkspaceFolder represents a workspace folder
type LSPWorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// LSPInitializeResult represents the result of initialize
type LSPInitializeResult struct {
	Capabilities LSPServerCapabilitiesResult `json:"capabilities"`
	ServerInfo   *LSPServerInfo              `json:"serverInfo,omitempty"`
}

// LSPServerCapabilitiesResult represents server capabilities from initialize
type LSPServerCapabilitiesResult struct {
	CompletionProvider          interface{} `json:"completionProvider,omitempty"`
	HoverProvider               interface{} `json:"hoverProvider,omitempty"`
	DefinitionProvider          interface{} `json:"definitionProvider,omitempty"`
	ReferencesProvider          interface{} `json:"referencesProvider,omitempty"`
	DiagnosticProvider          interface{} `json:"diagnosticProvider,omitempty"`
	CodeActionProvider          interface{} `json:"codeActionProvider,omitempty"`
	DocumentFormattingProvider  interface{} `json:"documentFormattingProvider,omitempty"`
	TextDocumentSync            interface{} `json:"textDocumentSync,omitempty"`
}

// LSPServerInfo represents server info from initialize
type LSPServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// LSPTextDocumentIdentifier identifies a text document
type LSPTextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// LSPTextDocumentPositionParams represents position params
type LSPTextDocumentPositionParams struct {
	TextDocument LSPTextDocumentIdentifier `json:"textDocument"`
	Position     LSPPosition               `json:"position"`
}

// LSPRange represents a range in a document
type LSPRange struct {
	Start LSPPosition `json:"start"`
	End   LSPPosition `json:"end"`
}

// LSPDiagnostic represents a diagnostic
type LSPDiagnostic struct {
	Range    LSPRange `json:"range"`
	Severity int      `json:"severity,omitempty"`
	Code     string   `json:"code,omitempty"`
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
}

// LSPTextDocumentItem represents a text document
type LSPTextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// LSPDidOpenTextDocumentParams represents didOpen params
type LSPDidOpenTextDocumentParams struct {
	TextDocument LSPTextDocumentItem `json:"textDocument"`
}

// NewLSPManager creates a new LSP manager
func NewLSPManager(repo *database.ModelMetadataRepository, cache CacheInterface, log *logrus.Logger) *LSPManager {
	return NewLSPManagerWithConfig(repo, cache, log, DefaultLSPConfig())
}

// NewLSPManagerWithConfig creates a new LSP manager with custom config
func NewLSPManagerWithConfig(repo *database.ModelMetadataRepository, cache CacheInterface, log *logrus.Logger, config *LSPConfig) *LSPManager {
	if config == nil {
		config = DefaultLSPConfig()
	}

	m := &LSPManager{
		repo:        repo,
		cache:       cache,
		log:         log,
		connections: make(map[string]*LSPConnection),
		servers:     make(map[string]*LSPServer),
		config:      config,
	}

	// Initialize server configurations
	m.initializeServers()

	return m
}

// initializeServers sets up the known LSP server configurations
func (m *LSPManager) initializeServers() {
	serverDefs := []struct {
		id       string
		name     string
		language string
		command  string
		args     []string
		caps     []LSPCapability
	}{
		{
			id:       "gopls",
			name:     "Go Language Server",
			language: "go",
			command:  "gopls",
			args:     []string{"serve"},
			caps: []LSPCapability{
				{Name: "completion", Description: "Code completion"},
				{Name: "diagnostics", Description: "Code diagnostics"},
				{Name: "hover", Description: "Hover information"},
				{Name: "definition", Description: "Go to definition"},
				{Name: "references", Description: "Find references"},
			},
		},
		{
			id:       "rust-analyzer",
			name:     "Rust Analyzer",
			language: "rust",
			command:  "rust-analyzer",
			args:     []string{},
			caps: []LSPCapability{
				{Name: "completion", Description: "Code completion"},
				{Name: "diagnostics", Description: "Code diagnostics"},
				{Name: "hover", Description: "Hover information"},
			},
		},
		{
			id:       "pylsp",
			name:     "Python Language Server",
			language: "python",
			command:  "pylsp",
			args:     []string{},
			caps: []LSPCapability{
				{Name: "completion", Description: "Code completion"},
				{Name: "diagnostics", Description: "Code diagnostics"},
				{Name: "hover", Description: "Hover information"},
				{Name: "definition", Description: "Go to definition"},
			},
		},
		{
			id:       "ts-language-server",
			name:     "TypeScript Language Server",
			language: "typescript",
			command:  "typescript-language-server",
			args:     []string{"--stdio"},
			caps: []LSPCapability{
				{Name: "completion", Description: "Code completion"},
				{Name: "diagnostics", Description: "Code diagnostics"},
				{Name: "hover", Description: "Hover information"},
				{Name: "definition", Description: "Go to definition"},
				{Name: "references", Description: "Find references"},
			},
		},
	}

	for _, def := range serverDefs {
		binaryPath, available := m.findBinary(def.command)

		server := &LSPServer{
			ID:           def.id,
			Name:         def.name,
			Language:     def.language,
			Command:      def.command,
			Args:         def.args,
			Enabled:      true,
			Workspace:    m.config.DefaultWorkspace,
			Capabilities: def.caps,
			BinaryPath:   binaryPath,
			Available:    available,
		}

		// Override with config if present
		if cfg, ok := m.config.ServerConfigs[def.id]; ok {
			server.Enabled = cfg.Enabled
			if cfg.Command != "" {
				server.Command = cfg.Command
			}
			if len(cfg.Args) > 0 {
				server.Args = cfg.Args
			}
			if cfg.WorkspaceDir != "" {
				server.Workspace = cfg.WorkspaceDir
			}
		}

		m.servers[def.id] = server
	}
}

// findBinary searches for an LSP binary in common paths
func (m *LSPManager) findBinary(command string) (string, bool) {
	// First check if it's an absolute path
	if filepath.IsAbs(command) {
		if fileExists(command) {
			return command, true
		}
		return "", false
	}

	// Check in PATH using exec.LookPath
	if path, err := exec.LookPath(command); err == nil {
		return path, true
	}

	// Check in configured search paths
	for _, searchPath := range m.config.BinarySearchPaths {
		fullPath := filepath.Join(searchPath, command)
		if fileExists(fullPath) {
			return fullPath, true
		}
	}

	return "", false
}

// fileExists checks if a file exists (can be overridden for testing)
var fileExists = func(path string) bool {
	_, err := exec.LookPath(path)
	return err == nil
}

// SetFileExistsFunc sets the file existence check function (for testing)
func (m *LSPManager) SetFileExistsFunc(fn func(string) bool) {
	fileExists = fn
}

// ListLSPServers lists all configured LSP servers
func (m *LSPManager) ListLSPServers(ctx context.Context) ([]LSPServer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	servers := make([]LSPServer, 0, len(m.servers))
	for _, server := range m.servers {
		servers = append(servers, *server)
	}

	m.log.WithField("count", len(servers)).Info("Listed LSP servers")
	return servers, nil
}

// GetLSPServer gets a specific LSP server by ID
func (m *LSPManager) GetLSPServer(ctx context.Context, serverID string) (*LSPServer, error) {
	m.mu.RLock()
	server, exists := m.servers[serverID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("LSP server %s not found", serverID)
	}

	return server, nil
}

// getOrCreateConnection gets an existing connection or creates a new one
func (m *LSPManager) getOrCreateConnection(ctx context.Context, serverID string) (*LSPConnection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for existing connection
	if conn, exists := m.connections[serverID]; exists && conn.connected {
		conn.lastUsed = time.Now()
		return conn, nil
	}

	// Get server config
	server, exists := m.servers[serverID]
	if !exists {
		return nil, fmt.Errorf("LSP server %s not found", serverID)
	}

	if !server.Enabled {
		return nil, fmt.Errorf("LSP server %s is not enabled", serverID)
	}

	if !server.Available {
		return nil, fmt.Errorf("LSP server %s binary not available: %s not found in PATH", serverID, server.Command)
	}

	// Start the server
	conn, err := m.startServer(ctx, server)
	if err != nil {
		return nil, fmt.Errorf("failed to start LSP server %s: %w", serverID, err)
	}

	m.connections[serverID] = conn
	return conn, nil
}

// startServer starts an LSP server process
func (m *LSPManager) startServer(ctx context.Context, server *LSPServer) (*LSPConnection, error) {
	command := server.Command
	if server.BinaryPath != "" {
		command = server.BinaryPath
	}

	cmd := exec.CommandContext(ctx, command, server.Args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("failed to start LSP server process: %w", err)
	}

	conn := &LSPConnection{
		ServerID:  server.ID,
		Server:    server,
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		stderr:    stderr,
		scanner:   bufio.NewReader(stdout),
		connected: true,
		lastUsed:  time.Now(),
	}

	// Initialize the server
	initCtx, cancel := context.WithTimeout(ctx, m.config.InitTimeout)
	defer cancel()

	if err := m.initializeConnection(initCtx, conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize LSP server: %w", err)
	}

	m.log.WithFields(logrus.Fields{
		"serverId": server.ID,
		"command":  command,
	}).Info("Started LSP server")

	return conn, nil
}

// initializeConnection sends the initialize request to the LSP server
func (m *LSPManager) initializeConnection(ctx context.Context, conn *LSPConnection) error {
	rootURI := "file://" + conn.Server.Workspace

	initParams := LSPInitializeParams{
		ProcessID: 0, // null process ID
		RootURI:   rootURI,
		Capabilities: LSPClientCapabilities{
			TextDocument: &LSPTextDocumentClientCapabilities{
				Completion: &LSPCompletionClientCapabilities{
					DynamicRegistration: false,
					CompletionItem: &LSPCompletionItemCapabilities{
						SnippetSupport:          true,
						CommitCharactersSupport: true,
						DocumentationFormat:     []string{"markdown", "plaintext"},
					},
				},
				Hover: &LSPHoverClientCapabilities{
					DynamicRegistration: false,
					ContentFormat:       []string{"markdown", "plaintext"},
				},
				Definition: &LSPDefinitionClientCapabilities{
					DynamicRegistration: false,
					LinkSupport:         true,
				},
				References: &LSPReferencesClientCapabilities{
					DynamicRegistration: false,
				},
				CodeAction: &LSPCodeActionClientCapabilities{
					DynamicRegistration: false,
				},
				Diagnostics: &LSPDiagnosticsClientCapabilities{
					DynamicRegistration: false,
				},
			},
			Workspace: &LSPWorkspaceClientCapabilities{
				ApplyEdit:        true,
				WorkspaceFolders: true,
				Configuration:    true,
			},
		},
		WorkspaceFolders: []LSPWorkspaceFolder{
			{
				URI:  rootURI,
				Name: filepath.Base(conn.Server.Workspace),
			},
		},
	}

	// Send initialize request
	response, err := m.sendRequest(ctx, conn, "initialize", initParams)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	// Parse capabilities
	if response.Result != nil {
		var initResult LSPInitializeResult
		resultBytes, _ := json.Marshal(response.Result)
		if err := json.Unmarshal(resultBytes, &initResult); err == nil {
			conn.capabilities = &LSPServerCapabilities{
				CompletionProvider:         initResult.Capabilities.CompletionProvider != nil,
				HoverProvider:              initResult.Capabilities.HoverProvider != nil,
				DefinitionProvider:         initResult.Capabilities.DefinitionProvider != nil,
				ReferencesProvider:         initResult.Capabilities.ReferencesProvider != nil,
				DiagnosticProvider:         initResult.Capabilities.DiagnosticProvider != nil,
				CodeActionProvider:         initResult.Capabilities.CodeActionProvider != nil,
				DocumentFormattingProvider: initResult.Capabilities.DocumentFormattingProvider != nil,
			}
		}
	}

	// Send initialized notification
	if err := m.sendNotification(conn, "initialized", map[string]interface{}{}); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	conn.initialized = true
	return nil
}

// nextMessageID generates the next message ID atomically
func (m *LSPManager) nextMessageID() int64 {
	return atomic.AddInt64(&m.messageID, 1)
}

// sendRequest sends a JSON-RPC request and waits for response
func (m *LSPManager) sendRequest(ctx context.Context, conn *LSPConnection, method string, params interface{}) (*LSPJSONRPCResponse, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if !conn.connected {
		return nil, fmt.Errorf("connection is closed")
	}

	request := LSPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      m.nextMessageID(),
		Method:  method,
		Params:  params,
	}

	// Encode and send
	if err := m.writeMessage(conn, request); err != nil {
		conn.connected = false
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response with timeout
	responseChan := make(chan *LSPJSONRPCResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		response, err := m.readMessage(conn)
		if err != nil {
			errChan <- err
			return
		}
		responseChan <- response
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errChan:
		return nil, err
	case response := <-responseChan:
		if response.Error != nil {
			return response, fmt.Errorf("LSP error %d: %s", response.Error.Code, response.Error.Message)
		}
		return response, nil
	}
}

// sendNotification sends a JSON-RPC notification (no response expected)
func (m *LSPManager) sendNotification(conn *LSPConnection, method string, params interface{}) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if !conn.connected {
		return fmt.Errorf("connection is closed")
	}

	notification := LSPJSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	return m.writeMessage(conn, notification)
}

// writeMessage writes a JSON-RPC message with Content-Length header
func (m *LSPManager) writeMessage(conn *LSPConnection, message interface{}) error {
	content, err := json.Marshal(message)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))
	fullMessage := header + string(content)

	_, err = conn.stdin.Write([]byte(fullMessage))
	return err
}

// readMessage reads a JSON-RPC message with Content-Length header
func (m *LSPManager) readMessage(conn *LSPConnection) (*LSPJSONRPCResponse, error) {
	// Read headers
	var contentLength int
	for {
		line, err := conn.scanner.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read header: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break // End of headers
		}

		if strings.HasPrefix(line, "Content-Length:") {
			_, err := fmt.Sscanf(line, "Content-Length: %d", &contentLength)
			if err != nil {
				return nil, fmt.Errorf("failed to parse Content-Length: %w", err)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing or invalid Content-Length header")
	}

	// Read content
	content := make([]byte, contentLength)
	_, err := io.ReadFull(conn.scanner, content)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	var response LSPJSONRPCResponse
	if err := json.Unmarshal(content, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// ExecuteLSPRequest executes a method on an LSP server
func (m *LSPManager) ExecuteLSPRequest(ctx context.Context, req LSPRequest) (*LSPResponse, error) {
	m.log.WithFields(logrus.Fields{
		"serverId": req.ServerID,
		"method":   req.Method,
	}).Info("Executing LSP request")

	// Try to get or create connection
	conn, err := m.getOrCreateConnection(ctx, req.ServerID)
	if err != nil {
		// Graceful degradation: return error response when server unavailable
		m.log.WithFields(logrus.Fields{
			"serverId": req.ServerID,
			"error":    err.Error(),
		}).Warn("LSP server unavailable, returning error")

		return &LSPResponse{
			Timestamp: time.Now(),
			Success:   false,
			Error:     fmt.Sprintf("LSP server unavailable: %v", err),
		}, nil
	}

	// Send the request
	timeout := m.config.RequestTimeout
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	response, err := m.sendRequest(reqCtx, conn, req.Method, req.Params)
	if err != nil {
		return &LSPResponse{
			Timestamp: time.Now(),
			Success:   false,
			Error:     fmt.Sprintf("LSP request failed: %v", err),
		}, nil
	}

	// Cache the response if caching is enabled
	if m.config.EnableCaching && m.cache != nil {
		cacheKey := fmt.Sprintf("lsp_response_%s_%s", req.ServerID, req.Method)
		m.log.WithField("cacheKey", cacheKey).Debug("Cached LSP response")
	}

	return &LSPResponse{
		Timestamp: time.Now(),
		Success:   true,
		Result:    response.Result,
	}, nil
}

// openDocument sends textDocument/didOpen notification
func (m *LSPManager) openDocument(ctx context.Context, conn *LSPConnection, fileURI, languageID, text string) error {
	params := LSPDidOpenTextDocumentParams{
		TextDocument: LSPTextDocumentItem{
			URI:        fileURI,
			LanguageID: languageID,
			Version:    1,
			Text:       text,
		},
	}

	return m.sendNotification(conn, "textDocument/didOpen", params)
}

// GetDiagnostics gets diagnostics for a file from an LSP server
func (m *LSPManager) GetDiagnostics(ctx context.Context, serverID, fileURI string) (interface{}, error) {
	m.log.WithFields(logrus.Fields{
		"serverId": serverID,
		"fileUri":  fileURI,
	}).Info("Getting LSP diagnostics")

	conn, err := m.getOrCreateConnection(ctx, serverID)
	if err != nil {
		// Graceful degradation
		return map[string]interface{}{
			"serverId":    serverID,
			"fileUri":     fileURI,
			"diagnostics": []interface{}{},
			"error":       fmt.Sprintf("LSP server unavailable: %v", err),
			"timestamp":   time.Now(),
		}, nil
	}

	// For diagnostics, we typically need to request them via textDocument/diagnostic
	// or receive them via notifications after opening a document
	timeout := m.config.RequestTimeout
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := map[string]interface{}{
		"textDocument": LSPTextDocumentIdentifier{URI: fileURI},
	}

	response, err := m.sendRequest(reqCtx, conn, "textDocument/diagnostic", params)
	if err != nil {
		// Some servers don't support pull diagnostics, return empty
		return map[string]interface{}{
			"serverId":    serverID,
			"fileUri":     fileURI,
			"diagnostics": []interface{}{},
			"error":       fmt.Sprintf("Diagnostics request failed: %v", err),
			"timestamp":   time.Now(),
		}, nil
	}

	return map[string]interface{}{
		"serverId":    serverID,
		"fileUri":     fileURI,
		"diagnostics": response.Result,
		"timestamp":   time.Now(),
	}, nil
}

// GetCodeActions gets code actions for a position from an LSP server
func (m *LSPManager) GetCodeActions(ctx context.Context, serverID, text, fileURI string, position LSPPosition) (interface{}, error) {
	m.log.WithFields(logrus.Fields{
		"serverId": serverID,
		"fileUri":  fileURI,
		"line":     position.Line,
	}).Info("Getting LSP code actions")

	conn, err := m.getOrCreateConnection(ctx, serverID)
	if err != nil {
		// Graceful degradation
		return map[string]interface{}{
			"serverId":  serverID,
			"fileUri":   fileURI,
			"actions":   []interface{}{},
			"error":     fmt.Sprintf("LSP server unavailable: %v", err),
			"timestamp": time.Now(),
		}, nil
	}

	timeout := m.config.RequestTimeout
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := map[string]interface{}{
		"textDocument": LSPTextDocumentIdentifier{URI: fileURI},
		"range": LSPRange{
			Start: position,
			End:   position,
		},
		"context": map[string]interface{}{
			"diagnostics": []interface{}{},
		},
	}

	response, err := m.sendRequest(reqCtx, conn, "textDocument/codeAction", params)
	if err != nil {
		return map[string]interface{}{
			"serverId":  serverID,
			"fileUri":   fileURI,
			"actions":   []interface{}{},
			"error":     fmt.Sprintf("Code action request failed: %v", err),
			"timestamp": time.Now(),
		}, nil
	}

	return map[string]interface{}{
		"serverId":  serverID,
		"fileUri":   fileURI,
		"actions":   response.Result,
		"timestamp": time.Now(),
	}, nil
}

// GetCompletion gets completion suggestions from an LSP server
func (m *LSPManager) GetCompletion(ctx context.Context, serverID, text, fileURI string, position LSPPosition) (interface{}, error) {
	m.log.WithFields(logrus.Fields{
		"serverId": serverID,
		"fileUri":  fileURI,
		"text":     text,
	}).Info("Getting LSP completion")

	conn, err := m.getOrCreateConnection(ctx, serverID)
	if err != nil {
		// Graceful degradation
		return map[string]interface{}{
			"serverId":    serverID,
			"fileUri":     fileURI,
			"completions": []interface{}{},
			"error":       fmt.Sprintf("LSP server unavailable: %v", err),
			"timestamp":   time.Now(),
		}, nil
	}

	timeout := m.config.RequestTimeout
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := LSPTextDocumentPositionParams{
		TextDocument: LSPTextDocumentIdentifier{URI: fileURI},
		Position:     position,
	}

	response, err := m.sendRequest(reqCtx, conn, "textDocument/completion", params)
	if err != nil {
		return map[string]interface{}{
			"serverId":    serverID,
			"fileUri":     fileURI,
			"completions": []interface{}{},
			"error":       fmt.Sprintf("Completion request failed: %v", err),
			"timestamp":   time.Now(),
		}, nil
	}

	return map[string]interface{}{
		"serverId":    serverID,
		"fileUri":     fileURI,
		"completions": response.Result,
		"timestamp":   time.Now(),
	}, nil
}

// GetHover gets hover information for a position from an LSP server
func (m *LSPManager) GetHover(ctx context.Context, serverID, fileURI string, line, character int) (interface{}, error) {
	m.log.WithFields(logrus.Fields{
		"serverId":  serverID,
		"fileUri":   fileURI,
		"line":      line,
		"character": character,
	}).Info("Getting LSP hover information")

	// Validate server exists
	if _, err := m.GetLSPServer(ctx, serverID); err != nil {
		return nil, err
	}

	// Validate file URI
	if fileURI == "" {
		return nil, fmt.Errorf("fileURI is required for hover")
	}

	conn, err := m.getOrCreateConnection(ctx, serverID)
	if err != nil {
		// Graceful degradation
		return map[string]interface{}{
			"serverId": serverID,
			"fileUri":  fileURI,
			"position": map[string]int{
				"line":      line,
				"character": character,
			},
			"contents":  nil,
			"error":     fmt.Sprintf("LSP server unavailable: %v", err),
			"timestamp": time.Now(),
		}, nil
	}

	timeout := m.config.RequestTimeout
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := LSPTextDocumentPositionParams{
		TextDocument: LSPTextDocumentIdentifier{URI: fileURI},
		Position: LSPPosition{
			Line:      line,
			Character: character,
		},
	}

	response, err := m.sendRequest(reqCtx, conn, "textDocument/hover", params)
	if err != nil {
		return map[string]interface{}{
			"serverId": serverID,
			"fileUri":  fileURI,
			"position": map[string]int{
				"line":      line,
				"character": character,
			},
			"contents":  nil,
			"error":     fmt.Sprintf("Hover request failed: %v", err),
			"timestamp": time.Now(),
		}, nil
	}

	return map[string]interface{}{
		"serverId": serverID,
		"fileUri":  fileURI,
		"position": map[string]int{
			"line":      line,
			"character": character,
		},
		"contents":  response.Result,
		"timestamp": time.Now(),
	}, nil
}

// GetDefinition gets the definition location for a symbol from an LSP server
func (m *LSPManager) GetDefinition(ctx context.Context, serverID, fileURI string, line, character int) (interface{}, error) {
	m.log.WithFields(logrus.Fields{
		"serverId":  serverID,
		"fileUri":   fileURI,
		"line":      line,
		"character": character,
	}).Info("Getting LSP definition")

	// Validate server exists
	if _, err := m.GetLSPServer(ctx, serverID); err != nil {
		return nil, err
	}

	// Validate file URI
	if fileURI == "" {
		return nil, fmt.Errorf("fileURI is required for definition")
	}

	conn, err := m.getOrCreateConnection(ctx, serverID)
	if err != nil {
		// Graceful degradation
		return map[string]interface{}{
			"serverId":  serverID,
			"fileUri":   fileURI,
			"location":  nil,
			"error":     fmt.Sprintf("LSP server unavailable: %v", err),
			"timestamp": time.Now(),
		}, nil
	}

	timeout := m.config.RequestTimeout
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := LSPTextDocumentPositionParams{
		TextDocument: LSPTextDocumentIdentifier{URI: fileURI},
		Position: LSPPosition{
			Line:      line,
			Character: character,
		},
	}

	response, err := m.sendRequest(reqCtx, conn, "textDocument/definition", params)
	if err != nil {
		return map[string]interface{}{
			"serverId":  serverID,
			"fileUri":   fileURI,
			"location":  nil,
			"error":     fmt.Sprintf("Definition request failed: %v", err),
			"timestamp": time.Now(),
		}, nil
	}

	return map[string]interface{}{
		"serverId":  serverID,
		"fileUri":   fileURI,
		"location":  response.Result,
		"timestamp": time.Now(),
	}, nil
}

// GetReferences gets all references to a symbol from an LSP server
func (m *LSPManager) GetReferences(ctx context.Context, serverID, fileURI string, line, character int) (interface{}, error) {
	m.log.WithFields(logrus.Fields{
		"serverId":  serverID,
		"fileUri":   fileURI,
		"line":      line,
		"character": character,
	}).Info("Getting LSP references")

	// Validate server exists
	if _, err := m.GetLSPServer(ctx, serverID); err != nil {
		return nil, err
	}

	// Validate file URI
	if fileURI == "" {
		return nil, fmt.Errorf("fileURI is required for references")
	}

	conn, err := m.getOrCreateConnection(ctx, serverID)
	if err != nil {
		// Graceful degradation
		return map[string]interface{}{
			"serverId":   serverID,
			"fileUri":    fileURI,
			"references": []interface{}{},
			"error":      fmt.Sprintf("LSP server unavailable: %v", err),
			"timestamp":  time.Now(),
		}, nil
	}

	timeout := m.config.RequestTimeout
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := map[string]interface{}{
		"textDocument": LSPTextDocumentIdentifier{URI: fileURI},
		"position": LSPPosition{
			Line:      line,
			Character: character,
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	}

	response, err := m.sendRequest(reqCtx, conn, "textDocument/references", params)
	if err != nil {
		return map[string]interface{}{
			"serverId":   serverID,
			"fileUri":    fileURI,
			"references": []interface{}{},
			"error":      fmt.Sprintf("References request failed: %v", err),
			"timestamp":  time.Now(),
		}, nil
	}

	return map[string]interface{}{
		"serverId":   serverID,
		"fileUri":    fileURI,
		"references": response.Result,
		"timestamp":  time.Now(),
	}, nil
}

// ValidateLSPRequest validates an LSP request
func (m *LSPManager) ValidateLSPRequest(ctx context.Context, req LSPRequest) error {
	// Basic validation
	if req.ServerID == "" {
		return fmt.Errorf("server ID is required")
	}

	if req.Method == "" {
		return fmt.Errorf("method is required")
	}

	// Check if server exists
	server, err := m.GetLSPServer(ctx, req.ServerID)
	if err != nil {
		return fmt.Errorf("invalid server ID: %w", err)
	}

	if !server.Enabled {
		return fmt.Errorf("server %s is not enabled", req.ServerID)
	}

	return nil // Validation passed
}

// SyncLSPServer synchronizes configuration with an LSP server
func (m *LSPManager) SyncLSPServer(ctx context.Context, serverID string) error {
	return m.refreshLSPServer(ctx, serverID)
}

// GetLSPStats returns statistics about LSP usage
func (m *LSPManager) GetLSPStats(ctx context.Context) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	servers := make([]LSPServer, 0, len(m.servers))
	for _, server := range m.servers {
		servers = append(servers, *server)
	}

	enabledCount := 0
	availableCount := 0
	connectedCount := 0
	totalCapabilities := 0

	for _, server := range servers {
		if server.Enabled {
			enabledCount++
			totalCapabilities += len(server.Capabilities)
		}
		if server.Available {
			availableCount++
		}
	}

	for _, conn := range m.connections {
		if conn.connected {
			connectedCount++
		}
	}

	stats := map[string]interface{}{
		"totalServers":      len(servers),
		"enabledServers":    enabledCount,
		"availableServers":  availableCount,
		"connectedServers":  connectedCount,
		"totalCapabilities": totalCapabilities,
		"lastSync":          time.Now(),
	}

	m.log.WithFields(stats).Info("LSP statistics retrieved")
	return stats, nil
}

// RefreshAllLSPServers refreshes all LSP servers
func (m *LSPManager) RefreshAllLSPServers(ctx context.Context) error {
	servers, err := m.ListLSPServers(ctx)
	if err != nil {
		m.log.WithError(err).Error("Failed to list LSP servers for refresh")
		return fmt.Errorf("failed to list LSP servers: %w", err)
	}

	var refreshErrors []error
	refreshedCount := 0

	for _, server := range servers {
		if !server.Enabled {
			m.log.WithField("serverId", server.ID).Debug("Skipping disabled LSP server")
			continue
		}

		if err := m.refreshLSPServer(ctx, server.ID); err != nil {
			m.log.WithFields(logrus.Fields{
				"serverId": server.ID,
				"error":    err.Error(),
			}).Warn("Failed to refresh LSP server")
			refreshErrors = append(refreshErrors, err)
		} else {
			refreshedCount++
		}
	}

	m.log.WithFields(logrus.Fields{
		"refreshedCount": refreshedCount,
		"totalServers":   len(servers),
		"errorCount":     len(refreshErrors),
	}).Info("LSP servers refresh completed")

	if len(refreshErrors) > 0 {
		return fmt.Errorf("failed to refresh %d of %d servers", len(refreshErrors), len(servers))
	}

	return nil
}

// refreshLSPServer refreshes a single LSP server by ID
func (m *LSPManager) refreshLSPServer(ctx context.Context, serverID string) error {
	server, err := m.GetLSPServer(ctx, serverID)
	if err != nil {
		return err
	}

	m.log.WithFields(logrus.Fields{
		"serverId": serverID,
		"language": server.Language,
	}).Debug("Refreshing LSP server")

	// Check if binary is available
	binaryPath, available := m.findBinary(server.Command)

	m.mu.Lock()
	if s, exists := m.servers[serverID]; exists {
		s.BinaryPath = binaryPath
		s.Available = available
		now := time.Now()
		s.LastSync = &now
	}
	m.mu.Unlock()

	// If there's an existing connection, check its health
	m.mu.RLock()
	conn, hasConnection := m.connections[serverID]
	m.mu.RUnlock()

	if hasConnection && conn != nil {
		if !conn.connected {
			// Connection is dead, remove it
			m.mu.Lock()
			delete(m.connections, serverID)
			m.mu.Unlock()
		}
	}

	// Clear any cached responses for this server
	cachePattern := fmt.Sprintf("lsp_response_%s_*", serverID)
	if m.cache != nil {
		if invalidator, ok := m.cache.(interface {
			InvalidateByPattern(ctx context.Context, pattern string) error
		}); ok {
			if err := invalidator.InvalidateByPattern(ctx, cachePattern); err != nil {
				m.log.WithError(err).Warn("Failed to invalidate cache entries during refresh")
			}
		}
	}

	m.log.WithFields(logrus.Fields{
		"serverId":  serverID,
		"available": available,
	}).Info("LSP server refreshed successfully")

	return nil
}

// Close closes a specific LSP server connection
func (m *LSPManager) Close(serverID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[serverID]
	if !exists {
		return fmt.Errorf("no connection for server %s", serverID)
	}

	err := conn.Close()
	delete(m.connections, serverID)
	return err
}

// CloseAll closes all LSP server connections
func (m *LSPManager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error
	for serverID, conn := range m.connections {
		if err := conn.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close %s: %w", serverID, err))
		}
	}

	m.connections = make(map[string]*LSPConnection)

	if len(errors) > 0 {
		return fmt.Errorf("errors closing connections: %v", errors)
	}
	return nil
}

// Close closes the LSP connection
func (c *LSPConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false

	var errors []error

	// Send shutdown request (best effort)
	if c.stdin != nil {
		shutdownReq := LSPJSONRPCRequest{
			JSONRPC: "2.0",
			ID:      999999,
			Method:  "shutdown",
		}
		content, _ := json.Marshal(shutdownReq)
		header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))
		c.stdin.Write([]byte(header + string(content)))

		// Send exit notification
		exitNotif := LSPJSONRPCNotification{
			JSONRPC: "2.0",
			Method:  "exit",
		}
		content, _ = json.Marshal(exitNotif)
		header = fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))
		c.stdin.Write([]byte(header + string(content)))
	}

	if c.stdin != nil {
		if err := c.stdin.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if c.stdout != nil {
		if err := c.stdout.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if c.stderr != nil {
		if err := c.stderr.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if c.cmd != nil && c.cmd.Process != nil {
		// Give the process a chance to exit gracefully
		done := make(chan error, 1)
		go func() {
			done <- c.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited
		case <-time.After(2 * time.Second):
			// Force kill if not responding
			if err := c.cmd.Process.Kill(); err != nil {
				errors = append(errors, err)
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during close: %v", errors)
	}
	return nil
}

// IsConnected returns whether the connection is active
func (c *LSPConnection) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// GetConnection returns the connection for a server (for testing)
func (m *LSPManager) GetConnection(serverID string) *LSPConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections[serverID]
}

// AddServer adds a custom server configuration (for testing)
func (m *LSPManager) AddServer(server *LSPServer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[server.ID] = server
}

// SetConnection sets a connection for a server (for testing)
func (m *LSPManager) SetConnection(serverID string, conn *LSPConnection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connections[serverID] = conn
}

// GetConfig returns the current configuration
func (m *LSPManager) GetConfig() *LSPConfig {
	return m.config
}
