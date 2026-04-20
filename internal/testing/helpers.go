// Package testing provides test utilities and helpers for HelixAgent tests.
package testing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// ============================================================================
// Mock MCP Server
// ============================================================================

// MockMCPServer provides a mock MCP server for testing purposes.
//
// Concurrency model (CONST-029): Handlers and RequestLog are
// *safe.Store / *safe.Slice containers; requestCount is atomic.Int64.
// No mu survives because the three fields have no cross-key
// invariants — each handler registration, request log append, and
// counter increment is an independent atomic op.
type MockMCPServer struct {
	// Server is the underlying HTTP test server
	Server *httptest.Server

	// Handlers maps method names to handler functions
	Handlers *safe.Store[string, MockMethodHandler]

	// RequestLog stores all received requests
	RequestLog *safe.Slice[MockMCPRequest]

	// InitializeResponse is the response to return for initialize
	InitializeResponse *MCPInitializeResponse

	// DefaultToolsResponse is the default tools/list response
	DefaultToolsResponse *MCPToolsListResponse

	// Latency adds artificial delay to responses
	Latency time.Duration

	// FailAfter makes the server return errors after N requests
	FailAfter int

	// requestCount tracks total requests
	requestCount atomic.Int64
}

// MockMethodHandler is a handler for a specific MCP method
type MockMethodHandler func(req *MockMCPRequest) (*MCPResponse, error)

// MockMCPRequest represents a request to the mock MCP server
type MockMCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Time    time.Time       `json:"-"`
}

// MCPResponse represents an MCP JSON-RPC response
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents an MCP JSON-RPC error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPInitializeResponse is the structure for initialize response
type MCPInitializeResponse struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    MCPCapabilities `json:"capabilities"`
	ServerInfo      MCPServerInfo   `json:"serverInfo"`
	Instructions    string          `json:"instructions,omitempty"`
}

// MCPCapabilities describes server capabilities
type MCPCapabilities struct {
	Experimental map[string]interface{}  `json:"experimental,omitempty"`
	Logging      map[string]interface{}  `json:"logging,omitempty"`
	Prompts      *MCPPromptsCapability   `json:"prompts,omitempty"`
	Resources    *MCPResourcesCapability `json:"resources,omitempty"`
	Tools        *MCPToolsCapability     `json:"tools,omitempty"`
}

// MCPPromptsCapability describes prompts capability
type MCPPromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPResourcesCapability describes resources capability
type MCPResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPToolsCapability describes tools capability
type MCPToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPServerInfo describes server information
type MCPServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPToolsListResponse is the response for tools/list
type MCPToolsListResponse struct {
	Tools []MCPTool `json:"tools"`
}

// MCPTool represents an MCP tool definition
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

// NewMockMCPServer creates a new mock MCP server with default configuration.
func NewMockMCPServer() *MockMCPServer {
	mock := &MockMCPServer{
		Handlers:   safe.NewStore[string, MockMethodHandler](),
		RequestLog: safe.NewSlice[MockMCPRequest](),
		InitializeResponse: &MCPInitializeResponse{
			ProtocolVersion: "2024-11-05",
			Capabilities: MCPCapabilities{
				Tools: &MCPToolsCapability{
					ListChanged: false,
				},
			},
			ServerInfo: MCPServerInfo{
				Name:    "mock-mcp-server",
				Version: "1.0.0",
			},
		},
		DefaultToolsResponse: &MCPToolsListResponse{
			Tools: []MCPTool{
				{
					Name:        "echo",
					Description: "Echoes the input back",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"message": map[string]interface{}{
								"type":        "string",
								"description": "The message to echo",
							},
						},
						"required": []string{"message"},
					},
				},
				{
					Name:        "ping",
					Description: "Returns pong",
				},
			},
		},
	}

	// Set up default handlers
	mock.setupDefaultHandlers()

	// Create HTTP server
	mock.Server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))

	return mock
}

