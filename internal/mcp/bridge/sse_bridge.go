// Package bridge provides HTTP/SSE bridge functionality for stdio-based MCP servers.
//
// The SSE bridge wraps any stdio-based MCP server and exposes it via HTTP endpoints,
// enabling web clients to communicate with MCP servers using Server-Sent Events (SSE)
// for streaming responses and standard HTTP POST for sending requests.
//
// Architecture:
//
//	┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
//	│   HTTP Client   │ ──── │   SSE Bridge    │ ──── │   MCP Server    │
//	│  (SSE + POST)   │      │   (Go Server)   │      │  (stdio-based)  │
//	└─────────────────┘      └─────────────────┘      └─────────────────┘
//
// Endpoints:
//   - GET  /sse      - Establish SSE connection for receiving MCP responses
//   - POST /message  - Send JSON-RPC requests to MCP server
//   - GET  /health   - Health check endpoint
//
// Usage:
//
//	bridge, err := NewSSEBridge(SSEBridgeConfig{
//	    Command:     []string{"npx", "-y", "mcp-fetch-server"},
//	    Environment: map[string]string{"NODE_ENV": "production"},
//	    Address:     ":8080",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer bridge.Shutdown(context.Background())
//
//	if err := bridge.Start(); err != nil {
//	    log.Fatal(err)
//	}
package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"github.com/sirupsen/logrus"
)

// SSEBridgeConfig holds configuration for the SSE bridge server.
type SSEBridgeConfig struct {
	// Command is the command to run the MCP server (e.g., ["npx", "-y", "mcp-fetch-server"])
	Command []string

	// Environment contains additional environment variables for the MCP process
	Environment map[string]string

	// Address is the HTTP server listen address (e.g., ":8080", "localhost:3000")
	Address string

	// ReadTimeout is the maximum duration for reading the entire request
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the next request
	IdleTimeout time.Duration

	// ShutdownTimeout is the timeout for graceful shutdown
	ShutdownTimeout time.Duration

	// MaxRequestSize is the maximum size of request body in bytes
	MaxRequestSize int64

	// SSEHeartbeatInterval is the interval for SSE heartbeat messages
	SSEHeartbeatInterval time.Duration

	// Logger is the logger instance (optional, creates default if nil)
	Logger *logrus.Logger

	// OnProcessExit is called when the MCP process exits unexpectedly (optional)
	OnProcessExit func(error)

	// WorkingDirectory is the working directory for the MCP process (optional)
	WorkingDirectory string
}

// DefaultSSEBridgeConfig returns a configuration with sensible defaults.
func DefaultSSEBridgeConfig() SSEBridgeConfig {
	return SSEBridgeConfig{
		Address:              ":8080",
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         30 * time.Second,
		IdleTimeout:          120 * time.Second,
		ShutdownTimeout:      30 * time.Second,
		MaxRequestSize:       10 * 1024 * 1024, // 10MB
		SSEHeartbeatInterval: 30 * time.Second,
	}
}

// SSEBridgeState represents the current state of the bridge.
type SSEBridgeState int32

const (
	// StateIdle indicates the bridge is created but not started
	StateIdle SSEBridgeState = iota
	// StateStarting indicates the bridge is in the process of starting
	StateStarting
	// StateRunning indicates the bridge is running and accepting connections
	StateRunning
	// StateStopping indicates the bridge is in the process of stopping
	StateStopping
	// StateStopped indicates the bridge has been stopped
	StateStopped
	// StateError indicates the bridge encountered an error
	StateError
)

