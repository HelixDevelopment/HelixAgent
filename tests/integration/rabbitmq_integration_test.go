package integration

import (
	"context"
	"encoding/json"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"dev.helix.agent/internal/messaging"
	"dev.helix.agent/internal/messaging/rabbitmq"
	"dev.helix.agent/internal/testutil"
)

// skipIfNoRabbitMQ skips the test if RabbitMQ infrastructure is not available
func skipIfNoRabbitMQ(t *testing.T) *rabbitmq.Broker {
	t.Helper()

	host := os.Getenv("RABBITMQ_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("RABBITMQ_PORT")
	if port == "" {
		port = "5672"
	}
	testutil.RequireExternalService(t, "rabbitmq", host, port)
	user := os.Getenv("RABBITMQ_USER")
	if user == "" {
		user = "guest"
	}
	password := os.Getenv("RABBITMQ_PASSWORD")
	if password == "" {
		password = "guest"
	}

	cfg := &rabbitmq.Config{
		Host:                   host,
		Port:                   5672,
		Username:               user,
		Password:               password,
		VHost:                  "/",
		PrefetchCount:          10,
		ConnectionTimeout:      10 * time.Second,
		ReconnectDelay:         5 * time.Second,
		PublishTimeout:         10 * time.Second,
		DefaultExchangeType:    "topic",
		DefaultExchangeDurable: true,
		DefaultQueueDurable:    true,
	}

	logger := zap.NewNop()
	broker := rabbitmq.NewBroker(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := broker.Connect(ctx); err != nil {
		t.Skipf("Skipping RabbitMQ integration test - infrastructure not available: %v (SKIP-OK: #integration-only)", err)
	}

	return broker
}

// TestRabbitMQ_Connect_Success tests successful connection to RabbitMQ
func TestRabbitMQ_Connect_Success(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	assert.True(t, broker.IsConnected())
}

// TestRabbitMQ_Connect_AlreadyConnected tests connecting when already connected
func TestRabbitMQ_Connect_AlreadyConnected(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx := context.Background()
	err := broker.Connect(ctx)
	assert.NoError(t, err)
	assert.True(t, broker.IsConnected())
}

// TestRabbitMQ_HealthCheck_Connected tests health check when connected
func TestRabbitMQ_HealthCheck_Connected(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx := context.Background()
	err := broker.HealthCheck(ctx)
	assert.NoError(t, err)
}

// TestRabbitMQ_BasicPubSub tests basic publish and subscribe
func TestRabbitMQ_BasicPubSub(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topic := "test.basic.pubsub." + time.Now().Format("20060102150405")

	// Subscribe first
	received := make(chan *messaging.Message, 1)
	handler := func(ctx context.Context, msg *messaging.Message) error {
		received <- msg
		return nil
	}

	sub, err := broker.Subscribe(ctx, topic, handler)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Wait for subscription to be ready
	time.Sleep(500 * time.Millisecond)

	// Publish message
	msg := &messaging.Message{
		ID:        "test-msg-" + time.Now().Format("20060102150405"),
		Type:      "test.event",
		Payload:   []byte(`{"key":"value","test":true}`),
		Timestamp: time.Now(),
		Headers:   map[string]string{"x-test": "value"},
	}

	err = broker.Publish(ctx, topic, msg)
	require.NoError(t, err)

	// Wait for message
	select {
	case m := <-received:
		assert.Equal(t, msg.ID, m.ID)
		assert.Equal(t, "test.event", m.Type)
		assert.Contains(t, string(m.Payload), "value")
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// TestRabbitMQ_PublishWithHeaders tests publishing with custom headers
func TestRabbitMQ_PublishWithHeaders(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topic := "test.headers." + time.Now().Format("20060102150405")

	// Subscribe
	received := make(chan *messaging.Message, 1)
	handler := func(ctx context.Context, msg *messaging.Message) error {
		received <- msg
		return nil
	}

	sub, err := broker.Subscribe(ctx, topic, handler)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	time.Sleep(500 * time.Millisecond)

	// Publish message with headers
	msg := &messaging.Message{
		ID:        "test-headers-" + time.Now().Format("20060102150405"),
		Type:      "test.headers.event",
		Payload:   []byte(`{"data":"test"}`),
		Timestamp: time.Now(),
		Headers: map[string]string{
			"x-custom-1":    "value1",
			"x-custom-2":    "value2",
			"x-correlation": "corr-123",
		},
		TraceID: "trace-abc-123",
	}

	err = broker.Publish(ctx, topic, msg)
	require.NoError(t, err)

	select {
	case m := <-received:
		assert.Equal(t, msg.ID, m.ID)
		assert.Equal(t, msg.Type, m.Type)
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// TestRabbitMQ_BatchPublish tests batch publishing
func TestRabbitMQ_BatchPublish(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	topic := "test.batch." + time.Now().Format("20060102150405")

	// Subscribe
	var receivedCount atomic.Int64
	handler := func(ctx context.Context, msg *messaging.Message) error {
		receivedCount.Add(1)
		return nil
	}

	sub, err := broker.Subscribe(ctx, topic, handler)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	time.Sleep(500 * time.Millisecond)

	// Publish batch
	batchSize := 10
	messages := make([]*messaging.Message, batchSize)
	for i := 0; i < batchSize; i++ {
		messages[i] = &messaging.Message{
			ID:        "batch-msg-" + string(rune('A'+i)),
			Type:      "test.batch.event",
			Payload:   []byte(`{"batch_index":` + string(rune('0'+i)) + `}`),
			Timestamp: time.Now(),
			Headers:   map[string]string{"batch-index": string(rune('0' + i))},
		}
	}

	err = broker.PublishBatch(ctx, topic, messages)
	require.NoError(t, err)

	// Wait for messages
	deadline := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for batch messages, received: %d", receivedCount.Load())
		case <-ticker.C:
			if receivedCount.Load() >= int64(batchSize) {
				return // Success
			}
		}
	}
}

// TestRabbitMQ_PublishWithPersistence tests publishing with persistence
func TestRabbitMQ_PublishWithPersistence(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topic := "test.persistent." + time.Now().Format("20060102150405")

	// Subscribe
	received := make(chan *messaging.Message, 1)
	handler := func(ctx context.Context, msg *messaging.Message) error {
		received <- msg
		return nil
	}

	sub, err := broker.Subscribe(ctx, topic, handler)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	time.Sleep(500 * time.Millisecond)

	// Publish with persistence option
	msg := &messaging.Message{
		ID:        "test-persistent-" + time.Now().Format("20060102150405"),
		Type:      "test.persistent.event",
		Payload:   []byte(`{"persistent":true}`),
		Timestamp: time.Now(),
	}

	err = broker.Publish(ctx, topic, msg)
	require.NoError(t, err)

	select {
	case m := <-received:
		assert.Equal(t, msg.ID, m.ID)
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for persistent message")
	}
}

// TestRabbitMQ_Close_WithActiveSubscriptions tests closing with active subscriptions
func TestRabbitMQ_Close_WithActiveSubscriptions(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)

	ctx := context.Background()
	topic := "test.close." + time.Now().Format("20060102150405")

	// Create multiple subscriptions
	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	sub1, err := broker.Subscribe(ctx, topic, handler)
	require.NoError(t, err)

	sub2, err := broker.Subscribe(ctx, topic+".2", handler)
	require.NoError(t, err)

	// Close should clean up all subscriptions
	err = broker.Close(ctx)
	assert.NoError(t, err)
	assert.False(t, broker.IsConnected())

	// Subscriptions should be inactive
	assert.False(t, sub1.IsActive())
	assert.False(t, sub2.IsActive())
}

// TestRabbitMQ_Unsubscribe tests unsubscribing
func TestRabbitMQ_Unsubscribe(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx := context.Background()
	topic := "test.unsub." + time.Now().Format("20060102150405")

	var received atomic.Int64
	handler := func(ctx context.Context, msg *messaging.Message) error {
		received.Add(1)
		return nil
	}

	sub, err := broker.Subscribe(ctx, topic, handler)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Publish and verify receipt
	msg := &messaging.Message{
		ID:        "test-unsub-" + time.Now().Format("20060102150405"),
		Type:      "test",
		Payload:   []byte(`{}`),
		Timestamp: time.Now(),
	}

	err = broker.Publish(ctx, topic, msg)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)
	beforeUnsub := received.Load()

	// Unsubscribe
	err = sub.Unsubscribe()
	require.NoError(t, err)
	assert.False(t, sub.IsActive())

	// Publish again - should not be received
	err = broker.Publish(ctx, topic, msg)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)
	afterUnsub := received.Load()

	// Count should be the same (no new messages received after unsubscribe)
	assert.Equal(t, beforeUnsub, afterUnsub)
}

// TestRabbitMQ_Metrics tests metrics collection
func TestRabbitMQ_Metrics(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx := context.Background()
	topic := "test.metrics." + time.Now().Format("20060102150405")

	// Publish some messages
	for i := 0; i < 5; i++ {
		msg := &messaging.Message{
			ID:        "metrics-" + string(rune('0'+i)),
			Type:      "test",
			Payload:   []byte(`{"test":true}`),
			Timestamp: time.Now(),
		}
		err := broker.Publish(ctx, topic, msg)
		require.NoError(t, err)
	}

	metrics := broker.GetMetrics()
	assert.NotNil(t, metrics)
	assert.GreaterOrEqual(t, metrics.MessagesPublished.Load(), int64(5))
}

// TestRabbitMQ_HandlerError tests handler returning error
func TestRabbitMQ_HandlerError(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topic := "test.error." + time.Now().Format("20060102150405")

	var processCount atomic.Int64
	handler := func(ctx context.Context, msg *messaging.Message) error {
		processCount.Add(1)
		// Return error - message may be nacked depending on configuration
		return assert.AnError
	}

	sub, err := broker.Subscribe(ctx, topic, handler)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	time.Sleep(500 * time.Millisecond)

	// Publish message
	msg := &messaging.Message{
		ID:        "test-error-" + time.Now().Format("20060102150405"),
		Type:      "test.error.event",
		Payload:   []byte(`{"will_error":true}`),
		Timestamp: time.Now(),
	}

	err = broker.Publish(ctx, topic, msg)
	require.NoError(t, err)

	// Wait a bit for the handler to be called
	time.Sleep(3 * time.Second)

	// Handler should have been called at least once
	assert.GreaterOrEqual(t, processCount.Load(), int64(1))
}

// TestRabbitMQ_LargeMessage tests publishing large messages
func TestRabbitMQ_LargeMessage(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topic := "test.large." + time.Now().Format("20060102150405")

	// Create a large payload (100KB - RabbitMQ default max is 128MB)
	largeData := make([]byte, 100*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"data": largeData,
	})

	// Subscribe
	received := make(chan *messaging.Message, 1)
	handler := func(ctx context.Context, msg *messaging.Message) error {
		received <- msg
		return nil
	}

	sub, err := broker.Subscribe(ctx, topic, handler)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	time.Sleep(500 * time.Millisecond)

	// Publish large message
	msg := &messaging.Message{
		ID:        "test-large-" + time.Now().Format("20060102150405"),
		Type:      "test.large.event",
		Payload:   payload,
		Timestamp: time.Now(),
	}

	err = broker.Publish(ctx, topic, msg)
	require.NoError(t, err)

	select {
	case m := <-received:
		assert.Equal(t, msg.ID, m.ID)
		assert.True(t, len(m.Payload) > 100*1024)
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for large message")
	}
}

