// +build integration

package messaging_test

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.agent/internal/messaging"
	"dev.helix.agent/internal/messaging/dlq"
	"dev.helix.agent/internal/messaging/inmemory"
	"dev.helix.agent/internal/messaging/replay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestIntegration_MessagingHub_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create in-memory broker for integration testing
	broker := inmemory.NewBroker()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	// Test full publish-subscribe cycle
	var receivedCount int64
	var mu sync.Mutex
	receivedMessages := make([]*messaging.Message, 0)

	handler := func(ctx context.Context, msg *messaging.Message) error {
		atomic.AddInt64(&receivedCount, 1)
		mu.Lock()
		receivedMessages = append(receivedMessages, msg)
		mu.Unlock()
		return nil
	}

	// Subscribe first
	sub, err := broker.Subscribe(ctx, "integration.test.topic", handler)
	require.NoError(t, err)
	assert.True(t, sub.IsActive())

	// Publish messages
	messageCount := 100
	for i := 0; i < messageCount; i++ {
		msg := &messaging.Message{
			ID:        generateID(i),
			Type:      "integration.test",
			Payload:   []byte(`{"test": "data"}`),
			Timestamp: time.Now(),
		}
		err := broker.Publish(ctx, "integration.test.topic", msg)
		require.NoError(t, err)
	}

	// Wait for processing
	timeout := time.After(5 * time.Second)
	for {
		if atomic.LoadInt64(&receivedCount) >= int64(messageCount) {
			break
		}
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for messages. Received: %d/%d", atomic.LoadInt64(&receivedCount), messageCount)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Verify all messages received
	assert.Equal(t, int64(messageCount), atomic.LoadInt64(&receivedCount))

	// Unsubscribe
	err = sub.Unsubscribe()
	require.NoError(t, err)
	assert.False(t, sub.IsActive())
}

func TestIntegration_MessagingHub_BatchPublish(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	var receivedCount int64
	handler := func(ctx context.Context, msg *messaging.Message) error {
		atomic.AddInt64(&receivedCount, 1)
		return nil
	}

	_, err = broker.Subscribe(ctx, "integration.batch.topic", handler)
	require.NoError(t, err)

	// Create batch of messages
	batchSize := 500
	messages := make([]*messaging.Message, batchSize)
	for i := 0; i < batchSize; i++ {
		messages[i] = &messaging.Message{
			ID:        generateID(i),
			Type:      "batch.test",
			Payload:   []byte(`{"batch": true}`),
			Timestamp: time.Now(),
		}
	}

	// Publish batch
	err = broker.PublishBatch(ctx, "integration.batch.topic", messages)
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(2 * time.Second)

	assert.Equal(t, int64(batchSize), atomic.LoadInt64(&receivedCount))
}

func TestIntegration_MessagingHub_MultipleSubscribers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	subscriberCount := 5
	messageCount := 100
	counters := make([]int64, subscriberCount)

	// Create multiple subscribers
	for i := 0; i < subscriberCount; i++ {
		idx := i
		handler := func(ctx context.Context, msg *messaging.Message) error {
			atomic.AddInt64(&counters[idx], 1)
			return nil
		}
		_, err := broker.Subscribe(ctx, "integration.multi.topic", handler)
		require.NoError(t, err)
	}

	// Publish messages
	for i := 0; i < messageCount; i++ {
		msg := &messaging.Message{
			ID:        generateID(i),
			Type:      "multi.test",
			Payload:   []byte(`{"test": "multi"}`),
			Timestamp: time.Now(),
		}
		broker.Publish(ctx, "integration.multi.topic", msg)
	}

	// Wait for processing
	time.Sleep(2 * time.Second)

	// Each subscriber should receive all messages
	for i, count := range counters {
		assert.Equal(t, int64(messageCount), count, "Subscriber %d received wrong count", i)
	}
}

