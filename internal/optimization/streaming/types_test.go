package streaming

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStreamStates(t *testing.T) {
	t.Parallel()
	assert.Equal(t, StreamState("pending"), StreamStatePending)
	assert.Equal(t, StreamState("active"), StreamStateActive)
	assert.Equal(t, StreamState("paused"), StreamStatePaused)
	assert.Equal(t, StreamState("completed"), StreamStateCompleted)
	assert.Equal(t, StreamState("failed"), StreamStateFailed)
	assert.Equal(t, StreamState("cancelled"), StreamStateCancelled)
}

func TestStreamMetadata(t *testing.T) {
	t.Parallel()
	metadata := &StreamMetadata{
		ID:              "stream-123",
		CreatedAt:       time.Now(),
		StartedAt:       time.Now(),
		State:           StreamStateActive,
		Model:           "gpt-4",
		Provider:        "openai",
		EstimatedTokens: 1000,
		Metadata:        map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, "stream-123", metadata.ID)
	assert.Equal(t, StreamStateActive, metadata.State)
	assert.Equal(t, "gpt-4", metadata.Model)
	assert.Equal(t, 1000, metadata.EstimatedTokens)
}

func TestStreamEventTypes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, StreamEventType("start"), EventTypeStart)
	assert.Equal(t, StreamEventType("chunk"), EventTypeChunk)
	assert.Equal(t, StreamEventType("progress"), EventTypeProgress)
	assert.Equal(t, StreamEventType("complete"), EventTypeComplete)
	assert.Equal(t, StreamEventType("error"), EventTypeError)
	assert.Equal(t, StreamEventType("pause"), EventTypePause)
	assert.Equal(t, StreamEventType("resume"), EventTypeResume)
	assert.Equal(t, StreamEventType("cancel"), EventTypeCancel)
}

func TestStreamEvent(t *testing.T) {
	t.Parallel()
	event := &StreamEvent{
		Type:      EventTypeChunk,
		Timestamp: time.Now(),
		StreamID:  "stream-123",
		Data:      "chunk data",
	}

	assert.Equal(t, EventTypeChunk, event.Type)
	assert.Equal(t, "stream-123", event.StreamID)
	assert.NotNil(t, event.Data)
}

func TestDefaultStreamOptions(t *testing.T) {
	t.Parallel()
	options := DefaultStreamOptions()

	assert.Equal(t, BufferTypeWord, options.BufferType)
	assert.True(t, options.EnableProgress)
	assert.Equal(t, 100*time.Millisecond, options.ProgressInterval)
	assert.True(t, options.EnableMetrics)
	assert.Equal(t, 5*time.Minute, options.Timeout)
	assert.False(t, options.RetryOnError)
	assert.Equal(t, 3, options.MaxRetries)
}

func TestStreamMetrics(t *testing.T) {
	t.Parallel()
	metrics := &StreamMetrics{
		TotalChunks:           100,
		TotalTokens:           500,
		TotalCharacters:       2000,
		BytesReceived:         2100,
		DurationMs:            1000,
		FirstChunkLatencyMs:   50,
		AverageChunkLatencyMs: 10.5,
	}

	metrics.CalculateThroughput()

	assert.Equal(t, int64(100), metrics.TotalChunks)
	assert.Equal(t, int64(500), metrics.TotalTokens)
	assert.Equal(t, float64(500), metrics.TokensPerSecond)
	assert.Equal(t, float64(100), metrics.ChunksPerSecond)
}

func TestStreamMetrics_ZeroDuration(t *testing.T) {
	t.Parallel()
	metrics := &StreamMetrics{
		TotalTokens: 100,
		DurationMs:  0,
	}

	metrics.CalculateThroughput()

	assert.Equal(t, float64(0), metrics.TokensPerSecond)
}

func TestChunkInfo(t *testing.T) {
	t.Parallel()
	chunk := &ChunkInfo{
		Index:          0,
		Content:        "Hello world",
		TokenCount:     2,
		CharacterCount: 11,
		ByteCount:      11,
		Timestamp:      time.Now(),
		LatencyMs:      15,
	}

	assert.Equal(t, 0, chunk.Index)
	assert.Equal(t, "Hello world", chunk.Content)
	assert.Equal(t, 2, chunk.TokenCount)
}

