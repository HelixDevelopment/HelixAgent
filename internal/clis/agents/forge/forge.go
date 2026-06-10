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

// errNoForgeWired is returned by create/deploy: no real Forge environment
// backend is wired, and no confirmed headless Forge CLI invocation was found in
// the 2026-06-10 currency/blocked-agent research. Reporting a "created"/
// "deployed" status without actually provisioning anything would be a BLUFF-001
// violation — so these commands return an honest error rather than a fabricated
// success.
var errNoForgeWired = fmt.Errorf(
	"forge environment ops are not wired to a real backend (research-blocked: no " +
		"confirmed headless Forge CLI); cannot report create/deploy without performing it")

// create validates input then returns an honest not-wired error rather than a
// fabricated "created" status (BLUFF-001).
func (f *Forge) create(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	return nil, errNoForgeWired
}

// deploy returns an honest not-wired error rather than a fabricated "deployed"
// status (BLUFF-001).
func (f *Forge) deploy(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, errNoForgeWired
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
