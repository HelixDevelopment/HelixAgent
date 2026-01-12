package notifications

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"dev.helix.agent/internal/models"
)

// NotificationType represents the type of notification channel
type NotificationType string

const (
	NotificationTypeSSE       NotificationType = "sse"
	NotificationTypeWebSocket NotificationType = "websocket"
	NotificationTypeWebhook   NotificationType = "webhook"
	NotificationTypePolling   NotificationType = "polling"
)

// TaskNotification represents a notification to be sent
type TaskNotification struct {
	TaskID    string                 `json:"task_id"`
	EventType string                 `json:"event_type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Task      *models.BackgroundTask `json:"task,omitempty"`
}

// NotificationHub is the central event distribution system
type NotificationHub struct {
	sseManager       *SSEManager
	wsServer         *WebSocketServer
	webhookDispatcher *WebhookDispatcher
	pollingStore     *PollingStore

	// Subscribers for task-specific events
	subscribers     map[string][]Subscriber
	subscribersMu   sync.RWMutex

	// Global event subscribers
	globalSubs      []Subscriber
	globalSubsMu    sync.RWMutex

	logger    *logrus.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Event channel for async processing
	eventChan chan *TaskNotification
}

// Subscriber represents a notification subscriber
type Subscriber interface {
	// Notify sends a notification to the subscriber
	Notify(ctx context.Context, notification *TaskNotification) error

	// Type returns the subscriber type
	Type() NotificationType

	// ID returns a unique identifier for the subscriber
	ID() string

	// IsActive returns whether the subscriber is still active
	IsActive() bool

	// Close closes the subscriber
	Close() error
}

// HubConfig holds configuration for the notification hub
type HubConfig struct {
	EventBufferSize      int           `yaml:"event_buffer_size"`
	WorkerCount          int           `yaml:"worker_count"`
	NotificationTimeout  time.Duration `yaml:"notification_timeout"`
	RetryEnabled         bool          `yaml:"retry_enabled"`
	MaxRetries           int           `yaml:"max_retries"`
	RetryBackoff         time.Duration `yaml:"retry_backoff"`
}

// DefaultHubConfig returns sensible defaults
func DefaultHubConfig() *HubConfig {
	return &HubConfig{
		EventBufferSize:     1000,
		WorkerCount:         5,
		NotificationTimeout: 30 * time.Second,
		RetryEnabled:        true,
		MaxRetries:          3,
		RetryBackoff:        time.Second,
	}
}

// NewNotificationHub creates a new notification hub
func NewNotificationHub(
	config *HubConfig,
	sseManager *SSEManager,
	wsServer *WebSocketServer,
	webhookDispatcher *WebhookDispatcher,
	pollingStore *PollingStore,
	logger *logrus.Logger,
) *NotificationHub {
	if config == nil {
		config = DefaultHubConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	hub := &NotificationHub{
		sseManager:       sseManager,
		wsServer:         wsServer,
		webhookDispatcher: webhookDispatcher,
		pollingStore:     pollingStore,
		subscribers:      make(map[string][]Subscriber),
		globalSubs:       make([]Subscriber, 0),
		logger:           logger,
		ctx:              ctx,
		cancel:           cancel,
		eventChan:        make(chan *TaskNotification, config.EventBufferSize),
	}

	// Start event processing workers
	for i := 0; i < config.WorkerCount; i++ {
		hub.wg.Add(1)
		go hub.processEvents()
	}

	return hub
}

// Start starts the notification hub
func (h *NotificationHub) Start() error {
	h.logger.Info("Notification hub started")
	return nil
}

// Stop gracefully stops the notification hub
func (h *NotificationHub) Stop() error {
	h.logger.Info("Stopping notification hub")
	h.cancel()
	close(h.eventChan)
	h.wg.Wait()

	// Close all subscribers
	h.subscribersMu.Lock()
	for _, subs := range h.subscribers {
		for _, sub := range subs {
			sub.Close()
		}
	}
	h.subscribers = make(map[string][]Subscriber)
	h.subscribersMu.Unlock()

	h.globalSubsMu.Lock()
	for _, sub := range h.globalSubs {
		sub.Close()
	}
	h.globalSubs = nil
	h.globalSubsMu.Unlock()

	return nil
}

// NotifyTaskEvent sends a notification for a task event
func (h *NotificationHub) NotifyTaskEvent(ctx context.Context, task *models.BackgroundTask, event string, data map[string]interface{}) error {
	notification := &TaskNotification{
		TaskID:    task.ID,
		EventType: event,
		Timestamp: time.Now(),
		Data:      data,
		Task:      task,
	}

	// Store in polling store for polling clients
	if h.pollingStore != nil {
		h.pollingStore.StoreEvent(notification)
	}

	// Send async via event channel
	select {
	case h.eventChan <- notification:
		// Event queued
	default:
		// Channel full, log and drop
		h.logger.WithFields(logrus.Fields{
			"task_id": task.ID,
			"event":   event,
		}).Warn("Event channel full, dropping notification")
	}

	return nil
}

// Subscribe adds a subscriber for task events
func (h *NotificationHub) Subscribe(taskID string, subscriber Subscriber) {
	h.subscribersMu.Lock()
	defer h.subscribersMu.Unlock()

	h.subscribers[taskID] = append(h.subscribers[taskID], subscriber)

	h.logger.WithFields(logrus.Fields{
		"task_id":       taskID,
		"subscriber_id": subscriber.ID(),
		"type":          subscriber.Type(),
	}).Debug("Subscriber added")
}

// Unsubscribe removes a subscriber
func (h *NotificationHub) Unsubscribe(taskID string, subscriberID string) {
	h.subscribersMu.Lock()
	defer h.subscribersMu.Unlock()

	subs := h.subscribers[taskID]
	for i, sub := range subs {
		if sub.ID() == subscriberID {
			h.subscribers[taskID] = append(subs[:i], subs[i+1:]...)
			sub.Close()
			break
		}
	}
}

// SubscribeGlobal adds a global subscriber for all task events
func (h *NotificationHub) SubscribeGlobal(subscriber Subscriber) {
	h.globalSubsMu.Lock()
	defer h.globalSubsMu.Unlock()

	h.globalSubs = append(h.globalSubs, subscriber)
}

// UnsubscribeGlobal removes a global subscriber
func (h *NotificationHub) UnsubscribeGlobal(subscriberID string) {
	h.globalSubsMu.Lock()
	defer h.globalSubsMu.Unlock()

	for i, sub := range h.globalSubs {
		if sub.ID() == subscriberID {
			h.globalSubs = append(h.globalSubs[:i], h.globalSubs[i+1:]...)
			sub.Close()
			break
		}
	}
}

// RegisterSSEClient registers a client for SSE notifications
func (h *NotificationHub) RegisterSSEClient(ctx context.Context, taskID string, client chan<- []byte) error {
	if h.sseManager != nil {
		return h.sseManager.RegisterClient(taskID, client)
	}
	return nil
}

// UnregisterSSEClient removes an SSE client
func (h *NotificationHub) UnregisterSSEClient(ctx context.Context, taskID string, client chan<- []byte) error {
	if h.sseManager != nil {
		return h.sseManager.UnregisterClient(taskID, client)
	}
	return nil
}

// RegisterWebSocketClient registers a WebSocket client
func (h *NotificationHub) RegisterWebSocketClient(ctx context.Context, taskID string, client WebSocketClientInterface) error {
	if h.wsServer != nil {
		return h.wsServer.RegisterClient(taskID, client)
	}
	return nil
}

// BroadcastToTask broadcasts a message to all clients watching a task
func (h *NotificationHub) BroadcastToTask(ctx context.Context, taskID string, message []byte) error {
	// SSE broadcast
	if h.sseManager != nil {
		h.sseManager.Broadcast(taskID, message)
	}

	// WebSocket broadcast
	if h.wsServer != nil {
		h.wsServer.Broadcast(taskID, message)
	}

	return nil
}

// processEvents processes events from the channel
func (h *NotificationHub) processEvents() {
	defer h.wg.Done()

	for notification := range h.eventChan {
		h.dispatchNotification(notification)
	}
}

// dispatchNotification sends a notification to all relevant subscribers
func (h *NotificationHub) dispatchNotification(notification *TaskNotification) {
	// Serialize notification
	data, err := json.Marshal(notification)
	if err != nil {
		h.logger.WithError(err).Error("Failed to serialize notification")
		return
	}

	// Send to task-specific subscribers
	h.subscribersMu.RLock()
	taskSubs := h.subscribers[notification.TaskID]
	h.subscribersMu.RUnlock()

	for _, sub := range taskSubs {
		if sub.IsActive() {
			if err := sub.Notify(h.ctx, notification); err != nil {
				h.logger.WithError(err).WithFields(logrus.Fields{
					"task_id":       notification.TaskID,
					"subscriber_id": sub.ID(),
				}).Debug("Failed to notify subscriber")
			}
		}
	}

	// Send to global subscribers
	h.globalSubsMu.RLock()
	globalSubs := make([]Subscriber, len(h.globalSubs))
	copy(globalSubs, h.globalSubs)
	h.globalSubsMu.RUnlock()

	for _, sub := range globalSubs {
		if sub.IsActive() {
			if err := sub.Notify(h.ctx, notification); err != nil {
				h.logger.WithError(err).WithField("subscriber_id", sub.ID()).Debug("Failed to notify global subscriber")
			}
		}
	}

	// Broadcast via SSE
	if h.sseManager != nil {
		h.sseManager.Broadcast(notification.TaskID, data)
	}

	// Broadcast via WebSocket
	if h.wsServer != nil {
		h.wsServer.Broadcast(notification.TaskID, data)
	}

	// Dispatch webhooks
	if h.webhookDispatcher != nil && notification.Task != nil {
		h.webhookDispatcher.Dispatch(notification)
	}
}

// GetActiveSubscribers returns the count of active subscribers for a task
func (h *NotificationHub) GetActiveSubscribers(taskID string) int {
	h.subscribersMu.RLock()
	defer h.subscribersMu.RUnlock()

	count := 0
	for _, sub := range h.subscribers[taskID] {
		if sub.IsActive() {
			count++
		}
	}
	return count
}

// CleanupInactiveSubscribers removes inactive subscribers
func (h *NotificationHub) CleanupInactiveSubscribers() {
	h.subscribersMu.Lock()
	defer h.subscribersMu.Unlock()

	for taskID, subs := range h.subscribers {
		activeSubs := make([]Subscriber, 0)
		for _, sub := range subs {
			if sub.IsActive() {
				activeSubs = append(activeSubs, sub)
			} else {
				sub.Close()
			}
		}
		if len(activeSubs) == 0 {
			delete(h.subscribers, taskID)
		} else {
			h.subscribers[taskID] = activeSubs
		}
	}

	h.globalSubsMu.Lock()
	defer h.globalSubsMu.Unlock()

	activeSubs := make([]Subscriber, 0)
	for _, sub := range h.globalSubs {
		if sub.IsActive() {
			activeSubs = append(activeSubs, sub)
		} else {
			sub.Close()
		}
	}
	h.globalSubs = activeSubs
}