func TestStreamResult(t *testing.T) {
	t.Parallel()
	result := &StreamResult{
		Success:     true,
		FullContent: "Hello world",
		Chunks: []ChunkInfo{
			{Index: 0, Content: "Hello "},
			{Index: 1, Content: "world"},
		},
		Metrics: &StreamMetrics{TotalChunks: 2},
	}

	assert.True(t, result.Success)
	assert.Equal(t, "Hello world", result.FullContent)
	assert.Len(t, result.Chunks, 2)
}

func TestBaseStreamHandler(t *testing.T) {
	t.Parallel()
	handler := &BaseStreamHandler{}

	// These should not panic
	handler.OnStart(&StreamMetadata{})
	handler.OnChunk(&ChunkInfo{})
	handler.OnProgress(&StreamProgress{})
	handler.OnComplete(&StreamResult{})
	handler.OnError(nil)
}

func TestIdentityTransformer(t *testing.T) {
	t.Parallel()
	transformer := &IdentityTransformer{}

	assert.Equal(t, "test", transformer.Transform("test"))

	chunk := &StreamChunk{Content: "hello"}
	result := transformer.TransformChunk(chunk)
	assert.Equal(t, "hello", result.Content)
}

func TestTrimTransformer(t *testing.T) {
	t.Parallel()
	transformer := &TrimTransformer{}

	// TrimTransformer doesn't trim individual chunks
	result := transformer.Transform("  hello  ")
	assert.Equal(t, "  hello  ", result)

	chunk := &StreamChunk{Content: "  world  "}
	resultChunk := transformer.TransformChunk(chunk)
	assert.Equal(t, "  world  ", resultChunk.Content)
}

func TestFilterTransformer(t *testing.T) {
	t.Parallel()
	transformer := &FilterTransformer{
		FilterFunc: func(s string) bool {
			return s == "filter_me"
		},
	}

	assert.Equal(t, "keep", transformer.Transform("keep"))
	assert.Equal(t, "", transformer.Transform("filter_me"))

	chunk := &StreamChunk{Content: "filter_me"}
	result := transformer.TransformChunk(chunk)
	assert.Equal(t, "", result.Content)
}

func TestFilterTransformer_NilFunc(t *testing.T) {
	t.Parallel()
	transformer := &FilterTransformer{}

	assert.Equal(t, "test", transformer.Transform("test"))
}

func TestStreamPipeline(t *testing.T) {
	t.Parallel()
	pipeline := NewStreamPipeline(
		&IdentityTransformer{},
		&FilterTransformer{
			FilterFunc: func(s string) bool {
				return s == "filter"
			},
		},
	)

	assert.Equal(t, "keep", pipeline.Transform("keep"))
	assert.Equal(t, "", pipeline.Transform("filter"))

	chunk := &StreamChunk{Content: "test"}
	result := pipeline.TransformChunk(chunk)
	assert.Equal(t, "test", result.Content)
}

func TestBackpressureStrategies(t *testing.T) {
	t.Parallel()
	assert.Equal(t, BackpressureStrategy("block"), BackpressureBlock)
	assert.Equal(t, BackpressureStrategy("buffer"), BackpressureBuffer)
	assert.Equal(t, BackpressureStrategy("drop"), BackpressureDrop)
	assert.Equal(t, BackpressureStrategy("sample"), BackpressureSample)
}

func TestDefaultBackpressureConfig(t *testing.T) {
	t.Parallel()
	config := DefaultBackpressureConfig()

	assert.Equal(t, BackpressureBlock, config.Strategy)
	assert.Equal(t, 100, config.BufferSize)
	assert.Equal(t, 0.5, config.SampleRate)
}

