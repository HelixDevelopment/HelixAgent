package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// waitForConnection Tests
// ============================================================================

func TestConnectionPool_waitForConnection(t *testing.T) {
	t.Run("Returns nil when connection becomes connected", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		err := pool.RegisterServer(MCPServerConfig{
			Name: "test-server",
			Type: MCPServerTypeRemote,
		})
		require.NoError(t, err)

		// Start waiting in a goroutine
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var waitErr error
		done := make(chan struct{})

		go func() {
			waitErr = pool.waitForConnection(ctx, "test-server")
			close(done)
		}()

		// Simulate connection becoming connected after a delay
		time.Sleep(100 * time.Millisecond)
		pool.mu.Lock()
		conn := pool.connections["test-server"]
		conn.mu.Lock()
		conn.Status = StatusConnectionConnected
		conn.mu.Unlock()
		pool.mu.Unlock()

		<-done
		assert.NoError(t, waitErr)
	})

	t.Run("Returns error when connection fails", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		err := pool.RegisterServer(MCPServerConfig{
			Name: "test-server",
			Type: MCPServerTypeRemote,
		})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var waitErr error
		done := make(chan struct{})

		go func() {
			waitErr = pool.waitForConnection(ctx, "test-server")
			close(done)
		}()

		// Simulate connection failure
		time.Sleep(100 * time.Millisecond)
		pool.mu.Lock()
		conn := pool.connections["test-server"]
		conn.mu.Lock()
		conn.Status = StatusConnectionFailed
		conn.LastError = fmt.Errorf("connection refused")
		conn.mu.Unlock()
		pool.mu.Unlock()

		<-done
		assert.Error(t, waitErr)
		assert.Contains(t, waitErr.Error(), "connection failed")
	})

	t.Run("Returns error when connection is closed", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		err := pool.RegisterServer(MCPServerConfig{
			Name: "test-server",
			Type: MCPServerTypeRemote,
		})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var waitErr error
		done := make(chan struct{})

		go func() {
			waitErr = pool.waitForConnection(ctx, "test-server")
			close(done)
		}()

		// Simulate connection being closed
		time.Sleep(100 * time.Millisecond)
		pool.mu.Lock()
		conn := pool.connections["test-server"]
		conn.mu.Lock()
		conn.Status = StatusConnectionClosed
		conn.mu.Unlock()
		pool.mu.Unlock()

		<-done
		assert.Error(t, waitErr)
		assert.Contains(t, waitErr.Error(), "connection closed")
	})

	t.Run("Returns error when server not found", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		err := pool.waitForConnection(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Returns error on context cancellation", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		err := pool.RegisterServer(MCPServerConfig{
			Name: "test-server",
			Type: MCPServerTypeRemote,
		})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err = pool.waitForConnection(ctx, "test-server")
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

// ============================================================================
// initializeMCPConnection Tests
// ============================================================================

