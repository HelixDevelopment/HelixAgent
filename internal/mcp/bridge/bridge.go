// Package bridge provides an SSE bridge for MCP servers.
// It wraps stdio-based MCP servers and exposes them over HTTP with SSE support.
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
	"strings"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/google/uuid"
)

// Config holds the bridge configuration
type Config struct {
	// Port to listen on (default: 9000)
	Port int
	// MCPCommand is the command to run the MCP server (e.g., "npx @modelcontextprotocol/server-filesystem")
	MCPCommand string
	// MCPArgs are additional arguments for the MCP command
	MCPArgs []string
	// ReadTimeout for HTTP server
	ReadTimeout time.Duration
	// WriteTimeout for HTTP server
	WriteTimeout time.Duration
	// IdleTimeout for HTTP server
	IdleTimeout time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Port:         9000,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// Bridge wraps an MCP server and exposes it over HTTP/SSE
type Bridge struct {
	config  *Config
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	stdinMu sync.Mutex // serialises writes to stdin only
	clients *safe.Store[string, chan []byte]
	done    chan struct{}
}

// New creates a new Bridge instance
func New(config *Config) *Bridge {
	if config == nil {
		config = DefaultConfig()
	}
	return &Bridge{
		config:  config,
		clients: safe.NewStore[string, chan []byte](),
		done:    make(chan struct{}),
	}
}

// Start starts the bridge server
func (b *Bridge) Start(ctx context.Context) error {
	// Parse the MCP command
	if b.config.MCPCommand == "" {
		return fmt.Errorf("MCP_COMMAND is required")
	}

	// Split command into parts
	parts := strings.Fields(b.config.MCPCommand)
	if len(parts) == 0 {
		return fmt.Errorf("invalid MCP_COMMAND: empty after parsing")
	}

	cmdName := parts[0]
	cmdArgs := parts[1:]
	cmdArgs = append(cmdArgs, b.config.MCPArgs...)

	// Start the MCP server process
	b.cmd = exec.CommandContext(ctx, cmdName, cmdArgs...)
	b.cmd.Env = os.Environ()

	var err error
	b.stdin, err = b.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	b.stdout, err = b.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	b.stderr, err = b.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := b.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	// Start reading from stdout (MCP responses)
	go b.readResponses()

	// Start reading stderr for logging
	go b.readStderr()

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", b.handleHealth)
	mux.HandleFunc("/sse", b.handleSSE)
	mux.HandleFunc("/message", b.handleMessage)
	mux.HandleFunc("/", b.handleRoot)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", b.config.Port),
		Handler:      mux,
		ReadTimeout:  b.config.ReadTimeout,
		WriteTimeout: b.config.WriteTimeout,
		IdleTimeout:  b.config.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("MCP SSE Bridge listening on port %d\n", b.config.Port)
		fmt.Printf("MCP Command: %s %s\n", cmdName, strings.Join(cmdArgs, " "))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	close(b.done)

	// Shutdown gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("HTTP server shutdown error: %v\n", err)
	}

	// Kill the MCP process
	if b.cmd.Process != nil {
		_ = b.cmd.Process.Kill() //nolint:errcheck
	}

	return nil
}

// readResponses reads from the MCP server's stdout and broadcasts to all SSE clients
func (b *Bridge) readResponses() {
	scanner := bufio.NewScanner(b.stdout)
	// MCP uses NDJSON, increase buffer for large responses
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Make a copy for broadcasting
		data := make([]byte, len(line))
		copy(data, line)

		// Broadcast to all SSE clients
		b.clients.Range(func(_ string, ch chan []byte) bool {
			select {
			case ch <- data:
			default:
				// Client channel full, skip
			}
			return true
		})
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading MCP stdout: %v\n", err)
	}
}

// readStderr reads from the MCP server's stderr for logging
func (b *Bridge) readStderr() {
	scanner := bufio.NewScanner(b.stderr)
	for scanner.Scan() {
		fmt.Printf("[MCP stderr] %s\n", scanner.Text())
	}
}

// handleHealth handles health check requests
func (b *Bridge) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Check if MCP process is still running
	if b.cmd == nil || b.cmd.Process == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"status": "unhealthy",
			"error":  "MCP process not running",
		})
		return
	}

	// Try to check process state (non-blocking)
	// Note: On Unix, this doesn't actually check if process is running
	// The process check is best-effort
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"pid":       b.cmd.Process.Pid,
	})
}

// handleSSE handles Server-Sent Events connections
func (b *Bridge) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client channel
	clientID := uuid.New().String()
	clientCh := make(chan []byte, 100)

	b.clients.Put(clientID, clientCh)

	// Cleanup on disconnect
	defer func() {
		b.clients.Delete(clientID)
		close(clientCh)
	}()

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Send initial connection event
	_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"clientId\":\"%s\"}\n\n", clientID)
	flusher.Flush()

	// Stream events
	for {
		select {
		case <-r.Context().Done():
			return
		case <-b.done:
			return
		case data := <-clientCh:
			// Send as SSE message event
			_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(data))
			flusher.Flush()
		}
	}
}

// handleMessage handles incoming MCP messages (JSON-RPC requests)
func (b *Bridge) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	// Validate it's valid JSON
	if !json.Valid(body) {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Write to MCP stdin (NDJSON format - newline delimited)
	b.stdinMu.Lock()
	_, err = fmt.Fprintf(b.stdin, "%s\n", body)
	b.stdinMu.Unlock()

	if err != nil {
		http.Error(w, "Failed to write to MCP server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"status": "accepted",
	})
}

// handleRoot handles the root endpoint with API info
func (b *Bridge) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"name":        "MCP SSE Bridge",
		"version":     "1.0.0",
		"description": "HTTP/SSE bridge for MCP (Model Context Protocol) servers",
		"endpoints": map[string]string{
			"GET /":         "This API info",
			"GET /health":   "Health check endpoint",
			"GET /sse":      "SSE endpoint for receiving MCP responses",
			"POST /message": "Send MCP JSON-RPC messages",
		},
	})
}
