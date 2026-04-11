// DEPRECATED: This package is superseded by the extracted Streaming/ submodule
// (module digital.vasic.streaming). It has zero production imports (only one
// integration test) and is retained only for reference. New code should use
// the submodule via adapters.
//
// Package streaming provides Kafka-backed stream persistence and replay.
package streaming

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/messaging"
)

// KafkaStreamTopic constants for streaming events - topic names, not credentials
const (
	TopicTokenStream       = "helixagent.stream.tokens" // #nosec G101 - Kafka topic name, not credentials
	TopicSSEEvents         = "helixagent.stream.sse"
	TopicWebSocketMessages = "helixagent.stream.websocket"
	TopicStreamEvents      = "helixagent.stream.events"
)

// StreamEvent represents a streaming event to be persisted to Kafka.
type StreamEvent struct {
	// ID is the unique event identifier.
	ID string `json:"id"`
	// StreamID identifies the stream this event belongs to.
	StreamID string `json:"stream_id"`
	// Type is the event type (token, chunk, done, error).
	Type string `json:"type"`
	// Content is the event content.
	Content string `json:"content,omitempty"`
	// Index is the event sequence number within the stream.
	Index int `json:"index"`
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// Metadata contains additional event data.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// Done indicates if this is the final event.
	Done bool `json:"done,omitempty"`
	// Error contains error information if applicable.
	Error string `json:"error,omitempty"`
}

// KafkaStreamWriterConfig holds configuration for the Kafka stream writer.
type KafkaStreamWriterConfig struct {
	// Enabled enables Kafka persistence.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Topic is the Kafka topic for stream events.
	Topic string `json:"topic" yaml:"topic"`
	// Async enables async publishing.
	Async bool `json:"async" yaml:"async"`
	// BufferSize is the async buffer size.
	BufferSize int `json:"buffer_size" yaml:"buffer_size"`
	// RetentionMs is how long events are retained in Kafka.
	RetentionMs int64 `json:"retention_ms" yaml:"retention_ms"`
}

// DefaultKafkaStreamWriterConfig returns default configuration.
func DefaultKafkaStreamWriterConfig() *KafkaStreamWriterConfig {
	return &KafkaStreamWriterConfig{
		Enabled:     true,
		Topic:       TopicStreamEvents,
		Async:       true,
		BufferSize:  1000,
		RetentionMs: 24 * 60 * 60 * 1000, // 24 hours
	}
}

// KafkaStreamWriter writes stream events to Kafka for persistence and replay.
type KafkaStreamWriter struct {
	hub      *messaging.MessagingHub
	config   *KafkaStreamWriterConfig
	logger   *logrus.Logger
	streamID string
	index    int
	eventCh  chan *StreamEvent
	stopCh   chan struct{}
	stopOnce sync.Once // Prevents double-close panic on Stop()
	wg       sync.WaitGroup
	mu       sync.Mutex
	started  bool
}

// NewKafkaStreamWriter creates a new Kafka stream writer.
func NewKafkaStreamWriter(
	hub *messaging.MessagingHub,
	streamID string,
	logger *logrus.Logger,
	config *KafkaStreamWriterConfig,
) *KafkaStreamWriter {
	if config == nil {
		config = DefaultKafkaStreamWriterConfig()
	}
	if logger == nil {
		logger = logrus.New()
	}

	w := &KafkaStreamWriter{
		hub:      hub,
		config:   config,
		logger:   logger,
		streamID: streamID,
		stopCh:   make(chan struct{}),
	}

	if config.Async && config.BufferSize > 0 {
		w.eventCh = make(chan *StreamEvent, config.BufferSize)
	}

	return w
}

// Start starts the async publishing goroutine if enabled.
func (w *KafkaStreamWriter) Start() {
	w.mu.Lock()
	if w.started || !w.config.Async || w.eventCh == nil {
		w.mu.Unlock()
		return
	}
	w.started = true
	w.mu.Unlock()

	w.wg.Add(1)
	go w.asyncPublishLoop()
}

// Stop stops the stream writer and flushes remaining events.
// It is safe to call multiple times (idempotent).
func (w *KafkaStreamWriter) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
		if w.eventCh != nil {
			close(w.eventCh)
		}
	})
	w.wg.Wait()
}

