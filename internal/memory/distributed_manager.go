package memory

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.concurrency/pkg/safe"

	"dev.helix.agent/internal/messaging"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// DistributedMemoryManager manages memory across multiple nodes with event sourcing.
//
// Concurrency model (CONST-029): subscribers → *safe.Slice; running →
// atomic.Bool. mu (Pattern Zeta, RWMutex → Mutex) survives only to
// serialise vectorClock mutations with the localManager write that
// produced them — the compound "increment clock → local Add" must be
// atomic so consumers see a monotonic event stream.
type DistributedMemoryManager struct {
	localManager     *Manager
	nodeID           string
	vectorClock      VectorClock
	eventLog         EventLog
	conflictResolver *CRDTResolver
	kafkaPublisher   messaging.MessageBroker
	logger           *logrus.Logger

	// State
	mu          sync.Mutex
	running     atomic.Bool
	subscribers *safe.Slice[chan *MemoryEvent]
}

// NewDistributedMemoryManager creates a new distributed memory manager
func NewDistributedMemoryManager(
	localManager *Manager,
	nodeID string,
	eventLog EventLog,
	conflictResolver *CRDTResolver,
	kafkaPublisher messaging.MessageBroker,
	logger *logrus.Logger,
) *DistributedMemoryManager {
	if nodeID == "" {
		nodeID = uuid.New().String()
	}
	if logger == nil {
		logger = logrus.New()
	}

	return &DistributedMemoryManager{
		localManager:     localManager,
		nodeID:           nodeID,
		vectorClock:      NewVectorClock(),
		eventLog:         eventLog,
		conflictResolver: conflictResolver,
		kafkaPublisher:   kafkaPublisher,
		logger:           logger,
		subscribers:      safe.NewSlice[chan *MemoryEvent](),
	}
}

// AddMemory adds a new memory and publishes event to Kafka
func (dmm *DistributedMemoryManager) AddMemory(ctx context.Context, memory *Memory) error {
	dmm.mu.Lock()
	defer dmm.mu.Unlock()

	// Increment vector clock
	dmm.vectorClock.Increment(dmm.nodeID)

	// Add to local store
	if err := dmm.localManager.AddMemory(ctx, memory); err != nil {
		return fmt.Errorf("failed to add memory locally: %w", err)
	}

	// Create memory event
	event := NewMemoryEvent(MemoryEventCreated, dmm.nodeID, memory.ID, memory.UserID)
	event.SessionID = memory.SessionID
	event.Content = memory.Content
	event.Embedding = memory.Embedding
	event.Importance = memory.Importance
	event.VectorClock = dmm.vectorClock.String()

	// Extract tags from metadata if present
	if tags, ok := memory.Metadata["tags"].([]string); ok {
		event.Tags = tags
	}

	// Extract entities from metadata if present
	if entities, ok := memory.Metadata["entities"].([]MemoryEntity); ok && len(entities) > 0 {
		event.Entities = entities
	}

	// Append to event log
	if err := dmm.eventLog.Append(event); err != nil {
		dmm.logger.WithError(err).Error("Failed to append to event log")
	}

	// Publish event to Kafka
	if err := dmm.publishEvent(ctx, event); err != nil {
		dmm.logger.WithError(err).Error("Failed to publish memory event to Kafka")
		return fmt.Errorf("failed to publish event: %w", err)
	}

	// Notify subscribers
	dmm.notifySubscribers(event)

	dmm.logger.WithFields(logrus.Fields{
		"memory_id": memory.ID,
		"user_id":   memory.UserID,
		"node_id":   dmm.nodeID,
	}).Debug("Memory added and event published")

	return nil
}

