// Package perplexity provides tests for Perplexity agent integration
package perplexity

import (
	"context"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPerplexity(t *testing.T) {
	t.Parallel()
	p := New()
	require.NotNil(t, p)

	info := p.Info()
	assert.Equal(t, agents.TypePerplexity, info.Type)
	assert.Equal(t, "Perplexity", info.Name)
	assert.Equal(t, "Perplexity", info.Vendor)
	assert.True(t, info.IsEnabled)
}

func TestPerplexityInitialize(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		APIKey:     "test-key",
		Model:      "sonar-research",
		SearchMode: false,
		Citations:  false,
	}

	err := p.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "test-key", p.config.APIKey)
	assert.Equal(t, "sonar-research", p.config.Model)
	assert.False(t, p.config.SearchMode)
	assert.False(t, p.config.Citations)
}

func TestPerplexityInitializeWithNilConfig(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	err := p.Initialize(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, "sonar-pro", p.config.Model) // Default value
	assert.True(t, p.config.SearchMode)          // Default value
	assert.True(t, p.config.Citations)           // Default value
}

func TestPerplexityStartStop(t *testing.T) {
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

func TestPerplexityExecute(t *testing.T) {
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
			// Reconciled (§11.4.120): search has no real backend; honest error.
			name:    "search command honest-errors (no backend)",
			command: "search",
			params:  map[string]interface{}{"query": "Go concurrency"},
			wantErr: true,
		},
		{
			name:    "search without query fails",
			command: "search",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			// Reconciled (§11.4.120): ask has no real backend; honest error.
			name:    "ask command honest-errors (no backend)",
			command: "ask",
			params:  map[string]interface{}{"question": "What is Go?"},
			wantErr: true,
		},
		{
			name:    "ask without question fails",
			command: "ask",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			// Reconciled (§11.4.120): code has no real backend; honest error.
			name:    "code command honest-errors (no backend)",
			command: "code",
			params:  map[string]interface{}{"prompt": "Write a function"},
			wantErr: true,
		},
		{
			name:    "code without prompt fails",
			command: "code",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			// Reconciled (§11.4.120): research has no real backend; honest error.
			name:    "research command honest-errors (no backend)",
			command: "research",
			params:  map[string]interface{}{"topic": "AI"},
			wantErr: true,
		},
		{
			name:    "research without topic fails",
			command: "research",
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

func TestPerplexityCapabilities(t *testing.T) {
	t.Parallel()
	p := New()
	info := p.Info()

	expectedCaps := []string{"search", "code_generation", "research", "citations", "real_time_info"}
	for _, cap := range expectedCaps {
		assert.Contains(t, info.Capabilities, cap)
	}
}

func TestPerplexityIsAvailable(t *testing.T) {
	t.Parallel()
	p := New()
	assert.False(t, p.IsAvailable()) // No API key set initially

	p.config.APIKey = "test-key"
	assert.True(t, p.IsAvailable())
}

// TestPerplexitySearchResult reconciled (§11.4.120 / §11.4.115): search no
// longer fabricates an answer + fake sources; with no real backend wired it MUST
// return the honest ErrNoBackend. This is the standing GREEN guard for D-17 —
// reverting to a fabricated answer/sources map makes it FAIL.
func TestPerplexitySearchResult(t *testing.T) {
	t.Parallel()
	p := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: t.TempDir(),
		},
		APIKey:    "test-key",
		Citations: true,
	}

	err := p.Initialize(ctx, config)
	require.NoError(t, err)

	res, err := p.Execute(ctx, "search", map[string]interface{}{
		"query": "Go channels",
	})
	require.ErrorIs(t, err, ErrNoBackend,
		"search must return the honest no-backend error, never fabricated answers/sources")
	assert.Nil(t, res)
}
