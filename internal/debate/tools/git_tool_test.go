package tools

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

// =============================================================================
// Git Tool Tests
// =============================================================================

// setupTestGitRepo creates a temporary git repository with an initial commit.
func setupTestGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "command %v failed: %s", args, string(out))
	}

	readme := filepath.Join(dir, "README.md")
	require.NoError(t, os.WriteFile(readme, []byte("# Test\n"), 0o644))

	addCmd := exec.Command("git", "-C", dir, "add", ".")
	out, err := addCmd.CombinedOutput()
	require.NoError(t, err, "git add failed: %s", string(out))

	commitCmd := exec.Command("git", "-C", dir, "commit", "-m", "initial")
	out, err = commitCmd.CombinedOutput()
	require.NoError(t, err, "git commit failed: %s", string(out))

	return dir
}

// gitAvailable returns true if the git binary is on the PATH.
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func TestNewGitTool_ValidRepo(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)

	tool, err := NewGitTool(GitToolConfig{
		RepoDir: repoDir,
	})
	require.NoError(t, err)
	require.NotNil(t, tool)

	assert.Equal(t, repoDir, tool.workDir)
	assert.Equal(t, filepath.Join(repoDir, ".debate-worktrees"), tool.worktreeDir)

	// Verify the worktree directory was created.
	info, err := os.Stat(tool.worktreeDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestNewGitTool_ValidRepo_CustomWorktreeDir(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	wtDir := filepath.Join(t.TempDir(), "custom-wt")

	tool, err := NewGitTool(GitToolConfig{
		RepoDir:     repoDir,
		WorktreeDir: wtDir,
	})
	require.NoError(t, err)
	require.NotNil(t, tool)
	assert.Equal(t, wtDir, tool.worktreeDir)
}

func TestNewGitTool_InvalidRepo(t *testing.T) {
	t.Parallel()
	tool, err := NewGitTool(GitToolConfig{
		RepoDir: "/nonexistent/path/does/not/exist",
	})
	assert.Error(t, err)
	assert.Nil(t, tool)
	assert.Contains(t, err.Error(), "repo dir does not exist")
}

func TestNewGitTool_EmptyRepoDir(t *testing.T) {
	t.Parallel()
	tool, err := NewGitTool(GitToolConfig{})
	assert.Error(t, err)
	assert.Nil(t, tool)
	assert.Contains(t, err.Error(), "repo_dir is required")
}

func TestNewGitTool_NotGitRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	tool, err := NewGitTool(GitToolConfig{
		RepoDir: dir,
	})
	assert.Error(t, err)
	assert.Nil(t, tool)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestGitTool_CreateWorktree(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	tool, err := NewGitTool(GitToolConfig{RepoDir: repoDir})
	require.NoError(t, err)

	ctx := context.Background()
	sessionID := "test-session-001"

	wt, err := tool.CreateWorktree(ctx, sessionID)
	require.NoError(t, err)
	require.NotNil(t, wt)

	assert.Equal(t, sessionID, wt.SessionID)
	assert.Equal(t, "debate/"+sessionID, wt.Branch)
	assert.Equal(t, filepath.Join(tool.worktreeDir, sessionID), wt.Path)
	assert.False(t, wt.CreatedAt.IsZero())

	// Verify the worktree directory exists on disk.
	info, err := os.Stat(wt.Path)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Clean up the worktree to avoid leaving git state behind.
	_ = tool.Cleanup(ctx, wt.Path)
}

func TestGitTool_CreateWorktree_EmptySessionID(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	tool, err := NewGitTool(GitToolConfig{RepoDir: repoDir})
	require.NoError(t, err)

	ctx := context.Background()
	wt, err := tool.CreateWorktree(ctx, "")
	assert.Error(t, err)
	assert.Nil(t, wt)
	assert.Contains(t, err.Error(), "session ID is required")
}

func TestGitTool_CreateWorktree_DuplicateSessionID(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	tool, err := NewGitTool(GitToolConfig{RepoDir: repoDir})
	require.NoError(t, err)

	ctx := context.Background()
	sessionID := "dup-session"

	wt, err := tool.CreateWorktree(ctx, sessionID)
	require.NoError(t, err)
	require.NotNil(t, wt)

	// Second attempt with same session ID should fail.
	wt2, err := tool.CreateWorktree(ctx, sessionID)
	assert.Error(t, err)
	assert.Nil(t, wt2)
	assert.Contains(t, err.Error(), "worktree already exists")

	_ = tool.Cleanup(ctx, wt.Path)
}

func TestGitTool_CommitSnapshot(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	tool, err := NewGitTool(GitToolConfig{RepoDir: repoDir})
	require.NoError(t, err)

	ctx := context.Background()
	wt, err := tool.CreateWorktree(ctx, "snapshot-session")
	require.NoError(t, err)

	code := "package main\n\nfunc main() {}\n"
	snap, err := tool.CommitSnapshot(
		ctx, wt.Path, code, "main.go", "debate snapshot",
	)
	require.NoError(t, err)
	require.NotNil(t, snap)

	assert.NotEmpty(t, snap.CommitHash)
	assert.Equal(t, "debate/snapshot-session", snap.Branch)
	assert.Equal(t, "debate snapshot", snap.Message)
	assert.False(t, snap.Timestamp.IsZero())

	// Verify the file was actually written.
	content, err := os.ReadFile(filepath.Join(wt.Path, "main.go"))
	require.NoError(t, err)
	assert.Equal(t, code, string(content))

	_ = tool.Cleanup(ctx, wt.Path)
}

func TestGitTool_CommitSnapshot_SubDirectory(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	tool, err := NewGitTool(GitToolConfig{RepoDir: repoDir})
	require.NoError(t, err)

	ctx := context.Background()
	wt, err := tool.CreateWorktree(ctx, "subdir-session")
	require.NoError(t, err)

	code := "package sub\n\nfunc Hello() {}\n"
	snap, err := tool.CommitSnapshot(
		ctx, wt.Path, code, "pkg/sub/hello.go",
		"create sub package",
	)
	require.NoError(t, err)
	require.NotNil(t, snap)
	assert.NotEmpty(t, snap.CommitHash)

	// Verify the file exists under the sub-directory.
	filePath := filepath.Join(wt.Path, "pkg", "sub", "hello.go")
	_, err = os.Stat(filePath)
	require.NoError(t, err)

	_ = tool.Cleanup(ctx, wt.Path)
}

func TestGitTool_CommitSnapshot_ValidationErrors(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	tool, err := NewGitTool(GitToolConfig{RepoDir: repoDir})
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name        string
		worktreeDir string
		code        string
		filename    string
		message     string
		errContains string
	}{
		{
			name:        "empty worktree dir",
			worktreeDir: "",
			code:        "code",
			filename:    "f.go",
			message:     "msg",
			errContains: "worktree directory is required",
		},
		{
			name:        "empty code",
			worktreeDir: "/tmp",
			code:        "",
			filename:    "f.go",
			message:     "msg",
			errContains: "code content is required",
		},
		{
			name:        "empty filename",
			worktreeDir: "/tmp",
			code:        "code",
			filename:    "",
			message:     "msg",
			errContains: "filename is required",
		},
		{
			name:        "empty message",
			worktreeDir: "/tmp",
			code:        "code",
			filename:    "f.go",
			message:     "",
			errContains: "commit message is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			snap, snapErr := tool.CommitSnapshot(
				ctx, tc.worktreeDir, tc.code, tc.filename, tc.message,
			)
			assert.Error(t, snapErr)
			assert.Nil(t, snap)
			assert.Contains(t, snapErr.Error(), tc.errContains)
		})
	}
}