// asyncPublishLoop processes async publish events.
func (w *KafkaStreamWriter) asyncPublishLoop() {
	defer w.wg.Done()

	for {
		select {
		case event, ok := <-w.eventCh:
			if !ok {
				return
			}
			if err := w.doPublish(context.Background(), event); err != nil {
				w.logger.WithError(err).Debug("Failed to publish event")
			}
		case <-w.stopCh:
			// Drain remaining events
			for event := range w.eventCh {
				if err := w.doPublish(context.Background(), event); err != nil {
					w.logger.WithError(err).Debug("Failed to publish event")
				}
			}
			return
		}
	}
}

// WriteToken writes a token event.
func (w *KafkaStreamWriter) WriteToken(ctx context.Context, token string) error {
	event := &StreamEvent{
		ID:        generateStreamEventID(),
		StreamID:  w.streamID,
		Type:      "token",
		Content:   token,
		Index:     w.nextIndex(),
		Timestamp: time.Now().UTC(),
	}
	return w.publish(ctx, event)
}

// WriteChunk writes a stream chunk event.
func (w *KafkaStreamWriter) WriteChunk(ctx context.Context, chunk *StreamChunk) error {
	event := &StreamEvent{
		ID:        generateStreamEventID(),
		StreamID:  w.streamID,
		Type:      "chunk",
		Content:   chunk.Content,
		Index:     w.nextIndex(),
		Timestamp: time.Now().UTC(),
		Done:      chunk.Done,
		Metadata:  chunk.Metadata,
	}
	if chunk.Error != nil {
		event.Error = chunk.Error.Error()
		event.Type = "error"
	}
	return w.publish(ctx, event)
}

// WriteEvent writes a generic stream event.
func (w *KafkaStreamWriter) WriteEvent(ctx context.Context, event *StreamEvent) error {
	if event.ID == "" {
		event.ID = generateStreamEventID()
	}
	if event.StreamID == "" {
		event.StreamID = w.streamID
	}
	if event.Index == 0 {
		event.Index = w.nextIndex()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	return w.publish(ctx, event)
}

// WriteDone writes a completion event.
func (w *KafkaStreamWriter) WriteDone(ctx context.Context) error {
	event := &StreamEvent{
		ID:        generateStreamEventID(),
		StreamID:  w.streamID,
		Type:      "done",
		Index:     w.nextIndex(),
		Timestamp: time.Now().UTC(),
		Done:      true,
	}
	return w.publish(ctx, event)
}

// WriteError writes an error event.
func (w *KafkaStreamWriter) WriteError(ctx context.Context, err error) error {
	event := &StreamEvent{
		ID:        generateStreamEventID(),
		StreamID:  w.streamID,
		Type:      "error",
		Index:     w.nextIndex(),
		Timestamp: time.Now().UTC(),
		Error:     err.Error(),
		Done:      true,
	}
	return w.publish(ctx, event)
}

// publish publishes an event (async or sync).
func (w *KafkaStreamWriter) publish(ctx context.Context, event *StreamEvent) error {
	if !w.config.Enabled || w.hub == nil {
		return nil
	}

	if w.config.Async && w.eventCh != nil {
		select {
		case w.eventCh <- event:
			return nil
		default:
			// Buffer full, publish synchronously
			w.logger.Warn("Stream event buffer full, publishing synchronously")
			return w.doPublish(ctx, event)
		}
	}

	return w.doPublish(ctx, event)
}

// doPublish performs the actual event publish.
func (w *KafkaStreamWriter) doPublish(ctx context.Context, event *StreamEvent) error {
	if event == nil {
		return nil
	}

	data, err := json.Marshal(event)
	if err != nil {
		w.logger.WithError(err).Error("Failed to marshal stream event")
		return err
	}

	msgEvent := &messaging.Event{
		ID:         event.ID,
		Type:       messaging.EventType("stream." + event.Type),
		Source:     "helixagent.streaming",
		Subject:    event.StreamID,
		Data:       data,
		DataSchema: "application/json",
		Timestamp:  event.Timestamp,
	}

	if err := w.hub.PublishEvent(ctx, w.config.Topic, msgEvent); err != nil {
		w.logger.WithError(err).WithFields(logrus.Fields{
			"stream_id":  event.StreamID,
			"event_type": event.Type,
			"index":      event.Index,
		}).Error("Failed to publish stream event")
		return err
	}

	w.logger.WithFields(logrus.Fields{
		"stream_id":  event.StreamID,
		"event_type": event.Type,
		"index":      event.Index,
	}).Debug("Published stream event")

	return nil
}

// nextIndex returns the next event index.
func (w *KafkaStreamWriter) nextIndex() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.index++
	return w.index
}

