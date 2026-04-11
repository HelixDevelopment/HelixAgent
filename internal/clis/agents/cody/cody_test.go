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

func TestCody_Execute(t *testing.T) {
	t.Parallel()
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
			name:    "chat command",
			command: "chat",
			params:  map[string]interface{}{"message": "Hello", "context": "test"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "Hello", m["message"])
				assert.Equal(t, "test", m["context"])
			},
		},
		{
			name:    "chat without message",
			command: "chat",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "message required",
		},
		{
			name:    "explain command",
			command: "explain",
			params:  map[string]interface{}{"code": "func main() {}"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotEmpty(t, m["explanation"])
			},
		},
		{
			name:    "generate command",
			command: "generate",
			params:  map[string]interface{}{"prompt": "Create function", "language": "go"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "go", m["language"])
				assert.NotEmpty(t, m["code"])
			},
		},
		{
			name:    "search command",
			command: "search",
			params:  map[string]interface{}{"query": "function"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotNil(t, m["results"])
			},
		},
		{
			name:    "review command",
			command: "review",
			params:  map[string]interface{}{"code": "test code"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotNil(t, m["issues"])
			},
		},
		{
			name:    "edit command",
			command: "edit",
			params:  map[string]interface{}{"file": "test.go", "instruction": "fix bug"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "test.go", m["file"])
			},
		},
		{
			name:    "symbol command",
			command: "symbol",
			params:  map[string]interface{}{"name": "MyFunc"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "MyFunc", m["symbol"])
			},
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
			t.Parallel()
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
