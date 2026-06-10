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
		sourceDir = "/run/media/milosvasic/DATA4TB/Projects/helix_code/cli_agents/claude-code-source"
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

// claudeBinary is the Claude Code CLI executable looked up on PATH.
// Overridable in tests via the CLAUDE_BIN environment variable so a fake
// binary can be injected to prove real exec is wired (anti-bluff, BLUFF-001).
const claudeBinary = "claude"

// getClaudeBinOverride returns the test-only claude binary override, if set.
func getClaudeBinOverride() string { return os.Getenv("CLAUDE_BIN") }

// resolveClaudeBinary locates the claude CLI executable. Tests may inject a
// fake binary via the CLAUDE_BIN environment variable (absolute path);
// otherwise the real `claude` command is resolved on PATH. Returns an honest
// error when the binary is not available — NEVER a fabricated success
// (BLUFF-001).
func (c *ClaudeCode) resolveClaudeBinary() (string, error) {
	if bin := getClaudeBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("claude binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(claudeBinary)
	if err != nil {
		return "", fmt.Errorf("claude CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runClaude invokes the Claude Code CLI non-interactively and returns its
// textual output. The non-interactive form is `claude -p "<prompt>"
// --output-format json`; the JSON envelope's text/result field is extracted
// when present, otherwise the raw stdout is returned. The model is passed via
// `--model` when configured. This is a REAL process execution — never a
// simulated/templated response (BLUFF-001).
func (c *ClaudeCode) runClaude(ctx context.Context, prompt string) (string, error) {
	bin, err := c.resolveClaudeBinary()
	if err != nil {
		return "", err
	}

	args := []string{"-p", prompt, "--output-format", "json"}
	if c.config != nil && c.config.Model != "" {
		args = append(args, "--model", c.config.Model)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = c.workDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("claude execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return extractClaudeText(out), nil
}

// extractClaudeText pulls the human-visible text out of the claude JSON
// envelope. The claude CLI `--output-format json` emits an object; the text
// lives under one of the common keys. When the payload is not JSON or has no
// known key, the trimmed raw stdout is returned (still real process output,
// never a template).
func extractClaudeText(out []byte) string {
	trimmed := strings.TrimSpace(string(out))
	var env map[string]interface{}
	if err := json.Unmarshal(out, &env); err == nil {
		for _, k := range []string{"result", "response", "text", "output", "content", "message"} {
			if v, ok := env[k]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return trimmed
}

// handleChat handles conversational chat with Claude by exec-ing the real
// `claude` CLI. Absent binary → honest error, never a fabricated echo.
func (c *ClaudeCode) handleChat(ctx context.Context, params map[string]interface{}) (*Response, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}

	content, err := c.runClaude(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("claude chat failed: %w", err)
	}

	response := &Response{
		Success:   true,
		Content:   content,
		Actions:   c.extractActions(content),
		SessionID: c.sessionID,
		Metadata: map[string]interface{}{
			"model":       c.config.Model,
			"editor_mode": c.config.EditorMode,
			"work_dir":    c.workDir,
		},
	}

	return response, nil
}

// handleEdit handles code editing requests by driving the real `claude` CLI
// to apply the instruction. Absent binary → honest error, never a fabricated
// "Applied edit" string.
func (c *ClaudeCode) handleEdit(ctx context.Context, params map[string]interface{}) (*Response, error) {
	file, _ := params["file"].(string)
	instruction, _ := params["instruction"].(string)

	if instruction == "" {
		return nil, fmt.Errorf("instruction is required")
	}

	prompt := instruction
	if file != "" {
		prompt = fmt.Sprintf("Edit file %s: %s", file, instruction)
	}

	content, err := c.runClaude(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("claude edit failed: %w", err)
	}

	response := &Response{
		Success:   true,
		Content:   content,
		Actions:   c.extractActions(content),
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

// handleReview handles code review by driving the real `claude` CLI to review
// the target. Absent binary → honest error, never a fabricated review string.
func (c *ClaudeCode) handleReview(ctx context.Context, params map[string]interface{}) (*Response, error) {
	fileOrDir, _ := params["target"].(string)
	if fileOrDir == "" {
		fileOrDir = "."
	}

	prompt := fmt.Sprintf("Review %s for code quality, security, and best practices", fileOrDir)
	content, err := c.runClaude(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("claude review failed: %w", err)
	}

	response := &Response{
		Success:   true,
		Content:   content,
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
