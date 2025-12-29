package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// MCPClient implements a real MCP (Model Context Protocol) client
type MCPClient struct {
	servers   map[string]*MCPServerConnection
	tools     map[string]*MCPTool
	messageID int
	mu        sync.RWMutex
	logger    *logrus.Logger
}

// MCPServerConnection represents a live connection to an MCP server
type MCPServerConnection struct {
	ID           string
	Name         string
	Transport    MCPTransport
	Capabilities map[string]interface{}
	Tools        []*MCPTool
	Connected    bool
	LastUsed     time.Time
}

// MCPTransport defines the interface for MCP communication
type MCPTransport interface {
	Send(ctx context.Context, message interface{}) error
	Receive(ctx context.Context) (interface{}, error)
	Close() error
	IsConnected() bool
}

// StdioTransport implements MCP transport over stdio
type StdioTransport struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	scanner   *bufio.Scanner
	connected bool
	mu        sync.Mutex
}

// HTTPTransport implements MCP transport over HTTP
type HTTPTransport struct {
	baseURL   string
	headers   map[string]string
	connected bool
	mu        sync.Mutex
}

// MCPRequest represents an MCP JSON-RPC request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents an MCP JSON-RPC response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPNotification represents an MCP JSON-RPC notification
type MCPNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPInitializeRequest represents an initialize request
type MCPInitializeRequest struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      map[string]string      `json:"clientInfo"`
}

// MCPInitializeResult represents an initialize response
type MCPInitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      map[string]string      `json:"serverInfo"`
	Instructions    string                 `json:"instructions,omitempty"`
}

// MCPToolCall represents a tool call request
type MCPToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// MCPToolResult represents a tool call result
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// MCPContent represents content in a tool result
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// NewMCPClient creates a new MCP client
func NewMCPClient(logger *logrus.Logger) *MCPClient {
	return &MCPClient{
		servers:   make(map[string]*MCPServerConnection),
		tools:     make(map[string]*MCPTool),
		messageID: 1,
		logger:    logger,
	}
}

// ConnectServer connects to an MCP server
func (c *MCPClient) ConnectServer(ctx context.Context, serverID, name, command string, args []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.servers[serverID]; exists {
		return fmt.Errorf("server %s already connected", serverID)
	}

	// Create transport
	transport, err := c.createStdioTransport(command, args)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	connection := &MCPServerConnection{
		ID:        serverID,
		Name:      name,
		Transport: transport,
		Connected: true,
		LastUsed:  time.Now(),
	}

	// Initialize the server
	if err := c.initializeServer(ctx, connection); err != nil {
		transport.Close()
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	c.servers[serverID] = connection
	c.logger.WithField("serverId", serverID).Info("Connected to MCP server")

	return nil
}

// DisconnectServer disconnects from an MCP server
func (c *MCPClient) DisconnectServer(serverID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	connection, exists := c.servers[serverID]
	if !exists {
		return fmt.Errorf("server %s not connected", serverID)
	}

	if err := connection.Transport.Close(); err != nil {
		c.logger.WithError(err).WithField("serverId", serverID).Warn("Error closing transport")
	}

	delete(c.servers, serverID)

	// Remove associated tools
	for toolName, tool := range c.tools {
		if tool.Server.Name == serverID {
			delete(c.tools, toolName)
		}
	}

	c.logger.WithField("serverId", serverID).Info("Disconnected from MCP server")
	return nil
}

// ListTools lists all available tools from all connected servers
func (c *MCPClient) ListTools(ctx context.Context) ([]*MCPTool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var allTools []*MCPTool
	for _, connection := range c.servers {
		if !connection.Connected {
			continue
		}

		tools, err := c.listServerTools(ctx, connection)
		if err != nil {
			c.logger.WithError(err).WithField("serverId", connection.ID).Warn("Failed to list tools from server")
			continue
		}

		allTools = append(allTools, tools...)
	}

	return allTools, nil
}

// CallTool executes a tool on a specific server
func (c *MCPClient) CallTool(ctx context.Context, serverID, toolName string, arguments map[string]interface{}) (*MCPToolResult, error) {
	c.mu.RLock()
	connection, exists := c.servers[serverID]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server %s not connected", serverID)
	}

	if !connection.Connected {
		return nil, fmt.Errorf("server %s not connected", serverID)
	}

	// Check if tool exists on this server
	tool, err := c.getToolFromServer(ctx, connection, toolName)
	if err != nil {
		return nil, fmt.Errorf("tool %s not available on server %s: %w", toolName, serverID, err)
	}

	// Validate arguments against schema
	if err := c.validateToolArguments(tool, arguments); err != nil {
		return nil, fmt.Errorf("invalid arguments for tool %s: %w", toolName, err)
	}

	// Execute the tool
	result, err := c.callServerTool(ctx, connection, toolName, arguments)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	connection.LastUsed = time.Now()
	return result, nil
}

