// Package integration provides integration tests for HelixAgent
package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// CLIAgent represents a CLI agent that we support
type CLIAgent struct {
	Name           string
	ConfigPath     string
	SchemaURL      string
	BinaryName     string
	ValidateCmd    []string
	ProjectPath    string
}

// MCPServerSchemaConfig represents an MCP server configuration per OpenCode schema
type MCPServerSchemaConfig struct {
	Type        string            `json:"type"`
	URL         string            `json:"url,omitempty"`
	Command     []string          `json:"command,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Timeout     *int              `json:"timeout,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	OAuth       interface{}       `json:"oauth,omitempty"`
}

// OpenCodeSchemaConfig represents a minimal OpenCode configuration for validation
type OpenCodeSchemaConfig struct {
	Schema   string                           `json:"$schema,omitempty"`
	Provider map[string]interface{}           `json:"provider,omitempty"`
	MCP      map[string]MCPServerSchemaConfig `json:"mcp,omitempty"`
	Agent    map[string]interface{}           `json:"agent,omitempty"`
}

// InvalidMCPFields lists fields that should NOT be in MCP server configs
var InvalidMCPFields = []string{
	"transport", // Not in OpenCode schema
	"env",       // Should be "environment" not "env"
}

// ValidMCPLocalFields lists valid fields for local MCP servers
var ValidMCPLocalFields = []string{
	"type", "command", "environment", "enabled", "timeout",
}

// ValidMCPRemoteFields lists valid fields for remote MCP servers
var ValidMCPRemoteFields = []string{
	"type", "url", "headers", "oauth", "enabled", "timeout",
}

// GetSupportedCLIAgents returns the list of CLI agents we support
func GetSupportedCLIAgents() []CLIAgent {
	homeDir := os.Getenv("HOME")
	projectsDir := "/run/media/milosvasic/DATA4TB/Projects"
	exampleProjectsDir := filepath.Join(projectsDir, "HelixCode", "Example_Projects")

	return []CLIAgent{
		{
			Name:        "OpenCode",
			ConfigPath:  filepath.Join(homeDir, ".config", "opencode", "opencode.json"),
			SchemaURL:   "https://opencode.ai/config.json",
			BinaryName:  "opencode",
			ValidateCmd: []string{"opencode", "--version"},
			ProjectPath: filepath.Join(exampleProjectsDir, "OpenCode"),
		},
		{
			Name:        "Claude Code",
			ConfigPath:  filepath.Join(homeDir, ".claude", "claude_desktop_config.json"),
			SchemaURL:   "",
			BinaryName:  "claude",
			ValidateCmd: []string{"claude", "--version"},
			ProjectPath: filepath.Join(exampleProjectsDir, "Claude_Code"),
		},
		{
			Name:        "Kilo Code",
			ConfigPath:  filepath.Join(homeDir, ".config", "kilo-code", "config.json"),
			SchemaURL:   "",
			BinaryName:  "kilo-code",
			ValidateCmd: []string{"kilo-code", "--version"},
			ProjectPath: filepath.Join(exampleProjectsDir, "Kilo-Code"),
		},
		{
			Name:        "Qwen Code",
			ConfigPath:  filepath.Join(homeDir, ".qwen", "config.json"),
			SchemaURL:   "",
			BinaryName:  "qwen-code",
			ValidateCmd: []string{"qwen-code", "--version"},
			ProjectPath: filepath.Join(exampleProjectsDir, "Qwen_Code"),
		},
		{
			Name:        "Gemini CLI",
			ConfigPath:  filepath.Join(homeDir, ".config", "gemini", "config.json"),
			SchemaURL:   "",
			BinaryName:  "gemini",
			ValidateCmd: []string{"gemini", "--version"},
			ProjectPath: filepath.Join(exampleProjectsDir, "Gemini_CLI"),
		},
		{
			Name:        "DeepSeek CLI",
			ConfigPath:  filepath.Join(homeDir, ".deepseek", "config.json"),
			SchemaURL:   "",
			BinaryName:  "deepseek",
			ValidateCmd: []string{"deepseek", "--version"},
			ProjectPath: filepath.Join(exampleProjectsDir, "DeepSeek_CLI"),
		},
		{
			Name:        "Aider",
			ConfigPath:  filepath.Join(homeDir, ".aider.conf.yml"),
			SchemaURL:   "",
			BinaryName:  "aider",
			ValidateCmd: []string{"aider", "--version"},
			ProjectPath: filepath.Join(exampleProjectsDir, "Aider"),
		},
		{
			Name:        "Cline",
			ConfigPath:  filepath.Join(homeDir, ".config", "cline", "config.json"),
			SchemaURL:   "",
			BinaryName:  "cline",
			ValidateCmd: []string{"cline", "--version"},
			ProjectPath: filepath.Join(exampleProjectsDir, "Cline"),
		},
	}
}

