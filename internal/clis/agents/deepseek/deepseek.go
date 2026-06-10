// Package deepseek provides DeepSeek agent integration.
// DeepSeek: High-performance code intelligence model.
//
// DeepSeek ships NO official headless coding-agent CLI binary — it is an LLM
// model family served through the DeepSeek provider HTTP API (or via Ollama for
// the open-weight variants). Per the 2026-06-10 currency/blocked-agent research
// there is no per-agent CLI to exec, so the generation commands return an honest
// error pointing at the provider path rather than a fabricated response
// (BLUFF-001). Real DeepSeek generation flows through HelixCode's provider
// abstraction (CONST-039), not this CLI-agent dispatch stub.
package deepseek

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// errNoHeadlessCLI is returned by every generation command: DeepSeek has no
// official headless coding-agent CLI to exec, so a fabricated text response
// would be a BLUFF-001 violation. Callers MUST route DeepSeek through the
// HelixCode provider API (CONST-039) instead.
var errNoHeadlessCLI = fmt.Errorf(
	"deepseek has no official headless CLI binary; use the DeepSeek provider API " +
		"(CONST-039) — this CLI-agent dispatch cannot generate without fabricating")

// DeepSeek provides DeepSeek integration
type DeepSeek struct {
	*base.BaseIntegration
	config *Config
}

// Config holds DeepSeek configuration
type Config struct {
	base.BaseConfig
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
}

// New creates a new DeepSeek integration
func New() *DeepSeek {
	info := agents.AgentInfo{
		Type:        agents.TypeDeepSeek,
		Name:        "DeepSeek",
		Description: "High-performance code intelligence",
		Vendor:      "DeepSeek",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_generation",
			"code_completion",
			"code_explanation",
			"chat",
			"reasoning",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &DeepSeek{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model:       "deepseek-coder-v3",
			MaxTokens:   8192,
			Temperature: 0.7,
		},
	}
}

// Initialize initializes DeepSeek
func (d *DeepSeek) Initialize(ctx context.Context, config interface{}) error {
	if err := d.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		d.config = cfg
	}

	return nil
}

// Execute executes a command
func (d *DeepSeek) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !d.IsStarted() {
		if err := d.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "chat":
		return d.chat(ctx, params)
	case "generate":
		return d.generate(ctx, params)
	case "complete":
		return d.complete(ctx, params)
	case "explain":
		return d.explain(ctx, params)
	case "status":
		return d.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// chat validates input then returns an honest no-headless-CLI error rather than
// a fabricated response (BLUFF-001).
func (d *DeepSeek) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}
	return nil, errNoHeadlessCLI
}

// generate validates input then returns an honest no-headless-CLI error.
func (d *DeepSeek) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}
	return nil, errNoHeadlessCLI
}

// complete validates input then returns an honest no-headless-CLI error.
func (d *DeepSeek) complete(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prefix, _ := params["prefix"].(string)
	if prefix == "" {
		return nil, fmt.Errorf("prefix required")
	}
	return nil, errNoHeadlessCLI
}

// explain validates input then returns an honest no-headless-CLI error.
func (d *DeepSeek) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}
	return nil, errNoHeadlessCLI
}

// status returns status
func (d *DeepSeek) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":  d.IsAvailable(),
		"model":      d.config.Model,
		"max_tokens": d.config.MaxTokens,
	}, nil
}

// IsAvailable checks availability
func (d *DeepSeek) IsAvailable() bool {
	return d.config.APIKey != ""
}

var _ agents.AgentIntegration = (*DeepSeek)(nil)
