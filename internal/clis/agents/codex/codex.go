// Package codex provides Codex CLI agent integration.
// Codex: OpenAI's official CLI coding agent.
package codex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Codex provides Codex CLI integration
type Codex struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Codex configuration
type Config struct {
	base.BaseConfig
	ApprovalMode    string // "suggest", "auto-edit", "full-auto"
	ContextWindow   int
	Editor          string
	FullAuto        bool
	ImageSupport    bool
	Quiet           bool
	ReasoningEffort string // "low", "medium", "high"
}

// New creates a new Codex integration
func New() *Codex {
	info := agents.AgentInfo{
		Type:        agents.TypeCodex,
		Name:        "Codex",
		Description: "OpenAI's official CLI coding agent",
		Vendor:      "OpenAI",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_generation",
			"code_editing",
			"terminal_commands",
			"file_operations",
			"image_understanding",
			"context_awareness",
			"multi_turn_chat",
		},
		IsEnabled: true,
		Priority:  1,
	}

	return &Codex{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				Model:     "codex",
				AutoStart: true,
			},
			ApprovalMode:    "suggest",
			ContextWindow:   128000,
			Editor:          "",
			FullAuto:        false,
			ImageSupport:    true,
			Quiet:           false,
			ReasoningEffort: "medium",
		},
	}
}

// Initialize initializes Codex
func (c *Codex) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	// Set API key if available
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		c.SetEnvVar("OPENAI_API_KEY", apiKey)
	}

	return nil
}

// Execute executes a Codex command
func (c *Codex) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "chat":
		return c.chat(ctx, params)
	case "run":
		return c.run(ctx, params)
	case "edit":
		return c.edit(ctx, params)
	case "explain":
		return c.explain(ctx, params)
	case "review":
		return c.review(ctx, params)
	case "test":
		return c.test(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// chat sends a chat message
func (c *Codex) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	args := c.buildArgs()
	args = append(args, "-p", message)

	output, err := c.ExecuteCommand(ctx, "codex", args...)
	if err != nil {
		return nil, fmt.Errorf("codex chat failed: %w\nOutput: %s", err, string(output))
	}

	return map[string]interface{}{
		"response": string(output),
		"success":  true,
	}, nil
}

// run runs Codex in interactive mode
func (c *Codex) run(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	args := c.buildArgs()

	if file, ok := params["file"].(string); ok {
		args = append(args, file)
	}

	output, err := c.ExecuteCommand(ctx, "codex", args...)
	if err != nil {
		return nil, fmt.Errorf("codex run failed: %w\nOutput: %s", err, string(output))
	}

	return map[string]interface{}{
		"response": string(output),
		"success":  true,
	}, nil
}

// edit performs a code edit
func (c *Codex) edit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	args := c.buildArgs()
	args = append(args, "-e", prompt)

	if file, ok := params["file"].(string); ok {
		args = append(args, file)
	}

	output, err := c.ExecuteCommand(ctx, "codex", args...)
	if err != nil {
		return nil, fmt.Errorf("codex edit failed: %w\nOutput: %s", err, string(output))
	}

	return map[string]interface{}{
		"response": string(output),
		"success":  true,
	}, nil
}

// explain explains code
func (c *Codex) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	file, _ := params["file"].(string)
	if file == "" {
		return nil, fmt.Errorf("file required")
	}

	args := c.buildArgs()
	args = append(args, "-x", file)

	output, err := c.ExecuteCommand(ctx, "codex", args...)
	if err != nil {
		return nil, fmt.Errorf("codex explain failed: %w", err)
	}

	return map[string]interface{}{
		"explanation": string(output),
		"success":     true,
	}, nil
}

// review reviews code
func (c *Codex) review(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	file, _ := params["file"].(string)
	if file == "" {
		return nil, fmt.Errorf("file required")
	}

	args := c.buildArgs()
	args = append(args, "-r", file)

	output, err := c.ExecuteCommand(ctx, "codex", args...)
	if err != nil {
		return nil, fmt.Errorf("codex review failed: %w", err)
	}

	return map[string]interface{}{
		"review":  string(output),
		"success": true,
	}, nil
}

// test generates tests
func (c *Codex) test(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	file, _ := params["file"].(string)
	if file == "" {
		return nil, fmt.Errorf("file required")
	}

	args := c.buildArgs()
	args = append(args, "-t", file)

	output, err := c.ExecuteCommand(ctx, "codex", args...)
	if err != nil {
		return nil, fmt.Errorf("codex test failed: %w", err)
	}

	return map[string]interface{}{
		"tests":   string(output),
		"success": true,
	}, nil
}

// buildArgs builds command-line arguments
func (c *Codex) buildArgs() []string {
	var args []string

	if c.config.Model != "" {
		args = append(args, "-m", c.config.Model)
	}

	if c.config.ApprovalMode != "" {
		args = append(args, "-a", c.config.ApprovalMode)
	}

	if c.config.ContextWindow > 0 {
		args = append(args, "-c", fmt.Sprintf("%d", c.config.ContextWindow))
	}

	if c.config.Editor != "" {
		args = append(args, "-e", c.config.Editor)
	}

	if c.config.FullAuto {
		args = append(args, "-y")
	}

	if c.config.Quiet {
		args = append(args, "-q")
	}

	if c.config.ReasoningEffort != "" {
		args = append(args, "-r", c.config.ReasoningEffort)
	}

	return args
}

// IsAvailable checks if Codex is installed
func (c *Codex) IsAvailable() bool {
	_, err := exec.LookPath("codex")
	return err == nil
}

// GetContextFiles returns context files for Codex
func (c *Codex) GetContextFiles(ctx context.Context, dir string) ([]string, error) {
	// Look for .codex/context.md or similar files
	contextFiles := []string{
		".codex/context.md",
		".codex/instructions.md",
		"CODEX.md",
		"codex.md",
	}

	var found []string
	for _, file := range contextFiles {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			found = append(found, path)
		}
	}

	return found, nil
}

// Ensure Codex implements AgentIntegration
var _ agents.AgentIntegration = (*Codex)(nil)
