# llm-streaming - Deep Analysis Report

## Repository Overview

- **Repository**: https://github.com/SaihanTaki/llm-streaming
- **Language**: Python
- **Purpose**: Tools and examples for implementing real-time token streaming
- **License**: MIT

## Core Architecture

### Directory Structure

```
llm-streaming/
├── examples/
│   ├── fastapi_streaming.py    # FastAPI SSE example
│   ├── langchain_streaming.py  # LangChain integration
│   └── transformers_streaming.py  # Hugging Face integration
├── streaming/
│   ├── sse.py                  # Server-Sent Events handler
│   ├── buffer.py               # Token buffering strategies
│   └── handlers.py             # Stream processing handlers
└── utils/
    └── helpers.py              # Utility functions
```

### Key Components

#### 1. SSE Handler (`streaming/sse.py`)

**Core SSE Implementation**

```python
from fastapi import Response
from fastapi.responses import StreamingResponse
import asyncio

class SSEResponse(StreamingResponse):
    """Server-Sent Events response for streaming."""

    media_type = "text/event-stream"

    def __init__(self, generator, **kwargs):
        super().__init__(
            content=self._wrap_generator(generator),
            media_type=self.media_type,
            headers={
                "Cache-Control": "no-cache",
                "Connection": "keep-alive",
                "X-Accel-Buffering": "no",
            },
            **kwargs
        )

    async def _wrap_generator(self, generator):
        """Wrap generator with SSE format."""
        async for data in generator:
            yield f"data: {data}\n\n"
        yield "data: [DONE]\n\n"


def format_sse_event(data: str, event: str = None, id: str = None) -> str:
    """Format data as SSE event."""
    lines = []
    if id:
        lines.append(f"id: {id}")
    if event:
        lines.append(f"event: {event}")
    lines.append(f"data: {data}")
    return "\n".join(lines) + "\n\n"
```

#### 2. Token Buffering (`streaming/buffer.py`)

**Character Buffer**

```python
class CharacterBuffer:
    """Buffer that flushes character by character."""

    def __init__(self):
        self.buffer = ""

    def add(self, text: str) -> Generator[str, None, None]:
        for char in text:
            yield char


class WordBuffer:
    """Buffer that flushes complete words."""

    def __init__(self, delimiter: str = " "):
        self.buffer = ""
        self.delimiter = delimiter

    def add(self, text: str) -> Generator[str, None, None]:
        self.buffer += text

        while self.delimiter in self.buffer:
            word, self.buffer = self.buffer.split(self.delimiter, 1)
            yield word + self.delimiter

    def flush(self) -> str:
        """Flush remaining content."""
        remaining = self.buffer
        self.buffer = ""
        return remaining


class SentenceBuffer:
    """Buffer that flushes complete sentences."""

    SENTENCE_ENDINGS = {'.', '!', '?'}

    def __init__(self):
        self.buffer = ""

    def add(self, text: str) -> Generator[str, None, None]:
        self.buffer += text

        for ending in self.SENTENCE_ENDINGS:
            while ending in self.buffer:
                idx = self.buffer.index(ending)
                sentence = self.buffer[:idx + 1]
                self.buffer = self.buffer[idx + 1:].lstrip()
                yield sentence

    def flush(self) -> str:
        remaining = self.buffer
        self.buffer = ""
        return remaining


class LineBuffer:
    """Buffer that flushes complete lines."""

    def __init__(self):
        self.buffer = ""

    def add(self, text: str) -> Generator[str, None, None]:
        self.buffer += text

        while "\n" in self.buffer:
            line, self.buffer = self.buffer.split("\n", 1)
            yield line + "\n"

    def flush(self) -> str:
        remaining = self.buffer
        self.buffer = ""
        return remaining
```

#### 3. Stream Processing (`streaming/handlers.py`)

**Progress Tracking**

