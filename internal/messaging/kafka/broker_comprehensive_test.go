package kafka

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.agent/internal/messaging"
)

// =============================================================================
// Config Validation Tests (10 tests)
// =============================================================================

func TestConfig_Validate_Success(t *testing.T) {
	cfg := DefaultConfig()
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_NoBrokers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Brokers = []string{}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one broker is required")
}

func TestConfig_Validate_EmptyBroker(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Brokers = []string{"localhost:9092", ""}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "broker address cannot be empty")
}

func TestConfig_Validate_EmptyClientID(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ClientID = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client_id is required")
}

func TestConfig_Validate_InvalidBatchSize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BatchSize = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch_size must be at least 1")
}

func TestConfig_Validate_InvalidPartitions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultPartitions = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default_partitions must be at least 1")
}

func TestConfig_Validate_InvalidReplication(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultReplication = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default_replication must be at least 1")
}

func TestConfig_Validate_SASLWithoutUsername(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SASLEnabled = true
	cfg.SASLUsername = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sasl_username is required when SASL is enabled")
}

func TestConfig_Validate_SASLWithUsername(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SASLEnabled = true
	cfg.SASLUsername = "testuser"
	cfg.SASLPassword = "testpass"
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_MultipleBrokers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Brokers = []string{"kafka1:9092", "kafka2:9092", "kafka3:9092"}
	err := cfg.Validate()
	assert.NoError(t, err)
}

// =============================================================================
// Broker Constructor Tests (8 tests)
// =============================================================================

func TestNewBroker_NilConfig_UsesDefaults(t *testing.T) {
	broker := NewBroker(nil, nil)
	require.NotNil(t, broker)
	assert.NotNil(t, broker.config)
	assert.NotNil(t, broker.metrics)
	assert.NotNil(t, broker.writers)
	assert.NotNil(t, broker.readers)
	assert.Equal(t, 0, broker.writers.Len())
	assert.Equal(t, 0, broker.readers.Len())
}

func TestNewBroker_NilLogger_UsesNop(t *testing.T) {
	broker := NewBroker(DefaultConfig(), nil)
	require.NotNil(t, broker)
	assert.NotNil(t, broker.logger)
}

func TestNewBroker_CustomConfig(t *testing.T) {
	cfg := &Config{
		Brokers:            []string{"custom:9092"},
		ClientID:           "custom-client",
		BatchSize:          100,
		DefaultPartitions:  5,
		DefaultReplication: 3,
	}
	broker := NewBroker(cfg, nil)
	assert.Equal(t, []string{"custom:9092"}, broker.config.Brokers)
	assert.Equal(t, "custom-client", broker.config.ClientID)
	assert.Equal(t, 100, broker.config.BatchSize)
}

func TestNewBroker_CustomLogger(t *testing.T) {
	logger := zap.NewNop()
	broker := NewBroker(nil, logger)
	assert.Equal(t, logger, broker.logger)
}

func TestNewBroker_InitialState(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.False(t, broker.IsConnected())
	assert.False(t, broker.closed.Load())
	assert.Equal(t, int64(0), broker.subCounter.Load())
}

func TestNewBroker_EmptyMaps(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.Equal(t, 0, broker.writers.Len())
	assert.Equal(t, 0, broker.readers.Len())
}

func TestNewBroker_MetricsInitialized(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()
	require.NotNil(t, metrics)
	assert.Equal(t, int64(0), metrics.MessagesReceived.Load())
}

func TestNewBroker_BrokerTypeKafka(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.Equal(t, messaging.BrokerTypeKafka, broker.BrokerType())
}

// =============================================================================
// Topic Config Tests (8 tests)
// =============================================================================

