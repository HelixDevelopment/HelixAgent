package search

import (
	"context"
	"testing"

	"dev.helix.agent/internal/search/types"
	"github.com/stretchr/testify/assert"
)

func TestSearchOptions(t *testing.T) {
	opts := types.SearchOptions{
		TopK:     10,
		MinScore: 0.8,
		Filters: map[string]interface{}{
			"language": "go",
		},
		IncludeContent: true,
	}

	assert.Equal(t, 10, opts.TopK)
	assert.Equal(t, float32(0.8), opts.MinScore)
	assert.NotNil(t, opts.Filters)
	assert.True(t, opts.IncludeContent)
}

func TestSearchResult(t *testing.T) {
	result := &types.SearchResult{
		Document: types.Document{
			ID:      "doc-1",
			Content: "test content",
			Metadata: map[string]interface{}{
				"file_path": "test.go",
			},
		},
		Score:    0.95,
		Distance: 0.05,
	}

	assert.Equal(t, "doc-1", result.ID)
	assert.Equal(t, "test content", result.Content)
	assert.Equal(t, float32(0.95), result.Score)
	assert.Equal(t, float32(0.05), result.Distance)
}

func TestDocument(t *testing.T) {
	doc := types.Document{
		ID:      "doc-1",
		Vector:  []float32{0.1, 0.2, 0.3},
		Content: "test content",
		Metadata: map[string]interface{}{
			"file_path": "test.go",
			"language":  "go",
		},
	}

	assert.Equal(t, "doc-1", doc.ID)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, doc.Vector)
	assert.Equal(t, "test content", doc.Content)
	assert.Equal(t, "test.go", doc.Metadata["file_path"])
}

func TestChunk(t *testing.T) {
	chunk := types.Chunk{
		ID:        "chunk-1",
		Content:   "func main() {}",
		FilePath:  "main.go",
		StartLine: 1,
		EndLine:   10,
		Language:  "go",
		Type:      types.ChunkTypeFunction,
	}

	assert.Equal(t, "chunk-1", chunk.ID)
	assert.Equal(t, "func main() {}", chunk.Content)
	assert.Equal(t, "main.go", chunk.FilePath)
	assert.Equal(t, 1, chunk.StartLine)
	assert.Equal(t, 10, chunk.EndLine)
	assert.Equal(t, "go", chunk.Language)
	assert.Equal(t, types.ChunkTypeFunction, chunk.Type)
}

// Mock implementations for testing
type mockEmbedder struct{}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = []float32{0.1, 0.2, 0.3}
	}
	return result, nil
}

func (m *mockEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *mockEmbedder) Dimensions() int {
	return 3
}

func TestMockEmbedder(t *testing.T) {
	embedder := &mockEmbedder{}

	ctx := context.Background()
	texts := []string{"hello", "world"}

	embeddings, err := embedder.Embed(ctx, texts)
	assert.NoError(t, err)
	assert.Len(t, embeddings, 2)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, embeddings[0])

	queryEmbedding, err := embedder.EmbedQuery(ctx, "test")
	assert.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, queryEmbedding)

	assert.Equal(t, 3, embedder.Dimensions())
}
