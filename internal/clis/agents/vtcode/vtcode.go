// Package vtcode provides VT Code agent integration.
// VT Code (vinhnx/vtcode): an open-source terminal coding agent. It exposes a
// non-interactive `vtcode ask "<prompt>"` command that prints the reply to
// stdout (metadata to stderr). See https://github.com/vinhnx/vtcode .
package vtcode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// vtcodeBinary is the VT Code CLI executable looked up on PATH. Overridable in
// tests via VTCODE_BIN so a fake binary can be injected to prove real exec is
// wired (anti-bluff, BLUFF-001).
const vtcodeBinary = "vtcode"

// getVtcodeBinOverride returns the test-only vtcode binary override, if set.
func getVtcodeBinOverride() string { return os.Getenv("VTCODE_BIN") }

// VTCode provides VT Code integration
type VTCode struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Language string
}

// New creates a new VT Code integration
func New() *VTCode {
	info := agents.AgentInfo{
		Type:        agents.TypeVtcode,
		Name:        "VT Code",
		Description: "Terminal coding agent",
		Vendor:      "vinhnx",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_generation",
			"chat",
			"shell_safety",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &VTCode{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Language: "en",
		},
	}
}

// Initialize initializes VT Code
func (v *VTCode) Initialize(ctx context.Context, config interface{}) error {
	if err := v.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		v.config = cfg
	}

	return nil
}

// Execute executes a command
func (v *VTCode) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !v.IsStarted() {
		if err := v.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "ask":
		return v.ask(ctx, params)
	case "status":
		return v.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveVtcodeBinary locates the vtcode CLI executable. Tests may inject a fake
// binary via VTCODE_BIN (absolute path); otherwise the real `vtcode` command is
// resolved on PATH. Returns an honest error when absent — NEVER a fabricated
// success (BLUFF-001).
func (v *VTCode) resolveVtcodeBinary() (string, error) {
	if bin := getVtcodeBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("vtcode binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(vtcodeBinary)
	if err != nil {
		return "", fmt.Errorf("vtcode CLI not found on PATH: %w", err)
	}
	return path, nil
}

// ask sends a prompt to the real `vtcode ask "<prompt>"` command and returns its
// real stdout reply. Returns an honest error when the binary is absent or fails
// — NEVER a fabricated "// Voice transcribed code" placeholder (BLUFF-001).
func (v *VTCode) ask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	bin, err := v.resolveVtcodeBinary()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, bin, "ask", prompt)
	cmd.Dir = v.GetWorkDir()

	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, fmt.Errorf("vtcode ask execution failed: %w (stderr: %s)", err, stderr)
	}

	return map[string]interface{}{
		"prompt":   prompt,
		"reply":    strings.TrimSpace(string(out)),
		"language": v.config.Language,
	}, nil
}

// status returns status
func (v *VTCode) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": v.IsAvailable(),
		"language":  v.config.Language,
	}, nil
}

// IsAvailable reports whether the real vtcode CLI is resolvable on PATH (or via
// the VTCODE_BIN override). Honest availability — not a hardcoded true.
func (v *VTCode) IsAvailable() bool {
	_, err := v.resolveVtcodeBinary()
	return err == nil
}

var _ agents.AgentIntegration = (*VTCode)(nil)
