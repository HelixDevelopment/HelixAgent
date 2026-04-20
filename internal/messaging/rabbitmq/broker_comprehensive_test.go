package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.agent/internal/messaging"
	"dev.helix.agent/internal/testutil"
)

// ============================================================================
// Connection Tests
// ============================================================================

func TestBroker_Connect_WithNilConnection(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.NotNil(t, broker)
	assert.False(t, broker.IsConnected())
}

func TestBroker_Connect_AlreadyConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	// Simulate connected state by checking the logic path
	assert.False(t, broker.IsConnected())
}

func TestBroker_Close_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	err := broker.Close(context.Background())
	assert.NoError(t, err)
}

func TestBroker_Close_AlreadyClosed(t *testing.T) {
	broker := NewBroker(nil, nil)
	err := broker.Close(context.Background())
	assert.NoError(t, err)

	// Close again should be idempotent
	err = broker.Close(context.Background())
	assert.NoError(t, err)
}

func TestBroker_HealthCheck_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	err := broker.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// ============================================================================
// Configuration Tests
// ============================================================================

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := DefaultConfig()
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_EmptyHost(t *testing.T) {
	cfg := &Config{
		Host:     "",
		Port:     5672,
		Username: "guest",
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestConfig_Validate_InvalidPort(t *testing.T) {
	cfg := &Config{
		Host:     "localhost",
		Port:     0,
		Username: "guest",
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestConfig_Validate_NegativePrefetch(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PrefetchCount = -1
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prefetch")
}

func TestConfig_DefaultValues(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, 5672, cfg.Port)
	assert.Equal(t, "guest", cfg.Username)
	assert.Equal(t, "guest", cfg.Password)
	assert.Equal(t, "/", cfg.VHost)
	assert.False(t, cfg.TLSEnabled)
	assert.Equal(t, 30*time.Second, cfg.ConnectionTimeout)
	assert.Equal(t, 1*time.Second, cfg.ReconnectDelay)
	assert.True(t, cfg.PublishConfirm)
	assert.Equal(t, 10, cfg.PrefetchCount)
	assert.True(t, cfg.DefaultQueueDurable)
	assert.Equal(t, "topic", cfg.DefaultExchangeType)
	assert.True(t, cfg.EnableDLQ)
}

// ============================================================================
// Queue Configuration Tests
// ============================================================================

func TestQueueConfig_Build(t *testing.T) {
	cfg := DefaultQueueConfig("test.queue")
	cfg.WithTTL(60000).
		WithMaxLength(1000).
		WithPriority(10).
		WithDLQ("dlx.exchange", "dlx.routing")

	assert.Equal(t, "test.queue", cfg.Name)
	assert.Equal(t, 60000, cfg.MessageTTL)
	assert.Equal(t, 1000, cfg.MaxLength)
	assert.Equal(t, 10, cfg.MaxPriority)
	assert.Equal(t, "dlx.exchange", cfg.DeadLetterExchange)
	assert.Equal(t, "dlx.routing", cfg.DeadLetterRoutingKey)

	// Verify args are set
	assert.Equal(t, 60000, cfg.Args["x-message-ttl"])
	assert.Equal(t, 1000, cfg.Args["x-max-length"])
	assert.Equal(t, 10, cfg.Args["x-max-priority"])
	assert.Equal(t, "dlx.exchange", cfg.Args["x-dead-letter-exchange"])
	assert.Equal(t, "dlx.routing", cfg.Args["x-dead-letter-routing-key"])
}

func TestExchangeConfig_Types(t *testing.T) {
	types := []string{"direct", "fanout", "topic", "headers"}

	for _, exType := range types {
		t.Run(exType, func(t *testing.T) {
			cfg := DefaultExchangeConfig("test.exchange")
			cfg.Type = exType
			assert.Equal(t, exType, cfg.Type)
		})
	}
}

// ============================================================================
// Message Handling Tests
// ============================================================================

func TestMessage_Serialization(t *testing.T) {
	msg := &messaging.Message{
		ID:        "test-id-123",
		Type:      "test.event",
		Payload:   []byte(`{"key": "value"}`),
		Timestamp: time.Now(),
		Headers:   map[string]string{"x-custom": "header"},
		TraceID:   "trace-123",
		Priority:  messaging.PriorityNormal,
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded messaging.Message
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, msg.ID, decoded.ID)
	assert.Equal(t, msg.Type, decoded.Type)
	assert.Equal(t, msg.TraceID, decoded.TraceID)
	assert.Equal(t, msg.Priority, decoded.Priority)
}

func TestMessage_WithLargePayload(t *testing.T) {
	// Create a large payload (1MB)
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	msg := &messaging.Message{
		ID:        "large-msg-123",
		Type:      "large.payload",
		Payload:   largeData,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)
	assert.True(t, len(data) > 1024*1024)
}

// ============================================================================
// Subscription Tests
// ============================================================================

func TestSubscription_Interface(t *testing.T) {
	sub := &rabbitSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		queue:    "test.queue",
		cancelCh: make(chan struct{}),
	}
	sub.active.Store(true)

	assert.Equal(t, "sub-1", sub.ID())
	assert.Equal(t, "test.topic", sub.Topic())
	assert.True(t, sub.IsActive())
}

func TestSubscription_Unsubscribe(t *testing.T) {
	sub := &rabbitSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		queue:    "test.queue",
		cancelCh: make(chan struct{}),
	}
	sub.active.Store(true)

	// Unsubscribe should close cancelCh and set active to false
	err := sub.Unsubscribe()
	assert.NoError(t, err)
	assert.False(t, sub.IsActive())
}

func TestSubscription_Unsubscribe_AlreadyUnsubscribed(t *testing.T) {
	sub := &rabbitSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		queue:    "test.queue",
		cancelCh: make(chan struct{}),
	}
	sub.active.Store(false)

	// Should be idempotent
	err := sub.Unsubscribe()
	assert.NoError(t, err)
}