func TestConnectionPool_initializeMCPConnection(t *testing.T) {
	t.Run("Successfully initializes MCP connection", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		// Create a mock transport that responds correctly to initialization
		mockTransport := &MockMCPTransportWithInit{
			connected: true,
			initResponse: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"serverInfo": map[string]interface{}{
						"name":    "test-server",
						"version": "1.0.0",
					},
				},
			},
		}

		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name: "test-server",
				Type: MCPServerTypeRemote,
			},
			Transport: mockTransport,
		}

		ctx := context.Background()
		err := pool.initializeMCPConnection(ctx, conn)
		assert.NoError(t, err)

		// Verify the initialization messages were sent
		assert.Len(t, mockTransport.sentMessages, 2) // initialize request + initialized notification
	})

	t.Run("Returns error on send failure", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		mockTransport := &MockMCPTransportWithInit{
			connected: true,
			sendError: fmt.Errorf("send failed"),
		}

		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name: "test-server",
				Type: MCPServerTypeRemote,
			},
			Transport: mockTransport,
		}

		ctx := context.Background()
		err := pool.initializeMCPConnection(ctx, conn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "send initialize request")
	})

	t.Run("Returns error on receive failure", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		mockTransport := &MockMCPTransportWithInit{
			connected:    true,
			receiveError: fmt.Errorf("receive failed"),
		}

		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name: "test-server",
				Type: MCPServerTypeRemote,
			},
			Transport: mockTransport,
		}

		ctx := context.Background()
		err := pool.initializeMCPConnection(ctx, conn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "receive initialize response")
	})

	t.Run("Returns error on invalid response format", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		mockTransport := &MockMCPTransportWithInit{
			connected:    true,
			initResponse: "invalid response", // Not a map
		}

		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name: "test-server",
				Type: MCPServerTypeRemote,
			},
			Transport: mockTransport,
		}

		ctx := context.Background()
		err := pool.initializeMCPConnection(ctx, conn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid initialize response format")
	})

	t.Run("Returns error when response contains error object", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		mockTransport := &MockMCPTransportWithInit{
			connected: true,
			initResponse: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"error": map[string]interface{}{
					"code":    -32600,
					"message": "Invalid Request",
				},
			},
		}

		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name: "test-server",
				Type: MCPServerTypeRemote,
			},
			Transport: mockTransport,
		}

		ctx := context.Background()
		err := pool.initializeMCPConnection(ctx, conn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "initialize error")
	})

	t.Run("Returns error when initialized notification fails", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		mockTransport := &MockMCPTransportWithInit{
			connected: true,
			initResponse: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
				},
			},
			failOnSecondSend: true, // Fail when sending initialized notification
		}

		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name: "test-server",
				Type: MCPServerTypeRemote,
			},
			Transport: mockTransport,
		}

		ctx := context.Background()
		err := pool.initializeMCPConnection(ctx, conn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "initialized notification")
	})
}

// MockMCPTransportWithInit is a mock transport for testing initialization
type MockMCPTransportWithInit struct {
	connected        bool
	sendError        error
	receiveError     error
	initResponse     interface{}
	sentMessages     []interface{}
	failOnSecondSend bool
	sendCount        int
	mu               sync.Mutex
}

func (m *MockMCPTransportWithInit) Send(ctx context.Context, message interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sendCount++
	m.sentMessages = append(m.sentMessages, message)

	if m.failOnSecondSend && m.sendCount > 1 {
		return fmt.Errorf("send failed on second call")
	}

	return m.sendError
}

func (m *MockMCPTransportWithInit) Receive(ctx context.Context) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.receiveError != nil {
		return nil, m.receiveError
	}
	return m.initResponse, nil
}

func (m *MockMCPTransportWithInit) Close() error {
	m.connected = false
	return nil
}

func (m *MockMCPTransportWithInit) IsConnected() bool {
	return m.connected
}

// ============================================================================
// connectRemoteServer Tests
// ============================================================================

func TestConnectionPool_connectRemoteServer(t *testing.T) {
	t.Run("Returns error when URL is empty", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name: "test-server",
				Type: MCPServerTypeRemote,
				URL:  "", // Empty URL
			},
		}

		ctx := context.Background()
		err := pool.connectRemoteServer(ctx, conn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no URL specified")
	})

	t.Run("Successfully connects to remote server", func(t *testing.T) {
		// Create a mock server that responds to MCP initialization
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"serverInfo": map[string]interface{}{
						"name":    "test-server",
						"version": "1.0.0",
					},
				},
			})
		}))
		defer server.Close()

		pool := NewConnectionPool(nil, nil, createTestLogger())

		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name:    "test-server",
				Type:    MCPServerTypeRemote,
				URL:     server.URL,
				Timeout: 10 * time.Second,
			},
		}

		ctx := context.Background()
		err := pool.connectRemoteServer(ctx, conn)
		assert.NoError(t, err)
		assert.NotNil(t, conn.Transport)
	})

	t.Run("Returns error on initialization failure", func(t *testing.T) {
		// Create a mock server that returns an error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"error": map[string]interface{}{
					"code":    -32600,
					"message": "Invalid Request",
				},
			})
		}))
		defer server.Close()

		pool := NewConnectionPool(nil, nil, createTestLogger())

		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name:    "test-server",
				Type:    MCPServerTypeRemote,
				URL:     server.URL,
				Timeout: 10 * time.Second,
			},
		}

		ctx := context.Background()
		err := pool.connectRemoteServer(ctx, conn)
		assert.Error(t, err)
		assert.Nil(t, conn.Transport)
	})
}

