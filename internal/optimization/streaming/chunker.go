// Package streaming provides enhanced streaming capabilities for LLM responses.
package streaming

import (
	"context"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"digital.vasic.concurrency/pkg/safe"
)

// Chunker defines the interface for smart token chunking.
type Chunker interface {
	// Chunk processes input text and returns chunks.
	Chunk(text string) []string
	// ChunkWithContext chunks with context awareness.
	ChunkWithContext(text string, ctx *ChunkingContext) []string
	// Reset resets the chunker state.
	Reset()
}

// ChunkingContext provides context for chunking decisions.
type ChunkingContext struct {
	// Language is the content language.
	Language string
	// ContentType is the type of content (code, prose, etc.).
	ContentType ContentType
	// PreserveFormatting preserves whitespace and formatting.
	PreserveFormatting bool
	// MaxChunkSize is the maximum chunk size in tokens.
	MaxChunkSize int
	// MinChunkSize is the minimum chunk size in tokens.
	MinChunkSize int
	// OverlapTokens is the number of tokens to overlap between chunks.
	OverlapTokens int
}

// ContentType defines the type of content being chunked.
type ContentType string

const (
	// ContentTypeProse is natural language prose.
	ContentTypeProse ContentType = "prose"
	// ContentTypeCode is source code.
	ContentTypeCode ContentType = "code"
	// ContentTypeMarkdown is Markdown content.
	ContentTypeMarkdown ContentType = "markdown"
	// ContentTypeJSON is JSON content.
	ContentTypeJSON ContentType = "json"
	// ContentTypeXML is XML content.
	ContentTypeXML ContentType = "xml"
	// ContentTypeUnknown is unknown content type.
	ContentTypeUnknown ContentType = "unknown"
)

// DefaultChunkingContext returns a default chunking context.
func DefaultChunkingContext() *ChunkingContext {
	return &ChunkingContext{
		Language:           "en",
		ContentType:        ContentTypeProse,
		PreserveFormatting: false,
		MaxChunkSize:       512,
		MinChunkSize:       64,
		OverlapTokens:      0,
	}
}

// SmartChunker implements intelligent chunking based on content structure.
type SmartChunker struct {
	mu            sync.Mutex
	config        *ChunkerConfig
	pendingBuffer strings.Builder
}

// ChunkerConfig holds configuration for the smart chunker.
type ChunkerConfig struct {
	// Strategy is the chunking strategy.
	Strategy ChunkingStrategy `json:"strategy"`
	// MaxTokens is the maximum tokens per chunk.
	MaxTokens int `json:"max_tokens"`
	// MinTokens is the minimum tokens per chunk.
	MinTokens int `json:"min_tokens"`
	// OverlapTokens is the overlap between chunks.
	OverlapTokens int `json:"overlap_tokens"`
	// PreserveSentences keeps sentences together.
	PreserveSentences bool `json:"preserve_sentences"`
	// PreserveParagraphs keeps paragraphs together.
	PreserveParagraphs bool `json:"preserve_paragraphs"`
	// SplitOnNewlines splits on newlines.
	SplitOnNewlines bool `json:"split_on_newlines"`
}

// ChunkingStrategy defines the chunking approach.
type ChunkingStrategy string

const (
	// StrategyFixed uses fixed-size chunks.
	StrategyFixed ChunkingStrategy = "fixed"
	// StrategySemantic uses semantic boundaries.
	StrategySemantic ChunkingStrategy = "semantic"
	// StrategyRecursive uses recursive splitting.
	StrategyRecursive ChunkingStrategy = "recursive"
	// StrategyHybrid combines multiple strategies.
	StrategyHybrid ChunkingStrategy = "hybrid"
)

// DefaultChunkerConfig returns a default chunker configuration.
func DefaultChunkerConfig() *ChunkerConfig {
	return &ChunkerConfig{
		Strategy:           StrategySemantic,
		MaxTokens:          512,
		MinTokens:          64,
		OverlapTokens:      0,
		PreserveSentences:  true,
		PreserveParagraphs: true,
		SplitOnNewlines:    true,
	}
}

// NewSmartChunker creates a new smart chunker.
func NewSmartChunker(config *ChunkerConfig) *SmartChunker {
	if config == nil {
		config = DefaultChunkerConfig()
	}
	return &SmartChunker{
		config: config,
	}
}

// Chunk processes input text and returns chunks.
func (c *SmartChunker) Chunk(text string) []string {
	return c.ChunkWithContext(text, DefaultChunkingContext())
}

