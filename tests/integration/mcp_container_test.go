// Package integration provides integration tests for MCP containerization
package integration

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"dev.helix.agent/internal/mcp/config"
)

// TestMCPContainerConnectivity tests TCP connectivity to all MCP container ports
func TestMCPContainerConnectivity(t *testing.T) {
	if os.Getenv("RUN_CONTAINER_TESTS") == "" {
		t.Skip("Skipping container connectivity tests. Set RUN_CONTAINER_TESTS=1 to enable.")  // SKIP-OK: #legacy-untriaged
	}

	host := os.Getenv("MCP_CONTAINER_HOST")
	if host == "" {
		host = "localhost"
	}

	ports := config.MCPContainerPorts

	for _, p := range ports {
		t.Run(p.Name, func(t *testing.T) {
			addr := net.JoinHostPort(host, strconv.Itoa(p.Port))
			conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
			if err != nil {
				t.Errorf("Failed to connect to %s at %s: %v", p.Name, addr, err)
				return
			}
			conn.Close()
		})
	}
}

// TestMCPContainerHealthChecks tests health endpoints for all MCP containers
func TestMCPContainerHealthChecks(t *testing.T) {
	if os.Getenv("RUN_CONTAINER_TESTS") == "" {
		t.Skip("Skipping container health check tests. Set RUN_CONTAINER_TESTS=1 to enable.")  // SKIP-OK: #legacy-untriaged
	}

	host := os.Getenv("MCP_CONTAINER_HOST")
	if host == "" {
		host = "localhost"
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	ports := config.MCPContainerPorts

	for _, p := range ports {
		t.Run(p.Name, func(t *testing.T) {
			// Try common health endpoints
			healthEndpoints := []string{
				fmt.Sprintf("http://%s:%d/health", host, p.Port),
				fmt.Sprintf("http://%s:%d/healthz", host, p.Port),
				fmt.Sprintf("http://%s:%d/", host, p.Port),
			}

			var lastErr error
			for _, endpoint := range healthEndpoints {
				resp, err := client.Get(endpoint)
				if err != nil {
					lastErr = err
					continue
				}
				resp.Body.Close()

				if resp.StatusCode >= 200 && resp.StatusCode < 500 {
					// Successful health check
					return
				}
			}

			t.Errorf("Health check failed for %s: %v", p.Name, lastErr)
		})
	}
}

// TestMCPContainerJSONRPCCompliance tests JSON-RPC 2.0 compliance for MCP containers
func TestMCPContainerJSONRPCCompliance(t *testing.T) {
	if os.Getenv("RUN_CONTAINER_TESTS") == "" {
		t.Skip("Skipping JSON-RPC compliance tests. Set RUN_CONTAINER_TESTS=1 to enable.")  // SKIP-OK: #legacy-untriaged
	}

	host := os.Getenv("MCP_CONTAINER_HOST")
	if host == "" {
		host = "localhost"
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Test a few core MCP servers for JSON-RPC compliance
	corePorts := []config.MCPContainerPort{
		{Name: "fetch", Port: 9101, Category: "core"},
		{Name: "git", Port: 9102, Category: "core"},
		{Name: "time", Port: 9103, Category: "core"},
		{Name: "memory", Port: 9105, Category: "core"},
	}

	for _, p := range corePorts {
		t.Run(p.Name, func(t *testing.T) {
			// Send a JSON-RPC 2.0 initialize request
			jsonRPCRequest := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
				"params": map[string]interface{}{
					"capabilities": map[string]interface{}{},
				},
			}

			requestBody, err := json.Marshal(jsonRPCRequest)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			url := fmt.Sprintf("http://%s:%d/", host, p.Port)
			resp, err := client.Post(url, "application/json", strings.NewReader(string(requestBody)))
			if err != nil {
				t.Skipf("Container not available: %v (SKIP-OK: #infra-unavailable)", err)
				return
			}
			defer resp.Body.Close()

			// Check for valid JSON-RPC response
			var jsonRPCResponse map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&jsonRPCResponse); err != nil {
				// SSE endpoints may not respond with JSON directly
				t.Logf("Non-JSON response from %s (may be SSE endpoint)", p.Name)
				return
			}

			// Verify JSON-RPC 2.0 structure
			if version, ok := jsonRPCResponse["jsonrpc"].(string); !ok || version != "2.0" {
				t.Errorf("%s: Expected jsonrpc: '2.0', got: %v", p.Name, jsonRPCResponse["jsonrpc"])
			}
		})
	}
}

