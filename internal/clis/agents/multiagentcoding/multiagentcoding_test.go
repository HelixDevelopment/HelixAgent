// Package multiagentcoding provides tests for Multi-Agent Coding integration
package multiagentcoding

import (
	"context"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMultiAgentCoding(t *testing.T) {
	t.Parallel()
	m := New()
	require.NotNil(t, m)

	info := m.Info()
	assert.Equal(t, agents.TypeMultiagentCoding, info.Type)
	assert.Equal(t, "Multi-Agent Coding", info.Name)
	assert.Equal(t, "MultiAgent", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestMultiAgentCodingInitialize(t *testing.T) {
	t.Parallel()
	m := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		AgentCount: 5,
	}

	err := m.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, 5, m.config.AgentCount)
}

func TestMultiAgentCodingInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	m := New()
	ctx := context.Background()

	err := m.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, m.config.AgentCount) // Default value
}

func TestMultiAgentCodingStartStop(t *testing.T) {
	t.Parallel()
	m := New()
	ctx := context.Background()

	err := m.Initialize(ctx, nil)
	require.NoError(t, err)

	err = m.Start(ctx)
	require.NoError(t, err)
	assert.True(t, m.IsStarted())

	err = m.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, m.IsStarted())
}

func TestMultiAgentCodingExecute(t *testing.T) {
	t.Parallel()
	m := New()
	ctx := context.Background()

	err := m.Initialize(ctx, nil)
	require.NoError(t, err)

	tests := []struct {
		name    string
		command string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "collaborate command",
			command: "collaborate",
			params: map[string]interface{}{
				"task": "Implement auth",
			},
			wantErr: false,
		},
		{
			name:    "collaborate without task fails",
			command: "collaborate",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "status command",
			command: "status",
			params:  map[string]interface{}{},
			wantErr: false,
		},
		{
			name:    "unknown command",
			command: "unknown",
			params:  map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := m.Execute(ctx, tt.command, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestMultiAgentCodingCapabilities(t *testing.T) {
	t.Parallel()
	m := New()
	info := m.Info()

	expectedCaps := []string{"multi_agent", "collaboration", "code_generation"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

func TestMultiAgentCodingIsAvailable(t *testing.T) {
	t.Parallel()
	m := New()
	assert.True(t, m.IsAvailable())
}

func TestMultiAgentCodingCollaborateResult(t *testing.T) {
	t.Parallel()
	m := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		AgentCount: 4,
	}

	err := m.Initialize(ctx, config)
	require.NoError(t, err)

	result, err := m.Execute(ctx, "collaborate", map[string]interface{}{
		"task": "Build API",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 4, resultMap["agents"])
	assert.Equal(t, "Build API", resultMap["task"])
	assert.Equal(t, "completed", resultMap["status"])
}