// StreamID returns the stream ID.
func (w *KafkaStreamWriter) StreamID() string {
	return w.streamID
}

// IsEnabled returns whether Kafka persistence is enabled.
func (w *KafkaStreamWriter) IsEnabled() bool {
	return w.config.Enabled && w.hub != nil
}

// generateStreamEventID generates a unique stream event ID.
func generateStreamEventID() string {
	return time.Now().UTC().Format("20060102150405.000000000") + "-" + randomStreamString(8)
}

// randomStreamString generates a random string.
func randomStreamString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))] // #nosec G404 -- non-security random (jitter/test data/load spread); math/rand is the correct choice
	}
	return string(b)
}

// HybridStreamWriter combines direct streaming with Kafka persistence.
// It writes to both the real-time output and Kafka for persistence/replay.
type HybridStreamWriter struct {
	// direct is the direct stream writer (SSE, WebSocket, etc.)
	direct interface{}
	// kafka is the Kafka persistence writer
	kafka *KafkaStreamWriter
	// streamType is the type of direct streaming
	streamType StreamingType
	// logger for logging
	logger *logrus.Logger
}

// NewHybridStreamWriter creates a new hybrid stream writer.
func NewHybridStreamWriter(
	direct interface{},
	kafka *KafkaStreamWriter,
	streamType StreamingType,
	logger *logrus.Logger,
) *HybridStreamWriter {
	return &HybridStreamWriter{
		direct:     direct,
		kafka:      kafka,
		streamType: streamType,
		logger:     logger,
	}
}

// WriteChunk writes a chunk to both direct output and Kafka.
func (h *HybridStreamWriter) WriteChunk(ctx context.Context, chunk *StreamChunk) error {
	// Write to direct output based on type
	var directErr error
	switch w := h.direct.(type) {
	case *SSEWriter:
		directErr = w.WriteJSON(chunk)
	case *WebSocketWriter:
		directErr = w.WriteJSON(chunk)
	case *JSONLWriter:
		directErr = w.WriteChunk(chunk)
	case *EventStreamWriter:
		directErr = w.WriteChunk(chunk)
	case *StdoutWriter:
		directErr = w.WriteChunk(chunk)
	}

	// Write to Kafka for persistence (don't fail on Kafka errors)
	if h.kafka != nil && h.kafka.IsEnabled() {
		if err := h.kafka.WriteChunk(ctx, chunk); err != nil {
			h.logger.WithError(err).Debug("Failed to persist chunk to Kafka")
		}
	}

	return directErr
}

// WriteDone writes completion to both direct output and Kafka.
func (h *HybridStreamWriter) WriteDone(ctx context.Context) error {
	// Write to direct output based on type
	var directErr error
	switch w := h.direct.(type) {
	case *SSEWriter:
		directErr = w.WriteDone()
	case *JSONLWriter:
		directErr = w.WriteDone()
	case *EventStreamWriter:
		directErr = w.WriteDone()
	}

	// Write to Kafka for persistence
	if h.kafka != nil && h.kafka.IsEnabled() {
		if err := h.kafka.WriteDone(ctx); err != nil {
			h.logger.WithError(err).Debug("Failed to persist done event to Kafka")
		}
	}

	return directErr
}

// StreamType returns the streaming type.
func (h *HybridStreamWriter) StreamType() StreamingType {
	return h.streamType
}

// KafkaWriter returns the Kafka writer.
func (h *HybridStreamWriter) KafkaWriter() *KafkaStreamWriter {
	return h.kafka
}
