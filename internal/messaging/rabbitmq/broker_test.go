package rabbitmq

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"dev.helix.agent/internal/messaging"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, 5672, cfg.Port)
	assert.Equal(t, "guest", cfg.Username)
	assert.Equal(t, "guest", cfg.Password)
	assert.Equal(t, "/", cfg.VHost)
	assert.False(t, cfg.TLSEnabled)
	assert.Equal(t, 30*time.Second, cfg.ConnectionTimeout)
	assert.Equal(t, 1*time.Second, cfg.ReconnectDelay)
	assert.Equal(t, 0, cfg.MaxReconnectCount)
	assert.True(t, cfg.PublishConfirm)
	assert.Equal(t, 10, cfg.PrefetchCount)
	assert.True(t, cfg.DefaultQueueDurable)
	assert.False(t, cfg.DefaultQueueAutoDelete)
	assert.Equal(t, "topic", cfg.DefaultExchangeType)
	assert.True(t, cfg.DefaultExchangeDurable)
}

func TestDefaultQueueConfig(t *testing.T) {
	cfg := DefaultQueueConfig("test.queue")

	assert.Equal(t, "test.queue", cfg.Name)
	assert.True(t, cfg.Durable)
	assert.False(t, cfg.AutoDelete)
	assert.False(t, cfg.Exclusive)
	assert.False(t, cfg.NoWait)
	assert.NotNil(t, cfg.Args)
}

func TestDefaultExchangeConfig(t *testing.T) {
	cfg := DefaultExchangeConfig("test.exchange")

	assert.Equal(t, "test.exchange", cfg.Name)
	assert.Equal(t, "topic", cfg.Type)
	assert.True(t, cfg.Durable)
	assert.False(t, cfg.AutoDelete)
	assert.False(t, cfg.Internal)
	assert.False(t, cfg.NoWait)
	assert.NotNil(t, cfg.Args)
}

func TestQueueConfig_WithDLQ(t *testing.T) {
	cfg := DefaultQueueConfig("test.queue")
	cfg.WithDLQ("dlx.exchange", "dlx.key")

	assert.Equal(t, "dlx.exchange", cfg.DeadLetterExchange)
	assert.Equal(t, "dlx.key", cfg.DeadLetterRoutingKey)
	assert.Equal(t, "dlx.exchange", cfg.Args["x-dead-letter-exchange"])
	assert.Equal(t, "dlx.key", cfg.Args["x-dead-letter-routing-key"])
}

func TestQueueConfig_WithTTL(t *testing.T) {
	cfg := DefaultQueueConfig("test.queue")
	cfg.WithTTL(3600000)

	assert.Equal(t, 3600000, cfg.MessageTTL)
	assert.Equal(t, 3600000, cfg.Args["x-message-ttl"])
}

func TestQueueConfig_WithMaxLength(t *testing.T) {
	cfg := DefaultQueueConfig("test.queue")
	cfg.WithMaxLength(1000)

	assert.Equal(t, 1000, cfg.MaxLength)
	assert.Equal(t, 1000, cfg.Args["x-max-length"])
}

func TestQueueConfig_WithPriority(t *testing.T) {
	cfg := DefaultQueueConfig("test.queue")
	cfg.WithPriority(10)

	assert.Equal(t, 10, cfg.MaxPriority)
	assert.Equal(t, 10, cfg.Args["x-max-priority"])
}

func TestNewBroker(t *testing.T) {
	// With nil config
	broker := NewBroker(nil, nil)
	assert.NotNil(t, broker)
	assert.NotNil(t, broker.config)
	assert.Equal(t, "localhost", broker.config.Host)

	// With custom config
	cfg := &Config{
		Host: "custom-host",
		Port: 5673,
	}
	broker2 := NewBroker(cfg, nil)
	assert.Equal(t, "custom-host", broker2.config.Host)
	assert.Equal(t, 5673, broker2.config.Port)
}

func TestBroker_BrokerType(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.Equal(t, messaging.BrokerTypeRabbitMQ, broker.BrokerType())
}

func TestBroker_IsConnected_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.False(t, broker.IsConnected())
}

func TestBroker_GetMetrics(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()
	assert.NotNil(t, metrics)
}

func TestConnectionState_String(t *testing.T) {
	tests := []struct {
		state    ConnectionState
		expected string
	}{
		{StateDisconnected, "disconnected"},
		{StateConnecting, "connecting"},
		{StateConnected, "connected"},
		{StateReconnecting, "reconnecting"},
		{StateClosed, "closed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

// ============================================================================
// Additional Broker Lifecycle Tests
// ============================================================================

func TestBroker_Close_MultipleCallsIdempotent(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	// First close
	err1 := broker.Close(ctx)
	assert.NoError(t, err1)

	// Second close should be idempotent (no error)
	err2 := broker.Close(ctx)
	assert.NoError(t, err2)

	// Third close should also be idempotent
	err3 := broker.Close(ctx)
	assert.NoError(t, err3)

	// Verify closed flag is still set
	assert.True(t, broker.closed.Load())
}

func TestBroker_Close_WithSubscriptions(t *testing.T) {
	broker := NewBroker(nil, nil)

	// Add a mock subscription to the internal map
	sub := &rabbitSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		queue:    "test.queue",
		cancelCh: make(chan struct{}),
	}
	sub.active.Store(true)
	broker.subscriptions.Put("test.topic", sub)

	ctx := context.Background()
	err := broker.Close(ctx)
	assert.NoError(t, err)

	// Verify subscription was deactivated
	assert.False(t, sub.active.Load())
}

// ============================================================================
// rabbitSubscription Tests
// ============================================================================

func TestRabbitSubscription_Unsubscribe_VerifyCancelChannelClosed(t *testing.T) {
	sub := &rabbitSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		queue:    "test.queue",
		cancelCh: make(chan struct{}),
		channel:  nil,
	}
	sub.active.Store(true)

	err := sub.Unsubscribe()
	assert.NoError(t, err)
	assert.False(t, sub.active.Load())

	// Verify cancel channel is closed by trying to receive
	select {
	case <-sub.cancelCh:
		// Channel was closed, this is expected
	default:
		t.Error("cancel channel should be closed")
	}
}

func TestRabbitSubscription_Unsubscribe_MultipleCallsIdempotent(t *testing.T) {
	sub := &rabbitSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		queue:    "test.queue",
		cancelCh: make(chan struct{}),
	}
	sub.active.Store(true)

	// First unsubscribe
	err1 := sub.Unsubscribe()
	assert.NoError(t, err1)

	// Second unsubscribe should be idempotent
	err2 := sub.Unsubscribe()
	assert.NoError(t, err2)

	// Third unsubscribe should also be idempotent
	err3 := sub.Unsubscribe()
	assert.NoError(t, err3)
}

