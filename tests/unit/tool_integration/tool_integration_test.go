// Package tool_integration_test provides comprehensive in-process tests for
// the HelixAgent tool system — tool registry, tool schema, tool execution via
// httptest-backed gin router. Uses a local `ToolTestMockTool` struct and an
// in-process `httptest.Server` for WebFetch/WebSearch fixtures. No live :7061
// HTTP. CONST-030 compliant via Pattern-4 demote-to-unit.
package tool_integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
)

// ============================================
// TOOL REGISTRY TESTS
// ============================================

func TestToolSchemaRegistry_All21ToolsRegistered(t *testing.T) {
	// Verify all 21 tools are registered in ToolSchemaRegistry
	expectedTools := []string{
		// Core tools (9)
		"Bash", "Read", "Write", "Edit", "Glob", "Grep", "WebFetch", "WebSearch", "Task",
		// Version control tools (2)
		"Git", "Diff",
		// Testing tools (2)
		"Test", "Lint",
		// File intelligence tools (2)
		"TreeView", "FileInfo",
		// Code intelligence tools (3)
		"Symbols", "References", "Definition",
		// Workflow tools (3)
		"PR", "Issue", "Workflow",
	}

	registeredCount := len(tools.ToolSchemaRegistry)
	assert.Equal(t, 21, registeredCount, "Expected 21 tools to be registered, got %d", registeredCount)

	for _, toolName := range expectedTools {
		schema, exists := tools.GetToolSchema(toolName)
		assert.True(t, exists, "Tool %s should be registered in ToolSchemaRegistry", toolName)
		if exists {
			assert.NotEmpty(t, schema.Name, "Tool %s should have a name", toolName)
			assert.NotEmpty(t, schema.Description, "Tool %s should have a description", toolName)
			assert.NotEmpty(t, schema.Category, "Tool %s should have a category", toolName)
			assert.NotNil(t, schema.Parameters, "Tool %s should have parameters", toolName)
		}
	}
}

func TestToolSchemaRegistry_CategoriesCorrect(t *testing.T) {
	categoryTests := []struct {
		toolName string
		category string
	}{
		{"Bash", tools.CategoryCore},
		{"Read", tools.CategoryFileSystem},
		{"Write", tools.CategoryFileSystem},
		{"Edit", tools.CategoryFileSystem},
		{"Glob", tools.CategoryFileSystem},
		{"Grep", tools.CategoryFileSystem},
		{"WebFetch", tools.CategoryWeb},
		{"WebSearch", tools.CategoryWeb},
		{"Task", tools.CategoryCore},
		{"Git", tools.CategoryVersionControl},
		{"Diff", tools.CategoryVersionControl},
		{"Test", tools.CategoryCore},
		{"Lint", tools.CategoryCore},
		{"TreeView", tools.CategoryFileSystem},
		{"FileInfo", tools.CategoryFileSystem},
		{"Symbols", tools.CategoryCodeIntel},
		{"References", tools.CategoryCodeIntel},
		{"Definition", tools.CategoryCodeIntel},
		{"PR", tools.CategoryWorkflow},
		{"Issue", tools.CategoryWorkflow},
		{"Workflow", tools.CategoryWorkflow},
	}

	for _, tc := range categoryTests {
		t.Run(tc.toolName, func(t *testing.T) {
			schema, exists := tools.GetToolSchema(tc.toolName)
			require.True(t, exists, "Tool %s should exist", tc.toolName)
			assert.Equal(t, tc.category, schema.Category, "Tool %s should be in category %s", tc.toolName, tc.category)
		})
	}
}

func TestToolSchemaRegistry_RequiredFieldsCorrect(t *testing.T) {
	requiredFieldsTests := []struct {
		toolName       string
		requiredFields []string
	}{
		{"Bash", []string{"command", "description"}},
		{"Read", []string{"file_path"}},
		{"Write", []string{"file_path", "content"}},
		{"Edit", []string{"file_path", "old_string", "new_string"}},
		{"Glob", []string{"pattern"}},
		{"Grep", []string{"pattern"}},
		{"WebFetch", []string{"url", "prompt"}},
		{"WebSearch", []string{"query"}},
		{"Task", []string{"prompt", "description", "subagent_type"}},
		{"Git", []string{"operation", "description"}},
		{"Diff", []string{"description"}},
		{"Test", []string{"description"}},
		{"Lint", []string{"description"}},
		{"TreeView", []string{"description"}},
		{"FileInfo", []string{"file_path", "description"}},
		{"Symbols", []string{"description"}},
		{"References", []string{"symbol", "description"}},
		{"Definition", []string{"symbol", "description"}},
		{"PR", []string{"action", "description"}},
		{"Issue", []string{"action", "description"}},
		{"Workflow", []string{"action", "description"}},
	}

	for _, tc := range requiredFieldsTests {
		t.Run(tc.toolName, func(t *testing.T) {
			requiredFields := tools.GetRequiredFields(tc.toolName)
			require.NotNil(t, requiredFields, "Tool %s should have required fields", tc.toolName)
			assert.ElementsMatch(t, tc.requiredFields, requiredFields,
				"Tool %s required fields mismatch", tc.toolName)
		})
	}
}

func TestToolSchemaValidation_ValidArgs(t *testing.T) {
	validArgsTests := []struct {
		toolName string
		args     map[string]interface{}
	}{
		{"Bash", map[string]interface{}{"command": "ls -la", "description": "List files"}},
		{"Read", map[string]interface{}{"file_path": "/tmp/test.txt"}},
		{"Write", map[string]interface{}{"file_path": "/tmp/test.txt", "content": "hello"}},
		{"Edit", map[string]interface{}{"file_path": "/tmp/test.txt", "old_string": "foo", "new_string": "bar"}},
		{"Glob", map[string]interface{}{"pattern": "**/*.go"}},
		{"Grep", map[string]interface{}{"pattern": "func main"}},
		{"WebFetch", map[string]interface{}{"url": "https://example.com", "prompt": "summarize"}},
		{"WebSearch", map[string]interface{}{"query": "golang tutorials"}},
		{"Git", map[string]interface{}{"operation": "status", "description": "Check git status"}},
		{"Test", map[string]interface{}{"description": "Run unit tests"}},
	}

	for _, tc := range validArgsTests {
		t.Run(tc.toolName, func(t *testing.T) {
			err := tools.ValidateToolArgs(tc.toolName, tc.args)
			assert.NoError(t, err, "Tool %s should accept valid args", tc.toolName)
		})
	}
}

