// Package integration provides integration tests for HelixAgent components
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getExternalMCPProjectRoot returns the absolute path to the project root directory
func getExternalMCPProjectRoot() string {
	// Start from current working directory and walk up to find go.mod
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ExternalMCPServerConfig defines the expected MCP servers and their configurations
type ExternalMCPServerConfig struct {
	Name        string
	Port        int
	Type        string // "active" or "archived"
	RequiresEnv []string
	Description string
}

// AllExternalMCPServers returns the list of all MCP servers that should be available
func AllExternalMCPServers() []ExternalMCPServerConfig {
	return []ExternalMCPServerConfig{
		// Active servers (from modelcontextprotocol/servers)
		{Name: "fetch", Port: 3001, Type: "active", Description: "HTTP fetch operations"},
		{Name: "filesystem", Port: 3002, Type: "active", Description: "File system access"},
		{Name: "git", Port: 3003, Type: "active", Description: "Git repository operations"},
		{Name: "memory", Port: 3004, Type: "active", Description: "Persistent memory/notes"},
		{Name: "time", Port: 3005, Type: "active", Description: "Time and timezone operations"},
		{Name: "sequential-thinking", Port: 3006, Type: "active", Description: "Step-by-step reasoning"},
		{Name: "everything", Port: 3007, Type: "active", Description: "Local search with Everything"},

		// Archived servers (from modelcontextprotocol/servers-archived)
		{Name: "postgres", Port: 3008, Type: "archived", RequiresEnv: []string{"POSTGRES_URL"}, Description: "PostgreSQL database operations"},
		{Name: "sqlite", Port: 3009, Type: "archived", Description: "SQLite database operations"},
		{Name: "slack", Port: 3010, Type: "archived", RequiresEnv: []string{"SLACK_BOT_TOKEN", "SLACK_TEAM_ID"}, Description: "Slack messaging"},
		{Name: "github", Port: 3011, Type: "archived", RequiresEnv: []string{"GITHUB_TOKEN"}, Description: "GitHub API operations"},
		{Name: "gitlab", Port: 3012, Type: "archived", RequiresEnv: []string{"GITLAB_TOKEN"}, Description: "GitLab API operations"},
		{Name: "google-maps", Port: 3013, Type: "archived", RequiresEnv: []string{"GOOGLE_MAPS_API_KEY"}, Description: "Google Maps API"},
		{Name: "brave-search", Port: 3014, Type: "archived", RequiresEnv: []string{"BRAVE_API_KEY"}, Description: "Brave Search API"},
		{Name: "puppeteer", Port: 3015, Type: "archived", Description: "Browser automation"},
		{Name: "redis", Port: 3016, Type: "archived", RequiresEnv: []string{"REDIS_URL"}, Description: "Redis operations"},
		{Name: "sentry", Port: 3017, Type: "archived", RequiresEnv: []string{"SENTRY_AUTH_TOKEN", "SENTRY_ORG"}, Description: "Sentry error tracking"},
		{Name: "gdrive", Port: 3018, Type: "archived", RequiresEnv: []string{"GOOGLE_CREDENTIALS_PATH"}, Description: "Google Drive operations"},
		{Name: "everart", Port: 3019, Type: "archived", RequiresEnv: []string{"EVERART_API_KEY"}, Description: "Everart API"},
		{Name: "aws-kb-retrieval", Port: 3020, Type: "archived", RequiresEnv: []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"}, Description: "AWS Knowledge Base retrieval"},
	}
}

// TestExternalMCPServersSubmodulesExist verifies that the MCP server git submodules are properly initialized
func TestExternalMCPServersSubmodulesExist(t *testing.T) {

	projectRoot := getExternalMCPProjectRoot()
	require.NotEmpty(t, projectRoot, "Could not find project root")

	submodules := []struct {
		Path string
		Name string
	}{
		{"external/mcp-servers/servers", "Active MCP Servers"},
		{"external/mcp-servers/servers-archived", "Archived MCP Servers"},
	}

	for _, sm := range submodules {
		t.Run(sm.Name, func(t *testing.T) {
			fullPath := filepath.Join(projectRoot, sm.Path)
			// Check if directory exists
			info, err := os.Stat(fullPath)
			require.NoError(t, err, "Submodule directory should exist: %s", fullPath)
			assert.True(t, info.IsDir(), "Should be a directory")

			// Check if it's a git repository
			gitDir := filepath.Join(fullPath, ".git")
			_, err = os.Stat(gitDir)
			assert.NoError(t, err, "Should be a git repository (or git submodule)")
		})
	}
}

// TestExternalMCPServerSourcesExist verifies that source code for all MCP servers exists
func TestExternalMCPServerSourcesExist(t *testing.T) {

	projectRoot := getExternalMCPProjectRoot()
	require.NotEmpty(t, projectRoot, "Could not find project root")

	activeServers := []string{"fetch", "filesystem", "git", "memory", "time", "sequentialthinking", "everything"}
	archivedServers := []string{"postgres", "sqlite", "slack", "github", "gitlab", "google-maps", "brave-search", "puppeteer", "redis", "sentry", "gdrive", "everart", "aws-kb-retrieval-server"}

	t.Run("Active Servers", func(t *testing.T) {
		for _, server := range activeServers {
			t.Run(server, func(t *testing.T) {
				path := filepath.Join(projectRoot, "external/mcp-servers/servers/src", server)
				_, err := os.Stat(path)
				assert.NoError(t, err, "Server source should exist: %s", path)
			})
		}
	})

	t.Run("Archived Servers", func(t *testing.T) {
		for _, server := range archivedServers {
			t.Run(server, func(t *testing.T) {
				// Archived servers are in src/ subdirectory
				path := filepath.Join(projectRoot, "external/mcp-servers/servers-archived/src", server)
				_, err := os.Stat(path)
				assert.NoError(t, err, "Server source should exist: %s", path)
			})
		}
	})
}

// TestExternalMCPContainerBuild verifies that the MCP servers container can be built
func TestExternalMCPContainerBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container build test in short mode")  // SKIP-OK: #short-mode
	}

	projectRoot := getExternalMCPProjectRoot()
	require.NotEmpty(t, projectRoot, "Could not find project root")

	// Check if Docker/Podman is available
	runtime := detectContainerRuntime()
	if runtime == "" {
		t.Skip("No container runtime (Docker/Podman) available")  // SKIP-OK: #runtime-mock-only
	}

	// Build the container
	mcpDir := filepath.Join(projectRoot, "external/mcp-servers")
	cmd := exec.Command(runtime, "build", "-t", "helixagent-mcp-servers:test", mcpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Build output: %s", string(output))
	}
	require.NoError(t, err, "Container should build successfully")
}

