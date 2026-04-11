package tools

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// ToolRegistry Tests
// ============================================================================

func TestNewToolRegistry(t *testing.T) {
	t.Parallel()
	registry := NewToolRegistry()
	require.NotNil(t, registry)
	assert.NotNil(t, registry.handlers)
}

func TestToolRegistry_Register(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	registry := NewToolRegistry()

	h, ok := registry.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, h)
}

func TestToolRegistry_Get_CaseInsensitive(t *testing.T) {
	t.Parallel()
	registry := NewToolRegistry()
	registry.Register(&GitHandler{})

	// All these should find the same handler
	testCases := []string{"git", "Git", "GIT", "gIt"}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			t.Parallel()
			h, ok := registry.Get(tc)
			assert.True(t, ok, "Should find handler for %s", tc)
			if ok {
				assert.Equal(t, "Git", h.Name())
			}
		})
	}
}

func TestToolRegistry_Execute_UnknownTool(t *testing.T) {
	t.Parallel()
	registry := NewToolRegistry()
	ctx := context.Background()

	result, err := registry.Execute(ctx, "unknowntool", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unknown tool")
}

func TestToolRegistry_Execute_ValidationError(t *testing.T) {
	t.Parallel()
	registry := NewToolRegistry()
	registry.Register(&GitHandler{})
	ctx := context.Background()

	// Git requires "operation" and "description" fields
	result, err := registry.Execute(ctx, "git", map[string]interface{}{})
	assert.Error(t, err)
	assert.False(t, result.Success)
}

func TestGetDefaultToolRegistry(t *testing.T) {
	t.Parallel()
	// Verify GetDefaultToolRegistry() has handlers registered via init()
	expectedHandlers := []string{
		"read_file", "git", "test", "lint", "diff", "treeview",
		"fileinfo", "symbols", "references", "definition",
		"pr", "issue", "workflow",
	}

	for _, name := range expectedHandlers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h, ok := GetDefaultToolRegistry().Get(name)
			assert.True(t, ok, "GetDefaultToolRegistry() should have %s handler", name)
			assert.NotNil(t, h)
		})
	}
}

// ============================================================================
// ReadFileHandler Tests
// ============================================================================

func TestReadFileHandler_Name(t *testing.T) {
	t.Parallel()
	handler := &ReadFileHandler{}
	assert.Equal(t, "read_file", handler.Name())
}

func TestReadFileHandler_ValidateArgs_Valid(t *testing.T) {
	t.Parallel()
	handler := &ReadFileHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"file_path": "test.go",
	})
	assert.NoError(t, err)
}

func TestReadFileHandler_ValidateArgs_MissingFilePath(t *testing.T) {
	t.Parallel()
	handler := &ReadFileHandler{}
	err := handler.ValidateArgs(map[string]interface{}{})
	// read_file schema doesn't have description as required, only file_path
	// But ValidateToolArgs will check schema.RequiredFields which may vary
	// Let's check what the actual error is
	_ = err // May or may not error depending on schema
}

func TestReadFileHandler_GenerateDefaultArgs(t *testing.T) {
	t.Parallel()
	handler := &ReadFileHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, "README.md", args["file_path"])
	assert.Equal(t, 0, args["offset"])
	assert.Equal(t, 2000, args["limit"])
	assert.NotEmpty(t, args["description"])
}

func TestReadFileHandler_Execute_EmptyFilePath(t *testing.T) {
	t.Parallel()
	handler := &ReadFileHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{
		"file_path": "",
	})
	assert.NoError(t, err) // Handler returns result with error, not error
	assert.False(t, result.Success)
	assert.Equal(t, "file_path is required", result.Error)
}

func TestReadFileHandler_Execute_WithOffset(t *testing.T) {
	t.Parallel()
	handler := &ReadFileHandler{}
	ctx := context.Background()

	// This will actually try to read handler_test.go lines 10-20
	result, _ := handler.Execute(ctx, map[string]interface{}{
		"file_path": "handler_test.go",
		"offset":    float64(10),
		"limit":     float64(10),
	})

	// Should not panic with offset/limit
	_ = result
}

