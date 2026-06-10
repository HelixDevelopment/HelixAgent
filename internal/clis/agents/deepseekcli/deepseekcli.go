// Package deepseekcli provides DeepSeek CLI agent integration.
// DeepSeek CLI: Command-line interface for DeepSeek models.
//
// There is NO official DeepSeek headless coding-agent CLI binary. DeepSeek is an
// LLM model family served through the DeepSeek provider HTTP API (or via Ollama
// for the open-weight variants). Per the 2026-06-10 currency/blocked-agent
// research there is no per-agent CLI to exec, so the generation commands return
// an honest error pointing at the provider path rather than a fabricated
// response (BLUFF-001). Real DeepSeek generation flows through HelixCode's
// provider abstraction (CONST-039), not this CLI-agent dispatch stub.
package deepseekcli

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// errNoHeadlessCLI is returned by every generation command: DeepSeek has no
// official headless CLI to exec, so a fabricated text response would be a
// BLUFF-001 violation. Callers MUST route DeepSeek through the HelixCode
// provider API (CONST-039) instead.
var errNoHeadlessCLI = fmt.Errorf(
	"deepseek has no official headless CLI binary; use the DeepSeek provider API " +
		"(CONST-039) — this CLI-agent dispatch cannot generate without fabricating")

// DeepSeekCLI provides DeepSeek CLI integration
type DeepSeekCLI struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	APIKey    string
	Model     string
	MaxTokens int
}

// New creates a new DeepSeek CLI integration
func New() *DeepSeekCLI {
	info := agents.AgentInfo{
		Type:        agents.TypeDeepseekCLI,
		Name:        "DeepSeek CLI",
		Description: "CLI for DeepSeek models",
		Vendor:      "DeepSeek",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_generation",
			"chat",
			"completion",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &DeepSeekCLI{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model:     "deepseek-chat",
			MaxTokens: 4096,
		},
	}
}

// Initialize initializes DeepSeek CLI
func (d *DeepSeekCLI) Initialize(ctx context.Context, config interface{}) error {
	if err := d.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		d.config = cfg
	}

	return nil
}

// Execute executes a command
func (d *DeepSeekCLI) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
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
	case "status":
		return d.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// chat validates input then returns an honest no-headless-CLI error rather than
// a fabricated response (BLUFF-001).
func (d *DeepSeekCLI) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}
	return nil, errNoHeadlessCLI
}

// generate validates input then returns an honest no-headless-CLI error.
func (d *DeepSeekCLI) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}
	return nil, errNoHeadlessCLI
}

// status returns status
func (d *DeepSeekCLI) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":  d.IsAvailable(),
		"model":      d.config.Model,
		"max_tokens": d.config.MaxTokens,
	}, nil
}

// IsAvailable checks availability
func (d *DeepSeekCLI) IsAvailable() bool {
	return d.config.APIKey != ""
}

var _ agents.AgentIntegration = (*DeepSeekCLI)(nil)