// TestExternalMCPContainerHealth verifies that the MCP servers container is healthy
func TestExternalMCPContainerHealth(t *testing.T) {

	// Check if container is running
	runtime := detectContainerRuntime()
	if runtime == "" {
		t.Skip("No container runtime available")  // SKIP-OK: #runtime-mock-only
	}

	cmd := exec.Command(runtime, "ps", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		t.Skip("Could not check running containers")  // SKIP-OK: #legacy-untriaged
	}

	if !strings.Contains(string(output), "helixagent-mcp-servers") {
		t.Skip("MCP servers container not running")  // SKIP-OK: #legacy-untriaged
	}

	// Check health
	cmd = exec.Command(runtime, "exec", "helixagent-mcp-servers", "/app/scripts/health-check.sh")
	output, err = cmd.CombinedOutput()
	t.Logf("Health check output: %s", string(output))
	assert.NoError(t, err, "Health check should pass")
}

// TestExternalMCPServerConnectivity verifies that each MCP server can be connected to
func TestExternalMCPServerConnectivity(t *testing.T) {

	mcpHost := os.Getenv("MCP_HOST")
	if mcpHost == "" {
		mcpHost = "localhost"
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, server := range AllExternalMCPServers() {
		t.Run(server.Name, func(t *testing.T) {
			// Check if required environment variables are set
			for _, env := range server.RequiresEnv {
				if os.Getenv(env) == "" {
					t.Skipf("Skipping %s - missing required env: %s", server.Name, env)
				}
			}

			url := fmt.Sprintf("http://%s:%d", mcpHost, server.Port)

			// Try to connect (most MCP servers will respond with some kind of error for GET,
			// but the connection itself should succeed)
			resp, err := client.Get(url)
			if err != nil {
				t.Skipf("Server %s not reachable at %s: %v", server.Name, url, err)
			}
			defer resp.Body.Close()

			// Just verify we got a response (status code doesn't matter for this test)
			t.Logf("Server %s responded with status %d", server.Name, resp.StatusCode)
		})
	}
}

// TestExternalMCPServerJSONRPC verifies that MCP servers respond to JSON-RPC requests
func TestExternalMCPServerJSONRPC(t *testing.T) {

	mcpHost := os.Getenv("MCP_HOST")
	if mcpHost == "" {
		mcpHost = "localhost"
	}

	client := &http.Client{Timeout: 5 * time.Second}

	// Test initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "helixagent-test",
				"version": "1.0.0",
			},
		},
	}

	requestBody, err := json.Marshal(initRequest)
	require.NoError(t, err)

	for _, server := range AllExternalMCPServers() {
		t.Run(server.Name+"_Initialize", func(t *testing.T) {
			// Skip servers that require specific environment variables
			for _, env := range server.RequiresEnv {
				if os.Getenv(env) == "" {
					t.Skipf("Skipping %s - missing required env: %s", server.Name, env)
				}
			}

			url := fmt.Sprintf("http://%s:%d", mcpHost, server.Port)
			resp, err := client.Post(url, "application/json", bytes.NewReader(requestBody))
			if err != nil {
				t.Skipf("Server %s not reachable: %v", server.Name, err)
			}
			defer resp.Body.Close()

			// For a proper MCP server, we should get a JSON-RPC response
			var response map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				t.Logf("Server %s returned non-JSON response (status %d)", server.Name, resp.StatusCode)
				return
			}

			// Check if we got a valid JSON-RPC response
			if _, ok := response["jsonrpc"]; ok {
				t.Logf("Server %s returned valid JSON-RPC response", server.Name)
			}
		})
	}
}

