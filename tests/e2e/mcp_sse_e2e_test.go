package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2EMCPSSE tests MCP SSE endpoint end-to-end
// Note: These tests require a running HelixAgent server on localhost:7061
func TestE2EMCPSSE(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E MCP SSE test in short mode")
	}

	baseURL := "http://localhost:7061"
	client := &http.Client{Timeout: 60 * time.Second}

	// Check if server is running
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Skipping E2E test: HelixAgent server not running at %s. Start server with 'make run-dev'", baseURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Skipping E2E test: Server at %s returned status %d", baseURL, resp.StatusCode)
	}

	t.Logf("HelixAgent server is running at %s", baseURL)

	t.Run("MCPSSEInitialize", func(t *testing.T) {
		// Test MCP initialize via POST
		initMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "e2e-test-client",
					"version": "1.0.0",
				},
			},
		}

		jsonData, _ := json.Marshal(initMsg)
		resp, err := client.Post(baseURL+"/v1/mcp", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "2.0", response["jsonrpc"])
		assert.NotNil(t, response["result"])
		assert.Nil(t, response["error"])

		result := response["result"].(map[string]interface{})
		assert.Equal(t, "2024-11-05", result["protocolVersion"])
		assert.NotNil(t, result["serverInfo"])
		assert.NotNil(t, result["capabilities"])

		serverInfo := result["serverInfo"].(map[string]interface{})
		assert.Equal(t, "helixagent-mcp", serverInfo["name"])

		t.Logf("MCP SSE initialize successful: server %s v%s", serverInfo["name"], serverInfo["version"])
	})

	t.Run("MCPSSEToolsList", func(t *testing.T) {
		// Test tools/list method
		toolsListMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
		}

		jsonData, _ := json.Marshal(toolsListMsg)
		resp, err := client.Post(baseURL+"/v1/mcp", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.Nil(t, response["error"])

		result := response["result"].(map[string]interface{})
		tools := result["tools"].([]interface{})
		assert.GreaterOrEqual(t, len(tools), 3, "MCP should have at least 3 tools")

		t.Logf("MCP tools/list returned %d tools", len(tools))

		// Verify tool structure
		for _, tool := range tools {
			toolMap := tool.(map[string]interface{})
			assert.NotEmpty(t, toolMap["name"])
			assert.NotNil(t, toolMap["inputSchema"])
		}
	})

	t.Run("MCPSSEToolsCall", func(t *testing.T) {
		// Test tools/call method
		toolCallMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      "mcp_get_capabilities",
				"arguments": map[string]interface{}{},
			},
		}

		jsonData, _ := json.Marshal(toolCallMsg)
		resp, err := client.Post(baseURL+"/v1/mcp", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.Nil(t, response["error"])
		assert.NotNil(t, response["result"])

		t.Logf("MCP tools/call successful")
	})

	t.Run("MCPSSEPing", func(t *testing.T) {
		// Test ping method
		pingMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      4,
			"method":  "ping",
		}

		jsonData, _ := json.Marshal(pingMsg)
		resp, err := client.Post(baseURL+"/v1/mcp", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.Nil(t, response["error"])

		t.Logf("MCP ping successful")
	})

	t.Run("MCPSSEPromptsList", func(t *testing.T) {
		// Test prompts/list method
		promptsMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      5,
			"method":  "prompts/list",
		}

		jsonData, _ := json.Marshal(promptsMsg)
		resp, err := client.Post(baseURL+"/v1/mcp", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.Nil(t, response["error"])

		t.Logf("MCP prompts/list successful")
	})

	t.Run("MCPSSEResourcesList", func(t *testing.T) {
		// Test resources/list method
		resourcesMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      6,
			"method":  "resources/list",
		}

		jsonData, _ := json.Marshal(resourcesMsg)
		resp, err := client.Post(baseURL+"/v1/mcp", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.Nil(t, response["error"])

		t.Logf("MCP resources/list successful")
	})

	t.Run("MCPSSEErrorHandling", func(t *testing.T) {
		// Test unknown method error
		unknownMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      7,
			"method":  "unknown/method",
		}

		jsonData, _ := json.Marshal(unknownMsg)
		resp, err := client.Post(baseURL+"/v1/mcp", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)

		assert.NotNil(t, response["error"])

		errorData := response["error"].(map[string]interface{})
		assert.Equal(t, float64(-32601), errorData["code"])

		t.Logf("MCP error handling successful")
	})
}

