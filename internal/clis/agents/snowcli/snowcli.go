// Package snowcli provides Snow CLI agent integration.
// Snow CLI (Snowflake `snow`): exposes `snow sql -q "<query>"` to run SQL
// non-interactively against a configured connection, and `snow cortex complete`
// for LLM completions. See https://docs.snowflake.com/en/developer-guide/snowflake-cli .
package snowcli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dev.helix.agent/internal/clis/agents"
	"dev.helix.agent/internal/clis/agents/base"
)

// snowBinary is the Snowflake CLI executable looked up on PATH. The published
// binary is `snow` (the package's agents.TypeSnowCLI value is "snow_cli", which
// is NOT the executable name). Overridable in tests via SNOW_BIN so a fake
// binary can be injected to prove real exec is wired (anti-bluff, BLUFF-001).
const snowBinary = "snow"

// getSnowBinOverride returns the test-only snow binary override, if set.
func getSnowBinOverride() string { return os.Getenv("SNOW_BIN") }

// SnowCLI provides Snow CLI integration
type SnowCLI struct {
	*base.BaseIntegration
	config *Config
}

// Config holds configuration
type Config struct {
	base.BaseConfig
	Account    string
	Warehouse  string
	Connection string
}

// New creates a new Snow CLI integration
func New() *SnowCLI {
	info := agents.AgentInfo{
		Type:        agents.TypeSnowCLI,
		Name:        "Snow CLI",
		Description: "Snowflake AI integration",
		Vendor:      "Snowflake",
		Version:     "1.0.0",
		Capabilities: []string{
			"data_warehouse",
			"sql",
			"analytics",
		},
		IsEnabled: true,
		Priority:  2,
	}

	return &SnowCLI{
		BaseIntegration: base.NewBaseIntegration(info),
		config: &Config{
			BaseConfig: base.BaseConfig{
				AutoStart: true,
			},
			Warehouse: "COMPUTE_WH",
		},
	}
}

// Initialize initializes Snow CLI
func (s *SnowCLI) Initialize(ctx context.Context, config interface{}) error {
	if err := s.BaseIntegration.Initialize(ctx, config); err != nil {
		return err
	}

	if cfg, ok := config.(*Config); ok {
		s.config = cfg
	}

	return nil
}

// Execute executes a command
func (s *SnowCLI) Execute(ctx context.Context, command string, params map[string]interface{}) (interface{}, error) {
	if !s.IsStarted() {
		if err := s.Start(ctx); err != nil {
			return nil, err
		}
	}

	switch command {
	case "query":
		return s.query(ctx, params)
	case "status":
		return s.status(ctx)
	default:
		return nil, fmt.Errorf("unknown command: %s", command)
	}
}

// resolveSnowBinary locates the snow CLI executable. Tests may inject a fake
// binary via SNOW_BIN (absolute path); otherwise the real `snow` command is
// resolved on PATH. Returns an honest error when the binary is not available —
// NEVER a fabricated success (BLUFF-001).
func (s *SnowCLI) resolveSnowBinary() (string, error) {
	if bin := getSnowBinOverride(); bin != "" {
		if _, err := exec.LookPath(bin); err != nil {
			return "", fmt.Errorf("snow binary override %q not executable: %w", bin, err)
		}
		return bin, nil
	}
	path, err := exec.LookPath(snowBinary)
	if err != nil {
		return "", fmt.Errorf("snow CLI not found on PATH: %w", err)
	}
	return path, nil
}

// query executes SQL by exec-ing the real `snow sql -q "<sql>"` command against
// the configured connection/warehouse and returns the CLI's real output.
// Returns an honest error when the binary is absent or the query fails — NEVER
// the fabricated "Query result" placeholder (BLUFF-001).
func (s *SnowCLI) query(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	sql, _ := params["sql"].(string)
	if sql == "" {
		return nil, fmt.Errorf("sql required")
	}

	bin, err := s.resolveSnowBinary()
	if err != nil {
		return nil, err
	}

	args := []string{"sql", "-q", sql}
	if s.config.Connection != "" {
		args = append(args, "--connection", s.config.Connection)
	}
	if s.config.Warehouse != "" {
		args = append(args, "--warehouse", s.config.Warehouse)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = s.GetWorkDir()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("snow sql execution failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return map[string]interface{}{
		"sql":       sql,
		"result":    strings.TrimSpace(string(out)),
		"warehouse": s.config.Warehouse,
	}, nil
}

// status returns status
func (s *SnowCLI) status(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"available": s.IsAvailable(),
		"account":   s.config.Account,
	}, nil
}

// IsAvailable reports whether the real snow CLI is resolvable on PATH (or via
// the SNOW_BIN override). Honest availability — not a config-flag proxy.
func (s *SnowCLI) IsAvailable() bool {
	_, err := s.resolveSnowBinary()
	return err == nil
}

var _ agents.AgentIntegration = (*SnowCLI)(nil)
