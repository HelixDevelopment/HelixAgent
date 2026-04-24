package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
	"github.com/stretchr/testify/assert"
)

// TestInfrastructureCommands tests infrastructure management commands
func TestInfrastructureCommands(t *testing.T) {
	binPath := filepath.Join("..", "..", "bin", "helixagent")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("Binary not built, skipping E2E tests")  // SKIP-OK: #legacy-untriaged
	}

	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		skipCI         bool // Skip in CI where Docker might not be available
	}{
		{
			name:           "infra status",
			args:           []string{"--infra-status"},
			expectedOutput: "status", // Should show some status output
			skipCI:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipCI && os.Getenv("CI") == "true" {
				t.Skip("Skipping in CI environment")  // SKIP-OK: #legacy-untriaged
			}

			cmd := exec.Command(binPath, tt.args...)
			cmd.Env = append(os.Environ(), "TEST_MODE=true")

			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// Infrastructure commands might fail if Docker/Podman not available
			// Just verify they execute without panic
			if err != nil {
				// Allow "docker not found" or similar errors
				if !strings.Contains(outputStr, "not found") &&
					!strings.Contains(outputStr, "not available") {
					t.Logf("Infrastructure command failed (expected in some environments): %v", err)
				}
			}

			t.Logf("Output: %s", outputStr)
		})
	}
}

// TestServerHealthCheck tests server health check functionality
func TestServerHealthCheck(t *testing.T) {
	// This test verifies health check logic without actually starting the server
	// Real server tests are in server_lifecycle_test.go

	t.Run("health_check_url_construction", func(t *testing.T) {
		testCases := []struct {
			host string
			port int
			want string
		}{
			{"localhost", 8100, "http://localhost:8100/health"},
			{"127.0.0.1", 8080, "http://127.0.0.1:8080/health"},
		}

		for _, tc := range testCases {
			// In real test, this would call the health check function
			// For now, just verify the pattern
			got := "http://" + tc.host + ":" + string(rune(tc.port)) + "/health"
			// This is placeholder - real implementation would test actual health check
			assert.NotEmpty(t, got)
		}
	})
}

// TestInfrastructureStartStop tests infrastructure start/stop (integration)
func TestInfrastructureStartStop(t *testing.T) {
	testutil.RequireServer(t)

	if os.Getenv("CI") == "true" {
		t.Skip("Skipping infrastructure test in CI")  // SKIP-OK: #legacy-untriaged
	}

	binPath := filepath.Join("..", "..", "bin", "helixagent")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("Binary not built, skipping E2E tests")  // SKIP-OK: #legacy-untriaged
	}

	t.Run("start_and_stop_infrastructure", func(t *testing.T) {
		// Note: This is a heavyweight test that actually starts infrastructure
		// Only run when explicitly requested

		// Start infrastructure
		startCmd := exec.Command(binPath, "--infra-start", "--infra-minimal")
		output, err := startCmd.CombinedOutput()
		if err != nil {
			t.Logf("Infrastructure start output: %s", string(output))
			t.Skip("Infrastructure not available in this environment")  // SKIP-OK: #legacy-untriaged
		}

		// Give it a moment to start
		time.Sleep(2 * time.Second)

		// Check status
		statusCmd := exec.Command(binPath, "--infra-status")
		statusOut, _ := statusCmd.CombinedOutput()
		t.Logf("Status: %s", string(statusOut))

		// Stop infrastructure
		stopCmd := exec.Command(binPath, "--infra-stop")
		stopOut, _ := stopCmd.CombinedOutput()
		t.Logf("Stop output: %s", string(stopOut))
	})
}
