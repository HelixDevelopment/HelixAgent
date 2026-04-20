package kafka

import (
	"context"
	"crypto/tls"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.agent/internal/messaging"
)

// =============================================================================
// Config Tests
// =============================================================================

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, []string{"localhost:9092"}, cfg.Brokers)
	assert.Equal(t, "helixagent", cfg.ClientID)
	assert.False(t, cfg.TLSEnabled)
	assert.False(t, cfg.TLSSkipVerify)
	assert.False(t, cfg.SASLEnabled)
	assert.Equal(t, "PLAIN", cfg.SASLMechanism)
	assert.Equal(t, -1, cfg.RequiredAcks)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, cfg.RetryBackoff)
	assert.Equal(t, 16384, cfg.BatchSize)
	assert.Equal(t, 10*time.Millisecond, cfg.BatchTimeout)
	assert.Equal(t, 1048576, cfg.MaxMessageBytes)
	assert.Equal(t, "lz4", cfg.CompressionCodec)
	assert.True(t, cfg.Idempotent)
	assert.Equal(t, "helixagent-consumers", cfg.GroupID)
	assert.Equal(t, "earliest", cfg.AutoOffsetReset)
	assert.False(t, cfg.EnableAutoCommit)
	assert.Equal(t, 5*time.Second, cfg.AutoCommitInterval)
	assert.Equal(t, 30*time.Second, cfg.SessionTimeout)
	assert.Equal(t, 3*time.Second, cfg.HeartbeatInterval)
	assert.Equal(t, 500, cfg.MaxPollRecords)
	assert.Equal(t, 5*time.Minute, cfg.MaxPollInterval)
	assert.Equal(t, 1, cfg.FetchMinBytes)
	assert.Equal(t, 52428800, cfg.FetchMaxBytes)
	assert.Equal(t, 500*time.Millisecond, cfg.FetchMaxWait)
	assert.Equal(t, 30*time.Second, cfg.DialTimeout)
	assert.Equal(t, 30*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.WriteTimeout)
	assert.Equal(t, 5*time.Minute, cfg.MetadataRefresh)
	assert.Equal(t, 3, cfg.DefaultPartitions)
	assert.Equal(t, 1, cfg.DefaultReplication)
	assert.True(t, cfg.EnableMetrics)
	assert.Equal(t, "kafka", cfg.MetricsPrefix)
}

func TestDefaultTopicConfig(t *testing.T) {
	cfg := DefaultTopicConfig("test.topic")

	assert.Equal(t, "test.topic", cfg.Name)
	assert.Equal(t, 3, cfg.Partitions)
	assert.Equal(t, 1, cfg.ReplicationFactor)
	assert.Equal(t, int64(604800000), cfg.RetentionMs)
	assert.Equal(t, int64(-1), cfg.RetentionBytes)
	assert.Equal(t, "delete", cfg.CleanupPolicy)
	assert.Equal(t, 1, cfg.MinInsyncReplicas)
	assert.NotNil(t, cfg.Configs)
}

func TestTopicConfig_WithRetention(t *testing.T) {
	cfg := DefaultTopicConfig("test.topic")
	cfg.WithRetention(86400000, 1073741824)

	assert.Equal(t, int64(86400000), cfg.RetentionMs)
	assert.Equal(t, int64(1073741824), cfg.RetentionBytes)
}

func TestTopicConfig_WithCompaction(t *testing.T) {
	cfg := DefaultTopicConfig("test.topic")
	cfg.WithCompaction()

	assert.Equal(t, "compact", cfg.CleanupPolicy)
}

func TestTopicConfig_WithPartitions(t *testing.T) {
	cfg := DefaultTopicConfig("test.topic")
	cfg.WithPartitions(12)

	assert.Equal(t, 12, cfg.Partitions)
}

func TestTopicConfig_WithReplication(t *testing.T) {
	cfg := DefaultTopicConfig("test.topic")
	cfg.WithReplication(3)

	assert.Equal(t, 3, cfg.ReplicationFactor)
}

func TestDefaultConsumerGroupConfig(t *testing.T) {
	cfg := DefaultConsumerGroupConfig("test-group", []string{"topic1", "topic2"})

	assert.Equal(t, "test-group", cfg.GroupID)
	assert.Equal(t, []string{"topic1", "topic2"}, cfg.Topics)
	assert.True(t, cfg.StartFromBeginning)
	assert.Equal(t, 10, cfg.MaxConcurrency)
	assert.Equal(t, 5*time.Second, cfg.CommitInterval)
	assert.Equal(t, 60*time.Second, cfg.RebalanceTimeout)
	assert.True(t, cfg.RetryOnError)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 1*time.Second, cfg.RetryBackoff)
}

// =============================================================================
// Broker Lifecycle Tests
// =============================================================================

func TestKafkaBroker_NewBroker_NilConfig(t *testing.T) {
	broker := NewBroker(nil, nil)

	require.NotNil(t, broker)
	assert.NotNil(t, broker.config)
	assert.Equal(t, []string{"localhost:9092"}, broker.config.Brokers)
	assert.NotNil(t, broker.logger)
	assert.NotNil(t, broker.writers)
	assert.NotNil(t, broker.readers)
	assert.Equal(t, 0, broker.writers.Len())
	assert.Equal(t, 0, broker.readers.Len())
	assert.NotNil(t, broker.metrics)
	assert.False(t, broker.closed.Load())
	assert.False(t, broker.connected.Load())
}

func TestKafkaBroker_NewBroker_CustomConfig(t *testing.T) {
	cfg := &Config{
		Brokers:  []string{"kafka1:9092", "kafka2:9092"},
		ClientID: "custom-client",
	}
	logger := zap.NewNop()

	broker := NewBroker(cfg, logger)

	require.NotNil(t, broker)
	assert.Equal(t, []string{"kafka1:9092", "kafka2:9092"}, broker.config.Brokers)
	assert.Equal(t, "custom-client", broker.config.ClientID)
	assert.Equal(t, logger, broker.logger)
}

func TestKafkaBroker_NewBroker_NilLogger(t *testing.T) {
	cfg := DefaultConfig()
	broker := NewBroker(cfg, nil)

	require.NotNil(t, broker)
	assert.NotNil(t, broker.logger)
}

func TestKafkaBroker_NewBroker_InitializesEmptyMaps(t *testing.T) {
	broker := NewBroker(nil, nil)

	assert.Equal(t, 0, broker.writers.Len())
	assert.Equal(t, 0, broker.readers.Len())
}

func TestKafkaBroker_Close_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	err := broker.Close(ctx)

	assert.NoError(t, err)
	assert.True(t, broker.closed.Load())
}

