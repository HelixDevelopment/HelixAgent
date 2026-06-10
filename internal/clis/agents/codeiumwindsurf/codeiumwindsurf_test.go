// Package codeiumwindsurf provides tests for the Codeium Windsurf integration
package codeiumwindsurf

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

	info := c.Info()
	assert.Equal(t, "Codeium Windsurf", info.Name)
	assert.Equal(t, "Codeium", info.Vendor)
	assert.Contains(t, info.Capabilities, "ai_completion")
	assert.Contains(t, info.Capabilities, "chat")
	assert.Contains(t, info.Capabilities, "cascade")
	assert.True(t, info.IsEnabled)
}

func TestCodeiumWindsurf_Initialize(t *testing.T) {
	t.Parallel()
	c := New()
	ctx := context.Background()

	config := &Config{
		APIKey: "test-api-key",
		Model:  "test-model",
	}

	err := c.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "test-api-key", c.config.APIKey)
	assert.Equal(t, "test-model", c.config.Model)
}

func TestCodeiumWindsurf_Execute(t *testing.T) {
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
		// RECONCILED per §11.4.120: complete/chat/cascade USED to assert a
		// fabricated "// Codeium completion" / "Codeium: <msg>" / templated
		// cascade PASS. Windsurf is an AI-native IDE with NO headless agent CLI,
		// so the de-bluffed handlers now return an HONEST error — these cases
		// assert that honest error rather than codifying the former bluff.
		{
			name:    "complete command (no headless CLI -> honest error)",
			command: "complete",
			params:  map[string]interface{}{"prefix": "func main"},
			wantErr: true,
			errMsg:  "no headless CLI",
		},
		{
			name:    "chat command (no headless CLI -> honest error)",
			command: "chat",
			params:  map[string]interface{}{"message": "Hello"},
			wantErr: true,
			errMsg:  "no headless CLI",
		},
		{
			name:    "chat without message",
			command: "chat",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "message required",
		},
		{
			name:    "cascade command (no headless CLI -> honest error)",
			command: "cascade",
			params:  map[string]interface{}{"prompt": "Create a web app"},
			wantErr: true,
			errMsg:  "no headless CLI",
		},
		{
			name:    "cascade without prompt",
			command: "cascade",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "prompt required",
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
				assert.Equal(t, "codeium-cascade", m["model"])
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

func TestCodeiumWindsurf_IsAvailable(t *testing.T) {
	t.Parallel()
	c := New()
	assert.False(t, c.IsAvailable())

	c.config.APIKey = "test-key"
	assert.True(t, c.IsAvailable())
}

func TestConfig(t *testing.T) {
	t.Parallel()
	config := &Config{
		APIKey: "test-key",
		Model:  "test-model",
	}
	assert.Equal(t, "test-key", config.APIKey)
	assert.Equal(t, "test-model", config.Model)
}
