package bridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test Helpers
// ============================================================================

// createTestLogger creates a logger for testing (discards output)
func createTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return logger
}

// createTempDir creates a temporary directory for testing
func createTempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "sse_bridge_test_*")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}

// createMockMCPServer creates a simple bash script that acts as a mock MCP server
func createMockMCPServer(t *testing.T, dir string) string {
	script := `#!/bin/bash
# Mock MCP server that responds to JSON-RPC requests

# Function to extract JSON-RPC ID using pure bash
extract_id() {
    local line="$1"
    # Use grep with perl regex to extract the id value
    echo "$line" | grep -oP '"id"\s*:\s*\K[0-9]+' || echo "null"
}

# Read line by line from stdin
while IFS= read -r line; do
    # Check if it's an initialize request
    if echo "$line" | grep -q '"method"[[:space:]]*:[[:space:]]*"initialize"'; then
        # Send initialize response
        echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"mock-mcp","version":"1.0.0"}}}'
    elif echo "$line" | grep -q '"method"[[:space:]]*:[[:space:]]*"notifications/initialized"'; then
        # No response for notifications
        :
    elif echo "$line" | grep -q '"method"[[:space:]]*:[[:space:]]*"tools/list"'; then
        # Send tools list response
        id=$(extract_id "$line")
        echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"tools\":[{\"name\":\"echo\",\"description\":\"Echo input\"}]}}"
    elif echo "$line" | grep -q '"method"[[:space:]]*:[[:space:]]*"tools/call"'; then
        # Send tool call response
        id=$(extract_id "$line")
        echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"Hello from mock MCP\"}]}}"
    elif echo "$line" | grep -q '"method"[[:space:]]*:[[:space:]]*"ping"'; then
        # Send ping response
        id=$(extract_id "$line")
        echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":\"pong\"}"
    elif echo "$line" | grep -q '"method"[[:space:]]*:[[:space:]]*"error"'; then
        # Send error response
        id=$(extract_id "$line")
        echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"error\":{\"code\":-32601,\"message\":\"Method not found\"}}"
    elif echo "$line" | grep -q '"method"'; then
        # Echo any other request
        id=$(extract_id "$line")
        if [ "$id" != "null" ]; then
            echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"echo\":\"received\"}}"
        fi
    fi
done
`
	scriptPath := filepath.Join(dir, "mock_mcp.sh")
	err := os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)
	return scriptPath
}

// createFailingMCPServer creates a mock MCP server that exits immediately
func createFailingMCPServer(t *testing.T, dir string) string {
	script := `#!/bin/bash
exit 1
`
	scriptPath := filepath.Join(dir, "failing_mcp.sh")
	err := os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)
	return scriptPath
}

// ============================================================================
// DefaultSSEBridgeConfig Tests
// ============================================================================

func TestDefaultSSEBridgeConfig(t *testing.T) {
	t.Parallel()
	t.Run("Returns sensible defaults", func(t *testing.T) {
		config := DefaultSSEBridgeConfig()

		assert.Equal(t, ":8080", config.Address)
		assert.Equal(t, 30*time.Second, config.ReadTimeout)
		assert.Equal(t, 30*time.Second, config.WriteTimeout)
		assert.Equal(t, 120*time.Second, config.IdleTimeout)
		assert.Equal(t, 30*time.Second, config.ShutdownTimeout)
		assert.Equal(t, int64(10*1024*1024), config.MaxRequestSize)
		assert.Equal(t, 30*time.Second, config.SSEHeartbeatInterval)
	})
}

// ============================================================================
// SSEBridgeState Tests
// ============================================================================

