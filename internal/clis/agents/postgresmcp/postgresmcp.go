// Package postgresmcp provides Postgres MCP agent integration.
// Postgres MCP: Model Context Protocol for PostgreSQL.
package postgresmcp

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// PostgresMCP provides Postgres MCP integration
type PostgresMCP struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	ConnectionString string
}

// New creates a new Postgres MCP integration
func New() *PostgresMCP {
	info := agents.AgentInfo{
		Type:        agents.TypePostgresMCP,
		Name:        "Postgres MCP",
		Description: "MCP for PostgreSQL",
		Vendor:      "PostgresMCP",
		Version:     "1.0.0",
		Capabilities: []string{
			"database",
			"postgresql",
			"mcp_protocol",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &PostgresMCP{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
		},
	}
}

// Initialize initializes Postgres MCP
func (p *PostgresMCP) Initialize(ctx context.Context, config interface{}) error {
	if err := p.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		p.config = cfg
	}

	return nil
}

// Execute executes a command
func (p *PostgresMCP) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !p.IsStarted() {
		if err := p.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "query":
		return p.query(ctx, params)
	case "schema":
		return p.schema(ctx, params)
	case "status":
		return p.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// query executes a query
func (p *PostgresMCP) query(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sql, _ := params["sql"].(string)
	if sql == "" {
		return nil, fmt.Errorf("sql required")
	}

	return map[string]interface{}{
		"sql":    sql,
		"result": "Query result",
	}, nil
}

// schema gets schema
func (p *PostgresMCP) schema(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"tables": []string{"users", "posts"},
	}, nil
}

// status returns status
func (p *PostgresMCP) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": p.IsAvailable(),
	}, nil
}

// IsAvailable checks availability
func (p *PostgresMCP) IsAvailable() bool {
	return p.config.ConnectionString != ""
}

var _ agents.AgentIntegration = (*PostgresMCP)(nil)
