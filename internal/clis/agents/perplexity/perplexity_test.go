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
			name:    "search command",
			command: "search",
			params: map[string]interface{}{
				"query": "Go concurrency",
			},
			wantErr: false,
		},
		{
			name:    "search without query fails",
			command: "search",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "ask command",
			command: "ask",
			params: map[string]interface{}{
				"question": "What is Go?",
			},
			wantErr: false,
		},
		{
			name:    "ask without question fails",
			command: "ask",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "code command",
			command: "code",
			params: map[string]interface{}{
				"prompt": "Write a function",
			},
			wantErr: false,
		},
		{
			name:    "code without prompt fails",
			command: "code",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "research command",
			command: "research",
			params: map[string]interface{}{
				"topic": "AI",
			},
			wantErr: false,
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

	result, err := p.Execute(ctx, "search", map[string]interface{}{
		"query": "Go channels",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Go channels", resultMap["query"])
	assert.True(t, resultMap["citations"].(bool))
}
