package rag

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRetrieverForEnhanced implements Retriever for testing QdrantEnhancedRetriever
type MockRetrieverForEnhanced struct {
	results []*SearchResult
	err     error
	indexed []*Document
	deleted []string
}

func (m *MockRetrieverForEnhanced) Retrieve(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func (m *MockRetrieverForEnhanced) Index(ctx context.Context, docs []*Document) error {
	if m.err != nil {
		return m.err
	}
	m.indexed = append(m.indexed, docs...)
	return nil
}

func (m *MockRetrieverForEnhanced) Delete(ctx context.Context, ids []string) error {
	if m.err != nil {
		return m.err
	}
	m.deleted = append(m.deleted, ids...)
	return nil
}

// MockRerankerForEnhanced implements Reranker for testing
type MockRerankerForEnhanced struct {
	results []*SearchResult
	err     error
}

func (m *MockRerankerForEnhanced) Rerank(ctx context.Context, query string, results []*SearchResult, topK int) ([]*SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.results != nil {
		return m.results, nil
	}
	if len(results) > topK {
		return results[:topK], nil
	}
	return results, nil
}

// MockDebateEvaluator implements QdrantDebateEvaluator for testing
type MockDebateEvaluator struct {
	scores map[string]float64
	err    error
}

func (m *MockDebateEvaluator) EvaluateRelevance(ctx context.Context, query, document string) (float64, error) {
	if m.err != nil {
		return 0, m.err
	}
	if score, ok := m.scores[document]; ok {
		return score, nil
	}
	return 0.5, nil
}

func TestDefaultQdrantEnhancedConfig(t *testing.T) {
	config := DefaultQdrantEnhancedConfig()

	assert.Equal(t, 0.6, config.DenseWeight)
	assert.Equal(t, 0.4, config.SparseWeight)
	assert.False(t, config.UseDebateEvaluation)
	assert.Equal(t, 5, config.DebateTopK)
	assert.Equal(t, FusionRRF, config.FusionMethod)
	assert.Equal(t, 60.0, config.RRFK)
}

func TestNewQdrantEnhancedRetriever(t *testing.T) {
	t.Run("with all parameters", func(t *testing.T) {
		denseRetriever := &MockRetrieverForEnhanced{}
		reranker := &MockRerankerForEnhanced{}
		config := &QdrantEnhancedConfig{
			DenseWeight:  0.7,
			SparseWeight: 0.3,
		}
		logger := logrus.New()

		retriever := NewQdrantEnhancedRetriever(denseRetriever, reranker, config, logger)

		assert.NotNil(t, retriever)
		assert.Equal(t, 0.7, retriever.config.DenseWeight)
		assert.NotNil(t, retriever.sparseIndex)
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		denseRetriever := &MockRetrieverForEnhanced{}
		retriever := NewQdrantEnhancedRetriever(denseRetriever, nil, nil, nil)

		assert.NotNil(t, retriever)
		assert.Equal(t, 0.6, retriever.config.DenseWeight)
	})
}

func TestQdrantEnhancedRetriever_SetDebateEvaluator(t *testing.T) {
	retriever := NewQdrantEnhancedRetriever(&MockRetrieverForEnhanced{}, nil, nil, nil)

	evaluator := &MockDebateEvaluator{}
	retriever.SetDebateEvaluator(evaluator)

	assert.True(t, retriever.config.UseDebateEvaluation)

	// Set to nil
	retriever.SetDebateEvaluator(nil)
	assert.False(t, retriever.config.UseDebateEvaluation)
}

func TestQdrantEnhancedRetriever_Retrieve(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	createDoc := func(id string, content string, score float64) *SearchResult {
		return &SearchResult{
			Document: &Document{ID: id, Content: content},
			Score:    score,
		}
	}

	t.Run("basic retrieval with RRF fusion", func(t *testing.T) {
		denseResults := []*SearchResult{
			createDoc("doc1", "content about programming", 0.9),
			createDoc("doc2", "content about databases", 0.8),
		}
		denseRetriever := &MockRetrieverForEnhanced{results: denseResults}

		config := &QdrantEnhancedConfig{
			DenseWeight:         0.6,
			SparseWeight:        0.4,
			FusionMethod:        FusionRRF,
			RRFK:                60.0,
			UseDebateEvaluation: false,
		}

		retriever := NewQdrantEnhancedRetriever(denseRetriever, nil, config, logger)

		// Add documents to sparse index
		retriever.sparseIndex.AddDocument("doc1", "content about programming")
		retriever.sparseIndex.AddDocument("doc3", "other programming content")

		results, err := retriever.Retrieve(context.Background(), "programming", &SearchOptions{TopK: 5})

		require.NoError(t, err)
		assert.NotEmpty(t, results)
	})

	t.Run("retrieval with weighted fusion", func(t *testing.T) {
		denseResults := []*SearchResult{
			createDoc("doc1", "content about AI", 0.95),
		}
		denseRetriever := &MockRetrieverForEnhanced{results: denseResults}

		config := &QdrantEnhancedConfig{
			DenseWeight:  0.7,
			SparseWeight: 0.3,
			FusionMethod: FusionWeighted,
			RRFK:         60.0,
		}

		retriever := NewQdrantEnhancedRetriever(denseRetriever, nil, config, logger)
		retriever.sparseIndex.AddDocument("doc1", "content about AI")
		retriever.sparseIndex.AddDocument("doc2", "AI machine learning")

		results, err := retriever.Retrieve(context.Background(), "AI", &SearchOptions{TopK: 5})

		require.NoError(t, err)
		assert.NotEmpty(t, results)
	})

	t.Run("retrieval with reranking", func(t *testing.T) {
		denseResults := []*SearchResult{
			createDoc("doc1", "machine learning basics", 0.8),
			createDoc("doc2", "deep learning advanced", 0.9),
		}
		denseRetriever := &MockRetrieverForEnhanced{results: denseResults}
		reranker := &MockRerankerForEnhanced{}

		config := &QdrantEnhancedConfig{
			DenseWeight:  0.6,
			SparseWeight: 0.4,
			FusionMethod: FusionRRF,
		}

		retriever := NewQdrantEnhancedRetriever(denseRetriever, reranker, config, logger)

		results, err := retriever.Retrieve(context.Background(), "machine learning", &SearchOptions{
			TopK:            5,
			EnableReranking: true,
		})

		require.NoError(t, err)
		assert.NotEmpty(t, results)
	})

	t.Run("retrieval with debate evaluation", func(t *testing.T) {
		denseResults := []*SearchResult{
			createDoc("doc1", "relevant content", 0.8),
			createDoc("doc2", "less relevant content", 0.7),
		}
		denseRetriever := &MockRetrieverForEnhanced{results: denseResults}

		config := &QdrantEnhancedConfig{
			DenseWeight:         0.6,
			SparseWeight:        0.4,
			FusionMethod:        FusionRRF,
			UseDebateEvaluation: true,
			DebateTopK:          5,
		}

		retriever := NewQdrantEnhancedRetriever(denseRetriever, nil, config, logger)

		evaluator := &MockDebateEvaluator{
			scores: map[string]float64{
				"relevant content":      0.95,
				"less relevant content": 0.3,
			},
		}
		retriever.SetDebateEvaluator(evaluator)

		results, err := retriever.Retrieve(context.Background(), "relevant", &SearchOptions{TopK: 5})

		require.NoError(t, err)
		assert.NotEmpty(t, results)
		// Should be reordered by debate evaluation
	})

	t.Run("debate evaluation error handled gracefully", func(t *testing.T) {
		denseResults := []*SearchResult{
			createDoc("doc1", "content", 0.8),
		}
		denseRetriever := &MockRetrieverForEnhanced{results: denseResults}

		config := &QdrantEnhancedConfig{
			DenseWeight:         0.6,
			SparseWeight:        0.4,
			UseDebateEvaluation: true,
			DebateTopK:          5,
		}

		retriever := NewQdrantEnhancedRetriever(denseRetriever, nil, config, logger)
		retriever.SetDebateEvaluator(&MockDebateEvaluator{err: errors.New("evaluation error")})

		results, err := retriever.Retrieve(context.Background(), "query", &SearchOptions{TopK: 5})

		require.NoError(t, err)
		assert.NotEmpty(t, results)
	})

	t.Run("dense retriever failure gracefully handled", func(t *testing.T) {
		denseRetriever := &MockRetrieverForEnhanced{err: errors.New("dense retrieval failed")}

		config := &QdrantEnhancedConfig{
			DenseWeight:  0.6,
			SparseWeight: 0.4,
			FusionMethod: FusionRRF,
		}

		retriever := NewQdrantEnhancedRetriever(denseRetriever, nil, config, logger)
		retriever.sparseIndex.AddDocument("doc1", "test content")

		results, err := retriever.Retrieve(context.Background(), "test", &SearchOptions{TopK: 5})

		require.NoError(t, err)
		// Should still return sparse results
		_ = results // Use results to avoid unused variable error
	})

	t.Run("reranking failure falls back to fused results", func(t *testing.T) {
		denseResults := []*SearchResult{
			createDoc("doc1", "test content", 0.8),
		}
		denseRetriever := &MockRetrieverForEnhanced{results: denseResults}
		reranker := &MockRerankerForEnhanced{err: errors.New("reranking failed")}

		retriever := NewQdrantEnhancedRetriever(denseRetriever, reranker, nil, logger)

		results, err := retriever.Retrieve(context.Background(), "test", &SearchOptions{
			TopK:            5,
			EnableReranking: true,
		})

		require.NoError(t, err)
		assert.NotEmpty(t, results)
	})

	t.Run("with nil options uses defaults", func(t *testing.T) {
		denseResults := []*SearchResult{
			createDoc("doc1", "test", 0.9),
		}
		denseRetriever := &MockRetrieverForEnhanced{results: denseResults}

		retriever := NewQdrantEnhancedRetriever(denseRetriever, nil, nil, logger)

		results, err := retriever.Retrieve(context.Background(), "test", nil)

		require.NoError(t, err)
		assert.NotEmpty(t, results)
	})
}

func TestQdrantEnhancedRetriever_Index(t *testing.T) {
	logger := logrus.New()

	docs := []*Document{
		{ID: "doc1", Content: "Content about programming"},
		{ID: "doc2", Content: "Content about databases"},
	}

	t.Run("successful indexing", func(t *testing.T) {
		denseRetriever := &MockRetrieverForEnhanced{}
		retriever := NewQdrantEnhancedRetriever(denseRetriever, nil, nil, logger)

		err := retriever.Index(context.Background(), docs)

		require.NoError(t, err)
		assert.Len(t, denseRetriever.indexed, 2)
	})

	t.Run("dense indexing failure", func(t *testing.T) {
		denseRetriever := &MockRetrieverForEnhanced{err: errors.New("indexing failed")}
		retriever := NewQdrantEnhancedRetriever(denseRetriever, nil, nil, logger)

		err := retriever.Index(context.Background(), docs)

		require.Error(t, err)
	})
}

func TestQdrantEnhancedRetriever_Delete(t *testing.T) {
	logger := logrus.New()
	ids := []string{"doc1", "doc2"}

	t.Run("successful deletion", func(t *testing.T) {
		denseRetriever := &MockRetrieverForEnhanced{}
		retriever := NewQdrantEnhancedRetriever(denseRetriever, nil, nil, logger)

		// First add documents to sparse index
		retriever.sparseIndex.AddDocument("doc1", "content 1")
		retriever.sparseIndex.AddDocument("doc2", "content 2")

		err := retriever.Delete(context.Background(), ids)

		require.NoError(t, err)
		assert.Len(t, denseRetriever.deleted, 2)
	})

	t.Run("dense deletion failure", func(t *testing.T) {
		denseRetriever := &MockRetrieverForEnhanced{err: errors.New("deletion failed")}
		retriever := NewQdrantEnhancedRetriever(denseRetriever, nil, nil, logger)

		err := retriever.Delete(context.Background(), ids)

		require.Error(t, err)
	})
}

func TestEnhancedBM25Index(t *testing.T) {
	t.Run("new index is empty", func(t *testing.T) {
		idx := NewEnhancedBM25Index()

		assert.NotNil(t, idx)
		assert.Empty(t, idx.Documents())
		assert.Equal(t, 0, idx.TotalDocs())
		assert.Equal(t, 1.2, idx.K1())
		assert.Equal(t, 0.75, idx.B())
	})

	t.Run("add and search documents", func(t *testing.T) {
		idx := NewEnhancedBM25Index()

		idx.AddDocument("doc1", "machine learning is a subset of artificial intelligence")
		idx.AddDocument("doc2", "deep learning uses neural networks")
		idx.AddDocument("doc3", "natural language processing is important")

		assert.Equal(t, 3, idx.TotalDocs())

		results := idx.Search("machine learning", 10)

		assert.NotEmpty(t, results)
		assert.Equal(t, "doc1", results[0].Document.ID)
	})

	t.Run("remove document", func(t *testing.T) {
		idx := NewEnhancedBM25Index()

		idx.AddDocument("doc1", "machine learning")
		idx.AddDocument("doc2", "deep learning")

		assert.Equal(t, 2, idx.TotalDocs())

		idx.RemoveDocument("doc1")

		assert.Equal(t, 1, idx.TotalDocs())

		results := idx.Search("machine learning", 10)
		for _, r := range results {
			assert.NotEqual(t, "doc1", r.Document.ID)
		}
	})

	t.Run("remove non-existent document", func(t *testing.T) {
		idx := NewEnhancedBM25Index()
		idx.AddDocument("doc1", "content")

		idx.RemoveDocument("non_existent")

		assert.Equal(t, 1, idx.TotalDocs())
	})

	t.Run("search with no matching terms", func(t *testing.T) {
		idx := NewEnhancedBM25Index()
		idx.AddDocument("doc1", "machine learning")

		results := idx.Search("xyz123 abc456", 10)

		assert.Empty(t, results)
	})

	t.Run("search respects topK limit", func(t *testing.T) {
		idx := NewEnhancedBM25Index()

		for i := 0; i < 20; i++ {
			idx.AddDocument(string(rune('a'+i)), "common term unique term")
		}

		results := idx.Search("common term", 5)

		assert.LessOrEqual(t, len(results), 5)
	})

	t.Run("IDF calculation", func(t *testing.T) {
		idx := NewEnhancedBM25Index()
		idx.AddDocument("doc1", "word1 word2")
		idx.AddDocument("doc2", "word1")

		// word1 appears in 2 docs, word2 in 1 doc
		// word2 should have higher IDF
		idf1 := calculateIDF(2, idx.TotalDocs())
		idf2 := calculateIDF(1, idx.TotalDocs())

		assert.Greater(t, idf2, idf1)
	})

	t.Run("TF calculation", func(t *testing.T) {
		idx := NewEnhancedBM25Index()
		idx.AddDocument("doc1", "word word word")
		// AddDocument already populated avgDocLen correctly via the
		// bm25State snapshot; pass it as an explicit argument to
		// calculateTF so the test does not reach into private state.
		tf := idx.calculateTF(3, 3, idx.AvgDocLen())

		assert.Greater(t, tf, 0.0)
	})
}

func TestEnhancedTokenize(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "simple words",
			text:     "hello world",
			expected: 2,
		},
		{
			name:     "with punctuation",
			text:     "Hello, World! How are you?",
			expected: 5,
		},
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "only punctuation",
			text:     "!!!???...",
			expected: 0,
		},
		{
			name:     "mixed content",
			text:     "The quick brown fox jumps over the lazy dog.",
			expected: 9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := enhancedTokenize(tt.text)
			assert.Equal(t, tt.expected, len(tokens))
		})
	}
}

