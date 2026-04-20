package inmemory

import (
	"context"
	"sync"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/messaging"
)

// Topic is an in-memory topic implementation for event streaming.
//
// Concurrency model (CONST-029): partitions is set at construction
// and never mutated (Pattern Alpha). subscribers is a *safe.Slice —
// Subscribe appends, Unsubscribe finds-and-deletes via predicate,
// and Publish iterates via Range. No mu survives.
type Topic struct {
	name        string
	partitions  []*Partition
	subscribers *safe.Slice[topicSubscriber]
	retention   time.Duration
}

// topicSubscriber holds a topic subscriber.
type topicSubscriber struct {
	handler messaging.EventHandler
	groupID string
	active  bool
	ch      chan *messaging.Event
}

// NewTopic creates a new in-memory topic.
func NewTopic(name string, partitionCount int, retention time.Duration) *Topic {
	partitions := make([]*Partition, partitionCount)
	for i := 0; i < partitionCount; i++ {
		partitions[i] = NewPartition(i, 10000)
	}
	return &Topic{
		name:        name,
		partitions:  partitions,
		subscribers: safe.NewSlice[topicSubscriber](),
		retention:   retention,
	}
}

// Name returns the topic name.
func (t *Topic) Name() string {
	return t.name
}

// PartitionCount returns the number of partitions.
func (t *Topic) PartitionCount() int {
	return len(t.partitions)
}

// Publish publishes a message to the topic.
func (t *Topic) Publish(msg *messaging.Message) error {
	// Determine partition based on key or round-robin
	partitionID := t.selectPartition(msg)
	partition := t.partitions[partitionID]

	offset, err := partition.Append(msg)
	if err != nil {
		return err
	}

	msg.Partition = int32(partitionID) // #nosec G115 - partition ID fits in int32
	msg.Offset = offset

	// Notify subscribers
	t.subscribers.Range(func(_ int, sub topicSubscriber) bool {
		if sub.active && sub.ch != nil {
			event, _ := messaging.EventFromMessage(msg) //nolint:errcheck
			select {
			case sub.ch <- event:
			default:
				// Channel full, skip
			}
		}
		return true
	})

	return nil
}

// PublishEvent publishes an event to the topic.
func (t *Topic) PublishEvent(event *messaging.Event) error {
	return t.Publish(event.ToMessage())
}