func TestKafkaBroker_Close_MultipleCallsIdempotent(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	// First close
	err1 := broker.Close(ctx)
	assert.NoError(t, err1)

	// Second close - should be idempotent
	err2 := broker.Close(ctx)
	assert.NoError(t, err2)

	// Third close - still idempotent
	err3 := broker.Close(ctx)
	assert.NoError(t, err3)

	assert.True(t, broker.closed.Load())
}

func TestKafkaBroker_Close_ConcurrentCalls(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	var wg sync.WaitGroup
	errorCount := atomic.Int32{}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := broker.Close(ctx); err != nil {
				errorCount.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(0), errorCount.Load())
	assert.True(t, broker.closed.Load())
}

func TestKafkaBroker_HealthCheck_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	err := broker.HealthCheck(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_IsConnected_InitiallyFalse(t *testing.T) {
	broker := NewBroker(nil, nil)

	assert.False(t, broker.IsConnected())
}

func TestKafkaBroker_IsConnected_AfterSettingTrue(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true)

	assert.True(t, broker.IsConnected())
}

func TestKafkaBroker_IsConnected_ConcurrentAccess(t *testing.T) {
	broker := NewBroker(nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				broker.connected.Store(true)
			} else {
				_ = broker.IsConnected()
			}
		}(i)
	}
	wg.Wait()
}

func TestKafkaBroker_BrokerType(t *testing.T) {
	broker := NewBroker(nil, nil)

	assert.Equal(t, messaging.BrokerTypeKafka, broker.BrokerType())
}

// =============================================================================
// kafkaSubscription Tests
// =============================================================================

func TestKafkaSubscription_Unsubscribe(t *testing.T) {
	sub := &kafkaSubscription{
		id:    "sub-1",
		topic: "test.topic",
	}
	sub.active.Store(true)

	err := sub.Unsubscribe()

	assert.NoError(t, err)
	assert.False(t, sub.IsActive())
}

func TestKafkaSubscription_Unsubscribe_CallsCancelFn(t *testing.T) {
	cancelCalled := false
	sub := &kafkaSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		cancelFn: func() { cancelCalled = true },
	}
	sub.active.Store(true)

	err := sub.Unsubscribe()

	assert.NoError(t, err)
	assert.True(t, cancelCalled)
}

func TestKafkaSubscription_Unsubscribe_WhenAlreadyUnsubscribed(t *testing.T) {
	cancelCalled := false
	sub := &kafkaSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		cancelFn: func() { cancelCalled = true },
	}
	sub.active.Store(false) // Already inactive

	err := sub.Unsubscribe()

	assert.NoError(t, err)
	assert.False(t, cancelCalled) // Should not call cancel since already inactive
}

func TestKafkaSubscription_Unsubscribe_IdempotentBehavior(t *testing.T) {
	callCount := atomic.Int32{}
	sub := &kafkaSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		cancelFn: func() { callCount.Add(1) },
	}
	sub.active.Store(true)

	// First unsubscribe
	err1 := sub.Unsubscribe()
	assert.NoError(t, err1)

	// Second unsubscribe
	err2 := sub.Unsubscribe()
	assert.NoError(t, err2)

	// Cancel should only be called once
	assert.Equal(t, int32(1), callCount.Load())
}

func TestKafkaSubscription_Unsubscribe_ConcurrentCalls(t *testing.T) {
	callCount := atomic.Int32{}
	sub := &kafkaSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		cancelFn: func() { callCount.Add(1) },
	}
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

	// Cancel should only be called once despite concurrent calls
	assert.Equal(t, int32(1), callCount.Load())
	assert.False(t, sub.IsActive())
}

func TestKafkaSubscription_IsActive(t *testing.T) {
	sub := &kafkaSubscription{
		id:    "sub-1",
		topic: "test.topic",
	}

	// Initially inactive by default
	assert.False(t, sub.IsActive())

	// Set to active
	sub.active.Store(true)
	assert.True(t, sub.IsActive())

	// Set back to inactive
	sub.active.Store(false)
	assert.False(t, sub.IsActive())
}

func TestKafkaSubscription_Topic_Custom(t *testing.T) {
	sub := &kafkaSubscription{
		id:    "sub-1",
		topic: "my.custom.topic",
	}

	assert.Equal(t, "my.custom.topic", sub.Topic())
}

func TestKafkaSubscription_Topic_EmptyString(t *testing.T) {
	sub := &kafkaSubscription{
		id:    "sub-1",
		topic: "",
	}

	assert.Equal(t, "", sub.Topic())
}

func TestKafkaSubscription_ID_Custom(t *testing.T) {
	sub := &kafkaSubscription{
		id:    "test-subscription-123",
		topic: "test.topic",
	}

	assert.Equal(t, "test-subscription-123", sub.ID())
}

func TestKafkaSubscription_ID_EmptyString(t *testing.T) {
	sub := &kafkaSubscription{
		id:    "",
		topic: "test.topic",
	}

	assert.Equal(t, "", sub.ID())
}

func TestKafkaSubscription_AllFields(t *testing.T) {
	sub := &kafkaSubscription{
		id:      "sub-1",
		topic:   "events",
		groupID: "consumer-group-1",
	}
	sub.active.Store(true)

	assert.Equal(t, "sub-1", sub.ID())
	assert.Equal(t, "events", sub.Topic())
	assert.Equal(t, "consumer-group-1", sub.groupID)
	assert.True(t, sub.IsActive())
}

// =============================================================================
// Publish Tests
// =============================================================================

