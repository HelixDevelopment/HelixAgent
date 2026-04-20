package streaming

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultChunkingContext(t *testing.T) {
	ctx := DefaultChunkingContext()

	assert.Equal(t, "en", ctx.Language)
	assert.Equal(t, ContentTypeProse, ctx.ContentType)
	assert.False(t, ctx.PreserveFormatting)
	assert.Equal(t, 512, ctx.MaxChunkSize)
	assert.Equal(t, 64, ctx.MinChunkSize)
	assert.Equal(t, 0, ctx.OverlapTokens)
}

func TestDefaultChunkerConfig(t *testing.T) {
	config := DefaultChunkerConfig()

	assert.Equal(t, StrategySemantic, config.Strategy)
	assert.Equal(t, 512, config.MaxTokens)
	assert.Equal(t, 64, config.MinTokens)
	assert.Equal(t, 0, config.OverlapTokens)
	assert.True(t, config.PreserveSentences)
	assert.True(t, config.PreserveParagraphs)
	assert.True(t, config.SplitOnNewlines)
}

func TestSmartChunker_FixedStrategy(t *testing.T) {
	config := &ChunkerConfig{
		Strategy:  StrategyFixed,
		MaxTokens: 10,
	}
	chunker := NewSmartChunker(config)

	text := "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12"
	chunks := chunker.Chunk(text)

	assert.True(t, len(chunks) >= 1)
	for _, chunk := range chunks {
		assert.LessOrEqual(t, len(strings.Fields(chunk)), 10)
	}
}

func TestSmartChunker_SemanticStrategy(t *testing.T) {
	config := &ChunkerConfig{
		Strategy:           StrategySemantic,
		MaxTokens:          50,
		PreserveSentences:  true,
		PreserveParagraphs: true,
	}
	chunker := NewSmartChunker(config)

	text := `This is the first paragraph. It has multiple sentences.

This is the second paragraph. It also has content.

This is the third paragraph.`

	chunks := chunker.Chunk(text)

	assert.True(t, len(chunks) >= 1)
}

func TestSmartChunker_RecursiveStrategy(t *testing.T) {
	config := &ChunkerConfig{
		Strategy:  StrategyRecursive,
		MaxTokens: 20,
	}
	chunker := NewSmartChunker(config)

	text := "First paragraph with content.\n\nSecond paragraph with more content.\n\nThird paragraph."
	chunks := chunker.Chunk(text)

	assert.True(t, len(chunks) >= 1)
	for _, chunk := range chunks {
		assert.LessOrEqual(t, countTokensSimple(chunk), 20)
	}
}

func TestSmartChunker_HybridStrategy(t *testing.T) {
	config := &ChunkerConfig{
		Strategy:  StrategyHybrid,
		MaxTokens: 30,
	}
	chunker := NewSmartChunker(config)

	text := "Short paragraph.\n\nThis is a much longer paragraph that contains many words and might exceed the maximum token limit if not handled properly by the chunking algorithm."
	chunks := chunker.Chunk(text)

	assert.True(t, len(chunks) >= 1)
}

func TestSmartChunker_Reset(t *testing.T) {
	chunker := NewSmartChunker(nil)
	chunker.pendingBuffer.WriteString("test")

	chunker.Reset()

	assert.Equal(t, 0, chunker.pendingBuffer.Len())
}

func TestSmartChunker_EmptyInput(t *testing.T) {
	chunker := NewSmartChunker(nil)

	chunks := chunker.Chunk("")
	assert.Nil(t, chunks)
}

func TestSmartChunker_WithContext(t *testing.T) {
	chunker := NewSmartChunker(nil)

	ctx := &ChunkingContext{
		ContentType:  ContentTypeCode,
		MaxChunkSize: 100,
	}

	text := "Line 1\nLine 2\nLine 3"
	chunks := chunker.ChunkWithContext(text, ctx)

	assert.True(t, len(chunks) >= 1)
}

