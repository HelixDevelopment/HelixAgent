package messaging

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/messaging"
)

func TestInMemoryBrokerAdapter_Connect(t *testing.T) {
	t.Parallel()
	adapter := NewInMemoryBrokerAdapter()

	ctx := context.Background()
	err := adapter.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, adapter.IsConnected())

	err = adapter.HealthCheck(ctx)
	require.NoError(t, err)

	err = adapter.Close(ctx)
	require.NoError(t, err)
	assert.False(t, adapter.IsConnected())
}

func TestInMemoryBrokerAdapter_PublishSubscribe(t *testing.T) {
	t.Parallel()
	adapter := NewInMemoryBrokerAdapter()

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
	msg := messaging.NewMessage("test", []byte("hello from inmemory"))
	err = adapter.Publish(ctx, "test-topic", msg)
	require.NoError(t, err)

	// Wait for message
	select {
	case received := <-receivedCh:
		assert.Equal(t, "hello from inmemory", string(received.Payload))
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}

	err = sub.Unsubscribe()
	require.NoError(t, err)
}

func TestInMemoryBrokerAdapter_PublishBatch(t *testing.T) {
	t.Parallel()
	adapter := NewInMemoryBrokerAdapter()

	ctx := context.Background()
	err := adapter.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = adapter.Close(ctx) }()

	var receivedCount int64
	receivedCh := make(chan struct{}, 3)
	handler := func(ctx context.Context, msg *messaging.Message) error {
		atomic.AddInt64(&receivedCount, 1)
		receivedCh <- struct{}{}
		return nil
	}

	_, err = adapter.Subscribe(ctx, "batch-topic", handler)
	require.NoError(t, err)

	// Publish batch
	messages := []*messaging.Message{
		messaging.NewMessage("test", []byte("msg1")),
		messaging.NewMessage("test", []byte("msg2")),
		messaging.NewMessage("test", []byte("msg3")),
	}

	err = adapter.PublishBatch(ctx, "batch-topic", messages)
	require.NoError(t, err)

	// Wait for all messages
	for i := 0; i < 3; i++ {
		select {
		case <-receivedCh:
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for message %d", i+1)
		}
	}
}

func TestInMemoryBrokerAdapter_BrokerType(t *testing.T) {
	t.Parallel()
	adapter := NewInMemoryBrokerAdapter()
	assert.Equal(t, messaging.BrokerTypeInMemory, adapter.BrokerType())
}

func TestInMemoryBrokerAdapter_Metrics(t *testing.T) {
	t.Parallel()
	adapter := NewInMemoryBrokerAdapter()

	ctx := context.Background()
	_ = adapter.Connect(ctx)
	defer func() { _ = adapter.Close(ctx) }()

	metrics := adapter.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, int64(1), metrics.ConnectionAttempts.Load())
}

func TestInMemoryBrokerAdapter_Unwrap(t *testing.T) {
	t.Parallel()
	adapter := NewInMemoryBrokerAdapter()
	assert.NotNil(t, adapter.Unwrap())
}
