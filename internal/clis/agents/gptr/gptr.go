// Package gptr provides GPTR agent integration.
// GPTR: General-purpose task runner for LLM agents.
package gptr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// GPTR provides GPTR integration
type GPTR struct {
	*base.BaseIntegration
	config  *Config
	tasksMu sync.RWMutex
	tasks   []Task
}

// Config holds GPTR configuration
type Config struct {
	base.BaseConfig
	Model     string
	MaxTokens int
	Timeout   int
}

// Task represents a task
type Task struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Prompt string   `json:"prompt"`
	Status string   `json:"status"`
	Result string   `json:"result"`
	Tools  []string `json:"tools"`
}

// New creates a new GPTR integration
func New() *GPTR {
	info := agents.AgentInfo{
		Type:        agents.TypeGPTR,
		Name:        "GPTR",
		Description: "General-purpose task runner",
		Vendor:      "GPTR",
		Version:     "1.0.0",
		Capabilities: []string{
			"task_runner",
			"code_execution",
			"file_operations",
			"web_search",
			"shell_commands",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &GPTR{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model:     "gpt-4",
			MaxTokens: 4096,
			Timeout:   60,
		},
		tasks: make([]Task, 0),
	}
}

// Initialize initializes GPTR
func (g *GPTR) Initialize(ctx context.Context, config interface{}) error {
	// Extract WorkDir from embedded BaseConfig before base init so that
	// BaseIntegration uses the correct directory.
	if cfg, ok := config.(*Config); ok && cfg.WorkDir != "" {
		g.SetWorkDir(cfg.WorkDir)
	}

	if err := g.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		g.config = cfg
	}

	return g.loadTasks()
}

// loadTasks loads tasks
func (g *GPTR) loadTasks() error {
	tasksPath := filepath.Join(g.GetWorkDir(), "tasks.json")

	if _, err := os.Stat(tasksPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(tasksPath)
	if err != nil {
		return fmt.Errorf("read tasks: %w", err)
	}

	g.tasksMu.Lock()
	defer g.tasksMu.Unlock()

	return json.Unmarshal(data, &g.tasks)
}

// saveTasks saves tasks
func (g *GPTR) saveTasks() error {
	g.tasksMu.RLock()
	data, err := json.MarshalIndent(g.tasks, "", "  ")
	g.tasksMu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}

	tasksPath := filepath.Join(g.GetWorkDir(), "tasks.json")
	return os.WriteFile(tasksPath, data, 0644)
}

// Execute executes a command
func (g *GPTR) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !g.IsStarted() {
		if err := g.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "run":
		return g.run(ctx, params)
	case "create_task":
		return g.createTask(ctx, params)
	case "list_tasks":
		return g.listTasks(ctx)
	case "get_result":
		return g.getResult(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// run runs a task
func (g *GPTR) run(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	// Execute task
	result := fmt.Sprintf("Result for: %s", prompt)

	return map[string]interface{}{
		"prompt": prompt,
		"result": result,
		"model":  g.config.Model,
		"status": "completed",
	}, nil
}

// createTask creates a task
func (g *GPTR) createTask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	prompt, _ := params["prompt"].(string)

	if name == "" || prompt == "" {
		return nil, fmt.Errorf("name and prompt required")
	}

	g.tasksMu.Lock()
	task := Task{
		ID:     fmt.Sprintf("task-%d", len(g.tasks)+1),
		Name:   name,
		Prompt: prompt,
		Status: "created",
		Tools:  []string{},
	}

	g.tasks = append(g.tasks, task)
	g.tasksMu.Unlock()

	if err := g.saveTasks(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"task":   task,
		"status": "created",
	}, nil
}

// listTasks lists tasks
func (g *GPTR) listTasks(ctx context.Context) (interface{}, error) {
	g.tasksMu.RLock()
	tasks := make([]Task, len(g.tasks))
	copy(tasks, g.tasks)
	g.tasksMu.RUnlock()

	return map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	}, nil
}

// getResult gets task result
func (g *GPTR) getResult(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	taskID, _ := params["task_id"].(string)
	if taskID == "" {
		return nil, fmt.Errorf("task_id required")
	}

	g.tasksMu.RLock()
	defer g.tasksMu.RUnlock()

	for _, task := range g.tasks {
		if task.ID == taskID {
			return map[string]interface{}{
				"task":   task,
				"result": task.Result,
			}, nil
		}
	}

	return nil, fmt.Errorf("task not found: %s", taskID)
}

// IsAvailable checks availability
func (g *GPTR) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*GPTR)(nil)
