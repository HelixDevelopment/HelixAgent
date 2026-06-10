// Package cline provides Cline CLI agent integration.
// Cline: AI assistant for VS Code with autonomous coding capabilities.
package cline

import (
	"context"
	"fmt"
	"os/exec"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Cline provides Cline integration
type Cline struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Cline configuration
type Config struct {
	base.BaseConfig
	VSCodePath         string
	AutoApprove        bool
	AutoRun            bool
	BrowserViewport    string
	CustomInstructions string
}

// New creates a new Cline integration
func New() *Cline {
	info := agents.AgentInfo{
		Type:        agents.TypeCline,
		Name:        "Cline",
		Description: "AI assistant for VS Code with autonomous coding",
		Vendor:      "Cline",
		Version:     "1.0.0",
		Capabilities: []string{
			"vs_code_integration",
			"autonomous_coding",
			"browser_automation",
			"terminal_integration",
			"file_editing",
			"context_awareness",
			"multi_step_tasks",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Cline{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				Model:     "claude-3-sonnet",
				AutoStart: true,
			},
			VSCodePath:      "code",
			AutoApprove:     false,
			AutoRun:         false,
			BrowserViewport: "1280x720",
		},
	}
}

// Initialize initializes Cline
func (c *Cline) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return nil
}

// Execute executes a Cline command
func (c *Cline) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "open":
		return c.openVSCode(ctx, params)
	case "chat":
		return c.chat(ctx, params)
	case "task":
		return c.task(ctx, params)
	case "browser":
		return c.browserAction(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// openVSCode opens VS Code with Cline
func (c *Cline) openVSCode(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	args := []string{"--extension-id", "saoudrizwan.claude-dev"}

	if folder, ok := params["folder"].(string); ok {
		args = append(args, folder)
	}

	output, err := c.ExecuteCommand(ctx, c.config.VSCodePath, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to open VS Code: %w\n%s", err, string(output))
	}

	return map[string]interface{}{
		"opened":  true,
		"message": "VS Code opened with Cline",
	}, nil
}

// errNoHeadlessCLI is the honest error returned for operations Cline cannot
// perform from a headless process. Cline is a VS Code extension driven through
// the editor's extension API + webview UI; it ships NO standalone headless chat
// / task / browser CLI. Fabricating a "sent" / "queued" / "executed" status
// without doing the work is a BLUFF-001/003 violation, so these surface an
// honest error instead.
var errNoHeadlessCLI = fmt.Errorf("cline has no headless CLI: it is a VS Code extension driven via the editor extension API; use the \"open\" command to launch VS Code with the Cline extension and drive it interactively")

// chat is unsupported headlessly — Cline has no standalone chat CLI.
func (c *Cline) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}
	return nil, fmt.Errorf("chat: %w", errNoHeadlessCLI)
}

// task is unsupported headlessly — Cline has no standalone task-runner CLI.
func (c *Cline) task(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	task, _ := params["task"].(string)
	if task == "" {
		return nil, fmt.Errorf("task required")
	}
	return nil, fmt.Errorf("task: %w", errNoHeadlessCLI)
}

// browserAction is unsupported headlessly — Cline's browser automation runs
// inside the VS Code extension, not from a standalone CLI.
func (c *Cline) browserAction(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("browser: %w", errNoHeadlessCLI)
}

// IsAvailable checks if VS Code and Cline are available
func (c *Cline) IsAvailable() bool {
	_, err := exec.LookPath(c.config.VSCodePath)
	return err == nil
}

var _ agents.AgentIntegration = (*Cline)(nil)
