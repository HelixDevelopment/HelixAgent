package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/messaging"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Broker implements the messaging.MessageBroker interface for Kafka.
//
// Concurrency model (CONST-029):
//   - writers / readers are safe.Store containers; each individual
//     lookup, insert, and delete is atomic without any caller-held lock.
//   - closed / connected / subCounter are atomic primitives.
//   - regMu (sync.Mutex) is a Pattern Zeta scalar survivor. It
//     serialises multi-store compound operations that must stay
//     atomic across keys — specifically Connect's "connect-then-mark",
//     Subscribe's "check-connected → create-reader → launch-goroutine
//     → store-subscription" sequence, and Close's "swap-closed →
//     range+close every writer/reader" teardown. Because no bare
//     map/slice sits beside regMu, the audit is happy.
type Broker struct {
	config     *Config
	logger     *zap.Logger
	writers    *safe.Store[string, *kafka.Writer]
	readers    *safe.Store[string, *kafkaSubscription]
	regMu      sync.Mutex
	metrics    *messaging.BrokerMetrics
	closed     atomic.Bool
	connected  atomic.Bool
	subCounter atomic.Int64
}

// kafkaSubscription holds subscription state
type kafkaSubscription struct {
	id       string
	topic    string
	groupID  string
	handler  messaging.MessageHandler
	reader   *kafka.Reader
	cancelFn context.CancelFunc
	active   atomic.Bool
}

// Subscription interface implementation
func (s *kafkaSubscription) Unsubscribe() error {
	if !s.active.Swap(false) {
		return nil
	}

	if s.cancelFn != nil {
		s.cancelFn()
	}

	if s.reader != nil {
		return s.reader.Close()
	}
	return nil
}

func (s *kafkaSubscription) IsActive() bool {
	return s.active.Load()
}

func (s *kafkaSubscription) Topic() string {
	return s.topic
}

func (s *kafkaSubscription) ID() string {
	return s.id
}

// NewBroker creates a new Kafka broker
func NewBroker(config *Config, logger *zap.Logger) *Broker {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Broker{
		config:  config,
		logger:  logger,
		writers: safe.NewStore[string, *kafka.Writer](),
		readers: safe.NewStore[string, *kafkaSubscription](),
		metrics: messaging.NewBrokerMetrics(),
	}
}

// Connect establishes connection to Kafka
func (b *Broker) Connect(ctx context.Context) error {
	b.regMu.Lock()
	defer b.regMu.Unlock()

	if b.connected.Load() {
		return nil
	}

	b.metrics.RecordConnectionAttempt()

	// Test connection by getting cluster metadata
	conn, err := kafka.DialContext(ctx, "tcp", b.config.Brokers[0])
	if err != nil {
		b.metrics.RecordConnectionFailure()
		return fmt.Errorf("failed to connect to Kafka: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Get controller to verify connection
	_, err = conn.Controller()
	if err != nil {
		b.metrics.RecordConnectionFailure()
		return fmt.Errorf("failed to get Kafka controller: %w", err)
	}

	b.connected.Store(true)
	b.metrics.RecordConnectionSuccess()

	b.logger.Info("Connected to Kafka",
		zap.Strings("brokers", b.config.Brokers))

	return nil
}

// Close closes the broker
func (b *Broker) Close(ctx context.Context) error {
	if b.closed.Swap(true) {
		return nil
	}

	b.regMu.Lock()
	defer b.regMu.Unlock()

	var errs []error

	// Close all writers.
	b.writers.Range(func(topic string, writer *kafka.Writer) bool {
		if err := writer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close writer for %s: %w", topic, err))
		}
		return true
	})

	// Close all readers.
	b.readers.Range(func(topic string, sub *kafkaSubscription) bool {
		sub.active.Store(false)
		if sub.cancelFn != nil {
			sub.cancelFn()
		}
		if sub.reader != nil {
			if err := sub.reader.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close reader for %s: %w", topic, err))
			}
		}
		return true
	})

	b.connected.Store(false)
	b.metrics.RecordDisconnection()

	if len(errs) > 0 {
		return fmt.Errorf("errors closing broker: %v", errs)
	}

	b.logger.Info("Kafka broker closed")
	return nil
}

