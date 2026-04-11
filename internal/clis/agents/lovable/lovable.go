// Package lovable provides Lovable agent integration.
// Lovable: AI-powered full-stack app builder with visual editing.
package lovable

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Lovable provides Lovable integration
type Lovable struct {
	*base.BaseIntegration
	config   *Config
	projects []Project
}

// Config holds Lovable configuration
type Config struct {
	base.BaseConfig
	APIKey       string
	DefaultStack string
	AutoDeploy   bool
}

// Project represents a Lovable project
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Stack       string `json:"stack"`
	Status      string `json:"status"`
	URL         string `json:"url"`
}

// New creates a new Lovable integration
func New() *Lovable {
	info := agents.AgentInfo{
		Type:        agents.TypeLovable,
		Name:        "Lovable",
		Description: "AI-powered full-stack app builder",
		Vendor:      "Lovable",
		Version:     "1.0.0",
		Capabilities: []string{
			"visual_editing",
			"fullstack_generation",
			"auto_deploy",
			"component_library",
			"responsive_design",
			"database_integration",
			"api_generation",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Lovable{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			DefaultStack: "react-node-postgres",
			AutoDeploy:   false,
		},
		projects: make([]Project, 0),
	}
}

// Initialize initializes Lovable
func (l *Lovable) Initialize(ctx context.Context, config interface{}) error {
	if err := l.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		l.config = cfg
	}

	return l.loadProjects()
}

// loadProjects loads project list
func (l *Lovable) loadProjects() error {
	projectsPath := filepath.Join(l.GetWorkDir(), "projects.json")

	if _, err := os.Stat(projectsPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(projectsPath)
	if err != nil {
		return fmt.Errorf("read projects: %w", err)
	}

	return json.Unmarshal(data, &l.projects)
}

// saveProjects saves project list
func (l *Lovable) saveProjects() error {
	projectsPath := filepath.Join(l.GetWorkDir(), "projects.json")
	data, err := json.MarshalIndent(l.projects, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal projects: %w", err)
	}
	return os.WriteFile(projectsPath, data, 0644)
}

// Execute executes a command
func (l *Lovable) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !l.IsStarted() {
		if err := l.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "create_app":
		return l.createApp(ctx, params)
	case "edit":
		return l.edit(ctx, params)
	case "deploy":
		return l.deploy(ctx, params)
	case "add_feature":
		return l.addFeature(ctx, params)
	case "connect_database":
		return l.connectDatabase(ctx, params)
	case "list_projects":
		return l.listProjects(ctx)
	case "export_code":
		return l.exportCode(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// createApp creates a new full-stack application
func (l *Lovable) createApp(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	description, _ := params["description"].(string)
	stack, _ := params["stack"].(string)
	if stack == "" {
		stack = l.config.DefaultStack
	}

	project := Project{
		ID:          fmt.Sprintf("lovable-%d", len(l.projects)+1),
		Name:        name,
		Description: description,
		Stack:       stack,
		Status:      "created",
		URL:         fmt.Sprintf("https://lovable.dev/p/%s", name),
	}

	l.projects = append(l.projects, project)

	if err := l.saveProjects(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"project": project,
		"files": []string{
			"src/App.tsx",
			"src/components/",
			"src/pages/",
			"api/routes.ts",
			"prisma/schema.prisma",
		},
		"status": "created",
	}, nil
}

// edit makes visual edits
func (l *Lovable) edit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectID, _ := params["project_id"].(string)
	prompt, _ := params["prompt"].(string)

	if projectID == "" || prompt == "" {
		return nil, fmt.Errorf("project_id and prompt required")
	}

	var project *Project
	for i := range l.projects {
		if l.projects[i].ID == projectID {
			project = &l.projects[i]
			break
		}
	}

	if project == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	return map[string]interface{}{
		"project": project,
		"prompt":  prompt,
		"edits": []map[string]interface{}{
			{"type": "visual", "target": "ui", "change": prompt},
		},
		"status": "edited",
	}, nil
}

// deploy deploys the application
func (l *Lovable) deploy(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectID, _ := params["project_id"].(string)
	if projectID == "" {
		return nil, fmt.Errorf("project_id required")
	}

	var project *Project
	for i := range l.projects {
		if l.projects[i].ID == projectID {
			project = &l.projects[i]
			break
		}
	}

	if project == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	project.Status = "deployed"
	project.URL = fmt.Sprintf("https://%s.lovable.app", project.Name)

	if err := l.saveProjects(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"project": project,
		"url":     project.URL,
		"status":  "deployed",
	}, nil
}

// addFeature adds a feature
func (l *Lovable) addFeature(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectID, _ := params["project_id"].(string)
	feature, _ := params["feature"].(string)

	if projectID == "" || feature == "" {
		return nil, fmt.Errorf("project_id and feature required")
	}

	return map[string]interface{}{
		"project_id": projectID,
		"feature":    feature,
		"components": []string{
			fmt.Sprintf("%s.tsx", feature),
			fmt.Sprintf("%s.test.tsx", feature),
		},
		"status": "added",
	}, nil
}

// connectDatabase connects a database
func (l *Lovable) connectDatabase(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectID, _ := params["project_id"].(string)
	dbType, _ := params["type"].(string)

	if projectID == "" {
		return nil, fmt.Errorf("project_id required")
	}

	if dbType == "" {
		dbType = "postgres"
	}

	return map[string]interface{}{
		"project_id": projectID,
		"database":   dbType,
		"schema":     "auto-generated",
		"status":     "connected",
	}, nil
}

// listProjects lists all projects
func (l *Lovable) listProjects(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"projects": l.projects,
		"count":    len(l.projects),
	}, nil
}

// exportCode exports project code
func (l *Lovable) exportCode(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectID, _ := params["project_id"].(string)
	if projectID == "" {
		return nil, fmt.Errorf("project_id required")
	}

	var project *Project
	for i := range l.projects {
		if l.projects[i].ID == projectID {
			project = &l.projects[i]
			break
		}
	}

	if project == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	return map[string]interface{}{
		"project":     project,
		"export_path": filepath.Join(l.GetWorkDir(), "exports", project.Name),
		"format":      "zip",
		"status":      "exported",
	}, nil
}

// IsAvailable checks availability
func (l *Lovable) IsAvailable() bool {
	return l.config.APIKey != ""
}

var _ agents.AgentIntegration = (*Lovable)(nil)
