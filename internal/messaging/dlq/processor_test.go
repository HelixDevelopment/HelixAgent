package dlq

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.agent/internal/messaging"
	"dev.helix.agent/internal/messaging/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewProcessor(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()

	processor := NewProcessor(broker, config, logger)

	assert.NotNil(t, processor)
	assert.Equal(t, config, processor.config)
	assert.NotNil(t, processor.handlers)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
	assert.Equal(t, 2.0, config.RetryBackoffMultiplier)
	assert.Equal(t, 30*time.Second, config.MaxRetryDelay)
	assert.Equal(t, 30*time.Second, config.ProcessingTimeout)
	assert.Equal(t, 10, config.BatchSize)
	assert.Equal(t, 5*time.Second, config.PollInterval)
}

func TestProcessor_RegisterHandler(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()
	processor := NewProcessor(broker, config, logger)

	handler := func(ctx context.Context, msg *DeadLetterMessage) error {
		return nil
	}

	processor.RegisterHandler("test.message", handler)

	processor.mu.RLock()
	_, exists := processor.handlers["test.message"]
	processor.mu.RUnlock()

	assert.True(t, exists)
}

func TestProcessor_StartStop(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	ctx := context.Background()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	config := DefaultConfig()
	config.PollInterval = 100 * time.Millisecond
	logger := zap.NewNop()

	processor := NewProcessor(broker, config, logger)

	// Start processor
	err = processor.Start(ctx)
	require.NoError(t, err)
	assert.True(t, processor.running.Load())

	// Starting again should error
	err = processor.Start(ctx)
	assert.Error(t, err)

	// Stop processor
	err = processor.Stop()
	require.NoError(t, err)
	assert.False(t, processor.running.Load())

	// Stopping again should be fine
	err = processor.Stop()
	require.NoError(t, err)
}

func TestProcessor_CalculateRetryDelay(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	config.RetryDelay = 1 * time.Second
	config.RetryBackoffMultiplier = 2.0
	config.MaxRetryDelay = 30 * time.Second
	logger := zap.NewNop()

	processor := NewProcessor(broker, config, logger)

	tests := []struct {
		retryCount int
		expected   time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 30 * time.Second}, // Capped at max
		{7, 30 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			delay := processor.calculateRetryDelay(tt.retryCount)
			assert.Equal(t, tt.expected, delay)
		})
	}
}

func TestProcessor_GetMetrics(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()

	processor := NewProcessor(broker, config, logger)

	// Set some metrics
	atomic.StoreInt64(&processor.metrics.MessagesProcessed, 100)
	atomic.StoreInt64(&processor.metrics.MessagesRetried, 20)
	atomic.StoreInt64(&processor.metrics.MessagesDiscarded, 5)
	atomic.StoreInt64(&processor.metrics.ProcessingErrors, 2)

	metrics := processor.GetMetrics()

	assert.Equal(t, int64(100), metrics.MessagesProcessed)
	assert.Equal(t, int64(20), metrics.MessagesRetried)
	assert.Equal(t, int64(5), metrics.MessagesDiscarded)
	assert.Equal(t, int64(2), metrics.ProcessingErrors)
}

func TestProcessor_ProcessMessage_MaxRetries(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	ctx := context.Background()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	config := DefaultConfig()
	config.MaxRetries = 3
	logger := zap.NewNop()

	processor := NewProcessor(broker, config, logger)

	// Create a DLQ message that has exceeded max retries
	dlqMsg := &DeadLetterMessage{
		ID:            "test-msg-1",
		OriginalQueue: "test.queue",
		OriginalTopic: "test.topic",
		OriginalMessage: &messaging.Message{
			ID:      "orig-msg-1",
			Type:    "test.type",
			Payload: []byte(`{"test": "data"}`),
		},
		FailureReason:  "test failure",
		FailureDetails: make(map[string]interface{}),
		RetryCount:     4, // Exceeded max of 3
		FirstFailure:   time.Now().Add(-1 * time.Hour),
		LastFailure:    time.Now(),
		Status:         StatusPending,
	}

	payload, _ := json.Marshal(dlqMsg)
	msg := &messaging.Message{
		ID:        "dlq-msg-1",
		Type:      "dlq.message",
		Payload:   payload,
		Timestamp: time.Now(),
	}

	err = processor.processMessage(ctx, msg)
	require.NoError(t, err)

	// Should be discarded
	assert.Equal(t, int64(1), atomic.LoadInt64(&processor.metrics.MessagesDiscarded))
}

