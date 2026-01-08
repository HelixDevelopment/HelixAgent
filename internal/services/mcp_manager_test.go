package services

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMCPTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

func TestNewMCPManager(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)

	require.NotNil(t, manager)
	assert.NotNil(t, manager.client)
	assert.Nil(t, manager.repo)
	assert.Nil(t, manager.cache)
	assert.NotNil(t, manager.log)
}

func TestMCPManager_ListMCPServers(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)
	ctx := context.Background()

	servers, err := manager.ListMCPServers(ctx)
	require.NoError(t, err)
	require.NotNil(t, servers)
	// Initially empty
	assert.Len(t, servers, 0)
}

func TestMCPManager_ExecuteMCPTool(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("fallback for non-unified request", func(t *testing.T) {
		result, err := manager.ExecuteMCPTool(ctx, "some-request")
		require.NoError(t, err)
		require.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.True(t, resultMap["success"].(bool))
		assert.NotEmpty(t, resultMap["result"])
		assert.NotEmpty(t, resultMap["timestamp"])
	})

	t.Run("unified protocol request without connected server", func(t *testing.T) {
		req := UnifiedProtocolRequest{
			ProtocolType: "mcp",
			ServerID:     "test-server",
			ToolName:     "test-tool",
			Arguments:    map[string]interface{}{"arg1": "value1"},
		}
		result, err := manager.ExecuteMCPTool(ctx, req)
		// Should fail because server is not connected
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestMCPManager_ListTools(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)

	tools := manager.ListTools()
	// Initially empty since no servers connected
	assert.Len(t, tools, 0)
}

func TestMCPManager_GetMCPTools(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)
	ctx := context.Background()

	tools, err := manager.GetMCPTools(ctx)
	require.NoError(t, err)
	require.NotNil(t, tools)
	// Initially empty
	assert.Len(t, tools, 0)
}

func TestMCPManager_ValidateMCPRequest(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("non-unified request passes validation", func(t *testing.T) {
		err := manager.ValidateMCPRequest(ctx, "simple-request")
		assert.NoError(t, err)
	})

	t.Run("unified request with non-existent server", func(t *testing.T) {
		req := UnifiedProtocolRequest{
			ProtocolType: "mcp",
			ServerID:     "non-existent-server",
			ToolName:     "test-tool",
		}
		err := manager.ValidateMCPRequest(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})
}

func TestMCPManager_SyncMCPServer(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)
	ctx := context.Background()

	// SyncMCPServer just logs, so it should always succeed
	err := manager.SyncMCPServer(ctx, "test-server")
	assert.NoError(t, err)
}

func TestMCPManager_GetMCPStats(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)
	ctx := context.Background()

	stats, err := manager.GetMCPStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Contains(t, stats, "totalServers")
	assert.Contains(t, stats, "connectedServers")
	assert.Contains(t, stats, "healthyServers")
	assert.Contains(t, stats, "totalTools")
	assert.Contains(t, stats, "lastSync")

	// Initially zero
	assert.Equal(t, 0, stats["totalServers"])
	assert.Equal(t, 0, stats["totalTools"])
}