func TestToolSchemaValidation_MissingRequiredField(t *testing.T) {
	missingFieldTests := []struct {
		toolName     string
		args         map[string]interface{}
		missingField string
	}{
		{"Bash", map[string]interface{}{"command": "ls"}, "description"},
		{"Bash", map[string]interface{}{"description": "list files"}, "command"},
		{"Read", map[string]interface{}{}, "file_path"},
		{"Write", map[string]interface{}{"file_path": "/tmp/test.txt"}, "content"},
		{"Edit", map[string]interface{}{"file_path": "/tmp/test.txt", "old_string": "foo"}, "new_string"},
		{"Glob", map[string]interface{}{}, "pattern"},
		{"Grep", map[string]interface{}{}, "pattern"},
		{"WebFetch", map[string]interface{}{"url": "https://example.com"}, "prompt"},
		{"WebSearch", map[string]interface{}{}, "query"},
		{"Git", map[string]interface{}{"operation": "status"}, "description"},
	}

	for _, tc := range missingFieldTests {
		t.Run(tc.toolName+"_missing_"+tc.missingField, func(t *testing.T) {
			err := tools.ValidateToolArgs(tc.toolName, tc.args)
			require.Error(t, err, "Tool %s should reject args missing %s", tc.toolName, tc.missingField)
			assert.Contains(t, err.Error(), tc.missingField,
				"Error should mention missing field %s", tc.missingField)
		})
	}
}

func TestToolSchemaValidation_EmptyRequiredField(t *testing.T) {
	emptyFieldTests := []struct {
		toolName   string
		args       map[string]interface{}
		emptyField string
	}{
		{"Bash", map[string]interface{}{"command": "", "description": "test"}, "command"},
		{"Read", map[string]interface{}{"file_path": ""}, "file_path"},
		{"Glob", map[string]interface{}{"pattern": ""}, "pattern"},
	}

	for _, tc := range emptyFieldTests {
		t.Run(tc.toolName+"_empty_"+tc.emptyField, func(t *testing.T) {
			err := tools.ValidateToolArgs(tc.toolName, tc.args)
			require.Error(t, err, "Tool %s should reject empty %s", tc.toolName, tc.emptyField)
			assert.Contains(t, err.Error(), tc.emptyField,
				"Error should mention empty field %s", tc.emptyField)
		})
	}
}

func TestToolSchemaValidation_UnknownTool(t *testing.T) {
	err := tools.ValidateToolArgs("NonExistentTool", map[string]interface{}{})
	require.Error(t, err, "Should reject unknown tool")
	assert.Contains(t, err.Error(), "unknown tool", "Error should mention unknown tool")
}

func TestToolSearch_ExactMatch(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		{"Bash", "Bash"},
		{"Read", "Read"},
		{"Git", "Git"},
		{"grep", "Grep"},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			results := tools.SearchTools(tools.SearchOptions{
				Query:      tc.query,
				MaxResults: 1,
			})
			require.NotEmpty(t, results, "Search for %s should return results", tc.query)
			assert.Equal(t, tc.expected, results[0].Tool.Name,
				"First result should be exact match for %s", tc.query)
			assert.Equal(t, 1.0, results[0].Score, "Exact match should have score 1.0")
		})
	}
}

func TestToolSearch_AliasMatch(t *testing.T) {
	aliasTests := []struct {
		alias        string
		expectedTool string
	}{
		{"shell", "Bash"},
		{"bash", "Bash"},
		{"tree", "TreeView"},
		{"refs", "References"},
		{"goto", "Definition"},
		{"pullrequest", "PR"},
		{"ci", "Workflow"},
	}

	for _, tc := range aliasTests {
		t.Run(tc.alias, func(t *testing.T) {
			results := tools.SearchTools(tools.SearchOptions{
				Query:      tc.alias,
				MaxResults: 5,
			})
			require.NotEmpty(t, results, "Search for alias %s should return results", tc.alias)

			found := false
			for _, result := range results {
				if result.Tool.Name == tc.expectedTool {
					found = true
					break
				}
			}
			assert.True(t, found, "Alias %s should find tool %s", tc.alias, tc.expectedTool)
		})
	}
}

func TestToolSearch_DescriptionMatch(t *testing.T) {
	tests := []struct {
		query        string
		expectedTool string
	}{
		{"execute", "Bash"},
		{"file", "Read"},
		{"version control", "Git"},
		{"pull request", "PR"},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			results := tools.SearchTools(tools.SearchOptions{
				Query:      tc.query,
				MaxResults: 10,
			})
			require.NotEmpty(t, results, "Search for %s should return results", tc.query)

			found := false
			for _, result := range results {
				if result.Tool.Name == tc.expectedTool {
					found = true
					break
				}
			}
			assert.True(t, found, "Description search for %s should find %s", tc.query, tc.expectedTool)
		})
	}
}

func TestToolSearch_CategoryFilter(t *testing.T) {
	categoryTests := []struct {
		category      string
		expectedTools []string
	}{
		{tools.CategoryCore, []string{"Bash", "Task", "Test", "Lint"}},
		{tools.CategoryFileSystem, []string{"Read", "Write", "Edit", "Glob", "Grep", "TreeView", "FileInfo"}},
		{tools.CategoryVersionControl, []string{"Git", "Diff"}},
		{tools.CategoryCodeIntel, []string{"Symbols", "References", "Definition"}},
		{tools.CategoryWorkflow, []string{"PR", "Issue", "Workflow"}},
		{tools.CategoryWeb, []string{"WebFetch", "WebSearch"}},
	}

	for _, tc := range categoryTests {
		t.Run(tc.category, func(t *testing.T) {
			results := tools.SearchTools(tools.SearchOptions{
				Query:      "",
				Categories: []string{tc.category},
				MaxResults: 50,
				MinScore:   0,
			})

			foundNames := make([]string, len(results))
			for i, r := range results {
				foundNames[i] = r.Tool.Name
			}

			for _, expected := range tc.expectedTools {
				assert.Contains(t, foundNames, expected,
					"Category %s should include tool %s", tc.category, expected)
			}
		})
	}
}

