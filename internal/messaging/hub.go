package messaging

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"
)

// MessagingHub provides a unified interface for all messaging operations.
// It combines task queuing (RabbitMQ) and event streaming (Kafka) with
// an in-memory fallback for testing and development.
//
// Concurrency model (CONST-029): subscriptions → *safe.Store; connected
// → atomic.Bool. No survivor mutex.
type MessagingHub struct {
	// taskQueue is the task queue broker (RabbitMQ).
	taskQueue TaskQueueBroker
	// eventStream is the event stream broker (Kafka).
	eventStream EventStreamBroker
	// fallback is the in-memory fallback broker.
	fallback MessageBroker

	// config is the hub configuration.
	config *HubConfig
	// router routes messages to appropriate brokers.
	router *MessageRouter
	// middleware is the middleware chain.
	middleware *MiddlewareChain
	// metrics collects hub metrics.
	metrics *HubMetrics
	// taskRegistry holds task handlers.
	taskRegistry *TaskRegistry
	// eventRegistry holds event handlers.
	eventRegistry *EventRegistry

	// subscriptions tracks active subscriptions.
	subscriptions *safe.Store[string, Subscription]

	// state
	connected atomic.Bool
	stopCh    chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// HubConfig holds configuration for the messaging hub.
type HubConfig struct {
	// TaskQueueEnabled enables task queue broker.
	TaskQueueEnabled bool `json:"task_queue_enabled" yaml:"task_queue_enabled"`
	// TaskQueueConfig is the task queue configuration.
	TaskQueueConfig *TaskQueueConfig `json:"task_queue_config" yaml:"task_queue_config"`

	// EventStreamEnabled enables event stream broker.
	EventStreamEnabled bool `json:"event_stream_enabled" yaml:"event_stream_enabled"`
	// EventStreamConfig is the event stream configuration.
	EventStreamConfig *EventStreamConfig `json:"event_stream_config" yaml:"event_stream_config"`

	// FallbackEnabled enables in-memory fallback.
	FallbackEnabled bool `json:"fallback_enabled" yaml:"fallback_enabled"`

	// UseFallbackOnError falls back to in-memory on broker errors.
	UseFallbackOnError bool `json:"use_fallback_on_error" yaml:"use_fallback_on_error"`

	// HealthCheckInterval is the health check interval.
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`

	// RetryConfig is the retry configuration.
	RetryConfig *RetryConfig `json:"retry_config" yaml:"retry_config"`

	// CircuitBreakerThreshold is the failure threshold for circuit breaker.
	CircuitBreakerThreshold int `json:"circuit_breaker_threshold" yaml:"circuit_breaker_threshold"`
	// CircuitBreakerTimeout is the reset timeout for circuit breaker.
	CircuitBreakerTimeout time.Duration `json:"circuit_breaker_timeout" yaml:"circuit_breaker_timeout"`
}

// DefaultHubConfig returns the default hub configuration.
func DefaultHubConfig() *HubConfig {
	return &HubConfig{
		TaskQueueEnabled:        true,
		TaskQueueConfig:         DefaultTaskQueueConfig(),
		EventStreamEnabled:      true,
		EventStreamConfig:       DefaultEventStreamConfig(),
		FallbackEnabled:         true,
		UseFallbackOnError:      true,
		HealthCheckInterval:     30 * time.Second,
		RetryConfig:             DefaultRetryConfig(),
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   30 * time.Second,
	}
}

// HubMetrics holds metrics for the messaging hub.
type HubMetrics struct {
	*BrokerMetrics
	TaskQueueMetrics   *BrokerMetrics `json:"task_queue_metrics,omitempty"`
	EventStreamMetrics *BrokerMetrics `json:"event_stream_metrics,omitempty"`
	FallbackMetrics    *BrokerMetrics `json:"fallback_metrics,omitempty"`
	FallbackUsages     atomic.Int64   `json:"fallback_usages"`
}

// NewHubMetrics creates new hub metrics.
func NewHubMetrics() *HubMetrics {
	return &HubMetrics{
		BrokerMetrics: NewBrokerMetrics(),
	}
}

// NewMessagingHub creates a new messaging hub.
func NewMessagingHub(config *HubConfig) *MessagingHub {
	if config == nil {
		config = DefaultHubConfig()
	}
	return &MessagingHub{
		config:        config,
		router:        NewMessageRouter(),
		middleware:    NewMiddlewareChain(),
		metrics:       NewHubMetrics(),
		taskRegistry:  NewTaskRegistry(),
		eventRegistry: NewEventRegistry(),
		subscriptions: safe.NewStore[string, Subscription](),
		stopCh:        make(chan struct{}),
	}
}

// Initialize initializes the messaging hub with the configured brokers.
func (h *MessagingHub) Initialize(ctx context.Context) error {
	var errs MultiError

	// Initialize task queue broker
	if h.config.TaskQueueEnabled && h.taskQueue != nil {
		if err := h.taskQueue.Connect(ctx); err != nil {
			errs.Add(fmt.Errorf("task queue connection failed: %w", err))
		} else {
			h.metrics.TaskQueueMetrics = h.taskQueue.GetMetrics()
		}
	}

	// Initialize event stream broker
	if h.config.EventStreamEnabled && h.eventStream != nil {
		if err := h.eventStream.Connect(ctx); err != nil {
			errs.Add(fmt.Errorf("event stream connection failed: %w", err))
		} else {
			h.metrics.EventStreamMetrics = h.eventStream.GetMetrics()
		}
	}

	// Initialize fallback broker
	if h.config.FallbackEnabled && h.fallback != nil {
		if err := h.fallback.Connect(ctx); err != nil {
			errs.Add(fmt.Errorf("fallback broker connection failed: %w", err))
		} else {
			h.metrics.FallbackMetrics = h.fallback.GetMetrics()
		}
	}

	if errs.HasErrors() && !h.config.UseFallbackOnError {
		return errs.ErrorOrNil()
	}

	h.connected.Store(true)

	// Start health check goroutine with WaitGroup tracking to prevent leaks.
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		h.healthCheckLoop()
	}()

	return nil
}

// Close closes the messaging hub and all brokers.
// It is safe to call Close multiple times; subsequent calls are no-ops for the
// stopCh signal. Close waits for the background health check goroutine to exit
// before tearing down brokers.
func (h *MessagingHub) Close(ctx context.Context) error {
	h.closeOnce.Do(func() { close(h.stopCh) })
	h.wg.Wait()

	var errs MultiError

	// Unsubscribe all subscriptions
	h.subscriptions.Range(func(_ string, sub Subscription) bool {
		if err := sub.Unsubscribe(); err != nil {
			errs.Add(err)
		}
		return true
	})
	h.subscriptions.Clear()

	// Close brokers
	if h.taskQueue != nil {
		if err := h.taskQueue.Close(ctx); err != nil {
			errs.Add(err)
		}
	}
	if h.eventStream != nil {
		if err := h.eventStream.Close(ctx); err != nil {
			errs.Add(err)
		}
	}
	if h.fallback != nil {
		if err := h.fallback.Close(ctx); err != nil {
			errs.Add(err)
		}
	}

	h.connected.Store(false)
	return errs.ErrorOrNil()
}

// SetTaskQueueBroker sets the task queue broker.
func (h *MessagingHub) SetTaskQueueBroker(broker TaskQueueBroker) {
	h.taskQueue = broker
}

// SetEventStreamBroker sets the event stream broker.
func (h *MessagingHub) SetEventStreamBroker(broker EventStreamBroker) {
	h.eventStream = broker
}

// SetFallbackBroker sets the fallback broker.
func (h *MessagingHub) SetFallbackBroker(broker MessageBroker) {
	h.fallback = broker
}

// GetTaskQueueBroker returns the task queue broker.
func (h *MessagingHub) GetTaskQueueBroker() TaskQueueBroker {
	return h.taskQueue
}

// GetEventStreamBroker returns the event stream broker.
func (h *MessagingHub) GetEventStreamBroker() EventStreamBroker {
	return h.eventStream
}

// GetFallbackBroker returns the fallback broker.
func (h *MessagingHub) GetFallbackBroker() MessageBroker {
	return h.fallback
}

// GetMessageBroker returns the appropriate broker for general messaging.
// Prefers event stream broker, falls back to fallback broker if event stream is not available.
func (h *MessagingHub) GetMessageBroker() MessageBroker {
	if h.eventStream != nil && h.eventStream.IsConnected() {
		return h.eventStream
	}
	if h.fallback != nil && h.fallback.IsConnected() {
		return h.fallback
	}
	return nil
}

// IsConnected returns true if the hub is connected.
func (h *MessagingHub) IsConnected() bool {
	return h.connected.Load()
}

// HealthCheck checks the health of all brokers.
func (h *MessagingHub) HealthCheck(ctx context.Context) error {
	var errs MultiError

	if h.taskQueue != nil && h.taskQueue.IsConnected() {
		if err := h.taskQueue.HealthCheck(ctx); err != nil {
			errs.Add(fmt.Errorf("task queue health check failed: %w", err))
		}
	}

	if h.eventStream != nil && h.eventStream.IsConnected() {
		if err := h.eventStream.HealthCheck(ctx); err != nil {
			errs.Add(fmt.Errorf("event stream health check failed: %w", err))
		}
	}

	return errs.ErrorOrNil()
}

// healthCheckLoop performs periodic health checks.
func (h *MessagingHub) healthCheckLoop() {
	ticker := time.NewTicker(h.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = h.HealthCheck(ctx) //nolint:errcheck
			cancel()
		case <-h.stopCh:
			return
		}
	}
}

// Task Queue Operations

// EnqueueTask adds a task to the queue.
func (h *MessagingHub) EnqueueTask(ctx context.Context, queue string, task *Task) error {
	if h.taskQueue != nil && h.taskQueue.IsConnected() {
		err := h.taskQueue.EnqueueTask(ctx, queue, task)
		if err == nil {
			h.metrics.RecordPublish(int64(len(task.Payload)), 0, true)
			return nil
		}
		h.metrics.RecordPublish(int64(len(task.Payload)), 0, false)

		// Fall back to in-memory if configured
		if h.config.UseFallbackOnError && h.fallback != nil {
			h.metrics.FallbackUsages.Add(1)
			return h.fallback.Publish(ctx, queue, task.ToMessage())
		}
		return err
	}

	// Use fallback
	if h.fallback != nil {
		h.metrics.FallbackUsages.Add(1)
		return h.fallback.Publish(ctx, queue, task.ToMessage())
	}

	return NewBrokerError(ErrCodeBrokerUnavailable, "no task queue broker available", nil)
}

// EnqueueTaskBatch adds multiple tasks to the queue.
func (h *MessagingHub) EnqueueTaskBatch(ctx context.Context, queue string, tasks []*Task) error {
	if h.taskQueue != nil && h.taskQueue.IsConnected() {
		return h.taskQueue.EnqueueTaskBatch(ctx, queue, tasks)
	}

	// Use fallback for batch
	if h.fallback != nil {
		h.metrics.FallbackUsages.Add(1)
		for _, task := range tasks {
			if err := h.fallback.Publish(ctx, queue, task.ToMessage()); err != nil {
				return err
			}
		}
		return nil
	}

	return NewBrokerError(ErrCodeBrokerUnavailable, "no task queue broker available", nil)
}

// SubscribeTasks subscribes to tasks from a queue.
func (h *MessagingHub) SubscribeTasks(ctx context.Context, queue string, handler TaskHandler, opts ...SubscribeOption) (Subscription, error) {
	// Wrap handler with middleware
	wrappedHandler := h.wrapTaskHandler(handler)

	var sub Subscription
	var err error

	if h.taskQueue != nil && h.taskQueue.IsConnected() {
		sub, err = h.taskQueue.SubscribeTasks(ctx, queue, wrappedHandler, opts...)
		if err == nil {
			h.trackSubscription(queue, sub)
			return sub, nil
		}
	}

	// Use fallback
	if h.fallback != nil {
		h.metrics.FallbackUsages.Add(1)
		sub, err = h.fallback.Subscribe(ctx, queue, func(ctx context.Context, msg *Message) error {
			task, err2 := TaskFromMessage(msg)
			if err2 != nil {
				return err2
			}
			return wrappedHandler(ctx, task)
		}, opts...)
		if err == nil {
			h.trackSubscription(queue, sub)
		}
		return sub, err
	}

	return nil, NewBrokerError(ErrCodeBrokerUnavailable, "no task queue broker available", nil)
}

// DeclareQueue declares a task queue.
func (h *MessagingHub) DeclareQueue(ctx context.Context, name string, opts ...QueueOption) error {
	if h.taskQueue != nil && h.taskQueue.IsConnected() {
		return h.taskQueue.DeclareQueue(ctx, name, opts...)
	}
	return nil // No-op for fallback
}

// GetQueueStats returns queue statistics.
func (h *MessagingHub) GetQueueStats(ctx context.Context, queue string) (*QueueStats, error) {
	if h.taskQueue != nil && h.taskQueue.IsConnected() {
		return h.taskQueue.GetQueueStats(ctx, queue)
	}
	return nil, NewBrokerError(ErrCodeBrokerUnavailable, "no task queue broker available", nil)
}

// Event Stream Operations

// PublishEvent publishes an event to a topic.
func (h *MessagingHub) PublishEvent(ctx context.Context, topic string, event *Event) error {
	if h.eventStream != nil && h.eventStream.IsConnected() {
		err := h.eventStream.PublishEvent(ctx, topic, event)
		if err == nil {
			h.metrics.RecordPublish(int64(len(event.Data)), 0, true)
			return nil
		}
		h.metrics.RecordPublish(int64(len(event.Data)), 0, false)

		// Fall back to in-memory if configured
		if h.config.UseFallbackOnError && h.fallback != nil {
			h.metrics.FallbackUsages.Add(1)
			return h.fallback.Publish(ctx, topic, event.ToMessage())
		}
		return err
	}

	// Use fallback
	if h.fallback != nil {
		h.metrics.FallbackUsages.Add(1)
		return h.fallback.Publish(ctx, topic, event.ToMessage())
	}

	return NewBrokerError(ErrCodeBrokerUnavailable, "no event stream broker available", nil)
}

// PublishEventBatch publishes multiple events to a topic.
func (h *MessagingHub) PublishEventBatch(ctx context.Context, topic string, events []*Event) error {
	if h.eventStream != nil && h.eventStream.IsConnected() {
		return h.eventStream.PublishEventBatch(ctx, topic, events)
	}

	// Use fallback for batch
	if h.fallback != nil {
		h.metrics.FallbackUsages.Add(1)
		for _, event := range events {
			if err := h.fallback.Publish(ctx, topic, event.ToMessage()); err != nil {
				return err
			}
		}
		return nil
	}

	return NewBrokerError(ErrCodeBrokerUnavailable, "no event stream broker available", nil)
}

// SubscribeEvents subscribes to events from a topic.
func (h *MessagingHub) SubscribeEvents(ctx context.Context, topic string, handler EventHandler, opts ...SubscribeOption) (Subscription, error) {
	// Wrap handler with middleware
	wrappedHandler := h.wrapEventHandler(handler)

	var sub Subscription
	var err error

	if h.eventStream != nil && h.eventStream.IsConnected() {
		sub, err = h.eventStream.SubscribeEvents(ctx, topic, wrappedHandler, opts...)
		if err == nil {
			h.trackSubscription(topic, sub)
			return sub, nil
		}
	}

	// Use fallback
	if h.fallback != nil {
		h.metrics.FallbackUsages.Add(1)
		sub, err = h.fallback.Subscribe(ctx, topic, func(ctx context.Context, msg *Message) error {
			event, err2 := EventFromMessage(msg)
			if err2 != nil {
				return err2
			}
			return wrappedHandler(ctx, event)
		}, opts...)
		if err == nil {
			h.trackSubscription(topic, sub)
		}
		return sub, err
	}

	return nil, NewBrokerError(ErrCodeBrokerUnavailable, "no event stream broker available", nil)
}

// StreamEvents returns a channel of events from a topic.
func (h *MessagingHub) StreamEvents(ctx context.Context, topic string, opts ...StreamOption) (<-chan *Event, error) {
	if h.eventStream != nil && h.eventStream.IsConnected() {
		return h.eventStream.StreamEvents(ctx, topic, opts...)
	}
	return nil, NewBrokerError(ErrCodeBrokerUnavailable, "no event stream broker available", nil)
}

// CreateTopic creates a new topic.
func (h *MessagingHub) CreateTopic(ctx context.Context, name string, partitions, replication int) error {
	if h.eventStream != nil && h.eventStream.IsConnected() {
		return h.eventStream.CreateTopic(ctx, name, partitions, replication)
	}
	return nil // No-op for fallback
}

// GetTopicMetadata returns metadata for a topic.
func (h *MessagingHub) GetTopicMetadata(ctx context.Context, topic string) (*TopicMetadata, error) {
	if h.eventStream != nil && h.eventStream.IsConnected() {
		return h.eventStream.GetTopicMetadata(ctx, topic)
	}
	return nil, NewBrokerError(ErrCodeBrokerUnavailable, "no event stream broker available", nil)
}

// Generic Operations

// Publish publishes a message to a topic.
func (h *MessagingHub) Publish(ctx context.Context, topic string, message *Message) error {
	// Route to appropriate broker
	if h.router.IsTaskQueue(topic) {
		task, err := TaskFromMessage(message)
		if err != nil {
			return err
		}
		return h.EnqueueTask(ctx, topic, task)
	}

	if h.router.IsEventStream(topic) {
		event, err := EventFromMessage(message)
		if err != nil {
			return err
		}
		return h.PublishEvent(ctx, topic, event)
	}

	// Default to event stream or fallback
	if h.eventStream != nil && h.eventStream.IsConnected() {
		return h.eventStream.Publish(ctx, topic, message)
	}
	if h.fallback != nil {
		return h.fallback.Publish(ctx, topic, message)
	}

	return NewBrokerError(ErrCodeBrokerUnavailable, "no broker available", nil)
}

// Subscribe subscribes to messages from a topic.
func (h *MessagingHub) Subscribe(ctx context.Context, topic string, handler MessageHandler, opts ...SubscribeOption) (Subscription, error) {
	wrappedHandler := h.middleware.Wrap(handler)

	// Route to appropriate broker
	if h.router.IsTaskQueue(topic) {
		if h.taskQueue != nil && h.taskQueue.IsConnected() {
			return h.taskQueue.Subscribe(ctx, topic, wrappedHandler, opts...)
		}
	}

	if h.router.IsEventStream(topic) {
		if h.eventStream != nil && h.eventStream.IsConnected() {
			return h.eventStream.Subscribe(ctx, topic, wrappedHandler, opts...)
		}
	}

	// Use fallback
	if h.fallback != nil {
		return h.fallback.Subscribe(ctx, topic, wrappedHandler, opts...)
	}

	return nil, NewBrokerError(ErrCodeBrokerUnavailable, "no broker available", nil)
}

// Registry Operations

// RegisterTaskHandler registers a handler for a task type.
func (h *MessagingHub) RegisterTaskHandler(taskType string, handler TaskHandler) {
	h.taskRegistry.Register(taskType, handler)
}

// RegisterEventHandler registers a handler for an event type.
func (h *MessagingHub) RegisterEventHandler(eventType EventType, handler EventHandler) {
	h.eventRegistry.Register(eventType, handler)
}

// Middleware Operations

// Use adds middleware to the hub.
func (h *MessagingHub) Use(middleware ...MessageMiddleware) {
	h.middleware.Add(middleware...)
}

// Metrics Operations

// GetMetrics returns hub metrics.
func (h *MessagingHub) GetMetrics() *HubMetrics {
	return h.metrics
}

// Helper methods

func (h *MessagingHub) trackSubscription(topic string, sub Subscription) {
	h.subscriptions.Put(topic, sub)
	h.metrics.RecordSubscription()
}

func (h *MessagingHub) wrapTaskHandler(handler TaskHandler) TaskHandler {
	return func(ctx context.Context, task *Task) error {
		// Apply any task-specific middleware here
		start := time.Now()
		err := handler(ctx, task)
		duration := time.Since(start)

		h.metrics.RecordReceive(int64(len(task.Payload)), duration)
		if err != nil {
			h.metrics.RecordFailed()
		} else {
			h.metrics.RecordProcessed()
		}

		return err
	}
}

func (h *MessagingHub) wrapEventHandler(handler EventHandler) EventHandler {
	return func(ctx context.Context, event *Event) error {
		// Apply any event-specific middleware here
		start := time.Now()
		err := handler(ctx, event)
		duration := time.Since(start)

		h.metrics.RecordReceive(int64(len(event.Data)), duration)
		if err != nil {
			h.metrics.RecordFailed()
		} else {
			h.metrics.RecordProcessed()
		}

		return err
	}
}

// MessageRouter routes messages to appropriate brokers.
type MessageRouter struct {
	taskQueuePrefixes   []string
	eventStreamPrefixes []string
}

// NewMessageRouter creates a new message router.
func NewMessageRouter() *MessageRouter {
	return &MessageRouter{
		taskQueuePrefixes: []string{
			"helixagent.tasks.",
			"tasks.",
		},
		eventStreamPrefixes: []string{
			"helixagent.events.",
			"helixagent.stream.",
			"events.",
		},
	}
}

// IsTaskQueue returns true if the topic is a task queue.
func (r *MessageRouter) IsTaskQueue(topic string) bool {
	for _, prefix := range r.taskQueuePrefixes {
		if len(topic) >= len(prefix) && topic[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// IsEventStream returns true if the topic is an event stream.
func (r *MessageRouter) IsEventStream(topic string) bool {
	for _, prefix := range r.eventStreamPrefixes {
		if len(topic) >= len(prefix) && topic[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// AddTaskQueuePrefix adds a task queue prefix.
func (r *MessageRouter) AddTaskQueuePrefix(prefix string) {
	r.taskQueuePrefixes = append(r.taskQueuePrefixes, prefix)
}

// AddEventStreamPrefix adds an event stream prefix.
func (r *MessageRouter) AddEventStreamPrefix(prefix string) {
	r.eventStreamPrefixes = append(r.eventStreamPrefixes, prefix)
}

// Global hub instance for convenience.
var globalHub *MessagingHub
var globalHubMu sync.RWMutex

// SetGlobalHub sets the global messaging hub.
func SetGlobalHub(hub *MessagingHub) {
	globalHubMu.Lock()
	defer globalHubMu.Unlock()
	globalHub = hub
}

// GetGlobalHub returns the global messaging hub.
func GetGlobalHub() *MessagingHub {
	globalHubMu.RLock()
	defer globalHubMu.RUnlock()
	return globalHub
}
