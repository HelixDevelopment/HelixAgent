// Package gptengineer provides GPT Engineer agent integration.
// GPT Engineer is a project-scaffolding tool (`gpt-engineer <projectdir>`) that
// writes generated files into a project directory using an OPENAI_API_KEY — it
// is NOT a prompt→text headless responder, and this integration does not yet
// wire the real `gpt-engineer` CLI run. Rather than fabricate a fixed file list,
// generate/improve return an HONEST error (anti-bluff: BLUFF-001).
package gptengineer

import (
	"context"
	"errors"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// GPTEngineer provides GPT Engineer integration
type GPTEngineer struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Model string
}

// New creates a new GPT Engineer integration
func New() *GPTEngineer {
	info := agents.AgentInfo{
		Type:        agents.TypeGptEngineer,
		Name:        "GPT Engineer",
		Description: "AI software engineer",
		Vendor:      "GPT Engineer",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_generation",
			"project_creation",
			"architecture",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &GPTEngineer{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "gpt-4",
		},
	}
}

// Initialize initializes GPT Engineer
func (g *GPTEngineer) Initialize(ctx context.Context, config interface{}) error {
	if err := g.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		g.config = cfg
	}

	return nil
}

// Execute executes a command
func (g *GPTEngineer) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !g.IsStarted() {
		if err := g.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "generate":
		return g.generate(ctx, params)
	case "improve":
		return g.improve(ctx, params)
	case "status":
		return g.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// ErrCLINotWired is returned by generate/improve because the real gpt-engineer
// project-scaffolding run is not wired. Per CONST-035 / BLUFF-001 the
// integration returns this honest error instead of fabricating a fixed file list
// (["main.py", "README.md", ...]) or a "generated"/"improved" status that no
// real run produced.
var ErrCLINotWired = errors.New("gptengineer: the real `gpt-engineer` project-scaffolding run is not wired; " +
	"generate/improve require a real CLI invocation that writes files with an OPENAI_API_KEY — refusing to fabricate a file list")

// generate generates a project. Honest error: the real gpt-engineer run is not
// wired, so we refuse to fabricate a fixed file list (BLUFF-001).
func (g *GPTEngineer) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}
	return nil, fmt.Errorf("gptengineer generate: %w", ErrCLINotWired)
}

// improve improves code. Honest error: the real gpt-engineer run is not wired
// (BLUFF-001).
func (g *GPTEngineer) improve(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	file, _ := params["file"].(string)
	if file == "" {
		return nil, fmt.Errorf("file required")
	}
	return nil, fmt.Errorf("gptengineer improve: %w", ErrCLINotWired)
}

// status returns status
func (g *GPTEngineer) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": g.IsAvailable(),
		"model":     g.config.Model,
	}, nil
}

// IsAvailable checks availability
func (g *GPTEngineer) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*GPTEngineer)(nil)
