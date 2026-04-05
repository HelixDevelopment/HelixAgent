// Package cursor provides tests for the Cursor IDE agent integration
package cursor

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
	assert.NotNil(t, c.sessions)

	info := c.Info()
	assert.Equal(t, "Cursor", info.Name)
	assert.Equal(t, "Cursor", info.Vendor)
	assert.Contains(t, info.Capabilities, "ai_chat")
	assert.Contains(t, info.Capabilities, "code_generation")
	assert.True(t, info.IsEnabled)
}

func TestCursor_Initialize(t *testing.T) {
	t.Parallel()
	c := New()
	ctx := context.Background()

	config := &Config{
		EditorPath:    "/usr/bin/cursor",
		AIProvider:    "anthropic",
		Model:         "claude-3",
		ContextWindow: 100000,
	}

	err := c.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/cursor", c.config.EditorPath)
	assert.Equal(t, "anthropic", c.config.AIProvider)
}

func TestCursor_Execute(t *testing.T) {
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
			params:  map[string]interface{}{"message": "Hello"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "Hello", m["message"])
				assert.NotEmpty(t, m["response"])
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
			name:    "edit command",
			command: "edit",
			params:  map[string]interface{}{"file": "test.go", "instruction": "refactor"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "test.go", m["file"])
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
				assert.NotEmpty(t, m["code"])
			},
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
			name:    "terminal command",
			command: "terminal",
			params:  map[string]interface{}{"command": "ls -la"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "ls -la", m["command"])
			},
		},
		{
			name:    "composer command",
			command: "composer",
			params:  map[string]interface{}{"prompt": "Create app", "files": []interface{}{"main.go"}},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "composed", m["status"])
			},
		},
		{
			name:    "create_session command",
			command: "create_session",
			params:  map[string]interface{}{"name": "Test Session", "context": "test"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "created", m["status"])
			},
		},
		{
			name:    "list_sessions command",
			command: "list_sessions",
			params:  map[string]interface{}{},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotNil(t, m["sessions"])
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

func TestCursor_IsAvailable(t *testing.T) {
	t.Parallel()
	c := New()
	// By default, AIProvider is set so it should be available
	assert.True(t, c.IsAvailable())

	// Clear both fields
	c.config.EditorPath = ""
	c.config.AIProvider = ""
	assert.False(t, c.IsAvailable())
}

func TestChatSession(t *testing.T) {
	t.Parallel()
	session := ChatSession{
		ID:     "1",
		Name:   "Test Session",
		Status: "active",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi"},
		},
	}
	assert.Equal(t, "1", session.ID)
	assert.Equal(t, "Test Session", session.Name)
	assert.Len(t, session.Messages, 2)
}

func TestMessage(t *testing.T) {
	t.Parallel()
	msg := Message{
		Role:    "user",
		Content: "Hello",
	}
	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "Hello", msg.Content)
}

func TestConfig(t *testing.T) {
	t.Parallel()
	config := &Config{
		EditorPath:    "/usr/bin/cursor",
		AIProvider:    "anthropic",
		Model:         "claude-sonnet-4",
		ContextWindow: 200000,
	}
	assert.Equal(t, "/usr/bin/cursor", config.EditorPath)
	assert.Equal(t, "claude-sonnet-4", config.Model)
}
