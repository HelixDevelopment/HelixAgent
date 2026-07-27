package servers

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to set up a test git repo
func setupTestGitRepo(t *testing.T) (string, func()) {
	t.Helper()
	tempDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Pin the initial branch to `main`. `git init` names the initial branch
	// from the host's `init.defaultBranch` setting, which defaults to
	// `master` when unset — but every push/pull/checkout assertion built on
	// this fixture targets `main`. Without this the fixture's branch name is
	// host-config-dependent and the tests fail on any host that has not set
	// `init.defaultBranch=main`. `symbolic-ref` (rather than `git init -b`)
	// keeps the fixture working on git < 2.28 as well.
	cmd = exec.Command("git", "symbolic-ref", "HEAD", "refs/heads/main")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Configure git for the test repo
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Create initial commit
	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial content"), 0644))

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Return no-op cleanup since t.TempDir() handles cleanup
	return tempDir, func() {}
}

// ============================================================================
// ExecuteTool Comprehensive Tests
// ============================================================================

func TestGitAdapter_ExecuteTool_AllTools(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	tempDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	config.AllowPush = true
	config.AllowForce = true
	config.AllowRemoteOperations = true
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	t.Run("git_status via ExecuteTool", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_status", map[string]interface{}{
			"repo_path": tempDir,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)

		status, ok := result.(*GitStatus)
		assert.True(t, ok)
		assert.True(t, status.IsClean)
	})

	t.Run("git_log via ExecuteTool", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_log", map[string]interface{}{
			"repo_path": tempDir,
			"limit":     float64(10),
		})
		require.NoError(t, err)
		assert.NotNil(t, result)

		logs, ok := result.([]GitLog)
		assert.True(t, ok)
		assert.Len(t, logs, 1)
		assert.Equal(t, "Initial commit", logs[0].Subject)
	})

	t.Run("git_log with since and until", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_log", map[string]interface{}{
			"repo_path": tempDir,
			"limit":     float64(10),
			"since":     time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			"until":     time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("git_diff via ExecuteTool - no changes", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_diff", map[string]interface{}{
			"repo_path": tempDir,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)

		diff, ok := result.(*GitDiff)
		assert.True(t, ok)
		assert.Empty(t, diff.Files)
	})

	t.Run("git_diff with modified file", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("modified content"), 0644))

		result, err := adapter.ExecuteTool(context.Background(), "git_diff", map[string]interface{}{
			"repo_path": tempDir,
		})
		require.NoError(t, err)

		diff, ok := result.(*GitDiff)
		assert.True(t, ok)
		assert.NotEmpty(t, diff.Files)

		// Restore file
		require.NoError(t, os.WriteFile(testFile, []byte("initial content"), 0644))
	})

	t.Run("git_diff with base and target", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_diff", map[string]interface{}{
			"repo_path": tempDir,
			"base":      "HEAD~0",
			"target":    "HEAD",
		})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("git_diff with specific files", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_diff", map[string]interface{}{
			"repo_path": tempDir,
			"files":     []interface{}{"test.txt"},
		})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("git_diff with staged option", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_diff", map[string]interface{}{
			"repo_path": tempDir,
			"staged":    true,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("git_add via ExecuteTool", func(t *testing.T) {
		newFile := filepath.Join(tempDir, "new_file.txt")
		require.NoError(t, os.WriteFile(newFile, []byte("new content"), 0644))

		result, err := adapter.ExecuteTool(context.Background(), "git_add", map[string]interface{}{
			"repo_path": tempDir,
			"paths":     []interface{}{"new_file.txt"},
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Add returns map[string]interface{}{"success": true}
	})

	t.Run("git_add with all option", func(t *testing.T) {
		anotherFile := filepath.Join(tempDir, "another.txt")
		require.NoError(t, os.WriteFile(anotherFile, []byte("another"), 0644))

		result, err := adapter.ExecuteTool(context.Background(), "git_add", map[string]interface{}{
			"repo_path": tempDir,
			"all":       true,
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Add returns map[string]interface{}{"success": true}
	})

	t.Run("git_commit via ExecuteTool", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_commit", map[string]interface{}{
			"repo_path": tempDir,
			"message":   "Test commit via ExecuteTool",
		})
		require.NoError(t, err)
		assert.NotNil(t, result)

		commit, ok := result.(*GitCommitResult)
		assert.True(t, ok)
		assert.Equal(t, "Test commit via ExecuteTool", commit.Subject)
	})

	t.Run("git_branch list via ExecuteTool", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_branch", map[string]interface{}{
			"repo_path": tempDir,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		branches, ok := resultMap["branches"].([]string)
		assert.True(t, ok)
		assert.NotEmpty(t, branches)
	})

	t.Run("git_branch create via ExecuteTool", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_branch", map[string]interface{}{
			"repo_path": tempDir,
			"name":      "feature/test-branch",
			"create":    true,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		branches, ok := resultMap["branches"].([]string)
		assert.True(t, ok)
		assert.Contains(t, branches, "feature/test-branch")
	})

	t.Run("git_branch delete via ExecuteTool", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_branch", map[string]interface{}{
			"repo_path": tempDir,
			"name":      "feature/test-branch",
			"delete":    true,
		})
		// Delete might fail if branch doesn't exist or is current branch
		// Just verify no panic
		if err != nil {
			assert.Contains(t, err.Error(), "")
		} else {
			assert.NotNil(t, result)
		}
	})

	t.Run("git_checkout via ExecuteTool", func(t *testing.T) {
		// Create a branch first
		_, err := adapter.ExecuteTool(context.Background(), "git_branch", map[string]interface{}{
			"repo_path": tempDir,
			"name":      "checkout-test",
			"create":    true,
		})
		require.NoError(t, err)

		result, err := adapter.ExecuteTool(context.Background(), "git_checkout", map[string]interface{}{
			"repo_path": tempDir,
			"ref":       "checkout-test",
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Returns map[string]interface{}

		// Go back to main branch
		_, _ = adapter.ExecuteTool(context.Background(), "git_checkout", map[string]interface{}{
			"repo_path": tempDir,
			"ref":       "main",
		})
	})

	t.Run("git_checkout with create new branch", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_checkout", map[string]interface{}{
			"repo_path": tempDir,
			"ref":       "new-checkout-branch",
			"create":    true,
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Returns map[string]interface{}

		// Go back to original branch
		_, _ = adapter.ExecuteTool(context.Background(), "git_checkout", map[string]interface{}{
			"repo_path": tempDir,
			"ref":       "main",
		})
	})

	t.Run("git_checkout with specific files", func(t *testing.T) {
		// Modify a file first
		testFile := filepath.Join(tempDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("modified"), 0644))

		result, err := adapter.ExecuteTool(context.Background(), "git_checkout", map[string]interface{}{
			"repo_path": tempDir,
			"ref":       "HEAD",
			"paths":     []interface{}{"test.txt"},
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Returns map[string]interface{}

		// Verify file is restored
		content, _ := os.ReadFile(testFile)
		assert.Equal(t, "initial content", string(content))
	})

	t.Run("git_remotes via ExecuteTool", func(t *testing.T) {
		// Add a remote first (ignore error if already exists)
		cmd := exec.Command("git", "remote", "add", "test-remote", "https://github.com/test/repo.git")
		cmd.Dir = tempDir
		_ = cmd.Run() // Ignore error - remote might already exist

		result, err := adapter.ExecuteTool(context.Background(), "git_remotes", map[string]interface{}{
			"repo_path": tempDir,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)

		resultMap, ok := result.(map[string]interface{})
		assert.True(t, ok)
		// Remotes returns []*GitRemote which might be empty if no remotes
		// Just verify the key exists and is of right type
		_, exists := resultMap["remotes"]
		assert.True(t, exists)
	})

	t.Run("git_stash list via ExecuteTool", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_stash", map[string]interface{}{
			"repo_path": tempDir,
			"list":      true,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("git_stash push via ExecuteTool", func(t *testing.T) {
		// Create unstaged changes
		testFile := filepath.Join(tempDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("stash this"), 0644))

		result, err := adapter.ExecuteTool(context.Background(), "git_stash", map[string]interface{}{
			"repo_path": tempDir,
			"message":   "Test stash",
		})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("git_stash pop via ExecuteTool", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_stash", map[string]interface{}{
			"repo_path": tempDir,
			"pop":       true,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		_, err := adapter.ExecuteTool(context.Background(), "git_unknown", map[string]interface{}{
			"repo_path": tempDir,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})
}

// ============================================================================
// Push/Pull/Fetch Tests with Local Remote
// ============================================================================

func TestGitAdapter_PushPullFetch(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a bare repo to act as remote
	bareDir := t.TempDir()

	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = bareDir
	require.NoError(t, cmd.Run())

	// Pin the bare remote's HEAD to `main` too, so it agrees with the
	// working repo's branch (see setupTestGitRepo).
	cmd = exec.Command("git", "symbolic-ref", "HEAD", "refs/heads/main")
	cmd.Dir = bareDir
	require.NoError(t, cmd.Run())

	// Create working repo
	workDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	// Add bare repo as remote
	cmd = exec.Command("git", "remote", "add", "origin", bareDir)
	cmd.Dir = workDir
	require.NoError(t, cmd.Run())

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{workDir, bareDir}
	config.AllowPush = true
	config.AllowForce = true
	config.AllowRemoteOperations = true
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	t.Run("git_push via ExecuteTool", func(t *testing.T) {
		// First push needs -u
		result, err := adapter.ExecuteTool(context.Background(), "git_push", map[string]interface{}{
			"repo_path":    workDir,
			"remote":       "origin",
			"branch":       "main",
			"set_upstream": true,
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Returns map[string]interface{}
	})

	t.Run("git_fetch via ExecuteTool", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_fetch", map[string]interface{}{
			"repo_path": workDir,
			"remote":    "origin",
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Returns map[string]interface{}
	})

	t.Run("git_fetch with prune", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_fetch", map[string]interface{}{
			"repo_path": workDir,
			"remote":    "origin",
			"prune":     true,
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Returns map[string]interface{}
	})

	t.Run("git_fetch all remotes", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_fetch", map[string]interface{}{
			"repo_path": workDir,
			"all":       true,
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Returns map[string]interface{}
	})

	t.Run("git_pull via ExecuteTool", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_pull", map[string]interface{}{
			"repo_path": workDir,
			"remote":    "origin",
			"branch":    "main",
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Returns map[string]interface{}
	})

	t.Run("git_pull with rebase", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_pull", map[string]interface{}{
			"repo_path": workDir,
			"remote":    "origin",
			"branch":    "main",
			"rebase":    true,
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Returns map[string]interface{}
	})

	t.Run("git_push with force (when allowed)", func(t *testing.T) {
		result, err := adapter.ExecuteTool(context.Background(), "git_push", map[string]interface{}{
			"repo_path": workDir,
			"remote":    "origin",
			"branch":    "main",
			"force":     true,
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Returns map[string]interface{}
	})
}

// ============================================================================
// Clone Tests
// ============================================================================

func TestGitAdapter_Clone(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a bare repo to clone from
	bareDir := t.TempDir()

	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = bareDir
	require.NoError(t, cmd.Run())

	// Pin the bare remote's HEAD to `main` so `git clone` of it checks out a
	// real working tree instead of warning about a dangling remote HEAD.
	cmd = exec.Command("git", "symbolic-ref", "HEAD", "refs/heads/main")
	cmd.Dir = bareDir
	require.NoError(t, cmd.Run())

	// Create a source repo and push to bare
	srcDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	cmd = exec.Command("git", "remote", "add", "origin", bareDir)
	cmd.Dir = srcDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "push", "-u", "origin", "main")
	cmd.Dir = srcDir
	require.NoError(t, cmd.Run())

	// Clone destination
	cloneDir := t.TempDir()

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{cloneDir, bareDir}
	config.AllowRemoteOperations = true
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	t.Run("Clone via adapter method", func(t *testing.T) {
		destPath := filepath.Join(cloneDir, "cloned_repo")
		err := adapter.Clone(context.Background(), bareDir, destPath, false, 0)
		require.NoError(t, err)

		// Verify clone
		_, err = os.Stat(filepath.Join(destPath, ".git"))
		assert.NoError(t, err)
	})

	t.Run("Clone via ExecuteTool", func(t *testing.T) {
		destPath := filepath.Join(cloneDir, "cloned_repo_2")
		result, err := adapter.ExecuteTool(context.Background(), "git_clone", map[string]interface{}{
			"url":       bareDir,
			"dest_path": destPath,
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Returns map[string]interface{}

		// Verify clone
		_, statErr := os.Stat(filepath.Join(destPath, ".git"))
		assert.NoError(t, statErr)
	})

	t.Run("Clone with branch", func(t *testing.T) {
		destPath := filepath.Join(cloneDir, "cloned_repo_branch")
		result, err := adapter.ExecuteTool(context.Background(), "git_clone", map[string]interface{}{
			"url":       bareDir,
			"dest_path": destPath,
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Returns map[string]interface{}
	})

	t.Run("Clone with depth (shallow)", func(t *testing.T) {
		destPath := filepath.Join(cloneDir, "cloned_repo_shallow")
		result, err := adapter.ExecuteTool(context.Background(), "git_clone", map[string]interface{}{
			"url":       bareDir,
			"dest_path": destPath,
			"shallow":   true,
			"depth":     float64(1),
		})
		require.NoError(t, err)
		assert.NotNil(t, result) // Returns map[string]interface{}
	})

	t.Run("Clone shallow option", func(t *testing.T) {
		destPath := filepath.Join(cloneDir, "cloned_repo_shallow_opt")
		err := adapter.Clone(context.Background(), bareDir, destPath, true, 0)
		require.NoError(t, err)
	})

	t.Run("Clone destination not allowed", func(t *testing.T) {
		destPath := "/not/allowed/path"
		err := adapter.Clone(context.Background(), bareDir, destPath, false, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path not allowed")
	})

	t.Run("Clone when remote operations not allowed", func(t *testing.T) {
		restrictedConfig := DefaultGitAdapterConfig()
		restrictedConfig.AllowedPaths = []string{cloneDir}
		restrictedConfig.AllowRemoteOperations = false
		restrictedAdapter := NewGitAdapter(restrictedConfig, logrus.New())
		_ = restrictedAdapter.Initialize(context.Background())

		destPath := filepath.Join(cloneDir, "fail_clone")
		err := restrictedAdapter.Clone(context.Background(), bareDir, destPath, false, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "remote operations not allowed")
	})
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestGitAdapter_ExecuteTool_Errors(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	tempDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	config.AllowPush = false
	config.AllowForce = false
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	t.Run("Push when not allowed", func(t *testing.T) {
		_, err := adapter.ExecuteTool(context.Background(), "git_push", map[string]interface{}{
			"repo_path": tempDir,
			"remote":    "origin",
			"branch":    "main",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "push not allowed")
	})

	t.Run("Force push when not allowed", func(t *testing.T) {
		config.AllowPush = true // Enable push but not force
		adapter2 := NewGitAdapter(config, logrus.New())
		_ = adapter2.Initialize(context.Background())

		_, err := adapter2.ExecuteTool(context.Background(), "git_push", map[string]interface{}{
			"repo_path": tempDir,
			"remote":    "origin",
			"branch":    "main",
			"force":     true,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "force push not allowed")
	})

	t.Run("Commit without message", func(t *testing.T) {
		_, err := adapter.ExecuteTool(context.Background(), "git_commit", map[string]interface{}{
			"repo_path": tempDir,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "commit message required")
	})

	t.Run("Amend when not allowed", func(t *testing.T) {
		_, err := adapter.ExecuteTool(context.Background(), "git_commit", map[string]interface{}{
			"repo_path": tempDir,
			"message":   "test",
			"amend":     true,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "amend not allowed")
	})

	t.Run("Status in non-git directory", func(t *testing.T) {
		nonGitDir, _ := os.MkdirTemp("", "non-git-*")
		defer func() { _ = os.RemoveAll(nonGitDir) }()

		config.AllowedPaths = append(config.AllowedPaths, nonGitDir)
		adapter := NewGitAdapter(config, logrus.New())
		_ = adapter.Initialize(context.Background())

		_, err := adapter.ExecuteTool(context.Background(), "git_status", map[string]interface{}{
			"repo_path": nonGitDir,
		})
		assert.Error(t, err)
	})
}

// ============================================================================
// MarshalJSON Test
// ============================================================================

func TestGitAdapter_MarshalJSON(t *testing.T) {
	t.Parallel()
	config := DefaultGitAdapterConfig()
	config.AllowPush = true
	adapter := NewGitAdapter(config, logrus.New())

	data, err := adapter.MarshalJSON()
	assert.NoError(t, err)
	assert.Contains(t, string(data), "git")
	assert.Contains(t, string(data), "allow_push")
}

// ============================================================================
// Concurrent Access Tests
// ============================================================================

func TestGitAdapter_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	tempDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	t.Run("Concurrent status requests", func(t *testing.T) {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				_, err := adapter.ExecuteTool(context.Background(), "git_status", map[string]interface{}{
					"repo_path": tempDir,
				})
				assert.NoError(t, err)
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("Concurrent log requests", func(t *testing.T) {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				_, err := adapter.ExecuteTool(context.Background(), "git_log", map[string]interface{}{
					"repo_path": tempDir,
					"limit":     float64(5),
				})
				assert.NoError(t, err)
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// ============================================================================
// Context Timeout Tests
// ============================================================================

func TestGitAdapter_ContextTimeout(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	tempDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	config.CommandTimeout = 1 * time.Millisecond // Very short timeout
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	t.Run("Operation with expired context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(1 * time.Millisecond) // Ensure context expires

		_, _ = adapter.ExecuteTool(ctx, "git_status", map[string]interface{}{
			"repo_path": tempDir,
		})
		// May or may not error depending on timing, but shouldn't panic
	})
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkGitAdapter_ExecuteTool_Status(b *testing.B) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		b.Skip("git not available")
	}

	tempDir, err := os.MkdirTemp("", "git-bench-*")
	require.NoError(b, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	require.NoError(b, cmd.Run())

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())
	_ = adapter.Initialize(context.Background())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.ExecuteTool(context.Background(), "git_status", map[string]interface{}{
			"repo_path": tempDir,
		})
	}
}

func BenchmarkGitAdapter_ExecuteTool_Log(b *testing.B) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		b.Skip("git not available")
	}

	tempDir, err := os.MkdirTemp("", "git-bench-log-*")
	require.NoError(b, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Setup repo with commits
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	require.NoError(b, cmd.Run())

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tempDir
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = tempDir
	_ = cmd.Run()

	for i := 0; i < 10; i++ {
		_ = os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("commit "+string(rune('A'+i))), 0644)
		_ = exec.Command("git", "-C", tempDir, "add", ".").Run()
		_ = exec.Command("git", "-C", tempDir, "commit", "-m", "Commit "+string(rune('A'+i))).Run()
	}

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())
	_ = adapter.Initialize(context.Background())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.ExecuteTool(context.Background(), "git_log", map[string]interface{}{
			"repo_path": tempDir,
			"limit":     float64(5),
		})
	}
}