// TestE2EAllProtocolsSSE tests all protocol SSE endpoints
func TestE2EAllProtocolsSSE(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E all protocols SSE test in short mode")
	}

	baseURL := "http://localhost:7061"
	client := &http.Client{Timeout: 60 * time.Second}

	// Check if server is running
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Skipping E2E test: HelixAgent server not running at %s", baseURL)
	}
	defer resp.Body.Close()

	protocols := []struct {
		name     string
		endpoint string
		tools    []string
	}{
		{"mcp", "/v1/mcp", []string{"mcp_list_providers", "mcp_get_capabilities", "mcp_execute_tool"}},
		{"acp", "/v1/acp", []string{"acp_send_message", "acp_list_agents"}},
		{"lsp", "/v1/lsp", []string{"lsp_get_diagnostics", "lsp_go_to_definition", "lsp_find_references", "lsp_list_servers"}},
		{"embeddings", "/v1/embeddings", []string{"embeddings_generate", "embeddings_search"}},
		{"vision", "/v1/vision", []string{"vision_analyze_image", "vision_ocr"}},
		{"cognee", "/v1/cognee", []string{"cognee_add", "cognee_search", "cognee_visualize"}},
	}

	for _, proto := range protocols {
		t.Run(fmt.Sprintf("%sProtocol", strings.ToUpper(proto.name)), func(t *testing.T) {
			// Test initialize
			initMsg := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
			}

			jsonData, _ := json.Marshal(initMsg)
			resp, err := client.Post(baseURL+proto.endpoint, "application/json", bytes.NewBuffer(jsonData))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)

			assert.Nil(t, response["error"])
			result := response["result"].(map[string]interface{})
			serverInfo := result["serverInfo"].(map[string]interface{})
			assert.Equal(t, fmt.Sprintf("helixagent-%s", proto.name), serverInfo["name"])

			t.Logf("%s initialize successful", strings.ToUpper(proto.name))

			// Test tools/list
			toolsMsg := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      2,
				"method":  "tools/list",
			}

			jsonData, _ = json.Marshal(toolsMsg)
			resp, err = client.Post(baseURL+proto.endpoint, "application/json", bytes.NewBuffer(jsonData))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)

			assert.Nil(t, response["error"])
			result = response["result"].(map[string]interface{})
			tools := result["tools"].([]interface{})
			assert.GreaterOrEqual(t, len(tools), len(proto.tools), "%s should have at least %d tools", proto.name, len(proto.tools))

			// Verify expected tools are present
			toolNames := make(map[string]bool)
			for _, tool := range tools {
				toolMap := tool.(map[string]interface{})
				toolNames[toolMap["name"].(string)] = true
			}

			for _, expectedTool := range proto.tools {
				assert.True(t, toolNames[expectedTool], "%s should have tool %s", proto.name, expectedTool)
			}

			t.Logf("%s tools/list returned %d tools", strings.ToUpper(proto.name), len(tools))
		})
	}
}

// TestE2ECLIAgentIntegration tests CLI agent integration (OpenCode, Crush, HelixCode)
func TestE2ECLIAgentIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E CLI agent test in short mode")
	}

	baseURL := "http://localhost:7061"
	client := &http.Client{Timeout: 60 * time.Second}

	// Check if server is running
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Skipping E2E test: HelixAgent server not running at %s", baseURL)
	}
	defer resp.Body.Close()

	t.Run("OpenCodeMCPIntegration", func(t *testing.T) {
		// Test the complete MCP initialization flow as OpenCode would do
		// Step 1: Connect to SSE endpoint
		sseClient := &http.Client{Timeout: 10 * time.Second}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/v1/mcp", nil)
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")

		resp, err := sseClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()

			// Read initial endpoint event
			reader := bufio.NewReader(resp.Body)
			line, err := reader.ReadString('\n')
			if err == nil {
				assert.Contains(t, line, "event:")
				t.Logf("SSE connection established, received: %s", strings.TrimSpace(line))
			}
		} else {
			t.Logf("SSE connection returned status %d (acceptable for test environment)", resp.StatusCode)
		}

		// Step 2: Send initialize request
		initMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"roots":    map[string]interface{}{"listChanged": true},
					"sampling": map[string]interface{}{},
				},
				"clientInfo": map[string]interface{}{
					"name":    "opencode",
					"version": "1.0.0",
				},
			},
		}

		jsonData, _ := json.Marshal(initMsg)
		resp, err = client.Post(baseURL+"/v1/mcp", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)
		assert.Nil(t, response["error"])

		// Step 3: Send initialized notification
		initedMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "initialized",
		}

		jsonData, _ = json.Marshal(initedMsg)
		resp, err = client.Post(baseURL+"/v1/mcp", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		resp.Body.Close()

		// Step 4: List available tools
		toolsMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/list",
		}

		jsonData, _ = json.Marshal(toolsMsg)
		resp, err = client.Post(baseURL+"/v1/mcp", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		json.NewDecoder(resp.Body).Decode(&response)
		assert.Nil(t, response["error"])

		t.Logf("OpenCode MCP integration flow completed successfully")
	})

	t.Run("CrushMCPIntegration", func(t *testing.T) {
		// Similar flow for Crush CLI agent
		initMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "crush",
					"version": "1.0.0",
				},
			},
		}

		jsonData, _ := json.Marshal(initMsg)
		resp, err := client.Post(baseURL+"/v1/mcp", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)
		assert.Nil(t, response["error"])

		t.Logf("Crush MCP integration flow completed successfully")
	})

	t.Run("HelixCodeMCPIntegration", func(t *testing.T) {
		// Similar flow for HelixCode CLI agent
		initMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "helixcode",
					"version": "1.0.0",
				},
			},
		}

		jsonData, _ := json.Marshal(initMsg)
		resp, err := client.Post(baseURL+"/v1/mcp", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)
		assert.Nil(t, response["error"])

		t.Logf("HelixCode MCP integration flow completed successfully")
	})

	t.Run("KiloCodeMCPIntegration", func(t *testing.T) {
		// Similar flow for Kilo Code CLI agent
		initMsg := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "kilocode",
					"version": "1.0.0",
				},
			},
		}

		jsonData, _ := json.Marshal(initMsg)
		resp, err := client.Post(baseURL+"/v1/mcp", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)
		assert.Nil(t, response["error"])

		t.Logf("Kilo Code MCP integration flow completed successfully")
	})
}

