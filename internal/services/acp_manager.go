package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/superagent/superagent/internal/config"
	"github.com/superagent/superagent/internal/database"
)

// ACPManager handles ACP (Agent Client Protocol) operations
type ACPManager struct {
	repo       *database.ModelMetadataRepository
	cache      CacheInterface
	log        *logrus.Logger
	config     *config.ACPConfig
	client     *ACPClient
	servers    map[string]*ACPServer
	serversMu  sync.RWMutex
	httpClient *http.Client
}

// ACPServer represents an ACP server configuration
type ACPServer struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	URL          string          `json:"url"`
	Enabled      bool            `json:"enabled"`
	Version      string          `json:"version"`
	Capabilities []ACPCapability `json:"capabilities"`
	LastSync     *time.Time      `json:"lastSync"`
}

// ACPCapability represents an ACP server capability
type ACPCapability struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ACPRequest represents a request to an ACP server
type ACPRequest struct {
	ServerID   string                 `json:"serverId"`
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
}

// ACPResponse represents a response from an ACP server
type ACPResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// ACPClient handles HTTP and WebSocket communication with ACP servers
type ACPClient struct {
	httpClient   *http.Client
	wsDialer     *websocket.Dialer
	wsConns      map[string]*websocket.Conn
	wsConnsMu    sync.RWMutex
	timeout      time.Duration
	maxRetries   int
	log          *logrus.Logger
}

// ACPProtocolRequest represents the ACP protocol request format
type ACPProtocolRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// ACPProtocolResponse represents the ACP protocol response format
type ACPProtocolResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id,omitempty"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *ACPRPCError  `json:"error,omitempty"`
}

// ACPRPCError represents an ACP RPC error
type ACPRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ACPServerInfo represents server info returned from sync
type ACPServerInfo struct {
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	Capabilities []ACPCapability `json:"capabilities"`
}

// NewACPClient creates a new ACP client for protocol communication
func NewACPClient(timeout time.Duration, maxRetries int, log *logrus.Logger) *ACPClient {
	return &ACPClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		wsDialer: &websocket.Dialer{
			HandshakeTimeout: timeout,
		},
		wsConns:    make(map[string]*websocket.Conn),
		timeout:    timeout,
		maxRetries: maxRetries,
		log:        log,
	}
}

// ExecuteHTTP executes an ACP action via HTTP
func (c *ACPClient) ExecuteHTTP(ctx context.Context, serverURL string, req ACPProtocolRequest) (*ACPProtocolResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", serverURL, bytes.NewReader(data))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("failed to send request: %w", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
			continue
		}

		var acpResp ACPProtocolResponse
		if err := json.Unmarshal(body, &acpResp); err != nil {
			lastErr = fmt.Errorf("failed to unmarshal response: %w", err)
			continue
		}

		return &acpResp, nil
	}

	return nil, lastErr
}

// ExecuteWS executes an ACP action via WebSocket
func (c *ACPClient) ExecuteWS(ctx context.Context, serverURL string, req ACPProtocolRequest) (*ACPProtocolResponse, error) {
	c.wsConnsMu.Lock()
	conn, exists := c.wsConns[serverURL]
	if !exists || conn == nil {
		// Establish new connection
		var err error
		conn, _, err = c.wsDialer.DialContext(ctx, serverURL, nil)
		if err != nil {
			c.wsConnsMu.Unlock()
			return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
		}
		c.wsConns[serverURL] = conn
	}
	c.wsConnsMu.Unlock()

	// Send request
	if err := conn.WriteJSON(req); err != nil {
		c.closeWSConn(serverURL)
		return nil, fmt.Errorf("failed to send WebSocket message: %w", err)
	}

	// Set read deadline
	if err := conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Read response
	var resp ACPProtocolResponse
	if err := conn.ReadJSON(&resp); err != nil {
		c.closeWSConn(serverURL)
		return nil, fmt.Errorf("failed to read WebSocket response: %w", err)
	}

	return &resp, nil
}

