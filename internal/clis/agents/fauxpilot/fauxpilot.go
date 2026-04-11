// Package fauxpilot provides Fauxpilot agent integration.
// Fauxpilot: Self-hosted GitHub Copilot alternative.
package fauxpilot

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Fauxpilot provides Fauxpilot integration
type Fauxpilot struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Endpoint string
	Model    string
}

// New creates a new Fauxpilot integration
func New() *Fauxpilot {
	info := agents.AgentInfo{
		Type:        agents.TypeFauxpilot,
		Name:        "Fauxpilot",
		Description: "Self-hosted Copilot alternative",
		Vendor:      "Fauxpilot",
		Version:     "1.0.0",
		Capabilities: []string{
			"self_hosted",
			"code_completion",
			"privacy",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Fauxpilot{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Endpoint: "http://localhost:5000",
			Model:    "salesforce/codegen-350M-mono",
		},
	}
}

// Initialize initializes Fauxpilot
func (f *Fauxpilot) Initialize(ctx context.Context, config interface{}) error {
	if err := f.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		f.config = cfg
	}

	return nil
}

// Execute executes a command
func (f *Fauxpilot) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !f.IsStarted() {
		if err := f.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "complete":
		return f.complete(ctx, params)
	case "status":
		return f.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// complete generates completion
func (f *Fauxpilot) complete(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prefix, _ := params["prefix"].(string)

	return map[string]interface{}{
		"prefix":     prefix,
		"completion": "// Fauxpilot completion",
		"model":      f.config.Model,
	}, nil
}

// status returns status
func (f *Fauxpilot) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": f.IsAvailable(),
		"endpoint":  f.config.Endpoint,
		"model":     f.config.Model,
	}, nil
}

// IsAvailable checks availability
func (f *Fauxpilot) IsAvailable() bool {
	return f.config.Endpoint != ""
}

var _ agents.AgentIntegration = (*Fauxpilot)(nil)