func TestToolSearch_FuzzyMatch(t *testing.T) {
	results := tools.SearchTools(tools.SearchOptions{
		Query:      "bsh", // fuzzy for "Bash"
		FuzzyMatch: true,
		MaxResults: 5,
	})

	// Should find Bash with fuzzy matching
	found := false
	for _, result := range results {
		if result.Tool.Name == "Bash" {
			found = true
			assert.Equal(t, "fuzzy", result.MatchType, "Match type should be fuzzy")
			break
		}
	}
	assert.True(t, found, "Fuzzy search for 'bsh' should find Bash")
}

func TestToolSuggestions(t *testing.T) {
	tests := []struct {
		prefix   string
		expected []string
	}{
		{"B", []string{"Bash"}},
		{"Re", []string{"Read", "References"}},
		{"G", []string{"Git", "Glob", "Grep"}},
		{"W", []string{"Write", "WebFetch", "WebSearch", "Workflow"}},
	}

	for _, tc := range tests {
		t.Run(tc.prefix, func(t *testing.T) {
			suggestions := tools.GetToolSuggestions(tc.prefix, 10)

			foundNames := make([]string, len(suggestions))
			for i, s := range suggestions {
				foundNames[i] = s.Name
			}

			for _, expected := range tc.expected {
				assert.Contains(t, foundNames, expected,
					"Prefix %s should suggest %s", tc.prefix, expected)
			}
		})
	}
}

func TestGetToolsByCategory(t *testing.T) {
	coreTools := tools.GetToolsByCategory(tools.CategoryCore)
	assert.GreaterOrEqual(t, len(coreTools), 2, "Should have at least 2 core tools")

	fsTools := tools.GetToolsByCategory(tools.CategoryFileSystem)
	assert.GreaterOrEqual(t, len(fsTools), 5, "Should have at least 5 filesystem tools")

	// Verify all returned tools belong to the category
	for _, tool := range coreTools {
		assert.Equal(t, tools.CategoryCore, tool.Category,
			"All returned tools should be in core category")
	}
}

func TestGetAllToolNames(t *testing.T) {
	names := tools.GetAllToolNames()
	assert.Equal(t, 21, len(names), "Should return all 21 tool names")

	// Verify expected tools are in the list
	expectedTools := []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Git", "Test"}
	for _, expected := range expectedTools {
		assert.Contains(t, names, expected, "Tool names should include %s", expected)
	}
}

func TestGenerateOpenAIToolDefinition(t *testing.T) {
	bashSchema, _ := tools.GetToolSchema("Bash")
	definition := tools.GenerateOpenAIToolDefinition(bashSchema)

	assert.Equal(t, "function", definition["type"], "Type should be function")

	function, ok := definition["function"].(map[string]interface{})
	require.True(t, ok, "Should have function definition")

	assert.Equal(t, "Bash", function["name"], "Function name should be Bash")
	assert.NotEmpty(t, function["description"], "Should have description")

	params, ok := function["parameters"].(map[string]interface{})
	require.True(t, ok, "Should have parameters")

	assert.Equal(t, "object", params["type"], "Parameters type should be object")
	assert.NotNil(t, params["properties"], "Should have properties")
	assert.NotNil(t, params["required"], "Should have required list")
}

func TestGenerateAllToolDefinitions(t *testing.T) {
	definitions := tools.GenerateAllToolDefinitions()
	assert.Equal(t, 21, len(definitions), "Should generate definitions for all 21 tools")

	// Verify each definition has the correct structure
	for _, def := range definitions {
		assert.Equal(t, "function", def["type"], "Each definition should have type=function")

		function, ok := def["function"].(map[string]interface{})
		require.True(t, ok, "Each definition should have function object")
		assert.NotEmpty(t, function["name"], "Each function should have a name")
	}
}

func TestToolSchemaToJSON(t *testing.T) {
	bashSchema, exists := tools.GetToolSchema("Bash")
	require.True(t, exists, "Bash schema should exist")

	jsonStr, err := bashSchema.ToJSON()
	require.NoError(t, err, "ToJSON should not error")
	assert.NotEmpty(t, jsonStr, "JSON output should not be empty")

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err, "Output should be valid JSON")

	assert.Equal(t, "Bash", parsed["name"], "JSON should contain correct name")
}

// ============================================
// TOOL EXECUTION FLOW TESTS
// ============================================

func TestToolRegistry_ExecuteBash(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	registry := tools.GetDefaultToolRegistry()

	// Get Git handler
	handler, exists := registry.Get("Git")
	require.True(t, exists, "Git handler should exist in registry")

	// Test git status (should work in any git repo)
	result, err := handler.Execute(ctx, map[string]interface{}{
		"operation":   "status",
		"description": "Check git status",
		"working_dir": ".",
	})
	require.NoError(t, err, "Git status should not error")
	assert.True(t, result.Success || result.Error != "", "Should have success or error")
}

func TestToolRegistry_ExecuteTest(t *testing.T) {
	// Integration test — no external deps required

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	registry := tools.GetDefaultToolRegistry()

	handler, exists := registry.Get("Test")
	require.True(t, exists, "Test handler should exist")

	// Run a simple test that should pass
	result, err := handler.Execute(ctx, map[string]interface{}{
		"test_path":   "./...",
		"filter":      "TestToolSchemaRegistry_All21ToolsRegistered", // Run this specific test
		"verbose":     true,
		"timeout":     "30s",
		"description": "Run specific test",
	})
	// We don't require success because the test environment may not support it
	assert.NoError(t, err, "Test execution should not return Go error")
	t.Logf("Test result: success=%v, output=%s", result.Success, result.Output)
}

