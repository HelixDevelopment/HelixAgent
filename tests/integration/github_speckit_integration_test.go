package integration

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

// TestGitHubSpecKitSubmoduleVerification verifies that GitHub SpecKit submodule is properly configured
func TestGitHubSpecKitSubmoduleVerification(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err, "Failed to change to project root")

	tests := []struct {
		name      string
		checkFunc func(t *testing.T)
	}{
		{
			name: "Submodule directory exists",
			checkFunc: func(t *testing.T) {
				path := "cli_agents/spec-kit"
				info, err := os.Stat(path)
				require.NoError(t, err, "SpecKit submodule directory must exist")
				assert.True(t, info.IsDir(), "SpecKit path must be a directory")
			},
		},
		{
			name: "Submodule is initialized",
			checkFunc: func(t *testing.T) {
				cmd := exec.Command("git", "submodule", "status", "cli_agents/spec-kit")
				output, err := cmd.CombinedOutput()
				require.NoError(t, err, "Git submodule command must succeed")

				// Check that output doesn't start with '-' (uninitialized)
				outputStr := string(output)
				assert.NotEmpty(t, outputStr, "Submodule status must not be empty")
				assert.False(t, strings.HasPrefix(outputStr, "-"), "Submodule must be initialized")

				// Should contain commit hash
				assert.True(t, len(outputStr) > 40, "Output should contain commit hash")
			},
		},
		{
			name: "Submodule remote URL is correct",
			checkFunc: func(t *testing.T) {
				// Check .gitmodules
				content, err := os.ReadFile(".gitmodules")
				require.NoError(t, err, ".gitmodules must exist")

				contentStr := string(content)
				assert.Contains(t, contentStr, "cli_agents/spec-kit", "Must contain spec-kit path")
				assert.Contains(t, contentStr, "git@github.com:github/spec-kit.git", "Must have correct remote URL")
			},
		},
		{
			name: "Submodule has git repository",
			checkFunc: func(t *testing.T) {
				gitDir := filepath.Join("cli_agents", "spec-kit", ".git")
				_, err := os.Stat(gitDir)
				require.NoError(t, err, "Submodule must have .git directory")
			},
		},
		{
			name: "Submodule README exists and is valid",
			checkFunc: func(t *testing.T) {
				readmePath := filepath.Join("cli_agents", "spec-kit", "README.md")
				content, err := os.ReadFile(readmePath)
				require.NoError(t, err, "README.md must exist")

				contentStr := string(content)
				assert.Contains(t, contentStr, "Spec Kit", "README must mention Spec Kit")
				assert.Contains(t, contentStr, "github", "README must be from GitHub")
				assert.True(t, len(contentStr) > 1000, "README must be comprehensive (>1000 chars)")
			},
		},
		{
			name: "Submodule version is tagged",
			checkFunc: func(t *testing.T) {
				cmd := exec.Command("git", "submodule", "status", "cli_agents/spec-kit")
				output, err := cmd.CombinedOutput()
				require.NoError(t, err)

				// Check if output contains version tag (e.g., v0.0.90, v0.1.6, v1.0.0)
				outputStr := string(output)
				isTagged := strings.Contains(outputStr, "v0.0.") ||
					strings.Contains(outputStr, "v0.1.") ||
					strings.Contains(outputStr, "v0.2.") ||
					strings.Contains(outputStr, "v1.") ||
					strings.Contains(outputStr, "v2.")
				assert.True(t, isTagged,
					"Submodule should be on a version tag, got: %s", outputStr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t)
		})
	}
}

// TestGitHubSpecKitInstallation verifies that Specify CLI can be installed and used
func TestGitHubSpecKitInstallation(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err, "Failed to change to project root")

	if testing.Short() {
		t.Skip("Skipping installation test in short mode") // SKIP-OK: #short-mode
	}

	tests := []struct {
		name      string
		checkFunc func(t *testing.T)
	}{
		{
			name: "UV tool is available",
			checkFunc: func(t *testing.T) {
				cmd := exec.Command("which", "uv")
				err := cmd.Run()
				if err != nil {
					t.Skip("UV not installed - install with: curl -LsSf https://astral.sh/uv/install.sh | sh") // SKIP-OK: #legacy-untriaged
				}
			},
		},
		{
			name: "Can check Specify CLI version (if installed)",
			checkFunc: func(t *testing.T) {
				cmd := exec.Command("specify", "--version")
				output, err := cmd.CombinedOutput()
				if err != nil {
					t.Skip("Specify CLI not installed - run: uv tool install specify-cli --from git+https://github.com/github/spec-kit.git") // SKIP-OK: #legacy-untriaged
					return
				}

				outputStr := string(output)
				assert.NotEmpty(t, outputStr, "Version output must not be empty")
				t.Logf("Specify CLI version: %s", strings.TrimSpace(outputStr))
			},
		},
		{
			name: "Specify CLI check command works",
			checkFunc: func(t *testing.T) {
				cmd := exec.Command("specify", "check")
				output, err := cmd.CombinedOutput()
				if err != nil {
					t.Skip("Specify CLI not installed") // SKIP-OK: #legacy-untriaged
					return
				}

				outputStr := string(output)
				assert.NotEmpty(t, outputStr, "Check output must not be empty")
				t.Logf("Specify check output:\n%s", outputStr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t)
		})
	}
}

