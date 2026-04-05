// Package opencodecli provides tests for Opencode CLI agent integration
package opencodecli

import (
	"context"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpencodeCLI(t *testing.T) {
	t.Parallel()
	o := New()
	require.NotNil(t, o)
	
	info := o.Info()
	assert.Equal(t, agents.TypeOpencodeCLI, info.Type)
	assert.Equal(t, "Opencode CLI", info.Name)
	assert.Equal(t, "Opencode", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestOpencodeCLIInitialize(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()
	
	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Model: "gpt-4",
	}
	
	err := o.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "gpt-4", o.config.Model)
}

func TestOpencodeCLIInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()
	
	err := o.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "default", o.config.Model) // Default value
}

func TestOpencodeCLIStartStop(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()
	
	err := o.Initialize(ctx, nil)
	require.NoError(t, err)
	
	err = o.Start(ctx)
	require.NoError(t, err)
	assert.True(t, o.IsStarted())
	
	err = o.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, o.IsStarted())
}

func TestOpencodeCLIExecute(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()
	
	err := o.Initialize(ctx, nil)
	require.NoError(t, err)
	
	tests := []struct {
		name    string
		command string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "chat command",
			command: "chat",
			params: map[string]interface{}{
				"message": "Hello",
			},
			wantErr: false,
		},
		{
			name:    "chat without message fails",
			command: "chat",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "generate command",
			command: "generate",
			params: map[string]interface{}{
				"prompt": "Create code",
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
			result, err := o.Execute(ctx, tt.command, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestOpencodeCLICapabilities(t *testing.T) {
	t.Parallel()
	o := New()
	info := o.Info()
	
	expectedCaps := []string{"open_source", "code_generation", "chat"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

func TestOpencodeCLIIsAvailable(t *testing.T) {
	t.Parallel()
	o := New()
	assert.True(t, o.IsAvailable())
}

func TestOpencodeCLIChatResult(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()
	
	err := o.Initialize(ctx, nil)
	require.NoError(t, err)
	
	result, err := o.Execute(ctx, "chat", map[string]interface{}{
		"message": "How are you?",
	})
	require.NoError(t, err)
	
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "How are you?", resultMap["message"])
}

func TestOpencodeCLIGenerateResult(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()
	
	err := o.Initialize(ctx, nil)
	require.NoError(t, err)
	
	result, err := o.Execute(ctx, "generate", map[string]interface{}{
		"prompt": "Create a handler",
	})
	require.NoError(t, err)
	
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Create a handler", resultMap["prompt"])
}
