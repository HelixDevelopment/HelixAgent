// Package kafka provides an Apache Kafka message broker implementation.
package kafka

import (
	"time"

	"dev.helix.agent/internal/messaging"
)

// Config holds configuration for the Kafka broker.
type Config struct {
	// Brokers is the list of Kafka broker addresses.
	Brokers []string `json:"brokers" yaml:"brokers"`

	// Client Configuration
	ClientID string `json:"client_id" yaml:"client_id"`
	GroupID  string `json:"group_id" yaml:"group_id"`

	// TLS Configuration
	TLS         bool   `json:"tls" yaml:"tls"`
	TLSCertFile string `json:"tls_cert_file,omitempty" yaml:"tls_cert_file,omitempty"`
	TLSKeyFile  string `json:"tls_key_file,omitempty" yaml:"tls_key_file,omitempty"`
	TLSCAFile   string `json:"tls_ca_file,omitempty" yaml:"tls_ca_file,omitempty"`
	TLSInsecure bool   `json:"tls_insecure,omitempty" yaml:"tls_insecure,omitempty"`

	// SASL Configuration
	SASLEnabled   bool   `json:"sasl_enabled" yaml:"sasl_enabled"`
	SASLMechanism string `json:"sasl_mechanism" yaml:"sasl_mechanism"` // PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
	SASLUsername  string `json:"sasl_username" yaml:"sasl_username"`
	SASLPassword  string `json:"sasl_password" yaml:"sasl_password"`

	// Connection settings
	DialTimeout  time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`

	// Producer settings
	RequiredAcks     RequiredAcks     `json:"required_acks" yaml:"required_acks"`
	Compression      CompressionCodec `json:"compression" yaml:"compression"`
	BatchSize        int              `json:"batch_size" yaml:"batch_size"`
	BatchBytes       int64            `json:"batch_bytes" yaml:"batch_bytes"`
	BatchTimeout     time.Duration    `json:"batch_timeout" yaml:"batch_timeout"`
	MaxAttempts      int              `json:"max_attempts" yaml:"max_attempts"`
	EnableIdempotent bool             `json:"enable_idempotent" yaml:"enable_idempotent"`

	// Consumer settings
	MinBytes             int           `json:"min_bytes" yaml:"min_bytes"`
	MaxBytes             int           `json:"max_bytes" yaml:"max_bytes"`
	MaxWait              time.Duration `json:"max_wait" yaml:"max_wait"`
	CommitInterval       time.Duration `json:"commit_interval" yaml:"commit_interval"`
	StartOffset          int64         `json:"start_offset" yaml:"start_offset"`
	SessionTimeout       time.Duration `json:"session_timeout" yaml:"session_timeout"`
	HeartbeatInterval    time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval"`
	RebalanceTimeout     time.Duration `json:"rebalance_timeout" yaml:"rebalance_timeout"`
	PartitionWatchInterval time.Duration `json:"partition_watch_interval" yaml:"partition_watch_interval"`
	WatchPartitionChanges  bool          `json:"watch_partition_changes" yaml:"watch_partition_changes"`

	// Topic settings
	AutoCreateTopics    bool  `json:"auto_create_topics" yaml:"auto_create_topics"`
	DefaultPartitions   int   `json:"default_partitions" yaml:"default_partitions"`
	DefaultReplication  int   `json:"default_replication" yaml:"default_replication"`
	DefaultRetentionMs  int64 `json:"default_retention_ms" yaml:"default_retention_ms"`
	DefaultSegmentBytes int64 `json:"default_segment_bytes" yaml:"default_segment_bytes"`
}

// RequiredAcks represents the number of acks required from kafka brokers.
type RequiredAcks int

const (
	// RequireNone does not wait for any broker acknowledgment.
	RequireNone RequiredAcks = 0
	// RequireOne waits for acknowledgment from the leader only.
	RequireOne RequiredAcks = 1
	// RequireAll waits for acknowledgment from all in-sync replicas.
	RequireAll RequiredAcks = -1
)

// String returns the string representation of RequiredAcks.
func (r RequiredAcks) String() string {
	switch r {
	case RequireNone:
		return "none"
	case RequireOne:
		return "leader"
	case RequireAll:
		return "all"
	default:
		return "unknown"
	}
}

// CompressionCodec represents the compression codec for messages.
type CompressionCodec int

