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
	Name        string
	ConfigPath  string
	SchemaURL   string
	BinaryName  string
	ValidateCmd []string
	ProjectPath string
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
	Schema string `json:"$schema,omitempty"`
	// Legacy schema (singular)
	Provider map[string]interface{}           `json:"provider,omitempty"`
	MCP      map[string]MCPServerSchemaConfig `json:"mcp,omitempty"`
	Agent    map[string]interface{}           `json:"agent,omitempty"`
	// New schema v1.1.30+ (plural)
	Providers  map[string]interface{}           `json:"providers,omitempty"`
	MCPServers map[string]MCPServerSchemaConfig `json:"mcpServers,omitempty"`
	Agents     map[string]interface{}           `json:"agents,omitempty"`
}

// GetProviders returns the provider map, checking both old and new schema keys
func (c *OpenCodeSchemaConfig) GetProviders() map[string]interface{} {
	if len(c.Providers) > 0 {
		return c.Providers
	}
	return c.Provider
}

// GetMCPs returns the MCP servers map, checking both old and new schema keys
func (c *OpenCodeSchemaConfig) GetMCPs() map[string]MCPServerSchemaConfig {
	if len(c.MCPServers) > 0 {
		return c.MCPServers
	}
	return c.MCP
}

// GetAgents returns the agents map, checking both old and new schema keys
func (c *OpenCodeSchemaConfig) GetAgents() map[string]interface{} {
	if len(c.Agents) > 0 {
		return c.Agents
	}
	return c.Agent
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

// openCodeReferenceConfigRelPath is the OpenCode-schema config artifact THIS
// REPOSITORY owns and ships: the reference config that cmd/helixagent's
// handleGenerateOpenCode names as the canonical form its output must match.
// It is tracked, so it is present in every checkout and identical in all of them.
const openCodeReferenceConfigRelPath = "configs/cli-agents/opencode.json"

// historicalOperatorHomeConfigPath rebuilds the path these tests used to treat as
// their subject: the OPERATOR'S personal OpenCode config, in their real home
// directory. It is the generator's DESTINATION (see handleGenerateOpenCode:
// "saved as opencode.json ... in ~/.config/opencode/"), never a project artifact —
// so a PASS asserted that this machine's config happened to be schema-valid, a
// fact about the developer's laptop rather than about this codebase.
//
// It is retained ONLY to build a string for the RED_MODE=1 polarity switch
// (§11.4.115), which asserts the subject is outside the repository. Neither
// polarity ever opens this path: the guard rejects it before any read.
func historicalOperatorHomeConfigPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "opencode", "opencode.json")
}

// openCodeSubjectPath resolves the config artifact under test and PROVES it
// belongs to this repository.
//
// RED_MODE=1 restores the historical operator-home path so the guard can show it
// still rejects a subject outside the checkout; the default resolves the tracked
// reference config. The guard — not the caller — is what makes a drift back to
// $HOME impossible to land silently.
func openCodeSubjectPath(t *testing.T) string {
	t.Helper()

	repoRoot := findProjectRootForOpenCode(t)

	path := filepath.Join(repoRoot, openCodeReferenceConfigRelPath)
	if os.Getenv("RED_MODE") == "1" {
		path = historicalOperatorHomeConfigPath()
	}

	// A subject outside the repository is a fact about the machine, not about
	// this codebase. Refuse it before reading a single byte.
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("config under test must live inside the repository %s, got %s "+
			"(a config outside the checkout describes the operator's machine, not this project)",
			repoRoot, path)
	}
	return path
}

// readOpenCodeSubject returns the bytes of the repository's OpenCode config artifact.
// Absence is a hard failure, not a skip: the file is tracked, so it is missing only
// if this repository is broken — which is exactly what a test here should report.
func readOpenCodeSubject(t *testing.T) []byte {
	t.Helper()

	path := openCodeSubjectPath(t)
	data, err := os.ReadFile(path) // #nosec G304 -- path is repo-rooted and guarded above
	if err != nil {
		t.Fatalf("Failed to read the repository's OpenCode config %s: %v", path, err)
	}
	t.Logf("subject under test: %s (%d bytes)", path, len(data))
	return data
}

// TestOpenCodeSchemaValidation validates the OpenCode config artifact THIS
// REPOSITORY ships against the OpenCode schema.
//
// It used to read $HOME/.config/opencode/opencode.json — the operator's personal
// config — so a PASS meant "this machine's OpenCode install is schema-valid" and a
// FAIL meant "the operator has not installed OpenCode". Neither outcome said
// anything about the code under test. The subject is now the tracked reference
// config, so the same assertions describe this project on every checkout.
func TestOpenCodeSchemaValidation(t *testing.T) {
	data := readOpenCodeSubject(t)

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
	if len(config.GetProviders()) == 0 {
		t.Error("Provider section is empty - at least one provider must be defined")
	}

	// Log success info
	t.Logf("OpenCode config validation passed:")
	t.Logf("  - Providers: %d", len(config.GetProviders()))
	t.Logf("  - MCP servers: %d", len(config.GetMCPs()))
	t.Logf("  - Agents: %d", len(config.GetAgents()))
}

