// Package lovable provides Lovable agent integration.
// Lovable is a HOSTED, web-only AI full-stack app builder (lovable.dev) — there
// is no headless CLI or local builder. Creating, editing, deploying, adding
// features to, connecting a database for, or exporting an app all require real
// calls to Lovable's hosted service. Until that real HTTP client is wired, those
// commands return an HONEST error rather than fabricating project URLs, file
// lists, or "created"/"deployed"/"exported" statuses (anti-bluff: BLUFF-001).
// Only the real local project registry (list_projects) is served locally.
package lovable

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// ErrHostedOnly is returned by the app-builder commands because Lovable runs
// only as a hosted web service and no real HTTP client is wired here. Per
// CONST-035 / BLUFF-001 those commands return this honest error instead of
// fabricating a project URL, a file/component list, or a success status that no
// real Lovable operation produced.
var ErrHostedOnly = errors.New("lovable: a hosted web-only builder with no headless CLI and no wired HTTP client; " +
	"create/edit/deploy/add_feature/connect_database/export require real calls to lovable.dev — refusing to fabricate a result")

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

// saveProjects persists the local project registry to projects.json. It is the
// write pair of loadProjects (which IS wired — Initialize calls it). The two
// call sites that invoked saveProjects (createApp + deploy appending a
// fabricated Project) were deliberately removed by the D-17 de-bluff
// (commit ecdf5b81): Lovable is a hosted web-only builder, so we no longer
// fabricate a local "created" project to persist (BLUFF-001). Kept per §11.4.124
// as the documented, tested round-trip pair of loadProjects: it writes valid
// JSON that loadProjects reads back, mirrors the windsurf agent's load/save
// idiom, and is the legitimate persistence path the moment a real hosted client
// produces a genuine local registry mutation worth persisting. Round-trip
// covered by TestLovable_SaveLoadProjects_RoundTrip.
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

// createApp creates a new full-stack application. Honest error: Lovable is a
// hosted web builder and no real client is wired, so we refuse to fabricate a
// project URL + file list + "created" status (BLUFF-001).
func (l *Lovable) createApp(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}
	return nil, fmt.Errorf("lovable create_app: %w", ErrHostedOnly)
}

// edit makes visual edits. Honest error: no real hosted client wired (BLUFF-001).
func (l *Lovable) edit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectID, _ := params["project_id"].(string)
	prompt, _ := params["prompt"].(string)
	if projectID == "" || prompt == "" {
		return nil, fmt.Errorf("project_id and prompt required")
	}
	return nil, fmt.Errorf("lovable edit: %w", ErrHostedOnly)
}

// deploy deploys the application. Honest error: no real hosted client wired, so
// we refuse to fabricate a deploy URL + "deployed" status (BLUFF-001).
func (l *Lovable) deploy(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectID, _ := params["project_id"].(string)
	if projectID == "" {
		return nil, fmt.Errorf("project_id required")
	}
	return nil, fmt.Errorf("lovable deploy: %w", ErrHostedOnly)
}

// addFeature adds a feature. Honest error: no real hosted client wired, so we
// refuse to fabricate a generated component list (BLUFF-001).
func (l *Lovable) addFeature(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectID, _ := params["project_id"].(string)
	feature, _ := params["feature"].(string)
	if projectID == "" || feature == "" {
		return nil, fmt.Errorf("project_id and feature required")
	}
	return nil, fmt.Errorf("lovable add_feature: %w", ErrHostedOnly)
}

// connectDatabase connects a database. Honest error: no real hosted client
// wired, so we refuse to claim a database was connected (BLUFF-001).
func (l *Lovable) connectDatabase(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectID, _ := params["project_id"].(string)
	if projectID == "" {
		return nil, fmt.Errorf("project_id required")
	}
	return nil, fmt.Errorf("lovable connect_database: %w", ErrHostedOnly)
}

// listProjects lists the real local project registry (read from projects.json
// on disk). This is honest local state — never fabricated.
func (l *Lovable) listProjects(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"projects": l.projects,
		"count":    len(l.projects),
	}, nil
}

// exportCode exports project code. Honest error: no real hosted client wired, so
// we refuse to claim an export happened to a fabricated path (BLUFF-001).
func (l *Lovable) exportCode(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	projectID, _ := params["project_id"].(string)
	if projectID == "" {
		return nil, fmt.Errorf("project_id required")
	}
	return nil, fmt.Errorf("lovable export_code: %w", ErrHostedOnly)
}

// IsAvailable checks availability
func (l *Lovable) IsAvailable() bool {
	return l.config.APIKey != ""
}

var _ agents.AgentIntegration = (*Lovable)(nil)