// setupDefaultHandlers sets up default method handlers
func (m *MockMCPServer) setupDefaultHandlers() {
	// Initialize handler
	m.Handlers.Put("initialize", func(req *MockMCPRequest) (*MCPResponse, error) {
		result, err := json.Marshal(m.InitializeResponse)
		if err != nil {
			return nil, err
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}, nil
	})

	// Tools list handler
	m.Handlers.Put("tools/list", func(req *MockMCPRequest) (*MCPResponse, error) {
		result, err := json.Marshal(m.DefaultToolsResponse)
		if err != nil {
			return nil, err
		}
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}, nil
	})

	// Tools call handler
	m.Handlers.Put("tools/call", func(req *MockMCPRequest) (*MCPResponse, error) {
		// Parse the tool call
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &MCPError{
					Code:    -32602,
					Message: "Invalid params",
					Data:    err.Error(),
				},
			}, nil
		}

		// Handle known tools
		switch params.Name {
		case "echo":
			msg, _ := params.Arguments["message"].(string)    //nolint:errcheck
			result, _ := json.Marshal(map[string]interface{}{ //nolint:errcheck
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": msg,
					},
				},
			})
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			}, nil
		case "ping":
			result, _ := json.Marshal(map[string]interface{}{ //nolint:errcheck
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "pong",
					},
				},
			})
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  result,
			}, nil
		default:
			return &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &MCPError{
					Code:    -32601,
					Message: fmt.Sprintf("Unknown tool: %s", params.Name),
				},
			}, nil
		}
	})

	// Ping handler
	m.Handlers.Put("ping", func(req *MockMCPRequest) (*MCPResponse, error) {
		result, _ := json.Marshal("pong") //nolint:errcheck
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}, nil
	})
}

// handleRequest processes incoming requests
func (m *MockMCPServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Add latency if configured
	if m.Latency > 0 {
		time.Sleep(m.Latency)
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	// Parse request
	var req MockMCPRequest
	if err = json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	req.Time = time.Now()

	// Log request
	m.RequestLog.Append(req)
	currentCount := m.requestCount.Add(1)

	// Check if we should fail
	if m.FailAfter > 0 && currentCount > int64(m.FailAfter) {
		resp := &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32000,
				Message: "Simulated server error",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp) //nolint:errcheck
		return
	}

	// Handle notifications (no response needed)
	if req.ID == nil && strings.HasPrefix(req.Method, "notifications/") {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Find handler
	handler, exists := m.Handlers.Get(req.Method)

	var resp *MCPResponse
	if exists {
		resp, err = handler(&req)
		if err != nil {
			resp = &MCPResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &MCPError{
					Code:    -32603,
					Message: "Internal error",
					Data:    err.Error(),
				},
			}
		}
	} else {
		resp = &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// URL returns the server URL
func (m *MockMCPServer) URL() string {
	return m.Server.URL
}

// Close shuts down the mock server
func (m *MockMCPServer) Close() {
	m.Server.Close()
}

// AddHandler adds a custom handler for a method
func (m *MockMCPServer) AddHandler(method string, handler MockMethodHandler) {
	m.Handlers.Put(method, handler)
}

// GetRequests returns all logged requests
func (m *MockMCPServer) GetRequests() []MockMCPRequest {
	return m.RequestLog.Snapshot()
}

// GetRequestCount returns the total number of requests
func (m *MockMCPServer) GetRequestCount() int {
	return int(m.requestCount.Load())
}

// Reset clears the request log and count
func (m *MockMCPServer) Reset() {
	m.RequestLog.Clear()
	m.requestCount.Store(0)
}

// ============================================================================
// Mock HTTP Response Helpers
// ============================================================================

// MockHTTPResponse represents a mock HTTP response configuration
type MockHTTPResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       string
	Delay      time.Duration
	Error      error
}

// MockHTTPServer creates a mock HTTP server with predefined responses.
type MockHTTPServer struct {
	Server    *httptest.Server
	Responses map[string]*MockHTTPResponse
	mu        sync.RWMutex
}

// NewMockHTTPServer creates a new mock HTTP server
func NewMockHTTPServer() *MockHTTPServer {
	mock := &MockHTTPServer{
		Responses: make(map[string]*MockHTTPResponse),
	}

	mock.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mock.mu.RLock()
		resp, exists := mock.Responses[r.URL.Path]
		mock.mu.RUnlock()

		if !exists {
			http.NotFound(w, r)
			return
		}

		if resp.Delay > 0 {
			time.Sleep(resp.Delay)
		}

		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}

		if resp.StatusCode == 0 {
			resp.StatusCode = http.StatusOK
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write([]byte(resp.Body)) //nolint:errcheck
	}))

	return mock
}

