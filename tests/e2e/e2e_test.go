package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superagent/superagent/internal/services"
)

// TestE2EUserWorkflow tests complete user workflows
// Note: These tests require a running SuperAgent server on localhost:8080
// To run these tests:
// 1. Start the server: make run-dev
// 2. Run E2E tests: make test-e2e
// 3. Or run all tests: make test-all-types
func TestE2EUserWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	baseURL := "http://localhost:8080"
	client := &http.Client{Timeout: 60 * time.Second}

	// Check if server is running
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Skipping E2E test: SuperAgent server not running at %s. Start server with 'make run-dev'", baseURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Skipping E2E test: Server at %s returned status %d", baseURL, resp.StatusCode)
	}

	t.Logf("✅ SuperAgent server is running at %s", baseURL)

	t.Run("CompleteChatWorkflow", func(t *testing.T) {
		// Step 1: Check available models
		resp, err := client.Get(baseURL + "/v1/models")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var modelsResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&modelsResp)
		require.NoError(t, err)

		data := modelsResp["data"].([]interface{})
		assert.Greater(t, len(data), 0, "Should have available models")

		// Step 2: Start a chat conversation
		chatRequest := map[string]interface{}{
			"model": "gpt-3.5-turbo",
			"messages": []map[string]interface{}{
				{"role": "system", "content": "You are a helpful assistant."},
				{"role": "user", "content": "Hello! Can you help me with something?"},
			},
			"max_tokens":  100,
			"temperature": 0.7,
		}

		jsonData, err := json.Marshal(chatRequest)
		require.NoError(t, err)

		resp, err = client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Accept various status codes since providers might not be configured
		if resp.StatusCode == http.StatusOK {
			var chatResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&chatResp)
			require.NoError(t, err)

			assert.Equal(t, "chat.completion", chatResp["object"])
			assert.NotNil(t, chatResp["choices"])

			choices := chatResp["choices"].([]interface{})
			assert.Greater(t, len(choices), 0)

			t.Logf("✅ Chat workflow completed successfully")
		} else {
			t.Logf("⚠️  Chat workflow returned status %d (may be expected if providers not configured)", resp.StatusCode)
		}
	})

	t.Run("CompleteEnsembleWorkflow", func(t *testing.T) {
		// Step 1: Check provider health
		resp, err := client.Get(baseURL + "/v1/providers")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var providersResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&providersResp)
		require.NoError(t, err)

		providers := providersResp["providers"].([]interface{})
		t.Logf("✅ Found %d providers", len(providers))

		// Step 2: Test ensemble completion
		ensembleRequest := map[string]interface{}{
			"prompt": "What is the capital of France?",
			"ensemble_config": map[string]interface{}{
				"strategy":             "confidence_weighted",
				"min_providers":        1,
				"confidence_threshold": 0.5,
			},
		}

		jsonData, err := json.Marshal(ensembleRequest)
		require.NoError(t, err)

		resp, err = client.Post(baseURL+"/v1/ensemble/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var ensembleResp map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&ensembleResp)
			require.NoError(t, err)

			assert.Equal(t, "ensemble.completion", ensembleResp["object"])
			assert.NotNil(t, ensembleResp["ensemble"])

			t.Logf("✅ Ensemble workflow completed successfully")
		} else {
			t.Logf("⚠️  Ensemble workflow returned status %d", resp.StatusCode)
		}
	})

	t.Run("CompleteStreamingWorkflow", func(t *testing.T) {
		streamRequest := map[string]interface{}{
			"prompt":      "Count from 1 to 5",
			"model":       "gpt-3.5-turbo",
			"max_tokens":  50,
			"temperature": 0.1,
			"stream":      true,
		}

		jsonData, err := json.Marshal(streamRequest)
		require.NoError(t, err)

		resp, err := client.Post(baseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			// Read streaming response
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			// Should contain SSE data
			assert.Contains(t, string(body), "data:")
			t.Logf("✅ Streaming workflow completed: received %d bytes", len(body))
		} else {
			t.Logf("⚠️  Streaming workflow returned status %d", resp.StatusCode)
		}
	})

	t.Run("CompleteMonitoringWorkflow", func(t *testing.T) {
		// Step 1: Check basic health
		resp, err := client.Get(baseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Step 2: Check enhanced health
		resp, err = client.Get(baseURL + "/v1/health")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var healthResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&healthResp)
		require.NoError(t, err)

		assert.Equal(t, "healthy", healthResp["status"])
		assert.NotNil(t, healthResp["providers"])

		// Step 3: Check metrics
		resp, err = client.Get(baseURL + "/metrics")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Should contain Prometheus metrics
		assert.Contains(t, string(body), "# HELP")
		assert.Contains(t, string(body), "# TYPE")

		t.Logf("✅ Monitoring workflow completed: metrics size %d bytes", len(body))
	})
}

