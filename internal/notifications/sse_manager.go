package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// SSEManager manages Server-Sent Events connections
type SSEManager struct {
	// Task-specific clients
	clients     map[string]map[chan<- []byte]struct{}
	clientsMu   sync.RWMutex

	// Global event clients (for all task events)
	globalClients   map[chan<- []byte]struct{}
	globalClientsMu sync.RWMutex

	// Configuration
	heartbeatInterval time.Duration
	bufferSize        int

	logger *logrus.Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// SSEConfig holds SSE configuration
type SSEConfig struct {
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	BufferSize        int           `yaml:"buffer_size"`
	MaxClients        int           `yaml:"max_clients"`
}

// DefaultSSEConfig returns default SSE configuration
func DefaultSSEConfig() *SSEConfig {
	return &SSEConfig{
		HeartbeatInterval: 30 * time.Second,
		BufferSize:        100,
		MaxClients:        1000,
	}
}

// NewSSEManager creates a new SSE manager
func NewSSEManager(config *SSEConfig, logger *logrus.Logger) *SSEManager {
	if config == nil {
		config = DefaultSSEConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &SSEManager{
		clients:           make(map[string]map[chan<- []byte]struct{}),
		globalClients:     make(map[chan<- []byte]struct{}),
		heartbeatInterval: config.HeartbeatInterval,
		bufferSize:        config.BufferSize,
		logger:            logger,
		ctx:               ctx,
		cancel:            cancel,
	}

	// Start heartbeat loop
	manager.wg.Add(1)
	go manager.heartbeatLoop()

	return manager
}

// Start starts the SSE manager
func (m *SSEManager) Start() error {
	m.logger.Info("SSE manager started")
	return nil
}

// Stop stops the SSE manager
func (m *SSEManager) Stop() error {
	m.logger.Info("Stopping SSE manager")
	m.cancel()
	m.wg.Wait()

	// Close all client channels
	m.clientsMu.Lock()
	for taskID, clients := range m.clients {
		for client := range clients {
			close(client)
		}
		delete(m.clients, taskID)
	}
	m.clientsMu.Unlock()

	m.globalClientsMu.Lock()
	for client := range m.globalClients {
		close(client)
	}
	m.globalClients = make(map[chan<- []byte]struct{})
	m.globalClientsMu.Unlock()

	return nil
}

// RegisterClient registers a client for a specific task
func (m *SSEManager) RegisterClient(taskID string, client chan<- []byte) error {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()

	if m.clients[taskID] == nil {
		m.clients[taskID] = make(map[chan<- []byte]struct{})
	}
	m.clients[taskID][client] = struct{}{}

	m.logger.WithFields(logrus.Fields{
		"task_id":      taskID,
		"total_clients": len(m.clients[taskID]),
	}).Debug("SSE client registered")

	return nil
}

// UnregisterClient removes a client from a task
func (m *SSEManager) UnregisterClient(taskID string, client chan<- []byte) error {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()

	if clients, exists := m.clients[taskID]; exists {
		delete(clients, client)
		if len(clients) == 0 {
			delete(m.clients, taskID)
		}
	}

	m.logger.WithField("task_id", taskID).Debug("SSE client unregistered")
	return nil
}

// RegisterGlobalClient registers a client for all events
func (m *SSEManager) RegisterGlobalClient(client chan<- []byte) error {
	m.globalClientsMu.Lock()
	defer m.globalClientsMu.Unlock()

	m.globalClients[client] = struct{}{}

	m.logger.WithField("total_global_clients", len(m.globalClients)).Debug("Global SSE client registered")
	return nil
}

// UnregisterGlobalClient removes a global client
func (m *SSEManager) UnregisterGlobalClient(client chan<- []byte) error {
	m.globalClientsMu.Lock()
	defer m.globalClientsMu.Unlock()

	delete(m.globalClients, client)
	return nil
}

// Broadcast sends a message to all clients watching a task
func (m *SSEManager) Broadcast(taskID string, data []byte) {
	m.clientsMu.RLock()
	clients := m.clients[taskID]
	m.clientsMu.RUnlock()

	// Format as SSE event
	sseData := formatSSEEvent("message", data)

	// Send to task-specific clients
	for client := range clients {
		select {
		case client <- sseData:
		default:
			// Client channel full, skip
			m.logger.WithField("task_id", taskID).Debug("SSE client channel full, skipping")
		}
	}

	// Also send to global clients
	m.broadcastGlobal(sseData)
}

// BroadcastEvent sends a named event to all clients watching a task
func (m *SSEManager) BroadcastEvent(taskID string, eventName string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	sseData := formatSSEEvent(eventName, jsonData)

	m.clientsMu.RLock()
	clients := m.clients[taskID]
	m.clientsMu.RUnlock()

	for client := range clients {
		select {
		case client <- sseData:
		default:
			m.logger.WithField("task_id", taskID).Debug("SSE client channel full")
		}
	}

	m.broadcastGlobal(sseData)
	return nil
}

// BroadcastAll sends a message to all connected clients
func (m *SSEManager) BroadcastAll(data []byte) {
	sseData := formatSSEEvent("message", data)

	m.clientsMu.RLock()
	for _, clients := range m.clients {
		for client := range clients {
			select {
			case client <- sseData:
			default:
			}
		}
	}
	m.clientsMu.RUnlock()

	m.broadcastGlobal(sseData)
}

// broadcastGlobal sends data to all global clients
func (m *SSEManager) broadcastGlobal(data []byte) {
	m.globalClientsMu.RLock()
	defer m.globalClientsMu.RUnlock()

	for client := range m.globalClients {
		select {
		case client <- data:
		default:
			m.logger.Debug("Global SSE client channel full")
		}
	}
}

// heartbeatLoop sends periodic heartbeats to keep connections alive
func (m *SSEManager) heartbeatLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.heartbeatInterval)
	defer ticker.Stop()

	heartbeat := formatSSEEvent("heartbeat", []byte(`{"type":"heartbeat"}`))

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.sendHeartbeats(heartbeat)
		}
	}
}