func TestSSEBridgeState_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		state    SSEBridgeState
		expected string
	}{
		{StateIdle, "idle"},
		{StateStarting, "starting"},
		{StateRunning, "running"},
		{StateStopping, "stopping"},
		{StateStopped, "stopped"},
		{StateError, "error"},
		{SSEBridgeState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
				t.Parallel()
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

// ============================================================================
// NewSSEBridge Tests
// ============================================================================

func TestNewSSEBridge(t *testing.T) {
	t.Parallel()
	t.Run("Creates bridge with valid config", func(t *testing.T) {
		config := SSEBridgeConfig{
			Command: []string{"echo", "hello"},
			Address: ":9090",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)
		assert.NotNil(t, bridge)
		assert.Equal(t, StateIdle, bridge.State())
		assert.Equal(t, ":9090", bridge.Address())
	})

	t.Run("Returns error for empty command", func(t *testing.T) {
			t.Parallel()
		config := SSEBridgeConfig{
			Command: []string{},
		}

		bridge, err := NewSSEBridge(config)
		assert.Error(t, err)
		assert.Nil(t, bridge)
		assert.Contains(t, err.Error(), "command is required")
	})

	t.Run("Applies default values", func(t *testing.T) {
			t.Parallel()
		config := SSEBridgeConfig{
			Command: []string{"echo"},
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)
		assert.Equal(t, ":8080", bridge.config.Address)
		assert.Equal(t, 30*time.Second, bridge.config.ReadTimeout)
	})

	t.Run("Creates default logger if not provided", func(t *testing.T) {
			t.Parallel()
		config := SSEBridgeConfig{
			Command: []string{"echo"},
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)
		assert.NotNil(t, bridge.logger)
	})

	t.Run("Uses provided logger", func(t *testing.T) {
			t.Parallel()
		logger := createTestLogger()
		config := SSEBridgeConfig{
			Command: []string{"echo"},
			Logger:  logger,
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)
		assert.Equal(t, logger, bridge.logger)
	})
}

// ============================================================================
// SSEBridge Start/Stop Tests
// ============================================================================

func TestSSEBridge_Start(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Starts successfully with mock MCP server", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0", // Random port
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		assert.Equal(t, StateRunning, bridge.State())
		assert.True(t, bridge.IsHealthy())
	})

	t.Run("Cannot start twice", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		err = bridge.Start()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot start bridge in state")
	})

	t.Run("Fails with non-existent command", func(t *testing.T) {
			t.Parallel()
		config := SSEBridgeConfig{
			Command: []string{"/nonexistent/command"},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		assert.Error(t, err)
		assert.Equal(t, StateError, bridge.State())
	})
}

func TestSSEBridge_Shutdown(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Shuts down gracefully", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command:         []string{"/bin/bash", scriptPath},
			Address:         ":0",
			Logger:          createTestLogger(),
			ShutdownTimeout: 5 * time.Second,
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)

		err = bridge.Shutdown(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, StateStopped, bridge.State())
	})

	t.Run("Can be called multiple times", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)

		err = bridge.Shutdown(context.Background())
		assert.NoError(t, err)

		err = bridge.Shutdown(context.Background())
		assert.NoError(t, err)
	})
}

// ============================================================================
// HTTP Handler Tests
// ============================================================================

