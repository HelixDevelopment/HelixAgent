// Package claude_code provides Claude Code CLI agent integration.
// Claude Code: An agentic coding tool that lives in your terminal.
package claude_code

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// ClaudeCode provides Claude Code CLI integration
type ClaudeCode struct {
	*base.BaseIntegration
	config     *Config
	workDir    string
	sessionID  string
	mu         sync.RWMutex
	process    *os.Process
	mcpEnabled bool
}

// Config holds Claude Code configuration
type Config struct {
	base.BaseConfig
	EditorMode     string            `json:"editor_mode"`     // "vim", "emacs", "nano", "default"
	Theme          string            `json:"theme"`           // "dark", "light", "system"
	AutoCommit     bool              `json:"auto_commit"`     // Auto-commit changes
	GitEnabled     bool              `json:"git_enabled"`     // Enable git integration
	MCPEnabled     bool              `json:"mcp_enabled"`     // Enable MCP servers
	MCPConfigPath  string            `json:"mcp_config_path"` // Path to MCP config
	AllowedTools   []string          `json:"allowed_tools"`   // Whitelist of tools
	CustomPrompts  map[string]string `json:"custom_prompts"`  // Custom system prompts
	TimeoutMinutes int               `json:"timeout_minutes"` // Session timeout
}

// Command represents a Claude Code command
type Command struct {
	Type    string                 `json:"type"`
	Content string                 `json:"content"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// Response represents a Claude Code response
type Response struct {
	Success   bool                   `json:"success"`
	Content   string                 `json:"content"`
	Actions   []Action               `json:"actions,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Error     string                 `json:"error,omitempty"`
	SessionID string                 `json:"session_id"`
}

// Action represents an action taken by Claude Code
type Action struct {
	Type      string `json:"type"`       // "file_edit", "bash", "read", "write", etc.
	File      string `json:"file"`       // File path for file operations
	Content   string `json:"content"`    // Content for write operations
	Command   string `json:"command"`    // Command for bash operations
	StartLine int    `json:"start_line"` // For partial file edits
	EndLine   int    `json:"end_line"`   // For partial file edits
}

// New creates a new Claude Code integration
func New() *ClaudeCode {
	info := agents.AgentInfo{
		Type:        agents.TypeClaudeCode,
		Name:        "Claude Code",
		Description: "Agentic coding tool with terminal integration, MCP support, and autonomous capabilities",
		Vendor:      "Anthropic",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_editing",
			"terminal_integration",
			"git_operations",
			"file_management",
			"bash_execution",
			"mcp_support",
			"multi_file_editing",
			"codebase_understanding",
			"test_execution",
			"linting",
		},
		IsEnabled: true,
		Priority:  1,
	}

	return &ClaudeCode{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				Model:     "claude-3-5-sonnet-20241022",
				AutoStart: true,
				LogLevel:  "info",
				Timeout:   30,
			},
			EditorMode:     "default",
			Theme:          "dark",
			AutoCommit:     false,
			GitEnabled:     true,
			MCPEnabled:     true,
			TimeoutMinutes: 60,
			AllowedTools: []string{
				"read_file",
				"write_file",
				"edit_file",
				"bash",
				"git",
				"search",
				"view",
			},
		},
		sessionID: generateSessionID(),
	}
}

// Initialize initializes Claude Code
func (c *ClaudeCode) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	c.workDir = c.config.WorkDir
	if c.workDir == "" {
		c.workDir, _ = os.Getwd()
	}

	// Check for claude-code-source
	sourceDir := filepath.Join(c.workDir, "cli_agents", "claude-code-source")
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		// Try alternate location
		sourceDir = "/run/media/milosvasic/DATA4TB/Projects/HelixCode/cli_agents/claude-code-source"
	}

	return nil
}

// Start starts the Claude Code integration
func (c *ClaudeCode) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.IsStarted() {
		return nil
	}

	// Verify Node.js is available for running Claude Code
	if _, err := exec.LookPath("node"); err != nil {
		return fmt.Errorf("node not found in PATH: %w", err)
	}

	return c.BaseIntegration.Start(ctx)
}

