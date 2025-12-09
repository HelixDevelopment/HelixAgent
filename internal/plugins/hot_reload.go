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
	"github.com/superagent/superagent/internal/config"
)

// HotReloadManager manages plugin hot-reloading functionality
type HotReloadManager struct {
	registry    *Registry
	loader      *Loader
	watcher     *fsnotify.Watcher
	pluginPaths []string
	pluginMap   map[string]string // plugin name -> file path
	mu          sync.RWMutex
	enabled     bool
	stopChan    chan struct{}
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
		pluginMap:   make(map[string]string),
		enabled:     true,
		stopChan:    make(chan struct{}),
	}

	// Add watch paths
	for _, path := range watchPaths {
		if err := watcher.Add(path); err != nil {
			watcher.Close()
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
		fmt.Errorf("Failed to load existing plugins: %v", err)
	}

	// Start the watch loop
	go h.watchLoop(ctx)

	fmt.Printf("Plugin hot-reload manager started successfully")
	return nil
}

// Stop stops the hot-reload monitoring
func (h *HotReloadManager) Stop() error {
	fmt.Printf("Stopping plugin hot-reload manager")

	close(h.stopChan)
	h.watcher.Close()

	fmt.Printf("Plugin hot-reload manager stopped")
	return nil
}

// LoadPlugin loads a plugin from the specified path
func (h *HotReloadManager) LoadPlugin(path string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

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
	h.pluginMap[pluginName] = path

	fmt.Printf("Successfully loaded plugin: %s from %s", pluginName, path)
	return nil
}

// UnloadPlugin unloads a plugin by name
func (h *HotReloadManager) UnloadPlugin(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if plugin is loaded
	path, exists := h.pluginMap[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// Unload the plugin
	if err := h.loader.Unload(name); err != nil {
		return fmt.Errorf("failed to unload plugin %s: %w", name, err)
	}

	// Remove from tracking
	delete(h.pluginMap, name)

	fmt.Printf("Successfully unloaded plugin: %s from %s", name, path)
	return nil
}

// ReloadPlugin reloads a plugin by name
func (h *HotReloadManager) ReloadPlugin(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Get the plugin path
	path, exists := h.pluginMap[name]
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
	h.pluginMap[name] = path

	fmt.Printf("Successfully reloaded plugin: %s from %s", name, path)
	return plugin.Init(nil) // Re-initialize with default config
}

// GetLoadedPlugins returns a list of currently loaded plugins
func (h *HotReloadManager) GetLoadedPlugins() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	plugins := make([]string, 0, len(h.pluginMap))
	for name := range h.pluginMap {
		plugins = append(plugins, name)
	}

	return plugins
}

// GetPluginInfo returns information about a loaded plugin
func (h *HotReloadManager) GetPluginInfo(name string) (map[string]interface{}, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	path, exists := h.pluginMap[name]
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
			if timer, exists := debounce[event.Name]; exists {
				timer.Stop()
			}

			debounce[event.Name] = time.AfterFunc(debounceDuration, func() {
				delete(debounce, event.Name)
				h.handleFileEvent(event)
			})

		case err, ok := <-h.watcher.Errors:
			if !ok {
				return
			}
			fmt.Errorf("File watcher error: %v", err)
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
			if _, exists := h.pluginMap[pluginName]; exists {
				// Reload existing plugin
				if err := h.ReloadPlugin(pluginName); err != nil {
					fmt.Errorf("Failed to reload plugin %s: %v", pluginName, err)
				} else {
					fmt.Printf("Successfully reloaded plugin: %s", pluginName)
				}
			} else {
				// Load new plugin
				if err := h.LoadPlugin(path); err != nil {
					fmt.Errorf("Failed to load plugin %s: %v", path, err)
				} else {
					fmt.Printf("Successfully loaded new plugin: %s", pluginName)
				}
			}
		}

	case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
		// File removed - unload plugin
		if pluginName != "" {
			if err := h.UnloadPlugin(pluginName); err != nil {
				fmt.Errorf("Failed to unload plugin %s: %v", pluginName, err)
			} else {
				fmt.Printf("Successfully unloaded plugin: %s", pluginName)
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
				fmt.Printf("Loading existing plugin: %s", path)
				if err := h.LoadPlugin(path); err != nil {
					fmt.Errorf("Failed to load plugin %s: %v", path, err)
					// Continue loading other plugins
				}
			}

			return nil
		}); err != nil {
			fmt.Errorf("Error walking plugin directory %s: %v", watchPath, err)
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
	if strings.HasSuffix(base, ".so") {
		base = base[:len(base)-3]
	}
	return base
}

// GetStats returns hot-reload statistics
func (h *HotReloadManager) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]interface{}{
		"enabled":        h.enabled,
		"watch_paths":    h.pluginPaths,
		"loaded_plugins": len(h.pluginMap),
		"plugin_names":   h.GetLoadedPlugins(),
	}
}

// Enable enables hot-reloading
func (h *HotReloadManager) Enable() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = true
	fmt.Printf("Plugin hot-reload enabled")
}

// Disable disables hot-reloading
func (h *HotReloadManager) Disable() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = false
	fmt.Printf("Plugin hot-reload disabled")
}

// IsEnabled returns whether hot-reloading is enabled
func (h *HotReloadManager) IsEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.enabled
}
