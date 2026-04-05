// Package continueagent provides tests for the Continue CLI agent integration
package continueagent

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
	assert.Equal(t, "Continue", info.Name)
	assert.Equal(t, "Continue", info.Vendor)
	assert.Contains(t, info.Capabilities, "ide_integration")
	assert.Contains(t, info.Capabilities, "autocomplete")
	assert.True(t, info.IsEnabled)
}

func TestContinue_Initialize(t *testing.T) {
	t.Parallel()
	c := New()
	ctx := context.Background()

	config := &Config{
		ServerURL:      "http://localhost:4000",
		AllowAnonymous: true,
	}

	err := c.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:4000", c.config.ServerURL)
	assert.True(t, c.config.AllowAnonymous)
}

func TestContinue_Execute(t *testing.T) {
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
				assert.Equal(t, "sent", m["status"])
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
			name:    "autocomplete command",
			command: "autocomplete",
			params:  map[string]interface{}{"file": "test.go", "line": 10, "column": 5},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "test.go", m["file"])
				assert.Equal(t, 10, m["line"])
			},
		},
		{
			name:    "edit command",
			command: "edit",
			params:  map[string]interface{}{"prompt": "Refactor this code"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "Refactor this code", m["prompt"])
			},
		},
		{
			name:    "edit without prompt",
			command: "edit",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "prompt required",
		},
		{
			name:    "action command",
			command: "action",
			params:  map[string]interface{}{"action": "explain"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "explain", m["action"])
			},
		},
		{
			name:    "action without action",
			command: "action",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "action required",
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

func TestContinue_IsAvailable(t *testing.T) {
	t.Parallel()
	c := New()
	// Availability depends on the presence of 'continue' command in PATH
	// Result may vary by system
	_ = c.IsAvailable()
}

func TestConfig(t *testing.T) {
	t.Parallel()
	config := &Config{
		ServerURL:      "http://localhost:3000",
		AllowAnonymous: false,
	}
	assert.Equal(t, "http://localhost:3000", config.ServerURL)
	assert.False(t, config.AllowAnonymous)
}