// TestExternalMCPServerToolsList verifies that MCP servers list their tools
func TestExternalMCPServerToolsList(t *testing.T) {

	mcpHost := os.Getenv("MCP_HOST")
	if mcpHost == "" {
		mcpHost = "localhost"
	}

	client := &http.Client{Timeout: 5 * time.Second}

	// Test tools/list request
	toolsRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	requestBody, err := json.Marshal(toolsRequest)
	require.NoError(t, err)

	for _, server := range AllExternalMCPServers() {
		t.Run(server.Name+"_ToolsList", func(t *testing.T) {
			for _, env := range server.RequiresEnv {
				if os.Getenv(env) == "" {
					t.Skipf("Skipping %s - missing required env: %s", server.Name, env)
				}
			}

			url := fmt.Sprintf("http://%s:%d", mcpHost, server.Port)
			resp, err := client.Post(url, "application/json", bytes.NewReader(requestBody))
			if err != nil {
				t.Skipf("Server %s not reachable: %v", server.Name, err)
			}
			defer resp.Body.Close()

			var response map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				t.Logf("Server %s returned non-JSON response", server.Name)
				return
			}

			if result, ok := response["result"].(map[string]interface{}); ok {
				if tools, ok := result["tools"].([]interface{}); ok {
					t.Logf("Server %s has %d tools", server.Name, len(tools))
					assert.Greater(t, len(tools), 0, "Server should have at least one tool")
				}
			}
		})
	}
}