// UpdateMemory updates an existing memory and publishes event
func (dmm *DistributedMemoryManager) UpdateMemory(ctx context.Context, memory *Memory) error {
	dmm.mu.Lock()
	defer dmm.mu.Unlock()

	// Increment vector clock
	dmm.vectorClock.Increment(dmm.nodeID)

	// Update local store
	if err := dmm.localManager.store.Update(ctx, memory); err != nil {
		return fmt.Errorf("failed to update memory locally: %w", err)
	}

	// Create update event
	event := NewMemoryEvent(MemoryEventUpdated, dmm.nodeID, memory.ID, memory.UserID)
	event.SessionID = memory.SessionID
	event.Content = memory.Content
	event.Embedding = memory.Embedding
	event.Importance = memory.Importance
	event.VectorClock = dmm.vectorClock.String()

	// Extract tags from metadata if present
	if tags, ok := memory.Metadata["tags"].([]string); ok {
		event.Tags = tags
	}

	// Append to event log
	if err := dmm.eventLog.Append(event); err != nil {
		dmm.logger.WithError(err).Error("Failed to append update event to log")
	}

	// Publish event
	if err := dmm.publishEvent(ctx, event); err != nil {
		dmm.logger.WithError(err).Error("Failed to publish update event")
		return fmt.Errorf("failed to publish event: %w", err)
	}

	// Notify subscribers
	dmm.notifySubscribers(event)

	return nil
}

// DeleteMemory deletes a memory and publishes event
func (dmm *DistributedMemoryManager) DeleteMemory(ctx context.Context, memoryID, userID string) error {
	dmm.mu.Lock()
	defer dmm.mu.Unlock()

	// Increment vector clock
	dmm.vectorClock.Increment(dmm.nodeID)

	// Delete from local store
	if err := dmm.localManager.DeleteMemory(ctx, memoryID); err != nil {
		return fmt.Errorf("failed to delete memory locally: %w", err)
	}

	// Create delete event
	event := NewMemoryEvent(MemoryEventDeleted, dmm.nodeID, memoryID, userID)
	event.VectorClock = dmm.vectorClock.String()

	// Append to event log
	if err := dmm.eventLog.Append(event); err != nil {
		dmm.logger.WithError(err).Error("Failed to append delete event to log")
	}

	// Publish event
	if err := dmm.publishEvent(ctx, event); err != nil {
		dmm.logger.WithError(err).Error("Failed to publish delete event")
		return fmt.Errorf("failed to publish event: %w", err)
	}

	// Notify subscribers
	dmm.notifySubscribers(event)

	return nil
}

// ApplyRemoteEvent applies an event from a remote node
func (dmm *DistributedMemoryManager) ApplyRemoteEvent(ctx context.Context, event *MemoryEvent) error {
	// Skip events from this node
	if event.NodeID == dmm.nodeID {
		return nil
	}

	dmm.mu.Lock()
	defer dmm.mu.Unlock()

	dmm.logger.WithFields(logrus.Fields{
		"event_id":   event.EventID,
		"event_type": event.EventType,
		"node_id":    event.NodeID,
		"memory_id":  event.MemoryID,
	}).Debug("Applying remote event")

	// Update vector clock
	remoteVC, err := ParseVectorClock(event.VectorClock)
	if err != nil {
		return fmt.Errorf("failed to parse vector clock: %w", err)
	}
	dmm.vectorClock.Update(remoteVC)

	// Apply event based on type
	switch event.EventType {
	case MemoryEventCreated:
		return dmm.applyRemoteCreate(ctx, event)

	case MemoryEventUpdated:
		return dmm.applyRemoteUpdate(ctx, event)

	case MemoryEventDeleted:
		return dmm.applyRemoteDelete(ctx, event)

	case MemoryEventMerged:
		return dmm.applyRemoteMerge(ctx, event)

	default:
		dmm.logger.WithField("event_type", event.EventType).Warn("Unknown event type")
		return nil
	}
}

