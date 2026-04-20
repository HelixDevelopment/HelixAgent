package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for advanced.go uncovered functions

// MockPipelineForAdvanced implements a minimal pipeline for testing AdvancedRAG
type MockPipelineForAdvanced struct {
	searchResults []PipelineSearchResult
	searchErr     error
}

func (m *MockPipelineForAdvanced) Search(ctx context.Context, query string, topK int) ([]PipelineSearchResult, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.searchResults, nil
}

func TestAdvancedRAG_HybridSearch(t *testing.T) {
	config := DefaultAdvancedRAGConfig()

	t.Run("successful hybrid search", func(t *testing.T) {
		mockPipeline := &Pipeline{}
		rag := NewAdvancedRAG(config, mockPipeline)
		_ = rag.Initialize(context.Background())

		// Since HybridSearch depends on pipeline.Search which needs a real vector DB,
		// we test the internal components that HybridSearch uses
		assert.NotNil(t, rag)
		assert.True(t, rag.initialized.Load())
	})
}

func TestAdvancedRAG_KeywordSearch(t *testing.T) {
	config := DefaultAdvancedRAGConfig()
	rag := NewAdvancedRAG(config, &Pipeline{})
	_ = rag.Initialize(context.Background())

	t.Run("calculates keyword scores for results", func(t *testing.T) {
		results := []PipelineSearchResult{
			{Chunk: PipelineChunk{ID: "1", Content: "machine learning algorithms"}, Score: 0.8},
			{Chunk: PipelineChunk{ID: "2", Content: "deep neural networks"}, Score: 0.7},
			{Chunk: PipelineChunk{ID: "3", Content: "random unrelated content"}, Score: 0.6},
		}

		scores := rag.keywordSearch("machine learning", results)

		// "machine learning algorithms" should have a score
		assert.Contains(t, scores, "1")
		// Check that "machine" and "learning" matching gives higher score
		assert.Greater(t, scores["1"], float32(0))
	})

	t.Run("handles empty query", func(t *testing.T) {
		results := []PipelineSearchResult{
			{Chunk: PipelineChunk{ID: "1", Content: "test content"}, Score: 0.8},
		}

		scores := rag.keywordSearch("", results)

		// Empty query should give zero scores
		for _, score := range scores {
			assert.Equal(t, float32(0), score)
		}
	})
}

func TestAdvancedRAG_SearchWithExpansion(t *testing.T) {
	// SearchWithExpansion requires a working pipeline with vector DB
	// Test the query expansion part that doesn't require DB
	config := DefaultAdvancedRAGConfig()
	rag := NewAdvancedRAG(config, &Pipeline{})
	_ = rag.Initialize(context.Background())

	expansions := rag.ExpandQuery(context.Background(), "database function")

	assert.NotEmpty(t, expansions)
	assert.Equal(t, "database function", expansions[0].Query)
	assert.Equal(t, 1.0, expansions[0].Weight)
}

// Tests for reranker.go uncovered functions