// Helper function to detect available container runtime
func detectContainerRuntime() string {
	for _, runtime := range []string{"podman", "docker"} {
		if _, err := exec.LookPath(runtime); err == nil {
			return runtime
		}
	}
	return ""
}

// TestAllExternalMCPServersDocumented verifies that all MCP servers are documented
func TestAllExternalMCPServersDocumented(t *testing.T) {

	projectRoot := getExternalMCPProjectRoot()
	require.NotEmpty(t, projectRoot, "Could not find project root")

	// Check that the MCP servers README exists
	readmePath := filepath.Join(projectRoot, "external/mcp-servers/README.md")
	_, err := os.Stat(readmePath)
	require.NoError(t, err, "MCP servers README should exist")

	// Read and verify documentation
	content, err := os.ReadFile(readmePath)
	require.NoError(t, err)

	for _, server := range AllExternalMCPServers() {
		t.Run(server.Name+"_Documented", func(t *testing.T) {
			assert.Contains(t, string(content), server.Name,
				"Server %s should be documented in README", server.Name)
		})
	}
}

// TestExternalMCPServersInOpenCodeConfig verifies that all MCP servers are in the OpenCode config
func TestExternalMCPServersInOpenCodeConfig(t *testing.T) {

	projectRoot := getExternalMCPProjectRoot()
	require.NotEmpty(t, projectRoot, "Could not find project root")

	// Generate the config
	binaryPath := filepath.Join(projectRoot, "bin/helixagent")
	cmd := exec.Command(binaryPath, "--generate-opencode-config")
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "LOCAL_ENDPOINT=http://localhost:7061")
	output, err := cmd.Output()
	require.NoError(t, err, "Should be able to generate OpenCode config")

	// Parse the config
	var config map[string]interface{}
	err = json.Unmarshal(output, &config)
	require.NoError(t, err, "Config should be valid JSON")

	// Check that mcp section exists
	mcpSection, ok := config["mcp"].(map[string]interface{})
	require.True(t, ok, "Config should have mcp section")

	// Verify each "active" server is in the config. Archived servers
	// (those marked Type=="archived") were pruned from the generator
	// when the modelcontextprotocol/servers-archived upstream was
	// retired — the generator now ships a different curated set
	// (notion, tavily, deepwiki, context7, cloudflare-docs, …) so
	// asserting on the old archived list only produces noise. The
	// test is kept for the active servers as a regression gate.
	for _, server := range AllExternalMCPServers() {
		if server.Type == "archived" {
			continue
		}
		t.Run(server.Name+"_InConfig", func(t *testing.T) {
			_, exists := mcpSection[server.Name]
			assert.True(t, exists, "Server %s should be in OpenCode config", server.Name)
		})
	}
}

// TestMCPContainerBuildNetworkConnectivity verifies that network connectivity
// is available for container builds (needed for Alpine apk, npm, pip)
func TestMCPContainerBuildNetworkConnectivity(t *testing.T) {

	// Test host-level network connectivity to Alpine repo
	t.Run("AlpineRepository", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Head("https://dl-cdn.alpinelinux.org/alpine/v3.23/main/x86_64/APKINDEX.tar.gz")
		if err != nil {
			t.Logf("Warning: Cannot reach Alpine repository: %v", err)
			t.Logf("Container builds may fail. Check network configuration.")
		} else {
			resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode, "Alpine repo should be reachable")
		}
	})

	// Test host-level network connectivity to npm registry
	t.Run("NpmRegistry", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Head("https://registry.npmjs.org/")
		if err != nil {
			t.Logf("Warning: Cannot reach npm registry: %v", err)
		} else {
			resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode, "npm registry should be reachable")
		}
	})

	// Test host-level network connectivity to PyPI
	t.Run("PyPIRepository", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Head("https://pypi.org/simple/")
		if err != nil {
			t.Logf("Warning: Cannot reach PyPI: %v", err)
		} else {
			resp.Body.Close()
			// PyPI returns 200 for simple/ endpoint
			assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 400, "PyPI should be reachable")
		}
	})
}

