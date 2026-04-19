// Package inmemory provides an in-memory message broker implementation
// for testing and development purposes.
package inmemory

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/messaging"
)

// Broker is an in-memory message broker implementation.
//
// Concurrent-safe by construction (CONST-029):
//
//   - queues / topics / subscribers / notifyCh are safe.Store
//     containers; each individual map lookup, insert, and delete is
//     atomic without any caller-held lock.
//   - connected is an atomic.Bool.
//   - regMu (sync.Mutex) is a Pattern Zeta scalar survivor. It
//     serialises multi-store compound operations that must stay
//     atomic across keys — specifically Publish's "is there a queue
//     OR a topic for this name? if not, create a queue" sequence,
//     Subscribe's "ensure queue AND notifyCh exist" sequence, and
//     Close's "flip connected then signal stopCh then clear every
//     store" teardown. Because no bare map/slice sits beside regMu,
//     the audit is happy.
type Broker struct {
	regMu       sync.Mutex
	queues      *safe.Store[string, *Queue]
	topics      *safe.Store[string, *Topic]
	metrics     *messaging.BrokerMetrics
	connected   atomic.Bool
	stopCh      chan struct{}
	config      *Config
	subscribers *safe.Store[string, []subscriberEntry]
	notifyCh    *safe.Store[string, chan struct{}] // Per-topic notification channels
	wg          sync.WaitGroup                     // Tracks consumeLoop goroutines
}

// subscriberEntry holds a subscriber and its options.
type subscriberEntry struct {
	handler messaging.MessageHandler
	opts    *messaging.SubscribeOptions
	active  bool
	subID   string
}

// Config holds configuration for the in-memory broker.
type Config struct {
	// DefaultQueueCapacity is the default capacity for queues.
	DefaultQueueCapacity int
	// DefaultTopicCapacity is the default capacity for topics.
	DefaultTopicCapacity int
	// MessageTTL is the default message time-to-live.
	MessageTTL time.Duration
	// ProcessingDelay adds artificial delay for testing.
	ProcessingDelay time.Duration
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		DefaultQueueCapacity: 10000,
		DefaultTopicCapacity: 10000,
		MessageTTL:           24 * time.Hour,
		ProcessingDelay:      0,
	}
}

// NewBroker creates a new in-memory broker.
func NewBroker(config *Config) *Broker {
	if config == nil {
		config = DefaultConfig()
	}
	return &Broker{
		queues:      safe.NewStore[string, *Queue](),
		topics:      safe.NewStore[string, *Topic](),
		metrics:     messaging.NewBrokerMetrics(),
		config:      config,
		subscribers: safe.NewStore[string, []subscriberEntry](),
		notifyCh:    safe.NewStore[string, chan struct{}](),
		stopCh:      make(chan struct{}),
	}
}

// Connect establishes a connection (no-op for in-memory).
func (b *Broker) Connect(ctx context.Context) error {
	b.metrics.RecordConnectionAttempt()
	b.connected.Store(true)
	b.metrics.RecordConnectionSuccess()
	return nil
}

// Close closes the broker.
func (b *Broker) Close(ctx context.Context) error {
	// Flip connected → false under regMu so no new Publish/Subscribe
	// races past the guard after we've already started shutdown.
	b.regMu.Lock()
	if !b.connected.Load() {
		b.regMu.Unlock()
		return nil
	}
	close(b.stopCh)
	b.connected.Store(false)
	b.metrics.RecordDisconnection()
	b.regMu.Unlock()

	// Wait for all consumeLoop goroutines to finish before clearing
	// the stores — consumers still referring to notifyCh entries must
	// wake and return on stopCh first.
	b.wg.Wait()

	b.regMu.Lock()
	b.queues.Clear()
	b.topics.Clear()
	b.subscribers.Clear()
	b.notifyCh.Clear()
	b.regMu.Unlock()

	return nil
}

// HealthCheck checks if the broker is healthy.
func (b *Broker) HealthCheck(ctx context.Context) error {
	if !b.connected.Load() {
		return messaging.ErrNotConnected
	}
	return nil
}

