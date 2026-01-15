// Package kafka provides an Apache Kafka message broker implementation.
package kafka

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"

	"dev.helix.agent/internal/messaging"
)

// Broker is a Kafka message broker implementation.
type Broker struct {
	config      *Config
	writer      *kafka.Writer
	readers     map[string]*kafka.Reader
	dialer      *kafka.Dialer
	metrics     *messaging.BrokerMetrics
	connected   bool
	mu          sync.RWMutex
	stopCh      chan struct{}
	subscribers map[string]*Subscription
}

// NewBroker creates a new Kafka broker.
func NewBroker(config *Config) *Broker {
	if config == nil {
		config = DefaultConfig()
	}
	return &Broker{
		config:      config,
		readers:     make(map[string]*kafka.Reader),
		metrics:     messaging.NewBrokerMetrics(),
		subscribers: make(map[string]*Subscription),
		stopCh:      make(chan struct{}),
	}
}

// Connect establishes a connection to Kafka.
func (b *Broker) Connect(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.connected {
		return nil
	}

	b.metrics.RecordConnectionAttempt()

	if err := b.config.Validate(); err != nil {
		b.metrics.RecordConnectionFailure()
		return err
	}

	// Create dialer with TLS and SASL if configured
	dialer := &kafka.Dialer{
		Timeout:   b.config.DialTimeout,
		DualStack: true,
		ClientID:  b.config.ClientID,
	}

	// Configure TLS
	if b.config.TLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: b.config.TLSInsecure,
		}
		dialer.TLS = tlsConfig
	}

	// Configure SASL
	if b.config.SASLEnabled {
		mechanism, err := b.getSASLMechanism()
		if err != nil {
			b.metrics.RecordConnectionFailure()
			return err
		}
		dialer.SASLMechanism = mechanism
	}

	b.dialer = dialer

	// Test connection by getting metadata
	conn, err := dialer.DialContext(ctx, "tcp", b.config.Brokers[0])
	if err != nil {
		b.metrics.RecordConnectionFailure()
		return messaging.ConnectionError("failed to connect to Kafka", err)
	}
	conn.Close()

	// Create writer
	b.writer = &kafka.Writer{
		Addr:         kafka.TCP(b.config.Brokers...),
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    b.config.BatchSize,
		BatchBytes:   b.config.BatchBytes,
		BatchTimeout: b.config.BatchTimeout,
		ReadTimeout:  b.config.ReadTimeout,
		WriteTimeout: b.config.WriteTimeout,
		RequiredAcks: kafka.RequiredAcks(b.config.RequiredAcks),
		MaxAttempts:  b.config.MaxAttempts,
		Compression:  b.getCompression(),
		Transport: &kafka.Transport{
			TLS:  dialer.TLS,
			SASL: dialer.SASLMechanism,
		},
	}

	b.connected = true
	b.metrics.RecordConnectionSuccess()

	return nil
}

// getSASLMechanism returns the SASL mechanism based on configuration.
func (b *Broker) getSASLMechanism() (sasl.Mechanism, error) {
	switch b.config.SASLMechanism {
	case "PLAIN":
		return plain.Mechanism{
			Username: b.config.SASLUsername,
			Password: b.config.SASLPassword,
		}, nil
	case "SCRAM-SHA-256":
		return scram.Mechanism(scram.SHA256, b.config.SASLUsername, b.config.SASLPassword)
	case "SCRAM-SHA-512":
		return scram.Mechanism(scram.SHA512, b.config.SASLUsername, b.config.SASLPassword)
	default:
		return nil, messaging.ConfigError("unsupported SASL mechanism: " + b.config.SASLMechanism)
	}
}

// getCompression returns the compression codec.
func (b *Broker) getCompression() kafka.Compression {
	switch b.config.Compression {
	case CompressionGzip:
		return kafka.Gzip
	case CompressionSnappy:
		return kafka.Snappy
	case CompressionLZ4:
		return kafka.Lz4
	case CompressionZstd:
		return kafka.Zstd
	default:
		return 0 // No compression
	}
}

// Close closes the connection to Kafka.
func (b *Broker) Close(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.connected {
		return nil
	}

	close(b.stopCh)

	var errs messaging.MultiError

	// Close all subscriptions
	for _, sub := range b.subscribers {
		sub.cancel()
	}
	b.subscribers = make(map[string]*Subscription)

	// Close writer
	if b.writer != nil {
		if err := b.writer.Close(); err != nil {
			errs.Add(err)
		}
	}

	// Close all readers
	for _, reader := range b.readers {
		if err := reader.Close(); err != nil {
			errs.Add(err)
		}
	}
	b.readers = make(map[string]*kafka.Reader)

	b.connected = false
	b.metrics.RecordDisconnection()

	return errs.ErrorOrNil()
}

