package services

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMCPClientTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

func TestMCPClient_GetServerInfo_NotConnected(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)

	info, err := client.GetServerInfo("non-existent")
	assert.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "not connected")
}

func TestMCPClient_DisconnectServer_NotConnected(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)

	err := client.DisconnectServer("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestMCPClient_ListTools_NoServers(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)
	ctx := context.Background()

	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	assert.Len(t, tools, 0)
}

func TestMCPClient_CallTool_NotConnected(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)
	ctx := context.Background()

	result, err := client.CallTool(ctx, "server", "tool", map[string]interface{}{})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not connected")
}

func TestMCPClient_NextMessageID(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)

	id1 := client.nextMessageID()
	id2 := client.nextMessageID()
	id3 := client.nextMessageID()

	// Test that IDs increment sequentially
	assert.Equal(t, id1+1, id2)
	assert.Equal(t, id2+1, id3)
	assert.Greater(t, id1, 0)
}

func TestMCPClient_ValidateToolArguments(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)

	t.Run("no required fields", func(t *testing.T) {
		tool := &MCPTool{
			Name:        "test-tool",
			Description: "A test tool",
			InputSchema: map[string]interface{}{},
		}
		err := client.validateToolArguments(tool, map[string]interface{}{})
		assert.NoError(t, err)
	})

	t.Run("required fields present", func(t *testing.T) {
		tool := &MCPTool{
			Name:        "test-tool",
			Description: "A test tool",
			InputSchema: map[string]interface{}{
				"required": []interface{}{"arg1", "arg2"},
			},
		}
		err := client.validateToolArguments(tool, map[string]interface{}{
			"arg1": "value1",
			"arg2": "value2",
		})
		assert.NoError(t, err)
	})

	t.Run("required field missing", func(t *testing.T) {
		tool := &MCPTool{
			Name:        "test-tool",
			Description: "A test tool",
			InputSchema: map[string]interface{}{
				"required": []interface{}{"arg1", "arg2"},
			},
		}
		err := client.validateToolArguments(tool, map[string]interface{}{
			"arg1": "value1",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required field 'arg2' is missing")
	})

	t.Run("all required fields missing", func(t *testing.T) {
		tool := &MCPTool{
			Name:        "test-tool",
			Description: "A test tool",
			InputSchema: map[string]interface{}{
				"required": []interface{}{"arg1"},
			},
		}
		err := client.validateToolArguments(tool, map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required field 'arg1' is missing")
	})
}

func TestMCPClient_UnmarshalResponse(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)

	t.Run("valid response", func(t *testing.T) {
		data := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1), // JSON unmarshals numbers as float64
			"result": map[string]interface{}{
				"tools": []interface{}{},
			},
		}

		var response MCPResponse
		err := client.unmarshalResponse(data, &response)
		require.NoError(t, err)
		assert.Equal(t, "2.0", response.JSONRPC)
		assert.Equal(t, float64(1), response.ID) // ID field is interface{}, stays as float64
		assert.NotNil(t, response.Result)
	})

	t.Run("error response", func(t *testing.T) {
		data := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"error": map[string]interface{}{
				"code":    -32600,
				"message": "Invalid Request",
			},
		}

		var response MCPResponse
		err := client.unmarshalResponse(data, &response)
		require.NoError(t, err)
		assert.NotNil(t, response.Error)
	})
}

func TestMCPClient_UnmarshalResult(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)

	t.Run("unmarshal tools list", func(t *testing.T) {
		result := map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"name":        "tool1",
					"description": "Tool 1",
				},
			},
		}

		var target struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		}

		err := client.unmarshalResult(result, &target)
		require.NoError(t, err)
		assert.Len(t, target.Tools, 1)
		assert.Equal(t, "tool1", target.Tools[0].Name)
	})

	t.Run("unmarshal server info", func(t *testing.T) {
		result := map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]interface{}{
				"name":    "test-server",
				"version": "1.0.0",
			},
		}

		var target struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		}

		err := client.unmarshalResult(result, &target)
		require.NoError(t, err)
		assert.Equal(t, "2024-11-05", target.ProtocolVersion)
		assert.Equal(t, "test-server", target.ServerInfo.Name)
	})
}

// Benchmarks

func BenchmarkMCPClient_NextMessageID(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	client := NewMCPClient(log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.nextMessageID()
	}
}

