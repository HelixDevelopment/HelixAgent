// Previously tests/automation/full_automation_test.go. CONST-030 forbids
// in-process mocks in non-unit tests; this 1,639-LOC harness simulated
// the entire HelixAgent CI pipeline (unit/integration/E2E phases) by
// spawning mock LLM servers via httptest and exercising a local
// MockTool + gin handlers — zero contact with a live :7061 HelixAgent.
// Moved to the unit tree in PR29 of the CONST-030 compliance campaign.
package automation_legacy_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/config"
	"dev.helix.agent/internal/handlers"
	"dev.helix.agent/internal/services"
	"digital.vasic.toolschema"

	"go.uber.org/goleak"
)

// =============================================================================
// CONFIGURATION
// =============================================================================

// AutomationConfig holds configuration for automation tests
type AutomationConfig struct {
	BaseURL           string
	MockLLMURL        string
	Timeout           time.Duration
	Concurrency       int
	CoverageThreshold float64
}

// AutomationResult holds results from automation tests
type AutomationResult struct {
	Phase    string
	TestName string
	Success  bool
	Duration time.Duration
	Message  string
	Details  map[string]interface{}
}

// AutomationSuite holds the test suite state
type AutomationSuite struct {
	config        *AutomationConfig
	mockLLMServer *httptest.Server
	testRouter    *gin.Engine
	testServer    *httptest.Server
	results       []AutomationResult
	mu            sync.Mutex
	started       time.Time
	logger        *logrus.Logger
}

// Global test suite instance
var suite *AutomationSuite

// =============================================================================
// TEST SUITE SETUP AND TEARDOWN
// =============================================================================

// TestMain sets up and tears down the test suite
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	// Initialize suite
	suite = &AutomationSuite{
		config: &AutomationConfig{
			BaseURL:           "http://localhost:7061",
			MockLLMURL:        "http://localhost:18081",
			Timeout:           60 * time.Second,
			Concurrency:       50,
			CoverageThreshold: 60.0, // Minimum 60% coverage
		},
		results: make([]AutomationResult, 0),
		started: time.Now(),
		logger:  logrus.New(),
	}
	suite.logger.SetLevel(logrus.InfoLevel)

	// goleak.VerifyTestMain runs m.Run() internally, then checks for goroutine leaks.
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("google.golang.org/grpc/internal/transport.(*http2Client).keepalive"),
	)

	// Print summary (runs after VerifyTestMain returns)
	suite.printSummary()
}

// addResult adds a test result to the suite
func (s *AutomationSuite) addResult(result AutomationResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = append(s.results, result)
}

// printSummary prints the test suite summary
func (s *AutomationSuite) printSummary() {
	s.mu.Lock()
	defer s.mu.Unlock()

	totalDuration := time.Since(s.started)
	passed := 0
	failed := 0

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("AUTOMATION TEST SUITE SUMMARY")
	fmt.Println(strings.Repeat("=", 80))

	// Group by phase
	phases := make(map[string][]AutomationResult)
	for _, r := range s.results {
		phases[r.Phase] = append(phases[r.Phase], r)
	}

	for phase, results := range phases {
		fmt.Printf("\n[%s]\n", phase)
		for _, r := range results {
			status := "PASS"
			if !r.Success {
				status = "FAIL"
				failed++
			} else {
				passed++
			}
			fmt.Printf("  %-50s %s (%v)\n", r.TestName, status, r.Duration.Round(time.Millisecond))
			if r.Message != "" && !r.Success {
				fmt.Printf("    -> %s\n", r.Message)
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("-", 80))
	fmt.Printf("Total: %d passed, %d failed, %d total\n", passed, failed, passed+failed)
	fmt.Printf("Duration: %v\n", totalDuration.Round(time.Second))
	fmt.Println(strings.Repeat("=", 80))
}

// =============================================================================
// PHASE 1: SETUP PHASE
// =============================================================================

func TestPhase1_Setup(t *testing.T) {
	t.Run("SetupMockLLMServer", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "1-Setup",
			TestName: "Setup Mock LLM Server",
			Details:  make(map[string]interface{}),
		}

		// Create mock LLM server
		suite.mockLLMServer = createMockLLMServer()
		require.NotNil(t, suite.mockLLMServer, "Mock LLM server should be created")

		result.Success = true
		result.Duration = time.Since(start)
		result.Details["url"] = suite.mockLLMServer.URL
		suite.addResult(result)

		t.Logf("Mock LLM server started at %s", suite.mockLLMServer.URL)
	})

	t.Run("SetupTestRouter", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "1-Setup",
			TestName: "Setup Test Router",
			Details:  make(map[string]interface{}),
		}

		// Create test router with handlers
		suite.testRouter = createTestRouter()
		require.NotNil(t, suite.testRouter, "Test router should be created")

		// Create test server
		suite.testServer = httptest.NewServer(suite.testRouter)
		require.NotNil(t, suite.testServer, "Test server should be created")

		result.Success = true
		result.Duration = time.Since(start)
		result.Details["url"] = suite.testServer.URL
		suite.addResult(result)

		t.Logf("Test server started at %s", suite.testServer.URL)
	})

	t.Run("VerifyEnvironment", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "1-Setup",
			TestName: "Verify Environment",
			Details:  make(map[string]interface{}),
		}

		// Verify Go version
		goVersion := runtime.Version()
		result.Details["go_version"] = goVersion

		// Verify required environment variables are accessible
		envVars := []string{"HOME", "PATH"}
		for _, env := range envVars {
			if os.Getenv(env) == "" {
				result.Message = fmt.Sprintf("Environment variable %s not set", env)
			}
		}

		result.Success = result.Message == ""
		result.Duration = time.Since(start)
		suite.addResult(result)

		t.Logf("Environment verified: Go %s", goVersion)
	})
}

