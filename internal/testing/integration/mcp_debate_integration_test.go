// Package integration provides integration tests for MCP servers with AI Debate system.
// These tests verify that MCP tools work correctly with the AI debate framework.
package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/testutil"
)

// MCPClient provides MCP protocol communication
type MCPClient struct {
	conn    net.Conn
	reader  *bufio.Reader
	reqID   int
	timeout time.Duration
}

// MCPRequest represents a JSON-RPC request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC response
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// DebateClient provides AI Debate API communication
type DebateClient struct {
	baseURL    string
	httpClient *http.Client
}

// DebateRequest represents a debate request
type DebateRequest struct {
	Topic        string      `json:"topic"`
	Mode         string      `json:"mode"`
	Participants []string    `json:"participants,omitempty"`
	MCPContext   *MCPContext `json:"mcp_context,omitempty"`
}

// MCPContext provides MCP tool results as context for debate
type MCPContext struct {
	ToolResults []ToolResult `json:"tool_results"`
	ServerInfo  []ServerInfo `json:"server_info"`
}

// ToolResult represents the result of an MCP tool call
type ToolResult struct {
	Server    string      `json:"server"`
	Tool      string      `json:"tool"`
	Arguments interface{} `json:"arguments"`
	Result    interface{} `json:"result"`
	Error     string      `json:"error,omitempty"`
}

// ServerInfo represents MCP server information
type ServerInfo struct {
	Name   string   `json:"name"`
	Port   int      `json:"port"`
	Tools  []string `json:"tools"`
	Status string   `json:"status"`
}

// DebateResponse represents the debate API response
type DebateResponse struct {
	DebateID   string                 `json:"debate_id"`
	Topic      string                 `json:"topic"`
	Mode       string                 `json:"mode"`
	Rounds     []DebateRound          `json:"rounds"`
	Consensus  string                 `json:"consensus"`
	Confidence float64                `json:"confidence"`
	Metadata   map[string]interface{} `json:"metadata"`
	Error      string                 `json:"error,omitempty"`
}

// DebateRound represents a single round of debate
type DebateRound struct {
	Round     int              `json:"round"`
	Position  string           `json:"position"`
	Arguments []DebateArgument `json:"arguments"`
}

// DebateArgument represents an argument from a participant
type DebateArgument struct {
	Provider   string  `json:"provider"`
	Model      string  `json:"model"`
	Argument   string  `json:"argument"`
	Confidence float64 `json:"confidence"`
}

// MCPServerConfig represents MCP server configuration
type MCPServerConfig struct {
	Name  string
	Port  int
	Tools []string
}

// CoreMCPServers defines the 7 core MCP servers
var CoreMCPServers = []MCPServerConfig{
	{Name: "fetch", Port: 9101, Tools: []string{"fetch"}},
	{Name: "git", Port: 9102, Tools: []string{"git_status", "git_log", "git_diff", "git_branch_list"}},
	{Name: "time", Port: 9103, Tools: []string{"get_current_time"}},
	{Name: "filesystem", Port: 9104, Tools: []string{"read_file", "write_file", "list_directory", "create_directory"}},
	{Name: "memory", Port: 9105, Tools: []string{"create_entities", "read_graph", "search_nodes", "add_observations"}},
	{Name: "everything", Port: 9106, Tools: []string{"search", "everything_search"}},
	{Name: "sequentialthinking", Port: 9107, Tools: []string{"think", "create_thinking_session", "continue_thinking"}},
}

// NewMCPClient creates a new MCP client
func NewMCPClient(port int, timeout time.Duration) (*MCPClient, error) {
	addr := fmt.Sprintf("localhost:%d", port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	return &MCPClient{
		conn:    conn,
		reader:  bufio.NewReader(conn),
		timeout: timeout,
	}, nil
}

// Close closes the MCP connection
func (c *MCPClient) Close() error {
	return c.conn.Close()
}

// Send sends a JSON-RPC request and returns the response
func (c *MCPClient) Send(method string, params interface{}) (*MCPResponse, error) {
	c.reqID++

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      c.reqID,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Set write deadline
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("failed to set write deadline: %w", err)
	}

	// Write NDJSON (newline-delimited JSON)
	if _, err := c.conn.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Set read deadline
	if err := c.conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Read response
	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var resp MCPResponse
	if err := json.Unmarshal(bytes.TrimSpace(line), &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

// Initialize initializes the MCP session
func (c *MCPClient) Initialize() error {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "HelixAgent-Integration-Test",
			"version": "1.0.0",
		},
	}

	resp, err := c.Send("initialize", params)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	return nil
}

// ListTools lists available tools
func (c *MCPClient) ListTools() ([]string, error) {
	resp, err := c.Send("tools/list", nil)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("list tools error: %s", resp.Error.Message)
	}

	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools: %w", err)
	}

	tools := make([]string, len(result.Tools))
	for i, t := range result.Tools {
		tools[i] = t.Name
	}
	return tools, nil
}

// CallTool calls an MCP tool
func (c *MCPClient) CallTool(name string, arguments map[string]interface{}) (*MCPResponse, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": arguments,
	}
	return c.Send("tools/call", params)
}

