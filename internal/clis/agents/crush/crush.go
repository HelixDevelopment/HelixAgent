// Package crush provides Crush agent integration.
// Crush: Charm's terminal-based AI coding agent (charmbracelet/crush).
package crush

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// crushBinary is the Crush CLI executable looked up on PATH.
// Overridable in tests via the CRUSH_BIN environment variable so a fake
// binary can be injected to prove real exec is wired (anti-bluff §11.4.115).
const crushBinary = "crush"

// getCrushBinOverride returns the test-only crush binary override, if set.
func getCrushBinOverride() string { return os.Getenv("CRUSH_BIN") }

// Crush provides Crush integration
type Crush struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Model string
}

// New creates a new Crush integration
func New() *Crush {
	info := agents.AgentInfo{
		Type:        agents.TypeCrush,
		Name:        "Crush",
		Description: "Charm terminal-based AI coding agent",
		Vendor:      "Charm",
		Version:     "1.0.0",
		Capabilities: []string{
			"test_generation",
			"qa_automation",
			"bug_detection",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Crush{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "gpt-4",
		},
	}
}

// Initialize initializes Crush
func (c *Crush) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return nil
}

// Execute executes a command
func (c *Crush) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "test":
		return c.test(ctx, params)
	case "analyze":
		return c.analyze(ctx, params)
	case "status":
		return c.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveCrushBinary locates the crush CLI executable. Tests may inject a fake
// binary via the CRUSH_BIN environment variable (absolute path); otherwise the
// real `crush` command is resolved on PATH. Returns an honest error when the
// binary is not available — NEVER a fabricated success (BLUFF-001).
func (c *Crush) resolveCrushBinary() (string, error) {
	if bin := getCrushBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("crush binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(crushBinary)
	if err != nil {
		return "", fmt.Errorf("crush CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runCrush invokes the crush CLI non-interactively and returns its textual
// output. Per the 2026-06-10 currency research the non-interactive form is
// `crush run --quiet --yolo "<prompt>"` (single-turn → stdout; --quiet
// suppresses the spinner for clean stdout; --yolo auto-approves tool calls so
// the run is unattended; Crush has no JSON output flag — honest negative
// finding). The model is configured via a Crush config file, not a CLI flag,
// so it is not forwarded here.
func (c *Crush) runCrush(ctx context.Context, prompt string) (string, error) {
	bin, err := c.resolveCrushBinary()
	if err != nil {
		return "", err
	}

	args := []string{"run", "--quiet", "--yolo", prompt}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = c.GetWorkDir()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("crush execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return strings.TrimSpace(string(out)), nil
}

// test generates tests by exec-ing the real crush CLI.
func (c *Crush) test(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	tests, err := c.runCrush(ctx, "Generate tests for the following code:\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code":   code,
		"tests":  tests,
		"status": "tested",
	}, nil
}

// analyze analyzes for bugs by exec-ing the real crush CLI.
func (c *Crush) analyze(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	analysis, err := c.runCrush(ctx, "Analyze the following code for bugs:\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code":     code,
		"analysis": analysis,
	}, nil
}

// status returns status
func (c *Crush) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": c.IsAvailable(),
		"model":     c.config.Model,
	}, nil
}

// IsAvailable checks availability by resolving the real crush CLI on PATH.
func (c *Crush) IsAvailable() bool {
	_, err := c.resolveCrushBinary()
	return err == nil
}

var _ agents.AgentIntegration = (*Crush)(nil)