// SetResponse sets a response for a given path
func (m *MockHTTPServer) SetResponse(path string, resp *MockHTTPResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses[path] = resp
}

// URL returns the server URL
func (m *MockHTTPServer) URL() string {
	return m.Server.URL
}

// Close shuts down the server
func (m *MockHTTPServer) Close() {
	m.Server.Close()
}

// ============================================================================
// Test Fixtures
// ============================================================================

// TestFixtures provides common test data
type TestFixtures struct {
	// TempDir is the temporary directory for test files
	TempDir string

	// t is the testing.T for cleanup
	t *testing.T
}

// NewTestFixtures creates a new test fixtures instance
func NewTestFixtures(t *testing.T) *TestFixtures {
	tempDir, err := os.MkdirTemp("", "helixagent_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	return &TestFixtures{
		TempDir: tempDir,
		t:       t,
	}
}

// CreateTempFile creates a temporary file with the given content
func (f *TestFixtures) CreateTempFile(name, content string) string {
	path := filepath.Join(f.TempDir, name)

	// Create parent directories if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		f.t.Fatalf("Failed to create directories: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		f.t.Fatalf("Failed to create temp file: %v", err)
	}

	return path
}

// CreateExecutableFile creates an executable script file
func (f *TestFixtures) CreateExecutableFile(name, content string) string {
	path := filepath.Join(f.TempDir, name)

	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		f.t.Fatalf("Failed to create executable file: %v", err)
	}

	return path
}

// CreateSubDir creates a subdirectory
func (f *TestFixtures) CreateSubDir(name string) string {
	path := filepath.Join(f.TempDir, name)

	if err := os.MkdirAll(path, 0755); err != nil {
		f.t.Fatalf("Failed to create subdirectory: %v", err)
	}

	return path
}

// SampleJSONRPCRequest returns a sample JSON-RPC request
func (f *TestFixtures) SampleJSONRPCRequest(method string, id interface{}, params interface{}) string {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}

	if id != nil {
		req["id"] = id
	}

	if params != nil {
		paramsBytes, _ := json.Marshal(params) //nolint:errcheck
		req["params"] = json.RawMessage(paramsBytes)
	}

	data, _ := json.Marshal(req) //nolint:errcheck
	return string(data)
}

// SampleMCPInitializeRequest returns a sample MCP initialize request
func (f *TestFixtures) SampleMCPInitializeRequest() string {
	return `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"clientInfo": {
				"name": "test-client",
				"version": "1.0.0"
			}
		}
	}`
}

// SampleMCPToolsCallRequest returns a sample tools/call request
func (f *TestFixtures) SampleMCPToolsCallRequest(toolName string, args map[string]interface{}, id interface{}) string {
	params := map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	}

	return f.SampleJSONRPCRequest("tools/call", id, params)
}

// ============================================================================
// Assertion Helpers
// ============================================================================

// AssertJSONEqual compares two JSON strings for equality
func AssertJSONEqual(t *testing.T, expected, actual string) {
	t.Helper()

	var expectedObj, actualObj interface{}

	if err := json.Unmarshal([]byte(expected), &expectedObj); err != nil {
		t.Fatalf("Failed to parse expected JSON: %v", err)
	}

	if err := json.Unmarshal([]byte(actual), &actualObj); err != nil {
		t.Fatalf("Failed to parse actual JSON: %v", err)
	}

	expectedBytes, _ := json.Marshal(expectedObj) //nolint:errcheck
	actualBytes, _ := json.Marshal(actualObj)     //nolint:errcheck

	if string(expectedBytes) != string(actualBytes) {
		t.Errorf("JSON not equal:\nExpected: %s\nActual: %s", expected, actual)
	}
}

// AssertJSONContains checks if a JSON string contains expected key-value pairs
func AssertJSONContains(t *testing.T, jsonStr string, expected map[string]interface{}) {
	t.Helper()

	var actual map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &actual); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			t.Errorf("Missing key %q in JSON", key)
			continue
		}

		expectedBytes, _ := json.Marshal(expectedValue) //nolint:errcheck
		actualBytes, _ := json.Marshal(actualValue)     //nolint:errcheck

		if string(expectedBytes) != string(actualBytes) {
			t.Errorf("Key %q: expected %v, got %v", key, expectedValue, actualValue)
		}
	}
}

