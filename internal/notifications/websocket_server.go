package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

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

// WebSocketServer manages WebSocket connections.
//
// Concurrency model (CONST-029): task-specific clients →
// *safe.Store[taskID, map[clientID]Client] with Update-based COW for
// the inner per-task client map; globalClients → *safe.Store. Both
// mutexes dropped entirely.
type WebSocketServer struct {
	// Task-specific clients (taskID → clientID → Client)
	clients *safe.Store[string, map[string]WebSocketClientInterface]

	// Global clients (clientID → Client)
	globalClients *safe.Store[string, WebSocketClientInterface]

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
		clients:       safe.NewStore[string, map[string]WebSocketClientInterface](),
		globalClients: safe.NewStore[string, WebSocketClientInterface](),
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
	s.clients.Range(func(_ string, clients map[string]WebSocketClientInterface) bool {
		for _, client := range clients {
			_ = client.Close()
		}
		return true
	})
	s.clients.Clear()

	s.globalClients.Range(func(_ string, client WebSocketClientInterface) bool {
		_ = client.Close()
		return true
	})
	s.globalClients.Clear()

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
		if err := s.RegisterClient(taskID, client); err != nil {
			s.logger.WithError(err).Debug("Failed to register client")
		}
		defer func() { _ = s.UnregisterClient(taskID, clientID) }() //nolint:errcheck
	} else {
		if err := s.RegisterGlobalClient(client); err != nil {
			s.logger.WithError(err).Debug("Failed to register global client")
		}
		defer func() { _ = s.UnregisterGlobalClient(clientID) }() //nolint:errcheck
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
	s.clients.Update(taskID, func(cur map[string]WebSocketClientInterface, _ bool) (map[string]WebSocketClientInterface, bool) {
		next := make(map[string]WebSocketClientInterface, len(cur)+1)
		for k, v := range cur {
			next[k] = v
		}
		next[client.ID()] = client
		return next, true
	})

	s.logger.WithFields(logrus.Fields{
		"task_id":   taskID,
		"client_id": client.ID(),
	}).Debug("WebSocket client registered")

	return nil
}

// UnregisterClient removes a client from a task
func (s *WebSocketServer) UnregisterClient(taskID, clientID string) error {
	var closed WebSocketClientInterface
	s.clients.Update(taskID, func(cur map[string]WebSocketClientInterface, present bool) (map[string]WebSocketClientInterface, bool) {
		if !present {
			return cur, false
		}
		client, ok := cur[clientID]
		if !ok {
			return cur, true
		}
		closed = client
		if len(cur) == 1 {
			return nil, false
		}
		next := make(map[string]WebSocketClientInterface, len(cur)-1)
		for k, v := range cur {
			if k != clientID {
				next[k] = v
			}
		}
		return next, true
	})
	if closed != nil {
		_ = closed.Close()
	}
	return nil
}

// RegisterGlobalClient registers a global client
func (s *WebSocketServer) RegisterGlobalClient(client WebSocketClientInterface) error {
	s.globalClients.Put(client.ID(), client)
	s.logger.WithField("client_id", client.ID()).Debug("Global WebSocket client registered")
	return nil
}

// UnregisterGlobalClient removes a global client
func (s *WebSocketServer) UnregisterGlobalClient(clientID string) error {
	if client, ok := s.globalClients.Delete(clientID); ok {
		_ = client.Close()
	}
	return nil
}

// Broadcast sends a message to all clients watching a task
func (s *WebSocketServer) Broadcast(taskID string, data []byte) {
	clients, _ := s.clients.Get(taskID)

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
	s.clients.Range(func(_ string, clients map[string]WebSocketClientInterface) bool {
		for _, client := range clients {
			if err := client.Send(data); err != nil {
				s.logger.WithError(err).Debug("Failed to send to WebSocket client")
			}
		}
		return true
	})

	s.broadcastGlobal(data)
}

// broadcastGlobal sends data to all global clients
func (s *WebSocketServer) broadcastGlobal(data []byte) {
	s.globalClients.Range(func(_ string, client WebSocketClientInterface) bool {
		if err := client.Send(data); err != nil {
			s.logger.WithError(err).Debug("Failed to send to global WebSocket client")
		}
		return true
	})
}

// readLoop reads messages from a WebSocket client
func (s *WebSocketServer) readLoop(client *WebSocketClient, taskID string) {
	conn := client.conn
	conn.SetReadLimit(s.config.MaxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(s.config.PongWait)) //nolint:errcheck
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(s.config.PongWait)) //nolint:errcheck
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
			_ = s.RegisterClient(msg.TaskID, client) //nolint:errcheck
		}
	case "unsubscribe":
		if msg.TaskID != "" {
			_ = s.UnregisterClient(msg.TaskID, client.ID()) //nolint:errcheck
		}
	case "ping":
		_ = client.Send([]byte(`{"type":"pong"}`)) //nolint:errcheck
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
			_ = client.conn.SetWriteDeadline(time.Now().Add(s.config.WriteWait)) //nolint:errcheck
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
	clients, _ := s.clients.Get(taskID)
	return len(clients)
}

// GetTotalClientCount returns the total number of connected clients
func (s *WebSocketServer) GetTotalClientCount() int {
	taskCount := 0
	s.clients.Range(func(_ string, clients map[string]WebSocketClientInterface) bool {
		taskCount += len(clients)
		return true
	})
	return taskCount + s.globalClients.Len()
}

// WebSocketMessage represents an incoming WebSocket message
type WebSocketMessage struct {
	Type   string      `json:"type"`
	TaskID string      `json:"task_id,omitempty"`
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

	_ = c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait)) //nolint:errcheck
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
