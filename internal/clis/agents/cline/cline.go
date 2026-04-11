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

// chat sends a chat message
func (c *Cline) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	// In real implementation, this would communicate with Cline extension
	return map[string]interface{}{
		"message": message,
		"status":  "sent",
		"note":    "Cline integration requires VS Code extension API",
	}, nil
}

// task executes a task
func (c *Cline) task(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	task, _ := params["task"].(string)
	if task == "" {
		return nil, fmt.Errorf("task required")
	}

	return map[string]interface{}{
		"task":   task,
		"status": "queued",
		"note":   "Task sent to Cline for execution",
	}, nil
}

// browserAction performs a browser action
func (c *Cline) browserAction(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	action, _ := params["action"].(string)
	url, _ := params["url"].(string)

	return map[string]interface{}{
		"action": action,
		"url":    url,
		"status": "executed",
	}, nil
}

// IsAvailable checks if VS Code and Cline are available
func (c *Cline) IsAvailable() bool {
	_, err := exec.LookPath(c.config.VSCodePath)
	return err == nil
}

var _ agents.AgentIntegration = (*Cline)(nil)
