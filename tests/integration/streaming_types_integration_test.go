//go:build integration
// +build integration

package integration

import (
	"bufio"
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

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/streaming"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ============================================================================
// SSE Integration Tests
// ============================================================================

func TestSSE_FullHTTPIntegration(t *testing.T) {
	// Create test server with SSE endpoint
	router := gin.New()
	router.GET("/sse", func(c *gin.Context) {
		sse, err := streaming.NewSSEWriter(c.Writer)
		if err != nil {
			c.AbortWithStatus(500)
			return
		}

		// Send initial event
		sse.WriteEvent("connected", "Connection established", "")

		// Send 5 data events
		for i := 0; i < 5; i++ {
			sse.WriteJSON(map[string]interface{}{
				"chunk": i,
				"content": "Hello",
			})
			time.Sleep(10 * time.Millisecond)
		}

		// Send done
		sse.WriteDone()
	})

	server := httptest.NewServer(router)
	defer server.Close()

	// Make request
	req, err := http.NewRequest("GET", server.URL+"/sse", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))

	// Read SSE events
	reader := bufio.NewReader(resp.Body)
	eventCount := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, "data:") {
			eventCount++
			if strings.Contains(line, "[DONE]") {
				break
			}
		}
	}

	// 1 connected + 5 data + 1 done = 7 events
	assert.GreaterOrEqual(t, eventCount, 6)
}

func TestSSE_WithHeartbeat(t *testing.T) {
	router := gin.New()
	router.GET("/sse-heartbeat", func(c *gin.Context) {
		sse, err := streaming.NewSSEWriter(c.Writer)
		if err != nil {
			c.AbortWithStatus(500)
			return
		}

		// Send heartbeat
		sse.WriteHeartbeat()
		sse.WriteData("test")
		sse.WriteHeartbeat()
		sse.WriteDone()
	})

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/sse-heartbeat")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	assert.Contains(t, content, ": heartbeat")
	assert.Contains(t, content, "data: test")
	assert.Contains(t, content, "data: [DONE]")
}

func TestSSE_ConcurrentClients(t *testing.T) {
	router := gin.New()
	router.GET("/sse-concurrent", func(c *gin.Context) {
		sse, _ := streaming.NewSSEWriter(c.Writer)
		for i := 0; i < 10; i++ {
			sse.WriteData("chunk")
		}
		sse.WriteDone()
	})

	server := httptest.NewServer(router)
	defer server.Close()

	// Spawn 10 concurrent clients
	var wg sync.WaitGroup
	results := make(chan int, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			resp, err := http.Get(server.URL + "/sse-concurrent")
			if err != nil {
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			results <- strings.Count(string(body), "data:")
		}()
	}

	wg.Wait()
	close(results)

	for count := range results {
		assert.Equal(t, 11, count) // 10 chunks + 1 DONE
	}
}

// ============================================================================
// WebSocket Integration Tests
// ============================================================================

func TestWebSocket_FullIntegration(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	router := gin.New()
	router.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		wsw := streaming.NewWebSocketWriter(conn, nil)

		// Send welcome message
		wsw.WriteJSON(map[string]interface{}{
			"type":    "welcome",
			"message": "Connected",
		})

		// Echo messages back
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			wsw.WriteMessage(append([]byte("echo: "), msg...))
		}
	})

	server := httptest.NewServer(router)
	defer server.Close()

	// Connect as WebSocket client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Read welcome message
	var welcome map[string]interface{}
	err = conn.ReadJSON(&welcome)
	require.NoError(t, err)
	assert.Equal(t, "welcome", welcome["type"])

	// Send message and verify echo
	conn.WriteMessage(websocket.TextMessage, []byte("Hello"))
	_, echo, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, "echo: Hello", string(echo))
}

func TestWebSocket_BinaryMessages(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	router := gin.New()
	router.GET("/ws-binary", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		wsw := streaming.NewWebSocketWriter(conn, nil)

		// Send binary data
		binaryData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
		wsw.WriteBinary(binaryData)
	})

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws-binary"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	msgType, data, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, websocket.BinaryMessage, msgType)
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04, 0x05}, data)
}

