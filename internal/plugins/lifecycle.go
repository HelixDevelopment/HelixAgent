package plugins

import (
	"context"
	"fmt"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/utils"
)

// LifecycleManager handles plugin lifecycle operations
type LifecycleManager struct {
	registry *Registry
	loader   *Loader
	health   *HealthMonitor
	running  *safe.Store[string, context.CancelFunc]
}

func NewLifecycleManager(registry *Registry, loader *Loader, health *HealthMonitor) *LifecycleManager {
	return &LifecycleManager{
		registry: registry,
		loader:   loader,
		health:   health,
		running:  safe.NewStore[string, context.CancelFunc](),
	}
}

func (l *LifecycleManager) StartPlugin(ctx context.Context, name string) error {
	if l.running.Has(name) {
		return fmt.Errorf("plugin %s is already running", name)
	}

	plugin, exists := l.registry.Get(name)
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// Create context for the plugin
	pluginCtx, cancel := context.WithCancel(ctx)
	if _, stored := l.running.PutIfAbsent(name, cancel); !stored {
		cancel()
		return fmt.Errorf("plugin %s is already running", name)
	}

	// Start plugin monitoring in background
	//nolint:gosec // G118: long-lived plugin monitor uses its own scoped context, intentionally decoupled from the caller
	go l.monitorPlugin(pluginCtx, plugin)

	utils.GetLogger().Infof("Started plugin %s", name)
	return nil
}

func (l *LifecycleManager) StopPlugin(name string) error {
	cancel, existed := l.running.Delete(name)
	if !existed {
		return fmt.Errorf("plugin %s is not running", name)
	}

	cancel()

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
	return l.running.Keys()
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
	for _, name := range l.running.Keys() {
		cancel, existed := l.running.Delete(name)
		if !existed {
			continue
		}
		cancel()
		if plugin, exists := l.registry.Get(name); exists {
			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
			if err := plugin.Shutdown(shutdownCtx); err != nil {
				utils.GetLogger().Warnf("Error shutting down plugin %s: %v", name, err)
			}
			shutdownCancel()
		}
	}

	utils.GetLogger().Info("Shut down all plugins")
	return nil
}