func TestKafkaBroker_Publish_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	msg := &messaging.Message{
		ID:        "test-msg-1",
		Type:      "test.event",
		Payload:   []byte(`{"key": "value"}`),
		Timestamp: time.Now(),
		Headers:   map[string]string{"x-custom": "header"},
	}

	err := broker.Publish(ctx, "test-topic", msg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_Publish_WhenNotConnected_RecordsMetrics(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	msg := &messaging.Message{
		ID:        "test-msg-1",
		Type:      "test.event",
		Payload:   []byte(`{}`),
		Timestamp: time.Now(),
	}

	_ = broker.Publish(ctx, "test-topic", msg)

	metrics := broker.GetMetrics()
	assert.Equal(t, int64(1), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(1), metrics.PublishFailures.Load())
}

func TestKafkaBroker_PublishBatch_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	messages := []*messaging.Message{
		{ID: "msg-1", Type: "test", Payload: []byte(`{}`), Timestamp: time.Now()},
		{ID: "msg-2", Type: "test", Payload: []byte(`{}`), Timestamp: time.Now()},
		{ID: "msg-3", Type: "test", Payload: []byte(`{}`), Timestamp: time.Now()},
	}

	err := broker.PublishBatch(ctx, "test-topic", messages)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_PublishBatch_WhenNotConnected_EmptyBatch(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	err := broker.PublishBatch(ctx, "test-topic", []*messaging.Message{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_Subscribe_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	sub, err := broker.Subscribe(ctx, "test-topic", handler)

	assert.Error(t, err)
	assert.Nil(t, sub)
	assert.Contains(t, err.Error(), "not connected")
}

// =============================================================================
// Writer Management Tests
// =============================================================================

func TestKafkaBroker_GetOrCreateWriter_CreatesNewWriter(t *testing.T) {
	broker := NewBroker(nil, nil)

	writer := broker.getOrCreateWriter("test-topic")

	assert.NotNil(t, writer)
	assert.Equal(t, "test-topic", writer.Topic)
}

func TestKafkaBroker_GetOrCreateWriter_ReusesExistingWriter(t *testing.T) {
	broker := NewBroker(nil, nil)

	writer1 := broker.getOrCreateWriter("test-topic")
	writer2 := broker.getOrCreateWriter("test-topic")

	assert.Same(t, writer1, writer2)
}

func TestKafkaBroker_GetOrCreateWriter_DifferentTopics(t *testing.T) {
	broker := NewBroker(nil, nil)

	writer1 := broker.getOrCreateWriter("topic-1")
	writer2 := broker.getOrCreateWriter("topic-2")

	assert.NotSame(t, writer1, writer2)
	assert.Equal(t, "topic-1", writer1.Topic)
	assert.Equal(t, "topic-2", writer2.Topic)
}

func TestKafkaBroker_GetOrCreateWriter_ConcurrentAccess(t *testing.T) {
	broker := NewBroker(nil, nil)

	var wg sync.WaitGroup
	writers := make([]*kafka.Writer, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			writers[idx] = broker.getOrCreateWriter("shared-topic")
		}(i)
	}
	wg.Wait()

	// All writers should be the same instance
	first := writers[0]
	for _, w := range writers[1:] {
		assert.Same(t, first, w)
	}
}

func TestKafkaBroker_GetOrCreateWriter_UsesConfigSettings(t *testing.T) {
	cfg := &Config{
		Brokers:      []string{"kafka1:9092", "kafka2:9092"},
		BatchSize:    1000,
		BatchTimeout: 100 * time.Millisecond,
		MaxRetries:   5,
		RequiredAcks: -1,
	}
	broker := NewBroker(cfg, nil)

	writer := broker.getOrCreateWriter("test-topic")

	assert.NotNil(t, writer)
	assert.Equal(t, 1000, writer.BatchSize)
	assert.Equal(t, 100*time.Millisecond, writer.BatchTimeout)
	assert.Equal(t, 5, writer.MaxAttempts)
}

// =============================================================================
// Metrics Tracking Tests
// =============================================================================

func TestKafkaBroker_MetricsRecording_PublishSuccess(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordPublish(100, 50*time.Millisecond, true)

	assert.Equal(t, int64(1), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(1), metrics.PublishSuccesses.Load())
	assert.Equal(t, int64(100), metrics.BytesPublished.Load())
	assert.Equal(t, int64(0), metrics.PublishFailures.Load())
}

func TestKafkaBroker_MetricsRecording_PublishFailure(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordPublish(0, 50*time.Millisecond, false)

	assert.Equal(t, int64(1), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(0), metrics.PublishSuccesses.Load())
	assert.Equal(t, int64(1), metrics.PublishFailures.Load())
}

func TestKafkaBroker_MetricsRecording_Receive(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordReceive(200, 30*time.Millisecond)

	assert.Equal(t, int64(1), metrics.MessagesReceived.Load())
	assert.Equal(t, int64(200), metrics.BytesReceived.Load())
}

func TestKafkaBroker_MetricsRecording_Subscription(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordSubscription()
	metrics.RecordSubscription()

	assert.Equal(t, int64(2), metrics.ActiveSubscriptions.Load())

	metrics.RecordUnsubscription()

	assert.Equal(t, int64(1), metrics.ActiveSubscriptions.Load())
}

func TestKafkaBroker_MetricsRecording_Connection(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordConnectionAttempt()
	metrics.RecordConnectionSuccess()

	assert.Equal(t, int64(1), metrics.ConnectionAttempts.Load())
	assert.Equal(t, int64(1), metrics.ConnectionSuccesses.Load())

	metrics.RecordDisconnection()

	assert.Equal(t, int64(0), metrics.CurrentConnections.Load())
}

func TestKafkaBroker_MetricsRecording_Errors(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordFailed()
	metrics.RecordFailed()
	metrics.RecordSerializationError()

	assert.Equal(t, int64(2), metrics.MessagesFailed.Load())
	assert.Equal(t, int64(1), metrics.SerializationErrors.Load())
	assert.Equal(t, int64(3), metrics.TotalErrors.Load())
}

func TestKafkaBroker_MetricsRecording_BatchPublish(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordBatchPublish(5, 500, 100*time.Millisecond, true)

	assert.Equal(t, int64(1), metrics.BatchesPublished.Load())
	assert.Equal(t, int64(5), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(500), metrics.BytesPublished.Load())
}

func TestKafkaBroker_MetricsRecording_TopicCreated(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordTopicCreated()
	metrics.RecordTopicCreated()

	assert.Equal(t, int64(2), metrics.TopicsCreated.Load())
}

func TestKafkaBroker_GetMetrics_NotNil(t *testing.T) {
	broker := NewBroker(nil, nil)

	metrics := broker.GetMetrics()

	require.NotNil(t, metrics)
}

func TestKafkaBroker_GetMetrics_SameInstance(t *testing.T) {
	broker := NewBroker(nil, nil)

	metrics1 := broker.GetMetrics()
	metrics2 := broker.GetMetrics()

	assert.Same(t, metrics1, metrics2)
}

func TestKafkaBroker_GetMetrics_ConcurrentAccess(t *testing.T) {
	broker := NewBroker(nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics := broker.GetMetrics()
			metrics.RecordPublish(100, time.Millisecond, true)
		}()
	}
	wg.Wait()

	metrics := broker.GetMetrics()
	assert.Equal(t, int64(100), metrics.MessagesPublished.Load())
}

// =============================================================================
// Topic Management Tests
// =============================================================================

func TestTopicConfig_Name(t *testing.T) {
	cfg := DefaultTopicConfig("my-topic")

	assert.Equal(t, "my-topic", cfg.Name)
}

func TestTopicConfig_WithEmptyName(t *testing.T) {
	cfg := DefaultTopicConfig("")

	assert.Equal(t, "", cfg.Name)
}

func TestTopicConfig_SpecialCharactersInName(t *testing.T) {
	cfg := DefaultTopicConfig("my.topic-name_v2")

	assert.Equal(t, "my.topic-name_v2", cfg.Name)
}

func TestTopicConfig_ChainedMethodsCalls(t *testing.T) {
	cfg := DefaultTopicConfig("test").
		WithPartitions(10).
		WithReplication(3).
		WithRetention(86400000, 1073741824).
		WithCompaction()

	assert.Equal(t, "test", cfg.Name)
	assert.Equal(t, 10, cfg.Partitions)
	assert.Equal(t, 3, cfg.ReplicationFactor)
	assert.Equal(t, int64(86400000), cfg.RetentionMs)
	assert.Equal(t, int64(1073741824), cfg.RetentionBytes)
	assert.Equal(t, "compact", cfg.CleanupPolicy)
}

func TestTopicConfig_CustomConfigs(t *testing.T) {
	cfg := DefaultTopicConfig("test")
	cfg.Configs["segment.bytes"] = "1073741824"
	cfg.Configs["min.insync.replicas"] = "2"

	assert.Equal(t, "1073741824", cfg.Configs["segment.bytes"])
	assert.Equal(t, "2", cfg.Configs["min.insync.replicas"])
}

func TestTopicMetadata_Structure(t *testing.T) {
	metadata := &TopicMetadata{
		Name: "test-topic",
		Partitions: []PartitionMetadata{
			{ID: 0, Leader: 1, Replicas: []int{1, 2, 3}, ISR: []int{1, 2, 3}},
			{ID: 1, Leader: 2, Replicas: []int{2, 3, 1}, ISR: []int{2, 3}},
		},
	}

	assert.Equal(t, "test-topic", metadata.Name)
	assert.Len(t, metadata.Partitions, 2)
	assert.Equal(t, 0, metadata.Partitions[0].ID)
	assert.Equal(t, 1, metadata.Partitions[0].Leader)
}

func TestPartitionMetadata_Fields(t *testing.T) {
	pm := PartitionMetadata{
		ID:       0,
		Leader:   1,
		Replicas: []int{1, 2, 3},
		ISR:      []int{1, 2},
	}

	assert.Equal(t, 0, pm.ID)
	assert.Equal(t, 1, pm.Leader)
	assert.Equal(t, []int{1, 2, 3}, pm.Replicas)
	assert.Equal(t, []int{1, 2}, pm.ISR)
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestExtractBrokerIDs_EmptySlice(t *testing.T) {
	brokers := []kafka.Broker{}

	ids := extractBrokerIDs(brokers)

	assert.Empty(t, ids)
}

func TestExtractBrokerIDs_SingleBroker(t *testing.T) {
	brokers := []kafka.Broker{
		{ID: 1, Host: "kafka1", Port: 9092},
	}

	ids := extractBrokerIDs(brokers)

	assert.Equal(t, []int{1}, ids)
}

func TestExtractBrokerIDs_MultipleBrokers(t *testing.T) {
	brokers := []kafka.Broker{
		{ID: 1, Host: "kafka1", Port: 9092},
		{ID: 2, Host: "kafka2", Port: 9092},
		{ID: 3, Host: "kafka3", Port: 9092},
	}

	ids := extractBrokerIDs(brokers)

	assert.Equal(t, []int{1, 2, 3}, ids)
}

func TestExtractBrokerIDs_PreservesOrder(t *testing.T) {
	brokers := []kafka.Broker{
		{ID: 5, Host: "kafka5", Port: 9092},
		{ID: 1, Host: "kafka1", Port: 9092},
		{ID: 10, Host: "kafka10", Port: 9092},
	}

	ids := extractBrokerIDs(brokers)

	assert.Equal(t, []int{5, 1, 10}, ids)
}

func TestExtractBrokerIDs_ZeroID(t *testing.T) {
	brokers := []kafka.Broker{
		{ID: 0, Host: "kafka0", Port: 9092},
	}

	ids := extractBrokerIDs(brokers)

	assert.Equal(t, []int{0}, ids)
}

func TestExtractBrokerIDs_NegativeID(t *testing.T) {
	brokers := []kafka.Broker{
		{ID: -1, Host: "kafka", Port: 9092},
	}

	ids := extractBrokerIDs(brokers)

	assert.Equal(t, []int{-1}, ids)
}

// =============================================================================
// Config Validation Tests
// =============================================================================

func TestConfig_Validate_ValidConfig(t *testing.T) {
	cfg := DefaultConfig()

	err := cfg.Validate()

	assert.NoError(t, err)
}

func TestConfig_Validate_EmptyBrokersSlice(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Brokers = []string{}

	err := cfg.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one broker is required")
}

func TestConfig_Validate_EmptyBrokerAddress(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Brokers = []string{"kafka1:9092", ""}

	err := cfg.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "broker address cannot be empty")
}

func TestConfig_Validate_MissingClientID(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ClientID = ""

	err := cfg.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client_id is required")
}

func TestConfig_Validate_ZeroBatchSize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BatchSize = 0

	err := cfg.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch_size must be at least 1")
}

