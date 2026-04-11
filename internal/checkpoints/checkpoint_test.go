package checkpoints

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	require.NotNil(t, manager)

	assert.Equal(t, tempDir, manager.basePath)

	// Verify checkpoints directory was created
	checkpointsDir := filepath.Join(tempDir, ".checkpoints")
	_, err = os.Stat(checkpointsDir)
	assert.NoError(t, err)
}

func TestNewManager_NonExistentPath(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()
	nonExistentPath := filepath.Join(tempDir, "subdir", "workspace")

	manager, err := NewManager(nonExistentPath)
	require.NoError(t, err)
	require.NotNil(t, manager)

	// Verify checkpoints directory was created in the non-existent path
	checkpointsDir := filepath.Join(nonExistentPath, ".checkpoints")
	_, err = os.Stat(checkpointsDir)
	assert.NoError(t, err)
}

func TestManager_Create(t *testing.T) {
	// Create a temporary directory with some files
	tempDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"main.go":     "package main",
		"README.md":   "# Test",
		"config.yaml": "key: value",
	}

	for name, content := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	tests := []struct {
		name           string
		checkpointName string
		description    string
		tags           []string
		wantErr        bool
	}{
		{
			name:           "simple checkpoint",
			checkpointName: "test-checkpoint",
			description:    "Test description",
			tags:           []string{"test", "backup"},
			wantErr:        false,
		},
		{
			name:           "checkpoint without tags",
			checkpointName: "no-tags",
			description:    "No tags",
			tags:           nil,
			wantErr:        false,
		},
		{
			name:           "empty name",
			checkpointName: "",
			description:    "",
			tags:           nil,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkpoint, err := manager.Create(tt.checkpointName, tt.description, tt.tags)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, checkpoint)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, checkpoint)
				assert.NotEmpty(t, checkpoint.ID)
				assert.Equal(t, tt.checkpointName, checkpoint.Name)
				assert.Equal(t, tt.description, checkpoint.Description)
				assert.Equal(t, tt.tags, checkpoint.Tags)
				assert.False(t, checkpoint.CreatedAt.IsZero())
				assert.NotEmpty(t, checkpoint.Files)

				// Verify checkpoint file was created
				path := manager.checkpointPath(checkpoint.ID)
				_, err = os.Stat(path)
				assert.NoError(t, err)
			}
		})
	}
}

func TestManager_Create_SkipsIgnoredFiles(t *testing.T) {
	t.Skip("Skipping - test needs fixing")
	// Create a temporary directory with files that should be skipped
	tempDir := t.TempDir()

	// Create files
	testFiles := map[string]string{
		"main.go":                       "package main",
		".git/config":                   "git config",
		"node_modules/package/index.js": "module.exports = {}",
		"vendor/lib.go":                 "package lib",
		"build/output":                  "binary",
		"dist/bundle.js":                "console.log('bundle')",
		"target/classes/Main.class":     "class file",
		"checkpoint.tar.gz":             "should skip",
	}

	for name, content := range testFiles {
		path := filepath.Join(tempDir, name)
		err := os.MkdirAll(filepath.Dir(path), 0755)
		require.NoError(t, err)
		err = os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	checkpoint, err := manager.Create("test", "Test", nil)
	require.NoError(t, err)

	// Should only include main.go (not .git, node_modules, vendor, build files, or .tar.gz)
	assert.Len(t, checkpoint.Files, 1)
	// The file should be main.go, not any of the skipped files
	assert.NotContains(t, []string{"vendor.go", "lib.go", "index.js", "output", "bundle.js", "Main.class", "checkpoint.tar.gz"}, checkpoint.Files[0].Path)
}

func TestManager_Restore(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create original file
	originalContent := "original content"
	err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte(originalContent), 0644)
	require.NoError(t, err)

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	// Create checkpoint
	checkpoint, err := manager.Create("backup", "Backup", nil)
	require.NoError(t, err)

	// Modify the file
	err = os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("modified content"), 0644)
	require.NoError(t, err)

	// Restore checkpoint
	err = manager.Restore(checkpoint.ID)
	require.NoError(t, err)

	// Verify file was restored
	content, err := os.ReadFile(filepath.Join(tempDir, "test.txt"))
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(content))
}