```python
import time
from dataclasses import dataclass
from typing import Callable, AsyncIterator

@dataclass
class StreamProgress:
    """Progress information for streaming."""
    tokens_generated: int
    elapsed_seconds: float
    tokens_per_second: float
    estimated_remaining: float
    percent_complete: float


class ProgressTracker:
    """Track streaming progress."""

    def __init__(self, estimated_tokens: int = None):
        self.estimated_tokens = estimated_tokens
        self.tokens_generated = 0
        self.start_time = None

    def start(self):
        self.start_time = time.time()
        self.tokens_generated = 0

    def update(self, tokens: int = 1) -> StreamProgress:
        self.tokens_generated += tokens
        elapsed = time.time() - self.start_time
        tps = self.tokens_generated / elapsed if elapsed > 0 else 0

        if self.estimated_tokens:
            remaining = (self.estimated_tokens - self.tokens_generated) / tps if tps > 0 else 0
            percent = (self.tokens_generated / self.estimated_tokens) * 100
        else:
            remaining = 0
            percent = 0

        return StreamProgress(
            tokens_generated=self.tokens_generated,
            elapsed_seconds=elapsed,
            tokens_per_second=tps,
            estimated_remaining=remaining,
            percent_complete=percent
        )


async def stream_with_progress(
    stream: AsyncIterator[str],
    progress_callback: Callable[[StreamProgress], None],
    progress_interval: float = 0.1
) -> AsyncIterator[str]:
    """Wrap stream with progress tracking."""
    tracker = ProgressTracker()
    tracker.start()

    last_progress_time = time.time()

    async for chunk in stream:
        progress = tracker.update(len(chunk.split()))

        # Emit progress at interval
        if time.time() - last_progress_time >= progress_interval:
            progress_callback(progress)
            last_progress_time = time.time()

        yield chunk

    # Final progress
    progress_callback(tracker.update(0))
```

#### 4. Rate Limiting

```python
import asyncio

class RateLimitedStream:
    """Rate limit token output."""

    def __init__(self, tokens_per_second: float):
        self.delay = 1.0 / tokens_per_second
        self.last_emit = 0

    async def limit(self, stream: AsyncIterator[str]) -> AsyncIterator[str]:
        async for token in stream:
            now = time.time()
            elapsed = now - self.last_emit

            if elapsed < self.delay:
                await asyncio.sleep(self.delay - elapsed)

            self.last_emit = time.time()
            yield token
```

#### 5. Stream Aggregation

```python
@dataclass
class AggregatedStream:
    """Aggregated stream result."""
    full_content: str
    chunks: List[str]
    token_count: int
    duration_seconds: float
    tokens_per_second: float


class StreamAggregator:
    """Aggregate streaming output while passing through."""

    def __init__(self):
        self.chunks = []
        self.start_time = None

    async def aggregate(
        self,
        stream: AsyncIterator[str]
    ) -> Tuple[AsyncIterator[str], Callable[[], AggregatedStream]]:
        """Wrap stream to aggregate while passing through."""
        self.start_time = time.time()
        self.chunks = []

        async def wrapped():
            async for chunk in stream:
                self.chunks.append(chunk)
                yield chunk

        def get_result() -> AggregatedStream:
            duration = time.time() - self.start_time
            full_content = "".join(self.chunks)
            token_count = len(full_content.split())

            return AggregatedStream(
                full_content=full_content,
                chunks=self.chunks,
                token_count=token_count,
                duration_seconds=duration,
                tokens_per_second=token_count / duration if duration > 0 else 0
            )

        return wrapped(), get_result
```

## Go Port Strategy

### Core Components to Implement

