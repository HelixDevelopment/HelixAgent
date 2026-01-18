package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helper Functions for VectorDocument Repository
// =============================================================================

func setupVectorDocumentTestDB(t *testing.T) (*pgxpool.Pool, *VectorDocumentRepository) {
	ctx := context.Background()
	connString := getTestDBConnString()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Skipf("Skipping test: database not available: %v", err)
		return nil, nil
	}

	repo := NewVectorDocumentRepository(pool)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("Skipping test: database connection failed: %v", err)
		pool.Close()
		return nil, nil
	}

	return pool, repo
}

func cleanupVectorDocumentTestDB(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DELETE FROM vector_documents WHERE title LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup vector_documents: %v", err)
	}
}

func getTestDBConnString() string {
	return "postgres://helixagent:secret@localhost:5432/helixagent_db?sslmode=disable"
}

func createTestVectorDocument() *VectorDocument {
	return &VectorDocument{
		Title:             "test-doc-" + time.Now().Format("20060102150405"),
		Content:           "This is test content for vector embedding.",
		Metadata:          json.RawMessage(`{"source": "test", "category": "unit-test"}`),
		EmbeddingProvider: "pgvector",
	}
}

// =============================================================================
// Unit Tests (no database required)
// =============================================================================

func TestNewVectorDocumentRepository(t *testing.T) {
	repo := NewVectorDocumentRepository(nil)
	assert.NotNil(t, repo)
}

