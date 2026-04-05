package messaging

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.messaging/pkg/broker"

	"dev.helix.agent/internal/messaging"
)

// =============================================================================
// ConsumerGroupAdapter Tests
// =============================================================================

func TestNewConsumerGroupAdapter(t *testing.T) {
	t.Parallel()
	b := broker.NewInMemoryBroker()
	adapter := NewConsumerGroupAdapter("test-group", b)

	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.cg)
	assert.NotNil(t, adapter.metrics)
}

func TestConsumerGroupAdapter_ID(t *testing.T) {
	t.Parallel()
	b := broker.NewInMemoryBroker()
	adapter := NewConsumerGroupAdapter("my-group-id", b)

	assert.Equal(t, "my-group-id", adapter.ID())
}

func TestConsumerGroupAdapter_Add(t *testing.T) {
	t.Parallel()
	b := broker.NewInMemoryBroker()
	adapter := NewConsumerGroupAdapter("test-group", b)

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	// Should not panic
	adapter.Add("test-topic", handler)
}

func TestConsumerGroupAdapter_Topics(t *testing.T) {
	t.Parallel()
	b := broker.NewInMemoryBroker()
	adapter := NewConsumerGroupAdapter("test-group", b)

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	adapter.Add("topic-a", handler)
	adapter.Add("topic-b", handler)

	topics := adapter.Topics()
	assert.Len(t, topics, 2)
	assert.Contains(t, topics, "topic-a")
	assert.Contains(t, topics, "topic-b")
}

func TestConsumerGroupAdapter_IsRunning_BeforeStart(t *testing.T) {
	t.Parallel()
	b := broker.NewInMemoryBroker()
	adapter := NewConsumerGroupAdapter("test-group", b)

	assert.False(t, adapter.IsRunning())
}

func TestConsumerGroupAdapter_StartStop(t *testing.T) {
	t.Parallel()
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	_ = b.Connect(ctx)
	defer func() { _ = b.Close(ctx) }()

	adapter := NewConsumerGroupAdapter("test-group", b)

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}
	adapter.Add("test-topic", handler)

	err := adapter.Start(ctx)
	require.NoError(t, err)
	assert.True(t, adapter.IsRunning())

	err = adapter.Stop()
	require.NoError(t, err)
	assert.False(t, adapter.IsRunning())
}

// =============================================================================
// RetryPolicyAdapter Tests
// =============================================================================

func TestNewRetryPolicyAdapter(t *testing.T) {
	t.Parallel()
	adapter := NewRetryPolicyAdapter()
	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.rp)
}

func TestNewRetryPolicyAdapterWithConfig(t *testing.T) {
	t.Parallel()
	adapter := NewRetryPolicyAdapterWithConfig(
		5,
		100*time.Millisecond,
		10*time.Second,
		2.0,
	)

	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.rp)
	assert.Equal(t, 5, adapter.rp.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, adapter.rp.BackoffBase)
	assert.Equal(t, 10*time.Second, adapter.rp.BackoffMax)
	assert.Equal(t, 2.0, adapter.rp.BackoffMultiplier)
}

func TestRetryPolicyAdapter_Delay(t *testing.T) {
	t.Parallel()
	adapter := NewRetryPolicyAdapterWithConfig(
		3,
		100*time.Millisecond,
		5*time.Second,
		2.0,
	)

	delay0 := adapter.Delay(0)
	delay1 := adapter.Delay(1)

	// Delay should increase with attempts
	assert.True(t, delay1 >= delay0, "delay should increase with attempts")
}

func TestRetryPolicyAdapter_ShouldRetry(t *testing.T) {
	t.Parallel()
	adapter := NewRetryPolicyAdapterWithConfig(
		3,
		100*time.Millisecond,
		5*time.Second,
		2.0,
	)

	assert.True(t, adapter.ShouldRetry(0))
	assert.True(t, adapter.ShouldRetry(1))
	assert.True(t, adapter.ShouldRetry(2))
	assert.False(t, adapter.ShouldRetry(3))
	assert.False(t, adapter.ShouldRetry(10))
}

