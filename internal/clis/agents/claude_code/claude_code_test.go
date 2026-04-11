package claude_code

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"dev.helix.agent/internal/clis/agents/base"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()
	cc := New()
	require.NotNil(t, cc)

	info := cc.Info()
	assert.Equal(t, "Claude Code", info.Name)
	assert.True(t, info.IsEnabled)
	assert.Equal(t, 1, info.Priority)

	config := cc.GetConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "dark", config.Theme)
	assert.True(t, config.GitEnabled)
}

func TestClaudeCode_Initialize(t *testing.T) {
	t.Parallel()
	cc := New()

	tempDir := t.TempDir()
	config := &Config{
		BaseConfig: base.BaseConfig{
			Model:     "claude-3-5-sonnet",
			WorkDir:   tempDir,
			Timeout:   30,
			LogLevel:  "info",
			AutoStart: true,
		},
		EditorMode:     "vim",
		Theme:          "light",
		AutoCommit:     true,
		MCPEnabled:     true,
		TimeoutMinutes: 30,
	}

	ctx := context.Background()
	err := cc.Initialize(ctx, config)
	require.NoError(t, err)

	assert.Equal(t, tempDir, cc.workDir)
}

func TestClaudeCode_StartStop(t *testing.T) {
	t.Parallel()
	cc := New()

	ctx := context.Background()

	// Initialize with temp dir
	tempDir := t.TempDir()
	config := &Config{BaseConfig: base.BaseConfig{WorkDir: tempDir}}
	err := cc.Initialize(ctx, config)
	require.NoError(t, err)

	// Start
	err = cc.Start(ctx)
	require.NoError(t, err)
	assert.True(t, cc.IsStarted())

	// Stop
	err = cc.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, cc.IsStarted())
}

func TestClaudeCode_Execute_Chat(t *testing.T) {
	t.Parallel()
	cc := New()

	ctx := context.Background()
	tempDir := t.TempDir()
	config := &Config{BaseConfig: base.BaseConfig{WorkDir: tempDir}}
	cc.Initialize(ctx, config)

	result, err := cc.Execute(ctx, "chat", map[string]interface{}{
		"message": "Hello, Claude!",
	})

	require.NoError(t, err)
	response, ok := result.(*Response)
	require.True(t, ok)
	assert.True(t, response.Success)
	assert.NotEmpty(t, response.SessionID)
	assert.Contains(t, response.Content, "Claude Code received")
}

func TestClaudeCode_Execute_Bash(t *testing.T) {
	t.Parallel()
	cc := New()

	ctx := context.Background()
	tempDir := t.TempDir()
	config := &Config{BaseConfig: base.BaseConfig{WorkDir: tempDir}}
	cc.Initialize(ctx, config)

	result, err := cc.Execute(ctx, "bash", map[string]interface{}{
		"command": "echo hello",
	})

	require.NoError(t, err)
	response, ok := result.(*Response)
	require.True(t, ok)
	assert.True(t, response.Success)
	assert.Contains(t, response.Content, "hello")
}

func TestClaudeCode_Execute_Git(t *testing.T) {
	t.Parallel()
	cc := New()

	ctx := context.Background()
	tempDir := t.TempDir()

	// Initialize git repo
	exec.Command("git", "init", tempDir).Run()

	config := &Config{BaseConfig: base.BaseConfig{WorkDir: tempDir}}
	cc.Initialize(ctx, config)

	result, err := cc.Execute(ctx, "git", map[string]interface{}{
		"subcommand": "status",
	})

	require.NoError(t, err)
	response, ok := result.(*Response)
	require.True(t, ok)
	// Git status should succeed on a new repo
	assert.NotNil(t, response)
}

func TestClaudeCode_Execute_Edit(t *testing.T) {
	t.Parallel()
	cc := New()

	ctx := context.Background()
	tempDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("old content"), 0644)
	require.NoError(t, err)

	config := &Config{BaseConfig: base.BaseConfig{WorkDir: tempDir}}
	cc.Initialize(ctx, config)

	result, err := cc.Execute(ctx, "edit", map[string]interface{}{
		"file":        testFile,
		"instruction": "Change to new content",
		"content":     "new content",
	})

	require.NoError(t, err)
	response, ok := result.(*Response)
	require.True(t, ok)
	assert.True(t, response.Success)
	assert.Len(t, response.Actions, 1)
}

