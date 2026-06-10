// Package copilotcli provides GitHub Copilot CLI agent integration.
// Copilot CLI: GitHub's official AI coding assistant for the command line.
package copilotcli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// copilotBinary is the GitHub Copilot CLI executable looked up on PATH.
// Overridable in tests via the COPILOT_BIN environment variable so a fake
// binary can be injected to prove real exec is wired (anti-bluff §11.4.115).
const copilotBinary = "copilot"

// getCopilotBinOverride returns the test-only copilot binary override, if set.
func getCopilotBinOverride() string { return os.Getenv("COPILOT_BIN") }

// CopilotCLI provides GitHub Copilot CLI integration
type CopilotCLI struct {
	*base.BaseIntegration
	config *Config
}

// Config holds Copilot CLI configuration
type Config struct {
	base.BaseConfig
	Editor      string
	EnableAuto  bool
	Suggestions bool
	PublicCode  bool
	Model       string
}

// New creates a new Copilot CLI integration
func New() *CopilotCLI {
	info := agents.AgentInfo{
		Type:        agents.TypeCopilotCLI,
		Name:        "Copilot CLI",
		Description: "GitHub's AI coding assistant CLI",
		Vendor:      "GitHub",
		Version:     "1.0.0",
		Capabilities: []string{
			"code_suggestions",
			"autocomplete",
			"code_explanation",
			"test_generation",
			"documentation",
			"shell_completions",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &CopilotCLI{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Editor:      "vim",
			EnableAuto:  true,
			Suggestions: true,
			PublicCode:  false,
		},
	}
}

// Initialize initializes Copilot CLI
func (c *CopilotCLI) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return nil
}

// Execute executes a Copilot CLI command
func (c *CopilotCLI) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "suggest":
		return c.suggest(ctx, params)
	case "explain":
		return c.explain(ctx, params)
	case "test":
		return c.test(ctx, params)
	case "fix":
		return c.fix(ctx, params)
	case "docs":
		return c.docs(ctx, params)
	case "status":
		return c.status(ctx)
	case "login":
		return c.login(ctx)
	case "logout":
		return c.logout(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveCopilotBinary locates the GitHub Copilot CLI executable. Tests may
// inject a fake binary via the COPILOT_BIN environment variable (absolute
// path); otherwise the real `copilot` command is resolved on PATH. Returns an
// honest error when the binary is not available — NEVER a fabricated success or
// fabricated fallback response (BLUFF-001).
func (c *CopilotCLI) resolveCopilotBinary() (string, error) {
	if bin := getCopilotBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("copilot binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(copilotBinary)
	if err != nil {
		return "", fmt.Errorf("copilot CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runCopilot invokes the GitHub Copilot CLI non-interactively and returns its
// textual output. Per the 2026-06-10 currency research the non-interactive form
// is `copilot -p "<prompt>" -s --allow-all-tools --no-ask-user`; `-s`/`--silent`
// strips stats/decoration for clean stdout (Copilot CLI has no JSON output flag
// — honest negative finding), `--allow-all-tools` + `--no-ask-user` make the run
// fully unattended. The model is passed via `--model` when configured.
func (c *CopilotCLI) runCopilot(ctx context.Context, prompt string) (string, error) {
	bin, err := c.resolveCopilotBinary()
	if err != nil {
		return "", err
	}

	args := []string{"-p", prompt, "-s", "--allow-all-tools", "--no-ask-user"}
	if c.config != nil && c.config.Model != "" {
		args = append(args, "--model", c.config.Model)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = c.GetWorkDir()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("copilot execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return strings.TrimSpace(string(out)), nil
}

// suggest gets code suggestions by exec-ing the real copilot CLI.
func (c *CopilotCLI) suggest(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	language, _ := params["language"].(string)
	if language == "" {
		language = "go"
	}

	suggestion, err := c.runCopilot(ctx, fmt.Sprintf("Suggest %s code for: %s", language, prompt))
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"prompt":     prompt,
		"language":   language,
		"suggestion": suggestion,
		"source":     "github_copilot",
	}, nil
}

// explain explains code by exec-ing the real copilot CLI.
func (c *CopilotCLI) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	explanation, err := c.runCopilot(ctx, "Explain the following code:\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code":        code,
		"explanation": explanation,
		"source":      "github_copilot",
	}, nil
}

// test generates tests by exec-ing the real copilot CLI.
func (c *CopilotCLI) test(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	file, _ := params["file"].(string)

	if code == "" && file == "" {
		return nil, fmt.Errorf("code or file required")
	}

	var target string
	if file != "" {
		target = file
	} else {
		target = code
	}

	tests, err := c.runCopilot(ctx, "Generate tests for the following:\n"+target)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"target": target,
		"tests":  tests,
		"source": "github_copilot",
	}, nil
}

// fix fixes code issues by exec-ing the real copilot CLI.
func (c *CopilotCLI) fix(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	fixed, err := c.runCopilot(ctx, "Fix issues in the following code and return the corrected code:\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code":   code,
		"fixed":  fixed,
		"source": "github_copilot",
	}, nil
}

// docs generates documentation by exec-ing the real copilot CLI.
func (c *CopilotCLI) docs(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	documentation, err := c.runCopilot(ctx, "Generate documentation for the following code:\n"+code)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"code":          code,
		"documentation": documentation,
		"source":        "github_copilot",
	}, nil
}

// status checks Copilot status via the real `gh auth status` command. Returns
// an honest error when the gh CLI is not available — never a fabricated status.
func (c *CopilotCLI) status(ctx context.Context) (interface{}, error) {
	output, err := c.ExecuteCommand(ctx, "gh", "auth", "status")
	if err != nil {
		return nil, fmt.Errorf("gh auth status failed (gh CLI required): %w", err)
	}

	isAuth := strings.Contains(string(output), "Logged in")

	return map[string]interface{}{
		"authenticated": isAuth,
		"status":        string(output),
		"enabled":       c.config.EnableAuto,
	}, nil
}

// login authenticates with GitHub via the real `gh auth login` command.
func (c *CopilotCLI) login(ctx context.Context) (interface{}, error) {
	output, err := c.ExecuteCommand(ctx, "gh", "auth", "login")
	if err != nil {
		return nil, fmt.Errorf("gh auth login failed (gh CLI required): %w", err)
	}

	return map[string]interface{}{
		"success": true,
		"message": string(output),
	}, nil
}

// logout logs out from GitHub via the real `gh auth logout` command.
func (c *CopilotCLI) logout(ctx context.Context) (interface{}, error) {
	output, err := c.ExecuteCommand(ctx, "gh", "auth", "logout")
	if err != nil {
		return nil, fmt.Errorf("gh auth logout failed (gh CLI required): %w", err)
	}

	return map[string]interface{}{
		"success": true,
		"message": string(output),
	}, nil
}

// IsAvailable checks if the GitHub Copilot CLI is available on PATH.
func (c *CopilotCLI) IsAvailable() bool {
	_, err := c.resolveCopilotBinary()
	return err == nil
}

// GetSuggestionsEnabled returns if suggestions are enabled
func (c *CopilotCLI) GetSuggestionsEnabled() bool {
	return c.config.Suggestions
}

// SetSuggestionsEnabled enables/disables suggestions
func (c *CopilotCLI) SetSuggestionsEnabled(enabled bool) {
	c.config.Suggestions = enabled
}

var _ agents.AgentIntegration = (*CopilotCLI)(nil)
