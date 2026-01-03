package plugins

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewReloader(t *testing.T) {
	registry := NewRegistry()
	configMgr := NewConfigManager("./configs")
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)

	reloader := NewReloader(registry, configMgr, lifecycle)

	assert.NotNil(t, reloader)
	assert.Equal(t, registry, reloader.registry)
	assert.Equal(t, configMgr, reloader.configMgr)
	assert.Equal(t, lifecycle, reloader.lifecycle)
	assert.NotNil(t, reloader.lastReload)
	assert.Equal(t, 5*time.Second, reloader.reloadDelay)
}

func TestReloader_ReloadPluginConfig_NotFound(t *testing.T) {
	registry := NewRegistry()
	configMgr := NewConfigManager("./configs")
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	ctx := context.Background()
	err := reloader.ReloadPluginConfig(ctx, "nonexistent-plugin")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestReloader_ReloadPluginConfig_TooFrequent(t *testing.T) {
	registry := NewRegistry()

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("test-plugin")
	plugin.On("Init", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	require.NoError(t, err)

	configMgr := NewConfigManager("./configs")
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	// Simulate a recent reload
	reloader.lastReload["test-plugin"] = time.Now()

	ctx := context.Background()
	err = reloader.ReloadPluginConfig(ctx, "test-plugin")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reload too frequent")
}

func TestReloader_GetLastReloadTime(t *testing.T) {
	registry := NewRegistry()
	configMgr := NewConfigManager("./configs")
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	t.Run("returns false for non-existent plugin", func(t *testing.T) {
		_, exists := reloader.GetLastReloadTime("nonexistent")
		assert.False(t, exists)
	})

	t.Run("returns time for reloaded plugin", func(t *testing.T) {
		now := time.Now()
		reloader.lastReload["test-plugin"] = now

		reloadTime, exists := reloader.GetLastReloadTime("test-plugin")
		assert.True(t, exists)
		assert.Equal(t, now, reloadTime)
	})
}

func TestReloader_ForceReload_NotFound(t *testing.T) {
	registry := NewRegistry()
	configMgr := NewConfigManager("./configs")
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	err := reloader.ForceReload("nonexistent-plugin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestReloader_ForceReload_ClearsLastReload(t *testing.T) {
	registry := NewRegistry()
	configMgr := NewConfigManager("./configs")
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	// Set a last reload time
	reloader.lastReload["test-plugin"] = time.Now()

	// ForceReload should clear it and then fail (plugin not found)
	err := reloader.ForceReload("test-plugin")

	// The error is expected because plugin doesn't exist
	assert.Error(t, err)

	// But lastReload should have been cleared
	_, exists := reloader.lastReload["test-plugin"]
	assert.False(t, exists)
}

func TestReloader_ReloadAllConfigs_EmptyRegistry(t *testing.T) {
	registry := NewRegistry()
	configMgr := NewConfigManager("./configs")
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	ctx := context.Background()
	err := reloader.ReloadAllConfigs(ctx)

	// Should succeed with no plugins
	assert.NoError(t, err)
}

func TestReloader_ConcurrentAccess(t *testing.T) {
	registry := NewRegistry()
	configMgr := NewConfigManager("./configs")
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			pluginName := "test-plugin"
			reloader.GetLastReloadTime(pluginName)

			reloader.mu.Lock()
			reloader.lastReload[pluginName] = time.Now()
			reloader.mu.Unlock()

			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestReloader_ReloadAllConfigs_WithPlugins(t *testing.T) {
	registry := NewRegistry()

	plugin1 := new(MockLLMPlugin)
	plugin1.On("Name").Return("plugin-1")
	plugin1.On("Init", mock.Anything).Return(nil)

	plugin2 := new(MockLLMPlugin)
	plugin2.On("Name").Return("plugin-2")
	plugin2.On("Init", mock.Anything).Return(nil)

	err := registry.Register(plugin1)
	require.NoError(t, err)
	err = registry.Register(plugin2)
	require.NoError(t, err)

	configMgr := NewConfigManager("./configs")
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	ctx := context.Background()
	err = reloader.ReloadAllConfigs(ctx)

	// Should not error (continues on individual failures)
	assert.NoError(t, err)
}

func TestReloader_ReloadPluginConfig_ConfigValidationError(t *testing.T) {
	registry := NewRegistry()

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("test-plugin")
	plugin.On("Init", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	require.NoError(t, err)

	// Use a non-existent config directory - will trigger validation error
	configMgr := NewConfigManager("/nonexistent/config/path")
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	ctx := context.Background()
	err = reloader.ReloadPluginConfig(ctx, "test-plugin")

	assert.Error(t, err)
	// The error message varies - either config load or validation fails
	assert.True(t, err != nil)
}

func TestReloader_ForceReload_ResetsDelay(t *testing.T) {
	registry := NewRegistry()

	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("test-plugin")
	plugin.On("Init", mock.Anything).Return(nil)

	err := registry.Register(plugin)
	require.NoError(t, err)

	configMgr := NewConfigManager("/nonexistent/path")
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	// Set a very recent reload time (would normally block)
	reloader.lastReload["test-plugin"] = time.Now()

	// ForceReload should clear lastReload and attempt reload
	// (will fail due to config validation, but exercises the code)
	err = reloader.ForceReload("test-plugin")

	// Should fail at config load or validation, not at "too frequent" check
	assert.Error(t, err)
	// Error should NOT be about "reload too frequent"
	assert.NotContains(t, err.Error(), "reload too frequent")

	// lastReload should be cleared
	_, exists := reloader.lastReload["test-plugin"]
	assert.False(t, exists)
}

func TestReloader_ReloadDelay(t *testing.T) {
	registry := NewRegistry()
	configMgr := NewConfigManager("./configs")
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	// Default reload delay should be 5 seconds
	assert.Equal(t, 5*time.Second, reloader.reloadDelay)
}

// =====================================================
// ADDITIONAL RELOAD TESTS FOR COVERAGE
// =====================================================

func TestReloader_WatchForConfigChanges_ContextCancel(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	configMgr := NewConfigManager(tmpDir)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		reloader.WatchForConfigChanges(ctx, tmpDir)
		close(done)
	}()

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("WatchForConfigChanges did not exit on context cancel")
	}
}

func TestReloader_WatchForConfigChanges_FileEvent(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()

	// Register a plugin
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("config-test")
	plugin.On("Init", mock.Anything).Return(nil)
	err := registry.Register(plugin)
	require.NoError(t, err)

	configMgr := NewConfigManager(tmpDir)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go reloader.WatchForConfigChanges(ctx, tmpDir)

	// Give watcher time to start
	time.Sleep(200 * time.Millisecond)

	// Create a config file
	configFile := filepath.Join(tmpDir, "config-test.yaml")
	err = os.WriteFile(configFile, []byte("name: test\nversion: 1.0.0"), 0644)
	require.NoError(t, err)

	// Wait for debounce + processing
	time.Sleep(2500 * time.Millisecond)
}

func TestReloader_WatchForConfigChanges_NonYamlIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()
	configMgr := NewConfigManager(tmpDir)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go reloader.WatchForConfigChanges(ctx, tmpDir)

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create a non-yaml file (should be ignored)
	txtFile := filepath.Join(tmpDir, "readme.txt")
	err := os.WriteFile(txtFile, []byte("test"), 0644)
	require.NoError(t, err)

	// Wait briefly
	time.Sleep(500 * time.Millisecond)
}

func TestReloader_WatchForConfigChanges_InvalidPath(t *testing.T) {
	registry := NewRegistry()
	configMgr := NewConfigManager("/nonexistent")
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should return early without panicking
	reloader.WatchForConfigChanges(ctx, "/nonexistent/path")
}

func TestReloader_ReloadPluginConfig_Success(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()

	// Create a config file with valid content
	configFile := filepath.Join(tmpDir, "success-plugin.json")
	configContent := `{"name": "success-plugin", "version": "1.0.0"}`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Register a plugin
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("success-plugin")
	plugin.On("Init", mock.Anything).Return(nil)
	err = registry.Register(plugin)
	require.NoError(t, err)

	configMgr := NewConfigManager(tmpDir)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	ctx := context.Background()
	err = reloader.ReloadPluginConfig(ctx, "success-plugin")
	assert.NoError(t, err)

	// Verify last reload time was set
	reloadTime, exists := reloader.GetLastReloadTime("success-plugin")
	assert.True(t, exists)
	assert.False(t, reloadTime.IsZero())
}

func TestReloader_ReloadPluginConfig_InitError(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()

	// Create a config file
	configFile := filepath.Join(tmpDir, "init-error.json")
	configContent := `{"name": "init-error", "version": "1.0.0"}`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Register a plugin that fails on Init
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("init-error")
	plugin.On("Init", mock.Anything).Return(errors.New("init failed"))
	err = registry.Register(plugin)
	require.NoError(t, err)

	configMgr := NewConfigManager(tmpDir)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	ctx := context.Background()
	err = reloader.ReloadPluginConfig(ctx, "init-error")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to apply new configuration")
}

func TestReloader_WatchForConfigChanges_YmlExtension(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()

	// Register a plugin
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("yml-test")
	plugin.On("Init", mock.Anything).Return(nil)
	err := registry.Register(plugin)
	require.NoError(t, err)

	configMgr := NewConfigManager(tmpDir)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go reloader.WatchForConfigChanges(ctx, tmpDir)

	// Give watcher time to start
	time.Sleep(200 * time.Millisecond)

	// Create a .yml config file (should be processed)
	configFile := filepath.Join(tmpDir, "yml-test.yml")
	err = os.WriteFile(configFile, []byte("name: test\nversion: 1.0.0"), 0644)
	require.NoError(t, err)

	// Wait for debounce + processing
	time.Sleep(2500 * time.Millisecond)
}

func TestReloader_WatchForConfigChanges_FileDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	registry := NewRegistry()

	// Register a plugin
	plugin := new(MockLLMPlugin)
	plugin.On("Name").Return("delete-test")
	plugin.On("Init", mock.Anything).Return(nil)
	err := registry.Register(plugin)
	require.NoError(t, err)

	configMgr := NewConfigManager(tmpDir)
	health := NewHealthMonitor(registry, 30*time.Second, 5*time.Second)
	loader := NewLoader(registry)
	lifecycle := NewLifecycleManager(registry, loader, health)
	reloader := NewReloader(registry, configMgr, lifecycle)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	go reloader.WatchForConfigChanges(ctx, tmpDir)

	// Give watcher time to start
	time.Sleep(200 * time.Millisecond)

	// Create and then delete a config file
	configFile := filepath.Join(tmpDir, "delete-test.yaml")
	err = os.WriteFile(configFile, []byte("name: test"), 0644)
	require.NoError(t, err)

	// Delete the file before debounce finishes
	time.Sleep(100 * time.Millisecond)
	os.Remove(configFile)

	// Wait for debounce to trigger and handle missing file
	time.Sleep(2500 * time.Millisecond)
}
