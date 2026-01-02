package streaming

import (
	"context"
	"strings"
	"sync"
	"time"
)

// AggregatedStream contains the aggregated result of a stream.
type AggregatedStream struct {
	FullContent     string   `json:"full_content"`
	Chunks          []string `json:"chunks"`
	TokenCount      int      `json:"token_count"`
	CharacterCount  int      `json:"character_count"`
	ChunkCount      int      `json:"chunk_count"`
	DurationSeconds float64  `json:"duration_seconds"`
	TokensPerSecond float64  `json:"tokens_per_second"`
	CharsPerSecond  float64  `json:"chars_per_second"`
}

// StreamAggregator aggregates streaming output while passing through.
type StreamAggregator struct {
	mu        sync.Mutex
	chunks    []string
	startTime time.Time
}

// NewStreamAggregator creates a new stream aggregator.
func NewStreamAggregator() *StreamAggregator {
	return &StreamAggregator{}
}

// Start begins aggregation.
func (a *StreamAggregator) Start() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.startTime = time.Now()
	a.chunks = nil
}

// Add adds a chunk to the aggregation.
func (a *StreamAggregator) Add(chunk string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.chunks = append(a.chunks, chunk)
}

// GetResult returns the aggregated result.
func (a *StreamAggregator) GetResult() *AggregatedStream {
	a.mu.Lock()
	defer a.mu.Unlock()

	duration := time.Since(a.startTime).Seconds()
	fullContent := strings.Join(a.chunks, "")
	tokenCount := len(strings.Fields(fullContent))
	charCount := len(fullContent)

	var tps, cps float64
	if duration > 0 {
		tps = float64(tokenCount) / duration
		cps = float64(charCount) / duration
	}

	return &AggregatedStream{
		FullContent:     fullContent,
		Chunks:          a.chunks,
		TokenCount:      tokenCount,
		CharacterCount:  charCount,
		ChunkCount:      len(a.chunks),
		DurationSeconds: duration,
		TokensPerSecond: tps,
		CharsPerSecond:  cps,
	}
}

// Reset clears the aggregator.
func (a *StreamAggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.chunks = nil
	a.startTime = time.Time{}
}

// Aggregate wraps a channel to aggregate content while passing through.
func (a *StreamAggregator) Aggregate(ctx context.Context, in <-chan string) (<-chan string, func() *AggregatedStream) {
	a.Start()
	out := make(chan string)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-in:
				if !ok {
					return
				}

				a.Add(chunk)

				select {
				case out <- chunk:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, a.GetResult
}

// StreamChunk represents a chunk in a stream.
type StreamChunk struct {
	Content  string `json:"content"`
	Index    int    `json:"index"`
	Done     bool   `json:"done"`
	Error    error  `json:"-"`
}

// ChunkAggregator aggregates StreamChunk objects.
type ChunkAggregator struct {
	mu        sync.Mutex
	chunks    []*StreamChunk
	startTime time.Time
}

// NewChunkAggregator creates a new chunk aggregator.
func NewChunkAggregator() *ChunkAggregator {
	return &ChunkAggregator{}
}

// Start begins aggregation.
func (a *ChunkAggregator) Start() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.startTime = time.Now()
	a.chunks = nil
}

// Add adds a chunk to the aggregation.
func (a *ChunkAggregator) Add(chunk *StreamChunk) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.chunks = append(a.chunks, chunk)
}

// GetResult returns the aggregated result.
func (a *ChunkAggregator) GetResult() *AggregatedStream {
	a.mu.Lock()
	defer a.mu.Unlock()

	duration := time.Since(a.startTime).Seconds()

	var builder strings.Builder
	chunkStrings := make([]string, 0, len(a.chunks))

	for _, chunk := range a.chunks {
		builder.WriteString(chunk.Content)
		chunkStrings = append(chunkStrings, chunk.Content)
	}

	fullContent := builder.String()
	tokenCount := len(strings.Fields(fullContent))
	charCount := len(fullContent)

	var tps, cps float64
	if duration > 0 {
		tps = float64(tokenCount) / duration
		cps = float64(charCount) / duration
	}

	return &AggregatedStream{
		FullContent:     fullContent,
		Chunks:          chunkStrings,
		TokenCount:      tokenCount,
		CharacterCount:  charCount,
		ChunkCount:      len(a.chunks),
		DurationSeconds: duration,
		TokensPerSecond: tps,
		CharsPerSecond:  cps,
	}
}

// AggregateChunks wraps a StreamChunk channel to aggregate content.
func (a *ChunkAggregator) AggregateChunks(ctx context.Context, in <-chan *StreamChunk) (<-chan *StreamChunk, func() *AggregatedStream) {
	a.Start()
	out := make(chan *StreamChunk)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-in:
				if !ok {
					return
				}

				a.Add(chunk)

				select {
				case out <- chunk:
				case <-ctx.Done():
					return
				}

				if chunk.Done {
					return
				}
			}
		}
	}()

	return out, a.GetResult
}
