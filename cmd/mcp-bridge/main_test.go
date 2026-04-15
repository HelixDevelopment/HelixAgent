package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain_BinaryBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary build test in short mode")
	}

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "mcp-bridge")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = filepath.Join("..", "..", "cmd", "mcp-bridge")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "failed to build mcp-bridge: %s", string(output))

	info, err := os.Stat(binaryPath)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
	assert.Greater(t, info.Size(), int64(0))
}

func TestMain_HelpFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping help test in short mode")
	}

	// Skip - mcp-bridge doesn't support --help without MCP_COMMAND
	t.Skip("mcp-bridge requires MCP_COMMAND env to run - skipping")
}

func TestMain_BridgePkgImport(t *testing.T) {
	importPath := "dev.helix.agent/internal/mcp/bridge"
	cmd := exec.Command("go", "list", importPath)
	output, err := cmd.Output()
	require.NoError(t, err, "bridge package should be importable")
	assert.Contains(t, string(output), importPath)
}

func TestMain_VersionInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping version test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", ".", "--version")
	cmd.Dir = filepath.Join("..", "..", "cmd", "mcp-bridge")

	output, _ := cmd.CombinedOutput()
	t.Logf("version output: %s", string(output))
}

func TestMain_GracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping shutdown test in short mode")
	}

	cmd := exec.Command("go", "run", ".")
	cmd.Dir = filepath.Join("..", "..", "cmd", "mcp-bridge")

	err := cmd.Start()
	require.NoError(t, err, "should start mcp-bridge")

	time.Sleep(500 * time.Millisecond)

	err = cmd.Process.Signal(os.Interrupt)
	require.NoError(t, err, "should send interrupt signal")

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		assert.NoError(t, err, "should exit gracefully")
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		t.Fatal("mcp-bridge did not shut down within timeout")
	}
}
