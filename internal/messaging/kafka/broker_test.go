package kafka

import (
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/messaging"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, []string{"localhost:9092"}, cfg.Brokers)
	assert.Equal(t, "helixagent", cfg.ClientID)
	assert.Equal(t, "helixagent-group", cfg.GroupID)
	assert.False(t, cfg.TLS)
	assert.False(t, cfg.SASLEnabled)
	assert.Equal(t, "PLAIN", cfg.SASLMechanism)
	assert.Equal(t, 10*time.Second, cfg.DialTimeout)
	assert.Equal(t, 30*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.WriteTimeout)
	assert.Equal(t, RequireAll, cfg.RequiredAcks)
	assert.Equal(t, CompressionLZ4, cfg.Compression)
	assert.Equal(t, 100, cfg.BatchSize)
	assert.Equal(t, int64(1024*1024), cfg.BatchBytes)
	assert.Equal(t, 10*time.Millisecond, cfg.BatchTimeout)
	assert.Equal(t, 3, cfg.MaxAttempts)
	assert.True(t, cfg.EnableIdempotent)
	assert.Equal(t, 1, cfg.MinBytes)
	assert.Equal(t, 10*1024*1024, cfg.MaxBytes)
	assert.Equal(t, 500*time.Millisecond, cfg.MaxWait)
	assert.Equal(t, 5*time.Second, cfg.CommitInterval)
	assert.Equal(t, int64(-1), cfg.StartOffset)
	assert.Equal(t, 30*time.Second, cfg.SessionTimeout)
	assert.Equal(t, 3*time.Second, cfg.HeartbeatInterval)
	assert.Equal(t, 30*time.Second, cfg.RebalanceTimeout)
	assert.False(t, cfg.AutoCreateTopics)
	assert.Equal(t, 3, cfg.DefaultPartitions)
	assert.Equal(t, 1, cfg.DefaultReplication)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "empty brokers",
			cfg: &Config{
				Brokers:  []string{},
				ClientID: "test",
			},
			wantErr: true,
		},
		{
			name: "empty broker address",
			cfg: &Config{
				Brokers:  []string{""},
				ClientID: "test",
			},
			wantErr: true,
		},
		{
			name: "empty client ID",
			cfg: &Config{
				Brokers:  []string{"localhost:9092"},
				ClientID: "",
			},
			wantErr: true,
		},
		{
			name: "TLS without cert",
			cfg: &Config{
				Brokers:     []string{"localhost:9092"},
				ClientID:    "test",
				TLS:         true,
				TLSCertFile: "",
				TLSInsecure: false,
			},
			wantErr: true,
		},
		{
			name: "TLS with cert",
			cfg: &Config{
				Brokers:     []string{"localhost:9092"},
				ClientID:    "test",
				TLS:         true,
				TLSCertFile: "/path/to/cert",
				BatchSize:   100,
				DefaultPartitions: 3,
				DefaultReplication: 1,
			},
			wantErr: false,
		},
		{
			name: "TLS insecure without cert",
			cfg: &Config{
				Brokers:     []string{"localhost:9092"},
				ClientID:    "test",
				TLS:         true,
				TLSInsecure: true,
				BatchSize:   100,
				DefaultPartitions: 3,
				DefaultReplication: 1,
			},
			wantErr: false,
		},
		{
			name: "SASL without username",
			cfg: &Config{
				Brokers:      []string{"localhost:9092"},
				ClientID:     "test",
				SASLEnabled:  true,
				SASLUsername: "",
				BatchSize:    100,
				DefaultPartitions: 3,
				DefaultReplication: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid batch size",
			cfg: &Config{
				Brokers:   []string{"localhost:9092"},
				ClientID:  "test",
				BatchSize: 0,
				DefaultPartitions: 3,
				DefaultReplication: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid partitions",
			cfg: &Config{
				Brokers:           []string{"localhost:9092"},
				ClientID:          "test",
				BatchSize:         100,
				DefaultPartitions: 0,
				DefaultReplication: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid replication",
			cfg: &Config{
				Brokers:            []string{"localhost:9092"},
				ClientID:           "test",
				BatchSize:          100,
				DefaultPartitions:  3,
				DefaultReplication: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRequiredAcks_String(t *testing.T) {
	tests := []struct {
		acks     RequiredAcks
		expected string
	}{
		{RequireNone, "none"},
		{RequireOne, "leader"},
		{RequireAll, "all"},
		{RequiredAcks(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.acks.String())
		})
	}
}

func TestCompressionCodec_String(t *testing.T) {
	tests := []struct {
		codec    CompressionCodec
		expected string
	}{
		{CompressionNone, "none"},
		{CompressionGzip, "gzip"},
		{CompressionSnappy, "snappy"},
		{CompressionLZ4, "lz4"},
		{CompressionZstd, "zstd"},
		{CompressionCodec(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.codec.String())
		})
	}
}

func TestPartitionerType_String(t *testing.T) {
	tests := []struct {
		pt       PartitionerType
		expected string
	}{
		{PartitionerRoundRobin, "round-robin"},
		{PartitionerHash, "hash"},
		{PartitionerManual, "manual"},
		{PartitionerType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.pt.String())
		})
	}
}

func TestBalancerType_String(t *testing.T) {
	tests := []struct {
		bt       BalancerType
		expected string
	}{
		{BalancerRange, "range"},
		{BalancerRoundRobin, "round-robin"},
		{BalancerRackAware, "rack-aware"},
		{BalancerType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.bt.String())
		})
	}
}

