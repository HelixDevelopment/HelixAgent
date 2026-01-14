// Package streaming provides comprehensive streaming support for all streaming types.
// HelixAgent supports ALL streaming mechanisms to ensure compatibility with every CLI agent.
package streaming

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// StreamingType represents the type of streaming mechanism
type StreamingType string

const (
	StreamingTypeSSE         StreamingType = "sse"             // Server-Sent Events
	StreamingTypeWebSocket   StreamingType = "websocket"       // WebSocket
	StreamingTypeAsyncGen    StreamingType = "async_generator" // AsyncGenerator/yield
	StreamingTypeJSONL       StreamingType = "jsonl"           // JSON Lines streaming
	StreamingTypeMpscStream  StreamingType = "mpsc_stream"     // Rust multi-producer channel
	StreamingTypeEventStream StreamingType = "event_stream"    // AWS EventStream
	StreamingTypeStdout      StreamingType = "stdout"          // Standard output streaming
	StreamingTypeNone        StreamingType = "none"            // No streaming support
)

// AllStreamingTypes returns all supported streaming types
func AllStreamingTypes() []StreamingType {
	return []StreamingType{
		StreamingTypeSSE,
		StreamingTypeWebSocket,
		StreamingTypeAsyncGen,
		StreamingTypeJSONL,
		StreamingTypeMpscStream,
		StreamingTypeEventStream,
		StreamingTypeStdout,
	}
}

