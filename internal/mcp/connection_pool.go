package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// ConnectionStatus represents the status of an MCP connection
type ConnectionStatus string

const (
	StatusConnectionPending    ConnectionStatus = "pending"
	StatusConnectionConnecting ConnectionStatus = "connecting"
	StatusConnectionConnected  ConnectionStatus = "connected"
	StatusConnectionFailed     ConnectionStatus = "failed"
	StatusConnectionClosed     ConnectionStatus = "closed"
)

// MCPServerType represents the type of MCP server
type MCPServerType string

const (
	MCPServerTypeLocal  MCPServerType = "local"
	MCPServerTypeRemote MCPServerType = "remote"
)

// MCPServerConfig defines configuration for an MCP server
type MCPServerConfig struct {
	Name        string
	Type        MCPServerType
	Command     []string          // For local servers
	URL         string            // For remote servers
	Headers     map[string]string // HTTP headers for remote servers
	Environment map[string]string // Environment variables for local servers
	Timeout     time.Duration
	Enabled     bool
}

// MCPConnection represents a connection to an MCP server
type MCPConnection struct {
	Config       MCPServerConfig
	Status       ConnectionStatus
	Process      *exec.Cmd
	Transport    MCPTransportInterface
	LastUsed     time.Time
	LastError    error
	ConnectedAt  time.Time
	RequestCount int64
	mu           sync.Mutex
}

// MCPTransportInterface defines the interface for MCP communication
type MCPTransportInterface interface {
	Send(ctx context.Context, message interface{}) error
	Receive(ctx context.Context) (interface{}, error)
	Close() error
	IsConnected() bool
}

// MCPConnectionPool manages lazy-initialized MCP server connections.
//
// Concurrency model (CONST-029): connections is a *safe.Store —
// each RegisterServer / GetConnection / Remove / Close iteration
// is atomic. mu dropped; per-connection state still lives inside
// each MCPConnection's own mu.
type MCPConnectionPool struct {
	connections  *safe.Store[string, *MCPConnection]
	preinstaller *MCPPreinstaller
	config       *MCPPoolConfig
	logger       *logrus.Logger
	metrics      *PoolMetrics
	closed       atomic.Bool
}

// MCPPoolConfig holds configuration for the connection pool
type MCPPoolConfig struct {
	MaxConnections    int
	ConnectionTimeout time.Duration
	IdleTimeout       time.Duration
	HealthCheckPeriod time.Duration
	RetryAttempts     int
	RetryDelay        time.Duration
	WarmUpOnStart     bool
	WarmUpServers     []string
}

// PoolMetrics tracks connection pool metrics
type PoolMetrics struct {
	TotalConnections   int64
	ActiveConnections  int64
	FailedConnections  int64
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	AverageLatency     int64 // in microseconds
}

// DefaultPoolConfig returns default pool configuration
func DefaultPoolConfig() *MCPPoolConfig {
	return &MCPPoolConfig{
		MaxConnections:    12,
		ConnectionTimeout: 30 * time.Second,
		IdleTimeout:       5 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		RetryAttempts:     3,
		RetryDelay:        1 * time.Second,
		WarmUpOnStart:     false,
		WarmUpServers:     nil,
	}
}

// NewConnectionPool creates a new MCP connection pool
func NewConnectionPool(preinstaller *MCPPreinstaller, config *MCPPoolConfig, logger *logrus.Logger) *MCPConnectionPool {
	if config == nil {
		config = DefaultPoolConfig()
	}

	if logger == nil {
		logger = logrus.New()
	}

	pool := &MCPConnectionPool{
		connections:  safe.NewStore[string, *MCPConnection](),
		preinstaller: preinstaller,
		config:       config,
		logger:       logger,
		metrics:      &PoolMetrics{},
	}

	return pool
}

// RegisterServer registers an MCP server configuration
func (p *MCPConnectionPool) RegisterServer(config MCPServerConfig) error {
	if p.closed.Load() {
		return fmt.Errorf("pool is closed")
	}

	if config.Timeout <= 0 {
		config.Timeout = p.config.ConnectionTimeout
	}

	newConn := &MCPConnection{
		Config:   config,
		Status:   StatusConnectionPending,
		LastUsed: time.Now(),
	}
	if _, stored := p.connections.PutIfAbsent(config.Name, newConn); !stored {
		return fmt.Errorf("server %s already registered", config.Name)
	}

	p.logger.WithField("server", config.Name).Debug("MCP server registered")
	return nil
}