// GetServerInfo fetches server info via HTTP GET
func (c *ACPClient) GetServerInfo(ctx context.Context, serverURL string) (*ACPServerInfo, error) {
	// Construct info endpoint URL
	infoURL := serverURL
	if !strings.HasSuffix(infoURL, "/info") {
		infoURL = strings.TrimSuffix(infoURL, "/") + "/info"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", infoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var info ACPServerInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &info, nil
}

// CloseAll closes all WebSocket connections
func (c *ACPClient) CloseAll() {
	c.wsConnsMu.Lock()
	defer c.wsConnsMu.Unlock()

	for url, conn := range c.wsConns {
		if conn != nil {
			conn.Close()
		}
		delete(c.wsConns, url)
	}
}

func (c *ACPClient) closeWSConn(serverURL string) {
	c.wsConnsMu.Lock()
	defer c.wsConnsMu.Unlock()

	if conn, exists := c.wsConns[serverURL]; exists && conn != nil {
		conn.Close()
		delete(c.wsConns, serverURL)
	}
}

// NewACPManager creates a new ACP manager
func NewACPManager(repo *database.ModelMetadataRepository, cache CacheInterface, log *logrus.Logger) *ACPManager {
	return NewACPManagerWithConfig(repo, cache, log, nil)
}

// NewACPManagerWithConfig creates a new ACP manager with configuration
func NewACPManagerWithConfig(repo *database.ModelMetadataRepository, cache CacheInterface, log *logrus.Logger, cfg *config.ACPConfig) *ACPManager {
	timeout := 30 * time.Second
	maxRetries := 3

	if cfg != nil {
		if cfg.DefaultTimeout > 0 {
			timeout = cfg.DefaultTimeout
		}
		if cfg.MaxRetries > 0 {
			maxRetries = cfg.MaxRetries
		}
	}

	m := &ACPManager{
		repo:    repo,
		cache:   cache,
		log:     log,
		config:  cfg,
		client:  NewACPClient(timeout, maxRetries, log),
		servers: make(map[string]*ACPServer),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}

	// Load servers from config
	if cfg != nil {
		for _, serverCfg := range cfg.Servers {
			m.servers[serverCfg.ID] = &ACPServer{
				ID:      serverCfg.ID,
				Name:    serverCfg.Name,
				URL:     serverCfg.URL,
				Enabled: serverCfg.Enabled,
			}
		}
	}

	return m
}

// RegisterServer registers an ACP server dynamically
func (m *ACPManager) RegisterServer(server *ACPServer) error {
	if server.ID == "" {
		return fmt.Errorf("server ID is required")
	}
	if server.URL == "" {
		return fmt.Errorf("server URL is required")
	}

	m.serversMu.Lock()
	defer m.serversMu.Unlock()

	m.servers[server.ID] = server
	m.log.WithFields(logrus.Fields{
		"serverId": server.ID,
		"name":     server.Name,
		"url":      server.URL,
	}).Info("Registered ACP server")

	return nil
}

// UnregisterServer removes an ACP server
func (m *ACPManager) UnregisterServer(serverID string) error {
	m.serversMu.Lock()
	defer m.serversMu.Unlock()

	if _, exists := m.servers[serverID]; !exists {
		return fmt.Errorf("ACP server %s not found", serverID)
	}

	delete(m.servers, serverID)
	m.log.WithField("serverId", serverID).Info("Unregistered ACP server")

	return nil
}

// ListACPServers lists all configured ACP servers
func (m *ACPManager) ListACPServers(ctx context.Context) ([]*ACPServer, error) {
	m.serversMu.RLock()
	defer m.serversMu.RUnlock()

	// If no servers are registered, return empty list
	if len(m.servers) == 0 {
		m.log.Info("No ACP servers configured")
		return []*ACPServer{}, nil
	}

	servers := make([]*ACPServer, 0, len(m.servers))
	for _, server := range m.servers {
		servers = append(servers, server)
	}

	m.log.WithField("count", len(servers)).Info("Listed ACP servers")
	return servers, nil
}

// GetACPServer gets a specific ACP server by ID
func (m *ACPManager) GetACPServer(ctx context.Context, serverID string) (*ACPServer, error) {
	m.serversMu.RLock()
	defer m.serversMu.RUnlock()

	server, exists := m.servers[serverID]
	if !exists {
		return nil, fmt.Errorf("ACP server %s not found", serverID)
	}

	return server, nil
}

// ExecuteACPAction executes an action on an ACP server
func (m *ACPManager) ExecuteACPAction(ctx context.Context, req ACPRequest) (*ACPResponse, error) {
	m.log.WithFields(logrus.Fields{
		"serverId": req.ServerID,
		"action":   req.Action,
	}).Info("Executing ACP action")

	// Validate server exists
	server, err := m.GetACPServer(ctx, req.ServerID)
	if err != nil {
		return nil, fmt.Errorf("invalid server ID: %w", err)
	}

	if !server.Enabled {
		return nil, fmt.Errorf("server %s is not enabled", req.ServerID)
	}

	// Build the ACP protocol request
	protocolReq := ACPProtocolRequest{
		JSONRPC: "2.0",
		ID:      time.Now().UnixNano(),
		Method:  req.Action,
		Params:  req.Parameters,
	}

	// Determine transport type based on URL scheme
	var protocolResp *ACPProtocolResponse
	if m.isWebSocketURL(server.URL) {
		protocolResp, err = m.client.ExecuteWS(ctx, server.URL, protocolReq)
	} else {
		protocolResp, err = m.client.ExecuteHTTP(ctx, server.URL, protocolReq)
	}

	if err != nil {
		m.log.WithError(err).WithField("serverId", req.ServerID).Error("ACP action execution failed")
		return &ACPResponse{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}, nil
	}

	// Convert protocol response to ACPResponse
	response := &ACPResponse{
		Timestamp: time.Now(),
	}

	if protocolResp.Error != nil {
		response.Success = false
		response.Error = protocolResp.Error.Message
	} else {
		response.Success = true
		response.Data = protocolResp.Result
	}

	m.log.WithField("timestamp", response.Timestamp).Info("ACP action execution completed")
	return response, nil
}

// ValidateACPRequest validates an ACP action request
func (m *ACPManager) ValidateACPRequest(ctx context.Context, req ACPRequest) error {
	if req.ServerID == "" {
		return fmt.Errorf("server ID is required")
	}

	if req.Action == "" {
		return fmt.Errorf("action is required")
	}

	// Check if server exists
	server, err := m.GetACPServer(ctx, req.ServerID)
	if err != nil {
		return fmt.Errorf("invalid server ID: %w", err)
	}

	if !server.Enabled {
		return fmt.Errorf("server %s is not enabled", req.ServerID)
	}

	return nil
}

// SyncACPServer synchronizes configuration with an ACP server
func (m *ACPManager) SyncACPServer(ctx context.Context, serverID string) error {
	m.log.WithField("serverId", serverID).Info("Synchronizing ACP server")

	server, err := m.GetACPServer(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// For WebSocket URLs, convert to HTTP for info endpoint
	infoURL := m.getHTTPURL(server.URL)

	// Fetch server info
	info, err := m.client.GetServerInfo(ctx, infoURL)
	if err != nil {
		m.log.WithError(err).WithField("serverId", serverID).Warn("Failed to sync ACP server")
		return fmt.Errorf("failed to fetch server info: %w", err)
	}

	// Update server with fetched info
	m.serversMu.Lock()
	defer m.serversMu.Unlock()

	if s, exists := m.servers[serverID]; exists {
		s.Version = info.Version
		s.Capabilities = info.Capabilities
		now := time.Now()
		s.LastSync = &now

		if info.Name != "" {
			s.Name = info.Name
		}
	}

	m.log.WithFields(logrus.Fields{
		"serverId": serverID,
		"version":  info.Version,
		"capabilities": len(info.Capabilities),
	}).Info("ACP server synchronization completed")

	return nil
}

// GetACPStats returns statistics about ACP usage
func (m *ACPManager) GetACPStats(ctx context.Context) (map[string]interface{}, error) {
	servers, err := m.ListACPServers(ctx)
	if err != nil {
		return nil, err
	}

	enabledCount := 0
	totalCapabilities := 0

	for _, server := range servers {
		if server.Enabled {
			enabledCount++
			totalCapabilities += len(server.Capabilities)
		}
	}

	stats := map[string]interface{}{
		"totalServers":      len(servers),
		"enabledServers":    enabledCount,
		"totalCapabilities": totalCapabilities,
		"lastSync":          time.Now(),
	}

	m.log.WithFields(stats).Info("ACP statistics retrieved")
	return stats, nil
}

// Close cleans up resources
func (m *ACPManager) Close() error {
	if m.client != nil {
		m.client.CloseAll()
	}
	return nil
}

// isWebSocketURL checks if the URL is a WebSocket URL
func (m *ACPManager) isWebSocketURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Scheme == "ws" || u.Scheme == "wss"
}

// getHTTPURL converts a WebSocket URL to HTTP URL for info endpoint
func (m *ACPManager) getHTTPURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	}

	return u.String()
}
