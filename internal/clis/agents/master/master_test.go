// Package master provides tests for the master integration
package master

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/clis/agents"
)

func TestNewMasterIntegration(t *testing.T) {
	t.Parallel()
	m, err := NewMasterIntegration()
	require.NoError(t, err)

	assert.NotNil(t, m)
	assert.NotNil(t, m.registry)
	assert.False(t, m.started)
}

func TestMasterIntegration_Start(t *testing.T) {
	t.Parallel()
	m, err := NewMasterIntegration()
	require.NoError(t, err)

	ctx := context.Background()

	err = m.Start(ctx)
	require.NoError(t, err)
	assert.True(t, m.started)

	// Starting again should be idempotent
	err = m.Start(ctx)
	require.NoError(t, err)
	assert.True(t, m.started)
}

func TestMasterIntegration_Stop(t *testing.T) {
	t.Parallel()
	m, err := NewMasterIntegration()
	require.NoError(t, err)

	ctx := context.Background()

	// Stop before start should not error
	err = m.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, m.started)

	// Start and then stop
	err = m.Start(ctx)
	require.NoError(t, err)

	err = m.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, m.started)

	// Stopping again should be idempotent
	err = m.Stop(ctx)
	require.NoError(t, err)
}

func TestMasterIntegration_IsStarted(t *testing.T) {
	t.Parallel()
	m, err := NewMasterIntegration()
	require.NoError(t, err)

	assert.False(t, m.IsStarted())

	ctx := context.Background()
	err = m.Start(ctx)
	require.NoError(t, err)

	assert.True(t, m.IsStarted())
}

func TestMasterIntegration_GetRegistry(t *testing.T) {
	t.Parallel()
	m, err := NewMasterIntegration()
	require.NoError(t, err)

	registry := m.GetRegistry()
	assert.NotNil(t, registry)

	// Should return the same registry
	registry2 := m.GetRegistry()
	assert.Equal(t, registry, registry2)
}

func TestMasterIntegration_ListAgents(t *testing.T) {
	t.Parallel()
	m, err := NewMasterIntegration()
	require.NoError(t, err)

	agents := m.ListAgents()
	assert.NotNil(t, agents)
	assert.Greater(t, len(agents), 0) // Should have registered some agents
}

func TestMasterIntegration_ListAvailable(t *testing.T) {
	t.Parallel()
	m, err := NewMasterIntegration()
	require.NoError(t, err)

	available := m.ListAvailable()
	assert.NotNil(t, available)
	// Available agents depend on system configuration
}

func TestMasterIntegration_GetAgent(t *testing.T) {
	t.Parallel()
	m, err := NewMasterIntegration()
	require.NoError(t, err)

	// Try to get a known agent type
	agent, ok := m.GetAgent(agents.TypeAider)
	// Result depends on whether aider is registered
	if ok {
		assert.NotNil(t, agent)
		info := agent.Info()
		assert.Equal(t, agents.TypeAider, info.Type)
	}

	// Try to get a non-existent agent
	agent, ok = m.GetAgent("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, agent)
}

func TestMasterIntegration_Execute(t *testing.T) {
	t.Parallel()
	m, err := NewMasterIntegration()
	require.NoError(t, err)

	ctx := context.Background()

	// Execute on a non-existent agent
	_, err = m.Execute(ctx, "nonexistent", "test", map[string]interface{}{})
	assert.Error(t, err)
}

func TestMasterIntegration_HealthCheck(t *testing.T) {
	t.Parallel()
	m, err := NewMasterIntegration()
	require.NoError(t, err)

	ctx := context.Background()

	results := m.HealthCheck(ctx)
	assert.NotNil(t, results)
	// Results depend on registered agents
}

func TestMasterIntegration_GetStats(t *testing.T) {
	t.Parallel()
	m, err := NewMasterIntegration()
	require.NoError(t, err)

	stats := m.GetStats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "total")
	assert.Contains(t, stats, "available")

	total, ok := stats["total"].(int)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, total, 0)
}

func TestGetMaster(t *testing.T) {
	t.Parallel()
	// Get master twice - should return same instance
	m1 := GetMaster()
	m2 := GetMaster()

	assert.NotNil(t, m1)
	assert.Equal(t, m1, m2)
}

func TestMasterIntegration_RegisterAllAgents(t *testing.T) {
	t.Parallel()
	m, err := NewMasterIntegration()
	require.NoError(t, err)

	// Verify that agents were registered
	agentList := m.ListAgents()
	assert.Greater(t, len(agentList), 0)

	// Check for expected agent types
	agentTypes := make(map[agents.AgentType]bool)
	for _, agent := range agentList {
		agentTypes[agent.Type] = true
	}

	// These should be registered based on master.go
	assert.Contains(t, agentTypes, agents.TypeAider)
	assert.Contains(t, agentTypes, agents.TypeOpenHands)
	assert.Contains(t, agentTypes, agents.TypeCodex)
	assert.Contains(t, agentTypes, agents.TypeCline)
	assert.Contains(t, agentTypes, agents.TypeGeminiCLI)
}

func TestMasterIntegration_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	m, err := NewMasterIntegration()
	require.NoError(t, err)

	ctx := context.Background()

	// Concurrent operations
	done := make(chan bool, 3)

	go func() {
		_ = m.Start(ctx)
		done <- true
	}()

	go func() {
		_ = m.ListAgents()
		done <- true
	}()

	go func() {
		_ = m.GetStats()
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			// Success
		case <-ctx.Done():
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}