func TestRabbitSubscription_IsActive(t *testing.T) {
	sub := &rabbitSubscription{
		id:    "sub-1",
		topic: "test.topic",
	}

	// Initially not active
	sub.active.Store(false)
	assert.False(t, sub.IsActive())

	// Set active
	sub.active.Store(true)
	assert.True(t, sub.IsActive())

	// Set inactive again
	sub.active.Store(false)
	assert.False(t, sub.IsActive())
}

func TestRabbitSubscription_Topic(t *testing.T) {
	sub := &rabbitSubscription{
		id:    "sub-1",
		topic: "orders.created",
		queue: "orders-queue",
	}

	assert.Equal(t, "orders.created", sub.Topic())
}

func TestRabbitSubscription_ID(t *testing.T) {
	sub := &rabbitSubscription{
		id:    "sub-42",
		topic: "test.topic",
	}

	assert.Equal(t, "sub-42", sub.ID())
}

func TestRabbitSubscription_ImplementsInterface(t *testing.T) {
	var _ messaging.Subscription = (*rabbitSubscription)(nil)
}

// ============================================================================
// Message Handling Tests (Without Real Connection)
// ============================================================================

func TestBroker_Publish_WhenNoChannel(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	msg := messaging.NewMessage("test.type", []byte(`{"key": "value"}`))

	err := broker.Publish(ctx, "test.topic", msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "publisher channel not available")

	// Verify metrics recorded the failure
	metrics := broker.GetMetrics()
	assert.Equal(t, int64(1), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(1), metrics.PublishFailures.Load())
}

func TestBroker_PublishBatch(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	messages := []*messaging.Message{
		messaging.NewMessage("type1", []byte(`{"msg": 1}`)),
		messaging.NewMessage("type2", []byte(`{"msg": 2}`)),
		messaging.NewMessage("type3", []byte(`{"msg": 3}`)),
	}

	err := broker.PublishBatch(ctx, "test.topic", messages)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "publisher channel not available")

	// First message should have been attempted
	metrics := broker.GetMetrics()
	assert.Equal(t, int64(1), metrics.MessagesPublished.Load())
}

// ============================================================================
// DLQ Configuration Tests
// ============================================================================

func TestConfig_WithDLQEnabled(t *testing.T) {
	cfg := DefaultConfig()

	assert.True(t, cfg.EnableDLQ)
	assert.Equal(t, "helixagent.dlq", cfg.DLQExchange)
	assert.Equal(t, "dlq", cfg.DLQRoutingKey)
	assert.Equal(t, 86400000, cfg.DLQMessageTTL) // 24 hours in milliseconds
	assert.Equal(t, 100000, cfg.DLQMaxLength)
}

func TestConfig_DLQSettings_Custom(t *testing.T) {
	cfg := &Config{
		Host:          "localhost",
		Port:          5672,
		Username:      "guest",
		Password:      "guest",
		VHost:         "/",
		EnableDLQ:     true,
		DLQExchange:   "custom.dlq.exchange",
		DLQRoutingKey: "custom.dlq.key",
		DLQMessageTTL: 3600000, // 1 hour
		DLQMaxLength:  10000,
	}

	assert.True(t, cfg.EnableDLQ)
	assert.Equal(t, "custom.dlq.exchange", cfg.DLQExchange)
	assert.Equal(t, "custom.dlq.key", cfg.DLQRoutingKey)
	assert.Equal(t, 3600000, cfg.DLQMessageTTL)
	assert.Equal(t, 10000, cfg.DLQMaxLength)
}

func TestConfig_DLQDisabled(t *testing.T) {
	cfg := &Config{
		Host:      "localhost",
		Port:      5672,
		Username:  "guest",
		Password:  "guest",
		VHost:     "/",
		EnableDLQ: false,
	}

	assert.False(t, cfg.EnableDLQ)
}

func TestQueueConfig_WithDLQ_NilArgs(t *testing.T) {
	cfg := &QueueConfig{
		Name: "test.queue",
		Args: nil,
	}

	// WithDLQ should initialize Args if nil
	cfg.WithDLQ("dlx.exchange", "dlx.key")

	assert.NotNil(t, cfg.Args)
	assert.Equal(t, "dlx.exchange", cfg.Args["x-dead-letter-exchange"])
	assert.Equal(t, "dlx.key", cfg.Args["x-dead-letter-routing-key"])
}

func TestQueueConfig_WithTTL_NilArgs(t *testing.T) {
	cfg := &QueueConfig{
		Name: "test.queue",
		Args: nil,
	}

	cfg.WithTTL(60000)

	assert.NotNil(t, cfg.Args)
	assert.Equal(t, 60000, cfg.Args["x-message-ttl"])
}

func TestQueueConfig_WithMaxLength_NilArgs(t *testing.T) {
	cfg := &QueueConfig{
		Name: "test.queue",
		Args: nil,
	}

	cfg.WithMaxLength(500)

	assert.NotNil(t, cfg.Args)
	assert.Equal(t, 500, cfg.Args["x-max-length"])
}

func TestQueueConfig_WithPriority_NilArgs(t *testing.T) {
	cfg := &QueueConfig{
		Name: "test.queue",
		Args: nil,
	}

	cfg.WithPriority(5)

	assert.NotNil(t, cfg.Args)
	assert.Equal(t, 5, cfg.Args["x-max-priority"])
}

// ============================================================================
// Metrics Tracking Tests
// ============================================================================