const (
	// CompressionNone disables compression.
	CompressionNone CompressionCodec = iota
	// CompressionGzip uses gzip compression.
	CompressionGzip
	// CompressionSnappy uses snappy compression.
	CompressionSnappy
	// CompressionLZ4 uses lz4 compression.
	CompressionLZ4
	// CompressionZstd uses zstd compression.
	CompressionZstd
)

// String returns the string representation of CompressionCodec.
func (c CompressionCodec) String() string {
	switch c {
	case CompressionNone:
		return "none"
	case CompressionGzip:
		return "gzip"
	case CompressionSnappy:
		return "snappy"
	case CompressionLZ4:
		return "lz4"
	case CompressionZstd:
		return "zstd"
	default:
		return "unknown"
	}
}

// DefaultConfig returns the default Kafka configuration.
func DefaultConfig() *Config {
	return &Config{
		Brokers:              []string{"localhost:9092"},
		ClientID:             "helixagent",
		GroupID:              "helixagent-group",
		TLS:                  false,
		SASLEnabled:          false,
		SASLMechanism:        "PLAIN",
		DialTimeout:          10 * time.Second,
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         30 * time.Second,
		RequiredAcks:         RequireAll,
		Compression:          CompressionLZ4,
		BatchSize:            100,
		BatchBytes:           1024 * 1024,        // 1MB
		BatchTimeout:         10 * time.Millisecond,
		MaxAttempts:          3,
		EnableIdempotent:     true,
		MinBytes:             1,
		MaxBytes:             10 * 1024 * 1024, // 10MB
		MaxWait:              500 * time.Millisecond,
		CommitInterval:       5 * time.Second,
		StartOffset:          -1, // Latest
		SessionTimeout:       30 * time.Second,
		HeartbeatInterval:    3 * time.Second,
		RebalanceTimeout:     30 * time.Second,
		PartitionWatchInterval: 5 * time.Second,
		WatchPartitionChanges:  false,
		AutoCreateTopics:     false,
		DefaultPartitions:    3,
		DefaultReplication:   1,
		DefaultRetentionMs:   7 * 24 * 60 * 60 * 1000, // 7 days
		DefaultSegmentBytes:  1024 * 1024 * 1024,       // 1GB
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if len(c.Brokers) == 0 {
		return messaging.ConfigError("at least one broker is required")
	}
	for _, broker := range c.Brokers {
		if broker == "" {
			return messaging.ConfigError("broker address cannot be empty")
		}
	}
	if c.ClientID == "" {
		return messaging.ConfigError("client ID is required")
	}
	if c.TLS && c.TLSCertFile == "" && !c.TLSInsecure {
		return messaging.ConfigError("TLS certificate file is required when TLS is enabled")
	}
	if c.SASLEnabled && c.SASLUsername == "" {
		return messaging.ConfigError("SASL username is required when SASL is enabled")
	}
	if c.BatchSize <= 0 {
		return messaging.ConfigError("batch size must be positive")
	}
	if c.DefaultPartitions <= 0 {
		return messaging.ConfigError("default partitions must be positive")
	}
	if c.DefaultReplication <= 0 {
		return messaging.ConfigError("default replication must be positive")
	}
	return nil
}

// TopicConfig holds configuration for a Kafka topic.
type TopicConfig struct {
	Name              string            `json:"name" yaml:"name"`
	NumPartitions     int               `json:"num_partitions" yaml:"num_partitions"`
	ReplicationFactor int               `json:"replication_factor" yaml:"replication_factor"`
	ReplicaAssignment map[int32][]int32 `json:"replica_assignment,omitempty" yaml:"replica_assignment,omitempty"`
	ConfigEntries     map[string]string `json:"config_entries,omitempty" yaml:"config_entries,omitempty"`
}

// DefaultTopicConfig returns the default topic configuration.
func DefaultTopicConfig(name string) *TopicConfig {
	return &TopicConfig{
		Name:              name,
		NumPartitions:     3,
		ReplicationFactor: 1,
	}
}

// WithPartitions sets the number of partitions.
func (t *TopicConfig) WithPartitions(n int) *TopicConfig {
	t.NumPartitions = n
	return t
}

// WithReplicationFactor sets the replication factor.
func (t *TopicConfig) WithReplicationFactor(n int) *TopicConfig {
	t.ReplicationFactor = n
	return t
}

// WithConfig sets a configuration entry.
func (t *TopicConfig) WithConfig(key, value string) *TopicConfig {
	if t.ConfigEntries == nil {
		t.ConfigEntries = make(map[string]string)
	}
	t.ConfigEntries[key] = value
	return t
}

// WithRetentionMs sets the retention period in milliseconds.
func (t *TopicConfig) WithRetentionMs(ms int64) *TopicConfig {
	return t.WithConfig("retention.ms", string(rune(ms)))
}

// WithCleanupPolicy sets the cleanup policy.
func (t *TopicConfig) WithCleanupPolicy(policy string) *TopicConfig {
	return t.WithConfig("cleanup.policy", policy)
}

// PartitionerType represents the partitioner type.
type PartitionerType int

const (
	// PartitionerRoundRobin distributes messages in round-robin fashion.
	PartitionerRoundRobin PartitionerType = iota
	// PartitionerHash uses message key hash for partitioning.
	PartitionerHash
	// PartitionerManual uses explicitly set partition.
	PartitionerManual
)

// String returns the string representation of PartitionerType.
func (p PartitionerType) String() string {
	switch p {
	case PartitionerRoundRobin:
		return "round-robin"
	case PartitionerHash:
		return "hash"
	case PartitionerManual:
		return "manual"
	default:
		return "unknown"
	}
}

// ConsumerGroupConfig holds configuration for a consumer group.
type ConsumerGroupConfig struct {
	GroupID               string          `json:"group_id" yaml:"group_id"`
	Topics                []string        `json:"topics" yaml:"topics"`
	Balancer              BalancerType    `json:"balancer" yaml:"balancer"`
	SessionTimeout        time.Duration   `json:"session_timeout" yaml:"session_timeout"`
	HeartbeatInterval     time.Duration   `json:"heartbeat_interval" yaml:"heartbeat_interval"`
	RebalanceTimeout      time.Duration   `json:"rebalance_timeout" yaml:"rebalance_timeout"`
	StartOffset           int64           `json:"start_offset" yaml:"start_offset"`
	RetentionTime         time.Duration   `json:"retention_time" yaml:"retention_time"`
	PartitionWatchInterval time.Duration  `json:"partition_watch_interval" yaml:"partition_watch_interval"`
	WatchPartitionChanges  bool           `json:"watch_partition_changes" yaml:"watch_partition_changes"`
}

// BalancerType represents the consumer group balancer type.
type BalancerType int

const (
	// BalancerRange assigns partitions by ranges.
	BalancerRange BalancerType = iota
	// BalancerRoundRobin assigns partitions in round-robin fashion.
	BalancerRoundRobin
	// BalancerRackAware considers rack affinity when assigning partitions.
	BalancerRackAware
)

// String returns the string representation of BalancerType.
func (b BalancerType) String() string {
	switch b {
	case BalancerRange:
		return "range"
	case BalancerRoundRobin:
		return "round-robin"
	case BalancerRackAware:
		return "rack-aware"
	default:
		return "unknown"
	}
}

// DefaultConsumerGroupConfig returns the default consumer group configuration.
func DefaultConsumerGroupConfig(groupID string, topics []string) *ConsumerGroupConfig {
	return &ConsumerGroupConfig{
		GroupID:               groupID,
		Topics:                topics,
		Balancer:              BalancerRoundRobin,
		SessionTimeout:        30 * time.Second,
		HeartbeatInterval:     3 * time.Second,
		RebalanceTimeout:      30 * time.Second,
		StartOffset:           -1, // Latest
		RetentionTime:         24 * time.Hour,
		PartitionWatchInterval: 5 * time.Second,
		WatchPartitionChanges:  false,
	}
}

// DefaultTopics returns the default HelixAgent topics.
func DefaultTopics() []*TopicConfig {
	return []*TopicConfig{
		DefaultTopicConfig(messaging.TopicLLMResponses).WithPartitions(6),
		DefaultTopicConfig(messaging.TopicDebateRounds).WithPartitions(6),
		DefaultTopicConfig(messaging.TopicVerificationResults).WithPartitions(3),
		DefaultTopicConfig(messaging.TopicProviderHealth).WithPartitions(3),
		DefaultTopicConfig(messaging.TopicAuditLog).WithPartitions(6).WithRetentionMs(30 * 24 * 60 * 60 * 1000), // 30 days
		DefaultTopicConfig(messaging.TopicMetrics).WithPartitions(3),
		DefaultTopicConfig(messaging.TopicErrors).WithPartitions(3),
		DefaultTopicConfig(messaging.TopicTokenStream).WithPartitions(12),
		DefaultTopicConfig(messaging.TopicSSEEvents).WithPartitions(6),
		DefaultTopicConfig(messaging.TopicWebSocketMessages).WithPartitions(6),
	}
}
