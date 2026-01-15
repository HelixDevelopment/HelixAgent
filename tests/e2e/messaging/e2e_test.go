// +build e2e

package messaging_e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

// E2E test configuration
var (
	testCtx    context.Context
	testCancel context.CancelFunc
	testLogger *zap.Logger
)

func TestMain(m *testing.M) {
	// Setup
	testCtx, testCancel = context.WithTimeout(context.Background(), 5*time.Minute)
	testLogger, _ = zap.NewDevelopment()

	// Run tests
	code := m.Run()

	// Teardown
	testCancel()
	testLogger.Sync()

	os.Exit(code)
}

// TestE2E_FullMessageLifecycle tests complete message flow from publish to consume
func TestE2E_FullMessageLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(testCtx, 60*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	require.NoError(t, broker.Connect(ctx))
	defer broker.Close(ctx)

	// Track message states
	type MessageState struct {
		Published time.Time
		Received  time.Time
		Processed time.Time
	}
	states := make(map[string]*MessageState)

	// Set up consumer
	var processedCount int64
	_, err := broker.Subscribe(ctx, "e2e.lifecycle.topic", func(ctx context.Context, msg *messaging.Message) error {
		if state, ok := states[msg.ID]; ok {
			state.Received = time.Now()
		}

		// Simulate processing
		time.Sleep(5 * time.Millisecond)

		if state, ok := states[msg.ID]; ok {
			state.Processed = time.Now()
		}

		atomic.AddInt64(&processedCount, 1)
		return nil
	})
	require.NoError(t, err)

	// Publish messages
	messageCount := 50
	for i := 0; i < messageCount; i++ {
		msgID := fmt.Sprintf("lifecycle-msg-%d", i)
		states[msgID] = &MessageState{Published: time.Now()}

		msg := &messaging.Message{
			ID:        msgID,
			Type:      "lifecycle.test",
			Payload:   []byte(fmt.Sprintf(`{"index": %d}`, i)),
			Headers:   map[string]string{"test": "e2e"},
			Timestamp: time.Now(),
		}
		require.NoError(t, broker.Publish(ctx, "e2e.lifecycle.topic", msg))
	}

	// Wait for all messages to be processed
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&processedCount) >= int64(messageCount) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	assert.Equal(t, int64(messageCount), atomic.LoadInt64(&processedCount))

	// Verify latencies
	var totalLatency time.Duration
	var processedMessages int
	for _, state := range states {
		if !state.Processed.IsZero() {
			latency := state.Processed.Sub(state.Published)
			totalLatency += latency
			processedMessages++
		}
	}

	if processedMessages > 0 {
		avgLatency := totalLatency / time.Duration(processedMessages)
		t.Logf("Average end-to-end latency: %v", avgLatency)
		assert.Less(t, avgLatency, 1*time.Second, "Latency too high")
	}
}

// TestE2E_DLQProcessingFlow tests complete DLQ workflow
func TestE2E_DLQProcessingFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(testCtx, 60*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	require.NoError(t, broker.Connect(ctx))
	defer broker.Close(ctx)

	// Set up DLQ processor
	config := dlq.DefaultConfig()
	config.MaxRetries = 3
	config.RetryDelay = 100 * time.Millisecond
	config.RetryBackoffMultiplier = 1.5
	config.PollInterval = 200 * time.Millisecond

	processor := dlq.NewProcessor(broker, config, testLogger)

	// Track retry behavior
	var failedAttempts, successfulRetries int64
	processor.RegisterHandler("e2e.dlq.test", func(ctx context.Context, msg *dlq.DeadLetterMessage) error {
		attempts := msg.RetryCount
		if attempts < 2 {
			atomic.AddInt64(&failedAttempts, 1)
			return fmt.Errorf("simulated failure on attempt %d", attempts)
		}
		atomic.AddInt64(&successfulRetries, 1)
		return nil
	})

	require.NoError(t, processor.Start(ctx))
	defer processor.Stop()

	// Create and publish DLQ message
	dlqMsg := &dlq.DeadLetterMessage{
		ID:            "e2e-dlq-test-1",
		OriginalTopic: "original.topic",
		OriginalMessage: &messaging.Message{
			ID:      "orig-e2e-1",
			Type:    "e2e.dlq.test",
			Payload: []byte(`{"test": "e2e"}`),
		},
		FailureReason:  "initial test failure",
		FailureDetails: map[string]interface{}{"test": true},
		RetryCount:     0,
		FirstFailure:   time.Now(),
		LastFailure:    time.Now(),
		Status:         dlq.StatusPending,
	}

	payload, _ := json.Marshal(dlqMsg)
	require.NoError(t, broker.Publish(ctx, "helixagent.dlq", &messaging.Message{
		ID:        "dlq-wrapper-e2e",
		Type:      "dlq",
		Payload:   payload,
		Timestamp: time.Now(),
	}))

	// Wait for processing
	time.Sleep(5 * time.Second)

	metrics := processor.GetMetrics()
	t.Logf("DLQ Metrics - Processed: %d, Retried: %d, Discarded: %d, Errors: %d",
		metrics.MessagesProcessed,
		metrics.MessagesRetried,
		metrics.MessagesDiscarded,
		metrics.ProcessingErrors)

	// Should have some retries
	assert.True(t, atomic.LoadInt64(&failedAttempts) >= 1 || atomic.LoadInt64(&successfulRetries) >= 1)
}