func TestRetryPolicyAdapter_Unwrap(t *testing.T) {
	t.Parallel()
	adapter := NewRetryPolicyAdapter()
	rp := adapter.Unwrap()
	assert.NotNil(t, rp)
}

// =============================================================================
// WithRetryAdapter Tests
// =============================================================================

func TestWithRetryAdapter_NilPolicy(t *testing.T) {
	t.Parallel()
	var called int32
	handler := func(ctx context.Context, msg *messaging.Message) error {
		atomic.AddInt32(&called, 1)
		return nil
	}

	// nil policy should use default
	retryHandler := WithRetryAdapter(handler, nil)
	assert.NotNil(t, retryHandler)

	msg := messaging.NewMessage("test", []byte("payload"))
	err := retryHandler(context.Background(), msg)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&called))
}

func TestWithRetryAdapter_WithPolicy(t *testing.T) {
	t.Parallel()
	var called int32
	handler := func(ctx context.Context, msg *messaging.Message) error {
		atomic.AddInt32(&called, 1)
		return nil
	}

	policy := NewRetryPolicyAdapterWithConfig(3, 10*time.Millisecond, 100*time.Millisecond, 2.0)
	retryHandler := WithRetryAdapter(handler, policy)
	assert.NotNil(t, retryHandler)

	msg := messaging.NewMessage("test", []byte("payload"))
	err := retryHandler(context.Background(), msg)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&called))
}

// =============================================================================
// DeadLetterHandlerAdapter Tests
// =============================================================================

func TestNewDeadLetterHandlerAdapter(t *testing.T) {
	t.Parallel()
	b := broker.NewInMemoryBroker()
	adapter := NewDeadLetterHandlerAdapter(b, "dlq-topic")

	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.dlh)
	assert.NotNil(t, adapter.metrics)
}

func TestDeadLetterHandlerAdapter_DLQTopic(t *testing.T) {
	t.Parallel()
	b := broker.NewInMemoryBroker()
	adapter := NewDeadLetterHandlerAdapter(b, "my-dlq")

	assert.Equal(t, "my-dlq", adapter.DLQTopic())
}

func TestDeadLetterHandlerAdapter_Count_Initially(t *testing.T) {
	t.Parallel()
	b := broker.NewInMemoryBroker()
	adapter := NewDeadLetterHandlerAdapter(b, "dlq-topic")

	assert.Equal(t, int64(0), adapter.Count())
}

func TestDeadLetterHandlerAdapter_Handle(t *testing.T) {
	t.Parallel()
	b := broker.NewInMemoryBroker()
	ctx := context.Background()
	_ = b.Connect(ctx)
	defer func() { _ = b.Close(ctx) }()

	adapter := NewDeadLetterHandlerAdapter(b, "dlq-topic")

	msg := messaging.NewMessage("failed-msg", []byte("error payload"))
	err := adapter.Handle(ctx, msg, assert.AnError)
	require.NoError(t, err)
	assert.Equal(t, int64(1), adapter.Count())
}

func TestDeadLetterHandlerAdapter_SetOnFailure(t *testing.T) {
	t.Parallel()
	b := broker.NewInMemoryBroker()
	adapter := NewDeadLetterHandlerAdapter(b, "dlq-topic")

	var callbackCalled int32
	adapter.SetOnFailure(func(ctx context.Context, msg *messaging.Message, err error) {
		atomic.AddInt32(&callbackCalled, 1)
	})

	// The callback is set, but is only invoked by internal DLH behavior
	assert.NotNil(t, adapter.dlh)
}

// =============================================================================
// BatchConsumerAdapter Tests
// =============================================================================

func TestNewBatchConsumerAdapter(t *testing.T) {
	t.Parallel()
	handler := func(ctx context.Context, msgs []*messaging.Message) error {
		return nil
	}

	adapter := NewBatchConsumerAdapter(10, time.Second, handler)
	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.bc)
	assert.NotNil(t, adapter.metrics)
}

