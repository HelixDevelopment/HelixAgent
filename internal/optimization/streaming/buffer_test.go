package streaming

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// CharacterBuffer Tests
// =============================================================================

func TestNewCharacterBuffer(t *testing.T) {
	t.Parallel()
	buf := NewCharacterBuffer()
	assert.NotNil(t, buf)
}

func TestCharacterBuffer_Add(t *testing.T) {
	t.Parallel()
	buf := NewCharacterBuffer()

	result := buf.Add("hello")
	assert.Equal(t, []string{"h", "e", "l", "l", "o"}, result)
}

func TestCharacterBuffer_Add_Empty(t *testing.T) {
	t.Parallel()
	buf := NewCharacterBuffer()

	result := buf.Add("")
	assert.Nil(t, result)
}

func TestCharacterBuffer_Add_Unicode(t *testing.T) {
	t.Parallel()
	buf := NewCharacterBuffer()

	result := buf.Add("abc")
	assert.Len(t, result, 3)
	assert.Equal(t, "a", result[0])
	assert.Equal(t, "b", result[1])
	assert.Equal(t, "c", result[2])
}

func TestCharacterBuffer_Flush(t *testing.T) {
	t.Parallel()
	buf := NewCharacterBuffer()
	assert.Equal(t, "", buf.Flush())
}

func TestCharacterBuffer_Reset_Dedicated(t *testing.T) {
	t.Parallel()
	buf := NewCharacterBuffer()
	buf.Reset() // Should not panic
}

func TestCharacterBuffer_ImplementsBuffer(t *testing.T) {
	t.Parallel()
	var _ Buffer = (*CharacterBuffer)(nil)
}

// =============================================================================
// WordBuffer Tests
// =============================================================================

func TestNewWordBuffer_DefaultDelimiter_Dedicated(t *testing.T) {
	t.Parallel()
	buf := NewWordBuffer("")
	assert.NotNil(t, buf)
	assert.Equal(t, " ", buf.delimiter)
}

func TestNewWordBuffer_CustomDelimiter_Dedicated(t *testing.T) {
	t.Parallel()
	buf := NewWordBuffer(",")
	assert.Equal(t, ",", buf.delimiter)
}

func TestWordBuffer_Add_CompleteWords(t *testing.T) {
	t.Parallel()
	buf := NewWordBuffer("")

	result := buf.Add("hello world ")
	assert.Equal(t, []string{"hello ", "world "}, result)
}

func TestWordBuffer_Add_PartialWord(t *testing.T) {
	t.Parallel()
	buf := NewWordBuffer("")

	// No delimiter at end means partial word stays in buffer
	result := buf.Add("hello")
	assert.Nil(t, result)

	// Complete the word
	result = buf.Add(" world ")
	assert.Equal(t, []string{"hello ", "world "}, result)
}

func TestWordBuffer_Add_CustomDelimiter(t *testing.T) {
	t.Parallel()
	buf := NewWordBuffer(",")

	result := buf.Add("a,b,c,")
	assert.Equal(t, []string{"a,", "b,", "c,"}, result)
}

func TestWordBuffer_Flush(t *testing.T) {
	t.Parallel()
	buf := NewWordBuffer("")
	buf.Add("incomplete")

	remaining := buf.Flush()
	assert.Equal(t, "incomplete", remaining)
}

func TestWordBuffer_Flush_Empty(t *testing.T) {
	t.Parallel()
	buf := NewWordBuffer("")
	assert.Equal(t, "", buf.Flush())
}

func TestWordBuffer_Reset(t *testing.T) {
	t.Parallel()
	buf := NewWordBuffer("")
	buf.Add("some text")
	buf.Reset()

	assert.Equal(t, "", buf.Flush())
}

func TestWordBuffer_ImplementsBuffer(t *testing.T) {
	t.Parallel()
	var _ Buffer = (*WordBuffer)(nil)
}

// =============================================================================
// SentenceBuffer Tests
// =============================================================================

func TestNewSentenceBuffer(t *testing.T) {
	t.Parallel()
	buf := NewSentenceBuffer()
	assert.NotNil(t, buf)
}

