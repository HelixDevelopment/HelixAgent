package servers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	adapter := NewMemoryAdapter(config, nil)

	assert.NotNil(t, adapter)
	assert.False(t, adapter.initialized)
	assert.Equal(t, 10000, adapter.config.MaxEntities)
	assert.Equal(t, 50000, adapter.config.MaxRelations)
}

func TestDefaultMemoryAdapterConfig(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()

	homeDir, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(homeDir, ".helix", "memory"), config.StoragePath)
	assert.Equal(t, 10000, config.MaxEntities)
	assert.Equal(t, 50000, config.MaxRelations)
	assert.True(t, config.EnablePersistence)
	assert.Equal(t, 5*time.Minute, config.AutoSaveInterval)
}

func TestMemoryAdapter_Initialize(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "memory-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	config := DefaultMemoryAdapterConfig()
	config.StoragePath = tempDir
	config.AutoSaveInterval = 0 // Disable auto-save for tests
	adapter := NewMemoryAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)
	assert.True(t, adapter.initialized)
	assert.NotNil(t, adapter.graph)
}

func TestMemoryAdapter_Initialize_DisabledPersistence(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)
	assert.True(t, adapter.initialized)
}

func TestMemoryAdapter_Health(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	// Health check should fail if not initialized
	err := adapter.Health(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Initialize and check again
	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	err = adapter.Health(context.Background())
	assert.NoError(t, err)
}

func TestMemoryAdapter_CreateEntity(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	entity, err := adapter.CreateEntity(
		context.Background(),
		"John Doe",
		"person",
		[]string{"Works at Acme", "Lives in NYC"},
		map[string]interface{}{"age": 30},
	)
	require.NoError(t, err)

	assert.NotEmpty(t, entity.ID)
	assert.Equal(t, "John Doe", entity.Name)
	assert.Equal(t, "person", entity.EntityType)
	assert.Len(t, entity.Observations, 2)
	assert.Equal(t, 30, entity.Properties["age"])
}

func TestMemoryAdapter_CreateEntity_MaxLimit(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	config.MaxEntities = 2
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, err = adapter.CreateEntity(context.Background(), "Entity1", "test", nil, nil)
	require.NoError(t, err)

	_, err = adapter.CreateEntity(context.Background(), "Entity2", "test", nil, nil)
	require.NoError(t, err)

	_, err = adapter.CreateEntity(context.Background(), "Entity3", "test", nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum entity limit")
}

func TestMemoryAdapter_GetEntity(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	created, err := adapter.CreateEntity(context.Background(), "TestEntity", "test", nil, nil)
	require.NoError(t, err)

	// Get by ID
	entity, err := adapter.GetEntity(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, "TestEntity", entity.Name)

	// Get non-existent
	_, err = adapter.GetEntity(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestMemoryAdapter_GetEntityByName(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, err = adapter.CreateEntity(context.Background(), "TestEntity", "test", nil, nil)
	require.NoError(t, err)

	// Get by name (case insensitive)
	entity, err := adapter.GetEntityByName(context.Background(), "testentity")
	require.NoError(t, err)
	assert.Equal(t, "TestEntity", entity.Name)

	// Get non-existent
	_, err = adapter.GetEntityByName(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestMemoryAdapter_UpdateEntity(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	created, err := adapter.CreateEntity(
		context.Background(),
		"TestEntity",
		"test",
		[]string{"observation1"},
		map[string]interface{}{"key1": "value1"},
	)
	require.NoError(t, err)

	updated, err := adapter.UpdateEntity(
		context.Background(),
		created.ID,
		[]string{"observation2"},
		map[string]interface{}{"key2": "value2"},
	)
	require.NoError(t, err)

	assert.Len(t, updated.Observations, 2)
	assert.Equal(t, "value1", updated.Properties["key1"])
	assert.Equal(t, "value2", updated.Properties["key2"])
}

func TestMemoryAdapter_DeleteEntity(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	entity, err := adapter.CreateEntity(context.Background(), "TestEntity", "test", nil, nil)
	require.NoError(t, err)

	err = adapter.DeleteEntity(context.Background(), entity.ID)
	require.NoError(t, err)

	_, err = adapter.GetEntity(context.Background(), entity.ID)
	assert.Error(t, err)
}

func TestMemoryAdapter_DeleteEntity_RemovesRelations(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	entity1, err := adapter.CreateEntity(context.Background(), "Entity1", "test", nil, nil)
	require.NoError(t, err)

	entity2, err := adapter.CreateEntity(context.Background(), "Entity2", "test", nil, nil)
	require.NoError(t, err)

	_, err = adapter.CreateRelation(context.Background(), entity1.ID, entity2.ID, "related_to", 1.0, nil)
	require.NoError(t, err)

	// Delete entity1 should remove the relation
	err = adapter.DeleteEntity(context.Background(), entity1.ID)
	require.NoError(t, err)

	relations, err := adapter.GetEntityRelations(context.Background(), entity2.ID, "all")
	require.NoError(t, err)
	assert.Len(t, relations, 0)
}

func TestMemoryAdapter_SearchEntities(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, err = adapter.CreateEntity(context.Background(), "John Doe", "person", []string{"Engineer"}, nil)
	require.NoError(t, err)

	_, err = adapter.CreateEntity(context.Background(), "Jane Smith", "person", []string{"Designer"}, nil)
	require.NoError(t, err)

	_, err = adapter.CreateEntity(context.Background(), "Acme Corp", "company", []string{"Tech company"}, nil)
	require.NoError(t, err)

	// Search by name
	results, err := adapter.SearchEntities(context.Background(), "john", "", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "John Doe", results[0].Name)

	// Search by type
	results, err = adapter.SearchEntities(context.Background(), "", "person", 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Search by observation
	results, err = adapter.SearchEntities(context.Background(), "engineer", "", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestMemoryAdapter_CreateRelation(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	entity1, err := adapter.CreateEntity(context.Background(), "Entity1", "test", nil, nil)
	require.NoError(t, err)

	entity2, err := adapter.CreateEntity(context.Background(), "Entity2", "test", nil, nil)
	require.NoError(t, err)

	relation, err := adapter.CreateRelation(
		context.Background(),
		entity1.ID,
		entity2.ID,
		"related_to",
		0.8,
		map[string]interface{}{"note": "test relation"},
	)
	require.NoError(t, err)

	assert.NotEmpty(t, relation.ID)
	assert.Equal(t, entity1.ID, relation.FromEntity)
	assert.Equal(t, entity2.ID, relation.ToEntity)
	assert.Equal(t, "related_to", relation.RelationType)
	assert.Equal(t, float32(0.8), relation.Strength)
}

func TestMemoryAdapter_CreateRelation_InvalidEntities(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, err = adapter.CreateRelation(
		context.Background(),
		"nonexistent1",
		"nonexistent2",
		"related_to",
		1.0,
		nil,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "entity not found")
}

func TestMemoryAdapter_GetEntityRelations(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	entity1, _ := adapter.CreateEntity(context.Background(), "Entity1", "test", nil, nil)
	entity2, _ := adapter.CreateEntity(context.Background(), "Entity2", "test", nil, nil)
	entity3, _ := adapter.CreateEntity(context.Background(), "Entity3", "test", nil, nil)

	_, _ = adapter.CreateRelation(context.Background(), entity1.ID, entity2.ID, "outgoing", 1.0, nil)
	_, _ = adapter.CreateRelation(context.Background(), entity3.ID, entity1.ID, "incoming", 1.0, nil)

	// All relations
	relations, err := adapter.GetEntityRelations(context.Background(), entity1.ID, "all")
	require.NoError(t, err)
	assert.Len(t, relations, 2)

	// Outgoing only
	relations, err = adapter.GetEntityRelations(context.Background(), entity1.ID, "outgoing")
	require.NoError(t, err)
	assert.Len(t, relations, 1)
	assert.Equal(t, "outgoing", relations[0].RelationType)

	// Incoming only
	relations, err = adapter.GetEntityRelations(context.Background(), entity1.ID, "incoming")
	require.NoError(t, err)
	assert.Len(t, relations, 1)
	assert.Equal(t, "incoming", relations[0].RelationType)
}

func TestMemoryAdapter_DeleteRelation(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	entity1, _ := adapter.CreateEntity(context.Background(), "Entity1", "test", nil, nil)
	entity2, _ := adapter.CreateEntity(context.Background(), "Entity2", "test", nil, nil)

	relation, _ := adapter.CreateRelation(context.Background(), entity1.ID, entity2.ID, "related_to", 1.0, nil)

	err = adapter.DeleteRelation(context.Background(), relation.ID)
	require.NoError(t, err)

	_, err = adapter.GetRelation(context.Background(), relation.ID)
	assert.Error(t, err)
}

func TestMemoryAdapter_GetStatistics(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, _ = adapter.CreateEntity(context.Background(), "Person1", "person", nil, nil)
	_, _ = adapter.CreateEntity(context.Background(), "Person2", "person", nil, nil)
	_, _ = adapter.CreateEntity(context.Background(), "Company1", "company", nil, nil)

	stats, err := adapter.GetStatistics(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 3, stats.TotalEntities)
	assert.Equal(t, 2, stats.EntityTypes["person"])
	assert.Equal(t, 1, stats.EntityTypes["company"])
}

func TestMemoryAdapter_AddObservation(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	entity, _ := adapter.CreateEntity(context.Background(), "TestEntity", "test", nil, nil)

	err = adapter.AddObservation(context.Background(), entity.ID, "New observation")
	require.NoError(t, err)

	updated, _ := adapter.GetEntity(context.Background(), entity.ID)
	assert.Contains(t, updated.Observations, "New observation")
}

func TestMemoryAdapter_ReadGraph(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	entity1, _ := adapter.CreateEntity(context.Background(), "Entity1", "test", nil, nil)
	entity2, _ := adapter.CreateEntity(context.Background(), "Entity2", "test", nil, nil)
	_, _ = adapter.CreateRelation(context.Background(), entity1.ID, entity2.ID, "related_to", 1.0, nil)

	result, err := adapter.ReadGraph(context.Background(), []string{"Entity1", "Entity2"})
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.NotNil(t, result["Entity1"])
	assert.NotNil(t, result["Entity2"])
	assert.Len(t, result["Entity1"].Relations, 1)
}

func TestMemoryAdapter_Clear(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, _ = adapter.CreateEntity(context.Background(), "Entity1", "test", nil, nil)
	_, _ = adapter.CreateEntity(context.Background(), "Entity2", "test", nil, nil)

	err = adapter.Clear(context.Background())
	require.NoError(t, err)

	stats, _ := adapter.GetStatistics(context.Background())
	assert.Equal(t, 0, stats.TotalEntities)
}

func TestMemoryAdapter_Persistence(t *testing.T) {
	t.Parallel()
	tempDir, err := os.MkdirTemp("", "memory-adapter-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create adapter and add data
	config := DefaultMemoryAdapterConfig()
	config.StoragePath = tempDir
	config.AutoSaveInterval = 0
	adapter := NewMemoryAdapter(config, logrus.New())

	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	entity, _ := adapter.CreateEntity(context.Background(), "PersistentEntity", "test", nil, nil)

	// Save
	err = adapter.Save(context.Background())
	require.NoError(t, err)

	_ = adapter.Close()

	// Create new adapter and verify data persisted
	adapter2 := NewMemoryAdapter(config, logrus.New())
	err = adapter2.Initialize(context.Background())
	require.NoError(t, err)

	loaded, err := adapter2.GetEntity(context.Background(), entity.ID)
	require.NoError(t, err)
	assert.Equal(t, "PersistentEntity", loaded.Name)
}

func TestMemoryAdapter_SearchWithRelevance(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, _ = adapter.CreateEntity(context.Background(), "Machine Learning", "concept",
		[]string{"AI technique", "Pattern recognition"}, nil)
	_, _ = adapter.CreateEntity(context.Background(), "Deep Learning", "concept",
		[]string{"Subset of machine learning", "Neural networks"}, nil)
	_, _ = adapter.CreateEntity(context.Background(), "Python", "language",
		[]string{"Programming language", "Popular for ML"}, nil)

	results, err := adapter.SearchWithRelevance(context.Background(), "machine learning", 10)
	require.NoError(t, err)

	// Machine Learning should be first (exact match)
	assert.GreaterOrEqual(t, len(results), 2)
	assert.Equal(t, "Machine Learning", results[0].Entity.Name)
	assert.Greater(t, results[0].Score, results[1].Score)
}

func TestMemoryAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	adapter := NewMemoryAdapter(config, logrus.New())

	tools := adapter.GetMCPTools()
	assert.Len(t, tools, 13)

	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "memory_create_entity")
	assert.Contains(t, toolNames, "memory_get_entity")
	assert.Contains(t, toolNames, "memory_update_entity")
	assert.Contains(t, toolNames, "memory_delete_entity")
	assert.Contains(t, toolNames, "memory_search")
	assert.Contains(t, toolNames, "memory_create_relation")
	assert.Contains(t, toolNames, "memory_get_relations")
	assert.Contains(t, toolNames, "memory_delete_relation")
	assert.Contains(t, toolNames, "memory_read_graph")
	assert.Contains(t, toolNames, "memory_add_observation")
	assert.Contains(t, toolNames, "memory_statistics")
	assert.Contains(t, toolNames, "memory_save")
	assert.Contains(t, toolNames, "memory_clear")
}

func TestMemoryAdapter_ExecuteTool(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	// Create entity via tool
	result, err := adapter.ExecuteTool(context.Background(), "memory_create_entity", map[string]interface{}{
		"name":         "TestEntity",
		"entity_type":  "test",
		"observations": []interface{}{"obs1", "obs2"},
	})
	require.NoError(t, err)

	entity, ok := result.(*Entity)
	require.True(t, ok)
	assert.Equal(t, "TestEntity", entity.Name)

	// Get entity via tool
	result, err = adapter.ExecuteTool(context.Background(), "memory_get_entity", map[string]interface{}{
		"name": "TestEntity",
	})
	require.NoError(t, err)

	fetched, ok := result.(*Entity)
	require.True(t, ok)
	assert.Equal(t, entity.ID, fetched.ID)

	// Get statistics via tool
	result, err = adapter.ExecuteTool(context.Background(), "memory_statistics", map[string]interface{}{})
	require.NoError(t, err)

	stats, ok := result.(*GraphStatistics)
	require.True(t, ok)
	assert.Equal(t, 1, stats.TotalEntities)
}

func TestMemoryAdapter_ExecuteTool_Unknown(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, err = adapter.ExecuteTool(context.Background(), "unknown_tool", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestMemoryAdapter_ExecuteTool_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	adapter := NewMemoryAdapter(config, logrus.New())

	_, err := adapter.ExecuteTool(context.Background(), "memory_statistics", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestMemoryAdapter_Close(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)
	assert.True(t, adapter.initialized)

	err = adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.initialized)
}

func TestMemoryAdapter_GetCapabilities(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	caps := adapter.GetCapabilities()
	assert.Equal(t, "memory", caps["name"])
	assert.Equal(t, 10000, caps["max_entities"])
	assert.Equal(t, 50000, caps["max_relations"])
	assert.Equal(t, false, caps["persistence"])
	assert.Equal(t, 13, caps["tools"])
	assert.Equal(t, 0, caps["entity_count"])
}

func TestMemoryAdapter_OpenNodes(t *testing.T) {
	t.Parallel()
	config := DefaultMemoryAdapterConfig()
	config.EnablePersistence = false
	adapter := NewMemoryAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	entity1, _ := adapter.CreateEntity(context.Background(), "Entity1", "test", nil, nil)
	entity2, _ := adapter.CreateEntity(context.Background(), "Entity2", "test", nil, nil)
	_, _ = adapter.CreateRelation(context.Background(), entity1.ID, entity2.ID, "related_to", 1.0, nil)

	nodes, err := adapter.OpenNodes(context.Background(), []string{entity1.ID})
	require.NoError(t, err)

	assert.Len(t, nodes, 1)
	assert.Equal(t, "Entity1", nodes[0].Entity.Name)
	assert.Len(t, nodes[0].Relations, 1)
}
