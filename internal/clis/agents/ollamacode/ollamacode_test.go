// Package ollamacode provides tests for Ollama Code agent integration
package ollamacode

import (
	"context"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOllamaCode(t *testing.T) {
	t.Parallel()
	o := New()
	require.NotNil(t, o)

	info := o.Info()
	assert.Equal(t, agents.TypeOllamaCode, info.Type)
	assert.Equal(t, "Ollama Code", info.Name)
	assert.Equal(t, "Ollama", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestOllamaCodeInitialize(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Endpoint: "http://custom:11434",
		Model:    "llama2",
	}

	err := o.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "http://custom:11434", o.config.Endpoint)
	assert.Equal(t, "llama2", o.config.Model)
}

func TestOllamaCodeInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()

	err := o.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:11434", o.config.Endpoint) // Default value
	assert.Equal(t, "codellama", o.config.Model)                 // Default value
}

func TestOllamaCodeStartStop(t *testing.T) {
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

func TestOllamaCodeExecute(t *testing.T) {
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

func TestOllamaCodeCapabilities(t *testing.T) {
	t.Parallel()
	o := New()
	info := o.Info()

	expectedCaps := []string{"local_llm", "privacy", "code_generation"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

func TestOllamaCodeIsAvailable(t *testing.T) {
	t.Parallel()
	o := New()
	assert.True(t, o.IsAvailable())

	// Test with empty endpoint
	o.config.Endpoint = ""
	assert.False(t, o.IsAvailable())
}

func TestOllamaCodeGenerateResult(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()

	err := o.Initialize(ctx, nil)
	require.NoError(t, err)

	result, err := o.Execute(ctx, "generate", map[string]interface{}{
		"prompt": "Build a struct",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Build a struct", resultMap["prompt"])
	assert.Equal(t, "codellama", resultMap["model"])
}
