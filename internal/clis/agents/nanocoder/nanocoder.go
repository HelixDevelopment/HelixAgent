// Package nanocoder provides Nanocoder agent integration.
// Nanocoder: local-first AI coding agent CLI (the `nanocoder` binary).
package nanocoder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// nanocoderBinary is the Nanocoder CLI executable looked up on PATH.
// Overridable in tests via NANOCODER_BIN so a fake binary can be injected to
// prove real exec is wired (anti-bluff, BLUFF-001/003).
const nanocoderBinary = "nanocoder"

func getNanocoderBinOverride() string { return os.Getenv("NANOCODER_BIN") }

// Nanocoder provides Nanocoder integration
type Nanocoder struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Model string
}

// New creates a new Nanocoder integration
func New() *Nanocoder {
	info := agents.AgentInfo{
		Type:        agents.TypeNanocoder,
		Name:        "Nanocoder",
		Description: "Minimalist AI code generator",
		Vendor:      "Nanocoder",
		Version:     "1.0.0",
		Capabilities: []string{
			"minimal",
			"fast",
			"code_generation",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Nanocoder{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "nano",
		},
	}
}

// Initialize initializes Nanocoder
func (n *Nanocoder) Initialize(ctx context.Context, config interface{}) error {
	if err := n.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		n.config = cfg
	}

	return nil
}

// Execute executes a command
func (n *Nanocoder) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !n.IsStarted() {
		if err := n.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "generate":
		return n.generate(ctx, params)
	case "status":
		return n.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveNanocoderBinary locates the nanocoder CLI executable. Tests may inject
// a fake binary via NANOCODER_BIN (absolute path). Returns an honest error when
// the binary is not available — NEVER a fabricated success (BLUFF-001/003).
func (n *Nanocoder) resolveNanocoderBinary() (string, error) {
	if bin := getNanocoderBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("nanocoder binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(nanocoderBinary)
	if err != nil {
		return "", fmt.Errorf("nanocoder CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runNanocoder invokes the nanocoder CLI non-interactively with a prompt and
// returns its real stdout. The non-interactive form passes the prompt via `-p`;
// the model is forwarded via `--model` when configured.
func (n *Nanocoder) runNanocoder(ctx context.Context, prompt string) (string, error) {
	bin, err := n.resolveNanocoderBinary()
	if err != nil {
		return "", err
	}

	args := []string{"-p", prompt}
	if n.config != nil && n.config.Model != "" {
		args = append(args, "--model", n.config.Model)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = n.GetWorkDir()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("nanocoder execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return strings.TrimSpace(string(out)), nil
}

// generate generates code by exec-ing the real nanocoder CLI.
func (n *Nanocoder) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	code, err := n.runNanocoder(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"prompt": prompt,
		"code":   code,
		"model":  n.config.Model,
	}, nil
}

// status returns status
func (n *Nanocoder) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": n.IsAvailable(),
		"model":     n.config.Model,
	}, nil
}

// IsAvailable checks availability — true only when the real nanocoder CLI
// (or a NANOCODER_BIN override) is resolvable on PATH.
func (n *Nanocoder) IsAvailable() bool {
	_, err := n.resolveNanocoderBinary()
	return err == nil
}

var _ agents.AgentIntegration = (*Nanocoder)(nil)
