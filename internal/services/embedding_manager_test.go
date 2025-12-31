package services

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newEmbeddingTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return log
}

func TestNewEmbeddingManager(t *testing.T) {
	log := newEmbeddingTestLogger()
	manager := NewEmbeddingManager(nil, nil, log)

	require.NotNil(t, manager)
	assert.Nil(t, manager.repo)
	assert.Nil(t, manager.cache)
	assert.NotNil(t, manager.log)
	assert.Equal(t, "pgvector", manager.vectorProvider)
}

func TestEmbeddingManager_GenerateEmbedding(t *testing.T) {
	log := newEmbeddingTestLogger()
	manager := NewEmbeddingManager(nil, nil, log)
	ctx := context.Background()

	response, err := manager.GenerateEmbedding(ctx, "test text")
	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.NotEmpty(t, response.Embeddings)
	assert.Equal(t, 384, len(response.Embeddings))
	assert.False(t, response.Timestamp.IsZero())
}

func TestEmbeddingManager_GenerateEmbeddings(t *testing.T) {
	log := newEmbeddingTestLogger()
	manager := NewEmbeddingManager(nil, nil, log)
	ctx := context.Background()

	t.Run("successful generation", func(t *testing.T) {
		req := EmbeddingRequest{
			Text:      "test embedding text",
			Model:     "text-embedding-ada-002",
			Dimension: 1536,
			Batch:     false,
		}
		response, err := manager.GenerateEmbeddings(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.True(t, response.Success)
		assert.NotEmpty(t, response.Embeddings)
		assert.Equal(t, 1536, len(response.Embeddings))
		assert.False(t, response.Timestamp.IsZero())
	})

	t.Run("empty text", func(t *testing.T) {
		req := EmbeddingRequest{
			Text: "",
		}
		response, err := manager.GenerateEmbeddings(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.True(t, response.Success)
	})
}

func TestEmbeddingManager_GetEmbeddingStats(t *testing.T) {
	log := newEmbeddingTestLogger()
	manager := NewEmbeddingManager(nil, nil, log)
	ctx := context.Background()

	stats, err := manager.GetEmbeddingStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Contains(t, stats, "totalEmbeddings")
	assert.Contains(t, stats, "vectorDimension")
	assert.Contains(t, stats, "vectorProvider")
	assert.Contains(t, stats, "lastUpdate")

	assert.Equal(t, "pgvector", stats["vectorProvider"])
	assert.Equal(t, 1536, stats["vectorDimension"])
	assert.Equal(t, 1000, stats["totalEmbeddings"])
}

func TestEmbeddingManager_ListEmbeddingProviders(t *testing.T) {
	log := newEmbeddingTestLogger()
	manager := NewEmbeddingManager(nil, nil, log)
	ctx := context.Background()

	providers, err := manager.ListEmbeddingProviders(ctx)
	require.NoError(t, err)
	require.NotNil(t, providers)
	assert.NotEmpty(t, providers)

	// Check that we have at least the expected providers
	providerNames := make([]string, 0)
	for _, p := range providers {
		if name, ok := p["name"].(string); ok {
			providerNames = append(providerNames, name)
		}
	}

	assert.Contains(t, providerNames, "openai-ada")
	assert.Contains(t, providerNames, "openai-3-small")
	assert.Contains(t, providerNames, "openai-3-large")
}

func TestEmbeddingManager_CosineSimilarity(t *testing.T) {
	log := newEmbeddingTestLogger()
	manager := NewEmbeddingManager(nil, nil, log)

	t.Run("identical vectors", func(t *testing.T) {
		a := []float64{1.0, 0.0, 0.0}
		b := []float64{1.0, 0.0, 0.0}
		similarity := manager.cosineSimilarity(a, b)
		assert.InDelta(t, 1.0, similarity, 0.0001)
	})

	t.Run("orthogonal vectors", func(t *testing.T) {
		a := []float64{1.0, 0.0, 0.0}
		b := []float64{0.0, 1.0, 0.0}
		similarity := manager.cosineSimilarity(a, b)
		assert.InDelta(t, 0.0, similarity, 0.0001)
	})

	t.Run("opposite vectors", func(t *testing.T) {
		a := []float64{1.0, 0.0, 0.0}
		b := []float64{-1.0, 0.0, 0.0}
		similarity := manager.cosineSimilarity(a, b)
		assert.InDelta(t, -1.0, similarity, 0.0001)
	})

	t.Run("different length vectors", func(t *testing.T) {
		a := []float64{1.0, 0.0}
		b := []float64{1.0, 0.0, 0.0}
		similarity := manager.cosineSimilarity(a, b)
		assert.Equal(t, 0.0, similarity)
	})

	t.Run("zero vector", func(t *testing.T) {
		a := []float64{0.0, 0.0, 0.0}
		b := []float64{1.0, 0.0, 0.0}
		similarity := manager.cosineSimilarity(a, b)
		assert.Equal(t, 0.0, similarity)
	})

	t.Run("both zero vectors", func(t *testing.T) {
		a := []float64{0.0, 0.0, 0.0}
		b := []float64{0.0, 0.0, 0.0}
		similarity := manager.cosineSimilarity(a, b)
		assert.Equal(t, 0.0, similarity)
	})

	t.Run("arbitrary vectors", func(t *testing.T) {
		a := []float64{1.0, 2.0, 3.0}
		b := []float64{4.0, 5.0, 6.0}
		// Expected: (1*4 + 2*5 + 3*6) / (sqrt(1+4+9) * sqrt(16+25+36))
		// = (4 + 10 + 18) / (sqrt(14) * sqrt(77))
		// = 32 / sqrt(1078) ≈ 0.9746
		similarity := manager.cosineSimilarity(a, b)
		assert.InDelta(t, 0.9746, similarity, 0.001)
	})

	t.Run("empty vectors", func(t *testing.T) {
		a := []float64{}
		b := []float64{}
		similarity := manager.cosineSimilarity(a, b)
		assert.Equal(t, 0.0, similarity)
	})
}

func TestEmbeddingManager_ConfigureVectorProvider(t *testing.T) {
	log := newEmbeddingTestLogger()
	manager := NewEmbeddingManager(nil, nil, log)
	ctx := context.Background()

	err := manager.ConfigureVectorProvider(ctx, "weaviate")
	assert.NoError(t, err)
	assert.Equal(t, "weaviate", manager.vectorProvider)
}

func TestEmbeddingManager_StoreEmbedding(t *testing.T) {
	log := newEmbeddingTestLogger()
	manager := NewEmbeddingManager(nil, nil, log)
	ctx := context.Background()

	t.Run("store embedding successfully", func(t *testing.T) {
		err := manager.StoreEmbedding(ctx, "doc-1", "test document text", []float64{0.1, 0.2, 0.3})
		assert.NoError(t, err)
	})

	t.Run("store embedding with long text", func(t *testing.T) {
		longText := "This is a longer document text that should be truncated in the log message for readability purposes"
		err := manager.StoreEmbedding(ctx, "doc-2", longText, []float64{0.4, 0.5, 0.6})
		assert.NoError(t, err)
	})

	t.Run("store embedding with empty vector", func(t *testing.T) {
		err := manager.StoreEmbedding(ctx, "doc-3", "text", []float64{})
		assert.NoError(t, err)
	})
}

func TestEmbeddingManager_VectorSearch(t *testing.T) {
	log := newEmbeddingTestLogger()
	manager := NewEmbeddingManager(nil, nil, log)
	ctx := context.Background()

	t.Run("successful vector search", func(t *testing.T) {
		req := VectorSearchRequest{
			Query:     "machine learning",
			Limit:     10,
			Threshold: 0.8,
		}
		response, err := manager.VectorSearch(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.True(t, response.Success)
		assert.NotEmpty(t, response.Results)
		assert.False(t, response.Timestamp.IsZero())
	})

	t.Run("vector search with vector", func(t *testing.T) {
		req := VectorSearchRequest{
			Query:     "",
			Vector:    []float64{0.1, 0.2, 0.3},
			Limit:     5,
			Threshold: 0.5,
		}
		response, err := manager.VectorSearch(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.True(t, response.Success)
	})

	t.Run("vector search results have expected structure", func(t *testing.T) {
		req := VectorSearchRequest{
			Query: "AI and ML",
			Limit: 10,
		}
		response, err := manager.VectorSearch(ctx, req)
		require.NoError(t, err)

		for _, result := range response.Results {
			assert.NotEmpty(t, result.ID)
			assert.NotEmpty(t, result.Content)
			assert.Greater(t, result.Score, 0.0)
			assert.NotNil(t, result.Metadata)
		}
	})
}

func TestEmbeddingManager_IndexDocument(t *testing.T) {
	log := newEmbeddingTestLogger()
	manager := NewEmbeddingManager(nil, nil, log)
	ctx := context.Background()

	t.Run("index document successfully", func(t *testing.T) {
		err := manager.IndexDocument(ctx, "doc-1", "Test Title", "This is the document content", map[string]interface{}{
			"author": "test",
			"source": "unit_test",
		})
		assert.NoError(t, err)
	})

	t.Run("index document with empty metadata", func(t *testing.T) {
		err := manager.IndexDocument(ctx, "doc-2", "Another Title", "More content here", nil)
		assert.NoError(t, err)
	})

	t.Run("index document with empty content", func(t *testing.T) {
		err := manager.IndexDocument(ctx, "doc-3", "Empty Doc", "", nil)
		assert.NoError(t, err)
	})
}

func TestEmbeddingManager_BatchIndexDocuments(t *testing.T) {
	log := newEmbeddingTestLogger()
	manager := NewEmbeddingManager(nil, nil, log)
	ctx := context.Background()

	t.Run("batch index documents successfully", func(t *testing.T) {
		documents := []map[string]interface{}{
			{
				"id":      "batch-doc-1",
				"title":   "First Document",
				"content": "Content of the first document",
				"metadata": map[string]interface{}{
					"type": "article",
				},
			},
			{
				"id":      "batch-doc-2",
				"title":   "Second Document",
				"content": "Content of the second document",
				"metadata": map[string]interface{}{
					"type": "blog",
				},
			},
		}
		err := manager.BatchIndexDocuments(ctx, documents)
		assert.NoError(t, err)
	})

	t.Run("batch index empty documents", func(t *testing.T) {
		documents := []map[string]interface{}{}
		err := manager.BatchIndexDocuments(ctx, documents)
		assert.NoError(t, err)
	})

	t.Run("batch index documents with missing fields", func(t *testing.T) {
		documents := []map[string]interface{}{
			{
				"id": "partial-doc-1",
				// Missing title and content
			},
			{
				"title":   "Only Title",
				"content": "Content but no id",
			},
		}
		// Should not fail, just log errors and continue
		err := manager.BatchIndexDocuments(ctx, documents)
		assert.NoError(t, err)
	})
}

// Test embedding types

func TestEmbeddingRequest_Structure(t *testing.T) {
	req := EmbeddingRequest{
		Text:      "test text",
		Model:     "text-embedding-ada-002",
		Dimension: 1536,
		Batch:     true,
	}

	assert.Equal(t, "test text", req.Text)
	assert.Equal(t, "text-embedding-ada-002", req.Model)
	assert.Equal(t, 1536, req.Dimension)
	assert.True(t, req.Batch)
}

func TestEmbeddingResponse_Structure(t *testing.T) {
	now := time.Now()
	resp := EmbeddingResponse{
		Success:    true,
		Embeddings: []float64{0.1, 0.2, 0.3},
		Error:      "",
		Timestamp:  now,
	}

	assert.True(t, resp.Success)
	assert.Len(t, resp.Embeddings, 3)
	assert.Empty(t, resp.Error)
	assert.Equal(t, now, resp.Timestamp)
}

func TestVectorSearchRequest_Structure(t *testing.T) {
	req := VectorSearchRequest{
		Query:     "search query",
		Vector:    []float64{0.1, 0.2, 0.3},
		Limit:     10,
		Threshold: 0.8,
	}

	assert.Equal(t, "search query", req.Query)
	assert.Len(t, req.Vector, 3)
	assert.Equal(t, 10, req.Limit)
	assert.Equal(t, 0.8, req.Threshold)
}

func TestVectorSearchResponse_Structure(t *testing.T) {
	now := time.Now()
	resp := VectorSearchResponse{
		Success: true,
		Results: []VectorSearchResult{
			{
				ID:       "doc-1",
				Content:  "Document content",
				Score:    0.95,
				Metadata: map[string]interface{}{"author": "test"},
			},
		},
		Error:     "",
		Timestamp: now,
	}

	assert.True(t, resp.Success)
	assert.Len(t, resp.Results, 1)
	assert.Equal(t, "doc-1", resp.Results[0].ID)
	assert.Equal(t, 0.95, resp.Results[0].Score)
}

func TestVectorSearchResult_Structure(t *testing.T) {
	result := VectorSearchResult{
		ID:       "result-123",
		Content:  "Result content text",
		Score:    0.92,
		Metadata: map[string]interface{}{"source": "test", "page": 5},
	}

	assert.Equal(t, "result-123", result.ID)
	assert.Equal(t, "Result content text", result.Content)
	assert.Equal(t, 0.92, result.Score)
	assert.Equal(t, "test", result.Metadata["source"])
	assert.Equal(t, 5, result.Metadata["page"])
}

func TestEmbeddingProviderInfo_Structure(t *testing.T) {
	now := time.Now()
	info := EmbeddingProviderInfo{
		Name:        "openai-ada",
		Model:       "text-embedding-ada-002",
		Dimension:   1536,
		Enabled:     true,
		MaxTokens:   8191,
		Description: "OpenAI Ada v2 embedding model",
		LastSync:    now,
	}

	assert.Equal(t, "openai-ada", info.Name)
	assert.Equal(t, "text-embedding-ada-002", info.Model)
	assert.Equal(t, 1536, info.Dimension)
	assert.True(t, info.Enabled)
	assert.Equal(t, 8191, info.MaxTokens)
	assert.Equal(t, "OpenAI Ada v2 embedding model", info.Description)
	assert.Equal(t, now, info.LastSync)
}

// Benchmarks

func BenchmarkEmbeddingManager_CosineSimilarity(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	manager := NewEmbeddingManager(nil, nil, log)

	a := make([]float64, 1536)
	bo := make([]float64, 1536)
	for i := 0; i < 1536; i++ {
		a[i] = float64(i) * 0.001
		bo[i] = float64(i) * 0.002
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.cosineSimilarity(a, bo)
	}
}

func BenchmarkEmbeddingManager_GenerateEmbedding(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	manager := NewEmbeddingManager(nil, nil, log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.GenerateEmbedding(ctx, "test text for embedding")
	}
}

func BenchmarkEmbeddingManager_ListEmbeddingProviders(b *testing.B) {
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	manager := NewEmbeddingManager(nil, nil, log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.ListEmbeddingProviders(ctx)
	}
}

func TestEmbeddingManager_RefreshAllEmbeddings(t *testing.T) {
	log := newEmbeddingTestLogger()
	manager := NewEmbeddingManager(nil, nil, log)
	ctx := context.Background()

	t.Run("refresh all embeddings no providers", func(t *testing.T) {
		err := manager.RefreshAllEmbeddings(ctx)
		assert.NoError(t, err)
	})

	t.Run("refresh with cache", func(t *testing.T) {
		cacheManager := NewEmbeddingManager(nil, NewInMemoryCache(5*time.Minute), log)
		err := cacheManager.RefreshAllEmbeddings(ctx)
		assert.NoError(t, err)
	})
}

func TestEmbeddingManager_CosineSimilarity_EdgeCases(t *testing.T) {
	log := newEmbeddingTestLogger()
	manager := NewEmbeddingManager(nil, nil, log)

	t.Run("different length vectors", func(t *testing.T) {
		a := []float64{1.0, 2.0, 3.0}
		b := []float64{1.0, 2.0}
		result := manager.cosineSimilarity(a, b)
		assert.Equal(t, 0.0, result)
	})

	t.Run("zero vector", func(t *testing.T) {
		a := []float64{0.0, 0.0, 0.0}
		b := []float64{1.0, 2.0, 3.0}
		result := manager.cosineSimilarity(a, b)
		assert.Equal(t, 0.0, result)
	})

	t.Run("both zero vectors", func(t *testing.T) {
		a := []float64{0.0, 0.0, 0.0}
		b := []float64{0.0, 0.0, 0.0}
		result := manager.cosineSimilarity(a, b)
		assert.Equal(t, 0.0, result)
	})

	t.Run("identical vectors", func(t *testing.T) {
		a := []float64{1.0, 2.0, 3.0}
		b := []float64{1.0, 2.0, 3.0}
		result := manager.cosineSimilarity(a, b)
		assert.InDelta(t, 1.0, result, 0.0001)
	})
}
