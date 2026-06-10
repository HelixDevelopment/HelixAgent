// Package junie provides Junie agent integration.
// Junie: JetBrains AI coding assistant, driven through the headless `junie`
// CLI (`junie --non-interactive "<prompt>"`).
package junie

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// junieBinary is the Junie CLI executable looked up on PATH.
// Overridable in tests via the JUNIE_BIN environment variable so a fake
// binary can be injected to prove real exec is wired (anti-bluff).
const junieBinary = "junie"

// getJunieBinOverride returns the test-only junie binary override, if set.
func getJunieBinOverride() string { return os.Getenv("JUNIE_BIN") }

// Junie provides Junie integration
type Junie struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	IDE string
}

// New creates a new Junie integration
func New() *Junie {
	info := agents.AgentInfo{
		Type:        agents.TypeJunie,
		Name:        "Junie",
		Description: "JetBrains AI assistant",
		Vendor:      "JetBrains",
		Version:     "1.0.0",
		Capabilities: []string{
			"ide_integration",
			"code_completion",
			"code_generation",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Junie{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			IDE: "intellij",
		},
	}
}

// Initialize initializes Junie
func (j *Junie) Initialize(ctx context.Context, config interface{}) error {
	if err := j.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		j.config = cfg
	}

	return nil
}

// Execute executes a command
func (j *Junie) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !j.IsStarted() {
		if err := j.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "complete":
		return j.complete(ctx, params)
	case "generate":
		return j.generate(ctx, params)
	case "status":
		return j.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveJunieBinary locates the junie CLI executable. Tests may inject a fake
// binary via the JUNIE_BIN environment variable (absolute path); otherwise the
// real `junie` command is resolved on PATH. Returns an honest error when the
// binary is not available — NEVER a fabricated success (BLUFF-001).
func (j *Junie) resolveJunieBinary() (string, error) {
	if bin := getJunieBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("junie binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(junieBinary)
	if err != nil {
		return "", fmt.Errorf("junie CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runJunie invokes the junie CLI non-interactively and returns its trimmed
// stdout — the model's real output, never a template.
func (j *Junie) runJunie(ctx context.Context, prompt string) (string, error) {
	bin, err := j.resolveJunieBinary()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, bin, "--non-interactive", prompt)
	cmd.Dir = j.GetWorkDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("junie execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// complete generates a completion by exec-ing the real junie CLI.
func (j *Junie) complete(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	completion, err := j.runJunie(ctx, "Complete the following code:\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code":       code,
		"completion": completion,
	}, nil
}

// generate generates code by exec-ing the real junie CLI.
func (j *Junie) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	code, err := j.runJunie(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"prompt": prompt,
		"code":   code,
	}, nil
}

// status returns status
func (j *Junie) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": j.IsAvailable(),
		"ide":       j.config.IDE,
	}, nil
}

// IsAvailable checks availability — the junie CLI must be installed on PATH.
func (j *Junie) IsAvailable() bool {
	_, err := exec.LookPath(junieBinary)
	return err == nil
}

var _ agents.AgentIntegration = (*Junie)(nil)