func TestGitTool_Cleanup(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	tool, err := NewGitTool(GitToolConfig{RepoDir: repoDir})
	require.NoError(t, err)

	ctx := context.Background()

	wt, err := tool.CreateWorktree(ctx, "cleanup-session")
	require.NoError(t, err)

	// Verify worktree exists before cleanup.
	_, err = os.Stat(wt.Path)
	require.NoError(t, err)

	err = tool.Cleanup(ctx, wt.Path)
	require.NoError(t, err)

	// Verify worktree directory was removed.
	_, err = os.Stat(wt.Path)
	assert.True(t, os.IsNotExist(err))
}

func TestGitTool_Cleanup_EmptyDir(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	tool, err := NewGitTool(GitToolConfig{RepoDir: repoDir})
	require.NoError(t, err)

	err = tool.Cleanup(context.Background(), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worktree directory is required")
}

func TestGitTool_ListWorktrees(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	tool, err := NewGitTool(GitToolConfig{RepoDir: repoDir})
	require.NoError(t, err)

	ctx := context.Background()

	// Initially no debate worktrees.
	list, err := tool.ListWorktrees(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)

	// Create two worktrees.
	wt1, err := tool.CreateWorktree(ctx, "list-s1")
	require.NoError(t, err)
	wt2, err := tool.CreateWorktree(ctx, "list-s2")
	require.NoError(t, err)

	list, err = tool.ListWorktrees(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	sessionIDs := make(map[string]bool)
	for _, w := range list {
		sessionIDs[w.SessionID] = true
	}
	assert.True(t, sessionIDs["list-s1"])
	assert.True(t, sessionIDs["list-s2"])

	_ = tool.Cleanup(ctx, wt1.Path)
	_ = tool.Cleanup(ctx, wt2.Path)
}

func TestGitTool_CleanupAll(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	tool, err := NewGitTool(GitToolConfig{RepoDir: repoDir})
	require.NoError(t, err)

	ctx := context.Background()

	_, err = tool.CreateWorktree(ctx, "cleanup-all-1")
	require.NoError(t, err)
	_, err = tool.CreateWorktree(ctx, "cleanup-all-2")
	require.NoError(t, err)

	err = tool.CleanupAll(ctx)
	require.NoError(t, err)

	list, err := tool.ListWorktrees(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestGitTool_CreateDiff(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	tool, err := NewGitTool(GitToolConfig{RepoDir: repoDir})
	require.NoError(t, err)

	ctx := context.Background()

	wt, err := tool.CreateWorktree(ctx, "diff-session")
	require.NoError(t, err)

	// First snapshot.
	snap1, err := tool.CommitSnapshot(
		ctx, wt.Path, "v1\n", "file.txt", "first version",
	)
	require.NoError(t, err)

	// Second snapshot.
	snap2, err := tool.CommitSnapshot(
		ctx, wt.Path, "v2\n", "file.txt", "second version",
	)
	require.NoError(t, err)

	diff, err := tool.CreateDiff(ctx, wt.Path, snap1.CommitHash, snap2.CommitHash)
	require.NoError(t, err)
	assert.Contains(t, diff, "-v1")
	assert.Contains(t, diff, "+v2")

	_ = tool.Cleanup(ctx, wt.Path)
}

func TestGitTool_CreateDiff_ValidationErrors(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	tool, err := NewGitTool(GitToolConfig{RepoDir: repoDir})
	require.NoError(t, err)

	tests := []struct {
		name        string
		wtDir       string
		from        string
		to          string
		errContains string
	}{
		{"empty worktree", "", "a", "b", "worktree directory is required"},
		{"empty fromRef", "/tmp", "", "b", "fromRef is required"},
		{"empty toRef", "/tmp", "a", "", "toRef is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
			_, diffErr := tool.CreateDiff(
				context.Background(), tc.wtDir, tc.from, tc.to,
			)
			assert.Error(t, diffErr)
			assert.Contains(t, diffErr.Error(), tc.errContains)
		})
	}
}

func TestGitTool_ContextCancellation(t *testing.T) {
	t.Parallel()
	if !gitAvailable() {
		t.Skip("git not available")
	}

	repoDir := setupTestGitRepo(t)
	tool, err := NewGitTool(GitToolConfig{RepoDir: repoDir})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(
		context.Background(), 1*time.Millisecond,
	)
	defer cancel()

	// Allow the context to expire.
	time.Sleep(5 * time.Millisecond)

	_, err = tool.CreateWorktree(ctx, "cancelled")
	assert.Error(t, err)
}
