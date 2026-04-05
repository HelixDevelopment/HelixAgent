// Package nanocoder provides tests for Nanocoder agent integration
package nanocoder

import (
	"context"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNanocoder(t *testing.T) {
	t.Parallel()
	n := New()
	require.NotNil(t, n)
	
	info := n.Info()
	assert.Equal(t, agents.TypeNanocoder, info.Type)
	assert.Equal(t, "Nanocoder", info.Name)
	assert.Equal(t, "Nanocoder", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestNanocoderInitialize(t *testing.T) {
	t.Parallel()
	n := New()
	ctx := context.Background()
	
	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Model: "custom-nano",
	}
	
	err := n.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "custom-nano", n.config.Model)
}

func TestNanocoderInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	n := New()
	ctx := context.Background()
	
	err := n.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "nano", n.config.Model) // Default value
}

func TestNanocoderStartStop(t *testing.T) {
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

func TestNanocoderExecute(t *testing.T) {
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
			name:    "generate command",
			command: "generate",
			params: map[string]interface{}{
				"prompt": "Create a function",
			},
			wantErr: false,
		},
		{
			name:    "generate without prompt fails",
			command: "generate",
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

func TestNanocoderCapabilities(t *testing.T) {
	t.Parallel()
	n := New()
	info := n.Info()
	
	expectedCaps := []string{"minimal", "fast", "code_generation"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

func TestNanocoderIsAvailable(t *testing.T) {
	t.Parallel()
	n := New()
	assert.True(t, n.IsAvailable())
}

func TestNanocoderGenerateResult(t *testing.T) {
	t.Parallel()
	n := New()
	ctx := context.Background()
	
	err := n.Initialize(ctx, nil)
	require.NoError(t, err)
	
	result, err := n.Execute(ctx, "generate", map[string]interface{}{
		"prompt": "Create a struct",
	})
	require.NoError(t, err)
	
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Create a struct", resultMap["prompt"])
}