// HealthCheck checks if the broker is healthy.
func (b *Broker) HealthCheck(ctx context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.connected {
		return messaging.ErrNotConnected
	}

	// Try to connect to any broker
	conn, err := b.dialer.DialContext(ctx, "tcp", b.config.Brokers[0])
	if err != nil {
		return messaging.ConnectionError("health check failed", err)
	}
	conn.Close()

	return nil
}

// IsConnected returns true if connected to Kafka.
func (b *Broker) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}

// Publish publishes a message to a topic.
func (b *Broker) Publish(ctx context.Context, topic string, message *messaging.Message, opts ...messaging.PublishOption) error {
	b.mu.RLock()
	if !b.connected {
		b.mu.RUnlock()
		return messaging.ErrNotConnected
	}
	writer := b.writer
	b.mu.RUnlock()

	options := messaging.ApplyPublishOptions(opts...)
	start := time.Now()

	// Build Kafka message
	kafkaMsg := kafka.Message{
		Topic: topic,
		Value: message.Payload,
		Time:  message.Timestamp,
	}

	// Set key if provided
	if options.Key != nil {
		kafkaMsg.Key = options.Key
	} else if message.CorrelationID != "" {
		kafkaMsg.Key = []byte(message.CorrelationID)
	}

	// Set partition if specified
	if options.Partition != nil {
		kafkaMsg.Partition = int(*options.Partition)
	}

	// Set headers
	kafkaMsg.Headers = make([]kafka.Header, 0)
	for k, v := range message.Headers {
		kafkaMsg.Headers = append(kafkaMsg.Headers, kafka.Header{Key: k, Value: []byte(v)})
	}
	if message.TraceID != "" {
		kafkaMsg.Headers = append(kafkaMsg.Headers, kafka.Header{Key: "trace_id", Value: []byte(message.TraceID)})
	}
	if message.ID != "" {
		kafkaMsg.Headers = append(kafkaMsg.Headers, kafka.Header{Key: "message_id", Value: []byte(message.ID)})
	}
	kafkaMsg.Headers = append(kafkaMsg.Headers, kafka.Header{Key: "message_type", Value: []byte(message.Type)})

	// Write with timeout
	writeCtx := ctx
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		writeCtx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	err := writer.WriteMessages(writeCtx, kafkaMsg)
	if err != nil {
		b.metrics.RecordPublish(int64(len(message.Payload)), time.Since(start), false)
		return messaging.PublishError(topic, err)
	}

	b.metrics.RecordPublish(int64(len(message.Payload)), time.Since(start), true)
	return nil
}

// PublishBatch publishes multiple messages to a topic.
func (b *Broker) PublishBatch(ctx context.Context, topic string, messages []*messaging.Message, opts ...messaging.PublishOption) error {
	b.mu.RLock()
	if !b.connected {
		b.mu.RUnlock()
		return messaging.ErrNotConnected
	}
	writer := b.writer
	b.mu.RUnlock()

	options := messaging.ApplyPublishOptions(opts...)
	start := time.Now()

	// Convert messages
	kafkaMessages := make([]kafka.Message, len(messages))
	totalBytes := int64(0)
	for i, msg := range messages {
		kafkaMessages[i] = kafka.Message{
			Topic: topic,
			Value: msg.Payload,
			Time:  msg.Timestamp,
		}
		if msg.CorrelationID != "" {
			kafkaMessages[i].Key = []byte(msg.CorrelationID)
		}
		kafkaMessages[i].Headers = make([]kafka.Header, 0)
		for k, v := range msg.Headers {
			kafkaMessages[i].Headers = append(kafkaMessages[i].Headers, kafka.Header{Key: k, Value: []byte(v)})
		}
		if msg.TraceID != "" {
			kafkaMessages[i].Headers = append(kafkaMessages[i].Headers, kafka.Header{Key: "trace_id", Value: []byte(msg.TraceID)})
		}
		if msg.ID != "" {
			kafkaMessages[i].Headers = append(kafkaMessages[i].Headers, kafka.Header{Key: "message_id", Value: []byte(msg.ID)})
		}
		kafkaMessages[i].Headers = append(kafkaMessages[i].Headers, kafka.Header{Key: "message_type", Value: []byte(msg.Type)})
		totalBytes += int64(len(msg.Payload))
	}

	// Write with timeout
	writeCtx := ctx
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		writeCtx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	err := writer.WriteMessages(writeCtx, kafkaMessages...)
	if err != nil {
		b.metrics.RecordBatchPublish(len(messages), totalBytes, time.Since(start), false)
		return messaging.PublishError(topic, err)
	}

	b.metrics.RecordBatchPublish(len(messages), totalBytes, time.Since(start), true)
	return nil
}

