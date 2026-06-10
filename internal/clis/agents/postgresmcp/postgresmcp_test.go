// Package postgresmcp provides tests for Postgres MCP agent integration
package postgresmcp

import (
	"context"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresMCP(t *testing.T) {
	t.Parallel()
	p := New()
	require.NotNil(t, p)

	info := p.Info()
	assert.Equal(t, agents.TypePostgresMCP, info.Type)
	assert.Equal(t, "Postgres MCP", info.Name)
	assert.Equal(t, "PostgresMCP", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestPostgresMCPInitialize(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		ConnectionString: "postgres://user:pass@localhost/db",
	}

	err := p.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "postgres://user:pass@localhost/db", p.config.ConnectionString)
}

func TestPostgresMCPInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	err := p.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, p.config.ConnectionString)
}

func TestPostgresMCPStartStop(t *testing.T) {
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

func TestPostgresMCPExecute(t *testing.T) {
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
			// Reconciled (§11.4.120): no real DB/MCP backend; honest error.
			name:    "query command honest-errors (no backend)",
			command: "query",
			params:  map[string]interface{}{"sql": "SELECT * FROM users"},
			wantErr: true,
		},
		{
			name:    "query without sql fails",
			command: "query",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			// Reconciled (§11.4.120): no real DB/MCP backend; honest error.
			name:    "schema command honest-errors (no backend)",
			command: "schema",
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

func TestPostgresMCPCapabilities(t *testing.T) {
	t.Parallel()
	p := New()
	info := p.Info()

	expectedCaps := []string{"database", "postgresql", "mcp_protocol"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

func TestPostgresMCPIsAvailable(t *testing.T) {
	t.Parallel()
	p := New()
	assert.False(t, p.IsAvailable()) // No connection string initially

	p.config.ConnectionString = "postgres://user:pass@localhost/db"
	assert.True(t, p.IsAvailable())
}

// TestPostgresMCPQueryResult reconciled (§11.4.120 / §11.4.115): query no longer
// returns a fabricated "Query result"; with no real DB/MCP backend wired it MUST
// return the honest ErrNoBackend. Standing GREEN guard for D-17.
func TestPostgresMCPQueryResult(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	err := p.Initialize(ctx, nil)
	require.NoError(t, err)

	res, err := p.Execute(ctx, "query", map[string]interface{}{
		"sql": "SELECT id FROM posts",
	})
	require.ErrorIs(t, err, ErrNoBackend,
		"query must return the honest no-backend error, never a fabricated result")
	assert.Nil(t, res)
}

// TestPostgresMCPSchemaResult reconciled (§11.4.120 / §11.4.115): schema no
// longer returns a hardcoded ["users","posts"] list; with no real DB/MCP backend
// wired it MUST return the honest ErrNoBackend. Standing GREEN guard for D-17.
func TestPostgresMCPSchemaResult(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	err := p.Initialize(ctx, nil)
	require.NoError(t, err)

	res, err := p.Execute(ctx, "schema", map[string]interface{}{})
	require.ErrorIs(t, err, ErrNoBackend,
		"schema must return the honest no-backend error, never a hardcoded table list")
	assert.Nil(t, res)
}