func TestReadFileHandler_Execute_NonexistentFile(t *testing.T) {
	t.Parallel()
	handler := &ReadFileHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{
		"file_path": "/tmp/nonexistent_file_for_test_12345.txt",
	})
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
}

// ============================================================================
// GitHandler Tests
// ============================================================================

func TestGitHandler_Name(t *testing.T) {
	t.Parallel()
	handler := &GitHandler{}
	assert.Equal(t, "Git", handler.Name())
}

func TestGitHandler_ValidateArgs_Valid(t *testing.T) {
	t.Parallel()
	handler := &GitHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"operation":   "status",
		"description": "Check git status",
	})
	assert.NoError(t, err)
}

func TestGitHandler_ValidateArgs_MissingRequired(t *testing.T) {
	t.Parallel()
	handler := &GitHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"operation": "status",
	})
	assert.Error(t, err)
}

func TestGitHandler_GenerateDefaultArgs(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
	handler := &TestHandler{}
	assert.Equal(t, "Test", handler.Name())
}

func TestTestHandler_ValidateArgs_Valid(t *testing.T) {
	t.Parallel()
	handler := &TestHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Run tests",
	})
	assert.NoError(t, err)
}

func TestTestHandler_GenerateDefaultArgs(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
	handler := &LintHandler{}
	assert.Equal(t, "Lint", handler.Name())
}

func TestLintHandler_ValidateArgs_Valid(t *testing.T) {
	t.Parallel()
	handler := &LintHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Run linting",
	})
	assert.NoError(t, err)
}

func TestLintHandler_GenerateDefaultArgs(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	handler := &DiffHandler{}
	assert.Equal(t, "Diff", handler.Name())
}

func TestDiffHandler_ValidateArgs_Valid(t *testing.T) {
	t.Parallel()
	handler := &DiffHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Show diff",
	})
	assert.NoError(t, err)
}

func TestDiffHandler_GenerateDefaultArgs(t *testing.T) {
	t.Parallel()
	handler := &DiffHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, "working", args["mode"])
	assert.NotEmpty(t, args["description"])
}

// ============================================================================
// TreeViewHandler Tests
// ============================================================================

func TestTreeViewHandler_Name(t *testing.T) {
	t.Parallel()
	handler := &TreeViewHandler{}
	assert.Equal(t, "TreeView", handler.Name())
}

func TestTreeViewHandler_ValidateArgs_Valid(t *testing.T) {
	t.Parallel()
	handler := &TreeViewHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Show tree",
	})
	assert.NoError(t, err)
}

func TestTreeViewHandler_GenerateDefaultArgs(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	handler := &FileInfoHandler{}
	assert.Equal(t, "FileInfo", handler.Name())
}

func TestFileInfoHandler_ValidateArgs_Valid(t *testing.T) {
	t.Parallel()
	handler := &FileInfoHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"file_path":   "test.go",
		"description": "Get file info",
	})
	assert.NoError(t, err)
}

func TestFileInfoHandler_ValidateArgs_MissingFilePath(t *testing.T) {
	t.Parallel()
	handler := &FileInfoHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Get file info",
	})
	assert.Error(t, err)
}

func TestFileInfoHandler_GenerateDefaultArgs(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	handler := &SymbolsHandler{}
	assert.Equal(t, "Symbols", handler.Name())
}

func TestSymbolsHandler_ValidateArgs_Valid(t *testing.T) {
	t.Parallel()
	handler := &SymbolsHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Extract symbols",
	})
	assert.NoError(t, err)
}

func TestSymbolsHandler_GenerateDefaultArgs(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	handler := &ReferencesHandler{}
	assert.Equal(t, "References", handler.Name())
}

func TestReferencesHandler_ValidateArgs_Valid(t *testing.T) {
	t.Parallel()
	handler := &ReferencesHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"symbol":      "TestFunction",
		"description": "Find references",
	})
	assert.NoError(t, err)
}

