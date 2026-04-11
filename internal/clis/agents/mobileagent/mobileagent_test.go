// Package mobileagent provides tests for Mobile Agent integration
package mobileagent

import (
	"context"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMobileAgent(t *testing.T) {
	t.Parallel()
	m := New()
	require.NotNil(t, m)

	info := m.Info()
	assert.Equal(t, agents.TypeMobileAgent, info.Type)
	assert.Equal(t, "Mobile Agent", info.Name)
	assert.Equal(t, "MobileAgent", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestMobileAgentInitialize(t *testing.T) {
	t.Parallel()
	m := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Platform: "ios",
	}

	err := m.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "ios", m.config.Platform)
}

func TestMobileAgentInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	m := New()
	ctx := context.Background()

	err := m.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "flutter", m.config.Platform) // Default value
}

func TestMobileAgentStartStop(t *testing.T) {
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

func TestMobileAgentExecute(t *testing.T) {
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
			name:    "generate command",
			command: "generate",
			params: map[string]interface{}{
				"prompt": "Create a button",
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
			name:    "build command",
			command: "build",
			params:  map[string]interface{}{},
			wantErr: false,
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

func TestMobileAgentCapabilities(t *testing.T) {
	t.Parallel()
	m := New()
	info := m.Info()

	expectedCaps := []string{"mobile_dev", "ios", "android", "flutter"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

func TestMobileAgentIsAvailable(t *testing.T) {
	t.Parallel()
	m := New()
	assert.True(t, m.IsAvailable())
}

func TestMobileAgentGenerateResult(t *testing.T) {
	t.Parallel()
	m := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		Platform: "android",
	}

	err := m.Initialize(ctx, config)
	require.NoError(t, err)

	result, err := m.Execute(ctx, "generate", map[string]interface{}{
		"prompt": "Create a list view",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "android", resultMap["platform"])
	assert.Equal(t, "Create a list view", resultMap["prompt"])
}
