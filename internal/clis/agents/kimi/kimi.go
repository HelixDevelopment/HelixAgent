// Package kimi provides Kimi agent integration.
// Kimi: Moonshot AI's long-context language model, driven through the
// `kimi` CLI (Moonshot AI ships a headless `kimi` command-line client).
package kimi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// kimiBinary is the Kimi CLI executable looked up on PATH.
// Overridable in tests via the KIMI_BIN environment variable so a fake
// binary can be injected to prove real exec is wired (anti-bluff).
const kimiBinary = "kimi"

// getKimiBinOverride returns the test-only kimi binary override, if set.
func getKimiBinOverride() string { return os.Getenv("KIMI_BIN") }

// Kimi provides Kimi integration
type Kimi struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Kimi configuration
type Config struct {
	base.BaseConfig
	APIKey        string
	Model         string
	ContextWindow int
	MaxTokens     int
}

// New creates a new Kimi integration
func New() *Kimi {
	info := agents.AgentInfo{
		Type:        agents.TypeKimi,
		Name:        "Kimi",
		Description: "Moonshot AI long-context model",
		Vendor:      "Moonshot AI",
		Version:     "1.0.0",
		Capabilities: []string{
			"long_context",
			"code_generation",
			"chat",
			"document_analysis",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Kimi{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model:         "kimi-k2",
			ContextWindow: 2000000,
			MaxTokens:     8192,
		},
	}
}

// Initialize initializes Kimi
func (k *Kimi) Initialize(ctx context.Context, config interface{}) error {
	if err := k.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		k.config = cfg
	}

	return nil
}

// Execute executes a command
func (k *Kimi) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !k.IsStarted() {
		if err := k.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "chat":
		return k.chat(ctx, params)
	case "generate":
		return k.generate(ctx, params)
	case "analyze":
		return k.analyze(ctx, params)
	case "status":
		return k.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveKimiBinary locates the kimi CLI executable. Tests may inject a fake
// binary via the KIMI_BIN environment variable (absolute path); otherwise the
// real `kimi` command is resolved on PATH. Returns an honest error when the
// binary is not available — NEVER a fabricated success (BLUFF-001/003).
func (k *Kimi) resolveKimiBinary() (string, error) {
	if bin := getKimiBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("kimi binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(kimiBinary)
	if err != nil {
		return "", fmt.Errorf("kimi CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runKimi invokes the kimi CLI non-interactively and returns its textual
// output. The non-interactive form is `kimi -p "<prompt>" --output-format json`;
// the JSON envelope's text/response field is extracted when present, otherwise
// the raw stdout is returned. The model is passed via `--model` when configured.
func (k *Kimi) runKimi(ctx context.Context, prompt string) (string, error) {
	bin, err := k.resolveKimiBinary()
	if err != nil {
		return "", err
	}

	args := []string{"-p", prompt, "--output-format", "json"}
	if k.config != nil && k.config.Model != "" {
		args = append(args, "--model", k.config.Model)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = k.GetWorkDir()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kimi execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return extractKimiText(out), nil
}

// extractKimiText pulls the human-visible text out of the kimi JSON envelope.
// When the payload is not JSON or has no known key, the trimmed raw stdout is
// returned (still real process output, never a template).
func extractKimiText(out []byte) string {
	trimmed := strings.TrimSpace(string(out))
	var env map[string]interface{}
	if err := json.Unmarshal(out, &env); err == nil {
		for _, key := range []string{"response", "text", "output", "content", "result", "message"} {
			if v, ok := env[key]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return trimmed
}

// chat performs chat by exec-ing the real kimi CLI.
func (k *Kimi) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	response, err := k.runKimi(ctx, message)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message":  message,
		"response": response,
		"model":    k.config.Model,
	}, nil
}

// generate generates code by exec-ing the real kimi CLI.
func (k *Kimi) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	code, err := k.runKimi(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"prompt": prompt,
		"code":   code,
		"model":  k.config.Model,
	}, nil
}

// analyze analyzes a document by exec-ing the real kimi CLI.
func (k *Kimi) analyze(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	document, _ := params["document"].(string)
	if document == "" {
		return nil, fmt.Errorf("document required")
	}

	analysis, err := k.runKimi(ctx, "Analyze the following document:\n"+document)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"document": document,
		"analysis": analysis,
		"model":    k.config.Model,
	}, nil
}

// status returns status
func (k *Kimi) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":      k.IsAvailable(),
		"model":          k.config.Model,
		"context_window": k.config.ContextWindow,
	}, nil
}

// IsAvailable checks availability
func (k *Kimi) IsAvailable() bool {
	return k.config.APIKey != ""
}

var _ agents.AgentIntegration = (*Kimi)(nil)