func TestClaudeCode_Execute_UnknownCommand(t *testing.T) {
	t.Parallel()
	cc := New()

	ctx := context.Background()
	tempDir := t.TempDir()
	config := &Config{BaseConfig: base.BaseConfig{WorkDir: tempDir}}
	cc.Initialize(ctx, config)

	_, err := cc.Execute(ctx, "unknown", map[string]interface{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestSession(t *testing.T) {
	t.Parallel()
	session := NewSession("/tmp/test", &Config{})

	assert.NotEmpty(t, session.ID)
	assert.Equal(t, "/tmp/test", session.WorkDir)
	assert.True(t, session.Active)
	assert.Empty(t, session.Messages)

	// Add messages
	session.AddMessage("user", "Hello")
	session.AddMessage("assistant", "Hi there!")

	assert.Len(t, session.Messages, 2)
	assert.Equal(t, "user", session.Messages[0].Role)

	// Get last messages
	last := session.GetLastMessages(1)
	assert.Len(t, last, 1)
	assert.Equal(t, "assistant", last[0].Role)

	// Clear
	session.Clear()
	assert.Empty(t, session.Messages)
}

func TestSession_IsExpired(t *testing.T) {
	t.Parallel()
	session := NewSession("/tmp/test", &Config{})

	// Not expired with 60 minute timeout
	assert.False(t, session.IsExpired(60))

	// Expired with 0 timeout (immediate)
	session.LastActivity = time.Now().Add(-1 * time.Minute)
	assert.True(t, session.IsExpired(0))
}

func TestSessionManager(t *testing.T) {
	t.Parallel()
	config := &Config{
		BaseConfig:     base.BaseConfig{WorkDir: t.TempDir()},
		TimeoutMinutes: 60,
	}

	sm := NewSessionManager(config)
	defer sm.Stop(context.Background())

	// Create session
	session1 := sm.CreateSession("/tmp/test1")
	assert.NotNil(t, session1)
	assert.NotEmpty(t, session1.ID)

	// Get session
	retrieved, ok := sm.GetSession(session1.ID)
	assert.True(t, ok)
	assert.Equal(t, session1.ID, retrieved.ID)

	// List sessions
	sessions := sm.ListSessions()
	assert.Len(t, sessions, 1)

	// Create another session
	session2 := sm.CreateSession("/tmp/test2")
	assert.Equal(t, 2, sm.GetSessionCount())
	assert.Equal(t, 2, sm.GetActiveSessionCount())

	// End session
	sm.EndSession(session1.ID)
	assert.Equal(t, 2, sm.GetSessionCount())
	assert.Equal(t, 1, sm.GetActiveSessionCount())

	// Delete session
	sm.DeleteSession(session2.ID)
	assert.Equal(t, 1, sm.GetSessionCount())
}

func TestToolExecutor(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	te := NewToolExecutor(tempDir, []string{"read_file", "write_file", "bash"})

	ctx := context.Background()

	// Test write_file
	result, err := te.ExecuteTool(ctx, "write_file", map[string]interface{}{
		"file_path": "test.txt",
		"content":   "Hello, World!",
	})
	require.NoError(t, err)
	assert.True(t, result.Success)

	// Test read_file
	result, err = te.ExecuteTool(ctx, "read_file", map[string]interface{}{
		"file_path": "test.txt",
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "Hello, World!", result.Output)

	// Test disallowed tool
	result, err = te.ExecuteTool(ctx, "disallowed_tool", map[string]interface{}{})
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "not allowed")
}

func TestMCPIntegration(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp.json")

	mcp := NewMCPIntegration(configPath)

	// Load config (should create default)
	err := mcp.LoadConfig()
	require.NoError(t, err)

	// Check that config was created
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Get servers
	servers := mcp.GetServers()
	assert.NotEmpty(t, servers)
	assert.Contains(t, servers, "filesystem")

	// Get specific server
	server, ok := mcp.GetServer("filesystem")
	assert.True(t, ok)
	assert.Equal(t, "filesystem", server.Name)
	assert.True(t, server.Enabled)

	// Enable/Disable
	err = mcp.EnableServer("filesystem", false)
	require.NoError(t, err)

	server, _ = mcp.GetServer("filesystem")
	assert.False(t, server.Enabled)
}

func TestMCPIntegration_CallTool(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mcp.json")

	mcp := NewMCPIntegration(configPath)
	mcp.LoadConfig()

	ctx := context.Background()
	result, err := mcp.CallTool(ctx, "filesystem", "read_file", map[string]interface{}{
		"path": "/tmp/test",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func BenchmarkClaudeCode_Execute(b *testing.B) {
	cc := New()
	ctx := context.Background()
	tempDir := b.TempDir()
	config := &Config{BaseConfig: base.BaseConfig{WorkDir: tempDir}}
	cc.Initialize(ctx, config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cc.Execute(ctx, "chat", map[string]interface{}{
			"message": "Test message",
		})
	}
}