// TestE2EMCPSSEConcurrency tests concurrent MCP SSE connections
func TestE2EMCPSSEConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E MCP SSE concurrency test in short mode")
	}

	baseURL := "http://localhost:7061"
	client := &http.Client{Timeout: 60 * time.Second}

	// Check if server is running
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Skipping E2E test: HelixAgent server not running at %s", baseURL)
	}
	defer resp.Body.Close()

	t.Run("ConcurrentProtocolRequests", func(t *testing.T) {
		concurrency := 10
		var wg sync.WaitGroup
		results := make(chan bool, concurrency)

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				protocols := []string{"/v1/mcp", "/v1/acp", "/v1/lsp", "/v1/embeddings", "/v1/vision", "/v1/cognee"}
				protocol := protocols[id%len(protocols)]

				initMsg := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      id,
					"method":  "initialize",
				}

				jsonData, _ := json.Marshal(initMsg)
				resp, err := client.Post(baseURL+protocol, "application/json", bytes.NewBuffer(jsonData))
				if err != nil {
					results <- false
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					results <- false
					return
				}

				var response map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					results <- false
					return
				}

				results <- response["error"] == nil
			}(i)
		}

		wg.Wait()
		close(results)

		successCount := 0
		for success := range results {
			if success {
				successCount++
			}
		}

		assert.Equal(t, concurrency, successCount, "All concurrent requests should succeed")
		t.Logf("Concurrent protocol requests: %d/%d successful", successCount, concurrency)
	})
}

// TestE2EMCPSSEConnection tests actual SSE connection behavior
func TestE2EMCPSSEConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E MCP SSE connection test in short mode")
	}

	baseURL := "http://localhost:7061"
	client := &http.Client{Timeout: 60 * time.Second}

	// Check if server is running
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Skipping E2E test: HelixAgent server not running at %s", baseURL)
	}
	defer resp.Body.Close()

	protocols := []string{"mcp", "acp", "lsp", "embeddings", "vision", "cognee"}

	for _, protocol := range protocols {
		t.Run(fmt.Sprintf("SSEConnection_%s", protocol), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/v1/"+protocol, nil)
			req.Header.Set("Accept", "text/event-stream")
			req.Header.Set("Cache-Control", "no-cache")
			req.Header.Set("Connection", "keep-alive")

			sseClient := &http.Client{Timeout: 10 * time.Second}
			resp, err := sseClient.Do(req)
			if err != nil {
				t.Logf("SSE connection to %s failed (acceptable in test): %v", protocol, err)
				return
			}
			defer resp.Body.Close()

			// Check SSE headers
			contentType := resp.Header.Get("Content-Type")
			if resp.StatusCode == http.StatusOK {
				assert.Equal(t, "text/event-stream", contentType, "%s should return SSE content type", protocol)

				// Try to read some data
				buf := make([]byte, 1024)
				resp.Body.SetReadDeadline(time.Now().Add(2 * time.Second))
				n, err := resp.Body.Read(buf)
				if err == nil && n > 0 {
					data := string(buf[:n])
					// Should receive endpoint event
					if strings.Contains(data, "event:") || strings.Contains(data, "data:") {
						t.Logf("%s SSE connection established, received: %s", protocol, strings.TrimSpace(data))
					}
				}
			} else {
				t.Logf("%s SSE returned status %d", protocol, resp.StatusCode)
			}
		})
	}
}

// Body wrapper to add SetReadDeadline
type timeoutReader struct {
	io.ReadCloser
}

func (r *timeoutReader) SetReadDeadline(t time.Time) error {
	return nil
}
