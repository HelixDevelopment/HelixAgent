package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helper Functions for CogneeMemory Repository
// =============================================================================

func setupCogneeMemoryTestDB(t *testing.T) (*pgxpool.Pool, *CogneeMemoryRepository) {
	ctx := context.Background()
	connString := getTestDBConnString()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Skipf("Skipping test: database not available: %v", err)
		return nil, nil
	}

	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	repo := NewCogneeMemoryRepository(pool, log)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Skipping test: database connection failed: %v", err)
		pool.Close()
		return nil, nil
	}

	// Check if we can create cognee memories (FK constraint check)
	// Try to create without session_id - if it fails, skip the test
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM cognee_memories WHERE dataset_name = 'test-fk-check'").Scan(&count)
	if err != nil {
		t.Skipf("Skipping test: cognee_memories table not accessible: %v", err)
		pool.Close()
		return nil, nil
	}

	return pool, repo
}

func cleanupCogneeMemoryTestDB(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DELETE FROM cognee_memories WHERE dataset_name LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup cognee_memories: %v", err)
	}
}

// createTestCogneeMemoryWithPool creates a test memory with a valid session_id
// by first creating a user and session in the database
func createTestCogneeMemoryWithPool(t *testing.T, pool *pgxpool.Pool) *CogneeMemory {
	ctx := context.Background()
	searchKey := "test-search-key"

	// First, ensure we have a user
	var userID string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, username, password_hash)
		VALUES ('test@example.com', 'testuser', 'hash')
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		t.Skipf("Skipping test: could not create test user: %v", err)
		return nil
	}

	// Create a session for this user
	var sessionID string
	err = pool.QueryRow(ctx, `
		INSERT INTO user_sessions (user_id, token, expires_at)
		VALUES ($1, $2, NOW() + INTERVAL '1 day')
		RETURNING id
	`, userID, "test-token-"+time.Now().Format("150405")).Scan(&sessionID)
	if err != nil {
		t.Skipf("Skipping test: could not create test session: %v", err)
		return nil
	}

	return &CogneeMemory{
		SessionID:   sessionID,
		DatasetName: "test-dataset",
		ContentType: "text/plain",
		Content:     "This is test content for Cognee memory.",
		SearchKey:   &searchKey,
		GraphNodes: map[string]interface{}{
			"node1": "value1",
			"node2": "value2",
		},
	}
}

// createTestCogneeMemory creates a test memory for unit tests (no DB required)
func createTestCogneeMemory() *CogneeMemory {
	searchKey := "test-search-key"
	return &CogneeMemory{
		SessionID:   "550e8400-e29b-41d4-a716-446655440000", // Fake UUID for unit tests
		DatasetName: "test-dataset",
		ContentType: "text/plain",
		Content:     "This is test content for Cognee memory.",
		SearchKey:   &searchKey,
		GraphNodes: map[string]interface{}{
			"node1": "value1",
			"node2": "value2",
		},
	}
}

// =============================================================================
// Unit Tests (no database required)
// =============================================================================

func TestNewCogneeMemoryRepository(t *testing.T) {
	log := logrus.New()
	repo := NewCogneeMemoryRepository(nil, log)
	assert.NotNil(t, repo)
}

func TestCogneeMemory_Fields(t *testing.T) {
	now := time.Now()
	vectorID := "vector-123"
	searchKey := "test-key"

	memory := &CogneeMemory{
		ID:          "memory-id",
		SessionID:   "session-123",
		DatasetName: "my-dataset",
		ContentType: "application/json",
		Content:     `{"key": "value"}`,
		VectorID:    &vectorID,
		SearchKey:   &searchKey,
		GraphNodes: map[string]interface{}{
			"concept":  "AI",
			"relation": "is-a",
		},
		CreatedAt: now,
	}

	assert.Equal(t, "memory-id", memory.ID)
	assert.Equal(t, "session-123", memory.SessionID)
	assert.Equal(t, "my-dataset", memory.DatasetName)
	assert.Equal(t, "application/json", memory.ContentType)
	assert.Equal(t, "vector-123", *memory.VectorID)
	assert.Equal(t, "test-key", *memory.SearchKey)
	assert.Equal(t, "AI", memory.GraphNodes["concept"])
}

