// Package forge provides Forge agent integration.
// Forge: AI-powered dev environment management.
package forge

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Forge provides Forge integration
type Forge struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Environment string
}

// New creates a new Forge integration
func New() *Forge {
	info := agents.AgentInfo{
		Type:        agents.TypeForge,
		Name:        "Forge",
		Description: "AI dev environment management",
		Vendor:      "Forge",
		Version:     "1.0.0",
		Capabilities: []string{
			"env_management",
			"provisioning",
			"automation",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Forge{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Environment: "default",
		},
	}
}

// Initialize initializes Forge
func (f *Forge) Initialize(ctx context.Context, config interface{}) error {
	if err := f.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		f.config = cfg
	}

	return nil
}

// Execute executes a command
func (f *Forge) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !f.IsStarted() {
		if err := f.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "create":
		return f.create(ctx, params)
	case "deploy":
		return f.deploy(ctx, params)
	case "status":
		return f.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// create creates environment
func (f *Forge) create(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	return map[string]interface{}{
		"name":   name,
		"status": "created",
	}, nil
}

// deploy deploys environment
func (f *Forge) deploy(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	env, _ := params["environment"].(string)
	if env == "" {
		env = f.config.Environment
	}

	return map[string]interface{}{
		"environment": env,
		"status":      "deployed",
	}, nil
}

// status returns status
func (f *Forge) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":   f.IsAvailable(),
		"environment": f.config.Environment,
	}, nil
}

// IsAvailable checks availability
func (f *Forge) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*Forge)(nil)
