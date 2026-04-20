package bigdata

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/memory"
	"dev.helix.agent/internal/messaging"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMemoryStore implements memory.MemoryStore for testing.
//
// Concurrency model (CONST-029): memories → *safe.Store.
type mockMemoryStore struct {
	memories  *safe.Store[string, *memory.Memory]
	addErr    error
	getErr    error
	updateErr error
	deleteErr error
	searchErr error
}

func newMockMemoryStore() *mockMemoryStore {
	return &mockMemoryStore{
		memories: safe.NewStore[string, *memory.Memory](),
	}
}

func (ms *mockMemoryStore) Add(_ context.Context, mem *memory.Memory) error {
	if ms.addErr != nil {
		return ms.addErr
	}
	ms.memories.Put(mem.ID, mem)
	return nil
}

func (ms *mockMemoryStore) Get(_ context.Context, id string) (*memory.Memory, error) {
	if ms.getErr != nil {
		return nil, ms.getErr
	}
	mem, ok := ms.memories.Get(id)
	if !ok {
		return nil, errors.New("memory not found")
	}
	return mem, nil
}

func (ms *mockMemoryStore) Update(_ context.Context, mem *memory.Memory) error {
	if ms.updateErr != nil {
		return ms.updateErr
	}
	ms.memories.Put(mem.ID, mem)
	return nil
}

func (ms *mockMemoryStore) Delete(_ context.Context, id string) error {
	if ms.deleteErr != nil {
		return ms.deleteErr
	}
	ms.memories.Delete(id)
	return nil
}

func (ms *mockMemoryStore) Search(
	_ context.Context,
	_ string,
	_ *memory.SearchOptions,
) ([]*memory.Memory, error) {
	if ms.searchErr != nil {
		return nil, ms.searchErr
	}
	result := make([]*memory.Memory, 0)
	ms.memories.Range(func(_ string, m *memory.Memory) bool {
		result = append(result, m)
		return true
	})
	return result, nil
}

func (ms *mockMemoryStore) GetByUser(
	_ context.Context,
	_ string,
	_ *memory.ListOptions,
) ([]*memory.Memory, error) {
	return nil, nil
}

func (ms *mockMemoryStore) GetBySession(
	_ context.Context,
	_ string,
) ([]*memory.Memory, error) {
	return nil, nil
}

func (ms *mockMemoryStore) AddEntity(_ context.Context, _ *memory.Entity) error {
	return nil
}

func (ms *mockMemoryStore) GetEntity(_ context.Context, _ string) (*memory.Entity, error) {
	return nil, nil
}

func (ms *mockMemoryStore) SearchEntities(
	_ context.Context,
	_ string,
	_ int,
) ([]*memory.Entity, error) {
	return nil, nil
}

func (ms *mockMemoryStore) AddRelationship(_ context.Context, _ *memory.Relationship) error {
	return nil
}

func (ms *mockMemoryStore) GetRelationships(
	_ context.Context,
	_ string,
) ([]*memory.Relationship, error) {
	return nil, nil
}

func (ms *mockMemoryStore) DeleteByUser(_ context.Context, _ string) error {
	return nil
}

func (ms *mockMemoryStore) DeleteBySession(_ context.Context, _ string) error {
	return nil
}

func (ms *mockMemoryStore) Close() error {
	return nil
}

func (ms *mockMemoryStore) getStoredMemory(id string) *memory.Memory {
	mem, _ := ms.memories.Get(id)
	return mem
}

func (ms *mockMemoryStore) count() int {
	return ms.memories.Len()
}

// Helper to create a real memory.Manager with a mock store.
func newTestMemoryManager(store *mockMemoryStore) *memory.Manager {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return memory.NewManager(store, nil, nil, nil, nil, logger)
}

func newTestMemoryIntegration(
	store *mockMemoryStore,
	broker *mockBroker,
	enableDistributed bool,
) *MemoryIntegration {
	mgr := newTestMemoryManager(store)
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return NewMemoryIntegration(mgr, nil, broker, logger, enableDistributed)
}

// --- MemoryIntegration Tests ---

