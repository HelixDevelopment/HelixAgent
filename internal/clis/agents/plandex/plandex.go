// Package plandex provides Plandex agent integration.
// Plandex: AI task planner and executor driven via the real `plandex` CLI
// non-interactively (`plandex tell --no-build "<task>"`).
package plandex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// plandexBinary is the Plandex CLI executable looked up on PATH.
// Overridable in tests via PLANDEX_BIN so a fake binary can be injected to prove
// real exec is wired (anti-bluff, BLUFF-001/003).
const plandexBinary = "plandex"

func getPlandexBinOverride() string { return os.Getenv("PLANDEX_BIN") }

// Plandex provides Plandex integration
type Plandex struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Mode string
}

// New creates a new Plandex integration
func New() *Plandex {
	info := agents.AgentInfo{
		Type:        agents.TypePlandex,
		Name:        "Plandex",
		Description: "AI task planner and executor",
		Vendor:      "Plandex",
		Version:     "1.0.0",
		Capabilities: []string{
			"task_planning",
			"execution",
			"multi_step",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Plandex{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Mode: "auto",
		},
	}
}

// Initialize initializes Plandex
func (p *Plandex) Initialize(ctx context.Context, config interface{}) error {
	if err := p.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		p.config = cfg
	}

	return nil
}

// Execute executes a command
func (p *Plandex) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !p.IsStarted() {
		if err := p.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "plan":
		return p.plan(ctx, params)
	case "execute":
		return p.execute(ctx, params)
	case "status":
		return p.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolvePlandexBinary locates the plandex CLI executable. Tests may inject a
// fake binary via PLANDEX_BIN (absolute path). Returns an honest error when the
// binary is not available — NEVER a fabricated success (BLUFF-001/003).
func (p *Plandex) resolvePlandexBinary() (string, error) {
	if bin := getPlandexBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("plandex binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(plandexBinary)
	if err != nil {
		return "", fmt.Errorf("plandex CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runPlandex invokes the plandex CLI non-interactively with the given
// subcommand + task and returns the real stdout.
func (p *Plandex) runPlandex(ctx context.Context, subcommand, task string) (string, error) {
	bin, err := p.resolvePlandexBinary()
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, bin, subcommand, task)
	cmd.Dir = p.GetWorkDir()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("plandex execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return strings.TrimSpace(string(out)), nil
}

// plan creates a plan by exec-ing the real plandex CLI (`plandex tell`).
func (p *Plandex) plan(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	task, _ := params["task"].(string)
	if task == "" {
		return nil, fmt.Errorf("task required")
	}

	out, err := p.runPlandex(ctx, "tell", task)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"task": task,
		"plan": out,
	}, nil
}

// execute executes a task by exec-ing the real plandex CLI (`plandex apply`).
func (p *Plandex) execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	task, _ := params["task"].(string)
	if task == "" {
		return nil, fmt.Errorf("task required")
	}

	out, err := p.runPlandex(ctx, "tell", task)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"task":   task,
		"result": out,
		"mode":   p.config.Mode,
	}, nil
}

// status returns status
func (p *Plandex) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": p.IsAvailable(),
		"mode":      p.config.Mode,
	}, nil
}

// IsAvailable checks availability — true only when the real plandex CLI
// (or a PLANDEX_BIN override) is resolvable on PATH.
func (p *Plandex) IsAvailable() bool {
	_, err := p.resolvePlandexBinary()
	return err == nil
}

var _ agents.AgentIntegration = (*Plandex)(nil)
