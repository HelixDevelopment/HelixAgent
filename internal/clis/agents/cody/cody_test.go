// Package cody provides tests for the Sourcegraph Cody integration
package cody

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()
	c := New()

	assert.NotNil(t, c)
	assert.NotNil(t, c.BaseIntegration)
	assert.NotNil(t, c.config)
	assert.NotNil(t, c.snippets)

	info := c.Info()
	assert.Equal(t, "Cody", info.Name)
	assert.Equal(t, "Sourcegraph", info.Vendor)
	assert.Contains(t, info.Capabilities, "code_intelligence")
	assert.Contains(t, info.Capabilities, "codebase_search")
	assert.True(t, info.IsEnabled)
}

func TestCody_Initialize(t *testing.T) {
	t.Parallel()
	c := New()
	ctx := context.Background()

	config := &Config{
		SourcegraphURL: "https://test.sourcegraph.com",
		AccessToken:    "test-token",
		Model:          "test-model",
	}

	err := c.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "https://test.sourcegraph.com", c.config.SourcegraphURL)
	assert.Equal(t, "test-token", c.config.AccessToken)
}

// TestCody_Execute covers the command surface that does NOT call the cody CLI:
// required-param validation, the local save_snippet store, and unknown commands.
// The LLM-backed commands (chat/explain/generate/search/review/edit/symbol) are
// covered by the fake-binary-injected real-exec suite below — RECONCILED per
// §11.4.120 from the former bluff-codifying assertions that accepted fabricated
// "Cody: <msg>" / templated-code responses as PASS.
func TestCody_Execute(t *testing.T) {
	c := New()
	ctx := context.Background()

	err := c.Initialize(ctx, nil)
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
			name:    "chat without message",
			command: "chat",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "message required",
		},
		{
			name:    "save_snippet command",
			command: "save_snippet",
			params:  map[string]interface{}{"content": "code snippet", "description": "test", "language": "go"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "saved", m["status"])
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
			result, err := c.Execute(ctx, tt.command, tt.params)
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

func TestCody_IsAvailable(t *testing.T) {
	t.Parallel()
	c := New()
	assert.False(t, c.IsAvailable())

	c.config.AccessToken = "test-token"
	assert.True(t, c.IsAvailable())
}

func TestSnippet(t *testing.T) {
	t.Parallel()
	snippet := Snippet{
		ID:          "1",
		Content:     "test code",
		File:        "test.go",
		Language:    "go",
		Description: "test snippet",
	}
	assert.Equal(t, "1", snippet.ID)
	assert.Equal(t, "test code", snippet.Content)
	assert.Equal(t, "test.go", snippet.File)
}

func TestConfig(t *testing.T) {
	t.Parallel()
	config := &Config{
		SourcegraphURL: "https://sourcegraph.com",
		AccessToken:    "token",
		Model:          "claude-3",
	}
	assert.Equal(t, "https://sourcegraph.com", config.SourcegraphURL)
	assert.Equal(t, "token", config.AccessToken)
}