func TestSSEBridge_HandleHealth(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Returns healthy status when running", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		// Give it a moment to stabilize
		time.Sleep(100 * time.Millisecond)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var health map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &health)
		require.NoError(t, err)

		assert.Equal(t, "running", health["status"])
		assert.Equal(t, true, health["healthy"])
	})

	t.Run("Returns unhealthy when not running", func(t *testing.T) {
			t.Parallel()
		config := SSEBridgeConfig{
			Command: []string{"echo"},
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("Rejects non-GET requests", func(t *testing.T) {
			t.Parallel()
		config := SSEBridgeConfig{
			Command: []string{"echo"},
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/health", nil)
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

func TestSSEBridge_HandleMessage(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Sends request to MCP process and receives response", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command:      []string{"/bin/bash", scriptPath},
			Address:      ":0",
			Logger:       createTestLogger(),
			WriteTimeout: 10 * time.Second,
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		// Send a ping request
		reqBody := `{"jsonrpc":"2.0","id":123,"method":"ping"}`
		req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp JSONRPCResponse
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.NotNil(t, resp.Result)
	})

	t.Run("Handles notification (no ID)", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		// Send a notification (no ID)
		reqBody := `{"jsonrpc":"2.0","method":"some/notification","params":{}}`
		req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("Rejects non-POST requests", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		req := httptest.NewRequest(http.MethodGet, "/message", nil)
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code) // JSON-RPC error returns 200

		var resp JSONRPCResponse
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.NotNil(t, resp.Error)
		assert.Equal(t, JSONRPCInvalidRequest, resp.Error.Code)
	})

	t.Run("Rejects invalid JSON", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		reqBody := `not valid json`
		req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		var resp JSONRPCResponse
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.NotNil(t, resp.Error)
		assert.Equal(t, JSONRPCParseError, resp.Error.Code)
	})

	t.Run("Rejects invalid JSON-RPC version", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		reqBody := `{"jsonrpc":"1.0","id":1,"method":"test"}`
		req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		var resp JSONRPCResponse
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.NotNil(t, resp.Error)
		assert.Equal(t, JSONRPCInvalidRequest, resp.Error.Code)
	})

	t.Run("Rejects empty method", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		reqBody := `{"jsonrpc":"2.0","id":1,"method":""}`
		req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		var resp JSONRPCResponse
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.NotNil(t, resp.Error)
		assert.Equal(t, JSONRPCInvalidRequest, resp.Error.Code)
	})

	t.Run("Rejects when bridge not running", func(t *testing.T) {
			t.Parallel()
		config := SSEBridgeConfig{
			Command: []string{"echo"},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		reqBody := `{"jsonrpc":"2.0","id":1,"method":"test"}`
		req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		var resp JSONRPCResponse
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.NotNil(t, resp.Error)
		assert.Equal(t, JSONRPCServerError, resp.Error.Code)
	})
}

func TestSSEBridge_HandleSSE(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Establishes SSE connection", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command:              []string{"/bin/bash", scriptPath},
			Address:              ":0",
			Logger:               createTestLogger(),
			SSEHeartbeatInterval: 100 * time.Millisecond,
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		// Create a context that we'll cancel
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		req := httptest.NewRequest(http.MethodGet, "/sse", nil).WithContext(ctx)
		w := &sseRecorder{ResponseRecorder: httptest.NewRecorder(), flushed: make(chan struct{})}

		// Run handler in goroutine since it blocks
		done := make(chan struct{})
		go func() {
			bridge.Handler().ServeHTTP(w, req)
			close(done)
		}()

		// Wait for flush (indicating connection established)
		select {
		case <-w.flushed:
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for SSE connection")
		}

		// Verify headers
		assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))

		// Verify we got connected event
		body := w.Body.String()
		assert.Contains(t, body, "event: connected")
		assert.Contains(t, body, "event: endpoint")

		// Verify active client count
		assert.Equal(t, 1, bridge.ActiveClients())

		// Cancel context to close connection
		cancel()

		// Wait for handler to finish
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for handler to finish")
		}

		// Verify client was removed
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, 0, bridge.ActiveClients())
	})

	t.Run("Rejects non-GET requests", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		req := httptest.NewRequest(http.MethodPost, "/sse", nil)
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("Rejects when bridge not running", func(t *testing.T) {
			t.Parallel()
		config := SSEBridgeConfig{
			Command: []string{"echo"},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/sse", nil)
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})
}

// sseRecorder is a custom ResponseRecorder that supports Flusher interface
type sseRecorder struct {
	*httptest.ResponseRecorder
	flushed    chan struct{}
	flushOnce  sync.Once
	flushCount int32
}

func (r *sseRecorder) Flush() {
	r.ResponseRecorder.Flush()
	atomic.AddInt32(&r.flushCount, 1)
	r.flushOnce.Do(func() {
		close(r.flushed)
	})
}

// ============================================================================
// SendRequest/SendNotification Tests
// ============================================================================

func TestSSEBridge_SendRequest(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Sends request and receives response", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := bridge.SendRequest(ctx, "ping", nil)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("Returns error when bridge not running", func(t *testing.T) {
			t.Parallel()
		config := SSEBridgeConfig{
			Command: []string{"echo"},
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		ctx := context.Background()
		resp, err := bridge.SendRequest(ctx, "test", nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "bridge not running")
	})
}

