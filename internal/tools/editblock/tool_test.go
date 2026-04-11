package editblock

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTool(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	tool := NewTool("/base/path", logger)

	require.NotNil(t, tool)
	assert.Equal(t, "/base/path", tool.basePath)
	assert.NotNil(t, tool.logger)
}

func TestTool_Name(t *testing.T) {
	t.Parallel()
	tool := NewTool("/tmp", logrus.New())
	assert.Equal(t, "EditBlock", tool.Name())
}

func TestTool_Description(t *testing.T) {
	t.Parallel()
	tool := NewTool("/tmp", logrus.New())
	desc := tool.Description()
	assert.Contains(t, desc, "search/replace")
	assert.Contains(t, desc, "Aider")
}

func TestTool_Schema(t *testing.T) {
	t.Parallel()
	tool := NewTool("/tmp", logrus.New())
	schema := tool.Schema()

	require.NotNil(t, schema)
	assert.Equal(t, "object", schema["type"])

	props, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, props, "file_path")
	assert.Contains(t, props, "search")
	assert.Contains(t, props, "replace")
	assert.Contains(t, props, "dry_run")

	required, ok := schema["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "file_path")
	assert.Contains(t, required, "search")
	assert.Contains(t, required, "replace")
}

func TestTool_Execute(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	tool := NewTool(tempDir, logrus.New())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path": "test.txt",
		"search":    "world",
		"replace":   "universe",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "test.txt", result.FilePath)
	assert.NotEmpty(t, result.Diff)

	// Verify file was modified
	content, _ := os.ReadFile(testFile)
	assert.Equal(t, "hello universe", string(content))
}

func TestTool_Execute_DryRun(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	tool := NewTool(tempDir, logrus.New())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path": "test.txt",
		"search":    "world",
		"replace":   "universe",
		"dry_run":   true,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.NotEmpty(t, result.Diff)

	// Verify file was NOT modified
	content, _ := os.ReadFile(testFile)
	assert.Equal(t, "hello world", string(content))
}

func TestTool_Execute_MissingFilePath(t *testing.T) {
	t.Parallel()
	tool := NewTool("/tmp", logrus.New())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"search":  "old",
		"replace": "new",
	})

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "file_path is required")
}

func TestTool_Execute_MissingSearch(t *testing.T) {
	t.Parallel()
	tool := NewTool("/tmp", logrus.New())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path": "test.txt",
		"replace":   "new",
	})

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "search is required")
}

func TestTool_Execute_MissingReplace(t *testing.T) {
	t.Parallel()
	tool := NewTool("/tmp", logrus.New())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path": "test.txt",
		"search":    "old",
	})

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "replace is required")
}

func TestTool_Execute_FileNotFound(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	tool := NewTool(tempDir, logrus.New())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path": "nonexistent.txt",
		"search":    "old",
		"replace":   "new",
	})

	require.NoError(t, err)
	assert.False(t, result.Success)
	// Error can be either "file not found" from Apply or file open error from Diff
	assert.True(t,
		strings.Contains(result.Error, "file not found") ||
			strings.Contains(result.Error, "no such file"),
		"Expected file not found error, got: %s", result.Error)
}

func TestTool_Execute_SearchNotFound(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))

	tool := NewTool(tempDir, logrus.New())
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path": "test.txt",
		"search":    "nonexistent",
		"replace":   "new",
	})

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "search pattern not found")
}

func TestTool_Apply(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	tool := NewTool(tempDir, logrus.New())
	ctx := context.Background()

	result, err := tool.Apply(ctx, "test.txt", "world", "universe")

	require.NoError(t, err)
	assert.True(t, result.Success)

	content, _ := os.ReadFile(testFile)
	assert.Equal(t, "hello universe", string(content))
}

func TestTool_Preview(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	tool := NewTool(tempDir, logrus.New())
	ctx := context.Background()

	result, err := tool.Preview(ctx, "test.txt", "world", "universe")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.NotEmpty(t, result.Diff)

	// File should not be modified
	content, _ := os.ReadFile(testFile)
	assert.Equal(t, "hello world", string(content))
}

func TestTool_ApplyFromText(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("old content here"), 0644))

	tool := NewTool(tempDir, logrus.New())
	ctx := context.Background()

	text := `<<<<<<< SEARCH
old content
=======
new content
>>>>>>> REPLACE`

	results, err := tool.ApplyFromText(ctx, "test.txt", text)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Success)

	content, _ := os.ReadFile(testFile)
	assert.Equal(t, "new content here", string(content))
}

func TestTool_ApplyFromText_NoBlocks(t *testing.T) {
	t.Parallel()
	tool := NewTool("/tmp", logrus.New())
	ctx := context.Background()

	_, err := tool.ApplyFromText(ctx, "test.txt", "no edit blocks here")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no edit blocks found")
}

func TestTool_BatchApply(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("a b c d"), 0644))

	tool := NewTool(tempDir, logrus.New())
	ctx := context.Background()

	operations := []map[string]string{
		{"file_path": "test.txt", "search": "a", "replace": "A"},
		{"file_path": "test.txt", "search": "c", "replace": "C"},
	}

	results, err := tool.BatchApply(ctx, operations)

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.True(t, results[0].Success)
	assert.True(t, results[1].Success)

	content, _ := os.ReadFile(testFile)
	assert.Equal(t, "A b C d", string(content))
}

func TestTool_FindSimilar(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	content := `line1
line2
line3
line4
line5`
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

	tool := NewTool(tempDir, logrus.New())

	matches, err := tool.FindSimilar("test.txt", "line2\nline3", 0.5)

	require.NoError(t, err)
	assert.NotEmpty(t, matches)
}

func TestCalculateSimilarity(t *testing.T) {
	t.Parallel()
	// Exact match
	assert.Equal(t, 1.0, calculateSimilarity(
		[]string{"line1", "line2"},
		[]string{"line1", "line2"},
	))

	// Partial match
	assert.Equal(t, 0.5, calculateSimilarity(
		[]string{"line1", "different"},
		[]string{"line1", "line2"},
	))

	// No match
	assert.Equal(t, 0.0, calculateSimilarity(
		[]string{"different1", "different2"},
		[]string{"line1", "line2"},
	))

	// Different lengths
	assert.Equal(t, 0.0, calculateSimilarity(
		[]string{"line1"},
		[]string{"line1", "line2"},
	))
}

func TestTool_Execute_InvalidDryRunType(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	tool := NewTool(tempDir, logrus.New())
	ctx := context.Background()

	// Pass dry_run as string instead of bool
	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path": "test.txt",
		"search":    "world",
		"replace":   "universe",
		"dry_run":   "invalid",
	})

	require.NoError(t, err)
	// Should treat invalid as false and apply the change
	assert.True(t, result.Success)

	// Verify file WAS modified (dry_run was ignored)
	content, _ := os.ReadFile(testFile)
	assert.Equal(t, "hello universe", string(content))
}

func TestTool_ConcurrentExecution(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("a b c d e f"), 0644))

	tool := NewTool(tempDir, logrus.New())

	// Run multiple edits concurrently
	edits := []struct {
		search  string
		replace string
	}{
		{"a", "A"},
		{"b", "B"},
		{"c", "C"},
	}

	done := make(chan *ToolResult, len(edits))

	for _, e := range edits {
		go func(search, replace string) {
			ctx := context.Background()
			result, _ := tool.Apply(ctx, "test.txt", search, replace)
			done <- result
		}(e.search, e.replace)
	}

	// Collect results
	for i := 0; i < len(edits); i++ {
		result := <-done
		// Some may fail due to concurrent modification, which is expected
		_ = result
	}
}