// TestOpenCodeSchemaValidationWithBinary actually runs OpenCode to validate the config
func TestOpenCodeSchemaValidationWithBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping opencode binary validation in short mode") // SKIP-OK: #short-mode
	}
	// Check if OpenCode binary is available
	_, err := exec.LookPath("opencode")
	if err != nil {
		t.Logf("OpenCode binary not available - skipping binary validation (acceptable)")
		return
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
				t.Skipf("Config file not found: %s (SKIP-OK: #unmarked-skip-needs-ticket)", agent.ConfigPath)
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

// historicalForeignCheckoutRoot is the absolute path this test used to hardcode as the
// generator's working directory. It belongs to one developer's machine and exists on no
// other checkout, so `exec` failed at `chdir` before the binary ever ran — the test could
// not pass anywhere else and never exercised the generator at all.
//
// It is retained ONLY as the RED_MODE=1 reproduction input (§11.4.115 polarity switch).
const historicalForeignCheckoutRoot = "/run/media/milosvasic/DATA4TB/Projects/HelixAgent"

// generatorWorkDir returns the working directory the helixagent binary is invoked from.
//
// RED_MODE=1 restores the historical hardcoded foreign path so the guard can prove it
// still detects the defect; the default (RED_MODE unset or 0) derives the repository root
// from the running test's location, so the test works on every checkout.
func generatorWorkDir(t *testing.T) string {
	t.Helper()
	if os.Getenv("RED_MODE") == "1" {
		return historicalForeignCheckoutRoot
	}
	return findProjectRootForOpenCode(t)
}

// buildGeneratorBinary compiles cmd/helixagent from this checkout into the test's
// own temp directory and returns the path. Nothing is written inside the
// repository, and t.TempDir() is removed when the test finishes.
func buildGeneratorBinary(t *testing.T) string {
	t.Helper()

	repoRoot := findProjectRootForOpenCode(t)
	binaryPath := filepath.Join(t.TempDir(), "helixagent")

	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/helixagent")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build the generator from %s: %v\n%s", repoRoot, err, out)
	}
	return binaryPath
}

// TestGeneratedConfigHasNoInvalidFields ensures the generator doesn't produce invalid configs
func TestGeneratedConfigHasNoInvalidFields(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping binary-dependent test in short mode") // SKIP-OK: #short-mode
	}

	// Per-test output path: a shared /tmp filename races concurrent runs on a shared host.
	outputPath := filepath.Join(t.TempDir(), "test_opencode_config.json")

	// Build the generator from THIS checkout's source rather than invoking a
	// prebuilt ./bin/helixagent. The prebuilt path was a second dependency on
	// something outside the repository's control: `make build` had to have been
	// run first, so the test failed with "fork/exec ./bin/helixagent: no such
	// file or directory" on any clean checkout. Building from source means the
	// test carries its own subject and exercises the code as committed, not
	// whatever binary happened to be lying in bin/.
	binaryPath := buildGeneratorBinary(t)

	// Generate a fresh config
	cmd := exec.Command(binaryPath, "-generate-opencode-config", "-opencode-output", outputPath)
	cmd.Dir = generatorWorkDir(t)
	output, err := cmd.CombinedOutput()

	if os.Getenv("RED_MODE") == "1" {
		// RED baseline: reproduce the historical defect on the pre-fix working directory.
		// The generator must fail to even start because the hardcoded path does not exist.
		if err == nil {
			t.Fatalf("RED_MODE: expected generation to fail from %q, but it succeeded", historicalForeignCheckoutRoot)
		}
		if !strings.Contains(err.Error(), "no such file or directory") {
			t.Fatalf("RED_MODE: expected a chdir/no-such-file failure from %q, got: %v\nOutput: %s",
				historicalForeignCheckoutRoot, err, output)
		}
		t.Logf("RED_MODE: reproduced the historical defect — %v", err)
		return
	}

	if err != nil {
		t.Fatalf("Failed to generate config: %v\nOutput: %s", err, output)
	}

	// Read and validate the generated config
	data, err := os.ReadFile(outputPath)
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

	// The generated artifact earns the same field-whitelist scrutiny as the
	// shipped reference config: a substring search for two known-bad names
	// cannot catch a field nobody has thought to blacklist yet.
	assertMCPFieldWhitelist(t, "generated config", data)

	// Parse and validate structure
	var config OpenCodeSchemaConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Generated config is not valid JSON: %v", err)
	}

	// Ensure required sections exist
	if len(config.GetProviders()) == 0 {
		t.Error("Generated config has no providers")
	}
	if len(config.GetMCPs()) < 6 {
		t.Errorf("Generated config should have at least 6 MCP servers, got %d", len(config.GetMCPs()))
	}
	// Current config has 4 agents: coder, task, title, summarizer
	if len(config.GetAgents()) < 4 {
		t.Errorf("Generated config should have at least 4 agents, got %d", len(config.GetAgents()))
	}

	t.Logf("Generated config validation passed: %d providers, %d MCP servers, %d agents",
		len(config.GetProviders()), len(config.GetMCPs()), len(config.GetAgents()))

	// t.TempDir() is removed automatically when the test finishes.
}