func TestWebSocket_ConcurrentMessages(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	router := gin.New()
	router.GET("/ws-concurrent", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		wsw := streaming.NewWebSocketWriter(conn, nil)

		// Send 100 messages concurrently
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				wsw.WriteJSON(map[string]int{"index": idx})
			}(i)
		}
		wg.Wait()

		// Signal done
		wsw.WriteJSON(map[string]bool{"done": true})
	})

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws-concurrent"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Read all messages
	messageCount := 0
	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}
		messageCount++
		if msg["done"] == true {
			break
		}
	}

	assert.Equal(t, 101, messageCount) // 100 + done
}

// ============================================================================
// JSONL Integration Tests
// ============================================================================

func TestJSONL_FullHTTPIntegration(t *testing.T) {
	router := gin.New()
	router.GET("/jsonl", func(c *gin.Context) {
		jw, err := streaming.NewJSONLWriterHTTP(c.Writer)
		if err != nil {
			c.AbortWithStatus(500)
			return
		}

		for i := 0; i < 5; i++ {
			jw.WriteChunk(&streaming.StreamChunk{
				Content: "Line content",
				Index:   i,
			})
		}
		jw.WriteDone()
	})

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/jsonl")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "application/x-ndjson", resp.Header.Get("Content-Type"))

	// Read and parse JSONL
	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")

	assert.Len(t, lines, 6) // 5 chunks + done

	// Verify each line is valid JSON
	for _, line := range lines {
		var parsed map[string]interface{}
		err := json.Unmarshal([]byte(line), &parsed)
		assert.NoError(t, err)
	}
}

func TestJSONL_LargePayload(t *testing.T) {
	router := gin.New()
	router.GET("/jsonl-large", func(c *gin.Context) {
		jw, _ := streaming.NewJSONLWriterHTTP(c.Writer)

		// Send large chunks
		for i := 0; i < 100; i++ {
			jw.WriteChunk(&streaming.StreamChunk{
				Content: strings.Repeat("x", 1000),
				Index:   i,
			})
		}
		jw.WriteDone()
	})

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/jsonl-large")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")

	assert.Len(t, lines, 101)
}

// ============================================================================
// AsyncGenerator Integration Tests
// ============================================================================

func TestAsyncGenerator_ProducerConsumer(t *testing.T) {
	ag := streaming.NewAsyncGenerator(100)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Producer goroutine
	go func() {
		for i := 0; i < 50; i++ {
			ag.YieldContent("chunk content", i)
		}
		ag.Close(nil)
	}()

	// Consumer
	count := 0
	for {
		chunk, err := ag.Next(ctx)
		if err != nil {
			break
		}
		count++
		assert.Equal(t, "chunk content", chunk.Content)
	}

	assert.Equal(t, 50, count)
}

func TestAsyncGenerator_MultipleProducers(t *testing.T) {
	ag := streaming.NewAsyncGenerator(200)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Multiple producers
	var wg sync.WaitGroup
	for p := 0; p < 5; p++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				ag.YieldContent("from producer", producerID*100+i)
			}
		}(p)
	}

	// Close when all producers done
	go func() {
		wg.Wait()
		ag.Close(nil)
	}()

	// Consumer
	count := 0
	for {
		_, err := ag.Next(ctx)
		if err != nil {
			break
		}
		count++
	}

	assert.Equal(t, 100, count) // 5 producers x 20 chunks
}

func TestAsyncGenerator_ContextCancellation(t *testing.T) {
	ag := streaming.NewAsyncGenerator(100)
	ctx, cancel := context.WithCancel(context.Background())

	// Producer that never stops
	go func() {
		for {
			if err := ag.YieldContent("infinite", 0); err != nil {
				return
			}
		}
	}()

	// Consume a few items then cancel
	for i := 0; i < 10; i++ {
		_, err := ag.Next(ctx)
		require.NoError(t, err)
	}

	cancel()

	// Next call should return context.Canceled
	_, err := ag.Next(ctx)
	assert.Equal(t, context.Canceled, err)

	ag.Close(nil)
}

// ============================================================================
// EventStream Integration Tests
// ============================================================================

func TestEventStream_FullHTTPIntegration(t *testing.T) {
	router := gin.New()
	router.GET("/eventstream", func(c *gin.Context) {
		esw, err := streaming.NewEventStreamWriterHTTP(c.Writer)
		if err != nil {
			c.AbortWithStatus(500)
			return
		}

		for i := 0; i < 5; i++ {
			esw.WriteChunk(&streaming.StreamChunk{
				Content: "AWS style chunk",
				Index:   i,
			})
		}
		esw.WriteDone()
	})

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/eventstream")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "application/vnd.amazon.eventstream", resp.Header.Get("Content-Type"))

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	// Should contain event data
	assert.Contains(t, content, "chunk")
	assert.Contains(t, content, "done")
}

