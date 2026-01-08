package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDiscoveryTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

// Tests for ACPClient helper functions specific to protocol_discovery.go

func TestDiscoveryACPClient_NextMessageID(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	id1 := client.nextMessageID()
	id2 := client.nextMessageID()
	id3 := client.nextMessageID()

	// Test that IDs increment sequentially
	assert.Equal(t, id1+1, id2)
	assert.Equal(t, id2+1, id3)
	assert.Greater(t, id1, 0)
}

func TestDiscoveryACPClient_UnmarshalMessage(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	t.Run("unmarshal marshal error", func(t *testing.T) {
		// Channels cannot be marshaled to JSON
		data := make(chan int)
		var message ACPMessage
		err := client.unmarshalMessage(data, &message)
		assert.Error(t, err)
	})

	t.Run("unmarshal valid message", func(t *testing.T) {
		data := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"method":  "test-method",
			"params": map[string]interface{}{
				"key": "value",
			},
		}

		var message ACPMessage
		err := client.unmarshalMessage(data, &message)
		require.NoError(t, err)
		assert.Equal(t, "2.0", message.JSONRPC)
		assert.Equal(t, "test-method", message.Method)
	})

	t.Run("unmarshal error response", func(t *testing.T) {
		data := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      float64(1),
			"error": map[string]interface{}{
				"code":    float64(-32600),
				"message": "Invalid Request",
			},
		}

		var message ACPMessage
		err := client.unmarshalMessage(data, &message)
		require.NoError(t, err)
		assert.NotNil(t, message.Error)
		assert.Equal(t, -32600, message.Error.Code)
	})
}

func TestDiscoveryACPClient_UnmarshalResult(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	t.Run("unmarshal marshal error", func(t *testing.T) {
		// Channels cannot be marshaled to JSON
		result := make(chan int)
		var target ACPInitializeResult
		err := client.unmarshalResult(result, &target)
		assert.Error(t, err)
	})

	t.Run("unmarshal initialize result", func(t *testing.T) {
		result := map[string]interface{}{
			"protocolVersion": "1.0.0",
			"capabilities":    map[string]interface{}{},
			"serverInfo": map[string]string{
				"name":    "test-agent",
				"version": "1.0.0",
			},
		}

		var target ACPInitializeResult
		err := client.unmarshalResult(result, &target)
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", target.ProtocolVersion)
	})

	t.Run("unmarshal action result", func(t *testing.T) {
		result := map[string]interface{}{
			"success": true,
			"result":  "operation completed",
		}

		var target ACPActionResult
		err := client.unmarshalResult(result, &target)
		require.NoError(t, err)
		assert.True(t, target.Success)
	})
}

func TestDiscoveryACPClient_ListAgents_Empty(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	agents := client.ListAgents()
	assert.Empty(t, agents)
}

func TestDiscoveryACPClient_HealthCheck_Empty(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)
	ctx := context.Background()

	health := client.HealthCheck(ctx)
	assert.Empty(t, health)
}

func TestDiscoveryACPClient_GetAgentStatus_NotConnected(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)
	ctx := context.Background()

	status, err := client.GetAgentStatus(ctx, "non-existent")
	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "not found")
}

func TestDiscoveryACPClient_BroadcastAction_NoAgents(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)
	ctx := context.Background()

	results := client.BroadcastAction(ctx, "test-action", nil)
	assert.Empty(t, results)
}

func TestDiscoveryACPClient_ConnectAgent_UnsupportedProtocol(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)
	ctx := context.Background()

	err := client.ConnectAgent(ctx, "test-agent", "Test Agent", "unknown://localhost")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported endpoint protocol")
}