func BenchmarkMCPClient_ValidateToolArguments(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	client := NewMCPClient(log)

	tool := &MCPTool{
		Name: "test-tool",
		InputSchema: map[string]interface{}{
			"required": []interface{}{"arg1", "arg2"},
		},
	}
	args := map[string]interface{}{
		"arg1": "value1",
		"arg2": "value2",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.validateToolArguments(tool, args)
	}
}

// MockMCPTransport implements MCPTransport for testing
type MockMCPTransport struct {
	connected    bool
	sendFunc     func(ctx context.Context, message interface{}) error
	receiveFunc  func(ctx context.Context) (interface{}, error)
	closeFunc    func() error
	sendCalls    []interface{}
	receiveCalls int
}

func NewMockMCPTransport() *MockMCPTransport {
	return &MockMCPTransport{
		connected: true,
		sendCalls: make([]interface{}, 0),
	}
}

func (m *MockMCPTransport) Send(ctx context.Context, message interface{}) error {
	m.sendCalls = append(m.sendCalls, message)
	if m.sendFunc != nil {
		return m.sendFunc(ctx, message)
	}
	return nil
}

func (m *MockMCPTransport) Receive(ctx context.Context) (interface{}, error) {
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

func (m *MockMCPTransport) Close() error {
	m.connected = false
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *MockMCPTransport) IsConnected() bool {
	return m.connected
}

// Tests with mock servers

func TestMCPClient_ListServers_WithServers(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)

	client.mu.Lock()
	client.servers["server-1"] = &MCPServerConnection{
		ID:        "server-1",
		Name:      "Server 1",
		Connected: true,
	}
	client.servers["server-2"] = &MCPServerConnection{
		ID:        "server-2",
		Name:      "Server 2",
		Connected: true,
	}
	client.mu.Unlock()

	servers := client.ListServers()
	assert.Len(t, servers, 2)
}

func TestMCPClient_GetServerInfo_Connected(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)

	client.mu.Lock()
	client.servers["test-server"] = &MCPServerConnection{
		ID:   "test-server",
		Name: "Test Server",
		Capabilities: map[string]interface{}{
			"tools": true,
		},
		Connected: true,
	}
	client.mu.Unlock()

	info, err := client.GetServerInfo("test-server")
	require.NoError(t, err)
	assert.NotNil(t, info)
}

func TestMCPClient_DisconnectServer_Connected(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)

	mockTransport := NewMockMCPTransport()

	client.mu.Lock()
	client.servers["test-server"] = &MCPServerConnection{
		ID:        "test-server",
		Name:      "Test Server",
		Transport: mockTransport,
		Connected: true,
	}
	client.mu.Unlock()

	err := client.DisconnectServer("test-server")
	require.NoError(t, err)

	// Verify server was removed
	client.mu.RLock()
	_, exists := client.servers["test-server"]
	client.mu.RUnlock()
	assert.False(t, exists)
}

func TestMCPClient_ListTools_WithServers(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)

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
					map[string]interface{}{
						"name":        "tool-2",
						"description": "Tool 2",
					},
				},
			},
		}, nil
	}

	client.mu.Lock()
	client.servers["test-server"] = &MCPServerConnection{
		ID:        "test-server",
		Name:      "Test Server",
		Transport: mockTransport,
		Tools: []*MCPTool{
			{Name: "tool-1", Description: "Tool 1"},
			{Name: "tool-2", Description: "Tool 2"},
		},
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	assert.Len(t, tools, 2)
}

// Note: CallTool tests with mock transports require complex setup
// because the actual implementation calls listServerTools which needs transport

func TestMCPClient_HealthCheck_Empty(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)

	ctx := context.Background()
	health := client.HealthCheck(ctx)
	assert.Empty(t, health)
}

func TestMCPClient_HealthCheck_WithServers(t *testing.T) {
	log := newMCPClientTestLogger()
	client := NewMCPClient(log)

	mockTransport := NewMockMCPTransport()
	mockTransport.connected = true

	client.mu.Lock()
	client.servers["test-server"] = &MCPServerConnection{
		ID:        "test-server",
		Transport: mockTransport,
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	health := client.HealthCheck(ctx)

	assert.Len(t, health, 1)
	assert.Contains(t, health, "test-server")
	assert.Equal(t, true, health["test-server"])
}

// Note: MCPServerConnection, MCPTool, MCPResponse, MCPError structure tests
// are already defined in mcp_manager_test.go
