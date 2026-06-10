// Package smolagents provides Smolagents integration.
// Smolagents: Lightweight agent framework from Hugging Face.
package smolagents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Smolagents provides Smolagents integration
type Smolagents struct {
	*base.BaseIntegration
	config *Config
	agents []Agent
}

// Config holds Smolagents configuration
type Config struct {
	base.BaseConfig
	Model    string
	Tools    []string
	MaxSteps int
}

// Agent represents an agent
type Agent struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Type   string   `json:"type"` // "tool_calling", "code_agent"
	Tools  []string `json:"tools"`
	Status string   `json:"status"`
}

// New creates a new Smolagents integration
func New() *Smolagents {
	info := agents.AgentInfo{
		Type:        agents.TypeSmolagents,
		Name:        "Smolagents",
		Description: "Lightweight agent framework",
		Vendor:      "Hugging Face",
		Version:     "1.0.0",
		Capabilities: []string{
			"tool_calling",
			"code_generation",
			"multi_step",
			"lightweight",
			"open_source",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Smolagents{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model:    "huggingface/meta-llama/Llama-3.1-70B-Instruct",
			Tools:    []string{"web_search", "python_interpreter"},
			MaxSteps: 10,
		},
		agents: make([]Agent, 0),
	}
}

// Initialize initializes Smolagents
func (s *Smolagents) Initialize(ctx context.Context, config interface{}) error {
	if err := s.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		s.config = cfg
	}

	return s.loadAgents()
}

// loadAgents loads agents
func (s *Smolagents) loadAgents() error {
	agentsPath := filepath.Join(s.GetWorkDir(), "agents.json")

	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(agentsPath)
	if err != nil {
		return fmt.Errorf("read agents: %w", err)
	}

	return json.Unmarshal(data, &s.agents)
}

// saveAgents saves agents
func (s *Smolagents) saveAgents() error {
	agentsPath := filepath.Join(s.GetWorkDir(), "agents.json")
	data, err := json.MarshalIndent(s.agents, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal agents: %w", err)
	}
	return os.WriteFile(agentsPath, data, 0644)
}

// Execute executes a command
func (s *Smolagents) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !s.IsStarted() {
		if err := s.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "create_agent":
		return s.createAgent(ctx, params)
	case "run":
		return s.run(ctx, params)
	case "list_agents":
		return s.listAgents(ctx)
	case "import_tool":
		return s.importTool(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// createAgent creates a new agent
func (s *Smolagents) createAgent(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	agentType, _ := params["type"].(string)
	if agentType == "" {
		agentType = "tool_calling"
	}

	tools, _ := params["tools"].([]string)
	if tools == nil {
		tools = s.config.Tools
	}

	agent := Agent{
		ID:     fmt.Sprintf("agent-%d", len(s.agents)+1),
		Name:   name,
		Type:   agentType,
		Tools:  tools,
		Status: "created",
	}

	s.agents = append(s.agents, agent)

	if err := s.saveAgents(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"agent":  agent,
		"status": "created",
	}, nil
}

// run runs an agent
func (s *Smolagents) run(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	agentID, _ := params["agent_id"].(string)
	if agentID == "" {
		return nil, fmt.Errorf("agent_id required")
	}

	task, _ := params["task"].(string)
	if task == "" {
		return nil, fmt.Errorf("task required")
	}

	var found bool
	for i := range s.agents {
		if s.agents[i].ID == agentID {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	// HONEST error: smolagents (huggingface/smolagents) is a Python agent library,
	// not a headless binary on PATH. Running an agent requires a Python runtime
	// hosting the framework (CodeAgent / ToolCallingAgent + a real model backend).
	// The previous implementation returned fixed "Analyze/Plan/Execute" steps and
	// "Task completed successfully" without running anything — a BLUFF-001 /
	// CONST-035 false-success. Refusing to fabricate a run transcript.
	// See https://github.com/huggingface/smolagents .
	return nil, fmt.Errorf("smolagents agent %q not run: it is a Python agent library driven through a Python runtime, not embedded here — refusing to fabricate a run transcript (BLUFF-001)", agentID)
}

// listAgents lists agents
func (s *Smolagents) listAgents(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"agents": s.agents,
		"count":  len(s.agents),
	}, nil
}

// importTool imports a tool
func (s *Smolagents) importTool(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	toolName, _ := params["tool"].(string)
	if toolName == "" {
		return nil, fmt.Errorf("tool required")
	}

	s.config.Tools = append(s.config.Tools, toolName)

	return map[string]interface{}{
		"tool":   toolName,
		"tools":  s.config.Tools,
		"status": "imported",
	}, nil
}

// IsAvailable checks availability
func (s *Smolagents) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*Smolagents)(nil)