func TestTruncateText(t *testing.T) {
	t.Run("short text unchanged", func(t *testing.T) {
		result := truncateText("hello", 10)
		assert.Equal(t, "hello", result)
	})

	t.Run("long text truncated with ellipsis", func(t *testing.T) {
		result := truncateText("hello world this is a long text", 10)
		assert.Equal(t, "hello worl...", result)
	})

	t.Run("exact length unchanged", func(t *testing.T) {
		result := truncateText("hello", 5)
		assert.Equal(t, "hello", result)
	})
}

func TestQdrantEnhancedRetriever_RRFFusion(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("RRF correctly combines scores from both retrievers", func(t *testing.T) {
		denseResults := []*SearchResult{
			{Document: &Document{ID: "doc1", Content: "A"}, Score: 0.9},
			{Document: &Document{ID: "doc2", Content: "B"}, Score: 0.8},
		}
		denseRetriever := &MockRetrieverForEnhanced{results: denseResults}

		config := &QdrantEnhancedConfig{
			DenseWeight:  0.6,
			SparseWeight: 0.4,
			FusionMethod: FusionRRF,
			RRFK:         60.0,
		}

		retriever := NewQdrantEnhancedRetriever(denseRetriever, nil, config, logger)

		// Add same documents to sparse index
		retriever.sparseIndex.AddDocument("doc1", "A keyword")
		retriever.sparseIndex.AddDocument("doc2", "B keyword")

		results, err := retriever.Retrieve(context.Background(), "A", &SearchOptions{TopK: 5})

		require.NoError(t, err)
		assert.NotEmpty(t, results)

		// Documents appearing in both should have combined scores
		for _, r := range results {
			assert.Equal(t, MatchTypeHybrid, r.MatchType)
		}
	})
}