// GetServerInfo returns information about a connected server
func (c *MCPClient) GetServerInfo(serverID string) (*MCPServerConnection, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	connection, exists := c.servers[serverID]
	if !exists {
		return nil, fmt.Errorf("server %s not connected", serverID)
	}

	return connection, nil
}

// ListServers returns all connected servers
func (c *MCPClient) ListServers() []*MCPServerConnection {
	c.mu.RLock()
	defer c.mu.RUnlock()

	servers := make([]*MCPServerConnection, 0, len(c.servers))
	for _, server := range c.servers {
		servers = append(servers, server)
	}

	return servers
}

// HealthCheck performs health checks on all connected servers
func (c *MCPClient) HealthCheck(ctx context.Context) map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make(map[string]bool)
	for serverID, connection := range c.servers {
		results[serverID] = connection.Transport.IsConnected()
	}

	return results
}

// Private methods

func (c *MCPClient) createStdioTransport(command string, args []string) (MCPTransport, error) {
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

	return &StdioTransport{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		scanner:   bufio.NewScanner(stdout),
		connected: true,
	}, nil
}

func (c *MCPClient) initializeServer(ctx context.Context, connection *MCPServerConnection) error {
	initRequest := MCPRequest{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "initialize",
		Params: MCPInitializeRequest{
			ProtocolVersion: "2024-11-05",
			Capabilities:    map[string]interface{}{},
			ClientInfo: map[string]string{
				"name":    "superagent",
				"version": "1.0.0",
			},
		},
	}

	if err := connection.Transport.Send(ctx, initRequest); err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	response, err := connection.Transport.Receive(ctx)
	if err != nil {
		return fmt.Errorf("failed to receive initialize response: %w", err)
	}

	var initResponse MCPResponse
	if err := c.unmarshalResponse(response, &initResponse); err != nil {
		return fmt.Errorf("failed to unmarshal initialize response: %w", err)
	}

	if initResponse.Error != nil {
		return fmt.Errorf("initialize failed: %s", initResponse.Error.Message)
	}

	var result MCPInitializeResult
	if err := c.unmarshalResult(initResponse.Result, &result); err != nil {
		return fmt.Errorf("failed to unmarshal initialize result: %w", err)
	}

	connection.Capabilities = result.Capabilities

	// Send initialized notification
	initializedNotification := MCPNotification{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  map[string]interface{}{},
	}

	if err := connection.Transport.Send(ctx, initializedNotification); err != nil {
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}

	return nil
}

func (c *MCPClient) listServerTools(ctx context.Context, connection *MCPServerConnection) ([]*MCPTool, error) {
	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}

	if err := connection.Transport.Send(ctx, request); err != nil {
		return nil, fmt.Errorf("failed to send tools/list request: %w", err)
	}

	response, err := connection.Transport.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive tools/list response: %w", err)
	}

	var toolsResponse MCPResponse
	if err := c.unmarshalResponse(response, &toolsResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/list response: %w", err)
	}

	if toolsResponse.Error != nil {
		return nil, fmt.Errorf("tools/list failed: %s", toolsResponse.Error.Message)
	}

	var result struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := c.unmarshalResult(toolsResponse.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/list result: %w", err)
	}

	var tools []*MCPTool
	for _, toolData := range result.Tools {
		tool := &MCPTool{
			Name:        toolData.Name,
			Description: toolData.Description,
			InputSchema: toolData.InputSchema,
			Server: &MCPServer{
				Name: connection.Name,
			},
		}
		tools = append(tools, tool)

		// Cache tool
		c.tools[toolData.Name] = tool
	}

	connection.Tools = tools
	return tools, nil
}

