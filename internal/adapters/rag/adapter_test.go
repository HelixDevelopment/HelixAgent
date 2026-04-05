package rag_test

import (
	"context"
	"testing"

	adapter "dev.helix.agent/internal/adapters/rag"
	helixrag "dev.helix.agent/internal/rag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// ChunkerAdapter Tests
// ============================================================================

func TestNewFixedSizeChunker(t *testing.T) {
	t.Parallel()
	chunker := adapter.NewFixedSizeChunker(100, 10)
	require.NotNil(t, chunker)
}

func TestNewRecursiveChunker(t *testing.T) {
	t.Parallel()
	chunker := adapter.NewRecursiveChunker(200, 20)
	require.NotNil(t, chunker)
}

func TestNewSentenceChunker(t *testing.T) {
	t.Parallel()
	chunker := adapter.NewSentenceChunker(150)
	require.NotNil(t, chunker)
}

func TestFixedSizeChunker_Chunk(t *testing.T) {
	t.Parallel()
	chunker := adapter.NewFixedSizeChunker(10, 0)
	text := "hello world, this is a test of chunking"
	chunks := chunker.Chunk(text)
	assert.NotNil(t, chunks)
	assert.NotEmpty(t, chunks)

	for _, chunk := range chunks {
		assert.NotEmpty(t, chunk.Content)
	}
}

func TestFixedSizeChunker_Chunk_EmptyText(t *testing.T) {
	t.Parallel()
	chunker := adapter.NewFixedSizeChunker(100, 10)
	chunks := chunker.Chunk("")
	assert.NotNil(t, chunks)
}

func TestRecursiveChunker_Chunk(t *testing.T) {
	t.Parallel()
	chunker := adapter.NewRecursiveChunker(50, 5)
	text := "First paragraph.\n\nSecond paragraph with more content.\n\nThird paragraph."
	chunks := chunker.Chunk(text)
	assert.NotNil(t, chunks)
}

func TestSentenceChunker_Chunk(t *testing.T) {
	t.Parallel()
	chunker := adapter.NewSentenceChunker(200)
	text := "First sentence. Second sentence. Third sentence."
	chunks := chunker.Chunk(text)
	assert.NotNil(t, chunks)
}

func TestFixedSizeChunker_ChunkDocument(t *testing.T) {
	t.Parallel()
	chunker := adapter.NewFixedSizeChunker(20, 0)
	doc := &helixrag.PipelineDocument{
		ID:       "doc-001",
		Content:  "This is the content of the test document for chunking purposes.",
		Metadata: map[string]interface{}{"source": "test"},
	}

	chunks := chunker.ChunkDocument(doc)
	assert.NotNil(t, chunks)
	assert.NotEmpty(t, chunks)
}

func TestFixedSizeChunker_ChunkDocument_NilDoc(t *testing.T) {
	t.Parallel()
	chunker := adapter.NewFixedSizeChunker(100, 10)
	chunks := chunker.ChunkDocument(nil)
	assert.Nil(t, chunks)
}

// ============================================================================
// RerankerAdapter Tests
// ============================================================================

func TestNewScoreReranker(t *testing.T) {
	t.Parallel()
	r := adapter.NewScoreReranker(10)
	require.NotNil(t, r)
}

func TestNewMMRReranker(t *testing.T) {
	t.Parallel()
	r := adapter.NewMMRReranker(0.7, 10)
	require.NotNil(t, r)
}

func TestScoreReranker_Rerank(t *testing.T) {
	t.Parallel()
	r := adapter.NewScoreReranker(5)
	ctx := context.Background()
	results := []helixrag.PipelineSearchResult{
		{Chunk: helixrag.PipelineChunk{ID: "c1", Content: "fox and the hound"}, Score: 0.8},
		{Chunk: helixrag.PipelineChunk{ID: "c2", Content: "the quick brown fox"}, Score: 0.6},
	}

	reranked, err := r.Rerank(ctx, "fox", results, 5)
	require.NoError(t, err)
	assert.NotNil(t, reranked)
}

func TestMMRReranker_Rerank(t *testing.T) {
	t.Parallel()
	r := adapter.NewMMRReranker(0.7, 5)
	ctx := context.Background()
	results := []helixrag.PipelineSearchResult{
		{Chunk: helixrag.PipelineChunk{ID: "c1", Content: "first result"}, Score: 0.9},
		{Chunk: helixrag.PipelineChunk{ID: "c2", Content: "second result"}, Score: 0.7},
	}

	reranked, err := r.Rerank(ctx, "test query", results, 2)
	require.NoError(t, err)
	assert.NotNil(t, reranked)
}

// ============================================================================
// PipelineAdapter Tests
// ============================================================================

func TestNewPipelineAdapter(t *testing.T) {
	t.Parallel()
	retrieverFn := func(ctx context.Context, query string, topK int) ([]helixrag.PipelineSearchResult, error) {
		return []helixrag.PipelineSearchResult{
			{
				Chunk: helixrag.PipelineChunk{ID: "r1", Content: "retrieved document"},
				Score: 0.9,
			},
		}, nil
	}

	p, err := adapter.NewPipelineAdapter(retrieverFn)
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestPipelineAdapter_Search(t *testing.T) {
	t.Parallel()
	retrieverFn := func(ctx context.Context, query string, topK int) ([]helixrag.PipelineSearchResult, error) {
		return []helixrag.PipelineSearchResult{
			{Chunk: helixrag.PipelineChunk{ID: "r1", Content: "retrieved: " + query}, Score: 0.9},
		}, nil
	}

	p, err := adapter.NewPipelineAdapter(retrieverFn)
	require.NoError(t, err)

	results, err := p.Search(context.Background(), "test query", 5)
	require.NoError(t, err)
	assert.NotNil(t, results)
}

// ============================================================================
// ToModuleDocument / ToHelixSearchResult Tests
// ============================================================================

func TestToModuleDocument(t *testing.T) {
	t.Parallel()
	doc := &helixrag.PipelineDocument{
		ID:      "doc-001",
		Content: "test content",
	}

	modDoc := adapter.ToModuleDocument(doc)
	assert.Equal(t, "doc-001", modDoc.ID)
	assert.Equal(t, "test content", modDoc.Content)
}

func TestToModuleDocument_Nil(t *testing.T) {
	t.Parallel()
	modDoc := adapter.ToModuleDocument(nil)
	assert.Empty(t, modDoc.ID)
	assert.Empty(t, modDoc.Content)
}

func TestToHelixSearchResult(t *testing.T) {
	t.Parallel()
	modDoc := adapter.ToModuleDocument(&helixrag.PipelineDocument{
		ID:      "doc-001",
		Content: "test content",
	})
	modDoc.Score = 0.85

	result := adapter.ToHelixSearchResult(modDoc)
	assert.Equal(t, "doc-001", result.Chunk.ID)
	assert.Equal(t, "test content", result.Chunk.Content)
	assert.InDelta(t, 0.85, float64(result.Score), 0.001)
}

// ============================================================================
// FusionMethod constants
// ============================================================================

func TestFusionMethodConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "rrf", adapter.FusionRRF)
	assert.Equal(t, "linear", adapter.FusionLinear)
}