// ChunkWithContext chunks with context awareness.
func (c *SmartChunker) ChunkWithContext(text string, ctx *ChunkingContext) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ctx == nil {
		ctx = DefaultChunkingContext()
	}

	switch c.config.Strategy {
	case StrategyFixed:
		return c.fixedChunk(text, ctx)
	case StrategySemantic:
		return c.semanticChunk(text, ctx)
	case StrategyRecursive:
		return c.recursiveChunk(text, ctx)
	case StrategyHybrid:
		return c.hybridChunk(text, ctx)
	default:
		return c.semanticChunk(text, ctx)
	}
}

// Reset resets the chunker state.
func (c *SmartChunker) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pendingBuffer.Reset()
}

func (c *SmartChunker) fixedChunk(text string, ctx *ChunkingContext) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	maxSize := c.config.MaxTokens
	if ctx.MaxChunkSize > 0 && ctx.MaxChunkSize < maxSize {
		maxSize = ctx.MaxChunkSize
	}

	var chunks []string
	var currentChunk []string

	for _, word := range words {
		if len(currentChunk) >= maxSize {
			chunks = append(chunks, strings.Join(currentChunk, " "))
			// Handle overlap
			if c.config.OverlapTokens > 0 && len(currentChunk) > c.config.OverlapTokens {
				currentChunk = currentChunk[len(currentChunk)-c.config.OverlapTokens:]
			} else {
				currentChunk = nil
			}
		}
		currentChunk = append(currentChunk, word)
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, strings.Join(currentChunk, " "))
	}

	return chunks
}

func (c *SmartChunker) semanticChunk(text string, ctx *ChunkingContext) []string {
	// Split into paragraphs first
	paragraphs := splitParagraphs(text)
	if len(paragraphs) == 0 {
		return nil
	}

	var chunks []string
	var currentChunk strings.Builder
	currentTokens := 0

	maxTokens := c.config.MaxTokens
	if ctx.MaxChunkSize > 0 && ctx.MaxChunkSize < maxTokens {
		maxTokens = ctx.MaxChunkSize
	}

	for _, para := range paragraphs {
		paraTokens := countTokensSimple(para)

		// If paragraph alone exceeds max, split by sentences
		if paraTokens > maxTokens {
			if currentChunk.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
				currentChunk.Reset()
				currentTokens = 0
			}

			sentences := splitSentences(para)
			for _, sent := range sentences {
				sentTokens := countTokensSimple(sent)

				if currentTokens+sentTokens > maxTokens && currentChunk.Len() > 0 {
					chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
					currentChunk.Reset()
					currentTokens = 0
				}

				if currentChunk.Len() > 0 {
					currentChunk.WriteString(" ")
				}
				currentChunk.WriteString(sent)
				currentTokens += sentTokens
			}
		} else if currentTokens+paraTokens > maxTokens {
			// Flush current chunk and start new one
			if currentChunk.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
				currentChunk.Reset()
			}
			currentChunk.WriteString(para)
			currentTokens = paraTokens
		} else {
			// Add paragraph to current chunk
			if currentChunk.Len() > 0 {
				currentChunk.WriteString("\n\n")
			}
			currentChunk.WriteString(para)
			currentTokens += paraTokens
		}
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	return chunks
}

func (c *SmartChunker) recursiveChunk(text string, ctx *ChunkingContext) []string {
	maxTokens := c.config.MaxTokens
	if ctx.MaxChunkSize > 0 && ctx.MaxChunkSize < maxTokens {
		maxTokens = ctx.MaxChunkSize
	}

	// Separators in order of preference
	separators := []string{"\n\n", "\n", ". ", ", ", " ", ""}

	return c.recursiveSplit(text, separators, maxTokens)
}

func (c *SmartChunker) recursiveSplit(text string, separators []string, maxTokens int) []string {
	if countTokensSimple(text) <= maxTokens {
		return []string{text}
	}

	if len(separators) == 0 {
		// Last resort: split by characters
		return c.splitByTokens(text, maxTokens)
	}

	sep := separators[0]
	parts := strings.Split(text, sep)

	var chunks []string
	var current strings.Builder

	for i, part := range parts {
		partTokens := countTokensSimple(part)
		currentTokens := countTokensSimple(current.String())

		if partTokens > maxTokens {
			// Part itself is too large, recursively split
			if current.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(current.String()))
				current.Reset()
			}
			subChunks := c.recursiveSplit(part, separators[1:], maxTokens)
			chunks = append(chunks, subChunks...)
		} else if currentTokens+partTokens > maxTokens {
			// Adding this part would exceed limit
			if current.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(current.String()))
				current.Reset()
			}
			current.WriteString(part)
		} else {
			// Add part to current chunk
			if current.Len() > 0 && sep != "" {
				current.WriteString(sep)
			}
			current.WriteString(part)
		}

		// Add separator back for all but last part
		if i < len(parts)-1 && sep == ". " {
			if current.Len() > 0 && !strings.HasSuffix(current.String(), ".") {
				current.WriteString(".")
			}
		}
	}

	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}

	return chunks
}