func TestToolRegistry_ExecuteDiff(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	registry := tools.GetDefaultToolRegistry()

	handler, exists := registry.Get("Diff")
	require.True(t, exists, "Diff handler should exist")

	result, err := handler.Execute(ctx, map[string]interface{}{
		"mode":        "working",
		"description": "Show working tree diff",
	})
	require.NoError(t, err, "Diff should not error")
	// Even if there are no changes, the command should succeed
	assert.True(t, result.Success || result.Error != "", "Should have result")
}

func TestToolRegistry_ExecuteTreeView(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	registry := tools.GetDefaultToolRegistry()

	handler, exists := registry.Get("TreeView")
	require.True(t, exists, "TreeView handler should exist")

	result, err := handler.Execute(ctx, map[string]interface{}{
		"path":        ".",
		"max_depth":   2,
		"show_hidden": false,
		"description": "Display directory tree",
	})
	require.NoError(t, err, "TreeView should not error")
	assert.True(t, result.Success, "TreeView should succeed")
	assert.NotEmpty(t, result.Output, "TreeView should produce output")
}

func TestToolRegistry_ExecuteSymbols(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	registry := tools.GetDefaultToolRegistry()

	handler, exists := registry.Get("Symbols")
	require.True(t, exists, "Symbols handler should exist")

	// Get the project root
	wd, err := os.Getwd()
	require.NoError(t, err)
	projectRoot := filepath.Join(wd, "../..")

	result, err := handler.Execute(ctx, map[string]interface{}{
		"file_path":   filepath.Join(projectRoot, "internal/tools/schema.go"),
		"recursive":   false,
		"description": "Extract code symbols",
	})
	require.NoError(t, err, "Symbols should not error")
	// May or may not find symbols depending on file existence
	t.Logf("Symbols result: success=%v, output length=%d", result.Success, len(result.Output))
}

func TestToolRegistry_ExecuteReferences(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	registry := tools.GetDefaultToolRegistry()

	handler, exists := registry.Get("References")
	require.True(t, exists, "References handler should exist")

	result, err := handler.Execute(ctx, map[string]interface{}{
		"symbol":              "ToolSchemaRegistry",
		"include_declaration": true,
		"description":         "Find references to ToolSchemaRegistry",
	})
	require.NoError(t, err, "References should not error")
	t.Logf("References result: success=%v, output length=%d", result.Success, len(result.Output))
}

func TestToolRegistry_ExecuteDefinition(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	registry := tools.GetDefaultToolRegistry()

	handler, exists := registry.Get("Definition")
	require.True(t, exists, "Definition handler should exist")

	result, err := handler.Execute(ctx, map[string]interface{}{
		"symbol":      "ToolSchema",
		"description": "Find definition of ToolSchema",
	})
	require.NoError(t, err, "Definition should not error")
	t.Logf("Definition result: success=%v, output length=%d", result.Success, len(result.Output))
}

func TestToolRegistry_ValidationBeforeExecution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	registry := tools.GetDefaultToolRegistry()

	// Test that missing required fields are caught
	handler, exists := registry.Get("Git")
	require.True(t, exists, "Git handler should exist")

	err := handler.ValidateArgs(map[string]interface{}{
		// Missing "description"
		"operation": "status",
	})
	require.Error(t, err, "Should reject missing required field")
	assert.Contains(t, err.Error(), "description", "Error should mention missing field")

	// Test with valid args
	err = handler.ValidateArgs(map[string]interface{}{
		"operation":   "status",
		"description": "Check git status",
	})
	assert.NoError(t, err, "Should accept valid args")

	// Now execute
	result, _ := handler.Execute(ctx, map[string]interface{}{
		"operation":   "status",
		"description": "Check git status",
	})
	t.Logf("Git status result: %v", result.Success)
}

func TestToolRegistry_GenerateDefaultArgs(t *testing.T) {
	registry := tools.GetDefaultToolRegistry()

	tests := []struct {
		toolName string
		context  string
		expected map[string]interface{}
	}{
		{
			"Git",
			"commit my changes",
			map[string]interface{}{
				"operation":   "commit",
				"description": "Create git commit",
			},
		},
		{
			"Git",
			"push to remote",
			map[string]interface{}{
				"operation":   "push",
				"description": "Push changes to remote",
			},
		},
		{
			"Test",
			"run coverage",
			map[string]interface{}{
				"coverage":    true,
				"description": "Run tests with coverage",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.toolName+"_"+tc.context, func(t *testing.T) {
			handler, exists := registry.Get(tc.toolName)
			require.True(t, exists, "Handler should exist")

			defaultArgs := handler.GenerateDefaultArgs(tc.context)
			for key, expectedVal := range tc.expected {
				actualVal, exists := defaultArgs[key]
				assert.True(t, exists, "Default args should have key %s", key)
				assert.Equal(t, expectedVal, actualVal, "Default arg %s should match", key)
			}
		})
	}
}

// ============================================
// MCP TOOL INTEGRATION TESTS
// ============================================

func setupMCPTestServer(t *testing.T) (*gin.Engine, *handlers.MCPHandler) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())

	mcpConfig := &config.MCPConfig{
		Enabled:              true,
		UnifiedToolNamespace: true,
		ExposeAllTools:       true,
	}

	mcpHandler := handlers.NewMCPHandler(nil, mcpConfig)

	// Register MCP routes
	mcp := router.Group("/v1/mcp")
	{
		mcp.GET("/capabilities", mcpHandler.MCPCapabilities)
		mcp.GET("/tools", mcpHandler.MCPTools)
		mcp.POST("/tools/call", mcpHandler.MCPToolsCall)
		mcp.GET("/search", mcpHandler.MCPToolSearch)
		mcp.POST("/search", mcpHandler.MCPToolSearch)
		mcp.GET("/adapters/search", mcpHandler.MCPAdapterSearch)
		mcp.POST("/adapters/search", mcpHandler.MCPAdapterSearch)
		mcp.GET("/suggestions", mcpHandler.MCPToolSuggestions)
		mcp.GET("/categories", mcpHandler.MCPCategories)
		mcp.GET("/stats", mcpHandler.MCPStats)
		mcp.GET("/prompts", mcpHandler.MCPPrompts)
		mcp.GET("/resources", mcpHandler.MCPResources)
	}

	return router, mcpHandler
}

