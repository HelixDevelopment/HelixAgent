// Package aider provides Aider CLI agent integration.
// Aider: AI pair programming with support for multiple LLMs and git integration.
package aider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Aider provides Aider CLI integration
type Aider struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Aider configuration
type Config struct {
	base.BaseConfig
	EditorModel   string
	ArchitectMode bool
	AutoCommits   bool
	AutoLint      bool
	AutoTest      bool
	DarkMode      bool
	ShowDiffs     bool
	GitIgnore     bool
}

// New creates a new Aider integration
func New() *Aider {
	info := agents.AgentInfo{
		Type:        agents.TypeAider,
		Name:        "Aider",
		Description: "AI pair programming with git integration",
		Vendor:      "Aider",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_editing",
			"git_integration",
			"multi_file_editing",
			"repo_map",
			"architect_mode",
			"auto_commit",
			"linting",
			"testing",
		},
		IsEnabled: true,
		Priority:  1,
	}

	return &Aider{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				Model:     "claude-3-sonnet",
				AutoStart: true,
			},
			EditorModel:   "claude-3-sonnet",
			ArchitectMode: true,
			AutoCommits:   true,
			AutoLint:      true,
			AutoTest:      false,
			DarkMode:      true,
			ShowDiffs:     true,
			GitIgnore:     true,
		},
	}
}

// Initialize initializes Aider
func (a *Aider) Initialize(ctx context.Context, config interface{}) error {
	if err := a.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		a.config = cfg
	}

	return nil
}

// Execute executes an Aider command
func (a *Aider) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !a.IsStarted() {
		if err := a.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "chat":
		return a.chat(ctx, params)
	case "edit":
		return a.edit(ctx, params)
	case "commit":
		return a.commit(ctx, params)
	case "undo":
		return a.undo(ctx)
	case "add":
		return a.addFiles(ctx, params)
	case "drop":
		return a.dropFiles(ctx, params)
	case "ls":
		return a.listFiles(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// chat sends a chat message to Aider
func (a *Aider) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	args := a.buildArgs()
	args = append(args, "--message", message)

	output, err := a.ExecuteCommand(ctx, "aider", args...)
	if err != nil {
		return nil, fmt.Errorf("aider chat failed: %w\nOutput: %s", err, string(output))
	}

	return map[string]interface{}{
		"response": string(output),
		"success":  true,
	}, nil
}

// edit performs a code edit
func (a *Aider) edit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	files, _ := params["files"].([]string)

	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	args := a.buildArgs()

	// Add files to context
	for _, file := range files {
		args = append(args, file)
	}

	args = append(args, "--edit", prompt)

	output, err := a.ExecuteCommand(ctx, "aider", args...)
	if err != nil {
		return nil, fmt.Errorf("aider edit failed: %w\nOutput: %s", err, string(output))
	}

	return map[string]interface{}{
		"response": string(output),
		"success":  true,
	}, nil
}

// commit commits changes
func (a *Aider) commit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)

	args := []string{"commit"}
	if message != "" {
		args = append(args, "--message", message)
	}

	output, err := a.ExecuteCommand(ctx, "aider", args...)
	if err != nil {
		return nil, fmt.Errorf("aider commit failed: %w", err)
	}

	return map[string]interface{}{
		"response": string(output),
		"success":  true,
	}, nil
}

// undo undoes the last change
func (a *Aider) undo(ctx context.Context) (interface{}, error) {
	output, err := a.ExecuteCommand(ctx, "aider", "undo")
	if err != nil {
		return nil, fmt.Errorf("aider undo failed: %w", err)
	}

	return map[string]interface{}{
		"response": string(output),
		"success":  true,
	}, nil
}

// addFiles adds files to the chat context
func (a *Aider) addFiles(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	files, _ := params["files"].([]string)
	if len(files) == 0 {
		return nil, fmt.Errorf("files required")
	}

	args := append([]string{"add"}, files...)
	output, err := a.ExecuteCommand(ctx, "aider", args...)
	if err != nil {
		return nil, fmt.Errorf("aider add failed: %w", err)
	}

	return map[string]interface{}{
		"response": string(output),
		"success":  true,
	}, nil
}

// dropFiles removes files from the chat context
func (a *Aider) dropFiles(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	files, _ := params["files"].([]string)
	if len(files) == 0 {
		return nil, fmt.Errorf("files required")
	}

	args := append([]string{"drop"}, files...)
	output, err := a.ExecuteCommand(ctx, "aider", args...)
	if err != nil {
		return nil, fmt.Errorf("aider drop failed: %w", err)
	}

	return map[string]interface{}{
		"response": string(output),
		"success":  true,
	}, nil
}

// listFiles lists files in the chat context
func (a *Aider) listFiles(ctx context.Context) (interface{}, error) {
	output, err := a.ExecuteCommand(ctx, "aider", "ls")
	if err != nil {
		return nil, fmt.Errorf("aider ls failed: %w", err)
	}

	return map[string]interface{}{
		"files":   strings.Split(strings.TrimSpace(string(output)), "\n"),
		"success": true,
	}, nil
}

// buildArgs builds command-line arguments
func (a *Aider) buildArgs() []string {
	var args []string

	if a.config.EditorModel != "" {
		args = append(args, "--editor-model", a.config.EditorModel)
	}

	if a.config.ArchitectMode {
		args = append(args, "--architect")
	}

	if !a.config.AutoCommits {
		args = append(args, "--no-auto-commits")
	}

	if !a.config.AutoLint {
		args = append(args, "--no-auto-lint")
	}

	if a.config.AutoTest {
		args = append(args, "--auto-test")
	}

	if a.config.DarkMode {
		args = append(args, "--dark-mode")
	}

	if !a.config.ShowDiffs {
		args = append(args, "--no-show-diffs")
	}

	if a.config.GitIgnore {
		args = append(args, "--gitignore")
	}

	return args
}

// IsAvailable checks if Aider is installed
func (a *Aider) IsAvailable() bool {
	_, err := exec.LookPath("aider")
	return err == nil
}

// GetRepoMap generates a repository map
func (a *Aider) GetRepoMap(ctx context.Context, repoPath string) (map[string]interface{}, error) {
	// Check if repo-map is available
	rmPath := filepath.Join(repoPath, ".aider", "repo-map.json")

	if _, err := os.Stat(rmPath); err == nil {
		// Read existing repo map
		data, err := os.ReadFile(rmPath)
		if err != nil {
			return nil, err
		}

		var repoMap map[string]interface{}
		if err := json.Unmarshal(data, &repoMap); err != nil {
			return nil, err
		}

		return repoMap, nil
	}

	// Generate new repo map
	return a.generateRepoMap(ctx, repoPath)
}

// generateRepoMap generates a new repository map
func (a *Aider) generateRepoMap(ctx context.Context, repoPath string) (map[string]interface{}, error) {
	// Find all source files
	files := make(map[string]interface{})

	extensions := []string{"*.go", "*.py", "*.js", "*.ts", "*.java", "*.rs", "*.c", "*.cpp", "*.h"}

	for _, ext := range extensions {
		matches, _ := filepath.Glob(filepath.Join(repoPath, "**", ext))
		for _, match := range matches {
			rel, _ := filepath.Rel(repoPath, match)
			files[rel] = map[string]interface{}{
				"path": rel,
				"type": "source",
			}
		}
	}

	return map[string]interface{}{
		"files":     files,
		"generated": true,
	}, nil
}

// Ensure Aider implements AgentIntegration
var _ agents.AgentIntegration = (*Aider)(nil)
