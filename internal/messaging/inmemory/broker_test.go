package inmemory

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/messaging"
)

func TestNewBroker(t *testing.T) {
	broker := NewBroker(nil)
	assert.NotNil(t, broker)
	assert.NotNil(t, broker.config)
	assert.Equal(t, 10000, broker.config.DefaultQueueCapacity)

	// With custom config
	config := &Config{
		DefaultQueueCapacity: 5000,
		MessageTTL:           1 * time.Hour,
	}
	broker2 := NewBroker(config)
	assert.Equal(t, 5000, broker2.config.DefaultQueueCapacity)
}

func TestBroker_Connect(t *testing.T) {
	broker := NewBroker(nil)

	err := broker.Connect(context.Background())
	require.NoError(t, err)
	assert.True(t, broker.IsConnected())

	metrics := broker.GetMetrics()
	assert.Equal(t, int64(1), metrics.ConnectionAttempts.Load())
	assert.Equal(t, int64(1), metrics.ConnectionSuccesses.Load())
}

func TestBroker_Close(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	err := broker.Close(context.Background())
	require.NoError(t, err)
	assert.False(t, broker.IsConnected())

	// Close when not connected
	err = broker.Close(context.Background())
	require.NoError(t, err)
}

func TestBroker_HealthCheck(t *testing.T) {
	broker := NewBroker(nil)

	// Not connected
	err := broker.HealthCheck(context.Background())
	assert.Error(t, err)

	// Connected
	_ = broker.Connect(context.Background())
	err = broker.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestBroker_Publish(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	msg := messaging.NewMessage("test.type", []byte("payload"))
	err := broker.Publish(context.Background(), "test.queue", msg)
	require.NoError(t, err)

	metrics := broker.GetMetrics()
	assert.Equal(t, int64(1), metrics.MessagesPublished.Load())
	assert.Equal(t, int64(1), metrics.PublishSuccesses.Load())
}

func TestBroker_Publish_NotConnected(t *testing.T) {
	broker := NewBroker(nil)

	msg := messaging.NewMessage("test.type", []byte("payload"))
	err := broker.Publish(context.Background(), "test.queue", msg)
	assert.Error(t, err)
}

func TestBroker_PublishBatch(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	messages := []*messaging.Message{
		messaging.NewMessage("test.type", []byte("payload1")),
		messaging.NewMessage("test.type", []byte("payload2")),
		messaging.NewMessage("test.type", []byte("payload3")),
	}

	err := broker.PublishBatch(context.Background(), "test.queue", messages)
	require.NoError(t, err)

	metrics := broker.GetMetrics()
	assert.Equal(t, int64(3), metrics.MessagesPublished.Load())
}

func TestBroker_Subscribe(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	received := make(chan *messaging.Message, 10)
	handler := func(ctx context.Context, msg *messaging.Message) error {
		received <- msg
		return nil
	}

	sub, err := broker.Subscribe(context.Background(), "test.queue", handler)
	require.NoError(t, err)
	assert.NotNil(t, sub)
	assert.True(t, sub.IsActive())
	assert.Equal(t, "test.queue", sub.Topic())

	// Publish a message
	msg := messaging.NewMessage("test.type", []byte("test payload"))
	err = broker.Publish(context.Background(), "test.queue", msg)
	require.NoError(t, err)

	// Wait for message
	select {
	case <-received:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}

	// Unsubscribe
	err = sub.Unsubscribe()
	require.NoError(t, err)
	assert.False(t, sub.IsActive())
}

func TestBroker_Subscribe_NotConnected(t *testing.T) {
	broker := NewBroker(nil)

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	sub, err := broker.Subscribe(context.Background(), "test.queue", handler)
	assert.Error(t, err)
	assert.Nil(t, sub)
}

func TestBroker_BrokerType(t *testing.T) {
	broker := NewBroker(nil)
	assert.Equal(t, messaging.BrokerTypeInMemory, broker.BrokerType())
}

func TestBroker_DeclareQueue(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	err := broker.DeclareQueue(context.Background(), "test.queue")
	require.NoError(t, err)

	metrics := broker.GetMetrics()
	assert.Equal(t, int64(1), metrics.QueuesDeclared.Load())

	// Declare same queue again (no-op)
	err = broker.DeclareQueue(context.Background(), "test.queue")
	require.NoError(t, err)
	assert.Equal(t, int64(1), metrics.QueuesDeclared.Load())
}

func TestBroker_EnqueueTask(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	task := messaging.NewTask("test.task", []byte("payload"))
	err := broker.EnqueueTask(context.Background(), "test.queue", task)
	require.NoError(t, err)
}

func TestBroker_EnqueueTaskBatch(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	tasks := []*messaging.Task{
		messaging.NewTask("test.task", []byte("payload1")),
		messaging.NewTask("test.task", []byte("payload2")),
	}

	err := broker.EnqueueTaskBatch(context.Background(), "test.queue", tasks)
	require.NoError(t, err)
}

func TestBroker_DequeueTask(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	// Declare queue first
	_ = broker.DeclareQueue(context.Background(), "test.queue")

	// Enqueue a task
	task := messaging.NewTask("test.task", []byte("payload"))
	_ = broker.EnqueueTask(context.Background(), "test.queue", task)

	// Dequeue
	dequeued, err := broker.DequeueTask(context.Background(), "test.queue", "worker-1")
	require.NoError(t, err)
	require.NotNil(t, dequeued)
	assert.Equal(t, task.Type, dequeued.Type)
}

func TestBroker_DequeueTask_QueueNotFound(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	_, err := broker.DequeueTask(context.Background(), "nonexistent", "worker-1")
	assert.Error(t, err)
}

func TestBroker_AckNackReject(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	// These are no-ops for in-memory broker
	err := broker.AckTask(context.Background(), 123)
	assert.NoError(t, err)

	err = broker.NackTask(context.Background(), 123, true)
	assert.NoError(t, err)

	err = broker.RejectTask(context.Background(), 123)
	assert.NoError(t, err)
}

func TestBroker_MoveToDeadLetter(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	task := messaging.NewTask("test.task", []byte("payload"))
	err := broker.MoveToDeadLetter(context.Background(), task, "max retries exceeded")
	require.NoError(t, err)

	assert.Equal(t, messaging.TaskStateDeadLettered, task.State)
	assert.Equal(t, "max retries exceeded", task.Error)
}

func TestBroker_GetQueueStats(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	// Declare and populate queue
	_ = broker.DeclareQueue(context.Background(), "test.queue")
	for i := 0; i < 5; i++ {
		task := messaging.NewTask("test.task", []byte("payload"))
		_ = broker.EnqueueTask(context.Background(), "test.queue", task)
	}

	stats, err := broker.GetQueueStats(context.Background(), "test.queue")
	require.NoError(t, err)
	assert.Equal(t, "test.queue", stats.Name)
	assert.Equal(t, int64(5), stats.Messages)
}

func TestBroker_GetQueueStats_NotFound(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	_, err := broker.GetQueueStats(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestBroker_GetQueueDepth(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	_ = broker.DeclareQueue(context.Background(), "test.queue")
	for i := 0; i < 3; i++ {
		task := messaging.NewTask("test.task", []byte("payload"))
		_ = broker.EnqueueTask(context.Background(), "test.queue", task)
	}

	depth, err := broker.GetQueueDepth(context.Background(), "test.queue")
	require.NoError(t, err)
	assert.Equal(t, int64(3), depth)
}

func TestBroker_PurgeQueue(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	_ = broker.DeclareQueue(context.Background(), "test.queue")
	for i := 0; i < 5; i++ {
		task := messaging.NewTask("test.task", []byte("payload"))
		_ = broker.EnqueueTask(context.Background(), "test.queue", task)
	}

	err := broker.PurgeQueue(context.Background(), "test.queue")
	require.NoError(t, err)

	depth, _ := broker.GetQueueDepth(context.Background(), "test.queue")
	assert.Equal(t, int64(0), depth)
}

func TestBroker_DeleteQueue(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	_ = broker.DeclareQueue(context.Background(), "test.queue")

	err := broker.DeleteQueue(context.Background(), "test.queue")
	require.NoError(t, err)

	_, err = broker.GetQueueStats(context.Background(), "test.queue")
	assert.Error(t, err)
}

func TestBroker_SubscribeTasks(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	received := make(chan *messaging.Task, 10)
	handler := func(ctx context.Context, task *messaging.Task) error {
		received <- task
		return nil
	}

	sub, err := broker.SubscribeTasks(context.Background(), "test.queue", handler)
	require.NoError(t, err)
	assert.NotNil(t, sub)

	// Enqueue a task
	task := messaging.NewTask("test.task", []byte("payload"))
	_ = broker.EnqueueTask(context.Background(), "test.queue", task)

	// Wait for task
	select {
	case rcvd := <-received:
		assert.Equal(t, task.Type, rcvd.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for task")
	}

	_ = sub.Unsubscribe()
}

func TestBroker_ConcurrentPublish(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	var wg sync.WaitGroup
	numGoroutines := 10
	messagesPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := messaging.NewMessage("test.type", []byte("payload"))
				_ = broker.Publish(context.Background(), "test.queue", msg)
			}
		}(i)
	}

	wg.Wait()

	metrics := broker.GetMetrics()
	expected := int64(numGoroutines * messagesPerGoroutine)
	assert.Equal(t, expected, metrics.MessagesPublished.Load())
}

func TestBroker_WithProcessingDelay(t *testing.T) {
	config := &Config{
		DefaultQueueCapacity: 1000,
		ProcessingDelay:      10 * time.Millisecond,
	}
	broker := NewBroker(config)
	_ = broker.Connect(context.Background())

	start := time.Now()
	msg := messaging.NewMessage("test.type", []byte("payload"))
	_ = broker.Publish(context.Background(), "test.queue", msg)
	duration := time.Since(start)

	assert.True(t, duration >= 10*time.Millisecond)
}

func TestSubscription_DoubleUnsubscribe(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	sub, _ := broker.Subscribe(context.Background(), "test.queue", handler)

	// First unsubscribe
	err := sub.Unsubscribe()
	assert.NoError(t, err)

	// Second unsubscribe (should be no-op)
	err = sub.Unsubscribe()
	assert.NoError(t, err)
}

func TestBroker_PriorityQueue(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	_ = broker.DeclareQueue(context.Background(), "test.queue")

	// Enqueue tasks with different priorities
	lowPriority := messaging.NewTask("low", []byte("low")).WithPriority(messaging.TaskPriorityLow)
	normalPriority := messaging.NewTask("normal", []byte("normal")).WithPriority(messaging.TaskPriorityNormal)
	highPriority := messaging.NewTask("high", []byte("high")).WithPriority(messaging.TaskPriorityHigh)

	// Enqueue in reverse priority order
	_ = broker.EnqueueTask(context.Background(), "test.queue", lowPriority)
	_ = broker.EnqueueTask(context.Background(), "test.queue", normalPriority)
	_ = broker.EnqueueTask(context.Background(), "test.queue", highPriority)

	// Dequeue should return highest priority first
	task1, _ := broker.DequeueTask(context.Background(), "test.queue", "worker")
	assert.Equal(t, "high", task1.Type)

	task2, _ := broker.DequeueTask(context.Background(), "test.queue", "worker")
	assert.Equal(t, "normal", task2.Type)

	task3, _ := broker.DequeueTask(context.Background(), "test.queue", "worker")
	assert.Equal(t, "low", task3.Type)
}

func TestBroker_RetryOnError(t *testing.T) {
	broker := NewBroker(nil)
	_ = broker.Connect(context.Background())

	var callCount atomic.Int32

	handler := func(ctx context.Context, msg *messaging.Message) error {
		callCount.Add(1)
		if callCount.Load() < 3 {
			return messaging.ErrHandlerError
		}
		return nil
	}

	_, _ = broker.Subscribe(context.Background(), "test.queue", handler,
		messaging.WithRetryOnError(true),
		messaging.WithMaxSubscribeRetries(3),
	)

	msg := messaging.NewMessage("test.type", []byte("payload"))
	msg.MaxRetries = 5
	_ = broker.Publish(context.Background(), "test.queue", msg)

	// Wait for retries
	time.Sleep(500 * time.Millisecond)

	// Handler should have been called at least once
	assert.True(t, callCount.Load() >= 1)
}

// TestBroker_RaceFree is the CONST-029 verification test. It drives
// parallel Publish + Subscribe + Unsubscribe across overlapping topics
// to shake out any regression where the migrated safe.Stores fail to
// serialise the check-create flow that used to live under mu.Lock.
//
// The old Broker held a single mu across all four maps, so any
// concurrent Publish to the same new topic would see at most one
// queue creation; the migrated version uses regMu as a Pattern Zeta
// serialiser across queues/topics/subscribers/notifyCh to preserve
// that same invariant without adjacent bare collections.
func TestBroker_RaceFree(t *testing.T) {
	broker := NewBroker(&Config{
		DefaultQueueCapacity: 10000,
		DefaultTopicCapacity: 10000,
		MessageTTL:           time.Minute,
	})
	require.NoError(t, broker.Connect(context.Background()))
	defer func() { _ = broker.Close(context.Background()) }()

	const publishers = 8
	const subscribers = 8
	const iterations = 100

	var wg sync.WaitGroup
	received := atomic.Int32{}

	// A pool of subscribers that come and go while publishers hammer topics.
	for s := 0; s < subscribers; s++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			topic := fmt.Sprintf("topic-%d", id%4) // 4 overlapping topics
			for i := 0; i < iterations; i++ {
				sub, err := broker.Subscribe(context.Background(), topic,
					func(ctx context.Context, msg *messaging.Message) error {
						received.Add(1)
						return nil
					})
				if err != nil {
					continue
				}
				_ = sub.Unsubscribe()
			}
		}(s)
	}

	// Publishers hammering all 4 overlapping topics.
	for p := 0; p < publishers; p++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				topic := fmt.Sprintf("topic-%d", (id+i)%4)
				msg := messaging.NewMessage("x", []byte("y"))
				_ = broker.Publish(context.Background(), topic, msg)
			}
		}(p)
	}

	wg.Wait()

	// Sanity check: broker is still healthy after the storm.
	require.NoError(t, broker.HealthCheck(context.Background()))
	assert.True(t, broker.IsConnected())
}
