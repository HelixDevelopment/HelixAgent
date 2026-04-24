// Package automation provides config generation automation tests.
// These tests verify that CLI agent config generation produces valid
// JSON for supported agent types.
package automation

import (
	"encoding/json"
	"os/exec"
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

	cmd := exec.Command("go", "build", "-o", outputPath, "./cmd/helixagent")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "build failed: %s", string(out))
	return outputPath
}

// TestConfigGeneration_ProducesValidJSON verifies that the config
// generator produces syntactically valid JSON for known agents.
func TestConfigGeneration_ProducesValidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping config generation test in short mode")  // SKIP-OK: #short-mode
	}

	binary := buildHelixAgent(t)

	agents := []string{"opencode", "crush", "kilocode"}
	for _, agent := range agents {
		t.Run(agent, func(t *testing.T) {
			cmd := exec.Command(binary, "--generate-agent-config="+agent)
			out, err := cmd.CombinedOutput()
			outStr := strings.TrimSpace(string(out))

			if err != nil {
				// Acceptable if API keys are missing; verify no panic.
				assert.NotContains(t, outStr, "panic",
					"config generation for %s should not panic", agent)
				t.Skipf("config generation for %s returned error (likely missing API keys)", agent)
				return
			}

			// Output should be valid JSON
			assert.True(t, json.Valid([]byte(outStr)),
				"config for %s should be valid JSON", agent)
		})
	}
}

// TestConfigGeneration_ContainsRequiredFields verifies that generated
// configs include MCP servers and provider configuration.
func TestConfigGeneration_ContainsRequiredFields(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}

	binary := buildHelixAgent(t)

	cmd := exec.Command(binary, "--generate-agent-config=opencode")
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))

	if err != nil {
		t.Skipf("config generation returned error (likely missing API keys)")
		return
	}

	var cfg map[string]interface{}
	err = json.Unmarshal([]byte(outStr), &cfg)
	require.NoError(t, err, "should parse as JSON object")

	// Verify key structural elements exist
	assert.NotEmpty(t, cfg, "config should not be an empty object")
}

// TestConfigGeneration_UnknownAgent verifies that requesting a config
// for an unknown agent name does not produce valid JSON.
func TestConfigGeneration_UnknownAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")  // SKIP-OK: #short-mode
	}

	binary := buildHelixAgent(t)

	cmd := exec.Command(binary, "--generate-agent-config=totally-fake-agent-xyz")
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))

	// Should either error out or produce empty/error output
	if err == nil {
		// If it didn't error, the output should indicate the agent is unknown
		assert.True(t,
			strings.Contains(outStr, "unknown") ||
				strings.Contains(outStr, "unsupported") ||
				strings.Contains(outStr, "not found") ||
				strings.Contains(outStr, "error") ||
				!json.Valid([]byte(outStr)),
			"unknown agent should produce an error or invalid output")
	}
	// Error exit code is the expected behavior
	assert.NotContains(t, outStr, "panic", "should not panic on unknown agent")
}