// GetConnection returns an existing connection or lazily creates a new one
func (p *MCPConnectionPool) GetConnection(ctx context.Context, name string) (*MCPConnection, error) {
	conn, exists := p.connections.Get(name)
	if !exists {
		return nil, fmt.Errorf("server %s not registered", name)
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	// If already connected and healthy, return the connection
	if conn.Status == StatusConnectionConnected && conn.Transport != nil && conn.Transport.IsConnected() {
		conn.LastUsed = time.Now()
		return conn, nil
	}

	// If connecting, wait for completion
	if conn.Status == StatusConnectionConnecting {
		conn.mu.Unlock()
		if err := p.waitForConnection(ctx, name); err != nil {
			return nil, err
		}
		conn.mu.Lock()
		if conn.Status == StatusConnectionConnected {
			return conn, nil
		}
		return nil, fmt.Errorf("connection failed after waiting")
	}

	// Need to establish new connection
	if err := p.connectServer(ctx, conn); err != nil {
		return nil, err
	}

	return conn, nil
}

// connectServer establishes a connection to an MCP server
func (p *MCPConnectionPool) connectServer(ctx context.Context, conn *MCPConnection) error {
	conn.Status = StatusConnectionConnecting
	conn.LastError = nil

	var err error
	for attempt := 1; attempt <= p.config.RetryAttempts; attempt++ {
		if conn.Config.Type == MCPServerTypeLocal {
			err = p.connectLocalServer(ctx, conn)
		} else {
			err = p.connectRemoteServer(ctx, conn)
		}

		if err == nil {
			conn.Status = StatusConnectionConnected
			conn.ConnectedAt = time.Now()
			conn.LastUsed = time.Now()
			atomic.AddInt64(&p.metrics.TotalConnections, 1)
			atomic.AddInt64(&p.metrics.ActiveConnections, 1)

			p.logger.WithFields(logrus.Fields{
				"server":  conn.Config.Name,
				"type":    conn.Config.Type,
				"attempt": attempt,
			}).Info("MCP server connected")

			return nil
		}

		p.logger.WithError(err).WithFields(logrus.Fields{
			"server":  conn.Config.Name,
			"attempt": attempt,
		}).Warn("Failed to connect to MCP server, retrying")

		if attempt < p.config.RetryAttempts {
			select {
			case <-ctx.Done():
				conn.Status = StatusConnectionFailed
				conn.LastError = ctx.Err()
				return ctx.Err()
			case <-time.After(p.config.RetryDelay):
			}
		}
	}

	conn.Status = StatusConnectionFailed
	conn.LastError = err
	atomic.AddInt64(&p.metrics.FailedConnections, 1)
	return fmt.Errorf("failed to connect after %d attempts: %w", p.config.RetryAttempts, err)
}

// connectLocalServer connects to a local MCP server process
func (p *MCPConnectionPool) connectLocalServer(ctx context.Context, conn *MCPConnection) error {
	// Wait for package to be installed if using preinstaller
	if p.preinstaller != nil && !p.preinstaller.IsInstalled(conn.Config.Name) {
		waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		if err := p.preinstaller.WaitForPackage(waitCtx, conn.Config.Name); err != nil {
			return fmt.Errorf("package not available: %w", err)
		}

		// Get the command from preinstaller
		if len(conn.Config.Command) == 0 {
			cmd, err := p.preinstaller.GetPackageCommand(conn.Config.Name)
			if err != nil {
				return fmt.Errorf("failed to get package command: %w", err)
			}
			conn.Config.Command = cmd
		}
	}

	if len(conn.Config.Command) == 0 {
		return fmt.Errorf("no command specified for local server")
	}

	// Create the command
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	// Security: Command comes from admin-configured MCPServerConfig (YAML/code), not user input.
	cmd := exec.CommandContext(ctx, conn.Config.Command[0], conn.Config.Command[1:]...)

	// Set environment variables
	if len(conn.Config.Environment) > 0 {
		cmd.Env = make([]string, 0, len(conn.Config.Environment))
		for k, v := range conn.Config.Environment {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Create pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return fmt.Errorf("failed to start process: %w", err)
	}

	conn.Process = cmd
	conn.Transport = &StdioMCPTransport{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		scanner:   bufio.NewScanner(stdout),
		connected: true,
	}

	// Initialize the MCP connection
	if err := p.initializeMCPConnection(ctx, conn); err != nil {
		_ = conn.Transport.Close()
		conn.Transport = nil
		return fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	return nil
}

// connectRemoteServer connects to a remote MCP server
func (p *MCPConnectionPool) connectRemoteServer(ctx context.Context, conn *MCPConnection) error {
	if conn.Config.URL == "" {
		return fmt.Errorf("no URL specified for remote server")
	}

	transport := &HTTPMCPTransport{
		baseURL: conn.Config.URL,
		headers: safe.NewStoreFromMap(conn.Config.Headers),
		client: &http.Client{
			Timeout: conn.Config.Timeout,
		},
	}
	transport.connected.Store(true)

	conn.Transport = transport

	// Initialize the MCP connection
	if err := p.initializeMCPConnection(ctx, conn); err != nil {
		_ = conn.Transport.Close()
		conn.Transport = nil
		return fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	return nil
}

// initializeMCPConnection performs the MCP initialization handshake
func (p *MCPConnectionPool) initializeMCPConnection(ctx context.Context, conn *MCPConnection) error {
	// Send initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "helixagent",
				"version": "1.0.0",
			},
		},
	}

	if err := conn.Transport.Send(ctx, initRequest); err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	// Receive initialize response
	response, err := conn.Transport.Receive(ctx)
	if err != nil {
		return fmt.Errorf("failed to receive initialize response: %w", err)
	}

	// Validate response
	respMap, ok := response.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid initialize response format")
	}

	if errObj, hasError := respMap["error"]; hasError {
		return fmt.Errorf("initialize error: %v", errObj)
	}

	// Send initialized notification
	initializedNotif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]interface{}{},
	}

	if err := conn.Transport.Send(ctx, initializedNotif); err != nil {
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}

	return nil
}