// Subscribe creates a subscription to a topic.
func (b *Broker) Subscribe(ctx context.Context, topic string, handler messaging.MessageHandler, opts ...messaging.SubscribeOption) (messaging.Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.connected {
		return nil, messaging.ErrNotConnected
	}

	options := messaging.ApplySubscribeOptions(opts...)

	// Create reader config
	readerConfig := kafka.ReaderConfig{
		Brokers:        b.config.Brokers,
		Topic:          topic,
		GroupID:        options.GroupID,
		MinBytes:       b.config.MinBytes,
		MaxBytes:       b.config.MaxBytes,
		MaxWait:        b.config.MaxWait,
		CommitInterval: options.CommitInterval,
		StartOffset:    b.config.StartOffset,
		Dialer:         b.dialer,
	}

	if options.GroupID == "" {
		readerConfig.GroupID = b.config.GroupID
	}

	reader := kafka.NewReader(readerConfig)

	sub := &Subscription{
		id:       generateSubscriptionID(),
		topic:    topic,
		broker:   b,
		reader:   reader,
		handler:  handler,
		options:  options,
		active:   true,
		stopCh:   make(chan struct{}),
	}

	b.subscribers[sub.id] = sub
	b.readers[sub.id] = reader
	b.metrics.RecordSubscription()

	// Start consuming in background
	go sub.consume(ctx)

	return sub, nil
}

// BrokerType returns the broker type.
func (b *Broker) BrokerType() messaging.BrokerType {
	return messaging.BrokerTypeKafka
}

// GetMetrics returns broker metrics.
func (b *Broker) GetMetrics() *messaging.BrokerMetrics {
	return b.metrics
}

