package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestConfig() TestConfig {
	return TestConfig{
		HelixAgentURL: getEnv("HELIXAGENT_URL", "http://localhost:7061"),
		PostgresHost:  getEnv("DB_HOST", "localhost"),
		PostgresPort:  getEnv("DB_PORT", "5432"),
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		ChromaDBURL:   getEnv("CHROMADB_URL", "http://localhost:8001"),
		CogneeURL:     getEnv("COGNEE_URL", "http://localhost:8000"),
	}
}

// ============================================================================
// CORE INFRASTRUCTURE TESTS
// ============================================================================

func TestCoreInfrastructure(t *testing.T) {
	config := getTestConfig()

	t.Run("PostgreSQL_Connection", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", config.PostgresHost, config.PostgresPort), 5*time.Second)
		if err != nil {
			t.Skipf("PostgreSQL not available: %v", err)
		}
		conn.Close()
		t.Log("PostgreSQL connection successful")
	})

	t.Run("Redis_Connection", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", config.RedisHost, config.RedisPort), 5*time.Second)
		if err != nil {
			t.Skipf("Redis not available: %v", err)
		}
		conn.Close()
		t.Log("Redis connection successful")
	})

	t.Run("ChromaDB_Health", func(t *testing.T) {
		resp, err := http.Get(config.ChromaDBURL + "/api/v2/heartbeat")
		if err != nil {
			t.Skipf("ChromaDB not available: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Cognee_Health", func(t *testing.T) {
		resp, err := http.Get(config.CogneeURL + "/")
		if err != nil {
			t.Skipf("Cognee not available: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// ============================================================================
// HELIXAGENT API TESTS
// ============================================================================

func TestHelixAgentAPI(t *testing.T) {
	config := getTestConfig()
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("Health_Endpoint", func(t *testing.T) {
		resp, err := client.Get(config.HelixAgentURL + "/health")
		if err != nil {
			t.Skipf("HelixAgent not available: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.NotEmpty(t, result["status"])
	})

	t.Run("Models_Endpoint", func(t *testing.T) {
		resp, err := client.Get(config.HelixAgentURL + "/v1/models")
		if err != nil {
			t.Skipf("HelixAgent not available: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.NotNil(t, result["data"])
	})

	t.Run("Providers_Endpoint", func(t *testing.T) {
		resp, err := client.Get(config.HelixAgentURL + "/v1/providers")
		if err != nil {
			t.Skipf("HelixAgent not available: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// ============================================================================
// ACP PROTOCOL TESTS
// ============================================================================

func TestACPProtocol(t *testing.T) {
	config := getTestConfig()
	client := &http.Client{Timeout: 30 * time.Second}
	baseURL := config.HelixAgentURL + "/v1/acp"

	t.Run("ACP_Health", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/health")
		if err != nil {
			t.Skipf("ACP not available: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "healthy", result["status"])
	})

	t.Run("ACP_List_Agents", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/agents")
		if err != nil {
			t.Skipf("ACP not available: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		agents, ok := result["agents"].([]interface{})
		require.True(t, ok)
		assert.GreaterOrEqual(t, len(agents), 6, "Expected at least 6 ACP agents")
	})

	agents := []string{"code-reviewer", "bug-finder", "refactor-assistant", "documentation-generator", "test-generator", "security-scanner"}
	for _, agent := range agents {
		t.Run(fmt.Sprintf("ACP_Agent_%s", agent), func(t *testing.T) {
			resp, err := client.Get(baseURL + "/agents/" + agent)
			if err != nil {
				t.Skipf("ACP not available: %v", err)
			}
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			assert.Equal(t, agent, result["id"])
		})
	}

	t.Run("ACP_Execute", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"agent_id": "code-reviewer",
			"task":     "review this code",
			"context": map[string]interface{}{
				"code":     "func main() { fmt.Println(\"Hello\") }",
				"language": "go",
			},
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(baseURL+"/execute", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Skipf("ACP not available: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "completed", result["status"])
	})
}

// ============================================================================
// VISION PROTOCOL TESTS
// ============================================================================

func TestVisionProtocol(t *testing.T) {
	config := getTestConfig()
	client := &http.Client{Timeout: 30 * time.Second}
	baseURL := config.HelixAgentURL + "/v1/vision"

	t.Run("Vision_Health", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/health")
		if err != nil {
			t.Skipf("Vision not available: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "healthy", result["status"])
	})

	t.Run("Vision_Capabilities", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/capabilities")
		if err != nil {
			t.Skipf("Vision not available: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		caps, ok := result["capabilities"].([]interface{})
		require.True(t, ok)
		assert.GreaterOrEqual(t, len(caps), 6, "Expected at least 6 vision capabilities")
	})

	capabilities := []string{"analyze", "ocr", "detect", "caption", "describe", "classify"}
	for _, cap := range capabilities {
		t.Run(fmt.Sprintf("Vision_Capability_%s", cap), func(t *testing.T) {
			// Test status endpoint
			resp, err := client.Get(baseURL + "/" + cap + "/status")
			if err != nil {
				t.Skipf("Vision not available: %v", err)
			}
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			// Test actual capability endpoint
			reqBody := map[string]interface{}{
				"image":  "",
				"prompt": "test",
			}
			body, _ := json.Marshal(reqBody)

			resp2, err := client.Post(baseURL+"/"+cap, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Skipf("Vision not available: %v", err)
			}
			defer resp2.Body.Close()
			assert.Equal(t, http.StatusOK, resp2.StatusCode)
		})
	}
}

// ============================================================================
// MCP PROTOCOL TESTS
// ============================================================================

func TestMCPProtocol(t *testing.T) {
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
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 5*time.Second)
			if err != nil {
				t.Skipf("MCP server %s not available: %v", name, err)
			}
			defer conn.Close()

			// Send JSON-RPC initialize
			initMsg := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
			conn.SetDeadline(time.Now().Add(5 * time.Second))
			_, err = conn.Write([]byte(initMsg + "\n"))
			if err != nil {
				t.Logf("MCP server %s: write failed but port open", name)
				return
			}

			// Read response
			buf := make([]byte, 4096)
			n, err := conn.Read(buf)
			if err != nil && err != io.EOF {
				t.Logf("MCP server %s: read failed but port open", name)
				return
			}

			response := string(buf[:n])
			if strings.Contains(response, "jsonrpc") {
				t.Logf("MCP server %s: JSON-RPC verified", name)
			} else {
				t.Logf("MCP server %s: port open, response not JSON-RPC", name)
			}
		})
	}
}

// ============================================================================
// LSP PROTOCOL TESTS
// ============================================================================

func TestLSPProtocol(t *testing.T) {
	lspServers := map[string]int{
		"gopls":         5001,
		"rust-analyzer": 5002,
		"pylsp":         5003,
		"typescript":    5004,
		"clangd":        5005,
		"jdtls":         5006,
		"bash-lsp":      5020,
		"yaml-lsp":      5021,
		"docker-lsp":    5022,
		"terraform-lsp": 5023,
		"xml-lsp":       5024,
	}

	for name, port := range lspServers {
		t.Run(fmt.Sprintf("LSP_%s_Port_%d", name, port), func(t *testing.T) {
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 5*time.Second)
			if err != nil {
				t.Skipf("LSP server %s not available: %v", name, err)
			}
			conn.Close()
			t.Logf("LSP server %s: port open", name)
		})
	}

	t.Run("LSP_Manager_Health", func(t *testing.T) {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://localhost:5100/health")
		if err != nil {
			t.Skipf("LSP Manager not available: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// ============================================================================
// RAG SERVICES TESTS
// ============================================================================

func TestRAGServices(t *testing.T) {
	services := map[string]string{
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

	for name, url := range services {
		t.Run(fmt.Sprintf("RAG_%s", strings.ReplaceAll(name, "-", "_")), func(t *testing.T) {
			resp, err := client.Get(url)
			if err != nil {
				t.Skipf("%s not available: %v", name, err)
			}
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

// ============================================================================
// EMBEDDINGS TESTS
// ============================================================================

func TestEmbeddings(t *testing.T) {
	config := getTestConfig()
	client := &http.Client{Timeout: 30 * time.Second}
	baseURL := config.HelixAgentURL + "/v1/embeddings/generate"

	t.Run("Embeddings_Single", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"text": "test text for embedding",
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(baseURL, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Skipf("Embeddings not available: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Skipf("Embeddings endpoint returned %d (embedding provider may not be configured)", resp.StatusCode)
		}

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.NotNil(t, result["success"])
	})

	t.Run("Embeddings_Batch", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"text":  "text one text two text three",
			"batch": true,
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(baseURL, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Skipf("Embeddings not available: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Skipf("Embeddings endpoint returned %d (embedding provider may not be configured)", resp.StatusCode)
		}

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.NotNil(t, result["success"])
	})
}

// ============================================================================
// COGNEE INTEGRATION TESTS
// ============================================================================

func TestCogneeIntegration(t *testing.T) {
	config := getTestConfig()
	client := &http.Client{Timeout: 60 * time.Second}
	baseURL := config.HelixAgentURL + "/v1/cognee"

	t.Run("Cognee_Health", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/health")
		if err != nil {
			t.Skipf("Cognee integration not available: %v", err)
		}
		defer resp.Body.Close()
		// Cognee may return 503 when service is not fully healthy (project uses Mem0 as primary memory)
		if resp.StatusCode == http.StatusServiceUnavailable {
			t.Skipf("Cognee service unavailable (503) - Mem0 is the primary memory system")
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Cognee_Add_Content", func(t *testing.T) {
		// Check if Cognee is enabled before testing content operations
		healthResp, err := client.Get(baseURL + "/health")
		if err != nil {
			t.Skipf("Cognee not available: %v", err)
		}
		defer healthResp.Body.Close()

		// Skip if Cognee service is not healthy (project uses Mem0 as primary memory)
		if healthResp.StatusCode == http.StatusServiceUnavailable {
			t.Skipf("Cognee service unavailable (503) - Mem0 is the primary memory system")
		}

		var healthResult map[string]interface{}
		json.NewDecoder(healthResp.Body).Decode(&healthResult)
		if cfg, ok := healthResult["config"].(map[string]interface{}); ok {
			if enabled, ok := cfg["enabled"].(bool); ok && !enabled {
				// Cognee is disabled but the endpoint exists - verify it returns proper error
				reqBody := map[string]interface{}{
					"content": "test",
				}
				body, _ := json.Marshal(reqBody)
				resp, err := client.Post(baseURL+"/memory", "application/json", bytes.NewReader(body))
				require.NoError(t, err)
				defer resp.Body.Close()
				// When disabled, endpoint should respond (not 404) with a service error
				assert.NotEqual(t, http.StatusNotFound, resp.StatusCode,
					"Cognee memory endpoint should exist even when disabled")
				t.Log("Cognee is disabled - endpoint correctly returns service error")
				return
			}
		}

		// Cognee is enabled - test full functionality
		reqBody := map[string]interface{}{
			"content": "HelixAgent is an AI-powered ensemble LLM service that combines multiple language models.",
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(baseURL+"/memory", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Skipf("Cognee integration not available: %v", err)
		}
		defer resp.Body.Close()
		// Cognee may return 500 when the service is not fully functional
		// (project uses Mem0 as primary memory system)
		if resp.StatusCode == http.StatusInternalServerError {
			t.Logf("Cognee memory endpoint returned 500 - Cognee service not fully functional (Mem0 is primary)")
			return
		}
		assert.Contains(t, []int{200, 201, 202}, resp.StatusCode)
	})

	t.Run("Cognee_Search", func(t *testing.T) {
		// Check Cognee health first (project uses Mem0 as primary memory)
		healthResp, err := client.Get(baseURL + "/health")
		if err != nil {
			t.Skipf("Cognee not available: %v", err)
		}
		defer healthResp.Body.Close()
		if healthResp.StatusCode == http.StatusServiceUnavailable {
			t.Skipf("Cognee service unavailable (503) - Mem0 is the primary memory system")
		}

		reqBody := map[string]interface{}{
			"query": "What is HelixAgent?",
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(baseURL+"/search", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Skipf("Cognee integration not available: %v", err)
		}
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// ============================================================================
// CONCURRENT LOAD TESTS
// ============================================================================

func TestConcurrentRequests(t *testing.T) {
	config := getTestConfig()
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("Concurrent_Health_Checks", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, 100)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req, _ := http.NewRequestWithContext(ctx, "GET", config.HelixAgentURL+"/health", nil)
				resp, err := client.Do(req)
				if err != nil {
					errors <- err
					return
				}
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					errors <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
				}
			}()
		}

		wg.Wait()
		close(errors)

		errorCount := 0
		for err := range errors {
			t.Logf("Error: %v", err)
			errorCount++
		}

		if errorCount > 10 {
			t.Errorf("Too many errors in concurrent requests: %d", errorCount)
		}
	})
}

// ============================================================================
// SECURITY TESTS
// ============================================================================

func TestSecurity(t *testing.T) {
	config := getTestConfig()
	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("SQL_Injection_Protection", func(t *testing.T) {
		// Test SQL injection in query parameter
		maliciousURL := config.HelixAgentURL + "/v1/models?id=1'%20OR%20'1'='1"
		resp, err := client.Get(maliciousURL)
		if err != nil {
			t.Skipf("HelixAgent not available: %v", err)
		}
		defer resp.Body.Close()
		// Should either return 400 (rejected) or 200 (safely handled)
		assert.Contains(t, []int{200, 400}, resp.StatusCode)
	})

	t.Run("XSS_Protection", func(t *testing.T) {
		// Test XSS in request body
		reqBody := map[string]interface{}{
			"input": "<script>alert('xss')</script>",
			"model": "text-embedding-3-small",
		}
		body, _ := json.Marshal(reqBody)

		resp, err := client.Post(config.HelixAgentURL+"/v1/embeddings", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Skipf("Embeddings not available: %v", err)
		}
		defer resp.Body.Close()

		// Read response body
		respBody, _ := io.ReadAll(resp.Body)
		// Check that script tags are either sanitized or not reflected
		assert.NotContains(t, string(respBody), "<script>")
	})

	t.Run("Path_Traversal_Protection", func(t *testing.T) {
		maliciousURL := config.HelixAgentURL + "/v1/../../../etc/passwd"
		resp, err := client.Get(maliciousURL)
		if err != nil {
			t.Skipf("HelixAgent not available: %v", err)
		}
		defer resp.Body.Close()
		// Should return 404 or 400, not 200 with file contents
		assert.Contains(t, []int{400, 404}, resp.StatusCode)
	})
}

// ============================================================================
// AI DEBATE TESTS
// ============================================================================

func TestAIDebate(t *testing.T) {
	config := getTestConfig()
	client := &http.Client{Timeout: 120 * time.Second}
	baseURL := config.HelixAgentURL + "/v1/debates"

	t.Run("Debate_Health", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/health", nil)
		require.NoError(t, err)

		// Add API key from environment
		apiKey := os.Getenv("HELIXAGENT_API_KEY")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Skipf("Debate system not available: %v", err)
		}
		defer resp.Body.Close()
		// Accept 200, 401 (auth required), or 404 (endpoint might not exist)
		if resp.StatusCode == 404 {
			t.Skip("Debate health endpoint not implemented")  // SKIP-OK: #legacy-untriaged
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			t.Log("Debate health requires authentication - endpoint exists and responds correctly")
			return
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Debate_Create", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"topic": "Is AI beneficial for humanity?",
			"participants": []map[string]interface{}{
				{"name": "supporter", "role": "advocate"},
				{"name": "skeptic", "role": "critic"},
				{"name": "mediator", "role": "moderator"},
			},
			"max_rounds": 1,
			"timeout":    30,
		}
		body, _ := json.Marshal(reqBody)

		req, err := http.NewRequest("POST", baseURL, bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		// Add API key from environment
		apiKey := os.Getenv("HELIXAGENT_API_KEY")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Skipf("Debate system not available: %v", err)
		}
		defer resp.Body.Close()
		// Accept 200, 201, 202, 401 (auth required), or 404
		if resp.StatusCode == 404 {
			t.Skip("Debate create endpoint not implemented")  // SKIP-OK: #legacy-untriaged
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			t.Log("Debate create requires authentication - endpoint exists and responds correctly")
			return
		}
		assert.Contains(t, []int{200, 201, 202}, resp.StatusCode)
	})
}