func TestBroker_MetricsRecording_InitialState(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	assert.Equal(t, int64(0), metrics.ConnectionAttempts.Load())
	assert.Equal(t, int64(0), metrics.ConnectionSuccesses.Load())
	assert.Equal(t, int64(0), metrics.ConnectionFailures.Load())
	assert.Equal(t, int64(0), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(0), metrics.PublishSuccesses.Load())
	assert.Equal(t, int64(0), metrics.PublishFailures.Load())
	assert.Equal(t, int64(0), metrics.MessagesReceived.Load())
	assert.Equal(t, int64(0), metrics.TotalErrors.Load())
}

func TestBroker_MetricsRecording_PublishFailure(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	msg := messaging.NewMessage("test.type", []byte(`test`))
	_ = broker.Publish(ctx, "test.topic", msg)

	metrics := broker.GetMetrics()
	assert.Equal(t, int64(1), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(0), metrics.PublishSuccesses.Load())
	assert.Equal(t, int64(1), metrics.PublishFailures.Load())
	assert.True(t, metrics.PublishLatencyCount.Load() >= 1)
}

func TestBroker_MetricsRecording_MultiplePublishFailures(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		msg := messaging.NewMessage("test.type", []byte(`test`))
		_ = broker.Publish(ctx, "test.topic", msg)
	}

	metrics := broker.GetMetrics()
	assert.Equal(t, int64(5), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(5), metrics.PublishFailures.Load())
	assert.Equal(t, int64(0), metrics.PublishSuccesses.Load())
}

func TestBroker_MetricsRecording_PublishWithOptions(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	msg := messaging.NewMessage("test.type", []byte(`test`))
	_ = broker.Publish(ctx, "test.topic", msg,
		messaging.WithExchange("custom.exchange"),
		messaging.WithRoutingKey("custom.key"),
	)

	metrics := broker.GetMetrics()
	assert.Equal(t, int64(1), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(1), metrics.PublishFailures.Load())
}

func TestMetrics_Clone(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	// Set some values
	metrics.ConnectionAttempts.Store(10)
	metrics.MessagesPublished.Store(100)
	metrics.PublishSuccesses.Store(95)
	metrics.PublishFailures.Store(5)

	// Clone
	clone := metrics.Clone()

	// Verify clone has same values
	assert.Equal(t, int64(10), clone.ConnectionAttempts.Load())
	assert.Equal(t, int64(100), clone.MessagesPublished.Load())
	assert.Equal(t, int64(95), clone.PublishSuccesses.Load())
	assert.Equal(t, int64(5), clone.PublishFailures.Load())

	// Verify clone is independent
	metrics.ConnectionAttempts.Store(20)
	assert.Equal(t, int64(10), clone.ConnectionAttempts.Load())
}

func TestMetrics_Reset(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	// Set some values
	metrics.ConnectionAttempts.Store(10)
	metrics.MessagesPublished.Store(100)
	metrics.TotalErrors.Store(5)

	// Reset
	metrics.Reset()

	// Verify all values are reset
	assert.Equal(t, int64(0), metrics.ConnectionAttempts.Load())
	assert.Equal(t, int64(0), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(0), metrics.TotalErrors.Load())
}

func TestMetrics_GetAverageSubscribeLatency(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	// No latency recorded yet
	assert.Equal(t, time.Duration(0), metrics.GetAverageSubscribeLatency())

	// Record some latency
	metrics.SubscribeLatencyTotal.Store(int64(300 * time.Millisecond))
	metrics.SubscribeLatencyCount.Store(3)

	avg := metrics.GetAverageSubscribeLatency()
	assert.Equal(t, 100*time.Millisecond, avg)
}

func TestMetrics_RecordReceive(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordReceive(256, 10*time.Millisecond)

	assert.Equal(t, int64(1), metrics.MessagesReceived.Load())
	assert.Equal(t, int64(256), metrics.BytesReceived.Load())
	assert.True(t, metrics.SubscribeLatencyCount.Load() >= 1)
}

func TestMetrics_RecordSubscription(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordSubscription()
	metrics.RecordSubscription()
	assert.Equal(t, int64(2), metrics.ActiveSubscriptions.Load())

	metrics.RecordUnsubscription()
	assert.Equal(t, int64(1), metrics.ActiveSubscriptions.Load())
}

func TestMetrics_RecordQueueDeclared(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordQueueDeclared()
	metrics.RecordQueueDeclared()

	assert.Equal(t, int64(2), metrics.QueuesDeclared.Load())
}

func TestMetrics_RecordSerializationError(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordSerializationError()

	assert.Equal(t, int64(1), metrics.SerializationErrors.Load())
	assert.Equal(t, int64(1), metrics.TotalErrors.Load())
}

func TestMetrics_RecordTopicCreated(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordTopicCreated()
	metrics.RecordTopicCreated()

	assert.Equal(t, int64(2), metrics.TopicsCreated.Load())
}

func TestMetrics_RecordConnectionAttempt(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordConnectionAttempt()
	metrics.RecordConnectionAttempt()

	assert.Equal(t, int64(2), metrics.ConnectionAttempts.Load())
}

func TestMetrics_RecordConnectionSuccess(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordConnectionSuccess()

	assert.Equal(t, int64(1), metrics.ConnectionSuccesses.Load())
	assert.Equal(t, int64(1), metrics.CurrentConnections.Load())
}

func TestMetrics_RecordConnectionFailure(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordConnectionFailure()

	assert.Equal(t, int64(1), metrics.ConnectionFailures.Load())
	assert.Equal(t, int64(1), metrics.ConnectionErrors.Load())
	assert.Equal(t, int64(1), metrics.TotalErrors.Load())
}

func TestMetrics_RecordDisconnection(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()
	metrics.CurrentConnections.Store(5)

	metrics.RecordDisconnection()

	assert.Equal(t, int64(4), metrics.CurrentConnections.Load())
}

func TestMetrics_RecordReconnectionAttempt(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordReconnectionAttempt()

	assert.Equal(t, int64(1), metrics.ReconnectionAttempts.Load())
	assert.False(t, metrics.GetLastPublishTime().IsZero() && metrics.LastReconnectTime.IsZero())
}

func TestMetrics_RecordPublishConfirmation(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordPublishConfirmation()
	metrics.RecordPublishConfirmation()

	assert.Equal(t, int64(2), metrics.PublishConfirmations.Load())
}

