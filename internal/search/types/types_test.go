package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChunkType_Constants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ct       ChunkType
		expected string
	}{
		{"function", ChunkTypeFunction, "function"},
		{"class", ChunkTypeClass, "class"},
		{"interface", ChunkTypeInterface, "interface"},
		{"method", ChunkTypeMethod, "method"},
		{"comment", ChunkTypeComment, "comment"},
		{"import", ChunkTypeImport, "import"},
		{"general", ChunkTypeGeneral, "general"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, ChunkType(tc.expected), tc.ct)
		})
	}
}

func TestChunk_Fields(t *testing.T) {
	t.Parallel()

	chunk := Chunk{
		ID:         "chunk-1",
		Content:    "func main() {}",
		FilePath:   "main.go",
		StartLine:  1,
		EndLine:    10,
		Language:   "go",
		Type:       ChunkTypeFunction,
		Parent:     "main",
		Imports:    []string{"fmt", "os"},
		Embeddings: []float32{0.1, 0.2, 0.3},
	}

	assert.Equal(t, "chunk-1", chunk.ID)
	assert.Equal(t, "func main() {}", chunk.Content)
	assert.Equal(t, "main.go", chunk.FilePath)
	assert.Equal(t, 1, chunk.StartLine)
	assert.Equal(t, 10, chunk.EndLine)
	assert.Equal(t, "go", chunk.Language)
	assert.Equal(t, ChunkTypeFunction, chunk.Type)
	assert.Equal(t, "main", chunk.Parent)
	assert.Equal(t, []string{"fmt", "os"}, chunk.Imports)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, chunk.Embeddings)
}

func TestChunk_ZeroValue(t *testing.T) {
	t.Parallel()

	var chunk Chunk
	assert.Empty(t, chunk.ID)
	assert.Empty(t, chunk.Content)
	assert.Empty(t, chunk.FilePath)
	assert.Zero(t, chunk.StartLine)
	assert.Zero(t, chunk.EndLine)
	assert.Empty(t, chunk.Language)
	assert.Empty(t, string(chunk.Type))
	assert.Empty(t, chunk.Parent)
	assert.Nil(t, chunk.Imports)
	assert.Nil(t, chunk.Embeddings)
}

func TestDocument_Fields(t *testing.T) {
	t.Parallel()

	doc := Document{
		ID:      "doc-1",
		Vector:  []float32{0.5, 0.6},
		Content: "test content",
		Metadata: map[string]interface{}{
			"file_path": "test.go",
			"language":  "go",
		},
	}

	assert.Equal(t, "doc-1", doc.ID)
	assert.Equal(t, []float32{0.5, 0.6}, doc.Vector)
	assert.Equal(t, "test content", doc.Content)
	assert.Equal(t, "test.go", doc.Metadata["file_path"])
	assert.Equal(t, "go", doc.Metadata["language"])
}

func TestDocument_ZeroValue(t *testing.T) {
	t.Parallel()

	var doc Document
	assert.Empty(t, doc.ID)
	assert.Nil(t, doc.Vector)
	assert.Empty(t, doc.Content)
	assert.Nil(t, doc.Metadata)
}

func TestSearchOptions_Fields(t *testing.T) {
	t.Parallel()

	opts := SearchOptions{
		TopK:           20,
		MinScore:       0.75,
		Filters:        map[string]interface{}{"lang": "go"},
		IncludeContent: true,
	}

	assert.Equal(t, 20, opts.TopK)
	assert.Equal(t, float32(0.75), opts.MinScore)
	assert.Equal(t, "go", opts.Filters["lang"])
	assert.True(t, opts.IncludeContent)
}

func TestSearchOptions_ZeroValue(t *testing.T) {
	t.Parallel()

	var opts SearchOptions
	assert.Zero(t, opts.TopK)
	assert.Zero(t, opts.MinScore)
	assert.Nil(t, opts.Filters)
	assert.False(t, opts.IncludeContent)
}

func TestSearchResult_Fields(t *testing.T) {
	t.Parallel()

	result := SearchResult{
		Document: Document{
			ID:      "r1",
			Content: "result content",
		},
		Score:    0.95,
		Distance: 0.05,
	}

	assert.Equal(t, "r1", result.ID)
	assert.Equal(t, "result content", result.Content)
	assert.Equal(t, float32(0.95), result.Score)
	assert.Equal(t, float32(0.05), result.Distance)
}

func TestSearchResult_EmbeddedDocument(t *testing.T) {
	t.Parallel()

	result := SearchResult{
		Document: Document{
			ID:       "r1",
			Vector:   []float32{0.1, 0.2},
			Content:  "content",
			Metadata: map[string]interface{}{"k": "v"},
		},
		Score: 0.9,
	}

	// SearchResult embeds Document, so fields are accessible directly
	assert.Equal(t, "r1", result.ID)
	assert.Equal(t, []float32{0.1, 0.2}, result.Vector)
	assert.Equal(t, "content", result.Content)
	assert.Equal(t, "v", result.Metadata["k"])
}

func TestCollectionStats_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	stats := CollectionStats{
		Name:        "test_collection",
		Count:       1000,
		Dimensions:  768,
		CreatedAt:   now,
		LastUpdated: now,
	}

	assert.Equal(t, "test_collection", stats.Name)
	assert.Equal(t, int64(1000), stats.Count)
	assert.Equal(t, 768, stats.Dimensions)
	assert.Equal(t, now, stats.CreatedAt)
	assert.Equal(t, now, stats.LastUpdated)
}

func TestCollectionStats_ZeroValue(t *testing.T) {
	t.Parallel()

	var stats CollectionStats
	assert.Empty(t, stats.Name)
	assert.Zero(t, stats.Count)
	assert.Zero(t, stats.Dimensions)
	assert.True(t, stats.CreatedAt.IsZero())
	assert.True(t, stats.LastUpdated.IsZero())
}

func TestIndexResult_Fields(t *testing.T) {
	t.Parallel()

	result := IndexResult{
		FilesIndexed:  42,
		ChunksCreated: 200,
		Errors:        nil,
		Duration:      5 * time.Second,
	}

	assert.Equal(t, 42, result.FilesIndexed)
	assert.Equal(t, 200, result.ChunksCreated)
	assert.Nil(t, result.Errors)
	assert.Equal(t, 5*time.Second, result.Duration)
}

func TestIndexResult_WithErrors(t *testing.T) {
	t.Parallel()

	errs := []error{assert.AnError}
	result := IndexResult{
		FilesIndexed:  10,
		ChunksCreated: 50,
		Errors:        errs,
		Duration:      1 * time.Second,
	}

	assert.Len(t, result.Errors, 1)
	assert.Equal(t, assert.AnError, result.Errors[0])
}

func TestIndexResult_ZeroValue(t *testing.T) {
	t.Parallel()

	var result IndexResult
	assert.Zero(t, result.FilesIndexed)
	assert.Zero(t, result.ChunksCreated)
	assert.Nil(t, result.Errors)
	assert.Zero(t, result.Duration)
}