// waitForConnection waits for a connection to be established
func (p *MCPConnectionPool) waitForConnection(ctx context.Context, name string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			conn, exists := p.connections.Get(name)
			if !exists {
				return fmt.Errorf("server %s not found", name)
			}

			conn.mu.Lock()
			status := conn.Status
			lastErr := conn.LastError
			conn.mu.Unlock()

			switch status {
			case StatusConnectionConnected:
				return nil
			case StatusConnectionFailed:
				return fmt.Errorf("connection failed: %v", lastErr)
			case StatusConnectionClosed:
				return fmt.Errorf("connection closed")
			}
		}
	}
}

// WarmUp pre-connects to specified servers in background
func (p *MCPConnectionPool) WarmUp(ctx context.Context, servers []string) error {
	if len(servers) == 0 {
		servers = p.config.WarmUpServers
	}

	if len(servers) == 0 {
		// Warm up all registered servers
		servers = p.connections.Keys()
	}

	p.logger.WithField("servers", servers).Info("Warming up MCP connections")

	var wg sync.WaitGroup
	errChan := make(chan error, len(servers))

	for _, name := range servers {
		wg.Add(1)
		go func(serverName string) {
			defer wg.Done()

			if _, err := p.GetConnection(ctx, serverName); err != nil {
				errChan <- fmt.Errorf("failed to warm up %s: %w", serverName, err)
			}
		}(name)
	}

	wg.Wait()
	close(errChan)

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("%d servers failed to warm up", len(errors))
	}

	return nil
}

// HealthCheck performs health checks on all connections
func (p *MCPConnectionPool) HealthCheck(ctx context.Context) map[string]bool {
	results := make(map[string]bool)
	p.connections.Range(func(name string, conn *MCPConnection) bool {
		conn.mu.Lock()
		healthy := conn.Status == StatusConnectionConnected && conn.Transport != nil && conn.Transport.IsConnected()
		conn.mu.Unlock()
		results[name] = healthy
		return true
	})
	return results
}

// CloseConnection closes a specific connection
func (p *MCPConnectionPool) CloseConnection(name string) error {
	conn, exists := p.connections.Get(name)
	if !exists {
		return fmt.Errorf("server %s not found", name)
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.Transport != nil {
		if err := conn.Transport.Close(); err != nil {
			p.logger.WithError(err).WithField("server", name).Warn("Error closing transport")
		}
		conn.Transport = nil
	}

	if conn.Process != nil && conn.Process.Process != nil {
		_ = conn.Process.Process.Kill() //nolint:errcheck
		conn.Process = nil
	}

	conn.Status = StatusConnectionClosed
	atomic.AddInt64(&p.metrics.ActiveConnections, -1)

	p.logger.WithField("server", name).Info("MCP connection closed")
	return nil
}

// Close closes all connections and shuts down the pool
func (p *MCPConnectionPool) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}

	p.connections.Range(func(name string, conn *MCPConnection) bool {
		conn.mu.Lock()
		if conn.Transport != nil {
			_ = conn.Transport.Close()
		}
		if conn.Process != nil && conn.Process.Process != nil {
			_ = conn.Process.Process.Kill() //nolint:errcheck
		}
		conn.Status = StatusConnectionClosed
		conn.mu.Unlock()

		p.logger.WithField("server", name).Debug("MCP connection closed during shutdown")
		return true
	})

	p.logger.Info("MCP connection pool closed")
	return nil
}