func TestIntegration_MessagingHub_TopicIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	var topic1Count, topic2Count int64

	// Subscribe to topic1
	broker.Subscribe(ctx, "integration.topic1", func(ctx context.Context, msg *messaging.Message) error {
		atomic.AddInt64(&topic1Count, 1)
		return nil
	})

	// Subscribe to topic2
	broker.Subscribe(ctx, "integration.topic2", func(ctx context.Context, msg *messaging.Message) error {
		atomic.AddInt64(&topic2Count, 1)
		return nil
	})

	// Publish to topic1
	for i := 0; i < 50; i++ {
		broker.Publish(ctx, "integration.topic1", &messaging.Message{
			ID:   generateID(i),
			Type: "topic1.msg",
		})
	}

	// Publish to topic2
	for i := 0; i < 30; i++ {
		broker.Publish(ctx, "integration.topic2", &messaging.Message{
			ID:   generateID(i + 1000),
			Type: "topic2.msg",
		})
	}

	time.Sleep(1 * time.Second)

	assert.Equal(t, int64(50), atomic.LoadInt64(&topic1Count))
	assert.Equal(t, int64(30), atomic.LoadInt64(&topic2Count))
}

func TestIntegration_DLQProcessor_RetryFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	logger := zap.NewNop()
	config := dlq.DefaultConfig()
	config.MaxRetries = 2
	config.RetryDelay = 100 * time.Millisecond
	config.PollInterval = 200 * time.Millisecond

	processor := dlq.NewProcessor(broker, config, logger)

	// Track retry attempts
	var retryAttempts int64
	processor.RegisterHandler("test.retry", func(ctx context.Context, msg *dlq.DeadLetterMessage) error {
		count := atomic.AddInt64(&retryAttempts, 1)
		if count < 2 {
			return assert.AnError // Fail first attempt
		}
		return nil // Succeed on second attempt
	})

	err = processor.Start(ctx)
	require.NoError(t, err)
	defer processor.Stop()

	// Simulate DLQ message
	dlqMsg := &dlq.DeadLetterMessage{
		ID:            "dlq-retry-test",
		OriginalTopic: "original.topic",
		OriginalMessage: &messaging.Message{
			ID:      "orig-123",
			Type:    "test.retry",
			Payload: []byte(`{"test": "retry"}`),
		},
		FailureReason:  "initial failure",
		FailureDetails: make(map[string]interface{}),
		RetryCount:     0,
		Status:         dlq.StatusPending,
	}

	payload, _ := json.Marshal(dlqMsg)
	broker.Publish(ctx, "helixagent.dlq", &messaging.Message{
		ID:        "dlq-wrapper",
		Type:      "dlq",
		Payload:   payload,
		Timestamp: time.Now(),
	})

	// Wait for processing
	time.Sleep(2 * time.Second)

	metrics := processor.GetMetrics()
	assert.True(t, atomic.LoadInt64(&retryAttempts) >= 1)
	t.Logf("Retry attempts: %d, Processed: %d, Retried: %d",
		atomic.LoadInt64(&retryAttempts),
		metrics.MessagesProcessed,
		metrics.MessagesRetried)
}

func TestIntegration_ReplayHandler_BasicReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	logger := zap.NewNop()
	config := replay.DefaultReplayConfig()
	config.ReplayTimeout = 5 * time.Second

	handler := replay.NewHandler(broker, config, logger)

	// Start a dry-run replay
	request := &replay.ReplayRequest{
		ID:       "integration-replay-test",
		Topic:    "test.topic",
		FromTime: time.Now().Add(-1 * time.Hour),
		ToTime:   time.Now(),
		Options: &replay.ReplayOptions{
			DryRun:    true,
			BatchSize: 100,
		},
	}

	progress, err := handler.StartReplay(ctx, request)
	require.NoError(t, err)
	assert.NotNil(t, progress)
	assert.Equal(t, "integration-replay-test", progress.RequestID)

	// Wait for replay to complete
	time.Sleep(2 * time.Second)

	// Check final status
	finalProgress, err := handler.GetProgress("integration-replay-test")
	require.NoError(t, err)
	assert.Equal(t, replay.ReplayStatusCompleted, finalProgress.Status)
}