// ============================================================================
// connectLocalServer Tests
// ============================================================================

func TestConnectionPool_connectLocalServer(t *testing.T) {
	t.Run("Returns error when no command specified", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name:    "test-server",
				Type:    MCPServerTypeLocal,
				Command: []string{}, // Empty command
			},
		}

		ctx := context.Background()
		err := pool.connectLocalServer(ctx, conn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no command specified")
	})

	t.Run("Returns error when command executable not found", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name:    "test-server",
				Type:    MCPServerTypeLocal,
				Command: []string{"/nonexistent/command/abcdef123"},
			},
		}

		ctx := context.Background()
		err := pool.connectLocalServer(ctx, conn)
		assert.Error(t, err)
	})
}

// ============================================================================
// connectServer Tests
// ============================================================================

func TestConnectionPool_connectServer(t *testing.T) {
	t.Run("Retries on failure", func(t *testing.T) {
		config := &MCPPoolConfig{
			RetryAttempts: 3,
			RetryDelay:    10 * time.Millisecond,
		}
		pool := NewConnectionPool(nil, config, createTestLogger())

		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name: "test-server",
				Type: MCPServerTypeRemote,
				URL:  "http://localhost:99999", // Invalid port
			},
		}

		ctx := context.Background()
		start := time.Now()
		err := pool.connectServer(ctx, conn)
		elapsed := time.Since(start)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect after")
		assert.Equal(t, StatusConnectionFailed, conn.Status)
		// Should have taken at least (retries-1) * delay
		assert.GreaterOrEqual(t, elapsed, 2*10*time.Millisecond)
	})

	t.Run("Respects context cancellation during retries", func(t *testing.T) {
		config := &MCPPoolConfig{
			RetryAttempts: 5,
			RetryDelay:    1 * time.Second, // Long delay
		}
		pool := NewConnectionPool(nil, config, createTestLogger())

		conn := &MCPConnection{
			Config: MCPServerConfig{
				Name: "test-server",
				Type: MCPServerTypeRemote,
				URL:  "http://localhost:99999",
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := pool.connectServer(ctx, conn)
		assert.Error(t, err)
		assert.Equal(t, StatusConnectionFailed, conn.Status)
	})
}

// ============================================================================
// GetConnection Tests (additional)
// ============================================================================

func TestConnectionPool_GetConnection_Connecting(t *testing.T) {
	t.Run("Waits for connection when status is connecting", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		err := pool.RegisterServer(MCPServerConfig{
			Name: "test-server",
			Type: MCPServerTypeRemote,
			URL:  "http://localhost:8080",
		})
		require.NoError(t, err)

		// Set status to connecting
		pool.mu.Lock()
		conn := pool.connections["test-server"]
		conn.mu.Lock()
		conn.Status = StatusConnectionConnecting
		conn.mu.Unlock()
		pool.mu.Unlock()

		// Start GetConnection in a goroutine
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var getErr error
		var gotConn *MCPConnection
		done := make(chan struct{})

		go func() {
			gotConn, getErr = pool.GetConnection(ctx, "test-server")
			close(done)
		}()

		// Simulate connection completing
		time.Sleep(100 * time.Millisecond)
		pool.mu.Lock()
		conn = pool.connections["test-server"]
		conn.mu.Lock()
		conn.Status = StatusConnectionConnected
		conn.Transport = NewMockMCPTransport()
		conn.mu.Unlock()
		pool.mu.Unlock()

		<-done
		assert.NoError(t, getErr)
		assert.NotNil(t, gotConn)
	})
}

// ============================================================================
// HTTP Transport Additional Tests
// ============================================================================

