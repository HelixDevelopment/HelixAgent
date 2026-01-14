package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Streaming Type Constants Tests
// ============================================================================

func TestStreamingTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		st       StreamingType
		expected string
	}{
		{"SSE", StreamingTypeSSE, "sse"},
		{"WebSocket", StreamingTypeWebSocket, "websocket"},
		{"AsyncGenerator", StreamingTypeAsyncGen, "async_generator"},
		{"JSONL", StreamingTypeJSONL, "jsonl"},
		{"MpscStream", StreamingTypeMpscStream, "mpsc_stream"},
		{"EventStream", StreamingTypeEventStream, "event_stream"},
		{"Stdout", StreamingTypeStdout, "stdout"},
		{"None", StreamingTypeNone, "none"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, string(tc.st))
		})
	}
}

func TestAllStreamingTypes(t *testing.T) {
	types := AllStreamingTypes()
	assert.Len(t, types, 7, "Should have 7 streaming types (excluding 'none')")

	// Verify all expected types are present
	expected := map[StreamingType]bool{
		StreamingTypeSSE:         true,
		StreamingTypeWebSocket:   true,
		StreamingTypeAsyncGen:    true,
		StreamingTypeJSONL:       true,
		StreamingTypeMpscStream:  true,
		StreamingTypeEventStream: true,
		StreamingTypeStdout:      true,
	}

	for _, st := range types {
		assert.True(t, expected[st], "Unexpected streaming type: %s", st)
	}
}

func TestIsStreamingSupported(t *testing.T) {
	tests := []struct {
		st        StreamingType
		supported bool
	}{
		{StreamingTypeSSE, true},
		{StreamingTypeWebSocket, true},
		{StreamingTypeAsyncGen, true},
		{StreamingTypeJSONL, true},
		{StreamingTypeMpscStream, true},
		{StreamingTypeEventStream, true},
		{StreamingTypeStdout, true},
		{StreamingTypeNone, false},
		{StreamingType("invalid"), false},
	}

	for _, tc := range tests {
		t.Run(string(tc.st), func(t *testing.T) {
			assert.Equal(t, tc.supported, IsStreamingSupported(tc.st))
		})
	}
}

func TestContentTypeForStreamingType(t *testing.T) {
	tests := []struct {
		st          StreamingType
		contentType string
	}{
		{StreamingTypeSSE, "text/event-stream"},
		{StreamingTypeWebSocket, "application/octet-stream"},
		{StreamingTypeJSONL, "application/x-ndjson"},
		{StreamingTypeEventStream, "application/vnd.amazon.eventstream"},
		{StreamingTypeAsyncGen, "text/event-stream"},
		{StreamingTypeMpscStream, "application/octet-stream"},
		{StreamingTypeStdout, "text/plain"},
		{StreamingTypeNone, "application/json"},
		{StreamingType("unknown"), "application/json"},
	}

	for _, tc := range tests {
		t.Run(string(tc.st), func(t *testing.T) {
			assert.Equal(t, tc.contentType, ContentTypeForStreamingType(tc.st))
		})
	}
}

// ============================================================================
// StreamChunk Tests
// ============================================================================

func TestStreamChunk(t *testing.T) {
	chunk := &StreamChunk{
		ID:        "test-id",
		Content:   "Hello, World!",
		Index:     0,
		Done:      false,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"key": "value",
		},
	}

	assert.Equal(t, "test-id", chunk.ID)
	assert.Equal(t, "Hello, World!", chunk.Content)
	assert.Equal(t, 0, chunk.Index)
	assert.False(t, chunk.Done)
	assert.NotNil(t, chunk.Metadata)
	assert.Equal(t, "value", chunk.Metadata["key"])
}

func TestStreamChunkJSON(t *testing.T) {
	chunk := &StreamChunk{
		ID:        "chunk-1",
		Content:   "Test content",
		Index:     5,
		Done:      true,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(chunk)
	require.NoError(t, err)

	var decoded StreamChunk
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, chunk.ID, decoded.ID)
	assert.Equal(t, chunk.Content, decoded.Content)
	assert.Equal(t, chunk.Index, decoded.Index)
	assert.Equal(t, chunk.Done, decoded.Done)
}