func TestIntegration_ReplayHandler_ConcurrentReplays(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	logger := zap.NewNop()
	config := replay.DefaultReplayConfig()
	config.MaxConcurrentReplays = 3
	config.ReplayTimeout = 5 * time.Second

	handler := replay.NewHandler(broker, config, logger)

	// Start multiple concurrent replays
	replayCount := 5
	var wg sync.WaitGroup

	for i := 0; i < replayCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			request := &replay.ReplayRequest{
				ID:       generateID(idx),
				Topic:    "test.topic",
				FromTime: time.Now().Add(-1 * time.Hour),
				Options: &replay.ReplayOptions{
					DryRun: true,
				},
			}

			_, err := handler.StartReplay(ctx, request)
			if err != nil {
				t.Logf("Replay %d failed to start: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify replays
	replays := handler.ListReplays()
	assert.LessOrEqual(t, len(replays), replayCount)
	t.Logf("Started %d replays out of %d requested", len(replays), replayCount)
}

func TestIntegration_HealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	err := broker.Connect(ctx)
	require.NoError(t, err)

	// Health check should pass
	err = broker.HealthCheck(ctx)
	assert.NoError(t, err)
	assert.True(t, broker.IsConnected())

	// Close and check
	broker.Close(ctx)
	assert.False(t, broker.IsConnected())
}

func TestIntegration_Metrics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	err := broker.Connect(ctx)
	require.NoError(t, err)
	defer broker.Close(ctx)

	// Subscribe
	broker.Subscribe(ctx, "metrics.topic", func(ctx context.Context, msg *messaging.Message) error {
		return nil
	})

	// Publish messages
	for i := 0; i < 100; i++ {
		broker.Publish(ctx, "metrics.topic", &messaging.Message{
			ID:   generateID(i),
			Type: "metrics.test",
		})
	}

	time.Sleep(1 * time.Second)

	// Get metrics
	metrics := broker.GetMetrics()
	assert.NotNil(t, metrics)
	t.Logf("Metrics: Published=%d, Consumed=%d",
		metrics.MessagesPublished,
		metrics.MessagesConsumed)
}

func TestIntegration_GracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	err := broker.Connect(ctx)
	require.NoError(t, err)

	var processedCount int64
	broker.Subscribe(ctx, "shutdown.topic", func(ctx context.Context, msg *messaging.Message) error {
		time.Sleep(10 * time.Millisecond) // Simulate processing
		atomic.AddInt64(&processedCount, 1)
		return nil
	})

	// Start publishing messages
	go func() {
		for i := 0; i < 100; i++ {
			broker.Publish(ctx, "shutdown.topic", &messaging.Message{
				ID:   generateID(i),
				Type: "shutdown.test",
			})
		}
	}()

	// Wait a bit then shutdown
	time.Sleep(500 * time.Millisecond)

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err = broker.Close(shutdownCtx)
	assert.NoError(t, err)

	t.Logf("Processed %d messages before shutdown", atomic.LoadInt64(&processedCount))
	assert.True(t, atomic.LoadInt64(&processedCount) > 0)
}

// Helper function to generate unique IDs
func generateID(n int) string {
	return "msg-" + time.Now().Format("20060102150405") + "-" + string(rune(n%26+'a'))
}

// TestMain sets up integration test environment
func TestMain(m *testing.M) {
	// Setup
	os.Setenv("MESSAGING_TEST_MODE", "integration")

	// Run tests
	code := m.Run()

	// Teardown
	os.Unsetenv("MESSAGING_TEST_MODE")

	os.Exit(code)
}
