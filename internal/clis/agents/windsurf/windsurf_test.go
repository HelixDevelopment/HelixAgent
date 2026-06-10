// Package windsurf provides tests for Windsurf agent integration
package windsurf

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

func TestNewWindsurf(t *testing.T) {
	t.Parallel()
	w := New()

	if w == nil {
		t.Fatal("New() = nil")
	}

	info := w.Info()
	if info.Type != agents.TypeWindsurf {
		t.Errorf("Info().Type = %q, want %q", info.Type, agents.TypeWindsurf)
	}

	if info.Name != "Windsurf" {
		t.Errorf("Info().Name = %q, want %q", info.Name, "Windsurf")
	}

	if info.Vendor != "Codeium" {
		t.Errorf("Info().Vendor = %q, want %q", info.Vendor, "Codeium")
	}
}

func TestWindsurfInitialize(t *testing.T) {
	t.Parallel()
	w := New()
	ctx := context.Background()

	tempDir := t.TempDir()
	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: tempDir,
		},
		EditorPath: "/usr/bin/windsurf",
		AIProvider: "anthropic",
		Model:      "claude-opus-4",
		AutoDeploy: true,
	}

	err := w.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if w.config.EditorPath != "/usr/bin/windsurf" {
		t.Errorf("config.EditorPath = %q, want %q", w.config.EditorPath, "/usr/bin/windsurf")
	}

	if w.config.AIProvider != "anthropic" {
		t.Errorf("config.AIProvider = %q, want %q", w.config.AIProvider, "anthropic")
	}

	if w.config.AutoDeploy != true {
		t.Errorf("config.AutoDeploy = %v, want %v", w.config.AutoDeploy, true)
	}
}

func TestWindsurfStartStop(t *testing.T) {
	t.Parallel()
	w := New()
	ctx := context.Background()

	err := w.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	err = w.Start(ctx)
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}

	if !w.IsStarted() {
		t.Error("IsStarted() = false after Start()")
	}

	err = w.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	if w.IsStarted() {
		t.Error("IsStarted() = true after Stop()")
	}
}

func TestWindsurfExecute(t *testing.T) {
	t.Parallel()
	w := New()
	ctx := context.Background()

	err := w.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	tests := []struct {
		name    string
		command string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			// Reconciled (§11.4.120): cascade now returns an honest error —
			// Windsurf Cascade is an in-IDE agentic flow with no headless CLI, so
			// it refuses to fabricate a full-stack file manifest (BLUFF-001).
			name:    "cascade command returns honest error (no headless CLI)",
			command: "cascade",
			params: map[string]interface{}{
				"prompt":       "Build a todo app",
				"project_type": "web",
				"framework":    "nextjs",
			},
			wantErr: true,
		},
		{
			name:    "cascade without prompt fails",
			command: "cascade",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "create_project command",
			command: "create_project",
			params: map[string]interface{}{
				"name":         "my-app",
				"project_type": "web",
				"framework":    "react",
			},
			wantErr: false,
		},
		{
			name:    "create_project without name fails",
			command: "create_project",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			// Reconciled (§11.4.120): generate_component now returns an honest
			// error — it previously returned a fixed boilerplate skeleton labelled
			// "generated" (BLUFF-001). In-IDE feature, no headless CLI.
			name:    "generate_component command returns honest error (no headless CLI)",
			command: "generate_component",
			params: map[string]interface{}{
				"name":      "Button",
				"type":      "functional",
				"framework": "react",
			},
			wantErr: true,
		},
		{
			name:    "generate_component without name fails",
			command: "generate_component",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "list_projects command",
			command: "list_projects",
			params:  map[string]interface{}{},
			wantErr: false,
		},
		{
			// Reconciled (§11.4.120): terminal_ai now returns an honest error — it
			// previously returned a "Enhanced: <cmd>" echo + fixed suggestions
			// (BLUFF-001). In-IDE feature, no headless CLI.
			name:    "terminal_ai command returns honest error (no headless CLI)",
			command: "terminal_ai",
			params: map[string]interface{}{
				"command": "git status",
			},
			wantErr: true,
		},
		{
			name:    "terminal_ai without command fails",
			command: "terminal_ai",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "unknown command",
			command: "unknown",
			params:  map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := w.Execute(ctx, tt.command, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result == nil {
				t.Error("Execute() result = nil, want non-nil")
			}
		})
	}
}