// ============================================================================
// StreamConfig Tests
// ============================================================================

func TestDefaultStreamConfig(t *testing.T) {
	for _, st := range AllStreamingTypes() {
		t.Run(string(st), func(t *testing.T) {
			config := DefaultStreamConfig(st)
			assert.Equal(t, st, config.Type)
			assert.Equal(t, 4096, config.BufferSize)
			assert.Equal(t, 30, config.HeartbeatSec)
			assert.Equal(t, 30*time.Minute, config.MaxDuration)
			assert.Equal(t, "\n", config.ChunkDelimiter)
		})
	}
}

func TestStreamProgress(t *testing.T) {
	progress := &StreamProgress{
		BytesSent:       1024,
		ChunksEmitted:   10,
		ElapsedMs:       500,
		PercentComplete: 50.0,
	}

	assert.Equal(t, int64(1024), progress.BytesSent)
	assert.Equal(t, 10, progress.ChunksEmitted)
	assert.Equal(t, int64(500), progress.ElapsedMs)
	assert.Equal(t, 50.0, progress.PercentComplete)
}

// ============================================================================
// SSE (Server-Sent Events) Tests
// ============================================================================

func TestNewSSEWriter(t *testing.T) {
	recorder := httptest.NewRecorder()
	sse, err := NewSSEWriter(recorder)

	require.NoError(t, err)
	assert.NotNil(t, sse)
	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", recorder.Header().Get("Connection"))
	assert.Equal(t, "no", recorder.Header().Get("X-Accel-Buffering"))
}

func TestSSEWriter_NoFlusher(t *testing.T) {
	// Create a writer that doesn't implement http.Flusher
	w := &nonFlushingWriter{}
	_, err := NewSSEWriter(w)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "streaming not supported")
}

func TestSSEWriter_WriteEvent(t *testing.T) {
	recorder := httptest.NewRecorder()
	sse, err := NewSSEWriter(recorder)
	require.NoError(t, err)

	err = sse.WriteEvent("message", "Hello, World!", "event-123")
	require.NoError(t, err)

	body := recorder.Body.String()
	assert.Contains(t, body, "id: event-123")
	assert.Contains(t, body, "event: message")
	assert.Contains(t, body, "data: Hello, World!")
}

func TestSSEWriter_WriteData(t *testing.T) {
	recorder := httptest.NewRecorder()
	sse, err := NewSSEWriter(recorder)
	require.NoError(t, err)

	err = sse.WriteData("test data")
	require.NoError(t, err)

	body := recorder.Body.String()
	assert.Contains(t, body, "data: test data")
	assert.NotContains(t, body, "event:")
	assert.NotContains(t, body, "id:")
}

func TestSSEWriter_WriteJSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	sse, err := NewSSEWriter(recorder)
	require.NoError(t, err)

	data := map[string]interface{}{
		"message": "Hello",
		"count":   42,
	}
	err = sse.WriteJSON(data)
	require.NoError(t, err)

	body := recorder.Body.String()
	assert.Contains(t, body, `"message":"Hello"`)
	assert.Contains(t, body, `"count":42`)
}

func TestSSEWriter_WriteDone(t *testing.T) {
	recorder := httptest.NewRecorder()
	sse, err := NewSSEWriter(recorder)
	require.NoError(t, err)

	err = sse.WriteDone()
	require.NoError(t, err)

	body := recorder.Body.String()
	assert.Contains(t, body, "data: [DONE]")
}

func TestSSEWriter_WriteHeartbeat(t *testing.T) {
	recorder := httptest.NewRecorder()
	sse, err := NewSSEWriter(recorder)
	require.NoError(t, err)

	err = sse.WriteHeartbeat()
	require.NoError(t, err)

	body := recorder.Body.String()
	assert.Contains(t, body, ": heartbeat")
}

func TestSSEWriter_Concurrent(t *testing.T) {
	recorder := httptest.NewRecorder()
	sse, err := NewSSEWriter(recorder)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sse.WriteData(strings.Repeat("x", 10))
		}(i)
	}
	wg.Wait()

	// Should have written 100 events without data race
	body := recorder.Body.String()
	count := strings.Count(body, "data:")
	assert.Equal(t, 100, count)
}

