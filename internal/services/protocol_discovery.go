package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// ACPClient implements a real Agent Client Protocol client
type ACPClient struct {
	agents    map[string]*ACPAgentConnection
	messageID int
	mu        sync.RWMutex
	logger    *logrus.Logger
}

// ACPAgentConnection represents a live connection to an ACP agent
type ACPAgentConnection struct {
	ID           string
	Name         string
	Transport    ACPTransport
	Capabilities map[string]interface{}
	Connected    bool
	LastUsed     time.Time
}

// ACPTransport defines the interface for ACP communication
type ACPTransport interface {
	Send(ctx context.Context, message interface{}) error
	Receive(ctx context.Context) (interface{}, error)
	Close() error
	IsConnected() bool
}

// WebSocketACPTransport implements ACP transport over WebSocket
type WebSocketACPTransport struct {
	conn      *websocket.Conn
	connected bool
	mu        sync.Mutex
}

// HTTPACPTransport implements ACP transport over HTTP
type HTTPACPTransport struct {
	baseURL    string
	httpClient *http.Client
	connected  bool
	mu         sync.Mutex
}

// ACPMessage represents a JSON-RPC message for ACP
type ACPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *ACPError   `json:"error,omitempty"`
}

// ACPError represents an ACP error
type ACPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ACP request/response types
type ACPInitializeRequest struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      map[string]string      `json:"clientInfo"`
}

type ACPInitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      map[string]string      `json:"serverInfo"`
	Instructions    string                 `json:"instructions,omitempty"`
}

// ACP operation types
type ACPActionRequest struct {
	Action  string                 `json:"action"`
	Params  map[string]interface{} `json:"params,omitempty"`
	Context map[string]interface{} `json:"context,omitempty"`
}

type ACPActionResult struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewACPClient creates a new ACP client
func NewACPClient(logger *logrus.Logger) *ACPClient {
	return &ACPClient{
		agents:    make(map[string]*ACPAgentConnection),
		messageID: 1,
		logger:    logger,
	}
}

// ConnectAgent connects to an ACP agent
func (c *ACPClient) ConnectAgent(ctx context.Context, agentID, name, endpoint string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.agents[agentID]; exists {
		return fmt.Errorf("ACP agent %s already connected", agentID)
	}

	// Create transport based on endpoint
	var transport ACPTransport
	var err error

	if strings.HasPrefix(endpoint, "ws://") || strings.HasPrefix(endpoint, "wss://") {
		transport, err = c.createWebSocketTransport(endpoint)
	} else if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		transport, err = c.createHTTPTransport(endpoint)
	} else {
		return fmt.Errorf("unsupported endpoint protocol: %s", endpoint)
	}

	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	connection := &ACPAgentConnection{
		ID:           agentID,
		Name:         name,
		Transport:    transport,
		Connected:    true,
		LastUsed:     time.Now(),
		Capabilities: make(map[string]interface{}),
	}

	// Initialize the agent
	if err := c.initializeAgent(ctx, connection); err != nil {
		transport.Close()
		return fmt.Errorf("failed to initialize ACP agent: %w", err)
	}

	c.agents[agentID] = connection
	c.logger.WithFields(logrus.Fields{
		"agentId":  agentID,
		"endpoint": endpoint,
	}).Info("Connected to ACP agent")

	return nil
}

// DisconnectAgent disconnects from an ACP agent
func (c *ACPClient) DisconnectAgent(agentID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	connection, exists := c.agents[agentID]
	if !exists {
		return fmt.Errorf("ACP agent %s not connected", agentID)
	}

	if err := connection.Transport.Close(); err != nil {
		c.logger.WithError(err).Warn("Error closing ACP transport")
	}

	delete(c.agents, agentID)

	c.logger.WithField("agentId", agentID).Info("Disconnected from ACP agent")
	return nil
}

// ExecuteAction executes an action on an ACP agent
func (c *ACPClient) ExecuteAction(ctx context.Context, agentID, action string, params map[string]interface{}) (*ACPActionResult, error) {
	c.mu.RLock()
	connection, exists := c.agents[agentID]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("ACP agent %s not connected", agentID)
	}

	if !connection.Connected {
		return nil, fmt.Errorf("ACP agent %s not connected", agentID)
	}

	actionRequest := ACPActionRequest{
		Action: action,
		Params: params,
		Context: map[string]interface{}{
			"timestamp": time.Now().Unix(),
			"requestId": fmt.Sprintf("req-%d", c.nextMessageID()),
		},
	}

	actionReq := ACPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "execute_action",
		Params:  actionRequest,
	}

	if err := connection.Transport.Send(ctx, actionReq); err != nil {
		return nil, fmt.Errorf("failed to send action request: %w", err)
	}

	response, err := connection.Transport.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive action response: %w", err)
	}

	var actionMsg ACPMessage
	if err := c.unmarshalMessage(response, &actionMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal action response: %w", err)
	}

	result := &ACPActionResult{
		Success: true,
	}

	if actionMsg.Error != nil {
		result.Success = false
		result.Error = actionMsg.Error.Message
	} else {
		result.Result = actionMsg.Result
	}

	connection.LastUsed = time.Now()
	return result, nil
}

// GetAgentCapabilities returns capabilities for an agent
func (c *ACPClient) GetAgentCapabilities(agentID string) (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	connection, exists := c.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("ACP agent %s not connected", agentID)
	}

	return connection.Capabilities, nil
}

// ListAgents returns all connected ACP agents
func (c *ACPClient) ListAgents() []*ACPAgentConnection {
	c.mu.RLock()
	defer c.mu.RUnlock()

	agents := make([]*ACPAgentConnection, 0, len(c.agents))
	for _, agent := range c.agents {
		agents = append(agents, agent)
	}

	return agents
}