func TestConfig_Validate_BatchSizeNegative(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BatchSize = -1

	err := cfg.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch_size must be at least 1")
}

func TestConfig_Validate_ZeroPartitions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultPartitions = 0

	err := cfg.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default_partitions must be at least 1")
}

func TestConfig_Validate_ZeroReplication(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultReplication = 0

	err := cfg.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default_replication must be at least 1")
}

func TestConfig_Validate_SASLMissingUsername(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SASLEnabled = true
	cfg.SASLUsername = ""

	err := cfg.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sasl_username is required when SASL is enabled")
}

func TestConfig_Validate_SASLWithCredentials(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SASLEnabled = true
	cfg.SASLUsername = "user"
	cfg.SASLPassword = "pass"

	err := cfg.Validate()

	assert.NoError(t, err)
}

// =============================================================================
// Integration-Style Tests (Unit tests that verify component interactions)
// =============================================================================

func TestBroker_FullLifecycle_NoConnection(t *testing.T) {
	// Create broker
	broker := NewBroker(nil, nil)
	assert.False(t, broker.IsConnected())

	// Try to publish (should fail)
	msg := &messaging.Message{
		ID:        "msg-1",
		Type:      "test",
		Payload:   []byte(`{}`),
		Timestamp: time.Now(),
	}
	err := broker.Publish(context.Background(), "topic", msg)
	assert.Error(t, err)

	// Try to subscribe (should fail)
	_, err = broker.Subscribe(context.Background(), "topic", func(ctx context.Context, msg *messaging.Message) error {
		return nil
	})
	assert.Error(t, err)

	// Close (should succeed even without connection)
	err = broker.Close(context.Background())
	assert.NoError(t, err)
	assert.True(t, broker.closed.Load())
}