// HealthCheck performs a health check
func (b *Broker) HealthCheck(ctx context.Context) error {
	if !b.connected.Load() {
		return fmt.Errorf("not connected to Kafka")
	}

	conn, err := kafka.DialContext(ctx, "tcp", b.config.Brokers[0])
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() { _ = conn.Close() }()

	_, err = conn.Brokers()
	if err != nil {
		return fmt.Errorf("failed to get brokers: %w", err)
	}

	return nil
}

// IsConnected returns true if connected
func (b *Broker) IsConnected() bool {
	return b.connected.Load()
}

// getOrCreateWriter gets or creates a writer for a topic. Concurrent
// callers for the same topic converge on a single writer instance via
// Store.PutIfAbsent — at most one caller's freshly allocated writer is
// retained; any losing allocations are unused struct copies with no
// active connections, so there is no resource leak.
func (b *Broker) getOrCreateWriter(topic string) *kafka.Writer {
	if writer, ok := b.writers.Get(topic); ok {
		return writer
	}

	candidate := &kafka.Writer{
		Addr:                   kafka.TCP(b.config.Brokers...),
		Topic:                  topic,
		Balancer:               &kafka.LeastBytes{},
		BatchSize:              b.config.BatchSize,
		BatchTimeout:           b.config.BatchTimeout,
		MaxAttempts:            b.config.MaxRetries,
		RequiredAcks:           kafka.RequiredAcks(b.config.RequiredAcks),
		Async:                  false,
		AllowAutoTopicCreation: true,
	}

	writer, _ := b.writers.PutIfAbsent(topic, candidate)
	return writer
}

// Publish publishes a message to a topic
func (b *Broker) Publish(ctx context.Context, topic string, message *messaging.Message, opts ...messaging.PublishOption) error {
	startTime := time.Now()

	if !b.connected.Load() {
		b.metrics.RecordPublish(0, time.Since(startTime), false)
		return fmt.Errorf("not connected to Kafka")
	}

	if message == nil {
		return fmt.Errorf("message is nil")
	}

	// Apply options
	options := messaging.ApplyPublishOptions(opts...)

	// Serialize message
	body, err := json.Marshal(message)
	if err != nil {
		b.metrics.RecordSerializationError()
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Build Kafka message
	kafkaMsg := kafka.Message{
		Key:   []byte(message.ID),
		Value: body,
		Time:  message.Timestamp,
		Headers: []kafka.Header{
			{Key: "type", Value: []byte(message.Type)},
			{Key: "id", Value: []byte(message.ID)},
		},
	}

	// Add headers
	for k, v := range message.Headers {
		kafkaMsg.Headers = append(kafkaMsg.Headers, kafka.Header{
			Key:   k,
			Value: []byte(v),
		})
	}

	// Add trace ID
	if message.TraceID != "" {
		kafkaMsg.Headers = append(kafkaMsg.Headers, kafka.Header{
			Key:   "trace-id",
			Value: []byte(message.TraceID),
		})
	}

	// Use partition key if specified
	if len(options.Key) > 0 {
		kafkaMsg.Key = options.Key
	}

	// Get or create writer
	writer := b.getOrCreateWriter(topic)

	// Write message
	if err := writer.WriteMessages(ctx, kafkaMsg); err != nil {
		b.metrics.RecordPublish(int64(len(body)), time.Since(startTime), false)
		return fmt.Errorf("failed to publish message: %w", err)
	}

	b.metrics.RecordPublish(int64(len(body)), time.Since(startTime), true)
	return nil
}

// PublishBatch publishes multiple messages
func (b *Broker) PublishBatch(ctx context.Context, topic string, messages []*messaging.Message, opts ...messaging.PublishOption) error {
	startTime := time.Now()

	if !b.connected.Load() {
		return fmt.Errorf("not connected to Kafka")
	}

	if len(messages) == 0 {
		return fmt.Errorf("no messages to publish")
	}

	var totalBytes int64
	kafkaMessages := make([]kafka.Message, len(messages))
	for i, msg := range messages {
		body, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to serialize message %s: %w", msg.ID, err)
		}
		totalBytes += int64(len(body))

		kafkaMessages[i] = kafka.Message{
			Key:   []byte(msg.ID),
			Value: body,
			Time:  msg.Timestamp,
			Headers: []kafka.Header{
				{Key: "type", Value: []byte(msg.Type)},
				{Key: "id", Value: []byte(msg.ID)},
			},
		}

		// Add headers
		for k, v := range msg.Headers {
			kafkaMessages[i].Headers = append(kafkaMessages[i].Headers, kafka.Header{
				Key:   k,
				Value: []byte(v),
			})
		}
	}

	writer := b.getOrCreateWriter(topic)
	if err := writer.WriteMessages(ctx, kafkaMessages...); err != nil {
		b.metrics.RecordBatchPublish(len(messages), totalBytes, time.Since(startTime), false)
		return fmt.Errorf("failed to publish batch: %w", err)
	}

	b.metrics.RecordBatchPublish(len(messages), totalBytes, time.Since(startTime), true)
	return nil
}

