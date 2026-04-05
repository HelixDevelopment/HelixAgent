package bigdata

import (
	"context"
	"fmt"
	"testing"
	"time"

	"dev.helix.agent/internal/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestEntity(id, name, entityType string) *memory.Entity {
	return &memory.Entity{
		ID:        id,
		Name:      name,
		Type:      entityType,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func newTestRelationship(id, sourceID, targetID, relType string) *memory.Relationship {
	return &memory.Relationship{
		ID:        id,
		SourceID:  sourceID,
		TargetID:  targetID,
		Type:      relType,
		Strength:  0.8,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// --- NewEntityIntegration tests ---

func TestEntityIntegration_New_Enabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	logger := newTestLogger()

	ei := NewEntityIntegration(broker, logger, true)
	require.NotNil(t, ei)
	assert.True(t, ei.enabled)
	assert.Equal(t, broker, ei.kafkaBroker)
	assert.Equal(t, logger, ei.logger)
}

func TestEntityIntegration_New_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	logger := newTestLogger()

	ei := NewEntityIntegration(broker, logger, false)
	require.NotNil(t, ei)
	assert.False(t, ei.enabled)
}

// --- PublishEntityCreated tests ---

func TestEntityIntegration_PublishEntityCreated_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	entity := newTestEntity("ent-1", "Claude", "person")

	err := ei.PublishEntityCreated(context.Background(), entity, "conv-100")
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
	assert.Equal(t, "helixagent.entities.updates", published[0].topic)
	assert.Equal(t, "entity.created", published[0].message.Headers["event_type"])
	assert.Equal(t, "conv-100", published[0].message.Headers["conversation_id"])
	assert.Equal(t, "ent-1", published[0].message.Headers["entity_id"])
	assert.Equal(t, "person", published[0].message.Headers["entity_type"])
}

func TestEntityIntegration_PublishEntityCreated_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), false)

	entity := newTestEntity("ent-dis", "Test", "concept")

	err := ei.PublishEntityCreated(context.Background(), entity, "conv-dis")
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestEntityIntegration_PublishEntityCreated_BrokerError(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("connection lost")
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	entity := newTestEntity("ent-err", "Broken", "thing")

	err := ei.PublishEntityCreated(context.Background(), entity, "conv-err")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kafka publish failed")
}

// --- PublishEntityUpdated tests ---

func TestEntityIntegration_PublishEntityUpdated_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	entity := newTestEntity("ent-2", "Updated Entity", "concept")

	err := ei.PublishEntityUpdated(context.Background(), entity, "conv-200")
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
	assert.Equal(t, "helixagent.entities.updates", published[0].topic)
	assert.Equal(t, "entity.updated", published[0].message.Headers["event_type"])
	assert.Equal(t, "ent-2", published[0].message.Headers["entity_id"])
}

func TestEntityIntegration_PublishEntityUpdated_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), false)

	entity := newTestEntity("ent-upd-dis", "Noop", "thing")

	err := ei.PublishEntityUpdated(context.Background(), entity, "conv-dis")
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestEntityIntegration_PublishEntityUpdated_BrokerError(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("write timeout")
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	entity := newTestEntity("ent-upd-err", "Fail", "place")

	err := ei.PublishEntityUpdated(context.Background(), entity, "conv-err")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kafka publish failed")
}

// --- PublishRelationshipCreated tests ---

func TestEntityIntegration_PublishRelationshipCreated_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	rel := newTestRelationship("rel-1", "ent-a", "ent-b", "knows")

	err := ei.PublishRelationshipCreated(context.Background(), rel, "conv-300")
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
	assert.Equal(t, "helixagent.relationships.updates", published[0].topic)
	assert.Equal(t, "relationship.created", published[0].message.Headers["event_type"])
	assert.Equal(t, "conv-300", published[0].message.Headers["conversation_id"])
	assert.Equal(t, "ent-a", published[0].message.Headers["source_id"])
	assert.Equal(t, "ent-b", published[0].message.Headers["target_id"])
	assert.Equal(t, "knows", published[0].message.Headers["relationship_type"])
}

func TestEntityIntegration_PublishRelationshipCreated_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), false)

	rel := newTestRelationship("rel-dis", "a", "b", "works_at")

	err := ei.PublishRelationshipCreated(context.Background(), rel, "conv-dis")
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestEntityIntegration_PublishRelationshipCreated_BrokerError(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("broker unavailable")
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	rel := newTestRelationship("rel-err", "x", "y", "located_in")

	err := ei.PublishRelationshipCreated(context.Background(), rel, "conv-err")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kafka publish failed")
}

// --- PublishEntitiesBatch tests ---

