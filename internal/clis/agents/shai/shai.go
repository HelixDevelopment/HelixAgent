// Package shai provides Shai agent integration.
// Shai: AI shell assistant.
package shai

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

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

// generate generates shell command
func (s *Shai) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	description, _ := params["description"].(string)
	if description == "" {
		return nil, fmt.Errorf("description required")
	}

	return map[string]interface{}{
		"description": description,
		"command":     fmt.Sprintf("# Shai command for: %s", description),
		"shell":       s.config.Shell,
	}, nil
}

// explain explains command
func (s *Shai) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	cmd, _ := params["command"].(string)
	if cmd == "" {
		return nil, fmt.Errorf("command required")
	}

	return map[string]interface{}{
		"command":     cmd,
		"explanation": fmt.Sprintf("Explanation of: %s", cmd),
	}, nil
}

// status returns status
func (s *Shai) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": s.IsAvailable(),
		"shell":     s.config.Shell,
	}, nil
}

// IsAvailable checks availability
func (s *Shai) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*Shai)(nil)
