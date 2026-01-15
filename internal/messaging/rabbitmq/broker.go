package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"dev.helix.agent/internal/messaging"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// Broker implements the messaging.MessageBroker interface for RabbitMQ
type Broker struct {
	config       *Config
	logger       *zap.Logger
	conn         *Connection
	pubChannel   *amqp.Channel
	mu           sync.RWMutex
	subscriptions map[string]*rabbitSubscription
	exchanges    map[string]bool
	queues       map[string]bool
	metrics      *messaging.BrokerMetrics
	closed       atomic.Bool
	subCounter   atomic.Int64
}

// rabbitSubscription holds subscription state
type rabbitSubscription struct {
	id        string
	topic     string
	queue     string
	handler   messaging.MessageHandler
	channel   *amqp.Channel
	consumer  string
	cancelCh  chan struct{}
	active    atomic.Bool
}

// Subscription interface implementation
func (s *rabbitSubscription) Unsubscribe() error {
	if !s.active.Swap(false) {
		return nil // Already unsubscribed
	}

	close(s.cancelCh)

	if s.channel != nil {
		if err := s.channel.Cancel(s.consumer, false); err != nil {
			return fmt.Errorf("failed to cancel consumer: %w", err)
		}
		return s.channel.Close()
	}
	return nil
}

func (s *rabbitSubscription) IsActive() bool {
	return s.active.Load()
}

func (s *rabbitSubscription) Topic() string {
	return s.topic
}

func (s *rabbitSubscription) ID() string {
	return s.id
}

// NewBroker creates a new RabbitMQ broker
func NewBroker(config *Config, logger *zap.Logger) *Broker {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Broker{
		config:        config,
		logger:        logger,
		subscriptions: make(map[string]*rabbitSubscription),
		exchanges:     make(map[string]bool),
		queues:        make(map[string]bool),
		metrics:       messaging.NewBrokerMetrics(),
	}
}

// Connect establishes connection to RabbitMQ
func (b *Broker) Connect(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.conn != nil && b.conn.IsConnected() {
		return nil
	}

	// Create connection
	b.conn = NewConnection(b.config, b.logger)

	// Set up reconnection callback to restore channels
	b.conn.OnReconnect(func() {
		b.restoreChannels()
	})

	b.metrics.RecordConnectionAttempt()

	if err := b.conn.Connect(ctx); err != nil {
		b.metrics.RecordConnectionFailure()
		return err
	}

	b.metrics.RecordConnectionSuccess()

	// Create publisher channel
	ch, err := b.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to create publisher channel: %w", err)
	}
	b.pubChannel = ch

	// Enable publisher confirms if configured
	if b.config.PublishConfirm {
		if err := b.pubChannel.Confirm(false); err != nil {
			b.logger.Warn("Failed to enable publisher confirms", zap.Error(err))
		}
	}

	// Set up default DLQ if enabled
	if b.config.EnableDLQ {
		if err := b.setupDLQ(ctx); err != nil {
			b.logger.Warn("Failed to set up DLQ", zap.Error(err))
		}
	}

	return nil
}

// setupDLQ creates the dead letter queue infrastructure
func (b *Broker) setupDLQ(ctx context.Context) error {
	// Declare DLQ exchange
	if err := b.pubChannel.ExchangeDeclare(
		b.config.DLQExchange,
		"topic",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare DLQ exchange: %w", err)
	}

	// Declare DLQ
	queueArgs := amqp.Table{
		"x-message-ttl": b.config.DLQMessageTTL,
		"x-max-length":  b.config.DLQMaxLength,
	}

	_, err := b.pubChannel.QueueDeclare(
		"helixagent.dlq",
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		queueArgs,
	)
	if err != nil {
		return fmt.Errorf("failed to declare DLQ: %w", err)
	}

	// Bind DLQ to exchange
	if err := b.pubChannel.QueueBind(
		"helixagent.dlq",
		"#", // all routing keys
		b.config.DLQExchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind DLQ: %w", err)
	}

	return nil
}

