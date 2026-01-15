package rabbitmq

import (
	"crypto/tls"
	"fmt"
	"time"
)

// Config holds RabbitMQ connection configuration
type Config struct {
	// Connection settings
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	VHost    string `json:"vhost" yaml:"vhost"`

	// TLS configuration
	TLSEnabled    bool        `json:"tls_enabled" yaml:"tls_enabled"`
	TLSConfig     *tls.Config `json:"-" yaml:"-"`
	TLSSkipVerify bool        `json:"tls_skip_verify" yaml:"tls_skip_verify"`

	// Connection pool settings
	MaxConnections     int           `json:"max_connections" yaml:"max_connections"`
	MaxChannels        int           `json:"max_channels" yaml:"max_channels"`
	ConnectionTimeout  time.Duration `json:"connection_timeout" yaml:"connection_timeout"`
	HeartbeatInterval  time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval"`
	ReconnectDelay     time.Duration `json:"reconnect_delay" yaml:"reconnect_delay"`
	MaxReconnectDelay  time.Duration `json:"max_reconnect_delay" yaml:"max_reconnect_delay"`
	ReconnectBackoff   float64       `json:"reconnect_backoff" yaml:"reconnect_backoff"`
	MaxReconnectCount  int           `json:"max_reconnect_count" yaml:"max_reconnect_count"`

	// Publisher settings
	PublishConfirm     bool          `json:"publish_confirm" yaml:"publish_confirm"`
	PublishTimeout     time.Duration `json:"publish_timeout" yaml:"publish_timeout"`
	MandatoryPublish   bool          `json:"mandatory_publish" yaml:"mandatory_publish"`
	ImmediatePublish   bool          `json:"immediate_publish" yaml:"immediate_publish"`

	// Consumer settings
	PrefetchCount      int           `json:"prefetch_count" yaml:"prefetch_count"`
	PrefetchSize       int           `json:"prefetch_size" yaml:"prefetch_size"`
	ConsumerTag        string        `json:"consumer_tag" yaml:"consumer_tag"`
	AutoAck            bool          `json:"auto_ack" yaml:"auto_ack"`
	Exclusive          bool          `json:"exclusive" yaml:"exclusive"`
	NoLocal            bool          `json:"no_local" yaml:"no_local"`
	NoWait             bool          `json:"no_wait" yaml:"no_wait"`

	// Queue defaults
	DefaultQueueDurable    bool `json:"default_queue_durable" yaml:"default_queue_durable"`
	DefaultQueueAutoDelete bool `json:"default_queue_auto_delete" yaml:"default_queue_auto_delete"`
	DefaultQueueExclusive  bool `json:"default_queue_exclusive" yaml:"default_queue_exclusive"`

	// Exchange defaults
	DefaultExchangeType    string `json:"default_exchange_type" yaml:"default_exchange_type"`
	DefaultExchangeDurable bool   `json:"default_exchange_durable" yaml:"default_exchange_durable"`

	// Dead letter queue
	EnableDLQ       bool   `json:"enable_dlq" yaml:"enable_dlq"`
	DLQExchange     string `json:"dlq_exchange" yaml:"dlq_exchange"`
	DLQRoutingKey   string `json:"dlq_routing_key" yaml:"dlq_routing_key"`
	DLQMessageTTL   int    `json:"dlq_message_ttl" yaml:"dlq_message_ttl"`
	DLQMaxLength    int    `json:"dlq_max_length" yaml:"dlq_max_length"`

	// Metrics and tracing
	EnableMetrics bool   `json:"enable_metrics" yaml:"enable_metrics"`
	MetricsPrefix string `json:"metrics_prefix" yaml:"metrics_prefix"`
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Host:               "localhost",
		Port:               5672,
		Username:           "guest",
		Password:           "guest",
		VHost:              "/",
		TLSEnabled:         false,
		TLSSkipVerify:      false,
		MaxConnections:     10,
		MaxChannels:        100,
		ConnectionTimeout:  30 * time.Second,
		HeartbeatInterval:  10 * time.Second,
		ReconnectDelay:     1 * time.Second,
		MaxReconnectDelay:  60 * time.Second,
		ReconnectBackoff:   2.0,
		MaxReconnectCount:  0, // 0 = unlimited
		PublishConfirm:     true,
		PublishTimeout:     30 * time.Second,
		MandatoryPublish:   false,
		ImmediatePublish:   false,
		PrefetchCount:      10,
		PrefetchSize:       0,
		ConsumerTag:        "",
		AutoAck:            false,
		Exclusive:          false,
		NoLocal:            false,
		NoWait:             false,
		DefaultQueueDurable:    true,
		DefaultQueueAutoDelete: false,
		DefaultQueueExclusive:  false,
		DefaultExchangeType:    "topic",
		DefaultExchangeDurable: true,
		EnableDLQ:              true,
		DLQExchange:            "helixagent.dlq",
		DLQRoutingKey:          "dlq",
		DLQMessageTTL:          86400000, // 24 hours in milliseconds
		DLQMaxLength:           100000,
		EnableMetrics:          true,
		MetricsPrefix:          "rabbitmq",
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if c.Username == "" {
		return fmt.Errorf("username is required")
	}
	if c.MaxConnections < 1 {
		return fmt.Errorf("max_connections must be at least 1")
	}
	if c.MaxChannels < 1 {
		return fmt.Errorf("max_channels must be at least 1")
	}
	if c.ConnectionTimeout < 0 {
		return fmt.Errorf("connection_timeout cannot be negative")
	}
	if c.PrefetchCount < 0 {
		return fmt.Errorf("prefetch_count cannot be negative")
	}
	return nil
}

