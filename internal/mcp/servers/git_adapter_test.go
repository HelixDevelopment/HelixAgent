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

func TestNewGitAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultGitAdapterConfig()
	adapter := NewGitAdapter(config, nil)

	assert.NotNil(t, adapter)
	assert.False(t, adapter.initialized)
	assert.Equal(t, "git", adapter.config.GitPath)
}

func TestDefaultGitAdapterConfig(t *testing.T) {
	t.Parallel()
	config := DefaultGitAdapterConfig()

	homeDir, _ := os.UserHomeDir()
	assert.Contains(t, config.AllowedPaths, homeDir)
	assert.False(t, config.AllowPush)
	assert.False(t, config.AllowForce)
	assert.True(t, config.AllowRemoteOperations)
	assert.Equal(t, "git", config.GitPath)
	assert.Equal(t, 120*time.Second, config.CommandTimeout)
}

func TestGitAdapter_Initialize(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	config := DefaultGitAdapterConfig()
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)
	assert.True(t, adapter.initialized)
	assert.Contains(t, adapter.gitVersion, "git version")
}

func TestGitAdapter_Initialize_InvalidGit(t *testing.T) {
	t.Parallel()
	config := DefaultGitAdapterConfig()
	config.GitPath = "/nonexistent/git"
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "git not available")
}