func TestTopicConfig_ChainedMethods(t *testing.T) {
	cfg := DefaultTopicConfig("test").
		WithPartitions(6).
		WithReplication(3).
		WithRetention(3600000, 1073741824)

	assert.Equal(t, "test", cfg.Name)
	assert.Equal(t, 6, cfg.Partitions)
	assert.Equal(t, 3, cfg.ReplicationFactor)
	assert.Equal(t, int64(3600000), cfg.RetentionMs)
	assert.Equal(t, int64(1073741824), cfg.RetentionBytes)
}

func TestTopicConfig_WithCompaction_SetsCleanupPolicy(t *testing.T) {
	cfg := DefaultTopicConfig("compacted").WithCompaction()
	assert.Equal(t, "compact", cfg.CleanupPolicy)
}

func TestTopicConfig_ConfigsMapNotNil(t *testing.T) {
	cfg := DefaultTopicConfig("test")
	require.NotNil(t, cfg.Configs)
	cfg.Configs["segment.bytes"] = "1073741824"
	assert.Equal(t, "1073741824", cfg.Configs["segment.bytes"])
}

func TestTopicConfig_Defaults(t *testing.T) {
	cfg := DefaultTopicConfig("my-topic")
	assert.Equal(t, "my-topic", cfg.Name)
	assert.Equal(t, 3, cfg.Partitions)
	assert.Equal(t, 1, cfg.ReplicationFactor)
	assert.Equal(t, int64(604800000), cfg.RetentionMs) // 7 days
	assert.Equal(t, int64(-1), cfg.RetentionBytes)     // unlimited
	assert.Equal(t, "delete", cfg.CleanupPolicy)
	assert.Equal(t, 1, cfg.MinInsyncReplicas)
}

func TestTopicConfig_WithRetention_CustomValues(t *testing.T) {
	cfg := DefaultTopicConfig("test")
	cfg.WithRetention(86400000, 536870912)
	assert.Equal(t, int64(86400000), cfg.RetentionMs)     // 1 day
	assert.Equal(t, int64(536870912), cfg.RetentionBytes) // 512MB
}

func TestTopicConfig_WithPartitions_Zero(t *testing.T) {
	cfg := DefaultTopicConfig("test").WithPartitions(0)
	assert.Equal(t, 0, cfg.Partitions)
}

func TestTopicConfig_WithReplication_High(t *testing.T) {
	cfg := DefaultTopicConfig("test").WithReplication(10)
	assert.Equal(t, 10, cfg.ReplicationFactor)
}

func TestTopicConfig_MultipleCalls(t *testing.T) {
	cfg := DefaultTopicConfig("test")
	cfg.WithPartitions(6)
	cfg.WithPartitions(12)
	assert.Equal(t, 12, cfg.Partitions)
}

// =============================================================================
// Consumer Group Config Tests (6 tests)
// =============================================================================

func TestConsumerGroupConfig_Defaults(t *testing.T) {
	topics := []string{"topic1", "topic2"}
	cfg := DefaultConsumerGroupConfig("my-group", topics)

	assert.Equal(t, "my-group", cfg.GroupID)
	assert.Equal(t, topics, cfg.Topics)
	assert.True(t, cfg.StartFromBeginning)
	assert.Equal(t, 10, cfg.MaxConcurrency)
	assert.Equal(t, 5*time.Second, cfg.CommitInterval)
	assert.Equal(t, 60*time.Second, cfg.RebalanceTimeout)
	assert.True(t, cfg.RetryOnError)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 1*time.Second, cfg.RetryBackoff)
}

func TestConsumerGroupConfig_EmptyTopics(t *testing.T) {
	cfg := DefaultConsumerGroupConfig("group", []string{})
	assert.Empty(t, cfg.Topics)
}

func TestConsumerGroupConfig_SingleTopic(t *testing.T) {
	cfg := DefaultConsumerGroupConfig("group", []string{"single-topic"})
	assert.Len(t, cfg.Topics, 1)
	assert.Equal(t, "single-topic", cfg.Topics[0])
}