// ============================================================================
// JSONL (JSON Lines) Tests
// ============================================================================

func TestNewJSONLWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	jw := NewJSONLWriter(buf)
	assert.NotNil(t, jw)
}

func TestNewJSONLWriterHTTP(t *testing.T) {
	recorder := httptest.NewRecorder()
	jw, err := NewJSONLWriterHTTP(recorder)

	require.NoError(t, err)
	assert.NotNil(t, jw)
	assert.Equal(t, "application/x-ndjson", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", recorder.Header().Get("Cache-Control"))
}

func TestJSONLWriter_NoFlusher(t *testing.T) {
	w := &nonFlushingWriter{}
	_, err := NewJSONLWriterHTTP(w)
	assert.Error(t, err)
}

func TestJSONLWriter_WriteLine(t *testing.T) {
	buf := &bytes.Buffer{}
	jw := NewJSONLWriter(buf)

	data := map[string]string{"message": "Hello"}
	err := jw.WriteLine(data)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), `{"message":"Hello"}`)
	assert.True(t, strings.HasSuffix(buf.String(), "\n"))
}

func TestJSONLWriter_WriteChunk(t *testing.T) {
	buf := &bytes.Buffer{}
	jw := NewJSONLWriter(buf)

	chunk := &StreamChunk{
		Content: "Test content",
		Index:   0,
		Done:    false,
	}
	err := jw.WriteChunk(chunk)
	require.NoError(t, err)

	var decoded StreamChunk
	err = json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &decoded)
	require.NoError(t, err)
	assert.Equal(t, "Test content", decoded.Content)
}

func TestJSONLWriter_WriteDone(t *testing.T) {
	buf := &bytes.Buffer{}
	jw := NewJSONLWriter(buf)

	err := jw.WriteDone()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), `"done":true`)
}

