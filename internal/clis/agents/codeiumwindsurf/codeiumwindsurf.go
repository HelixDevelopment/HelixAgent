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

// errNoHeadlessCLI is the honest error returned for Codeium Windsurf
// operations. Windsurf is an AI-native IDE; its completion / chat / Cascade
// agent run inside the editor, NOT from a standalone headless CLI this
// integration can exec. Fabricating "// Codeium completion" / "Codeium: <msg>"
// / a templated cascade result without running anything is a BLUFF-001
// violation, so these surface an honest error instead.
var errNoHeadlessCLI = fmt.Errorf("codeium windsurf has no headless CLI: it is an AI-native IDE whose completion/chat/Cascade run inside the editor, not from a standalone CLI")

// complete is unsupported headlessly — Windsurf completion runs in-editor.
func (c *CodeiumWindsurf) complete(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("complete: %w", errNoHeadlessCLI)
}

// chat is unsupported headlessly — Windsurf chat runs in-editor.
func (c *CodeiumWindsurf) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}
	return nil, fmt.Errorf("chat: %w", errNoHeadlessCLI)
}

// cascade is unsupported headlessly — Windsurf Cascade runs in-editor.
func (c *CodeiumWindsurf) cascade(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}
	return nil, fmt.Errorf("cascade: %w", errNoHeadlessCLI)
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
