package editblock

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditBlock_Apply(t *testing.T) {
	t.Parallel()
	// Create temp directory
	tempDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	originalContent := "line1\nline2\nline3\nline4\nline5"
	require.NoError(t, os.WriteFile(testFile, []byte(originalContent), 0644))

	block := EditBlock{
		FilePath: "test.txt",
		Search:   "line2\nline3",
		Replace:  "newLine2\nnewLine3",
	}

	result, err := block.Apply(tempDir)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "test.txt", result.FilePath)
	assert.Equal(t, 2, result.LineStart)
	assert.Equal(t, 3, result.LineEnd)
	assert.Equal(t, 1, result.ChangesMade)

	// Verify content
	newContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "line1\nnewLine2\nnewLine3\nline4\nline5", string(newContent))
}

func TestEditBlock_Apply_EmptyFilePath(t *testing.T) {
	t.Parallel()
	block := EditBlock{}
	result, err := block.Apply("/tmp")
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "file path is required")
}

func TestEditBlock_Apply_FileNotFound(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	block := EditBlock{
		FilePath: "nonexistent.txt",
		Search:   "test",
		Replace:  "replaced",
	}

	result, err := block.Apply(tempDir)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "file not found")
}

func TestEditBlock_Apply_PathEscape(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	block := EditBlock{
		FilePath: "../etc/passwd",
		Search:   "test",
		Replace:  "replaced",
	}

	result, err := block.Apply(tempDir)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "path escapes")
}

func TestEditBlock_Apply_SearchNotFound(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))

	block := EditBlock{
		FilePath: "test.txt",
		Search:   "nonexistent",
		Replace:  "replaced",
	}

	result, err := block.Apply(tempDir)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "search pattern not found")
}

func TestEditBlock_Apply_SingleLine(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	block := EditBlock{
		FilePath: "test.txt",
		Search:   "world",
		Replace:  "universe",
	}

	result, err := block.Apply(tempDir)
	require.NoError(t, err)
	assert.True(t, result.Success)

	content, _ := os.ReadFile(testFile)
	assert.Equal(t, "hello universe", string(content))
}

func TestEditBlock_Apply_FlexibleWhitespace(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	content := "  line1  \n  line2  "
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

	// Search without extra whitespace should still match
	block := EditBlock{
		FilePath: "test.txt",
		Search:   "line1\nline2",
		Replace:  "new1\nnew2",
	}

	result, err := block.Apply(tempDir)
	require.NoError(t, err)
	assert.True(t, result.Success)

	newContent, _ := os.ReadFile(testFile)
	assert.Contains(t, string(newContent), "new1")
	assert.Contains(t, string(newContent), "new2")
}

func TestApplyMultiple(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("a\nb\nc\nd"), 0644))

	blocks := []EditBlock{
		{FilePath: "test.txt", Search: "a", Replace: "A"},
		{FilePath: "test.txt", Search: "c", Replace: "C"},
	}

	results, err := ApplyMultiple(tempDir, blocks)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.True(t, results[0].Success)
	assert.True(t, results[1].Success)

	content, _ := os.ReadFile(testFile)
	assert.Equal(t, "A\nb\nC\nd", string(content))
}

func TestFormatEditBlock(t *testing.T) {
	t.Parallel()
	result := FormatEditBlock("test.go", "old code", "new code")
	assert.Contains(t, result, "<<<<<<< SEARCH")
	assert.Contains(t, result, "old code")
	assert.Contains(t, result, "=======")
	assert.Contains(t, result, "new code")
	assert.Contains(t, result, ">>>>>>> REPLACE")
}

func TestParseEditBlocks(t *testing.T) {
	t.Parallel()
	text := `Some context here
<<<<<<< SEARCH
old line 1
old line 2
=======
new line 1
new line 2
>>>>>>> REPLACE
More context here`

	blocks := ParseEditBlocks(text)
	require.Len(t, blocks, 1)
	assert.Equal(t, "old line 1\nold line 2", blocks[0].Search)
	assert.Equal(t, "new line 1\nnew line 2", blocks[0].Replace)
}

func TestParseEditBlocks_Multiple(t *testing.T) {
	t.Parallel()
	text := `<<<<<<< SEARCH
first search
=======
first replace
>>>>>>> REPLACE

<<<<<<< SEARCH
second search
=======
second replace
>>>>>>> REPLACE`

	blocks := ParseEditBlocks(text)
	require.Len(t, blocks, 2)
	assert.Equal(t, "first search", blocks[0].Search)
	assert.Equal(t, "second search", blocks[1].Search)
}