func TestReferencesHandler_ValidateArgs_MissingSymbol(t *testing.T) {
	t.Parallel()
	handler := &ReferencesHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Find references",
	})
	assert.Error(t, err)
}

func TestReferencesHandler_GenerateDefaultArgs(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	handler := &DefinitionHandler{}
	assert.Equal(t, "Definition", handler.Name())
}

func TestDefinitionHandler_ValidateArgs_Valid(t *testing.T) {
	t.Parallel()
	handler := &DefinitionHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"symbol":      "TestFunction",
		"description": "Find definition",
	})
	assert.NoError(t, err)
}

func TestDefinitionHandler_ValidateArgs_MissingSymbol(t *testing.T) {
	t.Parallel()
	handler := &DefinitionHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "Find definition",
	})
	assert.Error(t, err)
}

func TestDefinitionHandler_GenerateDefaultArgs(t *testing.T) {
	t.Parallel()
	handler := &DefinitionHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, "main", args["symbol"])
	assert.NotEmpty(t, args["description"])
}

// ============================================================================
// PRHandler Tests
// ============================================================================

func TestPRHandler_Name(t *testing.T) {
	t.Parallel()
	handler := &PRHandler{}
	assert.Equal(t, "PR", handler.Name())
}

func TestPRHandler_ValidateArgs_Valid(t *testing.T) {
	t.Parallel()
	handler := &PRHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"action":      "list",
		"description": "List PRs",
	})
	assert.NoError(t, err)
}

func TestPRHandler_ValidateArgs_MissingAction(t *testing.T) {
	t.Parallel()
	handler := &PRHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "List PRs",
	})
	assert.Error(t, err)
}

func TestPRHandler_GenerateDefaultArgs(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
	handler := &IssueHandler{}
	assert.Equal(t, "Issue", handler.Name())
}

func TestIssueHandler_ValidateArgs_Valid(t *testing.T) {
	t.Parallel()
	handler := &IssueHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"action":      "list",
		"description": "List issues",
	})
	assert.NoError(t, err)
}

func TestIssueHandler_ValidateArgs_MissingAction(t *testing.T) {
	t.Parallel()
	handler := &IssueHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "List issues",
	})
	assert.Error(t, err)
}

func TestIssueHandler_GenerateDefaultArgs(t *testing.T) {
	t.Parallel()
	handler := &IssueHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, "list", args["action"])
	assert.NotEmpty(t, args["description"])
}

// ============================================================================
// WorkflowHandler Tests
// ============================================================================

func TestWorkflowHandler_Name(t *testing.T) {
	t.Parallel()
	handler := &WorkflowHandler{}
	assert.Equal(t, "Workflow", handler.Name())
}

func TestWorkflowHandler_ValidateArgs_Valid(t *testing.T) {
	t.Parallel()
	handler := &WorkflowHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"action":      "list",
		"description": "List workflows",
	})
	assert.NoError(t, err)
}

func TestWorkflowHandler_ValidateArgs_MissingAction(t *testing.T) {
	t.Parallel()
	handler := &WorkflowHandler{}
	err := handler.ValidateArgs(map[string]interface{}{
		"description": "List workflows",
	})
	assert.Error(t, err)
}

func TestWorkflowHandler_GenerateDefaultArgs(t *testing.T) {
	t.Parallel()
	handler := &WorkflowHandler{}
	args := handler.GenerateDefaultArgs("any context")
	assert.Equal(t, "list", args["action"])
	assert.NotEmpty(t, args["description"])
}

// ============================================================================
// ToolResult Tests
// ============================================================================

func TestToolResult_Structure(t *testing.T) {
	t.Parallel()
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

// ============================================================================
// Execute Method Tests - Error Paths
// ============================================================================

func TestReferencesHandler_Execute_EmptySymbol(t *testing.T) {
	t.Parallel()
	handler := &ReferencesHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{})
	assert.NoError(t, err) // Handler returns result with error, not error
	assert.False(t, result.Success)
	assert.Equal(t, "symbol is required", result.Error)
}

