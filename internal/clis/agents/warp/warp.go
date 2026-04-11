// Package warp provides Warp agent integration.
// Warp: AI-powered terminal with collaborative features.
package warp

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Warp provides Warp integration
type Warp struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Theme     string
	AIEnabled bool
}

// New creates a new Warp integration
func New() *Warp {
	info := agents.AgentInfo{
		Type:        agents.TypeWarp,
		Name:        "Warp",
		Description: "AI-powered terminal",
		Vendor:      "Warp",
		Version:     "1.0.0",
		Capabilities: []string{
			"terminal",
			"ai_commands",
			"collaboration",
			"modern_ui",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Warp{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Theme:     "dark",
			AIEnabled: true,
		},
	}
}

// Initialize initializes Warp
func (w *Warp) Initialize(ctx context.Context, config interface{}) error {
	if err := w.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		w.config = cfg
	}

	return nil
}

// Execute executes a command
func (w *Warp) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !w.IsStarted() {
		if err := w.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "ai_command":
		return w.aiCommand(ctx, params)
	case "workflow":
		return w.workflow(ctx, params)
	case "status":
		return w.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// aiCommand generates AI command
func (w *Warp) aiCommand(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	description, _ := params["description"].(string)
	if description == "" {
		return nil, fmt.Errorf("description required")
	}

	return map[string]interface{}{
		"description": description,
		"command":     fmt.Sprintf("# Warp AI: %s", description),
		"ai_enabled":  w.config.AIEnabled,
	}, nil
}

// workflow manages workflows
func (w *Warp) workflow(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	return map[string]interface{}{
		"name":     name,
		"workflow": fmt.Sprintf("Workflow: %s", name),
	}, nil
}

// status returns status
func (w *Warp) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":  w.IsAvailable(),
		"theme":      w.config.Theme,
		"ai_enabled": w.config.AIEnabled,
	}, nil
}

// IsAvailable checks availability
func (w *Warp) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*Warp)(nil)
