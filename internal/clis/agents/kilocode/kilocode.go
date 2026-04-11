// Package kilocode provides Kilo Code agent integration.
// Kilo Code: Lightweight AI code assistant.
package kilocode

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// KiloCode provides Kilo Code integration
type KiloCode struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Model string
}

// New creates a new Kilo Code integration
func New() *KiloCode {
	info := agents.AgentInfo{
		Type:        agents.TypeKiloCode,
		Name:        "Kilo Code",
		Description: "Lightweight AI code assistant",
		Vendor:      "KiloCode",
		Version:     "1.0.0",
		Capabilities: []string{
			"lightweight",
			"code_completion",
			"fast",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &KiloCode{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "gpt-3.5-turbo",
		},
	}
}

// Initialize initializes Kilo Code
func (k *KiloCode) Initialize(ctx context.Context, config interface{}) error {
	if err := k.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		k.config = cfg
	}

	return nil
}

// Execute executes a command
func (k *KiloCode) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !k.IsStarted() {
		if err := k.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "complete":
		return k.complete(ctx, params)
	case "status":
		return k.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// complete generates completion
func (k *KiloCode) complete(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prefix, _ := params["prefix"].(string)

	return map[string]interface{}{
		"prefix":     prefix,
		"completion": "// KiloCode completion",
		"model":      k.config.Model,
	}, nil
}

// status returns status
func (k *KiloCode) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": k.IsAvailable(),
		"model":     k.config.Model,
	}, nil
}

// IsAvailable checks availability
func (k *KiloCode) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*KiloCode)(nil)