func TestSmartChunker_NilContext(t *testing.T) {
	chunker := NewSmartChunker(nil)

	text := "Some test text"
	chunks := chunker.ChunkWithContext(text, nil)

	assert.True(t, len(chunks) >= 1)
}

func TestStreamingChunker_Add(t *testing.T) {
	var chunks []string
	var mu sync.Mutex

	callback := func(chunk string, index int) {
		mu.Lock()
		chunks = append(chunks, chunk)
		mu.Unlock()
	}

	config := &ChunkerConfig{
		MaxTokens:         10,
		MinTokens:         3,
		PreserveSentences: true,
	}

	chunker := NewStreamingChunker(config, callback)

	// Add content that should trigger a chunk
	chunker.Add("This is a test sentence. ")
	chunker.Add("Another sentence here. ")
	chunker.Add("And more content.")

	remaining := chunker.Flush()

	mu.Lock()
	totalChunks := len(chunks)
	mu.Unlock()

	assert.True(t, totalChunks >= 1 || remaining != "")
}

func TestStreamingChunker_Flush(t *testing.T) {
	chunker := NewStreamingChunker(nil, nil)

	chunker.Add("Some content")
	remaining := chunker.Flush()

	assert.NotEmpty(t, remaining)
	assert.Equal(t, "", chunker.Flush()) // Second flush should be empty
}

func TestStreamingChunker_GetChunks(t *testing.T) {
	chunker := NewStreamingChunker(&ChunkerConfig{
		MaxTokens: 5,
		MinTokens: 1,
	}, nil)

	chunker.Add("word1 word2 word3 word4 word5 word6 word7")
	chunker.Flush()

	chunks := chunker.GetChunks()
	assert.True(t, len(chunks) >= 1)
}

func TestStreamingChunker_Reset(t *testing.T) {
	chunker := NewStreamingChunker(nil, nil)

	chunker.Add("Some content")
	chunker.Reset()

	assert.Equal(t, 0, chunker.buffer.Len())
	assert.Equal(t, 0, chunker.chunks.Len())
	assert.Equal(t, 0, chunker.tokenCount)
}

func TestStreamingChunker_SentenceBoundary(t *testing.T) {
	var emittedChunks []string
	callback := func(chunk string, index int) {
		emittedChunks = append(emittedChunks, chunk)
	}

	config := &ChunkerConfig{
		MaxTokens:         100,
		MinTokens:         3,
		PreserveSentences: true,
	}

	chunker := NewStreamingChunker(config, callback)

	// Add a complete sentence
	chunker.Add("This is a complete sentence. ")
	chunker.Add("Another one here.")
	chunker.Flush()

	assert.True(t, len(emittedChunks) >= 1)
}

func TestStreamingChunker_NewlineSplit(t *testing.T) {
	var emittedChunks []string
	callback := func(chunk string, index int) {
		emittedChunks = append(emittedChunks, chunk)
	}

	config := &ChunkerConfig{
		MaxTokens:       100,
		MinTokens:       3,
		SplitOnNewlines: true,
	}

	chunker := NewStreamingChunker(config, callback)

	chunker.Add("Line 1\n")
	chunker.Add("Line 2\n")
	chunker.Add("Line 3")
	chunker.Flush()

	assert.True(t, len(emittedChunks) >= 1)
}

func TestChunkerChannel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	config := &ChunkerConfig{
		MaxTokens: 10,
		MinTokens: 1,
	}

	input := make(chan string, 5)
	input <- "word1 word2 word3 word4 word5 "
	input <- "word6 word7 word8 word9 word10 "
	input <- "word11 word12"
	close(input)

	output := ChunkerChannel(ctx, input, config)

	var chunks []string
	for chunk := range output {
		chunks = append(chunks, chunk)
	}

	// At least one chunk should be produced
	assert.True(t, len(chunks) >= 1)
}