func TestConsumerGroupConfig_ManyTopics(t *testing.T) {
	topics := make([]string, 100)
	for i := range topics {
		topics[i] = "topic-" + string(rune('a'+i%26))
	}
	cfg := DefaultConsumerGroupConfig("group", topics)
	assert.Len(t, cfg.Topics, 100)
}

func TestConsumerGroupConfig_ModifyAfterCreate(t *testing.T) {
	cfg := DefaultConsumerGroupConfig("group", []string{"topic1"})
	cfg.MaxConcurrency = 50
	cfg.CommitInterval = 10 * time.Second
	cfg.RetryOnError = false

	assert.Equal(t, 50, cfg.MaxConcurrency)
	assert.Equal(t, 10*time.Second, cfg.CommitInterval)
	assert.False(t, cfg.RetryOnError)
}

func TestConsumerGroupConfig_NilTopics(t *testing.T) {
	cfg := DefaultConsumerGroupConfig("group", nil)
	assert.Nil(t, cfg.Topics)
}

// =============================================================================
// Subscription Tests (10 tests)
// =============================================================================

func TestKafkaSubscription_ID(t *testing.T) {
	sub := &kafkaSubscription{id: "test-sub-123"}
	assert.Equal(t, "test-sub-123", sub.ID())
}

func TestKafkaSubscription_Topic(t *testing.T) {
	sub := &kafkaSubscription{topic: "my.topic.name"}
	assert.Equal(t, "my.topic.name", sub.Topic())
}

func TestKafkaSubscription_IsActive_True(t *testing.T) {
	sub := &kafkaSubscription{}
	sub.active.Store(true)
	assert.True(t, sub.IsActive())
}

func TestKafkaSubscription_IsActive_False(t *testing.T) {
	sub := &kafkaSubscription{}
	sub.active.Store(false)
	assert.False(t, sub.IsActive())
}

func TestKafkaSubscription_Unsubscribe_SetsInactive(t *testing.T) {
	sub := &kafkaSubscription{}
	sub.active.Store(true)
	err := sub.Unsubscribe()
	assert.NoError(t, err)
	assert.False(t, sub.IsActive())
}

func TestKafkaSubscription_Unsubscribe_Idempotent(t *testing.T) {
	sub := &kafkaSubscription{}
	sub.active.Store(true)

	err1 := sub.Unsubscribe()
	assert.NoError(t, err1)

	err2 := sub.Unsubscribe()
	assert.NoError(t, err2)

	assert.False(t, sub.IsActive())
}

func TestKafkaSubscription_Unsubscribe_CallsCancel(t *testing.T) {
	cancelCalled := false
	sub := &kafkaSubscription{
		cancelFn: func() { cancelCalled = true },
	}
	sub.active.Store(true)

	err := sub.Unsubscribe()
	assert.NoError(t, err)
	assert.True(t, cancelCalled)
}

func TestKafkaSubscription_Unsubscribe_AlreadyInactive(t *testing.T) {
	cancelCalled := false
	sub := &kafkaSubscription{
		cancelFn: func() { cancelCalled = true },
	}
	sub.active.Store(false)

	err := sub.Unsubscribe()
	assert.NoError(t, err)
	// Cancel should not be called if already inactive
	assert.False(t, cancelCalled)
}

func TestKafkaSubscription_Fields(t *testing.T) {
	sub := &kafkaSubscription{
		id:      "sub-1",
		topic:   "events",
		groupID: "consumer-group",
	}
	sub.active.Store(true)

	assert.Equal(t, "sub-1", sub.ID())
	assert.Equal(t, "events", sub.Topic())
	assert.Equal(t, "consumer-group", sub.groupID)
	assert.True(t, sub.IsActive())
}

func TestKafkaSubscription_ConcurrentUnsubscribe(t *testing.T) {
	sub := &kafkaSubscription{}
	sub.active.Store(true)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sub.Unsubscribe()
		}()
	}
	wg.Wait()

	assert.False(t, sub.IsActive())
}

// =============================================================================
// Broker Connection State Tests (6 tests)
// =============================================================================

