package chunker

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSimpleChunker(t *testing.T) {
	t.Parallel()

	c := NewSimpleChunker(50, 10)
	require.NotNil(t, c)
	assert.Equal(t, 50, c.chunkSize)
	assert.Equal(t, 10, c.chunkOverlap)
}

func TestSimpleChunker_Chunk_Basic(t *testing.T) {
	t.Parallel()

	c := NewSimpleChunker(3, 1)

	// 5 lines of content
	content := "line1\nline2\nline3\nline4\nline5"
	chunks, err := c.Chunk(content, "go")

	require.NoError(t, err)
	require.NotEmpty(t, chunks)

	// First chunk: lines 0-2 (line1, line2, line3)
	assert.Equal(t, "line1\nline2\nline3", chunks[0].Content)
	assert.Equal(t, 1, chunks[0].StartLine)
	assert.Equal(t, 3, chunks[0].EndLine)
	assert.Equal(t, "go", chunks[0].Language)
	assert.Equal(t, ChunkTypeGeneral, chunks[0].Type)
}

func TestSimpleChunker_Chunk_SingleLine(t *testing.T) {
	t.Parallel()

	c := NewSimpleChunker(10, 2)
	chunks, err := c.Chunk("single line", "python")

	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, "single line", chunks[0].Content)
	assert.Equal(t, "python", chunks[0].Language)
	assert.Equal(t, 1, chunks[0].StartLine)
	assert.Equal(t, 1, chunks[0].EndLine)
}

func TestSimpleChunker_Chunk_EmptyContent(t *testing.T) {
	t.Parallel()

	c := NewSimpleChunker(10, 2)
	chunks, err := c.Chunk("", "go")

	require.NoError(t, err)
	// Empty string splits to [""] which is 1 element
	require.Len(t, chunks, 1)
	assert.Equal(t, "", chunks[0].Content)
}

func TestSimpleChunker_Chunk_ExactChunkSize(t *testing.T) {
	t.Parallel()

	c := NewSimpleChunker(3, 0)
	content := "a\nb\nc"
	chunks, err := c.Chunk(content, "go")

	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, "a\nb\nc", chunks[0].Content)
}

func TestSimpleChunker_Chunk_OverlapBehavior(t *testing.T) {
	t.Parallel()

	// 6 lines, chunk size 4, overlap 2 => step = 2
	// Chunk 1: lines 0-3
	// Chunk 2: lines 2-5
	c := NewSimpleChunker(4, 2)
	lines := make([]string, 6)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i)
	}
	content := strings.Join(lines, "\n")

	chunks, err := c.Chunk(content, "go")
	require.NoError(t, err)
	require.True(t, len(chunks) >= 2, "expected at least 2 chunks with overlap")

	// Verify overlap: chunks share some lines
	assert.Contains(t, chunks[0].Content, "line2")
	assert.Contains(t, chunks[1].Content, "line2")
}

func TestSimpleChunker_Chunk_IDFormat(t *testing.T) {
	t.Parallel()

	c := NewSimpleChunker(5, 0)
	content := "a\nb\nc\nd\ne\nf\ng"
	chunks, err := c.Chunk(content, "go")

	require.NoError(t, err)
	for _, chunk := range chunks {
		assert.Contains(t, chunk.ID, "chunk_")
	}
}

func TestSimpleChunker_Chunk_LargeContent(t *testing.T) {
	t.Parallel()

	c := NewSimpleChunker(10, 2)
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d content here", i)
	}
	content := strings.Join(lines, "\n")

	chunks, err := c.Chunk(content, "go")
	require.NoError(t, err)
	assert.True(t, len(chunks) > 1, "large content should produce multiple chunks")

	// All chunks should have the correct language
	for _, chunk := range chunks {
		assert.Equal(t, "go", chunk.Language)
		assert.Equal(t, ChunkTypeGeneral, chunk.Type)
	}
}

func TestNewLanguageBasedChunker(t *testing.T) {
	t.Parallel()

	c := NewLanguageBasedChunker(100)
	require.NotNil(t, c)
	assert.Equal(t, 100, c.maxChunkSize)
}

