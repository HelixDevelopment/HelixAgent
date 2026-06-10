// Package postgresmcp provides Postgres MCP agent integration.
// Postgres MCP: Model Context Protocol server for PostgreSQL — NOT a coding
// agent.
//
// No real PostgreSQL connection or MCP server is wired here. Rather than
// fabricate a "Query result" string or a hardcoded list of tables (the
// BLUFF-001/003 anti-pattern), the query/schema commands return an HONEST error.
package postgresmcp

import (
	"context"
	"errors"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// ErrNoBackend is returned because no real PostgreSQL connection / MCP server is
// wired here. Returning a fabricated query result or hardcoded schema instead
// would be a BLUFF-001/003 violation (CONST-035).
var ErrNoBackend = errors.New(
	"postgresmcp: no real backend wired — no PostgreSQL connection / MCP server " +
		"is configured; refusing to fabricate query results or schema")

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

	return nil, ErrNoBackend
}

// schema gets schema — honest error, no real DB connection wired.
func (p *PostgresMCP) schema(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, ErrNoBackend
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