```go
// internal/optimization/streaming/buffer.go

package streaming

import (
    "strings"
    "unicode"
)

// Buffer defines the buffer interface
type Buffer interface {
    Add(text string) []string
    Flush() string
}

// CharacterBuffer emits character by character
type CharacterBuffer struct{}

func NewCharacterBuffer() *CharacterBuffer {
    return &CharacterBuffer{}
}

func (b *CharacterBuffer) Add(text string) []string {
    result := make([]string, 0, len(text))
    for _, r := range text {
        result = append(result, string(r))
    }
    return result
}

func (b *CharacterBuffer) Flush() string {
    return ""
}

// WordBuffer emits complete words
type WordBuffer struct {
    buffer    strings.Builder
    delimiter string
}

func NewWordBuffer(delimiter string) *WordBuffer {
    if delimiter == "" {
        delimiter = " "
    }
    return &WordBuffer{delimiter: delimiter}
}

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

func (b *WordBuffer) Flush() string {
    remaining := b.buffer.String()
    b.buffer.Reset()
    return remaining
}

// SentenceBuffer emits complete sentences
type SentenceBuffer struct {
    buffer strings.Builder
}

func NewSentenceBuffer() *SentenceBuffer {
    return &SentenceBuffer{}
}

var sentenceEndings = map[rune]bool{'.': true, '!': true, '?': true}

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
            return i
        }
    }
    return -1
}

func (b *SentenceBuffer) Flush() string {
    remaining := b.buffer.String()
    b.buffer.Reset()
    return remaining
}

// LineBuffer emits complete lines
type LineBuffer struct {
    buffer strings.Builder
}

func NewLineBuffer() *LineBuffer {
    return &LineBuffer{}
}

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

func (b *LineBuffer) Flush() string {
    remaining := b.buffer.String()
    b.buffer.Reset()
    return remaining
}
```

### Progress Tracking

```go
// internal/optimization/streaming/progress.go

package streaming

import (
    "sync"
    "time"
)

// StreamProgress contains progress information
type StreamProgress struct {
    TokensGenerated    int
    ElapsedSeconds     float64
    TokensPerSecond    float64
    EstimatedRemaining float64
    PercentComplete    float64
}

// ProgressTracker tracks streaming progress
type ProgressTracker struct {
    mu              sync.Mutex
    estimatedTokens int
    tokensGenerated int
    startTime       time.Time
}

// NewProgressTracker creates a new tracker
func NewProgressTracker(estimatedTokens int) *ProgressTracker {
    return &ProgressTracker{
        estimatedTokens: estimatedTokens,
    }
}

// Start begins tracking
func (t *ProgressTracker) Start() {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.startTime = time.Now()
    t.tokensGenerated = 0
}

// Update updates progress and returns current state
func (t *ProgressTracker) Update(tokens int) *StreamProgress {
    t.mu.Lock()
    defer t.mu.Unlock()

    t.tokensGenerated += tokens
    elapsed := time.Since(t.startTime).Seconds()

    var tps float64
    if elapsed > 0 {
        tps = float64(t.tokensGenerated) / elapsed
    }

    var remaining, percent float64
    if t.estimatedTokens > 0 {
        if tps > 0 {
            remaining = float64(t.estimatedTokens-t.tokensGenerated) / tps
        }
        percent = (float64(t.tokensGenerated) / float64(t.estimatedTokens)) * 100
    }

    return &StreamProgress{
        TokensGenerated:    t.tokensGenerated,
        ElapsedSeconds:     elapsed,
        TokensPerSecond:    tps,
        EstimatedRemaining: remaining,
        PercentComplete:    percent,
    }
}

// ProgressCallback is called with progress updates
type ProgressCallback func(*StreamProgress)
```

### Rate Limiter

```go
// internal/optimization/streaming/rate_limiter.go

package streaming

import (
    "context"
    "time"
)

// RateLimiter limits token output rate
type RateLimiter struct {
    tokensPerSecond float64
    delay           time.Duration
    lastEmit        time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(tokensPerSecond float64) *RateLimiter {
    return &RateLimiter{
        tokensPerSecond: tokensPerSecond,
        delay:           time.Duration(float64(time.Second) / tokensPerSecond),
    }
}

// Limit rate limits a stream channel
func (r *RateLimiter) Limit(ctx context.Context, in <-chan string) <-chan string {
    out := make(chan string)

    go func() {
        defer close(out)

        for {
            select {
            case <-ctx.Done():
                return
            case token, ok := <-in:
                if !ok {
                    return
                }

                elapsed := time.Since(r.lastEmit)
                if elapsed < r.delay {
                    select {
                    case <-ctx.Done():
                        return
                    case <-time.After(r.delay - elapsed):
                    }
                }

                r.lastEmit = time.Now()

                select {
                case out <- token:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()

    return out
}
```