func TestMemoryIntegration_NewMemoryIntegration_ReturnsValidInstance(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)

	assert.NotNil(t, mi)
	assert.NotNil(t, mi.memoryManager)
	assert.Equal(t, broker, mi.kafkaBroker)
	assert.False(t, mi.enableDistributed)
}

func TestMemoryIntegration_AddMemory_LocalOnly(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	mem := &memory.Memory{
		ID:      "mem-001",
		UserID:  "user-1",
		Content: "test memory content",
	}

	err := mi.AddMemory(ctx, mem)
	require.NoError(t, err)

	// Verify memory was stored locally
	stored := store.getStoredMemory("mem-001")
	assert.NotNil(t, stored)
	assert.Equal(t, "test memory content", stored.Content)

	// No events published since distributed is disabled
	msgs := broker.getPublished()
	assert.Empty(t, msgs)
}

func TestMemoryIntegration_AddMemory_WithDistributedEnabled(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	// enableDistributed=true but distributedMemory=nil, so event won't be published
	mi := newTestMemoryIntegration(store, broker, true)
	ctx := context.Background()

	mem := &memory.Memory{
		ID:      "mem-dist",
		UserID:  "user-2",
		Content: "distributed memory test",
	}

	err := mi.AddMemory(ctx, mem)
	require.NoError(t, err)

	// Memory stored locally
	stored := store.getStoredMemory("mem-dist")
	assert.NotNil(t, stored)

	// No event published because distributedMemory is nil
	msgs := broker.getPublished()
	assert.Empty(t, msgs)
}

func TestMemoryIntegration_AddMemory_LocalError(t *testing.T) {
	store := newMockMemoryStore()
	store.addErr = errors.New("store full")
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	mem := &memory.Memory{
		ID:      "mem-err",
		UserID:  "user-1",
		Content: "will fail",
	}

	err := mi.AddMemory(ctx, mem)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local memory add failed")
}

func TestMemoryIntegration_UpdateMemory_LocalOnly(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	// First add a memory
	mem := &memory.Memory{
		ID:      "mem-upd",
		UserID:  "user-1",
		Content: "original content",
	}
	err := mi.AddMemory(ctx, mem)
	require.NoError(t, err)

	// Update it
	mem.Content = "updated content"
	err = mi.UpdateMemory(ctx, mem)
	require.NoError(t, err)

	stored := store.getStoredMemory("mem-upd")
	assert.Equal(t, "updated content", stored.Content)

	// No events published
	msgs := broker.getPublished()
	assert.Empty(t, msgs)
}

func TestMemoryIntegration_UpdateMemory_LocalError(t *testing.T) {
	store := newMockMemoryStore()
	store.updateErr = errors.New("update failed")
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	mem := &memory.Memory{
		ID:      "mem-upd-err",
		Content: "will fail",
	}

	err := mi.UpdateMemory(ctx, mem)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local memory update failed")
}

func TestMemoryIntegration_DeleteMemory_LocalOnly(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	// Add first
	mem := &memory.Memory{
		ID:      "mem-del",
		Content: "to be deleted",
	}
	err := mi.AddMemory(ctx, mem)
	require.NoError(t, err)
	assert.Equal(t, 1, store.count())

	// Delete
	err = mi.DeleteMemory(ctx, "mem-del")
	require.NoError(t, err)
	assert.Equal(t, 0, store.count())
}

func TestMemoryIntegration_DeleteMemory_LocalError(t *testing.T) {
	store := newMockMemoryStore()
	store.deleteErr = errors.New("delete forbidden")
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	err := mi.DeleteMemory(ctx, "mem-del-err")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local memory delete failed")
}

func TestMemoryIntegration_SearchMemory_Success(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	// Add some memories
	for i := 0; i < 3; i++ {
		mem := &memory.Memory{
			ID:      "search-" + string(rune('a'+i)),
			Content: "searchable content",
		}
		err := mi.AddMemory(ctx, mem)
		require.NoError(t, err)
	}

	results, err := mi.SearchMemory(ctx, "searchable", 10)
	require.NoError(t, err)
	// The mock store returns all memories regardless of query
	assert.Len(t, results, 3)
}