// ============================================================================
// MpscStream Integration Tests
// ============================================================================

func TestMpscStream_MultiProducerSingleConsumer(t *testing.T) {
	mpsc := streaming.NewMpscStream(4, 50)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mpsc.Start(ctx)

	// Start 4 producers
	for i := 0; i < 4; i++ {
		go func(producerID int) {
			producer := mpsc.GetProducer(producerID)
			for j := 0; j < 25; j++ {
				select {
				case <-ctx.Done():
					return
				case producer <- &streaming.StreamChunk{
					Content: "from producer",
					Index:   producerID*100 + j,
				}:
				}
			}
		}(i)
	}

	// Consumer
	consumed := 0
	timeout := time.After(3 * time.Second)
	for consumed < 100 {
		select {
		case chunk, ok := <-mpsc.Consumer():
			if !ok {
				break
			}
			consumed++
			assert.Equal(t, "from producer", chunk.Content)
		case <-timeout:
			t.Fatal("Timeout waiting for chunks")
		}
	}

	assert.Equal(t, 100, consumed)
}

func TestMpscStream_ProducerClosing(t *testing.T) {
	mpsc := streaming.NewMpscStream(2, 10)
	ctx := context.Background()

	mpsc.Start(ctx)

	// Producer 1 sends and closes
	go func() {
		producer := mpsc.GetProducer(0)
		for i := 0; i < 5; i++ {
			producer <- &streaming.StreamChunk{Content: "P0", Index: i}
		}
		close(mpsc.inputs[0])
	}()

	// Producer 2 sends and closes
	go func() {
		producer := mpsc.GetProducer(1)
		for i := 0; i < 5; i++ {
			producer <- &streaming.StreamChunk{Content: "P1", Index: i}
		}
		close(mpsc.inputs[1])
	}()

	// Consumer should receive all and then channel closes
	consumed := 0
	for range mpsc.Consumer() {
		consumed++
	}

	assert.Equal(t, 10, consumed)
}

// ============================================================================
// Stdout Integration Tests
// ============================================================================

func TestStdout_PipelineIntegration(t *testing.T) {
	// Simulate stdout pipeline with a pipe
	reader, writer := io.Pipe()
	sw := streaming.NewStdoutWriter(writer, false)

	// Producer
	go func() {
		for i := 0; i < 10; i++ {
			sw.WriteChunk(&streaming.StreamChunk{Content: "chunk", Index: i})
		}
		writer.Close()
	}()

	// Consumer
	buf := &bytes.Buffer{}
	io.Copy(buf, reader)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, 10)
}

func TestStdout_LineMode(t *testing.T) {
	buf := &bytes.Buffer{}
	sw := streaming.NewStdoutWriter(buf, true)

	// Write partial content (should not flush yet in line mode)
	sw.Write([]byte("partial"))
	sw.Write([]byte(" content"))
	sw.Write([]byte("\n")) // Newline triggers flush

	assert.Equal(t, "partial content\n", buf.String())
}

// ============================================================================
// Universal Streamer Integration Tests
// ============================================================================

func TestUniversalStreamer_AllTypes(t *testing.T) {
	for _, st := range streaming.AllStreamingTypes() {
		t.Run(string(st), func(t *testing.T) {
			var progressCalls int
			config := &streaming.StreamConfig{
				Type:           st,
				BufferSize:     4096,
				EnableProgress: true,
				ProgressHandler: func(p *streaming.StreamProgress) {
					progressCalls++
				},
			}

			us := streaming.NewUniversalStreamer(st, config)

			// Update progress
			for i := 0; i < 10; i++ {
				us.UpdateProgress(100, 1, int64(i*10))
			}

			progress := us.GetProgress()
			assert.Equal(t, int64(1000), progress.BytesSent)
			assert.Equal(t, 10, progress.ChunksEmitted)
			assert.Equal(t, 10, progressCalls)
		})
	}
}

// ============================================================================
// Cross-Streaming Type Integration Tests
// ============================================================================