// =============================================================================
// PHASE 2: UNIT TEST VERIFICATION
// =============================================================================

func TestPhase2_UnitTestVerification(t *testing.T) {
	t.Run("VerifyToolSchema", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "2-UnitTests",
			TestName: "Verify Tool Schema Registry",
			Details:  make(map[string]interface{}),
		}

		// Verify tool schema registry
		registry := tools.ToolSchemaRegistry
		require.NotEmpty(t, registry, "Tool schema registry should not be empty")

		// Verify required tools
		requiredTools := []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"}
		missingTools := []string{}

		for _, tool := range requiredTools {
			if _, exists := registry[tool]; !exists {
				missingTools = append(missingTools, tool)
			}
		}

		result.Success = len(missingTools) == 0
		result.Duration = time.Since(start)
		result.Details["total_tools"] = len(registry)
		result.Details["missing_tools"] = missingTools

		if !result.Success {
			result.Message = fmt.Sprintf("Missing tools: %v", missingTools)
		}

		suite.addResult(result)
		assert.Empty(t, missingTools, "All required tools should be registered")
		t.Logf("Tool schema registry contains %d tools", len(registry))
	})

	t.Run("VerifyServicesInitialization", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "2-UnitTests",
			TestName: "Verify Services Initialization",
			Details:  make(map[string]interface{}),
		}

		// Test MCP Manager creation
		mcpManager := services.NewMCPManager(nil, nil, suite.logger)
		assert.NotNil(t, mcpManager, "MCP Manager should initialize")

		// Test LSP Client creation
		lspClient := services.NewLSPClient(suite.logger)
		assert.NotNil(t, lspClient, "LSP Client should initialize")

		// Test Tool Registry creation
		toolRegistry := services.NewToolRegistry(mcpManager, lspClient)
		assert.NotNil(t, toolRegistry, "Tool Registry should initialize")

		// Test Context Manager creation
		contextManager := services.NewContextManager(100)
		assert.NotNil(t, contextManager, "Context Manager should initialize")

		result.Success = true
		result.Duration = time.Since(start)
		result.Details["services_tested"] = 5
		suite.addResult(result)

		t.Logf("All core services initialize correctly")
	})

	t.Run("VerifyConfigLoading", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "2-UnitTests",
			TestName: "Verify Config Loading",
			Details:  make(map[string]interface{}),
		}

		// Create a test config
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host: "localhost",
				Port: "7061",
			},
		}

		assert.NotEmpty(t, cfg.Server.Host, "Config should have server host")
		assert.NotEmpty(t, cfg.Server.Port, "Config should have server port")

		result.Success = true
		result.Duration = time.Since(start)
		suite.addResult(result)
	})
}

// =============================================================================
// PHASE 3: INTEGRATION TEST VERIFICATION
// =============================================================================