// TestWindsurfCascadeNoFabrication reconciles the former TestWindsurfCascade
// (§11.4.120): it asserted a fabricated file/component manifest was returned for
// a full-stack app that was never generated — BLUFF-001. cascade now returns an
// honest error. Standing GREEN regression guard (§11.4.135): reverting cascade
// to return a fake manifest (err == nil) makes this FAIL.
func TestWindsurfCascadeNoFabrication(t *testing.T) {
	t.Parallel()
	w := New()
	ctx := context.Background()

	if err := w.Initialize(ctx, nil); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := w.Execute(ctx, "cascade", map[string]interface{}{
		"prompt":       "Create an e-commerce site",
		"project_type": "web",
		"framework":    "nextjs",
	})
	if err == nil {
		t.Errorf("cascade returned nil error — must return an honest error, never a fabricated file manifest (BLUFF-001); got %v", result)
	}
	if result != nil {
		t.Errorf("cascade returned a result payload %v — must be nil when no real generation ran", result)
	}
}

func TestWindsurfCreateProject(t *testing.T) {
	t.Parallel()
	w := New()
	ctx := context.Background()

	tempDir := t.TempDir()
	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: tempDir,
		},
	}

	err := w.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := w.Execute(ctx, "create_project", map[string]interface{}{
		"name":         "test-project",
		"project_type": "api",
		"framework":    "express",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	project, ok := resultMap["project"].(Project)
	if !ok {
		projectMap, ok := resultMap["project"].(map[string]interface{})
		if !ok {
			t.Fatal("project is not a Project or map")
		}
		if projectMap["name"] != "test-project" {
			t.Errorf("project.name = %v, want %v", projectMap["name"], "test-project")
		}
	} else {
		if project.Name != "test-project" {
			t.Errorf("project.Name = %v, want %v", project.Name, "test-project")
		}
	}

	// Check that the project directory was created
	projectDir := filepath.Join(tempDir, "test-project")
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		t.Error("project directory was not created")
	}
}

// TestWindsurfGenerateComponentNoFabrication reconciles the former
// TestWindsurfGenerateComponent (§11.4.120): it asserted a fixed boilerplate
// skeleton was returned in the "code" field labelled "generated" — BLUFF-001
// (a static template presented as AI output). generate_component now returns an
// honest error. Standing GREEN regression guard (§11.4.135).
func TestWindsurfGenerateComponentNoFabrication(t *testing.T) {
	t.Parallel()
	w := New()
	ctx := context.Background()

	if err := w.Initialize(ctx, nil); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	for _, framework := range []string{"react", "vue", "unknown"} {
		result, err := w.Execute(ctx, "generate_component", map[string]interface{}{
			"name":      "Card",
			"type":      "functional",
			"framework": framework,
		})
		if err == nil {
			t.Errorf("generate_component(framework=%s) returned nil error — must return an honest error, never a fabricated component (BLUFF-001); got %v", framework, result)
		}
		if result != nil {
			t.Errorf("generate_component(framework=%s) returned a result payload %v — must be nil when no real generation ran", framework, result)
		}
	}
}

func TestWindsurfListProjects(t *testing.T) {
	t.Parallel()
	w := New()
	ctx := context.Background()

	tempDir := t.TempDir()
	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: tempDir,
		},
	}

	err := w.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Create a project first
	_, _ = w.Execute(ctx, "create_project", map[string]interface{}{
		"name": "project-1",
	})

	result, err := w.Execute(ctx, "list_projects", nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}

	if _, ok := resultMap["projects"]; !ok {
		t.Error("list_projects result missing 'projects' field")
	}

	if _, ok := resultMap["count"]; !ok {
		t.Error("list_projects result missing 'count' field")
	}

	if count, ok := resultMap["count"].(int); ok && count != 1 {
		t.Errorf("count = %v, want %v", count, 1)
	}
}

func TestWindsurfDeploy(t *testing.T) {
	t.Parallel()
	w := New()
	ctx := context.Background()

	tempDir := t.TempDir()
	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: tempDir,
		},
	}

	err := w.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Create a project first
	_, _ = w.Execute(ctx, "create_project", map[string]interface{}{
		"name": "deploy-test",
	})

	// Get the project ID
	projects := w.GetProjects()
	if len(projects) == 0 {
		t.Fatal("No projects found")
	}
	projectID := projects[0].ID

	// Reconciled (§11.4.120): deploy on an EXISTING project no longer fabricates
	// a "deployed" status + a guessed *.vercel.app URL for a deploy that never
	// ran (BLUFF-001). It returns an honest error. The not-found path
	// (TestWindsurfDeployProjectNotFound) still errors for a different reason.
	result, err := w.Execute(ctx, "deploy", map[string]interface{}{
		"project_id": projectID,
		"platform":   "vercel",
	})
	if err == nil {
		t.Errorf("deploy returned nil error — must return an honest error, never a fabricated 'deployed' status/URL (BLUFF-001); got %v", result)
	}
	if result != nil {
		t.Errorf("deploy returned a result payload %v — must be nil when no real deployment ran", result)
	}
}

