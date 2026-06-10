// Package qwencode provides Qwen Code agent integration.
// Qwen Code: Alibaba's AI coding assistant.
package qwencode

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

// qwenBinary is the Qwen Code CLI executable looked up on PATH.
// Overridable in tests via the QWEN_BIN environment variable so a fake
// binary can be injected to prove real exec is wired (anti-bluff).
const qwenBinary = "qwen"

// getQwenBinOverride returns the test-only qwen binary override, if set.
func getQwenBinOverride() string { return os.Getenv("QWEN_BIN") }

// QwenCode provides Qwen Code integration
type QwenCode struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	APIKey string
	Model  string
}

// New creates a new Qwen Code integration
func New() *QwenCode {
	info := agents.AgentInfo{
		Type:        agents.TypeQwenCode,
		Name:        "Qwen Code",
		Description: "Alibaba AI coding assistant",
		Vendor:      "Alibaba",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_generation",
			"code_completion",
			"chat",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &QwenCode{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model: "qwen-coder-plus",
		},
	}
}

// Initialize initializes Qwen Code
func (q *QwenCode) Initialize(ctx context.Context, config interface{}) error {
	if err := q.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		q.config = cfg
	}

	return nil
}

// Execute executes a command
func (q *QwenCode) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !q.IsStarted() {
		if err := q.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "complete":
		return q.complete(ctx, params)
	case "generate":
		return q.generate(ctx, params)
	case "chat":
		return q.chat(ctx, params)
	case "status":
		return q.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveQwenBinary locates the qwen CLI executable. Tests may inject a fake
// binary via the QWEN_BIN environment variable (absolute path); otherwise the
// real `qwen` command is resolved on PATH. Returns an honest error when the
// binary is not available — NEVER a fabricated success (BLUFF-001/003).
func (q *QwenCode) resolveQwenBinary() (string, error) {
	if bin := getQwenBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("qwen binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(qwenBinary)
	if err != nil {
		return "", fmt.Errorf("qwen CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runQwen invokes the qwen CLI non-interactively and returns its textual
// output. Per §11.4.99 the non-interactive form is `qwen -p "<prompt>"
// --output-format json`; the JSON envelope's text/response field is extracted
// when present, otherwise the raw stdout is returned. The model is passed via
// `--model` when configured.
func (q *QwenCode) runQwen(ctx context.Context, prompt string) (string, error) {
	bin, err := q.resolveQwenBinary()
	if err != nil {
		return "", err
	}

	args := []string{"-p", prompt, "--output-format", "json"}
	if q.config != nil && q.config.Model != "" {
		args = append(args, "--model", q.config.Model)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = q.GetWorkDir()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("qwen execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return extractQwenText(out), nil
}

// extractQwenText pulls the human-visible text out of the qwen JSON envelope.
// The qwen CLI `--output-format json` emits an object; the text lives under one
// of the common keys. When the payload is not JSON or has no known key, the
// trimmed raw stdout is returned (still real process output, never a template).
func extractQwenText(out []byte) string {
	trimmed := strings.TrimSpace(string(out))
	var env map[string]interface{}
	if err := json.Unmarshal(out, &env); err == nil {
		for _, k := range []string{"response", "text", "output", "content", "result", "message"} {
			if v, ok := env[k]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return trimmed
}

// complete generates completion by exec-ing the real qwen CLI.
func (q *QwenCode) complete(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prefix, _ := params["prefix"].(string)
	if prefix == "" {
		return nil, fmt.Errorf("prefix required")
	}

	completion, err := q.runQwen(ctx, "Complete the following code:\n"+prefix)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"prefix":     prefix,
		"completion": completion,
		"model":      q.config.Model,
	}, nil
}

// generate generates code by exec-ing the real qwen CLI.
func (q *QwenCode) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	code, err := q.runQwen(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"prompt": prompt,
		"code":   code,
		"model":  q.config.Model,
	}, nil
}

// chat performs chat by exec-ing the real qwen CLI.
func (q *QwenCode) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	response, err := q.runQwen(ctx, message)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message":  message,
		"response": response,
		"model":    q.config.Model,
	}, nil
}

// status returns status
func (q *QwenCode) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": q.IsAvailable(),
		"model":     q.config.Model,
	}, nil
}

// IsAvailable checks availability
func (q *QwenCode) IsAvailable() bool {
	return q.config.APIKey != ""
}

var _ agents.AgentIntegration = (*QwenCode)(nil)
