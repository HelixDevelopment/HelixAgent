// Package messaging provides a unified abstraction layer for message brokers.
// It supports RabbitMQ for task queuing and Apache Kafka for event streaming,
// with an in-memory fallback for testing and development.
package messaging

import (
	"context"
	"encoding/json"
	"time"

	"dev.helix.agent/internal/netaddr"
)

// BrokerType represents the type of message broker.
type BrokerType string

const (
	// BrokerTypeRabbitMQ represents RabbitMQ message broker.
	BrokerTypeRabbitMQ BrokerType = "rabbitmq"
	// BrokerTypeKafka represents Apache Kafka message broker.
	BrokerTypeKafka BrokerType = "kafka"
	// BrokerTypeInMemory represents in-memory message broker for testing.
	BrokerTypeInMemory BrokerType = "inmemory"
)

// String returns the string representation of BrokerType.
func (bt BrokerType) String() string {
	return string(bt)
}

// IsValid checks if the BrokerType is valid.
func (bt BrokerType) IsValid() bool {
	switch bt {
	case BrokerTypeRabbitMQ, BrokerTypeKafka, BrokerTypeInMemory:
		return true
	default:
		return false
	}
}

// MessagePriority represents the priority level of a message.
type MessagePriority int

const (
	// PriorityLow is for background tasks that can wait.
	PriorityLow MessagePriority = 1
	// PriorityNormal is the default priority.
	PriorityNormal MessagePriority = 5
	// PriorityHigh is for time-sensitive operations.
	PriorityHigh MessagePriority = 8
	// PriorityCritical is for urgent operations that must be processed immediately.
	PriorityCritical MessagePriority = 10
)

// DeliveryMode represents message persistence mode.
type DeliveryMode int

const (
	// DeliveryModeTransient means messages may be lost on broker restart.
	DeliveryModeTransient DeliveryMode = 1
	// DeliveryModePersistent means messages survive broker restart.
	DeliveryModePersistent DeliveryMode = 2
)

// MessageState represents the state of a message in the system.
type MessageState string

const (
	// MessageStatePending indicates the message is waiting to be processed.
	MessageStatePending MessageState = "pending"
	// MessageStateProcessing indicates the message is being processed.
	MessageStateProcessing MessageState = "processing"
	// MessageStateCompleted indicates the message was processed successfully.
	MessageStateCompleted MessageState = "completed"
	// MessageStateFailed indicates the message processing failed.
	MessageStateFailed MessageState = "failed"
	// MessageStateDeadLettered indicates the message was moved to dead letter queue.
	MessageStateDeadLettered MessageState = "dead_lettered"
)

// Message represents a message in the messaging system.
type Message struct {
	// ID is the unique identifier for the message.
	ID string `json:"id"`
	// Type is the message type for routing and handling.
	Type string `json:"type"`
	// Payload is the message content as raw bytes.
	Payload []byte `json:"payload"`
	// Headers contains message metadata.
	Headers map[string]string `json:"headers,omitempty"`
	// Timestamp is when the message was created.
	Timestamp time.Time `json:"timestamp"`
	// Priority is the message priority level.
	Priority MessagePriority `json:"priority"`
	// RetryCount is the number of times this message has been retried.
	RetryCount int `json:"retry_count"`
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int `json:"max_retries"`
	// TraceID is for distributed tracing.
	TraceID string `json:"trace_id,omitempty"`
	// CorrelationID links related messages together.
	CorrelationID string `json:"correlation_id,omitempty"`
	// ReplyTo specifies where to send the response.
	ReplyTo string `json:"reply_to,omitempty"`
	// Expiration is when the message expires (zero means never).
	Expiration time.Time `json:"expiration,omitempty"`
	// DeliveryMode specifies persistence behavior.
	DeliveryMode DeliveryMode `json:"delivery_mode"`
	// State is the current state of the message.
	State MessageState `json:"state"`
	// Error contains error information if the message failed.
	Error string `json:"error,omitempty"`
	// DeliveryTag is used for acknowledgment (broker-specific).
	DeliveryTag uint64 `json:"-"`
	// Redelivered indicates if the message was previously delivered.
	Redelivered bool `json:"redelivered,omitempty"`
	// Partition is the Kafka partition (if applicable).
	Partition int32 `json:"partition,omitempty"`
	// Offset is the Kafka offset (if applicable).
	Offset int64 `json:"offset,omitempty"`
}