func TestPhase3_IntegrationTests(t *testing.T) {
	t.Run("ToolRegistryIntegration", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "3-Integration",
			TestName: "Tool Registry Integration",
			Details:  make(map[string]interface{}),
		}

		// Create components
		mcpManager := services.NewMCPManager(nil, nil, suite.logger)
		lspClient := services.NewLSPClient(suite.logger)
		toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

		// Register a custom tool
		customTool := &MockTool{
			name:        "integration-test-tool",
			description: "Tool for integration testing",
			parameters:  map[string]interface{}{"param1": map[string]interface{}{"type": "string"}},
		}

		err := toolRegistry.RegisterCustomTool(customTool)
		assert.NoError(t, err, "Should register custom tool")

		// Verify tool is listed
		allTools := toolRegistry.ListTools()
		result.Details["total_tools"] = len(allTools)

		// Execute tool
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		execResult, err := toolRegistry.ExecuteTool(ctx, "integration-test-tool", map[string]interface{}{
			"param1": "test-value",
		})

		result.Success = err == nil && execResult != nil
		result.Duration = time.Since(start)

		if err != nil {
			result.Message = err.Error()
		}

		suite.addResult(result)
	})

	t.Run("ContextManagerIntegration", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "3-Integration",
			TestName: "Context Manager Integration",
			Details:  make(map[string]interface{}),
		}

		contextManager := services.NewContextManager(100)

		// Add various context entries
		entries := []*services.ContextEntry{
			{ID: "lsp-1", Type: "lsp", Source: "/test/file.go", Content: "LSP context", Priority: 8},
			{ID: "mcp-1", Type: "mcp", Source: "mcp-server", Content: "MCP context", Priority: 7},
			{ID: "tool-1", Type: "tool", Source: "grep", Content: "Tool result", Priority: 6},
		}

		for _, entry := range entries {
			err := contextManager.AddEntry(entry)
			assert.NoError(t, err, "Should add context entry")
		}

		// Build context for different request types
		requestTypes := []string{"code_completion", "tool_execution", "chat"}
		for _, reqType := range requestTypes {
			builtCtx, err := contextManager.BuildContext(reqType, 1000)
			assert.NoError(t, err, "Should build context for %s", reqType)
			assert.NotEmpty(t, builtCtx, "Built context should not be empty")
		}

		result.Success = true
		result.Duration = time.Since(start)
		result.Details["entries_added"] = len(entries)
		result.Details["request_types_tested"] = len(requestTypes)
		suite.addResult(result)
	})

	t.Run("IntegrationOrchestratorWorkflow", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "3-Integration",
			TestName: "Integration Orchestrator Workflow",
			Details:  make(map[string]interface{}),
		}

		// Create all components
		mcpManager := services.NewMCPManager(nil, nil, suite.logger)
		lspClient := services.NewLSPClient(suite.logger)
		toolRegistry := services.NewToolRegistry(mcpManager, lspClient)
		contextManager := services.NewContextManager(100)
		orchestrator := services.NewIntegrationOrchestrator(mcpManager, lspClient, toolRegistry, contextManager)

		// Register test tools
		testTool := &MockTool{
			name:        "workflow-tool",
			description: "Workflow test tool",
			parameters:  map[string]interface{}{"input": map[string]interface{}{"type": "string"}},
		}
		err := toolRegistry.RegisterCustomTool(testTool)
		require.NoError(t, err)

		// Create and execute tool chain
		toolChain := []services.IntegrationToolExecution{
			{
				ToolName:   "workflow-tool",
				Parameters: map[string]interface{}{"toolName": "workflow-tool", "input": "test data"},
				MaxRetries: 1,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		results, err := orchestrator.ExecuteToolChain(ctx, toolChain)

		result.Success = err == nil && len(results) > 0
		result.Duration = time.Since(start)
		result.Details["results_count"] = len(results)

		if err != nil {
			result.Message = err.Error()
		}

		suite.addResult(result)
	})

}

// =============================================================================
// PHASE 4: API ENDPOINT TESTS
// =============================================================================

