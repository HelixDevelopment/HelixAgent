// Package jetbrainsai provides tests for the JetBrains AI Assistant integration
package jetbrainsai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()
	j := New()

	assert.NotNil(t, j)
	assert.NotNil(t, j.BaseIntegration)
	assert.NotNil(t, j.config)

	info := j.Info()
	assert.Equal(t, "JetBrains AI", info.Name)
	assert.Equal(t, "JetBrains", info.Vendor)
	assert.Contains(t, info.Capabilities, "ide_integration")
	assert.Contains(t, info.Capabilities, "inline_completion")
	assert.True(t, info.IsEnabled)
}

func TestJetBrainsAI_Initialize(t *testing.T) {
	t.Parallel()
	j := New()
	ctx := context.Background()

	config := &Config{
		IDEType:      "goland",
		Model:        "gpt-4",
		EnableInline: false,
		EnableChat:   false,
	}

	err := j.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "goland", j.config.IDEType)
	assert.Equal(t, "gpt-4", j.config.Model)
	assert.False(t, j.config.EnableInline)
}

func TestJetBrainsAI_Execute(t *testing.T) {
	t.Parallel()
	j := New()
	ctx := context.Background()

	err := j.Initialize(ctx, nil)
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
			name:    "complete command",
			command: "complete",
			params:  map[string]interface{}{"file": "test.go", "line": 10, "column": 5},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "test.go", m["file"])
				assert.Equal(t, 10, m["line"])
				assert.Equal(t, "intellij", m["ide"])
			},
		},
		{
			name:    "chat command",
			command: "chat",
			params:  map[string]interface{}{"message": "Hello"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "Hello", m["message"])
				assert.Equal(t, "intellij", m["ide"])
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
			name:    "generate command",
			command: "generate",
			params:  map[string]interface{}{"prompt": "Create class", "language": "java"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotEmpty(t, m["code"])
				assert.Equal(t, "java", m["language"])
			},
		},
		{
			name:    "explain command",
			command: "explain",
			params:  map[string]interface{}{"code": "public class Main {}"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotEmpty(t, m["explanation"])
			},
		},
		{
			name:    "test command",
			command: "test",
			params:  map[string]interface{}{"code": "func Add(a, b int) int { return a + b }"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotEmpty(t, m["tests"])
			},
		},
		{
			name:    "docs command",
			command: "docs",
			params:  map[string]interface{}{"code": "func Process() {}"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotEmpty(t, m["docs"])
			},
		},
		{
			name:    "status command",
			command: "status",
			params:  map[string]interface{}{},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotNil(t, m["available"])
				assert.Equal(t, "intellij", m["ide"])
				assert.True(t, m["inline_enabled"].(bool))
				assert.True(t, m["chat_enabled"].(bool))
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
			result, err := j.Execute(ctx, tt.command, tt.params)
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

func TestJetBrainsAI_IsAvailable(t *testing.T) {
	t.Parallel()
	j := New()
	assert.True(t, j.IsAvailable()) // IDEType is set by default

	j.config.IDEType = ""
	assert.False(t, j.IsAvailable())
}

func TestConfig(t *testing.T) {
	t.Parallel()
	config := &Config{
		IDEType:      "pycharm",
		Model:        "claude-3",
		EnableInline: true,
		EnableChat:   false,
	}
	assert.Equal(t, "pycharm", config.IDEType)
	assert.Equal(t, "claude-3", config.Model)
	assert.True(t, config.EnableInline)
	assert.False(t, config.EnableChat)
}
