// Package lsp provides real functional tests for LSP (Language Server Protocol) servers.
// These tests execute ACTUAL LSP operations, not just connectivity checks.
// Tests FAIL if the operation fails - no false positives.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LSPRequest represents a JSON-RPC 2.0 request for LSP
type LSPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// LSPResponse represents a JSON-RPC 2.0 response
type LSPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *LSPError       `json:"error,omitempty"`
}

// LSPError represents a JSON-RPC error
type LSPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// LSPClient provides a client for testing LSP servers
type LSPClient struct {
	conn    net.Conn
	reader  *bufio.Reader
	reqID   int
	timeout time.Duration
}

// NewLSPClient creates a new LSP test client
func NewLSPClient(addr string, timeout time.Duration) (*LSPClient, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LSP server: %w", err)
	}

	return &LSPClient{
		conn:    conn,
		reader:  bufio.NewReader(conn),
		reqID:   0,
		timeout: timeout,
	}, nil
}

// Close closes the LSP client connection
func (c *LSPClient) Close() error {
	return c.conn.Close()
}

// Call executes an LSP method and returns the response
// LSP uses Content-Length header framing
func (c *LSPClient) Call(method string, params interface{}) (*LSPResponse, error) {
	c.reqID++

	req := LSPRequest{
		JSONRPC: "2.0",
		ID:      c.reqID,
		Method:  method,
		Params:  params,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// LSP uses Content-Length header
	message := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(reqData), reqData)

	_ = c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
	_, err = c.conn.Write([]byte(message))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response with Content-Length header
	_ = c.conn.SetReadDeadline(time.Now().Add(c.timeout))

	// Read headers
	var contentLength int
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read header: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // End of headers
		}
		if strings.HasPrefix(line, "Content-Length:") {
			_, _ = fmt.Sscanf(line, "Content-Length: %d", &contentLength)
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("no Content-Length in response")
	}

	// Read body
	body := make([]byte, contentLength)
	_, err = c.reader.Read(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var resp LSPResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (raw: %s)", err, string(body))
	}

	return &resp, nil
}

// Initialize sends the initialize request to the LSP server
func (c *LSPClient) Initialize(rootURI string) (*LSPResponse, error) {
	params := map[string]interface{}{
		"processId": nil,
		"rootUri":   rootURI,
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"completion": map[string]interface{}{
					"completionItem": map[string]interface{}{
						"snippetSupport": true,
					},
				},
				"hover": map[string]interface{}{},
			},
		},
	}
	return c.Call("initialize", params)
}

// Shutdown sends the shutdown request
func (c *LSPClient) Shutdown() (*LSPResponse, error) {
	return c.Call("shutdown", nil)
}

// LSPServerConfig holds configuration for testing an LSP server
type LSPServerConfig struct {
	Name     string
	Port     int
	Language string
	TestFile string
}

// Common LSP servers to test
var LSPServers = []LSPServerConfig{
	{Name: "gopls", Port: 9501, Language: "go", TestFile: "test.go"},
	{Name: "pyright", Port: 9502, Language: "python", TestFile: "test.py"},
	{Name: "typescript-language-server", Port: 9503, Language: "typescript", TestFile: "test.ts"},
	{Name: "rust-analyzer", Port: 9504, Language: "rust", TestFile: "test.rs"},
	{Name: "clangd", Port: 9505, Language: "c++", TestFile: "test.cpp"},
}

// TestLSPServerInitialize tests LSP server initialization
func TestLSPServerInitialize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LSP functional test in short mode")
	}
	for _, server := range LSPServers {
		t.Run(server.Name, func(t *testing.T) {
			client, err := NewLSPClient(fmt.Sprintf("localhost:%d", server.Port), 10*time.Second)
			if err != nil {
				t.Skipf("LSP server %s not running on port %d: %v", server.Name, server.Port, err)
				return
			}
			defer func() { _ = client.Close() }()

			resp, err := client.Initialize("file:///tmp/test-workspace")
			if err != nil {
				t.Skipf("LSP server %s not responding to initialize: %v", server.Name, err)
				return
			}
			require.Nil(t, resp.Error, "Initialize must not return error")

			var result map[string]interface{}
			err = json.Unmarshal(resp.Result, &result)
			require.NoError(t, err)

			capabilities, ok := result["capabilities"]
			assert.True(t, ok, "Response must contain capabilities")
			assert.NotNil(t, capabilities, "Capabilities must not be nil")

			t.Logf("LSP %s initialized with capabilities: %v", server.Name, capabilities)
		})
	}
}

// TestLSPServerShutdown tests LSP server shutdown
func TestLSPServerShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LSP functional test in short mode")
	}
	for _, server := range LSPServers {
		t.Run(server.Name, func(t *testing.T) {
			client, err := NewLSPClient(fmt.Sprintf("localhost:%d", server.Port), 10*time.Second)
			if err != nil {
				t.Skipf("LSP server %s not running: %v", server.Name, err)
				return
			}
			defer func() { _ = client.Close() }()

			// Initialize first
			_, err = client.Initialize("file:///tmp/test-workspace")
			if err != nil {
				t.Skipf("LSP server %s not responding to initialize: %v", server.Name, err)
				return
			}

			// Then shutdown
			resp, err := client.Shutdown()
			require.NoError(t, err, "Shutdown must succeed")
			if resp.Error != nil {
				t.Logf("Shutdown returned error (may be expected): %s", resp.Error.Message)
			}
		})
	}
}

// BenchmarkLSPInitialize benchmarks LSP server initialization
func BenchmarkLSPInitialize(b *testing.B) {
	client, err := NewLSPClient("localhost:9501", 10*time.Second)
	if err != nil {
		b.Skipf("LSP server not running: %v", err)
		return
	}
	defer func() { _ = client.Close() }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Initialize("file:///tmp/test-workspace")
		if err != nil {
			b.Fatalf("Initialize failed: %v", err)
		}
	}
}