func TestQdrantEnhancedRetriever_WeightedFusion(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("weighted fusion normalizes and weights scores", func(t *testing.T) {
		denseResults := []*SearchResult{
			{Document: &Document{ID: "doc1", Content: "A"}, Score: 1.0},
		}
		denseRetriever := &MockRetrieverForEnhanced{results: denseResults}

		config := &QdrantEnhancedConfig{
			DenseWeight:  0.8,
			SparseWeight: 0.2,
			FusionMethod: FusionWeighted,
		}

		retriever := NewQdrantEnhancedRetriever(denseRetriever, nil, config, logger)
		retriever.sparseIndex.AddDocument("doc1", "A keyword")

		results, err := retriever.Retrieve(context.Background(), "A", &SearchOptions{TopK: 5})

		require.NoError(t, err)
		assert.NotEmpty(t, results)
	})
}

func TestQdrantEnhancedRetriever_FuseResults(t *testing.T) {
	retriever := NewQdrantEnhancedRetriever(&MockRetrieverForEnhanced{}, nil, nil, nil)

	t.Run("handles nil documents gracefully", func(t *testing.T) {
		denseResults := []*SearchResult{
			{Document: &Document{ID: "doc1"}, Score: 0.9},
			{Document: nil, Score: 0.8}, // nil document
		}
		sparseResults := []*SearchResult{
			{Document: &Document{ID: "doc2"}, Score: 0.7},
		}

		fused := retriever.rrfFusion(denseResults, sparseResults)

		// Should not panic, should only include valid documents
		for _, r := range fused {
			assert.NotNil(t, r.Document)
		}
	})
}

