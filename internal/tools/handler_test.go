package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// ToolRegistry Tests
// ============================================================================

func TestNewToolRegistry(t *testing.T) {
	registry := NewToolRegistry()
	require.NotNil(t, registry)
	assert.NotNil(t, registry.handlers)
}

func TestToolRegistry_Register(t *testing.T) {
	registry := NewToolRegistry()
	handler := &GitHandler{}

	registry.Register(handler)

	// Should be retrievable
	h, ok := registry.Get("git")
	assert.True(t, ok)
	assert.NotNil(t, h)
	assert.Equal(t, "Git", h.Name())
}

func TestToolRegistry_Get_NotFound(t *testing.T) {
	registry := NewToolRegistry()

	h, ok := registry.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, h)
}

func TestToolRegistry_Get_CaseInsensitive(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(&GitHandler{})

	// All these should find the same handler
	testCases := []string{"git", "Git", "GIT", "gIt"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			h, ok := registry.Get(tc)
			assert.True(t, ok, "Should find handler for %s", tc)
			if ok {
				assert.Equal(t, "Git", h.Name())
			}
		})
	}
}

func TestToolRegistry_Execute_UnknownTool(t *testing.T) {
	registry := NewToolRegistry()
	ctx := context.Background()

	result, err := registry.Execute(ctx, "unknowntool", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unknown tool")
}

func TestToolRegistry_Execute_ValidationError(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(&GitHandler{})
	ctx := context.Background()

	// Git requires "operation" and "description" fields
	result, err := registry.Execute(ctx, "git", map[string]interface{}{})
	assert.Error(t, err)
	assert.False(t, result.Success)
}

func TestDefaultToolRegistry(t *testing.T) {
	// Verify DefaultToolRegistry has handlers registered via init()
	expectedHandlers := []string{
		"git", "test", "lint", "diff", "treeview",
		"fileinfo", "symbols", "references", "definition",
		"pr", "issue", "workflow",
	}

	for _, name := range expectedHandlers {
		t.Run(name, func(t *testing.T) {
			h, ok := DefaultToolRegistry.Get(name)
			assert.True(t, ok, "DefaultToolRegistry should have %s handler", name)
			assert.NotNil(t, h)
		})
	}
}

// ============================================================================
// GitHandler Tests
// ============================================================================

func TestGitHandler_Name(t *testing.T) {
	handler := &GitHandler{}
	assert.Equal(t, "Git", handler.Name())
}

func TestGitHandler_ValidateArgs_Valid(t *testing.T) {
	handler := &GitHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"operation":   "status",
		"description": "Check git status",
	})
	assert.NoError(t, err)
}

func TestGitHandler_ValidateArgs_MissingRequired(t *testing.T) {
	handler := &GitHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"operation": "status",
	})
	assert.Error(t, err)
}

func TestGitHandler_GenerateDefaultArgs(t *testing.T) {
	handler := &GitHandler{}

	testCases := []struct {
		context           string
		expectedOperation string
	}{
		{"Check the status", "status"},
		{"I want to commit changes", "commit"},
		{"push to remote", "push"},
		{"pull latest changes", "pull"},
		{"create a new branch", "branch"},
		{"checkout main", "checkout"},
		{"merge the code", "merge"},
		{"show diff", "diff"},
		{"view log", "log"},
		{"stash my changes", "stash"},
		{"random context", "status"}, // default
	}

	for _, tc := range testCases {
		t.Run(tc.context, func(t *testing.T) {
			args := handler.GenerateDefaultArgs(tc.context)
			assert.Equal(t, tc.expectedOperation, args["operation"])
			assert.NotEmpty(t, args["description"])
		})
	}
}

// ============================================================================
// TestHandler Tests
// ============================================================================

func TestTestHandler_Name(t *testing.T) {
	handler := &TestHandler{}
	assert.Equal(t, "Test", handler.Name())
}

func TestTestHandler_ValidateArgs_Valid(t *testing.T) {
	handler := &TestHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Run tests",
	})
	assert.NoError(t, err)
}

func TestTestHandler_GenerateDefaultArgs(t *testing.T) {
	handler := &TestHandler{}

	testCases := []struct {
		context          string
		expectedTestType string
		expectedCoverage bool
	}{
		{"run all tests", "all", false},
		{"run unit tests", "unit", false},
		{"run integration tests", "integration", false},
		{"run e2e tests", "e2e", false},
		{"run tests with coverage", "all", true},
		{"run unit tests with coverage", "unit", true},
	}

	for _, tc := range testCases {
		t.Run(tc.context, func(t *testing.T) {
			args := handler.GenerateDefaultArgs(tc.context)
			assert.Equal(t, tc.expectedTestType, args["test_type"])
			assert.Equal(t, tc.expectedCoverage, args["coverage"])
			assert.NotEmpty(t, args["test_path"])
			assert.NotEmpty(t, args["description"])
		})
	}
}