func TestDiscoveryACPClient_ConnectAgent_DuplicateConnection(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	// Add a fake agent connection directly to test duplicate check
	client.mu.Lock()
	client.agents["test-agent"] = &ACPAgentConnection{
		ID:        "test-agent",
		Name:      "Test Agent",
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	err := client.ConnectAgent(ctx, "test-agent", "Test Agent", "http://localhost:7061")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already connected")
}

func TestDiscoveryACPClient_DisconnectAgent_NotConnected(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	err := client.DisconnectAgent("non-existent-agent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestDiscoveryACPClient_DisconnectAgent_Connected(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	// Create a mock transport that uses the existing MockACPTransport from acp_client_test.go
	mockTransport := &MockACPTransport{}
	mockTransport.connected = true

	// Add a fake agent connection
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

func TestDiscoveryACPClient_GetAgentStatus_Connected(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	now := time.Now()
	// Add a fake agent connection
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

func TestDiscoveryACPClient_GetAgentStatus_WithWebSocketTransport(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	now := time.Now()
	wsTransport := &WebSocketACPTransport{
		connected: true,
	}

	client.mu.Lock()
	client.agents["ws-agent"] = &ACPAgentConnection{
		ID:        "ws-agent",
		Name:      "WebSocket Agent",
		Connected: true,
		LastUsed:  now,
		Transport: wsTransport,
		Capabilities: map[string]interface{}{
			"tools": true,
		},
	}
	client.mu.Unlock()

	ctx := context.Background()
	status, err := client.GetAgentStatus(ctx, "ws-agent")
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, "ws-agent", status["id"])
	assert.Equal(t, "websocket", status["transport"])
	// connected status comes from wsTransport.IsConnected() which requires a real conn
	// Just verify the transport type was detected correctly
	_, hasConnected := status["connected"]
	assert.True(t, hasConnected)
}

func TestDiscoveryACPClient_GetAgentStatus_WithHTTPTransport(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	// Create a test server for HTTP transport
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	now := time.Now()
	httpTransport := &HTTPACPTransport{
		baseURL:    server.URL,
		httpClient: server.Client(),
		connected:  true,
	}

	client.mu.Lock()
	client.agents["http-agent"] = &ACPAgentConnection{
		ID:        "http-agent",
		Name:      "HTTP Agent",
		Connected: true,
		LastUsed:  now,
		Transport: httpTransport,
		Capabilities: map[string]interface{}{
			"tools": true,
		},
	}
	client.mu.Unlock()

	ctx := context.Background()
	status, err := client.GetAgentStatus(ctx, "http-agent")
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, "http-agent", status["id"])
	assert.Equal(t, "http", status["transport"])
	assert.Equal(t, true, status["connected"])
}

func TestDiscoveryACPClient_BroadcastAction_WithAgents(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	// Create mock transports that will fail (no real connection)
	mockTransport1 := &MockACPTransport{connected: false}
	mockTransport2 := &MockACPTransport{connected: false}

	// Add fake agent connections
	client.mu.Lock()
	client.agents["agent-1"] = &ACPAgentConnection{
		ID:        "agent-1",
		Name:      "Agent 1",
		Transport: mockTransport1,
		Connected: true,
	}
	client.agents["agent-2"] = &ACPAgentConnection{
		ID:        "agent-2",
		Name:      "Agent 2",
		Transport: mockTransport2,
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	results := client.BroadcastAction(ctx, "test-action", map[string]interface{}{"key": "value"})

	// Results should have 2 entries (one per agent)
	assert.Len(t, results, 2)
}

func TestDiscoveryACPClient_BroadcastAction_DisconnectedAgent(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	mockTransport := &MockACPTransport{connected: false}

	// Add an agent that is marked as disconnected
	client.mu.Lock()
	client.agents["disconnected-agent"] = &ACPAgentConnection{
		ID:        "disconnected-agent",
		Name:      "Disconnected Agent",
		Transport: mockTransport,
		Connected: false, // Agent is not connected
	}
	client.mu.Unlock()

	ctx := context.Background()
	results := client.BroadcastAction(ctx, "test-action", nil)

	// Should have result for the disconnected agent
	require.Len(t, results, 1)
	result := results["disconnected-agent"]
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "agent not connected", result.Error)
}

func TestDiscoveryACPClient_BroadcastAction_MixedAgents(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	mockTransport1 := &MockACPTransport{connected: true}
	mockTransport2 := &MockACPTransport{connected: false}

	// Add mix of connected and disconnected agents
	client.mu.Lock()
	client.agents["connected-agent"] = &ACPAgentConnection{
		ID:        "connected-agent",
		Name:      "Connected Agent",
		Transport: mockTransport1,
		Connected: true,
	}
	client.agents["disconnected-agent"] = &ACPAgentConnection{
		ID:        "disconnected-agent",
		Name:      "Disconnected Agent",
		Transport: mockTransport2,
		Connected: false,
	}
	client.mu.Unlock()

	ctx := context.Background()
	results := client.BroadcastAction(ctx, "test-action", map[string]interface{}{"key": "value"})

	// Should have results for both agents
	assert.Len(t, results, 2)

	// Disconnected agent should have error
	disconnectedResult := results["disconnected-agent"]
	require.NotNil(t, disconnectedResult)
	assert.False(t, disconnectedResult.Success)
	assert.Equal(t, "agent not connected", disconnectedResult.Error)

	// Connected agent should have a result (may have error due to mock, but not "not connected")
	connectedResult := results["connected-agent"]
	require.NotNil(t, connectedResult)
}

func TestDiscoveryACPClient_ListAgents_WithAgents(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	// Add fake agent connections
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

	// Check that agent IDs are present in the returned slice
	agentIDs := make([]string, len(agents))
	for i, a := range agents {
		agentIDs[i] = a.ID
	}
	assert.Contains(t, agentIDs, "agent-1")
	assert.Contains(t, agentIDs, "agent-2")
}

func TestDiscoveryACPClient_HealthCheck_WithAgents(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	// Create a mock transport
	mockTransport := &MockACPTransport{connected: true}

	// Add a fake agent connection
	client.mu.Lock()
	client.agents["agent-1"] = &ACPAgentConnection{
		ID:        "agent-1",
		Name:      "Agent 1",
		Transport: mockTransport,
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	health := client.HealthCheck(ctx)

	assert.Len(t, health, 1)
	assert.Contains(t, health, "agent-1")
	// The health should reflect that the mock transport is "connected"
	assert.Equal(t, true, health["agent-1"])
}

// Note: MockACPTransport is already defined in acp_client_test.go

// WebSocketACPTransport tests for protocol_discovery.go

func TestDiscoveryWebSocketACPTransport_IsConnected_NotConnected(t *testing.T) {
	transport := &WebSocketACPTransport{
		connected: false,
		conn:      nil,
	}

	assert.False(t, transport.IsConnected())
}

func TestDiscoveryWebSocketACPTransport_Close_NilConnection(t *testing.T) {
	transport := &WebSocketACPTransport{
		connected: true,
		conn:      nil,
	}

	err := transport.Close()
	assert.NoError(t, err)
	assert.False(t, transport.connected)
}

// Additional structure tests for protocol_discovery.go types

func TestDiscoveryACPInitializeRequest_Structure(t *testing.T) {
	req := ACPInitializeRequest{
		ProtocolVersion: "1.0.0",
		Capabilities:    map[string]interface{}{"streaming": true},
		ClientInfo: map[string]string{
			"name":    "helixagent",
			"version": "1.0.0",
		},
	}

	assert.Equal(t, "1.0.0", req.ProtocolVersion)
	assert.Equal(t, true, req.Capabilities["streaming"])
	assert.Equal(t, "helixagent", req.ClientInfo["name"])
}

func TestDiscoveryACPInitializeResult_Structure(t *testing.T) {
	result := ACPInitializeResult{
		ProtocolVersion: "1.0.0",
		Capabilities:    map[string]interface{}{"tools": true},
		ServerInfo: map[string]string{
			"name":    "test-server",
			"version": "1.0.0",
		},
		Instructions: "Follow these instructions",
	}

	assert.Equal(t, "1.0.0", result.ProtocolVersion)
	assert.Equal(t, "Follow these instructions", result.Instructions)
}

func TestDiscoveryACPActionRequest_Structure(t *testing.T) {
	req := ACPActionRequest{
		Action: "execute",
		Params: map[string]interface{}{"arg1": "value1"},
		Context: map[string]interface{}{
			"workspace": "/workspace",
		},
	}

	assert.Equal(t, "execute", req.Action)
	assert.Equal(t, "value1", req.Params["arg1"])
	assert.Equal(t, "/workspace", req.Context["workspace"])
}

func TestDiscoveryACPActionResult_Structure(t *testing.T) {
	result := ACPActionResult{
		Success: true,
		Result:  "operation completed",
		Error:   "",
	}

	assert.True(t, result.Success)
	assert.Equal(t, "operation completed", result.Result)
	assert.Empty(t, result.Error)
}

func TestDiscoveryACPAgentConnection_Structure(t *testing.T) {
	now := time.Now()
	conn := &ACPAgentConnection{
		ID:           "agent-1",
		Name:         "Test Agent",
		Transport:    nil,
		Capabilities: map[string]interface{}{"tools": true},
		Connected:    true,
		LastUsed:     now,
	}

	assert.Equal(t, "agent-1", conn.ID)
	assert.Equal(t, "Test Agent", conn.Name)
	assert.True(t, conn.Connected)
}

// Benchmarks for protocol_discovery.go

func BenchmarkDiscoveryACPClient_NextMessageID(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	client := NewACPDiscoveryClient(log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.nextMessageID()
	}
}

func BenchmarkDiscoveryACPClient_UnmarshalMessage(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	client := NewACPDiscoveryClient(log)

	data := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      float64(1),
		"method":  "test-method",
		"params":  map[string]interface{}{"key": "value"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var msg ACPMessage
		_ = client.unmarshalMessage(data, &msg)
	}
}

func TestDiscoveryACPClient_ExecuteAction_NotConnected(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)
	ctx := context.Background()

	result, err := client.ExecuteAction(ctx, "non-existent-agent", "test-action", map[string]interface{}{})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not connected")
}

func TestDiscoveryACPClient_ExecuteAction_Connected(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)
	ctx := context.Background()

	// Create a mock transport
	mockTransport := &MockDiscoveryACPTransport{
		connected: true,
		receiveFunc: func(ctx context.Context) (interface{}, error) {
			return map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      float64(1),
				"result": map[string]interface{}{
					"success": true,
					"data":    map[string]interface{}{"key": "value"},
				},
			}, nil
		},
	}

	// Add agent connection
	client.mu.Lock()
	client.agents["test-agent"] = &ACPAgentConnection{
		ID:        "test-agent",
		Name:      "Test Agent",
		Transport: mockTransport,
		Connected: true,
	}
	client.mu.Unlock()

	result, err := client.ExecuteAction(ctx, "test-agent", "test-action", map[string]interface{}{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDiscoveryACPClient_GetAgentCapabilities_NotConnected(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	result, err := client.GetAgentCapabilities("non-existent-agent")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not connected")
}

func TestDiscoveryACPClient_GetAgentCapabilities_Connected(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPDiscoveryClient(log)

	// Add agent connection with capabilities
	client.mu.Lock()
	client.agents["test-agent"] = &ACPAgentConnection{
		ID:        "test-agent",
		Name:      "Test Agent",
		Connected: true,
		Capabilities: map[string]interface{}{
			"execute":   true,
			"broadcast": true,
		},
	}
	client.mu.Unlock()

	result, err := client.GetAgentCapabilities("test-agent")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, true, result["execute"])
}

// MockDiscoveryACPTransport for testing
type MockDiscoveryACPTransport struct {
	connected   bool
	sendFunc    func(ctx context.Context, message interface{}) error
	receiveFunc func(ctx context.Context) (interface{}, error)
	closeFunc   func() error
}

func (m *MockDiscoveryACPTransport) Send(ctx context.Context, message interface{}) error {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, message)
	}
	return nil
}

func (m *MockDiscoveryACPTransport) Receive(ctx context.Context) (interface{}, error) {
	if m.receiveFunc != nil {
		return m.receiveFunc(ctx)
	}
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      float64(1),
		"result":  map[string]interface{}{},
	}, nil
}

func (m *MockDiscoveryACPTransport) Close() error {
	m.connected = false
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *MockDiscoveryACPTransport) IsConnected() bool {
	return m.connected
}

func TestDiscoveryHTTPACPTransport_IsConnected(t *testing.T) {
	t.Run("not connected returns false immediately", func(t *testing.T) {
		transport := &HTTPACPTransport{
			baseURL:   "http://test.example.com",
			connected: false,
		}
		assert.False(t, transport.IsConnected())
	})

	t.Run("connected with healthy server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		transport := &HTTPACPTransport{
			baseURL:    server.URL,
			httpClient: server.Client(),
			connected:  true,
		}
		assert.True(t, transport.IsConnected())
	})

	t.Run("connected with unhealthy server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		transport := &HTTPACPTransport{
			baseURL:    server.URL,
			httpClient: server.Client(),
			connected:  true,
		}
		assert.False(t, transport.IsConnected())
	})
}