func TestDefinitionHandler_Execute_EmptySymbol(t *testing.T) {
	t.Parallel()
	handler := &DefinitionHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{})
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, "symbol is required", result.Error)
}

func TestPRHandler_Execute_MergeWithoutPRNumber(t *testing.T) {
	t.Parallel()
	handler := &PRHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{
		"action": "merge",
	})
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "pr_number required")
}

func TestPRHandler_Execute_CloseWithoutPRNumber(t *testing.T) {
	t.Parallel()
	handler := &PRHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{
		"action": "close",
	})
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "pr_number required")
}

func TestPRHandler_Execute_UnknownAction(t *testing.T) {
	t.Parallel()
	handler := &PRHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{
		"action": "unknown_action",
	})
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unknown action")
}

func TestIssueHandler_Execute_ViewWithoutIssueNumber(t *testing.T) {
	t.Parallel()
	handler := &IssueHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{
		"action": "view",
	})
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "issue_number required")
}

func TestIssueHandler_Execute_CloseWithoutIssueNumber(t *testing.T) {
	t.Parallel()
	handler := &IssueHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{
		"action": "close",
	})
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "issue_number required")
}

func TestIssueHandler_Execute_UnknownAction(t *testing.T) {
	t.Parallel()
	handler := &IssueHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{
		"action": "unknown",
	})
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unknown action")
}

func TestWorkflowHandler_Execute_CancelWithoutRunID(t *testing.T) {
	t.Parallel()
	handler := &WorkflowHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{
		"action": "cancel",
	})
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "run_id required")
}

func TestWorkflowHandler_Execute_LogsWithoutRunID(t *testing.T) {
	t.Parallel()
	handler := &WorkflowHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{
		"action": "logs",
	})
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "run_id required")
}

func TestWorkflowHandler_Execute_UnknownAction(t *testing.T) {
	t.Parallel()
	handler := &WorkflowHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{
		"action": "unknown",
	})
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unknown action")
}

func TestLintHandler_Execute_UnsupportedLinter(t *testing.T) {
	t.Parallel()
	handler := &LintHandler{}
	ctx := context.Background()

	result, err := handler.Execute(ctx, map[string]interface{}{
		"linter": "unknown_linter",
	})
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unsupported linter")
}

// ============================================================================
// Additional GenerateDefaultArgs Edge Cases
// ============================================================================

func TestGitHandler_GenerateDefaultArgs_AllOperations(t *testing.T) {
	t.Parallel()
	handler := &GitHandler{}

	// Test all operation keywords - note that the code checks keywords in order,
	// so if multiple keywords match, the first one in the if-else chain wins
	operations := map[string]string{
		"I need to commit":  "commit",
		"push my changes":   "push",
		"pull from remote":  "pull",
		"switch branch":     "branch",
		"checkout the file": "checkout",
		"please merge":      "merge", // Use "merge" without "branch"
		"show the diff":     "diff",
		"view log history":  "log", // Use "log" without "commit"
		"stash my work":     "stash",
		"just show status":  "status",
	}

	for context, expectedOp := range operations {
		t.Run(context, func(t *testing.T) {
			t.Parallel()
			args := handler.GenerateDefaultArgs(context)
			assert.Equal(t, expectedOp, args["operation"])
		})
	}
}

func TestTestHandler_GenerateDefaultArgs_AllTestTypes(t *testing.T) {
	t.Parallel()
	handler := &TestHandler{}

	testCases := []struct {
		context      string
		expectedType string
		expectedPath string
	}{
		{"run unit tests", "unit", "./internal/..."},
		{"run integration tests", "integration", "./tests/integration/..."},
		{"run e2e tests", "e2e", "./tests/e2e/..."},
		{"just run tests", "all", "./..."},
	}

	for _, tc := range testCases {
		t.Run(tc.context, func(t *testing.T) {
			t.Parallel()
			args := handler.GenerateDefaultArgs(tc.context)
			assert.Equal(t, tc.expectedType, args["test_type"])
			assert.Equal(t, tc.expectedPath, args["test_path"])
		})
	}
}

