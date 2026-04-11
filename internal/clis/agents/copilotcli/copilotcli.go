// Package copilotcli provides GitHub Copilot CLI agent integration.
// Copilot CLI: GitHub's official AI coding assistant for the command line.
package copilotcli

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

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

// suggest gets code suggestions
func (c *CopilotCLI) suggest(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	language, _ := params["language"].(string)
	if language == "" {
		language = "go"
	}

	// Build gh copilot command
	args := []string{"copilot", "suggest", "-t", language, prompt}

	output, err := c.ExecuteCommand(ctx, "gh", args...)
	if err != nil {
		// If gh command fails, provide fallback
		return map[string]interface{}{
			"prompt":     prompt,
			"language":   language,
			"suggestion": fmt.Sprintf("// Generated code for: %s\n// Language: %s\n", prompt, language),
			"note":       "Using fallback - gh copilot CLI not available",
		}, nil
	}

	return map[string]interface{}{
		"prompt":     prompt,
		"language":   language,
		"suggestion": string(output),
		"source":     "github_copilot",
	}, nil
}

// explain explains code
func (c *CopilotCLI) explain(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	args := []string{"copilot", "explain", code}

	output, err := c.ExecuteCommand(ctx, "gh", args...)
	if err != nil {
		return map[string]interface{}{
			"code":        code,
			"explanation": "Code explanation would be provided by GitHub Copilot",
			"note":        "Fallback - gh CLI not available",
		}, nil
	}

	return map[string]interface{}{
		"code":        code,
		"explanation": string(output),
		"source":      "github_copilot",
	}, nil
}

// test generates tests
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

	args := []string{"copilot", "suggest", "-t", "test", target}

	output, err := c.ExecuteCommand(ctx, "gh", args...)
	if err != nil {
		return map[string]interface{}{
			"target": target,
			"tests":  "// Generated tests would be provided by GitHub Copilot",
			"note":   "Fallback - gh CLI not available",
		}, nil
	}

	return map[string]interface{}{
		"target": target,
		"tests":  string(output),
		"source": "github_copilot",
	}, nil
}

// fix fixes code issues
func (c *CopilotCLI) fix(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	args := []string{"copilot", "suggest", "-t", "fix", code}

	output, err := c.ExecuteCommand(ctx, "gh", args...)
	if err != nil {
		return map[string]interface{}{
			"code":    code,
			"fixed":   "// Fixed code would be provided by GitHub Copilot",
			"changes": []string{"No changes - gh CLI not available"},
		}, nil
	}

	return map[string]interface{}{
		"code":    code,
		"fixed":   string(output),
		"changes": []string{"Applied Copilot suggestions"},
		"source":  "github_copilot",
	}, nil
}

// docs generates documentation
func (c *CopilotCLI) docs(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	code, _ := params["code"].(string)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}

	args := []string{"copilot", "suggest", "-t", "docs", code}

	output, err := c.ExecuteCommand(ctx, "gh", args...)
	if err != nil {
		return map[string]interface{}{
			"code":          code,
			"documentation": "// Generated documentation would be provided by GitHub Copilot",
			"note":          "Fallback - gh CLI not available",
		}, nil
	}

	return map[string]interface{}{
		"code":          code,
		"documentation": string(output),
		"source":        "github_copilot",
	}, nil
}

// status checks Copilot status
func (c *CopilotCLI) status(ctx context.Context) (interface{}, error) {
	args := []string{"auth", "status"}

	output, err := c.ExecuteCommand(ctx, "gh", args...)
	if err != nil {
		return map[string]interface{}{
			"authenticated": false,
			"status":        "unknown",
			"message":       "Could not check status - gh CLI not available",
		}, nil
	}

	isAuth := strings.Contains(string(output), "Logged in")

	return map[string]interface{}{
		"authenticated": isAuth,
		"status":        string(output),
		"enabled":       c.config.EnableAuto,
	}, nil
}

// login authenticates with GitHub
func (c *CopilotCLI) login(ctx context.Context) (interface{}, error) {
	args := []string{"auth", "login"}

	output, err := c.ExecuteCommand(ctx, "gh", args...)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   "gh CLI not available or login failed",
		}, nil
	}

	return map[string]interface{}{
		"success": true,
		"message": string(output),
	}, nil
}

// logout logs out from GitHub
func (c *CopilotCLI) logout(ctx context.Context) (interface{}, error) {
	args := []string{"auth", "logout"}

	output, err := c.ExecuteCommand(ctx, "gh", args...)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   "gh CLI not available",
		}, nil
	}

	return map[string]interface{}{
		"success": true,
		"message": string(output),
	}, nil
}

// IsAvailable checks if gh CLI is available
func (c *CopilotCLI) IsAvailable() bool {
	_, err := exec.LookPath("gh")
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