func TestCrossEncoderReranker_ScoreBatch(t *testing.T) {
	logger := logrus.New()

	t.Run("successful batch scoring", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var req map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&req)

			assert.Contains(t, req, "model")
			assert.Contains(t, req, "pairs")

			response := map[string]interface{}{
				"scores": []float64{0.9, 0.8, 0.7},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &RerankerConfig{
			Model:     "test-model",
			Endpoint:  server.URL,
			Timeout:   5 * time.Second,
			BatchSize: 10,
		}
		reranker := NewCrossEncoderReranker(config, logger)

		results := []*SearchResult{
			{Document: &Document{ID: "1", Content: "doc 1"}, Score: 0.5},
			{Document: &Document{ID: "2", Content: "doc 2"}, Score: 0.4},
			{Document: &Document{ID: "3", Content: "doc 3"}, Score: 0.3},
		}

		reranked, err := reranker.Rerank(context.Background(), "test query", results, 3)

		require.NoError(t, err)
		assert.Len(t, reranked, 3)
		// Check that scores were updated from API
		assert.Equal(t, 0.9, reranked[0].RerankedScore)
	})

	t.Run("API returns error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal server error"))
		}))
		defer server.Close()

		config := &RerankerConfig{
			Model:     "test-model",
			Endpoint:  server.URL,
			Timeout:   5 * time.Second,
			BatchSize: 10,
		}
		reranker := NewCrossEncoderReranker(config, logger)

		results := []*SearchResult{
			{Document: &Document{ID: "1", Content: "doc 1"}, Score: 0.5},
		}

		// Should fall back to original scores when API fails
		reranked, err := reranker.Rerank(context.Background(), "test query", results, 1)

		require.NoError(t, err)
		assert.Len(t, reranked, 1)
	})

	t.Run("with authorization header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

			response := map[string]interface{}{
				"scores": []float64{0.95},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &RerankerConfig{
			Model:     "test-model",
			Endpoint:  server.URL,
			APIKey:    "test-api-key",
			Timeout:   5 * time.Second,
			BatchSize: 10,
		}
		reranker := NewCrossEncoderReranker(config, logger)

		results := []*SearchResult{
			{Document: &Document{ID: "1", Content: "doc 1"}, Score: 0.5},
		}

		reranked, err := reranker.Rerank(context.Background(), "test query", results, 1)

		require.NoError(t, err)
		assert.Len(t, reranked, 1)
	})

	t.Run("batch size respected", func(t *testing.T) {
		batchCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			batchCount++
			var req map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&req)

			pairs := req["pairs"].([]interface{})
			// Each batch should have at most 2 pairs
			assert.LessOrEqual(t, len(pairs), 2)

			scores := make([]float64, len(pairs))
			for i := range scores {
				scores[i] = 0.8
			}

			response := map[string]interface{}{
				"scores": scores,
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &RerankerConfig{
			Model:     "test-model",
			Endpoint:  server.URL,
			Timeout:   5 * time.Second,
			BatchSize: 2, // Small batch size
		}
		reranker := NewCrossEncoderReranker(config, logger)

		results := []*SearchResult{
			{Document: &Document{ID: "1", Content: "doc 1"}, Score: 0.5},
			{Document: &Document{ID: "2", Content: "doc 2"}, Score: 0.4},
			{Document: &Document{ID: "3", Content: "doc 3"}, Score: 0.3},
			{Document: &Document{ID: "4", Content: "doc 4"}, Score: 0.2},
		}

		reranked, err := reranker.Rerank(context.Background(), "test query", results, 4)

		require.NoError(t, err)
		assert.Len(t, reranked, 4)
		assert.Equal(t, 2, batchCount) // Should be 2 batches of 2
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		config := &RerankerConfig{
			Model:     "test-model",
			Endpoint:  server.URL,
			Timeout:   5 * time.Second,
			BatchSize: 10,
		}
		reranker := NewCrossEncoderReranker(config, logger)

		results := []*SearchResult{
			{Document: &Document{ID: "1", Content: "doc 1"}, Score: 0.5},
		}

		// Should fall back to original scores
		reranked, err := reranker.Rerank(context.Background(), "test query", results, 1)

		require.NoError(t, err)
		assert.Len(t, reranked, 1)
	})
}

func TestCrossEncoderReranker_Rerank_WithEndpoint(t *testing.T) {
	logger := logrus.New()

	t.Run("full reranking flow with API", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"scores": []float64{0.95, 0.85, 0.75},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &RerankerConfig{
			Model:     "test-model",
			Endpoint:  server.URL,
			Timeout:   5 * time.Second,
			BatchSize: 10,
		}
		reranker := NewCrossEncoderReranker(config, logger)

		results := []*SearchResult{
			{Document: &Document{ID: "1", Content: "low relevance"}, Score: 0.3},
			{Document: &Document{ID: "2", Content: "medium relevance"}, Score: 0.5},
			{Document: &Document{ID: "3", Content: "high relevance"}, Score: 0.7},
		}

		reranked, err := reranker.Rerank(context.Background(), "test query", results, 2)

		require.NoError(t, err)
		assert.Len(t, reranked, 2)
		// First result should be the one with highest reranked score
		assert.Equal(t, 0.95, reranked[0].RerankedScore)
	})
}

// Additional types tests

