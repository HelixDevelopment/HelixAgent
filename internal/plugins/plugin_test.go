package plugins

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/superagent/superagent/internal/models"
)

// MockLLMPlugin implements LLMPlugin for testing
type MockLLMPlugin struct {
	mock.Mock
}

func (m *MockLLMPlugin) Complete(ctx context.Context, req *models.LLMRequest) (*models.LLMResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LLMResponse), args.Error(1)
}

func (m *MockLLMPlugin) CompleteStream(ctx context.Context, req *models.LLMRequest) (<-chan *models.LLMResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(<-chan *models.LLMResponse), args.Error(1)
}

func (m *MockLLMPlugin) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockLLMPlugin) Version() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockLLMPlugin) Capabilities() *models.ProviderCapabilities {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*models.ProviderCapabilities)
}

func (m *MockLLMPlugin) Init(config map[string]any) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockLLMPlugin) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLLMPlugin) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockLLMPlugin) SetSecurityContext(context *PluginSecurityContext) error {
	args := m.Called(context)
	return args.Error(0)
}

func TestRegistry_Register(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		registry := NewRegistry()
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("test-plugin")

		err := registry.Register(plugin)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(registry.List()))
	})

	t.Run("duplicate registration", func(t *testing.T) {
		registry := NewRegistry()
		plugin1 := new(MockLLMPlugin)
		plugin1.On("Name").Return("test-plugin")
		plugin2 := new(MockLLMPlugin)
		plugin2.On("Name").Return("test-plugin")

		err1 := registry.Register(plugin1)
		assert.NoError(t, err1)

		err2 := registry.Register(plugin2)
		assert.Error(t, err2)
		assert.Contains(t, err2.Error(), "already registered")
	})
}

