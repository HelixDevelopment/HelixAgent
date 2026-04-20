package memory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock implementations ---

// mockBroker implements messaging.MessageBroker for testing.
//
// Concurrency model (CONST-029): published → *safe.Slice.
type mockBroker struct {
	connected      bool
	published      *safe.Slice[*publishedMsg]
	publishErr     error
	subscriptions  []messaging.Subscription
	healthCheckErr error
}

type publishedMsg struct {
	topic   string
	message *messaging.Message
}

func newMockBroker() *mockBroker {
	return &mockBroker{
		connected: true,
		published: safe.NewSlice[*publishedMsg](),
	}
}

func (b *mockBroker) Connect(_ context.Context) error      { return nil }
func (b *mockBroker) Close(_ context.Context) error        { return nil }
func (b *mockBroker) IsConnected() bool                    { return b.connected }
func (b *mockBroker) BrokerType() messaging.BrokerType     { return messaging.BrokerTypeInMemory }
func (b *mockBroker) GetMetrics() *messaging.BrokerMetrics { return &messaging.BrokerMetrics{} }

func (b *mockBroker) HealthCheck(_ context.Context) error {
	return b.healthCheckErr
}

func (b *mockBroker) Publish(_ context.Context, topic string, message *messaging.Message, _ ...messaging.PublishOption) error {
	if b.publishErr != nil {
		return b.publishErr
	}
	b.published.Append(&publishedMsg{topic: topic, message: message})
	return nil
}

func (b *mockBroker) PublishBatch(_ context.Context, topic string, messages []*messaging.Message, _ ...messaging.PublishOption) error {
	for _, msg := range messages {
		if err := b.Publish(context.Background(), topic, msg); err != nil {
			return err
		}
	}
	return nil
}

func (b *mockBroker) Subscribe(_ context.Context, _ string, _ messaging.MessageHandler, _ ...messaging.SubscribeOption) (messaging.Subscription, error) {
	return nil, nil
}

func (b *mockBroker) getPublished() []*publishedMsg {
	return b.published.Snapshot()
}

// mockEventLog implements EventLog for testing.
//
// Concurrency model (CONST-029): events → *safe.Slice.
type mockEventLog struct {
	events *safe.Slice[*MemoryEvent]
	err    error
}

func newMockEventLog() *mockEventLog {
	return &mockEventLog{events: safe.NewSlice[*MemoryEvent]()}
}

func (l *mockEventLog) Append(event *MemoryEvent) error {
	if l.err != nil {
		return l.err
	}
	l.events.Append(event)
	return nil
}

func (l *mockEventLog) GetEvents(memoryID string) ([]*MemoryEvent, error) {
	var result []*MemoryEvent
	for _, e := range l.events.Snapshot() {
		if e.MemoryID == memoryID {
			result = append(result, e)
		}
	}
	return result, l.err
}

func (l *mockEventLog) GetEventsSince(timestamp time.Time) ([]*MemoryEvent, error) {
	var result []*MemoryEvent
	for _, e := range l.events.Snapshot() {
		if e.Timestamp.After(timestamp) || e.Timestamp.Equal(timestamp) {
			result = append(result, e)
		}
	}
	return result, l.err
}

func (l *mockEventLog) GetEventsForUser(userID string) ([]*MemoryEvent, error) {
	var result []*MemoryEvent
	for _, e := range l.events.Snapshot() {
		if e.UserID == userID {
			result = append(result, e)
		}
	}
	return result, l.err
}

func (l *mockEventLog) GetEventsFromNode(nodeID string) ([]*MemoryEvent, error) {
	var result []*MemoryEvent
	for _, e := range l.events.Snapshot() {
		if e.NodeID == nodeID {
			result = append(result, e)
		}
	}
	return result, l.err
}

// --- Helper to create a test DistributedMemoryManager ---

func newTestDMM(t *testing.T) (*DistributedMemoryManager, *mockBroker, *mockEventLog) {
	t.Helper()
	store := NewInMemoryStore()
	localManager := NewManager(store, nil, nil, nil, nil, nil)
	broker := newMockBroker()
	eventLog := newMockEventLog()
	resolver := NewCRDTResolver(ConflictStrategyLastWriteWins)

	dmm := NewDistributedMemoryManager(localManager, "test-node", eventLog, resolver, broker, nil)
	return dmm, broker, eventLog
}