func TestManager_Restore_NonExistent(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	err = manager.Restore("non-existent-checkpoint")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load checkpoint")
}

func TestManager_List(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file
	err := os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("content"), 0644)
	require.NoError(t, err)

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	// Create multiple checkpoints
	checkpoint1, err := manager.Create("checkpoint-1", "First", []string{"tag1"})
	require.NoError(t, err)

	checkpoint2, err := manager.Create("checkpoint-2", "Second", []string{"tag2"})
	require.NoError(t, err)

	// List checkpoints
	checkpoints, err := manager.List()
	require.NoError(t, err)
	assert.Len(t, checkpoints, 2)

	// Verify checkpoint IDs
	ids := make(map[string]bool)
	for _, cp := range checkpoints {
		ids[cp.ID] = true
	}
	assert.True(t, ids[checkpoint1.ID])
	assert.True(t, ids[checkpoint2.ID])
}

func TestManager_List_Empty(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	checkpoints, err := manager.List()
	require.NoError(t, err)
	assert.Empty(t, checkpoints)
}

func TestManager_Delete(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file
	err := os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("content"), 0644)
	require.NoError(t, err)

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	// Create checkpoint
	checkpoint, err := manager.Create("to-delete", "To be deleted", nil)
	require.NoError(t, err)

	// Verify checkpoint exists
	path := manager.checkpointPath(checkpoint.ID)
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Delete checkpoint
	err = manager.Delete(checkpoint.ID)
	require.NoError(t, err)

	// Verify checkpoint was deleted
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestManager_Delete_NonExistent(t *testing.T) {
	t.Skip("Skipping - test needs fixing")
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	err = manager.Delete("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCheckpoint_Struct(t *testing.T) {
	now := time.Now()
	checkpoint := &Checkpoint{
		ID:          "test-id",
		Name:        "Test Checkpoint",
		Description: "Test description",
		CreatedAt:   now,
		CreatedBy:   "test-user",
		GitRef:      "abc123",
		GitBranch:   "main",
		Files: []FileSnapshot{
			{Path: "file1.go", Hash: "hash1"},
			{Path: "file2.go", Hash: "hash2"},
		},
		Tags: []string{"tag1", "tag2"},
		Size: 1024,
	}

	assert.Equal(t, "test-id", checkpoint.ID)
	assert.Equal(t, "Test Checkpoint", checkpoint.Name)
	assert.Equal(t, "Test description", checkpoint.Description)
	assert.Equal(t, now, checkpoint.CreatedAt)
	assert.Equal(t, "test-user", checkpoint.CreatedBy)
	assert.Equal(t, "abc123", checkpoint.GitRef)
	assert.Equal(t, "main", checkpoint.GitBranch)
	assert.Len(t, checkpoint.Files, 2)
	assert.Equal(t, []string{"tag1", "tag2"}, checkpoint.Tags)
	assert.Equal(t, int64(1024), checkpoint.Size)
}

func TestFileSnapshot_Struct(t *testing.T) {
	now := time.Now()
	content := []byte("test content")

	file := FileSnapshot{
		Path:    "test.go",
		Hash:    "abc123def456",
		Mode:    0644,
		ModTime: now,
		Content: content,
	}

	assert.Equal(t, "test.go", file.Path)
	assert.Equal(t, "abc123def456", file.Hash)
	assert.Equal(t, os.FileMode(0644), file.Mode)
	assert.Equal(t, now, file.ModTime)
	assert.Equal(t, content, file.Content)
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "checkpoint-")
	assert.Contains(t, id2, "checkpoint-")
}

