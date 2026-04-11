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

// chat performs chat
func (t *Taskweaver) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	return map[string]interface{}{
		"message":  message,
		"response": fmt.Sprintf("Taskweaver: %s", message),
		"model":    t.config.Model,
	}, nil
}

// code generates code
func (t *Taskweaver) code(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return map[string]interface{}{
		"prompt": prompt,
		"code":   fmt.Sprintf("// Taskweaver\n// %s", prompt),
		"model":  t.config.Model,
	}, nil
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
