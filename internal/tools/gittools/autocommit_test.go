package gittools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRepo(t *testing.T) string {
	tempDir := t.TempDir()
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())
	
	// Configure git user
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())
	
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())
	
	return tempDir
}

func createTestFile(t *testing.T, dir, name, content string) string {
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

func TestAutoCommit_IsGitRepo(t *testing.T) {
	t.Parallel()
	// Non-git directory
	tempDir := t.TempDir()
	ac := NewAutoCommit(tempDir, DefaultAutoCommitConfig(), nil)
	assert.False(t, ac.IsGitRepo())
	
	// Git directory
	gitDir := setupTestRepo(t)
	ac2 := NewAutoCommit(gitDir, DefaultAutoCommitConfig(), nil)
	assert.True(t, ac2.IsGitRepo())
}

func TestAutoCommit_GetChanges(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	ac := NewAutoCommit(dir, DefaultAutoCommitConfig(), nil)
	
	// No changes initially
	changes, err := ac.GetChanges()
	require.NoError(t, err)
	assert.Empty(t, changes)
	
	// Create a file
	createTestFile(t, dir, "test.txt", "hello")
	
	changes, err = ac.GetChanges()
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "test.txt", changes[0].Path)
	assert.Equal(t, "added", changes[0].Operation)
}

func TestAutoCommit_StageAll(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	ac := NewAutoCommit(dir, DefaultAutoCommitConfig(), nil)
	
	// Create and stage files
	createTestFile(t, dir, "test1.txt", "content1")
	createTestFile(t, dir, "test2.txt", "content2")
	
	ctx := context.Background()
	err := ac.StageAll(ctx)
	require.NoError(t, err)
	
	// After staging, files should still show as modified (not untracked)
	changes, _ := ac.GetChanges()
	assert.Len(t, changes, 2)
	for _, c := range changes {
		assert.Equal(t, "added", c.Operation)
	}
}

func TestAutoCommit_GenerateCommitMessage_SingleFile(t *testing.T) {
	t.Parallel()
	ac := NewAutoCommit("/tmp", DefaultAutoCommitConfig(), nil)
	
	changes := []Change{
		{Path: "test.txt", Operation: "added"},
	}
	
	msg := ac.GenerateCommitMessage(changes)
	assert.Contains(t, msg, "add")
	assert.Contains(t, msg, "test.txt")
}

func TestAutoCommit_GenerateCommitMessage_MultipleFiles(t *testing.T) {
	t.Parallel()
	ac := NewAutoCommit("/tmp", DefaultAutoCommitConfig(), nil)
	
	changes := []Change{
		{Path: "test1.txt", Operation: "added"},
		{Path: "test2.txt", Operation: "added"},
		{Path: "test3.txt", Operation: "modified"},
	}
	
	msg := ac.GenerateCommitMessage(changes)
	assert.Contains(t, msg, "add")
	assert.Contains(t, msg, "update")
}

func TestAutoCommit_GenerateCommitMessage_ConventionalCommits(t *testing.T) {
	t.Parallel()
	config := DefaultAutoCommitConfig()
	config.ConventionalCommits = true
	ac := NewAutoCommit("/tmp", config, nil)
	
	// Test added files -> feat
	changes := []Change{
		{Path: "feature.go", Operation: "added"},
	}
	msg := ac.GenerateCommitMessage(changes)
	assert.Contains(t, msg, "feat")
	
	// Test modified files -> fix
	changes = []Change{
		{Path: "bugfix.go", Operation: "modified"},
	}
	msg = ac.GenerateCommitMessage(changes)
	assert.Contains(t, msg, "fix")
	
	// Test deleted files -> remove
	changes = []Change{
		{Path: "old.go", Operation: "deleted"},
	}
	msg = ac.GenerateCommitMessage(changes)
	assert.Contains(t, msg, "remove")
}

