// Package noi provides tests for Noi agent integration
package noi

import (
	"context"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNoi(t *testing.T) {
	t.Parallel()
	n := New()
	require.NotNil(t, n)
	
	info := n.Info()
	assert.Equal(t, agents.TypeNoi, info.Type)
	assert.Equal(t, "Noi", info.Name)
	assert.Equal(t, "Noi", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestNoiInitialize(t *testing.T) {
	t.Parallel()
	n := New()
	ctx := context.Background()
	
	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Model: "claude-3",
	}
	
	err := n.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "claude-3", n.config.Model)
}

func TestNoiInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	n := New()
	ctx := context.Background()
	
	err := n.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4", n.config.Model) // Default value
}

func TestNoiStartStop(t *testing.T) {
	t.Parallel()
	n := New()
	ctx := context.Background()
	
	err := n.Initialize(ctx, nil)
	require.NoError(t, err)
	
	err = n.Start(ctx)
	require.NoError(t, err)
	assert.True(t, n.IsStarted())
	
	err = n.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, n.IsStarted())
}

func TestNoiExecute(t *testing.T) {
	t.Parallel()
	n := New()
	ctx := context.Background()
	
	err := n.Initialize(ctx, nil)
	require.NoError(t, err)
	
	tests := []struct {
		name    string
		command string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "refactor command",
			command: "refactor",
			params: map[string]interface{}{
				"code": "func main() { print('hello') }",
			},
			wantErr: false,
		},
		{
			name:    "refactor without code fails",
			command: "refactor",
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
			result, err := n.Execute(ctx, tt.command, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestNoiCapabilities(t *testing.T) {
	t.Parallel()
	n := New()
	info := n.Info()
	
	expectedCaps := []string{"refactoring", "code_improvement"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

func TestNoiIsAvailable(t *testing.T) {
	t.Parallel()
	n := New()
	assert.True(t, n.IsAvailable())
}

func TestNoiRefactorResult(t *testing.T) {
	t.Parallel()
	n := New()
	ctx := context.Background()
	
	err := n.Initialize(ctx, nil)
	require.NoError(t, err)
	
	code := "func main() { println('hello') }"
	result, err := n.Execute(ctx, "refactor", map[string]interface{}{
		"code": code,
	})
	require.NoError(t, err)
	
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, code, resultMap["code"])
}
