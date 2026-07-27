// Package automation provides full boot automation tests.
// These tests verify that the HelixAgent binary builds successfully
// and exposes the expected CLI flags.
package automation

import (
	"encoding/json"
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
		t.Skip("skipping build test in short mode") // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "helixagent")

	cmd := repoCommand(t, "go", "build", "-o", outputPath, "./cmd/helixagent")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "helixagent should compile: %s", string(output))
}

// TestFullBoot_HelpFlag verifies that the compiled binary responds
// to --help without error.
func TestFullBoot_HelpFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode") // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "helixagent")

	buildCmd := repoCommand(t, "go", "build", "-o", outputPath, "./cmd/helixagent")
	buildOut, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "build failed: %s", string(buildOut))

	helpCmd := repoCommand(t, outputPath, "--help")
	helpOut, helpErr := helpCmd.CombinedOutput()
	outStr := string(helpOut)

	// The previous assertion ended in `|| len(outStr) > 0`, which makes the
	// whole disjunction true for any non-empty output — including a stack trace
	// or an error message. It could not fail. These assertions check the usage
	// text is actually printed and documents the flags the rest of this suite
	// drives.
	require.NoError(t, helpErr, "--help should exit cleanly: %s", outStr)
	assert.True(t, strings.Contains(outStr, "Usage:") || strings.Contains(outStr, "usage:"),
		"--help should print a usage section; got: %s", truncate(outStr, 300))
	for _, flag := range []string{"-config", "-generate-agent-config", "-version"} {
		assert.Contains(t, outStr, flag, "--help should document the %s flag", flag)
	}
}

// TestFullBoot_GenerateAgentConfigFlag verifies the binary accepts
// the --generate-agent-config flag without crashing.
func TestFullBoot_GenerateAgentConfigFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode") // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "helixagent")

	buildCmd := repoCommand(t, "go", "build", "-o", outputPath, "./cmd/helixagent")
	buildOut, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "build failed: %s", string(buildOut))

	// Run with a known agent name; the config document goes to stdout and the
	// binary's logs go to stderr, so the streams are read separately (merging
	// them yields log lines interleaved with JSON, which no correct binary can
	// make parse).
	stdout, stderr, err := generateAgentConfig(t, outputPath, "opencode")

	assert.NotContains(t, stdout+stderr, "panic", "flag processing should not panic")
	require.NoError(t, err, "config generation should succeed; stderr:\n%s", stderr)
	assert.True(t, json.Valid([]byte(stdout)),
		"successful config generation should write a JSON document to stdout; got: %s",
		truncate(stdout, 300))
}