func TestBroker_IsConnected_InitiallyFalse(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.False(t, broker.IsConnected())
}

func TestBroker_Connected_AtomicOperations(t *testing.T) {
	broker := NewBroker(nil, nil)

	broker.connected.Store(true)
	assert.True(t, broker.IsConnected())

	broker.connected.Store(false)
	assert.False(t, broker.IsConnected())
}

func TestBroker_Closed_InitiallyFalse(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.False(t, broker.closed.Load())
}

func TestBroker_SubCounter_Increments(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.Equal(t, int64(0), broker.subCounter.Load())

	v1 := broker.subCounter.Add(1)
	assert.Equal(t, int64(1), v1)

	v2 := broker.subCounter.Add(1)
	assert.Equal(t, int64(2), v2)
}

func TestBroker_Publish_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	msg := &messaging.Message{
		ID:        "test-msg-1",
		Type:      "test",
		Payload:   []byte(`{"test": true}`),
		Timestamp: time.Now(),
	}

	err := broker.Publish(ctx, "test-topic", msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestBroker_PublishBatch_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	messages := []*messaging.Message{
		{ID: "msg-1", Type: "test", Payload: []byte(`{}`), Timestamp: time.Now()},
		{ID: "msg-2", Type: "test", Payload: []byte(`{}`), Timestamp: time.Now()},
	}

	err := broker.PublishBatch(ctx, "test-topic", messages)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// =============================================================================
// Broker Metrics Tests (8 tests)
// =============================================================================

func TestBroker_GetMetrics_NotNil(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()
	require.NotNil(t, metrics)
}

func TestBroker_Metrics_RecordPublish(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordPublish(100, 50*time.Millisecond, true)
	assert.Equal(t, int64(1), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(100), metrics.BytesPublished.Load())
}

func TestBroker_Metrics_RecordReceive(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordReceive(200, 30*time.Millisecond)
	assert.Equal(t, int64(1), metrics.MessagesReceived.Load())
	assert.Equal(t, int64(200), metrics.BytesReceived.Load())
}

func TestBroker_Metrics_RecordFailed(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordFailed()
	metrics.RecordFailed()
	assert.Equal(t, int64(2), metrics.TotalErrors.Load())
}

func TestBroker_Metrics_RecordSubscription(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordSubscription()
	assert.Equal(t, int64(1), metrics.ActiveSubscriptions.Load())
}

func TestBroker_Metrics_RecordConnectionAttempt(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordConnectionAttempt()
	assert.Equal(t, int64(1), metrics.ConnectionAttempts.Load())
}

func TestBroker_Metrics_RecordProcessed(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordProcessed()
	metrics.RecordProcessed()
	assert.Equal(t, int64(2), metrics.MessagesProcessed.Load())
}

func TestBroker_Metrics_MultipleOperations(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	// Simulate various operations
	metrics.RecordConnectionAttempt()
	metrics.RecordConnectionSuccess()
	metrics.RecordSubscription()
	metrics.RecordPublish(100, 10*time.Millisecond, true)
	metrics.RecordReceive(50, 5*time.Millisecond)
	metrics.RecordProcessed()
	metrics.RecordFailed()

	assert.Equal(t, int64(1), metrics.ConnectionAttempts.Load())
	assert.Equal(t, int64(1), metrics.ActiveSubscriptions.Load())
	assert.Equal(t, int64(1), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(1), metrics.MessagesReceived.Load())
	assert.Equal(t, int64(1), metrics.TotalErrors.Load())
}

// =============================================================================
// Message Serialization Tests (6 tests)
// =============================================================================

func TestMessage_JSON_Roundtrip(t *testing.T) {
	original := &messaging.Message{
		ID:        "msg-123",
		Type:      "event.created",
		Payload:   []byte(`{"user_id": 42, "action": "login"}`),
		Timestamp: time.Now().UTC().Truncate(time.Millisecond),
		Headers:   map[string]string{"x-source": "test"},
		TraceID:   "trace-abc",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded messaging.Message
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.ID, decoded.ID)
	assert.Equal(t, original.Type, decoded.Type)
	assert.Equal(t, original.Payload, decoded.Payload)
	assert.Equal(t, original.TraceID, decoded.TraceID)
}

func TestMessage_EmptyPayload(t *testing.T) {
	msg := &messaging.Message{
		ID:        "msg-empty",
		Type:      "empty.event",
		Payload:   []byte{},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded messaging.Message
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, msg.ID, decoded.ID)
}

func TestMessage_NilHeaders(t *testing.T) {
	msg := &messaging.Message{
		ID:      "msg-no-headers",
		Type:    "simple",
		Payload: []byte(`{}`),
		Headers: nil,
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestMessage_EmptyHeaders(t *testing.T) {
	msg := &messaging.Message{
		ID:      "msg-empty-headers",
		Type:    "simple",
		Payload: []byte(`{}`),
		Headers: map[string]string{},
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded messaging.Message
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Empty(t, decoded.Headers)
}

func TestMessage_ManyHeaders(t *testing.T) {
	headers := make(map[string]string)
	for i := 0; i < 50; i++ {
		headers["header-"+string(rune('a'+i%26))] = "value"
	}

	msg := &messaging.Message{
		ID:      "msg-many-headers",
		Type:    "complex",
		Payload: []byte(`{}`),
		Headers: headers,
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestMessage_LargePayload(t *testing.T) {
	largePayload := make([]byte, 1024*1024) // 1MB
	for i := range largePayload {
		largePayload[i] = byte(i % 256)
	}

	msg := &messaging.Message{
		ID:        "msg-large",
		Type:      "large.event",
		Payload:   largePayload,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)
	assert.True(t, len(data) > len(largePayload)) // JSON encoding increases size
}

// =============================================================================
// Concurrency Tests (6 tests)
// =============================================================================

func TestBroker_ConcurrentMetricsAccess(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.RecordPublish(100, 10*time.Millisecond, true)
			metrics.RecordReceive(50, 5*time.Millisecond)
			metrics.RecordProcessed()
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(100), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(100), metrics.MessagesReceived.Load())
}

func TestBroker_ConcurrentIsConnected(t *testing.T) {
	broker := NewBroker(nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				broker.connected.Store(true)
			} else {
				broker.connected.Store(false)
			}
			_ = broker.IsConnected()
		}(i)
	}
	wg.Wait()
}

func TestBroker_ConcurrentSubCounter(t *testing.T) {
	broker := NewBroker(nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			broker.subCounter.Add(1)
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(100), broker.subCounter.Load())
}

func TestConfig_ConcurrentValidate(t *testing.T) {
	cfg := DefaultConfig()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := cfg.Validate()
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
}

func TestSubscription_ConcurrentActive(t *testing.T) {
	sub := &kafkaSubscription{}
	sub.active.Store(true)

	var wg sync.WaitGroup
	var trueCount, falseCount atomic.Int64

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sub.IsActive() {
				trueCount.Add(1)
			} else {
				falseCount.Add(1)
			}
		}()
	}
	wg.Wait()

	// All reads should see the same value since we never change it
	assert.Equal(t, int64(100), trueCount.Load())
	assert.Equal(t, int64(0), falseCount.Load())
}

func TestBroker_ConcurrentGetMetrics(t *testing.T) {
	broker := NewBroker(nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics := broker.GetMetrics()
			require.NotNil(t, metrics)
		}()
	}
	wg.Wait()
}