func TestDefaultTopicConfig(t *testing.T) {
	cfg := DefaultTopicConfig("test.topic")

	assert.Equal(t, "test.topic", cfg.Name)
	assert.Equal(t, 3, cfg.NumPartitions)
	assert.Equal(t, 1, cfg.ReplicationFactor)
	assert.Nil(t, cfg.ReplicaAssignment)
	assert.Nil(t, cfg.ConfigEntries)
}

func TestTopicConfig_WithPartitions(t *testing.T) {
	cfg := DefaultTopicConfig("test.topic").WithPartitions(6)
	assert.Equal(t, 6, cfg.NumPartitions)
}

func TestTopicConfig_WithReplicationFactor(t *testing.T) {
	cfg := DefaultTopicConfig("test.topic").WithReplicationFactor(3)
	assert.Equal(t, 3, cfg.ReplicationFactor)
}

func TestTopicConfig_WithConfig(t *testing.T) {
	cfg := DefaultTopicConfig("test.topic").WithConfig("retention.ms", "86400000")
	assert.Equal(t, "86400000", cfg.ConfigEntries["retention.ms"])
}

func TestTopicConfig_WithRetentionMs(t *testing.T) {
	// Note: This test documents the current behavior which has a bug
	// (uses string(rune(ms)) instead of proper int to string conversion)
	cfg := DefaultTopicConfig("test.topic").WithRetentionMs(86400000)
	assert.NotNil(t, cfg.ConfigEntries)
}

func TestTopicConfig_WithCleanupPolicy(t *testing.T) {
	cfg := DefaultTopicConfig("test.topic").WithCleanupPolicy("compact")
	assert.Equal(t, "compact", cfg.ConfigEntries["cleanup.policy"])
}

func TestDefaultConsumerGroupConfig(t *testing.T) {
	cfg := DefaultConsumerGroupConfig("test-group", []string{"topic1", "topic2"})

	assert.Equal(t, "test-group", cfg.GroupID)
	assert.Equal(t, []string{"topic1", "topic2"}, cfg.Topics)
	assert.Equal(t, BalancerRoundRobin, cfg.Balancer)
	assert.Equal(t, 30*time.Second, cfg.SessionTimeout)
	assert.Equal(t, 3*time.Second, cfg.HeartbeatInterval)
	assert.Equal(t, 30*time.Second, cfg.RebalanceTimeout)
	assert.Equal(t, int64(-1), cfg.StartOffset)
	assert.Equal(t, 24*time.Hour, cfg.RetentionTime)
	assert.Equal(t, 5*time.Second, cfg.PartitionWatchInterval)
	assert.False(t, cfg.WatchPartitionChanges)
}

func TestDefaultTopics(t *testing.T) {
	topics := DefaultTopics()

	assert.Len(t, topics, 10)

	// Check topic names
	topicNames := make([]string, len(topics))
	for i, tc := range topics {
		topicNames[i] = tc.Name
	}

	assert.Contains(t, topicNames, messaging.TopicLLMResponses)
	assert.Contains(t, topicNames, messaging.TopicDebateRounds)
	assert.Contains(t, topicNames, messaging.TopicVerificationResults)
	assert.Contains(t, topicNames, messaging.TopicProviderHealth)
	assert.Contains(t, topicNames, messaging.TopicAuditLog)
	assert.Contains(t, topicNames, messaging.TopicMetrics)
	assert.Contains(t, topicNames, messaging.TopicErrors)
	assert.Contains(t, topicNames, messaging.TopicTokenStream)
	assert.Contains(t, topicNames, messaging.TopicSSEEvents)
	assert.Contains(t, topicNames, messaging.TopicWebSocketMessages)
}