// String returns a string representation of the state.
func (s SSEBridgeState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// SSEBridgeMetrics holds runtime metrics for the bridge.
type SSEBridgeMetrics struct {
	// TotalRequests is the total number of requests received
	TotalRequests int64
	// SuccessfulRequests is the number of successful requests
	SuccessfulRequests int64
	// FailedRequests is the number of failed requests
	FailedRequests int64
	// ActiveSSEConnections is the current number of active SSE connections
	ActiveSSEConnections int64
	// TotalSSEConnections is the total number of SSE connections made
	TotalSSEConnections int64
	// BytesSent is the total bytes sent to clients
	BytesSent int64
	// BytesReceived is the total bytes received from clients
	BytesReceived int64
	// ProcessRestarts is the number of times the MCP process was restarted
	ProcessRestarts int64
	// StartTime is when the bridge was started
	StartTime time.Time
	// LastRequestTime is the time of the last request
	LastRequestTime time.Time
}

// SSEClient represents a connected SSE client.
type SSEClient struct {
	ID        string
	Writer    http.ResponseWriter
	Flusher   http.Flusher
	Done      chan struct{}
	CreatedAt time.Time
}

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC error codes
const (
	JSONRPCParseError     = -32700
	JSONRPCInvalidRequest = -32600
	JSONRPCMethodNotFound = -32601
	JSONRPCInvalidParams  = -32602
	JSONRPCInternalError  = -32603
	// Server errors
	JSONRPCServerError      = -32000
	JSONRPCProcessNotReady  = -32001
	JSONRPCProcessClosed    = -32002
	JSONRPCTimeout          = -32003
	JSONRPCBridgeShutdown   = -32004
	JSONRPCRequestTooLarge  = -32005
	JSONRPCTooManyRequests  = -32006
	JSONRPCConnectionClosed = -32007
)

// normalizeID normalizes a JSON-RPC ID to a consistent type for map lookups.
// JSON numbers become float64 when unmarshaled, so we need to handle the conversion.
func normalizeID(id interface{}) interface{} {
	switch v := id.(type) {
	case float64:
		// If it's a whole number, convert to int64 for consistent comparison
		if v == float64(int64(v)) {
			return int64(v)
		}
		return v
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		return v
	default:
		return id
	}
}

// SSEBridge wraps a stdio-based MCP server and exposes it via HTTP/SSE.
type SSEBridge struct {
	config     SSEBridgeConfig
	state      int32 // atomic SSEBridgeState
	logger     *logrus.Logger
	metrics    *SSEBridgeMetrics
	httpServer *http.Server
	mux        *http.ServeMux

	// Process management
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       io.ReadCloser
	stderr       io.ReadCloser
	scanner      *bufio.Scanner
	processLock  sync.Mutex
	processReady bool
	processDone  chan struct{}

	// Request tracking for correlating responses (CONST-029: safe.Store)
	pendingRequests *safe.Store[interface{}, chan *JSONRPCResponse]
	nextRequestID   int64

	// SSE client management (CONST-029: safe.Store)
	sseClients *safe.Store[string, *SSEClient]

	// Shutdown coordination
	shutdownOnce sync.Once
	shutdownDone chan struct{}

	// Write mutex for stdin
	stdinMux sync.Mutex

	// Read mutex for scanner
	scannerMux sync.Mutex
}

// NewSSEBridge creates a new SSE bridge with the given configuration.
func NewSSEBridge(config SSEBridgeConfig) (*SSEBridge, error) {
	// Validate configuration
	if len(config.Command) == 0 {
		return nil, fmt.Errorf("command is required")
	}

	// Apply defaults
	if config.Address == "" {
		config.Address = DefaultSSEBridgeConfig().Address
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = DefaultSSEBridgeConfig().ReadTimeout
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = DefaultSSEBridgeConfig().WriteTimeout
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = DefaultSSEBridgeConfig().IdleTimeout
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = DefaultSSEBridgeConfig().ShutdownTimeout
	}
	if config.MaxRequestSize == 0 {
		config.MaxRequestSize = DefaultSSEBridgeConfig().MaxRequestSize
	}
	if config.SSEHeartbeatInterval == 0 {
		config.SSEHeartbeatInterval = DefaultSSEBridgeConfig().SSEHeartbeatInterval
	}

	// Create logger if not provided
	logger := config.Logger
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
	}

	bridge := &SSEBridge{
		config:          config,
		state:           int32(StateIdle),
		logger:          logger,
		metrics:         &SSEBridgeMetrics{},
		pendingRequests: safe.NewStore[interface{}, chan *JSONRPCResponse](),
		sseClients:      safe.NewStore[string, *SSEClient](),
		shutdownDone:    make(chan struct{}),
		processDone:     make(chan struct{}),
	}

	// Set up HTTP mux
	bridge.mux = http.NewServeMux()
	bridge.mux.HandleFunc("/sse", bridge.handleSSE)
	bridge.mux.HandleFunc("/message", bridge.handleMessage)
	bridge.mux.HandleFunc("/health", bridge.handleHealth)

	// Create HTTP server
	bridge.httpServer = &http.Server{
		Addr:         config.Address,
		Handler:      bridge.mux,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	return bridge, nil
}

// Start starts the MCP process and HTTP server.
func (b *SSEBridge) Start() error {
	// Check state
	if !atomic.CompareAndSwapInt32(&b.state, int32(StateIdle), int32(StateStarting)) {
		currentState := SSEBridgeState(atomic.LoadInt32(&b.state))
		return fmt.Errorf("cannot start bridge in state %s", currentState)
	}

	b.logger.WithFields(logrus.Fields{
		"address": b.config.Address,
		"command": b.config.Command,
	}).Info("Starting SSE bridge")

	// Start MCP process
	if err := b.startProcess(); err != nil {
		atomic.StoreInt32(&b.state, int32(StateError))
		return fmt.Errorf("failed to start MCP process: %w", err)
	}

	// Initialize MCP connection
	if err := b.initializeMCP(); err != nil {
		b.stopProcess()
		atomic.StoreInt32(&b.state, int32(StateError))
		return fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	// Start stdout reader
	go b.readStdout()

	// Start stderr reader
	go b.readStderr()

	// Start process monitor
	go b.monitorProcess()

	// Update state and metrics
	atomic.StoreInt32(&b.state, int32(StateRunning))
	b.metrics.StartTime = time.Now()

	// Start HTTP server in goroutine
	go func() {
		b.logger.WithField("address", b.config.Address).Info("HTTP server listening")
		if err := b.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			b.logger.WithError(err).Error("HTTP server error")
			atomic.StoreInt32(&b.state, int32(StateError))
		}
	}()

	return nil
}

// startProcess starts the MCP subprocess.
func (b *SSEBridge) startProcess() error {
	b.processLock.Lock()
	defer b.processLock.Unlock()

	// Create command
	b.cmd = exec.Command(b.config.Command[0], b.config.Command[1:]...)

	// Set working directory if specified
	if b.config.WorkingDirectory != "" {
		b.cmd.Dir = b.config.WorkingDirectory
	}

	// Set environment
	b.cmd.Env = os.Environ()
	for key, value := range b.config.Environment {
		b.cmd.Env = append(b.cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Create pipes
	var err error
	b.stdin, err = b.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	b.stdout, err = b.cmd.StdoutPipe()
	if err != nil {
		_ = b.stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	b.stderr, err = b.cmd.StderrPipe()
	if err != nil {
		_ = b.stdin.Close()
		_ = b.stdout.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start process
	if err := b.cmd.Start(); err != nil {
		_ = b.stdin.Close()
		_ = b.stdout.Close()
		_ = b.stderr.Close()
		return fmt.Errorf("failed to start process: %w", err)
	}

	b.logger.WithFields(logrus.Fields{
		"pid":     b.cmd.Process.Pid,
		"command": b.config.Command,
	}).Debug("MCP process started")

	// Create scanner for stdout with large buffer
	const maxScanTokenSize = 10 * 1024 * 1024 // 10MB
	b.scanner = bufio.NewScanner(b.stdout)
	buf := make([]byte, maxScanTokenSize)
	b.scanner.Buffer(buf, maxScanTokenSize)

	// Reset process done channel
	b.processDone = make(chan struct{})

	return nil
}

// stopProcess stops the MCP subprocess.
func (b *SSEBridge) stopProcess() {
	b.processLock.Lock()
	defer b.processLock.Unlock()

	b.processReady = false

	// Clear scanner to stop readStdout goroutine
	b.scannerMux.Lock()
	b.scanner = nil
	b.scannerMux.Unlock()

	if b.stdin != nil {
		_ = b.stdin.Close()
		b.stdin = nil
	}

	if b.cmd != nil && b.cmd.Process != nil {
		// Try graceful termination first
		_ = b.cmd.Process.Signal(os.Interrupt) //nolint:errcheck

		// Wait briefly, then force kill
		done := make(chan struct{})
		go func() {
			_ = b.cmd.Wait() //nolint:errcheck
			close(done)
		}()

		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(5 * time.Second):
			// Force kill
			_ = b.cmd.Process.Kill() //nolint:errcheck
			<-done
		}

		b.cmd = nil
	}

	// Signal that process is done
	select {
	case <-b.processDone:
		// Already closed
	default:
		close(b.processDone)
	}
}

// initializeMCP performs the MCP initialization handshake.
func (b *SSEBridge) initializeMCP() error {
	// Create initialize request
	initRequest := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: json.RawMessage(`{
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"clientInfo": {
				"name": "helixagent-sse-bridge",
				"version": "1.0.0"
			}
		}`),
	}

	// Send initialize request
	if err := b.sendToProcess(&initRequest); err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	// Read initialize response (with timeout) using shared scanner
	respChan := make(chan *JSONRPCResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		b.scannerMux.Lock()
		defer b.scannerMux.Unlock()

		if b.scanner.Scan() {
			var resp JSONRPCResponse
			if err := json.Unmarshal(b.scanner.Bytes(), &resp); err != nil {
				errChan <- fmt.Errorf("failed to parse initialize response: %w", err)
				return
			}
			respChan <- &resp
		} else {
			if err := b.scanner.Err(); err != nil {
				errChan <- fmt.Errorf("failed to read initialize response: %w", err)
			} else {
				errChan <- fmt.Errorf("failed to read initialize response: unexpected EOF")
			}
		}
	}()

	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return fmt.Errorf("initialize error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
		}
		b.logger.Debug("MCP initialize response received")
	case err := <-errChan:
		return err
	case <-time.After(30 * time.Second):
		return fmt.Errorf("initialize timeout")
	}

	// Send initialized notification
	initializedNotif := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  json.RawMessage(`{}`),
	}

	if err := b.sendToProcess(&initializedNotif); err != nil {
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}

	b.processLock.Lock()
	b.processReady = true
	b.processLock.Unlock()

	b.logger.Info("MCP connection initialized successfully")
	return nil
}

// sendToProcess sends a JSON-RPC message to the MCP process.
func (b *SSEBridge) sendToProcess(msg interface{}) error {
	b.stdinMux.Lock()
	defer b.stdinMux.Unlock()

	b.processLock.Lock()
	if b.stdin == nil {
		b.processLock.Unlock()
		return fmt.Errorf("process stdin not available")
	}
	stdin := b.stdin
	b.processLock.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if _, err := stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write to process: %w", err)
	}

	atomic.AddInt64(&b.metrics.BytesSent, int64(len(data)+1))

	b.logger.WithField("message", string(data)).Debug("Sent message to MCP process")
	return nil
}