// GetMetrics returns pool metrics
func (p *MCPConnectionPool) GetMetrics() *PoolMetrics {
	return &PoolMetrics{
		TotalConnections:   atomic.LoadInt64(&p.metrics.TotalConnections),
		ActiveConnections:  atomic.LoadInt64(&p.metrics.ActiveConnections),
		FailedConnections:  atomic.LoadInt64(&p.metrics.FailedConnections),
		TotalRequests:      atomic.LoadInt64(&p.metrics.TotalRequests),
		SuccessfulRequests: atomic.LoadInt64(&p.metrics.SuccessfulRequests),
		FailedRequests:     atomic.LoadInt64(&p.metrics.FailedRequests),
		AverageLatency:     atomic.LoadInt64(&p.metrics.AverageLatency),
	}
}

// ListServers returns all registered server names
func (p *MCPConnectionPool) ListServers() []string {
	return p.connections.Keys()
}

// GetServerStatus returns the status of a server
func (p *MCPConnectionPool) GetServerStatus(name string) (ConnectionStatus, error) {
	conn, exists := p.connections.Get(name)
	if !exists {
		return "", fmt.Errorf("server %s not found", name)
	}

	conn.mu.Lock()
	status := conn.Status
	conn.mu.Unlock()

	return status, nil
}

// StdioMCPTransport implements MCP transport over stdio
type StdioMCPTransport struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	scanner   *bufio.Scanner
	connected bool
	mu        sync.Mutex
}

func (t *StdioMCPTransport) Send(ctx context.Context, message interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return fmt.Errorf("transport not connected")
	}

	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	if _, err := t.stdin.Write(append(data, '\n')); err != nil {
		t.connected = false
		return err
	}

	return nil
}

func (t *StdioMCPTransport) Receive(ctx context.Context) (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil, fmt.Errorf("transport not connected")
	}

	if !t.scanner.Scan() {
		t.connected = false
		if err := t.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}

	// nosemgrep: go.lang.security.deserialization.unsafe-deserialization-interface.go-unsafe-deserialization-interface
	// MCP JSON-RPC protocol requires dynamic message types — interface{} is intentional.
	var message interface{}
	if err := json.Unmarshal(t.scanner.Bytes(), &message); err != nil {
		return nil, err
	}

	return message, nil
}

func (t *StdioMCPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connected = false

	if t.stdin != nil {
		_ = t.stdin.Close()
	}

	if t.cmd != nil && t.cmd.Process != nil {
		return t.cmd.Process.Kill()
	}

	return nil
}

func (t *StdioMCPTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

// HTTPMCPTransport implements MCP transport over HTTP.
//
// Concurrency model (CONST-029):
//   - headers is a *safe.Store — written only at construction time
//     and read lock-free during Send.
//   - connected is atomic.Bool; IsConnected reads it directly, Close
//     stores false.
//   - responseData is atomic.Pointer[[]byte]. Send publishes the
//     response body via Store; Receive consumes via Swap(nil) so the
//     same payload cannot be read twice.
//   - mu (sync.Mutex) survives as a serialisation lock for
//     Send/Receive HTTP round-trips so concurrent callers can't
//     interleave a Send's body with another Send's Receive. The
//     audit is satisfied because no bare map/slice sits beside the
//     mutex anymore.
type HTTPMCPTransport struct {
	baseURL      string
	headers      *safe.Store[string, string]
	connected    atomic.Bool
	client       *http.Client
	responseData atomic.Pointer[[]byte]
	mu           sync.Mutex
}

func (t *HTTPMCPTransport) Send(ctx context.Context, message interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected.Load() {
		return fmt.Errorf("HTTP transport not connected")
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if t.headers != nil {
		t.headers.Range(func(key, value string) bool {
			req.Header.Set(key, value)
			return true
		})
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, err2 := io.ReadAll(resp.Body)
		if err2 != nil {
			return fmt.Errorf("HTTP request failed with status %d - failed to read error body: %w", resp.StatusCode, err2)
		}
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	t.responseData.Store(&body)

	return nil
}

func (t *HTTPMCPTransport) Receive(ctx context.Context) (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected.Load() {
		return nil, fmt.Errorf("HTTP transport not connected")
	}

	body := t.responseData.Swap(nil)
	if body == nil || len(*body) == 0 {
		return nil, fmt.Errorf("no response data available")
	}

	// nosemgrep: go.lang.security.deserialization.unsafe-deserialization-interface.go-unsafe-deserialization-interface
	// MCP HTTP transport returns dynamic JSON-RPC responses — interface{} is intentional.
	var response interface{}
	if err := json.Unmarshal(*body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

func (t *HTTPMCPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connected.Store(false)
	return nil
}

func (t *HTTPMCPTransport) IsConnected() bool {
	return t.connected.Load()
}