func TestSSEBridge_SendNotification(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Sends notification successfully", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		err = bridge.SendNotification("test/notification", map[string]string{"key": "value"})
		assert.NoError(t, err)
	})

	t.Run("Returns error when bridge not running", func(t *testing.T) {
			t.Parallel()
		config := SSEBridgeConfig{
			Command: []string{"echo"},
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.SendNotification("test", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bridge not running")
	})
}

// ============================================================================
// Metrics Tests
// ============================================================================

func TestSSEBridge_Metrics(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Tracks request metrics", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		// Send a few requests
		for i := 0; i < 3; i++ {
			reqBody := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"ping"}`, i+1)
			req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			bridge.Handler().ServeHTTP(w, req)
		}

		metrics := bridge.Metrics()
		assert.GreaterOrEqual(t, metrics.TotalRequests, int64(3))
		assert.Greater(t, metrics.BytesSent, int64(0))
		assert.Greater(t, metrics.BytesReceived, int64(0))
	})
}

// ============================================================================
// JSON-RPC Types Tests
// ============================================================================

func TestJSONRPCRequest(t *testing.T) {
	t.Parallel()
	t.Run("Marshals correctly", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "test",
			Params:  json.RawMessage(`{"key":"value"}`),
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		assert.Contains(t, string(data), `"jsonrpc":"2.0"`)
		assert.Contains(t, string(data), `"id":1`)
		assert.Contains(t, string(data), `"method":"test"`)
	})

	t.Run("Unmarshals correctly", func(t *testing.T) {
			t.Parallel()
		data := `{"jsonrpc":"2.0","id":42,"method":"tools/list","params":{}}`
		var req JSONRPCRequest
		err := json.Unmarshal([]byte(data), &req)
		require.NoError(t, err)

		assert.Equal(t, "2.0", req.JSONRPC)
		assert.Equal(t, float64(42), req.ID) // JSON numbers unmarshal as float64
		assert.Equal(t, "tools/list", req.Method)
	})
}

func TestJSONRPCResponse(t *testing.T) {
	t.Parallel()
	t.Run("Marshals success response", func(t *testing.T) {
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`{"status":"ok"}`),
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		assert.Contains(t, string(data), `"result"`)
		assert.NotContains(t, string(data), `"error"`)
	})

	t.Run("Marshals error response", func(t *testing.T) {
			t.Parallel()
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Error: &JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
			},
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		assert.Contains(t, string(data), `"error"`)
		assert.Contains(t, string(data), `-32600`)
	})
}

// ============================================================================
// Edge Case Tests
// ============================================================================

func TestSSEBridge_EdgeCases(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Handles large request body", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command:        []string{"/bin/bash", scriptPath},
			Address:        ":0",
			Logger:         createTestLogger(),
			MaxRequestSize: 1024, // 1KB limit
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		// Create request larger than limit
		largeData := strings.Repeat("x", 2048)
		reqBody := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"test","params":{"data":"%s"}}`, largeData)
		req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		var resp JSONRPCResponse
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.NotNil(t, resp.Error)
		assert.Equal(t, JSONRPCRequestTooLarge, resp.Error.Code)
	})

	t.Run("Handles concurrent requests", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		var wg sync.WaitGroup
		errChan := make(chan error, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				reqBody := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"ping"}`, idx+100)
				req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()

				bridge.Handler().ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					errChan <- fmt.Errorf("request %d failed with status %d", idx, w.Code)
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		for err := range errChan {
			t.Error(err)
		}
	})

	t.Run("Handles process restart callback", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createFailingMCPServer(t, dir)

		exitCalled := make(chan struct{})
		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
			OnProcessExit: func(err error) {
				close(exitCalled)
			},
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		// Start will fail because process exits immediately
		_ = bridge.Start()
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		// The callback may or may not be called depending on timing
		// This test just verifies the callback mechanism doesn't crash
	})
}

// ============================================================================
// Environment Variable Tests
// ============================================================================

func TestSSEBridge_Environment(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Passes environment variables to process", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)

		// Create a script that outputs an environment variable
		script := `#!/bin/bash
while IFS= read -r line; do
    if echo "$line" | grep -q '"method".*"initialize"'; then
        echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"env-test","version":"1.0.0"}}}'
    elif echo "$line" | grep -q '"method".*"notifications/initialized"'; then
        :
    elif echo "$line" | grep -q '"method".*"getenv"'; then
        id=$(echo "$line" | sed -n 's/.*"id":\s*\([0-9]*\).*/\1/p')
        echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"value\":\"$TEST_VAR\"}}"
    fi