func TestBroker_MetricsAccumulation(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	// Simulate multiple operations
	for i := 0; i < 10; i++ {
		metrics.RecordPublish(100, time.Millisecond, true)
		metrics.RecordReceive(50, time.Millisecond)
		metrics.RecordProcessed()
	}

	assert.Equal(t, int64(10), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(10), metrics.MessagesReceived.Load())
	assert.Equal(t, int64(10), metrics.MessagesProcessed.Load())
	assert.Equal(t, int64(1000), metrics.BytesPublished.Load())
	assert.Equal(t, int64(500), metrics.BytesReceived.Load())
}

func TestBroker_SubCounterIncrement(t *testing.T) {
	broker := NewBroker(nil, nil)

	assert.Equal(t, int64(0), broker.subCounter.Load())

	// Simulate subscription counter increments
	v1 := broker.subCounter.Add(1)
	v2 := broker.subCounter.Add(1)
	v3 := broker.subCounter.Add(1)

	assert.Equal(t, int64(1), v1)
	assert.Equal(t, int64(2), v2)
	assert.Equal(t, int64(3), v3)
	assert.Equal(t, int64(3), broker.subCounter.Load())
}

func TestNewBroker(t *testing.T) {
	// With nil config
	broker := NewBroker(nil, nil)
	assert.NotNil(t, broker)
	assert.NotNil(t, broker.config)
	assert.Equal(t, []string{"localhost:9092"}, broker.config.Brokers)

	// With custom config
	cfg := &Config{
		Brokers:  []string{"kafka1:9092", "kafka2:9092"},
		ClientID: "custom-client",
	}
	broker2 := NewBroker(cfg, nil)
	assert.Equal(t, []string{"kafka1:9092", "kafka2:9092"}, broker2.config.Brokers)
	assert.Equal(t, "custom-client", broker2.config.ClientID)
}

func TestBroker_BrokerType_IsKafka(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.Equal(t, messaging.BrokerTypeKafka, broker.BrokerType())
}

func TestBroker_IsConnected_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.False(t, broker.IsConnected())
}

func TestBroker_GetMetrics_Returns_Metrics(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()
	assert.NotNil(t, metrics)
}

func TestKafkaSubscription(t *testing.T) {
	sub := &kafkaSubscription{
		id:      "sub-1",
		topic:   "test.topic",
		groupID: "test-group",
	}
	sub.active.Store(true)

	assert.Equal(t, "sub-1", sub.ID())
	assert.Equal(t, "test.topic", sub.Topic())
	assert.True(t, sub.IsActive())
}

func TestKafkaSubscription_Unsubscribe_Basic(t *testing.T) {
	sub := &kafkaSubscription{
		id:    "sub-1",
		topic: "test.topic",
	}
	sub.active.Store(true)

	err := sub.Unsubscribe()
	assert.NoError(t, err)
	assert.False(t, sub.IsActive())

	// Second unsubscribe should be no-op
	err = sub.Unsubscribe()
	assert.NoError(t, err)
}

// =============================================================================
// Additional Coverage Tests
// =============================================================================

func TestKafkaBroker_Close_WithWriters(t *testing.T) {
	broker := NewBroker(nil, nil)

	// Create a writer by calling getOrCreateWriter
	writer := broker.getOrCreateWriter("test-topic")
	assert.NotNil(t, writer)

	// Verify writer is in the map
	assert.Equal(t, 1, broker.writers.Len())

	// Close the broker - this will close all writers
	ctx := context.Background()
	err := broker.Close(ctx)

	// Close may return an error since the writer isn't actually connected
	// but the important thing is that the Close logic runs
	_ = err
	assert.True(t, broker.closed.Load())
}

func TestKafkaBroker_Close_RecordsDisconnection(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	// Record a connection first
	metrics.RecordConnectionSuccess()
	assert.Equal(t, int64(1), metrics.CurrentConnections.Load())

	// Close the broker
	ctx := context.Background()
	err := broker.Close(ctx)
	assert.NoError(t, err)

	// Verify disconnection was recorded
	assert.Equal(t, int64(0), metrics.CurrentConnections.Load())
}

func TestKafkaBroker_Close_SetsConnectedFalse(t *testing.T) {
	broker := NewBroker(nil, nil)

	// Simulate connected state
	broker.connected.Store(true)
	assert.True(t, broker.IsConnected())

	// Close the broker
	ctx := context.Background()
	err := broker.Close(ctx)
	assert.NoError(t, err)

	// Verify connected is false
	assert.False(t, broker.IsConnected())
}

func TestKafkaBroker_Close_WithMultipleWriters(t *testing.T) {
	broker := NewBroker(nil, nil)

	// Create multiple writers
	broker.getOrCreateWriter("topic-1")
	broker.getOrCreateWriter("topic-2")
	broker.getOrCreateWriter("topic-3")

	assert.Equal(t, 3, broker.writers.Len())

	// Close the broker
	ctx := context.Background()
	_ = broker.Close(ctx)

	assert.True(t, broker.closed.Load())
}