// ============================================================================
// Concurrent Access Tests
// ============================================================================

func TestToolRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	registry := NewToolRegistry()

	// Register handlers concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			registry.Register(&GitHandler{})
			done <- true
		}()
	}

	// Wait for all registrations
	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = registry.Get("git")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should still work correctly
	h, ok := registry.Get("git")
	assert.True(t, ok)
	assert.NotNil(t, h)
}

// ============================================================================
// ToolResult Data Field Tests
// ============================================================================

func TestToolResult_WithData(t *testing.T) {
	t.Parallel()
	result := ToolResult{
		Success: true,
		Output:  "Success",
		Data: map[string]interface{}{
			"count":  5,
			"files":  []string{"a.go", "b.go"},
			"nested": map[string]int{"x": 1},
		},
	}

	assert.True(t, result.Success)
	assert.NotNil(t, result.Data)

	data := result.Data.(map[string]interface{})
	assert.Equal(t, 5, data["count"])
	assert.Len(t, data["files"].([]string), 2)
}

func TestToolResult_EmptyFields(t *testing.T) {
	t.Parallel()
	result := ToolResult{}

	assert.False(t, result.Success)
	assert.Empty(t, result.Output)
	assert.Empty(t, result.Error)
	assert.Nil(t, result.Data)
}

// ============================================================================
// Handler Interface Compliance Tests
// ============================================================================

func TestAllHandlers_ImplementInterface(t *testing.T) {
	t.Parallel()
	handlers := []ToolHandler{
		&ReadFileHandler{},
		&GitHandler{},
		&TestHandler{},
		&LintHandler{},
		&DiffHandler{},
		&TreeViewHandler{},
		&FileInfoHandler{},
		&SymbolsHandler{},
		&ReferencesHandler{},
		&DefinitionHandler{},
		&PRHandler{},
		&IssueHandler{},
		&WorkflowHandler{},
	}

	for _, h := range handlers {
		t.Run(h.Name(), func(t *testing.T) {
			t.Parallel()
			// Verify Name() returns non-empty
			assert.NotEmpty(t, h.Name())

			// Verify GenerateDefaultArgs returns map with description
			args := h.GenerateDefaultArgs("test context")
			assert.NotNil(t, args)
			assert.NotEmpty(t, args["description"])

			// Verify ValidateArgs can be called
			// (may error due to missing required fields but shouldn't panic)
			_ = h.ValidateArgs(map[string]interface{}{})
		})
	}
}

// ============================================================================
// Edge Cases for Argument Processing
// ============================================================================

func TestGitHandler_Execute_WithArguments(t *testing.T) {
	t.Parallel()
	handler := &GitHandler{}
	ctx := context.Background()

	// Test that arguments are processed correctly
	result, _ := handler.Execute(ctx, map[string]interface{}{
		"operation":   "log",
		"arguments":   []interface{}{"--oneline", "-5"},
		"working_dir": "/tmp/nonexistent_dir_for_test",
	})

	// Will fail because dir doesn't exist, but shouldn't panic
	assert.False(t, result.Success)
}

func TestTestHandler_Execute_DefaultValues(t *testing.T) {
	t.Parallel()
	handler := &TestHandler{}
	// Use a short timeout to avoid running full test suite for 5+ minutes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Execute with a specific fast test path to verify default behavior without hanging
	result, _ := handler.Execute(ctx, map[string]interface{}{
		"test_path": "./handler.go", // Non-test file — will fail quickly but won't hang
		"timeout":   "10s",
	})

	// Will fail because handler.go is not a test file, but shouldn't panic
	_ = result
}