// selectPartition selects a partition for a message.
func (t *Topic) selectPartition(msg *messaging.Message) int {
	if msg.Partition >= 0 && int(msg.Partition) < len(t.partitions) {
		return int(msg.Partition)
	}
	// Simple hash-based partitioning on message ID
	hash := 0
	for _, c := range msg.ID {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash % len(t.partitions)
}

// Subscribe subscribes to events from the topic.
func (t *Topic) Subscribe(handler messaging.EventHandler, groupID string) chan *messaging.Event {
	ch := make(chan *messaging.Event, 1000)
	t.subscribers.Append(topicSubscriber{
		handler: handler,
		groupID: groupID,
		active:  true,
		ch:      ch,
	})

	return ch
}

// Unsubscribe removes a subscriber.
func (t *Topic) Unsubscribe(groupID string) {
	removed, ok := t.subscribers.Delete(func(sub topicSubscriber) bool {
		return sub.groupID == groupID
	})
	if ok {
		close(removed.ch)
	}
}

// Read reads messages from a partition starting at an offset.
func (t *Topic) Read(partitionID int, offset int64, maxCount int) ([]*messaging.Message, error) {
	if partitionID < 0 || partitionID >= len(t.partitions) {
		return nil, messaging.NewBrokerError(messaging.ErrCodeInvalidConfig, "invalid partition", nil)
	}

	return t.partitions[partitionID].Read(offset, maxCount)
}

// GetHighWatermark returns the high watermark for a partition.
func (t *Topic) GetHighWatermark(partitionID int) int64 {
	if partitionID < 0 || partitionID >= len(t.partitions) {
		return 0
	}

	return t.partitions[partitionID].HighWatermark()
}

// GetLowWatermark returns the low watermark for a partition.
func (t *Topic) GetLowWatermark(partitionID int) int64 {
	if partitionID < 0 || partitionID >= len(t.partitions) {
		return 0
	}

	return t.partitions[partitionID].LowWatermark()
}

// Partition is an in-memory partition implementation.
type Partition struct {
	id            int
	messages      []*messaging.Message
	capacity      int
	lowWatermark  int64
	highWatermark int64
	mu            sync.RWMutex
}

// NewPartition creates a new partition.
func NewPartition(id int, capacity int) *Partition {
	return &Partition{
		id:            id,
		messages:      make([]*messaging.Message, 0, capacity),
		capacity:      capacity,
		lowWatermark:  0,
		highWatermark: 0,
	}
}

// ID returns the partition ID.
func (p *Partition) ID() int {
	return p.id
}

// Append appends a message to the partition.
func (p *Partition) Append(msg *messaging.Message) (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Compact if at capacity
	if len(p.messages) >= p.capacity {
		// Remove oldest half
		half := len(p.messages) / 2
		p.messages = p.messages[half:]
		p.lowWatermark += int64(half)
	}

	offset := p.highWatermark
	msg.Offset = offset
	msg.Partition = int32(p.id) // #nosec G115 - partition ID fits in int32
	p.messages = append(p.messages, msg)
	p.highWatermark++

	return offset, nil
}

// Read reads messages starting at an offset.
func (p *Partition) Read(offset int64, maxCount int) ([]*messaging.Message, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if offset < p.lowWatermark {
		offset = p.lowWatermark
	}

	if offset >= p.highWatermark {
		return []*messaging.Message{}, nil
	}

	startIndex := int(offset - p.lowWatermark)
	if startIndex < 0 {
		startIndex = 0
	}

	endIndex := startIndex + maxCount
	if endIndex > len(p.messages) {
		endIndex = len(p.messages)
	}

	result := make([]*messaging.Message, endIndex-startIndex)
	copy(result, p.messages[startIndex:endIndex])
	return result, nil
}

// HighWatermark returns the high watermark offset.
func (p *Partition) HighWatermark() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.highWatermark
}

// LowWatermark returns the low watermark offset.
func (p *Partition) LowWatermark() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lowWatermark
}

// Len returns the number of messages in the partition.
func (p *Partition) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.messages)
}

// StreamBroker is an in-memory event stream broker implementation.
type StreamBroker struct {
	topics    map[string]*Topic
	metrics   *messaging.BrokerMetrics
	connected bool
	mu        sync.RWMutex
	stopCh    chan struct{}
	config    *StreamConfig
	offsets   map[string]map[int32]int64 // groupID -> partition -> offset
	offsetsMu sync.RWMutex
}

// StreamConfig holds configuration for the stream broker.
type StreamConfig struct {
	DefaultPartitions int
	DefaultRetention  time.Duration
	MaxMessageSize    int
	EnableCompression bool
}

// DefaultStreamConfig returns the default configuration.
func DefaultStreamConfig() *StreamConfig {
	return &StreamConfig{
		DefaultPartitions: 3,
		DefaultRetention:  7 * 24 * time.Hour,
		MaxMessageSize:    1024 * 1024,
		EnableCompression: false,
	}
}

// NewStreamBroker creates a new in-memory stream broker.
func NewStreamBroker(config *StreamConfig) *StreamBroker {
	if config == nil {
		config = DefaultStreamConfig()
	}
	return &StreamBroker{
		topics:  make(map[string]*Topic),
		metrics: messaging.NewBrokerMetrics(),
		config:  config,
		stopCh:  make(chan struct{}),
		offsets: make(map[string]map[int32]int64),
	}
}

// Connect establishes a connection.
func (b *StreamBroker) Connect(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.metrics.RecordConnectionAttempt()
	b.connected = true
	b.metrics.RecordConnectionSuccess()

	return nil
}