func TestParseEditBlocks_Empty(t *testing.T) {
	t.Parallel()
	blocks := ParseEditBlocks("no edit blocks here")
	assert.Len(t, blocks, 0)
}

func TestEditBlock_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		block   EditBlock
		wantErr bool
	}{
		{
			name:    "valid",
			block:   EditBlock{FilePath: "test.go", Search: "old", Replace: "new"},
			wantErr: false,
		},
		{
			name:    "missing path",
			block:   EditBlock{Search: "old", Replace: "new"},
			wantErr: true,
		},
		{
			name:    "both empty",
			block:   EditBlock{FilePath: "test.go"},
			wantErr: true,
		},
		{
			name:    "empty search ok",
			block:   EditBlock{FilePath: "test.go", Search: "", Replace: "new"},
			wantErr: false,
		},
		{
			name:    "empty replace ok",
			block:   EditBlock{FilePath: "test.go", Search: "old", Replace: ""},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.block.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEditBlock_Diff(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("line1\nline2\nline3"), 0644))

	block := EditBlock{
		FilePath: "test.txt",
		Search:   "line2",
		Replace:  "modified",
	}

	diff, err := block.Diff(tempDir)
	require.NoError(t, err)
	assert.Contains(t, diff, "--- test.txt")
	assert.Contains(t, diff, "+++ test.txt")
	assert.Contains(t, diff, "-line2")
	assert.Contains(t, diff, "+modified")
}

func TestEditBlock_Diff_NotFound(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))

	block := EditBlock{
		FilePath: "test.txt",
		Search:   "nonexistent",
		Replace:  "replace",
	}

	_, err := block.Diff(tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestReadFileLines(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("line1\nline2\nline3"), 0644))

	lines, err := ReadFileLines(testFile)
	require.NoError(t, err)
	assert.Equal(t, []string{"line1", "line2", "line3"}, lines)
}

func TestWriteFileLines(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	err := WriteFileLines(testFile, []string{"line1", "line2", "line3"})
	require.NoError(t, err)

	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "line1\nline2\nline3\n", string(content))
}

func TestNormalizeLineEndings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"line1\r\nline2", "line1\nline2"},
		{"line1\rline2", "line1\nline2"},
		{"line1\nline2", "line1\nline2"},
		{"mixed\r\n\r\n", "mixed\n\n"},
	}

	for _, tt := range tests {
		result := normalizeLineEndings(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"  test  ", "test"},
		{"\ttest\t", "test"},
		{"test", "test"},
		{"", ""},
	}

	for _, tt := range tests {
		result := normalizeWhitespace(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "short", truncate("short", 100))
	assert.Equal(t, "long str...", truncate("long string", 8))
	assert.Equal(t, "", truncate("", 10))
}

func TestMatchBlock(t *testing.T) {
	t.Parallel()
	lines := []string{"  line1  ", "line2", "  line3  "}

	// Should match with flexible whitespace
	assert.True(t, matchBlock(lines, []string{"line1", "line2", "line3"}))

	// Should not match different content
	assert.False(t, matchBlock(lines, []string{"different", "content"}))

	// Should not match if too few lines
	assert.False(t, matchBlock(lines[:1], []string{"line1", "line2"}))
}

func TestFindAndReplace(t *testing.T) {
	t.Parallel()
	content := "line1\nline2\nline3\nline4"

	newContent, start, end, found := findAndReplace(content, "line2\nline3", "replaced", 0)
	assert.True(t, found)
	assert.Equal(t, 2, start)
	assert.Equal(t, 3, end)
	assert.Contains(t, newContent, "replaced")
	assert.NotContains(t, newContent, "line2\nline3")
}

func TestFindAndReplace_WithHint(t *testing.T) {
	t.Parallel()
	content := "a\na\na\ntarget\nb\nb\nb"

	newContent, start, end, found := findAndReplace(content, "target", "REPLACED", 4)
	assert.True(t, found)
	assert.Equal(t, 4, start)
	assert.Equal(t, 4, end)
	assert.Contains(t, newContent, "REPLACED")
}

func TestFindAndReplace_NotFound(t *testing.T) {
	t.Parallel()
	content := "line1\nline2"

	_, _, _, found := findAndReplace(content, "nonexistent", "replace", 0)
	assert.False(t, found)
}
