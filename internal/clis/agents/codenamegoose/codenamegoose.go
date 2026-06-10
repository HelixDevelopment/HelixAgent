// Package codenamegoose provides Codename Goose agent integration.
// Codename Goose: Multi-provider AI agent framework.
package codenamegoose

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// gooseBinary is the Block Goose CLI executable looked up on PATH.
// Overridable in tests via the GOOSE_BIN environment variable so a fake
// binary can be injected to prove real exec is wired (anti-bluff).
const gooseBinary = "goose"

// getGooseBinOverride returns the test-only goose binary override, if set.
func getGooseBinOverride() string { return os.Getenv("GOOSE_BIN") }

// CodenameGoose provides Codename Goose integration
type CodenameGoose struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Provider string
	Model    string
}

// New creates a new Codename Goose integration
func New() *CodenameGoose {
	info := agents.AgentInfo{
		Type:        agents.TypeCodenameGoose,
		Name:        "Codename Goose",
		Description: "Multi-provider AI agent",
		Vendor:      "Goose",
		Version:     "1.0.0",
		Capabilities: []string{
			"multi_provider",
			"extensible",
			"tool_use",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &CodenameGoose{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Provider: "anthropic",
			Model:    "claude-3-sonnet",
		},
	}
}

// Initialize initializes Codename Goose
func (c *CodenameGoose) Initialize(ctx context.Context, config interface{}) error {
	if err := c.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		c.config = cfg
	}

	return nil
}

// Execute executes a command
func (c *CodenameGoose) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !c.IsStarted() {
		if err := c.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "run":
		return c.run(ctx, params)
	case "configure":
		return c.configure(ctx, params)
	case "status":
		return c.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveGooseBinary locates the goose CLI executable. Tests may inject a fake
// binary via the GOOSE_BIN environment variable (absolute path); otherwise the
// real `goose` command is resolved on PATH. Returns an honest error when the
// binary is not available — NEVER a fabricated success (BLUFF-001/003).
func (c *CodenameGoose) resolveGooseBinary() (string, error) {
	if bin := getGooseBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("goose binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(gooseBinary)
	if err != nil {
		return "", fmt.Errorf("goose CLI not found on PATH: %w", err)
	}
	return path, nil
}

// run runs the agent by exec-ing the real goose CLI non-interactively.
// The headless form is `goose run --text "<prompt>"` (`-t`). The provider/model
// are forwarded via the environment when configured; goose reads them from its
// own config + the GOOSE_PROVIDER / GOOSE_MODEL environment variables.
func (c *CodenameGoose) run(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	prompt, _ := params["prompt"].(string)
	if prompt == "" {
		return nil, fmt.Errorf("prompt required")
	}

	bin, err := c.resolveGooseBinary()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, bin, "run", "--text", prompt)
	cmd.Dir = c.GetWorkDir()
	cmd.Env = os.Environ()
	if c.config != nil {
		if c.config.Provider != "" {
			cmd.Env = append(cmd.Env, "GOOSE_PROVIDER="+c.config.Provider)
		}
		if c.config.Model != "" {
			cmd.Env = append(cmd.Env, "GOOSE_MODEL="+c.config.Model)
		}
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("goose execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return map[string]interface{}{
		"prompt":   prompt,
		"result":   strings.TrimSpace(string(out)),
		"provider": c.config.Provider,
	}, nil
}

// configure configures the agent
func (c *CodenameGoose) configure(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if provider, ok := params["provider"].(string); ok {
		c.config.Provider = provider
	}
	if model, ok := params["model"].(string); ok {
		c.config.Model = model
	}

	return map[string]interface{}{
		"provider": c.config.Provider,
		"model":    c.config.Model,
	}, nil
}

// status returns status
func (c *CodenameGoose) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": c.IsAvailable(),
		"provider":  c.config.Provider,
		"model":     c.config.Model,
	}, nil
}

// IsAvailable checks availability
func (c *CodenameGoose) IsAvailable() bool {
	return true
}

var _ agents.AgentIntegration = (*CodenameGoose)(nil)
