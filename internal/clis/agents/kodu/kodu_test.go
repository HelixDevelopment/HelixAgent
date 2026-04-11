// Package kodu provides tests for the Kodu agent integration
package kodu

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()
	k := New()

	assert.NotNil(t, k)
	assert.NotNil(t, k.BaseIntegration)
	assert.NotNil(t, k.config)
	assert.NotNil(t, k.context)

	info := k.Info()
	assert.Equal(t, "Kodu", info.Name)
	assert.Equal(t, "Kodu", info.Vendor)
	assert.Contains(t, info.Capabilities, "semantic_search")
	assert.Contains(t, info.Capabilities, "code_understanding")
	assert.True(t, info.IsEnabled)
}

func TestKodu_Initialize(t *testing.T) {
	t.Parallel()
	k := New()
	ctx := context.Background()

	config := &Config{
		Model:         "claude-opus",
		ContextWindow: 200000,
		SemanticCache: false,
	}

	err := k.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "claude-opus", k.config.Model)
	assert.Equal(t, 200000, k.config.ContextWindow)
	assert.False(t, k.config.SemanticCache)
}

func TestKodu_Execute(t *testing.T) {
	t.Parallel()
	k := New()
	ctx := context.Background()

	err := k.Initialize(ctx, nil)
	require.NoError(t, err)

	tests := []struct {
		name      string
		command   string
		params    map[string]interface{}
		wantErr   bool
		errMsg    string
		checkFunc func(t *testing.T, result interface{})
	}{
		{
			name:    "ask command",
			command: "ask",
			params:  map[string]interface{}{"question": "What does this do?"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "What does this do?", m["question"])
				assert.NotNil(t, m["relevant_symbols"])
			},
		},
		{
			name:    "ask without question",
			command: "ask",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "question required",
		},
		{
			name:    "search command",
			command: "search",
			params:  map[string]interface{}{"query": "function"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "function", m["query"])
				assert.NotNil(t, m["results"])
			},
		},
		{
			name:    "explain command",
			command: "explain",
			params:  map[string]interface{}{"file": "main.go"},
			wantErr: true, // File not in context
			errMsg:  "file not in context",
		},
		{
			name:    "refactor command",
			command: "refactor",
			params:  map[string]interface{}{"file": "main.go", "instruction": "extract function"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "main.go", m["file"])
			},
		},
		{
			name:    "index command",
			command: "index",
			params:  map[string]interface{}{"directory": "."},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "indexed", m["status"])
			},
		},
		{
			name:    "navigate command",
			command: "navigate",
			params:  map[string]interface{}{"symbol": "MyFunc"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "MyFunc", m["symbol"])
			},
		},
		{
			name:    "relations command",
			command: "relations",
			params:  map[string]interface{}{"symbol": "MyFunc"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "MyFunc", m["symbol"])
				assert.NotNil(t, m["relations"])
			},
		},
		{
			name:    "unknown command",
			command: "unknown",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "unknown command: unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := k.Execute(ctx, tt.command, tt.params)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			if tt.checkFunc != nil {
				tt.checkFunc(t, result)
			}
		})
	}
}

func TestKodu_IsAvailable(t *testing.T) {
	t.Parallel()
	k := New()
	// Availability depends on the presence of 'kodu' command in PATH
	// Result may vary by system
	_ = k.IsAvailable()
}

func TestSymbol(t *testing.T) {
	t.Parallel()
	symbol := Symbol{
		Name:    "TestFunc",
		Type:    "func",
		File:    "main.go",
		Line:    10,
		Package: "main",
	}
	assert.Equal(t, "TestFunc", symbol.Name)
	assert.Equal(t, "func", symbol.Type)
	assert.Equal(t, 10, symbol.Line)
}

func TestRelation(t *testing.T) {
	t.Parallel()
	relation := Relation{
		From: "FuncA",
		To:   "FuncB",
		Type: "calls",
	}
	assert.Equal(t, "FuncA", relation.From)
	assert.Equal(t, "FuncB", relation.To)
	assert.Equal(t, "calls", relation.Type)
}

func TestConfig(t *testing.T) {
	t.Parallel()
	config := &Config{
		Model:         "claude-haiku",
		ContextWindow: 100000,
		SemanticCache: true,
	}
	assert.Equal(t, "claude-haiku", config.Model)
	assert.True(t, config.SemanticCache)
}