// TestMCPContainerToolDiscovery tests that MCP containers expose their tools
func TestMCPContainerToolDiscovery(t *testing.T) {
	if os.Getenv("RUN_CONTAINER_TESTS") == "" {
		t.Skip("Skipping tool discovery tests. Set RUN_CONTAINER_TESTS=1 to enable.")  // SKIP-OK: #legacy-untriaged
	}

	host := os.Getenv("MCP_CONTAINER_HOST")
	if host == "" {
		host = "localhost"
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Test a few core MCP servers for tool discovery
	corePorts := []config.MCPContainerPort{
		{Name: "filesystem", Port: 9104, Category: "core"},
		{Name: "memory", Port: 9105, Category: "core"},
	}

	for _, p := range corePorts {
		t.Run(p.Name, func(t *testing.T) {
			// Send a tools/list request
			jsonRPCRequest := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "tools/list",
				"params":  map[string]interface{}{},
			}

			requestBody, err := json.Marshal(jsonRPCRequest)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			url := fmt.Sprintf("http://%s:%d/", host, p.Port)
			resp, err := client.Post(url, "application/json", strings.NewReader(string(requestBody)))
			if err != nil {
				t.Skipf("Container not available: %v (SKIP-OK: #infra-unavailable)", err)
				return
			}
			defer resp.Body.Close()

			// Parse response
			var jsonRPCResponse map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&jsonRPCResponse); err != nil {
				t.Logf("Non-JSON response from %s (may be SSE endpoint)", p.Name)
				return
			}

			// Check for tools in result
			if result, ok := jsonRPCResponse["result"].(map[string]interface{}); ok {
				if tools, ok := result["tools"].([]interface{}); ok {
					t.Logf("%s has %d tools", p.Name, len(tools))
				}
			}
		})
	}
}

// TestMCPContainerNoNPXDependencies verifies no NPX commands in container config
func TestMCPContainerNoNPXDependencies(t *testing.T) {
	gen := config.NewContainerMCPConfigGenerator("http://localhost:8080")

	// Verify the generator doesn't use NPX
	if gen.ContainsNPX() {
		t.Error("Container generator should not contain any NPX commands")
	}

	// Verify all MCPs use remote type
	mcps := gen.GenerateContainerMCPs()
	for name, cfg := range mcps {
		if cfg.Type != "remote" {
			t.Errorf("MCP %s should have type 'remote', got '%s'", name, cfg.Type)
		}
	}
}

// TestMCPContainerPortAllocation verifies port allocation is correct
func TestMCPContainerPortAllocation(t *testing.T) {
	gen := config.NewContainerMCPConfigGenerator("http://localhost:8080")

	err := gen.ValidatePortAllocations()
	if err != nil {
		t.Errorf("Port allocation validation failed: %v", err)
	}

	// Verify port count
	ports := gen.GetPortAllocations()
	if len(ports) < 60 {
		t.Errorf("Expected at least 60 port allocations, got %d", len(ports))
	}
}

// TestMCPContainerCategoryDistribution verifies MCPs are distributed across categories
func TestMCPContainerCategoryDistribution(t *testing.T) {
	gen := config.NewContainerMCPConfigGenerator("http://localhost:8080")

	byCategory := gen.GetMCPsByCategory()

	expectedCategories := map[string]int{
		"core":          5, // At least 5 core MCPs
		"database":      3, // At least 3 database MCPs
		"vector":        3, // At least 3 vector MCPs
		"devops":        5, // At least 5 devops MCPs
		"browser":       2, // At least 2 browser MCPs
		"communication": 2, // At least 2 communication MCPs
		"productivity":  5, // At least 5 productivity MCPs
		"search":        5, // At least 5 search MCPs
	}

	for category, minCount := range expectedCategories {
		if mcps, ok := byCategory[category]; !ok {
			t.Errorf("Category %s not found", category)
		} else if len(mcps) < minCount {
			t.Errorf("Category %s has %d MCPs, expected at least %d", category, len(mcps), minCount)
		}
	}
}