// Subscribe subscribes to a topic
func (b *Broker) Subscribe(ctx context.Context, topic string, handler messaging.MessageHandler, opts ...messaging.SubscribeOption) (messaging.Subscription, error) {
	if topic == "" {
		return nil, fmt.Errorf("topic is required")
	}

	if handler == nil {
		return nil, fmt.Errorf("handler is required")
	}

	b.regMu.Lock()
	defer b.regMu.Unlock()

	if !b.connected.Load() {
		return nil, fmt.Errorf("not connected to Kafka")
	}

	// Apply options
	options := messaging.ApplySubscribeOptions(opts...)

	// Determine group ID
	groupID := b.config.GroupID
	if options.GroupID != "" {
		groupID = options.GroupID
	}

	// Determine start offset
	startOffset := kafka.FirstOffset
	if b.config.AutoOffsetReset == "latest" || options.OffsetReset == messaging.OffsetResetLatest {
		startOffset = kafka.LastOffset
	}

	// Create reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        b.config.Brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       b.config.FetchMinBytes,
		MaxBytes:       b.config.FetchMaxBytes,
		MaxWait:        b.config.FetchMaxWait,
		StartOffset:    startOffset,
		CommitInterval: b.config.AutoCommitInterval,
		Logger:         nil, // Use zap instead
	})

	// Create cancellation context
	subCtx, cancelFn := context.WithCancel(ctx)

	// Create subscription
	subID := fmt.Sprintf("sub-%d", b.subCounter.Add(1))
	sub := &kafkaSubscription{
		id:       subID,
		topic:    topic,
		groupID:  groupID,
		handler:  handler,
		reader:   reader,
		cancelFn: cancelFn,
	}
	sub.active.Store(true)

	b.readers.Put(topic, sub)
	b.metrics.RecordSubscription()

	// Start consumer
	go b.consumeMessages(subCtx, sub)

	b.logger.Info("Subscribed to Kafka topic",
		zap.String("topic", topic),
		zap.String("groupID", groupID))

	return sub, nil
}

// consumeMessages handles incoming messages
func (b *Broker) consumeMessages(ctx context.Context, sub *kafkaSubscription) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !sub.active.Load() {
			return
		}

		startTime := time.Now()

		// Fetch message
		kafkaMsg, err := sub.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled
			}
			b.logger.Error("Failed to fetch message",
				zap.String("topic", sub.topic),
				zap.Error(err))
			b.metrics.RecordFailed()
			continue
		}

		// Parse message
		var msg messaging.Message
		if err := json.Unmarshal(kafkaMsg.Value, &msg); err != nil {
			b.logger.Error("Failed to parse message",
				zap.Error(err),
				zap.ByteString("value", kafkaMsg.Value))
			// Commit offset to skip invalid message
			if err := sub.reader.CommitMessages(ctx, kafkaMsg); err != nil {
				b.logger.Warn("Failed to commit offset after parse error", zap.Error(err))
			}
			b.metrics.RecordFailed()
			b.metrics.RecordSerializationError()
			continue
		}

		b.metrics.RecordReceive(int64(len(kafkaMsg.Value)), time.Since(startTime))

		// Call handler
		if err := sub.handler(ctx, &msg); err != nil {
			b.logger.Error("Handler error",
				zap.String("topic", sub.topic),
				zap.String("messageId", msg.ID),
				zap.Error(err))
			b.metrics.RecordFailed()
			// Don't commit on error - message will be reprocessed
			continue
		}

		// Commit offset
		if !b.config.EnableAutoCommit {
			if err := sub.reader.CommitMessages(ctx, kafkaMsg); err != nil {
				b.logger.Error("Failed to commit offset",
					zap.String("topic", sub.topic),
					zap.Error(err))
			}
		}
		b.metrics.RecordProcessed()
	}
}

