// Package octogen provides tests for Octogen agent integration
package octogen

import (
	"context"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOctogen(t *testing.T) {
	t.Parallel()
	o := New()
	require.NotNil(t, o)

	info := o.Info()
	assert.Equal(t, agents.TypeOctogen, info.Type)
	assert.Equal(t, "Octogen", info.Name)
	assert.Equal(t, "Octogen", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestOctogenInitialize(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Models: []string{"model1", "model2"},
	}

	err := o.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, []string{"model1", "model2"}, o.config.Models)
}

func TestOctogenInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()

	err := o.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"gpt-4", "claude-3"}, o.config.Models) // Default value
}

func TestOctogenStartStop(t *testing.T) {
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

func TestOctogenExecute(t *testing.T) {
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
				"prompt": "Create a service",
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

func TestOctogenCapabilities(t *testing.T) {
	t.Parallel()
	o := New()
	info := o.Info()

	expectedCaps := []string{"multi_model", "ensemble", "code_generation"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

func TestOctogenIsAvailable(t *testing.T) {
	t.Parallel()
	o := New()
	assert.True(t, o.IsAvailable())
}

func TestOctogenGenerateResult(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()

	err := o.Initialize(ctx, nil)
	require.NoError(t, err)

	result, err := o.Execute(ctx, "generate", map[string]interface{}{
		"prompt": "Build an API",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Build an API", resultMap["prompt"])
	assert.NotNil(t, resultMap["models"])
}