func TestJSONLWriter_MultipleLines(t *testing.T) {
	buf := &bytes.Buffer{}
	jw := NewJSONLWriter(buf)

	for i := 0; i < 5; i++ {
		err := jw.WriteChunk(&StreamChunk{Content: "line", Index: i})
		require.NoError(t, err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, 5)
}

// ============================================================================
// AsyncGenerator Tests
// ============================================================================

func TestNewAsyncGenerator(t *testing.T) {
	ag := NewAsyncGenerator(100)
	assert.NotNil(t, ag)
	assert.NotNil(t, ag.output)
	assert.NotNil(t, ag.done)
}

func TestAsyncGenerator_DefaultBufferSize(t *testing.T) {
	ag := NewAsyncGenerator(0) // Should default to 100
	assert.NotNil(t, ag)
}

func TestAsyncGenerator_Yield(t *testing.T) {
	ag := NewAsyncGenerator(10)

	chunk := &StreamChunk{Content: "Hello", Index: 0}
	err := ag.Yield(chunk)
	require.NoError(t, err)

	received := <-ag.Channel()
	assert.Equal(t, "Hello", received.Content)
}

func TestAsyncGenerator_YieldContent(t *testing.T) {
	ag := NewAsyncGenerator(10)

	err := ag.YieldContent("Test content", 5)
	require.NoError(t, err)

	received := <-ag.Channel()
	assert.Equal(t, "Test content", received.Content)
	assert.Equal(t, 5, received.Index)
	assert.False(t, received.Timestamp.IsZero())
}

func TestAsyncGenerator_Next(t *testing.T) {
	ag := NewAsyncGenerator(10)
	ctx := context.Background()

	go func() {
		ag.YieldContent("Chunk 1", 0)
		ag.YieldContent("Chunk 2", 1)
	}()

	chunk1, err := ag.Next(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Chunk 1", chunk1.Content)

	chunk2, err := ag.Next(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Chunk 2", chunk2.Content)
}

func TestAsyncGenerator_NextContextCancel(t *testing.T) {
	ag := NewAsyncGenerator(10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ag.Next(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestAsyncGenerator_Close(t *testing.T) {
	ag := NewAsyncGenerator(10)

	ag.Close(nil)

	err := ag.Yield(&StreamChunk{Content: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "generator closed")
}

func TestAsyncGenerator_CloseWithError(t *testing.T) {
	ag := NewAsyncGenerator(10)
	expectedErr := io.EOF

	ag.Close(expectedErr)

	ctx := context.Background()
	_, err := ag.Next(ctx)
	assert.Equal(t, expectedErr, err)
}

func TestAsyncGenerator_Channel(t *testing.T) {
	ag := NewAsyncGenerator(10)

	go func() {
		for i := 0; i < 5; i++ {
			ag.YieldContent("chunk", i)
		}
		ag.Close(nil)
	}()

	count := 0
	for chunk := range ag.Channel() {
		count++
		assert.Equal(t, "chunk", chunk.Content)
	}
	assert.Equal(t, 5, count)
}

// ============================================================================
// EventStream (AWS) Tests
// ============================================================================

func TestNewEventStreamWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	esw := NewEventStreamWriter(buf)
	assert.NotNil(t, esw)
}

func TestNewEventStreamWriterHTTP(t *testing.T) {
	recorder := httptest.NewRecorder()
	esw, err := NewEventStreamWriterHTTP(recorder)

	require.NoError(t, err)
	assert.NotNil(t, esw)
	assert.Equal(t, "application/vnd.amazon.eventstream", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "chunked", recorder.Header().Get("Transfer-Encoding"))
}

func TestEventStreamWriter_NoFlusher(t *testing.T) {
	w := &nonFlushingWriter{}
	_, err := NewEventStreamWriterHTTP(w)
	assert.Error(t, err)
}

func TestEventStreamWriter_WriteEvent(t *testing.T) {
	buf := &bytes.Buffer{}
	esw := NewEventStreamWriter(buf)

	payload := []byte(`{"message":"Hello"}`)
	err := esw.WriteEvent("test-event", payload)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "test-event")
	// Payload is base64 encoded in the EventStreamMessage
	var msg EventStreamMessage
	err = json.Unmarshal([]byte(strings.TrimSpace(output)), &msg)
	require.NoError(t, err)
	assert.Equal(t, "test-event", msg.Headers[":event-type"])
	assert.Equal(t, `{"message":"Hello"}`, string(msg.Payload))
}

func TestEventStreamWriter_WriteChunk(t *testing.T) {
	buf := &bytes.Buffer{}
	esw := NewEventStreamWriter(buf)

	chunk := &StreamChunk{Content: "AWS chunk", Index: 0}
	err := esw.WriteChunk(chunk)
	require.NoError(t, err)

	output := buf.String()
	// Payload is JSON encoded StreamChunk within EventStreamMessage
	var msg EventStreamMessage
	err = json.Unmarshal([]byte(strings.TrimSpace(output)), &msg)
	require.NoError(t, err)
	assert.Equal(t, "chunk", msg.Headers[":event-type"])

	var decodedChunk StreamChunk
	err = json.Unmarshal(msg.Payload, &decodedChunk)
	require.NoError(t, err)
	assert.Equal(t, "AWS chunk", decodedChunk.Content)
}

func TestEventStreamWriter_WriteDone(t *testing.T) {
	buf := &bytes.Buffer{}
	esw := NewEventStreamWriter(buf)

	err := esw.WriteDone()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "done")
}

// ============================================================================
// MpscStream (Multi-Producer Single-Consumer) Tests
// ============================================================================

func TestNewMpscStream(t *testing.T) {
	mpsc := NewMpscStream(3, 100)
	assert.NotNil(t, mpsc)
	assert.Len(t, mpsc.inputs, 3)
}

func TestNewMpscStream_DefaultValues(t *testing.T) {
	mpsc := NewMpscStream(0, 0)
	assert.Len(t, mpsc.inputs, 1) // Default to 1 producer
}

func TestMpscStream_GetProducer(t *testing.T) {
	mpsc := NewMpscStream(3, 100)

	p0 := mpsc.GetProducer(0)
	p1 := mpsc.GetProducer(1)
	p2 := mpsc.GetProducer(2)
	pInvalid := mpsc.GetProducer(5)

	assert.NotNil(t, p0)
	assert.NotNil(t, p1)
	assert.NotNil(t, p2)
	assert.Nil(t, pInvalid)
}

func TestMpscStream_GetProducerNegativeIndex(t *testing.T) {
	mpsc := NewMpscStream(3, 100)
	p := mpsc.GetProducer(-1)
	assert.Nil(t, p)
}

func TestMpscStream_StartAndConsume(t *testing.T) {
	mpsc := NewMpscStream(2, 10)
	ctx := context.Background()

	mpsc.Start(ctx)

	// Send from producer 0
	mpsc.GetProducer(0) <- &StreamChunk{Content: "P0", Index: 0}

	// Send from producer 1
	mpsc.GetProducer(1) <- &StreamChunk{Content: "P1", Index: 1}

	// Close producers
	close(mpsc.inputs[0])
	close(mpsc.inputs[1])

	// Consume
	var received []string
	for chunk := range mpsc.Consumer() {
		received = append(received, chunk.Content)
	}

	assert.Len(t, received, 2)
	assert.Contains(t, received, "P0")
	assert.Contains(t, received, "P1")
}

func TestMpscStream_DoubleStart(t *testing.T) {
	mpsc := NewMpscStream(2, 10)
	ctx := context.Background()

	mpsc.Start(ctx)
	mpsc.Start(ctx) // Should be no-op

	assert.True(t, mpsc.started)
}

func TestMpscStream_ContextCancel(t *testing.T) {
	mpsc := NewMpscStream(2, 10)
	ctx, cancel := context.WithCancel(context.Background())

	mpsc.Start(ctx)

	// Send one chunk
	mpsc.GetProducer(0) <- &StreamChunk{Content: "test", Index: 0}

	// Cancel context
	cancel()

	// Consumer should eventually close
	time.Sleep(50 * time.Millisecond)
}

func TestMpscStream_Close(t *testing.T) {
	mpsc := NewMpscStream(2, 10)
	mpsc.Close()

	// Sending should fail (channel closed)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when sending to closed channel")
		}
	}()
	mpsc.inputs[0] <- &StreamChunk{}
}

// ============================================================================
// Stdout Streaming Tests
// ============================================================================

func TestNewStdoutWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	sw := NewStdoutWriter(buf, false)
	assert.NotNil(t, sw)
}

func TestStdoutWriter_Write(t *testing.T) {
	buf := &bytes.Buffer{}
	sw := NewStdoutWriter(buf, false)

	n, err := sw.Write([]byte("Hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "Hello", buf.String())
}

func TestStdoutWriter_WriteLine(t *testing.T) {
	buf := &bytes.Buffer{}
	sw := NewStdoutWriter(buf, false)

	err := sw.WriteLine("Hello, World!")
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!\n", buf.String())
}

func TestStdoutWriter_WriteLineWithNewline(t *testing.T) {
	buf := &bytes.Buffer{}
	sw := NewStdoutWriter(buf, false)

	err := sw.WriteLine("Already has newline\n")
	require.NoError(t, err)
	// Should not add extra newline
	assert.Equal(t, "Already has newline\n", buf.String())
}

func TestStdoutWriter_WriteChunk(t *testing.T) {
	buf := &bytes.Buffer{}
	sw := NewStdoutWriter(buf, false)

	chunk := &StreamChunk{Content: "Chunk content", Index: 0}
	err := sw.WriteChunk(chunk)
	require.NoError(t, err)
	assert.Equal(t, "Chunk content\n", buf.String())
}

func TestStdoutWriter_LineMode(t *testing.T) {
	buf := &bytes.Buffer{}
	sw := NewStdoutWriter(buf, true) // Line mode enabled

	// Write without newline - should buffer
	sw.Write([]byte("Partial"))

	// Write with newline - should flush
	sw.Write([]byte(" line\n"))

	assert.Equal(t, "Partial line\n", buf.String())
}

func TestStdoutWriter_Flush(t *testing.T) {
	buf := &bytes.Buffer{}
	sw := NewStdoutWriter(buf, true) // Line mode

	sw.Write([]byte("Buffered"))
	err := sw.Flush()
	require.NoError(t, err)
	assert.Equal(t, "Buffered", buf.String())
}

// ============================================================================
// Universal Streamer Tests
// ============================================================================

func TestNewUniversalStreamer(t *testing.T) {
	for _, st := range AllStreamingTypes() {
		t.Run(string(st), func(t *testing.T) {
			us := NewUniversalStreamer(st, nil)
			assert.NotNil(t, us)
			assert.Equal(t, st, us.StreamType())
		})
	}
}

func TestUniversalStreamer_WithConfig(t *testing.T) {
	config := &StreamConfig{
		Type:       StreamingTypeSSE,
		BufferSize: 8192,
	}
	us := NewUniversalStreamer(StreamingTypeSSE, config)
	assert.Equal(t, StreamingTypeSSE, us.StreamType())
	assert.Equal(t, 8192, us.config.BufferSize)
}

func TestUniversalStreamer_GetProgress(t *testing.T) {
	us := NewUniversalStreamer(StreamingTypeSSE, nil)
	progress := us.GetProgress()
	assert.NotNil(t, progress)
	assert.Equal(t, int64(0), progress.BytesSent)
	assert.Equal(t, 0, progress.ChunksEmitted)
}

func TestUniversalStreamer_UpdateProgress(t *testing.T) {
	us := NewUniversalStreamer(StreamingTypeSSE, nil)

	us.UpdateProgress(1024, 10, 500)
	progress := us.GetProgress()

	assert.Equal(t, int64(1024), progress.BytesSent)
	assert.Equal(t, 10, progress.ChunksEmitted)
	assert.Equal(t, int64(500), progress.ElapsedMs)
}

func TestUniversalStreamer_UpdateProgressAccumulates(t *testing.T) {
	us := NewUniversalStreamer(StreamingTypeSSE, nil)

	us.UpdateProgress(100, 5, 100)
	us.UpdateProgress(200, 10, 200)

	progress := us.GetProgress()
	assert.Equal(t, int64(300), progress.BytesSent)
	assert.Equal(t, 15, progress.ChunksEmitted)
	assert.Equal(t, int64(200), progress.ElapsedMs) // Last value, not accumulated
}

func TestUniversalStreamer_ProgressHandler(t *testing.T) {
	var callCount int
	handler := func(p *StreamProgress) {
		callCount++
	}

	config := &StreamConfig{
		Type:            StreamingTypeSSE,
		EnableProgress:  true,
		ProgressHandler: handler,
	}
	us := NewUniversalStreamer(StreamingTypeSSE, config)

	us.UpdateProgress(100, 1, 100)
	assert.Equal(t, 1, callCount)

	us.UpdateProgress(200, 2, 200)
	assert.Equal(t, 2, callCount)
}

func TestUniversalStreamer_ProgressHandlerDisabled(t *testing.T) {
	var callCount int
	handler := func(p *StreamProgress) {
		callCount++
	}

	config := &StreamConfig{
		Type:            StreamingTypeSSE,
		EnableProgress:  false, // Disabled
		ProgressHandler: handler,
	}
	us := NewUniversalStreamer(StreamingTypeSSE, config)

	us.UpdateProgress(100, 1, 100)
	assert.Equal(t, 0, callCount) // Handler should not be called
}

// ============================================================================
// WebSocket Tests (Mock)
// ============================================================================

func TestWebSocketWriter_WithMockConn(t *testing.T) {
	// Start a test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Echo messages back
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			conn.WriteMessage(msgType, msg)
		}
	}))
	defer server.Close()

	// Connect as client
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Create WebSocket writer
	wsw := NewWebSocketWriter(conn, nil)
	assert.NotNil(t, wsw)

	// Test WriteMessage
	err = wsw.WriteMessage([]byte("Hello"))
	require.NoError(t, err)

	// Read echo
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "Hello", string(msg))
}

