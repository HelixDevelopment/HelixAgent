// Package openhands provides OpenHands CLI agent integration.
// OpenHands: AI-powered software development agent with sandboxed execution.
package openhands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// OpenHands provides OpenHands CLI integration
type OpenHands struct {
	*base.BaseIntegration
	config *Config
}

// Config holds OpenHands configuration
type Config struct {
	base.BaseConfig
	SandboxType   string // "docker", "local", "none"
	WorkspaceDir  string
	LLMModel      string
	AgentName     string
	MaxIterations int
	AutoConfirm   bool
}

// New creates a new OpenHands integration
func New() *OpenHands {
	info := agents.AgentInfo{
		Type:        agents.TypeOpenHands,
		Name:        "OpenHands",
		Description: "AI software development agent with sandboxed execution",
		Vendor:      "OpenHands",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_development",
			"sandboxed_execution",
			"multi_step_tasks",
			"web_browsing",
			"file_operations",
			"terminal_commands",
			"github_integration",
			"iterative_refinement",
		},
		IsEnabled: true,
		Priority:  1,
	}

	return &OpenHands{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				Model:     "claude-3-sonnet",
				AutoStart: true,
			},
			SandboxType:   "docker",
			WorkspaceDir:  "",
			LLMModel:      "claude-3-sonnet-20240229",
			AgentName:     "CodeActAgent",
			MaxIterations: 100,
			AutoConfirm:   false,
		},
	}
}

// Initialize initializes OpenHands
func (o *OpenHands) Initialize(ctx context.Context, config interface{}) error {
	if err := o.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		o.config = cfg
	}

	// Set default workspace
	if o.config.WorkspaceDir == "" {
		o.config.WorkspaceDir = o.GetWorkDir()
	}

	return nil
}

// Execute executes an OpenHands command
func (o *OpenHands) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !o.IsStarted() {
		if err := o.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "start_task":
		return o.startTask(ctx, params)
	case "run":
		return o.run(ctx, params)
	case "eval":
		return o.eval(ctx, params)
	case "sandbox":
		return o.sandboxCmd(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// startTask starts a new development task
func (o *OpenHands) startTask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	task, _ := params["task"].(string)
	if task == "" {
		return nil, fmt.Errorf("task description required")
	}

	// Create task file
	taskFile := filepath.Join(o.GetWorkDir(), "task.txt")
	if err := os.WriteFile(taskFile, []byte(task), 0644); err != nil {
		return nil, fmt.Errorf("write task file: %w", err)
	}

	// Build command args
	args := o.buildArgs()
	args = append(args, "-t", task)

	output, err := o.ExecuteCommand(ctx, "openhands", args...)
	if err != nil {
		return nil, fmt.Errorf("openhands task failed: %w\nOutput: %s", err, string(output))
	}

	return map[string]interface{}{
		"task":     task,
		"response": string(output),
		"success":  true,
	}, nil
}

// run runs OpenHands with a specific configuration
func (o *OpenHands) run(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	args := o.buildArgs()

	// Add any additional params
	if evalFile, ok := params["eval_file"].(string); ok {
		args = append(args, "-e", evalFile)
	}

	output, err := o.ExecuteCommand(ctx, "openhands", args...)
	if err != nil {
		return nil, fmt.Errorf("openhands run failed: %w\nOutput: %s", err, string(output))
	}

	return map[string]interface{}{
		"response": string(output),
		"success":  true,
	}, nil
}

// eval runs evaluation
func (o *OpenHands) eval(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	evalFile, _ := params["file"].(string)
	if evalFile == "" {
		return nil, fmt.Errorf("eval file required")
	}

	args := []string{"eval", "-f", evalFile}

	output, err := o.ExecuteCommand(ctx, "openhands", args...)
	if err != nil {
		return nil, fmt.Errorf("openhands eval failed: %w", err)
	}

	return map[string]interface{}{
		"response": string(output),
		"success":  true,
	}, nil
}

// sandboxCmd executes a sandbox command
func (o *OpenHands) sandboxCmd(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sandboxCmd, _ := params["cmd"].(string)
	if sandboxCmd == "" {
		return nil, fmt.Errorf("sandbox command required")
	}

	// Execute in sandbox
	output, err := o.executeInSandbox(ctx, sandboxCmd)
	if err != nil {
		return nil, fmt.Errorf("sandbox execution failed: %w", err)
	}

	return map[string]interface{}{
		"command": sandboxCmd,
		"output":  string(output),
		"success": true,
	}, nil
}

// executeInSandbox executes a command in the OpenHands sandbox
func (o *OpenHands) executeInSandbox(ctx context.Context, cmd string) ([]byte, error) {
	switch o.config.SandboxType {
	case "docker":
		return o.executeInDocker(ctx, cmd)
	case "local":
		return o.ExecuteCommand(ctx, "sh", "-c", cmd)
	default:
		return nil, fmt.Errorf("unsupported sandbox type: %s", o.config.SandboxType)
	}
}

// executeInDocker executes a command in Docker sandbox
func (o *OpenHands) executeInDocker(ctx context.Context, cmd string) ([]byte, error) {
	dockerArgs := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/workspace", o.config.WorkspaceDir),
		"-w", "/workspace",
		"openhands/sandbox:latest",
		"sh", "-c", cmd,
	}

	return exec.CommandContext(ctx, "docker", dockerArgs...).CombinedOutput()
}

// buildArgs builds command-line arguments
func (o *OpenHands) buildArgs() []string {
	var args []string

	if o.config.LLMModel != "" {
		args = append(args, "-m", o.config.LLMModel)
	}

	if o.config.AgentName != "" {
		args = append(args, "-a", o.config.AgentName)
	}

	if o.config.WorkspaceDir != "" {
		args = append(args, "-w", o.config.WorkspaceDir)
	}

	if o.config.MaxIterations > 0 {
		args = append(args, "-i", fmt.Sprintf("%d", o.config.MaxIterations))
	}

	if o.config.AutoConfirm {
		args = append(args, "--auto-confirm")
	}

	return args
}

// IsAvailable checks if OpenHands is installed
func (o *OpenHands) IsAvailable() bool {
	_, err := exec.LookPath("openhands")
	return err == nil
}

// SetupSandbox sets up the sandbox environment
func (o *OpenHands) SetupSandbox(ctx context.Context) error {
	if o.config.SandboxType != "docker" {
		return nil
	}

	// Check if Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not available")
	}

	// Check if sandbox image exists
	cmd := exec.CommandContext(ctx, "docker", "images", "-q", "openhands/sandbox:latest")
	output, _ := cmd.Output()

	if len(output) == 0 {
		// Pull sandbox image
		pullCmd := exec.CommandContext(ctx, "docker", "pull", "openhands/sandbox:latest")
		if output, err := pullCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to pull sandbox image: %w\n%s", err, string(output))
		}
	}

	return nil
}

// GetSandboxStatus returns the sandbox status
func (o *OpenHands) GetSandboxStatus() map[string]interface{} {
	return map[string]interface{}{
		"type":      o.config.SandboxType,
		"workspace": o.config.WorkspaceDir,
		"available": o.IsAvailable(),
	}
}

// Ensure OpenHands implements AgentIntegration
var _ agents.AgentIntegration = (*OpenHands)(nil)
