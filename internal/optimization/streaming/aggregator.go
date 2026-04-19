package streaming

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
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
	chunks    *safe.Slice[string]
	startTime atomic.Pointer[time.Time]
}

// NewStreamAggregator creates a new stream aggregator.
func NewStreamAggregator() *StreamAggregator {
	return &StreamAggregator{
		chunks: safe.NewSlice[string](),
	}
}

// Start begins aggregation.
func (a *StreamAggregator) Start() {
	now := time.Now()
	a.startTime.Store(&now)
	a.chunks.Clear()
}

// Add adds a chunk to the aggregation.
func (a *StreamAggregator) Add(chunk string) {
	a.chunks.Append(chunk)
}

// GetResult returns the aggregated result.
func (a *StreamAggregator) GetResult() *AggregatedStream {
	var duration float64
	if st := a.startTime.Load(); st != nil && !st.IsZero() {
		duration = time.Since(*st).Seconds()
	}

	chunks := a.chunks.Snapshot()
	fullContent := strings.Join(chunks, "")
	tokenCount := len(strings.Fields(fullContent))
	charCount := len(fullContent)

	var tps, cps float64
	if duration > 0 {
		tps = float64(tokenCount) / duration
		cps = float64(charCount) / duration
	}

	return &AggregatedStream{
		FullContent:     fullContent,
		Chunks:          chunks,
		TokenCount:      tokenCount,
		CharacterCount:  charCount,
		ChunkCount:      len(chunks),
		DurationSeconds: duration,
		TokensPerSecond: tps,
		CharsPerSecond:  cps,
	}
}

// Reset clears the aggregator.
func (a *StreamAggregator) Reset() {
	a.chunks.Clear()
	var zero time.Time
	a.startTime.Store(&zero)
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
	Content string `json:"content"`
	Index   int    `json:"index"`
	Done    bool   `json:"done"`
	Error   error  `json:"-"`
}

// ChunkAggregator aggregates StreamChunk objects.
type ChunkAggregator struct {
	chunks    *safe.Slice[*StreamChunk]
	startTime atomic.Pointer[time.Time]
}

// NewChunkAggregator creates a new chunk aggregator.
func NewChunkAggregator() *ChunkAggregator {
	return &ChunkAggregator{
		chunks: safe.NewSlice[*StreamChunk](),
	}
}

// Start begins aggregation.
func (a *ChunkAggregator) Start() {
	now := time.Now()
	a.startTime.Store(&now)
	a.chunks.Clear()
}

// Add adds a chunk to the aggregation.
func (a *ChunkAggregator) Add(chunk *StreamChunk) {
	a.chunks.Append(chunk)
}

// GetResult returns the aggregated result.
func (a *ChunkAggregator) GetResult() *AggregatedStream {
	var duration float64
	if st := a.startTime.Load(); st != nil && !st.IsZero() {
		duration = time.Since(*st).Seconds()
	}

	chunks := a.chunks.Snapshot()

	var builder strings.Builder
	chunkStrings := make([]string, 0, len(chunks))

	for _, chunk := range chunks {
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
		ChunkCount:      len(chunks),
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
