package plugins

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func init() {
	runtime.GOMAXPROCS(2)
}

// ---- NewLifecycleManager ----

func TestNewLifecycleManager_Initialization(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)

	manager := NewLifecycleManager(registry, loader, health)
	require.NotNil(t, manager)
	assert.Equal(t, registry, manager.registry)
	assert.Equal(t, loader, manager.loader)
	assert.Equal(t, health, manager.health)
	assert.NotNil(t, manager.running)
	assert.Equal(t, 0, manager.running.Len())
}

func TestNewLifecycleManager_EmptyRunning(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	assert.Empty(t, manager.GetRunningPlugins())
}

// ---- StartPlugin ----

func TestLifecycleManager_StartPlugin_Success(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("start-test")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	require.NoError(t, err)

	err = manager.StartPlugin(context.Background(), "start-test")
	assert.NoError(t, err)
	assert.Contains(t, manager.GetRunningPlugins(), "start-test")

	// Clean up
	_ = manager.ShutdownAll(context.Background())
}

func TestLifecycleManager_StartPlugin_NotFound(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	err := manager.StartPlugin(context.Background(), "nonexistent-plugin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLifecycleManager_StartPlugin_AlreadyRunning(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("already-running")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	require.NoError(t, err)

	err = manager.StartPlugin(context.Background(), "already-running")
	require.NoError(t, err)

	err = manager.StartPlugin(context.Background(), "already-running")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	_ = manager.ShutdownAll(context.Background())
}

func TestLifecycleManager_StartPlugin_MultiplePlugins(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	names := []string{"plugin-alpha", "plugin-beta", "plugin-gamma"}
	for _, name := range names {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return(name)
		plugin.On("HealthCheck", mock.Anything).Return(nil)
		plugin.On("Shutdown", mock.Anything).Return(nil)
		err := registry.Register(plugin)
		require.NoError(t, err)
	}

	for _, name := range names {
		err := manager.StartPlugin(context.Background(), name)
		require.NoError(t, err)
	}

	running := manager.GetRunningPlugins()
	assert.Len(t, running, 3)
	for _, name := range names {
		assert.Contains(t, running, name)
	}

	_ = manager.ShutdownAll(context.Background())
}

func TestLifecycleManager_StartPlugin_WithCancelledContext(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("ctx-cancel")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	err = manager.StartPlugin(ctx, "ctx-cancel")
	require.NoError(t, err)

	// Cancel should stop the monitor goroutine
	cancel()
	time.Sleep(50 * time.Millisecond)

	// Plugin should still be in the running list (cancel stops monitor, not the entry)
	assert.Contains(t, manager.GetRunningPlugins(), "ctx-cancel")

	_ = manager.ShutdownAll(context.Background())
}

// ---- StopPlugin ----

func TestLifecycleManager_StopPlugin_Success(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("stop-test")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	require.NoError(t, err)

	err = manager.StartPlugin(context.Background(), "stop-test")
	require.NoError(t, err)

	err = manager.StopPlugin("stop-test")
	assert.NoError(t, err)
	assert.NotContains(t, manager.GetRunningPlugins(), "stop-test")
}

func TestLifecycleManager_StopPlugin_NotRunning(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	err := manager.StopPlugin("not-running")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestLifecycleManager_StopPlugin_ShutdownError_StillRemoved(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("shutdown-err")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(errors.New("shutdown failed"))

	err := registry.Register(plugin)
	require.NoError(t, err)

	err = manager.StartPlugin(context.Background(), "shutdown-err")
	require.NoError(t, err)

	// StopPlugin should succeed even if Shutdown returns error
	err = manager.StopPlugin("shutdown-err")
	assert.NoError(t, err)
	assert.NotContains(t, manager.GetRunningPlugins(), "shutdown-err")
}

func TestLifecycleManager_StopPlugin_UnregisteredAfterStart(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("unregistered-stop")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	require.NoError(t, err)

	err = manager.StartPlugin(context.Background(), "unregistered-stop")
	require.NoError(t, err)

	// Unregister from registry while running
	err = registry.Unregister("unregistered-stop")
	require.NoError(t, err)

	// StopPlugin should still work (cancel is called, just no shutdown)
	err = manager.StopPlugin("unregistered-stop")
	assert.NoError(t, err)
	assert.NotContains(t, manager.GetRunningPlugins(), "unregistered-stop")
}

// ---- RestartPlugin ----

func TestLifecycleManager_RestartPlugin_SuccessAfterStart(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("restart-ok")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	require.NoError(t, err)

	err = manager.StartPlugin(context.Background(), "restart-ok")
	require.NoError(t, err)

	err = manager.RestartPlugin(context.Background(), "restart-ok")
	assert.NoError(t, err)
	assert.Contains(t, manager.GetRunningPlugins(), "restart-ok")

	_ = manager.ShutdownAll(context.Background())
}

func TestLifecycleManager_RestartPlugin_StopFails(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("restart-stop-fail")
	plugin.On("HealthCheck", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	require.NoError(t, err)

	// Restart without starting first => stop will fail
	err = manager.RestartPlugin(context.Background(), "restart-stop-fail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stop plugin")
}

func TestLifecycleManager_RestartPlugin_PluginNotInRegistry(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	err := manager.RestartPlugin(context.Background(), "phantom")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stop plugin")
}

// ---- GetRunningPlugins ----

func TestLifecycleManager_GetRunningPlugins_EmptyRegistry(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	running := manager.GetRunningPlugins()
	assert.Empty(t, running)
	assert.NotNil(t, running)
}

func TestLifecycleManager_GetRunningPlugins_AfterStartAndStop(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("start-stop-list")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	require.NoError(t, err)

	err = manager.StartPlugin(context.Background(), "start-stop-list")
	require.NoError(t, err)
	assert.Len(t, manager.GetRunningPlugins(), 1)

	err = manager.StopPlugin("start-stop-list")
	require.NoError(t, err)
	assert.Empty(t, manager.GetRunningPlugins())
}

// ---- ShutdownAll ----

func TestLifecycleManager_ShutdownAll_NoPlugins(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	err := manager.ShutdownAll(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, manager.GetRunningPlugins())
}

func TestLifecycleManager_ShutdownAll_MultiplePlugins(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	names := []string{"sd-1", "sd-2", "sd-3"}
	for _, name := range names {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return(name)
		plugin.On("HealthCheck", mock.Anything).Return(nil)
		plugin.On("Shutdown", mock.Anything).Return(nil)
		err := registry.Register(plugin)
		require.NoError(t, err)
		err = manager.StartPlugin(context.Background(), name)
		require.NoError(t, err)
	}

	assert.Len(t, manager.GetRunningPlugins(), 3)

	err := manager.ShutdownAll(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, manager.GetRunningPlugins())
}

func TestLifecycleManager_ShutdownAll_WithShutdownErrors(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin1 := new(MockLLMPlugin)
	plugin1.On("Name").Return("err-sd-1")
	plugin1.On("HealthCheck", mock.Anything).Return(nil)
	plugin1.On("Shutdown", mock.Anything).Return(errors.New("shutdown error"))

	plugin2 := new(MockLLMPlugin)
	plugin2.On("Name").Return("err-sd-2")
	plugin2.On("HealthCheck", mock.Anything).Return(nil)
	plugin2.On("Shutdown", mock.Anything).Return(nil)

	_ = registry.Register(plugin1)
	_ = registry.Register(plugin2)

	_ = manager.StartPlugin(context.Background(), "err-sd-1")
	_ = manager.StartPlugin(context.Background(), "err-sd-2")

	err := manager.ShutdownAll(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, manager.GetRunningPlugins())
}

func TestLifecycleManager_ShutdownAll_ClearsRunningMap(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("clear-test")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	_ = registry.Register(plugin)
	_ = manager.StartPlugin(context.Background(), "clear-test")

	err := manager.ShutdownAll(context.Background())
	assert.NoError(t, err)

	// Should be able to start again after shutdown
	err = manager.StartPlugin(context.Background(), "clear-test")
	assert.NoError(t, err)
	assert.Contains(t, manager.GetRunningPlugins(), "clear-test")

	_ = manager.ShutdownAll(context.Background())
}

// ---- monitorPlugin (indirectly via StartPlugin) ----

func TestLifecycleManager_MonitorPlugin_ContextCancellation(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 50*time.Millisecond, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("monitor-cancel")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = manager.StartPlugin(ctx, "monitor-cancel")
	require.NoError(t, err)

	// Wait for context to expire
	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)

	// The plugin should still be in running map
	assert.Contains(t, manager.GetRunningPlugins(), "monitor-cancel")

	_ = manager.ShutdownAll(context.Background())
}

// ---- Concurrent operations ----

func TestLifecycleManager_ConcurrentStartStop(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	names := []string{"conc-1", "conc-2", "conc-3", "conc-4", "conc-5"}
	for _, name := range names {
		plugin := new(MockLLMPlugin)
		plugin.On("Name").Return(name)
		plugin.On("HealthCheck", mock.Anything).Return(nil)
		plugin.On("Shutdown", mock.Anything).Return(nil)
		_ = registry.Register(plugin)
	}

	var wg sync.WaitGroup

	// Start all concurrently
	for _, name := range names {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			_ = manager.StartPlugin(context.Background(), n)
		}(name)
	}
	wg.Wait()

	// Verify all running
	running := manager.GetRunningPlugins()
	assert.Len(t, running, 5)

	// Stop all concurrently
	for _, name := range names {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			_ = manager.StopPlugin(n)
		}(name)
	}
	wg.Wait()

	assert.Empty(t, manager.GetRunningPlugins())
}