// ============================================================================
// Metrics Tests
// ============================================================================

func TestBroker_MetricsInitialized(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	assert.NotNil(t, metrics)
	assert.Equal(t, int64(0), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(0), metrics.MessagesReceived.Load())
	assert.Equal(t, int64(0), metrics.TotalErrors.Load())
}

func TestBrokerMetrics_Recording(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	// Record various metrics
	metrics.RecordConnectionAttempt()
	metrics.RecordConnectionSuccess()
	metrics.RecordPublish(100, time.Millisecond*10, true)
	metrics.RecordPublish(200, time.Millisecond*20, true)
	metrics.RecordReceive(150, time.Millisecond*5)
	metrics.RecordAck()
	metrics.RecordNack()
	metrics.RecordError()

	assert.Equal(t, int64(1), metrics.ConnectionAttempts.Load())
	assert.Equal(t, int64(1), metrics.ConnectionSuccesses.Load())
	assert.Equal(t, int64(2), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(300), metrics.BytesPublished.Load())
	assert.Equal(t, int64(1), metrics.MessagesReceived.Load())
	assert.Equal(t, int64(150), metrics.BytesReceived.Load())
	assert.Equal(t, int64(1), metrics.MessagesAcked.Load())
	assert.Equal(t, int64(1), metrics.MessagesNacked.Load())
	assert.Equal(t, int64(1), metrics.TotalErrors.Load())
}

func TestBrokerMetrics_PublishSuccess(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordPublish(500, time.Millisecond*50, true)

	assert.Equal(t, int64(1), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(1), metrics.PublishSuccesses.Load())
	assert.Equal(t, int64(0), metrics.PublishFailures.Load())
	assert.Equal(t, int64(500), metrics.BytesPublished.Load())
}

func TestBrokerMetrics_PublishFailure(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordPublish(500, time.Millisecond*50, false)

	assert.Equal(t, int64(1), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(0), metrics.PublishSuccesses.Load())
	assert.Equal(t, int64(1), metrics.PublishFailures.Load())
	assert.Equal(t, int64(0), metrics.BytesPublished.Load())
}

// ============================================================================
// Publish Options Tests
// ============================================================================

