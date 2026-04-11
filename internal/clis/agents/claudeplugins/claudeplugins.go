// Package claudeplugins provides Claude Plugins agent integration.
// Claude Plugins: Plugin system for extending Claude capabilities.
package claudeplugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// ClaudePlugins provides Claude Plugins integration
type ClaudePlugins struct {
	*base.BaseIntegration
	config  *Config
	plugins []Plugin
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	AutoLoad bool
}

// Plugin represents a plugin
type Plugin struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Enabled     bool     `json:"enabled"`
	Hooks       []string `json:"hooks"`
}

// New creates a new Claude Plugins integration
func New() *ClaudePlugins {
	info := agents.AgentInfo{
		Type:        agents.TypeClaudePlugins,
		Name:        "Claude Plugins",
		Description: "Plugin system for Claude",
		Vendor:      "Anthropic",
		Version:     "1.0.0",
		Capabilities: []string{
			"plugin_system",
			"extensibility",
			"hook_system",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &ClaudePlugins{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			AutoLoad: true,
		},
		plugins: make([]Plugin, 0),
	}
}

// Initialize initializes Claude Plugins
func (c *ClaudePlugins) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return c.loadPlugins()
}

// loadPlugins loads plugins
func (c *ClaudePlugins) loadPlugins() error {
	pluginsPath := filepath.Join(c.GetWorkDir(), "plugins.json")

	if _, err := os.Stat(pluginsPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(pluginsPath)
	if err != nil {
		return fmt.Errorf("read plugins: %w", err)
	}

	return json.Unmarshal(data, &c.plugins)
}

// savePlugins saves plugins
func (c *ClaudePlugins) savePlugins() error {
	pluginsPath := filepath.Join(c.GetWorkDir(), "plugins.json")
	data, err := json.MarshalIndent(c.plugins, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plugins: %w", err)
	}
	return os.WriteFile(pluginsPath, data, 0644)
}

// Execute executes a command
func (c *ClaudePlugins) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "install":
		return c.install(ctx, params)
	case "uninstall":
		return c.uninstall(ctx, params)
	case "enable":
		return c.enable(ctx, params)
	case "disable":
		return c.disable(ctx, params)
	case "list":
		return c.list(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// install installs a plugin
func (c *ClaudePlugins) install(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	plugin := Plugin{
		ID:          fmt.Sprintf("plugin-%d", len(c.plugins)+1),
		Name:        name,
		Description: fmt.Sprintf("Plugin %s", name),
		Version:     "1.0.0",
		Enabled:     true,
		Hooks:       []string{"on_init", "on_message"},
	}

	c.plugins = append(c.plugins, plugin)

	if err := c.savePlugins(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"plugin": plugin,
		"status": "installed",
	}, nil
}

// uninstall uninstalls a plugin
func (c *ClaudePlugins) uninstall(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	pluginID, _ := params["plugin_id"].(string)
	if pluginID == "" {
		return nil, fmt.Errorf("plugin_id required")
	}

	for i, p := range c.plugins {
		if p.ID == pluginID {
			c.plugins = append(c.plugins[:i], c.plugins[i+1:]...)
			break
		}
	}

	if err := c.savePlugins(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"plugin_id": pluginID,
		"status":    "uninstalled",
	}, nil
}

// enable enables a plugin
func (c *ClaudePlugins) enable(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	pluginID, _ := params["plugin_id"].(string)
	if pluginID == "" {
		return nil, fmt.Errorf("plugin_id required")
	}

	for i := range c.plugins {
		if c.plugins[i].ID == pluginID {
			c.plugins[i].Enabled = true
			break
		}
	}

	if err := c.savePlugins(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"plugin_id": pluginID,
		"status":    "enabled",
	}, nil
}

// disable disables a plugin
func (c *ClaudePlugins) disable(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	pluginID, _ := params["plugin_id"].(string)
	if pluginID == "" {
		return nil, fmt.Errorf("plugin_id required")
	}

	for i := range c.plugins {
		if c.plugins[i].ID == pluginID {
			c.plugins[i].Enabled = false
			break
		}
	}

	if err := c.savePlugins(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"plugin_id": pluginID,
		"status":    "disabled",
	}, nil
}

// list lists plugins
func (c *ClaudePlugins) list(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"plugins": c.plugins,
		"count":   len(c.plugins),
	}, nil
}

// IsAvailable checks availability
func (c *ClaudePlugins) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*ClaudePlugins)(nil)