func TestPhase4_APIEndpoints(t *testing.T) {
	if suite.testServer == nil {
		t.Skip("Test server not initialized")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	baseURL := suite.testServer.URL

	t.Run("HealthEndpoint", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "4-API",
			TestName: "Health Endpoint",
			Details:  make(map[string]interface{}),
		}

		resp, err := client.Get(baseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		result.Success = resp.StatusCode == http.StatusOK
		result.Duration = time.Since(start)
		result.Details["status_code"] = resp.StatusCode

		if !result.Success {
			result.Message = fmt.Sprintf("Expected 200, got %d", resp.StatusCode)
		}

		suite.addResult(result)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("ModelsEndpoint", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "4-API",
			TestName: "Models Endpoint",
			Details:  make(map[string]interface{}),
		}

		resp, err := client.Get(baseURL + "/v1/models")
		require.NoError(t, err)
		defer resp.Body.Close()

		var response map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)

		result.Success = resp.StatusCode == http.StatusOK && err == nil
		result.Duration = time.Since(start)
		result.Details["status_code"] = resp.StatusCode

		suite.addResult(result)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("ChatCompletionsEndpoint", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "4-API",
			TestName: "Chat Completions Endpoint",
			Details:  make(map[string]interface{}),
		}

		requestBody := map[string]interface{}{
			"model": "gpt-3.5-turbo",
			"messages": []map[string]string{
				{"role": "user", "content": "Hello"},
			},
			"max_tokens":  10,
			"temperature": 0.1,
		}

		jsonData, _ := json.Marshal(requestBody)
		resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Accept 200 or 400/500 (provider might not be configured)
		result.Success = resp.StatusCode < 500 || resp.StatusCode == http.StatusOK
		result.Duration = time.Since(start)
		result.Details["status_code"] = resp.StatusCode

		suite.addResult(result)
	})

	t.Run("CompletionsEndpoint", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "4-API",
			TestName: "Completions Endpoint",
			Details:  make(map[string]interface{}),
		}

		requestBody := map[string]interface{}{
			"prompt":      "Hello world",
			"model":       "gpt-3.5-turbo",
			"max_tokens":  10,
			"temperature": 0.1,
		}

		jsonData, _ := json.Marshal(requestBody)
		resp, err := client.Post(baseURL+"/v1/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		result.Success = resp.StatusCode < 500
		result.Duration = time.Since(start)
		result.Details["status_code"] = resp.StatusCode

		suite.addResult(result)
	})

	t.Run("ProvidersEndpoint", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "4-API",
			TestName: "Providers Endpoint",
			Details:  make(map[string]interface{}),
		}

		resp, err := client.Get(baseURL + "/v1/providers")
		require.NoError(t, err)
		defer resp.Body.Close()

		result.Success = resp.StatusCode == http.StatusOK
		result.Duration = time.Since(start)
		result.Details["status_code"] = resp.StatusCode

		suite.addResult(result)
	})

	t.Run("MetricsEndpoint", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "4-API",
			TestName: "Metrics Endpoint",
			Details:  make(map[string]interface{}),
		}

		resp, err := client.Get(baseURL + "/metrics")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		hasPrometheusFormat := strings.Contains(string(body), "# HELP") || strings.Contains(string(body), "# TYPE")

		result.Success = resp.StatusCode == http.StatusOK
		result.Duration = time.Since(start)
		result.Details["status_code"] = resp.StatusCode
		result.Details["prometheus_format"] = hasPrometheusFormat

		suite.addResult(result)
	})

	t.Run("DebatesEndpoint", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "4-API",
			TestName: "Debates Endpoint",
			Details:  make(map[string]interface{}),
		}

		// Test creating a debate
		requestBody := map[string]interface{}{
			"topic": "Test debate topic",
			"participants": []map[string]interface{}{
				{"name": "Agent1", "provider": "mock"},
			},
		}

		jsonData, _ := json.Marshal(requestBody)
		resp, err := client.Post(baseURL+"/v1/debates", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Accept various status codes (endpoint may or may not exist)
		result.Success = resp.StatusCode < 500
		result.Duration = time.Since(start)
		result.Details["status_code"] = resp.StatusCode

		suite.addResult(result)
	})

	t.Run("EmbeddingsEndpoint", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "4-API",
			TestName: "Embeddings Endpoint",
			Details:  make(map[string]interface{}),
		}

		requestBody := map[string]interface{}{
			"input": "Test embedding input",
			"model": "text-embedding-ada-002",
		}

		jsonData, _ := json.Marshal(requestBody)
		resp, err := client.Post(baseURL+"/v1/embeddings", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		defer resp.Body.Close()

		result.Success = resp.StatusCode < 500
		result.Duration = time.Since(start)
		result.Details["status_code"] = resp.StatusCode

		suite.addResult(result)
	})

	t.Run("InvalidEndpointReturns404", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "4-API",
			TestName: "Invalid Endpoint Returns 404",
			Details:  make(map[string]interface{}),
		}

		resp, err := client.Get(baseURL + "/invalid/nonexistent/endpoint")
		require.NoError(t, err)
		defer resp.Body.Close()

		result.Success = resp.StatusCode == http.StatusNotFound
		result.Duration = time.Since(start)
		result.Details["status_code"] = resp.StatusCode

		suite.addResult(result)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("InvalidJSONReturnsBadRequest", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "4-API",
			TestName: "Invalid JSON Returns Bad Request",
			Details:  make(map[string]interface{}),
		}

		resp, err := client.Post(baseURL+"/v1/completions", "application/json", bytes.NewBuffer([]byte("invalid json")))
		require.NoError(t, err)
		defer resp.Body.Close()

		result.Success = resp.StatusCode == http.StatusBadRequest
		result.Duration = time.Since(start)
		result.Details["status_code"] = resp.StatusCode

		suite.addResult(result)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// =============================================================================
// PHASE 5: TOOL EXECUTION TESTS
// =============================================================================