func TestMetrics_RecordPublishTimeout(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordPublishTimeout()

	assert.Equal(t, int64(1), metrics.PublishTimeouts.Load())
	assert.Equal(t, int64(1), metrics.PublishErrors.Load())
	assert.Equal(t, int64(1), metrics.TotalErrors.Load())
}

func TestMetrics_RecordConsume(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordConsume(512, 20*time.Millisecond, true)

	assert.Equal(t, int64(1), metrics.MessagesConsumed.Load())
	assert.Equal(t, int64(512), metrics.BytesConsumed.Load())
}

func TestMetrics_RecordRetry(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordRetry()
	metrics.RecordRetry()

	assert.Equal(t, int64(2), metrics.MessagesRetried.Load())
}

func TestMetrics_RecordDeadLettered(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	metrics.RecordDeadLettered()

	assert.Equal(t, int64(1), metrics.MessagesDeadLettered.Load())
}

func TestMetrics_GetProcessingSuccessRate(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	// No messages received yet - should return 1.0
	assert.Equal(t, 1.0, metrics.GetProcessingSuccessRate())

	// Record some messages
	metrics.MessagesReceived.Store(100)
	metrics.MessagesProcessed.Store(80)

	rate := metrics.GetProcessingSuccessRate()
	assert.Equal(t, 0.8, rate)
}

func TestMetrics_GetUptime(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	// Should have non-zero uptime
	uptime := metrics.GetUptime()
	assert.True(t, uptime >= 0)
}

func TestMetrics_GetLastReceiveTime(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	// Initially zero
	lastReceive := metrics.GetLastReceiveTime()
	assert.True(t, lastReceive.IsZero())

	// Record a receive
	metrics.RecordReceive(100, time.Millisecond)

	lastReceive = metrics.GetLastReceiveTime()
	assert.False(t, lastReceive.IsZero())
}

func TestMetrics_GetLastErrorTime(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	// Initially zero
	lastError := metrics.GetLastErrorTime()
	assert.True(t, lastError.IsZero())

	// Record an error
	metrics.RecordError()

	lastError = metrics.GetLastErrorTime()
	assert.False(t, lastError.IsZero())
}

// ============================================================================
// Config Validation Tests
// ============================================================================

func TestConfig_Validate_EmptyUsername(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Username = ""

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "username is required")
}

func TestConfig_Validate_InvalidMaxConnections(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxConnections = 0

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_connections must be at least 1")
}

func TestConfig_Validate_InvalidMaxChannels(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxChannels = 0

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_channels must be at least 1")
}

func TestConfig_Validate_NegativeConnectionTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ConnectionTimeout = -1 * time.Second

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection_timeout cannot be negative")
}

// ============================================================================
// Connection Tests
// ============================================================================

func TestConnection_NewConnection(t *testing.T) {
	conn := NewConnection(nil, nil)

	assert.NotNil(t, conn)
	assert.NotNil(t, conn.config)
	assert.NotNil(t, conn.logger)
	assert.Equal(t, StateDisconnected, conn.State())
}

func TestConnection_NewConnection_WithConfig(t *testing.T) {
	cfg := &Config{
		Host: "custom-host",
		Port: 5673,
	}

	conn := NewConnection(cfg, nil)

	assert.Equal(t, "custom-host", conn.config.Host)
	assert.Equal(t, 5673, conn.config.Port)
}

func TestConnection_State(t *testing.T) {
	conn := NewConnection(nil, nil)

	assert.Equal(t, StateDisconnected, conn.State())

	conn.state.Store(int32(StateConnecting))
	assert.Equal(t, StateConnecting, conn.State())

	conn.state.Store(int32(StateConnected))
	assert.Equal(t, StateConnected, conn.State())
}

func TestConnection_IsConnected(t *testing.T) {
	conn := NewConnection(nil, nil)

	assert.False(t, conn.IsConnected())

	conn.state.Store(int32(StateConnected))
	assert.True(t, conn.IsConnected())

	conn.state.Store(int32(StateDisconnected))
	assert.False(t, conn.IsConnected())
}

func TestConnection_ReconnectCount(t *testing.T) {
	conn := NewConnection(nil, nil)

	assert.Equal(t, int64(0), conn.ReconnectCount())
}

func TestConnection_OnConnect(t *testing.T) {
	conn := NewConnection(nil, nil)

	called := false
	conn.OnConnect(func() {
		called = true
	})

	assert.Equal(t, 1, conn.onConnect.Len())

	// Simulate callback
	cb, ok := conn.onConnect.At(0)
	assert.True(t, ok)
	cb()
	assert.True(t, called)
}

func TestConnection_OnDisconnect(t *testing.T) {
	conn := NewConnection(nil, nil)

	var receivedErr error
	conn.OnDisconnect(func(err error) {
		receivedErr = err
	})

	assert.Equal(t, 1, conn.onDisconnect.Len())

	// Simulate callback
	testErr := assert.AnError
	cb, ok := conn.onDisconnect.At(0)
	assert.True(t, ok)
	cb(testErr)
	assert.Equal(t, testErr, receivedErr)
}

func TestConnection_OnReconnect(t *testing.T) {
	conn := NewConnection(nil, nil)

	called := false
	conn.OnReconnect(func() {
		called = true
	})

	assert.Equal(t, 1, conn.onReconnect.Len())

	// Simulate callback
	cb, ok := conn.onReconnect.At(0)
	assert.True(t, ok)
	cb()
	assert.True(t, called)
}

func TestConnection_Channel_WhenNotConnected(t *testing.T) {
	conn := NewConnection(nil, nil)

	ch, err := conn.Channel()
	assert.Error(t, err)
	assert.Nil(t, ch)
	assert.Contains(t, err.Error(), "connection is not available")
}

func TestConnection_GetConnection_WhenNotConnected(t *testing.T) {
	conn := NewConnection(nil, nil)

	amqpConn := conn.GetConnection()
	assert.Nil(t, amqpConn)
}

func TestConnection_Close_WhenNotConnected(t *testing.T) {
	conn := NewConnection(nil, nil)

	err := conn.Close()
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, conn.State())
}

func TestConnection_Close_MultipleCallsIdempotent(t *testing.T) {
	conn := NewConnection(nil, nil)

	err1 := conn.Close()
	assert.NoError(t, err1)

	err2 := conn.Close()
	assert.NoError(t, err2)

	assert.Equal(t, StateClosed, conn.State())
}