// --- NewDistributedMemoryManager ---

func TestNewDistributedMemoryManager(t *testing.T) {
	t.Run("WithAllParams", func(t *testing.T) {
		store := NewInMemoryStore()
		manager := NewManager(store, nil, nil, nil, nil, nil)
		broker := newMockBroker()
		eventLog := newMockEventLog()
		resolver := NewCRDTResolver(ConflictStrategyLastWriteWins)

		dmm := NewDistributedMemoryManager(manager, "node1", eventLog, resolver, broker, nil)
		require.NotNil(t, dmm)
		assert.Equal(t, "node1", dmm.nodeID)
		assert.NotNil(t, dmm.vectorClock)
		assert.NotNil(t, dmm.logger)
	})

	t.Run("EmptyNodeID_GeneratesUUID", func(t *testing.T) {
		store := NewInMemoryStore()
		manager := NewManager(store, nil, nil, nil, nil, nil)

		dmm := NewDistributedMemoryManager(manager, "", nil, nil, nil, nil)
		assert.NotEmpty(t, dmm.nodeID)
	})

	t.Run("NilLogger_CreatesDefault", func(t *testing.T) {
		store := NewInMemoryStore()
		manager := NewManager(store, nil, nil, nil, nil, nil)

		dmm := NewDistributedMemoryManager(manager, "node1", nil, nil, nil, nil)
		assert.NotNil(t, dmm.logger)
	})
}

// --- AddMemory ---

func TestDistributedMemoryManager_AddMemory(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		dmm, broker, eventLog := newTestDMM(t)
		ctx := context.Background()

		memory := &Memory{
			ID:      "mem1",
			UserID:  "user1",
			Content: "test content",
			Metadata: map[string]interface{}{
				"tags":     []string{"tag1"},
				"entities": []MemoryEntity{{ID: "e1", Name: "E1"}},
			},
		}

		err := dmm.AddMemory(ctx, memory)
		require.NoError(t, err)

		// Verify event was published to broker
		published := broker.getPublished()
		assert.Len(t, published, 1)
		assert.Equal(t, TopicMemoryEvents, published[0].topic)

		// Verify event was appended to log
		events := eventLog.events.Snapshot()
		assert.Len(t, events, 1)
		assert.Equal(t, MemoryEventCreated, events[0].EventType)

		// Verify vector clock was incremented
		vc := dmm.GetVectorClock()
		assert.Equal(t, int64(1), vc["test-node"])
	})

	t.Run("PublishFailure", func(t *testing.T) {
		dmm, broker, _ := newTestDMM(t)
		broker.publishErr = fmt.Errorf("broker down")
		ctx := context.Background()

		memory := &Memory{
			ID:       "mem2",
			UserID:   "user1",
			Content:  "test",
			Metadata: map[string]interface{}{},
		}

		err := dmm.AddMemory(ctx, memory)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to publish event")
	})

	t.Run("EventLogAppendFailure_StillPublishes", func(t *testing.T) {
		dmm, _, eventLog := newTestDMM(t)
		eventLog.err = fmt.Errorf("log error")
		ctx := context.Background()

		memory := &Memory{
			ID:       "mem3",
			UserID:   "user1",
			Content:  "test",
			Metadata: map[string]interface{}{},
		}

		// Should not fail; event log error is logged but not fatal
		err := dmm.AddMemory(ctx, memory)
		require.NoError(t, err)
	})

	t.Run("WithoutTagsAndEntities", func(t *testing.T) {
		dmm, _, _ := newTestDMM(t)
		ctx := context.Background()

		memory := &Memory{
			ID:       "mem4",
			UserID:   "user1",
			Content:  "test",
			Metadata: map[string]interface{}{},
		}

		err := dmm.AddMemory(ctx, memory)
		require.NoError(t, err)
	})
}

// --- UpdateMemory ---

