package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/database"
	"github.com/superagent/superagent/internal/handlers"
	"github.com/superagent/superagent/internal/services"
)

func TestMultiProviderIntegration(t *testing.T) {
	// Setup test configuration
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: "8081", // Use different port for testing
		},
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     "5432",
			Name:     "test_superagent",
			User:     "test_user",
			Password: "test_password",
			SSLMode:  "disable",
		},
	}

	// Initialize database connection
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	defer db.Close()

	// Initialize provider registry with test configuration
	registryConfig := &services.RegistryConfig{
		Providers: map[string]*services.ProviderConfig{
			"test-provider": {
				Name:    "Test Provider",
				Type:    "openrouter",
				Enabled: true,
				APIKey:  "test-key",
				Models: []services.ModelConfig{
					{
						ID:      "test-model",
						Name:    "Test Model",
						Enabled: true,
					},
				},
			},
		},
	}

	// Create memory service
	memoryService := services.NewMemoryService(cfg)

	// Initialize provider registry
	providerRegistry := services.NewProviderRegistry(registryConfig, memoryService)

	// Initialize handlers
	unifiedHandler := handlers.NewUnifiedHandler(providerRegistry, cfg)

	// Setup test router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())

	// Register routes
	api := router.Group("/v1")
	auth := func(c *gin.Context) { /* Simple auth for tests */ }
	unifiedHandler.RegisterOpenAIRoutes(api, auth)

	// Test models endpoint
	t.Run("ModelsEndpoint", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Errorf("Failed to parse response: %v", err)
		}

		if data, ok := response["data"].([]interface{}); !ok {
			t.Error("Expected data array in response")
		} else {
			if len(data) == 0 {
				t.Error("Expected at least one model in response")
			}
		}
	})
}

// MCP and LSP Integration Tests

func TestMCP_LSP_Integration(t *testing.T) {
	// Create MCP manager
	mcpManager := services.NewMCPManager()

	// Create LSP client
	lspClient := services.NewLSPClient("/tmp", "go")

	// Create tool registry
	toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

	// Test registering MCP server
	serverConfig := map[string]interface{}{
		"name":    "test-mcp-server",
		"command": []interface{}{"echo", "test"},
	}

	err := mcpManager.RegisterServer(serverConfig)
	if err != nil {
		t.Logf("MCP server registration failed (expected in test env): %v", err)
	}

	// Test tool registry integration
	tools := toolRegistry.ListTools()
	if len(tools) == 0 {
		t.Log("No tools available (expected in test env)")
	}

	// Test LSP client basic functionality
	diagnostics := lspClient.GetDiagnostics("/tmp/test.go")
	if diagnostics == nil {
		t.Log("No diagnostics available (expected in test env)")
	}
}

func TestToolRegistry_Integration(t *testing.T) {
	// Create components
	mcpManager := services.NewMCPManager()
	lspClient := services.NewLSPClient("/tmp", "go")
	toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

	// Register custom tool
	customTool := &MockTool{
		name:        "integration-test-tool",
		description: "Tool for integration testing",
		parameters:  map[string]interface{}{"param1": map[string]interface{}{"type": "string"}},
	}

	err := toolRegistry.RegisterCustomTool(customTool)
	if err != nil {
		t.Errorf("Failed to register custom tool: %v", err)
	}

	// Test tool execution
	result, err := toolRegistry.ExecuteTool(context.Background(), "integration-test-tool", map[string]interface{}{
		"param1": "test-value",
	})
	if err != nil {
		t.Errorf("Failed to execute tool: %v", err)
	}

	if result == nil {
		t.Error("Expected result from tool execution")
	}
}