// AssertHTTPStatus checks the HTTP status code
func AssertHTTPStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()

	if resp.StatusCode != expected {
		t.Errorf("Expected status %d, got %d", expected, resp.StatusCode)
	}
}

// AssertContains checks if a string contains a substring
func AssertContains(t *testing.T, s, substr string) {
	t.Helper()

	if !strings.Contains(s, substr) {
		t.Errorf("Expected string to contain %q, but it didn't:\n%s", substr, s)
	}
}

// AssertNotContains checks if a string does not contain a substring
func AssertNotContains(t *testing.T, s, substr string) {
	t.Helper()

	if strings.Contains(s, substr) {
		t.Errorf("Expected string to not contain %q, but it did:\n%s", substr, s)
	}
}

// AssertNoError fails if err is not nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// AssertError fails if err is nil
func AssertError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Error("Expected an error, got nil")
	}
}

// AssertErrorContains fails if err is nil or doesn't contain the expected message
func AssertErrorContains(t *testing.T, err error, expected string) {
	t.Helper()

	if err == nil {
		t.Errorf("Expected error containing %q, got nil", expected)
		return
	}

	if !strings.Contains(err.Error(), expected) {
		t.Errorf("Expected error containing %q, got: %v", expected, err)
	}
}

// AssertEqual compares two values for equality
func AssertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()

	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

// AssertNotNil fails if the value is nil
func AssertNotNil(t *testing.T, value interface{}) {
	t.Helper()

	if value == nil {
		t.Error("Expected non-nil value, got nil")
	}
}

// ============================================================================
// Context Helpers
// ============================================================================

// ContextWithTimeout creates a context with a timeout and cleanup
func ContextWithTimeout(t *testing.T, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	return ctx, cancel
}

// ContextWithCancel creates a cancellable context with cleanup
func ContextWithCancel(t *testing.T) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx, cancel
}

// ============================================================================
// Wait Helpers
// ============================================================================

// WaitForCondition waits for a condition to become true
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Errorf("Condition not met within %v: %s", timeout, message)
}

// WaitForServer waits for an HTTP server to become available
func WaitForServer(t *testing.T, url string, timeout time.Duration) {
	t.Helper()

	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Errorf("Server at %s not available within %v", url, timeout)
}

// ============================================================================
// Capture Helpers
// ============================================================================

// CaptureStdout captures stdout output during function execution
func CaptureStdout(t *testing.T, f func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	os.Stdout = w

	outCh := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r) //nolint:errcheck
		outCh <- buf.String()
	}()

	f()

	_ = w.Close()
	os.Stdout = old

	return <-outCh
}

// CaptureStderr captures stderr output during function execution
func CaptureStderr(t *testing.T, f func()) string {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	os.Stderr = w

	outCh := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r) //nolint:errcheck
		outCh <- buf.String()
	}()

	f()

	_ = w.Close()
	os.Stderr = old

	return <-outCh
}

// ============================================================================
// Test Logger
// ============================================================================

// TestLogger provides a logger for tests that captures output
type TestLogger struct {
	Buffer *bytes.Buffer
	mu     sync.Mutex
}

// NewTestLogger creates a new test logger
func NewTestLogger() *TestLogger {
	return &TestLogger{
		Buffer: new(bytes.Buffer),
	}
}

// Write implements io.Writer
func (l *TestLogger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Buffer.Write(p)
}

// String returns the captured log output
func (l *TestLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Buffer.String()
}

// Reset clears the buffer
func (l *TestLogger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Buffer.Reset()
}

// Contains checks if the log contains a string
func (l *TestLogger) Contains(s string) bool {
	return strings.Contains(l.String(), s)
}

// ============================================================================
// HTTP Test Helpers
// ============================================================================

// HTTPTestRequest creates an HTTP test request
func HTTPTestRequest(method, path string, body string) *http.Request {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" && method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	return req
}

// HTTPTestRequestWithHeaders creates an HTTP test request with custom headers
func HTTPTestRequestWithHeaders(method, path, body string, headers map[string]string) *http.Request {
	req := HTTPTestRequest(method, path, body)

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return req
}