func TestDistributedMemoryManager_UpdateMemory(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		dmm, broker, eventLog := newTestDMM(t)
		ctx := context.Background()

		// First add a memory
		memory := &Memory{
			ID:       "mem1",
			UserID:   "user1",
			Content:  "original",
			Metadata: map[string]interface{}{},
		}
		err := dmm.AddMemory(ctx, memory)
		require.NoError(t, err)

		// Now update
		memory.Content = "updated"
		memory.Metadata["tags"] = []string{"newtag"}
		err = dmm.UpdateMemory(ctx, memory)
		require.NoError(t, err)

		// Verify two events published (create + update)
		published := broker.getPublished()
		assert.Len(t, published, 2)

		// Verify vector clock incremented twice
		vc := dmm.GetVectorClock()
		assert.Equal(t, int64(2), vc["test-node"])

		// Verify event log has both events
		events := eventLog.events.Snapshot()
		assert.Len(t, events, 2)
		assert.Equal(t, MemoryEventUpdated, events[1].EventType)
	})

	t.Run("UpdateNonExistent_Fails", func(t *testing.T) {
		dmm, _, _ := newTestDMM(t)
		ctx := context.Background()

		memory := &Memory{
			ID:      "nonexistent",
			UserID:  "user1",
			Content: "test",
		}

		err := dmm.UpdateMemory(ctx, memory)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update memory locally")
	})

	t.Run("PublishFailure", func(t *testing.T) {
		dmm, broker, _ := newTestDMM(t)
		ctx := context.Background()

		// Add memory first
		memory := &Memory{ID: "mem1", UserID: "user1", Content: "test", Metadata: map[string]interface{}{}}
		err := dmm.AddMemory(ctx, memory)
		require.NoError(t, err)

		// Make broker fail, then try update
		broker.publishErr = fmt.Errorf("publish fail")
		memory.Content = "updated"
		err = dmm.UpdateMemory(ctx, memory)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to publish event")
	})
}

// --- DeleteMemory ---

func TestDistributedMemoryManager_DeleteMemory(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		dmm, broker, _ := newTestDMM(t)
		ctx := context.Background()

		// Add then delete
		memory := &Memory{ID: "mem1", UserID: "user1", Content: "test", Metadata: map[string]interface{}{}}
		err := dmm.AddMemory(ctx, memory)
		require.NoError(t, err)

		err = dmm.DeleteMemory(ctx, "mem1", "user1")
		require.NoError(t, err)

		published := broker.getPublished()
		assert.Len(t, published, 2) // create + delete

		vc := dmm.GetVectorClock()
		assert.Equal(t, int64(2), vc["test-node"])
	})

	t.Run("DeleteNonExistent_Fails", func(t *testing.T) {
		dmm, _, _ := newTestDMM(t)
		ctx := context.Background()

		err := dmm.DeleteMemory(ctx, "nonexistent", "user1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete memory locally")
	})

	t.Run("PublishFailure", func(t *testing.T) {
		dmm, broker, _ := newTestDMM(t)
		ctx := context.Background()

		memory := &Memory{ID: "mem1", UserID: "user1", Content: "test", Metadata: map[string]interface{}{}}
		err := dmm.AddMemory(ctx, memory)
		require.NoError(t, err)

		broker.publishErr = fmt.Errorf("kafka down")
		err = dmm.DeleteMemory(ctx, "mem1", "user1")
		require.Error(t, err)
	})
}

// --- ApplyRemoteEvent ---

