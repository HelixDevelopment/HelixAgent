// Package supermaven provides Supermaven agent integration.
// Supermaven: Ultra-fast AI code completion with large context window.
package supermaven

import (
	"context"
	"fmt"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

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

// complete generates code completion
func (s *Supermaven) complete(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prefix, _ := params["prefix"].(string)
	suffix, _ := params["suffix"].(string)
	language, _ := params["language"].(string)

	if language == "" {
		language = "go"
	}

	// Generate completion based on context
	completion := s.generateCompletion(prefix, suffix, language)

	return map[string]interface{}{
		"prefix":     prefix,
		"suffix":     suffix,
		"language":   language,
		"completion": completion,
		"mode":       s.config.CompletionMode,
	}, nil
}

// generateCompletion generates completion
func (s *Supermaven) generateCompletion(prefix, suffix, language string) string {
	// Simplified completion generation
	lines := strings.Split(prefix, "\n")
	lastLine := ""
	if len(lines) > 0 {
		lastLine = lines[len(lines)-1]
	}

	switch language {
	case "go":
		if strings.HasSuffix(lastLine, "func ") {
			return "functionName() {\n\t// Implementation\n}"
		}
		if strings.Contains(lastLine, "if ") {
			return " {\n\t// Condition body\n}"
		}
		return "// Supermaven completion"
	case "python":
		if strings.HasSuffix(lastLine, "def ") {
			return "function_name():\n    pass"
		}
		return "# Supermaven completion"
	case "typescript", "javascript":
		if strings.HasSuffix(lastLine, "function ") {
			return "functionName() {\n  // Implementation\n}"
		}
		return "// Supermaven completion"
	default:
		return "// Supermaven completion"
	}
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