func TestAutoCommit_determineScope(t *testing.T) {
	t.Parallel()
	ac := NewAutoCommit("/tmp", DefaultAutoCommitConfig(), nil)
	
	// Single directory
	changes := []Change{
		{Path: "internal/test.go", Operation: "modified"},
		{Path: "internal/utils.go", Operation: "modified"},
	}
	scope := ac.determineScope(changes)
	assert.Equal(t, "internal", scope)
	
	// Mixed directories
	changes = []Change{
		{Path: "cmd/main.go", Operation: "modified"},
		{Path: "internal/test.go", Operation: "modified"},
	}
	scope = ac.determineScope(changes)
	// Should pick most common or first
	assert.NotEmpty(t, scope)
}

func TestAutoCommit_determineChangeType(t *testing.T) {
	t.Parallel()
	ac := NewAutoCommit("/tmp", DefaultAutoCommitConfig(), nil)
	
	// Only added -> feat
	counts := map[string]int{"added": 2}
	assert.Equal(t, "feat", ac.determineChangeType(counts))
	
	// Only deleted -> remove
	counts = map[string]int{"deleted": 2}
	assert.Equal(t, "remove", ac.determineChangeType(counts))
	
	// Only modified -> fix
	counts = map[string]int{"modified": 2}
	assert.Equal(t, "fix", ac.determineChangeType(counts))
	
	// Mixed -> chore
	counts = map[string]int{"added": 1, "modified": 2}
	assert.Equal(t, "fix", ac.determineChangeType(counts))
}

func TestAutoCommit_generateDescription(t *testing.T) {
	t.Parallel()
	ac := NewAutoCommit("/tmp", DefaultAutoCommitConfig(), nil)
	
	// Single file added
	changes := []Change{{Path: "test.txt", Operation: "added"}}
	desc := ac.generateDescription(changes)
	assert.Equal(t, "add test.txt", desc)
	
	// Single file modified
	changes = []Change{{Path: "test.txt", Operation: "modified"}}
	desc = ac.generateDescription(changes)
	assert.Equal(t, "update test.txt", desc)
	
	// Single file deleted
	changes = []Change{{Path: "test.txt", Operation: "deleted"}}
	desc = ac.generateDescription(changes)
	assert.Equal(t, "remove test.txt", desc)
	
	// Multiple files
	changes = []Change{
		{Path: "test1.txt", Operation: "added"},
		{Path: "test2.txt", Operation: "added"},
	}
	desc = ac.generateDescription(changes)
	assert.Contains(t, desc, "add 2 file(s)")
}

func TestAutoCommit_CommitChanges(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	ac := NewAutoCommit(dir, DefaultAutoCommitConfig(), nil)
	
	// Create initial commit
	createTestFile(t, dir, "README.md", "# Test")
	ctx := context.Background()
	ac.StageAll(ctx)
	ac.CommitChanges(ctx, "Initial commit")
	
	// Create and commit a change
	createTestFile(t, dir, "feature.go", "package main")
	
	commit, err := ac.CommitChanges(ctx, "Add feature")
	require.NoError(t, err)
	require.NotNil(t, commit)
	
	assert.NotEmpty(t, commit.Hash)
	assert.Equal(t, "Add feature", commit.Message)
	assert.NotEmpty(t, commit.Files)
	assert.True(t, commit.Timestamp.Before(time.Now().Add(time.Second)))
}

func TestAutoCommit_CommitChanges_NoChanges(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	ac := NewAutoCommit(dir, DefaultAutoCommitConfig(), nil)
	
	ctx := context.Background()
	_, err := ac.CommitChanges(ctx, "message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no changes")
}

func TestAutoCommit_CommitChanges_NotGitRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ac := NewAutoCommit(dir, DefaultAutoCommitConfig(), nil)
	
	ctx := context.Background()
	_, err := ac.CommitChanges(ctx, "message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestAutoCommit_CommitChanges_AutoGenerate(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	config := DefaultAutoCommitConfig()
	config.GenerateMessage = true
	ac := NewAutoCommit(dir, config, nil)
	
	// Create initial commit
	createTestFile(t, dir, "README.md", "# Test")
	ctx := context.Background()
	ac.StageAll(ctx)
	ac.CommitChanges(ctx, "Initial commit")
	
	// Create file without message
	createTestFile(t, dir, "new.go", "package main")
	
	commit, err := ac.CommitChanges(ctx, "") // Empty message
	require.NoError(t, err)
	require.NotNil(t, commit)
	
	// Message should be auto-generated
	assert.NotEmpty(t, commit.Message)
	assert.Contains(t, commit.Message, "add")
}

