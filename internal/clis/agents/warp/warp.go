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

// aiCommand returns an HONEST error: Warp is a GUI terminal application whose AI
// command-generation runs inside the Warp app against Warp's own backend; there
// is no headless CLI that turns a description into a command from this
// integration. Refusing to fabricate a command (BLUFF-001). See https://www.warp.dev .
func (w *Warp) aiCommand(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	description, _ := params["description"].(string)
	if description == "" {
		return nil, fmt.Errorf("description required")
	}
	return nil, fmt.Errorf("warp has no headless AI CLI: AI command generation runs inside the Warp GUI terminal app, so it cannot be produced from this integration — refusing to fabricate a command (BLUFF-001)")
}

// workflow returns an HONEST error: Warp Workflows are managed inside the Warp
// app (and its shared YAML workflow repo); there is no headless CLI to
// synthesise a workflow here. Refusing to fabricate one (BLUFF-001).
func (w *Warp) workflow(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}
	return nil, fmt.Errorf("warp has no headless workflow CLI: workflows are managed inside the Warp GUI terminal app — refusing to fabricate a workflow (BLUFF-001)")
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