func TestWindsurfDeployProjectNotFound(t *testing.T) {
	t.Parallel()
	w := New()
	ctx := context.Background()

	err := w.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	_, err = w.Execute(ctx, "deploy", map[string]interface{}{
		"project_id": "non-existent-id",
	})
	if err == nil {
		t.Error("deploy with non-existent project_id should fail")
	}
}

// TestWindsurfTerminalAINoFabrication reconciles the former
// TestWindsurfTerminalAI (§11.4.120): it asserted a "Enhanced: <cmd>" echo + a
// fixed suggestion list was returned — fabricated AI output (BLUFF-001).
// terminal_ai now returns an honest error. Standing GREEN regression guard.
func TestWindsurfTerminalAINoFabrication(t *testing.T) {
	t.Parallel()
	w := New()
	ctx := context.Background()

	if err := w.Initialize(ctx, nil); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := w.Execute(ctx, "terminal_ai", map[string]interface{}{
		"command": "npm install",
	})
	if err == nil {
		t.Errorf("terminal_ai returned nil error — must return an honest error, never a fabricated echo/suggestions (BLUFF-001); got %v", result)
	}
	if result != nil {
		t.Errorf("terminal_ai returned a result payload %v — must be nil when no real AI ran", result)
	}
}

func TestWindsurfCapabilities(t *testing.T) {
	t.Parallel()
	w := New()
	info := w.Info()

	expectedCapabilities := []string{
		"cascade_flow",
		"fullstack_dev",
		"auto_deploy",
		"component_gen",
		"api_integration",
		"code_suggestions",
		"terminal_ai",
	}

	for _, cap := range expectedCapabilities {
		found := false
		for _, has := range info.Capabilities {
			if has == cap {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing capability: %s", cap)
		}
	}
}

func TestWindsurfHealth(t *testing.T) {
	t.Parallel()
	w := New()
	ctx := context.Background()

	// Before start, health should fail
	if err := w.Health(ctx); err == nil {
		t.Error("Health() before Start = nil, want error")
	}

	_ = w.Initialize(ctx, nil)
	_ = w.Start(ctx)

	// After start, health should pass
	if err := w.Health(ctx); err != nil {
		t.Errorf("Health() after Start error = %v", err)
	}
}

func TestWindsurfProjectsPersistence(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create first instance and add a project
	w1 := New()
	ctx := context.Background()

	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: tempDir,
		},
	}

	err := w1.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	_, err = w1.Execute(ctx, "create_project", map[string]interface{}{
		"name": "persistent-project",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Create second instance and load projects
	w2 := New()
	err = w2.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	projects := w2.GetProjects()
	if len(projects) != 1 {
		t.Errorf("loaded projects count = %d, want %d", len(projects), 1)
	}

	if len(projects) > 0 && projects[0].Name != "persistent-project" {
		t.Errorf("project name = %q, want %q", projects[0].Name, "persistent-project")
	}
}

func TestWindsurfConfigValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config with all fields",
			config: &Config{
				EditorPath: "/usr/bin/windsurf",
				AIProvider: "anthropic",
				Model:      "claude-opus-4",
				AutoDeploy: true,
			},
			wantErr: false,
		},
		{
			name:    "nil config uses defaults",
			config:  nil,
			wantErr: false,
		},
		{
			name: "empty config fields use defaults",
			config: &Config{
				EditorPath: "",
				Model:      "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := New()
			ctx := context.Background()

			cfg := tt.config
			if cfg == nil {
				cfg = &Config{
					BaseConfig: base.BaseConfig{
						WorkDir: t.TempDir(),
					},
				}
			} else if cfg.WorkDir == "" {
				cfg.WorkDir = t.TempDir()
			}

			err := w.Initialize(ctx, cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWindsurfGetProjects(t *testing.T) {
	t.Parallel()
	w := New()
	ctx := context.Background()

	tempDir := t.TempDir()
	config := &Config{
		BaseConfig: base.BaseConfig{
			WorkDir: tempDir,
		},
	}

	err := w.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Initially empty
	projects := w.GetProjects()
	if len(projects) != 0 {
		t.Errorf("initial projects count = %d, want %d", len(projects), 0)
	}

	// Create a project
	_, _ = w.Execute(ctx, "create_project", map[string]interface{}{
		"name": "test-project",
	})

	// Should have one project
	projects = w.GetProjects()
	if len(projects) != 1 {
		t.Errorf("projects count after create = %d, want %d", len(projects), 1)
	}
}

func BenchmarkWindsurfExecute(b *testing.B) {
	w := New()
	ctx := context.Background()
	_ = w.Initialize(ctx, nil)
	_ = w.Start(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = w.Execute(ctx, "cascade", map[string]interface{}{
			"prompt": "test",
		})
	}
}