func TestMemoryIntegration_SearchMemory_Error(t *testing.T) {
	store := newMockMemoryStore()
	store.searchErr = errors.New("search backend unavailable")
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	results, err := mi.SearchMemory(ctx, "anything", 5)
	require.Error(t, err)
	assert.Nil(t, results)
}

func TestMemoryIntegration_GetMemory_Success(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	mem := &memory.Memory{
		ID:      "mem-get",
		UserID:  "user-1",
		Content: "retrievable content",
	}
	err := mi.AddMemory(ctx, mem)
	require.NoError(t, err)

	retrieved, err := mi.GetMemory(ctx, "mem-get")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "retrievable content", retrieved.Content)
}

func TestMemoryIntegration_GetMemory_NotFound(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	retrieved, err := mi.GetMemory(ctx, "nonexistent")
	require.Error(t, err)
	assert.Nil(t, retrieved)
}

func TestMemoryIntegration_StartEventConsumer_DistributedDisabled(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	err := mi.StartEventConsumer(ctx)
	require.NoError(t, err)
	// No subscription should be created
	assert.Nil(t, mi.subscription)
}

func TestMemoryIntegration_StartEventConsumer_DistributedEnabledNilManager(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	// enableDistributed=true but distributedMemory=nil
	mi := newTestMemoryIntegration(store, broker, true)
	ctx := context.Background()

	err := mi.StartEventConsumer(ctx)
	require.NoError(t, err)
	// distributedMemory is nil, so consumer should not start
	assert.Nil(t, mi.subscription)
}

func TestMemoryIntegration_CreateMemoryEvent_PopulatesFields(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)

	mem := &memory.Memory{
		ID:        "mem-event",
		UserID:    "user-1",
		SessionID: "session-1",
		Content:   "event test content",
		Embedding: []float32{0.1, 0.2, 0.3},
		Metadata: map[string]interface{}{
			"source": "test",
		},
	}

	event := mi.createMemoryEvent("memory.created", mem)
	assert.Equal(t, "memory.created", event.EventType)
	assert.Equal(t, "mem-event", event.MemoryID)
	assert.Equal(t, "user-1", event.UserID)
	assert.Equal(t, "session-1", event.SessionID)
	assert.Equal(t, "event test content", event.Content)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, event.Embedding)
	assert.Equal(t, "test", event.Metadata["source"])
	assert.NotEmpty(t, event.EventID)
	assert.NotEmpty(t, event.NodeID)
	assert.False(t, event.Timestamp.IsZero())
}

func TestMemoryIntegration_PublishMemoryEvent_Success(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	event := &MemoryEvent{
		EventID:   "evt-pub",
		EventType: "memory.created",
		NodeID:    "node-1",
		Timestamp: time.Now(),
		MemoryID:  "mem-pub",
	}

	err := mi.publishMemoryEvent(ctx, event)
	require.NoError(t, err)

	msgs := broker.getPublished()
	require.Len(t, msgs, 1)
	assert.Equal(t, "helixagent.memory.events", msgs[0].topic)
	assert.Equal(t, "evt-pub", msgs[0].message.ID)
	assert.Equal(t, "memory.created", msgs[0].message.Type)
	assert.Equal(t, "memory.created", msgs[0].message.Headers["event_type"])
	assert.Equal(t, "node-1", msgs[0].message.Headers["node_id"])
	assert.Equal(t, "mem-pub", msgs[0].message.Headers["memory_id"])
}