func TestLanguageBasedChunker_Chunk_SupportedLanguages(t *testing.T) {
	t.Parallel()

	languages := []string{"go", "python", "javascript", "typescript", "rust"}
	content := "line1\nline2\nline3"

	for _, lang := range languages {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			c := NewLanguageBasedChunker(100)
			chunks, err := c.Chunk(content, lang)
			require.NoError(t, err)
			require.NotEmpty(t, chunks)
			assert.Equal(t, lang, chunks[0].Language)
		})
	}
}

func TestLanguageBasedChunker_Chunk_UnsupportedLanguage(t *testing.T) {
	t.Parallel()

	c := NewLanguageBasedChunker(100)
	content := "line1\nline2\nline3"
	chunks, err := c.Chunk(content, "cobol")

	require.NoError(t, err)
	require.NotEmpty(t, chunks)
	// Falls back to simple chunker
	assert.Equal(t, "cobol", chunks[0].Language)
}

func TestLanguageBasedChunker_Chunk_EmptyLanguage(t *testing.T) {
	t.Parallel()

	c := NewLanguageBasedChunker(100)
	chunks, err := c.Chunk("content", "")

	require.NoError(t, err)
	require.NotEmpty(t, chunks)
}

func TestChunkFile(t *testing.T) {
	t.Parallel()

	chunkerInst := NewSimpleChunker(5, 0)
	content := "func main() {\n\tfmt.Println(\"hello\")\n}"

	chunks, err := ChunkFile("main.go", content, "go", chunkerInst)
	require.NoError(t, err)
	require.NotEmpty(t, chunks)

	// Check that IDs include file path
	for _, chunk := range chunks {
		assert.Contains(t, chunk.ID, "main.go:")
		assert.Equal(t, "main.go", chunk.FilePath)
	}
}

func TestChunkFile_MultipleChunks(t *testing.T) {
	t.Parallel()

	chunkerInst := NewSimpleChunker(2, 0)
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	content := strings.Join(lines, "\n")

	chunks, err := ChunkFile("big.go", content, "go", chunkerInst)
	require.NoError(t, err)
	assert.True(t, len(chunks) > 1)

	for _, chunk := range chunks {
		assert.Equal(t, "big.go", chunk.FilePath)
		assert.Contains(t, chunk.ID, "big.go:")
	}
}

func TestChunkFile_EmptyContent(t *testing.T) {
	t.Parallel()

	chunkerInst := NewSimpleChunker(10, 2)
	chunks, err := ChunkFile("empty.go", "", "go", chunkerInst)

	require.NoError(t, err)
	// Empty content still produces at least one chunk from the simple chunker
	require.NotEmpty(t, chunks)
	assert.Equal(t, "empty.go", chunks[0].FilePath)
}

func TestChunkFile_IDContainsLineNumbers(t *testing.T) {
	t.Parallel()

	chunkerInst := NewSimpleChunker(3, 0)
	content := "a\nb\nc\nd\ne\nf"

	chunks, err := ChunkFile("test.py", content, "python", chunkerInst)
	require.NoError(t, err)
	require.NotEmpty(t, chunks)

	// IDs should contain file:startLine-endLine format
	for _, chunk := range chunks {
		assert.Regexp(t, `^test\.py:\d+-\d+$`, chunk.ID)
	}
}

// Verify type aliases match types package
func TestChunkerTypeAliases(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ChunkType("function"), ChunkTypeFunction)
	assert.Equal(t, ChunkType("class"), ChunkTypeClass)
	assert.Equal(t, ChunkType("interface"), ChunkTypeInterface)
	assert.Equal(t, ChunkType("method"), ChunkTypeMethod)
	assert.Equal(t, ChunkType("comment"), ChunkTypeComment)
	assert.Equal(t, ChunkType("import"), ChunkTypeImport)
	assert.Equal(t, ChunkType("general"), ChunkTypeGeneral)
}