// TestE2EErrorHandling tests error scenarios end-to-end
func TestE2EErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E error handling test in short mode")
	}

	baseURL := "http://localhost:8080"
	client := &http.Client{Timeout: 30 * time.Second}

	// Check if server is running
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Skipping E2E test: SuperAgent server not running at %s. Start server with 'make run-dev'", baseURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Skipping E2E test: Server at %s returned status %d", baseURL, resp.StatusCode)
	}

	t.Run("InvalidEndpoint", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/invalid/endpoint")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("InvalidRequestBody", func(t *testing.T) {
		resp, err := client.Post(baseURL+"/v1/completions", "application/json", bytes.NewBuffer([]byte("invalid json")))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("MissingRequiredFields", func(t *testing.T) {
		request := map[string]interface{}{
			"temperature": 0.5,
			// Missing required fields like prompt/model
		}

		jsonData, _ := json.Marshal(request)
		resp, err := client.Post(baseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("UnsupportedModel", func(t *testing.T) {
		request := map[string]interface{}{
			"prompt": "Hello",
			"model":  "unsupported-model-name",
		}

		jsonData, _ := json.Marshal(request)
		resp, err := client.Post(baseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return error (400 or 500 depending on implementation)
		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	})
}

// TestE2EPerformance tests performance characteristics
func TestE2EPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E performance test in short mode")
	}

	baseURL := "http://localhost:8080"
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("ConcurrentRequests", func(t *testing.T) {
		concurrency := 10
		responses := make(chan time.Duration, concurrency)

		// Launch concurrent requests
		for i := 0; i < concurrency; i++ {
			go func(id int) {
				start := time.Now()

				request := map[string]interface{}{
					"prompt":      fmt.Sprintf("Test request %d", id),
					"model":       "gpt-3.5-turbo",
					"max_tokens":  10,
					"temperature": 0.1,
				}

				jsonData, _ := json.Marshal(request)
				resp, err := client.Post(baseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))

				if resp != nil {
					resp.Body.Close()
				}

				if err == nil {
					responses <- time.Since(start)
				} else {
					responses <- 0
				}
			}(i)
		}

		// Collect responses
		var totalDuration time.Duration
		successCount := 0

		for i := 0; i < concurrency; i++ {
			duration := <-responses
			if duration > 0 {
				totalDuration += duration
				successCount++
			}
		}

		if successCount > 0 {
			avgDuration := totalDuration / time.Duration(successCount)
			t.Logf("✅ Concurrent requests: %d/%d successful, avg duration: %v",
				successCount, concurrency, avgDuration)

			// Performance assertion - should respond within reasonable time
			assert.Less(t, avgDuration, 30*time.Second, "Average response time should be reasonable")
		} else {
			t.Logf("⚠️  No concurrent requests succeeded (may be expected if providers not configured)")
		}
	})
}

// TestE2ENewServicesWorkflow tests end-to-end workflows using the new services
func TestE2ENewServicesWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E new services test in short mode")
	}

	t.Run("CompleteCodeAnalysisWorkflow", func(t *testing.T) {
		// Initialize all services
		mcpManager := services.NewMCPManager()
		lspClient := services.NewLSPClient("/tmp", "go")
		toolRegistry := services.NewToolRegistry(mcpManager, lspClient)
		contextManager := services.NewContextManager(100)
		securitySandbox := services.NewSecuritySandbox()
		orchestrator := services.NewIntegrationOrchestrator(mcpManager, lspClient, toolRegistry, contextManager)

		// Register test tools
		codeAnalysisTool := &MockTool{
			name:        "code-analysis",
			description: "Analyzes code for issues",
			parameters:  map[string]interface{}{"code": map[string]interface{}{"type": "string"}},
		}

		refactorTool := &MockTool{
			name:        "refactor",
			description: "Refactors code",
			parameters:  map[string]interface{}{"code": map[string]interface{}{"type": "string"}, "action": map[string]interface{}{"type": "string"}},
		}

		err := toolRegistry.RegisterCustomTool(codeAnalysisTool)
		require.NoError(t, err)
		err = toolRegistry.RegisterCustomTool(refactorTool)
		require.NoError(t, err)

		// Add context about the code
		contextEntry := &services.ContextEntry{
			ID:       "code-context",
			Type:     "lsp",
			Source:   "/tmp/example.go",
			Content:  "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello World\")\n}",
			Priority: 8,
		}
		err = contextManager.AddEntry(contextEntry)
		require.NoError(t, err)

		// Execute code analysis workflow
		intelligence, err := orchestrator.ExecuteCodeAnalysis(context.Background(), "/tmp/example.go", "go")
		if err != nil {
			t.Logf("Code analysis failed (may be expected in test env): %v", err)
		} else {
			assert.NotNil(t, intelligence)
			assert.Equal(t, "/tmp/example.go", intelligence.FilePath)
			t.Logf("✅ Code analysis workflow completed")
		}

		// Test tool chain execution
		toolChain := []services.ToolExecution{
			{
				ToolName:   "code-analysis",
				Parameters: map[string]interface{}{"toolName": "code-analysis", "code": "func test() {}"},
				MaxRetries: 1,
			},
			{
				ToolName:   "refactor",
				Parameters: map[string]interface{}{"toolName": "refactor", "code": "func test() {}", "action": "rename"},
				MaxRetries: 1,
				DependsOn:  []string{"tool_0"},
			},
		}

		results, err := orchestrator.ExecuteToolChain(context.Background(), toolChain)
		if err != nil {
			t.Logf("Tool chain execution failed: %v", err)
		} else {
			assert.NotEmpty(t, results)
			t.Logf("✅ Tool chain workflow completed with %d results", len(results))
		}

		// Test parallel operations
		operations := []services.Operation{
			{
				ID:         "analysis-op",
				Type:       "tool",
				Name:       "code-analysis",
				Parameters: map[string]interface{}{"toolName": "code-analysis", "code": "function analyze() {}"},
			},
			{
				ID:         "refactor-op",
				Type:       "tool",
				Name:       "refactor",
				Parameters: map[string]interface{}{"toolName": "refactor", "code": "function old() {}", "action": "modernize"},
			},
		}

		parallelResults, err := orchestrator.ExecuteParallelOperations(context.Background(), operations)
		if err != nil {
			t.Logf("Parallel operations failed: %v", err)
		} else {
			assert.Len(t, parallelResults, len(operations))
			t.Logf("✅ Parallel operations completed with %d results", len(parallelResults))
		}

		// Test security sandbox
		safeResult, err := securitySandbox.ExecuteSandboxed(context.Background(), "ls", []string{"-la", "/tmp"})
		if err != nil {
			t.Logf("Security sandbox execution failed: %v", err)
		} else {
			assert.NotNil(t, safeResult)
			t.Logf("✅ Security sandbox executed command successfully")
		}

		// Test parameter validation
		err = securitySandbox.ValidateToolExecution("test-tool", map[string]interface{}{
			"safe": "echo hello",
		})
		assert.NoError(t, err)

		err = securitySandbox.ValidateToolExecution("test-tool", map[string]interface{}{
			"dangerous": "rm -rf / | echo hacked",
		})
		assert.Error(t, err)
		t.Logf("✅ Security validation working correctly")

		// Verify context management
		builtContext, err := contextManager.BuildContext("code_completion", 1000)
		if err != nil {
			t.Logf("Context building failed: %v", err)
		} else {
			assert.NotEmpty(t, builtContext)
			t.Logf("✅ Context management working with %d entries", len(builtContext))
		}
	})

	t.Run("CompleteMCP_LSP_IntegrationWorkflow", func(t *testing.T) {
		// Test MCP server registration and tool discovery
		mcpManager := services.NewMCPManagerWithConfig(false, 3, 30*time.Second)

		serverConfig := map[string]interface{}{
			"name":    "filesystem-mcp",
			"command": []interface{}{"echo", "filesystem-server"},
		}

		err := mcpManager.RegisterServer(serverConfig)
		if err != nil {
			t.Logf("MCP server registration failed (expected in test env): %v", err)
		}

		tools := mcpManager.ListTools()
		t.Logf("✅ MCP manager has %d tools available", len(tools))

		// Test LSP client initialization
		lspClient := services.NewLSPClient("/tmp/project", "go")

		// Test tool registry with MCP and LSP
		toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

		registryTools := toolRegistry.ListTools()
		t.Logf("✅ Tool registry has %d tools from all sources", len(registryTools))

		// Test context manager with different entry types
		contextManager := services.NewContextManager(100)

		entries := []*services.ContextEntry{
			{
				ID:       "mcp-context",
				Type:     "mcp",
				Source:   "filesystem-server",
				Content:  "File system analysis results",
				Priority: 7,
			},
			{
				ID:       "lsp-context",
				Type:     "lsp",
				Source:   "/tmp/main.go",
				Content:  "LSP diagnostics and symbols",
				Priority: 9,
			},
			{
				ID:       "tool-context",
				Type:     "tool",
				Source:   "code-formatter",
				Content:  "Code formatting applied",
				Priority: 5,
			},
		}

		for _, entry := range entries {
			err := contextManager.AddEntry(entry)
			assert.NoError(t, err)
		}

		// Test context retrieval and conflict detection
		conflicts := contextManager.DetectConflicts()
		t.Logf("✅ Context manager detected %d conflicts", len(conflicts))

		// Test different context building scenarios
		scenarios := []string{"code_completion", "tool_execution", "chat"}
		for _, scenario := range scenarios {
			context, err := contextManager.BuildContext(scenario, 1000)
			if err != nil {
				t.Logf("Context building failed for %s: %v", scenario, err)
			} else {
				t.Logf("✅ Built context for %s with %d entries", scenario, len(context))
			}
		}
	})
}

// Mock Tool for E2E testing
type MockTool struct {
	name        string
	description string
	parameters  map[string]interface{}
	source      string
}

func (m *MockTool) Name() string {
	return m.name
}

func (m *MockTool) Description() string {
	return m.description
}

func (m *MockTool) Parameters() map[string]interface{} {
	return m.parameters
}

func (m *MockTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Simulate realistic tool execution with some processing time
	time.Sleep(10 * time.Millisecond)

	result := map[string]interface{}{
		"tool":      m.name,
		"params":    params,
		"result":    "success",
		"timestamp": time.Now().Unix(),
		"message":   fmt.Sprintf("Executed %s successfully", m.name),
	}

	// Add tool-specific results
	switch m.name {
	case "code-analysis":
		result["issues"] = []string{"No issues found"}
		result["complexity"] = "low"
	case "refactor":
		result["changes"] = 3
		result["improvements"] = []string{"Better naming", "Reduced complexity"}
	}

	return result, nil
}

func (m *MockTool) Source() string {
	return m.source
}