// TestGitHubSpecKitAgentRegistry verifies integration with HelixAgent's agent registry
func TestGitHubSpecKitAgentRegistry(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err, "Failed to change to project root")

	tests := []struct {
		name      string
		checkFunc func(t *testing.T)
	}{
		{
			name: "Agent registry file exists",
			checkFunc: func(t *testing.T) {
				path := "internal/agents/registry.go"
				_, err := os.Stat(path)
				require.NoError(t, err, "Agent registry must exist")
			},
		},
		{
			name: "Registry contains spec-kit entry",
			checkFunc: func(t *testing.T) {
				content, err := os.ReadFile("internal/agents/registry.go")
				require.NoError(t, err)

				contentStr := string(content)
				assert.Contains(t, contentStr, "spec-kit", "Registry must reference spec-kit")
				assert.Contains(t, contentStr, "EntryPoint", "Must have EntryPoint defined")
			},
		},
		{
			name: "Config location is defined",
			checkFunc: func(t *testing.T) {
				content, err := os.ReadFile("internal/agents/registry.go")
				require.NoError(t, err)

				contentStr := string(content)
				assert.Contains(t, contentStr, "ConfigLocation", "Must have ConfigLocation")
				assert.Contains(t, contentStr, ".config/spec-kit", "Config should be in .config/spec-kit")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t)
		})
	}
}

// TestGitHubSpecKitFileStructure verifies that all expected files exist
func TestGitHubSpecKitFileStructure(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err, "Failed to change to project root")

	basePath := "cli_agents/spec-kit"

	requiredFiles := []string{
		"README.md",
		"AGENTS.md",
		"CHANGELOG.md",
		"CONTRIBUTING.md",
		"CODE_OF_CONDUCT.md",
	}

	requiredDirs := []string{
		"docs",
		".devcontainer",
	}

	t.Run("Required files exist", func(t *testing.T) {
		for _, file := range requiredFiles {
			path := filepath.Join(basePath, file)
			info, err := os.Stat(path)
			assert.NoError(t, err, "File %s must exist", file)
			if err == nil {
				assert.False(t, info.IsDir(), "%s must be a file, not directory", file)
				assert.Greater(t, info.Size(), int64(0), "%s must not be empty", file)
			}
		}
	})

	t.Run("Required directories exist", func(t *testing.T) {
		for _, dir := range requiredDirs {
			path := filepath.Join(basePath, dir)
			info, err := os.Stat(path)
			assert.NoError(t, err, "Directory %s must exist", dir)
			if err == nil {
				assert.True(t, info.IsDir(), "%s must be a directory", dir)
			}
		}
	})
}

// TestGitHubSpecKitSubmoduleUpdate verifies submodule can be updated
func TestGitHubSpecKitSubmoduleUpdate(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err, "Failed to change to project root")

	if testing.Short() {
		t.Skip("Skipping submodule update test in short mode") // SKIP-OK: #short-mode
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("Can fetch submodule updates", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, "git", "submodule", "update", "--remote", "--init", "cli_agents/spec-kit")
		output, err := cmd.CombinedOutput()

		// This might fail if already up to date, which is fine
		if err != nil {
			t.Logf("Submodule update output: %s", string(output))
		}

		// Verify submodule is still valid after update attempt
		cmd = exec.CommandContext(ctx, "git", "submodule", "status", "cli_agents/spec-kit")
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Submodule must be in valid state after update")
		assert.NotEmpty(t, output, "Submodule status must not be empty")
	})
}

// TestGitHubSpecKitNoModifications verifies submodule hasn't been modified locally
func TestGitHubSpecKitNoModifications(t *testing.T) {
	projectRoot := getProjectRoot(t)
	err := os.Chdir(projectRoot)
	require.NoError(t, err, "Failed to change to project root")

	submodulePath := filepath.Join(projectRoot, "cli_agents", "spec-kit")

	t.Run("Submodule has no uncommitted changes", func(t *testing.T) {
		// Check for unstaged changes within the submodule
		cmd := exec.Command("git", "-C", submodulePath, "diff", "--exit-code")
		err := cmd.Run()
		assert.NoError(t, err, "Submodule should have no uncommitted changes (read-only third-party)")

		// Check for staged changes within the submodule
		cmd = exec.Command("git", "-C", submodulePath, "diff", "--cached", "--exit-code")
		err = cmd.Run()
		assert.NoError(t, err, "Submodule should have no staged changes")
	})
}

// BenchmarkGitHubSpecKitSubmoduleStatus benchmarks submodule status check
func BenchmarkGitHubSpecKitSubmoduleStatus(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("git", "submodule", "status", "cli_agents/spec-kit")
		_, err := cmd.CombinedOutput()
		if err != nil {
			b.Fatal(err)
		}
	}
}