// ============================================================================
// LintHandler Tests
// ============================================================================

func TestLintHandler_Name(t *testing.T) {
	handler := &LintHandler{}
	assert.Equal(t, "Lint", handler.Name())
}

func TestLintHandler_ValidateArgs_Valid(t *testing.T) {
	handler := &LintHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Run linting",
	})
	assert.NoError(t, err)
}

func TestLintHandler_GenerateDefaultArgs(t *testing.T) {
	handler := &LintHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, "./...", args["path"])
	assert.Equal(t, "auto", args["linter"])
	assert.Equal(t, false, args["auto_fix"])
	assert.NotEmpty(t, args["description"])
}

// ============================================================================
// DiffHandler Tests
// ============================================================================

func TestDiffHandler_Name(t *testing.T) {
	handler := &DiffHandler{}
	assert.Equal(t, "Diff", handler.Name())
}

func TestDiffHandler_ValidateArgs_Valid(t *testing.T) {
	handler := &DiffHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Show diff",
	})
	assert.NoError(t, err)
}

func TestDiffHandler_GenerateDefaultArgs(t *testing.T) {
	handler := &DiffHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, "working", args["mode"])
	assert.NotEmpty(t, args["description"])
}

// ============================================================================
// TreeViewHandler Tests
// ============================================================================

func TestTreeViewHandler_Name(t *testing.T) {
	handler := &TreeViewHandler{}
	assert.Equal(t, "TreeView", handler.Name())
}

func TestTreeViewHandler_ValidateArgs_Valid(t *testing.T) {
	handler := &TreeViewHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Show tree",
	})
	assert.NoError(t, err)
}

func TestTreeViewHandler_GenerateDefaultArgs(t *testing.T) {
	handler := &TreeViewHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, ".", args["path"])
	assert.Equal(t, 3, args["max_depth"])
	assert.Equal(t, false, args["show_hidden"])
	assert.NotEmpty(t, args["description"])
}

// ============================================================================
// FileInfoHandler Tests
// ============================================================================

func TestFileInfoHandler_Name(t *testing.T) {
	handler := &FileInfoHandler{}
	assert.Equal(t, "FileInfo", handler.Name())
}

func TestFileInfoHandler_ValidateArgs_Valid(t *testing.T) {
	handler := &FileInfoHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"file_path":   "test.go",
		"description": "Get file info",
	})
	assert.NoError(t, err)
}

func TestFileInfoHandler_ValidateArgs_MissingFilePath(t *testing.T) {
	handler := &FileInfoHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Get file info",
	})
	assert.Error(t, err)
}

func TestFileInfoHandler_GenerateDefaultArgs(t *testing.T) {
	handler := &FileInfoHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, "README.md", args["file_path"])
	assert.Equal(t, true, args["include_stats"])
	assert.Equal(t, false, args["include_git"])
	assert.NotEmpty(t, args["description"])
}

// ============================================================================
// SymbolsHandler Tests
// ============================================================================

func TestSymbolsHandler_Name(t *testing.T) {
	handler := &SymbolsHandler{}
	assert.Equal(t, "Symbols", handler.Name())
}

func TestSymbolsHandler_ValidateArgs_Valid(t *testing.T) {
	handler := &SymbolsHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Extract symbols",
	})
	assert.NoError(t, err)
}

func TestSymbolsHandler_GenerateDefaultArgs(t *testing.T) {
	handler := &SymbolsHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, ".", args["file_path"])
	assert.Equal(t, false, args["recursive"])
	assert.NotEmpty(t, args["description"])
}

// ============================================================================
// ReferencesHandler Tests
// ============================================================================

func TestReferencesHandler_Name(t *testing.T) {
	handler := &ReferencesHandler{}
	assert.Equal(t, "References", handler.Name())
}

func TestReferencesHandler_ValidateArgs_Valid(t *testing.T) {
	handler := &ReferencesHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"symbol":      "TestFunction",
		"description": "Find references",
	})
	assert.NoError(t, err)
}

func TestReferencesHandler_ValidateArgs_MissingSymbol(t *testing.T) {
	handler := &ReferencesHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Find references",
	})
	assert.Error(t, err)
}