func TestSentenceBuffer_Add_CompleteSentence(t *testing.T) {
	t.Parallel()
	buf := NewSentenceBuffer()

	result := buf.Add("Hello world. ")
	assert.Equal(t, []string{"Hello world."}, result)
}

func TestSentenceBuffer_Add_MultipleSentences(t *testing.T) {
	t.Parallel()
	buf := NewSentenceBuffer()

	result := buf.Add("First. Second! Third? ")
	assert.Len(t, result, 3)
	assert.Equal(t, "First.", result[0])
	assert.Equal(t, "Second!", result[1])
	assert.Equal(t, "Third?", result[2])
}

func TestSentenceBuffer_Add_PartialSentence(t *testing.T) {
	t.Parallel()
	buf := NewSentenceBuffer()

	result := buf.Add("Hello world")
	assert.Nil(t, result)

	result = buf.Add(". Next sentence. ")
	assert.Len(t, result, 2)
	assert.Equal(t, "Hello world.", result[0])
}

func TestSentenceBuffer_Add_PeriodInMiddle(t *testing.T) {
	t.Parallel()
	buf := NewSentenceBuffer()

	// "3.14" has a period but not followed by space at the end
	result := buf.Add("Value is 3.14 right")
	// The period after 3 is followed by '1' (not space), so it should not be treated as sentence end
	assert.Nil(t, result)
}

func TestSentenceBuffer_Flush(t *testing.T) {
	t.Parallel()
	buf := NewSentenceBuffer()
	buf.Add("Incomplete sentence")

	remaining := buf.Flush()
	assert.Equal(t, "Incomplete sentence", remaining)
}

func TestSentenceBuffer_Reset(t *testing.T) {
	t.Parallel()
	buf := NewSentenceBuffer()
	buf.Add("Some content")
	buf.Reset()

	assert.Equal(t, "", buf.Flush())
}

func TestSentenceBuffer_ImplementsBuffer(t *testing.T) {
	t.Parallel()
	var _ Buffer = (*SentenceBuffer)(nil)
}

// =============================================================================
// LineBuffer Tests
// =============================================================================

func TestNewLineBuffer(t *testing.T) {
	t.Parallel()
	buf := NewLineBuffer()
	assert.NotNil(t, buf)
}

func TestLineBuffer_Add_CompleteLine(t *testing.T) {
	t.Parallel()
	buf := NewLineBuffer()

	result := buf.Add("Hello\n")
	assert.Equal(t, []string{"Hello\n"}, result)
}

func TestLineBuffer_Add_MultipleLines(t *testing.T) {
	t.Parallel()
	buf := NewLineBuffer()

	result := buf.Add("Line 1\nLine 2\nLine 3\n")
	assert.Len(t, result, 3)
	assert.Equal(t, "Line 1\n", result[0])
	assert.Equal(t, "Line 2\n", result[1])
	assert.Equal(t, "Line 3\n", result[2])
}

func TestLineBuffer_Add_PartialLine(t *testing.T) {
	t.Parallel()
	buf := NewLineBuffer()

	result := buf.Add("partial")
	assert.Nil(t, result)

	result = buf.Add(" line\nnext")
	assert.Len(t, result, 1)
	assert.Equal(t, "partial line\n", result[0])
}

func TestLineBuffer_Flush(t *testing.T) {
	t.Parallel()
	buf := NewLineBuffer()
	buf.Add("no newline")

	remaining := buf.Flush()
	assert.Equal(t, "no newline", remaining)
}

func TestLineBuffer_Flush_Empty(t *testing.T) {
	t.Parallel()
	buf := NewLineBuffer()
	assert.Equal(t, "", buf.Flush())
}

func TestLineBuffer_Reset(t *testing.T) {
	t.Parallel()
	buf := NewLineBuffer()
	buf.Add("some content")
	buf.Reset()

	assert.Equal(t, "", buf.Flush())
}

func TestLineBuffer_ImplementsBuffer(t *testing.T) {
	t.Parallel()
	var _ Buffer = (*LineBuffer)(nil)
}

// =============================================================================
// ParagraphBuffer Tests
// =============================================================================

func TestNewParagraphBuffer(t *testing.T) {
	t.Parallel()
	buf := NewParagraphBuffer()
	assert.NotNil(t, buf)
}