// restoreChannels recreates channels after reconnection
func (b *Broker) restoreChannels() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Recreate publisher channel
	ch, err := b.conn.Channel()
	if err != nil {
		b.logger.Error("Failed to recreate publisher channel", zap.Error(err))
		return
	}
	b.pubChannel = ch

	if b.config.PublishConfirm {
		if err := b.pubChannel.Confirm(false); err != nil {
			b.logger.Warn("Failed to enable publisher confirms", zap.Error(err))
		}
	}

	// Restore subscriptions
	for _, sub := range b.subscriptions {
		if sub.active.Load() {
			go b.restoreSubscription(sub)
		}
	}
}

// restoreSubscription recreates a subscription after reconnection
func (b *Broker) restoreSubscription(sub *rabbitSubscription) {
	ctx := context.Background()

	ch, err := b.conn.Channel()
	if err != nil {
		b.logger.Error("Failed to recreate subscription channel",
			zap.String("topic", sub.topic),
			zap.Error(err))
		return
	}

	// Set QoS
	if err := ch.Qos(b.config.PrefetchCount, b.config.PrefetchSize, false); err != nil {
		b.logger.Warn("Failed to set QoS", zap.Error(err))
	}

	// Re-declare queue
	_, err = ch.QueueDeclare(
		sub.queue,
		b.config.DefaultQueueDurable,
		b.config.DefaultQueueAutoDelete,
		b.config.DefaultQueueExclusive,
		false,
		nil,
	)
	if err != nil {
		b.logger.Error("Failed to re-declare queue",
			zap.String("queue", sub.queue),
			zap.Error(err))
		return
	}

	// Start consuming
	deliveries, err := ch.Consume(
		sub.queue,
		sub.consumer,
		b.config.AutoAck,
		b.config.Exclusive,
		b.config.NoLocal,
		b.config.NoWait,
		nil,
	)
	if err != nil {
		b.logger.Error("Failed to start consuming",
			zap.String("queue", sub.queue),
			zap.Error(err))
		return
	}

	sub.channel = ch
	go b.consumeMessages(ctx, sub, deliveries)

	b.logger.Info("Subscription restored",
		zap.String("topic", sub.topic),
		zap.String("queue", sub.queue))
}

// Close closes the broker connection
func (b *Broker) Close(ctx context.Context) error {
	if b.closed.Swap(true) {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Cancel all subscriptions
	for _, sub := range b.subscriptions {
		sub.active.Store(false)
		close(sub.cancelCh)
		if sub.channel != nil {
			sub.channel.Close()
		}
	}

	// Close publisher channel
	if b.pubChannel != nil {
		b.pubChannel.Close()
	}

	// Close connection
	if b.conn != nil {
		b.metrics.RecordDisconnection()
		return b.conn.Close()
	}

	return nil
}

// HealthCheck performs a health check
func (b *Broker) HealthCheck(ctx context.Context) error {
	if !b.IsConnected() {
		return fmt.Errorf("not connected to RabbitMQ")
	}

	// Try to create a temporary channel as a health check
	ch, err := b.conn.Channel()
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	ch.Close()

	return nil
}

// IsConnected returns true if connected
func (b *Broker) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.conn != nil && b.conn.IsConnected()
}

