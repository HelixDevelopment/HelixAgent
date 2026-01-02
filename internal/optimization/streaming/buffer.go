// Package streaming provides enhanced streaming capabilities for LLM responses.
package streaming

import (
	"strings"
	"unicode"
)

// Buffer defines the interface for streaming buffers.
type Buffer interface {
	// Add adds text to the buffer and returns flushed content.
	Add(text string) []string
	// Flush returns any remaining content in the buffer.
	Flush() string
	// Reset clears the buffer.
	Reset()
}

// CharacterBuffer emits content character by character.
type CharacterBuffer struct{}

// NewCharacterBuffer creates a new character buffer.
func NewCharacterBuffer() *CharacterBuffer {
	return &CharacterBuffer{}
}

// Add returns each character as a separate string.
func (b *CharacterBuffer) Add(text string) []string {
	if text == "" {
		return nil
	}
	result := make([]string, 0, len(text))
	for _, r := range text {
		result = append(result, string(r))
	}
	return result
}

// Flush returns empty string (no buffering).
func (b *CharacterBuffer) Flush() string {
	return ""
}

// Reset is a no-op for character buffer.
func (b *CharacterBuffer) Reset() {}

// WordBuffer emits complete words.
type WordBuffer struct {
	buffer    strings.Builder
	delimiter string
}

// NewWordBuffer creates a new word buffer.
func NewWordBuffer(delimiter string) *WordBuffer {
	if delimiter == "" {
		delimiter = " "
	}
	return &WordBuffer{delimiter: delimiter}
}

// Add buffers text and returns complete words.
func (b *WordBuffer) Add(text string) []string {
	b.buffer.WriteString(text)
	content := b.buffer.String()

	var result []string
	for {
		idx := strings.Index(content, b.delimiter)
		if idx < 0 {
			break
		}
		word := content[:idx+len(b.delimiter)]
		result = append(result, word)
		content = content[idx+len(b.delimiter):]
	}

	b.buffer.Reset()
	b.buffer.WriteString(content)
	return result
}

// Flush returns remaining buffered content.
func (b *WordBuffer) Flush() string {
	remaining := b.buffer.String()
	b.buffer.Reset()
	return remaining
}

// Reset clears the buffer.
func (b *WordBuffer) Reset() {
	b.buffer.Reset()
}

// SentenceBuffer emits complete sentences.
type SentenceBuffer struct {
	buffer strings.Builder
}

// NewSentenceBuffer creates a new sentence buffer.
func NewSentenceBuffer() *SentenceBuffer {
	return &SentenceBuffer{}
}

var sentenceEndings = map[rune]bool{'.': true, '!': true, '?': true}

// Add buffers text and returns complete sentences.
func (b *SentenceBuffer) Add(text string) []string {
	b.buffer.WriteString(text)
	content := b.buffer.String()

	var result []string
	for {
		idx := b.findSentenceEnd(content)
		if idx < 0 {
			break
		}
		sentence := content[:idx+1]
		result = append(result, sentence)
		content = strings.TrimLeftFunc(content[idx+1:], unicode.IsSpace)
	}

	b.buffer.Reset()
	b.buffer.WriteString(content)
	return result
}

func (b *SentenceBuffer) findSentenceEnd(s string) int {
	for i, r := range s {
		if sentenceEndings[r] {
			// Check if followed by space or end of string
			if i == len(s)-1 || unicode.IsSpace(rune(s[i+1])) {
				return i
			}
		}
	}
	return -1
}

// Flush returns remaining buffered content.
func (b *SentenceBuffer) Flush() string {
	remaining := b.buffer.String()
	b.buffer.Reset()
	return remaining
}

// Reset clears the buffer.
func (b *SentenceBuffer) Reset() {
	b.buffer.Reset()
}

// LineBuffer emits complete lines.
type LineBuffer struct {
	buffer strings.Builder
}

// NewLineBuffer creates a new line buffer.
func NewLineBuffer() *LineBuffer {
	return &LineBuffer{}
}

