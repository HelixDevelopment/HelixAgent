// Package hunyuan provides tests for the Hunyuan agent integration
package hunyuan

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()
	h := New()

	assert.NotNil(t, h)
	assert.NotNil(t, h.BaseIntegration)
	assert.NotNil(t, h.config)

	info := h.Info()
	assert.Equal(t, "Hunyuan", info.Name)
	assert.Equal(t, "Tencent", info.Vendor)
	assert.Contains(t, info.Capabilities, "multimodal")
	assert.Contains(t, info.Capabilities, "code_generation")
	assert.True(t, info.IsEnabled)
}

func TestHunyuan_Initialize(t *testing.T) {
	t.Parallel()
	h := New()
	ctx := context.Background()

	config := &Config{
		APIKey:    "test-api-key",
		SecretID:  "test-secret-id",
		SecretKey: "test-secret-key",
		Region:    "ap-beijing",
		Model:     "hunyuan-lite",
	}

	err := h.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "test-api-key", h.config.APIKey)
	assert.Equal(t, "test-secret-id", h.config.SecretID)
	assert.Equal(t, "ap-beijing", h.config.Region)
}

func TestHunyuan_Execute(t *testing.T) {
	t.Parallel()
	h := New()
	ctx := context.Background()

	err := h.Initialize(ctx, nil)
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
				assert.Equal(t, "hunyuan-pro", m["model"])
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
			name:    "status command",
			command: "status",
			params:  map[string]interface{}{},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotNil(t, m["available"])
				assert.Equal(t, "hunyuan-pro", m["model"])
				assert.Equal(t, "ap-guangzhou", m["region"])
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
			result, err := h.Execute(ctx, tt.command, tt.params)
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

func TestHunyuan_IsAvailable(t *testing.T) {
	t.Parallel()
	h := New()
	assert.False(t, h.IsAvailable())

	h.config.APIKey = "test-key"
	assert.False(t, h.IsAvailable()) // Still false without SecretID

	h.config.SecretID = "test-secret-id"
	assert.True(t, h.IsAvailable())
}

func TestConfig(t *testing.T) {
	t.Parallel()
	config := &Config{
		APIKey:    "key",
		SecretID:  "secret-id",
		SecretKey: "secret-key",
		Region:    "ap-shanghai",
		Model:     "hunyuan-pro",
	}
	assert.Equal(t, "key", config.APIKey)
	assert.Equal(t, "secret-id", config.SecretID)
	assert.Equal(t, "ap-shanghai", config.Region)
	assert.Equal(t, "hunyuan-pro", config.Model)
}