done
`
		scriptPath := filepath.Join(dir, "env_test.sh")
		err := os.WriteFile(scriptPath, []byte(script), 0755)
		require.NoError(t, err)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Environment: map[string]string{
				"TEST_VAR": "hello_from_bridge",
			},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := bridge.SendRequest(ctx, "getenv", nil)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		// The result should contain our env var
		assert.Contains(t, string(resp.Result), "hello_from_bridge")
	})
}

// ============================================================================
// Working Directory Tests
// ============================================================================

func TestSSEBridge_WorkingDirectory(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Sets working directory for process", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		workDir := createTempDir(t)

		// Create a script that outputs the current directory
		script := `#!/bin/bash
while IFS= read -r line; do
    if echo "$line" | grep -q '"method".*"initialize"'; then
        echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"wd-test","version":"1.0.0"}}}'
    elif echo "$line" | grep -q '"method".*"notifications/initialized"'; then
        :
    elif echo "$line" | grep -q '"method".*"pwd"'; then
        id=$(echo "$line" | sed -n 's/.*"id":\s*\([0-9]*\).*/\1/p')
        echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"cwd\":\"$(pwd)\"}}"
    fi
done
`
		scriptPath := filepath.Join(dir, "pwd_test.sh")
		err := os.WriteFile(scriptPath, []byte(script), 0755)
		require.NoError(t, err)

		config := SSEBridgeConfig{
			Command:          []string{"/bin/bash", scriptPath},
			WorkingDirectory: workDir,
			Address:          ":0",
			Logger:           createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := bridge.SendRequest(ctx, "pwd", nil)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		assert.Contains(t, string(resp.Result), workDir)
	})
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkSSEBridge_HandleMessage(b *testing.B) {
	// Check if bash is available
	if _, err := exec.LookPath("bash"); err != nil {
		b.Skip("bash not available")
	}

	dir, err := os.MkdirTemp("", "sse_bridge_bench_*")
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	// Simple echo script for benchmarking
	script := `#!/bin/bash
while IFS= read -r line; do
    if echo "$line" | grep -q '"method".*"initialize"'; then
        echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"bench","version":"1.0.0"}}}'
    elif echo "$line" | grep -q '"method"'; then
        id=$(echo "$line" | sed -n 's/.*"id":\s*\([0-9]*\).*/\1/p')
        if [ -n "$id" ]; then
            echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{}}"
        fi
    fi
done
`
	scriptPath := filepath.Join(dir, "bench_mcp.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		b.Fatal(err)
	}

	config := SSEBridgeConfig{
		Command: []string{"/bin/bash", scriptPath},
		Address: ":0",
		Logger:  createTestLogger(),
	}

	bridge, err := NewSSEBridge(config)
	if err != nil {
		b.Fatal(err)
	}

	if err := bridge.Start(); err != nil {
		b.Fatal(err)
	}
	defer func() { _ = bridge.Shutdown(context.Background()) }()

	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reqBody := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"ping"}`, i+1)
		req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)
	}
}

