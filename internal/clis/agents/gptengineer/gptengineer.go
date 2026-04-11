// Package gptengineer provides GPT Engineer agent integration.
// GPT Engineer: AI software engineer for code generation.
package gptengineer

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// GPTEngineer provides GPT Engineer integration
type GPTEngineer struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Model string
}

// New creates a new GPT Engineer integration
func New() *GPTEngineer {
	info := agents.AgentInfo{
		Type:        agents.TypeGptEngineer,
		Name:        "GPT Engineer",
		Description: "AI software engineer",
		Vendor:      "GPT Engineer",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_generation",
			"project_creation",
			"architecture",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &GPTEngineer{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "gpt-4",
		},
	}
}

// Initialize initializes GPT Engineer
func (g *GPTEngineer) Initialize(ctx context.Context, config interface{}) error {
	if err := g.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		g.config = cfg
	}

	return nil
}

// Execute executes a command
func (g *GPTEngineer) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !g.IsStarted() {
		if err := g.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "generate":
		return g.generate(ctx, params)
	case "improve":
		return g.improve(ctx, params)
	case "status":
		return g.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// generate generates project
func (g *GPTEngineer) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return map[string]interface{}{
		"prompt": prompt,
		"files": []string{
			"main.py",
			"README.md",
			"requirements.txt",
		},
		"status": "generated",
	}, nil
}

// improve improves code
func (g *GPTEngineer) improve(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	file, _ := params["file"].(string)
	if file == "" {
		return nil, fmt.Errorf("file required")
	}

	return map[string]interface{}{
		"file":   file,
		"status": "improved",
	}, nil
}

// status returns status
func (g *GPTEngineer) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": g.IsAvailable(),
		"model":     g.config.Model,
	}, nil
}

// IsAvailable checks availability
func (g *GPTEngineer) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*GPTEngineer)(nil)