// Publish publishes a message to a topic
func (b *Broker) Publish(ctx context.Context, topic string, message *messaging.Message, opts ...messaging.PublishOption) error {
	startTime := time.Now()
	
	b.mu.RLock()
	ch := b.pubChannel
	b.mu.RUnlock()

	if ch == nil {
		b.metrics.RecordPublish(0, time.Since(startTime), false)
		return fmt.Errorf("publisher channel not available")
	}

	// Apply options
	options := messaging.ApplyPublishOptions(opts...)

	// Serialize message
	body, err := json.Marshal(message)
	if err != nil {
		b.metrics.RecordSerializationError()
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Build AMQP message
	publishing := amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
		Timestamp:    message.Timestamp,
		MessageId:    message.ID,
		Type:         message.Type,
		Headers:      amqp.Table{},
	}

	// Add headers
	for k, v := range message.Headers {
		publishing.Headers[k] = v
	}

	// Add priority if set
	if message.Priority > 0 {
		publishing.Priority = uint8(message.Priority)
	}

	// Add trace ID
	if message.TraceID != "" {
		publishing.Headers["trace-id"] = message.TraceID
	}

	// Ensure exchange exists
	exchange := topic
	if options.Exchange != "" {
		exchange = options.Exchange
	}

	if err := b.ensureExchange(exchange); err != nil {
		b.metrics.RecordPublish(0, time.Since(startTime), false)
		return err
	}

	// Determine routing key
	routingKey := topic
	if options.RoutingKey != "" {
		routingKey = options.RoutingKey
	}

	// Publish with confirm if enabled
	if b.config.PublishConfirm {
		confirm := make(chan amqp.Confirmation, 1)
		b.pubChannel.NotifyPublish(confirm)

		if err := ch.PublishWithContext(ctx, exchange, routingKey,
			b.config.MandatoryPublish, b.config.ImmediatePublish, publishing); err != nil {
			b.metrics.RecordPublish(int64(len(body)), time.Since(startTime), false)
			return fmt.Errorf("failed to publish message: %w", err)
		}

		select {
		case c := <-confirm:
			if !c.Ack {
				b.metrics.RecordPublish(int64(len(body)), time.Since(startTime), false)
				return fmt.Errorf("message was not confirmed")
			}
			b.metrics.RecordPublishConfirmation()
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(b.config.PublishTimeout):
			b.metrics.RecordPublishTimeout()
			return fmt.Errorf("publish confirmation timeout")
		}
	} else {
		if err := ch.PublishWithContext(ctx, exchange, routingKey,
			b.config.MandatoryPublish, b.config.ImmediatePublish, publishing); err != nil {
			b.metrics.RecordPublish(int64(len(body)), time.Since(startTime), false)
			return fmt.Errorf("failed to publish message: %w", err)
		}
	}

	b.metrics.RecordPublish(int64(len(body)), time.Since(startTime), true)
	return nil
}

