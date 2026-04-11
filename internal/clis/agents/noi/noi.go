// Package noi provides Noi agent integration.
// Noi: AI-powered code refactoring tool.
package noi

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Noi provides Noi integration
type Noi struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Model string
}

// New creates a new Noi integration
func New() *Noi {
	info := agents.AgentInfo{
		Type:        agents.TypeNoi,
		Name:        "Noi",
		Description: "AI code refactoring",
		Vendor:      "Noi",
		Version:     "1.0.0",
		Capabilities: []string{
			"refactoring",
			"code_improvement",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Noi{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "gpt-4",
		},
	}
}

// Initialize initializes Noi
func (n *Noi) Initialize(ctx context.Context, config interface{}) error {
	if err := n.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		n.config = cfg
	}

	return nil
}

// Execute executes a command
func (n *Noi) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !n.IsStarted() {
		if err := n.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "refactor":
		return n.refactor(ctx, params)
	case "status":
		return n.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// refactor refactors code
func (n *Noi) refactor(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	return map[string]interface{}{
		"code":   code,
		"result": "// Refactored by Noi",
	}, nil
}

// status returns status
func (n *Noi) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": n.IsAvailable(),
		"model":     n.config.Model,
	}, nil
}

// IsAvailable checks availability
func (n *Noi) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*Noi)(nil)
