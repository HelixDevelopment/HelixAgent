package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// OpenCode Integration Tests
// =============================================================================

// TestOpenCodeBinaryValidation tests the binary's validation command
func TestOpenCodeBinaryValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Find the helixagent binary
	binaryPath := findHelixAgentBinary(t)

	t.Run("ValidConfigFile", func(t *testing.T) {
		// Create a valid config file
		tmpFile := createTempOpenCodeConfig(t, map[string]interface{}{
			"$schema": "https://opencode.ai/config.json",
			"provider": map[string]interface{}{
				"test": map[string]interface{}{
					"options": map[string]interface{}{
						"apiKey":  "sk-test123",
						"baseURL": "http://localhost:7061/v1",
					},
				},
			},
		})
		defer os.Remove(tmpFile)

		cmd := exec.Command(binaryPath, "-validate-opencode-config", tmpFile)
		output, err := cmd.CombinedOutput()

		assert.NoError(t, err, "Valid config should pass validation")
		assert.Contains(t, string(output), "CONFIGURATION IS VALID")
		assert.Contains(t, string(output), "Providers: 1")
	})

	t.Run("InvalidConfigFile", func(t *testing.T) {
		// Create an invalid config file
		tmpFile := createTempOpenCodeConfig(t, map[string]interface{}{
			"invalid_key": true,
			"provider":    map[string]interface{}{},
		})
		defer os.Remove(tmpFile)

		cmd := exec.Command(binaryPath, "-validate-opencode-config", tmpFile)
		output, err := cmd.CombinedOutput()

		assert.Error(t, err, "Invalid config should fail validation")
		assert.Contains(t, string(output), "CONFIGURATION HAS ERRORS")
		assert.Contains(t, string(output), "invalid top-level keys")
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "-validate-opencode-config", "/nonexistent/path/config.json")
		output, err := cmd.CombinedOutput()

		assert.Error(t, err, "Non-existent file should fail")
		assert.Contains(t, string(output), "failed to read config file")
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		// Create a file with malformed JSON
		tmpFile, err := os.CreateTemp("", "opencode-malformed-*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString("{not valid json}")
		require.NoError(t, err)
		tmpFile.Close()

		cmd := exec.Command(binaryPath, "-validate-opencode-config", tmpFile.Name())
		output, err := cmd.CombinedOutput()

		assert.Error(t, err, "Malformed JSON should fail")
		assert.Contains(t, string(output), "invalid JSON")
	})
}

// TestOpenCodeBinaryGeneration tests the binary's generation command
func TestOpenCodeBinaryGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := findHelixAgentBinary(t)

	t.Run("GenerateToStdout", func(t *testing.T) {
		cmd := exec.Command(binaryPath, "-generate-opencode-config")
		output, err := cmd.CombinedOutput()

		assert.NoError(t, err, "Generation should succeed")

		// Parse the output as JSON
		var config map[string]interface{}
		err = json.Unmarshal(output, &config)
		assert.NoError(t, err, "Output should be valid JSON")
		assert.Contains(t, config, "$schema")
		assert.Contains(t, config, "provider")
	})

	t.Run("GenerateToFile", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "opencode-gen-*.json")
		require.NoError(t, err)
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		cmd := exec.Command(binaryPath, "-generate-opencode-config", "-opencode-output", tmpFile.Name())
		_, err = cmd.CombinedOutput()
		assert.NoError(t, err, "Generation to file should succeed")

		// Verify file contents
		data, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)

		var config map[string]interface{}
		err = json.Unmarshal(data, &config)
		assert.NoError(t, err, "Generated file should be valid JSON")
		assert.Contains(t, config, "$schema")
		assert.Contains(t, config, "provider")
	})

	t.Run("GenerateAndValidate", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "opencode-genval-*.json")
		require.NoError(t, err)
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		// Generate
		cmd := exec.Command(binaryPath, "-generate-opencode-config", "-opencode-output", tmpFile.Name())
		_, err = cmd.CombinedOutput()
		require.NoError(t, err, "Generation should succeed")

		// Validate the generated file
		cmd = exec.Command(binaryPath, "-validate-opencode-config", tmpFile.Name())
		output, err := cmd.CombinedOutput()

		assert.NoError(t, err, "Generated config should pass validation")
		assert.Contains(t, string(output), "CONFIGURATION IS VALID")
	})
}