func TestMCPManager_RegisterServer(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)

	t.Run("missing name", func(t *testing.T) {
		config := map[string]interface{}{
			"command": []interface{}{"echo", "hello"},
		}
		err := manager.RegisterServer(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must include 'name'")
	})

	t.Run("missing command", func(t *testing.T) {
		config := map[string]interface{}{
			"name": "test-server",
		}
		err := manager.RegisterServer(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must include 'command'")
	})

	t.Run("command wrong type", func(t *testing.T) {
		config := map[string]interface{}{
			"name":    "test-server",
			"command": "not-an-array",
		}
		err := manager.RegisterServer(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must include 'command' as array")
	})

	t.Run("empty command", func(t *testing.T) {
		config := map[string]interface{}{
			"name":    "test-server",
			"command": []interface{}{},
		}
		err := manager.RegisterServer(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "command cannot be empty")
	})

	t.Run("command argument wrong type", func(t *testing.T) {
		config := map[string]interface{}{
			"name":    "test-server",
			"command": []interface{}{123, "arg"},
		}
		err := manager.RegisterServer(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a string")
	})
}

func TestMCPManager_CallTool(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("no servers connected", func(t *testing.T) {
		result, err := manager.CallTool(ctx, "test-tool", map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no MCP servers connected")
		assert.Nil(t, result)
	})
}

func TestMCPManager_DisconnectServer(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)

	t.Run("disconnect non-existent server", func(t *testing.T) {
		err := manager.DisconnectServer("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})
}

// Test MCP types
func TestMCPTool_Structure(t *testing.T) {
	tool := MCPTool{
		Name:        "read-file",
		Description: "Read a file from disk",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]string{"type": "string"},
			},
			"required": []string{"path"},
		},
		Server: &MCPServer{Name: "filesystem"},
	}

	assert.Equal(t, "read-file", tool.Name)
	assert.Equal(t, "Read a file from disk", tool.Description)
	assert.NotNil(t, tool.InputSchema)
	assert.Equal(t, "filesystem", tool.Server.Name)
}

func TestMCPServer_Structure(t *testing.T) {
	server := MCPServer{
		Name: "filesystem-server",
	}

	assert.Equal(t, "filesystem-server", server.Name)
}

func TestMCPError_Structure(t *testing.T) {
	mcpErr := MCPError{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    map[string]string{"details": "missing method"},
	}

	assert.Equal(t, -32600, mcpErr.Code)
	assert.Equal(t, "Invalid Request", mcpErr.Message)
	assert.NotNil(t, mcpErr.Data)
}

func TestMCPServerConnection_Structure(t *testing.T) {
	connection := MCPServerConnection{
		ID:   "server-1",
		Name: "Filesystem Server",
		Capabilities: map[string]interface{}{
			"tools": true,
		},
		Tools:     []*MCPTool{},
		Connected: true,
	}

	assert.Equal(t, "server-1", connection.ID)
	assert.Equal(t, "Filesystem Server", connection.Name)
	assert.True(t, connection.Connected)
	assert.NotNil(t, connection.Capabilities)
}

func TestMCPRequest_Structure(t *testing.T) {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "read-file",
			"arguments": map[string]string{"path": "/tmp/test.txt"},
		},
	}

	assert.Equal(t, "2.0", req.JSONRPC)
	assert.Equal(t, 1, req.ID)
	assert.Equal(t, "tools/call", req.Method)
	assert.NotNil(t, req.Params)
}

func TestMCPResponse_Structure(t *testing.T) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "file contents"},
			},
		},
	}

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, resp.ID)
	assert.NotNil(t, resp.Result)
	assert.Nil(t, resp.Error)
}

func TestMCPResponse_WithError(t *testing.T) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Error: &MCPError{
			Code:    -32601,
			Message: "Method not found",
		},
	}

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Result)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
}

func TestMCPNotification_Structure(t *testing.T) {
	notification := MCPNotification{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  map[string]interface{}{},
	}

	assert.Equal(t, "2.0", notification.JSONRPC)
	assert.Equal(t, "notifications/initialized", notification.Method)
	assert.NotNil(t, notification.Params)
}

func TestMCPInitializeRequest_Structure(t *testing.T) {
	req := MCPInitializeRequest{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]interface{}{},
		ClientInfo: map[string]string{
			"name":    "helixagent",
			"version": "1.0.0",
		},
	}

	assert.Equal(t, "2024-11-05", req.ProtocolVersion)
	assert.NotNil(t, req.Capabilities)
	assert.Equal(t, "helixagent", req.ClientInfo["name"])
}

func TestMCPInitializeResult_Structure(t *testing.T) {
	result := MCPInitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: map[string]interface{}{
			"tools": map[string]bool{"listChanged": true},
		},
		ServerInfo: map[string]string{
			"name":    "filesystem-server",
			"version": "0.1.0",
		},
		Instructions: "Use this server to read and write files",
	}

	assert.Equal(t, "2024-11-05", result.ProtocolVersion)
	assert.NotNil(t, result.Capabilities)
	assert.Equal(t, "filesystem-server", result.ServerInfo["name"])
	assert.Equal(t, "Use this server to read and write files", result.Instructions)
}

