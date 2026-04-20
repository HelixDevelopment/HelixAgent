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

	_, exists := processor.handlers.Get("test.message")

	assert.True(t, exists)
}

func TestProcessor_StartStop(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	ctx := context.Background()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = broker.Close(ctx) }()

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
	defer func() { _ = broker.Close(ctx) }()

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
	defer func() { _ = broker.Close(ctx) }()

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
	ctx := context.Background()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = broker.Close(ctx) }()

	config := DefaultConfig()
	logger := zap.NewNop()

	processor := NewProcessor(broker, config, logger)

	// Add a message first
	dlqMsg := &DeadLetterMessage{
		ID:            "test-message-id",
		OriginalTopic: "test.topic",
		OriginalMessage: &messaging.Message{
			ID:      "orig-1",
			Type:    "test.type",
			Payload: []byte(`{"test": "data"}`),
		},
		Status:        StatusPending,
		FailureReason: "test failure",
	}
	err = processor.AddMessage(ctx, dlqMsg)
	require.NoError(t, err)

	err = processor.ReprocessMessage(ctx, "test-message-id")
	require.NoError(t, err)
}

func TestProcessor_DiscardMessage(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()

	processor := NewProcessor(broker, config, logger)

	ctx := context.Background()

	// Add a message first
	dlqMsg := &DeadLetterMessage{
		ID:            "test-message-id",
		OriginalTopic: "test.topic",
		OriginalMessage: &messaging.Message{
			ID:   "orig-1",
			Type: "test.type",
		},
		Status:        StatusPending,
		FailureReason: "test failure",
	}
	err := processor.AddMessage(ctx, dlqMsg)
	require.NoError(t, err)

	err = processor.DiscardMessage(ctx, "test-message-id", "test reason")
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
	defer func() { _ = broker.Close(ctx) }()

	config := DefaultConfig()
	logger := zap.NewNop()

	processor := NewProcessor(broker, config, logger)

	// Set up a subscriber to verify message is published
	var received atomic.Value
	_, err = broker.Subscribe(ctx, "test.topic", func(ctx context.Context, msg *messaging.Message) error {
		received.Store(msg)
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

	receivedMsg := received.Load()
	assert.NotNil(t, receivedMsg)
	assert.Equal(t, "orig-1", receivedMsg.(*messaging.Message).ID)
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

func TestProcessor_AddMessage(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()
	processor := NewProcessor(broker, config, logger)
	ctx := context.Background()

	t.Run("add new message", func(t *testing.T) {
		dlqMsg := &DeadLetterMessage{
			ID:            "test-add-1",
			OriginalQueue: "test.queue",
			OriginalTopic: "test.topic",
			OriginalMessage: &messaging.Message{
				ID:      "orig-1",
				Type:    "test.type",
				Payload: []byte(`{"test": "data"}`),
			},
			FailureReason: "test failure",
		}

		err := processor.AddMessage(ctx, dlqMsg)
		require.NoError(t, err)

		// Verify message was added
		assert.Equal(t, 1, processor.GetMessageCount(ctx))

		// Verify message can be retrieved
		retrieved, err := processor.GetMessage(ctx, "test-add-1")
		require.NoError(t, err)
		assert.Equal(t, "test-add-1", retrieved.ID)
		assert.Equal(t, StatusPending, retrieved.Status)
		assert.False(t, retrieved.FirstFailure.IsZero())
		assert.False(t, retrieved.NextRetry.IsZero())
	})

	t.Run("reject duplicate message", func(t *testing.T) {
		dlqMsg := &DeadLetterMessage{
			ID:            "test-add-1", // Same ID as before
			OriginalTopic: "test.topic",
			OriginalMessage: &messaging.Message{
				ID:   "orig-2",
				Type: "test.type",
			},
			FailureReason: "another failure",
		}

		err := processor.AddMessage(ctx, dlqMsg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("reject empty ID", func(t *testing.T) {
		dlqMsg := &DeadLetterMessage{
			ID:            "",
			OriginalTopic: "test.topic",
		}

		err := processor.AddMessage(ctx, dlqMsg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ID is required")
	})
}

func TestProcessor_GetMessage(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()
	processor := NewProcessor(broker, config, logger)
	ctx := context.Background()

	t.Run("get existing message", func(t *testing.T) {
		dlqMsg := &DeadLetterMessage{
			ID:            "test-get-1",
			OriginalTopic: "test.topic",
			OriginalMessage: &messaging.Message{
				ID:   "orig-1",
				Type: "test.type",
			},
			FailureReason: "test failure",
		}
		err := processor.AddMessage(ctx, dlqMsg)
		require.NoError(t, err)

		retrieved, err := processor.GetMessage(ctx, "test-get-1")
		require.NoError(t, err)
		assert.Equal(t, "test-get-1", retrieved.ID)
	})

	t.Run("get non-existent message", func(t *testing.T) {
		_, err := processor.GetMessage(ctx, "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestProcessor_ListMessages(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()
	processor := NewProcessor(broker, config, logger)
	ctx := context.Background()

	// Add multiple messages with different failure times
	for i := 1; i <= 5; i++ {
		dlqMsg := &DeadLetterMessage{
			ID:            "test-list-" + string(rune('0'+i)),
			OriginalTopic: "test.topic",
			OriginalMessage: &messaging.Message{
				ID:   "orig-" + string(rune('0'+i)),
				Type: "test.type",
			},
			FailureReason: "test failure",
			LastFailure:   time.Now().Add(time.Duration(i) * time.Hour),
		}
		err := processor.AddMessage(ctx, dlqMsg)
		require.NoError(t, err)
	}

	t.Run("list all messages", func(t *testing.T) {
		messages, err := processor.ListMessages(ctx, 0, 0)
		require.NoError(t, err)
		assert.Len(t, messages, 5)
	})

	t.Run("list with limit", func(t *testing.T) {
		messages, err := processor.ListMessages(ctx, 2, 0)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
	})

	t.Run("list with offset", func(t *testing.T) {
		messages, err := processor.ListMessages(ctx, 0, 2)
		require.NoError(t, err)
		assert.Len(t, messages, 3)
	})

	t.Run("list with limit and offset", func(t *testing.T) {
		messages, err := processor.ListMessages(ctx, 2, 2)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
	})

	t.Run("offset beyond list", func(t *testing.T) {
		messages, err := processor.ListMessages(ctx, 0, 10)
		require.NoError(t, err)
		assert.Len(t, messages, 0)
	})
}

func TestProcessor_GetMessagesByStatus(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()
	processor := NewProcessor(broker, config, logger)
	ctx := context.Background()

	// Add messages with different statuses
	statuses := []DLQStatus{StatusPending, StatusPending, StatusRetrying, StatusProcessed, StatusDiscarded}
	for i, status := range statuses {
		dlqMsg := &DeadLetterMessage{
			ID:            "test-status-" + string(rune('0'+i)),
			OriginalTopic: "test.topic",
			OriginalMessage: &messaging.Message{
				ID:   "orig-" + string(rune('0'+i)),
				Type: "test.type",
			},
			Status: status,
		}
		processor.messages.Put(dlqMsg.ID, dlqMsg)
	}

	t.Run("get pending messages", func(t *testing.T) {
		messages, err := processor.GetMessagesByStatus(ctx, StatusPending, 0)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
	})

	t.Run("get retrying messages", func(t *testing.T) {
		messages, err := processor.GetMessagesByStatus(ctx, StatusRetrying, 0)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
	})

	t.Run("with limit", func(t *testing.T) {
		messages, err := processor.GetMessagesByStatus(ctx, StatusPending, 1)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
	})
}

func TestProcessor_ReprocessMessage_Full(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	ctx := context.Background()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer func() { _ = broker.Close(ctx) }()

	config := DefaultConfig()
	logger := zap.NewNop()
	processor := NewProcessor(broker, config, logger)

	t.Run("reprocess non-existent message", func(t *testing.T) {
		err := processor.ReprocessMessage(ctx, "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("reprocess already processed message", func(t *testing.T) {
		dlqMsg := &DeadLetterMessage{
			ID:            "test-reprocess-processed",
			OriginalTopic: "test.topic",
			OriginalMessage: &messaging.Message{
				ID:   "orig-1",
				Type: "test.type",
			},
			Status: StatusProcessed,
		}
		processor.messages.Put(dlqMsg.ID, dlqMsg)

		err := processor.ReprocessMessage(ctx, "test-reprocess-processed")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be reprocessed")
	})

	t.Run("successful reprocess", func(t *testing.T) {
		// Set up subscriber to receive reprocessed message
		var received atomic.Value
		_, err := broker.Subscribe(ctx, "reprocess.topic", func(ctx context.Context, msg *messaging.Message) error {
			received.Store(msg)
			return nil
		})
		require.NoError(t, err)

		dlqMsg := &DeadLetterMessage{
			ID:            "test-reprocess-success",
			OriginalTopic: "reprocess.topic",
			OriginalMessage: &messaging.Message{
				ID:      "orig-reprocess",
				Type:    "test.type",
				Payload: []byte(`{"reprocess": true}`),
			},
			Status:        StatusPending,
			FailureReason: "initial failure",
		}
		processor.messages.Put(dlqMsg.ID, dlqMsg)

		err = processor.ReprocessMessage(ctx, "test-reprocess-success")
		require.NoError(t, err)

		// Wait for message delivery
		time.Sleep(100 * time.Millisecond)

		// Verify message was republished
		assert.NotNil(t, received.Load())

		// Verify status was updated
		updatedMsg, _ := processor.messages.Get("test-reprocess-success")
		assert.Equal(t, StatusProcessed, updatedMsg.Status)
	})
}

func TestProcessor_DiscardMessage_Full(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()
	processor := NewProcessor(broker, config, logger)
	ctx := context.Background()

	t.Run("discard non-existent message", func(t *testing.T) {
		err := processor.DiscardMessage(ctx, "non-existent", "test reason")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("discard already discarded message", func(t *testing.T) {
		dlqMsg := &DeadLetterMessage{
			ID:     "test-discard-already",
			Status: StatusDiscarded,
		}
		processor.messages.Put(dlqMsg.ID, dlqMsg)

		err := processor.DiscardMessage(ctx, "test-discard-already", "test reason")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already discarded")
	})

	t.Run("successful discard", func(t *testing.T) {
		dlqMsg := &DeadLetterMessage{
			ID:            "test-discard-success",
			OriginalTopic: "test.topic",
			OriginalMessage: &messaging.Message{
				ID:   "orig-1",
				Type: "test.type",
			},
			Status:        StatusPending,
			FailureReason: "initial failure",
		}
		processor.messages.Put(dlqMsg.ID, dlqMsg)

		err := processor.DiscardMessage(ctx, "test-discard-success", "manually discarded")
		require.NoError(t, err)

		// Verify status was updated
		updatedMsg, _ := processor.messages.Get("test-discard-success")
		assert.Equal(t, StatusDiscarded, updatedMsg.Status)
		assert.Equal(t, "manually discarded", updatedMsg.FailureDetails["discard_reason"])
	})
}

func TestProcessor_PurgeProcessed(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()
	processor := NewProcessor(broker, config, logger)
	ctx := context.Background()

	// Add messages with different statuses
	statuses := map[string]DLQStatus{
		"pending-1":   StatusPending,
		"pending-2":   StatusPending,
		"retrying-1":  StatusRetrying,
		"processed-1": StatusProcessed,
		"processed-2": StatusProcessed,
		"discarded-1": StatusDiscarded,
	}

	for id, status := range statuses {
		dlqMsg := &DeadLetterMessage{
			ID:     id,
			Status: status,
		}
		processor.messages.Put(id, dlqMsg)
	}

	assert.Equal(t, 6, processor.GetMessageCount(ctx))

	count, err := processor.PurgeProcessed(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count) // processed-1, processed-2, discarded-1

	// Verify remaining messages
	assert.Equal(t, 3, processor.GetMessageCount(ctx))

	// Verify pending and retrying are still there
	for _, id := range []string{"pending-1", "pending-2", "retrying-1"} {
		_, err := processor.GetMessage(ctx, id)
		assert.NoError(t, err)
	}

	// Verify processed and discarded are gone
	for _, id := range []string{"processed-1", "processed-2", "discarded-1"} {
		_, err := processor.GetMessage(ctx, id)
		assert.Error(t, err)
	}
}

func TestProcessor_UpdateDLQMessage_QueueDepth(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()
	processor := NewProcessor(broker, config, logger)
	ctx := context.Background()

	// Add multiple messages and check queue depth
	for i := 1; i <= 3; i++ {
		dlqMsg := &DeadLetterMessage{
			ID:            "test-depth-" + string(rune('0'+i)),
			OriginalTopic: "test.topic",
			OriginalMessage: &messaging.Message{
				ID:   "orig-" + string(rune('0'+i)),
				Type: "test.type",
			},
			Status: StatusPending,
		}
		err := processor.updateDLQMessage(ctx, dlqMsg)
		require.NoError(t, err)
	}

	metrics := processor.GetMetrics()
	assert.Equal(t, int64(3), metrics.CurrentQueueDepth)

	// Mark one as processed
	if m1, ok := processor.messages.Get("test-depth-1"); ok {
		m1.Status = StatusProcessed
	}

	// Update to recalculate depth
	m2, _ := processor.messages.Get("test-depth-2")
	err := processor.updateDLQMessage(ctx, m2)
	require.NoError(t, err)

	metrics = processor.GetMetrics()
	assert.Equal(t, int64(2), metrics.CurrentQueueDepth)
}

func TestProcessor_Messages_Concurrency(t *testing.T) {
	broker := inmemory.NewBroker(nil)
	config := DefaultConfig()
	logger := zap.NewNop()
	processor := NewProcessor(broker, config, logger)
	ctx := context.Background()

	// Concurrent adds
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			dlqMsg := &DeadLetterMessage{
				ID:            "concurrent-" + string(rune('a'+idx)),
				OriginalTopic: "test.topic",
				OriginalMessage: &messaging.Message{
					ID:   "orig-" + string(rune('a'+idx)),
					Type: "test.type",
				},
			}
			_ = processor.AddMessage(ctx, dlqMsg)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all messages were added
	assert.Equal(t, 10, processor.GetMessageCount(ctx))

	// Concurrent reads and writes
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = processor.ListMessages(ctx, 0, 0)
			done <- true
		}()
		go func() {
			_, _ = processor.GetMessagesByStatus(ctx, StatusPending, 0)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}