// assertMCPFieldWhitelist checks every MCP server entry in an OpenCode-schema
// config against the whitelist for its type. Extracted so the SAME assertion runs
// against both artifacts this repository owns: the shipped reference config and
// the generator's freshly produced output.
func assertMCPFieldWhitelist(t *testing.T, subject string, data []byte) {
	t.Helper()

	var rawConfig map[string]interface{}
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		t.Fatalf("[%s] Failed to parse config: %v", subject, err)
	}

	mcpRaw, ok := rawConfig["mcp"]
	if !ok {
		t.Fatalf("[%s] No MCP section in config", subject)
	}

	mcpMap, ok := mcpRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("[%s] MCP section is not a map", subject)
	}

	for serverName, serverRaw := range mcpMap {
		serverMap, ok := serverRaw.(map[string]interface{})
		if !ok {
			t.Errorf("[%s] MCP server %s is not a map", subject, serverName)
			continue
		}

		serverType, _ := serverMap["type"].(string)
		var validFields []string
		if serverType == "local" {
			validFields = ValidMCPLocalFields
		} else if serverType == "remote" {
			validFields = ValidMCPRemoteFields
		} else {
			t.Errorf("[%s] MCP server %s has invalid type: %s", subject, serverName, serverType)
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
				t.Errorf("[%s] MCP server %s (%s) has invalid field '%s'. Valid fields for %s servers: %v",
					subject, serverName, serverType, field, serverType, validFields)
			}
		}
	}

	t.Logf("[%s] field whitelist passed for %d MCP servers", subject, len(mcpMap))
}

// TestMCPServerFieldValidation tests that every MCP server in the OpenCode config
// artifact THIS REPOSITORY ships carries only fields the OpenCode schema allows.
//
// Same subject correction as TestOpenCodeSchemaValidation: this read the
// operator's personal ~/.config/opencode/opencode.json, so it graded the
// developer's machine rather than this codebase.
func TestMCPServerFieldValidation(t *testing.T) {
	assertMCPFieldWhitelist(t, "reference config", readOpenCodeSubject(t))
}

// TestMCPRemoteServerConnectivity tests that all remote MCP servers respond within timeout
// This is CRITICAL for rock-solid stability - servers MUST respond fast
func TestMCPRemoteServerConnectivity(t *testing.T) {
	// Check if HelixAgent is running
	resp, err := http.Get("http://localhost:8100/health")
	if err != nil {
		t.Logf("HelixAgent not running - cannot test MCP connectivity (acceptable)")
		return
	}
	resp.Body.Close()

	// SAME SUBJECT DEFECT as TestOpenCodeSchemaValidation once had: this reads the
	// operator's personal config rather than an artifact this repository owns.
	// NOT repointed to openCodeSubjectPath() here, because that is not a
	// plumbing change: the reference config's 12 remote entries include three
	// THIRD-PARTY endpoints (cloudflare-docs, context7, deepwiki), so repointing
	// turns this into a test that fails when someone else's service is down. The
	// gate above is also stale — it probes :8100 while the server binds :7061 per
	// helixAgentServePort — so the body has not run in a long time. Both need a
	// product decision (which endpoints are in scope for a connectivity gate),
	// not a subject swap. Tracked, deliberately left alone, not overlooked.
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "opencode", "opencode.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Skipf("OpenCode config not found: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
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

	for serverName, serverConfig := range config.GetMCPs() {
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
		t.Logf("Skipping npx server check in non-CI environment (acceptable)")
		return
	}

	// SAME SUBJECT DEFECT as TestOpenCodeSchemaValidation once had: the operator's
	// personal config, not an artifact this repository owns. NOT repointed to
	// openCodeSubjectPath() here because the swap is not neutral — it would flip
	// this test red: the shipped reference config declares NINE local npx servers
	// (everything, fetch, filesystem, git, memory, puppeteer, sequential-thinking,
	// sqlite, time), which this test asserts "MUST NOT be in config". Either the
	// assertion or the reference config is wrong, and deciding which is a product
	// call. Tracked, deliberately left alone, not overlooked.
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "opencode", "opencode.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Skipf("OpenCode config not found: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
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