func TestMCPToolCall_Structure(t *testing.T) {
	call := MCPToolCall{
		Name: "read-file",
		Arguments: map[string]interface{}{
			"path": "/tmp/test.txt",
		},
	}

	assert.Equal(t, "read-file", call.Name)
	assert.Equal(t, "/tmp/test.txt", call.Arguments["path"])
}

func TestMCPToolResult_Structure(t *testing.T) {
	result := MCPToolResult{
		Content: []MCPContent{
			{Type: "text", Text: "File contents here"},
		},
		IsError: false,
	}

	assert.Len(t, result.Content, 1)
	assert.Equal(t, "text", result.Content[0].Type)
	assert.Equal(t, "File contents here", result.Content[0].Text)
	assert.False(t, result.IsError)
}

func TestMCPToolResult_WithError(t *testing.T) {
	result := MCPToolResult{
		Content: []MCPContent{
			{Type: "text", Text: "Error: file not found"},
		},
		IsError: true,
	}

	assert.True(t, result.IsError)
}

func TestMCPContent_Structure(t *testing.T) {
	content := MCPContent{
		Type: "text",
		Text: "Some content",
	}

	assert.Equal(t, "text", content.Type)
	assert.Equal(t, "Some content", content.Text)
}

// Test MCPClient directly
func TestNewMCPClient(t *testing.T) {
	log := newMCPTestLogger()
	client := NewMCPClient(log)

	require.NotNil(t, client)
	assert.NotNil(t, client.servers)
	assert.NotNil(t, client.tools)
	assert.Equal(t, 1, client.messageID)
	assert.NotNil(t, client.logger)
}

func TestMCPClient_ListServers(t *testing.T) {
	log := newMCPTestLogger()
	client := NewMCPClient(log)

	servers := client.ListServers()
	require.NotNil(t, servers)
	assert.Len(t, servers, 0)
}

func TestMCPClient_ListTools(t *testing.T) {
	log := newMCPTestLogger()
	client := NewMCPClient(log)
	ctx := context.Background()

	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	// Returns nil when no servers, which is empty
	assert.Len(t, tools, 0)
}