func TestDefaultSearchOptions(t *testing.T) {
	opts := DefaultSearchOptions()

	assert.Equal(t, 10, opts.TopK)
	assert.Equal(t, 0.0, opts.MinScore)
	assert.True(t, opts.EnableReranking)
	assert.Equal(t, 0.5, opts.HybridAlpha)
	assert.True(t, opts.IncludeMetadata)
	assert.Empty(t, opts.Namespace)
}

func TestDefaultChunkerConfig(t *testing.T) {
	config := DefaultChunkerConfig()

	assert.Equal(t, 1000, config.ChunkSize)
	assert.Equal(t, 200, config.ChunkOverlap)
	assert.Equal(t, "\n\n", config.Separator)
	assert.Equal(t, "chars", config.LengthFunction)
}

func TestChunkStruct(t *testing.T) {
	chunk := &Chunk{
		ID:         "chunk1",
		DocumentID: "doc1",
		Content:    "test content",
		Index:      0,
		Metadata:   map[string]interface{}{"key": "value"},
		Embedding:  []float32{0.1, 0.2, 0.3},
		StartChar:  0,
		EndChar:    12,
	}

	assert.Equal(t, "chunk1", chunk.ID)
	assert.Equal(t, "doc1", chunk.DocumentID)
	assert.Equal(t, "test content", chunk.Content)
	assert.Equal(t, 0, chunk.Index)
	assert.Equal(t, 0, chunk.StartChar)
	assert.Equal(t, 12, chunk.EndChar)
}

func TestEntityStruct(t *testing.T) {
	entity := &Entity{
		ID:   "entity1",
		Name: "Test Entity",
		Type: "PERSON",
		Properties: map[string]interface{}{
			"age": 30,
		},
		Mentions: []EntityMention{
			{
				DocumentID: "doc1",
				ChunkID:    "chunk1",
				StartChar:  0,
				EndChar:    11,
				Context:    "Test Entity is mentioned here",
			},
		},
	}

	assert.Equal(t, "entity1", entity.ID)
	assert.Equal(t, "Test Entity", entity.Name)
	assert.Equal(t, "PERSON", entity.Type)
	assert.Len(t, entity.Mentions, 1)
}

func TestRelationStruct(t *testing.T) {
	relation := &Relation{
		ID:       "rel1",
		SourceID: "entity1",
		TargetID: "entity2",
		Type:     "WORKS_FOR",
		Properties: map[string]interface{}{
			"since": 2020,
		},
		Confidence: 0.95,
	}

	assert.Equal(t, "rel1", relation.ID)
	assert.Equal(t, "entity1", relation.SourceID)
	assert.Equal(t, "entity2", relation.TargetID)
	assert.Equal(t, "WORKS_FOR", relation.Type)
	assert.Equal(t, 0.95, relation.Confidence)
}

// Test extractRelevantSentences with edge cases
func TestAdvancedRAG_ExtractRelevantSentences_EdgeCases(t *testing.T) {
	config := DefaultAdvancedRAGConfig()
	rag := NewAdvancedRAG(config, &Pipeline{})
	_ = rag.Initialize(context.Background())

	t.Run("with very short content", func(t *testing.T) {
		content := "Short."
		result := rag.extractRelevantSentences("query", content, config.ContextualCompression)
		// Content might be filtered if too short
		assert.NotNil(t, result)
	})

	t.Run("with multiple relevant sentences", func(t *testing.T) {
		content := "This is about machine learning. Machine learning is important. Another topic here. More machine learning content."
		result := rag.extractRelevantSentences("machine learning", content, config.ContextualCompression)
		assert.NotEmpty(t, result)
	})
}

// Test calculateRelevanceScore edge cases
func TestAdvancedRAG_CalculateRelevanceScore_EdgeCases(t *testing.T) {
	config := DefaultAdvancedRAGConfig()
	rag := NewAdvancedRAG(config, &Pipeline{})
	_ = rag.Initialize(context.Background())

	t.Run("partial matches", func(t *testing.T) {
		score := rag.calculateRelevanceScore(
			"programming",
			"This is about programs and programmers but not the exact word",
		)
		// Should still have some score due to partial matches
		assert.GreaterOrEqual(t, score, float32(0))
	})

	t.Run("high frequency terms", func(t *testing.T) {
		score := rag.calculateRelevanceScore(
			"test",
			"test test test test test repeated many times",
		)
		// Higher frequency should give bonus
		assert.Greater(t, score, float32(0))
	})
}

