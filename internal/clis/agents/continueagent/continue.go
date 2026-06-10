// Package continueagent provides Continue CLI agent integration.
// Continue: Open-source AI code assistant for IDE.
package continueagent

import (
	"context"
	"fmt"
	"os/exec"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Continue provides Continue integration
type Continue struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Continue configuration
type Config struct {
	base.BaseConfig
	ServerURL      string
	AllowAnonymous bool
}

// New creates a new Continue integration
func New() *Continue {
	info := agents.AgentInfo{
		Type:        agents.TypeContinue,
		Name:        "Continue",
		Description: "Open-source AI code assistant for IDE",
		Vendor:      "Continue",
		Version:     "1.0.0",
		Capabilities: []string{
			"ide_integration",
			"autocomplete",
			"chat",
			"edit",
			"actions",
			"context_providers",
			"slash_commands",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Continue{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			ServerURL:      "http://localhost:3000",
			AllowAnonymous: false,
		},
	}
}

// Initialize initializes Continue
func (c *Continue) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return nil
}

// Execute executes a Continue command
func (c *Continue) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "chat":
		return c.chat(ctx, params)
	case "autocomplete":
		return c.autocomplete(ctx, params)
	case "edit":
		return c.edit(ctx, params)
	case "action":
		return c.action(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// errNoHeadlessCLI is returned by every command: Continue is an IDE
// extension (VS Code / JetBrains) with NO official headless coding-agent CLI to
// exec. Per the 2026-06-10 blocked-agent research there is no scriptable
// single-prompt invocation, so reporting a fabricated "sent"/"executed" status
// would be a BLUFF-001 violation — Continue cannot actually run anything here.
var errNoHeadlessCLI = fmt.Errorf(
	"continue is an IDE extension with no headless CLI; chat/edit/action require " +
		"the Continue IDE plugin — this dispatch cannot run it without fabricating")

// chat validates input then returns an honest no-headless-CLI error rather than
// a fabricated "sent" status (BLUFF-001).
func (c *Continue) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	return nil, errNoHeadlessCLI
}

// autocomplete validates input then returns an honest no-headless-CLI error.
func (c *Continue) autocomplete(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, errNoHeadlessCLI
}

// edit validates input then returns an honest no-headless-CLI error.
func (c *Continue) edit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return nil, errNoHeadlessCLI
}

// action validates input then returns an honest no-headless-CLI error.
func (c *Continue) action(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return nil, fmt.Errorf("action required")
	}

	return nil, errNoHeadlessCLI
}

// IsAvailable checks if Continue is available
func (c *Continue) IsAvailable() bool {
	// Check for VS Code or JetBrains extension
	// This is a simplified check
	_, err := exec.LookPath("continue")
	return err == nil
}

var _ agents.AgentIntegration = (*Continue)(nil)