// TestOpenCodeValidationScenarios tests various validation scenarios
func TestOpenCodeValidationScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := findHelixAgentBinary(t)

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectValid bool
		expectError string
	}{
		{
			name: "ValidMinimalConfig",
			config: map[string]interface{}{
				"$schema": "https://opencode.ai/config.json",
				"provider": map[string]interface{}{
					"test": map[string]interface{}{
						"options": map[string]interface{}{"apiKey": "test"},
					},
				},
			},
			expectValid: true,
		},
		{
			name: "ValidWithMCPLocal",
			config: map[string]interface{}{
				"$schema": "https://opencode.ai/config.json",
				"provider": map[string]interface{}{
					"test": map[string]interface{}{
						"options": map[string]interface{}{"apiKey": "test"},
					},
				},
				"mcp": map[string]interface{}{
					"filesystem": map[string]interface{}{
						"type":    "local",
						"command": []string{"npx", "-y", "@modelcontextprotocol/server-filesystem"},
					},
				},
			},
			expectValid: true,
		},
		{
			name: "ValidWithMCPRemote",
			config: map[string]interface{}{
				"$schema": "https://opencode.ai/config.json",
				"provider": map[string]interface{}{
					"test": map[string]interface{}{
						"options": map[string]interface{}{"apiKey": "test"},
					},
				},
				"mcp": map[string]interface{}{
					"remote": map[string]interface{}{
						"type": "remote",
						"url":  "http://localhost:7061/mcp",
					},
				},
			},
			expectValid: true,
		},
		{
			name: "InvalidTopLevelKey",
			config: map[string]interface{}{
				"$schema":    "https://opencode.ai/config.json",
				"invalid":    true,
				"provider": map[string]interface{}{
					"test": map[string]interface{}{
						"options": map[string]interface{}{"apiKey": "test"},
					},
				},
			},
			expectValid: false,
			expectError: "invalid top-level keys",
		},
		{
			name: "MissingProviderOptions",
			config: map[string]interface{}{
				"$schema": "https://opencode.ai/config.json",
				"provider": map[string]interface{}{
					"test": map[string]interface{}{
						"name": "Test Provider",
					},
				},
			},
			expectValid: false,
			expectError: "options",
		},
		{
			name: "MCPMissingType",
			config: map[string]interface{}{
				"$schema": "https://opencode.ai/config.json",
				"provider": map[string]interface{}{
					"test": map[string]interface{}{
						"options": map[string]interface{}{"apiKey": "test"},
					},
				},
				"mcp": map[string]interface{}{
					"bad": map[string]interface{}{
						"command": []string{"test"},
					},
				},
			},
			expectValid: false,
			expectError: "type is required",
		},
		{
			name: "MCPInvalidType",
			config: map[string]interface{}{
				"$schema": "https://opencode.ai/config.json",
				"provider": map[string]interface{}{
					"test": map[string]interface{}{
						"options": map[string]interface{}{"apiKey": "test"},
					},
				},
				"mcp": map[string]interface{}{
					"bad": map[string]interface{}{
						"type": "invalid",
					},
				},
			},
			expectValid: false,
			expectError: "'local' or 'remote'",
		},
		{
			name: "MCPLocalMissingCommand",
			config: map[string]interface{}{
				"$schema": "https://opencode.ai/config.json",
				"provider": map[string]interface{}{
					"test": map[string]interface{}{
						"options": map[string]interface{}{"apiKey": "test"},
					},
				},
				"mcp": map[string]interface{}{
					"bad": map[string]interface{}{
						"type": "local",
					},
				},
			},
			expectValid: false,
			expectError: "command is required",
		},
		{
			name: "MCPRemoteMissingURL",
			config: map[string]interface{}{
				"$schema": "https://opencode.ai/config.json",
				"provider": map[string]interface{}{
					"test": map[string]interface{}{
						"options": map[string]interface{}{"apiKey": "test"},
					},
				},
				"mcp": map[string]interface{}{
					"bad": map[string]interface{}{
						"type": "remote",
					},
				},
			},
			expectValid: false,
			expectError: "url is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpFile := createTempOpenCodeConfig(t, tc.config)
			defer os.Remove(tmpFile)

			cmd := exec.Command(binaryPath, "-validate-opencode-config", tmpFile)
			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			if tc.expectValid {
				assert.NoError(t, err, "Expected valid config")
				assert.Contains(t, outputStr, "CONFIGURATION IS VALID")
			} else {
				assert.Error(t, err, "Expected invalid config")
				assert.Contains(t, outputStr, "CONFIGURATION HAS ERRORS")
				if tc.expectError != "" {
					assert.Contains(t, outputStr, tc.expectError)
				}
			}
		})
	}
}

