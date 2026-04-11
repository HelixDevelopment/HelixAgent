// Package honeycomb provides tests for the Honeycomb agent integration
package honeycomb

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
	assert.Equal(t, "Honeycomb", info.Name)
	assert.Equal(t, "Honeycomb", info.Vendor)
	assert.Contains(t, info.Capabilities, "observability")
	assert.Contains(t, info.Capabilities, "debugging")
	assert.True(t, info.IsEnabled)
}

func TestHoneycomb_Initialize(t *testing.T) {
	t.Parallel()
	h := New()
	ctx := context.Background()

	config := &Config{
		APIKey:  "test-api-key",
		Dataset: "test-dataset",
		Service: "test-service",
	}

	err := h.Initialize(ctx, config)
	require.NoError(t, err)
	assert.Equal(t, "test-api-key", h.config.APIKey)
	assert.Equal(t, "test-dataset", h.config.Dataset)
	assert.Equal(t, "test-service", h.config.Service)
}

func TestHoneycomb_Execute(t *testing.T) {
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
			name:    "query command",
			command: "query",
			params:  map[string]interface{}{"query": "SELECT count() FROM events"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.NotEmpty(t, m["query"])
				assert.Equal(t, "production", m["dataset"])
			},
		},
		{
			name:    "query without query",
			command: "query",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "query required",
		},
		{
			name:    "analyze command",
			command: "analyze",
			params:  map[string]interface{}{"metric": "latency"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "latency", m["metric"])
				assert.NotNil(t, m["insights"])
			},
		},
		{
			name:    "analyze without metric",
			command: "analyze",
			params:  map[string]interface{}{},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "duration", m["metric"])
			},
		},
		{
			name:    "trace command",
			command: "trace",
			params:  map[string]interface{}{"trace_id": "abc123"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "abc123", m["trace_id"])
				assert.NotNil(t, m["spans"])
			},
		},
		{
			name:    "trace without trace_id",
			command: "trace",
			params:  map[string]interface{}{},
			wantErr: true,
			errMsg:  "trace_id required",
		},
		{
			name:    "alert command",
			command: "alert",
			params:  map[string]interface{}{"condition": "latency > 1000"},
			wantErr: false,
			checkFunc: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "latency > 1000", m["condition"])
				assert.Equal(t, "active", m["status"])
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
				assert.Equal(t, "production", m["dataset"])
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

func TestHoneycomb_IsAvailable(t *testing.T) {
	t.Parallel()
	h := New()
	assert.False(t, h.IsAvailable())

	h.config.APIKey = "test-key"
	assert.True(t, h.IsAvailable())
}

func TestConfig(t *testing.T) {
	t.Parallel()
	config := &Config{
		APIKey:  "test-key",
		Dataset: "my-dataset",
		Service: "my-service",
	}
	assert.Equal(t, "test-key", config.APIKey)
	assert.Equal(t, "my-dataset", config.Dataset)
	assert.Equal(t, "my-service", config.Service)
}
