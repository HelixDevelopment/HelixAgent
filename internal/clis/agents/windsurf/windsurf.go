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
	"sync/atomic"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Windsurf provides Windsurf IDE integration.
//
// Concurrent-safe by construction (CONST-029): projects is a safe.Slice;
// nextID is an atomic.Int64 counter that takes ID generation off the
// slice's len() so concurrent createProject calls cannot collide on
// IDs (the previous mutex-guarded `len(projects)+1` worked but was
// fragile under any future migration that broke len/append atomicity).
type Windsurf struct {
	*base.BaseIntegration
	config   *Config
	projects *safe.Slice[Project]
	nextID   atomic.Int64
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
		projects: safe.NewSlice[Project](),
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

	var loaded []Project
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	w.projects.Replace(loaded)
	// Seed the ID counter so subsequent IDs do not collide with loaded ones.
	if int64(len(loaded)) > w.nextID.Load() {
		w.nextID.Store(int64(len(loaded)))
	}
	return nil
}

// saveProjects saves project list
func (w *Windsurf) saveProjects() error {
	projectsPath := filepath.Join(w.GetWorkDir(), "projects.json")
	snapshot := w.projects.Snapshot()
	data, err := json.MarshalIndent(snapshot, "", "  ")
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

// cascade returns an HONEST error: Cascade is Windsurf's in-IDE agentic flow
// (Codeium's Windsurf editor); it has no headless CLI that synthesises a
// full-stack application from a prompt. Rather than fabricate a fake file
// manifest for an app that was never generated (BLUFF-001), this returns an
// honest error. See https://windsurf.com .
func (w *Windsurf) cascade(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}
	return nil, fmt.Errorf("windsurf Cascade has no headless CLI: it is an in-IDE agentic flow, so full-stack generation cannot be produced from this integration — refusing to fabricate a file manifest (BLUFF-001)")
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

	// Atomic ID gen via counter; safe.Slice.Append is independently atomic.
	id := w.nextID.Add(1)
	project := Project{
		ID:        fmt.Sprintf("project-%d", id),
		Name:      name,
		Path:      filepath.Join(w.GetWorkDir(), name),
		Type:      projectType,
		Framework: framework,
		Status:    "created",
	}

	if err := os.MkdirAll(project.Path, 0755); err != nil {
		return nil, fmt.Errorf("create project dir: %w", err)
	}

	w.projects.Append(project)

	if err := w.saveProjects(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"project": project,
		"message": "Project created successfully",
	}, nil
}

// generateComponent returns an HONEST error: AI component generation is a
// Windsurf in-IDE feature with no headless CLI. The previous implementation
// returned a static boilerplate template while labelling it "generated" — a
// BLUFF-001 (it claimed AI generation but only string-formatted a fixed
// skeleton). Refusing to present a hardcoded skeleton as a generated component.
func (w *Windsurf) generateComponent(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}
	return nil, fmt.Errorf("windsurf component generation has no headless CLI: it is an in-IDE feature, and returning a fixed boilerplate skeleton as AI output would be a bluff — refusing (BLUFF-001)")
}

// openProject opens a project in Windsurf
func (w *Windsurf) openProject(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectID, _ := params["project_id"].(string)
	if projectID == "" {
		return nil, fmt.Errorf("project_id required")
	}

	// Snapshot the project so callers don't race with concurrent
	// mutations.
	found, ok := w.projects.Find(func(p Project) bool { return p.ID == projectID })
	if !ok {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}
	project := &found

	// Try to open in Windsurf
	if w.config.EditorPath != "" {
		cmd := exec.CommandContext(ctx, w.config.EditorPath, project.Path) // #nosec G204 // EditorPath from trusted config
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

	// Snapshot the project (same pattern as openProject) so the not-found check
	// still validates input before reporting the unsupported deploy capability.
	if _, ok := w.projects.Find(func(p Project) bool { return p.ID == projectID }); !ok {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	// HONEST error: this integration performs no real deployment. Returning a
	// fabricated "deployed" status + a guessed *.vercel.app URL for a deploy that
	// never ran is a BLUFF-001. Real deployment requires the platform's own CLI/
	// API (e.g. `vercel deploy`) against authenticated credentials.
	return nil, fmt.Errorf("windsurf deploy performs no real deployment to %s: refusing to report a fabricated 'deployed' status and URL (BLUFF-001) — use the platform's own deploy CLI/API", platform)
}

// listProjects lists all projects — returns a defensive snapshot.
func (w *Windsurf) listProjects(ctx context.Context) (interface{}, error) {
	snapshot := w.projects.Snapshot()
	return map[string]interface{}{
		"projects": snapshot,
		"count":    len(snapshot),
	}, nil
}

// terminalAI returns an HONEST error: Windsurf's AI terminal is an in-IDE
// feature with no headless CLI. The previous implementation returned a
// "Enhanced: <cmd>" echo plus a fixed list of suggestions — fabricated AI
// output (BLUFF-001). Refusing to present a templated echo as AI assistance.
func (w *Windsurf) terminalAI(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("command required")
	}
	return nil, fmt.Errorf("windsurf terminal AI has no headless CLI: it is an in-IDE feature, and a templated echo + fixed suggestions is not real AI output — refusing (BLUFF-001)")
}

// IsAvailable checks availability
func (w *Windsurf) IsAvailable() bool {
	_, err := exec.LookPath("windsurf")
	return err == nil
}

// GetProjects returns all projects (defensive snapshot).
func (w *Windsurf) GetProjects() []Project {
	return w.projects.Snapshot()
}

var _ agents.AgentIntegration = (*Windsurf)(nil)