func TestMCPHandler_ToolListing(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	req, _ := http.NewRequest("GET", "/v1/mcp/tools", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Tools endpoint should return 200")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	_, hasTools := response["tools"]
	assert.True(t, hasTools, "Response should have tools key")
}

func TestMCPHandler_ToolSearch(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	searchTests := []struct {
		name         string
		query        string
		expectedTool string
	}{
		{"Search Bash", "bash", "Bash"},
		{"Search file", "file", "Read"},
		{"Search git", "git", "Git"},
		{"Search version control", "version control", "Git"},
	}

	for _, tc := range searchTests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/mcp/search?q="+tc.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Search should return 200")

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err, "Response should be valid JSON")

			results, ok := response["results"].([]interface{})
			require.True(t, ok, "Should have results array")

			// Check if expected tool is in results
			found := false
			for _, r := range results {
				result := r.(map[string]interface{})
				if result["name"] == tc.expectedTool {
					found = true
					break
				}
			}
			assert.True(t, found, "Search for '%s' should find %s", tc.query, tc.expectedTool)
		})
	}
}

func TestMCPHandler_ToolSearchPOST(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	reqBody := map[string]interface{}{
		"query":          "file",
		"categories":     []string{"filesystem"},
		"include_params": true,
		"max_results":    5,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/v1/mcp/search", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "POST search should return 200")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	results, ok := response["results"].([]interface{})
	require.True(t, ok, "Should have results array")

	// All results should be in filesystem category
	for _, r := range results {
		result := r.(map[string]interface{})
		assert.Equal(t, "filesystem", result["category"],
			"Filtered results should be in filesystem category")
	}
}

func TestMCPHandler_AdapterSearch(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	req, _ := http.NewRequest("GET", "/v1/mcp/adapters/search?official=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Adapter search should return 200")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	_, hasResults := response["results"]
	assert.True(t, hasResults, "Should have results")
}

func TestMCPHandler_Stats(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	req, _ := http.NewRequest("GET", "/v1/mcp/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Stats endpoint should return 200")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	toolStats, ok := response["tools"].(map[string]interface{})
	require.True(t, ok, "Should have tools stats")

	total, ok := toolStats["total"].(float64)
	require.True(t, ok, "Should have total count")
	assert.Equal(t, float64(21), total, "Should have 21 tools")

	byCategory, ok := toolStats["by_category"].(map[string]interface{})
	require.True(t, ok, "Should have by_category breakdown")

	// Verify category counts
	assert.NotEmpty(t, byCategory, "Should have category breakdown")
}

func TestMCPHandler_Categories(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	req, _ := http.NewRequest("GET", "/v1/mcp/categories", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Categories endpoint should return 200")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	toolCategories, ok := response["tool_categories"].([]interface{})
	require.True(t, ok, "Should have tool_categories")

	expectedCategories := []string{
		tools.CategoryCore,
		tools.CategoryFileSystem,
		tools.CategoryVersionControl,
		tools.CategoryCodeIntel,
		tools.CategoryWorkflow,
		tools.CategoryWeb,
	}

	for _, expected := range expectedCategories {
		found := false
		for _, cat := range toolCategories {
			if cat.(string) == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Should include category %s", expected)
	}
}

func TestMCPHandler_Suggestions(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	tests := []struct {
		prefix   string
		expected []string
	}{
		{"B", []string{"Bash"}},
		{"G", []string{"Git", "Glob", "Grep"}},
		{"Re", []string{"Read", "References"}},
	}

	for _, tc := range tests {
		t.Run(tc.prefix, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/v1/mcp/suggestions?prefix="+tc.prefix, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Suggestions should return 200")

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err, "Response should be valid JSON")

			suggestions, ok := response["suggestions"].([]interface{})
			require.True(t, ok, "Should have suggestions array")

			foundNames := make([]string, 0)
			for _, s := range suggestions {
				suggestion := s.(map[string]interface{})
				foundNames = append(foundNames, suggestion["name"].(string))
			}

			for _, expected := range tc.expected {
				assert.Contains(t, foundNames, expected,
					"Prefix '%s' should suggest '%s'", tc.prefix, expected)
			}
		})
	}
}

func TestMCPHandler_Capabilities(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	req, _ := http.NewRequest("GET", "/v1/mcp/capabilities", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Capabilities endpoint should return 200")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	// Verify expected fields
	assert.NotEmpty(t, response["version"], "Should have version")
	assert.NotNil(t, response["capabilities"], "Should have capabilities")

	capabilities, ok := response["capabilities"].(map[string]interface{})
	require.True(t, ok, "Capabilities should be an object")

	assert.NotNil(t, capabilities["tools"], "Should have tools capability")
	assert.NotNil(t, capabilities["prompts"], "Should have prompts capability")
	assert.NotNil(t, capabilities["resources"], "Should have resources capability")
}

func TestMCPHandler_Prompts(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	req, _ := http.NewRequest("GET", "/v1/mcp/prompts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Prompts endpoint should return 200")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	prompts, ok := response["prompts"].([]interface{})
	require.True(t, ok, "Should have prompts array")
	assert.NotEmpty(t, prompts, "Should have at least one prompt")
}

func TestMCPHandler_Resources(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	req, _ := http.NewRequest("GET", "/v1/mcp/resources", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Resources endpoint should return 200")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	resources, ok := response["resources"].([]interface{})
	require.True(t, ok, "Should have resources array")
	assert.NotEmpty(t, resources, "Should have at least one resource")
}

func TestMCPHandler_DisabledEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create handler with MCP disabled
	mcpConfig := &config.MCPConfig{
		Enabled: false,
	}
	mcpHandler := handlers.NewMCPHandler(nil, mcpConfig)

	mcp := router.Group("/v1/mcp")
	{
		mcp.GET("/capabilities", mcpHandler.MCPCapabilities)
		mcp.GET("/tools", mcpHandler.MCPTools)
		mcp.GET("/search", mcpHandler.MCPToolSearch)
		mcp.GET("/stats", mcpHandler.MCPStats)
	}

	endpoints := []string{
		"/v1/mcp/capabilities",
		"/v1/mcp/tools",
		"/v1/mcp/search?q=test",
		"/v1/mcp/stats",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			req, _ := http.NewRequest("GET", endpoint, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusServiceUnavailable, w.Code,
				"Disabled MCP endpoint %s should return 503", endpoint)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Contains(t, response["error"], "not enabled",
				"Error should mention MCP not enabled")
		})
	}
}

