// Package nanocoder provides Nanocoder agent integration.
// Nanocoder: Minimalist AI code generator.
package nanocoder

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Nanocoder provides Nanocoder integration
type Nanocoder struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Model string
}

// New creates a new Nanocoder integration
func New() *Nanocoder {
	info := agents.AgentInfo{
		Type:        agents.TypeNanocoder,
		Name:        "Nanocoder",
		Description: "Minimalist AI code generator",
		Vendor:      "Nanocoder",
		Version:     "1.0.0",
		Capabilities: []string{
			"minimal",
			"fast",
			"code_generation",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Nanocoder{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "nano",
		},
	}
}

// Initialize initializes Nanocoder
func (n *Nanocoder) Initialize(ctx context.Context, config interface{}) error {
	if err := n.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		n.config = cfg
	}

	return nil
}

// Execute executes a command
func (n *Nanocoder) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !n.IsStarted() {
		if err := n.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "generate":
		return n.generate(ctx, params)
	case "status":
		return n.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// generate generates code
func (n *Nanocoder) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return map[string]interface{}{
		"prompt": prompt,
		"code":   fmt.Sprintf("// Nanocoder\n%s", prompt),
	}, nil
}

// status returns status
func (n *Nanocoder) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": n.IsAvailable(),
		"model":     n.config.Model,
	}, nil
}

// IsAvailable checks availability
func (n *Nanocoder) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*Nanocoder)(nil)
