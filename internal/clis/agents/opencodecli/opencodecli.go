// Package opencodecli provides Opencode CLI agent integration.
// Opencode CLI: open-source coding agent driven via the real `opencode` CLI
// non-interactively (`opencode run "<prompt>"`).
package opencodecli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// opencodeBinary is the Opencode CLI executable looked up on PATH.
// Overridable in tests via OPENCODE_BIN so a fake binary can be injected to
// prove real exec is wired (anti-bluff, BLUFF-001/003).
const opencodeBinary = "opencode"

func getOpencodeBinOverride() string { return os.Getenv("OPENCODE_BIN") }

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

// resolveOpencodeBinary locates the opencode CLI executable. Tests may inject a
// fake binary via OPENCODE_BIN (absolute path). Returns an honest error when the
// binary is not available — NEVER a fabricated success (BLUFF-001/003).
func (o *OpencodeCLI) resolveOpencodeBinary() (string, error) {
	if bin := getOpencodeBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("opencode binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(opencodeBinary)
	if err != nil {
		return "", fmt.Errorf("opencode CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runOpencode invokes `opencode run "<prompt>"` non-interactively and returns
// the real stdout. The model is forwarded via `--model` when configured (and
// not the placeholder "default").
func (o *OpencodeCLI) runOpencode(ctx context.Context, prompt string) (string, error) {
	bin, err := o.resolveOpencodeBinary()
	if err != nil {
		return "", err
	}

	args := []string{"run", prompt}
	if o.config != nil && o.config.Model != "" && o.config.Model != "default" {
		args = append(args, "--model", o.config.Model)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = o.GetWorkDir()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("opencode execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return strings.TrimSpace(string(out)), nil
}

// chat performs chat by exec-ing the real opencode CLI.
func (o *OpencodeCLI) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	response, err := o.runOpencode(ctx, message)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message":  message,
		"response": response,
	}, nil
}

// generate generates code by exec-ing the real opencode CLI.
func (o *OpencodeCLI) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	code, err := o.runOpencode(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"prompt": prompt,
		"code":   code,
	}, nil
}

// status returns status
func (o *OpencodeCLI) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": o.IsAvailable(),
		"model":     o.config.Model,
	}, nil
}

// IsAvailable checks availability — true only when the real opencode CLI
// (or an OPENCODE_BIN override) is resolvable on PATH.
func (o *OpencodeCLI) IsAvailable() bool {
	_, err := o.resolveOpencodeBinary()
	return err == nil
}

var _ agents.AgentIntegration = (*OpencodeCLI)(nil)