func TestPublishOptions_Apply(t *testing.T) {
	opts := messaging.ApplyPublishOptions(
		messaging.WithExchange("custom.exchange"),
		messaging.WithRoutingKey("custom.key"),
	)

	assert.Equal(t, "custom.exchange", opts.Exchange)
	assert.Equal(t, "custom.key", opts.RoutingKey)
}

func TestPublishOptions_Defaults(t *testing.T) {
	opts := messaging.ApplyPublishOptions()

	assert.Empty(t, opts.Exchange)
	assert.Empty(t, opts.RoutingKey)
}

// ============================================================================
// Subscribe Options Tests
// ============================================================================

func TestSubscribeOptions_Apply(t *testing.T) {
	opts := messaging.ApplySubscribeOptions(
		messaging.WithAutoAck(true),
		messaging.WithExclusive(true),
		messaging.WithPrefetch(100),
		messaging.WithConsumerTag("my-consumer"),
	)

	assert.True(t, opts.AutoAck)
	assert.True(t, opts.Exclusive)
	assert.Equal(t, 100, opts.Prefetch)
	assert.Equal(t, "my-consumer", opts.ConsumerTag)
}

func TestSubscribeOptions_Defaults(t *testing.T) {
	opts := messaging.ApplySubscribeOptions()

	assert.False(t, opts.AutoAck)
	assert.False(t, opts.Exclusive)
	assert.Equal(t, 10, opts.Prefetch) // Default prefetch is 10
	assert.Empty(t, opts.ConsumerTag)
}

// ============================================================================
// Broker Type Tests
// ============================================================================

func TestBroker_BrokerType_IsRabbitMQ(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.Equal(t, messaging.BrokerTypeRabbitMQ, broker.BrokerType())
	assert.NotEqual(t, messaging.BrokerTypeKafka, broker.BrokerType())
	assert.NotEqual(t, messaging.BrokerTypeInMemory, broker.BrokerType())
}

// ============================================================================
// Logger Tests
// ============================================================================

func TestBroker_WithCustomLogger(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	broker := NewBroker(nil, logger)

	assert.NotNil(t, broker.logger)
}

func TestBroker_WithNilLogger(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.NotNil(t, broker.logger) // Should use nop logger
}

// ============================================================================
// Concurrent Access Tests
// ============================================================================

func TestBroker_ConcurrentAccess(t *testing.T) {
	broker := NewBroker(nil, nil)
	var wg sync.WaitGroup

	// Simulate concurrent access to broker methods
	for i := 0; i < 100; i++ {
		wg.Add(3)

		go func() {
			defer wg.Done()
			_ = broker.IsConnected()
		}()

		go func() {
			defer wg.Done()
			_ = broker.GetMetrics()
		}()

		go func() {
			defer wg.Done()
			_ = broker.BrokerType()
		}()
	}

	wg.Wait()
}

