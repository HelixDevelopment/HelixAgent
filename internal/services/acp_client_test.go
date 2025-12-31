package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newACPTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

// MockACPTransport implements ACPTransport for testing
type MockACPTransport struct {
	connected     bool
	sendFunc      func(ctx context.Context, message interface{}) error
	receiveFunc   func(ctx context.Context) (interface{}, error)
	closeFunc     func() error
	sendCalls     []interface{}
	receiveCalls  int
}

func NewMockACPTransport() *MockACPTransport {
	return &MockACPTransport{
		connected:  true,
		sendCalls:  make([]interface{}, 0),
	}
}

func (m *MockACPTransport) Send(ctx context.Context, message interface{}) error {
	m.sendCalls = append(m.sendCalls, message)
	if m.sendFunc != nil {
		return m.sendFunc(ctx, message)
	}
	return nil
}

func (m *MockACPTransport) Receive(ctx context.Context) (interface{}, error) {
	m.receiveCalls++
	if m.receiveFunc != nil {
		return m.receiveFunc(ctx)
	}
	// Return a mock initialize response
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]interface{}{
			"protocolVersion": "1.0.0",
			"capabilities":    map[string]interface{}{},
			"serverInfo":      map[string]string{"name": "mock-server"},
		},
	}, nil
}

func (m *MockACPTransport) Close() error {
	m.connected = false
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *MockACPTransport) IsConnected() bool {
	return m.connected
}

func TestNewACPClient(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPClient(log)

	require.NotNil(t, client)
	assert.NotNil(t, client.agents)
	assert.Equal(t, 1, client.messageID)
}

func TestACPClient_ListAgents(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPClient(log)

	t.Run("empty agents list", func(t *testing.T) {
		agents := client.ListAgents()
		assert.Empty(t, agents)
	})
}

func TestACPClient_HealthCheck(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPClient(log)

	t.Run("empty health check", func(t *testing.T) {
		results := client.HealthCheck(context.Background())
		assert.Empty(t, results)
	})
}

func TestACPClient_GetAgentCapabilities_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPClient(log)

	caps, err := client.GetAgentCapabilities("non-existent")
	assert.Error(t, err)
	assert.Nil(t, caps)
	assert.Contains(t, err.Error(), "not connected")
}

func TestACPClient_DisconnectAgent_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPClient(log)

	err := client.DisconnectAgent("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestACPClient_ExecuteAction_NotConnected(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPClient(log)

	result, err := client.ExecuteAction(context.Background(), "non-existent", "test", nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not connected")
}

func TestACPClient_GetAgentStatus_NotFound(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPClient(log)

	status, err := client.GetAgentStatus(context.Background(), "non-existent")
	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "not found")
}

func TestACPClient_BroadcastAction_Empty(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPClient(log)

	results := client.BroadcastAction(context.Background(), "test", nil)
	assert.Empty(t, results)
}

func TestACPClient_ConnectAgent_InvalidProtocol(t *testing.T) {
	log := newACPTestLogger()
	client := NewACPClient(log)

	err := client.ConnectAgent(context.Background(), "agent1", "Test Agent", "invalid://endpoint")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported endpoint protocol")
}

func TestACPAgentConnection_Structure(t *testing.T) {
	now := time.Now()
	connection := &ACPAgentConnection{
		ID:           "agent-123",
		Name:         "Test Agent",
		Transport:    nil,
		Capabilities: map[string]interface{}{"streaming": true},
		Connected:    true,
		LastUsed:     now,
	}

	assert.Equal(t, "agent-123", connection.ID)
	assert.Equal(t, "Test Agent", connection.Name)
	assert.True(t, connection.Connected)
	assert.Equal(t, true, connection.Capabilities["streaming"])
}

func TestACPMessage_Structure(t *testing.T) {
	message := &ACPMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  map[string]interface{}{"key": "value"},
		Result:  nil,
		Error:   nil,
	}

	assert.Equal(t, "2.0", message.JSONRPC)
	assert.Equal(t, 1, message.ID)
	assert.Equal(t, "initialize", message.Method)
}