// readStdout continuously reads from the MCP process stdout and processes responses.
func (b *SSEBridge) readStdout() {
	// Get the scanner reference once - it won't change while we're reading
	b.scannerMux.Lock()
	scanner := b.scanner
	b.scannerMux.Unlock()

	if scanner == nil {
		return
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Make a copy since scanner.Bytes() is reused
		lineCopy := make([]byte, len(line))
		copy(lineCopy, line)

		atomic.AddInt64(&b.metrics.BytesReceived, int64(len(lineCopy)))

		// Try to parse as JSON-RPC response
		var resp JSONRPCResponse
		if err := json.Unmarshal(lineCopy, &resp); err != nil {
			b.logger.WithError(err).WithField("line", string(lineCopy)).Warn("Failed to parse MCP response")
			continue
		}

		b.logger.WithFields(logrus.Fields{
			"id":     resp.ID,
			"method": resp.Result,
		}).Debug("Received MCP response")

		// Check if this is a response to a pending request
		if resp.ID != nil {
			normalizedID := normalizeID(resp.ID)
			ch, exists := b.pendingRequests.Get(normalizedID)
			if exists {
				select {
				case ch <- &resp:
				default:
					b.logger.Warn("Response channel full, dropping response")
				}
			}
		}

		// Broadcast to all SSE clients
		b.broadcastToSSE(&resp)
	}

	if err := scanner.Err(); err != nil {
		b.logger.WithError(err).Error("Error reading from MCP process stdout")
	}
}