// StreamChunk represents a chunk of streaming data
type StreamChunk struct {
	ID        string    `json:"id,omitempty"`
	Content   string    `json:"content"`
	Index     int       `json:"index"`
	Done      bool      `json:"done"`
	Error     error     `json:"-"`
	Timestamp time.Time `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// StreamConfig holds configuration for streaming
type StreamConfig struct {
	Type            StreamingType
	BufferSize      int
	HeartbeatSec    int
	MaxDuration     time.Duration
	ChunkDelimiter  string
	EnableProgress  bool
	ProgressHandler func(progress *StreamProgress)
}

// DefaultStreamConfig returns default stream configuration
func DefaultStreamConfig(streamType StreamingType) *StreamConfig {
	return &StreamConfig{
		Type:           streamType,
		BufferSize:     4096,
		HeartbeatSec:   30,
		MaxDuration:    30 * time.Minute,
		ChunkDelimiter: "\n",
	}
}

// StreamProgress represents streaming progress
type StreamProgress struct {
	BytesSent       int64   `json:"bytes_sent"`
	ChunksEmitted   int     `json:"chunks_emitted"`
	ElapsedMs       int64   `json:"elapsed_ms"`
	PercentComplete float64 `json:"percent_complete,omitempty"`
}

// ============================================================================
// SSE (Server-Sent Events) Streaming
// Used by: OpenCode, ClaudeCode, Plandex, Crush, QwenCode, HelixCode
// ============================================================================

// SSEWriter writes Server-Sent Events format
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
}

// NewSSEWriter creates a new SSE writer
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported: response writer does not implement http.Flusher")
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	return &SSEWriter{w: w, flusher: flusher}, nil
}

// WriteEvent writes an SSE event with optional event type and ID
func (s *SSEWriter) WriteEvent(event, data string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id != "" {
		if _, err := fmt.Fprintf(s.w, "id: %s\n", id); err != nil {
			return err
		}
	}
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

// WriteData writes data-only SSE event
func (s *SSEWriter) WriteData(data string) error {
	return s.WriteEvent("", data, "")
}

// WriteJSON writes JSON data as SSE event
func (s *SSEWriter) WriteJSON(data interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.WriteData(string(bytes))
}

// WriteDone writes the [DONE] event marking stream completion
func (s *SSEWriter) WriteDone() error {
	return s.WriteData("[DONE]")
}

// WriteHeartbeat writes a heartbeat comment
func (s *SSEWriter) WriteHeartbeat() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.w.Write([]byte(": heartbeat\n\n")); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// ============================================================================
// WebSocket Streaming
// Used by: ClaudeCode
// ============================================================================

// WebSocketWriter writes WebSocket messages
type WebSocketWriter struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	config *StreamConfig
}

// NewWebSocketWriter creates a new WebSocket writer
func NewWebSocketWriter(conn *websocket.Conn, config *StreamConfig) *WebSocketWriter {
	if config == nil {
		config = DefaultStreamConfig(StreamingTypeWebSocket)
	}
	return &WebSocketWriter{
		conn:   conn,
		config: config,
	}
}

// WriteMessage writes a text message
func (w *WebSocketWriter) WriteMessage(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteMessage(websocket.TextMessage, data)
}

// WriteJSON writes JSON data
func (w *WebSocketWriter) WriteJSON(data interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(data)
}

// WriteBinary writes binary data
func (w *WebSocketWriter) WriteBinary(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteMessage(websocket.BinaryMessage, data)
}

// WritePing writes a ping message
func (w *WebSocketWriter) WritePing() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteMessage(websocket.PingMessage, []byte{})
}

// Close closes the WebSocket connection
func (w *WebSocketWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.Close()
}

// ============================================================================
// JSONL (JSON Lines) Streaming
// Used by: GeminiCLI
// ============================================================================

// JSONLWriter writes JSON Lines format (newline-delimited JSON)
type JSONLWriter struct {
	w       io.Writer
	flusher http.Flusher
	mu      sync.Mutex
}

// NewJSONLWriter creates a new JSONL writer
func NewJSONLWriter(w io.Writer) *JSONLWriter {
	jw := &JSONLWriter{w: w}
	if flusher, ok := w.(http.Flusher); ok {
		jw.flusher = flusher
	}
	return jw
}

// NewJSONLWriterHTTP creates a JSONL writer for HTTP responses
func NewJSONLWriterHTTP(w http.ResponseWriter) (*JSONLWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	// Set JSONL headers
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	return &JSONLWriter{w: w, flusher: flusher}, nil
}

// WriteLine writes a single JSON line
func (j *JSONLWriter) WriteLine(data interface{}) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if _, err := j.w.Write(bytes); err != nil {
		return err
	}
	if _, err := j.w.Write([]byte("\n")); err != nil {
		return err
	}

	if j.flusher != nil {
		j.flusher.Flush()
	}
	return nil
}

// WriteChunk writes a stream chunk as JSONL
func (j *JSONLWriter) WriteChunk(chunk *StreamChunk) error {
	return j.WriteLine(chunk)
}

// WriteDone writes a done marker
func (j *JSONLWriter) WriteDone() error {
	return j.WriteLine(map[string]interface{}{
		"done": true,
	})
}

// ============================================================================
// AsyncGenerator Streaming
// Used by: KiloCode, Cline, OllamaCode
// Simulates Python/JS async generator pattern via Go channels
// ============================================================================

// AsyncGenerator represents an async generator that yields values
type AsyncGenerator struct {
	output chan *StreamChunk
	done   chan struct{}
	err    error
	closed bool
	mu     sync.RWMutex
}

// NewAsyncGenerator creates a new async generator
func NewAsyncGenerator(bufferSize int) *AsyncGenerator {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &AsyncGenerator{
		output: make(chan *StreamChunk, bufferSize),
		done:   make(chan struct{}),
	}
}

// Yield emits a value from the generator
func (g *AsyncGenerator) Yield(chunk *StreamChunk) error {
	g.mu.RLock()
	if g.closed {
		g.mu.RUnlock()
		return fmt.Errorf("generator closed")
	}
	g.mu.RUnlock()

	select {
	case <-g.done:
		return fmt.Errorf("generator closed")
	case g.output <- chunk:
		return nil
	}
}

// YieldContent yields string content
func (g *AsyncGenerator) YieldContent(content string, index int) error {
	return g.Yield(&StreamChunk{
		Content:   content,
		Index:     index,
		Timestamp: time.Now(),
	})
}

// Next returns the next value from the generator
func (g *AsyncGenerator) Next(ctx context.Context) (*StreamChunk, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-g.done:
		g.mu.RLock()
		err := g.err
		g.mu.RUnlock()
		return nil, err
	case chunk, ok := <-g.output:
		if !ok {
			return nil, io.EOF
		}
		return chunk, nil
	}
}

// Close closes the generator
func (g *AsyncGenerator) Close(err error) {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return
	}
	g.closed = true
	g.err = err
	g.mu.Unlock()
	close(g.done)
	close(g.output)
}

// Channel returns the output channel for range iteration
func (g *AsyncGenerator) Channel() <-chan *StreamChunk {
	return g.output
}

// ============================================================================
// EventStream (AWS) Streaming
// Used by: Amazon Q
// ============================================================================

// EventStreamMessage represents an AWS EventStream message
type EventStreamMessage struct {
	Headers map[string]string `json:"headers"`
	Payload []byte            `json:"payload"`
}

// EventStreamWriter writes AWS EventStream format
type EventStreamWriter struct {
	w       io.Writer
	flusher http.Flusher
	mu      sync.Mutex
}

// NewEventStreamWriter creates a new EventStream writer
func NewEventStreamWriter(w io.Writer) *EventStreamWriter {
	esw := &EventStreamWriter{w: w}
	if flusher, ok := w.(http.Flusher); ok {
		esw.flusher = flusher
	}
	return esw
}

// NewEventStreamWriterHTTP creates an EventStream writer for HTTP
func NewEventStreamWriterHTTP(w http.ResponseWriter) (*EventStreamWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	// Set EventStream headers (AWS format)
	w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Accel-Buffering", "no")

	return &EventStreamWriter{w: w, flusher: flusher}, nil
}

// WriteEvent writes an EventStream message
func (e *EventStreamWriter) WriteEvent(eventType string, payload []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// AWS EventStream format:
	// [prelude (8 bytes)] [headers] [payload] [message CRC (4 bytes)]
	msg := EventStreamMessage{
		Headers: map[string]string{
			":event-type":    eventType,
			":content-type":  "application/json",
			":message-type":  "event",
		},
		Payload: payload,
	}

	// Simplified encoding - real AWS EventStream uses binary framing
	encoded, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if _, err := e.w.Write(encoded); err != nil {
		return err
	}
	if _, err := e.w.Write([]byte("\n")); err != nil {
		return err
	}

	if e.flusher != nil {
		e.flusher.Flush()
	}
	return nil
}

// WriteChunk writes a stream chunk as EventStream
func (e *EventStreamWriter) WriteChunk(chunk *StreamChunk) error {
	payload, err := json.Marshal(chunk)
	if err != nil {
		return err
	}
	return e.WriteEvent("chunk", payload)
}

// WriteDone writes a completion event
func (e *EventStreamWriter) WriteDone() error {
	return e.WriteEvent("done", []byte(`{"done":true}`))
}

// ============================================================================
// MpscStream (Multi-Producer Single-Consumer) Streaming
// Used by: Forge (Rust)
// Implemented via Go channels with fan-in pattern
// ============================================================================

// MpscStream represents a multi-producer single-consumer stream
type MpscStream struct {
	inputs  []chan *StreamChunk
	output  chan *StreamChunk
	done    chan struct{}
	wg      sync.WaitGroup
	started bool
	mu      sync.Mutex
}

// NewMpscStream creates a new MPSC stream
func NewMpscStream(numProducers, bufferSize int) *MpscStream {
	if numProducers <= 0 {
		numProducers = 1
	}
	if bufferSize <= 0 {
		bufferSize = 100
	}

	inputs := make([]chan *StreamChunk, numProducers)
	for i := 0; i < numProducers; i++ {
		inputs[i] = make(chan *StreamChunk, bufferSize)
	}

	return &MpscStream{
		inputs: inputs,
		output: make(chan *StreamChunk, bufferSize*numProducers),
		done:   make(chan struct{}),
	}
}

// GetProducer returns a producer channel by index
func (m *MpscStream) GetProducer(index int) chan<- *StreamChunk {
	if index < 0 || index >= len(m.inputs) {
		return nil
	}
	return m.inputs[index]
}

// Start starts the fan-in goroutines
func (m *MpscStream) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()

	// Fan-in: collect from all producers
	for _, input := range m.inputs {
		m.wg.Add(1)
		go func(in chan *StreamChunk) {
			defer m.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-m.done:
					return
				case chunk, ok := <-in:
					if !ok {
						return
					}
					select {
					case m.output <- chunk:
					case <-ctx.Done():
						return
					case <-m.done:
						return
					}
				}
			}
		}(input)
	}

	// Close output when all producers done
	go func() {
		m.wg.Wait()
		close(m.output)
	}()
}

// Consumer returns the consumer channel
func (m *MpscStream) Consumer() <-chan *StreamChunk {
	return m.output
}

// Close closes the stream
func (m *MpscStream) Close() {
	close(m.done)
	for _, input := range m.inputs {
		close(input)
	}
}

// ============================================================================
// Stdout Streaming
// Used by: Aider, GPT Engineer
// ============================================================================

// StdoutWriter writes to stdout with buffering
type StdoutWriter struct {
	w         io.Writer
	buffered  *bufio.Writer
	mu        sync.Mutex
	lineMode  bool
	delimiter string
}

// NewStdoutWriter creates a new stdout writer
func NewStdoutWriter(w io.Writer, lineMode bool) *StdoutWriter {
	return &StdoutWriter{
		w:         w,
		buffered:  bufio.NewWriter(w),
		lineMode:  lineMode,
		delimiter: "\n",
	}
}

// Write writes raw bytes
func (s *StdoutWriter) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, err := s.buffered.Write(data)
	if err != nil {
		return n, err
	}

	// In line mode, only flush on newlines
	if s.lineMode {
		if bytes.Contains(data, []byte(s.delimiter)) {
			s.buffered.Flush()
		}
	} else {
		s.buffered.Flush()
	}

	return n, nil
}

// WriteLine writes a line with newline
func (s *StdoutWriter) WriteLine(line string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.buffered.WriteString(line); err != nil {
		return err
	}
	if !strings.HasSuffix(line, s.delimiter) {
		if _, err := s.buffered.WriteString(s.delimiter); err != nil {
			return err
		}
	}
	return s.buffered.Flush()
}

// WriteChunk writes a stream chunk
func (s *StdoutWriter) WriteChunk(chunk *StreamChunk) error {
	return s.WriteLine(chunk.Content)
}

// Flush flushes the buffer
func (s *StdoutWriter) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffered.Flush()
}

// ============================================================================
// Universal Stream Handler
// ============================================================================

// UniversalStreamer provides a unified interface for all streaming types
type UniversalStreamer struct {
	streamType StreamingType
	config     *StreamConfig
	progress   *StreamProgress
	mu         sync.Mutex
}

// NewUniversalStreamer creates a new universal streamer
func NewUniversalStreamer(streamType StreamingType, config *StreamConfig) *UniversalStreamer {
	if config == nil {
		config = DefaultStreamConfig(streamType)
	}
	return &UniversalStreamer{
		streamType: streamType,
		config:     config,
		progress:   &StreamProgress{},
	}
}

// StreamType returns the streaming type
func (u *UniversalStreamer) StreamType() StreamingType {
	return u.streamType
}

// GetProgress returns current progress
func (u *UniversalStreamer) GetProgress() *StreamProgress {
	u.mu.Lock()
	defer u.mu.Unlock()
	return &StreamProgress{
		BytesSent:       u.progress.BytesSent,
		ChunksEmitted:   u.progress.ChunksEmitted,
		ElapsedMs:       u.progress.ElapsedMs,
		PercentComplete: u.progress.PercentComplete,
	}
}

// UpdateProgress updates progress tracking
func (u *UniversalStreamer) UpdateProgress(bytesSent int64, chunksEmitted int, elapsedMs int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.progress.BytesSent += bytesSent
	u.progress.ChunksEmitted += chunksEmitted
	u.progress.ElapsedMs = elapsedMs

	if u.config.EnableProgress && u.config.ProgressHandler != nil {
		u.config.ProgressHandler(u.progress)
	}
}

// ContentTypeForStreamingType returns the appropriate Content-Type header
func ContentTypeForStreamingType(st StreamingType) string {
	switch st {
	case StreamingTypeSSE:
		return "text/event-stream"
	case StreamingTypeWebSocket:
		return "application/octet-stream" // WebSocket doesn't use Content-Type
	case StreamingTypeJSONL:
		return "application/x-ndjson"
	case StreamingTypeEventStream:
		return "application/vnd.amazon.eventstream"
	case StreamingTypeAsyncGen:
		return "text/event-stream" // AsyncGen typically uses SSE for HTTP transport
	case StreamingTypeMpscStream:
		return "application/octet-stream" // Internal streaming
	case StreamingTypeStdout:
		return "text/plain"
	default:
		return "application/json"
	}
}

// IsStreamingSupported checks if streaming type is supported
func IsStreamingSupported(st StreamingType) bool {
	for _, supported := range AllStreamingTypes() {
		if supported == st {
			return true
		}
	}
	return false
}