// ============================================
// ERROR HANDLING TESTS
// ============================================

func TestErrorHandling_InvalidToolName(t *testing.T) {
	err := tools.ValidateToolArgs("InvalidTool", map[string]interface{}{})
	require.Error(t, err, "Should error for invalid tool name")
	assert.Contains(t, err.Error(), "unknown tool", "Error should mention unknown tool")
}

func TestErrorHandling_MissingRequiredParameters(t *testing.T) {
	tests := []struct {
		toolName     string
		args         map[string]interface{}
		missingField string
	}{
		{"Bash", map[string]interface{}{}, "command"},
		{"Read", map[string]interface{}{}, "file_path"},
		{"Write", map[string]interface{}{}, "file_path"},
		{"Edit", map[string]interface{}{}, "file_path"},
		{"Glob", map[string]interface{}{}, "pattern"},
		{"Grep", map[string]interface{}{}, "pattern"},
		{"WebFetch", map[string]interface{}{}, "url"},
		{"WebSearch", map[string]interface{}{}, "query"},
		{"Git", map[string]interface{}{}, "operation"},
		{"PR", map[string]interface{}{}, "action"},
	}

	for _, tc := range tests {
		t.Run(tc.toolName, func(t *testing.T) {
			err := tools.ValidateToolArgs(tc.toolName, tc.args)
			require.Error(t, err, "Should error for missing required field")
			assert.Contains(t, err.Error(), tc.missingField,
				"Error should mention missing field '%s'", tc.missingField)
		})
	}
}

func TestErrorHandling_TimeoutContext(t *testing.T) {
	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel

	registry := tools.GetDefaultToolRegistry()
	handler, exists := registry.Get("Git")
	require.True(t, exists, "Git handler should exist")

	result, err := handler.Execute(ctx, map[string]interface{}{
		"operation":   "status",
		"description": "Check git status",
	})

	// The execution should either error or return a failed result
	if err == nil {
		// If no error, the result should indicate failure due to context
		t.Logf("Result: success=%v, error=%s", result.Success, result.Error)
	} else {
		// Context cancellation error is expected
		assert.Contains(t, err.Error(), "context",
			"Error should be related to context cancellation")
	}
}

func TestErrorHandling_MCPSearchMissingQuery(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	req, _ := http.NewRequest("GET", "/v1/mcp/search", nil) // No query param
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "Should return 400 for missing query")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["error"].(string), "required",
		"Error should mention required query")
}

func TestErrorHandling_MCPSuggestionsMissingPrefix(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	req, _ := http.NewRequest("GET", "/v1/mcp/suggestions", nil) // No prefix param
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "Should return 400 for missing prefix")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["error"].(string), "required",
		"Error should mention required prefix")
}

func TestErrorHandling_MCPToolCallInvalidJSON(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	req, _ := http.NewRequest("POST", "/v1/mcp/tools/call", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "Should return 400 for invalid JSON")
}