func TestConnectionState_String_Unknown(t *testing.T) {
	state := ConnectionState(999)
	assert.Equal(t, "unknown", state.String())
}

// ============================================================================
// Concurrent Access Tests
// ============================================================================

func TestBroker_ConcurrentClose(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Close from multiple goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := broker.Close(ctx)
			errors <- err
		}()
	}

	wg.Wait()
	close(errors)

	// All should succeed (idempotent)
	for err := range errors {
		assert.NoError(t, err)
	}
}

func TestBroker_ConcurrentPublish(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	var wg sync.WaitGroup

	// Publish from multiple goroutines (all should fail since not connected)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := messaging.NewMessage("test.type", []byte(`test`))
			_ = broker.Publish(ctx, "test.topic", msg)
		}(i)
	}

	wg.Wait()

	// All publishes should have been attempted
	metrics := broker.GetMetrics()
	assert.Equal(t, int64(10), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(10), metrics.PublishFailures.Load())
}

func TestBroker_ConcurrentIsConnected(t *testing.T) {
	broker := NewBroker(nil, nil)

	var wg sync.WaitGroup

	// Check IsConnected from multiple goroutines
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = broker.IsConnected()
		}()
	}

	wg.Wait()
}

func TestBroker_ConcurrentGetMetrics(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	var wg sync.WaitGroup

	// Concurrently publish (which records metrics) and get metrics
	for i := 0; i < 50; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			msg := messaging.NewMessage("test.type", []byte(`test`))
			_ = broker.Publish(ctx, "test.topic", msg)
		}()

		go func() {
			defer wg.Done()
			_ = broker.GetMetrics()
		}()
	}

	wg.Wait()
}

// ============================================================================
// Edge Cases and Error Handling Tests
// ============================================================================

func TestBroker_PublishWithNilMessage(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	// This should fail at the channel check before message handling
	err := broker.Publish(ctx, "test.topic", nil)
	assert.Error(t, err)
}

func TestBroker_PublishBatchWithNilInSlice(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	messages := []*messaging.Message{
		messaging.NewMessage("type1", []byte(`test`)),
		nil,
		messaging.NewMessage("type2", []byte(`test`)),
	}

	// Should fail at first message due to no channel
	err := broker.PublishBatch(ctx, "test.topic", messages)
	assert.Error(t, err)
}

func TestBroker_PublishWithCancelledContext(t *testing.T) {
	broker := NewBroker(nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	msg := messaging.NewMessage("test.type", []byte(`test`))
	err := broker.Publish(ctx, "test.topic", msg)

	// Should fail at channel check before context is used
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "publisher channel not available")
}

// ============================================================================
// Additional tests for NewBroker
// ============================================================================

func TestNewBroker_WithLogger(t *testing.T) {
	logger := zap.NewNop()
	broker := NewBroker(nil, logger)
	assert.NotNil(t, broker)
	assert.Equal(t, logger, broker.logger)
}

func TestNewBroker_InitializesInternalMaps(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.NotNil(t, broker.subscriptions)
	assert.NotNil(t, broker.exchanges)
	assert.NotNil(t, broker.queues)
	assert.NotNil(t, broker.metrics)
	assert.Equal(t, 0, broker.subscriptions.Len())
	assert.Equal(t, 0, broker.exchanges.Len())
	assert.Equal(t, 0, broker.queues.Len())
}

// ============================================================================
// MetricsSnapshot Tests
// ============================================================================

func TestNewMetricsSnapshot(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()
	metrics.MessagesPublished.Store(100)

	snapshot := messaging.NewMetricsSnapshot(messaging.BrokerTypeRabbitMQ, metrics)

	assert.Equal(t, messaging.BrokerTypeRabbitMQ, snapshot.BrokerType)
	assert.NotNil(t, snapshot.Metrics)
	assert.Equal(t, int64(100), snapshot.Metrics.MessagesPublished.Load())
	assert.False(t, snapshot.CollectedAt.IsZero())

	// Verify snapshot is independent of original
	metrics.MessagesPublished.Store(200)
	assert.Equal(t, int64(100), snapshot.Metrics.MessagesPublished.Load())
}

func TestMetricsSnapshot_WithQueueStats(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()
	snapshot := messaging.NewMetricsSnapshot(messaging.BrokerTypeRabbitMQ, metrics)

	queueStats := []messaging.QueueStats{
		{Name: "queue1", Messages: 100},
		{Name: "queue2", Messages: 200},
	}

	result := snapshot.WithQueueStats(queueStats)

	assert.Equal(t, snapshot, result) // Returns same snapshot for chaining
	assert.Len(t, snapshot.QueueStats, 2)
	assert.Equal(t, "queue1", snapshot.QueueStats[0].Name)
}

func TestMetricsSnapshot_WithTopicStats(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()
	snapshot := messaging.NewMetricsSnapshot(messaging.BrokerTypeRabbitMQ, metrics)

	topicStats := []messaging.TopicMetadata{
		{Name: "topic1", Partitions: 3},
		{Name: "topic2", Partitions: 6},
	}

	result := snapshot.WithTopicStats(topicStats)

	assert.Equal(t, snapshot, result)
	assert.Len(t, snapshot.TopicStats, 2)
	assert.Equal(t, "topic1", snapshot.TopicStats[0].Name)
}

// ============================================================================
// MetricsCollector Tests
// ============================================================================

func TestMetricsCollector_RegisterUnregister(t *testing.T) {
	collector := messaging.NewMetricsCollector(time.Second)
	broker := NewBroker(nil, nil)

	collector.Register("test-broker", broker)

	// Collect metrics
	snapshots := collector.Collect()
	assert.Len(t, snapshots, 1)

	// Unregister
	collector.Unregister("test-broker")

	snapshots = collector.Collect()
	assert.Len(t, snapshots, 0)
}

func TestMetricsCollector_Collect(t *testing.T) {
	collector := messaging.NewMetricsCollector(time.Second)

	broker1 := NewBroker(nil, nil)
	broker2 := NewBroker(nil, nil)

	collector.Register("broker1", broker1)
	collector.Register("broker2", broker2)

	snapshots := collector.Collect()
	assert.Len(t, snapshots, 2)
}

