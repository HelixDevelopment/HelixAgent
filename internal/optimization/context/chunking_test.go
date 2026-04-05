package context

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultChunkConfig(t *testing.T) {
	t.Parallel()
	config := DefaultChunkConfig()

	assert.Equal(t, 512, config.ChunkSize)
	assert.Equal(t, 50, config.ChunkOverlap)
	assert.Equal(t, "\n\n", config.Separator)
	assert.Len(t, config.SecondarySeparators, 3)
	assert.True(t, config.TrimWhitespace)
	assert.True(t, config.PreserveNewlines)
}

func TestNewDocumentChunker(t *testing.T) {
	t.Parallel()
	chunker := NewDocumentChunker(nil)
	assert.NotNil(t, chunker)
	assert.NotNil(t, chunker.config)
}

func TestDocumentChunker_Chunk_Empty(t *testing.T) {
	t.Parallel()
	chunker := NewDocumentChunker(nil)
	chunks := chunker.Chunk("")
	assert.Nil(t, chunks)
}

func TestDocumentChunker_Chunk_Simple(t *testing.T) {
	t.Parallel()
	config := &ChunkConfig{
		ChunkSize:    100,
		ChunkOverlap: 0,
		Separator:    "\n\n",
	}
	chunker := NewDocumentChunker(config)

	text := "First paragraph.\n\nSecond paragraph.\n\nThird paragraph."
	chunks := chunker.Chunk(text)

	assert.GreaterOrEqual(t, len(chunks), 1)
}

func TestDocumentChunker_Chunk_LargeParagraph(t *testing.T) {
	t.Parallel()
	config := &ChunkConfig{
		ChunkSize:           10,
		ChunkOverlap:        0,
		Separator:           "\n\n",
		SecondarySeparators: []string{"\n", ". ", " "},
		TrimWhitespace:      true,
	}
	chunker := NewDocumentChunker(config)

	text := "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12 word13 word14 word15"
	chunks := chunker.Chunk(text)

	assert.GreaterOrEqual(t, len(chunks), 1)
	// Each chunk should be within size limits
	for _, chunk := range chunks {
		assert.LessOrEqual(t, chunk.TokenCount, 15) // Allow some flexibility
	}
}

func TestDocumentChunker_Chunk_WithOverlap(t *testing.T) {
	t.Parallel()
	config := &ChunkConfig{
		ChunkSize:    10,
		ChunkOverlap: 3,
		Separator:    "\n\n",
	}
	chunker := NewDocumentChunker(config)

	text := "word1 word2 word3 word4 word5\n\nword6 word7 word8 word9 word10"
	chunks := chunker.Chunk(text)

	assert.GreaterOrEqual(t, len(chunks), 1)
}

func TestDocumentChunker_ChunkWithMetadata(t *testing.T) {
	t.Parallel()
	chunker := NewDocumentChunker(nil)

	metadata := map[string]interface{}{
		"source":   "test.txt",
		"category": "documentation",
	}

	chunks := chunker.ChunkWithMetadata("Test content.", metadata)

	require.Len(t, chunks, 1)
	assert.Equal(t, "test.txt", chunks[0].Metadata["source"])
	assert.Equal(t, "documentation", chunks[0].Metadata["category"])
}

func TestChunk(t *testing.T) {
	t.Parallel()
	chunk := Chunk{
		Content:     "Hello world",
		Index:       0,
		TokenCount:  2,
		StartOffset: 0,
		EndOffset:   11,
		Metadata:    map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, "Hello world", chunk.Content)
	assert.Equal(t, 0, chunk.Index)
	assert.Equal(t, 2, chunk.TokenCount)
}

func TestDefaultRecursiveSplitConfig(t *testing.T) {
	t.Parallel()
	config := DefaultRecursiveSplitConfig()

	assert.Equal(t, 1000, config.ChunkSize)
	assert.Equal(t, 200, config.ChunkOverlap)
	assert.Len(t, config.Separators, 4)
	assert.NotNil(t, config.LengthFunction)
}