// IsConnected returns true if connected.
func (b *Broker) IsConnected() bool {
	return b.connected.Load()
}

// Publish publishes a message to a topic or queue.
//
// The queue-vs-topic-vs-create check-create chain runs under regMu so
// two concurrent Publish calls for the same name can't both decide to
// "create a new queue on demand."
func (b *Broker) Publish(ctx context.Context, topic string, message *messaging.Message, opts ...messaging.PublishOption) error {
	b.regMu.Lock()
	defer b.regMu.Unlock()

	if !b.connected.Load() {
		return messaging.ErrNotConnected
	}

	start := time.Now()

	// Add processing delay if configured
	if b.config.ProcessingDelay > 0 {
		time.Sleep(b.config.ProcessingDelay)
	}

	// Clone message to prevent modifications
	msg := message.Clone()
	msg.Timestamp = time.Now().UTC()

	// Try to deliver to queue first (for subscriptions with consumeLoop)
	if queue, ok := b.queues.Get(topic); ok {
		if err := queue.Enqueue(msg); err != nil {
			b.metrics.RecordPublish(int64(len(msg.Payload)), time.Since(start), false)
			return err
		}
		// Signal waiting consumers that a message is available
		if ch, exists := b.notifyCh.Get(topic); exists {
			select {
			case ch <- struct{}{}:
			default: // Non-blocking - channel may be full or no receivers
			}
		}
		b.metrics.RecordPublish(int64(len(msg.Payload)), time.Since(start), true)
		return nil
	}

	// Try to deliver to topic (direct pub/sub without queue)
	if topicObj, ok := b.topics.Get(topic); ok {
		if err := topicObj.Publish(msg); err != nil {
			b.metrics.RecordPublish(int64(len(msg.Payload)), time.Since(start), false)
			return err
		}
		// Notify subscribers for topic-based delivery
		b.notifySubscribers(ctx, topic, msg)
		b.metrics.RecordPublish(int64(len(msg.Payload)), time.Since(start), true)
		return nil
	}

	// Create queue/topic on demand
	queue := NewQueue(topic, b.config.DefaultQueueCapacity)
	b.queues.Put(topic, queue)
	if err := queue.Enqueue(msg); err != nil {
		b.metrics.RecordPublish(int64(len(msg.Payload)), time.Since(start), false)
		return err
	}

	// Notify subscribers
	b.notifySubscribers(ctx, topic, msg)

	b.metrics.RecordPublish(int64(len(msg.Payload)), time.Since(start), true)
	return nil
}

