// Package ollamacode provides Ollama Code agent integration.
// Ollama Code: Local LLM code assistant.
package ollamacode

import (
	"context"
	"fmt"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// OllamaCode provides Ollama Code integration
type OllamaCode struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Endpoint string
	Model    string
}

// New creates a new Ollama Code integration
func New() *OllamaCode {
	info := agents.AgentInfo{
		Type:        agents.TypeOllamaCode,
		Name:        "Ollama Code",
		Description: "Local LLM code assistant",
		Vendor:      "Ollama",
		Version:     "1.0.0",
		Capabilities: []string{
			"local_llm",
			"privacy",
			"code_generation",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &OllamaCode{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Endpoint: "http://localhost:11434",
			Model:    "codellama",
		},
	}
}

// Initialize initializes Ollama Code
func (o *OllamaCode) Initialize(ctx context.Context, config interface{}) error {
	if err := o.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		o.config = cfg
	}

	return nil
}

// Execute executes a command
func (o *OllamaCode) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !o.IsStarted() {
		if err := o.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "generate":
		return o.generate(ctx, params)
	case "status":
		return o.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// generate generates code
func (o *OllamaCode) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	return map[string]interface{}{
		"prompt": prompt,
		"code":   fmt.Sprintf("// Ollama local\n// %s", prompt),
		"model":  o.config.Model,
	}, nil
}

// status returns status
func (o *OllamaCode) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": o.IsAvailable(),
		"endpoint":  o.config.Endpoint,
		"model":     o.config.Model,
	}, nil
}

// IsAvailable checks availability
func (o *OllamaCode) IsAvailable() bool {
	return o.config.Endpoint != ""
}

var _ agents.AgentIntegration = (*OllamaCode)(nil)