func TestPhase5_ToolExecution(t *testing.T) {
	t.Run("ToolSchemaValidation", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "5-ToolExec",
			TestName: "Tool Schema Validation",
			Details:  make(map[string]interface{}),
		}

		registry := tools.ToolSchemaRegistry
		validTools := 0
		invalidTools := []string{}

		for name, schema := range registry {
			// Verify required fields
			if schema.Name == "" || schema.Description == "" {
				invalidTools = append(invalidTools, name)
				continue
			}

			// Verify parameters exist for required fields
			for _, reqField := range schema.RequiredFields {
				if _, exists := schema.Parameters[reqField]; !exists {
					invalidTools = append(invalidTools, fmt.Sprintf("%s (missing param: %s)", name, reqField))
					continue
				}
			}

			validTools++
		}

		result.Success = len(invalidTools) == 0
		result.Duration = time.Since(start)
		result.Details["valid_tools"] = validTools
		result.Details["invalid_tools"] = invalidTools

		if !result.Success {
			result.Message = fmt.Sprintf("Invalid tools: %v", invalidTools)
		}

		suite.addResult(result)
	})

	t.Run("MockToolExecution", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "5-ToolExec",
			TestName: "Mock Tool Execution",
			Details:  make(map[string]interface{}),
		}

		mcpManager := services.NewMCPManager(nil, nil, suite.logger)
		lspClient := services.NewLSPClient(suite.logger)
		toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

		// Register multiple mock tools with matching parameter names
		mockTools := []struct {
			tool   *MockTool
			params map[string]interface{}
		}{
			{
				tool:   &MockTool{name: "code-analysis-mock", description: "Analyze code", parameters: map[string]interface{}{"code": map[string]interface{}{"type": "string"}}},
				params: map[string]interface{}{"code": "function test() {}"},
			},
			{
				tool:   &MockTool{name: "security-scan-mock", description: "Scan for security issues", parameters: map[string]interface{}{"target": map[string]interface{}{"type": "string"}}},
				params: map[string]interface{}{"target": "/tmp/test"},
			},
			{
				tool:   &MockTool{name: "refactor-mock", description: "Refactor code", parameters: map[string]interface{}{"code": map[string]interface{}{"type": "string"}}},
				params: map[string]interface{}{"code": "oldFunction()"},
			},
		}

		registeredCount := 0
		executedCount := 0
		for _, item := range mockTools {
			err := toolRegistry.RegisterCustomTool(item.tool)
			if err == nil {
				registeredCount++
			} else {
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			execResult, err := toolRegistry.ExecuteTool(ctx, item.tool.name, item.params)
			cancel()

			if err == nil && execResult != nil {
				executedCount++
			}
		}

		// Success if at least one tool was registered and executed
		result.Success = registeredCount > 0 && executedCount > 0
		result.Duration = time.Since(start)
		result.Details["tools_registered"] = registeredCount
		result.Details["tools_executed"] = executedCount
		result.Details["total_tools"] = len(mockTools)

		suite.addResult(result)
	})

	t.Run("ToolErrorHandling", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "5-ToolExec",
			TestName: "Tool Error Handling",
			Details:  make(map[string]interface{}),
		}

		mcpManager := services.NewMCPManager(nil, nil, suite.logger)
		lspClient := services.NewLSPClient(suite.logger)
		toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

		// Try to execute non-existent tool
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := toolRegistry.ExecuteTool(ctx, "non-existent-tool", map[string]interface{}{})

		// Should return an error for non-existent tool
		result.Success = err != nil
		result.Duration = time.Since(start)
		result.Details["error_returned"] = err != nil

		suite.addResult(result)
	})
}

// =============================================================================
// PHASE 6: END-TO-END FLOW TESTS
// =============================================================================

func TestPhase6_EndToEndFlows(t *testing.T) {
	if suite.testServer == nil {
		t.Skip("Test server not initialized")
	}

	client := &http.Client{Timeout: 60 * time.Second}
	baseURL := suite.testServer.URL

	t.Run("CompleteChatFlow", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "6-E2E",
			TestName: "Complete Chat Flow",
			Details:  make(map[string]interface{}),
		}

		// Step 1: Check health
		resp, err := client.Get(baseURL + "/health")
		require.NoError(t, err)
		resp.Body.Close()
		step1OK := resp.StatusCode == http.StatusOK

		// Step 2: Get models
		resp, err = client.Get(baseURL + "/v1/models")
		require.NoError(t, err)
		resp.Body.Close()
		step2OK := resp.StatusCode == http.StatusOK

		// Step 3: Send chat completion
		chatRequest := map[string]interface{}{
			"model": "gpt-3.5-turbo",
			"messages": []map[string]string{
				{"role": "system", "content": "You are a helpful assistant."},
				{"role": "user", "content": "Hello!"},
			},
			"max_tokens": 50,
		}
		jsonData, _ := json.Marshal(chatRequest)
		resp, err = client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		resp.Body.Close()
		step3OK := resp.StatusCode < 500

		result.Success = step1OK && step2OK && step3OK
		result.Duration = time.Since(start)
		result.Details["step1_health"] = step1OK
		result.Details["step2_models"] = step2OK
		result.Details["step3_chat"] = step3OK

		suite.addResult(result)
	})

	t.Run("ServiceIntegrationFlow", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "6-E2E",
			TestName: "Service Integration Flow",
			Details:  make(map[string]interface{}),
		}

		// Initialize all services
		mcpManager := services.NewMCPManager(nil, nil, suite.logger)
		lspClient := services.NewLSPClient(suite.logger)
		toolRegistry := services.NewToolRegistry(mcpManager, lspClient)
		contextManager := services.NewContextManager(100)
		orchestrator := services.NewIntegrationOrchestrator(mcpManager, lspClient, toolRegistry, contextManager)

		// Register test tool
		testTool := &MockTool{
			name:        "e2e-test-tool",
			description: "E2E test tool",
			parameters:  map[string]interface{}{"input": map[string]interface{}{"type": "string"}},
		}
		err := toolRegistry.RegisterCustomTool(testTool)
		require.NoError(t, err)

		// Add context
		contextEntry := &services.ContextEntry{
			ID:       "e2e-context",
			Type:     "system",
			Source:   "e2e-test",
			Content:  "E2E test context",
			Priority: 5,
		}
		err = contextManager.AddEntry(contextEntry)
		require.NoError(t, err)

		// Execute parallel operations
		operations := []services.Operation{
			{
				ID:         "op1",
				Type:       "tool",
				Name:       "e2e-test-tool",
				Parameters: map[string]interface{}{"toolName": "e2e-test-tool", "input": "test1"},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		results, err := orchestrator.ExecuteParallelOperations(ctx, operations)

		result.Success = err == nil && len(results) == len(operations)
		result.Duration = time.Since(start)
		result.Details["operations_executed"] = len(results)

		if err != nil {
			result.Message = err.Error()
		}

		suite.addResult(result)
	})

	t.Run("ContextBuildingFlow", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "6-E2E",
			TestName: "Context Building Flow",
			Details:  make(map[string]interface{}),
		}

		contextManager := services.NewContextManager(100)

		// Add various context entries
		entries := []*services.ContextEntry{
			{ID: "code-1", Type: "lsp", Source: "/test/main.go", Content: "func main() {}", Priority: 9},
			{ID: "mcp-1", Type: "mcp", Source: "fs-server", Content: "File listing", Priority: 7},
			{ID: "tool-1", Type: "tool", Source: "grep", Content: "Search results", Priority: 6},
			{ID: "system-1", Type: "system", Source: "config", Content: "System config", Priority: 5},
		}

		for _, entry := range entries {
			err := contextManager.AddEntry(entry)
			require.NoError(t, err)
		}

		// Build context for different scenarios
		scenarios := []string{"code_completion", "tool_execution", "chat"}
		successCount := 0

		for _, scenario := range scenarios {
			ctx, err := contextManager.BuildContext(scenario, 1000)
			if err == nil && len(ctx) > 0 {
				successCount++
			}
		}

		result.Success = successCount == len(scenarios)
		result.Duration = time.Since(start)
		result.Details["scenarios_passed"] = successCount
		result.Details["total_scenarios"] = len(scenarios)

		suite.addResult(result)
	})
}

