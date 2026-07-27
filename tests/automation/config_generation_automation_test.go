// Package automation provides config generation automation tests.
// These tests verify that CLI agent config generation produces valid
// JSON for supported agent types.
package automation

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildHelixAgent compiles the helixagent binary to a temp directory
// and returns its path.
func buildHelixAgent(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "helixagent")

	cmd := repoCommand(t, "go", "build", "-o", outputPath, "./cmd/helixagent")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "build failed: %s", string(out))
	return outputPath
}

// generateAgentConfig runs `--generate-agent-config=<agent>` and returns stdout
// and stderr SEPARATELY.
//
// The binary writes the generated configuration to stdout and its logs to
// stderr, which is what makes `helixagent --generate-agent-config=opencode >
// opencode.json` work for end users. These tests previously read
// CombinedOutput() and then asserted the merged text was valid JSON — an
// assertion a correct binary can never satisfy, because the interleaved log
// lines are not JSON. Reading the streams apart tests the contract users
// actually rely on.
func generateAgentConfig(t *testing.T, binary, agent string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := repoCommand(t, binary, "--generate-agent-config="+agent)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return strings.TrimSpace(outBuf.String()), strings.TrimSpace(errBuf.String()), err
}

// TestConfigGeneration_ProducesValidJSON verifies that the config
// generator writes a syntactically valid JSON document to stdout for known
// agents.
func TestConfigGeneration_ProducesValidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping config generation test in short mode") // SKIP-OK: #short-mode
	}

	binary := buildHelixAgent(t)

	agents := []string{"opencode", "crush", "kilocode"}
	for _, agent := range agents {
		t.Run(agent, func(t *testing.T) {
			stdout, stderr, err := generateAgentConfig(t, binary, agent)

			assert.NotContains(t, stdout+stderr, "panic",
				"config generation for %s must not panic", agent)
			require.NoError(t, err,
				"config generation for %s should succeed; stderr:\n%s", agent, stderr)

			require.True(t, json.Valid([]byte(stdout)),
				"config for %s must be valid JSON on stdout; got: %s", agent, truncate(stdout, 300))

			var cfg map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &cfg),
				"config for %s must be a JSON object", agent)
			assert.NotEmpty(t, cfg, "config for %s must not be an empty object", agent)
		})
	}
}

// TestConfigGeneration_ContainsRequiredFields verifies that generated
// configs include MCP servers and provider configuration.
func TestConfigGeneration_ContainsRequiredFields(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode") // SKIP-OK: #short-mode
	}

	binary := buildHelixAgent(t)

	// Each supported agent uses its own schema; the required *concepts* are the
	// same, so the keys are listed as per-schema alternatives.
	agents := []string{"opencode", "crush", "kilocode"}
	mcpKeys := []string{"mcp", "mcpServers"}
	providerKeys := []string{"provider", "providers"}

	for _, agent := range agents {
		t.Run(agent, func(t *testing.T) {
			stdout, stderr, err := generateAgentConfig(t, binary, agent)
			require.NoError(t, err,
				"config generation for %s should succeed; stderr:\n%s", agent, stderr)

			var cfg map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &cfg),
				"config for %s should parse as a JSON object; got: %s", agent, truncate(stdout, 300))
			require.NotEmpty(t, cfg, "config for %s should not be an empty object", agent)

			assert.True(t, hasAnyKey(cfg, mcpKeys),
				"config for %s must declare MCP servers (one of %v); keys present: %v",
				agent, mcpKeys, sortedKeys(cfg))
			assert.True(t, hasAnyKey(cfg, providerKeys),
				"config for %s must declare provider configuration (one of %v); keys present: %v",
				agent, providerKeys, sortedKeys(cfg))
		})
	}
}

// TestConfigGeneration_UnknownAgent verifies that requesting a config
// for an unknown agent name does not produce a usable config.
func TestConfigGeneration_UnknownAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode") // SKIP-OK: #short-mode
	}

	binary := buildHelixAgent(t)

	stdout, stderr, err := generateAgentConfig(t, binary, "totally-fake-agent-xyz")

	assert.NotContains(t, stdout+stderr, "panic", "should not panic on unknown agent")
	assert.Error(t, err, "an unknown agent must be reported as a failure, not silently accepted")
	assert.False(t, stdout != "" && json.Valid([]byte(stdout)),
		"an unknown agent must not yield a usable config document; got: %s", truncate(stdout, 300))
}

func hasAnyKey(m map[string]any, keys []string) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
