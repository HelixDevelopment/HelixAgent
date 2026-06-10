// Package getshitdone provides Get Shit Done agent integration.
// Get Shit Done: Task execution focused AI agent.
package getshitdone

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// GetShitDone provides Get Shit Done integration
type GetShitDone struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Mode string
}

// New creates a new Get Shit Done integration
func New() *GetShitDone {
	info := agents.AgentInfo{
		Type:        agents.TypeGetShitDone,
		Name:        "Get Shit Done",
		Description: "Task execution focused agent",
		Vendor:      "GSD",
		Version:     "1.0.0",
		Capabilities: []string{
			"task_execution",
			"productivity",
			"automation",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &GetShitDone{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Mode: "aggressive",
		},
	}
}

// Initialize initializes Get Shit Done
func (g *GetShitDone) Initialize(ctx context.Context, config interface{}) error {
	if err := g.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		g.config = cfg
	}

	return nil
}

// Execute executes a command
func (g *GetShitDone) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !g.IsStarted() {
		if err := g.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "execute":
		return g.execute(ctx, params)
	case "status":
		return g.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// errNoExecWired is returned by execute: no real Get-Shit-Done agent backend is
// wired, and no confirmed headless GSD CLI invocation was found in the
// 2026-06-10 currency/blocked-agent research. Echoing "Executed: <task>" without
// actually executing the task would be a BLUFF-001 violation — so execute
// returns an honest error rather than fabricating a result.
var errNoExecWired = fmt.Errorf(
	"get-shit-done execute is not wired to a real agent backend (research-blocked: " +
		"no confirmed headless GSD CLI); cannot report a result without performing the task")

// execute validates input then returns an honest not-wired error rather than an
// echoed "Executed: <task>" result (BLUFF-001).
func (g *GetShitDone) execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	task, _ := params["task"].(string)
	if task == "" {
		return nil, fmt.Errorf("task required")
	}

	return nil, errNoExecWired
}

// status returns status
func (g *GetShitDone) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": g.IsAvailable(),
		"mode":      g.config.Mode,
	}, nil
}

// IsAvailable checks availability
func (g *GetShitDone) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*GetShitDone)(nil)