// =============================================================================
// PHASE 7: PERFORMANCE TESTS
// =============================================================================

func TestPhase7_Performance(t *testing.T) {
	if suite.testServer == nil {
		t.Skip("Test server not initialized")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	baseURL := suite.testServer.URL

	t.Run("ResponseTimeUnderLoad", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "7-Performance",
			TestName: "Response Time Under Load",
			Details:  make(map[string]interface{}),
		}

		numRequests := 50
		var totalDuration time.Duration
		var mu sync.Mutex
		successCount := 0
		var wg sync.WaitGroup

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				reqStart := time.Now()
				resp, err := client.Get(baseURL + "/health")
				reqDuration := time.Since(reqStart)

				mu.Lock()
				if err == nil && resp.StatusCode == http.StatusOK {
					successCount++
					totalDuration += reqDuration
				}
				if resp != nil {
					resp.Body.Close()
				}
				mu.Unlock()
			}()
		}

		wg.Wait()

		var avgDuration time.Duration
		if successCount > 0 {
			avgDuration = totalDuration / time.Duration(successCount)
		}

		// Success if avg response time < 1 second and > 80% success rate
		result.Success = avgDuration < time.Second && successCount > int(float64(numRequests)*0.8)
		result.Duration = time.Since(start)
		result.Details["total_requests"] = numRequests
		result.Details["successful_requests"] = successCount
		result.Details["avg_response_time"] = avgDuration.String()

		suite.addResult(result)
	})

	t.Run("ConcurrentRequestHandling", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "7-Performance",
			TestName: "Concurrent Request Handling",
			Details:  make(map[string]interface{}),
		}

		concurrency := suite.config.Concurrency
		var successful int64
		var failed int64
		var wg sync.WaitGroup

		// Launch concurrent requests
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				resp, err := client.Get(baseURL + "/health")
				if err == nil && resp.StatusCode == http.StatusOK {
					atomic.AddInt64(&successful, 1)
					resp.Body.Close()
				} else {
					atomic.AddInt64(&failed, 1)
					if resp != nil {
						resp.Body.Close()
					}
				}
			}()
		}

		wg.Wait()

		successRate := float64(successful) / float64(concurrency) * 100

		// Success if > 80% of concurrent requests succeed
		result.Success = successRate >= 80
		result.Duration = time.Since(start)
		result.Details["concurrency"] = concurrency
		result.Details["successful"] = successful
		result.Details["failed"] = failed
		result.Details["success_rate"] = fmt.Sprintf("%.2f%%", successRate)

		suite.addResult(result)
	})

	t.Run("MemoryUsage", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "7-Performance",
			TestName: "Memory Usage",
			Details:  make(map[string]interface{}),
		}

		// Force GC and measure baseline
		runtime.GC()
		var baseline runtime.MemStats
		runtime.ReadMemStats(&baseline)

		// Perform some operations
		for i := 0; i < 100; i++ {
			resp, err := client.Get(baseURL + "/health")
			if err == nil && resp != nil {
				resp.Body.Close()
			}
		}

		// Force GC and measure final
		runtime.GC()
		var final runtime.MemStats
		runtime.ReadMemStats(&final)

		// Calculate memory difference (handle case where GC reduced memory)
		var memIncreaseMB float64
		if final.Alloc >= baseline.Alloc {
			memIncreaseMB = float64(final.Alloc-baseline.Alloc) / 1024 / 1024
		} else {
			// Memory was actually freed (good!)
			memIncreaseMB = -float64(baseline.Alloc-final.Alloc) / 1024 / 1024
		}

		// Success if memory increase < 50MB (or memory was freed)
		result.Success = memIncreaseMB < 50
		result.Duration = time.Since(start)
		result.Details["baseline_mb"] = fmt.Sprintf("%.2f", float64(baseline.Alloc)/1024/1024)
		result.Details["final_mb"] = fmt.Sprintf("%.2f", float64(final.Alloc)/1024/1024)
		result.Details["increase_mb"] = fmt.Sprintf("%.2f", memIncreaseMB)

		suite.addResult(result)
	})

	t.Run("ThroughputMeasurement", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "7-Performance",
			TestName: "Throughput Measurement",
			Details:  make(map[string]interface{}),
		}

		testDuration := 5 * time.Second
		var requestCount int64
		var successCount int64

		ctx, cancel := context.WithTimeout(context.Background(), testDuration)
		defer cancel()

		var wg sync.WaitGroup

		// Launch workers
		numWorkers := 10
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
						atomic.AddInt64(&requestCount, 1)
						resp, err := client.Get(baseURL + "/health")
						if err == nil && resp.StatusCode == http.StatusOK {
							atomic.AddInt64(&successCount, 1)
						}
						if resp != nil {
							resp.Body.Close()
						}
					}
				}
			}()
		}

		wg.Wait()

		throughput := float64(successCount) / testDuration.Seconds()

		// Success if throughput > 10 req/sec
		result.Success = throughput > 10
		result.Duration = time.Since(start)
		result.Details["total_requests"] = requestCount
		result.Details["successful_requests"] = successCount
		result.Details["throughput_per_sec"] = fmt.Sprintf("%.2f", throughput)

		suite.addResult(result)
	})
}

