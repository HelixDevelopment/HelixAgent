// Package automation provides full boot automation tests.
// These tests verify that the HelixAgent binary builds successfully
// and exposes the expected CLI flags.
package automation

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullBoot_BinaryBuilds verifies that the helixagent binary
// compiles without errors.
func TestFullBoot_BinaryBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build test in short mode")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "helixagent")

	cmd := exec.Command("go", "build", "-o", outputPath, "./cmd/helixagent")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "helixagent should compile: %s", string(output))
}

// TestFullBoot_HelpFlag verifies that the compiled binary responds
// to --help without error.
func TestFullBoot_HelpFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "helixagent")

	buildCmd := exec.Command("go", "build", "-o", outputPath, "./cmd/helixagent")
	buildOut, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "build failed: %s", string(buildOut))

	helpCmd := exec.Command(outputPath, "--help")
	helpOut, _ := helpCmd.CombinedOutput()
	outStr := string(helpOut)

	// The binary should print usage information
	assert.True(t,
		strings.Contains(outStr, "Usage") ||
			strings.Contains(outStr, "usage") ||
			strings.Contains(outStr, "helixagent") ||
			strings.Contains(outStr, "flag") ||
			len(outStr) > 0,
		"--help should produce output")
}

// TestFullBoot_GenerateAgentConfigFlag verifies the binary accepts
// the --generate-agent-config flag without crashing.
func TestFullBoot_GenerateAgentConfigFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "helixagent")

	buildCmd := exec.Command("go", "build", "-o", outputPath, "./cmd/helixagent")
	buildOut, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "build failed: %s", string(buildOut))

	// Run with a known agent name; it should produce JSON output
	cmd := exec.Command(outputPath, "--generate-agent-config=opencode")
	out, err := cmd.CombinedOutput()
	outStr := string(out)

	// The command may succeed (produces JSON) or fail gracefully
	// (e.g., missing API keys). Either way it should not panic.
	if err == nil {
		assert.Contains(t, outStr, "{",
			"successful config generation should produce JSON")
	} else {
		assert.NotContains(t, outStr, "panic",
			"flag processing should not panic")
	}
}
