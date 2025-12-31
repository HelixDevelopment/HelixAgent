package services

import (
	"context"
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
	client := NewACPClient(log)

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
	client := NewACPClient(log)

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
	client := NewACPClient(log)

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
	client := NewACPClient(log)

	agents := client.ListAgents()
	assert.Empty(t, agents)
}

func TestDiscoveryACPClient_HealthCheck_Empty(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPClient(log)
	ctx := context.Background()

	health := client.HealthCheck(ctx)
	assert.Empty(t, health)
}

func TestDiscoveryACPClient_GetAgentStatus_NotConnected(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPClient(log)
	ctx := context.Background()

	status, err := client.GetAgentStatus(ctx, "non-existent")
	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "not found")
}

func TestDiscoveryACPClient_BroadcastAction_NoAgents(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPClient(log)
	ctx := context.Background()

	results := client.BroadcastAction(ctx, "test-action", nil)
	assert.Empty(t, results)
}

func TestDiscoveryACPClient_ConnectAgent_UnsupportedProtocol(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPClient(log)
	ctx := context.Background()

	err := client.ConnectAgent(ctx, "test-agent", "Test Agent", "unknown://localhost")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported endpoint protocol")
}

func TestDiscoveryACPClient_ConnectAgent_DuplicateConnection(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPClient(log)

	// Add a fake agent connection directly to test duplicate check
	client.mu.Lock()
	client.agents["test-agent"] = &ACPAgentConnection{
		ID:        "test-agent",
		Name:      "Test Agent",
		Connected: true,
	}
	client.mu.Unlock()

	ctx := context.Background()
	err := client.ConnectAgent(ctx, "test-agent", "Test Agent", "http://localhost:8080")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already connected")
}

func TestDiscoveryACPClient_DisconnectAgent_NotConnected(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPClient(log)

	err := client.DisconnectAgent("non-existent-agent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestDiscoveryACPClient_DisconnectAgent_Connected(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPClient(log)

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
	client := NewACPClient(log)

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

func TestDiscoveryACPClient_BroadcastAction_WithAgents(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPClient(log)

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

func TestDiscoveryACPClient_ListAgents_WithAgents(t *testing.T) {
	log := newDiscoveryTestLogger()
	client := NewACPClient(log)

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
	client := NewACPClient(log)

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
			"name":    "superagent",
			"version": "1.0.0",
		},
	}

	assert.Equal(t, "1.0.0", req.ProtocolVersion)
	assert.Equal(t, true, req.Capabilities["streaming"])
	assert.Equal(t, "superagent", req.ClientInfo["name"])
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
	client := NewACPClient(log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.nextMessageID()
	}
}

func BenchmarkDiscoveryACPClient_UnmarshalMessage(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	client := NewACPClient(log)

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