// ExecuteHTTPHandler executes an HTTP handler and returns the response
func ExecuteHTTPHandler(t *testing.T, handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	return w
}

// ============================================================================
// Mock MCP Script Generator
// ============================================================================

// GenerateMockMCPScript generates a bash script that acts as an MCP server
func GenerateMockMCPScript(options *MockMCPScriptOptions) string {
	if options == nil {
		options = &MockMCPScriptOptions{}
	}

	// Set defaults
	if options.ServerName == "" {
		options.ServerName = "mock-mcp"
	}
	if options.Version == "" {
		options.Version = "1.0.0"
	}

	var sb strings.Builder

	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("# Auto-generated Mock MCP Server\n\n")

	// Add delay function if needed
	if options.ResponseDelay > 0 {
		sb.WriteString(fmt.Sprintf("DELAY=%f\n", options.ResponseDelay.Seconds()))
		sb.WriteString("delay() { sleep $DELAY; }\n\n")
	}

	// ID extraction function
	sb.WriteString(`extract_id() {
    echo "$1" | grep -oP '"id"\s*:\s*\K[0-9]+' || echo "null"
}
`)

	// Main loop
	sb.WriteString("\nwhile IFS= read -r line; do\n")

	// Initialize handler
	sb.WriteString(`    if echo "$line" | grep -q '"method"[[:space:]]*:[[:space:]]*"initialize"'; then
        `)
	if options.ResponseDelay > 0 {
		sb.WriteString("delay; ")
	}
	sb.WriteString(fmt.Sprintf(`echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"%s","version":"%s"}}}'
`, options.ServerName, options.Version))

	// Notifications handler
	sb.WriteString(`    elif echo "$line" | grep -q '"method"[[:space:]]*:[[:space:]]*"notifications/"'; then
        :
`)

	// Custom handlers
	for method, response := range options.CustomHandlers {
		sb.WriteString(fmt.Sprintf(`    elif echo "$line" | grep -q '"method"[[:space:]]*:[[:space:]]*"%s"'; then
        id=$(extract_id "$line")
        `, method))
		if options.ResponseDelay > 0 {
			sb.WriteString("delay; ")
		}
		sb.WriteString(fmt.Sprintf(`echo "%s" | sed "s/\\$id/$id/g"
`, strings.ReplaceAll(response, `"`, `\"`)))
	}

	// Default ping handler
	sb.WriteString(`    elif echo "$line" | grep -q '"method"[[:space:]]*:[[:space:]]*"ping"'; then
        id=$(extract_id "$line")
        `)
	if options.ResponseDelay > 0 {
		sb.WriteString("delay; ")
	}
	sb.WriteString(`echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":\"pong\"}"
`)

	// Exit handler
	if options.ExitOnMethod != "" {
		sb.WriteString(fmt.Sprintf(`    elif echo "$line" | grep -q '"method"[[:space:]]*:[[:space:]]*"%s"'; then
        exit %d
`, options.ExitOnMethod, options.ExitCode))
	}

	// Default handler for unknown methods
	sb.WriteString(`    elif echo "$line" | grep -q '"method"'; then
        id=$(extract_id "$line")
        if [ "$id" != "null" ]; then
            `)
	if options.ResponseDelay > 0 {
		sb.WriteString("delay; ")
	}
	sb.WriteString(`echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"echo\":\"received\"}}"
        fi
    fi
done
`)

	return sb.String()
}

// MockMCPScriptOptions configures the generated mock MCP script
type MockMCPScriptOptions struct {
	// ServerName is the name returned in serverInfo
	ServerName string

	// Version is the version returned in serverInfo
	Version string

	// ResponseDelay adds a delay before each response
	ResponseDelay time.Duration

	// CustomHandlers maps method names to response JSON templates
	// Use $id as placeholder for the request ID
	CustomHandlers map[string]string

	// ExitOnMethod causes the script to exit when this method is called
	ExitOnMethod string

	// ExitCode is the exit code to use when ExitOnMethod is triggered
	ExitCode int
}

// ============================================================================
// Retry Helper
// ============================================================================

// Retry retries a function until it succeeds or timeout
func Retry(t *testing.T, attempts int, delay time.Duration, f func() error) error {
	t.Helper()

	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := f(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if i < attempts-1 {
			time.Sleep(delay)
		}
	}

	return lastErr
}
