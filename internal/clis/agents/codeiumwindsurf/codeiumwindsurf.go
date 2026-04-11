// Package codeiumwindsurf provides Codeium Windsurf integration.
// Codeium Windsurf: AI-native IDE powered by Codeium.
package codeiumwindsurf

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// CodeiumWindsurf provides Codeium Windsurf integration
type CodeiumWindsurf struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Codeium Windsurf configuration
type Config struct {
	base.BaseConfig
	APIKey string
	Model  string
}

// New creates a new Codeium Windsurf integration
func New() *CodeiumWindsurf {
	info := agents.AgentInfo{
		Type:        agents.TypeCodeiumWindsurf,
		Name:        "Codeium Windsurf",
		Description: "AI-native IDE by Codeium",
		Vendor:      "Codeium",
		Version:     "1.0.0",
		Capabilities: []string{
			"ai_completion",
			"chat",
			"code_generation",
			"cascade",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &CodeiumWindsurf{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "codeium-cascade",
		},
	}
}

// Initialize initializes Codeium Windsurf
func (c *CodeiumWindsurf) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return nil
}

// Execute executes a command
func (c *CodeiumWindsurf) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "complete":
		return c.complete(ctx, params)
	case "chat":
		return c.chat(ctx, params)
	case "cascade":
		return c.cascade(ctx, params)
	case "status":
		return c.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// complete generates completion
func (c *CodeiumWindsurf) complete(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prefix, _ := params["prefix"].(string)

	return map[string]interface{}{
		"prefix":     prefix,
		"completion": "// Codeium completion",
	}, nil
}

// chat performs chat
func (c *CodeiumWindsurf) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	return map[string]interface{}{
		"message":  message,
		"response": fmt.Sprintf("Codeium: %s", message),
	}, nil
}

// cascade runs cascade
func (c *CodeiumWindsurf) cascade(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return map[string]interface{}{
		"prompt": prompt,
		"result": fmt.Sprintf("Cascade result for: %s", prompt),
		"files":  []string{"generated.go"},
	}, nil
}

// status returns status
func (c *CodeiumWindsurf) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": c.IsAvailable(),
		"model":     c.config.Model,
	}, nil
}

// IsAvailable checks availability
func (c *CodeiumWindsurf) IsAvailable() bool {
	return c.config.APIKey != ""
}

var _ agents.AgentIntegration = (*CodeiumWindsurf)(nil)