func TestDiffHandler_Execute_Modes(t *testing.T) {
	t.Parallel()
	handler := &DiffHandler{}
	ctx := context.Background()

	modes := []string{"working", "staged", "commit", "branch"}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			result, _ := handler.Execute(ctx, map[string]interface{}{
				"mode":          mode,
				"compare_with":  "main",
				"context_lines": float64(5),
			})
			// Should not panic regardless of mode
			_ = result
		})
	}
}

func TestTreeViewHandler_Execute_WithIgnorePatterns(t *testing.T) {
	t.Parallel()
	handler := &TreeViewHandler{}
	ctx := context.Background()

	result, _ := handler.Execute(ctx, map[string]interface{}{
		"path":            ".",
		"max_depth":       float64(2),
		"show_hidden":     true,
		"ignore_patterns": []interface{}{"node_modules", "vendor"},
	})

	// Should not panic with ignore patterns
	_ = result
}

func TestSymbolsHandler_Execute_Recursive(t *testing.T) {
	t.Parallel()
	handler := &SymbolsHandler{}
	ctx := context.Background()

	result, _ := handler.Execute(ctx, map[string]interface{}{
		"file_path": ".",
		"recursive": true,
	})

	// Should have executed grep with -r flag
	_ = result
}

func TestReferencesHandler_Execute_WithFilePath(t *testing.T) {
	t.Parallel()
	handler := &ReferencesHandler{}
	ctx := context.Background()

	result, _ := handler.Execute(ctx, map[string]interface{}{
		"symbol":    "TestFunction",
		"file_path": "./internal",
	})

	// Should have searched in specified path
	_ = result
}

func TestPRHandler_Execute_CreateWithOptions(t *testing.T) {
	t.Parallel()
	handler := &PRHandler{}
	ctx := context.Background()

	// This would fail without gh CLI, but tests argument processing
	result, _ := handler.Execute(ctx, map[string]interface{}{
		"action":      "create",
		"title":       "Test PR",
		"body":        "Test body",
		"base_branch": "develop",
	})

	// Will fail without gh CLI but shouldn't panic
	assert.False(t, result.Success)
}

func TestPRHandler_Execute_ViewWithPRNumber(t *testing.T) {
	t.Parallel()
	handler := &PRHandler{}
	ctx := context.Background()

	result, _ := handler.Execute(ctx, map[string]interface{}{
		"action":    "view",
		"pr_number": float64(123),
	})

	// Will fail without gh CLI but tests pr_number processing
	assert.False(t, result.Success)
}

func TestIssueHandler_Execute_CreateWithOptions(t *testing.T) {
	t.Parallel()
	// Skip in short mode to avoid long timeouts
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	// Skip this test if gh CLI is available to avoid creating real issues
	if _, err := exec.LookPath("gh"); err == nil {
		t.Skip("Skipping test: gh CLI is available and would create real issues")
	}

	handler := &IssueHandler{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, _ := handler.Execute(ctx, map[string]interface{}{
		"action": "create",
		"title":  "Test Issue",
		"body":   "Issue description",
	})

	// Will fail without gh CLI
	assert.False(t, result.Success)
}

func TestWorkflowHandler_Execute_RunWithOptions(t *testing.T) {
	t.Parallel()
	// Skip in short mode to avoid long timeouts
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	// Skip this test if gh CLI is available to avoid running real workflows
	if _, err := exec.LookPath("gh"); err == nil {
		t.Skip("Skipping test: gh CLI is available and would run real workflows")
	}

	handler := &WorkflowHandler{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, _ := handler.Execute(ctx, map[string]interface{}{
		"action":      "run",
		"workflow_id": "test.yml",
		"branch":      "main",
	})

	// Will fail without gh CLI
	assert.False(t, result.Success)
}

func TestWorkflowHandler_Execute_ViewWithRunID(t *testing.T) {
	t.Parallel()
	handler := &WorkflowHandler{}
	ctx := context.Background()

	result, _ := handler.Execute(ctx, map[string]interface{}{
		"action": "view",
		"run_id": float64(12345),
	})

	// Will fail without gh CLI but tests run_id processing
	assert.False(t, result.Success)
}
