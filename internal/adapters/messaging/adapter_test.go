package messaging

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.messaging/pkg/broker"

	"dev.helix.agent/internal/messaging"
)

func TestBrokerAdapter_Connect(t *testing.T) {
	t.Parallel()
	genericBroker := broker.NewInMemoryBroker()
	adapter := NewBrokerAdapter(genericBroker)

	ctx := context.Background()
	err := adapter.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, adapter.IsConnected())

	err = adapter.Close(ctx)
	require.NoError(t, err)
	assert.False(t, adapter.IsConnected())
}

func TestBrokerAdapter_PublishSubscribe(t *testing.T) {
	t.Parallel()
	genericBroker := broker.NewInMemoryBroker()
	adapter := NewBrokerAdapter(genericBroker)

	ctx := context.Background()
	err := adapter.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = adapter.Close(ctx) }()

	receivedCh := make(chan *messaging.Message, 1)
	handler := func(ctx context.Context, msg *messaging.Message) error {
		receivedCh <- msg
		return nil
	}

	sub, err := adapter.Subscribe(ctx, "test-topic", handler)
	require.NoError(t, err)
	assert.True(t, sub.IsActive())

	// Publish message
	msg := messaging.NewMessage("test", []byte("hello world"))
	msg.TraceID = "trace-123"
	msg.CorrelationID = "corr-456"

	err = adapter.Publish(ctx, "test-topic", msg)
	require.NoError(t, err)

	// Wait for message
	select {
	case received := <-receivedCh:
		assert.Equal(t, "hello world", string(received.Payload))
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}

	err = sub.Unsubscribe()
	require.NoError(t, err)
	assert.False(t, sub.IsActive())
}

func TestBrokerAdapter_BrokerType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		genericBroker broker.MessageBroker
		expectedType  messaging.BrokerType
	}{
		{
			name:          "InMemory",
			genericBroker: broker.NewInMemoryBroker(),
			expectedType:  messaging.BrokerTypeInMemory,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
			adapter := NewBrokerAdapter(tt.genericBroker)
			assert.Equal(t, tt.expectedType, adapter.BrokerType())
		})
	}
}

func TestBrokerAdapter_Metrics(t *testing.T) {
	t.Parallel()
	genericBroker := broker.NewInMemoryBroker()
	adapter := NewBrokerAdapter(genericBroker)

	ctx := context.Background()
	_ = adapter.Connect(ctx)
	defer func() { _ = adapter.Close(ctx) }()

	metrics := adapter.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, int64(1), metrics.ConnectionAttempts.Load())
	assert.Equal(t, int64(1), metrics.ConnectionSuccesses.Load())
}

func TestBrokerAdapter_Unwrap(t *testing.T) {
	t.Parallel()
	genericBroker := broker.NewInMemoryBroker()
	adapter := NewBrokerAdapter(genericBroker)

	unwrapped := adapter.Unwrap()
	assert.Equal(t, genericBroker, unwrapped)
}

func TestSubscriptionAdapter(t *testing.T) {
	t.Parallel()
	genericBroker := broker.NewInMemoryBroker()
	adapter := NewBrokerAdapter(genericBroker)

	ctx := context.Background()
	_ = adapter.Connect(ctx)
	defer func() { _ = adapter.Close(ctx) }()

	sub, err := adapter.Subscribe(ctx, "test-topic", func(ctx context.Context, msg *messaging.Message) error {
		return nil
	})
	require.NoError(t, err)

	assert.True(t, sub.IsActive())
	assert.Equal(t, "test-topic", sub.Topic())
	assert.NotEmpty(t, sub.ID())

	err = sub.Unsubscribe()
	require.NoError(t, err)
	assert.False(t, sub.IsActive())
}