func TestProcessor_ProcessMessage_InvalidFormat(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	ctx := context.Background()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	config := DefaultConfig()
	logger := zap.NewNop()

	processor := NewProcessor(broker, config, logger)

	// Create an invalid message
	msg := &messaging.Message{
		ID:        "invalid-msg",
		Type:      "invalid",
		Payload:   []byte(`not valid json{`),
		Timestamp: time.Now(),
	}

	err = processor.processMessage(ctx, msg)
	require.NoError(t, err)

	// Should be discarded due to invalid format
	assert.Equal(t, int64(1), atomic.LoadInt64(&processor.metrics.MessagesDiscarded))
}

func TestProcessor_ReprocessMessage(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()

	processor := NewProcessor(broker, config, logger)

	ctx := context.Background()
	err := processor.ReprocessMessage(ctx, "test-message-id")
	require.NoError(t, err)
}

func TestProcessor_DiscardMessage(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()

	processor := NewProcessor(broker, config, logger)

	ctx := context.Background()
	err := processor.DiscardMessage(ctx, "test-message-id", "test reason")
	require.NoError(t, err)

	assert.Equal(t, int64(1), atomic.LoadInt64(&processor.metrics.MessagesDiscarded))
}

func TestDeadLetterMessage_Serialization(t *testing.T) {
	dlqMsg := &DeadLetterMessage{
		ID:            "dlq-123",
		OriginalQueue: "helixagent.tasks.llm",
		OriginalTopic: "helixagent.events.llm.responses",
		OriginalMessage: &messaging.Message{
			ID:      "orig-456",
			Type:    "llm.response",
			Payload: []byte(`{"response": "test"}`),
			Headers: map[string]string{"trace-id": "trace-789"},
		},
		FailureReason: "timeout exceeded",
		FailureDetails: map[string]interface{}{
			"attempt":  3,
			"duration": "30s",
		},
		RetryCount:   2,
		FirstFailure: time.Now().Add(-1 * time.Hour),
		LastFailure:  time.Now(),
		NextRetry:    time.Now().Add(5 * time.Minute),
		Status:       StatusRetrying,
	}

	// Serialize
	data, err := json.Marshal(dlqMsg)
	require.NoError(t, err)

	// Deserialize
	var restored DeadLetterMessage
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, dlqMsg.ID, restored.ID)
	assert.Equal(t, dlqMsg.OriginalQueue, restored.OriginalQueue)
	assert.Equal(t, dlqMsg.FailureReason, restored.FailureReason)
	assert.Equal(t, dlqMsg.RetryCount, restored.RetryCount)
	assert.Equal(t, dlqMsg.Status, restored.Status)
}

func TestDLQStatus_Values(t *testing.T) {
	tests := []struct {
		status   DLQStatus
		expected string
	}{
		{StatusPending, "pending"},
		{StatusRetrying, "retrying"},
		{StatusProcessed, "processed"},
		{StatusDiscarded, "discarded"},
		{StatusExpired, "expired"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestProcessor_DefaultRetryHandler(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	ctx := context.Background()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	config := DefaultConfig()
	logger := zap.NewNop()

	processor := NewProcessor(broker, config, logger)

	// Set up a subscriber to verify message is published
	var received *messaging.Message
	_, err = broker.Subscribe(ctx, "test.topic", func(ctx context.Context, msg *messaging.Message) error {
		received = msg
		return nil
	})
	require.NoError(t, err)

	// Create DLQ message with original topic
	dlqMsg := &DeadLetterMessage{
		ID:            "dlq-1",
		OriginalTopic: "test.topic",
		OriginalMessage: &messaging.Message{
			ID:      "orig-1",
			Type:    "test.type",
			Payload: []byte(`{"test": "data"}`),
		},
	}

	err = processor.defaultRetryHandler(ctx, dlqMsg)
	require.NoError(t, err)

	// Wait for message delivery
	time.Sleep(100 * time.Millisecond)

	assert.NotNil(t, received)
	assert.Equal(t, "orig-1", received.ID)
}

func TestProcessor_DefaultRetryHandler_NoTopic(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()

	processor := NewProcessor(broker, config, logger)

	// Create DLQ message without original topic
	dlqMsg := &DeadLetterMessage{
		ID:            "dlq-1",
		OriginalTopic: "",
		OriginalMessage: &messaging.Message{
			ID:   "orig-1",
			Type: "test.type",
		},
	}

	ctx := context.Background()
	err := processor.defaultRetryHandler(ctx, dlqMsg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no original topic specified")
}