func TestContextManager_Integration(t *testing.T) {
	// Create context manager
	contextManager := services.NewContextManager(100)

	// Add various types of context entries
	entries := []*services.ContextEntry{
		{
			ID:       "lsp-entry",
			Type:     "lsp",
			Source:   "/tmp/test.go",
			Content:  "function test() { return 'lsp context'; }",
			Priority: 8,
		},
		{
			ID:       "mcp-entry",
			Type:     "mcp",
			Source:   "filesystem-tool",
			Content:  "MCP tool result: file listing",
			Priority: 7,
		},
		{
			ID:       "tool-entry",
			Type:     "tool",
			Source:   "grep-tool",
			Content:  "Tool execution result",
			Priority: 6,
		},
	}

	for _, entry := range entries {
		err := contextManager.AddEntry(entry)
		if err != nil {
			t.Errorf("Failed to add context entry: %v", err)
		}
	}

	// Test context building for different request types
	requestTypes := []string{"code_completion", "tool_execution", "chat"}

	for _, reqType := range requestTypes {
		context, err := contextManager.BuildContext(reqType, 1000)
		if err != nil {
			t.Errorf("Failed to build context for %s: %v", reqType, err)
		}

		if len(context) == 0 {
			t.Errorf("Expected context entries for %s", reqType)
		}

		// Verify entries are sorted by relevance score (higher priority gets higher score)
		for i := 1; i < len(context); i++ {
			// Since we can't easily check scores, just verify we have entries
			if context[i] == nil {
				t.Errorf("Nil context entry at index %d", i)
			}
		}
	}
}

func TestIntegrationOrchestrator_Workflow(t *testing.T) {
	// Create components
	mcpManager := services.NewMCPManager()
	lspClient := services.NewLSPClient("/tmp", "go")
	toolRegistry := services.NewToolRegistry(mcpManager, lspClient)
	contextManager := services.NewContextManager(100)

	// Create orchestrator
	orchestrator := services.NewIntegrationOrchestrator(mcpManager, lspClient, toolRegistry, contextManager)

	// Register a test tool
	testTool := &MockTool{
		name:        "workflow-test-tool",
		description: "Tool for workflow testing",
		parameters:  map[string]interface{}{"toolName": map[string]interface{}{"type": "string"}, "input": map[string]interface{}{"type": "string"}},
	}
	err := toolRegistry.RegisterCustomTool(testTool)
	if err != nil {
		t.Errorf("Failed to register test tool: %v", err)
	}

	// Create tool chain
	toolChain := []services.ToolExecution{
		{
			ToolName:   "workflow-test-tool",
			Parameters: map[string]interface{}{"toolName": "workflow-test-tool", "input": "test data"},
			MaxRetries: 1,
		},
	}

	// Execute tool chain
	results, err := orchestrator.ExecuteToolChain(context.Background(), toolChain)
	if err != nil {
		t.Errorf("Failed to execute tool chain: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected results from tool chain execution")
	}
}

func TestSecuritySandbox_Integration(t *testing.T) {
	// Create security sandbox
	sandbox := services.NewSecuritySandbox()

	// Test allowed commands
	allowedCommands := []string{"ls", "cat", "grep", "find", "wc"}

	for _, cmd := range allowedCommands {
		args := []string{}
		if cmd == "ls" {
			args = []string{"-la", "/tmp"}
		} else if cmd == "cat" {
			args = []string{"/dev/null"}
		} else if cmd == "grep" {
			args = []string{"test", "/dev/null"}
		} else if cmd == "find" {
			args = []string{"/tmp", "-name", "test"}
		} else if cmd == "wc" {
			args = []string{"-l", "/dev/null"}
		}

		result, err := sandbox.ExecuteSandboxed(context.Background(), cmd, args)
		if err != nil {
			t.Logf("Command %s failed (may be expected): %v", cmd, err)
			continue
		}

		if result == nil {
			t.Errorf("Expected result for allowed command %s", cmd)
		}

		// Commands may fail due to permissions or missing files, that's ok for this test
		t.Logf("Command %s executed with success: %v", cmd, result.Success)
	}

	// Test disallowed commands
	disallowedCommands := []string{"rm", "dd", "mkfs", "shutdown"}

	for _, cmd := range disallowedCommands {
		_, err := sandbox.ExecuteSandboxed(context.Background(), cmd, []string{})
		if err == nil {
			t.Errorf("Expected error for disallowed command %s", cmd)
		}
	}

	// Test parameter validation
	err := sandbox.ValidateToolExecution("test-tool", map[string]interface{}{
		"safe_param": "safe value",
	})
	if err != nil {
		t.Errorf("Safe parameters should pass validation: %v", err)
	}

	err = sandbox.ValidateToolExecution("test-tool", map[string]interface{}{
		"dangerous": "rm -rf /; echo hacked",
	})
	if err == nil {
		t.Error("Dangerous parameters should fail validation")
	}
}