func TestMetricsCollector_GetSnapshots(t *testing.T) {
	collector := messaging.NewMetricsCollector(time.Second)

	snapshots := collector.GetSnapshots()
	assert.Empty(t, snapshots)
}

// ============================================================================
// BrokerType Tests
// ============================================================================

func TestBrokerType_String(t *testing.T) {
	assert.Equal(t, "rabbitmq", messaging.BrokerTypeRabbitMQ.String())
	assert.Equal(t, "kafka", messaging.BrokerTypeKafka.String())
	assert.Equal(t, "inmemory", messaging.BrokerTypeInMemory.String())
}

func TestBrokerType_IsValid(t *testing.T) {
	assert.True(t, messaging.BrokerTypeRabbitMQ.IsValid())
	assert.True(t, messaging.BrokerTypeKafka.IsValid())
	assert.True(t, messaging.BrokerTypeInMemory.IsValid())
	assert.False(t, messaging.BrokerType("invalid").IsValid())
}

// ============================================================================
// QueueStats Tests
// ============================================================================

func TestQueueStats_Fields(t *testing.T) {
	stats := messaging.QueueStats{
		Name:            "test-queue",
		Messages:        100,
		Consumers:       5,
		MessagesReady:   80,
		MessagesUnacked: 20,
		MessageBytes:    1024,
		PublishRate:     10.5,
		DeliverRate:     9.8,
		Timestamp:       time.Now(),
	}

	assert.Equal(t, "test-queue", stats.Name)
	assert.Equal(t, int64(100), stats.Messages)
	assert.Equal(t, int64(5), stats.Consumers)
	assert.Equal(t, int64(80), stats.MessagesReady)
	assert.Equal(t, int64(20), stats.MessagesUnacked)
}

// ============================================================================
// TopicMetadata Tests
// ============================================================================

func TestTopicMetadata_Fields(t *testing.T) {
	metadata := messaging.TopicMetadata{
		Name:              "test-topic",
		Partitions:        12,
		ReplicationFactor: 3,
		RetentionMs:       86400000,
		CleanupPolicy:     "delete",
	}

	assert.Equal(t, "test-topic", metadata.Name)
	assert.Equal(t, 12, metadata.Partitions)
	assert.Equal(t, 3, metadata.ReplicationFactor)
	assert.Equal(t, int64(86400000), metadata.RetentionMs)
	assert.Equal(t, "delete", metadata.CleanupPolicy)
}

// ============================================================================
// PartitionInfo Tests
// ============================================================================

func TestPartitionInfo_Fields(t *testing.T) {
	info := messaging.PartitionInfo{
		ID:            0,
		Leader:        1,
		Replicas:      []int32{1, 2, 3},
		ISR:           []int32{1, 2},
		HighWatermark: 1000,
		LowWatermark:  100,
	}

	assert.Equal(t, int32(0), info.ID)
	assert.Equal(t, int32(1), info.Leader)
	assert.Equal(t, []int32{1, 2, 3}, info.Replicas)
	assert.Equal(t, []int32{1, 2}, info.ISR)
	assert.Equal(t, int64(1000), info.HighWatermark)
	assert.Equal(t, int64(100), info.LowWatermark)
}

// ============================================================================
// Additional Coverage Tests - Error Paths and Edge Cases (Part 2)
// ============================================================================

func TestBroker_SubCounter_Increments(t *testing.T) {
	broker := NewBroker(nil, nil)

	assert.Equal(t, int64(0), broker.subCounter.Load())

	v1 := broker.subCounter.Add(1)
	v2 := broker.subCounter.Add(1)
	v3 := broker.subCounter.Add(1)

	assert.Equal(t, int64(1), v1)
	assert.Equal(t, int64(2), v2)
	assert.Equal(t, int64(3), v3)
	assert.Equal(t, int64(3), broker.subCounter.Load())
}

func TestRabbitSubscription_AllFields(t *testing.T) {
	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	sub := &rabbitSubscription{
		id:       "sub-123",
		topic:    "orders.created",
		queue:    "orders-consumer-queue",
		handler:  handler,
		channel:  nil,
		consumer: "consumer-tag-123",
		cancelCh: make(chan struct{}),
	}
	sub.active.Store(true)

	assert.Equal(t, "sub-123", sub.ID())
	assert.Equal(t, "orders.created", sub.Topic())
	assert.Equal(t, "orders-consumer-queue", sub.queue)
	assert.NotNil(t, sub.handler)
	assert.Equal(t, "consumer-tag-123", sub.consumer)
	assert.True(t, sub.IsActive())
}

func TestRabbitSubscription_Unsubscribe_WhenInactive(t *testing.T) {
	sub := &rabbitSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		cancelCh: make(chan struct{}),
	}
	sub.active.Store(false) // Already inactive

	// Should return immediately without error
	err := sub.Unsubscribe()
	assert.NoError(t, err)
}

func TestRabbitSubscription_ConcurrentUnsubscribe(t *testing.T) {
	sub := &rabbitSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		cancelCh: make(chan struct{}),
	}
	sub.active.Store(true)

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := sub.Unsubscribe()
			errors <- err
		}()
	}

	wg.Wait()
	close(errors)

	// All should succeed
	for err := range errors {
		assert.NoError(t, err)
	}

	assert.False(t, sub.IsActive())
}

func TestBroker_PublishBatch_EmptySlice(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	err := broker.PublishBatch(ctx, "test.topic", []*messaging.Message{})
	// Should succeed with empty slice (no messages to publish)
	assert.NoError(t, err)
}

func TestBroker_Publish_MessageWithPriority(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	msg := messaging.NewMessage("test.type", []byte(`{"key": "value"}`))
	msg.Priority = 5

	err := broker.Publish(ctx, "test.topic", msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "publisher channel not available")
}

func TestBroker_Publish_MessageWithTraceID(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	msg := messaging.NewMessage("test.type", []byte(`{"key": "value"}`))
	msg.TraceID = "trace-abc-123"

	err := broker.Publish(ctx, "test.topic", msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "publisher channel not available")
}