// readStderr continuously reads from the MCP process stderr.
func (b *SSEBridge) readStderr() {
	scanner := bufio.NewScanner(b.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		b.logger.WithField("stderr", line).Warn("MCP process stderr")
	}

	if err := scanner.Err(); err != nil {
		b.logger.WithError(err).Error("Error reading from MCP process stderr")
	}
}

// monitorProcess monitors the MCP process and handles unexpected exits.
func (b *SSEBridge) monitorProcess() {
	b.processLock.Lock()
	cmd := b.cmd
	b.processLock.Unlock()

	if cmd == nil {
		return
	}

	err := cmd.Wait()

	// Check if bridge is shutting down
	state := SSEBridgeState(atomic.LoadInt32(&b.state))
	if state == StateStopping || state == StateStopped {
		return
	}

	b.logger.WithError(err).Warn("MCP process exited unexpectedly")
	atomic.AddInt64(&b.metrics.ProcessRestarts, 1)

	// Call exit callback if set
	if b.config.OnProcessExit != nil {
		b.config.OnProcessExit(err)
	}

	// Close pending requests
	b.pendingRequests.Range(func(id interface{}, ch chan *JSONRPCResponse) bool {
		select {
		case ch <- &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error: &JSONRPCError{
				Code:    JSONRPCProcessClosed,
				Message: "MCP process exited unexpectedly",
			},
		}:
		default:
		}
		close(ch)
		return true
	})
	b.pendingRequests.Clear()

	// Signal process done
	select {
	case <-b.processDone:
	default:
		close(b.processDone)
	}
}

