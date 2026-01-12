package notifications

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// PollingStore provides an in-memory event buffer for polling clients
type PollingStore struct {
	// Events by task ID
	taskEvents   map[string][]*TaskNotification
	taskEventsMu sync.RWMutex

	// Global events (most recent)
	globalEvents   []*TaskNotification
	globalEventsMu sync.RWMutex

	// Configuration
	config *PollingConfig

	logger *logrus.Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// PollingConfig holds polling store configuration
type PollingConfig struct {
	MaxEventsPerTask int           `yaml:"max_events_per_task"`
	MaxGlobalEvents  int           `yaml:"max_global_events"`
	EventTTL         time.Duration `yaml:"event_ttl"`
	CleanupInterval  time.Duration `yaml:"cleanup_interval"`
}

// DefaultPollingConfig returns default polling configuration
func DefaultPollingConfig() *PollingConfig {
	return &PollingConfig{
		MaxEventsPerTask: 100,
		MaxGlobalEvents:  1000,
		EventTTL:         15 * time.Minute,
		CleanupInterval:  1 * time.Minute,
	}
}

// NewPollingStore creates a new polling store
func NewPollingStore(config *PollingConfig, logger *logrus.Logger) *PollingStore {
	if config == nil {
		config = DefaultPollingConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	store := &PollingStore{
		taskEvents:   make(map[string][]*TaskNotification),
		globalEvents: make([]*TaskNotification, 0, config.MaxGlobalEvents),
		config:       config,
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Start cleanup loop
	store.wg.Add(1)
	go store.cleanupLoop()

	return store
}

// Start starts the polling store
func (s *PollingStore) Start() error {
	s.logger.Info("Polling store started")
	return nil
}

// Stop stops the polling store
func (s *PollingStore) Stop() error {
	s.logger.Info("Stopping polling store")
	s.cancel()
	s.wg.Wait()
	return nil
}

// StoreEvent stores an event for polling clients
func (s *PollingStore) StoreEvent(notification *TaskNotification) {
	// Store in task-specific events
	s.taskEventsMu.Lock()
	events := s.taskEvents[notification.TaskID]
	events = append(events, notification)

	// Trim if over limit
	if len(events) > s.config.MaxEventsPerTask {
		events = events[len(events)-s.config.MaxEventsPerTask:]
	}
	s.taskEvents[notification.TaskID] = events
	s.taskEventsMu.Unlock()

	// Store in global events
	s.globalEventsMu.Lock()
	s.globalEvents = append(s.globalEvents, notification)

	// Trim if over limit
	if len(s.globalEvents) > s.config.MaxGlobalEvents {
		s.globalEvents = s.globalEvents[len(s.globalEvents)-s.config.MaxGlobalEvents:]
	}
	s.globalEventsMu.Unlock()
}

// GetTaskEvents retrieves events for a specific task
func (s *PollingStore) GetTaskEvents(taskID string, since *time.Time, limit int) []*TaskNotification {
	s.taskEventsMu.RLock()
	defer s.taskEventsMu.RUnlock()

	events := s.taskEvents[taskID]
	if len(events) == 0 {
		return nil
	}

	result := make([]*TaskNotification, 0)
	for _, event := range events {
		if since != nil && !event.Timestamp.After(*since) {
			continue
		}
		result = append(result, event)
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

// GetGlobalEvents retrieves global events
func (s *PollingStore) GetGlobalEvents(since *time.Time, limit int) []*TaskNotification {
	s.globalEventsMu.RLock()
	defer s.globalEventsMu.RUnlock()

	if len(s.globalEvents) == 0 {
		return nil
	}

	result := make([]*TaskNotification, 0)
	for _, event := range s.globalEvents {
		if since != nil && !event.Timestamp.After(*since) {
			continue
		}
		result = append(result, event)
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

// GetLatestTaskEvent retrieves the most recent event for a task
func (s *PollingStore) GetLatestTaskEvent(taskID string) *TaskNotification {
	s.taskEventsMu.RLock()
	defer s.taskEventsMu.RUnlock()

	events := s.taskEvents[taskID]
	if len(events) == 0 {
		return nil
	}

	return events[len(events)-1]
}

// GetEventCount returns the number of events for a task
func (s *PollingStore) GetEventCount(taskID string) int {
	s.taskEventsMu.RLock()
	defer s.taskEventsMu.RUnlock()

	return len(s.taskEvents[taskID])
}

// GetGlobalEventCount returns the total number of global events
func (s *PollingStore) GetGlobalEventCount() int {
	s.globalEventsMu.RLock()
	defer s.globalEventsMu.RUnlock()

	return len(s.globalEvents)
}

// ClearTaskEvents removes all events for a task
func (s *PollingStore) ClearTaskEvents(taskID string) {
	s.taskEventsMu.Lock()
	delete(s.taskEvents, taskID)
	s.taskEventsMu.Unlock()
}

// cleanupLoop periodically removes expired events
func (s *PollingStore) cleanupLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

// cleanup removes expired events
func (s *PollingStore) cleanup() {
	cutoff := time.Now().Add(-s.config.EventTTL)

	// Cleanup task events
	s.taskEventsMu.Lock()
	for taskID, events := range s.taskEvents {
		filtered := make([]*TaskNotification, 0)
		for _, event := range events {
			if event.Timestamp.After(cutoff) {
				filtered = append(filtered, event)
			}
		}
		if len(filtered) == 0 {
			delete(s.taskEvents, taskID)
		} else {
			s.taskEvents[taskID] = filtered
		}
	}
	s.taskEventsMu.Unlock()

	// Cleanup global events
	s.globalEventsMu.Lock()
	filtered := make([]*TaskNotification, 0)
	for _, event := range s.globalEvents {
		if event.Timestamp.After(cutoff) {
			filtered = append(filtered, event)
		}
	}
	s.globalEvents = filtered
	s.globalEventsMu.Unlock()
}

// GetStats returns polling store statistics
func (s *PollingStore) GetStats() map[string]interface{} {
	s.taskEventsMu.RLock()
	taskCount := len(s.taskEvents)
	taskEventCount := 0
	for _, events := range s.taskEvents {
		taskEventCount += len(events)
	}
	s.taskEventsMu.RUnlock()

	s.globalEventsMu.RLock()
	globalEventCount := len(s.globalEvents)
	s.globalEventsMu.RUnlock()

	return map[string]interface{}{
		"tasks_with_events": taskCount,
		"task_events_total": taskEventCount,
		"global_events":     globalEventCount,
	}
}

// PollRequest represents a polling request
type PollRequest struct {
	TaskID string     `json:"task_id,omitempty"`
	Since  *time.Time `json:"since,omitempty"`
	Limit  int        `json:"limit,omitempty"`
}

// PollResponse represents a polling response
type PollResponse struct {
	Events    []*TaskNotification `json:"events"`
	Count     int                 `json:"count"`
	Timestamp time.Time           `json:"timestamp"`
	HasMore   bool                `json:"has_more"`
}

// Poll handles a polling request
func (s *PollingStore) Poll(req *PollRequest) *PollResponse {
	var events []*TaskNotification

	if req.Limit <= 0 {
		req.Limit = 100
	}

	if req.TaskID != "" {
		events = s.GetTaskEvents(req.TaskID, req.Since, req.Limit+1)
	} else {
		events = s.GetGlobalEvents(req.Since, req.Limit+1)
	}

	hasMore := len(events) > req.Limit
	if hasMore {
		events = events[:req.Limit]
	}

	return &PollResponse{
		Events:    events,
		Count:     len(events),
		Timestamp: time.Now(),
		HasMore:   hasMore,
	}
}

// PollingSubscriber implements the Subscriber interface for polling
type PollingSubscriber struct {
	id     string
	taskID string
	store  *PollingStore
	active bool
	mu     sync.RWMutex
}

// NewPollingSubscriber creates a new polling subscriber
func NewPollingSubscriber(id, taskID string, store *PollingStore) *PollingSubscriber {
	return &PollingSubscriber{
		id:     id,
		taskID: taskID,
		store:  store,
		active: true,
	}
}

func (s *PollingSubscriber) Notify(ctx context.Context, notification *TaskNotification) error {
	s.store.StoreEvent(notification)
	return nil
}

func (s *PollingSubscriber) Type() NotificationType {
	return NotificationTypePolling
}

func (s *PollingSubscriber) ID() string {
	return s.id
}

func (s *PollingSubscriber) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

func (s *PollingSubscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
	return nil
}