func TestMCPClient_GetServerInfo(t *testing.T) {
	log := newMCPTestLogger()
	client := NewMCPClient(log)

	t.Run("non-existent server", func(t *testing.T) {
		info, err := client.GetServerInfo("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
		assert.Nil(t, info)
	})
}

func TestMCPClient_HealthCheck(t *testing.T) {
	log := newMCPTestLogger()
	client := NewMCPClient(log)
	ctx := context.Background()

	results := client.HealthCheck(ctx)
	require.NotNil(t, results)
	assert.Len(t, results, 0)
}

func TestMCPClient_DisconnectServer(t *testing.T) {
	log := newMCPTestLogger()
	client := NewMCPClient(log)

	err := client.DisconnectServer("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestMCPClient_CallTool(t *testing.T) {
	log := newMCPTestLogger()
	client := NewMCPClient(log)
	ctx := context.Background()

	t.Run("server not connected", func(t *testing.T) {
		result, err := client.CallTool(ctx, "non-existent", "test-tool", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
		assert.Nil(t, result)
	})
}

// Test HTTPTransport
func TestHTTPTransport_Structure(t *testing.T) {
	transport := &HTTPTransport{
		baseURL:   "http://localhost:7061",
		headers:   map[string]string{"Authorization": "Bearer token"},
		connected: true,
	}

	assert.Equal(t, "http://localhost:7061", transport.baseURL)
	assert.Equal(t, "Bearer token", transport.headers["Authorization"])
	assert.True(t, transport.connected)
}

func TestHTTPTransport_IsConnected(t *testing.T) {
	transport := &HTTPTransport{
		connected: true,
	}

	assert.True(t, transport.IsConnected())

	transport.connected = false
	assert.False(t, transport.IsConnected())
}

func TestHTTPTransport_Close(t *testing.T) {
	transport := &HTTPTransport{
		connected: true,
	}

	err := transport.Close()
	assert.NoError(t, err)
	assert.False(t, transport.connected)
}

func TestHTTPTransport_Send_NotConnected(t *testing.T) {
	transport := &HTTPTransport{
		connected: false,
	}
	ctx := context.Background()

	err := transport.Send(ctx, map[string]string{"test": "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestHTTPTransport_Receive_NotConnected(t *testing.T) {
	transport := &HTTPTransport{
		connected: false,
	}
	ctx := context.Background()

	result, err := transport.Receive(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
	assert.Nil(t, result)
}

func TestHTTPTransport_Receive_NoData(t *testing.T) {
	transport := &HTTPTransport{
		connected:    true,
		responseData: nil,
	}
	ctx := context.Background()

	result, err := transport.Receive(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no response data")
	assert.Nil(t, result)
}

func TestHTTPTransport_Receive_WithData(t *testing.T) {
	transport := &HTTPTransport{
		connected:    true,
		responseData: []byte(`{"jsonrpc":"2.0","id":1,"result":{"success":true}}`),
	}
	ctx := context.Background()

	result, err := transport.Receive(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Response data should be cleared after receive
	result2, err := transport.Receive(ctx)
	assert.Error(t, err)
	assert.Nil(t, result2)
}

// Test StdioTransport
func TestStdioTransport_IsConnected(t *testing.T) {
	transport := &StdioTransport{
		connected: true,
	}

	assert.True(t, transport.IsConnected())

	transport.connected = false
	assert.False(t, transport.IsConnected())
}

func TestStdioTransport_Send_NotConnected(t *testing.T) {
	transport := &StdioTransport{
		connected: false,
	}
	ctx := context.Background()

	err := transport.Send(ctx, map[string]string{"test": "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestStdioTransport_Receive_NotConnected(t *testing.T) {
	transport := &StdioTransport{
		connected: false,
	}
	ctx := context.Background()

	result, err := transport.Receive(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
	assert.Nil(t, result)
}

func TestStdioTransport_Close(t *testing.T) {
	transport := &StdioTransport{
		connected: true,
		stdin:     nil,
		cmd:       nil,
	}

	err := transport.Close()
	assert.NoError(t, err)
	assert.False(t, transport.connected)
}

// Benchmarks
func BenchmarkMCPManager_GetMCPStats(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	manager := NewMCPManager(nil, nil, log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.GetMCPStats(ctx)
	}
}

func BenchmarkMCPManager_ListMCPServers(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	manager := NewMCPManager(nil, nil, log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.ListMCPServers(ctx)
	}
}

func BenchmarkMCPClient_ListServers(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	client := NewMCPClient(log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.ListServers()
	}
}

func TestMCPManager_CallTool_WithServer(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)
	ctx := context.Background()

	// Create a mock transport that returns tools first, then call result
	callCount := 0
	mockTransport := NewMockMCPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		callCount++
		if callCount == 1 {
			// First call: listServerTools
			return map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      float64(1),
				"result": map[string]interface{}{
					"tools": []interface{}{
						map[string]interface{}{
							"name":        "test-tool",
							"description": "Test Tool",
							"inputSchema": map[string]interface{}{},
						},
					},
				},
			}, nil
		}
		// Second call: callServerTool
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(2),
			"result": map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Success result",
					},
				},
			},
		}, nil
	}

	// Register a mock server with the client
	manager.client.mu.Lock()
	manager.client.servers["test-server"] = &MCPServerConnection{
		ID:        "test-server",
		Name:      "Test Server",
		Transport: mockTransport,
		Connected: true,
		Tools: []*MCPTool{
			{Name: "test-tool", Description: "Test Tool"},
		},
	}
	manager.client.mu.Unlock()

	// Call the tool
	result, err := manager.CallTool(ctx, "test-tool", map[string]interface{}{"param": "value"})
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMCPManager_ValidateMCPRequest_AllErrors(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("non-unified request", func(t *testing.T) {
		req := map[string]interface{}{
			"tool": "test-tool",
		}
		err := manager.ValidateMCPRequest(ctx, req)
		assert.NoError(t, err) // Non-unified requests pass validation
	})

	t.Run("unified request with missing server", func(t *testing.T) {
		req := UnifiedProtocolRequest{
			ServerID: "non-existent-server",
			ToolName: "test-tool",
		}
		err := manager.ValidateMCPRequest(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not connected")
	})
}

func TestMCPManager_GetMCPStats_AllPaths(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)
	ctx := context.Background()

	t.Run("no servers", func(t *testing.T) {
		stats, err := manager.GetMCPStats(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, stats)
	})

	t.Run("with mock server", func(t *testing.T) {
		mockTransport := NewMockMCPTransport()

		manager.client.mu.Lock()
		manager.client.servers["stats-server"] = &MCPServerConnection{
			ID:        "stats-server",
			Name:      "Stats Server",
			Transport: mockTransport,
			Connected: true,
			Tools: []*MCPTool{
				{Name: "tool1", Description: "Tool 1"},
				{Name: "tool2", Description: "Tool 2"},
			},
		}
		manager.client.mu.Unlock()

		stats, err := manager.GetMCPStats(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, stats)
	})
}

func TestMCPManager_ListTools_WithServer(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)

	mockTransport := NewMockMCPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"result": map[string]interface{}{
				"tools": []interface{}{
					map[string]interface{}{
						"name":        "tool-1",
						"description": "Tool 1",
					},
				},
			},
		}, nil
	}

	manager.client.mu.Lock()
	manager.client.servers["tools-server"] = &MCPServerConnection{
		ID:        "tools-server",
		Name:      "Tools Server",
		Transport: mockTransport,
		Connected: true,
		Tools: []*MCPTool{
			{Name: "tool-1", Description: "Tool 1"},
		},
	}
	manager.client.mu.Unlock()

	tools := manager.ListTools()
	assert.GreaterOrEqual(t, len(tools), 1)
}

func TestMCPManager_GetMCPTools_WithServer(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)
	ctx := context.Background()

	mockTransport := NewMockMCPTransport()
	mockTransport.receiveFunc = func(ctx context.Context) (interface{}, error) {
		return map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"result": map[string]interface{}{
				"tools": []interface{}{
					map[string]interface{}{
						"name":        "mcp-tool-1",
						"description": "MCP Tool 1",
					},
					map[string]interface{}{
						"name":        "mcp-tool-2",
						"description": "MCP Tool 2",
					},
				},
			},
		}, nil
	}

	manager.client.mu.Lock()
	manager.client.servers["mcp-tools-server"] = &MCPServerConnection{
		ID:        "mcp-tools-server",
		Name:      "MCP Tools Server",
		Transport: mockTransport,
		Connected: true,
		Tools: []*MCPTool{
			{Name: "mcp-tool-1", Description: "MCP Tool 1"},
			{Name: "mcp-tool-2", Description: "MCP Tool 2"},
		},
	}
	manager.client.mu.Unlock()

	tools, err := manager.GetMCPTools(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, tools)
	// Count total tools across all servers
	totalTools := 0
	for _, serverTools := range tools {
		totalTools += len(serverTools)
	}
	assert.GreaterOrEqual(t, totalTools, 2)
}

func TestMCPManager_RegisterServer_Errors(t *testing.T) {
	log := newMCPTestLogger()
	manager := NewMCPManager(nil, nil, log)

	t.Run("missing name", func(t *testing.T) {
		config := map[string]interface{}{
			"command": []interface{}{"test-command"},
		}
		err := manager.RegisterServer(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must include 'name'")
	})

	t.Run("missing command", func(t *testing.T) {
		config := map[string]interface{}{
			"name": "Test Server",
		}
		err := manager.RegisterServer(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must include 'command'")
	})
}