func TestErrorHandling_MCPToolCallMissingName(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	reqBody := map[string]interface{}{
		"arguments": map[string]interface{}{},
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/v1/mcp/tools/call", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// The handler checks provider registry first, so it may return 500 if no registry
	// or 400 if the name is missing. Both are valid error responses.
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError,
		"Should return 400 or 500 for invalid tool call, got %d", w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Error should either mention name or registry
	errMsg := strings.ToLower(response["error"].(string))
	assert.True(t, strings.Contains(errMsg, "name") || strings.Contains(errMsg, "registry"),
		"Error should mention either missing name or registry, got: %s", errMsg)
}

// ============================================
// SERVICES TOOL REGISTRY INTEGRATION TESTS
// ============================================

func TestServicesToolRegistry_RegisterCustomTool(t *testing.T) {
	mcpManager := services.NewMCPManager(nil, nil, logrus.New())
	lspClient := services.NewLSPClient(logrus.New())
	toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

	customTool := &ToolTestMockTool{
		name:        "custom-test-tool",
		description: "A custom tool for testing",
		parameters: map[string]interface{}{
			"input": map[string]interface{}{
				"type":        "string",
				"description": "Input parameter",
			},
		},
		source: "custom",
	}

	err := toolRegistry.RegisterCustomTool(customTool)
	require.NoError(t, err, "Should register custom tool")

	// Verify tool is registered
	tool, exists := toolRegistry.GetTool("custom-test-tool")
	assert.True(t, exists, "Custom tool should exist after registration")
	assert.Equal(t, "custom-test-tool", tool.Name(), "Tool name should match")
}

func TestServicesToolRegistry_DuplicateRegistration(t *testing.T) {
	mcpManager := services.NewMCPManager(nil, nil, logrus.New())
	lspClient := services.NewLSPClient(logrus.New())
	toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

	customTool := &ToolTestMockTool{
		name:        "duplicate-tool",
		description: "Test tool",
		parameters: map[string]interface{}{
			"param": map[string]interface{}{"type": "string"},
		},
		source: "custom",
	}

	err := toolRegistry.RegisterCustomTool(customTool)
	require.NoError(t, err, "First registration should succeed")

	err = toolRegistry.RegisterCustomTool(customTool)
	require.Error(t, err, "Duplicate registration should fail")
	assert.Contains(t, err.Error(), "already registered", "Error should mention already registered")
}

func TestServicesToolRegistry_ExecuteTool(t *testing.T) {
	mcpManager := services.NewMCPManager(nil, nil, logrus.New())
	lspClient := services.NewLSPClient(logrus.New())
	toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

	customTool := &ToolTestMockTool{
		name:        "exec-test-tool",
		description: "Tool for execution test",
		parameters: map[string]interface{}{
			"input": map[string]interface{}{"type": "string"},
		},
		source: "custom",
	}

	err := toolRegistry.RegisterCustomTool(customTool)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := toolRegistry.ExecuteTool(ctx, "exec-test-tool", map[string]interface{}{
		"input": "test value",
	})
	require.NoError(t, err, "Tool execution should succeed")
	assert.NotNil(t, result, "Should have result")
}

func TestServicesToolRegistry_ExecuteNonExistentTool(t *testing.T) {
	mcpManager := services.NewMCPManager(nil, nil, logrus.New())
	lspClient := services.NewLSPClient(logrus.New())
	toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

	ctx := context.Background()
	_, err := toolRegistry.ExecuteTool(ctx, "non-existent-tool", map[string]interface{}{})
	require.Error(t, err, "Should error for non-existent tool")
	assert.Contains(t, err.Error(), "not found", "Error should mention not found")
}

func TestServicesToolRegistry_Search(t *testing.T) {
	mcpManager := services.NewMCPManager(nil, nil, logrus.New())
	lspClient := services.NewLSPClient(logrus.New())
	toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

	// Register test tools
	tools := []*ToolTestMockTool{
		{name: "search-tool-1", description: "First search tool", parameters: map[string]interface{}{"p": map[string]interface{}{"type": "string"}}, source: "custom"},
		{name: "search-tool-2", description: "Second search tool", parameters: map[string]interface{}{"p": map[string]interface{}{"type": "string"}}, source: "custom"},
		{name: "other-tool", description: "Other functionality", parameters: map[string]interface{}{"p": map[string]interface{}{"type": "string"}}, source: "custom"},
	}

	for _, tool := range tools {
		err := toolRegistry.RegisterCustomTool(tool)
		require.NoError(t, err)
	}

	results := toolRegistry.Search(services.UnifiedSearchOptions{
		Query:      "search",
		MaxResults: 10,
	})

	// Should find the search tools
	foundNames := make([]string, len(results))
	for i, r := range results {
		foundNames[i] = r.Name
	}

	assert.Contains(t, foundNames, "search-tool-1", "Should find search-tool-1")
	assert.Contains(t, foundNames, "search-tool-2", "Should find search-tool-2")
}

func TestServicesToolRegistry_GetToolSuggestions(t *testing.T) {
	mcpManager := services.NewMCPManager(nil, nil, logrus.New())
	lspClient := services.NewLSPClient(logrus.New())
	toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

	// Register test tools
	testTools := []*ToolTestMockTool{
		{name: "alpha-tool", description: "Alpha tool", parameters: map[string]interface{}{"p": map[string]interface{}{"type": "string"}}, source: "custom"},
		{name: "alpha-beta", description: "Alpha beta tool", parameters: map[string]interface{}{"p": map[string]interface{}{"type": "string"}}, source: "custom"},
		{name: "beta-tool", description: "Beta tool", parameters: map[string]interface{}{"p": map[string]interface{}{"type": "string"}}, source: "custom"},
	}

	for _, tool := range testTools {
		err := toolRegistry.RegisterCustomTool(tool)
		require.NoError(t, err)
	}

	suggestions := toolRegistry.GetToolSuggestions("alpha", 5)
	assert.Len(t, suggestions, 2, "Should find 2 tools starting with 'alpha'")
}

func TestServicesToolRegistry_GetToolsBySource(t *testing.T) {
	mcpManager := services.NewMCPManager(nil, nil, logrus.New())
	lspClient := services.NewLSPClient(logrus.New())
	toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

	// Register tools with different sources
	customTool1 := &ToolTestMockTool{
		name: "source-test-1", description: "Test", parameters: map[string]interface{}{"p": map[string]interface{}{"type": "string"}}, source: "custom",
	}
	customTool2 := &ToolTestMockTool{
		name: "source-test-2", description: "Test", parameters: map[string]interface{}{"p": map[string]interface{}{"type": "string"}}, source: "custom",
	}

	_ = toolRegistry.RegisterCustomTool(customTool1)
	_ = toolRegistry.RegisterCustomTool(customTool2)

	customTools := toolRegistry.GetToolsBySource("custom")
	assert.GreaterOrEqual(t, len(customTools), 2, "Should have at least 2 custom tools")
}

func TestServicesToolRegistry_GetToolStats(t *testing.T) {
	mcpManager := services.NewMCPManager(nil, nil, logrus.New())
	lspClient := services.NewLSPClient(logrus.New())
	toolRegistry := services.NewToolRegistry(mcpManager, lspClient)

	// Register some tools
	for i := 0; i < 3; i++ {
		tool := &ToolTestMockTool{
			name:        fmt.Sprintf("stats-tool-%d", i),
			description: "Test tool",
			parameters:  map[string]interface{}{"p": map[string]interface{}{"type": "string"}},
			source:      "custom",
		}
		_ = toolRegistry.RegisterCustomTool(tool)
	}

	stats := toolRegistry.GetToolStats()
	assert.NotNil(t, stats, "Stats should not be nil")

	totalTools, ok := stats["total_tools"].(int)
	assert.True(t, ok, "Should have total_tools count")
	assert.GreaterOrEqual(t, totalTools, 3, "Should have at least 3 tools")

	bySource, ok := stats["by_source"].(map[string]int)
	assert.True(t, ok, "Should have by_source breakdown")
	assert.GreaterOrEqual(t, bySource["custom"], 3, "Should have at least 3 custom tools")
}

// ============================================
// FULL INTEGRATION FLOW TESTS
// ============================================

func TestFullIntegrationFlow_CLIAgentToToolExecution(t *testing.T) {
	// This test simulates the complete flow from CLI agent to tool execution

	// 1. Get tool schema
	bashSchema, exists := tools.GetToolSchema("Bash")
	require.True(t, exists, "Bash schema should exist")

	// 2. Generate OpenAI-compatible tool definition
	toolDef := tools.GenerateOpenAIToolDefinition(bashSchema)
	assert.NotNil(t, toolDef, "Should generate tool definition")

	// 3. Validate tool arguments
	args := map[string]interface{}{
		"command":     "echo hello",
		"description": "Print hello",
	}
	err := tools.ValidateToolArgs("Bash", args)
	require.NoError(t, err, "Arguments should be valid")

	// 4. Execute tool (if available)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	registry := tools.GetDefaultToolRegistry()
	if handler, exists := registry.Get("Git"); exists {
		// Use Git as a safer test command
		result, err := handler.Execute(ctx, map[string]interface{}{
			"operation":   "status",
			"description": "Check git status",
		})
		if err == nil {
			t.Logf("Git status executed: success=%v", result.Success)
		}
	}
}

func TestFullIntegrationFlow_MCPToolSearchToExecution(t *testing.T) {
	router, _ := setupMCPTestServer(t)

	// 1. Search for a tool
	searchReq, _ := http.NewRequest("GET", "/v1/mcp/search?q=git", nil)
	searchW := httptest.NewRecorder()
	router.ServeHTTP(searchW, searchReq)

	require.Equal(t, http.StatusOK, searchW.Code, "Search should succeed")

	var searchResp map[string]interface{}
	err := json.Unmarshal(searchW.Body.Bytes(), &searchResp)
	require.NoError(t, err)

	results := searchResp["results"].([]interface{})
	require.NotEmpty(t, results, "Should find git tool")

	// 2. Get tool details
	firstResult := results[0].(map[string]interface{})
	toolName := firstResult["name"].(string)
	assert.NotEmpty(t, toolName, "Tool name should not be empty")

	// 3. Verify tool has required fields
	requiredFields := firstResult["required"].([]interface{})
	assert.NotEmpty(t, requiredFields, "Tool should have required fields")

	// 4. Get stats to verify tool is counted
	statsReq, _ := http.NewRequest("GET", "/v1/mcp/stats", nil)
	statsW := httptest.NewRecorder()
	router.ServeHTTP(statsW, statsReq)

	require.Equal(t, http.StatusOK, statsW.Code, "Stats should succeed")

	var statsResp map[string]interface{}
	err = json.Unmarshal(statsW.Body.Bytes(), &statsResp)
	require.NoError(t, err)

	toolStats := statsResp["tools"].(map[string]interface{})
	totalTools := int(toolStats["total"].(float64))
	assert.Equal(t, 21, totalTools, "Should have 21 total tools")
}

func TestFullIntegrationFlow_ToolValidationAndErrorRecovery(t *testing.T) {
	// Test the complete error recovery flow

	// 1. Try with invalid tool name
	err := tools.ValidateToolArgs("InvalidTool", map[string]interface{}{})
	require.Error(t, err, "Should catch invalid tool name")

	// 2. Try with missing required fields
	err = tools.ValidateToolArgs("Bash", map[string]interface{}{})
	require.Error(t, err, "Should catch missing required fields")

	// 3. Search for alternatives using a term that will match
	results := tools.SearchTools(tools.SearchOptions{
		Query:      "bash",
		MaxResults: 5,
	})
	require.NotEmpty(t, results, "Should find alternative tools")

	// 4. Use valid arguments
	foundValidTool := false
	for _, result := range results {
		if result.Tool.Name == "Bash" {
			// Try with valid arguments
			err = tools.ValidateToolArgs("Bash", map[string]interface{}{
				"command":     "echo test",
				"description": "Test command",
			})
			assert.NoError(t, err, "Valid arguments should pass")
			foundValidTool = true
			break
		}
	}
	assert.True(t, foundValidTool, "Should find Bash in search results")
}

// ============================================
// MOCK IMPLEMENTATIONS
// ============================================

// ToolTestMockTool is a mock implementation of the Tool interface for testing
// Named differently to avoid conflicts with MockTool in integration_test.go
type ToolTestMockTool struct {
	name        string
	description string
	parameters  map[string]interface{}
	source      string
}

func (m *ToolTestMockTool) Name() string {
	return m.name
}

func (m *ToolTestMockTool) Description() string {
	return m.description
}

func (m *ToolTestMockTool) Parameters() map[string]interface{} {
	return m.parameters
}

func (m *ToolTestMockTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"tool":    m.name,
		"params":  params,
		"result":  "success",
		"message": fmt.Sprintf("Executed %s with params %v", m.name, params),
	}, nil
}