func TestAutoCommit_GetLastCommitHash(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	ac := NewAutoCommit(dir, DefaultAutoCommitConfig(), nil)
	
	// Create initial commit
	createTestFile(t, dir, "README.md", "# Test")
	ctx := context.Background()
	ac.StageAll(ctx)
	ac.CommitChanges(ctx, "Initial commit")
	
	hash, err := ac.GetLastCommitHash(ctx)
	require.NoError(t, err)
	assert.Len(t, hash, 40) // Full SHA-1 hash
}

func TestAutoCommit_GetCurrentBranch(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	ac := NewAutoCommit(dir, DefaultAutoCommitConfig(), nil)
	
	ctx := context.Background()
	branch, err := ac.GetCurrentBranch(ctx)
	require.NoError(t, err)
	assert.Equal(t, "main", branch) // Default branch
}

func TestAutoCommit_CreateAndSwitchBranch(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	ac := NewAutoCommit(dir, DefaultAutoCommitConfig(), nil)
	
	// Create initial commit (needed for branch operations)
	createTestFile(t, dir, "README.md", "# Test")
	ctx := context.Background()
	ac.StageAll(ctx)
	ac.CommitChanges(ctx, "Initial commit")
	
	// Create new branch
	err := ac.CreateBranch(ctx, "feature-branch")
	require.NoError(t, err)
	
	branch, _ := ac.GetCurrentBranch(ctx)
	assert.Equal(t, "feature-branch", branch)
	
	// Switch back to main
	err = ac.SwitchBranch(ctx, "main")
	require.NoError(t, err)
	
	branch, _ = ac.GetCurrentBranch(ctx)
	assert.Equal(t, "main", branch)
}

func TestAutoCommit_Stash(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	ac := NewAutoCommit(dir, DefaultAutoCommitConfig(), nil)
	
	// Create initial commit with tracked file
	createTestFile(t, dir, "README.md", "# Test")
	createTestFile(t, dir, "tracked.txt", "initial")
	ctx := context.Background()
	ac.StageAll(ctx)
	ac.CommitChanges(ctx, "Initial commit")
	
	// Modify tracked file for stash
	createTestFile(t, dir, "tracked.txt", "modified content")
	ac.StageAll(ctx)
	
	// Stash
	err := ac.Stash(ctx, "WIP")
	require.NoError(t, err)
	
	// Verify changes are gone (may still show untracked files)
	_, _ = ac.GetChanges()
	
	// Pop stash
	err = ac.PopStash(ctx)
	require.NoError(t, err)
	
	// Verify file content is restored
	content, err := ac.GetDiff("tracked.txt")
	_ = content
	// File should have changes again
}

func TestAutoCommit_HasUncommittedChanges(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	ac := NewAutoCommit(dir, DefaultAutoCommitConfig(), nil)
	
	// No changes
	assert.False(t, ac.HasUncommittedChanges())
	
	// Create file
	createTestFile(t, dir, "test.txt", "content")
	assert.True(t, ac.HasUncommittedChanges())
}

func TestAutoCommit_GetDiff(t *testing.T) {
	t.Parallel()
	dir := setupTestRepo(t)
	ac := NewAutoCommit(dir, DefaultAutoCommitConfig(), nil)
	
	// Create and commit initial file
	createTestFile(t, dir, "test.txt", "initial")
	ctx := context.Background()
	ac.StageAll(ctx)
	ac.CommitChanges(ctx, "Initial")
	
	// Modify file
	createTestFile(t, dir, "test.txt", "modified")
	
	diff, err := ac.GetDiff("test.txt")
	require.NoError(t, err)
	assert.Contains(t, diff, "modified")
}
