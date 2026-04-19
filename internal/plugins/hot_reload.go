package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/config"
	"github.com/fsnotify/fsnotify"
)

// HotReloadManager manages plugin hot-reloading functionality.
//
// Concurrent-safe by construction (CONST-029): pluginMap is a safe.Store;
// `enabled` is an atomic.Bool. pluginPaths is set once at construction
// and never mutated thereafter.
type HotReloadManager struct {
	registry    *Registry
	loader      *Loader
	watcher     *fsnotify.Watcher
	pluginPaths []string
	pluginMap   *safe.Store[string, string] // plugin name -> file path
	enabled     atomic.Bool
	stopChan    chan struct{}
	wg          sync.WaitGroup
	stopOnce    sync.Once
}

// HotReloadConfig holds configuration for hot-reload functionality
type HotReloadConfig struct {
	Enabled      bool          `json:"enabled"`
	WatchPaths   []string      `json:"watch_paths"`
	DebounceTime time.Duration `json:"debounce_time"`
	AutoReload   bool          `json:"auto_reload"`
}

// NewHotReloadManager creates a new hot-reload manager
func NewHotReloadManager(cfg *config.Config, registry *Registry) (*HotReloadManager, error) {
	loader := NewLoader(registry)

	// Default watch paths
	watchPaths := []string{"./plugins"}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	manager := &HotReloadManager{
		registry:    registry,
		loader:      loader,
		watcher:     watcher,
		pluginPaths: watchPaths,
		pluginMap:   safe.NewStore[string, string](),
		stopChan:    make(chan struct{}),
	}
	manager.enabled.Store(true)

	// Add watch paths
	for _, path := range watchPaths {
		if err := watcher.Add(path); err != nil {
			_ = watcher.Close()
			return nil, fmt.Errorf("failed to watch path %s: %w", path, err)
		}
	}

	return manager, nil
}

// Start begins the hot-reload monitoring
func (h *HotReloadManager) Start(ctx context.Context) error {
	fmt.Printf("Starting plugin hot-reload manager")

	// Load existing plugins
	if err := h.loadExistingPlugins(); err != nil {
		fmt.Printf("Failed to load existing plugins: %v\n", err)
	}

	// Start the watch loop with WaitGroup tracking
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		h.watchLoop(ctx)
	}()

	fmt.Printf("Plugin hot-reload manager started successfully")
	return nil
}

// Stop stops the hot-reload monitoring
func (h *HotReloadManager) Stop() error {
	fmt.Printf("Stopping plugin hot-reload manager")

	h.stopOnce.Do(func() {
		close(h.stopChan)
	})
	_ = h.watcher.Close()
	h.wg.Wait()

	fmt.Printf("Plugin hot-reload manager stopped")
	return nil
}

// LoadPlugin loads a plugin from the specified path
func (h *HotReloadManager) LoadPlugin(path string) error {
	// Check if plugin file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("plugin file does not exist: %s", path)
	}

	// Load the plugin
	plugin, err := h.loader.Load(path)
	if err != nil {
		return fmt.Errorf("failed to load plugin %s: %w", path, err)
	}

	// Track the plugin
	pluginName := plugin.Name()
	h.pluginMap.Put(pluginName, path)

	fmt.Printf("Successfully loaded plugin: %s from %s", pluginName, path)
	return nil
}

// UnloadPlugin unloads a plugin by name
func (h *HotReloadManager) UnloadPlugin(name string) error {
	path, exists := h.pluginMap.Get(name)
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// Unload the plugin
	if err := h.loader.Unload(name); err != nil {
		return fmt.Errorf("failed to unload plugin %s: %w", name, err)
	}

	// Remove from tracking
	h.pluginMap.Delete(name)

	fmt.Printf("Successfully unloaded plugin: %s from %s", name, path)
	return nil
}

// ReloadPlugin reloads a plugin by name
func (h *HotReloadManager) ReloadPlugin(name string) error {
	path, exists := h.pluginMap.Get(name)
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// Unload the old plugin
	if err := h.loader.Unload(name); err != nil {
		fmt.Printf("Failed to unload old plugin %s: %v", name, err)
	}

	// Load the new plugin
	plugin, err := h.loader.Load(path)
	if err != nil {
		return fmt.Errorf("failed to reload plugin %s: %w", name, err)
	}

	// Update tracking
	h.pluginMap.Put(name, path)

	fmt.Printf("Successfully reloaded plugin: %s from %s", name, path)
	return plugin.Init(nil) // Re-initialize with default config
}

// GetLoadedPlugins returns a list of currently loaded plugins
func (h *HotReloadManager) GetLoadedPlugins() []string {
	return h.pluginMap.Keys()
}

