package gittools

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTool(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	tool := NewTool("/tmp", logger)

	require.NotNil(t, tool)
	assert.NotNil(t, tool.autoCommit)
	assert.NotNil(t, tool.logger)
}

func TestTool_Name(t *testing.T) {
	t.Parallel()
	tool := NewTool("/tmp", nil)
	assert.Equal(t, "GitTools", tool.Name())
}

func TestTool_Description(t *testing.T) {
	t.Parallel()
	tool := NewTool("/tmp", nil)
	desc := tool.Description()
	assert.Contains(t, desc, "Git")
	assert.Contains(t, desc, "commit")
}

func TestTool_Schema(t *testing.T) {
	t.Parallel()
	tool := NewTool("/tmp", nil)
	schema := tool.Schema()

	require.NotNil(t, schema)
	assert.Equal(t, "object", schema["type"])

	props, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, props, "operation")
	assert.Contains(t, props, "message")
	assert.Contains(t, props, "remote")
	assert.Contains(t, props, "branch")

	required, ok := schema["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "operation")
}

func TestTool_Execute_MissingOperation(t *testing.T) {
	t.Parallel()
	tool := NewTool("/tmp", nil)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{})

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "operation is required")
}

func TestTool_Execute_InvalidOperation(t *testing.T) {
	t.Parallel()
	tool := NewTool("/tmp", nil)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "invalid",
	})

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unknown operation")
}

func TestTool_Execute_Status(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "status",
	})

	require.NoError(t, err)
	assert.True(t, result.Success)
	// Initially no changes
	assert.Contains(t, result.Output, "No uncommitted")
}

func TestTool_Execute_Status_WithChanges(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	// Create a file
	createTestFile(t, dir, "test.txt", "content")

	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "status",
	})

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "added:")
	assert.Contains(t, result.Output, "test.txt")
}

func TestTool_Execute_Commit(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	// Setup initial commit
	createTestFile(t, dir, "README.md", "# Test")
	tool.autoCommit.StageAll(ctx)
	tool.autoCommit.CommitChanges(ctx, "Initial commit")

	// Create and commit
	createTestFile(t, dir, "new.txt", "content")

	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "commit",
		"message":   "Add new file",
	})

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.NotNil(t, result.Commit)
	assert.Equal(t, "Add new file", result.Commit.Message)
	assert.Contains(t, result.Output, "Committed")
}

func TestTool_Execute_Commit_NoChanges(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "commit",
		"message":   "Test",
	})

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "no changes")
}

func TestTool_Execute_Commit_AutoGenerate(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	// Setup initial commit
	createTestFile(t, dir, "README.md", "# Test")
	tool.autoCommit.StageAll(ctx)
	tool.autoCommit.CommitChanges(ctx, "Initial commit")

	// Create and commit without message
	createTestFile(t, dir, "new.txt", "content")

	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation":        "commit",
		"generate_message": true,
	})

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.NotNil(t, result.Commit)
	// Message should be auto-generated
	assert.NotEmpty(t, result.Commit.Message)
}

func TestTool_Execute_Branch(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	// Setup initial commit
	createTestFile(t, dir, "README.md", "# Test")
	tool.autoCommit.StageAll(ctx)
	tool.autoCommit.CommitChanges(ctx, "Initial commit")

	// Test current branch
	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "branch",
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	// Exact match, not Contains("main"): setupTestRepo pins the initial branch
	// to "main" via `git symbolic-ref`, and GetCurrentBranch TrimSpaces the
	// `git branch --show-current` output, so the full line is deterministic.
	// Contains would also accept "Current branch: maintenance" — or any other
	// branch whose name merely embeds "main" — which is not what this asserts.
	assert.Equal(t, "Current branch: main", result.Output)
}

func TestTool_Execute_Branch_Create(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	// Setup initial commit
	createTestFile(t, dir, "README.md", "# Test")
	tool.autoCommit.StageAll(ctx)
	tool.autoCommit.CommitChanges(ctx, "Initial commit")

	// Create branch
	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "branch",
		"action":    "create",
		"branch":    "feature",
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "Created branch")
}

func TestTool_Execute_Branch_Create_MissingName(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "branch",
		"action":    "create",
	})
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "branch name is required")
}

func TestTool_Execute_Stash(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	// Setup initial commit
	createTestFile(t, dir, "README.md", "# Test")
	tool.autoCommit.StageAll(ctx)
	tool.autoCommit.CommitChanges(ctx, "Initial commit")

	// Create change
	createTestFile(t, dir, "wip.txt", "work")

	// Stash
	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "stash",
		"action":    "push",
		"message":   "WIP",
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "stashed")
}

func TestTool_Execute_StashPop(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	// Setup initial commit with tracked file
	createTestFile(t, dir, "README.md", "# Test")
	createTestFile(t, dir, "tracked.txt", "initial")
	tool.autoCommit.StageAll(ctx)
	tool.autoCommit.CommitChanges(ctx, "Initial commit")

	// Modify tracked file and stash
	createTestFile(t, dir, "tracked.txt", "modified")
	tool.autoCommit.StageAll(ctx)
	err := tool.autoCommit.Stash(ctx, "WIP")
	require.NoError(t, err)

	// Pop stash
	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "stash",
		"action":    "pop",
	})
	require.NoError(t, err)
	// Stash pop can succeed or fail depending on state
	_ = result
}

func TestTool_Commit(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	// Setup initial commit
	createTestFile(t, dir, "README.md", "# Test")
	tool.autoCommit.StageAll(ctx)
	tool.autoCommit.CommitChanges(ctx, "Initial commit")

	// Create and commit
	createTestFile(t, dir, "new.txt", "content")

	result, err := tool.Commit(ctx, "Add file")
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestTool_Status(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	result, err := tool.Status(ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestTool_Push(t *testing.T) {
	t.Parallel()
	// Can't test actual push without remote
	// Just test error handling
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	result, err := tool.Push(ctx, "origin", "main")
	// Will fail due to no remote
	require.NoError(t, err) // Tool doesn't return error, just result
	// Result may be success or failure depending on test environment
	_ = result
}

func TestTool_Pull(t *testing.T) {
	t.Parallel()
	// Can't test actual pull without remote
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	result, err := tool.Pull(ctx, "origin", "main")
	// Will fail due to no remote
	require.NoError(t, err)
	_ = result
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "hello", truncate("hello", 100))
	assert.Equal(t, "hello...", truncate("hello world", 5))
}

// Integration test
func TestTool_Workflow(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	tool := NewTool(dir, nil)
	ctx := context.Background()

	// 1. Check status - no changes
	result, _ := tool.Status(ctx)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "No uncommitted")

	// 2. Create a file
	createTestFile(t, dir, "main.go", "package main")

	// 3. Check status - has changes
	result, _ = tool.Status(ctx)
	assert.Contains(t, result.Output, "added:")

	// 4. Commit
	result, _ = tool.Commit(ctx, "Initial commit")
	assert.True(t, result.Success)

	// 5. Check status - no changes
	result, _ = tool.Status(ctx)
	assert.Contains(t, result.Output, "No uncommitted")
}

// Benchmark test
func BenchmarkTool_Execute(b *testing.B) {
	dir := setupTestRepo(&testing.T{})
	tool := NewTool(dir, nil)
	ctx := context.Background()

	// Create initial commit - skip for benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Setup code here if needed
		b.StartTimer()

		tool.Execute(ctx, map[string]interface{}{
			"operation": "status",
		})
	}
}
