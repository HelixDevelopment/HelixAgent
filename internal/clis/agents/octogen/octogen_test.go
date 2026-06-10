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
			// Reconciled (§11.4.120): generate has no real backend; it now
			// returns an HONEST error instead of fabricating code.
			name:    "generate command honest-errors (no backend)",
			command: "generate",
			params:  map[string]interface{}{"prompt": "Create a service"},
			wantErr: true,
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

// TestOctogenGenerateResult reconciled (§11.4.120 / §11.4.115): generate no
// longer returns the "// Octogen multi-model" literal; with no real backend
// wired it MUST return the honest ErrNoBackend. Standing GREEN guard for D-17.
func TestOctogenGenerateResult(t *testing.T) {
	t.Parallel()
	o := New()
	ctx := context.Background()

	err := o.Initialize(ctx, nil)
	require.NoError(t, err)

	res, err := o.Execute(ctx, "generate", map[string]interface{}{
		"prompt": "Build an API",
	})
	require.ErrorIs(t, err, ErrNoBackend,
		"generate must return the honest no-backend error, never a fabricated result")
	assert.Nil(t, res)
}