// broadcastToSSE sends a response to all connected SSE clients.
func (b *SSEBridge) broadcastToSSE(resp *JSONRPCResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		b.logger.WithError(err).Error("Failed to marshal SSE response")
		return
	}

	clients := make([]*SSEClient, 0, b.sseClients.Len())
	b.sseClients.Range(func(_ string, client *SSEClient) bool {
		clients = append(clients, client)
		return true
	})

	for _, client := range clients {
		select {
		case <-client.Done:
			continue
		default:
		}

		// Write SSE event
		_, err := fmt.Fprintf(client.Writer, "data: %s\n\n", data)
		if err != nil {
			b.logger.WithError(err).WithField("client", client.ID).Warn("Failed to write to SSE client")
			b.removeSSEClient(client.ID)
			continue
		}

		client.Flusher.Flush()
		atomic.AddInt64(&b.metrics.BytesSent, int64(len(data)+8)) // +8 for "data: " and "\n\n"
	}
}

// handleSSE handles SSE connection requests.
func (b *SSEBridge) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Check method
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check bridge state
	state := SSEBridgeState(atomic.LoadInt32(&b.state))
	if state != StateRunning {
		http.Error(w, fmt.Sprintf("Bridge not ready: %s", state), http.StatusServiceUnavailable)
		return
	}

	// Check if flusher is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no")

	// Create client
	clientID := fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddInt64(&b.metrics.TotalSSEConnections, 1))
	client := &SSEClient{
		ID:        clientID,
		Writer:    w,
		Flusher:   flusher,
		Done:      make(chan struct{}),
		CreatedAt: time.Now(),
	}

	// Register client
	b.sseClients.Put(clientID, client)

	atomic.AddInt64(&b.metrics.ActiveSSEConnections, 1)

	b.logger.WithField("client", clientID).Info("SSE client connected")

	// Send initial connection event
	_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"clientId\":\"%s\"}\n\n", clientID)
	flusher.Flush()

	// Send the endpoint information (MCP SSE standard)
	// #nosec G705 -- SSE data is not rendered as HTML; r.Host is used per MCP SSE protocol spec
	messageEndpoint := fmt.Sprintf("http://%s/message", r.Host)
	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", messageEndpoint)
	flusher.Flush()

	// Start heartbeat
	heartbeatTicker := time.NewTicker(b.config.SSEHeartbeatInterval)
	defer heartbeatTicker.Stop()

	// Wait for client disconnect or bridge shutdown
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			b.logger.WithField("client", clientID).Info("SSE client disconnected")
			b.removeSSEClient(clientID)
			return
		case <-b.shutdownDone:
			_, _ = fmt.Fprintf(w, "event: shutdown\ndata: {\"reason\":\"bridge shutting down\"}\n\n")
			flusher.Flush()
			b.removeSSEClient(clientID)
			return
		case <-heartbeatTicker.C:
			// Send heartbeat
			_, _ = fmt.Fprintf(w, ":heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// removeSSEClient removes an SSE client from the registry.
func (b *SSEBridge) removeSSEClient(clientID string) {
	if client, ok := b.sseClients.Delete(clientID); ok {
		close(client.Done)
		atomic.AddInt64(&b.metrics.ActiveSSEConnections, -1)
	}
}

// handleMessage handles POST requests with JSON-RPC messages.
func (b *SSEBridge) handleMessage(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&b.metrics.TotalRequests, 1)
	b.metrics.LastRequestTime = time.Now()

	// Check method
	if r.Method != http.MethodPost {
		b.writeJSONRPCError(w, nil, JSONRPCInvalidRequest, "Method not allowed", nil)
		atomic.AddInt64(&b.metrics.FailedRequests, 1)
		return
	}

	// Check bridge state
	state := SSEBridgeState(atomic.LoadInt32(&b.state))
	if state != StateRunning {
		b.writeJSONRPCError(w, nil, JSONRPCServerError, fmt.Sprintf("Bridge not ready: %s", state), nil)
		atomic.AddInt64(&b.metrics.FailedRequests, 1)
		return
	}

	// Check process readiness
	b.processLock.Lock()
	ready := b.processReady
	b.processLock.Unlock()
	if !ready {
		b.writeJSONRPCError(w, nil, JSONRPCProcessNotReady, "MCP process not ready", nil)
		atomic.AddInt64(&b.metrics.FailedRequests, 1)
		return
	}

	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" && contentType != "" {
		b.writeJSONRPCError(w, nil, JSONRPCInvalidRequest, "Content-Type must be application/json", nil)
		atomic.AddInt64(&b.metrics.FailedRequests, 1)
		return
	}

	// Read request body with size limit
	body := http.MaxBytesReader(w, r.Body, b.config.MaxRequestSize)
	data, err := io.ReadAll(body)
	if err != nil {
		if err.Error() == "http: request body too large" {
			b.writeJSONRPCError(w, nil, JSONRPCRequestTooLarge, "Request body too large", nil)
		} else {
			b.writeJSONRPCError(w, nil, JSONRPCParseError, "Failed to read request body", nil)
		}
		atomic.AddInt64(&b.metrics.FailedRequests, 1)
		return
	}

	atomic.AddInt64(&b.metrics.BytesReceived, int64(len(data)))

	// Parse JSON-RPC request
	var req JSONRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		b.writeJSONRPCError(w, nil, JSONRPCParseError, "Invalid JSON", err.Error())
		atomic.AddInt64(&b.metrics.FailedRequests, 1)
		return
	}

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		b.writeJSONRPCError(w, req.ID, JSONRPCInvalidRequest, "Invalid JSON-RPC version", nil)
		atomic.AddInt64(&b.metrics.FailedRequests, 1)
		return
	}

	// Validate method
	if req.Method == "" {
		b.writeJSONRPCError(w, req.ID, JSONRPCInvalidRequest, "Method is required", nil)
		atomic.AddInt64(&b.metrics.FailedRequests, 1)
		return
	}

	b.logger.WithFields(logrus.Fields{
		"method": req.Method,
		"id":     req.ID,
	}).Debug("Received JSON-RPC request")

	// For notifications (no ID), just send to process and return 204
	if req.ID == nil {
		if err := b.sendToProcess(&req); err != nil {
			b.writeJSONRPCError(w, nil, JSONRPCInternalError, "Failed to send to MCP process", err.Error())
			atomic.AddInt64(&b.metrics.FailedRequests, 1)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		atomic.AddInt64(&b.metrics.SuccessfulRequests, 1)
		return
	}

	// For requests (with ID), wait for response
	respChan := make(chan *JSONRPCResponse, 1)
	normalizedID := normalizeID(req.ID)

	// Register pending request
	b.pendingRequests.Put(normalizedID, respChan)
	defer b.pendingRequests.Delete(normalizedID)

	// Send to process
	if err := b.sendToProcess(&req); err != nil {
		b.writeJSONRPCError(w, req.ID, JSONRPCInternalError, "Failed to send to MCP process", err.Error())
		atomic.AddInt64(&b.metrics.FailedRequests, 1)
		return
	}

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		if resp == nil {
			b.writeJSONRPCError(w, req.ID, JSONRPCInternalError, "Received nil response", nil)
			atomic.AddInt64(&b.metrics.FailedRequests, 1)
			return
		}
		b.writeJSONRPCResponse(w, resp)
		atomic.AddInt64(&b.metrics.SuccessfulRequests, 1)
	case <-b.processDone:
		b.writeJSONRPCError(w, req.ID, JSONRPCProcessClosed, "MCP process closed", nil)
		atomic.AddInt64(&b.metrics.FailedRequests, 1)
	case <-time.After(b.config.WriteTimeout):
		b.writeJSONRPCError(w, req.ID, JSONRPCTimeout, "Request timeout", nil)
		atomic.AddInt64(&b.metrics.FailedRequests, 1)
	case <-r.Context().Done():
		b.writeJSONRPCError(w, req.ID, JSONRPCConnectionClosed, "Client connection closed", nil)
		atomic.AddInt64(&b.metrics.FailedRequests, 1)
	}
}