// Close closes the broker.
func (b *StreamBroker) Close(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.connected {
		return nil
	}

	close(b.stopCh)
	b.connected = false
	b.metrics.RecordDisconnection()

	// Close all topic subscribers
	for _, topic := range b.topics {
		topic.subscribers.Range(func(_ int, sub topicSubscriber) bool {
			if sub.ch != nil {
				close(sub.ch)
			}
			return true
		})
	}

	b.topics = make(map[string]*Topic)
	return nil
}

// HealthCheck checks if the broker is healthy.
func (b *StreamBroker) HealthCheck(ctx context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.connected {
		return messaging.ErrNotConnected
	}
	return nil
}

// IsConnected returns true if connected.
func (b *StreamBroker) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}

// Publish publishes a message to a topic.
func (b *StreamBroker) Publish(ctx context.Context, topicName string, message *messaging.Message, opts ...messaging.PublishOption) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.connected {
		return messaging.ErrNotConnected
	}

	topic, ok := b.topics[topicName]
	if !ok {
		topic = NewTopic(topicName, b.config.DefaultPartitions, b.config.DefaultRetention)
		b.topics[topicName] = topic
		b.metrics.RecordTopicCreated()
	}

	start := time.Now()
	err := topic.Publish(message)
	duration := time.Since(start)

	b.metrics.RecordPublish(int64(len(message.Payload)), duration, err == nil)
	return err
}