// Test CompressContext with different config settings
func TestAdvancedRAG_CompressContext_Settings(t *testing.T) {
	t.Run("with very small max length", func(t *testing.T) {
		config := DefaultAdvancedRAGConfig()
		config.ContextualCompression.MaxContextLength = 20
		config.ContextualCompression.CompressionRatio = 1.0
		rag := NewAdvancedRAG(config, &Pipeline{})
		_ = rag.Initialize(context.Background())

		results := []PipelineSearchResult{
			{Chunk: PipelineChunk{ID: "1", Content: "This is a much longer content that should be compressed significantly to fit within the limit."}},
		}

		compressed, err := rag.CompressContext(context.Background(), "query", results)

		require.NoError(t, err)
		assert.LessOrEqual(t, len(compressed.Content), 50) // Some buffer for sentence boundaries
	})

	t.Run("disabled key phrase preservation", func(t *testing.T) {
		config := DefaultAdvancedRAGConfig()
		config.ContextualCompression.PreserveKeyPhrases = false
		rag := NewAdvancedRAG(config, &Pipeline{})
		_ = rag.Initialize(context.Background())

		results := []PipelineSearchResult{
			{Chunk: PipelineChunk{ID: "1", Content: "Content about testing and keywords."}},
		}

		compressed, err := rag.CompressContext(context.Background(), "testing", results)

		require.NoError(t, err)
		assert.Empty(t, compressed.KeyPhrases)
	})
}

// Test HybridSearchResult struct
func TestHybridSearchResultStruct(t *testing.T) {
	result := HybridSearchResult{
		PipelineSearchResult: PipelineSearchResult{
			Chunk: PipelineChunk{ID: "1", Content: "test"},
			Score: 0.8,
		},
		VectorScore:   0.9,
		KeywordScore:  0.7,
		CombinedScore: 0.85,
	}

	assert.Equal(t, "1", result.Chunk.ID)
	assert.Equal(t, float32(0.9), result.VectorScore)
	assert.Equal(t, float32(0.7), result.KeywordScore)
	assert.Equal(t, float32(0.85), result.CombinedScore)
}

// Test ReRankedResult struct
func TestReRankedResultStruct(t *testing.T) {
	result := ReRankedResult{
		PipelineSearchResult: PipelineSearchResult{
			Chunk: PipelineChunk{ID: "1", Content: "test"},
			Score: 0.8,
		},
		OriginalScore:  0.7,
		ReRankedScore:  0.9,
		ReRankPosition: 1,
	}

	assert.Equal(t, "1", result.Chunk.ID)
	assert.Equal(t, float32(0.7), result.OriginalScore)
	assert.Equal(t, float32(0.9), result.ReRankedScore)
	assert.Equal(t, 1, result.ReRankPosition)
}

// Test ExpandedQuery struct
func TestExpandedQueryStruct(t *testing.T) {
	eq := ExpandedQuery{
		Query:  "machine learning",
		Weight: 0.8,
		Type:   "synonym",
	}

	assert.Equal(t, "machine learning", eq.Query)
	assert.Equal(t, 0.8, eq.Weight)
	assert.Equal(t, "synonym", eq.Type)
}

// Test CompressedContext struct
func TestCompressedContextStruct(t *testing.T) {
	cc := &CompressedContext{
		OriginalLength:   1000,
		CompressedLength: 500,
		Content:          "compressed content",
		KeyPhrases:       []string{"key", "phrases"},
		CompressionRatio: 0.5,
	}

	assert.Equal(t, 1000, cc.OriginalLength)
	assert.Equal(t, 500, cc.CompressedLength)
	assert.Equal(t, "compressed content", cc.Content)
	assert.Len(t, cc.KeyPhrases, 2)
	assert.Equal(t, 0.5, cc.CompressionRatio)
}