func TestParagraphBuffer_Add_CompleteParagraph(t *testing.T) {
	t.Parallel()
	buf := NewParagraphBuffer()

	result := buf.Add("First paragraph.\n\n")
	assert.Len(t, result, 1)
	assert.Equal(t, "First paragraph.\n\n", result[0])
}

func TestParagraphBuffer_Add_MultipleParagraphs(t *testing.T) {
	t.Parallel()
	buf := NewParagraphBuffer()

	result := buf.Add("Para 1.\n\nPara 2.\n\n")
	assert.Len(t, result, 2)
	assert.Equal(t, "Para 1.\n\n", result[0])
	assert.Equal(t, "Para 2.\n\n", result[1])
}

func TestParagraphBuffer_Add_PartialParagraph(t *testing.T) {
	t.Parallel()
	buf := NewParagraphBuffer()

	result := buf.Add("No double newline yet")
	assert.Nil(t, result)

	result = buf.Add("\n\nNext paragraph")
	assert.Len(t, result, 1)
	assert.Equal(t, "No double newline yet\n\n", result[0])
}

func TestParagraphBuffer_Flush(t *testing.T) {
	t.Parallel()
	buf := NewParagraphBuffer()
	buf.Add("Incomplete paragraph")

	remaining := buf.Flush()
	assert.Equal(t, "Incomplete paragraph", remaining)
}

func TestParagraphBuffer_Reset_Dedicated(t *testing.T) {
	t.Parallel()
	buf := NewParagraphBuffer()
	buf.Add("content")
	buf.Reset()

	assert.Equal(t, "", buf.Flush())
}

func TestParagraphBuffer_ImplementsBuffer(t *testing.T) {
	t.Parallel()
	var _ Buffer = (*ParagraphBuffer)(nil)
}

// =============================================================================
// TokenBuffer Tests
// =============================================================================

func TestNewTokenBuffer_DefaultThreshold_Dedicated(t *testing.T) {
	t.Parallel()
	buf := NewTokenBuffer(0)
	assert.Equal(t, 5, buf.threshold)
}

func TestNewTokenBuffer_NegativeThreshold(t *testing.T) {
	t.Parallel()
	buf := NewTokenBuffer(-1)
	assert.Equal(t, 5, buf.threshold)
}

func TestNewTokenBuffer_CustomThreshold_Dedicated(t *testing.T) {
	t.Parallel()
	buf := NewTokenBuffer(10)
	assert.Equal(t, 10, buf.threshold)
}

func TestTokenBuffer_Add_BelowThreshold(t *testing.T) {
	t.Parallel()
	buf := NewTokenBuffer(5)

	result := buf.Add("one two")
	assert.Nil(t, result)
}

func TestTokenBuffer_Add_AtThreshold(t *testing.T) {
	t.Parallel()
	buf := NewTokenBuffer(3)

	result := buf.Add("one two three")
	assert.Len(t, result, 1)
	assert.Equal(t, "one two three", result[0])
}

func TestTokenBuffer_Add_AccumulateToThreshold(t *testing.T) {
	t.Parallel()
	buf := NewTokenBuffer(3)

	result := buf.Add("one ")
	assert.Nil(t, result)

	result = buf.Add("two ")
	assert.Nil(t, result)

	result = buf.Add("three")
	assert.Len(t, result, 1)
	assert.Equal(t, "one two three", result[0])
}

func TestTokenBuffer_Flush(t *testing.T) {
	t.Parallel()
	buf := NewTokenBuffer(10)
	buf.Add("partial content")

	remaining := buf.Flush()
	assert.Equal(t, "partial content", remaining)

	// After flush, buffer should be empty
	assert.Equal(t, "", buf.Flush())
}

func TestTokenBuffer_Reset(t *testing.T) {
	t.Parallel()
	buf := NewTokenBuffer(5)
	buf.Add("some words here")
	buf.Reset()

	assert.Equal(t, "", buf.Flush())
	assert.Equal(t, 0, buf.tokenCount)
}

func TestTokenBuffer_ImplementsBuffer(t *testing.T) {
	t.Parallel()
	var _ Buffer = (*TokenBuffer)(nil)
}

// =============================================================================
// NewBuffer Factory Tests
// =============================================================================