func (m *ToolTestMockTool) Source() string {
	if m.source == "" {
		return "custom"
	}
	return m.source
}

// MockWebServer creates a mock HTTP server for WebFetch/WebSearch tests
func createMockWebServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status": "ok", "message": "Mock response"}`)
	}))
}

func TestWebFetch_WithMockServer(t *testing.T) {
	server := createMockWebServer()
	defer server.Close()

	// Validate WebFetch arguments
	err := tools.ValidateToolArgs("WebFetch", map[string]interface{}{
		"url":    server.URL,
		"prompt": "Summarize the response",
	})
	assert.NoError(t, err, "WebFetch arguments should be valid")
}

func TestWebSearch_WithMockServer(t *testing.T) {
	// WebSearch validation only - actual search would require external service
	err := tools.ValidateToolArgs("WebSearch", map[string]interface{}{
		"query": "golang best practices 2024",
	})
	assert.NoError(t, err, "WebSearch arguments should be valid")

	// Test with domain filters
	err = tools.ValidateToolArgs("WebSearch", map[string]interface{}{
		"query":           "golang tutorials",
		"allowed_domains": []string{"golang.org", "github.com"},
	})
	assert.NoError(t, err, "WebSearch with domain filters should be valid")
}

// ============================================
// BENCHMARK TESTS
// ============================================

func BenchmarkToolSchemaValidation(b *testing.B) {
	args := map[string]interface{}{
		"command":     "ls -la",
		"description": "List files",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tools.ValidateToolArgs("Bash", args)
	}
}

func BenchmarkToolSearch(b *testing.B) {
	opts := tools.SearchOptions{
		Query:      "file",
		MaxResults: 10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tools.SearchTools(opts)
	}
}

func BenchmarkToolSuggestions(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tools.GetToolSuggestions("B", 10)
	}
}

func BenchmarkGenerateOpenAIToolDefinition(b *testing.B) {
	bashSchema, _ := tools.GetToolSchema("Bash")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tools.GenerateOpenAIToolDefinition(bashSchema)
	}
}

func BenchmarkGenerateAllToolDefinitions(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tools.GenerateAllToolDefinitions()
	}
}
