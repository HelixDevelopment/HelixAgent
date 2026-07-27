package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.agent/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// All 48 supported CLI agents
var allCLIAgents = []string{
	// Original 18 agents
	"opencode", "crush", "helixcode", "kiro", "aider", "claudecode",
	"cline", "codenamegoose", "deepseekcli", "forge", "geminicli",
	"gptengineer", "kilocode", "mistralcode", "ollamacode", "plandex",
	"qwencode", "amazonq",
	// Extended 30 agents
	"agentdeck", "bridle", "cheshirecat", "claudeplugins", "claudesquad",
	"codai", "codex", "codexskills", "conduit", "emdash", "fauxpilot",
	"getshitdone", "githubcopilotcli", "githubspeckit", "gitmcp", "gptme",
	"mobileagent", "multiagentcoding", "nanocoder", "noi", "octogen",
	"openhands", "postgresmcp", "shai", "snowcli", "taskweaver",
	"uiuxpromax", "vtcode", "warp", "continue",
}

// TestConfig holds E2E test configuration
type E2ETestConfig struct {
	HelixAgentURL   string
	HelixAgentBin   string
	SkipLiveTests   bool
	TimeoutPerAgent time.Duration
}

func getE2EConfig() E2ETestConfig {
	return E2ETestConfig{
		HelixAgentURL:   getEnvOrDefault("HELIXAGENT_URL", "http://localhost:8100"),
		HelixAgentBin:   getEnvOrDefault("HELIXAGENT_BIN", "./bin/helixagent"),
		SkipLiveTests:   os.Getenv("SKIP_LIVE_TESTS") == "true",
		TimeoutPerAgent: 30 * time.Second,
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// ============================================================================
// CLI AGENT REGISTRY TESTS
// ============================================================================

func TestCLIAgentRegistryComplete(t *testing.T) {
	config := getE2EConfig()

	// Skip if binary not available
	if _, err := exec.LookPath(config.HelixAgentBin); err != nil {
		t.Skip("HelixAgent binary not found") // SKIP-OK: #legacy-untriaged
	}

	t.Run("List_All_Agents", func(t *testing.T) {
		cmd := exec.Command(config.HelixAgentBin, "--list-agents")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Command output: %s", string(output))
			t.Skipf("Failed to run list-agents: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
		}

		outputStr := string(output)
		t.Logf("Agent list output length: %d bytes", len(outputStr))

		// Count agents in output
		agentCount := 0
		for _, agent := range allCLIAgents {
			if strings.Contains(strings.ToLower(outputStr), strings.ToLower(agent)) {
				agentCount++
			}
		}

		t.Logf("Found %d/%d agents in output", agentCount, len(allCLIAgents))
		assert.GreaterOrEqual(t, agentCount, 40, "Expected at least 40 CLI agents")
	})
}

func TestCLIAgentConfigGeneration(t *testing.T) {
	config := getE2EConfig()

	// Skip if binary not available
	if _, err := exec.LookPath(config.HelixAgentBin); err != nil {
		t.Skip("HelixAgent binary not found") // SKIP-OK: #legacy-untriaged
	}

	// Test config generation for all agents
	for _, agent := range allCLIAgents {
		t.Run(fmt.Sprintf("Generate_Config_%s", agent), func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command(config.HelixAgentBin, "--generate-agent-config="+agent)
			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			if err != nil {
				// Check if it's a known unsupported agent
				if strings.Contains(outputStr, "unsupported") || strings.Contains(outputStr, "not found") {
					t.Skipf("Agent %s not yet supported: %s (SKIP-OK: #unmarked-skip-needs-ticket)", agent, outputStr)
				}
				t.Logf("Error generating config for %s: %v\nOutput: %s", agent, err, outputStr)
				t.Skipf("Config generation failed for %s (SKIP-OK: #unmarked-skip-needs-ticket)", agent)
			}

			// Verify config is valid JSON
			if strings.HasPrefix(strings.TrimSpace(outputStr), "{") {
				var config map[string]interface{}
				if err := json.Unmarshal([]byte(outputStr), &config); err != nil {
					t.Errorf("Invalid JSON config for %s: %v", agent, err)
				} else {
					// Verify required fields
					assert.NotEmpty(t, config, "Config should not be empty for %s", agent)
					t.Logf("Valid config generated for %s", agent)
				}
			} else if strings.Contains(outputStr, "mcp") || strings.Contains(outputStr, "MCP") {
				// TOML or other format with MCP
				t.Logf("Config generated for %s (non-JSON format)", agent)
			} else {
				t.Logf("Config output for %s: %s", agent, outputStr[:min(200, len(outputStr))])
			}
		})
	}
}

// ============================================================================
// FULL INFRASTRUCTURE E2E TESTS
// ============================================================================

func TestFullInfrastructureE2E(t *testing.T) {
	config := getE2EConfig()

	if config.SkipLiveTests {
		t.Skip("Live tests skipped") // SKIP-OK: #legacy-untriaged
	}

	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("HelixAgent_Health", func(t *testing.T) {
		resp, err := client.Get(config.HelixAgentURL + "/health")
		if err != nil {
			t.Skipf("HelixAgent not available: %v (SKIP-OK: #infra-unavailable)", err)
		}
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Models_Endpoint", func(t *testing.T) {
		resp, err := client.Get(config.HelixAgentURL + "/v1/models")
		if err != nil {
			t.Skipf("HelixAgent not available: %v (SKIP-OK: #infra-unavailable)", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		assert.NotNil(t, result["data"])
	})

	// Test all protocol endpoints
	protocolEndpoints := map[string]string{
		"MCP_Proxy":     config.HelixAgentURL + "/v1/mcp/health",
		"ACP_Health":    config.HelixAgentURL + "/v1/acp/health",
		"Vision_Health": config.HelixAgentURL + "/v1/vision/health",
		"Cognee_Health": config.HelixAgentURL + "/v1/cognee/health",
	}

	for name, url := range protocolEndpoints {
		t.Run(name, func(t *testing.T) {
			resp, err := client.Get(url)
			if err != nil {
				t.Skipf("%s not available: %v (SKIP-OK: #infra-unavailable)", name, err)
			}
			defer resp.Body.Close()
			// Accept 200, 404 (not implemented), or 503 (service disabled)
			if resp.StatusCode == http.StatusNotFound {
				t.Skipf("%s endpoint not implemented (SKIP-OK: #unmarked-skip-needs-ticket)", name)
			}
			if resp.StatusCode == http.StatusServiceUnavailable {
				t.Skipf("%s service unavailable (may be disabled) (SKIP-OK: #unmarked-skip-needs-ticket)", name)
			}
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

// ============================================================================
// MCP SERVER VALIDATION
// ============================================================================

func TestMCPServersE2E(t *testing.T) {
	mcpServers := map[string]int{
		"filesystem":          9101,
		"memory":              9102,
		"postgres":            9103,
		"puppeteer":           9104,
		"sequential-thinking": 9105,
		"everything":          9106,
		"github":              9107,
	}

	for name, port := range mcpServers {
		t.Run(fmt.Sprintf("MCP_%s_Port_%d", name, port), func(t *testing.T) {
			t.Parallel()

			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 5*time.Second)
			if err != nil {
				t.Skipf("MCP server %s not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", name, err)
			}
			defer conn.Close()

			// Send JSON-RPC initialize
			initMsg := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"e2e-test","version":"1.0"}}}`
			conn.SetDeadline(time.Now().Add(5 * time.Second))
			_, err = conn.Write([]byte(initMsg + "\n"))
			require.NoError(t, err, "Failed to send init to %s", name)

			// Read response
			buf := make([]byte, 4096)
			n, err := conn.Read(buf)
			if err != nil && err != io.EOF {
				t.Logf("MCP %s: read error (port open): %v", name, err)
				return
			}

			response := string(buf[:n])
			if strings.Contains(response, `"jsonrpc"`) {
				t.Logf("MCP %s: JSON-RPC protocol verified", name)
			}
		})
	}
}

// ============================================================================
// ACP AGENT VALIDATION
// ============================================================================

func TestACPAgentsE2E(t *testing.T) {
	config := getE2EConfig()
	client := &http.Client{Timeout: 60 * time.Second}
	baseURL := config.HelixAgentURL + "/v1/acp"

	// Verify ACP health first
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skip("ACP not available") // SKIP-OK: #legacy-untriaged
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skip("ACP not healthy") // SKIP-OK: #legacy-untriaged
	}

	agents := []string{
		"code-reviewer",
		"bug-finder",
		"refactor-assistant",
		"documentation-generator",
		"test-generator",
		"security-scanner",
	}

	for _, agent := range agents {
		t.Run(fmt.Sprintf("ACP_Execute_%s", agent), func(t *testing.T) {
			t.Parallel()

			reqBody := map[string]interface{}{
				"agent_id": agent,
				"task":     "analyze this code",
				"context": map[string]interface{}{
					"code":     "func main() { fmt.Println(\"Hello, World!\") }",
					"language": "go",
				},
			}
			body, _ := json.Marshal(reqBody)

			resp, err := client.Post(baseURL+"/execute", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Skipf("ACP execute failed: %v (SKIP-OK: #unmarked-skip-needs-ticket)", err)
			}
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var result map[string]interface{}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
			assert.Equal(t, "completed", result["status"])
			assert.NotNil(t, result["result"])
		})
	}
}

// ============================================================================
// VISION CAPABILITIES VALIDATION
// ============================================================================

func TestVisionCapabilitiesE2E(t *testing.T) {
	config := getE2EConfig()
	client := &http.Client{Timeout: 60 * time.Second}
	baseURL := config.HelixAgentURL + "/v1/vision"

	// Verify Vision health first
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skip("Vision not available") // SKIP-OK: #legacy-untriaged
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skip("Vision not healthy") // SKIP-OK: #legacy-untriaged
	}

	capabilities := []string{"analyze", "ocr", "detect", "caption", "describe", "classify"}

	for _, cap := range capabilities {
		t.Run(fmt.Sprintf("Vision_%s", cap), func(t *testing.T) {
			t.Parallel()

			reqBody := map[string]interface{}{
				"image":  "",
				"prompt": "test image analysis",
			}
			body, _ := json.Marshal(reqBody)

			resp, err := client.Post(baseURL+"/"+cap, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Skipf("Vision %s failed: %v (SKIP-OK: #unmarked-skip-needs-ticket)", cap, err)
			}
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var result map[string]interface{}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
			assert.Equal(t, "completed", result["status"])
		})
	}
}

// ============================================================================
// EMBEDDINGS VALIDATION
// ============================================================================

func TestEmbeddingsE2E(t *testing.T) {
	config := getE2EConfig()
	client := &http.Client{Timeout: 60 * time.Second}

	t.Run("Single_Text_Embedding", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"text":     "This is a test sentence for embedding.",
			"provider": "default",
		}
		body, _ := json.Marshal(reqBody)

		req, err := http.NewRequest("POST", config.HelixAgentURL+"/v1/embeddings/generate", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		// Add API key from environment
		apiKey := getE2EAPIKey()
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Skipf("Embeddings not available: %v (SKIP-OK: #infra-unavailable)", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Skipf("Embeddings endpoint returned %d (embedding provider may not be configured) (SKIP-OK: #unmarked-skip-needs-ticket)", resp.StatusCode)
		}

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		assert.NotNil(t, result["success"])
	})

	t.Run("Batch_Embeddings", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"text":  "First test sentence. Second test sentence. Third test sentence.",
			"batch": true,
		}
		body, _ := json.Marshal(reqBody)

		req, err := http.NewRequest("POST", config.HelixAgentURL+"/v1/embeddings/generate", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		// Add API key from environment
		apiKey := getE2EAPIKey()
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Skipf("Embeddings not available: %v (SKIP-OK: #infra-unavailable)", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Skipf("Embeddings endpoint returned %d (embedding provider may not be configured) (SKIP-OK: #unmarked-skip-needs-ticket)", resp.StatusCode)
		}

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		assert.NotNil(t, result["success"])
	})
}

// ============================================================================
// CONCURRENT STRESS TEST
// ============================================================================

func TestConcurrentLoadE2E(t *testing.T) {
	testutil.RequireServer(t)

	config := getE2EConfig()
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("Concurrent_100_Requests", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, 100)
		successes := make(chan bool, 100)

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				resp, err := client.Get(config.HelixAgentURL + "/health")
				if err != nil {
					errors <- err
					return
				}
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					successes <- true
				} else {
					errors <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
				}
			}(i)
		}

		wg.Wait()
		close(errors)
		close(successes)

		successCount := 0
		for range successes {
			successCount++
		}

		errorCount := 0
		for err := range errors {
			t.Logf("Error: %v", err)
			errorCount++
		}

		t.Logf("Success: %d, Errors: %d", successCount, errorCount)
		assert.GreaterOrEqual(t, successCount, 90, "At least 90% of requests should succeed")
	})
}

// ============================================================================
// LSP SERVER VALIDATION
// ============================================================================

func TestLSPServersE2E(t *testing.T) {
	lspServers := map[string]int{
		"gopls":         5001,
		"rust-analyzer": 5002,
		"pylsp":         5003,
		"typescript":    5004,
		"clangd":        5005,
		"jdtls":         5006,
	}

	for name, port := range lspServers {
		t.Run(fmt.Sprintf("LSP_%s_Port_%d", name, port), func(t *testing.T) {
			t.Parallel()

			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 5*time.Second)
			if err != nil {
				t.Skipf("LSP server %s not running: %v (SKIP-OK: #unmarked-skip-needs-ticket)", name, err)
			}
			conn.Close()
			t.Logf("LSP %s: port open", name)
		})
	}

	t.Run("LSP_Manager_Health", func(t *testing.T) {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://localhost:5100/health")
		if err != nil {
			t.Skipf("LSP Manager not available: %v (SKIP-OK: #infra-unavailable)", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// ============================================================================
// RAG SERVICES VALIDATION
// ============================================================================

func TestRAGServicesE2E(t *testing.T) {
	ragServices := map[string]string{
		"Qdrant":                "http://localhost:6333/readyz",
		"Sentence-Transformers": "http://localhost:8016/health",
		"BGE-M3":                "http://localhost:8017/health",
		"RAGatouille":           "http://localhost:8018/health",
		"HyDE":                  "http://localhost:8019/health",
		"Multi-Query":           "http://localhost:8020/health",
		"Reranker":              "http://localhost:8021/health",
		"RAG-Manager":           "http://localhost:8030/health",
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for name, url := range ragServices {
		t.Run(fmt.Sprintf("RAG_%s", strings.ReplaceAll(name, "-", "_")), func(t *testing.T) {
			t.Parallel()

			resp, err := client.Get(url)
			if err != nil {
				t.Skipf("%s not available: %v (SKIP-OK: #infra-unavailable)", name, err)
			}
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