// PublishBatch publishes multiple messages.
func (b *StreamBroker) PublishBatch(ctx context.Context, topicName string, messages []*messaging.Message, opts ...messaging.PublishOption) error {
	for _, msg := range messages {
		if err := b.Publish(ctx, topicName, msg, opts...); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe creates a subscription to a topic.
func (b *StreamBroker) Subscribe(ctx context.Context, topicName string, handler messaging.MessageHandler, opts ...messaging.SubscribeOption) (messaging.Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.connected {
		return nil, messaging.ErrNotConnected
	}

	options := messaging.ApplySubscribeOptions(opts...)

	topic, ok := b.topics[topicName]
	if !ok {
		topic = NewTopic(topicName, b.config.DefaultPartitions, b.config.DefaultRetention)
		b.topics[topicName] = topic
	}

	eventHandler := func(ctx context.Context, event *messaging.Event) error {
		return handler(ctx, event.ToMessage())
	}

	ch := topic.Subscribe(eventHandler, options.GroupID)
	b.metrics.RecordSubscription()

	sub := &StreamSubscription{
		id:      generateSubscriptionID(),
		topic:   topicName,
		broker:  b,
		ch:      ch,
		active:  true,
		groupID: options.GroupID,
	}

	// Start consumer goroutine
	go sub.consumeLoop(ctx, handler)

	return sub, nil
}

// BrokerType returns the broker type.
func (b *StreamBroker) BrokerType() messaging.BrokerType {
	return messaging.BrokerTypeInMemory
}

// GetMetrics returns broker metrics.
func (b *StreamBroker) GetMetrics() *messaging.BrokerMetrics {
	return b.metrics
}

// CreateTopic creates a new topic.
func (b *StreamBroker) CreateTopic(ctx context.Context, name string, partitions int, replication int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.topics[name]; ok {
		return nil // Topic already exists
	}

	b.topics[name] = NewTopic(name, partitions, b.config.DefaultRetention)
	b.metrics.RecordTopicCreated()
	return nil
}

// DeleteTopic deletes a topic.
func (b *StreamBroker) DeleteTopic(ctx context.Context, name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.topics, name)
	return nil
}

// ListTopics lists all topics.
func (b *StreamBroker) ListTopics(ctx context.Context) ([]string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topics := make([]string, 0, len(b.topics))
	for name := range b.topics {
		topics = append(topics, name)
	}
	return topics, nil
}

// GetTopicMetadata returns metadata for a topic.
func (b *StreamBroker) GetTopicMetadata(ctx context.Context, topicName string) (*messaging.TopicMetadata, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topic, ok := b.topics[topicName]
	if !ok {
		return nil, messaging.ErrTopicNotFound
	}

	partitionInfo := make([]messaging.PartitionInfo, topic.PartitionCount())
	for i := 0; i < topic.PartitionCount(); i++ {
		partitionInfo[i] = messaging.PartitionInfo{
			ID:            int32(i), // #nosec G115 - partition index fits in int32
			Leader:        0,
			Replicas:      []int32{0},
			ISR:           []int32{0},
			HighWatermark: topic.GetHighWatermark(i),
			LowWatermark:  topic.GetLowWatermark(i),
		}
	}

	return &messaging.TopicMetadata{
		Name:              topicName,
		Partitions:        topic.PartitionCount(),
		ReplicationFactor: 1,
		RetentionMs:       int64(b.config.DefaultRetention.Milliseconds()),
		CleanupPolicy:     "delete",
		PartitionInfo:     partitionInfo,
		Timestamp:         time.Now().UTC(),
	}, nil
}

// PublishEvent publishes an event to a topic.
func (b *StreamBroker) PublishEvent(ctx context.Context, topicName string, event *messaging.Event) error {
	return b.Publish(ctx, topicName, event.ToMessage())
}

// PublishEventBatch publishes multiple events to a topic.
func (b *StreamBroker) PublishEventBatch(ctx context.Context, topicName string, events []*messaging.Event) error {
	for _, event := range events {
		if err := b.PublishEvent(ctx, topicName, event); err != nil {
			return err
		}
	}
	return nil
}

// SubscribeEvents subscribes to events from a topic.
func (b *StreamBroker) SubscribeEvents(ctx context.Context, topicName string, handler messaging.EventHandler, opts ...messaging.SubscribeOption) (messaging.Subscription, error) {
	return b.Subscribe(ctx, topicName, func(ctx context.Context, msg *messaging.Message) error {
		event, err := messaging.EventFromMessage(msg)
		if err != nil {
			return err
		}
		return handler(ctx, event)
	}, opts...)
}

// StreamMessages returns a channel of messages from a topic.
func (b *StreamBroker) StreamMessages(ctx context.Context, topicName string, opts ...messaging.StreamOption) (<-chan *messaging.Message, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.connected {
		return nil, messaging.ErrNotConnected
	}

	options := messaging.ApplyStreamOptions(opts...)
	ch := make(chan *messaging.Message, options.BufferSize)

	topic, ok := b.topics[topicName]
	if !ok {
		close(ch)
		return ch, nil
	}

	// Start a goroutine to stream messages
	go func() {
		defer close(ch)

		offset := options.StartOffset
		if offset < 0 {
			offset = topic.GetHighWatermark(0) // Start from end
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-b.stopCh:
				return
			default:
				// Read from all partitions
				for i := 0; i < topic.PartitionCount(); i++ {
					msgs, err := topic.Read(i, offset, 100)
					if err != nil {
						continue
					}
					for _, msg := range msgs {
						select {
						case ch <- msg:
							offset = msg.Offset + 1
						case <-ctx.Done():
							return
						}
					}
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	return ch, nil
}

// StreamEvents returns a channel of events from a topic.
func (b *StreamBroker) StreamEvents(ctx context.Context, topicName string, opts ...messaging.StreamOption) (<-chan *messaging.Event, error) {
	msgCh, err := b.StreamMessages(ctx, topicName, opts...)
	if err != nil {
		return nil, err
	}

	eventCh := make(chan *messaging.Event, cap(msgCh))
	go func() {
		defer close(eventCh)
		for msg := range msgCh {
			event, err := messaging.EventFromMessage(msg)
			if err != nil {
				continue
			}
			eventCh <- event
		}
	}()

	return eventCh, nil
}

// CommitOffset commits the offset for a topic partition.
func (b *StreamBroker) CommitOffset(ctx context.Context, topic string, partition int32, offset int64) error {
	b.offsetsMu.Lock()
	defer b.offsetsMu.Unlock()

	// Use a default group ID if not specified
	groupID := "default"
	if _, ok := b.offsets[groupID]; !ok {
		b.offsets[groupID] = make(map[int32]int64)
	}
	b.offsets[groupID][partition] = offset
	return nil
}

// GetOffset returns the current offset for a topic partition.
func (b *StreamBroker) GetOffset(ctx context.Context, topic string, partition int32) (int64, error) {
	b.offsetsMu.RLock()
	defer b.offsetsMu.RUnlock()

	groupID := "default"
	if partitions, ok := b.offsets[groupID]; ok {
		if offset, ok := partitions[partition]; ok {
			return offset, nil
		}
	}
	return 0, nil
}

// SeekToOffset seeks to a specific offset.
func (b *StreamBroker) SeekToOffset(ctx context.Context, topic string, partition int32, offset int64) error {
	return b.CommitOffset(ctx, topic, partition, offset)
}

// SeekToTimestamp seeks to a specific timestamp.
func (b *StreamBroker) SeekToTimestamp(ctx context.Context, topic string, partition int32, ts time.Time) error {
	// For in-memory, just seek to beginning (simplified)
	return b.SeekToBeginning(ctx, topic, partition)
}

// SeekToBeginning seeks to the beginning of a topic partition.
func (b *StreamBroker) SeekToBeginning(ctx context.Context, topicName string, partition int32) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topic, ok := b.topics[topicName]
	if !ok {
		return messaging.ErrTopicNotFound
	}

	offset := topic.GetLowWatermark(int(partition))
	return b.CommitOffset(ctx, topicName, partition, offset)
}

// SeekToEnd seeks to the end of a topic partition.
func (b *StreamBroker) SeekToEnd(ctx context.Context, topicName string, partition int32) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topic, ok := b.topics[topicName]
	if !ok {
		return messaging.ErrTopicNotFound
	}

	offset := topic.GetHighWatermark(int(partition))
	return b.CommitOffset(ctx, topicName, partition, offset)
}

// CreateConsumerGroup creates a consumer group.
func (b *StreamBroker) CreateConsumerGroup(ctx context.Context, groupID string) error {
	b.offsetsMu.Lock()
	defer b.offsetsMu.Unlock()

	if _, ok := b.offsets[groupID]; !ok {
		b.offsets[groupID] = make(map[int32]int64)
	}
	return nil
}

// DeleteConsumerGroup deletes a consumer group.
func (b *StreamBroker) DeleteConsumerGroup(ctx context.Context, groupID string) error {
	b.offsetsMu.Lock()
	defer b.offsetsMu.Unlock()

	delete(b.offsets, groupID)
	return nil
}

// StreamSubscription is an in-memory stream subscription.
type StreamSubscription struct {
	id      string
	topic   string
	broker  *StreamBroker
	ch      chan *messaging.Event
	active  bool
	groupID string
	mu      sync.RWMutex
}

// Unsubscribe cancels the subscription.
func (s *StreamSubscription) Unsubscribe() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return nil
	}

	s.active = false
	s.broker.metrics.RecordUnsubscription()

	s.broker.mu.Lock()
	if topic, ok := s.broker.topics[s.topic]; ok {
		topic.Unsubscribe(s.groupID)
	}
	s.broker.mu.Unlock()

	return nil
}

// IsActive returns true if the subscription is active.
func (s *StreamSubscription) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

// Topic returns the subscribed topic.
func (s *StreamSubscription) Topic() string {
	return s.topic
}

// ID returns the subscription ID.
func (s *StreamSubscription) ID() string {
	return s.id
}

// consumeLoop consumes events from the channel.
func (s *StreamSubscription) consumeLoop(ctx context.Context, handler messaging.MessageHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-s.ch:
			if !ok {
				return
			}
			if !s.IsActive() {
				return
			}
			_ = handler(ctx, event.ToMessage()) //nolint:errcheck
		}
	}
}