// NewMessage creates a new message with default values.
func NewMessage(msgType string, payload []byte) *Message {
	return &Message{
		ID:           generateMessageID(),
		Type:         msgType,
		Payload:      payload,
		Headers:      make(map[string]string),
		Timestamp:    time.Now().UTC(),
		Priority:     PriorityNormal,
		RetryCount:   0,
		MaxRetries:   3,
		DeliveryMode: DeliveryModePersistent,
		State:        MessageStatePending,
	}
}

// NewMessageWithID creates a new message with a specific ID.
func NewMessageWithID(id, msgType string, payload []byte) *Message {
	msg := NewMessage(msgType, payload)
	msg.ID = id
	return msg
}

// SetHeader sets a header value.
func (m *Message) SetHeader(key, value string) *Message {
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[key] = value
	return m
}

// GetHeader gets a header value.
func (m *Message) GetHeader(key string) string {
	if m.Headers == nil {
		return ""
	}
	return m.Headers[key]
}

// WithPriority sets the message priority.
func (m *Message) WithPriority(priority MessagePriority) *Message {
	m.Priority = priority
	return m
}

// WithTraceID sets the trace ID.
func (m *Message) WithTraceID(traceID string) *Message {
	m.TraceID = traceID
	return m
}

// WithCorrelationID sets the correlation ID.
func (m *Message) WithCorrelationID(correlationID string) *Message {
	m.CorrelationID = correlationID
	return m
}

// WithReplyTo sets the reply-to destination.
func (m *Message) WithReplyTo(replyTo string) *Message {
	m.ReplyTo = replyTo
	return m
}

// WithExpiration sets the message expiration.
func (m *Message) WithExpiration(expiration time.Time) *Message {
	m.Expiration = expiration
	return m
}

// WithTTL sets the message time-to-live from now.
func (m *Message) WithTTL(ttl time.Duration) *Message {
	m.Expiration = time.Now().UTC().Add(ttl)
	return m
}

// WithMaxRetries sets the maximum retry attempts.
func (m *Message) WithMaxRetries(maxRetries int) *Message {
	m.MaxRetries = maxRetries
	return m
}

// WithDeliveryMode sets the delivery mode.
func (m *Message) WithDeliveryMode(mode DeliveryMode) *Message {
	m.DeliveryMode = mode
	return m
}

// IsExpired checks if the message has expired.
func (m *Message) IsExpired() bool {
	if m.Expiration.IsZero() {
		return false
	}
	return time.Now().UTC().After(m.Expiration)
}

// CanRetry checks if the message can be retried.
func (m *Message) CanRetry() bool {
	return m.RetryCount < m.MaxRetries
}

// IncrementRetry increments the retry count.
func (m *Message) IncrementRetry() {
	m.RetryCount++
}

// Clone creates a deep copy of the message.
func (m *Message) Clone() *Message {
	clone := &Message{
		ID:            m.ID,
		Type:          m.Type,
		Payload:       make([]byte, len(m.Payload)),
		Timestamp:     m.Timestamp,
		Priority:      m.Priority,
		RetryCount:    m.RetryCount,
		MaxRetries:    m.MaxRetries,
		TraceID:       m.TraceID,
		CorrelationID: m.CorrelationID,
		ReplyTo:       m.ReplyTo,
		Expiration:    m.Expiration,
		DeliveryMode:  m.DeliveryMode,
		State:         m.State,
		Error:         m.Error,
		DeliveryTag:   m.DeliveryTag,
		Redelivered:   m.Redelivered,
		Partition:     m.Partition,
		Offset:        m.Offset,
	}
	copy(clone.Payload, m.Payload)
	if m.Headers != nil {
		clone.Headers = make(map[string]string)
		for k, v := range m.Headers {
			clone.Headers[k] = v
		}
	}
	return clone
}