func TestBroker_Publish_MessageWithHeaders(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	msg := messaging.NewMessage("test.type", []byte(`{"key": "value"}`))
	msg.Headers = map[string]string{
		"header1": "value1",
		"header2": "value2",
	}

	err := broker.Publish(ctx, "test.topic", msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "publisher channel not available")
}

func TestConfig_AllSettingsCustom(t *testing.T) {
	cfg := &Config{
		Host:                   "rabbitmq.example.com",
		Port:                   5673,
		Username:               "myuser",
		Password:               "mypassword",
		VHost:                  "/production",
		TLSEnabled:             true,
		TLSSkipVerify:          true,
		ConnectionTimeout:      60 * time.Second,
		ReconnectDelay:         5 * time.Second,
		MaxReconnectCount:      10,
		PublishConfirm:         true,
		PublishTimeout:         30 * time.Second,
		PrefetchCount:          20,
		PrefetchSize:           1048576,
		DefaultQueueDurable:    true,
		DefaultQueueAutoDelete: false,
		DefaultQueueExclusive:  false,
		DefaultExchangeType:    "direct",
		DefaultExchangeDurable: true,
		AutoAck:                false,
		Exclusive:              false,
		NoLocal:                false,
		NoWait:                 false,
		MandatoryPublish:       true,
		ImmediatePublish:       false,
		EnableDLQ:              true,
		DLQExchange:            "custom.dlq",
		DLQRoutingKey:          "custom.key",
		DLQMessageTTL:          7200000,
		DLQMaxLength:           50000,
		MaxConnections:         10,
		MaxChannels:            100,
		HeartbeatInterval:      30 * time.Second,
		EnableMetrics:          true,
		MetricsPrefix:          "rabbitmq",
	}

	assert.Equal(t, "rabbitmq.example.com", cfg.Host)
	assert.Equal(t, 5673, cfg.Port)
	assert.Equal(t, "myuser", cfg.Username)
	assert.Equal(t, "mypassword", cfg.Password)
	assert.Equal(t, "/production", cfg.VHost)
	assert.True(t, cfg.TLSEnabled)
	assert.True(t, cfg.TLSSkipVerify)
	assert.Equal(t, 60*time.Second, cfg.ConnectionTimeout)
	assert.Equal(t, 5*time.Second, cfg.ReconnectDelay)
	assert.Equal(t, 10, cfg.MaxReconnectCount)
	assert.True(t, cfg.PublishConfirm)
	assert.Equal(t, 30*time.Second, cfg.PublishTimeout)
	assert.Equal(t, 20, cfg.PrefetchCount)
	assert.Equal(t, 1048576, cfg.PrefetchSize)
	assert.True(t, cfg.DefaultQueueDurable)
	assert.False(t, cfg.DefaultQueueAutoDelete)
	assert.False(t, cfg.DefaultQueueExclusive)
	assert.Equal(t, "direct", cfg.DefaultExchangeType)
	assert.True(t, cfg.DefaultExchangeDurable)
	assert.False(t, cfg.AutoAck)
	assert.False(t, cfg.Exclusive)
	assert.False(t, cfg.NoLocal)
	assert.False(t, cfg.NoWait)
	assert.True(t, cfg.MandatoryPublish)
	assert.False(t, cfg.ImmediatePublish)
	assert.True(t, cfg.EnableDLQ)
	assert.Equal(t, "custom.dlq", cfg.DLQExchange)
	assert.Equal(t, "custom.key", cfg.DLQRoutingKey)
	assert.Equal(t, 7200000, cfg.DLQMessageTTL)
	assert.Equal(t, 50000, cfg.DLQMaxLength)
	assert.Equal(t, 10, cfg.MaxConnections)
	assert.Equal(t, 100, cfg.MaxChannels)
	assert.Equal(t, 30*time.Second, cfg.HeartbeatInterval)
	assert.True(t, cfg.EnableMetrics)
	assert.Equal(t, "rabbitmq", cfg.MetricsPrefix)
}

func TestExchangeConfig_AllFields(t *testing.T) {
	cfg := &ExchangeConfig{
		Name:       "test.exchange",
		Type:       "fanout",
		Durable:    true,
		AutoDelete: false,
		Internal:   true,
		NoWait:     false,
		Args:       map[string]interface{}{"custom": "arg"},
	}

	assert.Equal(t, "test.exchange", cfg.Name)
	assert.Equal(t, "fanout", cfg.Type)
	assert.True(t, cfg.Durable)
	assert.False(t, cfg.AutoDelete)
	assert.True(t, cfg.Internal)
	assert.False(t, cfg.NoWait)
	assert.Equal(t, "arg", cfg.Args["custom"])
}

func TestQueueConfig_AllFields(t *testing.T) {
	cfg := &QueueConfig{
		Name:                 "test.queue",
		Durable:              true,
		AutoDelete:           false,
		Exclusive:            true,
		NoWait:               false,
		DeadLetterExchange:   "dlx.exchange",
		DeadLetterRoutingKey: "dlx.key",
		MessageTTL:           3600000,
		MaxLength:            10000,
		MaxPriority:          10,
		Args:                 map[string]interface{}{},
	}

	assert.Equal(t, "test.queue", cfg.Name)
	assert.True(t, cfg.Durable)
	assert.False(t, cfg.AutoDelete)
	assert.True(t, cfg.Exclusive)
	assert.False(t, cfg.NoWait)
	assert.Equal(t, "dlx.exchange", cfg.DeadLetterExchange)
	assert.Equal(t, "dlx.key", cfg.DeadLetterRoutingKey)
	assert.Equal(t, 3600000, cfg.MessageTTL)
	assert.Equal(t, 10000, cfg.MaxLength)
	assert.Equal(t, 10, cfg.MaxPriority)
}

func TestQueueConfig_ChainedMethods(t *testing.T) {
	cfg := DefaultQueueConfig("test.queue").
		WithDLQ("dlx.exchange", "dlx.key").
		WithTTL(7200000).
		WithMaxLength(50000).
		WithPriority(5)

	assert.Equal(t, "test.queue", cfg.Name)
	assert.Equal(t, "dlx.exchange", cfg.DeadLetterExchange)
	assert.Equal(t, "dlx.key", cfg.DeadLetterRoutingKey)
	assert.Equal(t, 7200000, cfg.MessageTTL)
	assert.Equal(t, 50000, cfg.MaxLength)
	assert.Equal(t, 5, cfg.MaxPriority)
}

