// Package gptme provides GPTMe agent integration.
// GPTMe: personal AI assistant for developers, driven through the `gptme`
// CLI in non-interactive mode (`gptme --non-interactive "<prompt>"`).
package gptme

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// gptmeBinary is the GPTMe CLI executable looked up on PATH.
// Overridable in tests via the GPTME_BIN environment variable so a fake
// binary can be injected to prove real exec is wired (anti-bluff).
const gptmeBinary = "gptme"

// getGptmeBinOverride returns the test-only gptme binary override, if set.
func getGptmeBinOverride() string { return os.Getenv("GPTME_BIN") }

// GPTMe provides GPTMe integration
type GPTMe struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Personality string
}

// New creates a new GPTMe integration
func New() *GPTMe {
	info := agents.AgentInfo{
		Type:        agents.TypeGptme,
		Name:        "GPTMe",
		Description: "Personal AI assistant",
		Vendor:      "GPTMe",
		Version:     "1.0.0",
		Capabilities: []string{
			"personal_assistant",
			"context_aware",
			"shell_integration",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &GPTMe{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Personality: "helpful",
		},
	}
}

// Initialize initializes GPTMe
func (g *GPTMe) Initialize(ctx context.Context, config interface{}) error {
	if err := g.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		g.config = cfg
	}

	return nil
}

// Execute executes a command
func (g *GPTMe) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !g.IsStarted() {
		if err := g.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "ask":
		return g.ask(ctx, params)
	case "shell":
		return g.shell(ctx, params)
	case "status":
		return g.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveGptmeBinary locates the gptme CLI executable. Tests may inject a fake
// binary via the GPTME_BIN environment variable (absolute path); otherwise the
// real `gptme` command is resolved on PATH. Returns an honest error when the
// binary is not available — NEVER a fabricated success (BLUFF-001).
func (g *GPTMe) resolveGptmeBinary() (string, error) {
	if bin := getGptmeBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("gptme binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(gptmeBinary)
	if err != nil {
		return "", fmt.Errorf("gptme CLI not found on PATH: %w", err)
	}
	return path, nil
}

// ask asks a question by exec-ing the real gptme CLI non-interactively.
// Per the gptme CLI, the non-interactive form is `gptme --non-interactive
// "<prompt>"`; its stdout is the model's real answer — never a template.
func (g *GPTMe) ask(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	question, _ := params["question"].(string)
	if question == "" {
		return nil, fmt.Errorf("question required")
	}

	bin, err := g.resolveGptmeBinary()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, bin, "--non-interactive", question)
	cmd.Dir = g.GetWorkDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gptme ask failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return map[string]interface{}{
		"question": question,
		"answer":   strings.TrimSpace(string(out)),
	}, nil
}

// shell runs a real shell command via os/exec and surfaces the real exit code
// and combined output — never a fabricated "Executed: <cmd>" template (BLUFF-003).
func (g *GPTMe) shell(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("command required")
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = g.GetWorkDir()
	out, runErr := cmd.CombinedOutput()

	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	result := map[string]interface{}{
		"command":   command,
		"output":    string(out),
		"exit_code": exitCode,
	}
	// A non-zero exit is real shell behaviour surfaced to the caller, not an
	// integration error; only a failure to launch the process is an error.
	if runErr != nil && exitCode == 0 {
		return nil, fmt.Errorf("gptme shell failed to launch %q: %w", command, runErr)
	}
	return result, nil
}

// status returns status
func (g *GPTMe) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available":   g.IsAvailable(),
		"personality": g.config.Personality,
	}, nil
}

// IsAvailable checks availability — the gptme CLI must be installed on PATH.
func (g *GPTMe) IsAvailable() bool {
	_, err := exec.LookPath(gptmeBinary)
	return err == nil
}

var _ agents.AgentIntegration = (*GPTMe)(nil)