// MarshalJSON implements json.Marshaler.
func (m *Message) MarshalJSON() ([]byte, error) {
	type Alias Message
	return json.Marshal(&struct {
		*Alias
		PayloadBase64 string `json:"payload_base64,omitempty"`
	}{
		Alias: (*Alias)(m),
	})
}

// MessageHandler is a function that processes messages.
type MessageHandler func(ctx context.Context, msg *Message) error

// MessageFilter is a function that filters messages.
type MessageFilter func(msg *Message) bool

// Subscription represents an active subscription to a topic or queue.
type Subscription interface {
	// Unsubscribe cancels the subscription.
	Unsubscribe() error
	// IsActive returns true if the subscription is still active.
	IsActive() bool
	// Topic returns the subscribed topic or queue name.
	Topic() string
	// ID returns the subscription identifier.
	ID() string
}

// MessageBroker defines the core interface for message brokers.
type MessageBroker interface {
	// Connect establishes a connection to the broker.
	Connect(ctx context.Context) error
	// Close closes the connection to the broker.
	Close(ctx context.Context) error
	// HealthCheck checks if the broker is healthy.
	HealthCheck(ctx context.Context) error
	// IsConnected returns true if connected to the broker.
	IsConnected() bool
	// Publish sends a message to a topic or queue.
	Publish(ctx context.Context, topic string, message *Message, opts ...PublishOption) error
	// PublishBatch sends multiple messages to a topic or queue.
	PublishBatch(ctx context.Context, topic string, messages []*Message, opts ...PublishOption) error
	// Subscribe creates a subscription to a topic or queue.
	Subscribe(ctx context.Context, topic string, handler MessageHandler, opts ...SubscribeOption) (Subscription, error)
	// BrokerType returns the type of this broker.
	BrokerType() BrokerType
	// GetMetrics returns broker metrics.
	GetMetrics() *BrokerMetrics
}

// BrokerConfig holds common configuration for message brokers.
type BrokerConfig struct {
	// Type is the broker type.
	Type BrokerType `json:"type" yaml:"type"`
	// Host is the broker host address.
	Host string `json:"host" yaml:"host"`
	// Port is the broker port.
	Port int `json:"port" yaml:"port"`
	// Username for authentication.
	Username string `json:"username" yaml:"username"`
	// Password for authentication.
	Password string `json:"password" yaml:"password"`
	// VirtualHost is the RabbitMQ virtual host (RabbitMQ only).
	VirtualHost string `json:"virtual_host,omitempty" yaml:"virtual_host,omitempty"`
	// TLS enables TLS/SSL connection.
	TLS bool `json:"tls" yaml:"tls"`
	// TLSCertFile is the path to TLS certificate file.
	TLSCertFile string `json:"tls_cert_file,omitempty" yaml:"tls_cert_file,omitempty"`
	// TLSKeyFile is the path to TLS key file.
	TLSKeyFile string `json:"tls_key_file,omitempty" yaml:"tls_key_file,omitempty"`
	// TLSCAFile is the path to TLS CA certificate file.
	TLSCAFile string `json:"tls_ca_file,omitempty" yaml:"tls_ca_file,omitempty"`
	// ConnectTimeout is the connection timeout.
	ConnectTimeout time.Duration `json:"connect_timeout" yaml:"connect_timeout"`
	// ReconnectInterval is the interval between reconnection attempts.
	ReconnectInterval time.Duration `json:"reconnect_interval" yaml:"reconnect_interval"`
	// MaxReconnectAttempts is the maximum number of reconnection attempts (0 = infinite).
	MaxReconnectAttempts int `json:"max_reconnect_attempts" yaml:"max_reconnect_attempts"`
	// Heartbeat is the heartbeat interval.
	Heartbeat time.Duration `json:"heartbeat" yaml:"heartbeat"`
}