// handleHealth handles health check requests.
func (b *SSEBridge) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := SSEBridgeState(atomic.LoadInt32(&b.state))

	b.processLock.Lock()
	processReady := b.processReady
	var processPid int
	if b.cmd != nil && b.cmd.Process != nil {
		processPid = b.cmd.Process.Pid
	}
	b.processLock.Unlock()

	health := map[string]interface{}{
		"status":       state.String(),
		"healthy":      state == StateRunning && processReady,
		"processReady": processReady,
		"processPid":   processPid,
		"uptime":       time.Since(b.metrics.StartTime).String(),
		"metrics": map[string]interface{}{
			"totalRequests":        atomic.LoadInt64(&b.metrics.TotalRequests),
			"successfulRequests":   atomic.LoadInt64(&b.metrics.SuccessfulRequests),
			"failedRequests":       atomic.LoadInt64(&b.metrics.FailedRequests),
			"activeSSEConnections": atomic.LoadInt64(&b.metrics.ActiveSSEConnections),
			"totalSSEConnections":  atomic.LoadInt64(&b.metrics.TotalSSEConnections),
			"bytesSent":            atomic.LoadInt64(&b.metrics.BytesSent),
			"bytesReceived":        atomic.LoadInt64(&b.metrics.BytesReceived),
			"processRestarts":      atomic.LoadInt64(&b.metrics.ProcessRestarts),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if state != StateRunning || !processReady {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(health) //nolint:errcheck
}

// writeJSONRPCResponse writes a JSON-RPC response.
func (b *SSEBridge) writeJSONRPCResponse(w http.ResponseWriter, resp *JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(resp)
	if err != nil {
		b.logger.WithError(err).Error("Failed to marshal JSON-RPC response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data) //nolint:errcheck
	atomic.AddInt64(&b.metrics.BytesSent, int64(len(data)))
}

// writeJSONRPCError writes a JSON-RPC error response.
func (b *SSEBridge) writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	resp := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	b.writeJSONRPCResponse(w, resp)
}

// Shutdown gracefully shuts down the SSE bridge.
func (b *SSEBridge) Shutdown(ctx context.Context) error {
	var shutdownErr error

	b.shutdownOnce.Do(func() {
		b.logger.Info("Shutting down SSE bridge")
		atomic.StoreInt32(&b.state, int32(StateStopping))

		// Signal shutdown to SSE clients
		close(b.shutdownDone)

		// Close all SSE clients
		b.sseClients.Range(func(_ string, client *SSEClient) bool {
			close(client.Done)
			return true
		})
		b.sseClients.Clear()

		// Shutdown HTTP server
		shutdownCtx, cancel := context.WithTimeout(ctx, b.config.ShutdownTimeout)
		defer cancel()

		if err := b.httpServer.Shutdown(shutdownCtx); err != nil {
			b.logger.WithError(err).Warn("HTTP server shutdown error")
			shutdownErr = err
		}

		// Stop MCP process
		b.stopProcess()

		atomic.StoreInt32(&b.state, int32(StateStopped))
		b.logger.Info("SSE bridge shutdown complete")
	})

	return shutdownErr
}

// State returns the current state of the bridge.
func (b *SSEBridge) State() SSEBridgeState {
	return SSEBridgeState(atomic.LoadInt32(&b.state))
}

// Metrics returns a copy of the current metrics.
func (b *SSEBridge) Metrics() SSEBridgeMetrics {
	return SSEBridgeMetrics{
		TotalRequests:        atomic.LoadInt64(&b.metrics.TotalRequests),
		SuccessfulRequests:   atomic.LoadInt64(&b.metrics.SuccessfulRequests),
		FailedRequests:       atomic.LoadInt64(&b.metrics.FailedRequests),
		ActiveSSEConnections: atomic.LoadInt64(&b.metrics.ActiveSSEConnections),
		TotalSSEConnections:  atomic.LoadInt64(&b.metrics.TotalSSEConnections),
		BytesSent:            atomic.LoadInt64(&b.metrics.BytesSent),
		BytesReceived:        atomic.LoadInt64(&b.metrics.BytesReceived),
		ProcessRestarts:      atomic.LoadInt64(&b.metrics.ProcessRestarts),
		StartTime:            b.metrics.StartTime,
		LastRequestTime:      b.metrics.LastRequestTime,
	}
}

// ActiveClients returns the number of active SSE clients.
func (b *SSEBridge) ActiveClients() int {
	return int(atomic.LoadInt64(&b.metrics.ActiveSSEConnections))
}

// SendNotification sends a JSON-RPC notification to the MCP process.
func (b *SSEBridge) SendNotification(method string, params interface{}) error {
	state := SSEBridgeState(atomic.LoadInt32(&b.state))
	if state != StateRunning {
		return fmt.Errorf("bridge not running: %s", state)
	}

	paramsData, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}

	notif := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsData,
	}

	return b.sendToProcess(&notif)
}

// SendRequest sends a JSON-RPC request to the MCP process and waits for a response.
func (b *SSEBridge) SendRequest(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
	state := SSEBridgeState(atomic.LoadInt32(&b.state))
	if state != StateRunning {
		return nil, fmt.Errorf("bridge not running: %s", state)
	}

	paramsData, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	// Generate unique request ID
	id := atomic.AddInt64(&b.nextRequestID, 1)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsData,
	}

	// Create response channel
	respChan := make(chan *JSONRPCResponse, 1)
	normalizedID := normalizeID(id)

	// Register pending request
	b.pendingRequests.Put(normalizedID, respChan)
	defer b.pendingRequests.Delete(normalizedID)

	// Send request
	if err := b.sendToProcess(&req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	select {
	case resp := <-respChan:
		if resp == nil {
			return nil, fmt.Errorf("received nil response")
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.processDone:
		return nil, fmt.Errorf("MCP process closed")
	}
}

// Handler returns the HTTP handler for the bridge.
// This allows embedding the bridge in existing HTTP servers.
func (b *SSEBridge) Handler() http.Handler {
	return b.mux
}

// Address returns the configured listen address.
func (b *SSEBridge) Address() string {
	return b.config.Address
}

// IsHealthy returns true if the bridge is running and the MCP process is ready.
func (b *SSEBridge) IsHealthy() bool {
	state := SSEBridgeState(atomic.LoadInt32(&b.state))
	if state != StateRunning {
		return false
	}

	b.processLock.Lock()
	ready := b.processReady
	b.processLock.Unlock()

	return ready
}