// =============================================================================
// Edge Cases and Error Handling Tests (8 tests)
// =============================================================================

func TestBroker_HealthCheck_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	err := broker.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestBroker_Subscribe_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	sub, err := broker.Subscribe(ctx, "test-topic", func(ctx context.Context, msg *messaging.Message) error {
		return nil
	})

	assert.Error(t, err)
	assert.Nil(t, sub)
	assert.Contains(t, err.Error(), "not connected")
}

func TestConfig_Validate_NegativeBatchSize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BatchSize = -1
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch_size must be at least 1")
}

func TestConfig_Validate_NegativePartitions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultPartitions = -1
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default_partitions must be at least 1")
}

func TestConfig_Validate_NegativeReplication(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultReplication = -1
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default_replication must be at least 1")
}

func TestBroker_Close_WhenAlreadyClosed(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.closed.Store(true)
	ctx := context.Background()

	err := broker.Close(ctx)
	assert.NoError(t, err)
}

func TestTopicConfig_EmptyName(t *testing.T) {
	cfg := DefaultTopicConfig("")
	assert.Equal(t, "", cfg.Name)
}

func TestConsumerGroupConfig_EmptyGroupID(t *testing.T) {
	cfg := DefaultConsumerGroupConfig("", nil)
	assert.Equal(t, "", cfg.GroupID)
}