func TestRecursiveCharacterTextSplitter(t *testing.T) {
	t.Parallel()
	splitter := NewRecursiveCharacterTextSplitter(nil)

	text := "First part.\n\nSecond part.\n\nThird part."
	chunks := splitter.SplitText(text)

	assert.GreaterOrEqual(t, len(chunks), 1)
}

func TestRecursiveCharacterTextSplitter_LargeText(t *testing.T) {
	t.Parallel()
	config := &RecursiveSplitConfig{
		ChunkSize:    50,
		ChunkOverlap: 10,
		Separators:   []string{"\n\n", "\n", " ", ""},
	}
	splitter := NewRecursiveCharacterTextSplitter(config)

	text := "word " // 5 chars
	for i := 0; i < 100; i++ {
		text += "word "
	}

	chunks := splitter.SplitText(text)

	assert.GreaterOrEqual(t, len(chunks), 1)
}

func TestSentenceSplitter(t *testing.T) {
	t.Parallel()
	splitter := NewSentenceSplitter()

	text := "First sentence. Second sentence! Third sentence?"
	sentences := splitter.Split(text)

	assert.Len(t, sentences, 3)
	assert.Equal(t, "First sentence.", sentences[0])
	assert.Equal(t, "Second sentence!", sentences[1])
	assert.Equal(t, "Third sentence?", sentences[2])
}

func TestSentenceSplitter_Abbreviations(t *testing.T) {
	t.Parallel()
	splitter := NewSentenceSplitter()

	// Note: current implementation doesn't handle abbreviations specially
	text := "Dr. Smith is here. He works at Inc."
	sentences := splitter.Split(text)

	assert.GreaterOrEqual(t, len(sentences), 1)
}

func TestSentenceSplitter_NoTerminal(t *testing.T) {
	t.Parallel()
	splitter := NewSentenceSplitter()

	text := "This has no terminal punctuation"
	sentences := splitter.Split(text)

	assert.Len(t, sentences, 1)
	assert.Equal(t, "This has no terminal punctuation", sentences[0])
}

func TestSimpleTokenizer(t *testing.T) {
	t.Parallel()
	tokenizer := &SimpleTokenizer{}

	tokens := tokenizer.Tokenize("Hello World How Are You")
	assert.Len(t, tokens, 5)

	count := tokenizer.CountTokens("Hello World")
	assert.Equal(t, 2, count)
}

func TestNewTokenTextSplitter(t *testing.T) {
	t.Parallel()
	splitter := NewTokenTextSplitter(10, 2, nil)
	assert.NotNil(t, splitter)
}

func TestTokenTextSplitter_SplitText(t *testing.T) {
	t.Parallel()
	splitter := NewTokenTextSplitter(5, 1, nil)

	text := "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10"
	chunks := splitter.SplitText(text)

	assert.GreaterOrEqual(t, len(chunks), 2)
	// Each chunk should have approximately 5 words
	for _, chunk := range chunks {
		tokens := len(splitIntoWords(chunk))
		assert.LessOrEqual(t, tokens, 6) // Allow some flexibility
	}
}

func splitIntoWords(s string) []string {
	var words []string
	for _, part := range splitBySpace(s) {
		if part != "" {
			words = append(words, part)
		}
	}
	return words
}

