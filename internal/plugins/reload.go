package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
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
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		utils.GetLogger().Errorf("Failed to create file watcher: %v", err)
		return
	}
	defer watcher.Close()

	// Add config directory to watcher
	if err := watcher.Add(configDir); err != nil {
		utils.GetLogger().Errorf("Failed to watch config directory %s: %v", configDir, err)
		return
	}

	utils.GetLogger().Infof("Watching for config changes in: %s", configDir)

	// Debounce mechanism
	debounceTimers := make(map[string]*time.Timer)
	debounceDelay := 2 * time.Second

	for {
		select {
		case <-ctx.Done():
			utils.GetLogger().Info("Stopping config watcher")
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Only process write and create events for YAML files
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			if !strings.HasSuffix(event.Name, ".yaml") && !strings.HasSuffix(event.Name, ".yml") {
				continue
			}

			// Extract plugin name from filename
			filename := filepath.Base(event.Name)
			pluginName := strings.TrimSuffix(filename, filepath.Ext(filename))

			// Debounce: cancel previous timer if exists
			if timer, exists := debounceTimers[pluginName]; exists {
				timer.Stop()
			}

			// Create new debounce timer
			debounceTimers[pluginName] = time.AfterFunc(debounceDelay, func() {
				r.mu.Lock()
				delete(debounceTimers, pluginName)
				r.mu.Unlock()

				// Check if file still exists
				if _, err := os.Stat(event.Name); os.IsNotExist(err) {
					utils.GetLogger().Debugf("Config file no longer exists: %s", event.Name)
					return
				}

				utils.GetLogger().Infof("Config file changed: %s, reloading plugin: %s", event.Name, pluginName)
				if err := r.ReloadPluginConfig(ctx, pluginName); err != nil {
					utils.GetLogger().Warnf("Failed to reload plugin %s: %v", pluginName, err)
				}
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			utils.GetLogger().Errorf("File watcher error: %v", err)
		}
	}
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
