// Package codenamegoose provides Codename Goose agent integration.
// Codename Goose: Multi-provider AI agent framework.
package codenamegoose

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// CodenameGoose provides Codename Goose integration
type CodenameGoose struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Provider string
	Model    string
}

// New creates a new Codename Goose integration
func New() *CodenameGoose {
	info := agents.AgentInfo{
		Type:        agents.TypeCodenameGoose,
		Name:        "Codename Goose",
		Description: "Multi-provider AI agent",
		Vendor:      "Goose",
		Version:     "1.0.0",
		Capabilities: []string{
			"multi_provider",
			"extensible",
			"tool_use",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &CodenameGoose{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Provider: "anthropic",
			Model:    "claude-3-sonnet",
		},
	}
}

// Initialize initializes Codename Goose
func (c *CodenameGoose) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return nil
}

// Execute executes a command
func (c *CodenameGoose) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "run":
		return c.run(ctx, params)
	case "configure":
		return c.configure(ctx, params)
	case "status":
		return c.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// run runs the agent
func (c *CodenameGoose) run(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return map[string]interface{}{
		"prompt":   prompt,
		"result":   fmt.Sprintf("Goose result: %s", prompt),
		"provider": c.config.Provider,
	}, nil
}

// configure configures the agent
func (c *CodenameGoose) configure(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if provider, ok := params["provider"].(string); ok {
		c.config.Provider = provider
	}
	if model, ok := params["model"].(string); ok {
		c.config.Model = model
	}

	return map[string]interface{}{
		"provider": c.config.Provider,
		"model":    c.config.Model,
	}, nil
}

// status returns status
func (c *CodenameGoose) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": c.IsAvailable(),
		"provider":  c.config.Provider,
		"model":     c.config.Model,
	}, nil
}

// IsAvailable checks availability
func (c *CodenameGoose) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*CodenameGoose)(nil)
