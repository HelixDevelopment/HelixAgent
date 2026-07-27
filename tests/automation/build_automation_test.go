package automation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAutomation_AllBinaries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build automation in short mode") // SKIP-OK: #short-mode
	}

	binaries := []string{
		"helixagent",
		"api",
		"grpc-server",
		"cognee-mock",
		"sanity-check",
		"mcp-bridge",
		"generate-constitution",
	}

	tmpDir := t.TempDir()

	for _, binary := range binaries {
		t.Run(binary, func(t *testing.T) {
			outputPath := filepath.Join(tmpDir, binary)

			cmd := repoCommand(t, "go", "build", "-o", outputPath, fmt.Sprintf("./cmd/%s", binary))
			output, err := cmd.CombinedOutput()

			require.NoError(t, err, "failed to build %s: %s", binary, string(output))

			info, err := os.Stat(outputPath)
			require.NoError(t, err)
			assert.False(t, info.IsDir())
			assert.Greater(t, info.Size(), int64(0))
		})
	}
}

// TestDockerAutomation_BuildImage builds the project's container build
// definition.
//
// This test previously targeted `docker/build/Dockerfile`, a path that has never
// existed in this repository (`git log -- docker/build/Dockerfile` is empty), so
// the test could never have passed. The real, tracked build definition — the one
// scripts/build/build-release.sh drives — is docker/build/Dockerfile.builder.
func TestDockerAutomation_BuildImage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker build in short mode") // SKIP-OK: #short-mode
	}

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available") // SKIP-OK: #requires-docker
	}

	const dockerfile = "docker/build/Dockerfile.builder"
	_, err := os.Stat(repoPath(t, dockerfile))
	require.NoError(t, err, "%s must exist for the release build pipeline", dockerfile)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := repoCommandContext(t, ctx, "docker", "build",
		"-t", "helixagent-test:automation", "-f", dockerfile, ".")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Logf("docker build output: %s", string(output))
	}
	require.NoError(t, err, "docker build should succeed")
	assert.Contains(t, string(output), "helixagent-test:automation",
		"build output should reference the tagged image")
}

func TestDockerAutomation_ComposeValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compose validation in short mode") // SKIP-OK: #short-mode
	}

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available") // SKIP-OK: #requires-docker
	}

	composeFiles := []string{
		"docker-compose.yml",
		"docker-compose.test.yml",
		"docker-compose.security.yml",
	}

	for _, composeFile := range composeFiles {
		t.Run(composeFile, func(t *testing.T) {
			_, err := os.Stat(repoPath(t, composeFile))
			require.NoError(t, err, "%s must exist", composeFile)

			cmd := repoCommand(t, "docker", "compose", "-f", composeFile, "config", "--quiet")
			output, err := cmd.CombinedOutput()

			assert.NoError(t, err, "compose file %s should be valid: %s", composeFile, string(output))
		})
	}
}

func TestLintAutomation_FmtVetLint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping lint automation in short mode") // SKIP-OK: #short-mode
	}

	t.Run("gofmt", func(t *testing.T) {
		// Scope: the Go files THIS repository tracks. A bare `gofmt -l .` at the
		// repository root also walks the checked-out submodules, whose contents
		// are owned by other repositories and cannot be corrected from here.
		// `git ls-files` lists submodules as gitlinks, not as their contents, so
		// this is exactly the set of files this repo is responsible for.
		listCmd := repoCommand(t, "git", "ls-files", "*.go")
		listed, err := listCmd.Output()
		require.NoError(t, err, "git ls-files should succeed")

		var goFiles []string
		for _, f := range strings.Split(string(listed), "\n") {
			if f = strings.TrimSpace(f); f != "" {
				goFiles = append(goFiles, f)
			}
		}
		require.NotEmpty(t, goFiles, "repository should track Go files")

		fmtCmd := repoCommand(t, "gofmt", append([]string{"-l"}, goFiles...)...)
		output, err := fmtCmd.Output()
		require.NoError(t, err, "gofmt should run successfully")

		var unformatted []string
		for _, f := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if f = strings.TrimSpace(f); f != "" {
				unformatted = append(unformatted, f)
			}
		}
		if len(unformatted) > 0 {
			shown := unformatted
			if len(shown) > 20 {
				shown = shown[:20]
			}
			t.Errorf("%d of %d tracked Go files need formatting (run `gofmt -w`); first %d:\n%s",
				len(unformatted), len(goFiles), len(shown), strings.Join(shown, "\n"))
		}
	})

	t.Run("go vet", func(t *testing.T) {
		cmd := repoCommand(t, "go", "vet", "./...")
		output, err := cmd.CombinedOutput()

		assert.NoError(t, err, "go vet should pass: %s", string(output))
	})
}

func TestSecurityAutomation_GosecScan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security automation in short mode") // SKIP-OK: #short-mode
	}

	if _, err := exec.LookPath("gosec"); err != nil {
		t.Skip("gosec not available") // SKIP-OK: #legacy-untriaged
	}

	cmd := repoCommand(t, "gosec", "-quiet", "-fmt=json", "./...")
	output, err := cmd.Output()

	if err != nil {
		t.Logf("gosec output: %s", string(output))
	}
	assert.NoError(t, err, "gosec scan should pass")
}