// TestOpenCodeRealConfigs tests with real config files if they exist
func TestOpenCodeRealConfigs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := findHelixAgentBinary(t)
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	t.Run("DownloadsConfig", func(t *testing.T) {
		configPath := filepath.Join(homeDir, "Downloads", "opencode-helix-agent.json")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Skip("Downloads config file does not exist")
		}

		cmd := exec.Command(binaryPath, "-validate-opencode-config", configPath)
		output, err := cmd.CombinedOutput()

		assert.NoError(t, err, "Downloads config should be valid")
		assert.Contains(t, string(output), "CONFIGURATION IS VALID")
	})

	t.Run("UserOpenCodeConfig", func(t *testing.T) {
		configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Skip("User opencode config file does not exist")
		}

		cmd := exec.Command(binaryPath, "-validate-opencode-config", configPath)
		output, err := cmd.CombinedOutput()

		assert.NoError(t, err, "User config should be valid")
		assert.Contains(t, string(output), "CONFIGURATION IS VALID")
	})
}

// TestOpenCodeHelpOutput tests that help mentions the validation flag
func TestOpenCodeHelpOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := findHelixAgentBinary(t)

	cmd := exec.Command(binaryPath, "-help")
	output, err := cmd.CombinedOutput()

	assert.NoError(t, err)
	outputStr := string(output)

	assert.Contains(t, outputStr, "-validate-opencode-config")
	assert.Contains(t, outputStr, "-generate-opencode-config")
	assert.Contains(t, outputStr, "-opencode-output")
}

// =============================================================================
// Helper Functions
// =============================================================================

// findHelixAgentBinary finds the helixagent binary
func findHelixAgentBinary(t *testing.T) string {
	// Try common locations
	locations := []string{
		"../../helixagent",
		"../../bin/helixagent",
		"./helixagent",
		"./bin/helixagent",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			absPath, _ := filepath.Abs(loc)
			return absPath
		}
	}

	// Try to build it
	t.Log("HelixAgent binary not found, attempting to build...")
	cmd := exec.Command("go", "build", "-o", "../../helixagent", "../../cmd/helixagent/")
	if err := cmd.Run(); err != nil {
		t.Skipf("Could not find or build helixagent binary: %v", err)
	}

	absPath, _ := filepath.Abs("../../helixagent")
	return absPath
}

// createTempOpenCodeConfig creates a temporary OpenCode config file
func createTempOpenCodeConfig(t *testing.T, config map[string]interface{}) string {
	tmpFile, err := os.CreateTemp("", "opencode-test-*.json")
	require.NoError(t, err)

	data, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)

	_, err = tmpFile.Write(data)
	require.NoError(t, err)
	tmpFile.Close()

	return tmpFile.Name()
}

// =============================================================================
// OpenCode API Tests (requires running server)
// =============================================================================

// TestOpenCodeWithRunningServer tests OpenCode validation with a running HelixAgent
func TestOpenCodeWithRunningServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if server is running
	if !isServerRunning("http://localhost:7061/health") {
		t.Skip("HelixAgent server not running on localhost:7061")
	}

	binaryPath := findHelixAgentBinary(t)

	t.Run("GeneratedConfigConnectsToServer", func(t *testing.T) {
		// Generate config
		tmpFile, err := os.CreateTemp("", "opencode-server-*.json")
		require.NoError(t, err)
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		cmd := exec.Command(binaryPath, "-generate-opencode-config", "-opencode-output", tmpFile.Name())
		_, err = cmd.CombinedOutput()
		require.NoError(t, err)

		// Validate it
		cmd = exec.Command(binaryPath, "-validate-opencode-config", tmpFile.Name())
		output, err := cmd.CombinedOutput()

		assert.NoError(t, err)
		assert.Contains(t, string(output), "CONFIGURATION IS VALID")

		// Verify the config has the correct baseURL
		data, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)

		var config map[string]interface{}
		err = json.Unmarshal(data, &config)
		require.NoError(t, err)

		// Check provider options
		providers := config["provider"].(map[string]interface{})
		for _, prov := range providers {
			provMap := prov.(map[string]interface{})
			options := provMap["options"].(map[string]interface{})
			baseURL := options["baseURL"].(string)
			assert.True(t, strings.Contains(baseURL, "localhost:7061") || strings.Contains(baseURL, "127.0.0.1:7061"),
				"Generated config should point to running server")
		}
	})
}

// isServerRunning checks if the server is running at the given URL
func isServerRunning(url string) bool {
	cmd := exec.Command("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", url)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "200"
}