func TestBroker_Close_WithMultipleSubscriptions(t *testing.T) {
	broker := NewBroker(nil, nil)

	// Add multiple subscriptions
	for i := 0; i < 5; i++ {
		sub := &rabbitSubscription{
			id:       "sub-" + string(rune('0'+i)),
			topic:    "topic." + string(rune('0'+i)),
			cancelCh: make(chan struct{}),
		}
		sub.active.Store(true)
		broker.subscriptions.Put("topic."+string(rune('0'+i)), sub)
	}

	assert.Equal(t, 5, broker.subscriptions.Len())

	ctx := context.Background()
	err := broker.Close(ctx)
	assert.NoError(t, err)

	// All subscriptions should be deactivated
	broker.subscriptions.Range(func(_ string, sub *rabbitSubscription) bool {
		assert.False(t, sub.active.Load())
		return true
	})
}

func TestConnection_MultipleCallbacks(t *testing.T) {
	conn := NewConnection(nil, nil)

	connectCalls := 0
	disconnectCalls := 0
	reconnectCalls := 0

	conn.OnConnect(func() { connectCalls++ })
	conn.OnConnect(func() { connectCalls++ })
	conn.OnDisconnect(func(err error) { disconnectCalls++ })
	conn.OnDisconnect(func(err error) { disconnectCalls++ })
	conn.OnReconnect(func() { reconnectCalls++ })
	conn.OnReconnect(func() { reconnectCalls++ })

	// Simulate callbacks
	conn.onConnect.Range(func(_ int, cb func()) bool {
		cb()
		return true
	})
	conn.onDisconnect.Range(func(_ int, cb func(error)) bool {
		cb(nil)
		return true
	})
	conn.onReconnect.Range(func(_ int, cb func()) bool {
		cb()
		return true
	})

	assert.Equal(t, 2, connectCalls)
	assert.Equal(t, 2, disconnectCalls)
	assert.Equal(t, 2, reconnectCalls)
}

func TestBroker_MetricsRecording_AllTypes(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	// Record various operations
	metrics.RecordConnectionAttempt()
	metrics.RecordConnectionSuccess()
	metrics.RecordPublish(100, 10*time.Millisecond, true)
	metrics.RecordPublish(0, 5*time.Millisecond, false)
	metrics.RecordReceive(200, 20*time.Millisecond)
	metrics.RecordSubscription()
	metrics.RecordUnsubscription()
	metrics.RecordQueueDeclared()
	metrics.RecordProcessed()
	metrics.RecordFailed()
	metrics.RecordAck()
	metrics.RecordNack()
	metrics.RecordRetry()
	metrics.RecordDeadLettered()
	metrics.RecordPublishConfirmation()
	metrics.RecordPublishTimeout()

	// Verify all metrics were recorded
	assert.Equal(t, int64(1), metrics.ConnectionAttempts.Load())
	assert.Equal(t, int64(1), metrics.ConnectionSuccesses.Load())
	assert.Equal(t, int64(2), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(1), metrics.PublishSuccesses.Load())
	assert.Equal(t, int64(1), metrics.PublishFailures.Load())
	assert.Equal(t, int64(1), metrics.MessagesReceived.Load())
	assert.Equal(t, int64(0), metrics.ActiveSubscriptions.Load()) // +1 -1
	assert.Equal(t, int64(1), metrics.QueuesDeclared.Load())
	assert.Equal(t, int64(1), metrics.MessagesProcessed.Load())
	assert.Equal(t, int64(1), metrics.MessagesFailed.Load())
	assert.Equal(t, int64(1), metrics.MessagesAcked.Load())
	assert.Equal(t, int64(1), metrics.MessagesNacked.Load())
	assert.Equal(t, int64(1), metrics.MessagesRetried.Load())
	assert.Equal(t, int64(1), metrics.MessagesDeadLettered.Load())
	assert.Equal(t, int64(1), metrics.PublishConfirmations.Load())
	assert.Equal(t, int64(1), metrics.PublishTimeouts.Load())
}

func TestMetrics_AveragePublishLatency(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	// No latency recorded yet
	assert.Equal(t, time.Duration(0), metrics.GetAveragePublishLatency())

	// Record some latency
	metrics.PublishLatencyTotal.Store(int64(600 * time.Millisecond))
	metrics.PublishLatencyCount.Store(3)

	avg := metrics.GetAveragePublishLatency()
	assert.Equal(t, 200*time.Millisecond, avg)
}

func TestMetrics_GetPublishSuccessRate(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	// No messages published yet - should return 1.0
	assert.Equal(t, 1.0, metrics.GetPublishSuccessRate())

	// Record some publishes
	metrics.MessagesPublished.Store(100)
	metrics.PublishSuccesses.Store(90)

	rate := metrics.GetPublishSuccessRate()
	assert.Equal(t, 0.9, rate)
}

func TestMetrics_GetLastPublishTime(t *testing.T) {
	metrics := messaging.NewBrokerMetrics()

	// Initially zero
	lastPublish := metrics.GetLastPublishTime()
	assert.True(t, lastPublish.IsZero())

	// Record a publish
	metrics.RecordPublish(100, time.Millisecond, true)

	lastPublish = metrics.GetLastPublishTime()
	assert.False(t, lastPublish.IsZero())
}

func TestBroker_IsConnected_Concurrent(t *testing.T) {
	broker := NewBroker(nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = broker.IsConnected()
		}()
	}
	wg.Wait()
}

func TestBroker_BrokerType_ReturnsRabbitMQ(t *testing.T) {
	broker := NewBroker(nil, nil)
	brokerType := broker.BrokerType()

	assert.Equal(t, messaging.BrokerTypeRabbitMQ, brokerType)
	assert.Equal(t, "rabbitmq", brokerType.String())
}

func TestConnection_Connect_WhenAlreadyConnected(t *testing.T) {
	conn := NewConnection(nil, nil)
	conn.state.Store(int32(StateConnected))

	ctx := context.Background()
	err := conn.Connect(ctx)

	// Should return nil when already connected
	assert.NoError(t, err)
}