func TestWebSocketWriter_WriteJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			conn.WriteMessage(websocket.TextMessage, msg)
		}
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	defer conn.Close()

	wsw := NewWebSocketWriter(conn, nil)

	// Test WriteJSON
	data := map[string]interface{}{
		"type":    "chunk",
		"content": "Hello",
	}
	err = wsw.WriteJSON(data)
	require.NoError(t, err)

	// Read echo
	var received map[string]interface{}
	err = conn.ReadJSON(&received)
	require.NoError(t, err)
	assert.Equal(t, "chunk", received["type"])
	assert.Equal(t, "Hello", received["content"])
}

func TestWebSocketWriter_WritePing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Handle ping/pong
		conn.SetPingHandler(func(appData string) error {
			return conn.WriteMessage(websocket.PongMessage, []byte(appData))
		})

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	defer conn.Close()

	wsw := NewWebSocketWriter(conn, nil)

	// Test WritePing
	err = wsw.WritePing()
	require.NoError(t, err)
}

func TestWebSocketWriter_Close(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)

	wsw := NewWebSocketWriter(conn, nil)

	err = wsw.Close()
	require.NoError(t, err)
}

// ============================================================================
// Concurrent Safety Tests
// ============================================================================

func TestSSEWriter_RaceSafety(t *testing.T) {
	recorder := httptest.NewRecorder()
	sse, err := NewSSEWriter(recorder)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func(idx int) {
			defer wg.Done()
			sse.WriteData("data")
		}(i)
		go func(idx int) {
			defer wg.Done()
			sse.WriteEvent("event", "payload", "")
		}(i)
		go func(idx int) {
			defer wg.Done()
			sse.WriteHeartbeat()
		}(i)
	}
	wg.Wait()
}