func TestLifecycleManager_ConcurrentGetRunningPlugins(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("conc-get")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	_ = registry.Register(plugin)
	_ = manager.StartPlugin(context.Background(), "conc-get")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			running := manager.GetRunningPlugins()
			assert.NotNil(t, running)
		}()
	}
	wg.Wait()

	_ = manager.ShutdownAll(context.Background())
}

// ---- State transitions ----

func TestLifecycleManager_StateTransitions_StartStopStart(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("state-trans")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	_ = registry.Register(plugin)

	// Start
	err := manager.StartPlugin(context.Background(), "state-trans")
	require.NoError(t, err)
	assert.Contains(t, manager.GetRunningPlugins(), "state-trans")

	// Stop
	err = manager.StopPlugin("state-trans")
	require.NoError(t, err)
	assert.NotContains(t, manager.GetRunningPlugins(), "state-trans")

	// Start again
	err = manager.StartPlugin(context.Background(), "state-trans")
	require.NoError(t, err)
	assert.Contains(t, manager.GetRunningPlugins(), "state-trans")

	_ = manager.ShutdownAll(context.Background())
}

func TestLifecycleManager_StateTransitions_ShutdownAllThenStart(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("post-shutdown")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	_ = registry.Register(plugin)
	_ = manager.StartPlugin(context.Background(), "post-shutdown")

	err := manager.ShutdownAll(context.Background())
	require.NoError(t, err)

	err = manager.StartPlugin(context.Background(), "post-shutdown")
	require.NoError(t, err)
	assert.Contains(t, manager.GetRunningPlugins(), "post-shutdown")

	_ = manager.ShutdownAll(context.Background())
}

// ---- ShutdownAll with context timeout ----

func TestLifecycleManager_ShutdownAll_WithContextTimeout(t *testing.T) {
	registry := NewRegistry()
	loader := NewLoader(registry)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	manager := NewLifecycleManager(registry, loader, health)

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("timeout-sd")
	plugin.On("HealthCheck", mock.Anything).Return(nil)
	plugin.On("Shutdown", mock.Anything).Return(nil)

	_ = registry.Register(plugin)
	_ = manager.StartPlugin(context.Background(), "timeout-sd")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := manager.ShutdownAll(ctx)
	assert.NoError(t, err)
	assert.Empty(t, manager.GetRunningPlugins())
}