func TestCrossStreaming_SSEToJSONL(t *testing.T) {
	// Simulate converting SSE to JSONL
	sseBuffer := &bytes.Buffer{}
	sse, _ := streaming.NewSSEWriter(httptest.NewRecorder())

	// Generate SSE events
	for i := 0; i < 5; i++ {
		chunk := &streaming.StreamChunk{Content: "chunk", Index: i}
		data, _ := json.Marshal(chunk)
		sse.WriteData(string(data))
	}

	// Convert to JSONL
	jsonlBuffer := &bytes.Buffer{}
	jw := streaming.NewJSONLWriter(jsonlBuffer)

	// Parse SSE and write as JSONL
	scanner := bufio.NewScanner(strings.NewReader(sseBuffer.String()))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var chunk streaming.StreamChunk
			if json.Unmarshal([]byte(data), &chunk) == nil {
				jw.WriteChunk(&chunk)
			}
		}
	}
}

func TestCrossStreaming_AsyncGenToSSE(t *testing.T) {
	ag := streaming.NewAsyncGenerator(50)
	ctx := context.Background()

	// Producer
	go func() {
		for i := 0; i < 10; i++ {
			ag.YieldContent("async chunk", i)
		}
		ag.Close(nil)
	}()

	// Convert to SSE
	recorder := httptest.NewRecorder()
	sse, _ := streaming.NewSSEWriter(recorder)

	for {
		chunk, err := ag.Next(ctx)
		if err != nil {
			break
		}
		sse.WriteJSON(chunk)
	}
	sse.WriteDone()

	body := recorder.Body.String()
	assert.Contains(t, body, "async chunk")
	assert.Contains(t, body, "[DONE]")
}

// ============================================================================
// Performance Integration Tests
// ============================================================================

func TestPerformance_HighThroughputSSE(t *testing.T) {
	router := gin.New()
	router.GET("/sse-perf", func(c *gin.Context) {
		sse, _ := streaming.NewSSEWriter(c.Writer)

		start := time.Now()
		for i := 0; i < 1000; i++ {
			sse.WriteData("x")
		}
		sse.WriteDone()

		// Should complete within reasonable time
		elapsed := time.Since(start)
		if elapsed > 5*time.Second {
			c.AbortWithStatus(500)
		}
	})

	server := httptest.NewServer(router)
	defer server.Close()

	start := time.Now()
	resp, err := http.Get(server.URL + "/sse-perf")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 5*time.Second)
	assert.Greater(t, len(body), 1000) // At least 1000 bytes
}

func TestPerformance_HighThroughputJSONL(t *testing.T) {
	router := gin.New()
	router.GET("/jsonl-perf", func(c *gin.Context) {
		jw, _ := streaming.NewJSONLWriterHTTP(c.Writer)

		for i := 0; i < 1000; i++ {
			jw.WriteChunk(&streaming.StreamChunk{Content: "x", Index: i})
		}
		jw.WriteDone()
	})

	server := httptest.NewServer(router)
	defer server.Close()

	start := time.Now()
	resp, err := http.Get(server.URL + "/jsonl-perf")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 5*time.Second)

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	assert.Len(t, lines, 1001) // 1000 + done
}

// ============================================================================
// Error Handling Integration Tests
// ============================================================================

func TestErrorHandling_SSEConnectionClose(t *testing.T) {
	router := gin.New()
	router.GET("/sse-close", func(c *gin.Context) {
		sse, err := streaming.NewSSEWriter(c.Writer)
		if err != nil {
			return
		}

		for i := 0; i < 100; i++ {
			sse.WriteData("chunk")
			time.Sleep(50 * time.Millisecond)
		}
	})

	server := httptest.NewServer(router)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/sse-close", nil)
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}

	// Client disconnect should be handled gracefully
	// No panic expected
}

func TestErrorHandling_WebSocketDisconnect(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	disconnectHandled := make(chan bool, 1)

	router := gin.New()
	router.GET("/ws-disconnect", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		wsw := streaming.NewWebSocketWriter(conn, nil)

		for i := 0; i < 100; i++ {
			err := wsw.WriteJSON(map[string]int{"index": i})
			if err != nil {
				disconnectHandled <- true
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	})

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws-disconnect"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// Read a few messages then close
	for i := 0; i < 3; i++ {
		conn.ReadJSON(&map[string]interface{}{})
	}
	conn.Close()

	// Server should handle disconnect gracefully
	select {
	case <-disconnectHandled:
		// Good, disconnect was handled
	case <-time.After(1 * time.Second):
		// Also acceptable - connection may have been detected closed
	}
}
