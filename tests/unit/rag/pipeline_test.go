// CONST-030: these tests exercise the in-process rag.Pipeline,
// rag.AdvancedRAG, query-expansion, re-ranking, chunking, and embedding
// registry wiring against a canned MockEmbeddingModel. No live vector DB
// or embeddings service is contacted — assertions are on in-process
// pipeline state. Unit-level per CONST-030. Demoted from tests/integration/
// to tests/unit/rag/ (PR14) and renamed package integration -> rag_test.
package rag_test

import (
	"context"
	"testing"
	"time"

	"dev.helix.agent/internal/embeddings/models"
	"dev.helix.agent/internal/rag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockEmbeddingModel implements models.EmbeddingModel for testing
type MockEmbeddingModel struct {
	dim int
}

func (m *MockEmbeddingModel) Encode(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		embedding := make([]float32, m.dim)
		for j := range embedding {
			embedding[j] = float32(i+1) * 0.1 / float32(j+1)
		}
		result[i] = embedding
	}
	return result, nil
}

func (m *MockEmbeddingModel) EncodeSingle(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := m.Encode(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

func (m *MockEmbeddingModel) Dimensions() int {
	return m.dim
}

func (m *MockEmbeddingModel) Name() string {
	return "mock-embedding"
}

func (m *MockEmbeddingModel) MaxTokens() int {
	return 8192
}

func (m *MockEmbeddingModel) Provider() string {
	return "mock"
}

func (m *MockEmbeddingModel) Health(ctx context.Context) error {
	return nil
}

func (m *MockEmbeddingModel) Close() error {
	return nil
}

func createTestEmbeddingRegistry() *models.EmbeddingModelRegistry {
	config := models.RegistryConfig{
		FallbackChain: []string{"mock"},
	}
	registry := models.NewEmbeddingModelRegistry(config)
	registry.Register("mock", &MockEmbeddingModel{dim: 384})
	return registry
}

// TestRAGPipeline_Integration tests the RAG pipeline with all components
func TestRAGPipeline_Integration(t *testing.T) {
	// Integration test — no external deps required

	_ = context.Background() // Available for future use
	registry := createTestEmbeddingRegistry()

	t.Run("PipelineCreation", func(t *testing.T) {
		config := rag.PipelineConfig{
			VectorDBType:   rag.VectorDBChroma,
			CollectionName: "test_collection",
			EmbeddingModel: "mock",
			ChunkingConfig: rag.DefaultChunkingConfig(),
		}

		pipeline := rag.NewPipeline(config, registry)
		require.NotNil(t, pipeline)
	})

	t.Run("DocumentChunking", func(t *testing.T) {
		config := rag.PipelineConfig{
			VectorDBType:   rag.VectorDBChroma,
			CollectionName: "test_collection",
			EmbeddingModel: "mock",
			ChunkingConfig: rag.ChunkingConfig{
				ChunkSize:    100,
				ChunkOverlap: 20,
				Separator:    "\n\n",
			},
		}

		pipeline := rag.NewPipeline(config, registry)

		doc := &rag.PipelineDocument{
			ID:      "doc1",
			Content: "This is the first paragraph.\n\nThis is the second paragraph.\n\nThis is the third paragraph.",
			Metadata: map[string]interface{}{
				"source": "test",
			},
			Source: "test_source",
		}

		chunks := pipeline.ChunkDocument(doc)
		assert.NotEmpty(t, chunks)
		assert.True(t, len(chunks) >= 1)

		// Verify chunk metadata
		for _, chunk := range chunks {
			assert.Equal(t, "doc1", chunk.DocID)
			assert.NotEmpty(t, chunk.ID)
			assert.NotEmpty(t, chunk.Content)
		}
	})
}

// TestAdvancedRAG_Integration tests advanced RAG techniques
func TestAdvancedRAG_Integration(t *testing.T) {
	// Integration test — no external deps required

	ctx := context.Background()
	registry := createTestEmbeddingRegistry()

	config := rag.PipelineConfig{
		VectorDBType:   rag.VectorDBChroma,
		CollectionName: "advanced_test",
		EmbeddingModel: "mock",
		ChunkingConfig: rag.DefaultChunkingConfig(),
	}

	pipeline := rag.NewPipeline(config, registry)

	advancedConfig := rag.DefaultAdvancedRAGConfig()
	advancedConfig.ReRanker.ScoreThreshold = 0.0 // Accept all scores for testing
	advancedRAG := rag.NewAdvancedRAG(advancedConfig, pipeline)

	err := advancedRAG.Initialize(ctx)
	require.NoError(t, err)

	t.Run("QueryExpansion", func(t *testing.T) {
		expansions := advancedRAG.ExpandQuery(ctx, "create function")

		assert.NotEmpty(t, expansions)
		assert.Equal(t, "create function", expansions[0].Query)
		assert.Equal(t, "original", expansions[0].Type)

		// Should have synonym expansions
		hasSynonymExpansion := false
		for _, exp := range expansions {
			if exp.Type == "synonym" {
				hasSynonymExpansion = true
				break
			}
		}
		assert.True(t, hasSynonymExpansion, "Should have synonym expansions")
	})

	t.Run("ReRanking", func(t *testing.T) {
		results := []rag.PipelineSearchResult{
			{
				Chunk: rag.PipelineChunk{ID: "1", Content: "Functions are important for code organization."},
				Score: 0.8,
			},
			{
				Chunk: rag.PipelineChunk{ID: "2", Content: "Random text about weather."},
				Score: 0.9,
			},
			{
				Chunk: rag.PipelineChunk{ID: "3", Content: "Creating functions in Go is straightforward."},
				Score: 0.7,
			},
		}

		reranked, err := advancedRAG.ReRank(ctx, "creating functions", results)
		require.NoError(t, err)
		assert.NotEmpty(t, reranked)

		// Check that positions are assigned
		for _, r := range reranked {
			assert.Greater(t, r.ReRankPosition, 0)
		}
	})

	t.Run("ContextCompression", func(t *testing.T) {
		results := []rag.PipelineSearchResult{
			{
				Chunk: rag.PipelineChunk{
					ID:      "1",
					Content: "This is a long document about creating functions. Functions help organize code. They make code reusable.",
				},
				Score: 0.9,
			},
			{
				Chunk: rag.PipelineChunk{
					ID:      "2",
					Content: "Another document about database queries. Queries retrieve data efficiently from databases.",
				},
				Score: 0.8,
			},
		}

		compressed, err := advancedRAG.CompressContext(ctx, "creating functions", results)
		require.NoError(t, err)
		assert.NotNil(t, compressed)
		assert.Greater(t, compressed.OriginalLength, 0)
		assert.LessOrEqual(t, compressed.CompressionRatio, 1.0)
	})
}

// TestEmbeddingModelRegistry_Integration tests embedding model registry
func TestEmbeddingModelRegistry_Integration(t *testing.T) {
	// Integration test — no external deps required

	ctx := context.Background()

	t.Run("RegistryCreation", func(t *testing.T) {
		config := models.RegistryConfig{
			FallbackChain: []string{"mock1", "mock2"},
		}
		registry := models.NewEmbeddingModelRegistry(config)
		require.NotNil(t, registry)
	})

	t.Run("ModelRegistration", func(t *testing.T) {
		config := models.RegistryConfig{
			FallbackChain: []string{"mock"},
		}
		registry := models.NewEmbeddingModelRegistry(config)

		mock := &MockEmbeddingModel{dim: 768}
		registry.Register("mock", mock)

		model, err := registry.Get("mock")
		require.NoError(t, err)
		assert.NotNil(t, model)
		assert.Equal(t, 768, model.Dimensions())
	})

	t.Run("FallbackChain", func(t *testing.T) {
		config := models.RegistryConfig{
			FallbackChain: []string{"primary", "secondary"},
		}
		registry := models.NewEmbeddingModelRegistry(config)

		secondary := &MockEmbeddingModel{dim: 512}
		registry.Register("secondary", secondary)

		// Should use secondary since primary is not registered
		embeddings, modelUsed, err := registry.EncodeWithFallback(ctx, []string{"test"})
		require.NoError(t, err)
		assert.Equal(t, "secondary", modelUsed)
		assert.Len(t, embeddings, 1)
		assert.Len(t, embeddings[0], 512)
	})
}

// TestRAGWithAdvancedRAG_EndToEnd tests the complete RAG workflow
func TestRAGWithAdvancedRAG_EndToEnd(t *testing.T) {
	// Integration test — no external deps required

	ctx := context.Background()

	// Create embedding registry
	embeddingConfig := models.RegistryConfig{
		FallbackChain: []string{"mock"},
	}
	embeddingRegistry := models.NewEmbeddingModelRegistry(embeddingConfig)
	embeddingRegistry.Register("mock", &MockEmbeddingModel{dim: 384})

	// Create pipeline config
	pipelineConfig := rag.PipelineConfig{
		VectorDBType:   rag.VectorDBChroma,
		CollectionName: "e2e_test",
		EmbeddingModel: "mock",
		ChunkingConfig: rag.ChunkingConfig{
			ChunkSize:    200,
			ChunkOverlap: 50,
			Separator:    "\n\n",
		},
		EnableCache: true,
		CacheTTL:    5 * time.Minute,
	}

	// Create pipeline
	pipeline := rag.NewPipeline(pipelineConfig, embeddingRegistry)
	require.NotNil(t, pipeline)

	// Create advanced RAG
	advancedConfig := rag.DefaultAdvancedRAGConfig()
	advancedConfig.HybridSearch.VectorWeight = 0.6
	advancedConfig.HybridSearch.KeywordWeight = 0.4
	advancedConfig.ReRanker.ScoreThreshold = 0.0

	advancedRAG := rag.NewAdvancedRAG(advancedConfig, pipeline)
	err := advancedRAG.Initialize(ctx)
	require.NoError(t, err)

	// Test document creation
	doc := &rag.PipelineDocument{
		ID:      "test_doc_1",
		Content: "Go is a statically typed, compiled programming language designed at Google. It is syntactically similar to C but with memory safety and garbage collection.",
		Metadata: map[string]interface{}{
			"language": "english",
			"topic":    "programming",
		},
		Source:    "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Test chunking
	chunks := pipeline.ChunkDocument(doc)
	assert.NotEmpty(t, chunks)

	// Test query expansion
	expansions := advancedRAG.ExpandQuery(ctx, "programming language")
	assert.NotEmpty(t, expansions)

	// Test re-ranking with mock results
	mockResults := []rag.PipelineSearchResult{
		{
			Chunk: rag.PipelineChunk{
				ID:      "chunk1",
				DocID:   "doc1",
				Content: "Go programming language is efficient and easy to learn. It supports concurrent programming with goroutines. The language was created at Google.",
			},
			Score: 0.85,
		},
		{
			Chunk: rag.PipelineChunk{
				ID:      "chunk2",
				DocID:   "doc2",
				Content: "Unrelated content about cooking recipes. This document has nothing to do with programming languages. It covers different types of cuisine.",
			},
			Score: 0.90,
		},
	}

	reranked, err := advancedRAG.ReRank(ctx, "programming language", mockResults)
	require.NoError(t, err)
	assert.NotEmpty(t, reranked)

	// The chunk about programming should be ranked higher after re-ranking
	assert.Equal(t, "chunk1", reranked[0].Chunk.ID)

	// Test compression
	compressed, err := advancedRAG.CompressContext(ctx, "programming language", mockResults)
	require.NoError(t, err)
	assert.NotEmpty(t, compressed.Content)
}

// TestConcurrentRAGOperations tests thread safety
func TestConcurrentRAGOperations(t *testing.T) {
	// Integration test — no external deps required

	ctx := context.Background()
	registry := createTestEmbeddingRegistry()

	config := rag.PipelineConfig{
		VectorDBType:   rag.VectorDBChroma,
		CollectionName: "concurrent_test",
		EmbeddingModel: "mock",
		ChunkingConfig: rag.DefaultChunkingConfig(),
	}

	pipeline := rag.NewPipeline(config, registry)

	advancedConfig := rag.DefaultAdvancedRAGConfig()
	advancedConfig.ReRanker.ScoreThreshold = 0.0
	advancedRAG := rag.NewAdvancedRAG(advancedConfig, pipeline)
	err := advancedRAG.Initialize(ctx)
	require.NoError(t, err)

	// Run concurrent operations
	done := make(chan bool)
	errs := make(chan error, 10)

	for i := 0; i < 5; i++ {
		go func(idx int) {
			defer func() { done <- true }()

			// Query expansion
			expansions := advancedRAG.ExpandQuery(ctx, "test query")
			if len(expansions) == 0 {
				errs <- assert.AnError
				return
			}

			// Re-ranking
			results := []rag.PipelineSearchResult{
				{Chunk: rag.PipelineChunk{ID: "1", Content: "test content"}, Score: 0.8},
			}
			_, err := advancedRAG.ReRank(ctx, "test", results)
			if err != nil {
				errs <- err
				return
			}

			// Compression
			_, err = advancedRAG.CompressContext(ctx, "test", results)
			if err != nil {
				errs <- err
				return
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	// Check for errors
	close(errs)
	for err := range errs {
		t.Errorf("Concurrent operation failed: %v", err)
	}
}
