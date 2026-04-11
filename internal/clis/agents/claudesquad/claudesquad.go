// Package claudesquad provides Claude Squad agent integration.
// Claude Squad: Multi-agent coordination system.
package claudesquad

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// ClaudeSquad provides Claude Squad integration
type ClaudeSquad struct {
	*base.BaseIntegration
	config *Config
	squads []Squad
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	MaxAgents int
}

// Squad represents a squad
type Squad struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Agents []string `json:"agents"`
	Status string   `json:"status"`
}

// New creates a new Claude Squad integration
func New() *ClaudeSquad {
	info := agents.AgentInfo{
		Type:        agents.TypeClaudeSquad,
		Name:        "Claude Squad",
		Description: "Multi-agent coordination",
		Vendor:      "Anthropic",
		Version:     "1.0.0",
		Capabilities: []string{
			"multi_agent",
			"coordination",
			"task_distribution",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &ClaudeSquad{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			MaxAgents: 5,
		},
		squads: make([]Squad, 0),
	}
}

// Initialize initializes Claude Squad
func (c *ClaudeSquad) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return c.loadSquads()
}

// loadSquads loads squads
func (c *ClaudeSquad) loadSquads() error {
	squadsPath := filepath.Join(c.GetWorkDir(), "squads.json")

	if _, err := os.Stat(squadsPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(squadsPath)
	if err != nil {
		return fmt.Errorf("read squads: %w", err)
	}

	return json.Unmarshal(data, &c.squads)
}

// saveSquads saves squads
func (c *ClaudeSquad) saveSquads() error {
	squadsPath := filepath.Join(c.GetWorkDir(), "squads.json")
	data, err := json.MarshalIndent(c.squads, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal squads: %w", err)
	}
	return os.WriteFile(squadsPath, data, 0644)
}

// Execute executes a command
func (c *ClaudeSquad) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "create":
		return c.create(ctx, params)
	case "add_agent":
		return c.addAgent(ctx, params)
	case "assign_task":
		return c.assignTask(ctx, params)
	case "list":
		return c.list(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// create creates a squad
func (c *ClaudeSquad) create(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	squad := Squad{
		ID:     fmt.Sprintf("squad-%d", len(c.squads)+1),
		Name:   name,
		Agents: []string{},
		Status: "created",
	}

	c.squads = append(c.squads, squad)

	if err := c.saveSquads(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"squad":  squad,
		"status": "created",
	}, nil
}

// addAgent adds an agent to squad
func (c *ClaudeSquad) addAgent(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	squadID, _ := params["squad_id"].(string)
	agent, _ := params["agent"].(string)

	if squadID == "" || agent == "" {
		return nil, fmt.Errorf("squad_id and agent required")
	}

	for i := range c.squads {
		if c.squads[i].ID == squadID {
			c.squads[i].Agents = append(c.squads[i].Agents, agent)
			break
		}
	}

	if err := c.saveSquads(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"squad_id": squadID,
		"agent":    agent,
		"status":   "added",
	}, nil
}

// assignTask assigns a task
func (c *ClaudeSquad) assignTask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	squadID, _ := params["squad_id"].(string)
	task, _ := params["task"].(string)

	if squadID == "" || task == "" {
		return nil, fmt.Errorf("squad_id and task required")
	}

	return map[string]interface{}{
		"squad_id": squadID,
		"task":     task,
		"status":   "assigned",
	}, nil
}

// list lists squads
func (c *ClaudeSquad) list(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"squads": c.squads,
		"count":  len(c.squads),
	}, nil
}

// IsAvailable checks availability
func (c *ClaudeSquad) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*ClaudeSquad)(nil)