func TestCodeChunker(t *testing.T) {
	chunker := NewCodeChunker("go", nil)

	code := `package main

import "fmt"

func main() {
    fmt.Println("Hello")
}

func helper() {
    // Helper function
}`

	chunks := chunker.ChunkCode(code)

	assert.True(t, len(chunks) >= 1)
}

func TestCodeChunker_CustomConfig(t *testing.T) {
	config := &ChunkerConfig{
		MaxTokens: 50,
		MinTokens: 10,
	}
	chunker := NewCodeChunker("python", config)

	code := "def hello():\n    print('Hello')\n\ndef world():\n    print('World')"
	chunks := chunker.ChunkCode(code)

	assert.True(t, len(chunks) >= 1)
}

func TestMarkdownChunker(t *testing.T) {
	chunker := NewMarkdownChunker(nil)

	markdown := `# Header 1

Introduction paragraph.

## Header 2

More content here.

### Header 3

Even more content.`

	chunks := chunker.ChunkMarkdown(markdown)

	assert.True(t, len(chunks) >= 1)
}

func TestMarkdownChunker_CustomConfig(t *testing.T) {
	config := &ChunkerConfig{
		MaxTokens: 100,
	}
	chunker := NewMarkdownChunker(config)

	markdown := "# Title\n\nContent paragraph."
	chunks := chunker.ChunkMarkdown(markdown)

	assert.True(t, len(chunks) >= 1)
}

func TestSplitParagraphs(t *testing.T) {
	text := "First paragraph.\n\nSecond paragraph.\n\nThird paragraph."
	paragraphs := splitParagraphs(text)

	assert.Len(t, paragraphs, 3)
	assert.Equal(t, "First paragraph.", paragraphs[0])
	assert.Equal(t, "Second paragraph.", paragraphs[1])
	assert.Equal(t, "Third paragraph.", paragraphs[2])
}

func TestSplitParagraphs_Empty(t *testing.T) {
	paragraphs := splitParagraphs("")
	assert.Empty(t, paragraphs)
}

func TestSplitSentences(t *testing.T) {
	text := "First sentence. Second sentence! Third sentence?"
	sentences := splitSentences(text)

	assert.Len(t, sentences, 3)
	assert.Equal(t, "First sentence.", sentences[0])
	assert.Equal(t, "Second sentence!", sentences[1])
	assert.Equal(t, "Third sentence?", sentences[2])
}

func TestSplitSentences_NoTerminal(t *testing.T) {
	text := "This has no terminal punctuation"
	sentences := splitSentences(text)

	assert.Len(t, sentences, 1)
	assert.Equal(t, "This has no terminal punctuation", sentences[0])
}

func TestCountTokensSimple(t *testing.T) {
	text := "This is a test with seven words"
	count := countTokensSimple(text)

	assert.Equal(t, 7, count)
}

func TestCountTokensSimple_Empty(t *testing.T) {
	count := countTokensSimple("")
	assert.Equal(t, 0, count)
}

func TestContentTypes(t *testing.T) {
	assert.Equal(t, ContentType("prose"), ContentTypeProse)
	assert.Equal(t, ContentType("code"), ContentTypeCode)
	assert.Equal(t, ContentType("markdown"), ContentTypeMarkdown)
	assert.Equal(t, ContentType("json"), ContentTypeJSON)
	assert.Equal(t, ContentType("xml"), ContentTypeXML)
	assert.Equal(t, ContentType("unknown"), ContentTypeUnknown)
}

func TestChunkingStrategies(t *testing.T) {
	assert.Equal(t, ChunkingStrategy("fixed"), StrategyFixed)
	assert.Equal(t, ChunkingStrategy("semantic"), StrategySemantic)
	assert.Equal(t, ChunkingStrategy("recursive"), StrategyRecursive)
	assert.Equal(t, ChunkingStrategy("hybrid"), StrategyHybrid)
}