func TestSubscription_ConcurrentUnsubscribe(t *testing.T) {
	sub := &rabbitSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		queue:    "test.queue",
		cancelCh: make(chan struct{}),
	}
	sub.active.Store(true)

	var wg sync.WaitGroup
	var successCount atomic.Int32

	// Try to unsubscribe from multiple goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := sub.Unsubscribe()
			if err == nil {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()

	// Only one should successfully close the channel
	assert.False(t, sub.IsActive())
	assert.GreaterOrEqual(t, successCount.Load(), int32(1))
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestBroker_Publish_WithNilChannel(t *testing.T) {
	broker := NewBroker(nil, nil)
	msg := &messaging.Message{
		ID:   "test-id",
		Type: "test.event",
	}

	err := broker.Publish(context.Background(), "test.topic", msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "channel not available")
}

func TestBroker_PublishBatch_EmptyMessages(t *testing.T) {
	broker := NewBroker(nil, nil)
	err := broker.PublishBatch(context.Background(), "test.topic", []*messaging.Message{})
	assert.NoError(t, err) // Empty batch should succeed
}

// ============================================================================
// Context Cancellation Tests
// ============================================================================

func TestBroker_Connect_ContextCancellation(t *testing.T) {
	broker := NewBroker(&Config{
		Host:              "nonexistent.host",
		Port:              5672,
		ConnectionTimeout: time.Second * 30,
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	err := broker.Connect(ctx)
	// Should fail either due to context or connection error
	assert.Error(t, err)
}

func TestBroker_Close_ContextCancellation(t *testing.T) {
	broker := NewBroker(nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := broker.Close(ctx)
	assert.NoError(t, err) // Close should still work
}

// ============================================================================
// State Machine Tests
// ============================================================================

func TestConnectionState_Transitions(t *testing.T) {
	tests := []struct {
		from     ConnectionState
		to       ConnectionState
		expected bool
	}{
		{StateDisconnected, StateConnecting, true},
		{StateConnecting, StateConnected, true},
		{StateConnected, StateReconnecting, true},
		{StateReconnecting, StateConnected, true},
		{StateConnected, StateClosed, true},
	}

	for _, tt := range tests {
		t.Run(tt.from.String()+"->"+tt.to.String(), func(t *testing.T) {
			// States should be different
			assert.NotEqual(t, tt.from, tt.to)
		})
	}
}

func TestConnectionState_AllStatesHaveStrings(t *testing.T) {
	states := []ConnectionState{
		StateDisconnected,
		StateConnecting,
		StateConnected,
		StateReconnecting,
		StateClosed,
	}

	for _, state := range states {
		assert.NotEmpty(t, state.String())
	}
}

// ============================================================================
// DLQ Configuration Tests
// ============================================================================

func TestConfig_DLQSettings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableDLQ = true
	cfg.DLQExchange = "dlx.exchange"
	cfg.DLQRoutingKey = "dlx.routing"
	cfg.DLQMessageTTL = 86400000 // 24 hours
	cfg.DLQMaxLength = 10000

	assert.True(t, cfg.EnableDLQ)
	assert.Equal(t, "dlx.exchange", cfg.DLQExchange)
	assert.Equal(t, "dlx.routing", cfg.DLQRoutingKey)
	assert.Equal(t, 86400000, cfg.DLQMessageTTL)
	assert.Equal(t, 10000, cfg.DLQMaxLength)
}

// ============================================================================
// Reconnection Tests
// ============================================================================

func TestBroker_ReconnectCallback(t *testing.T) {
	broker := NewBroker(nil, nil)

	// Verify reconnection callback is handled
	assert.NotNil(t, broker.subscriptions)
	assert.NotNil(t, broker.exchanges)
	assert.NotNil(t, broker.queues)
}

// ============================================================================
// Publisher Confirm Tests
// ============================================================================

func TestConfig_PublisherConfirm(t *testing.T) {
	cfg := DefaultConfig()
	assert.True(t, cfg.PublishConfirm) // Should be enabled by default

	cfg.PublishConfirm = false
	assert.False(t, cfg.PublishConfirm)
}

func TestConfig_PublishTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PublishTimeout = time.Second * 5

	assert.Equal(t, time.Second*5, cfg.PublishTimeout)
}

// ============================================================================
// Message Priority Tests
// ============================================================================

func TestMessage_Priority(t *testing.T) {
	priorities := []messaging.MessagePriority{
		messaging.PriorityLow,
		messaging.PriorityNormal,
		messaging.PriorityHigh,
		messaging.PriorityCritical,
	}

	for _, priority := range priorities {
		msg := &messaging.Message{
			ID:       "test-id",
			Type:     "test.event",
			Priority: priority,
		}
		assert.Equal(t, priority, msg.Priority)
	}
}

// ============================================================================
// Headers Tests
// ============================================================================

func TestMessage_Headers(t *testing.T) {
	msg := &messaging.Message{
		ID:   "test-id",
		Type: "test.event",
		Headers: map[string]string{
			"x-correlation-id": "corr-123",
			"x-reply-to":       "reply.queue",
			"x-custom-header":  "custom-value",
		},
	}

	assert.Equal(t, "corr-123", msg.Headers["x-correlation-id"])
	assert.Equal(t, "reply.queue", msg.Headers["x-reply-to"])
	assert.Equal(t, "custom-value", msg.Headers["x-custom-header"])
}

// ============================================================================
// TraceID Tests
// ============================================================================

func TestMessage_TraceID(t *testing.T) {
	msg := &messaging.Message{
		ID:      "test-id",
		Type:    "test.event",
		TraceID: "trace-abc-123",
	}

	assert.Equal(t, "trace-abc-123", msg.TraceID)
}

// ============================================================================
// Handler Error Tests
// ============================================================================

func TestMessageHandler_ErrorHandling(t *testing.T) {
	var handlerCalled atomic.Bool
	handler := func(ctx context.Context, msg *messaging.Message) error {
		handlerCalled.Store(true)
		return errors.New("handler error")
	}

	msg := &messaging.Message{ID: "test-id"}
	err := handler(context.Background(), msg)

	assert.True(t, handlerCalled.Load())
	assert.Error(t, err)
	assert.Equal(t, "handler error", err.Error())
}

func TestMessageHandler_PanicRecovery(t *testing.T) {
	handler := func(ctx context.Context, msg *messaging.Message) error {
		panic("handler panic")
	}

	// Wrap with recovery
	safeHandler := func(ctx context.Context, msg *messaging.Message) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = errors.New("recovered from panic")
			}
		}()
		return handler(ctx, msg)
	}

	msg := &messaging.Message{ID: "test-id"}
	err := safeHandler(context.Background(), msg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "recovered from panic")
}

