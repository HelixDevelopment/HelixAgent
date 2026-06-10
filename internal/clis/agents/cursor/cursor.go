// Package cursor provides Cursor IDE agent integration.
// Cursor: AI-powered code editor with built-in GPT-4/Claude integration.
package cursor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// Cursor provides Cursor IDE integration.
//
// sessions is a safe.Slice[ChatSession] — all mutations (append on
// createSession, in-place Messages extension on chat) route through
// UpdateAt/Append under the Slice's write lock, closing the BUGFIX
// #30 race on concurrent Execute() callers.
type Cursor struct {
	*base.BaseIntegration
	config   *Config
	sessions *safe.Slice[ChatSession]
	nextID   atomic.Int64
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
		sessions: safe.NewSlice[ChatSession](),
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

	var loaded []ChatSession
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	c.sessions.Replace(loaded)
	c.nextID.Store(int64(len(loaded)))
	return nil
}

// saveSessions saves chat sessions
func (c *Cursor) saveSessions() error {
	sessionsPath := filepath.Join(c.GetWorkDir(), "sessions.json")
	snapshot := c.sessions.Snapshot()
	data, err := json.MarshalIndent(snapshot, "", "  ")
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

// errNoHeadlessCLI is returned by every AI command: Cursor is a GUI IDE
// (built on VS Code) with NO official headless coding-agent CLI to exec. Per
// the 2026-06-10 blocked-agent research there is no scriptable single-prompt
// invocation, so fabricating an AI response would be a BLUFF-001 violation.
// Session CRUD (create_session / list_sessions) is genuine local state and
// remains functional; only the AI-generation commands return this error.
var errNoHeadlessCLI = fmt.Errorf(
	"cursor is a GUI IDE with no headless CLI; AI commands require the Cursor " +
		"editor — this dispatch cannot generate without fabricating")

// chat records the user message into the session then returns an honest
// no-headless-CLI error instead of a fabricated assistant response (BLUFF-001).
func (c *Cursor) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	sessionID, _ := params["session_id"].(string)

	// Persist only the real user message — never a fabricated assistant turn.
	// UpdateAt runs the mutation under the Slice's write lock, so the append is
	// atomic with respect to concurrent chats and createSessions.
	c.sessions.UpdateAt(
		func(s ChatSession) bool { return s.ID == sessionID },
		func(s ChatSession) ChatSession {
			s.Messages = append(s.Messages, Message{
				Role:    "user",
				Content: message,
			})
			return s
		},
	)
	_ = c.saveSessions()

	return nil, errNoHeadlessCLI
}

// edit validates input then returns an honest no-headless-CLI error.
func (c *Cursor) edit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	file, _ := params["file"].(string)
	instruction, _ := params["instruction"].(string)

	if file == "" || instruction == "" {
		return nil, fmt.Errorf("file and instruction required")
	}

	return nil, errNoHeadlessCLI
}

// generate validates input then returns an honest no-headless-CLI error.
func (c *Cursor) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)

	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return nil, errNoHeadlessCLI
}

// explain validates input then returns an honest no-headless-CLI error.
func (c *Cursor) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	return nil, errNoHeadlessCLI
}

// terminal validates input then returns an honest no-headless-CLI error.
func (c *Cursor) terminal(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("command required")
	}

	return nil, errNoHeadlessCLI
}

// composer validates input then returns an honest no-headless-CLI error.
func (c *Cursor) composer(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return nil, errNoHeadlessCLI
}

// createSession creates a new chat session
func (c *Cursor) createSession(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		name = "New Session"
	}

	context, _ := params["context"].(string)

	// Atomic ID generation from the counter keyed off Len() after
	// loadSessions — guarantees no duplicate IDs across concurrent
	// createSession calls.
	id := c.nextID.Add(1)
	session := ChatSession{
		ID:       fmt.Sprintf("session-%d", id),
		Name:     name,
		Context:  context,
		Messages: []Message{},
		Status:   "active",
	}
	c.sessions.Append(session)

	if err := c.saveSessions(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"session": session,
		"status":  "created",
	}, nil
}

// listSessions lists all sessions. Snapshot returns a copy safe to
// iterate outside the lock.
func (c *Cursor) listSessions(ctx context.Context) (interface{}, error) {
	snapshot := c.sessions.Snapshot()
	return map[string]interface{}{
		"sessions": snapshot,
		"count":    len(snapshot),
	}, nil
}

// IsAvailable checks availability
func (c *Cursor) IsAvailable() bool {
	return c.config.EditorPath != "" || c.config.AIProvider != ""
}

var _ agents.AgentIntegration = (*Cursor)(nil)
