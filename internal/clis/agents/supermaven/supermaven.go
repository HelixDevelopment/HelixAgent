// Package supermaven provides Supermaven agent integration.
// Supermaven: Ultra-fast AI code completion with large context window.
package supermaven

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// ErrNoHeadlessCLI is returned by completion/generation entrypoints that have no
// real backing capability. Supermaven ships ONLY as an editor plugin / language
// server embedded in IDEs (VS Code, JetBrains, Neovim, etc.); it exposes NO
// headless command-line interface that turns a prefix into a completion. Rather
// than fabricate a templated completion (BLUFF-001), these entrypoints return an
// honest error. See https://supermaven.com .
var ErrNoHeadlessCLI = fmt.Errorf("supermaven has no headless CLI: it ships only as an IDE/editor plugin (language server), so code completion cannot be produced from this integration — refusing to fabricate a completion (BLUFF-001)")

// Supermaven provides Supermaven integration
type Supermaven struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Supermaven configuration
type Config struct {
	base.BaseConfig
	APIKey         string
	ContextWindow  int
	CompletionMode string // "full", "single_line", "multi_line"
}

// New creates a new Supermaven integration
func New() *Supermaven {
	info := agents.AgentInfo{
		Type:        agents.TypeSupermaven,
		Name:        "Supermaven",
		Description: "Ultra-fast AI code completion",
		Vendor:      "Supermaven",
		Version:     "1.0.0",
		Capabilities: []string{
			"fast_completion",
			"large_context",
			"multi_line",
			"smart_indent",
			"language_aware",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Supermaven{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			ContextWindow:  1000000,
			CompletionMode: "multi_line",
		},
	}
}

// Initialize initializes Supermaven
func (s *Supermaven) Initialize(ctx context.Context, config interface{}) error {
	if err := s.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		s.config = cfg
	}

	return nil
}

// Execute executes a command
func (s *Supermaven) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !s.IsStarted() {
		if err := s.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "complete":
		return s.complete(ctx, params)
	case "accept":
		return s.accept(ctx, params)
	case "reject":
		return s.reject(ctx, params)
	case "status":
		return s.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// complete returns an HONEST error: Supermaven has no headless CLI that can
// produce a real completion outside an editor host, so this integration refuses
// to fabricate one (BLUFF-001). The prefix is required so the contract still
// validates input before reporting the unsupported capability.
func (s *Supermaven) complete(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prefix, _ := params["prefix"].(string)
	if prefix == "" {
		return nil, fmt.Errorf("prefix required")
	}
	return nil, ErrNoHeadlessCLI
}

// accept accepts a completion
func (s *Supermaven) accept(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	completion, _ := params["completion"].(string)

	return map[string]interface{}{
		"accepted":   true,
		"completion": completion,
		"stats": map[string]interface{}{
			"acceptances": 1,
		},
	}, nil
}

// reject rejects a completion
func (s *Supermaven) reject(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"rejected": true,
		"stats": map[string]interface{}{
			"rejections": 1,
		},
	}, nil
}

// status returns status
func (s *Supermaven) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":      s.IsAvailable(),
		"context_window": s.config.ContextWindow,
		"mode":           s.config.CompletionMode,
	}, nil
}

// IsAvailable checks availability
func (s *Supermaven) IsAvailable() bool {
	return s.config.APIKey != ""
}

var _ agents.AgentIntegration = (*Supermaven)(nil)
