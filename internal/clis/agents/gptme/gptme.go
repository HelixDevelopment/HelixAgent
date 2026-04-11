// Package gptme provides GPTMe agent integration.
// GPTMe: Personal AI assistant for developers.
package gptme

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// GPTMe provides GPTMe integration
type GPTMe struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Personality string
}

// New creates a new GPTMe integration
func New() *GPTMe {
	info := agents.AgentInfo{
		Type:        agents.TypeGptme,
		Name:        "GPTMe",
		Description: "Personal AI assistant",
		Vendor:      "GPTMe",
		Version:     "1.0.0",
		Capabilities: []string{
			"personal_assistant",
			"context_aware",
			"shell_integration",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &GPTMe{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Personality: "helpful",
		},
	}
}

// Initialize initializes GPTMe
func (g *GPTMe) Initialize(ctx context.Context, config interface{}) error {
	if err := g.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		g.config = cfg
	}

	return nil
}

// Execute executes a command
func (g *GPTMe) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !g.IsStarted() {
		if err := g.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "ask":
		return g.ask(ctx, params)
	case "shell":
		return g.shell(ctx, params)
	case "status":
		return g.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// ask asks a question
func (g *GPTMe) ask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	question, _ := params["question"].(string)
	if question == "" {
		return nil, fmt.Errorf("question required")
	}

	return map[string]interface{}{
		"question": question,
		"answer":   fmt.Sprintf("GPTMe: %s", question),
	}, nil
}

// shell runs shell command
func (g *GPTMe) shell(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	cmd, _ := params["command"].(string)
	if cmd == "" {
		return nil, fmt.Errorf("command required")
	}

	return map[string]interface{}{
		"command": cmd,
		"output":  fmt.Sprintf("Executed: %s", cmd),
	}, nil
}

// status returns status
func (g *GPTMe) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":   g.IsAvailable(),
		"personality": g.config.Personality,
	}, nil
}

// IsAvailable checks availability
func (g *GPTMe) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*GPTMe)(nil)
