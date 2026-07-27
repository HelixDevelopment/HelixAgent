package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain_BinaryBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary build test in short mode") // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "generate-constitution")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = filepath.Join("..", "..", "cmd", "generate-constitution")
	output, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "failed to build generate-constitution: %s", string(output))

	info, err := os.Stat(binaryPath)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
	assert.Greater(t, info.Size(), int64(0))
}

func TestMain_GeneratesConstitution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping constitution generation in short mode") // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()

	cmd := exec.Command("go", "run", ".")
	cmd.Dir = filepath.Join("..", "..", "cmd", "generate-constitution")
	cmd.Env = append(os.Environ(), "PROJECT_ROOT="+tmpDir)

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "constitution generation should succeed: %s", string(output))

	assert.FileExists(t, filepath.Join(tmpDir, "CONSTITUTION.json"))
	assert.FileExists(t, filepath.Join(tmpDir, "CONSTITUTION_REPORT.md"))
}

func TestMain_InvalidProjectRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping invalid path test in short mode") // SKIP-OK: #short-mode
	}

	cmd := exec.Command("go", "run", ".")
	cmd.Dir = filepath.Join("..", "..", "cmd", "generate-constitution")
	cmd.Env = append(os.Environ(), "PROJECT_ROOT=/nonexistent/path/12345")

	output, err := cmd.CombinedOutput()
	assert.Error(t, err, "should fail with invalid project root")
	t.Logf("expected error output: %s", string(output))
}

func TestMain_ConstitutionManagerImport(t *testing.T) {
	importPath := "dev.helix.agent/internal/services"
	cmd := exec.Command("go", "list", importPath)
	output, err := cmd.Output()
	require.NoError(t, err, "services package should be importable")
	assert.Contains(t, string(output), importPath)
}

func TestMain_ConstitutionFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping format validation in short mode") // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()

	cmd := exec.Command("go", "run", ".")
	cmd.Dir = filepath.Join("..", "..", "cmd", "generate-constitution")
	cmd.Env = append(os.Environ(), "PROJECT_ROOT="+tmpDir)

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "constitution generation should succeed: %s", string(output))

	content, err := os.ReadFile(filepath.Join(tmpDir, "CONSTITUTION.json"))
	require.NoError(t, err)

	assert.Contains(t, string(content), "\"version\"")
	assert.Contains(t, string(content), "\"rules\"")
}

func TestMain_ConstitutionSync(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping sync test in short mode") // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()

	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")

	_ = os.WriteFile(agentsPath, []byte("# Test AGENTS\n"), 0644)
	_ = os.WriteFile(claudePath, []byte("# Test CLAUDE\n"), 0644)

	cmd := exec.Command("go", "run", ".")
	cmd.Dir = filepath.Join("..", "..", "cmd", "generate-constitution")
	cmd.Env = append(os.Environ(), "PROJECT_ROOT="+tmpDir)

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "constitution generation should succeed: %s", string(output))

	assert.FileExists(t, filepath.Join(tmpDir, "CONSTITUTION.json"))
	assert.FileExists(t, filepath.Join(tmpDir, "CONSTITUTION_REPORT.md"))
}
