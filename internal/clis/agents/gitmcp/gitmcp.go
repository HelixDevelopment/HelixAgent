// Package gitmcp provides Git MCP agent integration.
// Git MCP: Model Context Protocol for Git operations. Operations are performed
// by exec-ing the real `git` CLI in the configured repository — never by
// fabricating commit hashes or statuses (anti-bluff: BLUFF-001/003).
package gitmcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// gitBinary is the git executable looked up on PATH.
// Overridable in tests via the GITMCP_GIT_BIN environment variable so a fake
// binary can be injected to prove real exec is wired (anti-bluff).
const gitBinary = "git"

// getGitBinOverride returns the test-only git binary override, if set.
func getGitBinOverride() string { return os.Getenv("GITMCP_GIT_BIN") }

// GitMCP provides Git MCP integration
type GitMCP struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Repository string
}

// New creates a new Git MCP integration
func New() *GitMCP {
	info := agents.AgentInfo{
		Type:        agents.TypeGitMCP,
		Name:        "Git MCP",
		Description: "MCP for Git operations",
		Vendor:      "GitMCP",
		Version:     "1.0.0",
		Capabilities: []string{
			"git_operations",
			"mcp_protocol",
			"version_control",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &GitMCP{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
		},
	}
}

// Initialize initializes Git MCP
func (g *GitMCP) Initialize(ctx context.Context, config interface{}) error {
	if err := g.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		g.config = cfg
	}

	return nil
}

// Execute executes a command
func (g *GitMCP) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !g.IsStarted() {
		if err := g.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "commit":
		return g.commit(ctx, params)
	case "branch":
		return g.branch(ctx, params)
	case "status":
		return g.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// repoDir returns the directory git commands run in: the configured Repository
// if set, otherwise the integration work dir.
func (g *GitMCP) repoDir() string {
	if g.config != nil && g.config.Repository != "" {
		return g.config.Repository
	}
	return g.GetWorkDir()
}

// resolveGitBinary locates the git executable. Tests may inject a fake binary
// via GITMCP_GIT_BIN (absolute path); otherwise the real `git` is resolved on
// PATH. Returns an honest error when git is not available.
func (g *GitMCP) resolveGitBinary() (string, error) {
	if bin := getGitBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("git binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(gitBinary)
	if err != nil {
		return "", fmt.Errorf("git CLI not found on PATH: %w", err)
	}
	return path, nil
}

// runGit invokes the real git CLI in the repo dir and returns its trimmed stdout.
func (g *GitMCP) runGit(ctx context.Context, args ...string) (string, error) {
	bin, err := g.resolveGitBinary()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = g.repoDir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w (output: %s)",
			strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// commit creates a REAL commit via `git commit` and returns the REAL resulting
// commit SHA from `git rev-parse HEAD` — never a fabricated "abc123".
func (g *GitMCP) commit(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	message, _ := params["message"].(string)
	if message == "" {
		return nil, fmt.Errorf("message required")
	}

	if _, err := g.runGit(ctx, "commit", "-m", message); err != nil {
		return nil, err
	}
	sha, err := g.runGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message": message,
		"commit":  sha,
		"status":  "committed",
	}, nil
}

// branch creates a REAL branch via `git branch <name>`.
func (g *GitMCP) branch(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}

	if _, err := g.runGit(ctx, "branch", name); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"branch": name,
		"status": "created",
	}, nil
}

// status returns the REAL `git status --porcelain` output for the repo.
func (g *GitMCP) status(ctx context.Context) (interface{}, error) {
	porcelain, err := g.runGit(ctx, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"available":  g.IsAvailable(),
		"repository": g.config.Repository,
		"porcelain":  porcelain,
		"clean":      porcelain == "",
	}, nil
}

// IsAvailable checks availability — the git CLI must be installed on PATH.
func (g *GitMCP) IsAvailable() bool {
	_, err := exec.LookPath(gitBinary)
	return err == nil
}

var _ agents.AgentIntegration = (*GitMCP)(nil)
