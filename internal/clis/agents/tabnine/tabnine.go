// Package tabnine provides Tabnine agent integration.
// Tabnine: AI code completion with privacy-focused local models.
package tabnine

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// ErrNoHeadlessCLI is returned by completion/chat/review entrypoints that have
// no real backing capability. Tabnine ships as an IDE/editor plugin (VS Code,
// JetBrains, Vim, etc.) backed by a local/cloud model engine reached through the
// editor host — it exposes NO general headless command-line interface that turns
// a prompt into a completion/chat/review. Rather than fabricate templated text
// (BLUFF-001), these entrypoints return an honest error. See https://www.tabnine.com .
var ErrNoHeadlessCLI = fmt.Errorf("tabnine has no headless CLI: it ships as an IDE/editor plugin backed by an engine reached through the editor host, so completion/chat/review cannot be produced from this integration — refusing to fabricate output (BLUFF-001)")

// Tabnine provides Tabnine integration
type Tabnine struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Tabnine configuration
type Config struct {
	base.BaseConfig
	APIKey       string
	LocalMode    bool
	ModelType    string // "local", "cloud", "hybrid"
	TeamMode     bool
	PrivacyLevel string // "local", "team", "enterprise"
}

// New creates a new Tabnine integration
func New() *Tabnine {
	info := agents.AgentInfo{
		Type:        agents.TypeTabnine,
		Name:        "Tabnine",
		Description: "AI code completion with privacy focus",
		Vendor:      "Tabnine",
		Version:     "1.0.0",
		Capabilities: []string{
			"local_models",
			"privacy_focused",
			"team_learning",
			"code_completion",
			"chat",
			"code_review",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Tabnine{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			LocalMode:    true,
			ModelType:    "hybrid",
			TeamMode:     false,
			PrivacyLevel: "local",
		},
	}
}

// Initialize initializes Tabnine
func (t *Tabnine) Initialize(ctx context.Context, config interface{}) error {
	if err := t.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		t.config = cfg
	}

	return nil
}

// Execute executes a command
func (t *Tabnine) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !t.IsStarted() {
		if err := t.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "complete":
		return t.complete(ctx, params)
	case "chat":
		return t.chat(ctx, params)
	case "review":
		return t.review(ctx, params)
	case "status":
		return t.status(ctx)
	case "configure":
		return t.configure(ctx, params)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// complete returns an HONEST error: Tabnine has no headless CLI that can produce
// a real completion outside an editor host, so this integration refuses to
// fabricate one (BLUFF-001).
func (t *Tabnine) complete(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prefix, _ := params["prefix"].(string)
	if prefix == "" {
		return nil, fmt.Errorf("prefix required")
	}
	return nil, ErrNoHeadlessCLI
}

// chat returns an HONEST error: Tabnine Chat is an in-editor feature with no
// headless CLI; this integration refuses to fabricate a reply (BLUFF-001/003).
func (t *Tabnine) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}
	return nil, ErrNoHeadlessCLI
}

// review returns an HONEST error: Tabnine code review runs inside the editor
// host with no headless CLI; this integration refuses to fabricate review
// findings (BLUFF-001).
func (t *Tabnine) review(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}
	return nil, ErrNoHeadlessCLI
}

// status returns status
func (t *Tabnine) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":     t.IsAvailable(),
		"local_mode":    t.config.LocalMode,
		"model_type":    t.config.ModelType,
		"team_mode":     t.config.TeamMode,
		"privacy_level": t.config.PrivacyLevel,
	}, nil
}

// configure configures Tabnine
func (t *Tabnine) configure(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if localMode, ok := params["local_mode"].(bool); ok {
		t.config.LocalMode = localMode
	}
	if modelType, ok := params["model_type"].(string); ok {
		t.config.ModelType = modelType
	}
	if teamMode, ok := params["team_mode"].(bool); ok {
		t.config.TeamMode = teamMode
	}
	if privacyLevel, ok := params["privacy_level"].(string); ok {
		t.config.PrivacyLevel = privacyLevel
	}

	return map[string]interface{}{
		"configured":    true,
		"local_mode":    t.config.LocalMode,
		"model_type":    t.config.ModelType,
		"team_mode":     t.config.TeamMode,
		"privacy_level": t.config.PrivacyLevel,
	}, nil
}

// IsAvailable checks availability
func (t *Tabnine) IsAvailable() bool {
	return t.config.APIKey != "" || t.config.LocalMode
}

var _ agents.AgentIntegration = (*Tabnine)(nil)