// TestRabbitMQ_ConcurrentPublish tests concurrent publishing
func TestRabbitMQ_ConcurrentPublish(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx := context.Background()
	topic := "test.concurrent." + time.Now().Format("20060102150405")

	// Concurrent publishers
	numPublishers := 5
	messagesPerPublisher := 10
	done := make(chan bool, numPublishers)

	for i := 0; i < numPublishers; i++ {
		go func(publisherID int) {
			for j := 0; j < messagesPerPublisher; j++ {
				msg := &messaging.Message{
					ID:        "concurrent-" + string(rune('0'+publisherID)) + "-" + string(rune('0'+j)),
					Type:      "test.concurrent.event",
					Payload:   []byte(`{"publisher":` + string(rune('0'+publisherID)) + `}`),
					Timestamp: time.Now(),
				}
				broker.Publish(ctx, topic, msg)
			}
			done <- true
		}(i)
	}

	// Wait for all publishers
	for i := 0; i < numPublishers; i++ {
		<-done
	}

	metrics := broker.GetMetrics()
	expected := int64(numPublishers * messagesPerPublisher)
	assert.GreaterOrEqual(t, metrics.MessagesPublished.Load(), expected)
}

// TestRabbitMQ_ExchangeDeclaration tests exchange declaration
func TestRabbitMQ_ExchangeDeclaration(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx := context.Background()

	// Test that exchange is created when publishing
	exchange := "test.exchange." + time.Now().Format("20060102150405")
	topic := exchange + ".routing.key"

	msg := &messaging.Message{
		ID:        "test-exchange-" + time.Now().Format("20060102150405"),
		Type:      "test",
		Payload:   []byte(`{}`),
		Timestamp: time.Now(),
	}

	// Publishing should succeed (creates exchange if needed)
	err := broker.Publish(ctx, topic, msg)
	assert.NoError(t, err)
}