// CreateTopic creates a new topic.
func (b *Broker) CreateTopic(ctx context.Context, name string, partitions int, replication int) error {
	b.mu.RLock()
	if !b.connected {
		b.mu.RUnlock()
		return messaging.ErrNotConnected
	}
	dialer := b.dialer
	b.mu.RUnlock()

	conn, err := dialer.DialContext(ctx, "tcp", b.config.Brokers[0])
	if err != nil {
		return messaging.ConnectionError("failed to connect for topic creation", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return messaging.TopicError(name, err)
	}

	controllerConn, err := dialer.DialContext(ctx, "tcp", controller.Host+":"+string(rune(controller.Port)))
	if err != nil {
		return messaging.ConnectionError("failed to connect to controller", err)
	}
	defer controllerConn.Close()

	topicConfig := kafka.TopicConfig{
		Topic:             name,
		NumPartitions:     partitions,
		ReplicationFactor: replication,
	}

	err = controllerConn.CreateTopics(topicConfig)
	if err != nil {
		return messaging.TopicError(name, err)
	}

	b.metrics.RecordTopicCreated()
	return nil
}

// DeleteTopic deletes a topic.
func (b *Broker) DeleteTopic(ctx context.Context, name string) error {
	b.mu.RLock()
	if !b.connected {
		b.mu.RUnlock()
		return messaging.ErrNotConnected
	}
	dialer := b.dialer
	b.mu.RUnlock()

	conn, err := dialer.DialContext(ctx, "tcp", b.config.Brokers[0])
	if err != nil {
		return messaging.ConnectionError("failed to connect for topic deletion", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return messaging.TopicError(name, err)
	}

	controllerConn, err := dialer.DialContext(ctx, "tcp", controller.Host+":"+string(rune(controller.Port)))
	if err != nil {
		return messaging.ConnectionError("failed to connect to controller", err)
	}
	defer controllerConn.Close()

	err = controllerConn.DeleteTopics(name)
	if err != nil {
		return messaging.TopicError(name, err)
	}

	return nil
}

// GetTopicMetadata returns metadata for a topic.
func (b *Broker) GetTopicMetadata(ctx context.Context, topic string) (*messaging.TopicMetadata, error) {
	b.mu.RLock()
	if !b.connected {
		b.mu.RUnlock()
		return nil, messaging.ErrNotConnected
	}
	dialer := b.dialer
	b.mu.RUnlock()

	conn, err := dialer.DialContext(ctx, "tcp", b.config.Brokers[0])
	if err != nil {
		return nil, messaging.ConnectionError("failed to connect for topic metadata", err)
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		return nil, messaging.TopicError(topic, err)
	}

	metadata := &messaging.TopicMetadata{
		Name:       topic,
		Partitions: len(partitions),
		Timestamp:  time.Now().UTC(),
	}

	metadata.PartitionInfo = make([]messaging.PartitionInfo, len(partitions))
	for i, p := range partitions {
		replicas := make([]int32, len(p.Replicas))
		for j, r := range p.Replicas {
			replicas[j] = int32(r.ID)
		}
		isr := make([]int32, len(p.Isr))
		for j, r := range p.Isr {
			isr[j] = int32(r.ID)
		}
		metadata.PartitionInfo[i] = messaging.PartitionInfo{
			ID:       int32(p.ID),
			Leader:   int32(p.Leader.ID),
			Replicas: replicas,
			ISR:      isr,
		}
	}

	return metadata, nil
}

// PublishEvent publishes an event to a topic.
func (b *Broker) PublishEvent(ctx context.Context, topic string, event *messaging.Event) error {
	// Serialize event
	eventData, err := json.Marshal(event)
	if err != nil {
		return messaging.SerializationError(err)
	}

	msg := &messaging.Message{
		ID:            event.ID,
		Type:          string(event.Type),
		Payload:       eventData,
		Headers:       event.Headers,
		Timestamp:     event.Timestamp,
		TraceID:       event.TraceID,
		CorrelationID: event.CorrelationID,
	}

	var opts []messaging.PublishOption
	if event.Key != nil {
		opts = append(opts, messaging.WithMessageKey(event.Key))
	}

	return b.Publish(ctx, topic, msg, opts...)
}

// StreamEvents streams events from a topic.
func (b *Broker) StreamEvents(ctx context.Context, topic string, opts ...messaging.StreamOption) (<-chan *messaging.Event, error) {
	b.mu.RLock()
	if !b.connected {
		b.mu.RUnlock()
		return nil, messaging.ErrNotConnected
	}
	b.mu.RUnlock()

	options := messaging.ApplyStreamOptions(opts...)

	readerConfig := kafka.ReaderConfig{
		Brokers:     b.config.Brokers,
		Topic:       topic,
		GroupID:     b.config.GroupID + "-stream",
		MinBytes:    options.MinBytes,
		MaxBytes:    options.MaxBytes,
		MaxWait:     options.MaxWait,
		StartOffset: options.StartOffset,
		Dialer:      b.dialer,
	}

	reader := kafka.NewReader(readerConfig)

	eventCh := make(chan *messaging.Event, options.BufferSize)

	go func() {
		defer close(eventCh)
		defer reader.Close()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				kafkaMsg, err := reader.FetchMessage(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					b.metrics.RecordError()
					continue
				}

				event := b.kafkaMessageToEvent(kafkaMsg)
				select {
				case eventCh <- event:
					reader.CommitMessages(ctx, kafkaMsg)
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return eventCh, nil
}

// kafkaMessageToEvent converts a Kafka message to an Event.
func (b *Broker) kafkaMessageToEvent(kafkaMsg kafka.Message) *messaging.Event {
	var event messaging.Event
	if err := json.Unmarshal(kafkaMsg.Value, &event); err != nil {
		// If not a valid event JSON, create a simple event
		event = messaging.Event{
			ID:        string(getHeaderValue(kafkaMsg.Headers, "message_id")),
			Type:      messaging.EventType(getHeaderValue(kafkaMsg.Headers, "message_type")),
			Data:      kafkaMsg.Value,
			Timestamp: kafkaMsg.Time,
			Partition: int32(kafkaMsg.Partition),
			Offset:    kafkaMsg.Offset,
		}
	}

	event.Partition = int32(kafkaMsg.Partition)
	event.Offset = kafkaMsg.Offset
	event.Key = kafkaMsg.Key

	if event.Headers == nil {
		event.Headers = make(map[string]string)
	}
	for _, h := range kafkaMsg.Headers {
		event.Headers[h.Key] = string(h.Value)
	}

	return &event
}

// getHeaderValue gets a header value from Kafka headers.
func getHeaderValue(headers []kafka.Header, key string) string {
	for _, h := range headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

// CommitOffset commits an offset for a partition.
func (b *Broker) CommitOffset(ctx context.Context, topic string, partition int32, offset int64) error {
	// This is handled automatically by the consumer group
	return nil
}

// SeekToOffset seeks to a specific offset.
func (b *Broker) SeekToOffset(ctx context.Context, topic string, partition int32, offset int64) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Find reader for this topic
	for _, reader := range b.readers {
		if reader.Config().Topic == topic {
			return reader.SetOffset(offset)
		}
	}

	return messaging.TopicError(topic, nil)
}

// SeekToTimestamp seeks to a specific timestamp.
func (b *Broker) SeekToTimestamp(ctx context.Context, topic string, partition int32, ts time.Time) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Find reader for this topic
	for _, reader := range b.readers {
		if reader.Config().Topic == topic {
			return reader.SetOffsetAt(ctx, ts)
		}
	}

	return messaging.TopicError(topic, nil)
}

// generateSubscriptionID generates a unique subscription ID.
func generateSubscriptionID() string {
	return "kafka-sub-" + time.Now().UTC().Format("20060102150405") + "-" + randomString(8)
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