func (c *SmartChunker) splitByTokens(text string, maxTokens int) []string {
	words := strings.Fields(text)
	var chunks []string
	var current []string

	for _, word := range words {
		if len(current) >= maxTokens {
			chunks = append(chunks, strings.Join(current, " "))
			current = nil
		}
		current = append(current, word)
	}

	if len(current) > 0 {
		chunks = append(chunks, strings.Join(current, " "))
	}

	return chunks
}

func (c *SmartChunker) hybridChunk(text string, ctx *ChunkingContext) []string {
	// First, try semantic chunking
	chunks := c.semanticChunk(text, ctx)

	maxTokens := c.config.MaxTokens
	if ctx.MaxChunkSize > 0 && ctx.MaxChunkSize < maxTokens {
		maxTokens = ctx.MaxChunkSize
	}

	// Then, ensure no chunk exceeds max tokens
	var result []string
	for _, chunk := range chunks {
		if countTokensSimple(chunk) > maxTokens {
			// Split oversized chunks further
			subChunks := c.recursiveChunk(chunk, ctx)
			result = append(result, subChunks...)
		} else {
			result = append(result, chunk)
		}
	}

	return result
}

// Helper functions

func splitParagraphs(text string) []string {
	// Split on double newlines
	parts := strings.Split(text, "\n\n")
	var paragraphs []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			paragraphs = append(paragraphs, p)
		}
	}
	return paragraphs
}

func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder
	runes := []rune(text)

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		current.WriteRune(r)

		// Check for sentence ending
		if r == '.' || r == '!' || r == '?' {
			// Check if followed by space or end
			if i == len(runes)-1 || unicode.IsSpace(runes[i+1]) {
				sentence := strings.TrimSpace(current.String())
				if sentence != "" {
					sentences = append(sentences, sentence)
				}
				current.Reset()
			}
		}
	}

	// Handle remaining text
	remaining := strings.TrimSpace(current.String())
	if remaining != "" {
		sentences = append(sentences, remaining)
	}

	return sentences
}

func countTokensSimple(text string) int {
	// Simple approximation: count words
	return len(strings.Fields(text))
}

// StreamingChunker chunks streaming content in real-time.
//
// Concurrency model (CONST-029):
//   - chunks is a safe.Slice; appends, length queries, and snapshots
//     are atomic without any caller-held lock.
//   - mu (sync.Mutex) is a Pattern Zeta scalar survivor that guards
//     the compound buffer + tokenCount mutations. The buffer /
//     tokenCount pair is updated as a unit by Add/maybeEmitChunk,
//     so the mutex cannot be dropped — but because no bare map or
//     slice sits beside it, the audit is happy.
type StreamingChunker struct {
	mu            sync.Mutex
	config        *ChunkerConfig
	buffer        strings.Builder
	chunks        *safe.Slice[string]
	tokenCount    int
	chunkCallback ChunkCallback
}

// ChunkCallback is called when a new chunk is ready.
type ChunkCallback func(chunk string, index int)

// NewStreamingChunker creates a new streaming chunker.
func NewStreamingChunker(config *ChunkerConfig, callback ChunkCallback) *StreamingChunker {
	if config == nil {
		config = DefaultChunkerConfig()
	}
	return &StreamingChunker{
		config:        config,
		chunks:        safe.NewSlice[string](),
		chunkCallback: callback,
	}
}

// Add adds content to the streaming chunker.
func (c *StreamingChunker) Add(content string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.buffer.WriteString(content)
	c.tokenCount += countTokensSimple(content)

	// Check if we should emit a chunk
	c.maybeEmitChunk()
}

func (c *StreamingChunker) maybeEmitChunk() {
	text := c.buffer.String()

	// Check for natural boundaries
	if c.config.PreserveSentences {
		// Try to emit complete sentences
		lastSentenceEnd := c.findLastSentenceEnd(text)
		if lastSentenceEnd > 0 && c.tokenCount >= c.config.MinTokens {
			chunk := strings.TrimSpace(text[:lastSentenceEnd+1])
			c.emitChunk(chunk)
			remaining := text[lastSentenceEnd+1:]
			c.buffer.Reset()
			c.buffer.WriteString(remaining)
			c.tokenCount = countTokensSimple(remaining)
			return
		}
	}

	if c.config.SplitOnNewlines {
		// Try to emit at newlines
		lastNewline := strings.LastIndex(text, "\n")
		if lastNewline > 0 && countTokensSimple(text[:lastNewline]) >= c.config.MinTokens {
			chunk := strings.TrimSpace(text[:lastNewline])
			c.emitChunk(chunk)
			remaining := text[lastNewline+1:]
			c.buffer.Reset()
			c.buffer.WriteString(remaining)
			c.tokenCount = countTokensSimple(remaining)
			return
		}
	}

	// Force emit if buffer is too large
	if c.tokenCount >= c.config.MaxTokens {
		chunk := strings.TrimSpace(text)
		c.emitChunk(chunk)
		c.buffer.Reset()
		c.tokenCount = 0
	}
}

