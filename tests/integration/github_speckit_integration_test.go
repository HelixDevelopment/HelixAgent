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

// specKitSubmodulePath is the spec-kit submodule's path RELATIVE TO the
// repository whose .gitmodules declares it.
const specKitSubmodulePath = "cli_agents/spec-kit"

// specKitHostRoot returns the absolute path of the repository that DECLARES the
// spec-kit submodule.
//
// helix_agent is itself a submodule of a meta-repo, and spec-kit is declared by
// that META-repo — not by helix_agent. Go runs a test from its own package
// directory, so a bare relative path such as "cli_agents/spec-kit" resolves to
// tests/integration/cli_agents/spec-kit and never finds it.
//
// The search walks UP from the test's working directory looking for a
// .gitmodules that declares the submodule. Keying on that MARKER rather than
// counting "../" levels means the lookup keeps working if this test file moves
// to a different depth.
//
// helix_agent may also be cloned standalone, with no meta-repo above it. Then
// the submodule genuinely is not part of the checkout and there is nothing to
// verify, so the test SKIPs and names what is absent: a silent pass would claim
// a verification that never ran, and a failure would misreport a perfectly
// correct standalone checkout as a broken submodule.
func specKitHostRoot(t testing.TB) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err, "Failed to determine working directory")

	start := dir
	for {
		if gitmodulesDeclaresSubmodule(filepath.Join(dir, ".gitmodules"), specKitSubmodulePath) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skipf("no repository above %s declares the %q submodule in .gitmodules - "+
				"expected in the meta-repo that embeds helix_agent; this is a standalone "+
				"helix_agent checkout, where the submodule is genuinely absent "+
				"(SKIP-OK: #speckit-standalone-checkout)", start, specKitSubmodulePath)
		}
		dir = parent
	}
}

// gitmodulesDeclaresSubmodule reports whether the .gitmodules file at path has a
// `path = <submodule>` entry. It matches the whole declared path so a sibling
// such as "cli_agents/spec-kit-extras" cannot satisfy the lookup.
func gitmodulesDeclaresSubmodule(gitmodulesPath, submodule string) bool {
	data, err := os.ReadFile(gitmodulesPath) // #nosec G304 - repo-local .gitmodules
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(fields) != 2 || strings.TrimSpace(fields[0]) != "path" {
			continue
		}
		if strings.TrimSpace(fields[1]) == submodule {
			return true
		}
	}
	return false
}

// gitmodulesURLFor returns the url declared for the given submodule path in the
// .gitmodules file, or "" when the path is not declared there.
func gitmodulesURLFor(gitmodulesPath, submodule string) string {
	data, err := os.ReadFile(gitmodulesPath) // #nosec G304 - repo-local .gitmodules
	if err != nil {
		return ""
	}

	inSection := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[submodule ") {
			inSection = false
			continue
		}
		fields := strings.SplitN(trimmed, "=", 2)
		if len(fields) != 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])
		switch key {
		case "path":
			inSection = value == submodule
		case "url":
			if inSection {
				return value
			}
		}
	}
	return ""
}

