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
			// Reconciled (§11.4.120): generate has no real backend; it now
			// returns an HONEST error instead of fabricating code.
			name:    "generate command honest-errors (no backend)",
			command: "generate",
			params:  map[string]interface{}{"prompt": "Create a button"},
			wantErr: true,
		},
		{
			name:    "generate without prompt fails",
			command: "generate",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			// Reconciled (§11.4.120): build has no real backend; honest error.
			name:    "build command honest-errors (no backend)",
			command: "build",
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

// TestMobileAgentGenerateResult reconciled (§11.4.120 / §11.4.115): generate and
// build no longer fabricate code/status; with no real backend wired they MUST
// return the honest ErrNoBackend. This is the standing GREEN guard for D-17 on
// this package — reverting to a fabricated map makes it FAIL.
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

	res, err := m.Execute(ctx, "generate", map[string]interface{}{
		"prompt": "Create a list view",
	})
	require.ErrorIs(t, err, ErrNoBackend,
		"generate must return the honest no-backend error, never a fabricated result")
	assert.Nil(t, res)

	bres, berr := m.Execute(ctx, "build", map[string]interface{}{})
	require.ErrorIs(t, berr, ErrNoBackend,
		"build must return the honest no-backend error, never a fabricated 'built' status")
	assert.Nil(t, bres)
}