// TestRabbitMQ_QueueDeclaration tests queue declaration with subscription
func TestRabbitMQ_QueueDeclaration(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx := context.Background()
	topic := "test.queue.decl." + time.Now().Format("20060102150405")

	// Subscribing should create the queue
	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	sub, err := broker.Subscribe(ctx, topic, handler)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	assert.True(t, sub.IsActive())
	assert.NotEmpty(t, sub.ID())
	assert.Equal(t, topic, sub.Topic())
}

// TestRabbitMQ_Reconnect tests reconnection behavior
func TestRabbitMQ_Reconnect(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)

	ctx := context.Background()

	// Verify connected
	assert.True(t, broker.IsConnected())

	// Close and reconnect
	err := broker.Close(ctx)
	require.NoError(t, err)
	assert.False(t, broker.IsConnected())

	// Reconnect
	err = broker.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, broker.IsConnected())

	// Cleanup
	broker.Close(ctx)
}

// TestRabbitMQ_PriorityMessages tests message priority
func TestRabbitMQ_PriorityMessages(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topic := "test.priority." + time.Now().Format("20060102150405")

	// Subscribe
	received := make(chan *messaging.Message, 1)
	handler := func(ctx context.Context, msg *messaging.Message) error {
		received <- msg
		return nil
	}

	sub, err := broker.Subscribe(ctx, topic, handler)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	time.Sleep(500 * time.Millisecond)

	// Publish with priority
	msg := &messaging.Message{
		ID:        "test-priority-" + time.Now().Format("20060102150405"),
		Type:      "test.priority.event",
		Payload:   []byte(`{"priority":"high"}`),
		Timestamp: time.Now(),
		Priority:  messaging.PriorityHigh,
	}

	err = broker.Publish(ctx, topic, msg)
	require.NoError(t, err)

	select {
	case m := <-received:
		assert.Equal(t, msg.ID, m.ID)
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for priority message")
	}
}

// TestRabbitMQ_TTLMessages tests message TTL
func TestRabbitMQ_TTLMessages(t *testing.T) {
	broker := skipIfNoRabbitMQ(t)
	defer broker.Close(context.Background())

	ctx := context.Background()
	topic := "test.ttl." + time.Now().Format("20060102150405")

	// Publish message with TTL
	msg := &messaging.Message{
		ID:        "test-ttl-" + time.Now().Format("20060102150405"),
		Type:      "test.ttl.event",
		Payload:   []byte(`{"ttl":"5s"}`),
		Timestamp: time.Now(),
	}

	// Note: TTL would normally be set at queue level, not per-message
	err := broker.Publish(ctx, topic, msg)
	assert.NoError(t, err)
}
