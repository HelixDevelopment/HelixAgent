package streaming

import (
	"context"
	"time"
)

// StreamConfig holds streaming configuration.
type StreamConfig struct {
	// BufferType determines how content is buffered before emitting.
	BufferType BufferType `yaml:"buffer_type" json:"buffer_type"`

	// ProgressInterval is how often to emit progress updates (0 = disabled).
	ProgressInterval time.Duration `yaml:"progress_interval" json:"progress_interval"`

	// RateLimit is the maximum tokens per second (0 = unlimited).
	RateLimit float64 `yaml:"rate_limit" json:"rate_limit"`

	// EstimatedTokens is the expected total tokens (for progress calculation).
	EstimatedTokens int `yaml:"estimated_tokens" json:"estimated_tokens"`

	// TokenBufferThreshold is the threshold for token-based buffering.
	TokenBufferThreshold int `yaml:"token_buffer_threshold" json:"token_buffer_threshold"`
}

// DefaultStreamConfig returns a default configuration.
func DefaultStreamConfig() *StreamConfig {
	return &StreamConfig{
		BufferType:           BufferTypeWord,
		ProgressInterval:     100 * time.Millisecond,
		RateLimit:            0, // Unlimited
		EstimatedTokens:      0,
		TokenBufferThreshold: 5,
	}
}

// EnhancedStreamer provides enhanced streaming capabilities.
type EnhancedStreamer struct {
	config *StreamConfig
}

// NewEnhancedStreamer creates a new enhanced streamer.
func NewEnhancedStreamer(config *StreamConfig) *EnhancedStreamer {
	if config == nil {
		config = DefaultStreamConfig()
	}
	return &EnhancedStreamer{config: config}
}

// StreamWithProgress streams with progress tracking.
func (e *EnhancedStreamer) StreamWithProgress(
	ctx context.Context,
	stream <-chan *StreamChunk,
	progress ProgressCallback,
) <-chan *StreamChunk {
	out := make(chan *StreamChunk)
	tracker := NewProgressTracker(e.config.EstimatedTokens)
	tracker.Start()

	throttled := NewThrottledCallback(progress, e.config.ProgressInterval)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-stream:
				if !ok {
					// Final progress update
					throttled.ForceCall(tracker.GetProgress())
					return
				}

				// Update progress
				p := tracker.Update(chunk.Content)
				throttled.Call(p)

				// Forward chunk
				select {
				case out <- chunk:
				case <-ctx.Done():
					return
				}

				if chunk.Done {
					throttled.ForceCall(tracker.GetProgress())
					return
				}
			}
		}
	}()

	return out
}

// StreamBuffered applies buffering to a stream.
func (e *EnhancedStreamer) StreamBuffered(
	ctx context.Context,
	stream <-chan *StreamChunk,
) <-chan *StreamChunk {
	buffer := e.createBuffer()
	out := make(chan *StreamChunk)
	index := 0

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-stream:
				if !ok {
					// Flush remaining
					if remaining := buffer.Flush(); remaining != "" {
						select {
						case out <- &StreamChunk{Content: remaining, Index: index}:
							index++
						case <-ctx.Done():
						}
					}
					return
				}

				// Buffer content
				buffered := buffer.Add(chunk.Content)
				for _, b := range buffered {
					select {
					case out <- &StreamChunk{Content: b, Index: index}:
						index++
					case <-ctx.Done():
						return
					}
				}

				// Forward done signal
				if chunk.Done {
					if remaining := buffer.Flush(); remaining != "" {
						select {
						case out <- &StreamChunk{Content: remaining, Index: index}:
							index++
						case <-ctx.Done():
							return
						}
					}
					select {
					case out <- &StreamChunk{Done: true, Index: index}:
					case <-ctx.Done():
					}
					return
				}
			}
		}
	}()

	return out
}

// StreamWithRateLimit applies rate limiting to a stream.
func (e *EnhancedStreamer) StreamWithRateLimit(
	ctx context.Context,
	stream <-chan *StreamChunk,
) <-chan *StreamChunk {
	if e.config.RateLimit <= 0 {
		return stream
	}

	limiter := NewRateLimiter(e.config.RateLimit)
	return limiter.LimitChunks(ctx, stream)
}

// StreamEnhanced applies all enhancements to a stream.
func (e *EnhancedStreamer) StreamEnhanced(
	ctx context.Context,
	stream <-chan *StreamChunk,
	progress ProgressCallback,
) (<-chan *StreamChunk, func() *AggregatedStream) {
	// Apply buffering
	buffered := e.StreamBuffered(ctx, stream)

	// Apply rate limiting
	limited := e.StreamWithRateLimit(ctx, buffered)

	// Apply progress tracking
	var tracked <-chan *StreamChunk
	if progress != nil {
		tracked = e.StreamWithProgress(ctx, limited, progress)
	} else {
		tracked = limited
	}

	// Apply aggregation
	aggregator := NewChunkAggregator()
	aggregated, getResult := aggregator.AggregateChunks(ctx, tracked)

	return aggregated, getResult
}

func (e *EnhancedStreamer) createBuffer() Buffer {
	switch e.config.BufferType {
	case BufferTypeCharacter:
		return NewCharacterBuffer()
	case BufferTypeWord:
		return NewWordBuffer(" ")
	case BufferTypeSentence:
		return NewSentenceBuffer()
	case BufferTypeLine:
		return NewLineBuffer()
	case BufferTypeParagraph:
		return NewParagraphBuffer()
	case BufferTypeToken:
		return NewTokenBuffer(e.config.TokenBufferThreshold)
	default:
		return NewWordBuffer(" ")
	}
}

// Config returns the streamer configuration.
func (e *EnhancedStreamer) Config() *StreamConfig {
	return e.config
}

// SetConfig updates the streamer configuration.
func (e *EnhancedStreamer) SetConfig(config *StreamConfig) {
	if config != nil {
		e.config = config
	}
}

// StringToChunkChannel converts a string channel to StreamChunk channel.
func StringToChunkChannel(ctx context.Context, in <-chan string) <-chan *StreamChunk {
	out := make(chan *StreamChunk)

	go func() {
		defer close(out)
		index := 0

		for {
			select {
			case <-ctx.Done():
				return
			case s, ok := <-in:
				if !ok {
					select {
					case out <- &StreamChunk{Done: true, Index: index}:
					case <-ctx.Done():
					}
					return
				}

				select {
				case out <- &StreamChunk{Content: s, Index: index}:
					index++
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out
}

// ChunkToStringChannel converts a StreamChunk channel to string channel.
func ChunkToStringChannel(ctx context.Context, in <-chan *StreamChunk) <-chan string {
	out := make(chan string)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-in:
				if !ok || chunk.Done {
					return
				}

				select {
				case out <- chunk.Content:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out
}
