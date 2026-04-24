package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIServerLifecycle tests the API server start/stop/restart
func TestAPIServerLifecycle(t *testing.T) {
	testutil.RequireServer(t)

	binPath := filepath.Join("..", "..", "bin", "helixagent")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("Binary not built, run 'make build' first")  // SKIP-OK: #legacy-untriaged
	}

	// Use a unique port to avoid conflicts
	port := "17061"
	baseURL := "http://localhost:" + port

	t.Run("server_starts_successfully", func(t *testing.T) {
		// Start server in background
		cmd := exec.Command(binPath)
		cmd.Env = append(os.Environ(),
			"PORT="+port,
			"GIN_MODE=test",
			"LOG_LEVEL=error",
		)

		// Capture output for debugging
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		err := cmd.Start()
		require.NoError(t, err, "Server should start")
		defer func() {
			if cmd.Process != nil {
				cmd.Process.Signal(syscall.SIGTERM)
				cmd.Wait()
			}
		}()

		// Wait for server to be ready (up to 10 seconds)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		serverReady := false
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for !serverReady {
			select {
			case <-ctx.Done():
				// Read any output for debugging
				outBytes, _ := io.ReadAll(stdout)
				errBytes, _ := io.ReadAll(stderr)
				t.Fatalf("Server did not become ready in time.\nStdout: %s\nStderr: %s",
					string(outBytes), string(errBytes))
			case <-ticker.C:
				resp, err := http.Get(baseURL + "/health")
				if err == nil && resp.StatusCode == 200 {
					resp.Body.Close()
					serverReady = true
				}
			}
		}

		assert.True(t, serverReady, "Server should respond to health checks")

		// Test basic API functionality
		resp, err := http.Get(baseURL + "/v1/models")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Models endpoint should return 200")

		// Graceful shutdown
		err = cmd.Process.Signal(syscall.SIGTERM)
		assert.NoError(t, err, "Should send SIGTERM successfully")

		// Wait for process to exit (up to 5 seconds)
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case <-time.After(5 * time.Second):
			t.Error("Server did not shutdown gracefully in 5 seconds")
			cmd.Process.Kill()
		case err := <-done:
			// Exit code might not be 0 due to signal
			t.Logf("Server exited: %v", err)
		}
	})
}

// TestServerHealthCheckWithExternalServer tests health check endpoint with external server
func TestServerHealthCheckWithExternalServer(t *testing.T) {
	testutil.RequireServer(t)

	// This test assumes a running server (from manual testing or CI)
	// For unit testing health check logic, see internal/handlers tests

	serverURL := os.Getenv("HELIXAGENT_URL")
	if serverURL == "" {
		serverURL = "http://localhost:7061"
	}

	// Try to connect, skip if not available
	resp, err := http.Get(serverURL + "/health")
	if err != nil {
		t.Skipf("Server not available at %s: %v", serverURL, err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"Health endpoint should return 200")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	bodyStr := string(body)
	assert.Contains(t, bodyStr, "status",
		"Health response should contain status field")
}

// TestServerRestart tests server restart capability
func TestServerRestart(t *testing.T) {
	testutil.RequireServer(t)

	if os.Getenv("CI") == "true" {
		t.Skip("Skipping restart test in CI")  // SKIP-OK: #legacy-untriaged
	}

	binPath := filepath.Join("..", "..", "bin", "helixagent")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("Binary not built")  // SKIP-OK: #legacy-untriaged
	}

	port := "17062"
	baseURL := "http://localhost:" + port

	// Helper to start server
	startServer := func() *exec.Cmd {
		cmd := exec.Command(binPath)
		cmd.Env = append(os.Environ(),
			"PORT="+port,
			"GIN_MODE=test",
			"LOG_LEVEL=error",
		)
		err := cmd.Start()
		require.NoError(t, err)

		// Wait for ready
		for i := 0; i < 50; i++ {
			resp, err := http.Get(baseURL + "/health")
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				return cmd
			}
			time.Sleep(100 * time.Millisecond)
		}

		t.Fatal("Server did not start")
		return nil
	}

	// Start first instance
	cmd1 := startServer()
	require.NotNil(t, cmd1)

	// Verify it works
	resp, err := http.Get(baseURL + "/health")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Stop it
	cmd1.Process.Signal(syscall.SIGTERM)
	cmd1.Wait()

	// Wait a moment for port to be released
	time.Sleep(500 * time.Millisecond)

	// Start second instance (restart)
	cmd2 := startServer()
	require.NotNil(t, cmd2)
	defer func() {
		cmd2.Process.Signal(syscall.SIGTERM)
		cmd2.Wait()
	}()

	// Verify it works
	resp, err = http.Get(baseURL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	t.Log("Server restarted successfully")
}

// TestServerSignalHandling tests various signal handling
func TestServerSignalHandling(t *testing.T) {
	testutil.RequireServer(t)

	if os.Getenv("CI") == "true" {
		t.Skip("Skipping signal test in CI")  // SKIP-OK: #legacy-untriaged
	}

	binPath := filepath.Join("..", "..", "bin", "helixagent")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("Binary not built")  // SKIP-OK: #legacy-untriaged
	}

	tests := []struct {
		name    string
		signal  syscall.Signal
		maxWait time.Duration
	}{
		{
			name:    "SIGTERM graceful shutdown",
			signal:  syscall.SIGTERM,
			maxWait: 5 * time.Second,
		},
		{
			name:    "SIGINT graceful shutdown",
			signal:  syscall.SIGINT,
			maxWait: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := fmt.Sprintf("1706%d", 3+len(tt.name)%10)

			cmd := exec.Command(binPath)
			cmd.Env = append(os.Environ(),
				"PORT="+port,
				"GIN_MODE=test",
				"LOG_LEVEL=error",
			)

			err := cmd.Start()
			require.NoError(t, err)

			// Wait for server to start
			baseURL := "http://localhost:" + port
			started := false
			for i := 0; i < 50; i++ {
				resp, err := http.Get(baseURL + "/health")
				if err == nil && resp.StatusCode == 200 {
					resp.Body.Close()
					started = true
					break
				}
				time.Sleep(100 * time.Millisecond)
			}

			if !started {
				cmd.Process.Kill()
				t.Fatal("Server did not start")
			}

			// Send signal
			err = cmd.Process.Signal(tt.signal)
			require.NoError(t, err, "Should send signal successfully")

			// Wait for graceful shutdown
			done := make(chan error, 1)
			go func() {
				done <- cmd.Wait()
			}()

			select {
			case <-time.After(tt.maxWait):
				cmd.Process.Kill()
				t.Errorf("Server did not shutdown within %v", tt.maxWait)
			case err := <-done:
				t.Logf("Server shut down with signal %v: %v", tt.signal, err)
			}
		})
	}
}
