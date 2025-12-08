package plugins

import (
	"fmt"
	"plugin"

	"github.com/superagent/superagent/internal/utils"
)

// Loader implements PluginLoader for loading Go plugins
type Loader struct {
	registry *Registry
}

func NewLoader(registry *Registry) *Loader {
	return &Loader{
		registry: registry,
	}
}

func (l *Loader) Load(path string) (LLMPlugin, error) {
	// Load the plugin
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin %s: %w", path, err)
	}

	// Look up the plugin symbol
	sym, err := p.Lookup("Plugin")
	if err != nil {
		return nil, fmt.Errorf("plugin %s does not export Plugin symbol: %w", path, err)
	}

	// Assert that the symbol is of the correct type
	pluginInstance, ok := sym.(LLMPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %s does not implement LLMPlugin interface", path)
	}

	// Register the plugin
	if err := l.registry.Register(pluginInstance); err != nil {
		return nil, fmt.Errorf("failed to register plugin %s: %w", path, err)
	}

	utils.GetLogger().Infof("Loaded plugin from %s", path)
	return pluginInstance, nil
}

func (l *Loader) Unload(name string) error {
	if err := l.registry.Unregister(name); err != nil {
		return fmt.Errorf("failed to unregister plugin %s: %w", name, err)
	}

	utils.GetLogger().Infof("Unloaded plugin %s", name)
	return nil
}