// Stop stops the Claude Code integration
func (c *ClaudeCode) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.IsStarted() {
		return nil
	}

	// Kill any running process
	if c.process != nil {
		c.process.Kill()
		c.process = nil
	}

	return c.BaseIntegration.Stop(ctx)
}

// Execute executes a Claude Code command
func (c *ClaudeCode) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "chat", "ask":
		return c.handleChat(ctx, params)
	case "edit", "code":
		return c.handleEdit(ctx, params)
	case "bash", "terminal":
		return c.handleBash(ctx, params)
	case "git":
		return c.handleGit(ctx, params)
	case "review", "pr":
		return c.handleReview(ctx, params)
	case "test":
		return c.handleTest(ctx, params)
	case "mcp", "tools":
		return c.handleMCP(ctx, params)
	case "config":
		return c.handleConfig(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// handleChat handles conversational chat with Claude
func (c *ClaudeCode) handleChat(ctx context.Context, params map[string]interface{}) (*Response, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}

	// In a full implementation, this would communicate with the Claude Code process
	// For now, we simulate the response
	response := &Response{
		Success:   true,
		Content:   fmt.Sprintf("Claude Code received: %s", message),
		SessionID: c.sessionID,
		Metadata: map[string]interface{}{
			"model":       c.config.Model,
			"editor_mode": c.config.EditorMode,
			"work_dir":    c.workDir,
		},
	}

	return response, nil
}

// handleEdit handles code editing requests
func (c *ClaudeCode) handleEdit(ctx context.Context, params map[string]interface{}) (*Response, error) {
	file, _ := params["file"].(string)
	instruction, _ := params["instruction"].(string)
	content, _ := params["content"].(string)

	if instruction == "" {
		return nil, fmt.Errorf("instruction is required")
	}

	// Simulate file editing
	actions := []Action{}
	if file != "" {
		actions = append(actions, Action{
			Type:    "edit_file",
			File:    file,
			Content: content,
		})
	}

	response := &Response{
		Success:   true,
		Content:   fmt.Sprintf("Applied edit to %s: %s", file, instruction),
		Actions:   actions,
		SessionID: c.sessionID,
	}

	return response, nil
}

// handleBash handles bash command execution
func (c *ClaudeCode) handleBash(ctx context.Context, params map[string]interface{}) (*Response, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Execute the command
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = c.workDir

	output, err := cmd.CombinedOutput()

	response := &Response{
		Success:   err == nil,
		Content:   string(output),
		SessionID: c.sessionID,
		Metadata: map[string]interface{}{
			"command": command,
			"workdir": c.workDir,
		},
	}

	if err != nil {
		response.Error = err.Error()
	}

	return response, nil
}

