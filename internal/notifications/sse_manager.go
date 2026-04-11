package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// SSEManager manages Server-Sent Events connections
type SSEManager struct {
	// Task-specific clients
	clients   map[string]map[chan<- []byte]struct{}
	clientsMu sync.RWMutex

	// Global event clients (for all task events)
	globalClients   map[chan<- []byte]struct{}
	globalClientsMu sync.RWMutex

	// Per-IP connection tracking for backpressure
	ipConns   map[string]int
	ipConnsMu sync.Mutex

	// Configuration
	heartbeatInterval time.Duration
	bufferSize        int
	maxConnsPerIP     int

	logger   *logrus.Logger
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	closed   atomic.Bool
	stopOnce sync.Once
}

// SSEConfig holds SSE configuration
type SSEConfig struct {
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	BufferSize        int           `yaml:"buffer_size"`
	MaxClients        int           `yaml:"max_clients"`
	MaxConnsPerIP     int           `yaml:"max_conns_per_ip"`
}

// DefaultSSEConfig returns default SSE configuration
func DefaultSSEConfig() *SSEConfig {
	return &SSEConfig{
		HeartbeatInterval: 30 * time.Second,
		BufferSize:        100,
		MaxClients:        1000,
		MaxConnsPerIP:     10,
	}
}

// NewSSEManager creates a new SSE manager
func NewSSEManager(config *SSEConfig, logger *logrus.Logger) *SSEManager {
	if config == nil {
		config = DefaultSSEConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	maxConnsPerIP := config.MaxConnsPerIP
	if maxConnsPerIP <= 0 {
		maxConnsPerIP = 10
	}

	manager := &SSEManager{
		clients:           make(map[string]map[chan<- []byte]struct{}),
		globalClients:     make(map[chan<- []byte]struct{}),
		ipConns:           make(map[string]int),
		heartbeatInterval: config.HeartbeatInterval,
		bufferSize:        config.BufferSize,
		maxConnsPerIP:     maxConnsPerIP,
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

// Stop stops the SSE manager. It is safe to call multiple times;
// only the first call performs the shutdown.
func (m *SSEManager) Stop() error {
	var stopErr error
	m.stopOnce.Do(func() {
		m.logger.Info("Stopping SSE manager")
		m.closed.Store(true)
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
	})
	return stopErr
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
		"task_id":       taskID,
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

// RegisterClientWithIP registers a task-scoped client and enforces a per-IP
// connection cap. Returns an error when the caller's IP has reached maxConnsPerIP.
func (m *SSEManager) RegisterClientWithIP(taskID string, clientIP string, client chan<- []byte) error {
	m.ipConnsMu.Lock()
	current := m.ipConns[clientIP]
	if current >= m.maxConnsPerIP {
		m.ipConnsMu.Unlock()
		m.logger.WithFields(logrus.Fields{
			"client_ip":        clientIP,
			"current_conns":    current,
			"max_conns_per_ip": m.maxConnsPerIP,
		}).Warn("SSE connection rejected: per-IP cap reached")
		return fmt.Errorf("connection limit reached for IP %s (%d/%d)", clientIP, current, m.maxConnsPerIP)
	}
	m.ipConns[clientIP]++
	m.ipConnsMu.Unlock()

	return m.RegisterClient(taskID, client)
}

// UnregisterClientWithIP removes a task-scoped client and decrements the per-IP counter.
func (m *SSEManager) UnregisterClientWithIP(taskID string, clientIP string, client chan<- []byte) error {
	m.ipConnsMu.Lock()
	if m.ipConns[clientIP] > 0 {
		m.ipConns[clientIP]--
		if m.ipConns[clientIP] == 0 {
			delete(m.ipConns, clientIP)
		}
	}
	m.ipConnsMu.Unlock()

	return m.UnregisterClient(taskID, client)
}

// GetIPConnCount returns the current number of connections for a given client IP.
func (m *SSEManager) GetIPConnCount(clientIP string) int {
	m.ipConnsMu.Lock()
	defer m.ipConnsMu.Unlock()
	return m.ipConns[clientIP]
}

// Broadcast sends a message to all clients watching a task
func (m *SSEManager) Broadcast(taskID string, data []byte) {
	if m.closed.Load() {
		return
	}

	// Format as SSE event
	sseData := formatSSEEvent("message", data)

	// Hold RLock during sends so Stop() cannot close channels
	// while we are iterating
	m.clientsMu.RLock()
	if !m.closed.Load() {
		for client := range m.clients[taskID] {
			select {
			case client <- sseData:
			default:
				// Client channel full, skip
				m.logger.WithField("task_id", taskID).Debug("SSE client channel full, skipping")
			}
		}
	}
	m.clientsMu.RUnlock()

	// Also send to global clients
	m.broadcastGlobal(sseData)
}

// BroadcastEvent sends a named event to all clients watching a task
func (m *SSEManager) BroadcastEvent(taskID string, eventName string, data interface{}) error {
	if m.closed.Load() {
		return nil
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	sseData := formatSSEEvent(eventName, jsonData)

	m.clientsMu.RLock()
	if !m.closed.Load() {
		for client := range m.clients[taskID] {
			select {
			case client <- sseData:
			default:
				m.logger.WithField("task_id", taskID).Debug("SSE client channel full")
			}
		}
	}
	m.clientsMu.RUnlock()

	m.broadcastGlobal(sseData)
	return nil
}

// BroadcastAll sends a message to all connected clients
func (m *SSEManager) BroadcastAll(data []byte) {
	if m.closed.Load() {
		return
	}

	sseData := formatSSEEvent("message", data)

	m.clientsMu.RLock()
	if !m.closed.Load() {
		for _, clients := range m.clients {
			for client := range clients {
				select {
				case client <- sseData:
				default:
				}
			}
		}
	}
	m.clientsMu.RUnlock()

	m.broadcastGlobal(sseData)
}

// broadcastGlobal sends data to all global clients
func (m *SSEManager) broadcastGlobal(data []byte) {
	if m.closed.Load() {
		return
	}

	m.globalClientsMu.RLock()
	if !m.closed.Load() {
		for client := range m.globalClients {
			select {
			case client <- data:
			default:
				m.logger.Debug("Global SSE client channel full")
			}
		}
	}
	m.globalClientsMu.RUnlock()
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
	if m.closed.Load() {
		return
	}

	m.clientsMu.RLock()
	if !m.closed.Load() {
		for _, clients := range m.clients {
			for client := range clients {
				select {
				case client <- heartbeat:
				default:
				}
			}
		}
	}
	m.clientsMu.RUnlock()

	m.globalClientsMu.RLock()
	if !m.closed.Load() {
		for client := range m.globalClients {
			select {
			case client <- heartbeat:
			default:
			}
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