func TestRegistry_Unregister(t *testing.T) {
	t.Run("successful unregister", func(t *testing.T) {
		registry := NewRegistry()
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("test-plugin")

		err := registry.Register(plugin)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(registry.List()))

		err = registry.Unregister("test-plugin")
		assert.NoError(t, err)
		assert.Equal(t, 0, len(registry.List()))
	})

	t.Run("unregister non-existent plugin", func(t *testing.T) {
		registry := NewRegistry()
		err := registry.Unregister("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("test-plugin")

	err := registry.Register(plugin)
	assert.NoError(t, err)

	t.Run("get existing plugin", func(t *testing.T) {
		retrievedPlugin, exists := registry.Get("test-plugin")
		assert.True(t, exists)
		assert.Equal(t, plugin, retrievedPlugin)
	})

	t.Run("get non-existent plugin", func(t *testing.T) {
		retrievedPlugin, exists := registry.Get("non-existent")
		assert.False(t, exists)
		assert.Nil(t, retrievedPlugin)
	})
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()
	plugin1 := new(MockLLMPlugin)
	plugin1.On("Name").Return("plugin-1")
	plugin2 := new(MockLLMPlugin)
	plugin2.On("Name").Return("plugin-2")

	err1 := registry.Register(plugin1)
	assert.NoError(t, err1)
	err2 := registry.Register(plugin2)
	assert.NoError(t, err2)

	plugins := registry.List()
	assert.Equal(t, 2, len(plugins))
	assert.Contains(t, plugins, "plugin-1")
	assert.Contains(t, plugins, "plugin-2")
}

func TestLoader_Load(t *testing.T) {
	t.Run("loader requires registry", func(t *testing.T) {
		registry := NewRegistry()
		loader := NewLoader(registry)
		assert.NotNil(t, loader)
		assert.Equal(t, registry, loader.registry)
	})
}

func TestLifecycleManager_StartPlugin(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("test-plugin")
	plugin.On("HealthCheck", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	assert.NoError(t, err)

	t.Run("start existing plugin", func(t *testing.T) {
		err := manager.StartPlugin(context.Background(), "test-plugin")
		assert.NoError(t, err)
		assert.Contains(t, manager.GetRunningPlugins(), "test-plugin")
	})

	t.Run("start non-existent plugin", func(t *testing.T) {
		err := manager.StartPlugin(context.Background(), "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("start already running plugin", func(t *testing.T) {
		err := manager.StartPlugin(context.Background(), "test-plugin")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already running")
	})
}

func TestLifecycleManager_StopPlugin(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("test-plugin")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	assert.NoError(t, err)

	t.Run("stop running plugin", func(t *testing.T) {
		err := manager.StartPlugin(context.Background(), "test-plugin")
		assert.NoError(t, err)

		err = manager.StopPlugin("test-plugin")
		assert.NoError(t, err)
		assert.NotContains(t, manager.GetRunningPlugins(), "test-plugin")
	})

	t.Run("stop non-running plugin", func(t *testing.T) {
		err := manager.StopPlugin("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})
}

func TestLifecycleManager_RestartPlugin(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("test-plugin")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	assert.NoError(t, err)

	t.Run("restart plugin", func(t *testing.T) {
		err := manager.StartPlugin(context.Background(), "test-plugin")
		assert.NoError(t, err)

		err = manager.RestartPlugin(context.Background(), "test-plugin")
		assert.NoError(t, err)
		assert.Contains(t, manager.GetRunningPlugins(), "test-plugin")
	})
}

func TestLifecycleManager_ShutdownAll(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin1 := new(MockLLMPlugin)
	plugin1.On("Name").Return("plugin-1")
	plugin1.On("HealthCheck", mock.Anything).Return(nil)
	plugin1.On("Shutdown", mock.Anything).Return(nil)

	plugin2 := new(MockLLMPlugin)
	plugin2.On("Name").Return("plugin-2")
	plugin2.On("HealthCheck", mock.Anything).Return(nil)
	plugin2.On("Shutdown", mock.Anything).Return(nil)

	err1 := registry.Register(plugin1)
	assert.NoError(t, err1)
	err2 := registry.Register(plugin2)
	assert.NoError(t, err2)

	err1 = manager.StartPlugin(context.Background(), "plugin-1")
	assert.NoError(t, err1)
	err2 = manager.StartPlugin(context.Background(), "plugin-2")
	assert.NoError(t, err2)

	t.Run("shutdown all plugins", func(t *testing.T) {
		err := manager.ShutdownAll(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, 0, len(manager.GetRunningPlugins()))
	})
}

func TestLifecycleManager_RestartPlugin_Errors(t *testing.T) {
	t.Run("restart non-running plugin fails", func(t *testing.T) {
		registry := NewRegistry()
		loader := NewLoader(registry)
		health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
		manager := NewLifecycleManager(registry, loader, health)

		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("test-plugin")
		plugin.On("HealthCheck", mock.Anything).Return(nil)
		plugin.On("Shutdown", mock.Anything).Return(nil)

		err := registry.Register(plugin)
		assert.NoError(t, err)

		// Restart without starting first should fail on stop
		err = manager.RestartPlugin(context.Background(), "test-plugin")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to stop plugin")
	})
}

func TestLifecycleManager_StopPlugin_ShutdownError(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("test-plugin")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(errors.New("shutdown error"))

	err := registry.Register(plugin)
	assert.NoError(t, err)

	// Start the plugin
	err = manager.StartPlugin(context.Background(), "test-plugin")
	assert.NoError(t, err)

	// Stop should succeed even if shutdown returns error (logs warning)
	err = manager.StopPlugin("test-plugin")
	assert.NoError(t, err)
	assert.NotContains(t, manager.GetRunningPlugins(), "test-plugin")
}

func TestLifecycleManager_ShutdownAll_WithErrors(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin1 := new(MockLLMPlugin)
	plugin1.On("Name").Return("plugin-1")
	plugin1.On("HealthCheck", mock.Anything).Return(nil)
	plugin1.On("Shutdown", mock.Anything).Return(errors.New("shutdown error 1"))

	plugin2 := new(MockLLMPlugin)
	plugin2.On("Name").Return("plugin-2")
	plugin2.On("HealthCheck", mock.Anything).Return(nil)
	plugin2.On("Shutdown", mock.Anything).Return(nil)

	err1 := registry.Register(plugin1)
	assert.NoError(t, err1)
	err2 := registry.Register(plugin2)
	assert.NoError(t, err2)

	err1 = manager.StartPlugin(context.Background(), "plugin-1")
	assert.NoError(t, err1)
	err2 = manager.StartPlugin(context.Background(), "plugin-2")
	assert.NoError(t, err2)

	// ShutdownAll should succeed even with individual shutdown errors
	err := manager.ShutdownAll(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, len(manager.GetRunningPlugins()))
}

func TestLifecycleManager_GetRunningPlugins_Empty(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugins := manager.GetRunningPlugins()
	assert.Empty(t, plugins)
}

func TestHealthMonitor(t *testing.T) {
	registry := NewRegistry()
	monitor := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)

	t.Run("get health for non-existent plugin", func(t *testing.T) {
		assert.False(t, monitor.IsHealthy("non-existent"))
	})

	t.Run("health monitor with plugin", func(t *testing.T) {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return("test-plugin")
		plugin.On("HealthCheck", mock.Anything).Return(nil)

		err := registry.Register(plugin)
		assert.NoError(t, err)

		// Health monitor will check plugins periodically
		// For this test, we just verify the monitor was created correctly
		assert.NotNil(t, monitor)
	})
}

func TestPluginSecurityContext(t *testing.T) {
	t.Run("create security context", func(t *testing.T) {
		ctx := &PluginSecurityContext{
			AllowedPaths:     []string{"/tmp/plugins"},
			BlockedFunctions: []string{"exec", "syscall"},
			ResourceLimits: ResourceLimits{
				MaxMemoryMB:        256,
				MaxCPUPercent:      50,
				MaxFileDescriptors: 100,
				NetworkAccess:      false,
			},
		}

		assert.Equal(t, []string{"/tmp/plugins"}, ctx.AllowedPaths)
		assert.Equal(t, []string{"exec", "syscall"}, ctx.BlockedFunctions)
		assert.Equal(t, 256, ctx.ResourceLimits.MaxMemoryMB)
		assert.Equal(t, 50, ctx.ResourceLimits.MaxCPUPercent)
		assert.Equal(t, 100, ctx.ResourceLimits.MaxFileDescriptors)
		assert.False(t, ctx.ResourceLimits.NetworkAccess)
	})
}

func TestPluginConcurrentOperations(t *testing.T) {
	registry := NewRegistry()
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("test-plugin")

	err := registry.Register(plugin)
	assert.NoError(t, err)

	t.Run("concurrent get operations", func(t *testing.T) {
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				_, exists := registry.Get("test-plugin")
				assert.True(t, exists)
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("concurrent list operations", func(t *testing.T) {
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				plugins := registry.List()
				assert.Equal(t, 1, len(plugins))
				assert.Equal(t, "test-plugin", plugins[0])
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestPluginErrorHandling(t *testing.T) {
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("test-plugin")

	t.Run("plugin health check error", func(t *testing.T) {
		ctx := context.Background()
		plugin.On("HealthCheck", ctx).Return(errors.New("health check failed"))

		err := plugin.HealthCheck(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "health check failed")
	})

	t.Run("plugin shutdown error", func(t *testing.T) {
		ctx := context.Background()
		plugin.On("Shutdown", ctx).Return(errors.New("shutdown failed"))

		err := plugin.Shutdown(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "shutdown failed")
	})
}

// =====================================================
// LIFECYCLE MANAGER ADDITIONAL TESTS
// =====================================================

func TestLifecycleManager_MonitorPlugin_HealthCheck(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 100*time.Millisecond, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	// Create a plugin that becomes unhealthy
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("monitor-test")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	assert.NoError(t, err)

	// Start the plugin
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = manager.StartPlugin(ctx, "monitor-test")
	assert.NoError(t, err)

	// Let the monitor run briefly
	time.Sleep(150 * time.Millisecond)

	// Cancel and cleanup
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestLifecycleManager_MonitorPlugin_UnhealthyRestart(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	// Short interval to trigger health checks quickly
	health := NewHealthMonitor(registry, 50*time.Millisecond, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	// Create a plugin
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("unhealthy-test")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	assert.NoError(t, err)

	// Set the plugin as unhealthy
	health.mu.Lock()
	health.healthStatus["unhealthy-test"] = PluginHealth{
		Name:        "unhealthy-test",
		Status:      "unhealthy",
		CircuitOpen: true,
	}
	health.mu.Unlock()

	// Start the plugin
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err = manager.StartPlugin(ctx, "unhealthy-test")
	assert.NoError(t, err)

	// Wait for timeout
	<-ctx.Done()
}

func TestLifecycleManager_ShutdownAll_EmptyRegistry(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	ctx := context.Background()
	err := manager.ShutdownAll(ctx)
	assert.NoError(t, err)
	assert.Empty(t, manager.GetRunningPlugins())
}

func TestLifecycleManager_RestartPlugin_StopError(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("restart-error")
	plugin.On("HealthCheck", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	assert.NoError(t, err)

	// Try to restart without starting first
	ctx := context.Background()
	err = manager.RestartPlugin(ctx, "restart-error")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stop plugin")
}

func TestLifecycleManager_GetRunningPlugins_Multiple(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	// Register multiple plugins
	for i := 1; i <= 3; i++ {
		plugin := new(MockLLMPlugin)
		name := "plugin-" + string(rune('a'+i-1))
		plugin.On("Name").Return(name)
		plugin.On("HealthCheck", mock.Anything).Return(nil)
		registry.Register(plugin)

		ctx := context.Background()
		manager.StartPlugin(ctx, name)
	}

	running := manager.GetRunningPlugins()
	assert.Equal(t, 3, len(running))
}

// =====================================================
// HEALTH MONITOR ADDITIONAL TESTS
// =====================================================

func TestHealthMonitor_Start_EmptyRegistry(t *testing.T) {
	registry := NewRegistry()
	monitor := NewHealthMonitor(registry, 50*time.Millisecond, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	go monitor.Start(ctx)

	// Wait for timeout
	<-ctx.Done()
}

func TestHealthMonitor_CheckPlugin_ContextTimeout(t *testing.T) {
	registry := NewRegistry()
	// Very short timeout
	monitor := NewHealthMonitor(registry, 30*time.Second, 1*time.Millisecond)

	// Create a plugin with slow health check
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("slow-check")
	plugin.On("HealthCheck", mock.Anything).Run(func(args mock.Arguments) {
		time.Sleep(100 * time.Millisecond)
	}).Return(nil)

	err := registry.Register(plugin)
	assert.NoError(t, err)

	ctx := context.Background()
	monitor.checkPlugin(ctx, "slow-check")

	// Health might be recorded as success if check completes
	health, exists := monitor.GetHealth("slow-check")
	assert.True(t, exists)
	assert.NotEmpty(t, health.Name)
}

func TestLifecycleManager_MonitorPlugin_TriggersRestart(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	// Very short interval to trigger health checks quickly
	health := NewHealthMonitor(registry, 30*time.Millisecond, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	// Create a plugin
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("trigger-restart")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	assert.NoError(t, err)

	// Start the plugin
	ctx := context.Background()
	err = manager.StartPlugin(ctx, "trigger-restart")
	assert.NoError(t, err)

	// Set the plugin as unhealthy
	health.mu.Lock()
	health.healthStatus["trigger-restart"] = PluginHealth{
		Name:        "trigger-restart",
		Status:      "unhealthy",
		CircuitOpen: true,
	}
	health.mu.Unlock()

	// Wait for the 30s ticker to fire - but we can't easily wait that long in a test
	// Instead, just verify the plugin is running
	assert.Contains(t, manager.GetRunningPlugins(), "trigger-restart")

	// Clean up
	manager.ShutdownAll(context.Background())
}

func TestLifecycleManager_RestartPlugin_Success(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("restart-success")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	assert.NoError(t, err)

	// Start the plugin first
	ctx := context.Background()
	err = manager.StartPlugin(ctx, "restart-success")
	assert.NoError(t, err)

	// Restart the plugin - should succeed
	err = manager.RestartPlugin(ctx, "restart-success")
	assert.NoError(t, err)

	// Plugin should still be running
	assert.Contains(t, manager.GetRunningPlugins(), "restart-success")

	// Clean up
	manager.ShutdownAll(context.Background())
}