func TestBatchConsumerAdapter_BatchSize(t *testing.T) {
	t.Parallel()
	handler := func(ctx context.Context, msgs []*messaging.Message) error {
		return nil
	}

	adapter := NewBatchConsumerAdapter(25, time.Second, handler)
	assert.Equal(t, 25, adapter.BatchSize())
}

func TestBatchConsumerAdapter_BufferLen_Empty(t *testing.T) {
	t.Parallel()
	handler := func(ctx context.Context, msgs []*messaging.Message) error {
		return nil
	}

	adapter := NewBatchConsumerAdapter(10, time.Second, handler)
	assert.Equal(t, 0, adapter.BufferLen())
}

func TestBatchConsumerAdapter_Add(t *testing.T) {
	t.Parallel()
	handler := func(ctx context.Context, msgs []*messaging.Message) error {
		return nil
	}

	adapter := NewBatchConsumerAdapter(10, time.Second, handler)

	msg := messaging.NewMessage("test", []byte("payload"))
	adapter.Add(msg)

	assert.Equal(t, 1, adapter.BufferLen())
}

func TestBatchConsumerAdapter_Add_Multiple(t *testing.T) {
	t.Parallel()
	handler := func(ctx context.Context, msgs []*messaging.Message) error {
		return nil
	}

	adapter := NewBatchConsumerAdapter(10, time.Second, handler)

	for i := 0; i < 5; i++ {
		msg := messaging.NewMessage("test", []byte("payload"))
		adapter.Add(msg)
	}

	assert.Equal(t, 5, adapter.BufferLen())
}

func TestBatchConsumerAdapter_Flush(t *testing.T) {
	t.Parallel()
	var receivedCount int32
	handler := func(ctx context.Context, msgs []*messaging.Message) error {
		atomic.AddInt32(&receivedCount, int32(len(msgs)))
		return nil
	}

	adapter := NewBatchConsumerAdapter(10, time.Second, handler)

	for i := 0; i < 3; i++ {
		msg := messaging.NewMessage("test", []byte("payload"))
		adapter.Add(msg)
	}

	err := adapter.Flush(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&receivedCount))
	assert.Equal(t, 0, adapter.BufferLen())
}

func TestBatchConsumerAdapter_AsHandler(t *testing.T) {
	t.Parallel()
	handler := func(ctx context.Context, msgs []*messaging.Message) error {
		return nil
	}

	adapter := NewBatchConsumerAdapter(10, time.Second, handler)
	msgHandler := adapter.AsHandler()

	assert.NotNil(t, msgHandler)

	msg := messaging.NewMessage("test", []byte("payload"))
	err := msgHandler(context.Background(), msg)
	assert.NoError(t, err)
	assert.Equal(t, 1, adapter.BufferLen())
}

func TestBatchConsumerAdapter_StartStop(t *testing.T) {
	t.Parallel()
	handler := func(ctx context.Context, msgs []*messaging.Message) error {
		return nil
	}

	adapter := NewBatchConsumerAdapter(10, 50*time.Millisecond, handler)

	ctx := context.Background()
	adapter.Start(ctx)

	// Add some messages
	for i := 0; i < 3; i++ {
		msg := messaging.NewMessage("test", []byte("payload"))
		adapter.Add(msg)
	}

	// Stop should flush remaining messages
	err := adapter.Stop(ctx)
	require.NoError(t, err)
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkRetryPolicyAdapter_Delay(b *testing.B) {
	adapter := NewRetryPolicyAdapterWithConfig(5, 100*time.Millisecond, 10*time.Second, 2.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.Delay(i % 5)
	}
}

func BenchmarkBatchConsumerAdapter_Add(b *testing.B) {
	handler := func(ctx context.Context, msgs []*messaging.Message) error {
		return nil
	}
	adapter := NewBatchConsumerAdapter(10000, time.Hour, handler)

	msg := messaging.NewMessage("test", []byte("payload"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.Add(msg)
	}
}