func TestReferencesHandler_GenerateDefaultArgs(t *testing.T) {
	handler := &ReferencesHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, "main", args["symbol"])
	assert.Equal(t, true, args["include_declaration"])
	assert.NotEmpty(t, args["description"])
}

// ============================================================================
// DefinitionHandler Tests
// ============================================================================

func TestDefinitionHandler_Name(t *testing.T) {
	handler := &DefinitionHandler{}
	assert.Equal(t, "Definition", handler.Name())
}

func TestDefinitionHandler_ValidateArgs_Valid(t *testing.T) {
	handler := &DefinitionHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"symbol":      "TestFunction",
		"description": "Find definition",
	})
	assert.NoError(t, err)
}

func TestDefinitionHandler_ValidateArgs_MissingSymbol(t *testing.T) {
	handler := &DefinitionHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Find definition",
	})
	assert.Error(t, err)
}

func TestDefinitionHandler_GenerateDefaultArgs(t *testing.T) {
	handler := &DefinitionHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, "main", args["symbol"])
	assert.NotEmpty(t, args["description"])
}

// ============================================================================
// PRHandler Tests
// ============================================================================

func TestPRHandler_Name(t *testing.T) {
	handler := &PRHandler{}
	assert.Equal(t, "PR", handler.Name())
}

func TestPRHandler_ValidateArgs_Valid(t *testing.T) {
	handler := &PRHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"action":      "list",
		"description": "List PRs",
	})
	assert.NoError(t, err)
}

func TestPRHandler_ValidateArgs_MissingAction(t *testing.T) {
	handler := &PRHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "List PRs",
	})
	assert.Error(t, err)
}

func TestPRHandler_GenerateDefaultArgs(t *testing.T) {
	handler := &PRHandler{}

	testCases := []struct {
		context        string
		expectedAction string
	}{
		{"list all PRs", "list"},
		{"create a PR", "create"},
		{"merge the PR", "merge"},
		{"view the PR", "view"},
		{"random context", "list"}, // default
	}

	for _, tc := range testCases {
		t.Run(tc.context, func(t *testing.T) {
			args := handler.GenerateDefaultArgs(tc.context)
			assert.Equal(t, tc.expectedAction, args["action"])
			assert.NotEmpty(t, args["description"])
		})
	}
}

// ============================================================================
// IssueHandler Tests
// ============================================================================

func TestIssueHandler_Name(t *testing.T) {
	handler := &IssueHandler{}
	assert.Equal(t, "Issue", handler.Name())
}

func TestIssueHandler_ValidateArgs_Valid(t *testing.T) {
	handler := &IssueHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"action":      "list",
		"description": "List issues",
	})
	assert.NoError(t, err)
}

func TestIssueHandler_ValidateArgs_MissingAction(t *testing.T) {
	handler := &IssueHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "List issues",
	})
	assert.Error(t, err)
}

func TestIssueHandler_GenerateDefaultArgs(t *testing.T) {
	handler := &IssueHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, "list", args["action"])
	assert.NotEmpty(t, args["description"])
}

// ============================================================================
// WorkflowHandler Tests
// ============================================================================

func TestWorkflowHandler_Name(t *testing.T) {
	handler := &WorkflowHandler{}
	assert.Equal(t, "Workflow", handler.Name())
}

func TestWorkflowHandler_ValidateArgs_Valid(t *testing.T) {
	handler := &WorkflowHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"action":      "list",
		"description": "List workflows",
	})
	assert.NoError(t, err)
}

func TestWorkflowHandler_ValidateArgs_MissingAction(t *testing.T) {
	handler := &WorkflowHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "List workflows",
	})
	assert.Error(t, err)
}

func TestWorkflowHandler_GenerateDefaultArgs(t *testing.T) {
	handler := &WorkflowHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, "list", args["action"])
	assert.NotEmpty(t, args["description"])
}

// ============================================================================
// ToolResult Tests
// ============================================================================

func TestToolResult_Structure(t *testing.T) {
	// Test successful result
	successResult := ToolResult{
		Success: true,
		Output:  "Command output",
		Data:    map[string]string{"key": "value"},
	}
	assert.True(t, successResult.Success)
	assert.Equal(t, "Command output", successResult.Output)
	assert.Empty(t, successResult.Error)

	// Test failure result
	failResult := ToolResult{
		Success: false,
		Output:  "Partial output",
		Error:   "Command failed",
	}
	assert.False(t, failResult.Success)
	assert.Equal(t, "Command failed", failResult.Error)
}