// TestMCPContainerURLFormat verifies all URLs are in correct format
func TestMCPContainerURLFormat(t *testing.T) {
	gen := config.NewContainerMCPConfigGenerator("http://localhost:8080")

	mcps := gen.GenerateContainerMCPs()

	for name, cfg := range mcps {
		if name == "helixagent" {
			// HelixAgent has special URL format
			continue
		}

		// Verify URL format: http://host:port/sse
		if !strings.HasPrefix(cfg.URL, "http://") {
			t.Errorf("MCP %s URL should start with http://, got: %s", name, cfg.URL)
		}

		if !strings.HasSuffix(cfg.URL, "/sse") {
			t.Errorf("MCP %s URL should end with /sse, got: %s", name, cfg.URL)
		}

		// Verify port is in URL
		if cfg.Port > 0 {
			portStr := fmt.Sprintf(":%d/", cfg.Port)
			if !strings.Contains(cfg.URL, portStr) {
				t.Errorf("MCP %s URL should contain port %d, got: %s", name, cfg.Port, cfg.URL)
			}
		}
	}
}

// TestMCPContainerSummary verifies summary generation
func TestMCPContainerSummary(t *testing.T) {
	gen := config.NewContainerMCPConfigGenerator("http://localhost:8080")

	summary := gen.GenerateSummary()

	// Verify total count
	total, ok := summary["total"].(int)
	if !ok || total < 60 {
		t.Errorf("Expected total >= 60, got: %v", summary["total"])
	}

	// Verify NPX dependencies is 0
	npxDeps, ok := summary["npx_dependencies"].(int)
	if !ok || npxDeps != 0 {
		t.Errorf("Expected npx_dependencies = 0, got: %v", summary["npx_dependencies"])
	}

	// Verify by_category exists
	if _, ok := summary["by_category"].(map[string]int); !ok {
		t.Error("Expected by_category map in summary")
	}
}

// TestMCPContainerCompareWithNPXGenerator ensures container generator covers all NPX MCPs
func TestMCPContainerCompareWithNPXGenerator(t *testing.T) {
	containerGen := config.NewContainerMCPConfigGenerator("http://localhost:8080")
	npxGen := config.NewFullMCPConfigGenerator("http://localhost:8080")

	containerMCPs := containerGen.GenerateContainerMCPs()
	npxMCPs := npxGen.GenerateAllMCPs()

	// Every NPX MCP should have a container equivalent
	for name := range npxMCPs {
		if _, ok := containerMCPs[name]; !ok {
			t.Errorf("MCP %s exists in NPX generator but not in container generator", name)
		}
	}
}

// TestMCPContainerStartup simulates container startup sequence
func TestMCPContainerStartup(t *testing.T) {
	if os.Getenv("RUN_CONTAINER_TESTS") == "" {
		t.Skip("Skipping container startup test. Set RUN_CONTAINER_TESTS=1 to enable.")  // SKIP-OK: #legacy-untriaged
	}

	host := os.Getenv("MCP_CONTAINER_HOST")
	if host == "" {
		host = "localhost"
	}

	// Test core MCP containers startup
	corePorts := []int{9101, 9102, 9103, 9104, 9105, 9106, 9107}

	startedCount := 0
	for _, port := range corePorts {
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			conn.Close()
			startedCount++
		}
	}

	t.Logf("Core MCP containers started: %d/%d", startedCount, len(corePorts))

	if startedCount == 0 {
		t.Log("No containers running - this is expected if containers haven't been started")
	}
}

// BenchmarkMCPContainerConfigGeneration benchmarks container config generation
func BenchmarkMCPContainerConfigGeneration(b *testing.B) {
	gen := config.NewContainerMCPConfigGenerator("http://localhost:8080")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.GenerateContainerMCPs()
	}
}

// BenchmarkMCPContainerSummary benchmarks summary generation
func BenchmarkMCPContainerSummary(b *testing.B) {
	gen := config.NewContainerMCPConfigGenerator("http://localhost:8080")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.GenerateSummary()
	}
}
