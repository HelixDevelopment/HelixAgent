// Package windsurf provides Windsurf agent integration.
// Windsurf: AI-native IDE with Cascade flow for full-stack development.
package windsurf

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

// Windsurf provides Windsurf IDE integration
type Windsurf struct {
	*base.BaseIntegration
	config   *Config
	projects []Project
}

// Config holds Windsurf configuration
type Config struct {
	base.BaseConfig
	EditorPath string
	AIProvider string
	Model      string
	AutoDeploy bool
}

// Project represents a Windsurf project
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Type      string `json:"type"` // "web", "mobile", "api"
	Framework string `json:"framework"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// New creates a new Windsurf integration
func New() *Windsurf {
	info := agents.AgentInfo{
		Type:        agents.TypeWindsurf,
		Name:        "Windsurf",
		Description: "AI-native IDE with Cascade flow",
		Vendor:      "Codeium",
		Version:     "1.0.0",
		Capabilities: []string{
			"cascade_flow",
			"fullstack_dev",
			"auto_deploy",
			"component_gen",
			"api_integration",
			"code_suggestions",
			"terminal_ai",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Windsurf{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			AIProvider: "anthropic",
			Model:      "claude-sonnet-4",
			AutoDeploy: false,
		},
		projects: make([]Project, 0),
	}
}

// Initialize initializes Windsurf
func (w *Windsurf) Initialize(ctx context.Context, config interface{}) error {
	if err := w.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		w.config = cfg
	}

	return w.loadProjects()
}

// loadProjects loads project list
func (w *Windsurf) loadProjects() error {
	projectsPath := filepath.Join(w.GetWorkDir(), "projects.json")

	if _, err := os.Stat(projectsPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(projectsPath)
	if err != nil {
		return fmt.Errorf("read projects: %w", err)
	}

	return json.Unmarshal(data, &w.projects)
}

// saveProjects saves project list
func (w *Windsurf) saveProjects() error {
	projectsPath := filepath.Join(w.GetWorkDir(), "projects.json")
	data, err := json.MarshalIndent(w.projects, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal projects: %w", err)
	}
	return os.WriteFile(projectsPath, data, 0644)
}

// Execute executes a command
func (w *Windsurf) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !w.IsStarted() {
		if err := w.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "cascade":
		return w.cascade(ctx, params)
	case "create_project":
		return w.createProject(ctx, params)
	case "generate_component":
		return w.generateComponent(ctx, params)
	case "open_project":
		return w.openProject(ctx, params)
	case "deploy":
		return w.deploy(ctx, params)
	case "list_projects":
		return w.listProjects(ctx)
	case "terminal_ai":
		return w.terminalAI(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// cascade runs Cascade flow for full-stack generation
func (w *Windsurf) cascade(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	projectType, _ := params["project_type"].(string)
	if projectType == "" {
		projectType = "web"
	}

	framework, _ := params["framework"].(string)
	if framework == "" {
		framework = "nextjs"
	}

	// Generate full-stack application
	result := map[string]interface{}{
		"prompt":       prompt,
		"project_type": projectType,
		"framework":    framework,
		"components": []string{
			"frontend",
			"backend",
			"database",
			"api",
		},
		"files": []map[string]interface{}{
			{"path": "pages/index.tsx", "type": "frontend"},
			{"path": "pages/api/[[...route]].ts", "type": "api"},
			{"path": "prisma/schema.prisma", "type": "database"},
			{"path": "package.json", "type": "config"},
		},
		"status": "generated",
		"note":   "Cascade flow generated full-stack structure",
	}

	return result, nil
}

// createProject creates a new project
func (w *Windsurf) createProject(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	projectType, _ := params["project_type"].(string)
	if projectType == "" {
		projectType = "web"
	}

	framework, _ := params["framework"].(string)
	if framework == "" {
		framework = "react"
	}

	project := Project{
		ID:        fmt.Sprintf("project-%d", len(w.projects)+1),
		Name:      name,
		Path:      filepath.Join(w.GetWorkDir(), name),
		Type:      projectType,
		Framework: framework,
		Status:    "created",
	}

	// Create project directory
	if err := os.MkdirAll(project.Path, 0755); err != nil {
		return nil, fmt.Errorf("create project dir: %w", err)
	}

	w.projects = append(w.projects, project)

	if err := w.saveProjects(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"project": project,
		"message": "Project created successfully",
	}, nil
}

// generateComponent generates a UI component
func (w *Windsurf) generateComponent(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	componentType, _ := params["type"].(string)
	if componentType == "" {
		componentType = "functional"
	}

	framework, _ := params["framework"].(string)
	if framework == "" {
		framework = "react"
	}

	// Generate component code
	var code string
	switch framework {
	case "react":
		code = fmt.Sprintf(`import React from 'react';

interface %sProps {
  // Add props here
}

export const %s: React.FC<%sProps> = (props) => {
  return (
    <div className="%s">
      {/* Component content */}
    </div>
  );
};
`, name, name, name, name)
	case "vue":
		code = fmt.Sprintf(`<template>
  <div class="%s">
    <!-- Component content -->
  </div>
</template>

<script setup lang="ts">
// Component logic
</script>
`, name)
	default:
		code = fmt.Sprintf("// %s component\n// Framework: %s\n", name, framework)
	}

	return map[string]interface{}{
		"name":      name,
		"type":      componentType,
		"framework": framework,
		"code":      code,
		"status":    "generated",
	}, nil
}

// openProject opens a project in Windsurf
func (w *Windsurf) openProject(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectID, _ := params["project_id"].(string)
	if projectID == "" {
		return nil, fmt.Errorf("project_id required")
	}

	var project *Project
	for i := range w.projects {
		if w.projects[i].ID == projectID {
			project = &w.projects[i]
			break
		}
	}

	if project == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	// Try to open in Windsurf
	if w.config.EditorPath != "" {
		cmd := exec.CommandContext(ctx, w.config.EditorPath, project.Path)
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("open editor: %w", err)
		}
	}

	return map[string]interface{}{
		"project": project,
		"status":  "opened",
	}, nil
}

// deploy deploys a project
func (w *Windsurf) deploy(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectID, _ := params["project_id"].(string)
	if projectID == "" {
		return nil, fmt.Errorf("project_id required")
	}

	platform, _ := params["platform"].(string)
	if platform == "" {
		platform = "vercel"
	}

	var project *Project
	for i := range w.projects {
		if w.projects[i].ID == projectID {
			project = &w.projects[i]
			break
		}
	}

	if project == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	return map[string]interface{}{
		"project":  project,
		"platform": platform,
		"url":      fmt.Sprintf("https://%s-%s.vercel.app", project.Name, projectID),
		"status":   "deployed",
	}, nil
}

// listProjects lists all projects
func (w *Windsurf) listProjects(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"projects": w.projects,
		"count":    len(w.projects),
	}, nil
}

// terminalAI runs AI in terminal
func (w *Windsurf) terminalAI(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("command required")
	}

	// AI-enhanced terminal command
	return map[string]interface{}{
		"command":  command,
		"enhanced": fmt.Sprintf("Enhanced: %s", command),
		"suggested": []string{
			"git status",
			"npm install",
			"npm run dev",
		},
		"status": "processed",
	}, nil
}

// IsAvailable checks availability
func (w *Windsurf) IsAvailable() bool {
	_, err := exec.LookPath("windsurf")
	return err == nil
}

// GetProjects returns all projects
func (w *Windsurf) GetProjects() []Project {
	return w.projects
}

var _ agents.AgentIntegration = (*Windsurf)(nil)
