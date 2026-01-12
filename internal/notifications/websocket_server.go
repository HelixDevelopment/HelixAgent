package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// WebSocketClientInterface defines the interface for WebSocket clients
type WebSocketClientInterface interface {
	Send(data []byte) error
	Close() error
	ID() string
}

// WebSocketServer manages WebSocket connections
type WebSocketServer struct {
	// Task-specific clients
	clients     map[string]map[string]WebSocketClientInterface
	clientsMu   sync.RWMutex

	// Global clients
	globalClients   map[string]WebSocketClientInterface
	globalClientsMu sync.RWMutex

	// Configuration
	config   *WebSocketConfig
	upgrader websocket.Upgrader

	logger *logrus.Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	ReadBufferSize  int           `yaml:"read_buffer_size"`
	WriteBufferSize int           `yaml:"write_buffer_size"`
	PingInterval    time.Duration `yaml:"ping_interval"`
	PongWait        time.Duration `yaml:"pong_wait"`
	WriteWait       time.Duration `yaml:"write_wait"`
	MaxMessageSize  int64         `yaml:"max_message_size"`
	AllowedOrigins  []string      `yaml:"allowed_origins"`
}

// DefaultWebSocketConfig returns default WebSocket configuration
func DefaultWebSocketConfig() *WebSocketConfig {
	return &WebSocketConfig{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		PingInterval:    54 * time.Second,
		PongWait:        60 * time.Second,
		WriteWait:       10 * time.Second,
		MaxMessageSize:  512 * 1024, // 512KB
		AllowedOrigins:  []string{"*"},
	}
}

// NewWebSocketServer creates a new WebSocket server
func NewWebSocketServer(config *WebSocketConfig, logger *logrus.Logger) *WebSocketServer {
	if config == nil {
		config = DefaultWebSocketConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	server := &WebSocketServer{
		clients:       make(map[string]map[string]WebSocketClientInterface),
		globalClients: make(map[string]WebSocketClientInterface),
		config:        config,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  config.ReadBufferSize,
			WriteBufferSize: config.WriteBufferSize,
			CheckOrigin: func(r *http.Request) bool {
				if len(config.AllowedOrigins) == 0 {
					return true
				}
				origin := r.Header.Get("Origin")
				for _, allowed := range config.AllowedOrigins {
					if allowed == "*" || allowed == origin {
						return true
					}
				}
				return false
			},
		},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	return server
}

// Start starts the WebSocket server
func (s *WebSocketServer) Start() error {
	s.logger.Info("WebSocket server started")
	return nil
}

// Stop stops the WebSocket server
func (s *WebSocketServer) Stop() error {
	s.logger.Info("Stopping WebSocket server")
	s.cancel()
	s.wg.Wait()

	// Close all clients
	s.clientsMu.Lock()
	for taskID, clients := range s.clients {
		for _, client := range clients {
			client.Close()
		}
		delete(s.clients, taskID)
	}
	s.clientsMu.Unlock()

	s.globalClientsMu.Lock()
	for _, client := range s.globalClients {
		client.Close()
	}
	s.globalClients = make(map[string]WebSocketClientInterface)
	s.globalClientsMu.Unlock()

	return nil
}

// HandleConnection handles a WebSocket connection upgrade
func (s *WebSocketServer) HandleConnection(c *gin.Context) {
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.logger.WithError(err).Error("Failed to upgrade WebSocket connection")
		return
	}

	taskID := c.Param("id")
	clientID := uuid.New().String()

	client := NewWebSocketClient(clientID, conn, s.config, s.logger)

	if taskID != "" {
		s.RegisterClient(taskID, client)
		defer s.UnregisterClient(taskID, clientID)
	} else {
		s.RegisterGlobalClient(client)
		defer s.UnregisterGlobalClient(clientID)
	}

	// Start reading messages
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.readLoop(client, taskID)
	}()

	// Start ping loop
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.pingLoop(client)
	}()
}

// RegisterClient registers a client for a specific task
func (s *WebSocketServer) RegisterClient(taskID string, client WebSocketClientInterface) error {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	if s.clients[taskID] == nil {
		s.clients[taskID] = make(map[string]WebSocketClientInterface)
	}
	s.clients[taskID][client.ID()] = client

	s.logger.WithFields(logrus.Fields{
		"task_id":   taskID,
		"client_id": client.ID(),
	}).Debug("WebSocket client registered")

	return nil
}

// UnregisterClient removes a client from a task
func (s *WebSocketServer) UnregisterClient(taskID, clientID string) error {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	if clients, exists := s.clients[taskID]; exists {
		if client, ok := clients[clientID]; ok {
			client.Close()
			delete(clients, clientID)
		}
		if len(clients) == 0 {
			delete(s.clients, taskID)
		}
	}

	return nil
}

// RegisterGlobalClient registers a global client
func (s *WebSocketServer) RegisterGlobalClient(client WebSocketClientInterface) error {
	s.globalClientsMu.Lock()
	defer s.globalClientsMu.Unlock()

	s.globalClients[client.ID()] = client

	s.logger.WithField("client_id", client.ID()).Debug("Global WebSocket client registered")
	return nil
}