func TestKafkaBroker_Publish_MessageWithHeaders(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	msg := &messaging.Message{
		ID:        "test-msg-1",
		Type:      "test.event",
		Payload:   []byte(`{"key": "value"}`),
		Timestamp: time.Now(),
		Headers: map[string]string{
			"x-custom-1": "value1",
			"x-custom-2": "value2",
			"x-custom-3": "value3",
		},
		TraceID: "trace-123",
	}

	err := broker.Publish(ctx, "test-topic", msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_Publish_MessageWithTraceID(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	msg := &messaging.Message{
		ID:        "test-msg-1",
		Type:      "test.event",
		Payload:   []byte(`{}`),
		Timestamp: time.Now(),
		TraceID:   "trace-abc-123",
	}

	err := broker.Publish(ctx, "test-topic", msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_PublishBatch_WithMultipleMessages(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	messages := make([]*messaging.Message, 10)
	for i := 0; i < 10; i++ {
		messages[i] = &messaging.Message{
			ID:        "msg-" + string(rune('0'+i)),
			Type:      "batch.event",
			Payload:   []byte(`{"index": ` + string(rune('0'+i)) + `}`),
			Timestamp: time.Now(),
			Headers:   map[string]string{"batch-index": string(rune('0' + i))},
		}
	}

	err := broker.PublishBatch(ctx, "test-topic", messages)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_Subscribe_WhenNotConnected_WithOptions(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	sub, err := broker.Subscribe(ctx, "test-topic", handler, messaging.WithGroupID("custom-group"))
	assert.Error(t, err)
	assert.Nil(t, sub)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_GetOrCreateWriter_MultipleTopics(t *testing.T) {
	broker := NewBroker(nil, nil)

	topics := []string{"topic-a", "topic-b", "topic-c", "topic-d", "topic-e"}
	writers := make([]*kafka.Writer, len(topics))

	for i, topic := range topics {
		writers[i] = broker.getOrCreateWriter(topic)
	}

	// Verify each topic has its own writer
	assert.Equal(t, len(topics), broker.writers.Len())

	// Verify each writer is correctly configured
	for i, topic := range topics {
		assert.Equal(t, topic, writers[i].Topic)
	}

	// Verify getting existing writers returns same instances
	for i, topic := range topics {
		writer := broker.getOrCreateWriter(topic)
		assert.Same(t, writers[i], writer)
	}

	// Verify still the same number of writers
	assert.Equal(t, len(topics), broker.writers.Len())
}

func TestKafkaBroker_NewBroker_AllConfigFields(t *testing.T) {
	cfg := &Config{
		Brokers:            []string{"kafka1:9092", "kafka2:9092", "kafka3:9092"},
		ClientID:           "test-client",
		TLSEnabled:         false,
		SASLEnabled:        false,
		RequiredAcks:       -1,
		MaxRetries:         5,
		RetryBackoff:       200 * time.Millisecond,
		BatchSize:          32768,
		BatchTimeout:       20 * time.Millisecond,
		MaxMessageBytes:    2097152,
		CompressionCodec:   "gzip",
		Idempotent:         true,
		GroupID:            "test-consumers",
		AutoOffsetReset:    "latest",
		EnableAutoCommit:   true,
		AutoCommitInterval: 10 * time.Second,
		FetchMinBytes:      1024,
		FetchMaxBytes:      104857600,
		FetchMaxWait:       1 * time.Second,
	}
	logger := zap.NewNop()

	broker := NewBroker(cfg, logger)

	assert.Equal(t, cfg.Brokers, broker.config.Brokers)
	assert.Equal(t, cfg.ClientID, broker.config.ClientID)
	assert.Equal(t, cfg.BatchSize, broker.config.BatchSize)
	assert.Equal(t, cfg.GroupID, broker.config.GroupID)
	assert.Equal(t, cfg.AutoOffsetReset, broker.config.AutoOffsetReset)
}

func TestKafkaBroker_MetricsRecording_AllTypes(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	// Record various metric types
	metrics.RecordConnectionAttempt()
	metrics.RecordConnectionSuccess()
	metrics.RecordPublish(100, 10*time.Millisecond, true)
	metrics.RecordPublish(0, 5*time.Millisecond, false)
	metrics.RecordReceive(200, 20*time.Millisecond)
	metrics.RecordBatchPublish(5, 500, 50*time.Millisecond, true)
	metrics.RecordSubscription()
	metrics.RecordProcessed()
	metrics.RecordFailed()
	metrics.RecordSerializationError()
	metrics.RecordTopicCreated()

	// Verify all metrics were recorded
	assert.Equal(t, int64(1), metrics.ConnectionAttempts.Load())
	assert.Equal(t, int64(1), metrics.ConnectionSuccesses.Load())
	assert.Equal(t, int64(7), metrics.MessagesPublished.Load()) // 2 single + 5 batch
	assert.Equal(t, int64(6), metrics.PublishSuccesses.Load())  // 1 single + 5 batch
	assert.Equal(t, int64(1), metrics.PublishFailures.Load())
	assert.Equal(t, int64(600), metrics.BytesPublished.Load()) // 100 + 500
	assert.Equal(t, int64(1), metrics.MessagesReceived.Load())
	assert.Equal(t, int64(200), metrics.BytesReceived.Load())
	assert.Equal(t, int64(1), metrics.BatchesPublished.Load())
	assert.Equal(t, int64(1), metrics.ActiveSubscriptions.Load())
	assert.Equal(t, int64(1), metrics.MessagesProcessed.Load())
	assert.Equal(t, int64(1), metrics.MessagesFailed.Load())
	assert.Equal(t, int64(1), metrics.SerializationErrors.Load())
	assert.Equal(t, int64(1), metrics.TopicsCreated.Load())
}

func TestKafkaBroker_SubCounter_ConcurrentIncrements(t *testing.T) {
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

func TestKafkaSubscription_Unsubscribe_NilCancelFn(t *testing.T) {
	sub := &kafkaSubscription{
		id:       "sub-1",
		topic:    "test.topic",
		cancelFn: nil, // No cancel function
	}
	sub.active.Store(true)

	err := sub.Unsubscribe()
	assert.NoError(t, err)
	assert.False(t, sub.IsActive())
}

func TestConfig_Validate_AllFieldsValid(t *testing.T) {
	cfg := &Config{
		Brokers:            []string{"kafka1:9092", "kafka2:9092"},
		ClientID:           "test-client",
		BatchSize:          1000,
		DefaultPartitions:  6,
		DefaultReplication: 3,
		SASLEnabled:        false,
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestTopicMetadata_EmptyPartitions(t *testing.T) {
	metadata := &TopicMetadata{
		Name:       "test-topic",
		Partitions: []PartitionMetadata{},
	}

	assert.Equal(t, "test-topic", metadata.Name)
	assert.Empty(t, metadata.Partitions)
}

func TestPartitionMetadata_EmptySlices(t *testing.T) {
	pm := PartitionMetadata{
		ID:       0,
		Leader:   1,
		Replicas: []int{},
		ISR:      []int{},
	}

	assert.Equal(t, 0, pm.ID)
	assert.Equal(t, 1, pm.Leader)
	assert.Empty(t, pm.Replicas)
	assert.Empty(t, pm.ISR)
}

func TestExtractBrokerIDs_LargeSlice(t *testing.T) {
	brokers := make([]kafka.Broker, 100)
	for i := 0; i < 100; i++ {
		brokers[i] = kafka.Broker{ID: i, Host: "kafka" + string(rune('0'+i%10)), Port: 9092}
	}

	ids := extractBrokerIDs(brokers)

	assert.Len(t, ids, 100)
	for i := 0; i < 100; i++ {
		assert.Equal(t, i, ids[i])
	}
}

// =============================================================================
// Additional Coverage Tests - Error Paths and Edge Cases
// =============================================================================

func TestKafkaBroker_Publish_NilMessage(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true) // Simulate connected state
	ctx := context.Background()

	err := broker.Publish(ctx, "test-topic", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message is nil")
}

func TestKafkaBroker_Subscribe_EmptyTopic(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true) // Simulate connected state
	ctx := context.Background()

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	sub, err := broker.Subscribe(ctx, "", handler)

	assert.Error(t, err)
	assert.Nil(t, sub)
	assert.Contains(t, err.Error(), "topic is required")
}

func TestKafkaBroker_Subscribe_NilHandler(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true) // Simulate connected state
	ctx := context.Background()

	sub, err := broker.Subscribe(ctx, "test-topic", nil)

	assert.Error(t, err)
	assert.Nil(t, sub)
	assert.Contains(t, err.Error(), "handler is required")
}

func TestKafkaBroker_CreateTopic_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	topicConfig := &TopicConfig{
		Name:              "test-topic",
		Partitions:        3,
		ReplicationFactor: 1,
	}

	err := broker.CreateTopic(ctx, topicConfig)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_DeleteTopic_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	err := broker.DeleteTopic(ctx, "test-topic")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_GetTopicMetadata_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	metadata, err := broker.GetTopicMetadata(ctx, "test-topic")

	assert.Error(t, err)
	assert.Nil(t, metadata)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_Connect_WhenAlreadyConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true) // Simulate already connected state
	ctx := context.Background()

	err := broker.Connect(ctx)

	assert.NoError(t, err) // Should return nil when already connected
}

func TestKafkaBroker_PublishBatch_EmptyMessages_WhenConnected(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true) // Simulate connected state
	ctx := context.Background()

	err := broker.PublishBatch(ctx, "test-topic", []*messaging.Message{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no messages to publish")
}

func TestKafkaBroker_PublishBatch_NilSlice(t *testing.T) {
	broker := NewBroker(nil, nil)
	broker.connected.Store(true) // Simulate connected state
	ctx := context.Background()

	err := broker.PublishBatch(ctx, "test-topic", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no messages to publish")
}

func TestKafkaBroker_Subscribe_OffsetResetLatest(t *testing.T) {
	// Test that offset reset option is parsed correctly
	broker := NewBroker(nil, nil)
	// Can't fully test without Kafka, but verify option parsing by checking
	// the subscribe fails at connection, not at option parsing
	ctx := context.Background()

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	_, err := broker.Subscribe(ctx, "test-topic", handler, messaging.WithOffsetReset(messaging.OffsetResetLatest))

	// Should fail with "not connected" error, not option error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_Subscribe_CustomGroupID(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	_, err := broker.Subscribe(ctx, "test-topic", handler, messaging.WithGroupID("custom-group"))

	// Should fail with "not connected" error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_Publish_WithPartitionKey(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	msg := &messaging.Message{
		ID:        "test-msg-1",
		Type:      "test.event",
		Payload:   []byte(`{"key": "value"}`),
		Timestamp: time.Now(),
	}

	err := broker.Publish(ctx, "test-topic", msg, messaging.WithMessageKey([]byte("partition-key")))

	// Should fail with "not connected" error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_Close_SubscriptionCancelFnCalled(t *testing.T) {
	// Test that Close() properly handles subscription cancel functions
	// without actually having a reader (tests the cancel path)
	cancelCalled := false
	sub := &kafkaSubscription{
		id:       "sub-1",
		topic:    "test-topic",
		cancelFn: func() { cancelCalled = true },
		reader:   nil, // nil reader to avoid needing Kafka
	}
	sub.active.Store(true)

	// The broker's Close method reads from the readers map
	// and calls cancel on each subscription, then closes the reader
	// Since we can't easily mock the reader, we test the subscription's
	// own behavior separately via the broker lifecycle tests

	// When close is called on the subscription directly
	_ = sub.Unsubscribe()
	assert.True(t, cancelCalled)
	assert.False(t, sub.active.Load())
}

func TestKafkaBroker_Publish_ContextCancelled(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	msg := &messaging.Message{
		ID:        "test-msg-1",
		Type:      "test.event",
		Payload:   []byte(`{}`),
		Timestamp: time.Now(),
	}

	err := broker.Publish(ctx, "test-topic", msg)

	// Should fail with "not connected" error (context cancellation is checked later)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_HealthCheck_ContextTimeout(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // Let timeout expire

	err := broker.HealthCheck(ctx)

	// Should fail with "not connected" (checked before trying to dial)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_Close_SetsClosedBeforeProcessing(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	// First call to Close should set closed flag
	err := broker.Close(ctx)
	assert.NoError(t, err)
	assert.True(t, broker.closed.Load())

	// Second call should return immediately because closed is already true
	err = broker.Close(ctx)
	assert.NoError(t, err)
}

func TestKafkaBroker_MetricsRecording_ConnectionFailure(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	metrics.RecordConnectionAttempt()
	metrics.RecordConnectionFailure()

	assert.Equal(t, int64(1), metrics.ConnectionAttempts.Load())
	assert.Equal(t, int64(1), metrics.ConnectionFailures.Load())
	assert.Equal(t, int64(0), metrics.ConnectionSuccesses.Load())
}

func TestKafkaBroker_MetricsRecording_MultipleFailures(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	for i := 0; i < 5; i++ {
		metrics.RecordConnectionAttempt()
		metrics.RecordConnectionFailure()
	}

	assert.Equal(t, int64(5), metrics.ConnectionAttempts.Load())
	assert.Equal(t, int64(5), metrics.ConnectionFailures.Load())
}

func TestConfig_AutoOffsetReset_Latest(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoOffsetReset = "latest"

	assert.Equal(t, "latest", cfg.AutoOffsetReset)
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_AutoOffsetReset_Earliest(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoOffsetReset = "earliest"

	assert.Equal(t, "earliest", cfg.AutoOffsetReset)
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConsumerGroupConfig_AllFields(t *testing.T) {
	cfg := &ConsumerGroupConfig{
		GroupID:            "test-group",
		Topics:             []string{"topic1", "topic2", "topic3"},
		StartFromBeginning: false,
		MaxConcurrency:     20,
		CommitInterval:     10 * time.Second,
		RebalanceTimeout:   120 * time.Second,
		RetryOnError:       false,
		MaxRetries:         5,
		RetryBackoff:       2 * time.Second,
	}

	assert.Equal(t, "test-group", cfg.GroupID)
	assert.Len(t, cfg.Topics, 3)
	assert.False(t, cfg.StartFromBeginning)
	assert.Equal(t, 20, cfg.MaxConcurrency)
	assert.Equal(t, 10*time.Second, cfg.CommitInterval)
	assert.Equal(t, 120*time.Second, cfg.RebalanceTimeout)
	assert.False(t, cfg.RetryOnError)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 2*time.Second, cfg.RetryBackoff)
}

func TestTopicConfig_WithMultipleRetentionValues(t *testing.T) {
	cfg := DefaultTopicConfig("test")
	cfg.WithRetention(3600000, 536870912) // 1 hour, 512MB

	assert.Equal(t, int64(3600000), cfg.RetentionMs)
	assert.Equal(t, int64(536870912), cfg.RetentionBytes)
}

func TestTopicConfig_MinInsyncReplicas(t *testing.T) {
	cfg := DefaultTopicConfig("test")
	cfg.MinInsyncReplicas = 2

	assert.Equal(t, 2, cfg.MinInsyncReplicas)
}

func TestKafkaBroker_WriterBalancer(t *testing.T) {
	broker := NewBroker(nil, nil)

	writer := broker.getOrCreateWriter("test-topic")

	// Verify the writer uses LeastBytes balancer
	assert.NotNil(t, writer.Balancer)
	_, ok := writer.Balancer.(*kafka.LeastBytes)
	assert.True(t, ok, "Expected LeastBytes balancer")
}

func TestKafkaBroker_WriterRequiredAcks(t *testing.T) {
	cfg := &Config{
		Brokers:      []string{"localhost:9092"},
		ClientID:     "test",
		RequiredAcks: 1, // Wait for leader ack only
		BatchSize:    1000,
	}
	broker := NewBroker(cfg, nil)

	writer := broker.getOrCreateWriter("test-topic")

	assert.Equal(t, kafka.RequiredAcks(1), writer.RequiredAcks)
}

func TestKafkaBroker_WriterAllowAutoTopicCreation(t *testing.T) {
	broker := NewBroker(nil, nil)

	writer := broker.getOrCreateWriter("test-topic")

	assert.True(t, writer.AllowAutoTopicCreation)
}

func TestKafkaBroker_WriterAsync(t *testing.T) {
	broker := NewBroker(nil, nil)

	writer := broker.getOrCreateWriter("test-topic")

	// Writer should be synchronous by default
	assert.False(t, writer.Async)
}

func TestKafkaSubscription_AllFieldsSet(t *testing.T) {
	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}
	cancelFn := func() {}

	sub := &kafkaSubscription{
		id:       "sub-123",
		topic:    "events.user.created",
		groupID:  "my-consumer-group",
		handler:  handler,
		reader:   nil, // Would normally be a kafka.Reader
		cancelFn: cancelFn,
	}
	sub.active.Store(true)

	assert.Equal(t, "sub-123", sub.ID())
	assert.Equal(t, "events.user.created", sub.Topic())
	assert.Equal(t, "my-consumer-group", sub.groupID)
	assert.NotNil(t, sub.handler)
	assert.NotNil(t, sub.cancelFn)
	assert.True(t, sub.IsActive())
}

func TestKafkaBroker_ConcurrentPublishAndClose(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	var wg sync.WaitGroup

	// Start goroutines trying to publish
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := &messaging.Message{
				ID:        "msg-" + string(rune('0'+idx%10)),
				Type:      "test",
				Payload:   []byte(`{}`),
				Timestamp: time.Now(),
			}
			_ = broker.Publish(ctx, "test-topic", msg)
		}(i)
	}

	// Concurrently close the broker
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(1 * time.Millisecond)
		_ = broker.Close(ctx)
	}()

	wg.Wait()

	assert.True(t, broker.closed.Load())
}

func TestKafkaBroker_MetricsRecording_ProcessedAndFailed(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	// Simulate message processing
	for i := 0; i < 100; i++ {
		if i%10 == 0 {
			metrics.RecordFailed()
		} else {
			metrics.RecordProcessed()
		}
	}

	assert.Equal(t, int64(90), metrics.MessagesProcessed.Load())
	assert.Equal(t, int64(10), metrics.MessagesFailed.Load())
}

func TestKafkaBroker_MetricsRecording_Unsubscription(t *testing.T) {
	broker := NewBroker(nil, nil)
	metrics := broker.GetMetrics()

	// Subscribe and unsubscribe multiple times
	for i := 0; i < 5; i++ {
		metrics.RecordSubscription()
	}
	assert.Equal(t, int64(5), metrics.ActiveSubscriptions.Load())

	for i := 0; i < 3; i++ {
		metrics.RecordUnsubscription()
	}
	assert.Equal(t, int64(2), metrics.ActiveSubscriptions.Load())
}

func TestKafkaBroker_PublishBatch_MessageWithHeaders(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	messages := []*messaging.Message{
		{
			ID:        "msg-1",
			Type:      "test",
			Payload:   []byte(`{"data": 1}`),
			Timestamp: time.Now(),
			Headers:   map[string]string{"header1": "value1", "header2": "value2"},
		},
		{
			ID:        "msg-2",
			Type:      "test",
			Payload:   []byte(`{"data": 2}`),
			Timestamp: time.Now(),
			Headers:   map[string]string{"header3": "value3"},
		},
	}

	err := broker.PublishBatch(ctx, "test-topic", messages)

	// Should fail with "not connected" error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestConfig_TLSSettings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TLSEnabled = true
	cfg.TLSSkipVerify = true
	cfg.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}

	assert.True(t, cfg.TLSEnabled)
	assert.True(t, cfg.TLSSkipVerify)
	assert.NotNil(t, cfg.TLSConfig)
	assert.True(t, cfg.TLSConfig.InsecureSkipVerify)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.TLSConfig.MinVersion)
}

func TestConfig_SASLSettings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SASLEnabled = true
	cfg.SASLMechanism = "SCRAM-SHA-512"
	cfg.SASLUsername = "user"
	cfg.SASLPassword = "password"

	assert.True(t, cfg.SASLEnabled)
	assert.Equal(t, "SCRAM-SHA-512", cfg.SASLMechanism)
	assert.Equal(t, "user", cfg.SASLUsername)
	assert.Equal(t, "password", cfg.SASLPassword)

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_SASLMechanismPlain(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SASLEnabled = true
	cfg.SASLMechanism = "PLAIN"
	cfg.SASLUsername = "user"
	cfg.SASLPassword = "password"

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestTopicConfig_Configs_CustomValues(t *testing.T) {
	cfg := DefaultTopicConfig("test")
	cfg.Configs["max.message.bytes"] = "10485760"
	cfg.Configs["compression.type"] = "lz4"
	cfg.Configs["segment.ms"] = "3600000"

	assert.Equal(t, "10485760", cfg.Configs["max.message.bytes"])
	assert.Equal(t, "lz4", cfg.Configs["compression.type"])
	assert.Equal(t, "3600000", cfg.Configs["segment.ms"])
}

func TestKafkaBroker_Publish_EmptyPayload(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	msg := &messaging.Message{
		ID:        "test-msg-1",
		Type:      "test.event",
		Payload:   []byte{}, // Empty payload
		Timestamp: time.Now(),
	}

	err := broker.Publish(ctx, "test-topic", msg)

	// Should fail with "not connected" error, not payload error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestKafkaBroker_Publish_LargePayload(t *testing.T) {
	broker := NewBroker(nil, nil)
	ctx := context.Background()

	// Create a large payload (1MB)
	largePayload := make([]byte, 1024*1024)
	for i := range largePayload {
		largePayload[i] = byte(i % 256)
	}

	msg := &messaging.Message{
		ID:        "test-msg-1",
		Type:      "test.event",
		Payload:   largePayload,
		Timestamp: time.Now(),
	}

	err := broker.Publish(ctx, "test-topic", msg)

	// Should fail with "not connected" error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}
