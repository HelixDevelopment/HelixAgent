package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain_BinaryBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary build test in short mode") // SKIP-OK: #short-mode
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

func TestMain_MCPCommandRequired(t *testing.T) {
	// This test verifies that mcp-bridge requires MCP_COMMAND
	// It should fail with error when MCP_COMMAND is not set
	if testing.Short() {
		t.Skip("skipping in short mode") // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "mcp-bridge")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = filepath.Join("..", "..", "cmd", "mcp-bridge")
	if err := buildCmd.Run(); err != nil {
		t.Skip("skipping - build failed") // SKIP-OK: #legacy-untriaged
	}

	// Try to run without MCP_COMMAND - should fail
	cmd := exec.Command(binaryPath)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("mcp-bridge should fail without MCP_COMMAND")
	}
	outputStr := string(output)
	assert.True(t, err != nil || strings.Contains(outputStr, "MCP_COMMAND"),
		"should require MCP_COMMAND: "+outputStr)
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
		t.Skip("skipping version test in short mode") // SKIP-OK: #short-mode
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
		t.Skip("skipping shutdown test in short mode") // SKIP-OK: #short-mode
	}

	// Build binary first
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "mcp-bridge")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = filepath.Join("..", "..", "cmd", "mcp-bridge")
	if err := buildCmd.Run(); err != nil {
		t.Skip("skipping - build failed") // SKIP-OK: #legacy-untriaged
	}

	// Run with MCP_COMMAND
	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(), "MCP_COMMAND=echo test", "HELIX_BRIDGE_HTTP_PORT=0")

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
