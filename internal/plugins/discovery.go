package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/superagent/superagent/internal/utils"
)

// Discovery handles automatic discovery and registration of plugins
type Discovery struct {
	loader    *Loader
	validator *SecurityValidator
	paths     []string
}

func NewDiscovery(loader *Loader, validator *SecurityValidator, paths []string) *Discovery {
	return &Discovery{
		loader:    loader,
		validator: validator,
		paths:     paths,
	}
}

func (d *Discovery) DiscoverAndLoad() error {
	for _, path := range d.paths {
		if err := d.discoverInPath(path); err != nil {
			utils.GetLogger().Warnf("Failed to discover plugins in %s: %v", path, err)
		}
	}
	return nil
}

func (d *Discovery) discoverInPath(rootPath string) error {
	return filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if it's a plugin file
		if strings.HasSuffix(path, ".so") {
			if err := d.loadPlugin(path); err != nil {
				utils.GetLogger().Errorf("Failed to load plugin %s: %v", path, err)
				// Continue with other plugins
			}
		}

		return nil
	})
}

func (d *Discovery) loadPlugin(path string) error {
	// Validate plugin path
	if err := d.validator.ValidatePluginPath(path); err != nil {
		return fmt.Errorf("security validation failed: %w", err)
	}

	// Load the plugin
	plugin, err := d.loader.Load(path)
	if err != nil {
		return fmt.Errorf("failed to load plugin: %w", err)
	}

	// Validate plugin capabilities
	if err := d.validator.ValidatePluginCapabilities(plugin); err != nil {
		return fmt.Errorf("capability validation failed: %w", err)
	}

	// Sandbox the plugin
	if err := d.validator.SandboxPlugin(plugin); err != nil {
		return fmt.Errorf("sandboxing failed: %w", err)
	}

	utils.GetLogger().Infof("Successfully discovered and loaded plugin: %s", plugin.Name())
	return nil
}

func (d *Discovery) WatchForChanges() {
	// TODO: Implement file system watching for hot-reload
	utils.GetLogger().Info("Plugin discovery watching not yet implemented")
}
