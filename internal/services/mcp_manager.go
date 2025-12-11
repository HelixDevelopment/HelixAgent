package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"
)

// MCPManager manages Model Context Protocol servers and tools
type MCPManager struct {
	servers             map[string]*MCPServer
	availableTools      map[string]*MCPTool
	messageID           int
	mu                  sync.RWMutex
	autoDiscover        bool
	retryAttempts       int
	healthCheckInterval time.Duration
}

// MCPServer represents a registered MCP server
type MCPServer struct {
	Name         string
	Command      []string
	Process      *exec.Cmd
	Stdin        io.WriteCloser
	Stdout       io.ReadCloser
	Capabilities map[string]interface{}
	Tools        []*MCPTool
	Initialized  bool
	LastHealth   time.Time
}

// MCPTool represents an MCP tool
type MCPTool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	Server      *MCPServer
}

// MCPMessage represents a JSON-RPC message
type MCPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewMCPManager creates a new MCP manager
func NewMCPManager() *MCPManager {
	return &MCPManager{
		servers:             make(map[string]*MCPServer),
		availableTools:      make(map[string]*MCPTool),
		messageID:           1,
		autoDiscover:        true,
		retryAttempts:       3,
		healthCheckInterval: 30 * time.Second,
	}
}

// NewMCPManagerWithConfig creates a new MCP manager with configuration
func NewMCPManagerWithConfig(autoDiscover bool, retryAttempts int, healthInterval time.Duration) *MCPManager {
	return &MCPManager{
		servers:             make(map[string]*MCPServer),
		availableTools:      make(map[string]*MCPTool),
		messageID:           1,
		autoDiscover:        autoDiscover,
		retryAttempts:       retryAttempts,
		healthCheckInterval: healthInterval,
	}
}

// RegisterServer registers an MCP server with enhanced error handling
func (m *MCPManager) RegisterServer(serverConfig map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name, ok := serverConfig["name"].(string)
	if !ok {
		return fmt.Errorf("server config must include 'name'")
	}

	if _, exists := m.servers[name]; exists {
		return fmt.Errorf("server %s already registered", name)
	}

	command, ok := serverConfig["command"].([]interface{})
	if !ok {
		return fmt.Errorf("server config must include 'command' as array")
	}

	cmd := make([]string, len(command))
	for i, c := range command {
		cmd[i] = c.(string)
	}

	server := &MCPServer{
		Name:         name,
		Command:      cmd,
		Capabilities: make(map[string]interface{}),
		Tools:        []*MCPTool{},
	}

	// Validate command exists
	if len(cmd) == 0 {
		return fmt.Errorf("server command cannot be empty")
	}

	// Try to start server immediately if auto-discover is enabled
	if m.autoDiscover {
		if err := m.startServerProcess(server); err != nil {
			log.Printf("Failed to start server %s: %v", name, err)
			// Don't fail registration, just log
		} else {
			if err := m.initializeServer(server); err != nil {
				log.Printf("Failed to initialize server %s: %v", name, err)
				m.stopServer(server)
			} else {
				if err := m.discoverServerTools(server); err != nil {
					log.Printf("Failed to discover tools for server %s: %v", name, err)
				}
			}
		}
	}

	m.servers[name] = server
	log.Printf("Registered MCP server: %s", name)

	return nil
}

// AutoDiscoverServers attempts to auto-discover available MCP servers
func (m *MCPManager) AutoDiscoverServers(ctx context.Context) error {
	if !m.autoDiscover {
		return nil
	}

	// Common MCP server configurations to try
	commonServers := []map[string]interface{}{
		{
			"name":    "filesystem",
			"command": []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
		},
		{
			"name":    "git",
			"command": []string{"npx", "-y", "@modelcontextprotocol/server-git", "--repository", "."},
		},
		{
			"name":    "sqlite",
			"command": []string{"npx", "-y", "@modelcontextprotocol/server-sqlite", "--db-path", ":memory:"},
		},
	}

	for _, serverConfig := range commonServers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := m.RegisterServer(serverConfig); err != nil {
				log.Printf("Failed to auto-discover server %s: %v", serverConfig["name"], err)
			}
		}
	}

	return nil
}

// stopServer stops an MCP server process
func (m *MCPManager) stopServer(server *MCPServer) {
	if server.Process != nil && server.Process.Process != nil {
		server.Process.Process.Kill()
		server.Process.Wait()
	}
	server.Process = nil
	server.Stdin = nil
	server.Stdout = nil
	server.Initialized = false
}

// startServerProcess starts the MCP server process
func (m *MCPManager) startServerProcess(server *MCPServer) error {
	cmd := exec.Command(server.Command[0], server.Command[1:]...)

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
		return fmt.Errorf("failed to start process: %w", err)
	}

	return nil
}