func TestVectorDocument_Fields(t *testing.T) {
	doc := &VectorDocument{
		ID:                "test-id",
		Title:             "Test Title",
		Content:           "Test Content",
		Metadata:          json.RawMessage(`{"key": "value"}`),
		EmbeddingProvider: "openai",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	assert.Equal(t, "test-id", doc.ID)
	assert.Equal(t, "Test Title", doc.Title)
	assert.Equal(t, "Test Content", doc.Content)
	assert.Equal(t, "openai", doc.EmbeddingProvider)
}

func TestVectorDocument_JSONMarshal(t *testing.T) {
	doc := &VectorDocument{
		ID:                "test-id",
		Title:             "Test Title",
		Content:           "Test Content",
		Metadata:          json.RawMessage(`{"key": "value"}`),
		EmbeddingProvider: "pgvector",
	}

	data, err := json.Marshal(doc)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-id")
	assert.Contains(t, string(data), "Test Title")
}

func TestVectorDocument_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"id": "test-id",
		"title": "Test Title",
		"content": "Test Content",
		"metadata": {"key": "value"},
		"embedding_provider": "pgvector"
	}`

	var doc VectorDocument
	err := json.Unmarshal([]byte(jsonData), &doc)
	require.NoError(t, err)
	assert.Equal(t, "test-id", doc.ID)
	assert.Equal(t, "Test Title", doc.Title)
}

func TestVectorDocumentFilter_Empty(t *testing.T) {
	filter := VectorDocumentFilter{}
	assert.Empty(t, filter.Provider)
	assert.Empty(t, filter.TitleLike)
	assert.Equal(t, 0, filter.Limit)
	assert.Equal(t, 0, filter.Offset)
}

func TestVectorDocumentFilter_WithValues(t *testing.T) {
	filter := VectorDocumentFilter{
		Provider:  "openai",
		TitleLike: "test",
		Limit:     10,
		Offset:    5,
	}
	assert.Equal(t, "openai", filter.Provider)
	assert.Equal(t, "test", filter.TitleLike)
	assert.Equal(t, 10, filter.Limit)
	assert.Equal(t, 5, filter.Offset)
}

func TestVectorSearchResult_Fields(t *testing.T) {
	result := VectorSearchResult{
		Document: VectorDocument{
			ID:    "doc-1",
			Title: "Test Doc",
		},
		Similarity: 0.95,
	}
	assert.Equal(t, "doc-1", result.Document.ID)
	assert.Equal(t, 0.95, result.Similarity)
}

// =============================================================================
// Integration Tests (require database)
// =============================================================================

func TestVectorDocumentRepository_Create(t *testing.T) {
	pool, repo := setupVectorDocumentTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupVectorDocumentTestDB(t, pool)

	ctx := context.Background()
	doc := createTestVectorDocument()

	err := repo.Create(ctx, doc)
	require.NoError(t, err)
	assert.NotEmpty(t, doc.ID)
	assert.False(t, doc.CreatedAt.IsZero())
}

func TestVectorDocumentRepository_GetByID(t *testing.T) {
	pool, repo := setupVectorDocumentTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupVectorDocumentTestDB(t, pool)

	ctx := context.Background()
	doc := createTestVectorDocument()
	err := repo.Create(ctx, doc)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, doc.ID, retrieved.ID)
	assert.Equal(t, doc.Title, retrieved.Title)
	assert.Equal(t, doc.Content, retrieved.Content)
}

func TestVectorDocumentRepository_Update(t *testing.T) {
	pool, repo := setupVectorDocumentTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupVectorDocumentTestDB(t, pool)

	ctx := context.Background()
	doc := createTestVectorDocument()
	err := repo.Create(ctx, doc)
	require.NoError(t, err)

	doc.Title = "test-updated-title"
	doc.Content = "Updated content"
	err = repo.Update(ctx, doc)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, "test-updated-title", retrieved.Title)
	assert.Equal(t, "Updated content", retrieved.Content)
}

func TestVectorDocumentRepository_Delete(t *testing.T) {
	pool, repo := setupVectorDocumentTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupVectorDocumentTestDB(t, pool)

	ctx := context.Background()
	doc := createTestVectorDocument()
	err := repo.Create(ctx, doc)
	require.NoError(t, err)

	err = repo.Delete(ctx, doc.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, doc.ID)
	assert.Error(t, err)
}

func TestVectorDocumentRepository_List(t *testing.T) {
	pool, repo := setupVectorDocumentTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupVectorDocumentTestDB(t, pool)

	ctx := context.Background()

	// Create multiple documents
	for i := 0; i < 5; i++ {
		doc := createTestVectorDocument()
		err := repo.Create(ctx, doc)
		require.NoError(t, err)
	}

	// List with limit
	filter := VectorDocumentFilter{Limit: 3}
	docs, err := repo.List(ctx, filter)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(docs), 3)
}

func TestVectorDocumentRepository_Count(t *testing.T) {
	pool, repo := setupVectorDocumentTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupVectorDocumentTestDB(t, pool)

	ctx := context.Background()

	initialCount, err := repo.Count(ctx)
	require.NoError(t, err)

	doc := createTestVectorDocument()
	err = repo.Create(ctx, doc)
	require.NoError(t, err)

	newCount, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, initialCount+1, newCount)
}

func TestVectorDocumentRepository_SearchByTitle(t *testing.T) {
	pool, repo := setupVectorDocumentTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupVectorDocumentTestDB(t, pool)

	ctx := context.Background()

	doc := createTestVectorDocument()
	doc.Title = "test-searchable-unique-title"
	err := repo.Create(ctx, doc)
	require.NoError(t, err)

	results, err := repo.SearchByTitle(ctx, "searchable-unique", 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
}

func TestVectorDocumentRepository_UpdateMetadata(t *testing.T) {
	pool, repo := setupVectorDocumentTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupVectorDocumentTestDB(t, pool)

	ctx := context.Background()

	doc := createTestVectorDocument()
	err := repo.Create(ctx, doc)
	require.NoError(t, err)

	newMetadata := json.RawMessage(`{"updated": true, "version": 2}`)
	err = repo.UpdateMetadata(ctx, doc.ID, newMetadata)
	require.NoError(t, err)

	retrieved, err := repo.GetByID(ctx, doc.ID)
	require.NoError(t, err)
	assert.Contains(t, string(retrieved.Metadata), "updated")
}

func TestVectorDocumentRepository_BulkCreate(t *testing.T) {
	pool, repo := setupVectorDocumentTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupVectorDocumentTestDB(t, pool)

	ctx := context.Background()

	docs := make([]*VectorDocument, 3)
	for i := range docs {
		docs[i] = createTestVectorDocument()
	}

	err := repo.BulkCreate(ctx, docs)
	require.NoError(t, err)

	for _, doc := range docs {
		assert.NotEmpty(t, doc.ID)
		assert.False(t, doc.CreatedAt.IsZero())
	}
}

func TestVectorDocumentRepository_DeleteByProvider(t *testing.T) {
	pool, repo := setupVectorDocumentTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupVectorDocumentTestDB(t, pool)

	ctx := context.Background()

	doc := createTestVectorDocument()
	doc.EmbeddingProvider = "test-provider-delete"
	err := repo.Create(ctx, doc)
	require.NoError(t, err)

	count, err := repo.DeleteByProvider(ctx, "test-provider-delete")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(1))
}

func TestVectorDocumentRepository_DeleteOlderThan(t *testing.T) {
	pool, repo := setupVectorDocumentTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupVectorDocumentTestDB(t, pool)

	ctx := context.Background()

	// Documents created now won't be deleted
	doc := createTestVectorDocument()
	err := repo.Create(ctx, doc)
	require.NoError(t, err)

	// Delete documents older than 1 hour from now (should not delete the new one)
	cutoff := time.Now().Add(-1 * time.Hour)
	_, err = repo.DeleteOlderThan(ctx, cutoff)
	require.NoError(t, err)

	// The document should still exist
	_, err = repo.GetByID(ctx, doc.ID)
	require.NoError(t, err)
}

func TestVectorDocumentRepository_CountByProvider(t *testing.T) {
	pool, repo := setupVectorDocumentTestDB(t)
	if pool == nil {
		return
	}
	defer pool.Close()
	defer cleanupVectorDocumentTestDB(t, pool)

	ctx := context.Background()

	doc := createTestVectorDocument()
	err := repo.Create(ctx, doc)
	require.NoError(t, err)

	counts, err := repo.CountByProvider(ctx)
	require.NoError(t, err)
	assert.NotNil(t, counts)
}
