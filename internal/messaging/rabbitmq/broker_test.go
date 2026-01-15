package rabbitmq

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
