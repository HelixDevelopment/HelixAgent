// Package uiuxpromax provides UI/UX Pro Max agent integration.
// UI/UX Pro Max: AI-powered UI/UX design tool.
package uiuxpromax

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// UIUXProMax provides UI/UX Pro Max integration
type UIUXProMax struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	DesignSystem string
}

// New creates a new UI/UX Pro Max integration
func New() *UIUXProMax {
	info := agents.AgentInfo{
		Type:        agents.TypeUIUXProMax,
		Name:        "UI/UX Pro Max",
		Description: "AI UI/UX design tool",
		Vendor:      "UIUXProMax",
		Version:     "1.0.0",
		Capabilities: []string{
			"ui_design",
			"ux_design",
			"prototyping",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &UIUXProMax{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			DesignSystem: "material",
		},
	}
}

// Initialize initializes UI/UX Pro Max
func (u *UIUXProMax) Initialize(ctx context.Context, config interface{}) error {
	if err := u.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		u.config = cfg
	}

	return nil
}

// Execute executes a command
func (u *UIUXProMax) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !u.IsStarted() {
		if err := u.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "design":
		return u.design(ctx, params)
	case "prototype":
		return u.prototype(ctx, params)
	case "status":
		return u.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// design generates design
func (u *UIUXProMax) design(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return map[string]interface{}{
		"prompt":        prompt,
		"design":        fmt.Sprintf("UI design for: %s", prompt),
		"design_system": u.config.DesignSystem,
	}, nil
}

// prototype creates prototype
func (u *UIUXProMax) prototype(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	return map[string]interface{}{
		"name":      name,
		"prototype": fmt.Sprintf("Prototype: %s", name),
	}, nil
}

// status returns status
func (u *UIUXProMax) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":     u.IsAvailable(),
		"design_system": u.config.DesignSystem,
	}, nil
}

// IsAvailable checks availability
func (u *UIUXProMax) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*UIUXProMax)(nil)