// applyRemoteCreate applies a remote memory creation event
func (dmm *DistributedMemoryManager) applyRemoteCreate(ctx context.Context, event *MemoryEvent) error {
	// Check if memory already exists
	existing, err := dmm.localManager.store.Get(ctx, event.MemoryID)
	if err == nil && existing != nil {
		// Memory exists - check for conflicts
		if dmm.conflictResolver != nil {
			merged := dmm.conflictResolver.Merge(existing, event)
			return dmm.localManager.store.Update(ctx, merged)
		}
		// No conflict resolver, skip
		return nil
	}

	// Create new memory from event
	memory := &Memory{
		ID:         event.MemoryID,
		UserID:     event.UserID,
		SessionID:  event.SessionID,
		Content:    event.Content,
		Embedding:  event.Embedding,
		Importance: event.Importance,
		CreatedAt:  event.Timestamp,
		UpdatedAt:  event.Timestamp,
		LastAccess: event.Timestamp,
		Metadata:   make(map[string]interface{}),
	}

	// Store tags in metadata
	if len(event.Tags) > 0 {
		memory.Metadata["tags"] = event.Tags
	}

	// Store entities in metadata
	if len(event.Entities) > 0 {
		memory.Metadata["entities"] = event.Entities
	}

	// Add to local store (bypass AddMemory to avoid publishing another event)
	return dmm.localManager.store.Add(ctx, memory)
}

// applyRemoteUpdate applies a remote memory update event
func (dmm *DistributedMemoryManager) applyRemoteUpdate(ctx context.Context, event *MemoryEvent) error {
	// Get existing memory
	existing, err := dmm.localManager.store.Get(ctx, event.MemoryID)
	if err != nil {
		// Memory doesn't exist, create it
		return dmm.applyRemoteCreate(ctx, event)
	}

	// Apply update with conflict resolution if needed
	if dmm.conflictResolver != nil {
		merged := dmm.conflictResolver.Merge(existing, event)
		return dmm.localManager.store.Update(ctx, merged)
	}

	// No conflict resolver, use last-write-wins
	existing.Content = event.Content
	existing.Embedding = event.Embedding
	existing.Importance = event.Importance
	existing.UpdatedAt = event.Timestamp

	// Update tags in metadata
	if len(event.Tags) > 0 {
		if existing.Metadata == nil {
			existing.Metadata = make(map[string]interface{})
		}
		existing.Metadata["tags"] = event.Tags
	}

	return dmm.localManager.store.Update(ctx, existing)
}

// applyRemoteDelete applies a remote memory deletion event
func (dmm *DistributedMemoryManager) applyRemoteDelete(ctx context.Context, event *MemoryEvent) error {
	// Delete from local store
	return dmm.localManager.store.Delete(ctx, event.MemoryID)
}

// applyRemoteMerge applies a remote memory merge event
func (dmm *DistributedMemoryManager) applyRemoteMerge(ctx context.Context, event *MemoryEvent) error {
	// For now, treat merge as an update
	return dmm.applyRemoteUpdate(ctx, event)
}

// publishEvent publishes a memory event to Kafka
func (dmm *DistributedMemoryManager) publishEvent(ctx context.Context, event *MemoryEvent) error {
	if dmm.kafkaPublisher == nil {
		return fmt.Errorf("no Kafka publisher configured")
	}

	data, err := event.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	msg := messaging.NewMessage(string(event.EventType), data)
	msg.SetHeader("node_id", event.NodeID)
	msg.SetHeader("memory_id", event.MemoryID)
	msg.SetHeader("user_id", event.UserID)

	return dmm.kafkaPublisher.Publish(ctx, TopicMemoryEvents, msg)
}

// Subscribe creates a subscription channel for memory events
func (dmm *DistributedMemoryManager) Subscribe() <-chan *MemoryEvent {
	dmm.mu.Lock()
	defer dmm.mu.Unlock()

	ch := make(chan *MemoryEvent, 100)
	dmm.subscribers.Append(ch)
	return ch
}