// NewDebateClient creates a new debate API client
func NewDebateClient(baseURL string) *DebateClient {
	return &DebateClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// CreateDebate creates a new debate
func (d *DebateClient) CreateDebate(req *DebateRequest) (*DebateResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := d.httpClient.Post(d.baseURL+"/v1/debates", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create debate: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("debate API failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result DebateResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// TestMCPServerConnectivity tests that all MCP servers are reachable
func TestMCPServerConnectivity(t *testing.T) {
	for _, server := range CoreMCPServers {
		t.Run(server.Name, func(t *testing.T) {
			testutil.RequireExternalService(t, server.Name, "localhost", fmt.Sprintf("%d", server.Port))
			client, err := NewMCPClient(server.Port, 10*time.Second)
			require.NoError(t, err)
			defer func() { _ = client.Close() }()

			assert.NotNil(t, client)
		})
	}
}

// TestMCPServerInitialize tests MCP protocol initialization
func TestMCPServerInitialize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MCP integration test in -short mode (requires running MCP servers)")  // SKIP-OK: #short-mode
	}
	for _, server := range CoreMCPServers {
		t.Run(server.Name, func(t *testing.T) {
			testutil.RequireExternalService(t, server.Name, "localhost", fmt.Sprintf("%d", server.Port))
			client, err := NewMCPClient(server.Port, 10*time.Second)
			require.NoError(t, err)
			defer func() { _ = client.Close() }()

			err = client.Initialize()
			require.NoError(t, err, "Initialize should succeed")
		})
	}
}

// TestMCPServerToolDiscovery tests MCP tool discovery
func TestMCPServerToolDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MCP integration test in -short mode")  // SKIP-OK: #short-mode
	}
	for _, server := range CoreMCPServers {
		t.Run(server.Name, func(t *testing.T) {
			testutil.RequireExternalService(t, server.Name, "localhost", fmt.Sprintf("%d", server.Port))
			client, err := NewMCPClient(server.Port, 10*time.Second)
			require.NoError(t, err)
			defer func() { _ = client.Close() }()

			err = client.Initialize()
			require.NoError(t, err)

			tools, err := client.ListTools()
			require.NoError(t, err)
			assert.NotEmpty(t, tools, "Should have at least one tool")

			t.Logf("Server %s has %d tools: %v", server.Name, len(tools), tools)
		})
	}
}

// TestMCPToolExecution tests actual MCP tool execution
func TestMCPToolExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MCP integration test in -short mode")  // SKIP-OK: #short-mode
	}
	testCases := []struct {
		server    string
		port      int
		tool      string
		arguments map[string]interface{}
	}{
		{
			server:    "time",
			port:      9103,
			tool:      "get_current_time",
			arguments: map[string]interface{}{"timezone": "UTC"},
		},
		{
			server:    "filesystem",
			port:      9104,
			tool:      "list_directory",
			arguments: map[string]interface{}{"path": "/tmp"},
		},
		{
			server: "memory",
			port:   9105,
			tool:   "create_entities",
			arguments: map[string]interface{}{
				"entities": []map[string]interface{}{
					{"name": "test", "entityType": "concept", "observations": []string{"test observation"}},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s/%s", tc.server, tc.tool), func(t *testing.T) {
			testutil.RequireExternalService(t, tc.server, "localhost", fmt.Sprintf("%d", tc.port))
			client, err := NewMCPClient(tc.port, 10*time.Second)
			require.NoError(t, err)
			defer func() { _ = client.Close() }()

			err = client.Initialize()
			require.NoError(t, err)

			resp, err := client.CallTool(tc.tool, tc.arguments)
			require.NoError(t, err)

			if resp.Error != nil {
				t.Logf("Tool %s returned error: %s", tc.tool, resp.Error.Message)
			} else {
				assert.NotEmpty(t, resp.Result, "Tool should return a result")
				t.Logf("Tool %s result: %s", tc.tool, string(resp.Result))
			}
		})
	}
}

// TestMCPDebateIntegration tests MCP tools with AI Debate system
func TestMCPDebateIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MCP integration test in -short mode")  // SKIP-OK: #short-mode
	}
	// First, collect MCP tool results
	mcpContext := &MCPContext{
		ToolResults: make([]ToolResult, 0),
		ServerInfo:  make([]ServerInfo, 0),
	}

	// Test time server and collect result
	testutil.RequireExternalService(t, "time", "localhost", "9103")
	timeClient, err := NewMCPClient(9103, 10*time.Second)
	require.NoError(t, err)

	err = timeClient.Initialize()
	require.NoError(t, err)

	tools, err := timeClient.ListTools()
	if err == nil {
		mcpContext.ServerInfo = append(mcpContext.ServerInfo, ServerInfo{
			Name:   "time",
			Port:   9103,
			Tools:  tools,
			Status: "running",
		})
	}

	resp, err := timeClient.CallTool("get_current_time", map[string]interface{}{"timezone": "UTC"})
	if err == nil && resp.Error == nil {
		var result interface{}
		_ = json.Unmarshal(resp.Result, &result)
		mcpContext.ToolResults = append(mcpContext.ToolResults, ToolResult{
			Server:    "time",
			Tool:      "get_current_time",
			Arguments: map[string]interface{}{"timezone": "UTC"},
			Result:    result,
		})
	}
	_ = timeClient.Close()

	// Now test with AI Debate
	debateClient := NewDebateClient("http://localhost:8080")

	debateReq := &DebateRequest{
		Topic:      "What is the current time and why is time measurement important?",
		Mode:       "consensus",
		MCPContext: mcpContext,
	}

	testutil.RequireServer(t)
	debateResp, err := debateClient.CreateDebate(debateReq)
	require.NoError(t, err, "Failed to create debate")

	assert.NotEmpty(t, debateResp.DebateID, "Should have debate ID")
	assert.NotEmpty(t, debateResp.Consensus, "Should have consensus")

	t.Logf("Debate ID: %s", debateResp.DebateID)
	t.Logf("Consensus: %s", debateResp.Consensus)
	t.Logf("Confidence: %.2f", debateResp.Confidence)
}