// GetPluginInfo returns information about a loaded plugin
func (h *HotReloadManager) GetPluginInfo(name string) (map[string]interface{}, error) {
	path, exists := h.pluginMap.Get(name)
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}

	plugin, exists := h.registry.Get(name)
	if !exists {
		return nil, fmt.Errorf("plugin %s not registered", name)
	}

	return map[string]interface{}{
		"name":         name,
		"path":         path,
		"version":      plugin.Version(),
		"capabilities": plugin.Capabilities(),
		"loaded_at":    time.Now().Format(time.RFC3339),
	}, nil
}

// watchLoop monitors file system changes
func (h *HotReloadManager) watchLoop(ctx context.Context) {
	debounce := make(map[string]*time.Timer)
	var debounceMu sync.Mutex
	const debounceDuration = 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopChan:
			return
		case event, ok := <-h.watcher.Events:
			if !ok {
				return
			}

			// Only watch for plugin files (.so files)
			if !h.isPluginFile(event.Name) {
				continue
			}

			// Debounce events to avoid multiple reloads
			debounceMu.Lock()
			if timer, exists := debounce[event.Name]; exists {
				timer.Stop()
			}

			eventName := event.Name // Capture for closure
			debounce[eventName] = time.AfterFunc(debounceDuration, func() {
				debounceMu.Lock()
				delete(debounce, eventName)
				debounceMu.Unlock()
				h.handleFileEvent(event)
			})
			debounceMu.Unlock()

		case err, ok := <-h.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("File watcher error: %v\n", err)
		}
	}
}

// handleFileEvent processes file system events
func (h *HotReloadManager) handleFileEvent(event fsnotify.Event) {
	path := event.Name
	pluginName := h.getPluginNameFromPath(path)

	fmt.Printf("Plugin file event: %s %s", event.Op.String(), path)

	switch {
	case event.Has(fsnotify.Create), event.Has(fsnotify.Write):
		// File created or modified - try to load/reload
		if pluginName != "" {
			// Check if plugin is already loaded
			if _, exists := h.pluginMap.Get(pluginName); exists {
				// Reload existing plugin
				if err := h.ReloadPlugin(pluginName); err != nil {
					fmt.Printf("Failed to reload plugin %s: %v\n", pluginName, err)
				} else {
					fmt.Printf("Successfully reloaded plugin: %s\n", pluginName)
				}
			} else {
				// Load new plugin
				if err := h.LoadPlugin(path); err != nil {
					fmt.Printf("Failed to load plugin %s: %v\n", path, err)
				} else {
					fmt.Printf("Successfully loaded new plugin: %s\n", pluginName)
				}
			}
		}

	case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
		// File removed - unload plugin
		if pluginName != "" {
			if err := h.UnloadPlugin(pluginName); err != nil {
				fmt.Printf("Failed to unload plugin %s: %v\n", pluginName, err)
			} else {
				fmt.Printf("Successfully unloaded plugin: %s\n", pluginName)
			}
		}
	}
}

// loadExistingPlugins loads all existing plugin files
func (h *HotReloadManager) loadExistingPlugins() error {
	for _, watchPath := range h.pluginPaths {
		if err := filepath.Walk(watchPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if h.isPluginFile(path) {
				fmt.Printf("Loading existing plugin: %s\n", path)
				if err := h.LoadPlugin(path); err != nil {
					fmt.Printf("Failed to load plugin %s: %v\n", path, err)
					// Continue loading other plugins
				}
			}

			return nil
		}); err != nil {
			fmt.Printf("Error walking plugin directory %s: %v\n", watchPath, err)
		}
	}

	return nil
}

// isPluginFile checks if a file is a plugin file
func (h *HotReloadManager) isPluginFile(path string) bool {
	return strings.HasSuffix(path, ".so") && !strings.HasPrefix(filepath.Base(path), ".")
}

// getPluginNameFromPath extracts plugin name from file path
func (h *HotReloadManager) getPluginNameFromPath(path string) string {
	base := filepath.Base(path)
	// Remove .so extension
	base = strings.TrimSuffix(base, ".so")
	return base
}

// GetStats returns hot-reload statistics
func (h *HotReloadManager) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"enabled":        h.enabled.Load(),
		"watch_paths":    h.pluginPaths,
		"loaded_plugins": h.pluginMap.Len(),
		"plugin_names":   h.GetLoadedPlugins(),
	}
}

// Enable enables hot-reloading
func (h *HotReloadManager) Enable() {
	h.enabled.Store(true)
	fmt.Printf("Plugin hot-reload enabled")
}

// Disable disables hot-reloading
func (h *HotReloadManager) Disable() {
	h.enabled.Store(false)
	fmt.Printf("Plugin hot-reload disabled")
}

// IsEnabled returns whether hot-reloading is enabled
func (h *HotReloadManager) IsEnabled() bool {
	return h.enabled.Load()
}