### Stream Aggregator

```go
// internal/optimization/streaming/aggregator.go

package streaming

import (
    "context"
    "strings"
    "sync"
    "time"
)

// AggregatedStream contains aggregated result
type AggregatedStream struct {
    FullContent     string
    Chunks          []string
    TokenCount      int
    DurationSeconds float64
    TokensPerSecond float64
}

// StreamAggregator aggregates while passing through
type StreamAggregator struct {
    mu        sync.Mutex
    chunks    []string
    startTime time.Time
}

// NewStreamAggregator creates a new aggregator
func NewStreamAggregator() *StreamAggregator {
    return &StreamAggregator{}
}

// Aggregate wraps a stream to aggregate content
func (a *StreamAggregator) Aggregate(ctx context.Context, in <-chan string) (<-chan string, func() *AggregatedStream) {
    a.mu.Lock()
    a.startTime = time.Now()
    a.chunks = nil
    a.mu.Unlock()

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

                a.mu.Lock()
                a.chunks = append(a.chunks, chunk)
                a.mu.Unlock()

                select {
                case out <- chunk:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()

    getResult := func() *AggregatedStream {
        a.mu.Lock()
        defer a.mu.Unlock()

        duration := time.Since(a.startTime).Seconds()
        fullContent := strings.Join(a.chunks, "")
        tokenCount := len(strings.Fields(fullContent))

        var tps float64
        if duration > 0 {
            tps = float64(tokenCount) / duration
        }

        return &AggregatedStream{
            FullContent:     fullContent,
            Chunks:          a.chunks,
            TokenCount:      tokenCount,
            DurationSeconds: duration,
            TokensPerSecond: tps,
        }
    }

    return out, getResult
}
```

### SSE Handler

```go
// internal/optimization/streaming/sse_handler.go

package streaming

import (
    "context"
    "fmt"
    "net/http"
)

// SSEWriter writes Server-Sent Events
type SSEWriter struct {
    w       http.ResponseWriter
    flusher http.Flusher
}

// NewSSEWriter creates a new SSE writer
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        return nil, fmt.Errorf("streaming not supported")
    }

    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")

    return &SSEWriter{w: w, flusher: flusher}, nil
}

// WriteEvent writes an SSE event
func (s *SSEWriter) WriteEvent(event, data string) error {
    if event != "" {
        if _, err := fmt.Fprintf(s.w, "event: %s\n", event); err != nil {
            return err
        }
    }
    if _, err := fmt.Fprintf(s.w, "data: %s\n\n", data); err != nil {
        return err
    }
    s.flusher.Flush()
    return nil
}

// WriteData writes data without event type
func (s *SSEWriter) WriteData(data string) error {
    return s.WriteEvent("", data)
}

// WriteDone writes the done event
func (s *SSEWriter) WriteDone() error {
    return s.WriteData("[DONE]")
}

// StreamToSSE streams a channel to SSE
func StreamToSSE(ctx context.Context, w http.ResponseWriter, stream <-chan string) error {
    sse, err := NewSSEWriter(w)
    if err != nil {
        return err
    }

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case chunk, ok := <-stream:
            if !ok {
                return sse.WriteDone()
            }
            if err := sse.WriteData(chunk); err != nil {
                return err
            }
        }
    }
}
```

### Enhanced Streamer Service