// initializeServer performs MCP initialization handshake
func (m *MCPManager) initializeServer(server *MCPServer) error {
	initRequest := MCPMessage{
		JSONRPC: "2.0",
		ID:      m.nextMessageID(),
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "SuperAgent",
				"version": "1.0.0",
			},
		},
	}

	response, err := m.sendMessage(server, initRequest)
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
	initializedMsg := MCPMessage{
		JSONRPC: "2.0",
		Method:  "initialized",
		Params:  map[string]interface{}{},
	}

	if err := m.sendNotification(server, initializedMsg); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	server.Initialized = true
	return nil
}

// discoverServerTools discovers available tools from the server
func (m *MCPManager) discoverServerTools(server *MCPServer) error {
	toolsRequest := MCPMessage{
		JSONRPC: "2.0",
		ID:      m.nextMessageID(),
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}

	response, err := m.sendMessage(server, toolsRequest)
	if err != nil {
		return fmt.Errorf("tools/list request failed: %w", err)
	}

	if response.Error != nil {
		return fmt.Errorf("tools/list error: %s", response.Error.Message)
	}

	// Parse tools from response
	if result, ok := response.Result.(map[string]interface{}); ok {
		if tools, ok := result["tools"].([]interface{}); ok {
			for _, toolInterface := range tools {
				if toolMap, ok := toolInterface.(map[string]interface{}); ok {
					tool := &MCPTool{
						Name:        toolMap["name"].(string),
						Description: toolMap["description"].(string),
						InputSchema: toolMap["inputSchema"].(map[string]interface{}),
						Server:      server,
					}

					server.Tools = append(server.Tools, tool)
					m.availableTools[tool.Name] = tool
				}
			}
		}
	}

	return nil
}

// CallTool executes an MCP tool
func (m *MCPManager) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (interface{}, error) {
	m.mu.RLock()
	tool, exists := m.availableTools[toolName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tool %s not found", toolName)
	}

	toolCallRequest := MCPMessage{
		JSONRPC: "2.0",
		ID:      m.nextMessageID(),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}

	response, err := m.sendMessage(tool.Server, toolCallRequest)
	if err != nil {
		return nil, fmt.Errorf("tool call failed: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("tool execution error: %s", response.Error.Message)
	}

	return response.Result, nil
}

// ListTools returns all available MCP tools
func (m *MCPManager) ListTools() []*MCPTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tools := make([]*MCPTool, 0, len(m.availableTools))
	for _, tool := range m.availableTools {
		tools = append(tools, tool)
	}

	return tools
}

// GetTool returns a specific tool by name
func (m *MCPManager) GetTool(name string) (*MCPTool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tool, exists := m.availableTools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	return tool, nil
}

// sendMessage sends a JSON-RPC request and waits for response
func (m *MCPManager) sendMessage(server *MCPServer, message MCPMessage) (*MCPMessage, error) {
	// Encode message
	data, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Send message
	if _, err := fmt.Fprintf(server.Stdin, "Content-Length: %d\r\n\r\n%s", len(data), data); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Read response
	scanner := bufio.NewScanner(server.Stdout)

	// Read Content-Length header
	if !scanner.Scan() {
		return nil, fmt.Errorf("failed to read response header")
	}
	header := scanner.Text()

	var contentLength int
	if _, err := fmt.Sscanf(header, "Content-Length: %d", &contentLength); err != nil {
		return nil, fmt.Errorf("invalid content-length header: %w", err)
	}

	// Skip empty line
	if !scanner.Scan() || scanner.Text() != "" {
		return nil, fmt.Errorf("expected empty line after header")
	}

	// Read content
	content := make([]byte, contentLength)
	if _, err := io.ReadFull(server.Stdout, content); err != nil {
		return nil, fmt.Errorf("failed to read response content: %w", err)
	}

	// Parse response
	var response MCPMessage
	if err := json.Unmarshal(content, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// sendNotification sends a JSON-RPC notification (no response expected)
func (m *MCPManager) sendNotification(server *MCPServer, message MCPMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	if _, err := fmt.Fprintf(server.Stdin, "Content-Length: %d\r\n\r\n%s", len(data), data); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

// nextMessageID returns the next message ID
func (m *MCPManager) nextMessageID() int {
	id := m.messageID
	m.messageID++
	return id
}

// Shutdown gracefully shuts down all MCP servers
func (m *MCPManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, server := range m.servers {
		if server.Process != nil && server.Process.Process != nil {
			if err := server.Process.Process.Kill(); err != nil {
				log.Printf("Error killing MCP server %s: %v", name, err)
				lastErr = err
			}
		}
	}

	return lastErr
}

// HealthCheck performs health checks on all servers
func (m *MCPManager) HealthCheck() map[string]error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]error)

	for name, server := range m.servers {
		if server.Process == nil || server.Process.Process == nil {
			results[name] = fmt.Errorf("server process not running")
			continue
		}

		// Simple health check - try to send a ping
		pingRequest := MCPMessage{
			JSONRPC: "2.0",
			ID:      m.nextMessageID(),
			Method:  "ping",
			Params:  map[string]interface{}{},
		}

		_, err := m.sendMessage(server, pingRequest)
		if err != nil {
			results[name] = err
		} else {
			results[name] = nil
			server.LastHealth = time.Now()
		}
	}

	return results
}