// =============================================================================
// PHASE 8: CLEANUP
// =============================================================================

func TestPhase8_Cleanup(t *testing.T) {
	t.Run("StopMockServers", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "8-Cleanup",
			TestName: "Stop Mock Servers",
			Details:  make(map[string]interface{}),
		}

		if suite.mockLLMServer != nil {
			suite.mockLLMServer.Close()
		}

		if suite.testServer != nil {
			suite.testServer.Close()
		}

		result.Success = true
		result.Duration = time.Since(start)
		suite.addResult(result)

		t.Log("Mock servers stopped")
	})

	t.Run("CleanupTestData", func(t *testing.T) {
		start := time.Now()
		result := AutomationResult{
			Phase:    "8-Cleanup",
			TestName: "Cleanup Test Data",
			Details:  make(map[string]interface{}),
		}

		// Clean up any test artifacts
		// (In a real scenario, this would clean up test databases, temp files, etc.)

		result.Success = true
		result.Duration = time.Since(start)
		suite.addResult(result)

		t.Log("Test data cleaned up")
	})
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// createMockLLMServer creates a mock LLM server for testing
func createMockLLMServer() *httptest.Server {
	handler := http.NewServeMux()

	// Health endpoint
	handler.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	// Chat completions endpoint
	handler.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"id":      "mock-chatcmpl-123",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]string{
						"role":    "assistant",
						"content": "This is a mock response from the test LLM server.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 15,
				"total_tokens":      25,
			},
		}
		json.NewEncoder(w).Encode(response)
	})

	// Completions endpoint
	handler.HandleFunc("/v1/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"id":      "mock-cmpl-123",
			"object":  "text_completion",
			"created": time.Now().Unix(),
			"choices": []map[string]interface{}{
				{
					"text":          "Mock completion response",
					"index":         0,
					"finish_reason": "stop",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	})

	// Models endpoint
	handler.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"id": "mock-model-1", "object": "model", "created": time.Now().Unix()},
				{"id": "mock-model-2", "object": "model", "created": time.Now().Unix()},
			},
		}
		json.NewEncoder(w).Encode(response)
	})

	// Embeddings endpoint
	handler.HandleFunc("/v1/embeddings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"object":    "embedding",
					"embedding": []float64{0.1, 0.2, 0.3, 0.4, 0.5},
					"index":     0,
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	})

	return httptest.NewServer(handler)
}

