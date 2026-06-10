// Package taskweaver provides Taskweaver agent integration.
// Taskweaver: Microsoft framework for conversational AI coding.
package taskweaver

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Taskweaver provides Taskweaver integration
type Taskweaver struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Model string
}

// New creates a new Taskweaver integration
func New() *Taskweaver {
	info := agents.AgentInfo{
		Type:        agents.TypeTaskweaver,
		Name:        "Taskweaver",
		Description: "Microsoft conversational AI coding",
		Vendor:      "Microsoft",
		Version:     "1.0.0",
		Capabilities: []string{
			"conversational",
			"code_generation",
			"planning",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Taskweaver{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "gpt-4",
		},
	}
}

// Initialize initializes Taskweaver
func (t *Taskweaver) Initialize(ctx context.Context, config interface{}) error {
	if err := t.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		t.config = cfg
	}

	return nil
}

// Execute executes a command
func (t *Taskweaver) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !t.IsStarted() {
		if err := t.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "chat":
		return t.chat(ctx, params)
	case "code":
		return t.code(ctx, params)
	case "status":
		return t.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// chat returns an HONEST error: TaskWeaver (microsoft/TaskWeaver) is a Python
// agent framework / library, not a headless binary on PATH. Driving it requires
// a Python runtime hosting the framework (its project/session API), which this
// Go integration does not embed. Rather than fabricate a reply (BLUFF-001/003),
// it returns an honest error. See https://github.com/microsoft/TaskWeaver .
func (t *Taskweaver) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}
	return nil, fmt.Errorf("taskweaver has no headless CLI: it is a Python agent framework driven through its Python session API, not embedded here — refusing to fabricate a reply (BLUFF-001)")
}

// code returns an HONEST error for the same reason as chat: there is no
// TaskWeaver binary to exec from this Go integration. Refusing to fabricate
// code (BLUFF-001).
func (t *Taskweaver) code(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}
	return nil, fmt.Errorf("taskweaver has no headless CLI: it is a Python agent framework driven through its Python session API, not embedded here — refusing to fabricate code (BLUFF-001)")
}

// status returns status
func (t *Taskweaver) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": t.IsAvailable(),
		"model":     t.config.Model,
	}, nil
}

// IsAvailable checks availability
func (t *Taskweaver) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*Taskweaver)(nil)