// TestOpenCodeSchemaValidation validates the OpenCode configuration against the schema
func TestOpenCodeSchemaValidation(t *testing.T) {
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "opencode", "opencode.json")

	// Read the configuration
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read OpenCode config: %v", err)
	}

	// Parse as generic JSON to check for invalid fields
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		t.Fatalf("Failed to parse OpenCode config as JSON: %v", err)
	}

	// Check MCP servers for invalid fields
	if mcpRaw, ok := rawConfig["mcp"]; ok {
		mcpMap, ok := mcpRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("MCP section is not a map")
		}

		for serverName, serverRaw := range mcpMap {
			serverMap, ok := serverRaw.(map[string]interface{})
			if !ok {
				t.Errorf("MCP server %s is not a map", serverName)
				continue
			}

			// Check for invalid fields
			for _, invalidField := range InvalidMCPFields {
				if _, exists := serverMap[invalidField]; exists {
					t.Errorf("MCP server %s contains invalid field '%s' - this field is NOT in the OpenCode schema", serverName, invalidField)
				}
			}

			// Validate field types based on server type
			serverType, _ := serverMap["type"].(string)
			switch serverType {
			case "local":
				// Must have command, must NOT have url
				if _, hasURL := serverMap["url"]; hasURL {
					t.Errorf("Local MCP server %s should not have 'url' field", serverName)
				}
				if _, hasCommand := serverMap["command"]; !hasCommand {
					t.Errorf("Local MCP server %s must have 'command' field", serverName)
				}
			case "remote":
				// Must have url, must NOT have command
				if _, hasCommand := serverMap["command"]; hasCommand {
					t.Errorf("Remote MCP server %s should not have 'command' field", serverName)
				}
				if _, hasURL := serverMap["url"]; !hasURL {
					t.Errorf("Remote MCP server %s must have 'url' field", serverName)
				}
			default:
				t.Errorf("MCP server %s has invalid type '%s' - must be 'local' or 'remote'", serverName, serverType)
			}
		}
	}

	// Parse into struct for additional validation
	var config OpenCodeSchemaConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse OpenCode config into struct: %v", err)
	}

	// Validate provider section
	if len(config.Provider) == 0 {
		t.Error("Provider section is empty - at least one provider must be defined")
	}

	// Log success info
	t.Logf("OpenCode config validation passed:")
	t.Logf("  - Providers: %d", len(config.Provider))
	t.Logf("  - MCP servers: %d", len(config.MCP))
	t.Logf("  - Agents: %d", len(config.Agent))
}

// TestOpenCodeSchemaValidationWithBinary actually runs OpenCode to validate the config
func TestOpenCodeSchemaValidationWithBinary(t *testing.T) {
	// Check if OpenCode binary is available
	_, err := exec.LookPath("opencode")
	if err != nil {
		t.Skip("OpenCode binary not available, skipping binary validation")
	}

	// Run opencode --version to verify it can start (this validates the config)
	cmd := exec.Command("opencode", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if error is config-related
		outputStr := string(output)
		if strings.Contains(outputStr, "Configuration is invalid") ||
			strings.Contains(outputStr, "Invalid input") {
			t.Fatalf("OpenCode config validation failed:\n%s", outputStr)
		}
		t.Logf("OpenCode command failed (may be expected): %v\nOutput: %s", err, output)
	} else {
		t.Logf("OpenCode binary validation passed: %s", strings.TrimSpace(string(output)))
	}
}