func TestSmartChunker_LargeParagraph(t *testing.T) {
	config := &ChunkerConfig{
		Strategy:  StrategyFixed, // Use fixed strategy for strict token limits
		MaxTokens: 10,
	}
	chunker := NewSmartChunker(config)

	// Create a paragraph larger than max tokens
	text := "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12 word13 word14 word15"
	chunks := chunker.Chunk(text)

	assert.True(t, len(chunks) >= 1)
	// Fixed strategy should produce multiple chunks
	assert.True(t, len(chunks) >= 2)
}

func TestSmartChunker_DefaultStrategy(t *testing.T) {
	config := &ChunkerConfig{
		Strategy:  "unknown", // Invalid strategy
		MaxTokens: 50,
	}
	chunker := NewSmartChunker(config)

	text := "Test content"
	chunks := chunker.Chunk(text)

	// Should fallback to semantic strategy
	assert.True(t, len(chunks) >= 1)
}

// ============================================================================
// splitByTokens and recursiveSplit with empty separators (method calls)
// splitByTokens is reached when recursiveSplit runs out of separators.
// Both are methods on SmartChunker, accessible from the same package.
// ============================================================================

func TestSplitByTokens_Basic(t *testing.T) {
	chunker := NewSmartChunker(nil)
	text := "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10"
	chunks := chunker.splitByTokens(text, 3)

	assert.True(t, len(chunks) >= 1)
	for _, c := range chunks {
		assert.LessOrEqual(t, countTokensSimple(c), 3)
	}
}

func TestSplitByTokens_SingleWord(t *testing.T) {
	chunker := NewSmartChunker(nil)
	chunks := chunker.splitByTokens("oneword", 5)
	assert.Len(t, chunks, 1)
	assert.Equal(t, "oneword", chunks[0])
}

func TestSplitByTokens_ExactLimit(t *testing.T) {
	chunker := NewSmartChunker(nil)
	// Text has exactly maxTokens words — should produce exactly 1 chunk
	text := "a b c d e" // 5 words
	chunks := chunker.splitByTokens(text, 5)
	assert.Len(t, chunks, 1)
}

func TestSplitByTokens_LargerThanLimit(t *testing.T) {
	chunker := NewSmartChunker(nil)
	// Text has more words than maxTokens — should produce multiple chunks
	text := "a b c d e f g h i j" // 10 words
	chunks := chunker.splitByTokens(text, 4)
	assert.True(t, len(chunks) >= 2)
}

func TestSplitByTokens_Empty(t *testing.T) {
	chunker := NewSmartChunker(nil)
	chunks := chunker.splitByTokens("", 10)
	assert.Empty(t, chunks)
}

func TestRecursiveSplit_EmptySeparators(t *testing.T) {
	chunker := NewSmartChunker(nil)
	// With empty separators slice, recursiveSplit falls through to splitByTokens
	text := "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12"
	chunks := chunker.recursiveSplit(text, []string{}, 5)

	assert.True(t, len(chunks) >= 1)
	for _, c := range chunks {
		assert.LessOrEqual(t, countTokensSimple(c), 5)
	}
}

func TestRecursiveSplit_ForcesTokenSplit(t *testing.T) {
	chunker := NewSmartChunker(nil)
	// Text without any separators in the list — falls back to splitByTokens
	text := "aaaaa bbbbb ccccc ddddd eeeee fffff ggggg hhhhh iiiii jjjjj"
	chunks := chunker.recursiveSplit(text, []string{""}, 3)

	assert.True(t, len(chunks) >= 1)
}

func TestRecursiveSplit_WithSeparators(t *testing.T) {
	chunker := NewSmartChunker(nil)
	text := "First part\n\nSecond part\n\nThird part"
	chunks := chunker.recursiveSplit(text, []string{"\n\n", "\n", " "}, 5)

	assert.True(t, len(chunks) >= 1)
}
