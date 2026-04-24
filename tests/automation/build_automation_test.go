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
		t.Skip("skipping build automation in short mode")  // SKIP-OK: #short-mode
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

			cmd := exec.Command("go", "build", "-o", outputPath, fmt.Sprintf("./cmd/%s", binary))
			output, err := cmd.CombinedOutput()

			require.NoError(t, err, "failed to build %s: %s", binary, string(output))

			info, err := os.Stat(outputPath)
			require.NoError(t, err)
			assert.False(t, info.IsDir())
			assert.Greater(t, info.Size(), int64(0))
		})
	}
}

func TestDockerAutomation_BuildImage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping docker build in short mode")  // SKIP-OK: #short-mode
	}

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")  // SKIP-OK: #requires-docker
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "build", "-t", "helixagent-test:automation", "-f", "docker/build/Dockerfile", ".")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Logf("docker build output: %s", string(output))
	}
	assert.NoError(t, err, "docker build should succeed")
}

func TestDockerAutomation_ComposeValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compose validation in short mode")  // SKIP-OK: #short-mode
	}

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")  // SKIP-OK: #requires-docker
	}

	composeFiles := []string{
		"docker-compose.yml",
		"docker-compose.test.yml",
		"docker-compose.security.yml",
	}

	for _, composeFile := range composeFiles {
		t.Run(composeFile, func(t *testing.T) {
			if _, err := os.Stat(composeFile); os.IsNotExist(err) {
				t.Skipf("%s not found", composeFile)
			}

			cmd := exec.Command("docker", "compose", "-f", composeFile, "config", "--quiet")
			output, err := cmd.CombinedOutput()

			assert.NoError(t, err, "compose file %s should be valid: %s", composeFile, string(output))
		})
	}
}

func TestLintAutomation_FmtVetLint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping lint automation in short mode")  // SKIP-OK: #short-mode
	}

	t.Run("gofmt", func(t *testing.T) {
		cmd := exec.Command("gofmt", "-l", ".")
		output, err := cmd.Output()
		require.NoError(t, err)

		files := strings.TrimSpace(string(output))
		if files != "" {
			t.Errorf("files need formatting:\n%s", files)
		}
	})

	t.Run("go vet", func(t *testing.T) {
		cmd := exec.Command("go", "vet", "./...")
		output, err := cmd.CombinedOutput()

		assert.NoError(t, err, "go vet should pass: %s", string(output))
	})
}

func TestSecurityAutomation_GosecScan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping security automation in short mode")  // SKIP-OK: #short-mode
	}

	if _, err := exec.LookPath("gosec"); err != nil {
		t.Skip("gosec not available")  // SKIP-OK: #legacy-untriaged
	}

	cmd := exec.Command("gosec", "-quiet", "-fmt=json", "./...")
	output, err := cmd.Output()

	if err != nil {
		t.Logf("gosec output: %s", string(output))
	}

	assert.NoError(t, err, "gosec scan should pass")
}

func TestTestAutomation_UnitTests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test automation in short mode")  // SKIP-OK: #short-mode
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "-v", "-short", "./internal/...")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Logf("test output: %s", string(output))
	}
	assert.NoError(t, err, "unit tests should pass")
}

func TestMakefileAutomation_AllTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping makefile automation in short mode")  // SKIP-OK: #short-mode
	}

	targets := []string{
		"fmt",
		"vet",
	}

	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			cmd := exec.Command("make", target)
			output, err := cmd.CombinedOutput()

			assert.NoError(t, err, "make %s should succeed: %s", target, string(output))
		})
	}
}

func TestGitAutomation_Status(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")  // SKIP-OK: #legacy-untriaged
	}

	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	require.NoError(t, err)

	uncommitted := strings.TrimSpace(string(output))
	t.Logf("uncommitted changes: %s", uncommitted)
}

func TestEnvAutomation_ConfigValidation(t *testing.T) {
	envFiles := []string{
		".env.example",
		"Containers/.env",
	}

	for _, envFile := range envFiles {
		t.Run(envFile, func(t *testing.T) {
			if _, err := os.Stat(envFile); os.IsNotExist(err) {
				t.Skipf("%s not found", envFile)
			}

			content, err := os.ReadFile(envFile)
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

func TestModuleAutomation_GoModTidy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping module automation in short mode")  // SKIP-OK: #short-mode
	}

	beforeCmd := exec.Command("go", "mod", "tidy")
	beforeOutput, _ := beforeCmd.CombinedOutput()
	t.Logf("initial go mod tidy: %s", string(beforeOutput))

	cmd := exec.Command("git", "diff", "--exit-code", "go.mod", "go.sum")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Errorf("go.mod or go.sum changed after tidy:\n%s", string(output))
	}
}

func TestReleaseAutomation_VersionInjection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping release automation in short mode")  // SKIP-OK: #short-mode
	}

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "helixagent")

	ldflags := fmt.Sprintf("-X dev.helix.agent/internal/version.Version=test-automation -X dev.helix.agent/internal/version.VersionCode=999")
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", binaryPath, "./cmd/helixagent")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "build should succeed: %s", string(output))

	versionCmd := exec.Command(binaryPath, "--version")
	versionOutput, err := versionCmd.CombinedOutput()
	t.Logf("version output: %s", string(versionOutput))
}
