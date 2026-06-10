// Package jetbrainsai provides JetBrains AI Assistant integration.
// JetBrains AI is an assistant embedded INSIDE JetBrains IDEs (IntelliJ,
// GoLand, PyCharm, …) — there is NO headless `jetbrains-ai` CLI and no public
// standalone API. Its AI commands therefore cannot be driven from a back-end
// process; they return an HONEST error rather than a fabricated response
// (anti-bluff: BLUFF-001). Only IDE-host metadata (status/availability) is real.
package jetbrainsai

import (
	"context"
	"errors"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// ErrIDEOnly is returned by the AI commands because JetBrains AI runs only
// inside the IDE process — there is no headless CLI or public standalone API to
// drive completion/chat/generation from a back-end. Per CONST-035 / BLUFF-001
// the integration returns this honest error instead of fabricating output.
var ErrIDEOnly = errors.New("jetbrains-ai: assistant runs only inside the JetBrains IDE process; " +
	"no headless CLI or public standalone API exists to drive it from a back-end — refusing to fabricate a response")

// JetBrainsAI provides JetBrains AI integration
type JetBrainsAI struct {
	*base.BaseIntegration
	config *Config
}

// Config holds JetBrains AI configuration
type Config struct {
	base.BaseConfig
	IDEType      string // "intellij", "pycharm", "goland", "webstorm"
	Model        string
	EnableInline bool
	EnableChat   bool
}

// New creates a new JetBrains AI integration
func New() *JetBrainsAI {
	info := agents.AgentInfo{
		Type:        agents.TypeJetBrainsAI,
		Name:        "JetBrains AI",
		Description: "AI assistant for JetBrains IDEs",
		Vendor:      "JetBrains",
		Version:     "1.0.0",
		Capabilities: []string{
			"ide_integration",
			"inline_completion",
			"chat",
			"code_generation",
			"code_explanation",
			"test_generation",
			"documentation",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &JetBrainsAI{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			IDEType:      "intellij",
			Model:        "claude-3-sonnet",
			EnableInline: true,
			EnableChat:   true,
		},
	}
}

// Initialize initializes JetBrains AI
func (j *JetBrainsAI) Initialize(ctx context.Context, config interface{}) error {
	if err := j.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		j.config = cfg
	}

	return nil
}

// Execute executes a command
func (j *JetBrainsAI) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !j.IsStarted() {
		if err := j.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "complete":
		return j.complete(ctx, params)
	case "chat":
		return j.chat(ctx, params)
	case "generate":
		return j.generate(ctx, params)
	case "explain":
		return j.explain(ctx, params)
	case "test":
		return j.test(ctx, params)
	case "docs":
		return j.docs(ctx, params)
	case "status":
		return j.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// complete generates inline completion. Honest error: no headless backend
// exists to produce a real completion (BLUFF-001).
func (j *JetBrainsAI) complete(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("jetbrains-ai complete: %w", ErrIDEOnly)
}

// chat performs chat. Honest error: no headless backend exists (BLUFF-001).
func (j *JetBrainsAI) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}
	return nil, fmt.Errorf("jetbrains-ai chat: %w", ErrIDEOnly)
}

// generate generates code. Honest error: no headless backend exists (BLUFF-001).
func (j *JetBrainsAI) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}
	return nil, fmt.Errorf("jetbrains-ai generate: %w", ErrIDEOnly)
}

// explain explains code. Honest error: no headless backend exists (BLUFF-001).
func (j *JetBrainsAI) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}
	return nil, fmt.Errorf("jetbrains-ai explain: %w", ErrIDEOnly)
}

// test generates tests. Honest error: no headless backend exists (BLUFF-001).
func (j *JetBrainsAI) test(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}
	return nil, fmt.Errorf("jetbrains-ai test: %w", ErrIDEOnly)
}

// docs generates documentation. Honest error: no headless backend (BLUFF-001).
func (j *JetBrainsAI) docs(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}
	return nil, fmt.Errorf("jetbrains-ai docs: %w", ErrIDEOnly)
}

// status returns status
func (j *JetBrainsAI) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":      j.IsAvailable(),
		"ide":            j.config.IDEType,
		"inline_enabled": j.config.EnableInline,
		"chat_enabled":   j.config.EnableChat,
	}, nil
}

// IsAvailable checks availability
func (j *JetBrainsAI) IsAvailable() bool {
	return j.config.IDEType != ""
}

var _ agents.AgentIntegration = (*JetBrainsAI)(nil)