func TestGitAdapter_Health(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	config := DefaultGitAdapterConfig()
	adapter := NewGitAdapter(config, logrus.New())

	// Health check should fail if not initialized
	err := adapter.Health(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Initialize and check again
	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	err = adapter.Health(context.Background())
	assert.NoError(t, err)
}

func TestGitAdapter_Status(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a temp git repo
	tempDir, err := os.MkdirTemp("", "git-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Configure git for the test repo
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	status, err := adapter.Status(context.Background(), tempDir)
	require.NoError(t, err)
	assert.NotNil(t, status)
	// New repo has no tracking branch so may show master/main depending on git config
	assert.True(t, status.IsClean)
}

func TestGitAdapter_Status_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitAdapterConfig()
	adapter := NewGitAdapter(config, logrus.New())

	_, err := adapter.Status(context.Background(), "/tmp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitAdapter_Status_WithFiles(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a temp git repo
	tempDir, err := os.MkdirTemp("", "git-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Configure git for the test repo
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Create an untracked file
	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	status, err := adapter.Status(context.Background(), tempDir)
	require.NoError(t, err)
	assert.False(t, status.IsClean)
	assert.Contains(t, status.Untracked, "test.txt")
}

func TestGitAdapter_Add(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a temp git repo
	tempDir, err := os.MkdirTemp("", "git-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Create a file
	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	// Add the file
	err = adapter.Add(context.Background(), tempDir, []string{"test.txt"}, false)
	assert.NoError(t, err)

	// Check status
	status, err := adapter.Status(context.Background(), tempDir)
	require.NoError(t, err)
	assert.Contains(t, status.Staged, "test.txt")
}

func TestGitAdapter_Add_All(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a temp git repo
	tempDir, err := os.MkdirTemp("", "git-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Create files
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "test1.txt"), []byte("test1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "test2.txt"), []byte("test2"), 0644))

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	// Add all files
	err = adapter.Add(context.Background(), tempDir, nil, true)
	assert.NoError(t, err)

	// Check status
	status, err := adapter.Status(context.Background(), tempDir)
	require.NoError(t, err)
	assert.Len(t, status.Untracked, 0)
	assert.GreaterOrEqual(t, len(status.Staged), 2)
}

func TestGitAdapter_Commit(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a temp git repo
	tempDir, err := os.MkdirTemp("", "git-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Configure git for the test repo
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Create and stage a file
	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	// Commit
	result, err := adapter.Commit(context.Background(), tempDir, "Test commit", false)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Hash)
	assert.NotEmpty(t, result.ShortHash)
	assert.Equal(t, "Test commit", result.Subject)
}

func TestGitAdapter_Commit_NoMessage(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	config := DefaultGitAdapterConfig()
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, err = adapter.Commit(context.Background(), "/tmp", "", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "commit message required")
}

func TestGitAdapter_Commit_Amend_NotAllowed(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	config := DefaultGitAdapterConfig()
	config.AllowForce = false
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, err = adapter.Commit(context.Background(), "/tmp", "Test", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "amend not allowed")
}

func TestGitAdapter_Log(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a temp git repo
	tempDir, err := os.MkdirTemp("", "git-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Configure git for the test repo
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Create and commit files
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(tempDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test content "+string(rune('A'+i))), 0644))

		cmd = exec.Command("git", "add", "test.txt")
		cmd.Dir = tempDir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "Commit "+string(rune('A'+i)))
		cmd.Dir = tempDir
		require.NoError(t, cmd.Run())
	}

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	logs, err := adapter.Log(context.Background(), tempDir, 10, "", "")
	require.NoError(t, err)
	assert.Len(t, logs, 3)
	assert.Equal(t, "Commit C", logs[0].Subject)
	assert.Equal(t, "Test User", logs[0].Author)
}

func TestGitAdapter_Branch(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a temp git repo
	tempDir, err := os.MkdirTemp("", "git-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repo
	cmd := exec.Command("git", "init")
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
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	// Create a branch
	branches, err := adapter.Branch(context.Background(), tempDir, "feature-branch", true, false)
	require.NoError(t, err)
	assert.Contains(t, branches, "feature-branch")

	// List branches
	branches, err = adapter.Branch(context.Background(), tempDir, "", false, false)
	require.NoError(t, err)
	assert.Contains(t, branches, "feature-branch")
}

func TestGitAdapter_Checkout(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a temp git repo
	tempDir, err := os.MkdirTemp("", "git-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repo
	cmd := exec.Command("git", "init")
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
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	// Create and checkout new branch
	err = adapter.Checkout(context.Background(), tempDir, "new-branch", true, nil)
	require.NoError(t, err)

	// Check we're on new branch
	status, err := adapter.Status(context.Background(), tempDir)
	require.NoError(t, err)
	assert.Equal(t, "new-branch", status.Branch)
}

func TestGitAdapter_Diff(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a temp git repo
	tempDir, err := os.MkdirTemp("", "git-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repo
	cmd := exec.Command("git", "init")
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

	// Modify the file
	require.NoError(t, os.WriteFile(testFile, []byte("modified content\nnew line"), 0644))

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	diff, err := adapter.Diff(context.Background(), tempDir, "", "", nil, false)
	require.NoError(t, err)
	assert.NotNil(t, diff)
	assert.Len(t, diff.Files, 1)
	assert.Equal(t, "test.txt", diff.Files[0].Path)
}

func TestGitAdapter_Remotes(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a temp git repo
	tempDir, err := os.MkdirTemp("", "git-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Add a remote
	cmd = exec.Command("git", "remote", "add", "origin", "https://example.com/test.git")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	remotes, err := adapter.Remotes(context.Background(), tempDir)
	require.NoError(t, err)
	assert.Len(t, remotes, 1)
	assert.Equal(t, "origin", remotes[0].Name)
	assert.Equal(t, "https://example.com/test.git", remotes[0].FetchURL)
}

func TestGitAdapter_Push_NotAllowed(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	config := DefaultGitAdapterConfig()
	config.AllowPush = false
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	err = adapter.Push(context.Background(), "/tmp", "origin", "main", false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "push not allowed")
}

func TestGitAdapter_Push_ForceNotAllowed(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	config := DefaultGitAdapterConfig()
	config.AllowPush = true
	config.AllowForce = false
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	err = adapter.Push(context.Background(), "/tmp", "origin", "main", true, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "force push not allowed")
}

func TestGitAdapter_RemoteOperations_NotAllowed(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	config := DefaultGitAdapterConfig()
	config.AllowRemoteOperations = false
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	err = adapter.Fetch(context.Background(), "/tmp", "origin", false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "remote operations not allowed")

	err = adapter.Pull(context.Background(), "/tmp", "origin", "main", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "remote operations not allowed")
}

func TestGitAdapter_Stash(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a temp git repo
	tempDir, err := os.MkdirTemp("", "git-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repo
	cmd := exec.Command("git", "init")
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

	// Modify the file
	require.NoError(t, os.WriteFile(testFile, []byte("modified content"), 0644))

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	// Stash changes
	_, err = adapter.Stash(context.Background(), tempDir, "Test stash", false, false)
	require.NoError(t, err)

	// List stashes
	result, err := adapter.Stash(context.Background(), tempDir, "", false, true)
	require.NoError(t, err)
	assert.Contains(t, result, "Test stash")
}

func TestGitAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	config := DefaultGitAdapterConfig()
	adapter := NewGitAdapter(config, logrus.New())

	tools := adapter.GetMCPTools()
	assert.Len(t, tools, 13)

	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "git_status")
	assert.Contains(t, toolNames, "git_log")
	assert.Contains(t, toolNames, "git_diff")
	assert.Contains(t, toolNames, "git_add")
	assert.Contains(t, toolNames, "git_commit")
	assert.Contains(t, toolNames, "git_branch")
	assert.Contains(t, toolNames, "git_checkout")
	assert.Contains(t, toolNames, "git_push")
	assert.Contains(t, toolNames, "git_pull")
	assert.Contains(t, toolNames, "git_fetch")
	assert.Contains(t, toolNames, "git_remotes")
	assert.Contains(t, toolNames, "git_stash")
	assert.Contains(t, toolNames, "git_clone")
}

func TestGitAdapter_ExecuteTool(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	// Create a temp git repo
	tempDir, err := os.MkdirTemp("", "git-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	// Execute git_status tool
	result, err := adapter.ExecuteTool(context.Background(), "git_status", map[string]interface{}{
		"repo_path": tempDir,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)

	status, ok := result.(*GitStatus)
	require.True(t, ok)
	assert.True(t, status.IsClean)
}

func TestGitAdapter_ExecuteTool_Unknown(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	config := DefaultGitAdapterConfig()
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, err = adapter.ExecuteTool(context.Background(), "unknown_tool", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestGitAdapter_ExecuteTool_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultGitAdapterConfig()
	adapter := NewGitAdapter(config, logrus.New())

	_, err := adapter.ExecuteTool(context.Background(), "git_status", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGitAdapter_Close(t *testing.T) {
	t.Parallel()
	config := DefaultGitAdapterConfig()
	adapter := NewGitAdapter(config, logrus.New())

	// Initialize if git available
	if _, err := exec.LookPath("git"); err == nil {
		err := adapter.Initialize(context.Background())
		require.NoError(t, err)
		assert.True(t, adapter.initialized)
	}

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.initialized)
}

func TestGitAdapter_GetCapabilities(t *testing.T) {
	t.Parallel()
	config := DefaultGitAdapterConfig()
	config.AllowPush = true
	config.AllowForce = true
	adapter := NewGitAdapter(config, logrus.New())

	caps := adapter.GetCapabilities()
	assert.Equal(t, "git", caps["name"])
	assert.Equal(t, true, caps["allow_push"])
	assert.Equal(t, true, caps["allow_force"])
	assert.Equal(t, true, caps["allow_remote_operations"])
	assert.Equal(t, 13, caps["tools"])
}

func TestGitAdapter_PathNotAllowed(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{"/allowed/path"}
	config.DeniedPaths = []string{}
	adapter := NewGitAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, err = adapter.Status(context.Background(), "/not/allowed/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path not allowed")
}

func TestGitAdapter_DeniedPaths(t *testing.T) {
	t.Parallel()
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available") // SKIP-OK: #legacy-untriaged
	}

	tempDir, err := os.MkdirTemp("", "git-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	config := DefaultGitAdapterConfig()
	config.AllowedPaths = []string{tempDir}
	config.DeniedPaths = []string{tempDir}
	adapter := NewGitAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, err = adapter.Status(context.Background(), tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path not allowed")
}