// TestAllCLIAgentsSchemaValidation validates configurations for all supported CLI agents
func TestAllCLIAgentsSchemaValidation(t *testing.T) {
	agents := GetSupportedCLIAgents()

	for _, agent := range agents {
		t.Run(agent.Name, func(t *testing.T) {
			// Check if config exists
			if _, err := os.Stat(agent.ConfigPath); os.IsNotExist(err) {
				t.Skipf("Config file not found: %s", agent.ConfigPath)
				return
			}

			// Read the config
			data, err := os.ReadFile(agent.ConfigPath)
			if err != nil {
				t.Fatalf("Failed to read config: %v", err)
			}

			// Validate JSON syntax
			var rawConfig map[string]interface{}
			if err := json.Unmarshal(data, &rawConfig); err != nil {
				t.Fatalf("Invalid JSON in config: %v", err)
			}

			// Check for common invalid fields in MCP sections
			if mcpRaw, ok := rawConfig["mcp"]; ok {
				mcpMap, ok := mcpRaw.(map[string]interface{})
				if ok {
					for serverName, serverRaw := range mcpMap {
						serverMap, ok := serverRaw.(map[string]interface{})
						if !ok {
							continue
						}

						for _, invalidField := range InvalidMCPFields {
							if _, exists := serverMap[invalidField]; exists {
								t.Errorf("[%s] MCP server %s contains invalid field '%s'", agent.Name, serverName, invalidField)
							}
						}
					}
				}
			}

			t.Logf("[%s] Config validation passed", agent.Name)
		})
	}
}

// TestGeneratedConfigHasNoInvalidFields ensures the generator doesn't produce invalid configs
func TestGeneratedConfigHasNoInvalidFields(t *testing.T) {
	// Generate a fresh config
	cmd := exec.Command("./bin/helixagent", "-generate-opencode-config", "-opencode-output", "/tmp/test_opencode_config.json")
	cmd.Dir = "/run/media/milosvasic/DATA4TB/Projects/HelixAgent"
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to generate config: %v\nOutput: %s", err, output)
	}

	// Read and validate the generated config
	data, err := os.ReadFile("/tmp/test_opencode_config.json")
	if err != nil {
		t.Fatalf("Failed to read generated config: %v", err)
	}

	// Check for invalid fields
	configStr := string(data)
	for _, invalidField := range InvalidMCPFields {
		searchStr := "\"" + invalidField + "\":"
		if strings.Contains(configStr, searchStr) {
			t.Errorf("Generated config contains invalid field '%s' - this will cause OpenCode validation errors", invalidField)
		}
	}

	// Parse and validate structure
	var config OpenCodeSchemaConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Generated config is not valid JSON: %v", err)
	}

	// Ensure required sections exist
	if len(config.Provider) == 0 {
		t.Error("Generated config has no providers")
	}
	if len(config.MCP) < 6 {
		t.Errorf("Generated config should have at least 6 MCP servers, got %d", len(config.MCP))
	}
	if len(config.Agent) < 5 {
		t.Errorf("Generated config should have at least 5 agents, got %d", len(config.Agent))
	}

	t.Logf("Generated config validation passed: %d providers, %d MCP servers, %d agents",
		len(config.Provider), len(config.MCP), len(config.Agent))

	// Cleanup
	os.Remove("/tmp/test_opencode_config.json")
}

// TestMCPServerFieldValidation tests that all MCP servers have only valid fields
func TestMCPServerFieldValidation(t *testing.T) {
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "opencode", "opencode.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var rawConfig map[string]interface{}
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	mcpRaw, ok := rawConfig["mcp"]
	if !ok {
		t.Fatal("No MCP section in config")
	}

	mcpMap, ok := mcpRaw.(map[string]interface{})
	if !ok {
		t.Fatal("MCP section is not a map")
	}

	for serverName, serverRaw := range mcpMap {
		serverMap, ok := serverRaw.(map[string]interface{})
		if !ok {
			t.Errorf("MCP server %s is not a map", serverName)
			continue
		}

		serverType, _ := serverMap["type"].(string)
		var validFields []string
		if serverType == "local" {
			validFields = ValidMCPLocalFields
		} else if serverType == "remote" {
			validFields = ValidMCPRemoteFields
		} else {
			t.Errorf("MCP server %s has invalid type: %s", serverName, serverType)
			continue
		}

		// Check each field in the server config
		for field := range serverMap {
			isValid := false
			for _, validField := range validFields {
				if field == validField {
					isValid = true
					break
				}
			}
			if !isValid {
				t.Errorf("MCP server %s (%s) has invalid field '%s'. Valid fields for %s servers: %v",
					serverName, serverType, field, serverType, validFields)
			}
		}
	}
}