func (c *MCPClient) getToolFromServer(ctx context.Context, connection *MCPServerConnection, toolName string) (*MCPTool, error) {
	// Check cache first
	if tool, exists := c.tools[toolName]; exists && tool.Server.Name == connection.Name {
		return tool, nil
	}

	// Refresh tools list
	tools, err := c.listServerTools(ctx, connection)
	if err != nil {
		return nil, err
	}

	for _, tool := range tools {
		if tool.Name == toolName {
			return tool, nil
		}
	}

	return nil, fmt.Errorf("tool not found")
}

func (c *MCPClient) callServerTool(ctx context.Context, connection *MCPServerConnection, toolName string, arguments map[string]interface{}) (*MCPToolResult, error) {
	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}

	if err := connection.Transport.Send(ctx, request); err != nil {
		return nil, fmt.Errorf("failed to send tools/call request: %w", err)
	}

	response, err := connection.Transport.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive tools/call response: %w", err)
	}

	var toolResponse MCPResponse
	if err := c.unmarshalResponse(response, &toolResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/call response: %w", err)
	}

	if toolResponse.Error != nil {
		return &MCPToolResult{
			Content: []MCPContent{
				{
					Type: "text",
					Text: fmt.Sprintf("Error: %s", toolResponse.Error.Message),
				},
			},
			IsError: true,
		}, nil
	}

	var result struct {
		Content []MCPContent `json:"content"`
	}

	if err := c.unmarshalResult(toolResponse.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/call result: %w", err)
	}

	return &MCPToolResult{
		Content: result.Content,
		IsError: false,
	}, nil
}

func (c *MCPClient) validateToolArguments(tool *MCPTool, arguments map[string]interface{}) error {
	// Basic validation - check required fields
	if required, ok := tool.InputSchema["required"].([]interface{}); ok {
		for _, reqField := range required {
			fieldName := reqField.(string)
			if _, exists := arguments[fieldName]; !exists {
				return fmt.Errorf("required field '%s' is missing", fieldName)
			}
		}
	}
	return nil
}

func (c *MCPClient) nextMessageID() int {
	c.messageID++
	return c.messageID
}

func (c *MCPClient) unmarshalResponse(data interface{}, response *MCPResponse) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, response)
}

func (c *MCPClient) unmarshalResult(result interface{}, target interface{}) error {
	jsonData, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}

// StdioTransport implementation

func (t *StdioTransport) Send(ctx context.Context, message interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return fmt.Errorf("transport not connected")
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	if _, err := t.stdin.Write(append(jsonData, '\n')); err != nil {
		t.connected = false
		return err
	}

	return nil
}

func (t *StdioTransport) Receive(ctx context.Context) (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil, fmt.Errorf("transport not connected")
	}

	if !t.scanner.Scan() {
		t.connected = false
		if err := t.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}

	var message interface{}
	if err := json.Unmarshal(t.scanner.Bytes(), &message); err != nil {
		return nil, err
	}

	return message, nil
}

func (t *StdioTransport) Close() error {
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

func (t *StdioTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

// HTTPTransport implementation (placeholder for future HTTP-based MCP servers)

func (t *HTTPTransport) Send(ctx context.Context, message interface{}) error {
	return fmt.Errorf("HTTP transport not implemented")
}

func (t *HTTPTransport) Receive(ctx context.Context) (interface{}, error) {
	return nil, fmt.Errorf("HTTP transport not implemented")
}

func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connected = false
	return nil
}

func (t *HTTPTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}
