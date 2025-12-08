package plugins

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/superagent/superagent/internal/utils"
)

// Reloader handles hot-reloading of plugin configurations without service interruption
type Reloader struct {
	registry    *Registry
	configMgr   *ConfigManager
	lifecycle   *LifecycleManager
	mu          sync.RWMutex
	lastReload  map[string]time.Time
	reloadDelay time.Duration
}

func NewReloader(registry *Registry, configMgr *ConfigManager, lifecycle *LifecycleManager) *Reloader {
	return &Reloader{
		registry:    registry,
		configMgr:   configMgr,
		lifecycle:   lifecycle,
		lastReload:  make(map[string]time.Time),
		reloadDelay: 5 * time.Second, // Minimum delay between reloads
	}
}

func (r *Reloader) ReloadPluginConfig(ctx context.Context, pluginName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check reload delay
	if last, exists := r.lastReload[pluginName]; exists {
		if time.Since(last) < r.reloadDelay {
			return fmt.Errorf("reload too frequent for plugin %s", pluginName)
		}
	}

	plugin, exists := r.registry.Get(pluginName)
	if !exists {
		return fmt.Errorf("plugin %s not found", pluginName)
	}

	// Load new configuration
	newConfig, err := r.configMgr.LoadPluginConfig(pluginName)
	if err != nil {
		return fmt.Errorf("failed to load new config: %w", err)
	}

	// Validate new configuration
	if err := r.configMgr.ValidateConfig(pluginName, newConfig); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Apply configuration to plugin
	if err := plugin.Init(newConfig); err != nil {
		return fmt.Errorf("failed to apply new configuration: %w", err)
	}

	r.lastReload[pluginName] = time.Now()
	utils.GetLogger().Infof("Successfully reloaded configuration for plugin %s", pluginName)
	return nil
}

func (r *Reloader) ReloadAllConfigs(ctx context.Context) error {
	plugins := r.registry.List()

	for _, name := range plugins {
		if err := r.ReloadPluginConfig(ctx, name); err != nil {
			utils.GetLogger().Warnf("Failed to reload config for plugin %s: %v", name, err)
			// Continue with other plugins
		}
	}

	utils.GetLogger().Info("Completed configuration reload for all plugins")
	return nil
}

func (r *Reloader) WatchForConfigChanges(ctx context.Context, configDir string) {
	// TODO: Watch configuration files for changes
	// This would integrate with the file watcher
	utils.GetLogger().Info("Configuration watching not yet implemented")
}

func (r *Reloader) GetLastReloadTime(pluginName string) (time.Time, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	time, exists := r.lastReload[pluginName]
	return time, exists
}

func (r *Reloader) ForceReload(pluginName string) error {
	r.mu.Lock()
	delete(r.lastReload, pluginName)
	r.mu.Unlock()

	ctx := context.Background()
	return r.ReloadPluginConfig(ctx, pluginName)
}