// TestMCPServerConnectivity tests that all remote MCP servers respond within timeout
// This is CRITICAL for rock-solid stability - servers MUST respond fast
func TestMCPServerConnectivity(t *testing.T) {
	// Check if HelixAgent is running
	resp, err := http.Get("http://localhost:7061/health")
	if err != nil {
		t.Skip("HelixAgent not running - cannot test MCP connectivity")
	}
	resp.Body.Close()

	configPath := filepath.Join(os.Getenv("HOME"), ".config", "opencode", "opencode.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Skipf("OpenCode config not found: %v", err)
	}

	var config OpenCodeSchemaConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	client := &http.Client{
		Timeout: 5 * time.Second, // MUST respond within 5 seconds
	}

	failures := 0
	successes := 0

	for serverName, serverConfig := range config.MCP {
		if serverConfig.Type != "remote" {
			continue
		}

		if serverConfig.URL == "" {
			t.Errorf("Remote MCP server %s has no URL", serverName)
			failures++
			continue
		}

		start := time.Now()
		req, err := http.NewRequest("POST", serverConfig.URL, strings.NewReader(`{"jsonrpc":"2.0","method":"ping","id":1}`))
		if err != nil {
			t.Errorf("Failed to create request for %s: %v", serverName, err)
			failures++
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("MCP server %s TIMEOUT after %v - UNACCEPTABLE! Error: %v", serverName, elapsed, err)
			failures++
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			t.Logf("MCP server %s: OK (%v, HTTP %d)", serverName, elapsed, resp.StatusCode)
			successes++
		} else {
			t.Errorf("MCP server %s: FAILED (HTTP %d, %v)", serverName, resp.StatusCode, elapsed)
			failures++
		}
	}

	if failures > 0 {
		t.Fatalf("MCP server connectivity: %d failures, %d success - MUST BE ROCK SOLID!", failures, successes)
	}

	t.Logf("All %d MCP servers responded within 5s timeout", successes)
}

// TestNoLocalNpxServers ensures no local npx servers are in config (they timeout)
// This test only enforces in CI environments; in local development, npx servers
// may be intentionally enabled for testing purposes.
func TestNoLocalNpxServers(t *testing.T) {
	// Skip in non-CI environments since local npx servers may be intentional
	if os.Getenv("CI") == "" && os.Getenv("GITHUB_ACTIONS") == "" {
		t.Skip("Skipping npx server check in non-CI environment (local dev may use npx servers)")
	}

	configPath := filepath.Join(os.Getenv("HOME"), ".config", "opencode", "opencode.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Skipf("OpenCode config not found: %v", err)
	}

	var rawConfig map[string]interface{}
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	mcpRaw, ok := rawConfig["mcp"]
	if !ok {
		return // No MCP section
	}

	mcpMap, ok := mcpRaw.(map[string]interface{})
	if !ok {
		return
	}

	var npxServers []string
	for serverName, serverRaw := range mcpMap {
		serverMap, ok := serverRaw.(map[string]interface{})
		if !ok {
			continue
		}

		if serverMap["type"] != "local" {
			continue
		}

		cmd, ok := serverMap["command"].([]interface{})
		if !ok || len(cmd) == 0 {
			continue
		}

		// Check if command uses npx
		for _, c := range cmd {
			if str, ok := c.(string); ok && str == "npx" {
				npxServers = append(npxServers, serverName)
				break
			}
		}
	}

	if len(npxServers) > 0 {
		t.Fatalf("Found local npx servers that will timeout: %v - These MUST NOT be in config!", npxServers)
	}

	t.Log("No local npx servers found (prevents timeout issues)")
}
