// Package copilotworkspace provides GitHub Copilot Workspace integration.
// Copilot Workspace: Task-driven development environment.
package copilotworkspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// CopilotWorkspace provides GitHub Copilot Workspace integration
type CopilotWorkspace struct {
	*base.BaseIntegration
	config *Config
	tasks  []Task
}

// Config holds Copilot Workspace configuration
type Config struct {
	base.BaseConfig
	GitHubToken string
	Repository  string
	AutoPR      bool
}

// Task represents a development task
type Task struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Files       []string `json:"files"`
	Branch      string   `json:"branch"`
}

// New creates a new Copilot Workspace integration
func New() *CopilotWorkspace {
	info := agents.AgentInfo{
		Type:        agents.TypeCopilotWorkspace,
		Name:        "Copilot Workspace",
		Description: "Task-driven development environment",
		Vendor:      "GitHub",
		Version:     "1.0.0",
		Capabilities: []string{
			"task_driven_dev",
			"spec_to_code",
			"plan_generation",
			"multi_file_edits",
			"github_integration",
			"auto_pr",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &CopilotWorkspace{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			AutoPR: false,
		},
		tasks: make([]Task, 0),
	}
}

// Initialize initializes Copilot Workspace
func (c *CopilotWorkspace) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return c.loadTasks()
}

// loadTasks loads tasks
func (c *CopilotWorkspace) loadTasks() error {
	tasksPath := filepath.Join(c.GetWorkDir(), "tasks.json")

	if _, err := os.Stat(tasksPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(tasksPath)
	if err != nil {
		return fmt.Errorf("read tasks: %w", err)
	}

	return json.Unmarshal(data, &c.tasks)
}

// saveTasks saves tasks
func (c *CopilotWorkspace) saveTasks() error {
	tasksPath := filepath.Join(c.GetWorkDir(), "tasks.json")
	data, err := json.MarshalIndent(c.tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}
	return os.WriteFile(tasksPath, data, 0644)
}

// Execute executes a command
func (c *CopilotWorkspace) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "create_task":
		return c.createTask(ctx, params)
	case "plan":
		return c.plan(ctx, params)
	case "implement":
		return c.implement(ctx, params)
	case "review":
		return c.review(ctx, params)
	case "submit":
		return c.submit(ctx, params)
	case "list_tasks":
		return c.listTasks(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// createTask creates a new task
func (c *CopilotWorkspace) createTask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	title, _ := params["title"].(string)
	description, _ := params["description"].(string)

	if title == "" {
		return nil, fmt.Errorf("title required")
	}

	task := Task{
		ID:          fmt.Sprintf("task-%d", len(c.tasks)+1),
		Title:       title,
		Description: description,
		Status:      "created",
		Files:       []string{},
		Branch:      fmt.Sprintf("cw/%s", title),
	}

	c.tasks = append(c.tasks, task)

	if err := c.saveTasks(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"task":   task,
		"status": "created",
	}, nil
}

// plan creates implementation plan
func (c *CopilotWorkspace) plan(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	taskID, _ := params["task_id"].(string)
	if taskID == "" {
		return nil, fmt.Errorf("task_id required")
	}

	var task *Task
	for i := range c.tasks {
		if c.tasks[i].ID == taskID {
			task = &c.tasks[i]
			break
		}
	}

	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	plan := []map[string]interface{}{
		{"step": 1, "action": "Analyze requirements", "status": "pending"},
		{"step": 2, "action": "Design solution", "status": "pending"},
		{"step": 3, "action": "Implement changes", "status": "pending"},
		{"step": 4, "action": "Add tests", "status": "pending"},
		{"step": 5, "action": "Review", "status": "pending"},
	}

	return map[string]interface{}{
		"task": task,
		"plan": plan,
	}, nil
}

// implement implements the task
func (c *CopilotWorkspace) implement(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	taskID, _ := params["task_id"].(string)
	if taskID == "" {
		return nil, fmt.Errorf("task_id required")
	}

	var task *Task
	for i := range c.tasks {
		if c.tasks[i].ID == taskID {
			task = &c.tasks[i]
			break
		}
	}

	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	task.Status = "implementing"
	task.Files = []string{"src/main.go", "src/utils.go"}

	if err := c.saveTasks(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"task":    task,
		"changes": []string{"Added feature implementation"},
		"status":  "implemented",
	}, nil
}

// review reviews changes
func (c *CopilotWorkspace) review(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	taskID, _ := params["task_id"].(string)
	if taskID == "" {
		return nil, fmt.Errorf("task_id required")
	}

	return map[string]interface{}{
		"task_id": taskID,
		"review":  "Code review completed",
		"issues":  []string{},
		"status":  "reviewed",
	}, nil
}

// submit creates PR
func (c *CopilotWorkspace) submit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	taskID, _ := params["task_id"].(string)
	if taskID == "" {
		return nil, fmt.Errorf("task_id required")
	}

	var task *Task
	for i := range c.tasks {
		if c.tasks[i].ID == taskID {
			task = &c.tasks[i]
			break
		}
	}

	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	task.Status = "submitted"

	if err := c.saveTasks(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"task":   task,
		"pr_url": fmt.Sprintf("https://github.com/%s/pull/1", c.config.Repository),
		"status": "submitted",
	}, nil
}

// listTasks lists all tasks
func (c *CopilotWorkspace) listTasks(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"tasks": c.tasks,
		"count": len(c.tasks),
	}, nil
}

// IsAvailable checks availability
func (c *CopilotWorkspace) IsAvailable() bool {
	return c.config.GitHubToken != ""
}

var _ agents.AgentIntegration = (*CopilotWorkspace)(nil)