func TestTestAutomation_UnitTests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test automation in short mode") // SKIP-OK: #short-mode
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := repoCommandContext(t, ctx, "go", "test", "-short", "./internal/...")
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Surface only the failing packages; the full log is tens of thousands
		// of lines and buries the finding.
		var failures []string
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(line, "FAIL") || strings.HasPrefix(line, "--- FAIL") {
				failures = append(failures, line)
			}
		}
		t.Logf("failing packages/tests:\n%s", strings.Join(failures, "\n"))
	}
	assert.NoError(t, err, "unit tests should pass")
}

func TestMakefileAutomation_AllTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping makefile automation in short mode") // SKIP-OK: #short-mode
	}

	// `make fmt` rewrites tracked source files across the whole repository. A
	// test must never mutate the working tree it is run in, so the fmt target is
	// resolved with `make -n` (which still fails, exit 2, when the target does
	// not exist — the failure mode this test exists to catch) while its
	// recipe is left for an operator to run deliberately. `make vet` is
	// side-effect free and is executed for real.
	targets := []struct {
		name    string
		args    []string
		dryRun  bool
		timeout time.Duration
	}{
		{name: "fmt", args: []string{"-n", "fmt"}, dryRun: true, timeout: 2 * time.Minute},
		{name: "vet", args: []string{"vet"}, timeout: 30 * time.Minute},
	}

	for _, target := range targets {
		t.Run(target.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), target.timeout)
			defer cancel()

			cmd := repoCommandContext(t, ctx, "make", target.args...)
			output, err := cmd.CombinedOutput()

			require.NoError(t, err, "make %v should succeed: %s", target.args, string(output))
			if target.dryRun {
				assert.NotEmpty(t, strings.TrimSpace(string(output)),
					"make -n %s should print the recipe it would run", target.name)
			}
		})
	}
}

func TestGitAutomation_Status(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	cmd := repoCommand(t, "git", "status", "--porcelain")
	output, err := cmd.Output()
	require.NoError(t, err)

	uncommitted := strings.TrimSpace(string(output))
	t.Logf("uncommitted changes: %s", uncommitted)
}

func TestEnvAutomation_ConfigValidation(t *testing.T) {
	envFiles := []string{
		".env.example",
		"containers/.env",
	}

	for _, envFile := range envFiles {
		t.Run(envFile, func(t *testing.T) {
			path := repoPath(t, envFile)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Skipf("%s not found (SKIP-OK: #unmarked-skip-needs-ticket)", envFile)
			}

			content, err := os.ReadFile(path)
			require.NoError(t, err)

			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				if !strings.Contains(line, "=") {
					t.Errorf("invalid env line in %s: %s", envFile, line)
				}
			}
		})
	}
}

// TestModuleAutomation_GoModTidy asserts go.mod / go.sum are already tidy.
//
// The previous implementation ran `go mod tidy` (which REWRITES go.mod and
// go.sum in the working tree) and then diffed against git. That made a test
// mutate tracked files, and made its verdict depend on whether those files
// happened to be dirty for unrelated reasons. `go mod tidy -diff` reports the
// same condition — it exits non-zero and prints the diff when tidying would
// change anything — without writing to the tree.
func TestModuleAutomation_GoModTidy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping module automation in short mode") // SKIP-OK: #short-mode
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := repoCommandContext(t, ctx, "go", "mod", "tidy", "-diff")
	output, err := cmd.CombinedOutput()

	assert.NoError(t, err, "go.mod/go.sum are not tidy; `go mod tidy` would change:\n%s", string(output))
}

// TestReleaseAutomation_VersionInjection asserts the release ldflags actually
// reach the binary. The previous implementation built with -ldflags and then
// only logged `--version` output, so it could not detect a broken injection
// path (wrong package path, renamed variable, stripped symbol).
func TestReleaseAutomation_VersionInjection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping release automation in short mode") // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "helixagent")

	const injectedVersion = "test-automation"
	const injectedCode = "999"
	ldflags := fmt.Sprintf(
		"-X dev.helix.agent/internal/version.Version=%s -X dev.helix.agent/internal/version.VersionCode=%s",
		injectedVersion, injectedCode)

	cmd := repoCommand(t, "go", "build", "-ldflags", ldflags, "-o", binaryPath, "./cmd/helixagent")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "build should succeed: %s", string(output))

	versionCmd := repoCommand(t, binaryPath, "--version")
	versionOutput, err := versionCmd.CombinedOutput()
	outStr := string(versionOutput)
	t.Logf("version output: %s", outStr)

	require.NoError(t, err, "--version should exit cleanly: %s", outStr)
	assert.Contains(t, outStr, injectedVersion,
		"the version injected via -ldflags must be reported by the binary")
	assert.Contains(t, outStr, injectedCode,
		"the version code injected via -ldflags must be reported by the binary")
}
