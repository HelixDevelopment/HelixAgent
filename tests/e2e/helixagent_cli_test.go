package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelixAgentCLI tests the main helixagent CLI commands
func TestHelixAgentCLI(t *testing.T) {
	// Build binary if not exists
	binPath := filepath.Join("..", "..", "bin", "helixagent")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("Binary not built, skipping E2E tests. Run 'make build' first.") // SKIP-OK: #legacy-untriaged
	}

	tests := []struct {
		name           string
		args           []string
		expectedExit   int
		expectedOutput string
		expectedError  string
	}{
		{
			name:           "version command",
			args:           []string{"--version"},
			expectedExit:   0,
			expectedOutput: "HelixAgent v",
		},
		{
			name:           "help command",
			args:           []string{"--help"},
			expectedExit:   0,
			expectedOutput: "Usage:",
		},
		{
			name:           "list agents command",
			args:           []string{"--list-agents"},
			expectedExit:   0,
			expectedOutput: "opencode",
		},
		{
			name:          "invalid command",
			args:          []string{"--invalid-flag"},
			expectedExit:  1,
			expectedError: "flag provided but not defined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binPath, tt.args...)
			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			if tt.expectedExit == 0 {
				assert.NoError(t, err, "Expected command to succeed")
			} else {
				assert.Error(t, err, "Expected command to fail")
			}

			if tt.expectedOutput != "" {
				assert.Contains(t, outputStr, tt.expectedOutput,
					"Expected output to contain: %s", tt.expectedOutput)
			}

			if tt.expectedError != "" {
				assert.Contains(t, strings.ToLower(outputStr), strings.ToLower(tt.expectedError),
					"Expected error to contain: %s", tt.expectedError)
			}
		})
	}
}

// TestHelixAgentConfigGeneration tests config generation commands
func TestHelixAgentConfigGeneration(t *testing.T) {
	binPath := filepath.Join("..", "..", "bin", "helixagent")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("Binary not built, skipping E2E tests") // SKIP-OK: #legacy-untriaged
	}

	// Create temp directory for output
	tmpDir := t.TempDir()

	agents := []string{"opencode", "crush"}

	for _, agent := range agents {
		t.Run("generate_"+agent+"_config", func(t *testing.T) {
			outputFile := filepath.Join(tmpDir, agent+".json")
			cmd := exec.Command(binPath,
				"-generate-agent-config="+agent,
				"-agent-config-output="+outputFile,
			)

			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "Config generation should succeed: %s", string(output))

			// Verify config file was created
			_, err = os.Stat(outputFile)
			assert.NoError(t, err, "Config file should exist: %s", outputFile)

			// Verify it's valid JSON
			data, err := os.ReadFile(outputFile)
			require.NoError(t, err)
			assert.True(t, len(data) > 0, "Config file should not be empty")
			assert.True(t, strings.HasPrefix(string(data), "{"), "Config should be JSON")
		})
	}
}