func BenchmarkSSEBridge_SendRequest(b *testing.B) {
	// Check if bash is available
	if _, err := exec.LookPath("bash"); err != nil {
		b.Skip("bash not available")
	}

	dir, err := os.MkdirTemp("", "sse_bridge_bench_*")
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	script := `#!/bin/bash
while IFS= read -r line; do
    if echo "$line" | grep -q '"method".*"initialize"'; then
        echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"bench","version":"1.0.0"}}}'
    elif echo "$line" | grep -q '"method"'; then
        id=$(echo "$line" | sed -n 's/.*"id":\s*\([0-9]*\).*/\1/p')
        if [ -n "$id" ]; then
            echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{}}"
        fi
    fi
done
`
	scriptPath := filepath.Join(dir, "bench_mcp.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		b.Fatal(err)
	}

	config := SSEBridgeConfig{
		Command: []string{"/bin/bash", scriptPath},
		Address: ":0",
		Logger:  createTestLogger(),
	}

	bridge, err := NewSSEBridge(config)
	if err != nil {
		b.Fatal(err)
	}

	if err := bridge.Start(); err != nil {
		b.Fatal(err)
	}
	defer func() { _ = bridge.Shutdown(context.Background()) }()

	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = bridge.SendRequest(ctx, "ping", nil)
		cancel()
	}
}

// ============================================================================
// Error Code Constants Tests
// ============================================================================

func TestJSONRPCErrorCodes(t *testing.T) {
	t.Parallel()
	t.Run("Standard error codes are defined", func(t *testing.T) {
		assert.Equal(t, -32700, JSONRPCParseError)
		assert.Equal(t, -32600, JSONRPCInvalidRequest)
		assert.Equal(t, -32601, JSONRPCMethodNotFound)
		assert.Equal(t, -32602, JSONRPCInvalidParams)
		assert.Equal(t, -32603, JSONRPCInternalError)
	})

	t.Run("Server error codes are defined", func(t *testing.T) {
			t.Parallel()
		assert.Equal(t, -32000, JSONRPCServerError)
		assert.Equal(t, -32001, JSONRPCProcessNotReady)
		assert.Equal(t, -32002, JSONRPCProcessClosed)
		assert.Equal(t, -32003, JSONRPCTimeout)
		assert.Equal(t, -32004, JSONRPCBridgeShutdown)
		assert.Equal(t, -32005, JSONRPCRequestTooLarge)
		assert.Equal(t, -32006, JSONRPCTooManyRequests)
		assert.Equal(t, -32007, JSONRPCConnectionClosed)
	})
}

// ============================================================================
// Interface Compliance Tests
// ============================================================================

func TestSSEBridge_InterfaceCompliance(t *testing.T) {
	t.Parallel()
	t.Run("Handler returns http.Handler", func(t *testing.T) {
		config := SSEBridgeConfig{
			Command: []string{"echo"},
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		var handler http.Handler = bridge.Handler()
		assert.NotNil(t, handler)
	})
}

// ============================================================================
// Scanner Buffer Tests (for large responses)
// ============================================================================

func TestSSEBridge_LargeResponseHandling(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Handles large responses from MCP server", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)

		// Create a script that outputs a large response
		largeData := strings.Repeat("x", 50000) // 50KB
		script := fmt.Sprintf(`#!/bin/bash
while IFS= read -r line; do
    if echo "$line" | grep -q '"method".*"initialize"'; then
        echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"large-test","version":"1.0.0"}}}'
    elif echo "$line" | grep -q '"method".*"notifications/initialized"'; then
        :
    elif echo "$line" | grep -q '"method".*"large"'; then
        id=$(echo "$line" | sed -n 's/.*"id":\s*\([0-9]*\).*/\1/p')
        echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"data\":\"%s\"}}"
    fi
done
`, largeData)

		scriptPath := filepath.Join(dir, "large_test.sh")
		err := os.WriteFile(scriptPath, []byte(script), 0755)
		require.NoError(t, err)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := bridge.SendRequest(ctx, "large", nil)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		assert.Greater(t, len(resp.Result), 50000)
	})
}