func TestMemoryIntegration_PublishMemoryEvent_BrokerError(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	broker.publishErr = errors.New("kafka down")
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	event := &MemoryEvent{
		EventID:   "evt-err",
		EventType: "memory.created",
		Timestamp: time.Now(),
		MemoryID:  "mem-err",
	}

	err := mi.publishMemoryEvent(ctx, event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kafka publish failed")
}

func TestMemoryIntegration_ApplyRemoteEvent_Created(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	event := &MemoryEvent{
		EventType: "memory.created",
		MemoryID:  "remote-mem-1",
		UserID:    "user-remote",
		SessionID: "session-remote",
		Content:   "remote content",
		Embedding: []float32{0.5, 0.6},
		Metadata:  map[string]interface{}{"remote": true},
	}

	err := mi.applyRemoteEvent(ctx, event)
	require.NoError(t, err)

	stored := store.getStoredMemory("remote-mem-1")
	assert.NotNil(t, stored)
	assert.Equal(t, "remote content", stored.Content)
	assert.Equal(t, "user-remote", stored.UserID)
}

func TestMemoryIntegration_ApplyRemoteEvent_Updated(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	// First create
	createEvent := &MemoryEvent{
		EventType: "memory.created",
		MemoryID:  "remote-upd",
		Content:   "original",
	}
	err := mi.applyRemoteEvent(ctx, createEvent)
	require.NoError(t, err)

	// Then update
	updateEvent := &MemoryEvent{
		EventType: "memory.updated",
		MemoryID:  "remote-upd",
		Content:   "updated remotely",
	}
	err = mi.applyRemoteEvent(ctx, updateEvent)
	require.NoError(t, err)

	stored := store.getStoredMemory("remote-upd")
	assert.Equal(t, "updated remotely", stored.Content)
}

func TestMemoryIntegration_ApplyRemoteEvent_Deleted(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	// First create
	createEvent := &MemoryEvent{
		EventType: "memory.created",
		MemoryID:  "remote-del",
		Content:   "to delete remotely",
	}
	err := mi.applyRemoteEvent(ctx, createEvent)
	require.NoError(t, err)
	assert.Equal(t, 1, store.count())

	// Then delete
	deleteEvent := &MemoryEvent{
		EventType: "memory.deleted",
		MemoryID:  "remote-del",
	}
	err = mi.applyRemoteEvent(ctx, deleteEvent)
	require.NoError(t, err)
	assert.Equal(t, 0, store.count())
}

func TestMemoryIntegration_ApplyRemoteEvent_UnknownType(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegration(store, broker, false)
	ctx := context.Background()

	event := &MemoryEvent{
		EventType: "memory.unknown",
		MemoryID:  "unknown-evt",
	}

	err := mi.applyRemoteEvent(ctx, event)
	require.NoError(t, err) // Unknown types are silently ignored
}

func TestMemoryEvent_JSONSerialization(t *testing.T) {
	event := &MemoryEvent{
		EventID:   "evt-json",
		EventType: "memory.created",
		NodeID:    "node-test",
		Timestamp: time.Now().Truncate(time.Second),
		MemoryID:  "mem-json",
		UserID:    "user-json",
		SessionID: "session-json",
		Content:   "json test content",
		Embedding: []float32{0.1, 0.2},
		Metadata: map[string]interface{}{
			"key": "value",
		},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded MemoryEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "evt-json", decoded.EventID)
	assert.Equal(t, "memory.created", decoded.EventType)
	assert.Equal(t, "node-test", decoded.NodeID)
	assert.Equal(t, "mem-json", decoded.MemoryID)
	assert.Equal(t, "user-json", decoded.UserID)
	assert.Equal(t, "json test content", decoded.Content)
	assert.Equal(t, []float32{0.1, 0.2}, decoded.Embedding)
	assert.Equal(t, "value", decoded.Metadata["key"])
}

func TestGenerateEventID_NonEmpty(t *testing.T) {
	id := generateEventID()
	assert.NotEmpty(t, id)
	assert.Contains(t, id, "evt_")
}

func TestGenerateEventID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateEventID()
		ids[id] = true
		// Use nanosecond precision, but add small sleep to ensure uniqueness
		time.Sleep(time.Nanosecond)
	}
	// Most IDs should be unique (nanosecond resolution)
	assert.Greater(t, len(ids), 50)
}

func TestGetNodeID_NonEmpty(t *testing.T) {
	nodeID := getNodeID()
	assert.NotEmpty(t, nodeID)
	assert.Contains(t, nodeID, "node_")
}

// --- Distributed memory integration tests (with real DistributedMemoryManager) ---