func TestNewBuffer_Character(t *testing.T) {
	t.Parallel()
	buf := NewBuffer(BufferTypeCharacter)
	require.NotNil(t, buf)
	_, ok := buf.(*CharacterBuffer)
	assert.True(t, ok)
}

func TestNewBuffer_Word(t *testing.T) {
	t.Parallel()
	buf := NewBuffer(BufferTypeWord)
	require.NotNil(t, buf)
	_, ok := buf.(*WordBuffer)
	assert.True(t, ok)
}

func TestNewBuffer_Word_WithDelimiter(t *testing.T) {
	t.Parallel()
	buf := NewBuffer(BufferTypeWord, ",")
	require.NotNil(t, buf)
	wb, ok := buf.(*WordBuffer)
	assert.True(t, ok)
	assert.Equal(t, ",", wb.delimiter)
}

func TestNewBuffer_Sentence(t *testing.T) {
	t.Parallel()
	buf := NewBuffer(BufferTypeSentence)
	require.NotNil(t, buf)
	_, ok := buf.(*SentenceBuffer)
	assert.True(t, ok)
}

func TestNewBuffer_Line(t *testing.T) {
	t.Parallel()
	buf := NewBuffer(BufferTypeLine)
	require.NotNil(t, buf)
	_, ok := buf.(*LineBuffer)
	assert.True(t, ok)
}

func TestNewBuffer_Paragraph(t *testing.T) {
	t.Parallel()
	buf := NewBuffer(BufferTypeParagraph)
	require.NotNil(t, buf)
	_, ok := buf.(*ParagraphBuffer)
	assert.True(t, ok)
}

func TestNewBuffer_Token(t *testing.T) {
	t.Parallel()
	buf := NewBuffer(BufferTypeToken)
	require.NotNil(t, buf)
	tb, ok := buf.(*TokenBuffer)
	assert.True(t, ok)
	assert.Equal(t, 5, tb.threshold) // default
}

func TestNewBuffer_Token_WithThreshold(t *testing.T) {
	t.Parallel()
	buf := NewBuffer(BufferTypeToken, 10)
	require.NotNil(t, buf)
	tb, ok := buf.(*TokenBuffer)
	assert.True(t, ok)
	assert.Equal(t, 10, tb.threshold)
}

func TestNewBuffer_Unknown(t *testing.T) {
	t.Parallel()
	buf := NewBuffer(BufferType("unknown"))
	require.NotNil(t, buf)
	// Should default to WordBuffer
	_, ok := buf.(*WordBuffer)
	assert.True(t, ok)
}

// =============================================================================
// BufferType Constants Tests
// =============================================================================

func TestBufferType_Constants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, BufferType("character"), BufferTypeCharacter)
	assert.Equal(t, BufferType("word"), BufferTypeWord)
	assert.Equal(t, BufferType("sentence"), BufferTypeSentence)
	assert.Equal(t, BufferType("line"), BufferTypeLine)
	assert.Equal(t, BufferType("paragraph"), BufferTypeParagraph)
	assert.Equal(t, BufferType("token"), BufferTypeToken)
}

// =============================================================================
// countTokens Tests
// =============================================================================

func TestCountTokens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty string", "", 0},
		{"single word", "hello", 1},
		{"multiple words", "hello world test", 3},
		{"extra spaces", "  hello   world  ", 2},
		{"tabs and newlines", "hello\tworld\ntest", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, countTokens(tt.input))
		})
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkCharacterBuffer_Add(b *testing.B) {
	buf := NewCharacterBuffer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Add("hello world")
	}
}

func BenchmarkWordBuffer_Add(b *testing.B) {
	buf := NewWordBuffer("")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Add("hello world ")
		buf.Reset()
	}
}

func BenchmarkSentenceBuffer_Add(b *testing.B) {
	buf := NewSentenceBuffer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Add("Hello world. This is a test. ")
		buf.Reset()
	}
}

func BenchmarkLineBuffer_Add(b *testing.B) {
	buf := NewLineBuffer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Add("Line one\nLine two\n")
		buf.Reset()
	}
}

func BenchmarkTokenBuffer_Add(b *testing.B) {
	buf := NewTokenBuffer(5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Add("one two three four five ")
		buf.Reset()
	}
}
