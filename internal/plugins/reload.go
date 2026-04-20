package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/utils"
	"github.com/fsnotify/fsnotify"
)

// Reloader handles hot-reloading of plugin configurations without service interruption.
//
// Concurrency model (CONST-029): lastReload is a *safe.Store whose
// Get/Put are atomic. mu (sync.Mutex) survives as a Pattern Zeta
// serialisation lock — it guarantees that the compound "check delay
// → load+validate config → Init plugin → record reload time" flow
// runs as one critical section per plugin. Without it, two concurrent
// reloads for the same plugin could both pass the delay check and
// then race on plugin.Init.
type Reloader struct {
	registry    *Registry
	configMgr   *ConfigManager
	lifecycle   *LifecycleManager
	mu          sync.Mutex
	lastReload  *safe.Store[string, time.Time]
	reloadDelay time.Duration
}

func NewReloader(registry *Registry, configMgr *ConfigManager, lifecycle *LifecycleManager) *Reloader {
	return &Reloader{
		registry:    registry,
		configMgr:   configMgr,
		lifecycle:   lifecycle,
		lastReload:  safe.NewStore[string, time.Time](),
		reloadDelay: 5 * time.Second, // Minimum delay between reloads
	}
}

func (r *Reloader) ReloadPluginConfig(ctx context.Context, pluginName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check reload delay
	if last, exists := r.lastReload.Get(pluginName); exists {
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

	r.lastReload.Put(pluginName, time.Now())
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
	defer func() { _ = watcher.Close() }()

	// Add config directory to watcher
	if err := watcher.Add(configDir); err != nil {
		utils.GetLogger().Errorf("Failed to watch config directory %s: %v", configDir, err)
		return
	}

	utils.GetLogger().Infof("Watching for config changes in: %s", configDir)

	// Debounce mechanism
	debounceTimers := make(map[string]*time.Timer)
	var debounceMu sync.Mutex
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
			debounceMu.Lock()
			if timer, exists := debounceTimers[pluginName]; exists {
				timer.Stop()
			}

			// Capture for closure
			capturedPluginName := pluginName
			capturedEventName := event.Name

			// Create new debounce timer
			debounceTimers[capturedPluginName] = time.AfterFunc(debounceDelay, func() {
				debounceMu.Lock()
				delete(debounceTimers, capturedPluginName)
				debounceMu.Unlock()

				// Check if file still exists
				if _, err := os.Stat(capturedEventName); os.IsNotExist(err) {
					utils.GetLogger().Debugf("Config file no longer exists: %s", capturedEventName)
					return
				}

				utils.GetLogger().Infof("Config file changed: %s, reloading plugin: %s", capturedEventName, capturedPluginName)
				if err := r.ReloadPluginConfig(ctx, capturedPluginName); err != nil {
					utils.GetLogger().Warnf("Failed to reload plugin %s: %v", capturedPluginName, err)
				}
			})
			debounceMu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			utils.GetLogger().Errorf("File watcher error: %v", err)
		}
	}
}

func (r *Reloader) GetLastReloadTime(pluginName string) (time.Time, bool) {
	return r.lastReload.Get(pluginName)
}

func (r *Reloader) ForceReload(pluginName string) error {
	r.lastReload.Delete(pluginName)

	ctx := context.Background()
	return r.ReloadPluginConfig(ctx, pluginName)
}
