// Package codai provides Codai agent integration.
// Codai: AI-powered code review and analysis.
package codai

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Codai provides Codai integration
type Codai struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Model string
}

// New creates a new Codai integration
func New() *Codai {
	info := agents.AgentInfo{
		Type:        agents.TypeCodai,
		Name:        "Codai",
		Description: "AI code review and analysis",
		Vendor:      "Codai",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_review",
			"analysis",
			"suggestions",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Codai{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "gpt-4",
		},
	}
}

// Initialize initializes Codai
func (c *Codai) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return nil
}

// Execute executes a command
func (c *Codai) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "review":
		return c.review(ctx, params)
	case "analyze":
		return c.analyze(ctx, params)
	case "status":
		return c.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// errNoHeadlessCLI is the honest error returned for Codai operations. Codai is
// a hosted/IDE-bound AI code-review product with no confirmed standalone
// headless CLI that this integration can exec. Returning a hardcoded
// "Code review by Codai" string + invented metrics without running anything is
// a BLUFF-001 violation, so these surface an honest error instead.
var errNoHeadlessCLI = fmt.Errorf("codai has no confirmed headless CLI / not supported: cannot run a real code review or analysis from this integration")

// review is unsupported — Codai has no exec-able headless review CLI.
func (c *Codai) review(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}
	return nil, fmt.Errorf("review: %w", errNoHeadlessCLI)
}

// analyze is unsupported — Codai has no exec-able headless analysis CLI.
func (c *Codai) analyze(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	file, _ := params["file"].(string)
	if file == "" {
		return nil, fmt.Errorf("file required")
	}
	return nil, fmt.Errorf("analyze: %w", errNoHeadlessCLI)
}

// status returns status
func (c *Codai) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": c.IsAvailable(),
		"model":     c.config.Model,
	}, nil
}

// IsAvailable reports false: Codai has no confirmed headless CLI this
// integration can exec, so no real operation is available (honest, never a
// fabricated "available").
func (c *Codai) IsAvailable() bool {
	return false
}

var _ agents.AgentIntegration = (*Codai)(nil)