// createTestRouter creates a test router with all handlers
func createTestRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// Health endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "timestamp": time.Now().Unix()})
	})

	// Metrics endpoint
	r.GET("/metrics", func(c *gin.Context) {
		metrics := `# HELP helixagent_requests_total Total number of requests
# TYPE helixagent_requests_total counter
helixagent_requests_total 100
# HELP helixagent_response_time_seconds Response time in seconds
# TYPE helixagent_response_time_seconds histogram
helixagent_response_time_seconds_bucket{le="0.1"} 50
`
		c.String(http.StatusOK, metrics)
	})

	// API v1 group
	v1 := r.Group("/v1")
	{
		// Models endpoint
		v1.GET("/models", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"object": "list",
				"data": []gin.H{
					{"id": "gpt-3.5-turbo", "object": "model"},
					{"id": "gpt-4", "object": "model"},
					{"id": "ai-debate-ensemble", "object": "model"},
				},
			})
		})

		// Health endpoint (v1)
		v1.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":    "healthy",
				"providers": []gin.H{{"name": "mock", "status": "healthy"}},
			})
		})

		// Providers endpoint
		v1.GET("/providers", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"providers": []gin.H{
					{"name": "mock-provider", "status": "healthy", "models": 2},
				},
			})
		})

		// Chat completions endpoint
		v1.POST("/chat/completions", func(c *gin.Context) {
			var req map[string]interface{}
			if err := c.BindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"id":      "chatcmpl-test",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"choices": []gin.H{
					{
						"index": 0,
						"message": gin.H{
							"role":    "assistant",
							"content": "Test response from automation suite",
						},
						"finish_reason": "stop",
					},
				},
			})
		})

		// Completions endpoint
		v1.POST("/completions", func(c *gin.Context) {
			var req map[string]interface{}
			if err := c.BindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"id":      "cmpl-test",
				"object":  "text_completion",
				"created": time.Now().Unix(),
				"choices": []gin.H{
					{
						"text":          "Test completion response",
						"index":         0,
						"finish_reason": "stop",
					},
				},
			})
		})

		// Embeddings endpoint
		v1.POST("/embeddings", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"object": "list",
				"data": []gin.H{
					{
						"object":    "embedding",
						"embedding": []float64{0.1, 0.2, 0.3},
						"index":     0,
					},
				},
			})
		})

		// Debates endpoint
		v1.POST("/debates", func(c *gin.Context) {
			var req map[string]interface{}
			if err := c.BindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"id":     "debate-test",
				"topic":  req["topic"],
				"status": "created",
			})
		})

		// Ensemble endpoint
		v1.POST("/ensemble/completions", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"object":   "ensemble.completion",
				"ensemble": gin.H{"strategy": "confidence_weighted"},
				"result":   "Ensemble response",
			})
		})
	}

	return r
}

// MockTool implements the CustomTool interface for testing
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
	// Simulate some processing time
	time.Sleep(10 * time.Millisecond)

	return map[string]interface{}{
		"tool":      m.name,
		"params":    params,
		"result":    "success",
		"timestamp": time.Now().Unix(),
		"message":   fmt.Sprintf("Executed %s successfully", m.name),
	}, nil
}

func (m *MockTool) Source() string {
	return m.source
}

// =============================================================================
// OPTIONAL: EXTERNAL SERVER TESTS (when real server is running)
// =============================================================================

func TestExternalServer_IfRunning(t *testing.T) {
	// Try to connect to a running HelixAgent server
	baseURL := "http://localhost:7061"

	// Check if server is running
	conn, err := net.DialTimeout("tcp", "localhost:7061", 2*time.Second)
	if err != nil {
		t.Skip("External HelixAgent server not running - skipping external tests")
		return
	}
	conn.Close()

	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("ExternalHealthCheck", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/health")
		if err != nil {
			t.Skipf("Cannot connect to external server: %v", err)
			return
		}
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		t.Logf("External server health check passed")
	})

	t.Run("ExternalModelsEndpoint", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/v1/models")
		if err != nil {
			t.Skipf("Cannot connect to external server: %v", err)
			return
		}
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)

		if data, ok := response["data"].([]interface{}); ok {
			t.Logf("External server has %d models", len(data))
		}
	})
}

// =============================================================================
// OPTIONAL: RUN EXTERNAL GO TESTS
// =============================================================================

func TestRunUnitTests_Optional(t *testing.T) {
	if os.Getenv("RUN_EXTERNAL_TESTS") != "true" {
		t.Skip("Set RUN_EXTERNAL_TESTS=true to run external Go tests")
	}

	start := time.Now()
	result := AutomationResult{
		Phase:    "2-UnitTests",
		TestName: "Run External Unit Tests",
		Details:  make(map[string]interface{}),
	}

	// Run unit tests using go test
	cmd := exec.Command("go", "test", "-v", "-short", "-count=1", "./internal/...")
	cmd.Dir = "/run/media/milosvasic/DATA4TB/Projects/HelixAgent"

	output, err := cmd.CombinedOutput()

	result.Success = err == nil
	result.Duration = time.Since(start)
	result.Details["output_length"] = len(output)

	if err != nil {
		result.Message = err.Error()
		t.Logf("Unit tests output:\n%s", string(output))
	}

	suite.addResult(result)
}

// Placeholder for handlers import (to ensure compilation)
var _ = handlers.NewHealthHandler