// sendHeartbeats sends heartbeat to all clients
func (m *SSEManager) sendHeartbeats(heartbeat []byte) {
	m.clientsMu.RLock()
	for _, clients := range m.clients {
		for client := range clients {
			select {
			case client <- heartbeat:
			default:
			}
		}
	}
	m.clientsMu.RUnlock()

	m.globalClientsMu.RLock()
	for client := range m.globalClients {
		select {
		case client <- heartbeat:
		default:
		}
	}
	m.globalClientsMu.RUnlock()
}

// GetClientCount returns the number of clients for a task
func (m *SSEManager) GetClientCount(taskID string) int {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()

	return len(m.clients[taskID])
}

// GetTotalClientCount returns the total number of connected clients
func (m *SSEManager) GetTotalClientCount() int {
	m.clientsMu.RLock()
	taskCount := 0
	for _, clients := range m.clients {
		taskCount += len(clients)
	}
	m.clientsMu.RUnlock()

	m.globalClientsMu.RLock()
	globalCount := len(m.globalClients)
	m.globalClientsMu.RUnlock()

	return taskCount + globalCount
}

// formatSSEEvent formats data as an SSE event
func formatSSEEvent(eventName string, data []byte) []byte {
	var result []byte
	result = append(result, []byte("event: "+eventName+"\n")...)
	result = append(result, []byte("data: ")...)
	result = append(result, data...)
	result = append(result, []byte("\n\n")...)
	return result
}

// SSESubscriber implements the Subscriber interface for SSE
type SSESubscriber struct {
	id       string
	taskID   string
	client   chan<- []byte
	active   bool
	activeMu sync.RWMutex
}

// NewSSESubscriber creates a new SSE subscriber
func NewSSESubscriber(id, taskID string, client chan<- []byte) *SSESubscriber {
	return &SSESubscriber{
		id:     id,
		taskID: taskID,
		client: client,
		active: true,
	}
}

func (s *SSESubscriber) Notify(ctx context.Context, notification *TaskNotification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	sseData := formatSSEEvent(notification.EventType, data)

	select {
	case s.client <- sseData:
		return nil
	default:
		return fmt.Errorf("client channel full")
	}
}

func (s *SSESubscriber) Type() NotificationType {
	return NotificationTypeSSE
}

func (s *SSESubscriber) ID() string {
	return s.id
}

func (s *SSESubscriber) IsActive() bool {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()
	return s.active
}

func (s *SSESubscriber) Close() error {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	s.active = false
	return nil
}
