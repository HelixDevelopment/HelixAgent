// Package shai provides Shai agent integration.
// Shai (ovh/shai): a terminal coding agent / AI shell assistant. It exposes a
// headless mode in which a prompt is piped on stdin and the agent streams its
// work; see https://github.com/ovh/shai (README "headless mode").
package shai

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// shaiBinary is the Shai CLI executable looked up on PATH. Overridable in tests
// via the SHAI_BIN environment variable so a fake binary can be injected to
// prove real exec is wired (anti-bluff, BLUFF-001/003).
const shaiBinary = "shai"

// getShaiBinOverride returns the test-only shai binary override, if set.
func getShaiBinOverride() string { return os.Getenv("SHAI_BIN") }

// Shai provides Shai integration
type Shai struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Shell string
}

// New creates a new Shai integration
func New() *Shai {
	info := agents.AgentInfo{
		Type:        agents.TypeShai,
		Name:        "Shai",
		Description: "AI shell assistant",
		Vendor:      "Shai",
		Version:     "1.0.0",
		Capabilities: []string{
			"shell",
			"command_generation",
			"automation",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Shai{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Shell: "bash",
		},
	}
}

// Initialize initializes Shai
func (s *Shai) Initialize(ctx context.Context, config interface{}) error {
	if err := s.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		s.config = cfg
	}

	return nil
}

// Execute executes a command
func (s *Shai) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !s.IsStarted() {
		if err := s.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "generate":
		return s.generate(ctx, params)
	case "explain":
		return s.explain(ctx, params)
	case "status":
		return s.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveShaiBinary locates the shai CLI executable. Tests may inject a fake
// binary via SHAI_BIN (absolute path); otherwise the real `shai` command is
// resolved on PATH. Returns an honest error when the binary is not available —
// NEVER a fabricated success (BLUFF-001/003).
func (s *Shai) resolveShaiBinary() (string, error) {
	if bin := getShaiBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("shai binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(shaiBinary)
	if err != nil {
		return "", fmt.Errorf("shai CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runShai invokes the shai CLI in headless mode (prompt piped on stdin) and
// returns its captured output. Per the ovh/shai README, headless mode reads the
// prompt from stdin and streams events; we capture combined stdout+stderr as the
// real result. Returns an honest error when the binary is absent or fails.
func (s *Shai) runShai(ctx context.Context, prompt string) (string, error) {
	bin, err := s.resolveShaiBinary()
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, bin)
	cmd.Dir = s.GetWorkDir()
	cmd.Stdin = strings.NewReader(prompt)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("shai execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return strings.TrimSpace(string(out)), nil
}

// generate generates a shell command by exec-ing the real shai CLI.
func (s *Shai) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	description, _ := params["description"].(string)
	if description == "" {
		return nil, fmt.Errorf("description required")
	}

	command, err := s.runShai(ctx, "Generate a "+s.config.Shell+" shell command for: "+description)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"description": description,
		"command":     command,
		"shell":       s.config.Shell,
	}, nil
}

// explain explains a command by exec-ing the real shai CLI.
func (s *Shai) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	cmd, _ := params["command"].(string)
	if cmd == "" {
		return nil, fmt.Errorf("command required")
	}

	explanation, err := s.runShai(ctx, "Explain this shell command: "+cmd)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"command":     cmd,
		"explanation": explanation,
	}, nil
}

// status returns status
func (s *Shai) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": s.IsAvailable(),
		"shell":     s.config.Shell,
	}, nil
}

// IsAvailable reports whether the real shai CLI is resolvable on PATH (or via
// the SHAI_BIN override). Honest availability — not a hardcoded true.
func (s *Shai) IsAvailable() bool {
	_, err := s.resolveShaiBinary()
	return err == nil
}

var _ agents.AgentIntegration = (*Shai)(nil)