func TestDistributedMemoryManager_ApplyRemoteEvent(t *testing.T) {
	t.Run("SkipOwnEvents", func(t *testing.T) {
		dmm, _, _ := newTestDMM(t)
		ctx := context.Background()

		event := &MemoryEvent{
			NodeID:      "test-node", // Same as dmm nodeID
			EventType:   MemoryEventCreated,
			MemoryID:    "mem1",
			VectorClock: `{"test-node":1}`,
		}

		err := dmm.ApplyRemoteEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("ApplyRemoteCreate_NewMemory", func(t *testing.T) {
		dmm, _, _ := newTestDMM(t)
		ctx := context.Background()

		event := &MemoryEvent{
			EventID:     "evt1",
			EventType:   MemoryEventCreated,
			NodeID:      "remote-node",
			MemoryID:    "mem1",
			UserID:      "user1",
			Content:     "remote content",
			Importance:  0.7,
			Timestamp:   time.Now(),
			VectorClock: `{"remote-node":1}`,
			Tags:        []string{"tag1"},
			Entities:    []MemoryEntity{{ID: "e1", Name: "E1"}},
		}

		err := dmm.ApplyRemoteEvent(ctx, event)
		require.NoError(t, err)

		// Verify memory was created locally
		mem, err := dmm.localManager.GetMemory(ctx, "mem1")
		require.NoError(t, err)
		assert.Equal(t, "remote content", mem.Content)
		assert.Equal(t, 0.7, mem.Importance)

		// Verify vector clock was updated
		vc := dmm.GetVectorClock()
		assert.Equal(t, int64(1), vc["remote-node"])
	})

	t.Run("ApplyRemoteCreate_ExistingMemory_WithResolver", func(t *testing.T) {
		dmm, _, _ := newTestDMM(t)
		ctx := context.Background()

		// Add existing memory
		existing := &Memory{
			ID:        "mem1",
			UserID:    "user1",
			Content:   "local content",
			Metadata:  map[string]interface{}{},
			UpdatedAt: time.Now().Add(-time.Hour),
		}
		err := dmm.localManager.AddMemory(ctx, existing)
		require.NoError(t, err)

		// Apply remote create for same ID
		event := &MemoryEvent{
			EventType:   MemoryEventCreated,
			NodeID:      "remote-node",
			MemoryID:    "mem1",
			UserID:      "user1",
			Content:     "remote content",
			Timestamp:   time.Now(),
			VectorClock: `{"remote-node":1}`,
		}

		err = dmm.ApplyRemoteEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("ApplyRemoteUpdate_ExistingMemory", func(t *testing.T) {
		dmm, _, _ := newTestDMM(t)
		ctx := context.Background()

		// Add memory first
		existing := &Memory{ID: "mem1", UserID: "user1", Content: "local", Metadata: map[string]interface{}{}}
		err := dmm.localManager.AddMemory(ctx, existing)
		require.NoError(t, err)

		event := &MemoryEvent{
			EventType:   MemoryEventUpdated,
			NodeID:      "remote-node",
			MemoryID:    "mem1",
			UserID:      "user1",
			Content:     "updated remotely",
			Timestamp:   time.Now(),
			VectorClock: `{"remote-node":1}`,
		}

		err = dmm.ApplyRemoteEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("ApplyRemoteUpdate_NonExistent_CreatesNew", func(t *testing.T) {
		dmm, _, _ := newTestDMM(t)
		ctx := context.Background()

		event := &MemoryEvent{
			EventType:   MemoryEventUpdated,
			NodeID:      "remote-node",
			MemoryID:    "new-mem",
			UserID:      "user1",
			Content:     "new from update",
			Timestamp:   time.Now(),
			VectorClock: `{"remote-node":1}`,
		}

		err := dmm.ApplyRemoteEvent(ctx, event)
		require.NoError(t, err)

		mem, err := dmm.localManager.GetMemory(ctx, "new-mem")
		require.NoError(t, err)
		assert.Equal(t, "new from update", mem.Content)
	})

	t.Run("ApplyRemoteDelete", func(t *testing.T) {
		dmm, _, _ := newTestDMM(t)
		ctx := context.Background()

		// Add then apply remote delete
		existing := &Memory{ID: "mem1", UserID: "user1", Content: "to delete", Metadata: map[string]interface{}{}}
		err := dmm.localManager.AddMemory(ctx, existing)
		require.NoError(t, err)

		event := &MemoryEvent{
			EventType:   MemoryEventDeleted,
			NodeID:      "remote-node",
			MemoryID:    "mem1",
			VectorClock: `{"remote-node":1}`,
		}

		err = dmm.ApplyRemoteEvent(ctx, event)
		require.NoError(t, err)

		// Verify deleted
		_, err = dmm.localManager.GetMemory(ctx, "mem1")
		require.Error(t, err)
	})

	t.Run("ApplyRemoteMerge_TreatedAsUpdate", func(t *testing.T) {
		dmm, _, _ := newTestDMM(t)
		ctx := context.Background()

		existing := &Memory{ID: "mem1", UserID: "user1", Content: "original", Metadata: map[string]interface{}{}}
		err := dmm.localManager.AddMemory(ctx, existing)
		require.NoError(t, err)

		event := &MemoryEvent{
			EventType:   MemoryEventMerged,
			NodeID:      "remote-node",
			MemoryID:    "mem1",
			UserID:      "user1",
			Content:     "merged content",
			Timestamp:   time.Now(),
			VectorClock: `{"remote-node":1}`,
		}

		err = dmm.ApplyRemoteEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("UnknownEventType_Ignored", func(t *testing.T) {
		dmm, _, _ := newTestDMM(t)
		ctx := context.Background()

		event := &MemoryEvent{
			EventType:   MemoryEventType("unknown.type"),
			NodeID:      "remote-node",
			MemoryID:    "mem1",
			VectorClock: `{"remote-node":1}`,
		}

		err := dmm.ApplyRemoteEvent(ctx, event)
		require.NoError(t, err)
	})

	t.Run("InvalidVectorClock_ReturnsError", func(t *testing.T) {
		dmm, _, _ := newTestDMM(t)
		ctx := context.Background()

		event := &MemoryEvent{
			EventType:   MemoryEventCreated,
			NodeID:      "remote-node",
			MemoryID:    "mem1",
			VectorClock: "invalid-json",
		}

		err := dmm.ApplyRemoteEvent(ctx, event)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse vector clock")
	})

	t.Run("ApplyRemoteUpdate_NoConflictResolver", func(t *testing.T) {
		store := NewInMemoryStore()
		localManager := NewManager(store, nil, nil, nil, nil, nil)
		broker := newMockBroker()
		eventLog := newMockEventLog()
		// No conflict resolver
		dmm := NewDistributedMemoryManager(localManager, "test-node", eventLog, nil, broker, nil)

		ctx := context.Background()
		existing := &Memory{ID: "mem1", UserID: "user1", Content: "original", Metadata: map[string]interface{}{}}
		err := localManager.AddMemory(ctx, existing)
		require.NoError(t, err)

		event := &MemoryEvent{
			EventType:   MemoryEventUpdated,
			NodeID:      "remote-node",
			MemoryID:    "mem1",
			UserID:      "user1",
			Content:     "remote update",
			Importance:  0.9,
			Timestamp:   time.Now(),
			VectorClock: `{"remote-node":1}`,
			Tags:        []string{"new-tag"},
		}

		err = dmm.ApplyRemoteEvent(ctx, event)
		require.NoError(t, err)

		mem, err := localManager.GetMemory(ctx, "mem1")
		require.NoError(t, err)
		assert.Equal(t, "remote update", mem.Content)
		assert.Equal(t, 0.9, mem.Importance)
	})

	t.Run("ApplyRemoteCreate_ExistingMemory_NoResolver", func(t *testing.T) {
		store := NewInMemoryStore()
		localManager := NewManager(store, nil, nil, nil, nil, nil)
		broker := newMockBroker()
		eventLog := newMockEventLog()
		dmm := NewDistributedMemoryManager(localManager, "test-node", eventLog, nil, broker, nil)

		ctx := context.Background()
		existing := &Memory{ID: "mem1", UserID: "user1", Content: "original", Metadata: map[string]interface{}{}}
		err := localManager.AddMemory(ctx, existing)
		require.NoError(t, err)

		event := &MemoryEvent{
			EventType:   MemoryEventCreated,
			NodeID:      "remote-node",
			MemoryID:    "mem1",
			Content:     "remote content",
			VectorClock: `{"remote-node":1}`,
		}

		err = dmm.ApplyRemoteEvent(ctx, event)
		require.NoError(t, err)

		// Should keep original (no resolver, skip)
		mem, err := localManager.GetMemory(ctx, "mem1")
		require.NoError(t, err)
		assert.Equal(t, "original", mem.Content)
	})
}

// --- Subscribe ---

func TestDistributedMemoryManager_Subscribe(t *testing.T) {
	dmm, _, _ := newTestDMM(t)

	ch := dmm.Subscribe()
	require.NotNil(t, ch)

	// Subscribe again
	ch2 := dmm.Subscribe()
	require.NotNil(t, ch2)

	assert.Equal(t, 2, dmm.subscribers.Len())
}

func TestDistributedMemoryManager_Subscribe_ReceivesEvents(t *testing.T) {
	dmm, _, _ := newTestDMM(t)
	ctx := context.Background()

	ch := dmm.Subscribe()

	memory := &Memory{
		ID:       "mem1",
		UserID:   "user1",
		Content:  "test",
		Metadata: map[string]interface{}{},
	}

	err := dmm.AddMemory(ctx, memory)
	require.NoError(t, err)

	select {
	case event := <-ch:
		assert.Equal(t, MemoryEventCreated, event.EventType)
		assert.Equal(t, "mem1", event.MemoryID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

// --- GetVectorClock ---

func TestDistributedMemoryManager_GetVectorClock(t *testing.T) {
	dmm, _, _ := newTestDMM(t)

	vc := dmm.GetVectorClock()
	assert.Empty(t, vc)

	// Add a memory to increment clock
	ctx := context.Background()
	memory := &Memory{ID: "mem1", UserID: "user1", Content: "test", Metadata: map[string]interface{}{}}
	err := dmm.AddMemory(ctx, memory)
	require.NoError(t, err)

	vc = dmm.GetVectorClock()
	assert.Equal(t, int64(1), vc["test-node"])

	// Verify it's a copy
	vc["test-node"] = 999
	vcAgain := dmm.GetVectorClock()
	assert.Equal(t, int64(1), vcAgain["test-node"])
}

// --- CreateSnapshot ---

func TestDistributedMemoryManager_CreateSnapshot(t *testing.T) {
	dmm, _, _ := newTestDMM(t)
	ctx := context.Background()

	// Add some memories
	for i := 0; i < 3; i++ {
		memory := &Memory{
			ID:       fmt.Sprintf("mem%d", i),
			UserID:   "user1",
			Content:  fmt.Sprintf("content %d", i),
			Metadata: map[string]interface{}{},
		}
		err := dmm.AddMemory(ctx, memory)
		require.NoError(t, err)
	}

	snapshot, err := dmm.CreateSnapshot(ctx, "user1")
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	assert.NotEmpty(t, snapshot.SnapshotID)
	assert.Equal(t, "test-node", snapshot.NodeID)
	assert.Equal(t, "user1", snapshot.UserID)
	assert.Len(t, snapshot.Memories, 3)
	assert.False(t, snapshot.Timestamp.IsZero())
	assert.NotNil(t, snapshot.VectorClock)
}

func TestDistributedMemoryManager_CreateSnapshot_EmptyUser(t *testing.T) {
	dmm, _, _ := newTestDMM(t)
	ctx := context.Background()

	snapshot, err := dmm.CreateSnapshot(ctx, "nonexistent-user")
	require.NoError(t, err)
	assert.Empty(t, snapshot.Memories)
}

// --- GetSyncStatus ---

func TestDistributedMemoryManager_GetSyncStatus(t *testing.T) {
	dmm, _, _ := newTestDMM(t)
	ctx := context.Background()

	// Add a memory to have some state
	memory := &Memory{ID: "mem1", UserID: "user1", Content: "test", Metadata: map[string]interface{}{}}
	err := dmm.AddMemory(ctx, memory)
	require.NoError(t, err)

	// Subscribe to have a subscriber
	_ = dmm.Subscribe()

	status := dmm.GetSyncStatus(ctx)
	require.NotNil(t, status)

	assert.Equal(t, "test-node", status["node_id"])
	assert.Equal(t, false, status["running"])
	assert.Equal(t, true, status["kafka_configured"])
	assert.Equal(t, true, status["conflict_resolver"])
	assert.Equal(t, 1, status["subscriber_count"])
	assert.Equal(t, "active", status["sync_status"])

	vcMap, ok := status["vector_clock"].(map[string]int64)
	assert.True(t, ok)
	assert.Equal(t, int64(1), vcMap["test-node"])
}

func TestDistributedMemoryManager_GetSyncStatus_NilEventLog(t *testing.T) {
	store := NewInMemoryStore()
	localManager := NewManager(store, nil, nil, nil, nil, nil)
	broker := newMockBroker()
	dmm := NewDistributedMemoryManager(localManager, "node1", nil, nil, broker, nil)

	status := dmm.GetSyncStatus(context.Background())
	assert.Equal(t, 0, status["event_count_24h"])
}

// --- ForceSync ---

func TestDistributedMemoryManager_ForceSync(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		dmm, broker, _ := newTestDMM(t)
		ctx := context.Background()

		err := dmm.ForceSync(ctx)
		require.NoError(t, err)

		published := broker.getPublished()
		assert.Len(t, published, 1)
		assert.Equal(t, TopicMemoryEvents, published[0].topic)
		assert.Equal(t, "true", published[0].message.GetHeader("sync_request"))
	})

	t.Run("NoBroker_ReturnsError", func(t *testing.T) {
		store := NewInMemoryStore()
		localManager := NewManager(store, nil, nil, nil, nil, nil)
		dmm := NewDistributedMemoryManager(localManager, "node1", nil, nil, nil, nil)

		err := dmm.ForceSync(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Kafka publisher not configured")
	})

	t.Run("PublishFailure", func(t *testing.T) {
		dmm, broker, _ := newTestDMM(t)
		broker.publishErr = fmt.Errorf("kafka error")

		err := dmm.ForceSync(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to publish sync request")
	})
}

// --- publishEvent with nil broker ---

func TestDistributedMemoryManager_publishEvent_NilBroker(t *testing.T) {
	store := NewInMemoryStore()
	localManager := NewManager(store, nil, nil, nil, nil, nil)
	eventLog := newMockEventLog()
	dmm := NewDistributedMemoryManager(localManager, "node1", eventLog, nil, nil, nil)

	ctx := context.Background()
	memory := &Memory{ID: "mem1", UserID: "user1", Content: "test", Metadata: map[string]interface{}{}}

	err := dmm.AddMemory(ctx, memory)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no Kafka publisher configured")
}

// --- Topic constants ---

func TestTopicConstants(t *testing.T) {
	assert.Equal(t, "helixagent.memory.events", TopicMemoryEvents)
	assert.Equal(t, "helixagent.memory.snapshots", TopicMemorySnapshots)
	assert.Equal(t, "helixagent.memory.conflicts", TopicMemoryConflicts)
}

// --- Manager.GetMemory and Manager.UpdateMemory (manager.go gaps) ---

func TestManager_GetMemory(t *testing.T) {
	store := NewInMemoryStore()
	manager := NewManager(store, nil, nil, nil, nil, nil)
	ctx := context.Background()

	// Add a memory
	err := store.Add(ctx, &Memory{ID: "mem1", UserID: "user1", Content: "test content"})
	require.NoError(t, err)

	t.Run("Success", func(t *testing.T) {
		mem, err := manager.GetMemory(ctx, "mem1")
		require.NoError(t, err)
		assert.Equal(t, "mem1", mem.ID)
		assert.Equal(t, "test content", mem.Content)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := manager.GetMemory(ctx, "nonexistent")
		require.Error(t, err)
	})
}

func TestManager_UpdateMemory(t *testing.T) {
	store := NewInMemoryStore()
	manager := NewManager(store, nil, nil, nil, nil, nil)
	ctx := context.Background()

	// Add a memory
	err := store.Add(ctx, &Memory{ID: "mem1", UserID: "user1", Content: "original"})
	require.NoError(t, err)

	t.Run("Success", func(t *testing.T) {
		beforeUpdate := time.Now()
		err := manager.UpdateMemory(ctx, &Memory{ID: "mem1", Content: "updated"})
		require.NoError(t, err)

		mem, err := store.Get(ctx, "mem1")
		require.NoError(t, err)
		assert.Equal(t, "updated", mem.Content)
		assert.True(t, mem.UpdatedAt.After(beforeUpdate) || mem.UpdatedAt.Equal(beforeUpdate))
	})

	t.Run("NotFound", func(t *testing.T) {
		err := manager.UpdateMemory(ctx, &Memory{ID: "nonexistent", Content: "test"})
		require.Error(t, err)
	})
}