func TestJSONLWriter_RaceSafety(t *testing.T) {
	buf := &bytes.Buffer{}
	jw := NewJSONLWriter(buf)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			jw.WriteChunk(&StreamChunk{Content: "test", Index: idx})
		}(i)
	}
	wg.Wait()
}

func TestAsyncGenerator_RaceSafety(t *testing.T) {
	ag := NewAsyncGenerator(100)
	ctx := context.Background()

	var wg sync.WaitGroup

	// Multiple producers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				ag.YieldContent("chunk", idx*10+j)
			}
		}(i)
	}

	// Consumer
	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		for {
			_, err := ag.Next(ctx)
			if err != nil {
				return
			}
			count++
			if count >= 100 {
				ag.Close(nil)
				return
			}
		}
	}()

	wg.Wait()
}

func TestStdoutWriter_RaceSafety(t *testing.T) {
	buf := &bytes.Buffer{}
	sw := NewStdoutWriter(buf, false)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			sw.Write([]byte("data"))
		}(i)
		go func(idx int) {
			defer wg.Done()
			sw.WriteLine("line")
		}(i)
	}
	wg.Wait()
}

func TestUniversalStreamer_RaceSafety(t *testing.T) {
	us := NewUniversalStreamer(StreamingTypeSSE, nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			us.UpdateProgress(int64(idx), 1, int64(idx*10))
		}(i)
		go func(idx int) {
			defer wg.Done()
			us.GetProgress()
		}(i)
	}
	wg.Wait()
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkSSEWriter_WriteData(b *testing.B) {
	recorder := httptest.NewRecorder()
	sse, _ := NewSSEWriter(recorder)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sse.WriteData("benchmark data")
	}
}

func BenchmarkJSONLWriter_WriteLine(b *testing.B) {
	buf := &bytes.Buffer{}
	jw := NewJSONLWriter(buf)
	data := map[string]interface{}{"key": "value", "count": 42}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jw.WriteLine(data)
	}
}

func BenchmarkAsyncGenerator_YieldNext(b *testing.B) {
	ag := NewAsyncGenerator(b.N)
	ctx := context.Background()

	b.ResetTimer()
	go func() {
		for i := 0; i < b.N; i++ {
			ag.YieldContent("content", i)
		}
		ag.Close(nil)
	}()

	for i := 0; i < b.N; i++ {
		ag.Next(ctx)
	}
}

func BenchmarkStdoutWriter_WriteLine(b *testing.B) {
	buf := &bytes.Buffer{}
	sw := NewStdoutWriter(buf, false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sw.WriteLine("benchmark line")
	}
}

// ============================================================================
// Helper Types
// ============================================================================

// nonFlushingWriter is a writer that doesn't implement http.Flusher
type nonFlushingWriter struct{}

func (w *nonFlushingWriter) Header() http.Header {
	return http.Header{}
}

func (w *nonFlushingWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (w *nonFlushingWriter) WriteHeader(statusCode int) {}