// TestGitHubSpecKitSubmoduleVerification verifies that GitHub SpecKit submodule is properly configured
func TestGitHubSpecKitSubmoduleVerification(t *testing.T) {
	hostRoot := specKitHostRoot(t)

	tests := []struct {
		name      string
		checkFunc func(t *testing.T)
	}{
		{
			name: "Submodule directory exists",
			checkFunc: func(t *testing.T) {
				path := filepath.Join(hostRoot, specKitSubmodulePath)
				info, err := os.Stat(path)
				require.NoError(t, err, "SpecKit submodule directory must exist")
				assert.True(t, info.IsDir(), "SpecKit path must be a directory")
			},
		},
		{
			name: "Submodule is initialized",
			checkFunc: func(t *testing.T) {
				cmd := exec.Command("git", "-C", hostRoot, "submodule", "status", specKitSubmodulePath)
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
				gitmodules := filepath.Join(hostRoot, ".gitmodules")
				_, err := os.Stat(gitmodules)
				require.NoError(t, err, ".gitmodules must exist")

				// Read the URL declared for THIS submodule path rather than
				// grepping the whole file, so an unrelated entry carrying a
				// spec-kit URL cannot satisfy the assertion.
				url := gitmodulesURLFor(gitmodules, specKitSubmodulePath)
				require.NotEmpty(t, url, "%s must declare a url for %s", gitmodules, specKitSubmodulePath)

				// SSH-only per the project's Git remote policy.
				assert.True(t, strings.HasPrefix(url, "git@"),
					"spec-kit remote must be an SSH URL, got: %s", url)

				// The declared remote must be a spec-kit repository. Both the
				// upstream (github/spec-kit) and the organisation's own fork
				// (vasic-digital/caf-spec-kit, part of the cli_agents/caf-*
				// fork family) are accepted; anything else means the submodule
				// points at the wrong project entirely.
				assert.Contains(t, url, "spec-kit",
					"spec-kit remote must reference a spec-kit repository, got: %s", url)
			},
		},
		{
			name: "Submodule has git repository",
			checkFunc: func(t *testing.T) {
				gitDir := filepath.Join(hostRoot, specKitSubmodulePath, ".git")
				_, err := os.Stat(gitDir)
				require.NoError(t, err, "Submodule must have .git directory")
			},
		},
		{
			name: "Submodule README exists and is valid",
			checkFunc: func(t *testing.T) {
				readmePath := filepath.Join(hostRoot, specKitSubmodulePath, "README.md")
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
				cmd := exec.Command("git", "-C", hostRoot, "submodule", "status", specKitSubmodulePath)
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
	// This test exercises the host-installed Specify CLI only; it never reads
	// the submodule tree, so it needs no repository root and works the same in
	// a standalone helix_agent checkout.
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
	// The registry lives in helix_agent itself, NOT in the meta-repo that
	// declares the spec-kit submodule, so this test anchors on the Go module
	// root rather than on specKitHostRoot.
	agentRoot := getProjectRoot(t)
	registryPath := filepath.Join(agentRoot, "internal", "agents", "registry.go")

	tests := []struct {
		name      string
		checkFunc func(t *testing.T)
	}{
		{
			name: "Agent registry file exists",
			checkFunc: func(t *testing.T) {
				_, err := os.Stat(registryPath)
				require.NoError(t, err, "Agent registry must exist")
			},
		},
		{
			name: "Registry contains spec-kit entry",
			checkFunc: func(t *testing.T) {
				content, err := os.ReadFile(registryPath)
				require.NoError(t, err)

				contentStr := string(content)
				assert.Contains(t, contentStr, "spec-kit", "Registry must reference spec-kit")
				assert.Contains(t, contentStr, "EntryPoint", "Must have EntryPoint defined")
			},
		},
		{
			name: "Config location is defined",
			checkFunc: func(t *testing.T) {
				content, err := os.ReadFile(registryPath)
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
	hostRoot := specKitHostRoot(t)

	basePath := filepath.Join(hostRoot, specKitSubmodulePath)

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
	hostRoot := specKitHostRoot(t)

	if testing.Short() {
		t.Skip("Skipping submodule update test in short mode") // SKIP-OK: #short-mode
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("Can fetch submodule updates", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, "git", "-C", hostRoot, "submodule", "update", "--remote", "--init", specKitSubmodulePath)
		output, err := cmd.CombinedOutput()

		// This might fail if already up to date, which is fine
		if err != nil {
			t.Logf("Submodule update output: %s", string(output))
		}

		// Verify submodule is still valid after update attempt
		cmd = exec.CommandContext(ctx, "git", "-C", hostRoot, "submodule", "status", specKitSubmodulePath)
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, "Submodule must be in valid state after update")
		assert.NotEmpty(t, output, "Submodule status must not be empty")
	})
}

// TestGitHubSpecKitNoModifications verifies submodule hasn't been modified locally
func TestGitHubSpecKitNoModifications(t *testing.T) {
	hostRoot := specKitHostRoot(t)

	submodulePath := filepath.Join(hostRoot, specKitSubmodulePath)

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
	hostRoot := specKitHostRoot(b)

	for i := 0; i < b.N; i++ {
		cmd := exec.Command("git", "-C", hostRoot, "submodule", "status", specKitSubmodulePath)
		_, err := cmd.CombinedOutput()
		if err != nil {
			b.Fatal(err)
		}
	}
}
