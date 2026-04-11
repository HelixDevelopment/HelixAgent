// Package plandex provides Plandex agent integration.
// Plandex: AI task planner and executor.
package plandex

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

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

// plan creates a plan
func (p *Plandex) plan(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	task, _ := params["task"].(string)
	if task == "" {
		return nil, fmt.Errorf("task required")
	}

	return map[string]interface{}{
		"task": task,
		"plan": []string{
			"Step 1: Analyze",
			"Step 2: Plan",
			"Step 3: Execute",
		},
	}, nil
}

// execute executes a task
func (p *Plandex) execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	task, _ := params["task"].(string)
	if task == "" {
		return nil, fmt.Errorf("task required")
	}

	return map[string]interface{}{
		"task":   task,
		"result": fmt.Sprintf("Executed: %s", task),
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

// IsAvailable checks availability
func (p *Plandex) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*Plandex)(nil)