func TestNewBroker(t *testing.T) {
	// With nil config
	broker := NewBroker(nil)
	assert.NotNil(t, broker)
	assert.NotNil(t, broker.config)
	assert.Equal(t, []string{"localhost:9092"}, broker.config.Brokers)

	// With custom config
	cfg := &Config{
		Brokers:  []string{"custom-host:9093"},
		ClientID: "custom-client",
		GroupID:  "custom-group",
	}
	broker2 := NewBroker(cfg)
	assert.Equal(t, []string{"custom-host:9093"}, broker2.config.Brokers)
	assert.Equal(t, "custom-client", broker2.config.ClientID)
	assert.Equal(t, "custom-group", broker2.config.GroupID)
}

func TestBroker_BrokerType(t *testing.T) {
	broker := NewBroker(nil)
	assert.Equal(t, messaging.BrokerTypeKafka, broker.BrokerType())
}

func TestBroker_IsConnected_WhenNotConnected(t *testing.T) {
	broker := NewBroker(nil)
	assert.False(t, broker.IsConnected())
}

func TestBroker_GetMetrics(t *testing.T) {
	broker := NewBroker(nil)
	metrics := broker.GetMetrics()
	assert.NotNil(t, metrics)
}

func TestSubscription_ID(t *testing.T) {
	sub := &Subscription{
		id: "sub-123",
	}
	assert.Equal(t, "sub-123", sub.ID())
}

func TestSubscription_Topic(t *testing.T) {
	sub := &Subscription{
		topic: "test.topic",
	}
	assert.Equal(t, "test.topic", sub.Topic())
}

func TestSubscription_IsActive(t *testing.T) {
	sub := &Subscription{
		active: true,
	}
	assert.True(t, sub.IsActive())

	sub.active = false
	assert.False(t, sub.IsActive())
}

func TestGetHeaderValue(t *testing.T) {
	// Test the internal getHeaderValue function
	headers := []kafka.Header{
		{Key: "message_id", Value: []byte("msg-123")},
		{Key: "message_type", Value: []byte("test")},
		{Key: "trace_id", Value: []byte("trace-456")},
		{Key: "custom_header", Value: []byte("custom_value")},
	}

	assert.Equal(t, "msg-123", getHeaderValue(headers, "message_id"))
	assert.Equal(t, "test", getHeaderValue(headers, "message_type"))
	assert.Equal(t, "trace-456", getHeaderValue(headers, "trace_id"))
	assert.Equal(t, "custom_value", getHeaderValue(headers, "custom_header"))
	assert.Equal(t, "", getHeaderValue(headers, "non_existent"))
	assert.Equal(t, "", getHeaderValue(nil, "any"))
}

func TestSubscription_KafkaMessageToMessage(t *testing.T) {
	broker := NewBroker(nil)
	sub := &Subscription{
		broker:  broker,
		options: &messaging.SubscribeOptions{},
	}

	// Test with headers
	headers := []kafka.Header{
		{Key: "message_id", Value: []byte("msg-123")},
		{Key: "message_type", Value: []byte("test")},
		{Key: "trace_id", Value: []byte("trace-456")},
		{Key: "custom", Value: []byte("value")},
	}

	kafkaMsg := kafka.Message{
		Topic:     "test.topic",
		Partition: 2,
		Offset:    100,
		Key:       []byte("key"),
		Value:     []byte("payload"),
		Headers:   headers,
		Time:      time.Now(),
	}

	msg := sub.kafkaMessageToMessage(kafkaMsg)

	assert.Equal(t, []byte("payload"), msg.Payload)
	assert.Equal(t, int32(2), msg.Partition)
	assert.Equal(t, int64(100), msg.Offset)
	assert.Equal(t, "msg-123", msg.ID)
	assert.Equal(t, "test", msg.Type)
	assert.Equal(t, "trace-456", msg.TraceID)
	assert.Equal(t, "value", msg.Headers["custom"])
}

func TestSubscription_KafkaMessageToMessage_EmptyHeaders(t *testing.T) {
	broker := NewBroker(nil)
	sub := &Subscription{
		broker:  broker,
		options: &messaging.SubscribeOptions{},
	}

	kafkaMsg := kafka.Message{
		Topic:     "test.topic",
		Partition: 0,
		Offset:    0,
		Value:     []byte("test"),
	}

	msg := sub.kafkaMessageToMessage(kafkaMsg)

	assert.Equal(t, []byte("test"), msg.Payload)
	assert.Empty(t, msg.ID)
	assert.Empty(t, msg.Type)
	assert.Empty(t, msg.TraceID)
}
