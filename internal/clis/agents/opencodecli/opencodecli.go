// Package opencodecli provides Opencode CLI agent integration.
// Opencode CLI: Open source coding assistant.
package opencodecli

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// OpencodeCLI provides Opencode CLI integration
type OpencodeCLI struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Model string
}

// New creates a new Opencode CLI integration
func New() *OpencodeCLI {
	info := agents.AgentInfo{
		Type:        agents.TypeOpencodeCLI,
		Name:        "Opencode CLI",
		Description: "Open source coding assistant",
		Vendor:      "Opencode",
		Version:     "1.0.0",
		Capabilities: []string{
			"open_source",
			"code_generation",
			"chat",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &OpencodeCLI{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "default",
		},
	}
}

// Initialize initializes Opencode CLI
func (o *OpencodeCLI) Initialize(ctx context.Context, config interface{}) error {
	if err := o.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		o.config = cfg
	}

	return nil
}

// Execute executes a command
func (o *OpencodeCLI) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !o.IsStarted() {
		if err := o.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "chat":
		return o.chat(ctx, params)
	case "generate":
		return o.generate(ctx, params)
	case "status":
		return o.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// chat performs chat
func (o *OpencodeCLI) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	return map[string]interface{}{
		"message":  message,
		"response": fmt.Sprintf("Opencode: %s", message),
	}, nil
}

// generate generates code
func (o *OpencodeCLI) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return map[string]interface{}{
		"prompt": prompt,
		"code":   fmt.Sprintf("// Opencode\n// %s", prompt),
	}, nil
}

// status returns status
func (o *OpencodeCLI) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": o.IsAvailable(),
		"model":     o.config.Model,
	}, nil
}

// IsAvailable checks availability
func (o *OpencodeCLI) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*OpencodeCLI)(nil)