```go
// internal/optimization/streaming/enhanced_streamer.go

package streaming

import (
    "context"

    "superagent/internal/models"
)

// BufferType defines the buffering strategy
type BufferType string

const (
    BufferTypeCharacter BufferType = "character"
    BufferTypeWord      BufferType = "word"
    BufferTypeSentence  BufferType = "sentence"
    BufferTypeLine      BufferType = "line"
)

// EnhancedStreamer provides enhanced streaming capabilities
type EnhancedStreamer struct {
    config *StreamConfig
}

// StreamConfig holds streaming configuration
type StreamConfig struct {
    BufferType       BufferType
    ProgressInterval time.Duration
    RateLimit        float64 // tokens per second, 0 = unlimited
}

// NewEnhancedStreamer creates a new enhanced streamer
func NewEnhancedStreamer(config *StreamConfig) *EnhancedStreamer {
    return &EnhancedStreamer{config: config}
}

// StreamWithProgress streams with progress tracking
func (e *EnhancedStreamer) StreamWithProgress(
    ctx context.Context,
    stream <-chan *models.StreamChunk,
    progress ProgressCallback,
) <-chan *models.StreamChunk {
    out := make(chan *models.StreamChunk)
    tracker := NewProgressTracker(0)
    tracker.Start()

    go func() {
        defer close(out)

        ticker := time.NewTicker(e.config.ProgressInterval)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                progress(tracker.Update(0))
            case chunk, ok := <-stream:
                if !ok {
                    progress(tracker.Update(0))
                    return
                }
                tracker.Update(len(strings.Fields(chunk.Content)))

                select {
                case out <- chunk:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()

    return out
}

// StreamBuffered applies buffering to a stream
func (e *EnhancedStreamer) StreamBuffered(
    ctx context.Context,
    stream <-chan *models.StreamChunk,
) <-chan *models.StreamChunk {
    buffer := e.createBuffer()
    out := make(chan *models.StreamChunk)

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
                        case out <- &models.StreamChunk{Content: remaining}:
                        case <-ctx.Done():
                        }
                    }
                    return
                }

                buffered := buffer.Add(chunk.Content)
                for _, b := range buffered {
                    select {
                    case out <- &models.StreamChunk{Content: b}:
                    case <-ctx.Done():
                        return
                    }
                }
            }
        }
    }()

    return out
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
    default:
        return NewWordBuffer(" ")
    }
}
```

## Integration with SuperAgent

### Provider Integration

```go
// Example integration with existing providers

func (p *ClaudeProvider) CompleteStreamEnhanced(
    ctx context.Context,
    req *models.LLMRequest,
    opts *streaming.StreamConfig,
) (<-chan *models.StreamChunk, error) {
    // Get base stream from provider
    baseStream, err := p.CompleteStream(ctx, req)
    if err != nil {
        return nil, err
    }

    // Apply enhancements
    streamer := streaming.NewEnhancedStreamer(opts)

    enhanced := streamer.StreamBuffered(ctx, baseStream)

    if opts.RateLimit > 0 {
        limiter := streaming.NewRateLimiter(opts.RateLimit)
        enhanced = limiter.Limit(ctx, enhanced)
    }

    return enhanced, nil
}
```

## Test Coverage Requirements

```go
// tests/optimization/unit/streaming/buffer_test.go

func TestCharacterBuffer_Add(t *testing.T)
func TestWordBuffer_Add(t *testing.T)
func TestWordBuffer_Flush(t *testing.T)
func TestSentenceBuffer_Add(t *testing.T)
func TestSentenceBuffer_MultipleSentences(t *testing.T)
func TestLineBuffer_Add(t *testing.T)

func TestProgressTracker_Start(t *testing.T)
func TestProgressTracker_Update(t *testing.T)
func TestProgressTracker_TokensPerSecond(t *testing.T)

func TestRateLimiter_Basic(t *testing.T)
func TestRateLimiter_ContextCancel(t *testing.T)

func TestStreamAggregator_Aggregate(t *testing.T)
func TestStreamAggregator_GetResult(t *testing.T)

func TestSSEWriter_WriteEvent(t *testing.T)
func TestSSEWriter_WriteData(t *testing.T)
func TestStreamToSSE(t *testing.T)

func TestEnhancedStreamer_StreamWithProgress(t *testing.T)
func TestEnhancedStreamer_StreamBuffered(t *testing.T)
```

## Conclusion

llm-streaming is the simplest tool to port to Go. The concepts (buffering, progress tracking, SSE) are straightforward and have direct Go equivalents. Go's channel-based concurrency model is actually better suited for streaming than Python's async generators.

**Estimated Implementation Time**: 3-4 days
**Risk Level**: Low
**Dependencies**: None
