// Package mobileagent provides Mobile Agent integration.
// Mobile Agent: AI for mobile development.
//
// There is no headless command-line "mobile agent" binary to drive, and no real
// model/build backend is wired here. Rather than fabricate generated code or a
// fake "built" status (the BLUFF-001/003 anti-pattern), the generate/build
// commands return an HONEST error.
package mobileagent

import (
	"context"
	"errors"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// ErrNoBackend is returned because no real mobile-agent backend (headless CLI
// or model API) is wired here. Returning a fabricated success instead would be
// a BLUFF-001/003 violation (CONST-035).
var ErrNoBackend = errors.New(
	"mobileagent: no real backend wired — no headless mobile-agent CLI is " +
		"available; refusing to fabricate a response")

// MobileAgent provides Mobile Agent integration
type MobileAgent struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Platform string // "ios", "android", "flutter"
}

// New creates a new Mobile Agent integration
func New() *MobileAgent {
	info := agents.AgentInfo{
		Type:        agents.TypeMobileAgent,
		Name:        "Mobile Agent",
		Description: "AI for mobile development",
		Vendor:      "MobileAgent",
		Version:     "1.0.0",
		Capabilities: []string{
			"mobile_dev",
			"ios",
			"android",
			"flutter",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &MobileAgent{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Platform: "flutter",
		},
	}
}

// Initialize initializes Mobile Agent
func (m *MobileAgent) Initialize(ctx context.Context, config interface{}) error {
	if err := m.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		m.config = cfg
	}

	return nil
}

// Execute executes a command
func (m *MobileAgent) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !m.IsStarted() {
		if err := m.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "generate":
		return m.generate(ctx, params)
	case "build":
		return m.build(ctx, params)
	case "status":
		return m.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// generate generates mobile code
func (m *MobileAgent) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return nil, ErrNoBackend
}

// build returns an honest error — no real build backend is wired.
func (m *MobileAgent) build(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, ErrNoBackend
}

// status returns status
func (m *MobileAgent) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": m.IsAvailable(),
		"platform":  m.config.Platform,
	}, nil
}

// IsAvailable checks availability
func (m *MobileAgent) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*MobileAgent)(nil)
