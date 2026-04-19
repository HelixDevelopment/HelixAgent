package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// SSEManager manages Server-Sent Events connections.
//
// Concurrent-safe by construction (CONST-029): the entire client topology
// (per-task clients, global clients, per-IP counters) lives in a single
// sseState held under a sentinel key in a safe.Store. Every broadcast,
// register, and close runs inside one Update callback, which preserves
// the original invariant that "channel close" and "channel send" are
// mutually exclusive — a property a per-collection Store cannot give us
// (Pattern Epsilon — joint atomicity via state struct).
type SSEManager struct {
	state *safe.Store[string, *sseState]

	// Configuration (read-only after construction)
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

// sseState is the joint mutable state held under sseStateKey.
type sseState struct {
	clients       map[string]map[chan<- []byte]struct{}
	globalClients map[chan<- []byte]struct{}
	ipConns       map[string]int
}

const sseStateKey = "_"

func newSSEState() *sseState {
	return &sseState{
		clients:       make(map[string]map[chan<- []byte]struct{}),
		globalClients: make(map[chan<- []byte]struct{}),
		ipConns:       make(map[string]int),
	}
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

	store := safe.NewStore[string, *sseState]()
	store.Put(sseStateKey, newSSEState())
	manager := &SSEManager{
		state:             store,
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

// withState runs fn under the state Store's write lock; fn may mutate
// the maps in-place.
func (m *SSEManager) withState(fn func(*sseState)) {
	m.state.Update(sseStateKey, func(s *sseState, _ bool) (*sseState, bool) {
		if s == nil {
			s = newSSEState()
		}
		fn(s)
		return s, true
	})
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

		// Close all client channels under the state Store's write lock
		// so concurrent Broadcasts (which also run under withState)
		// cannot send to a half-closed channel.
		m.withState(func(s *sseState) {
			for taskID, clients := range s.clients {
				for client := range clients {
					close(client)
				}
				delete(s.clients, taskID)
			}
			for client := range s.globalClients {
				close(client)
			}
			s.globalClients = make(map[chan<- []byte]struct{})
		})
	})
	return stopErr
}

// RegisterClient registers a client for a specific task
func (m *SSEManager) RegisterClient(taskID string, client chan<- []byte) error {
	var totalAfter int
	m.withState(func(s *sseState) {
		if s.clients[taskID] == nil {
			s.clients[taskID] = make(map[chan<- []byte]struct{})
		}
		s.clients[taskID][client] = struct{}{}
		totalAfter = len(s.clients[taskID])
	})

	m.logger.WithFields(logrus.Fields{
		"task_id":       taskID,
		"total_clients": totalAfter,
	}).Debug("SSE client registered")

	return nil
}

// UnregisterClient removes a client from a task
func (m *SSEManager) UnregisterClient(taskID string, client chan<- []byte) error {
	m.withState(func(s *sseState) {
		if clients, exists := s.clients[taskID]; exists {
			delete(clients, client)
			if len(clients) == 0 {
				delete(s.clients, taskID)
			}
		}
	})

	m.logger.WithField("task_id", taskID).Debug("SSE client unregistered")
	return nil
}

// RegisterGlobalClient registers a client for all events
func (m *SSEManager) RegisterGlobalClient(client chan<- []byte) error {
	var totalAfter int
	m.withState(func(s *sseState) {
		s.globalClients[client] = struct{}{}
		totalAfter = len(s.globalClients)
	})

	m.logger.WithField("total_global_clients", totalAfter).Debug("Global SSE client registered")
	return nil
}

// UnregisterGlobalClient removes a global client
func (m *SSEManager) UnregisterGlobalClient(client chan<- []byte) error {
	m.withState(func(s *sseState) {
		delete(s.globalClients, client)
	})
	return nil
}

// RegisterClientWithIP registers a task-scoped client and enforces a per-IP
// connection cap. Returns an error when the caller's IP has reached maxConnsPerIP.
func (m *SSEManager) RegisterClientWithIP(taskID string, clientIP string, client chan<- []byte) error {
	var (
		rejected bool
		current  int
	)
	m.withState(func(s *sseState) {
		current = s.ipConns[clientIP]
		if current >= m.maxConnsPerIP {
			rejected = true
			return
		}
		s.ipConns[clientIP]++
	})
	if rejected {
		m.logger.WithFields(logrus.Fields{
			"client_ip":        clientIP,
			"current_conns":    current,
			"max_conns_per_ip": m.maxConnsPerIP,
		}).Warn("SSE connection rejected: per-IP cap reached")
		return fmt.Errorf("connection limit reached for IP %s (%d/%d)", clientIP, current, m.maxConnsPerIP)
	}

	return m.RegisterClient(taskID, client)
}

// UnregisterClientWithIP removes a task-scoped client and decrements the per-IP counter.
func (m *SSEManager) UnregisterClientWithIP(taskID string, clientIP string, client chan<- []byte) error {
	m.withState(func(s *sseState) {
		if s.ipConns[clientIP] > 0 {
			s.ipConns[clientIP]--
			if s.ipConns[clientIP] == 0 {
				delete(s.ipConns, clientIP)
			}
		}
	})

	return m.UnregisterClient(taskID, client)
}

// GetIPConnCount returns the current number of connections for a given client IP.
func (m *SSEManager) GetIPConnCount(clientIP string) int {
	var count int
	m.withState(func(s *sseState) {
		count = s.ipConns[clientIP]
	})
	return count
}

// Broadcast sends a message to all clients watching a task
func (m *SSEManager) Broadcast(taskID string, data []byte) {
	if m.closed.Load() {
		return
	}

	sseData := formatSSEEvent("message", data)

	// Send under withState so concurrent Stop() cannot close channels
	// mid-send (Stop also runs its closes inside withState).
	m.withState(func(s *sseState) {
		if m.closed.Load() {
			return
		}
		for client := range s.clients[taskID] {
			select {
			case client <- sseData:
			default:
				m.logger.WithField("task_id", taskID).Debug("SSE client channel full, skipping")
			}
		}
		for client := range s.globalClients {
			select {
			case client <- sseData:
			default:
				m.logger.Debug("Global SSE client channel full")
			}
		}
	})
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

	m.withState(func(s *sseState) {
		if m.closed.Load() {
			return
		}
		for client := range s.clients[taskID] {
			select {
			case client <- sseData:
			default:
				m.logger.WithField("task_id", taskID).Debug("SSE client channel full")
			}
		}
		for client := range s.globalClients {
			select {
			case client <- sseData:
			default:
				m.logger.Debug("Global SSE client channel full")
			}
		}
	})
	return nil
}

// BroadcastAll sends a message to all connected clients
func (m *SSEManager) BroadcastAll(data []byte) {
	if m.closed.Load() {
		return
	}

	sseData := formatSSEEvent("message", data)

	m.withState(func(s *sseState) {
		if m.closed.Load() {
			return
		}
		for _, clients := range s.clients {
			for client := range clients {
				select {
				case client <- sseData:
				default:
				}
			}
		}
		for client := range s.globalClients {
			select {
			case client <- sseData:
			default:
				m.logger.Debug("Global SSE client channel full")
			}
		}
	})
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

	m.withState(func(s *sseState) {
		if m.closed.Load() {
			return
		}
		for _, clients := range s.clients {
			for client := range clients {
				select {
				case client <- heartbeat:
				default:
				}
			}
		}
		for client := range s.globalClients {
			select {
			case client <- heartbeat:
			default:
			}
		}
	})
}

// GetClientCount returns the number of clients for a task
func (m *SSEManager) GetClientCount(taskID string) int {
	var count int
	m.withState(func(s *sseState) {
		count = len(s.clients[taskID])
	})
	return count
}

// GetTotalClientCount returns the total number of connected clients
func (m *SSEManager) GetTotalClientCount() int {
	var total int
	m.withState(func(s *sseState) {
		for _, clients := range s.clients {
			total += len(clients)
		}
		total += len(s.globalClients)
	})
	return total
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
