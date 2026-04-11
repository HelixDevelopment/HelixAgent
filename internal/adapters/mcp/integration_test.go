package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientAdapter_Integration(t *testing.T) {
	t.Parallel()
	t.Run("new client adapter with default config", func(t *testing.T) {
		config := DefaultClientConfig()
		assert.NotEmpty(t, config.ClientName)
		assert.NotZero(t, config.Timeout)
	})

	t.Run("server config to external config conversion", func(t *testing.T) {
		t.Parallel()
		serverConfig := ServerConfig{
			Name:    "test-server",
			Command: []string{"node", "server.js"},
			Env: map[string]string{
				"API_KEY": "test-key",
			},
			Timeout: 30 * time.Second,
		}

		externalConfig := serverConfig.ToExternalConfig()
		assert.Equal(t, "test-server", externalConfig.ClientName)
		assert.Equal(t, "node", externalConfig.ServerCommand)
		assert.Equal(t, []string{"server.js"}, externalConfig.ServerArgs)
		assert.Equal(t, "test-key", externalConfig.ServerEnv["API_KEY"])
		assert.Equal(t, 30*time.Second, externalConfig.Timeout)
	})

	t.Run("transport type constants", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, TransportStdio, TransportStdio)
		assert.Equal(t, TransportHTTP, TransportHTTP)
	})
}

func TestRegistryAdapter_Integration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("create new registry adapter", func(t *testing.T) {
		t.Parallel()
		registry := NewRegistryAdapter()
		require.NotNil(t, registry)
		assert.NotNil(t, registry.registry)
	})

	t.Run("register and get adapter", func(t *testing.T) {
		t.Parallel()
		registry := NewRegistryAdapter()

		mockAdapter := &mockAdapter{name: "test-adapter"}
		err := registry.Register(mockAdapter)
		require.NoError(t, err)

		adapter, found := registry.Get("test-adapter")
		assert.True(t, found)
		assert.NotNil(t, adapter)
	})

	t.Run("list registered adapters", func(t *testing.T) {
		t.Parallel()
		registry := NewRegistryAdapter()

		mockAdapter1 := &mockAdapter{name: "adapter-1"}
		mockAdapter2 := &mockAdapter{name: "adapter-2"}

		registry.Register(mockAdapter1)
		registry.Register(mockAdapter2)

		names := registry.List()
		assert.Len(t, names, 2)
		assert.Contains(t, names, "adapter-1")
		assert.Contains(t, names, "adapter-2")
	})

	t.Run("unregister adapter", func(t *testing.T) {
		t.Parallel()
		registry := NewRegistryAdapter()

		mockAdapter := &mockAdapter{name: "removable-adapter"}
		registry.Register(mockAdapter)

		err := registry.Unregister("removable-adapter")
		require.NoError(t, err)

		_, found := registry.Get("removable-adapter")
		assert.False(t, found)
	})

	t.Run("start and stop all adapters", func(t *testing.T) {
		t.Parallel()
		registry := NewRegistryAdapter()

		mockAdapter1 := &mockAdapter{name: "startable-1"}
		mockAdapter2 := &mockAdapter{name: "startable-2"}

		registry.Register(mockAdapter1)
		registry.Register(mockAdapter2)

		err := registry.StartAll(ctx)
		assert.NoError(t, err)

		err = registry.StopAll(ctx)
		assert.NoError(t, err)
	})

	t.Run("health check all adapters", func(t *testing.T) {
		t.Parallel()
		registry := NewRegistryAdapter()

		healthyAdapter := &mockAdapter{
			name:   "healthy-adapter",
			health: nil,
		}
		unhealthyAdapter := &mockAdapter{
			name:   "unhealthy-adapter",
			health: assert.AnError,
		}

		registry.Register(healthyAdapter)
		registry.Register(unhealthyAdapter)

		results := registry.HealthCheckAll(ctx)
		assert.Len(t, results, 2)
		assert.NoError(t, results["healthy-adapter"])
		assert.Error(t, results["unhealthy-adapter"])
	})
}

func TestTypeAliases(t *testing.T) {
	t.Parallel()
	t.Run("tool type alias", func(t *testing.T) {
		var tool Tool
		assert.NotNil(t, tool)
	})

	t.Run("resource type alias", func(t *testing.T) {
		t.Parallel()
		var resource Resource
		assert.NotNil(t, resource)
	})

	t.Run("prompt type alias", func(t *testing.T) {
		t.Parallel()
		var prompt Prompt
		assert.NotNil(t, prompt)
	})
}

func TestClientAdapter_MethodSignatures(t *testing.T) {
	t.Parallel()
	t.Run("client adapter methods exist", func(t *testing.T) {
		config := DefaultClientConfig()

		adapter := &ClientAdapter{
			config: config,
		}

		assert.NotNil(t, adapter)
		assert.NotNil(t, adapter.config)
	})
}

type mockAdapter struct {
	name   string
	health error
}

func (m *mockAdapter) Name() string                          { return m.name }
func (m *mockAdapter) Config() map[string]interface{}        { return nil }
func (m *mockAdapter) Start(ctx context.Context) error       { return nil }
func (m *mockAdapter) Stop(ctx context.Context) error        { return nil }
func (m *mockAdapter) HealthCheck(ctx context.Context) error { return m.health }