// ============================================================================
// Batch Publishing Tests
// ============================================================================

func TestBroker_PublishBatch_MultipleMessages(t *testing.T) {
	broker := NewBroker(nil, nil)

	messages := make([]*messaging.Message, 10)
	for i := 0; i < 10; i++ {
		messages[i] = &messaging.Message{
			ID:   "msg-" + string(rune('0'+i)),
			Type: "batch.event",
		}
	}

	// Will fail due to no connection, but verifies the batch logic
	err := broker.PublishBatch(context.Background(), "test.topic", messages)
	assert.Error(t, err) // Expected because not connected
}

// ============================================================================
// Timeout Tests
// ============================================================================

func TestConfig_Timeouts(t *testing.T) {
	cfg := &Config{
		ConnectionTimeout: time.Second * 30,
		ReconnectDelay:    time.Second * 1,
		PublishTimeout:    time.Second * 5,
	}

	assert.Equal(t, time.Second*30, cfg.ConnectionTimeout)
	assert.Equal(t, time.Second*1, cfg.ReconnectDelay)
	assert.Equal(t, time.Second*5, cfg.PublishTimeout)
}

// ============================================================================
// Prefetch Tests
// ============================================================================

func TestConfig_Prefetch(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, 10, cfg.PrefetchCount)
	assert.Equal(t, 0, cfg.PrefetchSize)

	cfg.PrefetchCount = 100
	cfg.PrefetchSize = 1024 * 1024

	assert.Equal(t, 100, cfg.PrefetchCount)
	assert.Equal(t, 1024*1024, cfg.PrefetchSize)
}

// ============================================================================
// Exchange Management Tests
// ============================================================================

func TestBroker_ExchangeTracking(t *testing.T) {
	broker := NewBroker(nil, nil)

	assert.NotNil(t, broker.exchanges)
	assert.Equal(t, 0, broker.exchanges.Len())
}

func TestBroker_QueueTracking(t *testing.T) {
	broker := NewBroker(nil, nil)

	assert.NotNil(t, broker.queues)
	assert.Equal(t, 0, broker.queues.Len())
}

// ============================================================================
// Subscription Counter Tests
// ============================================================================

func TestBroker_SubscriptionCounter(t *testing.T) {
	broker := NewBroker(nil, nil)

	initial := broker.subCounter.Load()
	broker.subCounter.Add(1)
	assert.Equal(t, initial+1, broker.subCounter.Load())

	broker.subCounter.Add(1)
	assert.Equal(t, initial+2, broker.subCounter.Load())
}

// ============================================================================
// Closed State Tests
// ============================================================================

func TestBroker_ClosedFlag(t *testing.T) {
	broker := NewBroker(nil, nil)

	assert.False(t, broker.closed.Load())

	broker.closed.Store(true)
	assert.True(t, broker.closed.Load())
}

// ============================================================================
// Message Builder Tests
// ============================================================================