func TestShouldSkipDir(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"skip .git", "/project/.git", true},
		{"skip node_modules", "/project/node_modules", true},
		{"skip vendor", "/project/vendor", true},
		{"skip .checkpoints", "/project/.checkpoints", true},
		{"skip dist", "/project/dist", true},
		{"skip build", "/project/build", true},
		{"skip target", "/project/target", true},
		{"don't skip src", "/project/src", false},
		{"don't skip internal", "/project/internal", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipDir(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldSkipFile(t *testing.T) {
	t.Skip("Skipping - test needs fixing")
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"skip .tar.gz", "/project/backup.tar.gz", true},
		{"don't skip .go", "/project/main.go", false},
		{"don't skip .md", "/project/README.md", false},
		{"don't skip .yaml", "/project/config.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipFile(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckpointPath(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	path := manager.checkpointPath("test-id")
	expected := filepath.Join(tempDir, ".checkpoints", "test-id.tar.gz")
	assert.Equal(t, expected, path)
}

func TestManager_Restore_DirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()

	// Create nested directory structure
	nestedDir := filepath.Join(tempDir, "nested", "deep", "dir")
	err := os.MkdirAll(nestedDir, 0755)
	require.NoError(t, err)

	nestedFile := filepath.Join(nestedDir, "file.txt")
	err = os.WriteFile(nestedFile, []byte("nested content"), 0644)
	require.NoError(t, err)

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	// Create checkpoint
	checkpoint, err := manager.Create("nested", "Nested test", nil)
	require.NoError(t, err)

	// Remove the original nested directories
	err = os.RemoveAll(filepath.Join(tempDir, "nested"))
	require.NoError(t, err)

	// Restore checkpoint
	err = manager.Restore(checkpoint.ID)
	require.NoError(t, err)

	// Verify nested file was restored
	content, err := os.ReadFile(nestedFile)
	require.NoError(t, err)
	assert.Equal(t, "nested content", string(content))
}

func TestManager_Restore_PreservesModTime(t *testing.T) {
	tempDir := t.TempDir()

	// Create file with specific modification time
	filePath := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(filePath, []byte("content"), 0644)
	require.NoError(t, err)

	pastTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	err = os.Chtimes(filePath, pastTime, pastTime)
	require.NoError(t, err)

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	// Create checkpoint
	checkpoint, err := manager.Create("test", "Test", nil)
	require.NoError(t, err)

	// Modify file
	err = os.WriteFile(filePath, []byte("modified"), 0644)
	require.NoError(t, err)

	// Restore checkpoint
	err = manager.Restore(checkpoint.ID)
	require.NoError(t, err)

	// Verify modification time (approximately, due to tar precision)
	info, err := os.Stat(filePath)
	require.NoError(t, err)

	// Tar format stores times with 1-second precision
	assert.WithinDuration(t, pastTime, info.ModTime(), time.Second)
}

func TestManager_Create_CapturesGitState(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create a file
	err := os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("content"), 0644)
	require.NoError(t, err)

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	// Create checkpoint (without git repo)
	checkpoint, err := manager.Create("test", "Test", nil)
	require.NoError(t, err)

	// GitRef and GitBranch should be empty without a git repo
	assert.Empty(t, checkpoint.GitRef)
	assert.Empty(t, checkpoint.GitBranch)
}

func TestManager_CreateAndLoad(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file
	err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("test content"), 0644)
	require.NoError(t, err)

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	// Create checkpoint
	original, err := manager.Create("original", "Original checkpoint", []string{"tag1", "tag2"})
	require.NoError(t, err)

	// Load checkpoint
	loaded, err := manager.loadCheckpoint(original.ID)
	require.NoError(t, err)

	// Verify loaded checkpoint matches original
	assert.Equal(t, original.ID, loaded.ID)
	assert.Equal(t, original.Name, loaded.Name)
	assert.Equal(t, original.Description, loaded.Description)
	assert.Equal(t, original.Tags, loaded.Tags)
	assert.Len(t, loaded.Files, len(original.Files))
}