// notifySubscribers notifies all subscribers of a new event
func (dmm *DistributedMemoryManager) notifySubscribers(event *MemoryEvent) {
	for _, ch := range dmm.subscribers.Snapshot() {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
}

// GetVectorClock returns a copy of the current vector clock
func (dmm *DistributedMemoryManager) GetVectorClock() VectorClock {
	dmm.mu.Lock()
	defer dmm.mu.Unlock()
	return dmm.cloneVectorClockLocked()
}

// cloneVectorClockLocked returns a copy of the vector clock. Caller
// must hold dmm.mu.
func (dmm *DistributedMemoryManager) cloneVectorClockLocked() VectorClock {
	vc := NewVectorClock()
	for k, v := range dmm.vectorClock {
		vc[k] = v
	}
	return vc
}

// CreateSnapshot creates a snapshot of the current memory state
func (dmm *DistributedMemoryManager) CreateSnapshot(ctx context.Context, userID string) (*MemorySnapshot, error) {
	dmm.mu.Lock()
	defer dmm.mu.Unlock()

	// Get all memories for user
	memories, err := dmm.localManager.GetUserMemories(ctx, userID, &ListOptions{
		Limit:  10000,
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get memories: %w", err)
	}

	snapshot := &MemorySnapshot{
		SnapshotID:  uuid.New().String(),
		Timestamp:   time.Now(),
		NodeID:      dmm.nodeID,
		UserID:      userID,
		Memories:    memories,
		VectorClock: dmm.cloneVectorClockLocked(),
	}

	return snapshot, nil
}

// GetSyncStatus returns the status of distributed memory synchronization
func (dmm *DistributedMemoryManager) GetSyncStatus(ctx context.Context) map[string]interface{} {
	dmm.mu.Lock()
	defer dmm.mu.Unlock()

	// Get vector clock (under our own lock — avoid nested GetVectorClock call)
	vc := dmm.cloneVectorClockLocked()
	vcMap := make(map[string]int64)
	for node, count := range vc {
		vcMap[node] = count
	}

	// Get event log stats (if available)
	eventCount := 0
	if dmm.eventLog != nil {
		events, _ := dmm.eventLog.GetEventsSince(time.Now().Add(-24 * time.Hour)) //nolint:errcheck
		eventCount = len(events)
	}

	// Local memory stats would require store.Count method; skip for now

	// Get subscriber count
	subscriberCount := dmm.subscribers.Len()

	return map[string]interface{}{
		"node_id":           dmm.nodeID,
		"running":           dmm.running.Load(),
		"vector_clock":      vcMap,
		"event_count_24h":   eventCount,
		"subscriber_count":  subscriberCount,
		"kafka_configured":  dmm.kafkaPublisher != nil,
		"conflict_resolver": dmm.conflictResolver != nil,
		"sync_status":       "active", // Could be more detailed
		"timestamp":         time.Now(),
	}
}

// ForceSync forces synchronization with other nodes
func (dmm *DistributedMemoryManager) ForceSync(ctx context.Context) error {
	dmm.mu.Lock()
	defer dmm.mu.Unlock()

	if dmm.kafkaPublisher == nil {
		return fmt.Errorf("Kafka publisher not configured")
	}

	// Create a sync request event
	event := NewMemoryEvent(MemoryEventSyncRequest, dmm.nodeID, "", "")
	event.VectorClock = dmm.vectorClock.String()
	event.Timestamp = time.Now()

	// Publish sync request
	data, err := event.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize sync event: %w", err)
	}

	msg := messaging.NewMessage(string(event.EventType), data)
	msg.SetHeader("node_id", event.NodeID)
	msg.SetHeader("sync_request", "true")

	if err := dmm.kafkaPublisher.Publish(ctx, TopicMemoryEvents, msg); err != nil {
		return fmt.Errorf("failed to publish sync request: %w", err)
	}

	dmm.logger.WithFields(logrus.Fields{
		"node_id": dmm.nodeID,
	}).Info("Force sync requested")

	return nil
}

// Kafka topic for memory events
const (
	TopicMemoryEvents    = "helixagent.memory.events"
	TopicMemorySnapshots = "helixagent.memory.snapshots"
	TopicMemoryConflicts = "helixagent.memory.conflicts"
)
