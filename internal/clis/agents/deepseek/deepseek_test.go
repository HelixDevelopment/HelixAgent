// Package deepseek provides tests for the DeepSeek CLI agent integration
package deepseek

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()
	d := New()

	assert.NotNil(t, d)
	assert.NotNil(t, d.BaseIntegration)
	assert.NotNil(t, d.config)

	info := d.Info()
	assert.Equal(t, "DeepSeek", info.Name)
	assert.Equal(t, "DeepSeek", info.Vendor)
	assert.Contains(t, info.Capabilities, "code_generation")
	assert.Contains(t, info.Capabilities, "reasoning")
	assert.True(t, info.IsEnabled)
}

func TestDeepSeek_Initialize(t *testing.T) {
	t.Parallel()
	d := New()
	ctx := context.Background()

	config := &Config{
		APIKey:      "test-api-key",
		Model:       "deepseek-v3",
		MaxTokens:   4096,
		Temperature: 0.5,
	}

	err := d.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "test-api-key", d.config.APIKey)
	assert.Equal(t, "deepseek-v3", d.config.Model)
	assert.Equal(t, 4096, d.config.MaxTokens)
}

func TestDeepSeek_Execute(t *testing.T) {
	t.Parallel()
	d := New()
	ctx := context.Background()

	err := d.Initialize(ctx, nil)
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
			name:    "generate command",
			command: "generate",
			params:  map[string]interface{}{"prompt": "Create function", "language": "go"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotEmpty(t, m["code"])
				assert.Equal(t, "go", m["language"])
			},
		},
		{
			name:    "complete command",
			command: "complete",
			params:  map[string]interface{}{"prefix": "func ", "suffix": "}"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotEmpty(t, m["completion"])
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
			name:    "status command",
			command: "status",
			params:  map[string]interface{}{},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotNil(t, m["available"])
				assert.NotNil(t, m["model"])
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
			result, err := d.Execute(ctx, tt.command, tt.params)
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

func TestDeepSeek_IsAvailable(t *testing.T) {
	t.Parallel()
	d := New()
	assert.False(t, d.IsAvailable())

	d.config.APIKey = "test-key"
	assert.True(t, d.IsAvailable())
}

func TestConfig(t *testing.T) {
	t.Parallel()
	config := &Config{
		APIKey:      "test-key",
		Model:       "deepseek-coder-v3",
		MaxTokens:   8192,
		Temperature: 0.7,
	}
	assert.Equal(t, "test-key", config.APIKey)
	assert.Equal(t, "deepseek-coder-v3", config.Model)
	assert.Equal(t, 8192, config.MaxTokens)
	assert.Equal(t, 0.7, config.Temperature)
}
