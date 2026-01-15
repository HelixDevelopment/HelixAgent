package kafka

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/messaging"
)

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

func TestBroker_BrokerType(t *testing.T) {
	broker := NewBroker(nil, nil)
	assert.Equal(t, messaging.BrokerTypeKafka, broker.BrokerType())
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

func TestKafkaSubscription_Unsubscribe(t *testing.T) {
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