// PublishBatch publishes multiple messages
func (b *Broker) PublishBatch(ctx context.Context, topic string, messages []*messaging.Message, opts ...messaging.PublishOption) error {
	for _, msg := range messages {
		if err := b.Publish(ctx, topic, msg, opts...); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe subscribes to a topic
func (b *Broker) Subscribe(ctx context.Context, topic string, handler messaging.MessageHandler, opts ...messaging.SubscribeOption) (messaging.Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Apply options
	options := messaging.ApplySubscribeOptions(opts...)

	// Create channel for this subscription
	ch, err := b.conn.Channel()
	if err != nil {
		b.metrics.RecordError()
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	// Set QoS
	prefetch := b.config.PrefetchCount
	if options.Prefetch > 0 {
		prefetch = options.Prefetch
	}
	if err := ch.Qos(prefetch, b.config.PrefetchSize, false); err != nil {
		ch.Close()
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	// Ensure exchange exists
	if err := b.ensureExchange(topic); err != nil {
		ch.Close()
		return nil, err
	}

	// Declare queue
	queueName := topic
	queueArgs := amqp.Table{}
	if b.config.EnableDLQ {
		queueArgs["x-dead-letter-exchange"] = b.config.DLQExchange
		queueArgs["x-dead-letter-routing-key"] = b.config.DLQRoutingKey
	}

	q, err := ch.QueueDeclare(
		queueName,
		b.config.DefaultQueueDurable,
		b.config.DefaultQueueAutoDelete,
		options.Exclusive,
		options.NoWait,
		queueArgs,
	)
	if err != nil {
		ch.Close()
		b.metrics.RecordError()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	b.metrics.RecordQueueDeclared()

	// Bind queue to exchange
	routingKey := "#" // Subscribe to all messages on this topic
	if err := ch.QueueBind(q.Name, routingKey, topic, false, nil); err != nil {
		ch.Close()
		return nil, fmt.Errorf("failed to bind queue: %w", err)
	}

	// Generate consumer tag and subscription ID
	subID := fmt.Sprintf("sub-%d", b.subCounter.Add(1))
	consumerTag := options.ConsumerTag
	if consumerTag == "" {
		consumerTag = fmt.Sprintf("helixagent-%s-%d", topic, time.Now().UnixNano())
	}

	// Start consuming
	deliveries, err := ch.Consume(
		q.Name,
		consumerTag,
		options.AutoAck,
		options.Exclusive,
		options.NoLocal,
		options.NoWait,
		nil,
	)
	if err != nil {
		ch.Close()
		b.metrics.RecordError()
		return nil, fmt.Errorf("failed to start consuming: %w", err)
	}

	// Create subscription
	sub := &rabbitSubscription{
		id:       subID,
		topic:    topic,
		queue:    q.Name,
		handler:  handler,
		channel:  ch,
		consumer: consumerTag,
		cancelCh: make(chan struct{}),
	}
	sub.active.Store(true)

	b.subscriptions[topic] = sub
	b.metrics.RecordSubscription()

	// Start message consumer
	go b.consumeMessages(ctx, sub, deliveries)

	b.logger.Info("Subscribed to topic",
		zap.String("topic", topic),
		zap.String("queue", q.Name),
		zap.String("consumer", consumerTag))

	return sub, nil
}

// consumeMessages handles incoming messages
func (b *Broker) consumeMessages(ctx context.Context, sub *rabbitSubscription, deliveries <-chan amqp.Delivery) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-sub.cancelCh:
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}

			startTime := time.Now()

			// Parse message
			var msg messaging.Message
			if err := json.Unmarshal(d.Body, &msg); err != nil {
				b.logger.Error("Failed to parse message",
					zap.Error(err),
					zap.ByteString("body", d.Body))
				if !b.config.AutoAck {
					d.Nack(false, false) // Don't requeue invalid messages
				}
				b.metrics.RecordFailed()
				b.metrics.RecordSerializationError()
				continue
			}

			// Call handler
			b.metrics.RecordReceive(int64(len(d.Body)), time.Since(startTime))

			if err := sub.handler(ctx, &msg); err != nil {
				b.logger.Error("Handler error",
					zap.String("topic", sub.topic),
					zap.String("messageId", msg.ID),
					zap.Error(err))
				if !b.config.AutoAck {
					d.Nack(false, true) // Requeue on handler error
				}
				b.metrics.RecordFailed()
				b.metrics.RecordNack()
				continue
			}

			// Acknowledge message
			if !b.config.AutoAck {
				if err := d.Ack(false); err != nil {
					b.logger.Error("Failed to acknowledge message",
						zap.String("messageId", msg.ID),
						zap.Error(err))
				}
				b.metrics.RecordAck()
			}
			b.metrics.RecordProcessed()
		}
	}
}

// ensureExchange ensures an exchange exists
func (b *Broker) ensureExchange(name string) error {
	if b.exchanges[name] {
		return nil
	}

	if err := b.pubChannel.ExchangeDeclare(
		name,
		b.config.DefaultExchangeType,
		b.config.DefaultExchangeDurable,
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", name, err)
	}

	b.exchanges[name] = true
	return nil
}

// BrokerType returns the broker type
func (b *Broker) BrokerType() messaging.BrokerType {
	return messaging.BrokerTypeRabbitMQ
}

// GetMetrics returns broker metrics
func (b *Broker) GetMetrics() *messaging.BrokerMetrics {
	return b.metrics
}
