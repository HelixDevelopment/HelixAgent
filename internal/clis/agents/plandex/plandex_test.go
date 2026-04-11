// Package plandex provides tests for Plandex agent integration
package plandex

import (
	"context"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPlandex(t *testing.T) {
	t.Parallel()
	p := New()
	require.NotNil(t, p)

	info := p.Info()
	assert.Equal(t, agents.TypePlandex, info.Type)
	assert.Equal(t, "Plandex", info.Name)
	assert.Equal(t, "Plandex", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestPlandexInitialize(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Mode: "manual",
	}

	err := p.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "manual", p.config.Mode)
}

func TestPlandexInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	err := p.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "auto", p.config.Mode) // Default value
}

func TestPlandexStartStop(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	err := p.Initialize(ctx, nil)
	require.NoError(t, err)

	err = p.Start(ctx)
	require.NoError(t, err)
	assert.True(t, p.IsStarted())

	err = p.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, p.IsStarted())
}

func TestPlandexExecute(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	err := p.Initialize(ctx, nil)
	require.NoError(t, err)

	tests := []struct {
		name    string
		command string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "plan command",
			command: "plan",
			params: map[string]interface{}{
				"task": "Build an API",
			},
			wantErr: false,
		},
		{
			name:    "plan without task fails",
			command: "plan",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "execute command",
			command: "execute",
			params: map[string]interface{}{
				"task": "Implement auth",
			},
			wantErr: false,
		},
		{
			name:    "execute without task fails",
			command: "execute",
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
			result, err := p.Execute(ctx, tt.command, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestPlandexCapabilities(t *testing.T) {
	t.Parallel()
	p := New()
	info := p.Info()

	expectedCaps := []string{"task_planning", "execution", "multi_step"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

func TestPlandexIsAvailable(t *testing.T) {
	t.Parallel()
	p := New()
	assert.True(t, p.IsAvailable())
}

func TestPlandexPlanResult(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	err := p.Initialize(ctx, nil)
	require.NoError(t, err)

	result, err := p.Execute(ctx, "plan", map[string]interface{}{
		"task": "Create microservice",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Create microservice", resultMap["task"])
	assert.NotNil(t, resultMap["plan"])
}

func TestPlandexExecuteResult(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Mode: "review",
	}

	err := p.Initialize(ctx, config)
	require.NoError(t, err)

	result, err := p.Execute(ctx, "execute", map[string]interface{}{
		"task": "Deploy app",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Deploy app", resultMap["task"])
	assert.Equal(t, "review", resultMap["mode"])
}