// QueueConfig holds queue-specific configuration
type QueueConfig struct {
	Name       string                 `json:"name"`
	Durable    bool                   `json:"durable"`
	AutoDelete bool                   `json:"auto_delete"`
	Exclusive  bool                   `json:"exclusive"`
	NoWait     bool                   `json:"no_wait"`
	Args       map[string]interface{} `json:"args"`

	// Dead letter settings
	DeadLetterExchange   string `json:"dead_letter_exchange"`
	DeadLetterRoutingKey string `json:"dead_letter_routing_key"`

	// TTL and limits
	MessageTTL int `json:"message_ttl"` // milliseconds
	MaxLength  int `json:"max_length"`
	MaxBytes   int `json:"max_bytes"`

	// Priority
	MaxPriority int `json:"max_priority"` // 0-255
}

// ExchangeConfig holds exchange-specific configuration
type ExchangeConfig struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"` // direct, topic, fanout, headers
	Durable    bool                   `json:"durable"`
	AutoDelete bool                   `json:"auto_delete"`
	Internal   bool                   `json:"internal"`
	NoWait     bool                   `json:"no_wait"`
	Args       map[string]interface{} `json:"args"`
}

// BindingConfig holds binding configuration
type BindingConfig struct {
	Queue      string                 `json:"queue"`
	Exchange   string                 `json:"exchange"`
	RoutingKey string                 `json:"routing_key"`
	NoWait     bool                   `json:"no_wait"`
	Args       map[string]interface{} `json:"args"`
}

// DefaultQueueConfig returns a QueueConfig with defaults
func DefaultQueueConfig(name string) *QueueConfig {
	return &QueueConfig{
		Name:       name,
		Durable:    true,
		AutoDelete: false,
		Exclusive:  false,
		NoWait:     false,
		Args:       make(map[string]interface{}),
	}
}

// DefaultExchangeConfig returns an ExchangeConfig with defaults
func DefaultExchangeConfig(name string) *ExchangeConfig {
	return &ExchangeConfig{
		Name:       name,
		Type:       "topic",
		Durable:    true,
		AutoDelete: false,
		Internal:   false,
		NoWait:     false,
		Args:       make(map[string]interface{}),
	}
}

// WithDLQ adds dead letter queue configuration to a QueueConfig
func (qc *QueueConfig) WithDLQ(exchange, routingKey string) *QueueConfig {
	qc.DeadLetterExchange = exchange
	qc.DeadLetterRoutingKey = routingKey
	if qc.Args == nil {
		qc.Args = make(map[string]interface{})
	}
	qc.Args["x-dead-letter-exchange"] = exchange
	qc.Args["x-dead-letter-routing-key"] = routingKey
	return qc
}

// WithTTL adds message TTL to a QueueConfig
func (qc *QueueConfig) WithTTL(ttlMs int) *QueueConfig {
	qc.MessageTTL = ttlMs
	if qc.Args == nil {
		qc.Args = make(map[string]interface{})
	}
	qc.Args["x-message-ttl"] = ttlMs
	return qc
}

// WithMaxLength adds max length to a QueueConfig
func (qc *QueueConfig) WithMaxLength(maxLength int) *QueueConfig {
	qc.MaxLength = maxLength
	if qc.Args == nil {
		qc.Args = make(map[string]interface{})
	}
	qc.Args["x-max-length"] = maxLength
	return qc
}

// WithPriority adds priority support to a QueueConfig
func (qc *QueueConfig) WithPriority(maxPriority int) *QueueConfig {
	qc.MaxPriority = maxPriority
	if qc.Args == nil {
		qc.Args = make(map[string]interface{})
	}
	qc.Args["x-max-priority"] = maxPriority
	return qc
}