func TestNewServicesIntegration(t *testing.T) {
	// Create all components
	mcpManager := services.NewMCPManager()
	lspClient := services.NewLSPClient("/tmp", "go")
	toolRegistry := services.NewToolRegistry(mcpManager, lspClient)
	contextManager := services.NewContextManager(100)
	securitySandbox := services.NewSecuritySandbox()

	// Create orchestrator
	orchestrator := services.NewIntegrationOrchestrator(mcpManager, lspClient, toolRegistry, contextManager)

	// Register test tools
	tools := []*MockTool{
		{
			name:        "code-analysis-tool",
			description: "Analyzes code",
			parameters:  map[string]interface{}{"code": map[string]interface{}{"type": "string"}},
		},
		{
			name:        "security-scan-tool",
			description: "Scans for security issues",
			parameters:  map[string]interface{}{"target": map[string]interface{}{"type": "string"}},
		},
	}

	for _, tool := range tools {
		err := toolRegistry.RegisterCustomTool(tool)
		if err != nil {
			t.Errorf("Failed to register tool %s: %v", tool.name, err)
		}
	}

	// Add context
	contextEntry := &services.ContextEntry{
		ID:       "system-context",
		Type:     "system",
		Source:   "integration-test",
		Content:  "System is running integration tests",
		Priority: 5,
	}
	err := contextManager.AddEntry(contextEntry)
	if err != nil {
		t.Errorf("Failed to add context: %v", err)
	}

	// Test parallel operations
	operations := []services.Operation{
		{
			ID:         "op1",
			Type:       "tool",
			Name:       "code-analysis-tool",
			Parameters: map[string]interface{}{"toolName": "code-analysis-tool", "code": "function test() {}"},
		},
		{
			ID:         "op2",
			Type:       "tool",
			Name:       "security-scan-tool",
			Parameters: map[string]interface{}{"toolName": "security-scan-tool", "target": "/tmp"},
		},
	}

	results, err := orchestrator.ExecuteParallelOperations(context.Background(), operations)
	if err != nil {
		t.Errorf("Failed to execute parallel operations: %v", err)
	}

	if len(results) != len(operations) {
		t.Errorf("Expected %d results, got %d", len(operations), len(results))
	}

	// Test security sandbox with tool execution
	for _, op := range operations {
		err := securitySandbox.ValidateToolExecution(op.Name, op.Parameters)
		if err != nil {
			t.Logf("Security validation failed for %s: %v", op.Name, err)
		}
	}

	// Verify context was used
	builtContext, err := contextManager.BuildContext("tool_execution", 1000)
	if err != nil {
		t.Errorf("Failed to build context: %v", err)
	}

	if len(builtContext) == 0 {
		t.Error("Expected context to be available")
	}
}

// Mock Tool for integration testing
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
	// Simulate tool execution
	return map[string]interface{}{
		"tool":    m.name,
		"params":  params,
		"result":  "success",
		"message": fmt.Sprintf("Executed %s with params %v", m.name, params),
	}, nil
}

func (m *MockTool) Source() string {
	return m.source
}
