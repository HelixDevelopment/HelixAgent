package plugins

import (
	"fmt"
	"sync"
)

// Registry implements PluginRegistry with thread-safe operations
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]LLMPlugin
}

func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]LLMPlugin),
	}
}

func (r *Registry) Register(plugin LLMPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := plugin.Name()
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	r.plugins[name] = plugin
	return nil
}

func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	delete(r.plugins, name)
	return nil
}

func (r *Registry) Get(name string) (LLMPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.plugins[name]
	return plugin, exists
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}
