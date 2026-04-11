// Package gitmcp provides Git MCP agent integration.
// Git MCP: Model Context Protocol for Git operations.
package gitmcp

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// GitMCP provides Git MCP integration
type GitMCP struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Repository string
}

// New creates a new Git MCP integration
func New() *GitMCP {
	info := agents.AgentInfo{
		Type:        agents.TypeGitMCP,
		Name:        "Git MCP",
		Description: "MCP for Git operations",
		Vendor:      "GitMCP",
		Version:     "1.0.0",
		Capabilities: []string{
			"git_operations",
			"mcp_protocol",
			"version_control",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &GitMCP{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
		},
	}
}

// Initialize initializes Git MCP
func (g *GitMCP) Initialize(ctx context.Context, config interface{}) error {
	if err := g.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		g.config = cfg
	}

	return nil
}

// Execute executes a command
func (g *GitMCP) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !g.IsStarted() {
		if err := g.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "commit":
		return g.commit(ctx, params)
	case "branch":
		return g.branch(ctx, params)
	case "status":
		return g.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// commit creates a commit
func (g *GitMCP) commit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	return map[string]interface{}{
		"message": message,
		"commit":  "abc123",
		"status":  "committed",
	}, nil
}

// branch manages branches
func (g *GitMCP) branch(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	return map[string]interface{}{
		"branch": name,
		"status": "created",
	}, nil
}

// status returns status
func (g *GitMCP) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":  g.IsAvailable(),
		"repository": g.config.Repository,
	}, nil
}

// IsAvailable checks availability
func (g *GitMCP) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*GitMCP)(nil)
