package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/sirupsen/logrus"
)

// MCPClient implements a real MCP (Model Context Protocol) client.
//
// CONST-029 migration: the `sync.RWMutex + map[string]*MCPServerConnection`
// and `map[string]*MCPTool` pair was retired in favour of two safe.Store
// instances. The previous implementation had two pre-existing correctness
// hazards around tool caching:
//   - listServerTools wrote to c.tools while the caller held only c.mu.RLock
//   - getToolFromServer read c.tools without any lock at all
//
// Both are now fixed structurally: every read/write of the collections
// goes through safe.Store's internal RWMutex, and no compound invariant
// spans the two collections (DisconnectServer cascades via Snapshot →
// per-key Delete on c.tools after c.servers.Delete, which is acceptable
// since the server entry is already gone by then and concurrent callers
// will get "server not connected" consistently).
type MCPClient struct {
	servers   *safe.Store[string, *MCPServerConnection]
	tools     *safe.Store[string, *MCPTool]
	messageID atomic.Int64
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

// HTTPTransport implements MCP transport over HTTP.
//
// CONST-029 migration: the `sync.Mutex + map[string]string (headers) +
// []byte (responseData) + bool (connected)` combination was retired in
// favour of a hybrid layout:
//   - headers      → safe.Store[string, string] (structurally safe)
//   - connected    → atomic.Bool
//   - responseData → atomic.Pointer[[]byte]
//   - sendRecvMu   → NARROW Pattern-Zeta mutex that serialises the
//     request/response pair atomicity of Send() followed by Receive().
//     Without this, a second caller's Send could overwrite responseData
//     before the first caller's Receive reads it. The mutex guards a
//     sequence, not a collection, so it does not participate in the
//     bare-mutex+collection anti-pattern CONST-029 retires.
//
// HTTP/3/QUIC session affinity is carried by the *http.Client, which is
// itself safe for concurrent use; nothing here serialises that layer.
type HTTPTransport struct {
	baseURL      string
	headers      *safe.Store[string, string]
	connected    atomic.Bool
	client       *http.Client
	responseData atomic.Pointer[[]byte]
	// sendRecvMu serialises the Send→Receive request/response pair so
	// concurrent callers cannot interleave and cross-wire their
	// responses. This is a compound-invariant sequence lock, not a
	// collection lock.
	sendRecvMu sync.Mutex
}

// MCPRequest represents an MCP JSON-RPC request
// Alias for backward compatibility
type MCPRequest = JSONRPCRequest

// MCPResponse represents an MCP JSON-RPC response
// Alias for backward compatibility
type MCPResponse = JSONRPCResponse

// MCPNotification represents an MCP JSON-RPC notification
// Alias for backward compatibility
type MCPNotification = JSONRPCNotification

// MCPInitializeRequest is imported from mcp_types.go
// MCPInitializeResult is imported from mcp_types.go

// MCPToolCall represents a tool call request
// Alias for backward compatibility
type MCPToolCall = ToolCallRequest

// MCPToolResult represents a tool call result
// Alias for backward compatibility
type MCPToolResult = ToolCallResult

// MCPContent represents content in a tool result
// Alias for backward compatibility
type MCPContent = Content

// NewMCPClient creates a new MCP client
func NewMCPClient(logger *logrus.Logger) *MCPClient {
	client := &MCPClient{
		servers: safe.NewStore[string, *MCPServerConnection](),
		tools:   safe.NewStore[string, *MCPTool](),
		logger:  logger,
	}
	client.messageID.Store(0)
	return client
}

// NewHTTPTransport constructs an HTTPTransport with the given base URL
// and optional headers. The returned transport is not yet connected;
// callers flip the connected flag after their own handshake.
func NewHTTPTransport(baseURL string, headers map[string]string, client *http.Client) *HTTPTransport {
	t := &HTTPTransport{
		baseURL: baseURL,
		headers: safe.NewStore[string, string](),
		client:  client,
	}
	for k, v := range headers {
		t.headers.Put(k, v)
	}
	return t
}

// ConnectServer connects to an MCP server
func (c *MCPClient) ConnectServer(ctx context.Context, serverID, name, command string, args []string) error {
	// Reserve the serverID slot up-front; if another goroutine raced
	// ahead we bail out cleanly. We then have exclusive ownership of
	// spinning up the transport without holding any lock across the
	// process start.
	if c.servers.Has(serverID) {
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
		_ = transport.Close()
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Atomically install only if no other goroutine raced ahead.
	inserted := true
	c.servers.Update(serverID, func(current *MCPServerConnection, present bool) (*MCPServerConnection, bool) {
		if present && current != nil {
			inserted = false
			return current, true
		}
		return connection, true
	})
	if !inserted {
		_ = transport.Close()
		return fmt.Errorf("server %s already connected", serverID)
	}

	c.logger.WithField("serverId", serverID).Info("Connected to MCP server")

	return nil
}

// DisconnectServer disconnects from an MCP server
func (c *MCPClient) DisconnectServer(serverID string) error {
	connection, existed := c.servers.Delete(serverID)
	if !existed {
		return fmt.Errorf("server %s not connected", serverID)
	}

	if err := connection.Transport.Close(); err != nil {
		c.logger.WithError(err).WithField("serverId", serverID).Warn("Error closing transport")
	}

	// Cascade-remove associated tools. Collect keys under the store's
	// read lock via Snapshot, then delete outside to avoid nested
	// locking. Deletes are individually atomic.
	for toolName, tool := range c.tools.Snapshot() {
		if tool.Server != nil && tool.Server.Name == serverID {
			c.tools.Delete(toolName)
		}
	}

	c.logger.WithField("serverId", serverID).Info("Disconnected from MCP server")
	return nil
}

// ListTools lists all available tools from all connected servers
func (c *MCPClient) ListTools(ctx context.Context) ([]*MCPTool, error) {
	var allTools []*MCPTool
	for _, connection := range c.servers.Snapshot() {
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
	connection, exists := c.servers.Get(serverID)
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
	if validateErr := c.validateToolArguments(tool, arguments); validateErr != nil {
		return nil, fmt.Errorf("invalid arguments for tool %s: %w", toolName, validateErr)
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
	connection, exists := c.servers.Get(serverID)
	if !exists {
		return nil, fmt.Errorf("server %s not connected", serverID)
	}

	return connection, nil
}

// ListServers returns all connected servers
func (c *MCPClient) ListServers() []*MCPServerConnection {
	return c.servers.Values()
}

// HealthCheck performs health checks on all connected servers
func (c *MCPClient) HealthCheck(ctx context.Context) map[string]bool {
	results := make(map[string]bool)
	for serverID, connection := range c.servers.Snapshot() {
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
		_ = stdin.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
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
				"name":    "helixagent",
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
		c.tools.Put(toolData.Name, tool)
	}

	connection.Tools = tools
	return tools, nil
}

func (c *MCPClient) getToolFromServer(ctx context.Context, connection *MCPServerConnection, toolName string) (*MCPTool, error) {
	// Check cache first
	if tool, exists := c.tools.Get(toolName); exists && tool.Server != nil && tool.Server.Name == connection.Name {
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
			//nolint:errcheck // schema validation ensures correct type
			fieldName := reqField.(string)
			if _, exists := arguments[fieldName]; !exists {
				return fmt.Errorf("required field '%s' is missing", fieldName)
			}
		}
	}
	return nil
}

func (c *MCPClient) nextMessageID() int {
	return int(c.messageID.Add(1))
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
		_ = t.stdin.Close()
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

// HTTPTransport implementation for HTTP-based MCP servers

// Connect marks the transport as connected. Exposed so callers that
// construct an HTTPTransport via NewHTTPTransport can flip it live once
// their own handshake (if any) succeeds. Idempotent.
func (t *HTTPTransport) Connect() {
	t.connected.Store(true)
}

func (t *HTTPTransport) Send(ctx context.Context, message interface{}) error {
	// Narrow Pattern-Zeta: serialise Send→Receive so concurrent callers
	// cannot cross-wire their responseData. The lock is released only
	// after Receive (or the next Send on error) drains responseData.
	t.sendRecvMu.Lock()
	defer t.sendRecvMu.Unlock()

	if !t.connected.Load() {
		return fmt.Errorf("HTTP transport not connected")
	}

	// Convert message to JSON
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add custom headers from safe.Store snapshot.
	for key, value := range t.headers.Snapshot() {
		req.Header.Set(key, value)
	}

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("HTTP request failed with status %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read response
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Store response for Receive method (atomic pointer swap).
	buf := responseData
	t.responseData.Store(&buf)

	return nil
}

func (t *HTTPTransport) Receive(ctx context.Context) (interface{}, error) {
	// Paired with Send via sendRecvMu. Taking the same mutex here
	// guarantees Receive observes the responseData written by the
	// matched Send, and prevents a subsequent Send from overwriting
	// before Receive returns.
	t.sendRecvMu.Lock()
	defer t.sendRecvMu.Unlock()

	if !t.connected.Load() {
		return nil, fmt.Errorf("HTTP transport not connected")
	}

	ptr := t.responseData.Swap(nil)
	if ptr == nil || len(*ptr) == 0 {
		return nil, fmt.Errorf("no response data available")
	}

	// Parse JSON response
	var response interface{}
	if err := json.Unmarshal(*ptr, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

func (t *HTTPTransport) Close() error {
	t.connected.Store(false)
	t.responseData.Store(nil)
	return nil
}

func (t *HTTPTransport) IsConnected() bool {
	return t.connected.Load()
}
