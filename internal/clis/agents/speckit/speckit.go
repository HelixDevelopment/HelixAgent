// Package speckit provides SpecKit agent integration.
// SpecKit: AI specification and documentation tool.
package speckit

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// SpecKit provides SpecKit integration
type SpecKit struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Format string
}

// New creates a new SpecKit integration
func New() *SpecKit {
	info := agents.AgentInfo{
		Type:        agents.TypeSpecKit,
		Name:        "SpecKit",
		Description: "AI specification tool",
		Vendor:      "SpecKit",
		Version:     "1.0.0",
		Capabilities: []string{
			"specifications",
			"documentation",
			"requirements",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &SpecKit{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Format: "markdown",
		},
	}
}

// Initialize initializes SpecKit
func (s *SpecKit) Initialize(ctx context.Context, config interface{}) error {
	if err := s.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		s.config = cfg
	}

	return nil
}

// Execute executes a command
func (s *SpecKit) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !s.IsStarted() {
		if err := s.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "generate":
		return s.generate(ctx, params)
	case "status":
		return s.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// generate generates spec
func (s *SpecKit) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	requirement, _ := params["requirement"].(string)
	if requirement == "" {
		return nil, fmt.Errorf("requirement required")
	}

	return map[string]interface{}{
		"requirement": requirement,
		"spec":        fmt.Sprintf("# Spec for: %s", requirement),
		"format":      s.config.Format,
	}, nil
}

// status returns status
func (s *SpecKit) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": s.IsAvailable(),
		"format":    s.config.Format,
	}, nil
}

// IsAvailable checks availability
func (s *SpecKit) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*SpecKit)(nil)