// TestE2E_ReplayWorkflow tests complete message replay workflow
func TestE2E_ReplayWorkflow(t *testing.T) {
	ctx, cancel := context.WithTimeout(testCtx, 60*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	require.NoError(t, broker.Connect(ctx))
	defer broker.Close(ctx)

	config := replay.DefaultReplayConfig()
	config.ReplayTimeout = 10 * time.Second
	config.MaxConcurrentReplays = 2

	handler := replay.NewHandler(broker, config, testLogger)

	// Test 1: Basic replay
	request := &replay.ReplayRequest{
		ID:       "e2e-replay-1",
		Topic:    "e2e.source.topic",
		FromTime: time.Now().Add(-24 * time.Hour),
		ToTime:   time.Now(),
		Filter: &replay.ReplayFilter{
			MessageTypes: []string{"e2e.test"},
		},
		Options: &replay.ReplayOptions{
			DryRun:         true,
			BatchSize:      50,
			SkipDuplicates: true,
		},
	}

	progress, err := handler.StartReplay(ctx, request)
	require.NoError(t, err)
	assert.NotNil(t, progress)
	assert.Equal(t, replay.ReplayStatusPending, progress.Status)

	// Wait for completion
	time.Sleep(3 * time.Second)

	// Check status
	finalProgress, err := handler.GetProgress("e2e-replay-1")
	require.NoError(t, err)
	assert.Equal(t, replay.ReplayStatusCompleted, finalProgress.Status)

	// Test 2: Concurrent replays
	for i := 0; i < 3; i++ {
		go func(idx int) {
			req := &replay.ReplayRequest{
				ID:       fmt.Sprintf("e2e-concurrent-%d", idx),
				Topic:    "e2e.concurrent.topic",
				FromTime: time.Now().Add(-1 * time.Hour),
				Options:  &replay.ReplayOptions{DryRun: true},
			}
			handler.StartReplay(ctx, req)
		}(i)
	}

	time.Sleep(2 * time.Second)

	// List all replays
	replays := handler.ListReplays()
	t.Logf("Active replays: %d", len(replays))
	assert.GreaterOrEqual(t, len(replays), 1)

	// Cleanup
	removed := handler.CleanupOldReplays(0)
	t.Logf("Cleaned up %d old replays", removed)
}

// TestE2E_HighThroughput tests system under high load
func TestE2E_HighThroughput(t *testing.T) {
	ctx, cancel := context.WithTimeout(testCtx, 120*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	require.NoError(t, broker.Connect(ctx))
	defer broker.Close(ctx)

	// Set up consumer
	var receivedCount int64
	_, err := broker.Subscribe(ctx, "e2e.throughput.topic", func(ctx context.Context, msg *messaging.Message) error {
		atomic.AddInt64(&receivedCount, 1)
		return nil
	})
	require.NoError(t, err)

	// High-throughput publishing
	messageCount := 10000
	startTime := time.Now()

	for i := 0; i < messageCount; i++ {
		msg := &messaging.Message{
			ID:        fmt.Sprintf("throughput-msg-%d", i),
			Type:      "throughput.test",
			Payload:   []byte(fmt.Sprintf(`{"index": %d}`, i)),
			Timestamp: time.Now(),
		}
		broker.Publish(ctx, "e2e.throughput.topic", msg)
	}

	publishDuration := time.Since(startTime)
	publishRate := float64(messageCount) / publishDuration.Seconds()

	// Wait for consumption
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&receivedCount) >= int64(messageCount) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	totalDuration := time.Since(startTime)
	throughput := float64(atomic.LoadInt64(&receivedCount)) / totalDuration.Seconds()

	t.Logf("Performance Results:")
	t.Logf("  Messages: %d", messageCount)
	t.Logf("  Publish Duration: %v", publishDuration)
	t.Logf("  Publish Rate: %.2f msgs/sec", publishRate)
	t.Logf("  Total Duration: %v", totalDuration)
	t.Logf("  End-to-End Throughput: %.2f msgs/sec", throughput)
	t.Logf("  Received: %d/%d (%.2f%%)",
		atomic.LoadInt64(&receivedCount),
		messageCount,
		float64(atomic.LoadInt64(&receivedCount))/float64(messageCount)*100)

	assert.GreaterOrEqual(t, atomic.LoadInt64(&receivedCount), int64(messageCount*90/100),
		"Should receive at least 90% of messages")
}

// TestE2E_ErrorRecovery tests system recovery from errors
func TestE2E_ErrorRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(testCtx, 60*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	require.NoError(t, broker.Connect(ctx))
	defer broker.Close(ctx)

	var successCount, errorCount int64

	// Consumer that fails sometimes
	_, err := broker.Subscribe(ctx, "e2e.recovery.topic", func(ctx context.Context, msg *messaging.Message) error {
		// Parse index from payload
		var data map[string]int
		if err := json.Unmarshal(msg.Payload, &data); err == nil {
			if data["index"]%5 == 0 {
				atomic.AddInt64(&errorCount, 1)
				return fmt.Errorf("simulated error for message %d", data["index"])
			}
		}
		atomic.AddInt64(&successCount, 1)
		return nil
	})
	require.NoError(t, err)

	// Publish messages
	messageCount := 100
	for i := 0; i < messageCount; i++ {
		msg := &messaging.Message{
			ID:        fmt.Sprintf("recovery-msg-%d", i),
			Type:      "recovery.test",
			Payload:   []byte(fmt.Sprintf(`{"index": %d}`, i)),
			Timestamp: time.Now(),
		}
		broker.Publish(ctx, "e2e.recovery.topic", msg)
	}

	// Wait for processing
	time.Sleep(3 * time.Second)

	t.Logf("Recovery Results: Success=%d, Errors=%d",
		atomic.LoadInt64(&successCount),
		atomic.LoadInt64(&errorCount))

	// System should continue processing despite errors
	assert.Greater(t, atomic.LoadInt64(&successCount), int64(0))
}

// TestE2E_GracefulDegradation tests behavior under resource constraints
func TestE2E_GracefulDegradation(t *testing.T) {
	ctx, cancel := context.WithTimeout(testCtx, 60*time.Second)
	defer cancel()

	broker := inmemory.NewBroker()
	require.NoError(t, broker.Connect(ctx))
	defer broker.Close(ctx)

	var processedCount int64

	// Slow consumer
	_, err := broker.Subscribe(ctx, "e2e.degradation.topic", func(ctx context.Context, msg *messaging.Message) error {
		time.Sleep(50 * time.Millisecond) // Simulate slow processing
		atomic.AddInt64(&processedCount, 1)
		return nil
	})
	require.NoError(t, err)

	// Fast producer - overwhelm the consumer
	messageCount := 1000
	for i := 0; i < messageCount; i++ {
		msg := &messaging.Message{
			ID:        fmt.Sprintf("degradation-msg-%d", i),
			Type:      "degradation.test",
			Payload:   []byte(`{"test": "degradation"}`),
			Timestamp: time.Now(),
		}
		broker.Publish(ctx, "e2e.degradation.topic", msg)
	}

	// Wait some time for processing
	time.Sleep(10 * time.Second)

	processed := atomic.LoadInt64(&processedCount)
	t.Logf("Graceful Degradation: Processed %d/%d messages (%.2f%%)",
		processed, messageCount, float64(processed)/float64(messageCount)*100)

	// System should not crash and should process some messages
	assert.Greater(t, processed, int64(0), "System should process some messages")
}

// TestE2E_APIEndpoints tests HTTP API endpoints
func TestE2E_APIEndpoints(t *testing.T) {
	// Skip if server not running
	baseURL := os.Getenv("HELIXAGENT_TEST_URL")
	if baseURL == "" {
		t.Skip("HELIXAGENT_TEST_URL not set, skipping API tests")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Test health endpoint
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Server not available: %v", err)
	}
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test messaging health
	resp, err = client.Get(baseURL + "/v1/messaging/health")
	if err == nil {
		defer resp.Body.Close()
		t.Logf("Messaging health status: %d", resp.StatusCode)
	}

	// Test DLQ list
	resp, err = client.Get(baseURL + "/v1/messaging/dlq")
	if err == nil {
		defer resp.Body.Close()
		t.Logf("DLQ list status: %d", resp.StatusCode)
	}

	// Test replay list
	resp, err = client.Get(baseURL + "/v1/messaging/replay")
	if err == nil {
		defer resp.Body.Close()
		t.Logf("Replay list status: %d", resp.StatusCode)
	}
}