// TestMCPContextualDebate tests debate with multiple MCP tool contexts
func TestMCPContextualDebate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MCP integration test in -short mode")  // SKIP-OK: #short-mode
	}
	ctx := context.Background()
	mcpContext := &MCPContext{
		ToolResults: make([]ToolResult, 0),
		ServerInfo:  make([]ServerInfo, 0),
	}

	// Collect information from multiple MCP servers
	servers := []struct {
		name string
		port int
		tool string
		args map[string]interface{}
	}{
		{name: "time", port: 9103, tool: "get_current_time", args: map[string]interface{}{"timezone": "UTC"}},
		{name: "filesystem", port: 9104, tool: "list_directory", args: map[string]interface{}{"path": "/tmp"}},
	}

	for _, srv := range servers {
		func() {
			client, err := NewMCPClient(srv.port, 10*time.Second)
			if err != nil {
				return
			}
			defer func() { _ = client.Close() }()

			if err := client.Initialize(); err != nil {
				return
			}

			tools, _ := client.ListTools()
			mcpContext.ServerInfo = append(mcpContext.ServerInfo, ServerInfo{
				Name:   srv.name,
				Port:   srv.port,
				Tools:  tools,
				Status: "running",
			})

			resp, err := client.CallTool(srv.tool, srv.args)
			if err == nil && resp.Error == nil {
				var result interface{}
				_ = json.Unmarshal(resp.Result, &result)
				mcpContext.ToolResults = append(mcpContext.ToolResults, ToolResult{
					Server:    srv.name,
					Tool:      srv.tool,
					Arguments: srv.args,
					Result:    result,
				})
			}
		}()
	}

	if len(mcpContext.ToolResults) == 0 {
		t.Skip("No MCP tool results collected")  // SKIP-OK: #legacy-untriaged
		return
	}

	_ = ctx // Used for cancellation in real implementation

	t.Logf("Collected %d tool results from %d servers", len(mcpContext.ToolResults), len(mcpContext.ServerInfo))

	// Test debate with this context
	testutil.RequireServer(t)
	debateClient := NewDebateClient("http://localhost:8080")

	debateReq := &DebateRequest{
		Topic:      "Based on the file system information and current time, discuss how to organize temporary files effectively",
		Mode:       "adversarial",
		MCPContext: mcpContext,
	}

	debateResp, err := debateClient.CreateDebate(debateReq)
	require.NoError(t, err, "Failed to create debate")

	assert.NotEmpty(t, debateResp.DebateID)
	t.Logf("Debate created with MCP context: %s", debateResp.DebateID)
}

// TestAllMCPServersForDebate tests that all MCP servers can provide context for debate
func TestAllMCPServersForDebate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MCP integration test in -short mode")  // SKIP-OK: #short-mode
	}
	runningServers := 0

	for _, server := range CoreMCPServers {
		t.Run(server.Name, func(t *testing.T) {
			testutil.RequireExternalService(t, server.Name, "localhost", fmt.Sprintf("%d", server.Port))
			client, err := NewMCPClient(server.Port, 10*time.Second)
			require.NoError(t, err)
			defer func() { _ = client.Close() }()

			err = client.Initialize()
			require.NoError(t, err, "Initialize should succeed")

			tools, err := client.ListTools()
			require.NoError(t, err, "ListTools should succeed")
			assert.NotEmpty(t, tools, "Should have tools")

			runningServers++
			t.Logf("Server %s ready for debate with %d tools", server.Name, len(tools))
		})
	}

	t.Logf("Total running MCP servers: %d/%d", runningServers, len(CoreMCPServers))
}

// BenchmarkMCPToolCall benchmarks MCP tool call performance
func BenchmarkMCPToolCall(b *testing.B) {
	client, err := NewMCPClient(9103, 10*time.Second)
	if err != nil {
		b.Skipf("Time MCP server not running: %v", err)
		return
	}
	defer func() { _ = client.Close() }()

	if err := client.Initialize(); err != nil {
		b.Skipf("Failed to initialize: %v", err)
		return
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.CallTool("get_current_time", map[string]interface{}{"timezone": "UTC"})
	}
}