// PublishBatch publishes multiple messages.
func (b *Broker) PublishBatch(ctx context.Context, topic string, messages []*messaging.Message, opts ...messaging.PublishOption) error {
	for _, msg := range messages {
		if err := b.Publish(ctx, topic, msg, opts...); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe creates a subscription to a topic or queue.
//
// regMu serialises the compound "append subscriber + ensure queue +
// ensure notifyCh" sequence so two concurrent Subscribe calls for the
// same topic can't both race to create the queue / notifyCh.
func (b *Broker) Subscribe(ctx context.Context, topic string, handler messaging.MessageHandler, opts ...messaging.SubscribeOption) (messaging.Subscription, error) {
	b.regMu.Lock()
	defer b.regMu.Unlock()

	if !b.connected.Load() {
		return nil, messaging.ErrNotConnected
	}

	options := messaging.ApplySubscribeOptions(opts...)
	subID := generateSubscriptionID()

	entry := subscriberEntry{
		handler: handler,
		opts:    options,
		active:  true,
		subID:   subID,
	}

	b.subscribers.Update(topic, func(cur []subscriberEntry, _ bool) ([]subscriberEntry, bool) {
		return append(cur, entry), true
	})

	// Ensure queue exists
	if _, ok := b.queues.Get(topic); !ok {
		b.queues.Put(topic, NewQueue(topic, b.config.DefaultQueueCapacity))
	}

	// Create notification channel if it doesn't exist
	if _, ok := b.notifyCh.Get(topic); !ok {
		b.notifyCh.Put(topic, make(chan struct{}, 100)) // Buffered to avoid blocking publishers
	}

	b.metrics.RecordSubscription()

	sub := &Subscription{
		id:     subID,
		topic:  topic,
		broker: b,
		active: true,
	}

	// Start consuming in background with WaitGroup tracking
	b.wg.Add(1)
	go b.consumeLoop(ctx, topic, entry, sub)

	return sub, nil
}

// consumeLoop continuously consumes messages from a queue.
func (b *Broker) consumeLoop(ctx context.Context, topic string, entry subscriberEntry, sub *Subscription) {
	defer b.wg.Done()

	// Get the notification channel for this topic — set at Subscribe
	// time, never rewritten (only cleared on Close), so one Get is
	// enough for the lifetime of the loop.
	notifyCh, _ := b.notifyCh.Get(topic)

	for {
		if !sub.IsActive() {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		case <-notifyCh:
			// Message available notification received
		case <-time.After(10 * time.Millisecond):
			// Fallback timeout for polling
		}

		if !sub.IsActive() {
			return
		}

		queue, ok := b.queues.Get(topic)
		if !ok {
			continue
		}

		// Process all available messages
		for {
			msg, err := queue.Dequeue()
			if err != nil || msg == nil {
				break
			}

			// Apply filter if set
			if entry.opts.Filter != nil && !entry.opts.Filter(msg) {
				continue
			}

			start := time.Now()
			err = entry.handler(ctx, msg)
			duration := time.Since(start)

			b.metrics.RecordReceive(int64(len(msg.Payload)), duration)
			if err != nil {
				b.metrics.RecordFailed()
				// Requeue if retry is enabled. Queue.Enqueue is
				// concurrency-safe internally; no need to take a
				// broker-wide lock around it.
				if entry.opts.RetryOnError && msg.CanRetry() {
					msg.IncrementRetry()
					_ = queue.Enqueue(msg) //nolint:errcheck
					b.metrics.RecordRetry()
				}
			} else {
				b.metrics.RecordProcessed()
			}
		}
	}
}

// notifySubscribers notifies all subscribers of a new message.
func (b *Broker) notifySubscribers(ctx context.Context, topic string, msg *messaging.Message) {
	subscribers, _ := b.subscribers.Get(topic)
	for _, sub := range subscribers {
		if sub.active {
			go func(s subscriberEntry) {
				_ = s.handler(ctx, msg) //nolint:errcheck
			}(sub)
		}
	}
}

// BrokerType returns the broker type.
func (b *Broker) BrokerType() messaging.BrokerType {
	return messaging.BrokerTypeInMemory
}

// GetMetrics returns broker metrics.
func (b *Broker) GetMetrics() *messaging.BrokerMetrics {
	return b.metrics
}

// Queue-specific methods for TaskQueueBroker interface

// DeclareQueue declares a queue.
func (b *Broker) DeclareQueue(ctx context.Context, name string, opts ...messaging.QueueOption) error {
	if !b.connected.Load() {
		return messaging.ErrNotConnected
	}

	queue := NewQueue(name, b.config.DefaultQueueCapacity)
	if _, stored := b.queues.PutIfAbsent(name, queue); stored {
		b.metrics.RecordQueueDeclared()
	}

	return nil
}

// EnqueueTask adds a task to a queue.
func (b *Broker) EnqueueTask(ctx context.Context, queue string, task *messaging.Task) error {
	return b.Publish(ctx, queue, task.ToMessage())
}

// EnqueueTaskBatch adds multiple tasks to a queue.
func (b *Broker) EnqueueTaskBatch(ctx context.Context, queue string, tasks []*messaging.Task) error {
	for _, task := range tasks {
		if err := b.EnqueueTask(ctx, queue, task); err != nil {
			return err
		}
	}
	return nil
}

// DequeueTask retrieves a task from a queue.
func (b *Broker) DequeueTask(ctx context.Context, queue string, workerID string) (*messaging.Task, error) {
	if !b.connected.Load() {
		return nil, messaging.ErrNotConnected
	}

	q, ok := b.queues.Get(queue)
	if !ok {
		return nil, messaging.ErrQueueNotFound
	}

	msg, err := q.Dequeue()
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, nil
	}

	return messaging.TaskFromMessage(msg)
}

// AckTask acknowledges a task.
func (b *Broker) AckTask(ctx context.Context, deliveryTag uint64) error {
	// No-op for in-memory (messages are removed on dequeue)
	return nil
}

// NackTask negatively acknowledges a task.
func (b *Broker) NackTask(ctx context.Context, deliveryTag uint64, requeue bool) error {
	// No-op for in-memory
	return nil
}

// RejectTask rejects a task.
func (b *Broker) RejectTask(ctx context.Context, deliveryTag uint64) error {
	// No-op for in-memory
	return nil
}

// MoveToDeadLetter moves a task to dead letter queue.
func (b *Broker) MoveToDeadLetter(ctx context.Context, task *messaging.Task, reason string) error {
	task.SetError(reason)
	task.SetState(messaging.TaskStateDeadLettered)
	return b.EnqueueTask(ctx, messaging.QueueDeadLetter, task)
}

// GetQueueStats returns queue statistics.
func (b *Broker) GetQueueStats(ctx context.Context, queue string) (*messaging.QueueStats, error) {
	q, ok := b.queues.Get(queue)
	if !ok {
		return nil, messaging.ErrQueueNotFound
	}
	return &messaging.QueueStats{
		Name:     queue,
		Messages: int64(q.Len()),
	}, nil
}

// GetQueueDepth returns the number of messages in a queue.
func (b *Broker) GetQueueDepth(ctx context.Context, queue string) (int64, error) {
	q, ok := b.queues.Get(queue)
	if !ok {
		return 0, messaging.ErrQueueNotFound
	}
	return int64(q.Len()), nil
}

// PurgeQueue removes all messages from a queue.
func (b *Broker) PurgeQueue(ctx context.Context, queue string) error {
	if q, ok := b.queues.Get(queue); ok {
		q.Clear()
	}
	return nil
}

// DeleteQueue deletes a queue.
func (b *Broker) DeleteQueue(ctx context.Context, queue string) error {
	b.queues.Delete(queue)
	return nil
}

// SubscribeTasks subscribes to tasks from a queue.
func (b *Broker) SubscribeTasks(ctx context.Context, queue string, handler messaging.TaskHandler, opts ...messaging.SubscribeOption) (messaging.Subscription, error) {
	return b.Subscribe(ctx, queue, func(ctx context.Context, msg *messaging.Message) error {
		task, err := messaging.TaskFromMessage(msg)
		if err != nil {
			return err
		}
		return handler(ctx, task)
	}, opts...)
}

// Subscription is an in-memory subscription implementation.
type Subscription struct {
	id     string
	topic  string
	broker *Broker
	active bool
	mu     sync.RWMutex
}

// Unsubscribe cancels the subscription.
func (s *Subscription) Unsubscribe() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return nil
	}

	s.active = false
	s.broker.metrics.RecordUnsubscription()

	// Remove from broker's subscribers. Store.Update keeps the
	// read-modify-write atomic against concurrent Subscribe/Unsubscribe
	// for the same topic.
	s.broker.subscribers.Update(s.topic, func(cur []subscriberEntry, _ bool) ([]subscriberEntry, bool) {
		for i, sub := range cur {
			if sub.subID == s.id {
				next := make([]subscriberEntry, 0, len(cur)-1)
				next = append(next, cur[:i]...)
				next = append(next, cur[i+1:]...)
				if len(next) == 0 {
					return nil, false // drop the empty entry
				}
				return next, true
			}
		}
		return cur, true
	})

	return nil
}

// IsActive returns true if the subscription is active.
func (s *Subscription) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

// Topic returns the subscribed topic.
func (s *Subscription) Topic() string {
	return s.topic
}

// ID returns the subscription ID.
func (s *Subscription) ID() string {
	return s.id
}

// generateSubscriptionID generates a unique subscription ID.
func generateSubscriptionID() string {
	return "sub-" + time.Now().UTC().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random alphanumeric string.
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