// UnregisterGlobalClient removes a global client
func (s *WebSocketServer) UnregisterGlobalClient(clientID string) error {
	s.globalClientsMu.Lock()
	defer s.globalClientsMu.Unlock()

	if client, exists := s.globalClients[clientID]; exists {
		client.Close()
		delete(s.globalClients, clientID)
	}

	return nil
}

// Broadcast sends a message to all clients watching a task
func (s *WebSocketServer) Broadcast(taskID string, data []byte) {
	s.clientsMu.RLock()
	clients := s.clients[taskID]
	s.clientsMu.RUnlock()

	for _, client := range clients {
		if err := client.Send(data); err != nil {
			s.logger.WithError(err).WithField("client_id", client.ID()).Debug("Failed to send to WebSocket client")
		}
	}

	// Also send to global clients
	s.broadcastGlobal(data)
}

// BroadcastAll sends a message to all connected clients
func (s *WebSocketServer) BroadcastAll(data []byte) {
	s.clientsMu.RLock()
	for _, clients := range s.clients {
		for _, client := range clients {
			if err := client.Send(data); err != nil {
				s.logger.WithError(err).Debug("Failed to send to WebSocket client")
			}
		}
	}
	s.clientsMu.RUnlock()

	s.broadcastGlobal(data)
}

// broadcastGlobal sends data to all global clients
func (s *WebSocketServer) broadcastGlobal(data []byte) {
	s.globalClientsMu.RLock()
	defer s.globalClientsMu.RUnlock()

	for _, client := range s.globalClients {
		if err := client.Send(data); err != nil {
			s.logger.WithError(err).Debug("Failed to send to global WebSocket client")
		}
	}
}

// readLoop reads messages from a WebSocket client
func (s *WebSocketServer) readLoop(client *WebSocketClient, taskID string) {
	conn := client.conn
	conn.SetReadLimit(s.config.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(s.config.PongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(s.config.PongWait))
		return nil
	})

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					s.logger.WithError(err).Debug("WebSocket read error")
				}
				return
			}

			s.handleMessage(client, taskID, message)
		}
	}
}

// handleMessage handles incoming WebSocket messages
func (s *WebSocketServer) handleMessage(client *WebSocketClient, taskID string, message []byte) {
	var msg WebSocketMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		s.logger.WithError(err).Debug("Failed to parse WebSocket message")
		return
	}

	switch msg.Type {
	case "subscribe":
		if msg.TaskID != "" {
			s.RegisterClient(msg.TaskID, client)
		}
	case "unsubscribe":
		if msg.TaskID != "" {
			s.UnregisterClient(msg.TaskID, client.ID())
		}
	case "ping":
		client.Send([]byte(`{"type":"pong"}`))
	}
}

// pingLoop sends periodic pings to keep the connection alive
func (s *WebSocketServer) pingLoop(client *WebSocketClient) {
	ticker := time.NewTicker(s.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			client.mu.Lock()
			client.conn.SetWriteDeadline(time.Now().Add(s.config.WriteWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				client.mu.Unlock()
				return
			}
			client.mu.Unlock()
		}
	}
}

// GetClientCount returns the number of clients for a task
func (s *WebSocketServer) GetClientCount(taskID string) int {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	return len(s.clients[taskID])
}

// GetTotalClientCount returns the total number of connected clients
func (s *WebSocketServer) GetTotalClientCount() int {
	s.clientsMu.RLock()
	taskCount := 0
	for _, clients := range s.clients {
		taskCount += len(clients)
	}
	s.clientsMu.RUnlock()

	s.globalClientsMu.RLock()
	globalCount := len(s.globalClients)
	s.globalClientsMu.RUnlock()

	return taskCount + globalCount
}

// WebSocketMessage represents an incoming WebSocket message
type WebSocketMessage struct {
	Type   string `json:"type"`
	TaskID string `json:"task_id,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

// WebSocketClient represents a WebSocket client connection
type WebSocketClient struct {
	id     string
	conn   *websocket.Conn
	config *WebSocketConfig
	logger *logrus.Logger
	mu     sync.Mutex
	closed bool
}

// NewWebSocketClient creates a new WebSocket client
func NewWebSocketClient(id string, conn *websocket.Conn, config *WebSocketConfig, logger *logrus.Logger) *WebSocketClient {
	return &WebSocketClient{
		id:     id,
		conn:   conn,
		config: config,
		logger: logger,
	}
}

func (c *WebSocketClient) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *WebSocketClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.conn.Close()
}

func (c *WebSocketClient) ID() string {
	return c.id
}

// WebSocketSubscriber implements the Subscriber interface for WebSocket
type WebSocketSubscriber struct {
	id     string
	taskID string
	client WebSocketClientInterface
	active bool
	mu     sync.RWMutex
}

// NewWebSocketSubscriber creates a new WebSocket subscriber
func NewWebSocketSubscriber(id, taskID string, client WebSocketClientInterface) *WebSocketSubscriber {
	return &WebSocketSubscriber{
		id:     id,
		taskID: taskID,
		client: client,
		active: true,
	}
}

func (s *WebSocketSubscriber) Notify(ctx context.Context, notification *TaskNotification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	return s.client.Send(data)
}

func (s *WebSocketSubscriber) Type() NotificationType {
	return NotificationTypeWebSocket
}

func (s *WebSocketSubscriber) ID() string {
	return s.id
}

func (s *WebSocketSubscriber) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

func (s *WebSocketSubscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
	return s.client.Close()
}