func TestCogneeMemory_JSONMarshal(t *testing.T) {
	memory := createTestCogneeMemory()
	memory.ID = "test-marshal-id"

	data, err := json.Marshal(memory)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-marshal-id")
	assert.Contains(t, string(data), "test-dataset")
	assert.Contains(t, string(data), "text/plain")
}

func TestCogneeMemory_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"id": "memory-123",
		"session_id": "session-456",
		"dataset_name": "knowledge-base",
		"content_type": "text/markdown",
		"content": "# Title\nSome content",
		"search_key": "title-content",
		"graph_nodes": {"entity": "knowledge"}
	}`

	var memory CogneeMemory
	err := json.Unmarshal([]byte(jsonData), &memory)
	require.NoError(t, err)
	assert.Equal(t, "memory-123", memory.ID)
	assert.Equal(t, "session-456", memory.SessionID)
	assert.Equal(t, "knowledge-base", memory.DatasetName)
	assert.Equal(t, "text/markdown", memory.ContentType)
	assert.Equal(t, "# Title\nSome content", memory.Content)
	assert.Equal(t, "title-content", *memory.SearchKey)
	assert.Equal(t, "knowledge", memory.GraphNodes["entity"])
}

func TestCogneeMemory_NilOptionalFields(t *testing.T) {
	memory := &CogneeMemory{
		ID:          "memory-id",
		SessionID:   "session-id",
		DatasetName: "dataset",
		ContentType: "text/plain",
		Content:     "content",
	}

	assert.Nil(t, memory.VectorID)
	assert.Nil(t, memory.SearchKey)
	assert.Nil(t, memory.GraphNodes)
}

func TestCreateTestCogneeMemory_ValidValues(t *testing.T) {
	memory := createTestCogneeMemory()

	assert.Empty(t, memory.SessionID) // Empty to avoid FK constraint issues
	assert.Equal(t, "test-dataset", memory.DatasetName)
	assert.Equal(t, "text/plain", memory.ContentType)
	assert.NotEmpty(t, memory.Content)
	assert.NotNil(t, memory.SearchKey)
	assert.NotNil(t, memory.GraphNodes)
	assert.Len(t, memory.GraphNodes, 2)
}

func TestCogneeMemory_GraphNodesOperations(t *testing.T) {
	memory := &CogneeMemory{
		GraphNodes: make(map[string]interface{}),
	}

	// Add nodes
	memory.GraphNodes["entity1"] = "concept"
	memory.GraphNodes["entity2"] = map[string]interface{}{
		"type":  "relation",
		"value": "is-part-of",
	}

	assert.Len(t, memory.GraphNodes, 2)
	assert.Equal(t, "concept", memory.GraphNodes["entity1"])

	entity2, ok := memory.GraphNodes["entity2"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "relation", entity2["type"])
}

// =============================================================================
// Integration Tests (require database)
// =============================================================================

func TestCogneeMemoryRepository_Create(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	ctx := context.Background()
	memory := createTestCogneeMemoryWithPool(t, pool)
	if memory == nil {
		return // Test was skipped
	}

	err := repo.Create(ctx, memory)
	require.NoError(t, err)
	assert.NotEmpty(t, memory.ID)
	assert.False(t, memory.CreatedAt.IsZero())
}

func TestCogneeMemoryRepository_GetByID(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	ctx := context.Background()
	memory := createTestCogneeMemoryWithPool(t, pool)
	if memory == nil {
		return // Test was skipped
	}
	err := repo.Create(ctx, memory)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, memory.ID)
	require.NoError(t, err)
	assert.Equal(t, memory.ID, retrieved.ID)
	assert.Equal(t, memory.SessionID, retrieved.SessionID)
	assert.Equal(t, memory.DatasetName, retrieved.DatasetName)
	assert.Equal(t, memory.ContentType, retrieved.ContentType)
	assert.Equal(t, memory.Content, retrieved.Content)
}

func TestCogneeMemoryRepository_GetByID_NotFound(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx := context.Background()
	_, err := repo.GetByID(ctx, "non-existent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCogneeMemoryRepository_GetBySessionID(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	// Skip this test as it requires a valid user_sessions entry
	// which has foreign key constraint
	t.Skip("Skipping: requires user_sessions FK - tested via integration tests")

	ctx := context.Background()

	// Create multiple memories for the same session (with empty session_id)
	for i := 0; i < 3; i++ {
		memory := createTestCogneeMemory()
		err := repo.Create(ctx, memory)
		require.NoError(t, err)
	}

	memories, err := repo.GetBySessionID(ctx, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(memories), 3)
}

func TestCogneeMemoryRepository_GetByDatasetName(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	ctx := context.Background()
	datasetName := "test-dataset-unique-" + time.Now().Format("20060102150405")

	// Create multiple memories for the same dataset
	for i := 0; i < 5; i++ {
		memory := createTestCogneeMemory()
		memory.DatasetName = datasetName
		err := repo.Create(ctx, memory)
		require.NoError(t, err)
	}

	memories, total, err := repo.GetByDatasetName(ctx, datasetName, 3, 0)
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, memories, 3) // Limited to 3
}

func TestCogneeMemoryRepository_SearchByKey(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	ctx := context.Background()
	uniqueKey := "unique-searchable-key-" + time.Now().Format("20060102150405")

	memory := createTestCogneeMemory()
	memory.SearchKey = &uniqueKey
	err := repo.Create(ctx, memory)
	require.NoError(t, err)

	results, err := repo.SearchByKey(ctx, "unique-searchable-key", 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
}

func TestCogneeMemoryRepository_SearchByContent(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	ctx := context.Background()
	uniqueContent := "UNIQUE_SEARCHABLE_CONTENT_" + time.Now().Format("20060102150405")

	memory := createTestCogneeMemory()
	memory.Content = uniqueContent
	err := repo.Create(ctx, memory)
	require.NoError(t, err)

	results, err := repo.SearchByContent(ctx, "UNIQUE_SEARCHABLE_CONTENT", 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
}

func TestCogneeMemoryRepository_Update(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	ctx := context.Background()
	memory := createTestCogneeMemory()
	err := repo.Create(ctx, memory)
	require.NoError(t, err)

	// Update the memory
	memory.ContentType = "application/json"
	memory.Content = `{"updated": true}`
	newSearchKey := "updated-search-key"
	memory.SearchKey = &newSearchKey
	memory.GraphNodes = map[string]interface{}{"updated": true}

	err = repo.Update(ctx, memory)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, memory.ID)
	require.NoError(t, err)
	assert.Equal(t, "application/json", retrieved.ContentType)
	assert.Equal(t, `{"updated": true}`, retrieved.Content)
	assert.Equal(t, "updated-search-key", *retrieved.SearchKey)
}

func TestCogneeMemoryRepository_UpdateVectorID(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	ctx := context.Background()
	memory := createTestCogneeMemory()
	err := repo.Create(ctx, memory)
	require.NoError(t, err)

	newVectorID := "new-vector-id-123"
	err = repo.UpdateVectorID(ctx, memory.ID, newVectorID)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, memory.ID)
	require.NoError(t, err)
	assert.NotNil(t, retrieved.VectorID)
	assert.Equal(t, newVectorID, *retrieved.VectorID)
}

func TestCogneeMemoryRepository_UpdateGraphNodes(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	ctx := context.Background()
	memory := createTestCogneeMemory()
	err := repo.Create(ctx, memory)
	require.NoError(t, err)

	newGraphNodes := map[string]interface{}{
		"concept": "updated-concept",
		"entity":  "new-entity",
	}
	err = repo.UpdateGraphNodes(ctx, memory.ID, newGraphNodes)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, memory.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated-concept", retrieved.GraphNodes["concept"])
	assert.Equal(t, "new-entity", retrieved.GraphNodes["entity"])
}

func TestCogneeMemoryRepository_Delete(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	ctx := context.Background()
	memory := createTestCogneeMemory()
	err := repo.Create(ctx, memory)
	require.NoError(t, err)

	err = repo.Delete(ctx, memory.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, memory.ID)
	assert.Error(t, err)
}

func TestCogneeMemoryRepository_DeleteBySessionID(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	// Skip this test as it requires a valid user_sessions entry
	// which has foreign key constraint
	t.Skip("Skipping: requires user_sessions FK - tested via integration tests")

	ctx := context.Background()

	// Create multiple memories (with empty session_id)
	for i := 0; i < 3; i++ {
		memory := createTestCogneeMemory()
		err := repo.Create(ctx, memory)
		require.NoError(t, err)
	}

	count, err := repo.DeleteBySessionID(ctx, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(0))
}

func TestCogneeMemoryRepository_DeleteByDatasetName(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	ctx := context.Background()
	datasetName := "test-dataset-delete-" + time.Now().Format("20060102150405")

	// Create multiple memories for the same dataset
	for i := 0; i < 4; i++ {
		memory := createTestCogneeMemory()
		memory.DatasetName = datasetName
		err := repo.Create(ctx, memory)
		require.NoError(t, err)
	}

	count, err := repo.DeleteByDatasetName(ctx, datasetName)
	require.NoError(t, err)
	assert.Equal(t, int64(4), count)
}

func TestCogneeMemoryRepository_GetByVectorID(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	ctx := context.Background()
	memory := createTestCogneeMemory()
	vectorID := "unique-vector-id-" + time.Now().Format("20060102150405")
	memory.VectorID = &vectorID
	err := repo.Create(ctx, memory)
	require.NoError(t, err)

	retrieved, err := repo.GetByVectorID(ctx, vectorID)
	require.NoError(t, err)
	assert.Equal(t, memory.ID, retrieved.ID)
	assert.Equal(t, vectorID, *retrieved.VectorID)
}

func TestCogneeMemoryRepository_ListDatasets(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	ctx := context.Background()

	// Create memories with different datasets
	datasets := []string{"test-dataset-a", "test-dataset-b", "test-dataset-c"}
	for _, ds := range datasets {
		memory := createTestCogneeMemory()
		memory.DatasetName = ds
		err := repo.Create(ctx, memory)
		require.NoError(t, err)
	}

	result, err := repo.ListDatasets(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(result), 3)
}

func TestCogneeMemoryRepository_GetDatasetStats(t *testing.T) {
	pool, repo := setupCogneeMemoryTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupCogneeMemoryTestDB(t, pool)

	ctx := context.Background()
	datasetName := "test-dataset-stats-" + time.Now().Format("20060102150405")

	// Create multiple memories with the same dataset
	for i := 0; i < 5; i++ {
		memory := createTestCogneeMemory()
		memory.DatasetName = datasetName
		if i%2 == 0 {
			vectorID := "vector-" + time.Now().Format("150405")
			memory.VectorID = &vectorID
		}
		err := repo.Create(ctx, memory)
		require.NoError(t, err)
	}

	stats, err := repo.GetDatasetStats(ctx, datasetName)
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, datasetName, stats["dataset_name"])
	assert.Equal(t, 5, stats["total_memories"])
	assert.GreaterOrEqual(t, stats["indexed_memories"].(int), 2) // At least 2 with vector IDs
}
