package inmemory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.agent/internal/messaging"
)

// Topic tests

func TestNewTopic(t *testing.T) {
	topic := NewTopic("test-topic", 3, time.Hour)
	assert.NotNil(t, topic)
	assert.Equal(t, "test-topic", topic.Name())
	assert.Equal(t, 3, topic.PartitionCount())
}

func TestTopic_Publish(t *testing.T) {
	topic := NewTopic("test-topic", 3, time.Hour)

	msg := messaging.NewMessage("test.type", []byte("payload"))
	err := topic.Publish(msg)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, int(msg.Partition), 0)
	assert.Less(t, int(msg.Partition), 3)
}

func TestTopic_PublishEvent(t *testing.T) {
	topic := NewTopic("test-topic", 3, time.Hour)

	event := messaging.NewEvent("test.event", "test-source", []byte("event data"))
	err := topic.PublishEvent(event)
	require.NoError(t, err)
}

func TestTopic_Subscribe(t *testing.T) {
	topic := NewTopic("test-topic", 1, time.Hour)

	handler := func(ctx context.Context, event *messaging.Event) error {
		return nil
	}

	ch := topic.Subscribe(handler, "test-group")
	assert.NotNil(t, ch)
}

func TestTopic_Unsubscribe(t *testing.T) {
	topic := NewTopic("test-topic", 1, time.Hour)

	handler := func(ctx context.Context, event *messaging.Event) error {
		return nil
	}

	topic.Subscribe(handler, "test-group")
	topic.Unsubscribe("test-group")
	// Verify unsubscribe doesn't panic
}

func TestTopic_Read(t *testing.T) {
	topic := NewTopic("test-topic", 3, time.Hour)

	// Publish some messages
	for i := 0; i < 5; i++ {
		msg := messaging.NewMessage("test.type", []byte("payload"))
		msg.Partition = 0
		_ = topic.Publish(msg)
	}

	// Read from partition 0
	messages, err := topic.Read(0, 0, 10)
	require.NoError(t, err)
	assert.Len(t, messages, 5)
}

func TestTopic_ReadInvalidPartition(t *testing.T) {
	topic := NewTopic("test-topic", 3, time.Hour)

	_, err := topic.Read(-1, 0, 10)
	assert.Error(t, err)

	_, err = topic.Read(100, 0, 10)
	assert.Error(t, err)
}

func TestTopic_GetHighWatermark(t *testing.T) {
	topic := NewTopic("test-topic", 3, time.Hour)

	// Initial watermark
	assert.Equal(t, int64(0), topic.GetHighWatermark(0))

	// Publish messages
	msg := messaging.NewMessage("test.type", []byte("payload"))
	msg.Partition = 0
	_ = topic.Publish(msg)

	assert.Equal(t, int64(1), topic.GetHighWatermark(0))

	// Invalid partition
	assert.Equal(t, int64(0), topic.GetHighWatermark(-1))
	assert.Equal(t, int64(0), topic.GetHighWatermark(100))
}

func TestTopic_GetLowWatermark(t *testing.T) {
	topic := NewTopic("test-topic", 3, time.Hour)

	assert.Equal(t, int64(0), topic.GetLowWatermark(0))

	// Invalid partition
	assert.Equal(t, int64(0), topic.GetLowWatermark(-1))
	assert.Equal(t, int64(0), topic.GetLowWatermark(100))
}

// Partition tests

func TestNewPartition(t *testing.T) {
	p := NewPartition(0, 1000)
	assert.NotNil(t, p)
	assert.Equal(t, 0, p.ID())
	assert.Equal(t, int64(0), p.HighWatermark())
	assert.Equal(t, int64(0), p.LowWatermark())
	assert.Equal(t, 0, p.Len())
}

func TestPartition_Append(t *testing.T) {
	p := NewPartition(0, 1000)

	msg := messaging.NewMessage("test.type", []byte("payload"))
	offset, err := p.Append(msg)
	require.NoError(t, err)
	assert.Equal(t, int64(0), offset)
	assert.Equal(t, int64(1), p.HighWatermark())
	assert.Equal(t, 1, p.Len())
}

func TestPartition_AppendCompact(t *testing.T) {
	p := NewPartition(0, 4) // Small capacity

	// Fill the partition
	for i := 0; i < 4; i++ {
		msg := messaging.NewMessage("test.type", []byte("payload"))
		_, _ = p.Append(msg)
	}

	// This should trigger compaction
	msg := messaging.NewMessage("test.type", []byte("payload"))
	offset, err := p.Append(msg)
	require.NoError(t, err)
	assert.Equal(t, int64(4), offset)
	assert.Greater(t, p.LowWatermark(), int64(0)) // Compaction moved low watermark
}

