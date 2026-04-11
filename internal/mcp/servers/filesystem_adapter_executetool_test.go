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

// ============================================================================
// ExecuteTool Comprehensive Tests
// ============================================================================

func TestFilesystemAdapter_ExecuteTool_AllTools(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create test structure
	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))
	testDir := filepath.Join(tempDir, "testdir")
	require.NoError(t, os.MkdirAll(testDir, 0755))
	subFile := filepath.Join(testDir, "subfile.txt")
	require.NoError(t, os.WriteFile(subFile, []byte("sub content"), 0644))

	config := FilesystemAdapterConfig{
		AllowedPaths:   []string{tempDir},
		MaxFileSize:    1024 * 1024,
		AllowWrite:     true,
		AllowDelete:    true,
		AllowCreateDir: true,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	t.Run("filesystem_read_file via ExecuteTool", func(t *testing.T) {
		t.Parallel()
		args, _ := json.Marshal(map[string]string{"path": testFile})
		result, err := adapter.ExecuteTool(ctx, "filesystem_read_file", args)

		assert.NoError(t, err)
		assert.NotNil(t, result)

		content, ok := result.(*FileContent)
		assert.True(t, ok)
		assert.Equal(t, "test content", content.Content)
		assert.Equal(t, testFile, content.Path)
	})

	t.Run("filesystem_write_file via ExecuteTool", func(t *testing.T) {
		t.Parallel()
		newFile := filepath.Join(tempDir, "new_write.txt")
		args, _ := json.Marshal(map[string]string{
			"path":    newFile,
			"content": "written via executetool",
		})

		result, err := adapter.ExecuteTool(ctx, "filesystem_write_file", args)
		assert.NoError(t, err)
		assert.Nil(t, result) // Write returns nil on success

		// Verify file was written
		content, err := os.ReadFile(newFile)
		assert.NoError(t, err)
		assert.Equal(t, "written via executetool", string(content))
	})

	t.Run("filesystem_append_file via ExecuteTool", func(t *testing.T) {
		t.Parallel()
		appendFile := filepath.Join(tempDir, "append_test.txt")
		require.NoError(t, os.WriteFile(appendFile, []byte("initial"), 0644))

		args, _ := json.Marshal(map[string]string{
			"path":    appendFile,
			"content": " appended",
		})

		result, err := adapter.ExecuteTool(ctx, "filesystem_append_file", args)
		assert.NoError(t, err)
		assert.Nil(t, result)

		content, err := os.ReadFile(appendFile)
		assert.NoError(t, err)
		assert.Equal(t, "initial appended", string(content))
	})

	t.Run("filesystem_delete_file via ExecuteTool", func(t *testing.T) {
		t.Parallel()
		deleteFile := filepath.Join(tempDir, "to_delete.txt")
		require.NoError(t, os.WriteFile(deleteFile, []byte("delete me"), 0644))

		args, _ := json.Marshal(map[string]string{"path": deleteFile})

		result, err := adapter.ExecuteTool(ctx, "filesystem_delete_file", args)
		assert.NoError(t, err)
		assert.Nil(t, result)

		_, err = os.Stat(deleteFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("filesystem_list_directory via ExecuteTool", func(t *testing.T) {
		t.Parallel()
		args, _ := json.Marshal(map[string]string{"path": tempDir})

		result, err := adapter.ExecuteTool(ctx, "filesystem_list_directory", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)

		listing, ok := result.(*DirectoryListing)
		assert.True(t, ok)
		assert.Equal(t, tempDir, listing.Path)
		assert.Greater(t, listing.Count, 0)
	})

	t.Run("filesystem_create_directory via ExecuteTool", func(t *testing.T) {
		t.Parallel()
		newDir := filepath.Join(tempDir, "created_dir", "nested")
		args, _ := json.Marshal(map[string]string{"path": newDir})

		result, err := adapter.ExecuteTool(ctx, "filesystem_create_directory", args)
		assert.NoError(t, err)
		assert.Nil(t, result)

		info, err := os.Stat(newDir)
		assert.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("filesystem_delete_directory via ExecuteTool", func(t *testing.T) {
		t.Parallel()
		dirToDelete := filepath.Join(tempDir, "dir_to_delete")
		require.NoError(t, os.MkdirAll(dirToDelete, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dirToDelete, "file.txt"), []byte("content"), 0644))

		args, _ := json.Marshal(map[string]interface{}{
			"path":      dirToDelete,
			"recursive": true,
		})

		result, err := adapter.ExecuteTool(ctx, "filesystem_delete_directory", args)
		assert.NoError(t, err)
		assert.Nil(t, result)

		_, err = os.Stat(dirToDelete)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("filesystem_delete_directory non-recursive via ExecuteTool", func(t *testing.T) {
		t.Parallel()
		emptyDir := filepath.Join(tempDir, "empty_dir")
		require.NoError(t, os.MkdirAll(emptyDir, 0755))

		args, _ := json.Marshal(map[string]interface{}{
			"path":      emptyDir,
			"recursive": false,
		})

		result, err := adapter.ExecuteTool(ctx, "filesystem_delete_directory", args)
		assert.NoError(t, err)
		assert.Nil(t, result)

		_, err = os.Stat(emptyDir)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("filesystem_get_info via ExecuteTool", func(t *testing.T) {
		t.Parallel()
		args, _ := json.Marshal(map[string]string{"path": testFile})

		result, err := adapter.ExecuteTool(ctx, "filesystem_get_info", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)

		info, ok := result.(*FileInfo)
		assert.True(t, ok)
		assert.Equal(t, "test.txt", info.Name)
		assert.False(t, info.IsDir)
	})

	t.Run("filesystem_copy_file via ExecuteTool", func(t *testing.T) {
		t.Parallel()
		srcFile := filepath.Join(tempDir, "copy_src.txt")
		dstFile := filepath.Join(tempDir, "copy_dst.txt")
		require.NoError(t, os.WriteFile(srcFile, []byte("copy content"), 0644))

		args, _ := json.Marshal(map[string]string{
			"source":      srcFile,
			"destination": dstFile,
		})

		result, err := adapter.ExecuteTool(ctx, "filesystem_copy_file", args)
		assert.NoError(t, err)
		assert.Nil(t, result)

		content, err := os.ReadFile(dstFile)
		assert.NoError(t, err)
		assert.Equal(t, "copy content", string(content))

		// Verify source still exists
		_, err = os.Stat(srcFile)
		assert.NoError(t, err)
	})

	t.Run("filesystem_move_file via ExecuteTool", func(t *testing.T) {
		t.Parallel()
		srcFile := filepath.Join(tempDir, "move_src.txt")
		dstFile := filepath.Join(tempDir, "move_dst.txt")
		require.NoError(t, os.WriteFile(srcFile, []byte("move content"), 0644))

		args, _ := json.Marshal(map[string]string{
			"source":      srcFile,
			"destination": dstFile,
		})

		result, err := adapter.ExecuteTool(ctx, "filesystem_move_file", args)
		assert.NoError(t, err)
		assert.Nil(t, result)

		content, err := os.ReadFile(dstFile)
		assert.NoError(t, err)
		assert.Equal(t, "move content", string(content))

		// Verify source no longer exists
		_, err = os.Stat(srcFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("filesystem_search via ExecuteTool", func(t *testing.T) {
		t.Parallel()
		// Create some searchable files
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "search1.log"), []byte("log1"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "search2.log"), []byte("log2"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "search3.log"), []byte("log3"), 0644))

		args, _ := json.Marshal(map[string]interface{}{
			"root_path":   tempDir,
			"pattern":     "*.log",
			"max_results": 10,
		})

		result, err := adapter.ExecuteTool(ctx, "filesystem_search", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)

		results, ok := result.([]FileInfo)
		assert.True(t, ok)
		assert.GreaterOrEqual(t, len(results), 3)
	})

	t.Run("filesystem_search with default max_results", func(t *testing.T) {
		t.Parallel()
		args, _ := json.Marshal(map[string]interface{}{
			"root_path": tempDir,
			"pattern":   "*",
			// No max_results - should default to 100
		})

		result, err := adapter.ExecuteTool(ctx, "filesystem_search", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)

		results, ok := result.([]FileInfo)
		assert.True(t, ok)
		assert.NotEmpty(t, results)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		t.Parallel()
		args, _ := json.Marshal(map[string]string{})

		_, err := adapter.ExecuteTool(ctx, "filesystem_nonexistent", args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})

	t.Run("invalid JSON args returns error", func(t *testing.T) {
		t.Parallel()
		_, err := adapter.ExecuteTool(ctx, "filesystem_read_file", json.RawMessage(`not json`))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parse arguments")
	})
}

// ============================================================================
// ExecuteTool Error Handling Tests
// ============================================================================

func TestFilesystemAdapter_ExecuteTool_ErrorCases(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024,
		AllowWrite:   false, // Disabled
		AllowDelete:  false, // Disabled
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	t.Run("read_file nonexistent path", func(t *testing.T) {
		t.Parallel()
		args, _ := json.Marshal(map[string]string{"path": filepath.Join(tempDir, "nonexistent.txt")})
		_, err := adapter.ExecuteTool(ctx, "filesystem_read_file", args)
		assert.Error(t, err)
	})

	t.Run("write_file when writes disabled", func(t *testing.T) {
		t.Parallel()
		args, _ := json.Marshal(map[string]string{
			"path":    filepath.Join(tempDir, "test.txt"),
			"content": "test",
		})
		_, err := adapter.ExecuteTool(ctx, "filesystem_write_file", args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed")
	})

	t.Run("delete_file when deletes disabled", func(t *testing.T) {
		t.Parallel()
		testFile := filepath.Join(tempDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

		args, _ := json.Marshal(map[string]string{"path": testFile})
		_, err := adapter.ExecuteTool(ctx, "filesystem_delete_file", args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed")
	})

	t.Run("list_directory on file", func(t *testing.T) {
		t.Parallel()
		testFile := filepath.Join(tempDir, "testfile.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

		args, _ := json.Marshal(map[string]string{"path": testFile})
		_, err := adapter.ExecuteTool(ctx, "filesystem_list_directory", args)
		assert.Error(t, err)
	})

	t.Run("get_info nonexistent path", func(t *testing.T) {
		t.Parallel()
		args, _ := json.Marshal(map[string]string{"path": filepath.Join(tempDir, "nonexistent.txt")})
		_, err := adapter.ExecuteTool(ctx, "filesystem_get_info", args)
		assert.Error(t, err)
	})

	t.Run("copy_file when writes disabled", func(t *testing.T) {
		t.Parallel()
		srcFile := filepath.Join(tempDir, "src.txt")
		require.NoError(t, os.WriteFile(srcFile, []byte("test"), 0644))

		args, _ := json.Marshal(map[string]string{
			"source":      srcFile,
			"destination": filepath.Join(tempDir, "dst.txt"),
		})
		_, err := adapter.ExecuteTool(ctx, "filesystem_copy_file", args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed")
	})

	t.Run("search_files in nonexistent directory", func(t *testing.T) {
		t.Parallel()
		args, _ := json.Marshal(map[string]interface{}{
			"root_path": filepath.Join(tempDir, "nonexistent"),
			"pattern":   "*",
		})
		result, err := adapter.ExecuteTool(ctx, "filesystem_search", args)
		// filepath.Walk on nonexistent returns error
		// However, SearchFiles skips inaccessible paths, so it may just return empty
		if err == nil {
			// If no error, result should be empty array
			results, ok := result.([]FileInfo)
			assert.True(t, ok)
			assert.Empty(t, results)
		}
	})
}

// ============================================================================
// ExecuteTool Permission Tests
// ============================================================================

func TestFilesystemAdapter_ExecuteTool_Permissions(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	t.Run("CreateDirectory when not allowed", func(t *testing.T) {
		t.Parallel()
		config := FilesystemAdapterConfig{
			AllowedPaths:   []string{tempDir},
			AllowCreateDir: false,
		}
		adapter := NewFilesystemAdapter(config)
		ctx := context.Background()
		require.NoError(t, adapter.Initialize(ctx))

		args, _ := json.Marshal(map[string]string{"path": filepath.Join(tempDir, "newdir")})
		_, err := adapter.ExecuteTool(ctx, "filesystem_create_directory", args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed")
	})

	t.Run("DeleteDirectory when not allowed", func(t *testing.T) {
		t.Parallel()
		dirToDelete := filepath.Join(tempDir, "dir_to_delete")
		require.NoError(t, os.MkdirAll(dirToDelete, 0755))

		config := FilesystemAdapterConfig{
			AllowedPaths: []string{tempDir},
			AllowDelete:  false,
		}
		adapter := NewFilesystemAdapter(config)
		ctx := context.Background()
		require.NoError(t, adapter.Initialize(ctx))

		args, _ := json.Marshal(map[string]interface{}{
			"path":      dirToDelete,
			"recursive": false,
		})
		_, err := adapter.ExecuteTool(ctx, "filesystem_delete_directory", args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed")
	})

	t.Run("MoveFile requires both write and delete", func(t *testing.T) {
		t.Parallel()
		srcFile := filepath.Join(tempDir, "src.txt")
		require.NoError(t, os.WriteFile(srcFile, []byte("test"), 0644))

		config := FilesystemAdapterConfig{
			AllowedPaths: []string{tempDir},
			AllowWrite:   true,
			AllowDelete:  false, // Missing delete permission
		}
		adapter := NewFilesystemAdapter(config)
		ctx := context.Background()
		require.NoError(t, adapter.Initialize(ctx))

		args, _ := json.Marshal(map[string]string{
			"source":      srcFile,
			"destination": filepath.Join(tempDir, "dst.txt"),
		})
		_, err := adapter.ExecuteTool(ctx, "filesystem_move_file", args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required for move")
	})
}

// ============================================================================
// ExecuteTool Path Validation Tests
// ============================================================================

func TestFilesystemAdapter_ExecuteTool_PathValidation(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		DeniedPaths:  []string{"/secret"},
		MaxFileSize:  1024,
		AllowWrite:   true,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	t.Run("read_file outside allowed paths", func(t *testing.T) {
		t.Parallel()
		args, _ := json.Marshal(map[string]string{"path": "/etc/passwd"})
		_, err := adapter.ExecuteTool(ctx, "filesystem_read_file", args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "outside allowed")
	})

	t.Run("write_file outside allowed paths", func(t *testing.T) {
		t.Parallel()
		args, _ := json.Marshal(map[string]string{
			"path":    "/etc/test.txt",
			"content": "test",
		})
		_, err := adapter.ExecuteTool(ctx, "filesystem_write_file", args)
		assert.Error(t, err)
	})

	t.Run("copy_file destination outside allowed paths", func(t *testing.T) {
		t.Parallel()
		srcFile := filepath.Join(tempDir, "src.txt")
		require.NoError(t, os.WriteFile(srcFile, []byte("test"), 0644))

		args, _ := json.Marshal(map[string]string{
			"source":      srcFile,
			"destination": "/tmp/outside.txt",
		})
		_, err := adapter.ExecuteTool(ctx, "filesystem_copy_file", args)
		assert.Error(t, err)
	})
}

// ============================================================================
// ExecuteTool Edge Cases Tests
// ============================================================================

func TestFilesystemAdapter_ExecuteTool_EdgeCases(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	config := FilesystemAdapterConfig{
		AllowedPaths:   []string{tempDir},
		MaxFileSize:    1024 * 1024,
		AllowWrite:     true,
		AllowDelete:    true,
		AllowCreateDir: true,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(t, adapter.Initialize(ctx))

	t.Run("read_file empty path", func(t *testing.T) {
		t.Parallel()
		args, _ := json.Marshal(map[string]string{"path": ""})
		_, err := adapter.ExecuteTool(ctx, "filesystem_read_file", args)
		assert.Error(t, err)
	})

	t.Run("write_file empty content", func(t *testing.T) {
		t.Parallel()
		testFile := filepath.Join(tempDir, "empty.txt")
		args, _ := json.Marshal(map[string]string{
			"path":    testFile,
			"content": "",
		})

		result, err := adapter.ExecuteTool(ctx, "filesystem_write_file", args)
		assert.NoError(t, err)
		assert.Nil(t, result)

		content, err := os.ReadFile(testFile)
		assert.NoError(t, err)
		assert.Empty(t, content)
	})

	t.Run("search_files empty pattern", func(t *testing.T) {
		t.Parallel()
		args, _ := json.Marshal(map[string]interface{}{
			"root_path": tempDir,
			"pattern":   "",
		})

		result, err := adapter.ExecuteTool(ctx, "filesystem_search", args)
		// Empty pattern matches nothing according to filepath.Match
		// Verify no error and result is empty slice (may be nil or []FileInfo{})
		assert.NoError(t, err)
		// Result could be nil or empty slice - both are valid for empty pattern
		if result != nil {
			results, ok := result.([]FileInfo)
			assert.True(t, ok)
			assert.Empty(t, results)
		}
	})

	t.Run("delete_file that is a directory", func(t *testing.T) {
		t.Parallel()
		dirPath := filepath.Join(tempDir, "a_directory")
		require.NoError(t, os.MkdirAll(dirPath, 0755))

		args, _ := json.Marshal(map[string]string{"path": dirPath})
		_, err := adapter.ExecuteTool(ctx, "filesystem_delete_file", args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "directory")
	})

	t.Run("delete_directory that is a file", func(t *testing.T) {
		t.Parallel()
		testFile := filepath.Join(tempDir, "not_a_dir.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

		args, _ := json.Marshal(map[string]interface{}{
			"path":      testFile,
			"recursive": false,
		})
		_, err := adapter.ExecuteTool(ctx, "filesystem_delete_directory", args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a directory")
	})

	t.Run("copy_file source is directory", func(t *testing.T) {
		t.Parallel()
		dirPath := filepath.Join(tempDir, "src_dir")
		require.NoError(t, os.MkdirAll(dirPath, 0755))

		args, _ := json.Marshal(map[string]string{
			"source":      dirPath,
			"destination": filepath.Join(tempDir, "dst.txt"),
		})
		_, err := adapter.ExecuteTool(ctx, "filesystem_copy_file", args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "directory")
	})
}

// ============================================================================
// ExecuteTool Not Initialized Tests
// ============================================================================

func TestFilesystemAdapter_ExecuteTool_NotInitialized(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	config := FilesystemAdapterConfig{
		AllowedPaths:   []string{tempDir},
		AllowWrite:     true,
		AllowDelete:    true,
		AllowCreateDir: true,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	// Note: NOT initializing the adapter

	tools := []string{
		"filesystem_read_file",
		"filesystem_write_file",
		"filesystem_append_file",
		"filesystem_delete_file",
		"filesystem_list_directory",
		"filesystem_create_directory",
		"filesystem_delete_directory",
		"filesystem_get_info",
		"filesystem_copy_file",
		"filesystem_move_file",
		"filesystem_search",
	}

	for _, tool := range tools {
		t.Run(tool+" not initialized", func(t *testing.T) {
			t.Parallel()
			args, _ := json.Marshal(map[string]interface{}{
				"path":        tempDir,
				"content":     "test",
				"source":      tempDir,
				"destination": tempDir,
				"root_path":   tempDir,
				"pattern":     "*",
			})
			_, err := adapter.ExecuteTool(ctx, tool, args)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not initialized")
		})
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkFilesystemAdapter_ExecuteTool_ReadFile(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "fs_bench_test")
	require.NoError(b, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	testFile := filepath.Join(tempDir, "bench.txt")
	require.NoError(b, os.WriteFile(testFile, []byte("benchmark content"), 0644))

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(b, adapter.Initialize(ctx))

	args, _ := json.Marshal(map[string]string{"path": testFile})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.ExecuteTool(ctx, "filesystem_read_file", args)
	}
}

func BenchmarkFilesystemAdapter_ExecuteTool_ListDirectory(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "fs_bench_list_test")
	require.NoError(b, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create some files
	for i := 0; i < 100; i++ {
		require.NoError(b, os.WriteFile(filepath.Join(tempDir, "file_"+string(rune('a'+i%26))+".txt"), []byte("content"), 0644))
	}

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(b, adapter.Initialize(ctx))

	args, _ := json.Marshal(map[string]string{"path": tempDir})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.ExecuteTool(ctx, "filesystem_list_directory", args)
	}
}

func BenchmarkFilesystemAdapter_ExecuteTool_Search(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "fs_bench_search_test")
	require.NoError(b, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create nested structure
	for i := 0; i < 10; i++ {
		subDir := filepath.Join(tempDir, "dir_"+string(rune('a'+i)))
		require.NoError(b, os.MkdirAll(subDir, 0755))
		for j := 0; j < 10; j++ {
			require.NoError(b, os.WriteFile(filepath.Join(subDir, "file_"+string(rune('a'+j))+".txt"), []byte("content"), 0644))
		}
	}

	config := FilesystemAdapterConfig{
		AllowedPaths: []string{tempDir},
	}

	adapter := NewFilesystemAdapter(config)
	ctx := context.Background()
	require.NoError(b, adapter.Initialize(ctx))

	args, _ := json.Marshal(map[string]interface{}{
		"root_path":   tempDir,
		"pattern":     "*.txt",
		"max_results": 50,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.ExecuteTool(ctx, "filesystem_search", args)
	}
}
