package search

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"dev.helix.agent/internal/search/types"
)

func TestTypeAliases(t *testing.T) {
	t.Parallel()

	// Verify that re-exported type aliases work correctly
	var chunk Chunk
	chunk.ID = "test-chunk"
	chunk.Content = "content"
	chunk.FilePath = "file.go"
	chunk.StartLine = 1
	chunk.EndLine = 10
	chunk.Language = "go"
	chunk.Type = ChunkTypeFunction

	assert.Equal(t, "test-chunk", chunk.ID)
	assert.Equal(t, "content", chunk.Content)
	assert.Equal(t, "file.go", chunk.FilePath)
	assert.Equal(t, 1, chunk.StartLine)
	assert.Equal(t, 10, chunk.EndLine)
	assert.Equal(t, "go", chunk.Language)
	assert.Equal(t, types.ChunkTypeFunction, chunk.Type)
}

func TestChunkTypeConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		got      ChunkType
		expected types.ChunkType
	}{
		{"function", ChunkTypeFunction, types.ChunkTypeFunction},
		{"class", ChunkTypeClass, types.ChunkTypeClass},
		{"interface", ChunkTypeInterface, types.ChunkTypeInterface},
		{"method", ChunkTypeMethod, types.ChunkTypeMethod},
		{"comment", ChunkTypeComment, types.ChunkTypeComment},
		{"import", ChunkTypeImport, types.ChunkTypeImport},
		{"general", ChunkTypeGeneral, types.ChunkTypeGeneral},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.got)
		})
	}
}

func TestChunkTypeValues(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ChunkType("function"), ChunkTypeFunction)
	assert.Equal(t, ChunkType("class"), ChunkTypeClass)
	assert.Equal(t, ChunkType("interface"), ChunkTypeInterface)
	assert.Equal(t, ChunkType("method"), ChunkTypeMethod)
	assert.Equal(t, ChunkType("comment"), ChunkTypeComment)
	assert.Equal(t, ChunkType("import"), ChunkTypeImport)
	assert.Equal(t, ChunkType("general"), ChunkTypeGeneral)
}

func TestDocumentAlias(t *testing.T) {
	t.Parallel()

	doc := Document{
		ID:      "doc-1",
		Vector:  []float32{0.1, 0.2},
		Content: "hello",
		Metadata: map[string]interface{}{
			"key": "value",
		},
	}

	assert.Equal(t, "doc-1", doc.ID)
	assert.Equal(t, []float32{0.1, 0.2}, doc.Vector)
	assert.Equal(t, "hello", doc.Content)
	assert.Equal(t, "value", doc.Metadata["key"])
}

func TestSearchOptionsAlias(t *testing.T) {
	t.Parallel()

	opts := SearchOptions{
		TopK:           20,
		MinScore:       0.5,
		Filters:        map[string]interface{}{"lang": "go"},
		IncludeContent: true,
	}

	assert.Equal(t, 20, opts.TopK)
	assert.Equal(t, float32(0.5), opts.MinScore)
	assert.Equal(t, "go", opts.Filters["lang"])
	assert.True(t, opts.IncludeContent)
}

func TestSearchResultAlias(t *testing.T) {
	t.Parallel()

	result := SearchResult{
		Document: Document{ID: "r1", Content: "test"},
		Score:    0.95,
		Distance: 0.05,
	}

	assert.Equal(t, "r1", result.ID)
	assert.Equal(t, float32(0.95), result.Score)
	assert.Equal(t, float32(0.05), result.Distance)
}

func TestCollectionStatsAlias(t *testing.T) {
	t.Parallel()

	stats := CollectionStats{
		Name:       "test_col",
		Count:      1000,
		Dimensions: 768,
	}

	assert.Equal(t, "test_col", stats.Name)
	assert.Equal(t, int64(1000), stats.Count)
	assert.Equal(t, 768, stats.Dimensions)
}

func TestIndexResultAlias(t *testing.T) {
	t.Parallel()

	result := IndexResult{
		FilesIndexed:  42,
		ChunksCreated: 200,
	}

	assert.Equal(t, 42, result.FilesIndexed)
	assert.Equal(t, 200, result.ChunksCreated)
}