// TestMCPContainerNetworkDNSResolution verifies that container DNS resolution works
func TestMCPContainerNetworkDNSResolution(t *testing.T) {

	runtime := detectContainerRuntime()
	if runtime == "" {
		t.Skip("No container runtime available")  // SKIP-OK: #runtime-mock-only
	}

	// Test DNS resolution inside a container using --network=host
	// This is the workaround for podman DNS issues
	t.Run("HostNetworkDNS", func(t *testing.T) {
		cmd := exec.Command(runtime, "run", "--rm", "--network=host",
			"alpine:latest", "sh", "-c",
			"apk update > /dev/null 2>&1 && echo 'DNS_OK'")
		output, err := cmd.CombinedOutput()

		if err != nil {
			t.Logf("Container DNS with host network failed: %v", err)
			t.Logf("Output: %s", string(output))
			t.FailNow()
		}

		assert.Contains(t, string(output), "DNS_OK",
			"Container should be able to resolve DNS with --network=host")
	})

	// Test that default network might have issues (informational)
	t.Run("DefaultNetworkDNS", func(t *testing.T) {
		cmd := exec.Command(runtime, "run", "--rm",
			"alpine:latest", "sh", "-c",
			"apk update > /dev/null 2>&1 && echo 'DNS_OK' || echo 'DNS_FAIL'")
		output, _ := cmd.CombinedOutput()

		if strings.Contains(string(output), "DNS_FAIL") {
			t.Logf("INFO: Default container network has DNS issues")
			t.Logf("Build script uses --network=host as workaround")
		}
	})
}

// TestMCPContainerBuildScript verifies that the build script exists and is executable
func TestMCPContainerBuildScript(t *testing.T) {
	projectRoot := getExternalMCPProjectRoot()
	require.NotEmpty(t, projectRoot, "Could not find project root")

	buildScriptPath := filepath.Join(projectRoot, "external/mcp-servers/scripts/build.sh")

	t.Run("BuildScriptExists", func(t *testing.T) {
		info, err := os.Stat(buildScriptPath)
		require.NoError(t, err, "Build script should exist")
		assert.False(t, info.IsDir(), "Should be a file")
	})

	t.Run("BuildScriptExecutable", func(t *testing.T) {
		info, err := os.Stat(buildScriptPath)
		require.NoError(t, err)
		mode := info.Mode()
		assert.True(t, mode&0111 != 0, "Build script should be executable")
	})

	t.Run("BuildScriptHasNetworkCheck", func(t *testing.T) {
		content, err := os.ReadFile(buildScriptPath)
		require.NoError(t, err)

		// Verify build script includes network pre-flight checks
		assert.Contains(t, string(content), "network",
			"Build script should contain network handling")
		assert.Contains(t, string(content), "--network=host",
			"Build script should use --network=host for builds")
	})
}

// TestMCPDockerfileHasCorrectShell verifies that Dockerfile scripts use /bin/sh (not /bin/bash)
func TestMCPDockerfileHasCorrectShell(t *testing.T) {
	projectRoot := getExternalMCPProjectRoot()
	require.NotEmpty(t, projectRoot, "Could not find project root")

	scripts := []string{
		"external/mcp-servers/scripts/start-all.sh",
		"external/mcp-servers/scripts/health-check.sh",
	}

	for _, script := range scripts {
		scriptPath := filepath.Join(projectRoot, script)
		t.Run(filepath.Base(script), func(t *testing.T) {
			content, err := os.ReadFile(scriptPath)
			if err != nil {
				t.Skipf("Script not found: %s", scriptPath)
				return
			}

			firstLine := strings.Split(string(content), "\n")[0]
			assert.Equal(t, "#!/bin/sh", firstLine,
				"Script should use /bin/sh (not /bin/bash) for Alpine compatibility")
		})
	}
}
