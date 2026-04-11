// Package kimi provides tests for the Kimi agent integration
package kimi

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

	info := k.Info()
	assert.Equal(t, "Kimi", info.Name)
	assert.Equal(t, "Moonshot AI", info.Vendor)
	assert.Contains(t, info.Capabilities, "long_context")
	assert.Contains(t, info.Capabilities, "code_generation")
	assert.True(t, info.IsEnabled)
}

func TestKimi_Initialize(t *testing.T) {
	t.Parallel()
	k := New()
	ctx := context.Background()

	config := &Config{
		APIKey:        "test-api-key",
		Model:         "kimi-latest",
		ContextWindow: 1000000,
		MaxTokens:     16384,
	}

	err := k.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "test-api-key", k.config.APIKey)
	assert.Equal(t, "kimi-latest", k.config.Model)
	assert.Equal(t, 1000000, k.config.ContextWindow)
}

func TestKimi_Execute(t *testing.T) {
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
			name:    "chat command",
			command: "chat",
			params:  map[string]interface{}{"message": "Hello"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "Hello", m["message"])
				assert.NotEmpty(t, m["response"])
				assert.Equal(t, "kimi-k2", m["model"])
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
			params:  map[string]interface{}{"prompt": "Create function"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotEmpty(t, m["code"])
			},
		},
		{
			name:    "generate without prompt",
			command: "generate",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "prompt required",
		},
		{
			name:    "analyze command",
			command: "analyze",
			params:  map[string]interface{}{"document": "This is a document"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotEmpty(t, m["analysis"])
				assert.Equal(t, "kimi-k2", m["model"])
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
				assert.Equal(t, "kimi-k2", m["model"])
				assert.Equal(t, 2000000, m["context_window"])
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

func TestKimi_IsAvailable(t *testing.T) {
	t.Parallel()
	k := New()
	assert.False(t, k.IsAvailable())

	k.config.APIKey = "test-key"
	assert.True(t, k.IsAvailable())
}

func TestConfig(t *testing.T) {
	t.Parallel()
	config := &Config{
		APIKey:        "test-key",
		Model:         "kimi-k2",
		ContextWindow: 2000000,
		MaxTokens:     8192,
	}
	assert.Equal(t, "test-key", config.APIKey)
	assert.Equal(t, "kimi-k2", config.Model)
	assert.Equal(t, 2000000, config.ContextWindow)
	assert.Equal(t, 8192, config.MaxTokens)
}