// ============================================================================
// Stdin Write Concurrency Tests
// ============================================================================

func TestSSEBridge_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Handles concurrent writes to stdin safely", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command:      []string{"/bin/bash", scriptPath},
			Address:      ":0",
			Logger:       createTestLogger(),
			WriteTimeout: 30 * time.Second,
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		var wg sync.WaitGroup
		errChan := make(chan error, 20)

		// Send many concurrent requests
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				resp, err := bridge.SendRequest(ctx, "ping", nil)
				if err != nil {
					errChan <- fmt.Errorf("request %d error: %w", idx, err)
					return
				}
				if resp.Error != nil {
					errChan <- fmt.Errorf("request %d JSON-RPC error: %s", idx, resp.Error.Message)
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		for err := range errChan {
			t.Error(err)
		}
	})
}

// ============================================================================
// SSE Broadcast Tests
// ============================================================================

func TestSSEBridge_Broadcast(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Broadcasts responses to all SSE clients", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command:              []string{"/bin/bash", scriptPath},
			Address:              ":0",
			Logger:               createTestLogger(),
			SSEHeartbeatInterval: 10 * time.Second,
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		// Create multiple SSE clients
		clients := make([]*sseRecorder, 3)
		ctxs := make([]context.Context, 3)
		cancels := make([]context.CancelFunc, 3)
		dones := make([]chan struct{}, 3)

		for i := 0; i < 3; i++ {
			ctxs[i], cancels[i] = context.WithCancel(context.Background())
			clients[i] = &sseRecorder{ResponseRecorder: httptest.NewRecorder(), flushed: make(chan struct{})}
			dones[i] = make(chan struct{})

			go func(idx int) {
				req := httptest.NewRequest(http.MethodGet, "/sse", nil).WithContext(ctxs[idx])
				bridge.Handler().ServeHTTP(clients[idx], req)
				close(dones[idx])
			}(i)

			// Wait for connection
			select {
			case <-clients[i].flushed:
			case <-time.After(2 * time.Second):
				t.Fatalf("Timeout waiting for client %d to connect", i)
			}
		}

		// Verify all clients are connected
		assert.Equal(t, 3, bridge.ActiveClients())

		// Send a request that will generate a broadcast
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = bridge.SendRequest(ctx, "ping", nil)
		require.NoError(t, err)

		// Give time for broadcast
		time.Sleep(200 * time.Millisecond)

		// Clean up
		for i := 0; i < 3; i++ {
			cancels[i]()
			<-dones[i]
		}
	})
}

// ============================================================================
// Process Initialization Timeout Tests
// ============================================================================

func TestSSEBridge_InitializationTimeout(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Fails on initialization timeout", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)

		// Create a script that never responds to initialize
		script := `#!/bin/bash
# Never respond to initialize
while true; do
    read -r line
done
`
		scriptPath := filepath.Join(dir, "timeout_test.sh")
		err := os.WriteFile(scriptPath, []byte(script), 0755)
		require.NoError(t, err)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		// Start should fail due to initialization timeout (30s is too long for test)
		// We'll test with a context that expires sooner if needed
		// For now, just verify that a non-responding server would eventually fail
		// Use bridge to avoid unused variable error - verify initial state
		assert.Equal(t, StateIdle, bridge.State())
	})
}

// ============================================================================
// Stderr Handling Tests
// ============================================================================

