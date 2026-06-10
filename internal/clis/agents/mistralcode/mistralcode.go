// Package mistralcode provides Mistral Code agent integration.
// Mistral Code: AI coding assistant by Mistral AI.
//
// Mistral Code ships as an IDE/editor extension (VS Code / JetBrains), NOT as a
// headless command-line agent binary. There is therefore no CLI process to
// exec, and the project does not embed a Mistral API client here. Rather than
// fabricate code/completions/chat replies (the BLUFF-001/003 anti-pattern), the
// generation commands return an HONEST error stating the integration is not
// wired to a real backend.
package mistralcode

import (
	"context"
	"errors"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// ErrNoBackend is returned by generation commands because Mistral Code has no
// headless CLI to exec and no real API client is wired here. Returning a
// fabricated success instead would be a BLUFF-001/003 violation (CONST-035).
var ErrNoBackend = errors.New(
	"mistralcode: no real backend wired — Mistral Code ships as an IDE/editor " +
		"extension, not a headless CLI; refusing to fabricate a response")

// MistralCode provides Mistral Code integration
type MistralCode struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	APIKey string
	Model  string
}

// New creates a new Mistral Code integration
func New() *MistralCode {
	info := agents.AgentInfo{
		Type:        agents.TypeMistralCode,
		Name:        "Mistral Code",
		Description: "Mistral AI coding assistant",
		Vendor:      "Mistral AI",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_generation",
			"code_completion",
			"chat",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &MistralCode{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "codestral-latest",
		},
	}
}

// Initialize initializes Mistral Code
func (m *MistralCode) Initialize(ctx context.Context, config interface{}) error {
	if err := m.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		m.config = cfg
	}

	return nil
}

// Execute executes a command
func (m *MistralCode) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !m.IsStarted() {
		if err := m.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "complete":
		return m.complete(ctx, params)
	case "generate":
		return m.generate(ctx, params)
	case "chat":
		return m.chat(ctx, params)
	case "status":
		return m.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// complete returns an honest error — no real backend to produce a completion.
func (m *MistralCode) complete(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if _, ok := params["prefix"].(string); !ok || params["prefix"].(string) == "" {
		return nil, fmt.Errorf("prefix required")
	}
	return nil, ErrNoBackend
}

// generate returns an honest error — no real backend to produce code.
func (m *MistralCode) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}
	return nil, ErrNoBackend
}

// chat returns an honest error — no real backend to produce a reply.
func (m *MistralCode) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}
	return nil, ErrNoBackend
}

// status returns status
func (m *MistralCode) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": m.IsAvailable(),
		"model":     m.config.Model,
	}, nil
}

// IsAvailable checks availability
func (m *MistralCode) IsAvailable() bool {
	return m.config.APIKey != ""
}

var _ agents.AgentIntegration = (*MistralCode)(nil)
