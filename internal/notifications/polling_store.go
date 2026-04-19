package notifications

import (
	"context"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"
	"github.com/sirupsen/logrus"
)

// PollingStore provides an in-memory event buffer for polling clients
//
// Concurrent-safe by construction (CONST-029):
//   - taskEvents is a safe.Store; per-task slice mutations go through Update.
//   - globalEvents is a safe.Store with a single constant key holding the
//     full slice — append+trim is performed atomically under one Update
//     callback (Pattern Epsilon — joint atomicity via state-struct).
type PollingStore struct {
	// Events by task ID — value is a []*TaskNotification, replaced wholesale
	// on Update so readers can iterate a snapshot lock-free.
	taskEvents *safe.Store[string, []*TaskNotification]

	// Global events (most recent). Single-key Store: append+trim atomic.
	globalEvents *safe.Store[string, []*TaskNotification]

	// Configuration
	config *PollingConfig

	logger *logrus.Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// globalEventsKey is the single-key sentinel under which the entire
// globalEvents slice lives in the Store. Pattern Epsilon.
const globalEventsKey = "_"

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
		taskEvents:   safe.NewStore[string, []*TaskNotification](),
		globalEvents: safe.NewStore[string, []*TaskNotification](),
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
	maxPerTask := s.config.MaxEventsPerTask
	maxGlobal := s.config.MaxGlobalEvents

	// Store in task-specific events — atomic append + trim
	s.taskEvents.Update(notification.TaskID, func(events []*TaskNotification, _ bool) ([]*TaskNotification, bool) {
		events = append(events, notification)
		if len(events) > maxPerTask {
			events = events[len(events)-maxPerTask:]
		}
		return events, true
	})

	// Store in global events — atomic append + trim under single key
	s.globalEvents.Update(globalEventsKey, func(events []*TaskNotification, _ bool) ([]*TaskNotification, bool) {
		events = append(events, notification)
		if len(events) > maxGlobal {
			events = events[len(events)-maxGlobal:]
		}
		return events, true
	})
}

// GetTaskEvents retrieves events for a specific task
func (s *PollingStore) GetTaskEvents(taskID string, since *time.Time, limit int) []*TaskNotification {
	events, _ := s.taskEvents.Get(taskID)
	return filterEvents(events, since, limit)
}

// GetGlobalEvents retrieves global events
func (s *PollingStore) GetGlobalEvents(since *time.Time, limit int) []*TaskNotification {
	events, _ := s.globalEvents.Get(globalEventsKey)
	return filterEvents(events, since, limit)
}

// filterEvents copies events that match `since` (if set) up to `limit`.
// Safe to call lock-free on a slice obtained from safe.Store.Get because
// the Store mutates by replacement, never in-place.
func filterEvents(events []*TaskNotification, since *time.Time, limit int) []*TaskNotification {
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

// GetLatestTaskEvent retrieves the most recent event for a task
func (s *PollingStore) GetLatestTaskEvent(taskID string) *TaskNotification {
	events, _ := s.taskEvents.Get(taskID)
	if len(events) == 0 {
		return nil
	}
	return events[len(events)-1]
}

// GetEventCount returns the number of events for a task
func (s *PollingStore) GetEventCount(taskID string) int {
	events, _ := s.taskEvents.Get(taskID)
	return len(events)
}

// GetGlobalEventCount returns the total number of global events
func (s *PollingStore) GetGlobalEventCount() int {
	events, _ := s.globalEvents.Get(globalEventsKey)
	return len(events)
}

// ClearTaskEvents removes all events for a task
func (s *PollingStore) ClearTaskEvents(taskID string) {
	s.taskEvents.Delete(taskID)
}

// cleanupLoop periodically removes expired events
func (s *PollingStore) cleanupLoop() {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("cleanup loop panicked (recovered): %v", r)
		}
	}()

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

	// Cleanup task events — Update each key atomically; drop key when empty.
	for _, taskID := range s.taskEvents.Keys() {
		s.taskEvents.Update(taskID, func(events []*TaskNotification, ok bool) ([]*TaskNotification, bool) {
			if !ok {
				return nil, false
			}
			filtered := make([]*TaskNotification, 0, len(events))
			for _, event := range events {
				if event.Timestamp.After(cutoff) {
					filtered = append(filtered, event)
				}
			}
			return filtered, len(filtered) > 0
		})
	}

	// Cleanup global events — single-key Update.
	s.globalEvents.Update(globalEventsKey, func(events []*TaskNotification, _ bool) ([]*TaskNotification, bool) {
		filtered := make([]*TaskNotification, 0, len(events))
		for _, event := range events {
			if event.Timestamp.After(cutoff) {
				filtered = append(filtered, event)
			}
		}
		return filtered, true
	})
}

// GetStats returns polling store statistics
func (s *PollingStore) GetStats() map[string]interface{} {
	taskCount := s.taskEvents.Len()
	taskEventCount := 0
	s.taskEvents.Range(func(_ string, events []*TaskNotification) bool {
		taskEventCount += len(events)
		return true
	})

	globalEvents, _ := s.globalEvents.Get(globalEventsKey)
	return map[string]interface{}{
		"tasks_with_events": taskCount,
		"task_events_total": taskEventCount,
		"global_events":     len(globalEvents),
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