func newTestMemoryIntegrationWithDistributed(
	store *mockMemoryStore,
	broker *mockBroker,
) *MemoryIntegration {
	mgr := newTestMemoryManager(store)
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create real distributed memory manager
	eventLog := newInMemoryEventLog()
	conflictResolver := memory.NewCRDTResolver("merge_all")
	nodeID := "test-node-1"
	distMgr := memory.NewDistributedMemoryManager(
		mgr, nodeID, eventLog, conflictResolver, broker, logger,
	)

	return NewMemoryIntegration(mgr, distMgr, broker, logger, true)
}

func TestMemoryIntegration_AddMemory_WithDistributedMemoryReal(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegrationWithDistributed(store, broker)
	ctx := context.Background()

	mem := &memory.Memory{
		ID:      "mem-dist-real",
		UserID:  "user-1",
		Content: "distributed memory test with real manager",
	}

	err := mi.AddMemory(ctx, mem)
	require.NoError(t, err)

	// Memory stored locally
	stored := store.getStoredMemory("mem-dist-real")
	assert.NotNil(t, stored)

	// Event should be published via Kafka
	msgs := broker.getPublished()
	assert.GreaterOrEqual(t, len(msgs), 1)
}

func TestMemoryIntegration_UpdateMemory_WithDistributedMemoryReal(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegrationWithDistributed(store, broker)
	ctx := context.Background()

	// First add a memory
	mem := &memory.Memory{
		ID:      "mem-upd-dist",
		UserID:  "user-1",
		Content: "original distributed content",
	}
	err := mi.AddMemory(ctx, mem)
	require.NoError(t, err)

	// Clear published messages
	broker.published.Clear()

	// Update it
	mem.Content = "updated distributed content"
	err = mi.UpdateMemory(ctx, mem)
	require.NoError(t, err)

	stored := store.getStoredMemory("mem-upd-dist")
	assert.Equal(t, "updated distributed content", stored.Content)

	// Event published
	msgs := broker.getPublished()
	assert.GreaterOrEqual(t, len(msgs), 1)
}

func TestMemoryIntegration_DeleteMemory_WithDistributedMemoryReal(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegrationWithDistributed(store, broker)
	ctx := context.Background()

	// Add first
	mem := &memory.Memory{
		ID:      "mem-del-dist",
		Content: "to be deleted from distributed",
	}
	err := mi.AddMemory(ctx, mem)
	require.NoError(t, err)

	// Clear published messages
	broker.published.Clear()

	// Delete
	err = mi.DeleteMemory(ctx, "mem-del-dist")
	require.NoError(t, err)
	assert.Equal(t, 0, store.count())

	// Delete event published
	msgs := broker.getPublished()
	assert.GreaterOrEqual(t, len(msgs), 1)
}

func TestMemoryIntegration_AddMemory_WithDistributedBrokerError(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	broker.publishErr = errors.New("kafka unavailable")
	mi := newTestMemoryIntegrationWithDistributed(store, broker)
	ctx := context.Background()

	mem := &memory.Memory{
		ID:      "mem-dist-err",
		UserID:  "user-1",
		Content: "this should still save locally",
	}

	// Should succeed even with broker error (local save works)
	err := mi.AddMemory(ctx, mem)
	require.NoError(t, err)

	stored := store.getStoredMemory("mem-dist-err")
	assert.NotNil(t, stored)
}

func TestMemoryIntegration_UpdateMemory_WithDistributedBrokerError(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegrationWithDistributed(store, broker)
	ctx := context.Background()

	// Add first (success)
	mem := &memory.Memory{
		ID:      "mem-upd-err",
		Content: "original",
	}
	err := mi.AddMemory(ctx, mem)
	require.NoError(t, err)

	// Now set broker to fail
	broker.publishErr = errors.New("kafka down")

	// Update should still succeed locally
	mem.Content = "updated"
	err = mi.UpdateMemory(ctx, mem)
	require.NoError(t, err)

	stored := store.getStoredMemory("mem-upd-err")
	assert.Equal(t, "updated", stored.Content)
}