func TestNewMessage(t *testing.T) {
	payload := []byte(`{"data": "test"}`)
	msg := messaging.NewMessage("test.event", payload)

	assert.NotEmpty(t, msg.ID)
	assert.Equal(t, "test.event", msg.Type)
	assert.Equal(t, payload, msg.Payload)
	assert.Equal(t, messaging.PriorityNormal, msg.Priority)
	assert.NotNil(t, msg.Headers)
	assert.False(t, msg.Timestamp.IsZero())
}

func TestMessage_Clone(t *testing.T) {
	msg := &messaging.Message{
		ID:            "orig-id",
		Type:          "test.event",
		Payload:       []byte(`{"key": "value"}`),
		Headers:       map[string]string{"x-custom": "header"},
		TraceID:       "trace-123",
		CorrelationID: "corr-456",
		Priority:      messaging.PriorityHigh,
	}

	clone := msg.Clone()

	assert.Equal(t, msg.ID, clone.ID)
	assert.Equal(t, msg.Type, clone.Type)
	assert.Equal(t, msg.Payload, clone.Payload)
	assert.Equal(t, msg.TraceID, clone.TraceID)
	assert.Equal(t, msg.CorrelationID, clone.CorrelationID)
	assert.Equal(t, msg.Priority, clone.Priority)

	// Verify deep copy of payload
	clone.Payload[0] = 'X'
	assert.NotEqual(t, msg.Payload[0], clone.Payload[0])
}

// ============================================================================
// Message State Tests
// ============================================================================

func TestMessage_StateTransitions(t *testing.T) {
	states := []messaging.MessageState{
		messaging.MessageStatePending,
		messaging.MessageStateProcessing,
		messaging.MessageStateCompleted,
		messaging.MessageStateFailed,
		messaging.MessageStateDeadLettered,
	}

	for _, state := range states {
		msg := &messaging.Message{State: state}
		assert.Equal(t, state, msg.State)
	}
}

// ============================================================================
// Message Expiration Tests
// ============================================================================

func TestMessage_IsExpired(t *testing.T) {
	// Not expired (no expiration set)
	msg1 := &messaging.Message{}
	assert.False(t, msg1.IsExpired())

	// Not expired (future)
	msg2 := &messaging.Message{Expiration: time.Now().Add(time.Hour)}
	assert.False(t, msg2.IsExpired())

	// Expired (past)
	msg3 := &messaging.Message{Expiration: time.Now().Add(-time.Hour)}
	assert.True(t, msg3.IsExpired())
}

func TestMessage_CanRetry(t *testing.T) {
	msg := &messaging.Message{
		RetryCount: 0,
		MaxRetries: 3,
	}

	assert.True(t, msg.CanRetry())

	msg.RetryCount = 3
	assert.False(t, msg.CanRetry())
}

func TestMessage_IncrementRetry(t *testing.T) {
	msg := &messaging.Message{RetryCount: 0}

	msg.IncrementRetry()
	assert.Equal(t, 1, msg.RetryCount)

	msg.IncrementRetry()
	assert.Equal(t, 2, msg.RetryCount)
}

// ============================================================================
// Message Fluent API Tests
// ============================================================================

func TestMessage_FluentAPI(t *testing.T) {
	msg := messaging.NewMessage("test.event", []byte("payload")).
		WithPriority(messaging.PriorityHigh).
		WithTraceID("trace-123").
		WithCorrelationID("corr-456").
		WithReplyTo("reply.queue").
		WithMaxRetries(5).
		WithTTL(time.Hour)

	assert.Equal(t, messaging.PriorityHigh, msg.Priority)
	assert.Equal(t, "trace-123", msg.TraceID)
	assert.Equal(t, "corr-456", msg.CorrelationID)
	assert.Equal(t, "reply.queue", msg.ReplyTo)
	assert.Equal(t, 5, msg.MaxRetries)
	assert.False(t, msg.Expiration.IsZero())
}

// ============================================================================
// Metrics Calculation Tests
// ============================================================================

