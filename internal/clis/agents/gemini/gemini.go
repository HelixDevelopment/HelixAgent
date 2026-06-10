// Package gemini provides Google Gemini CLI agent integration.
// Gemini: Google's multimodal AI with code generation capabilities.
package gemini

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

// geminiBinary is the Gemini CLI executable looked up on PATH.
// Overridable in tests via the GEMINI_BIN environment variable so a fake
// binary can be injected to prove real exec is wired (anti-bluff §11.4.115).
const geminiBinary = "gemini"

// getGeminiBinOverride returns the test-only gemini binary override, if set.
func getGeminiBinOverride() string { return os.Getenv("GEMINI_BIN") }

// Gemini provides Google Gemini integration
type Gemini struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Gemini configuration
type Config struct {
	base.BaseConfig
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
	Multimodal  bool
}

// New creates a new Gemini integration
func New() *Gemini {
	info := agents.AgentInfo{
		Type:        agents.TypeGeminiCLI,
		Name:        "Gemini CLI",
		Description: "Google's multimodal AI coding assistant",
		Vendor:      "Google",
		Version:     "1.0.0",
		Capabilities: []string{
			"multimodal",
			"code_generation",
			"code_explanation",
			"chat",
			"image_understanding",
			"long_context",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &Gemini{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Model:       "gemini-2.5-pro",
			MaxTokens:   8192,
			Temperature: 0.7,
			Multimodal:  true,
		},
	}
}

// Initialize initializes Gemini
func (g *Gemini) Initialize(ctx context.Context, config interface{}) error {
	if err := g.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		g.config = cfg
	}

	return nil
}

// Execute executes a command
func (g *Gemini) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !g.IsStarted() {
		if err := g.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "chat":
		return g.chat(ctx, params)
	case "generate":
		return g.generate(ctx, params)
	case "explain":
		return g.explain(ctx, params)
	case "analyze_image":
		return g.analyzeImage(ctx, params)
	case "review":
		return g.review(ctx, params)
	case "status":
		return g.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveGeminiBinary locates the gemini CLI executable. Tests may inject a
// fake binary via the GEMINI_BIN environment variable (absolute path);
// otherwise the real `gemini` command is resolved on PATH. Returns an honest
// error when the binary is not available — NEVER a fabricated success
// (BLUFF-001).
func (g *Gemini) resolveGeminiBinary() (string, error) {
	if bin := getGeminiBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("gemini binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(geminiBinary)
	if err != nil {
		return "", fmt.Errorf("gemini CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runGemini invokes the gemini CLI non-interactively and returns its textual
// output. Per the 2026-06-10 currency research the non-interactive form is
// `gemini -p "<prompt>" --output-format json`; the JSON object's `response`
// field carries the text. The model is passed via `-m` when configured. When
// the payload is not JSON or has no `response` key, the trimmed raw stdout is
// returned (still real process output, never a template).
func (g *Gemini) runGemini(ctx context.Context, prompt string) (string, error) {
	bin, err := g.resolveGeminiBinary()
	if err != nil {
		return "", err
	}

	args := []string{"-p", prompt, "--output-format", "json"}
	if g.config != nil && g.config.Model != "" {
		args = append(args, "-m", g.config.Model)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = g.GetWorkDir()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gemini execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return extractGeminiText(out), nil
}

// extractGeminiText pulls the human-visible text out of the gemini JSON
// envelope. The gemini CLI `--output-format json` emits an object whose
// `response` field is a string; when the payload is not JSON or lacks a known
// key, the trimmed raw stdout is returned.
func extractGeminiText(out []byte) string {
	trimmed := strings.TrimSpace(string(out))
	var env map[string]interface{}
	if err := json.Unmarshal(out, &env); err == nil {
		for _, k := range []string{"response", "text", "output", "content", "result"} {
			if v, ok := env[k]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return trimmed
}

// chat performs chat by exec-ing the real gemini CLI.
func (g *Gemini) chat(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	response, err := g.runGemini(ctx, message)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message":  message,
		"response": response,
		"model":    g.config.Model,
	}, nil
}

// generate generates code by exec-ing the real gemini CLI.
func (g *Gemini) generate(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	language, _ := params["language"].(string)

	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	if language == "" {
		language = "go"
	}

	code, err := g.runGemini(ctx, fmt.Sprintf("Generate %s code for: %s", language, prompt))
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"prompt":   prompt,
		"language": language,
		"code":     code,
		"model":    g.config.Model,
	}, nil
}

// explain explains code by exec-ing the real gemini CLI.
func (g *Gemini) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	explanation, err := g.runGemini(ctx, "Explain the following code:\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code":        code,
		"explanation": explanation,
		"model":       g.config.Model,
	}, nil
}

// analyzeImage analyzes an image by exec-ing the real gemini CLI with the
// image path included in the prompt (Gemini CLI reads referenced files).
func (g *Gemini) analyzeImage(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	imagePath, _ := params["image_path"].(string)
	if imagePath == "" {
		return nil, fmt.Errorf("image_path required")
	}

	if !g.config.Multimodal {
		return nil, fmt.Errorf("multimodal mode not enabled")
	}

	analysis, err := g.runGemini(ctx, fmt.Sprintf("Analyze the image at @%s and describe its contents.", imagePath))
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"image_path": imagePath,
		"analysis":   analysis,
		"model":      g.config.Model,
	}, nil
}

// review reviews code by exec-ing the real gemini CLI.
func (g *Gemini) review(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	review, err := g.runGemini(ctx, "Review the following code and report issues:\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code":   code,
		"review": review,
		"model":  g.config.Model,
	}, nil
}

// status returns status
func (g *Gemini) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":  g.IsAvailable(),
		"model":      g.config.Model,
		"multimodal": g.config.Multimodal,
		"max_tokens": g.config.MaxTokens,
	}, nil
}

// IsAvailable checks availability by resolving the real gemini CLI on PATH.
func (g *Gemini) IsAvailable() bool {
	_, err := g.resolveGeminiBinary()
	return err == nil
}

var _ agents.AgentIntegration = (*Gemini)(nil)