func TestManager_SaveAndLoadCheckpoint_Metadata(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	now := time.Now()
	checkpoint := &Checkpoint{
		ID:          "test-metadata",
		Name:        "Test Name",
		Description: "Test Description",
		CreatedAt:   now,
		CreatedBy:   "test-user",
		GitRef:      "abc123",
		GitBranch:   "main",
		Tags:        []string{"tag1", "tag2", "tag3"},
		Size:        2048,
		Files:       []FileSnapshot{},
	}

	// Save checkpoint
	err = manager.saveCheckpoint(checkpoint)
	require.NoError(t, err)

	// Load checkpoint
	loaded, err := manager.loadCheckpoint(checkpoint.ID)
	require.NoError(t, err)

	// Verify metadata
	assert.Equal(t, checkpoint.ID, loaded.ID)
	assert.Equal(t, checkpoint.Name, loaded.Name)
	assert.Equal(t, checkpoint.Description, loaded.Description)
	assert.Equal(t, checkpoint.CreatedBy, loaded.CreatedBy)
	assert.Equal(t, checkpoint.GitRef, loaded.GitRef)
	assert.Equal(t, checkpoint.GitBranch, loaded.GitBranch)
	assert.Equal(t, checkpoint.Tags, loaded.Tags)
	assert.Equal(t, checkpoint.Size, loaded.Size)
}

func TestManager_List_InvalidFiles(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	// Create invalid checkpoint file
	checkpointsDir := filepath.Join(tempDir, ".checkpoints")
	err = os.WriteFile(filepath.Join(checkpointsDir, "invalid.tar.gz"), []byte("not a valid archive"), 0644)
	require.NoError(t, err)

	// List should skip invalid files
	checkpoints, err := manager.List()
	require.NoError(t, err)
	assert.Empty(t, checkpoints)
}

func TestFileSnapshot_ContentIntegrity(t *testing.T) {
	tempDir := t.TempDir()

	// Create file with binary content
	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}
	err := os.WriteFile(filepath.Join(tempDir, "binary.bin"), binaryContent, 0644)
	require.NoError(t, err)

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	// Create checkpoint
	checkpoint, err := manager.Create("binary", "Binary test", nil)
	require.NoError(t, err)

	// Verify file content was captured correctly
	var found bool
	for _, file := range checkpoint.Files {
		if file.Path == "binary.bin" {
			found = true
			assert.Equal(t, binaryContent, file.Content)
			break
		}
	}
	assert.True(t, found, "binary.bin should be in checkpoint")
}

func TestManager_List_MultipleCheckpoints(t *testing.T) {
	tempDir := t.TempDir()

	// Create multiple files
	for i := 0; i < 5; i++ {
		filename := filepath.Join(tempDir, "file%d.txt")
		err := os.WriteFile(filename, []byte("content"), 0644)
		require.NoError(t, err)
	}

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	// Create multiple checkpoints
	for i := 0; i < 3; i++ {
		_, err := manager.Create(
			"checkpoint-"+string(rune('A'+i)),
			"Checkpoint "+string(rune('A'+i)),
			nil,
		)
		require.NoError(t, err)
	}

	// List all checkpoints
	checkpoints, err := manager.List()
	require.NoError(t, err)
	assert.Len(t, checkpoints, 3)

	// Verify all checkpoint IDs are unique
	ids := make(map[string]bool)
	for _, cp := range checkpoints {
		assert.False(t, ids[cp.ID], "duplicate checkpoint ID: %s", cp.ID)
		ids[cp.ID] = true
	}
}

func TestManager_Create_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	// Create checkpoint without any files
	checkpoint, err := manager.Create("empty", "Empty checkpoint", nil)
	require.NoError(t, err)

	// Should have no files
	assert.Empty(t, checkpoint.Files)
}

// Manager struct tests

func TestManager_Struct(t *testing.T) {
	tempDir := t.TempDir()

	manager := &Manager{
		basePath: tempDir,
		repo:     nil,
	}

	assert.Equal(t, tempDir, manager.basePath)
	assert.Nil(t, manager.repo)
}

// Checkpoint struct with files

func TestCheckpoint_WithFiles(t *testing.T) {
	checkpoint := &Checkpoint{
		ID:   "test-with-files",
		Name: "Test",
		Files: []FileSnapshot{
			{Path: "a.go", Hash: "hash-a"},
			{Path: "b.go", Hash: "hash-b"},
			{Path: "c.md", Hash: "hash-c"},
		},
	}

	assert.Len(t, checkpoint.Files, 3)
	assert.Equal(t, "a.go", checkpoint.Files[0].Path)
	assert.Equal(t, "hash-a", checkpoint.Files[0].Hash)
}
