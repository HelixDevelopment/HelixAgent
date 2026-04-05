package servers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFilesystemAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultFilesystemAdapterConfig()
	adapter := NewFilesystemAdapter(config)

	assert.NotNil(t, adapter)
	assert.Equal(t, config, adapter.config)
	assert.False(t, adapter.initialized)
}

func TestDefaultFilesystemAdapterConfig(t *testing.T) {
	t.Parallel()
	config := DefaultFilesystemAdapterConfig()

	assert.NotEmpty(t, config.AllowedPaths)
	assert.NotEmpty(t, config.DeniedPaths)
	assert.Equal(t, int64(10*1024*1024), config.MaxFileSize)
	assert.False(t, config.AllowWrite)
	assert.False(t, config.AllowDelete)
	assert.False(t, config.AllowCreateDir)
	assert.False(t, config.FollowSymlinks)
}

func TestFilesystemAdapter_Initialize(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()

	err = adapter.Initialize(ctx)
	assert.NoError(t, err)
	assert.True(t, adapter.initialized)

	// Test idempotent initialization
	err = adapter.Initialize(ctx)
	assert.NoError(t, err)
}

func TestFilesystemAdapter_Initialize_InvalidPath(t *testing.T) {
	t.Parallel()
	config := FilesystemAdapterConfig{
		AllowedPaths: []string{"/nonexistent/path/12345"},
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()

	err := adapter.Initialize(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestFilesystemAdapter_Health(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()

	// Before initialization
	err = adapter.Health(ctx)
	assert.Error(t, err)

	// After initialization
	require.NoError(t, adapter.Initialize(ctx))
	err = adapter.Health(ctx)
	assert.NoError(t, err)
}

func TestFilesystemAdapter_ReadFile(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!"
	require.NoError(t, os.WriteFile(testFile, []byte(testContent), 0644))

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	content, err := adapter.ReadFile(ctx, testFile)
	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Equal(t, testContent, content.Content)
	assert.Equal(t, testFile, content.Path)
	assert.Equal(t, int64(len(testContent)), content.Size)
}

func TestFilesystemAdapter_ReadFile_NotInitialized(t *testing.T) {
	t.Parallel()
	adapter := NewFilesystemAdapter(DefaultFilesystemAdapterConfig())
	ctx := context.Background()

	_, err := adapter.ReadFile(ctx, "/some/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestFilesystemAdapter_ReadFile_PathNotAllowed(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	_, err = adapter.ReadFile(ctx, "/etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside allowed")
}

func TestFilesystemAdapter_ReadFile_TooLarge(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a file larger than max size
	testFile := filepath.Join(tempDir, "large.txt")
	require.NoError(t, os.WriteFile(testFile, make([]byte, 1024), 0644))

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  100, // Very small limit
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	_, err = adapter.ReadFile(ctx, testFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestFilesystemAdapter_WriteFile(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
		AllowWrite:   true,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	testFile := filepath.Join(tempDir, "new_file.txt")
	testContent := "New content"

	err = adapter.WriteFile(ctx, testFile, testContent)
	assert.NoError(t, err)

	// Verify file was written
	content, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}

func TestFilesystemAdapter_WriteFile_NotAllowed(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		AllowWrite:   false,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	err = adapter.WriteFile(ctx, filepath.Join(tempDir, "test.txt"), "content")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}

func TestFilesystemAdapter_AppendFile(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	testFile := filepath.Join(tempDir, "append.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("Initial"), 0644))

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
		AllowWrite:   true,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	err = adapter.AppendFile(ctx, testFile, " Appended")
	assert.NoError(t, err)

	content, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, "Initial Appended", string(content))
}

func TestFilesystemAdapter_DeleteFile(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	testFile := filepath.Join(tempDir, "delete.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("To delete"), 0644))

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		AllowDelete:  true,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	err = adapter.DeleteFile(ctx, testFile)
	assert.NoError(t, err)

	_, err = os.Stat(testFile)
	assert.True(t, os.IsNotExist(err))
}

func TestFilesystemAdapter_DeleteFile_NotAllowed(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		AllowDelete:  false,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	err = adapter.DeleteFile(ctx, filepath.Join(tempDir, "test.txt"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}

func TestFilesystemAdapter_ListDirectory(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create some test files
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("2"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(tempDir, "subdir"), 0755))

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	listing, err := adapter.ListDirectory(ctx, tempDir)
	assert.NoError(t, err)
	assert.NotNil(t, listing)
	assert.Equal(t, tempDir, listing.Path)
	assert.Equal(t, 3, listing.Count)

	// Check for specific files
	hasFile1 := false
	hasSubdir := false
	for _, f := range listing.Files {
		if f.Name == "file1.txt" {
			hasFile1 = true
		}
		if f.Name == "subdir" && f.IsDir {
			hasSubdir = true
		}
	}
	assert.True(t, hasFile1)
	assert.True(t, hasSubdir)
}

func TestFilesystemAdapter_CreateDirectory(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	config := FilesystemAdapterConfig{
		AllowedPaths:   []string{tempDir},
		AllowCreateDir: true,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	newDir := filepath.Join(tempDir, "new", "nested", "dir")
	err = adapter.CreateDirectory(ctx, newDir)
	assert.NoError(t, err)

	info, err := os.Stat(newDir)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestFilesystemAdapter_DeleteDirectory(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create directory with content
	dirToDelete := filepath.Join(tempDir, "to_delete")
	require.NoError(t, os.Mkdir(dirToDelete, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dirToDelete, "file.txt"), []byte("content"), 0644))

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		AllowDelete:  true,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	// Non-recursive should fail on non-empty directory
	err = adapter.DeleteDirectory(ctx, dirToDelete, false)
	assert.Error(t, err)

	// Recursive should succeed
	err = adapter.DeleteDirectory(ctx, dirToDelete, true)
	assert.NoError(t, err)

	_, err = os.Stat(dirToDelete)
	assert.True(t, os.IsNotExist(err))
}

func TestFilesystemAdapter_GetFileInfo(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	info, err := adapter.GetFileInfo(ctx, testFile)
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "test.txt", info.Name)
	assert.Equal(t, testFile, info.Path)
	assert.Equal(t, int64(7), info.Size)
	assert.False(t, info.IsDir)
}

func TestFilesystemAdapter_CopyFile(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	srcFile := filepath.Join(tempDir, "source.txt")
	dstFile := filepath.Join(tempDir, "dest.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("source content"), 0644))

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
		AllowWrite:   true,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	err = adapter.CopyFile(ctx, srcFile, dstFile)
	assert.NoError(t, err)

	// Verify copy
	content, err := os.ReadFile(dstFile)
	assert.NoError(t, err)
	assert.Equal(t, "source content", string(content))
}

func TestFilesystemAdapter_MoveFile(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	srcFile := filepath.Join(tempDir, "source.txt")
	dstFile := filepath.Join(tempDir, "dest.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("move content"), 0644))

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		AllowWrite:   true,
		AllowDelete:  true,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	err = adapter.MoveFile(ctx, srcFile, dstFile)
	assert.NoError(t, err)

	// Source should not exist
	_, err = os.Stat(srcFile)
	assert.True(t, os.IsNotExist(err))

	// Dest should have content
	content, err := os.ReadFile(dstFile)
	assert.NoError(t, err)
	assert.Equal(t, "move content", string(content))
}

func TestFilesystemAdapter_SearchFiles(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test files
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("2"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file.go"), []byte("3"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(tempDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "subdir", "file3.txt"), []byte("4"), 0644))

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	results, err := adapter.SearchFiles(ctx, tempDir, "*.txt", 100)
	assert.NoError(t, err)
	assert.Len(t, results, 3) // file1.txt, file2.txt, file3.txt
}

func TestFilesystemAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	adapter := NewFilesystemAdapter(DefaultFilesystemAdapterConfig())
	tools := adapter.GetMCPTools()

	assert.Len(t, tools, 11)

	// Check specific tools exist
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["filesystem_read_file"])
	assert.True(t, toolNames["filesystem_write_file"])
	assert.True(t, toolNames["filesystem_list_directory"])
	assert.True(t, toolNames["filesystem_search"])
}

func TestFilesystemAdapter_ExecuteTool(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("tool content"), 0644))

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	// Test read_file tool
	args, _ := json.Marshal(map[string]string{"path": testFile})
	result, err := adapter.ExecuteTool(ctx, "filesystem_read_file", args)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	fileContent, ok := result.(*FileContent)
	assert.True(t, ok)
	assert.Equal(t, "tool content", fileContent.Content)
}

func TestFilesystemAdapter_ExecuteTool_Unknown(t *testing.T) {
	t.Parallel()
	adapter := NewFilesystemAdapter(DefaultFilesystemAdapterConfig())
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	_, err := adapter.ExecuteTool(ctx, "unknown_tool", json.RawMessage(`{}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestFilesystemAdapter_Close(t *testing.T) {
	t.Parallel()
	config := FilesystemAdapterConfig{
		AllowedPaths: []string{os.TempDir()},
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))
	assert.True(t, adapter.initialized)

	err := adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.initialized)
}

func TestFilesystemAdapter_DeniedPaths(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "fs_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a denied directory
	deniedDir := filepath.Join(tempDir, "secret")
	require.NoError(t, os.Mkdir(deniedDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(deniedDir, "secret.txt"), []byte("secret"), 0644))

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		DeniedPaths:  []string{"/secret"},
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	_, err = adapter.ReadFile(ctx, filepath.Join(deniedDir, "secret.txt"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}
