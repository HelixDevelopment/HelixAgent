// Package octogen provides Octogen agent integration.
// Octogen: Multi-model code generation system.
package octogen

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Octogen provides Octogen integration
type Octogen struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Models []string
}

// New creates a new Octogen integration
func New() *Octogen {
	info := agents.AgentInfo{
		Type:        agents.TypeOctogen,
		Name:        "Octogen",
		Description: "Multi-model code generation",
		Vendor:      "Octogen",
		Version:     "1.0.0",
		Capabilities: []string{
			"multi_model",
			"ensemble",
			"code_generation",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Octogen{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Models: []string{"gpt-4", "claude-3"},
		},
	}
}

// Initialize initializes Octogen
func (o *Octogen) Initialize(ctx context.Context, config interface{}) error {
	if err := o.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		o.config = cfg
	}

	return nil
}

// Execute executes a command
func (o *Octogen) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !o.IsStarted() {
		if err := o.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "generate":
		return o.generate(ctx, params)
	case "status":
		return o.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// generate generates code
func (o *Octogen) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return map[string]interface{}{
		"prompt": prompt,
		"code":   fmt.Sprintf("// Octogen multi-model\n// %s", prompt),
		"models": o.config.Models,
	}, nil
}

// status returns status
func (o *Octogen) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": o.IsAvailable(),
		"models":    o.config.Models,
	}, nil
}

// IsAvailable checks availability
func (o *Octogen) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*Octogen)(nil)