func TestEntityIntegration_PublishEntitiesBatch_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	entities := []memory.Entity{
		{ID: "e1", Name: "Entity One", Type: "person", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "e2", Name: "Entity Two", Type: "place", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "e3", Name: "Entity Three", Type: "concept", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	err := ei.PublishEntitiesBatch(context.Background(), entities, "conv-batch")
	assert.NoError(t, err)

	published := broker.getPublished()
	assert.Len(t, published, 3)
	for _, pub := range published {
		assert.Equal(t, "helixagent.entities.updates", pub.topic)
		assert.Equal(t, "entity.created", pub.message.Headers["event_type"])
		assert.Equal(t, "conv-batch", pub.message.Headers["conversation_id"])
	}
}

func TestEntityIntegration_PublishEntitiesBatch_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), false)

	entities := []memory.Entity{
		{ID: "e1", Name: "E1", Type: "person"},
	}

	err := ei.PublishEntitiesBatch(context.Background(), entities, "conv-dis")
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestEntityIntegration_PublishEntitiesBatch_EmptySlice(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	err := ei.PublishEntitiesBatch(context.Background(), []memory.Entity{}, "conv-empty")
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestEntityIntegration_PublishEntitiesBatch_PartialBrokerError(t *testing.T) {
	t.Parallel()
	// Batch continues on error, always returns nil
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("publish error")
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	entities := []memory.Entity{
		{ID: "e1", Name: "E1", Type: "person", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "e2", Name: "E2", Type: "place", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	err := ei.PublishEntitiesBatch(context.Background(), entities, "conv-part-err")
	assert.NoError(t, err) // Returns nil despite individual failures
}

func TestEntityIntegration_PublishEntitiesBatch_NilSlice(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	err := ei.PublishEntitiesBatch(context.Background(), nil, "conv-nil")
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

// --- PublishRelationshipsBatch tests ---

func TestEntityIntegration_PublishRelationshipsBatch_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	rels := []memory.Relationship{
		{
			ID: "r1", SourceID: "a", TargetID: "b", Type: "knows",
			Strength: 0.9, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
		{
			ID: "r2", SourceID: "b", TargetID: "c", Type: "works_at",
			Strength: 0.7, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}

	err := ei.PublishRelationshipsBatch(context.Background(), rels, "conv-rbatch")
	assert.NoError(t, err)

	published := broker.getPublished()
	assert.Len(t, published, 2)
	for _, pub := range published {
		assert.Equal(t, "helixagent.relationships.updates", pub.topic)
		assert.Equal(t, "relationship.created", pub.message.Headers["event_type"])
	}
}

func TestEntityIntegration_PublishRelationshipsBatch_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), false)

	rels := []memory.Relationship{
		{ID: "r1", SourceID: "a", TargetID: "b", Type: "knows"},
	}

	err := ei.PublishRelationshipsBatch(context.Background(), rels, "conv-dis")
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestEntityIntegration_PublishRelationshipsBatch_EmptySlice(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	err := ei.PublishRelationshipsBatch(context.Background(), []memory.Relationship{}, "conv-empty")
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestEntityIntegration_PublishRelationshipsBatch_PartialBrokerError(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("publish failed")
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	rels := []memory.Relationship{
		{
			ID: "r1", SourceID: "a", TargetID: "b", Type: "knows",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}

	err := ei.PublishRelationshipsBatch(context.Background(), rels, "conv-err")
	assert.NoError(t, err) // Returns nil despite individual failures
}

// --- PublishEntityMerge tests ---

func TestEntityIntegration_PublishEntityMerge_Success(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	source := newTestEntity("ent-src", "Source Entity", "person")
	target := newTestEntity("ent-tgt", "Target Entity", "person")

	err := ei.PublishEntityMerge(context.Background(), source, target, "conv-merge")
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
	assert.Equal(t, "helixagent.entities.updates", published[0].topic)
	assert.Equal(t, "entity.merged", published[0].message.Headers["event_type"])
	assert.Equal(t, "conv-merge", published[0].message.Headers["conversation_id"])
	assert.Equal(t, "ent-src", published[0].message.Headers["source_id"])
	assert.Equal(t, "ent-tgt", published[0].message.Headers["target_id"])
}

func TestEntityIntegration_PublishEntityMerge_Disabled(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), false)

	source := newTestEntity("s", "S", "thing")
	target := newTestEntity("t", "T", "thing")

	err := ei.PublishEntityMerge(context.Background(), source, target, "conv-dis")
	assert.NoError(t, err)
	assert.Empty(t, broker.getPublished())
}

func TestEntityIntegration_PublishEntityMerge_BrokerError(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	broker.publishErr = fmt.Errorf("merge publish failed")
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	source := newTestEntity("s-err", "Source", "person")
	target := newTestEntity("t-err", "Target", "person")

	err := ei.PublishEntityMerge(context.Background(), source, target, "conv-err")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kafka publish failed")
}

// --- Entity with properties tests ---

func TestEntityIntegration_PublishEntityCreated_WithProperties(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	entity := &memory.Entity{
		ID:   "ent-props",
		Name: "Rich Entity",
		Type: "concept",
		Properties: map[string]interface{}{
			"category": "technology",
			"score":    42.5,
		},
		Aliases:   []string{"alias1", "alias2"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := ei.PublishEntityCreated(context.Background(), entity, "conv-props")
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
	assert.Equal(t, "ent-props", published[0].message.Headers["entity_id"])
	assert.Equal(t, "concept", published[0].message.Headers["entity_type"])
}

// --- Relationship with properties tests ---

func TestEntityIntegration_PublishRelationshipCreated_WithProperties(t *testing.T) {
	t.Parallel()
	broker := newMockBroker()
	ei := NewEntityIntegration(broker, newTestLogger(), true)

	rel := &memory.Relationship{
		ID:       "rel-props",
		SourceID: "src",
		TargetID: "tgt",
		Type:     "employed_by",
		Properties: map[string]interface{}{
			"since": "2024-01-01",
			"role":  "engineer",
		},
		Strength:  0.95,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := ei.PublishRelationshipCreated(context.Background(), rel, "conv-rel-props")
	assert.NoError(t, err)

	published := broker.getPublished()
	require.Len(t, published, 1)
	assert.Equal(t, "employed_by", published[0].message.Headers["relationship_type"])
	assert.Equal(t, "src", published[0].message.Headers["source_id"])
	assert.Equal(t, "tgt", published[0].message.Headers["target_id"])
}