func TestPartition_Read(t *testing.T) {
	p := NewPartition(0, 1000)

	// Append messages
	for i := 0; i < 5; i++ {
		msg := messaging.NewMessage("test.type", []byte("payload"))
		_, _ = p.Append(msg)
	}

	// Read from offset 0
	messages, err := p.Read(0, 3)
	require.NoError(t, err)
	assert.Len(t, messages, 3)

	// Read from offset 3
	messages, err = p.Read(3, 10)
	require.NoError(t, err)
	assert.Len(t, messages, 2)

	// Read from beyond high watermark
	messages, err = p.Read(10, 5)
	require.NoError(t, err)
	assert.Empty(t, messages)
}

// StreamBroker tests

func TestDefaultStreamConfig(t *testing.T) {
	config := DefaultStreamConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 3, config.DefaultPartitions)
	assert.Equal(t, 7*24*time.Hour, config.DefaultRetention)
}

func TestNewStreamBroker(t *testing.T) {
	broker := NewStreamBroker(nil)
	assert.NotNil(t, broker)
	assert.NotNil(t, broker.config)
}

func TestStreamBroker_ConnectClose(t *testing.T) {
	broker := NewStreamBroker(nil)

	err := broker.Connect(context.Background())
	require.NoError(t, err)
	assert.True(t, broker.IsConnected())

	err = broker.Close(context.Background())
	require.NoError(t, err)
	assert.False(t, broker.IsConnected())
}

