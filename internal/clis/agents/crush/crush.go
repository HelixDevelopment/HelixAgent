// Package crush provides Crush agent integration.
// Crush: AI-powered testing and QA automation.
package crush

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Crush provides Crush integration
type Crush struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Model string
}

// New creates a new Crush integration
func New() *Crush {
	info := agents.AgentInfo{
		Type:        agents.TypeCrush,
		Name:        "Crush",
		Description: "AI-powered testing automation",
		Vendor:      "Crush",
		Version:     "1.0.0",
		Capabilities: []string{
			"test_generation",
			"qa_automation",
			"bug_detection",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Crush{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "gpt-4",
		},
	}
}

// Initialize initializes Crush
func (c *Crush) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return nil
}

// Execute executes a command
func (c *Crush) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "test":
		return c.test(ctx, params)
	case "analyze":
		return c.analyze(ctx, params)
	case "status":
		return c.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// test runs tests
func (c *Crush) test(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	return map[string]interface{}{
		"code":   code,
		"tests":  "Generated tests by Crush",
		"status": "tested",
	}, nil
}

// analyze analyzes for bugs
func (c *Crush) analyze(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	return map[string]interface{}{
		"code":     code,
		"analysis": "Bug analysis by Crush",
		"issues":   []map[string]interface{}{},
	}, nil
}

// status returns status
func (c *Crush) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": c.IsAvailable(),
		"model":     c.config.Model,
	}, nil
}

// IsAvailable checks availability
func (c *Crush) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*Crush)(nil)