// =============================================================================
// Logger Tests (4 tests)
// =============================================================================

func TestBroker_LoggerInjection(t *testing.T) {
	logger := zap.NewNop()
	broker := NewBroker(nil, logger)
	assert.Equal(t, logger, broker.logger)
}

func TestBroker_LoggerNilDefault(t *testing.T) {
	broker := NewBroker(nil, nil)
	require.NotNil(t, broker.logger)
}

func TestBroker_LoggerWithProduction(t *testing.T) {
	logger, err := zap.NewProduction()
	require.NoError(t, err)
	defer func() { _ = logger.Sync() }()

	broker := NewBroker(nil, logger)
	assert.NotNil(t, broker.logger)
}

func TestBroker_LoggerWithDevelopment(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	defer func() { _ = logger.Sync() }()

	broker := NewBroker(nil, logger)
	assert.NotNil(t, broker.logger)
}

// =============================================================================
// Additional Error Path Tests (10 tests)
// =============================================================================

func TestBroker_Publish_NilMessage(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true) // Pretend we're connected
	ctx := context.Background()

	err := broker.Publish(ctx, "test-topic", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message is nil")
}

func TestBroker_PublishBatch_EmptyMessages(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true)
	ctx := context.Background()

	err := broker.PublishBatch(ctx, "test-topic", []*messaging.Message{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no messages to publish")
}

func TestBroker_PublishBatch_NilMessages(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true)
	ctx := context.Background()

	err := broker.PublishBatch(ctx, "test-topic", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no messages to publish")
}

func TestBroker_CreateTopic_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	cfg := DefaultTopicConfig("test-topic")
	err := broker.CreateTopic(ctx, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestBroker_DeleteTopic_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	err := broker.DeleteTopic(ctx, "test-topic")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestBroker_GetTopicMetadata_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	_, err := broker.GetTopicMetadata(ctx, "test-topic")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestBroker_Subscribe_EmptyTopic(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true)
	ctx := context.Background()

	sub, err := broker.Subscribe(ctx, "", func(ctx context.Context, msg *messaging.Message) error {
		return nil
	})

	assert.Error(t, err)
	assert.Nil(t, sub)
	assert.Contains(t, err.Error(), "topic is required")
}

func TestBroker_Subscribe_NilHandler(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true)
	ctx := context.Background()

	sub, err := broker.Subscribe(ctx, "test-topic", nil)

	assert.Error(t, err)
	assert.Nil(t, sub)
	assert.Contains(t, err.Error(), "handler is required")
}

func TestBroker_Connect_AlreadyConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true)
	ctx := context.Background()

	err := broker.Connect(ctx)
	assert.NoError(t, err) // Should return nil when already connected
}

func TestBroker_Close_NoWritersOrReaders(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true)
	ctx := context.Background()

	err := broker.Close(ctx)
	assert.NoError(t, err)
	assert.False(t, broker.IsConnected())
}