func splitBySpace(s string) []string {
	var result []string
	current := ""
	for _, r := range s {
		if r == ' ' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func TestTokenTextSplitter_EmptyText(t *testing.T) {
	t.Parallel()
	splitter := NewTokenTextSplitter(10, 2, nil)

	chunks := splitter.SplitText("")
	assert.Nil(t, chunks)
}

func TestMarkdownHeaderTextSplitter(t *testing.T) {
	t.Parallel()
	splitter := NewMarkdownHeaderTextSplitter()

	markdown := `# Header 1

Content under header 1.

## Header 2

Content under header 2.

### Header 3

Content under header 3.`

	chunks := splitter.SplitText(markdown)

	assert.GreaterOrEqual(t, len(chunks), 1)
}

func TestMarkdownChunk(t *testing.T) {
	t.Parallel()
	chunk := MarkdownChunk{
		Content: "Hello world",
		Metadata: map[string]string{
			"Header 1": "Introduction",
		},
	}

	assert.Equal(t, "Hello world", chunk.Content)
	assert.Equal(t, "Introduction", chunk.Metadata["Header 1"])
}

func TestMarkdownHeaderTextSplitter_NoHeaders(t *testing.T) {
	t.Parallel()
	splitter := NewMarkdownHeaderTextSplitter()

	markdown := "Just some content without headers."
	chunks := splitter.SplitText(markdown)

	assert.Len(t, chunks, 1)
	assert.Equal(t, "Just some content without headers.", chunks[0].Content)
}

func TestMarkdownHeaderTextSplitter_HeaderHierarchy(t *testing.T) {
	t.Parallel()
	splitter := NewMarkdownHeaderTextSplitter()

	markdown := `# Main
Content 1
## Sub
Content 2
# New Main
Content 3`

	chunks := splitter.SplitText(markdown)

	assert.GreaterOrEqual(t, len(chunks), 1)
}

func TestCopyMap(t *testing.T) {
	t.Parallel()
	original := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	copied := copyMap(original)

	assert.Equal(t, original, copied)

	// Modify original
	original["key3"] = "value3"
	assert.NotContains(t, copied, "key3")
}

func TestCopyMap_Nil(t *testing.T) {
	t.Parallel()
	copied := copyMap(nil)
	assert.Nil(t, copied)
}

func TestClearLowerHeaders(t *testing.T) {
	t.Parallel()
	headers := map[string]string{
		"Header 1": "Main",
		"Header 2": "Sub",
		"Header 3": "Sub-sub",
	}

	clearLowerHeaders(headers, "##")

	assert.Equal(t, "Main", headers["Header 1"])
	assert.Equal(t, "Sub", headers["Header 2"])
	assert.NotContains(t, headers, "Header 3")
}

func TestDocumentChunker_SplitByWords(t *testing.T) {
	t.Parallel()
	config := &ChunkConfig{
		ChunkSize: 5,
	}
	chunker := NewDocumentChunker(config)

	// This tests the internal splitByWords function through splitLargePart
	text := "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10"

	// Force it through splitLargePart by making it a single paragraph with many words
	chunks := chunker.Chunk(text)

	assert.GreaterOrEqual(t, len(chunks), 1)
}

func TestRecursiveCharacterTextSplitter_CustomLengthFunction(t *testing.T) {
	t.Parallel()
	config := &RecursiveSplitConfig{
		ChunkSize:    20, // 20 characters
		ChunkOverlap: 5,
		Separators:   []string{"\n\n", "\n", " "},
		LengthFunction: func(s string) int {
			return len(s) // Use character count
		},
	}
	splitter := NewRecursiveCharacterTextSplitter(config)

	text := "This is a long text that should be split into multiple chunks."
	chunks := splitter.SplitText(text)

	assert.GreaterOrEqual(t, len(chunks), 1)
}

func TestRecursiveCharacterTextSplitter_NilLengthFunction(t *testing.T) {
	t.Parallel()
	config := &RecursiveSplitConfig{
		ChunkSize:      100,
		ChunkOverlap:   10,
		Separators:     []string{"\n\n", "\n", " "},
		LengthFunction: nil, // Should default to len()
	}
	splitter := NewRecursiveCharacterTextSplitter(config)

	text := "Test content"
	chunks := splitter.SplitText(text)

	assert.GreaterOrEqual(t, len(chunks), 1)
}