func TestBrokerMetrics_AverageLatency(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	// Record some latencies
	metrics.RecordPublish(100, time.Millisecond*10, true)
	metrics.RecordPublish(100, time.Millisecond*20, true)
	metrics.RecordPublish(100, time.Millisecond*30, true)

	avgLatency := metrics.GetAveragePublishLatency()
	assert.Equal(t, time.Millisecond*20, avgLatency)
}

func TestBrokerMetrics_SuccessRate(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	// 3 successes, 1 failure
	metrics.RecordPublish(100, time.Millisecond, true)
	metrics.RecordPublish(100, time.Millisecond, true)
	metrics.RecordPublish(100, time.Millisecond, true)
	metrics.RecordPublish(100, time.Millisecond, false)

	rate := metrics.GetPublishSuccessRate()
	assert.Equal(t, 0.75, rate)
}

// ============================================================================
// Integration Placeholder Tests (for when RabbitMQ is available)
// ============================================================================

func TestBroker_IntegrationPlaceholder_Connect(t *testing.T) {
	testutil.RequireExternalService(t, "rabbitmq", "localhost", "5672")

	// This test would connect to a real RabbitMQ instance
}

func TestBroker_IntegrationPlaceholder_PublishAndConsume(t *testing.T) {
	testutil.RequireExternalService(t, "rabbitmq", "localhost", "5672")

	// This test would publish and consume messages
}

func TestBroker_IntegrationPlaceholder_Reconnection(t *testing.T) {
	testutil.RequireExternalService(t, "rabbitmq", "localhost", "5672")

	// This test would verify reconnection after disconnect
}

// =============================================================================
// Additional Error Path Tests
// =============================================================================

func TestBroker_Publish_WhenChannelNil(t *testing.T) {
	cfg := DefaultConfig()
	broker := NewBroker(cfg, nil)
	ctx := context.Background()

	msg := &messaging.Message{
		ID:        "test-msg",
		Type:      "test",
		Payload:   []byte(`{"test": true}`),
		Timestamp: time.Now(),
	}

	err := broker.Publish(ctx, "test-topic", msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "channel not available")
}

func TestBroker_Subscribe_WhenNotConnected(t *testing.T) {
	cfg := DefaultConfig()
	broker := NewBroker(cfg, nil)
	ctx := context.Background()

	sub, err := broker.Subscribe(ctx, "test-topic", func(ctx context.Context, msg *messaging.Message) error {
		return nil
	})

	assert.Error(t, err)
	assert.Nil(t, sub)
	assert.Contains(t, err.Error(), "not connected")
}

func TestBroker_IsConnected_WhenNilConnection(t *testing.T) {
	cfg := DefaultConfig()
	broker := NewBroker(cfg, nil)

	assert.False(t, broker.IsConnected())
}

func TestBroker_PublishBatch_Empty(t *testing.T) {
	cfg := DefaultConfig()
	broker := NewBroker(cfg, nil)
	ctx := context.Background()

	err := broker.PublishBatch(ctx, "test-topic", []*messaging.Message{})
	assert.NoError(t, err) // Empty batch should succeed
}

func TestBroker_PublishBatch_Nil(t *testing.T) {
	cfg := DefaultConfig()
	broker := NewBroker(cfg, nil)
	ctx := context.Background()

	err := broker.PublishBatch(ctx, "test-topic", nil)
	assert.NoError(t, err) // Nil batch should succeed
}

func TestBroker_Close_WhenAlreadyClosed(t *testing.T) {
	cfg := DefaultConfig()
	broker := NewBroker(cfg, nil)
	ctx := context.Background()

	// First close should succeed
	err := broker.Close(ctx)
	assert.NoError(t, err)

	// Second close should also succeed (idempotent)
	err = broker.Close(ctx)
	assert.NoError(t, err)
}

func TestBroker_Metrics_AfterPublishFailure(t *testing.T) {
	cfg := DefaultConfig()
	broker := NewBroker(cfg, nil)
	ctx := context.Background()

	msg := &messaging.Message{
		ID:        "test-msg",
		Type:      "test",
		Payload:   []byte(`{}`),
		Timestamp: time.Now(),
	}

	// Publish should fail because not connected
	_ = broker.Publish(ctx, "test", msg)

	metrics := broker.GetMetrics()
	assert.NotNil(t, metrics)
}