// CreateTopic creates a Kafka topic
func (b *Broker) CreateTopic(ctx context.Context, config *TopicConfig) error {
	if !b.connected.Load() {
		return fmt.Errorf("not connected to Kafka")
	}

	conn, err := kafka.DialContext(ctx, "tcp", b.config.Brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer func() { _ = conn.Close() }()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}

	controllerConn, err := kafka.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return fmt.Errorf("failed to connect to controller: %w", err)
	}
	defer func() { _ = controllerConn.Close() }()

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             config.Name,
			NumPartitions:     config.Partitions,
			ReplicationFactor: config.ReplicationFactor,
		},
	}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		return fmt.Errorf("failed to create topic: %w", err)
	}

	b.metrics.RecordTopicCreated()
	b.logger.Info("Created Kafka topic",
		zap.String("topic", config.Name),
		zap.Int("partitions", config.Partitions),
		zap.Int("replication", config.ReplicationFactor))

	return nil
}

// DeleteTopic deletes a Kafka topic
func (b *Broker) DeleteTopic(ctx context.Context, topic string) error {
	if !b.connected.Load() {
		return fmt.Errorf("not connected to Kafka")
	}

	conn, err := kafka.DialContext(ctx, "tcp", b.config.Brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer func() { _ = conn.Close() }()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("failed to get controller: %w", err)
	}

	controllerConn, err := kafka.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return fmt.Errorf("failed to connect to controller: %w", err)
	}
	defer func() { _ = controllerConn.Close() }()

	err = controllerConn.DeleteTopics(topic)
	if err != nil {
		return fmt.Errorf("failed to delete topic: %w", err)
	}

	b.logger.Info("Deleted Kafka topic", zap.String("topic", topic))
	return nil
}

// GetTopicMetadata returns topic metadata
func (b *Broker) GetTopicMetadata(ctx context.Context, topic string) (*TopicMetadata, error) {
	if !b.connected.Load() {
		return nil, fmt.Errorf("not connected to Kafka")
	}

	conn, err := kafka.DialContext(ctx, "tcp", b.config.Brokers[0])
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer func() { _ = conn.Close() }()

	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		return nil, fmt.Errorf("failed to read partitions: %w", err)
	}

	metadata := &TopicMetadata{
		Name:       topic,
		Partitions: make([]PartitionMetadata, len(partitions)),
	}

	for i, p := range partitions {
		metadata.Partitions[i] = PartitionMetadata{
			ID:       p.ID,
			Leader:   p.Leader.ID,
			Replicas: extractBrokerIDs(p.Replicas),
			ISR:      extractBrokerIDs(p.Isr),
		}
	}

	return metadata, nil
}

func extractBrokerIDs(brokers []kafka.Broker) []int {
	ids := make([]int, len(brokers))
	for i, b := range brokers {
		ids[i] = b.ID
	}
	return ids
}

// BrokerType returns the broker type
func (b *Broker) BrokerType() messaging.BrokerType {
	return messaging.BrokerTypeKafka
}

// GetMetrics returns broker metrics
func (b *Broker) GetMetrics() *messaging.BrokerMetrics {
	return b.metrics
}

// TopicMetadata holds topic metadata
type TopicMetadata struct {
	Name       string              `json:"name"`
	Partitions []PartitionMetadata `json:"partitions"`
}

// PartitionMetadata holds partition metadata
type PartitionMetadata struct {
	ID       int   `json:"id"`
	Leader   int   `json:"leader"`
	Replicas []int `json:"replicas"`
	ISR      []int `json:"isr"`
}
