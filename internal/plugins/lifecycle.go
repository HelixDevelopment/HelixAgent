package plugins

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/superagent/superagent/internal/utils"
)

// LifecycleManager handles plugin lifecycle operations
type LifecycleManager struct {
	registry *Registry
	loader   *Loader
	health   *HealthMonitor
	running  map[string]context.CancelFunc
	mu       sync.RWMutex
}

func NewLifecycleManager(registry *Registry, loader *Loader, health *HealthMonitor) *LifecycleManager {
	return &LifecycleManager{
		registry: registry,
		loader:   loader,
		health:   health,
		running:  make(map[string]context.CancelFunc),
	}
}

func (l *LifecycleManager) StartPlugin(ctx context.Context, name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.running[name]; exists {
		return fmt.Errorf("plugin %s is already running", name)
	}

	plugin, exists := l.registry.Get(name)
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// Create context for the plugin
	pluginCtx, cancel := context.WithCancel(ctx)
	l.running[name] = cancel

	// Start plugin monitoring in background
	go l.monitorPlugin(pluginCtx, plugin)

	utils.GetLogger().Infof("Started plugin %s", name)
	return nil
}

func (l *LifecycleManager) StopPlugin(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	cancel, exists := l.running[name]
	if !exists {
		return fmt.Errorf("plugin %s is not running", name)
	}

	cancel()
	delete(l.running, name)

	// Shutdown the plugin
	if plugin, exists := l.registry.Get(name); exists {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := plugin.Shutdown(ctx); err != nil {
			utils.GetLogger().Warnf("Error shutting down plugin %s: %v", name, err)
		}
	}

	utils.GetLogger().Infof("Stopped plugin %s", name)
	return nil
}

func (l *LifecycleManager) RestartPlugin(ctx context.Context, name string) error {
	if err := l.StopPlugin(name); err != nil {
		return fmt.Errorf("failed to stop plugin: %w", err)
	}

	// Wait a moment for cleanup
	time.Sleep(1 * time.Second)

	if err := l.StartPlugin(ctx, name); err != nil {
		return fmt.Errorf("failed to start plugin: %w", err)
	}

	utils.GetLogger().Infof("Restarted plugin %s", name)
	return nil
}

func (l *LifecycleManager) GetRunningPlugins() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	plugins := make([]string, 0, len(l.running))
	for name := range l.running {
		plugins = append(plugins, name)
	}
	return plugins
}

func (l *LifecycleManager) monitorPlugin(ctx context.Context, plugin LLMPlugin) {
	name := plugin.Name()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !l.health.IsHealthy(name) {
				utils.GetLogger().Warnf("Plugin %s is unhealthy, attempting restart", name)
				if err := l.RestartPlugin(context.Background(), name); err != nil {
					utils.GetLogger().Errorf("Failed to restart unhealthy plugin %s: %v", name, err)
				}
			}
		}
	}
}

func (l *LifecycleManager) ShutdownAll(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for name, cancel := range l.running {
		cancel()
		if plugin, exists := l.registry.Get(name); exists {
			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
			if err := plugin.Shutdown(shutdownCtx); err != nil {
				utils.GetLogger().Warnf("Error shutting down plugin %s: %v", name, err)
			}
			shutdownCancel()
		}
	}

	l.running = make(map[string]context.CancelFunc)
	utils.GetLogger().Info("Shut down all plugins")
	return nil
}