func (c *StreamingChunker) findLastSentenceEnd(text string) int {
	lastIdx := -1
	for i, r := range text {
		if r == '.' || r == '!' || r == '?' {
			// Check if followed by space or end
			next := i + utf8.RuneLen(r)
			if next >= len(text) || unicode.IsSpace(rune(text[next])) {
				lastIdx = i
			}
		}
	}
	return lastIdx
}

func (c *StreamingChunker) emitChunk(chunk string) {
	if chunk == "" {
		return
	}
	c.chunks.Append(chunk)
	if c.chunkCallback != nil {
		c.chunkCallback(chunk, c.chunks.Len()-1)
	}
}

// Flush flushes any remaining content as a chunk.
func (c *StreamingChunker) Flush() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	remaining := strings.TrimSpace(c.buffer.String())
	if remaining != "" {
		c.emitChunk(remaining)
	}
	c.buffer.Reset()
	c.tokenCount = 0
	return remaining
}

// GetChunks returns all emitted chunks.
func (c *StreamingChunker) GetChunks() []string {
	return c.chunks.Snapshot()
}

// Reset resets the streaming chunker.
func (c *StreamingChunker) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buffer.Reset()
	c.tokenCount = 0
	c.chunks.Clear()
}

// ChunkerChannel creates a channel-based chunking pipeline.
func ChunkerChannel(ctx context.Context, input <-chan string, config *ChunkerConfig) <-chan string {
	output := make(chan string)

	chunker := NewStreamingChunker(config, nil)

	go func() {
		defer close(output)

		for {
			select {
			case <-ctx.Done():
				return
			case text, ok := <-input:
				if !ok {
					// Flush remaining
					if remaining := chunker.Flush(); remaining != "" {
						select {
						case output <- remaining:
						case <-ctx.Done():
							return
						}
					}
					return
				}

				chunker.Add(text)

				// Check if any chunks are ready
				chunks := chunker.GetChunks()
				for i := len(chunks) - 1; i >= 0; i-- {
					select {
					case output <- chunks[i]:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return output
}

// CodeChunker is specialized for chunking source code.
type CodeChunker struct {
	*SmartChunker
	language string
}

// NewCodeChunker creates a new code chunker.
func NewCodeChunker(language string, config *ChunkerConfig) *CodeChunker {
	if config == nil {
		config = &ChunkerConfig{
			Strategy:           StrategySemantic,
			MaxTokens:          1024,
			MinTokens:          64,
			PreserveSentences:  false,
			PreserveParagraphs: false,
			SplitOnNewlines:    true,
		}
	}
	return &CodeChunker{
		SmartChunker: NewSmartChunker(config),
		language:     language,
	}
}

// ChunkCode chunks source code preserving logical boundaries.
func (c *CodeChunker) ChunkCode(code string) []string {
	ctx := &ChunkingContext{
		ContentType:        ContentTypeCode,
		PreserveFormatting: true,
		MaxChunkSize:       c.config.MaxTokens,
	}
	return c.ChunkWithContext(code, ctx)
}

// MarkdownChunker is specialized for chunking Markdown content.
type MarkdownChunker struct {
	*SmartChunker
}

// NewMarkdownChunker creates a new Markdown chunker.
func NewMarkdownChunker(config *ChunkerConfig) *MarkdownChunker {
	if config == nil {
		config = &ChunkerConfig{
			Strategy:           StrategySemantic,
			MaxTokens:          512,
			MinTokens:          64,
			PreserveSentences:  true,
			PreserveParagraphs: true,
			SplitOnNewlines:    true,
		}
	}
	return &MarkdownChunker{
		SmartChunker: NewSmartChunker(config),
	}
}

// ChunkMarkdown chunks Markdown content preserving structure.
func (c *MarkdownChunker) ChunkMarkdown(markdown string) []string {
	ctx := &ChunkingContext{
		ContentType:        ContentTypeMarkdown,
		PreserveFormatting: true,
		MaxChunkSize:       c.config.MaxTokens,
	}
	return c.ChunkWithContext(markdown, ctx)
}