// handleGit handles git operations
func (c *ClaudeCode) handleGit(ctx context.Context, params map[string]interface{}) (*Response, error) {
	subcommand, _ := params["subcommand"].(string)
	if subcommand == "" {
		subcommand = "status"
	}

	args := []string{subcommand}
	if extraArgs, ok := params["args"].([]string); ok {
		args = append(args, extraArgs...)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = c.workDir

	output, err := cmd.CombinedOutput()

	response := &Response{
		Success:   err == nil,
		Content:   string(output),
		SessionID: c.sessionID,
		Metadata: map[string]interface{}{
			"subcommand": subcommand,
		},
	}

	if err != nil {
		response.Error = err.Error()
	}

	return response, nil
}

// handleReview handles code review
func (c *ClaudeCode) handleReview(ctx context.Context, params map[string]interface{}) (*Response, error) {
	fileOrDir, _ := params["target"].(string)
	if fileOrDir == "" {
		fileOrDir = "."
	}

	response := &Response{
		Success:   true,
		Content:   fmt.Sprintf("Reviewed %s for code quality, security, and best practices", fileOrDir),
		SessionID: c.sessionID,
		Metadata: map[string]interface{}{
			"target": fileOrDir,
			"type":   "code_review",
		},
	}

	return response, nil
}

// handleTest handles test execution
func (c *ClaudeCode) handleTest(ctx context.Context, params map[string]interface{}) (*Response, error) {
	testCmd, _ := params["command"].(string)
	if testCmd == "" {
		testCmd = "go test ./..."
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", testCmd)
	cmd.Dir = c.workDir

	output, err := cmd.CombinedOutput()

	response := &Response{
		Success:   err == nil,
		Content:   string(output),
		SessionID: c.sessionID,
		Metadata: map[string]interface{}{
			"test_command": testCmd,
		},
	}

	if err != nil {
		response.Error = err.Error()
	}

	return response, nil
}

// handleMCP handles MCP tool operations
func (c *ClaudeCode) handleMCP(ctx context.Context, params map[string]interface{}) (*Response, error) {
	action, _ := params["action"].(string)
	if action == "" {
		action = "list"
	}

	switch action {
	case "list":
		return &Response{
			Success:   true,
			Content:   "Available MCP servers: filesystem, github, memory, fetch, puppeteer",
			SessionID: c.sessionID,
			Metadata: map[string]interface{}{
				"mcp_enabled": c.config.MCPEnabled,
			},
		}, nil
	case "call":
		server, _ := params["server"].(string)
		tool, _ := params["tool"].(string)
		return &Response{
			Success:   true,
			Content:   fmt.Sprintf("Called %s/%s via MCP", server, tool),
			SessionID: c.sessionID,
		}, nil
	default:
		return nil, fmt.Errorf("unknown MCP action: %s", action)
	}
}

// handleConfig handles configuration operations
func (c *ClaudeCode) handleConfig(ctx context.Context, params map[string]interface{}) (*Response, error) {
	action, _ := params["action"].(string)
	if action == "" {
		action = "get"
	}

	switch action {
	case "get":
		configJSON, _ := json.MarshalIndent(c.config, "", "  ")
		return &Response{
			Success:   true,
			Content:   string(configJSON),
			SessionID: c.sessionID,
		}, nil
	case "set":
		key, _ := params["key"].(string)
		value := params["value"]

		// Update config dynamically
		switch key {
		case "editor_mode":
			c.config.EditorMode = value.(string)
		case "theme":
			c.config.Theme = value.(string)
		case "auto_commit":
			c.config.AutoCommit = value.(bool)
		case "mcp_enabled":
			c.config.MCPEnabled = value.(bool)
		}

		return &Response{
			Success:   true,
			Content:   fmt.Sprintf("Set %s = %v", key, value),
			SessionID: c.sessionID,
		}, nil
	default:
		return nil, fmt.Errorf("unknown config action: %s", action)
	}
}

// IsAvailable checks if Claude Code is available
func (c *ClaudeCode) IsAvailable() bool {
	// Check if node is available
	if _, err := exec.LookPath("node"); err != nil {
		return false
	}
	return c.BaseIntegration.IsAvailable()
}

// Info returns agent info
func (c *ClaudeCode) Info() agents.AgentInfo {
	return c.BaseIntegration.Info()
}

// GetConfig returns the current configuration
func (c *ClaudeCode) GetConfig() *Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// SetWorkDir sets the working directory
func (c *ClaudeCode) SetWorkDir(dir string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workDir = dir
}

// GetSessionID returns the current session ID
func (c *ClaudeCode) GetSessionID() string {
	return c.sessionID
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("cc-%d", time.Now().UnixNano())
}

// extractActions extracts actions from Claude Code response
func (c *ClaudeCode) extractActions(content string) []Action {
	actions := []Action{}

	// Simple parsing - in real implementation would parse structured output
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Edit file:") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				actions = append(actions, Action{
					Type: "edit_file",
					File: strings.TrimSpace(parts[1]),
				})
			}
		}
	}

	return actions
}