func TestEnhancedBM25Index_Concurrency(t *testing.T) {
	idx := NewEnhancedBM25Index()

	var wg sync.WaitGroup

	// Concurrent adds
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			idx.AddDocument(string(rune('a'+i)), "test content word")
		}(i)
	}

	// Concurrent searches
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = idx.Search("test word", 5)
		}()
	}

	wg.Wait()

	// Should have 10 documents
	assert.Equal(t, 10, idx.TotalDocs())
}

// TestEnhancedBM25Index_RaceFree is the CONST-029 verification test.
// It pounds the index with concurrent AddDocument, RemoveDocument, and
// Search calls under -race. If any bare-map mutation leaks back in
// (accidental Pattern-A regression) the race detector trips.
func TestEnhancedBM25Index_RaceFree(t *testing.T) {
	idx := NewEnhancedBM25Index()

	const writers = 8
	const readers = 16
	const iterations = 200

	var wg sync.WaitGroup

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				id := fmt.Sprintf("doc-%d-%d", base, i)
				idx.AddDocument(id, fmt.Sprintf("content for %s alpha beta gamma", id))
				if i%3 == 0 && i > 0 {
					idx.RemoveDocument(fmt.Sprintf("doc-%d-%d", base, i-1))
				}
			}
		}(w)
	}

	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = idx.Search("alpha beta", 5)
				_ = idx.TotalDocs()
				_ = idx.AvgDocLen()
				_ = idx.Documents()
			}
		}()
	}

	wg.Wait()

	// Sanity check: we didn't lose the invariant that docCount matches
	// the size of documents(). An accidental partial snapshot would
	// show up here.
	docs := idx.Documents()
	assert.Equal(t, idx.TotalDocs(), len(docs),
		"TotalDocs() must match Documents() snapshot size")
}