func TestStreamError(t *testing.T) {
	t.Parallel()
	err := &StreamError{
		Code:        ErrorCodeTimeout,
		Message:     "Operation timed out",
		Recoverable: true,
		ChunkIndex:  5,
		Timestamp:   time.Now(),
	}

	assert.Equal(t, ErrorCodeTimeout, err.Code)
	assert.Equal(t, "Operation timed out", err.Error())
	assert.True(t, err.Recoverable)
	assert.Equal(t, 5, err.ChunkIndex)
}

func TestStreamErrorCodes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, StreamErrorCode("timeout"), ErrorCodeTimeout)
	assert.Equal(t, StreamErrorCode("cancelled"), ErrorCodeCancelled)
	assert.Equal(t, StreamErrorCode("connection"), ErrorCodeConnection)
	assert.Equal(t, StreamErrorCode("rate_limit"), ErrorCodeRateLimit)
	assert.Equal(t, StreamErrorCode("backend"), ErrorCodeBackend)
	assert.Equal(t, StreamErrorCode("invalid"), ErrorCodeInvalid)
}

func TestDefaultReconnectionStrategy(t *testing.T) {
	t.Parallel()
	strategy := DefaultReconnectionStrategy()

	assert.True(t, strategy.Enabled)
	assert.Equal(t, 3, strategy.MaxAttempts)
	assert.Equal(t, 100*time.Millisecond, strategy.InitialDelay)
	assert.Equal(t, 5*time.Second, strategy.MaxDelay)
	assert.Equal(t, 2.0, strategy.BackoffMultiplier)
}

func TestDefaultStreamCapabilities(t *testing.T) {
	t.Parallel()
	caps := DefaultStreamCapabilities()

	assert.True(t, caps.SupportsProgress)
	assert.True(t, caps.SupportsRateLimit)
	assert.True(t, caps.SupportsBuffering)
	assert.False(t, caps.SupportsPause)
	assert.True(t, caps.SupportsReconnect)
	assert.Equal(t, 10, caps.MaxConcurrentStreams)
	assert.Len(t, caps.SupportedBufferTypes, 6)
}

func TestStreamOptions(t *testing.T) {
	t.Parallel()
	options := &StreamOptions{
		BufferType:               BufferTypeSentence,
		BufferSize:               1024,
		RateLimitTokensPerSecond: 100,
		EnableProgress:           true,
		ProgressInterval:         50 * time.Millisecond,
		EnableMetrics:            true,
		Timeout:                  10 * time.Minute,
		RetryOnError:             true,
		MaxRetries:               5,
	}

	assert.Equal(t, BufferTypeSentence, options.BufferType)
	assert.Equal(t, 1024, options.BufferSize)
	assert.Equal(t, float64(100), options.RateLimitTokensPerSecond)
	assert.Equal(t, 5, options.MaxRetries)
}

func TestStreamResult_Failed(t *testing.T) {
	t.Parallel()
	result := &StreamResult{
		Success: false,
		Error:   "Connection lost",
	}

	assert.False(t, result.Success)
	assert.Equal(t, "Connection lost", result.Error)
}

func TestReconnectionStrategy(t *testing.T) {
	t.Parallel()
	strategy := &ReconnectionStrategy{
		Enabled:           true,
		MaxAttempts:       5,
		InitialDelay:      200 * time.Millisecond,
		MaxDelay:          10 * time.Second,
		BackoffMultiplier: 1.5,
	}

	assert.True(t, strategy.Enabled)
	assert.Equal(t, 5, strategy.MaxAttempts)
	assert.Equal(t, 200*time.Millisecond, strategy.InitialDelay)
	assert.Equal(t, 1.5, strategy.BackoffMultiplier)
}

func TestStreamCapabilities(t *testing.T) {
	t.Parallel()
	caps := &StreamCapabilities{
		SupportsProgress:     true,
		SupportsRateLimit:    true,
		SupportsBuffering:    true,
		SupportsPause:        true,
		SupportsReconnect:    true,
		MaxConcurrentStreams: 20,
		SupportedBufferTypes: []BufferType{BufferTypeWord, BufferTypeSentence},
	}

	assert.True(t, caps.SupportsPause)
	assert.Equal(t, 20, caps.MaxConcurrentStreams)
	assert.Len(t, caps.SupportedBufferTypes, 2)
}