func TestHTTPMCPTransport_SendReceive_Integration(t *testing.T) {
	t.Run("Full request-response cycle", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read request body
			body, _ := io.ReadAll(r.Body)
			var request map[string]interface{}
			_ = json.Unmarshal(body, &request)

			// Echo back a response
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      request["id"],
				"result": map[string]interface{}{
					"echo": request["params"],
				},
			})
		}))
		defer server.Close()

		transport := &HTTPMCPTransport{
			baseURL:   server.URL,
			headers:   map[string]string{"X-Test": "value"},
			connected: true,
			client:    &http.Client{Timeout: 10 * time.Second},
		}

		ctx := context.Background()

		// Send a request
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      42,
			"method":  "test",
			"params":  map[string]string{"key": "value"},
		}
		err := transport.Send(ctx, request)
		require.NoError(t, err)

		// Receive response
		response, err := transport.Receive(ctx)
		require.NoError(t, err)

		respMap, ok := response.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "2.0", respMap["jsonrpc"])
		assert.Equal(t, float64(42), respMap["id"])
	})

	t.Run("Handles multiple headers", func(t *testing.T) {
		var receivedHeaders http.Header
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeaders = r.Header
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})
		}))
		defer server.Close()

		transport := &HTTPMCPTransport{
			baseURL: server.URL,
			headers: map[string]string{
				"X-Header-1":    "value1",
				"X-Header-2":    "value2",
				"Authorization": "Bearer token123",
			},
			connected: true,
			client:    &http.Client{},
		}

		ctx := context.Background()
		err := transport.Send(ctx, map[string]string{})
		require.NoError(t, err)

		assert.Equal(t, "value1", receivedHeaders.Get("X-Header-1"))
		assert.Equal(t, "value2", receivedHeaders.Get("X-Header-2"))
		assert.Equal(t, "Bearer token123", receivedHeaders.Get("Authorization"))
	})
}

// ============================================================================
// Stdio Transport Additional Tests
// ============================================================================

func TestStdioMCPTransport_Send_ConnectionLost(t *testing.T) {
	t.Run("Sets connected to false on write error", func(t *testing.T) {
		// Use a pipe and close the reader to simulate write error
		reader, writer := io.Pipe()
		_ = reader.Close() // Close the reader to cause write error

		transport := &StdioMCPTransport{
			stdin:     writer,
			connected: true,
		}

		ctx := context.Background()
		err := transport.Send(ctx, map[string]string{"test": "data"})
		assert.Error(t, err)
		assert.False(t, transport.IsConnected())
	})
}

// ============================================================================
// Pool with Preinstaller Tests
// ============================================================================

func TestConnectionPool_WithPreinstaller_WaitsForInstallation(t *testing.T) {
	t.Run("Waits for package installation", func(t *testing.T) {
		tempDir := createTempDir(t)

		preConfig := PreinstallerConfig{
			InstallDir: tempDir,
			Packages: []MCPPackage{
				{Name: "test-server", NPM: "test-pkg", Description: "Test"},
			},
		}
		preinstaller, err := NewPreinstaller(preConfig)
		require.NoError(t, err)

		pool := NewConnectionPool(preinstaller, nil, createTestLogger())

		err = pool.RegisterServer(MCPServerConfig{
			Name: "test-server",
			Type: MCPServerTypeLocal,
			// No command - will try to get from preinstaller
		})
		require.NoError(t, err)

		// Simulate package being installed in background
		go func() {
			time.Sleep(100 * time.Millisecond)
			mutatePackageStatusForTest(preinstaller, "test-server", func(s *PackageStatus) {
				s.Status = StatusInstalled
				s.InstallPath = tempDir
			})
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		// This will fail but tests the preinstaller waiting logic
		_, err = pool.GetConnection(ctx, "test-server")
		// The error is expected because we don't have a real package
		assert.Error(t, err)
	})
}

// ============================================================================
// Pool Warmup Tests
// ============================================================================

func TestConnectionPool_WarmUp_AllServers(t *testing.T) {
	t.Run("Warms up all registered servers when no list provided", func(t *testing.T) {
		pool := NewConnectionPool(nil, nil, createTestLogger())

		// Register multiple servers
		for i := 0; i < 3; i++ {
			err := pool.RegisterServer(MCPServerConfig{
				Name: fmt.Sprintf("server-%d", i),
				Type: MCPServerTypeRemote,
				URL:  fmt.Sprintf("http://localhost:%d", 9000+i),
			})
			require.NoError(t, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Will fail because servers don't exist, but tests the logic
		err := pool.WarmUp(ctx, nil)
		// Should have errors because servers don't exist
		assert.Error(t, err)
	})
}

// Note: createTestLogger function is defined in mcp_test.go
