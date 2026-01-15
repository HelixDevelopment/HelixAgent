package kafka

import (
	"crypto/tls"
	"fmt"
	"time"
)

// Config holds Kafka connection configuration
type Config struct {
	// Broker settings
	Brokers []string `json:"brokers" yaml:"brokers"`
	ClientID string  `json:"client_id" yaml:"client_id"`

	// Security settings
	TLSEnabled    bool        `json:"tls_enabled" yaml:"tls_enabled"`
	TLSConfig     *tls.Config `json:"-" yaml:"-"`
	TLSSkipVerify bool        `json:"tls_skip_verify" yaml:"tls_skip_verify"`

	// SASL authentication
	SASLEnabled   bool   `json:"sasl_enabled" yaml:"sasl_enabled"`
	SASLMechanism string `json:"sasl_mechanism" yaml:"sasl_mechanism"` // PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
	SASLUsername  string `json:"sasl_username" yaml:"sasl_username"`
	SASLPassword  string `json:"sasl_password" yaml:"sasl_password"`

	// Producer settings
	RequiredAcks     int           `json:"required_acks" yaml:"required_acks"` // 0=none, 1=leader, -1=all
	MaxRetries       int           `json:"max_retries" yaml:"max_retries"`
	RetryBackoff     time.Duration `json:"retry_backoff" yaml:"retry_backoff"`
	BatchSize        int           `json:"batch_size" yaml:"batch_size"`
	BatchTimeout     time.Duration `json:"batch_timeout" yaml:"batch_timeout"`
	MaxMessageBytes  int           `json:"max_message_bytes" yaml:"max_message_bytes"`
	CompressionCodec string        `json:"compression_codec" yaml:"compression_codec"` // none, gzip, snappy, lz4, zstd
	Idempotent       bool          `json:"idempotent" yaml:"idempotent"`

	// Consumer settings
	GroupID           string        `json:"group_id" yaml:"group_id"`
	AutoOffsetReset   string        `json:"auto_offset_reset" yaml:"auto_offset_reset"` // earliest, latest
	EnableAutoCommit  bool          `json:"enable_auto_commit" yaml:"enable_auto_commit"`
	AutoCommitInterval time.Duration `json:"auto_commit_interval" yaml:"auto_commit_interval"`
	SessionTimeout    time.Duration `json:"session_timeout" yaml:"session_timeout"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval"`
	MaxPollRecords    int           `json:"max_poll_records" yaml:"max_poll_records"`
	MaxPollInterval   time.Duration `json:"max_poll_interval" yaml:"max_poll_interval"`
	FetchMinBytes     int           `json:"fetch_min_bytes" yaml:"fetch_min_bytes"`
	FetchMaxBytes     int           `json:"fetch_max_bytes" yaml:"fetch_max_bytes"`
	FetchMaxWait      time.Duration `json:"fetch_max_wait" yaml:"fetch_max_wait"`

	// Connection settings
	DialTimeout     time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	ReadTimeout     time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout" yaml:"write_timeout"`
	MetadataRefresh time.Duration `json:"metadata_refresh" yaml:"metadata_refresh"`

	// Topic settings
	DefaultPartitions   int `json:"default_partitions" yaml:"default_partitions"`
	DefaultReplication  int `json:"default_replication" yaml:"default_replication"`

	// Metrics
	EnableMetrics bool   `json:"enable_metrics" yaml:"enable_metrics"`
	MetricsPrefix string `json:"metrics_prefix" yaml:"metrics_prefix"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Brokers:           []string{"localhost:9092"},
		ClientID:          "helixagent",
		TLSEnabled:        false,
		TLSSkipVerify:     false,
		SASLEnabled:       false,
		SASLMechanism:     "PLAIN",
		RequiredAcks:      -1, // All replicas
		MaxRetries:        3,
		RetryBackoff:      100 * time.Millisecond,
		BatchSize:         16384, // 16KB
		BatchTimeout:      10 * time.Millisecond,
		MaxMessageBytes:   1048576, // 1MB
		CompressionCodec:  "lz4",
		Idempotent:        true,
		GroupID:           "helixagent-consumers",
		AutoOffsetReset:   "earliest",
		EnableAutoCommit:  false,
		AutoCommitInterval: 5 * time.Second,
		SessionTimeout:    30 * time.Second,
		HeartbeatInterval: 3 * time.Second,
		MaxPollRecords:    500,
		MaxPollInterval:   5 * time.Minute,
		FetchMinBytes:     1,
		FetchMaxBytes:     52428800, // 50MB
		FetchMaxWait:      500 * time.Millisecond,
		DialTimeout:       30 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		MetadataRefresh:   5 * time.Minute,
		DefaultPartitions: 3,
		DefaultReplication: 1,
		EnableMetrics:     true,
		MetricsPrefix:     "kafka",
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("at least one broker is required")
	}
	for _, broker := range c.Brokers {
		if broker == "" {
			return fmt.Errorf("broker address cannot be empty")
		}
	}
	if c.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}
	if c.BatchSize < 1 {
		return fmt.Errorf("batch_size must be at least 1")
	}
	if c.DefaultPartitions < 1 {
		return fmt.Errorf("default_partitions must be at least 1")
	}
	if c.DefaultReplication < 1 {
		return fmt.Errorf("default_replication must be at least 1")
	}
	if c.SASLEnabled && c.SASLUsername == "" {
		return fmt.Errorf("sasl_username is required when SASL is enabled")
	}
	return nil
}

// TopicConfig holds topic-specific configuration
type TopicConfig struct {
	Name              string                 `json:"name"`
	Partitions        int                    `json:"partitions"`
	ReplicationFactor int                    `json:"replication_factor"`
	RetentionMs       int64                  `json:"retention_ms"`
	RetentionBytes    int64                  `json:"retention_bytes"`
	CleanupPolicy     string                 `json:"cleanup_policy"` // delete, compact, delete,compact
	MinInsyncReplicas int                    `json:"min_insync_replicas"`
	Configs           map[string]string      `json:"configs"`
}

// DefaultTopicConfig returns a TopicConfig with defaults
func DefaultTopicConfig(name string) *TopicConfig {
	return &TopicConfig{
		Name:              name,
		Partitions:        3,
		ReplicationFactor: 1,
		RetentionMs:       604800000, // 7 days
		RetentionBytes:    -1,        // unlimited
		CleanupPolicy:     "delete",
		MinInsyncReplicas: 1,
		Configs:           make(map[string]string),
	}
}

// WithRetention sets retention settings
func (tc *TopicConfig) WithRetention(ms int64, bytes int64) *TopicConfig {
	tc.RetentionMs = ms
	tc.RetentionBytes = bytes
	return tc
}

// WithCompaction enables log compaction
func (tc *TopicConfig) WithCompaction() *TopicConfig {
	tc.CleanupPolicy = "compact"
	return tc
}

// WithPartitions sets the number of partitions
func (tc *TopicConfig) WithPartitions(n int) *TopicConfig {
	tc.Partitions = n
	return tc
}

// WithReplication sets the replication factor
func (tc *TopicConfig) WithReplication(n int) *TopicConfig {
	tc.ReplicationFactor = n
	return tc
}

// ConsumerGroupConfig holds consumer group configuration
type ConsumerGroupConfig struct {
	GroupID              string        `json:"group_id"`
	Topics               []string      `json:"topics"`
	StartFromBeginning   bool          `json:"start_from_beginning"`
	MaxConcurrency       int           `json:"max_concurrency"`
	CommitInterval       time.Duration `json:"commit_interval"`
	RebalanceTimeout     time.Duration `json:"rebalance_timeout"`
	RetryOnError         bool          `json:"retry_on_error"`
	MaxRetries           int           `json:"max_retries"`
	RetryBackoff         time.Duration `json:"retry_backoff"`
}

// DefaultConsumerGroupConfig returns a ConsumerGroupConfig with defaults
func DefaultConsumerGroupConfig(groupID string, topics []string) *ConsumerGroupConfig {
	return &ConsumerGroupConfig{
		GroupID:            groupID,
		Topics:             topics,
		StartFromBeginning: true,
		MaxConcurrency:     10,
		CommitInterval:     5 * time.Second,
		RebalanceTimeout:   60 * time.Second,
		RetryOnError:       true,
		MaxRetries:         3,
		RetryBackoff:       1 * time.Second,
	}
}