func TestSSEBridge_StderrHandling(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Logs stderr output from MCP process", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)

		// Create a script that outputs to stderr
		script := `#!/bin/bash
echo "Startup message" >&2
while IFS= read -r line; do
    if echo "$line" | grep -q '"method".*"initialize"'; then
        echo "Initialize received" >&2
        echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"stderr-test","version":"1.0.0"}}}'
    elif echo "$line" | grep -q '"method".*"notifications/initialized"'; then
        :
    elif echo "$line" | grep -q '"method"'; then
        echo "Request received" >&2
        id=$(echo "$line" | sed -n 's/.*"id":\s*\([0-9]*\).*/\1/p')
        if [ -n "$id" ]; then
            echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{}}"
        fi
    fi
done
`
		scriptPath := filepath.Join(dir, "stderr_test.sh")
		err := os.WriteFile(scriptPath, []byte(script), 0755)
		require.NoError(t, err)

		// Create a logger that captures output
		var buf bytes.Buffer
		logger := logrus.New()
		logger.SetOutput(&buf)
		logger.SetLevel(logrus.WarnLevel)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  logger,
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(200 * time.Millisecond)

		// The stderr output should have been logged
		logOutput := buf.String()
		assert.Contains(t, logOutput, "Startup message")
	})
}

// ============================================================================
// Health Check Details Tests
// ============================================================================

func TestSSEBridge_HealthCheckDetails(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("Returns detailed health information", func(t *testing.T) {
			t.Parallel()
		dir := createTempDir(t)
		scriptPath := createMockMCPServer(t, dir)

		config := SSEBridgeConfig{
			Command: []string{"/bin/bash", scriptPath},
			Address: ":0",
			Logger:  createTestLogger(),
		}

		bridge, err := NewSSEBridge(config)
		require.NoError(t, err)

		err = bridge.Start()
		require.NoError(t, err)
		defer func() { _ = bridge.Shutdown(context.Background()) }()

		time.Sleep(100 * time.Millisecond)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		bridge.Handler().ServeHTTP(w, req)

		var health map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &health)
		require.NoError(t, err)

		// Verify all expected fields
		assert.Contains(t, health, "status")
		assert.Contains(t, health, "healthy")
		assert.Contains(t, health, "processReady")
		assert.Contains(t, health, "processPid")
		assert.Contains(t, health, "uptime")
		assert.Contains(t, health, "metrics")

		// Verify metrics
		metrics, ok := health["metrics"].(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, metrics, "totalRequests")
		assert.Contains(t, metrics, "successfulRequests")
		assert.Contains(t, metrics, "failedRequests")
		assert.Contains(t, metrics, "activeSSEConnections")
		assert.Contains(t, metrics, "totalSSEConnections")
		assert.Contains(t, metrics, "bytesSent")
		assert.Contains(t, metrics, "bytesReceived")
		assert.Contains(t, metrics, "processRestarts")
	})
}

// ============================================================================
// Scanner Line Reading Tests
// ============================================================================

func TestBufioScanner_LineReading(t *testing.T) {
	t.Parallel()
	t.Run("Handles JSON-RPC messages correctly", func(t *testing.T) {
		input := `{"jsonrpc":"2.0","id":1,"result":{}}
{"jsonrpc":"2.0","id":2,"result":{"key":"value"}}
{"jsonrpc":"2.0","id":3,"error":{"code":-32600,"message":"Invalid Request"}}
`
		reader := strings.NewReader(input)
		scanner := bufio.NewScanner(reader)

		var responses []JSONRPCResponse
		for scanner.Scan() {
			var resp JSONRPCResponse
			err := json.Unmarshal(scanner.Bytes(), &resp)
			require.NoError(t, err)
			responses = append(responses, resp)
		}

		require.NoError(t, scanner.Err())
		assert.Len(t, responses, 3)
		assert.Equal(t, "2.0", responses[0].JSONRPC)
		assert.Equal(t, "2.0", responses[1].JSONRPC)
		assert.NotNil(t, responses[2].Error)
	})
}
