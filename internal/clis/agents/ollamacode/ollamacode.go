// Package ollamacode provides Ollama Code agent integration.
// Ollama Code: local LLM code assistant driven via the real `ollama` CLI
// (`ollama run <model> <prompt>`).
package ollamacode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// ollamaBinary is the Ollama CLI executable looked up on PATH.
// Overridable in tests via OLLAMA_BIN so a fake binary can be injected to prove
// real exec is wired (anti-bluff, BLUFF-001/003).
const ollamaBinary = "ollama"

func getOllamaBinOverride() string { return os.Getenv("OLLAMA_BIN") }

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

// resolveOllamaBinary locates the ollama CLI executable. Tests may inject a fake
// binary via OLLAMA_BIN (absolute path). Returns an honest error when the binary
// is not available — NEVER a fabricated success (BLUFF-001/003).
func (o *OllamaCode) resolveOllamaBinary() (string, error) {
	if bin := getOllamaBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("ollama binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(ollamaBinary)
	if err != nil {
		return "", fmt.Errorf("ollama CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runOllama invokes `ollama run <model> <prompt>` non-interactively and returns
// the real stdout. The OLLAMA_HOST env var is set from the configured endpoint
// so the local daemon is targeted.
func (o *OllamaCode) runOllama(ctx context.Context, prompt string) (string, error) {
	bin, err := o.resolveOllamaBinary()
	if err != nil {
		return "", err
	}

	model := "codellama"
	if o.config != nil && o.config.Model != "" {
		model = o.config.Model
	}

	cmd := exec.CommandContext(ctx, bin, "run", model, prompt)
	cmd.Dir = o.GetWorkDir()
	if o.config != nil && o.config.Endpoint != "" {
		cmd.Env = append(os.Environ(), "OLLAMA_HOST="+o.config.Endpoint)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ollama execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return strings.TrimSpace(string(out)), nil
}

// generate generates code by exec-ing the real ollama CLI.
func (o *OllamaCode) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	code, err := o.runOllama(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"prompt": prompt,
		"code":   code,
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

// IsAvailable checks availability — true only when the real ollama CLI
// (or an OLLAMA_BIN override) is resolvable on PATH.
func (o *OllamaCode) IsAvailable() bool {
	_, err := o.resolveOllamaBinary()
	return err == nil
}

var _ agents.AgentIntegration = (*OllamaCode)(nil)
