// Package snowcli provides Snow CLI agent integration.
// Snow CLI: Snowflake AI integration.
package snowcli

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// SnowCLI provides Snow CLI integration
type SnowCLI struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Account   string
	Warehouse string
}

// New creates a new Snow CLI integration
func New() *SnowCLI {
	info := agents.AgentInfo{
		Type:        agents.TypeSnowCLI,
		Name:        "Snow CLI",
		Description: "Snowflake AI integration",
		Vendor:      "Snowflake",
		Version:     "1.0.0",
		Capabilities: []string{
			"data_warehouse",
			"sql",
			"analytics",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &SnowCLI{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Warehouse: "COMPUTE_WH",
		},
	}
}

// Initialize initializes Snow CLI
func (s *SnowCLI) Initialize(ctx context.Context, config interface{}) error {
	if err := s.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		s.config = cfg
	}

	return nil
}

// Execute executes a command
func (s *SnowCLI) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !s.IsStarted() {
		if err := s.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "query":
		return s.query(ctx, params)
	case "status":
		return s.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// query executes SQL
func (s *SnowCLI) query(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sql, _ := params["sql"].(string)
	if sql == "" {
		return nil, fmt.Errorf("sql required")
	}

	return map[string]interface{}{
		"sql":       sql,
		"result":    "Query result",
		"warehouse": s.config.Warehouse,
	}, nil
}

// status returns status
func (s *SnowCLI) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": s.IsAvailable(),
		"account":   s.config.Account,
	}, nil
}

// IsAvailable checks availability
func (s *SnowCLI) IsAvailable() bool {
	return s.config.Account != ""
}

var _ agents.AgentIntegration = (*SnowCLI)(nil)