// DefaultBrokerConfig returns a default broker configuration.
func DefaultBrokerConfig() *BrokerConfig {
	return &BrokerConfig{
		Type:                 BrokerTypeInMemory,
		Host:                 "localhost",
		Port:                 5672,
		VirtualHost:          "/",
		TLS:                  false,
		ConnectTimeout:       30 * time.Second,
		ReconnectInterval:    5 * time.Second,
		MaxReconnectAttempts: 0, // Infinite
		Heartbeat:            60 * time.Second,
	}
}

// Validate validates the broker configuration.
func (c *BrokerConfig) Validate() error {
	if !c.Type.IsValid() {
		return NewBrokerError(ErrCodeInvalidConfig, "invalid broker type", nil)
	}
	if c.Host == "" {
		return NewBrokerError(ErrCodeInvalidConfig, "host is required", nil)
	}
	if c.Port <= 0 || c.Port > 65535 {
		return NewBrokerError(ErrCodeInvalidConfig, "invalid port number", nil)
	}
	if c.TLS && c.TLSCertFile == "" {
		return NewBrokerError(ErrCodeInvalidConfig, "TLS certificate file is required when TLS is enabled", nil)
	}
	return nil
}

// ConnectionString returns the connection string for the broker.
//
// HXC-286: naive "Host + \":\" + strconv.Itoa(Port)" composition breaks the
// authority's host:port splittability for an unbracketed IPv6 host (see
// address_composition_red_test.go for the measured failure and the
// reader-choice finding). RabbitMQ is a message broker HelixAgent does not
// own, so this uses netaddr.DialAddressForURL — bracket-safe, RFC 6874
// zone-encoded for URL embedding, no default-substitution.
//
// Userinfo escaping (Username/Password) is deliberately NOT changed here — a
// distinct defect class, out of scope for HXC-286 and still OPEN. A password
// containing "@" or ":" is spliced in raw below and will mis-parse; that is a
// real gap, stated rather than implied fixed (§11.4.6). See
// docs/research/address_composition_family_20260812/ANALYSIS.md IN THE
// META-REPO (that path does not exist inside this repository) for the family
// survey this scope decision came from.
//
// Reachability note (§11.4.6): this method currently has NO production
// caller — measured module-wide, the only non-test reference to
// ConnectionString() is an unrelated same-named method on
// internal/vectordb/pgvector's Config. An earlier revision of this note added
// that "the live AMQP path is internal/messaging/rabbitmq/connection.go's
// buildURL, which is separately wired"; that consolation was FALSE and is
// withdrawn — re-measured 2026-08-13, package internal/messaging/rabbitmq is
// absent from ALL EIGHT ./cmd/* dependency closures, so buildURL is no more
// wired than this method is. Both are corrected so a future caller inherits
// the right behaviour; neither is evidence of user impact, and this RED test
// proves a synthetic failure, not an end-user one.
func (c *BrokerConfig) ConnectionString() string {
	scheme := "amqp"
	if c.TLS {
		scheme = "amqps"
	}
	authority := netaddr.DialAddressForURL(c.Host, c.Port)
	if c.Username != "" {
		return scheme + "://" + c.Username + ":" + c.Password + "@" + authority + c.VirtualHost
	}
	return scheme + "://" + authority + c.VirtualHost
}

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	return time.Now().UTC().Format("20060102150405") + "-" + randomString(12)
}

// randomString generates a random alphanumeric string.
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond) // Ensure different values
	}
	return string(b)
}
