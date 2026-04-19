package plugins

import (
	"fmt"

	"digital.vasic.concurrency/pkg/safe"
)

// Registry implements PluginRegistry with thread-safe operations
type Registry struct {
	plugins *safe.Store[string, LLMPlugin]
}

func NewRegistry() *Registry {
	return &Registry{
		plugins: safe.NewStore[string, LLMPlugin](),
	}
}

func (r *Registry) Register(plugin LLMPlugin) error {
	name := plugin.Name()
	if _, stored := r.plugins.PutIfAbsent(name, plugin); !stored {
		return fmt.Errorf("plugin %s already registered", name)
	}
	return nil
}

func (r *Registry) Unregister(name string) error {
	if _, existed := r.plugins.Delete(name); !existed {
		return fmt.Errorf("plugin %s not found", name)
	}
	return nil
}

func (r *Registry) Get(name string) (LLMPlugin, bool) {
	return r.plugins.Get(name)
}

func (r *Registry) List() []string {
	return r.plugins.Keys()
}