func TestStreamBroker_HealthCheck(t *testing.T) {
	broker := NewStreamBroker(nil)

	// Not connected
	err := broker.HealthCheck(context.Background())
	assert.Error(t, err)

	// Connected
	_ = broker.Connect(context.Background())
	err = broker.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestStreamBroker_PublishSubscribe(t *testing.T) {
	broker := NewStreamBroker(nil)
	_ = broker.Connect(context.Background())
	defer func() { _ = broker.Close(context.Background()) }()

	// Create topic
	err := broker.CreateTopic(context.Background(), "test-topic", 3, 1)
	require.NoError(t, err)

	// Publish
	msg := messaging.NewMessage("test.type", []byte("payload"))
	err = broker.Publish(context.Background(), "test-topic", msg)
	require.NoError(t, err)
}

func TestStreamBroker_PublishBatch(t *testing.T) {
	broker := NewStreamBroker(nil)
	_ = broker.Connect(context.Background())
	defer func() { _ = broker.Close(context.Background()) }()

	_ = broker.CreateTopic(context.Background(), "test-topic", 3, 1)

	messages := []*messaging.Message{
		messaging.NewMessage("test.type", []byte("1")),
		messaging.NewMessage("test.type", []byte("2")),
	}

	err := broker.PublishBatch(context.Background(), "test-topic", messages)
	require.NoError(t, err)
}

func TestStreamBroker_Subscribe(t *testing.T) {
	broker := NewStreamBroker(nil)
	_ = broker.Connect(context.Background())
	defer func() { _ = broker.Close(context.Background()) }()

	_ = broker.CreateTopic(context.Background(), "test-topic", 3, 1)

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	_, err := broker.Subscribe(context.Background(), "test-topic", handler)
	require.NoError(t, err)
}

func TestStreamBroker_CreateDeleteTopic(t *testing.T) {
	broker := NewStreamBroker(nil)
	_ = broker.Connect(context.Background())
	defer func() { _ = broker.Close(context.Background()) }()

	err := broker.CreateTopic(context.Background(), "new-topic", 3, 1)
	require.NoError(t, err)

	topics, err := broker.ListTopics(context.Background())
	require.NoError(t, err)
	assert.Contains(t, topics, "new-topic")

	err = broker.DeleteTopic(context.Background(), "new-topic")
	require.NoError(t, err)

	topics, err = broker.ListTopics(context.Background())
	require.NoError(t, err)
	assert.NotContains(t, topics, "new-topic")
}

func TestStreamBroker_GetTopicMetadata(t *testing.T) {
	broker := NewStreamBroker(nil)
	_ = broker.Connect(context.Background())
	defer func() { _ = broker.Close(context.Background()) }()

	_ = broker.CreateTopic(context.Background(), "test-topic", 3, 1)

	metadata, err := broker.GetTopicMetadata(context.Background(), "test-topic")
	require.NoError(t, err)
	assert.NotNil(t, metadata)
}

func TestStreamBroker_BrokerType(t *testing.T) {
	broker := NewStreamBroker(nil)
	assert.Equal(t, messaging.BrokerType("inmemory"), broker.BrokerType())
}

func TestStreamBroker_GetMetrics(t *testing.T) {
	broker := NewStreamBroker(nil)
	metrics := broker.GetMetrics()
	assert.NotNil(t, metrics)
}

func TestStreamBroker_PublishEvent(t *testing.T) {
	broker := NewStreamBroker(nil)
	_ = broker.Connect(context.Background())
	defer func() { _ = broker.Close(context.Background()) }()

	_ = broker.CreateTopic(context.Background(), "test-topic", 3, 1)

	event := messaging.NewEvent("test.event", "source", []byte("data"))
	err := broker.PublishEvent(context.Background(), "test-topic", event)
	require.NoError(t, err)
}

func TestStreamBroker_PublishEventBatch(t *testing.T) {
	broker := NewStreamBroker(nil)
	_ = broker.Connect(context.Background())
	defer func() { _ = broker.Close(context.Background()) }()

	_ = broker.CreateTopic(context.Background(), "test-topic", 3, 1)

	events := []*messaging.Event{
		messaging.NewEvent("test.event", "source", []byte("1")),
		messaging.NewEvent("test.event", "source", []byte("2")),
	}

	err := broker.PublishEventBatch(context.Background(), "test-topic", events)
	require.NoError(t, err)
}

func TestStreamBroker_SubscribeEvents(t *testing.T) {
	broker := NewStreamBroker(nil)
	_ = broker.Connect(context.Background())
	defer func() { _ = broker.Close(context.Background()) }()

	_ = broker.CreateTopic(context.Background(), "test-topic", 3, 1)

	handler := func(ctx context.Context, event *messaging.Event) error {
		return nil
	}

	_, err := broker.SubscribeEvents(context.Background(), "test-topic", handler)
	require.NoError(t, err)
}

func TestStreamBroker_CommitGetOffset(t *testing.T) {
	broker := NewStreamBroker(nil)
	_ = broker.Connect(context.Background())
	defer func() { _ = broker.Close(context.Background()) }()

	_ = broker.CreateTopic(context.Background(), "test-topic", 3, 1)

	// Commit offset
	err := broker.CommitOffset(context.Background(), "test-topic", 0, 100)
	require.NoError(t, err)

	// Get offset
	offset, err := broker.GetOffset(context.Background(), "test-topic", 0)
	require.NoError(t, err)
	assert.Equal(t, int64(100), offset)
}

func TestStreamBroker_SeekOperations(t *testing.T) {
	broker := NewStreamBroker(nil)
	_ = broker.Connect(context.Background())
	defer func() { _ = broker.Close(context.Background()) }()

	_ = broker.CreateTopic(context.Background(), "test-topic", 3, 1)

	// Publish some messages
	for i := 0; i < 5; i++ {
		msg := messaging.NewMessage("test.type", []byte("payload"))
		_ = broker.Publish(context.Background(), "test-topic", msg)
	}

	err := broker.SeekToOffset(context.Background(), "test-topic", 0, 2)
	require.NoError(t, err)

	err = broker.SeekToTimestamp(context.Background(), "test-topic", 0, time.Now())
	require.NoError(t, err)

	err = broker.SeekToBeginning(context.Background(), "test-topic", 0)
	require.NoError(t, err)

	err = broker.SeekToEnd(context.Background(), "test-topic", 0)
	require.NoError(t, err)
}

func TestStreamBroker_ConsumerGroup(t *testing.T) {
	broker := NewStreamBroker(nil)
	_ = broker.Connect(context.Background())
	defer func() { _ = broker.Close(context.Background()) }()

	_ = broker.CreateTopic(context.Background(), "test-topic", 3, 1)

	err := broker.CreateConsumerGroup(context.Background(), "test-group")
	require.NoError(t, err)

	err = broker.DeleteConsumerGroup(context.Background(), "test-group")
	require.NoError(t, err)
}

func TestStreamBroker_SubscribeUnsubscribe(t *testing.T) {
	broker := NewStreamBroker(nil)
	_ = broker.Connect(context.Background())
	defer func() { _ = broker.Close(context.Background()) }()

	_ = broker.CreateTopic(context.Background(), "test-topic", 3, 1)

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	sub, err := broker.Subscribe(context.Background(), "test-topic", handler)
	require.NoError(t, err)
	require.NotNil(t, sub)

	// Unsubscribe via subscription
	err = sub.Unsubscribe()
	require.NoError(t, err)
}

// StreamSubscription tests

func TestStreamSubscription(t *testing.T) {
	broker := NewStreamBroker(nil)
	_ = broker.Connect(context.Background())
	defer func() { _ = broker.Close(context.Background()) }()

	_ = broker.CreateTopic(context.Background(), "test-topic", 3, 1)

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return nil
	}

	_, _ = broker.Subscribe(context.Background(), "test-topic", handler)

	// Get all subscriptions (internal check)
	if topic, exists := broker.topics.Get("test-topic"); exists {
		assert.Greater(t, topic.subscribers.Len(), 0)
	}
}

// Test selectPartition
func TestTopic_SelectPartition(t *testing.T) {
	topic := NewTopic("test-topic", 3, time.Hour)

	// With explicit partition
	msg := messaging.NewMessage("test.type", []byte("payload"))
	msg.Partition = 1
	_ = topic.Publish(msg)
	assert.Equal(t, int32(1), msg.Partition)

	// With negative partition (should hash)
	msg2 := messaging.NewMessage("test.type", []byte("payload"))
	msg2.Partition = -1
	_ = topic.Publish(msg2)
	assert.GreaterOrEqual(t, int(msg2.Partition), 0)
	assert.Less(t, int(msg2.Partition), 3)
}