// Add buffers text and returns complete lines.
func (b *LineBuffer) Add(text string) []string {
	b.buffer.WriteString(text)
	content := b.buffer.String()

	var result []string
	for {
		idx := strings.Index(content, "\n")
		if idx < 0 {
			break
		}
		line := content[:idx+1]
		result = append(result, line)
		content = content[idx+1:]
	}

	b.buffer.Reset()
	b.buffer.WriteString(content)
	return result
}

// Flush returns remaining buffered content.
func (b *LineBuffer) Flush() string {
	remaining := b.buffer.String()
	b.buffer.Reset()
	return remaining
}

// Reset clears the buffer.
func (b *LineBuffer) Reset() {
	b.buffer.Reset()
}

// ParagraphBuffer emits complete paragraphs (double newline).
type ParagraphBuffer struct {
	buffer strings.Builder
}

// NewParagraphBuffer creates a new paragraph buffer.
func NewParagraphBuffer() *ParagraphBuffer {
	return &ParagraphBuffer{}
}

// Add buffers text and returns complete paragraphs.
func (b *ParagraphBuffer) Add(text string) []string {
	b.buffer.WriteString(text)
	content := b.buffer.String()

	var result []string
	for {
		idx := strings.Index(content, "\n\n")
		if idx < 0 {
			break
		}
		paragraph := content[:idx+2]
		result = append(result, paragraph)
		content = content[idx+2:]
	}

	b.buffer.Reset()
	b.buffer.WriteString(content)
	return result
}

// Flush returns remaining buffered content.
func (b *ParagraphBuffer) Flush() string {
	remaining := b.buffer.String()
	b.buffer.Reset()
	return remaining
}

// Reset clears the buffer.
func (b *ParagraphBuffer) Reset() {
	b.buffer.Reset()
}

// TokenBuffer emits after accumulating N tokens (approximated by words).
type TokenBuffer struct {
	buffer     strings.Builder
	tokenCount int
	threshold  int
}

// NewTokenBuffer creates a new token buffer.
func NewTokenBuffer(threshold int) *TokenBuffer {
	if threshold <= 0 {
		threshold = 5
	}
	return &TokenBuffer{threshold: threshold}
}

// Add buffers text and returns when threshold is reached.
func (b *TokenBuffer) Add(text string) []string {
	b.buffer.WriteString(text)

	// Count approximate tokens (words)
	b.tokenCount += countTokens(text)

	if b.tokenCount >= b.threshold {
		content := b.buffer.String()
		b.buffer.Reset()
		b.tokenCount = 0
		return []string{content}
	}

	return nil
}

// Flush returns remaining buffered content.
func (b *TokenBuffer) Flush() string {
	remaining := b.buffer.String()
	b.buffer.Reset()
	b.tokenCount = 0
	return remaining
}

// Reset clears the buffer.
func (b *TokenBuffer) Reset() {
	b.buffer.Reset()
	b.tokenCount = 0
}

func countTokens(text string) int {
	// Simple approximation: count words
	words := strings.Fields(text)
	return len(words)
}

// BufferType defines the type of buffering strategy.
type BufferType string

const (
	BufferTypeCharacter BufferType = "character"
	BufferTypeWord      BufferType = "word"
	BufferTypeSentence  BufferType = "sentence"
	BufferTypeLine      BufferType = "line"
	BufferTypeParagraph BufferType = "paragraph"
	BufferTypeToken     BufferType = "token"
)

// NewBuffer creates a buffer of the specified type.
func NewBuffer(bufferType BufferType, options ...interface{}) Buffer {
	switch bufferType {
	case BufferTypeCharacter:
		return NewCharacterBuffer()
	case BufferTypeWord:
		delimiter := " "
		if len(options) > 0 {
			if d, ok := options[0].(string); ok {
				delimiter = d
			}
		}
		return NewWordBuffer(delimiter)
	case BufferTypeSentence:
		return NewSentenceBuffer()
	case BufferTypeLine:
		return NewLineBuffer()
	case BufferTypeParagraph:
		return NewParagraphBuffer()
	case BufferTypeToken:
		threshold := 5
		if len(options) > 0 {
			if t, ok := options[0].(int); ok {
				threshold = t
			}
		}
		return NewTokenBuffer(threshold)
	default:
		return NewWordBuffer(" ")
	}
}