func TestACPError_Structure(t *testing.T) {
	acpError := &ACPError{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    map[string]interface{}{"details": "error details"},
	}

	assert.Equal(t, -32600, acpError.Code)
	assert.Equal(t, "Invalid Request", acpError.Message)
}

func TestACPInitializeRequest_Structure(t *testing.T) {
	request := &ACPInitializeRequest{
		ProtocolVersion: "1.0.0",
		Capabilities:    map[string]interface{}{"streaming": true},
		ClientInfo: map[string]string{
			"name":    "test-client",
			"version": "1.0.0",
		},
	}

	assert.Equal(t, "1.0.0", request.ProtocolVersion)
	assert.Equal(t, true, request.Capabilities["streaming"])
	assert.Equal(t, "test-client", request.ClientInfo["name"])
}

func TestACPInitializeResult_Structure(t *testing.T) {
	result := &ACPInitializeResult{
		ProtocolVersion: "1.0.0",
		Capabilities:    map[string]interface{}{"tools": true},
		ServerInfo: map[string]string{
			"name":    "test-server",
			"version": "1.0.0",
		},
		Instructions: "Use tools for...",
	}

	assert.Equal(t, "1.0.0", result.ProtocolVersion)
	assert.Equal(t, "Use tools for...", result.Instructions)
}

func TestACPActionRequest_Structure(t *testing.T) {
	request := &ACPActionRequest{
		Action: "execute_tool",
		Params: map[string]interface{}{"tool": "calculator"},
		Context: map[string]interface{}{
			"session": "session-123",
		},
	}

	assert.Equal(t, "execute_tool", request.Action)
	assert.Equal(t, "calculator", request.Params["tool"])
}

func TestACPActionResult_Structure(t *testing.T) {
	result := &ACPActionResult{
		Success: true,
		Result:  map[string]interface{}{"output": "result"},
		Error:   "",
	}

	assert.True(t, result.Success)
	assert.Empty(t, result.Error)
}

func TestWebSocketACPTransport_Close(t *testing.T) {
	transport := &WebSocketACPTransport{
		conn:      nil,
		connected: true,
	}

	err := transport.Close()
	assert.NoError(t, err)
	assert.False(t, transport.connected)
}

func TestWebSocketACPTransport_IsConnected(t *testing.T) {
	t.Run("not connected", func(t *testing.T) {
		transport := &WebSocketACPTransport{
			conn:      nil,
			connected: false,
		}

		assert.False(t, transport.IsConnected())
	})
}

func TestHTTPACPTransport_Close(t *testing.T) {
	transport := &HTTPACPTransport{
		baseURL:   "http://localhost:8080",
		connected: true,
	}

	err := transport.Close()
	assert.NoError(t, err)
	assert.False(t, transport.connected)
}

func TestHTTPACPTransport_Send_NotConnected(t *testing.T) {
	transport := &HTTPACPTransport{
		baseURL:   "http://localhost:8080",
		connected: false,
	}

	err := transport.Send(context.Background(), map[string]interface{}{"test": "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestHTTPACPTransport_Receive(t *testing.T) {
	transport := &HTTPACPTransport{
		baseURL:   "http://localhost:8080",
		connected: true,
	}

	// HTTP transport doesn't support receive
	result, err := transport.Receive(context.Background())
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "does not support receive")
}

func TestWebSocketACPTransport_Send_NotConnected(t *testing.T) {
	transport := &WebSocketACPTransport{
		conn:      nil,
		connected: false,
	}

	err := transport.Send(context.Background(), map[string]interface{}{"test": "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestWebSocketACPTransport_Receive_NotConnected(t *testing.T) {
	transport := &WebSocketACPTransport{
		conn:      nil,
		connected: false,
	}

	result, err := transport.Receive(context.Background())
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not connected")
}

func BenchmarkACPClient_ListAgents(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	client := NewACPClient(log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.ListAgents()
	}
}

func BenchmarkACPClient_HealthCheck(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	client := NewACPClient(log)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.HealthCheck(ctx)
	}
}
