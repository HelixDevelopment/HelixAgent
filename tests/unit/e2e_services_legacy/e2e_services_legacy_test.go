// Package e2e_services_legacy holds the in-process subset of what used to
// live in `tests/e2e/e2e_test.go`. CONST-030 forbids in-process mocks in
// non-unit tests; the original `TestE2ENewServicesWorkflow` wired up a
// `MockTool` directly against `services.NewMCPManager` / `LSPClient` /
// `ContextManager` / `IntegrationOrchestrator` without touching the live
// HelixAgent on :8100, so it is a unit test by construction. It was
// demoted to this package (PR23, CONST-030 campaign) with no coverage
// change.
package e2e_services_legacy_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"dev.helix.agent/internal/services"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2ENewServicesWorkflow tests end-to-end workflows using the new services.
// Pure in-process; no network calls. Kept as a unit test for CONST-030.
func TestE2ENewServicesWorkflow(t *testing.T) {
	t.Run("CompleteCodeAnalysisWorkflow", func(t *testing.T) {
		// Initialize all services
		logger := logrus.New()
		logger.SetLevel(logrus.PanicLevel)
		mcpManager := services.NewMCPManager(nil, nil, logger)
		lspClient := services.NewLSPClient(logger)
		toolRegistry := services.NewToolRegistry(mcpManager, lspClient)
		contextManager := services.NewContextManager(100)
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
		}

		// Test tool chain execution
		toolChain := []services.IntegrationToolExecution{
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
		}

		// Verify context management
		builtContext, err := contextManager.BuildContext("code_completion", 1000)
		if err != nil {
			t.Logf("Context building failed: %v", err)
		} else {
			assert.NotEmpty(t, builtContext)
		}
	})

	t.Run("CompleteMCP_LSP_IntegrationWorkflow", func(t *testing.T) {
		// Test MCP server registration and tool discovery
		logger := logrus.New()
		logger.SetLevel(logrus.PanicLevel)
		mcpManager := services.NewMCPManager(nil, nil, logger)

		serverConfig := map[string]interface{}{
			"name":    "filesystem-mcp",
			"command": []interface{}{"echo", "filesystem-server"},
		}

		err := mcpManager.RegisterServer(serverConfig)
		if err != nil {
			t.Logf("MCP server registration failed (expected in test env): %v", err)
		}

		tools := mcpManager.ListTools()
		_ = tools

		// Test LSP client initialization
		lspClient := services.NewLSPClient(logger)

		// Test tool registry with MCP and LSP
		toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

		registryTools := toolRegistry.ListTools()
		_ = registryTools

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
		_ = conflicts

		// Test different context building scenarios
		scenarios := []string{"code_completion", "tool_execution", "chat"}
		for _, scenario := range scenarios {
			built, err := contextManager.BuildContext(scenario, 1000)
			if err != nil {
				t.Logf("Context building failed for %s: %v", scenario, err)
			}
			_ = built
		}
	})
}

// MockTool is the in-process tool fixture used by the demoted E2E flow test.
type MockTool struct {
	name        string
	description string
	parameters  map[string]interface{}
	source      string
}

func (m *MockTool) Name() string                       { return m.name }
func (m *MockTool) Description() string                { return m.description }
func (m *MockTool) Parameters() map[string]interface{} { return m.parameters }
func (m *MockTool) Source() string                     { return m.source }
func (m *MockTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	time.Sleep(10 * time.Millisecond)
	result := map[string]interface{}{
		"tool":      m.name,
		"params":    params,
		"result":    "success",
		"timestamp": time.Now().Unix(),
		"message":   fmt.Sprintf("Executed %s successfully", m.name),
	}
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
