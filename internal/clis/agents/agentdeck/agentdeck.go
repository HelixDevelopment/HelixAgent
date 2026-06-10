// Package agentdeck provides Agent Deck CLI integration.
// Agent Deck: Multi-agent orchestration platform for coordinating multiple AI agents.
package agentdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// AgentDeck provides Agent Deck CLI integration
type AgentDeck struct {
	*base.BaseIntegration
	config *Config
	agents []DeckAgent
	tasks  []Task
}

// Config holds Agent Deck configuration
type Config struct {
	base.BaseConfig
	OrchestrationMode string
	MaxConcurrent     int
	Timeout           int
	WorkspaceDir      string
}

// DeckAgent represents an agent in the deck
type DeckAgent struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Config   map[string]interface{} `json:"config"`
	Status   string                 `json:"status"`
	Priority int                    `json:"priority"`
}

// Task represents a task to be executed
type Task struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	AgentIDs    []string `json:"agent_ids"`
	Status      string   `json:"status"`
	Result      string   `json:"result"`
	DependsOn   []string `json:"depends_on"`
}

// New creates a new Agent Deck integration
func New() *AgentDeck {
	info := agents.AgentInfo{
		Type:        agents.TypeAgentDeck,
		Name:        "Agent Deck",
		Description: "Multi-agent orchestration platform",
		Vendor:      "Agent Deck",
		Version:     "1.0.0",
		Capabilities: []string{
			"multi_agent_orchestration",
			"agent_coordination",
			"task_decomposition",
			"parallel_execution",
			"hierarchical_planning",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &AgentDeck{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			OrchestrationMode: "parallel",
			MaxConcurrent:     5,
			Timeout:           300,
		},
		agents: make([]DeckAgent, 0),
		tasks:  make([]Task, 0),
	}
}

// Initialize initializes Agent Deck
func (a *AgentDeck) Initialize(ctx context.Context, config interface{}) error {
	if err := a.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		a.config = cfg
	}

	if a.config.WorkspaceDir == "" {
		a.config.WorkspaceDir = a.GetWorkDir()
	}

	return a.loadConfig()
}

// loadConfig loads agent configuration
func (a *AgentDeck) loadConfig() error {
	configPath := filepath.Join(a.config.WorkspaceDir, "deck.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return a.createDefaultConfig()
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var config struct {
		Agents []DeckAgent `json:"agents"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	a.agents = config.Agents
	return nil
}

// createDefaultConfig creates default configuration
func (a *AgentDeck) createDefaultConfig() error {
	a.agents = []DeckAgent{
		{ID: "architect-1", Name: "System Architect", Type: "architect", Config: map[string]interface{}{"focus": "system_design"}, Status: "idle"},
		{ID: "coder-1", Name: "Senior Developer", Type: "coder", Config: map[string]interface{}{"language": "go"}, Status: "idle"},
		{ID: "reviewer-1", Name: "Code Reviewer", Type: "reviewer", Config: map[string]interface{}{"strictness": "high"}, Status: "idle"},
		{ID: "tester-1", Name: "QA Engineer", Type: "tester", Config: map[string]interface{}{"coverage": "comprehensive"}, Status: "idle"},
	}
	return a.saveConfig()
}

// saveConfig saves configuration
func (a *AgentDeck) saveConfig() error {
	configPath := filepath.Join(a.config.WorkspaceDir, "deck.json")
	data, err := json.MarshalIndent(struct {
		Agents []DeckAgent `json:"agents"`
	}{Agents: a.agents}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(configPath, data, 0644)
}

// Execute executes a command
func (a *AgentDeck) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !a.IsStarted() {
		if err := a.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "orchestrate":
		return a.orchestrate(ctx, params)
	case "add_agent":
		return a.addAgent(ctx, params)
	case "remove_agent":
		return a.removeAgent(ctx, params)
	case "list_agents":
		return a.listAgents(ctx)
	case "assign_task":
		return a.assignTask(ctx, params)
	case "status":
		return a.getStatus(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// orchestrate orchestrates agents
func (a *AgentDeck) orchestrate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	task, _ := params["task"].(string)
	if task == "" {
		return nil, fmt.Errorf("task description required")
	}

	mode, _ := params["mode"].(string)
	if mode == "" {
		mode = a.config.OrchestrationMode
	}

	plan := a.createOrchestrationPlan(task, mode)

	// Anti-bluff (BLUFF-001, D-18): no agent is actually executed here — this
	// builds the execution plan only. Reporting "completed" would falsely
	// claim a multi-agent run finished. The honest state is that the plan was
	// PLANNED; the caller drives execution via assign_task / a real runner.
	return map[string]interface{}{
		"task":   task,
		"mode":   mode,
		"plan":   plan,
		"status": "planned",
	}, nil
}

// createOrchestrationPlan creates execution plan
func (a *AgentDeck) createOrchestrationPlan(task string, mode string) map[string]interface{} {
	return map[string]interface{}{
		"original_task": task,
		"mode":          mode,
		"steps": []map[string]interface{}{
			{"step": 1, "agent": "architect", "action": "analyze"},
			{"step": 2, "agent": "coder", "action": "implement"},
			{"step": 3, "agent": "reviewer", "action": "review"},
			{"step": 4, "agent": "tester", "action": "test"},
		},
	}
}

// addAgent adds an agent
func (a *AgentDeck) addAgent(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	agentType, _ := params["type"].(string)

	if name == "" || agentType == "" {
		return nil, fmt.Errorf("name and type required")
	}

	agent := DeckAgent{
		ID:     fmt.Sprintf("%s-%d", agentType, len(a.agents)+1),
		Name:   name,
		Type:   agentType,
		Config: params,
		Status: "idle",
	}

	a.agents = append(a.agents, agent)

	if err := a.saveConfig(); err != nil {
		return nil, err
	}

	return map[string]interface{}{"agent": agent, "status": "added"}, nil
}

// removeAgent removes an agent
func (a *AgentDeck) removeAgent(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	agentID, _ := params["id"].(string)
	if agentID == "" {
		return nil, fmt.Errorf("agent id required")
	}

	for i, agent := range a.agents {
		if agent.ID == agentID {
			a.agents = append(a.agents[:i], a.agents[i+1:]...)
			if err := a.saveConfig(); err != nil {
				return nil, err
			}
			return map[string]interface{}{"agent_id": agentID, "status": "removed"}, nil
		}
	}

	return nil, fmt.Errorf("agent not found: %s", agentID)
}

// listAgents lists agents
func (a *AgentDeck) listAgents(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{"agents": a.agents, "count": len(a.agents)}, nil
}

// assignTask assigns a task
func (a *AgentDeck) assignTask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	description, _ := params["description"].(string)
	agentIDs, _ := params["agent_ids"].([]string)

	if description == "" || len(agentIDs) == 0 {
		return nil, fmt.Errorf("description and agent_ids required")
	}

	task := Task{
		ID:          fmt.Sprintf("task-%d", len(a.tasks)+1),
		Description: description,
		AgentIDs:    agentIDs,
		Status:      "pending",
	}

	a.tasks = append(a.tasks, task)

	return map[string]interface{}{"task": task, "status": "assigned"}, nil
}

// getStatus returns status
func (a *AgentDeck) getStatus(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"agents":             len(a.agents),
		"tasks":              len(a.tasks),
		"orchestration_mode": a.config.OrchestrationMode,
	}, nil
}

// IsAvailable checks availability
func (a *AgentDeck) IsAvailable() bool {
	_, err := exec.LookPath("agent-deck")
	return err == nil
}

var _ agents.AgentIntegration = (*AgentDeck)(nil)