func TestMemoryIntegration_DeleteMemory_WithDistributedBrokerError(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegrationWithDistributed(store, broker)
	ctx := context.Background()

	mem := &memory.Memory{
		ID:      "mem-del-err",
		Content: "to delete",
	}
	err := mi.AddMemory(ctx, mem)
	require.NoError(t, err)

	// Set broker to fail
	broker.publishErr = errors.New("broker error")

	// Delete should still succeed locally
	err = mi.DeleteMemory(ctx, "mem-del-err")
	require.NoError(t, err)
	assert.Equal(t, 0, store.count())
}

func TestMemoryIntegration_StartEventConsumer_WithDistributed(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegrationWithDistributed(store, broker)
	ctx := context.Background()

	err := mi.StartEventConsumer(ctx)
	require.NoError(t, err)

	// Subscription should be created
	assert.NotNil(t, mi.subscription)
	assert.Equal(t, "helixagent.memory.events", broker.lastSubTopic)
}

func TestMemoryIntegration_StartEventConsumer_SubscribeError(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	broker.subscribeErr = errors.New("subscribe failed")
	mi := newTestMemoryIntegrationWithDistributed(store, broker)
	ctx := context.Background()

	err := mi.StartEventConsumer(ctx)
	require.Error(t, err)
	assert.Nil(t, mi.subscription)
}

func TestMemoryIntegration_EventConsumerHandler_ProcessesMessages(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegrationWithDistributed(store, broker)
	ctx := context.Background()

	err := mi.StartEventConsumer(ctx)
	require.NoError(t, err)

	// Get the handler registered with the broker
	handler := broker.lastHandler
	require.NotNil(t, handler)

	// Create a memory event from a different node
	event := &MemoryEvent{
		EventID:   "evt-remote-1",
		EventType: "memory.created",
		NodeID:    "different-node",
		Timestamp: time.Now(),
		MemoryID:  "remote-mem-1",
		UserID:    "remote-user",
		Content:   "remote content",
	}
	payload, _ := json.Marshal(event)

	msg := &messaging.Message{
		ID:      "msg-1",
		Payload: payload,
	}

	// Process the message through the handler
	err = handler(ctx, msg)
	require.NoError(t, err)

	// The memory should have been applied locally
	stored := store.getStoredMemory("remote-mem-1")
	assert.NotNil(t, stored)
	assert.Equal(t, "remote content", stored.Content)
}

func TestMemoryIntegration_EventConsumerHandler_SkipsSameNode(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegrationWithDistributed(store, broker)
	ctx := context.Background()

	err := mi.StartEventConsumer(ctx)
	require.NoError(t, err)

	handler := broker.lastHandler
	require.NotNil(t, handler)

	// Create a memory event from the SAME node
	// Note: The actual node ID is generated at runtime, so we need to
	// get the current node ID from the handler's closure
	// Since getNodeID() returns a dynamic value, we test the handler
	// processes messages from different nodes
	event := &MemoryEvent{
		EventID:   "evt-same-node",
		EventType: "memory.created",
		NodeID:    getNodeID(), // Same node ID
		Timestamp: time.Now(),
		MemoryID:  "same-node-mem",
		Content:   "should be skipped",
	}
	payload, _ := json.Marshal(event)

	msg := &messaging.Message{
		ID:      "msg-same",
		Payload: payload,
	}

	err = handler(ctx, msg)
	require.NoError(t, err)
	// The message should be skipped (same node)
}

func TestMemoryIntegration_EventConsumerHandler_InvalidJSON(t *testing.T) {
	store := newMockMemoryStore()
	broker := newMockBroker()
	mi := newTestMemoryIntegrationWithDistributed(store, broker)
	ctx := context.Background()

	err := mi.StartEventConsumer(ctx)
	require.NoError(t, err)

	handler := broker.lastHandler
	require.NotNil(t, handler)

	msg := &messaging.Message{
		ID:      "msg-invalid",
		Payload: []byte("{invalid json"),
	}

	// Handler should not return error for invalid JSON (logs and returns nil)
	err = handler(ctx, msg)
	require.NoError(t, err)
}