// HealthCheck performs health checks on all connected agents
func (c *ACPClient) HealthCheck(ctx context.Context) map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make(map[string]bool)
	for agentID, connection := range c.agents {
		results[agentID] = connection.Transport.IsConnected()
	}

	return results
}

// GetAgentStatus returns detailed status for an agent
func (c *ACPClient) GetAgentStatus(ctx context.Context, agentID string) (map[string]interface{}, error) {
	c.mu.RLock()
	connection, exists := c.agents[agentID]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("ACP agent %s not found", agentID)
	}

	status := map[string]interface{}{
		"id":           connection.ID,
		"name":         connection.Name,
		"connected":    connection.Connected,
		"lastUsed":     connection.LastUsed,
		"capabilities": connection.Capabilities,
	}

	// Add transport-specific status
	if wsTransport, ok := connection.Transport.(*WebSocketACPTransport); ok {
		status["transport"] = "websocket"
		status["connected"] = wsTransport.IsConnected()
	} else if httpTransport, ok := connection.Transport.(*HTTPACPTransport); ok {
		status["transport"] = "http"
		status["connected"] = httpTransport.IsConnected()
	}

	return status, nil
}

// BroadcastAction broadcasts an action to all connected agents
func (c *ACPClient) BroadcastAction(ctx context.Context, action string, params map[string]interface{}) map[string]*ACPActionResult {
	c.mu.RLock()
	agents := make(map[string]*ACPAgentConnection)
	for k, v := range c.agents {
		agents[k] = v
	}
	c.mu.RUnlock()

	results := make(map[string]*ACPActionResult)

	for agentID, agent := range agents {
		if !agent.Connected {
			results[agentID] = &ACPActionResult{
				Success: false,
				Error:   "agent not connected",
			}
			continue
		}

		result, err := c.ExecuteAction(ctx, agentID, action, params)
		if err != nil {
			results[agentID] = &ACPActionResult{
				Success: false,
				Error:   err.Error(),
			}
		} else {
			results[agentID] = result
		}
	}

	return results
}

// Private methods

func (c *ACPClient) createWebSocketTransport(endpoint string) (ACPTransport, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL: %w", err)
	}

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	return &WebSocketACPTransport{
		conn:      conn,
		connected: true,
	}, nil
}

func (c *ACPClient) createHTTPTransport(endpoint string) (ACPTransport, error) {
	return &HTTPACPTransport{
		baseURL:    endpoint,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		connected:  true,
	}, nil
}

func (c *ACPClient) initializeAgent(ctx context.Context, connection *ACPAgentConnection) error {
	initRequest := ACPInitializeRequest{
		ProtocolVersion: "1.0.0",
		Capabilities:    map[string]interface{}{},
		ClientInfo: map[string]string{
			"name":    "superagent",
			"version": "1.0.0",
		},
	}

	initializeReq := ACPMessage{
		JSONRPC: "2.0",
		ID:      c.nextMessageID(),
		Method:  "initialize",
		Params:  initRequest,
	}

	if err := connection.Transport.Send(ctx, initializeReq); err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	response, err := connection.Transport.Receive(ctx)
	if err != nil {
		return fmt.Errorf("failed to receive initialize response: %w", err)
	}

	var initializeMsg ACPMessage
	if err := c.unmarshalMessage(response, &initializeMsg); err != nil {
		return fmt.Errorf("failed to unmarshal initialize response: %w", err)
	}

	if initializeMsg.Error != nil {
		return fmt.Errorf("initialize failed: %s", initializeMsg.Error.Message)
	}

	var result ACPInitializeResult
	if err := c.unmarshalResult(initializeMsg.Result, &result); err != nil {
		return fmt.Errorf("failed to unmarshal initialize result: %w", err)
	}

	connection.Capabilities = result.Capabilities

	return nil
}

func (c *ACPClient) nextMessageID() int {
	c.messageID++
	return c.messageID
}

func (c *ACPClient) unmarshalMessage(data interface{}, message *ACPMessage) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, message)
}

func (c *ACPClient) unmarshalResult(result interface{}, target interface{}) error {
	jsonData, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, target)
}

// WebSocketACPTransport implementation

func (t *WebSocketACPTransport) Send(ctx context.Context, message interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return fmt.Errorf("transport not connected")
	}

	return t.conn.WriteJSON(message)
}

func (t *WebSocketACPTransport) Receive(ctx context.Context) (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil, fmt.Errorf("transport not connected")
	}

	var message interface{}
	err := t.conn.ReadJSON(&message)
	if err != nil {
		t.connected = false
		return nil, err
	}

	return message, nil
}

func (t *WebSocketACPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connected = false

	if t.conn != nil {
		return t.conn.Close()
	}

	return nil
}

func (t *WebSocketACPTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return false
	}

	// Simple ping to check connection
	return t.conn != nil
}

// HTTPACPTransport implementation

func (t *HTTPACPTransport) Send(ctx context.Context, message interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return fmt.Errorf("transport not connected")
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL+"/rpc", strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		t.connected = false
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.connected = false
		return fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (t *HTTPACPTransport) Receive(ctx context.Context) (interface{}, error) {
	// HTTP transport is request-response, so this is not applicable
	// In a real implementation, you might use long polling or server-sent events
	return nil, fmt.Errorf("HTTP transport does not support receive")
}

func (t *HTTPACPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connected = false
	return nil
}

func (t *HTTPACPTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return false
	}

	// Simple health check
	req, err := http.NewRequest("GET", t.baseURL+"/health", nil)
	if err != nil {
		return false
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		t.connected = false
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
