package plugins

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewHealthMonitor(t *testing.T) {
	registry := NewRegistry()
	monitor := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)

	require.NotNil(t, monitor)
	assert.Equal(t, registry, monitor.registry)
	assert.Equal(t, 30*time.Second, monitor.checkInterval)
	assert.Equal(t, 5*time.Second, monitor.timeout)
	assert.NotNil(t, monitor.healthStatus)
}

func TestHealthMonitor_GetHealth(t *testing.T) {
	registry := NewRegistry()
	monitor := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)

	t.Run("get health for non-existent plugin", func(t *testing.T) {
		health, exists := monitor.GetHealth("non-existent")
		assert.False(t, exists)
		assert.Equal(t, PluginHealth{}, health)
	})

	t.Run("get health after manual set", func(t *testing.T) {
		monitor.mu.Lock()
		monitor.healthStatus["test-plugin"] = PluginHealth{
			Name:   "test-plugin",
			Status: "healthy",
		}
		monitor.mu.Unlock()

		health, exists := monitor.GetHealth("test-plugin")
		assert.True(t, exists)
		assert.Equal(t, "test-plugin", health.Name)
		assert.Equal(t, "healthy", health.Status)
	})
}

func TestHealthMonitor_IsHealthy(t *testing.T) {
	registry := NewRegistry()
	monitor := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)

	t.Run("non-existent plugin not healthy", func(t *testing.T) {
		assert.False(t, monitor.IsHealthy("non-existent"))
	})

	t.Run("healthy plugin", func(t *testing.T) {
		monitor.mu.Lock()
		monitor.healthStatus["healthy-plugin"] = PluginHealth{
			Name:        "healthy-plugin",
			Status:      "healthy",
			CircuitOpen: false,
		}
		monitor.mu.Unlock()

		assert.True(t, monitor.IsHealthy("healthy-plugin"))
	})

	t.Run("degraded plugin not healthy", func(t *testing.T) {
		monitor.mu.Lock()
		monitor.healthStatus["degraded-plugin"] = PluginHealth{
			Name:        "degraded-plugin",
			Status:      "degraded",
			CircuitOpen: false,
		}
		monitor.mu.Unlock()

		assert.False(t, monitor.IsHealthy("degraded-plugin"))
	})

	t.Run("circuit open not healthy", func(t *testing.T) {
		monitor.mu.Lock()
		monitor.healthStatus["circuit-open"] = PluginHealth{
			Name:        "circuit-open",
			Status:      "healthy",
			CircuitOpen: true,
		}
		monitor.mu.Unlock()

		assert.False(t, monitor.IsHealthy("circuit-open"))
	})
}

func TestHealthMonitor_CheckPlugin(t *testing.T) {
	registry := NewRegistry()
	monitor := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)

	t.Run("successful health check", func(t *testing.T) {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("healthy-test")
		plugin.On("HealthCheck", mock.Anything).Return(nil)

		err := registry.Register(plugin)
		require.NoError(t, err)

		ctx := context.Background()
		monitor.checkPlugin(ctx, "healthy-test")

		health, exists := monitor.GetHealth("healthy-test")
		assert.True(t, exists)
		assert.Equal(t, "healthy", health.Status)
		assert.False(t, health.CircuitOpen)
		assert.Equal(t, 0, health.ConsecutiveFailures)
	})

	t.Run("failed health check", func(t *testing.T) {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("failing-test")
		plugin.On("HealthCheck", mock.Anything).Return(errors.New("connection refused"))

		err := registry.Register(plugin)
		require.NoError(t, err)

		ctx := context.Background()
		monitor.checkPlugin(ctx, "failing-test")

		health, exists := monitor.GetHealth("failing-test")
		assert.True(t, exists)
		assert.Equal(t, "unhealthy", health.Status)
		assert.Equal(t, 1, health.ConsecutiveFailures)
		assert.Equal(t, 1, health.ErrorCount)
	})

	t.Run("circuit breaker opens after 3 failures", func(t *testing.T) {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("circuit-test")
		plugin.On("HealthCheck", mock.Anything).Return(errors.New("error"))

		err := registry.Register(plugin)
		require.NoError(t, err)

		ctx := context.Background()

		// Fail 3 times
		for i := 0; i < 3; i++ {
			monitor.checkPlugin(ctx, "circuit-test")
		}

		health, exists := monitor.GetHealth("circuit-test")
		assert.True(t, exists)
		assert.True(t, health.CircuitOpen)
		assert.Equal(t, 3, health.ConsecutiveFailures)
	})

	t.Run("check non-existent plugin does nothing", func(t *testing.T) {
		ctx := context.Background()
		monitor.checkPlugin(ctx, "does-not-exist")
		// Should not panic or error
	})

	t.Run("circuit resets on success", func(t *testing.T) {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("recovery-test")
		// First call fails
		plugin.On("HealthCheck", mock.Anything).Return(errors.New("error")).Once()
		// Second call succeeds
		plugin.On("HealthCheck", mock.Anything).Return(nil).Once()

		err := registry.Register(plugin)
		require.NoError(t, err)

		ctx := context.Background()

		// First check - fails
		monitor.checkPlugin(ctx, "recovery-test")
		health, _ := monitor.GetHealth("recovery-test")
		assert.Equal(t, 1, health.ConsecutiveFailures)

		// Second check - succeeds
		monitor.checkPlugin(ctx, "recovery-test")
		health, _ = monitor.GetHealth("recovery-test")
		assert.Equal(t, 0, health.ConsecutiveFailures)
		assert.False(t, health.CircuitOpen)
	})
}

func TestHealthMonitor_CheckAllPlugins(t *testing.T) {
	registry := NewRegistry()
	monitor := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)

	plugin1 := new(MockLLMPlugin)
	plugin1.On("Name").Return("plugin-1")
	plugin1.On("HealthCheck", mock.Anything).Return(nil)

	plugin2 := new(MockLLMPlugin)
	plugin2.On("Name").Return("plugin-2")
	plugin2.On("HealthCheck", mock.Anything).Return(nil)

	err := registry.Register(plugin1)
	require.NoError(t, err)
	err = registry.Register(plugin2)
	require.NoError(t, err)

	ctx := context.Background()
	monitor.checkAllPlugins(ctx)

	health1, exists1 := monitor.GetHealth("plugin-1")
	assert.True(t, exists1)
	assert.Equal(t, "healthy", health1.Status)

	health2, exists2 := monitor.GetHealth("plugin-2")
	assert.True(t, exists2)
	assert.Equal(t, "healthy", health2.Status)
}

func TestHealthMonitor_Start(t *testing.T) {
	registry := NewRegistry()
	// Short interval for testing
	monitor := NewHealthMonitor(registry, 50*time.Millisecond, 5*time.Second)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("monitored-plugin")
	plugin.On("HealthCheck", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start monitor in goroutine
	go monitor.Start(ctx)

	// Wait for a few checks
	time.Sleep(150 * time.Millisecond)

	// Verify health was checked
	health, exists := monitor.GetHealth("monitored-plugin")
	assert.True(t, exists)
	assert.Equal(t, "healthy", health.Status)
}

func TestPluginHealth_Structure(t *testing.T) {
	health := PluginHealth{
		Name:                "test-plugin",
		Status:              "healthy",
		LastCheck:           time.Now(),
		ResponseTime:        100 * time.Millisecond,
		ErrorCount:          0,
		ConsecutiveFailures: 0,
		CircuitOpen:         false,
	}

	assert.Equal(t, "test-plugin", health.Name)
	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, 100*time.Millisecond, health.ResponseTime)
	assert.False(t, health.CircuitOpen)
}
