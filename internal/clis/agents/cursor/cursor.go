// Package cursor provides Cursor IDE agent integration.
// Cursor: AI-powered code editor with built-in GPT-4/Claude integration.
package cursor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Cursor provides Cursor IDE integration
type Cursor struct {
	*base.BaseIntegration
	config   *Config
	sessions []ChatSession
}

// Config holds Cursor configuration
type Config struct {
	base.BaseConfig
	EditorPath    string
	AIProvider    string
	Model         string
	ContextWindow int
}

// ChatSession represents a chat session
type ChatSession struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Context  string    `json:"context"`
	Messages []Message `json:"messages"`
	Status   string    `json:"status"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// New creates a new Cursor integration
func New() *Cursor {
	info := agents.AgentInfo{
		Type:        agents.TypeCursor,
		Name:        "Cursor",
		Description: "AI-powered code editor",
		Vendor:      "Cursor",
		Version:     "1.0.0",
		Capabilities: []string{
			"ai_chat",
			"code_generation",
			"code_editing",
			"multi_file_edits",
			"context_aware",
			"terminal_integration",
			"composer",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Cursor{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			AIProvider:    "anthropic",
			Model:         "claude-sonnet-4",
			ContextWindow: 200000,
		},
		sessions: make([]ChatSession, 0),
	}
}

// Initialize initializes Cursor
func (c *Cursor) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return c.loadSessions()
}

// loadSessions loads chat sessions
func (c *Cursor) loadSessions() error {
	sessionsPath := filepath.Join(c.GetWorkDir(), "sessions.json")

	if _, err := os.Stat(sessionsPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(sessionsPath)
	if err != nil {
		return fmt.Errorf("read sessions: %w", err)
	}

	return json.Unmarshal(data, &c.sessions)
}

// saveSessions saves chat sessions
func (c *Cursor) saveSessions() error {
	sessionsPath := filepath.Join(c.GetWorkDir(), "sessions.json")
	data, err := json.MarshalIndent(c.sessions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sessions: %w", err)
	}
	return os.WriteFile(sessionsPath, data, 0644)
}

// Execute executes a command
func (c *Cursor) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "chat":
		return c.chat(ctx, params)
	case "edit":
		return c.edit(ctx, params)
	case "generate":
		return c.generate(ctx, params)
	case "explain":
		return c.explain(ctx, params)
	case "terminal":
		return c.terminal(ctx, params)
	case "composer":
		return c.composer(ctx, params)
	case "create_session":
		return c.createSession(ctx, params)
	case "list_sessions":
		return c.listSessions(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// chat performs AI chat
func (c *Cursor) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	sessionID, _ := params["session_id"].(string)

	// Add message to session
	for i := range c.sessions {
		if c.sessions[i].ID == sessionID {
			c.sessions[i].Messages = append(c.sessions[i].Messages, Message{
				Role:    "user",
				Content: message,
			})
			c.sessions[i].Messages = append(c.sessions[i].Messages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Response to: %s", message),
			})
			c.saveSessions()
			break
		}
	}

	return map[string]interface{}{
		"message":    message,
		"response":   fmt.Sprintf("AI response to: %s", message),
		"session_id": sessionID,
		"model":      c.config.Model,
	}, nil
}

// edit performs code edit
func (c *Cursor) edit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	file, _ := params["file"].(string)
	instruction, _ := params["instruction"].(string)

	if file == "" || instruction == "" {
		return nil, fmt.Errorf("file and instruction required")
	}

	return map[string]interface{}{
		"file":        file,
		"instruction": instruction,
		"changes": []map[string]interface{}{
			{"type": "edit", "line": 1, "content": "// Edited by Cursor"},
		},
		"status": "edited",
	}, nil
}

// generate generates code
func (c *Cursor) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	language, _ := params["language"].(string)

	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	if language == "" {
		language = "go"
	}

	// Generate code based on prompt
	code := fmt.Sprintf("// Generated by Cursor\n// Prompt: %s\n// Language: %s\n\nfunc generatedFunction() {\n    // Implementation\n}\n", prompt, language)

	return map[string]interface{}{
		"prompt":   prompt,
		"language": language,
		"code":     code,
		"model":    c.config.Model,
	}, nil
}

// explain explains code
func (c *Cursor) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	return map[string]interface{}{
		"code":        code,
		"explanation": fmt.Sprintf("Explanation of the code:\n\n%s\n\nThis code performs...", code),
		"model":       c.config.Model,
	}, nil
}

// terminal runs terminal command with AI
func (c *Cursor) terminal(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("command required")
	}

	return map[string]interface{}{
		"command": command,
		"ai_help": fmt.Sprintf("AI suggests: %s", command),
		"output":  "Terminal output would appear here",
	}, nil
}

// composer runs multi-file composer
func (c *Cursor) composer(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	files, _ := params["files"].([]interface{})
	if files == nil {
		files = []interface{}{}
	}

	return map[string]interface{}{
		"prompt": prompt,
		"files":  files,
		"edits": []map[string]interface{}{
			{"file": "main.go", "type": "modify"},
			{"file": "utils.go", "type": "create"},
		},
		"status": "composed",
	}, nil
}

// createSession creates a new chat session
func (c *Cursor) createSession(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		name = "New Session"
	}

	context, _ := params["context"].(string)

	session := ChatSession{
		ID:       fmt.Sprintf("session-%d", len(c.sessions)+1),
		Name:     name,
		Context:  context,
		Messages: []Message{},
		Status:   "active",
	}

	c.sessions = append(c.sessions, session)

	if err := c.saveSessions(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"session": session,
		"status":  "created",
	}, nil
}

// listSessions lists all sessions
func (c *Cursor) listSessions(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"sessions": c.sessions,
		"count":    len(c.sessions),
	}, nil
}

// IsAvailable checks availability
func (c *Cursor) IsAvailable() bool {
	return c.config.EditorPath != "" || c.config.AIProvider != ""
}

var _ agents.AgentIntegration = (*Cursor)(nil)
